package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/alinger/community-handyman/server/internal/dto"
	"github.com/alinger/community-handyman/server/internal/middleware"
	"github.com/alinger/community-handyman/server/pkg/response"
)

// AdminCreateQuote 运营给陪诊订单报价。
// @Summary      陪诊报价
// @Description  写入报价单并把订单金额改为报价额；用户已确认后不可再改
// @Tags         运营后台
// @Accept       json
// @Produce      json
// @Param        orderNo  path  string            true  "订单号"
// @Param        body     body  object  true  "请求体：amount(int64,单位分)、duration_min(int)、note(string)"
// @Success      201  {object}  response.Body
// @Security     AdminToken
// @Router       /api/admin/orders/{orderNo}/quote [post]
func (a *API) AdminCreateQuote(c *gin.Context) {
	var req dto.CreateQuoteReq
	if !bindJSON(c, &req) {
		return
	}
	q, err := a.Quote.Create(c.Request.Context(), middleware.CurrentAdminID(c),
		c.Param("orderNo"), req.Amount, req.DurationMin, req.Note)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Created(c, gin.H{
		"quote_id":     q.ID,
		"amount":       q.Amount,
		"duration_min": q.DurationMin,
		"hint":         "报价已发送，等待用户确认支付",
	})
}

// RejectQuote 用户拒绝陪诊报价，订单关闭并释放时段。
// @Summary      拒绝陪诊报价
// @Description  订单关闭并释放时段
// @Tags         订单
// @Accept       json
// @Produce      json
// @Param        orderNo  path  string            true   "订单号"
// @Param        body     body  object  false  "请求体：reason(string)"
// @Success      200  {object}  response.Body
// @Security     UserID
// @Router       /api/v1/orders/{orderNo}/quote/reject [post]
func (a *API) RejectQuote(c *gin.Context) {
	var req dto.RejectQuoteReq
	if c.Request.ContentLength > 0 && !bindJSON(c, &req) {
		return
	}
	if err := a.Quote.Reject(c.Request.Context(), middleware.CurrentUserID(c),
		c.Param("orderNo"), req.Reason); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"rejected": true, "hint": "已拒绝报价，订单已关闭"})
}

// AdminRedeem 包月核销某一天的服务。
// @Summary      包月核销
// @Description  同一订单同一日期只能核销一次；首日下单时已占位，核销不再重复占
// @Tags         运营后台
// @Accept       json
// @Produce      json
// @Param        orderNo  path  string         true  "订单号"
// @Param        body     body  object  true  "请求体：date(string,2006-01-02)、remark(string)"
// @Success      201  {object}  response.Body
// @Failure      200  {object}  response.Body  "2003 重复核销 / 2002 当日时段已满"
// @Security     AdminToken
// @Router       /api/admin/orders/{orderNo}/redeem [post]
func (a *API) AdminRedeem(c *gin.Context) {
	var req dto.RedeemReq
	if !bindJSON(c, &req) {
		return
	}
	m, err := a.Monthly.Redeem(c.Request.Context(), middleware.CurrentAdminID(c),
		c.Param("orderNo"), req.Date, req.Remark)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Created(c, gin.H{
		"order_id":     m.OrderID,
		"service_date": m.ServiceDate.String(),
		"hint":         "核销成功，该日时段已占用",
	})
}

// MonthlyDetail 包月使用情况。
// @Summary      包月使用情况
// @Description  含服务期、已用/剩余次数与核销明细
// @Tags         订单
// @Produce      json
// @Param        orderNo  path  string  true  "订单号"
// @Success      200  {object}  response.Body  "data 为 service.MonthlySummary"
// @Security     UserID
// @Router       /api/v1/orders/{orderNo}/monthly [get]
func (a *API) MonthlyDetail(c *gin.Context) {
	sum, err := a.Monthly.Detail(c.Request.Context(), c.Param("orderNo"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, sum)
}
