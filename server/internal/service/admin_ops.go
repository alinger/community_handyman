package service

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/alinger/community-handyman/server/internal/dto"
	"github.com/alinger/community-handyman/server/internal/errs"
	"github.com/alinger/community-handyman/server/internal/model"
)

// 本文件是运营后台的扩展能力：财务流水、用户管理、看板统计、
// 月卡核销记录、范围规则修改、时段模板复制。
// 查询直接走 s.db（与 ListProviders 同一套路），复杂事务仍归各业务服务。

// ---------- 支付流水 ----------

// ListPayments 支付流水分页查询，可按订单号与状态过滤。
func (s *AdminService) ListPayments(ctx context.Context, orderNo, status string, page, size int) ([]dto.AdminPayment, int64, error) {
	normalizePage(&page, &size)
	q := s.db.WithContext(ctx).Model(&model.PaymentTransaction{})
	if orderNo != "" {
		q = q.Where("order_no LIKE ?", "%"+orderNo+"%")
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, errs.Wrap(errs.CodeInternal, err)
	}
	var list []model.PaymentTransaction
	if err := q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&list).Error; err != nil {
		return nil, 0, errs.Wrap(errs.CodeInternal, err)
	}
	out := make([]dto.AdminPayment, 0, len(list))
	for _, t := range list {
		out = append(out, dto.AdminPayment{
			ID: t.ID, OrderNo: t.OrderNo, TransactionID: t.TransactionID,
			OutTradeNo: t.OutTradeNo, Amount: t.Amount, Status: t.Status,
			PaidAt: t.PaidAt, CreatedAt: t.CreatedAt,
		})
	}
	return out, total, nil
}

// ---------- 退款单 ----------

// ListRefunds 退款单分页查询。 refunds 表只存 order_id，批量补 order_no。
func (s *AdminService) ListRefunds(ctx context.Context, orderNo, status string, page, size int) ([]dto.AdminRefund, int64, error) {
	normalizePage(&page, &size)
	q := s.db.WithContext(ctx).Model(&model.Refund{})
	if orderNo != "" || status != "" {
		q = q.Joins("JOIN orders ON orders.id = refunds.order_id")
	}
	if orderNo != "" {
		q = q.Where("orders.order_no LIKE ?", "%"+orderNo+"%")
	}
	if status != "" {
		q = q.Where("refunds.status = ?", status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, errs.Wrap(errs.CodeInternal, err)
	}
	var list []model.Refund
	if err := q.Select("refunds.*").Order("refunds.id DESC").
		Offset((page - 1) * size).Limit(size).Find(&list).Error; err != nil {
		return nil, 0, errs.Wrap(errs.CodeInternal, err)
	}

	// 批量补 order_no，避免 N+1
	ids := make([]uint64, 0, len(list))
	for _, r := range list {
		ids = append(ids, r.OrderID)
	}
	orderNoMap := map[uint64]string{}
	if len(ids) > 0 {
		var os []model.Order
		if err := s.db.WithContext(ctx).Select("id, order_no").
			Where("id IN ?", ids).Find(&os).Error; err != nil {
			return nil, 0, errs.Wrap(errs.CodeInternal, err)
		}
		for _, o := range os {
			orderNoMap[o.ID] = o.OrderNo
		}
	}
	out := make([]dto.AdminRefund, 0, len(list))
	for _, r := range list {
		out = append(out, dto.AdminRefund{
			ID: r.ID, OrderNo: orderNoMap[r.OrderID], OutRefundNo: r.OutRefundNo,
			RefundID: r.RefundID, Amount: r.Amount, Reason: r.Reason, Status: r.Status,
			OperatorID: r.OperatorID, RefundedAt: r.RefundedAt, CreatedAt: r.CreatedAt,
		})
	}
	return out, total, nil
}

// ---------- 订单时间线 ----------

// OrderEvents 订单节点回放，后台纠纷处理用。
func (s *AdminService) OrderEvents(ctx context.Context, orderNo string) ([]dto.AdminEvent, error) {
	o, err := s.orders.GetByNo(ctx, orderNo)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	if o == nil {
		return nil, errs.Biz(errs.CodeNotFound)
	}
	var list []model.OrderEvent
	if err := s.db.WithContext(ctx).Where("order_id = ?", o.ID).Order("id").Find(&list).Error; err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	out := make([]dto.AdminEvent, 0, len(list))
	for _, e := range list {
		out = append(out, dto.AdminEvent{
			ID: e.ID, Node: e.Node, Operator: e.Operator,
			OperatorID: e.OperatorID, Remark: e.Remark, CreatedAt: e.CreatedAt,
		})
	}
	return out, nil
}

// ---------- 看板统计 ----------

// Stats 看板首页数据。今日口径按服务器本地时区的自然日。
func (s *AdminService) Stats(ctx context.Context) (*dto.AdminStats, error) {
	dayStart := time.Now().Truncate(24 * time.Hour)
	st := &dto.AdminStats{StatusDist: []dto.AdminStatusCount{}, ServiceDist: []dto.AdminServiceCount{}}

	type cnt struct {
		C int64
	}
	one := func(q *gorm.DB, dst *int64) error {
		var r cnt
		if err := q.Scan(&r).Error; err != nil {
			return err
		}
		*dst = r.C
		return nil
	}

	if err := one(s.db.WithContext(ctx).Model(&model.Order{}).
		Where("created_at >= ?", dayStart).Select("COUNT(*) AS c"), &st.TodayOrders); err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	if err := one(s.db.WithContext(ctx).Model(&model.Order{}).
		Where("paid_at >= ?", dayStart).Select("COALESCE(SUM(paid_amount),0) AS c"), &st.TodayGMV); err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	if err := one(s.db.WithContext(ctx).Model(&model.Refund{}).
		Where("created_at >= ? AND status = ?", dayStart, model.RefundSuccess).
		Select("COALESCE(SUM(amount),0) AS c"), &st.TodayRefunded); err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	if err := one(s.db.WithContext(ctx).Model(&model.Order{}).
		Where("status = ?", model.OrderPendingAccept).Select("COUNT(*) AS c"), &st.PendingAccept); err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	if err := one(s.db.WithContext(ctx).Model(&model.Order{}).
		Where("status = ? AND provider_id IS NULL", model.OrderPendingAccept).
		Select("COUNT(*) AS c"), &st.Unassigned); err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	if err := one(s.db.WithContext(ctx).Model(&model.Order{}).
		Where("status IN ?", []model.OrderStatus{model.OrderAccepted, model.OrderOnTheWay,
			model.OrderInService, model.OrderPendingConfirm}).Select("COUNT(*) AS c"), &st.InService); err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	if err := one(s.db.WithContext(ctx).Model(&model.User{}).
		Where("role = ?", "user").Select("COUNT(*) AS c"), &st.TotalUsers); err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	if err := one(s.db.WithContext(ctx).Model(&model.User{}).
		Where("role = ?", "provider").Select("COUNT(*) AS c"), &st.Providers); err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}

	var sd []struct {
		Status string
		C      int64
	}
	if err := s.db.WithContext(ctx).Model(&model.Order{}).
		Select("status, COUNT(*) AS c").Group("status").Scan(&sd).Error; err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	for _, r := range sd {
		st.StatusDist = append(st.StatusDist, dto.AdminStatusCount{Status: r.Status, Count: r.C})
	}

	var vd []struct {
		ServiceType string
		C           int64
	}
	if err := s.db.WithContext(ctx).Model(&model.Order{}).
		Where("created_at >= ?", dayStart).
		Select("service_type, COUNT(*) AS c").Group("service_type").Scan(&vd).Error; err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	for _, r := range vd {
		st.ServiceDist = append(st.ServiceDist, dto.AdminServiceCount{ServiceType: r.ServiceType, Count: r.C})
	}
	return st, nil
}

// ---------- 用户管理 ----------

// ListUsers 用户/服务方分页查询，附订单数。
func (s *AdminService) ListUsers(ctx context.Context, role, keyword string, page, size int) ([]dto.AdminUserRow, int64, error) {
	normalizePage(&page, &size)
	q := s.db.WithContext(ctx).Model(&model.User{})
	if role != "" {
		q = q.Where("role = ?", role)
	}
	if keyword != "" {
		kw := "%" + keyword + "%"
		q = q.Where("nickname LIKE ? OR openid LIKE ? OR phone LIKE ?", kw, kw, kw)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, errs.Wrap(errs.CodeInternal, err)
	}
	var list []model.User
	if err := q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&list).Error; err != nil {
		return nil, 0, errs.Wrap(errs.CodeInternal, err)
	}

	ids := make([]uint64, 0, len(list))
	for _, u := range list {
		ids = append(ids, u.ID)
	}
	orderCnt := map[uint64]int64{}
	if len(ids) > 0 {
		var cs []struct {
			UserID uint64
			C      int64
		}
		if err := s.db.WithContext(ctx).Model(&model.Order{}).
			Select("user_id, COUNT(*) AS c").Where("user_id IN ?", ids).
			Group("user_id").Scan(&cs).Error; err != nil {
			return nil, 0, errs.Wrap(errs.CodeInternal, err)
		}
		for _, c := range cs {
			orderCnt[c.UserID] = c.C
		}
	}
	out := make([]dto.AdminUserRow, 0, len(list))
	for _, u := range list {
		out = append(out, dto.AdminUserRow{
			ID: u.ID, Nickname: u.Nickname, OpenID: u.OpenID, Phone: u.Phone,
			Role: u.Role, Status: u.Status, OrderCount: orderCnt[u.ID], CreatedAt: u.CreatedAt,
		})
	}
	return out, total, nil
}

// SetUserRole 角色切换：升为服务方（可被派单）或降回住户。
func (s *AdminService) SetUserRole(ctx context.Context, id uint64, role string) error {
	if role != "user" && role != "provider" {
		return errs.BizMsg(errs.CodeInvalidParam, "role 只能是 user 或 provider")
	}
	var n int64
	if err := s.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).Count(&n).Error; err != nil {
		return errs.Wrap(errs.CodeInternal, err)
	}
	if n == 0 {
		return errs.Biz(errs.CodeNotFound)
	}
	return s.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).
		Update("role", role).Error
}

// SetUserStatus 封禁/解封。封禁后小程序端登录被拒绝。
func (s *AdminService) SetUserStatus(ctx context.Context, id uint64, status int8) error {
	if status != model.UserStatusNormal && status != model.UserStatusBanned {
		return errs.BizMsg(errs.CodeInvalidParam, "status 只能是 0 或 1")
	}
	res := s.db.WithContext(ctx).Model(&model.User{}).
		Where("id = ?", id).Update("status", status)
	if res.Error != nil {
		return errs.Wrap(errs.CodeInternal, res.Error)
	}
	if res.RowsAffected == 0 {
		return errs.Biz(errs.CodeNotFound)
	}
	return nil
}

// ---------- 月卡核销 ----------

// ListRedemptions 月卡核销记录分页，可按服务日期过滤。
func (s *AdminService) ListRedemptions(ctx context.Context, date string, page, size int) ([]dto.AdminRedemptionRow, int64, error) {
	normalizePage(&page, &size)
	q := s.db.WithContext(ctx).Model(&model.MonthlyRedemption{})
	if date != "" {
		d, err := model.ParseDate(date)
		if err != nil {
			return nil, 0, errs.BizMsg(errs.CodeInvalidParam, "日期格式应为 2006-01-02")
		}
		q = q.Where("service_date = ?", d.Time())
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, errs.Wrap(errs.CodeInternal, err)
	}
	var list []model.MonthlyRedemption
	if err := q.Order("service_date DESC, id DESC").Offset((page - 1) * size).
		Limit(size).Find(&list).Error; err != nil {
		return nil, 0, errs.Wrap(errs.CodeInternal, err)
	}

	ids := make([]uint64, 0, len(list))
	for _, r := range list {
		ids = append(ids, r.OrderID)
	}
	orderNoMap := map[uint64]string{}
	if len(ids) > 0 {
		var os []model.Order
		if err := s.db.WithContext(ctx).Select("id, order_no").Where("id IN ?", ids).Find(&os).Error; err != nil {
			return nil, 0, errs.Wrap(errs.CodeInternal, err)
		}
		for _, o := range os {
			orderNoMap[o.ID] = o.OrderNo
		}
	}
	out := make([]dto.AdminRedemptionRow, 0, len(list))
	for _, r := range list {
		out = append(out, dto.AdminRedemptionRow{
			ID: r.ID, OrderNo: orderNoMap[r.OrderID],
			ServiceDate: r.ServiceDate.Time().Format("2006-01-02"),
			OperatorID:  r.OperatorID, Remark: r.Remark, CreatedAt: r.CreatedAt,
		})
	}
	return out, total, nil
}

// ---------- 服务范围修改 ----------

// UpdateArea 修改范围规则。只改规则本身，不碰启用状态（启停走独立接口）。
func (s *AdminService) UpdateArea(ctx context.Context, id uint64, req dto.UpdateAreaReq) error {
	parity := req.HouseParity
	if parity == "" {
		parity = "all"
	}
	res := s.db.WithContext(ctx).Model(&model.ServiceArea{}).Where("id = ?", id).
		Updates(map[string]any{
			"community":    req.Community,
			"lane":         req.Lane,
			"house_from":   req.HouseFrom,
			"house_to":     req.HouseTo,
			"house_parity": parity,
			"updated_at":   time.Now(),
		})
	if res.Error != nil {
		return errs.Wrap(errs.CodeInternal, res.Error)
	}
	if res.RowsAffected == 0 {
		return errs.Biz(errs.CodeNotFound)
	}
	return nil
}

// ---------- 时段模板复制 ----------

// CopySlots 把某天的时段配置整份复制到目标日期。已存在的目标时段会被覆盖 total。
func (s *AdminService) CopySlots(ctx context.Context, sourceDate string, targetDates []string) (int, error) {
	src, err := model.ParseDate(sourceDate)
	if err != nil {
		return 0, errs.BizMsg(errs.CodeInvalidParam, "source_date 格式应为 2006-01-02")
	}
	if len(targetDates) == 0 {
		return 0, errs.BizMsg(errs.CodeInvalidParam, "至少一个目标日期")
	}
	rows, err := s.slots.ListSlotsByDate(ctx, src)
	if err != nil {
		return 0, errs.Wrap(errs.CodeInternal, err)
	}
	if len(rows) == 0 {
		return 0, errs.BizMsg(errs.CodeInvalidParam, "源日期没有可复制的时段配置")
	}
	applied := 0
	for _, ds := range targetDates {
		d, err := model.ParseDate(ds)
		if err != nil {
			return applied, errs.BizMsg(errs.CodeInvalidParam, "目标日期格式应为 2006-01-02: "+ds)
		}
		for _, r := range rows {
			if err := s.slots.SetSlotTotal(ctx, d, r.SlotCode, r.ServiceType, r.Total); err != nil {
				return applied, errs.Wrap(errs.CodeInternal, err)
			}
			applied++
		}
	}
	return applied, nil
}
