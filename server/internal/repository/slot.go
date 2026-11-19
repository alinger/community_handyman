package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/alinger/community-handyman/server/internal/model"
)

// SlotRepo 时段库存数据访问。
// 并发扣减用单条 UPDATE 的原子性保证，条件里带 used < total，
// 影响行数为 0 即表示已满或不开放，无需额外加锁。
type SlotRepo struct {
	db *gorm.DB
}

func NewSlotRepo(db *gorm.DB) *SlotRepo { return &SlotRepo{db: db} }

func (r *SlotRepo) WithTx(tx *gorm.DB) *SlotRepo { return &SlotRepo{db: tx} }

// Get 查询指定日期的时段库存。
func (r *SlotRepo) Get(ctx context.Context, date model.Date, slotCode string, st model.ServiceType) (*model.SlotInventory, error) {
	var s model.SlotInventory
	err := r.db.WithContext(ctx).
		Where("service_date = ? AND slot_code = ? AND service_type = ?", date.Time(), slotCode, st).
		First(&s).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &s, nil
}

// ListByDate 返回某天某服务的全部时段，供预约页渲染可选时段。
func (r *SlotRepo) ListByDate(ctx context.Context, date model.Date, st model.ServiceType) ([]model.SlotInventory, error) {
	var list []model.SlotInventory
	err := r.db.WithContext(ctx).
		Where("service_date = ? AND service_type = ?", date.Time(), st).
		Order("slot_code").
		Find(&list).Error
	return list, err
}

// Reserve 预占一个名额。返回 false 表示已满或该时段未开放。
func (r *SlotRepo) Reserve(ctx context.Context, date model.Date, slotCode string, st model.ServiceType) (bool, error) {
	res := r.db.WithContext(ctx).Model(&model.SlotInventory{}).
		Where("service_date = ? AND slot_code = ? AND service_type = ? AND used < total", date.Time(), slotCode, st).
		Updates(map[string]any{
			"used":       gorm.Expr("used + 1"),
			"updated_at": time.Now(),
		})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

// Release 释放名额，用于关单与取消。不会把 used 减成负数。
func (r *SlotRepo) Release(ctx context.Context, date model.Date, slotCode string, st model.ServiceType) error {
	return r.db.WithContext(ctx).Model(&model.SlotInventory{}).
		Where("service_date = ? AND slot_code = ? AND service_type = ? AND used > 0", date.Time(), slotCode, st).
		Updates(map[string]any{
			"used":       gorm.Expr("used - 1"),
			"updated_at": time.Now(),
		}).Error
}
