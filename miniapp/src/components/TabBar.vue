<script setup lang="ts">
/**
 * 自定义底部导航。
 * 不用原生 tabBar：原生 tabBar 必须配图片图标，项目暂无设计资源，
 * 且服务方端也要复用这套导航（按角色换 tab），自定义更灵活。
 */
const props = defineProps<{ current: 'index' | 'orders' | 'mine' }>()

const items = [
  { key: 'index' as const, label: '首页', path: '/pages/index/index' },
  { key: 'orders' as const, label: '订单', path: '/pages/order/list' },
  { key: 'mine' as const, label: '我的', path: '/pages/mine/index' },
]

function go(item: (typeof items)[number]) {
  if (item.key === props.current) return
  // reLaunch 清空页面栈，避免来回切 tab 把栈堆满
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
      <view class="mark" />
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
  height: 110rpx;
  padding-bottom: constant(safe-area-inset-bottom);
  padding-bottom: env(safe-area-inset-bottom);
  background: rgba(255, 255, 255, 0.96);
  border-top: 1rpx solid var(--border);
  display: flex;
  align-items: center;
  z-index: 100;
}

.tab {
  flex: 1;
  height: 110rpx;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 6rpx;
}

.mark {
  width: 12rpx;
  height: 12rpx;
  border-radius: 50%;
  background: #d1d5db;
}

.label {
  font-size: 24rpx;
  color: var(--text-sub);
}

.tab.active .mark {
  background: var(--brand);
  width: 32rpx;
  border-radius: 6rpx;
}

.tab.active .label {
  color: var(--brand);
  font-weight: 600;
}
</style>
