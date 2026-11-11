export interface FriendUser {
  userID: number
  account: string
  nickname: string
  avatar: string
  abstract: string
  remark: string
  isOnline: boolean
  createdAt: string
}

export interface FriendSearchResult {
  userID: number
  account: string
  nickname: string
  avatar: string
  abstract: string
  isFriend: boolean
}

export type FriendRequestStatus = 0 | 1 | 2

export interface FriendRequest {
  id: number
  requester: { userID: number; nickname: string; avatar: string }
  receiverID: number
  status: FriendRequestStatus
  message: string
  createdAt: string
  updatedAt: string
}

export interface FriendListResponse {
  list: FriendUser[]
}

export interface FriendSearchResponse {
  list: FriendSearchResult[]
}

export interface FriendRequestListResponse {
  list: FriendRequest[]
}
