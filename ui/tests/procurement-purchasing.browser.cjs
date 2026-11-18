// All API requests are synthetic. No production records are accessed.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright')
const assert = require('node:assert/strict')
const base = process.env.PROCUREMENT_TEST_URL || 'http://127.0.0.1:4189'
;(async () => {
  const browser = await chromium.launch({ args: ['--no-sandbox'] })
  try {
    const page = await browser.newPage({ viewport: { width: 1366, height: 900 }, acceptDownloads: true })
    page.setDefaultTimeout(12000)
    const errors = [], writes = [], exports = []
    let warehouses = [{ id: 'w1', name: 'Induk Utama' }, { id: 'w2', name: 'Induk Kedua' }]
    const items = [{ id: 'i1', code: 'GULA', name: 'Gula Kotak', unit: 'kotak', base_unit: 'buah', purchase_ratio: 12, is_active: true, stock_by_warehouse: { w1: 24, w2: 60 } },
      { id: 'i2', code: 'TEPUNG', name: 'Tepung Terigu', unit: 'kg', base_unit: 'g', purchase_ratio: 1000, is_active: true, stock_by_warehouse: { w1: 0 } },
      { id: 'i3', code: 'NONAKTIF', name: 'Bahan Nonaktif', unit: 'kg', is_active: false, stock_by_warehouse: {} }]
    const pr = { id: 'pr1', request_number: 'PR-BAHAN-001', request_type: 'bahan', work_unit_id: 'u1', work_unit_name: 'Gudang Induk', requested_by: 'Kepala Gudang', vendor_id: 'v1', vendor_name: 'Vendor Uji', status: 'approved', receipt_status: '', paid_amount: 0, struks: [], version: 'v1', created_at: '2026-11-10T01:00:00Z', total_hps: 100000, total_final: 100000, total_amount: 100000, items: [{ name: 'Belanja bahan', hps_total: 100000, final_total: 100000, items: [{ name: 'Gula Kotak', qty: 10, unit: 'kotak', hps_price: 10000, hps_subtotal: 100000, final_price: 10000, final_subtotal: 100000 }] }] }
    page.on('pageerror', error => errors.push(error.message))
    page.on('console', message => { if (message.text().includes('[Vue warn]')) errors.push(message.text()) })
    await page.route('**/api/v1/**', async route => {
      const request = route.request(), url = new URL(request.url()), path = url.pathname
      const reply = data => route.fulfill({ json: { success: true, data } })
      if (path.endsWith('/stock-item-names')) { assert.equal(url.searchParams.get('include_stock'), '1'); return reply({ items, warehouses, as_of: '2026-11-10T01:00:00Z' }) }
      if (path.endsWith('/my-work-units')) return reply([{ id: 'u1', name: 'Gudang Induk', outlet_id: '' }])
      if (path.endsWith('/work-units/me')) return reply({ id: 'u1', name: 'Gudang Induk' })
      if (path.endsWith('/vendors')) return reply({ vendors: [{ id: 'v1', name: 'Vendor Uji' }], total: 1 })
      if (path.endsWith('/purchasing/export')) { exports.push(Object.fromEntries(url.searchParams)); return route.fulfill({ body: 'synthetic-xlsx', contentType: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' }) }
      if (path.endsWith('/pr1/items') && request.method() === 'PUT') {
        const payload = request.postDataJSON(); writes.push(payload)
        assert.equal(payload.items[0].items[0].qty, 10)
        assert.equal(payload.items[0].items[0].purchased_qty, 8)
        pr.items = payload.items
        pr.items[0].final_total = 80000
        pr.items[0].items[0].final_subtotal = 80000
        pr.total_final = pr.total_amount = 80000
        return reply(pr)
      }
      if (path.endsWith('/purchase-requests/pr1')) return reply(pr)
      if (path.endsWith('/purchase-requests')) {
        if (request.method() === 'POST') { writes.push(request.postDataJSON()); return reply({ id: 'created' }) }
        return reply({ requests: [pr], total_pages: 1 })
      }
      errors.push(`Unexpected API ${request.method()} ${path}`); return route.abort()
    })
    await page.goto(base + '/tests/fixtures/procurement-purchasing.html')
    await page.getByRole('button', { name: '+ Buat Pengajuan', exact: true }).click()
    const create = page.getByRole('dialog', { name: 'Pengajuan Pembelian Bahan Baku', exact: true })
    await create.getByText('Pilih satu Gudang Induk untuk melihat stoknya.', { exact: true }).waitFor()
    await create.getByRole('combobox', { name: 'Gudang Induk untuk cek stok' }).click()
    await page.getByRole('option', { name: 'Induk Utama', exact: true }).click()
    await create.getByRole('combobox', { name: 'Bahan baku pengadaan 1 item 1', exact: true }).click()
    await page.getByRole('combobox', { name: 'Cari pilihan', exact: true }).fill('GULA')
    assert.equal(await page.getByRole('option').count(), 1)
    await page.getByRole('option', { name: 'Gula Kotak · GULA (kotak)', exact: true }).click()
    await create.getByText('Stok Induk Utama: 2 kotak (24 buah)', { exact: true }).waitFor()
    assert.equal(await create.getByPlaceholder('Satuan', { exact: true }).inputValue(), 'kotak')
    await create.getByRole('combobox', { name: 'Gudang Induk untuk cek stok', exact: true }).click()
    await page.getByRole('option', { name: 'Induk Kedua', exact: true }).click()
    await create.getByText('Stok Induk Kedua: 5 kotak (60 buah)', { exact: true }).waitFor()
    await create.getByRole('combobox', { name: 'Bahan baku pengadaan 1 item 1', exact: true }).click()
    assert.equal(await page.getByRole('option').count(), 2, 'Inactive catalog item hidden')
    await page.keyboard.press('Escape')
    await create.getByRole('button', { name: 'Batal', exact: true }).click()
    console.log('PASS searchable raw-material catalog, auto units, separate selected-warehouse stock, inactive items hidden')

    warehouses = [warehouses[0]]
    await page.getByRole('button', { name: '+ Buat Pengajuan', exact: true }).click()
    await create.getByText('Induk Utama', { exact: true }).waitFor()
    assert.equal(await create.getByRole('combobox', { name: 'Gudang Induk untuk cek stok', exact: true }).count(), 0)
    await create.getByRole('button', { name: 'Batal', exact: true }).click()
    console.log('PASS one accessible main warehouse is selected automatically')

    await page.locator('.hidden.sm\\:block').getByRole('button', { name: 'Lihat', exact: true }).first().click()
    const detail = page.getByRole('dialog').first()
    await detail.getByRole('button', { name: /Beli Semua di 1 Vendor/ }).click()
    const purchasing = page.getByRole('dialog', { name: 'Jumlah & Harga Pembelian (Purchasing)', exact: true })
    await purchasing.getByRole('spinbutton', { name: 'Jumlah dibeli Gula Kotak', exact: true }).fill('8')
    await purchasing.getByText(/80[.]000/).first().waitFor()
    assert((await purchasing.textContent()).includes('Diajukan: 10 kotak'))
    await purchasing.getByRole('button', { name: 'Simpan Pembelian', exact: true }).click()
    await purchasing.waitFor({ state: 'hidden' })
    assert.equal(writes.at(-1).items[0].items[0].purchased_qty, 8)
    await detail.getByRole('button', { name: 'Tutup', exact: true }).last().click()
    console.log('PASS purchasing 10 requested -> 8 bought, final total 80000, requested quantity preserved')
    pr.receipt_status = 'partial'
    await page.locator('.hidden.sm\\:block').getByRole('button', { name: 'Lihat', exact: true }).first().click()
    await detail.getByRole('button', { name: /Beli Semua di 1 Vendor/ }).click()
    await purchasing.getByRole('spinbutton', { name: 'Jumlah dibeli Gula Kotak', exact: true }).waitFor()
    assert(await purchasing.getByRole('spinbutton', { name: 'Jumlah dibeli Gula Kotak', exact: true }).isDisabled())
    await purchasing.getByRole('button', { name: 'Batal', exact: true }).click()
    await detail.getByRole('button', { name: 'Tutup', exact: true }).last().click()
    console.log('PASS purchased quantity locks after receipt has started')

    await page.getByRole('combobox', { name: 'Filter status pengadaan', exact: true }).click()
    await page.getByRole('option', { name: 'Disetujui', exact: true }).click()
    const download = page.waitForEvent('download')
    await page.getByRole('button', { name: 'Excel per Vendor', exact: true }).click()
    assert((await download).suggestedFilename().endsWith('.xlsx'))
    assert.equal(exports.at(-1).type, 'bahan'); assert.equal(exports.at(-1).status, 'approved'); assert.equal(exports.at(-1).parent_id, 'all')
    assert.deepEqual(errors, [])
    console.log('PASS vendor Excel download preserves type and status filters')
  } finally { await browser.close() }
})().catch(error => { console.error(error); process.exitCode = 1 })
