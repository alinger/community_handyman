package service

import (
	"context"
	"encoding/json"

	"gorm.io/gorm"

	"github.com/alinger/community-handyman/server/internal/errs"
	"github.com/alinger/community-handyman/server/internal/model"
	"github.com/alinger/community-handyman/server/internal/repository"
)

// QuoteService 陪诊报价。
// 陪诊不做自动定价：运营报价 → 用户确认并支付 → 报价单落 confirmed_at。
// 用户拒绝时关单并释放时段，避免占着库存又不成单。
type QuoteService struct {
	db     *gorm.DB
	orders *repository.OrderRepo
	quotes *repository.QuoteRepo
	slots  *repository.SlotRepo
}

func NewQuoteService(db *gorm.DB, orders *repository.OrderRepo,
	quotes *repository.QuoteRepo, slots *repository.SlotRepo) *QuoteService {
	return &QuoteService{db: db, orders: orders, quotes: quotes, slots: slots}
}

// Create 运营创建报价，同时把订单应付金额改为报价金额。
func (s *QuoteService) Create(ctx context.Context, adminID uint64, orderNo string,
	amount int64, durationMin int, note string) (*model.Quote, error) {

	if amount <= 0 {
		return nil, errs.BizMsg(errs.CodeInvalidParam, "报价金额必须大于 0")
	}

	o, err := s.orders.GetByNo(ctx, orderNo)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	if o == nil {
		return nil, errs.Biz(errs.CodeNotFound)
	}
	if o.ServiceType != model.ServiceAccompany {
		return nil, errs.BizMsg(errs.CodeInvalidParam, "只有陪诊订单需要人工报价")
	}
	if o.Status != model.OrderPendingPayment {
		return nil, errs.BizMsg(errs.CodeIllegalStatus, "订单已支付或已关闭，不能再报价")
	}

	// 已确认过的报价不允许覆盖，否则用户确认的价格会被悄悄改掉
	latest, err := s.quotes.LatestByOrder(ctx, o.ID)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	if latest != nil && latest.ConfirmedAt != nil {
		return nil, errs.BizMsg(errs.CodeIllegalStatus, "用户已确认报价，不能再修改")
	}

	q := &model.Quote{
		OrderID:     o.ID,
		Amount:      amount,
		DurationMin: durationMin,
		OperatorID:  adminID,
	}
	noteJSON, _ := json.Marshal(map[string]string{"note": note})
	q.NegotiationLog = noteJSON

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.quotes.WithTx(tx).Create(ctx, q); err != nil {
			return err
		}
		if err := tx.Model(&model.Order{}).Where("id = ?", o.ID).
			Update("total_amount", amount).Error; err != nil {
			return err
		}
		return s.orders.WithTx(tx).AddEvent(ctx, &model.OrderEvent{
			OrderID:    o.ID,
			Node:       "quoted",
			Operator:   "admin",
			OperatorID: &adminID,
			Remark:     "运营已报价，等待用户确认支付",
		})
	})
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	return q, nil
}

// Reject 用户拒绝报价，订单关闭并释放时段。
func (s *QuoteService) Reject(ctx context.Context, userID uint64, orderNo, reason string) error {
	o, err := s.orders.GetByNo(ctx, orderNo)
	if err != nil {
		return errs.Wrap(errs.CodeInternal, err)
	}
	if o == nil || o.UserID != userID {
		return errs.Biz(errs.CodeNotFound)
	}
	if o.Status != model.OrderPendingPayment {
		return errs.BizMsg(errs.CodeIllegalStatus, "订单已支付或已关闭，无法拒绝报价")
	}

	latest, err := s.quotes.LatestByOrder(ctx, o.ID)
	if err != nil {
		return errs.Wrap(errs.CodeInternal, err)
	}
	if latest == nil {
		return errs.BizMsg(errs.CodeIllegalStatus, "该订单还没有报价")
	}

	if reason == "" {
		reason = "用户拒绝报价"
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ok, uerr := s.orders.WithTx(tx).UpdateStatus(ctx, o.ID, model.OrderPendingPayment,
			model.OrderCanceled, map[string]any{"cancel_reason": reason})
		if uerr != nil {
			return errs.Wrap(errs.CodeInternal, uerr)
		}
		if !ok {
			return errs.Biz(errs.CodeIllegalStatus)
		}
		if rerr := s.slots.WithTx(tx).Release(ctx, o.ServiceDate, o.SlotCode, o.ServiceType); rerr != nil {
			return errs.Wrap(errs.CodeInternal, rerr)
		}
		return s.orders.WithTx(tx).AddEvent(ctx, &model.OrderEvent{
			OrderID:    o.ID,
			Node:       string(model.OrderCanceled),
			Operator:   "user",
			OperatorID: &userID,
			Remark:     reason,
		})
	})
}

// Latest 取订单最新报价，供详情页展示。
func (s *QuoteService) Latest(ctx context.Context, orderID uint64) (*model.Quote, error) {
	q, err := s.quotes.LatestByOrder(ctx, orderID)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	return q, nil
}
