export interface RenderableImage {
  fileID: number
  url?: string
  width?: number
  height?: number
  size?: number
  mimeType?: string
}

export interface RenderableMessagePayload {
  type: number
  textMsg?: { content: string }
  imageMsg?: RenderableImage
}

export function messagePreviewText(message: { msg?: RenderableMessagePayload } | undefined): string {
  const payload = message?.msg
  if (!payload) return '暂无消息'
  if (payload.type === 2) return '[图片]'
  if (payload.type === 1) return payload.textMsg?.content ?? ''
  return '[暂不支持的消息]'
}

export function isImageMessage(payload: RenderableMessagePayload | undefined): payload is RenderableMessagePayload & { type: 2; imageMsg: RenderableImage } {
  return payload?.type === 2 && typeof payload.imageMsg?.fileID === 'number' && payload.imageMsg.fileID > 0
}

/** Validates an untrusted imageMsg object received over the WebSocket. */
export function isValidImagePayload(value: unknown): value is RenderableImage {
  if (typeof value !== 'object' || value === null) return false
  const image = value as Record<string, unknown>
  if (typeof image.fileID !== 'number' || !Number.isSafeInteger(image.fileID) || image.fileID <= 0) return false
  if (image.url !== undefined && typeof image.url !== 'string') return false
  if (image.mimeType !== undefined && typeof image.mimeType !== 'string') return false
  for (const key of ['width', 'height', 'size'] as const) {
    const item = image[key]
    if (item !== undefined && (typeof item !== 'number' || !Number.isSafeInteger(item) || item < 0)) return false
  }
  return true
}
