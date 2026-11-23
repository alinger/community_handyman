package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/alinger/community-handyman/server/internal/dto"
	"github.com/alinger/community-handyman/server/internal/errs"
	"github.com/alinger/community-handyman/server/internal/model"
	"github.com/alinger/community-handyman/server/internal/repository"
	"github.com/alinger/community-handyman/server/pkg/util"
)

// RescheduleService 订单改期。
//
// 两端的能力刻意不对称：
//   - 住户可以「直接改」，改完立即生效并通知已接单的服务方；
//   - 服务方只能「发起申请」，必须等住户确认——时间是对住户的承诺，
//     服务方单方面改掉会让住户在约定时间空等。
//
// 改期要同时动三处：时段库存、订单主表、节点事件，所以整体放在一个事务里。
type RescheduleService struct {
	db          *gorm.DB
	orders      *repository.OrderRepo
	slots       *repository.SlotRepo
	reschedules *repository.RescheduleRepo
	notify      *NotifyService
}

func NewRescheduleService(db *gorm.DB, orders *repository.OrderRepo, slots *repository.SlotRepo,
	reschedules *repository.RescheduleRepo, notify *NotifyService) *RescheduleService {
	return &RescheduleService{db: db, orders: orders, slots: slots,
		reschedules: reschedules, notify: notify}
}

// userReschedulable 住户可直接改期的状态。
// 进入 on_the_way 之后服务方已经在路上，改时间没有意义，只能走取消。
var userReschedulable = map[model.OrderStatus]bool{
	model.OrderPendingPayment: true,
	model.OrderPendingAccept:  true,
	model.OrderAccepted:       true,
}

// RescheduleByUser 住户改期，立即生效。
func (s *RescheduleService) RescheduleByUser(ctx context.Context, userID uint64, orderNo string,
	req dto.RescheduleReq) (*dto.RescheduleInfo, error) {

	o, err := s.load(ctx, orderNo)
	if err != nil {
		return nil, err
	}
	if o.UserID != userID {
		return nil, errs.Biz(errs.CodeNotFound)
	}
	if !userReschedulable[o.Status] {
		return nil, errs.BizMsg(errs.CodeIllegalStatus,
			"订单当前为「"+o.Status.Label()+"」，不能修改服务时间")
	}

	toDate, err := s.validateTarget(ctx, o, req.ServiceDate, req.SlotCode)
	if err != nil {
		return nil, err
	}

	rec := &model.OrderReschedule{
		OrderID:     o.ID,
		OrderNo:     o.OrderNo,
		FromDate:    o.ServiceDate,
		FromSlot:    o.SlotCode,
		ToDate:      toDate,
		ToSlot:      req.SlotCode,
		Reason:      strings.TrimSpace(req.Reason),
		InitiatedBy: model.RescheduleByUser,
		InitiatedID: userID,
		Status:      model.RescheduleApplied,
	}
	now := time.Now()
	rec.HandledAt = &now

	if err := s.apply(ctx, o, rec, "user", userID, "住户修改服务时间"); err != nil {
		return nil, err
	}

	// 服务方已接单才需要告知，未派单时没人受影响
	if o.ProviderID != nil {
		s.notify.OrderEvent(ctx, *o.ProviderID, o.OrderNo, "服务时间已修改",
			"新时间 "+rec.ToDate.String()+" "+rec.ToSlot)
	}
	return toRescheduleInfo(rec), nil
}

// RequestByProvider 服务方发起改期申请，等住户确认后生效。
func (s *RescheduleService) RequestByProvider(ctx context.Context, providerID uint64, orderNo string,
	req dto.RescheduleReq) (*dto.RescheduleInfo, error) {

	o, err := s.load(ctx, orderNo)
	if err != nil {
		return nil, err
	}
	if o.ProviderID == nil || *o.ProviderID != providerID {
		return nil, errs.BizMsg(errs.CodeForbidden, "只能为自己接的订单申请改期")
	}
	if o.Status != model.OrderAccepted {
		return nil, errs.BizMsg(errs.CodeIllegalStatus,
			"订单当前为「"+o.Status.Label()+"」，不能申请改期")
	}

	toDate, err := s.validateTarget(ctx, o, req.ServiceDate, req.SlotCode)
	if err != nil {
		return nil, err
	}

	// 同一订单只允许一条待确认申请，否则住户会看到两个"改到哪"的选项
	pending, err := s.reschedules.PendingByOrder(ctx, o.ID)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	if pending != nil {
		return nil, errs.BizMsg(errs.CodeIllegalStatus, "已有待确认的改期申请，请等待住户处理")
	}

	rec := &model.OrderReschedule{
		OrderID:     o.ID,
		OrderNo:     o.OrderNo,
		FromDate:    o.ServiceDate,
		FromSlot:    o.SlotCode,
		ToDate:      toDate,
		ToSlot:      req.SlotCode,
		Reason:      strings.TrimSpace(req.Reason),
		InitiatedBy: model.RescheduleByProvider,
		InitiatedID: providerID,
		Status:      model.ReschedulePending,
	}
	if err := s.reschedules.Create(ctx, rec); err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}

	s.notify.OrderEvent(ctx, o.UserID, o.OrderNo, "服务方申请改期",
		"新时间 "+rec.ToDate.String()+" "+rec.ToSlot)
	return toRescheduleInfo(rec), nil
}

// Respond 住户处理服务方的改期申请。同意则立即换库存与订单时间。
func (s *RescheduleService) Respond(ctx context.Context, userID uint64, orderNo string,
	approve bool, reason string) (*dto.RescheduleInfo, error) {

	o, err := s.load(ctx, orderNo)
	if err != nil {
		return nil, err
	}
	if o.UserID != userID {
		return nil, errs.Biz(errs.CodeNotFound)
	}

	rec, err := s.reschedules.PendingByOrder(ctx, o.ID)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	if rec == nil {
		return nil, errs.BizMsg(errs.CodeNotFound, "没有待确认的改期申请")
	}
	// 申请提交后订单可能已被推进（服务方出发了），此时不能再改
	if !userReschedulable[o.Status] {
		return nil, errs.BizMsg(errs.CodeIllegalStatus,
			"订单当前为「"+o.Status.Label()+"」，不能改期")
	}

	now := time.Now()
	if !approve {
		extra := map[string]any{"handled_at": now}
		if r := strings.TrimSpace(reason); r != "" {
			extra["reason"] = r
		}
		ok, uerr := s.reschedules.UpdateStatus(ctx, rec.ID, model.ReschedulePending,
			model.RescheduleRejected, extra)
		if uerr != nil {
			return nil, errs.Wrap(errs.CodeInternal, uerr)
		}
		if !ok {
			return nil, errs.BizMsg(errs.CodeIllegalStatus, "该申请已被处理")
		}
		rec.Status = model.RescheduleRejected
		rec.HandledAt = &now
		if r := strings.TrimSpace(reason); r != "" {
			rec.Reason = r
		}
		s.notify.OrderEvent(ctx, rec.InitiatedID, o.OrderNo, "改期申请被拒绝", "仍按原时间服务")
		return toRescheduleInfo(rec), nil
	}

	// 同意：先校验目标时段仍然可约（申请提交到确认之间可能被别人约满）
	if _, verr := s.validateTarget(ctx, o, rec.ToDate.String(), rec.ToSlot); verr != nil {
		return nil, verr
	}
	rec.HandledAt = &now
	rec.Status = model.RescheduleApplied
	if err := s.apply(ctx, o, rec, "user", userID, "住户同意服务方改期"); err != nil {
		return nil, err
	}
	s.notify.OrderEvent(ctx, rec.InitiatedID, o.OrderNo, "改期已确认",
		"新时间 "+rec.ToDate.String()+" "+rec.ToSlot)
	return toRescheduleInfo(rec), nil
}

// OfOrder 返回订单的改期记录，供详情页展示待确认申请与改期历史。
func (s *RescheduleService) OfOrder(ctx context.Context, orderID uint64, limit int) []dto.RescheduleInfo {
	list, err := s.reschedules.ListByOrder(ctx, orderID, limit)
	if err != nil || len(list) == 0 {
		return nil
	}
	out := make([]dto.RescheduleInfo, 0, len(list))
	for i := range list {
		out = append(out, *toRescheduleInfo(&list[i]))
	}
	return out
}

// AttachTo 把待确认申请与改期历史补到订单详情上。
// 订单主表只存最新时间，界面要还原「从几点改到几点」只能靠这张表。
func (s *RescheduleService) AttachTo(ctx context.Context, orderNo string, resp *dto.OrderResp) {
	if resp == nil {
		return
	}
	o, err := s.orders.GetByNo(ctx, orderNo)
	if err != nil || o == nil {
		return
	}
	list := s.OfOrder(ctx, o.ID, 10)
	if len(list) == 0 {
		return
	}
	resp.Reschedules = list
	for i := range list {
		if list[i].Status == model.ReschedulePending {
			pending := list[i]
			resp.PendingReschedule = &pending
			break
		}
	}
}

// AttachPendingTo 给列表里的订单批量填充「待确认的改期申请」。
// 列表页要让住户一眼看到有单子等自己点头，否则只能进详情才发现。
// 只填 pending 那一条、不填历史——列表不需要，填了只会把响应撑大。
func (s *RescheduleService) AttachPendingTo(ctx context.Context, list []dto.OrderResp) {
	if len(list) == 0 {
		return
	}
	nos := make([]string, 0, len(list))
	for i := range list {
		nos = append(nos, list[i].OrderNo)
	}
	m, err := s.reschedules.PendingByOrderNos(ctx, nos)
	if err != nil || len(m) == 0 {
		return
	}
	for i := range list {
		rec, ok := m[list[i].OrderNo]
		if !ok {
			continue
		}
		info := *toRescheduleInfo(&rec)
		list[i].PendingReschedule = &info
	}
}

// AttachProviderNoticeTo 给服务方列表批量补「住户改了时间、你还没确认」的提示。
//
// 住户改期是即时生效的：服务方界面上那张卡的日期会悄悄变掉，他不逐单翻详情
// 根本发现不了，结果就是按旧时间去上门。所以在他点「我知道了」之前一直挂着。
func (s *RescheduleService) AttachProviderNoticeTo(ctx context.Context, list []dto.OrderResp) {
	if len(list) == 0 {
		return
	}
	nos := make([]string, 0, len(list))
	for i := range list {
		nos = append(nos, list[i].OrderNo)
	}
	m, err := s.reschedules.UnackedByUser(ctx, nos)
	if err != nil || len(m) == 0 {
		return
	}
	for i := range list {
		rec, ok := m[list[i].OrderNo]
		if !ok {
			continue
		}
		info := *toRescheduleInfo(&rec)
		list[i].RescheduleNotice = &info
	}
}

// AttachProviderNotice 给单个订单补「住户改了时间、你还没确认」的提示。
//
// 只能给接单的服务方本人：住户看自己的单不需要这个字段，
// 下发出去反而等于告诉他「服务方还没看到」。所以这里自己查订单校验归属，
// 不依赖 OrderResp.ProviderID——那是后台扩展字段，用户端详情并不下发。
func (s *RescheduleService) AttachProviderNotice(ctx context.Context, viewerID uint64, resp *dto.OrderResp) {
	if resp == nil || resp.OrderNo == "" {
		return
	}
	o, err := s.orders.GetByNo(ctx, resp.OrderNo)
	if err != nil || o == nil || o.ProviderID == nil || *o.ProviderID != viewerID {
		return
	}
	m, err := s.reschedules.UnackedByUser(ctx, []string{resp.OrderNo})
	if err != nil {
		return
	}
	rec, ok := m[resp.OrderNo]
	if !ok {
		return
	}
	info := *toRescheduleInfo(&rec)
	resp.RescheduleNotice = &info
}

// AckProviderNotice 服务方确认知悉住户的改期。重复确认不报错。
func (s *RescheduleService) AckProviderNotice(ctx context.Context, providerID uint64, orderNo string) error {
	o, err := s.load(ctx, orderNo)
	if err != nil {
		return err
	}
	if o.ProviderID == nil || *o.ProviderID != providerID {
		return errs.BizMsg(errs.CodeForbidden, "只能确认自己接的订单")
	}
	if _, err := s.reschedules.AckProvider(ctx, o.ID); err != nil {
		return errs.Wrap(errs.CodeInternal, err)
	}
	return nil
}

// AckProviderNoticeQuiet 服务方推进订单时视为已看过时间变更。
//
// 能点「出发上门」「开始服务」，说明他必然核对过订单上的新时间；
// 此时再挂一条红条只会变成噪音。失败只记日志，不影响推进本身。
func (s *RescheduleService) AckProviderNoticeQuiet(ctx context.Context, orderNo string) {
	o, err := s.orders.GetByNo(ctx, orderNo)
	if err != nil || o == nil {
		return
	}
	if _, err := s.reschedules.AckProvider(ctx, o.ID); err != nil {
		slog.Warn("标记改期已读失败", "order_no", orderNo, "err", err)
	}
}

// load 取订单，统一处理"不存在"。
func (s *RescheduleService) load(ctx context.Context, orderNo string) (*model.Order, error) {
	o, err := s.orders.GetByNo(ctx, orderNo)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	if o == nil {
		return nil, errs.Biz(errs.CodeNotFound)
	}
	return o, nil
}

// validateTarget 校验目标时间。返回解析后的日期，时段码由调用方直接透传订单。
//
// 2 小时窗口与取消规则对齐：服务开始前 2 小时内既不能取消也不能改期，
// 否则服务方可能已经出门了。
func (s *RescheduleService) validateTarget(ctx context.Context, o *model.Order,
	dateStr, slotCode string) (model.Date, error) {

	// 包月的 service_date 是服务期开始日，改它等于改合同期，
	// 库存也不占用，语义与单次订单完全不同，交给运营人工处理
	if o.Plan == model.PlanMonthly {
		return model.Date{}, errs.BizMsg(errs.CodeIllegalStatus, "包月订单请联系客服调整服务期")
	}

	toDate, err := model.ParseDate(dateStr)
	if err != nil {
		return model.Date{}, errs.BizMsg(errs.CodeInvalidParam, "日期格式应为 2006-01-02")
	}
	today := model.NewDate(time.Now())
	if toDate.Time().Before(today.Time()) {
		return model.Date{}, errs.BizMsg(errs.CodeInvalidParam, "不能改到过去的日期")
	}
	if toDate.Time().Equal(o.ServiceDate.Time()) && slotCode == o.SlotCode {
		return model.Date{}, errs.BizMsg(errs.CodeInvalidParam, "新时间与当前时间相同")
	}

	start, err := util.SlotStart(toDate.Time(), slotCode)
	if err != nil {
		return model.Date{}, errs.BizMsg(errs.CodeInvalidParam, "时段不合法")
	}
	if !start.After(time.Now().Add(2 * time.Hour)) {
		return model.Date{}, errs.BizMsg(errs.CodeInvalidParam, "只能改到 2 小时之后的时段")
	}

	slot, err := s.slots.Get(ctx, toDate, slotCode, o.ServiceType)
	if err != nil {
		return model.Date{}, errs.Wrap(errs.CodeInternal, err)
	}
	if slot == nil || !slot.Available() {
		return model.Date{}, errs.Biz(errs.CodeSlotFull)
	}
	return toDate, nil
}

// apply 在一个事务里换库存、改订单、写事件与改期记录。
func (s *RescheduleService) apply(ctx context.Context, o *model.Order, rec *model.OrderReschedule,
	operator string, operatorID uint64, remark string) error {

	full := fmt.Sprintf("%s：%s %s → %s %s", remark,
		rec.FromDate.String(), rec.FromSlot, rec.ToDate.String(), rec.ToSlot)

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 先占新时段：名额被别人抢走时整体回滚，旧时段不动
		reserved, err := s.slots.WithTx(tx).Reserve(ctx, rec.ToDate, rec.ToSlot, o.ServiceType)
		if err != nil {
			return errs.Wrap(errs.CodeInternal, err)
		}
		if !reserved {
			return errs.Biz(errs.CodeSlotFull)
		}
		if err := s.slots.WithTx(tx).Release(ctx, o.ServiceDate, o.SlotCode, o.ServiceType); err != nil {
			return errs.Wrap(errs.CodeInternal, err)
		}

		res := tx.Model(&model.Order{}).Where("id = ? AND status = ?", o.ID, o.Status).
			Updates(map[string]any{
				"service_date": rec.ToDate.Time(),
				"slot_code":    rec.ToSlot,
				"updated_at":   time.Now(),
			})
		if res.Error != nil {
			return errs.Wrap(errs.CodeInternal, res.Error)
		}
		if res.RowsAffected == 0 {
			return errs.Biz(errs.CodeIllegalStatus)
		}

		if err := s.orders.WithTx(tx).AddEvent(ctx, &model.OrderEvent{
			OrderID:    o.ID,
			Node:       "reschedule",
			Operator:   operator,
			OperatorID: &operatorID,
			Remark:     full,
		}); err != nil {
			return errs.Wrap(errs.CodeInternal, err)
		}

		// 住户直接改期是新建记录；住户同意服务方申请是更新同一条记录的状态
		if rec.ID == 0 {
			// 住户已经自己给了新时间，服务方那条待确认申请就该一并作废，
			// 否则服务方一直等确认、住户还能回头点「同意」把时间改回服务方要的时段
			if _, err := s.reschedules.WithTx(tx).CancelPendingByOrder(ctx, o.ID); err != nil {
				return errs.Wrap(errs.CodeInternal, err)
			}
			if err := s.reschedules.WithTx(tx).Create(ctx, rec); err != nil {
				return errs.Wrap(errs.CodeInternal, err)
			}
			return nil
		}
		ok, uerr := s.reschedules.WithTx(tx).UpdateStatus(ctx, rec.ID,
			model.ReschedulePending, model.RescheduleApplied,
			map[string]any{"handled_at": rec.HandledAt})
		if uerr != nil {
			return errs.Wrap(errs.CodeInternal, uerr)
		}
		if !ok {
			return errs.BizMsg(errs.CodeIllegalStatus, "该申请已被处理")
		}
		return nil
	})
}

// toRescheduleInfo 转 DTO。日期与时段分开下发，前端按自己的格式拼文案。
func toRescheduleInfo(r *model.OrderReschedule) *dto.RescheduleInfo {
	return &dto.RescheduleInfo{
		ID:          r.ID,
		FromDate:    r.FromDate.String(),
		FromSlot:    r.FromSlot,
		ToDate:      r.ToDate.String(),
		ToSlot:      r.ToSlot,
		Reason:      r.Reason,
		Status:      r.Status,
		InitiatedBy: r.InitiatedBy,
		CreatedAt:   r.CreatedAt,
		HandledAt:   r.HandledAt,
		ProviderAck: r.ProviderAckAt,
	}
}
