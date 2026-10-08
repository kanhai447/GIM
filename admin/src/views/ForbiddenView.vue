<template>
  <main class="forbidden-page">
    <a-result status="403" title="无管理权限" subtitle="当前身份不能进入 GIM 管理端；服务端仍会对每个管理 API 进行角色校验。">
      <template #extra><router-link to="/login"><a-button type="primary">返回登录</a-button></router-link></template>
    </a-result>
  </main>
</template>

<style scoped>.forbidden-page { min-height: 100vh; display: grid; place-items: center; background: #f3f5f8; }</style>
