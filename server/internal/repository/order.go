package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/alinger/community-handyman/server/internal/model"
)

// OrderRepo 订单数据访问。
type OrderRepo struct {
	db *gorm.DB
}

func NewOrderRepo(db *gorm.DB) *OrderRepo { return &OrderRepo{db: db} }

func (r *OrderRepo) WithTx(tx *gorm.DB) *OrderRepo { return &OrderRepo{db: tx} }

// Create 写入订单。
func (r *OrderRepo) Create(ctx context.Context, o *model.Order) error {
	return r.db.WithContext(ctx).Create(o).Error
}

// GetByNo 按订单号查询。
func (r *OrderRepo) GetByNo(ctx context.Context, orderNo string) (*model.Order, error) {
	var o model.Order
	err := r.db.WithContext(ctx).Where("order_no = ?", orderNo).First(&o).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &o, nil
}

// ListByUser 按 Tab 分页查询。tab 取 pending / processing / completed / ""（全部）。
func (r *OrderRepo) ListByUser(ctx context.Context, userID uint64, tab string, page, size int) ([]model.Order, int64, error) {
	q := r.db.WithContext(ctx).Model(&model.Order{}).Where("user_id = ?", userID)

	switch tab {
	case "pending":
		q = q.Where("status IN ?", []model.OrderStatus{
			model.OrderPendingPayment, model.OrderPendingAccept, model.OrderAccepted, model.OrderOnTheWay})
	case "processing":
		q = q.Where("status IN ?", []model.OrderStatus{
			model.OrderInService, model.OrderPendingConfirm})
	case "completed":
		q = q.Where("status = ?", model.OrderCompleted)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var list []model.Order
	err := q.Order("id DESC").
		Offset((page - 1) * size).Limit(size).
		Find(&list).Error
	return list, total, err
}

// UpdateStatus 带前置状态的条件更新，避免并发下状态被覆盖。
// 返回 false 表示当前状态已不是 from，调用方应提示状态不允许。
func (r *OrderRepo) UpdateStatus(ctx context.Context, orderID uint64, from, to model.OrderStatus, extra map[string]any) (bool, error) {
	fields := map[string]any{
		"status":     to,
		"updated_at": time.Now(),
	}
	for k, v := range extra {
		fields[k] = v
	}
	res := r.db.WithContext(ctx).Model(&model.Order{}).
		Where("id = ? AND status = ?", orderID, from).
		Updates(fields)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

// AddEvent 追加节点。只插入不更新，保证时间轴可回放。
func (r *OrderRepo) AddEvent(ctx context.Context, e *model.OrderEvent) error {
	return r.db.WithContext(ctx).Create(e).Error
}

// Events 查询订单节点，按时间正序。
func (r *OrderRepo) Events(ctx context.Context, orderID uint64) ([]model.OrderEvent, error) {
	var list []model.OrderEvent
	err := r.db.WithContext(ctx).Where("order_id = ?", orderID).Order("id").Find(&list).Error
	return list, err
}

// CountCompleted 统计用户已完成订单数，用于遛宠熟悉费减免。
func (r *OrderRepo) CountCompleted(ctx context.Context, userID uint64, st model.ServiceType) (int, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&model.Order{}).
		Where("user_id = ? AND service_type = ? AND status = ?", userID, st, model.OrderCompleted).
		Count(&n).Error
	return int(n), err
}

// ListExpiredUnpaid 查询超时未支付订单，供关单任务使用。
func (r *OrderRepo) ListExpiredUnpaid(ctx context.Context, before time.Time, limit int) ([]model.Order, error) {
	var list []model.Order
	err := r.db.WithContext(ctx).
		Where("status = ? AND created_at < ?", model.OrderPendingPayment, before).
		Order("id").Limit(limit).Find(&list).Error
	return list, err
}
