<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'

import { PublicApiError } from '@/api/client'
import { useAuthStore } from '@/stores/auth'

interface LoginForm { account: string; password: string }

const form = reactive<LoginForm>({ account: '', password: '' })
const rules: FormRules<LoginForm> = {
  account: [{ required: true, min: 3, max: 64, message: '请输入 3–64 位账号', trigger: 'blur' }],
  password: [{ required: true, min: 8, max: 72, message: '请输入 8–72 位密码', trigger: 'blur' }],
}
const formRef = ref<FormInstance>()
const submitting = ref(false)
const auth = useAuthStore()
const route = useRoute()
const router = useRouter()

async function submit(): Promise<void> {
  if (!formRef.value || !(await formRef.value.validate().catch(() => false))) return
  submitting.value = true
  try {
    await auth.login(form)
    ElMessage.success('登录成功')
    const redirect = typeof route.query.redirect === 'string' && route.query.redirect.startsWith('/') ? route.query.redirect : '/'
    await router.replace(redirect)
  } catch (error) {
    ElMessage.error(error instanceof PublicApiError ? error.message : '登录失败，请稍后重试')
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <main class="auth-page">
    <section class="auth-intro">
      <span class="eyebrow">GIM · Reliable messaging</span>
      <h1>欢迎回来</h1>
      <p>从可靠的身份链路开始，逐步构建可解释、可测试的即时通信体验。</p>
    </section>
    <el-card class="auth-card" shadow="never">
      <h2>登录 GIM</h2>
      <p class="muted">使用你的账号继续</p>
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top" @submit.prevent="submit">
        <el-form-item label="账号" prop="account"><el-input v-model="form.account" autocomplete="username" /></el-form-item>
        <el-form-item label="密码" prop="password"><el-input v-model="form.password" type="password" show-password autocomplete="current-password" /></el-form-item>
        <el-button native-type="submit" type="primary" size="large" :loading="submitting" class="submit">登录</el-button>
      </el-form>
      <p class="switch-link">还没有账号？<router-link to="/register">立即注册</router-link></p>
    </el-card>
  </main>
</template>
