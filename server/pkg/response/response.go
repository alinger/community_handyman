// Package response 统一 HTTP 响应结构。
// 业务结果看 code，HTTP 状态码只表达协议层结果。
package response

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/alinger/community-handyman/server/internal/errs"
)

// CtxRequestID 是中间件写入 context 的 trace id 键。
const CtxRequestID = "request_id"

// Body 是统一响应体。
type Body struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
	Data      any    `json:"data,omitempty"`
}

// PageData 是分页响应的数据部分。
type PageData struct {
	List  any   `json:"list"`
	Total int64 `json:"total"`
	Page  int   `json:"page"`
	Size  int   `json:"size"`
}

// OK 返回成功。
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Body{
		Code:      errs.CodeOK,
		Message:   errs.Message(errs.CodeOK),
		RequestID: requestID(c),
		Data:      data,
	})
}

// Created 返回创建成功，HTTP 201，业务码仍为 0。
func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, Body{
		Code:      errs.CodeOK,
		Message:   errs.Message(errs.CodeOK),
		RequestID: requestID(c),
		Data:      data,
	})
}

// Page 返回分页结果。
func Page(c *gin.Context, list any, total int64, page, size int) {
	OK(c, PageData{List: list, Total: total, Page: page, Size: size})
}

// Fail 按错误类型输出：业务错误原样返回，未知错误统一转 500 并记日志。
// 底层错误串不返回给客户端，避免泄露表结构与内部路径。
func Fail(c *gin.Context, err error) {
	if be, ok := errs.As(err); ok {
		if be.Err != nil {
			slog.Error("业务错误", "code", be.Code, "msg", be.Msg, "cause", be.Err,
				"path", c.FullPath(), "request_id", requestID(c))
		}
		c.JSON(errs.HTTPStatus(be.Code), Body{
			Code:      be.Code,
			Message:   be.Msg,
			RequestID: requestID(c),
		})
		return
	}

	slog.Error("未处理异常", "err", err, "path", c.FullPath(), "request_id", requestID(c))
	c.JSON(http.StatusInternalServerError, Body{
		Code:      errs.CodeInternal,
		Message:   errs.Message(errs.CodeInternal),
		RequestID: requestID(c),
	})
}

func requestID(c *gin.Context) string {
	if v, ok := c.Get(CtxRequestID); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
