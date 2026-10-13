// @vitest-environment happy-dom

import { afterEach, describe, expect, it, vi } from 'vitest'

import { formatFileDate, formatFileSize, triggerBrowserDownload } from '@/utils/file'

describe('file utilities', () => {
  afterEach(() => vi.restoreAllMocks())

  it('formats byte sizes and rejects invalid values safely', () => {
    expect(formatFileSize(0)).toBe('0 B')
    expect(formatFileSize(1024)).toBe('1.00 KiB')
    expect(formatFileSize(12 * 1024 * 1024)).toBe('12.0 MiB')
    expect(formatFileSize(-1)).toBe('—')
    expect(formatFileDate('invalid')).toBe('—')
  })

  it('downloads through a temporary object URL and revokes it', () => {
    vi.useFakeTimers()
    const createObjectURL = vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:gim-test')
    const revokeObjectURL = vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => undefined)
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => undefined)
    triggerBrowserDownload(new Blob(['content']), '<report>.txt')
    expect(createObjectURL).toHaveBeenCalledOnce()
    expect(click).toHaveBeenCalledOnce()
    expect(document.querySelector('a')).toBeNull()
    vi.runAllTimers()
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:gim-test')
    vi.useRealTimers()
  })
})
