package service

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/alinger/community-handyman/server/internal/errs"
	"github.com/alinger/community-handyman/server/internal/model"
	"github.com/alinger/community-handyman/server/internal/repository"
)

// MonthlyDays 包月服务期，从订单的服务日期起算。
// 需求文档 Q-06：不累计、不顺延，过期未用自动作废，因此这里只做区间校验，不补次数。
const MonthlyDays = 30

// MonthlyService 包月核销。
// 不预占未来 30 天库存：核销当天才占用时段，否则会把可售时段长期锁死，
// 单次用户完全约不上，与需求文档第七章「包月先占位」的初衷相反。
type MonthlyService struct {
	db          *gorm.DB
	orders      *repository.OrderRepo
	slots       *repository.SlotRepo
	redemptions *repository.RedemptionRepo
}

func NewMonthlyService(db *gorm.DB, orders *repository.OrderRepo,
	slots *repository.SlotRepo, redemptions *repository.RedemptionRepo) *MonthlyService {
	return &MonthlyService{db: db, orders: orders, slots: slots, redemptions: redemptions}
}

// Redeem 核销某天的服务。当天已核销会返回明确提示，不会重复扣次数。
func (s *MonthlyService) Redeem(ctx context.Context, operatorID uint64, orderNo, dateStr, remark string) (*model.MonthlyRedemption, error) {
	o, err := s.orders.GetByNo(ctx, orderNo)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	if o == nil {
		return nil, errs.Biz(errs.CodeNotFound)
	}
	if o.Plan != model.PlanMonthly {
		return nil, errs.BizMsg(errs.CodeInvalidParam, "只有包月订单需要核销")
	}
	switch o.Status {
	case model.OrderPendingAccept, model.OrderAccepted, model.OrderInService:
	default:
		return nil, errs.BizMsg(errs.CodeIllegalStatus, "订单当前状态不可核销")
	}

	date, err := model.ParseDate(dateStr)
	if err != nil {
		return nil, errs.BizMsg(errs.CodeInvalidParam, "日期格式应为 2006-01-02")
	}

	start := o.ServiceDate.Time()
	end := start.AddDate(0, 0, MonthlyDays-1)
	day := date.Time()
	if day.Before(start) || day.After(end) {
		return nil, errs.BizMsg(errs.CodeInvalidParam,
			fmt.Sprintf("服务日期需在 %s 至 %s 之间", start.Format("2006-01-02"), end.Format("2006-01-02")))
	}
	if day.After(model.NewDate(time.Now()).Time()) {
		return nil, errs.BizMsg(errs.CodeInvalidParam, "不能核销未来日期")
	}

	m := &model.MonthlyRedemption{
		OrderID:     o.ID,
		ServiceDate: date,
		OperatorID:  operatorID,
		Remark:      remark,
	}

	// 首日在下单时已经占过位，核销时不能再占一次，否则同一单吃掉两个名额
	isFirstDay := date.String() == o.ServiceDate.String()

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if !isFirstDay {
			// 非首日：核销当天才占时段，满了就不核销，避免「核销了却没人上门」
			ok, rerr := s.slots.WithTx(tx).Reserve(ctx, date, o.SlotCode, o.ServiceType)
			if rerr != nil {
				return errs.Wrap(errs.CodeInternal, rerr)
			}
			if !ok {
				return errs.Biz(errs.CodeSlotFull)
			}
		}
		if cerr := s.redemptions.WithTx(tx).Create(ctx, m); cerr != nil {
			if cerr == repository.ErrDuplicate {
				return errs.BizMsg(errs.CodeIllegalStatus, "该日期已核销过")
			}
			return errs.Wrap(errs.CodeInternal, cerr)
		}
		return s.orders.WithTx(tx).AddEvent(ctx, &model.OrderEvent{
			OrderID:    o.ID,
			Node:       "redeemed",
			Operator:   "admin",
			OperatorID: &operatorID,
			Remark:     "包月核销 " + date.String(),
		})
	})
	if err != nil {
		return nil, err
	}
	return m, nil
}

// Summary 包月使用情况。
type MonthlySummary struct {
	Total       int                       `json:"total"`
	Used        int                       `json:"used"`
	Remaining   int                       `json:"remaining"`
	StartDate   string                    `json:"start_date"`
	EndDate     string                    `json:"end_date"`
	Redemptions []model.MonthlyRedemption `json:"redemptions"`
}

// Detail 返回包月使用情况与核销明细。
func (s *MonthlyService) Detail(ctx context.Context, orderNo string) (*MonthlySummary, error) {
	o, err := s.orders.GetByNo(ctx, orderNo)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	if o == nil {
		return nil, errs.Biz(errs.CodeNotFound)
	}
	if o.Plan != model.PlanMonthly {
		return nil, errs.BizMsg(errs.CodeInvalidParam, "该订单不是包月订单")
	}

	list, err := s.redemptions.ListByOrder(ctx, o.ID)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	start := o.ServiceDate.Time()
	return &MonthlySummary{
		Total:       MonthlyDays,
		Used:        len(list),
		Remaining:   MonthlyDays - len(list),
		StartDate:   start.Format("2006-01-02"),
		EndDate:     start.AddDate(0, 0, MonthlyDays-1).Format("2006-01-02"),
		Redemptions: list,
	}, nil
}
