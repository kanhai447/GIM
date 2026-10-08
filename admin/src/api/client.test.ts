import { describe, expect, it } from 'vitest'

import { apiBaseURL, PublicApiError, unwrapEnvelope } from '@/api/client'

describe('Admin API foundation', () => {
  it('unwraps successful envelopes', () => {
    expect(unwrapEnvelope({ code: 0, msg: '成功', data: ['users'] })).toEqual(['users'])
  })

  it('surfaces safe business messages', () => {
    expect(() => unwrapEnvelope({ code: 1202, msg: '用户已禁用', data: null })).toThrowError(PublicApiError)
  })

  it('normalizes the configurable Gateway base URL', () => {
    expect(apiBaseURL('http://localhost:8080/')).toBe('http://localhost:8080')
  })
})
