<script setup lang="ts">
import { ref } from 'vue'
import { onReachBottom, onShow } from '@dcloudio/uni-app'
import TabBar from '@/components/TabBar.vue'
import { listOrders, type Order } from '@/api'
import { useUserStore } from '@/store/user'
import { friendlyDate, price, SERVICE_LABEL, SLOTS } from '@/utils/format'

const user = useUserStore()

const tabs = [
  { key: '', label: '全部' },
  { key: 'pending', label: '待服务' },
  { key: 'processing', label: '服务中' },
  { key: 'completed', label: '已完成' },
]

const activeTab = ref('')
const orders = ref<Order[]>([])
const loading = ref(false)
const finished = ref(false)
const page = ref(1)
const total = ref(0)

onShow(() => {
  reload()
})

onReachBottom(() => {
  if (!finished.value && !loading.value) loadMore()
})

async function reload() {
  page.value = 1
  finished.value = false
  orders.value = []
  await fetchPage()
}

async function loadMore() {
  page.value += 1
  await fetchPage()
}

async function fetchPage() {
  if (loading.value) return
  loading.value = true
  try {
    await user.ensureLogin()
    const res = await listOrders(activeTab.value, page.value, 10)
    orders.value = page.value === 1 ? res.list : [...orders.value, ...res.list]
    total.value = res.total
    if (orders.value.length >= res.total) finished.value = true
  } catch (e) {
    uni.showToast({ title: '加载失败，请下拉重试', icon: 'none' })
  } finally {
    loading.value = false
  }
}

function switchTab(key: string) {
  if (activeTab.value === key) return
  activeTab.value = key
  reload()
}

function goDetail(orderNo: string) {
  uni.navigateTo({ url: `/pages/order/detail?orderNo=${orderNo}` })
}

function slotLabel(code: string): string {
  return SLOTS.find((s) => s.code === code)?.label || code
}

/** 列表上的主操作，按状态给不同引导 */
function primaryAction(o: Order): { text: string; type: string } | null {
  if (o.status === 'pending_payment') return { text: '去支付', type: 'pay' }
  if (o.status === 'pending_confirm') return { text: '确认完成', type: 'confirm' }
  if (o.status === 'completed') return { text: '查看记录', type: 'view' }
  return null
}

function onAction(o: Order) {
  if (o.status === 'pending_payment' || o.status === 'pending_confirm') {
    goDetail(o.order_no)
    return
  }
  goDetail(o.order_no)
}

/** 状态对应的颜色 */
function statusTone(status: string): string {
  if (status === 'completed') return 'done'
  if (status === 'canceled' || status === 'rejected') return 'closed'
  if (status === 'pending_payment') return 'warn'
  return 'active'
}
</script>

<template>
  <view class="page">
    <view class="tabs">
      <view
        v-for="t in tabs"
        :key="t.key"
        class="tab"
        :class="{ active: activeTab === t.key }"
        @tap="switchTab(t.key)"
      >
        {{ t.label }}
      </view>
    </view>

    <view v-if="!loading && orders.length === 0" class="empty">
      还没有订单，回首页挑一项服务吧
    </view>

    <view
      v-for="o in orders"
      :key="o.order_no"
      class="order-card"
      @tap="goDetail(o.order_no)"
    >
      <view class="card-head">
        <view class="head-left">
          <text class="service">{{ SERVICE_LABEL[o.service_type] || o.service_type }}</text>
          <text v-if="o.plan === 'monthly'" class="badge">包月</text>
        </view>
        <text class="status" :class="statusTone(o.status)">{{ o.status_label }}</text>
      </view>

      <view class="meta">
        <text class="meta-item">{{ friendlyDate(o.service_date) }}</text>
        <text class="dot">·</text>
        <text class="meta-item">{{ slotLabel(o.slot_code) }}</text>
      </view>

      <view v-if="o.address" class="addr">
        {{ o.address.community }} {{ o.address.lane }} {{ o.address.house_no }}
      </view>

      <view class="card-foot">
        <view class="amount">
          <text class="amount-num">{{ price(o.total_amount) }}</text>
          <text v-if="o.paid_amount > 0" class="paid">已付</text>
        </view>
        <view
          v-if="primaryAction(o)"
          class="action"
          :class="primaryAction(o)!.type"
          @tap.stop="onAction(o)"
        >
          {{ primaryAction(o)!.text }}
        </view>
      </view>
    </view>

    <view v-if="loading" class="loading">加载中…</view>
    <view v-else-if="finished && orders.length > 0" class="loading">没有更多了</view>

    <TabBar current="orders" />
  </view>
</template>

<style scoped>
.page {
  padding-top: 24rpx;
}

.tabs {
  display: flex;
  background: #fff;
  padding: 0 12rpx;
  margin-bottom: 20rpx;
  border-radius: 20rpx;
  margin-left: 24rpx;
  margin-right: 24rpx;
}

.tab {
  flex: 1;
  text-align: center;
  padding: 24rpx 0;
  font-size: 27rpx;
  color: var(--text-sub);
  position: relative;
}

.tab.active {
  color: var(--brand-dark);
  font-weight: 600;
}

.tab.active::after {
  content: '';
  position: absolute;
  left: 50%;
  bottom: 8rpx;
  transform: translateX(-50%);
  width: 40rpx;
  height: 6rpx;
  border-radius: 3rpx;
  background: var(--brand);
}

.order-card {
  background: #fff;
  border-radius: 24rpx;
  padding: 28rpx;
  margin: 0 24rpx 20rpx;
}

.card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.head-left {
  display: flex;
  align-items: center;
  gap: 12rpx;
}

.service {
  font-size: 30rpx;
  font-weight: 600;
}

.badge {
  font-size: 20rpx;
  color: #b45309;
  background: #fef3c7;
  padding: 4rpx 12rpx;
  border-radius: 8rpx;
}

.status {
  font-size: 25rpx;
  font-weight: 500;
}

.status.warn {
  color: #d97706;
}

.status.active {
  color: var(--brand-dark);
}

.status.done {
  color: var(--text-sub);
}

.status.closed {
  color: var(--text-light);
}

.meta {
  margin-top: 16rpx;
  font-size: 25rpx;
  color: var(--text-sub);
  display: flex;
  align-items: center;
  gap: 8rpx;
}

.dot {
  color: var(--text-light);
}

.addr {
  margin-top: 8rpx;
  font-size: 24rpx;
  color: var(--text-light);
}

.card-foot {
  margin-top: 20rpx;
  padding-top: 20rpx;
  border-top: 1rpx solid #f3f4f6;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.amount {
  display: flex;
  align-items: baseline;
  gap: 10rpx;
}

.amount-num {
  font-size: 32rpx;
  font-weight: 700;
}

.paid {
  font-size: 22rpx;
  color: var(--text-light);
}

.action {
  padding: 12rpx 32rpx;
  border-radius: 30rpx;
  font-size: 25rpx;
}

.action.pay {
  background: var(--brand);
  color: #fff;
}

.action.confirm {
  background: #fef3c7;
  color: #b45309;
}

.action.view {
  border: 1rpx solid var(--border);
  color: var(--text-sub);
}

.loading {
  text-align: center;
  color: var(--text-light);
  font-size: 24rpx;
  padding: 30rpx 0;
}
</style>
