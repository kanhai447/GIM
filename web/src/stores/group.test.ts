import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { PublicApiError } from '@/api/client'
import type { GroupConnectionState, GroupSendEnvelope, GroupServerEnvelope, GroupSession, GroupSummary } from '@/types/group'

interface SocketCallbacks {
  onStateChange: (state: GroupConnectionState) => void
  onEnvelope: (envelope: GroupServerEnvelope) => void
  onAuthenticationFailure: () => void
}

interface FakeSocketInstance {
  options: SocketCallbacks
  connect: ReturnType<typeof vi.fn>
  disconnect: ReturnType<typeof vi.fn>
  failAuthentication: ReturnType<typeof vi.fn>
  send: ReturnType<typeof vi.fn<(envelope: GroupSendEnvelope) => boolean>>
}

const harness = vi.hoisted(() => ({
  sessions: vi.fn(),
  myGroups: vi.fn(),
  info: vi.fn(),
  members: vi.fn(),
  history: vi.fn(),
  markRead: vi.fn(),
  create: vi.fn(),
  setMemberMute: vi.fn(),
  sockets: [] as FakeSocketInstance[],
}))

vi.mock('@/api/group', () => ({
  groupApi: {
    sessions: harness.sessions,
    myGroups: harness.myGroups,
    info: harness.info,
    members: harness.members,
    history: harness.history,
    markRead: harness.markRead,
    create: harness.create,
    setMemberMute: harness.setMemberMute,
  },
}))

vi.mock('@/service/ws/group_socket', () => ({
  GroupSocketClient: class {
    connect = vi.fn()
    disconnect = vi.fn()
    send = vi.fn<(envelope: GroupSendEnvelope) => boolean>(() => true)
    failAuthentication = vi.fn(() => this.options.onAuthenticationFailure())
    constructor(readonly options: SocketCallbacks) { harness.sockets.push(this) }
  },
}))

import { useGroupStore } from '@/stores/group'

const groupInfo: GroupSummary = {
  groupId: 5,
  title: 'Team Five',
  abstract: 'group abstract',
  avatar: '',
  ownerId: 1,
  status: 1,
  role: 1,
  memberCount: 3,
  isSearch: true,
  joinPolicy: 2,
  isProhibition: false,
  memberLimit: 100,
}

const firstMessage = {
  messageId: 10,
  groupID: 5,
  sendUserID: 2,
  clientMsgId: 'remote-10',
  msg: { type: 1 as const, textMsg: { content: 'first' } },
  createdAt: '2026-10-13T00:00:00Z',
}

const session: GroupSession = {
  group: { groupID: 5, name: 'Team Five', avatar: '' },
  lastMessage: firstMessage,
  lastMessageID: 10,
  lastMessageAt: firstMessage.createdAt,
  lastReadMessageID: null,
  unreadCount: 1,
  isTop: false,
}

function latestSocket(): FakeSocketInstance {
  return harness.sockets.at(-1)!
}

async function startStore() {
  const store = useGroupStore()
  store.start(1, 'test-token')
  await vi.waitFor(() => expect(harness.sessions).toHaveBeenCalled())
  await vi.waitFor(() => expect(harness.myGroups).toHaveBeenCalled())
  return store
}

async function openStore() {
  const store = await startStore()
  await store.openGroup(5)
  return store
}

describe('group store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    harness.sockets.length = 0
    harness.sessions.mockReset().mockResolvedValue({ list: [{ ...session }], count: 1 })
    harness.myGroups.mockReset().mockResolvedValue({ list: [{ ...groupInfo }], count: 1 })
    harness.info.mockReset().mockResolvedValue({ ...groupInfo })
    harness.members.mockReset().mockResolvedValue({ list: [
      { userId: 1, userNickname: 'Owner', avatar: '', role: 1, memberNickname: '', createdAt: 'now' },
      { userId: 2, userNickname: 'Peer', avatar: '', role: 3, memberNickname: 'P', createdAt: 'now' },
    ], count: 2 })
    harness.history.mockReset().mockResolvedValue({ list: [firstMessage], hasMore: false })
    harness.markRead.mockReset().mockResolvedValue({ ...session, unreadCount: 0, lastReadMessageID: 10 })
    harness.create.mockReset().mockResolvedValue({ ...groupInfo, groupId: 6, title: 'New Group' })
    harness.setMemberMute.mockReset().mockResolvedValue({ updated: true })
  })

  it('starts one independent socket and keeps server group/session collections distinct', async () => {
    const store = await startStore()
    expect(harness.sockets).toHaveLength(1)
    expect(store.sessions[0]?.group.groupID).toBe(5)
    expect(store.groups[0]?.groupId).toBe(5)
    store.start(1, 'test-token')
    expect(harness.sockets).toHaveLength(1)
  })

  it('opens a group and restores info, members, history and read state', async () => {
    const store = await openStore()
    expect(harness.info).toHaveBeenCalledWith(5)
    expect(harness.members).toHaveBeenCalledWith(5)
    expect(harness.history).toHaveBeenCalledWith(5, undefined, 20)
    expect(harness.markRead).toHaveBeenCalledWith(5, 10)
    expect(store.senderName(2)).toBe('P')
  })

  it('loads older history using the backend message cursor and deduplicates overlap', async () => {
    const older = { ...firstMessage, messageId: 8, clientMsgId: 'remote-8' }
    harness.history.mockResolvedValueOnce({ list: [firstMessage], hasMore: true, nextBeforeMessageID: 10 }).mockResolvedValueOnce({ list: [older, firstMessage], hasMore: false })
    const store = await openStore()
    await store.loadOlder()
    expect(harness.history).toHaveBeenLastCalledWith(5, 10, 20)
    expect(store.currentMessages.map((message) => message.messageId)).toEqual([8, 10])
  })

  it('uses one stable clientMsgId through reconnect resend', async () => {
    const store = await openStore()
    latestSocket().options.onStateChange('connected')
    const clientMsgId = store.sendText('hello group')!
    latestSocket().options.onStateChange('reconnecting')
    latestSocket().options.onStateChange('connected')
    const sent = latestSocket().send.mock.calls.map((call) => call[0].clientMsgId)
    expect(sent).toEqual([clientMsgId, clientMsgId])
  })

  it('merges ACK before group.message into one sent item', async () => {
    const store = await openStore()
    latestSocket().options.onStateChange('connected')
    const clientMsgId = store.sendText('ack first')!
    latestSocket().options.onEnvelope({ event: 'group.ack', clientMsgId, data: { messageId: 11, groupID: 5, createdAt: '2026-10-13T00:01:00Z' } })
    latestSocket().options.onEnvelope({ event: 'group.message', data: { messageId: 11, groupID: 5, sendUserID: 1, clientMsgId, msg: { type: 1, textMsg: { content: 'ack first' } }, createdAt: '2026-10-13T00:01:00Z' } })
    expect(store.currentMessages.filter((message) => message.messageId === 11)).toHaveLength(1)
    expect(store.currentMessages.find((message) => message.clientMsgId === clientMsgId)?.status).toBe('sent')
  })

  it('merges group.message before ACK into one sent item', async () => {
    const store = await openStore()
    latestSocket().options.onStateChange('connected')
    const clientMsgId = store.sendText('message first')!
    const incoming = { messageId: 12, groupID: 5, sendUserID: 1, clientMsgId, msg: { type: 1 as const, textMsg: { content: 'message first' } }, createdAt: '2026-10-13T00:02:00Z' }
    latestSocket().options.onEnvelope({ event: 'group.message', data: incoming })
    latestSocket().options.onEnvelope({ event: 'group.ack', clientMsgId, data: { messageId: 12, groupID: 5, createdAt: incoming.createdAt } })
    expect(store.currentMessages.filter((message) => message.clientMsgId === clientMsgId)).toHaveLength(1)
    expect(store.currentMessages.find((message) => message.clientMsgId === clientMsgId)).toMatchObject({ messageId: 12, status: 'sent' })
  })

  it('ignores duplicate realtime delivery by messageId', async () => {
    const store = await openStore()
    const incoming = { ...firstMessage, messageId: 13, clientMsgId: 'remote-13' }
    latestSocket().options.onEnvelope({ event: 'group.message', data: incoming })
    latestSocket().options.onEnvelope({ event: 'group.message', data: incoming })
    expect(store.currentMessages.filter((message) => message.messageId === 13)).toHaveLength(1)
  })

  it('marks ACK timeout failed and retries with the same ID', async () => {
    vi.useFakeTimers()
    try {
      const store = useGroupStore()
      store.start(1, 'test-token')
      await Promise.resolve()
      await store.openGroup(5)
      latestSocket().options.onStateChange('connected')
      const clientMsgId = store.sendText('lost ack')!
      vi.advanceTimersByTime(10_000)
      expect(store.currentMessages.find((message) => message.clientMsgId === clientMsgId)?.status).toBe('failed')
      store.retryMessage(clientMsgId)
      expect(latestSocket().send.mock.calls.at(-1)?.[0].clientMsgId).toBe(clientMsgId)
    } finally {
      vi.useRealTimers()
    }
  })

  it('keeps socket connected and fails a muted sender with a safe message', async () => {
    const store = await openStore()
    latestSocket().options.onStateChange('connected')
    const clientMsgId = store.sendText('blocked')!
    latestSocket().options.onEnvelope({ event: 'error', clientMsgId, error: { code: 2405, message: 'internal detail' } })
    expect(store.currentMessages.find((message) => message.clientMsgId === clientMsgId)).toMatchObject({ status: 'failed', retryable: true, errorMessage: '当前账号已被禁言，消息未发送' })
    expect(store.connectionState).toBe('connected')
  })

  it('marks nonmember errors non-retryable and refreshes authoritative lists', async () => {
    const store = await openStore()
    latestSocket().options.onStateChange('connected')
    const clientMsgId = store.sendText('removed')!
    harness.sessions.mockClear()
    latestSocket().options.onEnvelope({ event: 'error', clientMsgId, error: { code: 2404, message: 'sql detail' } })
    expect(store.currentUnavailable).toBe(true)
    expect(store.currentMessages.find((message) => message.clientMsgId === clientMsgId)).toMatchObject({ status: 'failed', retryable: false })
    await vi.waitFor(() => expect(harness.sessions).toHaveBeenCalled())
  })

  it('increments inactive-group unread locally and keeps backend reconciliation scheduled', async () => {
    const otherSession = { ...session, group: { groupID: 6, name: 'Other', avatar: '' }, lastMessageID: 0, unreadCount: 0 }
    harness.sessions.mockResolvedValue({ list: [{ ...session }, otherSession], count: 2 })
    const store = await openStore()
    latestSocket().options.onEnvelope({ event: 'group.message', data: { ...firstMessage, messageId: 20, groupID: 6, clientMsgId: 'remote-20' } })
    expect(store.sessions.find((item) => item.group.groupID === 6)?.unreadCount).toBe(1)
  })

  it('auto-reads current-group realtime only while visible and focused', async () => {
    const store = await openStore()
    harness.markRead.mockClear()
    store.setGroupActive(true)
    latestSocket().options.onEnvelope({ event: 'group.message', data: { ...firstMessage, messageId: 21, clientMsgId: 'remote-21' } })
    await vi.waitFor(() => expect(harness.markRead).toHaveBeenCalledWith(5, 21))
    harness.markRead.mockClear()
    store.setGroupActive(false)
    latestSocket().options.onEnvelope({ event: 'group.message', data: { ...firstMessage, messageId: 22, clientMsgId: 'remote-22' } })
    await Promise.resolve()
    expect(harness.markRead).not.toHaveBeenCalled()
  })

  it('creates a minimal group and refreshes My Groups', async () => {
    const store = await startStore()
    harness.myGroups.mockClear()
    const created = await store.createGroup({ name: 'New Group', userIdList: [2, 3] })
    expect(harness.create).toHaveBeenCalledWith({ name: 'New Group', userIdList: [2, 3] })
    expect(created?.groupId).toBe(6)
    expect(harness.myGroups).toHaveBeenCalled()
  })

  it('exposes role-gated mute actions through the authoritative API', async () => {
    const store = await openStore()
    expect(await store.setMemberMute(2, true)).toBe(true)
    expect(await store.setMemberMute(2, false)).toBe(true)
    expect(harness.setMemberMute).toHaveBeenNthCalledWith(1, 5, 2, 5)
    expect(harness.setMemberMute).toHaveBeenNthCalledWith(2, 5, 2, undefined)
  })

  it('stops and clears group-only memory without touching chat state', async () => {
    const store = await openStore()
    store.stop()
    expect(latestSocket().disconnect).toHaveBeenCalled()
    expect(store.sessions).toEqual([])
    expect(store.groups).toEqual([])
    expect(store.messagesByGroup).toEqual({})
  })

  it.each([1205, 1206])('treats HTTP auth code %i as terminal', async (code) => {
    const onFailure = vi.fn()
    harness.sessions.mockRejectedValueOnce(new PublicApiError('expired', code))
    const store = useGroupStore()
    store.start(1, 'invalid', onFailure)
    await vi.waitFor(() => expect(latestSocket().failAuthentication).toHaveBeenCalledOnce())
    await vi.waitFor(() => expect(onFailure).toHaveBeenCalledOnce())
    expect(latestSocket().disconnect).toHaveBeenCalled()
    expect(store.connectionState).toBe('disconnected')
  })
})
