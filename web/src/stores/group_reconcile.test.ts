import { describe, expect, it } from 'vitest'

import { latestGroupMessageID, mergeGroupMessages } from '@/stores/group_reconcile'
import type { GroupMessage, GroupMessageViewItem } from '@/types/group'

const message: GroupMessage = {
  messageId: 10,
  groupID: 5,
  sendUserID: 1,
  clientMsgId: 'client-10',
  msg: { type: 1, textMsg: { content: 'hello' } },
  createdAt: '2026-10-13T00:00:00Z',
}

describe('group message reconciliation', () => {
  it('merges a realtime message into its pending item by clientMsgId', () => {
    const pending: GroupMessageViewItem = { ...message, messageId: undefined, localID: 'pending', status: 'pending' }
    const merged = mergeGroupMessages([pending], [message], 1)
    expect(merged).toHaveLength(1)
    expect(merged[0]).toMatchObject({ localID: 'pending', messageId: 10, status: 'sent' })
  })

  it('deduplicates by server message ID', () => {
    const first = mergeGroupMessages([], [message], 1)
    expect(mergeGroupMessages(first, [message], 1)).toHaveLength(1)
  })

  it('does not merge another sender message by clientMsgId', () => {
    const pending: GroupMessageViewItem = { ...message, messageId: undefined, localID: 'pending', status: 'pending' }
    const remote = { ...message, messageId: 11, sendUserID: 2 }
    expect(mergeGroupMessages([pending], [remote], 1)).toHaveLength(2)
  })

  it('sorts persisted messages before pending messages', () => {
    const pending: GroupMessageViewItem = { ...message, messageId: undefined, localID: 'pending', status: 'pending' }
    const merged = mergeGroupMessages([pending], [{ ...message, messageId: 9, clientMsgId: 'other' }], 1)
    expect(merged.map((item) => item.messageId)).toEqual([9, undefined])
  })

  it('returns the latest persisted message ID and ignores pending', () => {
    const pending: GroupMessageViewItem = { ...message, messageId: undefined, localID: 'pending', status: 'pending' }
    const sent = mergeGroupMessages([], [message, { ...message, messageId: 12, clientMsgId: 'c12' }], 1)
    expect(latestGroupMessageID([...sent, pending])).toBe(12)
  })
})
