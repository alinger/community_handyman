/**
 * 运营后台接口封装。字段与 server/docs/swagger.json 及 dto/model 对齐。
 * 统一响应 { code, message, data }，code=0 为成功；分页 data={list,total,page,size}。
 */
import axios, { type InternalAxiosRequestConfig, type AxiosResponse } from 'axios'
import { ElMessage } from 'element-plus'

const TOKEN_KEY = 'ch_admin_token'

export function getToken(): string {
  return localStorage.getItem(TOKEN_KEY) || ''
}
export function setToken(token: string): void {
  localStorage.setItem(TOKEN_KEY, token)
}
export function clearToken(): void {
  localStorage.removeItem(TOKEN_KEY)
}

interface ApiBody<T> {
  code: number
  message: string
  request_id?: string
  data?: T
}

export interface PageData<T> {
  list: T[]
  total: number
  page: number
  size: number
}

export class ApiError extends Error {
  constructor(public code: number, message: string) {
    super(message)
  }
}

const http = axios.create({
  baseURL: '/',
  timeout: 15000,
})

http.interceptors.request.use((cfg: InternalAxiosRequestConfig) => {
  const token = getToken()
  if (token) cfg.headers.set('Authorization', `Bearer ${token}`)
  return cfg
})

http.interceptors.response.use(
  (res: AxiosResponse<ApiBody<unknown>>) => {
    const body = res.data
    if (body && typeof body === 'object' && 'code' in body) {
      if (body.code === 0) return res
      return Promise.reject(new ApiError(body.code, body.message || '请求失败'))
    }
    return Promise.reject(new ApiError(-1, '响应格式异常'))
  },
  (err) => {
    const status = err?.response?.status
    const body = err?.response?.data as ApiBody<unknown> | undefined
    if (status === 401) {
      clearToken()
      ElMessage.error('登录已过期，请重新登录')
      if (location.hash !== '#/login') location.hash = '#/login'
    } else if (body?.message) {
      ElMessage.error(body.message)
    } else {
      ElMessage.error(err?.message || '网络异常')
    }
    return Promise.reject(err)
  },
)

async function request<T>(url: string, method = 'GET', data?: unknown): Promise<T> {
  const res = await http.request<ApiBody<T>>({ url, method, data })
  return res.data.data as T
}

// ---------- 类型 ----------

export interface AdminUser {
  id: number
  username: string
  role: string
}
export interface LoginResp {
  token: string
  expires_in: number
  admin: AdminUser
}

export interface ServiceArea {
  id: number
  community: string
  lane: string
  house_from: number
  house_to: number
  house_parity: string
  status: number
  effective_at?: string | null
}
export interface CreateAreaPayload {
  community: string
  lane: string
  house_from: number
  house_to: number
  house_parity: 'all' | 'odd' | 'even'
}

export interface SlotInventory {
  id: number
  service_date: string
  slot_code: string
  service_type: string
  total: number
  used: number
}
export interface SetSlotItem {
  slot_code: string
  service_type: string
  total: number
}

export interface ServiceSku {
  id: number
  service_type: string
  plan: string
  base_price: number
  rules: Record<string, unknown>
  slots: unknown
  status: number
}

export interface ProviderOption {
  id: number
  nickname: string
  openid: string
}

export interface PriceItem {
  name: string
  amount: number
  note?: string
}
export interface OrderNode {
  node: string
  label: string
  reached: boolean
  at?: string | null
  remark?: string
}
export interface AddressResp {
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
  address?: AddressResp
  price_items?: PriceItem[]
  hospital?: string
  patient_name?: string
  accompany_items?: PriceItem[]
  quote?: { amount: number; duration_min: number; confirmed: boolean; created_at: string }
  // 后台扩展字段
  user_id?: number
  user_nickname?: string
  provider_id?: number | null
  provider_nickname?: string
  quantity?: number
  pet_count?: number
  refunded_amount?: number
  cancel_reason?: string
  paid_at?: string
  finished_at?: string
  monthly_start?: string
  monthly_end?: string
}

// ---------- 接口 ----------

export const login = (username: string, password: string) =>
  request<LoginResp>('/api/admin/login', 'POST', { username, password })

export const listAreas = () => request<ServiceArea[]>('/api/admin/areas')
export const createArea = (data: CreateAreaPayload) => request<ServiceArea>('/api/admin/areas', 'POST', data)
export const setAreaStatus = (id: number, status: 0 | 1) =>
  request<{ id: number; status: number }>(`/api/admin/areas/${id}/status`, 'PATCH', { status })

export const listSlots = (date: string) => request<SlotInventory[]>(`/api/admin/slots?date=${encodeURIComponent(date)}`)
export const setSlots = (date: string, items: SetSlotItem[]) =>
  request<SlotInventory[]>('/api/admin/slots', 'PUT', { date, items })

export const listSkus = () => request<ServiceSku[]>('/api/admin/skus')
export const updateSku = (id: number, basePrice: number, rules: Record<string, unknown>) =>
  request<{ id: number; base_price: number }>(`/api/admin/skus/${id}`, 'PUT', { base_price: basePrice, rules })

export const listProviders = () => request<ProviderOption[]>('/api/admin/providers')

export interface OrderQuery {
  status?: string
  service_type?: string
  keyword?: string
  page?: number
  size?: number
}
export const listOrders = (q: OrderQuery) => {
  const params = new URLSearchParams()
  Object.entries(q).forEach(([k, v]) => {
    if (v !== undefined && v !== '' && v !== null) params.set(k, String(v))
  })
  return request<PageData<Order>>(`/api/admin/orders?${params.toString()}`)
}
export const assignOrder = (orderNo: string, providerId: number) =>
  request<{ assigned: boolean; provider_id: number }>(`/api/admin/orders/${orderNo}/assign`, 'POST', { provider_id: providerId })
export const refund = (orderNo: string, amount: number, reason: string) =>
  request<{ out_refund_no: string; amount: number; status: string }>('/api/admin/refunds', 'POST', {
    order_no: orderNo,
    amount,
    reason,
  })
export const redeem = (orderNo: string, date: string, remark: string) =>
  request<unknown>(`/api/admin/orders/${orderNo}/redeem`, 'POST', { date, remark })
export const quote = (orderNo: string, amount: number, durationMin: number, note: string) =>
  request<unknown>(`/api/admin/orders/${orderNo}/quote`, 'POST', {
    amount,
    duration_min: durationMin,
    note,
  })

// ---------- 财务：支付流水 / 退款单 ----------

export interface AdminPayment {
  id: number
  order_no: string
  transaction_id: string
  out_trade_no: string
  amount: number
  status: string
  paid_at: string | null
  created_at: string
}
export interface AdminRefund {
  id: number
  order_no: string
  out_refund_no: string
  refund_id: string
  amount: number
  reason: string
  status: string
  operator_id: number
  refunded_at: string | null
  created_at: string
}

export const listPayments = (q: { order_no?: string; status?: string; page?: number; size?: number }) => {
  const p = new URLSearchParams()
  Object.entries(q).forEach(([k, v]) => {
    if (v !== undefined && v !== '') p.set(k, String(v))
  })
  return request<PageData<AdminPayment>>(`/api/admin/payments?${p.toString()}`)
}
export const listRefunds = (q: { order_no?: string; status?: string; page?: number; size?: number }) => {
  const p = new URLSearchParams()
  Object.entries(q).forEach(([k, v]) => {
    if (v !== undefined && v !== '') p.set(k, String(v))
  })
  return request<PageData<AdminRefund>>(`/api/admin/refunds?${p.toString()}`)
}

// ---------- 订单时间线 / 服务记录 ----------

export interface AdminEvent {
  id: number
  node: string
  operator: string
  operator_id: number | null
  remark: string
  created_at: string
}
export const listOrderEvents = (orderNo: string) =>
  request<AdminEvent[]>(`/api/admin/orders/${orderNo}/events`)

export interface AdminRecord {
  id: number
  media_type: string
  size: number
  duration_sec: number
  uploader_role: string
  created_at: string
  url: string
}
export const listOrderRecords = (orderNo: string) =>
  request<AdminRecord[]>(`/api/admin/orders/${orderNo}/records`)
/** 带 Bearer 拉取记录文件，返回浏览器可用的 blob URL（关闭预览后需 revoke） */
export const fetchRecordBlobUrl = async (r: AdminRecord): Promise<string> => {
  const res = await http.get<Blob>(r.url, { responseType: 'blob' })
  return URL.createObjectURL(res.data)
}

// ---------- 数据看板 ----------

export interface AdminStats {
  today_orders: number
  today_gmv: number
  today_refunded: number
  pending_accept: number
  unassigned: number
  in_service: number
  total_users: number
  providers: number
  status_dist: { status: string; count: number }[]
  service_dist: { service_type: string; count: number }[]
  recent_days?: { date: string; orders: number; gmv: number }[]
}
export const getStats = () => request<AdminStats>('/api/admin/stats')

// ---------- 用户管理 ----------

export interface AdminUserRow {
  id: number
  nickname: string
  openid: string
  phone: string
  role: string
  status: number
  order_count: number
  created_at: string
}
export const listUsers = (q: { role?: string; keyword?: string; page?: number; size?: number }) => {
  const p = new URLSearchParams()
  Object.entries(q).forEach(([k, v]) => {
    if (v !== undefined && v !== '') p.set(k, String(v))
  })
  return request<PageData<AdminUserRow>>(`/api/admin/users?${p.toString()}`)
}
export const setUserRole = (id: number, role: 'user' | 'provider') =>
  request<{ id: number; role: string }>(`/api/admin/users/${id}/role`, 'PATCH', { role })
export const setUserStatus = (id: number, status: 0 | 1) =>
  request<{ id: number; status: number }>(`/api/admin/users/${id}/status`, 'PATCH', { status })

// ---------- 月卡核销 / 区域编辑 / 时段复制 / 导出 ----------

export interface AdminRedemption {
  id: number
  order_no: string
  service_date: string
  start_date?: string
  end_date?: string
  operator_id: number
  remark: string
  created_at: string
}
export const listRedemptions = (q: { date?: string; page?: number; size?: number }) => {
  const p = new URLSearchParams()
  Object.entries(q).forEach(([k, v]) => {
    if (v !== undefined && v !== '') p.set(k, String(v))
  })
  return request<PageData<AdminRedemption>>(`/api/admin/redemptions?${p.toString()}`)
}

export const updateArea = (
  id: number,
  data: CreateAreaPayload,
) => request<{ id: number }>(`/api/admin/areas/${id}`, 'PUT', data)

export const copySlots = (sourceDate: string, targetDates: string[]) =>
  request<{ applied: number }>('/api/admin/slots/copy', 'POST', {
    source_date: sourceDate,
    target_dates: targetDates,
  })

export const exportOrdersUrl = (q: { status?: string; service_type?: string; keyword?: string }) => {
  const p = new URLSearchParams()
  Object.entries(q).forEach(([k, v]) => {
    if (v !== undefined && v !== '') p.set(k, String(v))
  })
  return `/api/admin/export/orders?${p.toString()}`
}

// ---------- 后台账号管理 ----------

export interface AdminAccount {
  id: number
  username: string
  role: string
  status: number
  created_at: string
}
export const listAccounts = () => request<AdminAccount[]>('/api/admin/accounts')
export const createAccount = (username: string, password: string, role: 'admin' | 'operator') =>
  request<AdminAccount>('/api/admin/accounts', 'POST', { username, password, role })
export const setAccountStatus = (id: number, status: 0 | 1) =>
  request<{ id: number; status: number }>(`/api/admin/accounts/${id}/status`, 'PATCH', { status })
export const resetAccountPassword = (id: number, password: string) =>
  request<{ id: number }>(`/api/admin/accounts/${id}/password`, 'PATCH', { password })
export const changeMyPassword = (oldPassword: string, newPassword: string) =>
  request<{ changed: boolean }>('/api/admin/password', 'PATCH', {
    old_password: oldPassword,
    new_password: newPassword,
  })

// ---------- 代客取消 / 改期 ----------

export const cancelOrderForUser = (orderNo: string, reason: string) =>
  request<{ canceled: boolean; refund_due: number; refund_ok?: boolean; refund_hint?: string }>(
    `/api/admin/orders/${orderNo}/cancel`, 'POST', { reason })
export const rescheduleOrder = (orderNo: string, serviceDate: string, slotCode: string) =>
  request<{ rescheduled: boolean; service_date: string; slot_code: string }>(
    `/api/admin/orders/${orderNo}/reschedule`, 'POST', { service_date: serviceDate, slot_code: slotCode })

// ---------- 服务方资质审核 ----------

export interface ProviderAppRow {
  id: number
  user_id: number
  user_nickname: string
  real_name: string
  phone: string
  service_type: string
  experience: string
  id_card_no: string
  status: 'pending' | 'approved' | 'rejected'
  status_label: string
  review_note: string
  reviewed_at: string
  created_at: string
  has_front: boolean
  has_back: boolean
  has_health: boolean
}

export const listProviderApps = (q: { status?: string; page?: number; size?: number }) => {
  const p = new URLSearchParams()
  Object.entries(q).forEach(([k, v]) => {
    if (v !== undefined && v !== '') p.set(k, String(v))
  })
  return request<{ list: ProviderAppRow[]; total: number; page: number; size: number }>(
    `/api/admin/provider-applications?${p.toString()}`,
  )
}

export const providerAppStats = () =>
  request<{ pending: number; approved: number; rejected: number }>(
    '/api/admin/provider-applications/stats',
  )

export const providerAppDetail = (id: number) =>
  request<ProviderAppRow>(`/api/admin/provider-applications/${id}`)

export const approveProviderApp = (id: number) =>
  request<{ approved: boolean }>(`/api/admin/provider-applications/${id}/approve`, 'POST')

export const rejectProviderApp = (id: number, note: string) =>
  request<{ rejected: boolean }>(`/api/admin/provider-applications/${id}/reject`, 'POST', { note })

/** 附件直链：需要 Authorization，配合 fetch 转 blob 或带 header 打开 */
export const providerCertUrl = (id: number, kind: 'front' | 'back' | 'health') =>
  `/api/admin/provider-applications/${id}/cert/${kind}`
