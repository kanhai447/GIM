import { apiClient, apiRequest, toPublicApiError } from '@/api/client'

export interface CursorPage<T> { list: T[]; nextBeforeId: number; hasMore: boolean }

export enum UserStatus {
  Active = 1,
  Disabled = 2,
}

export interface AdminUserRow {
  id: number; account: string; nickname: string; avatar: string; ip?: string; address?: string
  role: 1 | 2; status: UserStatus; isOnline: boolean; sendMessageCount: number; groupAdminCount: number; groupCount: number
  curtailChat: boolean; curtailAddUser: boolean; curtailCreateGroup: boolean; curtailInGroupChat: boolean; createdAt: string
}
export interface UserRestrictions { userId: number; curtailChat: boolean; curtailAddUser: boolean; curtailCreateGroup: boolean; curtailInGroupChat: boolean }

export interface AdminChatMessage {
  id: number; sender: { id: number; nickname: string; avatar: string }; receiver: { id: number; nickname: string; avatar: string }
  clientMsgId: string; messageType: number; preview: string; message: Record<string, unknown>; createdAt: string
}

export interface AdminGroupRow {
  id: number; name: string; avatar: string; description: string; ownerId: number; ownerNickname: string; status: 1 | 2
  memberCount: number; memberOnlineCount: number; messageCount: number; createdAt: string
}
export interface AdminGroupMessage {
  id: number; groupId: number; senderId: number; senderNickname: string; senderAvatar: string; memberNickname: string
  messageType: number; preview: string; message: Record<string, unknown>; createdAt: string
}

export interface AdminFileRow {
  id: number; userId: number; userNickname: string; uid: string; fileName: string; size: number; mimeType: string
  sha256: string; downloadUrl: string; createdAt: string
}

export interface PublicSettings {
  site: { name: string; version: string; notice: string; registrationEnabled: boolean }
  openLogin: { enabled: boolean }
}

export interface AdminOverview {
  usersTotal: number; usersActive: number; usersDisabled: number; privateMessages: number
  groupsActive: number; groupMessages: number; userFiles: number; auditEvents: number
}

export interface AuditLogRow {
  id: number; eventId: string; schemaVersion: number; eventType: string; service: string; action: string; userId?: number
  role?: number; resourceType?: string; resourceId?: string; requestId?: string; traceId?: string; status: string
  errorCode?: number; ip?: string; method?: string; path?: string; occurredAt: string; createdAt: string; metadata: Record<string, unknown>
}

function params(values: Record<string, unknown>): Record<string, unknown> {
  return Object.fromEntries(Object.entries(values).filter(([, value]) => value !== '' && value !== 0 && value !== undefined && value !== null))
}

export const adminApi = {
  overview() { return apiRequest<AdminOverview>({ method: 'GET', url: '/api/settings/admin/overview' }) },
  users(query: { beforeId?: number; limit?: number; search?: string; status?: number } = {}) {
    return apiRequest<CursorPage<AdminUserRow>>({ method: 'GET', url: '/api/user/users', params: params(query) })
  },
  updateRestrictions(input: UserRestrictions) {
    return apiRequest<{ updated: boolean }>({ method: 'POST', url: '/api/user/curtail', data: input })
  },
  disableUser(userId: number) {
    return apiRequest<{ disabled: boolean; status: UserStatus }>({ method: 'DELETE', url: '/api/user/users', params: { userId } })
  },
  enableUser(userId: number) {
    return apiRequest<{ enabled: boolean; status: UserStatus }>({ method: 'PUT', url: '/api/user/users', params: { userId } })
  },
  chats(query: { beforeId?: number; limit?: number; userId?: number; sendUserId?: number; revUserId?: number } = {}) {
    return apiRequest<CursorPage<AdminChatMessage>>({ method: 'GET', url: '/api/chat/admin/history', params: params(query) })
  },
  deleteChatMessage(messageId: number) {
    return apiRequest<{ deleted: boolean }>({ method: 'DELETE', url: '/api/chat/admin/history', params: { messageId } })
  },
  groups(query: { beforeId?: number; limit?: number; search?: string; status?: number } = {}) {
    return apiRequest<CursorPage<AdminGroupRow>>({ method: 'GET', url: '/api/group/groups', params: params(query) })
  },
  dissolveGroup(groupId: number) {
    return apiRequest<{ dissolved: boolean }>({ method: 'DELETE', url: '/api/group/groups', params: { groupId } })
  },
  groupMessages(groupId: number, query: { beforeId?: number; limit?: number } = {}) {
    return apiRequest<CursorPage<AdminGroupMessage>>({ method: 'GET', url: `/api/group/messages/${groupId}`, params: params(query) })
  },
  deleteGroupMessage(messageId: number) {
    return apiRequest<{ deleted: boolean }>({ method: 'DELETE', url: '/api/group/messages', params: { messageId } })
  },
  files(query: { beforeId?: number; limit?: number; userId?: number; search?: string } = {}) {
    return apiRequest<CursorPage<AdminFileRow>>({ method: 'GET', url: '/api/file/files', params: params(query) })
  },
  deleteFile(id: number) {
    return apiRequest<{ deleted: boolean }>({ method: 'DELETE', url: '/api/file/files', params: { id } })
  },
  async downloadFile(file: AdminFileRow): Promise<void> {
    try {
      const response = await apiClient.get<Blob>(file.downloadUrl, { responseType: 'blob' })
      const url = URL.createObjectURL(response.data)
      const link = document.createElement('a')
      link.href = url; link.download = file.fileName; link.click(); URL.revokeObjectURL(url)
    } catch (error) { throw toPublicApiError(error) }
  },
  settings() { return apiRequest<PublicSettings>({ method: 'GET', url: '/api/settings/info' }) },
  updateSettings(input: PublicSettings) { return apiRequest<PublicSettings>({ method: 'PUT', url: '/api/settings/admin/info', data: input }) },
  logs(query: { beforeId?: number; limit?: number; from?: string; to?: string; service?: string; eventType?: string; userId?: number; status?: string; requestId?: string } = {}) {
    return apiRequest<CursorPage<AuditLogRow>>({ method: 'GET', url: '/api/logs/logs', params: params(query) })
  },
  log(id: number) { return apiRequest<AuditLogRow>({ method: 'GET', url: `/api/logs/logs/${id}` }) },
}
