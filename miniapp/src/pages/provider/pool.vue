<script setup lang="ts">
import { ref } from 'vue'
import { onReachBottom, onShow } from '@dcloudio/uni-app'
import TabBar from '@/components/TabBar.vue'
import { acceptOrder, providerOpenOrders, type Order } from '@/api'
import { useUserStore } from '@/store/user'
import { ensureAuth } from '@/utils/auth'
import { friendlyDate, price, SERVICE_LABEL, slotTextOf } from '@/utils/format'
import { ApiError } from '@/utils/request'

const user = useUserStore()

const orders = ref<Order[]>([])
const loading = ref(false)
const finished = ref(false)
const page = ref(1)
const actingNo = ref('') // 正在抢单的订单号，按钮转圈用

onShow(async () => {
  if (!ensureAuth()) return
  await user.ensureLogin()
  // 无服务方权限不加载；有权限但从住户视角进来时切到服务方视角，
  // 否则底部导航显示的是住户三项，与当前页面不一致
  if (!user.isProvider) return
  if (!user.isProviderView) user.switchView('provider')
  reload()
})

onReachBottom(() => {
  if (!finished.value && !loading.value) loadMore()
})

function reload() {
  page.value = 1
  finished.value = false
  orders.value = []
  fetchPage()
}

function loadMore() {
  page.value += 1
  fetchPage()
}

async function fetchPage() {
  if (loading.value) return
  loading.value = true
  try {
    const res = await providerOpenOrders(page.value, 10)
    orders.value = page.value === 1 ? res.list : [...orders.value, ...res.list]
    if (orders.value.length >= res.total) finished.value = true
  } catch (e) {
    uni.showToast({ title: '加载失败', icon: 'none' })
  } finally {
    loading.value = false
  }
}

/**
 * 抢单。
 * 并发抢同一单时后端靠「状态 + 未派单」条件更新保证只有一人成功，
 * 这里拿到失败提示后刷新列表，让池子里消失的那单立即从视野移除。
 */
async function onAccept(o: Order) {
  if (actingNo.value) return
  actingNo.value = o.order_no
  try {
    await acceptOrder(o.order_no)
    // 包月接的是整段服务期，不是一个时段：弹窗里把「每天要来」说清楚，
    // 否则服务方按单次的心态接下，第二天才发现还有 29 天
    const content =
      o.plan === 'monthly'
        ? `${SERVICE_LABEL[o.service_type]} · 包月至 ${shortDate(o.monthly_end || o.service_date)}，每天需上门，当天在任务列表打卡`
        : `${SERVICE_LABEL[o.service_type]} · ${friendlyDate(o.service_date)}，请在任务列表查看`
    uni.showModal({
      title: '接单成功',
      content,
      showCancel: false,
    })
    await fetchPage()
  } catch (e) {
    uni.showModal({
      title: '没抢到',
      content: e instanceof ApiError ? e.message : '该订单可能已被其他服务方接走',
      showCancel: false,
    })
    await fetchPage()
  } finally {
    actingNo.value = ''
  }
}

function slotLabel(code: string): string {
  return slotTextOf(code)
}

/**
 * 包月订单不是「跑一趟」，是每天上门跑满整段服务期。
 * 接单池里它的价格和单次一模一样（都是 ¥30），只写日期和时段的话
 * 服务方会当成普通单次接走，接了才发现要连着来一个月。
 */
function monthlyPlanText(o: Order): string {
  const start = o.monthly_start || o.service_date
  const days = monthlyDays(o)
  return `${shortDate(start)} 起${days > 0 ? ' ' + days + ' 天' : ''} · 每天 ${slotLabel(o.extra_slot_code || o.slot_code)}`
}

/** 服务期天数（含首尾）。直接用日期字符串相减，避免本地时区把首尾各算错一天 */
function monthlyDays(o: Order): number {
  if (!o.monthly_start || !o.monthly_end) return 0
  const ms = Date.parse(o.monthly_end) - Date.parse(o.monthly_start)
  if (Number.isNaN(ms)) return 0
  return Math.round(ms / 86400000) + 1
}

/** 2026-11-22 → 11/22，接单池不需要年份 */
function shortDate(d: string): string {
  if (!d) return ''
  return d.slice(5).replace('-', '/')
}

/** 服务时间一行说清：单次是某天某时段，包月是起始日 + 天数 + 每天时段 */
function whenText(o: Order): string {
  if (o.plan === 'monthly') return monthlyPlanText(o)
  return `${friendlyDate(o.service_date)} · ${slotLabel(o.slot_code)}`
}
</script>

<template>
  <view class="page">
    <view class="tip">
      接单后请在约定时段上门服务，完成后上传照片留证。连续拒单会影响接单优先级。
    </view>

    <view v-if="!loading && orders.length === 0" class="empty">
      暂无可接订单，稍后再来看看
    </view>

    <view v-for="o in orders" :key="o.order_no" class="pool-card">
      <view class="head">
        <view class="head-left">
          <text class="service">{{ SERVICE_LABEL[o.service_type] }}</text>
          <!-- 包月和单次在池子里价格相同，不给标识等于让服务方盲接 -->
          <text v-if="o.plan === 'monthly'" class="badge">包月</text>
        </view>
        <text class="amount">{{ price(o.total_amount) }}</text>
      </view>

      <view class="meta">
        {{ whenText(o) }}
      </view>

      <view v-if="o.address" class="addr">
        {{ o.address.community }} {{ [o.address.lane, o.address.house_no].filter(Boolean).join(' ') }}
      </view>
      <view v-if="o.address && (o.address.contact_name || o.address.contact_phone)" class="contact">
        联系人：{{ o.address.contact_name }} {{ o.address.contact_phone }}
      </view>
      <view v-if="o.remark" class="remark">备注：{{ o.remark }}</view>

      <view class="foot">
        <text class="order-no">{{ o.order_no }}</text>
        <view
          class="accept-btn"
          :class="{ disabled: actingNo === o.order_no }"
          @tap.stop="onAccept(o)"
        >
          <!-- 包月是一个月的活，按钮上就写明白，别让服务方按单次的心态点下去 -->
          {{ actingNo === o.order_no ? '抢单中…' : o.plan === 'monthly' ? '接包月单' : '立即接单' }}
        </view>
      </view>
    </view>

    <view v-if="loading" class="loading">加载中…</view>
    <view v-else-if="finished && orders.length > 0" class="loading">没有更多了</view>

    <TabBar current="pool" />
  </view>
</template>

<style scoped>
.page {
  padding-top: 24rpx;
  /* 给固定 TabBar 留位置 */
  padding-bottom: 160rpx;
}

.tip {
  margin: 0 24rpx 20rpx;
  padding: 20rpx 28rpx;
  background: #fffbeb;
  border: 1rpx solid #fde68a;
  border-radius: 16rpx;
  font-size: 23rpx;
  color: #92400e;
  line-height: 1.6;
}

.pool-card {
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

/* 包月标签：与任务列表用同一套颜色，服务方在两个页面看到的是同一个记号 */
.badge {
  font-size: 20rpx;
  color: #b45309;
  background: #fef3c7;
  padding: 4rpx 12rpx;
  border-radius: 8rpx;
}

.service {
  font-size: 30rpx;
  font-weight: 600;
}

.amount {
  font-size: 32rpx;
  font-weight: 700;
  color: #ef4444;
}

.meta {
  margin-top: 12rpx;
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

.foot {
  margin-top: 20rpx;
  padding-top: 20rpx;
  border-top: 1rpx solid #f3f4f6;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.order-no {
  font-size: 22rpx;
  color: var(--text-light);
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
}

.accept-btn {
  background: #d97706;
  color: #fff;
  padding: 14rpx 40rpx;
  border-radius: 32rpx;
  font-size: 26rpx;
  font-weight: 500;
}

.accept-btn.disabled {
  background: #d1d5db;
}

.loading {
  text-align: center;
  color: var(--text-light);
  font-size: 24rpx;
  padding: 30rpx 0;
}
</style>
