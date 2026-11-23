package service

import (
	"context"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/alinger/community-handyman/server/internal/errs"
	"github.com/alinger/community-handyman/server/internal/repository"
	"github.com/alinger/community-handyman/server/internal/storage"
)

// 头像上限。小程序 chooseAvatar 给的临时图多在几十 KB，
// 5MB 足够覆盖用户从相册选的大图，也避免被当成图床。
const maxAvatarBytes = 5 << 20

// 头像地址形如 /api/v1/users/12/avatar/1732000000123.png
// 只存路径不存域名：换域名、多环境切换时不用改库里的数据。
const avatarURLFormat = "/api/v1/users/%d/avatar/%s"

// 允许的图片扩展名。微信临时文件通常没有扩展名或带 .jpg/.png，
// 缺省按 .jpg 处理（微信统一转码为 jpg）。
var avatarExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".gif": true,
}

// AvatarService 头像上传与读取。
//
// 头像是唯一需要「公开可读」的用户文件：小程序 <image> 标签没法带
// Authorization 头，走鉴权接口会 401。因此头像用不可猜的文件名 + 公开路由，
// 与需要鉴权的服务记录分开处理。
type AvatarService struct {
	users *repository.UserRepo
	store storage.Storage
}

func NewAvatarService(users *repository.UserRepo, store storage.Storage) *AvatarService {
	return &AvatarService{users: users, store: store}
}

// Save 保存头像并把地址写回用户资料，返回可访问的相对地址。
func (s *AvatarService) Save(ctx context.Context, userID uint64, filename string,
	r io.Reader, size int64) (string, error) {
	if size <= 0 {
		return "", errs.BizMsg(errs.CodeInvalidParam, "头像文件为空")
	}
	if size > maxAvatarBytes {
		return "", errs.BizMsg(errs.CodeInvalidParam, "头像不能超过 5MB")
	}

	ext := strings.ToLower(path.Ext(filename))
	if ext == "" {
		ext = ".jpg"
	}
	if !avatarExts[ext] {
		return "", errs.BizMsg(errs.CodeInvalidParam, "头像仅支持 jpg / png / webp / gif")
	}

	// 用毫秒时间戳命名：同一用户重复上传不会互相覆盖，也天然带缓存失效效果
	name := strconv.FormatInt(time.Now().UnixMilli(), 10) + ext
	key := fmt.Sprintf("avatars/%d/%s", userID, name)

	if _, err := s.store.Save(ctx, key, r, size); err != nil {
		return "", errs.Wrap(errs.CodeInternal, err)
	}

	url := fmt.Sprintf(avatarURLFormat, userID, name)
	if err := s.users.UpdateAvatar(ctx, userID, url); err != nil {
		return "", errs.Wrap(errs.CodeInternal, err)
	}
	return url, nil
}

// Open 读取头像文件。name 只允许「数字 + 合法扩展名」，防止路径穿越。
func (s *AvatarService) Open(ctx context.Context, userID uint64, name string) (io.ReadSeekCloser, int64, error) {
	if userID == 0 || !validAvatarName(name) {
		return nil, 0, errs.Biz(errs.CodeNotFound)
	}
	f, size, err := s.store.Open(ctx, fmt.Sprintf("avatars/%d/%s", userID, name))
	if err != nil {
		return nil, 0, errs.Biz(errs.CodeNotFound)
	}
	return f, size, nil
}

// validAvatarName 校验文件名：base 必须与原名一致（挡掉 ../ 与子路径），
// 主干是纯数字，扩展名在白名单内。
func validAvatarName(name string) bool {
	if name == "" || path.Base(name) != name {
		return false
	}
	ext := strings.ToLower(path.Ext(name))
	if !avatarExts[ext] {
		return false
	}
	stem := strings.TrimSuffix(name, path.Ext(name))
	if stem == "" {
		return false
	}
	for _, ch := range stem {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}
