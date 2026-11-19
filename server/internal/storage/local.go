package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Local 把文件落在本地目录，用于尚未接入对象存储的阶段。
// 生产环境应换成对象存储：本地磁盘无法多副本，也不适合多实例部署。
type Local struct {
	root string // 绝对路径
}

// NewLocal 创建本地存储。目录不存在时自动创建。
func NewLocal(root string) (*Local, error) {
	if strings.TrimSpace(root) == "" {
		root = "./uploads"
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("解析存储目录失败: %w", err)
	}
	if err := os.MkdirAll(abs, 0o750); err != nil {
		return nil, fmt.Errorf("创建存储目录失败: %w", err)
	}
	return &Local{root: abs}, nil
}

// Name 实现标识。
func (l *Local) Name() string { return "local" }

// Root 返回根目录，便于运维排查。
func (l *Local) Root() string { return l.root }

// Save 写入文件。key 由调用方生成（服务端 UUID），不接受用户提供的文件名。
func (l *Local) Save(_ context.Context, key string, r io.Reader, size int64) (Object, error) {
	p, err := l.path(key)
	if err != nil {
		return Object{}, err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		return Object{}, fmt.Errorf("创建子目录失败: %w", err)
	}

	// 0600：服务记录含病历与门禁照片，仅进程自身可读
	f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return Object{}, fmt.Errorf("创建文件失败: %w", err)
	}
	defer f.Close()

	n, err := io.Copy(f, r)
	if err != nil {
		os.Remove(p) // 写失败不留下半截文件
		return Object{}, fmt.Errorf("写入文件失败: %w", err)
	}
	if size > 0 && n != size {
		os.Remove(p)
		return Object{}, fmt.Errorf("写入长度与声明不符: 期望 %d 实际 %d", size, n)
	}
	return Object{Key: key, Size: n}, nil
}

// Open 打开文件用于流式下载。
func (l *Local) Open(_ context.Context, key string) (io.ReadSeekCloser, int64, error) {
	p, err := l.path(key)
	if err != nil {
		return nil, 0, err
	}
	f, err := os.Open(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, 0, ErrNotFound
		}
		return nil, 0, fmt.Errorf("打开文件失败: %w", err)
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, 0, fmt.Errorf("读取文件信息失败: %w", err)
	}
	return f, st.Size(), nil
}

// path 把 key 归一化到 root 下，阻止 ../ 与绝对路径穿越。
func (l *Local) path(key string) (string, error) {
	clean := filepath.Clean("/" + strings.ReplaceAll(key, "\\", "/"))
	if clean == "/" {
		return "", fmt.Errorf("存储键不能为空")
	}
	p := filepath.Join(l.root, clean)
	if p != l.root && !strings.HasPrefix(p, l.root+string(os.PathSeparator)) {
		return "", fmt.Errorf("非法存储键: %s", key)
	}
	return p, nil
}
