package model

// User 用户表。role 区分住户、服务方与后台，V1 阶段服务方与运营可由同一人承担。
type User struct {
	BaseModel
	OpenID      string `gorm:"column:openid;size:64;uniqueIndex:uk_users_openid;not null" json:"openid"`
	Nickname    string `gorm:"size:64;not null;default:''" json:"nickname"`
	Avatar      string `gorm:"size:512;not null;default:''" json:"avatar"`
	Phone       string `gorm:"size:20;not null;default:''" json:"phone"`
	DefaultArea string `gorm:"size:64;not null;default:''" json:"default_area"`
	Role        string `gorm:"size:16;not null;default:'user'" json:"role"`
}

// TableName 固定表名，避免 GORM 复数规则变化导致对不上迁移。
func (User) TableName() string { return "users" }

// Address 用户地址。in_service_area 在保存时即完成校验，下单时不再重复判定。
type Address struct {
	BaseModel
	UserID        uint64  `gorm:"not null;index:idx_addresses_user_id" json:"user_id"`
	Community     string  `gorm:"size:64;not null" json:"community"`
	Lane          string  `gorm:"size:64;not null" json:"lane"`
	HouseNo       string  `gorm:"size:32;not null" json:"house_no"`
	Extra         string  `gorm:"size:255;not null;default:''" json:"extra"`
	ContactName   string  `gorm:"size:32;not null;default:''" json:"contact_name"`
	ContactPhone  string  `gorm:"size:20;not null;default:''" json:"contact_phone"`
	InServiceArea bool    `gorm:"not null;default:false" json:"in_service_area"`
	AreaID        *uint64 `gorm:"index:idx_addresses_area_id" json:"area_id"`
}

func (Address) TableName() string { return "addresses" }

// ServiceArea 服务范围规则。house_from 为 0 表示不限起号，house_to 为 0 表示不限止号。
type ServiceArea struct {
	BaseModel
	Community   string `gorm:"size:64;not null;index:idx_service_areas_community" json:"community"`
	Lane        string `gorm:"size:128;not null" json:"lane"`
	HouseFrom   int    `gorm:"not null;default:0" json:"house_from"`
	HouseTo     int    `gorm:"not null;default:0" json:"house_to"`
	HouseParity string `gorm:"size:8;not null;default:'all'" json:"house_parity"`
	Status      int8   `gorm:"not null;default:1;index:idx_service_areas_status" json:"status"`
	EffectiveAt *Date  `gorm:"type:datetime(3)" json:"effective_at"`
}

func (ServiceArea) TableName() string { return "service_areas" }

// ServiceSku 服务产品。rules 保存计价参数，改价与改加价日期不需要发版。
type ServiceSku struct {
	BaseModel
	ServiceType ServiceType `gorm:"size:16;not null;uniqueIndex:uk_service_skus_type_plan" json:"service_type"`
	Plan        PlanType    `gorm:"size:16;not null;uniqueIndex:uk_service_skus_type_plan" json:"plan"`
	BasePrice   int64       `gorm:"not null;default:0" json:"base_price"`
	Rules       JSON        `gorm:"type:json" json:"rules"`
	Slots       JSON        `gorm:"type:json" json:"slots"`
	Status      int8        `gorm:"not null;default:1" json:"status"`
}

func (ServiceSku) TableName() string { return "service_skus" }

// SlotInventory 时段库存。唯一索引 (service_date, slot_code, service_type) 是防重复建的兜底。
// 并发扣减走 Redis，这里只做最终账。
type SlotInventory struct {
	BaseModel
	ServiceDate Date        `gorm:"type:date;not null;uniqueIndex:uk_slot" json:"service_date"`
	SlotCode    string      `gorm:"size:16;not null;uniqueIndex:uk_slot" json:"slot_code"`
	ServiceType ServiceType `gorm:"size:16;not null;uniqueIndex:uk_slot" json:"service_type"`
	Total       int         `gorm:"not null;default:0" json:"total"`
	Used        int         `gorm:"not null;default:0" json:"used"`
}

func (SlotInventory) TableName() string { return "slot_inventories" }

// Available 判断该时段是否还能接单。
func (s SlotInventory) Available() bool { return s.Used < s.Total }
