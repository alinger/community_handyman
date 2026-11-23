package model

// UserQuota 用户免费次数额度，按服务类型各记一条。
//
// 为什么独立成表：免费次数是「营销额度」，与订单、会员都不是一回事；
// 挂在 users 上会随业务扩展不断加列（garbage_free / dogwalk_free ...）。
// total 与 used 分开存，而不是只存剩余数，便于追溯发放量与消耗量。
type UserQuota struct {
	BaseModel
	UserID      uint64      `gorm:"not null;uniqueIndex:uk_user_quotas_user_service,priority:1" json:"user_id"`
	ServiceType ServiceType `gorm:"size:16;not null;uniqueIndex:uk_user_quotas_user_service,priority:2" json:"service_type"`
	Total       int         `gorm:"not null;default:0" json:"total"`
	Used        int         `gorm:"not null;default:0" json:"used"`
}

func (UserQuota) TableName() string { return "user_quotas" }

// Remaining 剩余可用次数。
func (q UserQuota) Remaining() int {
	left := q.Total - q.Used
	if left < 0 {
		return 0
	}
	return left
}
