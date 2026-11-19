package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"gorm.io/gorm"

	"github.com/alinger/community-handyman/server/internal/errs"
	"github.com/alinger/community-handyman/server/internal/model"
	"github.com/alinger/community-handyman/server/internal/payment"
	"github.com/alinger/community-handyman/server/internal/repository"
	"github.com/alinger/community-handyman/server/pkg/util"
)

// RefundService 退款。
// 两步提交：先落 pending 退款单（唯一号防重复），再调渠道，成功后回写状态与订单已退金额。
// 中途失败留下 pending 记录，供对账时人工处理，不会静默丢失。
type RefundService struct {
	db       *gorm.DB
	orders   *repository.OrderRepo
	slots    *repository.SlotRepo
	payments *repository.PaymentRepo
	gw       payment.Gateway
}

func NewRefundService(db *gorm.DB, orders *repository.OrderRepo, slots *repository.SlotRepo,
	payments *repository.PaymentRepo, gw payment.Gateway) *RefundService {
	return &RefundService{db: db, orders: orders, slots: slots, payments: payments, gw: gw}
}

// Refund 发起退款。amount 为 0 表示全额退剩余可退部分。
func (s *RefundService) Refund(ctx context.Context, orderNo string, amount int64, reason string,
	operatorID uint64, operator string) (*model.Refund, error) {

	o, err := s.orders.GetByNo(ctx, orderNo)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	if o == nil {
		return nil, errs.Biz(errs.CodeNotFound)
	}
	if o.PaidAmount <= 0 {
		return nil, errs.BizMsg(errs.CodeIllegalStatus, "该订单未支付，无需退款")
	}

	refundable := o.Refundable()
	if refundable <= 0 {
		return nil, errs.BizMsg(errs.CodeIllegalStatus, "该订单已全额退款")
	}
	if amount <= 0 {
		amount = refundable
	}
	if amount > refundable {
		return nil, errs.BizMsg(errs.CodeInvalidParam,
			fmt.Sprintf("退款金额超出可退上限，当前可退 %d 分", refundable))
	}

	rf := &model.Refund{
		OrderID:     o.ID,
		OutRefundNo: "RF" + o.OrderNo + util.RandomDigits(4),
		Amount:      amount,
		Reason:      reason,
		Status:      model.RefundPending,
		OperatorID:  operatorID,
	}

	// 先落单：out_refund_no 唯一，天然防止重复提交
	if err := s.payments.CreateRefund(ctx, rf); err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			return nil, errs.BizMsg(errs.CodeIllegalStatus, "该笔退款正在处理中，请勿重复提交")
		}
		return nil, errs.Wrap(errs.CodeInternal, err)
	}

	result, err := s.gw.Refund(ctx, payment.RefundRequest{
		OrderNo:     o.OrderNo,
		OutRefundNo: rf.OutRefundNo,
		Amount:      amount,
		Total:       o.PaidAmount,
		Reason:      reason,
	})
	if err != nil {
		// 渠道失败：标记失败并保留记录，方便运营重试与对账
		_ = s.payments.UpdateRefundStatus(ctx, rf.OutRefundNo, model.RefundFailed)
		return nil, errs.Wrap(errs.CodeRefundFailed, err)
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.payments.UpdateRefundStatus(ctx, rf.OutRefundNo, model.RefundSuccess); err != nil {
			return err
		}
		if err := tx.Model(&model.Order{}).Where("id = ?", o.ID).
			Update("refunded_amount", gorm.Expr("refunded_amount + ?", amount)).Error; err != nil {
			return err
		}

		// 全额退款且服务尚未履约：订单不能继续留在可接单池，否则服务方接了单却收不到钱
		if amount == refundable && !o.Status.Terminal() && o.Status != model.OrderCompleted {
			ok, uerr := s.orders.WithTx(tx).UpdateStatus(ctx, o.ID, o.Status, model.OrderCanceled,
				map[string]any{"cancel_reason": "全额退款，订单关闭"})
			if uerr != nil {
				return uerr
			}
			if ok {
				if rerr := s.slots.WithTx(tx).Release(ctx, o.ServiceDate, o.SlotCode, o.ServiceType); rerr != nil {
					return rerr
				}
				if eerr := s.orders.WithTx(tx).AddEvent(ctx, &model.OrderEvent{
					OrderID:    o.ID,
					Node:       string(model.OrderCanceled),
					Operator:   operator,
					OperatorID: &operatorID,
					Remark:     "全额退款，订单关闭并释放时段",
				}); eerr != nil {
					return eerr
				}
			}
		}

		return s.orders.WithTx(tx).AddEvent(ctx, &model.OrderEvent{
			OrderID:    o.ID,
			Node:       "refunded",
			Operator:   operator,
			OperatorID: &operatorID,
			Remark:     fmt.Sprintf("退款 %d 元：%s", amount/100, reason),
		})
	})
	if err != nil {
		slog.Error("退款已成功但本地记账失败，需人工核对",
			"out_refund_no", rf.OutRefundNo, "refund_id", result.RefundID, "err", err)
		return nil, errs.Wrap(errs.CodeInternal, err)
	}

	rf.Status = model.RefundSuccess
	return rf, nil
}
