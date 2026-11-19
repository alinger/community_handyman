package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/alinger/community-handyman/server/internal/dto"
	"github.com/alinger/community-handyman/server/internal/middleware"
	"github.com/alinger/community-handyman/server/pkg/response"
)

// ---------- 用户端注册与登录 ----------

// Register 手机号注册。
// @Summary      用户注册
// @Description  手机号 + 密码注册，成功后直接返回登录 token，无需再次登录
// @Tags         认证
// @Accept       json
// @Produce      json
// @Param        body  body  dto.RegisterReq  true  "手机号、密码、昵称（昵称可空）"
// @Success      200  {object}  response.Body  "data 为 service.Session"
// @Failure      200  {object}  response.Body  "1005 手机号已注册 / 1001 参数校验失败"
// @Router       /api/v1/auth/register [post]
func (a *API) Register(c *gin.Context) {
	var req dto.RegisterReq
	if !bindJSON(c, &req) {
		return
	}
	s, err := a.Auth.Register(c.Request.Context(), trimSpace(req.Phone), req.Password, trimSpace(req.Nickname))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, s)
}

// UserLogin 手机号登录。
// @Summary      用户登录
// @Description  手机号 + 密码登录，返回 token 与用户信息
// @Tags         认证
// @Accept       json
// @Produce      json
// @Param        body  body  dto.UserLoginReq  true  "手机号与密码"
// @Success      200  {object}  response.Body  "data 为 service.Session"
// @Failure      200  {object}  response.Body  "1002 手机号或密码错误"
// @Router       /api/v1/auth/login [post]
func (a *API) UserLogin(c *gin.Context) {
	var req dto.UserLoginReq
	if !bindJSON(c, &req) {
		return
	}
	s, err := a.Auth.Login(c.Request.Context(), trimSpace(req.Phone), req.Password)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, s)
}

// ChangePassword 修改密码。
// @Summary      修改密码
// @Description  校验旧密码后更新，旧密码错误返回 1001
// @Tags         认证
// @Accept       json
// @Produce      json
// @Param        body  body  dto.ChangePasswordReq  true  "旧密码与新密码"
// @Success      200  {object}  response.Body
// @Security     UserID
// @Router       /api/v1/auth/password [put]
func (a *API) ChangePassword(c *gin.Context) {
	var req dto.ChangePasswordReq
	if !bindJSON(c, &req) {
		return
	}
	err := a.Auth.ChangePassword(c.Request.Context(), middleware.CurrentUserID(c),
		req.OldPassword, req.NewPassword)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"changed": true})
}
