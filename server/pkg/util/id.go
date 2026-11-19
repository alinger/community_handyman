// Package util 提供无状态工具函数。
package util

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// NewRequestID 生成 16 字节随机串，用于 trace id。
func NewRequestID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand 不可用时退化为时间戳，保证链路日志仍有唯一标识
		return "t" + time.Now().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(b)
}

// ExtractFirstInt 取出字符串中的第一个连续数字，用于解析门牌号与路弄编号。
// "45号" -> 45；"1-9号" -> 1；"汇豪路71弄" -> 71；无数字返回 -1。
func ExtractFirstInt(s string) int {
	var buf strings.Builder
	started := false
	for _, r := range s {
		if r >= '0' && r <= '9' {
			buf.WriteRune(r)
			started = true
			continue
		}
		if started {
			break
		}
	}
	if buf.Len() == 0 {
		return -1
	}
	n, err := strconv.Atoi(buf.String())
	if err != nil {
		return -1
	}
	return n
}

// RandomDigits 生成 n 位随机数字串，用于订单号后缀。
func RandomDigits(n int) string {
	if n <= 0 {
		return ""
	}
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return strings.Repeat("0", n)
	}
	out := make([]byte, n)
	for i := range b {
		out[i] = '0' + b[i]%10
	}
	return string(out)
}

// SlotStart 把「日期 + 时段码」解析为服务开始时间。
// 时段码形如 08-10 / 12-14 / 18-20，取起始小时。
// 取消规则以它为基准：开始前 2 小时是免费取消与需审核的分界。
func SlotStart(date time.Time, slotCode string) (time.Time, error) {
	parts := strings.SplitN(strings.TrimSpace(slotCode), "-", 2)
	if len(parts) == 0 {
		return time.Time{}, fmt.Errorf("时段码为空")
	}
	hour, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || hour < 0 || hour > 23 {
		return time.Time{}, fmt.Errorf("时段码不合法: %s", slotCode)
	}
	y, m, d := date.Date()
	return time.Date(y, m, d, hour, 0, 0, 0, date.Location()), nil
}
