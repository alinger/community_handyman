// Package service 承载业务编排与事务边界。
package service

import (
	"context"

	"github.com/alinger/community-handyman/server/internal/dto"
	"github.com/alinger/community-handyman/server/internal/errs"
	"github.com/alinger/community-handyman/server/internal/model"
	"github.com/alinger/community-handyman/server/internal/repository"
)

// AddressService 地址与范围校验。
type AddressService struct {
	areas     *repository.AreaRepo
	addresses *repository.AddressRepo
}

func NewAddressService(areas *repository.AreaRepo, addresses *repository.AddressRepo) *AddressService {
	return &AddressService{areas: areas, addresses: addresses}
}

// Check 判断地址是否在服务范围内。
func (s *AddressService) Check(ctx context.Context, community, lane, houseNo string) (*model.ServiceArea, error) {
	area, err := s.areas.Match(ctx, community, lane, houseNo)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	if area == nil {
		return nil, errs.BizMsg(errs.CodeAddressOutOfArea,
			"该门牌号暂未开通服务，请确认后重试")
	}
	return area, nil
}

// Communities 返回后台配置的可选小区列表（启用的服务范围按小区去重）。
func (s *AddressService) Communities(ctx context.Context) ([]string, error) {
	list, err := s.areas.ListCommunities(ctx)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	return list, nil
}

// Create 新增地址。范围校验前置到这里，下单时不再重复判定。
func (s *AddressService) Create(ctx context.Context, userID uint64, req dto.CreateAddressReq) (*model.Address, error) {
	area, err := s.Check(ctx, req.Community, req.Lane, req.HouseNo)
	if err != nil {
		return nil, err
	}

	a := &model.Address{
		UserID:        userID,
		Community:     req.Community,
		Lane:          req.Lane,
		HouseNo:       req.HouseNo,
		Extra:         req.Extra,
		ContactName:   req.ContactName,
		ContactPhone:  req.ContactPhone,
		InServiceArea: true,
		AreaID:        &area.ID,
	}
	if err := s.addresses.Create(ctx, a); err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	return a, nil
}

// Update 编辑地址。先校验归属（GetByID 带 user_id），再重新做服务范围校验，
// 避免把地址改到服务范围外却仍标记 in_service_area=true。
func (s *AddressService) Update(ctx context.Context, userID, id uint64, req dto.UpdateAddressReq) (*model.Address, error) {
	a, err := s.addresses.GetByID(ctx, userID, id)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	if a == nil {
		return nil, errs.BizMsg(errs.CodeNotFound, "地址不存在")
	}
	area, err := s.Check(ctx, req.Community, req.Lane, req.HouseNo)
	if err != nil {
		return nil, err
	}

	a.Community = req.Community
	a.Lane = req.Lane
	a.HouseNo = req.HouseNo
	a.Extra = req.Extra
	a.ContactName = req.ContactName
	a.ContactPhone = req.ContactPhone
	a.InServiceArea = true
	a.AreaID = &area.ID
	if err := s.addresses.Update(ctx, userID, a); err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	return a, nil
}

// List 返回用户地址列表。
func (s *AddressService) List(ctx context.Context, userID uint64) ([]model.Address, error) {
	list, err := s.addresses.ListByUser(ctx, userID)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	return list, nil
}

// ToAddressResp 转换为响应结构。
func ToAddressResp(a *model.Address) dto.AddressResp {
	return dto.AddressResp{
		ID:            a.ID,
		Community:     a.Community,
		Lane:          a.Lane,
		HouseNo:       a.HouseNo,
		Extra:         a.Extra,
		ContactName:   a.ContactName,
		ContactPhone:  a.ContactPhone,
		InServiceArea: a.InServiceArea,
	}
}
