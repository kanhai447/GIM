import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { ChatSocketClient, chatSocketURL, parseChatServerEnvelope } from '@/service/ws/chat_socket'
import type { ChatConnectionState, ChatServerEnvelope } from '@/types/chat'

class FakeSocket {
  readyState = 0
  onopen: ((event: Event) => void) | null = null
  onmessage: ((event: MessageEvent) => void) | null = null
  onclose: ((event: CloseEvent) => void) | null = null
  onerror: ((event: Event) => void) | null = null
  readonly sent: string[] = []
  close = vi.fn(() => { this.readyState = 3 })

  send(data: string): void { this.sent.push(data) }
  open(): void { this.readyState = 1; this.onopen?.(new Event('open')) }
  message(data: string): void { this.onmessage?.({ data } as MessageEvent) }
  closed(code = 1006): void { this.readyState = 3; this.onclose?.({ code } as CloseEvent) }
}

function setup() {
  const sockets: FakeSocket[] = []
  const states: ChatConnectionState[] = []
  const envelopes: ChatServerEnvelope[] = []
  const authFailure = vi.fn()
  const client = new ChatSocketClient({
    getToken: () => 'test-token',
    onStateChange: (state) => states.push(state),
    onEnvelope: (envelope) => envelopes.push(envelope),
    onAuthenticationFailure: authFailure,
    configuredBaseURL: 'https://chat.example.test/base',
    socketFactory: () => {
      const socket = new FakeSocket()
      sockets.push(socket)
      return socket
    },
    random: () => 0,
  })
  return { client, sockets, states, envelopes, authFailure }
}

describe('ChatSocketClient', () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => vi.useRealTimers())

  it('connects once and exposes connecting then connected state', () => {
    const { client, sockets, states } = setup()
    client.connect()
    client.connect()
    expect(sockets).toHaveLength(1)
    expect(states).toEqual(['connecting'])
    sockets[0]!.open()
    expect(states).toEqual(['connecting', 'connected'])
  })

  it('uses bounded exponential reconnect delay with one active timer', () => {
    const { client, sockets, states } = setup()
    client.connect()
    sockets[0]!.open()
    sockets[0]!.closed()
    sockets[0]!.closed()
    expect(states.at(-1)).toBe('reconnecting')
    vi.advanceTimersByTime(999)
    expect(sockets).toHaveLength(1)
    vi.advanceTimersByTime(1)
    expect(sockets).toHaveLength(2)

    sockets[1]!.closed()
    vi.advanceTimersByTime(1_999)
    expect(sockets).toHaveLength(2)
    vi.advanceTimersByTime(1)
    expect(sockets).toHaveLength(3)
  })

  it('caps retry delay at 30 seconds', () => {
    const { client, sockets } = setup()
    client.connect()
    for (const delay of [1_000, 2_000, 4_000, 8_000, 16_000, 30_000, 30_000]) {
      sockets.at(-1)!.closed()
      vi.advanceTimersByTime(delay - 1)
      const before = sockets.length
      vi.advanceTimersByTime(1)
      expect(sockets.length).toBe(before + 1)
    }
  })

  it('disconnect cancels reconnect and prevents a reconnect storm', () => {
    const { client, sockets, states } = setup()
    client.connect()
    sockets[0]!.open()
    sockets[0]!.closed()
    client.disconnect()
    vi.advanceTimersByTime(60_000)
    expect(sockets).toHaveLength(1)
    expect(states.at(-1)).toBe('disconnected')
  })

  it('authentication close stops reconnect and reports failure', () => {
    const { client, sockets, authFailure, states } = setup()
    client.connect()
    sockets[0]!.open()
    sockets[0]!.closed(4401)
    vi.advanceTimersByTime(60_000)
    expect(authFailure).toHaveBeenCalledOnce()
    expect(sockets).toHaveLength(1)
    expect(states.at(-1)).toBe('disconnected')
  })

  it('serializes chat.send only while open', () => {
    const { client, sockets } = setup()
    client.connect()
    const frame = { event: 'chat.send' as const, clientMsgId: 'client-1', data: { revUserID: 2, msg: { type: 1 as const, textMsg: { content: 'hello' } } } }
    expect(client.send(frame)).toBe(false)
    sockets[0]!.open()
    expect(client.send(frame)).toBe(true)
    expect(JSON.parse(sockets[0]!.sent[0]!)).toEqual(frame)
  })

  it('parses typed server events and ignores malformed or unknown frames', () => {
    const { client, sockets, envelopes } = setup()
    client.connect()
    sockets[0]!.open()
    sockets[0]!.message('{bad json')
    sockets[0]!.message(JSON.stringify({ event: 'unknown', data: {} }))
    sockets[0]!.message(JSON.stringify({ event: 'chat.ack', clientMsgId: 'c1', data: { messageId: 9, createdAt: '2026-10-11T00:00:00Z' } }))
    expect(envelopes).toHaveLength(1)
    expect(envelopes[0]?.event).toBe('chat.ack')
  })
})

describe('chatSocketURL', () => {
  it('converts HTTP schemes and safely encodes the query token', () => {
    expect(chatSocketURL('a+b/c', 'http://localhost:8080')).toBe('ws://localhost:8080/api/chat/ws/chat?token=a%2Bb%2Fc')
    expect(chatSocketURL('token', 'https://gateway.example.test/path')).toBe('wss://gateway.example.test/api/chat/ws/chat?token=token')
  })

  it('rejects malformed protocol payloads without throwing', () => {
    expect(parseChatServerEnvelope(JSON.stringify({ event: 'chat.message', data: { messageId: 1 } }))).toBeNull()
  })
})
