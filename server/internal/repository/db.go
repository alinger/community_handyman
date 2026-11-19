// Package repository 是数据访问层。
// 约定：不向上层返回 *gorm.DB，避免事务控制权散落。
package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
)

// Open 建立数据库连接并校验可用性。
func Open(ctx context.Context, dsn string, maxOpen, maxIdle int, lifetime time.Duration, logSQL bool) (*gorm.DB, error) {
	level := gormLogger.Warn
	if logSQL {
		level = gormLogger.Info
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: gormLogger.Default.LogMode(level),
		// 表名由结构体名推导为复数蛇形，与迁移文件保持一致
		NamingStrategy: nil,
	})
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("获取底层连接池失败: %w", err)
	}
	sqlDB.SetMaxOpenConns(maxOpen)
	sqlDB.SetMaxIdleConns(maxIdle)
	sqlDB.SetConnMaxLifetime(lifetime)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(pingCtx); err != nil {
		return nil, fmt.Errorf("数据库不可达: %w", err)
	}

	return db, nil
}
