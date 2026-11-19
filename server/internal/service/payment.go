package service

import (
	"context"
	"errors"
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

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		pt := &model.PaymentTransaction{
			OrderNo:       notify.OrderNo,
			TransactionID: notify.TransactionID,
			OutTradeNo:    notify.OutTradeNo,
			Amount:        notify.Amount,
			Status:        model.PayStatusSuccess,
			RawCallback:   body,
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
				"order_no", o.OrderNo, "status", o.Status, "transaction_id", notify.TransactionID)
			return nil
		}

		ok, err := s.orders.WithTx(tx).UpdateStatus(ctx, o.ID,
			model.OrderPendingPayment, model.OrderPendingAccept,
			map[string]any{
				"paid_amount": notify.Amount,
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

func timePtr(t time.Time) *time.Time { return &t }
