package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/alinger/community-handyman/server/internal/model"
)

// RecordRepo 服务记录数据访问。不做删除，符合「记录不可删除」的业务要求。
type RecordRepo struct {
	db *gorm.DB
}

func NewRecordRepo(db *gorm.DB) *RecordRepo { return &RecordRepo{db: db} }

func (r *RecordRepo) Create(ctx context.Context, rec *model.ServiceRecord) error {
	return r.db.WithContext(ctx).Create(rec).Error
}

func (r *RecordRepo) ListByOrder(ctx context.Context, orderID uint64) ([]model.ServiceRecord, error) {
	var list []model.ServiceRecord
	err := r.db.WithContext(ctx).Where("order_id = ?", orderID).Order("id").Find(&list).Error
	return list, err
}

func (r *RecordRepo) GetByID(ctx context.Context, id uint64) (*model.ServiceRecord, error) {
	var rec model.ServiceRecord
	err := r.db.WithContext(ctx).First(&rec, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &rec, nil
}

// ListByProvider 查询某服务方名下的订单。
//
// 包月订单的 status 会一直停在「已受理」跑满整个服务期，只看状态分不出
// 「今天这一趟走到哪了」。所以三个 tab 都要把当天的核销记录算进来：
// 今天打过卡（人到了）归「服务中」，点过「完成服务」才归「已完成」，
// 第二天没有核销记录又自动回到「待服务」。
// 判断完全依赖查询当天的日期，不需要定时任务去翻状态。
func (r *OrderRepo) ListByProvider(ctx context.Context, providerID uint64, tab string, page, size int) ([]model.Order, int64, error) {
	today := model.NewDate(time.Now()).String()
	// 包月今天这趟是否已上门（打了卡）。单次订单没有核销记录，对它们恒为假。
	redeemedToday := "(plan = 'monthly' AND EXISTS (SELECT 1 FROM monthly_redemptions mr " +
		"WHERE mr.order_id = orders.id AND mr.service_date = ?))"
	// 包月今天这趟是否已收尾（点了「完成服务」）。
	finishedToday := "(plan = 'monthly' AND EXISTS (SELECT 1 FROM monthly_redemptions mr " +
		"WHERE mr.order_id = orders.id AND mr.service_date = ? AND mr.finished_at IS NOT NULL))"

	orderBy := "service_date, id"
	q := r.db.WithContext(ctx).Model(&model.Order{}).Where("provider_id = ?", providerID)
	switch tab {
	case "pending":
		// 今天已经上过门的月卡不在待服务：那一趟不再需要「出发上门」，
		// 它此刻在「服务中」等着点「完成服务」。
		q = q.Where("status IN ?", []model.OrderStatus{
			model.OrderPendingAccept, model.OrderAccepted, model.OrderOnTheWay}).
			Where("NOT "+redeemedToday, today)
	case "processing":
		// 月卡打卡后 status 仍是「已受理」，靠当天核销记录把它认成「服务中」：
		// 人到了、袋子还没提走，这一趟正等着收尾。
		q = q.Where("status IN ? OR ("+redeemedToday+" AND NOT "+finishedToday+")",
			[]model.OrderStatus{model.OrderInService, model.OrderPendingConfirm}, today, today)
	case "completed":
		// 除了真正结束的单，这里也承接「今天这趟已经做完」的包月单：
		// 服务方每天关心的是还剩几趟要跑，跑完的留在待服务里反而一直在催他。
		// 已完成是回看视角，最近的排最前——包月单的 service_date 是服务期首日，
		// 按日期升序会把它压到一堆历史单后面，看起来像没生效。
		q = q.Where("status = ? OR "+finishedToday, model.OrderCompleted, today)
		orderBy = "service_date DESC, id DESC"
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []model.Order
	err := q.Order(orderBy).Offset((page - 1) * size).Limit(size).Find(&list).Error
	return list, total, err
}

// ListOpenForProvider 查询可接单的订单（已支付、未派单）。
func (r *OrderRepo) ListOpenForProvider(ctx context.Context, serviceType string, page, size int) ([]model.Order, int64, error) {
	q := r.db.WithContext(ctx).Model(&model.Order{}).
		Where("status = ? AND provider_id IS NULL", model.OrderPendingAccept)
	if serviceType != "" {
		q = q.Where("service_type = ?", serviceType)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []model.Order
	err := q.Order("service_date, slot_code, id").Offset((page - 1) * size).Limit(size).Find(&list).Error
	return list, total, err
}
