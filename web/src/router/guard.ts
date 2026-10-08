import type { RouteLocationRaw, RouteMeta } from 'vue-router'

export interface GuardAuthState {
  authenticated: boolean
}

export function resolveAuthNavigation(
  meta: RouteMeta,
  fullPath: string,
  auth: GuardAuthState,
): RouteLocationRaw | true {
  if (meta.requiresAuth && !auth.authenticated) {
    return { name: 'login', query: { redirect: fullPath } }
  }
  if (meta.guestOnly && auth.authenticated) return { name: 'home' }
  return true
}
