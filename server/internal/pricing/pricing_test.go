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
			// 没到 5 单的单次订单照常收费
			name: "单次订单未满 5 单收熟悉费",
			in: Input{ServiceType: ServiceDogWalk, Plan: PlanSingle, PetCount: 1, DurationMin: 20,
				NeedFamiliar: true, FamiliarOrders: 4,
				ServiceDate: day("2026-12-01"), Rules: json.RawMessage(dogRules)},
			want: 6000, // 40 + 20
		},
		{
			// 仅喂食按时长没意义：改时长不该影响价格
			name: "仅喂食不按时长加价",
			in: Input{ServiceType: ServiceDogWalk, PetCount: 1, FeedOnly: true, DurationMin: 60,
				ServiceDate: day("2026-12-01"), Rules: json.RawMessage(dogRules)},
			want: 3500,
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

// accompanyRules 与 000006 迁移里的陪诊配置结构一致（子集，便于断言）。
const accompanyRules = `{
  "billing": "itemized",
  "hospitals": [
    {"code": "renji_east", "name": "仁济医院东院"},
    {"code": "zhoupu", "name": "周浦医院"}
  ],
  "service_items": [
    {"code": "registration", "name": "取号", "price": 500},
    {"code": "guide", "name": "引导就诊", "price": 2000},
    {"code": "medicine", "name": "取药取报告", "price": 1000}
  ]
}`

func TestCalcAccompanyByItems(t *testing.T) {
	cases := []struct {
		name      string
		rules     string
		hospital  string
		items     []string
		want      int64
		wantItems int
		wantErr   bool
	}{
		{
			name: "多选按后台配置求和", hospital: "renji_east",
			items: []string{"registration", "guide"}, want: 2500, wantItems: 2,
		},
		{
			name: "重复 code 只计一次", hospital: "zhoupu",
			items: []string{"guide", "guide", "medicine"}, want: 3000, wantItems: 2,
		},
		{
			name: "按配置顺序输出明细", hospital: "renji_east",
			items: []string{"medicine", "registration"}, want: 1500, wantItems: 2,
		},
		{
			name: "未选服务项", hospital: "renji_east", items: nil, wantErr: true,
		},
		{
			name: "医院不在后台选项内", hospital: "随手填的医院",
			items: []string{"guide"}, wantErr: true,
		},
		{
			name: "伪造服务项 code", hospital: "renji_east",
			items: []string{"guide", "not_configured"}, wantErr: true,
		},
		{
			name: "后台未配置服务项", hospital: "renji_east",
			items: []string{"guide"}, rules: `{"hospitals":[{"code":"renji_east","name":"仁济医院东院"}]}`, wantErr: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rules := c.rules
			if rules == "" {
				rules = accompanyRules
			}
			got, err := Calc(Input{
				ServiceType: ServiceAccompany, Plan: PlanSingle,
				Hospital: c.hospital, ServiceItems: c.items,
				Rules: json.RawMessage(rules),
			})
			if c.wantErr {
				if err == nil {
					t.Fatalf("应当报错，实际得到金额 %d", got.Total)
				}
				return
			}
			if err != nil {
				t.Fatalf("陪诊计价报错: %v", err)
			}
			if got.ManualQuote {
				t.Fatal("陪诊已改为自动计价，不应再标记人工报价")
			}
			if got.Total != c.want {
				t.Fatalf("金额不符，期望 %d 实际 %d，明细 %+v", c.want, got.Total, got.Items)
			}
			if len(got.Items) != c.wantItems {
				t.Fatalf("明细条数不符，期望 %d 实际 %d", c.wantItems, len(got.Items))
			}
		})
	}
}

func TestParseAccompanyOptions(t *testing.T) {
	opts, err := ParseAccompanyOptions(json.RawMessage(accompanyRules))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if !opts.HospitalValid("renji_east") || opts.HospitalValid("unknown") {
		t.Fatal("医院选项校验不正确")
	}
	if it, ok := opts.FindItem("guide"); !ok || it.Price != 2000 || it.Name != "引导就诊" {
		t.Fatalf("服务项查找不正确: %+v, %v", it, ok)
	}
	if _, ok := opts.FindItem("unknown"); ok {
		t.Fatal("未配置的 code 不应被找到")
	}
	if opts.BaseItem != nil {
		t.Fatal("未配置基础费时 BaseItem 应为空")
	}
}

// accompanyRulesWithBase 对应 000007 迁移后的配置：基础费 1000 分，默认必选。
const accompanyRulesWithBase = `{
  "billing": "itemized",
  "hospitals": [{"code": "renji_east", "name": "仁济医院东院"}],
  "base_item": {"code": "base_fee", "name": "基础服务费", "price": 1000},
  "service_items": [
    {"code": "registration", "name": "取号", "price": 500},
    {"code": "guide", "name": "引导就诊", "price": 2000}
  ]
}`

func TestCalcAccompanyBaseItem(t *testing.T) {
	cases := []struct {
		name      string
		items     []string
		want      int64
		wantItems int
		wantErr   bool
	}{
		{
			// 基础费强制计入：不选任何服务项也能下单，只收基础费
			name: "只收基础费", items: nil, want: 1000, wantItems: 1,
		},
		{
			// 前端不能通过不传 code 绕过基础费
			name: "基础费加可选项目", items: []string{"registration", "guide"}, want: 3500, wantItems: 3,
		},
		{
			// 前端回传 base code 必须被忽略，否则会重复计一次
			name: "前端回传基础费 code 不重复计", items: []string{"base_fee", "registration"}, want: 1500, wantItems: 2,
		},
		{
			name: "伪造服务项仍被拒", items: []string{"not_configured"}, wantErr: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Calc(Input{
				ServiceType: ServiceAccompany, Plan: PlanSingle,
				Hospital: "renji_east", ServiceItems: c.items,
				Rules: json.RawMessage(accompanyRulesWithBase),
			})
			if c.wantErr {
				if err == nil {
					t.Fatalf("应当报错，实际得到金额 %d", got.Total)
				}
				return
			}
			if err != nil {
				t.Fatalf("陪诊计价报错: %v", err)
			}
			if got.Total != c.want {
				t.Fatalf("金额不符，期望 %d 实际 %d，明细 %+v", c.want, got.Total, got.Items)
			}
			if len(got.Items) != c.wantItems {
				t.Fatalf("明细条数不符，期望 %d 实际 %d", c.wantItems, len(got.Items))
			}
			if len(got.Items) == 0 || got.Items[0].Name != "基础服务费" || got.Items[0].Amount != 1000 {
				t.Fatalf("基础费必须排在明细首位且金额来自配置: %+v", got.Items)
			}
		})
	}
}

func TestParseAccompanyBaseItem(t *testing.T) {
	opts, err := ParseAccompanyOptions(json.RawMessage(accompanyRulesWithBase))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if opts.BaseItem == nil || opts.BaseItem.Code != "base_fee" || opts.BaseItem.Price != 1000 {
		t.Fatalf("基础费解析不正确: %+v", opts.BaseItem)
	}
}

func TestCalcUnknownService(t *testing.T) {
	if _, err := Calc(Input{ServiceType: "unknown"}); err == nil {
		t.Fatal("未知服务类型应当报错")
	}
}

// TestFamiliarProgress 熟悉费进度必须如实反映「已完成几单 / 还差几单」，
// 前端据此展示进度条，写错会直接误导用户。
func TestFamiliarProgress(t *testing.T) {
	cases := []struct {
		name      string
		orders    int
		needFam   bool
		wantFree  bool
		wantDone  int
		wantTotal int64
	}{
		{name: "未开熟悉开关不返回进度", orders: 3, needFam: false, wantFree: false, wantDone: 0, wantTotal: 4000},
		{name: "已完成 3 单未达标", orders: 3, needFam: true, wantFree: false, wantDone: 3, wantTotal: 6000},
		{name: "已完成 5 单达标", orders: 5, needFam: true, wantFree: true, wantDone: 5, wantTotal: 4000},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d, _ := time.Parse("2006-01-02", "2026-12-01")
			got, err := Calc(Input{
				ServiceType: ServiceDogWalk, Plan: PlanSingle, PetCount: 1, DurationMin: 20,
				NeedFamiliar: c.needFam, FamiliarOrders: c.orders,
				ServiceDate: d, Rules: json.RawMessage(dogRules),
			})
			if err != nil {
				t.Fatalf("计价报错: %v", err)
			}
			if got.Total != c.wantTotal {
				t.Fatalf("金额不符，期望 %d 实际 %d", c.wantTotal, got.Total)
			}
			if !c.needFam {
				if got.Familiar != nil {
					t.Fatalf("未开熟悉开关不应返回进度，实际 %+v", got.Familiar)
				}
				return
			}
			if got.Familiar == nil {
				t.Fatal("开启熟悉开关必须返回进度")
			}
			if got.Familiar.Done != c.wantDone || got.Familiar.Need != 5 || got.Familiar.Free != c.wantFree {
				t.Fatalf("进度不符，实际 %+v", got.Familiar)
			}
		})
	}
}
