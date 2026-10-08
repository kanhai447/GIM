import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { authApi } from '@/api/auth'
import type { AuthUser, LoginRequest, RegisterRequest } from '@/types/api'
import { clearSession, readStoredSession, saveSession } from '@/utils/session'

export const useAuthStore = defineStore('auth', () => {
  const token = ref('')
  const user = ref<AuthUser | null>(null)
  const restored = ref(false)
  const authenticated = computed(() => token.value !== '' && user.value !== null)

  async function login(input: LoginRequest): Promise<void> {
    const result = await authApi.login(input)
    token.value = result.token
    user.value = result.user
    saveSession(result.token, result.user)
    restored.value = true
  }

  async function register(input: RegisterRequest): Promise<AuthUser> {
    return authApi.register(input)
  }

  async function restoreSession(): Promise<void> {
    if (restored.value) return
    restored.value = true
    const session = readStoredSession()
    if (!session) return
    token.value = session.token
    user.value = session.user
    try {
      const currentUser = await authApi.profile()
      user.value = currentUser
      saveSession(session.token, currentUser)
    } catch {
      reset()
    }
  }

  async function logout(): Promise<void> {
    try {
      if (token.value) await authApi.logout()
    } finally {
      reset()
    }
  }

  function reset(): void {
    token.value = ''
    user.value = null
    restored.value = true
    clearSession()
  }

  return { token, user, restored, authenticated, login, register, restoreSession, logout, reset }
})
