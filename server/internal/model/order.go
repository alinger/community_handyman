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
	// ExtraSlotCode 包月备选服务时间：选中后每日按它上门，取代默认时段；空串表示按默认时段
	ExtraSlotCode string `gorm:"size:16;not null;default:''" json:"extra_slot_code"`
	// MonthlyStart / MonthlyEnd 包月服务期（含首尾两天），单次订单为空
	MonthlyStart *Date    `gorm:"type:date" json:"monthly_start,omitempty"`
	MonthlyEnd   *Date    `gorm:"type:date" json:"monthly_end,omitempty"`
	Quantity     int      `gorm:"not null;default:1" json:"quantity"`
	PetCount     int      `gorm:"not null;default:0" json:"pet_count"`
	DurationMin  int      `gorm:"not null;default:0" json:"duration_min"`
	DistanceKm   *float64 `gorm:"type:decimal(5,2)" json:"distance_km"`
	GarbageType  string   `gorm:"size:16;not null;default:''" json:"garbage_type"`
	Hospital     string   `gorm:"size:64;not null;default:''" json:"hospital"`
	// PetNotes 宠物备注（类型/名字/情况），遛宠服务方接单后第一眼看的信息
	PetNotes        string `gorm:"size:200;not null;default:''" json:"pet_notes"`
	// EntryMethod 进入方式：knock 敲门 / password 门禁密码 / card 门禁卡 / property 物业代开 / other 其他
	EntryMethod string `gorm:"size:32;not null;default:''" json:"entry_method"`
	// AccessInfo 门禁密码或钥匙说明。落库前用 AES-256-GCM 加密，
	// 仅服务开始前 2 小时至服务结束后对履约服务方解密下发（需求「钥匙与门禁信息」）。
	AccessInfo      string `gorm:"size:512;not null;default:''" json:"-"`
	PatientName     string `gorm:"size:32;not null;default:''" json:"patient_name"`
	ServiceItems    JSON   `gorm:"type:json" json:"service_items"`
	Remark          string `gorm:"size:200;not null;default:''" json:"remark"`
	PricingSnapshot JSON   `gorm:"type:json" json:"pricing_snapshot"`
	TotalAmount     int64  `gorm:"not null;default:0" json:"total_amount"`
	PaidAmount      int64  `gorm:"not null;default:0" json:"paid_amount"`
	// FreeQuotaUsed 标记本单由注册赠送的免费次数抵扣（金额可能为 0），财务统计要能区分
	FreeQuotaUsed  bool        `gorm:"not null;default:0" json:"free_quota_used"`
	RefundedAmount int64       `gorm:"not null;default:0" json:"refunded_amount"`
	Status         OrderStatus `gorm:"size:24;not null;index:idx_orders_user_status,priority:2;index:idx_orders_status" json:"status"`
	ProviderID     *uint64     `gorm:"index:idx_orders_provider_id" json:"provider_id"`
	PaidAt         *time.Time  `json:"paid_at"`
	FinishedAt     *time.Time  `json:"finished_at"`
	CancelReason   string      `gorm:"size:200;not null;default:''" json:"cancel_reason"`
}

func (Order) TableName() string { return "orders" }

// ProviderIDOrDefault 服务方 ID，未指派时返回 0（免费单/异常数据兜底）。
func (o Order) ProviderIDOrDefault() uint64 {
	if o.ProviderID != nil {
		return *o.ProviderID
	}
	return 0
}

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
	OrderID     uint64 `gorm:"not null;index:idx_service_records_order_id" json:"order_id"`
	MediaType   string `gorm:"size:16;not null" json:"media_type"`
	ObjectKey   string `gorm:"size:255;not null" json:"object_key"`
	URL         string `gorm:"size:512;not null;default:''" json:"url"`
	Size        int64  `gorm:"not null;default:0" json:"size"`
	DurationSec int    `gorm:"not null;default:0" json:"duration_sec"`
	// OwnerConsented 1 表示主人已同意拍摄（流程图 P2：服务中记录需经主人同意）
	OwnerConsented bool `gorm:"not null;default:0" json:"owner_consented"`
	// Stage 凭证阶段：start=开始服务凭证 / finish=完成服务凭证 / 空串=服务过程随手拍。
	// 详情页按它分组展示，历史记录没有阶段，归入过程记录。
	Stage       string `gorm:"size:16;not null;default:''" json:"stage"`
	UploaderID  uint64 `gorm:"not null;default:0" json:"uploader_id"`
	UploaderRole string `gorm:"size:16;not null;default:''" json:"uploader_role"`
}

func (ServiceRecord) TableName() string { return "service_records" }

// Quote 陪诊报价单（历史兼容）。陪诊自 000006 起按服务项自动计价，新订单不再产生报价单；
// 本表保留以让改造前的订单仍可查询报价记录。
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
