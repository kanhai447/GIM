import { describe, expect, it } from 'vitest'

import { resolveAdminNavigation } from '@/router/guard'

describe('Admin role guard', () => {
  it('redirects anonymous users to login', () => {
    expect(resolveAdminNavigation({ title: '用户', requiresAdmin: true }, '/admin/users', { authenticated: false, role: null })).toEqual({
      name: 'login',
      query: { redirect: '/admin/users' },
    })
  })

  it('rejects member UI access and allows admins', () => {
    expect(resolveAdminNavigation({ title: '用户', requiresAdmin: true }, '/admin/users', { authenticated: true, role: 2 })).toEqual({ name: 'forbidden' })
    expect(resolveAdminNavigation({ title: '用户', requiresAdmin: true }, '/admin/users', { authenticated: true, role: 1 })).toBe(true)
  })
})
