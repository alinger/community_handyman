// Package pricing 是计价引擎。
// 纯函数：输入订单参数与 SKU 规则，输出总额与明细，不访问数据库、不读配置，
// 便于用表驱动测试覆盖需求文档 AC-06 这类多条件叠加场景。
package pricing

import (
	"encoding/json"
	"fmt"
	"time"
)

// Input 是计价入参。
type Input struct {
	ServiceType ServiceType `json:"service_type"`
	Plan        PlanType    `json:"plan"`
	BasePrice   int64       `json:"base_price"` // SKU 基础价，单位分

	Quantity    int     `json:"quantity"`     // 袋数
	PetCount    int     `json:"pet_count"`    // 宠物只数
	DurationMin int     `json:"duration_min"` // 服务时长（分钟）
	DistanceKm  float64 `json:"distance_km"`  // 服务者到地址的距离
	FeedOnly    bool    `json:"feed_only"`    // 仅喂食清洁，不含遛

	Hospital     string   `json:"hospital"`      // 就诊医院 code，陪诊必填且必须来自后台选项
	PatientName  string   `json:"patient_name"`  // 就诊人
	ServiceItems []string `json:"service_items"` // 已勾选的陪诊服务项 code

	NeedFamiliar   bool `json:"need_familiar"`   // 是否需要提前上门熟悉
	FamiliarOrders int  `json:"familiar_orders"` // 该用户历史完成订单数，用于熟悉费减免

	ServiceDate time.Time       `json:"service_date"`
	Rules       json.RawMessage `json:"-"`
}

// ServiceType 与 PlanType 在计价包内单独定义，避免与 model 相互依赖。
type ServiceType string

type PlanType string

const (
	ServiceGarbage   ServiceType = "garbage"
	ServiceDogWalk   ServiceType = "dogwalk"
	ServiceAccompany ServiceType = "accompany"

	PlanSingle  PlanType = "single"
	PlanMonthly PlanType = "monthly"
)

// Item 是一条费用明细，下单时整体存入 pricing_snapshot。
type Item struct {
	Name   string `json:"name"`
	Amount int64  `json:"amount"`
	Note   string `json:"note,omitempty"`
}

// Result 是计价结果。
type Result struct {
	Total       int64  `json:"total"`        // 应付总额，单位分
	Items       []Item `json:"items"`        // 明细，必须展示给用户
	ManualQuote bool   `json:"manual_quote"` // 历史字段：陪诊改为自动计价后恒为 false
}

// DogWalkRules 遛宠计价参数，对应 service_skus.rules。
type DogWalkRules struct {
	FeedOnly           int64    `json:"feed_only"`             // 仅喂食清洁单价
	Base               int64    `json:"base"`                  // 遛狗基础价
	BaseMinutes        int      `json:"base_minutes"`          // 基础价包含时长
	StepMinutes        int      `json:"step_minutes"`          // 超出后每档分钟数
	StepPrice          int64    `json:"step_price"`            // 每档加价
	ExtraPet           int64    `json:"extra_pet"`             // 每多一只加价
	OverKm             float64  `json:"distance_threshold_km"` // 距离阈值，缺省 3 公里
	OverKmPrice        int64    `json:"over_3km"`              // 超出距离加价
	Familiar           int64    `json:"familiar"`              // 提前上门熟悉费
	FamiliarFreeOrders int      `json:"familiar_free_orders"`  // 达到该订单数免熟悉费
	HolidayExtra       int64    `json:"holiday_extra"`         // 节日加价
	HolidayDates       []string `json:"holiday_dates"`         // 可配置的加价日期
	MaxMinutes         int      `json:"max_minutes"`           // 时长上限
}

// GarbageRules 代倒垃圾计价参数。
type GarbageRules struct {
	ExtraBag        int64 `json:"extra_bag"`           // 超出 1 袋后每袋加价
	BagsIncluded    int   `json:"bags_included"`       // 基础价包含袋数
	MonthlyPrice    int64 `json:"monthly_price"`       // 包月价
	MonthlyDailyCap int   `json:"monthly_daily_limit"` // 包月每日次数
}

// Calc 计算订单金额。
func Calc(in Input) (Result, error) {
	switch in.ServiceType {
	case ServiceGarbage:
		return calcGarbage(in)
	case ServiceDogWalk:
		return calcDogWalk(in)
	case ServiceAccompany:
		return calcAccompany(in)
	default:
		return Result{}, fmt.Errorf("未知服务类型: %s", in.ServiceType)
	}
}

func calcGarbage(in Input) (Result, error) {
	var rules GarbageRules
	if len(in.Rules) > 0 {
		if err := json.Unmarshal(in.Rules, &rules); err != nil {
			return Result{}, fmt.Errorf("解析垃圾计价规则失败: %w", err)
		}
	}

	if in.Plan == PlanMonthly {
		price := rules.MonthlyPrice
		if price == 0 {
			price = in.BasePrice
		}
		return Result{
			Total: price,
			Items: []Item{{Name: "包月代倒垃圾", Amount: price, Note: fmt.Sprintf("每日 %d 次，不累计不顺延", max(rules.MonthlyDailyCap, 1))}},
		}, nil
	}

	qty := in.Quantity
	if qty <= 0 {
		qty = 1
	}
	included := rules.BagsIncluded
	if included <= 0 {
		included = 1
	}

	items := []Item{{Name: "单次代倒垃圾", Amount: in.BasePrice, Note: fmt.Sprintf("含 %d 袋", included)}}
	total := in.BasePrice
	if extra := qty - included; extra > 0 && rules.ExtraBag > 0 {
		amount := int64(extra) * rules.ExtraBag
		items = append(items, Item{Name: fmt.Sprintf("超出袋数 ×%d", extra), Amount: amount})
		total += amount
	}
	return Result{Total: total, Items: items}, nil
}

func calcDogWalk(in Input) (Result, error) {
	var r DogWalkRules
	if len(in.Rules) > 0 {
		if err := json.Unmarshal(in.Rules, &r); err != nil {
			return Result{}, fmt.Errorf("解析遛宠计价规则失败: %w", err)
		}
	}
	if r.Base == 0 {
		r.Base = in.BasePrice
	}
	if r.BaseMinutes <= 0 {
		r.BaseMinutes = 20
	}
	if r.StepMinutes <= 0 {
		r.StepMinutes = 10
	}
	if r.OverKm <= 0 {
		r.OverKm = 3
	}

	pets := in.PetCount
	if pets <= 0 {
		pets = 1
	}

	// 仅喂食清洁：按只计价，不含遛、不收熟悉费
	if in.FeedOnly {
		if r.FeedOnly <= 0 {
			return Result{}, fmt.Errorf("未配置仅喂食单价")
		}
		amount := r.FeedOnly * int64(pets)
		return Result{
			Total: amount,
			Items: []Item{{Name: "喂食清洁（不含遛）", Amount: amount, Note: fmt.Sprintf("%d 元/只 × %d 只", r.FeedOnly/100, pets)}},
		}, nil
	}

	items := []Item{{Name: "遛狗基础", Amount: r.Base, Note: fmt.Sprintf("含 %d 分钟", r.BaseMinutes)}}
	total := r.Base

	// 时长加价：超出部分按 10 分钟一档向上取整
	if in.DurationMin > r.BaseMinutes {
		if r.MaxMinutes > 0 && in.DurationMin > r.MaxMinutes {
			return Result{}, fmt.Errorf("单次时长不能超过 %d 分钟", r.MaxMinutes)
		}
		extraMin := in.DurationMin - r.BaseMinutes
		steps := (extraMin + r.StepMinutes - 1) / r.StepMinutes
		amount := int64(steps) * r.StepPrice
		items = append(items, Item{
			Name:   fmt.Sprintf("时长加价 %d 分钟", extraMin),
			Amount: amount,
			Note:   fmt.Sprintf("%d 档 × %d 元", steps, r.StepPrice/100),
		})
		total += amount
	}

	// 多宠加价：第一只含在基础价内
	if pets > 1 && r.ExtraPet > 0 {
		amount := int64(pets-1) * r.ExtraPet
		items = append(items, Item{Name: fmt.Sprintf("多宠 %d 只", pets-1), Amount: amount})
		total += amount
	}

	// 距离加价
	if in.DistanceKm > r.OverKm && r.OverKmPrice > 0 {
		items = append(items, Item{
			Name:   "超出距离",
			Amount: r.OverKmPrice,
			Note:   fmt.Sprintf("超过 %.0f 公里", r.OverKm),
		})
		total += r.OverKmPrice
	}

	// 提前上门熟悉：达到累计次数免费
	if in.NeedFamiliar && r.Familiar > 0 {
		if in.FamiliarOrders >= r.FamiliarFreeOrders && r.FamiliarFreeOrders > 0 {
			items = append(items, Item{Name: "提前上门熟悉", Amount: 0, Note: "累计订单已达标，免费"})
		} else {
			items = append(items, Item{Name: "提前上门熟悉", Amount: r.Familiar})
			total += r.Familiar
		}
	}

	// 节日加价：日期可配置，命中才加，改日期不发版
	if r.HolidayExtra > 0 && in.ServiceDate.IsZero() == false {
		if hit, name := matchHoliday(in.ServiceDate, r.HolidayDates); hit {
			items = append(items, Item{Name: "节日加价", Amount: r.HolidayExtra, Note: name})
			total += r.HolidayExtra
		}
	}

	return Result{Total: total, Items: items}, nil
}

// calcAccompany 按后台配置的服务项单价求和。
// 用户只回传 code，价格一律从 SKU 规则读取，前端无法篡改金额；
// 医院必须来自后台选项，防止用自由文本绕过配置。
func calcAccompany(in Input) (Result, error) {
	opts, err := ParseAccompanyOptions(in.Rules)
	if err != nil {
		return Result{}, err
	}
	if len(opts.ServiceItems) == 0 {
		return Result{}, fmt.Errorf("陪诊服务项尚未配置，请联系运营在后台设置")
	}
	if !opts.HospitalValid(in.Hospital) {
		return Result{}, fmt.Errorf("请选择列表中的就诊医院")
	}
	if len(in.ServiceItems) == 0 {
		return Result{}, fmt.Errorf("请至少选择一项陪诊服务")
	}

	// 先把用户勾选的 code 去重，任何一个不在配置里都直接拒绝，避免静默少算
	wanted := make(map[string]struct{}, len(in.ServiceItems))
	for _, code := range in.ServiceItems {
		if _, ok := opts.FindItem(code); !ok {
			return Result{}, fmt.Errorf("包含未配置的陪诊服务项，请刷新后重试")
		}
		wanted[code] = struct{}{}
	}

	// 按后台配置顺序展开明细，价格全部取自配置
	items := make([]Item, 0, len(wanted))
	var total int64
	for _, it := range opts.ServiceItems {
		if _, ok := wanted[it.Code]; !ok {
			continue
		}
		items = append(items, Item{Name: it.Name, Amount: it.Price})
		total += it.Price
	}
	return Result{Total: total, Items: items}, nil
}

// matchHoliday 判断日期是否落在配置的加价日期内。
func matchHoliday(d time.Time, dates []string) (bool, string) {
	day := d.Format("2006-01-02")
	for _, hd := range dates {
		if hd == day {
			return true, day
		}
	}
	return false, ""
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
