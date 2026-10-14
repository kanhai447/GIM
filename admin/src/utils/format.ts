export function formatDate(value: string): string {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString('zh-CN', { hour12: false })
}

export function formatBytes(value: number): string {
  if (!Number.isFinite(value) || value < 0) return '-'
  const units = ['B', 'KiB', 'MiB', 'GiB']
  let size = value; let index = 0
  while (size >= 1024 && index < units.length - 1) { size /= 1024; index++ }
  return `${size.toFixed(index === 0 ? 0 : 1)} ${units[index]}`
}

export function safeJSON(value: unknown): string {
  try { return JSON.stringify(value ?? {}, null, 2) } catch { return '{}' }
}

export function optionalPositiveInt(value: string): number | undefined {
  if (value.trim() === '') return undefined
  const parsed = Number(value)
  return Number.isSafeInteger(parsed) && parsed > 0 ? parsed : undefined
}
