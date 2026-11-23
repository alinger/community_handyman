package wxauth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newFakeWX 起一个假的微信接口服务，按预设返回体回包。
func newFakeWX(t *testing.T, body string, status int) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 校验必要的查询参数确实被带上了，避免拼错参数名还"测得过"
		q := r.URL.Query()
		if q.Get("appid") == "" || q.Get("secret") == "" || q.Get("js_code") == "" ||
			q.Get("grant_type") != "authorization_code" {
			t.Errorf("请求参数缺失: %s", r.URL.RawQuery)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return New("testappid", "testsecret", srv.URL)
}

func TestCode2SessionSuccess(t *testing.T) {
	c := newFakeWX(t, `{"openid":"o_abc123","session_key":"sk","unionid":"u_1"}`, http.StatusOK)
	s, err := c.Code2Session(context.Background(), "code1")
	if err != nil {
		t.Fatalf("不应报错: %v", err)
	}
	if s.OpenID != "o_abc123" || s.UnionID != "u_1" {
		t.Fatalf("返回值不符: %+v", s)
	}
}

func TestCode2SessionWechatError(t *testing.T) {
	c := newFakeWX(t, `{"errcode":40029,"errmsg":"invalid code"}`, http.StatusOK)
	_, err := c.Code2Session(context.Background(), "bad")
	if err == nil {
		t.Fatal("应返回错误")
	}
	we, ok := err.(*Error)
	if !ok {
		t.Fatalf("应为 *Error，实际 %T", err)
	}
	if we.Code != ErrCodeInvalidCode {
		t.Fatalf("错误码应为 40029，实际 %d", we.Code)
	}
}

func TestCode2SessionEmptyOpenID(t *testing.T) {
	// 微信偶尔返回 200 却无 openid，必须当失败处理，否则会建成空 openid 的账号
	c := newFakeWX(t, `{"errcode":0,"openid":""}`, http.StatusOK)
	if _, err := c.Code2Session(context.Background(), "code"); err == nil {
		t.Fatal("openid 为空时应报错")
	}
}

func TestEnabled(t *testing.T) {
	if New("", "sec", "").Enabled() {
		t.Error("缺 appid 不应视为可用")
	}
	if New("id", "", "").Enabled() {
		t.Error("缺 secret 不应视为可用")
	}
	if !New("id", "sec", "").Enabled() {
		t.Error("配置齐全应可用")
	}
	var nilClient *Client
	if nilClient.Enabled() {
		t.Error("nil 客户端不应视为可用")
	}
}
