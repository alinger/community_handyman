package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/alinger/community-handyman/server/internal/dto"
	"github.com/alinger/community-handyman/server/pkg/response"
)

// SendSmsCode 下发短信验证码。
// @Summary      发送短信验证码
// @Description  通过阿里云短信下发 6 位验证码。同号同用途 60 秒一次、每日 10 条；验证码 5 分钟有效。
// @Description  scene=login 用于登录/注册，scene=reset 用于重置密码（仅已注册手机号）。
// @Description  响应只含 expires_in（有效期秒）与 interval（重发间隔秒），不返回验证码本身。
// @Tags         认证
// @Accept       json
// @Produce      json
// @Param        body  body  dto.SendSmsCodeReq  true  "手机号与用途"
// @Success      200  {object}  response.Body  "data 为 service.SendCodeResult"
// @Failure      200  {object}  response.Body  "1001 参数错误或发送过频 / 3003 短信通道失败"
// @Router       /api/v1/auth/sms/code [post]
func (a *API) SendSmsCode(c *gin.Context) {
	var req dto.SendSmsCodeReq
	if !bindJSON(c, &req) {
		return
	}
	res, err := a.SMS.SendCode(c.Request.Context(), trimSpace(req.Phone), req.Scene, c.ClientIP())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

// SmsLogin 验证码登录，未注册自动建号。
// @Summary      验证码登录/注册
// @Description  校验验证码；手机号未注册时自动创建账号（昵称可空，默认「住户」）。返回结构与密码登录一致。
// @Tags         认证
// @Accept       json
// @Produce      json
// @Param        body  body  dto.SmsLoginReq  true  "手机号、验证码、昵称（可选）"
// @Success      200  {object}  response.Body  "data 为 service.Session"
// @Failure      200  {object}  response.Body  "3004 验证码错误或已失效 / 1003 账号已封禁"
// @Router       /api/v1/auth/sms/login [post]
func (a *API) SmsLogin(c *gin.Context) {
	var req dto.SmsLoginReq
	if !bindJSON(c, &req) {
		return
	}
	s, err := a.SMS.Login(c.Request.Context(), trimSpace(req.Phone), req.Code, trimSpace(req.Nickname))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, s)
}

// ResetPasswordBySms 凭验证码重置密码。
// @Summary      重置密码（短信验证码）
// @Description  校验 scene=reset 的验证码后重设密码，用于用户忘记密码。验证码一次性消费。
// @Description  成功后直接返回登录态（与密码登录结构一致），前端无需再登一次。
// @Tags         认证
// @Accept       json
// @Produce      json
// @Param        body  body  dto.SmsResetPasswordReq  true  "手机号、验证码、新密码"
// @Success      200  {object}  response.Body  "data 为 service.Session"
// @Failure      200  {object}  response.Body  "3004 验证码错误或已失效 / 1001 新密码不合规"
// @Router       /api/v1/auth/password/reset [post]
func (a *API) ResetPasswordBySms(c *gin.Context) {
	var req dto.SmsResetPasswordReq
	if !bindJSON(c, &req) {
		return
	}
	s, err := a.SMS.ResetPassword(c.Request.Context(), trimSpace(req.Phone), req.Code, req.NewPassword)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, s)
}
