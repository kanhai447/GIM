import assert from 'node:assert/strict'
import { access } from 'node:fs/promises'

import { chromium } from 'playwright-core'

for (const name of ['GIM_E2E_ADMIN_ACCOUNT', 'GIM_E2E_ADMIN_PASSWORD']) {
  if (!process.env[name]) throw new Error(`missing required environment variable ${name}`)
}

const baseURL = process.env.GIM_E2E_ADMIN_URL || 'http://127.0.0.1:4174'
const browserExecutable = process.env.GIM_E2E_BROWSER || 'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe'
await access(browserExecutable)

const browser = await chromium.launch({ executablePath: browserExecutable, headless: true })
const context = await browser.newContext()
const page = await context.newPage()
const failed = []
page.on('response', (response) => {
  if (new URL(response.url()).pathname.startsWith('/api/') && response.status() >= 400) {
    failed.push(`${response.status()} ${new URL(response.url()).pathname}`)
  }
})

try {
  await page.goto(`${baseURL}/login?redirect=/admin`)
  await page.locator('input[autocomplete="username"]').fill(process.env.GIM_E2E_ADMIN_ACCOUNT)
  await page.locator('input[autocomplete="current-password"]').fill(process.env.GIM_E2E_ADMIN_PASSWORD)
  await page.locator('button[type="submit"]').click()
  await page.waitForURL((url) => url.pathname === '/admin', { timeout: 15_000 })
  await page.getByRole('heading', { name: '欢迎进入管理控制台' }).waitFor()

  await page.goto(`${baseURL}/admin/users`)
  await page.getByRole('heading', { name: '用户管理' }).waitFor()
  await page.locator('.arco-table').waitFor()

  await page.goto(`${baseURL}/admin/logs`)
  await page.getByRole('heading', { name: '结构化审计日志' }).waitFor()
  await page.locator('.arco-table').waitFor()

  assert.deepEqual(failed, [], `admin API failures: ${failed.join(', ')}`)
  console.log(JSON.stringify({ result: 'PASS', dashboard: true, users: true, logs: true }))
} finally {
  await context.close()
  await browser.close()
}
