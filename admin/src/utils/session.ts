import type { AdminUser } from '@/types/api'

const TOKEN_KEY = 'gim.admin.token'
const USER_KEY = 'gim.admin.user'

function storageAvailable(): boolean {
  return typeof localStorage !== 'undefined'
}

function isAdminUser(value: unknown): value is AdminUser {
  if (typeof value !== 'object' || value === null) return false
  const candidate = value as Partial<AdminUser>
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

export function readStoredSession(): { token: string; user: AdminUser } | null {
  if (!storageAvailable()) return null
  const token = localStorage.getItem(TOKEN_KEY)?.trim()
  const rawUser = localStorage.getItem(USER_KEY)
  if (!token || !rawUser) return null
  try {
    const user: unknown = JSON.parse(rawUser)
    return isAdminUser(user) ? { token, user } : null
  } catch {
    return null
  }
}

export function saveSession(token: string, user: AdminUser): void {
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
