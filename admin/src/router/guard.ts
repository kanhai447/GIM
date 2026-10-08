import type { RouteLocationRaw, RouteMeta } from 'vue-router'

export interface AdminGuardState {
  authenticated: boolean
  role: 1 | 2 | null
}

export function resolveAdminNavigation(
  meta: RouteMeta,
  fullPath: string,
  auth: AdminGuardState,
): RouteLocationRaw | true {
  if (meta.requiresAdmin && !auth.authenticated) {
    return { name: 'login', query: { redirect: fullPath } }
  }
  if (meta.requiresAdmin && auth.role !== 1) return { name: 'forbidden' }
  if (meta.guestOnly && auth.authenticated && auth.role === 1) return { name: 'dashboard' }
  return true
}
