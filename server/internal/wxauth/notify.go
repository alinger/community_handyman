package wxauth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// 订阅消息发送接口（小程序）。
const SubscribeSendURL = "https://api.weixin.qq.com/cgi-bin/message/subscribe/send"

// ErrCodeNotSubscribed 用户未订阅该模板（一次性订阅授权已用完）。
// 这在真实场景里是常态：用户没点「总是保持以上选择」就会报 43101。
const ErrCodeNotSubscribed = 43101

type subscribeResp struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

// SendSubscribe 发送一条订阅消息。
//
// templateID 为空表示后台未配置模板，调用方应在此之前拦截（这里是兜底）；
// data 的 key 必须与模板里申请的字段一致（thing1 / time2 ...），值长度受微信限制。
// page 为点击消息后打开的小程序页面路径（可选）。
func (c *Client) SendSubscribe(ctx context.Context, openid, templateID, page string,
	data map[string]string) error {
	if templateID == "" {
		return nil
	}

	token, err := c.AccessToken(ctx)
	if err != nil {
		return err
	}

	payload := map[string]any{
		"touser":      openid,
		"template_id": templateID,
		"page":        page,
		"data":        data,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("构造订阅消息失败: %w", err)
	}

	u := c.subscribeURL() + "?access_token=" + url.QueryEscape(token)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("构造订阅消息请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("发送订阅消息失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err != nil {
		return fmt.Errorf("读取订阅消息响应失败: %w", err)
	}
	var out subscribeResp
	if err := json.Unmarshal(body, &out); err != nil {
		return fmt.Errorf("解析订阅消息响应失败: %w", err)
	}
	if out.ErrCode != 0 {
		return &Error{Code: out.ErrCode, Msg: out.ErrMsg}
	}
	return nil
}

func (c *Client) subscribeURL() string {
	if c.subscribeSendURL != "" {
		return c.subscribeSendURL
	}
	return SubscribeSendURL
}
