package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/alinger/community-handyman/server/internal/dto"
	"github.com/alinger/community-handyman/server/internal/errs"
	"github.com/alinger/community-handyman/server/internal/model"
	"github.com/alinger/community-handyman/server/internal/payment"
	"github.com/alinger/community-handyman/server/internal/pricing"
	"github.com/alinger/community-handyman/server/internal/repository"
	"github.com/alinger/community-handyman/server/pkg/securefield"
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
	users     *repository.UserRepo
	quotas    *repository.QuotaRepo
	income    *repository.IncomeRepo
	notify    *NotifyService
	pay       payment.Gateway
	// box 门禁密码等敏感字段的加解密器，未配置时相关字段不会落库
	box *securefield.Box
}

func NewOrderService(db *gorm.DB, orders *repository.OrderRepo, slots *repository.SlotRepo,
	skus *repository.SkuRepo, addresses *repository.AddressRepo, areas *repository.AreaRepo,
	quotes *repository.QuoteRepo, users *repository.UserRepo, quotas *repository.QuotaRepo,
	income *repository.IncomeRepo, notify *NotifyService, pay payment.Gateway,
	box *securefield.Box) *OrderService {
	return &OrderService{db: db, orders: orders, slots: slots, skus: skus,
		addresses: addresses, areas: areas, quotes: quotes, users: users,
		quotas: quotas, income: income, notify: notify, pay: pay, box: box}
}

// AccompanyOptions 返回陪诊可选项（医院与可勾选服务项及单价）。
// 数据来自 service_skus.rules，后台用 PUT /api/admin/skus/{id} 改，用户端只读。
func (s *OrderService) AccompanyOptions(ctx context.Context) (pricing.AccompanyOptions, error) {
	sku, err := s.skus.Get(ctx, model.ServiceAccompany, model.PlanSingle)
	if err != nil {
		return pricing.AccompanyOptions{}, errs.Wrap(errs.CodeInternal, err)
	}
	if sku == nil {
		return pricing.AccompanyOptions{}, errs.BizMsg(errs.CodeNotFound, "陪诊服务未配置")
	}
	opts, err := pricing.ParseAccompanyOptions(json.RawMessage(sku.Rules))
	if err != nil {
		return pricing.AccompanyOptions{}, errs.Wrap(errs.CodeInternal, err)
	}
	if len(opts.Hospitals) == 0 || len(opts.ServiceItems) == 0 {
		return pricing.AccompanyOptions{}, errs.BizMsg(errs.CodeNotFound, "陪诊选项尚未配置，请联系运营在后台设置")
	}
	return opts, nil
}

// DogwalkOptions 返回遛宠预约选项（服务时长档位与一口价）。
// 数据来自 service_skus.rules.duration_options，后台用 PUT /api/admin/skus/{id} 改，用户端只读。
func (s *OrderService) DogwalkOptions(ctx context.Context) (pricing.DogWalkRules, error) {
	sku, err := s.skus.Get(ctx, model.ServiceDogWalk, model.PlanSingle)
	if err != nil {
		return pricing.DogWalkRules{}, errs.Wrap(errs.CodeInternal, err)
	}
	if sku == nil {
		return pricing.DogWalkRules{}, errs.BizMsg(errs.CodeNotFound, "遛宠服务未配置")
	}
	var rules pricing.DogWalkRules
	if err := json.Unmarshal(json.RawMessage(sku.Rules), &rules); err != nil {
		return pricing.DogWalkRules{}, errs.Wrap(errs.CodeInternal, err)
	}
	if len(rules.DurationOptions) == 0 {
		return pricing.DogWalkRules{}, errs.BizMsg(errs.CodeNotFound, "遛宠时长档位尚未配置，请联系运营在后台设置")
	}
	return rules, nil
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

	// 单次倒垃圾若还有免费次数，试算也要显示 0 元：
	// 否则页面显示 5 元、下单却是 0 元，用户会以为价格出了错。
	if model.ServiceType(req.ServiceType) == model.ServiceGarbage &&
		model.PlanType(defaultPlan(req.Plan)) == model.PlanSingle {
		q, qerr := s.quotas.Get(ctx, userID, model.ServiceGarbage)
		if qerr == nil && q.Remaining() > 0 {
			res = pricing.Result{
				Total: 0,
				Items: []pricing.Item{{
					Name:   "免费倒垃圾（次数抵扣）",
					Amount: 0,
					Note:   fmt.Sprintf("原价 %s，已用赠送次数抵扣（剩余 %d 次）", yuan(res.Total), q.Remaining()),
				}},
			}
		}
	}

	out := &dto.PreviewOrderResp{Total: res.Total, Items: toPriceItems(res.Items), ManualQuote: res.ManualQuote}
	if res.Familiar != nil {
		out.Familiar = &dto.PriceFamiliar{
			Done: res.Familiar.Done,
			Need: res.Familiar.Need,
			Free: res.Familiar.Free,
		}
	}
	return out, nil
}

// Create 创建订单并预占库存，随后发起支付。
// 陪诊自 000006 起按服务项自动计价，与其它服务一样下单即支付；
// res.ManualQuote 恒为 false，下面的分支仅作为历史兜底保留。
func (s *OrderService) Create(ctx context.Context, userID uint64, req dto.CreateOrderReq) (*model.Order, *payment.PrepayResult, error) {
	addr, sku, date, err := s.validate(ctx, userID, req)
	if err != nil {
		return nil, nil, err
	}

	res, err := s.calc(ctx, userID, addr, sku, date, req)
	if err != nil {
		return nil, nil, err
	}

	// 代倒垃圾自 000013 起的业务口径：
	//   - 包月：30 元 / 30 天，每晚固定时段派单，不占时段库存；
	//   - 单次：只能用注册赠送的免费次数，额度用完必须买包月。
	// 其它服务维持原有「选时段 + 扣库存」流程。
	isGarbage := model.ServiceType(req.ServiceType) == model.ServiceGarbage
	plan := model.PlanType(defaultPlan(req.Plan))
	isMonthly := isGarbage && plan == model.PlanMonthly

	var (
		slotCode      = req.SlotCode
		extraSlot     = ""
		monthlyStart  *model.Date
		monthlyEnd    *model.Date
		freeQuotaUsed bool
	)

	if isGarbage {
		rules := parseGarbageRules(json.RawMessage(sku.Rules))
		if isMonthly {
			// 包月不锁库存：用户买了 30 天服务，不该因为某天库存满了而约不上
			if rules.DefaultSlot != "" {
				slotCode = rules.DefaultSlot
			}
			if req.ExtraSlotCode != "" {
				if !containsStr(rules.ExtraSlotOptions, req.ExtraSlotCode) {
					return nil, nil, errs.BizMsg(errs.CodeInvalidParam, "备选服务时间不在可选项内")
				}
				extraSlot = req.ExtraSlotCode
			}
			days := rules.DurationDays
			if days <= 0 {
				days = 30
			}
			start := model.NewDate(date.Time())
			end := model.NewDate(date.Time().AddDate(0, 0, days-1))
			monthlyStart, monthlyEnd = &start, &end
		} else {
			// 单次：正常按次收费；仍有赠送额度时自动抵扣（额度只是抵扣，不是准入门槛）
			quota, qerr := s.quotas.Get(ctx, userID, model.ServiceGarbage)
			if qerr != nil {
				return nil, nil, errs.Wrap(errs.CodeInternal, qerr)
			}
			if quota.Remaining() > 0 {
				freeQuotaUsed = true
				res = pricing.Result{
					Total: 0,
					Items: []pricing.Item{{
						Name:   "免费倒垃圾（次数抵扣）",
						Amount: 0,
						Note: fmt.Sprintf("原价 %s，已用赠送次数抵扣（剩余 %d 次）",
							yuan(res.Total), quota.Remaining()),
					}},
				}
			}
		}
	}

	// 时段库存校验：total 为 0 表示该时段不开放。
	// 可选时段由后台「时段库存」配置，铺满全天即可让用户任选任意时间段。
	if !isMonthly {
		slot, serr := s.slots.Get(ctx, date, slotCode, model.ServiceType(req.ServiceType))
		if serr != nil {
			return nil, nil, errs.Wrap(errs.CodeInternal, serr)
		}
		if slot == nil || !slot.Available() {
			return nil, nil, errs.Biz(errs.CodeSlotFull)
		}
	}

	addrSnapshot, _ := json.Marshal(ToAddressResp(addr))
	pricingSnapshot, _ := json.Marshal(dto.PreviewOrderResp{Total: res.Total, Items: toPriceItems(res.Items), ManualQuote: res.ManualQuote})

	// 陪诊把用户所选服务项的 code 单独留档：后台改价后仍可还原当时选了什么；
	// 医院存名称而不是 code，详情页直接可读，也避免后台改名后旧订单显示不出内容
	var serviceItems model.JSON
	hospitalName := req.Hospital
	if req.ServiceType == string(model.ServiceAccompany) {
		serviceItems, _ = json.Marshal(req.ServiceItems)
		if opts, perr := pricing.ParseAccompanyOptions(json.RawMessage(sku.Rules)); perr == nil {
			for _, h := range opts.Hospitals {
				if h.Code == req.Hospital {
					hospitalName = h.Name
					break
				}
			}
		}
	}

	orderNo := generateOrderNo()
	order := &model.Order{
		OrderNo:         orderNo,
		UserID:          userID,
		ServiceType:     model.ServiceType(req.ServiceType),
		Plan:            model.PlanType(defaultPlan(req.Plan)),
		AddressID:       addr.ID,
		AddressSnapshot: addrSnapshot,
		ServiceDate:     date,
		SlotCode:        slotCode,
		ExtraSlotCode:   extraSlot,
		MonthlyStart:    monthlyStart,
		MonthlyEnd:      monthlyEnd,
		FreeQuotaUsed:   freeQuotaUsed,
		Quantity:        defaultInt(req.Quantity, 1),
		PetCount:        req.PetCount,
		DurationMin:     req.DurationMin,
		DistanceKm:      nilIfZero(req.DistanceKm),
		GarbageType:     req.GarbageType,
		Hospital:        hospitalName,
		PetNotes:        strings.TrimSpace(req.PetNotes),
		EntryMethod:     req.EntryMethod,
		AccessInfo:      encryptAccessInfo(s.box, req.AccessInfo, orderNo),
		PatientName:     req.PatientName,
		ServiceItems:    serviceItems,
		Remark:          req.Remark,
		PricingSnapshot: pricingSnapshot,
		TotalAmount:     res.Total,
		Status:          model.OrderPendingPayment,
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 免费次数在事务内扣减：条件更新保证并发下不会两个请求抢同一次额度
		if freeQuotaUsed {
			ok, qerr := s.quotas.WithTx(tx).Consume(ctx, userID, model.ServiceGarbage)
			if qerr != nil {
				return errs.Wrap(errs.CodeInternal, qerr)
			}
			if !ok {
				return errs.BizMsg(errs.CodeQuotaExhausted, "免费次数已被用完，请重试或购买包月")
			}
		}
		if !isMonthly {
			ok, rerr := s.slots.WithTx(tx).Reserve(ctx, date, slotCode, order.ServiceType)
			if rerr != nil {
				return errs.Wrap(errs.CodeInternal, rerr)
			}
			if !ok {
				return errs.Biz(errs.CodeSlotFull)
			}
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

	// 0 元订单（免费次数抵扣）没有支付环节，渠道也不接受金额 0 的预下单
	if order.TotalAmount <= 0 {
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
		return nil, errs.BizMsg(errs.CodeIllegalStatus, "订单金额异常，请联系客服")
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
	now := time.Now()
	for i := range list {
		resp := s.ToOrderResp(&list[i], nil)
		applyAccessInfo(&resp, &list[i], userID, s.box, now)
		out = append(out, resp)
	}
	return out, total, nil
}

// AttachAccess 按查看者身份填充进入方式与门禁信息。
// 导出给服务方端复用：服务方订单列表也走同一套可见性规则，
// 免得两处各写一套判断，日后改窗口时间要改两个地方。
func (s *OrderService) AttachAccess(resp *dto.OrderResp, o *model.Order, viewerID uint64) {
	applyAccessInfo(resp, o, viewerID, s.box, time.Now())
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
	applyAccessInfo(&resp, o, userID, s.box, time.Now())
	// 服务方联系方式只在接单后下发：未接单时连是谁都不知道，
	// 提前给出电话会让住户绕过平台催单，也是服务方不愿意的。
	if o.ProviderID != nil && providerContactVisible(o.Status) {
		if pu, err := s.users.GetByID(ctx, *o.ProviderID); err == nil && pu != nil {
			resp.ProviderNickname = pu.Nickname
			resp.ProviderPhone = pu.Phone
		}
	}
	return &resp, nil
}

// providerContactVisible 判断当前状态是否可以给住户看服务方电话。
// 从「已接单」开始才可见，取消/被拒/未支付/待接单都不给。
func providerContactVisible(st model.OrderStatus) bool {
	switch st {
	case model.OrderAccepted, model.OrderInService, model.OrderPendingConfirm, model.OrderCompleted:
		return true
	}
	return false
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
		// 包月订单从未占用库存，Release 会按 slot_code(22:00-22:30) 找不到记录；
		// Release 内部对不存在的库存是幂等忽略，因此这里只在非包月时调用，语义更清楚。
		if o.Plan != model.PlanMonthly {
			if err := s.slots.WithTx(tx).Release(ctx, o.ServiceDate, o.SlotCode, o.ServiceType); err != nil {
				return errs.Wrap(errs.CodeInternal, err)
			}
		}
		// 免费次数抵扣的订单被取消，把次数还回去，避免用户白丢一次机会
		if o.FreeQuotaUsed {
			if err := s.quotas.WithTx(tx).Restore(ctx, o.UserID, o.ServiceType); err != nil {
				return errs.Wrap(errs.CodeInternal, err)
			}
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

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ok, err := s.orders.WithTx(tx).UpdateStatus(ctx, o.ID,
			model.OrderPendingConfirm, model.OrderCompleted,
			map[string]any{"finished_at": time.Now()})
		if err != nil {
			return errs.Wrap(errs.CodeInternal, err)
		}
		if !ok {
			return errs.Biz(errs.CodeIllegalStatus)
		}

		if err := s.orders.WithTx(tx).AddEvent(ctx, &model.OrderEvent{
			OrderID:  o.ID,
			Node:     string(model.OrderCompleted),
			Operator: "user",
			Remark:   "用户确认服务完成",
		}); err != nil {
			return errs.Wrap(errs.CodeInternal, err)
		}

		// 流程图 P2：订单完成 → 自动入账 + 通知客户。
		// 入账幂等（order_id 唯一），通知失败只记日志。
		if err := s.income.WithTx(tx).Create(ctx, &model.ProviderIncome{
			ProviderID: o.ProviderIDOrDefault(),
			OrderID:    o.ID,
			OrderNo:    o.OrderNo,
			Amount:     o.PaidAmount,
		}); err != nil {
			return errs.Wrap(errs.CodeInternal, err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	s.notify.OrderEvent(ctx, userID, orderNo, "服务已完成", "服务已完成，可查看服务报告")
	return nil
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

	// 熟悉费减免：单次卡累计已完成 N 单后免。遛宠目前只有单次卡，
	// 因此这里只统计单次卡订单；将来若加月卡，月卡订单不会计入累计。
	familiarOrders := 0
	if req.NeedFamiliar {
		n, err := s.orders.CountSingleCompleted(ctx, userID, sku.ServiceType)
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
		Hospital:       req.Hospital,
		PatientName:    req.PatientName,
		ServiceItems:   req.ServiceItems,
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

		ExtraSlotCode: o.ExtraSlotCode,
		FreeQuotaUsed: o.FreeQuotaUsed,
		PetNotes:      o.PetNotes,
		// 这几个字段原来只在后台 handler 里补，住户侧接口一直拿不到：
		// 「完成时间」明明写库了却不下发，前端没法展示；数量同理。
		Quantity:   o.Quantity,
		PetCount:   o.PetCount,
		PaidAt:     o.PaidAt,
		FinishedAt: o.FinishedAt,
		// 拒单原因与退款金额对用户可见：拒单自动退款后，用户要能自己查到钱去哪了
		CancelReason:   o.CancelReason,
		RefundedAmount: o.RefundedAmount,
	}
	if o.MonthlyStart != nil {
		resp.MonthlyStart = o.MonthlyStart.String()
	}
	if o.MonthlyEnd != nil {
		resp.MonthlyEnd = o.MonthlyEnd.String()
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
			if o.ServiceType == model.ServiceAccompany {
				resp.AccompanyItems = p.Items
			}
		}
	}
	if o.ServiceType == model.ServiceAccompany {
		resp.Hospital = o.Hospital
		resp.PatientName = o.PatientName
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
		// 文案对齐流程图：开始打卡（入户/接到宠物）→ 服务中记录 → 结束打卡（锁门/交还宠物）
		defs = []nodeDef{
			{string(model.OrderAccepted), "已认领"},
			{string(model.OrderInService), "开始打卡 · 服务中"},
			{string(model.OrderPendingConfirm), "结束打卡 · 待确认"},
		}
	case model.ServiceAccompany:
		defs = []nodeDef{
			{string(model.OrderAccepted), "已受理"},
			{string(model.OrderInService), "服务中"},
			{string(model.OrderPendingConfirm), "待完成"},
		}
	default:
		// 垃圾代倒一次上门就结束：服务方点「出发上门」直接进服务中，
		// 点「完成服务」就终态，没有「待上门」「待完成」这两个中间态。
		// 时间轴必须跟着减节点，否则会出现「服务中已点亮、待上门未点亮」的错位。
		defs = []nodeDef{
			{string(model.OrderAccepted), "已受理"},
			{string(model.OrderInService), "服务中"},
			{string(model.OrderCompleted), "已完成"},
		}
	}

	reached := make(map[string]model.OrderEvent, len(events))
	for _, e := range events {
		reached[e.Node] = e
	}

	// 已完成时全部节点点亮（含没有事件的历史单）
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

// SlotAvailability 返回某天某服务类型的时段余量，供预约页把满员时段置灰。
// 后台没配该日库存时返回空列表：前端按"全部时段可选"处理，
// 与下单时后端兜底（库存记录不存在则拒绝）保持一致。
func (s *OrderService) SlotAvailability(ctx context.Context, dateStr string, st model.ServiceType) ([]dto.SlotAvailability, error) {
	date, err := model.ParseDate(dateStr)
	if err != nil {
		return nil, errs.BizMsg(errs.CodeInvalidParam, "日期格式应为 YYYY-MM-DD")
	}
	list, err := s.slots.ListByDate(ctx, date, st)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	out := make([]dto.SlotAvailability, 0, len(list))
	for _, s := range list {
		remaining := s.Total - s.Used
		if remaining < 0 {
			remaining = 0
		}
		out = append(out, dto.SlotAvailability{
			SlotCode:  s.SlotCode,
			Total:     s.Total,
			Used:      s.Used,
			Remaining: remaining,
			Available: remaining > 0,
		})
	}
	return out, nil
}

// parseGarbageRules 解析垃圾 SKU 规则，解析失败返回零值（走代码里的默认值）。
func parseGarbageRules(raw json.RawMessage) pricing.GarbageRules {
	var r pricing.GarbageRules
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &r)
	}
	return r
}

// containsStr 判断字符串是否在候选列表内。
func containsStr(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

// GarbagePlan 返回代倒垃圾包月方案，供小程序下单页展示。
func (s *OrderService) GarbagePlan(ctx context.Context) (*dto.GarbagePlanResp, error) {
	sku, err := s.skus.Get(ctx, model.ServiceGarbage, model.PlanMonthly)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	if sku == nil {
		return nil, errs.BizMsg(errs.CodeNotFound, "包月方案未配置")
	}
	rules := parseGarbageRules(json.RawMessage(sku.Rules))

	price := rules.MonthlyPrice
	if price <= 0 {
		price = sku.BasePrice
	}
	days := rules.DurationDays
	if days <= 0 {
		days = 30
	}
	defaultSlot := rules.DefaultSlot
	if defaultSlot == "" {
		defaultSlot = "22:00-22:30"
	}
	extra := rules.ExtraSlotOptions
	if extra == nil {
		extra = []string{}
	}
	// 单次价格单独查：用户按次下单时前端要显示「¥5.00 / 次」，有额度则显示抵扣
	singlePrice := int64(0)
	if single, serr := s.skus.Get(ctx, model.ServiceGarbage, model.PlanSingle); serr == nil && single != nil {
		singlePrice = single.BasePrice
	}

	return &dto.GarbagePlanResp{
		Price:       price,
		SinglePrice: singlePrice,
		Days:        days,
		DefaultSlot: defaultSlot,
		ExtraSlots:  extra,
		FreePerUser: registerFreeGarbageQuota,
	}, nil
}

// yuan 把分转成「¥5.00」形式，仅用于费用说明文案。
func yuan(fen int64) string {
	return fmt.Sprintf("¥%d.%02d", fen/100, fen%100)
}
