<script setup lang="ts">
import { computed, ref } from 'vue'

import type { GroupConnectionState } from '@/types/group'

const props = withDefaults(defineProps<{ disabled: boolean; connectionState: GroupConnectionState; uploading?: boolean; uploadingFile?: boolean }>(), {
  uploading: false,
  uploadingFile: false,
})
const emit = defineEmits<{ send: [content: string]; image: [file: File]; file: [file: File] }>()
const content = ref('')
const imageInput = ref<HTMLInputElement | null>(null)
const fileInput = ref<HTMLInputElement | null>(null)
const busy = computed(() => props.disabled || props.uploading || props.uploadingFile)
const hint = computed(() => props.connectionState === 'connected'
  ? '群连接已就绪'
  : props.connectionState === 'reconnecting'
    ? '群连接正在重连，消息留在当前内存队列'
    : '群连接离线，恢复后将发送当前内存队列')

function submit(): void {
  const value = content.value.trim()
  if (!value || busy.value) return
  emit('send', value)
  content.value = ''
}

function handleKeydown(event: KeyboardEvent): void {
  if (event.key !== 'Enter' || event.shiftKey) return
  event.preventDefault()
  submit()
}

function chooseImage(): void {
  if (!busy.value) imageInput.value?.click()
}

function chooseFile(): void {
  if (!busy.value) fileInput.value?.click()
}

function onImageSelected(event: Event): void {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (file && !busy.value) emit('image', file)
}

function onFileSelected(event: Event): void {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (file && !busy.value) emit('file', file)
}
</script>

<template>
  <footer class="composer">
    <div class="status" :data-connection="connectionState">{{ uploading ? '图片上传中…' : uploadingFile ? '文件上传中…' : hint }}</div>
    <div class="row">
      <el-input v-model="content" type="textarea" :rows="3" resize="none" maxlength="4000" show-word-limit :disabled="busy" placeholder="输入群消息，Enter 发送，Shift + Enter 换行" @keydown="handleKeydown" />
      <div class="actions">
        <input ref="imageInput" class="file-input" type="file" accept="image/*" @change="onImageSelected" />
        <input ref="fileInput" class="file-input" type="file" @change="onFileSelected" />
        <el-button :disabled="busy" :loading="uploading" @click="chooseImage">图片</el-button>
        <el-button :disabled="busy" :loading="uploadingFile" @click="chooseFile">文件</el-button>
        <el-button type="primary" :disabled="busy || !content.trim()" @click="submit">发送</el-button>
      </div>
    </div>
  </footer>
</template>

<style scoped>
.composer { padding: 12px 18px 16px; border-top: 1px solid #e7ebf1; background: #fff; }
.status { margin-bottom: 7px; color: #8a95a6; font-size: 11px; }
.status[data-connection='connected'] { color: #2f855a; }
.status[data-connection='reconnecting'] { color: #b7791f; }
.row { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 12px; align-items: end; }
.actions { display: flex; gap: 8px; align-items: center; }
.file-input { display: none; }
</style>
