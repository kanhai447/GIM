import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { chatApi } from '@/api/chat'
import { PublicApiError } from '@/api/client'
import { ChatSocketClient } from '@/service/ws/chat_socket'
import {
  latestServerMessageID,
  mergeServerMessages,
  messagePreview,
  peerIDForMessage,
  toSentMessage,
} from '@/stores/chat_reconcile'
import type {
  ChatConnectionState,
  ChatHistoryState,
  ChatMessageItem,
  ChatServerEnvelope,
  ChatSession,
  PrivateChatMessage,
} from '@/types/chat'

const ACK_TIMEOUT_MS = 10_000
const SESSION_REFRESH_DELAY_MS = 250
const AUTH_ERROR_CODES = new Set([1203, 1204, 1205, 1206])

function emptyHistoryState(): ChatHistoryState {
  return { loaded: false, loading: false, hasMore: true }
}

function newClientMsgID(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') return crypto.randomUUID()
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}-${Math.random().toString(36).slice(2)}`
}

function safeErrorMessage(error: unknown, fallback: string): string {
  return error instanceof PublicApiError ? error.message : fallback
}

export const useChatStore = defineStore('chat', () => {
  const sessions = ref<ChatSession[]>([])
  const sessionsLoading = ref(false)
  const currentPeerID = ref<number | null>(null)
  const messagesByPeer = ref<Record<string, ChatMessageItem[]>>({})
  const historyByPeer = ref<Record<string, ChatHistoryState>>({})
  const connectionState = ref<ChatConnectionState>('disconnected')
  const currentUserID = ref(0)
  const lastError = ref('')

  let token = ''
  let socket: ChatSocketClient | null = null
  let authFailureHandler: (() => void | Promise<void>) | null = null
  let sessionRefreshTimer: ReturnType<typeof setTimeout> | null = null
  let outageChecked = false
  let conversationActive = false
  const ackTimers = new Map<string, ReturnType<typeof setTimeout>>()
  const readInFlight = new Set<number>()
  const readRequestedThrough = new Map<number, number>()

  const currentMessages = computed(() => {
    if (!currentPeerID.value) return []
    return messagesFor(currentPeerID.value)
  })
  const currentSession = computed(() => sessions.value.find((item) => item.peer.userID === currentPeerID.value) ?? null)
  const currentHistory = computed(() => currentPeerID.value ? historyFor(currentPeerID.value) : emptyHistoryState())

  function messagesFor(peerID: number): ChatMessageItem[] {
    const key = String(peerID)
    messagesByPeer.value[key] ??= []
    return messagesByPeer.value[key]!
  }

  function historyFor(peerID: number): ChatHistoryState {
    const key = String(peerID)
    historyByPeer.value[key] ??= emptyHistoryState()
    return historyByPeer.value[key]!
  }

  function isAuthenticationFailure(error: unknown): boolean {
    return error instanceof PublicApiError && (error.status === 401 || (error.code !== undefined && AUTH_ERROR_CODES.has(error.code)))
  }

  function failAuthentication(): void {
    socket?.failAuthentication()
  }

  async function notifyAuthenticationFailure(): Promise<void> {
    clearRuntime(true)
    if (authFailureHandler) await authFailureHandler()
  }

  function handleRequestFailure(error: unknown, fallback: string): void {
    if (isAuthenticationFailure(error)) {
      failAuthentication()
      return
    }
    lastError.value = safeErrorMessage(error, fallback)
  }

  function start(userID: number, authToken: string, onAuthenticationFailure?: () => void | Promise<void>): void {
    const normalizedToken = authToken.trim()
    if (!userID || !normalizedToken) {
      stop()
      return
    }
    if (onAuthenticationFailure) authFailureHandler = onAuthenticationFailure
    if (socket && currentUserID.value === userID && token === normalizedToken) {
      socket.connect()
      return
    }

    clearRuntime(true)
    currentUserID.value = userID
    token = normalizedToken
    socket = new ChatSocketClient({
      getToken: () => token,
      onStateChange: onSocketStateChange,
      onEnvelope: handleEnvelope,
      onAuthenticationFailure: () => { void notifyAuthenticationFailure() },
    })
    void refreshSessions()
    socket.connect()
  }

  function stop(): void {
    clearRuntime(true)
  }

  function clearRuntime(clearData: boolean): void {
    socket?.disconnect()
    socket = null
    token = ''
    currentUserID.value = 0
    connectionState.value = 'disconnected'
    conversationActive = false
    outageChecked = false
    if (sessionRefreshTimer) clearTimeout(sessionRefreshTimer)
    sessionRefreshTimer = null
    for (const timer of ackTimers.values()) clearTimeout(timer)
    ackTimers.clear()
    readInFlight.clear()
    readRequestedThrough.clear()
    if (!clearData) return
    sessions.value = []
    currentPeerID.value = null
    messagesByPeer.value = {}
    historyByPeer.value = {}
    lastError.value = ''
  }

  function onSocketStateChange(state: ChatConnectionState): void {
    connectionState.value = state
    if (state === 'connected') {
      outageChecked = false
      flushPendingMessages()
      scheduleSessionRefresh()
      return
    }
    if (state === 'reconnecting') {
      clearAckTimersForPending()
      if (!outageChecked) {
        outageChecked = true
        void refreshSessions()
      }
    }
  }

  async function refreshSessions(): Promise<void> {
    if (!currentUserID.value || !token || sessionsLoading.value) return
    sessionsLoading.value = true
    try {
      const result = await chatApi.sessions()
      sessions.value = result.list
      lastError.value = ''
    } catch (error) {
      handleRequestFailure(error, '会话列表加载失败')
    } finally {
      sessionsLoading.value = false
    }
  }

  function scheduleSessionRefresh(): void {
    if (sessionRefreshTimer || !currentUserID.value) return
    sessionRefreshTimer = setTimeout(() => {
      sessionRefreshTimer = null
      void refreshSessions()
    }, SESSION_REFRESH_DELAY_MS)
  }

  async function openConversation(peerID: number): Promise<void> {
    if (!Number.isSafeInteger(peerID) || peerID <= 0 || peerID === currentUserID.value) return
    currentPeerID.value = peerID
    const state = historyFor(peerID)
    if (!state.loaded) await loadLatest(peerID)
    await markConversationRead(peerID)
  }

  function closeConversation(): void {
    currentPeerID.value = null
    conversationActive = false
  }

  async function loadLatest(peerID = currentPeerID.value): Promise<void> {
    if (!peerID) return
    const state = historyFor(peerID)
    if (state.loading) return
    state.loading = true
    try {
      const page = await chatApi.history(peerID, undefined, 20)
      messagesByPeer.value[String(peerID)] = mergeServerMessages(messagesFor(peerID), page.list, currentUserID.value)
      state.loaded = true
      state.hasMore = page.hasMore
      state.nextBeforeMessageID = page.nextBeforeMessageID
      lastError.value = ''
    } catch (error) {
      handleRequestFailure(error, '历史消息加载失败')
    } finally {
      state.loading = false
    }
  }

  async function loadOlder(peerID = currentPeerID.value): Promise<void> {
    if (!peerID) return
    const state = historyFor(peerID)
    if (state.loading || !state.loaded || !state.hasMore || !state.nextBeforeMessageID) return
    state.loading = true
    try {
      const page = await chatApi.history(peerID, state.nextBeforeMessageID, 20)
      messagesByPeer.value[String(peerID)] = mergeServerMessages(messagesFor(peerID), page.list, currentUserID.value)
      state.hasMore = page.hasMore
      state.nextBeforeMessageID = page.nextBeforeMessageID
      lastError.value = ''
    } catch (error) {
      handleRequestFailure(error, '更早消息加载失败')
    } finally {
      state.loading = false
    }
  }

  function sendText(content: string): string | null {
    const peerID = currentPeerID.value
    const text = content.trim()
    if (!peerID || !currentUserID.value || !text) return null
    const clientMsgId = newClientMsgID()
    messagesFor(peerID).push({
      localID: `client:${clientMsgId}`,
      sendUserID: currentUserID.value,
      revUserID: peerID,
      clientMsgId,
      msg: { type: 1, textMsg: { content: text } },
      createdAt: new Date().toISOString(),
      status: 'pending',
    })
    sendPending(clientMsgId)
    return clientMsgId
  }

  function retryMessage(clientMsgId: string): void {
    const message = findByClientMsgID(clientMsgId)
    if (!message || message.sendUserID !== currentUserID.value) return
    message.status = 'pending'
    message.errorMessage = undefined
    sendPending(clientMsgId)
  }

  function sendPending(clientMsgId: string): void {
    const message = findByClientMsgID(clientMsgId)
    if (!message || message.status !== 'pending' || !socket) return
    const sent = socket.send({
      event: 'chat.send',
      clientMsgId: message.clientMsgId,
      data: { revUserID: message.revUserID, msg: message.msg },
    })
    if (!sent) return
    clearAckTimer(clientMsgId)
    ackTimers.set(clientMsgId, setTimeout(() => {
      ackTimers.delete(clientMsgId)
      const pending = findByClientMsgID(clientMsgId)
      if (!pending || pending.status !== 'pending') return
      pending.status = 'failed'
      pending.errorMessage = '服务端确认超时，可重试'
    }, ACK_TIMEOUT_MS))
  }

  function flushPendingMessages(): void {
    for (const messages of Object.values(messagesByPeer.value)) {
      for (const message of messages) {
        if (message.status === 'pending') sendPending(message.clientMsgId)
      }
    }
  }

  function clearAckTimersForPending(): void {
    for (const [clientMsgId, timer] of ackTimers) {
      clearTimeout(timer)
      ackTimers.delete(clientMsgId)
    }
  }

  function clearAckTimer(clientMsgId: string): void {
    const timer = ackTimers.get(clientMsgId)
    if (timer) clearTimeout(timer)
    ackTimers.delete(clientMsgId)
  }

  function findByClientMsgID(clientMsgId: string): ChatMessageItem | undefined {
    for (const messages of Object.values(messagesByPeer.value)) {
      const found = messages.find((item) => item.clientMsgId === clientMsgId)
      if (found) return found
    }
    return undefined
  }

  function handleEnvelope(envelope: ChatServerEnvelope): void {
    if (envelope.event === 'chat.ack') {
      const message = findByClientMsgID(envelope.clientMsgId)
      if (!message) return
      clearAckTimer(envelope.clientMsgId)
      message.messageId = envelope.data.messageId
      message.createdAt = envelope.data.createdAt
      message.status = 'sent'
      message.errorMessage = undefined
      updateLocalSession({
        messageId: envelope.data.messageId,
        sendUserID: message.sendUserID,
        revUserID: message.revUserID,
        clientMsgId: message.clientMsgId,
        msg: message.msg,
        createdAt: envelope.data.createdAt,
      }, false)
      scheduleSessionRefresh()
      return
    }
    if (envelope.event === 'error') {
      if (!envelope.clientMsgId) return
      const message = findByClientMsgID(envelope.clientMsgId)
      if (!message) return
      clearAckTimer(envelope.clientMsgId)
      message.status = 'failed'
      message.errorMessage = envelope.error.message || '消息发送失败'
      return
    }
    receiveMessage(envelope.data)
  }

  function receiveMessage(message: PrivateChatMessage): void {
    const peerID = peerIDForMessage(message, currentUserID.value)
    const existing = messagesFor(peerID)
    if (existing.some((item) => item.messageId === message.messageId)) return
    messagesByPeer.value[String(peerID)] = mergeServerMessages(existing, [message], currentUserID.value)
    const shouldMarkRead = currentPeerID.value === peerID && conversationActive
    updateLocalSession(message, !shouldMarkRead && message.sendUserID === peerID)
    scheduleSessionRefresh()
    if (shouldMarkRead) void markConversationRead(peerID)
  }

  function updateLocalSession(message: PrivateChatMessage, incrementUnread: boolean): void {
    const peerID = peerIDForMessage(message, currentUserID.value)
    const session = sessions.value.find((item) => item.peer.userID === peerID)
    if (!session) {
      scheduleSessionRefresh()
      return
    }
    if (message.messageId >= session.lastMessageID) {
      session.lastMessage = message
      session.lastMessageID = message.messageId
      session.lastMessageAt = message.createdAt
    }
    if (incrementUnread) session.unreadCount += 1
  }

  async function markConversationRead(peerID = currentPeerID.value): Promise<void> {
    if (!peerID) return
    const target = latestServerMessageID(messagesFor(peerID))
    if (!target) return
    readRequestedThrough.set(peerID, Math.max(readRequestedThrough.get(peerID) ?? 0, target))
    if (readInFlight.has(peerID)) return

    readInFlight.add(peerID)
    try {
      let completed = 0
      while ((readRequestedThrough.get(peerID) ?? 0) > completed) {
        const requested = readRequestedThrough.get(peerID)!
        const updated = await chatApi.markRead(peerID, requested)
        completed = requested
        const session = sessions.value.find((item) => item.peer.userID === peerID)
        if (session) {
          session.unreadCount = updated.unreadCount
          session.lastReadMessageID = updated.lastReadMessageID
        }
      }
      lastError.value = ''
    } catch (error) {
      handleRequestFailure(error, '已读状态更新失败')
    } finally {
      readInFlight.delete(peerID)
      readRequestedThrough.delete(peerID)
    }
  }

  function setConversationActive(active: boolean): void {
    conversationActive = active
    if (active) void markConversationRead()
  }

  async function setSessionTop(peerID: number, isTop: boolean): Promise<void> {
    try {
      await chatApi.setTop(peerID, isTop)
      await refreshSessions()
    } catch (error) {
      handleRequestFailure(error, '置顶状态更新失败')
    }
  }

  function previewForSession(session: ChatSession): string {
    return session.lastMessage ? messagePreview(session.lastMessage) : '暂无消息'
  }

  return {
    sessions,
    sessionsLoading,
    currentPeerID,
    messagesByPeer,
    historyByPeer,
    connectionState,
    currentUserID,
    lastError,
    currentMessages,
    currentSession,
    currentHistory,
    start,
    stop,
    refreshSessions,
    openConversation,
    closeConversation,
    loadLatest,
    loadOlder,
    sendText,
    retryMessage,
    markConversationRead,
    setConversationActive,
    setSessionTop,
    previewForSession,
  }
})
