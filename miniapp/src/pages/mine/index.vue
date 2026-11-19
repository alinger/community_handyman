<script setup lang="ts">
import { onShow } from '@dcloudio/uni-app'
import TabBar from '@/components/TabBar.vue'
import { useUserStore } from '@/store/user'

const user = useUserStore()

onShow(async () => {
  try {
    await user.ensureLogin()
    await Promise.all([user.loadAddresses(), user.loadProfile()])
  } catch (e) {
    console.warn('[mine] 初始化失败', e)
  }
})

function goAddress() {
  uni.navigateTo({ url: '/pages/address/index' })
}

/** 设置入口放在个人信息区右上角，符合主流 App 的入口习惯 */
function goSettings() {
  uni.navigateTo({ url: '/pages/settings/index' })
}

function goOrders() {
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

/**
 * 开发环境切换演示身份（住户 ↔ 服务方）。
 * 生产环境身份由微信登录决定，这个入口不会渲染。
 */
async function onSwitchRole(role: 'user' | 'provider') {
  if (user.role === role) {
    uni.showToast({ title: '已经是该身份', icon: 'none' })
    return
  }
  try {
    await user.switchRole(role)
  } catch (e) {
    // 切换依赖后端的 /dev/login，该路由只在 dev 注册，
    // 生产环境点到这里会失败，给一句人话而不是抛异常
    uni.showModal({
      title: '无法切换',
      content: '身份切换仅演示环境可用，正式环境由微信登录决定身份',
      showCancel: false,
    })
  }
}
</script>

<template>
  <view class="page">
    <!-- 个人信息：右上角进设置 -->
    <view class="profile">
      <view class="avatar">{{ (user.nickname || '住')[0] }}</view>
      <view class="profile-info">
        <view class="nickname">{{ user.nickname || '住户' }}</view>
        <view class="uid">
          ID {{ user.userId || '-' }}
          <text v-if="user.phone" class="phone">· {{ user.phone }}</text>
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
          <text class="menu-value">v1.0.0</text>
          <text class="chev">›</text>
        </view>
      </view>
    </view>

    <!-- 服务范围说明 -->
    <view class="card range-note">
      <view class="section-title">服务范围</view>
      <view class="muted">
        东方冠郡一期、东方冠郡二期、万科清林径、兴欣花苑、同润欣汇名邸、同润新云都会、
        招商象屿星耀翠湾、中粮首创禧瑞祥云、丝庐雅苑一期、丝庐雅苑二期、中洲华庭、吉宝汇锦城一期。
      </view>
    </view>

    <!-- 演示身份切换：仅开发环境 -->
    <view class="demo-switch" @tap="onSwitchRole(user.role === 'user' ? 'provider' : 'user')">
      切换为{{ user.role === 'user' ? '服务方' : '住户' }}（演示）
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

.nickname {
  font-size: 34rpx;
  font-weight: 600;
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

.range-note {
  line-height: 1.7;
}

.demo-switch {
  margin: 40rpx 32rpx 0;
  text-align: center;
  font-size: 23rpx;
  color: var(--text-light);
  padding: 16rpx 0;
}

.demo-switch:active {
  opacity: 0.6;
}
</style>
