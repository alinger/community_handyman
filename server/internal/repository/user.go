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

// ListByIDs 批量查询用户（后台订单列表回填昵称用，防 N+1）。
func (r *UserRepo) ListByIDs(ctx context.Context, ids []uint64) ([]model.User, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var us []model.User
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&us).Error; err != nil {
		return nil, err
	}
	return us, nil
}

// GetByPhone 按手机号取用户。空手机号直接返回 nil，避免命中历史空值记录。
func (r *UserRepo) GetByPhone(ctx context.Context, phone string) (*model.User, error) {
	if phone == "" {
		return nil, nil
	}
	var u model.User
	if err := r.db.WithContext(ctx).Where("phone = ?", phone).First(&u).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

// SetPassword 设置登录密码哈希（注册时调用）。
func (r *UserRepo) SetPassword(ctx context.Context, userID uint64, hash string) error {
	return r.db.WithContext(ctx).Model(&model.User{}).
		Where("id = ?", userID).Update("password_hash", hash).Error
}

func (r *UserRepo) Create(ctx context.Context, u *model.User) error {
	return r.db.WithContext(ctx).Create(u).Error
}

// UpdateRole 更新角色。
func (r *UserRepo) UpdateRole(ctx context.Context, userID uint64, role string) error {
	return r.db.WithContext(ctx).Model(&model.User{}).
		Where("id = ?", userID).Update("role", role).Error
}

// UpdateProfile 更新用户资料（设置页整表提交，phone 传空串即清除）。
func (r *UserRepo) UpdateProfile(ctx context.Context, userID uint64, nickname, phone string) error {
	return r.db.WithContext(ctx).Model(&model.User{}).
		Where("id = ?", userID).
		Updates(map[string]any{"nickname": nickname, "phone": phone}).Error
}

// AddressRepo 地址数据访问。
type AddressRepo struct {
	db *gorm.DB
}

func NewAddressRepo(db *gorm.DB) *AddressRepo { return &AddressRepo{db: db} }

// Create 保存地址。用户首条地址自动设为默认，保证任何时刻都有默认地址可用。
func (r *AddressRepo) Create(ctx context.Context, a *model.Address) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&model.Address{}).Where("user_id = ?", a.UserID).Count(&count).Error; err != nil {
			return err
		}
		a.IsDefault = count == 0
		return tx.Create(a).Error
	})
}

// SetDefault 把指定地址设为默认，事务内先清除该用户其它地址的默认标记。
// GetByID 已在 service 层校验归属，这里再带 user_id 条件防越权。
func (r *AddressRepo) SetDefault(ctx context.Context, userID, id uint64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&model.Address{}).
			Where("id = ? AND user_id = ?", id, userID).
			Update("is_default", true)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return tx.Model(&model.Address{}).
			Where("user_id = ? AND id <> ?", userID, id).
			Update("is_default", false).Error
	})
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
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("is_default DESC, id DESC").Find(&list).Error
	return list, err
}

// Delete 软删除地址。删掉的是默认地址时，把剩余最新一条提升为默认，
// 避免用户删完默认地址后 currentAddress 为空导致无法下单。
func (r *AddressRepo) Delete(ctx context.Context, userID, id uint64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var a model.Address
		if err := tx.Where("id = ? AND user_id = ?", id, userID).First(&a).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return gorm.ErrRecordNotFound
			}
			return err
		}
		if err := tx.Delete(&model.Address{}, id).Error; err != nil {
			return err
		}
		if !a.IsDefault {
			return nil
		}
		var next model.Address
		err := tx.Where("user_id = ?", userID).Order("id DESC").First(&next).Error
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				return nil // 用户没有其它地址了，无需提升
			}
			return err
		}
		return tx.Model(&model.Address{}).Where("id = ?", next.ID).Update("is_default", true).Error
	})
}
