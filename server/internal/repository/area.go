package repository

import (
	"context"
	"strings"

	"gorm.io/gorm"

	"github.com/alinger/community-handyman/server/internal/model"
	"github.com/alinger/community-handyman/server/pkg/util"
)

// AreaRepo 服务范围数据访问。
type AreaRepo struct {
	db *gorm.DB
}

func NewAreaRepo(db *gorm.DB) *AreaRepo { return &AreaRepo{db: db} }

// WithTx 绑定事务，返回新的 repo 实例。
func (r *AreaRepo) WithTx(tx *gorm.DB) *AreaRepo { return &AreaRepo{db: tx} }

// ListActive 返回全部启用的范围规则。
func (r *AreaRepo) ListActive(ctx context.Context) ([]model.ServiceArea, error) {
	var areas []model.ServiceArea
	err := r.db.WithContext(ctx).Where("status = ?", 1).Order("id").Find(&areas).Error
	return areas, err
}

// Match 判断地址是否落在服务范围内，命中则返回对应规则。
// 匹配三要素：小区、路弄、门牌号（含区间与单双号）。
func (r *AreaRepo) Match(ctx context.Context, community, lane, houseNo string) (*model.ServiceArea, error) {
	var areas []model.ServiceArea
	err := r.db.WithContext(ctx).
		Where("status = ? AND community = ?", 1, community).
		Order("id").
		Find(&areas).Error
	if err != nil {
		return nil, err
	}

	house := util.ExtractFirstInt(houseNo)
	for i := range areas {
		if !laneMatched(areas[i].Lane, lane) {
			continue
		}
		if houseMatched(areas[i], house) {
			return &areas[i], nil
		}
	}
	return nil, nil
}

// laneMatched 判断地址的路弄是否属于规则的任一路段。
// 规则里多段用顿号分隔（如「汇腾路、新环东路、汇豪路、汇友路」），
// 地址只写其中一段（如「汇豪路71弄」），因此用包含关系判断。
func laneMatched(ruleLane, addrLane string) bool {
	addrLane = strings.TrimSpace(addrLane)
	if addrLane == "" {
		return false
	}
	for _, seg := range strings.Split(ruleLane, "、") {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		if strings.Contains(addrLane, seg) || strings.Contains(seg, addrLane) {
			return true
		}
	}
	return false
}

// houseMatched 判断门牌号是否命中区间与单双约束。house 为 -1 表示地址里没有数字。
func houseMatched(a model.ServiceArea, house int) bool {
	if house < 0 {
		// 地址没写门牌号时，只有全段规则（起止均为 0）可放行
		return a.HouseFrom == 0 && a.HouseTo == 0
	}
	if a.HouseFrom > 0 && house < a.HouseFrom {
		return false
	}
	if a.HouseTo > 0 && house > a.HouseTo {
		return false
	}
	switch a.HouseParity {
	case "odd":
		return house%2 == 1
	case "even":
		return house%2 == 0
	default:
		return true
	}
}
