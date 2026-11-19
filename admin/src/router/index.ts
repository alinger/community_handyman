import { createRouter, createWebHashHistory, type RouteRecordRaw } from 'vue-router'
import { getToken } from '@/api'

const routes: RouteRecordRaw[] = [
  { path: '/login', name: 'login', component: () => import('@/views/login/index.vue') },
  {
    path: '/',
    component: () => import('@/layout/index.vue'),
    redirect: '/orders',
    children: [
      { path: 'orders', name: 'orders', component: () => import('@/views/orders/index.vue'), meta: { title: '订单管理' } },
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
