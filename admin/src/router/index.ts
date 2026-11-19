import { createRouter, createWebHashHistory, type RouteRecordRaw } from 'vue-router'
import { getToken } from '@/api'

const routes: RouteRecordRaw[] = [
  { path: '/login', name: 'login', component: () => import('@/views/login/index.vue') },
  {
    path: '/',
    component: () => import('@/layout/index.vue'),
    redirect: '/dashboard',
    children: [
      { path: 'dashboard', name: 'dashboard', component: () => import('@/views/dashboard/index.vue'), meta: { title: '数据看板' } },
      { path: 'orders', name: 'orders', component: () => import('@/views/orders/index.vue'), meta: { title: '订单管理' } },
      { path: 'finance', name: 'finance', component: () => import('@/views/finance/index.vue'), meta: { title: '财务管理' } },
      { path: 'users', name: 'users', component: () => import('@/views/users/index.vue'), meta: { title: '用户管理' } },
      { path: 'redemptions', name: 'redemptions', component: () => import('@/views/redemptions/index.vue'), meta: { title: '月卡核销' } },
      { path: 'areas', name: 'areas', component: () => import('@/views/areas/index.vue'), meta: { title: '服务范围' } },
      { path: 'slots', name: 'slots', component: () => import('@/views/slots/index.vue'), meta: { title: '时段库存' } },
      { path: 'skus', name: 'skus', component: () => import('@/views/skus/index.vue'), meta: { title: '服务与价格' } },
    ],
  },
]

export const router = createRouter({
  history: createWebHashHistory(),
  routes,
})

router.beforeEach((to) => {
  if (to.path !== '/login' && !getToken()) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
  if (to.path === '/login' && getToken()) {
    return { path: '/' }
  }
  return true
})
