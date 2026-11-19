package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/alinger/community-handyman/server/internal/dto"
	"github.com/alinger/community-handyman/server/internal/middleware"
	"github.com/alinger/community-handyman/server/pkg/response"
)

// OpenOrders 可接单池。
func (a *API) OpenOrders(c *gin.Context) {
	page := atoiDefault(c.Query("page"), 1)
	size := atoiDefault(c.Query("size"), 10)
	list, total, err := a.Provider.OpenOrders(c.Request.Context(), c.Query("service_type"), page, size)
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

// MyOrders 服务方名下订单。
func (a *API) MyOrders(c *gin.Context) {
	page := atoiDefault(c.Query("page"), 1)
	size := atoiDefault(c.Query("size"), 10)
	list, total, err := a.Provider.MyOrders(c.Request.Context(), middleware.CurrentUserID(c), c.Query("tab"), page, size)
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

// AcceptOrder 接单。
func (a *API) AcceptOrder(c *gin.Context) {
	if err := a.Provider.Accept(c.Request.Context(), middleware.CurrentUserID(c), c.Param("orderNo")); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"accepted": true})
}

// RejectOrder 拒单。
func (a *API) RejectOrder(c *gin.Context) {
	var req struct {
		Reason string `json:"reason" binding:"max=200"`
	}
	if !bindJSON(c, &req) {
		return
	}
	if err := a.Provider.Reject(c.Request.Context(), middleware.CurrentUserID(c), c.Param("orderNo"), req.Reason); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"rejected": true})
}

// AdvanceOrder 推进节点，target 取 on_the_way / in_service / pending_confirm。
func (a *API) AdvanceOrder(c *gin.Context) {
	var req struct {
		Node string `json:"node" binding:"required,max=32"`
	}
	if !bindJSON(c, &req) {
		return
	}
	if err := a.Provider.Advance(c.Request.Context(), middleware.CurrentUserID(c), c.Param("orderNo"), req.Node); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"node": req.Node})
}

// ConfirmOrder 用户确认服务完成。
func (a *API) ConfirmOrder(c *gin.Context) {
	if err := a.Order.Confirm(c.Request.Context(), middleware.CurrentUserID(c), c.Param("orderNo")); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"completed": true})
}
