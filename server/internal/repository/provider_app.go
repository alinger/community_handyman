package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/alinger/community-handyman/server/internal/model"
)

// ProviderAppRepo 服务方资质申请数据访问。
type ProviderAppRepo struct {
	db *gorm.DB
}

func NewProviderAppRepo(db *gorm.DB) *ProviderAppRepo { return &ProviderAppRepo{db: db} }

// GetByUser 取某用户的申请记录。一个用户只有一条，没有则返回 nil。
func (r *ProviderAppRepo) GetByUser(ctx context.Context, userID uint64) (*model.ProviderApplication, error) {
	var a model.ProviderApplication
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&a).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &a, nil
}

// GetByID 按主键取。
func (r *ProviderAppRepo) GetByID(ctx context.Context, id uint64) (*model.ProviderApplication, error) {
	var a model.ProviderApplication
	err := r.db.WithContext(ctx).First(&a, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &a, nil
}

// Save 新建或整体更新申请（按 user_id 判存）。
// 重新提交沿同一行，状态回到 pending 并清空上次审核痕迹。
func (r *ProviderAppRepo) Save(ctx context.Context, a *model.ProviderApplication) error {
	exist, err := r.GetByUser(ctx, a.UserID)
	if err != nil {
		return err
	}
	if exist == nil {
		return r.db.WithContext(ctx).Create(a).Error
	}
	a.ID = exist.ID
	return r.db.WithContext(ctx).Model(&model.ProviderApplication{}).
		Where("id = ?", exist.ID).
		Updates(map[string]any{
			"real_name":         a.RealName,
			"phone":             a.Phone,
			"service_type":      a.ServiceType,
			"experience":        a.Experience,
			"id_card_no":        a.IDCardNo,
			"id_card_front_key": a.IDCardFrontKey,
			"id_card_back_key":  a.IDCardBackKey,
			"health_cert_key":   a.HealthCertKey,
			"status":            a.Status,
			"reviewer_id":       0,
			"review_note":       "",
			"reviewed_at":       nil,
			"updated_at":        time.Now(),
		}).Error
}

// List 后台分页查询。status 为空表示全部。
func (r *ProviderAppRepo) List(ctx context.Context, status string, page, size int) ([]model.ProviderApplication, int64, error) {
	q := r.db.WithContext(ctx).Model(&model.ProviderApplication{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []model.ProviderApplication
	err := q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&list).Error
	return list, total, err
}

// Review 写入审核结果。
func (r *ProviderAppRepo) Review(ctx context.Context, id uint64, status model.ProviderApplicationStatus,
	reviewerID uint64, note string, at time.Time) error {
	return r.db.WithContext(ctx).Model(&model.ProviderApplication{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":      status,
			"reviewer_id": reviewerID,
			"review_note": note,
			"reviewed_at": at,
			"updated_at":  time.Now(),
		}).Error
}

// ReviewAndGrant 审核通过并授予服务方角色，两写同一事务。
// 只更新 status='pending' 的行：并发重复点「通过」时第二次影响 0 行，
// 会整体回滚，避免重复授权或覆盖已驳回的结果。
func (r *ProviderAppRepo) ReviewAndGrant(ctx context.Context, id, userID, reviewerID uint64, at time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&model.ProviderApplication{}).
			Where("id = ? AND status = ?", id, model.AppPending).
			Updates(map[string]any{
				"status":      model.AppApproved,
				"reviewer_id": reviewerID,
				"review_note": "",
				"reviewed_at": at,
				"updated_at":  time.Now(),
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return tx.Model(&model.User{}).Where("id = ?", userID).
			Update("role", "provider").Error
	})
}

// CountByStatus 各状态数量，用于后台顶部统计。
func (r *ProviderAppRepo) CountByStatus(ctx context.Context) (map[string]int64, error) {
	type row struct {
		Status string
		N      int64
	}
	var rows []row
	err := r.db.WithContext(ctx).Model(&model.ProviderApplication{}).
		Select("status, COUNT(*) AS n").Group("status").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := map[string]int64{
		string(model.AppPending): 0, string(model.AppApproved): 0, string(model.AppRejected): 0,
	}
	for _, r := range rows {
		out[r.Status] = r.N
	}
	return out, nil
}
