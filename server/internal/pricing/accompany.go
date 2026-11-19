package pricing

import (
	"encoding/json"
	"fmt"
)

// HospitalOption 后台配置的就诊医院选项。用户只能从这些选项里选，不能自由填写。
type HospitalOption struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// AccompanyItem 后台配置的可勾选陪诊服务项。Price 单位为分，是唯一的计价依据。
type AccompanyItem struct {
	Code  string `json:"code"`
	Name  string `json:"name"`
	Price int64  `json:"price"`
}

// AccompanyOptions 陪诊可选项，来自 service_skus.rules。
// 后台通过 PUT /api/admin/skus/{id} 的 rules 部分合并来改，改完不用发版。
type AccompanyOptions struct {
	Hospitals    []HospitalOption `json:"hospitals"`
	ServiceItems []AccompanyItem  `json:"service_items"`
	// BaseItem 默认必选的基础服务费。未配置时为 nil，计价退化为旧的「只按所选项目求和」。
	BaseItem *AccompanyItem `json:"base_item,omitempty"`
}

// ParseAccompanyOptions 解析 SKU 规则中的陪诊选项。
// 计价、下单校验、用户端选项接口共用同一个解析入口，避免各处对 JSON 结构理解不一致。
func ParseAccompanyOptions(rules json.RawMessage) (AccompanyOptions, error) {
	var out AccompanyOptions
	if len(rules) == 0 {
		return out, nil
	}
	if err := json.Unmarshal(rules, &out); err != nil {
		return out, fmt.Errorf("解析陪诊选项配置失败: %w", err)
	}
	return out, nil
}

// FindItem 按 code 查服务项，返回后台配置的价格。
func (o AccompanyOptions) FindItem(code string) (AccompanyItem, bool) {
	for _, it := range o.ServiceItems {
		if it.Code == code {
			return it, true
		}
	}
	return AccompanyItem{}, false
}

// HospitalValid 判断医院 code 是否来自后台配置。
func (o AccompanyOptions) HospitalValid(code string) bool {
	if code == "" {
		return false
	}
	for _, h := range o.Hospitals {
		if h.Code == code {
			return true
		}
	}
	return false
}
