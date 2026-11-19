package service

import (
	"context"

	"github.com/alinger/community-handyman/server/internal/errs"
	"github.com/alinger/community-handyman/server/internal/model"
	"github.com/alinger/community-handyman/server/internal/repository"
)

// UserService 用户相关。
type UserService struct {
	users *repository.UserRepo
}

func NewUserService(users *repository.UserRepo) *UserService {
	return &UserService{users: users}
}

// EnsureByOpenID 按 openid 取用户，不存在则创建。
// 正式版这里是微信 code2session 的落地位置，当前用于本地联调。
// SetRole 更新用户角色。dev 造服务方账号用，正式环境角色由后台分配。
func (s *UserService) SetRole(ctx context.Context, userID uint64, role string) error {
	return s.users.UpdateRole(ctx, userID, role)
}

func (s *UserService) EnsureByOpenID(ctx context.Context, openid, nickname string) (*model.User, error) {
	u, err := s.users.GetByOpenID(ctx, openid)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	if u != nil {
		return u, nil
	}

	u = &model.User{OpenID: openid, Nickname: nickname, Role: "user"}
	if err := s.users.Create(ctx, u); err != nil {
		// 并发创建可能撞唯一索引，重查一次
		again, e2 := s.users.GetByOpenID(ctx, openid)
		if e2 == nil && again != nil {
			return again, nil
		}
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	return u, nil
}
