package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/alinger/community-handyman/server/internal/errs"
	"github.com/alinger/community-handyman/server/internal/middleware"
	"github.com/alinger/community-handyman/server/internal/service"
	"github.com/alinger/community-handyman/server/pkg/response"
)

// maxCertUploadBytes 证件照上传报文上限（图片 8MB + 表单字段余量）。
const maxCertUploadBytes = 8<<20 + 1<<20

// ---------- 用户端：服务方资质申请 ----------

// MyProviderApplication 查当前用户的认证状态。
// @Summary      我的服务方认证状态
// @Description  data 为 null 表示从未申请；否则返回申请内容与审核结果
// @Tags         服务方认证
// @Produce      json
// @Success      200  {object}  response.Body
// @Security     UserID
// @Router       /api/v1/provider/application [get]
func (a *API) MyProviderApplication(c *gin.Context) {
	app, err := a.ProviderApp.MyApplication(c.Request.Context(), middleware.CurrentUserID(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	if app == nil {
		response.OK(c, nil)
		return
	}
	// 只暴露必要字段：身份证号脱敏，附件走单独鉴权接口
	response.OK(c, gin.H{
		"id":           app.ID,
		"real_name":    app.RealName,
		"phone":        app.Phone,
		"service_type": app.ServiceType,
		"experience":   app.Experience,
		"id_card_no":   maskIDCard(app.IDCardNo),
		"status":       string(app.Status),
		"status_label": app.Status.Label(),
		"review_note":  app.ReviewNote,
		"reviewed_at":  app.ReviewedAt,
		"created_at":   app.CreatedAt,
		"has_front":    app.IDCardFrontKey != "",
		"has_back":     app.IDCardBackKey != "",
		"has_health":   app.HealthCertKey != "",
	})
}

// SubmitProviderApplication 提交/重新提交认证申请。
// @Summary      提交服务方认证申请
// @Description  驳回后可修改重提；待审核与已通过状态不允许重复提交
// @Tags         服务方认证
// @Accept       json
// @Produce      json
// @Param        body  body  object  true  "real_name/phone/service_type/experience/id_card_no/附件 key"
// @Success      200  {object}  response.Body
// @Security     UserID
// @Router       /api/v1/provider/application [post]
func (a *API) SubmitProviderApplication(c *gin.Context) {
	var req struct {
		RealName       string `json:"real_name" binding:"required,max=32"`
		Phone          string `json:"phone" binding:"required,max=20"`
		ServiceType    string `json:"service_type" binding:"required,max=16"`
		Experience     string `json:"experience" binding:"required,max=500"`
		IDCardNo       string `json:"id_card_no" binding:"max=32"`
		IDCardFrontKey string `json:"id_card_front_key" binding:"max=255"`
		IDCardBackKey  string `json:"id_card_back_key" binding:"max=255"`
		HealthCertKey  string `json:"health_cert_key" binding:"max=255"`
	}
	if !bindJSON(c, &req) {
		return
	}
	app, err := a.ProviderApp.Submit(c.Request.Context(), middleware.CurrentUserID(c), service.ApplyInput{
		RealName:       req.RealName,
		Phone:          req.Phone,
		ServiceType:    req.ServiceType,
		Experience:     req.Experience,
		IDCardNo:       req.IDCardNo,
		IDCardFrontKey: req.IDCardFrontKey,
		IDCardBackKey:  req.IDCardBackKey,
		HealthCertKey:  req.HealthCertKey,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"id": app.ID, "status": string(app.Status), "status_label": app.Status.Label()})
}

// UploadProviderCert 上传证件照，返回 object key。
// @Summary      上传认证附件
// @Description  仅支持 jpg/png/webp 且 ≤8MB；返回的 key 在提交申请时回传
// @Tags         服务方认证
// @Accept       multipart/form-data
// @Produce      json
// @Param        kind  formData  string  true  "front / back / health"
// @Param        file  formData  file    true  "图片"
// @Success      200  {object}  response.Body  "data.key 为附件 key"
// @Security     UserID
// @Router       /api/v1/provider/application/cert [post]
func (a *API) UploadProviderCert(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxCertUploadBytes)
	if err := c.Request.ParseMultipartForm(8 << 20); err != nil {
		response.Fail(c, errs.BizMsg(errs.CodeInvalidParam, "上传报文解析失败或超出大小限制"))
		return
	}
	kind := c.PostForm("kind")
	fh, err := c.FormFile("file")
	if err != nil {
		response.Fail(c, errs.BizMsg(errs.CodeInvalidParam, "缺少文件字段 file"))
		return
	}
	f, err := fh.Open()
	if err != nil {
		response.Fail(c, errs.BizMsg(errs.CodeInvalidParam, "读取上传文件失败"))
		return
	}
	defer f.Close()

	key, err := a.ProviderApp.SaveCertImage(c.Request.Context(), middleware.CurrentUserID(c),
		kind, fh.Filename, f, fh.Size)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"key": key})
}

// ProviderCertFile 读取自己的认证附件。
// @Summary      查看认证附件
// @Tags         服务方认证
// @Produce      octet-stream
// @Param        kind  path  string  true  "front / back / health"
// @Security     UserID
// @Router       /api/v1/provider/application/cert/{kind} [get]
func (a *API) ProviderCertFile(c *gin.Context) {
	app, err := a.ProviderApp.MyApplication(c.Request.Context(), middleware.CurrentUserID(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	if app == nil {
		response.Fail(c, errs.Biz(errs.CodeNotFound))
		return
	}
	a.serveCert(c, app.ID, c.Param("kind"), middleware.CurrentUserID(c), false)
}

// ---------- 后台：服务方审核 ----------

// AdminListProviderApps 申请列表。
// @Summary      服务方申请列表
// @Tags         服务方审核
// @Produce      json
// @Param        status  query  string  false  "pending/approved/rejected，空为全部"
// @Param        page    query  int     false  "页码"
// @Param        size    query  int     false  "每页条数"
// @Success      200  {object}  response.Body
// @Security     AdminToken
// @Router       /api/admin/provider-applications [get]
func (a *API) AdminListProviderApps(c *gin.Context) {
	page := atoiDefault(c.Query("page"), 1)
	size := atoiDefault(c.Query("size"), 20)
	list, total, err := a.ProviderApp.List(c.Request.Context(), c.Query("status"), page, size)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"list": list, "total": total, "page": page, "size": size})
}

// AdminProviderAppStats 各状态数量。
// @Summary      服务方申请统计
// @Tags         服务方审核
// @Produce      json
// @Success      200  {object}  response.Body
// @Security     AdminToken
// @Router       /api/admin/provider-applications/stats [get]
func (a *API) AdminProviderAppStats(c *gin.Context) {
	m, err := a.ProviderApp.Stats(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, m)
}

// AdminProviderAppDetail 申请详情。
// @Summary      服务方申请详情
// @Tags         服务方审核
// @Produce      json
// @Param        id  path  int  true  "申请 ID"
// @Success      200  {object}  response.Body
// @Security     AdminToken
// @Router       /api/admin/provider-applications/{id} [get]
func (a *API) AdminProviderAppDetail(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	row, err := a.ProviderApp.Detail(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, row)
}

// AdminApproveProviderApp 审核通过。
// @Summary      通过服务方认证
// @Description  同事务内把用户 role 置为 provider；非 pending 状态返回 2003
// @Tags         服务方审核
// @Produce      json
// @Param        id  path  int  true  "申请 ID"
// @Success      200  {object}  response.Body
// @Security     AdminToken
// @Router       /api/admin/provider-applications/{id}/approve [post]
func (a *API) AdminApproveProviderApp(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	if err := a.ProviderApp.Approve(c.Request.Context(), id, middleware.CurrentAdminID(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"approved": true})
}

// AdminRejectProviderApp 驳回申请。
// @Summary      驳回服务方认证
// @Tags         服务方审核
// @Accept       json
// @Produce      json
// @Param        id    path  int     true  "申请 ID"
// @Param        body  body  object  true  "note：驳回原因"
// @Success      200  {object}  response.Body
// @Security     AdminToken
// @Router       /api/admin/provider-applications/{id}/reject [post]
func (a *API) AdminRejectProviderApp(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var req struct {
		Note string `json:"note" binding:"required,max=200"`
	}
	if !bindJSON(c, &req) {
		return
	}
	if err := a.ProviderApp.Reject(c.Request.Context(), id, middleware.CurrentAdminID(c), req.Note); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"rejected": true})
}

// AdminProviderCertFile 审核时查看附件。
// @Summary      查看申请附件
// @Tags         服务方审核
// @Produce      octet-stream
// @Param        id    path  int     true  "申请 ID"
// @Param        kind  path  string  true  "front / back / health"
// @Security     AdminToken
// @Router       /api/admin/provider-applications/{id}/cert/{kind} [get]
func (a *API) AdminProviderCertFile(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	a.serveCert(c, id, c.Param("kind"), 0, true)
}

// serveCert 统一输出附件内容，用户端与后台共用。
func (a *API) serveCert(c *gin.Context, appID uint64, kind string, requesterID uint64, isAdmin bool) {
	f, size, ct, err := a.ProviderApp.OpenCertImage(c.Request.Context(), appID, kind, requesterID, isAdmin)
	if err != nil {
		response.Fail(c, err)
		return
	}
	defer f.Close()
	c.Header("Content-Type", ct)
	c.Header("Content-Length", strconv.FormatInt(size, 10))
	// 证件照不进浏览器缓存，避免共用电脑时被下一个人翻到
	c.Header("Cache-Control", "no-store")
	http.ServeContent(c.Writer, c.Request, "", time.Time{}, f)
}

// parseIDParam 解析路径上的 :id，非法时直接写响应并返回 false。
func parseIDParam(c *gin.Context) (uint64, bool) {
	id := uint64(atoiDefault(c.Param("id"), 0))
	if id == 0 {
		response.Fail(c, errs.BizMsg(errs.CodeInvalidParam, "ID 不合法"))
		return 0, false
	}
	return id, true
}

// maskIDCard 身份证号脱敏：保留前 6 后 4，中间打码。
func maskIDCard(s string) string {
	if len(s) < 11 {
		return s
	}
	return s[:6] + "********" + s[len(s)-4:]
}
