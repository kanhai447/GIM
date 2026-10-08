import { createRouter, createWebHistory } from 'vue-router'

import { resolveAdminNavigation } from '@/router/guard'
import { useAuthStore } from '@/stores/auth'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    { path: '/', redirect: '/admin' },
    { path: '/login', name: 'login', component: () => import('@/views/LoginView.vue'), meta: { title: '登录', guestOnly: true } },
    { path: '/forbidden', name: 'forbidden', component: () => import('@/views/ForbiddenView.vue'), meta: { title: '无权限' } },
    {
      path: '/admin',
      component: () => import('@/layouts/AdminLayout.vue'),
      meta: { title: '管理控制台', requiresAdmin: true },
      children: [
        { path: '', name: 'dashboard', component: () => import('@/views/DashboardView.vue'), meta: { title: '概览', requiresAdmin: true } },
        { path: 'users', name: 'users', component: () => import('@/views/UsersView.vue'), meta: { title: '用户管理', requiresAdmin: true } },
        { path: 'chats', name: 'chats', component: () => import('@/views/ChatsView.vue'), meta: { title: '私聊管理', requiresAdmin: true } },
        { path: 'groups', name: 'groups', component: () => import('@/views/GroupsView.vue'), meta: { title: '群聊管理', requiresAdmin: true } },
        { path: 'files', name: 'files', component: () => import('@/views/FilesView.vue'), meta: { title: '文件管理', requiresAdmin: true } },
        { path: 'settings', name: 'settings', component: () => import('@/views/SettingsView.vue'), meta: { title: '系统设置', requiresAdmin: true } },
        { path: 'logs', name: 'logs', component: () => import('@/views/LogsView.vue'), meta: { title: '日志管理', requiresAdmin: true } },
      ],
    },
    { path: '/:pathMatch(.*)*', redirect: '/admin' },
  ],
})

router.beforeEach(async (to) => {
  const auth = useAuthStore()
  await auth.restoreSession()
  const result = resolveAdminNavigation(to.meta, to.fullPath, {
    authenticated: auth.authenticated,
    role: auth.user?.role ?? null,
  })
  if (result === true) document.title = `${to.meta.title} · GIM Admin`
  return result
})

export default router
