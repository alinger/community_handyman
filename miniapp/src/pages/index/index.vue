<script setup lang="ts">
import { APP_VERSION } from '@/utils/version'
import { computed, onMounted, ref } from 'vue'
import { onShow } from '@dcloudio/uni-app'
import TabBar from '@/components/TabBar.vue'
import { useUserStore } from '@/store/user'
import { ensureAuth, requireLogin } from '@/utils/auth'
import { SLOTS } from '@/utils/format'

const user = useUserStore()
const statusBarHeight = ref(0)
const menuOpen = ref(false)

/**
 * 服务入口。
 * 首页不展示价格：设计稿把价格信息留给了预约页的费用估算，
 * 首页只承担「介绍与入口」的作用。
 * 图标改为 CSS 绘制的几何图形 —— emoji 在不同机型上字重、色彩差异很大，
 * 三张白卡并排时视觉噪音明显，用统一描边图标更稳。
 * soon: true 的服务只展示不可进入（点一下提示敬请期待）。
 */
const services = [
  {
    type: 'garbage',
    name: '代倒垃圾',
    desc: '专业准时 · 便捷省心',
    tone: 'green',
    tag: '按月计费',
  },
  {
    type: 'dogwalk',
    name: '遛宠服务',
    desc: '贴心陪伴 · 放心出行',
    tone: 'amber',
    tag: '20 分钟起',
  },
  {
    type: 'accompany',
    name: '陪诊服务',
    desc: '贴心陪同 · 安心就医',
    tone: 'blue',
    tag: '敬请期待',
    soon: true,
  },
  {
    type: 'kidride',
    name: '接送小孩',
    desc: '准时接送 · 安全放心',
    tone: 'purple',
    tag: '敬请期待',
    soon: true,
  },
  {
    type: 'express',
    name: '代取快递',
    desc: '快递代取 · 省时省力',
    tone: 'cyan',
    tag: '敬请期待',
    soon: true,
  },
]

const steps = [
  { no: '1', label: '选服务', desc: '挑一项' },
  { no: '2', label: '约时间', desc: '选时段' },
  { no: '3', label: '上门服务', desc: '在家等' },
]

/** 问候语随时段变化，比固定的「把琐事交给我们」更有温度 */
const greeting = computed(() => {
  const h = new Date().getHours()
  if (h < 6) return '夜深了'
  if (h < 11) return '早上好'
  if (h < 14) return '中午好'
  if (h < 18) return '下午好'
  return '晚上好'
})

onMounted(() => {
  // 自定义导航栏需要自己避开状态栏
  const info = uni.getSystemInfoSync()
  statusBarHeight.value = info.statusBarHeight || 0
})

onShow(async () => {
  if (!ensureAuth()) return
  // 访客态：不拉地址（需要登录态），页面按无地址渲染
  if (user.isGuest) return
  try {
    await user.ensureLogin()
    // 只有「服务方视角」才跳工作台；已认证但处于住户视角时正常展示首页，
    // 否则服务方永远无法给自己下单
    if (user.isProviderView) {
      uni.reLaunch({ url: '/pages/provider/tasks' })
      return
    }
    await user.loadAddresses()
  } catch (e) {
    console.warn('[index] 加载失败', e)
  }
})

function onServiceTap(s: (typeof services)[number]) {
  // 未上线的服务只提示，不进入预约页
  if (s.soon) {
    uni.showToast({ title: '敬请期待', icon: 'none' })
    return
  }
  goCreate(s.type)
}

function goCreate(type: string) {
  // 访客/未登录：先问要不要登录，避免直接跳走打断浏览
  if (!requireLogin()) return
  uni.navigateTo({ url: `/pages/order/create?type=${type}` })
}

function goLogin() {
  uni.navigateTo({ url: '/pages/auth/login' })
}

function goMine() {
  uni.reLaunch({ url: '/pages/mine/index' })
}

function goOrders() {
  uni.reLaunch({ url: '/pages/order/list' })
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
  { icon: '🗺️', label: '服务范围', go: () => uni.navigateTo({ url: '/pages/scope/index' }) },
  { icon: '⚙️', label: '设置', go: () => uni.navigateTo({ url: '/pages/settings/index' }) },
  { icon: '📄', label: '服务条款', go: () => uni.navigateTo({ url: '/pages/doc/index?type=terms' }) },
  { icon: '🔒', label: '隐私政策', go: () => uni.navigateTo({ url: '/pages/doc/index?type=privacy' }) },
  { icon: '☎️', label: '联系客服', go: () => uni.showModal({ title: '联系客服', content: '客服电话：021-0000 0000（工作日 9:00-18:00）', showCancel: false }) },
]
</script>

<template>
  <view class="page">
    <!-- 顶部导航：汉堡 / 标题 / 头像，背景与头部渐变衔接 -->
    <view class="nav" :style="{ paddingTop: statusBarHeight + 'px' }">
      <view class="nav-inner">
        <view class="nav-btn" @tap="openMenu">
          <view class="burger"><view /><view /><view /></view>
        </view>
        <text class="nav-title">邻有回应</text>
        <text v-if="user.isGuest" class="guest-tag">访客</text>
        <view class="nav-btn" @tap="goMine">
          <view class="avatar">
            <text class="avatar-char">{{ (user.nickname || '住')[0] }}</text>
          </view>
        </view>
      </view>
    </view>

    <!-- 访客横幅：明确告知当前是浏览模式，并给一个显眼登录入口 -->
    <view v-if="user.isGuest" class="guest-banner" @tap="goLogin">
      <text class="guest-banner-text">当前为访客模式，登录后可预约服务、查看订单</text>
      <text class="guest-banner-btn">去登录 ›</text>
    </view>

    <!-- 头部：问候 + 主标题（定位行已按要求隐藏） -->
    <view class="hero">
      <view class="hero-greet">
        <text class="greet-text">{{ greeting }}</text>
        <text v-if="user.nickname" class="greet-name">，{{ user.nickname }}</text>
      </view>
      <view class="hero-title">把琐事<text class="hl">交给我们</text></view>
      <view class="hero-sub">代倒垃圾 · 遛宠 · 陪诊，家门口的安心服务</view>
    </view>

    <!-- 主视觉卡片：叠在渐变上与下方内容形成层次 -->
    <view class="banner">
      <view class="banner-glow" />
      <view class="banner-copy">
        <view class="banner-badge">社区专享</view>
        <view class="banner-title">邻里生活<br />轻松一点</view>
        <view class="banner-sub">12 个小区 · 专业服务 · 温暖社区</view>
      </view>
      <view class="banner-art">
        <view class="art-house">
          <view class="art-roof" />
          <view class="art-body">
            <view class="art-door" />
            <view class="art-window" />
          </view>
        </view>
        <view class="art-tree" />
      </view>
    </view>

    <!-- 五个服务入口：三个未上线（敬请期待），点了只提示不进入 -->
    <view class="services">
      <view
        v-for="s in services"
        :key="s.type"
        class="svc"
        :class="[`tone-${s.tone}`, { 'svc-soon': s.soon }]"
        @tap="onServiceTap(s)"
      >
        <view class="svc-top">
          <view class="svc-icon">
            <!-- 垃圾桶 -->
            <view v-if="s.type === 'garbage'" class="ic-trash">
              <view class="lid" />
              <view class="body">
                <view class="rib" /><view class="rib" />
              </view>
            </view>
            <!-- 爪印 -->
            <view v-else-if="s.type === 'dogwalk'" class="ic-paw">
              <view class="pad" />
              <view class="toe t1" /><view class="toe t2" /><view class="toe t3" /><view class="toe t4" />
            </view>
            <!-- 急救箱 -->
            <view v-else-if="s.type === 'accompany'" class="ic-aid">
              <view class="aid-cross" />
            </view>
            <!-- 书包（接送小孩） -->
            <view v-else-if="s.type === 'kidride'" class="ic-bag">
              <view class="bag-handle" />
              <view class="bag-body">
                <view class="bag-pocket" />
              </view>
            </view>
            <!-- 快递箱 -->
            <view v-else-if="s.type === 'express'" class="ic-box">
              <view class="tape-h" /><view class="tape-v" />
            </view>
          </view>
          <view class="svc-tag">{{ s.tag }}</view>
        </view>
        <view class="svc-name">{{ s.name }}</view>
        <view class="svc-desc">{{ s.desc }}</view>
        <!-- 未上线服务不显示底部动作行：角标已表达「敬请期待」，重复一遍显得啰嗦 -->
        <view v-if="!s.soon" class="svc-arrow">立即预约 ›</view>
      </view>
    </view>

    <!-- 提示条 -->
    <view class="notice">
      <view class="notice-dot" />
      <text class="notice-text">遛宠需牵引 · 猫咪按习性自愿外出</text>
    </view>

    <!-- 预约很简单 -->
    <view class="block-head">
      <text class="block-title">预约很简单</text>
      <view class="block-line" />
    </view>
    <view class="steps">
      <template v-for="(s, i) in steps" :key="s.label">
        <view class="step">
          <view class="step-no">{{ s.no }}</view>
          <view class="step-body">
            <view class="step-label">{{ s.label }}</view>
            <view class="step-desc">{{ s.desc }}</view>
          </view>
        </view>
        <view v-if="i < steps.length - 1" class="step-line" />
      </template>
    </view>

    <!-- 可预约时段 -->
    <view class="block-head">
      <text class="block-title">可预约时段</text>
      <view class="block-line" />
      <text class="block-more" @tap="goOrders">我的订单 ›</text>
    </view>
    <view class="slots">
      <view v-for="s in SLOTS" :key="s.code" class="slot">
        <view class="slot-clock" />
        <text class="slot-text">{{ s.label }}</text>
      </view>
    </view>

    <view class="foot-note">三档时段 · 每个时段限量接单，约满自动置灰</view>

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
        <view class="drawer-foot">邻有回应 · v{{ APP_VERSION }}</view>
      </view>
    </view>
  </view>
</template>

<style scoped>
.page {
  background: var(--bg);
  padding-bottom: 160rpx;
}

/* ---------- 顶部导航 ---------- */
.nav {
  background: transparent;
  position: relative;
  z-index: 3;
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

.guest-tag {
  margin-left: 12rpx;
  padding: 2rpx 12rpx;
  border-radius: 20rpx;
  background: rgba(255, 255, 255, 0.28);
  font-size: 20rpx;
  font-weight: 400;
}

.guest-banner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16rpx;
  margin: 0 28rpx 8rpx;
  padding: 20rpx 24rpx;
  border-radius: 16rpx;
  background: #fef5e6;
}

.guest-banner-text {
  flex: 1;
  font-size: 23rpx;
  color: #a9762a;
  line-height: 1.5;
}

.guest-banner-btn {
  flex-shrink: 0;
  font-size: 24rpx;
  font-weight: 600;
  color: #a9762a;
}

.nav-title {
  font-size: 34rpx;
  font-weight: 700;
  color: var(--brand-dark);
  letter-spacing: 2rpx;
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
  width: 56rpx;
  height: 56rpx;
  border-radius: 50%;
  background: #fff;
  border: 2rpx solid rgba(15, 127, 120, 0.18);
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 2rpx 8rpx rgba(15, 127, 120, 0.08);
}

.avatar-char {
  font-size: 26rpx;
  font-weight: 700;
  color: var(--brand-dark);
}

/* ---------- 头部问候 ---------- */
/* 渐变铺满「导航 + 头部」整块，靠 .page 的背景色在下方接回浅灰。
   不用负 margin 把内容拉到导航下面 —— 那样要靠 z-index 抢层，
   滚动或自定义状态栏高度变化时容易露出空白。 */
.nav,
.hero {
  background: linear-gradient(180deg, #e8f6f3 0%, #f4faf8 100%);
}

.hero {
  padding: 8rpx 36rpx 30rpx;
}

.hero-greet {
  display: flex;
  align-items: baseline;
  gap: 4rpx;
}

.greet-text {
  font-size: 24rpx;
  color: var(--brand-dark);
  font-weight: 500;
  letter-spacing: 1rpx;
}

.greet-name {
  font-size: 24rpx;
  color: var(--text-sub);
  max-width: 300rpx;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.hero-title {
  margin-top: 14rpx;
  font-size: 54rpx;
  font-weight: 800;
  letter-spacing: -1rpx;
  color: #16201d;
  line-height: 1.25;
}

/* 高亮不换字号，只换色 + 底部一条粗下划线，
   避免「交给我们」看起来像另一个标题。
   下划线用绝对定位贴在文字基线下方，不用 z-index:-1 —— 
   -1 会让它沉到父级背景之后，在部分渲染器上表现为穿透文字。 */
.hl {
  color: var(--brand);
  position: relative;
  display: inline-block;
}

.hl::after {
  content: '';
  position: absolute;
  left: -2rpx;
  right: -2rpx;
  bottom: 2rpx;
  height: 12rpx;
  background: rgba(15, 127, 120, 0.16);
  border-radius: 6rpx;
}

.hero-sub {
  margin-top: 14rpx;
  font-size: 24rpx;
  color: var(--text-sub);
  letter-spacing: 0.5rpx;
}

/* ---------- 主视觉卡片 ---------- */
.banner {
  position: relative;
  /* 负 margin 让卡片压在渐变区下沿，形成「浮起」的层次感 */
  margin: -14rpx 24rpx 0;
  height: 268rpx;
  border-radius: 28rpx;
  background: linear-gradient(135deg, #12867e 0%, #3fa89c 55%, #7fc9bd 100%);
  overflow: hidden;
  display: flex;
  align-items: center;
  padding: 0 36rpx;
  box-shadow: 0 14rpx 36rpx rgba(15, 127, 120, 0.22);
}

/* 右上角高光，让纯色渐变有质感 */
.banner-glow {
  position: absolute;
  right: -70rpx;
  top: -90rpx;
  width: 300rpx;
  height: 300rpx;
  border-radius: 50%;
  background: radial-gradient(circle, rgba(255, 255, 255, 0.28) 0%, rgba(255, 255, 255, 0) 70%);
}

.banner-copy {
  position: relative;
  z-index: 2;
}

.banner-badge {
  display: inline-block;
  font-size: 19rpx;
  color: #0c5f59;
  background: rgba(255, 255, 255, 0.88);
  padding: 4rpx 14rpx;
  border-radius: 20rpx;
  letter-spacing: 1rpx;
}

.banner-title {
  margin-top: 14rpx;
  font-size: 42rpx;
  font-weight: 800;
  color: #fff;
  line-height: 1.28;
  letter-spacing: 1rpx;
  text-shadow: 0 2rpx 10rpx rgba(8, 60, 56, 0.18);
}

.banner-sub {
  margin-top: 14rpx;
  font-size: 21rpx;
  color: rgba(255, 255, 255, 0.9);
  letter-spacing: 0.5rpx;
}

/* 用 CSS 拼的房子 + 树，替代 emoji 占位图。
   树用两个叠起来的圆做树冠 + 一根树干，比纯三角形更像树；
   摆在房子左侧且略矮，形成前后层次。 */
.banner-art {
  position: absolute;
  right: 16rpx;
  bottom: 0;
  width: 262rpx;
  height: 226rpx;
  z-index: 1;
}

.art-house {
  position: absolute;
  right: 14rpx;
  bottom: 0;
  width: 136rpx;
  height: 152rpx;
}

.art-roof {
  position: absolute;
  top: 0;
  left: 50%;
  transform: translateX(-50%);
  width: 0;
  height: 0;
  border-left: 76rpx solid transparent;
  border-right: 76rpx solid transparent;
  border-bottom: 58rpx solid #fff;
}

.art-body {
  position: absolute;
  bottom: 0;
  left: 50%;
  transform: translateX(-50%);
  width: 118rpx;
  height: 100rpx;
  background: #fff;
  border-radius: 5rpx;
}

.art-door {
  position: absolute;
  bottom: 0;
  left: 24rpx;
  width: 34rpx;
  height: 54rpx;
  background: #35a094;
  border-radius: 8rpx 8rpx 0 0;
}

.art-window {
  position: absolute;
  top: 24rpx;
  right: 26rpx;
  width: 30rpx;
  height: 30rpx;
  background: #7fc9bd;
  border-radius: 4rpx;
}

/* 树：树干 + 两个圆树冠。整体比房子矮一截，且离左边缘留出余量，
   否则树冠会贴到卡片圆角上被裁掉 */
.art-tree {
  position: absolute;
  left: 34rpx;
  bottom: 0;
  width: 62rpx;
  height: 132rpx;
}

.art-tree::before {
  content: '';
  position: absolute;
  left: 50%;
  bottom: 0;
  transform: translateX(-50%);
  width: 10rpx;
  height: 38rpx;
  background: rgba(255, 255, 255, 0.5);
  border-radius: 3rpx;
}

.art-tree::after {
  content: '';
  position: absolute;
  left: 50%;
  bottom: 28rpx;
  transform: translateX(-50%);
  width: 58rpx;
  height: 58rpx;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.48);
  /* 上方再叠一个小圆，形成圆润的树冠轮廓 */
  box-shadow: 0 -26rpx 0 -10rpx rgba(255, 255, 255, 0.48);
}

/* ---------- 服务入口 ---------- */
.services {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 14rpx;
  padding: 0 24rpx;
  margin-top: 22rpx;
}

.svc {
  background: #fff;
  border-radius: 22rpx;
  padding: 22rpx 16rpx 20rpx;
  min-width: 0;
  box-shadow: 0 4rpx 16rpx rgba(24, 48, 44, 0.05);
  transition: transform 0.16s ease;
}

.svc:active {
  transform: scale(0.97);
}

.svc-top {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  margin-bottom: 14rpx;
}

.svc-icon {
  width: 76rpx;
  height: 76rpx;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
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

/* 小标签，比描述行更能一眼区分计费方式 */
.svc-tag {
  font-size: 17rpx;
  padding: 3rpx 10rpx;
  border-radius: 16rpx;
  white-space: nowrap;
}

.tone-green .svc-tag {
  color: #0b6b66;
  background: #e4f3f0;
}

.tone-amber .svc-tag {
  color: #a8600d;
  background: #fdf0dd;
}

.tone-blue .svc-tag {
  color: #2c5c94;
  background: #e6effa;
}

.tone-purple .svc-tag {
  color: #6b4fa0;
  background: #efe9f8;
}

.tone-cyan .svc-tag {
  color: #0d6e8c;
  background: #e2f2f6;
}

/* 未上线服务（敬请期待）：整卡降不透明度置灰，角标盖一层灰色。
   选择器多一层 .svc 保证优先级压过上面的 tone 配色 */
.svc.svc-soon {
  opacity: 0.66;
}

.svc.svc-soon .svc-tag {
  color: #5c6662;
  background: #e2e7e5;
}

.svc-name {
  font-size: 27rpx;
  font-weight: 700;
  color: var(--text);
}

.svc-desc {
  margin-top: 6rpx;
  font-size: 19rpx;
  color: var(--text-light);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.svc-arrow {
  margin-top: 14rpx;
  font-size: 20rpx;
  font-weight: 600;
}

.tone-green .svc-arrow {
  color: #0b6b66;
}

.tone-amber .svc-arrow {
  color: #b45309;
}

.tone-blue .svc-arrow {
  color: #2c5c94;
}

/* ---- 垃圾桶图标 ---- */
.ic-trash {
  width: 40rpx;
  height: 42rpx;
  position: relative;
  color: #0b6b66;
}

.ic-trash .lid {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 6rpx;
  background: currentColor;
  border-radius: 3rpx;
}

.ic-trash .body {
  position: absolute;
  top: 10rpx;
  left: 5rpx;
  right: 5rpx;
  bottom: 0;
  border: 3rpx solid currentColor;
  border-top: none;
  border-radius: 0 0 8rpx 8rpx;
  display: flex;
  justify-content: center;
  gap: 6rpx;
  padding-top: 8rpx;
}

.ic-trash .rib {
  width: 3rpx;
  height: 18rpx;
  background: currentColor;
  border-radius: 2rpx;
}

/* ---- 爪印图标 ---- */
.ic-paw {
  width: 44rpx;
  height: 42rpx;
  position: relative;
  color: #a8600d;
}

.ic-paw .pad {
  position: absolute;
  bottom: 0;
  left: 50%;
  transform: translateX(-50%);
  width: 26rpx;
  height: 22rpx;
  background: currentColor;
  border-radius: 50% 50% 46% 46%;
}

.ic-paw .toe {
  position: absolute;
  width: 11rpx;
  height: 13rpx;
  background: currentColor;
  border-radius: 50%;
}

.ic-paw .t1 { left: 0;    top: 8rpx; }
.ic-paw .t2 { left: 11rpx; top: 0; }
.ic-paw .t3 { right: 11rpx; top: 0; }
.ic-paw .t4 { right: 0;   top: 8rpx; }

/* ---- 急救箱图标 ---- */
.ic-aid {
  width: 42rpx;
  height: 38rpx;
  border: 3rpx solid #2c5c94;
  border-radius: 8rpx;
  position: relative;
  box-sizing: border-box;
}

.ic-aid::before {
  content: '';
  position: absolute;
  top: -10rpx;
  left: 50%;
  transform: translateX(-50%);
  width: 18rpx;
  height: 8rpx;
  border: 3rpx solid #2c5c94;
  border-bottom: none;
  border-radius: 6rpx 6rpx 0 0;
  box-sizing: border-box;
}

.aid-cross {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  width: 18rpx;
  height: 18rpx;
}

.aid-cross::before,
.aid-cross::after {
  content: '';
  position: absolute;
  background: #2c5c94;
  border-radius: 2rpx;
}

.aid-cross::before {
  left: 50%;
  top: 0;
  transform: translateX(-50%);
  width: 5rpx;
  height: 100%;
}

.aid-cross::after {
  top: 50%;
  left: 0;
  transform: translateY(-50%);
  height: 5rpx;
  width: 100%;
}

/* 书包（接送小孩）：提手 + 包身 + 前袋，统一描边几何风 */
.ic-bag {
  width: 42rpx;
  height: 46rpx;
  position: relative;
}

.bag-handle {
  width: 16rpx;
  height: 10rpx;
  margin: 0 auto;
  border: 3rpx solid #6b4fa0;
  border-bottom: none;
  border-radius: 6rpx 6rpx 0 0;
  box-sizing: border-box;
}

.bag-body {
  width: 42rpx;
  height: 34rpx;
  border: 3rpx solid #6b4fa0;
  border-radius: 8rpx;
  box-sizing: border-box;
  display: flex;
  align-items: flex-end;
  justify-content: center;
  padding-bottom: 6rpx;
}

.bag-pocket {
  width: 22rpx;
  height: 6rpx;
  background: #6b4fa0;
  border-radius: 3rpx;
}

/* 快递箱（代取快递）：箱体 + 箱盖横线 + 中缝竖线 */
.ic-box {
  width: 44rpx;
  height: 38rpx;
  border: 3rpx solid #0d6e8c;
  border-radius: 6rpx;
  position: relative;
  box-sizing: border-box;
}

.ic-box .tape-h {
  position: absolute;
  left: 0;
  right: 0;
  top: 9rpx;
  height: 3rpx;
  background: #0d6e8c;
}

.ic-box .tape-v {
  position: absolute;
  top: 0;
  bottom: 0;
  left: 50%;
  width: 3rpx;
  margin-left: -2rpx;
  background: #0d6e8c;
}

/* ---------- 提示条 ---------- */
.notice {
  display: flex;
  align-items: center;
  gap: 12rpx;
  margin: 18rpx 24rpx 0;
  padding: 16rpx 22rpx;
  background: var(--accent-soft);
  border-radius: 14rpx;
}

.notice-dot {
  width: 10rpx;
  height: 10rpx;
  border-radius: 50%;
  background: #e8a33d;
  flex-shrink: 0;
}

.notice-text {
  font-size: 22rpx;
  color: #b45309;
}

/* ---------- 区块标题 ---------- */
.block-head {
  display: flex;
  align-items: center;
  gap: 14rpx;
  padding: 0 24rpx;
  margin: 34rpx 0 16rpx;
}

.block-title {
  font-size: 30rpx;
  font-weight: 800;
  color: var(--text);
  letter-spacing: 0.5rpx;
}

.block-line {
  flex: 1;
  height: 1rpx;
  background: linear-gradient(90deg, #dde5e2 0%, rgba(221, 229, 226, 0) 100%);
}

.block-more {
  font-size: 21rpx;
  color: var(--brand-dark);
  flex-shrink: 0;
}

/* ---------- 三步 ---------- */
.steps {
  display: flex;
  align-items: stretch;
  padding: 0 24rpx;
}

.step {
  flex: 1;
  background: #fff;
  border-radius: 18rpx;
  padding: 20rpx 16rpx;
  display: flex;
  align-items: center;
  gap: 12rpx;
  min-width: 0;
  box-shadow: 0 4rpx 14rpx rgba(24, 48, 44, 0.04);
}

.step-no {
  width: 38rpx;
  height: 38rpx;
  border-radius: 50%;
  background: var(--brand-tint);
  color: var(--brand);
  font-size: 21rpx;
  font-weight: 800;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.step-body {
  min-width: 0;
}

.step-label {
  font-size: 24rpx;
  font-weight: 600;
  color: var(--text);
  white-space: nowrap;
}

.step-desc {
  font-size: 18rpx;
  color: var(--text-light);
  margin-top: 2rpx;
  white-space: nowrap;
}

.step-line {
  width: 20rpx;
  flex-shrink: 0;
  position: relative;
}

.step-line::after {
  content: '';
  position: absolute;
  top: 50%;
  left: 6rpx;
  right: 6rpx;
  height: 1rpx;
  background: #d5dfdc;
}

/* ---------- 时段 ---------- */
.slots {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 14rpx;
  padding: 0 24rpx;
}

.slot {
  background: #fff;
  border: 1rpx solid #f0e3cc;
  border-radius: 16rpx;
  padding: 18rpx 0;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8rpx;
}

.slot-clock {
  width: 22rpx;
  height: 22rpx;
  border: 2.5rpx solid #d99a2b;
  border-radius: 50%;
  position: relative;
  flex-shrink: 0;
  box-sizing: border-box;
}

/* 指针画成 L 形（竖的是分针、横的是时针），比一根竖线更像时钟 */
.slot-clock::after {
  content: '';
  position: absolute;
  top: 3rpx;
  left: 50%;
  width: 2.5rpx;
  height: 7rpx;
  background: #d99a2b;
  transform: translateX(-50%);
  border-radius: 1rpx;
}

.slot-clock::before {
  content: '';
  position: absolute;
  top: 50%;
  left: 50%;
  width: 6rpx;
  height: 2.5rpx;
  background: #d99a2b;
  transform: translateY(-50%);
  border-radius: 1rpx;
}

.slot-text {
  font-size: 23rpx;
  color: #b45309;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.foot-note {
  text-align: center;
  font-size: 20rpx;
  color: var(--text-light);
  padding: 22rpx 24rpx 8rpx;
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
