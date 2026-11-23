package service

import (
	"context"

	"github.com/alinger/community-handyman/server/internal/dto"
	"github.com/alinger/community-handyman/server/internal/errs"
	"github.com/alinger/community-handyman/server/internal/model"
	"github.com/alinger/community-handyman/server/internal/repository"
)

// QuotaService 免费次数查询。
// 只看不发：发放由注册流程触发，扣减由下单事务完成。
type QuotaService struct {
	quotas *repository.QuotaRepo
}

func NewQuotaService(quotas *repository.QuotaRepo) *QuotaService {
	return &QuotaService{quotas: quotas}
}

// List 返回该用户的免费次数（当前只有代倒垃圾有额度）。
// 没领过额度的用户返回 0，前端据此判断是否需要展示「免费次数」入口。
func (s *QuotaService) List(ctx context.Context, userID uint64) ([]dto.QuotaResp, error) {
	q, err := s.quotas.Get(ctx, userID, model.ServiceGarbage)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	return []dto.QuotaResp{{
		ServiceType: string(q.ServiceType),
		Total:       q.Total,
		Used:        q.Used,
		Remaining:   q.Remaining(),
	}}, nil
}
