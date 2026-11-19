<script setup lang="ts">
/**
 * 自定义底部导航，按「视角」分流而不是按权限分流：
 * 住户视角：首页 / 订单 / 我的；服务方视角：任务 / 接单池 / 我的。
 *
 * 身份切换入口不在这里 —— 放在「我的」页用户名后面。
 * 底部导航只承担导航职责，塞进业务操作会让三项等分结构变复杂，
 * 也容易让用户误以为那是第四个页面。
 *
 * 图标用 CSS 画（房子、清单、人像），不引入图片资源：
 * 小程序不支持 <svg> 标签，图标字体又要多带字体文件，
 * 三个线性图标用 CSS 拼最划算，且能随选中态在「线框 / 实心」间切换。
 */
import { computed } from 'vue'
import { useUserStore } from '@/store/user'

const props = defineProps<{ current: string }>()
const user = useUserStore()

const items = computed(() => {
  if (user.isProviderView) {
    return [
      { key: 'ptasks', label: '任务', path: '/pages/provider/tasks', icon: 'list' },
      { key: 'pool', label: '接单池', path: '/pages/provider/pool', icon: 'house' },
      { key: 'mine', label: '我的', path: '/pages/mine/index', icon: 'user' },
    ]
  }
  return [
    { key: 'index', label: '首页', path: '/pages/index/index', icon: 'house' },
    { key: 'orders', label: '订单', path: '/pages/order/list', icon: 'list' },
    { key: 'mine', label: '我的', path: '/pages/mine/index', icon: 'user' },
  ]
})

function go(item: { key: string; path: string }) {
  if (item.key === props.current) return
  uni.reLaunch({ url: item.path })
}
</script>

<template>
  <view class="tabbar">
    <view
      v-for="item in items"
      :key="item.key"
      class="tab"
      :class="{ active: item.key === current }"
      @tap="go(item)"
    >
      <!-- 房子 -->
      <view v-if="item.icon === 'house'" class="ico house">
        <view class="roof" />
        <view class="wall" />
      </view>

      <!-- 清单 -->
      <view v-else-if="item.icon === 'list'" class="ico list">
        <view class="line" />
        <view class="line" />
        <view class="line" />
      </view>

      <!-- 人像 -->
      <view v-else class="ico user">
        <view class="head" />
        <view class="shoulder" />
      </view>

      <text class="label">{{ item.label }}</text>
    </view>
  </view>
</template>

<style scoped>
.tabbar {
  position: fixed;
  left: 0;
  right: 0;
  bottom: 0;
  height: 108rpx;
  padding-bottom: constant(safe-area-inset-bottom);
  padding-bottom: env(safe-area-inset-bottom);
  background: #fff;
  border-top: 1rpx solid var(--border);
  display: flex;
  align-items: center;
  z-index: 100;
}

.tab {
  flex: 1;
  height: 108rpx;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8rpx;
  color: #a8b0b8;
}

.tab.active {
  color: var(--brand);
}

.label {
  font-size: 22rpx;
  color: inherit;
}

.tab.active .label {
  font-weight: 600;
}

/* ---- 房子 ---- */
.ico.house {
  width: 42rpx;
  height: 40rpx;
  position: relative;
}

.house .roof {
  position: absolute;
  top: 0;
  left: 50%;
  transform: translateX(-50%);
  width: 0;
  height: 0;
  border-left: 21rpx solid transparent;
  border-right: 21rpx solid transparent;
  border-bottom: 17rpx solid currentColor;
}

.house .wall {
  position: absolute;
  bottom: 0;
  left: 50%;
  transform: translateX(-50%);
  width: 28rpx;
  height: 22rpx;
  border: 3rpx solid currentColor;
  border-top: none;
  border-radius: 0 0 4rpx 4rpx;
  box-sizing: border-box;
}

.tab.active .house .wall {
  background: currentColor;
}

/* ---- 清单 ---- */
.ico.list {
  width: 38rpx;
  height: 40rpx;
  border: 3rpx solid currentColor;
  border-radius: 7rpx;
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 6rpx;
}

.list .line {
  width: 20rpx;
  height: 3rpx;
  background: currentColor;
  border-radius: 2rpx;
}

.tab.active .ico.list {
  background: currentColor;
}

.tab.active .list .line {
  background: #fff;
}

/* ---- 人像 ---- */
.ico.user {
  width: 42rpx;
  height: 40rpx;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 4rpx;
}

.user .head {
  width: 17rpx;
  height: 17rpx;
  border: 3rpx solid currentColor;
  border-radius: 50%;
  box-sizing: border-box;
}

.user .shoulder {
  width: 32rpx;
  height: 17rpx;
  border: 3rpx solid currentColor;
  border-bottom: none;
  border-radius: 18rpx 18rpx 0 0;
  box-sizing: border-box;
}

.tab.active .user .head {
  background: currentColor;
}

.tab.active .user .shoulder {
  background: currentColor;
}
</style>
