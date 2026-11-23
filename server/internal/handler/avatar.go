package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/alinger/community-handyman/server/internal/errs"
	"github.com/alinger/community-handyman/server/internal/middleware"
	"github.com/alinger/community-handyman/server/pkg/response"
)

// maxAvatarUpload 是头像上传报文的上限，比服务记录小得多。
const maxAvatarUpload = 6 << 20

// UploadAvatar 上传头像（multipart）。
// @Summary      上传头像
// @Description  微信 chooseAvatar 回调给的是临时文件路径，前端需把它上传到这里落地。
// @Description  支持 jpg / png / webp / gif，≤5MB；成功后返回可直接用于 image 的地址。
// @Tags         个人信息
// @Accept       multipart/form-data
// @Produce      json
// @Param        file  formData  file  true  "头像文件"
// @Success      200  {object}  response.Body  "data 为 {avatar: string}"
// @Failure      200  {object}  response.Body  "1001 文件类型或大小不合规"
// @Security     UserID
// @Router       /api/v1/me/avatar [post]
func (a *API) UploadAvatar(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxAvatarUpload)
	if err := c.Request.ParseMultipartForm(4 << 20); err != nil {
		response.Fail(c, errs.BizMsg(errs.CodeInvalidParam, "上传报文解析失败或超出大小限制"))
		return
	}

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

	url, err := a.Avatar.Save(c.Request.Context(), middleware.CurrentUserID(c),
		fh.Filename, f, fh.Size)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"avatar": url})
}

// ServeAvatar 读取头像。
// @Summary      读取头像
// @Description  公开接口：小程序 image 标签无法携带 Authorization 头，故不鉴权。
// @Description  文件名由服务端生成且不可猜，不暴露用户目录结构。
// @Tags         个人信息
// @Produce      image/png
// @Param        id    path  int     true  "用户 ID"
// @Param        name  path  string  true  "文件名"
// @Success      200   {file}  file
// @Failure      404   {object}  response.Body  "文件不存在"
// @Router       /api/v1/users/{id}/avatar/{name} [get]
func (a *API) ServeAvatar(c *gin.Context) {
	uid := uint64(atoiDefault(c.Param("id"), 0))
	name := c.Param("name")

	f, _, err := a.Avatar.Open(c.Request.Context(), uid, name)
	if err != nil {
		c.JSON(http.StatusNotFound, response.Body{
			Code:      errs.CodeNotFound,
			Message:   "头像不存在",
			RequestID: c.GetString(response.CtxRequestID),
		})
		return
	}
	defer f.Close()

	// 文件名带毫秒时间戳，内容不会变，可以放心长缓存
	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeContent(c.Writer, c.Request, name, time.Time{}, f)
}
