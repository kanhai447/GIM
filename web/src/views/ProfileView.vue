<script setup lang="ts">
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
</script>

<template>
  <el-card shadow="never" class="profile-card">
    <div class="profile-heading">
      <el-avatar :size="72" :src="auth.user?.avatar || undefined">{{ auth.user?.nickname.slice(0, 1) }}</el-avatar>
      <div><h2>{{ auth.user?.nickname }}</h2><p>@{{ auth.user?.account }}</p></div>
    </div>
    <el-descriptions :column="1" border>
      <el-descriptions-item label="用户 ID">{{ auth.user?.userID }}</el-descriptions-item>
      <el-descriptions-item label="角色">{{ auth.user?.role === 1 ? '管理员' : '普通用户' }}</el-descriptions-item>
      <el-descriptions-item label="状态">{{ auth.user?.status === 1 ? '正常' : '已禁用' }}</el-descriptions-item>
    </el-descriptions>
  </el-card>
</template>

<style scoped>
.profile-card { max-width: 760px; }
.profile-heading { display: flex; align-items: center; gap: 18px; margin-bottom: 26px; }
.profile-heading h2 { margin: 0 0 6px; }
.profile-heading p { margin: 0; color: #7f8998; }
</style>
