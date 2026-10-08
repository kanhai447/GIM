<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'

import { PublicApiError } from '@/api/client'
import { useAuthStore } from '@/stores/auth'

interface RegisterForm { account: string; nickname: string; pwd: string; rePwd: string }

const form = reactive<RegisterForm>({ account: '', nickname: '', pwd: '', rePwd: '' })
const formRef = ref<FormInstance>()
const submitting = ref(false)
const auth = useAuthStore()
const router = useRouter()
const rules: FormRules<RegisterForm> = {
  account: [{ required: true, min: 3, max: 64, message: '请输入 3–64 位账号', trigger: 'blur' }],
  nickname: [{ required: true, min: 1, max: 32, message: '请输入 1–32 位昵称', trigger: 'blur' }],
  pwd: [{ required: true, min: 8, max: 72, message: '请输入 8–72 位密码', trigger: 'blur' }],
  rePwd: [{
    validator: (_rule, value: string, callback) => value === form.pwd ? callback() : callback(new Error('两次输入的密码不一致')),
    trigger: 'blur',
  }],
}

async function submit(): Promise<void> {
  if (!formRef.value || !(await formRef.value.validate().catch(() => false))) return
  submitting.value = true
  try {
    await auth.register(form)
    ElMessage.success('注册成功，请登录')
    await router.replace({ name: 'login' })
  } catch (error) {
    ElMessage.error(error instanceof PublicApiError ? error.message : '注册失败，请稍后重试')
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <main class="auth-page">
    <section class="auth-intro">
      <span class="eyebrow">GIM · Start clearly</span>
      <h1>创建账号</h1>
      <p>注册请求只经 Gateway 进入 Auth，凭据由后端安全处理，不在浏览器日志中留下痕迹。</p>
    </section>
    <el-card class="auth-card" shadow="never">
      <h2>注册 GIM</h2>
      <p class="muted">填写基础资料即可开始</p>
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top" @submit.prevent="submit">
        <el-form-item label="账号" prop="account"><el-input v-model="form.account" autocomplete="username" /></el-form-item>
        <el-form-item label="昵称" prop="nickname"><el-input v-model="form.nickname" /></el-form-item>
        <el-form-item label="密码" prop="pwd"><el-input v-model="form.pwd" type="password" show-password autocomplete="new-password" /></el-form-item>
        <el-form-item label="确认密码" prop="rePwd"><el-input v-model="form.rePwd" type="password" show-password autocomplete="new-password" /></el-form-item>
        <el-button native-type="submit" type="primary" size="large" :loading="submitting" class="submit">创建账号</el-button>
      </el-form>
      <p class="switch-link">已有账号？<router-link to="/login">返回登录</router-link></p>
    </el-card>
  </main>
</template>
