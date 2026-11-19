<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  listOrders, listProviders, assignOrder, refund, redeem, quote,
  listOrderEvents, listOrderRecords, fetchRecordBlobUrl, exportOrdersUrl,
  cancelOrderForUser, rescheduleOrder,
  type Order, type ProviderOption, type OrderQuery, type AdminEvent, type AdminRecord, ApiError,
} from '@/api'
import { useAuthStore } from '@/store/auth'

const auth = useAuthStore()
const loading = ref(false)
const rows = ref<Order[]>([])
const total = ref(0)
const providers = ref<ProviderOption[]>([])
const query = reactive<OrderQuery>({ status: '', service_type: '', keyword: '', page: 1, size: 10 })

const statusOptions = [
  { value: 'pending_payment', label: '待支付' },
  { value: 'pending_accept', label: '待接单' },
  { value: 'accepted', label: '已受理' },
  { value: 'on_the_way', label: '待上门' },
  { value: 'in_service', label: '服务中' },
  { value: 'pending_confirm', label: '待完成' },
  { value: 'completed', label: '已完成' },
  { value: 'canceled', label: '已取消' },
  { value: 'rejected', label: '已拒单' },
]
const typeOptions = [
  { value: 'garbage', label: '代倒垃圾' },
  { value: 'dogwalk', label: '遛宠' },
  { value: 'accompany', label: '陪诊服务' },
]

const money = (fen: number) => `¥${(fen / 100).toFixed(2)}`
const typeLabel = (t: string) => typeOptions.find((x) => x.value === t)?.label || t
const statusType = (s: string): 'success' | 'warning' | 'info' | 'danger' | 'primary' => {
  if (s === 'completed') return 'success'
  if (['pending_payment', 'pending_accept'].includes(s)) return 'warning'
  if (['canceled', 'rejected'].includes(s)) return 'info'
  if (['in_service', 'on_the_way'].includes(s)) return 'danger'
  return 'primary'
}

async function load() {
  loading.value = true
  try {
    const page = await listOrders(query)
    rows.value = page.list
    total.value = page.total
  } catch {
    // 拦截器已提示
  } finally {
    loading.value = false
  }
}

function reset() {
  query.status = ''
  query.service_type = ''
  query.keyword = ''
  query.page = 1
  load()
}

async function loadProviders() {
  try {
    providers.value = await listProviders()
  } catch {
    // 派单时才需要，加载失败不阻塞列表
  }
}

onMounted(() => {
  load()
  loadProviders()
})

// ---- 详情抽屉 ----
const detailVisible = ref(false)
const detail = ref<Order | null>(null)
const events = ref<AdminEvent[]>([])
const records = ref<AdminRecord[]>([])
const recordUrls = new Map<number, string>() // id -> blob url

const NODE_LABELS: Record<string, string> = {
  created: '创建订单', paid: '支付成功', pending_accept: '待接单', assigned: '已派单',
  accepted: '服务方接单', on_the_way: '前往中', in_service: '服务开始',
  pending_confirm: '服务完成待确认', completed: '订单完成', canceled: '订单取消', rejected: '已拒单',
}

async function openDetail(o: Order) {
  detail.value = o
  detailVisible.value = true
  events.value = []
  records.value = []
  recordUrls.forEach((u) => URL.revokeObjectURL(u))
  recordUrls.clear()
  listOrderEvents(o.order_no).then((v) => { events.value = v }).catch(() => {})
  listOrderRecords(o.order_no).then(async (v) => {
    records.value = v
    for (const r of v) {
      try {
        recordUrls.set(r.id, await fetchRecordBlobUrl(r))
      } catch {
        // 预览失败不阻塞
      }
    }
  }).catch(() => {})
}

const MEDIA_LABELS: Record<string, string> = {
  photo: '照片', video: '视频', advice: '医嘱', document: '文书',
}
const recordBlobUrl = (r: AdminRecord) => recordUrls.get(r.id) || ''
function previewRecord(r: AdminRecord) {
  const url = recordBlobUrl(r)
  if (!url) {
    ElMessage.warning('文件加载中或加载失败，请稍后再试')
    return
  }
  window.open(url, '_blank')
}

// ---- 派单 ----
const assignVisible = ref(false)
const assignOrderNo = ref('')
const assignProvider = ref<number | null>(null)
const assigning = ref(false)
function openAssign(o: Order) {
  assignOrderNo.value = o.order_no
  assignProvider.value = null
  assignVisible.value = true
}
async function doAssign() {
  if (!assignProvider.value) {
    ElMessage.warning('请选择服务方')
    return
  }
  assigning.value = true
  try {
    await assignOrder(assignOrderNo.value, assignProvider.value)
    ElMessage.success('派单成功')
    assignVisible.value = false
    load()
  } catch (e) {
    if (e instanceof ApiError) ElMessage.error(e.message)
  } finally {
    assigning.value = false
  }
}

// ---- 退款 ----
const refundVisible = ref(false)
const refundForm = reactive({ orderNo: '', amount: 0, reason: '' })
const refunding = ref(false)
function openRefund(o: Order) {
  refundForm.orderNo = o.order_no
  refundForm.amount = 0
  refundForm.reason = ''
  refundVisible.value = true
}
async function doRefund() {
  if (!refundForm.reason.trim()) {
    ElMessage.warning('请填写退款原因')
    return
  }
  refunding.value = true
  try {
    await refund(refundForm.orderNo, refundForm.amount, refundForm.reason.trim())
    ElMessage.success('退款已发起')
    refundVisible.value = false
    load()
  } catch (e) {
    if (e instanceof ApiError) ElMessage.error(e.message)
  } finally {
    refunding.value = false
  }
}

// ---- 包月核销 ----
const redeemVisible = ref(false)
const redeemForm = reactive({ orderNo: '', date: '', remark: '' })
async function openRedeem(o: Order) {
  redeemForm.orderNo = o.order_no
  redeemForm.date = o.service_date || new Date().toISOString().slice(0, 10)
  redeemForm.remark = ''
  redeemVisible.value = true
}
async function doRedeem() {
  if (!redeemForm.date) {
    ElMessage.warning('请选择核销日期')
    return
  }
  try {
    await redeem(redeemForm.orderNo, redeemForm.date, redeemForm.remark.trim())
    ElMessage.success('核销成功')
    redeemVisible.value = false
    load()
  } catch (e) {
    if (e instanceof ApiError) ElMessage.error(e.message)
  }
}

// ---- 历史陪诊报价（兼容金额为 0 的旧订单）----
const quoteVisible = ref(false)
const quoteForm = reactive({ orderNo: '', amount: 0, durationMin: 0, note: '' })
async function openQuote(o: Order) {
  quoteForm.orderNo = o.order_no
  quoteForm.amount = 0
  quoteForm.durationMin = 0
  quoteForm.note = ''
  quoteVisible.value = true
}
async function doQuote() {
  if (quoteForm.amount <= 0) {
    ElMessage.warning('请输入报价金额')
    return
  }
  try {
    await quote(quoteForm.orderNo, quoteForm.amount, quoteForm.durationMin, quoteForm.note.trim())
    ElMessage.success('报价已提交')
    quoteVisible.value = false
    load()
  } catch (e) {
    if (e instanceof ApiError) ElMessage.error(e.message)
  }
}

// ---- 代客取消 / 改期 ----
const ADMIN_CANCELABLE = ['pending_payment', 'pending_accept', 'accepted', 'on_the_way']
const adminCancelable = (s: string) => ADMIN_CANCELABLE.includes(s)

const cancelVisible = ref(false)
const cancelForm = reactive({ orderNo: '', reason: '' })
const canceling = ref(false)
function openCancel(o: Order) {
  cancelForm.orderNo = o.order_no
  cancelForm.reason = ''
  cancelVisible.value = true
}
async function doCancel() {
  if (!cancelForm.reason.trim()) {
    ElMessage.warning('请填写取消原因')
    return
  }
  canceling.value = true
  try {
    const res = await cancelOrderForUser(cancelForm.orderNo, cancelForm.reason.trim())
    if (res.canceled && res.refund_due > 0 && res.refund_ok) {
      ElMessage.success(`已取消并全额退款 ¥${(res.refund_due / 100).toFixed(2)}`)
    } else if (res.canceled && res.refund_due > 0 && !res.refund_ok) {
      ElMessage.warning(res.refund_hint || '订单已取消，退款处理失败，请到财务管理检查退款单')
    } else {
      ElMessage.success('订单已取消')
    }
    cancelVisible.value = false
    load()
  } catch (e) {
    if (e instanceof ApiError) ElMessage.error(e.message)
  } finally {
    canceling.value = false
  }
}

const RESLOT_OPTIONS = ['08-10', '10-12', '12-14', '14-16', '16-18', '18-20']
const rescheduleVisible = ref(false)
const rescheduleForm = reactive({ orderNo: '', cur: '', date: '', slot: '' })
const rescheduling = ref(false)
function openReschedule(o: Order) {
  rescheduleForm.orderNo = o.order_no
  rescheduleForm.cur = `${o.service_date} ${o.slot_code}`
  rescheduleForm.date = ''
  rescheduleForm.slot = ''
  rescheduleVisible.value = true
}
async function doReschedule() {
  if (!rescheduleForm.date || !rescheduleForm.slot) {
    ElMessage.warning('请选择新日期与新时段')
    return
  }
  rescheduling.value = true
  try {
    await rescheduleOrder(rescheduleForm.orderNo, rescheduleForm.date, rescheduleForm.slot)
    ElMessage.success('改期成功')
    rescheduleVisible.value = false
    load()
  } catch (e) {
    if (e instanceof ApiError) ElMessage.error(e.message)
  } finally {
    rescheduling.value = false
  }
}

const title = computed(() => `${auth.username || '运营'} · 订单管理`)

// ---- CSV 导出（带当前筛选条件）----
const exporting = ref(false)
async function onExport() {
  exporting.value = true
  try {
    // Bearer 鉴权不能直接 window.open，走 fetch + blob
    const token = auth.token
    const res = await fetch(exportOrdersUrl({
      status: query.status || undefined,
      service_type: query.service_type || undefined,
      keyword: query.keyword || undefined,
    }), { headers: { Authorization: `Bearer ${token}` } })
    if (!res.ok) throw new Error('导出失败')
    const blob = await res.blob()
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `orders_${new Date().toISOString().slice(0, 10)}.csv`
    a.click()
    URL.revokeObjectURL(url)
  } catch {
    ElMessage.error('导出失败，请重试')
  } finally {
    exporting.value = false
  }
}
</script>

<template>
  <div>
    <div class="toolbar">
      <el-form inline @submit.prevent>
        <el-form-item label="订单号">
          <el-input v-model="query.keyword" placeholder="模糊搜索" clearable style="width: 200px" @keyup.enter="load" />
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="query.status" clearable placeholder="全部" style="width: 140px">
            <el-option v-for="s in statusOptions" :key="s.value" :label="s.label" :value="s.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="服务">
          <el-select v-model="query.service_type" clearable placeholder="全部" style="width: 140px">
            <el-option v-for="t in typeOptions" :key="t.value" :label="t.label" :value="t.value" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="load">查询</el-button>
          <el-button @click="reset">重置</el-button>
          <el-button :loading="exporting" @click="onExport">导出 CSV</el-button>
        </el-form-item>
      </el-form>
    </div>

    <el-table :data="rows" v-loading="loading" border stripe style="width: 100%">
      <el-table-column prop="order_no" label="订单号" width="190" />
      <el-table-column label="服务" width="110">
        <template #default="{ row }"><el-tag size="small">{{ typeLabel(row.service_type) }}</el-tag></template>
      </el-table-column>
      <el-table-column label="状态" width="110">
        <template #default="{ row }">
          <el-tag :type="statusType(row.status)" size="small">{{ row.status_label }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="金额" width="110" align="right">
        <template #default="{ row }">{{ money(row.total_amount) }}</template>
      </el-table-column>
      <el-table-column prop="service_date" label="服务日期" width="120" />
      <el-table-column label="联系人" min-width="160">
        <template #default="{ row }">
          {{ row.address?.contact_name || '-' }} {{ row.address?.contact_phone || '' }}
        </template>
      </el-table-column>
      <el-table-column label="操作" width="360" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openDetail(row)">详情</el-button>
          <el-button v-if="row.status === 'pending_accept'" link type="primary" @click="openAssign(row)">派单</el-button>
          <el-button v-if="adminCancelable(row.status)" link type="warning" @click="openReschedule(row)">改期</el-button>
          <el-button v-if="adminCancelable(row.status)" link type="danger" @click="openCancel(row)">代客取消</el-button>
          <el-button v-if="row.plan === 'monthly' && row.status !== 'canceled'" link type="primary" @click="openRedeem(row)">核销</el-button>
          <el-button v-if="row.status === 'pending_accept' && row.total_amount === 0" link type="primary" @click="openQuote(row)">历史报价</el-button>
          <el-button v-if="row.paid_amount > 0 && !['canceled', 'completed'].includes(row.status)" link type="danger" @click="openRefund(row)">退款</el-button>
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

    <!-- 详情 -->
    <el-drawer v-model="detailVisible" title="订单详情" size="560px">
      <template v-if="detail">
        <el-descriptions :column="2" border>
          <el-descriptions-item label="订单号" :span="2">{{ detail.order_no }}</el-descriptions-item>
          <el-descriptions-item label="服务">{{ typeLabel(detail.service_type) }}</el-descriptions-item>
          <el-descriptions-item label="状态">{{ detail.status_label }}</el-descriptions-item>
          <el-descriptions-item label="服务时间">{{ detail.service_date }} {{ detail.slot_code }}</el-descriptions-item>
          <el-descriptions-item label="金额">{{ money(detail.total_amount) }}</el-descriptions-item>
          <el-descriptions-item label="已付">{{ money(detail.paid_amount) }}</el-descriptions-item>
          <el-descriptions-item label="下单时间">{{ detail.created_at?.slice(0, 19).replace('T', ' ') }}</el-descriptions-item>
          <el-descriptions-item label="服务地址" :span="2">
            {{ detail.address?.community }} {{ detail.address?.house_no }}
          </el-descriptions-item>
          <el-descriptions-item label="联系人" :span="2">
            {{ detail.address?.contact_name }} {{ detail.address?.contact_phone }}
          </el-descriptions-item>
          <el-descriptions-item v-if="detail.hospital" label="就诊医院">{{ detail.hospital }}</el-descriptions-item>
          <el-descriptions-item v-if="detail.patient_name" label="就诊人">{{ detail.patient_name }}</el-descriptions-item>
          <el-descriptions-item v-if="detail.remark" label="备注" :span="2">{{ detail.remark }}</el-descriptions-item>
        </el-descriptions>

        <h4 class="sec">费用明细</h4>
        <el-table :data="detail.price_items || []" size="small" border>
          <el-table-column prop="name" label="项目" />
          <el-table-column label="金额" width="110" align="right">
            <template #default="{ row }">{{ money(row.amount) }}</template>
          </el-table-column>
          <el-table-column prop="note" label="备注" />
        </el-table>

        <h4 class="sec">服务进度</h4>
        <el-timeline>
          <el-timeline-item
            v-for="n in detail.nodes"
            :key="n.node"
            :timestamp="n.at ? n.at.slice(0, 19).replace('T', ' ') : ''"
            :color="n.reached ? '#0f7f78' : '#dcdfe6'"
          >
            {{ n.label }}<span v-if="!n.reached" class="muted">（未到达）</span>
          </el-timeline-item>
        </el-timeline>

        <h4 class="sec">操作轨迹</h4>
        <el-timeline v-if="events.length">
          <el-timeline-item
            v-for="e in events"
            :key="e.id"
            :timestamp="e.created_at?.slice(0, 19).replace('T', ' ')"
            color="#67c23a"
          >
            {{ NODE_LABELS[e.node] || e.node }}
            <el-tag size="small" type="info" class="op-tag">{{ e.operator }}</el-tag>
            <span v-if="e.remark" class="muted">（{{ e.remark }}）</span>
          </el-timeline-item>
        </el-timeline>
        <p v-else class="muted">暂无操作记录</p>

        <h4 class="sec">服务记录</h4>
        <div v-if="records.length" class="records">
          <div
            v-for="r in records"
            :key="r.id"
            class="record-card"
            @click="previewRecord(r)"
          >
            <img
              v-if="r.media_type === 'photo' && recordBlobUrl(r)"
              :src="recordBlobUrl(r)"
              class="record-thumb"
            />
            <div v-else class="record-thumb record-doc">{{ MEDIA_LABELS[r.media_type] || r.media_type }}</div>
            <div class="record-meta">
              <div>{{ MEDIA_LABELS[r.media_type] || r.media_type }} · #{{ r.id }}</div>
              <div class="muted">
                {{ r.uploader_role === 'provider' ? '服务方' : '住户' }} ·
                {{ (r.size / 1024).toFixed(0) }}KB
                <template v-if="r.media_type === 'video'"> · {{ r.duration_sec }}s</template>
              </div>
            </div>
          </div>
        </div>
        <p v-else class="muted">服务方尚未上传记录</p>
      </template>
    </el-drawer>

    <!-- 派单 -->
    <el-dialog v-model="assignVisible" title="派单 / 改派" width="420px">
      <p class="dialog-hint">订单：{{ assignOrderNo }}</p>
      <el-select v-model="assignProvider" placeholder="选择服务方" style="width: 100%" filterable>
        <el-option
          v-for="p in providers"
          :key="p.id"
          :label="`${p.nickname || p.openid}（ID ${p.id}）`"
          :value="p.id"
        />
      </el-select>
      <template #footer>
        <el-button @click="assignVisible = false">取消</el-button>
        <el-button type="primary" :loading="assigning" @click="doAssign">确认派单</el-button>
      </template>
    </el-dialog>

    <!-- 退款 -->
    <el-dialog v-model="refundVisible" title="发起退款" width="460px">
      <p class="dialog-hint">订单：{{ refundForm.orderNo }}</p>
      <el-form label-width="90px">
        <el-form-item label="退款金额">
          <el-input-number v-model="refundForm.amount" :min="0" :step="100" style="width: 100%" />
          <div class="form-hint">单位：分。填 0 表示全额退剩余部分。</div>
        </el-form-item>
        <el-form-item label="退款原因">
          <el-input v-model="refundForm.reason" type="textarea" :rows="3" placeholder="请填写原因" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="refundVisible = false">取消</el-button>
        <el-button type="danger" :loading="refunding" @click="doRefund">确认退款</el-button>
      </template>
    </el-dialog>

    <!-- 核销 -->
    <el-dialog v-model="redeemVisible" title="包月核销" width="440px">
      <el-form label-width="90px">
        <el-form-item label="核销日期"><el-date-picker v-model="redeemForm.date" type="date" value-format="YYYY-MM-DD" style="width: 100%" /></el-form-item>
        <el-form-item label="备注"><el-input v-model="redeemForm.remark" placeholder="如：首日服务完成" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="redeemVisible = false">取消</el-button>
        <el-button type="primary" @click="doRedeem">确认核销</el-button>
      </template>
    </el-dialog>

    <!-- 历史报价 -->
    <el-dialog v-model="quoteVisible" title="历史陪诊报价（兼容旧订单）" width="460px">
      <p class="dialog-hint">仅金额为 0 的历史订单可报价；新订单已按配置自动计价。</p>
      <el-form label-width="90px">
        <el-form-item label="报价金额"><el-input-number v-model="quoteForm.amount" :min="1" :step="100" style="width: 100%" /><div class="form-hint">单位：分</div></el-form-item>
        <el-form-item label="服务时长"><el-input-number v-model="quoteForm.durationMin" :min="0" :max="600" style="width: 100%" /><div class="form-hint">单位：分钟</div></el-form-item>
        <el-form-item label="备注"><el-input v-model="quoteForm.note" type="textarea" :rows="2" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="quoteVisible = false">取消</el-button>
        <el-button type="primary" @click="doQuote">提交报价</el-button>
      </template>
    </el-dialog>
    <!-- 代客取消 -->
    <el-dialog v-model="cancelVisible" title="代客取消" width="460px">
      <p class="dialog-hint">订单：{{ cancelForm.orderNo }}。关单并释放时段；已支付订单将自动全额退剩余部分。</p>
      <el-form label-width="90px">
        <el-form-item label="取消原因">
          <el-input v-model="cancelForm.reason" type="textarea" :rows="3" placeholder="必填，将写入订单轨迹" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="cancelVisible = false">取消</el-button>
        <el-button type="danger" :loading="canceling" @click="doCancel">确认取消订单</el-button>
      </template>
    </el-dialog>

    <!-- 代客改期 -->
    <el-dialog v-model="rescheduleVisible" title="代客改期" width="440px">
      <p class="dialog-hint">订单：{{ rescheduleForm.orderNo }}<br />当前时段：{{ rescheduleForm.cur }}。目标时段需已开放且有剩余名额。</p>
      <el-form label-width="90px">
        <el-form-item label="新日期">
          <el-date-picker v-model="rescheduleForm.date" type="date" value-format="YYYY-MM-DD" style="width: 100%" />
        </el-form-item>
        <el-form-item label="新时段">
          <el-select v-model="rescheduleForm.slot" style="width: 100%">
            <el-option v-for="s in RESLOT_OPTIONS" :key="s" :label="s" :value="s" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="rescheduleVisible = false">取消</el-button>
        <el-button type="primary" :loading="rescheduling" @click="doReschedule">确认改期</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.toolbar { background: #fff; padding: 16px; border-radius: 8px; margin-bottom: 16px; }
.pager { margin-top: 16px; justify-content: flex-end; }
.sec { margin: 24px 0 12px; font-size: 15px; }
.muted { color: #9aa3ad; }
.dialog-hint { margin: 0 0 16px; color: #6b7280; font-size: 13px; }
.form-hint { color: #9aa3ad; font-size: 12px; line-height: 1.4; }
.op-tag { margin-left: 6px; }
.records { display: flex; flex-wrap: wrap; gap: 10px; }
.record-card {
  width: 130px; cursor: pointer; border: 1px solid #eef0f2; border-radius: 8px;
  overflow: hidden; background: #fff;
}
.record-card:hover { border-color: #0f7f78; }
.record-thumb { width: 130px; height: 90px; object-fit: cover; display: block; }
.record-doc {
  display: flex; align-items: center; justify-content: center;
  background: #f3f6f6; color: #0c4a47; font-size: 13px; font-weight: 600;
}
.record-meta { padding: 6px 8px; font-size: 12px; color: #374151; }
</style>
