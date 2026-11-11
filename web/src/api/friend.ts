import { apiRequest } from '@/api/client'
import type {
  FriendListResponse,
  FriendRequest,
  FriendRequestListResponse,
  FriendSearchResponse,
  FriendUser,
} from '@/types/friend'

export const friendApi = {
  search(keyword: string, limit = 20): Promise<FriendSearchResponse> {
    return apiRequest<FriendSearchResponse>({ method: 'GET', url: '/api/user/search', params: { keyword, limit } })
  },

  sendRequest(friendID: number, message = ''): Promise<FriendRequest> {
    return apiRequest<FriendRequest>({ method: 'POST', url: '/api/user/friends', data: { friendID, message } })
  },

  requests(status: number | 'all' = 0, limit = 20): Promise<FriendRequestListResponse> {
    return apiRequest<FriendRequestListResponse>({ method: 'GET', url: '/api/user/valid', params: { status, limit } })
  },

  respond(validID: number, status: 1 | 2): Promise<FriendRequest> {
    return apiRequest<FriendRequest>({ method: 'PUT', url: '/api/user/valid_status', data: { validID, status } })
  },

  friends(limit = 50): Promise<FriendListResponse> {
    return apiRequest<FriendListResponse>({ method: 'GET', url: '/api/user/friends', params: { limit } })
  },

  updateRemark(friendID: number, remark: string): Promise<FriendUser> {
    return apiRequest<FriendUser>({ method: 'PUT', url: '/api/user/friends', data: { friendID, remark } })
  },

  remove(friendID: number): Promise<{ deleted: boolean }> {
    return apiRequest<{ deleted: boolean }>({ method: 'DELETE', url: '/api/user/friends', params: { friendID } })
  },

  info(friendID: number): Promise<FriendUser> {
    return apiRequest<FriendUser>({ method: 'GET', url: '/api/user/friend_info', params: { friendID } })
  },
}
