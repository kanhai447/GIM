import axios, { type AxiosProgressEvent } from 'axios'

import { apiClient, apiRequest, PublicApiError, toPublicApiError } from '@/api/client'
import type { FileUploadResult, UserFilePage, UserFileRecord } from '@/types/file'

async function toDownloadError(error: unknown): Promise<PublicApiError> {
  if (axios.isAxiosError(error) && error.response?.data instanceof Blob) {
    try {
      const body = JSON.parse(await error.response.data.text()) as { code?: unknown; msg?: unknown }
      if (typeof body.code === 'number' && typeof body.msg === 'string') {
        const message = body.msg.trim()
        return new PublicApiError(message && message.length <= 120 ? message : '下载失败', body.code, error.response.status)
      }
    } catch {
      // A non-JSON download failure is mapped by the shared safe error policy below.
    }
  }
  return toPublicApiError(error)
}

async function downloadEnvelopeError(blob: Blob, status?: number): Promise<PublicApiError> {
  try {
    const body = JSON.parse(await blob.text()) as { code?: unknown; msg?: unknown }
    if (typeof body.code === 'number' && body.code !== 0 && typeof body.msg === 'string') {
      const message = body.msg.trim()
      return new PublicApiError(message && message.length <= 120 ? message : '下载失败', body.code, status)
    }
  } catch {
    // The response is neither an attachment nor a valid public API envelope.
  }
  return new PublicApiError('下载响应格式异常', undefined, status)
}

export const fileApi = {
  list(beforeId?: number, limit = 20): Promise<UserFilePage> {
    return apiRequest<UserFilePage>({ method: 'GET', url: '/api/file/user-files', params: { beforeId, limit } })
  },

  detail(id: number): Promise<UserFileRecord> {
    return apiRequest<UserFileRecord>({ method: 'GET', url: `/api/file/user-files/${id}` })
  },

  upload(file: File, onProgress?: (progress: number) => void): Promise<FileUploadResult> {
    const form = new FormData()
    form.append('file', file, file.name)
    return apiRequest<FileUploadResult>({
      method: 'POST',
      url: '/api/file/file',
      data: form,
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: 120_000,
      onUploadProgress: (event: AxiosProgressEvent) => {
        const total = event.total || file.size
        if (total > 0) onProgress?.(Math.min(99, Math.round((event.loaded / total) * 100)))
      },
    })
  },

  async download(id: number): Promise<Blob> {
    try {
      const response = await apiClient.request<Blob>({ method: 'GET', url: `/api/file/${id}`, responseType: 'blob', timeout: 120_000 })
      const disposition = String(response.headers['content-disposition'] ?? '')
      if (!/^attachment(?:;|$)/i.test(disposition.trim())) throw await downloadEnvelopeError(response.data, response.status)
      return response.data
    } catch (error) {
      throw await toDownloadError(error)
    }
  },

  remove(id: number): Promise<{ deleted: boolean }> {
    return apiRequest<{ deleted: boolean }>({ method: 'DELETE', url: `/api/file/user-files/${id}` })
  },
}
