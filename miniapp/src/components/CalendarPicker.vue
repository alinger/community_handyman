<script setup lang="ts">
/**
 * 日历选择弹层：月视图 + 时段，一次性选完「哪天 + 哪个时段」。
 *
 * 可选范围与后端库存对齐（今天起 7 天）：库存表只灌了 7 天，
 * 日历上也只放这 7 天可选，避免用户选了约不上的日期。
 * 遛宠在工作日只开晚间，时段列表随日期动态变化。
 */
import { computed, ref, watch } from 'vue'
import { slotAvailability, type SlotAvailability } from '@/api'
import { SLOTS } from '@/utils/format'

const props = defineProps<{
  visible: boolean
  date: string
  slot: string
  serviceType: string
}>()

const emit = defineEmits<{
  (e: 'update:visible', v: boolean): void
  (e: 'confirm', payload: { date: string; slot: string }): void
}>()

const WEEK_LABELS = ['一', '二', '三', '四', '五', '六', '日']
/** 与后端 slot_inventories 的灌数范围保持一致 */
const BOOKABLE_DAYS = 7

function pad(n: number): string {
  return n < 10 ? '0' + n : String(n)
}

function toStr(y: number, m: number, d: number): string {
  return `${y}-${pad(m + 1)}-${pad(d)}`
}

const today = new Date()
const minDate = toStr(today.getFullYear(), today.getMonth(), today.getDate())
const maxDay = new Date(today.getFullYear(), today.getMonth(), today.getDate() + BOOKABLE_DAYS - 1)
const maxDate = toStr(maxDay.getFullYear(), maxDay.getMonth(), maxDay.getDate())

// 视图所在月份
const viewYear = ref(today.getFullYear())
const viewMonth = ref(today.getMonth())

// 弹层内的临时选择，点「确定」才回传
const pickedDate = ref(props.date || minDate)
const pickedSlot = ref(props.slot || '')

watch(
  () => props.visible,
  (v) => {
    if (!v) return
    pickedDate.value = props.date || minDate
    pickedSlot.value = props.slot || ''
    // 打开时把视图定位到已选日期所在月
    const d = new Date(pickedDate.value.replace(/-/g, '/'))
    if (!Number.isNaN(d.getTime())) {
      viewYear.value = d.getFullYear()
      viewMonth.value = d.getMonth()
    }
  },
)

/** 当月网格：前面按周一补空格，再排天数 */
const cells = computed(() => {
  const first = new Date(viewYear.value, viewMonth.value, 1)
  // getDay: 0=周日，转成周一起算的偏移
  const offset = (first.getDay() + 6) % 7
  const daysInMonth = new Date(viewYear.value, viewMonth.value + 1, 0).getDate()

  const out: (number | null)[] = []
  for (let i = 0; i < offset; i++) out.push(null)
  for (let d = 1; d <= daysInMonth; d++) out.push(d)
  return out
})

const monthTitle = computed(() => `${viewYear.value} 年 ${viewMonth.value + 1} 月`)

function dateOf(day: number): string {
  return toStr(viewYear.value, viewMonth.value, day)
}

function selectable(day: number): boolean {
  const s = dateOf(day)
  return s >= minDate && s <= maxDate
}

function isPicked(day: number): boolean {
  return dateOf(day) === pickedDate.value
}

function isToday(day: number): boolean {
  return dateOf(day) === minDate
}

const canPrev = computed(() => {
  const cur = viewYear.value * 12 + viewMonth.value
  const min = today.getFullYear() * 12 + today.getMonth()
  return cur > min
})

const canNext = computed(() => {
  const cur = viewYear.value * 12 + viewMonth.value
  const max = maxDay.getFullYear() * 12 + maxDay.getMonth()
  return cur < max
})

function prevMonth() {
  if (!canPrev.value) return
  if (viewMonth.value === 0) {
    viewMonth.value = 11
    viewYear.value -= 1
  } else {
    viewMonth.value -= 1
  }
}

function nextMonth() {
  if (!canNext.value) return
  if (viewMonth.value === 11) {
    viewMonth.value = 0
    viewYear.value += 1
  } else {
    viewMonth.value += 1
  }
}

function pickDay(day: number) {
  if (!selectable(day)) return
  pickedDate.value = dateOf(day)
}

/** 当前选中日可约的时段 */
const daySlots = computed(() => {
  if (props.serviceType !== 'dogwalk') return SLOTS
  const d = new Date(pickedDate.value.replace(/-/g, '/'))
  const weekday = d.getDay() >= 1 && d.getDay() <= 5
  return weekday ? SLOTS.filter((s) => s.code === '18-20') : SLOTS
})

/**
 * 时段余量：只查当前选中日，满员时段置灰。
 * 后端未配该日库存时返回空数组，此情况下按全部可约处理（与后端下单兜底一致）。
 */
const avail = ref<Record<string, SlotAvailability>>({})

async function loadAvail() {
  const date = pickedDate.value
  if (!date) return
  try {
    const list = await slotAvailability(date, props.serviceType)
    const map: Record<string, SlotAvailability> = {}
    for (const s of list) map[s.slot_code] = s
    // 请求回来时用户可能已切到别的日期，丢弃过期结果
    if (date === pickedDate.value) avail.value = map
  } catch (e) {
    if (date === pickedDate.value) avail.value = {}
  }
}

watch(
  () => [props.visible, pickedDate.value, props.serviceType],
  () => {
    if (props.visible) loadAvail()
  },
  { immediate: true },
)

function slotFull(code: string): boolean {
  const s = avail.value[code]
  return s ? !s.available : false
}

function slotTip(code: string): string {
  const s = avail.value[code]
  if (!s) return ''
  if (!s.available) return '已满'
  if (s.remaining <= 2) return `余${s.remaining}`
  return ''
}

// 换日期后原时段可能不可约或已满，自动落到第一个既开放又有余量的时段
watch([daySlots, avail], () => {
  const usable = daySlots.value.filter((s) => !slotFull(s.code))
  if (!usable.some((s) => s.code === pickedSlot.value)) {
    pickedSlot.value = usable[0]?.code || ''
  }
})

function close() {
  emit('update:visible', false)
}

function confirm() {
  if (!pickedDate.value || !pickedSlot.value) {
    uni.showToast({ title: '请选择日期与时段', icon: 'none' })
    return
  }
  emit('confirm', { date: pickedDate.value, slot: pickedSlot.value })
  emit('update:visible', false)
}
</script>

<template>
  <view v-if="visible" class="mask" @tap="close">
    <view class="sheet" @tap.stop>
      <view class="sheet-head">
        <text class="sheet-title">选择服务时间</text>
        <view class="close" @tap="close">✕</view>
      </view>

      <!-- 月份切换 -->
      <view class="month-bar">
        <view class="month-btn" :class="{ disabled: !canPrev }" @tap="prevMonth">‹</view>
        <text class="month-title">{{ monthTitle }}</text>
        <view class="month-btn" :class="{ disabled: !canNext }" @tap="nextMonth">›</view>
      </view>

      <!-- 星期表头 -->
      <view class="week-row">
        <text v-for="w in WEEK_LABELS" :key="w" class="week-cell">{{ w }}</text>
      </view>

      <!-- 日期网格 -->
      <view class="day-grid">
        <view
          v-for="(d, i) in cells"
          :key="i"
          class="day-cell"
          :class="{
            empty: !d,
            picked: d && isPicked(d),
            disabled: d && !selectable(d),
            today: d && isToday(d),
          }"
          @tap="d && pickDay(d)"
        >
          <text v-if="d" class="day-num">{{ d }}</text>
          <view v-if="d && isToday(d)" class="today-dot" />
        </view>
      </view>

      <view class="divider" />

      <!-- 时段 -->
      <view class="slot-title">可选时段</view>
      <view class="slot-grid">
        <view
          v-for="s in daySlots"
          :key="s.code"
          class="slot-chip"
          :class="{ active: pickedSlot === s.code, full: slotFull(s.code) }"
          @tap="!slotFull(s.code) && (pickedSlot = s.code)"
        >
          <text>{{ s.label }}</text>
          <text v-if="slotTip(s.code)" class="chip-tip">{{ slotTip(s.code) }}</text>
        </view>
      </view>
      <view v-if="serviceType === 'dogwalk'" class="slot-hint">
        遛宠工作日仅开放晚间时段，周末全天可约
      </view>
      <view v-else-if="daySlots.every((s) => slotFull(s.code))" class="slot-hint">
        当日时段已约满，请换一天
      </view>

      <view class="btn-primary confirm" @tap="confirm">确定</view>
    </view>
  </view>
</template>

<style scoped>
.mask {
  position: fixed;
  inset: 0;
  background: rgba(15, 30, 28, 0.42);
  z-index: 300;
  display: flex;
  align-items: flex-end;
}

.sheet {
  width: 100%;
  background: #fff;
  border-radius: 28rpx 28rpx 0 0;
  padding: 32rpx 28rpx;
  padding-bottom: calc(32rpx + env(safe-area-inset-bottom));
  animation: slide-up 0.22s ease-out;
}

@keyframes slide-up {
  from {
    transform: translateY(100%);
  }
  to {
    transform: translateY(0);
  }
}

.sheet-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12rpx;
}

.sheet-title {
  font-size: 32rpx;
  font-weight: 700;
  color: var(--text);
}

.close {
  width: 56rpx;
  height: 56rpx;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-light);
  font-size: 28rpx;
}

/* ---- 月份 ---- */
.month-bar {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 40rpx;
  padding: 16rpx 0 20rpx;
}

.month-title {
  font-size: 28rpx;
  font-weight: 600;
  color: var(--text);
  min-width: 200rpx;
  text-align: center;
}

.month-btn {
  width: 56rpx;
  height: 56rpx;
  border-radius: 50%;
  background: #f4f6f5;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 32rpx;
  color: var(--text);
}

.month-btn.disabled {
  color: #d5dbd9;
  background: #fafbfa;
}

/* ---- 日历 ---- */
.week-row {
  display: grid;
  grid-template-columns: repeat(7, 1fr);
  padding-bottom: 8rpx;
}

.week-cell {
  text-align: center;
  font-size: 22rpx;
  color: var(--text-light);
}

.day-grid {
  display: grid;
  grid-template-columns: repeat(7, 1fr);
  gap: 4rpx 0;
}

.day-cell {
  height: 82rpx;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  position: relative;
}

.day-num {
  width: 62rpx;
  height: 62rpx;
  line-height: 62rpx;
  text-align: center;
  border-radius: 50%;
  font-size: 27rpx;
  color: var(--text);
}

.day-cell.disabled .day-num {
  color: #d3d9d7;
}

.day-cell.picked .day-num {
  background: var(--brand);
  color: #fff;
  font-weight: 600;
}

.today-dot {
  position: absolute;
  bottom: 8rpx;
  width: 8rpx;
  height: 8rpx;
  border-radius: 50%;
  background: var(--brand);
}

.day-cell.picked .today-dot {
  background: transparent;
}

.divider {
  height: 1rpx;
  background: #f2f4f3;
  margin: 20rpx 0;
}

/* ---- 时段 ---- */
.slot-title {
  font-size: 26rpx;
  font-weight: 600;
  color: var(--text);
  margin-bottom: 16rpx;
}

.slot-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 14rpx;
}

.slot-chip {
  text-align: center;
  padding: 20rpx 0;
  border: 1rpx solid #e3e8e6;
  border-radius: 12rpx;
  font-size: 24rpx;
  color: var(--text-sub);
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4rpx;
}

.slot-chip.active {
  border-color: var(--brand);
  background: var(--brand-light);
  color: var(--brand-dark);
  font-weight: 600;
}

/* 满员：置灰不可点，划掉价格区避免误导 */
.slot-chip.full {
  background: #f6f7f8;
  border-color: #eceff1;
  color: #b6bec6;
}

.slot-chip.full text:first-child {
  text-decoration: line-through;
}

.chip-tip {
  font-size: 18rpx;
  color: #b6bec6;
}

.slot-chip.active .chip-tip {
  color: var(--brand-dark);
}

.slot-hint {
  margin-top: 14rpx;
  font-size: 21rpx;
  color: var(--text-light);
}

.confirm {
  margin-top: 28rpx;
}
</style>
