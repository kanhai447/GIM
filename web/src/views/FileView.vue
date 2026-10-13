<script setup lang="ts">
import { onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'

import FileUploadPanel from '@/components/file/FileUploadPanel.vue'
import UserFileList from '@/components/file/UserFileList.vue'
import { useFileStore } from '@/stores/file'
import type { UserFileRecord } from '@/types/file'

const fileStore = useFileStore()

onMounted(() => { void fileStore.loadInitial() })

async function download(file: UserFileRecord): Promise<void> {
  if (await fileStore.download(file)) ElMessage.success('下载已开始')
  else ElMessage.error(fileStore.lastError)
}

async function remove(file: UserFileRecord): Promise<void> {
  try {
    await ElMessageBox.confirm(`确认删除“${file.fileName}”的当前账号文件记录？共享内容不会被误删。`, '删除文件', { type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消' })
  } catch {
    return
  }
  if (await fileStore.remove(file)) ElMessage.success('文件记录已删除')
  else ElMessage.error(fileStore.lastError)
}
</script>

<template>
  <main class="file-page">
    <FileUploadPanel :tasks="fileStore.uploadTasks" @select="fileStore.upload" />
    <p v-if="fileStore.lastError" class="file-error" role="alert">{{ fileStore.lastError }}</p>
    <UserFileList
      :files="fileStore.files"
      :loading="fileStore.loading"
      :loading-more="fileStore.loadingMore"
      :has-more="fileStore.hasMore"
      @download="download"
      @remove="remove"
      @load-more="fileStore.loadMore"
    />
  </main>
</template>

<style scoped>
.file-page { display: grid; gap: 20px; max-width: 1180px; margin: 0 auto; }
.file-error { margin: 0; padding: 10px 14px; border-radius: 8px; background: #fff3f3; color: #b33c3c; font-size: 13px; }
</style>
