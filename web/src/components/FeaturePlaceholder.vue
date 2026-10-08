<script setup lang="ts">
defineProps<{ title: string; description: string }>()
</script>

<template>
  <el-card shadow="never" class="placeholder-card">
    <el-empty :description="description">
      <template #image>
        <div class="placeholder-mark">{{ title.slice(0, 1) }}</div>
      </template>
      <el-tag effect="plain">{{ title }} · 后续 Checkpoint</el-tag>
    </el-empty>
  </el-card>
</template>

<style scoped>
.placeholder-card { min-height: 420px; display: grid; place-items: center; }
.placeholder-mark { width: 72px; height: 72px; border-radius: 22px; display: grid; place-items: center; background: #eaf2ff; color: #315ea8; font-size: 28px; font-weight: 700; }
</style>
