// Package router 负责路由注册与依赖装配。
package router

import (
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
	"github.com/alinger/community-handyman/server/internal/storage"
	"github.com/alinger/community-handyman/server/pkg/response"
)

const (
	defaultTimeout = 10 * time.Second
	defaultMaxBody = 1 << 20 // 1MB
)

// Deps 暴露给调用方（main）使用的后台任务依赖。
type Deps struct {
	Order *service.OrderService
}

// New 构建 Gin 引擎并装配业务路由。
// db 为 nil 时只注册探活接口，便于在无数据库环境下本地起服务。
func New(cfg *config.Config, db *gorm.DB, gw payment.Gateway, store storage.Storage) (*gin.Engine, *Deps) {
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

	addrSvc := service.NewAddressService(areaRepo, addrRepo)
	orderSvc := service.NewOrderService(db, orderRepo, slotRepo, skuRepo, addrRepo, areaRepo, quoteRepo, gw)
	paySvc := service.NewPaymentService(db, orderRepo, payRepo, gw)
	userSvc := service.NewUserService(userRepo)
	recordSvc := service.NewRecordService(recordRepo, orderRepo, store)
	providerSvc := service.NewProviderService(db, orderRepo)
	adminSvc := service.NewAdminService(db, adminRepo, areaRepo, slotRepo, skuRepo, orderRepo,
		cfg.Auth.Secret, cfg.Auth.TokenTTL)
	refundSvc := service.NewRefundService(db, orderRepo, slotRepo, payRepo, gw)
	quoteSvc := service.NewQuoteService(db, orderRepo, quoteRepo, slotRepo)
	monthlySvc := service.NewMonthlyService(db, orderRepo, slotRepo, redemptionRepo)
	deps.Order = orderSvc

	api := handler.New(handler.Deps{
		Address:  addrSvc,
		Order:    orderSvc,
		Payment:  paySvc,
		User:     userSvc,
		Record:   recordSvc,
		Provider: providerSvc,
		Admin:    adminSvc,
		Refund:   refundSvc,
		Quote:    quoteSvc,
		Monthly:  monthlySvc,
		Env:      cfg.App.Env,
		TokenTTL: cfg.Auth.TokenTTL,
	})

	v1 := r.Group("/api/v1")
	{
		// 本地联调登录，正式环境不注册该路由
		if cfg.App.Env == "dev" {
			v1.POST("/dev/login", api.DevLogin)
		}

		authed := v1.Group("", middleware.RequireUser())
		{
			authed.POST("/addresses", api.CreateAddress)
			authed.GET("/addresses", api.ListAddresses)
			authed.GET("/addresses/communities", api.ListCommunities)
			authed.PUT("/addresses/:id", api.UpdateAddress)
			authed.PUT("/addresses/:id/default", api.SetDefaultAddress)
			authed.POST("/addresses/check", api.CheckArea)

			// 陪诊可选项：医院与服务项由后台在 SKU 规则里配置，用户端只读
			authed.GET("/services/accompany/options", api.AccompanyOptions)

			authed.POST("/orders/preview", api.PreviewOrder)
			authed.POST("/orders", api.CreateOrder)
			authed.GET("/orders", api.ListOrders)
			authed.GET("/orders/:orderNo", api.OrderDetail)
			authed.POST("/orders/:orderNo/pay", api.RepayOrder)
			authed.POST("/orders/:orderNo/confirm", api.ConfirmOrder)
			authed.POST("/orders/:orderNo/cancel", api.CancelOrder)
			authed.POST("/orders/:orderNo/quote/reject", api.RejectQuote)
			authed.GET("/orders/:orderNo/monthly", api.MonthlyDetail)

			// 沙箱模拟支付，仅 dev 注册；生产环境走真实回调
			if cfg.App.Env == "dev" {
				authed.POST("/dev/orders/:orderNo/mock-pay", api.DevMockPay)
			}

			// 服务记录：可上传、可读，不提供删除
			authed.POST("/orders/:orderNo/records", api.UploadRecord)
			authed.GET("/orders/:orderNo/records", api.ListRecords)
			authed.GET("/orders/:orderNo/records/:recordID/file", api.DownloadRecord)

			// 服务方端
			provider := authed.Group("/provider")
			{
				provider.GET("/orders/open", api.OpenOrders)
				provider.GET("/orders", api.MyOrders)
				provider.POST("/orders/:orderNo/accept", api.AcceptOrder)
				provider.POST("/orders/:orderNo/reject", api.RejectOrder)
				provider.POST("/orders/:orderNo/advance", api.AdvanceOrder)
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
			authedAdmin.PUT("/areas/:id", api.AdminUpdateArea)
			authedAdmin.POST("/slots/copy", api.AdminCopySlots)

			// 导出（静态路径放参数路由前注册，避免与 /orders/:orderNo 冲突）
			authedAdmin.GET("/export/orders", api.AdminExportOrders)

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
