package dto

import "time"

// ---------- 后台运营扩展：财务 / 用户 / 统计 ----------

// SetUserRoleReq 切换用户角色。
type SetUserRoleReq struct {
	Role string `json:"role" binding:"required,oneof=user provider" example:"provider"`
}

// SetUserStatusReq 封禁/解封用户。
type SetUserStatusReq struct {
	Status int8 `json:"status" binding:"oneof=0 1" example:"0"`
}

// UpdateAreaReq 修改服务范围规则。
type UpdateAreaReq struct {
	Community   string `json:"community" binding:"required,max=64" example:"东方冠郡二期"`
	Lane        string `json:"lane" binding:"required,max=128" example:"众安路779弄"`
	HouseFrom   int    `json:"house_from" binding:"min=0" example:"1"`
	HouseTo     int    `json:"house_to" binding:"min=0" example:"89"`
	HouseParity string `json:"house_parity" binding:"omitempty,oneof=all odd even" example:"all"`
}

// CopySlotsReq 时段模板复制：把某天已配置的时段库存复制到一批目标日期。
type CopySlotsReq struct {
	SourceDate  string   `json:"source_date" binding:"required" example:"2026-11-20"`
	TargetDates []string `json:"target_dates" binding:"required,min=1,max=62,dive,required" example:"2026-11-21"`
}

// CreateAdminReq 新建后台账号（仅超管）。
type CreateAdminReq struct {
	Username string `json:"username" binding:"required,max=32" example:"operator01"`
	Password string `json:"password" binding:"required,min=6,max=64" example:"your-password"`
	Role     string `json:"role" binding:"required,oneof=admin operator" example:"operator"`
}

// SetAdminStatusReq 启停后台账号（仅超管）。
type SetAdminStatusReq struct {
	Status int8 `json:"status" binding:"oneof=0 1" example:"0"`
}

// ResetAdminPwdReq 超管重置账号密码（仅超管）。
type ResetAdminPwdReq struct {
	Password string `json:"password" binding:"required,min=6,max=64" example:"new-password"`
}

// ChangeMyPwdReq 后台账号自助修改密码（需验证旧密码）。
type ChangeMyPwdReq struct {
	OldPassword string `json:"old_password" binding:"required,max=64" example:"old-password"`
	NewPassword string `json:"new_password" binding:"required,min=6,max=64" example:"new-password"`
}

// AdminCancelReq 运营代客取消。
type AdminCancelReq struct {
	Reason string `json:"reason" binding:"required,max=200" example:"用户来电要求取消"`
}

// AdminRescheduleReq 运营代客改期。
type AdminRescheduleReq struct {
	ServiceDate string `json:"service_date" binding:"required" example:"2026-11-21"`
	SlotCode    string `json:"slot_code" binding:"required,max=16" example:"10-12"`
}

// AdminPayment 支付流水行。
type AdminPayment struct {
	ID            uint64     `json:"id"`
	OrderNo       string     `json:"order_no"`
	TransactionID string     `json:"transaction_id"`
	OutTradeNo    string     `json:"out_trade_no"`
	Amount        int64      `json:"amount"`
	Status        string     `json:"status"`
	PaidAt        *time.Time `json:"paid_at"`
	CreatedAt     time.Time  `json:"created_at"`
}

// AdminRefund 退款单行。
type AdminRefund struct {
	ID          uint64     `json:"id"`
	OrderNo     string     `json:"order_no"`
	OutRefundNo string     `json:"out_refund_no"`
	RefundID    string     `json:"refund_id"`
	Amount      int64      `json:"amount"`
	Reason      string     `json:"reason"`
	Status      string     `json:"status"`
	OperatorID  uint64     `json:"operator_id"`
	RefundedAt  *time.Time `json:"refunded_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

// AdminEvent 订单时间线节点。
type AdminEvent struct {
	ID         uint64    `json:"id"`
	Node       string    `json:"node"`
	Operator   string    `json:"operator"`
	OperatorID *uint64   `json:"operator_id"`
	Remark     string    `json:"remark"`
	CreatedAt  time.Time `json:"created_at"`
}

// AdminUserRow 用户/服务方列表行。
type AdminUserRow struct {
	ID         uint64    `json:"id"`
	Nickname   string    `json:"nickname"`
	OpenID     string    `json:"openid"`
	Phone      string    `json:"phone"`
	Role       string    `json:"role"`
	Status     int8      `json:"status"`
	OrderCount int64     `json:"order_count"`
	CreatedAt  time.Time `json:"created_at"`
}

// AdminRedemptionRow 月卡核销行。
type AdminRedemptionRow struct {
	ID          uint64    `json:"id"`
	OrderNo     string    `json:"order_no"`
	ServiceDate string    `json:"service_date"`
	OperatorID  uint64    `json:"operator_id"`
	Remark      string    `json:"remark"`
	CreatedAt   time.Time `json:"created_at"`
}

// AdminStats 看板统计。
type AdminStats struct {
	TodayOrders   int64               `json:"today_orders"`
	TodayGMV      int64               `json:"today_gmv"`
	TodayRefunded int64               `json:"today_refunded"`
	PendingAccept int64               `json:"pending_accept"`
	Unassigned    int64               `json:"unassigned"`
	InService     int64               `json:"in_service"`
	TotalUsers    int64               `json:"total_users"`
	Providers     int64               `json:"providers"`
	StatusDist    []AdminStatusCount  `json:"status_dist"`
	ServiceDist   []AdminServiceCount `json:"service_dist"`
}

// AdminStatusCount 各状态订单数。
type AdminStatusCount struct {
	Status string `json:"status"`
	Count  int64  `json:"count"`
}

// AdminServiceCount 各服务类型订单数（今日）。
type AdminServiceCount struct {
	ServiceType string `json:"service_type"`
	Count       int64  `json:"count"`
}

// AdminAccountRow 后台账号行。
type AdminAccountRow struct {
	ID        uint64    `json:"id"`
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	Status    int8      `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}
