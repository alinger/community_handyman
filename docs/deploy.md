# 生产环境部署与配置

面向：Ubuntu 22.04 / Debian 12 / CentOS Stream 9 一台云服务器，MySQL 8 同机或内网。
本文覆盖「配置怎么给、服务怎么起、上线前怎么验」。

---

## 零、先选一条路（两条路不要同时用）

仓库里已有两套托管方式，都能跑，但**别同时启用**——都会去抢 8080，谁先起谁占坑，另一个反复重启。

| 方式 | 位置 | 特点 | 适合 |
|---|---|---|---|
| **pm2（既有方案）** | `server/deployments/` | 服务器 `git pull` 后本地 `make build`，`deploy.sh` 一条命令发布 | 服务器装了 Go、习惯 pm2 |
| **systemd + 上传二进制** | `server/deploy/` + 本文 | 本地交叉编译上传，服务器不需要 Go | 服务器不装 Go、或想固定产物 |

本文下半部分讲的是 **systemd 这条路**；走 pm2 请直接看 `server/deployments/README.md`，两边只有「进程托管」不同，配置注入方式完全一样。

---

## 零点五、要上传哪些文件

### 走 pm2（既有方案）：几乎不用传文件

```bash
# 一次性：初始化脚本
scp server/deployments/init-server.sh user@host:/tmp/
ssh user@host 'bash /tmp/init-server.sh'      # 装依赖、git clone、配 pm2 自启

# 配置在服务器上填，不要从本地传（避免把开发密钥带上线）
ssh user@host 'vim /opt/community-handyman/server/.env'

# 之后每次发布：一条命令，代码从 git 拉
ssh user@host 'cd /opt/community-handyman/server && ./deployments/deploy.sh'
```

> 注意：`init-server.sh` 会检查 `git / make / mysql / pm2 / node / go` 六项，缺任何一项直接退出并提示安装地址，但**它不会替你装**。Go 需要 1.25+，`apt` 源里的版本通常偏旧，建议按 go.dev/dl 装官方包。

### 走 systemd（上传二进制）：清单如下

| 本地路径 | 服务器路径 | 必需 | 说明 |
|---|---|---|---|
| `server/bin/api-linux-amd64` | `/opt/community-handyman/bin/api` | ✅ | `make build-linux` 产物，记得 `chmod +x` |
| `server/bin/migrate-linux-amd64` | `/opt/community-handyman/bin/migrate` | ✅ | 首次迁移、回滚、建后台账号都要它 |
| `server/configs/config.prod.yaml` | `.../configs/` | ✅ | 结构文件，不含密钥 |
| `server/migrations/` | `.../migrations/` | ✅ | migrate 按工作目录查找，缺了启动前置就失败 |
| `server/deployments/ecosystem.config.js` | `.../deployments/` | 用 pm2 时 | 走 systemd 则不需要 |
| `server/deploy/community-handyman.service` | `/etc/systemd/system/` | 走 systemd | 走 pm2 则不需要 |
| `admin/dist/` | `/var/www/handyman-admin/` | 用后台管理时 | 构建产物，见下方说明 |
| `server/.env.prod.example` | 服务器 `/etc/community-handyman/prod.env` | ✅ | 传模板后改名填值，**不要传本地 `.env`** |

**不需要上传的**：

- `server/docs/`（`swagger.json` 已通过 `docs.go` 编译进二进制）
- `miniapp/`（小程序代码用微信开发者工具上传，不经过服务器）
- `node_modules/`、`output/`、`server/logs/`、`server/bin/*.bak`（本地调试产物）
- `server/uploads/`（新机器留空；**迁移老数据时**才需要连同数据库一起搬）

### 用户端（小程序 / H5）的环境切换

接口地址同样是**构建期变量**，`miniapp/src/utils/request.ts` 只读 `VITE_API_BASE`：

| 文件 | 何时生效 | `VITE_API_BASE` | 备注 |
|---|---|---|---|
| `.env.development` | `npm run dev:*`、`--mode development` | 空 | H5 走 vite proxy；小程序回落到 `127.0.0.1:8080` |
| `.env.production` | `npm run build:*` | `https://m.yubangbzz.com` | 小程序与 H5 都直连生产 |

```bash
# 本地联调（热更新，接口指向本机 8080）
npm run dev:mp-weixin

# 生产构建（开发者工具导入 dist/build/mp-weixin 后上传）
npm run build:mp-weixin

# 想用开发者工具看「本地接口」但不想起 watch：产出到 dist/dev
npm run build:mp-weixin:dev
```

> 注意：`uni build` 默认就是 **production** 模式。直接 `uni build` 再丢进 `dist/dev` 会让开发包指向生产接口——
> 需要 dev 产物时务必带 `--mode development`（已封装成 `build:mp-weixin:dev`）。

临时连测试服：

```bash
VITE_API_BASE=https://test.yubangbzz.com VITE_APP_ENV=测试 npm run build:mp-weixin
```

「我的 → 关于我们」右侧会显示环境标签（仅非生产构建出现），真机排查时一眼能看出包连的是哪套接口。

**真机仍需两步**：小程序后台把 `https://m.yubangbzz.com` 加进「request 合法域名」；H5 站点地址要进服务端 `H5_ORIGIN` 白名单（CORS）。

### 后台管理前端的环境切换（`admin/`）

后台的接口地址由**构建时的环境变量**决定，代码里没有硬编码域名：

| 文件 | 模式 | `VITE_API_BASE` | 用途 |
|---|---|---|---|
| `.env.development` | `npm run dev` | `/`（vite proxy → `VITE_PROXY_TARGET`，默认 `127.0.0.1:8080`） | 本地开发 |
| `.env.production` | `npm run build` | `/` | 生产同域部署（Nginx 反代），**推荐** |
| `.env.remote` | `npm run build:remote` | `https://m.yubangbzz.com` | 后台与 API 分域，或本地直连生产排查 |

```bash
# 生产同域（后台和 API 同一个域名下，最省事、无 CORS）
npm run build              # → 接口走 /api，由 Nginx 转发

# 后台独立域名、直连 m.yubangbzz.com
npm run build:remote

# 临时连测试服，不改任何文件
VITE_API_BASE=https://test.yubangbzz.com VITE_APP_ENV=测试 npm run build
```

登录页底部会显示**当前环境标签 + 实际接口地址**（直连远端时标签变暖色），避免误在生产后台里改价、退款。

> 分域直连必须在服务端放行后台域名：`config.prod.yaml` 的 `allow_origins` 已加入 `${ADMIN_ORIGIN}`，
> 在 `/etc/community-handyman/prod.env` 里填 `ADMIN_ORIGIN=https://admin.yubangbzz.com` 即可。
> 同域部署留空——中间件遇到空值会忽略，不会变成「放行任意来源」。

### 后台管理前端（`admin/`）

```bash
cd admin && npm ci && npm run build      # 产物 admin/dist，约 1.7MB
rsync -av dist/ user@host:/var/www/handyman-admin/
```

它的 `baseURL` 是 `/`，即默认**与 API 同域**，靠 Nginx 把 `/api/` 反代到 `127.0.0.1:8080`：

```nginx
server {
    listen 443 ssl http2;
    server_name m.yubangbzz.com;          # 后台与 API 同域：/api 走反代，其余是静态资源

    root /var/www/handyman-admin;
    index index.html;

    location / {
        try_files $uri $uri/ /index.html;    # history 路由
    }
    location /api/ {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }
    location /callback/ {                     # 支付/退款回调
        proxy_pass http://127.0.0.1:8080;
    }
}
```

---

## 一、生产配置的注入方式

配置分两层，**不要混在一起**：

| 层 | 文件 | 内容 | 入库 |
|---|---|---|---|
| 结构 | `server/configs/config.prod.yaml` | 端口、超时、连接池、各类开关 | ✅ 已入库 |
| 值 | 环境变量（`/etc/community-handyman/prod.env`） | 数据库密码、密钥、AppID、短信 AK | ❌ 绝不入库 |

`config.prod.yaml` 里全是 `${VAR}` 占位符，启动时由 `os.Expand` 从环境变量展开。
**env=prod 时缺 `MYSQL_DSN`、`AUTH_SECRET`、短信四项中任意一项，进程会直接退出**——这是故意的，避免带着 localhost 的 DSN 或空密钥上线。

### 1. 生成配置

```bash
sudo mkdir -p /etc/community-handyman
sudo cp server/.env.prod.example /etc/community-handyman/prod.env
sudo chmod 600 /etc/community-handyman/prod.env
sudo chown root:root /etc/community-handyman/prod.env   # 只 root 可读，systemd 以 handyman 运行也能读到（见下方说明）
```

> 权限说明：`EnvironmentFile` 由 systemd（root）在降权前读取，所以 `600 root:root` 是安全的，业务进程本身读不到这个文件——这是有意的隔离。

### 2. 必填项与不填的后果

| 变量 | 示例 | 不填会怎样 |
|---|---|---|
| `MYSQL_DSN` | `app:xxx@tcp(127.0.0.1:3306)/community_handyman?charset=utf8mb4&parseTime=True&loc=Local` | 启动即退出 |
| `AUTH_SECRET` | `openssl rand -hex 32` | 启动即退出（检测到 `dev-only` 默认值也拒启） |
| `SMS_PROVIDER` | `aliyun` | 启动即退出（prod 禁用 console） |
| `SMS_ACCESS_KEY_ID` / `_SECRET` / `SMS_SIGN_NAME` / `SMS_TEMPLATE_CODE` | 阿里云控制台 | 启动即退出 |
| `H5_ORIGIN` | `https://admin.example.com` | 后台页面跨域被拦 |
| `WECHAT_APP_ID` / `WECHAT_APP_SECRET` | 小程序后台 | 微信登录报 1002，只能手机号登录 |
| `STORAGE_LOCAL_DIR` | `/var/lib/community-handyman/uploads` | 文件落到工作目录下，易被误删 |

密码含 `@ : / ? #` 必须 URL 编码后写进 DSN（`@`→`%40`，`:`→`%3A`，`/`→`%2F`）。

### 3. 生产与开发的关键差异

生产 yaml 已经写死，不要改成跟 dev 一样：

| 项 | dev | prod | 原因 |
|---|---|---|---|
| `server.mode` | debug | `release` | 关掉 gin 调试输出 |
| `database.log_sql` | true | `false` | 避免 SQL 明细把日志刷爆 |
| 连接池 | 25/10 | 50/20 | 线上并发更高 |
| `allow_origins` | localhost:5173 | 真实域名 | 不放通配 |
| Swagger | 开启 | **自动关闭**（`env=prod` 时不注册路由） | 不把后台接口摊开给外部 |
| `payment.skip_pay` | true | true（商户号齐备后改 false） | 试运行期不扣款 |

---

## 二、目录与账号

```bash
sudo useradd -r -s /sbin/nologin handyman
sudo mkdir -p /opt/community-handyman/{bin,configs,uploads}
sudo mkdir -p /var/lib/community-handyman/uploads
sudo chown -R handyman:handyman /opt/community-handyman /var/lib/community-handyman
```

上线后的结构：

```
/opt/community-handyman/
├── bin/api                    # make build-linux 产物
├── bin/migrate
├── configs/config.prod.yaml
├── migrations/                # migrate 按工作目录查找，必须上传
└── uploads -> /var/lib/community-handyman/uploads   # 可选：软链到数据盘

/etc/community-handyman/prod.env     # 600
/etc/systemd/system/community-handyman.service
```

---

## 三、数据库准备

```sql
-- 不要用 root 跑业务
CREATE DATABASE community_handyman DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
CREATE USER 'app'@'127.0.0.1' IDENTIFIED BY '强密码';
GRANT SELECT, INSERT, UPDATE, DELETE, CREATE, ALTER, INDEX, REFERENCES ON community_handyman.* TO 'app'@'127.0.0.1';
FLUSH PRIVILEGES;
```

时区：MySQL 与服务器都用 `Asia/Shanghai`，否则「预约日期」「15 分钟关单」这类判断会偏 8 小时。

```bash
sudo timedatectl set-timezone Asia/Shanghai
```

---

## 四、启动服务

```bash
# 1. 上传二进制（在本地交叉编译）
make build-linux ARCH=amd64        # 或 ARCH=arm64
scp server/bin/api-linux-amd64  root@host:/opt/community-handyman/bin/api
scp server/bin/migrate-linux-amd64 root@host:/opt/community-handyman/bin/migrate
scp server/configs/config.prod.yaml root@host:/opt/community-handyman/configs/
scp -r server/migrations root@host:/opt/community-handyman/     # 迁移脚本必须随二进制上传
scp server/deploy/community-handyman.service root@host:/etc/systemd/system/

# 2. 服务器上
sudo chmod +x /opt/community-handyman/bin/*
sudo systemctl daemon-reload
sudo systemctl enable --now community-handyman
systemctl status community-handyman
journalctl -u community-handyman -f
```

`ExecStartPre` 每次启动都会跑 `migrate -cmd up`，迁移是幂等的（已是最新则跳过），不会重复执行。注意 `bin/migrate` 默认按**工作目录**找 `migrations/`，所以该目录要一起上传到 `/opt/community-handyman/`，否则启动前置会失败。

### Nginx 反代（小程序 request 合法域名必须是 HTTPS）

```nginx
server {
    listen 443 ssl http2;
    server_name api.example.com;

    ssl_certificate     /etc/letsencrypt/live/api.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/api.example.com/privkey.pem;

    # 服务记录视频上限 80MB，留出余量
    client_max_body_size 100m;
    # 大文件上传与导出需要更长超时
    proxy_read_timeout 120s;
    proxy_send_timeout 120s;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

拿到域名后，在小程序后台「开发管理 → 服务器域名」把它加进 **request 合法域名**，否则真机全部请求被拦。

---

## 五、上线验收清单

按顺序跑一遍，全过再交付：

```bash
curl -s https://api.example.com/healthz            # {"code":0,...,"status":"ok"}
curl -s -o /dev/null -w "%{http_code}\n" https://api.example.com/swagger/index.html   # 期望 404
curl -s https://api.example.com/api/v1/service-areas | head -c 100                     # 公开接口应可用
```

| 检查项 | 期望 |
|---|---|
| 未登录访问 `/api/v1/orders` | 401 |
| 手机号发码 | 真机收到短信（不是日志里打码） |
| 微信登录 | 能建号，`need_bind_phone` 为 true |
| 微信授权手机号 | 绑定成功（需小程序非个人主体已认证） |
| 下单 | 直接进入「待接单」、无支付弹窗（skip_pay=true） |
| 头像上传与显示 | 上传成功，`/api/v1/users/:id/avatar/*` 可直接访问 |
| 服务记录上传 80MB 视频 | 不被 nginx 413 拦 |
| 后台登录 | 用 `make seed-admin` 建的账号能进 |

后台账号初始化：

```bash
cd /opt/community-handyman
# 建后台账号（密码走环境变量，别写在命令行历史里）
sudo -u handyman env ADMIN_PASSWORD='临时强密码' \
  ./bin/migrate -config configs/config.prod.yaml -cmd seed-admin -username admin -role admin
```

`-role` 可选 `admin`（全权限）或 `operator`（运营，能做退款审核但改不了账号）。

---

## 六、更新与回滚

```bash
# 更新：留一份上一版二进制，回滚只要换回来（有新增迁移时同步上传 migrations/）
sudo cp /opt/community-handyman/bin/api /opt/community-handyman/bin/api.bak-$(date +%F-%H%M)
sudo systemctl stop community-handyman
sudo cp api-linux-amd64 /opt/community-handyman/bin/api
sudo systemctl start community-handyman

# 回滚
sudo systemctl stop community-handyman
sudo cp /opt/community-handyman/bin/api.bak-2026-11-20-2011 /opt/community-handyman/bin/api
sudo systemctl start community-handyman
```

**迁移只向前**：`migrate up` 不会自动回滚。涉及删列/改类型的迁移，先在预发库跑一遍，并确认 `migrate down` 可用（项目里每个 `.up.sql` 都有对应 `.down.sql`）。

---

## 七、备份

```bash
# 每天 3 点：数据库 + 上传文件
0 3 * * *  mysqldump --single-transaction -u app -p'密码' community_handyman | gzip > /backup/db-$(date +\%F).sql.gz
30 3 * * * rsync -a --delete /var/lib/community-handyman/uploads/ /backup/uploads/
# 保留 14 天
0 4 * * *  find /backup -name "*.sql.gz" -mtime +14 -delete
```

`--single-transaction` 保证 InnoDB 一致性快照，不锁表；上传文件与数据库要**同一天**的备份，否则记录里的文件路径会对不上。

---

## 八、常见坑

| 现象 | 原因 | 处理 |
|---|---|---|
| 启动即退出，提示缺 `MYSQL_DSN` | 环境变量没被 systemd 读到 | 检查 `EnvironmentFile` 路径与文件权限 |
| 图片上传成功但刷新 404 | `storage.local_dir` 用了相对路径，工作目录变了 | 用绝对路径 `/var/lib/...` |
| 真机请求全部失败，工具里正常 | 域名没进「request 合法域名」或不是 HTTPS | 配域名 + 证书 |
| 小程序登录报 1002 | 服务端没重启（旧 AppID 还在内存里）或 AppID/Secret 不匹配 | 重启 + 核对 |
| 后台页面跨域报错 | `H5_ORIGIN` 与实际域名不一致 | 改环境变量后重启 |
| 短信发不出去 | 签名/模板未审核通过，或模板变量名不是 `${code}` | 控制台核对 |
| 订单一直是待支付 | `payment.skip_pay` 被设成 false 但没接真实支付 | 改回 true 或接支付 |
| 时间差 8 小时 | 服务器或 MySQL 时区不是 `Asia/Shanghai` | 统一时区 |

---

## 九、当前还缺的两件事（上线前必须补）

1. **微信支付**：商户号、API v3 密钥、商户私钥、回调地址，然后把 `payment.provider` 改 `wechat`、`skip_pay` 改 `false`。代码里 `payment.Gateway` 已抽好接口，新增 `wechat.go` 实现即可，业务层不用动。
2. **对象存储**：`storage.provider=local` 在多实例部署下会导致「A 机器上传、B 机器读不到」。一旦要扩到 2 台以上，必须切 OSS/COS/S3。
