export type GroupConnectionState = 'disconnected' | 'connecting' | 'connected' | 'reconnecting'

export interface GroupTextPayload {
  type: 1
  textMsg: {
    content: string
  }
}

export interface GroupMessage {
  messageId: number
  groupID: number
  sendUserID: number
  clientMsgId: string
  msg: GroupTextPayload
  createdAt: string
}

export interface GroupSummary {
  groupId: number
  title: string
  abstract: string
  avatar: string
  ownerId: number
  status: 1 | 2
  role: 1 | 2 | 3
  memberCount: number
  isSearch: boolean
  joinPolicy: number
  isProhibition: boolean
  memberLimit: number
}

export interface GroupList {
  list: GroupSummary[]
  count: number
}

export interface GroupMember {
  userId: number
  userNickname: string
  avatar: string
  role: 1 | 2 | 3
  memberNickname: string
  createdAt: string
}

export interface GroupMemberList {
  list: GroupMember[]
  count: number
}

export interface GroupSession {
  group: {
    groupID: number
    name: string
    avatar: string
  }
  lastMessage?: GroupMessage
  lastMessageID: number
  lastMessageAt: string
  lastReadMessageID: number | null
  unreadCount: number
  isTop: boolean
}

export interface GroupSessionList {
  list: GroupSession[]
  count: number
}

export interface GroupHistoryPage {
  list: GroupMessage[]
  hasMore: boolean
  nextBeforeMessageID?: number
}

export interface GroupSendEnvelope {
  event: 'group.send'
  clientMsgId: string
  data: {
    groupID: number
    msg: GroupTextPayload
  }
}

export interface GroupAckEnvelope {
  event: 'group.ack'
  clientMsgId: string
  data: {
    messageId: number
    groupID: number
    createdAt: string
  }
}

export interface GroupMessageEnvelope {
  event: 'group.message'
  clientMsgId?: string
  data: GroupMessage
}

export interface GroupErrorEnvelope {
  event: 'error'
  clientMsgId?: string
  error: {
    code: number
    message: string
  }
}

export type GroupServerEnvelope = GroupAckEnvelope | GroupMessageEnvelope | GroupErrorEnvelope
export type GroupLocalMessageStatus = 'pending' | 'sent' | 'failed'

export interface GroupMessageItem extends GroupMessage {
  localID: string
  messageId: number
  status: GroupLocalMessageStatus
  errorMessage?: string
  retryable?: boolean
}

export interface GroupPendingMessageItem extends Omit<GroupMessageItem, 'messageId'> {
  messageId?: number
}

export type GroupMessageViewItem = GroupMessageItem | GroupPendingMessageItem

export interface GroupHistoryState {
  loaded: boolean
  loading: boolean
  hasMore: boolean
  nextBeforeMessageID?: number
}

export interface CreateGroupInput {
  name: string
  userIdList: number[]
  size?: number
  isSearch?: boolean
}
