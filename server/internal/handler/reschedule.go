package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/alinger/community-handyman/server/internal/dto"
	"github.com/alinger/community-handyman/server/internal/middleware"
	"github.com/alinger/community-handyman/server/pkg/response"
)

// RescheduleOrder 住户修改服务时间，立即生效。
// @Summary      修改服务时间
// @Description  待支付/待接单/已接单可改；自动换时段库存并通知已接单的服务方
// @Tags         订单
// @Accept       json
// @Produce      json
// @Param        orderNo  path  string               true  "订单号"
// @Param        body     body  dto.RescheduleReq    true  "新日期与新时段"
// @Success      200      {object}  response.Body  "data 为 dto.RescheduleInfo"
// @Failure      200      {object}  response.Body  "2002 时段已满 / 2003 状态不允许"
// @Security     UserID
// @Router       /api/v1/orders/{orderNo}/reschedule [post]
func (a *API) RescheduleOrder(c *gin.Context) {
	var req dto.RescheduleReq
	if !bindJSON(c, &req) {
		return
	}
	res, err := a.Reschedule.RescheduleByUser(c.Request.Context(),
		middleware.CurrentUserID(c), c.Param("orderNo"), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

// RespondReschedule 住户同意或拒绝服务方的改期申请。
// @Summary      处理改期申请
// @Description  同意则立即换时段库存并改期，拒绝则保留原时间
// @Tags         订单
// @Accept       json
// @Produce      json
// @Param        orderNo  path  string                    true  "订单号"
// @Param        body     body  dto.RespondRescheduleReq  true  "是否同意"
// @Success      200      {object}  response.Body  "data 为 dto.RescheduleInfo"
// @Security     UserID
// @Router       /api/v1/orders/{orderNo}/reschedule/respond [post]
func (a *API) RespondReschedule(c *gin.Context) {
	var req dto.RespondRescheduleReq
	if !bindJSON(c, &req) {
		return
	}
	res, err := a.Reschedule.Respond(c.Request.Context(), middleware.CurrentUserID(c),
		c.Param("orderNo"), req.Approve, req.Reason)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

// ProviderReschedule 服务方申请改期，需住户确认后才生效。
// @Summary      申请改期
// @Description  仅已受理订单可申请；提交后等住户确认，确认前仍按原时间服务
// @Tags         服务方
// @Accept       json
// @Produce      json
// @Param        orderNo  path  string             true  "订单号"
// @Param        body     body  dto.RescheduleReq  true  "新日期与新时段"
// @Success      200      {object}  response.Body  "data 为 dto.RescheduleInfo"
// @Failure      200      {object}  response.Body  "2003 状态不允许或有待确认申请"
// @Security     UserID
// @Router       /api/v1/provider/orders/{orderNo}/reschedule [post]
func (a *API) ProviderReschedule(c *gin.Context) {
	var req dto.RescheduleReq
	if !bindJSON(c, &req) {
		return
	}
	res, err := a.Reschedule.RequestByProvider(c.Request.Context(),
		middleware.CurrentUserID(c), c.Param("orderNo"), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}
