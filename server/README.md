# 小区好帮手 · 服务端

社区生活服务预约平台后端，Go + Gin + GORM + MySQL + Redis。对应需求文档 V1.1 与《Go 项目骨架说明》。

## 快速开始

```bash
# 1. 准备数据库
mysql -uroot -p -e "CREATE DATABASE community_handyman CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"

# 2. 填 .env：把 MYSQL_DSN 里的 CHANGE_ME 换成你的 MySQL 密码
#    程序启动时会自动加载 .env，无需 export
${EDITOR:-vim} .env

# 3. 建表并灌种子数据
make migrate-up

# 4. 起服务
make run
curl http://127.0.0.1:8080/healthz
```

`.env` 已被 `.gitignore` 忽略，不会进版本库；`.env.example` 是可提交的模板。
命令行显式 `export MYSQL_DSN=...` 的优先级高于 `.env`，便于临时切换库。

## 常用命令

| 命令 | 作用 |
|---|---|
| `make run` | 启动 HTTP 服务（默认 8080） |
| `make build` | 编译 `bin/api` 与 `bin/migrate` |
| `make migrate-up` | 执行全部迁移 |
| `make migrate-down steps=1` | 回滚一步 |
| `make migrate-version` | 查看当前迁移版本 |
| `ADMIN_PASSWORD=xxx make seed-admin user=admin` | 创建后台账号 |
| `make fmt` / `make vet` | 格式化与静态检查 |

## 接口（v1）

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/v1/dev/login` | 本地联调：按 openid 取或建用户（仅 dev） |
| POST | `/api/v1/addresses` | 新增地址，保存前校验服务范围 |
| GET | `/api/v1/addresses` | 地址列表 |
| PUT | `/api/v1/addresses/:id` | 编辑本人地址，重新校验归属与服务范围 |
| POST | `/api/v1/addresses/check` | 地址范围预校验 |
| POST | `/api/v1/orders/preview` | 费用试算，不落库 |
| POST | `/api/v1/orders` | 下单，返回支付参数 |
| GET | `/api/v1/orders?tab=` | 订单列表（pending / processing / completed） |
| GET | `/api/v1/orders/:orderNo` | 订单详情与节点时间轴 |
| POST | `/api/v1/orders/:orderNo/pay` | 重新发起支付 |
| POST | `/api/v1/orders/:orderNo/confirm` | 用户确认服务完成 |
| POST | `/api/v1/orders/:orderNo/records` | 上传服务记录（multipart） |
| GET | `/api/v1/orders/:orderNo/records` | 服务记录列表 |
| GET | `/api/v1/orders/:orderNo/records/:id/file` | 下载记录文件（鉴权 + Range） |
| GET | `/api/v1/provider/orders/open` | 可接单池 |
| GET | `/api/v1/provider/orders` | 服务方名下订单 |
| POST | `/api/v1/provider/orders/:orderNo/accept` | 接单 |
| POST | `/api/v1/provider/orders/:orderNo/reject` | 拒单 |
| POST | `/api/v1/provider/orders/:orderNo/advance` | 推进节点 |
| POST | `/api/v1/orders/:orderNo/cancel` | 用户取消（2 小时规则） |
| GET | `/api/v1/services/accompany/options` | 陪诊可选项：医院、可多选服务项、默认必选的基础费（均后台配置） |
| POST | `/api/v1/orders/:orderNo/quote/reject` | 拒绝历史陪诊报价，订单关闭（兼容旧订单） |
| GET | `/api/v1/orders/:orderNo/monthly` | 包月使用情况与核销明细 |
| POST | `/callback/sandbox/pay` | 沙箱支付回调（仅 sandbox 渠道） |

### 运营后台

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/admin/login` | 账号密码登录，返回 Bearer token（12 小时有效） |
| GET | `/api/admin/areas` | 服务范围列表 |
| POST | `/api/admin/areas` | 新增服务范围 |
| PATCH | `/api/admin/areas/:id/status` | 启停服务范围 |
| GET | `/api/admin/slots?date=` | 某天时段库存 |
| PUT | `/api/admin/slots` | 批量设置可接单量（total=0 即不开放） |
| GET | `/api/admin/skus` | 服务产品列表 |
| PUT | `/api/admin/skus/:id` | 改价与计价规则（rules 部分合并） |
| GET | `/api/admin/orders` | 订单查询（status / service_type / keyword） |
| POST | `/api/admin/orders/:orderNo/assign` | 派单或改派 |
| POST | `/api/admin/refunds` | 退款（**仅 admin 角色**，amount=0 为全额） |
| POST | `/api/admin/orders/:orderNo/quote` | 历史陪诊报价（兼容金额为 0 的旧订单；新订单自动计价，调用返回 2003） |
| POST | `/api/admin/orders/:orderNo/redeem` | 包月核销 |

除登录与回调外均需 `X-User-Id` 头（接入微信登录后改为 JWT，只改 `middleware.RequireUser`）。

支付流程默认走 sandbox 渠道：下单返回 `prepay_id`，向回调地址推送带 HMAC 签名的报文即模拟支付成功。商户号与证书齐备后，在 `payment.New` 中实现 `wechat` 分支即可，业务代码不动。

## 目录

```
cmd/api          HTTP 服务入口
cmd/migrate      迁移与种子数据命令
internal/
  config         配置加载与校验
  router         路由注册与中间件装配
  middleware     请求链路中间件
  handler        HTTP 层
  service        业务编排（事务边界）
  repository     数据访问
  model          GORM 模型与枚举
  dto            请求响应结构
  pricing        计价引擎（纯函数）
  payment        微信支付封装
  storage        对象存储
  errs           业务错误码
pkg/             通用工具（响应、日志、ID）
migrations/      golang-migrate SQL
```

## 数据库

当前迁移版本对应 12 张表：

- 业务表：`users` `addresses` `service_areas` `service_skus` `slot_inventories` `orders` `order_events` `service_records` `quotes`
- 支撑表：`payment_transactions` `refunds` `admin_users`

种子数据包含 13 条服务范围、4 档服务产品、未来 7 天时段库存。遛宠在工作日只开晚间，用 `slot_inventories.total = 0` 表达，规则落在数据而不是代码分支。

## 文件存储

默认 `storage.provider = local`，文件落在 `./uploads`（已被 .gitignore 忽略），权限 0600。

- **不挂静态目录**：服务记录含病历与门禁照片，只提供 `/records/:id/file` 鉴权下载，并校验订单归属。
- **切换云存储**：把 `provider` 改成 `oss` / `cos` / `s3` 并补齐密钥，实现 `storage.Storage` 接口即可；当前这些分支会明确报"尚未接入"，不会静默降级。

## 退款与取消

- 退款两步提交：先落 `pending` 退款单（`out_refund_no` 唯一防重复），再调渠道，成功后回写状态与 `refunded_amount`。渠道失败保留记录供对账。
- **全额退款且服务未履约时，订单转为「已取消」并释放时段**，否则会出现「服务方接了单却收不到钱」。
- 用户取消以「时段起始时间 − 2 小时」为界：2 小时外免费取消并原路退款，2 小时内返回 `need_review` 交运营处理，服务开始后直接拒绝。
- 后台角色两级：`operator` 可查可配，`admin` 才能退款。

## 包月与陪诊

- **陪诊**按后台配置的服务项自动计价：医院列表与可多选的服务项（含单价）写在陪诊 SKU 的 `rules.hospitals` / `rules.service_items`，用户端只读 `GET /api/v1/services/accompany/options` 获取；下单只回传医院 code 与所选服务项 code，后端按同一份配置求和并返回支付参数，直接支付，不再走人工报价。前端传价无效；改价、改选项只需 `PUT /api/admin/skus/:id` 部分合并 rules，无需发版。
- **基础服务费**写在 `rules.base_item`（默认 1000 分），默认必选、后端强制计入明细首位，前端不传 code 也照收、回传也不会重复计；配置缺失时退化为旧的「只按所选项目求和」。因基础费本身构成一单，只选基础费也能下单。
- **历史报价兼容**：`POST /api/admin/orders/:orderNo/quote` 与 `POST /api/v1/orders/:orderNo/quote/reject` 仅用于金额为 0 的历史待报价订单（`quotes` 表保留）；新陪诊订单 `total_amount > 0` 调用报价接口返回 2003，避免运营误改自动计价结果。
- **包月**服务期 30 天，从订单服务日期起算，不累计不顺延。不预占未来 30 天库存，只在核销当天占时段；首日在下单时已占位，核销时不再重复占（否则同一单吃掉两个名额）。
- 核销靠 `monthly_redemptions` 的 `(order_id, service_date)` 唯一索引防重复，重复核销返回「该日期已核销过」，且同一事务内的时段预占会一并回滚。

## 约定

- 金额一律 `int64`，单位分，禁止浮点参与计算。
- 订单状态变更必须同时写 `order_events`，可回放时间轴。
- 支付幂等依赖 `payment_transactions.transaction_id` 唯一索引，不要先查后插。
- 密钥走环境变量，`configs/config.yaml` 只写占位符。
- 改表必须附 up/down 迁移，生产禁用 `AutoMigrate`。
