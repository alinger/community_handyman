import { useUserStore } from '@/store/user'

/**
 * 页面级登录守卫。未登录时跳登录页并记住要回来的地址，返回 false。
 *
 * 用法：在页面 onShow / onLoad 的第一行调用
 *   if (!ensureAuth()) return
 *
 * 为什么不用 uni.addInterceptor：拦截器只能拦 navigateTo/reLaunch 这类主动跳转，
 * 无法拦住用户直接打开页面、或从 tab 切换过来的情况，页面级判断才是唯一可靠的位置。
 */
export function ensureAuth(): boolean {
  const user = useUserStore()
  if (user.isLoggedIn) return true

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

  // 已经在登录页时不再重复跳，避免死循环
  if (route.startsWith('pages/auth/')) return true

  uni.reLaunch({ url: `/pages/auth/login?redirect=${encodeURIComponent(redirect)}` })
  return false
}
