package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/alinger/community-handyman/server/internal/dto"
	"github.com/alinger/community-handyman/server/internal/middleware"
	"github.com/alinger/community-handyman/server/pkg/response"
)

// WechatLogin 微信一键授权登录。
// @Summary      微信授权登录
// @Description  前端调 wx.login 拿 code，后端用 code 换 openid。openid 不存在时自动建号。
// @Description  响应中的 need_bind_phone 为真表示账号还没手机号，前端应引导绑定（不阻断浏览）。
// @Description  dev 环境未配置 AppID/AppSecret 时，code 会被直接当作 openid 使用，便于本地联调。
// @Tags         认证
// @Accept       json
// @Produce      json
// @Param        body  body  dto.WechatLoginReq  true  "微信 code"
// @Success      200  {object}  response.Body  "data 为 service.WechatLoginResult"
// @Failure      200  {object}  response.Body  "1002 微信登录失效 / 1003 账号已封禁"
// @Router       /api/v1/auth/wechat [post]
func (a *API) WechatLogin(c *gin.Context) {
	var req dto.WechatLoginReq
	if !bindJSON(c, &req) {
		return
	}
	res, err := a.WechatAuth.Login(c.Request.Context(), req.Code, trimSpace(req.Nickname), trimSpace(req.Avatar))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

// BindPhone 给当前登录账号绑定手机号。
// @Summary      绑定手机号
// @Description  微信登录后补手机号；该号码已被其他账号占用时返回 1005，不做自动合并。
// @Tags         认证
// @Accept       json
// @Produce      json
// @Param        body  body  dto.BindPhoneReq  true  "手机号与验证码"
// @Success      200  {object}  response.Body  "data 为 service.Session"
// @Failure      200  {object}  response.Body  "1005 号码已被占用"
// @Security     UserID
// @Router       /api/v1/auth/phone [post]
func (a *API) BindPhone(c *gin.Context) {
	var req dto.BindPhoneReq
	if !bindJSON(c, &req) {
		return
	}
	s, err := a.WechatAuth.BindPhone(c.Request.Context(), middleware.CurrentUserID(c),
		trimSpace(req.Phone), req.Code)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, s)
}

// BindPhoneByWechat 用微信「手机号快速验证组件」一键绑定手机号。
// @Summary      微信授权手机号
// @Description  前端 button open-type="getPhoneNumber" 拿到 code，后端用 access_token 换手机号并绑定。
// @Description  code 5 分钟内有效且只能消费一次；号码已被其他账号占用时返回 1005，不做自动合并。
// @Description  该能力仅对非个人主体且已认证的小程序开放，未开通时前端应引导使用短信验证码绑定。
// @Tags         认证
// @Accept       json
// @Produce      json
// @Param        body  body  dto.WechatPhoneReq  true  "手机号授权 code"
// @Success      200  {object}  response.Body  "data 为 service.Session"
// @Failure      200  {object}  response.Body  "1002 授权失效 / 1005 号码已被占用"
// @Security     UserID
// @Router       /api/v1/auth/wechat/phone [post]
func (a *API) BindPhoneByWechat(c *gin.Context) {
	var req dto.WechatPhoneReq
	if !bindJSON(c, &req) {
		return
	}
	s, err := a.WechatAuth.BindPhoneByWechatCode(c.Request.Context(),
		middleware.CurrentUserID(c), trimSpace(req.Code))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, s)
}
