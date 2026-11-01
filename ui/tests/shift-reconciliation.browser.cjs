// Run against a local Vite server. Every API response is synthetic and intercepted.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright')
const assert = require('node:assert/strict')
const base = process.env.SHIFT_RECON_TEST_URL || 'http://127.0.0.1:4189'
const url = `${base}/tests/fixtures/shift-reconciliation.html`
const row = (outlet = 'o1', overrides = {}) => ({
  shift_id: `shift-${outlet}`, outlet_id: outlet, outlet_name: outlet === 'o1' ? 'Outlet Pusat' : 'Outlet Timur',
  opened_at: '2026-11-02 00:00', closed_at: '2026-11-02 01:00', cashiers: 'Kasir Uji',
  cashier_sales: 200000, cashier_count: 2, cloud_sales: 100000, cloud_count: 1,
  diff: 100000, status: 'short', adjusted: false, adjustment_id: 0, adjustment_amount: 0, ...overrides,
})
function report(shifts) {
  const summary = { total_shifts: shifts.length, balanced_count: 0, short_count: 0, short_total: 0,
    over_count: 0, over_total: 0, adjusted_count: 0, adjusted_total: 0 }
  for (const shift of shifts) {
    summary[`${shift.status}_count`]++
    if (shift.status !== 'balanced') summary[`${shift.status}_total`] += shift.diff
    if (shift.adjusted) { summary.adjusted_count++; summary.adjusted_total += shift.adjustment_amount }
  }
  return { summary, shifts }
}

;(async () => {
  const browser = await chromium.launch({ args: ['--no-sandbox'] })
  try {
    const page = await browser.newPage({ viewport: { width: 1366, height: 950 }, timezoneId: 'Asia/Jakarta' })
    page.setDefaultTimeout(10000)
    await page.clock.setFixedTime(new Date('2026-11-02T02:00:00+07:00'))
    const errors = [], queries = [], writes = [], held = []
    let responseRows = [row()], holdOutlet = null, failOutlet = null
    page.on('pageerror', error => errors.push(error.message))
    page.on('console', message => { if (message.text().includes('[Vue warn]')) errors.push(message.text()) })
    const reply = (route, shifts) => route.fulfill({ json: { success: true, data: report(shifts) } })
    await page.route('**/api/v1/**', async route => {
      const request = route.request(), requestURL = new URL(request.url()), path = requestURL.pathname
      if (path === '/api/v1/admin/my-outlets') return route.fulfill({ json: { success: true, data: [
        { id: 'o1', name: 'Outlet Pusat' }, { id: 'o2', name: 'Outlet Timur' },
      ] } })
      if (path === '/api/v1/admin/shift-reconciliation' && request.method() === 'GET') {
        const params = Object.fromEntries(requestURL.searchParams)
        queries.push(params)
        if (params.outlet_id === holdOutlet || holdOutlet === '*') { held.push({ route, params }); return }
        if (params.outlet_id === failOutlet) return route.fulfill({ status: 500, json: { success: false, error: 'Laporan uji gagal' } })
        return reply(route, params.outlet_id ? responseRows.map(shift => row(params.outlet_id, {
          ...shift, shift_id: `shift-${params.outlet_id}`, outlet_id: params.outlet_id,
          outlet_name: params.outlet_id === 'o1' ? 'Outlet Pusat' : 'Outlet Timur',
        })) : responseRows)
      }
      if (request.method() === 'POST' && path.endsWith('/shift-o1/apply')) {
        writes.push(path)
        responseRows = [row('o1', { adjusted: true, adjustment_id: 7, adjustment_amount: 100000 })]
        return route.fulfill({ json: { success: true } })
      }
      if (request.method() === 'POST' && path.endsWith('/adjustments/7/revert')) {
        writes.push(path); responseRows = [row()]
        return route.fulfill({ json: { success: true } })
      }
      errors.push(`Unexpected API ${request.method()} ${path}`)
      return route.abort()
    })
    const table = page.locator('table')
    const settled = () => page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))))
    async function refresh() {
      const response = page.waitForResponse(response => new URL(response.url()).pathname === '/api/v1/admin/shift-reconciliation')
      await page.getByRole('button', { name: 'Tampilkan', exact: true }).click()
      await response; await settled()
    }
    async function choose(label) {
      await page.getByRole('combobox', { name: 'Filter outlet', exact: true }).click()
      await page.getByRole('option', { name: label, exact: true }).click()
    }
    async function waitHeld(count) {
      await page.waitForFunction(() => document.body.textContent.includes('Memuat…'))
      for (let i = 0; held.length < count && i < 100; i++) await new Promise(resolve => setTimeout(resolve, 10))
      assert.equal(held.length, count)
    }
    await page.goto(url)
    await table.waitFor()
    assert.equal(queries[0].date_from, '2026-11-02')
    assert.equal(queries[0].date_to, '2026-11-02')
    await page.locator('.drp-trigger').click()
    await page.getByRole('button', { name: 'Hari Ini', exact: true }).click()
    await table.waitFor()
    assert.equal(queries.at(-1).date_from, '2026-11-02')
    console.log('PASS Hari Ini uses Jakarta date at 02.00 WIB and matches date picker')

    responseRows = [row('o1', { status: 'over', diff: -100000, cloud_sales: 300000 })]
    await refresh()
    assert.equal(await page.locator('.notif-ok').count(), 0)
    assert((await page.locator('.notif-warn').innerText()).includes('1 shift lebih di cloud'))
    responseRows = [row('o1', { status: 'balanced', diff: 0, cloud_sales: 200000 })]
    await refresh()
    assert((await page.locator('.notif-ok').innerText()).includes('Semua 1 shift cocok'))
    console.log('PASS excess cloud sales show warning, while balanced shifts show success')

    responseRows = [row()]
    await refresh()
    await table.getByRole('button', { name: 'Ikuti kasir', exact: true }).click()
    await page.getByRole('dialog').getByRole('button', { name: 'Ya, sesuaikan', exact: true }).click()
    await table.getByRole('button', { name: 'Batalkan', exact: true }).waitFor()
    assert.equal(writes[0], '/api/v1/admin/shift-reconciliation/shift-o1/apply')
    assert((await table.innerText()).includes('Cloud asli'))
    assert((await table.innerText()).includes('Manual Rp'))
    responseRows = [row('o1', { adjusted: true, adjustment_id: 7, adjustment_amount: 40000 })]
    await refresh()
    assert.equal(await table.locator('.rbadge').innerText(), 'Disesuaikan · Kurang')
    responseRows = [row('o1', { status: 'over', diff: -100000, cloud_sales: 300000,
      adjusted: true, adjustment_id: 7, adjustment_amount: 0 })]
    await refresh()
    assert.equal(await table.locator('.rbadge').innerText(), 'Disesuaikan · Lebih')
    await table.getByRole('button', { name: 'Batalkan', exact: true }).click()
    await page.getByRole('dialog').getByRole('button', { name: 'Ya, batalkan', exact: true }).click()
    await table.getByRole('button', { name: 'Ikuti kasir', exact: true }).waitFor()
    assert.equal(writes[1], '/api/v1/admin/shift-reconciliation/adjustments/7/revert')
    console.log('PASS synthetic apply/revert, manual amount labels, and remaining short/over status')

    holdOutlet = '*'
    await choose('Outlet Pusat'); await waitHeld(1)
    await choose('Outlet Timur'); await waitHeld(2)
    await reply(held.shift().route, [row('o1')]); await settled()
    assert.equal(await table.count(), 0, 'Old response must not clear the newest loading state')
    assert((await page.textContent('body')).includes('Memuat…'))
    await reply(held.shift().route, [row('o2')])
    await table.waitFor()
    assert((await table.innerText()).includes('Outlet Timur'))

    await choose('Outlet Pusat'); await waitHeld(1)
    await choose('Outlet Timur'); await waitHeld(2)
    await reply(held.pop().route, [row('o2')]); await table.waitFor()
    await reply(held.pop().route, [row('o1')]); await settled()
    assert((await table.innerText()).includes('Outlet Timur'))
    assert(!(await table.innerText()).includes('Outlet Pusat'))
    console.log('PASS out-of-order responses cannot overwrite newest filter or finish its loading state')

    holdOutlet = 'o1'; failOutlet = 'o2'
    await choose('Outlet Pusat'); await waitHeld(1)
    await choose('Outlet Timur')
    await page.getByRole('alert').waitFor()
    assert((await page.getByRole('alert').innerText()).includes('Laporan uji gagal'))
    assert.equal(await table.count(), 0)
    assert.equal(await page.getByRole('button', { name: 'Ikuti kasir', exact: true }).count(), 0)
    await reply(held.pop().route, [row('o1')]); await settled()
    assert.equal(await table.count(), 0, 'Old data cannot restore actionable rows after latest failure')
    assert.equal(await page.locator('.notif-ok').count(), 0)
    holdOutlet = null; failOutlet = null
    await refresh(); await table.waitFor()
    assert.equal(await page.getByRole('alert').count(), 0)
    console.log('PASS failed reload removes stale actions, ignores older success, and supports retry')

    holdOutlet = 'o1'
    await choose('Outlet Pusat'); await waitHeld(1)
    await choose('Outlet Timur'); await table.waitFor()
    await held.pop().route.fulfill({ status: 500, json: { success: false, error: 'Galat permintaan lama' } })
    await settled()
    assert.equal(await page.getByRole('alert').count(), 0)
    assert((await table.innerText()).includes('Outlet Timur'))
    console.log('PASS an older failure cannot add an error to the newest successful report')

    await page.setViewportSize({ width: 390, height: 844 })
    await page.goto(`${url}?readonly`)
    await page.locator('ul.sm\\:hidden > li').waitFor()
    assert.equal(await page.getByRole('button', { name: /Ikuti kasir|Ikuti versi kasir|Batalkan penyesuaian/ }).count(), 0)
    assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth))
    console.log('PASS mobile layout and view-only users have no adjustment actions')

    // Configured reporting timezone must also win when the browser runs in UTC.
    const utcPage = await browser.newPage({ timezoneId: 'UTC' })
    await utcPage.clock.setFixedTime(new Date('2026-11-02T02:00:00+07:00'))
    let utcQuery
    await utcPage.route('**/api/v1/**', route => {
      const requestURL = new URL(route.request().url())
      if (requestURL.pathname.endsWith('/my-outlets')) return route.fulfill({ json: { success: true, data: [] } })
      if (requestURL.pathname.endsWith('/shift-reconciliation')) {
        utcQuery = Object.fromEntries(requestURL.searchParams)
        return reply(route, [])
      }
      return route.abort()
    })
    await utcPage.addInitScript(() => localStorage.setItem('cloud_pos_timezone', 'Asia/Jakarta'))
    const utcLoaded = utcPage.waitForResponse(response => new URL(response.url()).pathname.endsWith('/shift-reconciliation'))
    await utcPage.goto(url)
    await utcLoaded
    await utcPage.getByText('Belum ada shift tertutup pada periode ini.', { exact: true }).waitFor()
    assert.equal(utcQuery.date_from, '2026-11-02')
    assert.equal(utcQuery.date_to, '2026-11-02')
    assert.deepEqual(errors, [])
    console.log('PASS configured timezone is respected independently of browser timezone; no Vue errors')
  } finally { await browser.close() }
})().catch(error => { console.error(error); process.exitCode = 1 })
