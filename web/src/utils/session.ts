import type { AuthUser } from '@/types/api'

const TOKEN_KEY = 'gim.web.token'
const USER_KEY = 'gim.web.user'

function storageAvailable(): boolean {
  return typeof localStorage !== 'undefined'
}

function isAuthUser(value: unknown): value is AuthUser {
  if (typeof value !== 'object' || value === null) return false
  const candidate = value as Partial<AuthUser>
  return (
    typeof candidate.userID === 'number' &&
    candidate.userID > 0 &&
    typeof candidate.account === 'string' &&
    typeof candidate.nickname === 'string' &&
    typeof candidate.avatar === 'string' &&
    (candidate.role === 1 || candidate.role === 2) &&
    (candidate.status === 1 || candidate.status === 2)
  )
}

export function readStoredSession(): { token: string; user: AuthUser } | null {
  if (!storageAvailable()) return null
  const token = localStorage.getItem(TOKEN_KEY)?.trim()
  const rawUser = localStorage.getItem(USER_KEY)
  if (!token || !rawUser) return null
  try {
    const user: unknown = JSON.parse(rawUser)
    if (!isAuthUser(user)) return null
    return { token, user }
  } catch {
    return null
  }
}

export function saveSession(token: string, user: AuthUser): void {
  if (!storageAvailable()) return
  localStorage.setItem(TOKEN_KEY, token)
  localStorage.setItem(USER_KEY, JSON.stringify(user))
}

export function clearSession(): void {
  if (!storageAvailable()) return
  localStorage.removeItem(TOKEN_KEY)
  localStorage.removeItem(USER_KEY)
}

export function readToken(): string | null {
  return readStoredSession()?.token ?? null
}
