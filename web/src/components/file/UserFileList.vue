<script setup lang="ts">
import type { UserFileRecord } from '@/types/file'
import { formatFileDate, formatFileSize } from '@/utils/file'

defineProps<{ files: UserFileRecord[]; loading: boolean; loadingMore: boolean; hasMore: boolean }>()
defineEmits<{ download: [file: UserFileRecord]; remove: [file: UserFileRecord]; loadMore: [] }>()
</script>

<template>
  <section class="file-list-panel">
    <header><div><p class="eyebrow">My files</p><h2>我的文件</h2></div><span>仅展示当前账号拥有的文件引用</span></header>
    <div v-if="loading" class="empty-state">正在加载文件…</div>
    <div v-else-if="!files.length" class="empty-state"><strong>还没有文件</strong><span>选择文件后会在这里显示上传记录。</span></div>
    <ul v-else class="file-list">
      <li v-for="file in files" :key="file.id" class="file-row" :data-file-id="file.id">
        <div class="file-main"><strong class="file-name">{{ file.fileName }}</strong><span>{{ file.mimeType }} · {{ formatFileSize(file.size) }}</span></div>
        <code :title="file.sha256">{{ file.sha256.slice(0, 12) }}…</code>
        <time :datetime="file.createdAt">{{ formatFileDate(file.createdAt) }}</time>
        <div class="file-actions"><el-button size="small" @click="$emit('download', file)">下载</el-button><el-button size="small" type="danger" plain @click="$emit('remove', file)">删除</el-button></div>
      </li>
    </ul>
    <div v-if="files.length" class="more-row"><el-button v-if="hasMore" :loading="loadingMore" @click="$emit('loadMore')">加载更多</el-button><span v-else>已显示全部文件</span></div>
  </section>
</template>

<style scoped>
.file-list-panel { padding: 22px 24px; border: 1px solid #e2e7ee; border-radius: 16px; background: #fff; box-shadow: 0 14px 38px rgba(39,55,84,.07); }
header { display: flex; align-items: end; justify-content: space-between; gap: 20px; padding-bottom: 16px; border-bottom: 1px solid #edf0f4; }
header h2, header p { margin: 0; } header span { color: #8993a4; font-size: 12px; }
.file-list { margin: 0; padding: 0; list-style: none; }
.file-row { display: grid; grid-template-columns: minmax(200px, 1fr) 130px 165px auto; align-items: center; gap: 18px; padding: 15px 0; border-bottom: 1px solid #edf0f4; }
.file-main { min-width: 0; display: flex; flex-direction: column; gap: 5px; }
.file-name { overflow-wrap: anywhere; } .file-main span, time, code { color: #7b8798; font-size: 12px; }
code { overflow: hidden; text-overflow: ellipsis; } .file-actions { display: flex; }
.empty-state { min-height: 180px; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 7px; color: #8993a4; }
.empty-state strong { color: #45536a; }
.more-row { display: flex; justify-content: center; padding-top: 18px; color: #9aa4b4; font-size: 12px; }
@media (max-width: 950px) { .file-row { grid-template-columns: minmax(0, 1fr) auto; } .file-row code, .file-row time { display: none; } }
</style>
