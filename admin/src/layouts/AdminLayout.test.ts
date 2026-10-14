import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

function source(relativePath: string): string {
  return readFileSync(fileURLToPath(new URL(relativePath, import.meta.url)), 'utf8')
}

describe('Admin table layout contract', () => {
  it('allows the fixed-sidebar main layout and content to shrink to the viewport', () => {
    const layout = source('./AdminLayout.vue')
    expect(layout).toMatch(/\.admin-shell\s*>\s*\.arco-layout\s*\{[^}]*min-width:\s*0/)
    expect(layout).toMatch(/\.admin-content\s*\{[^}]*min-width:\s*0/)
  })

  it('constrains shared page cards and Arco tables to the available content width', () => {
    const css = source('../styles/base.css')
    expect(css).toMatch(/\.page-stack\s*\{[^}]*min-width:\s*0/)
    expect(css).toContain('.page-stack .arco-table-container { min-width: 0; max-width: 100%; }')
  })

  it('keeps the Users table horizontal-scroll contract for its fixed action column', () => {
    const users = source('../views/UsersView.vue')
    expect(users).toContain(':scroll="{x:1500}"')
    expect(users).toContain('fixed="right"')
  })
})
