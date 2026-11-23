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
	uid := middleware.CurrentUserID(c)
	for i := range list {
		resp := a.Order.ToOrderResp(&list[i], nil)
		// 服务方名下订单按时间窗下发门禁信息；接单池不经过这里，所以不会提前泄露
		a.Order.AttachAccess(&resp, &list[i], uid)
		out = append(out, resp)
	}
	// 让服务方也看到「已提交、等住户确认」，而不是点了改期才被 2003 拦住
	if a.Reschedule != nil {
		a.Reschedule.AttachPendingTo(c.Request.Context(), out)
		// 住户直接改期会改掉卡片上的日期却不通知服务方，靠这条提示把他叫醒
		a.Reschedule.AttachProviderNoticeTo(c.Request.Context(), out)
	}
	// 月卡订单状态长期停在「已受理」，卡片上必须另外告诉服务方今天打过卡没有
	if a.Monthly != nil {
		a.Monthly.AttachProgressTo(c.Request.Context(), list, out)
	}
	response.Page(c, out, total, page, size)
}

// ProviderMonthlyRedeem 服务方打卡「今日已服务」。
// @Summary      月卡今日打卡
// @Description  包月代倒垃圾按天核销；同一天重复打卡返回 2003，不在服务期内返回 1001
// @Tags         服务方
// @Produce      json
// @Param        orderNo  path  string  true  "订单号"
// @Success      200  {object}  response.Body  "data 为 dto.MonthlyProgress"
// @Security     UserID
// @Router       /api/v1/provider/orders/{orderNo}/monthly/redeem [post]
func (a *API) ProviderMonthlyRedeem(c *gin.Context) {
	p, err := a.Monthly.ProviderRedeem(c.Request.Context(),
		middleware.CurrentUserID(c), c.Param("orderNo"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, p)
}

// ProviderMonthlyFinish 服务方收尾今天这一趟（「完成服务」）。
// @Summary      月卡本趟完成
// @Description  打卡之后才能点；同一天重复完成返回 2003，未打卡返回 2003
// @Tags         服务方
// @Produce      json
// @Param        orderNo  path  string  true  "订单号"
// @Success      200  {object}  response.Body  "data 为 dto.MonthlyProgress"
// @Security     UserID
// @Router       /api/v1/provider/orders/{orderNo}/monthly/finish [post]
func (a *API) ProviderMonthlyFinish(c *gin.Context) {
	p, err := a.Monthly.ProviderFinish(c.Request.Context(),
		middleware.CurrentUserID(c), c.Param("orderNo"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, p)
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
	// 能推进到出发/开始服务，说明新时间已经核对过，改期提示不必再挂着
	if a.Reschedule != nil {
		a.Reschedule.AckProviderNoticeQuiet(c.Request.Context(), c.Param("orderNo"))
	}
	response.OK(c, gin.H{"node": req.Node})
}

// AckProviderReschedule 服务方确认已知悉住户的改期。
// @Summary      确认知悉住户改期
// @Tags         服务方
// @Produce      json
// @Param        orderNo  path  string  true  "订单号"
// @Success      200  {object}  response.Body
// @Security     UserID
// @Router       /api/v1/provider/orders/{orderNo}/reschedule/ack [post]
func (a *API) AckProviderReschedule(c *gin.Context) {
	if err := a.Reschedule.AckProviderNotice(c.Request.Context(),
		middleware.CurrentUserID(c), c.Param("orderNo")); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"ack": true})
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

// ProviderIncome 服务方收入。
// @Summary      我的收入
// @Description  已完成订单自动入账后的累计金额与最近明细
// @Tags         服务方
// @Produce      json
// @Success      200  {object}  response.Body  "data 为 dto.ProviderIncomeResp"
// @Security     UserID
// @Router       /api/v1/provider/income [get]
func (a *API) ProviderIncome(c *gin.Context) {
	res, err := a.Provider.Income(c.Request.Context(), middleware.CurrentUserID(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}
