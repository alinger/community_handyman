package model

import "time"

// 改期记录状态。pending 是服务方发起后等待住户确认的唯一中间态。
const (
	ReschedulePending  = "pending"
	RescheduleApplied  = "applied"
	RescheduleRejected = "rejected"
	RescheduleCanceled = "canceled"
)

// 改期发起方。住户直接改期立即生效，服务方只能申请。
const (
	RescheduleByUser     = "user"
	RescheduleByProvider = "provider"
)

// OrderReschedule 订单改期记录。
// 订单表的 service_date/slot_code 被覆盖后就找不回原时间了，
// 因此每次改期都独立留一条 from → to 的快照。
type OrderReschedule struct {
	BaseModel
	OrderID     uint64     `gorm:"not null;index:idx_order_reschedules_order,priority:1" json:"order_id"`
	OrderNo     string     `gorm:"size:32;not null;index:idx_order_reschedules_order_no" json:"order_no"`
	FromDate    Date       `gorm:"type:date;not null" json:"from_date"`
	FromSlot    string     `gorm:"size:16;not null" json:"from_slot"`
	ToDate      Date       `gorm:"type:date;not null" json:"to_date"`
	ToSlot      string     `gorm:"size:16;not null" json:"to_slot"`
	Reason      string     `gorm:"size:200;not null;default:''" json:"reason"`
	InitiatedBy string     `gorm:"size:16;not null" json:"initiated_by"`
	InitiatedID uint64     `gorm:"not null;default:0" json:"initiated_id"`
	Status      string     `gorm:"size:16;not null;index:idx_order_reschedules_order,priority:2" json:"status"`
	HandledAt   *time.Time `json:"handled_at"`
	// ProviderAckAt 服务方确认知悉这次改期的时间。
	// 住户改期是立即生效的，服务方可能已经按旧时间排好了行程，
	// 所以在他点「我知道了」之前，列表与详情都会一直挂提示。
	ProviderAckAt *time.Time `json:"provider_ack_at"`
}

func (OrderReschedule) TableName() string { return "order_reschedules" }
