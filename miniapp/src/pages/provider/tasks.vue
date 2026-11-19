<script setup lang="ts">
import { ref } from 'vue'
import { onReachBottom, onShow } from '@dcloudio/uni-app'
import TabBar from '@/components/TabBar.vue'
import {
  acceptOrder,
  advanceOrder,
  providerMyOrders,
  providerOpenOrders,
  type Order,
} from '@/api'
import { useUserStore } from '@/store/user'
import { friendlyDate, price, SERVICE_LABEL, SLOTS } from '@/utils/format'
import { ApiError } from '@/utils/request'

const user = useUserStore()

const tabs = [
  { key: 'pending', label: '待服务' },
  { key: 'processing', label: '服务中' },
  { key: 'completed', label: '已完成' },
]
const activeTab = ref('pending')

const orders = ref<Order[]>([])
const openCount = ref(0)
const loading = ref(false)
const finished = ref(false)
const page = ref(1)
const acting = ref(false)

onShow(async () => {
  await user.ensureLogin()
  // 服务方身份校验：住户误入时弹提示并跳回
  if (user.role !== 'provider') {
    uni.showModal({
      title: '身份不符',
      content: '当前是住户账号，请先在「我的」里切换为服务方',
      showCancel: false,
      success: () => uni.reLaunch({ url: '/pages/index/index' }),
    })
    return
  }
  await Promise.all([reload(), loadOpenCount()])
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
    const res = await providerMyOrders(activeTab.value, page.value, 10)
    orders.value = page.value === 1 ? res.list : [...orders.value, ...res.list]
    if (orders.value.length >= res.total) finished.value = true
  } catch (e) {
    uni.showToast({ title: '加载失败', icon: 'none' })
  } finally {
    loading.value = false
  }
}

async function loadOpenCount() {
  try {
    const res = await providerOpenOrders(1, 1)
    openCount.value = res.total
  } catch {
    openCount.value = 0
  }
}

function switchTab(key: string) {
  if (activeTab.value === key) return
  activeTab.value = key
  reload()
}

function goPool() {
  uni.navigateTo({ url: '/pages/provider/pool' })
}

function goDetail(orderNo: string) {
  uni.navigateTo({ url: `/pages/order/detail?orderNo=${orderNo}` })
}

function slotLabel(code: string): string {
  return SLOTS.find((s) => s.code === code)?.label || code
}

/**
 * 按当前状态给出合法的推进动作。
 * 后端 CanTransit 是白名单，这里只展示界面入口，
 * 真正的合法性仍由后端兜底（防重复点击、非法跳跃都返回 2003）。
 */
function actions(o: Order): { node: string; label: string; primary: boolean }[] {
  const acts: { node: string; label: string; primary: boolean }[] = []
  if (o.status === 'accepted') {
    // 垃圾回收要先到楼下，遛宠直接开始
    if (o.service_type === 'garbage') {
      acts.push({ node: 'on_the_way', label: '出发上门', primary: false })
    }
    acts.push({ node: 'in_service', label: '开始服务', primary: true })
  } else if (o.status === 'on_the_way') {
    acts.push({ node: 'in_service', label: '开始服务', primary: true })
  } else if (o.status === 'in_service') {
    acts.push({ node: 'pending_confirm', label: '完成服务', primary: true })
  }
  return acts
}

async function onAdvance(o: Order, node: string) {
  if (acting.value) return
  acting.value = true
  try {
    await advanceOrder(o.order_no, node)
    uni.showToast({ title: '已更新', icon: 'success' })
    await reload()
  } catch (e) {
    uni.showModal({
      title: '操作失败',
      content: e instanceof ApiError ? e.message : '请稍后重试',
      showCancel: false,
    })
  } finally {
    acting.value = false
  }
}
</script>

<template>
  <view class="page">
    <view class="pool-entry" @tap="goPool">
      <view class="pool-text">
        <text class="pool-num">{{ openCount }}</text>
        <text class="pool-label">个新订单待接</text>
      </view>
      <text class="pool-go">去接单 ›</text>
    </view>

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
      暂无任务，去接单池看看
    </view>

    <view v-for="o in orders" :key="o.order_no" class="task-card" @tap="goDetail(o.order_no)">
      <view class="head">
        <view class="head-left">
          <text class="service">{{ SERVICE_LABEL[o.service_type] }}</text>
          <text v-if="o.plan === 'monthly'" class="badge">包月</text>
        </view>
        <text class="status" :class="o.status">{{ o.status_label }}</text>
      </view>

      <view class="meta">
        {{ friendlyDate(o.service_date) }} · {{ slotLabel(o.slot_code) }}
      </view>
      <view v-if="o.address" class="addr">
        {{ o.address.community }} {{ o.address.lane }} {{ o.address.house_no }}
      </view>
      <view v-if="o.address && (o.address.contact_name || o.address.contact_phone)" class="contact">
        联系人：{{ o.address.contact_name }} {{ o.address.contact_phone }}
      </view>
      <view v-if="o.remark" class="remark">备注：{{ o.remark }}</view>

      <view v-if="actions(o).length" class="actions">
        <view
          v-for="a in actions(o)"
          :key="a.node"
          class="action"
          :class="{ primary: a.primary }"
          @tap.stop="onAdvance(o, a.node)"
        >
          {{ a.label }}
        </view>
      </view>
    </view>

    <view v-if="loading" class="loading">加载中…</view>
    <view v-else-if="finished && orders.length > 0" class="loading">没有更多了</view>

    <TabBar current="ptasks" />
  </view>
</template>

<style scoped>
.page {
  padding-top: 24rpx;
}

.pool-entry {
  margin: 0 24rpx 24rpx;
  padding: 28rpx 32rpx;
  border-radius: 24rpx;
  background: linear-gradient(160deg, #f59e0b, #d97706);
  color: #fff;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.pool-num {
  font-size: 44rpx;
  font-weight: 700;
}

.pool-label {
  font-size: 26rpx;
  margin-left: 10rpx;
  opacity: 0.9;
}

.pool-go {
  font-size: 27rpx;
  font-weight: 500;
}

.tabs {
  display: flex;
  background: #fff;
  border-radius: 20rpx;
  margin: 0 24rpx 20rpx;
  padding: 0 12rpx;
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

.task-card {
  background: #fff;
  border-radius: 24rpx;
  padding: 28rpx;
  margin: 0 24rpx 20rpx;
}

.head {
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
  color: var(--brand-dark);
}

.status.in_service {
  color: #2563eb;
}

.status.pending_confirm {
  color: #d97706;
}

.status.completed {
  color: var(--text-sub);
}

.meta {
  margin-top: 14rpx;
  font-size: 25rpx;
  color: var(--text-sub);
}

.addr {
  margin-top: 8rpx;
  font-size: 25rpx;
  color: var(--text);
}

.contact {
  margin-top: 6rpx;
  font-size: 24rpx;
  color: var(--brand-dark);
}

.remark {
  margin-top: 10rpx;
  font-size: 23rpx;
  color: var(--text-light);
  line-height: 1.6;
  overflow: hidden;
  text-overflow: ellipsis;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
}

.actions {
  margin-top: 20rpx;
  padding-top: 20rpx;
  border-top: 1rpx solid #f3f4f6;
  display: flex;
  gap: 16rpx;
  justify-content: flex-end;
}

.action {
  padding: 14rpx 36rpx;
  border-radius: 32rpx;
  font-size: 26rpx;
  border: 1rpx solid var(--border);
  color: var(--text);
}

.action.primary {
  background: var(--brand);
  color: #fff;
  border-color: var(--brand);
  font-weight: 500;
}

.loading {
  text-align: center;
  color: var(--text-light);
  font-size: 24rpx;
  padding: 30rpx 0;
}
</style>
