import axios, { type AxiosError, type AxiosRequestConfig } from 'axios'

import type { ApiEnvelope } from '@/types/api'
import { clearSession, readToken } from '@/utils/session'

const DEFAULT_TIMEOUT_MS = 10_000

export class PublicApiError extends Error {
  constructor(
    message: string,
    readonly code?: number,
    readonly status?: number,
  ) {
    super(message)
    this.name = 'PublicApiError'
  }
}

export function apiBaseURL(configured = import.meta.env.VITE_API_BASE_URL): string {
  const value = configured?.trim()
  return value ? value.replace(/\/$/, '') : '/'
}

function apiTimeout(configured = import.meta.env.VITE_API_TIMEOUT_MS): number {
  const parsed = Number(configured)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : DEFAULT_TIMEOUT_MS
}

function isEnvelope(value: unknown): value is ApiEnvelope<unknown> {
  if (typeof value !== 'object' || value === null) return false
  const candidate = value as Partial<ApiEnvelope<unknown>>
  return typeof candidate.code === 'number' && typeof candidate.msg === 'string' && 'data' in candidate
}

function publicMessage(message: string, fallback: string): string {
  const trimmed = message.trim()
  return trimmed && trimmed.length <= 120 ? trimmed : fallback
}

export function unwrapEnvelope<T>(envelope: ApiEnvelope<T>): T {
  if (!isEnvelope(envelope)) throw new PublicApiError('服务响应格式异常')
  if (envelope.code !== 0) {
    throw new PublicApiError(publicMessage(envelope.msg, '请求失败'), envelope.code)
  }
  return envelope.data
}

export function toPublicApiError(error: unknown): PublicApiError {
  if (error instanceof PublicApiError) return error
  if (!axios.isAxiosError(error)) return new PublicApiError('请求失败，请稍后重试')

  const axiosError = error as AxiosError<unknown>
  const status = axiosError.response?.status
  if (status === 401) {
    clearSession()
    return new PublicApiError('登录状态已失效，请重新登录', undefined, status)
  }
  if (axiosError.code === 'ECONNABORTED' || axiosError.code === 'ETIMEDOUT') {
    return new PublicApiError('请求超时，请稍后重试', undefined, status)
  }
  if (isEnvelope(axiosError.response?.data)) {
    const body = axiosError.response.data
    return new PublicApiError(publicMessage(body.msg, '请求失败'), body.code, status)
  }
  if (!axiosError.response) return new PublicApiError('网络连接失败，请检查网络后重试')
  return new PublicApiError('服务暂时不可用，请稍后重试', undefined, status)
}

export const apiClient = axios.create({
  baseURL: apiBaseURL(),
  timeout: apiTimeout(),
  headers: { 'Content-Type': 'application/json' },
})

apiClient.interceptors.request.use((config) => {
  const token = readToken()
  if (token) config.headers.set('token', token)
  return config
})

export async function apiRequest<T>(config: AxiosRequestConfig): Promise<T> {
  try {
    const response = await apiClient.request<ApiEnvelope<T>>(config)
    return unwrapEnvelope(response.data)
  } catch (error) {
    throw toPublicApiError(error)
  }
}
