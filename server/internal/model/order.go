package model

import "time"

// Order 订单主表。
// address_snapshot 与 pricing_snapshot 是刻意冗余：地址被修改或价格调整后，
// 订单仍要能还原下单当时的地址与费用明细，否则纠纷时无法举证。
type Order struct {
	BaseModel
	OrderNo         string      `gorm:"size:32;not null;uniqueIndex:uk_orders_order_no" json:"order_no"`
	UserID          uint64      `gorm:"not null;index:idx_orders_user_status,priority:1" json:"user_id"`
	ServiceType     ServiceType `gorm:"size:16;not null" json:"service_type"`
	Plan            PlanType    `gorm:"size:16;not null" json:"plan"`
	AddressID       uint64      `gorm:"not null" json:"address_id"`
	AddressSnapshot JSON        `gorm:"type:json" json:"address_snapshot"`
	ServiceDate     Date        `gorm:"type:date;not null;index:idx_orders_service_date" json:"service_date"`
	SlotCode        string      `gorm:"size:16;not null" json:"slot_code"`
	Quantity        int         `gorm:"not null;default:1" json:"quantity"`
	PetCount        int         `gorm:"not null;default:0" json:"pet_count"`
	DurationMin     int         `gorm:"not null;default:0" json:"duration_min"`
	DistanceKm      *float64    `gorm:"type:decimal(5,2)" json:"distance_km"`
	GarbageType     string      `gorm:"size:16;not null;default:''" json:"garbage_type"`
	Remark          string      `gorm:"size:200;not null;default:''" json:"remark"`
	PricingSnapshot JSON        `gorm:"type:json" json:"pricing_snapshot"`
	TotalAmount     int64       `gorm:"not null;default:0" json:"total_amount"`
	PaidAmount      int64       `gorm:"not null;default:0" json:"paid_amount"`
	RefundedAmount  int64       `gorm:"not null;default:0" json:"refunded_amount"`
	Status          OrderStatus `gorm:"size:24;not null;index:idx_orders_user_status,priority:2;index:idx_orders_status" json:"status"`
	ProviderID      *uint64     `gorm:"index:idx_orders_provider_id" json:"provider_id"`
	PaidAt          *time.Time  `json:"paid_at"`
	FinishedAt      *time.Time  `json:"finished_at"`
	CancelReason    string      `gorm:"size:200;not null;default:''" json:"cancel_reason"`
}

func (Order) TableName() string { return "orders" }

// Refundable 可退金额，用于退款校验。
func (o Order) Refundable() int64 {
	left := o.PaidAmount - o.RefundedAmount
	if left < 0 {
		return 0
	}
	return left
}

// OrderEvent 订单节点，只追加不修改，支撑进度条时间轴回放。
type OrderEvent struct {
	BaseModel
	OrderID    uint64  `gorm:"not null;index:idx_order_events_order_id" json:"order_id"`
	Node       string  `gorm:"size:32;not null" json:"node"`
	Operator   string  `gorm:"size:32;not null" json:"operator"`
	OperatorID *uint64 `json:"operator_id"`
	Remark     string  `gorm:"size:255;not null;default:''" json:"remark"`
}

func (OrderEvent) TableName() string { return "order_events" }

// ServiceRecord 服务记录，照片、视频、医嘱、医疗文书。
// 一经上传不可删除，只能追加，故业务层不提供删除接口。
type ServiceRecord struct {
	BaseModel
	OrderID      uint64 `gorm:"not null;index:idx_service_records_order_id" json:"order_id"`
	MediaType    string `gorm:"size:16;not null" json:"media_type"`
	ObjectKey    string `gorm:"size:255;not null" json:"object_key"`
	URL          string `gorm:"size:512;not null;default:''" json:"url"`
	Size         int64  `gorm:"not null;default:0" json:"size"`
	DurationSec  int    `gorm:"not null;default:0" json:"duration_sec"`
	UploaderID   uint64 `gorm:"not null;default:0" json:"uploader_id"`
	UploaderRole string `gorm:"size:16;not null;default:''" json:"uploader_role"`
}

func (ServiceRecord) TableName() string { return "service_records" }

// Quote 陪诊报价单。陪诊不做自动定价，报价、协商、用户确认三步都要留痕。
type Quote struct {
	BaseModel
	OrderID        uint64     `gorm:"not null;index:idx_quotes_order_id" json:"order_id"`
	Amount         int64      `gorm:"not null" json:"amount"`
	DurationMin    int        `gorm:"not null;default:0" json:"duration_min"`
	NegotiationLog JSON       `gorm:"type:json" json:"negotiation_log"`
	ConfirmedAt    *time.Time `json:"confirmed_at"`
	OperatorID     uint64     `gorm:"not null;default:0" json:"operator_id"`
}

func (Quote) TableName() string { return "quotes" }

// Confirmed 判断用户是否已确认报价。
func (q Quote) Confirmed() bool { return q.ConfirmedAt != nil }
