package repository

import (
	"context"

	"gorm.io/gorm"

	"github.com/alinger/community-handyman/server/internal/model"
)

// UserRepo 用户数据访问。
type UserRepo struct {
	db *gorm.DB
}

func NewUserRepo(db *gorm.DB) *UserRepo { return &UserRepo{db: db} }

func (r *UserRepo) GetByID(ctx context.Context, id uint64) (*model.User, error) {
	var u model.User
	if err := r.db.WithContext(ctx).First(&u, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

func (r *UserRepo) GetByOpenID(ctx context.Context, openid string) (*model.User, error) {
	var u model.User
	if err := r.db.WithContext(ctx).Where("openid = ?", openid).First(&u).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

func (r *UserRepo) Create(ctx context.Context, u *model.User) error {
	return r.db.WithContext(ctx).Create(u).Error
}

// UpdateRole 更新角色。
func (r *UserRepo) UpdateRole(ctx context.Context, userID uint64, role string) error {
	return r.db.WithContext(ctx).Model(&model.User{}).
		Where("id = ?", userID).Update("role", role).Error
}

// AddressRepo 地址数据访问。
type AddressRepo struct {
	db *gorm.DB
}

func NewAddressRepo(db *gorm.DB) *AddressRepo { return &AddressRepo{db: db} }

// Create 保存地址。
func (r *AddressRepo) Create(ctx context.Context, a *model.Address) error {
	return r.db.WithContext(ctx).Create(a).Error
}

// GetByID 按主键查询，同时校验归属，避免越权读取他人地址。
func (r *AddressRepo) GetByID(ctx context.Context, userID, id uint64) (*model.Address, error) {
	var a model.Address
	err := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).First(&a).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &a, nil
}

// Update 保存地址可编辑字段（含归属条件，防越权改他人地址）。
func (r *AddressRepo) Update(ctx context.Context, userID uint64, a *model.Address) error {
	res := r.db.WithContext(ctx).
		Model(&model.Address{}).
		Where("id = ? AND user_id = ?", a.ID, userID).
		Updates(map[string]interface{}{
			"community":       a.Community,
			"lane":            a.Lane,
			"house_no":        a.HouseNo,
			"extra":           a.Extra,
			"contact_name":    a.ContactName,
			"contact_phone":   a.ContactPhone,
			"in_service_area": a.InServiceArea,
			"area_id":         a.AreaID,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ListByUser 返回用户的全部地址，默认地址优先。
func (r *AddressRepo) ListByUser(ctx context.Context, userID uint64) ([]model.Address, error) {
	var list []model.Address
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("id DESC").Find(&list).Error
	return list, err
}
