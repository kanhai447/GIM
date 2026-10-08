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

const user = { userID: 9, account: 'contract-user', nickname: 'Contract', avatar: '', role: 2 as const, status: 1 as const }

describe('Web Auth API contract', () => {
  const requests: InternalAxiosRequestConfig[] = []
  const adapter: AxiosAdapter = async (config) => {
    requests.push(config)
    const data = config.url === '/api/auth/login'
      ? { token: 'opaque-test-value', user }
      : config.url === '/api/auth/register'
        ? user
        : config.url === '/api/user/user_info'
          ? user
          : { loggedOut: true }
    return { data: { code: 0, msg: '成功', data }, status: 200, statusText: 'OK', headers: {}, config }
  }

  beforeEach(() => {
    requests.length = 0
    vi.stubGlobal('localStorage', new MemoryStorage())
    apiClient.defaults.adapter = adapter
  })

  it('uses the current Gateway login and register contracts', async () => {
    await authApi.login({ account: user.account, password: 'test-input-only' })
    await authApi.register({ account: user.account, nickname: user.nickname, pwd: 'test-input-only', rePwd: 'test-input-only' })
    expect(requests.map(({ method, url }) => [method, url])).toEqual([
      ['post', '/api/auth/login'],
      ['post', '/api/auth/register'],
    ])
  })

  it('sends the token header to profile and the correct logout URL', async () => {
    saveSession('opaque-test-value', user)
    await authApi.profile()
    await authApi.logout()
    expect(requests.map(({ url }) => url)).toEqual(['/api/user/user_info', '/api/auth/logout'])
    expect(requests.every((request) => request.headers.get('token') === 'opaque-test-value')).toBe(true)
  })
})
