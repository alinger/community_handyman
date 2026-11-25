package payment

import "fmt"

// New 按配置创建支付渠道。
// provider=wechat 时需要 wechat 段配置齐备（商户号/证书序列号/API 私钥/APIv3 密钥/回调地址），
// 配置缺失会在启动时报错，不会带病运行到收单环节。
func New(provider, sandboxSecret string, wechat WechatOptions) (Gateway, error) {
	switch provider {
	case "", "sandbox":
		return NewSandbox(sandboxSecret), nil
	case "wechat":
		return NewWechat(wechat)
	default:
		return nil, fmt.Errorf("未知支付渠道: %s", provider)
	}
}
