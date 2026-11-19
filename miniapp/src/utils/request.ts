/**
 * 请求层：统一处理地址拼接、身份注入、响应解包与错误提示。
 *
 * 后端约定：HTTP 状态码只表达协议层结果，业务结果看 body.code。
 * 所以这里不按 HTTP 状态码判断成败，只有网络异常才走 catch。
 */

/** 统一响应体 */
export interface ApiBody<T = unknown> {
  code: number
  message: string
  request_id?: string
  data?: T
}

/** 业务错误：code 来自后端 errs 包 */
export class ApiError extends Error {
  code: number

  constructor(code: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.code = code
  }

  /** 是否为「地址不在服务范围」 */
  get isOutOfArea() {
    return this.code === 2001
  }

  /** 是否为「时段已约满」 */
  get isSlotFull() {
    return this.code === 2002
  }
}

/**
 * H5 开发留空，走 vite proxy 同源转发，免去跨域配置；
 * 小程序端没有 proxy，必须用完整地址（开发者工具里勾「不校验合法域名」）。
 * 真机联调时把 VITE_API_BASE 换成局域网 IP，例如 http://192.168.1.10:8080。
 */
// uni/vite 在编译期把 process.env.UNI_PLATFORM 替换成常量，这里仅补类型声明（项目未引入 @types/node）
declare const process: { env: Record<string, string | undefined> }
const isH5 = process.env.UNI_PLATFORM === 'h5'
export const BASE_URL =
  (import.meta.env.VITE_API_BASE as string) || (isH5 ? '' : 'http://127.0.0.1:8080')

const KEY_USER_ID = 'ch_user_id'

export function getUserId(): string {
  return (uni.getStorageSync(KEY_USER_ID) as string) || ''
}

export function setUserId(id: string | number) {
  uni.setStorageSync(KEY_USER_ID, String(id))
}

export function clearUserId() {
  uni.removeStorageSync(KEY_USER_ID)
}

interface RequestOptions {
  url: string
  method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'
  data?: Record<string, unknown>
  /** 是否需要用户身份，默认 true */
  auth?: boolean
  /** 是否显示全局 loading，默认 false */
  loading?: boolean
}

/**
 * 发起请求并解包 data。
 * 业务错误统一抛 ApiError，调用方用 try/catch 或 .catch 处理；
 * 未捕获时页面会白屏，所以每个调用点都要给出用户可读的提示。
 */
export function request<T>(opts: RequestOptions): Promise<T> {
  const { url, method = 'GET', data, auth = true, loading = false } = opts
  const header: Record<string, string> = { 'Content-Type': 'application/json' }

  if (auth) {
    const uid = getUserId()
    if (uid) header['X-User-Id'] = uid
  }

  if (loading) uni.showLoading({ title: '加载中', mask: true })

  return new Promise<T>((resolve, reject) => {
    uni.request({
      url: BASE_URL + url,
      // uni 的 method 类型定义漏了 PATCH，底层 wx.request/XHR 实际支持，收窄到其声明范围
      method: method as Exclude<RequestOptions['method'], 'PATCH'>,
      data,
      header,
      timeout: 15000,
      success: (res) => {
        const body = res.data as ApiBody<T>

        // 非 2xx 且没有统一结构，视为网关/服务异常
        if (!body || typeof body.code !== 'number') {
          reject(new ApiError(res.statusCode || 0, `服务异常（HTTP ${res.statusCode}）`))
          return
        }
        if (body.code !== 0) {
          reject(new ApiError(body.code, body.message || '请求失败'))
          return
        }
        resolve(body.data as T)
      },
      fail: (err) => {
        reject(new ApiError(-1, err.errMsg?.includes('timeout') ? '网络超时，请重试' : '网络异常，请检查网络'))
      },
      complete: () => {
        if (loading) uni.hideLoading()
      },
    })
  })
}

/**
 * 上传文件（multipart）。
 * 单独封装而不复用 request，因为 uni.uploadFile 的入参结构不同。
 */
export function upload(url: string, filePath: string, formData: Record<string, string>): Promise<ApiBody> {
  const uid = getUserId()
  return new Promise((resolve, reject) => {
    uni.uploadFile({
      url: BASE_URL + url,
      filePath,
      name: 'file',
      formData,
      header: uid ? { 'X-User-Id': uid } : {},
      success: (res) => {
        const body = JSON.parse(res.data) as ApiBody
        if (body.code !== 0) {
          reject(new ApiError(body.code, body.message))
          return
        }
        resolve(body)
      },
      fail: () => reject(new ApiError(-1, '上传失败，请重试')),
    })
  })
}
