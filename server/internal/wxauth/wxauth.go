// Package wxauth 封装微信小程序登录态换取。
//
// 它只做一件事：用 wx.login 拿到的 code 向微信服务器换取 openid / unionid。
// 业务侧（注册建号、签发 token）不在这里，避免把登录政策和微信协议耦合在一起。
package wxauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// Code2SessionURL 是微信官方接口地址，固定不变。
const Code2SessionURL = "https://api.weixin.qq.com/sns/jscode2session"

// Session 是 code 换取的结果。
type Session struct {
	OpenID     string `json:"openid"`
	UnionID    string `json:"unionid"`
	SessionKey string `json:"session_key"`
}

// 微信错误码。这些码说明 code 本身有问题，属于用户侧失败，不是系统故障。
const (
	ErrCodeInvalidCode   = 40029 // code 无效
	ErrCodeCodeUsed      = 40163 // code 已被使用
	ErrCodeRiskControl   = 40226 // 高风险用户，微信拦截
	ErrCodeInvalidAppID  = 40013 // AppID 无效：配置错，不是用户问题
	ErrCodeInvalidSecret = 40125 // AppSecret 无效：配置错，不是用户问题
	ErrCodeSystemBusy    = -1    // 微信侧繁忙，可重试
)

// Error 是微信接口返回的业务错误。
type Error struct {
	Code int
	Msg  string
}

func (e *Error) Error() string {
	return fmt.Sprintf("微信登录失败: errcode=%d errmsg=%s", e.Code, e.Msg)
}

// Client 调用微信接口。不引入 SDK：只有几个 GET/POST 接口，手写更可控。
type Client struct {
	appID     string
	appSecret string
	endpoint  string
	http      *http.Client

	// 接口调用凭证缓存。access_token 有效期 7200 秒且有调用频次限制，
	// 必须缓存复用，不能每次请求都去换。
	tokenMu     sync.Mutex
	token       string
	tokenExpire time.Time

	// 接口地址，留空走官方地址；测试注入假服务用。
	accessTokenURL   string
	phoneNumberURL   string
	subscribeSendURL string
}

// New 构造客户端。endpoint 留空用官方地址，测试时可指向本地假服务。
func New(appID, appSecret, endpoint string) *Client {
	if endpoint == "" {
		endpoint = Code2SessionURL
	}
	return &Client{
		appID:     appID,
		appSecret: appSecret,
		endpoint:  endpoint,
		http:      &http.Client{Timeout: 6 * time.Second},
	}
}

// Enabled 判断是否具备调用条件。缺配置时业务层应走降级分支而不是报错崩掉。
func (c *Client) Enabled() bool {
	return c != nil && c.appID != "" && c.appSecret != ""
}

// code2SessionResp 微信返回体。errcode 为 0 或缺省表示成功。
type code2SessionResp struct {
	OpenID     string `json:"openid"`
	UnionID    string `json:"unionid"`
	SessionKey string `json:"session_key"`
	ErrCode    int    `json:"errcode"`
	ErrMsg     string `json:"errmsg"`
}

// Code2Session 用临时 code 换取 openid。
func (c *Client) Code2Session(ctx context.Context, code string) (*Session, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("未配置微信 AppID / AppSecret")
	}
	if code == "" {
		return nil, fmt.Errorf("code 不能为空")
	}

	q := url.Values{}
	q.Set("appid", c.appID)
	q.Set("secret", c.appSecret)
	q.Set("js_code", code)
	q.Set("grant_type", "authorization_code")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint+"?"+q.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("构造微信请求失败: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("调用微信接口失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err != nil {
		return nil, fmt.Errorf("读取微信响应失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("微信接口返回 HTTP %d", resp.StatusCode)
	}

	var out code2SessionResp
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("解析微信响应失败: %w", err)
	}
	// 微信成功时也可能带 errcode:0，用非 0 判断失败更稳妥
	if out.ErrCode != 0 {
		return nil, &Error{Code: out.ErrCode, Msg: out.ErrMsg}
	}
	if out.OpenID == "" {
		return nil, fmt.Errorf("微信未返回 openid")
	}
	return &Session{OpenID: out.OpenID, UnionID: out.UnionID, SessionKey: out.SessionKey}, nil
}
