<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listUsers, setUserRole, setUserStatus, type AdminUserRow } from '@/api'

const loading = ref(false)
const list = ref<AdminUserRow[]>([])
const total = ref(0)
const query = reactive({ role: '', keyword: '', page: 1, size: 10 })

const ROLE_LABEL: Record<string, string> = { user: '住户', provider: '服务方' }

async function load() {
  loading.value = true
  try {
    const data = await listUsers({ ...query })
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

async function onRoleChange(row: AdminUserRow, role: 'user' | 'provider') {
  const to = ROLE_LABEL[role]
  try {
    await ElMessageBox.confirm(
      `确认把「${row.nickname || row.openid}」切换为${to}？${role === 'provider' ? '切换后可被派单。' : '切换后将不再出现在派单下拉。'}`,
      '切换角色', { type: 'warning' },
    )
  } catch {
    return
  }
  await setUserRole(row.id, role)
  ElMessage.success('角色已切换')
  load()
}

async function onToggleBan(row: AdminUserRow) {
  const ban = row.status === 1
  try {
    await ElMessageBox.confirm(
      ban ? `确认封禁「${row.nickname || row.openid}」？封禁后无法登录小程序。` : `确认解除「${row.nickname || row.openid}」的封禁？`,
      ban ? '封禁账号' : '解除封禁', { type: 'warning' },
    )
  } catch {
    return
  }
  await setUserStatus(row.id, ban ? 0 : 1)
  ElMessage.success(ban ? '已封禁' : '已解封')
  load()
}

const fmt = (s: string) => s.replace('T', ' ').slice(0, 19)

onMounted(load)
</script>

<template>
  <div>
    <el-form inline class="filter">
      <el-form-item label="角色">
        <el-select v-model="query.role" clearable placeholder="全部" style="width: 140px" @change="search">
          <el-option label="住户" value="user" />
          <el-option label="服务方" value="provider" />
        </el-select>
      </el-form-item>
      <el-form-item label="关键词">
        <el-input v-model="query.keyword" placeholder="昵称 / openid / 手机号" clearable style="width: 220px" @keyup.enter="search" />
      </el-form-item>
      <el-form-item>
        <el-button type="primary" @click="search">查询</el-button>
      </el-form-item>
    </el-form>

    <el-table v-loading="loading" :data="list" stripe>
      <el-table-column prop="id" label="ID" width="60" />
      <el-table-column label="昵称" min-width="120">
        <template #default="{ row }">{{ row.nickname || '-' }}</template>
      </el-table-column>
      <el-table-column prop="openid" label="openid" min-width="140" show-overflow-tooltip />
      <el-table-column label="手机号" width="120">
        <template #default="{ row }">{{ row.phone || '-' }}</template>
      </el-table-column>
      <el-table-column label="角色" width="150">
        <template #default="{ row }">
          <el-select
            :model-value="row.role" size="small" style="width: 110px"
            @change="(r: string) => onRoleChange(row, r as 'user' | 'provider')"
          >
            <el-option label="住户" value="user" />
            <el-option label="服务方" value="provider" />
          </el-select>
        </template>
      </el-table-column>
      <el-table-column label="订单数" width="80" align="center">
        <template #default="{ row }">{{ row.order_count }}</template>
      </el-table-column>
      <el-table-column label="状态" width="90">
        <template #default="{ row }">
          <el-tag :type="row.status === 1 ? 'success' : 'danger'" size="small">
            {{ row.status === 1 ? '正常' : '已封禁' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="注册时间" width="170">
        <template #default="{ row }">{{ fmt(row.created_at) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="100" fixed="right">
        <template #default="{ row }">
          <el-button v-if="row.status === 1" link type="danger" size="small" @click="onToggleBan(row)">封禁</el-button>
          <el-button v-else link type="success" size="small" @click="onToggleBan(row)">解封</el-button>
        </template>
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
.pager { margin-top: 14px; justify-content: flex-end; }
</style>
