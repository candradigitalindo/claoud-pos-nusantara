// Audit Keuangan atas Pembelian Darurat (19 Nov 2026): halaman Pembayaran
// menyaring jenis (ikut ke Excel) dan menampilkan bukti struk & pengesahan
// tanpa tombol aksi; halaman Rekap Pembelian Darurat merangkum per outlet.
// All API requests are synthetic. No production records are accessed.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright')
const assert = require('node:assert/strict')
const base = process.env.PROCUREMENT_TEST_URL || 'http://127.0.0.1:4189'
const emergency = {
  id: 'pr-e', request_number: 'PR-DRT-9', request_type: 'jasa', requested_by: 'Manajer Uji', work_unit_name: 'Sekar Ayu',
  outlet_id: 'o1', outlet_name: 'Sekar Ayu', vendor_name: 'Teknisi Kulkas', status: 'paid', receipt_status: '', paid_amount: 250000,
  paid_at: '2026-11-19T05:10:00Z', total_final: 250000, total_hps: 250000, total_amount: 250000, version: 'v-e9', urgency: 'urgent',
  created_at: '2026-11-19T05:00:00Z', struks: [], pickups: [],
  items: [{ name: 'DARURAT: Servis kulkas', items: [{ name: 'Servis kulkas', qty: 1, unit: 'kali', hps_price: 250000, final_price: 250000 }] }],
  pay_to: { ready: false, problem: 'Pembelian Darurat dibayar tunai dari Kas Kasir Outlet lewat tablet kasir' },
  emergency: { category: 'peralatan_rusak', category_label: 'Peralatan operasional rusak/mati', reason: 'Kulkas tidak dingin',
    recipient: 'Leo', proof_url: '/uploads/struk.jpg', proof_total: 230000, proof_by: 'Manajer Uji', ratification: 'rejected',
    ratified_by: 'Owner Uji', ratified_at: '2026-11-19T06:00:00Z', ratification_notes: 'Bisa menunggu teknisi langganan' },
}
const bucket = (count, amount) => ({ count, amount })
const totals = (name) => ({ outlet_id: name === 'Semua outlet' ? '' : 'o1', outlet_name: name, recorded: bucket(3, 700000),
  waiting_cash: bucket(1, 300000), waiting_proof: bucket(0, 0), waiting_ratification: bucket(0, 0), ratified: bucket(1, 150000),
  rejected: bucket(1, 250000), cancelled: bucket(1, 90000), proof_difference: 20000, other_cash_out: bucket(2, 435000) })
const recap = {
  month: '2026-11', month_label: 'November 2026', settings: { max_amount: 500000, monthly_cap: 0 },
  totals: totals('Semua outlet'), outlets: [totals('Sekar Ayu')],
  rows: [
    { id: 'pr-a', request_number: 'PR-DRT-7', created_at: '2026-11-18T03:00:00Z', outlet_name: 'Sekar Ayu', items: 'Steam valve ×1',
      category_label: 'Peralatan operasional rusak/mati', reason: 'Bar berhenti', recipient: 'Adri', amount: 300000, stage: 'waiting_cash' },
    { id: 'pr-b', request_number: 'PR-DRT-8', created_at: '2026-11-18T04:00:00Z', outlet_name: 'Sekar Ayu', items: 'Regulator gas ×1',
      category_label: 'Keselamatan & keamanan', reason: 'Regulator bocor', recipient: 'Pio', amount: 150000, stage: 'ratified',
      cashier: 'Kasir Sore', authorized_by: 'Manajer Sekar', proof_total: 150000, difference: 0, ratified_by: 'Owner Uji' },
    { id: 'pr-e', request_number: 'PR-DRT-9', created_at: '2026-11-19T05:00:00Z', outlet_name: 'Sekar Ayu', items: 'Servis kulkas ×1',
      category_label: 'Peralatan operasional rusak/mati', reason: 'Kulkas tidak dingin', recipient: 'Leo', amount: 250000, stage: 'rejected',
      cashier: 'Kasir Sore', authorized_by: 'Manajer Sekar', proof_total: 230000, difference: 20000, ratified_by: 'Owner Uji' },
  ],
}
;(async () => {
  const browser = await chromium.launch({ args: ['--no-sandbox'] })
  try {
    const page = await browser.newPage({ viewport: { width: 1366, height: 1000 }, acceptDownloads: true })
    page.setDefaultTimeout(12000)
    const errors = [], lists = [], exports = [], recapCalls = [], recapExports = []
    page.on('pageerror', e => errors.push(e.message))
    page.on('console', m => { if (m.text().includes('[Vue warn]')) errors.push(m.text()) })
    await page.route('**/api/v1/**', async route => {
      const request = route.request(), url = new URL(request.url()), path = url.pathname
      const reply = data => route.fulfill({ json: { success: true, data } })
      const xlsx = () => route.fulfill({ body: 'synthetic-xlsx', contentType: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' })
      if (path.endsWith('/payment-stats')) return reply({ waiting: 0, total_waiting: 0, accounts_payable: { total_amount: 0 } })
      if (path.endsWith('/bank-accounts')) return reply([])
      if (path.endsWith('/projects')) return reply([])
      if (path.endsWith('/my-outlets')) return reply([{ id: 'o1', name: 'Sekar Ayu' }])
      if (path.endsWith('/procurement-payments/export')) { exports.push(Object.fromEntries(url.searchParams)); return xlsx() }
      if (path.endsWith('/procurement/emergency-recap/export')) { recapExports.push(Object.fromEntries(url.searchParams)); return xlsx() }
      if (path.endsWith('/procurement/emergency-recap')) { recapCalls.push(Object.fromEntries(url.searchParams)); return reply(recap) }
      if (path.endsWith('/purchase-requests/pr-e/payment-histories')) return reply([])
      if (path.endsWith('/purchase-requests/pr-e')) return reply(emergency)
      if (path.endsWith('/purchase-requests')) { lists.push(Object.fromEntries(url.searchParams)); return reply({ requests: [emergency], total_pages: 1 }) }
      errors.push(`Unexpected API ${request.method()} ${path}`); return route.abort()
    })

    // ── Halaman Pembayaran ──
    await page.goto(base + '/tests/fixtures/payment-marketplace.html')
    await page.getByText('Pengesahan Ditolak', { exact: true }).first().waitFor()
    await page.getByRole('combobox', { name: 'Filter jenis pembelian' }).click()
    await page.getByRole('option', { name: 'Darurat — pengesahan ditolak (temuan)', exact: true }).click()
    await page.getByRole('button', { name: 'Tampilkan', exact: true }).click()
    await page.waitForFunction(() => true)
    await new Promise(r => setTimeout(r, 400))
    assert.equal(lists.at(-1).ratification, 'rejected', 'saringan jenis ikut ke daftar')
    const download = page.waitForEvent('download')
    await page.getByRole('button', { name: 'Download Excel', exact: true }).click()
    await download
    assert.equal(exports.at(-1).ratification, 'rejected', 'saringan jenis ikut ke Excel')
    await page.locator('tr', { hasText: 'PR-DRT-9' }).getByRole('button').first().click()
    const detail = page.getByRole('dialog').first()
    await detail.getByText('Struk lebih kecil', { exact: false }).waitFor()
    await detail.getByText('Temuan: pembayaran tidak dibatalkan', { exact: false }).waitFor()
    assert.equal(await detail.getByRole('button', { name: 'Sahkan', exact: true }).count(), 0, 'Keuangan hanya membaca')
    assert.equal(await detail.getByRole('button', { name: 'Ganti bukti', exact: true }).count(), 0)

    // ── Rekap Pembelian Darurat ──
    await page.goto(base + '/tests/fixtures/emergency-recap.html')
    await page.getByText('Per Outlet — November 2026', { exact: true }).waitFor()
    assert.match(recapCalls[0].month, /^\d{4}-\d{2}$/)
    await page.getByText('Ditolak — temuan', { exact: true }).waitFor()
    assert.equal(await page.getByText('Rincian (3)', { exact: true }).count(), 1)
    await page.getByRole('button', { name: 'Ditolak (temuan)', exact: true }).click()
    await page.getByText('Rincian (1)', { exact: true }).waitFor()
    await page.locator('tr', { hasText: 'PR-DRT-9' }).getByRole('button').first().click()
    const modal = page.getByRole('dialog', { name: 'Pembelian Darurat PR-DRT-9', exact: true })
    await modal.getByText('Pengesahan ditolak oleh', { exact: false }).waitFor()
    assert.equal(await modal.getByRole('button', { name: 'Sahkan', exact: true }).count(), 0)
    await modal.getByRole('button', { name: 'Tutup', exact: true }).last().click()
    await page.locator('#recap-month').fill('2026-10')
    await page.getByRole('button', { name: 'Tampilkan', exact: true }).click()
    await new Promise(r => setTimeout(r, 400))
    assert.equal(recapCalls.at(-1).month, '2026-10')
    const dl = page.waitForEvent('download')
    await page.getByRole('button', { name: 'Download Excel', exact: true }).click()
    assert.equal((await dl).suggestedFilename(), 'Rekap-Pembelian-Darurat_2026-10.xlsx')
    assert.equal(recapExports.at(-1).month, '2026-10')
    assert.deepEqual(errors, [])
    console.log('finance emergency UI: OK')
  } finally {
    await browser.close()
  }
})().catch(error => { console.error(error); process.exit(1) })
