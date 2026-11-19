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

// 证件照上传限制。证件照只在审核时看，不需要大图。
const maxCertImageSize = 8 << 20 // 8MB

// 允许的服务类型。all 表示三类都接。
var validServiceTypes = map[string]bool{
	"garbage": true, "dogwalk": true, "accompany": true, "all": true,
}

// ProviderAppService 服务方资质申请与审核。
//
// 权限门禁原则：users.role 是唯一权限依据，只有在审核通过时才改；
// 申请表本身不参与鉴权判断，避免「申请记录状态」与「实际权限」不一致。
type ProviderAppService struct {
	apps  *repository.ProviderAppRepo
	users *repository.UserRepo
	store storage.Storage
}

func NewProviderAppService(apps *repository.ProviderAppRepo, users *repository.UserRepo, store storage.Storage) *ProviderAppService {
	return &ProviderAppService{apps: apps, users: users, store: store}
}

// ApplyInput 提交申请的入参。
type ApplyInput struct {
	RealName    string
	Phone       string
	ServiceType string
	Experience  string
	IDCardNo    string
	// 附件 object key。为空表示本次不更新该附件（重提时保留原图）。
	IDCardFrontKey string
	IDCardBackKey  string
	HealthCertKey  string
}

// Submit 提交或重新提交申请。
//
// 已通过审核的用户不允许再提交：role 已经是 provider，
// 再改资料应该走「资料变更」，那是另一套流程，不能混在认证申请里。
func (s *ProviderAppService) Submit(ctx context.Context, userID uint64, in ApplyInput) (*model.ProviderApplication, error) {
	in.RealName = strings.TrimSpace(in.RealName)
	in.Phone = strings.TrimSpace(in.Phone)
	in.Experience = strings.TrimSpace(in.Experience)
	in.IDCardNo = strings.ToUpper(strings.TrimSpace(in.IDCardNo))

	if in.RealName == "" {
		return nil, errs.BizMsg(errs.CodeInvalidParam, "请填写真实姓名")
	}
	if !phoneRe.MatchString(in.Phone) {
		return nil, errs.BizMsg(errs.CodeInvalidParam, "手机号格式不正确")
	}
	if !validServiceTypes[in.ServiceType] {
		return nil, errs.BizMsg(errs.CodeInvalidParam, "请选择要承接的服务类型")
	}
	if in.Experience == "" {
		return nil, errs.BizMsg(errs.CodeInvalidParam, "请填写相关经验说明")
	}

	exist, err := s.apps.GetByUser(ctx, userID)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	if exist != nil && exist.Status == model.AppApproved {
		return nil, errs.BizMsg(errs.CodeInvalidParam, "已通过认证，无需重复申请")
	}
	if exist != nil && exist.Status == model.AppPending {
		return nil, errs.BizMsg(errs.CodeInvalidParam, "申请已提交，请等待审核")
	}

	app := &model.ProviderApplication{
		UserID:      userID,
		RealName:    in.RealName,
		Phone:       in.Phone,
		ServiceType: in.ServiceType,
		Experience:  in.Experience,
		IDCardNo:    in.IDCardNo,
		Status:      model.AppPending,
	}
	// 重提时未重新上传的附件沿用旧值，避免用户每改一个字都要重传证件照
	if exist != nil {
		app.IDCardFrontKey = exist.IDCardFrontKey
		app.IDCardBackKey = exist.IDCardBackKey
		app.HealthCertKey = exist.HealthCertKey
	}
	if in.IDCardFrontKey != "" {
		app.IDCardFrontKey = in.IDCardFrontKey
	}
	if in.IDCardBackKey != "" {
		app.IDCardBackKey = in.IDCardBackKey
	}
	if in.HealthCertKey != "" {
		app.HealthCertKey = in.HealthCertKey
	}

	if app.IDCardFrontKey == "" || app.IDCardBackKey == "" {
		return nil, errs.BizMsg(errs.CodeInvalidParam, "请上传身份证正反面照片")
	}

	if err := s.apps.Save(ctx, app); err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	return s.apps.GetByUser(ctx, userID)
}

// MyApplication 查当前用户的申请状态。没有记录时返回 nil。
func (s *ProviderAppService) MyApplication(ctx context.Context, userID uint64) (*model.ProviderApplication, error) {
	a, err := s.apps.GetByUser(ctx, userID)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	return a, nil
}

// SaveCertImage 保存一张证件照，返回 object key。
// kind 取 front / back / health，用于分层目录与后缀校验。
func (s *ProviderAppService) SaveCertImage(ctx context.Context, userID uint64, kind, filename string,
	r io.Reader, size int64) (string, error) {

	if kind != "front" && kind != "back" && kind != "health" {
		return "", errs.BizMsg(errs.CodeInvalidParam, "附件类型不合法")
	}
	if size <= 0 {
		return "", errs.BizMsg(errs.CodeInvalidParam, "文件为空")
	}
	if size > maxCertImageSize {
		return "", errs.BizMsg(errs.CodeInvalidParam, "图片不能超过 8MB")
	}
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp":
	default:
		return "", errs.BizMsg(errs.CodeInvalidParam, "仅支持 jpg/png/webp 图片")
	}

	key := fmt.Sprintf("provider-certs/%d/%s-%s%s", userID, kind, util.NewRequestID(), ext)
	if _, err := s.store.Save(ctx, key, r, size); err != nil {
		return "", errs.Wrap(errs.CodeInternal, err)
	}
	return key, nil
}

// OpenCertImage 读取证件照，仅申请人本人与后台审核可访问。
// 附件不暴露公网直链，避免身份证照片被转发。
func (s *ProviderAppService) OpenCertImage(ctx context.Context, appID uint64, kind string,
	requesterID uint64, isAdmin bool) (io.ReadSeekCloser, int64, string, error) {

	app, err := s.apps.GetByID(ctx, appID)
	if err != nil {
		return nil, 0, "", errs.Wrap(errs.CodeInternal, err)
	}
	if app == nil {
		return nil, 0, "", errs.Biz(errs.CodeNotFound)
	}
	if !isAdmin && app.UserID != requesterID {
		return nil, 0, "", errs.Biz(errs.CodeForbidden)
	}

	var key string
	switch kind {
	case "front":
		key = app.IDCardFrontKey
	case "back":
		key = app.IDCardBackKey
	case "health":
		key = app.HealthCertKey
	default:
		return nil, 0, "", errs.BizMsg(errs.CodeInvalidParam, "附件类型不合法")
	}
	if key == "" {
		return nil, 0, "", errs.Biz(errs.CodeNotFound)
	}

	f, size, err := s.store.Open(ctx, key)
	if err != nil {
		if err == storage.ErrNotFound {
			return nil, 0, "", errs.Biz(errs.CodeNotFound)
		}
		return nil, 0, "", errs.Wrap(errs.CodeInternal, err)
	}
	ct := "image/jpeg"
	if strings.HasSuffix(strings.ToLower(key), ".png") {
		ct = "image/png"
	} else if strings.HasSuffix(strings.ToLower(key), ".webp") {
		ct = "image/webp"
	}
	return f, size, ct, nil
}

// ---------- 后台审核 ----------

// AdminList 后台列表，附带申请人昵称。
type AdminAppRow struct {
	ID          uint64 `json:"id"`
	UserID      uint64 `json:"user_id"`
	UserNick    string `json:"user_nickname"`
	RealName    string `json:"real_name"`
	Phone       string `json:"phone"`
	ServiceType string `json:"service_type"`
	Experience  string `json:"experience"`
	IDCardNo    string `json:"id_card_no"`
	Status      string `json:"status"`
	StatusLabel string `json:"status_label"`
	ReviewNote  string `json:"review_note"`
	ReviewedAt  string `json:"reviewed_at"`
	CreatedAt   string `json:"created_at"`
	HasFront    bool   `json:"has_front"`
	HasBack     bool   `json:"has_back"`
	HasHealth   bool   `json:"has_health"`
}

// List 后台分页列表。昵称用一次 IN 查询批量回填，避免 N+1。
func (s *ProviderAppService) List(ctx context.Context, status string, page, size int) ([]AdminAppRow, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	list, total, err := s.apps.List(ctx, status, page, size)
	if err != nil {
		return nil, 0, errs.Wrap(errs.CodeInternal, err)
	}
	ids := make([]uint64, 0, len(list))
	for _, a := range list {
		ids = append(ids, a.UserID)
	}
	nick := map[uint64]string{}
	if users, err := s.users.ListByIDs(ctx, ids); err == nil {
		for _, u := range users {
			n := u.Nickname
			if n == "" {
				n = fmt.Sprintf("用户%d", u.ID)
			}
			nick[u.ID] = n
		}
	}

	out := make([]AdminAppRow, 0, len(list))
	for _, a := range list {
		out = append(out, s.toRow(&a, nick[a.UserID]))
	}
	return out, total, nil
}

// Stats 各状态数量。
func (s *ProviderAppService) Stats(ctx context.Context) (map[string]int64, error) {
	m, err := s.apps.CountByStatus(ctx)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	return m, nil
}

// Detail 后台查看单条（含完整身份证号）。
func (s *ProviderAppService) Detail(ctx context.Context, id uint64) (*AdminAppRow, error) {
	a, err := s.apps.GetByID(ctx, id)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err)
	}
	if a == nil {
		return nil, errs.Biz(errs.CodeNotFound)
	}
	u, _ := s.users.GetByID(ctx, a.UserID)
	name := "用户"
	if u != nil && u.Nickname != "" {
		name = u.Nickname
	}
	row := s.toRow(a, name)
	return &row, nil
}

// Approve 审核通过：写审核结果并把 users.role 置为 provider。
// 两件事必须在同一事务里，否则会出现「审核通过但没权限」或反之的半截状态。
func (s *ProviderAppService) Approve(ctx context.Context, id, reviewerID uint64) error {
	a, err := s.apps.GetByID(ctx, id)
	if err != nil {
		return errs.Wrap(errs.CodeInternal, err)
	}
	if a == nil {
		return errs.Biz(errs.CodeNotFound)
	}
	if a.Status != model.AppPending {
		return errs.BizMsg(errs.CodeIllegalStatus, "该申请已处理，请刷新列表")
	}
	if err := s.apps.ReviewAndGrant(ctx, id, a.UserID, reviewerID, time.Now()); err != nil {
		return errs.Wrap(errs.CodeInternal, err)
	}
	return nil
}

// Reject 驳回申请，必须填原因。
func (s *ProviderAppService) Reject(ctx context.Context, id, reviewerID uint64, note string) error {
	note = strings.TrimSpace(note)
	if note == "" {
		return errs.BizMsg(errs.CodeInvalidParam, "请填写驳回原因")
	}
	a, err := s.apps.GetByID(ctx, id)
	if err != nil {
		return errs.Wrap(errs.CodeInternal, err)
	}
	if a == nil {
		return errs.Biz(errs.CodeNotFound)
	}
	if a.Status != model.AppPending {
		return errs.BizMsg(errs.CodeIllegalStatus, "该申请已处理，请刷新列表")
	}
	if err := s.apps.Review(ctx, id, model.AppRejected, reviewerID, note, time.Now()); err != nil {
		return errs.Wrap(errs.CodeInternal, err)
	}
	return nil
}

func (s *ProviderAppService) toRow(a *model.ProviderApplication, nick string) AdminAppRow {
	reviewed := ""
	if a.ReviewedAt != nil {
		reviewed = a.ReviewedAt.Format("2006-01-02 15:04")
	}
	return AdminAppRow{
		ID:          a.ID,
		UserID:      a.UserID,
		UserNick:    nick,
		RealName:    a.RealName,
		Phone:       a.Phone,
		ServiceType: a.ServiceType,
		Experience:  a.Experience,
		IDCardNo:    a.IDCardNo,
		Status:      string(a.Status),
		StatusLabel: a.Status.Label(),
		ReviewNote:  a.ReviewNote,
		ReviewedAt:  reviewed,
		CreatedAt:   a.CreatedAt.Format("2006-01-02 15:04"),
		HasFront:    a.IDCardFrontKey != "",
		HasBack:     a.IDCardBackKey != "",
		HasHealth:   a.HealthCertKey != "",
	}
}
