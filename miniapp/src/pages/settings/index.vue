<script setup lang="ts">
import { reactive, ref } from 'vue'
import { onShow } from '@dcloudio/uni-app'
import { changePassword } from '@/api'
import { useUserStore } from '@/store/user'
import { ensureAuth } from '@/utils/auth'
import { ApiError } from '@/utils/request'

const user = useUserStore()

const saving = ref(false)
const loaded = ref(false)
const form = reactive({
  nickname: '',
  phone: '',
})

// 修改密码弹层
const pwdVisible = ref(false)
const pwdSaving = ref(false)
const pwdForm = reactive({ oldPwd: '', newPwd: '', confirm: '' })
const pwdErr = ref('')

function openPwd() {
  pwdForm.oldPwd = ''
  pwdForm.newPwd = ''
  pwdForm.confirm = ''
  pwdErr.value = ''
  pwdVisible.value = true
}

async function onSavePwd() {
  if (pwdSaving.value) return
  pwdErr.value = ''
  if (!pwdForm.oldPwd) {
    pwdErr.value = '请输入原密码'
    return
  }
  if (pwdForm.newPwd.length < 6) {
    pwdErr.value = '新密码至少 6 位'
    return
  }
  if (pwdForm.newPwd !== pwdForm.confirm) {
    pwdErr.value = '两次输入的新密码不一致'
    return
  }
  pwdSaving.value = true
  try {
    await changePassword({ old_password: pwdForm.oldPwd, new_password: pwdForm.newPwd })
    pwdVisible.value = false
    uni.showToast({ title: '密码已修改', icon: 'success' })
  } catch (e) {
    pwdErr.value = e instanceof ApiError ? e.message : '请稍后重试'
  } finally {
    pwdSaving.value = false
  }
}

onShow(async () => {
  if (!ensureAuth()) return
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
      // 退出后回登录页，不能回首页（首页需要登录态）
      setTimeout(() => uni.reLaunch({ url: '/pages/auth/login' }), 500)
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
      <view class="section-title">账号安全</view>
      <view class="menu-item" @tap="openPwd">
        <text>修改密码</text>
        <text class="chev">›</text>
      </view>
      <view class="logout-btn" @tap="onLogout">退出登录</view>
      <view class="form-tip center">退出后将清除本机登录状态</view>
    </view>

    <!-- 修改密码弹层 -->
    <view v-if="pwdVisible" class="mask" @tap="pwdVisible = false">
      <view class="sheet" @tap.stop>
        <view class="sheet-title">修改密码</view>
        <view class="form-item">
          <text class="form-label">原密码</text>
          <input v-model="pwdForm.oldPwd" class="form-input" password maxlength="32" placeholder="请输入原密码" />
        </view>
        <view class="form-item">
          <text class="form-label">新密码</text>
          <input v-model="pwdForm.newPwd" class="form-input" password maxlength="32" placeholder="至少 6 位" />
        </view>
        <view class="form-item">
          <text class="form-label">确认新密码</text>
          <input v-model="pwdForm.confirm" class="form-input" password maxlength="32" placeholder="再输入一次" />
        </view>
        <view v-if="pwdErr" class="pwd-err">{{ pwdErr }}</view>
        <view class="sheet-actions">
          <view class="btn-ghost half" @tap="pwdVisible = false">取消</view>
          <view class="btn-primary half" :class="{ 'is-disabled': pwdSaving }" @tap="onSavePwd">
            {{ pwdSaving ? '提交中…' : '确认修改' }}
          </view>
        </view>
      </view>
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
  margin-top: 12rpx;
}

.logout-btn:active {
  opacity: 0.8;
}

.menu-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 26rpx 0;
  border-bottom: 1rpx solid #f3f4f6;
  font-size: 28rpx;
}

.chev {
  color: var(--text-light);
}

.pwd-err {
  color: var(--danger);
  font-size: 23rpx;
  margin-top: 14rpx;
}

.mask {
  position: fixed;
  inset: 0;
  background: rgba(15, 23, 42, 0.45);
  z-index: 300;
  display: flex;
  align-items: flex-end;
}

.sheet {
  width: 100%;
  background: #fff;
  border-radius: 32rpx 32rpx 0 0;
  padding: 40rpx 32rpx;
  padding-bottom: calc(40rpx + env(safe-area-inset-bottom));
}

.sheet-title {
  font-size: 32rpx;
  font-weight: 600;
  text-align: center;
  margin-bottom: 20rpx;
}

.sheet-actions {
  display: flex;
  gap: 20rpx;
  margin-top: 36rpx;
}

.half {
  flex: 1;
}
</style>
