package repository

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/alinger/community-handyman/server/internal/model"
)

// QuotaRepo 免费次数额度数据访问。
//
// 发放与扣减都靠单条 SQL 完成：
//   发放 INSERT ... ON DUPLICATE KEY UPDATE total = total + n（幂等累加）
//   扣减 UPDATE ... SET used = used + 1 WHERE used < total（条件更新，影响行数为 0 即额度不足）
// 这样并发下不会超发或超扣，也不需要额外加锁。
type QuotaRepo struct {
	db *gorm.DB
}

func NewQuotaRepo(db *gorm.DB) *QuotaRepo { return &QuotaRepo{db: db} }

func (r *QuotaRepo) WithTx(tx *gorm.DB) *QuotaRepo { return &QuotaRepo{db: tx} }

// Grant 给用户发 n 次免费额度。重复调用会累加，适合「注册赠送」这类一次性动作。
func (r *QuotaRepo) Grant(ctx context.Context, userID uint64, st model.ServiceType, n int) error {
	if n <= 0 {
		return nil
	}
	q := &model.UserQuota{
		UserID:      userID,
		ServiceType: st,
		Total:       n,
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "service_type"}},
		DoUpdates: clause.Assignments(map[string]any{"total": gorm.Expr("total + ?", n)}),
	}).Create(q).Error
}

// Get 读取额度。没有记录时返回总数为 0 的空对象，调用方不用区分 nil。
func (r *QuotaRepo) Get(ctx context.Context, userID uint64, st model.ServiceType) (*model.UserQuota, error) {
	var q model.UserQuota
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND service_type = ?", userID, st).
		First(&q).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return &model.UserQuota{UserID: userID, ServiceType: st}, nil
		}
		return nil, err
	}
	return &q, nil
}

// Consume 扣减一次额度。返回 false 表示额度已用完（并发下由条件更新保证不会超扣）。
func (r *QuotaRepo) Consume(ctx context.Context, userID uint64, st model.ServiceType) (bool, error) {
	res := r.db.WithContext(ctx).Model(&model.UserQuota{}).
		Where("user_id = ? AND service_type = ? AND used < total", userID, st).
		UpdateColumn("used", gorm.Expr("used + 1"))
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

// Restore 归还一次额度。订单取消时调用，避免用户白丢一次免费机会。
func (r *QuotaRepo) Restore(ctx context.Context, userID uint64, st model.ServiceType) error {
	return r.db.WithContext(ctx).Model(&model.UserQuota{}).
		Where("user_id = ? AND service_type = ? AND used > 0", userID, st).
		UpdateColumn("used", gorm.Expr("used - 1")).Error
}
