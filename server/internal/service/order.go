package service

import (
	"context"
	"encoding/json"
	"time"

	"gorm.io/gorm"

	"github.com/alinger/community-handyman/server/internal/dto"
	"github.com/alinger/community-handyman/server/internal/errs"
	"github.com/alinger/community-handyman/server/internal/model"
	"github.com/alinger/community-handyman/server/internal/payment"
	"github.com/alinger/community-handyman/server/internal/pricing"
	"github.com/alinger/community-handyman/server/internal/repository"
	"github.com/alinger/community-handyman/server/pkg/util"
)

// PayTimeout 未支付订单的存活时间，超时自动关单释放库存。
const PayTimeout = 15 * time.Minute

// OrderService 订单编排。
// 事务边界在这里：库存预占与订单写入必须同生共死。
type OrderService struct {
	db        *gorm.DB
	orders    *repository.OrderRepo
	slots     *repository.SlotRepo
	skus      *repository.SkuRepo
	addresses *repository.AddressRepo
	areas     *repository.AreaRepo
	quotes    *repository.QuoteRepo
	pay       payment.Gateway
}

func NewOrderService(db *gorm.DB, orders *repository.OrderRepo, slots *repository.SlotRepo,
	skus *repository.SkuRepo, addresses *repository.AddressRepo, areas *repository.AreaRepo,
	quotes *repository.QuoteRepo, pay payment.Gateway) *OrderService {
	return &OrderService{db: db, orders: orders, slots: slots, skus: skus,
		addresses: addresses, areas: areas, quotes: quotes, pay: pay}
}

// Preview 费用试算，不落库、不占库存。前端在提交前展示明细用。
func (s *OrderService) Preview(ctx context.Context, userID uint64, req dto.PreviewOrderReq) (*dto.PreviewOrderResp, error) {
	addr, sku, date, err := s.validate(ctx, userID, req)
	if err != nil {
		return nil, err
	}

	res, err := s.calc(ctx, userID, addr, sku, date, req)
	if err != nil {
		return nil, err
	}
	return &dto.PreviewOrderResp{Total: res.Total, Items: toPriceItems(res.Items), ManualQuote: res.ManualQuote}, nil
}

// Create 创建订单并预占库存，随后发起支付。
// 返回的 pay 为 nil 表示无需立即支付（陪诊需人工报价）。
func (s *OrderService) Create(ctx context.Context, userID uint64, req dto.CreateOrderReq) (*model.Order, *payment.PrepayResult, error) {
	addr, sku, date, err := s.validate(ctx, userID, req)
	if err != nil {
		return nil, nil, err
	}

	res, err := s.calc(ctx, userID, addr, sku, date, req)
	if err != nil {
		return nil, nil, err
	}

	// 时段库存校验：total 为 0 表示该时段不开放（如工作日遛宠只开晚间）
	slot, err := s.slots.Get(ctx, date, req.SlotCode, model.ServiceType(req.ServiceType))
	if err != nil {
		return nil, nil, errs.Wrap(errs.CodeInternal, err)
	}
	if slot == nil || !slot.Available() {
		return nil, nil, errs.Biz(errs.CodeSlotFull)
	}

	addrSnapshot, _ := json.Marshal(ToAddressResp(addr))
	pricingSnapshot, _ := json.Marshal(dto.PreviewOrderResp{Total: res.Total, Items: toPriceItems(res.Items), ManualQuote: res.ManualQuote})

	order := &model.Order{
		OrderNo:         generateOrderNo(),
		UserID:          userID,
		ServiceType:     model.ServiceType(req.ServiceType),
		Plan:            model.PlanType(defaultPlan(req.Plan)),
		AddressID:       addr.ID,
		AddressSnapshot: addrSnapshot,
		ServiceDate:     date,
		SlotCode:        req.SlotCode,
		Quantity:        defaultInt(req.Quantity, 1),
		PetCount:        req.PetCount,
		DurationMin:     req.DurationMin,
		DistanceKm:      nilIfZero(req.DistanceKm),
		GarbageType:     req.GarbageType,
		Remark:          req.Remark,
		PricingSnapshot: pricingSnapshot,
		TotalAmount:     res.Total,
		Status:          model.OrderPendingPayment,
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ok, err := s.slots.WithTx(tx).Reserve(ctx, date, req.SlotCode, order.ServiceType)
		if err != nil {
			return errs.Wrap(errs.CodeInternal, err)
		}
		if !ok {
			return errs.Biz(errs.CodeSlotFull)
		}
		if err := s.orders.WithTx(tx).Create(ctx, order); err != nil {
			return errs.Wrap(errs.CodeInternal, err)
		}
		return s.orders.WithTx(tx).AddEvent(ctx, &model.OrderEvent{
			OrderID:  order.ID,
			Node:     string(model.OrderPendingPayment),
			Operator: "user",
			Remark:   "用户提交预约",
		})
	})
	if err != nil {
		return nil, nil, err
	}

	if res.ManualQuote {
		return order, nil, nil
	}

	pr, err := s.pay.Prepay(ctx, payment.PrepayRequest{
		OrderNo:     order.OrderNo,
		Amount:      order.TotalAmount,
		Description: serviceName(order.ServiceType),
	})
	if err != nil {
		// 订单已落库并占库存，支付失败允许用户重新发起，不整单回滚
		return order, nil, errs.Wrap(errs.CodePayFailed, err)
	}
	return order, &pr, nil
}

// Repay 重新发起支付，用于支付失败或用户主动重试。
func (s *OrderService) Repay(ctx context.Context, userID uint64, orderNo string) (*payment.PrepayResult, error) {
	o, err := s.orders.GetByNo(ctx, orderNo)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	if o == nil || o.UserID != userID {
		return nil, errs.Biz(errs.CodeNotFound)
	}
	if o.Status != model.OrderPendingPayment {
		return nil, errs.BizMsg(errs.CodeIllegalStatus, "该订单当前不可支付")
	}
	if time.Since(o.CreatedAt) > PayTimeout {
		return nil, errs.Biz(errs.CodePayTimeout)
	}
	if o.TotalAmount <= 0 {
		return nil, errs.BizMsg(errs.CodeIllegalStatus, "该订单需等待人工报价")
	}

	pr, err := s.pay.Prepay(ctx, payment.PrepayRequest{
		OrderNo:     o.OrderNo,
		Amount:      o.TotalAmount,
		Description: serviceName(o.ServiceType),
	})
	if err != nil {
		return nil, errs.Wrap(errs.CodePayFailed, err)
	}

	// 用户主动发起支付即视为确认报价，此处落 confirmed_at，
	// 之后运营不能再改价（QuoteService.Create 会拦住）。
	if s.quotes != nil {
		if q, qerr := s.quotes.LatestByOrder(ctx, o.ID); qerr == nil && q != nil && q.ConfirmedAt == nil {
			_, _ = s.quotes.Confirm(ctx, q.ID)
		}
	}
	return &pr, nil
}

// List 按 Tab 查询订单。
func (s *OrderService) List(ctx context.Context, userID uint64, tab string, page, size int) ([]dto.OrderResp, int64, error) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 || size > 50 {
		size = 10
	}
	list, total, err := s.orders.ListByUser(ctx, userID, tab, page, size)
	if err != nil {
		return nil, 0, errs.Wrap(errs.CodeInternal, err)
	}

	out := make([]dto.OrderResp, 0, len(list))
	for i := range list {
		out = append(out, s.ToOrderResp(&list[i], nil))
	}
	return out, total, nil
}

// Detail 订单详情，带节点时间轴。
// 下单用户与该单服务方都可以看：服务方接单后需要地址与备注才能上门。
func (s *OrderService) Detail(ctx context.Context, userID uint64, orderNo string) (*dto.OrderResp, error) {
	o, err := s.orders.GetByNo(ctx, orderNo)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	if o == nil || !canAccessOrder(o, userID) {
		return nil, errs.Biz(errs.CodeNotFound)
	}
	events, err := s.orders.Events(ctx, o.ID)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	resp := s.ToOrderResp(o, events)
	return &resp, nil
}

// CancelResult 取消结果。
// NeedReview 为真表示落在 2 小时窗口内，需运营审核，此时订单状态不变。
type CancelResult struct {
	Canceled   bool   `json:"canceled"`
	NeedReview bool   `json:"need_review"`
	RefundDue  int64  `json:"refund_due"`
	Message    string `json:"message"`
}

// Cancel 用户取消订单。
// 规则来自需求文档 Q-07：服务开始前 2 小时以上免费取消；
// 2 小时内需运营审核；服务开始后不可取消。未支付订单可直接取消。
func (s *OrderService) Cancel(ctx context.Context, userID uint64, orderNo, reason string) (*CancelResult, error) {
	o, err := s.orders.GetByNo(ctx, orderNo)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	if o == nil || o.UserID != userID {
		return nil, errs.Biz(errs.CodeNotFound)
	}
	if o.Status.Terminal() {
		return nil, errs.BizMsg(errs.CodeIllegalStatus, "订单已结束，无法取消")
	}

	// 未支付：直接关闭并释放库存，不涉及退款
	if o.Status == model.OrderPendingPayment {
		if err := s.closeOrder(ctx, o, "user", userID, "用户取消未支付订单"); err != nil {
			return nil, err
		}
		return &CancelResult{Canceled: true, Message: "订单已取消"}, nil
	}

	start, err := util.SlotStart(o.ServiceDate.Time(), o.SlotCode)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	now := time.Now()
	if !now.Before(start) {
		return nil, errs.BizMsg(errs.CodeIllegalStatus, "服务已开始，不能取消")
	}
	if start.Sub(now) < 2*time.Hour {
		return &CancelResult{
			NeedReview: true,
			Message:    "服务开始前 2 小时内取消需运营审核，我们会尽快联系你",
		}, nil
	}

	if err := s.closeOrder(ctx, o, "user", userID, defaultStr(reason, "用户取消订单")); err != nil {
		return nil, err
	}
	return &CancelResult{
		Canceled:  true,
		RefundDue: o.Refundable(),
		Message:   "已取消，退款将原路退回",
	}, nil
}

// closeOrder 统一关闭订单：改状态、释放库存、写节点。
func (s *OrderService) closeOrder(ctx context.Context, o *model.Order, operator string, operatorID uint64, remark string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ok, err := s.orders.WithTx(tx).UpdateStatus(ctx, o.ID, o.Status, model.OrderCanceled,
			map[string]any{"cancel_reason": remark})
		if err != nil {
			return errs.Wrap(errs.CodeInternal, err)
		}
		if !ok {
			return errs.Biz(errs.CodeIllegalStatus)
		}
		if err := s.slots.WithTx(tx).Release(ctx, o.ServiceDate, o.SlotCode, o.ServiceType); err != nil {
			return errs.Wrap(errs.CodeInternal, err)
		}
		return s.orders.WithTx(tx).AddEvent(ctx, &model.OrderEvent{
			OrderID:    o.ID,
			Node:       string(model.OrderCanceled),
			Operator:   operator,
			OperatorID: &operatorID,
			Remark:     remark,
		})
	})
}

// Confirm 用户确认服务完成。完成后订单进入终态，可查看服务记录。
func (s *OrderService) Confirm(ctx context.Context, userID uint64, orderNo string) error {
	o, err := s.orders.GetByNo(ctx, orderNo)
	if err != nil {
		return errs.Wrap(errs.CodeInternal, err)
	}
	if o == nil || o.UserID != userID {
		return errs.Biz(errs.CodeNotFound)
	}
	if o.Status != model.OrderPendingConfirm {
		return errs.BizMsg(errs.CodeIllegalStatus, "只有待完成的订单可以确认")
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ok, err := s.orders.WithTx(tx).UpdateStatus(ctx, o.ID,
			model.OrderPendingConfirm, model.OrderCompleted,
			map[string]any{"finished_at": time.Now()})
		if err != nil {
			return errs.Wrap(errs.CodeInternal, err)
		}
		if !ok {
			return errs.Biz(errs.CodeIllegalStatus)
		}
		return s.orders.WithTx(tx).AddEvent(ctx, &model.OrderEvent{
			OrderID:  o.ID,
			Node:     string(model.OrderCompleted),
			Operator: "user",
			Remark:   "用户确认服务完成",
		})
	})
}

// CloseExpired 关闭超时未支付订单，释放库存。返回关闭数量。
func (s *OrderService) CloseExpired(ctx context.Context, limit int) (int, error) {
	list, err := s.orders.ListExpiredUnpaid(ctx, time.Now().Add(-PayTimeout), limit)
	if err != nil {
		return 0, errs.Wrap(errs.CodeInternal, err)
	}

	closed := 0
	for i := range list {
		o := list[i]
		err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			ok, err := s.orders.WithTx(tx).UpdateStatus(ctx, o.ID, model.OrderPendingPayment, model.OrderCanceled,
				map[string]any{"cancel_reason": "支付超时，系统自动关闭"})
			if err != nil {
				return err
			}
			if !ok {
				return nil // 已被回调或其他流程改状态，跳过
			}
			if err := s.slots.WithTx(tx).Release(ctx, o.ServiceDate, o.SlotCode, o.ServiceType); err != nil {
				return err
			}
			if err := s.orders.WithTx(tx).AddEvent(ctx, &model.OrderEvent{
				OrderID:  o.ID,
				Node:     string(model.OrderCanceled),
				Operator: "system",
				Remark:   "支付超时自动关闭",
			}); err != nil {
				return err
			}
			closed++
			return nil
		})
		if err != nil {
			return closed, errs.Wrap(errs.CodeInternal, err)
		}
	}
	return closed, nil
}

// validate 校验地址归属、服务范围、日期与时段格式。
func (s *OrderService) validate(ctx context.Context, userID uint64, req dto.CreateOrderReq) (*model.Address, *model.ServiceSku, model.Date, error) {
	addr, err := s.addresses.GetByID(ctx, userID, req.AddressID)
	if err != nil {
		return nil, nil, model.Date{}, errs.Wrap(errs.CodeInternal, err)
	}
	if addr == nil {
		return nil, nil, model.Date{}, errs.BizMsg(errs.CodeNotFound, "地址不存在")
	}
	if !addr.InServiceArea {
		return nil, nil, model.Date{}, errs.Biz(errs.CodeAddressOutOfArea)
	}

	plan := model.PlanType(defaultPlan(req.Plan))
	sku, err := s.skus.Get(ctx, model.ServiceType(req.ServiceType), plan)
	if err != nil {
		return nil, nil, model.Date{}, errs.Wrap(errs.CodeInternal, err)
	}
	if sku == nil {
		return nil, nil, model.Date{}, errs.BizMsg(errs.CodeInvalidParam, "该服务方案未配置或已下架")
	}

	date, err := model.ParseDate(req.ServiceDate)
	if err != nil {
		return nil, nil, model.Date{}, errs.BizMsg(errs.CodeInvalidParam, "服务日期格式应为 2006-01-02")
	}
	today := model.NewDate(time.Now())
	if date.Time().Before(today.Time()) {
		return nil, nil, model.Date{}, errs.BizMsg(errs.CodeInvalidParam, "不能预约过去的日期")
	}

	return addr, sku, date, nil
}

// calc 把订单参数与 SKU 规则交给计价引擎。
func (s *OrderService) calc(ctx context.Context, userID uint64, _ *model.Address,
	sku *model.ServiceSku, date model.Date, req dto.CreateOrderReq) (pricing.Result, error) {

	familiarOrders := 0
	if req.NeedFamiliar {
		n, err := s.orders.CountCompleted(ctx, userID, sku.ServiceType)
		if err != nil {
			return pricing.Result{}, errs.Wrap(errs.CodeInternal, err)
		}
		familiarOrders = n
	}

	res, err := pricing.Calc(pricing.Input{
		ServiceType:    pricing.ServiceType(sku.ServiceType),
		Plan:           pricing.PlanType(sku.Plan),
		BasePrice:      sku.BasePrice,
		Quantity:       req.Quantity,
		PetCount:       req.PetCount,
		DurationMin:    req.DurationMin,
		DistanceKm:     req.DistanceKm,
		FeedOnly:       req.FeedOnly,
		NeedFamiliar:   req.NeedFamiliar,
		FamiliarOrders: familiarOrders,
		ServiceDate:    date.Time(),
		Rules:          json.RawMessage(sku.Rules),
	})
	if err != nil {
		return pricing.Result{}, errs.BizMsg(errs.CodeInvalidParam, err.Error())
	}
	return res, nil
}

// ToOrderResp 组装订单响应，节点按服务类型映射文案。
// 导出供服务方端复用，避免两处各写一套转换逻辑。
func (s *OrderService) ToOrderResp(o *model.Order, events []model.OrderEvent) dto.OrderResp {
	resp := dto.OrderResp{
		OrderNo:     o.OrderNo,
		ServiceType: string(o.ServiceType),
		Plan:        string(o.Plan),
		ServiceDate: o.ServiceDate.String(),
		SlotCode:    o.SlotCode,
		Status:      string(o.Status),
		StatusLabel: o.Status.Label(),
		Tab:         o.Status.Tab(),
		TotalAmount: o.TotalAmount,
		PaidAmount:  o.PaidAmount,
		Remark:      o.Remark,
		CreatedAt:   o.CreatedAt,
		Nodes:       buildNodes(o.ServiceType, o.Status, events),
	}

	if len(o.AddressSnapshot) > 0 {
		var addr dto.AddressResp
		if err := json.Unmarshal(o.AddressSnapshot, &addr); err == nil {
			resp.Address = &addr
		}
	}
	if len(o.PricingSnapshot) > 0 {
		var p dto.PreviewOrderResp
		if err := json.Unmarshal(o.PricingSnapshot, &p); err == nil {
			resp.PriceItems = p.Items
		}
	}
	if o.Status == model.OrderPendingPayment {
		expire := o.CreatedAt.Add(PayTimeout)
		resp.PayExpiresAt = &expire
	}
	if s.quotes != nil {
		if q, err := s.quotes.LatestByOrder(context.Background(), o.ID); err == nil && q != nil {
			resp.Quote = &dto.QuoteResp{
				Amount:      q.Amount,
				DurationMin: q.DurationMin,
				Confirmed:   q.ConfirmedAt != nil,
				CreatedAt:   q.CreatedAt,
			}
		}
	}
	return resp
}

// buildNodes 生成进度条数据。三类服务的中间节点文案不同，由服务类型驱动。
func buildNodes(st model.ServiceType, status model.OrderStatus, events []model.OrderEvent) []dto.NodeResp {
	type nodeDef struct {
		key   string
		label string
	}
	var defs []nodeDef

	switch st {
	case model.ServiceDogWalk:
		defs = []nodeDef{
			{string(model.OrderAccepted), "已接宠"},
			{string(model.OrderInService), "服务中"},
			{string(model.OrderPendingConfirm), "待送回"},
		}
	case model.ServiceAccompany:
		defs = []nodeDef{
			{string(model.OrderAccepted), "已受理"},
			{string(model.OrderInService), "服务中"},
			{string(model.OrderPendingConfirm), "待完成"},
		}
	default:
		// 垃圾类可能跳过「待上门」直接开始服务，节点里保留服务中
		defs = []nodeDef{
			{string(model.OrderAccepted), "已受理"},
			{string(model.OrderOnTheWay), "待上门"},
			{string(model.OrderInService), "服务中"},
			{string(model.OrderPendingConfirm), "待完成"},
		}
	}

	reached := make(map[string]model.OrderEvent, len(events))
	for _, e := range events {
		reached[e.Node] = e
	}

	// 已完成时三个节点全部点亮
	allDone := status == model.OrderCompleted

	nodes := make([]dto.NodeResp, 0, len(defs))
	for _, d := range defs {
		n := dto.NodeResp{Node: d.key, Label: d.label}
		if e, ok := reached[d.key]; ok {
			n.Reached = true
			at := e.CreatedAt
			n.At = &at
			n.Remark = e.Remark
		}
		if allDone {
			n.Reached = true
		}
		nodes = append(nodes, n)
	}
	return nodes
}

func toPriceItems(items []pricing.Item) []dto.PriceItem {
	out := make([]dto.PriceItem, 0, len(items))
	for _, it := range items {
		out = append(out, dto.PriceItem{Name: it.Name, Amount: it.Amount, Note: it.Note})
	}
	return out
}

func generateOrderNo() string {
	return "CH" + time.Now().Format("20060102150405") + util.RandomDigits(4)
}

func serviceName(st model.ServiceType) string {
	switch st {
	case model.ServiceGarbage:
		return "代倒垃圾服务"
	case model.ServiceDogWalk:
		return "遛宠服务"
	case model.ServiceAccompany:
		return "陪诊服务"
	default:
		return "社区生活服务"
	}
}

func defaultPlan(p string) string {
	if p == "" {
		return string(model.PlanSingle)
	}
	return p
}

func defaultStr(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func defaultInt(v, def int) int {
	if v <= 0 {
		return def
	}
	return v
}

func nilIfZero(f float64) *float64 {
	if f == 0 {
		return nil
	}
	return &f
}
