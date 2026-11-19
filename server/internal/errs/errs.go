// Package errs 定义业务错误码与错误包装。
// 约定：HTTP 状态码只表达协议层结果，业务结果看 code。
package errs

import (
	"errors"
	"fmt"
	"net/http"
)

// 业务错误码。
const (
	CodeOK               = 0
	CodeInvalidParam     = 1001
	CodeUnauthorized     = 1002
	CodeForbidden        = 1003
	CodeNotFound         = 1004
	CodeAddressOutOfArea = 2001
	CodeSlotFull         = 2002
	CodeIllegalStatus    = 2003
	CodePayTimeout       = 2004
	CodePayFailed        = 3001
	CodeRefundFailed     = 3002
	CodeInternal         = 5000
)

var messages = map[int]string{
	CodeOK:               "成功",
	CodeInvalidParam:     "参数校验失败",
	CodeUnauthorized:     "未登录或登录已失效",
	CodeForbidden:        "无权限执行该操作",
	CodeNotFound:         "资源不存在",
	CodeAddressOutOfArea: "该地址暂未开通服务",
	CodeSlotFull:         "该时段已约满，请选择其他时段",
	CodeIllegalStatus:    "当前订单状态不允许此操作",
	CodePayTimeout:       "支付超时，订单已关闭",
	CodePayFailed:        "微信支付下单失败",
	CodeRefundFailed:     "退款失败",
	CodeInternal:         "服务异常，请稍后再试",
}

// BizError 是业务错误，可被 response.Fail 直接识别。
type BizError struct {
	Code int
	Msg  string
	Err  error // 底层错误，只进日志不出响应
}

func (e *BizError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%d] %s: %v", e.Code, e.Msg, e.Err)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Msg)
}

func (e *BizError) Unwrap() error { return e.Err }

// Biz 构造业务错误，使用错误码的默认文案。
func Biz(code int) *BizError {
	msg, ok := messages[code]
	if !ok {
		msg = messages[CodeInternal]
	}
	return &BizError{Code: code, Msg: msg}
}

// BizMsg 构造业务错误并覆盖默认文案。
func BizMsg(code int, msg string) *BizError {
	return &BizError{Code: code, Msg: msg}
}

// Wrap 把底层错误包进业务错误，底层原因只用于日志排查。
func Wrap(code int, err error) *BizError {
	e := Biz(code)
	e.Err = err
	return e
}

// As 提取业务错误。
func As(err error) (*BizError, bool) {
	var be *BizError
	if errors.As(err, &be) {
		return be, true
	}
	return nil, false
}

// HTTPStatus 给出错误码对应的 HTTP 状态码。
func HTTPStatus(code int) int {
	switch code {
	case CodeOK:
		return http.StatusOK
	case CodeInvalidParam:
		return http.StatusBadRequest
	case CodeUnauthorized:
		return http.StatusUnauthorized
	case CodeForbidden:
		return http.StatusForbidden
	case CodeNotFound:
		return http.StatusNotFound
	case CodePayFailed, CodeRefundFailed, CodeInternal:
		return http.StatusInternalServerError
	default:
		return http.StatusOK
	}
}

// Message 返回错误码默认文案。
func Message(code int) string {
	if msg, ok := messages[code]; ok {
		return msg
	}
	return messages[CodeInternal]
}
