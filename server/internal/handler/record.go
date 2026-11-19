package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/alinger/community-handyman/server/internal/errs"
	"github.com/alinger/community-handyman/server/internal/middleware"
	"github.com/alinger/community-handyman/server/internal/model"
	"github.com/alinger/community-handyman/server/internal/service"
	"github.com/alinger/community-handyman/server/pkg/response"
)

// maxUploadBytes 是单次上传的硬上限，比业务上限多留 1MB 给表单字段。
const maxUploadBytes = 80<<20 + 1<<20

// UploadRecord 上传服务记录（multipart）。
// 记录一经写入不可删除，服务端不暴露删除接口。
func (a *API) UploadRecord(c *gin.Context) {
	orderNo := c.Param("orderNo")

	// 全局 BodyLimit 放过了 multipart，这里按上传上限再包一层
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadBytes)
	if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
		response.Fail(c, errs.BizMsg(errs.CodeInvalidParam, "上传报文解析失败或超出大小限制"))
		return
	}

	mediaType := c.PostForm("media_type")
	durationSec := atoiDefault(c.PostForm("duration_sec"), 0)

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

	rec, err := a.Record.Upload(c.Request.Context(), middleware.CurrentUserID(c),
		orderNo, mediaType, fh.Filename, f, fh.Size, durationSec)
	if err != nil {
		response.Fail(c, err)
		return
	}

	out := service.ToRecordResps([]model.ServiceRecord{*rec}, orderNo)
	response.Created(c, out[0])
}

// ListRecords 查询订单服务记录。
func (a *API) ListRecords(c *gin.Context) {
	orderNo := c.Param("orderNo")
	list, err := a.Record.List(c.Request.Context(), middleware.CurrentUserID(c), orderNo)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, service.ToRecordResps(list, orderNo))
}

// DownloadRecord 流式下载记录文件。
// 不暴露静态目录：文件含病历与门禁照片，必须带鉴权并校验订单归属。
func (a *API) DownloadRecord(c *gin.Context) {
	orderNo := c.Param("orderNo")
	recordID := uint64(atoiDefault(c.Param("recordID"), 0))
	if recordID == 0 {
		response.Fail(c, errs.BizMsg(errs.CodeInvalidParam, "记录 ID 不合法"))
		return
	}

	f, _, rec, err := a.Record.Open(c.Request.Context(), middleware.CurrentUserID(c), orderNo, recordID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	defer f.Close()

	// ServeContent 支持 Range，视频可以拖动进度
	http.ServeContent(c.Writer, c.Request, rec.ObjectKey, rec.CreatedAt, f)
}
