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

	NeedFamiliar bool `json:"need_familiar"` // 是否需要提前上门熟悉
	// FamiliarOrders 只统计「单次卡」的已完成订单数（遛宠目前只有单次卡）
	FamiliarOrders int `json:"familiar_orders"`

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
	// Familiar 熟悉费累计进度；只在开启「提前上门熟悉」时非空，
	// 供前端展示「已完成 3 单，再 2 单免熟悉费」，避免用户只看到一句静态规则
	Familiar *FamiliarProgress `json:"familiar,omitempty"`
}

// FamiliarProgress 熟悉费累计进度。
// 文案里的「5 单」来自后台规则 familiar_free_orders，前端不要写死数字。
type FamiliarProgress struct {
	Done int  `json:"done"` // 已完成单次订单数
	Need int  `json:"need"` // 免熟悉费需要的单数
	Free bool `json:"free"` // 本次是否已免（达到或超过 Need）
}

// DogWalkRules 遛宠计价参数，对应 service_skus.rules。
type DogWalkRules struct {
	FeedOnly           int64    `json:"feed_only"`             // 仅喂食清洁单价
	Base               int64    `json:"base"`                  // [已废弃] 旧公式基础价，000022 起时长走档位表
	BaseMinutes        int      `json:"base_minutes"`          // [已废弃] 旧公式包含时长
	StepMinutes        int      `json:"step_minutes"`          // [已废弃] 旧公式每档分钟数
	StepPrice          int64    `json:"step_price"`            // [已废弃] 旧公式每档加价
	ExtraPet           int64    `json:"extra_pet"`             // 每多一只加价
	OverKm             float64  `json:"distance_threshold_km"` // 距离阈值，缺省 3 公里
	OverKmPrice        int64    `json:"over_3km"`              // 超出距离加价
	Familiar           int64    `json:"familiar"`              // 提前上门熟悉费
	FamiliarFreeOrders int      `json:"familiar_free_orders"`  // 达到该订单数免熟悉费
	HolidayExtra       int64    `json:"holiday_extra"`         // 节日加价
	HolidayDates       []string `json:"holiday_dates"`         // 可配置的加价日期
	MaxMinutes         int      `json:"max_minutes"`           // [已废弃] 旧公式时长上限
	// DurationOptions 服务时长档位（自 000022 起）：后台逐档配置，
	// 用户单选一档，档位金额即「基础费用」；金额 0 的档位不出现在费用明细里。
	DurationOptions []AccompanyItem `json:"duration_options"`
}

// GarbageRules 代倒垃圾计价参数。
type GarbageRules struct {
	ExtraBag        int64 `json:"extra_bag"`           // 超出 1 袋后每袋加价
	BagsIncluded    int   `json:"bags_included"`       // 基础价包含袋数
	MonthlyPrice    int64 `json:"monthly_price"`       // 包月价
	MonthlyDailyCap int   `json:"monthly_daily_limit"` // 包月每日次数
	// DurationDays 包月服务天数（当期产品为 30 天）
	DurationDays int `json:"duration_days"`
	// DefaultSlot 包月默认取件时段，用户不选就按这个派单
	DefaultSlot string `json:"default_slot"`
	// ExtraSlotOptions 除默认时段外可额外指定的候选时段
	ExtraSlotOptions []string `json:"extra_slot_options"`
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
		days := rules.DurationDays
		if days <= 0 {
			days = 30
		}
		note := fmt.Sprintf("%d 天，每日 %d 次，不累计不顺延", days, max(rules.MonthlyDailyCap, 1))
		if rules.DefaultSlot != "" {
			note += "，默认时段 " + rules.DefaultSlot
		}
		return Result{
			Total: price,
			Items: []Item{{Name: "包月代倒垃圾", Amount: price, Note: note}},
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

// findDurationOption 按分钟数查时长档位。
func findDurationOption(opts []AccompanyItem, minutes int) (AccompanyItem, bool) {
	for _, o := range opts {
		if o.Minutes == minutes {
			return o, true
		}
	}
	return AccompanyItem{}, false
}

func calcDogWalk(in Input) (Result, error) {
	var r DogWalkRules
	if len(in.Rules) > 0 {
		if err := json.Unmarshal(in.Rules, &r); err != nil {
			return Result{}, fmt.Errorf("解析遛宠计价规则失败: %w", err)
		}
	}
	if r.OverKm <= 0 {
		r.OverKm = 3
	}

	pets := in.PetCount
	if pets <= 0 {
		pets = 1
	}

	var items []Item
	var total int64
	var familiar *FamiliarProgress

	if in.FeedOnly {
		// 仅喂食清洁：按「只数 × 单价」计价，不按时长、不再叠多宠加价。
		//
		// 但上门相关的一次性费用仍然成立 —— 送不送狗出去遛，服务方都得进这个门：
		// 首次上门的熟悉费、超出距离的加价、节假日加价，与「遛不遛」无关。
		// 早前这里直接 return，导致开了仅喂食后这些费用被静默吞掉，
		// 用户看到「熟悉开关打开了却没加钱」，像是选项失效。
		if r.FeedOnly <= 0 {
			return Result{}, fmt.Errorf("未配置仅喂食单价")
		}
		amount := r.FeedOnly * int64(pets)
		items = []Item{{
			Name:   "喂食清洁（不含遛）",
			Amount: amount,
			Note:   fmt.Sprintf("%d 元/只 × %d 只", r.FeedOnly/100, pets),
		}}
		total = amount
	} else {
		// 时长计价（自 000022 起）：档位表由后台逐档配置，单选一档即订单金额，
		// 旧的「基础价含 20 分钟 + 超出按档加价」公式废弃。
		opt, ok := findDurationOption(r.DurationOptions, in.DurationMin)
		if !ok {
			return Result{}, fmt.Errorf("请选择可用的服务时长")
		}
		// 档位金额即「基础费用」明细；配成 0 元的免费档不出现在费用列表里
		if opt.Price > 0 {
			items = append(items, Item{Name: "基础费用", Amount: opt.Price, Note: opt.Name})
		}
		total += opt.Price

		// 多宠加价：第一只含在档位价内
		if pets > 1 && r.ExtraPet > 0 {
			amount := int64(pets-1) * r.ExtraPet
			items = append(items, Item{Name: fmt.Sprintf("多宠 %d 只", pets-1), Amount: amount})
			total += amount
		}
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

	// 提前上门熟悉：单次卡累计满 N 单后免
	if in.NeedFamiliar && r.Familiar > 0 {
		free := r.FamiliarFreeOrders > 0 && in.FamiliarOrders >= r.FamiliarFreeOrders
		if free {
			items = append(items, Item{Name: "提前上门熟悉", Amount: 0, Note: "累计订单已达标，免费"})
		} else {
			items = append(items, Item{Name: "提前上门熟悉", Amount: r.Familiar})
			total += r.Familiar
		}
		familiar = &FamiliarProgress{
			Done: in.FamiliarOrders,
			Need: r.FamiliarFreeOrders,
			Free: free,
		}
	}

	// 节日加价：日期可配置，命中才加，改日期不发版
	if r.HolidayExtra > 0 && in.ServiceDate.IsZero() == false {
		if hit, name := matchHoliday(in.ServiceDate, r.HolidayDates); hit {
			items = append(items, Item{Name: "节日加价", Amount: r.HolidayExtra, Note: name})
			total += r.HolidayExtra
		}
	}

	return Result{Total: total, Items: items, Familiar: familiar}, nil
}

// calcAccompany 按后台配置的计价档位计价（自 000021 起）。
// 用户单选一个档位（如 20分钟/25元），回传其 code，价格一律从 SKU 规则读取，
// 前端无法篡改金额；医院必须来自后台选项，防止用自由文本绕过配置。
// 旧的多选求和与基础服务费（base_item）已随档位模式废弃，配置由迁移清理。
func calcAccompany(in Input) (Result, error) {
	opts, err := ParseAccompanyOptions(in.Rules)
	if err != nil {
		return Result{}, err
	}
	if len(opts.ServiceItems) == 0 {
		return Result{}, fmt.Errorf("陪诊服务档位尚未配置，请联系运营在后台设置")
	}
	if !opts.HospitalValid(in.Hospital) {
		return Result{}, fmt.Errorf("请选择列表中的就诊医院")
	}

	// 档位模式：必须且只能选一个。多选/零选都拒绝，避免旧客户端语义混入。
	if len(in.ServiceItems) != 1 {
		return Result{}, fmt.Errorf("请选择一个陪诊服务档位")
	}
	it, ok := opts.FindItem(in.ServiceItems[0])
	if !ok {
		return Result{}, fmt.Errorf("所选陪诊服务档位不存在，请刷新后重试")
	}
	items := []Item{{Name: it.Name, Amount: it.Price}}
	return Result{Total: it.Price, Items: items}, nil
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
