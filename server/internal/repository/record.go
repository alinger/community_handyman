package repository

import (
	"context"

	"gorm.io/gorm"

	"github.com/alinger/community-handyman/server/internal/model"
)

// RecordRepo 服务记录数据访问。不做删除，符合「记录不可删除」的业务要求。
type RecordRepo struct {
	db *gorm.DB
}

func NewRecordRepo(db *gorm.DB) *RecordRepo { return &RecordRepo{db: db} }

func (r *RecordRepo) Create(ctx context.Context, rec *model.ServiceRecord) error {
	return r.db.WithContext(ctx).Create(rec).Error
}

func (r *RecordRepo) ListByOrder(ctx context.Context, orderID uint64) ([]model.ServiceRecord, error) {
	var list []model.ServiceRecord
	err := r.db.WithContext(ctx).Where("order_id = ?", orderID).Order("id").Find(&list).Error
	return list, err
}

func (r *RecordRepo) GetByID(ctx context.Context, id uint64) (*model.ServiceRecord, error) {
	var rec model.ServiceRecord
	err := r.db.WithContext(ctx).First(&rec, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &rec, nil
}

// ListByProvider 查询某服务方名下的订单。
func (r *OrderRepo) ListByProvider(ctx context.Context, providerID uint64, tab string, page, size int) ([]model.Order, int64, error) {
	q := r.db.WithContext(ctx).Model(&model.Order{}).Where("provider_id = ?", providerID)
	switch tab {
	case "pending":
		q = q.Where("status IN ?", []model.OrderStatus{
			model.OrderPendingAccept, model.OrderAccepted, model.OrderOnTheWay})
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
	err := q.Order("service_date, id").Offset((page - 1) * size).Limit(size).Find(&list).Error
	return list, total, err
}

// ListOpenForProvider 查询可接单的订单（已支付、未派单）。
func (r *OrderRepo) ListOpenForProvider(ctx context.Context, serviceType string, page, size int) ([]model.Order, int64, error) {
	q := r.db.WithContext(ctx).Model(&model.Order{}).
		Where("status = ? AND provider_id IS NULL", model.OrderPendingAccept)
	if serviceType != "" {
		q = q.Where("service_type = ?", serviceType)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []model.Order
	err := q.Order("service_date, slot_code, id").Offset((page - 1) * size).Limit(size).Find(&list).Error
	return list, total, err
}
