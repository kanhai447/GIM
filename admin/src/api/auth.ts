import { apiRequest } from '@/api/client'
import type { AdminUser, LoginRequest, LoginResult, LogoutResult } from '@/types/api'

export const authApi = {
  login(input: LoginRequest): Promise<LoginResult> {
    return apiRequest<LoginResult>({ method: 'POST', url: '/api/auth/login', data: input })
  },
  profile(): Promise<AdminUser> {
    return apiRequest<AdminUser>({ method: 'GET', url: '/api/user/user_info' })
  },
  logout(): Promise<LogoutResult> {
    return apiRequest<LogoutResult>({ method: 'POST', url: '/api/auth/logout' })
  },
}
