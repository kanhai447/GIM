export type ChatConnectionState = 'disconnected' | 'connecting' | 'connected' | 'reconnecting'

export interface TextMessagePayload {
  type: 1
  textMsg: {
    content: string
  }
}

export type ChatMessagePayload = TextMessagePayload

export interface PrivateChatMessage {
  messageId: number
  sendUserID: number
  revUserID: number
  clientMsgId: string
  msg: ChatMessagePayload
  createdAt: string
}

export interface ChatPeer {
  userID: number
  nickname: string
  avatar: string
}

export interface ChatSession {
  peer: ChatPeer
  lastMessage?: PrivateChatMessage
  lastMessageID: number
  lastMessageAt: string
  lastReadMessageID: number | null
  unreadCount: number
  isTop: boolean
}

export interface ChatSessionList {
  list: ChatSession[]
  count: number
}

export interface ChatHistoryPage {
  list: PrivateChatMessage[]
  hasMore: boolean
  nextBeforeMessageID?: number
}

export interface ChatSendEnvelope {
  event: 'chat.send'
  clientMsgId: string
  data: {
    revUserID: number
    msg: ChatMessagePayload
  }
}

export interface ChatAckEnvelope {
  event: 'chat.ack'
  clientMsgId: string
  data: {
    messageId: number
    createdAt: string
  }
}

export interface ChatMessageEnvelope {
  event: 'chat.message'
  data: PrivateChatMessage
}

export interface ChatErrorEnvelope {
  event: 'error'
  clientMsgId?: string
  error: {
    code: number
    message: string
  }
}

export type ChatServerEnvelope = ChatAckEnvelope | ChatMessageEnvelope | ChatErrorEnvelope

export type LocalMessageStatus = 'pending' | 'sent' | 'failed'

export interface ChatMessageItem {
  localID: string
  messageId?: number
  sendUserID: number
  revUserID: number
  clientMsgId: string
  msg: ChatMessagePayload
  createdAt: string
  status: LocalMessageStatus
  errorMessage?: string
}

export interface ChatHistoryState {
  loaded: boolean
  loading: boolean
  hasMore: boolean
  nextBeforeMessageID?: number
}
