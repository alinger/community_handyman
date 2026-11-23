// Package model 定义 GORM 模型与枚举常量。
// 字段与 migrations 中的建表语句严格对应，改动任一侧都必须同步另一侧。
package model

import (
	"database/sql/driver"
	"fmt"
	"strings"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// JSON 是 JSON 列的别名，指向 GORM 官方 datatypes 实现，避免自己处理扫描与序列化。
type JSON = datatypes.JSON

// BaseModel 是所有业务表的公共字段。
// 主键用 uint64；软删除保证服务记录等凭证不可物理删除。
type BaseModel struct {
	ID        uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt time.Time      `gorm:"index" json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// ServiceType 服务类型。
type ServiceType string

const (
	ServiceGarbage   ServiceType = "garbage"   // 代倒垃圾
	ServiceDogWalk   ServiceType = "dogwalk"   // 遛宠
	ServiceAccompany ServiceType = "accompany" // 陪诊
)

// PlanType 购买方案。
type PlanType string

const (
	PlanSingle  PlanType = "single"
	PlanMonthly PlanType = "monthly"
)

// OrderStatus 订单内部状态，与需求文档表 2 一致。
type OrderStatus string

const (
	OrderPendingPayment OrderStatus = "pending_payment" // 待支付
	OrderPendingAccept  OrderStatus = "pending_accept"  // 待接单
	OrderAccepted       OrderStatus = "accepted"        // 已受理
	OrderOnTheWay       OrderStatus = "on_the_way"      // 待上门（遛宠为待接宠）
	OrderInService      OrderStatus = "in_service"      // 服务中
	OrderPendingConfirm OrderStatus = "pending_confirm" // 待完成（遛宠为待送回）
	OrderCompleted      OrderStatus = "completed"       // 已完成
	OrderCanceled       OrderStatus = "canceled"        // 已取消
	OrderRejected       OrderStatus = "rejected"        // 已拒单
)

// Terminal 判断是否为终态。
func (s OrderStatus) Terminal() bool {
	return s == OrderCompleted || s == OrderCanceled || s == OrderRejected
}

// Valid 校验状态值合法性，防止写库时出现拼写错误的字符串。
func (s OrderStatus) Valid() bool {
	switch s {
	case OrderPendingPayment, OrderPendingAccept, OrderAccepted, OrderOnTheWay,
		OrderInService, OrderPendingConfirm, OrderCompleted, OrderCanceled, OrderRejected:
		return true
	}
	return false
}

// Tab 返回订单在客户端所属的 Tab，对应需求文档四（五）的映射。
func (s OrderStatus) Tab() string {
	switch s {
	case OrderPendingPayment, OrderPendingAccept, OrderAccepted, OrderOnTheWay:
		return "pending"
	case OrderInService, OrderPendingConfirm:
		return "processing"
	case OrderCompleted:
		return "completed"
	default:
		return "closed"
	}
}

// allowedTransitions 定义合法的状态流转，非法流转一律拒绝，避免状态被随意改写。
var allowedTransitions = map[OrderStatus][]OrderStatus{
	OrderPendingPayment: {OrderPendingAccept, OrderCanceled},
	OrderPendingAccept:  {OrderAccepted, OrderRejected, OrderCanceled},
	OrderAccepted:       {OrderOnTheWay, OrderInService, OrderCanceled},
	OrderOnTheWay:       {OrderInService, OrderCanceled},
	// 服务中可以直接完成：垃圾代倒一次上门即结束，不需要住户再点一次确认。
	// 这类「跳过确认」只对垃圾放开，是否允许由 ProviderService 按服务类型判定，
	// 状态机本身只表达「结构上可以这么流转」。
	OrderInService:      {OrderPendingConfirm, OrderCompleted, OrderCanceled},
	OrderPendingConfirm: {OrderCompleted, OrderInService},
	OrderCompleted:      {},
	OrderCanceled:       {},
	OrderRejected:       {},
}

// CanTransit 判断状态是否可以从 from 流转到 to。
func CanTransit(from, to OrderStatus) bool {
	for _, s := range allowedTransitions[from] {
		if s == to {
			return true
		}
	}
	return false
}

// Label 返回状态的中文文案。
func (s OrderStatus) Label() string {
	switch s {
	case OrderPendingPayment:
		return "待支付"
	case OrderPendingAccept:
		return "待接单"
	case OrderAccepted:
		return "已受理"
	case OrderOnTheWay:
		return "待上门"
	case OrderInService:
		return "服务中"
	case OrderPendingConfirm:
		return "待完成"
	case OrderCompleted:
		return "已完成"
	case OrderCanceled:
		return "已取消"
	case OrderRejected:
		return "已拒单"
	default:
		return string(s)
	}
}

// Date 是只保留年月日的日期类型，对应 MySQL DATE。
// 直接用 time.Time 会带上时分秒，按时段查询时容易因精度差异漏数据。
type Date time.Time

const dateLayout = "2006-01-02"

// NewDate 由时间构造日期。
func NewDate(t time.Time) Date {
	y, m, d := t.Date()
	return Date(time.Date(y, m, d, 0, 0, 0, 0, t.Location()))
}

// ParseDate 解析 2006-01-02 格式。
func ParseDate(s string) (Date, error) {
	t, err := time.ParseInLocation(dateLayout, strings.TrimSpace(s), time.Local)
	if err != nil {
		return Date{}, fmt.Errorf("日期格式应为 2006-01-02: %w", err)
	}
	return Date(t), nil
}

// Time 转回 time.Time。
func (d Date) Time() time.Time { return time.Time(d) }

func (d Date) String() string { return time.Time(d).Format(dateLayout) }

// MarshalJSON 输出 2006-01-02。
func (d Date) MarshalJSON() ([]byte, error) {
	return []byte(`"` + d.String() + `"`), nil
}

// UnmarshalJSON 接受 2006-01-02 字符串。
func (d *Date) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		return nil
	}
	parsed, err := ParseDate(s)
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

// Value 实现 driver.Valuer。
func (d Date) Value() (driver.Value, error) { return d.String(), nil }

// Scan 实现 sql.Scanner，兼容 []byte 与 time.Time 两种返回。
func (d *Date) Scan(v any) error {
	switch val := v.(type) {
	case nil:
		return nil
	case time.Time:
		*d = NewDate(val)
		return nil
	case []byte:
		parsed, err := ParseDate(string(val))
		if err != nil {
			return err
		}
		*d = parsed
		return nil
	case string:
		parsed, err := ParseDate(val)
		if err != nil {
			return err
		}
		*d = parsed
		return nil
	}
	return fmt.Errorf("Date 无法解析类型 %T", v)
}
