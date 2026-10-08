<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Message } from '@arco-design/web-vue'

import { PublicApiError } from '@/api/client'
import { useAuthStore } from '@/stores/auth'

const form = reactive({ account: '', password: '' })
const submitting = ref(false)
const auth = useAuthStore()
const route = useRoute()
const router = useRouter()

async function submit(): Promise<void> {
  if (form.account.trim().length < 3 || form.password.length < 8) {
    Message.warning('请输入有效的账号和密码')
    return
  }
  submitting.value = true
  try {
    await auth.login({ account: form.account.trim(), password: form.password })
    if (!auth.isAdmin) {
      Message.error('当前账号没有管理权限')
      await router.replace({ name: 'forbidden' })
      return
    }
    Message.success('登录成功')
    const redirect = typeof route.query.redirect === 'string' && route.query.redirect.startsWith('/admin') ? route.query.redirect : '/admin'
    await router.replace(redirect)
  } catch (error) {
    Message.error(error instanceof PublicApiError ? error.message : '登录失败，请稍后重试')
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <main class="admin-login">
    <section class="login-copy">
      <span>GIM · Operations</span>
      <h1>看清系统，<br />再做决定。</h1>
      <p>管理端复用统一 Auth 身份链路。界面守卫只改善体验，所有管理操作仍必须由服务端验证管理员角色。</p>
    </section>
    <a-card :bordered="false" class="login-card">
      <h2>管理端登录</h2>
      <p>使用具备管理员角色的 GIM 账号</p>
      <a-form :model="form" layout="vertical" @submit-success="submit">
        <a-form-item field="account" label="账号" :rules="[{ required: true, minLength: 3, message: '请输入有效账号' }]">
          <a-input v-model="form.account" autocomplete="username" />
        </a-form-item>
        <a-form-item field="password" label="密码" :rules="[{ required: true, minLength: 8, message: '请输入有效密码' }]">
          <a-input-password v-model="form.password" autocomplete="current-password" />
        </a-form-item>
        <a-button html-type="submit" type="primary" size="large" long :loading="submitting">进入控制台</a-button>
      </a-form>
    </a-card>
  </main>
</template>

<style scoped>
.admin-login { min-height: 100vh; display: grid; grid-template-columns: minmax(0, 1.1fr) minmax(360px, .7fr); align-items: center; gap: 80px; padding: clamp(32px, 8vw, 120px); background: #17243d; color: white; }
.login-copy span { color: #7fb0ff; font-size: 12px; font-weight: 700; letter-spacing: .18em; text-transform: uppercase; }
.login-copy h1 { margin: 20px 0; font-size: clamp(48px, 7vw, 82px); line-height: 1.02; letter-spacing: -.04em; }
.login-copy p { max-width: 620px; color: #aebdd3; font-size: 16px; line-height: 1.8; }
.login-card { width: 100%; max-width: 460px; justify-self: end; padding: 18px; border-radius: 18px; color: #17243d; box-shadow: 0 28px 80px rgba(0,0,0,.24); }
.login-card h2 { margin-bottom: 6px; font-size: 27px; }
.login-card > p { margin-bottom: 26px; color: #86909c; }
@media (max-width: 820px) { .admin-login { grid-template-columns: 1fr; } .login-copy { display: none; } .login-card { justify-self: center; } }
</style>
