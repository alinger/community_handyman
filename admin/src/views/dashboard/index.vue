<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { getStats, type AdminStats } from '@/api'

const router = useRouter()
const stats = ref<AdminStats | null>(null)
const loading = ref(false)

const STATUS_LABELS: Record<string, string> = {
  pending_payment: '待支付',
  pending_accept: '待接单',
  accepted: '已受理',
  on_the_way: '待上门',
  in_service: '服务中',
  pending_confirm: '待完成',
  completed: '已完成',
  canceled: '已取消',
  rejected: '已拒单',
}
const SERVICE_LABELS: Record<string, string> = {
  garbage: '代倒垃圾',
  dogwalk: '遛宠',
  accompany: '陪诊',
}
const yuan = (fen: number) => `¥${(fen / 100).toFixed(2)}`

async function load() {
  loading.value = true
  try {
    stats.value = await getStats()
  } finally {
    loading.value = false
  }
}
onMounted(load)
</script>

<template>
  <div v-loading="loading">
    <el-row :gutter="16">
      <el-col :span="6">
        <el-card class="stat">
          <div class="stat-label">今日订单</div>
          <div class="stat-num">{{ stats?.today_orders ?? '-' }}</div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card class="stat">
          <div class="stat-label">今日流水</div>
          <div class="stat-num">{{ stats ? yuan(stats.today_gmv) : '-' }}</div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card class="stat">
          <div class="stat-label">今日退款</div>
          <div class="stat-num refund">{{ stats ? yuan(stats.today_refunded) : '-' }}</div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card class="stat">
          <div class="stat-label">待接单 / 未派单</div>
          <div class="stat-num warn">
            {{ stats?.pending_accept ?? '-' }} / {{ stats?.unassigned ?? '-' }}
          </div>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="16" class="mt">
      <el-col :span="16">
        <el-card>
          <template #header>
            <div class="card-head">
              <span>订单状态分布（全部）</span>
              <el-button link type="primary" @click="router.push('/orders')">去订单管理</el-button>
            </div>
          </template>
          <div class="dist">
            <div
              v-for="s in stats?.status_dist ?? []" :key="s.status"
              class="dist-row"
            >
              <span class="dist-label">{{ STATUS_LABELS[s.status] || s.status }}</span>
              <el-progress
                :percentage="stats && (stats.today_orders || 1) ? Math.round(s.count / Math.max(...stats.status_dist.map(x => x.count)) * 100) : 0"
                :stroke-width="14" class="dist-bar"
              />
              <span class="dist-count">{{ s.count }}</span>
            </div>
            <el-empty v-if="stats && !stats.status_dist.length" description="暂无订单" :image-size="60" />
          </div>
        </el-card>
      </el-col>
      <el-col :span="8">
        <el-card>
          <template #header>今日服务构成</template>
          <div class="svc">
            <div v-for="s in stats?.service_dist ?? []" :key="s.service_type" class="svc-row">
              <el-tag>{{ SERVICE_LABELS[s.service_type] || s.service_type }}</el-tag>
              <b>{{ s.count }}</b>
            </div>
            <el-empty v-if="stats && !stats.service_dist.length" description="今日暂无新订单" :image-size="60" />
          </div>
        </el-card>
        <el-card class="mt">
          <template #header>账号规模</template>
          <div class="svc-row"><span>住户</span><b>{{ stats?.total_users ?? '-' }}</b></div>
          <div class="svc-row"><span>服务方</span><b>{{ stats?.providers ?? '-' }}</b></div>
          <div class="svc-row"><span>履约中</span><b>{{ stats?.in_service ?? '-' }}</b></div>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<style scoped>
.stat { text-align: left; }
.stat-label { color: #6b7280; font-size: 13px; }
.stat-num { font-size: 26px; font-weight: 700; margin-top: 6px; color: #0c4a47; }
.stat-num.refund { color: #c45656; }
.stat-num.warn { color: #b88230; }
.mt { margin-top: 16px; }
.card-head { display: flex; justify-content: space-between; align-items: center; }
.dist-row { display: flex; align-items: center; gap: 12px; margin-bottom: 10px; }
.dist-label { width: 70px; color: #374151; font-size: 13px; flex-shrink: 0; }
.dist-bar { flex: 1; }
.dist-count { width: 32px; text-align: right; font-weight: 600; }
.svc-row { display: flex; justify-content: space-between; align-items: center; margin-bottom: 10px; }
</style>
