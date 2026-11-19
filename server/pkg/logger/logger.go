// Package logger 基于标准库 slog 提供结构化日志。
// 选型理由：不引入第三方依赖，Go 1.21+ 原生支持，JSON 输出可直接被日志采集。
package logger

import (
	"log/slog"
	"os"
	"strings"
)

// Init 初始化全局 logger。env=dev 用文本格式便于阅读，其余用 JSON。
func Init(env, level string) *slog.Logger {
	var lv slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lv = slog.LevelDebug
	case "warn":
		lv = slog.LevelWarn
	case "error":
		lv = slog.LevelError
	default:
		lv = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level:     lv,
		AddSource: lv == slog.LevelDebug,
	}

	var h slog.Handler
	if env == "dev" {
		h = slog.NewTextHandler(os.Stdout, opts)
	} else {
		h = slog.NewJSONHandler(os.Stdout, opts)
	}

	l := slog.New(h)
	slog.SetDefault(l)
	return l
}

// Redact 对敏感字段做脱敏，用于手机号、门禁密码、钥匙信息。
// 需保留前 3 后 4 之外的字符不可识别，避免日志泄露住户信息。
func Redact(s string) string {
	r := []rune(s)
	if len(r) <= 7 {
		return "***"
	}
	return string(r[:3]) + "****" + string(r[len(r)-2:])
}
