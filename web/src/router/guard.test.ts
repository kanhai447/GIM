import { describe, expect, it } from 'vitest'

import { resolveAuthNavigation } from '@/router/guard'

describe('Web auth route guard', () => {
  it('redirects anonymous users away from protected routes', () => {
    expect(resolveAuthNavigation({ title: '私聊', requiresAuth: true }, '/chat', { authenticated: false })).toEqual({
      name: 'login',
      query: { redirect: '/chat' },
    })
  })

  it('allows authenticated users and redirects them away from guest pages', () => {
    expect(resolveAuthNavigation({ title: '私聊', requiresAuth: true }, '/chat', { authenticated: true })).toBe(true)
    expect(resolveAuthNavigation({ title: '登录', guestOnly: true }, '/login', { authenticated: true })).toEqual({ name: 'home' })
  })
})
