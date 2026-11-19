<script setup lang="ts">
/**
 * 自定义底部导航，按角色分流：
 * 住户：首页 / 订单 / 我的
 * 服务方：任务 / 接单池 / 我的
 * 不用原生 tabBar：一是要配图片图标（暂无设计资源），
 * 二是同一工程要承载两种角色，原生 tab 配置是全局唯一的。
 */
import { computed } from 'vue'
import { useUserStore } from '@/store/user'

const props = defineProps<{ current: string }>()
const user = useUserStore()

const items = computed(() => {
  if (user.role === 'provider') {
    return [
      { key: 'ptasks', label: '任务', path: '/pages/provider/tasks' },
      { key: 'pool', label: '接单池', path: '/pages/provider/pool' },
      { key: 'mine', label: '我的', path: '/pages/mine/index' },
    ]
  }
  return [
    { key: 'index', label: '首页', path: '/pages/index/index' },
    { key: 'orders', label: '订单', path: '/pages/order/list' },
    { key: 'mine', label: '我的', path: '/pages/mine/index' },
  ]
})

function go(item: { key: string; label: string; path: string }) {
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
