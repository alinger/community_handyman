package repository

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/alinger/community-handyman/server/internal/model"
)

// AdminUserRepo 后台账号数据访问。
type AdminUserRepo struct {
	db *gorm.DB
}

func NewAdminUserRepo(db *gorm.DB) *AdminUserRepo { return &AdminUserRepo{db: db} }

func (r *AdminUserRepo) GetByUsername(ctx context.Context, username string) (*model.AdminUser, error) {
	var a model.AdminUser
	err := r.db.WithContext(ctx).Where("username = ? AND status = ?", username, 1).First(&a).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &a, nil
}

// ListAreas 后台查看全部范围规则，含停用。
func (r *AreaRepo) ListAreas(ctx context.Context) ([]model.ServiceArea, error) {
	var list []model.ServiceArea
	err := r.db.WithContext(ctx).Order("id").Find(&list).Error
	return list, err
}

// CreateArea 新增范围规则。
func (r *AreaRepo) CreateArea(ctx context.Context, a *model.ServiceArea) error {
	return r.db.WithContext(ctx).Create(a).Error
}

// UpdateAreaStatus 启停范围规则。停用后已下单订单不受影响。
func (r *AreaRepo) UpdateAreaStatus(ctx context.Context, id uint64, status int8) error {
	return r.db.WithContext(ctx).Model(&model.ServiceArea{}).
		Where("id = ?", id).Update("status", status).Error
}

// SetSlotTotal 设置某天某时段的可接单量，不存在则创建。
// 用 upsert 而不是先查后插，避免并发下建出两条同键记录。
func (r *SlotRepo) SetSlotTotal(ctx context.Context, date model.Date, slotCode string, st model.ServiceType, total int) error {
	s := &model.SlotInventory{
		ServiceDate: date,
		SlotCode:    slotCode,
		ServiceType: st,
		Total:       total,
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "service_date"}, {Name: "slot_code"}, {Name: "service_type"}},
		DoUpdates: clause.AssignmentColumns([]string{"total", "updated_at"}),
	}).Create(s).Error
}

// ListSlotsByDate 后台查看某天全部时段。
func (r *SlotRepo) ListSlotsByDate(ctx context.Context, date model.Date) ([]model.SlotInventory, error) {
	var list []model.SlotInventory
	err := r.db.WithContext(ctx).Where("service_date = ?", date.Time()).
		Order("service_type, slot_code").Find(&list).Error
	return list, err
}

// ListSkus 后台查看全部 SKU，含停用。
func (r *SkuRepo) ListSkus(ctx context.Context) ([]model.ServiceSku, error) {
	var list []model.ServiceSku
	err := r.db.WithContext(ctx).Order("service_type, plan").Find(&list).Error
	return list, err
}

// UpdateSkuPrice 调整价格与计价规则。改完立即生效，无需发版。
// rules 用 JSON_MERGE_PATCH 做部分合并：运营只传要改的键（例如仅 holiday_dates），
// 不会把其余计价参数整块覆盖掉——整体替换是这类接口最常见的误操作来源。
func (r *SkuRepo) UpdateSkuPrice(ctx context.Context, id uint64, basePrice int64, rules []byte) error {
	fields := map[string]any{"base_price": basePrice, "updated_at": time.Now()}
	if len(rules) > 0 {
		fields["rules"] = gorm.Expr("JSON_MERGE_PATCH(COALESCE(rules, JSON_OBJECT()), ?)", string(rules))
	}
	return r.db.WithContext(ctx).Model(&model.ServiceSku{}).
		Where("id = ?", id).Updates(fields).Error
}

// ListAllOrders 后台订单查询，支持状态与服务类型筛选。
func (r *OrderRepo) ListAllOrders(ctx context.Context, status, serviceType, keyword string, page, size int) ([]model.Order, int64, error) {
	q := r.db.WithContext(ctx).Model(&model.Order{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if serviceType != "" {
		q = q.Where("service_type = ?", serviceType)
	}
	if keyword != "" {
		q = q.Where("order_no LIKE ?", "%"+keyword+"%")
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []model.Order
	err := q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&list).Error
	return list, total, err
}

// AssignProvider 派单或改派。
func (r *OrderRepo) AssignProvider(ctx context.Context, orderID uint64, providerID uint64) (bool, error) {
	res := r.db.WithContext(ctx).Model(&model.Order{}).
		Where("id = ? AND status = ?", orderID, model.OrderPendingAccept).
		Updates(map[string]any{"provider_id": providerID, "updated_at": time.Now()})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

// ListRefundsByOrder 查询订单退款记录。
func (r *PaymentRepo) ListRefundsByOrder(ctx context.Context, orderID uint64) ([]model.Refund, error) {
	var list []model.Refund
	err := r.db.WithContext(ctx).Where("order_id = ?", orderID).Order("id").Find(&list).Error
	return list, err
}
