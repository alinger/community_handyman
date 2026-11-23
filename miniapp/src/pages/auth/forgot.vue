<script setup lang="ts">
import { computed, onUnmounted, reactive, ref } from 'vue'
import { useUserStore } from '@/store/user'
import { sendSmsCode } from '@/api'
import { ApiError } from '@/utils/request'

const user = useUserStore()

const form = reactive({
  phone: '',
  code: '',
  password: '',
  confirm: '',
})
const submitting = ref(false)
const sending = ref(false)
/** 发码后自动聚焦验证码输入框；blur 后复位，否则再次点击不会重新聚焦 */
const codeFocused = ref(false)
const showPwd = ref(false)
const countdown = ref(0)
let timer: ReturnType<typeof setInterval> | null = null

const phoneOk = computed(() => /^1[3-9]\d{9}$/.test(form.phone))
const pwdOk = computed(() => form.password.length >= 6)
const matchOk = computed(() => form.password !== '' && form.password === form.confirm)
const canSubmit = computed(() => phoneOk.value && form.code.length === 6 && pwdOk.value && matchOk.value)

onUnmounted(() => {
  if (timer) clearInterval(timer)
})

function togglePwd() {
  showPwd.value = !showPwd.value
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

/** 只有已注册手机号才发得出重置验证码，后端会拦未注册号码 */
async function onSendCode() {
  if (sending.value || countdown.value > 0) return
  if (!phoneOk.value) {
    uni.showToast({ title: '请输入正确的手机号', icon: 'none' })
    return
  }
  sending.value = true
  try {
    const res = await sendSmsCode({ phone: form.phone, scene: 'reset' })
    form.code = ''
    uni.showToast({ title: '验证码已发送，请查收短信', icon: 'none' })
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
  if (!phoneOk.value) {
    uni.showToast({ title: '请输入正确的手机号', icon: 'none' })
    return
  }
  if (form.code.length !== 6) {
    uni.showToast({ title: '请输入 6 位验证码', icon: 'none' })
    return
  }
  if (!pwdOk.value) {
    uni.showToast({ title: '密码至少 6 位', icon: 'none' })
    return
  }
  if (!matchOk.value) {
    uni.showToast({ title: '两次输入的密码不一致', icon: 'none' })
    return
  }

  submitting.value = true
  try {
    // store 内部会 applySession：重置成功即登录，用户不必再用新密码登一次
    await user.resetPassword(form.phone, form.code, form.password)
    uni.showToast({ title: '密码已重置', icon: 'success' })
    setTimeout(() => uni.reLaunch({ url: '/pages/index/index' }), 600)
  } catch (e) {
    uni.showModal({
      title: '重置失败',
      content: e instanceof ApiError ? e.message : '请稍后重试',
      showCancel: false,
    })
  } finally {
    submitting.value = false
  }
}

function goLogin() {
  const pages = getCurrentPages()
  if (pages.length > 1) uni.navigateBack()
  else uni.reLaunch({ url: '/pages/auth/login' })
}
</script>

<template>
  <view class="page">
    <view class="head">
      <view class="title">找回密码</view>
      <view class="sub">验证手机号后设置新密码，忘记原密码也能重新登录</view>
    </view>

    <view class="form-card">
      <view class="field">
        <text class="label">手机号</text>
        <input
          v-model="form.phone"
          class="input"
          type="number"
          maxlength="11"
          placeholder="请输入注册时的手机号"
          placeholder-class="ph"
        />
        <text v-if="form.phone" class="check" :class="{ ok: phoneOk }">{{ phoneOk ? '✓' : '✕' }}</text>
      </view>

      <view class="field">
        <text class="label">验证码</text>
        <input
          v-model="form.code"
          class="input"
          type="number"
          maxlength="6"
          :focus="codeFocused"
          placeholder="6 位短信验证码"
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

      <view class="field">
        <text class="label">新密码</text>
        <input
          v-model="form.password"
          class="input"
          :password="!showPwd"
          maxlength="32"
          placeholder="至少 6 位"
          placeholder-class="ph"
        />
        <text class="eye" @tap="togglePwd">{{ showPwd ? '隐藏' : '显示' }}</text>
      </view>

      <view class="field">
        <text class="label">确认密码</text>
        <input
          v-model="form.confirm"
          class="input"
          :password="!showPwd"
          maxlength="32"
          placeholder="再输入一次新密码"
          placeholder-class="ph"
        />
        <text v-if="form.confirm" class="check" :class="{ ok: matchOk }">{{ matchOk ? '✓' : '✕' }}</text>
      </view>

      <view
        class="btn-primary submit"
        :class="{ 'is-disabled': !canSubmit || submitting }"
        @tap="onSubmit"
      >
        {{ submitting ? '提交中…' : '重置密码' }}
      </view>

      <view class="switch-row">
        <text class="switch-text">想起来了？</text>
        <text class="switch-link" @tap="goLogin">返回登录</text>
      </view>
    </view>

    <view class="foot-note">
      验证码 5 分钟内有效，同一手机号 60 秒可重发一次、每日最多 10 条。
      为保障账号安全，重置成功后原密码立即失效。
    </view>
  </view>
</template>

<style scoped>
.page {
  min-height: 100vh;
  padding: 0 40rpx 60rpx;
  background: linear-gradient(180deg, #e9f5f3 0%, #f8f8f5 300rpx, #f8f8f5 100%);
  box-sizing: border-box;
}

.head {
  padding: 80rpx 8rpx 44rpx;
}

.title {
  font-size: 44rpx;
  font-weight: 800;
  color: var(--brand-dark);
}

.sub {
  margin-top: 14rpx;
  font-size: 24rpx;
  color: var(--text-sub);
  line-height: 1.6;
}

.form-card {
  background: #fff;
  border-radius: 24rpx;
  padding: 24rpx 32rpx 36rpx;
  box-shadow: 0 6rpx 24rpx rgba(15, 40, 38, 0.05);
}

.field {
  display: flex;
  align-items: center;
  gap: 20rpx;
  padding: 26rpx 0;
  border-bottom: 1rpx solid #f3f4f6;
}

.label {
  width: 140rpx;
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
  flex-shrink: 0;
  padding-left: 10rpx;
}

.code-btn {
  flex-shrink: 0;
  padding-left: 10rpx;
  font-size: 24rpx;
  color: var(--brand-dark);
  font-weight: 600;
}

.code-btn.is-disabled {
  color: #b9c0c8;
  font-weight: 400;
}

.check {
  font-size: 26rpx;
  color: var(--danger);
  flex-shrink: 0;
}

.check.ok {
  color: var(--brand);
}

.submit {
  margin-top: 40rpx;
}

.switch-row {
  margin-top: 26rpx;
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

.foot-note {
  margin-top: 36rpx;
  padding: 0 8rpx;
  font-size: 21rpx;
  color: var(--text-light);
  line-height: 1.7;
  text-align: center;
}
</style>
