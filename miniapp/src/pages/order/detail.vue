<script setup lang="ts">
import { computed, ref } from 'vue'
import { onLoad } from '@dcloudio/uni-app'
import {
  advanceOrder,
  cancelOrder,
  confirmOrder,
  listRecords,
  orderDetail,
  payOrder,
  uploadRecord,
  type Order,
  type ServiceRecord,
} from '@/api'
import { useUserStore } from '@/store/user'
import { friendlyDate, price, SERVICE_LABEL, SLOTS } from '@/utils/format'
import { ApiError, BASE_URL, getUserId } from '@/utils/request'

const user = useUserStore()
const orderNo = ref('')
const order = ref<Order | null>(null)
const records = ref<ServiceRecord[]>([])
const loading = ref(true)
const acting = ref(false)
const uploading = ref(false)

/** 服务方视角：接单后可推进节点、上传记录 */
const isProvider = computed(() => user.role === 'provider')

const slotText = computed(() => {
  const code = order.value?.slot_code || ''
  return SLOTS.find((s) => s.code === code)?.label || code
})

/** 服务地址上的联系人与联系方式 */
const addressContact = computed(() => {
  const a = order.value?.address
  if (!a) return ''
  return [a.contact_name, a.contact_phone].filter(Boolean).join(' · ')
})

/** 状态区文案 */
const statusHint = computed(() => {
  const o = order.value
  if (!o) return ''
  switch (o.status) {
    case 'pending_payment':
      return o.pay_expires_at
        ? `请在 ${formatTime(o.pay_expires_at)} 前完成支付`
        : '请尽快完成支付'
    case 'pending_accept':
      return '已支付，等待服务方接单'
    case 'accepted':
      return '服务方已接单，请保持电话畅通'
    case 'on_the_way':
      return '服务方正在前往，请留意门禁'
    case 'in_service':
      return '服务进行中'
    case 'pending_confirm':
      return '服务已完成，请确认后结束订单'
    case 'completed':
      return '感谢使用，欢迎再次预约'
    case 'canceled':
      return o.remark || '订单已取消'
    case 'rejected':
      return '服务方无法接单，已为你关闭订单'
    default:
      return ''
  }
})

onLoad(async (query) => {
  orderNo.value = (query?.orderNo as string) || ''
  await load()
})

async function load() {
  if (!orderNo.value) return
  loading.value = true
  try {
    await user.ensureLogin()
    order.value = await orderDetail(orderNo.value)
    // 已完成的订单才有服务记录可看
    if (order.value.status === 'completed' || order.value.status === 'pending_confirm') {
      records.value = await listRecords(orderNo.value).catch(() => [])
    }
  } catch (e) {
    uni.showToast({ title: e instanceof ApiError ? e.message : '加载失败', icon: 'none' })
  } finally {
    loading.value = false
  }
}

function formatTime(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return `${pad(d.getHours())}:${pad(d.getMinutes())}`
}

function pad(n: number): string {
  return n < 10 ? '0' + n : String(n)
}

/** 支付。沙箱下走 mock-pay，真实环境换成 uni.requestPayment */
async function onPay() {
  if (acting.value || !order.value) return
  acting.value = true
  try {
    const params = await payOrder(order.value.order_no)

    // #ifdef MP-WEIXIN
    // 真实支付：uni.requestPayment({ provider: 'wxpay', ...params.params })
    uni.showModal({
      title: '待支付',
      content: params.mock ? '当前为沙箱渠道，请用 H5 或接口完成支付' : '请在小程序内完成支付',
      showCancel: false,
    })
    // #endif

    // #ifndef MP-WEIXIN
    if (params.mock) {
      await mockPay(order.value.order_no)
      uni.showToast({ title: '支付成功', icon: 'success' })
      await load()
    }
    // #endif
  } catch (e) {
    uni.showModal({
      title: '支付失败',
      content: e instanceof ApiError ? e.message : '请稍后重试',
      showCancel: false,
    })
  } finally {
    acting.value = false
  }
}

/** 构造模拟支付请求（仅开发环境可用） */
function mockPay(orderNumber: string) {
  return new Promise<void>((resolve, reject) => {
    uni.request({
      url: `${BASE_URL}/api/v1/dev/orders/${orderNumber}/mock-pay`,
      method: 'POST',
      header: { 'X-User-Id': getUserId() },
      success: (res) => {
        const body = res.data as { code: number; message: string }
        if (body?.code === 0) resolve()
        else reject(new ApiError(body?.code ?? -1, body?.message || '模拟支付失败'))
      },
      fail: () => reject(new ApiError(-1, '网络异常')),
    })
  })
}

async function onCancel() {
  if (!order.value || acting.value) return
  const r = await new Promise<boolean>((resolve) => {
    uni.showModal({
      title: '确认取消订单？',
      content: '服务开始前 2 小时以上可免费取消并原路退款；2 小时内需运营审核',
      confirmText: '确认取消',
      cancelText: '再想想',
      success: (res) => resolve(res.confirm),
      fail: () => resolve(false),
    })
  })
  if (!r) return

  acting.value = true
  try {
    const res = await cancelOrder(order.value.order_no, '用户主动取消')
    if (res.need_review) {
      uni.showModal({ title: '已提交申请', content: res.message || '需运营审核', showCancel: false })
    } else if (res.canceled) {
      uni.showModal({
        title: '订单已取消',
        content: res.hint || '退款将原路退回',
        showCancel: false,
      })
    }
    await load()
  } catch (e) {
    uni.showModal({
      title: '取消失败',
      content: e instanceof ApiError ? e.message : '请稍后重试',
      showCancel: false,
    })
  } finally {
    acting.value = false
  }
}

async function onConfirm() {
  if (!order.value || acting.value) return
  acting.value = true
  try {
    await confirmOrder(order.value.order_no)
    uni.showToast({ title: '已确认完成', icon: 'success' })
    await load()
  } catch (e) {
    uni.showToast({ title: e instanceof ApiError ? e.message : '操作失败', icon: 'none' })
  } finally {
    acting.value = false
  }
}

function previewImage(rec: ServiceRecord) {
  const images = records.value.filter((r) => r.media_type === 'photo')
  const urls = images.map((r) => `${BASE_URL}${r.url}`)
  const current = `${BASE_URL}${rec.url}`
  // 附件需要带 X-User-Id，H5 下可直接预览；小程序需先下载再 preview
  uni.previewImage({ urls, current })
}

/** 服务方按状态可执行的推进动作，合法性最终由后端白名单兜底 */
const providerActions = computed(() => {
  const o = order.value
  if (!isProvider.value || !o) return []
  const acts: { node: string; label: string }[] = []
  if (o.status === 'accepted') {
    if (o.service_type === 'garbage') acts.push({ node: 'on_the_way', label: '出发上门' })
    acts.push({ node: 'in_service', label: '开始服务' })
  } else if (o.status === 'on_the_way') {
    acts.push({ node: 'in_service', label: '开始服务' })
  } else if (o.status === 'in_service') {
    acts.push({ node: 'pending_confirm', label: '完成服务' })
  }
  return acts
})

/** 已接单到完成后都可补传照片 */
const canUploadRecord = computed(() => {
  if (!isProvider.value || !order.value) return false
  return ['accepted', 'on_the_way', 'in_service', 'pending_confirm', 'completed'].includes(
    order.value.status,
  )
})

async function onAdvance(node: string) {
  if (!order.value || acting.value) return
  acting.value = true
  try {
    await advanceOrder(order.value.order_no, node)
    uni.showToast({ title: '已更新', icon: 'success' })
    await load()
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

function chooseUpload() {
  if (uploading.value || !order.value) return
  // #ifdef MP-WEIXIN
  uni.chooseMedia({
    count: 1,
    mediaType: ['image'],
    success: (res) => {
      if (res.tempFiles?.length) doUpload(res.tempFiles[0].tempFilePath)
    },
  })
  // #endif
  // #ifndef MP-WEIXIN
  uni.chooseImage({
    count: 1,
    success: (res) => {
      if (res.tempFilePaths?.length) doUpload(res.tempFilePaths[0])
    },
  })
  // #endif
}

async function doUpload(path: string) {
  const orderNo = order.value?.order_no
  if (!orderNo) return
  uploading.value = true
  try {
    await uploadRecord(orderNo, path, 'photo')
    uni.showToast({ title: '已上传', icon: 'success' })
    records.value = await listRecords(orderNo)
  } catch (e) {
    uni.showToast({ title: e instanceof ApiError ? e.message : '上传失败', icon: 'none' })
  } finally {
    uploading.value = false
  }
}

/** 能否取消 */
const cancellable = computed(() => {
  const s = order.value?.status
  return s === 'pending_payment' || s === 'pending_accept' || s === 'accepted'
})
</script>

<template>
  <view class="page">
    <view v-if="loading" class="empty">加载中…</view>

    <template v-else-if="order">
      <!-- 状态 -->
      <view class="status-hero" :class="`st-${order.status}`">
        <view class="status-title">{{ order.status_label }}</view>
        <view class="status-hint">{{ statusHint }}</view>
      </view>

      <!-- 时间轴 -->
      <view v-if="order.nodes?.length" class="card">
        <view class="section-title">服务进度</view>
        <view class="timeline">
          <view
            v-for="(n, i) in order.nodes"
            :key="n.node"
            class="node"
            :class="{ reached: n.reached }"
          >
            <view class="node-dot">
              <view class="inner" />
            </view>
            <view v-if="i < order.nodes.length - 1" class="node-line" />
            <view class="node-body">
              <view class="node-label">{{ n.label }}</view>
              <view v-if="n.at" class="node-time">{{ n.at.slice(5, 16).replace('T', ' ') }}</view>
              <!-- 未到达不写「待完成」：三个节点都这么写会显得像重复渲染 -->
              <view v-else class="node-time placeholder" />
            </view>
          </view>
        </view>
      </view>

      <!-- 服务信息 -->
      <view class="card">
        <view class="section-title">服务信息</view>
        <view class="info-row">
          <text class="info-label">服务项目</text>
          <text class="info-value">
            {{ SERVICE_LABEL[order.service_type] }}{{ order.plan === 'monthly' ? '（包月）' : '' }}
          </text>
        </view>
        <view class="info-row">
          <text class="info-label">服务时间</text>
          <text class="info-value">{{ friendlyDate(order.service_date) }} {{ slotText }}</text>
        </view>
        <view v-if="order.address" class="info-row">
          <text class="info-label">服务地址</text>
          <text class="info-value">
            {{ order.address.community }} {{ order.address.lane }} {{ order.address.house_no }}
          </text>
        </view>
        <view v-if="addressContact" class="info-row">
          <text class="info-label">联系人</text>
          <text class="info-value">{{ addressContact }}</text>
        </view>
        <view v-if="order.hospital" class="info-row">
          <text class="info-label">就诊医院</text>
          <text class="info-value">{{ order.hospital }}</text>
        </view>
        <view v-if="order.patient_name" class="info-row">
          <text class="info-label">就诊人</text>
          <text class="info-value">{{ order.patient_name }}</text>
        </view>
        <view v-if="order.accompany_items?.length" class="info-row">
          <text class="info-label">陪诊服务</text>
          <text class="info-value">
            {{ order.accompany_items.map((it) => it.name).join('、') }}
          </text>
        </view>
        <view v-if="order.remark" class="info-row">
          <text class="info-label">备注</text>
          <text class="info-value">{{ order.remark }}</text>
        </view>
        <view class="info-row">
          <text class="info-label">订单号</text>
          <text class="info-value mono">{{ order.order_no }}</text>
        </view>
      </view>

      <!-- 陪诊报价 -->
      <view v-if="order.quote" class="card quote-card">
        <view class="section-title">运营报价</view>
        <view class="info-row">
          <text class="info-label">报价金额</text>
          <text class="info-value strong">{{ price(order.quote.amount) }}</text>
        </view>
        <view class="info-row">
          <text class="info-label">预计时长</text>
          <text class="info-value">{{ order.quote.duration_min }} 分钟</text>
        </view>
        <view class="info-row">
          <text class="info-label">状态</text>
          <text class="info-value">{{ order.quote.confirmed ? '已确认' : '待确认支付' }}</text>
        </view>
      </view>

      <!-- 费用 -->
      <view v-if="order.price_items?.length" class="card">
        <view class="section-title">费用明细</view>
        <view v-for="(item, i) in order.price_items" :key="i" class="price-row">
          <view class="price-name">
            {{ item.name }}
            <text v-if="item.note" class="price-note">{{ item.note }}</text>
          </view>
          <text>{{ item.amount === 0 ? '免费' : price(item.amount) }}</text>
        </view>
        <view class="divider" />
        <view class="price-row total">
          <text>合计</text>
          <text class="total-amount">{{ price(order.total_amount) }}</text>
        </view>
        <view v-if="order.paid_amount > 0" class="price-row">
          <text class="muted">已支付</text>
          <text class="muted">{{ price(order.paid_amount) }}</text>
        </view>
      </view>

      <!-- 服务记录 -->
      <view v-if="records.length || canUploadRecord" class="card">
        <view class="row">
          <view class="section-title">服务记录</view>
          <view v-if="canUploadRecord" class="upload-btn" @tap="chooseUpload">
            {{ uploading ? '上传中…' : '📷 上传照片' }}
          </view>
        </view>
        <view v-if="!records.length" class="hint">还没有记录，服务过程中拍照上传</view>
        <view class="record-grid">
          <view v-for="r in records" :key="r.id" class="record" @tap="previewImage(r)">
            <image
              v-if="r.media_type === 'photo'"
              class="thumb"
              :src="`${BASE_URL}${r.url}`"
              mode="aspectFill"
            />
            <view v-else class="thumb video">
              <text class="video-icon">▶</text>
              <text class="video-label">{{ r.duration_sec }}s</text>
            </view>
          </view>
        </view>
        <view class="hint">记录由服务方上传，不可删除</view>
      </view>

      <!-- 操作：服务方推进节点，住户支付/取消/确认 -->
      <view v-if="order.status !== 'canceled' && order.status !== 'rejected'" class="bottom-bar">
        <template v-if="isProvider">
          <view
            v-for="a in providerActions"
            :key="a.node"
            class="btn-primary half"
            @tap="onAdvance(a.node)"
          >
            {{ acting ? '处理中…' : a.label }}
          </view>
          <view v-if="!providerActions.length" class="btn-ghost half">
            等待住户确认
          </view>
        </template>
        <template v-else>
          <view v-if="cancellable" class="btn-ghost half" @tap="onCancel">取消订单</view>
          <view v-if="order.status === 'pending_payment'" class="btn-primary half" @tap="onPay">
            {{ acting ? '处理中…' : '立即支付' }}
          </view>
          <view v-if="order.status === 'pending_confirm'" class="btn-primary half" @tap="onConfirm">
            确认完成
          </view>
        </template>
      </view>
    </template>

    <view v-else class="empty">订单不存在或已关闭</view>
  </view>
</template>

<style scoped>
.page {
  padding: 24rpx 0 180rpx;
}

.status-hero {
  margin: 20rpx 24rpx;
  padding: 36rpx 32rpx;
  border-radius: 22rpx;
  background: linear-gradient(135deg, #e9f5f3 0%, #e4f3f0 100%);
  color: #14413c;
}

.status-hero.st-pending_payment {
  background: linear-gradient(135deg, #fef5e6 0%, #fdf0dd 100%);
  color: #8a5a12;
}

.status-hero.st-completed {
  background: #f1f4f3;
  color: #3f4a48;
}

.status-hero.st-canceled,
.status-hero.st-rejected {
  background: #f4f5f6;
  color: #7b8489;
}

.status-title {
  font-size: 40rpx;
  font-weight: 800;
}

.status-hint {
  margin-top: 12rpx;
  font-size: 24rpx;
  opacity: 0.86;
  line-height: 1.5;
}

.timeline {
  display: flex;
  flex-direction: column;
  position: relative;
}

.node {
  display: flex;
  gap: 20rpx;
  position: relative;
  padding-bottom: 32rpx;
}

.node:last-child {
  padding-bottom: 0;
}

.node-dot {
  width: 28rpx;
  height: 28rpx;
  border-radius: 50%;
  border: 3rpx solid #d1d5db;
  background: #fff;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  margin-top: 4rpx;
  z-index: 2;
}

.node.reached .node-dot {
  border-color: var(--brand);
}

.node-dot .inner {
  width: 12rpx;
  height: 12rpx;
  border-radius: 50%;
  background: #d1d5db;
}

.node.reached .node-dot .inner {
  background: var(--brand);
}

.node-line {
  position: absolute;
  left: 13rpx;
  top: 30rpx;
  bottom: 0;
  width: 2rpx;
  background: #e5e7eb;
}

.node.reached .node-line {
  background: var(--brand-light);
}

.node-body {
  flex: 1;
}

.node-label {
  font-size: 27rpx;
  color: var(--text-light);
}

.node.reached .node-label {
  color: var(--text);
  font-weight: 500;
}

.node-time {
  font-size: 23rpx;
  color: var(--text-light);
  margin-top: 4rpx;
  min-height: 30rpx;
}

.node-time.placeholder {
  height: 30rpx;
}

.info-row {
  display: flex;
  justify-content: space-between;
  gap: 24rpx;
  padding: 14rpx 0;
  font-size: 27rpx;
}

.info-label {
  color: var(--text-sub);
  flex-shrink: 0;
}

.info-value {
  text-align: right;
  color: var(--text);
  word-break: break-all;
}

.info-value.strong {
  font-weight: 600;
  color: #d97706;
}

.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 24rpx;
  color: var(--text-sub);
}

.price-row {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  padding: 12rpx 0;
  font-size: 27rpx;
  color: var(--text-sub);
}

.price-name {
  display: flex;
  flex-direction: column;
  gap: 4rpx;
}

.price-note {
  font-size: 22rpx;
  color: var(--text-light);
}

.divider {
  height: 1rpx;
  background: #f3f4f6;
  margin: 12rpx 0;
}

.total {
  align-items: center;
  color: var(--text);
}

.total-amount {
  font-size: 34rpx;
  font-weight: 700;
  color: #ef4444;
}

.record-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 16rpx;
}

.record {
  width: 150rpx;
  height: 150rpx;
  border-radius: 16rpx;
  overflow: hidden;
  background: #f3f4f6;
}

.thumb {
  width: 100%;
  height: 100%;
}

.thumb.video {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 6rpx;
  color: var(--text-sub);
}

.video-icon {
  font-size: 32rpx;
}

.video-label {
  font-size: 22rpx;
}

.hint {
  margin-top: 16rpx;
  font-size: 22rpx;
  color: var(--text-light);
}

.bottom-bar {
  position: fixed;
  left: 0;
  right: 0;
  bottom: 0;
  display: flex;
  gap: 20rpx;
  padding: 20rpx 24rpx;
  padding-bottom: calc(20rpx + env(safe-area-inset-bottom));
  background: #fff;
  border-top: 1rpx solid var(--border);
}

.half {
  flex: 1;
}

.upload-btn {
  font-size: 24rpx;
  color: var(--brand-dark);
  background: var(--brand-light);
  padding: 10rpx 24rpx;
  border-radius: 26rpx;
}

.upload-btn {
  font-size: 24rpx;
  color: var(--brand-dark);
  background: var(--brand-light);
  padding: 10rpx 24rpx;
  border-radius: 26rpx;
}
</style>
