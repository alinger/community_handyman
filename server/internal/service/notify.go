package service

import (
	"context"
	"log/slog"

	"github.com/alinger/community-handyman/server/internal/repository"
	"github.com/alinger/community-handyman/server/internal/wxauth"
)

// NotifyService 订单状态通知（微信订阅消息）。
//
// 设计取舍：通知是「尽力而为」的旁路——发送失败不影响主流程，
// 用户未订阅（43101）更是常态。所以这里只记日志，绝不向上抛错。
type NotifyService struct {
	wx         *wxauth.Client
	users      *repository.UserRepo
	templateID string
}

func NewNotifyService(wx *wxauth.Client, users *repository.UserRepo, templateID string) *NotifyService {
	return &NotifyService{wx: wx, users: users, templateID: templateID}
}

// OrderEvent 用户订单状态变化时发送订阅消息。
// templateID 未配置时打日志跳过：等小程序后台申请好模板、配上环境变量即可生效。
func (s *NotifyService) OrderEvent(ctx context.Context, userID uint64, orderNo, state, remark string) {
	if s.templateID == "" {
		slog.Info("订阅消息模板未配置，跳过通知",
			"user_id", userID, "order_no", orderNo, "state", state)
		return
	}

	u, err := s.users.GetByID(ctx, userID)
	if err != nil || u == nil {
		slog.Warn("通知取用户失败", "user_id", userID, "err", err)
		return
	}

	data := map[string]string{
		"thing1": truncate(orderNo, 20),
		"thing2": truncate(state, 20),
		"thing3": truncate(remark, 20),
	}
	err = s.wx.SendSubscribe(ctx, u.OpenID, s.templateID,
		"pages/order/detail?orderNo="+orderNo, data)
	if err != nil {
		if we, ok := err.(*wxauth.Error); ok && we.Code == wxauth.ErrCodeNotSubscribed {
			slog.Info("用户未订阅消息，跳过", "user_id", userID, "order_no", orderNo)
			return
		}
		slog.Error("发送订阅消息失败", "user_id", userID, "order_no", orderNo, "err", err)
	}
}

// truncate 订阅消息 thing 类型字段上限 20 字符，超长会直接被微信拒绝。
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
