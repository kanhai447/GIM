export interface ApiEnvelope<T> {
  code: number
  msg: string
  data: T
}

export interface AuthUser {
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
  user: AuthUser
}

export interface RegisterRequest {
  account: string
  nickname: string
  pwd: string
  rePwd: string
}

export interface LogoutResult {
  loggedOut: boolean
}
