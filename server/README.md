# 邻有回应 · 服务端

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
| `make e2e` | 端到端主流程测试（独立库 + 独立端口，不碰开发库与真实短信） |

## 接口（v1）

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/v1/auth/register` | 手机号 + 密码注册，直接返回 token |
| POST | `/api/v1/auth/login` | 手机号 + 密码登录 |
| POST | `/api/v1/auth/sms/code` | 发送短信验证码（scene=login 登录/注册，reset 找回密码） |
| POST | `/api/v1/auth/sms/login` | 验证码登录，未注册手机号自动建号 |
| POST | `/api/v1/auth/password/reset` | 凭短信验证码重置密码，成功直接返回登录态 |
| POST | `/api/v1/auth/wechat` | 微信一键授权登录，openid 不存在则自动建号 |
| POST | `/api/v1/auth/phone` | 微信登录后绑定手机号（号码被占用返回 1005） |
| POST | `/api/v1/dev/login` | 本地联调：按 openid 取或建用户（仅 dev） |
| POST | `/api/v1/addresses` | 新增地址，保存前按小区+门牌校验服务范围 |
| GET | `/api/v1/addresses` | 地址列表 |
| GET | `/api/v1/addresses/communities` | 可选小区列表（后台配置的启用范围按小区去重） |
| PUT | `/api/v1/addresses/:id` | 编辑本人地址，重新校验归属与服务范围 |
| PUT | `/api/v1/addresses/:id/default` | 设为默认地址，事务内互斥清除该用户其它默认，返回更新后的列表 |
| POST | `/api/v1/addresses/check` | 地址范围预校验（小区+门牌，路弄可空） |
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
| GET | `/api/admin/orders` | 订单查询（status / service_type / keyword；含下单用户/服务方昵称、已退金额、取消原因、支付/完成时间等扩展字段） |
| GET | `/api/admin/providers` | 可派单服务方列表（供后台派单下拉选人） |
| POST | `/api/admin/orders/:orderNo/assign` | 派单或改派 |
| POST | `/api/admin/refunds` | 退款（**仅 admin 角色**，amount=0 为全额） |
| POST | `/api/admin/orders/:orderNo/quote` | 历史陪诊报价（兼容金额为 0 的旧订单；新订单自动计价，调用返回 2003） |
| POST | `/api/admin/orders/:orderNo/redeem` | 包月核销 |
| GET | `/api/admin/orders/:orderNo/events` | 订单节点回放（操作轨迹） |
| GET | `/api/admin/orders/:orderNo/records` | 后台查看服务记录 |
| GET | `/api/admin/records/:id/file` | 后台下载/预览记录文件（鉴权 + Range） |
| GET | `/api/admin/stats` | 运营看板（今日订单/流水/退款/待派单/状态分布）（含近 7 日订单/流水趋势） |
| GET | `/api/admin/payments` | 支付流水分页（order_no / status 筛选） |
| GET | `/api/admin/refunds` | 退款单分页（含操作人与退款状态跟踪） |
| GET | `/api/admin/users` | 用户/服务方分页（role / keyword） |
| PATCH | `/api/admin/users/:id/role` | 角色切换（user ↔ provider，切换后即可被派单） |
| PATCH | `/api/admin/users/:id/status` | 封禁/解封（封禁后小程序端拒绝登录） |
| GET | `/api/admin/redemptions` | 月卡核销记录（按服务日期筛选）（含所属订单的包月开始/结束日期） |
| PUT | `/api/admin/areas/:id` | 修改服务范围规则 |
| POST | `/api/admin/slots/copy` | 时段模板复制（源日配置 → 多个目标日期） |
| GET | `/api/admin/export/orders` | 订单 CSV 导出（同列表筛选，带 BOM） |
| GET | `/api/admin/accounts` | 后台账号列表（**仅超管**） |
| POST | `/api/admin/accounts` | 新建后台账号（operator/admin，**仅超管**） |
| PATCH | `/api/admin/accounts/:id/status` | 启停后台账号（不能停用自己，**仅超管**） |
| PATCH | `/api/admin/accounts/:id/password` | 重置账号密码（**仅超管**） |
| PATCH | `/api/admin/password` | 自助修改密码（需验证旧密码） |
| POST | `/api/admin/orders/:orderNo/cancel` | 代客取消（**仅超管**；关单释放时段并自动全额退剩余） |
| POST | `/api/admin/orders/:orderNo/reschedule` | 代客改期（**仅超管**；校验目标时段余量，同事务换占） |

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

## 短信验证码

登录入口支持手机号验证码，不需要密码；手机号首次验证即自动建号（昵称默认「住户」）。
渠道默认阿里云短信（Dysmsapi `SendSms`，按官方 RPC 签名自己实现，未引入 SDK）。

配置（`.env`）：

```bash
SMS_PROVIDER=aliyun          # 留空：dev 用 console，其它环境用 aliyun
SMS_ACCESS_KEY_ID=xxx
SMS_ACCESS_KEY_SECRET=xxx
SMS_SIGN_NAME=邻有回应      # 控制台审核通过的签名
SMS_TEMPLATE_CODE=SMS_123456 # 模板内容需含 ${code} 变量
SMS_ENDPOINT=                # 可选，专有云/海外区域
```

- **本地联调**：`SMS_PROVIDER` 留空走 console 渠道，验证码只打到服务日志，
  接口不返回验证码本身，联调时看日志抄码。
- **限流**：同号同用途 60 秒一次、每日 10 条；验证码 5 分钟有效、一次性消费、
  最多错 5 次；下发失败的记录不计入限流。
- **安全**：接口响应只含有效期与重发间隔，**任何环境都不回传验证码**——
  验证码一旦出现在响应里，等于把「持有该手机号」的证明公开给调用方，校验环节就失效了。
  验证码只存库、不明文输出，校验用常量时间比较；重置密码只对已注册手机号下发；
  生产环境启动时若 `SMS_PROVIDER=aliyun` 缺密钥会直接失败。

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

## 登录方式

四种入口，都最终签发同一套 token：

| 方式 | 适用场景 | 说明 |
|---|---|---|
| 微信一键授权 | 小程序内主推 | `wx.login` 拿 code → 后端换 openid → 建号/登录；无手机号时 `need_bind_phone=true` |
| 手机号 + 密码 | 老用户、H5 | 注册与登录分离 |
| 手机号验证码 | 免密登录 | 未注册手机号自动建号 |
| 访客 | 只浏览 | 不建账号、不发 token，仅放行首页/我的/文档页 |

**微信登录配置**（`.env`）：

```bash
WECHAT_APP_ID=wx1234567890abcdef    # 小程序 appid，同时要填 manifest.json 的 mp-weixin.appid
WECHAT_APP_SECRET=xxx               # 小程序 AppSecret，只用于换 openid，与支付密钥无关
```

- **未配 AppID/Secret 时**：dev 环境退化为「code 即 openid」（`devwx_<code>`），前端流程可完整联调；
  生产环境缺配置会明确报错，不会放行。
- **手机号绑定冲突**：号码已被其他账号占用时返回 1005，**不做自动合并** — 自动合并会把两个账号的订单混在一起，属于不可逆事故。
- **访客边界**：`AUTH_PAGES`（订单/地址/设置/服务方端）仍需登录；受限操作弹「去登录 / 再逛逛」，不强行踢走。

## 端到端测试

```bash
make e2e
```

一条命令完成：重建独立测试库 → 执行迁移 → 建后台账号 → 编译并启动临时实例（`:8098`）
→ 跑完 59 项用例 → 清理进程。

- **不污染开发库**：全程操作 `community_handyman_e2e`，跑完不会留下数据；
- **不发真实短信**：`SMS_PROVIDER=console`，验证码只进 `/tmp/e2e-server.log`；
- **可重复执行**：每次重建库，手机号按时间戳生成，不会撞唯一索引；
- **端口占用会直接报错**：不会把请求打到残留实例上（否则结果是假绿）。

覆盖范围（12 组）：

| 组 | 内容 |
|---|---|
| 1 | 注册、密码登录、错误密码、未授权访问 |
| 2 | 地址新增、服务范围校验（超范围 2001）、默认地址 |
| 3 | 代倒垃圾/遛宠/陪诊三类计价，陪诊强制含基础费 |
| 4 | 下单、沙箱支付、**回调幂等**、重复支付拦截 |
| 5 | 服务方证件上传、提交申请、后台审核授予角色 |
| 6 | 接单、节点推进、非法跃迁拦截、越权推进拦截 |
| 7 | 服务记录上传、本人可读、履约方可读、**无关第三方 403** |
| 8 | 用户确认完成、终态不可取消 |
| 9 | 未支付取消、已支付取消**自动退款**、重复退款拦截 |
| 10 | 包月购买、未来日期核销拦截、当日核销、同日重复核销拦截 |
| 11 | 验证码下发（**响应不含验证码**）、限频、一次性消费、重置密码 |
| 11.5 | 微信授权登录建号、重复登录不重复建号、绑手机号、号码冲突拦截 |
| 12 | 后台看板、订单列表、CSV 导出、用户封禁 |

单项失败不影响其余用例继续跑，最后统一汇总；失败会打印期望码与实际响应，便于定位。

## 约定

- 金额一律 `int64`，单位分，禁止浮点参与计算。
- 订单状态变更必须同时写 `order_events`，可回放时间轴。
- 支付幂等依赖 `payment_transactions.transaction_id` 唯一索引，不要先查后插。
- 密钥走环境变量，`configs/config.yaml` 只写占位符。
- 改表必须附 up/down 迁移，生产禁用 `AutoMigrate`。
