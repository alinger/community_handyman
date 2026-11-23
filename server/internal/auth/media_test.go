package auth

import (
	"encoding/base64"
	"strconv"
	"strings"
	"testing"
	"time"
)

const testSecret = "test-secret-for-media-ticket"

const testPath = "/api/v1/orders/CH202611221515013918/records/2/file"

func TestMediaTicketRoundTrip(t *testing.T) {
	tk := SignMediaTicket(testSecret, 42, testPath, time.Minute)
	got, err := ParseMediaTicket(testSecret, tk)
	if err != nil {
		t.Fatalf("应解析成功，实际 %v", err)
	}
	if got.UID != 42 || got.Path != testPath {
		t.Fatalf("内容不匹配：%+v", got)
	}
}

func TestMediaTicketRejectsTamperedSignature(t *testing.T) {
	tk := SignMediaTicket(testSecret, 42, testPath, time.Minute)
	// 换掉最后一个字符，签名必然不匹配
	tampered := tk[:len(tk)-1] + "0"
	if _, err := ParseMediaTicket(testSecret, tampered); err == nil {
		t.Fatal("签名被篡改仍解析成功")
	}
}

func TestMediaTicketRejectsOtherSecret(t *testing.T) {
	tk := SignMediaTicket(testSecret, 42, testPath, time.Minute)
	if _, err := ParseMediaTicket("another-secret", tk); err == nil {
		t.Fatal("换密钥仍解析成功")
	}
}

func TestMediaTicketRejectsExpired(t *testing.T) {
	// 手工构造一张已过期的票：SignMediaTicket 对 ttl<=0 会退回默认值，
	// 生产上不会签发过期票，这里直接拼 payload 验证校验逻辑。
	raw := strconv.FormatInt(time.Now().Add(-time.Minute).Unix(), 10) + "|42|" + testPath
	body := base64.RawURLEncoding.EncodeToString([]byte(raw))
	tk := body + "." + sign(testSecret, body)
	if _, err := ParseMediaTicket(testSecret, tk); err == nil {
		t.Fatal("过期票据仍解析成功")
	}
}

func TestMediaTicketRejectsGarbage(t *testing.T) {
	for _, s := range []string{"", "abc", "a.b.c", ".", strings.Repeat("x", 64)} {
		if _, err := ParseMediaTicket(testSecret, s); err == nil {
			t.Fatalf("非法票据 %q 被接受", s)
		}
	}
}

// 票据的内容是 uid|path|exp，路径里出现 | 时不能被拆成四段而误解析。
func TestMediaTicketPathWithSeparator(t *testing.T) {
	p := "/api/v1/orders/X|Y/records/1/file"
	tk := SignMediaTicket(testSecret, 7, p, time.Minute)
	got, err := ParseMediaTicket(testSecret, tk)
	if err != nil {
		t.Fatalf("含 | 的路径应可解析，实际 %v", err)
	}
	if got.Path != p || got.UID != 7 {
		t.Fatalf("内容不匹配：%+v", got)
	}
}
