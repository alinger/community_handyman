package handler

import (
	"encoding/json"

	"github.com/gin-gonic/gin"

	"github.com/alinger/community-handyman/server/internal/dto"
	"github.com/alinger/community-handyman/server/internal/errs"
	"github.com/alinger/community-handyman/server/internal/middleware"
	"github.com/alinger/community-handyman/server/internal/model"
	"github.com/alinger/community-handyman/server/internal/service"
	"github.com/alinger/community-handyman/server/pkg/response"
)

// AdminLogin 后台登录。
func (a *API) AdminLogin(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required,max=32"`
		Password string `json:"password" binding:"required,max=64"`
	}
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
func (a *API) AdminListAreas(c *gin.Context) {
	list, err := a.Admin.ListAreas(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, list)
}

// AdminCreateArea 新增服务范围。
func (a *API) AdminCreateArea(c *gin.Context) {
	var req struct {
		Community   string `json:"community" binding:"required,max=64"`
		Lane        string `json:"lane" binding:"required,max=128"`
		HouseFrom   int    `json:"house_from" binding:"min=0"`
		HouseTo     int    `json:"house_to" binding:"min=0"`
		HouseParity string `json:"house_parity" binding:"omitempty,oneof=all odd even"`
	}
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
func (a *API) AdminSetAreaStatus(c *gin.Context) {
	id := uint64(atoiDefault(c.Param("id"), 0))
	if id == 0 {
		response.Fail(c, errs.BizMsg(errs.CodeInvalidParam, "ID 不合法"))
		return
	}
	var req struct {
		Status int8 `json:"status" binding:"oneof=0 1"`
	}
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
func (a *API) AdminListSlots(c *gin.Context) {
	list, err := a.Admin.ListSlots(c.Request.Context(), c.Query("date"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, list)
}

// AdminSetSlots 批量设置时段可接单量。
func (a *API) AdminSetSlots(c *gin.Context) {
	var req struct {
		Date  string                   `json:"date" binding:"required"`
		Items []service.SlotTotalInput `json:"items" binding:"required,min=1,dive"`
	}
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
func (a *API) AdminListSkus(c *gin.Context) {
	list, err := a.Admin.ListSkus(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, list)
}

// AdminUpdateSku 改价格与计价规则（含加价日期）。
func (a *API) AdminUpdateSku(c *gin.Context) {
	id := uint64(atoiDefault(c.Param("id"), 0))
	if id == 0 {
		response.Fail(c, errs.BizMsg(errs.CodeInvalidParam, "ID 不合法"))
		return
	}
	var req struct {
		BasePrice int64           `json:"base_price" binding:"min=0"`
		Rules     json.RawMessage `json:"rules"`
	}
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
	response.Page(c, out, total, page, size)
}

// AdminAssign 派单或改派。
func (a *API) AdminAssign(c *gin.Context) {
	var req struct {
		ProviderID uint64 `json:"provider_id" binding:"required"`
	}
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
func (a *API) AdminRefund(c *gin.Context) {
	var req struct {
		OrderNo string `json:"order_no" binding:"required,max=32"`
		Amount  int64  `json:"amount" binding:"min=0"`
		Reason  string `json:"reason" binding:"required,max=200"`
	}
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
func (a *API) CancelOrder(c *gin.Context) {
	var req struct {
		Reason string `json:"reason" binding:"max=200"`
	}
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
