export interface RenderableImage {
  fileID: number
  url?: string
  width?: number
  height?: number
  size?: number
  mimeType?: string
}

export interface RenderableFile {
  fileID: number
  url?: string
  fileName?: string
  size?: number
  mimeType?: string
}

export interface RenderableWithdraw {
  msgID: number
}

export interface RenderableMessagePayload {
  type: number
  textMsg?: { content: string }
  imageMsg?: RenderableImage
  fileMsg?: RenderableFile
  withdrawMsg?: RenderableWithdraw
}

export function messagePreviewText(message: { msg?: RenderableMessagePayload } | undefined): string {
  const payload = message?.msg
  if (!payload) return '暂无消息'
  if (payload.type === 4) return '[撤回]'
  if (payload.type === 3) return '[文件]'
  if (payload.type === 2) return '[图片]'
  if (payload.type === 1) return payload.textMsg?.content ?? ''
  return '[暂不支持的消息]'
}

export const RECALL_WINDOW_MS = 120_000

export function canRecallMessage(
  message: { sendUserID: number; messageId?: number; msg: RenderableMessagePayload; createdAt: string },
  currentUserID: number,
  now = Date.now(),
): boolean {
  if (!message.messageId || message.sendUserID !== currentUserID) return false
  if (message.msg.type < 1 || message.msg.type > 3) return false
  const createdAt = Date.parse(message.createdAt)
  if (Number.isNaN(createdAt)) return false
  return now - createdAt <= RECALL_WINDOW_MS
}

export function isWithdrawMessage(payload: RenderableMessagePayload | undefined): payload is RenderableMessagePayload & { type: 4; withdrawMsg: RenderableWithdraw } {
  return payload?.type === 4 && typeof payload.withdrawMsg?.msgID === 'number' && payload.withdrawMsg.msgID > 0
}

export function isValidWithdrawPayload(value: unknown): value is RenderableWithdraw {
  if (typeof value !== 'object' || value === null) return false
  const withdraw = value as Record<string, unknown>
  return typeof withdraw.msgID === 'number' && Number.isSafeInteger(withdraw.msgID) && withdraw.msgID > 0
}

export function isImageMessage(payload: RenderableMessagePayload | undefined): payload is RenderableMessagePayload & { type: 2; imageMsg: RenderableImage } {
  return payload?.type === 2 && typeof payload.imageMsg?.fileID === 'number' && payload.imageMsg.fileID > 0
}

export function isFileMessage(payload: RenderableMessagePayload | undefined): payload is RenderableMessagePayload & { type: 3; fileMsg: RenderableFile } {
  return payload?.type === 3 && typeof payload.fileMsg?.fileID === 'number' && payload.fileMsg.fileID > 0
}

export function formatFileSize(size: number | undefined): string {
  if (typeof size !== 'number' || !Number.isFinite(size) || size < 0) return ''
  if (size < 1024) return `${size} B`
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`
  if (size < 1024 * 1024 * 1024) return `${(size / (1024 * 1024)).toFixed(1)} MB`
  return `${(size / (1024 * 1024 * 1024)).toFixed(2)} GB`
}

/** Validates an untrusted fileMsg object received over the WebSocket. */
export function isValidFilePayload(value: unknown): value is RenderableFile {
  if (typeof value !== 'object' || value === null) return false
  const file = value as Record<string, unknown>
  if (typeof file.fileID !== 'number' || !Number.isSafeInteger(file.fileID) || file.fileID <= 0) return false
  if (file.url !== undefined && typeof file.url !== 'string') return false
  if (file.fileName !== undefined && typeof file.fileName !== 'string') return false
  if (file.mimeType !== undefined && typeof file.mimeType !== 'string') return false
  if (file.size !== undefined && (typeof file.size !== 'number' || !Number.isSafeInteger(file.size) || file.size < 0)) return false
  return true
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
