import { defineStore } from 'pinia'
import { getToken, setToken, clearToken, login as apiLogin, type AdminUser } from '@/api'

export const useAuthStore = defineStore('auth', {
  state: () => ({
    token: getToken(),
    id: Number(localStorage.getItem('ch_admin_id')) || 0,
    username: localStorage.getItem('ch_admin_username') || '',
    role: localStorage.getItem('ch_admin_role') || '',
  }),
  getters: {
    isLogged: (s) => !!s.token,
    isAdmin: (s) => s.role === 'admin',
  },
  actions: {
    async login(username: string, password: string) {
      const res = await apiLogin(username, password)
      this.token = res.token
      this.id = res.admin.id
      this.username = res.admin.username
      this.role = res.admin.role
      setToken(res.token)
      localStorage.setItem('ch_admin_id', String(res.admin.id))
      localStorage.setItem('ch_admin_username', res.admin.username)
      localStorage.setItem('ch_admin_role', res.admin.role)
      return res.admin as AdminUser
    },
    logout() {
      this.token = ''
      this.id = 0
      this.username = ''
      this.role = ''
      clearToken()
      localStorage.removeItem('ch_admin_id')
      localStorage.removeItem('ch_admin_username')
      localStorage.removeItem('ch_admin_role')
    },
  },
})
