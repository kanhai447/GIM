<script setup lang="ts">
import { useRoute, useRouter } from 'vue-router'
import { Message } from '@arco-design/web-vue'

import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const route = useRoute()
const router = useRouter()

const navigation = [
  { path: '/admin', label: '概览', mark: 'OV' },
  { path: '/admin/users', label: '用户管理', mark: 'US' },
  { path: '/admin/chats', label: '私聊管理', mark: 'CH' },
  { path: '/admin/groups', label: '群聊管理', mark: 'GR' },
  { path: '/admin/files', label: '文件管理', mark: 'FI' },
  { path: '/admin/settings', label: '系统设置', mark: 'SE' },
  { path: '/admin/logs', label: '日志管理', mark: 'LO' },
]

async function logout(): Promise<void> {
  try {
    await auth.logout()
    Message.success('已安全退出')
  } catch {
    Message.warning('服务暂时不可用，本地登录状态已清理')
  } finally {
    await router.replace({ name: 'login' })
  }
}
</script>

<template>
  <a-layout class="admin-shell">
    <a-layout-sider :width="238" class="admin-sider">
      <router-link to="/admin" class="admin-brand">
        <strong>GIM</strong><span>Operations Console</span>
      </router-link>
      <a-menu :selected-keys="[route.path]" class="admin-menu" @menu-item-click="(key: string) => router.push(key)">
        <a-menu-item v-for="item in navigation" :key="item.path">
          <span class="menu-mark">{{ item.mark }}</span>{{ item.label }}
        </a-menu-item>
      </a-menu>
      <div class="security-note">前端角色仅用于界面导航<br />服务端权限校验才是安全边界</div>
    </a-layout-sider>
    <a-layout>
      <a-layout-header class="admin-header">
        <div><strong>{{ route.meta.title }}</strong><span>Day 1 · Foundation</span></div>
        <div class="admin-user">
          <a-avatar :size="34" :image-url="auth.user?.avatar || undefined">{{ auth.user?.nickname.slice(0, 1) }}</a-avatar>
          <span>{{ auth.user?.nickname }}</span>
          <a-button type="text" @click="logout">退出</a-button>
        </div>
      </a-layout-header>
      <a-layout-content class="admin-content"><router-view /></a-layout-content>
    </a-layout>
  </a-layout>
</template>

<style scoped>
.admin-shell { min-height: 100vh; }
.admin-sider { position: fixed; inset: 0 auto 0 0; background: #17243d; color: #fff; }
.admin-brand { display: flex; flex-direction: column; padding: 30px 28px 22px; color: white; text-decoration: none; }
.admin-brand strong { font-size: 26px; letter-spacing: .08em; }
.admin-brand span { margin-top: 4px; color: #9eb3d4; font-size: 10px; letter-spacing: .15em; text-transform: uppercase; }
.admin-menu { background: transparent; color: #d9e5f9; }
.menu-mark { display: inline-grid; width: 28px; height: 22px; margin-right: 10px; place-items: center; border: 1px solid rgba(255,255,255,.18); border-radius: 6px; font-size: 9px; }
.security-note { position: absolute; right: 24px; bottom: 24px; left: 24px; color: #7890b5; font-size: 11px; line-height: 1.7; }
.admin-shell > .arco-layout { margin-left: 238px; }
.admin-header { height: 72px; display: flex; align-items: center; justify-content: space-between; padding: 0 28px; border-bottom: 1px solid #e8ebef; background: white; }
.admin-header > div:first-child { display: flex; flex-direction: column; gap: 4px; }
.admin-header > div:first-child span { color: #86909c; font-size: 11px; }
.admin-user { display: flex; align-items: center; gap: 10px; }
.admin-content { min-height: calc(100vh - 72px); padding: 28px; background: #f3f5f8; }
@media (max-width: 780px) { .admin-sider { width: 176px !important; } .admin-shell > .arco-layout { margin-left: 176px; } .admin-user > span { display: none; } }
</style>
