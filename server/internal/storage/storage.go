// Package storage 封装文件存储。
// 业务层只依赖 Storage 接口：现在落本地目录，接入对象存储后换实现即可，
// service 代码不需要改动。
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
)

// ErrNotFound 表示文件不存在。
var ErrNotFound = errors.New("file not found")

// Object 是保存结果。
type Object struct {
	Key  string // 存储键，相对路径或对象键，落库用
	Size int64
}

// Storage 是存储抽象。
// 本地实现把文件写到磁盘；对象存储实现把文件写到 bucket。
// 两者的差异（直传、签名 URL）通过可选扩展接口暴露，见 Presigner。
type Storage interface {
	// Name 返回实现标识，用于日志与前端选择上传方式。
	Name() string
	// Save 保存文件。key 由服务端生成，不接受客户端文件名，避免目录穿越。
	Save(ctx context.Context, key string, r io.Reader, size int64) (Object, error)
	// Open 打开文件用于流式下载，返回内容与总长度。
	// 返回 ErrNotFound 时应映射为 404。
	Open(ctx context.Context, key string) (io.ReadSeekCloser, int64, error)
}

// Presigner 是可选的直传能力。
// 本地实现不支持，前端会回退到服务端上传；对象存储实现返回临时凭证。
type Presigner interface {
	Presign(ctx context.Context, key string, contentType string, expireSeconds int64) (map[string]string, error)
}

// New 按配置创建存储实现。
func New(provider, localDir string) (Storage, error) {
	switch provider {
	case "", "local":
		return NewLocal(localDir)
	case "oss", "cos", "s3":
		return nil, fmt.Errorf("对象存储 %s 尚未接入：需配置 endpoint、bucket 与访问密钥后实现该分支", provider)
	default:
		return nil, fmt.Errorf("未知存储类型: %s", provider)
	}
}
