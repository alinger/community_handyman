import { defineStore } from 'pinia'
import { devLogin, listAddresses, type Address } from '@/api'
import { getUserId, setUserId, clearUserId } from '@/utils/request'

const KEY_OPENID = 'ch_openid'

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
    addresses: [] as Address[],
    currentAddressId: 0,
    ready: false,
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
    /** 确保已登录，重复调用不会重复请求 */
    async ensureLogin() {
      if (this.userId) return
      // 开发期用固定 openid：H5 与开发者工具共用同一账号，
      // 便于在后台造数据后回到页面直接看到。生产环境这段整体被微信登录替换。
      let openid = (uni.getStorageSync(KEY_OPENID) as string) || ''
      if (!openid) {
        openid = 'mp_dev_user'
        uni.setStorageSync(KEY_OPENID, openid)
      }
      const res = await devLogin(openid, '住户')
      this.userId = String(res.user_id)
      this.nickname = res.nickname || '住户'
      setUserId(res.user_id)
      this.ready = true
    },

    /** 加载地址列表，默认选中第一个 */
    async loadAddresses() {
      const list = await listAddresses()
      this.addresses = list || []
      if (!this.currentAddressId && this.addresses.length) {
        this.currentAddressId = this.addresses[0].id
      }
      return this.addresses
    },

    setCurrentAddress(id: number) {
      this.currentAddressId = id
    },

    logout() {
      this.userId = ''
      this.nickname = ''
      this.addresses = []
      this.currentAddressId = 0
      this.ready = false
      clearUserId()
    },
  },
})
