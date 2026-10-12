import type { GroupMessage, GroupMessageViewItem } from '@/types/group'

export function toSentGroupMessage(message: GroupMessage, localID = `message:${message.messageId}`): GroupMessageViewItem {
  return { ...message, localID, status: 'sent' }
}

export function mergeGroupMessages(
  existing: GroupMessageViewItem[],
  incoming: GroupMessage[],
  currentUserID: number,
): GroupMessageViewItem[] {
  const result = [...existing]
  for (const message of incoming) {
    const serverIndex = result.findIndex((item) => item.messageId === message.messageId)
    if (serverIndex >= 0) {
      const prior = result[serverIndex]!
      result[serverIndex] = toSentGroupMessage(message, prior.localID)
      continue
    }
    const pendingIndex = message.sendUserID === currentUserID
      ? result.findIndex((item) => item.clientMsgId === message.clientMsgId && item.sendUserID === currentUserID)
      : -1
    if (pendingIndex >= 0) {
      result[pendingIndex] = toSentGroupMessage(message, result[pendingIndex]!.localID)
      continue
    }
    result.push(toSentGroupMessage(message))
  }
  return result.sort((left, right) => {
    if (left.messageId !== undefined && right.messageId !== undefined) return left.messageId - right.messageId
    if (left.messageId !== undefined) return -1
    if (right.messageId !== undefined) return 1
    return left.createdAt.localeCompare(right.createdAt)
  })
}

export function latestGroupMessageID(messages: GroupMessageViewItem[]): number | undefined {
  let latest: number | undefined
  for (const message of messages) {
    if (message.messageId !== undefined && (latest === undefined || message.messageId > latest)) latest = message.messageId
  }
  return latest
}
