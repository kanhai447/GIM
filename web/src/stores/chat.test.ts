import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { PublicApiError } from '@/api/client'
import { useFriendStore } from '@/stores/friend'
import type { ChatConnectionState, ChatSendEnvelope, ChatServerEnvelope, ChatSession } from '@/types/chat'

interface SocketCallbacks {
  onStateChange: (state: ChatConnectionState) => void
  onEnvelope: (envelope: ChatServerEnvelope) => void
  onAuthenticationFailure: () => void
}

interface FakeSocketInstance {
  options: SocketCallbacks
  connect: ReturnType<typeof vi.fn>
  disconnect: ReturnType<typeof vi.fn>
  failAuthentication: ReturnType<typeof vi.fn>
  send: ReturnType<typeof vi.fn<(envelope: ChatSendEnvelope) => boolean>>
}

const harness = vi.hoisted(() => ({
  sessions: vi.fn(),
  history: vi.fn(),
  markRead: vi.fn(),
  setTop: vi.fn(),
  uploadImage: vi.fn(),
  upload: vi.fn(),
  sockets: [] as FakeSocketInstance[],
}))

vi.mock('@/api/chat', () => ({
  chatApi: {
    sessions: harness.sessions,
    history: harness.history,
    markRead: harness.markRead,
    setTop: harness.setTop,
  },
}))

vi.mock('@/api/file', () => ({
  fileApi: { uploadImage: harness.uploadImage, upload: harness.upload },
}))

vi.mock('@/service/ws/chat_socket', () => ({
  ChatSocketClient: class {
    connect = vi.fn()
    disconnect = vi.fn()
    send = vi.fn<(envelope: ChatSendEnvelope) => boolean>(() => true)
    failAuthentication = vi.fn(() => this.options.onAuthenticationFailure())

    constructor(readonly options: SocketCallbacks) {
      harness.sockets.push(this)
    }
  },
}))

import { useChatStore } from '@/stores/chat'

const firstMessage = {
  messageId: 10,
  sendUserID: 2,
  revUserID: 1,
  clientMsgId: 'remote-10',
  msg: { type: 1 as const, textMsg: { content: 'first' } },
  createdAt: '2026-10-11T00:00:00Z',
}

const session: ChatSession = {
  peer: { userID: 2, nickname: 'Peer', avatar: '' },
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
  const store = useChatStore()
  store.start(1, 'test-token')
  await vi.waitFor(() => expect(harness.sessions).toHaveBeenCalled())
  return store
}

describe('chat store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    harness.sockets.length = 0
    harness.sessions.mockReset().mockResolvedValue({ list: [{ ...session }], count: 1 })
    harness.history.mockReset().mockResolvedValue({ list: [firstMessage], hasMore: false })
    harness.markRead.mockReset().mockResolvedValue({ ...session, unreadCount: 0, lastReadMessageID: 10 })
    harness.setTop.mockReset().mockResolvedValue({ ...session, isTop: true })
    harness.uploadImage.mockReset()
    harness.upload.mockReset()
  })

  it('starts one socket and loads server-ordered sessions', async () => {
    const store = await startStore()
    expect(harness.sockets).toHaveLength(1)
    expect(latestSocket().connect).toHaveBeenCalledOnce()
    expect(store.sessions.map((item) => item.peer.userID)).toEqual([2])

    store.start(1, 'test-token')
    expect(harness.sockets).toHaveLength(1)
  })

  it('opens a peer, loads latest history, and marks the latest message read', async () => {
    const store = await startStore()
    await store.openConversation(2)
    expect(harness.history).toHaveBeenCalledWith(2, undefined, 20)
    expect(store.currentMessages.map((item) => item.messageId)).toEqual([10])
    expect(harness.markRead).toHaveBeenCalledWith(2, 10)
    expect(store.sessions[0]?.unreadCount).toBe(0)
  })

  it('loads older pages by cursor and merges without duplicates', async () => {
    const older = { ...firstMessage, messageId: 8, clientMsgId: 'remote-8', createdAt: '2026-10-10T23:00:00Z' }
    harness.history
      .mockResolvedValueOnce({ list: [firstMessage], hasMore: true, nextBeforeMessageID: 10 })
      .mockResolvedValueOnce({ list: [older, firstMessage], hasMore: false })
    const store = await startStore()
    await store.openConversation(2)
    await store.loadOlder()
    expect(harness.history).toHaveBeenLastCalledWith(2, 10, 20)
    expect(store.currentMessages.map((item) => item.messageId)).toEqual([8, 10])
  })

  it('keeps a stable clientMsgId when pending is resent after reconnect', async () => {
    const store = await startStore()
    await store.openConversation(2)
    latestSocket().options.onStateChange('connected')
    const clientMsgId = store.sendText('hello')!
    const firstFrame = latestSocket().send.mock.calls.at(-1)?.[0]
    latestSocket().options.onStateChange('reconnecting')
    latestSocket().options.onStateChange('connected')
    const secondFrame = latestSocket().send.mock.calls.at(-1)?.[0]
    expect(firstFrame?.clientMsgId).toBe(clientMsgId)
    expect(secondFrame?.clientMsgId).toBe(clientMsgId)
  })

  it('uploads an image before sending type=2 and keeps the same clientMsgId on retry', async () => {
    harness.uploadImage.mockResolvedValue({ id: 42, url: '/api/file/42', size: 2048, mimeType: 'image/png' })
    const store = await startStore()
    await store.openConversation(2)
    latestSocket().options.onStateChange('connected')
    const clientMsgId = await store.sendImage(new File(['image-bytes'], 'photo.png', { type: 'image/png' }))
    expect(clientMsgId).not.toBeNull()
    expect(harness.uploadImage).toHaveBeenCalledOnce()
    expect(latestSocket().send.mock.calls.at(-1)?.[0].data.msg).toEqual({
      type: 2,
      imageMsg: { fileID: 42, url: '/api/file/42', size: 2048, mimeType: 'image/png' },
    })
    expect(store.currentMessages.find((item) => item.clientMsgId === clientMsgId)?.msg.type).toBe(2)
    store.retryMessage(clientMsgId!)
    expect(latestSocket().send.mock.calls.at(-1)?.[0].clientMsgId).toBe(clientMsgId)
  })

  it('does not send an image message when the upload fails', async () => {
    harness.uploadImage.mockRejectedValue(new PublicApiError('图片上传失败', 2502))
    const store = await startStore()
    await store.openConversation(2)
    latestSocket().options.onStateChange('connected')
    const clientMsgId = await store.sendImage(new File(['x'], 'bad.png', { type: 'image/png' }))
    expect(clientMsgId).toBeNull()
    expect(store.lastError).toBe('图片上传失败')
    expect(store.currentMessages.some((item) => item.msg.type === 2)).toBe(false)
    expect(latestSocket().send).not.toHaveBeenCalled()
  })

  it('uploads a file and sends type=3 with server metadata', async () => {
    harness.upload.mockResolvedValue({ id: 51, url: '/api/file/51', fileName: 'report.pdf', size: 2048, mimeType: 'application/pdf' })
    const store = await startStore()
    await store.openConversation(2)
    latestSocket().options.onStateChange('connected')
    const clientMsgId = await store.sendFile(new File(['pdf'], 'report.pdf', { type: 'application/pdf' }))
    expect(clientMsgId).not.toBeNull()
    expect(harness.upload).toHaveBeenCalledOnce()
    expect(latestSocket().send.mock.calls.at(-1)?.[0].data.msg).toEqual({
      type: 3,
      fileMsg: { fileID: 51, url: '/api/file/51', fileName: 'report.pdf', size: 2048, mimeType: 'application/pdf' },
    })
    expect(store.currentMessages.find((item) => item.clientMsgId === clientMsgId)?.msg.type).toBe(3)
  })

  it('does not send a file message when the upload fails', async () => {
    harness.upload.mockRejectedValue(new PublicApiError('文件上传失败', 2502))
    const store = await startStore()
    await store.openConversation(2)
    latestSocket().options.onStateChange('connected')
    const clientMsgId = await store.sendFile(new File(['x'], 'bad.bin', { type: 'application/octet-stream' }))
    expect(clientMsgId).toBeNull()
    expect(store.lastError).toBe('文件上传失败')
    expect(store.currentMessages.some((item) => item.msg.type === 3)).toBe(false)
    expect(latestSocket().send).not.toHaveBeenCalled()
  })

  it('applies friend presence events to the friend store', async () => {
    const store = await startStore()
    const friendStore = useFriendStore()
    friendStore.friends = [{
      userID: 2, account: 'friend', nickname: 'Friend', avatar: '', abstract: '', remark: '',
      isOnline: false, createdAt: '2026-11-14T12:00:00Z',
    }]
    latestSocket().options.onEnvelope({ event: 'friend.online', data: { userID: 2, at: '2026-11-14T12:00:00Z' } })
    expect(friendStore.friends[0]?.isOnline).toBe(true)
    latestSocket().options.onEnvelope({ event: 'friend.offline', data: { userID: 2, at: '2026-11-14T12:00:01Z' } })
    expect(friendStore.friends[0]?.isOnline).toBe(false)
    latestSocket().options.onEnvelope({ event: 'friend.online', data: { userID: 999, at: '2026-11-14T12:00:02Z' } })
    expect(friendStore.friends).toHaveLength(1)
  })

  it('recalls an own sent message and replaces it on ACK', async () => {
    const store = await startStore()
    await store.openConversation(2)
    latestSocket().options.onStateChange('connected')
    const targetClient = store.sendText('to recall')!
    latestSocket().options.onEnvelope({ event: 'chat.ack', clientMsgId: targetClient, data: { messageId: 30, createdAt: '2026-11-14T12:00:00Z' } })
    const recallClient = store.recallMessage(30)!
    expect(latestSocket().send.mock.calls.at(-1)?.[0].data.msg).toEqual({ type: 4, withdrawMsg: { msgID: 30 } })
    expect(store.recalling).toEqual([30])
    latestSocket().options.onEnvelope({ event: 'chat.ack', clientMsgId: recallClient, data: { messageId: 31, createdAt: '2026-11-14T12:00:01Z' } })
    expect(store.currentMessages.find((item) => item.messageId === 30)?.msg).toEqual({ type: 4, withdrawMsg: { msgID: 30 } })
    expect(store.recalling).toEqual([])
  })

  it('keeps the original and reports an error when recall fails', async () => {
    const store = await startStore()
    await store.openConversation(2)
    latestSocket().options.onStateChange('connected')
    const targetClient = store.sendText('keep me')!
    latestSocket().options.onEnvelope({ event: 'chat.ack', clientMsgId: targetClient, data: { messageId: 32, createdAt: '2026-11-14T12:00:00Z' } })
    const recallClient = store.recallMessage(32)!
    latestSocket().options.onEnvelope({ event: 'error', clientMsgId: recallClient, error: { code: 2220, message: '超过撤回时间窗口' } })
    expect(store.currentMessages.find((item) => item.messageId === 32)?.msg.type).toBe(1)
    expect(store.lastError).toBe('超过撤回时间窗口')
    expect(store.recalling).toEqual([])
  })

  it('applies a pushed withdraw to the target without adding a separate row', async () => {
    const store = await startStore()
    await store.openConversation(2)
    const before = store.currentMessages.length
    latestSocket().options.onEnvelope({
      event: 'chat.message',
      data: { messageId: 33, sendUserID: 1, revUserID: 2, clientMsgId: 'push-recall', msg: { type: 4, withdrawMsg: { msgID: 10 } }, createdAt: '2026-11-14T12:00:02Z' },
    })
    expect(store.currentMessages.find((item) => item.messageId === 10)?.msg).toEqual({ type: 4, withdrawMsg: { msgID: 10 } })
    expect(store.currentMessages).toHaveLength(before)
    expect(store.currentMessages.some((item) => item.messageId === 33)).toBe(false)
  })

  it('moves pending to sent and assigns messageId on ACK without adding a duplicate', async () => {
    const store = await startStore()
    await store.openConversation(2)
    latestSocket().options.onStateChange('connected')
    const clientMsgId = store.sendText('hello')!
    const before = store.currentMessages.length
    const ack: ChatServerEnvelope = { event: 'chat.ack', clientMsgId, data: { messageId: 11, createdAt: '2026-10-11T00:01:00Z' } }
    latestSocket().options.onEnvelope(ack)
    latestSocket().options.onEnvelope(ack)
    expect(store.currentMessages).toHaveLength(before)
    expect(store.currentMessages.find((item) => item.clientMsgId === clientMsgId)).toMatchObject({ status: 'sent', messageId: 11 })
  })

  it('marks only the correlated pending message failed on an error event', async () => {
    const store = await startStore()
    await store.openConversation(2)
    latestSocket().options.onStateChange('connected')
    const clientMsgId = store.sendText('hello')!
    latestSocket().options.onEnvelope({ event: 'error', clientMsgId, error: { code: 2201, message: '消息参数无效' } })
    expect(store.currentMessages.find((item) => item.clientMsgId === clientMsgId)).toMatchObject({ status: 'failed', errorMessage: '消息参数无效' })
    expect(store.connectionState).toBe('connected')
  })

  it('marks ACK timeout failed and retries with the same clientMsgId', async () => {
    vi.useFakeTimers()
    try {
      const store = useChatStore()
      store.start(1, 'test-token')
      await Promise.resolve()
      await store.openConversation(2)
      latestSocket().options.onStateChange('connected')
      const clientMsgId = store.sendText('timeout')!
      vi.advanceTimersByTime(10_000)
      expect(store.currentMessages.find((item) => item.clientMsgId === clientMsgId)?.status).toBe('failed')
      store.retryMessage(clientMsgId)
      expect(latestSocket().send.mock.calls.at(-1)?.[0].clientMsgId).toBe(clientMsgId)
    } finally {
      vi.useRealTimers()
    }
  })

  it('ignores duplicate chat.message by server message ID', async () => {
    const store = await startStore()
    await store.openConversation(2)
    const incoming = { ...firstMessage, messageId: 12, clientMsgId: 'remote-12' }
    latestSocket().options.onEnvelope({ event: 'chat.message', data: incoming })
    latestSocket().options.onEnvelope({ event: 'chat.message', data: incoming })
    expect(store.currentMessages.filter((item) => item.messageId === 12)).toHaveLength(1)
  })

  it('increments another peer unread locally before bounded server reconciliation', async () => {
    const otherSession = { ...session, peer: { userID: 3, nickname: 'Other', avatar: '' }, unreadCount: 0 }
    harness.sessions.mockResolvedValue({ list: [{ ...session }, otherSession], count: 2 })
    const store = await startStore()
    await store.openConversation(2)
    latestSocket().options.onEnvelope({ event: 'chat.message', data: { ...firstMessage, messageId: 20, sendUserID: 3, clientMsgId: 'remote-20' } })
    expect(store.sessions.find((item) => item.peer.userID === 3)?.unreadCount).toBe(1)
  })

  it('marks realtime current-peer messages read only while the conversation is active', async () => {
    const store = await startStore()
    await store.openConversation(2)
    harness.markRead.mockClear()
    store.setConversationActive(true)
    latestSocket().options.onEnvelope({ event: 'chat.message', data: { ...firstMessage, messageId: 21, clientMsgId: 'remote-21' } })
    await vi.waitFor(() => expect(harness.markRead).toHaveBeenCalledWith(2, 21))
  })

  it('does not auto-read realtime messages while the current conversation is inactive', async () => {
    const store = await startStore()
    await store.openConversation(2)
    harness.markRead.mockClear()
    store.setConversationActive(false)
    latestSocket().options.onEnvelope({ event: 'chat.message', data: { ...firstMessage, messageId: 22, clientMsgId: 'remote-22' } })
    await Promise.resolve()
    expect(harness.markRead).not.toHaveBeenCalled()
    expect(store.sessions[0]?.unreadCount).toBe(1)
  })

  it('logout closes the singleton socket and clears in-memory chat state', async () => {
    const store = await startStore()
    store.stop()
    expect(latestSocket().disconnect).toHaveBeenCalled()
    expect(store.connectionState).toBe('disconnected')
    expect(store.sessions).toEqual([])
    expect(store.messagesByPeer).toEqual({})
  })

  it('authentication failure stops reconnect, clears state, and invokes the app handler', async () => {
    const onFailure = vi.fn()
    const store = useChatStore()
    store.start(1, 'test-token', onFailure)
    await vi.waitFor(() => expect(store.sessions).toHaveLength(1))
    latestSocket().failAuthentication()
    await vi.waitFor(() => expect(onFailure).toHaveBeenCalledOnce())
    expect(store.sessions).toEqual([])
    expect(store.connectionState).toBe('disconnected')
  })

  it.each([1205, 1206])('treats HTTP auth code %i as terminal and stops reconnect', async (code) => {
    const onFailure = vi.fn()
    harness.sessions.mockRejectedValueOnce(new PublicApiError('登录状态已失效', code))
    const store = useChatStore()
    store.start(1, 'invalid-token', onFailure)
    await vi.waitFor(() => expect(latestSocket().failAuthentication).toHaveBeenCalledOnce())
    await vi.waitFor(() => expect(onFailure).toHaveBeenCalledOnce())
    expect(latestSocket().disconnect).toHaveBeenCalled()
    expect(store.sessions).toEqual([])
    expect(store.connectionState).toBe('disconnected')
  })
})
