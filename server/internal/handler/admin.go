package handler

import (
	"context"

	"github.com/gin-gonic/gin"

	"github.com/alinger/community-handyman/server/internal/dto"
	"github.com/alinger/community-handyman/server/internal/errs"
	"github.com/alinger/community-handyman/server/internal/middleware"
	"github.com/alinger/community-handyman/server/internal/model"
	"github.com/alinger/community-handyman/server/internal/service"
	"github.com/alinger/community-handyman/server/pkg/response"
)

// AdminLogin 后台登录。
// @Summary      后台登录
// @Description  返回 Bearer token，默认 12 小时有效
// @Tags         运营后台
// @Accept       json
// @Produce      json
// @Param        body  body  dto.AdminLoginReq  true  "账号密码"
// @Success      200  {object}  response.Body
// @Failure      200  {object}  response.Body  "1002 账号或密码错误"
// @Router       /api/admin/login [post]
func (a *API) AdminLogin(c *gin.Context) {
	var req dto.AdminLoginReq
	if !bindJSON(c, &req) {
		return
	}
	token, admin, err := a.Admin.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{
		"token":      token,
		"expires_in": int(a.TokenTTL.Seconds()),
		"admin": gin.H{
			"id":       admin.ID,
			"username": admin.Username,
			"role":     admin.Role,
		},
	})
}

// AdminListAreas 服务范围列表。
// @Summary      服务范围列表
// @Tags         运营后台
// @Produce      json
// @Success      200  {object}  response.Body
// @Security     AdminToken
// @Router       /api/admin/areas [get]
func (a *API) AdminListAreas(c *gin.Context) {
	list, err := a.Admin.ListAreas(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, list)
}

// AdminCreateArea 新增服务范围。
// @Summary      新增服务范围
// @Tags         运营后台
// @Accept       json
// @Produce      json
// @Param        body  body  dto.CreateAreaReq  true  "范围规则"
// @Success      201  {object}  response.Body
// @Security     AdminToken
// @Router       /api/admin/areas [post]
func (a *API) AdminCreateArea(c *gin.Context) {
	var req dto.CreateAreaReq
	if !bindJSON(c, &req) {
		return
	}
	area, err := a.Admin.CreateArea(c.Request.Context(), &model.ServiceArea{
		Community:   req.Community,
		Lane:        req.Lane,
		HouseFrom:   req.HouseFrom,
		HouseTo:     req.HouseTo,
		HouseParity: req.HouseParity,
		Status:      1,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Created(c, area)
}

// AdminSetAreaStatus 启停服务范围。
// @Summary      启停服务范围
// @Description  停用不影响已存在的订单
// @Tags         运营后台
// @Accept       json
// @Produce      json
// @Param        id    path  int                     true  "范围 ID"
// @Param        body  body  dto.SetAreaStatusReq    true  "1 启用 / 0 停用"
// @Success      200  {object}  response.Body
// @Security     AdminToken
// @Router       /api/admin/areas/{id}/status [patch]
func (a *API) AdminSetAreaStatus(c *gin.Context) {
	id := uint64(atoiDefault(c.Param("id"), 0))
	if id == 0 {
		response.Fail(c, errs.BizMsg(errs.CodeInvalidParam, "ID 不合法"))
		return
	}
	var req dto.SetAreaStatusReq
	if !bindJSON(c, &req) {
		return
	}
	if err := a.Admin.SetAreaStatus(c.Request.Context(), id, req.Status); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"id": id, "status": req.Status})
}

// AdminListSlots 某天时段库存。
// @Summary      某天时段库存
// @Tags         运营后台
// @Produce      json
// @Param        date  query  string  true  "日期，格式 2006-01-02"
// @Success      200  {object}  response.Body
// @Security     AdminToken
// @Router       /api/admin/slots [get]
func (a *API) AdminListSlots(c *gin.Context) {
	list, err := a.Admin.ListSlots(c.Request.Context(), c.Query("date"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, list)
}

// AdminSetSlots 批量设置时段可接单量。
// @Summary      批量设置时段可接单量
// @Description  total=0 表示该时段不开放
// @Tags         运营后台
// @Accept       json
// @Produce      json
// @Param        body  body  dto.SetSlotsReq  true  "日期与时段列表"
// @Success      200  {object}  response.Body
// @Security     AdminToken
// @Router       /api/admin/slots [put]
func (a *API) AdminSetSlots(c *gin.Context) {
	var req dto.SetSlotsReq
	if !bindJSON(c, &req) {
		return
	}
	if err := a.Admin.SetSlotTotals(c.Request.Context(), req.Date, req.Items); err != nil {
		response.Fail(c, err)
		return
	}
	list, err := a.Admin.ListSlots(c.Request.Context(), req.Date)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, list)
}

// AdminListSkus 服务产品列表。
// @Summary      服务产品列表
// @Tags         运营后台
// @Produce      json
// @Success      200  {object}  response.Body
// @Security     AdminToken
// @Router       /api/admin/skus [get]
func (a *API) AdminListSkus(c *gin.Context) {
	list, err := a.Admin.ListSkus(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, list)
}

// AdminUpdateSku 改价格与计价规则（含加价日期）。
// @Summary      改价与计价规则
// @Description  rules 做 JSON 部分合并，只传要改的键（如 holiday_dates），其余计价参数保持不变
// @Tags         运营后台
// @Accept       json
// @Produce      json
// @Param        id    path  int                true  "SKU ID"
// @Param        body  body  dto.UpdateSkuReq   true  "价格与规则"
// @Success      200  {object}  response.Body
// @Security     AdminToken
// @Router       /api/admin/skus/{id} [put]
func (a *API) AdminUpdateSku(c *gin.Context) {
	id := uint64(atoiDefault(c.Param("id"), 0))
	if id == 0 {
		response.Fail(c, errs.BizMsg(errs.CodeInvalidParam, "ID 不合法"))
		return
	}
	var req dto.UpdateSkuReq
	if !bindJSON(c, &req) {
		return
	}
	if err := a.Admin.UpdateSku(c.Request.Context(), id, req.BasePrice, req.Rules); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"id": id, "base_price": req.BasePrice})
}

// AdminListOrders 后台订单查询。
// @Summary      后台订单查询
// @Tags         运营后台
// @Produce      json
// @Param        status        query  string  false  "订单状态"
// @Param        service_type  query  string  false  "服务类型"
// @Param        keyword       query  string  false  "订单号模糊搜索"
// @Param        page          query  int     false  "页码"
// @Param        size          query  int     false  "每页条数"
// @Success      200  {object}  response.Body
// @Security     AdminToken
// @Router       /api/admin/orders [get]
func (a *API) AdminListOrders(c *gin.Context) {
	page := atoiDefault(c.Query("page"), 1)
	size := atoiDefault(c.Query("size"), 10)
	list, total, err := a.Admin.ListOrders(c.Request.Context(),
		c.Query("status"), c.Query("service_type"), c.Query("keyword"), page, size)
	if err != nil {
		response.Fail(c, err)
		return
	}
	out := make([]dto.OrderResp, 0, len(list))
	for i := range list {
		out = append(out, a.Order.ToOrderResp(&list[i], nil))
	}
	a.fillAdminOrderExt(c.Request.Context(), list, out)
	response.Page(c, out, total, page, size)
}

// fillAdminOrderExt 批量回填后台扩展信息：下单用户/服务方昵称、数量、退款额、取消原因、关键时间点。
func (a *API) fillAdminOrderExt(ctx context.Context, orders []model.Order, outs []dto.OrderResp) {
	ids := make([]uint64, 0, len(orders)*2)
	for i := range orders {
		ids = append(ids, orders[i].UserID)
		if orders[i].ProviderID != nil {
			ids = append(ids, *orders[i].ProviderID)
		}
	}
	nick, err := a.Admin.NicknameMap(ctx, ids)
	if err != nil {
		return // 昵称回填失败不影响主数据
	}
	for i := range orders {
		o, r := &orders[i], &outs[i]
		r.UserID = o.UserID
		r.UserNickname = nick[o.UserID]
		r.ProviderID = o.ProviderID
		if o.ProviderID != nil {
			r.ProviderNickname = nick[*o.ProviderID]
		}
		r.Quantity = o.Quantity
		r.PetCount = o.PetCount
		r.RefundedAmount = o.RefundedAmount
		r.CancelReason = o.CancelReason
		r.PaidAt = o.PaidAt
		r.FinishedAt = o.FinishedAt
		if o.Plan == model.PlanMonthly {
			start := o.ServiceDate.Time()
			r.MonthlyStart = start.Format("2006-01-02")
			r.MonthlyEnd = start.AddDate(0, 0, service.MonthlyDays-1).Format("2006-01-02")
		}
	}
}

// AdminListProviders 可派单的服务方列表。
// @Summary      可派单服务方列表
// @Tags         运营后台
// @Produce      json
// @Success      200  {object}  response.Body  "data 为 dto.ProviderOption 数组"
// @Security     AdminToken
// @Router       /api/admin/providers [get]
func (a *API) AdminListProviders(c *gin.Context) {
	list, err := a.Admin.ListProviders(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, list)
}

// AdminAssign 派单或改派。
// @Summary      派单或改派
// @Description  仅待接单的订单可派
// @Tags         运营后台
// @Accept       json
// @Produce      json
// @Param        orderNo  path  string             true  "订单号"
// @Param        body     body  dto.AssignOrderReq true  "服务方 ID"
// @Success      200  {object}  response.Body
// @Security     AdminToken
// @Router       /api/admin/orders/{orderNo}/assign [post]
func (a *API) AdminAssign(c *gin.Context) {
	var req dto.AssignOrderReq
	if !bindJSON(c, &req) {
		return
	}
	if err := a.Admin.Assign(c.Request.Context(), middleware.CurrentAdminID(c), c.Param("orderNo"), req.ProviderID); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"assigned": true, "provider_id": req.ProviderID})
}

// AdminRefund 发起退款。amount 为 0 表示全额退剩余部分。
// @Summary      发起退款
// @Description  仅 admin 角色可调用。amount 为 0 表示全额退剩余部分；全额退款且未履约时订单自动关闭并释放时段
// @Tags         运营后台
// @Accept       json
// @Produce      json
// @Param        body  body  dto.RefundReq  true  "退款参数"
// @Success      200  {object}  response.Body
// @Failure      403  {object}  response.Body  "1003 需要超管权限"
// @Security     AdminToken
// @Router       /api/admin/refunds [post]
func (a *API) AdminRefund(c *gin.Context) {
	var req dto.RefundReq
	if !bindJSON(c, &req) {
		return
	}
	rf, err := a.Refund.Refund(c.Request.Context(), req.OrderNo, req.Amount, req.Reason,
		middleware.CurrentAdminID(c), "admin")
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{
		"out_refund_no": rf.OutRefundNo,
		"amount":        rf.Amount,
		"status":        rf.Status,
	})
}

// CancelOrder 用户取消订单。2 小时内返回需审核标志，不直接取消。
// @Summary      用户取消订单
// @Description  距服务开始 2 小时外免费取消并退款；2 小时内返回 need_review；已开始不可取消
// @Tags         订单
// @Accept       json
// @Produce      json
// @Param        orderNo  path  string           true   "订单号"
// @Param        body     body  dto.CancelOrderReq  false  "取消原因"
// @Success      200  {object}  response.Body  "data 为 service.CancelResult"
// @Security     UserID
// @Router       /api/v1/orders/{orderNo}/cancel [post]
func (a *API) CancelOrder(c *gin.Context) {
	var req dto.CancelOrderReq
	if c.Request.ContentLength > 0 && !bindJSON(c, &req) {
		return
	}
	res, err := a.Order.Cancel(c.Request.Context(), middleware.CurrentUserID(c), c.Param("orderNo"), req.Reason)
	if err != nil {
		response.Fail(c, err)
		return
	}

	// 免费取消且已付款：立即原路退款
	if res.Canceled && res.RefundDue > 0 {
		rf, rerr := a.Refund.Refund(c.Request.Context(), c.Param("orderNo"), res.RefundDue,
			"用户取消订单", middleware.CurrentUserID(c), "user")
		if rerr != nil {
			// 取消已成功，退款失败要如实告知，不能假装成功
			response.OK(c, gin.H{
				"canceled":    true,
				"refund_due":  res.RefundDue,
				"refund_ok":   false,
				"refund_hint": "订单已取消，退款处理失败，请联系客服",
			})
			return
		}
		response.OK(c, gin.H{
			"canceled":      true,
			"refund_due":    res.RefundDue,
			"refund_ok":     true,
			"out_refund_no": rf.OutRefundNo,
			"hint":          "退款将原路退回，1-3 个工作日到账",
		})
		return
	}

	response.OK(c, res)
}
