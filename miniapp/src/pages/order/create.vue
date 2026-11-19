<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { onLoad } from '@dcloudio/uni-app'
import CalendarPicker from '@/components/CalendarPicker.vue'
import {
  accompanyOptions,
  createOrder,
  previewOrder,
  type AccompanyHospital,
  type AccompanyServiceItem,
  type CreateOrderPayload,
  type PreviewResp,
} from '@/api'
import { useUserStore } from '@/store/user'
import { availableSlots, nextDays, price, SLOTS } from '@/utils/format'
import { ApiError } from '@/utils/request'

const user = useUserStore()

const dates = nextDays(7)
const durations = [20, 30, 40, 60]
const garbageTypes = [
  { value: 'kitchen', label: '厨余垃圾' },
  { value: 'other', label: '其他垃圾' },
  { value: 'recyclable', label: '可回收物' },
]

const form = reactive({
  serviceType: 'garbage',
  serviceDate: dates[0].value,
  slotCode: '',
  plan: 'single',
  garbageType: 'kitchen',
  petCount: 1,
  durationMin: 20,
  feedOnly: false,
  needFamiliar: false,
  patientName: '',
  hospital: '', // 存 code，名称从后台选项映射
  serviceItems: [] as string[], // 已勾选的陪诊服务项 code
  remark: '',
})

const preview = ref<PreviewResp | null>(null)
const previewError = ref('')
const submitting = ref(false)
const showCalendar = ref(false)

/** 陪诊可选项全部来自后台配置，前端不内置医院与服务项 */
const hospitals = ref<AccompanyHospital[]>([])
const serviceItemOptions = ref<AccompanyServiceItem[]>([])

const hospitalName = computed(() =>
  hospitals.value.find((h) => h.code === form.hospital)?.name || '',
)

/** 顶部 banner 按服务类型切换文案 */
const banner = computed(() => {
  const map: Record<string, { title: string; sub: string; art: string }> = {
    garbage: { title: '上门代倒垃圾', sub: '规范分类，按时上门', art: '🗑' },
    dogwalk: { title: '上门遛宠', sub: '20 分钟起，可加时长', art: '🐾' },
    accompany: { title: '医院陪诊', sub: '按项目明码计价', art: '🏥' },
  }
  return map[form.serviceType] || map.garbage
})

const slots = computed(() =>
  form.serviceType === 'dogwalk' ? availableSlots(form.serviceDate) : SLOTS,
)

const addressText = computed(() => {
  const a = user.currentAddress
  if (!a) return ''
  return `${a.community} ${a.lane} ${a.house_no}`
})

/** 所有服务都在地址卡展示联系人与联系方式，方便服务方上门联系 */
const addressContact = computed(() => {
  const a = user.currentAddress
  if (!a) return ''
  return [a.contact_name, a.contact_phone].filter(Boolean).join(' · ')
})

const timeText = computed(() => {
  if (!form.slotCode) return ''
  const d = dates.find((x) => x.value === form.serviceDate)
  const s = slots.value.find((x) => x.code === form.slotCode)
  return [d?.label, s?.label].filter(Boolean).join(' ')
})

onLoad(async (query) => {
  if (query?.type && ['garbage', 'dogwalk', 'accompany'].includes(query.type)) {
    form.serviceType = query.type as string
  }
  try {
    await user.ensureLogin()
    await user.loadAddresses()
  } catch (e) {
    console.warn('[create] 初始化失败', e)
  }
  await loadAccompanyOptions()
  syncSlot()
  refreshPreview()
})

/** 医院与陪诊服务项都从后台拉取，用户只能选、不能自由输入 */
async function loadAccompanyOptions() {
  try {
    const opts = await accompanyOptions()
    hospitals.value = opts.hospitals || []
    serviceItemOptions.value = opts.service_items || []
    if (!hospitals.value.some((h) => h.code === form.hospital)) {
      form.hospital = hospitals.value[0]?.code || ''
    }
    const valid = new Set(serviceItemOptions.value.map((it) => it.code))
    form.serviceItems = form.serviceItems.filter((c) => valid.has(c))
  } catch (e) {
    console.warn('[create] 陪诊选项加载失败', e)
  }
}

function chooseHospital() {
  if (!hospitals.value.length) {
    uni.showToast({ title: '暂无可选医院', icon: 'none' })
    return
  }
  uni.showActionSheet({
    itemList: hospitals.value.map((h) => h.name),
    success: (r) => {
      form.hospital = hospitals.value[r.tapIndex].code
    },
  })
}

function toggleServiceItem(code: string) {
  const i = form.serviceItems.indexOf(code)
  if (i >= 0) form.serviceItems.splice(i, 1)
  else form.serviceItems.push(code)
}

function syncSlot() {
  const list = slots.value
  if (!list.some((s) => s.code === form.slotCode)) {
    form.slotCode = list.length ? list[0].code : ''
  }
}

watch(
  () => form.serviceDate,
  () => syncSlot(),
)

let timer: ReturnType<typeof setTimeout> | null = null
watch(
  () => [
    form.serviceType,
    form.serviceDate,
    form.slotCode,
    form.plan,
    form.petCount,
    form.durationMin,
    form.feedOnly,
    form.needFamiliar,
    form.hospital,
    form.serviceItems.join(','), // 数组内容变化也要触发试算
  ],
  () => {
    if (timer) clearTimeout(timer)
    timer = setTimeout(refreshPreview, 300)
  },
)

function buildPayload(): CreateOrderPayload | null {
  const addr = user.currentAddress
  if (!addr) return null

  const payload: CreateOrderPayload = {
    address_id: addr.id,
    service_type: form.serviceType,
    service_date: form.serviceDate,
    slot_code: form.slotCode || slots.value[0]?.code || '',
    remark: form.remark,
  }

  if (form.serviceType === 'garbage') {
    payload.plan = form.plan
    payload.garbage_type = form.garbageType
  } else if (form.serviceType === 'dogwalk') {
    payload.pet_count = form.petCount
    payload.duration_min = form.durationMin
    payload.feed_only = form.feedOnly
    payload.need_familiar = form.needFamiliar
  } else {
    // 陪诊：只回传后台配置项的 code，价格由后端按 SKU 规则计算
    payload.hospital = form.hospital
    payload.patient_name = form.patientName.trim()
    payload.service_items = [...form.serviceItems]
    payload.remark = form.remark
  }

  return payload
}

async function refreshPreview() {
  if (!user.currentAddress) {
    preview.value = null
    previewError.value = '请先添加服务地址'
    return
  }
  const payload = buildPayload()
  if (!payload || !payload.slot_code) return
  try {
    preview.value = await previewOrder(payload)
    previewError.value = ''
  } catch (e) {
    preview.value = null
    previewError.value = e instanceof ApiError ? e.message : '试算失败'
  }
}

function chooseAddress() {
  if (!user.hasAddress) {
    uni.showModal({
      title: '还没有地址',
      content: '去「我的」页面添加常用地址',
      confirmText: '去添加',
      success: (r) => {
        if (r.confirm) uni.reLaunch({ url: '/pages/mine/index' })
      },
    })
    return
  }
  const names = user.addresses.map((a) => `${a.community} ${a.lane} ${a.house_no}`)
  uni.showActionSheet({
    itemList: names,
    success: (r) => {
      user.setCurrentAddress(user.addresses[r.tapIndex].id)
      refreshPreview()
    },
  })
}

function onConfirmTime(payload: { date: string; slot: string }) {
  form.serviceDate = payload.date
  form.slotCode = payload.slot
}

async function submit() {
  if (submitting.value) return
  if (!user.currentAddress) {
    uni.showToast({ title: '请先添加地址', icon: 'none' })
    return
  }
  if (!form.slotCode) {
    uni.showToast({ title: '请选择服务时间', icon: 'none' })
    showCalendar.value = true
    return
  }
  if (form.serviceType === 'accompany') {
    if (!form.hospital) {
      uni.showToast({ title: '请选择就诊医院', icon: 'none' })
      return
    }
    if (!form.serviceItems.length) {
      uni.showToast({ title: '请至少选择一项陪诊服务', icon: 'none' })
      return
    }
  }

  const payload = buildPayload()
  if (!payload) return

  submitting.value = true
  try {
    const res = await createOrder(payload)
    await startPay(res.order.order_no)
    uni.redirectTo({ url: `/pages/order/detail?orderNo=${res.order.order_no}` })
  } catch (e) {
    uni.showModal({
      title: '提交失败',
      content: e instanceof ApiError ? e.message : '请重试',
      showCancel: false,
    })
  } finally {
    submitting.value = false
  }
}

/**
 * 发起支付。沙箱渠道无法调起真实微信支付，
 * 开发环境用后端 mock-pay 走完回调；接商户号后换成 uni.requestPayment。
 */
async function startPay(orderNo: string) {
  // #ifdef MP-WEIXIN
  uni.showModal({
    title: '待支付',
    content: '开发环境使用沙箱支付，可在订单详情页完成支付',
    showCancel: false,
  })
  return
  // #endif

  // #ifndef MP-WEIXIN
  await new Promise<void>((resolve, reject) => {
    uni.request({
      url: `${import.meta.env.VITE_API_BASE || ''}/api/v1/dev/orders/${orderNo}/mock-pay`,
      method: 'POST',
      header: { 'X-User-Id': user.userId },
      success: () => resolve(),
      fail: () => reject(new Error('模拟支付失败')),
    })
  })
  // #endif
}
</script>

<template>
  <view class="page">
    <!-- 服务 banner -->
    <view class="banner">
      <view class="banner-copy">
        <view class="banner-title">{{ banner.title }}</view>
        <view class="banner-sub">
          {{ user.currentAddress?.community || '示例小区' }} · 单次服务
        </view>
      </view>
      <view class="banner-art">{{ banner.art }}</view>
    </view>

    <!-- 服务地址：地址 + 联系人与联系方式 -->
    <view class="card" @tap="chooseAddress">
      <view class="f-label">服务地址</view>
      <view class="f-value" :class="{ placeholder: !addressText }">
        <view class="pin" />
        <text class="f-text">{{ addressText || '请选择服务地址' }}</text>
        <text class="chev">›</text>
      </view>
      <view v-if="addressContact" class="addr-contact">
        <text class="contact-dot" />
        <text>{{ addressContact }}</text>
      </view>
    </view>

    <!-- 服务时间：点开日历选择 -->
    <view class="card" @tap="showCalendar = true">
      <view class="f-label">服务时间</view>
      <view class="f-value" :class="{ placeholder: !timeText }">
        <text class="f-text">{{ timeText || '请选择日期与时段' }}</text>
        <text class="chev">›</text>
      </view>
    </view>

    <!-- 服务方案（仅垃圾） -->
    <view v-if="form.serviceType === 'garbage'" class="card">
      <view class="f-label">服务方案</view>
      <view class="pills">
        <view class="pill" :class="{ active: form.plan === 'single' }" @tap="form.plan = 'single'">
          单次
        </view>
        <view class="pill" :class="{ active: form.plan === 'monthly' }" @tap="form.plan = 'monthly'">
          包月
        </view>
      </view>
    </view>

    <!-- 垃圾类型 -->
    <view v-if="form.serviceType === 'garbage'" class="card">
      <view class="f-label">垃圾类型</view>
      <view class="pills">
        <view
          v-for="t in garbageTypes"
          :key="t.value"
          class="pill soft"
          :class="{ active: form.garbageType === t.value }"
          @tap="form.garbageType = t.value"
        >
          {{ t.label }}
        </view>
      </view>
      <view class="f-hint">有害垃圾请按社区要求处理</view>
    </view>

    <!-- 遛宠参数 -->
    <view v-if="form.serviceType === 'dogwalk'" class="card">
      <view class="f-row">
        <view class="f-label">宠物只数</view>
        <view class="stepper">
          <view class="step-btn" @tap="form.petCount = Math.max(1, form.petCount - 1)">−</view>
          <text class="step-val">{{ form.petCount }} 只</text>
          <view class="step-btn" @tap="form.petCount = Math.min(5, form.petCount + 1)">＋</view>
        </view>
      </view>
      <view class="divider" />
      <view class="f-label">服务时长</view>
      <view class="pills">
        <view
          v-for="d in durations"
          :key="d"
          class="pill soft"
          :class="{ active: form.durationMin === d }"
          @tap="form.durationMin = d"
        >
          {{ d }} 分钟
        </view>
      </view>
      <view class="divider" />
      <view class="f-row">
        <view class="f-label">仅喂食清洁（不遛）</view>
        <switch class="sw" :checked="form.feedOnly" color="#0f7f78" @change="form.feedOnly = !form.feedOnly" />
      </view>
      <view class="f-row">
        <view class="f-label">需要提前上门熟悉</view>
        <switch
          class="sw"
          :checked="form.needFamiliar"
          color="#0f7f78"
          @change="form.needFamiliar = !form.needFamiliar"
        />
      </view>
      <view class="f-hint">累计完成 5 单后免熟悉费</view>
    </view>

    <!-- 陪诊信息：医院只能从后台选项选，服务项可多选 -->
    <view v-if="form.serviceType === 'accompany'" class="card">
      <view class="f-row">
        <view class="f-label">就诊人</view>
        <input v-model="form.patientName" class="f-input" placeholder="如：张阿姨" />
      </view>
      <view class="divider" />
      <view class="f-value" :class="{ placeholder: !hospitalName }" @tap="chooseHospital">
        <view class="f-label">就诊医院</view>
        <text class="f-text">{{ hospitalName || '请选择就诊医院' }}</text>
        <text class="chev">›</text>
      </view>
      <view class="divider" />
      <view class="f-label">陪诊服务 <text class="f-label-sub">（可多选）</text></view>
      <view class="service-grid">
        <view
          v-for="it in serviceItemOptions"
          :key="it.code"
          class="service-chip"
          :class="{ active: form.serviceItems.includes(it.code) }"
          @tap="toggleServiceItem(it.code)"
        >
          <view class="service-chip-name">
            <text class="check-box">{{ form.serviceItems.includes(it.code) ? '✓' : '' }}</text>
            <text>{{ it.name }}</text>
          </view>
          <text class="service-chip-price">{{ price(it.price) }}</text>
        </view>
      </view>
      <view class="f-hint">按所选项目计价，医院与服务项均由社区统一配置</view>
    </view>

    <!-- 备注 -->
    <view class="card">
      <view class="f-label">备注</view>
      <input
        v-model="form.remark"
        class="remark-input"
        maxlength="200"
        placeholder="例如：请提前联系 / 门禁密码"
      />
    </view>

    <!-- 费用估算 -->
    <view class="fee-card">
      <view class="fee-row">
        <text class="fee-label">费用估算</text>
        <text class="fee-amount">
          {{ preview ? price(preview.total) : '--' }}
        </text>
      </view>
      <view class="fee-note">按当前所选项目计算，最终以订单为准</view>
      <view v-if="preview?.items?.length" class="fee-items">
        <view v-for="(it, i) in preview.items" :key="i" class="fee-item">
          <text>{{ it.name }}<text v-if="it.note" class="fee-item-note">（{{ it.note }}）</text></text>
          <text>{{ it.amount === 0 ? '免费' : price(it.amount) }}</text>
        </view>
      </view>
      <view v-if="previewError" class="fee-error">{{ previewError }}</view>
    </view>

    <!-- 提交 -->
    <view class="submit-wrap">
      <view class="btn-primary" :class="{ 'is-disabled': submitting }" @tap="submit">
        {{ submitting ? '提交中…' : '预约服务' }}
      </view>
      <view class="foot-note">提交后可在「订单」中查看服务进度</view>
    </view>

    <CalendarPicker
      v-model:visible="showCalendar"
      :date="form.serviceDate"
      :slot="form.slotCode"
      :service-type="form.serviceType"
      @confirm="onConfirmTime"
    />
  </view>
</template>

<style scoped>
.page {
  padding-bottom: 60rpx;
}

/* ---------- banner ---------- */
.banner {
  position: relative;
  margin: 20rpx 24rpx;
  height: 196rpx;
  border-radius: 22rpx;
  background: linear-gradient(135deg, #e9f5f3 0%, #e4f3f0 100%);
  overflow: hidden;
  display: flex;
  align-items: center;
  padding: 0 34rpx;
}

.banner-copy {
  position: relative;
  z-index: 2;
}

.banner-title {
  font-size: 40rpx;
  font-weight: 800;
  color: #14413c;
  letter-spacing: 0.5rpx;
}

.banner-sub {
  margin-top: 10rpx;
  font-size: 23rpx;
  color: #5c7b76;
}

.banner-art {
  position: absolute;
  right: 24rpx;
  bottom: -10rpx;
  font-size: 132rpx;
  line-height: 1;
  opacity: 0.9;
}

/* ---------- 卡片 ---------- */
.card {
  background: #fff;
  border-radius: 18rpx;
  margin: 0 24rpx 18rpx;
  padding: 26rpx 30rpx;
}

.f-label {
  font-size: 28rpx;
  font-weight: 600;
  color: var(--text);
}

.f-value {
  margin-top: 16rpx;
  display: flex;
  align-items: center;
  gap: 10rpx;
}

.f-value.placeholder .f-text {
  color: var(--text-light);
}

.f-text {
  flex: 1;
  font-size: 26rpx;
  color: var(--text);
}

.chev {
  color: var(--text-light);
  font-size: 26rpx;
}

/* 地址卡上的联系人与联系方式 */
.addr-contact {
  margin-top: 14rpx;
  display: flex;
  align-items: center;
  gap: 10rpx;
  font-size: 23rpx;
  color: var(--brand-dark);
}

.contact-dot {
  width: 8rpx;
  height: 8rpx;
  border-radius: 50%;
  background: var(--brand);
  flex-shrink: 0;
}

.f-label-sub {
  font-size: 22rpx;
  font-weight: 400;
  color: var(--text-light);
}

/* 陪诊服务项多选 */
.service-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14rpx;
  margin: 16rpx 0 4rpx;
}

.service-chip {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8rpx;
  padding: 18rpx 18rpx;
  border: 1rpx solid #e3e8e6;
  border-radius: 12rpx;
  font-size: 24rpx;
  color: var(--text-sub);
}

.service-chip.active {
  border-color: var(--brand);
  background: var(--brand-light);
  color: var(--brand-dark);
}

.service-chip-name {
  display: flex;
  align-items: center;
  gap: 10rpx;
  min-width: 0;
}

.check-box {
  width: 28rpx;
  height: 28rpx;
  border: 1rpx solid #cbd5d2;
  border-radius: 6rpx;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 20rpx;
  color: #fff;
}

.service-chip.active .check-box {
  background: var(--brand);
  border-color: var(--brand);
}

.service-chip-price {
  font-size: 23rpx;
  color: var(--accent);
  flex-shrink: 0;
}

.pin {
  width: 14rpx;
  height: 14rpx;
  border-radius: 50% 50% 50% 0;
  transform: rotate(-45deg);
  background: var(--brand);
  flex-shrink: 0;
}

.f-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20rpx;
  /* 一个卡片里放多个字段时，没有上下留白会挤成一片 */
  padding: 12rpx 0;
}

.f-hint {
  margin-top: 14rpx;
  font-size: 22rpx;
  color: var(--text-light);
  line-height: 1.6;
}

.divider {
  height: 1rpx;
  background: #f2f4f3;
  margin: 22rpx 0;
}

.f-input {
  flex: 1;
  text-align: right;
  font-size: 26rpx;
}

/* 不设 flex-shrink:0 时，switch 在 flex 容器里会被压扁 */
.sw {
  flex-shrink: 0;
  /* uni-app 的 switch 默认比其他控件小一圈，放大到与标签视觉等重 */
  transform: scale(1.15);
  transform-origin: right center;
}

.remark-input {
  margin-top: 16rpx;
  height: 76rpx;
  border: 1rpx solid #e8ecea;
  border-radius: 12rpx;
  padding: 0 22rpx;
  font-size: 25rpx;
}

/* ---------- 胶囊 ---------- */
.pills {
  display: flex;
  flex-wrap: wrap;
  gap: 14rpx;
  margin: 16rpx 0 4rpx;
}

.pill {
  padding: 14rpx 40rpx;
  border-radius: 34rpx;
  border: 1rpx solid #e3e8e6;
  background: #fff;
  font-size: 26rpx;
  color: var(--text-sub);
}

.pill.active {
  background: var(--brand);
  border-color: var(--brand);
  color: #fff;
  font-weight: 500;
}

/* 垃圾类型用浅底选中，与设计稿一致 */
.pill.soft {
  padding: 12rpx 28rpx;
  border-radius: 30rpx;
  font-size: 24rpx;
}

.pill.soft.active {
  background: #e4f3f0;
  border-color: #e4f3f0;
  color: var(--brand-dark);
}

/* ---------- 步进器 ---------- */
.stepper {
  display: flex;
  align-items: center;
  gap: 26rpx;
}

.step-btn {
  width: 52rpx;
  height: 52rpx;
  border-radius: 50%;
  border: 1rpx solid #e3e8e6;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 30rpx;
  color: var(--text);
  background: #fff;
}

.step-val {
  font-size: 27rpx;
  color: var(--text);
  min-width: 90rpx;
  text-align: center;
}

/* ---------- 费用估算 ---------- */
.fee-card {
  margin: 22rpx 24rpx 0;
  padding: 26rpx 30rpx;
  background: var(--accent-soft);
  border-radius: 18rpx;
}

.fee-row {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
}

.fee-label {
  font-size: 27rpx;
  font-weight: 600;
  color: #8a5a12;
}

.fee-amount {
  font-size: 40rpx;
  font-weight: 700;
  color: var(--accent);
}

.fee-note {
  margin-top: 8rpx;
  font-size: 22rpx;
  color: #b08a52;
}

.fee-items {
  margin-top: 16rpx;
  padding-top: 14rpx;
  border-top: 1rpx dashed #ecd9b8;
}

.fee-item {
  display: flex;
  justify-content: space-between;
  font-size: 22rpx;
  color: #9a7742;
  padding: 6rpx 0;
}

.fee-item-note {
  color: #b9a184;
}

.fee-error {
  margin-top: 10rpx;
  font-size: 22rpx;
  color: var(--danger);
}

/* ---------- 提交 ---------- */
.submit-wrap {
  padding: 30rpx 24rpx 0;
}

.foot-note {
  margin-top: 18rpx;
  text-align: center;
  font-size: 21rpx;
  color: var(--text-light);
}
</style>
