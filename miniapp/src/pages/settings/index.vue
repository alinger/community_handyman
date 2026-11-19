<script setup lang="ts">
import { reactive, ref } from 'vue'
import { onShow } from '@dcloudio/uni-app'
import { useUserStore } from '@/store/user'
import { ApiError } from '@/utils/request'

const user = useUserStore()

const saving = ref(false)
const loaded = ref(false)
const form = reactive({
  nickname: '',
  phone: '',
})

onShow(async () => {
  try {
    await user.ensureLogin()
    await user.loadProfile()
    form.nickname = user.nickname
    form.phone = user.phone
    loaded.value = true
  } catch (e) {
    console.warn('[settings] 加载个人资料失败', e)
    uni.showToast({ title: '加载失败，请返回重试', icon: 'none' })
  }
})

async function onSave() {
  if (saving.value) return
  const nickname = form.nickname.trim()
  if (!nickname) {
    uni.showToast({ title: '昵称不能为空', icon: 'none' })
    return
  }
  const phone = form.phone.trim()
  if (phone && !/^1[3-9]\d{9}$/.test(phone)) {
    uni.showToast({ title: '手机号格式不正确', icon: 'none' })
    return
  }
  saving.value = true
  try {
    await user.saveProfile(nickname, phone)
    uni.showToast({ title: '已保存', icon: 'success' })
    setTimeout(() => uni.navigateBack(), 600)
  } catch (e) {
    uni.showModal({
      title: '保存失败',
      content: e instanceof ApiError ? e.message : '请稍后重试',
      showCancel: false,
    })
  } finally {
    saving.value = false
  }
}

function onLogout() {
  uni.showModal({
    title: '退出登录',
    content: '退出后需要重新登录才能继续使用，确定退出吗？',
    confirmText: '退出',
    confirmColor: '#dc2626',
    success: (r) => {
      if (!r.confirm) return
      user.logout()
      uni.showToast({ title: '已退出', icon: 'none' })
      // 重新进入首页：dev 环境会自动重新联调登录，生产环境重新走微信登录
      setTimeout(() => uni.reLaunch({ url: '/pages/index/index' }), 500)
    },
  })
}
</script>

<template>
  <view class="page">
    <view class="card section">
      <view class="section-title">个人信息</view>

      <view v-if="loaded" class="profile-head">
        <view class="avatar">{{ (user.nickname || '用')[0] }}</view>
        <view class="uid">ID {{ user.userId || '-' }}</view>
      </view>

      <view class="form-item">
        <text class="form-label">昵称</text>
        <input
          v-model="form.nickname"
          class="form-input"
          maxlength="32"
          placeholder="怎么称呼您"
          :disabled="!loaded"
        />
      </view>
      <view class="form-item">
        <text class="form-label">手机号</text>
        <input
          v-model="form.phone"
          class="form-input"
          type="number"
          maxlength="11"
          placeholder="选填，方便服务方联系"
          :disabled="!loaded"
        />
      </view>
      <view class="form-tip">昵称会在预约与服务过程中展示给服务方</view>

      <view class="btn-primary save-btn" :class="{ 'is-disabled': saving }" @tap="onSave">
        {{ saving ? '保存中…' : '保存' }}
      </view>
    </view>

    <view class="card section">
      <view class="section-title">账号</view>
      <view class="logout-btn" @tap="onLogout">退出登录</view>
      <view class="form-tip center">退出后将清除本机登录状态</view>
    </view>
  </view>
</template>

<style scoped>
.page {
  min-height: 100vh;
  padding: 24rpx 24rpx 60rpx;
}

.section {
  padding: 32rpx;
}

.section-title {
  font-size: 30rpx;
  font-weight: 600;
  margin-bottom: 24rpx;
}

.profile-head {
  display: flex;
  align-items: center;
  gap: 20rpx;
  padding-bottom: 24rpx;
}

.avatar {
  width: 96rpx;
  height: 96rpx;
  border-radius: 50%;
  background: var(--brand-light);
  color: var(--brand-dark);
  font-size: 40rpx;
  font-weight: 600;
  display: flex;
  align-items: center;
  justify-content: center;
}

.uid {
  font-size: 24rpx;
  color: var(--text-light);
}

.form-item {
  display: flex;
  align-items: center;
  gap: 24rpx;
  padding: 22rpx 0;
  border-bottom: 1rpx solid #f3f4f6;
}

.form-label {
  width: 140rpx;
  font-size: 27rpx;
  color: var(--text-sub);
  flex-shrink: 0;
}

.form-input {
  flex: 1;
  font-size: 28rpx;
}

.form-tip {
  font-size: 23rpx;
  color: var(--text-light);
  margin-top: 16rpx;
  line-height: 1.6;
}

.form-tip.center {
  text-align: center;
  margin-top: 20rpx;
}

.save-btn {
  margin-top: 36rpx;
}

.logout-btn {
  border: 1rpx solid #fecaca;
  background: #fef2f2;
  color: var(--danger);
  border-radius: 16rpx;
  text-align: center;
  padding: 22rpx 0;
  font-size: 28rpx;
  font-weight: 500;
}

.logout-btn:active {
  opacity: 0.8;
}
</style>
