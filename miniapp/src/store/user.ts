import { defineStore } from 'pinia'
import { devLogin, getMe, listAddresses, setDefaultAddress as apiSetDefault, updateMe, type Address } from '@/api'
import { getUserId, setUserId, clearUserId } from '@/utils/request'

const KEY_OPENID = 'ch_openid'
const KEY_ROLE = 'ch_role'

/**
 * 用户与地址状态。
 *
 * 当前用 /api/v1/dev/login 按 openid 取用户，仅开发环境可用。
 * 接入微信登录后改成：uni.login 拿 code → 后端 code2session 换 openid 并签发 JWT，
 * 这里只需替换 ensureLogin 的实现，页面不用动。
 */
export const useUserStore = defineStore('user', {
  state: () => ({
    userId: getUserId(),
    nickname: '',
    phone: '',
    role: (uni.getStorageSync(KEY_ROLE) as string) || 'user',
    addresses: [] as Address[],
    currentAddressId: 0,
    ready: false,
    loginPromise: null as Promise<void> | null,
  }),

  getters: {
    currentAddress(state): Address | undefined {
      return state.addresses.find((a) => a.id === state.currentAddressId) || state.addresses[0]
    },
    hasAddress(state): boolean {
      return state.addresses.length > 0
    },
  },

  actions: {
    /**
     * 确保已登录，重复调用不会重复请求。
     * App.onLaunch 与页面 onShow 会并发调用，这里用同一个 Promise 去重，
     * 页面侧 await 它即可保证拿到登录结果再发业务请求。
     */
    async ensureLogin() {
      if (this.userId) return
      if (!this.loginPromise) {
        this.loginPromise = this.doLogin().finally(() => {
          this.loginPromise = null
        })
      }
      await this.loginPromise
    },

    async doLogin() {
      if (this.userId) return
      // 开发期用固定 openid：H5 与开发者工具共用同一账号，
      // 便于在后台造数据后回到页面直接看到。生产环境这段整体被微信登录替换。
      let openid = (uni.getStorageSync(KEY_OPENID) as string) || ''
      if (!openid) {
        openid = 'mp_dev_user'
        uni.setStorageSync(KEY_OPENID, openid)
      }
      // H5 支持在地址上加 ?as=provider|user 快速切换演示身份。
      // /dev/login 只在后端 dev 环境注册，生产后端直接 404，此入口天然失效。
      // #ifdef H5
      {
        const as = new URLSearchParams(window.location.search).get('as')
        if (as === 'provider' || as === 'user') {
          openid = `mp_dev_${as}`
          uni.setStorageSync(KEY_OPENID, openid)
        }
      }
      // #endif
      const isProviderAcct = openid === 'mp_dev_provider'
      const nickname = isProviderAcct ? '王师傅' : '住户'
      const res = await devLogin(openid, nickname, isProviderAcct ? 'provider' : 'user')
      this.userId = String(res.user_id)
      this.nickname = res.nickname || nickname
      this.role = res.role || 'user'
      uni.setStorageSync(KEY_ROLE, this.role)
      setUserId(res.user_id)
      this.ready = true
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

    /**
     * 切换演示身份（仅开发环境）。
     * 住户与服务方是两个独立账号，切换后整页刷新。
     * 生产环境由微信登录决定身份，此方法不会被调用。
     */
    async switchRole(role: 'user' | 'provider') {
      const openid = role === 'provider' ? 'mp_dev_provider' : 'mp_dev_user'
      uni.setStorageSync(KEY_OPENID, openid)
      uni.setStorageSync(KEY_ROLE, role)
      clearUserId()
      this.userId = ''
      this.addresses = []
      this.currentAddressId = 0
      await this.ensureLogin()
      uni.reLaunch({ url: role === 'provider' ? '/pages/provider/tasks' : '/pages/index/index' })
    },

    /** 从后端拉取最新个人资料（设置页进入时刷新昵称与手机号） */
    async loadProfile() {
      const me = await getMe()
      this.nickname = me.nickname || ''
      this.phone = me.phone || ''
      return me
    },

    /** 保存个人资料并同步本地状态 */
    async saveProfile(nickname: string, phone: string) {
      const me = await updateMe({ nickname, phone })
      this.nickname = me.nickname
      this.phone = me.phone
      return me
    },

    /** 退出登录：清空本地会话与缓存身份。正式接入微信登录后，下次进入重新走登录流程 */
    logout() {
      this.userId = ''
      this.nickname = ''
      this.phone = ''
      this.addresses = []
      this.currentAddressId = 0
      this.ready = false
      this.role = 'user'
      clearUserId()
      uni.removeStorageSync(KEY_OPENID)
      uni.removeStorageSync(KEY_ROLE)
    },
  },
})
