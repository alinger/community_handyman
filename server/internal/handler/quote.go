package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/alinger/community-handyman/server/internal/middleware"
	"github.com/alinger/community-handyman/server/pkg/response"
)

// AdminCreateQuote 运营给陪诊订单报价。
func (a *API) AdminCreateQuote(c *gin.Context) {
	var req struct {
		Amount      int64  `json:"amount" binding:"required,min=1"`
		DurationMin int    `json:"duration_min" binding:"min=0,max=600"`
		Note        string `json:"note" binding:"max=500"`
	}
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
func (a *API) RejectQuote(c *gin.Context) {
	var req struct {
		Reason string `json:"reason" binding:"max=200"`
	}
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
func (a *API) AdminRedeem(c *gin.Context) {
	var req struct {
		Date   string `json:"date" binding:"required"`
		Remark string `json:"remark" binding:"max=200"`
	}
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
func (a *API) MonthlyDetail(c *gin.Context) {
	sum, err := a.Monthly.Detail(c.Request.Context(), c.Param("orderNo"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, sum)
}
