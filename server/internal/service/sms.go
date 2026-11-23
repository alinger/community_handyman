package service

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"fmt"
	"math/big"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/alinger/community-handyman/server/internal/auth"
	"github.com/alinger/community-handyman/server/internal/errs"
	"github.com/alinger/community-handyman/server/internal/model"
	"github.com/alinger/community-handyman/server/internal/repository"
	"github.com/alinger/community-handyman/server/internal/sms"
)

const (
	smsCodeTTL       = 5 * time.Minute  // 验证码有效期
	smsSendInterval  = 60 * time.Second // 同号同场景发送间隔
	smsDailyLimit    = 10               // 同号每日上限
	smsMaxAttempts   = 5                // 单条验证码允许的错误次数
	smsCodeLength    = 6
	smsPurgeInterval = 7 * 24 * time.Hour // 历史记录保留时长，仅作审计
)

// SmsService 手机号验证码：下发、校验、验证码登录与重置密码。
type SmsService struct {
	codes  *repository.SmsRepo
	users  *repository.UserRepo
	auth   *AuthService
	sender sms.Sender

	now func() time.Time
}

// NewSmsService 构造短信服务。
func NewSmsService(codes *repository.SmsRepo, users *repository.UserRepo, authSvc *AuthService,
	sender sms.Sender) *SmsService {
	return &SmsService{codes: codes, users: users, auth: authSvc, sender: sender, now: time.Now}
}

// SendCodeResult 下发结果。
// 只回传有效期与重发间隔，绝不包含验证码本身：
// 验证码一旦出现在响应里，就等于向调用方公开了「持有该手机号」的证明，
// 校验环节形同虚设，任何人都能拿别人手机号登录。本地联调请看服务日志。
type SendCodeResult struct {
	ExpiresIn int `json:"expires_in"`
	Interval  int `json:"interval"`
}

// SendCode 下发验证码。
// 频率限制先于生成：同号同场景 60 秒一次、每日 10 条，防止被刷短信费用。
func (s *SmsService) SendCode(ctx context.Context, phone, scene, ip string) (*SendCodeResult, error) {
	phone = strings.TrimSpace(phone)
	if !phoneRe.MatchString(phone) {
		return nil, errs.BizMsg(errs.CodeInvalidParam, "手机号格式不正确")
	}
	scene = normalizeScene(scene)
	if scene == "" {
		return nil, errs.BizMsg(errs.CodeInvalidParam, "验证码用途不合法")
	}

	// 重置密码只对已注册手机号下发，避免给陌生号码发短信
	if scene == model.SMSSceneReset {
		u, err := s.users.GetByPhone(ctx, phone)
		if err != nil {
			return nil, errs.Wrap(errs.CodeInternal, err)
		}
		if u == nil {
			return nil, errs.BizMsg(errs.CodeInvalidParam, "该手机号尚未注册")
		}
	}

	now := s.now()
	last, err := s.codes.LastSentAt(ctx, phone, scene)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	if !last.IsZero() {
		if wait := smsSendInterval - now.Sub(last); wait > 0 {
			return nil, errs.BizMsg(errs.CodeInvalidParam,
				fmt.Sprintf("发送过于频繁，请 %d 秒后重试", int(wait.Seconds())+1))
		}
	}

	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	count, err := s.codes.CountSince(ctx, phone, todayStart)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	if count >= smsDailyLimit {
		return nil, errs.BizMsg(errs.CodeInvalidParam, "今日验证码发送次数已达上限，请明天再试")
	}

	code, err := randomSMSCode()
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}

	rec := &model.SMSCode{
		Phone:     phone,
		Scene:     scene,
		Code:      code,
		Status:    model.SMSStatusPending,
		ExpiresAt: now.Add(smsCodeTTL),
		IP:        ip,
	}
	if err := s.codes.Create(ctx, rec); err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}

	// 下发失败要把记录标记掉：既不计入频率限制，也避免留下可用的"幽灵验证码"
	if err := s.sender.SendCode(ctx, phone, code); err != nil {
		if e := s.codes.UpdateStatus(ctx, rec.ID, model.SMSStatusFailed); e != nil {
			return nil, errs.Wrap(errs.CodeInternal, e)
		}
		return nil, errs.Wrap(errs.CodeSmsSendFailed, err)
	}
	if err := s.codes.UpdateStatus(ctx, rec.ID, model.SMSStatusSent); err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}

	return &SendCodeResult{
		ExpiresIn: int(smsCodeTTL.Seconds()),
		Interval:  int(smsSendInterval.Seconds()),
	}, nil
}

// Login 验证码登录。手机号未注册时自动创建账号，等价于"注册并登录"。
func (s *SmsService) Login(ctx context.Context, phone, code, nickname string) (*Session, error) {
	phone = strings.TrimSpace(phone)
	if err := s.verify(ctx, phone, model.SMSSceneLogin, code); err != nil {
		return nil, err
	}

	u, err := s.users.GetByPhone(ctx, phone)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	if u == nil {
		u, err = s.createUser(ctx, phone, nickname)
		if err != nil {
			return nil, err
		}
	}
	if u.Banned() {
		return nil, errs.BizMsg(errs.CodeForbidden, "账号已被封禁，请联系客服")
	}
	return s.auth.SessionFor(u)
}

// ResetPassword 用验证码重置密码。只对已注册手机号生效。
// 成功后直接签发会话返回，用户不必用新密码再登一次。
func (s *SmsService) ResetPassword(ctx context.Context, phone, code, newPwd string) (*Session, error) {
	phone = strings.TrimSpace(phone)
	if err := validatePassword(newPwd); err != nil {
		return nil, err
	}
	if err := s.verify(ctx, phone, model.SMSSceneReset, code); err != nil {
		return nil, err
	}

	u, err := s.users.GetByPhone(ctx, phone)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	if u == nil {
		return nil, errs.BizMsg(errs.CodeInvalidParam, "该手机号尚未注册")
	}
	if u.Banned() {
		return nil, errs.BizMsg(errs.CodeForbidden, "账号已被封禁，请联系客服")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPwd), bcrypt.DefaultCost)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	if err := s.users.SetPassword(ctx, u.ID, string(hash)); err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	return s.auth.SessionFor(u)
}

// PurgeExpired 清掉过期的历史验证码，由启动时的后台任务周期调用。
func (s *SmsService) PurgeExpired(ctx context.Context) (int64, error) {
	return s.codes.PurgeBefore(ctx, s.now().Add(-smsPurgeInterval))
}

// verify 校验验证码并消费。校验通过后该码立即失效，不能复用。
func (s *SmsService) verify(ctx context.Context, phone, scene, code string) error {
	if !phoneRe.MatchString(phone) {
		return errs.BizMsg(errs.CodeInvalidParam, "手机号格式不正确")
	}
	code = strings.TrimSpace(code)
	if len(code) != smsCodeLength {
		return errs.BizMsg(errs.CodeInvalidParam, "请输入 6 位数字验证码")
	}

	rec, err := s.codes.LatestValid(ctx, phone, scene, s.now())
	if err != nil {
		return errs.Wrap(errs.CodeInternal, err)
	}
	if rec == nil {
		return errs.BizMsg(errs.CodeSmsCodeInvalid, "验证码已失效，请重新获取")
	}
	if rec.Attempts >= smsMaxAttempts {
		return errs.BizMsg(errs.CodeSmsCodeInvalid, "验证码错误次数过多，请重新获取")
	}

	if subtle.ConstantTimeCompare([]byte(rec.Code), []byte(code)) != 1 {
		if err := s.codes.IncrAttempts(ctx, rec.ID); err != nil {
			return errs.Wrap(errs.CodeInternal, err)
		}
		if remain := smsMaxAttempts - int(rec.Attempts) - 1; remain > 0 {
			return errs.BizMsg(errs.CodeSmsCodeInvalid, fmt.Sprintf("验证码错误，还可尝试 %d 次", remain))
		}
		return errs.BizMsg(errs.CodeSmsCodeInvalid, "验证码错误次数过多，请重新获取")
	}

	ok, err := s.codes.MarkUsed(ctx, rec.ID)
	if err != nil {
		return errs.Wrap(errs.CodeInternal, err)
	}
	if !ok {
		// 并发下被其它请求抢先消费
		return errs.BizMsg(errs.CodeSmsCodeInvalid, "验证码已使用，请重新获取")
	}
	return nil
}

// createUser 验证码登录时自动建号。
// openid 由手机号派生，占住唯一约束，也便于和微信登录体系对接。
func (s *SmsService) createUser(ctx context.Context, phone, nickname string) (*model.User, error) {
	u := &model.User{
		OpenID:   "phone_" + phone,
		Nickname: defaultNickname(strings.TrimSpace(nickname)),
		Phone:    phone,
		Role:     auth.RoleUser,
	}
	if err := s.users.Create(ctx, u); err != nil {
		// 并发首次登录会撞 uk_users_phone，重查一次即可
		if again, e2 := s.users.GetByPhone(ctx, phone); e2 == nil && again != nil {
			return again, nil
		}
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	return u, nil
}

func normalizeScene(scene string) string {
	switch strings.TrimSpace(scene) {
	case "":
		return model.SMSSceneLogin
	case model.SMSSceneLogin:
		return model.SMSSceneLogin
	case model.SMSSceneReset:
		return model.SMSSceneReset
	default:
		return ""
	}
}

func randomSMSCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}
