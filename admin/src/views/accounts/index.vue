<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  listAccounts, createAccount, setAccountStatus, resetAccountPassword,
  type AdminAccount, ApiError,
} from '@/api'
import { useAuthStore } from '@/store/auth'

const auth = useAuthStore()
const loading = ref(false)
const rows = ref<AdminAccount[]>([])

const visible = ref(false)
const saving = ref(false)
const form = reactive({ username: '', password: '', role: 'operator' as 'admin' | 'operator' })
const resetVisible = ref(false)
const resetForm = reactive({ id: 0, username: '', password: '' })
const resetting = ref(false)

async function load() {
  loading.value = true
  try {
    rows.value = await listAccounts()
  } finally {
    loading.value = false
  }
}
onMounted(load)

function openAdd() {
  Object.assign(form, { username: '', password: '', role: 'operator' })
  visible.value = true
}
async function save() {
  if (!form.username.trim() || form.password.length < 6) {
    ElMessage.warning('用户名不能为空，密码至少 6 位')
    return
  }
  saving.value = true
  try {
    await createAccount(form.username.trim(), form.password, form.role)
    ElMessage.success('账号已创建')
    visible.value = false
    load()
  } catch (e) {
    if (e instanceof ApiError) ElMessage.error(e.message)
  } finally {
    saving.value = false
  }
}

async function toggle(row: AdminAccount) {
  const next = row.status === 1 ? 0 : 1
  const tip = next === 0
    ? `确认停用「${row.username}」？停用后该账号无法登录后台。`
    : `确认启用「${row.username}」？`
  try {
    await ElMessageBox.confirm(tip, next === 0 ? '停用账号' : '启用账号', { type: 'warning' })
  } catch {
    return
  }
  try {
    await setAccountStatus(row.id, next as 0 | 1)
    ElMessage.success(next === 0 ? '已停用' : '已启用')
    load()
  } catch (e) {
    if (e instanceof ApiError) ElMessage.error(e.message)
  }
}

function openReset(row: AdminAccount) {
  resetForm.id = row.id
  resetForm.username = row.username
  resetForm.password = ''
  resetVisible.value = true
}
async function doReset() {
  if (resetForm.password.length < 6) {
    ElMessage.warning('新密码至少 6 位')
    return
  }
  resetting.value = true
  try {
    await resetAccountPassword(resetForm.id, resetForm.password)
    ElMessage.success(`已重置「${resetForm.username}」的密码`)
    resetVisible.value = false
  } catch (e) {
    if (e instanceof ApiError) ElMessage.error(e.message)
  } finally {
    resetting.value = false
  }
}

const fmt = (s: string) => s.replace('T', ' ').slice(0, 19)
</script>

<template>
  <div>
    <div class="toolbar">
      <span class="title">后台账号</span>
      <el-button type="primary" @click="openAdd">＋ 新建账号</el-button>
    </div>
    <div class="note">operator 角色可做日常运营（派单/核销/改价），admin 角色额外可退款、取消改期与管理账号。密码 bcrypt 存储，不可查看只能重置。</div>

    <el-table v-loading="loading" :data="rows" border stripe>
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="username" label="用户名" min-width="140" />
      <el-table-column label="角色" width="120">
        <template #default="{ row }">
          <el-tag :type="row.role === 'admin' ? 'danger' : 'primary'" size="small">
            {{ row.role === 'admin' ? '超管' : '运营' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="row.status === 1 ? 'success' : 'info'" size="small">
            {{ row.status === 1 ? '启用' : '停用' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="创建时间" width="170">
        <template #default="{ row }">{{ fmt(row.created_at) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="180" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openReset(row)">重置密码</el-button>
          <el-button
            link :type="row.status === 1 ? 'danger' : 'success'"
            :disabled="row.id === auth.id" @click="toggle(row)"
          >
            {{ row.status === 1 ? '停用' : '启用' }}
          </el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="visible" title="新建后台账号" width="440px">
      <el-form label-width="90px">
        <el-form-item label="用户名"><el-input v-model="form.username" placeholder="登录用户名" /></el-form-item>
        <el-form-item label="密码"><el-input v-model="form.password" show-password placeholder="至少 6 位" /></el-form-item>
        <el-form-item label="角色">
          <el-radio-group v-model="form.role">
            <el-radio value="operator">运营</el-radio>
            <el-radio value="admin">超管</el-radio>
          </el-radio-group>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="visible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">创建</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="resetVisible" title="重置密码" width="420px">
      <p class="dialog-hint">为「{{ resetForm.username }}」设置新密码，重置后请通知该账号重新登录。</p>
      <el-input v-model="resetForm.password" show-password placeholder="新密码，至少 6 位" />
      <template #footer>
        <el-button @click="resetVisible = false">取消</el-button>
        <el-button type="primary" :loading="resetting" @click="doReset">确认重置</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.toolbar { display: flex; align-items: center; justify-content: space-between; background: #fff; padding: 16px; border-radius: 8px; margin-bottom: 12px; }
.title { font-size: 16px; font-weight: 600; }
.note { color: #6b7280; font-size: 13px; margin-bottom: 12px; line-height: 1.6; }
.dialog-hint { margin: 0 0 16px; color: #6b7280; font-size: 13px; }
</style>
