<script setup lang="ts">
import { computed, ref } from 'vue'
import type { ChatConnectionState } from '@/types/chat'

const props = defineProps<{ disabled: boolean; connectionState: ChatConnectionState }>()
const emit = defineEmits<{ send: [content: string] }>()
const content = ref('')
const hint = computed(() => props.connectionState === 'connected'
  ? '已连接'
  : props.connectionState === 'reconnecting'
    ? '正在重连，消息会在连接恢复后发送'
    : '当前离线，消息会进入内存待发送队列')

function submit(): void {
  const value = content.value.trim()
  if (!value || props.disabled) return
  emit('send', value)
  content.value = ''
}

function handleKeydown(event: KeyboardEvent): void {
  if (event.key !== 'Enter' || event.shiftKey) return
  event.preventDefault()
  submit()
}
</script>

<template>
  <footer class="composer">
    <div class="composer-status" :data-connection="connectionState">{{ hint }}</div>
    <div class="composer-row">
      <el-input v-model="content" type="textarea" :rows="3" resize="none" maxlength="4000" show-word-limit :disabled="disabled" placeholder="输入消息，Enter 发送，Shift + Enter 换行" @keydown="handleKeydown" />
      <el-button type="primary" :disabled="disabled || !content.trim()" @click="submit">发送</el-button>
    </div>
  </footer>
</template>

<style scoped>
.composer { padding: 12px 18px 16px; border-top: 1px solid #e7ebf1; background: #fff; }
.composer-status { margin-bottom: 7px; color: #8a95a6; font-size: 11px; }
.composer-status[data-connection='connected'] { color: #2f855a; }
.composer-status[data-connection='reconnecting'] { color: #b7791f; }
.composer-row { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 12px; align-items: end; }
</style>
