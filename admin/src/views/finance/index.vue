<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import {
  listPayments, listRefunds,
  type AdminPayment, type AdminRefund,
} from '@/api'

const PAY_STATUS: Record<string, { label: string; type: 'success' | 'info' | 'warning' }> = {
  success: { label: '支付成功', type: 'success' },
  closed: { label: '已关闭', type: 'info' },
  refunded: { label: '已退款', type: 'warning' },
}
const REFUND_STATUS: Record<string, { label: string; type: 'success' | 'info' | 'warning' | 'danger' }> = {
  pending: { label: '退款中', type: 'warning' },
  success: { label: '退款成功', type: 'success' },
  failed: { label: '退款失败', type: 'danger' },
}
const yuan = (fen: number) => `¥${(fen / 100).toFixed(2)}`
const fmt = (s: string | null) => (s ? s.replace('T', ' ').slice(0, 19) : '-')

// ---- 支付流水 ----
const tab = ref('payments')
const payLoading = ref(false)
const payList = ref<AdminPayment[]>([])
const payTotal = ref(0)
const payQuery = reactive({ order_no: '', status: '', page: 1, size: 10 })

async function loadPayments() {
  payLoading.value = true
  try {
    const data = await listPayments({ ...payQuery })
    payList.value = data.list
    payTotal.value = data.total
  } finally {
    payLoading.value = false
  }
}
function searchPayments() {
  payQuery.page = 1
  loadPayments()
}

// ---- 退款单 ----
const refundLoading = ref(false)
const refundList = ref<AdminRefund[]>([])
const refundTotal = ref(0)
const refundQuery = reactive({ order_no: '', status: '', page: 1, size: 10 })

async function loadRefunds() {
  refundLoading.value = true
  try {
    const data = await listRefunds({ ...refundQuery })
    refundList.value = data.list
    refundTotal.value = data.total
  } finally {
    refundLoading.value = false
  }
}
function searchRefunds() {
  refundQuery.page = 1
  loadRefunds()
}
function copyOrderNo(no: string) {
  navigator.clipboard?.writeText(no)
  ElMessage.success('订单号已复制')
}

onMounted(() => {
  loadPayments()
  loadRefunds()
})
</script>

<template>
  <el-tabs v-model="tab">
    <el-tab-pane label="支付流水" name="payments">
      <el-form inline class="filter">
        <el-form-item label="订单号">
          <el-input v-model="payQuery.order_no" placeholder="模糊搜索" clearable style="width: 220px" @keyup.enter="searchPayments" />
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="payQuery.status" clearable placeholder="全部" style="width: 140px" @change="searchPayments">
            <el-option v-for="(v, k) in PAY_STATUS" :key="k" :label="v.label" :value="k" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="searchPayments">查询</el-button>
        </el-form-item>
      </el-form>
      <el-table v-loading="payLoading" :data="payList" stripe>
        <el-table-column label="订单号" min-width="200">
          <template #default="{ row }">
            <el-link type="primary" @click="copyOrderNo(row.order_no)">{{ row.order_no }}</el-link>
          </template>
        </el-table-column>
        <el-table-column prop="transaction_id" label="交易号" min-width="180" show-overflow-tooltip />
        <el-table-column label="金额" width="110">
          <template #default="{ row }"><b>{{ yuan(row.amount) }}</b></template>
        </el-table-column>
        <el-table-column label="状态" width="110">
          <template #default="{ row }">
            <el-tag :type="PAY_STATUS[row.status]?.type || 'info'">{{ PAY_STATUS[row.status]?.label || row.status }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="支付时间" width="170">
          <template #default="{ row }">{{ fmt(row.paid_at) }}</template>
        </el-table-column>
        <el-table-column label="创建时间" width="170">
          <template #default="{ row }">{{ fmt(row.created_at) }}</template>
        </el-table-column>
      </el-table>
      <el-pagination
        v-model:current-page="payQuery.page" v-model:page-size="payQuery.size"
        :total="payTotal" layout="total, prev, pager, next" class="pager"
        @current-change="loadPayments"
      />
    </el-tab-pane>

    <el-tab-pane label="退款单" name="refunds">
      <el-form inline class="filter">
        <el-form-item label="订单号">
          <el-input v-model="refundQuery.order_no" placeholder="模糊搜索" clearable style="width: 220px" @keyup.enter="searchRefunds" />
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="refundQuery.status" clearable placeholder="全部" style="width: 140px" @change="searchRefunds">
            <el-option v-for="(v, k) in REFUND_STATUS" :key="k" :label="v.label" :value="k" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="searchRefunds">查询</el-button>
        </el-form-item>
      </el-form>
      <el-table v-loading="refundLoading" :data="refundList" stripe>
        <el-table-column label="订单号" min-width="200">
          <template #default="{ row }">
            <el-link type="primary" @click="copyOrderNo(row.order_no)">{{ row.order_no }}</el-link>
          </template>
        </el-table-column>
        <el-table-column prop="out_refund_no" label="退款单号" min-width="200" show-overflow-tooltip />
        <el-table-column label="金额" width="110">
          <template #default="{ row }"><b>{{ yuan(row.amount) }}</b></template>
        </el-table-column>
        <el-table-column label="状态" width="110">
          <template #default="{ row }">
            <el-tag :type="REFUND_STATUS[row.status]?.type || 'info'">{{ REFUND_STATUS[row.status]?.label || row.status }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="reason" label="原因" min-width="140" show-overflow-tooltip />
        <el-table-column label="操作人" width="90">
          <template #default="{ row }">#{{ row.operator_id }}</template>
        </el-table-column>
        <el-table-column label="退款时间" width="170">
          <template #default="{ row }">{{ fmt(row.refunded_at) }}</template>
        </el-table-column>
        <el-table-column label="发起时间" width="170">
          <template #default="{ row }">{{ fmt(row.created_at) }}</template>
        </el-table-column>
      </el-table>
      <el-pagination
        v-model:current-page="refundQuery.page" v-model:page-size="refundQuery.size"
        :total="refundTotal" layout="total, prev, pager, next" class="pager"
        @current-change="loadRefunds"
      />
    </el-tab-pane>
  </el-tabs>
</template>

<style scoped>
.filter { margin-bottom: 4px; }
.pager { margin-top: 14px; justify-content: flex-end; }
</style>
