# 部署说明（pm2）

服务端用 pm2 托管 Go 二进制，不用 Docker。目标机器：Ubuntu/Debian，2C4G。

## 为什么是 pm2 而不是 systemd

两者都能托管进程，选 pm2 的理由只有一个：团队已有 pm2 运维习惯，日志轮转、开机自启、`pm2 logs` 查看都不用重新学。代价是 pm2 为 Node 设计，托管 Go 二进制要显式配 `kill_timeout`——Go 服务收到 SIGINT 后会关连接、停定时任务，默认的 1.6 秒不够，会被强杀导致在处理的请求中断。

## 首次部署

```bash
# 1. 服务器初始化（装依赖、克隆代码、配 pm2 自启）
scp deployments/init-server.sh user@host:/tmp/
ssh user@host 'bash /tmp/init-server.sh'

# 2. 填环境变量（脚本会提示必填项）
ssh user@host 'vim /opt/community-handyman/server/.env'

# 3. 发布
ssh user@host 'cd /opt/community-handyman/server && ./deployments/deploy.sh'
```

## 必填环境变量

| 变量 | 说明 | 生成方式 |
|---|---|---|
| `MYSQL_DSN` | 含库名的完整 DSN | `root:pwd@tcp(127.0.0.1:3306)/community_handyman?charset=utf8mb4&parseTime=True&loc=Local` |
| `AUTH_SECRET` | 后台 token 签名密钥 | `openssl rand -hex 32` |
| `PAYMENT_PROVIDER` | `sandbox` 或 `wechat` | 商户号下来前填 sandbox |
| `STORAGE_PROVIDER` | `local` 或 `oss` | 单机先用 local |

`env=prod` 时缺 `MYSQL_DSN` 或 `AUTH_SECRET` 会**直接拒绝启动**，不会带着默认值上线。

## 常用命令

```bash
pm2 list                                  # 进程状态
pm2 logs community-handyman-api --lines 100
pm2 restart community-handyman-api        # 重启
pm2 reload community-handyman-api         # 平滑重启
pm2 stop community-handyman-api
```

## 发布流程

`deploy.sh` 做四件事：`git pull` → `make build` → `make migrate-up` → `pm2 startOrReload`，最后打一次 `/healthz` 确认。任一步失败即中断，不会带着半成品上线。

迁移是幂等的，重复执行只会输出「无需迁移」；回滚用 `make migrate-down steps=1 CONFIG=configs/config.prod.yaml`。

## 日志与轮转

- 应用日志：`server/logs/api-out.log`、`api-error.log`（pm2 托管）
- 业务日志：Go 用 `slog` 输出到 stdout，被 pm2 收进上面两个文件
- 轮转：`init-server.sh` 会安装 `pm2-logrotate`，单文件 50M、保留 14 份

## 已知限制

- **单实例**：`instances: 1`。Go 服务无状态，理论上可以多实例，但时段库存扣减目前靠数据库原子更新，多实例没问题；真正的限制是本地文件存储（`uploads/`）无法跨实例共享，切到对象存储后才能扩到多实例。
- **本地磁盘存储**：`storage.provider=local` 时迁移服务器要一并迁移 `uploads/` 目录。
