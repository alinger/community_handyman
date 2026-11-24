// Package router 负责路由注册与依赖装配。
package router

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"gorm.io/gorm"

	"github.com/alinger/community-handyman/server/internal/config"
	"github.com/alinger/community-handyman/server/internal/errs"
	"github.com/alinger/community-handyman/server/internal/handler"
	"github.com/alinger/community-handyman/server/internal/middleware"
	"github.com/alinger/community-handyman/server/internal/payment"
	"github.com/alinger/community-handyman/server/internal/repository"
	"github.com/alinger/community-handyman/server/internal/service"
	"github.com/alinger/community-handyman/server/internal/sms"
	"github.com/alinger/community-handyman/server/internal/storage"
	"github.com/alinger/community-handyman/server/internal/wxauth"
	"github.com/alinger/community-handyman/server/pkg/response"
	"github.com/alinger/community-handyman/server/pkg/securefield"
)

const (
	defaultTimeout = 10 * time.Second
	defaultMaxBody = 1 << 20 // 1MB
)

// Deps 暴露给调用方（main）使用的后台任务依赖。
type Deps struct {
	Order *service.OrderService
	SMS   *service.SmsService
}

// New 构建 Gin 引擎并装配业务路由。
// db 为 nil 时只注册探活接口，便于在无数据库环境下本地起服务。
func New(cfg *config.Config, db *gorm.DB, gw payment.Gateway, store storage.Storage,
	smsSender sms.Sender) (*gin.Engine, *Deps) {
	gin.SetMode(cfg.Server.Mode)

	r := gin.New()
	r.Use(middleware.Default(cfg.Server.AllowOrigins, defaultTimeout, defaultMaxBody)...)
	r.GET("/healthz", handler.Health)

	// 未命中路由统一返回 JSON，避免前端拿到 HTML 文本
	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, response.Body{
			Code:      errs.CodeNotFound,
			Message:   errs.Message(errs.CodeNotFound),
			RequestID: c.GetString(response.CtxRequestID),
		})
	})

	// 接口文档：生产环境不暴露，避免把后台接口与错误码摊开给外部
	if cfg.App.Env != "prod" {
		r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	deps := &Deps{}
	if db == nil {
		return r, deps
	}

	// 依赖装配
	areaRepo := repository.NewAreaRepo(db)
	addrRepo := repository.NewAddressRepo(db)
	slotRepo := repository.NewSlotRepo(db)
	orderRepo := repository.NewOrderRepo(db)
	skuRepo := repository.NewSkuRepo(db)
	payRepo := repository.NewPaymentRepo(db)
	userRepo := repository.NewUserRepo(db)
	recordRepo := repository.NewRecordRepo(db)
	adminRepo := repository.NewAdminUserRepo(db)
	quoteRepo := repository.NewQuoteRepo(db)
	redemptionRepo := repository.NewRedemptionRepo(db)
	providerAppRepo := repository.NewProviderAppRepo(db)
	smsRepo := repository.NewSmsRepo(db)
	quotaRepo := repository.NewQuotaRepo(db)
	quotaSvc := service.NewQuotaService(quotaRepo)

	addrSvc := service.NewAddressService(areaRepo, addrRepo)
	// 订阅消息发送依赖 wxClient，先于使用它的服务构造
	wxClient := wxauth.New(cfg.Wechat.AppID, cfg.Wechat.AppSecret, "")
	notifySvc := service.NewNotifyService(wxClient, userRepo, cfg.Wechat.SubscribeTemplateID)
	incomeRepo := repository.NewIncomeRepo(db)
	// 门禁密码等敏感字段的加密器。独立口令优先，未配置时退回登录签名密钥派生，
	// 这样老环境不改 .env 也能启动，但生产应显式配置 SECURITY_ACCESS_KEY。
	accessKey := cfg.Security.AccessKey
	if accessKey == "" {
		accessKey = cfg.Auth.Secret
		slog.Warn("未配置 SECURITY_ACCESS_KEY，门禁信息将使用 AUTH_SECRET 派生密钥")
	}
	secureBox, boxErr := securefield.New(accessKey)
	if boxErr != nil {
		// 只有口令为空才会走到这里（前面已兜底成 AUTH_SECRET）。
		// 降级为「不保存门禁信息」而不是让整个服务起不来：
		// 少存一个选填字段，比全站不可用轻得多。
		slog.Error("初始化敏感字段加密器失败，门禁信息将不会被保存", "err", boxErr)
		secureBox = nil
	}
	orderSvc := service.NewOrderService(db, orderRepo, slotRepo, skuRepo, addrRepo, areaRepo,
		quoteRepo, userRepo, quotaRepo, incomeRepo, notifySvc, gw, secureBox)
	paySvc := service.NewPaymentService(db, orderRepo, payRepo, gw)
	userSvc := service.NewUserService(userRepo)
	avatarSvc := service.NewAvatarService(userRepo, store)
	authSvc := service.NewAuthService(userRepo, quotaRepo, cfg.Auth.Secret, cfg.Auth.TokenTTL)
	recordSvc := service.NewRecordService(recordRepo, orderRepo, store)
	refundSvc := service.NewRefundService(db, orderRepo, slotRepo, payRepo, gw)
	providerSvc := service.NewProviderService(db, orderRepo, slotRepo, quotaRepo,
		incomeRepo, userRepo, notifySvc, refundSvc)
	adminSvc := service.NewAdminService(db, adminRepo, areaRepo, slotRepo, skuRepo, orderRepo,
		cfg.Auth.Secret, cfg.Auth.TokenTTL, notifySvc)
	quoteSvc := service.NewQuoteService(db, orderRepo, quoteRepo, slotRepo)
	monthlySvc := service.NewMonthlyService(db, orderRepo, redemptionRepo)
	providerAppSvc := service.NewProviderAppService(providerAppRepo, userRepo, store)
	rescheduleRepo := repository.NewRescheduleRepo(db)
	rescheduleSvc := service.NewRescheduleService(db, orderRepo, slotRepo, rescheduleRepo, notifySvc)

	// 短信渠道由调用方注入；缺省时退化为控制台输出，保证本地无密钥也能跑通流程
	if smsSender == nil {
		smsSender, _ = sms.New(sms.Options{Provider: "console", Log: slog.Default()})
	}
	smsSvc := service.NewSmsService(smsRepo, userRepo, authSvc, smsSender)

	// 微信登录：dev 且未配 appid 时退化为「code 即 openid」，让前端流程可联调
	wechatAuthSvc := service.NewWechatAuthService(userRepo, authSvc, wxClient,
		slog.Default(), cfg.App.Env == "dev")
	deps.Order = orderSvc
	deps.SMS = smsSvc

	api := handler.New(handler.Deps{
		Address:     addrSvc,
		Order:       orderSvc,
		Payment:     paySvc,
		User:        userSvc,
		Auth:        authSvc,
		Record:      recordSvc,
		Provider:    providerSvc,
		Admin:       adminSvc,
		Refund:      refundSvc,
		Quote:       quoteSvc,
		Monthly:     monthlySvc,
		ProviderApp: providerAppSvc,
		SMS:         smsSvc,
		WechatAuth:  wechatAuthSvc,
		Avatar:      avatarSvc,
		Quota:       quotaSvc,
		Reschedule:  rescheduleSvc,
		Env:         cfg.App.Env,
		TokenTTL:    cfg.Auth.TokenTTL,
		Secret:      cfg.Auth.Secret,
		SkipPay:     cfg.Payment.SkipPay,
	})

	v1 := r.Group("/api/v1")
	{
		// 注册与登录不需要登录态
		v1.POST("/auth/register", api.Register)
		v1.POST("/auth/login", api.UserLogin)
		// 短信验证码：下发、验证码登录（未注册自动建号）、找回密码
		v1.POST("/auth/sms/code", api.SendSmsCode)
		v1.POST("/auth/sms/login", api.SmsLogin)
		v1.POST("/auth/password/reset", api.ResetPasswordBySms)
		// 微信一键授权登录（openid 不存在则建号）
		v1.POST("/auth/wechat", api.WechatLogin)

		// 服务范围公开：访客下单前要先能确认自己小区是否覆盖
		v1.GET("/service-areas", api.ListServiceAreas)
		// 包月方案公开：访客在首页点进预约页就能看到价格与时段
		v1.GET("/services/garbage-plan", api.GarbagePlan)

		// 头像公开读取：image 标签无法带 Authorization 头，文件名由服务端生成、不可猜
		v1.GET("/users/:id/avatar/:name", api.ServeAvatar)

		// 服务记录文件同理：图片/视频由原生组件加载，带不上 Authorization 头，
		// 所以移到鉴权组外，用 URL 上的短期票据（?t=）鉴权，票据绑定用户与具体路径。
		// 带 Bearer 的普通请求仍然放行，前端不需要为它单独改调用方式。
		v1.GET("/orders/:orderNo/records/:recordID/file",
			middleware.RequireUserOrMediaTicket(cfg.Auth.Secret, cfg.App.Env != "prod"),
			api.DownloadRecord)

		// 本地联调登录，正式环境不注册该路由
		if cfg.App.Env == "dev" {
			v1.POST("/dev/login", api.DevLogin)
		}

		// X-User-Id 兼容只在非生产环境开启，线上必须走 Bearer token
		authed := v1.Group("", middleware.RequireUser(cfg.Auth.Secret, cfg.App.Env != "prod"))
		{
			// 个人信息（设置页）
			authed.GET("/me", api.GetMe)
			authed.PUT("/me", api.UpdateMe)
			authed.PUT("/auth/password", api.ChangePassword)
			// 微信登录后补手机号
			authed.POST("/auth/phone", api.BindPhone)
			authed.POST("/auth/wechat/phone", api.BindPhoneByWechat)
			authed.POST("/me/avatar", api.UploadAvatar)
			authed.GET("/me/quotas", api.ListQuotas)

			authed.POST("/addresses", api.CreateAddress)
			authed.GET("/addresses", api.ListAddresses)
			authed.GET("/addresses/communities", api.ListCommunities)
			authed.PUT("/addresses/:id", api.UpdateAddress)
			authed.PUT("/addresses/:id/default", api.SetDefaultAddress)
			authed.DELETE("/addresses/:id", api.DeleteAddress)
			authed.POST("/addresses/check", api.CheckArea)

			// 陪诊可选项：医院与服务项由后台在 SKU 规则里配置，用户端只读
			authed.GET("/services/accompany/options", api.AccompanyOptions)
			// 遛宠时长档位：由后台在 SKU 规则里配置，用户端只读
			authed.GET("/services/dogwalk/options", api.DogwalkOptions)
			// 时段余量：预约页据此把满员时段置灰
			authed.GET("/services/slots", api.SlotAvailability)

			// 服务方资质申请：认证通过后 role 才会被置为 provider
			authed.GET("/provider/application", api.MyProviderApplication)
			authed.POST("/provider/application", api.SubmitProviderApplication)
			authed.POST("/provider/application/cert", api.UploadProviderCert)
			authed.GET("/provider/application/cert/:kind", api.ProviderCertFile)

			authed.POST("/orders/preview", api.PreviewOrder)
			authed.POST("/orders", api.CreateOrder)
			authed.GET("/orders", api.ListOrders)
			authed.GET("/orders/:orderNo", api.OrderDetail)
			authed.POST("/orders/:orderNo/pay", api.RepayOrder)
			authed.POST("/orders/:orderNo/confirm", api.ConfirmOrder)
			authed.POST("/orders/:orderNo/cancel", api.CancelOrder)
			authed.POST("/orders/:orderNo/quote/reject", api.RejectQuote)
			authed.GET("/orders/:orderNo/monthly", api.MonthlyDetail)
			// 改期：住户直接改立即生效；respond 用于处理服务方发起的申请
			authed.POST("/orders/:orderNo/reschedule", api.RescheduleOrder)
			authed.POST("/orders/:orderNo/reschedule/respond", api.RespondReschedule)

			// 沙箱模拟支付，仅 dev 注册；生产环境走真实回调
			if cfg.App.Env == "dev" {
				authed.POST("/dev/orders/:orderNo/mock-pay", api.DevMockPay)
			}

			// 服务记录：可上传、可读，不提供删除
			authed.POST("/orders/:orderNo/records", api.UploadRecord)
			authed.GET("/orders/:orderNo/records", api.ListRecords)
			// 文件直链单独注册在鉴权组外（见上），因为原生图片组件带不上 header

			// 服务方端：必须已通过资质认证（查 users.role，不信 token 里的旧角色）
			roleOf := func(ctx context.Context, uid uint64) (string, error) {
				u, err := userRepo.GetByID(ctx, uid)
				if err != nil {
					return "", err
				}
				if u == nil {
					return "", nil
				}
				return u.Role, nil
			}
			provider := authed.Group("/provider", middleware.RequireProvider(roleOf))
			{
				provider.GET("/orders/open", api.OpenOrders)
				provider.GET("/orders", api.MyOrders)
				provider.POST("/orders/:orderNo/accept", api.AcceptOrder)
				provider.POST("/orders/:orderNo/reject", api.RejectOrder)
				provider.GET("/income", api.ProviderIncome)
				provider.POST("/orders/:orderNo/advance", api.AdvanceOrder)
				// 服务方只能发起申请，生效要等住户确认
				provider.POST("/orders/:orderNo/reschedule", api.ProviderReschedule)
				// 住户直接改期后，服务方点「我知道了」清掉提示
				provider.POST("/orders/:orderNo/reschedule/ack", api.AckProviderReschedule)
				// 月卡按天核销，服务方上门当天自己打卡
				provider.POST("/orders/:orderNo/monthly/redeem", api.ProviderMonthlyRedeem)
				provider.POST("/orders/:orderNo/monthly/finish", api.ProviderMonthlyFinish)
			}
		}
	}

	// 运营后台：登录不鉴权，其余走独立 token
	adminGroup := r.Group("/api/admin")
	{
		adminGroup.POST("/login", api.AdminLogin)

		authedAdmin := adminGroup.Group("", middleware.AdminAuth(cfg.Auth.Secret))
		{
			authedAdmin.GET("/areas", api.AdminListAreas)
			authedAdmin.POST("/areas", api.AdminCreateArea)
			authedAdmin.PATCH("/areas/:id/status", api.AdminSetAreaStatus)

			authedAdmin.GET("/slots", api.AdminListSlots)
			authedAdmin.PUT("/slots", api.AdminSetSlots)

			authedAdmin.GET("/skus", api.AdminListSkus)
			authedAdmin.PUT("/skus/:id", api.AdminUpdateSku)

			authedAdmin.GET("/orders", api.AdminListOrders)
			authedAdmin.GET("/providers", api.AdminListProviders)
			authedAdmin.POST("/orders/:orderNo/assign", api.AdminAssign)
			authedAdmin.POST("/orders/:orderNo/quote", api.AdminCreateQuote)
			authedAdmin.POST("/orders/:orderNo/redeem", api.AdminRedeem)
			authedAdmin.GET("/orders/:orderNo/events", api.AdminOrderEvents)
			authedAdmin.GET("/orders/:orderNo/records", api.AdminOrderRecords)
			authedAdmin.GET("/records/:id/file", api.AdminRecordFile)
			authedAdmin.POST("/orders/:orderNo/cancel", middleware.RequireAdminRole(), api.AdminCancelForUser)
			authedAdmin.POST("/orders/:orderNo/reschedule", middleware.RequireAdminRole(), api.AdminReschedule)

			// 看板与财务
			authedAdmin.GET("/stats", api.AdminStats)
			authedAdmin.GET("/payments", api.AdminListPayments)
			authedAdmin.GET("/refunds", api.AdminListRefunds)

			// 用户管理
			authedAdmin.GET("/users", api.AdminListUsers)
			authedAdmin.PATCH("/users/:id/role", api.AdminSetUserRole)
			authedAdmin.PATCH("/users/:id/status", api.AdminSetUserStatus)

			// 月卡核销与运营配置
			authedAdmin.GET("/redemptions", api.AdminListRedemptions)

			// 服务方资质审核
			authedAdmin.GET("/provider-applications", api.AdminListProviderApps)
			authedAdmin.GET("/provider-applications/stats", api.AdminProviderAppStats)
			authedAdmin.GET("/provider-applications/:id", api.AdminProviderAppDetail)
			authedAdmin.GET("/provider-applications/:id/cert/:kind", api.AdminProviderCertFile)
			authedAdmin.POST("/provider-applications/:id/approve", api.AdminApproveProviderApp)
			authedAdmin.POST("/provider-applications/:id/reject", api.AdminRejectProviderApp)
			authedAdmin.PUT("/areas/:id", api.AdminUpdateArea)
			authedAdmin.POST("/slots/copy", api.AdminCopySlots)

			// 导出（静态路径放参数路由前注册，避免与 /orders/:orderNo 冲突）
			authedAdmin.GET("/export/orders", api.AdminExportOrders)

			// 后台账号管理：仅超管
			accounts := authedAdmin.Group("/accounts", middleware.RequireAdminRole())
			{
				accounts.GET("", api.AdminListAccounts)
				accounts.POST("", api.AdminCreateAccount)
				accounts.PATCH("/:id/status", api.AdminSetAccountStatus)
				accounts.PATCH("/:id/password", api.AdminResetAccountPassword)
			}
			// 自助改密：任意已登录后台账号
			authedAdmin.PATCH("/password", api.AdminChangeMyPassword)

			// 退款是资金操作，要求超管
			authedAdmin.POST("/refunds", middleware.RequireAdminRole(), api.AdminRefund)
		}
	}

	// 支付回调不带用户鉴权，靠渠道验签；沙箱渠道仅在本地启用
	if gw != nil && gw.Name() == "sandbox" {
		r.POST("/callback/sandbox/pay", api.SandboxPayCallback)
	}

	return r, deps
}
