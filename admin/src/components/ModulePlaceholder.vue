<script setup lang="ts">
defineProps<{ title: string; description: string }>()
</script>

<template>
  <a-card :bordered="false" class="module-card">
    <a-empty :description="description">
      <template #image><div class="module-mark">{{ title.slice(0, 1) }}</div></template>
    </a-empty>
    <div class="scope-note">
      <a-tag color="arcoblue">{{ title }}</a-tag>
      <span>当前仅提供安全路由和页面骨架</span>
    </div>
  </a-card>
</template>

<style scoped>
.module-card { min-height: 430px; display: grid; place-items: center; text-align: center; }
.module-mark { width: 68px; height: 68px; display: grid; place-items: center; border-radius: 18px; background: #eef4ff; color: #315ea8; font-size: 26px; font-weight: 800; }
.scope-note { display: flex; align-items: center; justify-content: center; gap: 10px; margin-top: 16px; color: #86909c; }
</style>
