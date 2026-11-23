<script setup lang="ts">
import { computed, onUnmounted, reactive, ref } from 'vue'
import { onLoad } from '@dcloudio/uni-app'
import { useUserStore } from '@/store/user'
import { sendSmsCode } from '@/api'
import { ApiError } from '@/utils/request'

const user = useUserStore()

/** 登录方式：密码登录 / 短信验证码登录 */
const mode = ref<'password' | 'sms'>('password')
const form = reactive({ phone: '', password: '', code: '' })
const submitting = ref(false)
const showPwd = ref(false)
const sending = ref(false)
/** 发码后自动聚焦验证码输入框；blur 后复位，否则再次点击不会重新聚焦 */
const codeFocused = ref(false)
const countdown = ref(0)
let timer: ReturnType<typeof setInterval> | null = null
/** 登录成功后要去哪：从业务页被拦截过来时带 redirect */
const redirect = ref('')
/** 被拦截过来的原因：auth 表示是「操作需要登录」，用于顶部提示 */
const reason = ref('')
const wxLoading = ref(false)

const canSubmit = computed(() => {
  if (form.phone.length !== 11) return false
  return mode.value === 'password' ? form.password.length >= 6 : form.code.length === 6
})

onLoad((query) => {
  // uni-app 会把 query 参数解一次码；这里再兜底解一次，
  // 避免 redirect 里含 %2F 这类嵌套编码时二次跳转路径变成乱码
  let r = (query?.redirect as string) || ''
  if (r) {
    try {
      r = decodeURIComponent(r)
    } catch {
      // 解码失败说明本身没有编码，直接用原值
    }
  }
  redirect.value = r
  reason.value = (query?.reason as string) || ''
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
})

function togglePwd() {
  showPwd.value = !showPwd.value
}

function switchMode(next: 'password' | 'sms') {
  if (mode.value === next) return
  mode.value = next
}

function validPhone() {
  if (!/^1[3-9]\d{9}$/.test(form.phone)) {
    uni.showToast({ title: '请输入正确的手机号', icon: 'none' })
    return false
  }
  return true
}

function startCountdown(sec: number) {
  countdown.value = sec
  if (timer) clearInterval(timer)
  timer = setInterval(() => {
    countdown.value -= 1
    if (countdown.value <= 0 && timer) {
      clearInterval(timer)
      timer = null
    }
  }, 1000)
}

/** 获取验证码。验证码只会发到手机上，接口不返回，需用户自行填写 */
async function onSendCode() {
  if (sending.value || countdown.value > 0) return
  if (!validPhone()) return
  sending.value = true
  try {
    const res = await sendSmsCode({ phone: form.phone, scene: 'login' })
    // 清空上一次的输入，避免旧码残留导致用户以为没收到新码
    form.code = ''
    uni.showToast({ title: '验证码已发送，请查收短信', icon: 'none' })
    // 让用户直接接着输验证码，少点一次
    codeFocused.value = true
    startCountdown(res.interval || 60)
  } catch (e) {
    uni.showModal({
      title: '发送失败',
      content: e instanceof ApiError ? e.message : '请稍后重试',
      showCancel: false,
    })
  } finally {
    sending.value = false
  }
}

async function onSubmit() {
  if (submitting.value) return
  if (!validPhone()) return
  if (mode.value === 'password' && form.password.length < 6) {
    uni.showToast({ title: '密码至少 6 位', icon: 'none' })
    return
  }
  if (mode.value === 'sms' && form.code.length !== 6) {
    uni.showToast({ title: '请输入 6 位验证码', icon: 'none' })
    return
  }

  submitting.value = true
  try {
    if (mode.value === 'password') {
      await user.login(form.phone, form.password)
    } else {
      // 未注册手机号会在后端自动建号，等价于注册并登录
      await user.loginBySms(form.phone, form.code)
    }
    uni.showToast({ title: '登录成功', icon: 'success' })
    goAfterAuth()
  } catch (e) {
    uni.showModal({
      title: '登录失败',
      content: e instanceof ApiError ? e.message : '请稍后重试',
      showCancel: false,
    })
  } finally {
    submitting.value = false
  }
}

function goAfterAuth() {
  // onLoad 已解码过一次，这里不再 decode，否则会被解两遍
  const target = redirect.value || '/pages/index/index'
  setTimeout(() => {
    // 登录页在栈底，用 reLaunch 保证返回键不会退回登录页
    uni.reLaunch({ url: target })
  }, 500)
}

/** 微信一键授权登录：小程序内直接拉授权，H5 会提示不支持 */
async function onWechatLogin() {
  if (wxLoading.value) return
  wxLoading.value = true
  try {
    // 昵称头像不强制索取：拿不到也能建号，用户可在设置页补
    const s = await user.loginByWechat()
    // 还没手机号：先去补资料（头像/昵称/手机号都能从微信授权拿到），
    // 不阻断浏览，但下单前必须有手机号，所以趁登录这一次一次性补齐最省事
    if (s.need_bind_phone) {
      uni.reLaunch({ url: '/pages/auth/profile' })
      return
    }
    uni.showToast({ title: '登录成功', icon: 'success' })
    goAfterAuth()
  } catch (e) {
    const msg = e instanceof ApiError ? e.message : (e as Error).message || '请稍后重试'
    uni.showModal({ title: '微信登录失败', content: msg, showCancel: false })
  } finally {
    wxLoading.value = false
  }
}

/** 先逛逛：不建账号，仅放行浏览 */
function onGuest() {
  user.enterGuest()
  uni.reLaunch({ url: '/pages/index/index' })
}

function goForgot() {
  uni.navigateTo({ url: '/pages/auth/forgot' })
}

function goRegister() {
  uni.navigateTo({ url: '/pages/auth/register' })
}

function goDoc(type: 'terms' | 'privacy') {
  uni.navigateTo({ url: `/pages/doc/index?type=${type}` })
}

</script>

<template>
  <view class="page">
    <view class="hero">
      <image class="logo" src="/static/logo.png" mode="aspectFit" />
      <view class="title">邻有回应</view>
      <view class="sub">把琐事交给我们 · 家门口的安心服务</view>
    </view>

    <view class="form-card">
      <view v-if="reason === 'auth'" class="hint-bar">
        该操作需要登录后才能继续
      </view>

      <!-- 微信一键授权：主推路径，放在最前 -->
      <view class="btn-wx" :class="{ 'is-disabled': wxLoading }" @tap="onWechatLogin">
        <text class="wx-icon">微</text>
        <text>{{ wxLoading ? '授权中…' : '微信一键登录' }}</text>
      </view>

      <view class="or-row">
        <view class="or-line" />
        <text class="or-text">或使用手机号</text>
        <view class="or-line" />
      </view>

      <view class="tabs">
        <view class="tab" :class="{ active: mode === 'password' }" @tap="switchMode('password')">
          密码登录
        </view>
        <view class="tab" :class="{ active: mode === 'sms' }" @tap="switchMode('sms')">
          验证码登录
        </view>
      </view>

      <view class="field">
        <text class="label">手机号</text>
        <input
          v-model="form.phone"
          class="input"
          type="number"
          maxlength="11"
          placeholder="请输入手机号"
          placeholder-class="ph"
        />
      </view>

      <view v-if="mode === 'password'" class="field">
        <text class="label">密码</text>
        <input
          v-model="form.password"
          class="input"
          :password="!showPwd"
          maxlength="32"
          placeholder="请输入密码"
          placeholder-class="ph"
        />
        <text class="eye" @tap="togglePwd">{{ showPwd ? '隐藏' : '显示' }}</text>
      </view>

      <view v-else class="field">
        <text class="label">验证码</text>
        <input
          v-model="form.code"
          class="input"
          type="number"
          maxlength="6"
          :focus="codeFocused"
          placeholder="请输入短信验证码"
          placeholder-class="ph"
          @blur="codeFocused = false"
        />
        <text
          class="code-btn"
          :class="{ 'is-disabled': sending || countdown > 0 }"
          @tap="onSendCode"
        >
          {{ countdown > 0 ? `${countdown}s 后重发` : sending ? '发送中…' : '获取验证码' }}
        </text>
      </view>

      <view
        class="btn-primary submit"
        :class="{ 'is-disabled': !canSubmit || submitting }"
        @tap="onSubmit"
      >
        {{ submitting ? '登录中…' : '登 录' }}
      </view>

      <view class="switch-row">
        <text class="switch-text">还没有账号？</text>
        <text class="switch-link" @tap="goRegister">立即注册</text>
        <text class="divider">·</text>
        <text class="switch-link" @tap="goForgot">忘记密码</text>
      </view>

      <view class="guest-row">
        <text class="guest-link" @tap="onGuest">先随便逛逛 ›</text>
      </view>
      <view v-if="mode === 'sms'" class="tip-row">未注册的手机号验证后将自动创建账号</view>
    </view>

    <view class="agree">
      登录即表示同意
      <text class="link" @tap="goDoc('terms')">《服务条款》</text>
      与
      <text class="link" @tap="goDoc('privacy')">《隐私政策》</text>
    </view>

  </view>
</template>

<style scoped>
.page {
  min-height: 100vh;
  padding: 0 40rpx;
  background: linear-gradient(180deg, #e9f5f3 0%, #f8f8f5 340rpx, #f8f8f5 100%);
  box-sizing: border-box;
}

.hero {
  padding: 120rpx 0 60rpx;
  text-align: center;
}

.logo {
  display: block;
  width: 140rpx;
  height: 140rpx;
  margin: 0 auto 28rpx;
}

.title {
  font-size: 44rpx;
  font-weight: 800;
  color: var(--brand-dark);
  letter-spacing: 2rpx;
}

.sub {
  margin-top: 12rpx;
  font-size: 24rpx;
  color: var(--text-sub);
}

.form-card {
  background: #fff;
  border-radius: 24rpx;
  padding: 40rpx 32rpx;
  box-shadow: 0 6rpx 24rpx rgba(15, 40, 38, 0.05);
}

.hint-bar {
  margin-bottom: 20rpx;
  padding: 16rpx 20rpx;
  border-radius: 12rpx;
  background: #fef5e6;
  color: #a9762a;
  font-size: 23rpx;
}

.btn-wx {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12rpx;
  height: 92rpx;
  border-radius: 16rpx;
  background: #07c160;
  color: #fff;
  font-size: 30rpx;
  font-weight: 600;
  box-shadow: 0 6rpx 18rpx rgba(7, 193, 96, 0.24);
}

.btn-wx.is-disabled {
  opacity: 0.6;
}

.wx-icon {
  width: 40rpx;
  height: 40rpx;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.24);
  font-size: 22rpx;
  line-height: 40rpx;
  text-align: center;
}

.or-row {
  display: flex;
  align-items: center;
  gap: 16rpx;
  padding: 28rpx 0 8rpx;
}

.or-line {
  flex: 1;
  height: 1rpx;
  background: #eef0f2;
}

.or-text {
  font-size: 21rpx;
  color: var(--text-light);
}

.guest-row {
  margin-top: 22rpx;
  text-align: center;
}

.guest-link {
  font-size: 24rpx;
  color: var(--text-sub);
}

.tabs {
  display: flex;
  gap: 40rpx;
  padding-bottom: 12rpx;
  border-bottom: 1rpx solid #f3f4f6;
}

.tab {
  position: relative;
  padding: 8rpx 0 16rpx;
  font-size: 29rpx;
  color: var(--text-light);
}

.tab.active {
  color: var(--brand-dark);
  font-weight: 700;
}

.tab.active::after {
  content: '';
  position: absolute;
  left: 50%;
  bottom: 0;
  width: 48rpx;
  height: 6rpx;
  margin-left: -24rpx;
  border-radius: 3rpx;
  background: var(--brand);
}

.field {
  display: flex;
  align-items: center;
  gap: 20rpx;
  padding: 24rpx 0;
  border-bottom: 1rpx solid #f3f4f6;
}

.label {
  width: 110rpx;
  font-size: 27rpx;
  color: var(--text-sub);
  flex-shrink: 0;
}

.input {
  flex: 1;
  font-size: 29rpx;
  color: var(--text);
}

.ph {
  color: #b9c0c8;
  font-size: 27rpx;
}

.eye {
  font-size: 23rpx;
  color: var(--brand-dark);
  padding-left: 12rpx;
  flex-shrink: 0;
}

.code-btn {
  flex-shrink: 0;
  padding-left: 20rpx;
  font-size: 24rpx;
  color: var(--brand-dark);
  font-weight: 600;
}

.code-btn.is-disabled {
  color: #b9c0c8;
  font-weight: 400;
}

.submit {
  margin-top: 44rpx;
}

.switch-row {
  margin-top: 28rpx;
  text-align: center;
  font-size: 25rpx;
}

.switch-text {
  color: var(--text-light);
}

.switch-link {
  color: var(--brand-dark);
  font-weight: 600;
}

.divider {
  padding: 0 12rpx;
  color: #cfd8d6;
}

.tip-row {
  margin-top: 12rpx;
  text-align: center;
  font-size: 22rpx;
  color: var(--text-light);
}

.agree {
  margin-top: 32rpx;
  text-align: center;
  font-size: 22rpx;
  color: var(--text-light);
  line-height: 1.7;
}

.agree .link {
  color: var(--brand-dark);
}
</style>
