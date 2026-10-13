import assert from 'node:assert/strict'
import { createHash } from 'node:crypto'
import { access, mkdtemp, open, readFile, rm, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'

import { chromium } from 'playwright-core'

const baseURL = process.env.GIM_E2E_WEB_URL || 'http://127.0.0.1:5173'
const browserExecutable = process.env.GIM_E2E_BROWSER || 'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe'
await access(browserExecutable)

const runID = Date.now()
const password = `Smoke-${runID}-Aa!`
const fixtureRoot = await mkdtemp(join(tmpdir(), 'gim-file-e2e-'))
const largePath = join(fixtureRoot, 'large-12m.bin')
const oversizedPath = join(fixtureRoot, 'oversized-50m.bin')
const largePayload = Buffer.alloc(12 * 1024 * 1024)
for (let index = 0; index < largePayload.length; index += 4096) largePayload.writeUInt32LE(index, index)
await writeFile(largePath, largePayload)
const oversized = await open(oversizedPath, 'w')
await oversized.truncate(50 * 1024 * 1024 + 1)
await oversized.close()

function sha256(payload) {
  return createHash('sha256').update(payload).digest('hex')
}

async function register(label) {
  const account = `gim_cp6_file_${label}_${runID}`
  const response = await fetch(`${baseURL}/api/auth/register`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ account, nickname: `File ${label}`, pwd: password, rePwd: password }),
  })
  const body = await response.json()
  assert.equal(body.code, 0, `register ${label} failed: ${body.msg}`)
  return { account, id: body.data.userID }
}

async function login(page, user) {
  await page.goto(`${baseURL}/login`)
  await page.locator('input[autocomplete="username"]').fill(user.account)
  await page.locator('input[autocomplete="current-password"]').fill(password)
  await page.locator('button[type="submit"]').click()
  await page.waitForURL((url) => url.pathname === '/', { timeout: 15_000 })
  await page.goto(`${baseURL}/files`)
  await page.locator('.file-list-panel').waitFor({ timeout: 15_000 })
}

async function uploadPayloads(page, payloads) {
  await page.locator('.file-picker input[type="file"]').setInputFiles(payloads)
  for (const payload of payloads) {
    const name = typeof payload === 'string' ? payload.split(/[\\/]/).at(-1) : payload.name
    const task = page.locator('.upload-task').filter({ hasText: name }).first()
    await task.waitFor({ timeout: 10_000 })
    await page.waitForFunction(
      (fileName) => [...document.querySelectorAll('.upload-task')].some((item) => item.textContent?.includes(fileName) && ['success', 'failed'].includes(item.getAttribute('data-status') || '')),
      name,
      { timeout: 30_000 },
    ).catch(async (error) => {
      throw new Error(`upload ${name} timed out: status=${await task.getAttribute('data-status')} text=${await task.textContent()}`, { cause: error })
    })
    const status = await task.getAttribute('data-status')
    assert.equal(status, 'success', `upload ${name} failed: ${await task.textContent()}`)
  }
}

async function downloadRow(page, fileName) {
  const row = page.locator('.file-row').filter({ hasText: fileName }).first()
  await row.waitFor({ timeout: 15_000 })
  const downloadPromise = page.waitForEvent('download', { timeout: 120_000 })
  await row.locator('button', { hasText: '下载' }).click()
  const download = await downloadPromise
  assert(download.suggestedFilename().length > 0)
  assert(!/[\\/\r\n]/.test(download.suggestedFilename()), 'browser download filename was unsafe')
  const path = await download.path()
  assert(path, `download path missing for ${fileName}`)
  return readFile(path)
}

async function deleteRow(page, fileName) {
  const row = page.locator('.file-row').filter({ hasText: fileName }).first()
  await row.locator('button', { hasText: '删除' }).click()
  const dialog = page.locator('.el-message-box')
  await dialog.waitFor()
  assert((await dialog.textContent()).includes('共享内容不会被误删'))
  await dialog.locator('button', { hasText: '删除' }).click()
  await row.waitFor({ state: 'detached', timeout: 15_000 })
}

const browser = await chromium.launch({ executablePath: browserExecutable, headless: true })
const contextA = await browser.newContext({ acceptDownloads: true })
const contextB = await browser.newContext({ acceptDownloads: true })
const pageA = await contextA.newPage()
const pageB = await contextB.newPage()
for (const page of [pageA, pageB]) {
  page.on('requestfailed', (request) => {
    if (new URL(request.url()).pathname.startsWith('/api/file/')) console.log(`FILE_E2E request-failed ${new URL(request.url()).pathname} ${request.failure()?.errorText || ''}`)
  })
  page.on('response', (response) => {
    if (new URL(response.url()).pathname.startsWith('/api/file/')) console.log(`FILE_E2E response ${response.status()} ${new URL(response.url()).pathname}`)
  })
}

try {
  console.log('FILE_E2E register-and-login')
  const [userA, userB] = await Promise.all([register('A'), register('B')])
  await Promise.all([login(pageA, userA), login(pageB, userB)])

  await pageA.evaluate(() => {
    globalThis.__gimFileTaskStates = []
    const capture = () => {
      for (const task of document.querySelectorAll('.upload-task')) {
        const name = task.querySelector('strong')?.textContent || ''
        const status = task.getAttribute('data-status') || ''
        const text = task.textContent || ''
        globalThis.__gimFileTaskStates.push({ name, status, text })
      }
    }
    new MutationObserver(capture).observe(document.body, { childList: true, subtree: true, attributes: true, characterData: true })
  })

  const maliciousName = '<img src=x onerror=alert(1)>.txt'
  const maliciousPayload = Buffer.from(`literal filename ${runID}`)
  console.log('FILE_E2E small-and-malicious-upload')
  await uploadPayloads(pageA, [{ name: maliciousName, mimeType: 'text/plain', buffer: maliciousPayload }])
  const maliciousRow = pageA.locator('.file-row').filter({ hasText: maliciousName }).first()
  await maliciousRow.waitFor()
  assert.equal(await maliciousRow.locator('.file-name').textContent(), maliciousName)
  assert.equal(await maliciousRow.locator('.file-name img').count(), 0, 'filename was interpreted as HTML')
  const aFileID = Number(await maliciousRow.getAttribute('data-file-id'))
  assert(Number.isSafeInteger(aFileID) && aFileID > 0)

  console.log('FILE_E2E large-upload-and-limit')
  await uploadPayloads(pageA, [largePath])
  const observed = await pageA.evaluate(() => globalThis.__gimFileTaskStates)
  assert(observed.some((item) => item.name === 'large-12m.bin' && item.status === 'uploading'), 'large upload never exposed uploading progress state')
  assert(observed.some((item) => item.name === 'large-12m.bin' && item.status === 'success'), 'large upload never completed in UI')

  await pageA.locator('.file-picker input[type="file"]').setInputFiles(oversizedPath)
  const oversizedTask = pageA.locator('.upload-task').filter({ hasText: 'oversized-50m.bin' }).first()
  await oversizedTask.waitFor()
  await pageA.waitForFunction(() => [...document.querySelectorAll('.upload-task')].some((item) => item.textContent?.includes('oversized-50m.bin') && item.getAttribute('data-status') === 'failed'))
  assert((await oversizedTask.textContent()).includes('未开始上传'))

  console.log('FILE_E2E owner-download-refresh')
  assert.equal(sha256(await downloadRow(pageA, maliciousName)), sha256(maliciousPayload))
  assert.equal(sha256(await downloadRow(pageA, 'large-12m.bin')), sha256(largePayload))

  await pageA.reload()
  await pageA.locator('.file-row').filter({ hasText: maliciousName }).waitFor({ timeout: 15_000 })
  await pageA.locator('.file-row').filter({ hasText: 'large-12m.bin' }).waitFor()
  assert.equal(await pageA.locator('button', { hasText: '预览' }).count(), 0, 'V1 unexpectedly exposed preview')

  console.log('FILE_E2E cross-user-ownership')
  const ownership = await pageB.evaluate(async (id) => {
    const token = localStorage.getItem('gim.web.token') || ''
    const call = async (path, method = 'GET') => {
      const response = await fetch(path, { method, headers: { token } })
      return { status: response.status, body: await response.json() }
    }
    return {
      detail: await call(`/api/file/user-files/${id}`),
      download: await call(`/api/file/${id}`),
      remove: await call(`/api/file/user-files/${id}`, 'DELETE'),
    }
  }, aFileID)
  for (const result of Object.values(ownership)) assert.equal(result.body.code, 2503)
  assert.equal(await pageA.locator('.file-row').filter({ hasText: maliciousName }).count(), 1, 'unauthorized delete changed owner state')

  console.log('FILE_E2E shared-object-delete')
  const sharedPayload = Buffer.from(`shared physical object ${runID}`)
  await uploadPayloads(pageA, [{ name: 'shared-a.bin', mimeType: 'application/octet-stream', buffer: sharedPayload }])
  await uploadPayloads(pageB, [{ name: 'shared-b.bin', mimeType: 'application/octet-stream', buffer: sharedPayload }])
  await deleteRow(pageA, 'shared-a.bin')
  assert.equal(sha256(await downloadRow(pageB, 'shared-b.bin')), sha256(sharedPayload), 'shared object was removed with A reference')

  console.log('FILE_E2E cursor-pagination')
  const pagePayloads = Array.from({ length: 21 }, (_, index) => ({
    name: `page-${String(index).padStart(2, '0')}-${runID}.txt`,
    mimeType: 'text/plain',
    buffer: Buffer.from(`page ${index} ${runID}`),
  }))
  await uploadPayloads(pageA, pagePayloads)
  await pageA.reload()
  await pageA.waitForFunction(() => document.querySelectorAll('.file-row').length === 20, undefined, { timeout: 20_000 })
  await pageA.locator('.more-row button', { hasText: '加载更多' }).click()
  await pageA.waitForFunction(() => document.querySelectorAll('.file-row').length > 20, undefined, { timeout: 20_000 })

  console.log('FILE_E2E final-delete-and-download')
  await deleteRow(pageA, maliciousName)
  assert.equal(await pageA.locator('.file-row').filter({ hasText: maliciousName }).count(), 0)
  assert.equal(sha256(await downloadRow(pageA, 'large-12m.bin')), sha256(largePayload))

  console.log(JSON.stringify({
    result: 'PASS',
    users: { A: userA.id, B: userB.id },
    uploadListRefresh: true,
    independentProgress: true,
    largeFileHash: true,
    oversizedBlockedBeforeUpload: true,
    authenticatedBlobDownload: true,
    ownerOnlyDetailDownloadDelete: true,
    sharedPhysicalObjectDeleteSafe: true,
    cursorPagination: true,
    deleteConfirmation: true,
    safeFilenameRendering: true,
    previewDeferred: true,
  }))
} finally {
  await contextA.close()
  await contextB.close()
  await browser.close()
  await rm(fixtureRoot, { recursive: true, force: true })
}
