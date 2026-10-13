import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { fileApi } from '@/api/file'
import { PublicApiError } from '@/api/client'
import type { FileUploadTask, UserFileRecord } from '@/types/file'
import { MAX_UPLOAD_BYTES, triggerBrowserDownload } from '@/utils/file'

const PAGE_SIZE = 20
const AUTH_ERROR_CODES = new Set([1203, 1204, 1205, 1206])

function taskID(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') return crypto.randomUUID()
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`
}

function errorMessage(error: unknown, fallback: string): string {
  if (!(error instanceof PublicApiError)) return fallback
  if (error.code === 2502) return '文件超过 50 MiB 限制'
  if (error.code === 2503) return '无权访问或文件不存在'
  if (error.code === 2504) return '文件服务暂不可用，请稍后重试'
  return error.message
}

export const useFileStore = defineStore('file', () => {
  const files = ref<UserFileRecord[]>([])
  const uploadTasks = ref<FileUploadTask[]>([])
  const loading = ref(false)
  const loadingMore = ref(false)
  const nextBeforeId = ref<number | undefined>()
  const reachedEnd = ref(false)
  const lastError = ref('')
  let authFailureHandler: (() => void | Promise<void>) | null = null

  const hasMore = computed(() => !reachedEnd.value && nextBeforeId.value !== undefined)

  function start(onAuthenticationFailure?: () => void | Promise<void>): void {
    authFailureHandler = onAuthenticationFailure ?? null
  }

  function stop(): void {
    files.value = []
    uploadTasks.value = []
    loading.value = false
    loadingMore.value = false
    nextBeforeId.value = undefined
    reachedEnd.value = false
    lastError.value = ''
    authFailureHandler = null
  }

  function isAuthenticationFailure(error: unknown): boolean {
    return error instanceof PublicApiError && (error.status === 401 || (error.code !== undefined && AUTH_ERROR_CODES.has(error.code)))
  }

  function handleFailure(error: unknown, fallback: string): string {
    if (isAuthenticationFailure(error)) {
      void authFailureHandler?.()
      return '登录状态已失效，请重新登录'
    }
    return errorMessage(error, fallback)
  }

  function mergeFiles(incoming: UserFileRecord[], prepend: boolean): void {
    const merged = new Map<number, UserFileRecord>()
    for (const file of prepend ? [...incoming, ...files.value] : [...files.value, ...incoming]) merged.set(file.id, file)
    files.value = [...merged.values()].sort((left, right) => right.id - left.id)
  }

  async function loadInitial(): Promise<void> {
    if (loading.value) return
    loading.value = true
    try {
      const page = await fileApi.list(undefined, PAGE_SIZE)
      files.value = [...page.list].sort((left, right) => right.id - left.id)
      nextBeforeId.value = page.nextBeforeId || undefined
      reachedEnd.value = page.list.length < PAGE_SIZE || !page.nextBeforeId
      lastError.value = ''
    } catch (error) {
      lastError.value = handleFailure(error, '文件列表加载失败')
    } finally {
      loading.value = false
    }
  }

  async function loadMore(): Promise<void> {
    if (loadingMore.value || !hasMore.value || !nextBeforeId.value) return
    loadingMore.value = true
    try {
      const page = await fileApi.list(nextBeforeId.value, PAGE_SIZE)
      mergeFiles(page.list, false)
      nextBeforeId.value = page.nextBeforeId || undefined
      reachedEnd.value = page.list.length < PAGE_SIZE || !page.nextBeforeId
      lastError.value = ''
    } catch (error) {
      lastError.value = handleFailure(error, '更多文件加载失败')
    } finally {
      loadingMore.value = false
    }
  }

  async function upload(selected: File[]): Promise<void> {
    const jobs = selected.map((file) => uploadOne(file))
    await Promise.all(jobs)
  }

  function updateTask(id: string, patch: Partial<FileUploadTask>): void {
    const index = uploadTasks.value.findIndex((item) => item.id === id)
    if (index < 0) return
    uploadTasks.value[index] = { ...uploadTasks.value[index]!, ...patch }
  }

  async function uploadOne(file: File): Promise<void> {
    const id = taskID()
    const task: FileUploadTask = { id, fileName: file.name, size: file.size, progress: 0, status: 'waiting' }
    uploadTasks.value.unshift(task)
    if (file.size > MAX_UPLOAD_BYTES) {
      updateTask(id, { status: 'failed', errorMessage: '文件超过 50 MiB 限制，未开始上传' })
      return
    }
    updateTask(id, { status: 'uploading' })
    try {
      const uploaded = await fileApi.upload(file, (progress) => { updateTask(id, { progress }) })
      updateTask(id, { progress: 100, status: 'success', userFileID: uploaded.id })
      mergeFiles([uploaded], true)
      lastError.value = ''
    } catch (error) {
      const message = handleFailure(error, '上传失败，请稍后重试')
      updateTask(id, { status: 'failed', errorMessage: message })
      lastError.value = message
    }
  }

  async function download(file: UserFileRecord): Promise<boolean> {
    try {
      const blob = await fileApi.download(file.id)
      triggerBrowserDownload(blob, file.fileName)
      lastError.value = ''
      return true
    } catch (error) {
      lastError.value = handleFailure(error, '下载失败，请稍后重试')
      return false
    }
  }

  async function remove(file: UserFileRecord): Promise<boolean> {
    try {
      await fileApi.remove(file.id)
      files.value = files.value.filter((item) => item.id !== file.id)
      lastError.value = ''
      return true
    } catch (error) {
      lastError.value = handleFailure(error, '删除失败，请稍后重试')
      return false
    }
  }

  return {
    files, uploadTasks, loading, loadingMore, hasMore, lastError,
    start, stop, loadInitial, loadMore, upload, download, remove,
  }
})
