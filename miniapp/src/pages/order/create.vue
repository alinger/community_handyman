<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { onLoad } from '@dcloudio/uni-app'
import CalendarPicker from '@/components/CalendarPicker.vue'
import {
  accompanyOptions,
  checkArea,
  createAddress,
  createOrder,
  getGarbagePlan,
  getQuotas,
  listCommunities,
  previewOrder,
  slotAvailability,
  updateAddress,
  type AccompanyHospital,
  type AccompanyServiceItem,
  type CreateOrderPayload,
  type GarbagePlan,
  type PreviewResp,
  type SlotAvailability,
} from '@/api'
import { useUserStore } from '@/store/user'
import { ensureAuth } from '@/utils/auth'
import { nextDays, price, SLOTS, slotTextOf } from '@/utils/format'
import { ApiError, BASE_URL } from '@/utils/request'

const user = useUserStore()

const dates = nextDays(7)
const durations = [20, 30, 40, 60]
/** 进入方式：与服务端 order.entry_method 的枚举保持一致 */
const ENTRY_METHODS = [
  { value: 'knock', label: '敲门' },
  { value: 'password', label: '门禁密码' },
  { value: 'card', label: '门禁卡' },
  { value: 'property', label: '物业代开' },
  { value: 'other', label: '其他' },
]

/**
 * 宠物预约（遛宠）只保留「门禁密码 / 门禁卡 / 其他」三个选项。
 * 敲门、物业代开仍保留给陪诊等其它上门服务，只是不在遛宠下单页出现。
 */
const DOGWALK_ENTRY_METHODS = ['password', 'card', 'other']

const garbageTypes = [
  { value: 'kitchen', label: '厨余垃圾' },
  { value: 'other', label: '其他垃圾' },
  { value: 'recyclable', label: '可回收物' },
]

/**
 * 遛宠「仅喂食清洁」「提前上门熟悉」两个开关暂时隐藏。
 * 后端计价与订单字段保留，接口继续传 false；需要恢复时改回 true 即可。
 */
const SHOW_DOGWALK_SWITCHES = false

/** 宠物只数选择暂时隐藏，固定按 1 只下单、按 1 只计价 */
const SHOW_PET_COUNT = false

const form = reactive({
  serviceType: 'garbage',
  serviceDate: dates[0].value,
  slotCode: '',
  // 代倒垃圾自 2026-11 起只卖包月；单次仅限注册赠送的免费次数
  plan: 'monthly',
  extraSlotCode: '',
  garbageType: 'kitchen',
  petCount: 1,
  durationMin: 20,
  feedOnly: false,
  needFamiliar: false,
  entryMethod: '' as string,
  accessInfo: '',
  hospital: '', // 存 code，名称从后台选项映射
  serviceItems: [] as string[], // 已勾选的陪诊服务项 code
  remark: '',
})

/** 当前服务类型下可选的进入方式：宠物预约（遛宠）只留门禁密码/门禁卡/其他 */
const entryMethods = computed(() =>
  form.serviceType === 'dogwalk'
    ? ENTRY_METHODS.filter((m) => DOGWALK_ENTRY_METHODS.includes(m.value))
    : ENTRY_METHODS,
)

/** 切换服务类型后清掉不属于当前类型的已选项，避免提交界面上不存在的进入方式 */
watch(
  () => form.serviceType,
  () => {
    if (form.entryMethod && !entryMethods.value.some((m) => m.value === form.entryMethod)) {
      form.entryMethod = ''
      form.accessInfo = ''
    }
  },
)

/** 门禁密码/其他才需要补充文字说明，卡与物业代开不用 */
const needsAccessText = computed(
  () => form.entryMethod === 'password' || form.entryMethod === 'other',
)

/** 包月方案（价格/天数/默认时段/额外时段），数据来自后端 SKU 规则 */
const garbagePlan = ref<GarbagePlan | null>(null)
/** 剩余免费次数 */
const garbageFree = ref(0)

const preview = ref<PreviewResp | null>(null)
/** 熟悉费累计进度：后端只在开启「提前上门熟悉」时返回 */
const familiarProgress = computed(() => preview.value?.familiar_progress)
/** 进度条宽度（%），达标后固定 100 */
const familiarPercent = computed(() => {
  const f = familiarProgress.value
  if (!f || f.need <= 0) return 0
  return Math.min(100, Math.round((f.done / f.need) * 100))
})
/** 进度文案：未拿到进度时退回静态规则说明（比如还没填地址时） */
const familiarText = computed(() => {
  const f = familiarProgress.value
  if (!f) return '累计完成 5 单后免熟悉费'
  if (f.free) return `已累计完成 ${f.done} 单，本次免熟悉费`
  const left = Math.max(f.need - f.done, 0)
  return `已累计完成 ${f.done} 单，再 ${left} 单免熟悉费`
})
const previewError = ref('')
const submitting = ref(false)
const showCalendar = ref(false)

/** 陪诊可选项全部来自后台配置，前端不内置医院与服务项 */
const hospitals = ref<AccompanyHospital[]>([])
const serviceItemOptions = ref<AccompanyServiceItem[]>([])
/** 默认必选的基础服务费，勾选态锁定；价格只用于展示，金额由后端计算 */
const baseItem = ref<AccompanyServiceItem | null>(null)

/** 服务地址选择弹层：展示小区/门牌/联系人/电话/默认标签，比系统 actionSheet 信息更全 */
const showAddressPicker = ref(false)
/** 服务地址编辑弹层：小区从后台配置点选、无路弄字段，保存后即时刷新试算 */
const showAddressEdit = ref(false)
const editingAddressId = ref(0) // 0 表示新增，非 0 表示编辑该地址
const addressSaving = ref(false)
const addressCheckMsg = ref<{ ok: boolean; text: string } | null>(null)
const communities = ref<string[]>([])
/** 小区选择弹层：showActionSheet 最多 6 项，后台小区已 12 个，必须自己列表 */
const showCommunityPicker = ref(false)
const addressForm = reactive({
  community: '',
  house_no: '',
  contact_name: '',
  contact_phone: '',
})

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

/**
 * 时段余量：满员时段置灰不可选。
 * 后端未配该日库存时返回空数组，此时视为全部可选（与后端下单兜底一致）。
 */
const slotAvail = ref<Record<string, SlotAvailability>>({})

async function loadSlotAvail() {
  try {
    const list = await slotAvailability(form.serviceDate, form.serviceType)
    const map: Record<string, SlotAvailability> = {}
    for (const s of list) map[s.slot_code] = s
    slotAvail.value = map
  } catch (e) {
    // 库存查询失败不阻塞下单：交给后端在提交时兜底
    slotAvail.value = {}
  }
}

/**
 * 时段列表：直接取后台「时段库存」配置（铺满全天即可让用户任选任意时间段），
 * 当日未配置时回落内置三段。与 CalendarPicker 的 daySlots 同一口径。
 */
const slots = computed(() => {
  const codes = Object.keys(slotAvail.value).sort()
  if (codes.length) return codes.map((code) => ({ code, label: slotTextOf(code) }))
  return SLOTS
})

/** 某时段是否可约：无库存记录视为可约 */
function slotUsable(code: string): boolean {
  const s = slotAvail.value[code]
  return s ? s.available : true
}

/** 剩余量文案，只在快满时才提示，避免每个时段都堆数字 */
function slotHint(code: string): string {
  const s = slotAvail.value[code]
  if (!s) return ''
  if (!s.available) return '已约满'
  if (s.remaining <= 2) return `余 ${s.remaining}`
  return ''
}

const addressText = computed(() => {
  const a = user.currentAddress
  if (!a) return ''
  return `${a.community} ${a.house_no}`
})

/** 所有服务都在地址卡展示联系人与联系方式，方便服务方上门联系 */
const addressContact = computed(() => {
  const a = user.currentAddress
  if (!a) return ''
  return [a.contact_name, a.contact_phone].filter(Boolean).join(' · ')
})

/** 当前是否走了包月流程（垃圾 + 包月） */
const isGarbageMonthly = computed(
  () => form.serviceType === 'garbage' && form.plan === 'monthly',
)

const timeText = computed(() => {
  const d = dates.find((x) => x.value === form.serviceDate)
  if (isGarbageMonthly.value) {
    // 包月不选时段：显示开始日 + 每日取件时间；选了备选时间则备选取代默认
    const slot = form.extraSlotCode || garbagePlan.value?.default_slot || '22:00-22:30'
    return d ? `${d.label} 开始 · 每天 ${slot} 取件` : ''
  }
  if (!form.slotCode) return d?.label || ''
  const s = slots.value.find((x) => x.code === form.slotCode)
  return [d?.label, s?.label].filter(Boolean).join(' ')
})

onLoad(async (query) => {
  if (!ensureAuth()) return
  if (query?.type && ['garbage', 'dogwalk', 'accompany'].includes(query.type)) {
    form.serviceType = query.type as string
  }
  try {
    await user.ensureLogin()
    await Promise.all([user.loadAddresses(), loadCommunities()])
  } catch (e) {
    console.warn('[create] 初始化失败', e)
  }
  await loadAccompanyOptions()
  if (form.serviceType === 'garbage') {
    await loadGarbageInfo()
  }
  if (!isGarbageMonthly.value) {
    await loadSlotAvail()
    syncSlot()
  }
  refreshPreview()
})

/**
 * 加载代倒垃圾的包月方案与剩余免费次数。
 * 价格、天数、时段全部来自后端配置，前端不写死，后台改价不用发版。
 */
async function loadGarbageInfo() {
  try {
    garbagePlan.value = await getGarbagePlan()
  } catch (e) {
    console.warn('[create] 包月方案加载失败', e)
  }
  try {
    const list = await getQuotas()
    garbageFree.value = list.find((q) => q.service_type === 'garbage')?.remaining || 0
  } catch (e) {
    console.warn('[create] 免费次数加载失败', e)
  }
  // 默认选包月；单次随时可买，有赠送次数时自动抵扣
}

/** 医院与陪诊服务项都从后台拉取，用户只能选、不能自由输入 */
async function loadAccompanyOptions() {
  try {
    const opts = await accompanyOptions()
    hospitals.value = opts.hospitals || []
    serviceItemOptions.value = opts.service_items || []
    baseItem.value = opts.base_item || null
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
    // 微信对 itemList 有 6 项上限，超限会静默失败；至少给个提示，别让用户以为没点到
    fail: () => uni.showToast({ title: '选项过多无法展开，请稍后重试', icon: 'none' }),
  })
}

function toggleServiceItem(code: string) {
  const i = form.serviceItems.indexOf(code)
  if (i >= 0) form.serviceItems.splice(i, 1)
  else form.serviceItems.push(code)
}

/** 选中项失效时回落到第一个仍可约的时段，全部约满则清空以触发提交拦截 */
function syncSlot() {
  const usable = slots.value.filter((s) => slotUsable(s.code))
  if (!usable.some((s) => s.code === form.slotCode)) {
    form.slotCode = usable.length ? usable[0].code : ''
  }
}

watch(
  () => [form.serviceDate, form.serviceType, form.plan],
  async () => {
    // 包月不选时段，也不查库存
    if (isGarbageMonthly.value) return
    await loadSlotAvail()
    syncSlot()
  },
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
    form.extraSlotCode,
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
    slot_code: form.slotCode || slots.value.filter((s) => slotUsable(s.code))[0]?.code || '',
    remark: form.remark,
  }

  // 进入方式只对需要进屋/上门的遛宠、陪诊有意义；
  // 代倒垃圾在门口取袋，不进房间，不下发这两个字段。
  if (form.serviceType !== 'garbage' && form.entryMethod) {
    payload.entry_method = form.entryMethod
    if (form.accessInfo.trim()) payload.access_info = form.accessInfo.trim()
  }

  if (form.serviceType === 'garbage') {
    payload.plan = form.plan
    payload.garbage_type = form.garbageType
    if (form.plan === 'monthly') {
      // 包月不占库存也不选时段：默认取件时间由后端按规则落单
      payload.slot_code = garbagePlan.value?.default_slot || '22:00-22:30'
      if (form.extraSlotCode) payload.extra_slot_code = form.extraSlotCode
    }
  } else if (form.serviceType === 'dogwalk') {
    payload.pet_count = form.petCount
    payload.duration_min = form.durationMin
    payload.feed_only = form.feedOnly
    payload.need_familiar = form.needFamiliar
  } else {
    // 陪诊：只回传后台配置项的 code，价格由后端按 SKU 规则计算
    // 就诊人直接取服务地址的联系人，不再单独输入
    payload.hospital = form.hospital
    payload.patient_name = addr.contact_name.trim()
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

/** 加载后台配置的小区列表，供地址表单点选 */
async function loadCommunities() {
  try {
    communities.value = await listCommunities()
  } catch (e) {
    console.warn('[create] 小区列表加载失败', e)
  }
}

function chooseAddress() {
  if (!user.hasAddress) {
    openAddressAdd()
    return
  }
  showAddressPicker.value = true
}

/** 从弹层选中地址：切换默认地址后即时刷新费用试算 */
function selectAddress(id: number) {
  user.setCurrentAddress(id)
  showAddressPicker.value = false
  refreshPreview()
}

/**
 * 新增地址：清空表单、editingAddressId 固定为 0，绝不预填当前地址。
 * 与编辑彻底分开，避免靠"有没有传 id"猜意图导致点新增却弹出编辑。
 */
function openAddressAdd() {
  editingAddressId.value = 0
  addressForm.community = ''
  addressForm.house_no = ''
  addressForm.contact_name = ''
  addressForm.contact_phone = ''
  addressCheckMsg.value = null
  showAddressPicker.value = false
  showAddressEdit.value = true
}

/**
 * 编辑指定地址：必须显式传数字 id，找不到则安全降级为新增。
 * 模板里若写 `@tap="openAddressEdit"` 会把事件对象当 id 传入，这里用 typeof 兜底。
 */
function openAddressEdit(id?: number) {
  if (typeof id !== 'number') {
    openAddressAdd()
    return
  }
  const a = user.addresses.find((x) => x.id === id)
  if (!a) {
    openAddressAdd()
    return
  }
  editingAddressId.value = a.id
  addressForm.community = a.community
  addressForm.house_no = a.house_no
  addressForm.contact_name = a.contact_name
  addressForm.contact_phone = a.contact_phone
  addressCheckMsg.value = null
  showAddressPicker.value = false
  showAddressEdit.value = true
}

/**
 * 小区只能从后台配置列表点选，避免自由输入不规范名称。
 *
 * 这里不能用 uni.showActionSheet：微信限制 itemList 最多 6 项，
 * 后台已配置 12 个小区，超限会直接 fail 且没有任何提示，
 * 表现就是「点了没反应」。改成自绘列表弹层，数量不受限也更好点。
 */
async function chooseCommunity() {
  if (!communities.value.length) {
    // 首屏加载可能失败过（网络抖动），进弹层前补一次
    await loadCommunities()
  }
  if (!communities.value.length) {
    uni.showToast({ title: '小区列表加载失败，请重试', icon: 'none' })
    return
  }
  showCommunityPicker.value = true
}

function pickCommunity(name: string) {
  addressForm.community = name
  showCommunityPicker.value = false
  onAddressCheck()
}

/** 输入门牌后即时校验范围，避免保存时才被后端拒绝 */
async function onAddressCheck() {
  if (!addressForm.community || !addressForm.house_no) {
    addressCheckMsg.value = null
    return
  }
  try {
    await checkArea({
      community: addressForm.community,
      house_no: addressForm.house_no,
    })
    addressCheckMsg.value = { ok: true, text: '该地址在服务范围内' }
  } catch (e) {
    addressCheckMsg.value = {
      ok: false,
      text: e instanceof ApiError ? e.message : '校验失败',
    }
  }
}

async function saveAddress() {
  if (addressSaving.value) return
  if (!addressForm.community || !addressForm.house_no) {
    uni.showToast({ title: '请选择小区并填写门牌号', icon: 'none' })
    return
  }
  addressSaving.value = true
  try {
    const data = {
      community: addressForm.community.trim(),
      house_no: addressForm.house_no.trim(),
      contact_name: addressForm.contact_name.trim(),
      contact_phone: addressForm.contact_phone.trim(),
    }
    const saved = editingAddressId.value
      ? await updateAddress(editingAddressId.value, data)
      : await createAddress(data)
    await user.loadAddresses()
    user.setCurrentAddress(saved.id)
    showAddressEdit.value = false
    uni.showToast({ title: editingAddressId.value ? '已更新' : '已添加', icon: 'success' })
    refreshPreview()
  } catch (e) {
    uni.showModal({
      title: '保存失败',
      content: e instanceof ApiError ? e.message : '请稍后重试',
      showCancel: false,
    })
  } finally {
    addressSaving.value = false
  }
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
  // 包月不选时段，直接跳过这项校验
  if (!isGarbageMonthly.value && !form.slotCode) {
    // 区分「没选」与「所选日期已全部约满」，后者要让用户换日期而不是重选时段
    const allFull = slots.value.length > 0 && !slots.value.some((s) => slotUsable(s.code))
    uni.showToast({
      title: allFull ? '该日期已约满，请换个日期' : '请选择服务时间',
      icon: 'none',
    })
    if (!allFull) showCalendar.value = true
    return
  }
  if (form.serviceType === 'accompany') {
    if (!form.hospital) {
      uni.showToast({ title: '请选择就诊医院', icon: 'none' })
      return
    }
    // 基础服务费本身即构成一单，只有未配置基础费的老配置才要求至少选一项
    if (!form.serviceItems.length && !baseItem.value) {
      uni.showToast({ title: '请至少选择一项陪诊服务', icon: 'none' })
      return
    }
  }

  const payload = buildPayload()
  if (!payload) return

  submitting.value = true
  try {
    const res = await createOrder(payload)
    // 试运行期后端会跳过支付、直接进入待接单，此时不需要也不应该调起支付。
    // 保留 pending_payment 分支：微信支付接入后这段代码不用改。
    if (res.order.status === 'pending_payment') {
      await startPay(res.order.order_no)
    } else {
      uni.showToast({ title: '下单成功', icon: 'success' })
    }
    // 用 navigateTo 保留页面栈，详情页原生导航栏才会显示返回箭头；
    // redirectTo 会销毁预约页，栈里只剩详情页，左上角返回按钮消失。
    uni.navigateTo({ url: `/pages/order/detail?orderNo=${res.order.order_no}` })
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
      // 用统一的 BASE_URL，避免和 request.ts 的回落逻辑不一致
      url: `${BASE_URL}/api/v1/dev/orders/${orderNo}/mock-pay`,
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
          {{ user.currentAddress?.community || '示例小区' }} ·
          {{ isGarbageMonthly ? '包月服务' : '单次服务' }}
        </view>
      </view>
      <view class="banner-art">{{ banner.art }}</view>
    </view>

    <!-- 服务地址：地址 + 联系人与联系方式，可直接编辑 -->
    <view class="card">
      <view class="f-row">
        <view class="f-label">服务地址</view>
        <text class="addr-edit" @tap.stop="openAddressEdit(user.currentAddress?.id)">编辑</text>
      </view>
      <view class="f-value" :class="{ placeholder: !addressText }" @tap="chooseAddress">
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

    <!-- 服务方案（仅垃圾）：包月为主，单次只能用注册赠送的免费次数 -->
    <view v-if="form.serviceType === 'garbage'" class="card">
      <view class="f-label">服务方案</view>

      <view class="plan-card" :class="{ active: form.plan === 'monthly' }" @tap="form.plan = 'monthly'">
        <view class="plan-main">
          <view class="plan-name">包月代倒垃圾</view>
          <view class="plan-sub">每天 {{ garbagePlan?.default_slot || '22:00-22:30' }} 上门取件</view>
        </view>
        <view class="plan-price">
          {{ garbagePlan ? price(garbagePlan.price) : '¥30' }}
          <text class="plan-unit">/{{ garbagePlan?.days || 30 }} 天</text>
        </view>
      </view>

      <view class="plan-card" :class="{ active: form.plan === 'single' }" @tap="form.plan = 'single'">
        <view class="plan-main">
          <view class="plan-name">按次倒垃圾</view>
          <view class="plan-sub">
            <template v-if="garbageFree > 0">赠送次数抵扣，剩余 {{ garbageFree }} 次</template>
            <template v-else>上门取件一次，需选择时段</template>
          </view>
        </view>
        <view v-if="garbageFree > 0" class="plan-price free">免费</view>
        <view v-else class="plan-price">
          {{ garbagePlan ? price(garbagePlan.single_price) : '¥5.00' }}
          <text class="plan-unit">/次</text>
        </view>
      </view>

      <!-- 备选时段：选中后取代默认时段，每天改按备选时间上门取件 -->
      <template v-if="form.plan === 'monthly' && garbagePlan && garbagePlan.extra_slots.length">
        <view class="divider" />
        <view class="f-label">
          备选服务时间<text class="f-label-sub">（可选，替换默认取件时间）</text>
        </view>
        <view class="pills">
          <view class="pill soft" :class="{ active: !form.extraSlotCode }" @tap="form.extraSlotCode = ''">
            不需要
          </view>
          <view
            v-for="s in garbagePlan.extra_slots"
            :key="s"
            class="pill soft"
            :class="{ active: form.extraSlotCode === s }"
            @tap="form.extraSlotCode = s"
          >
            {{ s }}
          </view>
        </view>
      </template>
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
      <!-- 宠物只数暂时隐藏，固定 1 只 -->
      <view v-if="SHOW_PET_COUNT" class="f-row">
        <view class="f-label">宠物只数</view>
        <view class="stepper">
          <view class="step-btn" @tap="form.petCount = Math.max(1, form.petCount - 1)">−</view>
          <text class="step-val">{{ form.petCount }} 只</text>
          <view class="step-btn" @tap="form.petCount = Math.min(5, form.petCount + 1)">＋</view>
        </view>
      </view>
      <!-- 仅喂食按次计价，时长没有意义，直接收起避免「选了没反应」 -->
      <template v-if="!form.feedOnly">
        <!-- 只数隐藏后「服务时长」是卡片首行，不再需要顶部分隔线 -->
        <view v-if="SHOW_PET_COUNT" class="divider" />
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
      </template>
      <!-- 宠物情况并到页面底部的「备注」，不在遛宠卡片里单独留一个备注框 -->
      <!-- 仅喂食 / 提前熟悉两个开关暂时隐藏，后端能力保留 -->
      <template v-if="SHOW_DOGWALK_SWITCHES">
        <view class="divider" />
        <view class="f-row">
          <view class="f-label">仅喂食清洁（不遛）</view>
          <switch class="sw" :checked="form.feedOnly" color="#0f7f78" @change="form.feedOnly = !form.feedOnly" />
        </view>
        <view v-if="form.feedOnly" class="f-hint">
          按只数计价，不按时长计费；提前熟悉、超距离与节日加价照常计算
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
        <view class="familiar-box">
          <view v-if="familiarProgress" class="fp-bar">
            <view class="fp-fill" :class="{ done: familiarProgress.free }" :style="{ width: familiarPercent + '%' }" />
          </view>
          <text class="f-hint">{{ familiarText }}</text>
        </view>
      </template>
    </view>

    <!-- 陪诊信息：就诊人取服务地址联系人；医院只能从后台选项选，右对齐展示 -->
    <view v-if="form.serviceType === 'accompany'" class="card">
      <view class="f-row hospital-row" @tap="chooseHospital">
        <view class="f-label">就诊医院</view>
        <view class="hospital-value" :class="{ placeholder: !hospitalName }">
          <text>{{ hospitalName || '请选择就诊医院' }}</text>
          <text class="chev">›</text>
        </view>
      </view>
      <view class="divider" />
      <!-- 基础服务费：默认必选，勾选态锁定，价格展示用，金额由后端算 -->
      <view v-if="baseItem" class="base-item">
        <view class="base-item-name">
          <text class="check-box checked">✓</text>
          <text>{{ baseItem.name }}</text>
          <text class="base-tag">必选</text>
        </view>
        <text class="service-chip-price">{{ price(baseItem.price) }}</text>
      </view>
      <view class="f-label service-label">陪诊服务 <text class="f-label-sub">（可多选）</text></view>
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

    <!-- 进入方式（选填）：门禁信息加密存储，服务方只在服务前后窗口内可见。
         代倒垃圾只到门口取袋，不进房间，所以这类订单不问进入方式。 -->
    <view v-if="form.serviceType !== 'garbage'" class="card">
      <view class="f-label">进入方式（选填）</view>
      <view class="pills">
        <view
          v-for="m in entryMethods"
          :key="m.value"
          class="pill soft"
          :class="{ active: form.entryMethod === m.value }"
          @tap="form.entryMethod = form.entryMethod === m.value ? '' : m.value"
        >
          {{ m.label }}
        </view>
      </view>
      <input
        v-if="needsAccessText"
        v-model="form.accessInfo"
        class="remark-input"
        maxlength="100"
        :placeholder="form.entryMethod === 'password' ? '门禁密码，例如 8888#' : '钥匙在哪、怎么进门'"
      />
      <view class="f-hint">
        加密保存，服务方仅在服务开始前 2 小时至服务结束后可见
      </view>
    </view>

    <!-- 备注：遛宠的宠物情况也写在这里，全页只留一个备注入口；门禁信息归「进入方式」卡片 -->
    <view class="card">
      <view class="f-label">
        备注
        <text v-if="form.serviceType === 'dogwalk'" class="f-label-sub">（选填：宠物类型、名字、情况或其他要求）</text>
      </view>
      <textarea
        v-if="form.serviceType === 'dogwalk'"
        v-model="form.remark"
        class="f-textarea"
        placeholder="如：金毛，叫豆豆，怕打雷；请提前联系"
        maxlength="200"
      />
      <input
        v-else
        v-model="form.remark"
        class="remark-input"
        maxlength="200"
        placeholder="例如：请提前联系"
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

    <!-- 服务地址选择弹层：比系统 actionSheet 展示更多信息 -->
    <view v-if="showAddressPicker" class="mask" @tap="showAddressPicker = false">
      <view class="sheet" @tap.stop>
        <view class="sheet-title">选择服务地址</view>
        <view class="picker-list">
          <view
            v-for="a in user.addresses"
            :key="a.id"
            class="picker-item"
            :class="{ active: a.id === user.currentAddressId }"
            @tap="selectAddress(a.id)"
          >
            <view class="picker-info">
              <view class="picker-line">
                <text class="picker-community">{{ a.community }}</text>
                <text class="picker-house">{{ a.house_no }}</text>
                <text v-if="a.id === user.currentAddressId" class="picker-tag">当前</text>
              </view>
              <view class="picker-contact">
                <text class="contact-dot" />
                <text>{{ [a.contact_name, a.contact_phone].filter(Boolean).join(' · ') || '未填写联系人' }}</text>
              </view>
            </view>
            <text class="picker-edit" @tap.stop="openAddressEdit(a.id)">编辑</text>
          </view>
        </view>
        <view class="sheet-actions">
          <view class="btn-ghost half" @tap="showAddressPicker = false">取消</view>
          <view class="btn-primary half" @tap="openAddressAdd">＋ 新增地址</view>
        </view>
      </view>
    </view>

    <!-- 服务地址编辑弹层：预约页就地改地址，不必跳「我的」页 -->
    <view v-if="showAddressEdit" class="mask" @tap="showAddressEdit = false">
      <view class="sheet" @tap.stop>
        <view class="sheet-title">
          {{ editingAddressId ? '编辑服务地址' : '添加服务地址' }}
        </view>
        <view class="form-item form-select-item" @tap="chooseCommunity">
          <text class="form-label">小区</text>
          <text class="form-select" :class="{ placeholder: !addressForm.community }">
            {{ addressForm.community || '请选择小区' }}
          </text>
          <text class="chev">›</text>
        </view>
        <view class="form-item">
          <text class="form-label">门牌号</text>
          <input v-model="addressForm.house_no" class="form-input" placeholder="如：45号" @blur="onAddressCheck" />
        </view>
        <view class="form-item">
          <text class="form-label">联系人</text>
          <input v-model="addressForm.contact_name" class="form-input" maxlength="16" placeholder="如：张阿姨" />
        </view>
        <view class="form-item">
          <text class="form-label">联系电话</text>
          <input
            v-model="addressForm.contact_phone"
            class="form-input"
            type="number"
            maxlength="11"
            placeholder="上门服务联系用"
          />
        </view>
        <view v-if="addressCheckMsg" class="check-msg" :class="{ ok: addressCheckMsg.ok }">
          {{ addressCheckMsg.text }}
        </view>
        <view class="sheet-actions">
          <view class="btn-ghost half" @tap="showAddressEdit = false">取消</view>
          <view class="btn-primary half" :class="{ 'is-disabled': addressSaving }" @tap="saveAddress">
            {{ addressSaving ? '保存中…' : '保存' }}
          </view>
        </view>
      </view>
    </view>

    <view v-if="showCommunityPicker" class="mask mask-top" @tap="showCommunityPicker = false">
      <view class="sheet" @tap.stop>
        <view class="sheet-title">选择小区</view>
        <scroll-view scroll-y class="comm-list">
          <view
            v-for="c in communities"
            :key="c"
            class="comm-item"
            :class="{ active: c === addressForm.community }"
            @tap="pickCommunity(c)"
          >
            <text class="comm-name">{{ c }}</text>
            <text v-if="c === addressForm.community" class="comm-check">✓</text>
          </view>
        </scroll-view>
        <view class="sheet-actions">
          <view class="btn-ghost full" @tap="showCommunityPicker = false">关闭</view>
        </view>
      </view>
    </view>

    <CalendarPicker
      v-model:visible="showCalendar"
      :date="form.serviceDate"
      :slot="form.slotCode"
      :service-type="form.serviceType"
      :date-only="isGarbageMonthly"
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

/* 就诊医院：标签靠左、选中医院右对齐，与原就诊人输入框视觉一致 */
.hospital-row {
  padding: 8rpx 0;
}

.hospital-value {
  display: flex;
  align-items: center;
  gap: 12rpx;
  font-size: 26rpx;
  color: var(--text);
}

.hospital-value.placeholder {
  color: var(--text-light);
}

/* 地址卡右上角就地编辑入口 */
.addr-edit {
  font-size: 24rpx;
  color: var(--brand-dark);
  padding: 4rpx 16rpx;
  background: var(--brand-light);
  border-radius: 20rpx;
}

/* 陪诊基础服务费：默认必选、锁定勾选态 */
.base-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12rpx;
  margin: 16rpx 0 4rpx;
  padding: 18rpx;
  border: 1rpx solid var(--brand);
  border-radius: 12rpx;
  background: var(--brand-light);
  font-size: 24rpx;
  color: var(--brand-dark);
}

.base-item-name {
  display: flex;
  align-items: center;
  gap: 10rpx;
  min-width: 0;
}

.check-box.checked {
  background: var(--brand);
  border-color: var(--brand);
  color: #fff;
}

.base-tag {
  font-size: 20rpx;
  padding: 2rpx 10rpx;
  border-radius: 8rpx;
  background: rgba(15, 127, 120, 0.14);
}

.service-label {
  margin-top: 22rpx;
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

/* 熟悉费累计进度：让「再几单免费」可见，而不是只给一句静态规则 */
.familiar-box {
  margin-top: 12rpx;
}

.fp-bar {
  height: 8rpx;
  border-radius: 999rpx;
  background: var(--brand-light);
  overflow: hidden;
}

.fp-fill {
  height: 100%;
  border-radius: 999rpx;
  background: var(--mint-1);
  transition: width 0.25s ease;
}

.fp-fill.done {
  background: var(--brand);
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

.f-textarea {
  width: 100%;
  min-height: 120rpx;
  padding: 20rpx 24rpx;
  box-sizing: border-box;
  font-size: 27rpx;
  color: var(--text);
  background: var(--bg);
  border-radius: 16rpx;
}

/* 方案卡：包月为主推，选中态用主色描边 */
.plan-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20rpx;
  padding: 24rpx;
  margin-bottom: 16rpx;
  border: 2rpx solid var(--border);
  border-radius: 16rpx;
  background: #fff;
}

.plan-card.active {
  border-color: var(--brand);
  background: var(--brand-tint);
}

.plan-main {
  flex: 1;
  min-width: 0;
}

.plan-name {
  font-size: 29rpx;
  font-weight: 700;
  color: var(--text);
}

.plan-sub {
  margin-top: 8rpx;
  font-size: 23rpx;
  color: var(--text-sub);
  line-height: 1.5;
}

.plan-price {
  font-size: 32rpx;
  font-weight: 800;
  color: var(--brand-dark);
  white-space: nowrap;
}

.plan-price.free {
  font-size: 28rpx;
  color: var(--accent);
}

.plan-unit {
  font-size: 22rpx;
  font-weight: 500;
  color: var(--text-light);
}

.plan-tip {
  margin-top: 8rpx;
  font-size: 23rpx;
  color: var(--text-light);
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

/* ---------- 地址编辑弹层 ---------- */
.mask {
  position: fixed;
  inset: 0;
  background: rgba(15, 23, 42, 0.45);
  z-index: 200;
  display: flex;
  align-items: flex-end;
}

.sheet {
  width: 100%;
  background: #fff;
  border-radius: 32rpx 32rpx 0 0;
  padding: 40rpx 32rpx;
  padding-bottom: calc(40rpx + env(safe-area-inset-bottom));
}

.sheet-title {
  font-size: 32rpx;
  font-weight: 600;
  text-align: center;
  margin-bottom: 24rpx;
}

.form-item {
  display: flex;
  align-items: center;
  gap: 24rpx;
  padding: 22rpx 0;
  border-bottom: 1rpx solid #f3f4f6;
}

.form-label {
  width: 150rpx;
  font-size: 27rpx;
  color: var(--text-sub);
  flex-shrink: 0;
}

.form-input {
  flex: 1;
  font-size: 28rpx;
}

/* 小区为后台配置点选，不可自由输入 */
.form-select-item {
  position: relative;
}

.form-select {
  flex: 1;
  font-size: 28rpx;
  color: var(--text);
}

.form-select.placeholder {
  color: var(--text-light);
}

/* 地址选择弹层：一条地址展示小区、门牌、联系人与电话 */
.picker-list {
  max-height: 60vh;
  overflow-y: auto;
}

/* 小区选择弹层要盖在地址编辑弹层之上 */
.mask-top {
  z-index: 220;
}

.comm-list {
  max-height: 56vh;
  height: 56vh;
}

.comm-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 26rpx 20rpx;
  border-bottom: 1rpx solid #f3f4f6;
  font-size: 28rpx;
  color: var(--text);
}

.comm-item.active {
  color: var(--brand);
  background: var(--brand-tint);
  font-weight: 600;
}

.comm-check {
  color: var(--brand);
  font-size: 26rpx;
}

.btn-ghost.full {
  flex: 1;
}

.picker-item {
  display: flex;
  align-items: center;
  gap: 16rpx;
  padding: 24rpx 20rpx;
  border: 1rpx solid #e8ecea;
  border-radius: 14rpx;
  margin-bottom: 14rpx;
}

.picker-item.active {
  border-color: var(--brand);
  background: var(--brand-light);
}

.picker-info {
  flex: 1;
  min-width: 0;
}

.picker-line {
  display: flex;
  align-items: center;
  gap: 12rpx;
  flex-wrap: wrap;
}

.picker-community {
  font-size: 28rpx;
  font-weight: 500;
  color: var(--text);
}

.picker-house {
  font-size: 26rpx;
  color: var(--text-sub);
}

.picker-tag {
  font-size: 20rpx;
  color: #fff;
  background: var(--brand);
  padding: 2rpx 12rpx;
  border-radius: 8rpx;
}

.picker-contact {
  margin-top: 10rpx;
  display: flex;
  align-items: center;
  gap: 8rpx;
  font-size: 23rpx;
  color: var(--brand-dark);
}

.picker-edit {
  flex-shrink: 0;
  font-size: 23rpx;
  color: var(--brand-dark);
  padding: 8rpx 18rpx;
  border: 1rpx solid var(--brand-light);
  border-radius: 18rpx;
}

.check-msg {
  margin-top: 16rpx;
  font-size: 24rpx;
  color: var(--danger);
}

.check-msg.ok {
  color: var(--brand-dark);
}

.sheet-actions {
  display: flex;
  gap: 20rpx;
  margin-top: 36rpx;
}

.half {
  flex: 1;
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
