// Run against Vite/preview with RECIPE_AUTHORING_TEST_URL. Every API request is
// intercepted: this suite never needs a backend or writes business records.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright')
const assert = require('node:assert/strict')
const base = process.env.RECIPE_AUTHORING_TEST_URL || 'http://127.0.0.1:4192'

function fixtures() {
  const item = (id, code, name, base_unit = 'gram') => ({
    id, code, name, base_unit, dist_unit: 'kg', dist_unit_label: 'kg', dist_ratio: 1000,
    category: 'Bahan uji', is_active: true, min_stock: 0, total_stock: 1000,
    avg_cost: 10, warehouse_qty: 1000, warehouse_min_stock: 0, warehouse_avg_cost: 10, in_warehouse: true,
  })
  const items = [item('sfg', 'SFG', 'Saus Uji', 'ml'), item('sugar', 'GULA', 'Gula Uji'),
    item('water', 'AIR', 'Air Uji', 'ml'), item('chicken', 'AYAM', 'Ayam Uji'),
    item('thigh', 'PAHA', 'Paha Uji'), item('breast', 'DADA', 'Dada Uji')]
  const recipes = [
    { id: 'component-1', parent_item_id: 'sfg', child_item_id: 'sugar', child_name: 'Gula Uji', child_code: 'GULA', qty_base: 0.5, unit: 'gram', notes: 'Larutkan gula lebih dahulu', visibility: 'public' },
    { id: 'component-2', parent_item_id: 'sfg', child_item_id: 'water', child_name: 'Air Uji', child_code: 'AIR', qty_base: 0.25, unit: 'ml', notes: 'Gunakan air matang', visibility: 'public' },
  ]
  const output = (id, qty, weight) => ({ output_item_id: id, output_item_name: items.find(i => i.id === id).name, output_item_code: items.find(i => i.id === id).code, output_base_unit: 'gram', qty_per_input: qty, cost_weight: weight, notes: '' })
  const template = { input_item_id: 'chicken', patterns: [
    { id: 'pattern-existing-1', name: 'Per Bagian', notes: 'Pola pertama', is_active: true, outputs: [output('thigh', 350, 1), output('breast', 450, 2)] },
    { id: 'pattern-existing-2', name: 'Fillet', notes: 'Pola kedua dipertahankan', is_active: false, outputs: [output('breast', 700, 1)] },
  ] }
  return { items, recipes, template }
}

async function session(browser, permissions, scope = 'all') {
  const context = await browser.newContext({ viewport: { width: 1366, height: 900 }, serviceWorkers: 'block' })
  // Deliberately stale cache proves the router refreshes scope/permissions.
  await context.addInitScript(() => {
    localStorage.setItem('cloud_pos_token', 'intercepted-browser-test')
    localStorage.setItem('cloud_pos_admin', JSON.stringify({ id: 'test-admin', name: 'Penguji', username: 'test', role: 'staff' }))
    localStorage.setItem('cloud_pos_permissions', JSON.stringify(['recipes.update', 'stockledger.adjust']))
    localStorage.setItem('cloud_pos_scope', 'all')
    localStorage.setItem('cloud_pos_scope_outlet_ids', 'null')
  })
  const state = { ...fixtures(), reads: [], writes: [], errors: [], failRecipe: false, failPattern: false }
  await context.route('**/*', async route => {
    const req = route.request(), url = new URL(req.url()), path = url.pathname
    if (!path.startsWith('/api/')) {
      if (['fetch', 'xhr'].includes(req.resourceType())) {
        state.errors.push(`Unexpected request: ${req.method()} ${url.origin}${path}`)
        return route.abort()
      }
      return route.continue()
    }
    const reply = data => route.fulfill({ json: { success: true, data } })
    const paged = data => route.fulfill({ json: { success: true, data, total: data.length, page: 1, total_pages: 1 } })
    const fail = () => route.fulfill({ status: 503, json: { success: false, error: 'Simulasi gagal memuat formula' } })
    if (req.method() === 'GET') {
      state.reads.push({ path, query: Object.fromEntries(url.searchParams) })
      if (path === '/api/v1/timezone') return reply({ timezone: 'Asia/Jakarta' })
      if (path === '/api/v1/admin/me/permissions') return reply({ permissions, scope_type: scope, scope_outlet_ids: scope === 'specific' ? ['outlet-1'] : null })
      if (path === '/api/v1/admin/stock-items') return paged(state.items)
      if (path === '/api/v1/admin/stock-items/sfg/recipes') return state.failRecipe ? fail() : reply(state.recipes)
      if (path === '/api/v1/admin/stock-items/chicken/breakdown') return state.failPattern ? fail() : reply(state.template)
      if (path === '/api/v1/admin/warehouses') return paged([{ id: 'warehouse-1', code: 'WH-UJI', name: 'Gudang Uji', type: 'central', is_active: true }])
      if (path === '/api/v1/admin/stock-ledger') return paged([])
      if (path === '/api/v1/admin/recipe-masters') return reply([])
      if (path === '/api/v1/admin/outlets') return reply([{ id: 'outlet-1', name: 'Outlet Uji' }])
    } else {
      const body = req.postDataJSON()
      state.writes.push({ method: req.method(), path, body })
      if (req.method() === 'PUT' && path === '/api/v1/admin/stock-items/sfg/recipes') {
        state.recipes = body.items.map((i, n) => ({ id: `component-${n + 1}`, parent_item_id: 'sfg', child_item_id: i.item_id,
          child_name: state.items.find(item => item.id === i.item_id).name, qty_base: i.qty_base, unit: i.unit, notes: i.notes, visibility: body.visibility }))
        return reply({})
      }
      if (req.method() === 'PUT' && path === '/api/v1/admin/stock-items/chicken/breakdown') {
        state.template.patterns = body.patterns
        return reply(state.template)
      }
      if (req.method() === 'POST' && path === '/api/v1/admin/stock-items/sfg/produce') return reply({ id: 'production-test-1' })
    }
    state.errors.push(`Unexpected API: ${req.method()} ${path}`)
    return route.abort()
  })
  const page = await context.newPage()
  page.setDefaultTimeout(12000)
  page.on('pageerror', error => state.errors.push(error.message))
  page.on('console', message => { if (message.text().includes('[Vue warn]')) state.errors.push(message.text()) })
  return { page, state, close: async () => { assert.deepEqual(state.errors, []); await context.close() } }
}

async function openSfg(page, action = 'Atur Resep') {
  await page.goto(base + '/fnb/recipes?tab=sfg')
  await page.getByTestId('formula-item-sfg').getByRole('button', { name: action, exact: true }).click()
  const dialog = page.getByRole('dialog', { name: 'Resep Bahan Setengah Jadi', exact: true })
  await dialog.waitFor()
  return dialog
}
async function openPattern(page, action = 'Atur Pola') {
  await page.getByTestId('recipe-tab-breakdown').click()
  await page.getByTestId('formula-item-chicken').getByRole('button', { name: action, exact: true }).click()
  const dialog = page.getByRole('dialog', { name: 'Atur Pemotongan', exact: true })
  await dialog.waitFor()
  return dialog
}
async function cancel(dialog) {
  await dialog.getByRole('button', { name: 'Batal', exact: true }).click()
  await dialog.waitFor({ state: 'hidden' })
}
async function assertReadOnly(dialog) {
  const controls = dialog.locator('fieldset input, fieldset button, fieldset select, fieldset textarea')
  assert(await controls.count() > 0, 'read-only detail must still show existing values')
  for (const control of await controls.all()) assert(await control.isDisabled(), 'all formula controls must be disabled')
  assert.equal(await dialog.getByRole('button', { name: /Simpan|Tambah|Hapus/ }).count(), 0)
}

;(async () => {
  const browser = await chromium.launch({ args: ['--no-sandbox'] })
  try {
    // F&B editors require no stockitems permission and update the original API
    // resources; reopening must not append components or duplicate pattern IDs.
    const editor = await session(browser, ['recipes.update'])
    const { page, state } = editor
    let dialog = await openSfg(page)
    await dialog.getByLabel('Jumlah komponen').first().waitFor()
    assert.equal(await dialog.getByLabel('Jumlah komponen').count(), 2)
    assert.equal(await dialog.getByLabel('Jumlah komponen').first().inputValue(), '0.5')
    await dialog.getByLabel('Jumlah komponen').first().fill('0')
    await dialog.getByRole('button', { name: 'Simpan Resep', exact: true }).click()
    await page.getByText('Pilih bahan dan isi jumlah positif pada setiap komponen', { exact: true }).waitFor()
    assert.equal(state.writes.length, 0, 'invalid amount must not silently remove an existing component')
    assert(await dialog.isVisible())
    await dialog.getByLabel('Jumlah komponen').first().fill('0.75')
    await dialog.getByRole('button', { name: 'Simpan Resep', exact: true }).click()
    await dialog.waitFor({ state: 'hidden' })
    assert.equal(state.writes.length, 1)
    assert.equal(state.writes[0].method, 'PUT')
    assert.equal(state.writes[0].path, '/api/v1/admin/stock-items/sfg/recipes')
    assert.equal(state.writes[0].body.parent_item_id, 'sfg')
    assert.equal(state.writes[0].body.visibility, 'public')
    assert.deepEqual(state.writes[0].body.items.map(i => [i.item_id, i.qty_base]), [['sugar', 0.75], ['water', 0.25]])
    assert.deepEqual(state.writes[0].body.items.map(i => i.notes), ['Larutkan gula lebih dahulu', 'Gunakan air matang'])
    dialog = await openSfg(page)
    await dialog.getByLabel('Jumlah komponen').first().waitFor()
    assert.deepEqual(await dialog.getByLabel('Jumlah komponen').evaluateAll(inputs => inputs.map(i => i.value)), ['0.75', '0.25'])
    await cancel(dialog)
    console.log('PASS F&B recipes.update edits/reopens existing SFG using PUT, with no duplicate components')

    dialog = await openPattern(page)
    await dialog.getByLabel('Nama pola', { exact: true }).first().waitFor()
    assert.deepEqual(await dialog.getByLabel('Nama pola', { exact: true }).evaluateAll(inputs => inputs.map(i => i.value)), ['Per Bagian', 'Fillet'])
    await dialog.getByLabel('Nama pola', { exact: true }).first().fill('Per Bagian Revisi')
    await dialog.getByLabel('Hasil standar', { exact: true }).first().fill('375')
    await dialog.getByLabel('Bobot biaya', { exact: true }).first().fill('1.5')
    await dialog.getByRole('button', { name: 'Simpan Pola', exact: true }).click()
    await dialog.waitFor({ state: 'hidden' })
    assert.equal(state.writes.length, 2)
    const write = state.writes[1]
    assert.equal(write.method, 'PUT')
    assert.equal(write.path, '/api/v1/admin/stock-items/chicken/breakdown')
    assert.deepEqual(write.body.patterns.map(p => p.id), ['pattern-existing-1', 'pattern-existing-2'])
    assert.equal(write.body.patterns[0].name, 'Per Bagian Revisi')
    assert.deepEqual(write.body.patterns[0].outputs.map(o => [o.output_item_id, o.qty_per_input, o.cost_weight]), [['thigh', 375, 1.5], ['breast', 450, 2]])
    assert.equal(write.body.patterns[1].name, 'Fillet')
    assert.equal(write.body.patterns[1].is_active, false)
    assert.equal(write.body.patterns[1].notes, 'Pola kedua dipertahankan')
    dialog = await openPattern(page)
    await dialog.getByLabel('Nama pola', { exact: true }).first().waitFor()
    assert.deepEqual(await dialog.getByLabel('Nama pola', { exact: true }).evaluateAll(inputs => inputs.map(i => i.value)), ['Per Bagian Revisi', 'Fillet'])
    assert.equal(await dialog.getByLabel('Hasil standar', { exact: true }).first().inputValue(), '375')
    await cancel(dialog)
    assert(state.reads.filter(r => r.path.endsWith('/stock-items')).every(r => !r.query.warehouse_id), 'F&B uses the global catalog')
    console.log('PASS existing pattern IDs, alternate inactive pattern, quantities and cost weights survive PUT/reopen')

    state.failRecipe = true
    dialog = await openSfg(page)
    await dialog.locator('[role="alert"]').waitFor()
    assert(await dialog.getByRole('button', { name: 'Simpan Resep', exact: true }).isDisabled())
    await cancel(dialog)
    state.failPattern = true
    dialog = await openPattern(page)
    await dialog.locator('[role="alert"]').waitFor()
    assert(await dialog.getByRole('button', { name: 'Simpan Pola', exact: true }).isDisabled())
    await cancel(dialog)
    assert.equal(state.writes.length, 2)
    await editor.close()
    console.log('PASS failed detail loads cannot overwrite existing formulas with an empty payload')

    for (const [permissions, scope, label] of [
      [['recipes.view'], 'all', 'read-only F&B'],
      [['recipes.update'], 'specific', 'outlet-scoped F&B editor'],
    ]) {
      const viewer = await session(browser, permissions, scope)
      const sfg = await openSfg(viewer.page, 'Lihat Resep')
      await sfg.getByLabel('Jumlah komponen').first().waitFor()
      await assertReadOnly(sfg)
      await cancel(sfg)
      const pattern = await openPattern(viewer.page, 'Lihat Pola')
      await pattern.getByLabel('Nama pola', { exact: true }).first().waitFor()
      await assertReadOnly(pattern)
      await cancel(pattern)
      assert.equal(viewer.state.writes.length, 0)
      await viewer.close()
      console.log(`PASS ${label}: refreshed permissions/scope allow reading but disable all authoring`)
    }

    const warehouse = await session(browser, ['stockitems.view', 'stockledger.view', 'stockledger.adjust'])
    await warehouse.page.goto(base + '/stock-items')
    await warehouse.page.getByRole('heading', { name: 'Bahan Baku', exact: true }).waitFor()
    await warehouse.page.getByText('Saus Uji', { exact: true }).last().waitFor()
    assert.equal(await warehouse.page.getByRole('button', { name: /^(Resep|Atur Resep|Atur Pemotongan|Atur Pola)$/ }).count(), 0)
    assert.equal(await warehouse.page.getByRole('link', { name: 'Buka pusat resep F & B', exact: true }).count(), 0)
    await warehouse.page.goto(base + '/fnb/recipes?tab=sfg')
    await warehouse.page.getByRole('heading', { name: 'Akses ditolak', exact: true }).waitFor()
    await warehouse.page.goto(base + '/stock-ledger')
    await warehouse.page.getByRole('button', { name: 'Produksi (Masak)', exact: true }).click()
    const produce = warehouse.page.getByRole('dialog', { name: 'Produksi / Memasak', exact: true })
    await produce.getByRole('combobox').nth(0).click()
    await warehouse.page.getByRole('option', { name: 'Saus Uji (SFG)', exact: true }).click()
    await produce.getByRole('combobox').nth(1).click()
    await warehouse.page.getByRole('option', { name: /Gudang Uji/ }).click()
    await produce.getByLabel('Kuantitas Hasil Produksi', { exact: true }).fill('12.5')
    await produce.getByLabel('Catatan Produksi', { exact: true }).fill('Produksi uji tanpa izin pengaturan resep')
    await produce.getByRole('button', { name: 'Proses Produksi', exact: true }).click()
    await produce.waitFor({ state: 'hidden' })
    assert.deepEqual(warehouse.state.writes, [{ method: 'POST', path: '/api/v1/admin/stock-items/sfg/produce', body: { item_id: 'sfg', warehouse_id: 'warehouse-1', qty_produce: 12.5, notes: 'Produksi uji tanpa izin pengaturan resep' } }])
    await warehouse.close()
    console.log('PASS warehouse-only: no recipe editor/F&B access; stockledger.adjust still executes production')

    // Even recipe authors cannot execute stock movements without adjust.
    const noAdjust = await session(browser, ['stockitems.view', 'stockledger.view', 'recipes.update'])
    await noAdjust.page.goto(base + '/stock-items')
    await noAdjust.page.getByText('Saus Uji', { exact: true }).last().waitFor()
    assert.equal(await noAdjust.page.getByRole('button', { name: /^(Resep|Atur Resep|Atur Pemotongan|Atur Pola)$/ }).count(), 0)
    assert.equal(await noAdjust.page.getByRole('link', { name: 'Buka pusat resep F & B', exact: true }).count(), 0)
    await noAdjust.page.goto(base + '/stock-ledger')
    await noAdjust.page.getByRole('heading', { name: 'Kartu Stok', exact: true }).waitFor()
    assert.equal(await noAdjust.page.getByRole('link', { name: 'Buka pusat resep F & B', exact: true }).count(), 0)
    assert.equal(await noAdjust.page.getByRole('button', { name: /^(Produksi \(Masak\)|Pemotongan)$/ }).count(), 0)
    assert.equal(noAdjust.state.writes.length, 0)
    await noAdjust.close()
    console.log('PASS recipe authoring stays out of Gudang; recipes.update never grants stock execution')
  } finally {
    await browser.close()
  }
})().catch(error => { console.error(error); process.exitCode = 1 })
