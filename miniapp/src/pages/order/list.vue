<script setup lang="ts">
import { ref } from 'vue'
import { onReachBottom, onShow } from '@dcloudio/uni-app'
import TabBar from '@/components/TabBar.vue'
import { listOrders, type Order } from '@/api'
import { useUserStore } from '@/store/user'
import { ensureAuth } from '@/utils/auth'
import { friendlyDate, price, SERVICE_LABEL, slotTextOf } from '@/utils/format'
import { contactProvider } from '@/utils/provider-contact'

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
  return slotTextOf(code)
}

/** 卡片副标题：小区 + 单次/包月 */
function subLine(o: Order): string {
  const area = o.address ? o.address.community : ''
  const plan = o.plan === 'monthly' ? '包月服务' : '单次服务'
  return [area, plan].filter(Boolean).join(' · ')
}

/** 时间文案：今天/明天优先，其他显示日期；包月选了备选时间则显示备选 */
function timeLine(o: Order): string {
  const d = friendlyDate(o.service_date)
  const base = `${d} ${slotLabel(o.extra_slot_code || o.slot_code)}`
  // 拒单自动退款后，列表里就能看到钱去哪了，不用点进详情查
  if (o.status === 'rejected' && (o.refunded_amount || 0) > 0) {
    return `${base} · 已退 ¥${((o.refunded_amount || 0) / 100).toFixed(2)}`
  }
  return base
}

function statusClass(status: string): string {
  // 待支付与待接单都属于「待服务」，用橙色；履约中才是主色绿
  if (status === 'pending_payment' || status === 'pending_accept') return 'warn'
  if (status === 'completed') return 'done'
  if (status === 'canceled' || status === 'rejected') return 'closed'
  return 'active'
}

/** 待确认改期的目标时间文案。模板里抽成函数，避免 vue-tsc 对嵌套可空字段的窄化报错 */
function rescheduleTime(o: Order): string {
  const rs = o.pending_reschedule
  if (!rs) return ''
  return `${friendlyDate(rs.to_date)} ${slotLabel(rs.to_slot)}`
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
  // 有待确认的改期申请时，主按钮直接指向「处理改期」而不是泛泛的「查看进度」
  if (o.pending_reschedule) {
    return [
      { label: '联系服务方', kind: 'ghost', key: 'contact' },
      { label: '处理改期', kind: 'primary', key: 'detail' },
    ]
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
      // 未接单只提示，接单后才显示电话（逻辑集中在 utils/provider-contact）
      contactProvider(o.order_no)
      break
    default:
      goDetail(o.order_no)
  }
}
</script>

<template>
  <view class="page">
    <!-- 分段控制器吸顶：列表滚动时保持在顶部，切换 tab 不用滚回去 -->
    <view class="seg-wrap">
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
    </view>

    <view v-if="!loading && orders.length === 0" class="empty">
      还没有订单，回首页挑一项服务吧
    </view>

    <view v-for="o in orders" :key="o.order_no" class="order-card">
      <!-- 服务方申请改期：必须在列表就能看到，否则住户要逐单点进详情才知道有人在等自己确认 -->
      <view v-if="o.pending_reschedule" class="rs-banner" @tap.stop="goDetail(o.order_no)">
        <view class="rs-banner-body">
          <view class="rs-banner-title">
            <text class="rs-banner-tag">待确认</text>
            <text class="rs-banner-name">服务方申请改期</text>
          </view>
          <text class="rs-banner-time">改到 {{ rescheduleTime(o) }}</text>
        </view>
        <text class="rs-banner-go">去处理 ›</text>
      </view>

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
  /* 顶部内边距交给吸顶容器，否则吸顶后卡片会从缝隙里露出来 */
  padding-bottom: 160rpx;
}

/*
 * 分段器吸顶容器：
 * - sticky 不占额外高度，滚动到位后自动黏住，省得算固定高度；
 * - 必须给不透明底色，否则卡片会从分段器下方的圆角缝隙里透出来。
 */
.seg-wrap {
  position: sticky;
  top: 0;
  z-index: 20;
  padding: 20rpx 0 0;
  background: var(--bg);
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

/* ---------- 改期待确认提示 ---------- */
/* 用橙色（牌面里代表「需要你动作」）而不是主色绿，避免和「进行中」的常态混淆 */
.rs-banner {
  display: flex;
  align-items: center;
  gap: 16rpx;
  margin-bottom: 20rpx;
  padding: 16rpx 18rpx;
  background: #fff8ed;
  border: 1rpx solid #fcd9a4;
  border-radius: 14rpx;
}

.rs-banner-body {
  flex: 1;
  min-width: 0;
}

.rs-banner-title {
  display: flex;
  align-items: center;
  gap: 10rpx;
}

.rs-banner-tag {
  flex-shrink: 0;
  padding: 2rpx 10rpx;
  border-radius: 6rpx;
  background: #d97706;
  color: #fff;
  font-size: 19rpx;
  font-weight: 700;
}

.rs-banner-name {
  font-size: 25rpx;
  font-weight: 600;
  color: #b45309;
}

.rs-banner-time {
  display: block;
  margin-top: 6rpx;
  font-size: 22rpx;
  color: #b45309;
  opacity: 0.86;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rs-banner-go {
  flex-shrink: 0;
  font-size: 23rpx;
  font-weight: 600;
  color: #d97706;
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
