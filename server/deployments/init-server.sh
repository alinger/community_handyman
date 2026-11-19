#!/usr/bin/env bash
# 服务器首次初始化（Ubuntu / Debian）。只需跑一次。
set -euo pipefail

APP_DIR="${APP_DIR:-/opt/community-handyman}"
REPO="${REPO:-https://github.com/alinger/community_handyman.git}"

echo "==> 检查依赖"
for cmd in git make mysql pm2 node; do
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "缺少 $cmd，请先安装后重试" >&2
    exit 1
  fi
done
if ! command -v go >/dev/null 2>&1; then
  echo "缺少 Go 工具链（需 1.25+）：https://go.dev/dl/" >&2
  exit 1
fi

echo "==> 克隆代码到 $APP_DIR"
if [ -d "$APP_DIR/.git" ]; then
  echo "目录已存在，跳过克隆"
else
  sudo mkdir -p "$APP_DIR"
  sudo chown "$(id -u):$(id -g)" "$APP_DIR"
  git clone "$REPO" "$APP_DIR"
fi

cd "$APP_DIR/server"
mkdir -p bin logs uploads

echo "==> 配置环境变量"
if [ ! -f "$APP_DIR/.env" ]; then
  cp .env.example "$APP_DIR/.env"
  cat >&2 <<'EOF'
已生成 .env 模板，请先填写以下必填项后重新运行 deploy.sh：
  MYSQL_DSN        数据库连接（含库名 community_handyman）
  AUTH_SECRET      后台 token 签名密钥，建议 openssl rand -hex 32
  PAYMENT_PROVIDER sandbox 或 wechat
  STORAGE_PROVIDER local 或 oss
EOF
  exit 0
fi

echo "==> 安装 pm2 日志轮转"
pm2 install pm2-logrotate >/dev/null 2>&1 || echo "pm2-logrotate 安装失败，可稍后手动安装"
pm2 set pm2-logrotate:max_size 50M >/dev/null 2>&1 || true
pm2 set pm2-logrotate:retain 14 >/dev/null 2>&1 || true

echo "==> 配置开机自启"
pm2 startup >/dev/null 2>&1 || echo "pm2 startup 需要 sudo，请按提示手动执行"
pm2 save >/dev/null 2>&1 || true

echo "初始化完成。下一步：编辑 $APP_DIR/.env 后执行 ./deployments/deploy.sh"
