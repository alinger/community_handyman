package repository

import (
	"context"

	"gorm.io/gorm"

	"github.com/alinger/community-handyman/server/internal/model"
)

// SkuRepo 服务产品数据访问。
type SkuRepo struct {
	db *gorm.DB
}

func NewSkuRepo(db *gorm.DB) *SkuRepo { return &SkuRepo{db: db} }

// Get 按服务类型与方案取启用中的 SKU。
func (r *SkuRepo) Get(ctx context.Context, st model.ServiceType, plan model.PlanType) (*model.ServiceSku, error) {
	var s model.ServiceSku
	err := r.db.WithContext(ctx).
		Where("service_type = ? AND plan = ? AND status = ?", st, plan, 1).
		First(&s).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &s, nil
}

// ListByType 返回某服务的全部方案，供前端展示可选方案。
func (r *SkuRepo) ListByType(ctx context.Context, st model.ServiceType) ([]model.ServiceSku, error) {
	var list []model.ServiceSku
	err := r.db.WithContext(ctx).
		Where("service_type = ? AND status = ?", st, 1).
		Order("plan").Find(&list).Error
	return list, err
}
