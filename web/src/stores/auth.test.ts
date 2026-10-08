import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { login, register, profile, logout } = vi.hoisted(() => ({
  login: vi.fn(),
  register: vi.fn(),
  profile: vi.fn(),
  logout: vi.fn(),
}))

vi.mock('@/api/auth', () => ({ authApi: { login, register, profile, logout } }))

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

const member = { userID: 9, account: 'gim-user', nickname: 'GIM User', avatar: '', role: 2 as const, status: 1 as const }

describe('Web auth store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.stubGlobal('localStorage', new MemoryStorage())
  })

  it('persists a successful login and clears it after server logout', async () => {
    login.mockResolvedValue({ token: 'test-only-token', user: member })
    logout.mockResolvedValue({ loggedOut: true })
    const store = useAuthStore()

    await store.login({ account: member.account, password: 'test-input-only' })
    expect(store.authenticated).toBe(true)
    expect(store.user).toEqual(member)
    expect(localStorage.getItem('gim.web.token')).toBe('test-only-token')

    await store.logout()
    expect(store.authenticated).toBe(false)
    expect(localStorage.getItem('gim.web.token')).toBeNull()
  })

  it('validates a restored session against the protected profile API', async () => {
    localStorage.setItem('gim.web.token', 'stored-test-token')
    localStorage.setItem('gim.web.user', JSON.stringify(member))
    profile.mockResolvedValue({ ...member, nickname: 'Fresh User' })
    const store = useAuthStore()

    await store.restoreSession()
    expect(profile).toHaveBeenCalledOnce()
    expect(store.user?.nickname).toBe('Fresh User')
  })
})
