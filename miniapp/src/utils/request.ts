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

/** 构建环境标识（本地 / 生产），仅用于界面提示与排查，不参与业务逻辑 */
export const APP_ENV = (import.meta.env.VITE_APP_ENV as string | undefined)?.trim() || ''

/**
 * 接口根地址，由构建模式决定（见 .env.development / .env.production）：
 *   VITE_API_BASE 有值 → 用它（生产固定 https://m.yubangbzz.com）
 *   VITE_API_BASE 为空 → H5 留空走同源 proxy（免跨域）；小程序回落到本机 8080
 * 临时连测试服：VITE_API_BASE=https://test.xxx.com npm run build:mp-weixin
 */
const BUILD_BASE = (import.meta.env.VITE_API_BASE as string) || (isH5 ? 'http://127.0.0.1:8080' : 'https://m.yubangbzz.com')
// const BUILD_BASE = (import.meta.env.VITE_API_BASE as string) || (isH5 ? '' : 'http://127.0.0.1:8080')

/** 构建期的环境名（本地 / 生产），自定义构建可能为空 */
const BUILD_ENV_NAME = APP_ENV || '构建默认'

/**
 * 接口根地址。用 `export let` 而不是 const：运行期允许在「我的 → 接口环境」里临时覆盖
 * （真机连开发机、体验版切测试服），所有请求与资源拼址都读这个变量。
 * 注意：ESM 的 import 是活绑定，页面里 `${BASE_URL}xxx` 在切换后重新渲染就能拿到新值。
 */
export let BASE_URL = BUILD_BASE

/** 可选环境：build=跟随构建（默认）；prod/local=固定预设；custom=手填地址 */
export type ApiEnvMode = 'build' | 'prod' | 'local' | 'custom'

const KEY_API_ENV = 'ch_api_env'
const KEY_ENV_ENTRY = 'ch_env_entry'

/** 生产与本地预设。生产写死域名，避免手抖把正式包指到测试服 */
const PROD_BASE = 'https://m.yubangbzz.com'
const LOCAL_BASE = 'http://127.0.0.1:8080'

const ENV_MODES: ApiEnvMode[] = ['build', 'prod', 'local', 'custom']

/** 用户可能只填了 IP:PORT，补上协议；末尾斜杠去掉，避免拼出 //api */
function normalizeBase(raw: string): string {
  const s = (raw || '').trim().replace(/\/+$/, '')
  if (!s) return ''
  return /^https?:\/\//i.test(s) ? s : `http://${s}`
}

function readEnvStore(): { mode: ApiEnvMode; custom: string } {
  const raw = uni.getStorageSync(KEY_API_ENV) as string
  if (!raw) return { mode: 'build', custom: '' }
  try {
    const o = JSON.parse(raw) as { mode?: string; custom?: string }
    const mode = (o.mode || 'build') as ApiEnvMode
    return {
      mode: ENV_MODES.includes(mode) ? mode : 'build',
      custom: normalizeBase(o.custom || ''),
    }
  } catch {
    // 存储被写坏时退回构建默认，不让调试开关把 App 卡死
    return { mode: 'build', custom: '' }
  }
}

function baseOf(st: { mode: ApiEnvMode; custom: string }): string {
  if (st.mode === 'prod') return PROD_BASE
  if (st.mode === 'local') return LOCAL_BASE
  if (st.mode === 'custom') return st.custom || BUILD_BASE
  return BUILD_BASE
}

/** 启动即应用上次选的环境，保证首屏请求就走对地址 */
function applyStoredEnv() {
  const st = readEnvStore()
  if (st.mode !== 'build') BASE_URL = baseOf(st)
}
applyStoredEnv()

export interface ApiEnvInfo {
  mode: ApiEnvMode
  /** 实际请求根地址；空串表示 H5 同源（走 vite proxy） */
  base: string
  /** 用于界面展示的短名：生产 / 本地 / 自定义 / 构建默认 */
  label: string
  /** 展示用的地址：去协议，空地址显示「同源 /api」 */
  short: string
}

/** 当前生效的环境（每次都从存储读，切完立刻准确） */
export function apiEnv(): ApiEnvInfo {
  const st = readEnvStore()
  const base = baseOf(st)
  const label =
    st.mode === 'prod'
      ? '生产'
      : st.mode === 'local'
        ? '本地'
        : st.mode === 'custom'
          ? '自定义'
          : BUILD_ENV_NAME
  return { mode: st.mode, base, label, short: base ? base.replace(/^https?:\/\//i, '') : '同源 /api' }
}

/** 切换环境：写存储 + 立刻改运行时地址（不重启进程，但已加载的页面数据不会自动刷新） */
export function setApiEnv(mode: ApiEnvMode, custom = ''): ApiEnvInfo {
  const st = { mode, custom: normalizeBase(custom) }
  uni.setStorageSync(KEY_API_ENV, JSON.stringify(st))
  BASE_URL = baseOf(st)
  return apiEnv()
}

/** 恢复成构建时决定的环境 */
export function resetApiEnv(): ApiEnvInfo {
  uni.removeStorageSync(KEY_API_ENV)
  BASE_URL = BUILD_BASE
  return apiEnv()
}

/** 调试入口是否显示（默认关闭，连点版本号 5 次打开，只影响本机） */
export function envEntryOn(): boolean {
  return uni.getStorageSync(KEY_ENV_ENTRY) === '1'
}

export function setEnvEntryOn(on: boolean) {
  if (on) uni.setStorageSync(KEY_ENV_ENTRY, '1')
  else uni.removeStorageSync(KEY_ENV_ENTRY)
}

/** 生产域名，供「切到生产」时做二次确认文案 */
export const PROD_API_BASE = PROD_BASE


const KEY_USER_ID = 'ch_user_id'
const KEY_TOKEN = 'ch_token'

export function getUserId(): string {
  return (uni.getStorageSync(KEY_USER_ID) as string) || ''
}

export function setUserId(id: string | number) {
  uni.setStorageSync(KEY_USER_ID, String(id))
}

export function clearUserId() {
  uni.removeStorageSync(KEY_USER_ID)
}

/** 登录 token。存在时请求优先用 Bearer，X-User-Id 只作开发联调兜底 */
export function getToken(): string {
  return (uni.getStorageSync(KEY_TOKEN) as string) || ''
}

export function setToken(token: string) {
  if (!token) uni.removeStorageSync(KEY_TOKEN)
  else uni.setStorageSync(KEY_TOKEN, token)
}

export function clearToken() {
  uni.removeStorageSync(KEY_TOKEN)
}

/** 登录态失效（1002 / HTTP 401）时统一清理并跳登录页 */
let redirectingToLogin = false

export function handleUnauthorized() {
  clearToken()
  clearUserId()
  if (redirectingToLogin) return
  redirectingToLogin = true
  uni.showToast({ title: '登录已失效，请重新登录', icon: 'none' })
  setTimeout(() => {
    uni.reLaunch({
      url: '/pages/auth/login',
      complete: () => {
        redirectingToLogin = false
      },
    })
  }, 600)
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
    const token = getToken()
    if (token) {
      header['Authorization'] = `Bearer ${token}`
    } else {
      // 没有 token 时用 X-User-Id 走联调通道（后端仅非生产环境接受）
      const uid = getUserId()
      if (uid) header['X-User-Id'] = uid
    }
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
        if (body.code === 1002 || res.statusCode === 401) {
          // 登录态失效：清本地凭证并回登录页，避免用户停在空白页反复重试
          if (auth) handleUnauthorized()
          reject(new ApiError(body.code || 1002, body.message || '登录已失效'))
          return
        }
        if (body.code !== 0) {
          reject(new ApiError(body.code, body.message || '请求失败'))
          return
        }
        resolve(body.data as T)
      },
      fail: (err) => {
        const msg = err.errMsg || ''
        // 只抛统一文案会把真实原因吞掉：域名白名单、连接被拒、超时在调用方看来一模一样。
        // 控制台留一份原始 errMsg + 实际地址，排查时不用再猜。
        console.warn('[request] fail', BASE_URL + url, msg)
        if (msg.includes('timeout')) {
          reject(new ApiError(-1, '网络超时，请重试'))
          return
        }
        // 开发者工具没勾「不校验合法域名」时的典型报错，和真连不上要区分开
        if (msg.includes('domain list') || msg.includes('not in domain')) {
          reject(new ApiError(-1, '接口域名未加入白名单，请在开发者工具勾选「不校验合法域名」'))
          return
        }
        reject(new ApiError(-1, '网络异常，请检查网络'))
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
  const token = getToken()
  const uid = getUserId()
  const authHeader: Record<string, string> = token
    ? { Authorization: `Bearer ${token}` }
    : uid
      ? { 'X-User-Id': uid }
      : {}
  return new Promise((resolve, reject) => {
    uni.uploadFile({
      url: BASE_URL + url,
      filePath,
      name: 'file',
      formData,
      header: authHeader,
      success: (res) => {
        const body = JSON.parse(res.data) as ApiBody
        if (body.code === 1002) {
          handleUnauthorized()
          reject(new ApiError(body.code, body.message || '登录已失效'))
          return
        }
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
