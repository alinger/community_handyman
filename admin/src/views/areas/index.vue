<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  listAreas, createArea, setAreaStatus, updateArea,
  type ServiceArea, type CreateAreaPayload, ApiError,
} from '@/api'

const loading = ref(false)
const rows = ref<ServiceArea[]>([])
const visible = ref(false)
const saving = ref(false)
const editingId = ref<number | null>(null)
const form = reactive<CreateAreaPayload>({
  community: '', lane: '', house_from: 0, house_to: 0, house_parity: 'all',
})

const parityLabel = (p: string) => ({ all: '全部', odd: '单号', even: '双号' }[p] || p)

async function load() {
  loading.value = true
  try {
    rows.value = await listAreas()
  } catch {
    // 拦截器已提示
  } finally {
    loading.value = false
  }
}
onMounted(load)

function openAdd() {
  editingId.value = null
  Object.assign(form, { community: '', lane: '', house_from: 0, house_to: 0, house_parity: 'all' })
  visible.value = true
}

function openEdit(row: ServiceArea) {
  editingId.value = row.id
  Object.assign(form, {
    community: row.community, lane: row.lane,
    house_from: row.house_from, house_to: row.house_to,
    house_parity: (row.house_parity as CreateAreaPayload['house_parity']) || 'all',
  })
  visible.value = true
}

async function save() {
  if (!form.community.trim() || !form.lane.trim()) {
    ElMessage.warning('请填写小区与路弄')
    return
  }
  saving.value = true
  try {
    const payload = { ...form, community: form.community.trim(), lane: form.lane.trim() }
    if (editingId.value) {
      await updateArea(editingId.value, payload)
      ElMessage.success('已保存修改')
    } else {
      await createArea(payload)
      ElMessage.success('已新增服务范围')
    }
    visible.value = false
    load()
  } catch (e) {
    if (e instanceof ApiError) ElMessage.error(e.message)
  } finally {
    saving.value = false
  }
}

async function toggle(row: ServiceArea) {
  const next = row.status === 1 ? 0 : 1
  try {
    await ElMessageBox.confirm(
      next ? `确认启用「${row.community} ${row.lane}」？` : `确认停用「${row.community} ${row.lane}」？停用不影响已存在订单。`,
      next ? '启用' : '停用',
      { type: 'warning' },
    )
    await setAreaStatus(row.id, next as 0 | 1)
    ElMessage.success(next ? '已启用' : '已停用')
    load()
  } catch (e) {
    if (e instanceof ApiError) ElMessage.error(e.message)
    // 用户取消确认不算错误
  }
}
</script>

<template>
  <div>
    <div class="toolbar">
      <span class="title">服务范围规则</span>
      <el-button type="primary" @click="openAdd">＋ 新增范围</el-button>
    </div>
    <div class="note">小区与门牌规则由后台维护，住户新增地址时只能从启用的范围里选择小区，并按门牌区间校验。路弄为空的历史规则按小区+门牌匹配。</div>

    <el-table :data="rows" v-loading="loading" border stripe style="width: 100%">
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="community" label="小区" min-width="160" />
      <el-table-column prop="lane" label="路弄" min-width="180" />
      <el-table-column label="门牌区间" width="160">
        <template #default="{ row }">
          {{ row.house_from || '不限' }} ~ {{ row.house_to || '不限' }}
        </template>
      </el-table-column>
      <el-table-column label="单双号" width="100">
        <template #default="{ row }"><el-tag size="small">{{ parityLabel(row.house_parity) }}</el-tag></template>
      </el-table-column>
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="row.status === 1 ? 'success' : 'info'" size="small">
            {{ row.status === 1 ? '启用' : '停用' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="140" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
          <el-button link :type="row.status === 1 ? 'danger' : 'primary'" @click="toggle(row)">
            {{ row.status === 1 ? '停用' : '启用' }}
          </el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="visible" :title="editingId ? '编辑服务范围' : '新增服务范围'" width="500px">
      <el-form label-width="90px">
        <el-form-item label="小区"><el-input v-model="form.community" placeholder="如：东方冠郡二期" /></el-form-item>
        <el-form-item label="路弄"><el-input v-model="form.lane" placeholder="如：众安路779弄" /></el-form-item>
        <el-form-item label="起始门牌"><el-input-number v-model="form.house_from" :min="0" /></el-form-item>
        <el-form-item label="结束门牌"><el-input-number v-model="form.house_to" :min="0" /><div class="form-hint">0 表示不限</div></el-form-item>
        <el-form-item label="单双号">
          <el-radio-group v-model="form.house_parity">
            <el-radio value="all">全部</el-radio>
            <el-radio value="odd">单号</el-radio>
            <el-radio value="even">双号</el-radio>
          </el-radio-group>
        </el-form-item>
      </el-form>
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
.form-hint { color: #9aa3ad; font-size: 12px; }
</style>
