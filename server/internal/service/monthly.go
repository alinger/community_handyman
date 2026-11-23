package service

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/alinger/community-handyman/server/internal/dto"
	"github.com/alinger/community-handyman/server/internal/errs"
	"github.com/alinger/community-handyman/server/internal/model"
	"github.com/alinger/community-handyman/server/internal/repository"
)

// MonthlyDays 包月服务期，从订单的服务日期起算。
// 需求文档 Q-06：不累计、不顺延，过期未用自动作废，因此这里只做区间校验，不补次数。
const MonthlyDays = 30

// MonthlyService 包月核销。
//
// 包月不碰时段库存：库里的时段是给单次订单按天卖的（08-10 / 12-14 / 18-20），
// 包月单自带固定上门时段（22:00-22:30），下单时也不占库存。早期版本在打卡时
// 调了 slots.Reserve，用包月的 slot_code 去扣单次的时段池，两边编码压根对不上，
// 非首日打卡必然一行都匹配不到，于是每次都报「该时段已约满，请选择其他时段」——
// 而服务方根本没法改时段，这句话对他没有任何可操作信息。
type MonthlyService struct {
	db          *gorm.DB
	orders      *repository.OrderRepo
	redemptions *repository.RedemptionRepo
}

func NewMonthlyService(db *gorm.DB, orders *repository.OrderRepo,
	redemptions *repository.RedemptionRepo) *MonthlyService {
	return &MonthlyService{db: db, orders: orders, redemptions: redemptions}
}

// monthlyRange 月卡服务期（含首尾两天）。
// 以订单上的 monthly_start/end 为准；历史数据缺列时按服务日期 + 29 天兜底，
// 避免因为这些订单没有服务期就永远不能打卡。
func monthlyRange(o *model.Order) (start, end model.Date) {
	start = o.ServiceDate
	if o.MonthlyStart != nil {
		start = *o.MonthlyStart
	}
	end = model.NewDate(start.Time().AddDate(0, 0, MonthlyDays-1))
	if o.MonthlyEnd != nil {
		end = *o.MonthlyEnd
	}
	return start, end
}

// Redeem 运营在后台核销某天的服务。当天已核销会返回明确提示，不会重复扣次数。
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

	start, end := monthlyRange(o)
	day := date.Time()
	if day.Before(start.Time()) || day.After(end.Time()) {
		return nil, errs.BizMsg(errs.CodeInvalidParam,
			fmt.Sprintf("服务日期需在 %s 至 %s 之间", start.String(), end.String()))
	}
	if day.After(model.NewDate(time.Now()).Time()) {
		return nil, errs.BizMsg(errs.CodeInvalidParam, "不能核销未来日期")
	}

	return s.redeem(ctx, o, date, operatorID, "admin", remark)
}

// ProviderRedeem 服务方在任务列表里打卡「今日已服务」。
// 运营和履约人共用同一张核销表，靠 operator 区分来源：
// 不这样的话服务方每天都得等后台代录，列表上的「今天打过卡没有」就成了摆设。
func (s *MonthlyService) ProviderRedeem(ctx context.Context, providerID uint64, orderNo string) (*dto.MonthlyProgress, error) {
	o, err := s.orders.GetByNo(ctx, orderNo)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	if o == nil {
		return nil, errs.Biz(errs.CodeNotFound)
	}
	if o.Plan != model.PlanMonthly {
		return nil, errs.BizMsg(errs.CodeInvalidParam, "只有包月订单需要打卡")
	}
	if o.ProviderID == nil || *o.ProviderID != providerID {
		return nil, errs.BizMsg(errs.CodeForbidden, "只能给自己接的订单打卡")
	}
	switch o.Status {
	case model.OrderPendingAccept, model.OrderAccepted, model.OrderOnTheWay, model.OrderInService:
	default:
		return nil, errs.BizMsg(errs.CodeIllegalStatus, "订单当前状态不可打卡")
	}

	today := model.NewDate(time.Now())
	start, end := monthlyRange(o)
	if today.Time().Before(start.Time()) {
		return nil, errs.BizMsg(errs.CodeInvalidParam, "服务期从 "+start.String()+" 开始，今天还不用上门")
	}
	if today.Time().After(end.Time()) {
		return nil, errs.BizMsg(errs.CodeInvalidParam, "服务期已在 "+end.String()+" 结束，无法打卡")
	}

	if _, err := s.redeem(ctx, o, today, providerID, "provider", "服务方打卡"); err != nil {
		return nil, err
	}

	// 打卡后回填最新进度，前端拿到就能直接改卡片状态，不用整页重拉
	p, err := s.progressOne(ctx, o)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// ProviderFinish 服务方收尾今天这一趟（「完成服务」）。
//
// 与打卡分开的原因：打卡只说明人到了，袋子提走没有是另一回事。合成一步时
// 服务方点一下「出发上门」当天就算服务完成，住户端也立刻显示「今日已服务」，
// 中间没有任何确认动作。
//
// 不改变订单状态：月卡 status 仍停在「已受理」，靠当天核销记录的 finished_at
// 把这张单从「服务中」挪到「已完成」，第二天自动回到「待服务」。
func (s *MonthlyService) ProviderFinish(ctx context.Context, providerID uint64, orderNo string) (*dto.MonthlyProgress, error) {
	o, err := s.orders.GetByNo(ctx, orderNo)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	if o == nil {
		return nil, errs.Biz(errs.CodeNotFound)
	}
	if o.Plan != model.PlanMonthly {
		return nil, errs.BizMsg(errs.CodeInvalidParam, "只有包月订单需要每日完成")
	}
	if o.ProviderID == nil || *o.ProviderID != providerID {
		return nil, errs.BizMsg(errs.CodeForbidden, "只能完成自己接的订单")
	}
	switch o.Status {
	case model.OrderPendingAccept, model.OrderAccepted, model.OrderOnTheWay, model.OrderInService:
	default:
		return nil, errs.BizMsg(errs.CodeIllegalStatus, "订单当前状态不可完成")
	}

	today := model.NewDate(time.Now())
	start, end := monthlyRange(o)
	if today.Time().Before(start.Time()) {
		return nil, errs.BizMsg(errs.CodeInvalidParam, "服务期从 "+start.String()+" 开始，今天还不用上门")
	}
	if today.Time().After(end.Time()) {
		return nil, errs.BizMsg(errs.CodeInvalidParam, "服务期已在 "+end.String()+" 结束")
	}

	// 没打卡就不能完成：否则等于跳过「人到了」直接补一个完成标记
	rec, err := s.redemptions.GetByDate(ctx, o.ID, today)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	if rec == nil {
		return nil, errs.BizMsg(errs.CodeIllegalStatus, "今天还没点「出发上门」")
	}

	now := time.Now()
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ok, uerr := s.redemptions.WithTx(tx).FinishToday(ctx, o.ID, today, now)
		if uerr != nil {
			return errs.Wrap(errs.CodeInternal, uerr)
		}
		if !ok {
			return errs.BizMsg(errs.CodeIllegalStatus, "今天这趟已经完成了")
		}
		return s.orders.WithTx(tx).AddEvent(ctx, &model.OrderEvent{
			OrderID:    o.ID,
			Node:       "monthly_finished",
			Operator:   "provider",
			OperatorID: &providerID,
			Remark:     "包月本趟完成 " + today.String(),
		})
	})
	if err != nil {
		return nil, err
	}

	p, err := s.progressOne(ctx, o)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// redeem 写入一天的核销记录。
//
// 只写核销表与事件，不扣时段库存：包月的时段由订单自带（22:00-22:30），
// 与单次订单按天卖的时段池（08-10 / 12-14 / 18-20）是两套编码。
// 之前在这里 Reserve 过，非首日打卡必然匹配不到库存行而被判成「已约满」。
func (s *MonthlyService) redeem(ctx context.Context, o *model.Order, date model.Date,
	operatorID uint64, operator, remark string) (*model.MonthlyRedemption, error) {
	m := &model.MonthlyRedemption{
		OrderID:     o.ID,
		ServiceDate: date,
		OperatorID:  operatorID,
		Remark:      remark,
	}
	// 运营在后台代录的核销，记的是「这天的服务已经做完了」，不是「人刚到」。
	// 不顺手盖上完成时间的话，后台核销过的日期会一直显示成「已上门未完成」，
	// 订单也会在服务方那边的「服务中」里挂着。
	if operator == "admin" {
		at := time.Now()
		m.FinishedAt = &at
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if cerr := s.redemptions.WithTx(tx).Create(ctx, m); cerr != nil {
			if cerr == repository.ErrDuplicate {
				return errs.BizMsg(errs.CodeIllegalStatus, "今天已经打过卡了")
			}
			return errs.Wrap(errs.CodeInternal, cerr)
		}
		return s.orders.WithTx(tx).AddEvent(ctx, &model.OrderEvent{
			OrderID:    o.ID,
			Node:       "redeemed",
			Operator:   operator,
			OperatorID: &operatorID,
			Remark:     "包月核销 " + date.String(),
		})
	})
	if err != nil {
		return nil, err
	}
	return m, nil
}

// MonthlySummary 包月使用情况。
type MonthlySummary struct {
	Total       int                       `json:"total"`
	Used        int                       `json:"used"`
	Remaining   int                       `json:"remaining"`
	StartDate   string                    `json:"start_date"`
	EndDate     string                    `json:"end_date"`
	Redemptions []model.MonthlyRedemption `json:"redemptions"`
	// Today / TodayDone 供服务方详情页直接判断今天打过卡没有
	Today     string     `json:"today"`
	TodayDone bool       `json:"today_done"`
	TodayAt   *time.Time `json:"today_at,omitempty"`
	// TodayFinished 今天这趟是否已点过「完成服务」；TodayFinishedAt 为完成时间
	TodayFinished   bool       `json:"today_finished"`
	TodayFinishedAt *time.Time `json:"today_finished_at,omitempty"`
	InPeriod        bool       `json:"in_period"`
	Ended           bool       `json:"ended"`
}

// Detail 返回包月使用情况与核销明细。
// 只允许订单住户本人与服务方本人查看：核销明细包含每天的上门时间，
// 不校验归属等于把任意订单的履约轨迹开放给所有登录用户。
func (s *MonthlyService) Detail(ctx context.Context, viewerID uint64, orderNo string) (*MonthlySummary, error) {
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
	isOwner := o.UserID == viewerID
	isProvider := o.ProviderID != nil && *o.ProviderID == viewerID
	if viewerID != 0 && !isOwner && !isProvider {
		return nil, errs.Biz(errs.CodeForbidden)
	}

	list, err := s.redemptions.ListByOrder(ctx, o.ID)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	start, end := monthlyRange(o)
	p := s.buildProgress(o, start, end, model.NewDate(time.Now()),
		repository.MonthlyStat{Used: len(list)})
	// 明细里已经有全部记录，今日打卡时间不必再查一次
	for i := range list {
		if list[i].ServiceDate.String() == p.Today {
			at := list[i].CreatedAt
			p.TodayAt = &at
			p.TodayDone = true
			if list[i].FinishedAt != nil {
				p.TodayFinishedAt = list[i].FinishedAt
				p.TodayFinished = true
			}
		}
	}
	return &MonthlySummary{
		Total:           p.Total,
		Used:            p.Used,
		Remaining:       p.Total - p.Used,
		StartDate:       p.StartDate,
		EndDate:         p.EndDate,
		Redemptions:     list,
		Today:           p.Today,
		TodayDone:       p.TodayDone,
		TodayAt:         p.TodayAt,
		TodayFinished:   p.TodayFinished,
		TodayFinishedAt: p.TodayFinishedAt,
		InPeriod:        p.InPeriod,
		Ended:           p.Ended,
	}, nil
}

// buildProgress 由服务期与核销汇总拼出今日进度。
func (s *MonthlyService) buildProgress(o *model.Order, start, end, today model.Date,
	stat repository.MonthlyStat) dto.MonthlyProgress {
	inPeriod := !today.Time().Before(start.Time()) && !today.Time().After(end.Time())
	return dto.MonthlyProgress{
		Today:           today.String(),
		StartDate:       start.String(),
		EndDate:         end.String(),
		Used:            stat.Used,
		Total:           MonthlyDays,
		TodayDone:       stat.TodayAt != nil,
		TodayAt:         stat.TodayAt,
		TodayFinished:   stat.TodayFinishedAt != nil,
		TodayFinishedAt: stat.TodayFinishedAt,
		InPeriod:        inPeriod,
		Ended:           today.Time().After(end.Time()),
	}
}

// progressOne 单张订单的进度，用于打卡后回填。
func (s *MonthlyService) progressOne(ctx context.Context, o *model.Order) (dto.MonthlyProgress, error) {
	today := model.NewDate(time.Now())
	stats, err := s.redemptions.StatsByOrderIDs(ctx, []uint64{o.ID}, today)
	if err != nil {
		return dto.MonthlyProgress{}, errs.Wrap(errs.CodeInternal, err)
	}
	start, end := monthlyRange(o)
	return s.buildProgress(o, start, end, today, stats[o.ID]), nil
}

// AttachProgressTo 给列表补月卡今日进度。
// orders 与 list 必须同下标对应（handler 里是同一次循环产出的），
// 靠下标而不是 order_no 关联，省一次字段解析。
func (s *MonthlyService) AttachProgressTo(ctx context.Context, orders []model.Order, list []dto.OrderResp) {
	ids := make([]uint64, 0, len(orders))
	for i := range orders {
		if orders[i].Plan == model.PlanMonthly {
			ids = append(ids, orders[i].ID)
		}
	}
	if len(ids) == 0 {
		return
	}
	today := model.NewDate(time.Now())
	stats, err := s.redemptions.StatsByOrderIDs(ctx, ids, today)
	if err != nil {
		// 月卡进度只是卡片上的辅助信息，查失败不影响订单列表本身
		return
	}
	for i := range orders {
		if i >= len(list) || orders[i].Plan != model.PlanMonthly {
			continue
		}
		start, end := monthlyRange(&orders[i])
		p := s.buildProgress(&orders[i], start, end, today, stats[orders[i].ID])
		list[i].Monthly = &p
	}
}
