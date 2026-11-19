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
	Payment  PaymentConfig  `yaml:"payment"`
	Storage  StorageConfig  `yaml:"storage"`
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
	AppID           string `yaml:"app_id"`
	MchID           string `yaml:"mch_id"`
	SerialNo        string `yaml:"serial_no"`
	APIv3Key        string `yaml:"api_v3_key"`
	PrivateKeyPath  string `yaml:"private_key_path"`
	NotifyURL       string `yaml:"notify_url"`
	RefundNotifyURL string `yaml:"refund_notify_url"`
}

// AuthConfig 鉴权配置。secret 用于签发 token，生产必须通过环境变量注入。
type AuthConfig struct {
	Secret   string        `yaml:"secret"`
	TokenTTL time.Duration `yaml:"token_ttl"`
}

// PaymentConfig 支付渠道配置。provider 决定使用哪个实现：
// sandbox 用于本地联调，wechat 需要商户号与证书。
type PaymentConfig struct {
	Provider      string `yaml:"provider"`
	SandboxSecret string `yaml:"sandbox_secret"`
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

	// 占位符未注入环境变量时展开为空串，会覆盖默认值，这里回落一次，
	// 避免本地开发因为忘记 export 而直接起不来。
	if strings.TrimSpace(cfg.Database.DSN) == "" {
		cfg.Database.DSN = Default().Database.DSN
	}
	// 同理：AUTH_SECRET 未注入时不能留空，否则签名密钥为空串
	if strings.TrimSpace(cfg.Auth.Secret) == "" {
		cfg.Auth.Secret = Default().Auth.Secret
	}
	if cfg.Auth.TokenTTL <= 0 {
		cfg.Auth.TokenTTL = Default().Auth.TokenTTL
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
