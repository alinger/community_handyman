package sms

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	aliyunAPIVersion = "2017-05-25"
	aliyunDefaultURL = "https://dysmsapi.aliyuncs.com/"
)

// AliyunSender 阿里云短信（Dysmsapi RPC 风格接口）。
//
// 这里按官方 RPC 签名规则自己实现，不引入 alibabacloud-go 全家桶：
// 只调一个 SendSms，多一层 SDK 依赖会拖大编译产物与依赖面。
// 签名规则见 https://help.aliyun.com/zh/sms/developer-reference/api-dysmsapi-2017-05-25-sendsms
type AliyunSender struct {
	accessKeyID     string
	accessKeySecret string
	signName        string
	templateCode    string
	endpoint        string
	client          *http.Client
}

// NewAliyunSender 构造阿里云渠道。
func NewAliyunSender(opts Options) *AliyunSender {
	endpoint := opts.Endpoint
	if endpoint == "" {
		endpoint = aliyunDefaultURL
	}
	return &AliyunSender{
		accessKeyID:     opts.AccessKeyID,
		accessKeySecret: opts.AccessKeySecret,
		signName:        opts.SignName,
		templateCode:    opts.TemplateCode,
		endpoint:        endpoint,
		client:          &http.Client{Timeout: 6 * time.Second},
	}
}

func (s *AliyunSender) Name() string { return "aliyun" }

// aliyunResp 阿里云通用响应。Code 为 OK 表示成功。
type aliyunResp struct {
	Code      string `json:"Code"`
	Message   string `json:"Message"`
	BizID     string `json:"BizId"`
	RequestID string `json:"RequestId"`
}

// SendCode 下发验证码，模板变量名固定为 code。
func (s *AliyunSender) SendCode(ctx context.Context, phone, code string) error {
	param, err := json.Marshal(map[string]string{"code": code})
	if err != nil {
		return fmt.Errorf("序列化模板参数失败: %w", err)
	}
	_, err = s.call(ctx, "SendSms", map[string]string{
		"PhoneNumbers":  phone,
		"SignName":      s.signName,
		"TemplateCode":  s.templateCode,
		"TemplateParam": string(param),
	})
	return err
}

// QuerySendDetails 查询某天的下发记录。仅用于接入自检与排障，不参与业务。
func (s *AliyunSender) QuerySendDetails(ctx context.Context, phone, sendDate string) (*aliyunResp, error) {
	return s.call(ctx, "QuerySendDetails", map[string]string{
		"PhoneNumber": phone,
		"SendDate":    sendDate,
		"PageSize":    "10",
		"CurrentPage": "1",
	})
}

// call 发起一次 RPC 调用。公共参数在此统一拼装，业务参数由调用方提供。
func (s *AliyunSender) call(ctx context.Context, action string, biz map[string]string) (*aliyunResp, error) {
	params := map[string]string{
		"AccessKeyId":      s.accessKeyID,
		"Action":           action,
		"Format":           "JSON",
		"RegionId":         "cn-hangzhou",
		"SignatureMethod":  "HMAC-SHA1",
		"SignatureNonce":   randomNonce(),
		"SignatureVersion": "1.0",
		"Timestamp":        time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		"Version":          aliyunAPIVersion,
	}
	for k, v := range biz {
		params[k] = v
	}
	reqURL := s.endpoint + "?Signature=" + percentEncode(s.sign(params)) + "&" + canonicalQuery(params)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("构造短信请求失败: %w", err)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("调用阿里云短信失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err != nil {
		return nil, fmt.Errorf("读取短信响应失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("阿里云短信返回 HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	var out aliyunResp
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("解析短信响应失败: %s", truncate(string(body), 200))
	}
	if !strings.EqualFold(out.Code, "OK") {
		return &out, fmt.Errorf("阿里云短信返回失败: action=%s code=%s message=%s requestId=%s",
			action, out.Code, out.Message, out.RequestID)
	}
	return &out, nil
}

// sign 计算 RPC 签名：HMAC-SHA1(AccessKeySecret + "&", StringToSign)，再 Base64。
func (s *AliyunSender) sign(params map[string]string) string {
	stringToSign := http.MethodGet + "&%2F&" + percentEncode(canonicalQuery(params))
	mac := hmac.New(sha1.New, []byte(s.accessKeySecret+"&"))
	mac.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// canonicalQuery 按参数名 ASCII 升序拼接 k=v，键值均需百分号编码。
func canonicalQuery(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte('&')
		}
		b.WriteString(percentEncode(k))
		b.WriteByte('=')
		b.WriteString(percentEncode(params[k]))
	}
	return b.String()
}

// percentEncode 阿里云要求的 RFC3986 编码：空格为 %20、* 为 %2A、~ 保留。
func percentEncode(s string) string {
	e := url.QueryEscape(s)
	e = strings.ReplaceAll(e, "+", "%20")
	e = strings.ReplaceAll(e, "*", "%2A")
	e = strings.ReplaceAll(e, "%7E", "~")
	return e
}

func randomNonce() string {
	// 阿里云要求每次请求唯一；纳秒时间戳叠加随机数，重试也不会撞值
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
