package payment

import "fmt"

// New 按配置创建支付渠道。
// 商户号与证书齐备后实现 wechat 分支即可，业务层无需改动。
func New(provider, sandboxSecret string) (Gateway, error) {
	switch provider {
	case "", "sandbox":
		return NewSandbox(sandboxSecret), nil
	case "wechat":
		return nil, fmt.Errorf("微信支付渠道尚未接入：需要商户号、APIv3 密钥与商户证书，配置齐备后实现 payment.NewWechat")
	default:
		return nil, fmt.Errorf("未知支付渠道: %s", provider)
	}
}
