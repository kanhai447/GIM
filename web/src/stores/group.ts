import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { PublicApiError } from '@/api/client'
import { groupApi } from '@/api/group'
import { GroupSocketClient } from '@/service/ws/group_socket'
import { latestGroupMessageID, mergeGroupMessages } from '@/stores/group_reconcile'
import type {
  CreateGroupInput,
  GroupConnectionState,
  GroupHistoryState,
  GroupMember,
  GroupMessage,
  GroupMessageViewItem,
  GroupServerEnvelope,
  GroupSession,
  GroupSummary,
} from '@/types/group'

const ACK_TIMEOUT_MS = 10_000
const RECONCILE_DELAY_MS = 250
const AUTH_ERROR_CODES = new Set([1203, 1204, 1205, 1206])
const MEMBERSHIP_ERROR_CODES = new Set([2302, 2304, 2404])
const NON_RETRYABLE_ERROR_CODES = new Set([2302, 2304, 2404, 2407])

function emptyHistory(): GroupHistoryState {
  return { loaded: false, loading: false, hasMore: true }
}

function newClientMsgID(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') return crypto.randomUUID()
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}-${Math.random().toString(36).slice(2)}`
}

function safeRequestError(error: unknown, fallback: string): string {
  return error instanceof PublicApiError ? error.message : fallback
}

function safeSocketError(code: number): string {
  if (code === 2405) return '当前账号已被禁言，消息未发送'
  if (code === 2302) return '群聊已解散或不存在'
  if (code === 2304 || code === 2404) return '你已不是该群成员'
  if (code === 2407) return '消息标识冲突，请重新发送'
  if (code === 2403) return '消息内容无效'
  return '群消息发送失败，请稍后重试'
}

export const useGroupStore = defineStore('group', () => {
  const sessions = ref<GroupSession[]>([])
  const groups = ref<GroupSummary[]>([])
  const sessionsLoading = ref(false)
  const groupsLoading = ref(false)
  const currentGroupID = ref<number | null>(null)
  const currentGroup = ref<GroupSummary | null>(null)
  const currentMembers = ref<GroupMember[]>([])
  const messagesByGroup = ref<Record<string, GroupMessageViewItem[]>>({})
  const historyByGroup = ref<Record<string, GroupHistoryState>>({})
  const connectionState = ref<GroupConnectionState>('disconnected')
  const currentUserID = ref(0)
  const lastError = ref('')
  const currentUnavailable = ref(false)

  let token = ''
  let socket: GroupSocketClient | null = null
  let authFailureHandler: (() => void | Promise<void>) | null = null
  let reconcileTimer: ReturnType<typeof setTimeout> | null = null
  let groupActive = false
  let outageChecked = false
  const ackTimers = new Map<string, ReturnType<typeof setTimeout>>()
  const readInFlight = new Set<number>()
  const readRequestedThrough = new Map<number, number>()

  const currentMessages = computed(() => currentGroupID.value ? messagesFor(currentGroupID.value) : [])
  const currentSession = computed(() => sessions.value.find((item) => item.group.groupID === currentGroupID.value) ?? null)
  const currentHistory = computed(() => currentGroupID.value ? historyFor(currentGroupID.value) : emptyHistory())
  const memberByUserID = computed(() => new Map(currentMembers.value.map((member) => [member.userId, member])))

  function messagesFor(groupID: number): GroupMessageViewItem[] {
    const key = String(groupID)
    messagesByGroup.value[key] ??= []
    return messagesByGroup.value[key]!
  }

  function historyFor(groupID: number): GroupHistoryState {
    const key = String(groupID)
    historyByGroup.value[key] ??= emptyHistory()
    return historyByGroup.value[key]!
  }

  function isAuthenticationFailure(error: unknown): boolean {
    return error instanceof PublicApiError && (error.status === 401 || (error.code !== undefined && AUTH_ERROR_CODES.has(error.code)))
  }

  function handleRequestFailure(error: unknown, fallback: string): void {
    if (isAuthenticationFailure(error)) {
      socket?.failAuthentication()
      return
    }
    if (error instanceof PublicApiError && error.code !== undefined && MEMBERSHIP_ERROR_CODES.has(error.code)) {
      currentUnavailable.value = true
      lastError.value = safeSocketError(error.code)
      void reconcileGroupLists()
      return
    }
    lastError.value = safeRequestError(error, fallback)
  }

  async function notifyAuthenticationFailure(): Promise<void> {
    clearRuntime(true)
    if (authFailureHandler) await authFailureHandler()
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
    socket = new GroupSocketClient({
      getToken: () => token,
      onStateChange: handleSocketState,
      onEnvelope: handleEnvelope,
      onAuthenticationFailure: () => { void notifyAuthenticationFailure() },
    })
    void reconcileGroupLists()
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
    groupActive = false
    outageChecked = false
    if (reconcileTimer) clearTimeout(reconcileTimer)
    reconcileTimer = null
    for (const timer of ackTimers.values()) clearTimeout(timer)
    ackTimers.clear()
    readInFlight.clear()
    readRequestedThrough.clear()
    if (!clearData) return
    sessions.value = []
    groups.value = []
    currentGroupID.value = null
    currentGroup.value = null
    currentMembers.value = []
    messagesByGroup.value = {}
    historyByGroup.value = {}
    lastError.value = ''
    currentUnavailable.value = false
  }

  function handleSocketState(state: GroupConnectionState): void {
    connectionState.value = state
    if (state === 'connected') {
      outageChecked = false
      flushPending()
      scheduleReconcile()
      return
    }
    if (state === 'reconnecting') {
      clearAckTimers()
      if (!outageChecked) {
        outageChecked = true
        void reconcileGroupLists()
      }
    }
  }

  async function refreshSessions(): Promise<void> {
    if (!currentUserID.value || !token || sessionsLoading.value) return
    sessionsLoading.value = true
    try {
      const result = await groupApi.sessions()
      sessions.value = result.list
      lastError.value = ''
    } catch (error) {
      handleRequestFailure(error, '群会话加载失败')
    } finally {
      sessionsLoading.value = false
    }
  }

  async function refreshGroups(): Promise<void> {
    if (!currentUserID.value || !token || groupsLoading.value) return
    groupsLoading.value = true
    try {
      const result = await groupApi.myGroups()
      groups.value = result.list
      lastError.value = ''
    } catch (error) {
      handleRequestFailure(error, '我的群聊加载失败')
    } finally {
      groupsLoading.value = false
    }
  }

  async function reconcileGroupLists(): Promise<void> {
    await Promise.all([refreshSessions(), refreshGroups()])
  }

  function scheduleReconcile(): void {
    if (reconcileTimer || !currentUserID.value) return
    reconcileTimer = setTimeout(() => {
      reconcileTimer = null
      void reconcileGroupLists()
    }, RECONCILE_DELAY_MS)
  }

  async function openGroup(groupID: number): Promise<void> {
    if (!Number.isSafeInteger(groupID) || groupID <= 0) return
    currentGroupID.value = groupID
    currentUnavailable.value = false
    currentMembers.value = []
    await Promise.all([loadGroupInfo(groupID), loadMembers(groupID)])
    if (currentUnavailable.value || currentGroupID.value !== groupID) return
    const history = historyFor(groupID)
    if (!history.loaded) await loadLatest(groupID)
    await markGroupRead(groupID)
  }

  function closeGroup(): void {
    currentGroupID.value = null
    currentGroup.value = null
    currentMembers.value = []
    currentUnavailable.value = false
    groupActive = false
  }

  async function loadGroupInfo(groupID: number): Promise<void> {
    try {
      const info = await groupApi.info(groupID)
      if (currentGroupID.value === groupID) currentGroup.value = info
      const index = groups.value.findIndex((item) => item.groupId === groupID)
      if (index >= 0) groups.value[index] = info
      else groups.value.push(info)
    } catch (error) {
      handleRequestFailure(error, '群资料加载失败')
    }
  }

  async function loadMembers(groupID: number): Promise<void> {
    try {
      const result = await groupApi.members(groupID)
      if (currentGroupID.value === groupID) currentMembers.value = result.list
    } catch (error) {
      handleRequestFailure(error, '群成员加载失败')
    }
  }

  async function loadLatest(groupID = currentGroupID.value): Promise<void> {
    if (!groupID) return
    const history = historyFor(groupID)
    if (history.loading) return
    history.loading = true
    try {
      const page = await groupApi.history(groupID, undefined, 20)
      messagesByGroup.value[String(groupID)] = mergeGroupMessages(messagesFor(groupID), page.list, currentUserID.value)
      history.loaded = true
      history.hasMore = page.hasMore
      history.nextBeforeMessageID = page.nextBeforeMessageID
      lastError.value = ''
    } catch (error) {
      handleRequestFailure(error, '群历史加载失败')
    } finally {
      history.loading = false
    }
  }

  async function loadOlder(groupID = currentGroupID.value): Promise<void> {
    if (!groupID) return
    const history = historyFor(groupID)
    if (history.loading || !history.loaded || !history.hasMore || !history.nextBeforeMessageID) return
    history.loading = true
    try {
      const page = await groupApi.history(groupID, history.nextBeforeMessageID, 20)
      messagesByGroup.value[String(groupID)] = mergeGroupMessages(messagesFor(groupID), page.list, currentUserID.value)
      history.hasMore = page.hasMore
      history.nextBeforeMessageID = page.nextBeforeMessageID
      lastError.value = ''
    } catch (error) {
      handleRequestFailure(error, '更早群消息加载失败')
    } finally {
      history.loading = false
    }
  }

  function sendText(content: string): string | null {
    const groupID = currentGroupID.value
    const text = content.trim()
    if (!groupID || !currentUserID.value || !text || currentUnavailable.value) return null
    const clientMsgId = newClientMsgID()
    messagesFor(groupID).push({
      localID: `client:${clientMsgId}`,
      groupID,
      sendUserID: currentUserID.value,
      clientMsgId,
      msg: { type: 1, textMsg: { content: text } },
      createdAt: new Date().toISOString(),
      status: 'pending',
      retryable: true,
    })
    sendPending(clientMsgId)
    return clientMsgId
  }

  function retryMessage(clientMsgId: string): void {
    const message = findByClientMsgID(clientMsgId)
    if (!message || message.sendUserID !== currentUserID.value || message.retryable === false) return
    message.status = 'pending'
    message.errorMessage = undefined
    sendPending(clientMsgId)
  }

  function sendPending(clientMsgId: string): void {
    const message = findByClientMsgID(clientMsgId)
    if (!message || message.status !== 'pending' || !socket) return
    const sent = socket.send({
      event: 'group.send',
      clientMsgId: message.clientMsgId,
      data: { groupID: message.groupID, msg: message.msg },
    })
    if (!sent) return
    clearAckTimer(clientMsgId)
    ackTimers.set(clientMsgId, setTimeout(() => {
      ackTimers.delete(clientMsgId)
      const pending = findByClientMsgID(clientMsgId)
      if (!pending || pending.status !== 'pending') return
      pending.status = 'failed'
      pending.errorMessage = '服务端确认超时，可使用同一消息标识重试'
      pending.retryable = true
    }, ACK_TIMEOUT_MS))
  }

  function flushPending(): void {
    for (const messages of Object.values(messagesByGroup.value)) {
      for (const message of messages) if (message.status === 'pending') sendPending(message.clientMsgId)
    }
  }

  function clearAckTimers(): void {
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

  function findByClientMsgID(clientMsgId: string): GroupMessageViewItem | undefined {
    for (const messages of Object.values(messagesByGroup.value)) {
      const found = messages.find((message) => message.clientMsgId === clientMsgId)
      if (found) return found
    }
    return undefined
  }

  function handleEnvelope(envelope: GroupServerEnvelope): void {
    if (envelope.event === 'group.ack') {
      const pending = findByClientMsgID(envelope.clientMsgId)
      if (!pending || pending.groupID !== envelope.data.groupID) return
      clearAckTimer(envelope.clientMsgId)
      pending.messageId = envelope.data.messageId
      pending.createdAt = envelope.data.createdAt
      pending.status = 'sent'
      pending.errorMessage = undefined
      pending.retryable = undefined
      updateLocalSession({
        messageId: envelope.data.messageId,
        groupID: pending.groupID,
        sendUserID: pending.sendUserID,
        clientMsgId: pending.clientMsgId,
        msg: pending.msg,
        createdAt: envelope.data.createdAt,
      }, false)
      scheduleReconcile()
      return
    }
    if (envelope.event === 'error') {
      if (!envelope.clientMsgId) return
      const pending = findByClientMsgID(envelope.clientMsgId)
      if (!pending) return
      clearAckTimer(envelope.clientMsgId)
      pending.status = 'failed'
      pending.errorMessage = safeSocketError(envelope.error.code)
      pending.retryable = !NON_RETRYABLE_ERROR_CODES.has(envelope.error.code)
      lastError.value = pending.errorMessage
      if (MEMBERSHIP_ERROR_CODES.has(envelope.error.code)) {
        if (currentGroupID.value === pending.groupID) currentUnavailable.value = true
        void reconcileGroupLists()
      }
      return
    }
    receiveMessage(envelope.data)
  }

  function receiveMessage(message: GroupMessage): void {
    const existing = messagesFor(message.groupID)
    const alreadyPresent = existing.some((item) => item.messageId === message.messageId)
    messagesByGroup.value[String(message.groupID)] = mergeGroupMessages(existing, [message], currentUserID.value)
    if (alreadyPresent) return
    clearAckTimer(message.clientMsgId)
    const shouldRead = currentGroupID.value === message.groupID && groupActive
    updateLocalSession(message, message.sendUserID !== currentUserID.value && !shouldRead)
    scheduleReconcile()
    if (shouldRead) void markGroupRead(message.groupID)
  }

  function updateLocalSession(message: GroupMessage, incrementUnread: boolean): void {
    const session = sessions.value.find((item) => item.group.groupID === message.groupID)
    if (!session) {
      scheduleReconcile()
      return
    }
    if (message.messageId >= session.lastMessageID) {
      session.lastMessage = message
      session.lastMessageID = message.messageId
      session.lastMessageAt = message.createdAt
    }
    if (incrementUnread) session.unreadCount += 1
    sessions.value = [...sessions.value].sort((left, right) => {
      if (left.isTop !== right.isTop) return left.isTop ? -1 : 1
      const time = right.lastMessageAt.localeCompare(left.lastMessageAt)
      return time || right.lastMessageID - left.lastMessageID
    })
  }

  async function markGroupRead(groupID = currentGroupID.value): Promise<void> {
    if (!groupID) return
    const target = latestGroupMessageID(messagesFor(groupID))
    if (!target) return
    readRequestedThrough.set(groupID, Math.max(readRequestedThrough.get(groupID) ?? 0, target))
    if (readInFlight.has(groupID)) return
    readInFlight.add(groupID)
    try {
      let completed = 0
      while ((readRequestedThrough.get(groupID) ?? 0) > completed) {
        const requested = readRequestedThrough.get(groupID)!
        const updated = await groupApi.markRead(groupID, requested)
        completed = requested
        const session = sessions.value.find((item) => item.group.groupID === groupID)
        if (session) {
          session.unreadCount = updated.unreadCount
          session.lastReadMessageID = updated.lastReadMessageID
        }
      }
      lastError.value = ''
    } catch (error) {
      handleRequestFailure(error, '群已读状态更新失败')
    } finally {
      readInFlight.delete(groupID)
      readRequestedThrough.delete(groupID)
    }
  }

  function setGroupActive(active: boolean): void {
    groupActive = active
    if (active) void markGroupRead()
  }

  async function createGroup(input: CreateGroupInput): Promise<GroupSummary | null> {
    try {
      const created = await groupApi.create(input)
      await refreshGroups()
      return created
    } catch (error) {
      handleRequestFailure(error, '创建群聊失败')
      return null
    }
  }

  async function setMemberMute(memberID: number, muted: boolean): Promise<boolean> {
    const groupID = currentGroupID.value
    if (!groupID) return false
    try {
      await groupApi.setMemberMute(groupID, memberID, muted ? 5 : undefined)
      lastError.value = muted ? '成员已禁言 5 分钟' : '成员已解除禁言'
      return true
    } catch (error) {
      handleRequestFailure(error, muted ? '成员禁言失败' : '解除禁言失败')
      return false
    }
  }

  function senderName(userID: number): string {
    if (userID === currentUserID.value) return '我'
    const member = memberByUserID.value.get(userID)
    return member?.memberNickname || member?.userNickname || `成员 ${userID}`
  }

  return {
    sessions,
    groups,
    sessionsLoading,
    groupsLoading,
    currentGroupID,
    currentGroup,
    currentMembers,
    messagesByGroup,
    historyByGroup,
    connectionState,
    currentUserID,
    lastError,
    currentUnavailable,
    currentMessages,
    currentSession,
    currentHistory,
    start,
    stop,
    refreshSessions,
    refreshGroups,
    reconcileGroupLists,
    openGroup,
    closeGroup,
    loadLatest,
    loadOlder,
    sendText,
    retryMessage,
    markGroupRead,
    setGroupActive,
    createGroup,
    setMemberMute,
    senderName,
  }
})
