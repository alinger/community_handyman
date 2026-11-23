package auth

import (
	"crypto/hmac"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"
)

// MediaTicketTTL 是媒体直链票据的有效期。
//
// 服务记录的照片/视频由 <image>、<video>、uni.previewImage 这类原生组件加载，
// 它们无法携带 Authorization 头，凭证只能放进 URL；而 URL 会进访问日志、
// 也可能被用户转发，所以必须短时效。30 分钟足够走完「进详情页 → 看照片 → 播视频」，
// 页面重新拉列表时会换一张新票。
const MediaTicketTTL = 30 * time.Minute

// ErrInvalidTicket 表示票据格式错误、签名不匹配或已过期。
var ErrInvalidTicket = errors.New("票据无效或已过期")

// MediaTicket 绑定「谁 + 哪个文件 + 何时过期」。
//
// 绑定路径是关键：否则拿到任意一张票就能遍历其它记录 ID 逐个读文件。
type MediaTicket struct {
	UID  uint64
	Path string
	Exp  int64
}

// SignMediaTicket 为单个资源路径签发票据。
//
// 与登录 token 分开实现而不是复用 Claims：票据是只读的、绑路径的，
// 万一泄露危害远小于登录态，不能出现「拿票据当 token 调接口」的可能。
func SignMediaTicket(secret string, uid uint64, path string, ttl time.Duration) string {
	if ttl <= 0 {
		ttl = MediaTicketTTL
	}
	exp := time.Now().Add(ttl).Unix()
	// 顺序固定为 exp|uid|path，且 path 放最后：路径里万一带 | 也不会被拆错
	raw := strconv.FormatInt(exp, 10) + "|" + strconv.FormatUint(uid, 10) + "|" + path
	body := base64.RawURLEncoding.EncodeToString([]byte(raw))
	return body + "." + sign(secret, body)
}

// ParseMediaTicket 校验票据。调用方还须确认 Path 与当前请求路径一致，
// 校验放在中间件里做，避免每个 handler 各写一遍漏掉。
func ParseMediaTicket(secret, ticket string) (MediaTicket, error) {
	parts := strings.Split(ticket, ".")
	if len(parts) != 2 {
		return MediaTicket{}, ErrInvalidTicket
	}
	body, sig := parts[0], parts[1]
	if !hmac.Equal([]byte(sign(secret, body)), []byte(sig)) {
		return MediaTicket{}, ErrInvalidTicket
	}

	raw, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return MediaTicket{}, ErrInvalidTicket
	}
	// exp|uid|path，path 在最后可含 |
	segs := strings.SplitN(string(raw), "|", 3)
	if len(segs) != 3 {
		return MediaTicket{}, ErrInvalidTicket
	}
	exp, errExp := strconv.ParseInt(segs[0], 10, 64)
	uid, errUID := strconv.ParseUint(segs[1], 10, 64)
	if errUID != nil || errExp != nil || uid == 0 {
		return MediaTicket{}, ErrInvalidTicket
	}
	if time.Now().Unix() > exp {
		return MediaTicket{}, ErrInvalidTicket
	}
	return MediaTicket{UID: uid, Path: segs[2], Exp: exp}, nil
}
