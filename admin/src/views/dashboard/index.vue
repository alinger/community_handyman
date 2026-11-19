<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
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

// 趋势柱状：按组内最大值归一，最高 120px
const maxOrders = computed(() => Math.max(1, ...(stats.value?.recent_days ?? []).map((d) => d.orders)))
const maxGmv = computed(() => Math.max(1, ...(stats.value?.recent_days ?? []).map((d) => d.gmv)))
const barH = (v: number, max: number) => `${Math.max(2, Math.round((v / max) * 120))}px`

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
      <el-col :span="24">
        <el-card>
          <template #header>
            <div class="card-head">
              <span>近 7 日订单与流水趋势</span>
              <span class="trend-legend">
                <i class="lg lg-order" /> 订单数
                <i class="lg lg-gmv" /> 流水
              </span>
            </div>
          </template>
          <div v-if="stats?.recent_days?.length" class="trend">
            <div v-for="d in stats.recent_days" :key="d.date" class="trend-col">
              <div class="bars">
                <div
                  class="bar bar-order" :style="{ height: barH(d.orders, maxOrders) }"
                  :title="`${d.date} 订单 ${d.orders} 单`"
                />
                <div
                  class="bar bar-gmv" :style="{ height: barH(d.gmv, maxGmv) }"
                  :title="`${d.date} 流水 ${yuan(d.gmv)}`"
                />
              </div>
              <div class="trend-num">{{ d.orders }}</div>
              <div class="trend-gmv">{{ yuan(d.gmv) }}</div>
              <div class="trend-date">{{ d.date.slice(5) }}</div>
            </div>
          </div>
          <el-empty v-else description="暂无数据" :image-size="60" />
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
.trend { display: flex; justify-content: space-around; align-items: flex-end; min-height: 190px; padding-top: 8px; }
.trend-col { text-align: center; width: 90px; }
.bars { display: flex; gap: 6px; align-items: flex-end; justify-content: center; height: 122px; }
.bar { width: 26px; border-radius: 4px 4px 0 0; }
.bar-order { background: #0f7f78; }
.bar-gmv { background: #e6b26e; }
.trend-num { font-weight: 700; margin-top: 6px; }
.trend-gmv { color: #b88230; font-size: 12px; }
.trend-date { color: #9aa3ad; font-size: 12px; margin-top: 2px; }
.trend-legend { font-size: 12px; color: #6b7280; display: flex; align-items: center; gap: 6px; }
.lg { display: inline-block; width: 10px; height: 10px; border-radius: 2px; margin-right: 2px; }
.lg-order { background: #0f7f78; }
.lg-gmv { background: #e6b26e; }
</style>
