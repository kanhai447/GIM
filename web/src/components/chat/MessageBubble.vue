<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'

import { fileApi } from '@/api/file'
import { formatFileSize, type RenderableMessagePayload } from '@/utils/message'

const props = defineProps<{ msg: RenderableMessagePayload }>()

const source = ref('')
const loading = ref(false)
const failed = ref(false)
const preview = ref(false)
const downloading = ref(false)
const downloadFailed = ref(false)
let objectURL = ''

function releaseObjectURL(): void {
  if (objectURL) {
    URL.revokeObjectURL(objectURL)
    objectURL = ''
  }
}

async function loadImage(fileID: number): Promise<void> {
  releaseObjectURL()
  source.value = ''
  failed.value = false
  loading.value = true
  try {
    const blob = await fileApi.download(fileID)
    objectURL = URL.createObjectURL(blob)
    source.value = objectURL
  } catch {
    failed.value = true
  } finally {
    loading.value = false
  }
}

function retry(): void {
  const fileID = props.msg.imageMsg?.fileID
  if (fileID) void loadImage(fileID)
}

async function downloadFile(): Promise<void> {
  const file = props.msg.fileMsg
  if (!file || downloading.value) return
  downloading.value = true
  downloadFailed.value = false
  try {
    const blob = await fileApi.download(file.fileID)
    const url = URL.createObjectURL(blob)
    const anchor = document.createElement('a')
    anchor.href = url
    anchor.download = file.fileName || `file-${file.fileID}`
    document.body.appendChild(anchor)
    anchor.click()
    anchor.remove()
    URL.revokeObjectURL(url)
  } catch {
    downloadFailed.value = true
  } finally {
    downloading.value = false
  }
}

watch(
  () => props.msg.imageMsg?.fileID,
  (fileID) => {
    if (props.msg.type === 2 && fileID) void loadImage(fileID)
  },
  { immediate: true },
)

onBeforeUnmount(() => {
  releaseObjectURL()
})
</script>

<template>
  <span v-if="msg.type === 1" class="text-content">{{ msg.textMsg?.content }}</span>
  <span v-else-if="msg.type === 2" class="image-content">
    <img
      v-if="source"
      class="image-thumb"
      :src="source"
      alt="图片消息"
      data-testid="image-thumb"
      @click="preview = true"
    />
    <button v-else-if="failed" type="button" class="image-failed" @click="retry">图片加载失败，点击重试</button>
    <span v-else class="image-loading">图片加载中…</span>
  </span>
  <span v-else-if="msg.type === 3 && msg.fileMsg" class="file-content">
    <span class="file-icon" aria-hidden="true">📄</span>
    <span class="file-info">
      <strong class="file-name">{{ msg.fileMsg.fileName || `文件 ${msg.fileMsg.fileID}` }}</strong>
      <span class="file-size">{{ formatFileSize(msg.fileMsg.size) }}</span>
    </span>
    <button type="button" class="file-download" :disabled="downloading" @click="downloadFile">
      {{ downloading ? '下载中…' : downloadFailed ? '重试下载' : '下载' }}
    </button>
  </span>
  <span v-else-if="msg.type === 4" class="withdrawn">消息已撤回</span>
  <span v-else class="unsupported">暂不支持的消息</span>

  <div v-if="preview && source" class="image-preview" role="dialog" aria-modal="true" @click.self="preview = false">
    <img :src="source" alt="图片预览" data-testid="image-preview" />
    <button type="button" class="preview-close" @click="preview = false">关闭</button>
  </div>
</template>

<style scoped>
.text-content { white-space: pre-wrap; }
.image-content { display: block; }
.image-thumb { display: block; max-width: min(320px, 60vw); max-height: 320px; border-radius: 10px; cursor: zoom-in; object-fit: cover; }
.image-loading, .image-failed { display: inline-block; padding: 10px 12px; border: 1px dashed #c9d4e4; border-radius: 10px; color: #7d8899; font-size: 12px; }
.image-failed { border-color: #e2b4b4; color: #c05353; cursor: pointer; background: #fff7f7; }
.unsupported { color: #8a95a6; font-size: 12px; }
.withdrawn { color: #8a95a6; font-size: 12px; font-style: italic; }
.file-content { display: flex; gap: 10px; align-items: center; min-width: 220px; padding: 10px 12px; border: 1px solid #dfe6f0; border-radius: 12px; background: #fff; box-shadow: 0 5px 14px rgba(40, 57, 86, .05); }
.file-icon { font-size: 22px; }
.file-info { display: flex; flex-direction: column; min-width: 0; flex: 1; }
.file-name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: #25324a; font-size: 13px; }
.file-size { color: #8a95a6; font-size: 11px; }
.file-download { border: 1px solid #315ea8; border-radius: 8px; padding: 5px 10px; background: #315ea8; color: #fff; font-size: 12px; cursor: pointer; }
.file-download:disabled { opacity: .6; cursor: default; }
.image-preview { position: fixed; inset: 0; z-index: 3000; display: grid; place-items: center; gap: 12px; padding: 32px; background: rgba(15, 23, 42, .82); }
.image-preview img { max-width: min(90vw, 1200px); max-height: 82vh; border-radius: 8px; box-shadow: 0 24px 70px rgba(0, 0, 0, .4); }
.preview-close { position: fixed; top: 20px; right: 24px; border: 1px solid rgba(255, 255, 255, .5); border-radius: 8px; padding: 6px 14px; background: rgba(255, 255, 255, .12); color: #fff; cursor: pointer; }
</style>
