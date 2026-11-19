// Command migrate 执行数据库迁移与初始账号创建。
// 用法：
//
//	go run ./cmd/migrate -cmd up
//	go run ./cmd/migrate -cmd down -steps 1
//	go run ./cmd/migrate -cmd version
//	go run ./cmd/migrate -cmd force -force 2
//	go run ./cmd/migrate -cmd seed-admin -username admin
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"golang.org/x/crypto/bcrypt"

	"github.com/alinger/community-handyman/server/internal/config"
	"github.com/alinger/community-handyman/server/internal/model"
	"github.com/alinger/community-handyman/server/internal/repository"
)

func main() {
	cfgPath := flag.String("config", "configs/config.yaml", "配置文件路径")
	dir := flag.String("dir", "migrations", "迁移文件目录")
	cmd := flag.String("cmd", "up", "up | down | version | force | seed-admin")
	steps := flag.Int("steps", 1, "down 时回滚的步数")
	forceVersion := flag.Int("force", -1, "force 时指定的版本号")
	username := flag.String("username", "admin", "seed-admin 的后台账号")
	password := flag.String("password", "", "seed-admin 的密码，留空则读 ADMIN_PASSWORD")
	role := flag.String("role", model.AdminRoleAdmin, "seed-admin 的角色：admin | operator")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		fail("加载配置失败: %v", err)
	}

	switch *cmd {
	case "up":
		runUp(*dir, cfg.Database.DSN)
	case "down":
		runDown(*dir, cfg.Database.DSN, *steps)
	case "version":
		showVersion(*dir, cfg.Database.DSN)
	case "force":
		if *forceVersion < 0 {
			fail("force 需要同时指定 -force <版本号>")
		}
		runForce(*dir, cfg.Database.DSN, *forceVersion)
	case "seed-admin":
		runSeedAdmin(cfg, *username, *password, *role)
	default:
		fail("未知的子命令: %s", *cmd)
	}
}

func newMigrator(dir, gormDSN string) *migrate.Migrate {
	m, err := migrate.New("file://"+dir, normalizeDSN(gormDSN))
	if err != nil {
		fail("初始化迁移器失败: %v", err)
	}
	return m
}

func runUp(dir, dsn string) {
	m := newMigrator(dir, dsn)
	defer closeMigrator(m)

	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			fmt.Println("已是最新版本，无需迁移")
			return
		}
		fail("迁移失败: %v", err)
	}
	v, dirty, err := m.Version()
	if err != nil {
		fail("读取版本失败: %v", err)
	}
	fmt.Printf("迁移完成，当前版本 %d，dirty=%v\n", v, dirty)
}

func runDown(dir, dsn string, steps int) {
	if steps <= 0 {
		fail("steps 必须大于 0，整库回滚请显式执行 down 全部步数")
	}
	m := newMigrator(dir, dsn)
	defer closeMigrator(m)

	if err := m.Steps(-steps); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			fmt.Println("没有可回滚的版本")
			return
		}
		fail("回滚失败: %v", err)
	}
	v, dirty, _ := m.Version()
	fmt.Printf("回滚 %d 步完成，当前版本 %d，dirty=%v\n", steps, v, dirty)
}

func showVersion(dir, dsn string) {
	m := newMigrator(dir, dsn)
	defer closeMigrator(m)

	v, dirty, err := m.Version()
	if err != nil {
		if errors.Is(err, migrate.ErrNilVersion) {
			fmt.Println("尚未执行任何迁移")
			return
		}
		fail("读取版本失败: %v", err)
	}
	fmt.Printf("当前版本 %d，dirty=%v\n", v, dirty)
}

func runForce(dir, dsn string, version int) {
	m := newMigrator(dir, dsn)
	defer closeMigrator(m)

	if err := m.Force(version); err != nil {
		fail("force 失败: %v", err)
	}
	fmt.Printf("版本已强制置为 %d\n", version)
}

// runSeedAdmin 创建后台账号。密码只从参数或环境变量读，不落版本库。
func runSeedAdmin(cfg *config.Config, username, password, role string) {
	if password == "" {
		password = os.Getenv("ADMIN_PASSWORD")
	}
	if password == "" {
		fail("缺少密码：用 -password 或环境变量 ADMIN_PASSWORD 提供")
	}
	if len(password) < 8 {
		fail("密码至少 8 位")
	}
	if role != model.AdminRoleAdmin && role != model.AdminRoleOperator {
		fail("角色只能是 admin 或 operator")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		fail("生成密码哈希失败: %v", err)
	}

	db, err := repository.Open(context.Background(), cfg.Database.DSN,
		cfg.Database.MaxOpenConns, cfg.Database.MaxIdleConns,
		cfg.Database.ConnMaxLifetime, cfg.Database.LogSQL)
	if err != nil {
		fail("连接数据库失败: %v", err)
	}

	admin := model.AdminUser{Username: username, PasswordHash: string(hash), Role: role, Status: 1}
	if err := db.Create(&admin).Error; err != nil {
		fail("创建账号失败（用户名可能已存在）: %v", err)
	}
	fmt.Printf("后台账号已创建：%s（角色 %s）\n", username, role)
}

// normalizeDSN 把 GORM 的 DSN 转成 golang-migrate mysql driver 需要的 URL 形式。
// root:pwd@tcp(127.0.0.1:3306)/db?charset=utf8mb4 -> mysql://root:pwd@tcp(127.0.0.1:3306)/db
func normalizeDSN(dsn string) string {
	if strings.HasPrefix(dsn, "mysql://") {
		return dsn
	}
	if idx := strings.Index(dsn, "?"); idx >= 0 {
		dsn = dsn[:idx]
	}
	return "mysql://" + dsn
}

func closeMigrator(m *migrate.Migrate) {
	srcErr, dbErr := m.Close()
	if srcErr != nil {
		fmt.Fprintf(os.Stderr, "关闭迁移源失败: %v\n", srcErr)
	}
	if dbErr != nil {
		fmt.Fprintf(os.Stderr, "关闭迁移数据库失败: %v\n", dbErr)
	}
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
