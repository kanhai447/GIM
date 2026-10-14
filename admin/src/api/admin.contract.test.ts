import type { AxiosAdapter, InternalAxiosRequestConfig } from 'axios'
import { beforeEach, describe, expect, it } from 'vitest'

import { adminApi } from '@/api/admin'
import { apiClient } from '@/api/client'

describe('Admin management API contracts', () => {
  const requests: InternalAxiosRequestConfig[] = []
  const adapter: AxiosAdapter = async (config) => {
    requests.push(config)
    return { data: { code: 0, msg: '成功', data: { list: [], nextBeforeId: 0, hasMore: false } }, status: 200, statusText: 'OK', headers: {}, config }
  }
  beforeEach(() => { requests.length = 0; apiClient.defaults.adapter = adapter })

  it('uses only Gateway-routed real service endpoints', async () => {
    await adminApi.overview(); await adminApi.users({ limit: 50 }); await adminApi.chats({ userId: 7 }); await adminApi.groups(); await adminApi.files(); await adminApi.logs()
    expect(requests.map((request) => request.url)).toEqual([
      '/api/settings/admin/overview', '/api/user/users', '/api/chat/admin/history', '/api/group/groups', '/api/file/files', '/api/logs/logs',
    ])
    expect(requests.some((request) => request.url?.startsWith('/api/data/'))).toBe(false)
  })

  it('uses the formal user status endpoints for disable and enable', async () => {
    await adminApi.disableUser(7)
    await adminApi.enableUser(7)
    expect(requests.map(({ method, url, params }) => ({ method, url, params }))).toEqual([
      { method: 'delete', url: '/api/user/users', params: { userId: 7 } },
      { method: 'put', url: '/api/user/users', params: { userId: 7 } },
    ])
  })
})
