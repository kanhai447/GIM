import { beforeEach, describe, expect, it, vi } from 'vitest'

const harness = vi.hoisted(() => ({ apiRequest: vi.fn() }))

vi.mock('@/api/client', () => ({
  apiRequest: harness.apiRequest,
  PublicApiError: class PublicApiError extends Error {
    constructor(message: string, readonly code?: number, readonly status?: number) { super(message) }
  },
}))

import { friendApi } from '@/api/friend'

describe('friend API', () => {
  beforeEach(() => harness.apiRequest.mockReset())

  it('uses the documented friend routes and parameters', async () => {
    harness.apiRequest.mockResolvedValue({ list: [] })
    await friendApi.search('alice', 10)
    expect(harness.apiRequest).toHaveBeenLastCalledWith({ method: 'GET', url: '/api/user/search', params: { keyword: 'alice', limit: 10 } })

    await friendApi.friends(50)
    expect(harness.apiRequest).toHaveBeenLastCalledWith({ method: 'GET', url: '/api/user/friends', params: { limit: 50 } })

    await friendApi.requests(0, 20)
    expect(harness.apiRequest).toHaveBeenLastCalledWith({ method: 'GET', url: '/api/user/valid', params: { status: 0, limit: 20 } })

    await friendApi.info(8)
    expect(harness.apiRequest).toHaveBeenLastCalledWith({ method: 'GET', url: '/api/user/friend_info', params: { friendID: 8 } })
  })

  it('sends mutations to the friend routes', async () => {
    harness.apiRequest.mockResolvedValue({})
    await friendApi.sendRequest(8, 'hi')
    expect(harness.apiRequest).toHaveBeenLastCalledWith({ method: 'POST', url: '/api/user/friends', data: { friendID: 8, message: 'hi' } })

    await friendApi.respond(3, 1)
    expect(harness.apiRequest).toHaveBeenLastCalledWith({ method: 'PUT', url: '/api/user/valid_status', data: { validID: 3, status: 1 } })

    await friendApi.updateRemark(8, 'buddy')
    expect(harness.apiRequest).toHaveBeenLastCalledWith({ method: 'PUT', url: '/api/user/friends', data: { friendID: 8, remark: 'buddy' } })

    await friendApi.remove(8)
    expect(harness.apiRequest).toHaveBeenLastCalledWith({ method: 'DELETE', url: '/api/user/friends', params: { friendID: 8 } })
  })
})
