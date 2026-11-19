package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/alinger/community-handyman/server/internal/model"
)

// ErrDuplicate 表示唯一键冲突，用于支付幂等判定。
var ErrDuplicate = errors.New("duplicate key")

// PaymentRepo 支付与退款数据访问。
type PaymentRepo struct {
	db *gorm.DB
}

func NewPaymentRepo(db *gorm.DB) *PaymentRepo { return &PaymentRepo{db: db} }

func (r *PaymentRepo) WithTx(tx *gorm.DB) *PaymentRepo { return &PaymentRepo{db: tx} }

// InsertTransaction 幂等写入支付流水。
// 依赖 transaction_id 唯一索引：冲突即说明该笔支付已处理过，返回 ErrDuplicate。
// 不要改成「先查再插」，并发回调下必然重复入账。
func (r *PaymentRepo) InsertTransaction(ctx context.Context, t *model.PaymentTransaction) error {
	res := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(t)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrDuplicate
	}
	return nil
}

// GetTransaction 按微信交易号查询。
func (r *PaymentRepo) GetTransaction(ctx context.Context, transactionID string) (*model.PaymentTransaction, error) {
	var t model.PaymentTransaction
	err := r.db.WithContext(ctx).Where("transaction_id = ?", transactionID).First(&t).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &t, nil
}

// CreateRefund 写入退款单，out_refund_no 唯一保证不重复发起。
func (r *PaymentRepo) CreateRefund(ctx context.Context, rf *model.Refund) error {
	res := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(rf)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrDuplicate
	}
	return nil
}

// UpdateRefundStatus 更新退款结果。
func (r *PaymentRepo) UpdateRefundStatus(ctx context.Context, outRefundNo, status string) error {
	fields := map[string]any{"status": status}
	if status == model.RefundSuccess {
		fields["refunded_at"] = gorm.Expr("NOW(3)")
	}
	return r.db.WithContext(ctx).Model(&model.Refund{}).
		Where("out_refund_no = ?", outRefundNo).Updates(fields).Error
}
