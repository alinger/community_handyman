// Package handler 是 HTTP 层，只做参数绑定、调用 service、封装响应。
package handler

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/alinger/community-handyman/server/internal/dto"
	"github.com/alinger/community-handyman/server/internal/errs"
	"github.com/alinger/community-handyman/server/internal/middleware"
	"github.com/alinger/community-handyman/server/internal/model"
	"github.com/alinger/community-handyman/server/internal/service"
	"github.com/alinger/community-handyman/server/pkg/response"
)

// API 聚合各业务 handler。
type API struct {
	Address     *service.AddressService
	Order       *service.OrderService
	Payment     *service.PaymentService
	User        *service.UserService
	Auth        *service.AuthService
	Record      *service.RecordService
	Provider    *service.ProviderService
	Admin       *service.AdminService
	Refund      *service.RefundService
	Quote       *service.QuoteService
	Monthly     *service.MonthlyService
	ProviderApp *service.ProviderAppService
	SMS         *service.SmsService
	WechatAuth  *service.WechatAuthService
	Avatar      *service.AvatarService
	Quota       *service.QuotaService
	// Reschedule 改期（住户直接改 / 服务方申请 / 住户确认）
	Reschedule *service.RescheduleService

	Env      string
	TokenTTL time.Duration
	// Secret 用于签发文件直链的短期票据（Auth 密钥），与登录 token 同源
	Secret string
	// SkipPay 为真时下单后直接按支付成功推进订单（试运行期，未接微信支付）
	SkipPay bool
}

// Deps 是 handler 的依赖集合，参数较多时用具名结构比长参数列表清晰。
type Deps struct {
	Address     *service.AddressService
	Order       *service.OrderService
	Payment     *service.PaymentService
	User        *service.UserService
	Auth        *service.AuthService
	Record      *service.RecordService
	Provider    *service.ProviderService
	Admin       *service.AdminService
	Refund      *service.RefundService
	Quote       *service.QuoteService
	Monthly     *service.MonthlyService
	ProviderApp *service.ProviderAppService
	SMS         *service.SmsService
	WechatAuth  *service.WechatAuthService
	Avatar      *service.AvatarService
	Quota       *service.QuotaService
	Reschedule  *service.RescheduleService
	Env         string
	TokenTTL    time.Duration
	Secret      string
	SkipPay     bool
}

// New 构造 handler 集合。
func New(d Deps) *API {
	return &API{
		Address: d.Address, Order: d.Order, Payment: d.Payment, User: d.User, Auth: d.Auth,
		Record: d.Record, Provider: d.Provider, Admin: d.Admin, Refund: d.Refund,
		Quote: d.Quote, Monthly: d.Monthly, ProviderApp: d.ProviderApp, SMS: d.SMS,
		WechatAuth: d.WechatAuth, Avatar: d.Avatar, Quota: d.Quota, Env: d.Env, TokenTTL: d.TokenTTL,
		Reschedule: d.Reschedule,
		Secret:     d.Secret, SkipPay: d.SkipPay,
	}
}

func bindJSON(c *gin.Context, obj any) bool {
	if err := c.ShouldBindJSON(obj); err != nil {
		response.Fail(c, errs.BizMsg(errs.CodeInvalidParam, "参数校验失败: "+err.Error()))
		return false
	}
	return true
}

// trimSpace 去除首尾空白（含全角空格）。
func trimSpace(s string) string {
	return strings.Trim(s, " \t\n\r　")
}

// ---------- 地址 ----------

// CreateAddress 新增地址，保存前完成服务范围校验。
// @Summary      新增地址
// @Description  保存前校验服务范围，超出范围返回 2001
// @Tags         地址
// @Accept       json
// @Produce      json
// @Param        body  body  dto.CreateAddressReq  true  "地址信息"
// @Success      201   {object}  response.Body  "data 为 dto.AddressResp"
// @Failure      200   {object}  response.Body  "业务错误，见 code"
// @Security     UserID
// @Router       /api/v1/addresses [post]
func (a *API) CreateAddress(c *gin.Context) {
	var req dto.CreateAddressReq
	if !bindJSON(c, &req) {
		return
	}
	addr, err := a.Address.Create(c.Request.Context(), middleware.CurrentUserID(c), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Created(c, service.ToAddressResp(addr))
}

// UpdateAddress 编辑地址，保存前重新校验服务范围与归属。
// @Summary      编辑地址
// @Description  只能编辑本人地址；超出服务范围返回 2001，地址不存在返回 1004
// @Tags         地址
// @Accept       json
// @Produce      json
// @Param        id    path  int                  true  "地址 ID"
// @Param        body  body  dto.UpdateAddressReq true  "地址信息"
// @Success      200   {object}  response.Body  "data 为 dto.AddressResp"
// @Failure      200   {object}  response.Body  "业务错误，见 code"
// @Security     UserID
// @Router       /api/v1/addresses/{id} [put]
func (a *API) UpdateAddress(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.Fail(c, errs.BizMsg(errs.CodeInvalidParam, "地址 ID 不合法"))
		return
	}
	var req dto.UpdateAddressReq
	if !bindJSON(c, &req) {
		return
	}
	addr, err := a.Address.Update(c.Request.Context(), middleware.CurrentUserID(c), id, req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, service.ToAddressResp(addr))
}

// SetDefaultAddress 把某条地址设为默认。
// @Summary      设置默认地址
// @Description  设为默认后该用户其它地址自动取消默认，返回更新后的地址列表
// @Tags         地址
// @Produce      json
// @Param        id   path      int  true  "地址 ID"
// @Success      200  {object}  response.Body  "data 为 dto.AddressResp 数组"
// @Security     UserID
// @Router       /api/v1/addresses/{id}/default [put]
func (a *API) SetDefaultAddress(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.Fail(c, errs.BizMsg(errs.CodeInvalidParam, "地址 ID 不合法"))
		return
	}
	if err := a.Address.SetDefault(c.Request.Context(), middleware.CurrentUserID(c), id); err != nil {
		response.Fail(c, err)
		return
	}
	list, err := a.Address.List(c.Request.Context(), middleware.CurrentUserID(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	out := make([]dto.AddressResp, 0, len(list))
	for i := range list {
		out = append(out, service.ToAddressResp(&list[i]))
	}
	response.OK(c, out)
}

// DeleteAddress 删除地址。
// @Summary      删除地址
// @Description  只能删除本人地址；删除默认地址时自动把剩余最新一条提升为默认，返回删除后的地址列表
// @Tags         地址
// @Produce      json
// @Param        id   path      int  true  "地址 ID"
// @Success      200  {object}  response.Body  "data 为 dto.AddressResp 数组"
// @Failure      200  {object}  response.Body  "1004 地址不存在"
// @Security     UserID
// @Router       /api/v1/addresses/{id} [delete]
func (a *API) DeleteAddress(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.Fail(c, errs.BizMsg(errs.CodeInvalidParam, "地址 ID 不合法"))
		return
	}
	uid := middleware.CurrentUserID(c)
	if err := a.Address.Delete(c.Request.Context(), uid, id); err != nil {
		response.Fail(c, err)
		return
	}
	// 直接返回剩余列表，前端删完不用再拉一次
	list, err := a.Address.List(c.Request.Context(), uid)
	if err != nil {
		response.Fail(c, err)
		return
	}
	out := make([]dto.AddressResp, 0, len(list))
	for i := range list {
		out = append(out, service.ToAddressResp(&list[i]))
	}
	response.OK(c, out)
}

// ListAddresses 地址列表。
// @Summary      地址列表
// @Tags         地址
// @Produce      json
// @Success      200  {object}  response.Body  "data 为 dto.AddressResp 数组"
// @Security     UserID
// @Router       /api/v1/addresses [get]
func (a *API) ListAddresses(c *gin.Context) {
	list, err := a.Address.List(c.Request.Context(), middleware.CurrentUserID(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	out := make([]dto.AddressResp, 0, len(list))
	for i := range list {
		out = append(out, service.ToAddressResp(&list[i]))
	}
	response.OK(c, out)
}

// ListCommunities 可选小区列表（后台配置的服务范围按小区去重）。
// @Summary      可选小区列表
// @Description  新增地址时小区只能从这份后台配置列表中选择，不能自由输入
// @Tags         地址
// @Produce      json
// @Success      200  {object}  response.Body  "data 为小区名称字符串数组"
// @Failure      200  {object}  response.Body  "业务错误，见 code"
// @Security     UserID
// @Router       /api/v1/addresses/communities [get]
// GarbagePlan 代倒垃圾包月方案（公开）。
// @Summary      包月方案
// @Description  价格、天数、默认取件时段与可选额外时段；数据来自 SKU 规则，改配置不用发版
// @Tags         服务
// @Produce      json
// @Success      200  {object}  response.Body  "data 为 dto.GarbagePlanResp"
// @Router       /api/v1/services/garbage-plan [get]
func (a *API) GarbagePlan(c *gin.Context) {
	res, err := a.Order.GarbagePlan(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

// ListQuotas 我的免费次数。
// @Summary      免费次数余额
// @Description  注册赠送的免费倒垃圾次数；remaining 为 0 时前端应引导购买包月
// @Tags         个人信息
// @Produce      json
// @Success      200  {object}  response.Body  "data 为 []dto.QuotaResp"
// @Security     UserID
// @Router       /api/v1/me/quotas [get]
func (a *API) ListQuotas(c *gin.Context) {
	list, err := a.Quota.List(c.Request.Context(), middleware.CurrentUserID(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, list)
}

// ListServiceAreas 已开通的服务范围（公开）。
// @Summary      服务范围
// @Description  按小区聚合的路段与门牌区间，供「服务范围」页展示；无需登录，访客也能看。
// @Tags         地址
// @Produce      json
// @Success      200  {object}  response.Body  "data 为 []dto.ServiceAreaScope"
// @Failure      200  {object}  response.Body  "业务错误，见 code"
// @Router       /api/v1/service-areas [get]
func (a *API) ListServiceAreas(c *gin.Context) {
	list, err := a.Address.AreaScopes(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	if list == nil {
		list = []dto.ServiceAreaScope{}
	}
	response.OK(c, list)
}

func (a *API) ListCommunities(c *gin.Context) {
	list, err := a.Address.Communities(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	if list == nil {
		list = []string{}
	}
	response.OK(c, list)
}

// CheckArea 地址范围预校验，用于用户输入时的即时反馈。
// @Summary      地址范围预校验
// @Tags         地址
// @Accept       json
// @Produce      json
// @Param        body  body  dto.CheckAreaReq  true  "小区与门牌"
// @Success      200  {object}  response.Body
// @Failure      200  {object}  response.Body  "2001 表示超出服务范围"
// @Security     UserID
// @Router       /api/v1/addresses/check [post]
func (a *API) CheckArea(c *gin.Context) {
	var req dto.CheckAreaReq
	if !bindJSON(c, &req) {
		return
	}
	area, err := a.Address.Check(c.Request.Context(), req.Community, req.Lane, req.HouseNo)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{
		"in_service_area": true,
		"community":       area.Community,
		"lane":            area.Lane,
	})
}

// AccompanyOptions 陪诊可选项（医院与可勾选服务项及单价）。
// @Summary      陪诊服务选项
// @Description  返回后台配置的医院列表与可多选的陪诊服务项（含单价，单位分）；价格仅供展示，下单金额由后端按同一配置计算
// @Tags         服务配置
// @Produce      json
// @Success      200  {object}  response.Body  "data 为 pricing.AccompanyOptions"
// @Security     UserID
// @Router       /api/v1/services/accompany/options [get]
func (a *API) AccompanyOptions(c *gin.Context) {
	opts, err := a.Order.AccompanyOptions(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, opts)
}

// DogwalkOptions 遛宠预约选项（服务时长档位与一口价）。
// @Summary      遛宠服务选项
// @Description  返回后台配置的服务时长档位（含一口价，单位分）；价格仅供展示，下单金额由后端按同一配置计算
// @Tags         服务配置
// @Produce      json
// @Success      200  {object}  response.Body  "data 为 pricing.DogWalkRules"
// @Security     UserID
// @Router       /api/v1/services/dogwalk/options [get]
func (a *API) DogwalkOptions(c *gin.Context) {
	opts, err := a.Order.DogwalkOptions(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, opts)
}

// SlotAvailability 某日各时段余量（预约页据此把满员时段置灰）。
// @Summary      时段余量
// @Description  返回指定日期与服务类型的各时段库存；后台未配置该日库存时返回空数组，前端视为全部可选
// @Tags         服务配置
// @Produce      json
// @Param        date         query  string  true  "服务日期 YYYY-MM-DD"
// @Param        service_type query  string  true  "服务类型 garbage/dogwalk/accompany"
// @Success      200  {object}  response.Body  "data 为 dto.SlotAvailability 数组"
// @Security     UserID
// @Router       /api/v1/services/slots [get]
func (a *API) SlotAvailability(c *gin.Context) {
	date := c.Query("date")
	st := c.Query("service_type")
	if date == "" || st == "" {
		response.Fail(c, errs.BizMsg(errs.CodeInvalidParam, "缺少 date 或 service_type"))
		return
	}
	list, err := a.Order.SlotAvailability(c.Request.Context(), date, model.ServiceType(st))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, list)
}

// ---------- 订单 ----------

// PreviewOrder 费用试算。
// @Summary      费用试算
// @Description  只计算不落库，用于提交前展示费用明细
// @Tags         订单
// @Accept       json
// @Produce      json
// @Param        body  body  dto.CreateOrderReq  true  "订单参数"
// @Success      200  {object}  response.Body  "data 为 dto.PreviewOrderResp"
// @Security     UserID
// @Router       /api/v1/orders/preview [post]
func (a *API) PreviewOrder(c *gin.Context) {
	var req dto.PreviewOrderReq
	if !bindJSON(c, &req) {
		return
	}
	res, err := a.Order.Preview(c.Request.Context(), middleware.CurrentUserID(c), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

// CreateOrder 下单并返回支付参数。
// @Summary      创建订单
// @Description  预占时段并返回支付参数；陪诊按后台配置的服务项单价自动求和，直接支付
// @Tags         订单
// @Accept       json
// @Produce      json
// @Param        body  body  dto.CreateOrderReq  true  "订单参数"
// @Success      201  {object}  response.Body  "data 为 dto.CreateOrderResp"
// @Failure      200  {object}  response.Body  "2001 超范围 / 2002 时段已满 / 3001 支付下单失败"
// @Security     UserID
// @Router       /api/v1/orders [post]
func (a *API) CreateOrder(c *gin.Context) {
	var req dto.CreateOrderReq
	if !bindJSON(c, &req) {
		return
	}
	order, pay, err := a.Order.Create(c.Request.Context(), middleware.CurrentUserID(c), req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	// 试运行期跳过支付：直接按「支付成功」把订单推到待接单，并作废支付参数。
	// 前端拿到的 order.status 已经是待接单，因此不会再走支付弹窗。
	// 0 元订单（免费次数抵扣）无论开关如何都没有支付环节，走同一条推进逻辑。
	if a.SkipPay || order.TotalAmount == 0 {
		if err := a.Payment.SkipPay(c.Request.Context(), order.OrderNo); err != nil {
			response.Fail(c, err)
			return
		}
		pay = nil
	}

	// 跳过支付后状态已变，详情必须重新取
	detail, derr := a.Order.Detail(c.Request.Context(), order.UserID, order.OrderNo)
	if derr != nil {
		response.Fail(c, derr)
		return
	}
	resp := dto.CreateOrderResp{Order: *detail}
	if pay != nil {
		resp.Pay = pay
	} else if a.SkipPay || order.TotalAmount == 0 {
		resp.PayHint = "无需在线支付，订单已提交"
	} else {
		// 正常流程所有服务都会返回支付参数；这里只留兜底，不再有人工报价
		resp.PayHint = "请在订单详情页完成支付"
	}
	response.Created(c, resp)
}

// ListOrders 我的订单，tab 取值 pending / processing / completed。
// @Summary      我的订单
// @Tags         订单
// @Produce      json
// @Param        tab   query  string  false  "pending / processing / completed，留空为全部"
// @Param        page  query  int     false  "页码，默认 1"
// @Param        size  query  int     false  "每页条数，默认 10"
// @Success      200  {object}  response.Body  "data 为 dto.OrderResp 分页"
// @Security     UserID
// @Router       /api/v1/orders [get]
func (a *API) ListOrders(c *gin.Context) {
	page := atoiDefault(c.Query("page"), 1)
	size := atoiDefault(c.Query("size"), 10)
	list, total, err := a.Order.List(c.Request.Context(), middleware.CurrentUserID(c), c.Query("tab"), page, size)
	if err != nil {
		response.Fail(c, err)
		return
	}
	// 列表里也要标出「有待确认的改期申请」：不说的话住户要逐单点进详情才知道有人在等自己确认
	if a.Reschedule != nil {
		a.Reschedule.AttachPendingTo(c.Request.Context(), list)
	}
	response.Page(c, list, total, page, size)
}

// OrderDetail 订单详情。
// @Summary      订单详情
// @Description  含费用明细、地址快照与节点时间轴
// @Tags         订单
// @Produce      json
// @Param        orderNo  path  string  true  "订单号"
// @Success      200  {object}  response.Body  "data 为 dto.OrderResp"
// @Security     UserID
// @Router       /api/v1/orders/{orderNo} [get]
func (a *API) OrderDetail(c *gin.Context) {
	resp, err := a.Order.Detail(c.Request.Context(), middleware.CurrentUserID(c), c.Param("orderNo"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	// 改期信息单独查：列表接口不需要，只有详情要用到「待确认申请」与改期历史
	if a.Reschedule != nil {
		a.Reschedule.AttachTo(c.Request.Context(), c.Param("orderNo"), resp)
		// 服务方看自己的单时，住户改过的日期需要一个明确交代
		a.Reschedule.AttachProviderNotice(c.Request.Context(), middleware.CurrentUserID(c), resp)
	}
	response.OK(c, resp)
}

// RepayOrder 重新发起支付。
// @Summary      重新发起支付
// @Tags         订单
// @Produce      json
// @Param        orderNo  path  string  true  "订单号"
// @Success      200  {object}  response.Body  "data 为支付参数"
// @Failure      200  {object}  response.Body  "2004 已超时 / 2003 状态不允许"
// @Security     UserID
// @Router       /api/v1/orders/{orderNo}/pay [post]
func (a *API) RepayOrder(c *gin.Context) {
	pr, err := a.Order.Repay(c.Request.Context(), middleware.CurrentUserID(c), c.Param("orderNo"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, pr)
}

// ---------- 回调 ----------

// SandboxPayCallback 沙箱支付回调，仅沙箱渠道启用。
// @Summary      沙箱支付回调
// @Description  仅 sandbox 渠道启用。报文：order_no、transaction_id、amount、sign（HMAC-SHA256）
// @Tags         回调
// @Accept       json
// @Produce      json
// @Success      200  {object}  response.Body
// @Failure      200  {object}  response.Body  "1001 验签失败或金额不一致"
// @Router       /callback/sandbox/pay [post]
func (a *API) SandboxPayCallback(c *gin.Context) {
	body, err := c.GetRawData()
	if err != nil {
		response.Fail(c, errs.BizMsg(errs.CodeInvalidParam, "读取回调报文失败"))
		return
	}
	if err := a.Payment.HandleNotify(c.Request.Context(), c.Request.Header, body); err != nil {
		response.Fail(c, err)
		return
	}
	// 渠道侧约定：返回 200 即视为已受理
	response.OK(c, gin.H{"received": true})
}

// WeChatPayCallback 微信支付 APIv3 回调。
// 验签与解密在 Gateway.VerifyNotify 内完成，与沙箱回调共用 HandleNotify 落账逻辑：
// 幂等靠流水号唯一索引，金额校验在 PaymentService，渠道由配置决定，路由只负责收包。
func (a *API) WeChatPayCallback(c *gin.Context) {
	a.SandboxPayCallback(c)
}

// ---------- 个人信息（设置页） ----------

// GetMe 当前用户信息。
// @Summary      当前用户信息
// @Description  设置页展示昵称、手机号等个人资料
// @Tags         个人信息
// @Produce      json
// @Success      200  {object}  response.Body  "data 为 dto.MeResp"
// @Security     UserID
// @Router       /api/v1/me [get]
func (a *API) GetMe(c *gin.Context) {
	u, err := a.User.GetByID(c.Request.Context(), middleware.CurrentUserID(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, toMeResp(u))
}

// UpdateMe 修改个人信息（昵称、手机号）。
// @Summary      修改个人信息
// @Description  整表提交，nickname 必填、phone 可留空清除
// @Tags         个人信息
// @Accept       json
// @Produce      json
// @Param        body  body  dto.UpdateMeReq  true  "昵称与手机号"
// @Success      200  {object}  response.Body  "data 为更新后的 dto.MeResp"
// @Failure      200  {object}  response.Body  "业务错误，见 code"
// @Security     UserID
// @Router       /api/v1/me [put]
func (a *API) UpdateMe(c *gin.Context) {
	var req dto.UpdateMeReq
	if !bindJSON(c, &req) {
		return
	}
	nickname := trimSpace(req.Nickname)
	if nickname == "" {
		response.Fail(c, errs.BizMsg(errs.CodeInvalidParam, "昵称不能为空"))
		return
	}
	uid := middleware.CurrentUserID(c)
	u, err := a.User.UpdateProfile(c.Request.Context(), uid, nickname, trimSpace(req.Phone),
		trimSpace(req.Avatar))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, toMeResp(u))
}

func toMeResp(u *model.User) dto.MeResp {
	return dto.MeResp{
		UserID:   u.ID,
		Nickname: u.Nickname,
		Phone:    u.Phone,
		Role:     u.Role,
		Avatar:   u.Avatar,
	}
}

// ---------- 联调辅助 ----------

// DevLogin 本地联调用：按 openid 取或建用户，返回 user_id。
// 仅在 env=dev 时注册，正式环境不存在该路由。
// @Summary      本地联调登录
// @Description  按 openid 取或建用户，仅在 env=dev 时注册该路由
// @Tags         联调
// @Accept       json
// @Produce      json
// @Param        body  body  dto.DevLoginReq  true  "openid 与角色"
// @Success      200  {object}  response.Body  "data 含 user_id"
// @Router       /api/v1/dev/login [post]
func (a *API) DevLogin(c *gin.Context) {
	var req dto.DevLoginReq
	if !bindJSON(c, &req) {
		return
	}
	u, err := a.User.EnsureByOpenID(c.Request.Context(), req.OpenID, req.Nickname)
	if err != nil {
		response.Fail(c, err)
		return
	}
	if req.Role != "" && req.Role != u.Role {
		if err := a.User.SetRole(c.Request.Context(), u.ID, req.Role); err != nil {
			response.Fail(c, err)
			return
		}
		u.Role = req.Role
	}
	// 与正式登录返回同一套结构，前端两条路径共用一份会话处理逻辑
	s, err := a.Auth.SessionFor(u)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{
		"user_id":    u.ID,
		"openid":     u.OpenID,
		"nickname":   u.Nickname,
		"role":       u.Role,
		"token":      s.Token,
		"expires_in": s.ExpiresIn,
	})
}

// DevMockPay 模拟支付成功，仅 dev 环境注册该路由。
func (a *API) DevMockPay(c *gin.Context) {
	if err := a.Payment.MockPay(c.Request.Context(), c.Param("orderNo")); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"paid": true})
}

// atoiDefault 解析查询参数，失败时取默认值。
func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return def
		}
		n = n*10 + int(r-'0')
	}
	if n == 0 {
		return def
	}
	return n
}
