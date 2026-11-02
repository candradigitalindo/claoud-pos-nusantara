// Synthetic period movements and intercepted GETs only; no production reads/writes.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const base = process.env.BALANCE_TEST_URL || 'http://127.0.0.1:4189'
const url = `${base}/tests/fixtures/balance-report.html`
const months = ['Januari', 'Februari', 'Maret', 'April', 'Mei', 'Juni', 'Juli', 'Agustus', 'September', 'Oktober', 'November', 'Desember']
const rupiah = new Intl.NumberFormat('id-ID', { style: 'currency', currency: 'IDR', minimumFractionDigits: 0, maximumFractionDigits: 0 })
const norm = text => String(text).replace(/\s+/g, ' ').trim()
const key = params => `${params.date_from}/${params.date_to}`
const cashDays = [
  { date: '2026-10-31', receipts: 500000, payments: 263400 },
  { date: '2026-11-01', receipts: 450000, payments: 68300 },
  { date: '2026-11-02', receipts: 500000, payments: 118300 },
]
function position(date) {
  const cash = 4999000000 + cashDays.filter(day => day.date <= date).reduce((sum, day) => sum + day.receipts - day.payments, 0)
  const receivables = date >= '2026-11-02' ? 100000 : date >= '2026-11-01' ? 120000 : date >= '2026-10-31' ? 150000 : 200000
  const fixed = date >= '2026-11-02' ? 150000 : date >= '2026-11-01' ? 155000 : 160000
  const total = cash + receivables + 250000 + fixed
  return { cash_and_equivalents: cash, receivables, inventory: 250000, fixed_assets: fixed, projects_in_progress: 0,
    total_assets: total, accounts_payable: 100000, customer_deposits: 0, tax_payable: 0,
    financing_liabilities: 0, total_liabilities: 100000, total_equity: total - 100000,
    total_revenue: cash + 1000000, total_cash_in: 50000, total_expense: 1050000, unpaid_amount: receivables }
}
function section(values, review_count = 0) {
  return { ...values, review_count, balance_difference: 0,
    accounts: [{ code: '1-100', name: 'Kas', amount: values.cash_and_equivalents }],
    outlets: [{ ...values, outlet_id: 'outlet-a', outlet_name: 'Outlet Uji' }] }
}
function balance(params, periodReview = 0, closingReview = 0) {
  const before = new Date(params.date_from + 'T00:00:00Z'); before.setUTCDate(before.getUTCDate() - 1)
  const opening = position(before.toISOString().slice(0, 10)), closing = position(params.date_to)
  const movement = Object.fromEntries(Object.keys(closing).map(field => [field, closing[field] - opening[field]]))
  const selected = cashDays.filter(day => day.date >= params.date_from && day.date <= params.date_to)
  return { ...section(closing, closingReview), ...params,
    opening: section(opening), period: { ...section(movement, periodReview),
      cash_receipts: selected.reduce((sum, day) => sum + day.receipts, 0),
      cash_payments: selected.reduce((sum, day) => sum + day.payments, 0) } }
}
async function pickDay(dialog, date) {
  const [year, month] = date.split('-').map(Number)
  const [shownMonth, shownYear] = (await dialog.locator('.drp-cal-title').innerText()).split(' ')
  const steps = (year - Number(shownYear)) * 12 + month - 1 - months.indexOf(shownMonth)
  assert(Math.abs(steps) < 120)
  for (let i = 0; i < Math.abs(steps); i++) {
    await dialog.getByRole('button', { name: steps < 0 ? 'Bulan sebelumnya' : 'Bulan berikutnya', exact: true }).click()
  }
  await dialog.locator(`.drp-day[data-date="${date}"]`).click()
}
async function pendingRange(page, from, to) {
  await page.getByRole('button', { name: 'Periode laporan', exact: true }).click()
  const dialog = page.getByRole('dialog', { name: 'Pilih Rentang Tanggal', exact: true })
  await dialog.waitFor()
  await pickDay(dialog, from)
  assert(await dialog.getByRole('button', { name: 'Terapkan', exact: true }).isDisabled())
  await pickDay(dialog, to)
  return dialog
}
async function selectRange(page, from, to) {
  const dialog = await pendingRange(page, from, to)
  await dialog.getByRole('button', { name: 'Terapkan', exact: true }).click()
  await dialog.waitFor({ state: 'hidden' })
}
async function loaded(page, from, to) {
  const formatted = [from, to].map(date => date.split('-').reverse().join('-'))
  await page.waitForFunction(dates => {
    const text = document.querySelector('p[role=status]')?.textContent || ''
    return dates.every(date => text.includes(date))
  }, formatted)
  await page.locator('[data-cash-summary="cash_and_equivalents"]').waitFor()
}
async function cashEquals(page, field, amount) {
  assert.equal(norm(await page.locator(`[data-cash-summary="${field}"] [data-cash-value]`).innerText()), norm(rupiah.format(amount)))
}
async function comparisonEquals(page, field, part, amount) {
  const cell = page.locator(`[data-balance-field="${field}"] [data-value="${part}"]`)
  assert.equal(norm(await cell.innerText()), norm(rupiah.format(amount)))
}

;(async () => {
  const browser = await chromium.launch({ args: ['--no-sandbox'] })
  try {
    const page = await browser.newPage({ viewport: { width: 1366, height: 1000 }, timezoneId: 'Asia/Jakarta' })
    page.setDefaultTimeout(10000)
    await page.clock.setFixedTime(new Date('2026-11-02T02:00:00+07:00'))
    const errors = [], requests = [], held = [], exports = [], heldExports = [], downloads = []
    let holdReports = true, failRange = null, legacyRange = null, holdExport = false, periodReview = 0, closingReview = 0
    page.on('pageerror', error => errors.push(error.message))
    page.on('console', message => { if (message.text().includes('[Vue warn]')) errors.push(message.text()) })
    page.on('download', download => downloads.push(download))
    const reply = pending => {
      const data = balance(pending.params, periodReview, closingReview)
      if (key(pending.params) === legacyRange) { delete data.period; delete data.opening }
      return pending.route.fulfill({ json: { success: true, data } })
    }
    const replyExport = pending => pending.route.fulfill({
      contentType: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
      body: Buffer.from(`synthetic-balance-period=${key(pending.params)}`),
    })
    await page.route('**/api/v1/**', route => {
      const request = route.request(), requestURL = new URL(request.url()), path = requestURL.pathname
      const params = Object.fromEntries(requestURL.searchParams), pending = { route, params }
      assert.equal(request.method(), 'GET')
      if (path === '/api/v1/admin/balance-report') {
        requests.push(params)
        if (holdReports) { held.push(pending); return }
        if (key(params) === failRange) return route.fulfill({ status: 500, json: { success: false, error: 'Laporan uji gagal dimuat' } })
        return reply(pending)
      }
      if (path === '/api/v1/admin/balance-report/export') {
        exports.push(params)
        if (holdExport) { heldExports.push(pending); return }
        return replyExport(pending)
      }
      errors.push(`Unexpected API ${path}`); return route.abort()
    })
    const picker = page.getByRole('button', { name: 'Periode laporan', exact: true })
    const exportButton = page.getByRole('button', { name: 'Download Excel', exact: true })
    const settled = () => page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))))
    async function waitHeld(list, count) {
      for (let attempt = 0; list.length < count && attempt < 100; attempt++) await new Promise(resolve => setTimeout(resolve, 10))
      assert.equal(list.length, count)
    }
    await page.goto(url); await picker.waitFor(); await waitHeld(held, 1)
    assert((await picker.innerText()).includes('Hari Ini'))
    assert.deepEqual(requests[0], { date_from: '2026-11-02', date_to: '2026-11-02' })
    assert(await exportButton.isDisabled())
    await reply(held.shift()); await loaded(page, '2026-11-02', '2026-11-02')
    await cashEquals(page, 'cash_receipts', 500000)
    await cashEquals(page, 'cash_payments', 118300)
    await cashEquals(page, 'cash_and_equivalents', 381700)
    await comparisonEquals(page, 'cash_and_equivalents', 'opening', 4999618300)
    await comparisonEquals(page, 'cash_and_equivalents', 'period', 381700)
    await comparisonEquals(page, 'cash_and_equivalents', 'closing', 5000000000)
    const accounts = page.getByRole('heading', { name: 'Rincian per Akun', exact: true }).locator('..')
    assert((await accounts.innerText()).includes(rupiah.format(381700)))
    assert(!(await accounts.innerText()).includes(rupiah.format(5000000000)))
    assert((await page.locator('[data-section="period-outlets"]').innerText()).includes(rupiah.format(381700)))
    assert((await page.getByRole('heading', { name: 'Perubahan Aset', exact: true }).boundingBox()).y < (await page.getByRole('heading', { name: 'Saldo Awal dan Saldo Akhir', exact: true }).boundingBox()).y)
    assert.equal(await page.locator('input[type=date]').count(), 0)
    assert((await page.textContent('body')).includes('tunai, QRIS, kartu, dan transfer'))
    console.log('PASS Hari Ini shows 500k receipts, 118300 payments and 381700 movement; 5bn remains only in explicit closing comparison')

    holdReports = false
    await selectRange(page, '2026-11-01', '2026-11-02'); await loaded(page, '2026-11-01', '2026-11-02')
    await cashEquals(page, 'cash_receipts', 950000)
    await cashEquals(page, 'cash_payments', 186600)
    await cashEquals(page, 'cash_and_equivalents', 763400)
    await comparisonEquals(page, 'cash_and_equivalents', 'closing', 5000000000)
    await selectRange(page, '2026-10-31', '2026-11-02'); await loaded(page, '2026-10-31', '2026-11-02')
    await cashEquals(page, 'cash_and_equivalents', 1000000)
    await comparisonEquals(page, 'cash_and_equivalents', 'opening', 4999000000)
    await comparisonEquals(page, 'cash_and_equivalents', 'closing', 5000000000)
    assert.deepEqual(requests.at(-1), { date_from: '2026-10-31', date_to: '2026-11-02' })
    console.log('PASS changing start with same end changes period amounts while opening + movement equals the unchanged closing position')

    holdReports = true
    await selectRange(page, '2026-11-01', '2026-11-02'); await waitHeld(held, 1)
    await selectRange(page, '2026-11-02', '2026-11-02'); await waitHeld(held, 2)
    await reply(held.shift()); await settled()
    assert.equal(await page.locator('[data-cash-summary]').count(), 0)
    assert(await exportButton.isDisabled())
    await reply(held.shift()); await loaded(page, '2026-11-02', '2026-11-02')
    await selectRange(page, '2026-10-31', '2026-11-02'); await waitHeld(held, 1)
    await selectRange(page, '2026-11-01', '2026-11-02'); await waitHeld(held, 2)
    await reply(held.pop()); await loaded(page, '2026-11-01', '2026-11-02')
    await reply(held.pop()); await settled()
    await cashEquals(page, 'cash_and_equivalents', 763400)
    console.log('PASS stale responses cannot overwrite either active range boundary or end a newer loading state')

    await selectRange(page, '2026-10-31', '2026-11-02'); await waitHeld(held, 1)
    holdReports = false; failRange = '2026-11-02/2026-11-02'
    await selectRange(page, '2026-11-02', '2026-11-02'); await page.getByRole('alert').waitFor()
    assert.equal(await page.locator('[data-cash-summary]').count(), 0)
    assert.equal(await page.locator('[data-section="balance-comparison"]').count(), 0)
    assert(await exportButton.isDisabled())
    await reply(held.pop()); await settled()
    assert.equal(await page.locator('[data-cash-summary]').count(), 0)
    failRange = null
    await page.getByRole('button', { name: 'Tampilkan', exact: true }).click(); await loaded(page, '2026-11-02', '2026-11-02')
    assert.equal(await page.getByRole('alert').count(), 0)
    console.log('PASS failed latest range clears all old amounts/export, ignores older success and supports retry')

    legacyRange = '2026-11-01/2026-11-02'
    await selectRange(page, '2026-11-01', '2026-11-02'); await page.getByRole('alert').waitFor()
    assert((await page.getByRole('alert').innerText()).includes('Rincian perubahan periode belum tersedia'))
    assert.equal(await page.locator('[data-cash-summary]').count(), 0)
    assert(await exportButton.isDisabled())
    legacyRange = null
    await page.getByRole('button', { name: 'Tampilkan', exact: true }).click(); await loaded(page, '2026-11-01', '2026-11-02')
    console.log('PASS missing period metadata never substitutes cumulative billions as period amounts')

    holdReports = true
    await selectRange(page, '2026-10-31', '2026-11-02'); await waitHeld(held, 1)
    holdReports = false
    await selectRange(page, '2026-11-02', '2026-11-02'); await loaded(page, '2026-11-02', '2026-11-02')
    await held.pop().route.fulfill({ status: 500, json: { success: false, error: 'Permintaan lama gagal' } }); await settled()
    assert.equal(await page.getByRole('alert').count(), 0)
    const requestCount = requests.length
    const pending = await pendingRange(page, '2026-10-31', '2026-11-01')
    assert.equal(requests.length, requestCount)
    await page.keyboard.press('Escape'); await pending.waitFor({ state: 'hidden' })
    assert.equal(requests.length, requestCount)
    await cashEquals(page, 'cash_and_equivalents', 381700)
    assert(await exportButton.isEnabled())
    console.log('PASS stale failures and cancelled pending ranges preserve the applied report and export')

    holdExport = true
    await selectRange(page, '2026-11-01', '2026-11-02'); await loaded(page, '2026-11-01', '2026-11-02')
    const downloadPromise = page.waitForEvent('download')
    await exportButton.click(); await waitHeld(heldExports, 1)
    assert.deepEqual(exports.at(-1), { date_from: '2026-11-01', date_to: '2026-11-02' })
    await selectRange(page, '2026-10-31', '2026-11-02'); await loaded(page, '2026-10-31', '2026-11-02')
    await replyExport(heldExports.pop())
    const download = await downloadPromise
    assert.equal(download.suggestedFilename(), 'Laporan-Neraca_2026-11-01_2026-11-02.xlsx')
    assert.equal(fs.readFileSync(await download.path(), 'utf8'), 'synthetic-balance-period=2026-11-01/2026-11-02')
    await cashEquals(page, 'cash_and_equivalents', 1000000)
    console.log('PASS in-flight Excel retains both original range boundaries in request, bytes and filename')

    await exportButton.click(); await waitHeld(heldExports, 1)
    await selectRange(page, '2026-11-02', '2026-11-02'); await loaded(page, '2026-11-02', '2026-11-02')
    await heldExports.pop().route.fulfill({ status: 500, json: { success: false, error: 'Ekspor lama gagal' } }); await settled()
    assert.equal(await page.getByRole('alert').count(), 0)
    await exportButton.click(); await waitHeld(heldExports, 1)
    await heldExports.pop().route.fulfill({ status: 500, json: { success: false, error: 'Ekspor gagal' } }); await page.getByRole('alert').waitFor()
    assert(await exportButton.isEnabled())
    await cashEquals(page, 'cash_and_equivalents', 381700)
    await exportButton.click(); await waitHeld(heldExports, 1)
    await page.getByRole('button', { name: 'Tutup laporan uji', exact: true }).click()
    await page.getByText('Laporan ditutup', { exact: true }).waitFor()
    const beforeUnmount = downloads.length
    await replyExport(heldExports.pop()); await settled()
    assert.equal(downloads.length, beforeUnmount)
    console.log('PASS export failure isolation, retry and unmount completion preserve report lifecycle safety')

    const mobile = await browser.newPage({ viewport: { width: 390, height: 844 }, timezoneId: 'UTC', isMobile: true, hasTouch: true })
    mobile.on('pageerror', error => errors.push(error.message))
    mobile.on('console', message => { if (message.text().includes('[Vue warn]')) errors.push(message.text()) })
    await mobile.clock.setFixedTime(new Date('2026-11-02T02:00:00+07:00'))
    await mobile.addInitScript(() => localStorage.setItem('cloud_pos_timezone', 'Asia/Jakarta'))
    const mobileRequests = []
    await mobile.route('**/api/v1/**', route => {
      assert.equal(route.request().method(), 'GET')
      const params = Object.fromEntries(new URL(route.request().url()).searchParams)
      mobileRequests.push(params); return reply({ route, params })
    })
    await mobile.goto(url); await loaded(mobile, '2026-11-02', '2026-11-02')
    assert.deepEqual(mobileRequests[0], { date_from: '2026-11-02', date_to: '2026-11-02' })
    const mobilePicker = mobile.getByRole('button', { name: 'Periode laporan', exact: true })
    await mobilePicker.click()
    const dialog = mobile.getByRole('dialog', { name: 'Pilih Rentang Tanggal', exact: true })
    assert.equal(await dialog.locator('.drp-day.is-today').getAttribute('data-date'), '2026-11-02')
    await dialog.getByRole('button', { name: 'Kemarin', exact: true }).click(); await loaded(mobile, '2026-11-01', '2026-11-01')
    assert.deepEqual(mobileRequests.at(-1), { date_from: '2026-11-01', date_to: '2026-11-01' })
    await cashEquals(mobile, 'cash_receipts', 450000)
    await mobilePicker.click()
    await mobile.getByRole('dialog').getByRole('button', { name: 'Bulan Ini', exact: true }).click()
    await loaded(mobile, '2026-11-01', '2026-11-02')
    assert.deepEqual(mobileRequests.at(-1), { date_from: '2026-11-01', date_to: '2026-11-02' })
    await cashEquals(mobile, 'cash_and_equivalents', 763400)
    await comparisonEquals(mobile, 'cash_and_equivalents', 'closing', 5000000000)
    assert.equal(await mobile.locator('input[type=date]').count(), 0)
    assert(await mobile.evaluate(() => document.documentElement.scrollWidth <= innerWidth))
    console.log('PASS mobile range calendar, gross cash, comparison and Jakarta presets near midnight with browser UTC')

    periodReview = 2; closingReview = 7
    await mobile.getByRole('button', { name: 'Tampilkan', exact: true }).click()
    await mobile.getByRole('alert').filter({ hasText: '2 catatan pada periode ini' }).waitFor()
    assert(await mobile.getByRole('alert').filter({ hasText: '7 catatan kumulatif pada saldo akhir' }).isVisible())
    periodReview = 0
    await mobile.getByRole('button', { name: 'Tampilkan', exact: true }).click(); await loaded(mobile, '2026-11-01', '2026-11-02')
    assert.equal(await mobile.getByRole('alert').count(), 1)
    assert((await mobile.getByRole('alert').innerText()).includes('7 catatan kumulatif'))
    closingReview = 0
    await mobile.getByRole('button', { name: 'Tampilkan', exact: true }).click(); await loaded(mobile, '2026-11-01', '2026-11-02')
    assert.equal(await mobile.getByRole('alert').count(), 0)
    assert.deepEqual(errors, [])
    console.log('PASS period review counts are distinct from cumulative closing reviews, with no Vue/runtime errors')
  } finally { await browser.close() }
})().catch(error => { console.error(error); process.exitCode = 1 })
