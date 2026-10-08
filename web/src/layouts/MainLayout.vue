<script setup lang="ts">
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'

import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const route = useRoute()
const router = useRouter()

async function logout(): Promise<void> {
  try {
    await auth.logout()
    ElMessage.success('已安全退出')
  } catch {
    ElMessage.warning('服务暂时不可用，本地登录状态已清理')
  } finally {
    await router.replace({ name: 'login' })
  }
}
</script>

<template>
  <el-container class="app-shell">
    <el-aside width="224px" class="sidebar">
      <router-link to="/" class="brand">GIM<span>Messaging</span></router-link>
      <el-menu :default-active="route.path" router class="nav-menu">
        <el-menu-item index="/">概览</el-menu-item>
        <el-menu-item index="/chat">私聊</el-menu-item>
        <el-menu-item index="/group">群聊</el-menu-item>
        <el-menu-item index="/files">文件</el-menu-item>
        <el-menu-item index="/profile">个人资料</el-menu-item>
      </el-menu>
      <p class="phase-note">Day 1 · Foundation</p>
    </el-aside>
    <el-container>
      <el-header class="topbar">
        <div>
          <strong>{{ route.meta.title }}</strong>
          <span class="muted">稳定基础，逐步构建实时通信</span>
        </div>
        <div class="user-actions">
          <el-avatar :src="auth.user?.avatar || undefined">{{ auth.user?.nickname.slice(0, 1) }}</el-avatar>
          <span>{{ auth.user?.nickname }}</span>
          <el-button text type="primary" @click="logout">退出登录</el-button>
        </div>
      </el-header>
      <el-main class="content"><router-view /></el-main>
    </el-container>
  </el-container>
</template>

<style scoped>
.app-shell { min-height: 100vh; }
.sidebar { position: relative; display: flex; flex-direction: column; background: #f7f9fc; border-right: 1px solid #e6eaf0; }
.brand { display: flex; flex-direction: column; padding: 28px 26px 22px; color: #17243d; font-size: 27px; font-weight: 800; letter-spacing: .08em; text-decoration: none; }
.brand span { margin-top: 3px; color: #6a7891; font-size: 11px; font-weight: 600; letter-spacing: .18em; text-transform: uppercase; }
.nav-menu { border-right: 0; background: transparent; }
.phase-note { margin: auto 24px 24px; color: #9aa4b4; font-size: 12px; }
.topbar { height: 74px; display: flex; align-items: center; justify-content: space-between; border-bottom: 1px solid #edf0f4; background: rgba(255,255,255,.92); }
.topbar > div:first-child { display: flex; flex-direction: column; gap: 4px; }
.muted { color: #8993a4; font-size: 12px; }
.user-actions { display: flex; align-items: center; gap: 12px; color: #364156; }
.content { background: #f4f6f9; padding: 28px; }
@media (max-width: 760px) { .sidebar { width: 168px !important; } .muted, .user-actions > span { display: none; } .content { padding: 16px; } }
</style>
