package repository

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/alinger/community-handyman/server/internal/model"
)

// IncomeRepo 服务方入账数据访问。
// order_id 唯一索引保证一笔订单只入账一次，重复触发用「冲突即忽略」消化。
type IncomeRepo struct {
	db *gorm.DB
}

func NewIncomeRepo(db *gorm.DB) *IncomeRepo { return &IncomeRepo{db: db} }

func (r *IncomeRepo) WithTx(tx *gorm.DB) *IncomeRepo { return &IncomeRepo{db: tx} }

// Create 写入入账。同一订单重复入账时静默忽略（幂等）。
func (r *IncomeRepo) Create(ctx context.Context, inc *model.ProviderIncome) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(inc).Error
}

// Summary 汇总服务方累计入账。
func (r *IncomeRepo) Summary(ctx context.Context, providerID uint64) (total int64, count int64, err error) {
	err = r.db.WithContext(ctx).Model(&model.ProviderIncome{}).
		Where("provider_id = ?", providerID).
		Select("COALESCE(SUM(amount),0) AS total, COUNT(*) AS count").
		Row().Scan(&total, &count)
	return
}

// List 最近入账明细。
func (r *IncomeRepo) List(ctx context.Context, providerID uint64, limit int) ([]model.ProviderIncome, error) {
	var list []model.ProviderIncome
	err := r.db.WithContext(ctx).
		Where("provider_id = ?", providerID).
		Order("id DESC").Limit(limit).Find(&list).Error
	return list, err
}
