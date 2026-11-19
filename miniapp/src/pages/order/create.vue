<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { onLoad } from '@dcloudio/uni-app'
import { createOrder, previewOrder, type CreateOrderPayload, type PreviewResp } from '@/api'
import { useUserStore } from '@/store/user'
import { availableSlots, nextDays, price, SERVICE_LABEL, SLOTS } from '@/utils/format'
import { ApiError } from '@/utils/request'

const user = useUserStore()

const serviceTypes = [
  { type: 'garbage', name: '代倒垃圾' },
  { type: 'dogwalk', name: '遛宠' },
  { type: 'accompany', name: '陪诊' },
]

const dates = nextDays(7)

const form = reactive({
  serviceType: 'garbage',
  serviceDate: dates[0].value,
  slotCode: '',
  quantity: 1,
  garbageType: 'kitchen',
  petCount: 1,
  durationMin: 20,
  feedOnly: false,
  needFamiliar: false,
  // 陪诊无结构化字段，先收在这三个输入里，提交时拼进 remark
  patientName: '',
  hospital: '',
  department: '',
  remark: '',
})

const preview = ref<PreviewResp | null>(null)
const previewError = ref('')
const submitting = ref(false)

const slots = computed(() => {
  if (form.serviceType === 'dogwalk') return availableSlots(form.serviceDate)
  return SLOTS
})

const addressText = computed(() => {
  const a = user.currentAddress
  if (!a) return '请添加地址'
  return `${a.community} ${a.lane} ${a.house_no}`
})

const garbageTypes = [
  { value: 'kitchen', label: '厨余垃圾' },
  { value: 'other', label: '其他垃圾' },
  { value: 'recyclable', label: '可回收物' },
]

const durations = [20, 30, 40, 60]

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
  syncSlot()
  refreshPreview()
})

/** 切换服务后时段可能变化，需要纠正当前选择 */
function syncSlot() {
  const list = slots.value
  if (!list.some((s) => s.code === form.slotCode)) {
    form.slotCode = list.length ? list[0].code : ''
  }
}

watch(
  () => form.serviceDate,
  () => {
    syncSlot()
  },
)

function switchService(type: string) {
  form.serviceType = type
  syncSlot()
  refreshPreview()
}

function pickDate(v: string) {
  form.serviceDate = v
}

// 任一影响价格的字段变化都重新试算，节流 300ms 避免连续点击打爆接口
let timer: ReturnType<typeof setTimeout> | null = null
watch(
  () => [
    form.serviceType,
    form.serviceDate,
    form.slotCode,
    form.quantity,
    form.petCount,
    form.durationMin,
    form.feedOnly,
    form.needFamiliar,
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
    payload.quantity = form.quantity
    payload.garbage_type = form.garbageType
  } else if (form.serviceType === 'dogwalk') {
    payload.pet_count = form.petCount
    payload.duration_min = form.durationMin
    payload.feed_only = form.feedOnly
    payload.need_familiar = form.needFamiliar
  } else {
    // 陪诊信息后端暂无结构化字段，拼进备注，由运营报价时读取
    const parts = [
      form.patientName && `就诊人：${form.patientName}`,
      form.hospital && `医院：${form.hospital}`,
      form.department && `科室：${form.department}`,
      form.remark,
    ].filter(Boolean)
    payload.remark = parts.join('；').slice(0, 200)
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

async function submit() {
  if (submitting.value) return
  if (!user.currentAddress) {
    uni.showToast({ title: '请先添加地址', icon: 'none' })
    return
  }
  if (!form.slotCode) {
    uni.showToast({ title: '请选择时段', icon: 'none' })
    return
  }
  if (form.serviceType === 'accompany' && !form.hospital) {
    uni.showToast({ title: '请填写就诊医院', icon: 'none' })
    return
  }

  const payload = buildPayload()
  if (!payload) return

  submitting.value = true
  try {
    const res = await createOrder(payload)

    if (res.manual_quote) {
      uni.showModal({
        title: '已提交，等待报价',
        content: '陪诊按小时人工报价，运营确认后会通知你支付',
        showCancel: false,
        success: () => {
          uni.redirectTo({ url: `/pages/order/detail?orderNo=${res.order.order_no}` })
        },
      })
      return
    }

    await startPay(res.order.order_no)
    uni.redirectTo({ url: `/pages/order/detail?orderNo=${res.order.order_no}` })
  } catch (e) {
    const msg = e instanceof ApiError ? e.message : '提交失败，请重试'
    uni.showModal({ title: '提交失败', content: msg, showCancel: false })
  } finally {
    submitting.value = false
  }
}

/**
 * 发起支付。
 * 沙箱渠道无法调起真实微信支付，开发环境用后端 mock-pay 接口走完回调链路；
 * 接入真实商户号后换成 uni.requestPayment(params)。
 */
async function startPay(orderNo: string) {
  // #ifdef MP-WEIXIN
  uni.showModal({
    title: '待支付',
    content: '开发环境使用沙箱支付，订单详情页可模拟支付成功',
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
    <!-- 服务类型 -->
    <view class="service-tabs">
      <view
        v-for="s in serviceTypes"
        :key="s.type"
        class="tab"
        :class="{ active: form.serviceType === s.type }"
        @tap="switchService(s.type)"
      >
        {{ s.name }}
      </view>
    </view>

    <!-- 地址 -->
    <view class="card">
      <view class="field" @tap="chooseAddress">
        <text class="label">服务地址</text>
        <view class="value-wrap">
          <text class="value">{{ addressText }}</text>
          <text class="chev">›</text>
        </view>
      </view>
      <view v-if="!user.hasAddress" class="hint">请先在「我的」页面添加常用地址</view>
    </view>

    <!-- 时间 -->
    <view class="card">
      <view class="section-title">预约时间</view>
      <scroll-view scroll-x class="dates" :show-scrollbar="false">
        <view
          v-for="d in dates"
          :key="d.value"
          class="date-chip"
          :class="{ active: form.serviceDate === d.value }"
          @tap="pickDate(d.value)"
        >
          {{ d.label }}
        </view>
      </scroll-view>
      <view class="slots">
        <view
          v-for="s in slots"
          :key="s.code"
          class="slot"
          :class="{ active: form.slotCode === s.code }"
          @tap="form.slotCode = s.code"
        >
          {{ s.label }}
        </view>
      </view>
      <view v-if="form.serviceType === 'dogwalk'" class="hint">
        遛宠工作日仅开放晚间时段，周末全天可约
      </view>
    </view>

    <!-- 服务明细 -->
    <view class="card">
      <view class="section-title">{{ SERVICE_LABEL[form.serviceType] }}信息</view>

      <template v-if="form.serviceType === 'garbage'">
        <view class="field">
          <text class="label">垃圾袋数</text>
          <view class="stepper">
            <view class="step-btn" @tap="form.quantity = Math.max(1, form.quantity - 1)">−</view>
            <text class="step-val">{{ form.quantity }}</text>
            <view class="step-btn" @tap="form.quantity = Math.min(20, form.quantity + 1)">＋</view>
          </view>
        </view>
        <view class="field">
          <text class="label">垃圾类型</text>
          <view class="chips">
            <view
              v-for="t in garbageTypes"
              :key="t.value"
              class="chip"
              :class="{ active: form.garbageType === t.value }"
              @tap="form.garbageType = t.value"
            >
              {{ t.label }}
            </view>
          </view>
        </view>
      </template>

      <template v-else-if="form.serviceType === 'dogwalk'">
        <view class="field">
          <text class="label">宠物只数</text>
          <view class="stepper">
            <view class="step-btn" @tap="form.petCount = Math.max(1, form.petCount - 1)">−</view>
            <text class="step-val">{{ form.petCount }}</text>
            <view class="step-btn" @tap="form.petCount = Math.min(5, form.petCount + 1)">＋</view>
          </view>
        </view>
        <view class="field">
          <text class="label">服务时长</text>
          <view class="chips">
            <view
              v-for="d in durations"
              :key="d"
              class="chip"
              :class="{ active: form.durationMin === d }"
              @tap="form.durationMin = d"
            >
              {{ d }} 分钟
            </view>
          </view>
        </view>
        <view class="field">
          <text class="label">仅喂食清洁（不遛）</text>
          <switch :checked="form.feedOnly" color="#16a34a" @change="form.feedOnly = !form.feedOnly" />
        </view>
        <view class="field">
          <text class="label">需要提前上门熟悉</text>
          <switch :checked="form.needFamiliar" color="#16a34a" @change="form.needFamiliar = !form.needFamiliar" />
        </view>
        <view class="hint">累计完成 5 单后免熟悉费</view>
      </template>

      <template v-else>
        <view class="field">
          <text class="label">就诊人</text>
          <input v-model="form.patientName" class="input" placeholder="如：张阿姨" />
        </view>
        <view class="field">
          <text class="label">就诊医院</text>
          <input v-model="form.hospital" class="input" placeholder="如：仁济医院东院" />
        </view>
        <view class="field">
          <text class="label">科室</text>
          <input v-model="form.department" class="input" placeholder="如：心内科" />
        </view>
        <view class="hint">陪诊按小时人工报价，提交后运营会联系确认</view>
      </template>
    </view>

    <!-- 备注 -->
    <view class="card">
      <view class="section-title">补充说明</view>
      <textarea
        v-model="form.remark"
        class="textarea"
        maxlength="200"
        placeholder="如门禁密码、宠物习性、特殊要求（选填）"
      />
    </view>

    <!-- 费用明细 -->
    <view v-if="preview" class="card">
      <view class="section-title">费用明细</view>
      <view v-for="(item, i) in preview.items" :key="i" class="price-row">
        <view class="price-name">
          {{ item.name }}
          <text v-if="item.note" class="price-note">{{ item.note }}</text>
        </view>
        <text class="price-amount">{{ item.amount === 0 ? '免费' : price(item.amount) }}</text>
      </view>
      <view class="divider" />
      <view class="price-row total">
        <text>合计</text>
        <text class="total-amount">{{ preview.manual_quote ? '待报价' : price(preview.total) }}</text>
      </view>
    </view>

    <view v-else-if="previewError" class="card error-tip">{{ previewError }}</view>

    <!-- 底部提交 -->
    <view class="bottom-bar">
      <view class="sum">
        <text class="sum-label">合计</text>
        <text class="sum-amount">{{ preview ? (preview.manual_quote ? '待报价' : price(preview.total)) : '--' }}</text>
      </view>
      <view class="submit" :class="{ disabled: submitting }" @tap="submit">
        {{ submitting ? '提交中…' : '提交预约' }}
      </view>
    </view>
  </view>
</template>

<style scoped>
.page {
  padding: 24rpx 0 200rpx;
}

.service-tabs {
  display: flex;
  gap: 16rpx;
  padding: 0 24rpx 24rpx;
}

.tab {
  flex: 1;
  text-align: center;
  padding: 20rpx 0;
  background: #fff;
  border-radius: 16rpx;
  font-size: 27rpx;
  color: var(--text-sub);
  border: 1rpx solid transparent;
}

.tab.active {
  color: var(--brand-dark);
  background: var(--brand-light);
  border-color: var(--brand);
  font-weight: 600;
}

.field {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 20rpx 0;
  border-bottom: 1rpx solid #f3f4f6;
  gap: 20rpx;
}

.field:last-child {
  border-bottom: none;
}

.label {
  font-size: 27rpx;
  color: var(--text);
  flex-shrink: 0;
}

.value-wrap {
  display: flex;
  align-items: center;
  gap: 8rpx;
  max-width: 70%;
}

.value {
  font-size: 27rpx;
  color: var(--text-sub);
  text-align: right;
}

.chev {
  color: var(--text-light);
}

.hint {
  font-size: 23rpx;
  color: var(--text-light);
  margin-top: 12rpx;
  line-height: 1.6;
}

.dates {
  white-space: nowrap;
  margin-bottom: 20rpx;
}

.date-chip {
  display: inline-block;
  padding: 14rpx 28rpx;
  margin-right: 16rpx;
  background: #f3f4f6;
  border-radius: 32rpx;
  font-size: 26rpx;
  color: var(--text-sub);
}

.date-chip.active {
  background: var(--brand);
  color: #fff;
  font-weight: 500;
}

.slots {
  display: flex;
  gap: 16rpx;
}

.slot {
  flex: 1;
  text-align: center;
  padding: 18rpx 0;
  border: 1rpx solid var(--border);
  border-radius: 16rpx;
  font-size: 25rpx;
  color: var(--text-sub);
}

.slot.active {
  border-color: var(--brand);
  color: var(--brand-dark);
  background: var(--brand-light);
  font-weight: 500;
}

.stepper {
  display: flex;
  align-items: center;
  gap: 24rpx;
}

.step-btn {
  width: 56rpx;
  height: 56rpx;
  border-radius: 50%;
  background: #f3f4f6;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 32rpx;
  color: var(--text);
}

.step-val {
  font-size: 30rpx;
  font-weight: 600;
  min-width: 48rpx;
  text-align: center;
}

.chips {
  display: flex;
  gap: 12rpx;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.chip {
  padding: 10rpx 16rpx;
  border-radius: 24rpx;
  background: #f3f4f6;
  font-size: 22rpx;
  color: var(--text-sub);
  white-space: nowrap;
}

.chip.active {
  background: var(--brand-light);
  color: var(--brand-dark);
  font-weight: 500;
}

.input {
  flex: 1;
  text-align: right;
  font-size: 27rpx;
}

.textarea {
  width: 100%;
  height: 140rpx;
  font-size: 27rpx;
  line-height: 1.6;
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

.price-amount {
  color: var(--text);
}

.divider {
  height: 1rpx;
  background: #f3f4f6;
  margin: 12rpx 0;
}

.total {
  align-items: center;
  font-size: 28rpx;
  color: var(--text);
}

.total-amount {
  font-size: 34rpx;
  font-weight: 700;
  color: #ef4444;
}

.error-tip {
  color: var(--danger);
  font-size: 26rpx;
}

.bottom-bar {
  position: fixed;
  left: 0;
  right: 0;
  bottom: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 20rpx 24rpx;
  padding-bottom: calc(20rpx + env(safe-area-inset-bottom));
  background: #fff;
  border-top: 1rpx solid var(--border);
}

.sum {
  display: flex;
  align-items: baseline;
  gap: 10rpx;
}

.sum-label {
  font-size: 24rpx;
  color: var(--text-sub);
}

.sum-amount {
  font-size: 38rpx;
  font-weight: 700;
  color: #ef4444;
}

.submit {
  background: var(--brand);
  color: #fff;
  padding: 0 64rpx;
  height: 84rpx;
  line-height: 84rpx;
  border-radius: 42rpx;
  font-size: 30rpx;
  font-weight: 500;
}

.submit.disabled {
  background: #d1d5db;
}
</style>
