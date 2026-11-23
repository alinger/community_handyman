/**
 * 接口封装。字段与 server/docs/swagger.json 保持一致，
 * 后端改字段时这里要同步，否则问题是运行期才暴露。
 */
import {
  request, upload, ApiError, BASE_URL, getToken, getUserId,
} from '@/utils/request'

export interface Address {
  id: number
  community: string
  lane: string
  house_no: string
  extra: string
  contact_name: string
  contact_phone: string
  is_default: boolean
  in_service_area: boolean
}

/** 陪诊可选项，来自后台 service_skus.rules 配置，价格单位分 */
export interface AccompanyHospital {
  code: string
  name: string
}

export interface AccompanyServiceItem {
  code: string
  name: string
  price: number
}

export interface AccompanyOptions {
  hospitals: AccompanyHospital[]
  service_items: AccompanyServiceItem[]
  /** 默认必选的基础服务费，后端强制计入；老配置可能没有此字段 */
  base_item?: AccompanyServiceItem
}

export interface PriceItem {
  name: string
  amount: number
  note?: string
}

/** 熟悉费累计进度：仅遛宠开启「提前上门熟悉」时返回 */
export interface FamiliarProgress {
  done: number
  /** 免熟悉费所需单数，来自后台规则 */
  need: number
  free: boolean
}

export interface PreviewResp {
  total: number
  items: PriceItem[]
  manual_quote: boolean
  familiar_progress?: FamiliarProgress
}

export interface OrderNode {
  node: string
  label: string
  reached: boolean
  at?: string
  remark?: string
}

export interface Quote {
  amount: number
  duration_min: number
  confirmed: boolean
  created_at: string
}

export interface Order {
  order_no: string
  service_type: string
  plan: string
  service_date: string
  slot_code: string
  status: string
  status_label: string
  tab: string
  nodes: OrderNode[]
  total_amount: number
  paid_amount: number
  remark: string
  created_at: string
  address?: Address
  price_items?: PriceItem[]
  pay_expires_at?: string
  quote?: Quote
  hospital?: string
  patient_name?: string
  /** 包月备选服务时间（选中即取代默认取件时段，展示优先于 slot_code） */
  extra_slot_code?: string
  /** 服务方昵称与电话：仅「已接单」及之后的订单才有，未接单为空 */
  provider_nickname?: string
  provider_phone?: string
  /** 宠物备注（遛宠订单） */
  pet_notes?: string
  /** 进入方式：knock 敲门 / password 门禁密码 / card 门禁卡 / property 物业代开 / other 其他 */
  entry_method?: string
  /** 门禁密码或钥匙说明，仅在服务开始前 2 小时至服务结束后下发 */
  access_info?: string
  /** 钥匙信息可见性：不存在=没填，false=填了但还没到可见时间 */
  access_visible?: boolean
  /** 拒单原因与已退金额：拒单自动退款后用户可自查 */
  cancel_reason?: string
  refunded_amount?: number
  /** 支付成功时间与完成时间（ISO8601），完成后详情页展示 */
  paid_at?: string
  finished_at?: string
  accompany_items?: PriceItem[]
  /** 服务方发起、等待住户确认的改期申请；没有则为空 */
  pending_reschedule?: RescheduleInfo
  /** 改期历史，最新在前 */
  reschedules?: RescheduleInfo[]
  /** 住户改了时间、服务方还没确认知悉（服务方视角才有） */
  reschedule_notice?: RescheduleInfo
  /** 月卡（包月代倒垃圾）今日进度；只有 plan=monthly 的订单有 */
  monthly?: MonthlyProgress
  /** 包月服务期起止。接单池靠它说明「这单要跑多少天」，没有它只看价格会和单次混淆 */
  monthly_start?: string
  monthly_end?: string
}

/**
 * 月卡今日进度。
 * 月卡订单跑满 30 天，status 一直停在「已受理」，
 * 所以「今天上门没有」只能看这个对象，不能看 order.status。
 */
export interface MonthlyProgress {
  /** 服务端认定的今天，不要用本机时间自己算 */
  today: string
  start_date: string
  end_date: string
  used: number
  total: number
  /** 今天是否已点「出发上门」（人到了） */
  today_done: boolean
  today_at?: string
  /** 今天这趟是否已点「完成服务」；打卡只证明人到了，袋子提走没有看这个 */
  today_finished: boolean
  today_finished_at?: string
  /** 今天是否在服务期内，false 时不能打卡 */
  in_period: boolean
  /** 服务期已结束，可以提交完成结算 */
  ended: boolean
}

/** 一条改期记录：from_* 是原时间，to_* 是目标时间 */
export interface RescheduleInfo {
  id: number
  from_date: string
  from_slot: string
  to_date: string
  to_slot: string
  reason?: string
  /** pending 待确认 / applied 已生效 / rejected 已拒绝 / canceled 已作废 */
  status: string
  /** user 住户发起 / provider 服务方发起 */
  initiated_by: string
  created_at: string
  handled_at?: string
  /** 服务方确认知悉这次改期的时间，没有值表示还没确认 */
  provider_ack_at?: string
}

export interface ReschedulePayload {
  /** 让本类型可作为通用 JSON 请求体（Record<string, unknown>）传给 request */
  [key: string]: unknown
  service_date: string
  slot_code: string
  reason?: string
}

export interface PayParams {
  provider: string
  prepay_id: string
  params?: Record<string, string>
  mock: boolean
  mock_hint?: string
}

export interface CreateOrderResp {
  order: Order
  pay?: PayParams
  manual_quote: boolean
  pay_hint?: string
}

export interface PageResp<T> {
  list: T[]
  total: number
  page: number
  size: number
}

export interface ServiceRecord {
  id: number
  media_type: string
  size: number
  duration_sec: number
  /** 1 表示主人已同意拍摄 */
  owner_consented: number
  /** 凭证阶段：start=开始服务 / finish=完成服务 / 空=过程记录 */
  stage: string
  uploader_role: string
  created_at: string
  url: string
}

/** 服务方收入汇总 */
export interface ProviderIncome {
  total: number
  order_count: number
  items: { order_no: string; amount: number; created_at: string }[]
}

export interface MonthlySummary {
  total: number
  used: number
  remaining: number
  start_date: string
  end_date: string
  redemptions: { id: number; service_date: string; remark: string; finished_at?: string }[]
  today: string
  today_done: boolean
  today_at?: string
  today_finished: boolean
  today_finished_at?: string
  in_period: boolean
  ended: boolean
}

export interface CreateOrderPayload {
  /** 让本类型可作为通用 JSON 请求体（Record<string, unknown>）传给 request */
  [key: string]: unknown
  address_id: number
  service_type: string
  plan?: string
  service_date: string
  slot_code: string
  /** 包月备选服务时间（选中后取代默认取件时段） */
  extra_slot_code?: string
  quantity?: number
  pet_count?: number
  duration_min?: number
  distance_km?: number
  garbage_type?: string
  remark?: string
  feed_only?: boolean
  need_familiar?: boolean
  /** 进入方式与门禁信息（选填）；access_info 由服务端加密落库，不会明文回传 */
  entry_method?: string
  access_info?: string
  /** 陪诊专用：只回传 code，价格由后端按 SKU 配置计算 */
  hospital?: string
  patient_name?: string
  service_items?: string[]
}

/* ---------- 认证 ---------- */

export interface SessionUser {
  user_id: number
  nickname: string
  phone: string
  role: string
  /** 头像相对地址，展示时拼 BASE_URL；空串表示未设置 */
  avatar?: string
}

export interface Session {
  token: string
  expires_in: number
  user: SessionUser
}

/** 手机号注册，成功后直接返回登录态，无需再登录一次 */
export function register(data: { phone: string; password: string; nickname?: string }) {
  return request<Session>({
    url: '/api/v1/auth/register',
    method: 'POST',
    data,
    auth: false,
    loading: true,
  })
}

export function login(data: { phone: string; password: string }) {
  return request<Session>({
    url: '/api/v1/auth/login',
    method: 'POST',
    data,
    auth: false,
    loading: true,
  })
}

/**
 * 发送短信验证码。scene=login 登录/注册，reset 找回密码。
 * 响应只回有效期与重发间隔，服务端不会返回验证码本身。
 */
export function sendSmsCode(data: { phone: string; scene?: 'login' | 'reset' }) {
  return request<{ expires_in: number; interval: number }>({
    url: '/api/v1/auth/sms/code',
    method: 'POST',
    data,
    auth: false,
  })
}

/** 验证码登录，手机号未注册时后端自动建号 */
export function loginBySms(data: { phone: string; code: string; nickname?: string }) {
  return request<Session>({
    url: '/api/v1/auth/sms/login',
    method: 'POST',
    data,
    auth: false,
    loading: true,
  })
}

export interface WechatLoginResult extends Session {
  /** 为真表示账号还没手机号，前端应引导绑定 */
  need_bind_phone: boolean
}

/**
 * 微信一键授权登录。code 来自 uni.login（小程序端会走 wx.login）。
 * openid 不存在时后端自动建号。
 */
export function wechatLogin(data: { code: string; nickname?: string; avatar?: string }) {
  return request<WechatLoginResult>({
    url: '/api/v1/auth/wechat',
    method: 'POST',
    data,
    auth: false,
    loading: true,
  })
}

/** 给当前账号绑定手机号 */
export function bindPhone(data: { phone: string; code?: string }) {
  return request<Session>({
    url: '/api/v1/auth/phone',
    method: 'POST',
    data,
    loading: true,
  })
}

/**
 * 微信「手机号快速验证组件」一键绑定手机号。
 * code 来自 button open-type="getPhoneNumber" 的回调，5 分钟内有效且只能用一次。
 */
export function bindPhoneByWechat(data: { code: string }) {
  return request<Session>({
    url: '/api/v1/auth/wechat/phone',
    method: 'POST',
    data,
    loading: true,
  })
}

/** 凭短信验证码重置密码；成功后直接返回登录态，无需再登录 */
export function resetPasswordBySms(data: { phone: string; code: string; new_password: string }) {
  return request<Session>({
    url: '/api/v1/auth/password/reset',
    method: 'POST',
    data,
    auth: false,
    loading: true,
  })
}

export function changePassword(data: { old_password: string; new_password: string }) {
  return request<{ changed: boolean }>({
    url: '/api/v1/auth/password',
    method: 'PUT',
    data,
    loading: true,
  })
}

/**
 * 本地联调登录，仅 dev 环境可用。
 * role 用于把 openid 对应的账号设成服务方，开发期身份切换靠它。
 */
export function devLogin(openid: string, nickname: string, role?: string) {
  return request<{
    user_id: number
    openid: string
    nickname: string
    role: string
    token: string
    expires_in: number
  }>({
    url: '/api/v1/dev/login',
    method: 'POST',
    data: { openid, nickname, role },
    auth: false,
  })
}

/* ---------- 个人信息（设置页） ---------- */

export interface MeProfile {
  user_id: number
  nickname: string
  phone: string
  role: string
  avatar?: string
}

export function getMe() {
  return request<MeProfile>({ url: '/api/v1/me' })
}

/** 整表提交：phone 传空串即清除 */
export function updateMe(data: { nickname: string; phone: string; avatar?: string }) {
  return request<MeProfile>({ url: '/api/v1/me', method: 'PUT', data, loading: true })
}

/**
 * 上传头像。微信 chooseAvatar 给的是临时文件路径，必须先落到服务端，
 * 否则页面一刷新就失效（临时路径只在本次运行内有效）。
 */
export function uploadAvatar(filePath: string) {
  return upload('/api/v1/me/avatar', filePath, {}).then((body) => {
    const data = body.data as { avatar?: string } | undefined
    if (!data?.avatar) throw new ApiError(-1, '头像上传响应缺少地址')
    return data.avatar
  })
}

/** 代倒垃圾包月方案（公开接口） */
export interface GarbagePlan {
  price: number
  /** 单次价格（分）；有赠送次数时实际支付为 0 */
  single_price: number
  days: number
  default_slot: string
  extra_slots: string[]
  free_per_user: number
}

export function getGarbagePlan() {
  return request<GarbagePlan>({ url: '/api/v1/services/garbage-plan', auth: false })
}

/** 注册赠送的免费次数余额 */
export interface QuotaBalance {
  service_type: string
  total: number
  used: number
  remaining: number
}

export function getQuotas() {
  return request<QuotaBalance[]>({ url: '/api/v1/me/quotas' })
}

/** 服务范围里的一个路段：门牌区间与单双号限制 */
export interface ServiceAreaLane {
  lane: string
  /** 门牌起止，两者都为 0 表示不限门牌；to 为 0 表示无上限 */
  house_from: number
  house_to: number
  /** all=不限，odd=单号，even=双号 */
  house_parity: 'all' | 'odd' | 'even'
}

/** 一个已开通小区的服务范围（按小区聚合） */
export interface ServiceAreaScope {
  community: string
  lanes: ServiceAreaLane[]
}

/**
 * 已开通的服务范围。公开接口，访客也能看 —— 下单前先确认自己小区是否覆盖。
 */
export function listServiceAreas() {
  return request<ServiceAreaScope[]>({ url: '/api/v1/service-areas', auth: false })
}

export function listAddresses() {
  return request<Address[]>({ url: '/api/v1/addresses' })
}

/** 可选小区：后台配置的服务范围按小区去重，新增地址只能从此处选 */
export function listCommunities() {
  return request<string[]>({ url: '/api/v1/addresses/communities' })
}

export function createAddress(data: {
  community: string
  lane?: string
  house_no: string
  extra?: string
  contact_name?: string
  contact_phone?: string
}) {
  return request<Address>({ url: '/api/v1/addresses', method: 'POST', data })
}

export function updateAddress(
  id: number,
  data: {
    community: string
    lane?: string
    house_no: string
    extra?: string
    contact_name?: string
    contact_phone?: string
  },
) {
  return request<Address>({ url: `/api/v1/addresses/${id}`, method: 'PUT', data })
}

/** 设为默认地址，返回更新后的地址列表（默认项已排在最前） */
export function setDefaultAddress(id: number) {
  return request<Address[]>({ url: `/api/v1/addresses/${id}/default`, method: 'PUT' })
}

/** 删除地址，返回删除后的剩余列表（删默认地址时后端会自动提升下一条为默认） */
export function deleteAddress(id: number) {
  return request<Address[]>({ url: `/api/v1/addresses/${id}`, method: 'DELETE' })
}

/** 时段余量：满员时段前端置灰不可选 */
export interface SlotAvailability {
  slot_code: string
  total: number
  used: number
  remaining: number
  available: boolean
}

export function slotAvailability(date: string, serviceType: string) {
  return request<SlotAvailability[]>({
    url: `/api/v1/services/slots?date=${date}&service_type=${serviceType}`,
  })
}

/** 陪诊可选项：医院、可多选服务项与默认必选的基础费，由后台配置 */
export function accompanyOptions() {
  return request<AccompanyOptions>({ url: '/api/v1/services/accompany/options' })
}

export function checkArea(data: { community: string; lane?: string; house_no: string }) {
  return request<{ in_service_area: boolean; community: string; lane: string }>({
    url: '/api/v1/addresses/check',
    method: 'POST',
    data,
  })
}

export function previewOrder(data: CreateOrderPayload) {
  return request<PreviewResp>({ url: '/api/v1/orders/preview', method: 'POST', data })
}

export function createOrder(data: CreateOrderPayload) {
  return request<CreateOrderResp>({ url: '/api/v1/orders', method: 'POST', data, loading: true })
}

export function listOrders(tab: string, page = 1, size = 10) {
  return request<PageResp<Order>>({
    url: `/api/v1/orders?tab=${tab}&page=${page}&size=${size}`,
  })
}

export function orderDetail(orderNo: string) {
  return request<Order>({ url: `/api/v1/orders/${orderNo}` })
}

export function payOrder(orderNo: string) {
  return request<PayParams>({ url: `/api/v1/orders/${orderNo}/pay`, method: 'POST', loading: true })
}

export function cancelOrder(orderNo: string, reason: string) {
  return request<{
    canceled: boolean
    need_review: boolean
    refund_due: number
    refund_ok?: boolean
    hint?: string
    message?: string
  }>({ url: `/api/v1/orders/${orderNo}/cancel`, method: 'POST', data: { reason }, loading: true })
}

export function confirmOrder(orderNo: string) {
  return request<{ completed: boolean }>({
    url: `/api/v1/orders/${orderNo}/confirm`,
    method: 'POST',
    loading: true,
  })
}

/** 住户修改服务时间，立即生效 */
export function rescheduleOrder(orderNo: string, data: ReschedulePayload) {
  return request<RescheduleInfo>({
    url: `/api/v1/orders/${orderNo}/reschedule`,
    method: 'POST',
    data,
    loading: true,
  })
}

/** 住户处理服务方的改期申请：同意则立即生效 */
export function respondReschedule(orderNo: string, approve: boolean, reason = '') {
  return request<RescheduleInfo>({
    url: `/api/v1/orders/${orderNo}/reschedule/respond`,
    method: 'POST',
    data: { approve, reason },
    loading: true,
  })
}

/** 服务方申请改期，需住户确认后生效 */
export function providerReschedule(orderNo: string, data: ReschedulePayload) {
  return request<RescheduleInfo>({
    url: `/api/v1/provider/orders/${orderNo}/reschedule`,
    method: 'POST',
    data,
    loading: true,
  })
}

/**
 * 服务方确认已看到住户的改期。
 * 确认后列表与详情上的提示条才会消失，所以不做 loading 遮罩——它是个轻动作。
 */
export function ackProviderReschedule(orderNo: string) {
  return request<{ ack: boolean }>({
    url: `/api/v1/provider/orders/${orderNo}/reschedule/ack`,
    method: 'POST',
  })
}

export function listRecords(orderNo: string) {
  return request<ServiceRecord[]>({ url: `/api/v1/orders/${orderNo}/records` })
}

export function monthlyDetail(orderNo: string) {
  return request<MonthlySummary>({ url: `/api/v1/orders/${orderNo}/monthly` })
}

/* ---------- 服务方资质申请 ---------- */

export interface ProviderApplication {
  id: number
  real_name: string
  phone: string
  service_type: string
  experience: string
  /** 已脱敏：310115********1234 */
  id_card_no: string
  status: 'pending' | 'approved' | 'rejected'
  status_label: string
  review_note: string
  reviewed_at: string | null
  created_at: string
  has_front: boolean
  has_back: boolean
  has_health: boolean
}

/** 查当前认证状态，data 为 null 表示从未申请 */
export function myProviderApplication() {
  return request<ProviderApplication | null>({ url: '/api/v1/provider/application' })
}

export interface SubmitApplicationPayload {
  /** 让本类型可作为通用 JSON 请求体（Record<string, unknown>）传给 request */
  [key: string]: unknown
  real_name: string
  phone: string
  service_type: string
  experience: string
  id_card_no?: string
  id_card_front_key?: string
  id_card_back_key?: string
  health_cert_key?: string
}

export function submitProviderApplication(data: SubmitApplicationPayload) {
  return request<{ id: number; status: string; status_label: string }>({
    url: '/api/v1/provider/application',
    method: 'POST',
    data,
    loading: true,
  })
}

/** 上传证件照，返回 object key 供提交申请时回传 */
export function uploadProviderCert(kind: 'front' | 'back' | 'health', filePath: string) {
  const token = getToken()
  const uid = getUserId()
  const header: Record<string, string> = token
    ? { Authorization: `Bearer ${token}` }
    : uid
      ? { 'X-User-Id': uid }
      : {}
  return new Promise<{ key: string }>((resolve, reject) => {
    uni.uploadFile({
      url: `${BASE_URL}/api/v1/provider/application/cert`,
      filePath,
      name: 'file',
      formData: { kind },
      header,
      success: (res) => {
        try {
          const body = JSON.parse(res.data) as { code: number; message: string; data?: { key: string } }
          if (body.code !== 0 || !body.data) {
            reject(new ApiError(body.code, body.message || '上传失败'))
            return
          }
          resolve(body.data)
        } catch {
          reject(new ApiError(-1, '上传响应解析失败'))
        }
      },
      fail: () => reject(new ApiError(-1, '上传失败，请重试')),
    })
  })
}

/* ---------- 服务方端 ---------- */

/** 可接单池：已支付且未派单的订单 */
export function providerOpenOrders(page = 1, size = 10) {
  return request<PageResp<Order>>({
    url: `/api/v1/provider/orders/open?page=${page}&size=${size}`,
  })
}

/** 服务方名下订单 */
export function providerMyOrders(tab: string, page = 1, size = 10) {
  return request<PageResp<Order>>({
    url: `/api/v1/provider/orders?tab=${tab}&page=${page}&size=${size}`,
  })
}

export function acceptOrder(orderNo: string) {
  return request<{ accepted: boolean }>({
    url: `/api/v1/provider/orders/${orderNo}/accept`,
    method: 'POST',
    loading: true,
  })
}

export function rejectOrder(orderNo: string, reason: string) {
  return request<{ rejected: boolean }>({
    url: `/api/v1/provider/orders/${orderNo}/reject`,
    method: 'POST',
    data: { reason },
    loading: true,
  })
}

/** 推进节点，node 取 on_the_way / in_service / pending_confirm */
export function advanceOrder(orderNo: string, node: string) {
  return request<{ node: string }>({
    url: `/api/v1/provider/orders/${orderNo}/advance`,
    method: 'POST',
    data: { node },
    loading: true,
  })
}

/** 月卡今日打卡：服务方上门当天点一次，返回最新进度 */
export function providerMonthlyRedeem(orderNo: string) {
  return request<MonthlyProgress>({
    url: `/api/v1/provider/orders/${orderNo}/monthly/redeem`,
    method: 'POST',
    loading: true,
  })
}

/**
 * 月卡本趟完成（「完成服务」）。
 * 与打卡分开：打卡记「人到了」，这一步才把今天这一趟收尾，
 * 收尾后卡片才会从待服务挪到已完成。
 */
export function providerMonthlyFinish(orderNo: string) {
  return request<MonthlyProgress>({
    url: `/api/v1/provider/orders/${orderNo}/monthly/finish`,
    method: 'POST',
    loading: true,
  })
}

/** 上传服务记录（图片）；stage 标记凭证阶段：start=开始服务 / finish=完成服务 / 空=过程记录 */
export function uploadRecord(
  orderNo: string,
  filePath: string,
  mediaType = 'photo',
  ownerConsented = true,
  stage = '',
) {
  return upload(`/api/v1/orders/${orderNo}/records`, filePath, {
    media_type: mediaType,
    duration_sec: '0',
    // 流程图 P2：服务中拍照需经主人同意，上传时随表单留痕
    owner_consented: ownerConsented ? 'true' : 'false',
    stage,
  })
}

export function providerIncome() {
  return request<ProviderIncome>({ url: '/api/v1/provider/income' })
}
