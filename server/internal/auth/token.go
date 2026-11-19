// Package auth 提供轻量签名 token。
// 格式：base64url(payload) + "." + hex(hmacSHA256)。
// 不引入 JWT 库：字段只有主体、角色、过期时间三个，手写 60 行足够，
// 且将来微信登录签发用户 token 时用的是同一套逻辑。
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// 角色常量。
const (
	RoleUser     = "user"
	RoleProvider = "provider"
	RoleOperator = "operator"
	RoleAdmin    = "admin"
)

var (
	// ErrInvalidToken 表示格式错误或签名不匹配。
	ErrInvalidToken = errors.New("token 无效")
	// ErrExpired 表示已过期。
	ErrExpired = errors.New("token 已过期")
)

// Claims 是 token 载荷。
type Claims struct {
	Sub  uint64 `json:"sub"`  // 主体 ID：用户或后台账号
	Role string `json:"role"` // 角色
	Exp  int64  `json:"exp"`  // 过期时间戳（秒）
}

// Sign 签发 token。
func Sign(secret string, c Claims, ttl time.Duration) string {
	if c.Exp == 0 {
		c.Exp = time.Now().Add(ttl).Unix()
	}
	payload, _ := json.Marshal(c)
	body := base64.RawURLEncoding.EncodeToString(payload)
	return body + "." + sign(secret, body)
}

// Parse 校验并解析 token。
func Parse(secret, token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return Claims{}, ErrInvalidToken
	}
	body, sig := parts[0], parts[1]
	if !hmac.Equal([]byte(sign(secret, body)), []byte(sig)) {
		return Claims{}, ErrInvalidToken
	}

	raw, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return Claims{}, ErrInvalidToken
	}
	var c Claims
	if err := json.Unmarshal(raw, &c); err != nil {
		return Claims{}, ErrInvalidToken
	}
	if c.Exp > 0 && time.Now().Unix() > c.Exp {
		return Claims{}, ErrExpired
	}
	if c.Sub == 0 {
		return Claims{}, ErrInvalidToken
	}
	return c, nil
}

// Bearer 从 Authorization 头取出 token。
func Bearer(header string) (string, error) {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", fmt.Errorf("Authorization 头缺少 Bearer 前缀")
	}
	token := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	if token == "" {
		return "", ErrInvalidToken
	}
	return token, nil
}

func sign(secret, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return hex.EncodeToString(mac.Sum(nil))
}
