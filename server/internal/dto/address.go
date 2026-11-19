// Package dto 定义请求与响应结构，绑定与校验规则写在这里。
package dto

// CreateAddressReq 新增地址。
type CreateAddressReq struct {
	Community    string `json:"community" binding:"required,max=64"`
	Lane         string `json:"lane" binding:"required,max=64"`
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
	InServiceArea bool   `json:"in_service_area"`
}

// CheckAreaReq 地址范围预校验，用于新增地址前的即时提示。
type CheckAreaReq struct {
	Community string `json:"community" binding:"required,max=64"`
	Lane      string `json:"lane" binding:"required,max=64"`
	HouseNo   string `json:"house_no" binding:"required,max=32"`
}
