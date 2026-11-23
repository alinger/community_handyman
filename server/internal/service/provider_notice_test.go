package service

import (
	"testing"

	"github.com/alinger/community-handyman/server/internal/model"
)

// TestAdvanceNoticeCoversFulfillment 履约链路上的每个节点都要有推送文案。
// 之前只有「完成」会发通知，出发上门/开始服务都是静默的，
// 住户只能靠手动刷新才知道服务方到哪一步了。
func TestAdvanceNoticeCoversFulfillment(t *testing.T) {
	cases := []struct {
		to    model.OrderStatus
		state string
	}{
		{model.OrderOnTheWay, "服务方已出发"},
		{model.OrderInService, "服务已开始"},
		{model.OrderPendingConfirm, "服务待确认"},
		{model.OrderCompleted, "服务已完成"},
	}
	for _, c := range cases {
		state, remark, ok := advanceNotice(c.to)
		if !ok {
			t.Fatalf("%s 应当有推送文案", c.to)
		}
		if state != c.state {
			t.Fatalf("%s 状态文案 = %q，期望 %q", c.to, state, c.state)
		}
		// 订阅消息 thing 字段上限 20 字，超了会被微信直接拒收
		if remark == "" || len([]rune(remark)) > 20 {
			t.Fatalf("%s 的备注长度不合法: %q", c.to, remark)
		}
	}
}

// TestAdvanceNoticeSkipsNonFulfillment 取消、拒单等不在履约推送范围内，
// 避免和各自路径里的通知重复发送。
func TestAdvanceNoticeSkipsNonFulfillment(t *testing.T) {
	for _, s := range []model.OrderStatus{
		model.OrderPendingPayment, model.OrderPendingAccept, model.OrderCanceled, model.OrderRejected,
	} {
		if _, _, ok := advanceNotice(s); ok {
			t.Fatalf("%s 不应触发履约推送", s)
		}
	}
}
