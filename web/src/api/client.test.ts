import { describe, expect, it } from 'vitest'

import { apiBaseURL, PublicApiError, unwrapEnvelope } from '@/api/client'

describe('API envelope parsing', () => {
  it('returns typed data for successful responses', () => {
    expect(unwrapEnvelope({ code: 0, msg: '成功', data: { userID: 7 } })).toEqual({ userID: 7 })
  })

  it('maps business failures to a public error without raw objects', () => {
    expect(() => unwrapEnvelope({ code: 1201, msg: '账号或密码错误', data: null })).toThrowError(PublicApiError)
    try {
      unwrapEnvelope({ code: 1201, msg: '账号或密码错误', data: null })
    } catch (error) {
      expect(error).toMatchObject({ message: '账号或密码错误', code: 1201 })
    }
  })

  it('normalizes configurable Gateway base URLs', () => {
    expect(apiBaseURL('http://localhost:8080/')).toBe('http://localhost:8080')
    expect(apiBaseURL('')).toBe('/')
  })
})
