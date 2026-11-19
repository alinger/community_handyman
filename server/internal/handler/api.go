// Package handler 是 HTTP 层，只做参数绑定、调用 service、封装响应。
package handler

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/alinger/community-handyman/server/internal/dto"
	"github.com/alinger/community-handyman/server/internal/errs"
	"github.com/alinger/community-handyman/server/internal/middleware"
	"github.com/alinger/community-handyman/server/internal/service"
	"github.com/alinger/community-handyman/server/pkg/response"
)

// API 聚合各业务 handler。
type API struct {
	Address  *service.AddressService
	Order    *service.OrderService
	Payment  *service.PaymentService
	User     *service.UserService
	Record   *service.RecordService
	Provider *service.ProviderService
	Admin    *service.AdminService
	Refund   *service.RefundService
	Quote    *service.QuoteService
	Monthly  *service.MonthlyService

	Env      string
	TokenTTL time.Duration
}

// Deps 是 handler 的依赖集合，参数较多时用具名结构比长参数列表清晰。
type Deps struct {
	Address  *service.AddressService
	Order    *service.OrderService
	Payment  *service.PaymentService
	User     *service.UserService
	Record   *service.RecordService
	Provider *service.ProviderService
	Admin    *service.AdminService
	Refund   *service.RefundService
	Quote    *service.QuoteService
	Monthly  *service.MonthlyService
	Env      string
	TokenTTL time.Duration
}

// New 构造 handler 集合。
func New(d Deps) *API {
	return &API{
		Address: d.Address, Order: d.Order, Payment: d.Payment, User: d.User,
		Record: d.Record, Provider: d.Provider, Admin: d.Admin, Refund: d.Refund,
		Quote: d.Quote, Monthly: d.Monthly,
		Env: d.Env, TokenTTL: d.TokenTTL,
	}
}

func bindJSON(c *gin.Context, obj any) bool {
	if err := c.ShouldBindJSON(obj); err != nil {
		response.Fail(c, errs.BizMsg(errs.CodeInvalidParam, "参数校验失败: "+err.Error()))
		return false
	}
	return true
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
	detail, derr := a.Order.Detail(c.Request.Context(), order.UserID, order.OrderNo)
	if derr != nil {
		response.Fail(c, derr)
		return
	}
	resp := dto.CreateOrderResp{Order: *detail}
	if pay != nil {
		resp.Pay = pay
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
	response.OK(c, gin.H{"user_id": u.ID, "openid": u.OpenID, "nickname": u.Nickname, "role": u.Role})
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
