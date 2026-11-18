// All API requests are intercepted. No live vendors, accounts or payments are read/written.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright')
const assert = require('node:assert/strict')
const base = process.env.PAYMENT_MARKETPLACE_TEST_URL || 'http://127.0.0.1:4189'

;(async () => {
  const browser = await chromium.launch({ args: ['--no-sandbox'] })
  try {
    const page = await browser.newPage({ viewport: { width: 1366, height: 1000 } })
    const errors = [], payments = [], histories = []
    let rejectNextPayment = true
    const vendor = { id: 'v1', name: 'Toko Uji', bank_name: 'BCA', account_number: '111222333', account_holder: 'Toko Uji', bank_changes: [] }
    const bill = id => ({
      id, request_number: 'PR-BAHAN-' + id, request_type: 'bahan', requested_by: 'Kepala Gudang',
      work_unit_name: 'Gudang Induk', vendor_id: vendor.id, vendor_name: vendor.name,
      status: 'payment_requested', receipt_status: '', paid_amount: 0, total_final: 80000,
      total_hps: 100000, total_amount: 80000, created_at: '2026-11-10T01:00:00Z', struks: [],
      items: [{ name: 'Belanja bahan', hps_total: 100000, final_total: 80000, items: [
        { name: 'Gula Kotak', qty: 10, purchased_qty: 8, unit: 'kotak', hps_price: 10000, final_price: 10000, final_subtotal: 80000 },
      ] }],
    })
    const bills = [bill('pr1'), bill('pr2')]
    page.on('pageerror', error => errors.push(error.message))
    page.on('console', message => { if (message.text().includes('[Vue warn]')) errors.push(message.text()) })
    await page.route('**/api/v1/**', async route => {
      const request = route.request(), path = new URL(request.url()).pathname
      const reply = data => route.fulfill({ json: { success: true, data } })
      if (path.endsWith('/payment-stats')) return reply({ waiting: 2, total_waiting: 160000, accounts_payable: { total_amount: 160000 } })
      if (path.endsWith('/bank-accounts')) return reply([{ id: 'source-bank', bank_name: 'BNI', account_number: '555666777', account_holder: 'Perusahaan Uji', is_active: true }])
      if (path.endsWith('/projects')) return reply([])
      if (path.endsWith('/vendors/v1')) return reply(vendor)
      if (path.endsWith('/purchase-requests/pr1/payment-histories')) return reply(histories)
      if (path.endsWith('/purchase-requests/pr1/status') && request.method() === 'PUT') {
        const payload = request.postDataJSON()
        payments.push(payload)
        if (rejectNextPayment) {
          rejectNextPayment = false
          return route.fulfill({ status: 400, json: { success: false, error: 'Virtual account pesanan sudah berubah, periksa kembali.' } })
        }
        bills[0].status = 'paid'
        bills[0].paid_amount = 80000
        bills[0].payment_account_dest = `VA ${payload.payment_bank_name} ${payload.payment_account_number} a.n. ${payload.payment_account_holder}`
        histories.push({
          id: 'payment-1', amount: 80000, paid_by: 'Keuangan Uji', created_at: '2026-11-10T02:00:00Z',
          vendor_name: vendor.name, payment_account_dest: bills[0].payment_account_dest,
          dest_manual: true, payment_method: payload.payment_method,
          payment_marketplace: payload.payment_marketplace, payment_order_number: payload.payment_order_number,
          payment_account_source: payload.payment_account_source,
        })
        return reply(bills[0])
      }
      const found = bills.find(row => path.endsWith('/purchase-requests/' + row.id))
      if (found) return reply(found)
      if (path.endsWith('/purchase-requests') && request.method() === 'GET') return reply({ requests: bills, total_pages: 1 })
      errors.push(`Unexpected API ${request.method()} ${path}`)
      return route.abort()
    })

    await page.goto(base + '/tests/fixtures/payment-marketplace.html')
    await page.getByRole('button', { name: 'Bayar', exact: true }).first().click()
    const dialog = page.getByRole('dialog', { name: 'Pembayaran Pengajuan', exact: true })
    const method = dialog.getByLabel('Metode Pembayaran', { exact: true })
    await method.waitFor({ state: 'visible' })
    await page.waitForFunction(() => !document.querySelector('#payment-method').disabled)
    assert.equal(await method.inputValue(), 'vendor_account')
    await dialog.getByText('BCA 111222333 a.n. Toko Uji', { exact: true }).waitFor()
    await method.selectOption('marketplace_va')
    await dialog.getByLabel('Nomor Virtual Account *', { exact: true }).waitFor()
    assert.equal(await dialog.getByText('BCA 111222333 a.n. Toko Uji', { exact: true }).count(), 0)
    console.log('PASS marketplace VA selectable despite existing registered vendor account')

    const field = name => dialog.getByLabel(name, { exact: true })
    const confirm = dialog.getByRole('button', { name: 'Konfirmasi Pembayaran', exact: true })
    const required = 'Marketplace, nomor pesanan, bank, dan nomor virtual account wajib diisi.'
    await field('Marketplace *').fill('Shopee')
    await field('Bank Virtual Account *').fill('Mandiri')
    await field('Nomor Virtual Account *').fill('001234567890')
    await confirm.click()
    await page.getByRole('alert').filter({ hasText: required }).first().waitFor()
    assert(await dialog.isVisible())
    assert.equal(payments.length, 0, 'Missing order must not reach payment API')
    assert.equal(await field('Nomor Virtual Account *').inputValue(), '001234567890')

    await field('Nomor Pesanan *').fill('ORDER-SHOPEE-001')
    await field('Nomor Virtual Account *').fill('')
    await confirm.click()
    assert(await dialog.isVisible())
    assert.equal(payments.length, 0, 'Missing VA must not reach payment API')
    await field('Nomor Virtual Account *').fill('00INVALID')
    await confirm.click()
    await page.getByRole('alert').filter({ hasText: 'Nomor virtual account harus berisi angka.' }).waitFor()
    assert.equal(payments.length, 0, 'Invalid VA must not reach payment API')
    console.log('PASS order/VA required and numeric validation preserves entered form')

    await field('Nomor Virtual Account *').fill('001234567890')
    await field('Atas Nama Virtual Account').fill('Shopee Checkout')
    await dialog.getByRole('combobox', { name: 'Rekening Asal (Perusahaan)', exact: true }).click()
    await page.getByRole('option', { name: 'BNI 555666777 a.n. Perusahaan Uji', exact: true }).click()
    await confirm.click()
    await page.getByRole('alert').filter({ hasText: 'Virtual account pesanan sudah berubah, periksa kembali.' }).waitFor()
    assert(await dialog.isVisible(), 'Server validation must retain the payment form')
    assert.equal(await field('Nomor Pesanan *').inputValue(), 'ORDER-SHOPEE-001')
    assert.equal(await field('Nomor Virtual Account *').inputValue(), '001234567890')
    assert.equal(payments.length, 1)
    console.log('PASS server rejection retains order, VA, and form for correction')

    await confirm.click()
    await dialog.waitFor({ state: 'hidden' })
    assert.equal(payments.length, 2)
    const saved = payments[1]
    assert.equal(saved.payment_method, 'marketplace_va')
    assert.equal(saved.payment_marketplace, 'Shopee')
    assert.equal(saved.payment_order_number, 'ORDER-SHOPEE-001')
    assert.equal(typeof saved.payment_account_number, 'string')
    assert.equal(saved.payment_account_number, '001234567890')
    assert.equal(saved.payment_bank_name, 'Mandiri')
    assert.equal(saved.payment_account_holder, 'Shopee Checkout')
    assert.equal(saved.payment_account_source, 'BNI 555666777 a.n. Perusahaan Uji')
    assert.equal(vendor.account_number, '111222333')
    console.log('PASS payment payload preserves leading-zero VA, method, order, and company source')

    await page.getByRole('button', { name: 'Lihat Detail', exact: true }).first().click()
    const detail = page.getByRole('dialog', { name: 'Detail Pengajuan', exact: true })
    await detail.getByText('VA Marketplace', { exact: true }).waitFor()
    await detail.getByText('Shopee · ORDER-SHOPEE-001', { exact: true }).waitFor()
    assert((await detail.textContent()).includes('VA Mandiri 001234567890 a.n. Shopee Checkout'))
    await detail.getByRole('button', { name: 'Tutup', exact: true }).last().click()
    await detail.waitFor({ state: 'hidden' })
    console.log('PASS history displays recorded marketplace, order, and VA destination')

    await page.getByRole('button', { name: 'Bayar', exact: true }).click()
    await page.waitForFunction(() => !document.querySelector('#payment-method').disabled)
    assert.equal(await method.inputValue(), 'vendor_account')
    await method.selectOption('marketplace_va')
    assert.equal(await field('Nomor Virtual Account *').inputValue(), '')
    assert.equal(await field('Nomor Pesanan *').inputValue(), '')
    await dialog.getByRole('button', { name: 'Batal', exact: true }).click()
    assert.deepEqual(errors, [])
    console.log('PASS next payment starts without previous order/VA and no browser/Vue errors')
  } finally {
    await browser.close()
  }
})().catch(error => { console.error(error); process.exitCode = 1 })
