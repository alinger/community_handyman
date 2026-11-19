import { defineStore } from 'pinia'
import {
  deleteAddress as apiDeleteAddress,
  getMe,
  listAddresses,
  login as apiLogin,
  register as apiRegister,
  setDefaultAddress as apiSetDefault,
  updateMe,
  type Session,
} from '@/api'
import type { Address } from '@/api'
import {
  getToken,
  getUserId,
  setToken,
  setUserId,
  clearToken,
  clearUserId,
} from '@/utils/request'

const KEY_ROLE = 'ch_role'

/**
 * 用户与地址状态。
 *
 * 登录态：正式流程走手机号 + 密码（/auth/login、/auth/register）拿 token，
 * 请求层用 Authorization: Bearer 带上。token 与 userId 都落在本地缓存，
 * 冷启动时 isLoggedIn 直接读缓存判断，不需要先请求一次后端。
 *
 * 开发便利：H5 地址栏带 ?as=provider|user 时会自动走 /dev/login 造出对应身份，
 * 该路由后端只在 dev 注册，生产环境自然失效。
 */
export const useUserStore = defineStore('user', {
  state: () => ({
    userId: getUserId(),
    token: getToken(),
    nickname: '',
    phone: '',
    role: (uni.getStorageSync(KEY_ROLE) as string) || 'user',
    addresses: [] as Address[],
    currentAddressId: 0,
    ready: false,
    loginPromise: null as Promise<void> | null,
  }),

  getters: {
    /** 是否已登录：有 token 才算，光有 userId 是历史联调残留 */
    isLoggedIn(state): boolean {
      return !!state.token && !!state.userId
    },
    currentAddress(state): Address | undefined {
      return state.addresses.find((a) => a.id === state.currentAddressId) || state.addresses[0]
    },
    hasAddress(state): boolean {
      return state.addresses.length > 0
    },
  },

  actions: {
    /**
     * 确保已登录。未登录时抛错，由调用页决定跳登录页。
     * App.onLaunch 与页面 onShow 会并发调用，这里用同一个 Promise 去重。
     */
    async ensureLogin() {
      if (this.isLoggedIn) return
      if (!this.loginPromise) {
        this.loginPromise = this.restoreSession().finally(() => {
          this.loginPromise = null
        })
      }
      await this.loginPromise
    },

    /**
     * 恢复会话：只认本地已有 token，不再有任何自动登录通道。
     *
     * 服务方身份只能来自后台审核通过后 role 被置为 provider，
     * 所以这里不存在「自己切身份」的入口 —— 没有 token 就是未登录，
     * 由页面守卫跳登录页。
     */
    async restoreSession() {
      if (this.isLoggedIn) return
      // 有 userId 但没 token：历史联调残留，清掉后按未登录处理
      if (this.userId && !this.token) {
        this.userId = ''
        clearUserId()
      }
      if (!this.token) {
        throw new Error('未登录')
      }
    },

    /** 应用正式登录/注册返回的会话 */
    applySession(s: Session) {
      this.token = s.token
      this.userId = String(s.user.user_id)
      this.nickname = s.user.nickname || ''
      this.phone = s.user.phone || ''
      this.role = s.user.role || 'user'
      setToken(s.token)
      setUserId(s.user.user_id)
      uni.setStorageSync(KEY_ROLE, this.role)
      this.ready = true
    },

    async login(phone: string, password: string) {
      const s = await apiLogin({ phone, password })
      this.applySession(s)
      return s
    },

    async register(phone: string, password: string, nickname: string) {
      const s = await apiRegister({ phone, password, nickname })
      this.applySession(s)
      return s
    },

    /** 加载地址列表，默认项以后端 is_default 为准（列表已按默认优先排序） */
    async loadAddresses() {
      const list = await listAddresses()
      this.addresses = list || []
      const def = this.addresses.find((a) => a.is_default)
      const stillExists = this.addresses.some((a) => a.id === this.currentAddressId)
      if (def) {
        this.currentAddressId = def.id
      } else if (!stillExists && this.addresses.length) {
        this.currentAddressId = this.addresses[0].id
      }
      return this.addresses
    },

    setCurrentAddress(id: number) {
      this.currentAddressId = id
    },

    /** 设为默认地址：持久化到后端，并用返回列表刷新本地状态 */
    async setDefaultAddress(id: number) {
      this.addresses = await apiSetDefault(id)
      this.currentAddressId = id
      return this.addresses
    },

    /** 删除地址：后端返回剩余列表，删默认地址时会自动提升下一条 */
    async removeAddress(id: number) {
      this.addresses = await apiDeleteAddress(id)
      const stillExists = this.addresses.some((a) => a.id === this.currentAddressId)
      if (!stillExists) {
        const def = this.addresses.find((a) => a.is_default)
        this.currentAddressId = def ? def.id : this.addresses[0]?.id || 0
      }
      return this.addresses
    },

    /** 从后端拉取最新个人资料（设置页进入时刷新昵称与手机号） */
    async loadProfile() {
      const me = await getMe()
      this.nickname = me.nickname || ''
      this.phone = me.phone || ''
      // role 以后端为准：审核通过后 token 里的角色不会变（签发时就固定了），
      // 但权限判定用的 users.role 已经更新，这里同步过来，
      // 用户刷新「我的」页就能看到服务方入口，不必重新登录。
      if (me.role && me.role !== this.role) {
        this.role = me.role
        uni.setStorageSync(KEY_ROLE, me.role)
      }
      return me
    },

    /** 保存个人资料并同步本地状态 */
    async saveProfile(nickname: string, phone: string) {
      const me = await updateMe({ nickname, phone })
      this.nickname = me.nickname
      this.phone = me.phone
      return me
    },

    /** 退出登录：清空本地会话与缓存身份，由调用方跳登录页 */
    logout() {
      this.userId = ''
      this.token = ''
      this.nickname = ''
      this.phone = ''
      this.addresses = []
      this.currentAddressId = 0
      this.ready = false
      this.role = 'user'
      clearToken()
      clearUserId()
      uni.removeStorageSync(KEY_ROLE)
    },
  },
})
