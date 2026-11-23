<script setup lang="ts">
import { APP_VERSION } from '@/utils/version'
import { computed, ref } from 'vue'
import { onShow } from '@dcloudio/uni-app'
import TabBar from '@/components/TabBar.vue'
import { useUserStore } from '@/store/user'
import { ensureAuth, requireLogin } from '@/utils/auth'
import {
  BASE_URL,
  APP_ENV,
  apiEnv,
  setApiEnv,
  resetApiEnv,
  envEntryOn,
  setEnvEntryOn,
  clearToken,
  clearUserId,
} from '@/utils/request'

const user = useUserStore()

/** 头像地址：为空时退回昵称首字圆形占位 */
const avatarSrc = computed(() => (user.avatar ? `${BASE_URL}${user.avatar}` : ''))

/** 手机号脱敏：首页是高频曝光页面，不展示完整号码 */
const maskedPhone = computed(() => {
  const p = user.phone
  if (!p) return ''
  return p.length === 11 ? `${p.slice(0, 3)}****${p.slice(7)}` : p
})

/* ---------------- 接口环境调试入口 ----------------
 * 默认隐藏，连点「关于我们」右侧版本号 5 次打开；打开状态存在本机，
 * 只影响当前设备，不会同步给其他用户。用于真机连开发机、体验版切环境验证。
 */
const envEntry = ref(envEntryOn())
const envInfo = ref(apiEnv())

/**
 * 「关于我们」右侧的环境标签只反映构建环境。
 * 一旦运行期切过环境，标签就与真实请求地址不符了，此时隐藏它，
 * 统一看下面的调试卡，避免「标签写本地、实际连生产」这种误导。
 */
const buildEnvTag = computed(() => {
  if (envInfo.value.mode !== 'build') return ''
  return APP_ENV && APP_ENV !== '生产' ? APP_ENV : ''
})
let verTaps = 0
let verTapTimer: ReturnType<typeof setTimeout> | null = null

/** 连点版本号 5 次（间隔 1.2 秒内）作为隐藏开关，避免给正式用户看到调试项 */
function onVersionTap() {
  verTaps += 1
  if (verTapTimer) clearTimeout(verTapTimer)
  verTapTimer = setTimeout(() => {
    verTaps = 0
  }, 1200)
  if (verTaps < 5) return
  verTaps = 0
  setEnvEntryOn(true)
  envEntry.value = true
  envInfo.value = apiEnv()
  uni.showToast({ title: '调试入口已开启', icon: 'none' })
}

function onToggleEntry(e: Event) {
  // switch 的 change 事件在 uni 里带 detail.value，模板类型只声明成 Event，这里取一次
  const on = (e as unknown as { detail: { value: boolean } }).detail.value
  setEnvEntryOn(on)
  envEntry.value = on
  if (!on) {
    uni.showToast({ title: '已隐藏，连点版本号 5 次可再打开', icon: 'none' })
  }
}

/** 真正应用环境：清掉旧环境的登录态，回首页重新走一遍登录 */
function applyEnv(mode: 'build' | 'prod' | 'local' | 'custom', custom = '') {
  const next = mode === 'build' ? resetApiEnv() : setApiEnv(mode, custom)
  envInfo.value = next
  clearToken()
  clearUserId()
  uni.showToast({ title: `已切到${next.label}，需重新登录`, icon: 'none' })
  setTimeout(() => uni.reLaunch({ url: '/pages/index/index' }), 800)
}

function chooseEnv() {
  uni.showActionSheet({
    itemList: ['生产环境（m.yubangbzz.com）', '本地开发机（127.0.0.1:8080）', '自定义地址…', '恢复构建默认'],
    success: ({ tapIndex }) => {
      if (tapIndex === 0) {
        uni.showModal({
          title: '切到生产环境？',
          content: '生产环境是真实数据，改动会影响线上订单与资金。仅排查问题时使用。',
          confirmText: '仍要切换',
          success: (r) => {
            if (r.confirm) applyEnv('prod')
          },
        })
        return
      }
      if (tapIndex === 1) {
        applyEnv('local')
        return
      }
      if (tapIndex === 2) {
        // editable 需要基础库 2.17.1+；H5 不支持时 content 为空，直接忽略
        uni.showModal({
          title: '自定义接口地址',
          editable: true,
          placeholderText: '如 192.168.1.10:8080',
          success: (r) => {
            const v = (r.content || '').trim()
            if (!r.confirm || !v) return
            applyEnv('custom', v)
          },
        } as UniApp.ShowModalOptions)
        return
      }
      applyEnv('build')
    },
  })
}

function goProfile() {
  uni.navigateTo({ url: '/pages/auth/profile' })
}

function goScope() {
  uni.navigateTo({ url: '/pages/scope/index' })
}

onShow(async () => {
  envInfo.value = apiEnv()
  if (!ensureAuth()) return
  // 访客态：不调需要登录态的接口，页面按未登录渲染
  if (user.isGuest) return
  try {
    await user.ensureLogin()
    await Promise.all([user.loadAddresses(), user.loadProfile()])
  } catch (e) {
    console.warn('[mine] 初始化失败', e)
  }
})

function goAddress() {
  if (!requireLogin()) return
  uni.navigateTo({ url: '/pages/address/index' })
}

function goLogin() {
  uni.navigateTo({ url: '/pages/auth/login?reason=auth' })
}

/** 设置入口放在个人信息区右上角，符合主流 App 的入口习惯 */
function goSettings() {
  if (!requireLogin()) return
  uni.navigateTo({ url: '/pages/settings/index' })
}

function goOrders() {
  if (!requireLogin()) return
  uni.reLaunch({ url: '/pages/order/list' })
}

function callService() {
  uni.showModal({
    title: '联系客服',
    content: '客服电话：021-0000 0000（工作日 9:00-18:00）',
    showCancel: false,
  })
}

/** 静态条款页：服务条款 / 隐私政策 / 关于我们，内容集中在一个只读页里按 type 渲染 */
function goDoc(type: 'terms' | 'privacy' | 'about') {
  uni.navigateTo({ url: `/pages/doc/index?type=${type}` })
}

/** 服务方认证申请/查看状态 */
function goProviderApply() {
  uni.navigateTo({ url: '/pages/provider/apply' })
}

/** 已认证服务方的工作台 */
function goProviderWork() {
  uni.reLaunch({ url: '/pages/provider/tasks' })
}

/**
 * 切换身份视角。已认证用户既是住户也是服务方，
 * 这里切的是「看哪一侧」，不动后端权限。
 * 胶囊按钮由 v-if="user.isProvider" 控制渲染，未认证时根本点不到。
 */
function onSwitchView() {
  if (!user.isProvider) return
  user.toggleView()
}
</script>

<template>
  <view class="page">
    <!-- 访客态：不展示用户资料，直接给登录引导 -->
    <view v-if="user.isGuest" class="profile guest-profile" @tap="goLogin">
      <view class="avatar guest-avatar">访</view>
      <view class="profile-info">
        <view class="name-row">
          <text class="nickname">访客</text>
        </view>
        <view class="uid">登录后可预约服务、查看订单与地址</view>
      </view>
      <view class="gear">
        <text class="gear-text">去登录</text>
        <text class="gear-chev">›</text>
      </view>
    </view>

    <!-- 个人信息：右上角进设置 -->
    <view v-else class="profile">
      <image v-if="avatarSrc" class="avatar" :src="avatarSrc" mode="aspectFill" @tap="goProfile" />
      <view v-else class="avatar" @tap="goProfile">{{ (user.nickname || '住')[0] }}</view>
      <view class="profile-info" @tap="goProfile">
        <!-- 用户名后面放身份切换：只有已认证服务方才渲染 -->
        <view class="name-row">
          <text class="nickname">{{ user.nickname || '住户' }}</text>
          <view
            v-if="user.isProvider"
            class="role-pill"
            :class="{ provider: user.isProviderView }"
            @tap.stop="onSwitchView"
          >
            <text class="pill-swap">⇅</text>
            <text class="pill-text">{{ user.isProviderView ? '切为住户' : '切为服务方' }}</text>
          </view>
          <view v-else class="role-pill ghost" @tap.stop="goProviderApply">
            <text class="pill-text">认证服务方</text>
          </view>
        </view>
        <view class="uid">
          ID {{ user.userId || '-' }}
          <text v-if="maskedPhone" class="phone">· {{ maskedPhone }}</text>
        </view>
      </view>
      <view class="gear" @tap="goSettings">
        <text class="gear-text">设置</text>
        <text class="gear-chev">›</text>
      </view>
    </view>

    <!-- 快捷入口 -->
    <view class="card menu">
      <view class="menu-item" @tap="goOrders">
        <text>我的订单</text>
        <text class="chev">›</text>
      </view>
      <view class="menu-item" @tap="goAddress">
        <text>常用地址</text>
        <view class="menu-right">
          <text class="menu-value">{{ user.currentAddress?.community || '未添加' }}</text>
          <text class="chev">›</text>
        </view>
      </view>
      <view class="menu-item" @tap="callService">
        <text>联系客服</text>
        <view class="menu-right">
          <text class="menu-value">021-0000 0000</text>
          <text class="chev">›</text>
        </view>
      </view>
      <!-- 认证通过后可在服务方视角直接进工作台；未认证走申请 -->
      <view v-if="user.isProvider" class="menu-item" @tap="goProviderWork">
        <text>服务方工作台</text>
        <view class="menu-right">
          <text class="menu-value">已认证</text>
          <text class="chev">›</text>
        </view>
      </view>
      <view v-else class="menu-item" @tap="goProviderApply">
        <text>成为服务方</text>
        <view class="menu-right">
          <text class="menu-value">认证后接单</text>
          <text class="chev">›</text>
        </view>
      </view>
    </view>

    <!-- 关于与条款 -->
    <view class="card menu">
      <view class="menu-item" @tap="goDoc('terms')">
        <text>服务条款</text>
        <text class="chev">›</text>
      </view>
      <view class="menu-item" @tap="goDoc('privacy')">
        <text>隐私政策</text>
        <text class="chev">›</text>
      </view>
      <view class="menu-item" @tap="goDoc('about')">
        <text>关于我们</text>
        <view class="menu-right">
          <!-- 连点 5 次打开接口环境调试入口，正式用户基本不会误触 -->
          <text class="menu-value" @tap.stop="onVersionTap">v{{ APP_VERSION }}</text>
          <text v-if="buildEnvTag" class="env-tag">{{ buildEnvTag }}</text>
          <text class="chev">›</text>
        </view>
      </view>
    </view>

    <!-- 接口环境调试：默认隐藏，只影响本机；切换后清登录态回首页 -->
    <view v-if="envEntry" class="card menu">
      <view class="menu-item" @tap="chooseEnv">
        <text>接口环境</text>
        <view class="menu-right">
          <text class="env-chip" :class="envInfo.mode">{{ envInfo.label }}</text>
          <text class="menu-value">{{ envInfo.short }}</text>
          <text class="chev">›</text>
        </view>
      </view>
      <view class="menu-item">
        <text>显示调试入口</text>
        <switch :checked="envEntry" color="#0f7f78" class="sw" @change="onToggleEntry" />
      </view>
      <view class="env-hint">
        切换会清掉登录态并回到首页；只对本机生效，关闭后连点「关于我们」右侧版本号 5 次可重新打开。H5 预览切生产会被跨域拦截，属正常现象，真机不受影响
      </view>
    </view>

    <!-- 服务范围：独立页面，按小区卡片展示 -->
    <view class="card scope-entry" @tap="goScope">
      <view class="scope-entry-main">
        <view class="section-title">服务范围</view>
        <view class="muted">查看已开通的小区、路段与门牌区间</view>
      </view>
      <text class="chev">›</text>
    </view>

    <TabBar current="mine" />
  </view>
</template>

<style scoped>
.page {
  padding-top: 24rpx;
  /* 给固定 TabBar 留位置，否则最后一张卡片会被遮挡 */
  padding-bottom: 200rpx;
}

.profile {
  display: flex;
  align-items: center;
  gap: 24rpx;
  padding: 24rpx 32rpx 32rpx;
}

.guest-avatar {
  background: #d9e2e0;
  color: #6b7c79;
}

.avatar {
  width: 104rpx;
  height: 104rpx;
  border-radius: 50%;
  background: var(--brand-light);
  color: var(--brand-dark);
  font-size: 40rpx;
  font-weight: 600;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.profile-info {
  flex: 1;
  min-width: 0;
}

/* 用户名 + 身份切换并排 */
.name-row {
  display: flex;
  align-items: center;
  gap: 14rpx;
  min-width: 0;
}

.nickname {
  font-size: 34rpx;
  font-weight: 600;
  max-width: 260rpx;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

/* 身份切换胶囊：已认证服务方显示，点击在两侧视角间切换 */
.role-pill {
  display: flex;
  align-items: center;
  gap: 4rpx;
  height: 40rpx;
  padding: 0 14rpx;
  border-radius: 20rpx;
  background: var(--brand-light);
  border: 1rpx solid #b9ded8;
  flex-shrink: 0;
}

.role-pill:active {
  opacity: 0.75;
}

/* 处于服务方视角时反色，一眼能看出当前在哪一侧 */
.role-pill.provider {
  background: var(--brand);
  border-color: var(--brand);
}

.pill-swap {
  font-size: 20rpx;
  color: var(--brand-dark);
  line-height: 1;
}

.role-pill.provider .pill-swap {
  color: #fff;
}

.pill-text {
  font-size: 20rpx;
  color: var(--brand-dark);
  font-weight: 600;
  line-height: 1;
}

.role-pill.provider .pill-text {
  color: #fff;
}

/* 未认证：虚线弱化，提示可去认证 */
.role-pill.ghost {
  background: #fff;
  border: 1rpx dashed #cfd8d6;
}

.role-pill.ghost .pill-text {
  color: var(--text-sub);
  font-weight: 400;
}

.uid {
  font-size: 23rpx;
  color: var(--text-light);
  margin-top: 6rpx;
}

.phone {
  color: var(--text-sub);
}

/* 设置入口：文字按钮比齿轮更直白，避免 CSS 画出的图标被误认为准星 */
.gear {
  height: 56rpx;
  padding: 0 22rpx;
  border-radius: 28rpx;
  display: flex;
  align-items: center;
  gap: 4rpx;
  flex-shrink: 0;
  background: #f4f7f6;
  color: var(--text-sub);
}

.gear:active {
  background: #e8eeec;
}

.gear-text {
  font-size: 24rpx;
  color: var(--text-sub);
}

.gear-chev {
  font-size: 26rpx;
  color: var(--text-light);
  margin-top: -3rpx;
}

.menu {
  padding: 0 32rpx;
}

.menu-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 28rpx 0;
  border-bottom: 1rpx solid #f3f4f6;
  font-size: 28rpx;
}

.menu-item:last-child {
  border-bottom: none;
}

.menu-right {
  display: flex;
  align-items: center;
  gap: 12rpx;
}

.menu-value {
  font-size: 24rpx;
  color: var(--text-light);
  max-width: 280rpx;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.chev {
  color: var(--text-light);
}

/* 环境标签：只在本地的构建里出现，方便一眼区分连的是哪套接口 */
.env-tag {
  margin-right: 8rpx;
  padding: 2rpx 12rpx;
  font-size: 20rpx;
  color: var(--accent);
  background: var(--accent-soft);
  border-radius: 999rpx;
}

/* 调试卡：环境胶囊用颜色区分，生产用警示色，避免误以为自己还在本地 */
.env-chip {
  padding: 2rpx 14rpx;
  font-size: 20rpx;
  border-radius: 999rpx;
  color: var(--text-sub);
  background: #f1f4f3;
}

.env-chip.prod {
  color: #b45309;
  background: #fef3c7;
}

.env-chip.local {
  color: #0f7f78;
  background: #e7f5f3;
}

.env-hint {
  padding: 4rpx 0 24rpx;
  font-size: 21rpx;
  line-height: 1.5;
  color: var(--text-light);
}

.sw {
  transform: scale(0.8);
  transform-origin: right center;
}

/* 服务范围入口卡：整卡可点，右侧箭头提示可进入 */
.scope-entry {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20rpx;
}

.scope-entry-main {
  display: flex;
  flex-direction: column;
  gap: 10rpx;
}


</style>
