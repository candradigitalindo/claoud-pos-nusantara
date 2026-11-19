// Tingkat kebutuhan & Pembelian Darurat (keputusan owner 19 Nov 2026): lencana
// URGENT/DARURAT di daftar, form darurat menolak di atas batas Rp500.000 dan
// mengirim isian yang benar, panel detail mengesahkan dengan versi dokumen.
// All API requests are synthetic. No production records are accessed.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright')
const assert = require('node:assert/strict')
const base = process.env.PROCUREMENT_TEST_URL || 'http://127.0.0.1:4189'
;(async () => {
  const browser = await chromium.launch({ args: ['--no-sandbox'] })
  try {
    const page = await browser.newPage({ viewport: { width: 1366, height: 900 } })
    page.setDefaultTimeout(12000)
    const errors = [], created = [], decisions = []
    const urgent = { id: 'pr-u', request_number: 'PR-URG-1', request_type: 'barang', status: 'pending', urgency: 'urgent', need_by_date: '2026-11-20',
      work_unit_name: 'Healing & Chill', requested_by: 'Manajer Uji', total_amount: 1250000, items: [{ name: 'Pompa air toilet', items: [] }], created_at: '2026-11-19T05:00:00Z' }
    const emergency = { id: 'pr-e', request_number: 'PR-DRT-1', request_type: 'barang', status: 'paid', urgency: 'urgent', need_by_date: '2026-11-19',
      work_unit_name: 'Healing & Chill', requested_by: 'Manajer Uji', vendor_name: 'Toko Teknik Jaya', version: 'v-e1',
      total_amount: 300000, total_final: 300000, total_hps: 300000, paid_amount: 300000, paid_at: '2026-11-19T05:10:00Z', struks: [], pickups: [],
      items: [{ name: 'DARURAT: Steam valve', hps_total: 300000, final_total: 300000, items: [{ name: 'Steam valve', qty: 1, unit: 'pcs', hps_price: 300000, final_price: 300000, hps_subtotal: 300000, final_subtotal: 300000 }] }],
      pay_to: { ready: false, problem: 'Pembelian Darurat dibayar tunai dari Kas Kasir Outlet lewat tablet kasir' },
      emergency: { category: 'peralatan_rusak', category_label: 'Peralatan operasional rusak/mati', reason: 'Steam valve bocor, bar berhenti', recipient: 'Adri',
        proof_url: '/uploads/struk.jpg', proof_total: 300000, proof_by: 'Manajer Uji', ratification: 'pending' },
      created_at: '2026-11-19T05:05:00Z' }
    page.on('pageerror', error => errors.push(error.message))
    page.on('console', message => { if (message.text().includes('[Vue warn]')) errors.push(message.text()) })
    await page.route('**/api/v1/**', async route => {
      const request = route.request(), url = new URL(request.url()), path = url.pathname
      const reply = data => route.fulfill({ json: { success: true, data } })
      if (path.endsWith('/stock-item-names')) return reply({ items: [], warehouses: [] })
      if (path.endsWith('/my-work-units')) return reply([{ id: 'u1', name: 'Healing & Chill', outlet_id: 'o1', outlet_name: 'Nusantara Healing & Chill' }])
      if (path.endsWith('/work-units/me')) return reply({ id: 'u1', name: 'Healing & Chill' })
      if (path.endsWith('/vendors')) return reply({ vendors: [], total: 0 })
      if (path.endsWith('/admin/projects')) return reply([])
      if (path.endsWith('/procurement/emergency-settings')) return reply({ max_amount: 500000, monthly_cap: 0 })
      if (path.endsWith('/purchase-requests/emergency/context')) {
        assert.equal(url.searchParams.get('work_unit_id'), 'u1')
        return reply({ max_amount: 500000, monthly_cap: 0, outlet_id: 'o1', outlet_name: 'Nusantara Healing & Chill', used_this_month: 450000, count_this_month: 1,
          remaining: null, tablet_ready: false, tablet_version: '1.2.0+93421153', tablet_note: 'Tablet kasir outlet ini belum memakai App POS v10.20.' })
      }
      if (path.endsWith('/purchase-requests/emergency') && request.method() === 'POST') {
        created.push(request.postDataJSON())
        return route.fulfill({ status: 201, json: { success: true, data: { ...emergency, id: 'pr-new', request_number: 'PR-DRT-2', status: 'payment_requested', paid_amount: 0,
          emergency: { ...emergency.emergency, proof_url: '', proof_total: null }, cash_payout: { status: 'pending' } } } })
      }
      if (path.endsWith('/purchase-requests/pr-new')) return reply({ ...emergency, id: 'pr-new', request_number: 'PR-DRT-2', status: 'payment_requested', paid_amount: 0,
        emergency: { ...emergency.emergency, proof_url: '', proof_total: null }, cash_payout: { status: 'pending' } })
      if (path.endsWith('/purchase-requests/pr-e/emergency/decision')) {
        decisions.push(request.postDataJSON())
        emergency.emergency = { ...emergency.emergency, ratification: 'ratified', ratified_by: 'Owner Uji', ratified_at: '2026-11-19T06:00:00Z' }
        return reply(emergency)
      }
      if (path.endsWith('/purchase-requests/pr-e')) return reply(emergency)
      if (path.endsWith('/purchase-requests')) return reply({ requests: [emergency, urgent], total_pages: 1 })
      errors.push(`Unexpected API ${request.method()} ${path}`); return route.abort()
    })
    await page.goto(base + '/tests/fixtures/procurement-emergency.html')
    const table = page.locator('table')
    await table.getByText('Urgent · butuh 20 Nov', { exact: false }).waitFor()
    await table.getByText('Darurat', { exact: true }).waitFor()
    await table.getByText('Menunggu Pengesahan', { exact: true }).waitFor()

    // Form darurat: peringatan tablet, batas Rp500.000, isian yang dikirim.
    await page.getByRole('button', { name: 'Pembelian Darurat', exact: true }).click()
    const dlg = page.getByRole('dialog', { name: 'Pembelian Darurat — Barang', exact: true })
    await dlg.getByText('Tablet kasir outlet ini belum memakai App POS v10.20.', { exact: false }).waitFor()
    await dlg.getByText('Kerusakan bangunan/instalasi', { exact: true }).click()
    await dlg.locator('textarea').fill('Atap dapur bocor, air menetes ke kompor')
    await dlg.getByPlaceholder('mis. Steam valve mesin kopi', { exact: true }).fill('Seng + paku atap')
    await dlg.getByPlaceholder('Harga satuan', { exact: true }).fill('600000')
    await dlg.getByText('Melebihi batas Pembelian Darurat', { exact: false }).waitFor()
    const submit = dlg.getByRole('button', { name: 'Catat & Kirim ke Kasir', exact: true })
    assert.equal(await submit.isDisabled(), true, 'di atas batas tidak bisa dikirim')
    await dlg.getByPlaceholder('Harga satuan', { exact: true }).fill('350000')
    await dlg.getByPlaceholder('Nama staf yang berbelanja', { exact: true }).fill('Pio')
    assert.equal(await submit.isDisabled(), false)
    await submit.click()
    await page.getByText('Menunggu kasir menyerahkan kas', { exact: true }).waitFor()
    assert.equal(created.length, 1)
    assert.deepEqual(created[0], {
      work_unit_id: 'u1', request_type: 'barang', category: 'bangunan', reason: 'Atap dapur bocor, air menetes ke kompor',
      shop_name: '', recipient_name: 'Pio', notes: '', items: [{ name: 'Seng + paku atap', qty: 1, unit: 'pcs', price: 350000 }],
    })
    // Setelah dicatat, detailnya terbuka: tutup lewat tombol Tutup.
    await page.getByRole('dialog', { name: 'Detail Pengajuan Barang', exact: true }).getByRole('button', { name: 'Tutup', exact: true }).first().click()

    // Detail darurat berstruk: Sahkan mengirim versi dokumen.
    await table.locator('tr', { hasText: 'PR-DRT-1' }).getByRole('button').first().click()
    const detail = page.getByRole('dialog', { name: 'Detail Pengajuan Barang', exact: true })
    await detail.getByText('Kas Rp', { exact: false }).first().waitFor()
    await detail.getByRole('button', { name: 'Sahkan', exact: true }).click()
    await detail.getByText('Disahkan oleh', { exact: false }).waitFor()
    assert.deepEqual(decisions, [{ decision: 'ratify', notes: '', version: 'v-e1' }])
    assert.deepEqual(errors, [])
    console.log('procurement emergency UI: OK')
  } finally {
    await browser.close()
  }
})().catch(error => { console.error(error); process.exit(1) })
