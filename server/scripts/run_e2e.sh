#!/usr/bin/env bash
# 端到端测试编排：独立库 + 独立端口 + console 短信渠道，全程不碰开发库与真实短信。
#
# 用法：bash scripts/run_e2e.sh   或   make e2e
#
# 设计要点：
# - 每次重建测试库，保证用例从干净状态开始，可无限重跑；
# - 端口默认 8098，避免和 make run 的 8080 抢；
# - SMS_PROVIDER=console 让验证码只进服务日志，绝不发真实短信；
# - 任何一步失败立即退出（set -e），并保证临时服务被清理。

set -euo pipefail

cd "$(dirname "$0")/.."

E2E_DB="${E2E_DB:-community_handyman_e2e}"
E2E_PORT="${E2E_PORT:-8098}"
E2E_ADMIN_PWD="${E2E_ADMIN_PWD:-E2eTest@123}"
LOG=/tmp/e2e-server.log

# 从 .env 取 MySQL 密码，保持与本机开发环境一致
ENV_FILE=".env"
[ -f "$ENV_FILE" ] || { echo "缺少 .env，请先按 README 配置数据库连接"; exit 1; }
MYSQL_PWD_VALUE="$(grep -E '^MYSQL_DSN=' "$ENV_FILE" | head -1 \
  | sed -E 's#^MYSQL_DSN=root:([^@]*)@.*#\1#')"
[ -n "$MYSQL_PWD_VALUE" ] || { echo "无法从 .env 解析 MySQL 密码"; exit 1; }

DSN="root:${MYSQL_PWD_VALUE}@tcp(127.0.0.1:3306)/${E2E_DB}?charset=utf8mb4&parseTime=True&loc=Local"
MYSQL() { mysql -uroot -p"${MYSQL_PWD_VALUE}" -h127.0.0.1 "$@" 2>/dev/null; }

cleanup() {
  if [ -f /tmp/e2e-server.pid ]; then
    kill "$(cat /tmp/e2e-server.pid)" 2>/dev/null || true
    rm -f /tmp/e2e-server.pid
  fi
  # 兜底：按端口再清一次，避免子进程或异常退出留下监听
  for p in $(lsof -nP -iTCP:"${E2E_PORT}" -sTCP:LISTEN -t 2>/dev/null); do
    kill "$p" 2>/dev/null || true
  done
  rm -f /tmp/e2e-api
}
trap cleanup EXIT

echo "==> 重建测试库 ${E2E_DB}"
MYSQL -e "DROP DATABASE IF EXISTS ${E2E_DB};
          CREATE DATABASE ${E2E_DB} CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"

echo "==> 执行迁移"
MYSQL_DSN="$DSN" go run ./cmd/migrate -config configs/config.dev.yaml -cmd up >/dev/null
MYSQL_DSN="$DSN" go run ./cmd/migrate -config configs/config.dev.yaml \
  -cmd seed-admin -username admin -password "$E2E_ADMIN_PWD" >/dev/null

# 端口必须干净：若旧实例还在，新实例会 bind 失败，
# 而 healthz 仍由旧实例应答，测试就会打到一个连着旧库的进程上，结果全是假象。
if lsof -nP -iTCP:"${E2E_PORT}" -sTCP:LISTEN -t >/dev/null 2>&1; then
  echo "端口 ${E2E_PORT} 已被占用，请先停掉占用进程："
  lsof -nP -iTCP:"${E2E_PORT}" -sTCP:LISTEN
  exit 1
fi

echo "==> 编译测试二进制"
# 不用 go run：go run 会把程序作为子进程拉起，杀父进程不会杀掉真正监听端口的进程，
# 结果就是每次跑完都残留一个实例占着端口。
go build -o /tmp/e2e-api ./cmd/api

echo "==> 启动临时实例 :${E2E_PORT}（短信走 console，不发真实短信）"
sed "s/port: 8080/port: ${E2E_PORT}/" configs/config.dev.yaml > /tmp/e2e-config.yaml
# WECHAT_APP_ID / SECRET 显式清空：用例里的微信 code 是假的，
# 配了真实 appid 就会真的去调微信接口并返回 40029，整套微信用例必挂。
# 清空后服务端走 dev 降级分支（code 即 openid），这是该组用例的前提。
# PAYMENT_SKIP_PAY=false：跳过支付是给试运行期用的，
# 用例要覆盖「下单 → 支付回调 → 待接单」的完整链路，必须走真实沙箱支付。
MYSQL_DSN="$DSN" SMS_PROVIDER=console WECHAT_APP_ID="" WECHAT_APP_SECRET="" \
  PAYMENT_SKIP_PAY=false \
  /tmp/e2e-api -config /tmp/e2e-config.yaml > "$LOG" 2>&1 &
echo $! > /tmp/e2e-server.pid

# 以日志出现「开始监听」为准，避免把 curl 通就当成就绪（可能连到别人的进程）
for _ in $(seq 1 40); do
  grep -q "开始监听" "$LOG" 2>/dev/null && break
  sleep 1
done
if ! grep -q "开始监听" "$LOG" 2>/dev/null; then
  echo "服务启动失败，日志：$LOG"
  tail -30 "$LOG"
  exit 1
fi
if grep -q "address already in use" "$LOG" 2>/dev/null; then
  echo "端口冲突，启动失败"; tail -10 "$LOG"; exit 1
fi

echo "==> 运行用例"
set +e
E2E_BASE="http://127.0.0.1:${E2E_PORT}" E2E_DB="$E2E_DB" E2E_ADMIN_PWD="$E2E_ADMIN_PWD" \
  python3 scripts/e2e_test.py
CODE=$?
set -e

if [ $CODE -ne 0 ]; then
  echo
  echo "服务端日志尾部（排查用）："
  tail -20 "$LOG"
fi
exit $CODE
