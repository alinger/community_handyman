package dto

import "time"

// CreateOrderReq 创建订单。
type CreateOrderReq struct {
	AddressID    uint64  `json:"address_id" binding:"required"`
	ServiceType  string  `json:"service_type" binding:"required,oneof=garbage dogwalk accompany"`
	Plan         string  `json:"plan" binding:"omitempty,oneof=single monthly"`
	ServiceDate  string  `json:"service_date" binding:"required"` // 2006-01-02
	SlotCode     string  `json:"slot_code" binding:"required,max=16"`
	Quantity     int     `json:"quantity" binding:"omitempty,min=0,max=50"`
	PetCount     int     `json:"pet_count" binding:"omitempty,min=0,max=10"`
	DurationMin  int     `json:"duration_min" binding:"omitempty,min=0,max=120"`
	DistanceKm   float64 `json:"distance_km" binding:"omitempty,min=0,max=50"`
	GarbageType  string  `json:"garbage_type" binding:"omitempty,oneof=kitchen other recyclable"`
	Remark       string  `json:"remark" binding:"max=200"`
	FeedOnly     bool    `json:"feed_only"`
	NeedFamiliar bool    `json:"need_familiar"`

	// 陪诊专用：医院与服务项只回传 code，价格由后端按 SKU 规则计算
	Hospital     string   `json:"hospital" binding:"omitempty,max=64"`
	PatientName  string   `json:"patient_name" binding:"omitempty,max=32"`
	ServiceItems []string `json:"service_items" binding:"omitempty,max=20,dive,max=32"`
}

// PreviewOrderReq 费用试算，字段与下单一致，但不产生订单。
type PreviewOrderReq = CreateOrderReq

// PriceItem 费用明细项。
type PriceItem struct {
	Name   string `json:"name"`
	Amount int64  `json:"amount"`
	Note   string `json:"note,omitempty"`
}

// PreviewOrderResp 试算结果。
type PreviewOrderResp struct {
	Total       int64       `json:"total"`
	Items       []PriceItem `json:"items"`
	ManualQuote bool        `json:"manual_quote"`
}

// OrderResp 订单响应。
type OrderResp struct {
	OrderNo      string       `json:"order_no"`
	ServiceType  string       `json:"service_type"`
	Plan         string       `json:"plan"`
	ServiceDate  string       `json:"service_date"`
	SlotCode     string       `json:"slot_code"`
	Status       string       `json:"status"`
	StatusLabel  string       `json:"status_label"`
	Tab          string       `json:"tab"`
	Nodes        []NodeResp   `json:"nodes"`
	TotalAmount  int64        `json:"total_amount"`
	PaidAmount   int64        `json:"paid_amount"`
	Remark       string       `json:"remark"`
	CreatedAt    time.Time    `json:"created_at"`
	Address      *AddressResp `json:"address,omitempty"`
	PriceItems   []PriceItem  `json:"price_items,omitempty"`
	PayExpiresAt *time.Time   `json:"pay_expires_at,omitempty"`
	Quote        *QuoteResp   `json:"quote,omitempty"`

	// 陪诊订单已选内容，供详情页与后台还原
	Hospital       string      `json:"hospital,omitempty"`
	PatientName    string      `json:"patient_name,omitempty"`
	AccompanyItems []PriceItem `json:"accompany_items,omitempty"`

	// 以下为后台扩展字段，用户端不填充（omitempty 不下发）
	UserNickname    string     `json:"user_nickname,omitempty"`
	UserID          uint64     `json:"user_id,omitempty"`
	ProviderID      *uint64    `json:"provider_id,omitempty"`
	ProviderNickname string    `json:"provider_nickname,omitempty"`
	Quantity        int        `json:"quantity,omitempty"`
	PetCount        int        `json:"pet_count,omitempty"`
	RefundedAmount  int64      `json:"refunded_amount,omitempty"`
	CancelReason    string     `json:"cancel_reason,omitempty"`
	PaidAt          *time.Time `json:"paid_at,omitempty"`
	FinishedAt      *time.Time `json:"finished_at,omitempty"`
	MonthlyStart    string     `json:"monthly_start,omitempty"`
	MonthlyEnd      string     `json:"monthly_end,omitempty"`
}

// QuoteResp 陪诊报价。Confirmed 为真表示用户已确认并去支付。
type QuoteResp struct {
	Amount      int64     `json:"amount"`
	DurationMin int       `json:"duration_min"`
	Confirmed   bool      `json:"confirmed"`
	CreatedAt   time.Time `json:"created_at"`
}

// NodeResp 进度节点。
type NodeResp struct {
	Node    string     `json:"node"`
	Label   string     `json:"label"`
	Reached bool       `json:"reached"`
	At      *time.Time `json:"at,omitempty"`
	Remark  string     `json:"remark,omitempty"`
}

// CreateOrderResp 下单结果。ManualQuote 为真时不返回支付参数，等待运营报价。
type CreateOrderResp struct {
	Order       OrderResp `json:"order"`
	Pay         any       `json:"pay,omitempty"`
	ManualQuote bool      `json:"manual_quote"`
	PayHint     string    `json:"pay_hint,omitempty"`
}
