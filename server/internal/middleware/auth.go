package middleware

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/alinger/community-handyman/server/internal/auth"
	"github.com/alinger/community-handyman/server/internal/errs"
	"github.com/alinger/community-handyman/server/pkg/response"
)

// RequireUser 是用户端鉴权中间件。
//
// 当前实现：从 X-User-Id 头取用户，供本地联调与沙箱联调用。
// 接入微信登录后改为解析 Authorization: Bearer <JWT>，只保留这一处改动，
// 业务层通过 CurrentUserID 取用户，不感知凭据形式。
func RequireUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := c.GetHeader("X-User-Id")
		if raw == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, response.Body{
				Code:      errs.CodeUnauthorized,
				Message:   errs.Message(errs.CodeUnauthorized),
				RequestID: c.GetString(response.CtxRequestID),
			})
			return
		}
		uid, err := strconv.ParseUint(raw, 10, 64)
		if err != nil || uid == 0 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, response.Body{
				Code:      errs.CodeUnauthorized,
				Message:   errs.Message(errs.CodeUnauthorized),
				RequestID: c.GetString(response.CtxRequestID),
			})
			return
		}
		c.Set(CtxUserID, uid)
		c.Next()
	}
}

// CurrentUserID 从 context 取当前用户。
func CurrentUserID(c *gin.Context) uint64 {
	if v, ok := c.Get(CtxUserID); ok {
		if id, ok := v.(uint64); ok {
			return id
		}
	}
	return 0
}

// CtxAdminID / CtxRole 是后台鉴权写入 context 的键。
const (
	CtxAdminID = "admin_id"
	CtxRole    = "role"
)

// AdminAuth 校验后台 token。
// 只认 Authorization: Bearer，不接受 X-User-Id，避免用户端凭据越权进后台。
func AdminAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw, err := auth.Bearer(c.GetHeader("Authorization"))
		if err != nil {
			abortUnauthorized(c)
			return
		}
		claims, err := auth.Parse(secret, raw)
		if err != nil {
			abortUnauthorized(c)
			return
		}
		if claims.Role != auth.RoleAdmin && claims.Role != auth.RoleOperator {
			c.AbortWithStatusJSON(http.StatusForbidden, response.Body{
				Code:      errs.CodeForbidden,
				Message:   errs.Message(errs.CodeForbidden),
				RequestID: c.GetString(response.CtxRequestID),
			})
			return
		}
		c.Set(CtxAdminID, claims.Sub)
		c.Set(CtxRole, claims.Role)
		c.Next()
	}
}

// RequireAdminRole 要求超管角色，用于退款与改价这类高风险操作。
func RequireAdminRole() gin.HandlerFunc {
	return func(c *gin.Context) {
		if role, _ := c.Get(CtxRole); role != auth.RoleAdmin {
			c.AbortWithStatusJSON(http.StatusForbidden, response.Body{
				Code:      errs.CodeForbidden,
				Message:   "该操作需要超管权限",
				RequestID: c.GetString(response.CtxRequestID),
			})
			return
		}
		c.Next()
	}
}

// CurrentAdminID 取当前后台账号 ID。
func CurrentAdminID(c *gin.Context) uint64 {
	if v, ok := c.Get(CtxAdminID); ok {
		if id, ok := v.(uint64); ok {
			return id
		}
	}
	return 0
}

// CurrentRole 取当前角色。
func CurrentRole(c *gin.Context) string {
	if v, ok := c.Get(CtxRole); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func abortUnauthorized(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, response.Body{
		Code:      errs.CodeUnauthorized,
		Message:   errs.Message(errs.CodeUnauthorized),
		RequestID: c.GetString(response.CtxRequestID),
	})
}
