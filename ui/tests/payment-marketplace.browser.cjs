// Halaman Pembayaran (Keuangan) sejak 19 Nov 2026: tujuan pembayaran diisi
// bagian Pembelian (pay_to) — Keuangan hanya membaca + menyalin, memilih
// sumber dana, dan mengembalikan detail yang salah ke Pembelian.
// All API requests are intercepted. No live vendors, accounts or payments are read/written.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright')
const assert = require('node:assert/strict')
const base = process.env.PAYMENT_MARKETPLACE_TEST_URL || 'http://127.0.0.1:4189'

;(async () => {
  const browser = await chromium.launch({ args: ['--no-sandbox'] })
  try {
    const context = await browser.newContext({ viewport: { width: 1366, height: 1000 }, permissions: ['clipboard-read', 'clipboard-write'] })
    const page = await context.newPage()
    page.setDefaultTimeout(12000)
    const errors = [], payments = [], returns = []
    let conflictNextPayment = true
    const vendor = { id: 'v1', name: 'Toko Uji', bank_name: 'BCA', account_number: '111222333', account_holder: 'Toko Uji', bank_changes: [] }
    const ready = (extra) => ({
      method: 'vendor_account', effective: 'vendor_account', bank_name: 'BCA', account_number: '111222333', account_holder: 'Toko Uji',
      marketplace: '', order_number: '', text: '', note: '', set_by: 'Budi Pembelian', set_at: '2026-11-19T01:00:00Z',
      returned_at: null, returned_by: '', return_reason: '', ready: true, problem: '',
      vendor_bank_name: 'BCA', vendor_account_number: '111222333', vendor_account_holder: 'Toko Uji', ...extra,
    })
    const bill = (id, payTo, extra = {}) => ({
      id, request_number: 'PR-BAHAN-' + id, request_type: 'bahan', requested_by: 'Kepala Gudang',
      work_unit_name: 'Gudang Induk', vendor_id: vendor.id, vendor_name: vendor.name, outlet_id: 'o1', outlet_name: 'Outlet Uji',
      status: 'payment_requested', receipt_status: '', paid_amount: 0, total_final: 80000, version: id + '-v1',
      total_hps: 100000, total_amount: 80000, created_at: '2026-11-10T01:00:00Z', struks: [], pay_to: payTo,
      items: [{ name: 'Belanja bahan', hps_total: 100000, final_total: 80000, items: [
        { name: 'Gula Kotak', qty: 10, purchased_qty: 8, unit: 'kotak', hps_price: 10000, final_price: 10000, final_subtotal: 80000 },
      ] }],
      ...extra,
    })
    const vaPayTo = ready({
      method: 'marketplace_va', effective: 'marketplace_va', marketplace: 'Shopee', order_number: 'ORDER-SHOPEE-001',
      bank_name: 'Mandiri', account_number: '001234567890', account_holder: 'Shopee Checkout', note: 'VA kedaluwarsa pukul 15.00',
    })
    const bills = [
      bill('pr1', vaPayTo),
      bill('pr2', ready({})),
      bill('pr3', ready({ ready: false, returned_at: '2026-11-19T02:00:00Z', returned_by: 'Sari Keuangan',
        return_reason: 'Rekening tidak sesuai invoice', problem: 'detail pembayaran dikembalikan ke Pembelian: Rekening tidak sesuai invoice' })),
    ]
    page.on('pageerror', error => errors.push(error.message))
    page.on('console', message => { if (message.text().includes('[Vue warn]')) errors.push(message.text()) })
    await page.route('**/api/v1/**', async route => {
      const request = route.request(), path = new URL(request.url()).pathname
      const reply = data => route.fulfill({ json: { success: true, data } })
      if (path.endsWith('/payment-stats')) return reply({ waiting: 3, total_waiting: 240000, accounts_payable: { total_amount: 240000 } })
      if (path.endsWith('/bank-accounts')) return reply([{ id: 'source-bank', bank_name: 'BNI', account_number: '555666777', account_holder: 'Perusahaan Uji', is_active: true }])
      if (path.endsWith('/projects')) return reply([])
      if (path.endsWith('/vendors/v1')) return reply(vendor)
      if (/\/purchase-requests\/pr\d\/payment-histories$/.test(path)) return reply([])
      if (path.endsWith('/purchase-requests/pr1/status') && request.method() === 'PUT') {
        const payload = request.postDataJSON()
        payments.push(payload)
        if (conflictNextPayment) {
          // Pembelian mengganti VA setelah form dibuka → server menolak 409.
          conflictNextPayment = false
          bills[0].version = 'pr1-v2'
          bills[0].pay_to = { ...vaPayTo, order_number: 'ORDER-SHOPEE-002', account_number: '009876543210' }
          return route.fulfill({ status: 409, json: { success: false, error: 'Pengajuan berubah, muat ulang' } })
        }
        bills[0].status = 'paid'
        bills[0].paid_amount = 80000
        return reply(bills[0])
      }
      if (path.endsWith('/purchase-requests/pr2/payment-details/return') && request.method() === 'POST') {
        const payload = request.postDataJSON()
        returns.push(payload)
        bills[1].pay_to = { ...bills[1].pay_to, ready: false, returned_at: '2026-11-19T03:00:00Z', returned_by: 'Keuangan Uji',
          return_reason: payload.reason, problem: 'detail pembayaran dikembalikan ke Pembelian: ' + payload.reason }
        return reply(bills[1])
      }
      const found = bills.find(row => path.endsWith('/purchase-requests/' + row.id))
      if (found) return reply(found)
      if (path.endsWith('/purchase-requests') && request.method() === 'GET') return reply({ requests: bills, total_pages: 1 })
      errors.push(`Unexpected API ${request.method()} ${path}`)
      return route.abort()
    })

    await page.goto(base + '/tests/fixtures/payment-marketplace.html')
    const rowOf = number => page.locator('tbody tr').filter({ hasText: number })
    await rowOf('PR-BAHAN-pr3').getByText('Dikembalikan ke Pembelian', { exact: true }).waitFor()
    await rowOf('PR-BAHAN-pr3').getByText('Alasan: Rekening tidak sesuai invoice', { exact: true }).waitFor()
    assert.equal(await rowOf('PR-BAHAN-pr3').getByRole('button', { name: 'Bayar', exact: true }).count(), 0)
    assert.equal(await page.getByRole('button', { name: 'Bayar', exact: true }).count(), 2)
    console.log('PASS returned bill shows reason and cannot be paid')

    await rowOf('PR-BAHAN-pr1').getByRole('button', { name: 'Bayar', exact: true }).click()
    const dialog = page.getByRole('dialog', { name: 'Pembayaran Pengajuan', exact: true })
    await dialog.getByText('Virtual Account Marketplace', { exact: true }).waitFor()
    await dialog.getByText('ORDER-SHOPEE-001', { exact: true }).waitFor()
    await dialog.getByText('001234567890', { exact: true }).waitFor()
    await dialog.getByText('VA kedaluwarsa pukul 15.00').waitFor()
    assert.equal(await page.locator('#payment-method').count(), 0, 'Finance no longer chooses the payment method')
    assert.equal(await dialog.getByLabel('Nomor Virtual Account *', { exact: true }).count(), 0, 'Finance no longer types the VA')
    await dialog.getByRole('button', { name: 'Salin No. VA', exact: true }).click()
    await page.getByRole('alert').filter({ hasText: 'No. VA disalin.' }).first().waitFor()
    assert.equal(await page.evaluate(() => navigator.clipboard.readText()), '001234567890', 'Leading zeros preserved when copying')
    await dialog.getByRole('button', { name: 'Salin Nominal', exact: true }).click()
    assert.equal(await page.evaluate(() => navigator.clipboard.readText()), '80000')
    console.log('PASS VA destination is read-only with copy buttons (leading zeros kept, nominal digits)')

    const outletCash = dialog.locator('input[type=radio][value=outlet_cash]')
    assert(await outletCash.isDisabled(), 'Outlet cash is not offered for a marketplace VA')
    await dialog.getByText('VA Marketplace dibayar lewat transfer', { exact: true }).waitFor()
    console.log('PASS marketplace VA can only be paid by head-office transfer')

    const confirm = dialog.getByRole('button', { name: 'Konfirmasi Pembayaran', exact: true })
    await confirm.click()
    await page.getByRole('alert').filter({ hasText: 'Rekening asal wajib diisi.' }).first().waitFor()
    assert.equal(payments.length, 0)
    await dialog.getByRole('combobox', { name: 'Rekening Asal (Perusahaan)', exact: true }).click()
    await page.getByRole('option', { name: 'BNI 555666777 a.n. Perusahaan Uji', exact: true }).click()
    await confirm.click()
    await page.getByRole('alert').filter({ hasText: 'pengajuan berubah sejak form dibuka' }).first().waitFor()
    await dialog.waitFor({ state: 'hidden' })
    assert.equal(payments.length, 1)
    const first = payments[0]
    assert.equal(first.action, 'pay')
    assert.equal(first.version, 'pr1-v1', 'Payment carries the version finance looked at')
    for (const key of ['payment_method', 'payment_account_dest', 'payment_account_number', 'payment_bank_name', 'payment_marketplace', 'payment_order_number']) {
      assert(!(key in first), `${key} must not be sent — destination belongs to purchasing`)
    }
    assert.equal(first.payment_account_source, 'BNI 555666777 a.n. Perusahaan Uji')
    console.log('PASS payment sends version + company source only; 409 closes the form and reloads')

    await rowOf('PR-BAHAN-pr1').getByRole('button', { name: 'Bayar', exact: true }).click()
    await dialog.getByText('009876543210', { exact: true }).waitFor()
    await dialog.getByRole('combobox', { name: 'Rekening Asal (Perusahaan)', exact: true }).click()
    await page.getByRole('option', { name: 'BNI 555666777 a.n. Perusahaan Uji', exact: true }).click()
    await confirm.click()
    await dialog.waitFor({ state: 'hidden' })
    assert.equal(payments.length, 2)
    assert.equal(payments[1].version, 'pr1-v2')
    console.log('PASS reopened form shows the new VA and pays with the new version')

    await rowOf('PR-BAHAN-pr2').getByRole('button', { name: 'Kembalikan detail pembayaran ke Pembelian', exact: true }).click()
    const ret = page.getByRole('dialog', { name: 'Kembalikan ke Pembelian', exact: true })
    const send = ret.getByRole('button', { name: 'Kembalikan', exact: true })
    await ret.getByLabel('Alasan *', { exact: true }).fill('abc')
    assert(await send.isDisabled(), 'Reason shorter than 5 letters is rejected')
    await ret.getByLabel('Alasan *', { exact: true }).fill('Rekening bukan milik vendor')
    await send.click()
    await ret.waitFor({ state: 'hidden' })
    assert.deepEqual(returns, [{ reason: 'Rekening bukan milik vendor', version: 'pr2-v1' }])
    await rowOf('PR-BAHAN-pr2').getByText('Dikembalikan ke Pembelian', { exact: true }).waitFor()
    assert.equal(await rowOf('PR-BAHAN-pr2').getByRole('button', { name: 'Bayar', exact: true }).count(), 0)
    console.log('PASS finance returns wrong details to purchasing with a reason; bill locks')

    await rowOf('PR-BAHAN-pr3').getByRole('button', { name: 'Lihat Detail', exact: true }).click()
    const detail = page.getByRole('dialog', { name: 'Detail Pengajuan', exact: true })
    await detail.getByText('Rekening tidak sesuai invoice', { exact: true }).waitFor()
    assert.equal(await detail.getByRole('button', { name: 'Bayar', exact: true }).count(), 0)
    await detail.getByRole('button', { name: 'Tutup', exact: true }).last().click()
    assert.deepEqual(errors, [])
    console.log('PASS detail shows the return reason; no browser/Vue errors')
  } finally {
    await browser.close()
  }
})().catch(error => { console.error(error); process.exitCode = 1 })
