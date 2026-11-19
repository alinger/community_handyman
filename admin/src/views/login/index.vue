<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, type FormInstance } from 'element-plus'
import { User, Lock } from '@element-plus/icons-vue'
import { useAuthStore } from '@/store/auth'

const auth = useAuthStore()
const router = useRouter()
const route = useRoute()
const formRef = ref<FormInstance>()
const form = ref({ username: 'admin', password: '' })
const loading = ref(false)

const rules = {
  username: [{ required: true, message: '请输入账号', trigger: 'blur' }],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }],
}

async function onSubmit() {
  if (!formRef.value) return
  const ok = await formRef.value.validate().catch(() => false)
  if (!ok) return
  loading.value = true
  try {
    await auth.login(form.value.username.trim(), form.value.password)
    ElMessage.success('登录成功')
    const redirect = route.query.redirect as string | undefined
    router.push(redirect && redirect.startsWith('/') ? redirect : '/')
  } catch {
    // 错误提示已在请求拦截器统一处理
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="login">
    <div class="card">
      <div class="brand">
        <span class="mark">助</span>
        <h1>社区好帮手</h1>
        <p>运营后台</p>
      </div>
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top" @keyup.enter="onSubmit">
        <el-form-item label="账号" prop="username">
          <el-input v-model="form.username" :prefix-icon="User" placeholder="管理员账号" size="large" />
        </el-form-item>
        <el-form-item label="密码" prop="password">
          <el-input v-model="form.password" :prefix-icon="Lock" type="password" show-password placeholder="密码" size="large" />
        </el-form-item>
        <el-button type="primary" size="large" class="submit" :loading="loading" @click="onSubmit">登 录</el-button>
      </el-form>
      <div class="tip">账号由运营在服务端创建，密码不写入版本库</div>
    </div>
  </div>
</template>

<style scoped>
.login {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #e4f3f0 0%, #f5f7fa 60%);
}
.card {
  width: 400px;
  background: #fff;
  border-radius: 16px;
  padding: 40px 36px 28px;
  box-shadow: 0 12px 40px rgba(15, 127, 120, 0.12);
}
.brand { text-align: center; margin-bottom: 28px; }
.mark {
  display: inline-flex;
  width: 52px;
  height: 52px;
  border-radius: 14px;
  background: var(--brand);
  color: #fff;
  font-size: 26px;
  align-items: center;
  justify-content: center;
}
.brand h1 { font-size: 22px; margin: 12px 0 4px; }
.brand p { margin: 0; color: #9aa3ad; font-size: 13px; }
.submit { width: 100%; margin-top: 8px; }
.tip { text-align: center; color: #9aa3ad; font-size: 12px; margin-top: 20px; }
</style>
