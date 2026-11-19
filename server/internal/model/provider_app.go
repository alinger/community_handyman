package model

import "time"

// ProviderApplicationStatus 服务方资质申请状态。
type ProviderApplicationStatus string

const (
	// AppPending 待审核：用户已提交，等待运营处理。
	AppPending ProviderApplicationStatus = "pending"
	// AppApproved 已通过：用户 role 被置为 provider。
	AppApproved ProviderApplicationStatus = "approved"
	// AppRejected 已驳回：用户可修改后重新提交。
	AppRejected ProviderApplicationStatus = "rejected"
)

// Valid 校验状态值合法性。
func (s ProviderApplicationStatus) Valid() bool {
	switch s {
	case AppPending, AppApproved, AppRejected:
		return true
	}
	return false
}

// Label 中文文案，后台列表直接展示。
func (s ProviderApplicationStatus) Label() string {
	switch s {
	case AppPending:
		return "待审核"
	case AppApproved:
		return "已通过"
	case AppRejected:
		return "已驳回"
	}
	return string(s)
}

// ProviderApplication 服务方资质申请。
//
// 一个用户只有一条记录（uk_provider_applications_user），
// 驳回后重新提交是更新同一条记录回 pending，而不是插新行：
// 这样「当前认证状态」永远是唯一可查的，不需要在列表里挑最新一条。
//
// 认证通过才把 users.role 置为 provider；role 是权限的唯一依据，
// 申请表只负责「申请过程与审核留痕」。
type ProviderApplication struct {
	BaseModel
	UserID uint64 `gorm:"not null;uniqueIndex:uk_provider_applications_user" json:"user_id"`

	RealName    string `gorm:"size:32;not null" json:"real_name"`
	Phone       string `gorm:"size:20;not null" json:"phone"`
	ServiceType string `gorm:"size:16;not null" json:"service_type"` // garbage/dogwalk/accompany/all
	Experience  string `gorm:"size:500;not null;default:''" json:"experience"`
	IDCardNo    string `gorm:"size:32;not null;default:''" json:"id_card_no"`

	// 证件照与健康证等附件，存 storage 的 object key（逗号分隔）。
	// 不直接存 URL：访问要走鉴权接口，避免链接被外传。
	IDCardFrontKey  string `gorm:"size:255;not null;default:''" json:"-"`
	IDCardBackKey   string `gorm:"size:255;not null;default:''" json:"-"`
	HealthCertKey   string `gorm:"size:255;not null;default:''" json:"-"`

	Status     ProviderApplicationStatus `gorm:"size:16;not null;default:'pending';index:idx_provider_applications_status" json:"status"`
	ReviewerID uint64                    `gorm:"not null;default:0" json:"reviewer_id"`
	ReviewNote string                    `gorm:"size:200;not null;default:''" json:"review_note"`
	ReviewedAt *time.Time                `json:"reviewed_at"`
}

// TableName 固定表名。
func (ProviderApplication) TableName() string { return "provider_applications" }
