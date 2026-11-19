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
const toYuan = (fen: unknown) => (typeof fen === 'number' ? fen / 100 : 0)
const toFen = (yuan: number) => Math.round(yuan * 100)

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
  garbage: { unit: 'bag', bags_included: 1, extra_bagYuan: 0, monthly_priceYuan: 0, monthly_daily_limit: 1, carry_over: false, daily_limit: 1 },
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
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column label="服务" width="130">
        <template #default="{ row }">{{ typeLabel(row.service_type) }}</template>
      </el-table-column>
      <el-table-column label="方案" width="90">
        <template #default="{ row }"><el-tag size="small">{{ planLabel(row.plan) }}</el-tag></template>
      </el-table-column>
      <el-table-column label="基础价" width="120" align="right">
        <template #default="{ row }">{{ money(row.base_price) }}</template>
      </el-table-column>
      <el-table-column label="计价规则（rules）" min-width="420">
        <template #default="{ row }">
          <code class="rules-preview">{{ JSON.stringify(row.rules || {}) }}</code>
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

<script lang="ts">
// 标签常量（模板中使用）
export default {
  data() {
    return {
      DOG_PRICE_LABELS: {
        base: '起步价（元）', familiar: '熟悉费（元）', over_3km: '超3km加价（元）',
        extra_pet: '每多一只（元）', feed_only: '仅喂食（元）', step_price: '时长每档加价（元）',
        holiday_extra: '节日加价（元）',
      },
      DOG_COUNT_LABELS: {
        base_minutes: '含时长（分钟）', step_minutes: '每档时长（分钟）', max_minutes: '最长时长（分钟）',
        familiar_free_orders: '免熟悉费订单数', distance_threshold_km: '距离阈值（km）',
      },
    }
  },
}
</script>

<style scoped>
.toolbar { display: flex; align-items: center; justify-content: space-between; background: #fff; padding: 16px; border-radius: 8px; margin-bottom: 12px; }
.title { font-size: 16px; font-weight: 600; }
.note { color: #6b7280; font-size: 13px; margin-bottom: 12px; line-height: 1.6; }
.rules-preview { display: block; max-width: 420px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 12px; color: #6b7280; }
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
