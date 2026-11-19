package service

import (
	"context"
	"encoding/json"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/alinger/community-handyman/server/internal/auth"
	"github.com/alinger/community-handyman/server/internal/errs"
	"github.com/alinger/community-handyman/server/internal/model"
	"github.com/alinger/community-handyman/server/internal/repository"
)

// AdminService 运营后台：登录、配置、订单管理。
type AdminService struct {
	db     *gorm.DB
	admins *repository.AdminUserRepo
	areas  *repository.AreaRepo
	slots  *repository.SlotRepo
	skus   *repository.SkuRepo
	orders *repository.OrderRepo

	secret   string
	tokenTTL time.Duration
}

func NewAdminService(db *gorm.DB, admins *repository.AdminUserRepo, areas *repository.AreaRepo,
	slots *repository.SlotRepo, skus *repository.SkuRepo, orders *repository.OrderRepo,
	secret string, ttl time.Duration) *AdminService {
	return &AdminService{db: db, admins: admins, areas: areas, slots: slots,
		skus: skus, orders: orders, secret: secret, tokenTTL: ttl}
}

// Login 校验账号密码并签发 token。
// 密码错误与账号不存在返回同一提示，避免被用来枚举用户名。
func (s *AdminService) Login(ctx context.Context, username, password string) (string, *model.AdminUser, error) {
	a, err := s.admins.GetByUsername(ctx, username)
	if err != nil {
		return "", nil, errs.Wrap(errs.CodeInternal, err)
	}
	if a == nil {
		return "", nil, errs.BizMsg(errs.CodeUnauthorized, "账号或密码错误")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(a.PasswordHash), []byte(password)); err != nil {
		return "", nil, errs.BizMsg(errs.CodeUnauthorized, "账号或密码错误")
	}

	token := auth.Sign(s.secret, auth.Claims{Sub: a.ID, Role: a.Role}, s.tokenTTL)
	return token, a, nil
}

// ---------- 服务范围 ----------

// ListAreas 全部范围规则。
func (s *AdminService) ListAreas(ctx context.Context) ([]model.ServiceArea, error) {
	list, err := s.areas.ListAreas(ctx)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	return list, nil
}

// CreateArea 新增范围规则。
func (s *AdminService) CreateArea(ctx context.Context, a *model.ServiceArea) (*model.ServiceArea, error) {
	if a.Community == "" || a.Lane == "" {
		return nil, errs.BizMsg(errs.CodeInvalidParam, "小区与路弄不能为空")
	}
	if a.HouseParity == "" {
		a.HouseParity = "all"
	}
	if a.Status == 0 {
		a.Status = 1
	}
	if err := s.areas.CreateArea(ctx, a); err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	return a, nil
}

// SetAreaStatus 启停范围。停用不影响已存在订单。
func (s *AdminService) SetAreaStatus(ctx context.Context, id uint64, status int8) error {
	if status != 0 && status != 1 {
		return errs.BizMsg(errs.CodeInvalidParam, "status 只能是 0 或 1")
	}
	return s.areas.UpdateAreaStatus(ctx, id, status)
}

// ---------- 时段库存 ----------

// ListSlots 某天全部时段。
func (s *AdminService) ListSlots(ctx context.Context, dateStr string) ([]model.SlotInventory, error) {
	date, err := model.ParseDate(dateStr)
	if err != nil {
		return nil, errs.BizMsg(errs.CodeInvalidParam, "日期格式应为 2006-01-02")
	}
	list, err := s.slots.ListSlotsByDate(ctx, date)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	return list, nil
}

// SlotTotalInput 单条时段设置。
type SlotTotalInput struct {
	SlotCode    string `json:"slot_code" binding:"required,max=16"`
	ServiceType string `json:"service_type" binding:"required,oneof=garbage dogwalk accompany"`
	Total       int    `json:"total" binding:"min=0,max=200"`
}

// SetSlotTotals 批量设置某天可接单量。total=0 表示该时段不开放。
func (s *AdminService) SetSlotTotals(ctx context.Context, dateStr string, items []SlotTotalInput) error {
	date, err := model.ParseDate(dateStr)
	if err != nil {
		return errs.BizMsg(errs.CodeInvalidParam, "日期格式应为 2006-01-02")
	}
	if len(items) == 0 {
		return errs.BizMsg(errs.CodeInvalidParam, "至少提供一条时段设置")
	}
	for _, it := range items {
		if err := s.slots.SetSlotTotal(ctx, date, it.SlotCode, model.ServiceType(it.ServiceType), it.Total); err != nil {
			return errs.Wrap(errs.CodeInternal, err)
		}
	}
	return nil
}

// ---------- 服务产品 ----------

// ListSkus 全部 SKU。
func (s *AdminService) ListSkus(ctx context.Context) ([]model.ServiceSku, error) {
	list, err := s.skus.ListSkus(ctx)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	return list, nil
}

// UpdateSku 调整价格与计价规则。
// rules 做部分合并：只传需要改的键（如 holiday_dates），其余计价参数保持不变。
func (s *AdminService) UpdateSku(ctx context.Context, id uint64, basePrice int64, rules json.RawMessage) error {
	if basePrice < 0 {
		return errs.BizMsg(errs.CodeInvalidParam, "价格不能为负")
	}
	if len(rules) > 0 && !json.Valid(rules) {
		return errs.BizMsg(errs.CodeInvalidParam, "rules 不是合法 JSON")
	}
	if err := s.skus.UpdateSkuPrice(ctx, id, basePrice, rules); err != nil {
		return errs.Wrap(errs.CodeInternal, err)
	}
	return nil
}

// ---------- 订单管理 ----------

// ListOrders 订单查询。
func (s *AdminService) ListOrders(ctx context.Context, status, serviceType, keyword string, page, size int) ([]model.Order, int64, error) {
	normalizePage(&page, &size)
	list, total, err := s.orders.ListAllOrders(ctx, status, serviceType, keyword, page, size)
	if err != nil {
		return nil, 0, errs.Wrap(errs.CodeInternal, err)
	}
	return list, total, nil
}

// Assign 派单或改派，只能派待接单的订单。
func (s *AdminService) Assign(ctx context.Context, adminID uint64, orderNo string, providerID uint64) error {
	o, err := s.orders.GetByNo(ctx, orderNo)
	if err != nil {
		return errs.Wrap(errs.CodeInternal, err)
	}
	if o == nil {
		return errs.Biz(errs.CodeNotFound)
	}
	if o.Status != model.OrderPendingAccept {
		return errs.BizMsg(errs.CodeIllegalStatus, "只有待接单的订单可以派单")
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ok, err := s.orders.WithTx(tx).AssignProvider(ctx, o.ID, providerID)
		if err != nil {
			return errs.Wrap(errs.CodeInternal, err)
		}
		if !ok {
			return errs.Biz(errs.CodeIllegalStatus)
		}
		return s.orders.WithTx(tx).AddEvent(ctx, &model.OrderEvent{
			OrderID:    o.ID,
			Node:       string(model.OrderPendingAccept),
			Operator:   "admin",
			OperatorID: &adminID,
			Remark:     "运营派单",
		})
	})
}
