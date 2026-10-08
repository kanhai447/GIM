import { apiRequest } from '@/api/client'
import type { AuthUser, LoginRequest, LoginResult, LogoutResult, RegisterRequest } from '@/types/api'

export const authApi = {
  login(input: LoginRequest): Promise<LoginResult> {
    return apiRequest<LoginResult>({ method: 'POST', url: '/api/auth/login', data: input })
  },
  register(input: RegisterRequest): Promise<AuthUser> {
    return apiRequest<AuthUser>({ method: 'POST', url: '/api/auth/register', data: input })
  },
  profile(): Promise<AuthUser> {
    return apiRequest<AuthUser>({ method: 'GET', url: '/api/user/user_info' })
  },
  logout(): Promise<LogoutResult> {
    return apiRequest<LogoutResult>({ method: 'POST', url: '/api/auth/logout' })
  },
}
