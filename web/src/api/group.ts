import { apiRequest } from '@/api/client'
import type {
  CreateGroupInput,
  GroupHistoryPage,
  GroupList,
  GroupMemberList,
  GroupSession,
  GroupSessionList,
  GroupSummary,
} from '@/types/group'

export const groupApi = {
  sessions(limit = 100): Promise<GroupSessionList> {
    return apiRequest<GroupSessionList>({ method: 'GET', url: '/api/group/session', params: { limit } })
  },

  myGroups(limit = 100): Promise<GroupList> {
    return apiRequest<GroupList>({ method: 'GET', url: '/api/group/my', params: { limit } })
  },

  info(groupID: number): Promise<GroupSummary> {
    return apiRequest<GroupSummary>({ method: 'GET', url: `/api/group/group/${groupID}` })
  },

  members(groupID: number, limit = 100): Promise<GroupMemberList> {
    return apiRequest<GroupMemberList>({ method: 'GET', url: '/api/group/member', params: { id: groupID, limit } })
  },

  history(groupID: number, beforeMessageID?: number, limit = 20): Promise<GroupHistoryPage> {
    return apiRequest<GroupHistoryPage>({
      method: 'GET',
      url: `/api/group/history/${groupID}`,
      params: { beforeMessageID, limit },
    })
  },

  markRead(groupID: number, lastReadMessageID: number): Promise<GroupSession> {
    return apiRequest<GroupSession>({
      method: 'PUT',
      url: '/api/group/read',
      data: { groupID, lastReadMessageID },
    })
  },

  create(input: CreateGroupInput): Promise<GroupSummary> {
    return apiRequest<GroupSummary>({ method: 'POST', url: '/api/group/group', data: input })
  },

  setMemberMute(groupID: number, memberID: number, minutes?: number): Promise<{ updated: boolean }> {
    return apiRequest<{ updated: boolean }>({
      method: 'PUT',
      url: '/api/group/member/prohibition',
      data: { groupId: groupID, memberId: memberID, prohibitionTime: minutes },
    })
  },
}
