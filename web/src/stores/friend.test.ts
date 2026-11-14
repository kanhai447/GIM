// @vitest-environment happy-dom

import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { PublicApiError } from '@/api/client'
import type { FriendRequest, FriendUser } from '@/types/friend'

const harness = vi.hoisted(() => ({
  search: vi.fn(), friends: vi.fn(), requests: vi.fn(), sendRequest: vi.fn(),
  respond: vi.fn(), updateRemark: vi.fn(), remove: vi.fn(), info: vi.fn(),
}))

vi.mock('@/api/friend', () => ({
  friendApi: {
    search: harness.search, friends: harness.friends, requests: harness.requests, sendRequest: harness.sendRequest,
    respond: harness.respond, updateRemark: harness.updateRemark, remove: harness.remove, info: harness.info,
  },
}))

import { useFriendStore } from '@/stores/friend'

function friend(userID: number, isOnline = false, remark = ''): FriendUser {
  return { userID, account: `user${userID}`, nickname: `User ${userID}`, avatar: '', abstract: '', remark, isOnline, createdAt: '2026-11-11T00:00:00Z' }
}

function request(id: number): FriendRequest {
  return {
    id, requester: { userID: 9, nickname: 'Nine', avatar: '' }, receiverID: 1, status: 0, message: 'hi',
    createdAt: '2026-11-11T00:00:00Z', updatedAt: '2026-11-11T00:00:00Z',
  }
}

describe('friend store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    for (const mock of Object.values(harness)) mock.mockReset()
    harness.friends.mockResolvedValue({ list: [friend(2), friend(3, true)] })
    harness.requests.mockResolvedValue({ list: [request(5)] })
    harness.search.mockResolvedValue({ list: [{ userID: 9, account: 'nine', nickname: 'Nine', avatar: '', abstract: '', isFriend: false }] })
    harness.sendRequest.mockResolvedValue(request(6))
    harness.respond.mockResolvedValue({ ...request(5), status: 1 })
    harness.updateRemark.mockResolvedValue(friend(2, false, 'buddy'))
    harness.remove.mockResolvedValue({ deleted: true })
  })

  it('loads friends with online contacts first and counts them', async () => {
    const store = useFriendStore()
    await store.loadFriends()
    expect(store.friends.map((item) => item.userID)).toEqual([3, 2])
    expect(store.onlineCount).toBe(1)
    expect(store.friendIDs.has(2)).toBe(true)
  })

  it('loads pending requests and accepts one', async () => {
    const store = useFriendStore()
    await store.loadRequests()
    expect(store.pendingRequestCount).toBe(1)
    expect(await store.respond(5, 1)).toBe(true)
    expect(store.requests).toHaveLength(0)
    expect(harness.respond).toHaveBeenCalledWith(5, 1)
    expect(harness.friends).toHaveBeenCalled()
  })

  it('trims search keywords and rejects blank input locally', async () => {
    const store = useFriendStore()
    await store.search('  nine  ')
    expect(harness.search).toHaveBeenCalledWith('nine')
    expect(store.searchResults).toHaveLength(1)
    await store.search('   ')
    expect(store.lastError).toBe('请输入搜索关键词')
    expect(harness.search).toHaveBeenCalledTimes(1)
  })

  it('tracks outbound requests and maps duplicate-request errors', async () => {
    const store = useFriendStore()
    expect(await store.sendRequest(9)).toBe(true)
    expect(store.sentRequestIDs).toEqual([9])

    harness.sendRequest.mockRejectedValueOnce(new PublicApiError('duplicate', 1116))
    expect(await store.sendRequest(10)).toBe(false)
    expect(store.lastError).toBe('已有待处理的好友申请')
  })

  it('updates a remark in place and removes a friend', async () => {
    const store = useFriendStore()
    await store.loadFriends()
    expect(await store.updateRemark(2, 'buddy')).toBe(true)
    expect(store.friends.find((item) => item.userID === 2)?.remark).toBe('buddy')

    expect(await store.remove(2)).toBe(true)
    expect(store.friends.map((item) => item.userID)).toEqual([3])
  })

  it('applies presence updates for known friends only', async () => {
    const store = useFriendStore()
    await store.loadFriends()
    store.applyPresence(2, true)
    expect(store.friends.find((item) => item.userID === 2)?.isOnline).toBe(true)
    store.applyPresence(3, false)
    expect(store.friends.find((item) => item.userID === 3)?.isOnline).toBe(false)
    store.applyPresence(999, true)
    expect(store.friends).toHaveLength(2)
  })

  it('reports authentication failures through the handler', async () => {
    const store = useFriendStore()
    const handler = vi.fn()
    store.start(handler)
    harness.friends.mockRejectedValueOnce(new PublicApiError('expired', 1204, 401))
    await store.loadFriends()
    expect(handler).toHaveBeenCalled()
    expect(store.lastError).toBe('登录状态已失效，请重新登录')
  })
})
