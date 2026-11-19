<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessageBox } from 'element-plus'
import {
  Odometer, Location, Calendar, PriceTag, SwitchButton, Avatar,
} from '@element-plus/icons-vue'
import { useAuthStore } from '@/store/auth'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()

const menus = [
  { path: '/orders', title: '订单管理', icon: Odometer },
  { path: '/areas', title: '服务范围', icon: Location },
  { path: '/slots', title: '时段库存', icon: Calendar },
  { path: '/skus', title: '服务与价格', icon: PriceTag },
]
const active = computed(() => route.path)

const pageTitle = computed(() => (route.meta.title as string) || '运营后台')

async function onLogout() {
  try {
    await ElMessageBox.confirm('确认退出登录？', '提示', { type: 'warning' })
    auth.logout()
    router.push('/login')
  } catch {
    // 取消
  }
}
</script>

<template>
  <el-container class="layout">
    <el-aside width="220px" class="aside">
      <div class="logo">
        <span class="logo-mark">助</span>
        <span>社区好帮手<small>运营后台</small></span>
      </div>
      <el-menu
        :default-active="active"
        router
        background-color="#0c4a47"
        text-color="#cfe4e1"
        active-text-color="#ffffff"
      >
        <el-menu-item v-for="m in menus" :key="m.path" :index="m.path">
          <el-icon><component :is="m.icon" /></el-icon>
          <span>{{ m.title }}</span>
        </el-menu-item>
      </el-menu>
    </el-aside>

    <el-container>
      <el-header class="header">
        <div class="crumb">运营后台 / <b>{{ pageTitle }}</b></div>
        <div class="user">
          <el-icon><Avatar /></el-icon>
          <span>{{ auth.username || '管理员' }}</span>
          <el-tag v-if="auth.isAdmin" size="small" type="success">超管</el-tag>
          <el-button link :icon="SwitchButton" @click="onLogout">退出</el-button>
        </div>
      </el-header>
      <el-main class="main">
        <router-view />
      </el-main>
    </el-container>
  </el-container>
</template>

<style scoped>
.layout { height: 100%; }
.aside { background: #0c4a47; }
.logo {
  height: 60px;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 0 18px;
  color: #fff;
  font-size: 17px;
  font-weight: 600;
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
}
.logo-mark {
  width: 32px;
  height: 32px;
  border-radius: 8px;
  background: #87c9bd;
  color: #0c4a47;
  display: flex;
  align-items: center;
  justify-content: center;
}
.logo small { display: block; font-size: 11px; font-weight: 400; opacity: 0.7; }
.el-menu { border-right: none; }
.header {
  background: #fff;
  border-bottom: 1px solid #eef0f2;
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.crumb { font-size: 15px; color: #6b7280; }
.crumb b { color: #1f2328; }
.user { display: flex; align-items: center; gap: 10px; font-size: 14px; }
.main { padding: 20px; }
</style>
