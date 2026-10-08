import { createRouter, createWebHistory } from 'vue-router'

import { resolveAuthNavigation } from '@/router/guard'
import { useAuthStore } from '@/stores/auth'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/login',
      name: 'login',
      component: () => import('@/views/LoginView.vue'),
      meta: { title: '登录', guestOnly: true },
    },
    {
      path: '/register',
      name: 'register',
      component: () => import('@/views/RegisterView.vue'),
      meta: { title: '注册', guestOnly: true },
    },
    {
      path: '/',
      component: () => import('@/layouts/MainLayout.vue'),
      meta: { title: 'GIM', requiresAuth: true },
      children: [
        { path: '', name: 'home', component: () => import('@/views/HomeView.vue'), meta: { title: '首页', requiresAuth: true } },
        { path: 'chat', name: 'chat', component: () => import('@/views/ChatView.vue'), meta: { title: '私聊', requiresAuth: true } },
        { path: 'group', name: 'group', component: () => import('@/views/GroupView.vue'), meta: { title: '群聊', requiresAuth: true } },
        { path: 'files', name: 'files', component: () => import('@/views/FileView.vue'), meta: { title: '文件', requiresAuth: true } },
        { path: 'profile', name: 'profile', component: () => import('@/views/ProfileView.vue'), meta: { title: '个人资料', requiresAuth: true } },
      ],
    },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})

router.beforeEach(async (to) => {
  const auth = useAuthStore()
  await auth.restoreSession()
  const result = resolveAuthNavigation(to.meta, to.fullPath, auth)
  if (result === true) document.title = `${to.meta.title} · GIM`
  return result
})

export default router
