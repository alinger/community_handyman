<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { listSkus, updateSku, type ServiceSku, ApiError } from '@/api'

const loading = ref(false)
const rows = ref<ServiceSku[]>([])
const visible = ref(false)
const saving = ref(false)
const editing = ref<ServiceSku | null>(null)
const advanced = ref(false) // 高级 JSON 模式
const rulesText = ref('{}')
const ruleError = ref('')

const typeLabel = (t: string) => ({ garbage: '代倒垃圾', dogwalk: '遛宠', accompany: '陪诊服务' }[t] || t)
const planLabel = (p: string) => (p === 'monthly' ? '包月' : '单次')
const money = (fen: number) => `¥${(fen / 100).toFixed(2)}`
/** 时段码格式，必须与服务端 slotRange 的解析格式一致，否则下单时才报错 */
const SLOT_RE = /^([01]\d|2[0-3]):[0-5]\d-([01]\d|2[0-3]):[0-5]\d$/

/** 多行文本转时段数组：容忍换行/逗号分隔，去重保序 */
function parseSlotLines(text: string): string[] {
  const seen = new Set<string>()
  const out: string[] = []
  for (const raw of text.split(/[\n,，;；]/)) {
    const v = raw.trim()
    if (!v || seen.has(v)) continue
    seen.add(v)
    out.push(v)
  }
  return out
}
const toYuan = (fen: unknown) => (typeof fen === 'number' ? fen / 100 : 0)
const toFen = (yuan: number) => Math.round(yuan * 100)

// ---------- 列表摘要工具 ----------
const fen2 = (v: unknown) => (typeof v === 'number' ? `¥${(v / 100).toFixed(2)}` : '—')
const numS = (v: unknown, d: number) => (typeof v === 'number' ? String(v) : String(d))
type KV = { label: string; value: string }

const DOG_PRICE_LABELS: Record<string, string> = {
  base: '起步价', familiar: '熟悉费', over_3km: '超3km加价', extra_pet: '每多一只',
  feed_only: '仅喂食', step_price: '时长每档加价', holiday_extra: '节日加价',
}
const DOG_COUNT_LABELS: Record<string, string> = {
  base_minutes: '含时长（分钟）', step_minutes: '每档时长（分钟）', max_minutes: '最长时长（分钟）',
  familiar_free_orders: '免熟悉费订单数', distance_threshold_km: '距离阈值（km）',
}

function itemsOf(sku: ServiceSku) {
  const r = (sku.rules ?? {}) as Record<string, unknown>
  return Array.isArray(r.service_items) ? (r.service_items as { code: string; name: string; price: number }[]) : []
}
function hospitalsOf(sku: ServiceSku) {
  const r = (sku.rules ?? {}) as Record<string, unknown>
  return Array.isArray(r.hospitals) ? (r.hospitals as { code: string; name: string }[]) : []
}
function datesOf(sku: ServiceSku) {
  const r = (sku.rules ?? {}) as Record<string, unknown>
  return Array.isArray(r.holiday_dates) ? (r.holiday_dates as string[]) : []
}
function clip(names: string[], n = 3) {
  return names.length > n ? `${names.slice(0, n).join('、')} 等` : names.join('、')
}

function summaryLines(sku: ServiceSku): string[] {
  const r = (sku.rules ?? {}) as Record<string, unknown>
  if (sku.service_type === 'accompany') {
    const bi = (r.base_item ?? {}) as { name?: string; price?: number }
    const items = itemsOf(sku)
    const hs = hospitalsOf(sku)
    const lines: string[] = []
    lines.push(
      r.manual_quote === true
        ? '手动报价模式：下单后由运营报价确认'
        : `按服务项自动计价 · 基础服务费 ${bi.name || '基础服务费'} ${fen2(bi.price)}`,
    )
    if (items.length) lines.push(`服务项 ${items.length} 项：${clip(items.map((i) => `${i.name} ${fen2(i.price)}`))}`)
    if (hs.length) lines.push(`支持医院 ${hs.length} 家：${clip(hs.map((h) => h.name))}`)
    return lines
  }
  if (sku.service_type === 'dogwalk') {
    const lines = [
      `起步 ${fen2(r.base)}（含 ${numS(r.base_minutes, 20)} 分钟，超时每 ${numS(r.step_minutes, 10)} 分钟加 ${fen2(r.step_price)}）`,
      `最长 ${numS(r.max_minutes, 120)} 分钟 · 距离阈值 ${numS(r.distance_threshold_km, 3)}km，超出加 ${fen2(r.over_3km)}`,
      `熟悉费 ${fen2(r.familiar)}（前 ${numS(r.familiar_free_orders, 5)} 单免收）· 每多一只 ${fen2(r.extra_pet)} · 仅喂食 ${fen2(r.feed_only)}`,
      `节日加价 ${fen2(r.holiday_extra)}${datesOf(sku).length ? `（已设 ${datesOf(sku).length} 个日期）` : ''}`,
    ]
    return lines
  }
  // 垃圾
  if (sku.plan === 'monthly') {
    const extras = Array.isArray(r.extra_slot_options) ? (r.extra_slot_options as string[]) : []
    return [
      `${fen2(r.monthly_price)}/月 · ${numS(r.duration_days, 30)} 天`,
      `默认取件 ${typeof r.default_slot === 'string' && r.default_slot ? r.default_slot : '22:00-22:30'}` +
        (extras.length ? ` · 可选 ${extras.join(' / ')}` : ''),
      `每日上限 ${numS(r.daily_limit, 1)} 次 · ${r.carry_over === true ? '次数可结转' : '次数不可结转'}`,
    ]
  }
  return [
    `按袋计价 · 基础价含 ${numS(r.bags_included, 1)} 袋，超袋 ${fen2(r.extra_bag)}/袋`,
    `包月参考 ${fen2(r.monthly_price)}/月（每日 ${numS(r.monthly_daily_limit, 1)} 次）`,
  ]
}

function detailRows(sku: ServiceSku): KV[] {
  const r = (sku.rules ?? {}) as Record<string, unknown>
  if (sku.service_type === 'accompany') {
    const bi = (r.base_item ?? {}) as { name?: string; price?: number }
    return [
      { label: '手动报价', value: r.manual_quote === true ? '开启' : '关闭（按服务项自动计价）' },
      { label: '基础服务费', value: `${bi.name || '—'}　${fen2(bi.price)}` },
      { label: '服务项数', value: `${itemsOf(sku).length} 项` },
      { label: '支持医院', value: `${hospitalsOf(sku).length} 家` },
    ]
  }
  if (sku.service_type === 'dogwalk') {
    const out: KV[] = []
    for (const k of Object.keys(DOG_PRICE_LABELS)) out.push({ label: DOG_PRICE_LABELS[k]! + '（元）', value: fen2(r[k]) })
    for (const k of Object.keys(DOG_COUNT_LABELS)) out.push({ label: DOG_COUNT_LABELS[k]! + '', value: numS(r[k], 0) })
    return out
  }
  // 垃圾
  if (sku.plan === 'monthly') {
    const extras = Array.isArray(r.extra_slot_options) ? (r.extra_slot_options as string[]) : []
    return [
      { label: '包月价', value: fen2(r.monthly_price) + ' / 月' },
      { label: '服务天数', value: numS(r.duration_days, 30) + ' 天' },
      { label: '默认取件时段', value: typeof r.default_slot === 'string' && r.default_slot ? r.default_slot : '22:00-22:30' },
      { label: '可选额外时段', value: extras.length ? extras.join(' / ') : '—' },
      { label: '每日次数上限', value: numS(r.daily_limit, 1) },
      { label: '次数可结转', value: r.carry_over === true ? '是' : '否' },
    ]
  }
  return [
    { label: '计价单位', value: r.unit === 'bag' ? '按袋' : String(r.unit ?? '—') },
    { label: '含袋数', value: numS(r.bags_included, 1) },
    { label: '超袋加价', value: fen2(r.extra_bag) + ' / 袋' },
    { label: '包月价', value: fen2(r.monthly_price) + ' / 月' },
    { label: '包月每日上限', value: numS(r.monthly_daily_limit, 1) },
  ]
}

// ---------- 结构化表单状态 ----------
interface NamedRow { code: string; name: string; priceYuan: number }
const form = reactive({
  basePriceYuan: 0,
  // 陪诊
  manualQuote: false,
  baseItemName: '',
  baseItemPriceYuan: 0,
  hospitals: [] as { code: string; name: string }[],
  serviceItems: [] as NamedRow[],
  // 遛宠：价格（元）
  dogYuan: { base: 0, familiar: 0, over_3km: 0, extra_pet: 0, feed_only: 0, step_price: 0, holiday_extra: 0 } as Record<string, number>,
  dogCount: { max_minutes: 120, base_minutes: 20, step_minutes: 10, familiar_free_orders: 5, distance_threshold_km: 3 } as Record<string, number>,
  holidayDates: [] as string[],
  // 垃圾
  garbage: {
    unit: 'bag', bags_included: 1, extra_bagYuan: 0,
    monthly_priceYuan: 0, monthly_daily_limit: 1, carry_over: false, daily_limit: 1,
    // 包月：服务天数、默认取件时段、可选额外时段（一行一个）
    duration_days: 30, default_slot: '22:00-22:30', extraSlotText: '',
  },
})

const isAccompany = computed(() => editing.value?.service_type === 'accompany')
const isDogwalk = computed(() => editing.value?.service_type === 'dogwalk')
const isGarbage = computed(() => editing.value?.service_type === 'garbage')

function genCode(prefix: string) {
  return `${prefix}_${Date.now().toString(36)}${Math.floor(Math.random() * 1e4).toString(36)}`
}

async function load() {
  loading.value = true
  try {
    rows.value = await listSkus()
  } catch {
    // 拦截器已提示
  } finally {
    loading.value = false
  }
}
onMounted(load)

function openEdit(sku: ServiceSku) {
  editing.value = sku
  advanced.value = false
  ruleError.value = ''
  form.basePriceYuan = toYuan(sku.base_price)
  const r = (sku.rules ?? {}) as Record<string, unknown>

  // 陪诊
  form.manualQuote = r.manual_quote === true
  const bi = (r.base_item ?? null) as { code?: string; name?: string; price?: number } | null
  form.baseItemName = bi?.name || '基础服务费'
  form.baseItemPriceYuan = toYuan(bi?.price)
  form.hospitals = Array.isArray(r.hospitals)
    ? (r.hospitals as { code: string; name: string }[]).map((h) => ({ code: h.code, name: h.name }))
    : []
  form.serviceItems = Array.isArray(r.service_items)
    ? (r.service_items as { code: string; name: string; price: number }[]).map((i) => ({
        code: i.code, name: i.name, priceYuan: toYuan(i.price),
      }))
    : []

  // 遛宠
  const dogPriceKeys = ['base', 'familiar', 'over_3km', 'extra_pet', 'feed_only', 'step_price', 'holiday_extra']
  for (const k of dogPriceKeys) form.dogYuan[k] = toYuan(r[k])
  const dogCountKeys = ['max_minutes', 'base_minutes', 'step_minutes', 'familiar_free_orders', 'distance_threshold_km']
  for (const k of dogCountKeys) {
    const v = r[k]
    form.dogCount[k] = typeof v === 'number' ? v : form.dogCount[k]
  }
  form.holidayDates = Array.isArray(r.holiday_dates) ? (r.holiday_dates as string[]).slice() : []

  // 垃圾
  form.garbage.unit = typeof r.unit === 'string' ? r.unit : 'bag'
  form.garbage.bags_included = typeof r.bags_included === 'number' ? r.bags_included : 1
  form.garbage.extra_bagYuan = toYuan(r.extra_bag)
  form.garbage.monthly_priceYuan = toYuan(r.monthly_price)
  form.garbage.monthly_daily_limit = typeof r.monthly_daily_limit === 'number' ? r.monthly_daily_limit : 1
  form.garbage.carry_over = r.carry_over === true
  form.garbage.daily_limit = typeof r.daily_limit === 'number' ? r.daily_limit : 1
  form.garbage.duration_days = typeof r.duration_days === 'number' ? r.duration_days : 30
  form.garbage.default_slot =
    typeof r.default_slot === 'string' && r.default_slot ? r.default_slot : '22:00-22:30'
  form.garbage.extraSlotText = Array.isArray(r.extra_slot_options)
    ? (r.extra_slot_options as string[]).join('\n')
    : ''

  visible.value = true
}

// 由结构化表单生成完整 rules（未识别键原样保留）
function buildRules(): Record<string, unknown> {
  const src = (editing.value?.rules ?? {}) as Record<string, unknown>
  const out: Record<string, unknown> = { ...src }
  if (isAccompany.value) {
    out.manual_quote = form.manualQuote
    out.base_item = {
      code: (src.base_item as { code?: string } | undefined)?.code || 'base_fee',
      name: form.baseItemName,
      price: toFen(form.baseItemPriceYuan),
    }
    out.hospitals = form.hospitals
      .filter((h) => h.name.trim())
      .map((h) => ({ code: h.code, name: h.name.trim() }))
    out.service_items = form.serviceItems
      .filter((i) => i.name.trim())
      .map((i) => ({ code: i.code, name: i.name.trim(), price: toFen(i.priceYuan) }))
    out.items = (out.service_items as { name: string }[]).map((i) => i.name)
  } else if (isDogwalk.value) {
    for (const k of Object.keys(form.dogYuan)) out[k] = toFen(form.dogYuan[k])
    for (const k of Object.keys(form.dogCount)) out[k] = form.dogCount[k]
    out.holiday_dates = [...form.holidayDates].sort()
  } else if (isGarbage.value) {
    if (editing.value?.plan === 'monthly') {
      // 这四项是 GarbagePlan 真正读取的配置（包月价、服务天数、默认时段、额外时段），
      // 之前只能改数据库，运营改价要发版。
      out.monthly_price = toFen(form.garbage.monthly_priceYuan)
      out.duration_days = form.garbage.duration_days
      out.default_slot = form.garbage.default_slot.trim()
      out.extra_slot_options = parseSlotLines(form.garbage.extraSlotText)
      out.carry_over = form.garbage.carry_over
      out.daily_limit = form.garbage.daily_limit
    } else {
      out.unit = form.garbage.unit
      out.bags_included = form.garbage.bags_included
      out.extra_bag = toFen(form.garbage.extra_bagYuan)
      out.monthly_price = toFen(form.garbage.monthly_priceYuan)
      out.monthly_daily_limit = form.garbage.monthly_daily_limit
    }
  }
  return out
}

function switchAdvanced(on: boolean) {
  if (on) {
    rulesText.value = JSON.stringify(buildRules(), null, 2)
    ruleError.value = ''
  }
  advanced.value = on
}

function addHospital() {
  form.hospitals.push({ code: genCode('h'), name: '' })
}
function removeHospital(i: number) {
  form.hospitals.splice(i, 1)
}
function addServiceItem() {
  form.serviceItems.push({ code: genCode('si'), name: '', priceYuan: 0 })
}
function removeServiceItem(i: number) {
  form.serviceItems.splice(i, 1)
}

async function save() {
  if (!editing.value) return

  // 结构化模式下先校验包月时段：格式错了要等到用户下单才炸，很难查
  if (!advanced.value && isGarbage.value && editing.value.plan === 'monthly') {
    const def = form.garbage.default_slot.trim()
    if (!SLOT_RE.test(def)) {
      ElMessage.error('默认取件时段格式应为 HH:MM-HH:MM，例如 22:00-22:30')
      return
    }
    const extras = parseSlotLines(form.garbage.extraSlotText)
    const bad = extras.find((s) => !SLOT_RE.test(s))
    if (bad) {
      ElMessage.error(`额外时段「${bad}」格式应为 HH:MM-HH:MM`)
      return
    }
    if (extras.includes(def)) {
      ElMessage.error('额外时段不能与默认时段相同')
      return
    }
  }

  let rules: Record<string, unknown>
  if (advanced.value) {
    try {
      const parsed = JSON.parse(rulesText.value)
      if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) throw new Error('规则必须是 JSON 对象')
      rules = parsed
      ruleError.value = ''
    } catch (e) {
      ruleError.value = e instanceof Error ? e.message : 'JSON 格式错误'
      return
    }
  } else {
    rules = buildRules()
  }
  saving.value = true
  try {
    await updateSku(editing.value.id, toFen(form.basePriceYuan), rules)
    ElMessage.success('已保存，计价规则即时生效，无需发版')
    visible.value = false
    load()
  } catch (e) {
    if (e instanceof ApiError) ElMessage.error(e.message)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div>
    <div class="toolbar">
      <span class="title">服务与价格</span>
      <el-button @click="load">刷新</el-button>
    </div>
    <div class="note">
      编辑弹窗按服务类型提供结构化表单（价格为元）；「高级 JSON 编辑」可改全部原始键。保存后即时生效、无需发版。
    </div>

    <el-table :data="rows" v-loading="loading" border stripe style="width: 100%">
      <el-table-column type="expand">
        <template #default="{ row }">
          <div class="expand-box">
            <div class="expand-sec">
              <div class="expand-title">规则详情</div>
              <div class="kv-grid">
                <div v-for="d in detailRows(row)" :key="d.label" class="kv">
                  <span class="kv-label">{{ d.label }}</span>
                  <span class="kv-value">{{ d.value }}</span>
                </div>
              </div>
            </div>
            <div v-if="row.service_type === 'accompany' && itemsOf(row).length" class="expand-sec">
              <div class="expand-title">服务项（{{ itemsOf(row).length }}）</div>
              <el-tag v-for="it in itemsOf(row)" :key="it.code" size="small" class="mtag">
                {{ it.name }} {{ money(it.price) }}
              </el-tag>
            </div>
            <div v-if="row.service_type === 'accompany' && hospitalsOf(row).length" class="expand-sec">
              <div class="expand-title">支持医院（{{ hospitalsOf(row).length }}）</div>
              <el-tag v-for="h in hospitalsOf(row)" :key="h.code" size="small" type="info" class="mtag">{{ h.name }}</el-tag>
            </div>
            <div v-if="row.service_type === 'dogwalk' && datesOf(row).length" class="expand-sec">
              <div class="expand-title">节日加价日（{{ datesOf(row).length }}）</div>
              <el-tag v-for="d in datesOf(row)" :key="d" size="small" type="warning" class="mtag">{{ d }}</el-tag>
            </div>
            <div class="expand-sec">
              <div class="expand-title">原始 JSON</div>
              <code class="rules-json">{{ JSON.stringify(row.rules || {}, null, 2) }}</code>
            </div>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="服务" min-width="180">
        <template #default="{ row }">
          <div class="svc">
            <span class="svc-name">{{ typeLabel(row.service_type) }}</span>
            <el-tag size="small" :type="row.plan === 'monthly' ? 'warning' : 'info'">{{ planLabel(row.plan) }}</el-tag>
            <el-tag size="small" :type="row.status === 1 ? 'success' : 'danger'">{{ row.status === 1 ? '上架' : '下架' }}</el-tag>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="基础价" width="110" align="right">
        <template #default="{ row }"><span class="price">{{ money(row.base_price) }}</span></template>
      </el-table-column>
      <el-table-column label="计价规则摘要" min-width="400">
        <template #default="{ row }">
          <div v-for="(l, i) in summaryLines(row)" :key="i" class="sum-line">{{ l }}</div>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="90" fixed="right">
        <template #default="{ row }"><el-button link type="primary" @click="openEdit(row)">编辑</el-button></template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="visible" title="编辑服务价格与规则" width="860px" top="6vh">
      <template v-if="editing">
        <el-form label-width="110px">
          <el-form-item label="服务">
            <el-input :model-value="`${typeLabel(editing.service_type)} · ${planLabel(editing.plan)}`" disabled />
          </el-form-item>
          <el-form-item label="基础价（元）">
            <el-input-number v-model="form.basePriceYuan" :min="0" :precision="2" :step="1" style="width: 220px" />
            <span class="hint">= {{ money(toFen(form.basePriceYuan)) }}</span>
          </el-form-item>

          <!-- 高级 JSON 模式 -->
          <template v-if="advanced">
            <el-form-item label="规则 JSON">
              <el-input v-model="rulesText" type="textarea" :rows="18" spellcheck="false" class="json-input" />
              <div v-if="ruleError" class="err">{{ ruleError }}</div>
              <div class="hint">直接编辑原始 JSON 对象，保存整体提交。</div>
            </el-form-item>
          </template>

          <!-- 结构化表单 -->
          <template v-else>
            <!-- 陪诊 -->
            <template v-if="isAccompany">
              <el-form-item label="手动报价">
                <el-switch v-model="form.manualQuote" />
                <span class="hint">开启后用户下单进入「待报价」，由运营报价确认；关闭则按服务项自动计价</span>
              </el-form-item>
              <el-form-item label="基础服务费">
                <el-input v-model="form.baseItemName" style="width: 200px" placeholder="名称" />
                <el-input-number v-model="form.baseItemPriceYuan" :min="0" :precision="2" :step="1" style="width: 160px; margin-left: 8px" />
                <span class="hint">元。用户下单固定收取</span>
              </el-form-item>
              <el-form-item label="服务项">
                <div class="rows">
                  <div v-for="(it, i) in form.serviceItems" :key="it.code" class="row">
                    <el-input v-model="it.name" placeholder="项目名称" style="width: 260px" />
                    <el-input-number v-model="it.priceYuan" :min="0" :precision="2" :step="1" style="width: 150px; margin-left: 8px" />
                    <span class="row-unit">元</span>
                    <el-button link type="danger" @click="removeServiceItem(i)">删除</el-button>
                  </div>
                  <el-button size="small" @click="addServiceItem">+ 添加服务项</el-button>
                  <div class="hint" style="margin-left: 0">按项计价；用户端展示的项目名与价格即来自这里</div>
                </div>
              </el-form-item>
              <el-form-item label="支持医院">
                <div class="rows">
                  <div v-for="(h, i) in form.hospitals" :key="h.code" class="row">
                    <el-input v-model="h.name" placeholder="医院名称" style="width: 320px" />
                    <el-button link type="danger" @click="removeHospital(i)">删除</el-button>
                  </div>
                  <el-button size="small" @click="addHospital">+ 添加医院</el-button>
                </div>
              </el-form-item>
            </template>

            <!-- 遛宠 -->
            <template v-else-if="isDogwalk">
              <el-form-item label="价格（元）">
                <div class="grid">
                  <div v-for="k in (['base','familiar','over_3km','extra_pet','feed_only','step_price','holiday_extra'] as const)" :key="k" class="cell">
                    <span class="cell-label">{{ DOG_PRICE_LABELS[k] }}</span>
                    <el-input-number v-model="form.dogYuan[k]" :min="0" :precision="2" :step="1" style="width: 150px" />
                  </div>
                </div>
              </el-form-item>
              <el-form-item label="时长 / 规则">
                <div class="grid">
                  <div v-for="k in (['base_minutes','step_minutes','max_minutes','familiar_free_orders','distance_threshold_km'] as const)" :key="k" class="cell">
                    <span class="cell-label">{{ DOG_COUNT_LABELS[k] }}</span>
                    <el-input-number v-model="form.dogCount[k]" :min="0" style="width: 150px" />
                  </div>
                </div>
              </el-form-item>
              <el-form-item label="节日加价日">
                <el-date-picker
                  v-model="form.holidayDates" type="dates" value-format="YYYY-MM-DD"
                  placeholder="选择一个或多个日期" style="width: 420px"
                />
                <div class="hint">所选日期下单按「节日加价」收取</div>
              </el-form-item>
            </template>

            <!-- 垃圾 -->
            <template v-else-if="isGarbage">
              <template v-if="editing.plan === 'monthly'">
                <el-form-item label="包月价">
                  <el-input-number v-model="form.garbage.monthly_priceYuan" :min="0" :precision="2" :step="10" style="width: 200px" />
                  <span class="hint">元 / 月，用户端下单页直接展示这个价</span>
                </el-form-item>
                <el-form-item label="服务天数">
                  <el-input-number v-model="form.garbage.duration_days" :min="1" :max="365" style="width: 200px" />
                  <span class="hint">天，下单后按此推算服务期结束日</span>
                </el-form-item>
                <el-form-item label="默认取件时段">
                  <el-input v-model="form.garbage.default_slot" placeholder="22:00-22:30" style="width: 200px" />
                  <span class="hint">HH:MM-HH:MM，包月订单不选时段时用它</span>
                </el-form-item>
                <el-form-item label="可选额外时段">
                  <el-input
                    v-model="form.garbage.extraSlotText"
                    type="textarea"
                    :rows="4"
                    placeholder="22:30-23:00&#10;07:00-07:30&#10;19:00-19:30"
                    style="width: 320px"
                  />
                  <span class="hint">每行一个；不能与默认时段重复，不填表示只能按默认时段</span>
                </el-form-item>
                <el-form-item label="每日次数上限">
                  <el-input-number v-model="form.garbage.daily_limit" :min="1" :max="10" style="width: 200px" />
                </el-form-item>
                <el-form-item label="次数可结转">
                  <el-switch v-model="form.garbage.carry_over" />
                  <span class="hint">当天未用次数是否结转到次日</span>
                </el-form-item>
              </template>
              <template v-else>
                <el-form-item label="计价单位">
                  <el-radio-group v-model="form.garbage.unit">
                    <el-radio value="bag">按袋</el-radio>
                  </el-radio-group>
                </el-form-item>
                <el-form-item label="含袋数">
                  <el-input-number v-model="form.garbage.bags_included" :min="1" :max="10" style="width: 200px" />
                  <span class="hint">基础价包含的袋数</span>
                </el-form-item>
                <el-form-item label="超袋加价">
                  <el-input-number v-model="form.garbage.extra_bagYuan" :min="0" :precision="2" :step="1" style="width: 200px" />
                  <span class="hint">元 / 袋</span>
                </el-form-item>
                <el-form-item label="包月价">
                  <el-input-number v-model="form.garbage.monthly_priceYuan" :min="0" :precision="2" :step="10" style="width: 200px" />
                  <span class="hint">元 / 月，展示与包月升级参考价</span>
                </el-form-item>
                <el-form-item label="包月每日上限">
                  <el-input-number v-model="form.garbage.monthly_daily_limit" :min="1" :max="10" style="width: 200px" />
                </el-form-item>
              </template>
            </template>
          </template>
        </el-form>

        <div class="advanced-toggle">
          <el-switch :model-value="advanced" @change="(v: unknown) => switchAdvanced(v === true)" />
          <span class="hint">高级 JSON 编辑（结构化表单之外的全部原始键）</span>
        </div>
      </template>
      <template #footer>
        <el-button @click="visible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.toolbar { display: flex; align-items: center; justify-content: space-between; background: #fff; padding: 16px; border-radius: 8px; margin-bottom: 12px; }
.title { font-size: 16px; font-weight: 600; }
.note { color: #6b7280; font-size: 13px; margin-bottom: 12px; line-height: 1.6; }
.rules-preview { display: block; max-width: 420px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 12px; color: #6b7280; }
.svc { display: flex; align-items: center; gap: 8px; }
.svc-name { font-weight: 600; font-size: 14px; color: #1f2937; }
.price { font-weight: 600; color: #0f7f78; }
.sum-line { font-size: 13px; color: #374151; line-height: 1.7; }
.expand-box { padding: 8px 16px 16px 48px; }
.expand-sec { margin-bottom: 14px; }
.expand-title { font-size: 13px; font-weight: 600; color: #111827; margin-bottom: 8px; }
.kv-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 6px 24px; }
.kv { display: flex; gap: 8px; font-size: 13px; line-height: 1.6; }
.kv-label { color: #6b7280; flex-shrink: 0; }
.kv-value { color: #1f2937; }
.mtag { margin: 0 8px 6px 0; }
.rules-json { display: block; background: #f8fafc; border: 1px solid #e5e7eb; border-radius: 6px; padding: 10px 12px; font-family: 'SF Mono', Menlo, Consolas, monospace; font-size: 12px; line-height: 1.5; color: #374151; white-space: pre-wrap; word-break: break-all; max-height: 260px; overflow: auto; }
.hint { color: #9aa3ad; font-size: 12px; margin-left: 12px; line-height: 1.5; }
.err { color: #dc2626; font-size: 12px; margin-top: 4px; }
.rows { display: flex; flex-direction: column; gap: 8px; width: 100%; }
.row { display: flex; align-items: center; gap: 8px; }
.row-unit { color: #6b7280; font-size: 13px; }
.grid { display: grid; grid-template-columns: repeat(2, 1fr); gap: 10px 24px; width: 100%; }
.cell { display: flex; align-items: center; justify-content: space-between; }
.cell-label { color: #374151; font-size: 13px; }
.advanced-toggle { display: flex; align-items: center; margin-top: 8px; padding-top: 12px; border-top: 1px dashed #e5e7eb; }
:deep(.json-input textarea) { font-family: 'SF Mono', Menlo, Consolas, monospace; font-size: 12px; line-height: 1.5; }
</style>
