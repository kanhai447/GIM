import { beforeEach, describe, expect, it, vi } from 'vitest'

const harness = vi.hoisted(() => ({ request: vi.fn(), apiRequest: vi.fn() }))

vi.mock('@/api/client', () => ({
  apiClient: { request: harness.request },
  apiRequest: harness.apiRequest,
  PublicApiError: class PublicApiError extends Error {
    constructor(message: string, readonly code?: number, readonly status?: number) { super(message) }
  },
  toPublicApiError: (error: unknown) => error,
}))

import { fileApi } from '@/api/file'

describe('file API', () => {
  beforeEach(() => {
    harness.request.mockReset()
    harness.apiRequest.mockReset()
  })

  it('uses the user-owned cursor list and delete endpoints', async () => {
    harness.apiRequest.mockResolvedValue({ list: [], nextBeforeId: 0 })
    await fileApi.list(22, 20)
    expect(harness.apiRequest).toHaveBeenCalledWith({ method: 'GET', url: '/api/file/user-files', params: { beforeId: 22, limit: 20 } })
    await fileApi.remove(9)
    expect(harness.apiRequest).toHaveBeenLastCalledWith({ method: 'DELETE', url: '/api/file/user-files/9' })
  })

  it('sends multipart file uploads with progress', async () => {
    harness.apiRequest.mockImplementation(async (config) => {
      config.onUploadProgress({ loaded: 5, total: 10 })
      expect(config.data).toBeInstanceOf(FormData)
      return { id: 1 }
    })
    const progress = vi.fn()
    await fileApi.upload(new File(['0123456789'], 'safe.txt'), progress)
    expect(harness.apiRequest).toHaveBeenCalledWith(expect.objectContaining({ method: 'POST', url: '/api/file/file', timeout: 120_000 }))
    expect(progress).toHaveBeenCalledWith(50)
  })

  it('downloads an authenticated blob through the shared client', async () => {
    const blob = new Blob(['download'])
    harness.request.mockResolvedValue({ data: blob, status: 200, headers: { 'content-disposition': 'attachment; filename="safe.txt"' } })
    await expect(fileApi.download(17)).resolves.toBe(blob)
    expect(harness.request).toHaveBeenCalledWith({ method: 'GET', url: '/api/file/17', responseType: 'blob', timeout: 120_000 })
  })

  it('rejects a 200 business-error envelope instead of downloading it as a file', async () => {
    harness.request.mockResolvedValue({
      data: new Blob([JSON.stringify({ code: 2503, msg: '文件不存在', data: null })], { type: 'application/json' }),
      status: 200,
      headers: { 'content-type': 'application/json' },
    })
    await expect(fileApi.download(99)).rejects.toMatchObject({ code: 2503, message: '文件不存在' })
  })
})
