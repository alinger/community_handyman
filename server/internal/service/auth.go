package service

import (
	"context"
	"log/slog"
	"regexp"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/alinger/community-handyman/server/internal/auth"
	"github.com/alinger/community-handyman/server/internal/errs"
	"github.com/alinger/community-handyman/server/internal/model"
	"github.com/alinger/community-handyman/server/internal/repository"
)

// 手机号与密码的长度约束，与前端校验保持一致。
const (
	phonePattern   = `^1[3-9]\d{9}$`
	minPasswordLen = 6
	maxPasswordLen = 32
)

var phoneRe = regexp.MustCompile(phonePattern)

// AuthService 用户端注册与登录。
//
// 登录标识选手机号而不是 openid：本项目线下社区场景，用户不一定会走微信授权，
// 手机号也是服务方上门联系的必要信息，注册时顺带收集。
// openid 仍保留唯一约束，将来接微信登录时按 openid 找人或补绑手机号。
type AuthService struct {
	users    *repository.UserRepo
	quotas   *repository.QuotaRepo
	secret   string
	tokenTTL time.Duration
}

func NewAuthService(users *repository.UserRepo, quotas *repository.QuotaRepo,
	secret string, ttl time.Duration) *AuthService {
	return &AuthService{users: users, quotas: quotas, secret: secret, tokenTTL: ttl}
}

// 注册赠送的免费倒垃圾次数。改这里即可调整，无需动数据库。
const registerFreeGarbageQuota = 2

// grantWelcomeQuota 发放注册礼：免费倒垃圾次数。
// 失败只记日志不阻断注册 —— 额度发不出去可以补发，注册失败会让用户体验直接断掉。
func (s *AuthService) grantWelcomeQuota(ctx context.Context, userID uint64) {
	if s.quotas == nil {
		return
	}
	if err := s.quotas.Grant(ctx, userID, model.ServiceGarbage, registerFreeGarbageQuota); err != nil {
		slog.Error("发放注册免费次数失败", "user_id", userID, "err", err)
	}
}

// Session 登录/注册成功后返回给前端的内容。
type Session struct {
	Token     string   `json:"token"`
	ExpiresIn int      `json:"expires_in"`
	User      UserInfo `json:"user"`
}

// UserInfo 会话中的用户信息，不含密码等敏感字段。
type UserInfo struct {
	UserID   uint64 `json:"user_id"`
	Nickname string `json:"nickname"`
	Phone    string `json:"phone"`
	Role     string `json:"role"`
	Avatar   string `json:"avatar"`
}

// Register 手机号注册。openid 用手机号派生，保证唯一且不与 dev 账号冲突。
func (s *AuthService) Register(ctx context.Context, phone, password, nickname string) (*Session, error) {
	if !phoneRe.MatchString(phone) {
		return nil, errs.BizMsg(errs.CodeInvalidParam, "手机号格式不正确")
	}
	if err := validatePassword(password); err != nil {
		return nil, err
	}
	nickname = defaultNickname(nickname)

	// 先查一次给出友好提示；并发下仍可能撞唯一索引，靠下面的重查兜底
	exist, err := s.users.GetByPhone(ctx, phone)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	if exist != nil {
		return nil, errs.Biz(errs.CodePhoneTaken)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}

	u := &model.User{
		OpenID:       "phone_" + phone, // 派生 openid，占住唯一约束
		Nickname:     nickname,
		Phone:        phone,
		PasswordHash: string(hash),
		Role:         auth.RoleUser,
	}
	if err := s.users.Create(ctx, u); err != nil {
		// 并发注册同一手机号会撞 uk_users_phone
		if again, e2 := s.users.GetByPhone(ctx, phone); e2 == nil && again != nil {
			return nil, errs.Biz(errs.CodePhoneTaken)
		}
		return nil, errs.Wrap(errs.CodeInternal, err)
	}

	s.grantWelcomeQuota(ctx, u.ID)
	return s.session(u)
}

// Login 手机号 + 密码登录。
// 账号不存在与密码错误返回同一提示，避免被用来枚举已注册手机号。
func (s *AuthService) Login(ctx context.Context, phone, password string) (*Session, error) {
	if phone == "" || password == "" {
		return nil, errs.BizMsg(errs.CodeInvalidParam, "请输入手机号与密码")
	}
	u, err := s.users.GetByPhone(ctx, phone)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	if u == nil || !u.HasPassword() {
		return nil, errs.BizMsg(errs.CodeUnauthorized, "手机号或密码错误")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return nil, errs.BizMsg(errs.CodeUnauthorized, "手机号或密码错误")
	}
	if u.Banned() {
		return nil, errs.BizMsg(errs.CodeForbidden, "账号已被封禁，请联系客服")
	}
	return s.session(u)
}

// ChangePassword 修改密码，需校验旧密码。
func (s *AuthService) ChangePassword(ctx context.Context, userID uint64, oldPwd, newPwd string) error {
	u, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return errs.Wrap(errs.CodeInternal, err)
	}
	if u == nil {
		return errs.Biz(errs.CodeNotFound)
	}
	if !u.HasPassword() {
		return errs.BizMsg(errs.CodeInvalidParam, "该账号尚未设置密码")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(oldPwd)); err != nil {
		return errs.BizMsg(errs.CodeInvalidParam, "原密码不正确")
	}
	if err := validatePassword(newPwd); err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPwd), bcrypt.DefaultCost)
	if err != nil {
		return errs.Wrap(errs.CodeInternal, err)
	}
	if err := s.users.SetPassword(ctx, userID, string(hash)); err != nil {
		return errs.Wrap(errs.CodeInternal, err)
	}
	return nil
}

// SessionFor 为已有用户直接签发会话，供 dev 联调登录复用同一套返回结构。
func (s *AuthService) SessionFor(u *model.User) (*Session, error) {
	return s.session(u)
}

func (s *AuthService) session(u *model.User) (*Session, error) {
	token := auth.Sign(s.secret, auth.Claims{Sub: u.ID, Role: u.Role}, s.tokenTTL)
	return &Session{
		Token:     token,
		ExpiresIn: int(s.tokenTTL.Seconds()),
		User: UserInfo{
			UserID:   u.ID,
			Nickname: u.Nickname,
			Phone:    u.Phone,
			Role:     u.Role,
			Avatar:   u.Avatar,
		},
	}, nil
}

func validatePassword(pwd string) error {
	if len(pwd) < minPasswordLen {
		return errs.BizMsg(errs.CodeInvalidParam, "密码至少 6 位")
	}
	if len(pwd) > maxPasswordLen {
		return errs.BizMsg(errs.CodeInvalidParam, "密码最多 32 位")
	}
	return nil
}

func defaultNickname(n string) string {
	if n == "" {
		return "住户"
	}
	return n
}
