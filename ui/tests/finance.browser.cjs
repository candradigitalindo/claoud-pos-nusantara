// All API calls are intercepted: no production or development records are written.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright')
const assert = require('node:assert/strict')
const base = process.env.FINANCE_TEST_URL || 'http://127.0.0.1:4189'
;(async () => {
  const browser = await chromium.launch({ args: ['--no-sandbox'] })
  try {
    const page = await browser.newPage({ viewport: { width: 1360, height: 1000 } })
    const errors = [], requests = []
    page.on('pageerror', e => { errors.push(e.message); console.error(e.message) })
    page.on('console', m => { if (m.text().includes('[Vue warn]')) errors.push(m.text()) })
    await page.addInitScript(() => {
      localStorage.setItem('cloud_pos_permissions', JSON.stringify(['view', 'create', 'post', 'void'].map(p => 'finance.entries.' + p)))
    })
    const accounts = [
      { code: '5-310', name: 'Beban Gaji & Upah', direction: 'out' },
      { code: '5-320', name: 'Beban Listrik, Air & Internet', direction: 'out' },
      { code: '4-200', name: 'Pendapatan Lainnya', direction: 'in' },
    ]
    let entries = [], fail = false
    const reply = (route, data) => route.fulfill({ json: { success: true, data } })
    await page.route('**/api/v1/**', async route => {
      const req = route.request(), path = new URL(req.url()).pathname, method = req.method()
      if (path.endsWith('/finance/lookups')) return reply(route, { accounts, outlets: [{ id: 'outlet-a', name: 'Outlet A' }], work_units: [{ id: 'div-fnb', name: 'Divisi F & B', outlet_id: null }], banks: [], allow_central: true })
      if (path.endsWith('/finance/operations')) {
        if (fail) return route.fulfill({ status: 500, json: { error: 'Laporan gagal dimuat, coba ulang' } })
        const posted = entries.filter(e => e.status === 'posted')
        return reply(route, { rows: posted.map(e => ({ id: e.id, source: 'manual', source_id: e.id, date: e.occurred_at, description: e.description, notes: e.notes, reference: e.number, outlet_name: 'Outlet A', cash_in: 0, cash_out: e.amount, postings: [{ account_code: e.account_code, account_name: 'Beban Gaji & Upah', debit: e.amount, credit: 0 }, { account_code: '1-100', account_name: 'Kas', debit: 0, credit: e.amount }] })), total: posted.length, cash_in: 0, cash_out: posted.reduce((s,e) => s+e.amount,0), review_count: 0 })
      }
      if (path.endsWith('/upload')) return reply(route, { url: '/uploads/test/gaji.png' })
      if (path.endsWith('/finance/entries') && method === 'GET') return reply(route, entries)
      if (path.endsWith('/finance/entries') && method === 'POST') {
        const data = req.postDataJSON(); requests.push(data)
        entries.push({ ...data, id: 'entry-1', number: 'FIN-UJI-001', status: 'draft', version: 1, outlet_name: 'Outlet A', work_unit_name: '', history: [{ action: 'created', actor: 'Pencatat', at: new Date().toISOString(), notes: data.notes }] })
        return reply(route, entries[0])
      }
      if (path.endsWith('/entry-1/actions')) {
        const data = req.postDataJSON(); requests.push(data)
        entries[0].status = data.action === 'post' ? 'posted' : 'voided'; entries[0].version++
        entries[0].history.push({ action: data.action, actor: 'Supervisor', at: new Date().toISOString(), notes: data.reason })
        return reply(route, entries[0])
      }
      if (path.endsWith('/entry-1')) return reply(route, entries[0])
      errors.push('Unexpected API: ' + path); return route.abort()
    })
    await page.goto(base + '/tests/fixtures/finance.html')
    await page.getByRole('button', { name: '+ Pengeluaran', exact: true }).click()
    const dialog = page.getByRole('dialog'), form = page.locator('#finance-form')
    async function choose(label, term, option) {
      await form.locator('label').filter({ has: page.locator('.field-label', { hasText: label }) }).getByRole('combobox').click()
      await page.getByRole('combobox', { name: 'Cari pilihan' }).fill(term)
      await page.getByRole('option', { name: option, exact: true }).click()
    }
    await choose(/^Akun/, 'gaji', '5-310 · Beban Gaji & Upah')
    await form.getByLabel('Nominal (Rp) *', { exact: true }).fill('8000000')
    await form.locator('input[type=month]').fill('2026-10')
    await choose(/^Outlet/, 'Outlet A', 'Outlet A')
    await form.getByLabel('Dibayarkan kepada *', { exact: true }).fill('Tim Outlet A')
    await form.getByLabel('Uraian *', { exact: true }).fill('Gaji tim Oktober')
    await form.getByLabel('Catatan rinci', { exact: true }).fill('Total 8 staf divisi operasional')
    await form.locator('input[type=file]').setInputFiles({ name: 'gaji.png', mimeType: 'image/png', buffer: Buffer.from('test-image') })
    await dialog.getByRole('link', { name: 'Lihat bukti terunggah' }).waitFor()
    await dialog.getByRole('button', { name: 'Simpan Draf', exact: true }).click()
    await page.getByText('FIN-UJI-001', { exact: false }).waitFor()
    assert.equal(requests[0].account_code, '5-310'); assert.equal(requests[0].amount, 8000000)
    assert.equal(requests[0].period, '2026-10'); assert.equal(requests[0].outlet_id, 'outlet-a')
    assert.equal(requests[0].proof_url, '/uploads/test/gaji.png'); assert(requests[0].request_key)
    const post = page.getByRole('button', { name: 'Bukukan transaksi', exact: true })
    assert.equal((await post.textContent()).trim(), '')
    assert.equal(await post.locator('svg').count(), 1)
    await post.click(); await dialog.getByRole('button', { name: 'Konfirmasi Pembukuan' }).click()
    await page.getByRole('button', { name: 'Balik transaksi', exact: true }).waitFor()
    assert.equal(await page.getByRole('button', { name: 'Edit draf', exact: true }).count(), 0)
    console.log('PASS form gaji, pencarian akun/outlet, upload, draf dan pembukuan dengan ikon SVG')
    await page.getByRole('button', { name: 'Detail transaksi', exact: true }).click()
    await dialog.getByRole('heading', { name: 'Riwayat' }).waitFor()
    assert(await dialog.getByRole('link', { name: 'Lihat bukti transaksi' }).isVisible())
    await dialog.getByRole('button', { name: 'Tutup', exact: true }).click()
    await page.getByRole('button', { name: 'Semua Operasional', exact: true }).click()
    await page.getByRole('button', { name: 'Detail pencatatan', exact: true }).click()
    await dialog.locator('th').filter({ hasText: /^Debit$/ }).waitFor()
    await dialog.getByRole('button', { name: 'Tutup', exact: true }).click()
    if (process.env.FINANCE_SCREENSHOT) await page.screenshot({ path: process.env.FINANCE_SCREENSHOT, fullPage: true })
    await page.setViewportSize({ width: 390, height: 844 })
    assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth))
    await page.getByRole('button', { name: 'Transaksi Manual & Draf', exact: true }).click()
    await page.getByRole('button', { name: 'Balik transaksi', exact: true }).click()
    assert(await dialog.getByRole('button', { name: 'Konfirmasi Pembatalan' }).isDisabled())
    await dialog.getByLabel('Alasan *', { exact: true }).fill('Koreksi rekap gaji')
    await dialog.getByRole('button', { name: 'Konfirmasi Pembatalan' }).click()
    await page.getByText('Dibatalkan', { exact: false }).waitFor()
    assert.equal(requests[2].reason, 'Koreksi rekap gaji')
    assert.equal(await page.getByRole('button', { name: 'Balik transaksi', exact: true }).count(), 0)
    console.log('PASS detail, bukti, riwayat, pembalikan beralasan, dan layar ponsel')
    await page.evaluate(() => localStorage.setItem('cloud_pos_permissions', JSON.stringify(['finance.entries.view'])))
    // Init script sets permissions on navigation; overwrite it for the viewer check.
    await page.addInitScript(() => localStorage.setItem('cloud_pos_permissions', JSON.stringify(['finance.entries.view'])))
    await page.reload(); await page.getByRole('button', { name: 'Tampilkan', exact: true }).waitFor()
    assert.equal(await page.getByRole('button', { name: '+ Pengeluaran', exact: true }).count(), 0)
    fail = true; await page.getByRole('button', { name: 'Tampilkan', exact: true }).click()
    await page.getByText('Laporan gagal dimuat, coba ulang', { exact: true }).waitFor()
    assert.deepEqual(errors, [])
    console.log('PASS tampilan role pembaca dan galat API tanpa angka laporan lama')
  } finally { await browser.close() }
})().catch(e => { console.error(e); process.exitCode = 1 })
