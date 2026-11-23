package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"gorm.io/gorm"

	"github.com/alinger/community-handyman/server/internal/errs"
	"github.com/alinger/community-handyman/server/internal/model"
	"github.com/alinger/community-handyman/server/internal/payment"
	"github.com/alinger/community-handyman/server/internal/repository"
)

// PaymentService 处理支付回调。
// 幂等依赖 payment_transactions.transaction_id 唯一索引，重复回调不会重复入账。
type PaymentService struct {
	db       *gorm.DB
	orders   *repository.OrderRepo
	payments *repository.PaymentRepo
	gw       payment.Gateway
}

func NewPaymentService(db *gorm.DB, orders *repository.OrderRepo,
	payments *repository.PaymentRepo, gw payment.Gateway) *PaymentService {
	return &PaymentService{db: db, orders: orders, payments: payments, gw: gw}
}

// HandleNotify 处理渠道回调。返回 nil 表示已受理，渠道可停止重推。
func (s *PaymentService) HandleNotify(ctx context.Context, header http.Header, body []byte) error {
	notify, err := s.gw.VerifyNotify(ctx, header, body)
	if err != nil {
		return errs.Wrap(errs.CodeInvalidParam, err)
	}
	if !notify.Success {
		slog.Warn("收到非成功支付回调", "order_no", notify.OrderNo, "transaction_id", notify.TransactionID)
		return nil
	}

	o, err := s.orders.GetByNo(ctx, notify.OrderNo)
	if err != nil {
		return errs.Wrap(errs.CodeInternal, err)
	}
	if o == nil {
		return errs.BizMsg(errs.CodeNotFound, "订单不存在")
	}

	// 金额必须与订单一致，防止渠道侧金额被篡改或串单
	if notify.Amount != o.TotalAmount {
		slog.Error("支付金额与订单不一致",
			"order_no", o.OrderNo, "order_amount", o.TotalAmount, "notify_amount", notify.Amount)
		return errs.BizMsg(errs.CodeInvalidParam, "支付金额与订单不一致")
	}

	return s.applyPaid(ctx, o, notify.TransactionID, notify.OutTradeNo, notify.Amount, body)
}

// applyPaid 把订单推进到「待接单」并记一条支付流水。
//
// 微信回调和「试运行期跳过支付」共用这一段：
// 幂等靠 transaction_id 唯一索引，状态推进靠「待支付 → 待接单」的 CAS 更新，
// 因此两条入口的语义完全一致，只是流水号来源不同。
func (s *PaymentService) applyPaid(ctx context.Context, o *model.Order,
	transactionID, outTradeNo string, amount int64, raw []byte) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		pt := &model.PaymentTransaction{
			OrderNo:       o.OrderNo,
			TransactionID: transactionID,
			OutTradeNo:    outTradeNo,
			Amount:        amount,
			Status:        model.PayStatusSuccess,
			RawCallback:   raw,
			PaidAt:        timePtr(time.Now()),
		}
		if err := s.payments.WithTx(tx).InsertTransaction(ctx, pt); err != nil {
			if errors.Is(err, repository.ErrDuplicate) {
				// 已处理过的回调，直接确认，避免渠道无限重推
				return nil
			}
			return errs.Wrap(errs.CodeInternal, err)
		}

		// 订单可能已被超时关单：流水已记，但状态不再推进，需人工退款
		if o.Status != model.OrderPendingPayment {
			slog.Error("支付成功但订单已不在待支付状态，需人工处理",
				"order_no", o.OrderNo, "status", o.Status, "transaction_id", transactionID)
			return nil
		}

		ok, err := s.orders.WithTx(tx).UpdateStatus(ctx, o.ID,
			model.OrderPendingPayment, model.OrderPendingAccept,
			map[string]any{
				"paid_amount": amount,
				"paid_at":     time.Now(),
			})
		if err != nil {
			return errs.Wrap(errs.CodeInternal, err)
		}
		if !ok {
			// 并发下状态已被改，流水已入账，视为成功
			return nil
		}

		return s.orders.WithTx(tx).AddEvent(ctx, &model.OrderEvent{
			OrderID:  o.ID,
			Node:     string(model.OrderPendingAccept),
			Operator: "system",
			Remark:   "支付成功，等待服务方接单",
		})
	})
}

// SkipPay 试运行期跳过支付：下单后直接按「支付成功」推进订单。
//
// 为什么不能只在前端跳过：订单停在「待支付」会被 15 分钟超时任务关单，
// 时段也会被释放。所以必须走服务端的状态推进，只是把流水号换成 skip_ 前缀，
// 财务对账时一眼能看出这笔不是真实支付（接微信支付后即消失）。
func (s *PaymentService) SkipPay(ctx context.Context, orderNo string) error {
	o, err := s.orders.GetByNo(ctx, orderNo)
	if err != nil {
		return errs.Wrap(errs.CodeInternal, err)
	}
	if o == nil {
		return errs.Biz(errs.CodeNotFound)
	}
	// 已经不在待支付（重复调用或免支付订单）时什么都不做
	if o.Status != model.OrderPendingPayment {
		return nil
	}
	// 0 元订单没有支付环节，直接把状态推过去，不记流水
	if o.TotalAmount <= 0 {
		ok, err := s.orders.UpdateStatus(ctx, o.ID,
			model.OrderPendingPayment, model.OrderPendingAccept,
			map[string]any{"paid_at": time.Now()})
		if err != nil {
			return errs.Wrap(errs.CodeInternal, err)
		}
		if !ok {
			return nil
		}
		return s.orders.AddEvent(ctx, &model.OrderEvent{
			OrderID:  o.ID,
			Node:     string(model.OrderPendingAccept),
			Operator: "system",
			Remark:   "0 元订单，无需支付",
		})
	}

	raw := []byte(`{"channel":"skip","reason":"试运行期跳过支付，未发生真实扣款"}`)
	return s.applyPaid(ctx, o, "skip_"+o.OrderNo, "SKIP-"+o.OrderNo, o.TotalAmount, raw)
}

// MockPay 模拟一笔支付成功，仅沙箱渠道可用，且只在 dev 环境注册路由。
// 前端开发阶段没有商户号，靠它走完「下单 → 回调 → 状态推进」整条链路。
func (s *PaymentService) MockPay(ctx context.Context, orderNo string) error {
	sb, ok := s.gw.(*payment.Sandbox)
	if !ok {
		return errs.BizMsg(errs.CodeInvalidParam, "当前支付渠道不支持模拟支付")
	}

	o, err := s.orders.GetByNo(ctx, orderNo)
	if err != nil {
		return errs.Wrap(errs.CodeInternal, err)
	}
	if o == nil {
		return errs.Biz(errs.CodeNotFound)
	}
	if o.TotalAmount <= 0 {
		return errs.BizMsg(errs.CodeIllegalStatus, "订单金额为 0，无需支付")
	}

	txid := "mock_" + o.OrderNo
	body := []byte(fmt.Sprintf(`{"order_no":%q,"transaction_id":%q,"amount":%d,"sign":%q}`,
		o.OrderNo, txid, o.TotalAmount, sb.Sign(o.OrderNo, txid, o.TotalAmount)))

	return s.HandleNotify(ctx, http.Header{}, body)
}

func timePtr(t time.Time) *time.Time { return &t }
