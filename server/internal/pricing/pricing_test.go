package pricing

import (
	"encoding/json"
	"testing"
	"time"
)

// dogRules 与 000003 种子中的遛宠规则一致。
const dogRules = `{
  "feed_only": 3500,
  "base": 4000,
  "base_minutes": 20,
  "step_minutes": 10,
  "step_price": 1000,
  "extra_pet": 500,
  "over_3km": 1000,
  "distance_threshold_km": 3,
  "familiar": 2000,
  "familiar_free_orders": 5,
  "holiday_extra": 1500,
  "holiday_dates": ["2027-02-06", "2027-02-07"],
  "max_minutes": 120
}`

func TestCalcDogWalk(t *testing.T) {
	day := func(s string) time.Time {
		d, _ := time.ParseInLocation("2006-01-02", s, time.Local)
		return d
	}

	cases := []struct {
		name string
		in   Input
		want int64
	}{
		{
			name: "AC-06 两只 30 分钟 超 3 公里 首次含熟悉费",
			in: Input{ServiceType: ServiceDogWalk, PetCount: 2, DurationMin: 30,
				DistanceKm: 4.2, NeedFamiliar: true, FamiliarOrders: 0,
				ServiceDate: day("2026-12-01"), Rules: json.RawMessage(dogRules)},
			want: 8500, // 40 + 10 + 5 + 10 + 20
		},
		{
			name: "基础价 20 分钟 1 只 不熟悉",
			in: Input{ServiceType: ServiceDogWalk, PetCount: 1, DurationMin: 20,
				DistanceKm: 1, ServiceDate: day("2026-12-01"), Rules: json.RawMessage(dogRules)},
			want: 4000,
		},
		{
			name: "25 分钟按 30 分钟档计（向上取整）",
			in: Input{ServiceType: ServiceDogWalk, PetCount: 1, DurationMin: 25,
				DistanceKm: 1, ServiceDate: day("2026-12-01"), Rules: json.RawMessage(dogRules)},
			want: 5000, // 40 + 10
		},
		{
			name: "满 5 单免熟悉费",
			in: Input{ServiceType: ServiceDogWalk, PetCount: 1, DurationMin: 20,
				NeedFamiliar: true, FamiliarOrders: 5,
				ServiceDate: day("2026-12-01"), Rules: json.RawMessage(dogRules)},
			want: 4000,
		},
		{
			name: "配置的节日加价命中",
			in: Input{ServiceType: ServiceDogWalk, PetCount: 1, DurationMin: 20,
				ServiceDate: day("2027-02-06"), Rules: json.RawMessage(dogRules)},
			want: 5500, // 40 + 15
		},
		{
			name: "非配置日期不加价",
			in: Input{ServiceType: ServiceDogWalk, PetCount: 1, DurationMin: 20,
				ServiceDate: day("2027-02-08"), Rules: json.RawMessage(dogRules)},
			want: 4000,
		},
		{
			name: "仅喂食清洁 2 只",
			in: Input{ServiceType: ServiceDogWalk, PetCount: 2, FeedOnly: true,
				ServiceDate: day("2026-12-01"), Rules: json.RawMessage(dogRules)},
			want: 7000, // 35 × 2
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Calc(c.in)
			if err != nil {
				t.Fatalf("计价报错: %v", err)
			}
			if got.Total != c.want {
				t.Fatalf("金额不符，期望 %d 实际 %d，明细 %+v", c.want, got.Total, got.Items)
			}
			if len(got.Items) == 0 {
				t.Fatal("明细不能为空")
			}
		})
	}
}

func TestCalcDogWalkOverMaxDuration(t *testing.T) {
	in := Input{ServiceType: ServiceDogWalk, PetCount: 1, DurationMin: 180,
		ServiceDate: time.Now(), Rules: json.RawMessage(dogRules)}
	if _, err := Calc(in); err == nil {
		t.Fatal("超过时长上限应当报错")
	}
}

func TestCalcGarbage(t *testing.T) {
	rules := `{"unit":"bag","bags_included":1,"extra_bag":300,"monthly_price":20000,"monthly_daily_limit":1}`

	single, err := Calc(Input{ServiceType: ServiceGarbage, Plan: PlanSingle, BasePrice: 500,
		Quantity: 3, Rules: json.RawMessage(rules)})
	if err != nil {
		t.Fatalf("单次计价报错: %v", err)
	}
	if single.Total != 1100 { // 5 + 3×2
		t.Fatalf("单次金额应为 1100，实际 %d", single.Total)
	}

	monthly, err := Calc(Input{ServiceType: ServiceGarbage, Plan: PlanMonthly, BasePrice: 500,
		Rules: json.RawMessage(rules)})
	if err != nil {
		t.Fatalf("包月计价报错: %v", err)
	}
	if monthly.Total != 20000 {
		t.Fatalf("包月金额应为 20000，实际 %d", monthly.Total)
	}
}

func TestCalcAccompanyIsManual(t *testing.T) {
	got, err := Calc(Input{ServiceType: ServiceAccompany, Plan: PlanSingle})
	if err != nil {
		t.Fatalf("陪诊计价报错: %v", err)
	}
	if !got.ManualQuote {
		t.Fatal("陪诊应标记为人工报价")
	}
	if got.Total != 0 {
		t.Fatalf("人工报价金额应为 0，实际 %d", got.Total)
	}
}

func TestCalcUnknownService(t *testing.T) {
	if _, err := Calc(Input{ServiceType: "unknown"}); err == nil {
		t.Fatal("未知服务类型应当报错")
	}
}
