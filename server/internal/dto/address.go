// Package dto 定义请求与响应结构，绑定与校验规则写在这里。
package dto

// CreateAddressReq 新增地址。
// 小区改为后台配置选择、路弄字段已取消，因此 lane 允许为空（兼容历史数据仍可带值）。
type CreateAddressReq struct {
	Community    string `json:"community" binding:"required,max=64"`
	Lane         string `json:"lane" binding:"max=64"`
	HouseNo      string `json:"house_no" binding:"required,max=32"`
	Extra        string `json:"extra" binding:"max=255"`
	ContactName  string `json:"contact_name" binding:"max=32"`
	ContactPhone string `json:"contact_phone" binding:"max=20"`
}

// UpdateAddressReq 编辑地址。字段与新增一致，复用同一套校验规则。
type UpdateAddressReq struct {
	Community    string `json:"community" binding:"required,max=64"`
	Lane         string `json:"lane" binding:"max=64"`
	HouseNo      string `json:"house_no" binding:"required,max=32"`
	Extra        string `json:"extra" binding:"max=255"`
	ContactName  string `json:"contact_name" binding:"max=32"`
	ContactPhone string `json:"contact_phone" binding:"max=20"`
}

// AddressResp 地址响应。
type AddressResp struct {
	ID            uint64 `json:"id"`
	Community     string `json:"community"`
	Lane          string `json:"lane"`
	HouseNo       string `json:"house_no"`
	Extra         string `json:"extra"`
	ContactName   string `json:"contact_name"`
	ContactPhone  string `json:"contact_phone"`
	IsDefault     bool   `json:"is_default"`
	InServiceArea bool   `json:"in_service_area"`
}

// CheckAreaReq 地址范围预校验，用于新增地址前的即时提示。
// lane 可空：小区已由后台配置限定，路弄取消后按小区+门牌校验。
type CheckAreaReq struct {
	Community string `json:"community" binding:"required,max=64"`
	Lane      string `json:"lane" binding:"max=64"`
	HouseNo   string `json:"house_no" binding:"required,max=32"`
}

// SlotAvailability 某日某服务类型的时段余量，用于预约页把满员时段置灰。
type SlotAvailability struct {
	SlotCode string `json:"slot_code"`
	Total    int    `json:"total"`
	Used     int    `json:"used"`
	// Remaining 剩余可约量，0 表示已满（前端置灰）
	Remaining int  `json:"remaining"`
	Available bool `json:"available"`
}

// ServiceAreaLane 一个小区下的一段服务范围（路段 + 门牌区间）。
type ServiceAreaLane struct {
	Lane string `json:"lane" example:"1 号楼"`
	// HouseFrom / HouseTo 为门牌号区间，两者都为 0 表示不限门牌
	HouseFrom int `json:"house_from" example:"1"`
	HouseTo   int `json:"house_to" example:"30"`
	// HouseParity 门牌单双：all=不限，odd=单号，even=双号
	HouseParity string `json:"house_parity" example:"odd"`
}

// ServiceAreaScope 一个已开通小区的服务范围。
type ServiceAreaScope struct {
	Community string            `json:"community" example:"东方冠郡一期"`
	Lanes     []ServiceAreaLane `json:"lanes"`
}
