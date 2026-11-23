# 本文件不含任何凭据。数据库连接由 .env 提供（程序启动自动加载）。
# 需要临时切换库时在命令行前置环境变量即可，例如：
#   MYSQL_DSN='root:pwd@tcp(127.0.0.1:3306)/other_db?...' make migrate-up

SWAG ?= $(shell go env GOPATH)/bin/swag

.PHONY: help run build build-linux tidy fmt vet doc migrate-up migrate-down migrate-version seed-admin dev-restart e2e

# E2E 测试用独立库与独立端口，不碰开发库
E2E_DB ?= community_handyman_e2e
E2E_PORT ?= 8098
E2E_ADMIN_PWD ?= E2eTest@123

help:
	@echo "make run                   启动 HTTP 服务"
	@echo "make dev-restart           重新编译 + 迁移 + 重启本地后台服务（日志 logs/api.out）"
	@echo "make e2e                   端到端测试（自动建库/迁移/起服务/跑用例）"
	@echo "make build                 编译到 bin/"
	@echo "make build-linux           交叉编译 Linux 版（ARCH=amd64 默认，可传 arm64）"
	@echo "make migrate-up            执行迁移（读 .env）"
	@echo "make migrate-down steps=1  回滚 N 步"
	@echo "make migrate-version       查看当前版本"
	@echo "make seed-admin user=admin 创建后台账号（密码读 .env 的 ADMIN_PASSWORD）"
	@echo "make doc                   重新生成接口文档（需 swag CLI）"

# 本地默认用 dev 配置；指定 CONFIG=configs/config.prod.yaml 可验证生产配置
CONFIG ?= configs/config.dev.yaml

run:
	go run ./cmd/api -config $(CONFIG)

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o bin/api ./cmd/api
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o bin/migrate ./cmd/migrate

# 交叉编译 Linux 版，产物带架构后缀，避免和本机二进制互相覆盖。
# CGO_ENABLED=0 是前提：本项目不依赖 cgo，关掉后才是真正的静态单文件，服务器不用装 glibc 版本对应的库。
ARCH ?= amd64
build-linux:
	GOOS=linux GOARCH=$(ARCH) CGO_ENABLED=0 \
		go build -trimpath -ldflags "-s -w" -o bin/api-linux-$(ARCH) ./cmd/api
	GOOS=linux GOARCH=$(ARCH) CGO_ENABLED=0 \
		go build -trimpath -ldflags "-s -w" -o bin/migrate-linux-$(ARCH) ./cmd/migrate

tidy:
	go mod tidy

fmt:
	gofmt -w .

vet:
	go vet ./...

# 生成 OpenAPI：改动注解后执行，产物在 docs/
doc:
	$(SWAG) init -g cmd/api/main.go -o docs --parseDependency --parseInternal
	@echo "文档已生成：http://127.0.0.1:8080/swagger/index.html"

migrate-up:
	go run ./cmd/migrate -config $(CONFIG) -cmd up

migrate-down:
	go run ./cmd/migrate -config $(CONFIG) -cmd down -steps $(steps)

migrate-version:
	go run ./cmd/migrate -config $(CONFIG) -cmd version

seed-admin:
	go run ./cmd/migrate -config $(CONFIG) -cmd seed-admin -username $(user)

# 本地开发一条龙：改完后端 Go 代码后一条命令让本地服务生效。
# 顺序有讲究：先迁移再重启，否则新代码 INSERT 新列会因列不存在直接报错。
# 端口取自 dev 配置（8080），换配置时用 PORT=xxx 覆盖。
PORT ?= 8080
LOG_FILE ?= logs/api.out

dev-restart:
	go build -trimpath -ldflags "-s -w" -o bin/api ./cmd/api
	go run ./cmd/migrate -config $(CONFIG) -cmd up
	@old=$$(lsof -tiTCP:$(PORT) -sTCP:LISTEN 2>/dev/null); \
		if [ -n "$$old" ]; then kill $$old && sleep 1; fi; \
		nohup ./bin/api -config $(CONFIG) > $(LOG_FILE) 2>&1 & \
		for i in 1 2 3 4 5 6 7 8 9 10; do \
			sleep 0.5; \
			curl -sf http://127.0.0.1:$(PORT)/healthz > /dev/null 2>&1 && break; \
		done; \
		if curl -sf http://127.0.0.1:$(PORT)/healthz > /dev/null 2>&1; then \
			new=$$(lsof -tiTCP:$(PORT) -sTCP:LISTEN 2>/dev/null | head -1); \
			echo "✅ dev-restart OK  PID=$$new  LOG=$$PWD/$(LOG_FILE)"; \
		else \
			echo "❌ 启动失败，最近日志："; \
			tail -5 $(LOG_FILE); \
			exit 1; \
		fi

# 端到端测试：逻辑都在 scripts/run_e2e.sh，这里只做入口
e2e:
	@bash scripts/run_e2e.sh
