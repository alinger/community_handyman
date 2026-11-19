/**
 * 接口封装。字段与 server/docs/swagger.json 保持一致，
 * 后端改字段时这里要同步，否则问题是运行期才暴露。
 */
import { request } from '@/utils/request'

export interface Address {
  id: number
  community: string
  lane: string
  house_no: string
  extra: string
  contact_phone: string
  in_service_area: boolean
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
}

/** 本地联调登录，仅 dev 环境可用 */
export function devLogin(openid: string, nickname: string) {
  return request<{ user_id: number; openid: string; nickname: string; role: string }>({
    url: '/api/v1/dev/login',
    method: 'POST',
    data: { openid, nickname },
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
  contact_phone?: string
}) {
  return request<Address>({ url: '/api/v1/addresses', method: 'POST', data })
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
