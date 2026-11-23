package sms

import (
	"strings"
	"testing"
)

func TestPercentEncode(t *testing.T) {
	cases := map[string]string{
		"abc":          "abc",
		"a b":          "a%20b",
		"a*b":          "a%2Ab",
		"a~b":          "a~b", // 阿里云要求 ~ 不编码
		"a/b":          "a%2Fb",
		"小区好帮手":        "%E5%B0%8F%E5%8C%BA%E5%A5%BD%E5%B8%AE%E6%89%8B",
		`{"code":"1"}`: "%7B%22code%22%3A%221%22%7D",
	}
	for in, want := range cases {
		if got := percentEncode(in); got != want {
			t.Errorf("percentEncode(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCanonicalQuery(t *testing.T) {
	params := map[string]string{
		"Version":      "2017-05-25",
		"Action":       "SendSms",
		"PhoneNumbers": "13800138000",
	}
	want := "Action=SendSms&PhoneNumbers=13800138000&Version=2017-05-25"
	if got := canonicalQuery(params); got != want {
		t.Fatalf("canonicalQuery = %q, want %q", got, want)
	}
}

// TestSignGolden 用固定入参锁住签名结果。
// 期望值由 Python 独立实现同一套规则算出，改动签名逻辑会立刻失败。
func TestSignGolden(t *testing.T) {
	s := NewAliyunSender(Options{AccessKeyID: "testId", AccessKeySecret: "testSecret"})
	params := map[string]string{
		"AccessKeyId":      "testId",
		"Action":           "SendSms",
		"Format":           "JSON",
		"RegionId":         "cn-hangzhou",
		"SignatureMethod":  "HMAC-SHA1",
		"SignatureNonce":   "abc123",
		"SignatureVersion": "1.0",
		"Timestamp":        "2026-01-01T00:00:00Z",
		"Version":          "2017-05-25",
		"PhoneNumbers":     "13800138000",
		"SignName":         "小区好帮手",
		"TemplateCode":     "SMS_123",
		"TemplateParam":    `{"code":"123456"}`,
	}
	const want = "CUaH8ou4HTVdVlCotHhYxdOJOWI="
	if got := s.sign(params); got != want {
		t.Fatalf("sign = %q, want %q", got, want)
	}
}

func TestNewValidatesAliyunConfig(t *testing.T) {
	if _, err := New(Options{Provider: "aliyun", AccessKeyID: "id"}); err == nil ||
		!strings.Contains(err.Error(), "SMS_ACCESS_KEY_SECRET") {
		t.Fatalf("缺少密钥应报错，实际: %v", err)
	}
	if _, err := New(Options{Provider: "aliyun", AccessKeyID: "id", AccessKeySecret: "sec",
		SignName: "签名", TemplateCode: "SMS_1"}); err != nil {
		t.Fatalf("配置齐全不应报错: %v", err)
	}
	if _, err := New(Options{Provider: "unknown"}); err == nil {
		t.Fatal("未知渠道应报错")
	}
}
