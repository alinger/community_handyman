package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/alinger/community-handyman/server/internal/model"
)

// RescheduleRepo 改期记录数据访问。
type RescheduleRepo struct {
	db *gorm.DB
}

func NewRescheduleRepo(db *gorm.DB) *RescheduleRepo { return &RescheduleRepo{db: db} }

func (r *RescheduleRepo) WithTx(tx *gorm.DB) *RescheduleRepo { return &RescheduleRepo{db: tx} }

// Create 写入一条改期记录。
func (r *RescheduleRepo) Create(ctx context.Context, v *model.OrderReschedule) error {
	return r.db.WithContext(ctx).Create(v).Error
}

// Get 按 ID 查询。
func (r *RescheduleRepo) Get(ctx context.Context, id uint64) (*model.OrderReschedule, error) {
	var v model.OrderReschedule
	err := r.db.WithContext(ctx).First(&v, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &v, nil
}

// PendingByOrder 取该订单待住户确认的申请。同一订单同时只允许一条，
// 由服务层在创建前检查，这里按最新一条返回即可。
func (r *RescheduleRepo) PendingByOrder(ctx context.Context, orderID uint64) (*model.OrderReschedule, error) {
	var v model.OrderReschedule
	err := r.db.WithContext(ctx).
		Where("order_id = ? AND status = ?", orderID, model.ReschedulePending).
		Order("id DESC").First(&v).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &v, nil
}

// ListByOrder 按时间倒序返回该订单的改期记录。
func (r *RescheduleRepo) ListByOrder(ctx context.Context, orderID uint64, limit int) ([]model.OrderReschedule, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	var list []model.OrderReschedule
	err := r.db.WithContext(ctx).
		Where("order_id = ?", orderID).
		Order("id DESC").Limit(limit).
		Find(&list).Error
	return list, err
}

// PendingByOrderNos 批量取多个订单待确认的申请，供列表接口一次性填充。
// 列表一页最多几十条，用一次 IN 查询而不是逐单查（避免 N+1）。
// 返回 key 为订单号；同一订单若有多条（理论上不该有），取最新那条。
func (r *RescheduleRepo) PendingByOrderNos(ctx context.Context, orderNos []string) (map[string]model.OrderReschedule, error) {
	out := make(map[string]model.OrderReschedule, len(orderNos))
	if len(orderNos) == 0 {
		return out, nil
	}
	var list []model.OrderReschedule
	err := r.db.WithContext(ctx).
		Where("order_no IN ? AND status = ?", orderNos, model.ReschedulePending).
		Order("id DESC").
		Find(&list).Error
	if err != nil {
		return nil, err
	}
	for _, v := range list {
		if _, ok := out[v.OrderNo]; !ok {
			out[v.OrderNo] = v
		}
	}
	return out, nil
}

// CancelPendingByOrder 作废该订单所有待确认申请，返回作废条数。
//
// 住户自己直接把时间改掉之后，服务方那条「改到哪」的申请就失去意义了：
// 留着会让服务方一直以为还在等确认，住户也可能回头点「同意改期」把时间又改回去。
func (r *RescheduleRepo) CancelPendingByOrder(ctx context.Context, orderID uint64) (int64, error) {
	res := r.db.WithContext(ctx).Model(&model.OrderReschedule{}).
		Where("order_id = ? AND status = ?", orderID, model.ReschedulePending).
		Updates(map[string]any{
			"status":     model.RescheduleCanceled,
			"handled_at": time.Now(),
		})
	return res.RowsAffected, res.Error
}

// UnackedByUser 批量取「住户改过期、服务方还没确认知悉」的记录。
//
// 只看 initiated_by=user 且 status=applied：服务方自己发起的申请不提醒他本人；
// 被拒或作废的改期没有生效，也不该再打扰。返回 key 为订单号，同单多条取最新。
func (r *RescheduleRepo) UnackedByUser(ctx context.Context, orderNos []string) (map[string]model.OrderReschedule, error) {
	out := make(map[string]model.OrderReschedule, len(orderNos))
	if len(orderNos) == 0 {
		return out, nil
	}
	var list []model.OrderReschedule
	err := r.db.WithContext(ctx).
		Where("order_no IN ? AND initiated_by = ? AND status = ? AND provider_ack_at IS NULL",
			orderNos, model.RescheduleByUser, model.RescheduleApplied).
		Order("id DESC").
		Find(&list).Error
	if err != nil {
		return nil, err
	}
	for _, v := range list {
		if _, ok := out[v.OrderNo]; !ok {
			out[v.OrderNo] = v
		}
	}
	return out, nil
}

// AckProvider 标记服务方已知悉。条件更新保证重复点击不报错，
// 也只对「住户发起且已生效」的记录生效，避免误把别人的申请标成已读。
func (r *RescheduleRepo) AckProvider(ctx context.Context, orderID uint64) (int64, error) {
	res := r.db.WithContext(ctx).Model(&model.OrderReschedule{}).
		Where("order_id = ? AND initiated_by = ? AND status = ? AND provider_ack_at IS NULL",
			orderID, model.RescheduleByUser, model.RescheduleApplied).
		Update("provider_ack_at", time.Now())
	return res.RowsAffected, res.Error
}

// UpdateStatus 带前置状态的条件更新，避免同一申请被两个端同时处理。
// 返回 false 表示状态已被别人改过。
func (r *RescheduleRepo) UpdateStatus(ctx context.Context, id uint64, from, to string, extra map[string]any) (bool, error) {
	fields := map[string]any{"status": to}
	for k, v := range extra {
		fields[k] = v
	}
	res := r.db.WithContext(ctx).Model(&model.OrderReschedule{}).
		Where("id = ? AND status = ?", id, from).
		Updates(fields)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}
