<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { onLoad } from '@dcloudio/uni-app'
import { useUserStore } from '@/store/user'
import { ApiError } from '@/utils/request'

const user = useUserStore()

const form = reactive({ phone: '', password: '' })
const submitting = ref(false)
const showPwd = ref(false)
/** 登录成功后要去哪：从业务页被拦截过来时带 redirect */
const redirect = ref('')

const canSubmit = computed(() => form.phone.length === 11 && form.password.length >= 6)

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
})

function togglePwd() {
  showPwd.value = !showPwd.value
}

async function onSubmit() {
  if (submitting.value) return
  if (!/^1[3-9]\d{9}$/.test(form.phone)) {
    uni.showToast({ title: '请输入正确的手机号', icon: 'none' })
    return
  }
  if (form.password.length < 6) {
    uni.showToast({ title: '密码至少 6 位', icon: 'none' })
    return
  }
  submitting.value = true
  try {
    await user.login(form.phone, form.password)
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
      <view class="logo">助</view>
      <view class="title">小区好帮手</view>
      <view class="sub">把琐事交给我们 · 家门口的安心服务</view>
    </view>

    <view class="form-card">
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

      <view class="field">
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
      </view>
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
  width: 120rpx;
  height: 120rpx;
  margin: 0 auto 28rpx;
  border-radius: 34rpx;
  background: var(--brand);
  color: #fff;
  font-size: 56rpx;
  font-weight: 700;
  line-height: 120rpx;
  text-align: center;
  box-shadow: 0 10rpx 30rpx rgba(15, 127, 120, 0.22);
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
