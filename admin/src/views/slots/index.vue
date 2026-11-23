<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import {
  listSlots, setSlots, copySlots,
  type SlotInventory, type SetSlotItem, ApiError,
} from '@/api'

const loading = ref(false)
const saving = ref(false)
const rows = ref<SlotInventory[]>([])
const date = ref(new Date().toISOString().slice(0, 10))

const serviceTypes = [
  { value: 'garbage', label: '代倒垃圾' },
  { value: 'dogwalk', label: '遛宠' },
  { value: 'accompany', label: '陪诊服务' },
]
// 全天 12 个两小时时段：用户端可选时段直接读这里的配置，
// 铺满全天即可让住户任选任意时间段；total=0 表示该时段不开放
const slotCodes = [
  '00-02', '02-04', '04-06', '06-08',
  '08-10', '10-12', '12-14', '14-16',
  '16-18', '18-20', '20-22', '22-24',
]

const label = (t: string) => serviceTypes.find((x) => x.value === t)?.label || t

// 编辑矩阵：service_type -> slot_code -> total。
// 先按全部服务/时段初始化为 0，避免首屏异步加载完成前模板读到 undefined。
const matrix = reactive<Record<string, Record<string, number>>>({})
serviceTypes.forEach((t) => {
  matrix[t.value] = {}
  slotCodes.forEach((code) => { matrix[t.value][code] = 0 })
})

async function load() {
  if (!date.value) return
  loading.value = true
  try {
    rows.value = await listSlots(date.value)
    // 初始化矩阵：已配置用 total，否则 0
    serviceTypes.forEach((t) => {
      matrix[t.value] = matrix[t.value] || {}
      slotCodes.forEach((code) => {
        const found = rows.value.find((r) => r.service_type === t.value && r.slot_code === code)
        matrix[t.value][code] = found ? found.total : 0
      })
    })
  } catch {
    // 拦截器已提示
  } finally {
    loading.value = false
  }
}
onMounted(load)

const changedCount = computed(() => {
  let n = 0
  serviceTypes.forEach((t) => slotCodes.forEach((code) => {
    const found = rows.value.find((r) => r.service_type === t.value && r.slot_code === code)
    const old = found ? found.total : 0
    if ((matrix[t.value]?.[code] ?? 0) !== old) n++
  }))
  return n
})

async function save() {
  const items: SetSlotItem[] = []
  serviceTypes.forEach((t) => slotCodes.forEach((code) => {
    const v = matrix[t.value]?.[code] ?? 0
    const found = rows.value.find((r) => r.service_type === t.value && r.slot_code === code)
    if (v !== (found ? found.total : 0)) items.push({ slot_code: code, service_type: t.value, total: v })
  }))
  if (!items.length) {
    ElMessage.info('没有改动')
    return
  }
  saving.value = true
  try {
    await setSlots(date.value, items)
    ElMessage.success(`已保存 ${items.length} 项设置`)
    load()
  } catch (e) {
    if (e instanceof ApiError) ElMessage.error(e.message)
  } finally {
    saving.value = false
  }
}

async function fillAll(v: number) {
  serviceTypes.forEach((t) => slotCodes.forEach((code) => { matrix[t.value][code] = v }))
}

// ---- 模板复制：把当前日期的配置复制到一批未来日期 ----
const copyVisible = ref(false)
const copyDates = ref<string[]>([])
const copying = ref(false)
function openCopy() {
  copyDates.value = []
  copyVisible.value = true
}
async function doCopy() {
  if (!copyDates.value.length) {
    ElMessage.warning('请选择目标日期')
    return
  }
  copying.value = true
  try {
    const res = await copySlots(date.value, copyDates.value)
    ElMessage.success(`已复制到 ${copyDates.value.length} 天，共写入 ${res.applied} 个时段`)
    copyVisible.value = false
  } catch (e) {
    if (e instanceof ApiError) ElMessage.error(e.message)
  } finally {
    copying.value = false
  }
}
</script>

<template>
  <div>
    <div class="toolbar">
      <div class="left">
        <span class="title">时段库存</span>
        <el-date-picker v-model="date" type="date" value-format="YYYY-MM-DD" placeholder="选择日期" @change="load" />
        <el-button @click="load">刷新</el-button>
      </div>
      <div class="right">
        <el-button size="small" @click="openCopy">复制到未来日期</el-button>
        <el-button size="small" @click="fillAll(0)">全部置 0</el-button>
        <el-button size="small" @click="fillAll(5)">全部置 5</el-button>
        <el-button type="primary" :loading="saving" :disabled="changedCount === 0" @click="save">
          保存改动<template v-if="changedCount">（{{ changedCount }}）</template>
        </el-button>
      </div>
    </div>
    <div class="note">
      每格是该服务在该时段的可接单总量（total=0 表示不开放）。「已用」为实时占用，修改总量不会清除已用。
      住户端可选时段即本页配置：时段铺满全天（total &gt; 0）即可让住户任选任意时间段。
    </div>

    <el-table :data="serviceTypes" v-loading="loading" border style="width: 100%">
      <el-table-column label="服务类型" width="150">
        <template #default="{ row }"><b>{{ row.label }}</b></template>
      </el-table-column>
      <el-table-column v-for="code in slotCodes" :key="code" :label="code" align="center">
        <template #default="{ row }">
          <el-input-number
            v-model="matrix[row.value][code]"
            :min="0"
            :max="200"
            controls-position="right"
            size="small"
            style="width: 84px"
          />
          <div class="used">
            已用 {{ rows.find((r) => r.service_type === row.value && r.slot_code === code)?.used || 0 }}
          </div>
        </template>
      </el-table-column>
    </el-table>

    <!-- 模板复制 -->
    <el-dialog v-model="copyVisible" title="复制时段配置" width="480px">
      <p class="dialog-hint">
        把 <b>{{ date }}</b> 的全部时段配置复制到目标日期；目标日期已有的配置会被覆盖（已用计数不变）。
      </p>
      <el-date-picker
        v-model="copyDates" type="dates" value-format="YYYY-MM-DD"
        placeholder="选择一个或多个目标日期" style="width: 100%"
      />
      <template #footer>
        <el-button @click="copyVisible = false">取消</el-button>
        <el-button type="primary" :loading="copying" @click="doCopy">确认复制</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.toolbar { display: flex; align-items: center; justify-content: space-between; background: #fff; padding: 16px; border-radius: 8px; margin-bottom: 12px; }
.left, .right { display: flex; align-items: center; gap: 10px; }
.title { font-size: 16px; font-weight: 600; margin-right: 8px; }
.note { color: #6b7280; font-size: 13px; margin-bottom: 12px; }
.used { color: #9aa3ad; font-size: 11px; margin-top: 4px; }
.dialog-hint { margin: 0 0 16px; color: #6b7280; font-size: 13px; line-height: 1.6; }
</style>
