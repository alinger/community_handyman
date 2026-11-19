<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  listOrders, listProviders, assignOrder, refund, redeem, quote,
  type Order, type ProviderOption, type OrderQuery, ApiError,
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
function openDetail(o: Order) {
  detail.value = o
  detailVisible.value = true
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

const title = computed(() => `${auth.username || '运营'} · 订单管理`)
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
      <el-table-column label="操作" width="300" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openDetail(row)">详情</el-button>
          <el-button v-if="row.status === 'pending_accept'" link type="primary" @click="openAssign(row)">派单</el-button>
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
  </div>
</template>

<style scoped>
.toolbar { background: #fff; padding: 16px; border-radius: 8px; margin-bottom: 16px; }
.pager { margin-top: 16px; justify-content: flex-end; }
.sec { margin: 24px 0 12px; font-size: 15px; }
.muted { color: #9aa3ad; }
.dialog-hint { margin: 0 0 16px; color: #6b7280; font-size: 13px; }
.form-hint { color: #9aa3ad; font-size: 12px; line-height: 1.4; }
</style>
