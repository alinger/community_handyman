package service

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/alinger/community-handyman/server/internal/errs"
	"github.com/alinger/community-handyman/server/internal/model"
	"github.com/alinger/community-handyman/server/internal/repository"
	"github.com/alinger/community-handyman/server/internal/storage"
	"github.com/alinger/community-handyman/server/pkg/util"
)

// 上传限制。视频按需求文档 R-07 限制单条时长，同时用大小兜底。
const (
	maxPhotoSize    = 10 << 20 // 10MB
	maxVideoSize    = 80 << 20 // 80MB
	maxDocSize      = 20 << 20 // 20MB
	maxVideoSeconds = 60
)

// RecordService 服务记录上传与读取。
type RecordService struct {
	records *repository.RecordRepo
	orders  *repository.OrderRepo
	store   storage.Storage
}

func NewRecordService(records *repository.RecordRepo, orders *repository.OrderRepo, store storage.Storage) *RecordService {
	return &RecordService{records: records, orders: orders, store: store}
}

// Upload 保存一条服务记录。
// 只允许订单的用户本人与该单服务方上传；记录一经写入不可删除。
func (s *RecordService) Upload(ctx context.Context, userID uint64, orderNo, mediaType, filename string,
	r io.Reader, size int64, durationSec int, ownerConsented bool, stage string) (*model.ServiceRecord, error) {

	order, err := s.orders.GetByNo(ctx, orderNo)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	if order == nil {
		return nil, errs.Biz(errs.CodeNotFound)
	}
	if !canAccessOrder(order, userID) {
		return nil, errs.Biz(errs.CodeForbidden)
	}

	ext, err := validateUpload(mediaType, filename, size, durationSec)
	if err != nil {
		return nil, err
	}

	key := fmt.Sprintf("records/%s/%s%s", order.OrderNo, util.NewRequestID(), ext)
	obj, err := s.store.Save(ctx, key, r, size)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}

	rec := &model.ServiceRecord{
		OrderID:        order.ID,
		MediaType:      mediaType,
		ObjectKey:      obj.Key,
		Size:           obj.Size,
		DurationSec:    durationSec,
		OwnerConsented: ownerConsented,
		Stage:          stage,
		UploaderID:     userID,
		UploaderRole:   roleOf(order, userID),
	}
	if err := s.records.Create(ctx, rec); err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	return rec, nil
}

// List 查询订单的服务记录。
func (s *RecordService) List(ctx context.Context, userID uint64, orderNo string) ([]model.ServiceRecord, error) {
	order, err := s.orders.GetByNo(ctx, orderNo)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	if order == nil {
		return nil, errs.Biz(errs.CodeNotFound)
	}
	if !canAccessOrder(order, userID) {
		return nil, errs.Biz(errs.CodeForbidden)
	}
	list, err := s.records.ListByOrder(ctx, order.ID)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	return list, nil
}

// Open 打开记录文件用于流式下载。必须校验订单归属，不能凭记录 ID 直接取。
func (s *RecordService) Open(ctx context.Context, userID uint64, orderNo string, recordID uint64) (io.ReadSeekCloser, int64, *model.ServiceRecord, error) {
	order, err := s.orders.GetByNo(ctx, orderNo)
	if err != nil {
		return nil, 0, nil, errs.Wrap(errs.CodeInternal, err)
	}
	if order == nil {
		return nil, 0, nil, errs.Biz(errs.CodeNotFound)
	}
	if !canAccessOrder(order, userID) {
		return nil, 0, nil, errs.Biz(errs.CodeForbidden)
	}

	rec, err := s.records.GetByID(ctx, recordID)
	if err != nil {
		return nil, 0, nil, errs.Wrap(errs.CodeInternal, err)
	}
	if rec == nil || rec.OrderID != order.ID {
		return nil, 0, nil, errs.Biz(errs.CodeNotFound)
	}

	f, size, err := s.store.Open(ctx, rec.ObjectKey)
	if err != nil {
		if err == storage.ErrNotFound {
			return nil, 0, nil, errs.BizMsg(errs.CodeNotFound, "文件已不存在")
		}
		return nil, 0, nil, errs.Wrap(errs.CodeInternal, err)
	}
	return f, size, rec, nil
}

// ListForAdmin 后台查看订单服务记录。后台已过 AdminAuth，跳过用户归属校验。
func (s *RecordService) ListForAdmin(ctx context.Context, orderNo string) ([]model.ServiceRecord, error) {
	order, err := s.orders.GetByNo(ctx, orderNo)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	if order == nil {
		return nil, errs.Biz(errs.CodeNotFound)
	}
	return s.records.ListByOrder(ctx, order.ID)
}

// OpenForAdmin 后台打开记录文件用于流式读取。
func (s *RecordService) OpenForAdmin(ctx context.Context, recordID uint64) (io.ReadSeekCloser, int64, *model.ServiceRecord, error) {
	rec, err := s.records.GetByID(ctx, recordID)
	if err != nil {
		return nil, 0, nil, errs.Wrap(errs.CodeInternal, err)
	}
	if rec == nil {
		return nil, 0, nil, errs.Biz(errs.CodeNotFound)
	}
	f, size, err := s.store.Open(ctx, rec.ObjectKey)
	if err != nil {
		if err == storage.ErrNotFound {
			return nil, 0, nil, errs.BizMsg(errs.CodeNotFound, "文件已不存在")
		}
		return nil, 0, nil, errs.Wrap(errs.CodeInternal, err)
	}
	return f, size, rec, nil
}

// validateUpload 校验媒体类型、扩展名与大小，返回扩展名。
func validateUpload(mediaType, filename string, size int64, durationSec int) (string, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	ext = strings.TrimPrefix(ext, ".")

	var allowExt []string
	var limit int64

	switch mediaType {
	case "photo":
		allowExt, limit = []string{"jpg", "jpeg", "png", "webp"}, maxPhotoSize
	case "video":
		allowExt, limit = []string{"mp4", "mov", "m4v"}, maxVideoSize
		if durationSec > maxVideoSeconds {
			return "", errs.BizMsg(errs.CodeInvalidParam, fmt.Sprintf("视频时长不能超过 %d 秒", maxVideoSeconds))
		}
	case "advice", "document":
		allowExt, limit = []string{"jpg", "jpeg", "png", "pdf"}, maxDocSize
	default:
		return "", errs.BizMsg(errs.CodeInvalidParam, "media_type 只能是 photo / video / advice / document")
	}

	if ext == "" {
		return "", errs.BizMsg(errs.CodeInvalidParam, "文件名缺少扩展名")
	}
	for _, e := range allowExt {
		if e == ext {
			if size > limit {
				return "", errs.BizMsg(errs.CodeInvalidParam,
					fmt.Sprintf("文件过大，%s 类型上限 %d MB", mediaType, limit>>20))
			}
			if size == 0 {
				return "", errs.BizMsg(errs.CodeInvalidParam, "文件为空")
			}
			return "." + ext, nil
		}
	}
	return "", errs.BizMsg(errs.CodeInvalidParam,
		fmt.Sprintf("%s 类型不支持 .%s 格式，允许：%s", mediaType, ext, strings.Join(allowExt, " / ")))
}

// canAccessOrder 判断用户是否有权访问该订单：下单用户或该单服务方。
func canAccessOrder(o *model.Order, userID uint64) bool {
	if o.UserID == userID {
		return true
	}
	return o.ProviderID != nil && *o.ProviderID == userID
}

// roleOf 标记上传者身份，便于纠纷时区分是住户拍的还是服务方拍的。
func roleOf(o *model.Order, userID uint64) string {
	if o.UserID == userID {
		return "user"
	}
	return "provider"
}

// ToRecordResp 记录响应，不返回 object_key，避免绕过鉴权直取文件。
type ToRecordResp struct {
	ID             uint64    `json:"id"`
	MediaType      string    `json:"media_type"`
	Size           int64     `json:"size"`
	DurationSec    int       `json:"duration_sec"`
	OwnerConsented bool      `json:"owner_consented"`
	// Stage 凭证阶段：start=开始服务 / finish=完成服务 / 空=过程记录
	Stage        string    `json:"stage"`
	UploaderRole string    `json:"uploader_role"`
	CreatedAt    time.Time `json:"created_at"`
	// 下载地址。图片/视频由原生组件加载，带不上 Authorization 头，
	// 因此地址里带一段短期票据（?t=...），由 sign 回调生成。
	URL string `json:"url"`
}

// ToRecordResps 批量转换。
// sign 为当前用户签发直链票据，返回空串则不附票据（仅留裸路径，调用方需自行带 header）。
func ToRecordResps(records []model.ServiceRecord, orderNo string, sign func(path string) string) []ToRecordResp {
	out := make([]ToRecordResp, 0, len(records))
	for _, r := range records {
		path := fmt.Sprintf("/api/v1/orders/%s/records/%d/file", orderNo, r.ID)
		if sign != nil {
			if t := sign(path); t != "" {
				path += "?t=" + t
			}
		}
		out = append(out, ToRecordResp{
			ID:             r.ID,
			MediaType:      r.MediaType,
			Size:           r.Size,
			DurationSec:    r.DurationSec,
			OwnerConsented: r.OwnerConsented,
			Stage:          r.Stage,
			UploaderRole:   r.UploaderRole,
			CreatedAt:      r.CreatedAt,
			URL:            path,
		})
	}
	return out
}
