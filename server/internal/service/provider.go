package service

import (
	"context"

	"gorm.io/gorm"

	"github.com/alinger/community-handyman/server/internal/errs"
	"github.com/alinger/community-handyman/server/internal/model"
	"github.com/alinger/community-handyman/server/internal/repository"
)

// ProviderService 服务方端：接单、拒单、推进节点。
// V1 阶段服务方与运营可能由同一人承担，权限靠 users.role 与订单归属区分。
type ProviderService struct {
	db     *gorm.DB
	orders *repository.OrderRepo
}

func NewProviderService(db *gorm.DB, orders *repository.OrderRepo) *ProviderService {
	return &ProviderService{db: db, orders: orders}
}

// OpenOrders 可接单池：已支付且未派单的订单。
func (s *ProviderService) OpenOrders(ctx context.Context, serviceType string, page, size int) ([]model.Order, int64, error) {
	normalizePage(&page, &size)
	list, total, err := s.orders.ListOpenForProvider(ctx, serviceType, page, size)
	if err != nil {
		return nil, 0, errs.Wrap(errs.CodeInternal, err)
	}
	return list, total, nil
}

// MyOrders 服务方名下的订单。
func (s *ProviderService) MyOrders(ctx context.Context, providerID uint64, tab string, page, size int) ([]model.Order, int64, error) {
	normalizePage(&page, &size)
	list, total, err := s.orders.ListByProvider(ctx, providerID, tab, page, size)
	if err != nil {
		return nil, 0, errs.Wrap(errs.CodeInternal, err)
	}
	return list, total, nil
}

// Accept 接单。并发下靠「状态 + 未派单」条件更新，先到先得。
func (s *ProviderService) Accept(ctx context.Context, providerID uint64, orderNo string) error {
	o, err := s.orders.GetByNo(ctx, orderNo)
	if err != nil {
		return errs.Wrap(errs.CodeInternal, err)
	}
	if o == nil {
		return errs.Biz(errs.CodeNotFound)
	}
	if o.ProviderID != nil {
		return errs.BizMsg(errs.CodeIllegalStatus, "该订单已被其他服务方接单")
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ok, err := s.orders.WithTx(tx).UpdateStatus(ctx, o.ID,
			model.OrderPendingAccept, model.OrderAccepted,
			map[string]any{"provider_id": providerID})
		if err != nil {
			return errs.Wrap(errs.CodeInternal, err)
		}
		if !ok {
			return errs.BizMsg(errs.CodeIllegalStatus, "该订单已被其他服务方接单")
		}
		return s.orders.WithTx(tx).AddEvent(ctx, &model.OrderEvent{
			OrderID:    o.ID,
			Node:       string(model.OrderAccepted),
			Operator:   "provider",
			OperatorID: &providerID,
			Remark:     "服务方已接单",
		})
	})
}

// Reject 拒单，订单回到「已拒单」终态，由运营介入改约。
func (s *ProviderService) Reject(ctx context.Context, providerID uint64, orderNo, reason string) error {
	o, err := s.orders.GetByNo(ctx, orderNo)
	if err != nil {
		return errs.Wrap(errs.CodeInternal, err)
	}
	if o == nil {
		return errs.Biz(errs.CodeNotFound)
	}
	if o.Status != model.OrderPendingAccept {
		return errs.BizMsg(errs.CodeIllegalStatus, "只有待接单的订单可以拒单")
	}
	if reason == "" {
		reason = "服务方无暇接单"
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ok, err := s.orders.WithTx(tx).UpdateStatus(ctx, o.ID,
			model.OrderPendingAccept, model.OrderRejected,
			map[string]any{"cancel_reason": reason})
		if err != nil {
			return errs.Wrap(errs.CodeInternal, err)
		}
		if !ok {
			return errs.Biz(errs.CodeIllegalStatus)
		}
		return s.orders.WithTx(tx).AddEvent(ctx, &model.OrderEvent{
			OrderID:    o.ID,
			Node:       string(model.OrderRejected),
			Operator:   "provider",
			OperatorID: &providerID,
			Remark:     reason,
		})
	})
}

// Advance 推进节点。目标状态必须满足状态机白名单。
func (s *ProviderService) Advance(ctx context.Context, providerID uint64, orderNo, target string) error {
	o, err := s.orders.GetByNo(ctx, orderNo)
	if err != nil {
		return errs.Wrap(errs.CodeInternal, err)
	}
	if o == nil {
		return errs.Biz(errs.CodeNotFound)
	}
	if o.ProviderID == nil || *o.ProviderID != providerID {
		return errs.BizMsg(errs.CodeForbidden, "只能推进自己接的订单")
	}

	to := model.OrderStatus(target)
	if !to.Valid() {
		return errs.BizMsg(errs.CodeInvalidParam, "未知的节点: "+target)
	}
	if !model.CanTransit(o.Status, to) {
		return errs.BizMsg(errs.CodeIllegalStatus,
			"订单当前为「"+o.Status.Label()+"」，不能推进到「"+to.Label()+"」")
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ok, err := s.orders.WithTx(tx).UpdateStatus(ctx, o.ID, o.Status, to, nil)
		if err != nil {
			return errs.Wrap(errs.CodeInternal, err)
		}
		if !ok {
			return errs.Biz(errs.CodeIllegalStatus)
		}
		return s.orders.WithTx(tx).AddEvent(ctx, &model.OrderEvent{
			OrderID:    o.ID,
			Node:       string(to),
			Operator:   "provider",
			OperatorID: &providerID,
			Remark:     "服务方更新节点",
		})
	})
}

func normalizePage(page, size *int) {
	if *page <= 0 {
		*page = 1
	}
	if *size <= 0 || *size > 50 {
		*size = 10
	}
}
