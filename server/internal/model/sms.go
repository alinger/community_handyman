package model

import "time"

// 短信验证码用途。不同用途各自独立校验，登录码不能拿去重置密码。
const (
	SMSSceneLogin = "login" // 登录/注册
	SMSSceneReset = "reset" // 重置密码
)

// 短信下发状态。
const (
	SMSStatusPending int8 = 0 // 已落库，尚未确认下发结果
	SMSStatusSent    int8 = 1 // 渠道返回成功
	SMSStatusFailed  int8 = 2 // 渠道返回失败，不计入频率限制
)

// SMSCode 一条短信验证码记录。
// 保留历史记录用于风控与对账，因此不物理删除。
type SMSCode struct {
	BaseModel
	Phone     string    `gorm:"size:20;not null;index:idx_sms_phone_scene,priority:1" json:"phone"`
	Scene     string    `gorm:"size:16;not null;default:login;index:idx_sms_phone_scene,priority:2" json:"scene"`
	Code      string    `gorm:"size:6;not null" json:"-"`
	Status    int8      `gorm:"not null;default:0" json:"status"`
	Attempts  int8      `gorm:"not null;default:0" json:"attempts"`
	Used      int8      `gorm:"not null;default:0" json:"used"`
	ExpiresAt time.Time `gorm:"not null" json:"expires_at"`
	IP        string    `gorm:"size:64;not null;default:''" json:"ip"`
}

// TableName 固定表名，避免 GORM 复数化推断出错。
func (SMSCode) TableName() string { return "sms_codes" }
