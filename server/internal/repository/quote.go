package repository

import (
	"context"
	"time"

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

// MonthlyStat 单张月卡订单的核销汇总。
type MonthlyStat struct {
	OrderID uint64
	Used    int
	// TodayAt 今天那次打卡的时间，未打卡为 nil
	TodayAt *time.Time
	// TodayFinishedAt 今天这趟的完成时间；nil 表示已上门但还没点「完成服务」
	TodayFinishedAt *time.Time
}

// StatsByOrderIDs 一次查出多张月卡订单的已核销天数、今日打卡与完成时间。
// 列表页每屏最多 10 条，逐单 Count 就是 10 次往返；这里用一条 GROUP BY 解决。
func (r *RedemptionRepo) StatsByOrderIDs(ctx context.Context, orderIDs []uint64, today model.Date) (map[uint64]MonthlyStat, error) {
	out := make(map[uint64]MonthlyStat, len(orderIDs))
	if len(orderIDs) == 0 {
		return out, nil
	}
	type row struct {
		OrderID         uint64
		Used            int
		TodayAt         *time.Time
		TodayFinishedAt *time.Time
	}
	var rows []row
	err := r.db.WithContext(ctx).Model(&model.MonthlyRedemption{}).
		Select("order_id, COUNT(*) AS used, "+
			"MAX(CASE WHEN service_date = ? THEN created_at END) AS today_at, "+
			"MAX(CASE WHEN service_date = ? THEN finished_at END) AS today_finished_at",
			today.String(), today.String()).
		Where("order_id IN ?", orderIDs).
		Group("order_id").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, it := range rows {
		out[it.OrderID] = MonthlyStat{
			OrderID:         it.OrderID,
			Used:            it.Used,
			TodayAt:         it.TodayAt,
			TodayFinishedAt: it.TodayFinishedAt,
		}
	}
	return out, nil
}

// GetByDate 取某天那条核销记录，没有则返回 nil。
// 「完成服务」必须先确认当天真的打过卡，否则等于让服务方凭空补一个完成标记。
func (r *RedemptionRepo) GetByDate(ctx context.Context, orderID uint64, date model.Date) (*model.MonthlyRedemption, error) {
	var m model.MonthlyRedemption
	err := r.db.WithContext(ctx).
		Where("order_id = ? AND service_date = ?", orderID, date.String()).
		First(&m).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &m, nil
}

// FinishToday 给当天那条核销记录盖上完成时间。
// 带 finished_at IS NULL 条件：并发或连点两次时只有一次 RowsAffected=1，
// 调用方据此区分「刚完成」与「早就完成过」。
func (r *RedemptionRepo) FinishToday(ctx context.Context, orderID uint64, date model.Date, at time.Time) (bool, error) {
	res := r.db.WithContext(ctx).Model(&model.MonthlyRedemption{}).
		Where("order_id = ? AND service_date = ? AND finished_at IS NULL", orderID, date.String()).
		Update("finished_at", at)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
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
