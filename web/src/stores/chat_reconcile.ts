import type { ChatMessageItem, PrivateChatMessage } from '@/types/chat'
import { messagePreviewText } from '@/utils/message'

export function peerIDForMessage(message: Pick<PrivateChatMessage, 'sendUserID' | 'revUserID'>, currentUserID: number): number {
  return message.sendUserID === currentUserID ? message.revUserID : message.sendUserID
}

export function toSentMessage(message: PrivateChatMessage): ChatMessageItem {
  return {
    localID: `message:${message.messageId}`,
    messageId: message.messageId,
    sendUserID: message.sendUserID,
    revUserID: message.revUserID,
    clientMsgId: message.clientMsgId,
    msg: message.msg,
    createdAt: message.createdAt,
    status: 'sent',
  }
}

export function mergeServerMessages(
  existing: ChatMessageItem[],
  incoming: PrivateChatMessage[],
  currentUserID: number,
): ChatMessageItem[] {
  const result = [...existing]
  for (const message of incoming) {
    const byServerID = result.findIndex((item) => item.messageId === message.messageId)
    if (byServerID >= 0) continue

    const pendingIndex = message.sendUserID === currentUserID
      ? result.findIndex((item) => item.clientMsgId === message.clientMsgId && item.sendUserID === currentUserID)
      : -1
    if (pendingIndex >= 0) {
      result[pendingIndex] = { ...toSentMessage(message), localID: result[pendingIndex]!.localID }
      continue
    }
    result.push(toSentMessage(message))
  }

  return result.sort((left, right) => {
    if (left.messageId !== undefined && right.messageId !== undefined) return left.messageId - right.messageId
    if (left.messageId !== undefined) return -1
    if (right.messageId !== undefined) return 1
    return left.createdAt.localeCompare(right.createdAt)
  })
}

export function latestServerMessageID(messages: ChatMessageItem[]): number | undefined {
  let latest: number | undefined
  for (const message of messages) {
    if (message.messageId !== undefined && (latest === undefined || message.messageId > latest)) {
      latest = message.messageId
    }
  }
  return latest
}

export function messagePreview(message: Pick<PrivateChatMessage, 'msg'>): string {
  return messagePreviewText(message)
}

/**
 * Replaces recalled originals with the withdraw placeholder and hides the
 * separate withdraw row when its target is present in the same list.
 */
export function applyWithdrawals(items: ChatMessageItem[]): ChatMessageItem[] {
  const byID = new Map<number, ChatMessageItem>()
  for (const item of items) {
    if (item.messageId !== undefined) byID.set(item.messageId, item)
  }
  const removed = new Set<number>()
  for (const item of items) {
    if (item.messageId === undefined || item.msg.type !== 4) continue
    const targetID = item.msg.withdrawMsg.msgID
    const target = byID.get(targetID)
    if (!target || target === item) continue
    target.msg = { type: 4, withdrawMsg: { msgID: targetID } }
    target.status = 'sent'
    removed.add(item.messageId)
  }
  return removed.size ? items.filter((item) => item.messageId === undefined || !removed.has(item.messageId)) : items
}
