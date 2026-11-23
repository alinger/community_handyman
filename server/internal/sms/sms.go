// Package sms 封装短信下发渠道。
// 业务层只依赖 Sender 接口，切换渠道（阿里云 / 控制台 / 其它厂商）不改调用方。
package sms

import (
	"context"
	"errors"
	"fmt"

	"log/slog"
)

// Sender 短信下发渠道。
type Sender interface {
	// Name 渠道名，用于启动日志。
	Name() string
	// SendCode 向指定手机号下发验证码。
	SendCode(ctx context.Context, phone, code string) error
}

// Options 渠道初始化参数。密钥只从环境变量注入。
type Options struct {
	Provider        string // aliyun / console
	AccessKeyID     string
	AccessKeySecret string
	SignName        string
	TemplateCode    string
	Endpoint        string // 留空用官方默认地址
	Log             *slog.Logger
}

// New 按 provider 构造渠道。
// 生产环境缺少必要配置时直接报错，避免服务起来了却发不出短信。
func New(opts Options) (Sender, error) {
	switch opts.Provider {
	case "console":
		if opts.Log == nil {
			return nil, errors.New("console 短信渠道需要 logger")
		}
		return &ConsoleSender{log: opts.Log}, nil

	case "aliyun":
		if opts.AccessKeyID == "" || opts.AccessKeySecret == "" {
			return nil, errors.New("阿里云短信需要 SMS_ACCESS_KEY_ID / SMS_ACCESS_KEY_SECRET")
		}
		if opts.SignName == "" {
			return nil, errors.New("阿里云短信需要 SMS_SIGN_NAME（短信签名）")
		}
		if opts.TemplateCode == "" {
			return nil, errors.New("阿里云短信需要 SMS_TEMPLATE_CODE（模板 CODE）")
		}
		return NewAliyunSender(opts), nil

	case "":
		if opts.Log != nil {
			opts.Log.Warn("未配置短信渠道，回落到控制台输出（生产环境必须配置 SMS_PROVIDER=aliyun）")
			return &ConsoleSender{log: opts.Log}, nil
		}
		return nil, errors.New("未配置短信渠道")

	default:
		return nil, fmt.Errorf("不支持的短信渠道: %s", opts.Provider)
	}
}

// ConsoleSender 把验证码打到日志，仅用于本地联调。
type ConsoleSender struct {
	log *slog.Logger
}

func (s *ConsoleSender) Name() string { return "console" }

func (s *ConsoleSender) SendCode(_ context.Context, phone, code string) error {
	s.log.Warn("【开发模式】短信验证码未真实下发", "phone", phone, "code", code)
	return nil
}
