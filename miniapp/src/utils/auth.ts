import { useUserStore } from '@/store/user'

/**
 * 需要登录才能访问的页面（下单、订单、地址、设置、服务方端）。
 * 访客态在这些页面仍被拦截，只是跳转时带上原因，提示更明确。
 */
const AUTH_PAGES = [
  'pages/order/',
  'pages/address/',
  'pages/settings/',
  'pages/provider/',
]

/** 访客可浏览的页面：不拦，页面内自行降级展示 */
const GUEST_PAGES = ['pages/index/index', 'pages/mine/index', 'pages/doc/index']

function currentRoute(): { route: string; redirect: string } {
  const pages = getCurrentPages()
  const cur = pages.length ? pages[pages.length - 1] : null
  const route = (cur as { route?: string } | null)?.route || ''
  const options = ((cur as { options?: Record<string, string> } | null)?.options) || {}

  let redirect = route ? `/${route}` : '/pages/index/index'
  const qs = Object.keys(options)
    .filter((k) => options[k] !== undefined)
    .map((k) => `${k}=${encodeURIComponent(options[k])}`)
    .join('&')
  if (qs) redirect += `?${qs}`
  return { route, redirect }
}

/**
 * 页面级登录守卫。未登录时跳登录页并记住要回来的地址，返回 false。
 *
 * 用法：在页面 onShow / onLoad 的第一行调用
 *   if (!ensureAuth()) return
 *
 * 为什么不用 uni.addInterceptor：拦截器只能拦 navigateTo/reLaunch 这类主动跳转，
 * 无法拦住用户直接打开页面、或从 tab 切换过来的情况，页面级判断才是唯一可靠的位置。
 *
 * 访客态（guest）只放行浏览类页面；下单/订单等仍需登录，
 * 跳转时带上 reason=auth，让用户知道这次是「操作需要登录」而不是莫名被踢。
 */
export function ensureAuth(): boolean {
  const user = useUserStore()
  if (user.isLoggedIn) return true

  const { route, redirect } = currentRoute()

  // 已经在登录/注册等公开页时不再重复跳，避免死循环
  if (route.startsWith('pages/auth/')) return true

  const needAuth = AUTH_PAGES.some((p) => route.startsWith(p))
  const canGuest = GUEST_PAGES.some((p) => route.startsWith(p))

  // 访客态且当前页允许浏览：放行，页面自己处理无数据的情况
  if (user.isGuest && canGuest && !needAuth) return true

  const reason = needAuth ? '&reason=auth' : ''
  uni.reLaunch({ url: `/pages/auth/login?redirect=${encodeURIComponent(redirect)}${reason}` })
  return false
}

/**
 * 受限操作前调用（如点「立即预约」）：未登录时弹窗询问再去登录，返回 false。
 * 给用户一个「再逛逛」的退路，比直接跳走体验好，也符合访客模式的初衷。
 */
export function requireLogin(): boolean {
  const user = useUserStore()
  if (user.isLoggedIn) return true

  const { redirect } = currentRoute()
  uni.showModal({
    title: '需要登录',
    content: '登录后才能预约服务，是否前往登录？',
    confirmText: '去登录',
    cancelText: '再逛逛',
    success: (r) => {
      if (r.confirm) {
        uni.reLaunch({ url: `/pages/auth/login?redirect=${encodeURIComponent(redirect)}&reason=auth` })
      }
    },
  })
  return false
}
