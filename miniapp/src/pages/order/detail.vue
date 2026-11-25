<script setup lang="ts">
import { computed, ref } from 'vue'
import { onLoad } from '@dcloudio/uni-app'
import RejectSheet from '@/components/RejectSheet.vue'
import RescheduleSheet from '@/components/RescheduleSheet.vue'
import FinishSheet from '@/components/FinishSheet.vue'
import {
  ackProviderReschedule,
  advanceOrder,
  rejectOrder,
  cancelOrder,
  confirmOrder,
  listRecords,
  monthlyDetail,
  orderDetail,
  providerMonthlyFinish,
  providerMonthlyRedeem,
  providerReschedule,
  rescheduleOrder,
  respondReschedule,
  uploadRecord,
  type MonthlySummary,
  type Order,
  type ServiceRecord,
} from '@/api'
import { useUserStore } from '@/store/user'
import { ensureAuth } from '@/utils/auth'
import { friendlyDate, price, SERVICE_LABEL, slotTextOf } from '@/utils/format'
import { ApiError, BASE_URL } from '@/utils/request'
import { contactProvider } from '@/utils/provider-contact'
import { startOrderPay } from '@/utils/pay'

const user = useUserStore()
const orderNo = ref('')
const order = ref<Order | null>(null)
const records = ref<ServiceRecord[]>([])
const monthly = ref<MonthlySummary | null>(null)
const loading = ref(true)
const acting = ref(false)
const uploading = ref(false)
/** 流程图 P2：服务中拍照需经主人同意，勾选后随上传留痕 */
const ownerConsented = ref(true)

/** 服务方视角：接单后可推进节点、上传记录。
    用视角判断而非 role：已认证用户切回住户视角时，看自己的订单不该出现推进按钮 */
const isProvider = computed(() => user.isProviderView)

const slotText = computed(() =>
  slotTextOf(order.value?.extra_slot_code || order.value?.slot_code || ''),
)

/** 进入方式的展示文案，与服务端枚举对齐 */
const ENTRY_LABEL: Record<string, string> = {
  knock: '敲门',
  password: '门禁密码',
  card: '门禁卡',
  property: '物业代开',
  other: '其他',
}

/** 是否要展示「进入方式」这一行：没填就整行隐藏 */
const hasAccessInfo = computed(() => {
  const o = order.value
  if (!o) return false
  return Boolean(o.entry_method || o.access_info || o.access_visible === false)
})

/**
 * 进入方式的展示文本。
 * access_visible === false 表示填了但还没到可见时间，
 * 这时要给出明确提示，而不是显示空白让人以为没填。
 */
const entryText = computed(() => {
  const o = order.value
  if (!o) return ''
  const method = ENTRY_LABEL[o.entry_method || ''] || ''
  if (o.access_info) return method ? `${method} · ${o.access_info}` : o.access_info
  if (o.access_visible === false) {
    return `${method || '已填写'} · 服务开始前 2 小时显示`
  }
  return method || '已填写'
})

function copyAccess() {
  const v = order.value?.access_info
  if (!v) return
  uni.setClipboardData({ data: v })
}

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
  if (!ensureAuth()) return
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
    // 包月订单额外拉本周期核销进度（单次订单没有这个区块）
    if (order.value.plan === 'monthly') {
      monthly.value = await monthlyDetail(orderNo.value).catch(() => null)
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

/** 支付。渠道由后端配置决定（wechat / sandbox），调起逻辑在 utils/pay */
async function onPay() {
  if (acting.value || !order.value) return
  acting.value = true
  try {
    await startOrderPay(order.value.order_no)
    // 微信回调是异步推进订单状态的，稍等片刻再刷新；期间状态未变时下拉刷新即可
    await new Promise((r) => setTimeout(r, 800))
    await load()
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
  // 附件地址由后端签了短期票据（?t=），原生 image / previewImage 直接可用。
  // 不要在这里再拼 Authorization：这些组件发起的请求带不上自定义头。
  uni.previewImage({ urls, current })
}

/**
 * 服务记录按凭证阶段分组：start=开始服务 / finish=完成服务 / 空=过程记录。
 * 全部都是过程记录时（垃圾单与历史数据没有阶段），不分组、保持原来的平铺网格。
 */
const recordGroups = computed(() => {
  if (!records.value.some((r) => r.stage)) {
    return records.value.length ? [{ key: 'all', label: '', list: records.value }] : []
  }
  const pick = (stage: string) => records.value.filter((r) => r.stage === stage)
  return [
    { key: 'start', label: '开始服务凭证', list: pick('start') },
    { key: 'finish', label: '完成服务凭证', list: pick('finish') },
    { key: 'process', label: '过程记录', list: records.value.filter((r) => !r.stage) },
  ].filter((g) => g.list.length)
})

/**
 * 月卡今日状态一句话。
 * 进度条只说明「用掉了几天」，回答不了「今天来没来」——
 * 而后者才是住户和上门服务方每天都要确认的事。
 */
const monthlyToday = computed(() => {
  const m = monthly.value
  if (!m) return ''
  if (m.today_finished) return '今日已服务'
  // 到了门口还没收尾：说的不是「已服务」，住户不会以为这趟已经结束
  if (m.today_done) return '服务方已上门，服务进行中'
  if (m.in_period) return '今天还没上门'
  return m.ended ? '服务期已结束' : '今天不在服务期'
})

/** 核销记录一行：说清「上门了」还是「做完了」，住户最关心后者 */
function redemptionNote(r: { service_date: string; finished_at?: string }): string {
  if (r.finished_at) return `已完成 ${formatTime(r.finished_at)}`
  return '已上门，待完成'
}

/** 服务方已无可做动作的两个状态：等住户确认、已完成 */
const isClosed = computed(
  () => order.value?.status === 'pending_confirm' || order.value?.status === 'completed',
)

/** 服务方今天还要不要收尾这一趟（打了卡但没点完成） */
const monthlyNeedFinish = computed(
  () => !!monthly.value?.today_done && !monthly.value?.today_finished && !isClosed.value,
)

/**
 * 今天已经上过门（打过卡）。
 * 包月打卡后订单状态仍停在「已受理」，只看状态会给一张已经上过门的单
 * 留着「无法接单」——人都来了还能拒，说不通，点了也只会拿到 2003。
 */
const servedMonthlyToday = computed(
  () => !!monthly.value?.today_done && !isClosed.value,
)

/**
 * 服务方按状态可执行的推进动作，合法性最终由后端白名单兜底。
 * 垃圾代倒出发即到场、完成即终态；遛宠/陪诊仍是一步一按钮，
 * 住户要看完「送回宠物/陪诊交接」再确认，中间态不能被跳过。
 */
const providerActions = computed(() => {
  const o = order.value
  if (!isProvider.value || !o) return []
  // 月卡走每日打卡，不给单次的三步推进：点一下「完成服务」整月订单就结束了，
  // 后面二十多天住户的钱等于白付。结算入口在服务期结束后由列表页给出。
  if (o.plan === 'monthly') return []
  const acts: { node: string; label: string }[] = []
  if (o.service_type === 'garbage') {
    if (o.status === 'accepted') {
      acts.push({ node: 'in_service', label: '出发上门' })
    } else if (o.status === 'on_the_way') {
      acts.push({ node: 'in_service', label: '开始服务' })
    } else if (o.status === 'in_service') {
      acts.push({ node: 'completed', label: '完成服务' })
    }
    return acts
  }
  if (o.status === 'accepted') {
    acts.push({ node: 'on_the_way', label: '出发上门' })
  } else if (o.status === 'on_the_way') {
    acts.push({ node: 'in_service', label: '开始服务' })
  } else if (o.status === 'in_service') {
    acts.push({ node: 'pending_confirm', label: '完成服务' })
  }
  return acts
})

/**
 * 月卡今日打卡（「出发上门」）。只刷新核销进度，不重拉整个订单——
 * 打卡不改变订单状态，重拉会让页面闪一下又回到同样的内容。
 */
async function onRedeemMonthly() {
  const o = order.value
  if (!o || acting.value) return
  acting.value = true
  try {
    await providerMonthlyRedeem(o.order_no)
    monthly.value = await monthlyDetail(o.order_no).catch(() => monthly.value)
    uni.showToast({ title: '已记录上门，请点完成服务', icon: 'none' })
  } catch (e) {
    uni.showModal({
      title: '打卡失败',
      content: e instanceof ApiError ? e.message : '请稍后重试',
      showCancel: false,
    })
  } finally {
    acting.value = false
  }
}

/** 已接单到完成后都可补传照片 */
const canUploadRecord = computed(() => {
  if (!isProvider.value || !order.value) return false
  return ['accepted', 'on_the_way', 'in_service', 'pending_confirm', 'completed'].includes(
    order.value.status,
  )
})

/**
 * ---------- 改期 ----------
 * 遛宠对上门时间最敏感（宠物要提前安排、家里要有人），所以入口只对 dogwalk 开放；
 * 后端本身是通用的，日后要在垃圾/陪诊放开只需去掉这里的 service_type 判断。
 *
 * 两端能力不同：住户改完立即生效，服务方只能发起申请、等住户确认。
 */
const RESCHEDULABLE = ['pending_payment', 'pending_accept', 'accepted']

const rescheduleOpen = ref(false)

const isDogwalk = computed(() => order.value?.service_type === 'dogwalk')

/** 住户可以改：还没进入出发/服务阶段 */
const canReschedule = computed(() => {
  const o = order.value
  if (!o || !isDogwalk.value || isProvider.value) return false
  return RESCHEDULABLE.includes(o.status)
})

/** 服务方可以申请：已接单且没有待确认的申请，避免重复提交 */
const canRequestReschedule = computed(() => {
  const o = order.value
  if (!o || !isDogwalk.value || !isProvider.value) return false
  return o.status === 'accepted' && !o.pending_reschedule
})

const rescheduleMode = computed<'user' | 'provider'>(() =>
  isProvider.value ? 'provider' : 'user',
)

/** 弹层入参集中一处：模板里少写几个可选链，小程序端编译器对 ?. 支持不一致 */
const rescheduleFrom = computed(() => ({
  serviceType: order.value?.service_type || '',
  date: order.value?.service_date || '',
  slot: order.value?.slot_code || '',
}))

function openReschedule() {
  if (acting.value || !order.value) return
  rescheduleOpen.value = true
}

async function onReschedule(payload: { date: string; slot: string; reason: string }) {
  const o = order.value
  if (!o || acting.value) return
  acting.value = true
  try {
    const body = { service_date: payload.date, slot_code: payload.slot, reason: payload.reason }
    if (isProvider.value) await providerReschedule(o.order_no, body)
    else await rescheduleOrder(o.order_no, body)
    // 先关弹层再提示，否则 toast 会被遮罩挡住
    rescheduleOpen.value = false
    uni.showToast({
      title: isProvider.value ? '已提交，等待确认' : '时间已修改',
      icon: 'success',
    })
    await load()
  } catch (e) {
    uni.showModal({
      title: isProvider.value ? '申请失败' : '修改失败',
      content: e instanceof ApiError ? e.message : '请稍后重试',
      showCancel: false,
    })
  } finally {
    acting.value = false
  }
}

/** 住户处理服务方的改期申请 */
async function onRespondReschedule(approve: boolean) {
  const o = order.value
  if (!o || acting.value) return
  if (!approve) {
    const r = await new Promise<boolean>((resolve) => {
      uni.showModal({
        title: '保持原时间？',
        content: '拒绝后订单仍按原时间服务，服务方会收到通知',
        confirmText: '保持原时间',
        cancelText: '再想想',
        success: (res) => resolve(res.confirm),
        fail: () => resolve(false),
      })
    })
    if (!r) return
  }
  acting.value = true
  try {
    await respondReschedule(o.order_no, approve)
    uni.showToast({ title: approve ? '已同意改期' : '已保持原时间', icon: 'success' })
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

/** 已生效的改期记录：订单主表只剩最新时间，历史靠这张表还原 */
const appliedReschedules = computed(() =>
  (order.value?.reschedules || []).filter((r) => r.status === 'applied'),
)

function dateSlot(date: string, slot: string): string {
  return `${friendlyDate(date)} ${slotTextOf(slot)}`
}

/** 住户改期的原时间 / 新时间。抽成函数避开嵌套可空字段的窄化报错 */
function noticeFrom(o: Order | null): string {
  const rs = o?.reschedule_notice
  return rs ? dateSlot(rs.from_date, rs.from_slot) : ''
}

function noticeTo(o: Order | null): string {
  const rs = o?.reschedule_notice
  return rs ? dateSlot(rs.to_date, rs.to_slot) : ''
}

/**
 * 服务方点「我知道了」。
 * 住户改期是立即生效的，服务方进来核对完新时间，把这条提示收掉。
 */
async function onAckReschedule() {
  const o = order.value
  if (!o || acting.value) return
  acting.value = true
  try {
    await ackProviderReschedule(o.order_no)
    o.reschedule_notice = undefined
    uni.showToast({ title: '已知悉', icon: 'success' })
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

/** 服务方无法接单：订单关闭并自动退款，不会回到接单池。弹层与任务列表共用同一组件 */
const rejectOpen = ref(false)

function onReject() {
  if (!order.value || acting.value) return
  rejectOpen.value = true
}

async function confirmReject(reason: string) {
  if (!order.value || acting.value) return
  acting.value = true
  try {
    await rejectOrder(order.value.order_no, reason || '服务方无暇接单')
    // 先关弹层再提示，否则 toast 会被遮罩挡住
    rejectOpen.value = false
    uni.showToast({ title: '已退回', icon: 'success' })
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

/**
 * 「完成服务」先开一层（可选拍照），确认后再落库。
 * 单次垃圾单推进到终态；月卡只盖今天那条核销记录的完成时间，不动服务期。
 * 与任务列表同一个组件、同一段顺序：先传照片，再改状态。
 */
const finishOpen = ref(false)
/** 这次完成的是月卡当日（true）还是单次订单（false） */
const finishMonthly = ref(false)
/** 单次订单完成后推进到哪个节点：垃圾=completed（终态），遛宠=pending_confirm（等住户确认） */
const finishNode = ref<'completed' | 'pending_confirm'>('completed')
/** 完成弹层按服务区分文案 */
const finishService = ref<'garbage' | 'dogwalk'>('garbage')

/** 弹层句柄：失败时把已上传成功的从待传列表移除，重试只传剩下的 */
const finishSheet = ref<{ dropUploaded: (n: number) => void } | null>(null)
const startSheet = ref<{ dropUploaded: (n: number) => void } | null>(null)

function openFinish(monthly: boolean) {
  if (acting.value) return
  finishMonthly.value = monthly
  finishOpen.value = true
}

async function confirmFinish(photos: string[]) {
  const o = order.value
  if (!o || acting.value) return
  acting.value = true
  let done = 0
  try {
    // 逐张上传；中途失败时已传的不重复传（弹层列表会去掉已传的）
    for (const p of photos) {
      await uploadRecord(o.order_no, p, 'photo', false, 'finish')
      done++
    }
    if (finishMonthly.value) {
      await providerMonthlyFinish(o.order_no)
      // 月卡状态不变，只刷新今日进度；重拉整页会闪一下
      monthly.value = await monthlyDetail(o.order_no).catch(() => monthly.value)
    } else {
      await advanceOrder(o.order_no, finishNode.value)
    }
    finishOpen.value = false
    // 遛宠完成进的是「待住户确认」，不是终态，toast 要说清下一步在哪
    uni.showToast({
      title:
        finishNode.value === 'pending_confirm'
          ? '已提交待确认'
          : photos.length
            ? '已留证并完成'
            : '已完成',
      icon: 'success',
    })
    await load()
  } catch (e) {
    // 弹层不关：已传成功的从列表里去掉，节点没推进，原地重试不重复上传
    finishSheet.value?.dropUploaded(done)
    uni.showModal({
      title: '完成失败',
      content: e instanceof ApiError ? e.message : '请稍后重试',
      showCancel: false,
    })
  } finally {
    acting.value = false
  }
}

/**
 * 遛宠「开始服务」：先拍上门凭证（必填）再推进到服务中，与任务列表同一套弹层。
 * 顺序同样是先上传后推进，理由同 confirmFinish。
 */
const startOpen = ref(false)

async function confirmStart(photos: string[], consent: boolean) {
  const o = order.value
  if (!o || acting.value) return
  acting.value = true
  let done = 0
  try {
    // 先传凭证再推进，逐张上传；中途失败时已传的不重复传
    for (const p of photos) {
      await uploadRecord(o.order_no, p, 'photo', consent, 'start')
      done++
    }
    await advanceOrder(o.order_no, 'in_service')
    startOpen.value = false
    uni.showToast({ title: '已开始服务', icon: 'success' })
    await load()
  } catch (e) {
    // 弹层不关：已传成功的从列表里去掉，节点没推进，原地重试不重复上传
    startSheet.value?.dropUploaded(done)
    uni.showModal({
      title: '开始失败',
      content: e instanceof ApiError ? e.message : '请稍后重试',
      showCancel: false,
    })
  } finally {
    acting.value = false
  }
}

async function onAdvance(node: string) {
  const o = order.value
  if (!o || acting.value) return
  if (node === 'completed' && o.service_type === 'garbage') {
    finishMonthly.value = false
    finishNode.value = 'completed'
    finishService.value = 'garbage'
    finishOpen.value = true
    return
  }
  // 遛宠完成服务：弹层支持上传服务凭证（送回宠物的照片）
  if (node === 'pending_confirm' && o.service_type === 'dogwalk') {
    finishMonthly.value = false
    finishNode.value = 'pending_confirm'
    finishService.value = 'dogwalk'
    finishOpen.value = true
    return
  }
  // 遛宠开始服务必须先上传凭证
  if (node === 'in_service' && o.service_type === 'dogwalk') {
    startOpen.value = true
    return
  }
  acting.value = true
  try {
    await advanceOrder(o.order_no, node)
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
    await uploadRecord(orderNo, path, 'photo', ownerConsented.value)
    uni.showToast({ title: '已上传', icon: 'success' })
    records.value = await listRecords(orderNo)
  } catch (e) {
    uni.showToast({ title: e instanceof ApiError ? e.message : '上传失败', icon: 'none' })
  } finally {
    uploading.value = false
  }
}

/** 能否取消 */
/** 未接单也能点，用来提示「接单后才显示联系方式」；取消/被拒的订单没必要展示 */
const contactable = computed(() => {
  const s = order.value?.status
  return !!s && s !== 'canceled' && s !== 'rejected' && s !== 'pending_payment'
})

function onContactProvider() {
  if (!order.value) return
  contactProvider(order.value.order_no)
}

/** 服务报告：从节点事件里取开始/结束打卡时间，配合影像数量聚合 */
const reportCard = computed(() => {
  const o = order.value
  if (!o || o.service_type !== 'dogwalk') return null
  const find = (node: string) => o.nodes?.find((n) => n.node === node && n.reached)?.at || ''
  const start = find('in_service')
  const end = find('pending_confirm')
  if (!start && !end) return null
  return { start, end }
})

const cancellable = computed(() => {
  const s = order.value?.status
  return s === 'pending_payment' || s === 'pending_accept' || s === 'accepted'
})

/** 进度条宽度，total 为 0 时按 0% 处理避免除零 */
function pct(used: number, total: number): string {
  if (!total) return '0%'
  return `${Math.min(100, Math.round((used / total) * 100))}%`
}
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

      <!-- 住户改期立即生效，卡片上的时间会直接换掉；这里把「从几点改到几点」讲清楚，
           否则服务方很可能按旧时间出门。点「我知道了」才收起。 -->
      <view v-if="order.reschedule_notice" class="card rs-notice-card">
        <view class="rs-nt-head">
          <text class="rs-nt-title">住户修改了服务时间</text>
          <text class="rs-nt-tag">请按新时间上门</text>
        </view>
        <view class="rs-nt-time">
          <text class="rs-nt-from">{{ noticeFrom(order) }}</text>
          <text class="rs-nt-arrow">→</text>
          <text class="rs-nt-to">{{ noticeTo(order) }}</text>
        </view>
        <view v-if="order.reschedule_notice.reason" class="rs-nt-reason">
          住户说明：{{ order.reschedule_notice.reason }}
        </view>
        <view class="rs-nt-tip">如新时间无法履约，可点下方「改期」与住户重新协商</view>
        <view class="rs-nt-ack" @tap="onAckReschedule">
          {{ acting ? '处理中…' : '我知道了' }}
        </view>
      </view>

      <!-- 服务方发起的改期待确认：住户在这里就能处理，不用打电话确认 -->
      <view v-if="order.pending_reschedule" class="card rs-pending">
        <view class="rs-pd-head">
          <text class="rs-pd-title">服务方申请改期</text>
          <text class="rs-pd-tag">待确认</text>
        </view>
        <view class="rs-pd-time">
          <text class="rs-pd-from">{{ dateSlot(order.pending_reschedule.from_date, order.pending_reschedule.from_slot) }}</text>
          <text class="rs-pd-arrow">→</text>
          <text class="rs-pd-to">{{ dateSlot(order.pending_reschedule.to_date, order.pending_reschedule.to_slot) }}</text>
        </view>
        <view v-if="order.pending_reschedule.reason" class="rs-pd-reason">
          原因：{{ order.pending_reschedule.reason }}
        </view>
        <view v-if="!isProvider" class="rs-pd-actions">
          <view class="btn-ghost half" @tap="onRespondReschedule(false)">保持原时间</view>
          <view class="btn-primary half" @tap="onRespondReschedule(true)">
            {{ acting ? '处理中…' : '同意改期' }}
          </view>
        </view>
        <view v-else class="rs-pd-hint">已提交，等待住户确认；确认前仍按原时间服务</view>
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

      <!-- 包月核销进度 -->
      <view v-if="monthly" class="card">
        <view class="row">
          <view class="section-title">包月服务进度</view>
          <text class="monthly-range">{{ monthly.start_date }} 至 {{ monthly.end_date }}</text>
        </view>
        <view class="monthly-stat">
          <view class="mstat">
            <view class="mnum">{{ monthly.used }}</view>
            <view class="mlabel">已使用</view>
          </view>
          <view class="mstat">
            <view class="mnum highlight">{{ monthly.remaining }}</view>
            <view class="mlabel">剩余次数</view>
          </view>
          <view class="mstat">
            <view class="mnum">{{ monthly.total }}</view>
            <view class="mlabel">周期总额</view>
          </view>
        </view>
        <!-- 住户和月卡服务方都最关心这一句：今天到底做完没有 -->
        <view class="mtoday" :class="{ done: monthly.today_finished }">
          <text class="mtoday-dot" />
          <text>{{ monthlyToday }}</text>
        </view>
        <view class="mbar">
          <view class="mbar-fill" :style="{ width: pct(monthly.used, monthly.total) }" />
        </view>
        <view class="mhint">
          当日未使用的次数不累计、不顺延，过期作废
        </view>
        <template v-if="monthly.redemptions?.length">
          <view class="divider" />
          <view class="section-sub">核销记录</view>
          <view v-for="r in monthly.redemptions" :key="r.id" class="red-row">
            <text class="red-date">{{ r.service_date }}</text>
            <text class="red-remark">{{ redemptionNote(r) }}</text>
          </view>
        </template>
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
            {{ order.address.community }} {{ [order.address.lane, order.address.house_no].filter(Boolean).join(' ') }}
          </text>
        </view>
        <view v-if="addressContact" class="info-row">
          <text class="info-label">联系人</text>
          <text class="info-value">{{ addressContact }}</text>
        </view>
        <view v-if="hasAccessInfo" class="info-row">
          <text class="info-label">进入方式</text>
          <view class="info-value access-value">
            <text>{{ entryText }}</text>
            <text v-if="order.access_info" class="copy-link" @tap.stop="copyAccess">复制</text>
          </view>
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
        <!-- 完成时间：后端已下发 finished_at，之前接口漏了字段所以一直没得展示 -->
        <view v-if="order.finished_at" class="info-row">
          <text class="info-label">完成时间</text>
          <text class="info-value">{{ order.finished_at.slice(0, 16).replace('T', ' ') }}</text>
        </view>
      </view>

      <!-- 改期记录：订单主表只留最新时间，从哪改到哪靠这里还原 -->
      <view v-if="appliedReschedules.length" class="card">
        <view class="section-title">改期记录</view>
        <view v-for="r in appliedReschedules" :key="r.id" class="rs-log">
          <view class="rs-log-time">
            <text class="rs-log-from">{{ dateSlot(r.from_date, r.from_slot) }}</text>
            <text class="rs-log-arrow">→</text>
            <text class="rs-log-to">{{ dateSlot(r.to_date, r.to_slot) }}</text>
          </view>
          <view class="rs-log-meta">
            {{ r.initiated_by === 'provider' ? '服务方申请' : '住户修改' }}
            <text v-if="r.reason"> · {{ r.reason }}</text>
          </view>
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
        <label v-if="canUploadRecord" class="consent-row">
          <checkbox class="consent-box" :checked="ownerConsented" @tap="ownerConsented = !ownerConsented" />
          <text class="consent-text">已征得主人同意拍摄</text>
        </label>
        <view v-if="!records.length" class="hint">还没有记录，服务过程中拍照上传</view>
        <!-- 凭证按动作分组：开始/完成各一块；纯过程记录（垃圾单、历史数据）不分组 -->
        <template v-for="g in recordGroups" :key="g.key">
          <view v-if="g.label" class="record-stage">{{ g.label }}</view>
          <view class="record-grid">
            <view v-for="r in g.list" :key="r.id" class="record" @tap="previewImage(r)">
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
        </template>
        <view class="hint">记录由服务方上传，不可删除</view>
      </view>

            <!-- 服务报告：聚合打卡时间与影像（流程图「结束打卡=生成报告」） -->
      <view
        v-if="order.service_type === 'dogwalk' && reportCard"
        class="card"
      >
        <view class="section-title">服务报告</view>
        <view class="report-row">
          <text class="report-label">开始打卡</text>
          <text class="report-value">{{ reportCard.start || '—' }}</text>
        </view>
        <view class="report-row">
          <text class="report-label">结束打卡</text>
          <text class="report-value">{{ reportCard.end || '—' }}</text>
        </view>
        <view class="report-row">
          <text class="report-label">影像记录</text>
          <text class="report-value">{{ records.length }} 份</text>
        </view>
        <view v-if="order.pet_notes" class="report-row">
          <text class="report-label">宠物情况</text>
          <text class="report-value">{{ order.pet_notes }}</text>
        </view>
        <view class="hint">报告由打卡时间与影像自动生成</view>
      </view>

      <!-- 宠物备注与拒单/退款信息：服务方接单前必须核对的交接要求 -->
      <view v-if="order.pet_notes || order.cancel_reason" class="card">
        <view v-if="order.pet_notes" class="pet-row">
          <view class="f-label">宠物备注</view>
          <view class="pet-text">{{ order.pet_notes }}</view>
        </view>
        <view v-if="order.cancel_reason" class="pet-row">
          <view class="f-label">{{ order.status === 'rejected' ? '拒单原因' : '取消原因' }}</view>
          <view class="pet-text">{{ order.cancel_reason }}</view>
        </view>
        <view v-if="(order.refunded_amount || 0) > 0" class="refund-row">
          已退款 ¥{{ ((order.refunded_amount || 0) / 100).toFixed(2) }}，原路退回
        </view>
      </view>

<!-- 操作：服务方推进节点，住户支付/取消/确认 -->
      <view v-if="order.status !== 'canceled' && order.status !== 'rejected'" class="bottom-bar">
        <template v-if="isProvider">
          <view
            v-if="order.status === 'accepted' && !servedMonthlyToday"
            class="btn-ghost half"
            @tap="onReject"
          >
            无法接单
          </view>
          <!-- 遛宠在已接单阶段给一个改期入口：只能申请，生效要等住户确认 -->
          <view v-if="canRequestReschedule" class="btn-ghost half" @tap="openReschedule">
            修改时间
          </view>
          <!-- 已完成/待确认：不论是不是月卡，服务方这里都没有可做的动作 -->
          <view v-if="order.status === 'pending_confirm'" class="btn-ghost half">
            等待住户确认
          </view>
          <view v-else-if="order.status === 'completed'" class="btn-ghost half">服务已完成</view>
          <!-- 月卡：服务方上门当天先打卡，再点「完成服务」收尾这一趟 -->
          <view
            v-else-if="monthly && monthly.in_period && !monthly.today_done"
            class="btn-primary half"
            @tap="onRedeemMonthly"
          >
            {{ acting ? '处理中…' : '出发上门' }}
          </view>
          <view
            v-else-if="monthlyNeedFinish"
            class="btn-primary half"
            @tap="openFinish(true)"
          >
            {{ acting ? '处理中…' : '完成服务' }}
          </view>
          <view v-else-if="monthly && monthly.today_finished" class="btn-ghost half">
            今日已服务
          </view>
          <view v-else-if="monthly" class="btn-ghost half">
            {{ monthly.ended ? '服务期已结束' : '今日不在服务期' }}
          </view>
          <view v-else-if="order.plan === 'monthly'" class="btn-ghost half">打卡信息加载失败</view>
          <view
            v-for="a in providerActions"
            :key="a.node"
            class="btn-primary half"
            @tap="onAdvance(a.node)"
          >
            {{ acting ? '处理中…' : a.label }}
          </view>
          <view
            v-if="!providerActions.length && order.plan !== 'monthly' && !isClosed"
            class="btn-ghost half"
          >
            等待住户确认
          </view>
        </template>
        <template v-else>
          <view v-if="contactable" class="btn-ghost half" @tap="onContactProvider">联系服务方</view>
          <view v-if="cancellable" class="btn-ghost half" @tap="onCancel">取消订单</view>
          <view v-if="canReschedule" class="btn-ghost half" @tap="openReschedule">修改时间</view>
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

    <!-- 拒单弹层：与任务列表共用，保证两个入口的文案和样式一致 -->
    <RejectSheet
      :visible="rejectOpen"
      :loading="acting"
      @close="rejectOpen = false"
      @confirm="confirmReject"
    />

    <!-- 完成服务弹层：垃圾/遛宠文案不同，凭证选填，不拍也能完成 -->
    <FinishSheet
      :visible="finishOpen"
      :loading="acting"
      :service="finishService"
      ref="finishSheet"
      @close="finishOpen = false"
      @confirm="confirmFinish"
    />

    <!-- 遛宠开始服务弹层：上门凭证必填，同一个组件的 start 模式 -->
    <FinishSheet
      :visible="startOpen"
      :loading="acting"
      mode="start"
      ref="startSheet"
      @close="startOpen = false"
      @confirm="confirmStart"
    />

    <!-- 改期弹层：住户改完立即生效，服务方提交后等人确认 -->
    <RescheduleSheet
      :visible="rescheduleOpen"
      :loading="acting"
      :mode="rescheduleMode"
      :service-type="rescheduleFrom.serviceType"
      :from-date="rescheduleFrom.date"
      :from-slot="rescheduleFrom.slot"
      @close="rescheduleOpen = false"
      @confirm="onReschedule"
    />
  </view>
</template>

<style scoped>
/* ---------- 改期 ---------- */
/* 住户改期通知：服务方视角才有。用偏红的暖色而不是品牌绿——
   绿代表「一切正常」，而这条的意思是「你记的时间已经作废」。 */
.rs-notice-card {
  border: 1rpx solid #fed7c3;
  background: #fef3ec;
}

.rs-nt-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12rpx;
}

.rs-nt-title {
  font-size: 29rpx;
  font-weight: 600;
  color: #c2410c;
}

.rs-nt-tag {
  font-size: 21rpx;
  color: #c2410c;
  background: #ffe4d3;
  padding: 4rpx 14rpx;
  border-radius: 8rpx;
  white-space: nowrap;
}

.rs-nt-time {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12rpx;
  margin-top: 16rpx;
  font-size: 27rpx;
}

.rs-nt-from {
  color: #b08a72;
  text-decoration: line-through;
}

.rs-nt-arrow {
  color: #c2410c;
}

.rs-nt-to {
  color: #9a3412;
  font-weight: 600;
}

.rs-nt-reason {
  margin-top: 12rpx;
  font-size: 23rpx;
  color: #9a3412;
  line-height: 1.6;
}

.rs-nt-tip {
  margin-top: 12rpx;
  font-size: 22rpx;
  color: #b08a72;
  line-height: 1.6;
}

.rs-nt-ack {
  margin-top: 18rpx;
  height: 76rpx;
  line-height: 76rpx;
  text-align: center;
  border-radius: 38rpx;
  background: #c2410c;
  color: #fff;
  font-size: 27rpx;
  font-weight: 600;
}

/* 待确认申请用暖黄底：这是需要住户行动的一件事，不能被当成普通信息卡划过去 */
.rs-pending {
  border: 1rpx solid #fde68a;
  background: #fffbeb;
}

.rs-pd-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.rs-pd-title {
  font-size: 29rpx;
  font-weight: 600;
  color: #92400e;
}

.rs-pd-tag {
  font-size: 21rpx;
  color: #b45309;
  background: #fef3c7;
  padding: 4rpx 14rpx;
  border-radius: 8rpx;
}

.rs-pd-time {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12rpx;
  margin-top: 16rpx;
  font-size: 26rpx;
}

.rs-pd-from {
  color: var(--text-light);
  text-decoration: line-through;
}

.rs-pd-arrow {
  color: #b45309;
}

.rs-pd-to {
  color: var(--text);
  font-weight: 600;
}

.rs-pd-reason {
  margin-top: 10rpx;
  font-size: 24rpx;
  line-height: 1.5;
  color: var(--text-sub);
}

.rs-pd-actions {
  display: flex;
  gap: 20rpx;
  margin-top: 22rpx;
}

/* 全局 btn-ghost 是 88rpx、btn-primary 是 92rpx，并排会一高一矮 */
.rs-pd-actions .btn-ghost {
  height: 92rpx;
  line-height: 92rpx;
}

.rs-pd-hint {
  margin-top: 14rpx;
  font-size: 23rpx;
  color: var(--text-light);
}

.rs-log {
  padding: 14rpx 0;
  border-bottom: 1rpx solid #f6f7f8;
}

.rs-log:last-child {
  border-bottom: none;
}

.rs-log-time {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10rpx;
  font-size: 25rpx;
}

.rs-log-from {
  color: var(--text-light);
}

.rs-log-arrow {
  color: var(--text-light);
}

.rs-log-to {
  color: var(--text);
  font-weight: 500;
}

.rs-log-meta {
  margin-top: 8rpx;
  font-size: 22rpx;
  line-height: 1.5;
  color: var(--text-light);
}

.copy-link {
  font-size: 22rpx;
  color: #2f7a5f;
  padding: 2rpx 12rpx;
  border: 1rpx solid #cfe5da;
  border-radius: 8rpx;
  flex-shrink: 0;
}

.report-row {
  display: flex;
  justify-content: space-between;
  gap: 20rpx;
  padding: 14rpx 0;
}

.report-label { font-size: 25rpx; color: var(--text-sub); flex-shrink: 0; }
.report-value { font-size: 25rpx; color: var(--text); text-align: right; word-break: break-all; }

.consent-row {
  display: flex;
  align-items: center;
  gap: 10rpx;
  margin-bottom: 16rpx;
}
.consent-box { transform: scale(0.8); }
.consent-text { font-size: 24rpx; color: var(--text-sub); }
.f-label {
  font-size: 26rpx;
  font-weight: 600;
  color: var(--text-sub);
  margin-bottom: 10rpx;
}

.pet-row { margin-bottom: 18rpx; }
.pet-text { font-size: 27rpx; color: var(--text); line-height: 1.6; }
.refund-row {
  padding: 14rpx 20rpx;
  font-size: 24rpx;
  color: var(--brand-dark);
  background: var(--brand-light);
  border-radius: 12rpx;
}
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

/* 进入方式这一行要放「复制」按钮，单独开一个 flex 容器，
   不动 .info-value 的右对齐（其它行还在用） */
.access-value {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 12rpx;
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

/* ---------- 包月核销进度 ---------- */
.monthly-range {
  font-size: 21rpx;
  color: var(--text-light);
}

.monthly-stat {
  display: flex;
  margin: 8rpx 0 20rpx;
}

.mstat {
  flex: 1;
  text-align: center;
}

.mnum {
  font-size: 42rpx;
  font-weight: 800;
  color: var(--text);
  line-height: 1.2;
}

.mnum.highlight {
  color: var(--brand);
}

.mlabel {
  margin-top: 6rpx;
  font-size: 21rpx;
  color: var(--text-light);
}

.mbar {
  height: 10rpx;
  border-radius: 6rpx;
  background: #eef2f1;
  overflow: hidden;
}

/* 今日状态：默认橙色（还没上门），打卡后转绿。
   和列表页月卡状态带用同一套颜色语义，避免两处判断标准不一致 */
.mtoday {
  display: flex;
  align-items: center;
  gap: 10rpx;
  margin: 18rpx 0 12rpx;
  padding: 12rpx 18rpx;
  border-radius: 12rpx;
  background: #fff7ed;
  border: 1rpx solid #fed7aa;
  font-size: 24rpx;
  font-weight: 500;
  color: #c2410c;
}

.mtoday.done {
  background: #ecfdf5;
  border-color: #a7f3d0;
  color: #047857;
}

.mtoday-dot {
  width: 12rpx;
  height: 12rpx;
  border-radius: 6rpx;
  background: #f59e0b;
}

.mtoday.done .mtoday-dot {
  background: #10b981;
}

.mbar-fill {
  height: 100%;
  border-radius: 6rpx;
  background: linear-gradient(90deg, #4cb3aa 0%, var(--brand) 100%);
  transition: width 0.3s ease;
}

.mhint {
  margin-top: 14rpx;
  font-size: 21rpx;
  color: var(--text-light);
}

.section-sub {
  font-size: 25rpx;
  font-weight: 600;
  color: var(--text-sub);
  margin: 16rpx 0 10rpx;
}

.red-row {
  display: flex;
  justify-content: space-between;
  gap: 20rpx;
  padding: 14rpx 0;
  border-bottom: 1rpx solid #f6f7f8;
  font-size: 24rpx;
}

.red-row:last-child {
  border-bottom: none;
}

.red-date {
  color: var(--text);
  flex-shrink: 0;
}

.red-remark {
  color: var(--text-light);
  text-align: right;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
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

/* 凭证分组小标题：比正文重一点、比卡片标题轻，只做视觉分隔 */
.record-stage {
  margin: 20rpx 0 12rpx;
  font-size: 24rpx;
  font-weight: 500;
  color: var(--text-sub);
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
