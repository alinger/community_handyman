import { defineStore } from 'pinia'
import {
  deleteAddress as apiDeleteAddress,
  getMe,
  listAddresses,
  login as apiLogin,
  loginBySms as apiLoginBySms,
  wechatLogin as apiWechatLogin,
  bindPhone as apiBindPhone,
  bindPhoneByWechat as apiBindPhoneByWechat,
  register as apiRegister,
  resetPasswordBySms as apiResetPassword,
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
const KEY_VIEW = 'ch_view_mode'
/** 访客模式标记：只放行浏览，不代表登录态 */
const KEY_GUEST = 'ch_guest_mode'

/**
 * 用户与地址状态。
 *
 * 登录态：正式流程走手机号 + 密码（/auth/login、/auth/register）拿 token，
 * 请求层用 Authorization: Bearer 带上。token 与 userId 都落在本地缓存，
 * 冷启动时 isLoggedIn 直接读缓存判断，不需要先请求一次后端。
 *
 * 身份与视角是两件事，不能混：
 *   role     权限，来自后端 users.role，只有审核通过才变 provider，决定「能不能」进服务方端；
 *   viewMode 视角，纯前端偏好，决定「现在在看」住户侧还是服务方侧。
 * 服务方本身也是住户（也要给自己下单），所以已认证用户可以自由切视角，
 * 切换只改 viewMode 不动 role —— 既不会提权也不会丢权限。
 */
export const useUserStore = defineStore('user', {
  state: () => ({
    userId: getUserId(),
    token: getToken(),
    nickname: '',
    phone: '',
    /** 头像相对地址（服务端返回），展示时拼 BASE_URL */
    avatar: '',
    role: (uni.getStorageSync(KEY_ROLE) as string) || 'user',
    /** 当前视角：user=住户侧，provider=服务方侧。未认证时恒为 user */
    viewMode: (uni.getStorageSync(KEY_VIEW) as string) || 'user',
    addresses: [] as Address[],
    currentAddressId: 0,
    ready: false,
    loginPromise: null as Promise<void> | null,
    /**
     * 访客模式：用户选择「先逛逛」时置真。
     * 它只表示「允许停留」，不代表有身份 —— isLoggedIn 仍为 false，
     * 所有需要 user_id 的接口都不会被调用。刷新后保留，避免每次都被弹回登录页。
     */
    guest: (uni.getStorageSync(KEY_GUEST) as string) === '1',
    /** 微信登录后仍未绑定手机号，用于提示补资料 */
    needBindPhone: false,
  }),

  getters: {
    /** 是否已登录：有 token 才算，光有 userId 是历史联调残留 */
    isLoggedIn(state): boolean {
      return !!state.token && !!state.userId
    },
    /** 是否处于访客态：未登录但被允许浏览 */
    isGuest(state): boolean {
      return !state.token && state.guest
    },
    /** 是否具备服务方权限（能否进服务方端） */
    isProvider(state): boolean {
      return state.role === 'provider'
    },
    /** 当前是否处于服务方视角：权限与视角同时满足才算 */
    isProviderView(state): boolean {
      return state.role === 'provider' && state.viewMode === 'provider'
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

    /** 进入访客模式：不建账号、不发 token，只放行浏览 */
    enterGuest() {
      this.guest = true
      uni.setStorageSync(KEY_GUEST, '1')
    },

    /** 退出访客模式（去登录时调用），避免登录页返回又落回访客态 */
    exitGuest() {
      this.guest = false
      uni.removeStorageSync(KEY_GUEST)
    },

    /**
     * 微信一键授权登录。
     * 小程序端 uni.login 返回的 code 交给后端换 openid；
     * H5 没有微信环境，uni.login 会失败，由调用方提示改用其它方式。
     */
    async loginByWechat(profile?: { nickname?: string; avatar?: string }) {
      const code = await new Promise<string>((resolve, reject) => {
        uni.login({
          provider: 'weixin',
          success: (res) => (res.code ? resolve(res.code) : reject(new Error('未获取到微信授权码'))),
          fail: () => reject(new Error('当前环境不支持微信授权登录')),
        })
      })
      const s = await apiWechatLogin({ code, ...profile })
      this.applySession(s)
      this.needBindPhone = s.need_bind_phone
      this.exitGuest()
      return s
    },

    /** 绑定手机号，成功后刷新会话 */
    async bindPhone(phone: string, code = '') {
      const s = await apiBindPhone({ phone, code })
      this.applySession(s)
      this.needBindPhone = false
      return s
    },

    /** 应用正式登录/注册返回的会话 */
    applySession(s: Session) {
      this.token = s.token
      this.userId = String(s.user.user_id)
      this.nickname = s.user.nickname || ''
      this.phone = s.user.phone || ''
      this.avatar = s.user.avatar || ''
      this.role = s.user.role || 'user'
      setToken(s.token)
      setUserId(s.user.user_id)
      uni.setStorageSync(KEY_ROLE, this.role)
      this.guest = false
      uni.removeStorageSync(KEY_GUEST)
      this.syncViewMode()
      this.ready = true
    },

    /**
     * 校正视角与权限的一致性，并把结果写回本地。
     * 没有服务方权限时视角必须是住户侧，否则会出现「进入服务方界面但接口全 403」；
     * 有权限时保留上次选择（下次打开直接回到上次看的那一侧）。
     * 无论是否发生校正都写一次，保证 storage 里始终有明确值，
     * 冷启动读到的就不会是空字符串。
     */
    syncViewMode() {
      if (this.role !== 'provider' && this.viewMode !== 'user') {
        this.viewMode = 'user'
      }
      uni.setStorageSync(KEY_VIEW, this.viewMode)
    },

    async login(phone: string, password: string) {
      const s = await apiLogin({ phone, password })
      this.applySession(s)
      return s
    },

    /** 验证码登录；首次登录手机号时后端自动建号，与注册等价 */
    async loginBySms(phone: string, code: string, nickname = '') {
      const s = await apiLoginBySms({ phone, code, nickname })
      this.applySession(s)
      return s
    },

    /** 凭短信验证码重置密码，成功后直接落地登录态 */
    async resetPassword(phone: string, code: string, newPassword: string) {
      const s = await apiResetPassword({ phone, code, new_password: newPassword })
      this.applySession(s)
      return s
    },

    async register(phone: string, password: string, nickname: string) {
      const s = await apiRegister({ phone, password, nickname })
      this.applySession(s)
      return s
    },

    /**
     * 切换视角：住户侧 ↔ 服务方侧。
     * 只改本地偏好，不改 role、不请求后端 —— 权限始终由 users.role 决定。
     * 没有服务方权限时拒绝切换，返回 false 由调用方提示。
     */
    switchView(mode: 'user' | 'provider'): boolean {
      if (mode === 'provider' && !this.isProvider) return false
      this.viewMode = mode
      uni.setStorageSync(KEY_VIEW, mode)
      return true
    },

    /** 视角在家页与工作台之间跳一次，配合 TabBar 的中间按钮 */
    toggleView(): 'user' | 'provider' {
      const next = this.isProviderView ? 'user' : 'provider'
      this.switchView(next)
      uni.reLaunch({
        url: next === 'provider' ? '/pages/provider/tasks' : '/pages/index/index',
      })
      return next
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
      this.avatar = me.avatar || ''
      // role 以后端为准：审核通过后 token 里的角色不会变（签发时就固定了），
      // 但权限判定用的 users.role 已经更新，这里同步过来，
      // 用户刷新「我的」页就能看到服务方入口，不必重新登录。
      if (me.role && me.role !== this.role) {
        this.role = me.role
        uni.setStorageSync(KEY_ROLE, me.role)
        this.syncViewMode()
      }
      return me
    },

    /** 保存个人资料并同步本地状态（avatar 传已上传后的地址，留空表示不改头像） */
    async saveProfile(nickname: string, phone: string, avatar = '') {
      const me = await updateMe({ nickname, phone, avatar })
      this.nickname = me.nickname
      this.phone = me.phone
      if (me.avatar !== undefined) this.avatar = me.avatar || ''
      return me
    },

    /** 微信授权手机号一键绑定：成功后会话里已带手机号 */
    async bindPhoneByWechat(code: string) {
      const s = await apiBindPhoneByWechat({ code })
      this.applySession(s)
      this.needBindPhone = false
      return s
    },

    /** 退出登录：清空本地会话与缓存身份，由调用方跳登录页 */
    logout() {
      this.userId = ''
      this.token = ''
      this.nickname = ''
      this.phone = ''
      this.avatar = ''
      this.addresses = []
      this.currentAddressId = 0
      this.ready = false
      this.role = 'user'
      this.viewMode = 'user'
      clearToken()
      clearUserId()
      uni.removeStorageSync(KEY_ROLE)
      uni.removeStorageSync(KEY_VIEW)
    },
  },
})
