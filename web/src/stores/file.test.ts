// @vitest-environment happy-dom

import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { watch } from 'vue'

import { PublicApiError } from '@/api/client'
import type { UserFileRecord } from '@/types/file'
import { MAX_UPLOAD_BYTES } from '@/utils/file'

const harness = vi.hoisted(() => ({
  list: vi.fn(), detail: vi.fn(), upload: vi.fn(), download: vi.fn(), remove: vi.fn(), trigger: vi.fn(),
}))

vi.mock('@/api/file', () => ({ fileApi: {
  list: harness.list, detail: harness.detail, upload: harness.upload, download: harness.download, remove: harness.remove,
} }))
vi.mock('@/utils/file', async (importOriginal) => ({
  ...await importOriginal<typeof import('@/utils/file')>(),
  triggerBrowserDownload: harness.trigger,
}))

import { useFileStore } from '@/stores/file'

function record(id: number, fileName = `file-${id}.txt`): UserFileRecord {
  return { id, uid: `00000000-0000-4000-8000-${String(id).padStart(12, '0')}`, fileName, size: id, mimeType: 'text/plain', sha256: String(id).padStart(64, '0'), url: `/api/file/${id}`, createdAt: '2026-10-13T00:00:00Z' }
}

describe('file store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    for (const mock of Object.values(harness)) mock.mockReset()
    harness.list.mockResolvedValue({ list: [record(3), record(2)], nextBeforeId: 2 })
    harness.upload.mockImplementation(async (file: File, onProgress: (value: number) => void) => {
      onProgress(42)
      return { ...record(4, file.name), objectCreated: true, referenceCreated: true, src: '/api/file/4' }
    })
    harness.download.mockResolvedValue(new Blob(['ok']))
    harness.remove.mockResolvedValue({ deleted: true })
  })

  it('loads the first page and merges cursor pages without duplicates', async () => {
    const store = useFileStore()
    harness.list.mockResolvedValueOnce({ list: Array.from({ length: 20 }, (_, index) => record(21 - index)), nextBeforeId: 2 })
    await store.loadInitial()
    harness.list.mockResolvedValueOnce({ list: [record(2), record(1)], nextBeforeId: 1 })
    await store.loadMore()
    expect(harness.list).toHaveBeenLastCalledWith(2, 20)
    expect(store.files.map((file) => file.id)).toEqual(Array.from({ length: 21 }, (_, index) => 21 - index))
  })

  it('keeps multiple upload tasks independent and reconciles duplicate UserFile ids', async () => {
    const store = useFileStore()
    const observed: string[] = []
    const stop = watch(() => store.uploadTasks.map((task) => task.status).join(','), (value) => observed.push(value), { flush: 'sync' })
    await store.upload([new File(['a'], 'one.txt'), new File(['b'], 'two.txt')])
    stop()
    expect(store.uploadTasks).toHaveLength(2)
    expect(store.uploadTasks.every((task) => task.status === 'success' && task.progress === 100)).toBe(true)
    expect(observed.some((value) => value.includes('uploading'))).toBe(true)
    expect(observed.at(-1)).toBe('success,success')
    expect(store.files.filter((file) => file.id === 4)).toHaveLength(1)
  })

  it('rejects oversized files before calling the API', async () => {
    const store = useFileStore()
    const oversized = { name: 'too-large.bin', size: MAX_UPLOAD_BYTES + 1 } as File
    await store.upload([oversized])
    expect(harness.upload).not.toHaveBeenCalled()
    expect(store.uploadTasks[0]).toMatchObject({ status: 'failed', progress: 0, errorMessage: '文件超过 50 MiB 限制，未开始上传' })
  })

  it('downloads by blob and removes only the selected UserFile reference', async () => {
    const store = useFileStore()
    await store.loadInitial()
    expect(await store.download(store.files[0]!)).toBe(true)
    expect(harness.trigger).toHaveBeenCalledWith(expect.any(Blob), 'file-3.txt')
    expect(await store.remove(store.files[0]!)).toBe(true)
    expect(harness.remove).toHaveBeenCalledWith(3)
    expect(store.files.map((file) => file.id)).toEqual([2])
  })

  it('uses safe ownership errors and delegates terminal authentication failure', async () => {
    const onAuthFailure = vi.fn()
    const store = useFileStore()
    store.start(onAuthFailure)
    harness.download.mockRejectedValueOnce(new PublicApiError('raw detail', 2503))
    expect(await store.download(record(8))).toBe(false)
    expect(store.lastError).toBe('无权访问或文件不存在')
    harness.list.mockRejectedValueOnce(new PublicApiError('expired', 1205, 401))
    await store.loadInitial()
    expect(onAuthFailure).toHaveBeenCalledOnce()
  })
})
