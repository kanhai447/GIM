import { describe, expect, it } from 'vitest'

import { applyWithdrawals, mergeServerMessages } from '@/stores/chat_reconcile'
import type { ChatMessageItem, PrivateChatMessage } from '@/types/chat'

const persisted: PrivateChatMessage = {
  messageId: 10,
  sendUserID: 1,
  revUserID: 2,
  clientMsgId: 'client-10',
  msg: { type: 1, textMsg: { content: 'hello' } },
  createdAt: '2026-10-11T00:00:00Z',
}

describe('chat message reconciliation', () => {
  it('merges HTTP and realtime copies by server message ID', () => {
    const once = mergeServerMessages([], [persisted], 1)
    expect(mergeServerMessages(once, [persisted], 1)).toHaveLength(1)
  })

  it('replaces a local pending sender item by clientMsgId', () => {
    const pending: ChatMessageItem = {
      localID: 'client:client-10',
      sendUserID: 1,
      revUserID: 2,
      clientMsgId: 'client-10',
      msg: persisted.msg,
      createdAt: persisted.createdAt,
      status: 'pending',
    }
    const merged = mergeServerMessages([pending], [persisted], 1)
    expect(merged).toHaveLength(1)
    expect(merged[0]).toMatchObject({ localID: pending.localID, messageId: 10, status: 'sent' })
  })

  it('keeps message-ID order while leaving local pending at the end', () => {
    const newer = { ...persisted, messageId: 12, clientMsgId: 'client-12' }
    const pending: ChatMessageItem = { ...mergeServerMessages([], [persisted], 1)[0]!, localID: 'pending', messageId: undefined, clientMsgId: 'pending', status: 'pending' }
    expect(mergeServerMessages([pending], [newer, persisted], 1).map((item) => item.messageId)).toEqual([10, 12, undefined])
  })

  it('collapses duplicate withdraw messages targeting the same original', () => {
    const withdraw = (messageId: number): PrivateChatMessage => ({
      messageId,
      sendUserID: 1,
      revUserID: 2,
      clientMsgId: `withdraw-${messageId}`,
      msg: { type: 4, withdrawMsg: { msgID: 10 } },
      createdAt: '2026-11-14T12:00:00Z',
    })
    const merged = applyWithdrawals(mergeServerMessages([], [persisted, withdraw(20), withdraw(21)], 1))
    expect(merged).toHaveLength(1)
    expect(merged[0]?.messageId).toBe(10)
    expect(merged[0]?.msg).toEqual({ type: 4, withdrawMsg: { msgID: 10 } })
  })
})
