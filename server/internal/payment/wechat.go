package payment

import (
	"bytes"
	"context"
	"fmt"
	"net/http"

	core2 "github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/core/auth/verifiers"
	"github.com/wechatpay-apiv3/wechatpay-go/core/downloader"
	"github.com/wechatpay-apiv3/wechatpay-go/core/notify"
	"github.com/wechatpay-apiv3/wechatpay-go/core/option"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/jsapi"
	"github.com/wechatpay-apiv3/wechatpay-go/services/refunddomestic"
	"github.com/wechatpay-apiv3/wechatpay-go/utils"
)

// WechatOptions 微信支付渠道参数，来自 config.WechatConfig（yaml wechat 段 + 环境变量）。
type WechatOptions struct {
	AppID          string // 小程序 appid（JSAPI 支付的 appid 与 payer.openid 必须同主体）
	MchID          string
	SerialNo       string // 商户 API 证书序列号
	PrivateKeyPath string // 商户 API 私钥 PEM 文件路径
	APIv3Key       string
	NotifyURL      string // 支付结果回调，须为公网 HTTPS（如 https://域名/api/v1/callback/wechat/pay）
}

// Wechat 微信支付 APIv3 渠道实现（JSAPI / 小程序支付）。
//
// 平台证书用 SDK 的自动下载器：启动时拉取并周期刷新平台证书，
// 回调验签从 downloader 管理器取与商户号对应的实时证书访问器。
// 商户号下来后只需在环境变量里填好 wechat 段配置并把 PAYMENT_PROVIDER 切成 wechat。
type Wechat struct {
	client *core2.Client
	opts   WechatOptions
}

// NewWechat 创建微信支付渠道。参数不完整或私钥/证书不可用时返回错误，
// 启动即失败，避免带病运行到收单环节才暴露。
func NewWechat(opts WechatOptions) (*Wechat, error) {
	if opts.AppID == "" || opts.MchID == "" || opts.SerialNo == "" || opts.APIv3Key == "" ||
		opts.PrivateKeyPath == "" || opts.NotifyURL == "" {
		return nil, fmt.Errorf("微信支付配置不完整：需要 wechat.app_id / mch_id / serial_no / api_v3_key / private_key_path / notify_url")
	}
	privKey, err := utils.LoadPrivateKeyWithPath(opts.PrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("加载商户 API 私钥失败(%s): %w", opts.PrivateKeyPath, err)
	}
	// WithWechatPayAutoAuthCipher 内部注册平台证书自动下载器（启动即拉取，失败报错），
	// 同时给 client 配齐「请求签名 / 响应验签 / 敏感字段加解密」
	ctx := context.Background()
	client, err := core2.NewClient(ctx,
		option.WithWechatPayAutoAuthCipher(opts.MchID, opts.SerialNo, privKey, opts.APIv3Key),
	)
	if err != nil {
		return nil, fmt.Errorf("初始化微信支付客户端失败: %w", err)
	}
	return &Wechat{client: client, opts: opts}, nil
}

func (w *Wechat) Name() string { return "wechat" }

// Prepay JSAPI 统一下单并生成小程序调起支付的参数（timeStamp/nonceStr/package/signType/paySign）。
// JSAPI 支付强制要求 payer.openid，纯手机号注册（未微信登录）的用户无法发起。
func (w *Wechat) Prepay(ctx context.Context, req PrepayRequest) (PrepayResult, error) {
	if req.OpenID == "" {
		return PrepayResult{}, fmt.Errorf("微信支付需要用户 openid：请引导用户使用微信登录后重试")
	}
	svc := jsapi.JsapiApiService{Client: w.client}
	resp, _, err := svc.PrepayWithRequestPayment(ctx, jsapi.PrepayRequest{
		Appid:       core2.String(w.opts.AppID),
		Mchid:       core2.String(w.opts.MchID),
		Description: core2.String(req.Description),
		OutTradeNo:  core2.String(req.OrderNo),
		NotifyUrl:   core2.String(w.opts.NotifyURL),
		Amount:      &jsapi.Amount{Total: core2.Int64(req.Amount), Currency: core2.String("CNY")},
		Payer:       &jsapi.Payer{Openid: core2.String(req.OpenID)},
	})
	if err != nil {
		return PrepayResult{}, fmt.Errorf("微信下单失败: %w", err)
	}
	return PrepayResult{
		Provider: w.Name(),
		PrepayID: deref(resp.PrepayId),
		Params: map[string]string{
			"appId":     w.opts.AppID,
			"timeStamp": deref(resp.TimeStamp),
			"nonceStr":  deref(resp.NonceStr),
			"package":   deref(resp.Package),
			"signType":  deref(resp.SignType),
			"paySign":   deref(resp.PaySign),
		},
	}, nil
}

// VerifyNotify 验签并解密回调报文。验签失败返回错误，渠道会重推。
// SDK 的 ParseNotifyRequest 需要 *http.Request（读 Wechatpay-* 头与报文体），
// 而 Gateway 接口约定传 header+body，这里按约定重建一个无 URL 的请求对象。
func (w *Wechat) VerifyNotify(ctx context.Context, header http.Header, body []byte) (*Notify, error) {
	req, err := http.NewRequest(http.MethodPost, "", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header = header

	// 证书访问器随自动下载器实时刷新，验签始终用微信侧最新平台证书
	verifier := verifiers.NewSHA256WithRSAVerifier(
		downloader.MgrInstance().GetCertificateVisitor(w.opts.MchID),
	)
	handler, err := notify.NewRSANotifyHandler(w.opts.APIv3Key, verifier)
	if err != nil {
		return nil, err
	}

	var tx payments.Transaction
	if _, err := handler.ParseNotifyRequest(ctx, req, &tx); err != nil {
		return nil, fmt.Errorf("微信回调验签/解密失败: %w", err)
	}

	var amount int64
	if tx.Amount != nil && tx.Amount.Total != nil {
		amount = *tx.Amount.Total
	}
	success := tx.TradeState != nil && *tx.TradeState == "SUCCESS"
	return &Notify{
		OrderNo:       deref(tx.OutTradeNo),
		TransactionID: deref(tx.TransactionId),
		OutTradeNo:    deref(tx.OutTradeNo),
		Amount:        amount,
		Success:       success,
		Raw:           body,
	}, nil
}

// Refund 发起退款。退款是异步终态：SUCCESS/PROCESSING（已受理）都视为发起成功，
// 最终结果以渠道为准；业务侧拒单退款按此口径放行。
func (w *Wechat) Refund(ctx context.Context, req RefundRequest) (RefundResult, error) {
	svc := refunddomestic.RefundsApiService{Client: w.client}
	refund, total, currency := int64(req.Amount), int64(req.Total), "CNY"
	reason, outRefundNo, outTradeNo := req.Reason, req.OutRefundNo, req.OrderNo
	resp, _, err := svc.Create(ctx, refunddomestic.CreateRequest{
		OutTradeNo:  &outTradeNo,
		OutRefundNo: &outRefundNo,
		Reason:      &reason,
		Amount:      &refunddomestic.AmountReq{Refund: &refund, Total: &total, Currency: &currency},
	})
	if err != nil {
		return RefundResult{}, fmt.Errorf("微信退款失败: %w", err)
	}
	status := ""
	if resp.Status != nil {
		status = string(*resp.Status)
	}
	success := status == string(refunddomestic.STATUS_SUCCESS) || status == string(refunddomestic.STATUS_PROCESSING)
	return RefundResult{
		RefundID:    deref(resp.RefundId),
		OutRefundNo: deref(resp.OutRefundNo),
		Success:     success,
	}, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
