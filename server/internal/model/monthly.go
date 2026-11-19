package model

// MonthlyRedemption 包月核销记录。
// 唯一索引 (order_id, service_date) 是不可省略的：重复核销等于多扣用户一天的权益，
// 而包月「不累计、不顺延」，扣错了无法补回。
type MonthlyRedemption struct {
	BaseModel
	OrderID     uint64 `gorm:"not null;uniqueIndex:uk_monthly_order_date" json:"order_id"`
	ServiceDate Date   `gorm:"type:date;not null;uniqueIndex:uk_monthly_order_date" json:"service_date"`
	OperatorID  uint64 `gorm:"not null;default:0" json:"operator_id"`
	Remark      string `gorm:"size:200;not null;default:''" json:"remark"`
}

func (MonthlyRedemption) TableName() string { return "monthly_redemptions" }
