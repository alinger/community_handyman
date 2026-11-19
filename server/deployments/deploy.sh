#!/usr/bin/env bash
# 一键发布：拉代码 → 编译 → 迁移 → reload。
# 前提：服务器已完成 init-server.sh 初始化，且 /opt/community-handyman/.env 已配好。
set -euo pipefail

APP_DIR="${APP_DIR:-/opt/community-handyman}"
BRANCH="${BRANCH:-main}"

cd "$APP_DIR"

echo "==> 拉取代码 ($BRANCH)"
git fetch origin "$BRANCH"
git checkout "$BRANCH"
git pull --ff-only origin "$BRANCH"

echo "==> 编译"
cd "$APP_DIR/server"
mkdir -p bin logs
make build

echo "==> 执行迁移"
# 迁移可重复执行，已是最新版本时输出「无需迁移」
make migrate-up CONFIG=configs/config.prod.yaml

echo "==> reload 服务"
# reload 而非 restart：先起新进程再切流量，停机时间更短；
# 单实例场景下 reload 仍是先停后起，但能避免 pm2 的进程表错乱。
pm2 startOrReload deployments/ecosystem.config.js --update-env
pm2 save

echo "==> 健康检查"
sleep 2
if curl -fsS http://127.0.0.1:8080/healthz >/dev/null; then
  echo "发布成功：/healthz 正常"
else
  echo "发布后健康检查失败，请查看 pm2 logs community-handyman-api" >&2
  exit 1
fi
