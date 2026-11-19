package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/alinger/community-handyman/server/internal/dto"
	"github.com/alinger/community-handyman/server/internal/middleware"
	"github.com/alinger/community-handyman/server/pkg/response"
)

// OpenOrders 可接单池。
// @Summary      可接单池
// @Description  已支付且未派单的订单
// @Tags         服务方
// @Produce      json
// @Param        service_type  query  string  false  "garbage / dogwalk / accompany"
// @Param        page          query  int     false  "页码"
// @Param        size          query  int     false  "每页条数"
// @Success      200  {object}  response.Body
// @Security     UserID
// @Router       /api/v1/provider/orders/open [get]
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
// @Summary      服务方名下订单
// @Tags         服务方
// @Produce      json
// @Param        tab   query  string  false  "pending / processing / completed"
// @Param        page  query  int     false  "页码"
// @Param        size  query  int     false  "每页条数"
// @Success      200  {object}  response.Body
// @Security     UserID
// @Router       /api/v1/provider/orders [get]
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
// @Summary      接单
// @Description  条件更新，先到先得；已被接单返回 2003
// @Tags         服务方
// @Produce      json
// @Param        orderNo  path  string  true  "订单号"
// @Success      200  {object}  response.Body
// @Security     UserID
// @Router       /api/v1/provider/orders/{orderNo}/accept [post]
func (a *API) AcceptOrder(c *gin.Context) {
	if err := a.Provider.Accept(c.Request.Context(), middleware.CurrentUserID(c), c.Param("orderNo")); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"accepted": true})
}

// RejectOrder 拒单。
// @Summary      拒单
// @Tags         服务方
// @Accept       json
// @Produce      json
// @Param        orderNo  path  string        true   "订单号"
// @Param        body     body  dto.RejectOrderReq  false  "拒单原因"
// @Success      200  {object}  response.Body
// @Security     UserID
// @Router       /api/v1/provider/orders/{orderNo}/reject [post]
func (a *API) RejectOrder(c *gin.Context) {
	var req dto.RejectOrderReq
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
// @Summary      推进节点
// @Description  目标状态必须满足状态机白名单，非法跳转返回 2003
// @Tags         服务方
// @Accept       json
// @Produce      json
// @Param        orderNo  path  string         true  "订单号"
// @Param        body     body  dto.AdvanceOrderReq  true  "目标节点"
// @Success      200  {object}  response.Body
// @Security     UserID
// @Router       /api/v1/provider/orders/{orderNo}/advance [post]
func (a *API) AdvanceOrder(c *gin.Context) {
	var req dto.AdvanceOrderReq
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
// @Summary      确认服务完成
// @Tags         订单
// @Produce      json
// @Param        orderNo  path  string  true  "订单号"
// @Success      200  {object}  response.Body
// @Security     UserID
// @Router       /api/v1/orders/{orderNo}/confirm [post]
func (a *API) ConfirmOrder(c *gin.Context) {
	if err := a.Order.Confirm(c.Request.Context(), middleware.CurrentUserID(c), c.Param("orderNo")); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"completed": true})
}
