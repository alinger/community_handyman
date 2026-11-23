<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { onLoad } from '@dcloudio/uni-app'
import { useUserStore } from '@/store/user'
import { uploadAvatar, sendSmsCode } from '@/api'
import { ApiError, BASE_URL } from '@/utils/request'

/**
 * 资料完善页：头像、昵称、手机号。
 *
 * 三个字段都来自微信授权，但方式不同，页面上要分别处理：
 * - 头像：button open-type="chooseAvatar"，拿到的是微信头像的临时路径；
 * - 昵称：input type="nickname"，微信键盘会给出「使用微信昵称」；
 * - 手机号：button open-type="getPhoneNumber"，拿到一次性 code，后端换号。
 *
 * 微信已回收 getUserProfile / getUserInfo 的静默授权，没有「一键拿到头像昵称」的接口了，
 * 必须由用户各自点一下确认——这不是实现取巧，是平台现在的规则。
 */

const user = useUserStore()

const nickname = ref('')
/** 已上传到服务端的头像地址（相对路径） */
const avatarUrl = ref('')
/** 本地临时文件路径：选了新头像但还没上传 */
const localAvatar = ref('')
const saving = ref(false)
const uploadingAvatar = ref(false)
const bindingPhone = ref(false)

/** 短信兜底：个人主体或手机号能力未开通时用 */
const sms = reactive({ phone: '', code: '' })
const smsOpen = ref(false)
const sending = ref(false)
const countdown = ref(0)
let timer: ReturnType<typeof setInterval> | null = null

/** 展示用头像地址：本地临时图优先，其次服务端地址 */
const avatarShow = computed(() => {
  if (localAvatar.value) return localAvatar.value
  if (avatarUrl.value) return `${BASE_URL}${avatarUrl.value}`
  return ''
})

const phoneBound = computed(() => !!user.phone)
const maskedPhone = computed(() => {
  const p = user.phone
  return p.length === 11 ? `${p.slice(0, 3)}****${p.slice(7)}` : p
})

onLoad(async () => {
  nickname.value = user.nickname || ''
  avatarUrl.value = user.avatar || ''
  try {
    const me = await user.loadProfile()
    nickname.value = me.nickname || ''
    avatarUrl.value = me.avatar || ''
  } catch {
    // 拉取失败不阻断：用本地缓存值先展示，保存时会再次提交
  }
})

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

/** 选头像：微信端用 chooseAvatar，其它端退化为相册选图 */
function pickAvatar() {
  // #ifdef MP-WEIXIN
  // chooseAvatar 必须由 button open-type 触发，这里只做兜底提示
  uni.showToast({ title: '请点击头像区域选择', icon: 'none' })
  // #endif
  // #ifndef MP-WEIXIN
  uni.chooseImage({
    count: 1,
    sizeType: ['compressed'],
    success: (res) => {
      const p = res.tempFilePaths?.[0]
      if (p) localAvatar.value = p
    },
  })
  // #endif
}

function onChooseAvatar(e: any) {
  const url = e?.detail?.avatarUrl as string | undefined
  if (url) localAvatar.value = url
}

/** 微信授权手机号 */
async function onGetPhoneNumber(e: any) {
  if (bindingPhone.value) return
  const detail = e?.detail || {}
  const code = detail.code as string | undefined
  const errMsg = (detail.errMsg as string) || ''

  if (!code) {
    // 用户点「拒绝」不算故障，不弹错误弹窗
    if (errMsg.includes('deny') || errMsg.includes('cancel')) return
    // 1400001：额度用尽；no permission：主体/权限不满足
    if (detail.errno === 1400001) {
      uni.showModal({
        title: '暂时无法使用',
        content: '手机号授权次数已达上限，请改用短信验证码绑定',
        showCancel: false,
        success: () => (smsOpen.value = true),
      })
      return
    }
    uni.showModal({
      title: '手机号授权失败',
      content: '当前小程序暂不支持微信手机号授权，请改用短信验证码绑定',
      showCancel: false,
      success: () => (smsOpen.value = true),
    })
    return
  }

  bindingPhone.value = true
  try {
    await user.bindPhoneByWechat(code)
    uni.showToast({ title: '手机号已绑定', icon: 'success' })
  } catch (err) {
    const msg = err instanceof ApiError ? err.message : '请稍后重试'
    uni.showModal({
      title: '绑定失败',
      content: msg,
      showCancel: false,
      success: () => (smsOpen.value = true),
    })
  } finally {
    bindingPhone.value = false
  }
}

async function onSendCode() {
  if (sending.value || countdown.value > 0) return
  if (!/^1[3-9]\d{9}$/.test(sms.phone)) {
    uni.showToast({ title: '请输入正确的手机号', icon: 'none' })
    return
  }
  sending.value = true
  try {
    const res = await sendSmsCode({ phone: sms.phone, scene: 'login' })
    sms.code = ''
    uni.showToast({ title: '验证码已发送', icon: 'none' })
    startCountdown(res.interval || 60)
  } catch (err) {
    uni.showModal({
      title: '发送失败',
      content: err instanceof ApiError ? err.message : '请稍后重试',
      showCancel: false,
    })
  } finally {
    sending.value = false
  }
}

async function onBindBySms() {
  if (bindingPhone.value) return
  if (!/^1[3-9]\d{9}$/.test(sms.phone)) {
    uni.showToast({ title: '请输入正确的手机号', icon: 'none' })
    return
  }
  if (sms.code.length !== 6) {
    uni.showToast({ title: '请输入 6 位验证码', icon: 'none' })
    return
  }
  bindingPhone.value = true
  try {
    await user.bindPhone(sms.phone, sms.code)
    uni.showToast({ title: '手机号已绑定', icon: 'success' })
    smsOpen.value = false
  } catch (err) {
    uni.showModal({
      title: '绑定失败',
      content: err instanceof ApiError ? err.message : '请稍后重试',
      showCancel: false,
    })
  } finally {
    bindingPhone.value = false
  }
}

/** 保存：先上传头像（若有新选择），再提交昵称与头像地址 */
async function onSave(goNext = true) {
  if (saving.value) return
  const name = nickname.value.trim()
  if (!name) {
    uni.showToast({ title: '请填写昵称', icon: 'none' })
    return
  }

  saving.value = true
  try {
    let avatar = avatarUrl.value
    if (localAvatar.value) {
      uploadingAvatar.value = true
      avatar = await uploadAvatar(localAvatar.value)
      avatarUrl.value = avatar
      localAvatar.value = ''
      uploadingAvatar.value = false
    }
    await user.saveProfile(name, user.phone, avatar)
    uni.showToast({ title: '已保存', icon: 'success' })
    if (goNext) setTimeout(() => uni.reLaunch({ url: '/pages/index/index' }), 500)
  } catch (err) {
    uni.showModal({
      title: '保存失败',
      content: err instanceof ApiError ? err.message : '请稍后重试',
      showCancel: false,
    })
  } finally {
    uploadingAvatar.value = false
    saving.value = false
  }
}

function onSkip() {
  uni.reLaunch({ url: '/pages/index/index' })
}
</script>

<template>
  <view class="page">
    <view class="head">
      <view class="title">完善资料</view>
      <view class="sub">头像和昵称会让服务方更容易认出你，手机号用于上门前联系</view>
    </view>

    <!-- 头像 -->
    <view class="card sec">
      <view class="row-label">头像</view>
      <!-- #ifdef MP-WEIXIN -->
      <button class="avatar-btn" open-type="chooseAvatar" @chooseavatar="onChooseAvatar">
        <image v-if="avatarShow" class="avatar" :src="avatarShow" mode="aspectFill" />
        <view v-else class="avatar avatar-empty">点击选择</view>
      </button>
      <!-- #endif -->
      <!-- #ifndef MP-WEIXIN -->
      <view class="avatar-btn" @tap="pickAvatar">
        <image v-if="avatarShow" class="avatar" :src="avatarShow" mode="aspectFill" />
        <view v-else class="avatar avatar-empty">点击选择</view>
      </view>
      <!-- #endif -->
      <view class="row-tip">选择后使用你的微信头像，保存时上传</view>
    </view>

    <!-- 昵称 -->
    <view class="card sec">
      <view class="row-label">昵称</view>
      <!-- #ifdef MP-WEIXIN -->
      <input
        v-model="nickname"
        class="input"
        type="nickname"
        placeholder="点击输入，可用微信昵称"
        maxlength="32"
      />
      <!-- #endif -->
      <!-- #ifndef MP-WEIXIN -->
      <input v-model="nickname" class="input" placeholder="请输入昵称" maxlength="32" />
      <!-- #endif -->
      <view class="row-tip">键盘上方会提示「使用微信昵称」，点一下即可填入</view>
    </view>

    <!-- 手机号 -->
    <view class="card sec">
      <view class="row-label">手机号</view>

      <view v-if="phoneBound" class="phone-bound">{{ maskedPhone }}（已绑定）</view>

      <template v-else>
        <!-- #ifdef MP-WEIXIN -->
        <button
          class="btn-wx"
          open-type="getPhoneNumber"
          :disabled="bindingPhone"
          @getphonenumber="onGetPhoneNumber"
        >
          {{ bindingPhone ? '绑定中…' : '微信授权手机号' }}
        </button>
        <view class="row-tip">使用你微信绑定的手机号，无需填验证码</view>
        <!-- #endif -->

        <view v-if="!smsOpen" class="link-row" @tap="smsOpen = true">
          用短信验证码绑定
        </view>

        <view v-if="smsOpen" class="sms-box">
          <input
            v-model="sms.phone"
            class="input"
            type="number"
            maxlength="11"
            placeholder="输入手机号"
          />
          <view class="code-row">
            <input
              v-model="sms.code"
              class="input code-input"
              type="number"
              maxlength="6"
              placeholder="6 位验证码"
            />
            <view class="code-btn" :class="{ 'is-disabled': countdown > 0 || sending }" @tap="onSendCode">
              {{ countdown > 0 ? `${countdown}s` : '获取验证码' }}
            </view>
          </view>
          <view class="btn-primary" :class="{ 'is-disabled': bindingPhone }" @tap="onBindBySms">
            {{ bindingPhone ? '绑定中…' : '绑定手机号' }}
          </view>
        </view>
      </template>
    </view>

    <view class="btn-primary save" :class="{ 'is-disabled': saving }" @tap="onSave(true)">
      {{ saving ? '保存中…' : '保存并继续' }}
    </view>
    <view class="skip" @tap="onSkip">稍后再说</view>
  </view>
</template>

<style scoped>
.page {
  min-height: 100vh;
  padding: 24rpx 24rpx 60rpx;
}

.head {
  padding: 16rpx 8rpx 24rpx;
}

.title {
  font-size: 38rpx;
  font-weight: 800;
  color: var(--text);
}

.sub {
  margin-top: 12rpx;
  font-size: 25rpx;
  color: var(--text-sub);
  line-height: 1.7;
}

.sec {
  padding: 28rpx 32rpx;
  margin-bottom: 20rpx;
}

.row-label {
  font-size: 27rpx;
  font-weight: 700;
  color: var(--text);
  margin-bottom: 18rpx;
}

.row-tip {
  margin-top: 14rpx;
  font-size: 23rpx;
  color: var(--text-light);
  line-height: 1.6;
}

/* 微信要求 open-type 按钮保持默认可点击，这里把按钮外观重置成图片容器 */
.avatar-btn {
  display: flex;
  align-items: center;
  justify-content: flex-start;
  width: 128rpx;
  height: 128rpx;
  padding: 0;
  margin: 0;
  background: transparent;
  border: none;
  line-height: 1;
}

.avatar-btn::after {
  border: none;
}

.avatar {
  width: 128rpx;
  height: 128rpx;
  border-radius: 50%;
  background: var(--brand-light);
}

.avatar-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 22rpx;
  color: var(--brand);
  border: 2rpx dashed var(--mint-1);
}

.input {
  width: 100%;
  height: 84rpx;
  padding: 0 24rpx;
  box-sizing: border-box;
  font-size: 28rpx;
  color: var(--text);
  background: var(--bg);
  border-radius: 16rpx;
}

.phone-bound {
  font-size: 28rpx;
  color: var(--text);
}

.btn-wx {
  width: 100%;
  height: 88rpx;
  line-height: 88rpx;
  font-size: 28rpx;
  font-weight: 700;
  color: #fff;
  background: var(--brand);
  border-radius: 18rpx;
  border: none;
}

.btn-wx::after {
  border: none;
}

.btn-wx[disabled] {
  opacity: 0.6;
}

.link-row {
  margin-top: 20rpx;
  font-size: 25rpx;
  color: var(--brand);
}

.sms-box {
  margin-top: 20rpx;
}

.code-row {
  display: flex;
  align-items: center;
  margin-top: 16rpx;
}

.code-input {
  flex: 1;
}

.code-btn {
  margin-left: 16rpx;
  padding: 0 24rpx;
  height: 84rpx;
  line-height: 84rpx;
  font-size: 25rpx;
  color: var(--brand);
  background: var(--brand-light);
  border-radius: 16rpx;
  white-space: nowrap;
}

.code-btn.is-disabled {
  color: var(--text-light);
  background: var(--bg);
}

.btn-primary {
  margin-top: 20rpx;
  height: 88rpx;
  line-height: 88rpx;
  text-align: center;
  font-size: 28rpx;
  font-weight: 700;
  color: #fff;
  background: var(--brand);
  border-radius: 18rpx;
}

.btn-primary.is-disabled {
  opacity: 0.6;
}

.save {
  margin-top: 32rpx;
}

.skip {
  margin-top: 24rpx;
  text-align: center;
  font-size: 25rpx;
  color: var(--text-sub);
}
</style>
