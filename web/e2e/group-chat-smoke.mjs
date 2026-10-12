import assert from 'node:assert/strict'
import { access } from 'node:fs/promises'

import { chromium } from 'playwright-core'

const baseURL = process.env.GIM_E2E_WEB_URL || 'http://127.0.0.1:5173'
const browserExecutable = process.env.GIM_E2E_BROWSER || 'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe'
await access(browserExecutable)

const runID = Date.now()
const password = `Smoke-${runID}-Aa!`

async function register(label) {
  const account = `gim_cp4_${label}_${runID}`
  const response = await fetch(`${baseURL}/api/auth/register`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ account, nickname: `CP4 ${label}`, pwd: password, rePwd: password }),
  })
  const body = await response.json()
  assert.equal(body.code, 0, `register ${label} failed: ${body.msg}`)
  assert(Number.isSafeInteger(body.data.userID) && body.data.userID > 0)
  return { account, id: body.data.userID, nickname: body.data.nickname }
}

const [userA, userB, userC] = await Promise.all([register('A'), register('B'), register('C')])
const browser = await chromium.launch({ executablePath: browserExecutable, headless: true })
const contextA = await browser.newContext()
const contextB = await browser.newContext()
const contextC = await browser.newContext()
const pageA = await contextA.newPage()
const pageB = await contextB.newPage()
const pageC = await contextC.newPage()

function trackSockets(page) {
  const records = []
  page.on('websocket', (socket) => {
    const path = new URL(socket.url()).pathname
    if (path !== '/api/chat/ws/chat' && path !== '/api/group/ws/chat') return
    const record = { path, closed: false }
    socket.on('close', () => { record.closed = true })
    records.push(record)
  })
  return records
}

const socketsA = trackSockets(pageA)
const socketsB = trackSockets(pageB)
trackSockets(pageC)

async function login(page, user) {
  await page.goto(`${baseURL}/login`)
  await page.locator('input[autocomplete="username"]').fill(user.account)
  await page.locator('input[autocomplete="current-password"]').fill(password)
  await page.locator('button[type="submit"]').click()
  await page.waitForURL((url) => url.pathname === '/', { timeout: 15_000 })
}

async function waitGroupConnected(page) {
  await page.locator('.connection-pill[data-state="connected"]').waitFor({ timeout: 20_000 })
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
  assert.equal(await page.locator('.message .bubble').filter({ hasText: text }).count(), 1, `sender duplicated ${text}`)
}

async function openGroupFromSidebar(page, groupName) {
  const item = page.locator('.list-item').filter({ hasText: groupName }).first()
  await item.waitFor({ timeout: 15_000 })
  await item.click()
  await page.waitForURL((url) => /^\/group\/\d+$/.test(url.pathname), { timeout: 10_000 })
  await waitGroupConnected(page)
}

try {
  await Promise.all([login(pageA, userA), login(pageB, userB), login(pageC, userC)])
  await Promise.all([pageA.goto(`${baseURL}/group`), pageB.goto(`${baseURL}/group`), pageC.goto(`${baseURL}/group`)])

  const groupName = `CP4 Group ${runID}`
  await pageA.locator('.create-button').click()
  await pageA.locator('input[aria-label="群名称"]').fill(groupName)
  await pageA.locator('input[aria-label="邀请成员 ID"]').fill(`${userB.id},${userC.id}`)
  await pageA.locator('.create-form button[type="submit"]').click()
  await pageA.waitForURL((url) => /^\/group\/\d+$/.test(url.pathname), { timeout: 15_000 })
  const groupID = Number(new URL(pageA.url()).pathname.split('/').at(-1))
  assert(Number.isSafeInteger(groupID) && groupID > 0)
  await waitGroupConnected(pageA)

  await Promise.all([pageB.reload(), pageC.reload()])
  await Promise.all([pageB.locator('.list-item').filter({ hasText: groupName }).first().waitFor(), pageC.locator('.list-item').filter({ hasText: groupName }).first().waitFor()])

  await pageA.evaluate(() => {
    globalThis.__gimGroupStatuses = []
    const capture = () => {
      for (const item of document.querySelectorAll('.message[data-status]')) {
        const status = item.getAttribute('data-status')
        if (status) globalThis.__gimGroupStatuses.push(status)
      }
    }
    new MutationObserver(capture).observe(document.body, { childList: true, subtree: true, attributes: true, attributeFilter: ['data-status'] })
  })

  const hello = `hello-group-${runID}`
  await sendAndWait(pageA, hello)
  const statuses = await pageA.evaluate(() => globalThis.__gimGroupStatuses)
  assert(statuses.includes('pending'), 'sender never rendered pending')
  assert(statuses.includes('sent'), 'sender never rendered sent')

  const sessionB = pageB.locator('.list-item').filter({ hasText: hello }).first()
  const sessionC = pageC.locator('.list-item').filter({ hasText: hello }).first()
  await Promise.all([sessionB.waitFor({ timeout: 10_000 }), sessionC.waitFor({ timeout: 10_000 })])
  assert.equal(await sessionB.locator('.unread').textContent(), '1')
  assert.equal(await sessionC.locator('.unread').textContent(), '1')
  await sessionB.click()
  await pageB.waitForURL((url) => url.pathname === `/group/${groupID}`)
  await pageB.locator('.message').filter({ hasText: hello }).waitFor()
  await pageB.locator('.unread').waitFor({ state: 'detached', timeout: 10_000 })
  await pageB.reload()
  await waitGroupConnected(pageB)
  await pageB.locator('.message').filter({ hasText: hello }).waitFor()

  const peerRow = pageA.locator('.info-panel li').filter({ hasText: userB.nickname })
  await peerRow.locator('button', { hasText: '禁言' }).click()
  const blocked = `muted-${runID}`
  await pageB.locator('.composer textarea').fill(blocked)
  await pageB.locator('.composer button', { hasText: '发送' }).click()
  const blockedRow = pageB.locator('.message').filter({ hasText: blocked }).last()
  await blockedRow.waitFor()
  await pageB.waitForFunction((text) => [...document.querySelectorAll('.message')].some((item) => item.textContent?.includes(text) && item.getAttribute('data-status') === 'failed'), blocked)
  assert((await blockedRow.textContent()).includes('发送失败'))
  assert((await pageB.locator('.group-error').textContent()).includes('禁言'))

  const whileMuted = `receive-while-muted-${runID}`
  await sendAndWait(pageA, whileMuted)
  await pageB.locator('.message').filter({ hasText: whileMuted }).waitFor({ timeout: 10_000 })
  await peerRow.locator('button', { hasText: '解除' }).click()
  await sendAndWait(pageB, `after-unmute-${runID}`)

  const htmlText = `<b>hello group ${runID}</b>`
  await sendAndWait(pageA, htmlText)
  const htmlBubble = pageB.locator('.message .bubble').filter({ hasText: htmlText }).last()
  await htmlBubble.waitFor()
  assert.equal(await htmlBubble.textContent(), htmlText)
  assert.equal(await htmlBubble.locator('b').count(), 0)

  const pageB2 = await contextB.newPage()
  const socketsB2 = trackSockets(pageB2)
  await pageB2.goto(`${baseURL}/group`)
  const multi = `multi-device-${runID}`
  await sendAndWait(pageA, multi)
  await Promise.all([
    pageB.locator('.message').filter({ hasText: multi }).waitFor(),
    pageB2.locator('.list-item').filter({ hasText: multi }).first().waitFor(),
  ])
  assert.equal(await pageB.locator('.message .bubble').filter({ hasText: multi }).count(), 1)
  assert(socketsB2.some((socket) => socket.path === '/api/group/ws/chat'))
  await pageB2.reload()
  const sharedRead = pageB2.locator('.list-item').filter({ hasText: multi }).first()
  await sharedRead.waitFor()
  assert.equal(await sharedRead.locator('.unread').count(), 0, 'second device did not reconcile shared group read')

  for (let index = 0; index < 21; index += 1) {
    await sendAndWait(pageA, `group-page-${runID}-${index.toString().padStart(2, '0')}`)
  }
  await pageB.locator('.message').filter({ hasText: `group-page-${runID}-20` }).waitFor({ timeout: 15_000 })
  await pageB.reload()
  await waitGroupConnected(pageB)
  await pageB.locator('.message').filter({ hasText: `group-page-${runID}-20` }).waitFor({ timeout: 15_000 })
  assert.equal(await pageB.locator('.message').count(), 20, 'latest group history page must be limited to 20')
  await pageB.locator('.older-row button', { hasText: '加载更早消息' }).click()
  await pageB.waitForFunction(() => document.querySelectorAll('.message').length > 20)

  await pageA.evaluate(() => {
    globalThis.__gimGroupConnectionStates = []
    const pill = document.querySelector('.connection-pill')
    if (pill?.getAttribute('data-state')) globalThis.__gimGroupConnectionStates.push(pill.getAttribute('data-state'))
    new MutationObserver(() => {
      const state = document.querySelector('.connection-pill')?.getAttribute('data-state')
      if (state) globalThis.__gimGroupConnectionStates.push(state)
    }).observe(document.body, { subtree: true, attributes: true, attributeFilter: ['data-state'] })
  })
  console.log('RECONNECT_READY')
  await pageA.waitForFunction(() => globalThis.__gimGroupConnectionStates.includes('reconnecting'), undefined, { timeout: 30_000 })
  await Promise.all([waitGroupConnected(pageA), waitGroupConnected(pageB)])
  await pageA.waitForTimeout(750)
  assert(socketsA.some((socket) => socket.path === '/api/chat/ws/chat' && !socket.closed), 'Chat socket was disrupted by Group restart')
  const reconnected = `reconnected-${runID}`
  await sendAndWait(pageA, reconnected)
  await pageB.locator('.message').filter({ hasText: reconnected }).waitFor()

  const socketCountBeforeLogout = socketsA.length
  await pageA.locator('button', { hasText: '退出登录' }).click()
  await pageA.waitForURL((url) => url.pathname === '/login')
  await pageA.waitForTimeout(1_500)
  assert.equal(socketsA.length, socketCountBeforeLogout, 'logout created a new socket')
  assert.equal(await pageA.evaluate(() => localStorage.getItem('gim.web.token')), null)

  const socketCountBeforeRevoked = socketsB.length
  assert(await pageB.evaluate(async () => {
    const token = localStorage.getItem('gim.web.token')
    if (!token) return false
    return (await fetch('/api/auth/logout', { method: 'POST', headers: { token } })).ok
  }))
  await pageB.reload()
  await pageB.waitForURL((url) => url.pathname === '/login')
  await pageB.waitForTimeout(1_500)
  const afterRevoked = socketsB.length
  assert(afterRevoked <= socketCountBeforeRevoked + 2)
  await pageB.waitForTimeout(1_500)
  assert.equal(socketsB.length, afterRevoked, 'revoked auth entered a reconnect loop')

  console.log(JSON.stringify({
    result: 'PASS',
    groupID,
    users: { A: userA.id, B: userB.id, C: userC.id },
    pendingToSent: true,
    threeMemberRealtimeAndUnread: true,
    focusAwareRead: true,
    refreshRecovery: true,
    muteAndReceive: true,
    safeTextRendering: true,
    multiDeviceExactlyOnce: true,
    sharedReadState: true,
    cursorPagination: true,
    groupReconnectChatUnaffected: true,
    logoutStopsBothSockets: true,
    revokedAuthStopsReconnect: true,
  }))
} finally {
  await contextA.close()
  await contextB.close()
  await contextC.close()
  await browser.close()
}
