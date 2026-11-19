package repository

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/alinger/community-handyman/server/internal/model"
)

// RedemptionRepo 包月核销记录数据访问。
type RedemptionRepo struct {
	db *gorm.DB
}

func NewRedemptionRepo(db *gorm.DB) *RedemptionRepo { return &RedemptionRepo{db: db} }

func (r *RedemptionRepo) WithTx(tx *gorm.DB) *RedemptionRepo { return &RedemptionRepo{db: tx} }

// Create 写入核销记录。唯一索引冲突即表示当天已核销过。
func (r *RedemptionRepo) Create(ctx context.Context, m *model.MonthlyRedemption) error {
	res := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(m)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrDuplicate
	}
	return nil
}

// ListByOrder 按日期正序返回核销记录。
func (r *RedemptionRepo) ListByOrder(ctx context.Context, orderID uint64) ([]model.MonthlyRedemption, error) {
	var list []model.MonthlyRedemption
	err := r.db.WithContext(ctx).Where("order_id = ?", orderID).Order("service_date").Find(&list).Error
	return list, err
}

// CountByOrder 已核销次数。
func (r *RedemptionRepo) CountByOrder(ctx context.Context, orderID uint64) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&model.MonthlyRedemption{}).
		Where("order_id = ?", orderID).Count(&n).Error
	return n, err
}

// QuoteRepo 陪诊报价单数据访问。
type QuoteRepo struct {
	db *gorm.DB
}

func NewQuoteRepo(db *gorm.DB) *QuoteRepo { return &QuoteRepo{db: db} }

func (r *QuoteRepo) WithTx(tx *gorm.DB) *QuoteRepo { return &QuoteRepo{db: tx} }

func (r *QuoteRepo) Create(ctx context.Context, q *model.Quote) error {
	return r.db.WithContext(ctx).Create(q).Error
}

// LatestByOrder 取该订单最新一条报价。
func (r *QuoteRepo) LatestByOrder(ctx context.Context, orderID uint64) (*model.Quote, error) {
	var q model.Quote
	err := r.db.WithContext(ctx).Where("order_id = ?", orderID).Order("id DESC").First(&q).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &q, nil
}

// Confirm 标记用户已确认报价，只对未确认的记录生效。
func (r *QuoteRepo) Confirm(ctx context.Context, quoteID uint64) (bool, error) {
	res := r.db.WithContext(ctx).Model(&model.Quote{}).
		Where("id = ? AND confirmed_at IS NULL", quoteID).
		Update("confirmed_at", gorm.Expr("NOW(3)"))
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}
