<script setup lang="ts">
import { onLaunch } from '@dcloudio/uni-app'
import { useUserStore } from '@/store/user'

onLaunch(async () => {
  const user = useUserStore()
  try {
    await user.ensureLogin()
    await user.loadAddresses()
  } catch (e) {
    // 启动阶段静默失败，进入具体页面时会再次尝试并给出提示
    console.warn('[app] 初始化用户失败', e)
  }
})
</script>

<style>
/* 全局设计变量。uni-app 的 App.vue 样式会作用于所有页面。 */
page {
  --brand: #16a34a;
  --brand-dark: #15803d;
  --brand-light: #dcfce7;
  --text: #1f2937;
  --text-sub: #6b7280;
  --text-light: #9ca3af;
  --bg: #f5f7fa;
  --card: #ffffff;
  --border: #e5e7eb;
  --warn: #f59e0b;
  --danger: #ef4444;
  --info: #3b82f6;

  background-color: var(--bg);
  color: var(--text);
  font-size: 28rpx;
  font-family: -apple-system, BlinkMacSystemFont, 'PingFang SC', 'Helvetica Neue', sans-serif;
}

.page {
  min-height: 100vh;
  padding-bottom: 140rpx; /* 给底部导航留位 */
  box-sizing: border-box;
}

.card {
  background: var(--card);
  border-radius: 24rpx;
  padding: 32rpx;
  margin: 0 24rpx 24rpx;
  box-shadow: 0 2rpx 16rpx rgba(15, 23, 42, 0.04);
}

.section-title {
  font-size: 30rpx;
  font-weight: 600;
  margin-bottom: 20rpx;
  color: var(--text);
}

.muted {
  color: var(--text-sub);
  font-size: 24rpx;
}

.row {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.btn-primary {
  background: var(--brand);
  color: #fff;
  border-radius: 44rpx;
  height: 88rpx;
  line-height: 88rpx;
  text-align: center;
  font-size: 30rpx;
  font-weight: 500;
}

.btn-primary.is-disabled {
  background: #d1d5db;
}

.btn-ghost {
  border: 1rpx solid var(--border);
  color: var(--text);
  border-radius: 44rpx;
  height: 88rpx;
  line-height: 88rpx;
  text-align: center;
  font-size: 30rpx;
  background: #fff;
}

.empty {
  padding: 120rpx 0;
  text-align: center;
  color: var(--text-light);
  font-size: 26rpx;
}
</style>
