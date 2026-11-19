<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { listSkus, updateSku, type ServiceSku, ApiError } from '@/api'

const loading = ref(false)
const rows = ref<ServiceSku[]>([])
const visible = ref(false)
const saving = ref(false)
const editing = ref<ServiceSku | null>(null)
const basePrice = ref(0)
const rulesText = ref('{}')
const ruleError = ref('')

const typeLabel = (t: string) => ({ garbage: '代倒垃圾', dogwalk: '遛宠', accompany: '陪诊服务' }[t] || t)
const planLabel = (p: string) => (p === 'monthly' ? '包月' : '单次')
const money = (fen: number) => `¥${(fen / 100).toFixed(2)}`

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
  basePrice.value = sku.base_price
  rulesText.value = JSON.stringify(sku.rules ?? {}, null, 2)
  ruleError.value = ''
  visible.value = true
}

async function save() {
  if (!editing.value) return
  let parsed: Record<string, unknown>
  try {
    parsed = JSON.parse(rulesText.value)
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) throw new Error('规则必须是 JSON 对象')
    ruleError.value = ''
  } catch (e) {
    ruleError.value = e instanceof Error ? e.message : 'JSON 格式错误'
    return
  }
  saving.value = true
  try {
    await updateSku(editing.value.id, basePrice.value, parsed)
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
      基础价单位为分；rules 是计价规则（陪诊医院/服务项/基础费、加价日期等）。保存时 rules 做 JSON 部分合并，只提交要修改的键；下方展示的是当前完整规则，可直接编辑后整体提交。改价改规则即时生效、无需发版。
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

    <el-dialog v-model="visible" title="编辑服务价格与规则" width="720px">
      <template v-if="editing">
        <el-form label-width="100px">
          <el-form-item label="服务">
            <el-input :model-value="`${typeLabel(editing.service_type)} · ${planLabel(editing.plan)}`" disabled />
          </el-form-item>
          <el-form-item label="基础价（分）">
            <el-input-number v-model="basePrice" :min="0" :step="100" style="width: 220px" />
            <span class="hint">= {{ money(basePrice) }}</span>
          </el-form-item>
          <el-form-item label="计价规则">
            <el-input v-model="rulesText" type="textarea" :rows="16" spellcheck="false" class="json-input" />
            <div v-if="ruleError" class="err">{{ ruleError }}</div>
            <div class="hint">JSON 对象。陪诊可编辑 hospitals / service_items / base_item；通用可编辑 holiday_dates、billing 等。</div>
          </el-form-item>
        </el-form>
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
.hint { color: #9aa3ad; font-size: 12px; margin-left: 12px; }
.err { color: #dc2626; font-size: 12px; margin-top: 4px; }
:deep(.json-input textarea) { font-family: 'SF Mono', Menlo, Consolas, monospace; font-size: 12px; line-height: 1.5; }
</style>
