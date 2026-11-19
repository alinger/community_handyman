package handler

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/alinger/community-handyman/server/internal/dto"
	"github.com/alinger/community-handyman/server/internal/errs"
	"github.com/alinger/community-handyman/server/pkg/response"
)

// adminRecordResp 后台记录响应，URL 指向 admin 文件通道。
type adminRecordResp struct {
	ID           uint64    `json:"id"`
	MediaType    string    `json:"media_type"`
	Size         int64     `json:"size"`
	DurationSec  int       `json:"duration_sec"`
	UploaderRole string    `json:"uploader_role"`
	CreatedAt    time.Time `json:"created_at"`
	URL          string    `json:"url"`
}

// AdminListPayments 支付流水分页。
// @Summary      支付流水列表
// @Tags         运营后台
// @Produce      json
// @Param        order_no  query  string  false  "订单号模糊搜索"
// @Param        status    query  string  false  "success / closed / refunded"
// @Param        page      query  int     false  "页码"
// @Param        size      query  int     false  "每页条数"
// @Success      200  {object}  response.Body
// @Security     AdminToken
// @Router       /api/admin/payments [get]
func (a *API) AdminListPayments(c *gin.Context) {
	page := atoiDefault(c.Query("page"), 1)
	size := atoiDefault(c.Query("size"), 10)
	list, total, err := a.Admin.ListPayments(c.Request.Context(),
		c.Query("order_no"), c.Query("status"), page, size)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Page(c, list, total, page, size)
}

// AdminListRefunds 退款单分页。
// @Summary      退款单列表
// @Description  含发起人 operator_id 与退款状态，用于跟踪退款结果
// @Tags         运营后台
// @Produce      json
// @Param        order_no  query  string  false  "订单号模糊搜索"
// @Param        status    query  string  false  "pending / success / failed"
// @Param        page      query  int     false  "页码"
// @Param        size      query  int     false  "每页条数"
// @Success      200  {object}  response.Body
// @Security     AdminToken
// @Router       /api/admin/refunds [get]
func (a *API) AdminListRefunds(c *gin.Context) {
	page := atoiDefault(c.Query("page"), 1)
	size := atoiDefault(c.Query("size"), 10)
	list, total, err := a.Admin.ListRefunds(c.Request.Context(),
		c.Query("order_no"), c.Query("status"), page, size)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Page(c, list, total, page, size)
}

// AdminOrderEvents 订单时间线。
// @Summary      订单节点回放
// @Tags         运营后台
// @Produce      json
// @Param        orderNo  path  string  true  "订单号"
// @Success      200  {object}  response.Body
// @Security     AdminToken
// @Router       /api/admin/orders/{orderNo}/events [get]
func (a *API) AdminOrderEvents(c *gin.Context) {
	list, err := a.Admin.OrderEvents(c.Request.Context(), c.Param("orderNo"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, list)
}

// AdminOrderRecords 订单服务记录列表。
// @Summary      后台查看服务记录
// @Tags         运营后台
// @Produce      json
// @Param        orderNo  path  string  true  "订单号"
// @Success      200  {object}  response.Body
// @Security     AdminToken
// @Router       /api/admin/orders/{orderNo}/records [get]
func (a *API) AdminOrderRecords(c *gin.Context) {
	orderNo := c.Param("orderNo")
	list, err := a.Record.ListForAdmin(c.Request.Context(), orderNo)
	if err != nil {
		response.Fail(c, err)
		return
	}
	out := make([]adminRecordResp, 0, len(list))
	for _, r := range list {
		out = append(out, adminRecordResp{
			ID: r.ID, MediaType: r.MediaType, Size: r.Size, DurationSec: r.DurationSec,
			UploaderRole: r.UploaderRole, CreatedAt: r.CreatedAt,
			URL: fmt.Sprintf("/api/admin/records/%d/file", r.ID),
		})
	}
	response.OK(c, out)
}

// AdminRecordFile 流式读取服务记录文件。
// @Summary      后台下载服务记录文件
// @Description  支持 Range；图片浏览器可直接预览
// @Tags         运营后台
// @Produce      application/octet-stream
// @Param        id  path  int  true  "记录 ID"
// @Success      200  {file}  binary
// @Security     AdminToken
// @Router       /api/admin/records/{id}/file [get]
func (a *API) AdminRecordFile(c *gin.Context) {
	id := uint64(atoiDefault(c.Param("id"), 0))
	if id == 0 {
		response.Fail(c, errs.BizMsg(errs.CodeInvalidParam, "记录 ID 不合法"))
		return
	}
	f, _, rec, err := a.Record.OpenForAdmin(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	defer f.Close()
	http.ServeContent(c.Writer, c.Request, rec.ObjectKey, rec.CreatedAt, f)
}

// AdminStats 看板统计。
// @Summary      运营看板
// @Tags         运营后台
// @Produce      json
// @Success      200  {object}  response.Body
// @Security     AdminToken
// @Router       /api/admin/stats [get]
func (a *API) AdminStats(c *gin.Context) {
	st, err := a.Admin.Stats(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, st)
}

// AdminListUsers 用户分页。
// @Summary      用户/服务方列表
// @Tags         运营后台
// @Produce      json
// @Param        role     query  string  false  "user / provider"
// @Param        keyword  query  string  false  "昵称/openid/手机号模糊搜索"
// @Param        page     query  int     false  "页码"
// @Param        size     query  int     false  "每页条数"
// @Success      200  {object}  response.Body
// @Security     AdminToken
// @Router       /api/admin/users [get]
func (a *API) AdminListUsers(c *gin.Context) {
	page := atoiDefault(c.Query("page"), 1)
	size := atoiDefault(c.Query("size"), 10)
	list, total, err := a.Admin.ListUsers(c.Request.Context(),
		c.Query("role"), c.Query("keyword"), page, size)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Page(c, list, total, page, size)
}

// AdminSetUserRole 角色切换。
// @Summary      切换用户角色
// @Description  升为 provider 后可被派单；降回 user 后不再出现在派单下拉
// @Tags         运营后台
// @Accept       json
// @Produce      json
// @Param        id    path  int                true  "用户 ID"
// @Param        body  body  dto.SetUserRoleReq true  "目标角色"
// @Success      200  {object}  response.Body
// @Security     AdminToken
// @Router       /api/admin/users/{id}/role [patch]
func (a *API) AdminSetUserRole(c *gin.Context) {
	id := uint64(atoiDefault(c.Param("id"), 0))
	if id == 0 {
		response.Fail(c, errs.BizMsg(errs.CodeInvalidParam, "ID 不合法"))
		return
	}
	var req dto.SetUserRoleReq
	if !bindJSON(c, &req) {
		return
	}
	if err := a.Admin.SetUserRole(c.Request.Context(), id, req.Role); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"id": id, "role": req.Role})
}

// AdminSetUserStatus 封禁/解封。
// @Summary      封禁或解封用户
// @Description  封禁后小程序端无法登录
// @Tags         运营后台
// @Accept       json
// @Produce      json
// @Param        id    path  int                   true  "用户 ID"
// @Param        body  body  dto.SetUserStatusReq  true  "1 正常 / 0 封禁"
// @Success      200  {object}  response.Body
// @Security     AdminToken
// @Router       /api/admin/users/{id}/status [patch]
func (a *API) AdminSetUserStatus(c *gin.Context) {
	id := uint64(atoiDefault(c.Param("id"), 0))
	if id == 0 {
		response.Fail(c, errs.BizMsg(errs.CodeInvalidParam, "ID 不合法"))
		return
	}
	var req dto.SetUserStatusReq
	if !bindJSON(c, &req) {
		return
	}
	if err := a.Admin.SetUserStatus(c.Request.Context(), id, req.Status); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"id": id, "status": req.Status})
}

// AdminListRedemptions 月卡核销记录。
// @Summary      月卡核销列表
// @Tags         运营后台
// @Produce      json
// @Param        date  query  string  false  "服务日期 2006-01-02"
// @Param        page  query  int     false  "页码"
// @Param        size  query  int     false  "每页条数"
// @Success      200  {object}  response.Body
// @Security     AdminToken
// @Router       /api/admin/redemptions [get]
func (a *API) AdminListRedemptions(c *gin.Context) {
	page := atoiDefault(c.Query("page"), 1)
	size := atoiDefault(c.Query("size"), 10)
	list, total, err := a.Admin.ListRedemptions(c.Request.Context(), c.Query("date"), page, size)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Page(c, list, total, page, size)
}

// AdminUpdateArea 修改服务范围规则。
// @Summary      修改服务范围
// @Tags         运营后台
// @Accept       json
// @Produce      json
// @Param        id    path  int                true  "范围 ID"
// @Param        body  body  dto.UpdateAreaReq  true  "范围规则"
// @Success      200  {object}  response.Body
// @Security     AdminToken
// @Router       /api/admin/areas/{id} [put]
func (a *API) AdminUpdateArea(c *gin.Context) {
	id := uint64(atoiDefault(c.Param("id"), 0))
	if id == 0 {
		response.Fail(c, errs.BizMsg(errs.CodeInvalidParam, "ID 不合法"))
		return
	}
	var req dto.UpdateAreaReq
	if !bindJSON(c, &req) {
		return
	}
	if err := a.Admin.UpdateArea(c.Request.Context(), id, req); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"id": id})
}

// AdminCopySlots 时段模板复制。
// @Summary      复制某天时段配置到多个日期
// @Description  覆盖目标日期已有时段 total；源日期无配置时报错
// @Tags         运营后台
// @Accept       json
// @Produce      json
// @Param        body  body  dto.CopySlotsReq  true  "源日期与目标日期列表"
// @Success      200  {object}  response.Body  "data.applied 为写入的时段条数"
// @Security     AdminToken
// @Router       /api/admin/slots/copy [post]
func (a *API) AdminCopySlots(c *gin.Context) {
	var req dto.CopySlotsReq
	if !bindJSON(c, &req) {
		return
	}
	applied, err := a.Admin.CopySlots(c.Request.Context(), req.SourceDate, req.TargetDates)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"applied": applied})
}

// AdminExportOrders 订单 CSV 导出。
// @Summary      订单导出 CSV
// @Description  与订单列表同筛选条件；带 BOM，Excel 直接打开不乱码；上限 5000 行
// @Tags         运营后台
// @Produce      text/csv
// @Param        status        query  string  false  "订单状态"
// @Param        service_type  query  string  false  "服务类型"
// @Param        keyword       query  string  false  "订单号模糊搜索"
// @Success      200  {file}  file
// @Security     AdminToken
// @Router       /api/admin/export/orders [get]
func (a *API) AdminExportOrders(c *gin.Context) {
	const exportCap = 5000
	list, _, err := a.Admin.ListOrders(c.Request.Context(),
		c.Query("status"), c.Query("service_type"), c.Query("keyword"), 1, exportCap)
	if err != nil {
		response.Fail(c, err)
		return
	}

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition",
		fmt.Sprintf("attachment; filename=orders_%s.csv", time.Now().Format("20060102_150405")))
	// BOM：Excel 打开 UTF-8 CSV 不乱码
	c.Writer.WriteString("\xEF\xBB\xBF")
	w := csv.NewWriter(c.Writer)
	_ = w.Write([]string{"订单号", "服务类型", "计划", "服务日期", "时段", "状态",
		"总价(分)", "已付(分)", "已退(分)", "服务方ID", "创建时间", "完成时间", "取消原因"})
	for _, o := range list {
		finished := ""
		if o.FinishedAt != nil {
			finished = o.FinishedAt.Format("2006-01-02 15:04:05")
		}
		provider := ""
		if o.ProviderID != nil {
			provider = strconv.FormatUint(*o.ProviderID, 10)
		}
		_ = w.Write([]string{
			o.OrderNo, string(o.ServiceType), string(o.Plan),
			o.ServiceDate.Time().Format("2006-01-02"), o.SlotCode, string(o.Status),
			strconv.FormatInt(o.TotalAmount, 10), strconv.FormatInt(o.PaidAmount, 10),
			strconv.FormatInt(o.RefundedAmount, 10), provider,
			o.CreatedAt.Format("2006-01-02 15:04:05"), finished, o.CancelReason,
		})
	}
	w.Flush()
}
