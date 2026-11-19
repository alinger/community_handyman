<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { listRedemptions, type AdminRedemption } from '@/api'

const loading = ref(false)
const list = ref<AdminRedemption[]>([])
const total = ref(0)
const query = reactive({ date: '', page: 1, size: 10 })

async function load() {
  loading.value = true
  try {
    const data = await listRedemptions({ ...query })
    list.value = data.list
    total.value = data.total
  } finally {
    loading.value = false
  }
}
function search() {
  query.page = 1
  load()
}
const fmt = (s: string) => s.replace('T', ' ').slice(0, 19)

onMounted(load)
</script>

<template>
  <div>
    <el-form inline class="filter">
      <el-form-item label="服务日期">
        <el-date-picker
          v-model="query.date" type="date" value-format="YYYY-MM-DD"
          placeholder="全部" clearable style="width: 160px" @change="search"
        />
      </el-form-item>
      <el-form-item>
        <el-button type="primary" @click="search">查询</el-button>
      </el-form-item>
    </el-form>

    <el-alert
      class="hint" type="info" :closable="false"
      title="包月权益不累计、不顺延：每条核销 = 扣一天服务，重复核销即重复扣权益，请谨慎核对日期。"
    />

    <el-table v-loading="loading" :data="list" stripe>
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="order_no" label="订单号" min-width="200" />
      <el-table-column prop="service_date" label="服务日期" width="120" />
      <el-table-column label="核销人" width="90">
        <template #default="{ row }">#{{ row.operator_id }}</template>
      </el-table-column>
      <el-table-column label="备注" min-width="140">
        <template #default="{ row }">{{ row.remark || '-' }}</template>
      </el-table-column>
      <el-table-column label="核销时间" width="170">
        <template #default="{ row }">{{ fmt(row.created_at) }}</template>
      </el-table-column>
    </el-table>
    <el-pagination
      v-model:current-page="query.page" v-model:page-size="query.size"
      :total="total" layout="total, prev, pager, next" class="pager"
      @current-change="load"
    />
  </div>
</template>

<style scoped>
.filter { margin-bottom: 4px; }
.hint { margin-bottom: 12px; }
.pager { margin-top: 14px; justify-content: flex-end; }
</style>
