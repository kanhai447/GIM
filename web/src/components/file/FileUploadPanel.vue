<script setup lang="ts">
import type { FileUploadTask } from '@/types/file'
import { formatFileSize, MAX_UPLOAD_BYTES } from '@/utils/file'

defineProps<{ tasks: FileUploadTask[] }>()
const emit = defineEmits<{ select: [files: File[]] }>()

function selectFiles(event: Event): void {
  const input = event.target as HTMLInputElement
  emit('select', Array.from(input.files ?? []))
  input.value = ''
}
</script>

<template>
  <section class="upload-panel">
    <div>
      <p class="eyebrow">Secure streaming upload</p>
      <h2>上传文件</h2>
      <p>支持多文件独立上传，单文件最大 {{ formatFileSize(MAX_UPLOAD_BYTES) }}。</p>
    </div>
    <label class="file-picker">
      <span>选择文件</span>
      <input type="file" multiple @change="selectFiles">
    </label>
    <ul v-if="tasks.length" class="upload-tasks">
      <li v-for="task in tasks" :key="task.id" class="upload-task" :data-status="task.status">
        <div class="task-title"><strong>{{ task.fileName }}</strong><span>{{ formatFileSize(task.size) }}</span></div>
        <el-progress :percentage="task.progress" :status="task.status === 'failed' ? 'exception' : task.status === 'success' ? 'success' : undefined" />
        <small>{{ task.errorMessage || (task.status === 'success' ? '上传完成' : task.status === 'waiting' ? '等待上传' : '正在上传') }}</small>
      </li>
    </ul>
  </section>
</template>

<style scoped>
.upload-panel { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 18px 28px; padding: 24px; border: 1px solid #e2e7ee; border-radius: 16px; background: #fff; box-shadow: 0 14px 38px rgba(39,55,84,.07); }
.upload-panel h2 { margin: 5px 0; }
.upload-panel p { margin: 0; color: #78859a; }
.file-picker { align-self: center; padding: 11px 18px; border-radius: 9px; background: #315ea8; color: #fff; font-weight: 700; cursor: pointer; }
.file-picker input { position: absolute; width: 1px; height: 1px; opacity: 0; }
.upload-tasks { grid-column: 1 / -1; display: grid; gap: 10px; margin: 0; padding: 0; list-style: none; }
.upload-task { padding: 12px 14px; border-radius: 10px; background: #f7f9fc; }
.upload-task[data-status='failed'] { background: #fff5f5; }
.task-title { display: flex; justify-content: space-between; gap: 16px; margin-bottom: 6px; }
.task-title strong { overflow-wrap: anywhere; }
.task-title span, small { color: #8993a4; font-size: 12px; }
@media (max-width: 680px) { .upload-panel { grid-template-columns: 1fr; } .file-picker { justify-self: start; } }
</style>
