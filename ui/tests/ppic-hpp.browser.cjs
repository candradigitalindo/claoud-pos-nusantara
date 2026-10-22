// Mocked API only: no production writes.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright')
const assert = require('node:assert/strict')
;(async () => {
  const browser = await chromium.launch({ args: ['--no-sandbox'] })
  try {
    const page = await browser.newPage({ viewport: { width: 1366, height: 900 } })
    const errors = []
    page.on('pageerror', e => errors.push(e.message))
    page.on('console', m => { if (m.text().includes('[Vue warn]')) errors.push(m.text()) })
    const menu = { product_id: 'p1', product_name: 'Menu cabe', outlet_name: 'Outlet A', category: 'Makanan', price: 10000, hpp: 4000, hpp_pct: 40, margin: 6000, target_cost_pct: 40, ideal_cost: 4000, cost_status: 'ready', cost_basis: 'outlet_stock', is_over: true, has_recipe: true, projected_hpp: 5000, projected_cost_pct: 50, projected_is_over: true, packaging_cost: 1000, overhead_cost: 1000, ingredients: [{ item_id: 'i1', item_name: 'Cabe', qty_base: 100, unit: 'g', cost_per_base: 20, cost: 2000 }] }
    const report = { product_count: 2, with_recipe: 2, avg_hpp_pct: 40, over_count: 1, no_recipe_sample: '', rows: [menu, { ...menu, product_id: 'p2', product_name: 'Harga belum ada', outlet_name: 'Outlet B', cost_status: 'missing_cost', target_cost_pct: 45, ideal_cost: 4500, hpp: 0, hpp_pct: 0, is_over: false, projected_hpp: null, projected_cost_pct: null, projected_is_over: false, ingredients: [] }] }
    await page.route('**/api/v1/**', route => {
      assert.equal(route.request().method(), 'GET')
      const url = new URL(route.request().url())
      assert(!url.searchParams.has('ideal_pct'), 'Use saved outlet limits, not a global query override')
      const data = url.pathname.endsWith('/reports/hpp') ? report
        : url.pathname.endsWith('/reports/sold') ? { total_qty: 0, product_count: 0, days: 30, no_recipe_qty: 0, rows: [] }
        : []
      return route.fulfill({ json: { success: true, data } })
    })
    await page.goto((process.env.PRODUCT_RECIPE_TEST_URL || 'http://127.0.0.1:4189') + '/tests/fixtures/ppic-hpp.html')
    await page.getByText('Tidak ada penjualan pada rentang ini.', { exact: true }).waitFor()
    await page.getByRole('button', { name: 'HPP Menu', exact: true }).click()
    const current = page.locator('tr').filter({ has: page.getByText('Menu cabe', { exact: true }) })
    await current.waitFor()
    assert.match(await current.innerText(), /Stok outlet/)
    assert.match(await current.innerText(), /Proyeksi Rp\s*5\.000/)
    assert.match(await current.innerText(), /40\.0%/)
    assert(await current.evaluate(el => el.classList.contains('row-over')))
    const missing = page.locator('tr').filter({ has: page.getByText('Harga belum ada', { exact: true }) })
    assert.match(await missing.innerText(), /45%/)
    assert.equal((await missing.locator('td').nth(6).innerText()).trim(), '—')
    assert.match(await missing.locator('td').nth(4).innerText(), /Biaya belum lengkap/)
    assert.equal(await page.getByRole('spinbutton').count(), 0)
    await current.click()
    await page.getByText('Cabe', { exact: true }).waitFor()
    await page.getByText('Kemasan Rp 1.000 + overhead Rp 1.000 / menu.', { exact: false }).waitFor()
    if (process.env.PPIC_HPP_SCREENSHOT) await page.screenshot({ path: process.env.PPIC_HPP_SCREENSHOT, animations: 'disabled' })
    await page.setViewportSize({ width: 390, height: 844 })
    assert(await page.getByRole('heading', { name: 'Laporan PPIC', exact: true }).isVisible())
    assert(!(await page.locator('body').innerText()).includes('NaN'))
    assert.deepEqual(errors, [])
    console.log('PASS PPIC outlet limits, current/projection costs, missing cost, packaging/overhead, and mobile rendering')
  } finally { await browser.close() }
})().catch(e => { console.error(e); process.exit(1) })
