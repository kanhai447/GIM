import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { login, profile, logout } = vi.hoisted(() => ({
  login: vi.fn(),
  profile: vi.fn(),
  logout: vi.fn(),
}))

vi.mock('@/api/auth', () => ({ authApi: { login, profile, logout } }))

import { useAuthStore } from '@/stores/auth'

class MemoryStorage implements Storage {
  private readonly values = new Map<string, string>()
  get length(): number { return this.values.size }
  clear(): void { this.values.clear() }
  getItem(key: string): string | null { return this.values.get(key) ?? null }
  key(index: number): string | null { return [...this.values.keys()][index] ?? null }
  removeItem(key: string): void { this.values.delete(key) }
  setItem(key: string, value: string): void { this.values.set(key, value) }
}

const admin = { userID: 1, account: 'gim-admin', nickname: 'Admin', avatar: '', role: 1 as const, status: 1 as const }

describe('Admin auth store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.stubGlobal('localStorage', new MemoryStorage())
  })

  it('derives admin UI access from the server login response', async () => {
    login.mockResolvedValue({ token: 'admin-test-token', user: admin })
    const store = useAuthStore()
    await store.login({ account: admin.account, password: 'test-input-only' })
    expect(store.authenticated).toBe(true)
    expect(store.isAdmin).toBe(true)
  })

  it('clears local state even when remote logout fails', async () => {
    login.mockResolvedValue({ token: 'admin-test-token', user: admin })
    logout.mockRejectedValue(new Error('test-only failure'))
    const store = useAuthStore()
    await store.login({ account: admin.account, password: 'test-input-only' })
    await expect(store.logout()).rejects.toThrow('test-only failure')
    expect(store.authenticated).toBe(false)
    expect(localStorage.getItem('gim.admin.token')).toBeNull()
  })
})
