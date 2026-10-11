import { apiRequest } from '@/api/client'
import type { ChatHistoryPage, ChatSession, ChatSessionList } from '@/types/chat'

export const chatApi = {
  sessions(limit = 100): Promise<ChatSessionList> {
    return apiRequest<ChatSessionList>({ method: 'GET', url: '/api/chat/session', params: { limit } })
  },

  history(peerUserID: number, beforeMessageID?: number, limit = 20): Promise<ChatHistoryPage> {
    return apiRequest<ChatHistoryPage>({
      method: 'GET',
      url: '/api/chat/history',
      params: { peerUserID, beforeMessageID, limit },
    })
  },

  markRead(peerUserID: number, lastReadMessageID: number): Promise<ChatSession> {
    return apiRequest<ChatSession>({
      method: 'PUT',
      url: '/api/chat/read',
      data: { peerUserID, lastReadMessageID },
    })
  },

  setTop(peerUserID: number, isTop: boolean): Promise<ChatSession> {
    return apiRequest<ChatSession>({
      method: 'POST',
      url: '/api/chat/user_top',
      data: { peerUserID, isTop },
    })
  },
}
