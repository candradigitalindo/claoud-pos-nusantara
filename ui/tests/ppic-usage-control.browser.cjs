// Every API request is intercepted. This suite never reads or writes live records.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright')
const assert = require('node:assert/strict')
const base = process.env.PPIC_USAGE_TEST_URL || 'http://127.0.0.1:4193'
const endpoint = '/api/v1/admin/ppic/usage-control'

;(async () => {
  const browser = await chromium.launch({ args: ['--no-sandbox'] })
  try {
    const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } })
    page.setDefaultTimeout(12000)
    const errors = [], requests = [], writes = [], policies = new Map(), history = new Map()
    let failReport = false
    let serverPermissions = []
    page.on('pageerror', error => errors.push(error.message))
    page.on('console', message => { if (message.text().includes('[Vue warn]')) errors.push(message.text()) })
    await page.addInitScript(() => {
      if (!localStorage.getItem('cloud_pos_token')) {
        localStorage.setItem('cloud_pos_token', 'mock-ppic')
        localStorage.setItem('cloud_pos_admin', JSON.stringify({ id: 'ppic-a', name: 'PPIC Outlet A', role: 'ppic' }))
        localStorage.setItem('cloud_pos_permissions', JSON.stringify(['ppic.control.view', 'ppic.control.update', 'ppic.control.export']))
        localStorage.setItem('cloud_pos_scope', 'specific')
        localStorage.setItem('cloud_pos_scope_outlet_ids', JSON.stringify(['outlet-a']))
      }
    })
    const warehouse = { id: 'warehouse-a', name: 'Gudang Outlet A', type: 'outlet', outlet_id: 'outlet-a', outlet_name: 'Outlet A' }
    const baseRow = {
      item_id: '', item_code: '', item_name: '', category: 'Bahan baku', base_unit: 'kg',
      warehouse_id: warehouse.id, warehouse_name: warehouse.name, warehouse_type: 'outlet', outlet_name: 'Outlet A',
      stock_qty: 50, sale_qty: 100, production_qty: 0, production_reference_qty: 0, breakdown_qty: 0,
      other_usage_qty: 0, stocktake_shortage_qty: 0, stocktake_surplus_qty: 0, adjustment_shortage_qty: 0,
      transfer_out_qty: 0, quality_hold_qty: 0, reference_qty: 100, evaluated_usage_qty: 100, excess_usage_qty: 0,
      waste_qty: 0, void_waste_qty: 0, undocumented_waste_qty: 0, waste_value: 0, shortage_value: 0,
      usage_limit_qty: null, waste_limit_qty: null, waste_pct: 0, usage_variance_pct: 0,
      stocktake_at: '', stocktake_number: '', has_stocktake: false, policy: null, status: 'unset', flags: [],
    }
    const policy = (itemID, usage = 5, waste = 2) => ({
      item_id: itemID, warehouse_id: warehouse.id, usage_tolerance_pct: usage, waste_tolerance_pct: waste,
      version: 1, reason: 'Standar hasil pemeriksaan PPIC', updated_by: 'PPIC Awal', updated_at: '2026-10-20T03:00:00Z',
    })
    const row = (itemID, itemName, values = {}) => ({ ...baseRow, item_id: itemID, item_name: itemName, item_code: itemID.toUpperCase(), ...values })
    const rows = [
      row('bawang', 'Bawang belum berbatas'),
      row('tepung', 'Tepung tanpa dasar', { sale_qty: 0, reference_qty: 0, evaluated_usage_qty: 0, waste_qty: 2, waste_value: 24000, waste_pct: null, usage_variance_pct: null, policy: policy('tepung'), status: 'no_basis', flags: ['no_reference'] }),
      row('beras', 'Beras perlu opname', { policy: policy('beras'), usage_limit_qty: 105, waste_limit_qty: 2, status: 'needs_count' }),
      row('minyak', 'Minyak melewati batas', { other_usage_qty: 10, stocktake_shortage_qty: 2, evaluated_usage_qty: 112, excess_usage_qty: 12, waste_qty: 6, void_waste_qty: 2, undocumented_waste_qty: 3, waste_value: 90000, shortage_value: 30000, waste_pct: 6, usage_variance_pct: 12, transfer_out_qty: 40, quality_hold_qty: 9, usage_limit_qty: 105, waste_limit_qty: 2, policy: policy('minyak'), stocktake_at: '2026-10-25T07:00:00Z', stocktake_number: 'OPN-UJI-001', has_stocktake: true, status: 'over_limit', flags: ['waste_document_missing', 'void_waste_included', 'usage_over_limit', 'waste_over_limit', 'stocktake_shortage'] }),
      row('gula', 'Gula sudah diperiksa', { policy: policy('gula'), usage_limit_qty: 105, waste_limit_qty: 2, stocktake_at: '2026-10-25T07:00:00Z', stocktake_number: 'OPN-UJI-002', has_stocktake: true, status: 'within_limit' }),
      row('garam', 'Garam tanpa aktivitas', { sale_qty: 0, reference_qty: 0, evaluated_usage_qty: 0, waste_pct: null, usage_variance_pct: null, policy: policy('garam'), status: 'no_activity' }),
      row('telur', 'Telur data tertunda', { policy: policy('telur'), usage_limit_qty: 105, waste_limit_qty: 2, status: 'data_incomplete', flags: ['pending_deductions'] }),
      ...Array.from({ length: 65 }, (_, i) => row(`minyak-${i + 1}`, `Minyak tambahan ${String(i + 1).padStart(2, '0')}`, { policy: policy(`minyak-${i + 1}`), evaluated_usage_qty: 110, other_usage_qty: 10, excess_usage_qty: 10, usage_variance_pct: 10, usage_limit_qty: 105, waste_limit_qty: 2, status: 'over_limit' })),
    ]
    for (const entry of rows) if (entry.policy) {
      policies.set(entry.item_id, entry.policy)
      history.set(entry.item_id, [entry.policy])
    }
    const currentRows = () => rows.map(entry => {
      if (entry.item_id !== 'bawang' || !policies.has('bawang')) return entry
      const saved = policies.get('bawang')
      return { ...entry, policy: saved, status: 'needs_count', usage_limit_qty: entry.reference_qty * (1 + saved.usage_tolerance_pct / 100), waste_limit_qty: entry.reference_qty * saved.waste_tolerance_pct / 100 }
    })
    const reply = (route, data) => route.fulfill({ json: { success: true, data } })
    await page.route('**/api/v1/**', async route => {
      const request = route.request(), url = new URL(request.url()), path = url.pathname, query = Object.fromEntries(url.searchParams)
      requests.push({ path, method: request.method(), query })
      if (path === '/api/v1/admin/me/permissions' && request.method() === 'GET') {
        return reply(route, { permissions: serverPermissions, scope_type: 'specific', scope_outlet_ids: ['outlet-a'] })
      }
      if ([endpoint + '/lookups', '/api/v1/admin/ppic/lookups'].includes(path) && request.method() === 'GET') {
        return reply(route, { warehouses: [warehouse], items: rows.map(entry => ({ id: entry.item_id, code: entry.item_code, name: entry.item_name, category: entry.category, base_unit: entry.base_unit })) })
      }
      if (path === '/api/v1/admin/ppic/opnames' && request.method() === 'GET') return reply(route, { data: [], total: 0, total_pages: 1 })
      if (path === '/api/v1/admin/ppic/expiry' && request.method() === 'GET') {
        return reply(route, { data: [{ batch_id: 'batch-a', item_id: 'tepung', item_name: 'Tepung kedaluwarsa', item_code: 'TEPUNG', category: 'Bahan baku', warehouse_id: warehouse.id, warehouse_name: warehouse.name, base_unit: 'kg', qty_base: 2, value: 24000, expiry_date: '2026-10-20', days_left: -7, bucket: 'expired', ref_type: 'goods_receipt', ack_at: '', ack_note: '' }], total: 1, no_expiry_count: 0, summary: [{ bucket: 'expired', count: 1, value: 24000 }] })
      }
      if (path === endpoint + '/policy/history' && request.method() === 'GET') {
        assert.equal(query.warehouse_id, warehouse.id)
        return reply(route, history.get(query.item_id) || [])
      }
      if (path === endpoint + '/policy' && request.method() === 'PUT') {
        const input = request.postDataJSON()
        writes.push(input)
        assert.equal(input.warehouse_id, warehouse.id)
        assert.equal(typeof input.usage_tolerance_pct, 'number')
        assert.equal(typeof input.waste_tolerance_pct, 'number')
        assert.equal(typeof input.version, 'number')
        assert(input.reason.trim())
        const existing = policies.get(input.item_id)
        if (input.version !== (existing?.version || 0)) return route.fulfill({ status: 409, json: { error: 'Batas telah berubah; muat ulang sebelum menyimpan.' } })
        const saved = { ...input, version: input.version + 1, updated_by: 'PPIC Outlet A', updated_at: '2026-10-27T03:00:00Z' }
        policies.set(input.item_id, saved)
        history.set(input.item_id, [saved, ...(history.get(input.item_id) || [])])
        return reply(route, saved)
      }
      if (path === endpoint + '/export' && request.method() === 'GET') return route.fulfill({ contentType: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet', body: 'mock-ppic-export' })
      if (path === endpoint && request.method() === 'GET') {
        if (failReport) return route.fulfill({ status: 500, json: { error: 'Laporan kontrol gagal dimuat' } })
        const all = currentRows()
        const filtered = all.filter(entry => (!query.warehouse_id || entry.warehouse_id === query.warehouse_id) && (!query.status || entry.status === query.status) && (!query.search || entry.item_name.toLowerCase().includes(query.search.toLowerCase())))
        const limit = Number(query.limit || 25), offset = (Number(query.page || 1) - 1) * limit
        return reply(route, {
          date_from: query.date_from, date_to: query.date_to, rows: filtered.slice(offset, offset + limit), total_rows: filtered.length,
          summary: { items_count: all.length, over_limit_count: all.filter(entry => entry.status === 'over_limit').length, unset_count: policies.has('bawang') ? 0 : 1, needs_count: policies.has('bawang') ? 2 : 1, no_basis_count: 1, waste_value: 114000, shortage_value: 30000, pending_deductions: 1 },
          notes: ['Pemakaian tercatat perlu dicocokkan dengan opname fisik.', 'Waste dari void dihitung sekali; titip pesanan belum memotong stok.'],
        })
      }
      errors.push(`Unexpected API: ${request.method()} ${path}`)
      return route.abort()
    })

    await page.goto(base + '/tests/fixtures/ppic-usage-control.html')
    await page.getByRole('heading', { name: 'Kontrol Pemakaian Bahan', exact: true }).waitFor()
    const tableRow = name => page.locator('tbody > tr').filter({ has: page.getByText(name, { exact: true }) }).first()
    const dialog = page.getByRole('dialog', { name: 'Atur Batas Pemakaian', exact: true })
    await tableRow('Bawang belum berbatas').waitFor()
    const requestAfter = async (action, predicate = () => true) => {
      const response = page.waitForResponse(response => new URL(response.url()).pathname === endpoint && predicate(new URL(response.url()).searchParams))
      await action()
      return (await response).request()
    }
    const choose = async (scope, label, option) => {
      await scope.getByRole('combobox', { name: label, exact: true }).click()
      await page.getByRole('option', { name: option, exact: true }).click()
    }

    // Page assertions follow the public controls rather than component internals.
    assert.match(await tableRow('Bawang belum berbatas').innerText(), /Belum ditetapkan/)
    assert.doesNotMatch(await tableRow('Bawang belum berbatas').innerText(), /Dalam batas|Sesuai batas|Aman/i)
    assert.match(await tableRow('Tepung tanpa dasar').innerText(), /—/)
    assert.doesNotMatch(await tableRow('Tepung tanpa dasar').innerText(), /NaN|Infinity|100%|Dalam batas|Sesuai batas|Aman/i)
    assert.equal(requests.filter(entry => !entry.path.startsWith(endpoint)).length, 0)
    assert.match(await tableRow('Beras perlu opname').innerText(), /Perlu cek fisik/)
    assert.match(await tableRow('Minyak melewati batas').innerText(), /Melebihi batas/)
    assert.match(await tableRow('Gula sudah diperiksa').innerText(), /Dalam batas tercatat/)
    assert.match(await tableRow('Garam tanpa aktivitas').innerText(), /Tidak ada pemakaian/)
    assert.match(await tableRow('Telur data tertunda').innerText(), /Data perlu dilengkapi/)
    assert.equal(await tableRow('Bawang belum berbatas').locator('.status-badge.success').count(), 0)
    assert.equal(await tableRow('Tepung tanpa dasar').locator('.status-badge.success').count(), 0)
    console.log('PASS unset and zero-baseline rows are not presented as safe; only scoped PPIC APIs requested')

    // Explicit zero is different from an unset tolerance; blank fields cannot become zero.
    await tableRow('Bawang belum berbatas').getByRole('button', { name: 'Atur Batas', exact: true }).click()
    const usageInput = dialog.getByLabel('Toleransi pemakaian (%)', { exact: true })
    const wasteInput = dialog.getByLabel('Toleransi waste (%)', { exact: true })
    const reasonInput = dialog.getByLabel('Alasan penetapan / perubahan', { exact: true })
    const save = dialog.getByRole('button', { name: 'Simpan Batas', exact: true })
    assert.equal(await usageInput.inputValue(), '')
    assert.equal(await wasteInput.inputValue(), '')
    await save.click()
    assert.equal(writes.length, 0)
    await usageInput.fill('0')
    await reasonInput.fill('Uji awal tanpa toleransi')
    await save.click()
    assert.equal(writes.length, 0, 'An empty waste tolerance must not silently save as zero')
    await wasteInput.fill('0')
    await reasonInput.fill('')
    await save.click()
    assert.equal(writes.length, 0, 'Policy changes require a reason')
    await reasonInput.fill('Uji awal tanpa toleransi')
    await requestAfter(() => save.click())
    await dialog.waitFor({ state: 'hidden' })
    await tableRow('Bawang belum berbatas').getByRole('button', { name: 'Ubah Batas', exact: true }).waitFor()
    assert.deepEqual(writes[0], { warehouse_id: warehouse.id, item_id: 'bawang', usage_tolerance_pct: 0, waste_tolerance_pct: 0, version: 0, reason: 'Uji awal tanpa toleransi' })
    await tableRow('Bawang belum berbatas').getByRole('button', { name: 'Ubah Batas', exact: true }).click()
    assert.equal(await usageInput.inputValue(), '0')
    assert.equal(await wasteInput.inputValue(), '0')
    assert.equal(await reasonInput.inputValue(), '', 'A new revision must supply its own reason')
    await dialog.getByText('Mengubah revisi 1.', { exact: false }).waitFor()
    await usageInput.fill('1.5')
    await reasonInput.fill('Evaluasi porsi setelah opname')
    await requestAfter(() => save.click())
    await dialog.waitFor({ state: 'hidden' })
    assert.equal(writes[1].version, 1)
    assert.equal(writes[1].usage_tolerance_pct, 1.5)
    assert.equal(writes[1].waste_tolerance_pct, 0)
    console.log('PASS missing limits stay blank; zero/decimal limits, mandatory reason and numeric revision payload survive save/reopen')

    // Revision history stays readable and preserves both reasons and actors.
    await tableRow('Bawang belum berbatas').getByRole('button', { name: 'Rincian', exact: true }).click()
    let detail = page.getByRole('dialog', { name: 'Rincian Kontrol — Bawang belum berbatas', exact: true })
    await detail.getByRole('heading', { name: 'Riwayat Batas PPIC', exact: true }).waitFor()
    await detail.getByText('Uji awal tanpa toleransi', { exact: true }).waitFor()
    assert(await detail.getByText('Evaluasi porsi setelah opname', { exact: true }).isVisible())
    assert.match(await detail.innerText(), /Revisi 2 · Pakai \+1,5% · Waste 0%/)
    assert.match(await detail.innerText(), /Revisi 1 · Pakai \+0% · Waste 0%/)
    assert.match(await detail.innerText(), /PPIC Outlet A/)
    await detail.getByRole('button', { name: 'Tutup', exact: true }).last().click()
    await tableRow('Minyak melewati batas').getByRole('button', { name: 'Rincian', exact: true }).click()
    detail = page.getByRole('dialog', { name: 'Rincian Kontrol — Minyak melewati batas', exact: true })
    assert.equal((await detail.locator('dt').filter({ hasText: /^Waste total$/ }).locator('+ dd').innerText()).trim(), '6')
    assert.equal((await detail.locator('dt').filter({ hasText: /^Termasuk waste dari void$/ }).locator('+ dd').innerText()).trim(), '2')
    assert.match(await detail.innerText(), /Transfer keluar 40 dan karantina 9/)
    assert.match(await detail.innerText(), /OPN-UJI-001/)
    assert.match(await detail.innerText(), /Waste total sudah mencakup 3 kg pengeluaran stok tanpa dokumen waste yang sesuai/)
    assert(await detail.getByText('Ada pengeluaran waste tanpa dokumen waste yang lengkap.', { exact: true }).isVisible())
    assert(await detail.getByText('Waste dari void sudah termasuk dalam total waste.', { exact: true }).isVisible())
    assert(await detail.getByText('Pemakaian dan koreksi negatif melewati batas PPIC.', { exact: true }).isVisible())
    assert(await detail.getByText('Waste melewati batas PPIC.', { exact: true }).isVisible())
    assert.doesNotMatch(await detail.innerText(), /waste_document_missing|void_waste_included|usage_over_limit|waste_over_limit|stocktake_shortage/)
    await detail.getByRole('button', { name: 'Tutup', exact: true }).last().click()
    console.log('PASS policy history, translated flags, waste/void/orphan waste, transfer/quarantine explanation and physical count evidence')

    // Global editor obtains the current pair revision via its scoped lookup API.
    await page.locator('header').getByRole('button', { name: 'Atur Batas', exact: true }).click()
    await dialog.getByRole('combobox', { name: 'Gudang kebijakan', exact: true }).click()
    const warehouseChoices = await page.getByRole('option').allTextContents()
    assert(warehouseChoices.some(name => name.includes('Gudang Outlet A')))
    assert(!warehouseChoices.some(name => /Pusat|Outlet B/.test(name)))
    await page.getByRole('option', { name: 'Gudang Outlet A · Outlet A', exact: true }).click()
    await choose(dialog, 'Bahan', 'BAWANG · Bawang belum berbatas (kg)')
    await dialog.getByText('Mengubah revisi 2.', { exact: false }).waitFor()
    assert.equal(await usageInput.inputValue(), '1.5')
    assert.equal(await wasteInput.inputValue(), '0')
    await dialog.getByRole('button', { name: 'Batal', exact: true }).click()
    assert.equal(requests.filter(entry => entry.path === endpoint + '/lookups').length, 1)
    console.log('PASS scoped warehouse choices and current policy loading for the selected material/warehouse')

    await choose(page, 'Gudang', 'Gudang Outlet A · Outlet A')
    await choose(page, 'Status kontrol', 'Melebihi batas')
    await page.getByLabel('Dari tanggal', { exact: true }).fill('2026-10-01')
    await page.getByLabel('Sampai tanggal', { exact: true }).fill('2026-10-27')
    await page.getByLabel('Cari bahan', { exact: true }).fill('Minyak')
    const filteredRequest = await requestAfter(() => page.getByRole('button', { name: 'Tampilkan', exact: true }).click())
    const applied = Object.fromEntries(new URL(filteredRequest.url()).searchParams)
    assert.deepEqual(applied, { date_from: '2026-10-01', date_to: '2026-10-27', warehouse_id: warehouse.id, search: 'Minyak', status: 'over_limit', page: '1', limit: '20' })
    await tableRow('Minyak melewati batas').waitFor()
    assert.equal(await tableRow('Bawang belum berbatas').count(), 0)
    // Editing a filter does not silently change the active report, pagination or export.
    await page.getByLabel('Cari bahan', { exact: true }).fill('Belum diterapkan')
    const nextRequest = await requestAfter(() => page.getByRole('button', { name: 'Next ›', exact: true }).click(), params => params.get('page') === '2')
    assert.deepEqual(Object.fromEntries(new URL(nextRequest.url()).searchParams), { ...applied, page: '2' })
    await tableRow('Minyak tambahan 20').waitFor()
    const downloaded = page.waitForEvent('download')
    await page.getByRole('button', { name: 'Export Excel', exact: true }).click()
    assert.equal((await downloaded).suggestedFilename(), 'Kontrol-Bahan-2026-10-01-2026-10-27.xlsx')
    const exportRequest = requests.filter(entry => entry.path === endpoint + '/export').at(-1)
    const { page: ignoredPage, limit: ignoredLimit, ...exportFilters } = applied
    assert.deepEqual(exportRequest.query, exportFilters)
    await page.getByLabel('Cari bahan', { exact: true }).fill('Minyak')
    await requestAfter(() => page.getByRole('button', { name: 'Tampilkan', exact: true }).click(), params => params.get('page') === '1')
    await tableRow('Minyak melewati batas').waitFor()
    console.log('PASS date/warehouse/search/status filters, paging reset, applied filters preserved and full-report Excel export')

    if (process.env.PPIC_USAGE_SCREENSHOT) await page.screenshot({ path: process.env.PPIC_USAGE_SCREENSHOT, fullPage: true })
    await page.setViewportSize({ width: 390, height: 844 })
    assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), 'Mobile page should fit the viewport')
    const mobileCard = page.locator('li').filter({ has: page.getByText('Minyak melewati batas', { exact: true }) }).first()
    await mobileCard.getByRole('button', { name: 'Ubah Batas', exact: true }).click()
    const bounds = await dialog.boundingBox()
    assert(bounds.x >= 0 && bounds.x + bounds.width <= 390)
    assert(await save.isVisible(), 'The editor action remains visible on mobile')
    await dialog.getByRole('button', { name: 'Batal', exact: true }).click()
    await mobileCard.getByRole('button', { name: 'Rincian', exact: true }).click()
    detail = page.getByRole('dialog', { name: 'Rincian Kontrol — Minyak melewati batas', exact: true })
    assert(await detail.getByRole('button', { name: 'Tutup', exact: true }).last().isVisible())
    assert(await detail.evaluate(el => el.scrollWidth <= el.clientWidth), 'Mobile detail must not overflow horizontally')
    await detail.getByRole('button', { name: 'Tutup', exact: true }).last().click()
    console.log('PASS mobile report cards and editor/detail modal sizing with visible actions')

    await page.evaluate(() => localStorage.setItem('cloud_pos_permissions', JSON.stringify(['ppic.control.view'])))
    await page.setViewportSize({ width: 1440, height: 1000 })
    await page.reload()
    await tableRow('Bawang belum berbatas').waitFor()
    assert.equal(await page.getByRole('button', { name: 'Atur Batas', exact: true }).count(), 0)
    assert.equal(await page.getByRole('button', { name: 'Ubah Batas', exact: true }).count(), 0)
    assert.equal(await page.getByRole('button', { name: 'Export Excel', exact: true }).count(), 0)
    await tableRow('Bawang belum berbatas').getByRole('button', { name: 'Rincian', exact: true }).click()
    detail = page.getByRole('dialog', { name: 'Rincian Kontrol — Bawang belum berbatas', exact: true })
    await detail.getByText('Evaluasi porsi setelah opname', { exact: true }).waitFor()
    assert.equal(await detail.getByRole('button', { name: 'Ubah Batas', exact: true }).count(), 0)
    await detail.getByRole('button', { name: 'Tutup', exact: true }).last().click()
    failReport = true
    await requestAfter(() => page.getByRole('button', { name: 'Tampilkan', exact: true }).click())
    await page.getByText('Laporan kontrol gagal dimuat', { exact: true }).waitFor()
    assert.equal(await tableRow('Bawang belum berbatas').count(), 0)
    assert.equal(await page.getByText('Waste tercatat', { exact: true }).count(), 0)
    assert.equal(writes.length, 2)
    failReport = false
    await requestAfter(() => page.getByRole('button', { name: 'Tampilkan', exact: true }).click())
    await tableRow('Bawang belum berbatas').waitFor()
    assert.equal(await page.getByText('Laporan kontrol gagal dimuat', { exact: true }).count(), 0)
    console.log('PASS read-only policy/history access, failed report clears stale rows/totals, and retry recovers')

    // Real router/sidebar tests follow the role permissions returned by the server.
    const openShell = async (path, permissions, storedPermissions = permissions, redirect = '/ppic/dashboard') => {
      serverPermissions = permissions
      await page.evaluate(({ storedPermissions, redirect }) => {
        localStorage.setItem('cloud_pos_permissions', JSON.stringify(storedPermissions))
        localStorage.setItem('cloud_pos_redirect_to', redirect)
      }, { storedPermissions, redirect })
      await page.goto(base + '/tests/fixtures/ppic-usage-control.html?shell=1&route=' + encodeURIComponent(path))
    }
    const retained = ['ppic.control.view', 'ppic.opname.view', 'ppic.opname.create', 'ppic.expiry.view']
    await openShell('/ppic/usage-control', retained)
    await tableRow('Bawang belum berbatas').waitFor()
    const ppicGroup = page.locator('.nav-group').filter({ has: page.getByRole('button', { name: 'PPIC', exact: true }) })
    const links = await ppicGroup.getByRole('link').allTextContents()
    assert.deepEqual(links.map(name => name.trim()).sort(), ['Kontrol Pemakaian Bahan', 'Monitor Kedaluwarsa', 'Stock Opname'].sort())
    await ppicGroup.getByRole('link', { name: 'Stock Opname', exact: true }).click()
    await page.getByRole('heading', { name: 'Stock Opname (Cycle Count)', exact: true }).waitFor()
    await page.getByText('Belum ada sesi opname.', { exact: true }).waitFor()
    await page.getByRole('button', { name: '+ Sesi Opname Baru', exact: true }).click()
    const opnameDialog = page.getByRole('dialog', { name: 'Sesi Opname Baru', exact: true })
    await opnameDialog.getByRole('combobox').first().click()
    const opnameWarehouses = await page.getByRole('option').allTextContents()
    assert(opnameWarehouses.some(name => name.includes(warehouse.name)))
    assert(!opnameWarehouses.some(name => /Pusat|Outlet B/.test(name)))
    await page.getByRole('option', { name: /Gudang Outlet A/ }).click()
    await opnameDialog.getByRole('combobox').nth(1).click()
    await page.getByRole('option', { name: 'Bahan baku', exact: true }).click()
    await opnameDialog.getByRole('button', { name: 'Batal', exact: true }).click()
    await ppicGroup.getByRole('link', { name: 'Monitor Kedaluwarsa', exact: true }).click()
    await page.getByRole('heading', { name: 'Monitor Kedaluwarsa (FEFO)', exact: true }).waitFor()
    await page.getByText('Tepung kedaluwarsa', { exact: true }).waitFor()
    await page.getByRole('combobox').first().click()
    await page.getByRole('option', { name: /Gudang Outlet A/ }).click()
    assert.equal(await page.getByRole('button', { name: 'Tandai Ditindak', exact: true }).count(), 0)
    assert.equal(await page.getByRole('link', { name: 'Lihat Waste', exact: true }).count(), 0)
    assert.equal(requests.filter(entry => /\/warehouses$|\/stock-item-categories$/.test(entry.path)).length, 0)
    assert(requests.filter(entry => entry.path === '/api/v1/admin/ppic/lookups').length >= 2)
    console.log('PASS exactly three PPIC navigation entries and scoped opname/expiry lookups without warehouse permissions')

    const retiredPaths = ['dashboard', 'planning-params', 'forecast', 'mrp', 'production-plans', 'work-orders', 'reports']
    for (const path of retiredPaths) {
      await openShell('/ppic/' + path, ['ppic.control.view'], ['ppic.control.view'], '/ppic/' + path)
      await page.waitForURL(url => url.pathname === '/ppic/usage-control')
      await tableRow('Bawang belum berbatas').waitFor()
      assert.equal(await page.evaluate(() => localStorage.getItem('cloud_pos_redirect_to')), '/ppic/usage-control')
    }
    // Refresh server permissions before choosing a replacement for a bookmarked route.
    await openShell('/ppic/work-orders/old-work-order', ['ppic.opname.view'], ['ppic.control.view'])
    await page.waitForURL(url => url.pathname === '/ppic/opname')
    await page.getByText('Belum ada sesi opname.', { exact: true }).waitFor()
    assert.equal(await page.getByRole('button', { name: '+ Sesi Opname Baru', exact: true }).count(), 0)
    await openShell('/ppic/reports?tab=hpp', ['ppic.expiry.view'])
    await page.waitForURL(url => url.pathname === '/ppic/expiry')
    await page.getByText('Tepung kedaluwarsa', { exact: true }).waitFor()
    await openShell('/ppic/dashboard', ['ppic.dashboard.view'], ['ppic.control.view'])
    await page.waitForURL(url => url.pathname === '/403')
    await page.getByRole('heading', { name: 'Akses ditolak', exact: true }).waitFor()
    assert.equal(await page.getByRole('button', { name: 'PPIC', exact: true }).count(), 0)
    assert.equal(requests.filter(entry => /\/ppic\/(dashboard|planning-params|forecasts|mrp|production-plans|work-orders|reports)(\/|$)/.test(entry.path)).length, 0)
    assert.equal(writes.length, 2)
    console.log('PASS retired bookmarks redirect after permission refresh to control/opname/expiry or 403; no retired API is called')
    assert.deepEqual(errors, [])
  } finally { await browser.close() }
})().catch(error => { console.error(error); process.exitCode = 1 })
