import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { GroupSocketClient, groupSocketURL, parseGroupServerEnvelope } from '@/service/ws/group_socket'
import type { GroupConnectionState, GroupServerEnvelope } from '@/types/group'

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

function setup(token = 'test-token') {
  const sockets: FakeSocket[] = []
  const states: GroupConnectionState[] = []
  const envelopes: GroupServerEnvelope[] = []
  const authFailure = vi.fn()
  const client = new GroupSocketClient({
    getToken: () => token,
    onStateChange: (state) => states.push(state),
    onEnvelope: (envelope) => envelopes.push(envelope),
    onAuthenticationFailure: authFailure,
    configuredBaseURL: 'https://group.example.test/base',
    socketFactory: () => {
      const socket = new FakeSocket()
      sockets.push(socket)
      return socket
    },
    random: () => 0,
  })
  return { client, sockets, states, envelopes, authFailure }
}

describe('GroupSocketClient', () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => vi.useRealTimers())

  it('connects only one user-level socket', () => {
    const { client, sockets, states } = setup()
    client.connect()
    client.connect()
    expect(sockets).toHaveLength(1)
    expect(states).toEqual(['connecting'])
    sockets[0]!.open()
    expect(states.at(-1)).toBe('connected')
  })

  it('uses one bounded exponential reconnect timer', () => {
    const { client, sockets } = setup()
    client.connect()
    sockets[0]!.open()
    sockets[0]!.closed()
    sockets[0]!.closed()
    vi.advanceTimersByTime(999)
    expect(sockets).toHaveLength(1)
    vi.advanceTimersByTime(1)
    expect(sockets).toHaveLength(2)
    sockets[1]!.closed()
    vi.advanceTimersByTime(2_000)
    expect(sockets).toHaveLength(3)
  })

  it('caps reconnect delay at 30 seconds', () => {
    const { client, sockets } = setup()
    client.connect()
    for (const delay of [1_000, 2_000, 4_000, 8_000, 16_000, 30_000, 30_000]) {
      sockets.at(-1)!.closed()
      vi.advanceTimersByTime(delay)
    }
    expect(sockets).toHaveLength(8)
  })

  it('resets backoff after successful reconnect', () => {
    const { client, sockets } = setup()
    client.connect()
    sockets[0]!.closed()
    vi.advanceTimersByTime(1_000)
    sockets[1]!.closed()
    vi.advanceTimersByTime(2_000)
    sockets[2]!.open()
    sockets[2]!.closed()
    vi.advanceTimersByTime(1_000)
    expect(sockets).toHaveLength(4)
  })

  it('disconnect cancels reconnect permanently', () => {
    const { client, sockets, states } = setup()
    client.connect()
    sockets[0]!.open()
    sockets[0]!.closed()
    client.disconnect()
    vi.advanceTimersByTime(60_000)
    expect(sockets).toHaveLength(1)
    expect(states.at(-1)).toBe('disconnected')
  })

  it('does not connect without a token', () => {
    const { client, sockets } = setup('   ')
    client.connect()
    expect(sockets).toHaveLength(0)
  })

  it('stops reconnect on an authentication close code', () => {
    const { client, sockets, authFailure } = setup()
    client.connect()
    sockets[0]!.open()
    sockets[0]!.closed(4401)
    vi.advanceTimersByTime(60_000)
    expect(authFailure).toHaveBeenCalledOnce()
    expect(sockets).toHaveLength(1)
  })

  it('sends only while connected', () => {
    const { client, sockets } = setup()
    client.connect()
    const envelope = { event: 'group.send' as const, clientMsgId: 'client-1', data: { groupID: 9, msg: { type: 1 as const, textMsg: { content: 'hello' } } } }
    expect(client.send(envelope)).toBe(false)
    sockets[0]!.open()
    expect(client.send(envelope)).toBe(true)
    expect(JSON.parse(sockets[0]!.sent[0]!)).toEqual(envelope)
  })

  it('dispatches typed envelopes and ignores unknown or malformed frames', () => {
    const { client, sockets, envelopes } = setup()
    client.connect()
    sockets[0]!.open()
    sockets[0]!.message('{bad')
    sockets[0]!.message(JSON.stringify({ event: 'unknown', data: {} }))
    sockets[0]!.message(JSON.stringify({ event: 'group.ack', clientMsgId: 'c1', data: { messageId: 7, groupID: 9, createdAt: 'now' } }))
    expect(envelopes).toHaveLength(1)
    expect(envelopes[0]?.event).toBe('group.ack')
  })
})

describe('group protocol parsing', () => {
  it('builds ws/wss URL without retaining a configured path or query', () => {
    expect(groupSocketURL('secret', 'https://example.test/base?old=1')).toBe('wss://example.test/api/group/ws/chat?token=secret')
    expect(groupSocketURL('secret', 'http://example.test')).toBe('ws://example.test/api/group/ws/chat?token=secret')
  })

  it('parses ack, message and error variants', () => {
    expect(parseGroupServerEnvelope(JSON.stringify({ event: 'group.ack', clientMsgId: 'c1', data: { messageId: 1, groupID: 2, createdAt: 'now' } }))?.event).toBe('group.ack')
    expect(parseGroupServerEnvelope(JSON.stringify({ event: 'group.message', data: { messageId: 1, groupID: 2, sendUserID: 3, clientMsgId: 'c1', msg: { type: 1, textMsg: { content: 'hi' } }, createdAt: 'now' } }))?.event).toBe('group.message')
    expect(parseGroupServerEnvelope(JSON.stringify({ event: 'error', clientMsgId: 'c1', error: { code: 2405, message: 'muted' } }))?.event).toBe('error')
  })

  it('rejects invalid numeric and message fields', () => {
    expect(parseGroupServerEnvelope(JSON.stringify({ event: 'group.ack', clientMsgId: 'c1', data: { messageId: 0, groupID: 2, createdAt: 'now' } }))).toBeNull()
    expect(parseGroupServerEnvelope(JSON.stringify({ event: 'group.message', data: { messageId: 1, groupID: 2, sendUserID: 3, clientMsgId: 'c1', msg: { type: 2 }, createdAt: 'now' } }))).toBeNull()
  })
})
