package wxauth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// 微信「手机号快速验证组件」相关接口。
//
// 链路：小程序 <button open-type="getPhoneNumber"> 拿到一次性 code → 后端用
// access_token 调 getuserphonenumber 换成手机号。code 有效期 5 分钟且只能消费一次，
// 与 wx.login 的 code 不是一回事，不能混用。
const (
	AccessTokenURL = "https://api.weixin.qq.com/cgi-bin/token"
	PhoneNumberURL = "https://api.weixin.qq.com/wxa/business/getuserphonenumber"
)

// 凭证类错误码。出现它们说明 access_token 过期或失效，刷新后重试一次即可。
const (
	ErrCodeTokenInvalid = 40001
	ErrCodeTokenExpired = 42001
)

// PhoneInfo 是换取到的手机号信息。只保留业务需要的字段。
type PhoneInfo struct {
	PhoneNumber     string `json:"phoneNumber"`     // 带区号，如 +8613800138000
	PurePhoneNumber string `json:"purePhoneNumber"` // 纯号码，如 13800138000
	CountryCode     string `json:"countryCode"`
}

type accessTokenResp struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
}

type phoneNumberResp struct {
	ErrCode   int        `json:"errcode"`
	ErrMsg    string     `json:"errmsg"`
	PhoneInfo *PhoneInfo `json:"phone_info"`
}

// AccessToken 取接口调用凭证，进程内缓存到过期前 5 分钟。
//
// 不缓存会踩两个坑：微信对 token 获取有频次限制，且新 token 签发会让旧 token
// 在部分接口上立即失效，多实例/多请求并发换 token 会互相踢掉。
func (c *Client) AccessToken(ctx context.Context) (string, error) {
	if !c.Enabled() {
		return "", fmt.Errorf("未配置微信 AppID / AppSecret")
	}

	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()
	if c.token != "" && time.Now().Before(c.tokenExpire) {
		return c.token, nil
	}

	u := c.tokenURL()
	q := url.Values{}
	q.Set("grant_type", "client_credential")
	q.Set("appid", c.appID)
	q.Set("secret", c.appSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u+"?"+q.Encode(), nil)
	if err != nil {
		return "", fmt.Errorf("构造 access_token 请求失败: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("获取 access_token 失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err != nil {
		return "", fmt.Errorf("读取 access_token 响应失败: %w", err)
	}
	var out accessTokenResp
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("解析 access_token 响应失败: %w", err)
	}
	if out.ErrCode != 0 {
		return "", &Error{Code: out.ErrCode, Msg: out.ErrMsg}
	}
	if out.AccessToken == "" {
		return "", fmt.Errorf("access_token 为空")
	}

	// 提前 5 分钟过期，避免临界点上的请求带着刚失效的 token 出去
	ttl := time.Duration(out.ExpiresIn) * time.Second
	if ttl > 10*time.Minute {
		ttl -= 5 * time.Minute
	}
	c.token = out.AccessToken
	c.tokenExpire = time.Now().Add(ttl)
	return c.token, nil
}

// PhoneNumber 用 getPhoneNumber 的动态令牌换取手机号。
//
// 令牌只能消费一次，因此这里不做重试：token 失效（40001/42001）属于凭证问题，
// 刷新凭证后重试是安全的；其他错误直接把微信错误码抛给上层判断。
func (c *Client) PhoneNumber(ctx context.Context, code string) (*PhoneInfo, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("未配置微信 AppID / AppSecret")
	}
	if code == "" {
		return nil, fmt.Errorf("code 不能为空")
	}

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		token, err := c.AccessToken(ctx)
		if err != nil {
			return nil, err
		}

		info, err := c.getPhoneNumber(ctx, token, code)
		if err == nil {
			return info, nil
		}
		lastErr = err

		// 凭证失效：清缓存后重试一次；第二次仍失败说明不是凭证问题
		if we, ok := err.(*Error); ok &&
			(we.Code == ErrCodeTokenInvalid || we.Code == ErrCodeTokenExpired) &&
			attempt == 0 {
			c.invalidateToken()
			continue
		}
		return nil, err
	}
	return nil, lastErr
}

func (c *Client) getPhoneNumber(ctx context.Context, token, code string) (*PhoneInfo, error) {
	payload, err := json.Marshal(map[string]string{"code": code})
	if err != nil {
		return nil, fmt.Errorf("构造手机号请求失败: %w", err)
	}

	u := c.phoneURL() + "?access_token=" + url.QueryEscape(token)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("构造手机号请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("调用微信手机号接口失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err != nil {
		return nil, fmt.Errorf("读取手机号响应失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("微信手机号接口返回 HTTP %d", resp.StatusCode)
	}

	var out phoneNumberResp
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("解析手机号响应失败: %w", err)
	}
	if out.ErrCode != 0 {
		return nil, &Error{Code: out.ErrCode, Msg: out.ErrMsg}
	}
	if out.PhoneInfo == nil {
		return nil, fmt.Errorf("微信未返回手机号信息")
	}
	return out.PhoneInfo, nil
}

// invalidateToken 让下一次调用重新取凭证。
func (c *Client) invalidateToken() {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()
	c.token = ""
	c.tokenExpire = time.Time{}
}

// 接口地址允许测试替换，生产固定走官方地址。
func (c *Client) tokenURL() string {
	if c.accessTokenURL != "" {
		return c.accessTokenURL
	}
	return AccessTokenURL
}

func (c *Client) phoneURL() string {
	if c.phoneNumberURL != "" {
		return c.phoneNumberURL
	}
	return PhoneNumberURL
}
