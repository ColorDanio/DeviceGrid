import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { login as loginApi, getMe, type LoginRequest } from '@/api/auth'

export const useAuthStore = defineStore('auth', () => {
  const token = ref(localStorage.getItem('dg_token') || '')
  const username = ref(localStorage.getItem('dg_username') || '')
  const role = ref(localStorage.getItem('dg_role') || '')
  const sidebarCollapsed = ref(false)

  const isAuthenticated = computed(() => !!token.value)
  const isAdmin = computed(() => role.value === 'admin')

  async function login(req: LoginRequest) {
    const info = await loginApi(req)
    token.value = info.token
    username.value = info.username
    role.value = info.role
    localStorage.setItem('dg_token', info.token)
    localStorage.setItem('dg_username', info.username)
    localStorage.setItem('dg_role', info.role)
  }

  function logout() {
    token.value = ''
    username.value = ''
    role.value = ''
    localStorage.removeItem('dg_token')
    localStorage.removeItem('dg_username')
    localStorage.removeItem('dg_role')
  }

  async function fetchMe() {
    try {
      const me = await getMe()
      username.value = me.username
      role.value = me.role
    } catch {
      logout()
    }
  }

  function toggleSidebar() {
    sidebarCollapsed.value = !sidebarCollapsed.value
  }

  return {
    token,
    username,
    role,
    sidebarCollapsed,
    isAuthenticated,
    isAdmin,
    login,
    logout,
    fetchMe,
    toggleSidebar,
  }
})
