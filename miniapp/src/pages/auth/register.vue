<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useUserStore } from '@/store/user'
import { ApiError } from '@/utils/request'

const user = useUserStore()

const form = reactive({
  phone: '',
  nickname: '',
  password: '',
  confirm: '',
  agreed: false,
})
const submitting = ref(false)
const showPwd = ref(false)

const phoneOk = computed(() => /^1[3-9]\d{9}$/.test(form.phone))
const pwdOk = computed(() => form.password.length >= 6)
const matchOk = computed(() => form.password !== '' && form.password === form.confirm)
const canSubmit = computed(() => phoneOk.value && pwdOk.value && matchOk.value && form.agreed)

function togglePwd() {
  showPwd.value = !showPwd.value
}

function toggleAgree() {
  form.agreed = !form.agreed
}

async function onSubmit() {
  if (submitting.value) return
  if (!phoneOk.value) {
    uni.showToast({ title: '请输入正确的手机号', icon: 'none' })
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
  if (!form.agreed) {
    uni.showToast({ title: '请先同意服务条款与隐私政策', icon: 'none' })
    return
  }

  submitting.value = true
  try {
    await user.register(form.phone, form.password, form.nickname.trim())
    uni.showToast({ title: '注册成功', icon: 'success' })
    setTimeout(() => uni.reLaunch({ url: '/pages/index/index' }), 600)
  } catch (e) {
    uni.showModal({
      title: '注册失败',
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

function goDoc(type: 'terms' | 'privacy') {
  uni.navigateTo({ url: `/pages/doc/index?type=${type}` })
}
</script>

<template>
  <view class="page">
    <view class="head">
      <view class="title">创建账号</view>
      <view class="sub">注册后即可预约代倒垃圾、遛宠与陪诊服务</view>
    </view>

    <view class="form-card">
      <view class="field">
        <text class="label">手机号</text>
        <input
          v-model="form.phone"
          class="input"
          type="number"
          maxlength="11"
          placeholder="用于登录与服务方联系"
          placeholder-class="ph"
        />
        <text v-if="form.phone" class="check" :class="{ ok: phoneOk }">{{ phoneOk ? '✓' : '✕' }}</text>
      </view>

      <view class="field">
        <text class="label">昵称</text>
        <input
          v-model="form.nickname"
          class="input"
          maxlength="32"
          placeholder="怎么称呼您（选填）"
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
          placeholder="再输入一次密码"
          placeholder-class="ph"
        />
        <text v-if="form.confirm" class="check" :class="{ ok: matchOk }">{{ matchOk ? '✓' : '✕' }}</text>
      </view>

      <view class="agree-row" @tap="toggleAgree">
        <view class="checkbox" :class="{ checked: form.agreed }">
          <text v-if="form.agreed" class="tick">✓</text>
        </view>
        <view class="agree-text">
          我已阅读并同意
          <text class="link" @tap.stop="goDoc('terms')">《服务条款》</text>
          与
          <text class="link" @tap.stop="goDoc('privacy')">《隐私政策》</text>
        </view>
      </view>

      <view
        class="btn-primary submit"
        :class="{ 'is-disabled': !canSubmit || submitting }"
        @tap="onSubmit"
      >
        {{ submitting ? '注册中…' : '注 册' }}
      </view>

      <view class="switch-row">
        <text class="switch-text">已有账号？</text>
        <text class="switch-link" @tap="goLogin">去登录</text>
      </view>
    </view>

    <view class="foot-note">
      注册手机号将用于服务方上门前的联系确认，我们不会用于其它用途。
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

.check {
  font-size: 26rpx;
  color: var(--danger);
  flex-shrink: 0;
}

.check.ok {
  color: var(--brand);
}

.agree-row {
  display: flex;
  align-items: flex-start;
  gap: 14rpx;
  padding: 30rpx 0 8rpx;
}

.checkbox {
  width: 34rpx;
  height: 34rpx;
  border: 2rpx solid #cfd8d6;
  border-radius: 8rpx;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  margin-top: 2rpx;
  background: #fff;
}

.checkbox.checked {
  background: var(--brand);
  border-color: var(--brand);
}

.tick {
  color: #fff;
  font-size: 22rpx;
  line-height: 1;
}

.agree-text {
  flex: 1;
  font-size: 23rpx;
  color: var(--text-sub);
  line-height: 1.7;
}

.link {
  color: var(--brand-dark);
}

.submit {
  margin-top: 36rpx;
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
