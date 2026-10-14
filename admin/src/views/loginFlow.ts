import type { Ref } from 'vue'
import type { RouteLocationRaw } from 'vue-router'

import { PublicApiError } from '@/api/client'
import type { LoginRequest } from '@/types/api'

export interface LoginForm {
  account: string
  password: string
}

export interface LoginNotifier {
  warning(message: string): void
  error(message: string): void
  success(message: string): void
}

export interface LoginDependencies {
  login(input: LoginRequest): Promise<void>
  isAdmin(): boolean
  replace(target: RouteLocationRaw): Promise<unknown> | unknown
  notify: LoginNotifier
}

export type LoginOutcome = 'busy' | 'invalid' | 'forbidden' | 'failed' | 'success'

export async function submitAdminLogin(
  form: LoginForm,
  redirect: unknown,
  submitting: Ref<boolean>,
  dependencies: LoginDependencies,
): Promise<LoginOutcome> {
  if (submitting.value) return 'busy'

  const account = form.account.trim()
  if (account.length < 3 || form.password.length < 8) {
    dependencies.notify.warning('请输入有效的账号和密码')
    return 'invalid'
  }

  submitting.value = true
  try {
    await dependencies.login({ account, password: form.password })
    if (!dependencies.isAdmin()) {
      dependencies.notify.error('当前账号没有管理权限')
      await dependencies.replace({ name: 'forbidden' })
      return 'forbidden'
    }

    dependencies.notify.success('登录成功')
    const target = typeof redirect === 'string' && redirect.startsWith('/admin') ? redirect : '/admin'
    await dependencies.replace(target)
    return 'success'
  } catch (error) {
    dependencies.notify.error(error instanceof PublicApiError ? error.message : '登录失败，请稍后重试')
    return 'failed'
  } finally {
    submitting.value = false
  }
}
