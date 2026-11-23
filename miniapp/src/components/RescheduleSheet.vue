<script setup lang="ts">
/**
 * 改期弹层。住户「直接改」与服务方「申请改期」共用一份，靠 mode 切换文案与提交语义。
 *
 * 为什么不用 CalendarPicker：它是月视图，而改期只有未来 7 天可约（库存表也只灌 7 天），
 * 且这里还要顺带填原因。日期条 + 时段芯片一屏选完，少一次弹层切换。
 */
import { computed, ref, watch } from 'vue'
import { slotAvailability, type SlotAvailability } from '@/api'
import { friendlyDate, SLOTS, slotTextOf } from '@/utils/format'

const props = defineProps<{
  visible: boolean
  /** 提交中：禁用按钮，防止连点重复改期 */
  loading?: boolean
  /** user 住户直接改（立即生效）/ provider 服务方发起申请（需住户确认） */
  mode: 'user' | 'provider'
  serviceType: string
  fromDate: string
  fromSlot: string
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'confirm', payload: { date: string; slot: string; reason: string }): void
}>()

/** 常见改期原因，省去打字；仍可在下方补充 */
const QUICK = ['临时有事', '天气原因', '时间冲突', '宠物状况有变']

/** 与后端库存灌数范围保持一致 */
const BOOKABLE_DAYS = 7

const pickedDate = ref('')
const pickedSlot = ref('')
const picked = ref('')
const detail = ref('')
const avail = ref<Record<string, SlotAvailability>>({})
/** 用户是否手动点过日期：手动选的日子即使全约满也停在那，不要把选择权抢回来 */
const userPickedDate = ref(false)

function pad(n: number): string {
  return n < 10 ? '0' + n : String(n)
}

function fmt(d: Date): string {
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

const WEEK = ['周日', '周一', '周二', '周三', '周四', '周五', '周六']

/** 未来 7 天的日期条，top 是「今天/明天/周X」，bottom 是「11/22」 */
const dates = computed(() => {
  const now = new Date()
  const out: { value: string; top: string; bottom: string }[] = []
  for (let i = 0; i < BOOKABLE_DAYS; i++) {
    const d = new Date(now.getFullYear(), now.getMonth(), now.getDate() + i)
    const top = i === 0 ? '今天' : i === 1 ? '明天' : i === 2 ? '后天' : WEEK[d.getDay()]
    out.push({ value: fmt(d), top, bottom: `${d.getMonth() + 1}/${d.getDate()}` })
  }
  return out
})

/**
 * 时段列表：读后台「时段库存」配置（铺满全天即可覆盖任意时间），未配置回落内置三段。
 * 与预约页 CalendarPicker 同一口径，改期可选范围与下单一致。
 */
const slots = computed(() => {
  const codes = Object.keys(avail.value).sort()
  if (codes.length) return codes.map((code) => ({ code, label: slotTextOf(code) }))
  return SLOTS
})

/** 时段起始时刻；解析失败返回 null（不参与「来不及」判断） */
function slotStart(dateStr: string, code: string): Date | null {
  const hour = Number.parseInt(code.split('-')[0], 10)
  if (Number.isNaN(hour)) return null
  const d = new Date(`${dateStr.replace(/-/g, '/')} ${pad(hour)}:00:00`)
  return Number.isNaN(d.getTime()) ? null : d
}

/**
 * 距离开场不足 2 小时不能再改：服务方可能已经出门了。
 * 规则与后端一致，前端先置灰，免得用户点了才收到报错。
 */
function tooSoon(code: string): boolean {
  const s = slotStart(pickedDate.value, code)
  if (!s) return false
  return s.getTime() - Date.now() < 2 * 3600 * 1000
}

function slotFull(code: string): boolean {
  const s = avail.value[code]
  return s ? !s.available : false
}

function slotTip(code: string): string {
  if (tooSoon(code)) return '来不及'
  const s = avail.value[code]
  if (!s) return ''
  if (!s.available) return '已满'
  if (s.remaining <= 2) return `余${s.remaining}`
  return ''
}

function usable(code: string): boolean {
  return !slotFull(code) && !tooSoon(code)
}

async function loadAvail() {
  const date = pickedDate.value
  if (!date) return
  try {
    const list = await slotAvailability(date, props.serviceType)
    const map: Record<string, SlotAvailability> = {}
    for (const s of list) map[s.slot_code] = s
    // 请求回来时用户可能已切到别的日期，丢弃过期结果
    if (date === pickedDate.value) avail.value = map
  } catch {
    if (date === pickedDate.value) avail.value = {}
  }
}

// 每次打开都重置成订单当前时间，避免上一单的选择串到下一单
watch(
  () => props.visible,
  (v) => {
    if (!v) return
    pickedDate.value = props.fromDate || dates.value[0].value
    pickedSlot.value = props.fromSlot || ''
    picked.value = ''
    detail.value = ''
    userPickedDate.value = false
  },
)

watch(
  () => [props.visible, pickedDate.value, props.serviceType],
  () => {
    if (props.visible) loadAvail()
  },
  { immediate: true },
)

// 换日期后原时段可能不在可选范围或已满，自动落到第一个可约时段。
// 若整天都不可约（今天的时段通常已经过了/来不及），且用户还没手动选过日期，
// 顺延到下一天——否则一打开就是个死局，得先自己想明白要往后翻。
watch([slots, avail, pickedDate], () => {
  if (slots.value.some((s) => usable(s.code))) {
    if (!slots.value.some((s) => s.code === pickedSlot.value && usable(s.code))) {
      pickedSlot.value = slots.value.filter((s) => usable(s.code))[0]?.code || ''
    }
    return
  }
  if (userPickedDate.value) {
    pickedSlot.value = ''
    return
  }
  const idx = dates.value.findIndex((d) => d.value === pickedDate.value)
  if (idx >= 0 && idx < dates.value.length - 1) {
    pickedDate.value = dates.value[idx + 1].value
  }
})

function slotLabel(code: string): string {
  return slotTextOf(code)
}

const fromText = computed(() => {
  if (!props.fromDate) return ''
  return `${friendlyDate(props.fromDate)} ${slotLabel(props.fromSlot)}`
})

const pickedText = computed(() => {
  if (!pickedDate.value || !pickedSlot.value) return '请选择新时间'
  return `${friendlyDate(pickedDate.value)} ${slotLabel(pickedSlot.value)}`
})

/** 跟原时间一样就没必要提交，后端也会拒 */
const changed = computed(
  () => pickedDate.value !== props.fromDate || pickedSlot.value !== props.fromSlot,
)

const count = computed(() => detail.value.length)

const finalReason = computed(() => [picked.value, detail.value.trim()].filter(Boolean).join('，'))

const isProvider = computed(() => props.mode === 'provider')

function pickDate(v: string) {
  userPickedDate.value = true
  pickedDate.value = v
}

function pickSlot(code: string) {
  if (!usable(code)) return
  pickedSlot.value = code
}

function pickReason(item: string) {
  picked.value = picked.value === item ? '' : item
}

function close() {
  if (props.loading) return
  emit('close')
}

function submit() {
  if (props.loading) return
  if (!pickedDate.value || !pickedSlot.value) {
    uni.showToast({ title: '请选择日期与时段', icon: 'none' })
    return
  }
  if (!changed.value) {
    uni.showToast({ title: '请选择与原时间不同的时段', icon: 'none' })
    return
  }
  emit('confirm', { date: pickedDate.value, slot: pickedSlot.value, reason: finalReason.value })
}
</script>

<template>
  <view v-if="visible" class="rsc-mask" @tap="close">
    <view class="rsc-panel" @tap.stop>
      <view class="rsc-head">
        <text class="rsc-title">{{ isProvider ? '申请修改服务时间' : '修改服务时间' }}</text>
        <text class="rsc-sub">
          {{ isProvider ? '提交后需住户确认，确认前仍按原时间服务' : '改完立即生效，原时段名额会释放' }}
        </text>
      </view>

      <view class="rsc-current">
        <text class="rsc-current-label">当前时间</text>
        <text class="rsc-current-val">{{ fromText }}</text>
      </view>

      <view class="rsc-block">
        <view class="rsc-label-row">
          <text class="rsc-label">改为</text>
          <text class="rsc-picked">{{ pickedText }}</text>
        </view>
        <scroll-view class="rsc-scroll" scroll-x :show-scrollbar="false">
          <view class="rsc-days">
            <view
              v-for="d in dates"
              :key="d.value"
              class="rsc-day"
              :class="{ on: pickedDate === d.value }"
              @tap="pickDate(d.value)"
            >
              <text class="rsc-day-top">{{ d.top }}</text>
              <text class="rsc-day-bottom">{{ d.bottom }}</text>
            </view>
          </view>
        </scroll-view>
      </view>

      <view class="rsc-block">
        <text class="rsc-label">可选时段</text>
        <view class="rsc-slots">
          <view
            v-for="s in slots"
            :key="s.code"
            class="rsc-slot"
            :class="{ on: pickedSlot === s.code, off: !usable(s.code) }"
            @tap="pickSlot(s.code)"
          >
            <text>{{ s.label }}</text>
            <text v-if="slotTip(s.code)" class="rsc-tip">{{ slotTip(s.code) }}</text>
          </view>
        </view>
        <view v-if="serviceType === 'dogwalk'" class="rsc-hint">
          遛宠工作日仅开放晚间时段，周末全天可约
        </view>
      </view>

      <view class="rsc-block">
        <view class="rsc-label-row">
          <text class="rsc-label">原因<text class="rsc-label-sub">（选填，对方可见）</text></text>
          <text class="rsc-count">{{ count }}/100</text>
        </view>
        <view class="rsc-chips">
          <view
            v-for="item in QUICK"
            :key="item"
            class="rsc-chip"
            :class="{ on: picked === item }"
            @tap="pickReason(item)"
          >
            {{ item }}
          </view>
        </view>
        <textarea
          v-model="detail"
          class="rsc-input"
          placeholder="补充说明，例如：周五临时加班，想挪到周六上午"
          placeholder-class="rsc-ph"
          :maxlength="100"
          :cursor-spacing="32"
        />
      </view>

      <view class="rsc-actions">
        <view class="btn-ghost rsc-btn" @tap="close">不改了</view>
        <view class="rsc-confirm rsc-btn" :class="{ off: loading }" @tap="submit">
          {{ loading ? '提交中…' : isProvider ? '提交申请' : '确认修改' }}
        </view>
      </view>
    </view>
  </view>
</template>

<style scoped>
.rsc-mask {
  position: fixed;
  inset: 0;
  z-index: 999;
  background: rgba(20, 24, 28, 0.45);
  display: flex;
  align-items: flex-end;
}

.rsc-panel {
  width: 100%;
  background: var(--card);
  border-radius: 28rpx 28rpx 0 0;
  padding: 36rpx 32rpx calc(32rpx + env(safe-area-inset-bottom));
  box-sizing: border-box;
}

.rsc-head {
  display: flex;
  flex-direction: column;
  margin-bottom: 22rpx;
}

.rsc-title {
  font-size: 36rpx;
  font-weight: 600;
  color: var(--text);
  line-height: 1.4;
}

.rsc-sub {
  margin-top: 8rpx;
  font-size: 24rpx;
  color: var(--text-sub);
}

/* 原时间：给用户一个对照系，避免选完忘记原本约的几点 */
.rsc-current {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 18rpx 22rpx;
  margin-bottom: 26rpx;
  background: #f6f7f8;
  border-radius: 14rpx;
}

.rsc-current-label {
  font-size: 25rpx;
  color: var(--text-sub);
}

.rsc-current-val {
  font-size: 26rpx;
  font-weight: 500;
  color: var(--text);
}

.rsc-block {
  margin-bottom: 26rpx;
}

.rsc-label-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.rsc-label {
  font-size: 26rpx;
  font-weight: 500;
  color: var(--text);
}

.rsc-label-sub {
  font-size: 22rpx;
  font-weight: 400;
  color: var(--text-light);
}

.rsc-picked {
  font-size: 24rpx;
  color: var(--brand-dark);
}

.rsc-count {
  font-size: 22rpx;
  color: var(--text-light);
}

/* 日期条横向滚动：7 天里通常只看前三个，但周末要能划到 */
.rsc-scroll {
  width: 100%;
  margin-top: 16rpx;
  white-space: nowrap;
}

.rsc-days {
  display: inline-flex;
  gap: 14rpx;
}

.rsc-day {
  width: 108rpx;
  padding: 16rpx 0;
  border: 1rpx solid var(--border);
  border-radius: 14rpx;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6rpx;
}

.rsc-day.on {
  border-color: var(--brand);
  background: var(--brand-light);
}

.rsc-day-top {
  font-size: 24rpx;
  color: var(--text);
}

.rsc-day.on .rsc-day-top {
  color: var(--brand-dark);
  font-weight: 600;
}

.rsc-day-bottom {
  font-size: 22rpx;
  color: var(--text-light);
}

.rsc-slots {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 14rpx;
  margin-top: 16rpx;
}

.rsc-slot {
  padding: 18rpx 0;
  border: 1rpx solid var(--border);
  border-radius: 12rpx;
  text-align: center;
  font-size: 24rpx;
  color: var(--text-sub);
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4rpx;
}

.rsc-slot.on {
  border-color: var(--brand);
  background: var(--brand-light);
  color: var(--brand-dark);
  font-weight: 600;
}

/* 不可约：划掉时段文字，避免误以为还能选 */
.rsc-slot.off {
  background: #f6f7f8;
  border-color: #eceff1;
  color: #b6bec6;
}

.rsc-slot.off text:first-child {
  text-decoration: line-through;
}

.rsc-tip {
  font-size: 18rpx;
  color: #b6bec6;
}

.rsc-slot.on .rsc-tip {
  color: var(--brand-dark);
}

.rsc-hint {
  margin-top: 12rpx;
  font-size: 21rpx;
  color: var(--text-light);
}

.rsc-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 14rpx;
  margin-top: 16rpx;
}

.rsc-chip {
  padding: 0 24rpx;
  height: 58rpx;
  line-height: 58rpx;
  font-size: 25rpx;
  color: var(--text-sub);
  background: #f6f7f8;
  border: 1rpx solid transparent;
  border-radius: 999rpx;
}

.rsc-chip.on {
  color: var(--brand);
  background: var(--brand-light);
  border-color: var(--brand);
  font-weight: 500;
}

.rsc-input {
  width: 100%;
  height: 140rpx;
  margin-top: 16rpx;
  padding: 20rpx 24rpx;
  box-sizing: border-box;
  font-size: 27rpx;
  line-height: 1.6;
  color: var(--text);
  background: #f6f7f8;
  border: 1rpx solid var(--border);
  border-radius: 14rpx;
}

.rsc-ph {
  color: var(--text-light);
  font-size: 26rpx;
}

.rsc-actions {
  display: flex;
  margin-top: 6rpx;
}

.rsc-btn {
  flex: 1;
}

.rsc-btn + .rsc-btn {
  margin-left: 20rpx;
}

.rsc-confirm {
  flex: 1.4;
  background: var(--brand);
  color: #fff;
  border-radius: 12rpx;
  height: 88rpx;
  line-height: 88rpx;
  text-align: center;
  font-size: 30rpx;
  font-weight: 500;
}

.rsc-confirm.off {
  background: #7fbdb7;
}
</style>
