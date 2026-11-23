// Package config 负责加载与校验服务配置。
// 配置来源优先级：环境变量展开后的 yaml 文件 > 代码内默认值。
package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

// Config 是服务全量配置。
type Config struct {
	App      AppConfig      `yaml:"app"`
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Redis    RedisConfig    `yaml:"redis"`
	Wechat   WechatConfig   `yaml:"wechat"`
	Auth     AuthConfig     `yaml:"auth"`
	Security SecurityConfig `yaml:"security"`
	Payment  PaymentConfig  `yaml:"payment"`
	Storage  StorageConfig  `yaml:"storage"`
	SMS      SMSConfig      `yaml:"sms"`
}

type AppConfig struct {
	Name string `yaml:"name"`
	Env  string `yaml:"env"` // dev / staging / prod
}

type ServerConfig struct {
	Port            int           `yaml:"port"`
	Mode            string        `yaml:"mode"` // debug / release
	ReadTimeout     time.Duration `yaml:"read_timeout"`
	WriteTimeout    time.Duration `yaml:"write_timeout"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
	AllowOrigins    []string      `yaml:"allow_origins"`
}

type DatabaseConfig struct {
	DSN             string        `yaml:"dsn"`
	MaxOpenConns    int           `yaml:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
	LogSQL          bool          `yaml:"log_sql"`
}

type RedisConfig struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

// Wechat 保存微信支付所需凭据，私钥只存路径不存内容。
type WechatConfig struct {
	AppID string `yaml:"app_id"`
	// AppSecret 是 wx.login 换 openid 必需的（与支付证书无关，别混用）
	AppSecret       string `yaml:"app_secret"`
	MchID           string `yaml:"mch_id"`
	SerialNo        string `yaml:"serial_no"`
	APIv3Key        string `yaml:"api_v3_key"`
	PrivateKeyPath  string `yaml:"private_key_path"`
	NotifyURL       string `yaml:"notify_url"`
	RefundNotifyURL string `yaml:"refund_notify_url"`
	// SubscribeTemplateID 订阅消息模板 ID；未配置时订单状态通知只记日志不发送
	SubscribeTemplateID string `yaml:"subscribe_template_id"`
}

// AuthConfig 鉴权配置。secret 用于签发 token，生产必须通过环境变量注入。
type AuthConfig struct {
	Secret   string        `yaml:"secret"`
	TokenTTL time.Duration `yaml:"token_ttl"`
}

// SecurityConfig 敏感字段加密配置。
// 门禁密码、钥匙说明等落库前必须加密，密钥独立于登录签名密钥，
// 这样更换 AUTH_SECRET 不会让历史密文全部失效。
type SecurityConfig struct {
	// AccessKey 对称加密口令，任意长度，内部 SHA-256 派生 32 字节密钥。
	// 留空时退回用 AUTH_SECRET 派生，保证老环境不改配置也能启动。
	AccessKey string `yaml:"access_key"`
}

// PaymentConfig 支付渠道配置。provider 决定使用哪个实现：
// sandbox 用于本地联调，wechat 需要商户号与证书。
type PaymentConfig struct {
	Provider      string `yaml:"provider"`
	SandboxSecret string `yaml:"sandbox_secret"`
	// SkipPay 为真时下单即视为已支付，不进入支付环节。
	// 商户号与证书下来之前用它保持交易链路通畅；接微信支付后改回 false。
	SkipPay bool `yaml:"skip_pay"`
}

type StorageConfig struct {
	Provider  string `yaml:"provider"`  // local / oss / cos / s3
	LocalDir  string `yaml:"local_dir"` // local 实现的根目录
	Endpoint  string `yaml:"endpoint"`
	Region    string `yaml:"region"`
	Bucket    string `yaml:"bucket"`
	AccessKey string `yaml:"access_key"`
	SecretKey string `yaml:"secret_key"`
}

// SMSConfig 短信渠道配置。provider 决定实现：aliyun 走阿里云短信，
// console 只把验证码打到日志，仅限本地联调。
type SMSConfig struct {
	Provider        string `yaml:"provider"`
	AccessKeyID     string `yaml:"access_key_id"`
	AccessKeySecret string `yaml:"access_key_secret"`
	SignName        string `yaml:"sign_name"`
	TemplateCode    string `yaml:"template_code"`
	Endpoint        string `yaml:"endpoint"`
}

// Load 读取 yaml 配置并把 ${VAR} 形式的占位符替换为环境变量值。
// 生产环境的密钥一律走环境变量，配置文件里只写占位符。
func Load(path string) (*Config, error) {
	loadDotEnv()

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	expanded := os.Expand(string(raw), func(key string) string {
		return os.Getenv(key)
	})

	cfg := Default()
	if err := yaml.Unmarshal([]byte(expanded), cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 支付开关允许用环境变量覆盖 yaml：
	// 商户号下来后切换支付渠道时，改环境变量比改配置文件更不容易漏改环境。
	if v := strings.TrimSpace(os.Getenv("PAYMENT_SKIP_PAY")); v != "" {
		switch v {
		case "true", "1", "yes":
			cfg.Payment.SkipPay = true
		case "false", "0", "no":
			cfg.Payment.SkipPay = false
		default:
			return nil, fmt.Errorf("PAYMENT_SKIP_PAY 只能是 true / false，当前为 %q", v)
		}
	}

	// 占位符未注入环境变量时展开为空串。
	// 只有 dev 允许回落到默认值；生产环境必须显式注入，
	// 否则会带着 localhost 的 DSN 或默认密钥上线，出问题是静默的。
	if strings.TrimSpace(cfg.Database.DSN) == "" {
		if cfg.App.Env == "dev" {
			cfg.Database.DSN = Default().Database.DSN
		} else {
			return nil, fmt.Errorf("环境 %s 必须通过 MYSQL_DSN 注入数据库连接", cfg.App.Env)
		}
	}
	if strings.TrimSpace(cfg.Auth.Secret) == "" {
		if cfg.App.Env == "dev" {
			cfg.Auth.Secret = Default().Auth.Secret
		} else {
			return nil, fmt.Errorf("环境 %s 必须通过 AUTH_SECRET 注入鉴权密钥", cfg.App.Env)
		}
	}
	if cfg.Auth.TokenTTL <= 0 {
		cfg.Auth.TokenTTL = Default().Auth.TokenTTL
	}

	// 未显式配置短信渠道时：dev 打日志方便联调，其余环境按阿里云处理，
	// 让缺失密钥在启动阶段就暴露，而不是用户点"获取验证码"时才失败。
	if strings.TrimSpace(cfg.SMS.Provider) == "" {
		if cfg.App.Env == "dev" {
			cfg.SMS.Provider = "console"
		} else {
			cfg.SMS.Provider = "aliyun"
		}
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// loadDotEnv 尝试加载 .env，文件缺失不报错。
// godotenv 默认不覆盖已存在的环境变量，因此命令行显式指定的值优先。
// 依次尝试当前目录与上两级，兼容从 server/ 或项目根目录启动。
func loadDotEnv() {
	for _, p := range []string{".env", "../.env", "../../.env"} {
		if _, err := os.Stat(p); err != nil {
			continue
		}
		if err := godotenv.Load(p); err != nil {
			fmt.Fprintf(os.Stderr, "加载 %s 失败: %v\n", p, err)
		}
		return
	}
}

// Default 给出可直接启动本地开发的默认值。
func Default() *Config {
	return &Config{
		App: AppConfig{Name: "community-handyman", Env: "dev"},
		Server: ServerConfig{
			Port:            8080,
			Mode:            "debug",
			ReadTimeout:     15 * time.Second,
			WriteTimeout:    30 * time.Second,
			ShutdownTimeout: 10 * time.Second,
			AllowOrigins:    []string{"http://localhost:5173"},
		},
		Database: DatabaseConfig{
			DSN:             "root:root@tcp(127.0.0.1:3306)/community_handyman?charset=utf8mb4&parseTime=True&loc=Local",
			MaxOpenConns:    25,
			MaxIdleConns:    10,
			ConnMaxLifetime: time.Hour,
		},
		Redis:   RedisConfig{Addr: "127.0.0.1:6379"},
		Auth:    AuthConfig{Secret: "dev-only-secret-change-me", TokenTTL: 12 * time.Hour},
		Payment: PaymentConfig{Provider: "sandbox"},
		Storage: StorageConfig{Provider: "local", LocalDir: "./uploads"},
	}
}

// Validate 拦截启动即会失败的配置错误，避免带病起服务。
func (c *Config) Validate() error {
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port 非法: %d", c.Server.Port)
	}
	if strings.TrimSpace(c.Database.DSN) == "" {
		return fmt.Errorf("database.dsn 不能为空")
	}
	if c.Server.Mode != "debug" && c.Server.Mode != "release" {
		return fmt.Errorf("server.mode 只能是 debug 或 release，当前为 %q", c.Server.Mode)
	}
	// 生产环境不允许用默认密钥，否则任何人可伪造后台 token
	if c.App.Env != "dev" && strings.Contains(c.Auth.Secret, "dev-only") {
		return fmt.Errorf("生产环境必须通过 AUTH_SECRET 环境变量设置鉴权密钥")
	}
	// 生产环境不能只把验证码打日志，否则用户永远收不到短信
	switch c.SMS.Provider {
	case "aliyun":
		if strings.TrimSpace(c.SMS.AccessKeyID) == "" || strings.TrimSpace(c.SMS.AccessKeySecret) == "" ||
			strings.TrimSpace(c.SMS.SignName) == "" || strings.TrimSpace(c.SMS.TemplateCode) == "" {
			return fmt.Errorf("短信渠道 aliyun 需要 SMS_ACCESS_KEY_ID / SMS_ACCESS_KEY_SECRET / SMS_SIGN_NAME / SMS_TEMPLATE_CODE")
		}
	case "console":
		if c.App.Env != "dev" {
			return fmt.Errorf("环境 %s 不允许使用 console 短信渠道", c.App.Env)
		}
	default:
		return fmt.Errorf("sms.provider 只能是 aliyun 或 console，当前为 %q", c.SMS.Provider)
	}
	return nil
}

// MaskedDSN 返回隐去密码的 DSN，供日志输出。
func (c *Config) MaskedDSN() string {
	dsn := c.Database.DSN
	at := strings.LastIndex(dsn, "@")
	if at < 0 {
		return dsn
	}
	colon := strings.Index(dsn, ":")
	if colon < 0 || colon > at {
		return dsn
	}
	return dsn[:colon] + ":***" + dsn[at:]
}
