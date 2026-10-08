import type { AxiosAdapter, InternalAxiosRequestConfig } from 'axios'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { authApi } from '@/api/auth'
import { apiClient } from '@/api/client'
import { saveSession } from '@/utils/session'

class MemoryStorage implements Storage {
  private readonly values = new Map<string, string>()
  get length(): number { return this.values.size }
  clear(): void { this.values.clear() }
  getItem(key: string): string | null { return this.values.get(key) ?? null }
  key(index: number): string | null { return [...this.values.keys()][index] ?? null }
  removeItem(key: string): void { this.values.delete(key) }
  setItem(key: string, value: string): void { this.values.set(key, value) }
}

const admin = { userID: 1, account: 'contract-admin', nickname: 'Admin', avatar: '', role: 1 as const, status: 1 as const }

describe('Admin Auth API contract', () => {
  const requests: InternalAxiosRequestConfig[] = []
  const adapter: AxiosAdapter = async (config) => {
    requests.push(config)
    const data = config.url === '/api/auth/login' ? { token: 'opaque-admin-test-value', user: admin } : config.url === '/api/user/user_info' ? admin : { loggedOut: true }
    return { data: { code: 0, msg: '成功', data }, status: 200, statusText: 'OK', headers: {}, config }
  }

  beforeEach(() => {
    requests.length = 0
    vi.stubGlobal('localStorage', new MemoryStorage())
    apiClient.defaults.adapter = adapter
  })

  it('reuses Gateway Auth and never calls the legacy logout path', async () => {
    await authApi.login({ account: admin.account, password: 'test-input-only' })
    saveSession('opaque-admin-test-value', admin)
    await authApi.profile()
    await authApi.logout()
    expect(requests.map(({ url }) => url)).toEqual([
      '/api/auth/login',
      '/api/user/user_info',
      '/api/auth/logout',
    ])
    expect(requests.at(-1)?.headers.get('token')).toBe('opaque-admin-test-value')
  })
})
