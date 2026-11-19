package dto

import "encoding/json"

// 本文件集中定义各接口的请求体。
// 原先这些结构散落在 handler 里用匿名 struct，导致无法生成接口文档、
// 也无法被多处复用，统一提到这里。

// CancelOrderReq 取消订单。
type CancelOrderReq struct {
	Reason string `json:"reason" binding:"max=200" example:"临时有事"`
}

// RejectQuoteReq 拒绝陪诊报价。
type RejectQuoteReq struct {
	Reason string `json:"reason" binding:"max=200" example:"价格超出预算"`
}

// ConfirmOrderReq 确认完成（当前无字段，占位保持文档一致）。
type ConfirmOrderReq struct{}

// RejectOrderReq 服务方拒单。
type RejectOrderReq struct {
	Reason string `json:"reason" binding:"max=200" example:"当天排期满"`
}

// AdvanceOrderReq 推进节点。
type AdvanceOrderReq struct {
	Node string `json:"node" binding:"required,max=32" example:"in_service"`
}

// AssignOrderReq 运营派单。
type AssignOrderReq struct {
	ProviderID uint64 `json:"provider_id" binding:"required" example:"3"`
}

// RefundReq 发起退款。amount 为 0 表示全额退剩余部分。
type RefundReq struct {
	OrderNo string `json:"order_no" binding:"required,max=32" example:"CH202611191335571191"`
	Amount  int64  `json:"amount" binding:"min=0" example:"1000"`
	Reason  string `json:"reason" binding:"required,max=200" example:"服务时长不足补偿"`
}

// CreateQuoteReq 陪诊报价。
type CreateQuoteReq struct {
	Amount      int64  `json:"amount" binding:"required,min=1" example:"15000"`
	DurationMin int    `json:"duration_min" binding:"min=0,max=600" example:"180"`
	Note        string `json:"note" binding:"max=500"`
}

// RedeemReq 包月核销。
type RedeemReq struct {
	Date   string `json:"date" binding:"required" example:"2026-11-20"`
	Remark string `json:"remark" binding:"max=200" example:"首日服务完成"`
}

// CreateAreaReq 新增服务范围。
type CreateAreaReq struct {
	Community   string `json:"community" binding:"required,max=64" example:"东方冠郡二期"`
	Lane        string `json:"lane" binding:"required,max=128" example:"众安路779弄"`
	HouseFrom   int    `json:"house_from" binding:"min=0" example:"1"`
	HouseTo     int    `json:"house_to" binding:"min=0" example:"89"`
	HouseParity string `json:"house_parity" binding:"omitempty,oneof=all odd even" example:"all"`
}

// SetAreaStatusReq 启停范围。
type SetAreaStatusReq struct {
	Status int8 `json:"status" binding:"oneof=0 1" example:"0"`
}

// SetSlotsReq 批量设置时段可接单量。
type SetSlotsReq struct {
	Date  string        `json:"date" binding:"required" example:"2026-11-20"`
	Items []SetSlotItem `json:"items" binding:"required,min=1,dive"`
}

// SetSlotItem 单条时段设置。
type SetSlotItem struct {
	SlotCode    string `json:"slot_code" binding:"required,max=16" example:"18-20"`
	ServiceType string `json:"service_type" binding:"required,oneof=garbage dogwalk accompany" example:"dogwalk"`
	Total       int    `json:"total" binding:"min=0,max=200" example:"5"`
}

// UpdateSkuReq 改价与计价规则。rules 部分合并，只传要改的键。
type UpdateSkuReq struct {
	BasePrice int64           `json:"base_price" binding:"min=0" example:"4000"`
	Rules     json.RawMessage `json:"rules" swaggertype:"object"`
}

// AdminLoginReq 后台登录。
type AdminLoginReq struct {
	Username string `json:"username" binding:"required,max=32" example:"admin"`
	Password string `json:"password" binding:"required,max=64" example:"your-password"`
}

// DevLoginReq 本地联调登录，仅 dev 环境注册。
type DevLoginReq struct {
	OpenID   string `json:"openid" binding:"required,max=64" example:"resident_01"`
	Nickname string `json:"nickname" binding:"max=64" example:"住户甲"`
	Role     string `json:"role" binding:"omitempty,oneof=user provider admin" example:"user"`
}
