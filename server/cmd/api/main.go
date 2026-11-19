// Command api 启动 HTTP 服务。
//
// @title           小区好帮手 API
// @version         1.1
// @description     社区生活服务预约平台后端接口：地址范围校验、下单支付、履约、运营后台。
// @description     用户端需 X-User-Id（接入微信登录后改为 Bearer JWT）；后台需 Bearer token。
// @BasePath        /
// @securityDefinitions.apikey UserID
// @in header
// @name X-User-Id
// @description 本地联调的用户标识
// @securityDefinitions.apikey AdminToken
// @in header
// @name Authorization
// @description 后台 token，格式：Bearer <token>
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alinger/community-handyman/server/internal/config"
	"github.com/alinger/community-handyman/server/internal/payment"
	"github.com/alinger/community-handyman/server/internal/repository"
	"github.com/alinger/community-handyman/server/internal/router"
	"github.com/alinger/community-handyman/server/internal/service"
	"github.com/alinger/community-handyman/server/internal/storage"
	"github.com/alinger/community-handyman/server/pkg/logger"

	// 生成的接口文档包，仅注册副作用
	_ "github.com/alinger/community-handyman/server/docs"
)

func main() {
	cfgPath := flag.String("config", "configs/config.dev.yaml", "配置文件路径")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		os.Exit(1)
	}

	log := logger.Init(cfg.App.Env, "info")

	store, err := storage.New(cfg.Storage.Provider, cfg.Storage.LocalDir)
	if err != nil {
		log.Error("存储初始化失败", "err", err)
		os.Exit(1)
	}

	gw, err := payment.New(cfg.Payment.Provider, cfg.Payment.SandboxSecret)
	if err != nil {
		log.Error("支付渠道初始化失败", "err", err)
		os.Exit(1)
	}

	log.Info("服务启动中",
		"app", cfg.App.Name, "env", cfg.App.Env, "dsn", cfg.MaskedDSN(),
		"payment", gw.Name(), "storage", store.Name())

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// 数据库连接失败在 dev 下只告警，便于在无库环境下验证探活与路由
	db, dbErr := repository.Open(ctx, cfg.Database.DSN,
		cfg.Database.MaxOpenConns, cfg.Database.MaxIdleConns,
		cfg.Database.ConnMaxLifetime, cfg.Database.LogSQL)
	if dbErr != nil {
		if cfg.App.Env == "dev" {
			log.Warn("数据库连接失败，dev 环境继续启动（业务接口不可用）", "err", dbErr)
			db = nil
		} else {
			log.Error("数据库连接失败，拒绝启动", "err", dbErr)
			os.Exit(1)
		}
	} else {
		log.Info("数据库连接成功")
	}

	engine, deps := router.New(cfg, db, gw, store)
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      engine,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// 超时关单任务：每 60 秒扫描一次，关闭未支付订单并释放库存
	if deps.Order != nil {
		go runCloseExpired(ctx, deps.Order, log)
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("开始监听", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		log.Error("监听失败", "err", err)
		os.Exit(1)
	case <-ctx.Done():
		log.Info("收到退出信号，开始优雅关闭")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("优雅关闭超时", "err", err)
		os.Exit(1)
	}
	slog.Info("服务已退出")
}

// runCloseExpired 定时关闭超时未支付订单。
func runCloseExpired(ctx context.Context, svc *service.OrderService, log *slog.Logger) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n, err := svc.CloseExpired(ctx, 100)
			if err != nil {
				log.Error("关闭超时订单失败", "err", err)
				continue
			}
			if n > 0 {
				log.Info("已关闭超时未支付订单", "count", n)
			}
		}
	}
}
