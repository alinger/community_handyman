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

// ---------- 订单 ----------

// PreviewOrder 费用试算。
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
		resp.ManualQuote = true
		resp.PayHint = "陪诊服务需人工报价，运营确认后再支付"
	}
	response.Created(c, resp)
}

// ListOrders 我的订单，tab 取值 pending / processing / completed。
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
func (a *API) OrderDetail(c *gin.Context) {
	resp, err := a.Order.Detail(c.Request.Context(), middleware.CurrentUserID(c), c.Param("orderNo"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, resp)
}

// RepayOrder 重新发起支付。
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
func (a *API) DevLogin(c *gin.Context) {
	var req struct {
		OpenID   string `json:"openid" binding:"required,max=64"`
		Nickname string `json:"nickname" binding:"max=64"`
		Role     string `json:"role" binding:"omitempty,oneof=user provider admin"`
	}
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
