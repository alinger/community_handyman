package model

import "time"

// MonthlyRedemption 包月核销记录。
// 唯一索引 (order_id, service_date) 是不可省略的：重复核销等于多扣用户一天的权益，
// 而包月「不累计、不顺延」，扣错了无法补回。
//
// 一天分两段：CreatedAt 是「人到了」（打卡），FinishedAt 是「这趟收尾了」。
// 打卡就记一天用量（权益当天就用掉了），完成只是这一趟的结束标记——
// 少了它，服务方点一下打卡当天就算服务完成，看不出袋子到底提走没有。
type MonthlyRedemption struct {
	BaseModel
	OrderID     uint64     `gorm:"not null;uniqueIndex:uk_monthly_order_date" json:"order_id"`
	ServiceDate Date       `gorm:"type:date;not null;uniqueIndex:uk_monthly_order_date" json:"service_date"`
	OperatorID  uint64     `gorm:"not null;default:0" json:"operator_id"`
	Remark      string     `gorm:"size:200;not null;default:''" json:"remark"`
	FinishedAt  *time.Time `gorm:"type:datetime" json:"finished_at,omitempty"`
}

func (MonthlyRedemption) TableName() string { return "monthly_redemptions" }
