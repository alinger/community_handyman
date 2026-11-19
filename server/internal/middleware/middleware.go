// Package middleware 提供 Gin 中间件。
// 装配顺序有语义要求，见 Usage：RequestID 必须最外层，Recovery 必须在业务逻辑之前。
package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/alinger/community-handyman/server/internal/errs"
	"github.com/alinger/community-handyman/server/pkg/response"
	"github.com/alinger/community-handyman/server/pkg/util"
)

// HeaderRequestID 是透传 trace id 的请求头与响应头名。
const HeaderRequestID = "X-Request-ID"

// CtxUserID 是鉴权后写入 context 的用户标识键。
const CtxUserID = "user_id"

// RequestID 为每个请求生成或透传 trace id。
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(HeaderRequestID)
		if id == "" {
			id = util.NewRequestID()
		}
		c.Set(response.CtxRequestID, id)
		c.Header(HeaderRequestID, id)
		c.Next()
	}
}

// AccessLog 输出结构化访问日志。
// 不记录请求体与查询串，避免病历、门禁密码、钥匙信息进入日志。
func AccessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		// 未命中路由时 FullPath 为空，回落到实际请求路径，便于排查扫描类请求
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		attrs := []any{
			"method", c.Request.Method,
			"path", path,
			"status", c.Writer.Status(),
			"cost_ms", time.Since(start).Milliseconds(),
			"ip", c.ClientIP(),
			"request_id", c.GetString(response.CtxRequestID),
		}
		if uid, ok := c.Get(CtxUserID); ok {
			attrs = append(attrs, "user_id", uid)
		}
		if len(c.Errors) > 0 {
			attrs = append(attrs, "err", c.Errors.String())
		}

		switch {
		case c.Writer.Status() >= 500:
			slog.Error("请求完成", attrs...)
		case c.Writer.Status() >= 400:
			slog.Warn("请求完成", attrs...)
		default:
			slog.Info("请求完成", attrs...)
		}
	}
}

// Recovery 捕获 panic，转 500 并打完整堆栈。
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("panic 已恢复",
					"panic", r,
					"path", c.FullPath(),
					"request_id", c.GetString(response.CtxRequestID),
					"stack", string(debug.Stack()),
				)
				// 响应头可能已写出，此时只中止不重写
				if c.Writer.Written() {
					c.Abort()
					return
				}
				c.AbortWithStatusJSON(http.StatusInternalServerError, response.Body{
					Code:      errs.CodeInternal,
					Message:   errs.Message(errs.CodeInternal),
					RequestID: c.GetString(response.CtxRequestID),
				})
			}
		}()
		c.Next()
	}
}

// CORS 按白名单放行，不使用通配符。
func CORS(allowOrigins []string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(allowOrigins))
	for _, o := range allowOrigins {
		allowed[strings.TrimSpace(o)] = struct{}{}
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if _, ok := allowed[origin]; ok && origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Authorization,Content-Type,X-Request-ID")
			c.Header("Access-Control-Max-Age", "86400")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// Timeout 把 deadline 注入请求 context，供下游 DB/HTTP 调用感知。
// 注意：它不强制中断已开始的处理，需要各层主动检查 ctx.Done()。
func Timeout(d time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), d)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// BodyLimit 限制请求体大小。
// multipart 上传跳过此处：文件体积远超 1MB，由上传路由自己用更大上限包裹，
// 否则会被这里先截断，报错信息还会很难懂。
func BodyLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.HasPrefix(c.ContentType(), "multipart/form-data") {
			c.Next()
			return
		}
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		}
		c.Next()
	}
}

// Default 按固定顺序装配通用中间件。
func Default(allowOrigins []string, timeout time.Duration, maxBody int64) []gin.HandlerFunc {
	return []gin.HandlerFunc{
		RequestID(),
		AccessLog(),
		Recovery(),
		CORS(allowOrigins),
		Timeout(timeout),
		BodyLimit(maxBody),
	}
}
