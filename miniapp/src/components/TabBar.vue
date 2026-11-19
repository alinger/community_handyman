<script setup lang="ts">
/**
 * 自定义底部导航，按「视角」分流而不是按权限分流：
 * 住户视角：首页 / 订单 / 我的；服务方视角：任务 / 接单池 / 我的。
 *
 * 服务方本身也是住户（也要给自己下单），所以已认证用户在这里多一个
 * 中间的「切换」按钮，切的是 viewMode（看哪一侧），不动 role（有没有权限）。
 * 未认证用户中间不渲染该按钮，保持三项等分。
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

/** 已认证用户才能看到视角切换 */
const canSwitch = computed(() => user.isProvider)

function go(item: { key: string; path: string }) {
  if (item.key === props.current) return
  uni.reLaunch({ url: item.path })
}

function onSwitch() {
  user.toggleView()
}
</script>

<template>
  <view class="tabbar">
    <!-- 前两项 -->
    <view
      v-for="item in items.slice(0, 2)"
      :key="item.key"
      class="tab"
      :class="{ active: item.key === current }"
      @tap="go(item)"
    >
      <view v-if="item.icon === 'house'" class="ico house">
        <view class="roof" />
        <view class="wall" />
      </view>
      <view v-else class="ico list">
        <view class="line" />
        <view class="line" />
        <view class="line" />
      </view>
      <text class="label">{{ item.label }}</text>
    </view>

    <!-- 中间：身份切换（仅已认证服务方可见） -->
    <view v-if="canSwitch" class="tab switch-tab" @tap="onSwitch">
      <view class="switch-btn">
        <view class="swap">
          <view class="arrow up" />
          <view class="arrow down" />
        </view>
      </view>
      <text class="label switch-label">{{ user.isProviderView ? '切到住户' : '切到服务方' }}</text>
    </view>

    <!-- 我的 -->
    <view
      class="tab"
      :class="{ active: items[2].key === current }"
      @tap="go(items[2])"
    >
      <view class="ico user">
        <view class="head" />
        <view class="shoulder" />
      </view>
      <text class="label">{{ items[2].label }}</text>
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

/* ---- 身份切换：中间凸起圆形按钮 ---- */
.switch-tab {
  flex: 0 0 150rpx;
  position: relative;
  height: 108rpx;
}

.switch-btn {
  position: absolute;
  left: 50%;
  top: -26rpx;
  transform: translateX(-50%);
  width: 84rpx;
  height: 84rpx;
  border-radius: 50%;
  background: var(--brand);
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 6rpx 18rpx rgba(15, 127, 120, 0.32);
  border: 4rpx solid #fff;
}

.switch-btn:active {
  transform: translateX(-50%) scale(0.94);
}

.swap {
  display: flex;
  flex-direction: column;
  gap: 5rpx;
}

.arrow {
  width: 26rpx;
  height: 26rpx;
  border-right: 4rpx solid #fff;
  border-bottom: 4rpx solid #fff;
  box-sizing: border-box;
}

.arrow.up {
  transform: rotate(-135deg);
  margin-bottom: -6rpx;
}

.arrow.down {
  transform: rotate(45deg);
  margin-top: -6rpx;
}

.switch-label {
  /* 与两侧 tab 的 label 同高：两侧是「图标 40rpx + gap 8rpx + 文字」垂直居中，
     这里直接绝对贴底，保证基线一致 */
  position: absolute;
  bottom: 14rpx;
  left: 0;
  right: 0;
  text-align: center;
  color: var(--brand-dark);
  font-size: 20rpx;
  font-weight: 600;
  line-height: 1.2;
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
