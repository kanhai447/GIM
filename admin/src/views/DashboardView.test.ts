import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

describe('Admin dashboard contract', () => {
  it('loads the real overview API without FIM mock routes', () => {
    const source = readFileSync(fileURLToPath(new URL('./DashboardView.vue', import.meta.url)), 'utf8')
    expect(source).toContain('adminApi.overview()')
    expect(source).not.toMatch(/['"]\/api\/data\/(?:weather|statistic|login_statistic)/)
    expect(source).toContain('summary.privateMessages')
    expect(source).toContain('summary.auditEvents')
  })
})
