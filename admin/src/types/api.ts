export interface ApiEnvelope<T> {
  code: number
  msg: string
  data: T
}

export interface AdminUser {
  userID: number
  account: string
  nickname: string
  avatar: string
  role: 1 | 2
  status: 1 | 2
}

export interface LoginRequest {
  account: string
  password: string
}

export interface LoginResult {
  token: string
  user: AdminUser
}

export interface LogoutResult {
  loggedOut: boolean
}
