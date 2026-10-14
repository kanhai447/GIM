import { createPinia } from 'pinia'
import { createSSRApp, ref } from 'vue'
import { renderToString } from 'vue/server-renderer'
import { createMemoryHistory, createRouter } from 'vue-router'
import { describe, expect, it, vi } from 'vitest'

import { PublicApiError } from '@/api/client'
import { registerArcoComponents } from '@/ui/arco'
import LoginView from '@/views/LoginView.vue'
import { submitAdminLogin, type LoginDependencies, type LoginForm } from '@/views/loginFlow'

function dependencies(overrides: Partial<LoginDependencies> = {}): LoginDependencies {
  return {
    login: vi.fn().mockResolvedValue(undefined),
    isAdmin: () => true,
    replace: vi.fn().mockResolvedValue(undefined),
    notify: { warning: vi.fn(), error: vi.fn(), success: vi.fn() },
    ...overrides,
  }
}

describe('Admin Login View', () => {
  it('renders real account, password and submit controls through registered Arco components', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/login', component: LoginView }],
    })
    await router.push('/login?redirect=/admin')
    await router.isReady()

    const app = createSSRApp(LoginView)
    app.use(createPinia())
    app.use(router)
    registerArcoComponents(app)
    const html = await renderToString(app)

    expect(html).toContain('autocomplete="username"')
    expect(html).toContain('autocomplete="current-password"')
    expect(html).toMatch(/type="password"/)
    expect(html).toContain('进入控制台')
    expect((html.match(/<input/g) ?? []).length).toBeGreaterThanOrEqual(2)
  })

  it('submits entered credentials and preserves a safe admin redirect', async () => {
    const input: LoginForm = { account: '  local-admin  ', password: 'test-password-input' }
    const deps = dependencies()

    const outcome = await submitAdminLogin(input, '/admin/users?status=1', ref(false), deps)

    expect(outcome).toBe('success')
    expect(deps.login).toHaveBeenCalledWith({ account: 'local-admin', password: 'test-password-input' })
    expect(deps.replace).toHaveBeenCalledWith('/admin/users?status=1')
    expect(deps.notify.success).toHaveBeenCalledWith('登录成功')
  })

  it('keeps entered values and shows a safe error after login failure', async () => {
    const input: LoginForm = { account: 'local-admin', password: 'test-password-input' }
    const deps = dependencies({ login: vi.fn().mockRejectedValue(new PublicApiError('账号或密码错误', 1201)) })

    const outcome = await submitAdminLogin(input, '/admin', ref(false), deps)

    expect(outcome).toBe('failed')
    expect(input).toEqual({ account: 'local-admin', password: 'test-password-input' })
    expect(deps.replace).not.toHaveBeenCalled()
    expect(deps.notify.error).toHaveBeenCalledWith('账号或密码错误')
  })

  it('prevents duplicate submission while loading', async () => {
    const deps = dependencies()
    const outcome = await submitAdminLogin(
      { account: 'local-admin', password: 'test-password-input' },
      '/admin',
      ref(true),
      deps,
    )

    expect(outcome).toBe('busy')
    expect(deps.login).not.toHaveBeenCalled()
  })
})
