<script setup lang="ts">
import { onShow } from '@dcloudio/uni-app'
import TabBar from '@/components/TabBar.vue'
import { useUserStore } from '@/store/user'

const user = useUserStore()

onShow(async () => {
  try {
    await user.ensureLogin()
    await user.loadAddresses()
  } catch (e) {
    console.warn('[mine] 初始化失败', e)
  }
})

function goAddress() {
  uni.navigateTo({ url: '/pages/address/index' })
}

function goSettings() {
  uni.navigateTo({ url: '/pages/settings/index' })
}

function goOrders() {
  uni.reLaunch({ url: '/pages/order/list' })
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

function callService() {
  uni.showModal({
    title: '联系客服',
    content: '客服电话：021-0000 0000（工作日 9:00-18:00）',
    showCancel: false,
  })
}
</script>

<template>
  <view class="page">
    <view class="profile">
      <view class="avatar">住</view>
      <view class="profile-info">
        <view class="nickname">{{ user.nickname || '住户' }}</view>
        <view class="uid">ID {{ user.userId || '-' }}</view>
      </view>
    </view>

    <view class="card menu">
      <view class="menu-item" @tap="goAddress">
        <text>常用地址</text>
        <view class="menu-right">
          <text class="menu-value">{{ user.currentAddress?.community || '未添加' }}</text>
          <text class="chev">›</text>
        </view>
      </view>
      <view class="menu-item" @tap="goSettings">
        <text>设置</text>
        <text class="chev">›</text>
      </view>
      <view class="menu-item" @tap="goOrders">
        <text>我的订单</text>
        <text class="chev">›</text>
      </view>
      <view class="menu-item" @tap="callService">
        <text>联系客服</text>
        <text class="chev">›</text>
      </view>
      <view class="menu-item" @tap="onSwitchRole(user.role === 'user' ? 'provider' : 'user')">
        <text>切换为{{ user.role === 'user' ? '服务方' : '住户' }}（演示）</text>
        <text class="chev">›</text>
      </view>
    </view>

    <view class="card range-note">
      <view class="section-title">服务范围</view>
      <view class="muted">
        东方冠郡一期、东方冠郡二期、万科清林径、兴欣花苑、同润欣汇名邸、同润新云都会、
        招商象屿星耀翠湾、中粮首创禧瑞祥云、丝庐雅苑一期、丝庐雅苑二期、中洲华庭、吉宝汇锦城一期。
      </view>
    </view>

    <TabBar current="mine" />
  </view>
</template>

<style scoped>
.page {
  padding-top: 24rpx;
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
</style>
