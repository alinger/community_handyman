package sms

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/joho/godotenv"
)

// TestAliyunLiveAuth 真实调用一次阿里云只读接口，验证 AccessKey 与签名算法。
// 默认跳过，避免 CI 误发请求：
//
//	SMS_LIVE_TEST=1 go test ./internal/sms -run TestAliyunLiveAuth -v
//
// 用 QuerySendDetails 而不是 SendSms，是为了不产生真实短信与费用。
func TestAliyunLiveAuth(t *testing.T) {
	if os.Getenv("SMS_LIVE_TEST") != "1" {
		t.Skip("设置 SMS_LIVE_TEST=1 才执行真实调用")
	}
	// go test 的 cwd 是包目录，.env 在项目根
	_ = godotenv.Load("../../.env")

	if p := os.Getenv("SMS_PROVIDER"); p != "aliyun" {
		t.Fatalf("SMS_PROVIDER=%q，需为 aliyun", p)
	}
	s := NewAliyunSender(Options{
		AccessKeyID:     os.Getenv("SMS_ACCESS_KEY_ID"),
		AccessKeySecret: os.Getenv("SMS_ACCESS_KEY_SECRET"),
		SignName:        os.Getenv("SMS_SIGN_NAME"),
		TemplateCode:    os.Getenv("SMS_TEMPLATE_CODE"),
		Endpoint:        os.Getenv("SMS_ENDPOINT"),
	})
	if s.accessKeyID == "" || s.accessKeySecret == "" {
		t.Fatal("未读到 SMS_ACCESS_KEY_ID / SMS_ACCESS_KEY_SECRET")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 查昨天的记录：无记录也应返回 OK，重点看是否报签名/鉴权类错误
	day := time.Now().AddDate(0, 0, -1).Format("20060102")
	resp, err := s.QuerySendDetails(ctx, "13800138000", day)
	if err != nil {
		t.Fatalf("阿里云鉴权失败: %v", err)
	}
	t.Logf("鉴权通过: code=%s requestId=%s", resp.Code, resp.RequestID)
}
