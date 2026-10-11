import assert from 'node:assert/strict'
import { access } from 'node:fs/promises'

import { chromium } from 'playwright-core'

const required = ['GIM_E2E_A_ACCOUNT', 'GIM_E2E_B_ACCOUNT', 'GIM_E2E_PASSWORD', 'GIM_E2E_A_ID', 'GIM_E2E_B_ID']
for (const name of required) {
  if (!process.env[name]) throw new Error(`missing required environment variable ${name}`)
}

const baseURL = process.env.GIM_E2E_WEB_URL || 'http://localhost:5173'
const browserExecutable = process.env.GIM_E2E_BROWSER || 'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe'
await access(browserExecutable)

const accountA = process.env.GIM_E2E_A_ACCOUNT
const accountB = process.env.GIM_E2E_B_ACCOUNT
const password = process.env.GIM_E2E_PASSWORD
const userA = Number(process.env.GIM_E2E_A_ID)
const userB = Number(process.env.GIM_E2E_B_ID)
assert(Number.isSafeInteger(userA) && userA > 0)
assert(Number.isSafeInteger(userB) && userB > 0)

const browser = await chromium.launch({ executablePath: browserExecutable, headless: true })
const aliceContext = await browser.newContext()
const bobContext = await browser.newContext()
const alice = await aliceContext.newPage()
const bob = await bobContext.newPage()

async function login(page, account) {
  await page.goto(`${baseURL}/login`)
  await page.locator('input[autocomplete="username"]').fill(account)
  await page.locator('input[autocomplete="current-password"]').fill(password)
  await page.locator('button[type="submit"]').click()
  await page.waitForURL((url) => url.pathname === '/', { timeout: 15_000 })
}

async function waitConnected(page) {
  await page.locator('.connection-pill[data-state="connected"]').waitFor({ timeout: 15_000 })
}

async function sendAndWait(page, text) {
  await page.locator('.composer textarea').fill(text)
  await page.locator('.composer button', { hasText: '发送' }).click()
  const row = page.locator('.message').filter({ hasText: text }).last()
  await row.waitFor({ timeout: 10_000 })
  await page.waitForFunction(
    (content) => [...document.querySelectorAll('.message')].some((item) => item.textContent?.includes(content) && item.getAttribute('data-status') === 'sent'),
    text,
    { timeout: 10_000 },
  )
}

try {
  const runID = Date.now()
  await Promise.all([login(alice, accountA), login(bob, accountB)])

  await alice.goto(`${baseURL}/chat/${userB}`)
  await waitConnected(alice)
  await bob.goto(`${baseURL}/chat/${userA}`)
  await waitConnected(bob)
  await bob.goto(`${baseURL}/chat`)

  await alice.evaluate(() => {
    globalThis.__gimObservedStatuses = []
    const capture = () => {
      for (const item of document.querySelectorAll('.message[data-status]')) {
        const status = item.getAttribute('data-status')
        if (status) globalThis.__gimObservedStatuses.push(status)
      }
    }
    new MutationObserver(capture).observe(document.body, { childList: true, subtree: true, attributes: true, attributeFilter: ['data-status'] })
  })

  const firstText = `cp6-browser-${runID}`
  await sendAndWait(alice, firstText)
  const observedStatuses = await alice.evaluate(() => globalThis.__gimObservedStatuses)
  assert(observedStatuses.includes('pending'), 'sender UI never exposed pending state')
  assert(observedStatuses.includes('sent'), 'sender UI never reached sent state')

  const bobSession = bob.locator('.session-item').filter({ hasText: firstText })
  await bobSession.waitFor({ timeout: 10_000 })
  assert.equal(await bobSession.locator('.unread-badge').textContent(), '1')
  await bobSession.click()
  await bob.waitForURL((url) => url.pathname === `/chat/${userA}`)
  await bob.locator('.message').filter({ hasText: firstText }).waitFor({ timeout: 10_000 })
  await bob.locator('.unread-badge').waitFor({ state: 'detached', timeout: 10_000 })

  const bobDevice2 = await bobContext.newPage()
  await bobDevice2.goto(`${baseURL}/chat/${userA}`)
  await waitConnected(bobDevice2)
  const multiDeviceText = `cp6-multi-device-${runID}`
  await sendAndWait(alice, multiDeviceText)
  await Promise.all([
    bob.locator('.message').filter({ hasText: multiDeviceText }).waitFor({ timeout: 10_000 }),
    bobDevice2.locator('.message').filter({ hasText: multiDeviceText }).waitFor({ timeout: 10_000 }),
  ])

  for (let index = 0; index < 20; index += 1) {
    await sendAndWait(alice, `cp6-page-${runID}-${index.toString().padStart(2, '0')}`)
  }
  await bob.locator('.message').filter({ hasText: `cp6-page-${runID}-19` }).waitFor({ timeout: 15_000 })
  await bob.reload()
  await waitConnected(bob)
  await bob.locator('.older-row button', { hasText: '加载更早消息' }).waitFor({ timeout: 10_000 })
  const firstPageCount = await bob.locator('.message').count()
  assert.equal(firstPageCount, 20, 'latest history page must use the server limit')
  await bob.locator('.older-row button', { hasText: '加载更早消息' }).click()
  await bob.waitForFunction(() => document.querySelectorAll('.message').length > 20, undefined, { timeout: 10_000 })

  await alice.evaluate(() => {
    const pill = document.querySelector('.connection-pill')
    globalThis.__gimConnectionStates = pill?.getAttribute('data-state') ? [pill.getAttribute('data-state')] : []
    new MutationObserver(() => {
      const state = document.querySelector('.connection-pill')?.getAttribute('data-state')
      if (state) globalThis.__gimConnectionStates.push(state)
    }).observe(document.body, { childList: true, subtree: true, attributes: true, attributeFilter: ['data-state'] })
  })
  console.log('RECONNECT_READY')
  await alice.waitForFunction(() => globalThis.__gimConnectionStates.includes('reconnecting'), undefined, { timeout: 20_000 })
  await waitConnected(alice)
  const reconnectText = `cp6-reconnected-${runID}`
  await sendAndWait(alice, reconnectText)
  await bob.locator('.message').filter({ hasText: reconnectText }).waitFor({ timeout: 10_000 })

  console.log(JSON.stringify({
    result: 'PASS',
    twoUserRealtime: true,
    senderPendingToSent: true,
    unreadThenRead: true,
    refreshHistory: true,
    multiDeviceReceiver: true,
    cursorPagination: true,
    reconnect: true,
  }))
} finally {
  await aliceContext.close()
  await bobContext.close()
  await browser.close()
}
