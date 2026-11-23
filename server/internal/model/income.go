package model

// ProviderIncome 服务方入账流水。
// 订单完成（用户确认）时自动写入，order_id 唯一保证一笔订单只入一次账。
// 平台抽成等结算策略后续在这张表上加字段即可，不改动入账触发逻辑。
type ProviderIncome struct {
	BaseModel
	ProviderID uint64 `gorm:"not null;index:idx_provider_income_provider" json:"provider_id"`
	OrderID    uint64 `gorm:"not null;uniqueIndex:uk_provider_income_order" json:"order_id"`
	OrderNo    string `gorm:"size:32;not null" json:"order_no"`
	Amount     int64  `gorm:"not null;default:0" json:"amount"`
}

func (ProviderIncome) TableName() string { return "provider_income" }
