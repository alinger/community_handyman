package service

import (
	"context"
	"log/slog"
	"strings"

	"github.com/alinger/community-handyman/server/internal/auth"
	"github.com/alinger/community-handyman/server/internal/errs"
	"github.com/alinger/community-handyman/server/internal/model"
	"github.com/alinger/community-handyman/server/internal/repository"
	"github.com/alinger/community-handyman/server/internal/wxauth"
)

// WechatAuthService 微信一键授权登录。
//
// 与手机号登录的关系：openid 是主身份，手机号是「补全资料」。
// 微信登录先按 openid 找人，没有就建号；之后再引导绑定手机号，
// 绑定时若该手机号已被别的账号占用，明确拒绝而不是静默合并 ——
// 自动合并会把两个账号的订单混在一起，属于不可逆的数据事故。
type WechatAuthService struct {
	users *repository.UserRepo
	auth  *AuthService
	wx    *wxauth.Client
	log   *slog.Logger
	// debug 为 dev 且未配置 appid 时，允许用 code 直接当 openid，
	// 让没有小程序凭据的环境也能跑通前端流程。
	debug bool
}

// NewWechatAuthService 构造微信登录服务。
func NewWechatAuthService(users *repository.UserRepo, authSvc *AuthService,
	wx *wxauth.Client, log *slog.Logger, debug bool) *WechatAuthService {
	if log == nil {
		log = slog.Default()
	}
	return &WechatAuthService{users: users, auth: authSvc, wx: wx, log: log, debug: debug}
}

// WechatLoginResult 登录结果。NeedBindPhone 为真表示账号还没有手机号，
// 前端应引导绑定（不阻断浏览，但下单需要手机号联系）。
type WechatLoginResult struct {
	*Session
	NeedBindPhone bool `json:"need_bind_phone"`
}

// Login 用 wx.login 的 code 登录，openid 不存在则自动建号。
func (s *WechatAuthService) Login(ctx context.Context, code, nickname, avatar string) (*WechatLoginResult, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, errs.BizMsg(errs.CodeInvalidParam, "缺少微信登录 code")
	}

	openid, err := s.resolveOpenID(ctx, code)
	if err != nil {
		return nil, err
	}

	u, err := s.users.GetByOpenID(ctx, openid)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	if u == nil {
		u, err = s.createUser(ctx, openid, nickname, avatar)
		if err != nil {
			return nil, err
		}
	}
	if u.Banned() {
		return nil, errs.BizMsg(errs.CodeForbidden, "账号已被封禁，请联系客服")
	}

	// 微信侧昵称/头像更新时同步，但不覆盖用户在设置页自己改过的昵称
	if nickname != "" && (u.Nickname == "" || u.Nickname == defaultNickname("")) {
		u.Nickname = nickname
	}
	if avatar != "" && u.Avatar == "" {
		u.Avatar = avatar
	}
	if nickname != "" || avatar != "" {
		if err := s.users.UpdateWechatProfile(ctx, u.ID, u.Nickname, u.Avatar); err != nil {
			return nil, errs.Wrap(errs.CodeInternal, err)
		}
	}

	sess, err := s.auth.SessionFor(u)
	if err != nil {
		return nil, err
	}
	return &WechatLoginResult{Session: sess, NeedBindPhone: u.Phone == ""}, nil
}

// BindPhone 给已登录账号补手机号。
//
// 手机号唯一性是前提：已被占用的号码不能绑到第二个账号，
// 否则同一个手机号能登两个不同的 openid 账号，订单归属就乱了。
func (s *WechatAuthService) BindPhone(ctx context.Context, userID uint64, phone, code string) (*Session, error) {
	return s.bindPhone(ctx, userID, phone)
}

// BindPhoneByWechatCode 用「手机号快速验证组件」返回的 code 绑定手机号。
//
// 与 BindPhone 的区别只在号码来源：这里号码由微信校验后下发，
// 不需要用户手输、也不需要短信验证码。绑定规则（唯一性、不合并账号）完全一致。
// code 有效期 5 分钟且只能消费一次，失败只能让用户重新点一次授权。
func (s *WechatAuthService) BindPhoneByWechatCode(ctx context.Context, userID uint64, code string) (*Session, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, errs.BizMsg(errs.CodeInvalidParam, "缺少手机号授权 code")
	}
	if !s.wx.Enabled() {
		return nil, errs.BizMsg(errs.CodeInternal, "未配置微信登录，请改用短信验证码绑定")
	}

	info, err := s.wx.PhoneNumber(ctx, code)
	if err != nil {
		if we, ok := err.(*wxauth.Error); ok {
			switch we.Code {
			case wxauth.ErrCodeInvalidCode, wxauth.ErrCodeCodeUsed:
				s.log.WarnContext(ctx, "微信手机号授权 code 无效",
					"errcode", we.Code, "errmsg", we.Msg)
				return nil, errs.BizMsg(errs.CodeUnauthorized, "手机号授权已失效，请重新点击授权")
			case wxauth.ErrCodeInvalidAppID, wxauth.ErrCodeInvalidSecret,
				wxauth.ErrCodeTokenInvalid, wxauth.ErrCodeTokenExpired:
				// 配置或凭证问题：用户重试没用，直接引导走短信
				s.log.ErrorContext(ctx, "微信手机号接口配置有误，请检查 AppID / AppSecret 与手机号能力权限",
					"errcode", we.Code, "errmsg", we.Msg)
				return nil, errs.BizMsg(errs.CodeInternal, "手机号授权暂不可用，请改用短信验证码绑定")
			}
		}
		s.log.ErrorContext(ctx, "微信手机号换取失败", "err", err)
		return nil, errs.Wrap(errs.CodeInternal, err)
	}

	phone := strings.TrimSpace(info.PurePhoneNumber)
	if phone == "" {
		phone = strings.TrimSpace(info.PhoneNumber)
	}
	return s.bindPhone(ctx, userID, phone)
}

// bindPhone 落库绑定：校验格式与唯一性后写入，返回新会话。
func (s *WechatAuthService) bindPhone(ctx context.Context, userID uint64, phone string) (*Session, error) {
	phone = strings.TrimSpace(phone)
	if !phoneRe.MatchString(phone) {
		return nil, errs.BizMsg(errs.CodeInvalidParam, "手机号格式不正确")
	}

	u, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	if u == nil {
		return nil, errs.Biz(errs.CodeNotFound)
	}

	// 手机号已被其他账号占用时明确拒绝
	exist, err := s.users.GetByPhone(ctx, phone)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	if exist != nil && exist.ID != userID {
		return nil, errs.BizMsg(errs.CodePhoneTaken,
			"该手机号已绑定其他账号，请直接用它登录或联系客服")
	}

	if err := s.users.UpdatePhone(ctx, userID, phone); err != nil {
		if again, e2 := s.users.GetByPhone(ctx, phone); e2 == nil && again != nil && again.ID != userID {
			return nil, errs.BizMsg(errs.CodePhoneTaken, "该手机号已绑定其他账号")
		}
		return nil, errs.Wrap(errs.CodeInternal, err)
	}

	u.Phone = phone
	return s.auth.SessionFor(u)
}

// resolveOpenID 把 code 换成 openid。
// 未配置 appid 的 dev 环境退化为「code 即 openid」，让前端流程可联调；
// 生产缺配置属于部署错误，必须显式报错而不是悄悄放行。
func (s *WechatAuthService) resolveOpenID(ctx context.Context, code string) (string, error) {
	if s.wx.Enabled() {
		sess, err := s.wx.Code2Session(ctx, code)
		if err != nil {
			// 微信侧的错误码转成可读提示，不把 errmsg 原文抛给用户。
			// 但服务端必须留痕：code 类错误是用户侧失败（前端可重试），
			// AppID / Secret 类错误是配置故障（重启后没生效、换了 appid 没同步等），
			// 两者的排查方向完全相反，靠打日志区分。
			if we, ok := err.(*wxauth.Error); ok {
				switch we.Code {
				case wxauth.ErrCodeInvalidCode, wxauth.ErrCodeCodeUsed:
					s.log.WarnContext(ctx, "微信登录 code 无效",
						"errcode", we.Code, "errmsg", we.Msg)
					return "", errs.BizMsg(errs.CodeUnauthorized, "微信登录已失效，请重新授权")
				case wxauth.ErrCodeRiskControl:
					s.log.WarnContext(ctx, "微信登录被风控拦截",
						"errcode", we.Code, "errmsg", we.Msg)
					return "", errs.BizMsg(errs.CodeForbidden, "当前环境存在风险，请稍后再试")
				case wxauth.ErrCodeInvalidAppID, wxauth.ErrCodeInvalidSecret:
					// 配置类故障：用户重试多少次都没用，报内部错误并记 Error 级日志
					s.log.ErrorContext(ctx, "微信登录配置有误，请检查 AppID / AppSecret 是否为「小程序」类型且相互匹配",
						"errcode", we.Code, "errmsg", we.Msg)
					return "", errs.BizMsg(errs.CodeInternal, "微信登录暂不可用，请稍后再试或使用手机号登录")
				default:
					s.log.ErrorContext(ctx, "微信登录调用失败",
						"errcode", we.Code, "errmsg", we.Msg)
					return "", errs.Wrap(errs.CodeInternal, err)
				}
			}
			s.log.ErrorContext(ctx, "微信登录调用失败", "err", err)
			return "", errs.Wrap(errs.CodeInternal, err)
		}
		return sess.OpenID, nil
	}

	if s.debug {
		return "devwx_" + code, nil
	}
	return "", errs.BizMsg(errs.CodeInternal, "未配置微信登录，请联系管理员")
}

// createUser 微信首次登录建号。昵称缺省为「住户」，与手机号注册保持一致。
func (s *WechatAuthService) createUser(ctx context.Context, openid, nickname, avatar string) (*model.User, error) {
	u := &model.User{
		OpenID:   openid,
		Nickname: defaultNickname(strings.TrimSpace(nickname)),
		Avatar:   avatar,
		Role:     auth.RoleUser,
	}
	if err := s.users.Create(ctx, u); err != nil {
		// 并发首次登录会撞 uk_users_openid
		if again, e2 := s.users.GetByOpenID(ctx, openid); e2 == nil && again != nil {
			return again, nil
		}
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	// 微信首登建号同样享有注册礼
	s.auth.grantWelcomeQuota(ctx, u.ID)
	return u, nil
}
