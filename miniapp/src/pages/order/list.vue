<script setup lang="ts">
import { ref } from 'vue'
import { onReachBottom, onShow } from '@dcloudio/uni-app'
import TabBar from '@/components/TabBar.vue'
import { listOrders, type Order } from '@/api'
import { useUserStore } from '@/store/user'
import { ensureAuth } from '@/utils/auth'
import { friendlyDate, price, SERVICE_LABEL, SLOTS } from '@/utils/format'

const user = useUserStore()

// 文案与设计稿一致：全部 / 待服务 / 进行中 / 已完成
const tabs = [
  { key: '', label: '全部' },
  { key: 'pending', label: '待服务' },
  { key: 'processing', label: '进行中' },
  { key: 'completed', label: '已完成' },
]

const ICONS: Record<string, string> = { garbage: '🗑', dogwalk: '🐾', accompany: '🏥' }

const activeTab = ref('')
const orders = ref<Order[]>([])
const loading = ref(false)
const finished = ref(false)
const page = ref(1)

onShow(() => {
  if (!ensureAuth()) return
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
    if (orders.value.length >= res.total) finished.value = true
  } catch (e) {
    uni.showToast({ title: '加载失败', icon: 'none' })
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

/** 卡片副标题：小区 + 单次/包月 */
function subLine(o: Order): string {
  const area = o.address ? o.address.community : ''
  const plan = o.plan === 'monthly' ? '包月服务' : '单次服务'
  return [area, plan].filter(Boolean).join(' · ')
}

/** 时间文案：今天/明天优先，其他显示日期 */
function timeLine(o: Order): string {
  const d = friendlyDate(o.service_date)
  return `${d} ${slotLabel(o.slot_code)}`
}

function statusClass(status: string): string {
  // 待支付与待接单都属于「待服务」，用橙色；履约中才是主色绿
  if (status === 'pending_payment' || status === 'pending_accept') return 'warn'
  if (status === 'completed') return 'done'
  if (status === 'canceled' || status === 'rejected') return 'closed'
  return 'active'
}

/** 底部按钮组，按状态给不同引导 */
function actions(o: Order): { label: string; kind: 'ghost' | 'primary'; key: string }[] {
  if (o.status === 'pending_payment') {
    return [
      { label: '取消订单', kind: 'ghost', key: 'cancel' },
      { label: '去支付', kind: 'primary', key: 'pay' },
    ]
  }
  if (o.status === 'completed') {
    return [
      { label: '查看记录', kind: 'ghost', key: 'records' },
      { label: '再次预约', kind: 'primary', key: 'again' },
    ]
  }
  if (o.status === 'canceled' || o.status === 'rejected') {
    return [{ label: '再次预约', kind: 'primary', key: 'again' }]
  }
  return [
    { label: '联系服务方', kind: 'ghost', key: 'contact' },
    { label: '查看进度', kind: 'primary', key: 'detail' },
  ]
}

function onAction(o: Order, key: string) {
  switch (key) {
    case 'again':
      uni.navigateTo({ url: `/pages/order/create?type=${o.service_type}` })
      break
    case 'contact':
      // 服务方联系方式不在订单里，接单后才由平台下发
      uni.showModal({
        title: '联系服务方',
        content: '服务方接单后，平台会通过短信发送联系方式',
        showCancel: false,
      })
      break
    default:
      goDetail(o.order_no)
  }
}
</script>

<template>
  <view class="page">
    <!-- 分段控制器：深青绿底，选中项浅色块 -->
    <view class="seg">
      <view
        v-for="t in tabs"
        :key="t.key"
        class="seg-item"
        :class="{ active: activeTab === t.key }"
        @tap="switchTab(t.key)"
      >
        {{ t.label }}
      </view>
    </view>

    <view v-if="!loading && orders.length === 0" class="empty">
      还没有订单，回首页挑一项服务吧
    </view>

    <view v-for="o in orders" :key="o.order_no" class="order-card">
      <view class="top">
        <view class="thumb" :class="o.service_type">{{ ICONS[o.service_type] || '📦' }}</view>
        <view class="head-main">
          <view class="title-row">
            <text class="title">{{ SERVICE_LABEL[o.service_type] }}</text>
            <text class="status" :class="statusClass(o.status)">{{ o.status_label }}</text>
          </view>
          <view class="sub">{{ subLine(o) }}</view>
          <view class="time">{{ timeLine(o) }}</view>
        </view>
      </view>

      <!-- 进度节点：已到达绿色，未到达灰色 -->
      <view v-if="o.nodes?.length" class="progress">
        <template v-for="(n, i) in o.nodes" :key="n.node">
          <view class="node" :class="{ reached: n.reached }">
            <text class="node-label">{{ n.label }}</text>
          </view>
          <view v-if="i < o.nodes.length - 1" class="node-line" :class="{ reached: n.reached }" />
        </template>
      </view>

      <view class="actions">
        <view
          v-for="a in actions(o)"
          :key="a.key"
          class="act"
          :class="a.kind"
          @tap.stop="onAction(o, a.key)"
        >
          {{ a.label }}
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
  padding-top: 20rpx;
  /* 给固定 TabBar 留位置 */
  padding-bottom: 160rpx;
}

/* ---------- 分段控制器 ---------- */
.seg {
  display: flex;
  margin: 0 24rpx 20rpx;
  padding: 8rpx;
  background: var(--brand);
  border-radius: 14rpx;
}

.seg-item {
  flex: 1;
  text-align: center;
  padding: 18rpx 0;
  font-size: 26rpx;
  color: rgba(255, 255, 255, 0.72);
  border-radius: 10rpx;
}

.seg-item.active {
  background: rgba(255, 255, 255, 0.2);
  color: #fff;
  font-weight: 600;
}

/* ---------- 订单卡 ---------- */
.order-card {
  background: #fff;
  border-radius: 20rpx;
  margin: 0 24rpx 20rpx;
  padding: 26rpx 28rpx 22rpx;
}

.top {
  display: flex;
  gap: 20rpx;
}

.thumb {
  width: 104rpx;
  height: 104rpx;
  border-radius: 22rpx;
  font-size: 54rpx;
  line-height: 104rpx;
  text-align: center;
  flex-shrink: 0;
}

.thumb.garbage {
  background: #e4f3f0;
}

.thumb.dogwalk {
  background: #fdf0dd;
}

.thumb.accompany {
  background: #e6effa;
}

.head-main {
  flex: 1;
  min-width: 0;
}

.title-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12rpx;
}

.title {
  font-size: 31rpx;
  font-weight: 700;
  color: var(--text);
}

.status {
  font-size: 24rpx;
  font-weight: 600;
  flex-shrink: 0;
}

.status.warn {
  color: var(--accent);
}

.status.active {
  color: var(--brand);
}

.status.done {
  color: var(--text-sub);
}

.status.closed {
  color: var(--text-light);
}

.sub {
  margin-top: 10rpx;
  font-size: 23rpx;
  color: var(--text-sub);
}

.time {
  margin-top: 6rpx;
  font-size: 23rpx;
  color: var(--text-sub);
}

/* ---------- 进度 ---------- */
.progress {
  display: flex;
  align-items: center;
  margin-top: 22rpx;
  padding: 0 4rpx;
}

.node {
  flex-shrink: 0;
}

.node-label {
  font-size: 23rpx;
  color: #b6bec6;
}

.node.reached .node-label {
  color: var(--brand);
  font-weight: 700;
}

.node-line {
  flex: 1;
  height: 2rpx;
  background: #e6eae8;
  margin: 0 14rpx;
}

.node-line.reached {
  background: #a8d8d2;
}

/* ---------- 操作 ---------- */
.actions {
  display: flex;
  gap: 16rpx;
  margin-top: 22rpx;
}

.act {
  flex: 1;
  text-align: center;
  padding: 18rpx 0;
  border-radius: 12rpx;
  font-size: 25rpx;
}

.act.ghost {
  border: 1rpx solid #e6eae8;
  color: var(--text);
  background: #fff;
}

.act.primary {
  background: var(--brand-light);
  color: var(--brand-dark);
  font-weight: 600;
}

.loading {
  text-align: center;
  color: var(--text-light);
  font-size: 23rpx;
  padding: 28rpx 0;
}
</style>
