package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/alinger/community-handyman/server/internal/model"
)

// SmsRepo 短信验证码数据访问。
type SmsRepo struct {
	db *gorm.DB
}

func NewSmsRepo(db *gorm.DB) *SmsRepo { return &SmsRepo{db: db} }

// Create 写入一条验证码记录。
func (r *SmsRepo) Create(ctx context.Context, c *model.SMSCode) error {
	return r.db.WithContext(ctx).Create(c).Error
}

// UpdateStatus 回写下发结果。
func (r *SmsRepo) UpdateStatus(ctx context.Context, id uint64, status int8) error {
	return r.db.WithContext(ctx).Model(&model.SMSCode{}).
		Where("id = ?", id).Update("status", status).Error
}

// LastSentAt 返回该手机号在指定用途下最近一次成功下发的时间。
// 下发失败的记录不计入，避免渠道故障把用户锁在 60 秒冷却里。
func (r *SmsRepo) LastSentAt(ctx context.Context, phone, scene string) (time.Time, error) {
	var row model.SMSCode
	err := r.db.WithContext(ctx).
		Select("created_at").
		Where("phone = ? AND scene = ? AND status <> ?", phone, scene, model.SMSStatusFailed).
		Order("id DESC").First(&row).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return time.Time{}, nil
		}
		return time.Time{}, err
	}
	return row.CreatedAt, nil
}

// CountSince 统计该手机号在 since 之后的下发次数（不含下发失败）。
func (r *SmsRepo) CountSince(ctx context.Context, phone string, since time.Time) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&model.SMSCode{}).
		Where("phone = ? AND status <> ? AND created_at >= ?", phone, model.SMSStatusFailed, since).
		Count(&n).Error
	return n, err
}

// LatestValid 取该手机号指定用途下最近一条未使用且未过期的记录。
func (r *SmsRepo) LatestValid(ctx context.Context, phone, scene string, now time.Time) (*model.SMSCode, error) {
	var row model.SMSCode
	err := r.db.WithContext(ctx).
		Where("phone = ? AND scene = ? AND used = 0 AND expires_at > ?", phone, scene, now).
		Order("id DESC").First(&row).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

// MarkUsed 原子消费验证码，返回是否消费成功。
// 并发下两个请求同时校验同一码时，只有一个能把 used 从 0 改为 1。
func (r *SmsRepo) MarkUsed(ctx context.Context, id uint64) (bool, error) {
	res := r.db.WithContext(ctx).Model(&model.SMSCode{}).
		Where("id = ? AND used = 0", id).Update("used", 1)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

// IncrAttempts 校验失败次数 +1。
func (r *SmsRepo) IncrAttempts(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Model(&model.SMSCode{}).
		Where("id = ?", id).UpdateColumn("attempts", gorm.Expr("attempts + 1")).Error
}

// PurgeBefore 清理过期较久的记录，避免表无限增长。
func (r *SmsRepo) PurgeBefore(ctx context.Context, before time.Time) (int64, error) {
	res := r.db.WithContext(ctx).Unscoped().
		Where("created_at < ?", before).Delete(&model.SMSCode{})
	return res.RowsAffected, res.Error
}
