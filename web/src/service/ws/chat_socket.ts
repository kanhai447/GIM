import type {
  ChatAckEnvelope,
  ChatConnectionState,
  ChatErrorEnvelope,
  ChatMessageEnvelope,
  ChatSendEnvelope,
  ChatServerEnvelope,
  PrivateChatMessage,
} from '@/types/chat'

const OPEN = 1
const AUTH_CLOSE_CODES = new Set([1008, 4001, 4003, 4401, 4403])

interface SocketLike {
  readonly readyState: number
  onopen: ((event: Event) => void) | null
  onmessage: ((event: MessageEvent) => void) | null
  onclose: ((event: CloseEvent) => void) | null
  onerror: ((event: Event) => void) | null
  send(data: string): void
  close(code?: number, reason?: string): void
}

export interface ChatSocketOptions {
  getToken: () => string
  onStateChange: (state: ChatConnectionState) => void
  onEnvelope: (envelope: ChatServerEnvelope) => void
  onAuthenticationFailure: () => void
  configuredBaseURL?: string
  socketFactory?: (url: string) => SocketLike
  random?: () => number
  setTimer?: (handler: () => void, timeout: number) => ReturnType<typeof setTimeout>
  clearTimer?: (timer: ReturnType<typeof setTimeout>) => void
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null
}

function isPositiveNumber(value: unknown): value is number {
  return typeof value === 'number' && Number.isSafeInteger(value) && value > 0
}

function isMessage(value: unknown): value is PrivateChatMessage {
  if (!isRecord(value) || !isPositiveNumber(value.messageId) || !isPositiveNumber(value.sendUserID) || !isPositiveNumber(value.revUserID)) return false
  if (typeof value.clientMsgId !== 'string' || typeof value.createdAt !== 'string' || !isRecord(value.msg)) return false
  const text = value.msg.textMsg
  return value.msg.type === 1 && isRecord(text) && typeof text.content === 'string'
}

export function parseChatServerEnvelope(raw: string): ChatServerEnvelope | null {
  let value: unknown
  try {
    value = JSON.parse(raw)
  } catch {
    return null
  }
  if (!isRecord(value) || typeof value.event !== 'string') return null

  if (value.event === 'chat.ack' && typeof value.clientMsgId === 'string' && isRecord(value.data)) {
    if (!isPositiveNumber(value.data.messageId) || typeof value.data.createdAt !== 'string') return null
    return value as unknown as ChatAckEnvelope
  }
  if (value.event === 'chat.message' && isMessage(value.data)) {
    return value as unknown as ChatMessageEnvelope
  }
  if (value.event === 'error' && isRecord(value.error) && typeof value.error.code === 'number' && typeof value.error.message === 'string') {
    if (value.clientMsgId !== undefined && typeof value.clientMsgId !== 'string') return null
    return value as unknown as ChatErrorEnvelope
  }
  return null
}

export function chatSocketURL(
  token: string,
  configuredBaseURL = import.meta.env.VITE_WS_BASE_URL || import.meta.env.VITE_API_BASE_URL,
): string {
  const configured = configuredBaseURL?.trim()
  let base: URL
  if (configured) {
    const fallbackOrigin = typeof window === 'undefined' ? 'http://localhost' : window.location.origin
    base = new URL(configured, fallbackOrigin)
  } else if (typeof window !== 'undefined') {
    base = new URL(window.location.origin)
  } else {
    base = new URL('http://localhost')
  }
  base.protocol = base.protocol === 'https:' || base.protocol === 'wss:' ? 'wss:' : 'ws:'
  base.pathname = '/api/chat/ws/chat'
  base.search = ''
  base.searchParams.set('token', token)
  return base.toString()
}

export class ChatSocketClient {
  private socket: SocketLike | null = null
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null
  private reconnectAttempt = 0
  private active = false
  private state: ChatConnectionState = 'disconnected'
  private readonly createSocket: (url: string) => SocketLike
  private readonly random: () => number
  private readonly setTimer: NonNullable<ChatSocketOptions['setTimer']>
  private readonly clearTimer: NonNullable<ChatSocketOptions['clearTimer']>

  constructor(private readonly options: ChatSocketOptions) {
    this.createSocket = options.socketFactory ?? ((url) => new WebSocket(url))
    this.random = options.random ?? Math.random
    this.setTimer = options.setTimer ?? ((handler, timeout) => setTimeout(handler, timeout))
    this.clearTimer = options.clearTimer ?? ((timer) => clearTimeout(timer))
  }

  connect(): void {
    this.active = true
    if (this.socket || this.reconnectTimer) return
    const token = this.options.getToken().trim()
    if (!token) {
      this.active = false
      this.setState('disconnected')
      return
    }
    this.open(false, token)
  }

  disconnect(): void {
    this.active = false
    this.reconnectAttempt = 0
    this.clearReconnectTimer()
    const socket = this.socket
    this.socket = null
    if (socket) socket.close(1000, 'client logout')
    this.setState('disconnected')
  }

  failAuthentication(): void {
    this.active = false
    this.clearReconnectTimer()
    const socket = this.socket
    this.socket = null
    if (socket) socket.close(1000, 'authentication required')
    this.setState('disconnected')
    this.options.onAuthenticationFailure()
  }

  send(envelope: ChatSendEnvelope): boolean {
    if (this.state !== 'connected' || !this.socket || this.socket.readyState !== OPEN) return false
    this.socket.send(JSON.stringify(envelope))
    return true
  }

  private open(reconnecting: boolean, token: string): void {
    this.setState(reconnecting ? 'reconnecting' : 'connecting')
    let socket: SocketLike
    try {
      socket = this.createSocket(chatSocketURL(token, this.options.configuredBaseURL))
    } catch {
      this.scheduleReconnect()
      return
    }
    this.socket = socket
    socket.onopen = () => {
      if (this.socket !== socket || !this.active) return
      this.reconnectAttempt = 0
      this.clearReconnectTimer()
      this.setState('connected')
    }
    socket.onmessage = (event) => {
      if (this.socket !== socket || typeof event.data !== 'string') return
      const envelope = parseChatServerEnvelope(event.data)
      if (envelope) this.options.onEnvelope(envelope)
    }
    socket.onerror = () => {
      // Browsers intentionally hide WebSocket handshake details. onclose owns recovery.
    }
    socket.onclose = (event) => {
      if (this.socket !== socket) return
      this.socket = null
      if (!this.active) {
        this.setState('disconnected')
        return
      }
      if (AUTH_CLOSE_CODES.has(event.code)) {
        this.failAuthentication()
        return
      }
      this.scheduleReconnect()
    }
  }

  private scheduleReconnect(): void {
    if (!this.active || this.reconnectTimer) return
    this.setState('reconnecting')
    const baseDelay = Math.min(1_000 * (2 ** this.reconnectAttempt), 30_000)
    const delay = Math.round(baseDelay * (1 + this.random() * 0.2))
    this.reconnectAttempt += 1
    this.reconnectTimer = this.setTimer(() => {
      this.reconnectTimer = null
      if (!this.active) return
      const token = this.options.getToken().trim()
      if (!token) {
        this.failAuthentication()
        return
      }
      this.open(true, token)
    }, delay)
  }

  private clearReconnectTimer(): void {
    if (!this.reconnectTimer) return
    this.clearTimer(this.reconnectTimer)
    this.reconnectTimer = null
  }

  private setState(state: ChatConnectionState): void {
    if (this.state === state) return
    this.state = state
    this.options.onStateChange(state)
  }
}
