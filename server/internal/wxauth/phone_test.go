package wxauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// newFakePhoneAPI 起一个假的 token + 手机号接口，统计各被调用次数。
// tokenCalls 用来验证凭证确实被缓存复用（微信对 token 获取有频次限制）。
func newFakePhoneAPI(t *testing.T, tokenBody, phoneBody string, tokenCalls *int32) *Client {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(tokenCalls, 1)
		q := r.URL.Query()
		if q.Get("grant_type") != "client_credential" || q.Get("appid") == "" || q.Get("secret") == "" {
			t.Errorf("token 请求参数不对: %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(tokenBody))
	})
	mux.HandleFunc("/phone", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("手机号接口应为 POST，实际 %s", r.Method)
		}
		if r.URL.Query().Get("access_token") == "" {
			t.Error("缺少 access_token")
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body["code"] == "" {
			t.Errorf("请求体应包含 code，实际 %v", body)
		}
		_, _ = w.Write([]byte(phoneBody))
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c := New("testappid", "testsecret", "")
	c.accessTokenURL = srv.URL + "/token"
	c.phoneNumberURL = srv.URL + "/phone"
	return c
}

func TestPhoneNumberSuccessAndTokenCached(t *testing.T) {
	var tokenCalls int32
	c := newFakePhoneAPI(t,
		`{"access_token":"AT_1","expires_in":7200}`,
		`{"errcode":0,"phone_info":{"phoneNumber":"+8613800138000","purePhoneNumber":"13800138000","countryCode":"86"}}`,
		&tokenCalls)

	info, err := c.PhoneNumber(context.Background(), "phonecode")
	if err != nil {
		t.Fatalf("不应报错: %v", err)
	}
	if info.PurePhoneNumber != "13800138000" {
		t.Fatalf("手机号不符: %+v", info)
	}

	// 再换一次：凭证应命中缓存，token 接口只被调用一次
	if _, err := c.PhoneNumber(context.Background(), "phonecode2"); err != nil {
		t.Fatalf("第二次调用失败: %v", err)
	}
	if got := atomic.LoadInt32(&tokenCalls); got != 1 {
		t.Fatalf("access_token 应缓存复用，实际请求 %d 次", got)
	}
}

func TestPhoneNumberInvalidCode(t *testing.T) {
	var tokenCalls int32
	c := newFakePhoneAPI(t,
		`{"access_token":"AT_1","expires_in":7200}`,
		`{"errcode":40029,"errmsg":"invalid code"}`,
		&tokenCalls)

	_, err := c.PhoneNumber(context.Background(), "bad")
	we, ok := err.(*Error)
	if !ok {
		t.Fatalf("应为 *Error，实际 %T", err)
	}
	if we.Code != ErrCodeInvalidCode {
		t.Fatalf("错误码应为 40029，实际 %d", we.Code)
	}
	// 令牌类错误不该触发凭证重试
	if got := atomic.LoadInt32(&tokenCalls); got != 1 {
		t.Fatalf("不应重复取凭证，实际 %d 次", got)
	}
}

func TestPhoneNumberEmptyCode(t *testing.T) {
	var tokenCalls int32
	c := newFakePhoneAPI(t, `{"access_token":"AT_1","expires_in":7200}`, `{}`, &tokenCalls)
	if _, err := c.PhoneNumber(context.Background(), "  "); err == nil {
		t.Fatal("空 code 应报错")
	}
}

func TestAccessTokenWechatError(t *testing.T) {
	var tokenCalls int32
	c := newFakePhoneAPI(t, `{"errcode":40125,"errmsg":"invalid appsecret"}`, `{}`, &tokenCalls)
	_, err := c.AccessToken(context.Background())
	we, ok := err.(*Error)
	if !ok {
		t.Fatalf("应为 *Error，实际 %T", err)
	}
	if we.Code != ErrCodeInvalidSecret {
		t.Fatalf("错误码应为 40125，实际 %d", we.Code)
	}
}
