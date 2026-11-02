// Synthetic balances and intercepted GETs only; no production data or writes.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const base = process.env.BALANCE_TEST_URL || 'http://127.0.0.1:4189'
const url = `${base}/tests/fixtures/balance-report.html`
const months = ['Januari', 'Februari', 'Maret', 'April', 'Mei', 'Juni', 'Juli', 'Agustus', 'September', 'Oktober', 'November', 'Desember']
async function chooseCalendarDay(page, date) {
  await page.locator('.drp-trigger').click()
  const dialog = page.getByRole('dialog', { name: 'Pilih Tanggal Posisi', exact: true })
  await dialog.waitFor()
  const [year, month] = date.split('-').map(Number)
  const [shownMonth, shownYear] = (await dialog.locator('.drp-cal-title').innerText()).split(' ')
  const steps = (year - Number(shownYear)) * 12 + month - 1 - months.indexOf(shownMonth)
  assert(Math.abs(steps) < 120, 'Calendar navigation must stay within the fixture years')
  for (let i = 0; i < Math.abs(steps); i++) {
    await dialog.getByRole('button', { name: steps < 0 ? 'Bulan sebelumnya' : 'Bulan berikutnya', exact: true }).click()
  }
  await dialog.locator(`.drp-day[data-date="${date}"]`).click()
  return dialog
}
function balance(date) {
  const cash = date === '2026-11-02' ? 5000000000 : date === '2026-11-01' ? 4999618300 : 4000000000
  const row = {
    outlet_id: 'outlet-a', outlet_name: 'Outlet Uji', cash_and_equivalents: cash,
    receivables: 100000, inventory: 250000, fixed_assets: 150000, projects_in_progress: 0,
    total_assets: cash + 500000, accounts_payable: 100000, customer_deposits: 0, tax_payable: 0,
    financing_liabilities: 0, total_liabilities: 100000, total_equity: cash + 400000,
    total_revenue: cash + 1000000, total_cash_in: 50000, total_expense: 1050000, unpaid_amount: 100000,
  }
  return { ...row, date_from: date, date_to: date, outlets: [row],
    accounts: [{ code: '1-100', name: 'Kas', amount: cash }] }
}

;(async () => {
  const browser = await chromium.launch({ args: ['--no-sandbox'] })
  try {
    const page = await browser.newPage({ viewport: { width: 1366, height: 1000 }, timezoneId: 'Asia/Jakarta' })
    page.setDefaultTimeout(10000)
    await page.clock.setFixedTime(new Date('2026-11-02T02:00:00+07:00'))
    const errors = [], requests = [], held = [], exports = [], heldExports = [], downloads = []
    let holdReports = true, failDate = null, holdExport = false, reviewCount
    page.on('pageerror', error => errors.push(error.message))
    page.on('console', message => { if (message.text().includes('[Vue warn]')) errors.push(message.text()) })
    page.on('download', download => downloads.push(download))
    const reply = (route, date) => route.fulfill({ json: { success: true, data: {
      ...balance(date), ...(reviewCount === undefined ? {} : { review_count: reviewCount, balance_difference: 0 }),
    } } })
    const replyExport = ({ route, params }) => route.fulfill({
      contentType: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
      body: Buffer.from(`synthetic-balance-position=${params.date_to}`),
    })
    await page.route('**/api/v1/**', route => {
      const request = route.request(), requestURL = new URL(request.url()), path = requestURL.pathname
      const params = Object.fromEntries(requestURL.searchParams)
      assert.equal(request.method(), 'GET', 'A view-only report must never mutate records')
      assert.equal(params.date_from, params.date_to, 'Balance API uses a single position date')
      if (path === '/api/v1/admin/balance-report') {
        requests.push(params)
        if (holdReports) { held.push({ route, params }); return }
        if (params.date_to === failDate) return route.fulfill({ status: 500, json: { success: false, error: 'Neraca uji gagal dimuat' } })
        return reply(route, params.date_to)
      }
      if (path === '/api/v1/admin/balance-report/export') {
        exports.push(params)
        const pending = { route, params }
        if (holdExport) { heldExports.push(pending); return }
        return replyExport(pending)
      }
      errors.push(`Unexpected API ${path}`)
      return route.abort()
    })
    const input = page.getByRole('button', { name: 'Tanggal posisi neraca', exact: true })
    const exportButton = page.getByRole('button', { name: 'Download Excel', exact: true })
    const assets = page.getByRole('heading', { name: 'Aset', exact: true })
    const status = page.locator('p[role=status]')
    const settled = () => page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))))
    async function waitHeld(list, count) {
      for (let attempt = 0; list.length < count && attempt < 100; attempt++) await new Promise(resolve => setTimeout(resolve, 10))
      assert.equal(list.length, count)
    }
    async function selectDate(date) {
      const dialog = await chooseCalendarDay(page, date)
      await dialog.getByRole('button', { name: 'Terapkan', exact: true }).click()
      await dialog.waitFor({ state: 'hidden' })
    }
    async function loaded(date) {
      const formatted = date.split('-').reverse().join('-')
      await page.waitForFunction(formatted => document.querySelector('p[role=status]')?.textContent.includes(formatted), formatted)
      await assets.waitFor()
    }
    await page.goto(url)
    await input.waitFor(); await waitHeld(held, 1)
    assert((await input.innerText()).includes('Hari Ini'))
    assert((await input.innerText()).includes('2 Nov 2026'))
    assert.deepEqual(requests[0], { date_from: '2026-11-02', date_to: '2026-11-02' })
    assert(await exportButton.isDisabled())
    assert.equal(await assets.count(), 0)
    await reply(held.shift().route, '2026-11-02'); await loaded('2026-11-02')
    assert((await status.innerText()).includes('Saldo kumulatif'))
    assert((await page.textContent('body')).includes('Rp 5.000.000.000'))
    assert.equal(await page.locator('input[type=date]').count(), 0)
    assert.equal(await page.locator('.drp-trigger').count(), 1)
    assert((await assets.boundingBox()).y < (await page.getByRole('heading', { name: 'Rincian per Akun', exact: true }).boundingBox()).y,
      'Main monetary cards must precede account details')
    for (const label of ['Pendapatan Transaksi Kumulatif', 'Pendapatan Lainnya Kumulatif', 'Beban Kumulatif']) {
      assert(await page.getByText(label, { exact: true }).isVisible())
    }
    assert(await exportButton.isEnabled())
    assert(await page.getByText('Ekuitas dan akumulasi hasil usaha', { exact: true }).isVisible())
    console.log('PASS shared custom position picker, Indonesian date, prominent monetary cards, and cumulative nominal preserved')

    let pending = await chooseCalendarDay(page, '2026-10-31')
    assert.equal(requests.length, 1, 'Calendar clicks remain pending until applied')
    assert((await status.innerText()).includes('02-11-2026'))
    await page.keyboard.press('Escape')
    await pending.waitFor({ state: 'hidden' })
    assert.equal(requests.length, 1, 'Closing the picker must not change the applied date')
    await input.click()
    const presets = page.getByRole('dialog', { name: 'Pilih Tanggal Posisi', exact: true })
    assert.equal(await presets.getByRole('button', { name: 'Bulan Ini', exact: true }).count(), 0)
    for (const name of ['Hari Ini', 'Kemarin', 'Akhir Minggu Lalu', 'Akhir Bulan Lalu', 'Akhir Tahun Lalu']) {
      assert(await presets.getByRole('button', { name, exact: true }).isVisible())
    }
    await presets.getByRole('button', { name: 'Akhir Bulan Lalu', exact: true }).click()
    await waitHeld(held, 1)
    assert.deepEqual(held[0].params, { date_from: '2026-10-31', date_to: '2026-10-31' })
    await reply(held.shift().route, '2026-10-31'); await loaded('2026-10-31')
    await selectDate('2026-11-02'); await waitHeld(held, 1)
    await reply(held.shift().route, '2026-11-02'); await loaded('2026-11-02')
    console.log('PASS pending selection, Escape cancellation, month-end preset, and explicit one-date requests')

    await selectDate('2026-10-31'); await waitHeld(held, 1)
    await selectDate('2026-11-01'); await waitHeld(held, 2)
    await reply(held.shift().route, '2026-10-31'); await settled()
    assert.equal(await assets.count(), 0, 'Older completion must not end the newest loading state')
    assert(await exportButton.isDisabled())
    await reply(held.shift().route, '2026-11-01'); await loaded('2026-11-01')
    await selectDate('2026-10-31'); await waitHeld(held, 1)
    await selectDate('2026-11-02'); await waitHeld(held, 2)
    await reply(held.pop().route, '2026-11-02'); await loaded('2026-11-02')
    await reply(held.pop().route, '2026-10-31'); await settled()
    assert((await status.innerText()).includes('02-11-2026'))
    assert((await page.textContent('body')).includes('Rp 5.000.000.000'))
    console.log('PASS stale responses cannot replace the loaded position date or end a newer loading state')

    await selectDate('2026-10-31'); await waitHeld(held, 1)
    holdReports = false; failDate = '2026-11-01'
    await selectDate('2026-11-01'); await page.getByRole('alert').waitFor()
    assert.equal(await assets.count(), 0)
    assert.equal(await status.count(), 0)
    assert(await exportButton.isDisabled())
    await reply(held.pop().route, '2026-10-31'); await settled()
    assert.equal(await assets.count(), 0, 'An old success cannot restore stale report data after failure')
    failDate = null
    await page.getByRole('button', { name: 'Tampilkan', exact: true }).click(); await loaded('2026-11-01')
    assert.equal(await page.getByRole('alert').count(), 0)
    console.log('PASS failed reload clears old amounts/export, ignores older success, and supports retry')

    holdReports = true
    await selectDate('2026-10-31'); await waitHeld(held, 1)
    holdReports = false
    await selectDate('2026-11-02'); await loaded('2026-11-02')
    await held.pop().route.fulfill({ status: 500, json: { success: false, error: 'Permintaan lama gagal' } }); await settled()
    assert.equal(await page.getByRole('alert').count(), 0)
    const requestCount = requests.length
    pending = await chooseCalendarDay(page, '2026-10-31')
    await pending.getByRole('button', { name: 'Tutup pemilih tanggal', exact: true }).click()
    await pending.waitFor({ state: 'hidden' })
    assert.equal(requests.length, requestCount)
    assert(await exportButton.isEnabled())
    assert((await status.innerText()).includes('02-11-2026'))
    console.log('PASS older errors are ignored and cancelled calendar changes preserve the loaded report/export')

    holdExport = true
    const firstDownload = page.waitForEvent('download')
    await exportButton.click(); await waitHeld(heldExports, 1)
    assert.deepEqual(exports.at(-1), { date_from: '2026-11-02', date_to: '2026-11-02' })
    await selectDate('2026-10-31'); await loaded('2026-10-31')
    await replyExport(heldExports.pop())
    const download = await firstDownload
    assert.equal(download.suggestedFilename(), 'Laporan-Neraca_per_2026-11-02.xlsx')
    assert.equal(fs.readFileSync(await download.path(), 'utf8'), 'synthetic-balance-position=2026-11-02')
    assert((await status.innerText()).includes('31-10-2026'))
    console.log('PASS an in-progress export retains its original loaded date in both file bytes and filename')

    await exportButton.waitFor({ state: 'visible' })
    await exportButton.click(); await waitHeld(heldExports, 1)
    await selectDate('2026-11-01'); await loaded('2026-11-01')
    await heldExports.pop().route.fulfill({ status: 500, json: { success: false, error: 'Ekspor lama gagal' } }); await settled()
    assert.equal(await page.getByRole('alert').count(), 0)
    assert(await exportButton.isEnabled())
    await exportButton.click(); await waitHeld(heldExports, 1)
    await heldExports.pop().route.fulfill({ status: 500, json: { success: false, error: 'Ekspor gagal' } })
    await page.getByRole('alert').waitFor()
    assert(await assets.isVisible())
    assert(await exportButton.isEnabled(), 'A failed download can be retried without discarding a valid report')
    console.log('PASS export failures are isolated to their loaded date and remain retryable')

    await exportButton.click(); await waitHeld(heldExports, 1)
    await page.getByRole('button', { name: 'Tutup laporan uji', exact: true }).click()
    await page.getByText('Laporan ditutup', { exact: true }).waitFor()
    const downloadsBeforeUnmount = downloads.length
    await replyExport(heldExports.pop()); await settled()
    assert.equal(downloads.length, downloadsBeforeUnmount, 'Unmounting must stop late downloads')
    assert.deepEqual(errors, [])
    console.log('PASS unmounted reports ignore export completion without Vue errors')

    const mobile = await browser.newPage({ viewport: { width: 390, height: 844 }, timezoneId: 'UTC', isMobile: true, hasTouch: true })
    mobile.on('pageerror', error => errors.push(error.message))
    mobile.on('console', message => { if (message.text().includes('[Vue warn]')) errors.push(message.text()) })
    await mobile.clock.setFixedTime(new Date('2026-11-02T02:00:00+07:00'))
    await mobile.addInitScript(() => localStorage.setItem('cloud_pos_timezone', 'Asia/Jakarta'))
    const mobileRequests = []
    await mobile.route('**/api/v1/**', route => {
      assert.equal(route.request().method(), 'GET')
      const params = Object.fromEntries(new URL(route.request().url()).searchParams)
      mobileRequests.push(params)
      return reply(route, params.date_to)
    })
    await mobile.goto(url)
    await mobile.locator('p[role=status]').waitFor()
    const mobilePicker = mobile.getByRole('button', { name: 'Tanggal posisi neraca', exact: true })
    assert((await mobilePicker.innerText()).includes('Hari Ini'))
    assert.equal(await mobile.locator('input[type=date]').count(), 0)
    assert.deepEqual(mobileRequests[0], { date_from: '2026-11-02', date_to: '2026-11-02' })
    await mobilePicker.click()
    const mobileDialog = mobile.getByRole('dialog', { name: 'Pilih Tanggal Posisi', exact: true })
    assert((await mobileDialog.locator('.drp-cal-title').innerText()).includes('November 2026'))
    assert.equal(await mobileDialog.locator('.drp-day.is-today').getAttribute('data-date'), '2026-11-02')
    const yesterday = mobile.waitForResponse(response => new URL(response.url()).searchParams.get('date_to') === '2026-11-01')
    await mobileDialog.getByRole('button', { name: 'Kemarin', exact: true }).click(); await yesterday
    await mobile.waitForFunction(() => document.querySelector('p[role=status]')?.textContent.includes('01-11-2026'))
    assert.deepEqual(mobileRequests.at(-1), { date_from: '2026-11-01', date_to: '2026-11-01' })
    await mobilePicker.click()
    const todayResponse = mobile.waitForResponse(response => new URL(response.url()).searchParams.get('date_to') === '2026-11-02')
    await mobile.getByRole('dialog').getByRole('button', { name: 'Hari Ini', exact: true }).click(); await todayResponse
    await mobile.waitForFunction(() => document.querySelector('p[role=status]')?.textContent.includes('02-11-2026'))
    assert(await mobile.evaluate(() => document.documentElement.scrollWidth <= innerWidth))
    assert.deepEqual(await mobile.evaluate(() => JSON.parse(localStorage.getItem('cloud_pos_permissions'))), ['reports.balance.view'])
    assert.deepEqual(errors, [])
    console.log('PASS configured reporting timezone independent of browser timezone, mobile layout, and view-only GET access')

    reviewCount = 3
    const reviewedResponse = mobile.waitForResponse(response => new URL(response.url()).pathname === '/api/v1/admin/balance-report')
    await mobile.getByRole('button', { name: 'Tampilkan', exact: true }).click(); await reviewedResponse
    const warning = mobile.getByRole('alert')
    await warning.waitFor()
    assert((await warning.innerText()).includes('3 catatan sumber perlu ditinjau'))
    assert((await warning.innerText()).includes('harga pokok penjualan (HPP)'))
    assert((await warning.innerText()).includes('Status seimbang belum menjamin semua catatan sudah lengkap'))
    assert((await mobile.textContent('body')).includes('Neraca Seimbang'), 'Completeness warning must remain visible even with balanced arithmetic')
    assert(await mobile.getByText('Saldo buku penerimaan dan pengeluaran kas sampai tanggal posisi', { exact: true }).isVisible())
    assert(await mobile.getByText('Sisa tagihan per tanggal posisi, termasuk pembayaran sebagian', { exact: true }).isVisible())
    reviewCount = 0
    const completeResponse = mobile.waitForResponse(response => new URL(response.url()).pathname === '/api/v1/admin/balance-report')
    await mobile.getByRole('button', { name: 'Tampilkan', exact: true }).click(); await completeResponse
    await mobile.locator('p[role=status]').waitFor()
    assert.equal(await mobile.getByRole('alert').count(), 0)
    assert.deepEqual(errors, [])
    console.log('PASS source review warning with balanced totals; zero or missing review_count stays compatible')
  } finally { await browser.close() }
})().catch(error => { console.error(error); process.exitCode = 1 })
