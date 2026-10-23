// Requests use the real API envelope and are intercepted before reaching a server.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright')
const assert = require('node:assert/strict')
const fs = require('node:fs')

;(async () => {
  const browser = await chromium.launch({ args: ['--no-sandbox'] })
  try {
    const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } })
    const errors = [], requests = []
    let failList = false, failDetail = false, empty = false, holdSearch, releaseSearch, holdDetail, releaseDetail
    const customers = Array.from({ length: 25 }, (_, i) => ({
      id: `c${i + 1}`, name: ['Ayu Lestari', 'Bima Pratama', 'Nadia Putri', 'Rizky Ramadhan', 'Dewi Anggraini'][i % 5] + (i >= 5 ? ` ${i + 1}` : ''),
      phone: i % 3 ? '' : `0812345678${String(i).padStart(2, '0')}`, visit_count: i % 4 ? 1 : 12,
      total_spent: i % 4 ? 125000 : 1350000, outlet_names: i % 2 ? 'Nusantara Bandung' : 'Nusantara Jakarta, Nusantara Bogor',
      first_visit_at: '2026-01-01T02:00:00Z', last_visit_at: '2026-10-23T03:30:00Z',
    }))
    const visits = Array.from({ length: 12 }, (_, i) => ({ order_id: `order-${i}`, outlet_id: 'o1', outlet_name: 'Nusantara Jakarta', created_at: '2026-10-23T03:30:00Z', status: ['completed', 'cancelled', 'cooking'][i % 3], total_amount: 112500, table_number: 'A-01', pax: 2, items: i === 1 ? '{broken' : [{ product_name: 'Nasi goreng kampung', qty: 2, price: 45000, subtotal: 90000 }, { product_name: 'Es teh melati', qty: 1, price: 22500 }] }))
    const detail = customer => ({ customer, outlets: [{ outlet_id: 'o1', outlet_name: 'Nusantara Jakarta', visit_count: customer.visit_count, total_spent: customer.total_spent, last_visit_at: customer.last_visit_at }], visits: customer.id === 'c1' ? visits : null })
    page.on('pageerror', error => errors.push(error.message))
    page.on('console', message => { if (message.text().includes('[Vue warn]')) errors.push(message.text()) })
    await page.addInitScript(() => {
      localStorage.setItem('cloud_pos_token', 'customer-test')
      localStorage.setItem('cloud_pos_admin', JSON.stringify({ role: 'viewer', name: 'Tim Pelayanan', username: 'pelayanan' }))
      localStorage.setItem('cloud_pos_permissions', JSON.stringify(['customers.view']))
      localStorage.setItem('cloud_pos_timezone', 'Asia/Jakarta')
    })
    await page.route('**/api/v1/**', async route => {
      const req = route.request(), url = new URL(req.url())
      assert.equal(req.method(), 'GET')
      assert.equal(req.headers().authorization, 'Bearer customer-test')
      assert(url.pathname.startsWith('/api/v1/admin/customers'))
      const ok = data => route.fulfill({ json: { success: true, data } })
      if (url.pathname.endsWith('/customers')) {
        const search = url.searchParams.get('search') || '', current = Number(url.searchParams.get('page'))
        requests.push({ search, page: current })
        if (failList) return route.fulfill({ status: 500, json: { success: false, error: 'Pelanggan gagal dimuat. Coba lagi.' } })
        if (search === 'lama' && holdSearch) { holdSearch(); await new Promise(resolve => { releaseSearch = resolve }); return ok({ customers: [customers[0]], pagination: { total: 1 } }) }
        const filtered = empty ? [] : customers.filter(c => `${c.name} ${c.phone}`.toLowerCase().includes(search.toLowerCase()))
        return ok({ customers: filtered.slice((current - 1) * 20, current * 20), pagination: { total: filtered.length, page: current, limit: 20 } })
      }
      const id = url.pathname.split('/').pop()
      if (failDetail) return route.fulfill({ status: 404, json: { success: false, error: 'Detail pelanggan tidak tersedia.' } })
      if (id === 'c1' && holdDetail) { holdDetail(); await new Promise(resolve => { releaseDetail = resolve }) }
      return ok(detail(customers.find(c => c.id === id)))
    })
    const waitList = () => page.getByText('Menampilkan 1–20 dari 25 pelanggan.', { exact: true }).waitFor()
    const firstCustomer = () => page.getByRole('button', { name: 'Lihat detail Ayu Lestari', exact: true })
    const dialog = () => page.getByRole('dialog', { name: 'Detail pelanggan', exact: true })
    const noOverflow = async () => {
      assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), 'page overflows viewport')
      if (await dialog().count()) assert(await dialog().evaluate(el => [...el.querySelectorAll('div, dl, details')].every(node => node.scrollWidth <= node.clientWidth + 1)), 'dialog content overflows')
    }
    await page.goto((process.env.CUSTOMERS_TEST_URL || 'http://127.0.0.1:4193') + '/tests/fixtures/customers.html')
    await waitList()
    assert(await page.getByRole('table').isVisible())
    assert.equal(await page.locator('.customer-hero h1').evaluate(el => getComputedStyle(el).color), 'rgb(255, 255, 255)')
    assert.equal(requests.length, 1)
    const summary = page.getByRole('region', { name: 'Ringkasan pelanggan' })
    assert.match(await summary.innerText(), /25/)
    assert.match(await summary.innerText(), /halaman ini/)
    await noOverflow()
    const artifacts = process.env.CUSTOMERS_SCREENSHOTS
    if (artifacts) { fs.mkdirSync(artifacts, { recursive: true }); await page.screenshot({ path: `${artifacts}/customers-desktop.png`, fullPage: true }) }
    console.log('PASS real response envelope, summary scope, desktop list and timezone')

    await page.getByRole('button', { name: 'Next ›', exact: true }).click()
    await page.getByText('Menampilkan 21–25 dari 25 pelanggan.', { exact: true }).waitFor()
    assert.equal(requests.filter(req => req.page === 2).length, 1, 'pagination must issue one request')
    const search = page.getByRole('searchbox', { name: 'Cari pelanggan' })
    await search.fill('Bima')
    await page.getByText('Menampilkan 1–5 dari 5 pelanggan yang cocok.', { exact: true }).waitFor()
    assert.equal(requests.at(-1).page, 1)
    const oldSearchStarted = new Promise(resolve => { holdSearch = resolve })
    await search.fill('lama'); await oldSearchStarted
    await search.fill('Nadia')
    await page.getByText('Menampilkan 1–5 dari 5 pelanggan yang cocok.', { exact: true }).waitFor()
    const oldSearchDone = page.waitForResponse(response => new URL(response.url()).searchParams.get('search') === 'lama')
    releaseSearch(); await oldSearchDone; await page.waitForTimeout(100)
    assert(await page.getByRole('button', { name: 'Lihat detail Nadia Putri', exact: true }).isVisible())
    assert.equal(await firstCustomer().count(), 0)
    await search.fill('tidak ditemukan')
    await page.getByRole('heading', { name: 'Pelanggan tidak ditemukan', exact: true }).waitFor()
    await page.getByRole('button', { name: 'Hapus pencarian' }).click(); await waitList()
    failList = true
    await page.getByRole('button', { name: 'Muat ulang', exact: true }).click()
    await page.getByRole('alert').getByText('Pelanggan gagal dimuat. Coba lagi.').waitFor()
    failList = false
    await page.getByRole('button', { name: 'Coba lagi', exact: true }).click(); await waitList()
    console.log('PASS pagination, search reset, late response protection and list retry')

    await firstCustomer().click()
    await dialog().getByRole('heading', { name: 'Ayu Lestari', exact: true }).waitFor()
    assert.equal(await dialog().getByRole('link', { name: 'Hubungi pelanggan' }).getAttribute('href'), 'tel:081234567800')
    assert(await dialog().getByText('2× Nasi goreng kampung', { exact: true }).first().isVisible())
    assert.equal(await dialog().locator('details').count(), 10)
    assert(await dialog().getByText('Selesai', { exact: true }).first().isVisible())
    await dialog().locator('summary').nth(1).click()
    await dialog().getByText('Rincian menu belum tersedia.', { exact: true }).waitFor()
    await dialog().getByRole('button', { name: 'Tampilkan kunjungan berikutnya' }).click()
    assert.equal(await dialog().locator('details').count(), 12)
    await noOverflow()
    if (artifacts) {
      await dialog().locator('.overflow-y-auto').evaluate(el => { el.scrollTop = 0 })
      await page.screenshot({ path: `${artifacts}/customers-detail-desktop.png` })
    }
    await dialog().getByRole('button', { name: 'Tutup detail', exact: true }).focus()
    await page.keyboard.press('Tab')
    assert(await dialog().getByRole('button', { name: 'Tutup', exact: true }).evaluate(el => el === document.activeElement))
    await page.keyboard.press('Escape'); await dialog().waitFor({ state: 'hidden' })
    assert(await firstCustomer().evaluate(el => el === document.activeElement))
    failDetail = true
    await firstCustomer().click(); await dialog().getByRole('alert').waitFor()
    failDetail = false
    await dialog().getByRole('button', { name: 'Coba lagi', exact: true }).click()
    await dialog().getByRole('heading', { name: 'Ayu Lestari', exact: true }).waitFor()
    await page.keyboard.press('Escape'); await dialog().waitFor({ state: 'hidden' })
    const oldDetailStarted = new Promise(resolve => { holdDetail = resolve })
    await firstCustomer().click(); await oldDetailStarted
    await page.keyboard.press('Escape'); await dialog().waitFor({ state: 'hidden' })
    await page.getByRole('button', { name: 'Lihat detail Bima Pratama', exact: true }).click()
    await dialog().getByRole('heading', { name: 'Bima Pratama', exact: true }).waitFor()
    const oldDetailDone = page.waitForResponse(response => response.url().endsWith('/customers/c1'))
    releaseDetail(); await oldDetailDone; await page.waitForTimeout(100); holdDetail = null
    assert(await dialog().getByRole('heading', { name: 'Bima Pratama', exact: true }).isVisible())
    assert.equal(await dialog().getByRole('link', { name: 'Hubungi pelanggan' }).count(), 0)
    await dialog().getByText('Belum ada riwayat kunjungan yang dapat ditampilkan.', { exact: true }).waitFor()
    await page.keyboard.press('Escape'); await dialog().waitFor({ state: 'hidden' })
    console.log('PASS detail, menu/status, history expansion, keyboard focus, retry and late detail response')

    customers[2].name = 'NamaPelangganSangatPanjangTanpaSpasi'.repeat(3)
    customers[2].outlet_names = 'OutletDenganNamaSangatPanjangTanpaSpasi'.repeat(3)
    customers[2].total_spent = 1234567890123
    customers[3].name = ''; customers[3].last_visit_at = null
    await page.getByRole('button', { name: 'Muat ulang', exact: true }).click(); await waitList()
    for (const width of [1440, 1280, 1024, 768, 390, 320]) {
      await page.setViewportSize({ width, height: width < 640 ? 844 : 1000 })
      await noOverflow()
      assert(await firstCustomer().isVisible())
      if (width < 1280) assert(await page.getByRole('list', { name: 'Kartu pelanggan' }).isVisible())
      await firstCustomer().click()
      await dialog().getByRole('heading', { name: 'Ayu Lestari', exact: true }).waitFor()
      await noOverflow()
      if (artifacts && width === 390) await page.screenshot({ path: `${artifacts}/customers-detail-mobile.png` })
      await page.keyboard.press('Escape'); await dialog().waitFor({ state: 'hidden' })
      if (artifacts && width === 390) {
        await page.locator('.content-area').evaluate(el => { el.scrollTop = 0 })
        await page.screenshot({ path: `${artifacts}/customers-mobile.png`, fullPage: true })
      }
    }
    await page.getByRole('button', { name: `Lihat detail ${customers[2].name}`, exact: true }).click()
    await dialog().getByRole('heading', { name: customers[2].name, exact: true }).waitFor()
    await noOverflow()
    await page.keyboard.press('Escape'); await dialog().waitFor({ state: 'hidden' })
    empty = true
    await page.getByRole('button', { name: 'Muat ulang', exact: true }).click()
    await page.getByRole('heading', { name: 'Belum ada pelanggan tercatat', exact: true }).waitFor()
    assert.deepEqual(errors, [])
    console.log('PASS 320/390/768/1024/1280/1440 layouts, long names/values, missing data and empty state; no browser errors')
  } finally { await browser.close() }
})().catch(error => { console.error(error); process.exitCode = 1 })
