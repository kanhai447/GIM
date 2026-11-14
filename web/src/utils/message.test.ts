import { describe, expect, it } from 'vitest'

import { formatFileSize, isFileMessage, isImageMessage, isValidFilePayload, isValidImagePayload, messagePreviewText } from '@/utils/message'

describe('message helpers', () => {
  it('previews text, image, file and unsupported payloads', () => {
    expect(messagePreviewText({ msg: { type: 1, textMsg: { content: 'hello' } } })).toBe('hello')
    expect(messagePreviewText({ msg: { type: 2, imageMsg: { fileID: 9 } } })).toBe('[图片]')
    expect(messagePreviewText({ msg: { type: 3, fileMsg: { fileID: 9 } } })).toBe('[文件]')
    expect(messagePreviewText({ msg: { type: 4 } })).toBe('[暂不支持的消息]')
    expect(messagePreviewText(undefined)).toBe('暂无消息')
  })

  it('recognizes image payloads and validates untrusted metadata', () => {
    expect(isImageMessage({ type: 2, imageMsg: { fileID: 9 } })).toBe(true)
    expect(isImageMessage({ type: 2, imageMsg: { fileID: 0 } })).toBe(false)
    expect(isImageMessage({ type: 1, textMsg: { content: 'x' } })).toBe(false)

    expect(isValidImagePayload({ fileID: 9 })).toBe(true)
    expect(isValidImagePayload({ fileID: 9, width: 800, height: 600, size: 1024, url: '/api/file/9', mimeType: 'image/png' })).toBe(true)
    expect(isValidImagePayload({ fileID: 0 })).toBe(false)
    expect(isValidImagePayload({ fileID: 9, width: -1 })).toBe(false)
    expect(isValidImagePayload({ fileID: 9, size: 1.5 })).toBe(false)
    expect(isValidImagePayload({ fileID: 9, mimeType: 5 })).toBe(false)
    expect(isValidImagePayload(null)).toBe(false)
  })

  it('recognizes file payloads and formats sizes', () => {
    expect(isFileMessage({ type: 3, fileMsg: { fileID: 9 } })).toBe(true)
    expect(isFileMessage({ type: 2, imageMsg: { fileID: 9 } })).toBe(false)
    expect(isFileMessage({ type: 3, fileMsg: { fileID: 0 } })).toBe(false)

    expect(isValidFilePayload({ fileID: 9 })).toBe(true)
    expect(isValidFilePayload({ fileID: 9, fileName: 'report.pdf', size: 2048, mimeType: 'application/pdf' })).toBe(true)
    expect(isValidFilePayload({ fileID: 0 })).toBe(false)
    expect(isValidFilePayload({ fileID: 9, size: -1 })).toBe(false)
    expect(isValidFilePayload({ fileID: 9, fileName: 5 })).toBe(false)
    expect(isValidFilePayload(undefined)).toBe(false)

    expect(formatFileSize(512)).toBe('512 B')
    expect(formatFileSize(2048)).toBe('2.0 KB')
    expect(formatFileSize(5 * 1024 * 1024)).toBe('5.0 MB')
    expect(formatFileSize(undefined)).toBe('')
  })
})
