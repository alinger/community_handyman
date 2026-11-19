package service

import (
	"context"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/alinger/community-handyman/server/internal/dto"
	"github.com/alinger/community-handyman/server/internal/errs"
	"github.com/alinger/community-handyman/server/internal/model"
	"github.com/alinger/community-handyman/server/pkg/util"
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

// ---------- 后台账号管理 ----------

// ListAdminAccounts 后台账号列表（仅超管调用）。
func (s *AdminService) ListAdminAccounts(ctx context.Context) ([]dto.AdminAccountRow, error) {
	var list []model.AdminUser
	if err := s.db.WithContext(ctx).Order("id").Find(&list).Error; err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	out := make([]dto.AdminAccountRow, 0, len(list))
	for _, a := range list {
		out = append(out, dto.AdminAccountRow{
			ID: a.ID, Username: a.Username, Role: a.Role, Status: a.Status, CreatedAt: a.CreatedAt,
		})
	}
	return out, nil
}

// CreateAdminAccount 新建后台账号。用户名唯一，密码 bcrypt 落库。
func (s *AdminService) CreateAdminAccount(ctx context.Context, username, password, role string) (*dto.AdminAccountRow, error) {
	username = strings.TrimSpace(username)
	if username == "" || len(password) < 6 {
		return nil, errs.BizMsg(errs.CodeInvalidParam, "用户名不能为空，密码至少 6 位")
	}
	if role != "admin" && role != "operator" {
		return nil, errs.BizMsg(errs.CodeInvalidParam, "role 只能是 admin 或 operator")
	}
	var n int64
	if err := s.db.WithContext(ctx).Model(&model.AdminUser{}).
		Where("username = ?", username).Count(&n).Error; err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	if n > 0 {
		return nil, errs.BizMsg(errs.CodeInvalidParam, "用户名已存在")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	a := &model.AdminUser{Username: username, PasswordHash: string(hash), Role: role, Status: 1}
	if err := s.db.WithContext(ctx).Create(a).Error; err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	return &dto.AdminAccountRow{ID: a.ID, Username: a.Username, Role: a.Role, Status: a.Status, CreatedAt: a.CreatedAt}, nil
}

// SetAdminAccountStatus 启停后台账号。不能停用自己，避免超管把自己锁在门外。
func (s *AdminService) SetAdminAccountStatus(ctx context.Context, id, operatorID uint64, status int8) error {
	if status != 0 && status != 1 {
		return errs.BizMsg(errs.CodeInvalidParam, "status 只能是 0 或 1")
	}
	if id == operatorID {
		return errs.BizMsg(errs.CodeInvalidParam, "不能停用自己的账号")
	}
	res := s.db.WithContext(ctx).Model(&model.AdminUser{}).
		Where("id = ?", id).Update("status", status)
	if res.Error != nil {
		return errs.Wrap(errs.CodeInternal, res.Error)
	}
	if res.RowsAffected == 0 {
		return errs.Biz(errs.CodeNotFound)
	}
	return nil
}

// ResetAdminAccountPassword 超管重置某账号密码。
func (s *AdminService) ResetAdminAccountPassword(ctx context.Context, id uint64, password string) error {
	if len(password) < 6 {
		return errs.BizMsg(errs.CodeInvalidParam, "密码至少 6 位")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return errs.Wrap(errs.CodeInternal, err)
	}
	res := s.db.WithContext(ctx).Model(&model.AdminUser{}).
		Where("id = ?", id).Update("password_hash", string(hash))
	if res.Error != nil {
		return errs.Wrap(errs.CodeInternal, res.Error)
	}
	if res.RowsAffected == 0 {
		return errs.Biz(errs.CodeNotFound)
	}
	return nil
}

// ChangeMyPassword 自助修改密码：先验旧密码，再写新哈希。
func (s *AdminService) ChangeMyPassword(ctx context.Context, adminID uint64, oldPwd, newPwd string) error {
	if len(newPwd) < 6 {
		return errs.BizMsg(errs.CodeInvalidParam, "新密码至少 6 位")
	}
	var a model.AdminUser
	if err := s.db.WithContext(ctx).First(&a, adminID).Error; err != nil {
		return errs.Wrap(errs.CodeInternal, err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(a.PasswordHash), []byte(oldPwd)); err != nil {
		return errs.BizMsg(errs.CodeUnauthorized, "旧密码不正确")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPwd), bcrypt.DefaultCost)
	if err != nil {
		return errs.Wrap(errs.CodeInternal, err)
	}
	return s.db.WithContext(ctx).Model(&model.AdminUser{}).
		Where("id = ?", adminID).Update("password_hash", string(hash)).Error
}

// ---------- 代客取消 / 改期 ----------

// cancelableStatuses 运营可介入关单的状态。
func cancelableStatuses() []model.OrderStatus {
	return []model.OrderStatus{
		model.OrderPendingPayment, model.OrderPendingAccept,
		model.OrderAccepted, model.OrderOnTheWay,
	}
}

// CancelForUser 运营代客取消：关单、释放时段、写节点。
// 返回应退金额（未支付为 0），退款由 handler 调 RefundService 完成原路退回。
func (s *AdminService) CancelForUser(ctx context.Context, adminID uint64, orderNo, reason string) (int64, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return 0, errs.BizMsg(errs.CodeInvalidParam, "请填写取消原因")
	}
	o, err := s.orders.GetByNo(ctx, orderNo)
	if err != nil {
		return 0, errs.Wrap(errs.CodeInternal, err)
	}
	if o == nil {
		return 0, errs.Biz(errs.CodeNotFound)
	}
	if o.Status.Terminal() {
		return 0, errs.BizMsg(errs.CodeIllegalStatus, "订单已结束，无法取消")
	}

	cancelable := false
	for _, st := range cancelableStatuses() {
		if o.Status == st {
			cancelable = true
			break
		}
	}
	if !cancelable {
		return 0, errs.BizMsg(errs.CodeIllegalStatus, "服务已开始或待确认，不能代客取消")
	}

	// 服务开始的订单不能关（未支付单除外：本就未占用履约）
	if o.Status != model.OrderPendingPayment {
		start, err := util.SlotStart(o.ServiceDate.Time(), o.SlotCode)
		if err != nil {
			return 0, errs.Wrap(errs.CodeInternal, err)
		}
		if !time.Now().Before(start) {
			return 0, errs.BizMsg(errs.CodeIllegalStatus, "服务已开始，不能代客取消")
		}
	}

	remark := "运营代客取消：" + reason
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ok, err := s.orders.WithTx(tx).UpdateStatus(ctx, o.ID, o.Status, model.OrderCanceled,
			map[string]any{"cancel_reason": remark})
		if err != nil {
			return errs.Wrap(errs.CodeInternal, err)
		}
		if !ok {
			return errs.Biz(errs.CodeIllegalStatus)
		}
		if err := s.slots.WithTx(tx).Release(ctx, o.ServiceDate, o.SlotCode, o.ServiceType); err != nil {
			return errs.Wrap(errs.CodeInternal, err)
		}
		return s.orders.WithTx(tx).AddEvent(ctx, &model.OrderEvent{
			OrderID:    o.ID,
			Node:       string(model.OrderCanceled),
			Operator:   "admin",
			OperatorID: &adminID,
			Remark:     remark,
		})
	})
	if err != nil {
		return 0, err
	}
	return o.Refundable(), nil
}

// Reschedule 运营代客改期：校验目标时段余量，同事务内释放旧时段、占用新时段。
func (s *AdminService) Reschedule(ctx context.Context, adminID uint64, orderNo, dateStr, slotCode string) error {
	o, err := s.orders.GetByNo(ctx, orderNo)
	if err != nil {
		return errs.Wrap(errs.CodeInternal, err)
	}
	if o == nil {
		return errs.Biz(errs.CodeNotFound)
	}
	switch o.Status {
	case model.OrderPendingAccept, model.OrderAccepted, model.OrderOnTheWay:
	default:
		return errs.BizMsg(errs.CodeIllegalStatus, "仅待接单/已受理/待上门的订单可改期")
	}

	newDate, err := model.ParseDate(dateStr)
	if err != nil {
		return errs.BizMsg(errs.CodeInvalidParam, "日期格式应为 2006-01-02")
	}
	// 不能改到过去，也不能晚于今天 +90 天
	today, _ := model.ParseDate(time.Now().Format("2006-01-02"))
	if newDate.Time().Before(today.Time()) {
		return errs.BizMsg(errs.CodeInvalidParam, "不能改到过去的日期")
	}
	if newDate.Time().After(today.Time().AddDate(0, 0, 90)) {
		return errs.BizMsg(errs.CodeInvalidParam, "改期不能超过 90 天")
	}

	// 目标时段必须开放且有剩余名额
	slot, err := s.slots.Get(ctx, newDate, slotCode, o.ServiceType)
	if err != nil {
		return errs.Wrap(errs.CodeInternal, err)
	}
	if slot == nil || !slot.Available() {
		return errs.Biz(errs.CodeSlotFull)
	}

	sameSlot := o.ServiceDate.Time().Equal(newDate.Time()) && o.SlotCode == slotCode
	if sameSlot {
		return errs.BizMsg(errs.CodeInvalidParam, "新时段与当前相同")
	}

	remark := "运营代客改期：" + o.ServiceDate.Time().Format("2006-01-02") + " " + o.SlotCode +
		" → " + dateStr + " " + slotCode
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 先占新时段：名额被别人抢走时整体回滚
		reserved, err := s.slots.WithTx(tx).Reserve(ctx, newDate, slotCode, o.ServiceType)
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
				"service_date": newDate.Time(),
				"slot_code":    slotCode,
				"updated_at":   time.Now(),
			})
		if res.Error != nil {
			return errs.Wrap(errs.CodeInternal, res.Error)
		}
		if res.RowsAffected == 0 {
			return errs.Biz(errs.CodeIllegalStatus)
		}
		return s.orders.WithTx(tx).AddEvent(ctx, &model.OrderEvent{
			OrderID:    o.ID,
			Node:       string(o.Status),
			Operator:   "admin",
			OperatorID: &adminID,
			Remark:     remark,
		})
	})
}

// NicknameMap 批量查询用户昵称（含服务方），供订单列表回填展示。
func (s *AdminService) NicknameMap(ctx context.Context, ids []uint64) (map[uint64]string, error) {
	out := make(map[uint64]string, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	var us []model.User
	if err := s.db.WithContext(ctx).Where("id IN ?", ids).Find(&us).Error; err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	for _, u := range us {
		name := u.Nickname
		if name == "" {
			name = "用户" + strconv.FormatUint(u.ID, 10)
		}
		out[u.ID] = name
	}
	return out, nil
}
