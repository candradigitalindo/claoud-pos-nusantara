// Tujuan pembayaran diisi bagian Pembelian (keputusan owner 19 Nov 2026):
// dialog Ajukan Pembayaran memuat cara bayar vendor, detail yang dikembalikan
// Keuangan tampil di daftar & detail, dan Pembelian memperbaikinya.
// All API requests are synthetic. No production records are accessed.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright')
const assert = require('node:assert/strict')
const base = process.env.PROCUREMENT_TEST_URL || 'http://127.0.0.1:4189'
;(async () => {
  const browser = await chromium.launch({ args: ['--no-sandbox'] })
  try {
    const page = await browser.newPage({ viewport: { width: 1366, height: 900 } })
    page.setDefaultTimeout(12000)
    const errors = [], statusWrites = [], detailWrites = []
    const noAccount = { vendor_bank_name: '', vendor_account_number: '', vendor_account_holder: '' }
    const pr = {
      id: 'pr1', request_number: 'PR-BAHAN-001', request_type: 'bahan', work_unit_id: 'u1', work_unit_name: 'Gudang Induk',
      requested_by: 'Kepala Gudang', vendor_id: 'v1', vendor_name: 'Toko Online Uji', status: 'approved', receipt_status: '',
      paid_amount: 0, struks: [], version: 'v1', created_at: '2026-11-10T01:00:00Z', total_hps: 100000, total_final: 80000, total_amount: 80000,
      items: [{ name: 'Belanja bahan', hps_total: 100000, final_total: 80000, items: [{ name: 'Gula Kotak', qty: 10, unit: 'kotak', hps_price: 10000, hps_subtotal: 100000, final_price: 8000, final_subtotal: 80000 }] }],
      pay_to: { method: '', effective: '', ready: false, problem: 'Pembelian belum mengisi detail pembayaran (vendor belum punya rekening terdaftar)', returned_at: null, ...noAccount },
    }
    page.on('pageerror', error => errors.push(error.message))
    page.on('console', message => { if (message.text().includes('[Vue warn]')) errors.push(message.text()) })
    await page.route('**/api/v1/**', async route => {
      const request = route.request(), url = new URL(request.url()), path = url.pathname
      const reply = data => route.fulfill({ json: { success: true, data } })
      if (path.endsWith('/stock-item-names')) return reply({ items: [], warehouses: [{ id: 'w1', name: 'Induk Utama' }], as_of: '2026-11-10T01:00:00Z' })
      if (path.endsWith('/my-work-units')) return reply([{ id: 'u1', name: 'Gudang Induk', outlet_id: '' }])
      if (path.endsWith('/work-units/me')) return reply({ id: 'u1', name: 'Gudang Induk' })
      if (path.endsWith('/vendors')) return reply({ vendors: [{ id: 'v1', name: 'Toko Online Uji' }], total: 1 })
      if (path.endsWith('/purchase-requests/pr1/status') && request.method() === 'PUT') {
        const payload = request.postDataJSON(); statusWrites.push(payload)
        pr.status = 'payment_requested'; pr.version = 'v2'
        pr.pay_to = { ...pr.pay_to, ...payload.pay_to, effective: payload.pay_to.method, ready: true, problem: '', set_by: 'Kepala Gudang Uji', set_at: '2026-11-19T01:00:00Z' }
        return reply(pr)
      }
      if (path.endsWith('/purchase-requests/pr1/payment-details') && request.method() === 'PUT') {
        const payload = request.postDataJSON(); detailWrites.push(payload)
        pr.version = 'v4'
        pr.pay_to = { ...pr.pay_to, ...payload, effective: payload.method, ready: true, problem: '', returned_at: null, returned_by: '', return_reason: '' }
        return reply(pr)
      }
      if (path.endsWith('/purchase-requests/pr1')) return reply(pr)
      if (path.endsWith('/purchase-requests') && request.method() === 'GET') return reply({ requests: [pr], total_pages: 1 })
      errors.push(`Unexpected API ${request.method()} ${path}`); return route.abort()
    })
    await page.goto(base + '/tests/fixtures/procurement-purchasing.html')
    const desktop = page.locator('.hidden.sm\\:block')
    const openDetail = async () => {
      await desktop.getByRole('button', { name: 'Lihat', exact: true }).first().click()
      const d = page.getByRole('dialog', { name: 'Detail Pengajuan Bahan Baku', exact: true })
      await d.waitFor()
      return d
    }

    // ── Ajukan Pembayaran: vendor tanpa rekening → wajib VA / tunai ─────────
    let detail = await openDetail()
    await detail.getByRole('button', { name: 'Ajukan Pembayaran', exact: true }).click()
    const ask = page.getByRole('dialog', { name: 'Ajukan Pembayaran', exact: true })
    await ask.getByText('Cara Bayar ke Vendor', { exact: true }).waitFor()
    assert(await ask.locator('input[type=radio][value=vendor_account]').isDisabled(), 'No registered account → option disabled')
    assert(!(await ask.locator('input[type=radio][value=manual]').isDisabled()), 'Cash allowed for vendor without account')
    const submit = ask.getByRole('button', { name: 'Ajukan Pembayaran', exact: true })
    await submit.click()
    await ask.getByText('Pilih cara bayar ke vendor.', { exact: true }).waitFor()
    assert.equal(statusWrites.length, 0)
    await ask.getByText('VA Marketplace', { exact: true }).click()
    await ask.getByLabel('Marketplace *', { exact: true }).fill('Shopee')
    await ask.getByLabel('Bank Virtual Account *', { exact: true }).fill('BCA')
    await ask.getByLabel('Nomor Virtual Account *', { exact: true }).fill('00123x')
    await submit.click()
    await ask.getByText('Marketplace, nomor pesanan, bank, dan nomor virtual account wajib diisi.', { exact: true }).waitFor()
    await ask.getByLabel('Nomor Pesanan *', { exact: true }).fill('SO-2611-001')
    await submit.click()
    await ask.getByText('Nomor virtual account harus berisi angka.', { exact: true }).waitFor()
    assert.equal(statusWrites.length, 0, 'Invalid details never reach the API')
    await ask.getByLabel('Nomor Virtual Account *', { exact: true }).fill(' 0012345 ')
    await ask.getByLabel(/Catatan untuk Keuangan/).fill('Bayar sebelum 15.00')
    await submit.click()
    await ask.waitFor({ state: 'hidden' })
    assert.deepEqual(statusWrites, [{
      action: 'request_payment', actor_name: 'Kepala Gudang Uji', version: 'v1',
      pay_to: { method: 'marketplace_va', note: 'Bayar sebelum 15.00', marketplace: 'Shopee', order_number: 'SO-2611-001', bank_name: 'BCA', account_number: '0012345', account_holder: '' },
    }])
    console.log('PASS request payment requires purchasing to choose how the vendor is paid (VA validated, leading zeros kept)')

    // Detail menampilkan tujuan + tombol Ubah.
    await detail.getByText('Virtual Account Marketplace', { exact: true }).waitFor()
    await detail.getByText('0012345', { exact: true }).waitFor()
    await detail.getByRole('button', { name: 'Ubah Detail Pembayaran', exact: true }).waitFor()
    await detail.getByRole('button', { name: 'Tutup', exact: true }).last().click()
    console.log('PASS detail shows the destination set by purchasing with an edit action')

    // ── Dikembalikan Keuangan → lencana + Perbaiki ──────────────────────────
    pr.version = 'v3'
    pr.pay_to = { ...pr.pay_to, ready: false, returned_at: '2026-11-19T02:00:00Z', returned_by: 'Sari Keuangan',
      return_reason: 'VA sudah kedaluwarsa', problem: 'detail pembayaran dikembalikan ke Pembelian: VA sudah kedaluwarsa' }
    await page.reload()
    await desktop.getByText('Detail bayar dikembalikan', { exact: true }).waitFor()
    detail = await openDetail()
    await detail.getByText('VA sudah kedaluwarsa', { exact: true }).waitFor()
    await detail.getByRole('button', { name: 'Perbaiki Detail Pembayaran', exact: true }).click()
    const fix = page.getByRole('dialog', { name: 'Perbaiki Detail Pembayaran', exact: true })
    await fix.getByText('oleh Sari Keuangan', { exact: false }).waitFor()
    assert.equal(await fix.getByLabel('Nomor Pesanan *', { exact: true }).inputValue(), 'SO-2611-001', 'Form starts from the previous details')
    await fix.getByLabel('Nomor Virtual Account *', { exact: true }).fill('0099887')
    await fix.getByRole('button', { name: 'Simpan & Kirim ke Keuangan', exact: true }).click()
    await fix.waitFor({ state: 'hidden' })
    assert.deepEqual(detailWrites, [{
      method: 'marketplace_va', note: 'Bayar sebelum 15.00', marketplace: 'Shopee', order_number: 'SO-2611-001',
      bank_name: 'BCA', account_number: '0099887', account_holder: '', version: 'v3',
    }])
    // (Fixture ini tidak memasang ToastContainer — yang diperiksa detail terbaru.)
    await detail.getByText('0099887', { exact: true }).waitFor()
    assert.equal(await detail.getByRole('button', { name: 'Perbaiki Detail Pembayaran', exact: true }).count(), 0)
    console.log('PASS returned details are fixed by purchasing with the version they saw')

    // ── Vendor berekening: rekening terdaftar, tunai nonaktif ───────────────
    await detail.getByRole('button', { name: 'Tutup', exact: true }).last().click()
    pr.pay_to = { ...pr.pay_to, vendor_bank_name: 'BNI', vendor_account_number: '0123456789', vendor_account_holder: 'Toko Online Uji' }
    await page.reload()
    detail = await openDetail()
    await detail.getByRole('button', { name: 'Ubah Detail Pembayaran', exact: true }).click()
    const edit = page.getByRole('dialog', { name: 'Ubah Detail Pembayaran', exact: true })
    assert(await edit.locator('input[type=radio][value=manual]').isDisabled(), 'Vendor with account cannot be paid "manual"')
    await edit.getByText('Rekening Vendor', { exact: true }).click()
    await edit.getByText('0123456789', { exact: true }).waitFor()
    assert.equal(await edit.getByLabel('Nomor Virtual Account *', { exact: true }).count(), 0, 'Registered account cannot be typed')
    await edit.getByRole('button', { name: 'Batal', exact: true }).click()
    assert.deepEqual(errors, [])
    console.log('PASS registered vendor account is shown read-only; cash disabled; no browser/Vue errors')
  } finally { await browser.close() }
})().catch(error => { console.error(error); process.exitCode = 1 })
