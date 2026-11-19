<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  approveProviderApp,
  getToken,
  listProviderApps,
  providerAppDetail,
  providerAppStats,
  providerCertUrl,
  rejectProviderApp,
  type ProviderAppRow,
  ApiError,
} from '@/api'

const loading = ref(false)
const rows = ref<ProviderAppRow[]>([])
const total = ref(0)
const stats = reactive({ pending: 0, approved: 0, rejected: 0 })
const query = reactive({ status: 'pending', page: 1, size: 20 })

const STATUS_TABS = [
  { key: 'pending', label: '待审核' },
  { key: 'approved', label: '已通过' },
  { key: 'rejected', label: '已驳回' },
  { key: '', label: '全部' },
]

const SERVICE_LABEL: Record<string, string> = {
  all: '三类都接',
  garbage: '代倒垃圾',
  dogwalk: '遛宠',
  accompany: '陪诊',
}

const statusType = (s: string) =>
  s === 'approved' ? 'success' : s === 'rejected' ? 'danger' : 'warning'
const serviceLabel = (v: string) => SERVICE_LABEL[v] || v

// 详情抽屉
const detailVisible = ref(false)
const detail = ref<ProviderAppRow | null>(null)

// 附件预览
const previewVisible = ref(false)
const previewSrc = ref('')
const previewTitle = ref('')

// 驳回弹窗
const rejectVisible = ref(false)
const rejectId = ref(0)
const rejectNote = ref('')
const rejecting = ref(false)

const pendingCount = computed(() => stats.pending)

async function load() {
  loading.value = true
  try {
    const res = await listProviderApps(query)
    rows.value = res.list || []
    total.value = res.total
  } catch (e) {
    if (e instanceof ApiError) ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

async function loadStats() {
  try {
    const s = await providerAppStats()
    stats.pending = s.pending
    stats.approved = s.approved
    stats.rejected = s.rejected
  } catch {
    // 统计失败不影响列表
  }
}

onMounted(() => {
  load()
  loadStats()
})

function switchStatus(key: string) {
  if (query.status === key) return
  query.status = key
  query.page = 1
  load()
}

function reset() {
  query.status = 'pending'
  query.page = 1
  load()
}

async function openDetail(row: ProviderAppRow) {
  try {
    detail.value = await providerAppDetail(row.id)
    detailVisible.value = true
  } catch (e) {
    if (e instanceof ApiError) ElMessage.error(e.message)
  }
}

/** 附件要走鉴权接口，先 fetch 成 blob 再用 objectURL 预览 */
async function openCert(row: ProviderAppRow, kind: 'front' | 'back' | 'health') {
  const titleMap = { front: '身份证人像面', back: '身份证国徽面', health: '健康证' }
  try {
    const res = await fetch(providerCertUrl(row.id, kind), {
      headers: { Authorization: `Bearer ${getToken()}` },
    })
    if (!res.ok) {
      ElMessage.error('附件读取失败，可能未上传')
      return
    }
    const blob = await res.blob()
    previewSrc.value = URL.createObjectURL(blob)
    previewTitle.value = `${row.real_name} · ${titleMap[kind]}`
    previewVisible.value = true
  } catch {
    ElMessage.error('附件读取失败')
  }
}

function closePreview() {
  previewVisible.value = false
  if (previewSrc.value) {
    URL.revokeObjectURL(previewSrc.value)
    previewSrc.value = ''
  }
}

async function onApprove(row: ProviderAppRow) {
  const ok = await new Promise<boolean>((resolve) => {
    ElMessageBox.confirm(
      `确认通过「${row.real_name}」的服务方认证？通过后该用户身份将变为服务方，可承接订单。`,
      '通过认证',
      { confirmButtonText: '确认通过', cancelButtonText: '取消', type: 'warning' },
    )
      .then(() => resolve(true))
      .catch(() => resolve(false))
  })
  if (!ok) return
  try {
    await approveProviderApp(row.id)
    ElMessage.success('已通过，该用户已获得服务方权限')
    detailVisible.value = false
    load()
    loadStats()
  } catch (e) {
    if (e instanceof ApiError) ElMessage.error(e.message)
  }
}

function openReject(row: ProviderAppRow) {
  rejectId.value = row.id
  rejectNote.value = ''
  rejectVisible.value = true
}

async function onReject() {
  if (!rejectNote.value.trim()) {
    ElMessage.warning('请填写驳回原因，用户需要据此修改')
    return
  }
  rejecting.value = true
  try {
    await rejectProviderApp(rejectId.value, rejectNote.value.trim())
    ElMessage.success('已驳回')
    rejectVisible.value = false
    detailVisible.value = false
    load()
    loadStats()
  } catch (e) {
    if (e instanceof ApiError) ElMessage.error(e.message)
  } finally {
    rejecting.value = false
  }
}
</script>

<template>
  <div>
    <div class="toolbar">
      <div class="stat-row">
        <div class="stat">
          <div class="stat-num pending">{{ stats.pending }}</div>
          <div class="stat-label">待审核</div>
        </div>
        <div class="stat">
          <div class="stat-num approved">{{ stats.approved }}</div>
          <div class="stat-label">已通过</div>
        </div>
        <div class="stat">
          <div class="stat-num rejected">{{ stats.rejected }}</div>
          <div class="stat-label">已驳回</div>
        </div>
      </div>
      <el-button @click="load">刷新</el-button>
    </div>

    <div class="note">
      认证通过后该用户身份自动变为服务方，可进入服务方工作台接单；驳回需填写原因，用户可修改后重新提交。
    </div>

    <el-tabs :model-value="query.status" @tab-change="switchStatus">
      <el-tab-pane
        v-for="t in STATUS_TABS"
        :key="t.key"
        :name="t.key"
        :label="t.key === 'pending' && pendingCount > 0 ? `${t.label} (${pendingCount})` : t.label"
      />
    </el-tabs>

    <el-table :data="rows" v-loading="loading" border stripe style="width: 100%">
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column label="申请人" width="120" show-overflow-tooltip>
        <template #default="{ row }">
          <div class="applicant">{{ row.real_name }}</div>
          <div class="sub">{{ row.user_nickname }}</div>
        </template>
      </el-table-column>
      <el-table-column label="联系电话" width="126" />
      <el-table-column label="承接类型" width="104">
        <template #default="{ row }"><el-tag size="small">{{ serviceLabel(row.service_type) }}</el-tag></template>
      </el-table-column>
      <el-table-column label="经验说明" min-width="200" show-overflow-tooltip>
        <template #default="{ row }">{{ row.experience }}</template>
      </el-table-column>
      <el-table-column label="证件" width="150">
        <template #default="{ row }">
          <el-link v-if="row.has_front" type="primary" :underline="false" @click="openCert(row, 'front')">人像面</el-link>
          <el-link v-if="row.has_back" type="primary" :underline="false" class="cert-link" @click="openCert(row, 'back')">国徽面</el-link>
          <el-link v-if="row.has_health" type="primary" :underline="false" class="cert-link" @click="openCert(row, 'health')">健康证</el-link>
          <span v-if="!row.has_front && !row.has_back && !row.has_health" class="sub">无</span>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="96">
        <template #default="{ row }">
          <el-tag :type="statusType(row.status)" size="small">{{ row.status_label }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="提交时间" width="120">
        <template #default="{ row }">{{ row.created_at }}</template>
      </el-table-column>
      <el-table-column label="操作" width="180" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openDetail(row)">详情</el-button>
          <template v-if="row.status === 'pending'">
            <el-button link type="success" @click="onApprove(row)">通过</el-button>
            <el-button link type="danger" @click="openReject(row)">驳回</el-button>
          </template>
        </template>
      </el-table-column>
    </el-table>

    <el-pagination
      class="pager"
      background
      layout="total, sizes, prev, pager, next"
      :total="total"
      v-model:current-page="query.page"
      v-model:page-size="query.size"
      :page-sizes="[10, 20, 50]"
      @current-change="load"
      @size-change="load"
    />

    <!-- 详情抽屉 -->
    <el-drawer v-model="detailVisible" title="认证申请详情" size="520px">
      <template v-if="detail">
        <el-descriptions :column="1" border>
          <el-descriptions-item label="申请人">{{ detail.real_name }}（{{ detail.user_nickname }}）</el-descriptions-item>
          <el-descriptions-item label="联系电话">{{ detail.phone }}</el-descriptions-item>
          <el-descriptions-item label="身份证号">
            <span class="mono">{{ detail.id_card_no || '未填写' }}</span>
          </el-descriptions-item>
          <el-descriptions-item label="承接类型">{{ serviceLabel(detail.service_type) }}</el-descriptions-item>
          <el-descriptions-item label="经验说明">{{ detail.experience }}</el-descriptions-item>
          <el-descriptions-item label="当前状态">
            <el-tag :type="statusType(detail.status)" size="small">{{ detail.status_label }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="提交时间">{{ detail.created_at }}</el-descriptions-item>
          <el-descriptions-item v-if="detail.reviewed_at" label="审核时间">{{ detail.reviewed_at }}</el-descriptions-item>
          <el-descriptions-item v-if="detail.review_note" label="审核意见">
            <span class="reject-note">{{ detail.review_note }}</span>
          </el-descriptions-item>
        </el-descriptions>

        <div class="cert-block">
          <div class="cert-block-title">证件照片</div>
          <div class="cert-actions">
            <el-button v-if="detail.has_front" size="small" @click="openCert(detail, 'front')">查看人像面</el-button>
            <el-button v-if="detail.has_back" size="small" @click="openCert(detail, 'back')">查看国徽面</el-button>
            <el-button v-if="detail.has_health" size="small" @click="openCert(detail, 'health')">查看健康证</el-button>
            <span v-if="!detail.has_front && !detail.has_back && !detail.has_health" class="sub">未上传</span>
          </div>
        </div>

        <div v-if="detail.status === 'pending'" class="drawer-actions">
          <el-button type="danger" plain @click="openReject(detail)">驳回</el-button>
          <el-button type="primary" @click="onApprove(detail)">通过认证</el-button>
        </div>
      </template>
    </el-drawer>

    <!-- 附件预览 -->
    <el-dialog v-model="previewVisible" :title="previewTitle" width="680px" @close="closePreview">
      <img v-if="previewSrc" :src="previewSrc" class="cert-img" alt="证件照" />
    </el-dialog>

    <!-- 驳回弹窗 -->
    <el-dialog v-model="rejectVisible" title="驳回申请" width="480px">
      <p class="dialog-hint">驳回原因会展示给申请人，请写清楚需要改什么。</p>
      <el-input
        v-model="rejectNote"
        type="textarea"
        :rows="4"
        maxlength="200"
        show-word-limit
        placeholder="例如：身份证照片模糊看不清号码，请重新上传"
      />
      <template #footer>
        <el-button @click="rejectVisible = false">取消</el-button>
        <el-button type="danger" :loading="rejecting" @click="onReject">确认驳回</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: #fff;
  padding: 16px 20px;
  border-radius: 8px;
  margin-bottom: 12px;
}
.stat-row { display: flex; gap: 40px; }
.stat { text-align: center; min-width: 64px; }
.stat-num { font-size: 24px; font-weight: 700; line-height: 1.2; }
.stat-num.pending { color: #e6a23c; }
.stat-num.approved { color: #0f7f78; }
.stat-num.rejected { color: #dc2626; }
.stat-label { font-size: 12px; color: #6b7280; margin-top: 2px; }
.note { color: #6b7280; font-size: 13px; margin-bottom: 12px; line-height: 1.6; }
.applicant { font-weight: 600; font-size: 13px; }
.sub { color: #9aa3ad; font-size: 12px; }
.cert-link { margin-left: 8px; }
.pager { margin-top: 16px; justify-content: flex-end; }
.mono { font-family: 'SF Mono', Menlo, Consolas, monospace; }
.reject-note { color: #dc2626; }
.cert-block { margin-top: 20px; }
.cert-block-title { font-size: 14px; font-weight: 600; margin-bottom: 10px; }
.cert-actions { display: flex; gap: 8px; align-items: center; }
.drawer-actions {
  display: flex; justify-content: flex-end; gap: 12px;
  margin-top: 28px; padding-top: 18px; border-top: 1px solid #eef0f2;
}
.cert-img { width: 100%; display: block; border-radius: 6px; }
.dialog-hint { margin: 0 0 12px; color: #6b7280; font-size: 13px; }
</style>
