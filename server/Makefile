# 本文件不含任何凭据。数据库连接由 .env 提供（程序启动自动加载）。
# 需要临时切换库时在命令行前置环境变量即可，例如：
#   MYSQL_DSN='root:pwd@tcp(127.0.0.1:3306)/other_db?...' make migrate-up

SWAG ?= $(shell go env GOPATH)/bin/swag

.PHONY: help run build tidy fmt vet doc migrate-up migrate-down migrate-version seed-admin

help:
	@echo "make run                   启动 HTTP 服务"
	@echo "make build                 编译到 bin/"
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
