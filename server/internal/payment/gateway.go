// Package payment 封装支付渠道。
// 业务层只依赖 Gateway 接口：商户号下来后新增微信实现即可，service 代码不用改。
package payment

import (
	"context"
	"net/http"
)

// PrepayRequest 统一下单入参。
type PrepayRequest struct {
	OrderNo     string
	Amount      int64 // 单位分
	Description string
	OpenID      string // 小程序支付必需
}

// PrepayResult 统一下单结果。Params 直接交给小程序 wx.requestPayment。
type PrepayResult struct {
	Provider string            `json:"provider"`
	PrepayID string            `json:"prepay_id"`
	Params   map[string]string `json:"params,omitempty"`
	Mock     bool              `json:"mock"`
	MockHint string            `json:"mock_hint,omitempty"`
}

// Notify 是归一化后的支付回调。
type Notify struct {
	OrderNo       string
	TransactionID string
	OutTradeNo    string
	Amount        int64
	Success       bool
	Raw           []byte
}

// RefundRequest 退款入参。Total 是原订单金额，微信要求同时传。
type RefundRequest struct {
	OrderNo     string
	OutRefundNo string
	Amount      int64
	Total       int64
	Reason      string
}

// RefundResult 退款结果。
type RefundResult struct {
	RefundID    string
	OutRefundNo string
	Success     bool
}

// Gateway 是支付渠道抽象。
type Gateway interface {
	// Name 返回渠道标识，用于日志与回调路由。
	Name() string
	// Prepay 统一下单。
	Prepay(ctx context.Context, req PrepayRequest) (PrepayResult, error)
	// VerifyNotify 验签并解析回调。验签失败必须返回错误。
	VerifyNotify(ctx context.Context, header http.Header, body []byte) (*Notify, error)
	// Refund 发起退款。
	Refund(ctx context.Context, req RefundRequest) (RefundResult, error)
}
