package payment

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// Sandbox 是本地与联调用的沙箱渠道。
// 它不做真实扣款，但回调的签名校验、幂等、金额校验走的是同一条链路，
// 因此可以用来验证整套支付流程，等商户号下来直接换 Wechat 实现。
type Sandbox struct {
	secret string
}

// NewSandbox 创建沙箱渠道。secret 用于回调签名。
func NewSandbox(secret string) *Sandbox {
	if secret == "" {
		secret = "sandbox-secret"
	}
	return &Sandbox{secret: secret}
}

// Name 渠道标识。
func (s *Sandbox) Name() string { return "sandbox" }

// Prepay 生成假的预支付单。
func (s *Sandbox) Prepay(_ context.Context, req PrepayRequest) (PrepayResult, error) {
	if req.OrderNo == "" || req.Amount <= 0 {
		return PrepayResult{}, fmt.Errorf("沙箱下单参数不合法")
	}
	return PrepayResult{
		Provider: s.Name(),
		PrepayID: "sandbox_" + req.OrderNo,
		Params: map[string]string{
			"prepay_id": "sandbox_" + req.OrderNo,
			"order_no":  req.OrderNo,
			"amount":    strconv.FormatInt(req.Amount, 10),
		},
		Mock:     true,
		MockHint: "沙箱环境，调用 /callback/sandbox/pay 模拟支付成功",
	}, nil
}

// sandboxNotifyBody 是沙箱回调报文结构。
type sandboxNotifyBody struct {
	OrderNo       string `json:"order_no"`
	TransactionID string `json:"transaction_id"`
	Amount        int64  `json:"amount"`
	Sign          string `json:"sign"`
}

// VerifyNotify 校验沙箱回调签名。
func (s *Sandbox) VerifyNotify(_ context.Context, _ http.Header, body []byte) (*Notify, error) {
	var b sandboxNotifyBody
	if err := json.Unmarshal(body, &b); err != nil {
		return nil, fmt.Errorf("回调报文解析失败: %w", err)
	}
	if b.OrderNo == "" || b.TransactionID == "" {
		return nil, fmt.Errorf("回调缺少订单号或交易号")
	}
	want := s.Sign(b.OrderNo, b.TransactionID, b.Amount)
	if !hmac.Equal([]byte(want), []byte(b.Sign)) {
		return nil, fmt.Errorf("回调签名校验失败")
	}
	return &Notify{
		OrderNo:       b.OrderNo,
		TransactionID: b.TransactionID,
		OutTradeNo:    b.OrderNo,
		Amount:        b.Amount,
		Success:       true,
		Raw:           body,
	}, nil
}

// Refund 沙箱退款直接成功。
func (s *Sandbox) Refund(_ context.Context, req RefundRequest) (RefundResult, error) {
	if req.OutRefundNo == "" || req.Amount <= 0 {
		return RefundResult{}, fmt.Errorf("沙箱退款参数不合法")
	}
	return RefundResult{
		RefundID:    "sandbox_refund_" + req.OutRefundNo,
		OutRefundNo: req.OutRefundNo,
		Success:     true,
	}, nil
}

// Sign 计算回调签名，供联调脚本与测试构造报文。
func (s *Sandbox) Sign(orderNo, transactionID string, amount int64) string {
	mac := hmac.New(sha256.New, []byte(s.secret))
	mac.Write([]byte(orderNo + "|" + transactionID + "|" + strconv.FormatInt(amount, 10)))
	return hex.EncodeToString(mac.Sum(nil))
}
