package service

import (
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/alinger/community-handyman/server/internal/dto"
	"github.com/alinger/community-handyman/server/internal/model"
	"github.com/alinger/community-handyman/server/pkg/securefield"
)

// accessLeadTime 钥匙信息提前多久对服务方开放。
// 需求原文：仅服务开始前 2 小时至服务结束后可见。
const accessLeadTime = 2 * time.Hour

// slotRange 把「服务日期 + 时段码」解析成起止时刻。
//
// 时段码在历史数据里有两套写法，必须都认：
//   - "22:00-22:30"（代倒垃圾包月与额外时段）
//   - "08-10"（遛宠、陪诊）
func slotRange(day model.Date, code string) (start, end time.Time, ok bool) {
	d := day.Time()
	parts := strings.SplitN(strings.TrimSpace(code), "-", 2)
	if len(parts) != 2 {
		return time.Time{}, time.Time{}, false
	}
	sh, sm, ok1 := parseHourMinute(parts[0])
	eh, em, ok2 := parseHourMinute(parts[1])
	if !ok1 || !ok2 {
		return time.Time{}, time.Time{}, false
	}
	start = time.Date(d.Year(), d.Month(), d.Day(), sh, sm, 0, 0, d.Location())
	end = time.Date(d.Year(), d.Month(), d.Day(), eh, em, 0, 0, d.Location())
	if !end.After(start) {
		// 跨零点时段，例如 23:00-01:00
		end = end.Add(24 * time.Hour)
	}
	return start, end, true
}

// parseHourMinute 解析 "22:00" 或 "08"，返回时与分。
func parseHourMinute(s string) (h, m int, ok bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, 0, false
	}
	hm := strings.SplitN(s, ":", 2)
	h, err := strconv.Atoi(hm[0])
	if err != nil || h < 0 || h > 23 {
		return 0, 0, false
	}
	if len(hm) == 2 {
		m, err = strconv.Atoi(hm[1])
		if err != nil || m < 0 || m > 59 {
			return 0, 0, false
		}
	}
	return h, m, true
}

// accessWindowOpen 判断 now 是否落在钥匙信息的可见窗口内。
// 时段码解析失败一律判不可见：宁可让服务方多问一句，也不要提前泄露门禁密码。
func accessWindowOpen(day model.Date, code string, now time.Time) bool {
	start, end, ok := slotRange(day, code)
	if !ok {
		return false
	}
	return !now.Before(start.Add(-accessLeadTime)) && !now.After(end)
}

// applyAccessInfo 按查看者身份决定要不要把进入方式与门禁信息写进响应。
//
// 规则：
//   - 下单住户本人：始终可见（自己填的，没有隐藏的理由）；
//   - 履约服务方：只在 [服务开始前 2 小时, 服务结束] 内可见；
//   - 其他人（含运营后台）：完全不下发。
//
// 填了但还没到时间时下发 access_visible=false，
// 让前端能区分「没填」和「还不能看」，不至于显示一片空白。
func applyAccessInfo(resp *dto.OrderResp, o *model.Order, viewerID uint64,
	box *securefield.Box, now time.Time) {

	if o.EntryMethod == "" && o.AccessInfo == "" {
		return
	}
	isOwner := o.UserID != 0 && o.UserID == viewerID
	isProvider := o.ProviderID != nil && *o.ProviderID == viewerID
	if !isOwner && !isProvider {
		return
	}

	visible := isOwner || accessWindowOpen(o.ServiceDate, o.SlotCode, now)
	v := visible
	resp.AccessVisible = &v
	if !visible {
		return
	}

	resp.EntryMethod = o.EntryMethod
	if o.AccessInfo == "" {
		return
	}
	if !box.Enabled() {
		// 密钥没配时宁可不下发，也不能退化成明文
		slog.Warn("门禁信息加密组件未启用，跳过下发", "order_no", o.OrderNo)
		return
	}
	plain, err := box.Decrypt(o.AccessInfo)
	if err != nil {
		slog.Error("门禁信息解密失败，可能是密钥变更", "order_no", o.OrderNo, "err", err)
		return
	}
	resp.AccessInfo = plain
}

// encryptAccessInfo 加密门禁信息。密钥不可用时返回空串并记日志：
// 业务上「这单没填门禁密码」比「明文落库」可接受得多。
func encryptAccessInfo(box *securefield.Box, plain, orderNo string) string {
	plain = strings.TrimSpace(plain)
	if plain == "" {
		return ""
	}
	if !box.Enabled() {
		slog.Error("门禁信息加密组件未启用，已丢弃该字段", "order_no", orderNo)
		return ""
	}
	enc, err := box.Encrypt(plain)
	if err != nil {
		slog.Error("门禁信息加密失败，已丢弃该字段", "order_no", orderNo, "err", err)
		return ""
	}
	return enc
}
