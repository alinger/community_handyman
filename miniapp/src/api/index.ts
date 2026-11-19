/**
 * 接口封装。字段与 server/docs/swagger.json 保持一致，
 * 后端改字段时这里要同步，否则问题是运行期才暴露。
 */
import { request, upload } from '@/utils/request'

export interface Address {
  id: number
  community: string
  lane: string
  house_no: string
  extra: string
  contact_name: string
  contact_phone: string
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
}

export interface PriceItem {
  name: string
  amount: number
  note?: string
}

export interface PreviewResp {
  total: number
  items: PriceItem[]
  manual_quote: boolean
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
  accompany_items?: PriceItem[]
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
  uploader_role: string
  created_at: string
  url: string
}

export interface MonthlySummary {
  total: number
  used: number
  remaining: number
  start_date: string
  end_date: string
  redemptions: { id: number; service_date: string; remark: string }[]
}

export interface CreateOrderPayload {
  /** 让本类型可作为通用 JSON 请求体（Record<string, unknown>）传给 request */
  [key: string]: unknown
  address_id: number
  service_type: string
  plan?: string
  service_date: string
  slot_code: string
  quantity?: number
  pet_count?: number
  duration_min?: number
  distance_km?: number
  garbage_type?: string
  remark?: string
  feed_only?: boolean
  need_familiar?: boolean
  /** 陪诊专用：只回传 code，价格由后端按 SKU 配置计算 */
  hospital?: string
  patient_name?: string
  service_items?: string[]
}

/**
 * 本地联调登录，仅 dev 环境可用。
 * role 用于把 openid 对应的账号设成服务方，开发期身份切换靠它。
 */
export function devLogin(openid: string, nickname: string, role?: string) {
  return request<{ user_id: number; openid: string; nickname: string; role: string }>({
    url: '/api/v1/dev/login',
    method: 'POST',
    data: { openid, nickname, role },
    auth: false,
  })
}

export function listAddresses() {
  return request<Address[]>({ url: '/api/v1/addresses' })
}

export function createAddress(data: {
  community: string
  lane: string
  house_no: string
  extra?: string
  contact_name?: string
  contact_phone?: string
}) {
  return request<Address>({ url: '/api/v1/addresses', method: 'POST', data })
}

/** 陪诊可选项：医院与可多选服务项（含单价），由后台配置 */
export function accompanyOptions() {
  return request<AccompanyOptions>({ url: '/api/v1/services/accompany/options' })
}

export function checkArea(data: { community: string; lane: string; house_no: string }) {
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

export function listRecords(orderNo: string) {
  return request<ServiceRecord[]>({ url: `/api/v1/orders/${orderNo}/records` })
}

export function monthlyDetail(orderNo: string) {
  return request<MonthlySummary>({ url: `/api/v1/orders/${orderNo}/monthly` })
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

/** 上传服务记录（图片） */
export function uploadRecord(orderNo: string, filePath: string, mediaType = 'photo') {
  return upload(`/api/v1/orders/${orderNo}/records`, filePath, {
    media_type: mediaType,
    duration_sec: '0',
  })
}
