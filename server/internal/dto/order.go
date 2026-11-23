package dto

import "time"

// CreateOrderReq 创建订单。
type CreateOrderReq struct {
	AddressID   uint64  `json:"address_id" binding:"required"`
	ServiceType string  `json:"service_type" binding:"required,oneof=garbage dogwalk accompany"`
	Plan        string  `json:"plan" binding:"omitempty,oneof=single monthly"`
	ServiceDate string  `json:"service_date" binding:"required"` // 2006-01-02
	SlotCode    string  `json:"slot_code" binding:"required,max=16"`
	Quantity    int     `json:"quantity" binding:"omitempty,min=0,max=50"`
	PetCount    int     `json:"pet_count" binding:"omitempty,min=0,max=10"`
	DurationMin int     `json:"duration_min" binding:"omitempty,min=0,max=120"`
	DistanceKm  float64 `json:"distance_km" binding:"omitempty,min=0,max=50"`
	GarbageType string  `json:"garbage_type" binding:"omitempty,oneof=kitchen other recyclable"`
	Remark      string  `json:"remark" binding:"max=200"`
	FeedOnly    bool    `json:"feed_only"`
	// ExtraSlotCode 包月备选服务时间：选中后取代默认时段，每天按它上门（空表示按默认时段）
	ExtraSlotCode string `json:"extra_slot_code" binding:"omitempty,max=16"`
	// PetNotes 宠物备注（选填）：类型、名字、情况等
	PetNotes     string `json:"pet_notes" binding:"omitempty,max=200"`
	NeedFamiliar bool   `json:"need_familiar"`

	// EntryMethod / AccessInfo 进入方式与门禁信息（选填）。
	// AccessInfo 是明文入参，服务端加密后落库，任何接口都不会明文回传原始密文。
	EntryMethod string `json:"entry_method" binding:"omitempty,oneof=knock password card property other"`
	AccessInfo  string `json:"access_info" binding:"omitempty,max=100"`

	// 陪诊专用：医院与服务项只回传 code，价格由后端按 SKU 规则计算
	Hospital     string   `json:"hospital" binding:"omitempty,max=64"`
	PatientName  string   `json:"patient_name" binding:"omitempty,max=32"`
	ServiceItems []string `json:"service_items" binding:"omitempty,max=20,dive,max=32"`
}

// PreviewOrderReq 费用试算，字段与下单一致，但不产生订单。
type PreviewOrderReq = CreateOrderReq

// PriceItem 费用明细项。
type PriceItem struct {
	Name   string `json:"name"`
	Amount int64  `json:"amount"`
	Note   string `json:"note,omitempty"`
}

// PriceFamiliar 熟悉费累计进度，前端用来展示「还差几单」。
type PriceFamiliar struct {
	Done int  `json:"done"` // 已完成单次订单数
	Need int  `json:"need"` // 免熟悉费所需单数（来自后台规则，前端不要写死）
	Free bool `json:"free"` // 本次是否已免
}

// PreviewOrderResp 试算结果。
type PreviewOrderResp struct {
	Total       int64       `json:"total"`
	Items       []PriceItem `json:"items"`
	ManualQuote bool        `json:"manual_quote"`
	// Familiar 仅遛宠开启「提前上门熟悉」时返回
	Familiar *PriceFamiliar `json:"familiar_progress,omitempty"`
}

// OrderResp 订单响应。
type OrderResp struct {
	OrderNo      string       `json:"order_no"`
	ServiceType  string       `json:"service_type"`
	Plan         string       `json:"plan"`
	ServiceDate  string       `json:"service_date"`
	SlotCode     string       `json:"slot_code"`
	Status       string       `json:"status"`
	StatusLabel  string       `json:"status_label"`
	Tab          string       `json:"tab"`
	Nodes        []NodeResp   `json:"nodes"`
	TotalAmount  int64        `json:"total_amount"`
	PaidAmount   int64        `json:"paid_amount"`
	Remark       string       `json:"remark"`
	CreatedAt    time.Time    `json:"created_at"`
	Address      *AddressResp `json:"address,omitempty"`
	PriceItems   []PriceItem  `json:"price_items,omitempty"`
	PayExpiresAt *time.Time   `json:"pay_expires_at,omitempty"`
	Quote        *QuoteResp   `json:"quote,omitempty"`

	// 陪诊订单已选内容，供详情页与后台还原
	Hospital       string      `json:"hospital,omitempty"`
	PatientName    string      `json:"patient_name,omitempty"`
	AccompanyItems []PriceItem `json:"accompany_items,omitempty"`

	// 以下为后台扩展字段，用户端不填充（omitempty 不下发）
	UserNickname     string  `json:"user_nickname,omitempty"`
	UserID           uint64  `json:"user_id,omitempty"`
	ProviderID       *uint64 `json:"provider_id,omitempty"`
	ProviderNickname string  `json:"provider_nickname,omitempty"`
	// ProviderPhone 服务方联系电话。仅「已接单」及之后的订单对住户下发，
	// 未接单时为空，前端据此提示「服务方接单后可显示联系方式」。
	ProviderPhone  string     `json:"provider_phone,omitempty"`
	Quantity       int        `json:"quantity,omitempty"`
	PetCount       int        `json:"pet_count,omitempty"`
	RefundedAmount int64      `json:"refunded_amount,omitempty"`
	CancelReason   string     `json:"cancel_reason,omitempty"`
	PaidAt         *time.Time `json:"paid_at,omitempty"`
	FinishedAt     *time.Time `json:"finished_at,omitempty"`
	// ExtraSlotCode 包月备选服务时间（选中即取代默认时段，展示时优先于 slot_code）
	ExtraSlotCode string `json:"extra_slot_code,omitempty"`
	// FreeQuotaUsed 表示本单用注册赠送的免费次数抵扣（金额为 0）
	FreeQuotaUsed bool `json:"free_quota_used,omitempty"`
	// PetNotes 宠物备注，服务方接单后核对交接要求用
	PetNotes string `json:"pet_notes,omitempty"`
	// EntryMethod 进入方式，接管订单后即可下发（不敏感）
	EntryMethod string `json:"entry_method,omitempty"`
	// AccessInfo 门禁密码/钥匙说明，只在服务开始前 2 小时至服务结束后下发
	AccessInfo string `json:"access_info,omitempty"`
	// AccessVisible 钥匙信息当前是否可见。nil 表示这单没填；
	// false 表示填了但还没到可见时间，前端据此提示而不是显示成空白。
	AccessVisible *bool  `json:"access_visible,omitempty"`
	MonthlyStart  string `json:"monthly_start,omitempty"`
	MonthlyEnd    string `json:"monthly_end,omitempty"`

	// PendingReschedule 服务方发起、还等住户确认的改期申请。
	// 详情与列表都会填充：列表页靠它提示「有单子等你确认」，否则用户只能逐单点进去才发现。
	PendingReschedule *RescheduleInfo `json:"pending_reschedule,omitempty"`
	// Reschedules 改期历史，最新的在前（仅详情接口填充）
	Reschedules []RescheduleInfo `json:"reschedules,omitempty"`
	// RescheduleNotice 住户改了时间、服务方还没确认知悉（仅服务方视角填充）。
	// 时间变更会直接改掉卡片上的日期，没有这条提示服务方要逐单点进详情才发现。
	RescheduleNotice *RescheduleInfo `json:"reschedule_notice,omitempty"`

	// Monthly 月卡（包月代倒垃圾）的今日进度，仅 plan=monthly 的订单填充。
	// 一张月卡订单跑满 30 天，状态字段永远停在「已受理」，
	// 单看 status 无法回答「今天到底上没上门」，所以单独下发这一块。
	Monthly *MonthlyProgress `json:"monthly,omitempty"`
}

// MonthlyProgress 月卡今日进度。
type MonthlyProgress struct {
	// Today 服务端认定的今天（按服务器时区），前端不要用本机时间自己算，
	// 否则跨时区/跨零点时「今天」会和打卡记录对不上。
	Today     string `json:"today"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	// Used / Total 已核销天数 / 服务期总天数
	Used  int `json:"used"`
	Total int `json:"total"`
	// TodayDone 今天是否已打卡；TodayAt 是打卡时间（未打卡为空）
	TodayDone bool       `json:"today_done"`
	TodayAt   *time.Time `json:"today_at,omitempty"`
	// TodayFinished 今天这趟是否已点过「完成服务」。
	// 打卡只证明人到了，袋子提走没有要看这个：两者都真才算今天这一趟跑完。
	TodayFinished   bool       `json:"today_finished"`
	TodayFinishedAt *time.Time `json:"today_finished_at,omitempty"`
	// InPeriod 今天是否落在服务期内（含首尾），false 时不允许打卡
	InPeriod bool `json:"in_period"`
	// Ended 服务期已结束：此时才允许把整张月卡提交「完成服务」结算
	Ended bool `json:"ended"`
}

// RescheduleReq 改期请求。住户直接改期与服务方发起申请共用同一份入参。
type RescheduleReq struct {
	ServiceDate string `json:"service_date" binding:"required"` // 2006-01-02
	SlotCode    string `json:"slot_code" binding:"required,max=16"`
	// Reason 改期原因，服务方申请时强烈建议填写，住户据此判断是否同意
	Reason string `json:"reason" binding:"max=200"`
}

// RespondRescheduleReq 住户处理服务方的改期申请。
type RespondRescheduleReq struct {
	Approve bool `json:"approve"`
	// Reason 拒绝原因，会覆盖到改期记录里给服务方看
	Reason string `json:"reason" binding:"max=200"`
}

// RescheduleInfo 一条改期记录。
// From 是发起时的原时间，To 是目标时间；订单主表只保留最新值，
// 历史靠这张快照还原。
type RescheduleInfo struct {
	ID          uint64     `json:"id"`
	FromDate    string     `json:"from_date"`
	FromSlot    string     `json:"from_slot"`
	ToDate      string     `json:"to_date"`
	ToSlot      string     `json:"to_slot"`
	Reason      string     `json:"reason,omitempty"`
	Status      string     `json:"status"`
	InitiatedBy string     `json:"initiated_by"`
	CreatedAt   time.Time  `json:"created_at"`
	HandledAt   *time.Time `json:"handled_at,omitempty"`
	// ProviderAck 服务方确认知悉这次改期的时间（仅住户发起的改期有意义）
	ProviderAck *time.Time `json:"provider_ack_at,omitempty"`
}

// QuoteResp 陪诊报价。Confirmed 为真表示用户已确认并去支付。
type QuoteResp struct {
	Amount      int64     `json:"amount"`
	DurationMin int       `json:"duration_min"`
	Confirmed   bool      `json:"confirmed"`
	CreatedAt   time.Time `json:"created_at"`
}

// NodeResp 进度节点。
type NodeResp struct {
	Node    string     `json:"node"`
	Label   string     `json:"label"`
	Reached bool       `json:"reached"`
	At      *time.Time `json:"at,omitempty"`
	Remark  string     `json:"remark,omitempty"`
}

// CreateOrderResp 下单结果。ManualQuote 为真时不返回支付参数，等待运营报价。
type CreateOrderResp struct {
	Order       OrderResp `json:"order"`
	Pay         any       `json:"pay,omitempty"`
	ManualQuote bool      `json:"manual_quote"`
	PayHint     string    `json:"pay_hint,omitempty"`
}

// GarbagePlanResp 代倒垃圾包月方案（公开接口，前端展示用）。
// 价格与时段都从 SKU 规则读，后台改配置后小程序不用发版。
type GarbagePlanResp struct {
	Price int64 `json:"price" example:"3000"`
	// SinglePrice 单次价格；有赠送额度时前端显示为抵扣
	SinglePrice int64    `json:"single_price" example:"500"`
	Days        int      `json:"days" example:"30"`
	DefaultSlot string   `json:"default_slot" example:"22:00-22:30"`
	ExtraSlots  []string `json:"extra_slots"`
	FreePerUser int      `json:"free_per_user" example:"2"`
}

// ProviderIncomeResp 服务方收入汇总。
type ProviderIncomeResp struct {
	Total      int64        `json:"total"`
	OrderCount int64        `json:"order_count"`
	Items      []IncomeItem `json:"items"`
}

// IncomeItem 一笔入账。
type IncomeItem struct {
	OrderNo   string    `json:"order_no"`
	Amount    int64     `json:"amount"`
	CreatedAt time.Time `json:"created_at"`
}
