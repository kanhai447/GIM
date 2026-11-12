import { describe, expect, it } from 'vitest'

import { isImageMessage, isValidImagePayload, messagePreviewText } from '@/utils/message'

describe('message helpers', () => {
  it('previews text, image and unsupported payloads', () => {
    expect(messagePreviewText({ msg: { type: 1, textMsg: { content: 'hello' } } })).toBe('hello')
    expect(messagePreviewText({ msg: { type: 2, imageMsg: { fileID: 9 } } })).toBe('[图片]')
    expect(messagePreviewText({ msg: { type: 3 } })).toBe('[暂不支持的消息]')
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
})
