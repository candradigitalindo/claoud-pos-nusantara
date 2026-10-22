// Mocked API only: no production writes.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright')
const assert = require('node:assert/strict')
;(async () => {
  const browser = await chromium.launch({ args: ['--no-sandbox'] })
  try {
    const page = await browser.newPage({ viewport: { width: 1366, height: 900 } })
    const errors = [], writes = []
    let targetPct = 40
    await page.addInitScript(() => localStorage.setItem('cloud_pos_permissions', JSON.stringify(['fnb.costing.view', 'fnb.costing.update'])))
    page.on('pageerror', e => errors.push(e.message))
    page.on('console', m => { if (m.text().includes('[Vue warn]')) errors.push(m.text()) })
    const ingredient = { item_id: 'i1', item_name: 'Bahan A', unit: 'botol', qty_base: 3.5, cost_per_base: 11000, cost: 38500 }
    const recipe = { recipe_master_id: 'r1', name: 'Resep dasar', multiplier: 1, current_cost: 22000, initial_cost: { total: 20000, captured_at: '2026-10-21T03:00:00Z', source: 'central', ingredients: [{ ...ingredient, qty_base: 2, cost_per_base: 10000, cost: 20000 }] } }
    const row = { product_id: 'p1', product_name: 'Menu gabungan', outlet_name: 'Outlet Pusat', menu_kind: 'food', category: 'Menu utama', cost_status: 'ready', price: 100000, total_cost: 41500, material_cost: 38500, reference_material_cost: 35000, packaging_cost: 1000, overhead_cost: 2000, cost_pct: 41.5, target_cost_pct: 40, outlet_target_cost_pct: 40, target_source: 'outlet', cost_basis: 'outlet_stock', projected_total_cost: 50000, projected_cost_pct: 50, projected_is_over: true, projected_suggested_price: 125000, max_material_cost: 37000, material_reduction: 1500, projected_material_reduction: 10000, price_comparisons: [{ item_id: 'i1', item_name: 'Bahan A', unit: 'botol', qty_base: 3.5, current_unit_cost: 11000, projected_unit_cost: 20000, source: 'central_purchase', as_of: '2026-10-22T00:00:00Z' }], margin: 58500, suggested_price: 118572, is_over: true, ingredients: [ingredient], recipes: [recipe, { ...recipe, recipe_master_id: 'r2', name: 'Saus pelengkap', multiplier: 0.5, current_cost: 33000, initial_cost: { ...recipe.initial_cost, total: 30000 } }] }
    const missing = { ...row, product_id: 'p2', product_name: 'Menu tanpa acuan', cost_status: 'missing_cost', total_cost: 0, reference_material_cost: null, is_over: false, projected_total_cost: null, projected_is_over: false, projected_cost_pct: null, price_comparisons: [], recipes: [{ ...recipe, current_cost: null, initial_cost: null }], ingredients: [] }
    await page.route('**/api/v1/**', route => {
      const path = new URL(route.request().url()).pathname
      if (route.request().method() === 'PUT') {
        assert(path.endsWith('/outlets/o1/policy'))
        writes.push(route.request().postDataJSON()); targetPct = writes.at(-1).target_cost_pct
        return route.fulfill({ json: { success: true } })
      }
      assert.equal(route.request().method(), 'GET')
      const data = path.endsWith('/policy') ? { outlet_id: 'o1', outlet_name: 'Outlet Pusat', target_cost_pct: targetPct, configured: true }
        : path.endsWith('/costing') ? { rows: [{ ...row, target_cost_pct: targetPct, outlet_target_cost_pct: targetPct, is_over: row.cost_pct >= targetPct }, missing], total: 2 }
        : [{ id: 'o1', name: 'Outlet Pusat' }]
      return route.fulfill({ json: { success: true, data } })
    })
    await page.goto((process.env.PRODUCT_RECIPE_TEST_URL || 'http://127.0.0.1:4189') + '/tests/fixtures/fnb-costing.html')
    await page.getByText('Menu gabungan', { exact: true }).waitFor()
    const policyButton = page.getByRole('button', { name: 'Atur batas outlet', exact: true })
    assert(await policyButton.isDisabled())
    await page.locator('.ss-trigger').first().click()
    await page.getByRole('option', { name: 'Outlet Pusat', exact: true }).click()
    await policyButton.click()
    const policyDialog = page.getByRole('dialog', { name: 'Batas HPP outlet', exact: true })
    await policyDialog.getByRole('spinbutton', { name: 'Batas HPP (%)' }).fill('45')
    await policyDialog.getByRole('button', { name: 'Batal', exact: true }).click()
    assert.equal(writes.length, 0)
    await policyButton.click()
    await policyDialog.getByRole('spinbutton', { name: 'Batas HPP (%)' }).fill('45')
    await policyDialog.getByRole('button', { name: 'Simpan batas outlet', exact: true }).click()
    await policyDialog.waitFor({ state: 'hidden' })
    assert.deepEqual(writes, [{ target_cost_pct: 45 }])
    await page.getByRole('button', { name: 'Rincian', exact: true }).first().click()
    await page.getByText('Peringatan dini: proyeksi mencapai batas 45%', { exact: true }).waitFor()
    await page.getByText('1. Tinjau harga jual', { exact: true }).waitFor()
    await page.getByText('2. Tinjau biaya resep', { exact: true }).waitFor()
    await page.getByText('Penerimaan induk terbaru', { exact: false }).first().waitFor()
    await page.getByText('Resep dasar × 1', { exact: true }).waitFor()
    await page.getByText('Saus pelengkap × 0.5', { exact: true }).waitFor()
    const detail = page.locator('tr').filter({ hasText: 'Resep untuk 1 menu' })
    assert.match(await detail.innerText(), /38\.500/)
    assert.match(await detail.innerText(), /35\.000/)
    assert.match(await detail.innerText(), /3\.500/)
    await page.getByText('Harga dan acuan resep', { exact: true }).first().click()
    await page.locator('.recipe-cost-summary summary').first().click()
    await page.locator('.reference-line').first().waitFor()
    if (process.env.FNB_COST_SCREENSHOT) await page.screenshot({ path: process.env.FNB_COST_SCREENSHOT, animations: 'disabled' })
    await page.getByRole('button', { name: 'Rincian', exact: true }).nth(1).click()
    await page.getByText('Belum tersedia lengkap', { exact: true }).waitFor()
    await page.getByText('Harga dan acuan resep', { exact: true }).click()
    await page.getByText('Harga belum lengkap', { exact: true }).first().waitFor()
    await page.getByText('Belum direkam', { exact: true }).waitFor()
    await page.setViewportSize({ width: 390, height: 844 })
    assert(await page.getByRole('heading', { name: 'HPP Menu', exact: true }).isVisible())
    assert(!((await page.locator('body').innerText()).includes('NaN')))
    assert.deepEqual(errors, [])
    console.log('PASS outlet policy save/cancel, projected alert, price/recipe recommendations, menu recipe breakdown, current/reference difference, snapshot composition, missing prices and mobile rendering')
  } finally { await browser.close() }
})().catch(e => { console.error(e); process.exitCode = 1 })
