package model

import "time"

// PaymentTransaction 支付流水。
// transaction_id 的唯一索引是整个支付幂等的唯一依靠：重复回调插入冲突即视为已处理，
// 不要用「先查再插」替代，并发下必然重复入账。
type PaymentTransaction struct {
	BaseModel
	OrderNo       string     `gorm:"size:32;not null;index:idx_payment_transactions_order_no" json:"order_no"`
	TransactionID string     `gorm:"size:64;not null;uniqueIndex:uk_payment_transactions_transaction_id" json:"transaction_id"`
	OutTradeNo    string     `gorm:"size:64;not null;default:''" json:"out_trade_no"`
	Amount        int64      `gorm:"not null" json:"amount"`
	Status        string     `gorm:"size:16;not null" json:"status"`
	RawCallback   JSON       `gorm:"type:json" json:"raw_callback"`
	PaidAt        *time.Time `json:"paid_at"`
}

func (PaymentTransaction) TableName() string { return "payment_transactions" }

// 支付流水状态。
const (
	PayStatusSuccess  = "success"
	PayStatusClosed   = "closed"
	PayStatusRefunded = "refunded"
)

// Refund 退款单。out_refund_no 唯一，保证同一笔退款不会重复发起。
type Refund struct {
	BaseModel
	OrderID     uint64     `gorm:"not null;index:idx_refunds_order_id" json:"order_id"`
	OutRefundNo string     `gorm:"size:64;not null;uniqueIndex:uk_refunds_out_refund_no" json:"out_refund_no"`
	RefundID    string     `gorm:"size:64;not null;default:''" json:"refund_id"`
	Amount      int64      `gorm:"not null" json:"amount"`
	Reason      string     `gorm:"size:200;not null;default:''" json:"reason"`
	Status      string     `gorm:"size:16;not null;index:idx_refunds_status" json:"status"`
	OperatorID  uint64     `gorm:"not null;default:0" json:"operator_id"`
	RefundedAt  *time.Time `json:"refunded_at"`
}

func (Refund) TableName() string { return "refunds" }

// 退款单状态。
const (
	RefundPending = "pending"
	RefundSuccess = "success"
	RefundFailed  = "failed"
)

// AdminUser 后台账号。password_hash 用 bcrypt，永不出现在 JSON 响应中。
type AdminUser struct {
	BaseModel
	Username     string `gorm:"size:32;not null;uniqueIndex:uk_admin_users_username" json:"username"`
	PasswordHash string `gorm:"size:128;not null" json:"-"`
	Role         string `gorm:"size:16;not null;default:'operator'" json:"role"`
	Status       int8   `gorm:"not null;default:1" json:"status"`
}

func (AdminUser) TableName() string { return "admin_users" }

// 后台角色。
const (
	AdminRoleAdmin    = "admin"
	AdminRoleOperator = "operator"
)

// IsAdmin 判断是否具备管理权限，退款等写操作要求 admin。
func (a AdminUser) IsAdmin() bool { return a.Role == AdminRoleAdmin }
