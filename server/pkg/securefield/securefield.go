// Package securefield 加解密那些必须落库、但不该明文保存的字段
// （门禁密码、钥匙说明、进入方式补充等）。
//
// 设计取舍：
//   - 用 AES-256-GCM 而不是自拼 XOR/Base64：带完整性校验，密文被改动会解不开而不是解出垃圾；
//   - 密钥由口令经 SHA-256 派生：口令通常不是 32 字节，直接截断会浪费熵，哈希后长度也自动对齐；
//   - nonce 随机且随密文一起存，同一明文每次加密结果都不同，避免按密文比对猜出内容。
package securefield

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
)

// Box 是进程内复用的加解密器，持有派生好的 AEAD。
type Box struct {
	aead cipher.AEAD
}

// New 由口令派生密钥。口令为空视为配置缺失，直接报错，
// 避免悄悄用一个空口令加密、上线后才发现密文形同虚设。
func New(passphrase string) (*Box, error) {
	if passphrase == "" {
		return nil, errors.New("securefield: 加密口令为空")
	}
	sum := sha256.Sum256([]byte(passphrase))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Box{aead: aead}, nil
}

// Encrypt 加密并返回 base64(nonce || ciphertext)。空串原样返回空串，
// 让「用户没填」和「加密后是空」两种情况在前端表现一致。
func (b *Box) Encrypt(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := b.aead.Seal(nonce, nonce, []byte(plain), nil)
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

// Decrypt 解密。解不开通常意味着换过密钥或密文被篡改，
// 调用方应把它当成「没有这个值」处理，而不是让整个请求 500。
func (b *Box) Decrypt(enc string) (string, error) {
	if enc == "" {
		return "", nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(enc)
	if err != nil {
		return "", err
	}
	ns := b.aead.NonceSize()
	if len(raw) < ns {
		return "", errors.New("securefield: 密文长度不足")
	}
	plain, err := b.aead.Open(nil, raw[:ns], raw[ns:], nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

// Enabled 报告是否可用，供调用方在密钥缺失时降级（不写敏感字段）而不是报错。
func (b *Box) Enabled() bool { return b != nil && b.aead != nil }
