package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/alinger/community-handyman/server/internal/dto"

	"gorm.io/gorm"

	"github.com/alinger/community-handyman/server/internal/errs"
	"github.com/alinger/community-handyman/server/internal/model"
	"github.com/alinger/community-handyman/server/internal/repository"
)

// ProviderService 服务方端：接单、拒单、推进节点。
// V1 阶段服务方与运营可能由同一人承担，权限靠 users.role 与订单归属区分。
type ProviderService struct {
	db        *gorm.DB
	orders    *repository.OrderRepo
	slots     *repository.SlotRepo
	quotas    *repository.QuotaRepo
	income    *repository.IncomeRepo
	users     *repository.UserRepo
	notify    *NotifyService
	refundSvc *RefundService
}

func NewProviderService(db *gorm.DB, orders *repository.OrderRepo, slots *repository.SlotRepo,
	quotas *repository.QuotaRepo, income *repository.IncomeRepo, users *repository.UserRepo,
	notify *NotifyService, refundSvc *RefundService) *ProviderService {
	return &ProviderService{db: db, orders: orders, slots: slots, quotas: quotas,
		income: income, users: users, notify: notify, refundSvc: refundSvc}
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
	// 派单即刻生效后订单落在「已受理」，所以未开始服务的两个状态都允许拒。
	// 一旦开始服务就只能走完成流程，不能让服务方中途甩单。
	if o.Status != model.OrderPendingAccept && o.Status != model.OrderAccepted {
		return errs.BizMsg(errs.CodeIllegalStatus, "只有尚未开始服务的订单可以拒单")
	}
	// 只能拒指派给自己的订单。池子里未派单的单不需要「拒」——不接就行了，
	// 拒掉会让住户的订单直接作废并触发退款，等于给了任何人瘫痪业务的开关。
	if o.ProviderID == nil || *o.ProviderID != providerID {
		return errs.BizMsg(errs.CodeForbidden, "只能拒绝指派给自己的订单")
	}
	if reason == "" {
		reason = "服务方无暇接单"
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ok, err := s.orders.WithTx(tx).UpdateStatus(ctx, o.ID,
			o.Status, model.OrderRejected,
			map[string]any{"cancel_reason": reason})
		if err != nil {
			return errs.Wrap(errs.CodeInternal, err)
		}
		if !ok {
			return errs.Biz(errs.CodeIllegalStatus)
		}
		// 单次订单建单时占用的时段要在拒单时释放（包月不占库存，跳过）
		if o.Plan != model.PlanMonthly {
			if rerr := s.slots.WithTx(tx).Release(ctx, o.ServiceDate, o.SlotCode, o.ServiceType); rerr != nil {
				return errs.Wrap(errs.CodeInternal, rerr)
			}
		}
		// 免费次数抵扣的单被拒，把次数还回去，用户不该为没发生的服务消耗额度
		if o.FreeQuotaUsed {
			if rerr := s.quotas.WithTx(tx).Restore(ctx, o.UserID, o.ServiceType); rerr != nil {
				return errs.Wrap(errs.CodeInternal, rerr)
			}
		}
		return s.orders.WithTx(tx).AddEvent(ctx, &model.OrderEvent{
			OrderID:    o.ID,
			Node:       string(model.OrderRejected),
			Operator:   "provider",
			OperatorID: &providerID,
			Remark:     reason,
		})
	})
	if err != nil {
		return err
	}

	// 已支付订单拒单必须自动退款（流程图「退款后关闭」分支）。
	// 退款失败不回滚拒单：订单已关闭、时段已释放，回滚只会让状态更乱；
	// 与 Refund 的既有约定一致——记 Error 日志交运营人工核对。
	remark := "服务方未能上门，请联系客服改期或退款"
	if o.PaidAmount > 0 {
		if _, rerr := s.refundSvc.Refund(ctx, orderNo, o.PaidAmount,
			"服务方拒单，自动退款", providerID, "provider"); rerr != nil {
			slog.Error("拒单自动退款失败，需人工核对",
				"order_no", orderNo, "provider_id", providerID, "amount", o.PaidAmount, "err", rerr)
			remark = "服务方未能上门，退款处理中，请稍后查收"
		} else {
			remark = "服务方未能上门，费用已原路退回"
		}
	}
	s.notify.OrderEvent(ctx, o.UserID, o.OrderNo, "服务方已拒单", remark)
	return nil
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
	// 月卡不走单次的「出发 → 开始 → 完成」流程：一张订单跑满 30 天，
	// 中途点「完成服务」会让月卡提前结束，剩下的天数就没人上门了。
	// 服务期内只允许每日打卡，服务期结束后才放行完成结算。
	if o.Plan == model.PlanMonthly {
		_, end := monthlyRange(o)
		if !model.NewDate(time.Now()).Time().After(end.Time()) {
			switch to {
			case model.OrderOnTheWay, model.OrderInService, model.OrderPendingConfirm:
				return errs.BizMsg(errs.CodeIllegalStatus,
					"包月服务期至 "+end.String()+"，请每天在任务里打卡，服务期结束后再提交完成")
			}
		}
	}
	// 垃圾代倒一次上门就结束：服务方点「完成服务」直接进终态。
	// 遛宠/陪诊不能这样跳——那两类有「送回宠物」「陪诊结束交接」的验收环节，
	// 绕过去等于住户连点开看一眼的机会都没有。
	if to == model.OrderCompleted && o.Status == model.OrderInService &&
		o.ServiceType != model.ServiceGarbage {
		return errs.BizMsg(errs.CodeIllegalStatus, "该服务需住户确认完成，请先提交完成")
	}
	if !model.CanTransit(o.Status, to) {
		return errs.BizMsg(errs.CodeIllegalStatus,
			"订单当前为「"+o.Status.Label()+"」，不能推进到「"+to.Label()+"」")
	}

	// 完成要有完成时间：住户确认那条路径一直在写，服务方直接完成这条不能漏，
	// 否则订单详情与财务报表里「完成时间」是空的。
	var extra map[string]any
	if to == model.OrderCompleted {
		extra = map[string]any{"finished_at": time.Now()}
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ok, err := s.orders.WithTx(tx).UpdateStatus(ctx, o.ID, o.Status, to, extra)
		if err != nil {
			return errs.Wrap(errs.CodeInternal, err)
		}
		if !ok {
			return errs.Biz(errs.CodeIllegalStatus)
		}
		if err := s.orders.WithTx(tx).AddEvent(ctx, &model.OrderEvent{
			OrderID:    o.ID,
			Node:       string(to),
			Operator:   "provider",
			OperatorID: &providerID,
			Remark:     advanceRemark(o.ServiceType, to),
		}); err != nil {
			return err
		}

		// 服务方直接完成订单（与用户确认等效）也要入账，两条路径不能只记一条
		if to == model.OrderCompleted {
			if ierr := s.income.WithTx(tx).Create(ctx, &model.ProviderIncome{
				ProviderID: providerID,
				OrderID:    o.ID,
				OrderNo:    o.OrderNo,
				Amount:     o.PaidAmount,
			}); ierr != nil {
				return errs.Wrap(errs.CodeInternal, ierr)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	if state, remark, ok := advanceNotice(to); ok {
		s.notify.OrderEvent(ctx, o.UserID, o.OrderNo, state, remark)
	}
	return nil
}

// advanceRemark 节点事件上的人话备注。
//
// 垃圾代倒「出发上门」直接落到服务中、点「完成服务」直接终态，
// 时间轴如果照写「服务方更新节点」，出了纠纷看不出到底是哪个动作。
func advanceRemark(st model.ServiceType, to model.OrderStatus) string {
	switch {
	case to == model.OrderInService && st == model.ServiceGarbage:
		return "服务方上门，开始服务"
	case to == model.OrderCompleted && st == model.ServiceGarbage:
		return "服务方完成服务"
	}
	return "服务方更新节点"
}

// advanceNotice 返回节点推进后发给住户的推送文案。
//
// 只覆盖住户真正关心的节点：出发上门（可以准备门禁/把垃圾放门口）、
// 开始服务、待确认、已完成。取消/拒单等由各自路径负责，不在这里兜。
// 通知是尽力而为的旁路，模板未配置时 NotifyService 只记日志。
func advanceNotice(to model.OrderStatus) (string, string, bool) {
	switch to {
	case model.OrderOnTheWay:
		return "服务方已出发", "服务方正在前往，请留意门禁与来电", true
	case model.OrderInService:
		return "服务已开始", "服务方已到场，服务进行中", true
	case model.OrderPendingConfirm:
		return "服务待确认", "服务方已完成，请确认验收", true
	case model.OrderCompleted:
		return "服务已完成", "服务已完成，可查看服务报告", true
	}
	return "", "", false
}

// Income 服务方收入汇总与明细。
// 金额一律来自已完成订单的实付金额；退款在退款流水里，不在这里冲减。
func (s *ProviderService) Income(ctx context.Context, providerID uint64) (*dto.ProviderIncomeResp, error) {
	total, count, err := s.income.Summary(ctx, providerID)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	list, err := s.income.List(ctx, providerID, 20)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	items := make([]dto.IncomeItem, 0, len(list))
	for _, it := range list {
		items = append(items, dto.IncomeItem{
			OrderNo:   it.OrderNo,
			Amount:    it.Amount,
			CreatedAt: it.CreatedAt,
		})
	}
	return &dto.ProviderIncomeResp{
		Total:      total,
		OrderCount: count,
		Items:      items,
	}, nil
}

// normalizePage 归一化分页参数。
func normalizePage(page, size *int) {
	if *page <= 0 {
		*page = 1
	}
	if *size <= 0 || *size > 50 {
		*size = 10
	}
}
