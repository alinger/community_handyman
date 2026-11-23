<script setup lang="ts">
import { computed, ref } from 'vue'
import { onReachBottom, onShow } from '@dcloudio/uni-app'
import TabBar from '@/components/TabBar.vue'
import RejectSheet from '@/components/RejectSheet.vue'
import RescheduleSheet from '@/components/RescheduleSheet.vue'
import FinishSheet from '@/components/FinishSheet.vue'
import {
  acceptOrder,
  advanceOrder,
  ackProviderReschedule,
  providerIncome,
  providerMonthlyFinish,
  providerMonthlyRedeem,
  providerMyOrders,
  providerOpenOrders,
  providerReschedule,
  rejectOrder,
  uploadRecord,
  type Order,
  type ProviderIncome,
} from '@/api'
import { useUserStore } from '@/store/user'
import { ensureAuth } from '@/utils/auth'
import { friendlyDate, price, SERVICE_LABEL, slotTextOf } from '@/utils/format'
import { ApiError } from '@/utils/request'

const user = useUserStore()

const tabs = [
  { key: 'pending', label: '待服务' },
  { key: 'processing', label: '服务中' },
  { key: 'completed', label: '已完成' },
]
const activeTab = ref('pending')

const orders = ref<Order[]>([])

/**
 * 按服务种类分组展示。
 * 三类服务的履约动作完全不同（垃圾看门禁密码、遛宠看宠物备注、陪诊看医院），
 * 混在一张列表里要逐条读标题才知道这单该带什么，很容易拿错。
 * 顺序固定 garbage → dogwalk → accompany，不按条数排：
 * 位置每天变来变去，服务方反而找不到自己的单。
 */
const SERVICE_ORDER = ['garbage', 'dogwalk', 'accompany']

const groups = computed(() => {
  const map = new Map<string, Order[]>()
  for (const o of orders.value) {
    const arr = map.get(o.service_type)
    if (arr) arr.push(o)
    else map.set(o.service_type, [o])
  }
  const keys = SERVICE_ORDER.filter((k) => map.has(k))
  for (const k of map.keys()) {
    if (!SERVICE_ORDER.includes(k)) keys.push(k)
  }
  return keys.map((k) => ({
    key: k,
    label: SERVICE_LABEL[k] || k,
    list: map.get(k) as Order[],
  }))
})
/**
 * 各 tab 的订单数，用于标签角标。
 * 后端 tab 映射：pending = 待接单+已接单+待上门；processing = 服务中+待确认；completed = 已完成。
 * 列表接口本来就返回 total，这里复用同一接口（size=1 只取 total），不额外加后端接口。
 */
const counts = ref<Record<string, number>>({ pending: 0, processing: 0, completed: 0 })
/** 累计收入：来自已完成订单的自动入账 */
const income = ref<ProviderIncome | null>(null)
const openCount = ref(0)
const loading = ref(false)
const finished = ref(false)
const page = ref(1)
const acting = ref(false)

async function loadIncome() {
  try {
    income.value = await providerIncome()
  } catch {
    // 收入拉取失败不阻断列表加载
  }
}

onShow(async () => {
  if (!ensureAuth()) return
  await user.ensureLogin()
  // 权限校验：未通过认证的用户误入时提示去申请
  if (!user.isProvider) {
    uni.showModal({
      title: '还没有服务方权限',
      content: '请先在「我的 - 成为服务方」提交资质认证，通过后即可接单',
      showCancel: false,
      success: () => uni.reLaunch({ url: '/pages/index/index' }),
    })
    return
  }
  // 有权限但从住户视角点进来：顺手把视角切过去，避免底部导航与页面内容不一致
  if (!user.isProviderView) user.switchView('provider')
  await Promise.all([reload(), loadOpenCount(), loadCounts()])
  await loadIncome()
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
    const res = await providerMyOrders(activeTab.value, page.value, 10)
    orders.value = page.value === 1 ? res.list : [...orders.value, ...res.list]
    if (page.value === 1) counts.value[activeTab.value] = res.total
    if (orders.value.length >= res.total) finished.value = true
  } catch (e) {
    uni.showToast({ title: '加载失败', icon: 'none' })
  } finally {
    loading.value = false
  }
}

/** 三个 tab 的订单数并行拉取；任一个失败只影响它自己的角标，不阻断列表 */
async function loadCounts() {
  const keys = tabs.map((t) => t.key)
  const res = await Promise.all(
    keys.map((k) => providerMyOrders(k, 1, 1).catch(() => null)),
  )
  keys.forEach((k, i) => {
    const r = res[i]
    if (r) counts.value[k] = r.total
  })
}

/** 已完成不给角标：那是历史累计，天天挂个数字没有行动含义 */
function badgeCount(key: string): number {
  if (key === 'completed') return 0
  return counts.value[key] || 0
}

async function loadOpenCount() {
  try {
    const res = await providerOpenOrders(1, 1)
    openCount.value = res.total
  } catch {
    openCount.value = 0
  }
}

function switchTab(key: string) {
  if (activeTab.value === key) return
  activeTab.value = key
  reload()
}

function goPool() {
  uni.navigateTo({ url: '/pages/provider/pool' })
}

function goDetail(orderNo: string) {
  uni.navigateTo({ url: `/pages/order/detail?orderNo=${orderNo}` })
}

function slotLabel(code: string): string {
  return slotTextOf(code)
}

/*
 * 月卡（包月代倒垃圾）：一张订单跑 30 天，状态一直停在「已受理」。
 * 所以卡片上的「今天上门没有」不能看 status，只能看后端下发的 monthly。
 */

/** 月卡今日状态色调，决定卡片上那条提示的颜色与动作 */
function monthlyTone(o: Order): string {
  const m = o.monthly
  if (!m) return ''
  // 订单已经提交确认/完成：服务期内的「今天」不再有意义，一律走灰色
  if (o.status === 'pending_confirm' || o.status === 'completed') return 'ended'
  if (m.ended) return 'ended'
  if (!m.in_period) return 'out'
  if (m.today_finished) return 'done'
  // 到了门口但还没点完成：介于「待上门」与「已服务」之间的中间态，用蓝色区分开，
  // 免得服务方以为这趟已经收尾、掉头就走。
  return m.today_done ? 'doing' : 'todo'
}

/** 卡片右上角那一眼：月卡说今天完成没有，比「已受理」有信息量得多 */
function monthlyHead(o: Order): string {
  const m = o.monthly
  if (!m) return ''
  if (o.status === 'pending_confirm') return '待住户确认'
  if (o.status === 'completed') return '服务已完成'
  if (m.ended) return '待结算'
  if (!m.in_period) return '未在服务期'
  if (m.today_finished) return '今日已服务'
  return m.today_done ? '服务中' : '今日待上门'
}

function monthlyText(o: Order): string {
  const m = o.monthly
  if (!m) return ''
  if (o.status === 'pending_confirm') return '服务已提交，等住户确认'
  if (o.status === 'completed') return '本期服务已结束'
  if (m.ended) return '服务期已结束，可提交完成'
  if (!m.in_period) return '今天不在服务期内'
  if (!m.today_done) return '今天还没上门'
  const at = clockOf(m.today_at)
  if (m.today_finished) {
    const done = clockOf(m.today_finished_at) || at
    return done ? `今日已服务 ${done}` : '今日已服务'
  }
  return at ? `服务中，${at} 上门` : '服务中，还没点完成服务'
}

/** 已服务天数与到期日：月卡要跟着走 30 天，随时能看出还剩多少 */
function monthlyMeta(o: Order): string {
  const m = o.monthly
  if (!m) return ''
  return `已服务 ${m.used}/${m.total} 天 · 至 ${shortDate(m.end_date)}`
}

/** 服务期起止，月卡没有「某一天的服务时间」，只有一段区间 */
function monthlyRange(o: Order): string {
  const m = o.monthly
  if (!m) return ''
  return `${shortDate(m.start_date)} - ${shortDate(m.end_date)}`
}

/** 2026-12-21 → 12/21，卡片上不需要年份 */
function shortDate(d: string): string {
  if (!d) return ''
  return d.slice(5).replace('-', '/')
}

/** 打卡时间只显示时分：服务方只关心今天几点上的门 */
function clockOf(iso?: string): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}

/** 订单已走到「等住户确认/已完成」：服务方这边不该再出现打卡或结算按钮 */
function isClosed(o: Order): boolean {
  return o.status === 'completed' || o.status === 'pending_confirm'
}

/** 月卡今天还能不能打卡：订单没终结、今天在服务期内、且今天还没打过 */
function canRedeem(o: Order): boolean {
  const m = o.monthly
  if (!m || isClosed(o)) return false
  return m.in_period && !m.today_done
}

/**
 * 月卡今天这趟还要不要收尾。
 * 打卡只证明人到了，袋子提走没有得再点一下「完成服务」——
 * 少了这一步，服务方点完「出发上门」当天就算服务完成，住户也立刻看到「今日已服务」。
 */
function canFinishMonthly(o: Order): boolean {
  const m = o.monthly
  if (!m || isClosed(o)) return false
  return m.in_period && m.today_done && !m.today_finished
}

/** 今天这一趟已经收尾。包月看当天的核销记录，不看 status——它一直停在已受理 */
function servedToday(o: Order): boolean {
  return !!o.monthly?.today_finished && !isClosed(o)
}

/**
 * 今天已经上过门（打过卡）。
 * 「无法接单」用它而不是 servedToday：人都到门口了还能点拒单说不通，
 * 哪怕这趟还没点完成服务。
 */
function arrivedToday(o: Order): boolean {
  return !!o.monthly?.today_done && !isClosed(o)
}

const redeeming = ref('')

/**
 * 月卡今日打卡（「出发上门」）。
 * 打完卡这一趟就进入「服务中」：人已经到了，等着把袋子提走后点「完成服务」。
 * 卡片会从「待服务」挪走，所以顺手切到「服务中」——不切的话服务方看着卡片
 * 凭空消失，还得自己猜去哪了。跨天没有核销记录时自动回到「待服务」。
 */
async function onRedeem(o: Order) {
  if (redeeming.value || o.monthly?.today_done) return
  redeeming.value = o.order_no
  try {
    await providerMonthlyRedeem(o.order_no)
    activeTab.value = 'processing'
    uni.showToast({ title: '已进入服务中', icon: 'success' })
    await Promise.all([reload(), loadCounts()])
  } catch (e) {
    uni.showModal({
      title: '打卡失败',
      content: e instanceof ApiError ? e.message : '请稍后重试',
      showCancel: false,
    })
  } finally {
    redeeming.value = ''
  }
}

/**
 * 按当前状态给出合法的推进动作。
 * 后端 CanTransit 是白名单，这里只展示界面入口，
 * 真正的合法性仍由后端兜底（防重复点击、非法跳跃都返回 2003）。
 *
 * 两类服务的节奏不同：
 * - 垃圾代倒：出发即到场（accepted → 服务中），完成即终态（service → completed），两步结束。
 * - 遛宠/陪诊：出发 → 到场 → 完成待确认，住户看完再点头，中间态不能被跳过。
 */
function actions(o: Order): { node: string; label: string; primary: boolean }[] {
  const acts: { node: string; label: string; primary: boolean }[] = []
  // 月卡服务期内不给「出发上门/开始服务」：点一下就推进节点会让月卡提前跑完。
  // 服务期结束后才给一个「提交完成」，一次把订单推到待确认结算。
  if (o.monthly) {
    // 已完成/待确认的月卡不再给结算入口，否则会重复提交被后端拦住
    const closed = o.status === 'completed' || o.status === 'pending_confirm'
    if (o.monthly.ended && !closed) {
      acts.push({ node: 'monthly_finish', label: '提交完成', primary: true })
    }
    return acts
  }
  // 垃圾代倒：一次上门就结束。点「出发上门」直接进服务中，
  // 到了再点「完成服务」就是终态——袋子提走这单就完了，没有「住户验收」这一步。
  // 遛宠/陪诊仍要两步：它们有送回宠物、陪诊交接，住户得看一眼再确认。
  if (o.service_type === 'garbage') {
    if (o.status === 'accepted') {
      acts.push({ node: 'in_service', label: '出发上门', primary: true })
    } else if (o.status === 'on_the_way') {
      // 旧版本留在「待上门」的单：给「开始服务」，点完就与新流程一致
      acts.push({ node: 'in_service', label: '开始服务', primary: true })
    } else if (o.status === 'in_service') {
      acts.push({ node: 'completed', label: '完成服务', primary: true })
    }
    return acts
  }
  if (o.status === 'accepted') {
    acts.push({ node: 'on_the_way', label: '出发上门', primary: true })
  } else if (o.status === 'on_the_way') {
    acts.push({ node: 'in_service', label: '开始服务', primary: true })
  } else if (o.status === 'in_service') {
    acts.push({ node: 'pending_confirm', label: '完成服务', primary: true })
  }
  return acts
}

/**
 * 无法接单。拒单会把订单置为终态并自动退款给住户，
 * 单子不会回到接单池——所以必须把后果说在弹层里，且原因会落到 cancel_reason 给住户看。
 * 弹层是自定义组件（原生 showModal 的 editable 在微信端会把说明文字塞进输入框、还只有四行高）。
 */
const rejectTarget = ref<Order | null>(null)

function onReject(o: Order) {
  if (acting.value) return
  rejectTarget.value = o
}

async function confirmReject(reason: string) {
  const o = rejectTarget.value
  if (!o || acting.value) return
  acting.value = true
  try {
    await rejectOrder(o.order_no, reason || '服务方无暇接单')
    // 先关弹层再提示，否则 toast 会被遮罩挡住
    rejectTarget.value = null
    uni.showToast({ title: '已退回', icon: 'success' })
    await Promise.all([reload(), loadCounts()])
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
 * 完成服务前先弹一层（可选留一张服务凭证），再由 confirmFinish 真正落库。
 *
 * 四种单都走这里：
 * - 单次垃圾单：推进到 completed（终态，住户不再确认）
 * - 遛宠单：推进到 pending_confirm，住户看完服务记录再确认
 * - 月卡当日：给当天核销记录盖完成时间（订单状态不变，卡片从待服务挪到已完成）
 * - 月卡到期结算：仍走「提交完成」三步，不经过这里
 * 照片在这一层是因为：垃圾完成即终态，这张照片是唯一的现场留证；
 * 遛宠送回宠物同样人在现场，此刻不拍，之后再补传十有八九没人做。
 */
type FinishTarget = { no: string; monthly: boolean; node: string; service: 'garbage' | 'dogwalk' }
const finishTarget = ref<FinishTarget | null>(null)

/** 弹层句柄：失败时把已上传成功的从待传列表移除，重试只传剩下的，不会重复上传 */
const finishSheet = ref<{ dropUploaded: (n: number) => void } | null>(null)
const startSheet = ref<{ dropUploaded: (n: number) => void } | null>(null)

async function confirmFinish(photos: string[]) {
  const t = finishTarget.value
  if (!t || acting.value) return
  acting.value = true
  let done = 0
  try {
    // 先传凭证再推进：反过来的话推进成功、上传失败，订单已经推进却拿不出凭证，
    // 而「完成服务」弹层也再不会出现，等于彻底失去补拍入口。逐张上传。
    for (const p of photos) {
      await uploadRecord(t.no, p, 'photo', false, 'finish')
      done++
    }
    if (t.monthly) {
      await providerMonthlyFinish(t.no)
    } else {
      await advanceOrder(t.no, t.node)
    }
    finishTarget.value = null
    // 遛宠完成进的是「待住户确认」，不是终态，toast 要说清下一步在哪
    uni.showToast({
      title:
        t.node === 'pending_confirm'
          ? '已提交待确认'
          : photos.length
            ? '已留证并完成'
            : '已完成',
      icon: 'success',
    })
    await Promise.all([reload(), loadCounts()])
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

/** 月卡「完成服务」：收尾今天这一趟，不影响服务期与剩余天数 */
function onFinishMonthly(o: Order) {
  if (acting.value) return
  finishTarget.value = { no: o.order_no, monthly: true, node: '', service: 'garbage' }
}

/**
 * 遛宠「开始服务」：先拍上门凭证（必填）再推进到服务中。
 * 接到宠物那一刻人就在现场，凭证最能证明服务确实开始了；
 * 与详情页同一套弹层，两个入口的行为不会分叉。
 */
const startTarget = ref('')

async function confirmStart(photos: string[], consent: boolean) {
  const no = startTarget.value
  if (!no || acting.value) return
  acting.value = true
  let done = 0
  try {
    // 先传凭证再推进，与完成服务同序：反过来的话推进成功、上传失败，
    // 订单进了服务中却拿不出开始凭证，弹层也再不会出现。逐张上传。
    for (const p of photos) {
      await uploadRecord(no, p, 'photo', consent, 'start')
      done++
    }
    await advanceOrder(no, 'in_service')
    startTarget.value = ''
    uni.showToast({ title: '已开始服务', icon: 'success' })
    await Promise.all([reload(), loadCounts()])
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

async function onAdvance(o: Order, node: string) {
  // 垃圾单直接完成前先给拍照位；跳过这层等于放弃唯一的现场留证
  if (node === 'completed' && o.service_type === 'garbage') {
    if (acting.value) return
    finishTarget.value = { no: o.order_no, monthly: false, node: 'completed', service: 'garbage' }
    return
  }
  // 遛宠完成服务：弹层支持上传服务凭证（送回宠物的照片）
  if (node === 'pending_confirm' && o.service_type === 'dogwalk') {
    if (acting.value) return
    finishTarget.value = {
      no: o.order_no,
      monthly: false,
      node: 'pending_confirm',
      service: 'dogwalk',
    }
    return
  }
  // 遛宠开始服务必须先上传凭证
  if (node === 'in_service' && o.service_type === 'dogwalk') {
    if (acting.value) return
    startTarget.value = o.order_no
    return
  }
  if (acting.value) return
  acting.value = true
  try {
    if (node === 'monthly_finish') {
      // 状态机里 accepted 不能直接到 pending_confirm，月卡跑满 30 天后
      // 让服务方连点「出发→开始→完成」三步毫无意义，这里一次点完；
      // 第一步成功后即使第二步失败，订单停在「服务中」，再点一次就能继续。
      if (o.status !== 'in_service' && o.status !== 'pending_confirm') {
        await advanceOrder(o.order_no, 'in_service')
      }
      await advanceOrder(o.order_no, 'pending_confirm')
    } else {
      await advanceOrder(o.order_no, node)
    }
    uni.showToast({ title: '已更新', icon: 'success' })
    await Promise.all([reload(), loadCounts()])
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
 * 服务方申请改期。
 * 时间是对住户的承诺，所以这里只能「申请」，真正生效要等住户在订单详情里点同意。
 * 遛宠才给入口：宠物上门需要主人留在家，改期是高频需求；垃圾/陪诊没这个场景。
 */
const rescheduleTarget = ref<Order | null>(null)

function canReschedule(o: Order): boolean {
  // 已有待确认申请时不再给入口：重复提交只会被后端 2003 拦住
  return o.service_type === 'dogwalk' && o.status === 'accepted' && !o.pending_reschedule
}

/** 已提交申请的目标时间，模板里抽成函数避免嵌套可空字段的窄化报错 */
function pendingTime(o: Order): string {
  const rs = o.pending_reschedule
  if (!rs) return ''
  return `${friendlyDate(rs.to_date)} ${slotLabel(rs.to_slot)}`
}

/** 住户改期前的原时间，标删除线让「变了多少」一眼可见 */
function noticeFrom(o: Order): string {
  const rs = o.reschedule_notice
  if (!rs) return ''
  return `${friendlyDate(rs.from_date)} ${slotLabel(rs.from_slot)}`
}

/** 住户改期后的新时间 */
function noticeTo(o: Order): string {
  const rs = o.reschedule_notice
  if (!rs) return ''
  return `${friendlyDate(rs.to_date)} ${slotLabel(rs.to_slot)}`
}

const acking = ref('')

/**
 * 服务方确认「知道时间变了」。
 * 只清本地这条提示、不整页刷新——刷新会让列表跳动，而确认本身不改变订单数据。
 */
async function onAckReschedule(o: Order) {
  if (acking.value) return
  acking.value = o.order_no
  try {
    await ackProviderReschedule(o.order_no)
    o.reschedule_notice = undefined
  } catch (e) {
    uni.showToast({ title: e instanceof ApiError ? e.message : '操作失败', icon: 'none' })
  } finally {
    acking.value = ''
  }
}

function openReschedule(o: Order) {
  if (acting.value) return
  rescheduleTarget.value = o
}

async function confirmReschedule(payload: { date: string; slot: string; reason: string }) {
  const o = rescheduleTarget.value
  if (!o || acting.value) return
  acting.value = true
  try {
    await providerReschedule(o.order_no, {
      service_date: payload.date,
      slot_code: payload.slot,
      reason: payload.reason,
    })
    // 先关弹层再提示，否则 toast 会被遮罩挡住
    rescheduleTarget.value = null
    uni.showToast({ title: '已提交，等待确认', icon: 'success' })
    await Promise.all([reload(), loadCounts()])
  } catch (e) {
    uni.showModal({
      title: '申请失败',
      content: e instanceof ApiError ? e.message : '请稍后重试',
      showCancel: false,
    })
  } finally {
    acting.value = false
  }
}
</script>

<template>
  <view class="page">
    <view v-if="income" class="income-card">
      <view class="income-main">
        <text class="income-label">累计收入</text>
        <text class="income-num">¥{{ ((income.total || 0) / 100).toFixed(2) }}</text>
      </view>
      <!-- 这里说的是「已经结算入账的单」，和下面的「已完成」tab 不是一回事：
           包里今日已服务的单会进已完成，但钱要等服务期结束提交后才结算。 -->
      <text class="income-sub">已结算 {{ income.order_count }} 单</text>
    </view>

    <view class="pool-entry" @tap="goPool">
      <view class="pool-text">
        <text class="pool-num">{{ openCount }}</text>
        <text class="pool-label">个新订单待接</text>
      </view>
      <text class="pool-go">去接单 ›</text>
    </view>

    <view class="tabs">
      <view
        v-for="t in tabs"
        :key="t.key"
        class="tab"
        :class="{ active: activeTab === t.key }"
        @tap="switchTab(t.key)"
      >
        <view class="tab-inner">
          <text>{{ t.label }}</text>
          <text v-if="badgeCount(t.key) > 0" class="tab-badge">{{
            badgeCount(t.key) > 99 ? '99+' : badgeCount(t.key)
          }}</text>
        </view>
      </view>
    </view>

    <view v-if="!loading && orders.length === 0" class="empty">
      暂无任务，去接单池看看
    </view>

    <!-- 按服务种类分组：三类服务的上门动作不同，混在一起容易拿错单 -->
    <view v-for="g in groups" :key="g.key" class="group">
      <view class="group-head">
        <view class="group-name-wrap">
          <text class="group-dot" :class="g.key" />
          <text class="group-name">{{ g.label }}</text>
        </view>
        <!-- 条数只在整页加载完后才显示：分页中途只有「已加载条数」，
             写「3 单」会让服务方以为这一类只有 3 单 -->
        <text v-if="finished" class="group-count">共 {{ g.list.length }} 单</text>
      </view>

      <view
        v-for="o in g.list"
        :key="o.order_no"
        class="task-card"
        @tap="goDetail(o.order_no)"
      >
        <!-- 住户直接改期是立即生效的：卡片上的时间会悄悄变掉，不逐单翻详情发现不了。
             所以这里给一条红橙通栏，服务方点「我知道了」才消失。 -->
        <view v-if="o.reschedule_notice" class="rs-notice">
          <view class="rs-notice-head">
            <text class="rs-notice-tag">住户改了时间</text>
            <view class="rs-notice-ack" @tap.stop="onAckReschedule(o)">
              {{ acking === o.order_no ? '确认中…' : '我知道了' }}
            </view>
          </view>
          <view class="rs-notice-body">
            <text class="rs-notice-from">{{ noticeFrom(o) }}</text>
            <text class="rs-notice-arrow">→</text>
            <text class="rs-notice-to">{{ noticeTo(o) }}</text>
          </view>
        </view>

        <view class="head">
          <view class="head-left">
            <text class="service">{{ SERVICE_LABEL[o.service_type] }}</text>
            <text v-if="o.plan === 'monthly'" class="badge">包月</text>
          </view>
          <!-- 月卡的订单状态永远是「已受理」，放在这里没有信息量，
               换成今天到底上门没有——这才是服务方每天要看的那一眼 -->
          <text v-if="o.monthly" class="status" :class="'m-' + monthlyTone(o)">
            {{ monthlyHead(o) }}
          </text>
          <text v-else class="status" :class="o.status">{{ o.status_label }}</text>
        </view>

        <!-- 月卡：一张单跑 30 天，单独一条状态带说清今天完成没有 + 走到第几天 -->
        <view v-if="o.monthly" class="m-strip" :class="monthlyTone(o)">
          <view class="m-top">
            <view class="m-left">
              <text class="m-dot" />
              <text class="m-text">{{ monthlyText(o) }}</text>
            </view>
            <text class="m-day">已服务 {{ o.monthly.used }}/{{ o.monthly.total }} 天</text>
          </view>
          <view class="m-bottom">
            服务期 {{ monthlyRange(o) }} · 每天 {{ slotLabel(o.extra_slot_code || o.slot_code) }} 上门
          </view>
        </view>

        <view v-else class="meta">
          {{ friendlyDate(o.service_date) }} · {{ slotLabel(o.slot_code) }}
        </view>
        <view v-if="o.address" class="addr">
          {{ o.address.community }} {{ [o.address.lane, o.address.house_no].filter(Boolean).join(' ') }}
        </view>
        <view v-if="o.address && (o.address.contact_name || o.address.contact_phone)" class="contact">
          联系人：{{ o.address.contact_name }} {{ o.address.contact_phone }}
        </view>
        <view v-if="o.remark" class="remark">备注：{{ o.remark }}</view>

        <!-- 改期申请已提交：不提示的话服务方会以为按钮丢了，或反复点改期被后端拒绝 -->
        <view v-if="o.pending_reschedule" class="rs-hint">
          改期待住户确认：{{ pendingTime(o) }}
        </view>

        <!-- 整卡可点，但用户看不出哪里能点：尤其「服务中」只有一个主按钮、左侧空着。
             所以固定给一个显性的「任务详情」入口，操作按钮靠右成组。 -->
        <view class="actions">
          <view class="action detail" @tap.stop="goDetail(o.order_no)">任务详情 ›</view>
          <view class="actions-right">
            <!-- 今天这趟已经服务完就不再给「无法接单」：人都上过门了，按钮还挂着只会点出 2003 -->
            <view
              v-if="o.status === 'accepted' && !arrivedToday(o)"
              class="action"
              @tap.stop="onReject(o)"
            >
              无法接单
            </view>
            <!-- 改期只能申请，住户同意后才生效 -->
            <view
              v-if="canReschedule(o)"
              class="action"
              @tap.stop="openReschedule(o)"
            >
              改期
            </view>
            <!-- 月卡服务期内的主操作分两段：先「出发上门」记录人到门口，再「完成服务」收尾 -->
            <view
              v-if="canRedeem(o)"
              class="action primary"
              @tap.stop="onRedeem(o)"
            >
              {{ redeeming === o.order_no ? '记录中…' : '出发上门' }}
            </view>
            <view
              v-else-if="canFinishMonthly(o)"
              class="action primary"
              @tap.stop="onFinishMonthly(o)"
            >
              完成服务
            </view>
            <view
              v-else-if="servedToday(o)"
              class="action done"
            >
              今日已服务
            </view>
            <view
              v-for="a in actions(o)"
              :key="a.node"
              class="action"
              :class="{ primary: a.primary }"
              @tap.stop="onAdvance(o, a.node)"
            >
              {{ a.label }}
            </view>
          </view>
        </view>
      </view>
    </view>

    <view v-if="loading" class="loading">加载中…</view>
    <view v-else-if="finished && orders.length > 0" class="loading">没有更多了</view>

    <TabBar current="ptasks" />

    <!-- 拒单弹层：默认隐藏，点卡片「无法接单」才出现 -->
    <RejectSheet
      :visible="!!rejectTarget"
      :loading="acting"
      @close="rejectTarget = null"
      @confirm="confirmReject"
    />

    <!-- 垃圾单完成弹层：可选留一张现场照片，不拍也能完成 -->
    <FinishSheet
      :visible="!!finishTarget"
      :loading="acting"
      :service="finishTarget?.service"
      ref="finishSheet"
      @close="finishTarget = null"
      @confirm="confirmFinish"
    />

    <!-- 遛宠开始服务：凭证必填弹层，同一个组件的 start 模式 -->
    <FinishSheet
      :visible="!!startTarget"
      :loading="acting"
      mode="start"
      ref="startSheet"
      @close="startTarget = ''"
      @confirm="confirmStart"
    />

    <!-- 改期弹层：提交后等住户在订单详情里确认 -->
    <RescheduleSheet
      :visible="!!rescheduleTarget"
      :loading="acting"
      mode="provider"
      :service-type="rescheduleTarget ? rescheduleTarget.service_type : ''"
      :from-date="rescheduleTarget ? rescheduleTarget.service_date : ''"
      :from-slot="rescheduleTarget ? rescheduleTarget.slot_code : ''"
      @close="rescheduleTarget = null"
      @confirm="confirmReschedule"
    />
  </view>
</template>

<style scoped>
.income-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: 0 24rpx 20rpx;
  padding: 26rpx 30rpx;
  background: linear-gradient(135deg, #e9f5f3 0%, #e4f3f0 100%);
  border-radius: 20rpx;
}

.income-main { display: flex; align-items: center; gap: 16rpx; }
.income-label { font-size: 26rpx; color: var(--text-sub); }
.income-num { font-size: 40rpx; font-weight: 800; color: var(--brand-dark); }
.income-sub { font-size: 23rpx; color: var(--text-light); }
.page {
  padding-top: 24rpx;
  /* 给固定 TabBar 留位置 */
  padding-bottom: 160rpx;
}

.pool-entry {
  margin: 0 24rpx 24rpx;
  padding: 28rpx 32rpx;
  border-radius: 24rpx;
  background: linear-gradient(160deg, #f59e0b, #d97706);
  color: #fff;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.pool-num {
  font-size: 44rpx;
  font-weight: 700;
}

.pool-label {
  font-size: 26rpx;
  margin-left: 10rpx;
  opacity: 0.9;
}

.pool-go {
  font-size: 27rpx;
  font-weight: 500;
}

.tabs {
  display: flex;
  background: #fff;
  border-radius: 20rpx;
  margin: 0 24rpx 20rpx;
  padding: 0 12rpx;
}

.tab {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24rpx 0;
  font-size: 27rpx;
  color: var(--text-sub);
  position: relative;
}

.tab-inner {
  position: relative;
  display: inline-flex;
  align-items: center;
}

/* 角标绝对定位浮在文字右上角：不占布局宽度，不会把三个标签挤偏，
   下划线（.tab.active::after）也仍然对齐文字中心 */
.tab-badge {
  position: absolute;
  right: -32rpx;
  top: -14rpx;
  min-width: 30rpx;
  height: 30rpx;
  padding: 0 7rpx;
  border-radius: 15rpx;
  background: #ef4444;
  color: #fff;
  font-size: 19rpx;
  font-weight: 600;
  line-height: 30rpx;
  text-align: center;
  box-sizing: border-box;
}

.tab.active {
  color: var(--brand-dark);
  font-weight: 600;
}

.tab.active::after {
  content: '';
  position: absolute;
  left: 50%;
  bottom: 8rpx;
  transform: translateX(-50%);
  width: 40rpx;
  height: 6rpx;
  border-radius: 3rpx;
  background: var(--brand);
}

.task-card {
  background: #fff;
  border-radius: 24rpx;
  padding: 28rpx;
  margin: 0 24rpx 20rpx;
}

/* 分组标题：沿用卡片左右边距，让分组感和卡片对齐在一条竖线上 */
.group {
  margin-bottom: 8rpx;
}

.group-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6rpx 4rpx 14rpx;
  margin: 0 24rpx;
}

.group-name-wrap {
  display: flex;
  align-items: center;
  gap: 12rpx;
}

/* 每类服务的色点固定：垃圾橙、遛宠绿、陪诊蓝，和详情页的必要性标签同色系 */
.group-dot {
  width: 12rpx;
  height: 12rpx;
  border-radius: 6rpx;
  background: var(--brand);
}

.group-dot.garbage { background: #f59e0b; }
.group-dot.dogwalk { background: #10b981; }
.group-dot.accompany { background: #3b82f6; }

.group-name {
  font-size: 27rpx;
  font-weight: 600;
  color: var(--text);
}

.group-count {
  font-size: 23rpx;
  color: var(--text-light);
}

/* 月卡状态带：绿=今天做完了，橙=还等着上门，灰=服务期外/已结束。
   这四个状态是服务方每天唯一要判断的事，所以给它整条底色而不是一行小字 */
.m-strip {
  margin-top: 14rpx;
  padding: 14rpx 18rpx;
  border-radius: 14rpx;
  border: 1rpx solid transparent;
}

.m-strip.done { background: #ecfdf5; border-color: #a7f3d0; }
.m-strip.todo { background: #fff7ed; border-color: #fed7aa; }
/* 已上门未完成：蓝色，和「还没上门」（橙）、「已服务」（绿）区分开 */
.m-strip.doing { background: #eff6ff; border-color: #bfdbfe; }
.m-strip.out,
.m-strip.ended { background: #f8fafc; border-color: #e5e7eb; }

.m-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12rpx;
}

.m-left {
  display: flex;
  align-items: center;
  gap: 10rpx;
  min-width: 0;
}

.m-dot {
  width: 12rpx;
  height: 12rpx;
  border-radius: 6rpx;
  background: var(--text-light);
  flex-shrink: 0;
}

.m-strip.done .m-dot { background: #10b981; }
.m-strip.todo .m-dot { background: #f59e0b; }
.m-strip.doing .m-dot { background: #2563eb; }

.m-text {
  font-size: 25rpx;
  font-weight: 600;
  color: var(--text-sub);
}

.m-strip.done .m-text { color: #047857; }
.m-strip.todo .m-text { color: #c2410c; }
.m-strip.doing .m-text { color: #1d4ed8; }

.m-day {
  font-size: 22rpx;
  color: var(--text-light);
  flex-shrink: 0;
}

.m-bottom {
  margin-top: 6rpx;
  font-size: 21rpx;
  color: var(--text-light);
}

/* 月卡订单头上那个状态：换成今天完成没有，颜色跟着状态带走 */
.status.m-done { color: #047857; }
.status.m-todo { color: #c2410c; }
.status.m-doing { color: #1d4ed8; }
.status.m-out,
.status.m-ended { color: var(--text-light); }

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

.service {
  font-size: 30rpx;
  font-weight: 600;
}

.badge {
  font-size: 20rpx;
  color: #b45309;
  background: #fef3c7;
  padding: 4rpx 12rpx;
  border-radius: 8rpx;
}

.status {
  font-size: 25rpx;
  font-weight: 500;
  color: var(--brand-dark);
}

.status.in_service {
  color: #2563eb;
}

.status.pending_confirm {
  color: #d97706;
}

.status.completed {
  color: var(--text-sub);
}

.meta {
  margin-top: 14rpx;
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

/* 申请中状态说明：橙色和「待确认」的牌面语义一致，但不用整条 banner，
   服务方这边只是状态告知，不需要催他动作 */
.rs-hint {
  margin-top: 12rpx;
  padding: 10rpx 16rpx;
  border-radius: 10rpx;
  background: #fff8ed;
  font-size: 22rpx;
  color: #b45309;
}

/* 住户改期通栏：用暖红而不是品牌绿——绿色在这套界面里代表「一切正常」，
   而这条恰恰是「你原来看的信息已经作废了」，需要他停下确认。
   和上面 rs-hint 的浅橙状态告知刻意拉开层次：那条只是告知，这条要动作。 */
.rs-notice {
  margin-bottom: 20rpx;
  padding: 18rpx 20rpx;
  border-radius: 16rpx;
  background: #fef3ec;
  border: 1rpx solid #fed7c3;
}

.rs-notice-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10rpx;
}

.rs-notice-tag {
  font-size: 24rpx;
  font-weight: 600;
  color: #c2410c;
}

.rs-notice-ack {
  padding: 8rpx 22rpx;
  border-radius: 24rpx;
  background: #c2410c;
  color: #fff;
  font-size: 22rpx;
  line-height: 1.4;
}

.rs-notice-body {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8rpx;
  font-size: 24rpx;
  color: #9a3412;
}

/* 划掉的旧时间：弱化它，让新时间成为唯一的视觉焦点 */
.rs-notice-from {
  color: #b08a72;
  text-decoration: line-through;
}

.rs-notice-arrow {
  color: #c2410c;
}

.rs-notice-to {
  font-weight: 600;
  color: #9a3412;
}

/* 「任务详情」固定靠左，操作按钮靠右成组；一行放不下时操作按钮整体换行，
   不让「无法接单/改期/出发上门」三个按钮被压成两三个字一行 */
.actions {
  margin-top: 20rpx;
  padding-top: 20rpx;
  border-top: 1rpx solid #f3f4f6;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 12rpx;
}

.actions-right {
  display: flex;
  align-items: center;
  gap: 12rpx;
  margin-left: auto;
}

.action {
  padding: 12rpx 26rpx;
  border-radius: 32rpx;
  font-size: 25rpx;
  border: 1rpx solid var(--border);
  color: var(--text);
}

/* 次要入口：弱化描边与字色 + 浅底色，避免和主操作抢注意力，
   同时保证 1rpx 边框在低 DPI 屏上也看得出来是个按钮而不是一行文字 */
.action.detail {
  color: var(--text-sub);
  padding: 12rpx 22rpx;
  background: #f8faf9;
}

.action.primary {
  background: var(--brand);
  color: #fff;
  border-color: var(--brand);
  font-weight: 500;
}

/* 已打卡：不能再点，用浅绿色实心块表示「今天这件事完了」 */
.action.done {
  background: #ecfdf5;
  color: #047857;
  border-color: #a7f3d0;
  font-weight: 500;
}

.loading {
  text-align: center;
  color: var(--text-light);
  font-size: 24rpx;
  padding: 30rpx 0;
}
</style>
