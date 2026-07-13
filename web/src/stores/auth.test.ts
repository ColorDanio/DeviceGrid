import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

const { login, getMe } = vi.hoisted(() => ({ login: vi.fn(), getMe: vi.fn() }))

vi.mock('@/api/auth', () => ({ login, getMe }))

import { useAuthStore } from './auth'

describe('auth store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    login.mockReset()
    getMe.mockReset()
  })

  it('persists a successful sign-in and clears it on logout', async () => {
    login.mockResolvedValue({ token: 'token-1', username: 'alice', role: 'admin' })
    const store = useAuthStore()

    await store.login({ username: 'alice', password: 'secret' })

    expect(store.isAuthenticated).toBe(true)
    expect(store.isAdmin).toBe(true)
    expect(sessionStorage.getItem('dg_token')).toBe('token-1')

    store.logout()

    expect(store.isAuthenticated).toBe(false)
    expect(sessionStorage.getItem('dg_token')).toBeNull()
  })

  it('logs out when the identity refresh fails', async () => {
    getMe.mockRejectedValue(new Error('expired'))
    sessionStorage.setItem('dg_token', 'expired-token')
    const store = useAuthStore()

    await store.fetchMe()

    expect(store.isAuthenticated).toBe(false)
  })
})
