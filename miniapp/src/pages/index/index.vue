<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { onShow } from '@dcloudio/uni-app'
import TabBar from '@/components/TabBar.vue'
import { useUserStore } from '@/store/user'
import { ensureAuth } from '@/utils/auth'
import { SLOTS } from '@/utils/format'

const user = useUserStore()
const statusBarHeight = ref(0)
const menuOpen = ref(false)

/**
 * 服务入口。
 * 首页不展示价格：设计稿把价格信息留给了预约页的费用估算，
 * 首页只承担「介绍与入口」的作用。
 */
const services = [
  { type: 'garbage', name: '代倒垃圾', icon: '🗑', desc: '专业准时，便捷省心', tone: 'green' },
  { type: 'dogwalk', name: '遛宠服务', icon: '🐾', desc: '贴心陪伴，放心出行', tone: 'amber' },
  { type: 'accompany', name: '陪诊服务', icon: '🏥', desc: '贴心陪同，安心就医', tone: 'blue' },
]

const steps = [
  { icon: '📋', label: '选服务' },
  { icon: '📅', label: '约时间' },
  { icon: '🏠', label: '上门服务' },
]

onMounted(() => {
  // 自定义导航栏需要自己避开状态栏
  const info = uni.getSystemInfoSync()
  statusBarHeight.value = info.statusBarHeight || 0
})

onShow(async () => {
  if (!ensureAuth()) return
  try {
    await user.ensureLogin()
    if (user.role === 'provider') {
      uni.reLaunch({ url: '/pages/provider/tasks' })
      return
    }
    await user.loadAddresses()
  } catch (e) {
    console.warn('[index] 加载失败', e)
  }
})

function goCreate(type: string) {
  uni.navigateTo({ url: `/pages/order/create?type=${type}` })
}

function goMine() {
  uni.reLaunch({ url: '/pages/mine/index' })
}

/** 汉堡菜单：常用入口侧边抽屉，替代原先的「功能开发中」空提示 */
function openMenu() {
  menuOpen.value = true
}

function closeMenu() {
  menuOpen.value = false
}

function fromMenu(fn: () => void) {
  closeMenu()
  // 等抽屉收起动画结束再跳转，避免跳转被遮罩打断
  setTimeout(fn, 160)
}

const menuItems = [
  { icon: '📋', label: '我的订单', go: () => uni.reLaunch({ url: '/pages/order/list' }) },
  { icon: '📍', label: '常用地址', go: () => uni.navigateTo({ url: '/pages/address/index' }) },
  { icon: '⚙️', label: '设置', go: () => uni.navigateTo({ url: '/pages/settings/index' }) },
  { icon: '📄', label: '服务条款', go: () => uni.navigateTo({ url: '/pages/doc/index?type=terms' }) },
  { icon: '🔒', label: '隐私政策', go: () => uni.navigateTo({ url: '/pages/doc/index?type=privacy' }) },
  { icon: '☎️', label: '联系客服', go: () => uni.showModal({ title: '联系客服', content: '客服电话：021-0000 0000（工作日 9:00-18:00）', showCancel: false }) },
]
</script>

<template>
  <view class="page">
    <!-- 自定义导航：汉堡 / 标题 / 头像 -->
    <view class="nav" :style="{ paddingTop: statusBarHeight + 'px' }">
      <view class="nav-inner">
        <view class="nav-btn" @tap="openMenu">
          <view class="burger"><view /><view /><view /></view>
        </view>
        <text class="nav-title">小区好帮手</text>
        <view class="nav-btn" @tap="goMine">
          <view class="avatar"><view class="head" /><view class="shoulder" /></view>
        </view>
      </view>
    </view>

    <!-- 定位 + 主标题 -->
    <view class="intro">
      <view class="locate" @tap="goMine">
        <view class="pin" />
        <text class="locate-text">{{ user.currentAddress?.community || '示例小区' }}</text>
        <text class="caret">⌄</text>
      </view>
      <view class="hero-title">把琐事交给我们</view>
      <view class="hero-sub">家门口的安心服务</view>
    </view>

    <!-- 插画 banner -->
    <view class="banner">
      <view class="banner-copy">
        <view class="banner-title">邻里生活，<br />轻松一点</view>
        <view class="banner-rule" />
        <view class="banner-sub">专业服务 · 温暖社区</view>
      </view>
      <!-- 设计稿此处为 3D 插画，暂无设计资源，用 emoji 组合占位 -->
      <view class="banner-art">🏘️</view>
    </view>

    <!-- 三个服务入口 -->
    <view class="services">
      <view
        v-for="s in services"
        :key="s.type"
        class="svc"
        :class="`tone-${s.tone}`"
        @tap="goCreate(s.type)"
      >
        <view class="svc-icon">{{ s.icon }}</view>
        <view class="svc-name">{{ s.name }}</view>
        <view class="svc-desc">{{ s.desc }}</view>
      </view>
    </view>

    <!-- 提示条 -->
    <view class="notice">
      <text class="notice-icon">🐕</text>
      <text class="notice-text">遛宠需牵引 · 猫咪按习性自愿外出</text>
    </view>

    <!-- 预约很简单 -->
    <view class="block-head">
      <view class="sq green" />
      <text class="block-title">预约很简单</text>
      <view class="block-line" />
    </view>
    <view class="steps">
      <template v-for="(s, i) in steps" :key="s.label">
        <view class="step">
          <view class="step-icon">{{ s.icon }}</view>
          <text class="step-label">{{ s.label }}</text>
        </view>
        <text v-if="i < steps.length - 1" class="step-arrow">›</text>
      </template>
    </view>

    <!-- 可预约时段 -->
    <view class="block-head">
      <view class="sq orange" />
      <text class="block-title">可预约时段</text>
      <view class="block-line" />
    </view>
    <view class="slots">
      <view v-for="s in SLOTS" :key="s.code" class="slot">{{ s.label }}</view>
    </view>

    <TabBar current="index" />

    <!-- 汉堡菜单抽屉 -->
    <view v-if="menuOpen" class="drawer-mask" @tap="closeMenu">
      <view class="drawer" @tap.stop>
        <view class="drawer-head" :style="{ paddingTop: statusBarHeight + 20 + 'px' }">
          <view class="drawer-avatar">{{ (user.nickname || '住')[0] }}</view>
          <view class="drawer-info">
            <view class="drawer-name">{{ user.nickname || '住户' }}</view>
            <view class="drawer-addr">{{ user.currentAddress?.community || '未设置地址' }}</view>
          </view>
        </view>
        <view class="drawer-list">
          <view
            v-for="m in menuItems"
            :key="m.label"
            class="drawer-item"
            @tap="fromMenu(m.go)"
          >
            <text class="drawer-icon">{{ m.icon }}</text>
            <text class="drawer-label">{{ m.label }}</text>
            <text class="chev">›</text>
          </view>
        </view>
        <view class="drawer-foot">小区好帮手 · v1.0.0</view>
      </view>
    </view>
  </view>
</template>

<style scoped>
.page {
  background: var(--bg);
  padding-bottom: 160rpx;
}

/* ---------- 导航 ---------- */
.nav {
  background: #fff;
}

.nav-inner {
  height: 88rpx;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 28rpx;
}

.nav-btn {
  width: 64rpx;
  height: 64rpx;
  display: flex;
  align-items: center;
  justify-content: center;
}

.nav-title {
  font-size: 34rpx;
  font-weight: 600;
  color: var(--brand-dark);
  letter-spacing: 1rpx;
}

.burger {
  display: flex;
  flex-direction: column;
  gap: 7rpx;
}

.burger view {
  width: 34rpx;
  height: 3.5rpx;
  background: var(--text);
  border-radius: 2rpx;
}

.avatar {
  width: 44rpx;
  height: 44rpx;
  border: 2rpx solid #c9d2d0;
  border-radius: 50%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 2rpx;
}

.avatar .head {
  width: 11rpx;
  height: 11rpx;
  border: 2rpx solid #7d8a88;
  border-radius: 50%;
  box-sizing: border-box;
}

.avatar .shoulder {
  width: 20rpx;
  height: 9rpx;
  border: 2rpx solid #7d8a88;
  border-bottom: none;
  border-radius: 12rpx 12rpx 0 0;
  box-sizing: border-box;
}

/* ---------- 标题区 ---------- */
.intro {
  padding: 24rpx 32rpx 0;
  background: #fff;
}

.locate {
  display: flex;
  align-items: center;
  gap: 8rpx;
}

.pin {
  width: 16rpx;
  height: 16rpx;
  border-radius: 50% 50% 50% 0;
  transform: rotate(-45deg);
  background: var(--brand);
  flex-shrink: 0;
}

.locate-text {
  font-size: 26rpx;
  color: var(--text-sub);
}

.caret {
  font-size: 24rpx;
  color: var(--text-light);
  margin-top: -6rpx;
}

.hero-title {
  margin-top: 18rpx;
  font-size: 58rpx;
  font-weight: 800;
  letter-spacing: -1rpx;
  color: #1a1d21;
  line-height: 1.2;
}

.hero-sub {
  margin-top: 12rpx;
  font-size: 26rpx;
  color: var(--text-sub);
}

/* ---------- banner ---------- */
.banner {
  position: relative;
  margin: 24rpx 24rpx 0;
  height: 250rpx;
  border-radius: 22rpx;
  background: linear-gradient(135deg, #87c9bd 0%, #cbeae3 100%);
  overflow: hidden;
  display: flex;
  align-items: center;
  padding: 0 32rpx;
}

.banner-copy {
  position: relative;
  z-index: 2;
}

.banner-title {
  font-size: 44rpx;
  font-weight: 800;
  color: #fff;
  line-height: 1.3;
  letter-spacing: 1rpx;
  text-shadow: 0 2rpx 8rpx rgba(15, 127, 120, 0.18);
}

.banner-rule {
  width: 56rpx;
  height: 3rpx;
  background: rgba(255, 255, 255, 0.85);
  margin: 18rpx 0 14rpx;
}

.banner-sub {
  font-size: 22rpx;
  color: rgba(255, 255, 255, 0.95);
  letter-spacing: 1rpx;
}

.banner-art {
  position: absolute;
  right: 18rpx;
  bottom: -6rpx;
  font-size: 168rpx;
  line-height: 1;
  opacity: 0.96;
  z-index: 1;
}

/* ---------- 服务入口 ---------- */
.services {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 16rpx;
  padding: 0 24rpx;
  margin-top: 20rpx;
}

.svc {
  background: #fff;
  border-radius: 20rpx;
  padding: 26rpx 12rpx 24rpx;
  text-align: center;
  min-width: 0;
}

.svc-icon {
  width: 88rpx;
  height: 88rpx;
  margin: 0 auto 14rpx;
  border-radius: 50%;
  font-size: 46rpx;
  line-height: 88rpx;
  text-align: center;
}

.tone-green .svc-icon {
  background: #e4f3f0;
}

.tone-amber .svc-icon {
  background: #fdf0dd;
}

.tone-blue .svc-icon {
  background: #e6effa;
}

.svc-name {
  font-size: 27rpx;
  font-weight: 600;
  color: var(--text);
}

.svc-desc {
  margin-top: 6rpx;
  font-size: 20rpx;
  color: var(--text-light);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

/* ---------- 提示条 ---------- */
.notice {
  display: flex;
  align-items: center;
  gap: 10rpx;
  margin: 18rpx 24rpx 0;
  padding: 16rpx 22rpx;
  background: var(--accent-soft);
  border: 1rpx solid #f3d9b4;
  border-radius: 14rpx;
}

.notice-icon {
  font-size: 24rpx;
}

.notice-text {
  font-size: 22rpx;
  color: #b45309;
}

/* ---------- 区块标题 ---------- */
.block-head {
  display: flex;
  align-items: center;
  gap: 12rpx;
  padding: 0 24rpx;
  margin: 34rpx 0 18rpx;
}

.sq {
  width: 26rpx;
  height: 26rpx;
  border-radius: 7rpx;
}

.sq.green {
  background: var(--brand);
}

.sq.orange {
  background: #e8a33d;
  border-radius: 50%;
}

.block-title {
  font-size: 29rpx;
  font-weight: 700;
  color: var(--text);
}

.block-line {
  flex: 1;
  height: 1rpx;
  background: #e9ece9;
  margin-left: 6rpx;
}

/* ---------- 三步 ---------- */
.steps {
  display: flex;
  align-items: center;
  padding: 0 24rpx;
  gap: 8rpx;
}

.step {
  flex: 1;
  background: #fff;
  border-radius: 18rpx;
  padding: 22rpx 8rpx;
  text-align: center;
  min-width: 0;
}

.step-icon {
  font-size: 36rpx;
  line-height: 1.2;
}

.step-label {
  display: block;
  margin-top: 10rpx;
  font-size: 24rpx;
  color: var(--text);
}

.step-arrow {
  color: #c2cbc9;
  font-size: 32rpx;
  flex-shrink: 0;
}

/* ---------- 时段 ---------- */
.slots {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 14rpx;
  padding: 0 24rpx;
}

.slot {
  background: #fffaf2;
  border: 1rpx solid #f0d6ae;
  border-radius: 14rpx;
  padding: 18rpx 0;
  text-align: center;
  font-size: 24rpx;
  color: #b45309;
  font-weight: 500;
}

/* ---------- 汉堡菜单抽屉 ---------- */
.drawer-mask {
  position: fixed;
  inset: 0;
  background: rgba(15, 23, 42, 0.45);
  z-index: 300;
  display: flex;
}

.drawer {
  width: 560rpx;
  height: 100%;
  background: #fff;
  border-radius: 0 28rpx 28rpx 0;
  display: flex;
  flex-direction: column;
  padding-bottom: env(safe-area-inset-bottom);
}

.drawer-head {
  display: flex;
  align-items: center;
  gap: 20rpx;
  padding: 0 32rpx 32rpx;
  background: linear-gradient(135deg, #e9f5f3 0%, #f5fbfa 100%);
}

.drawer-avatar {
  width: 88rpx;
  height: 88rpx;
  border-radius: 50%;
  background: #fff;
  color: var(--brand-dark);
  font-size: 36rpx;
  font-weight: 600;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.drawer-info {
  min-width: 0;
}

.drawer-name {
  font-size: 30rpx;
  font-weight: 600;
  color: var(--text);
}

.drawer-addr {
  margin-top: 6rpx;
  font-size: 22rpx;
  color: var(--text-sub);
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.drawer-list {
  flex: 1;
  padding: 12rpx 16rpx;
}

.drawer-item {
  display: flex;
  align-items: center;
  gap: 18rpx;
  padding: 26rpx 16rpx;
  border-radius: 14rpx;
}

.drawer-item:active {
  background: #f5f8f7;
}

.drawer-icon {
  font-size: 30rpx;
  width: 40rpx;
  text-align: center;
}

.drawer-label {
  flex: 1;
  font-size: 28rpx;
  color: var(--text);
}

.drawer .chev {
  color: var(--text-light);
}

.drawer-foot {
  text-align: center;
  font-size: 21rpx;
  color: var(--text-light);
  padding: 24rpx 0;
}
</style>
