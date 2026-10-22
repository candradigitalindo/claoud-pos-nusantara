// Requests are intercepted; the checks never write production records.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright')
const assert = require('node:assert/strict')
const base = process.env.PRODUCT_RECIPE_TEST_URL || 'http://127.0.0.1:4189'
;(async () => {
  const browser = await chromium.launch({ args: ['--no-sandbox'] })
  try {
    const page = await browser.newPage({ viewport: { width: 1366, height: 768 } })
    const errors = [], writes = []
    let failList = false, failDetail = false, delayDetail = false, noRecipes = false
    let releaseDetail
    const recipes = [
      { id: 'r1', name: 'Nasi Goreng Spesial', menu_kind: 'food', item_count: 3, description: 'Nasi goreng dengan ayam, telur, dan bumbu pilihan.', total_time: 12 },
      { id: 'r2', name: 'Es Teh Lemon', menu_kind: 'beverage', item_count: 2, description: 'Teh segar dengan perasan lemon.', total_time: 4 },
      { id: 'r3', name: 'Ayam Bakar Madu', menu_kind: 'food', item_count: 2, description: 'Ayam bakar dengan olesan madu.', total_time: 20 },
    ]
    const items = [
      { id: 'i1', item_id: 'i1', item_name: 'Beras', qty_base: 125.5, unit: 'g', notes: 'Timbang sebelum dimasak.' },
      { id: 'i2', item_id: 'i2', item_name: 'Telur ayam', qty_base: 1, unit: 'butir', notes: '' },
      { id: 'i3', item_id: 'i3', item_name: 'Bumbu nasi goreng', qty_base: 25, unit: 'g', notes: 'Gunakan bumbu standar dapur.' },
    ]
    const instructions = JSON.stringify([{ text: '<p>Panaskan wajan.</p><p>Masukkan bahan dan aduk rata.</p><script>window.recipeXss=true</script><img src=x onerror="window.recipeXss=true">', duration: 5 }, { text: '<p>Sajikan selagi hangat.</p>', duration: 2 }])
    const product = { id: 'p1', outlet_id: 'o1', outlet_name: 'Outlet Pusat', name: 'Nasi Goreng Spesial', code: 'NGS', description: 'Nasi goreng dengan telur dan ayam.', price: 28000, category_id: 'c1', category_name: 'Makanan', recipe_master_id: 'r1', recipe_master_name: recipes[0].name, stock_type: 'recipe' }
    page.on('pageerror', e => errors.push(e.message))
    page.on('console', m => { if (m.text().includes('[Vue warn]')) errors.push(m.text()) })
    await page.route('**/api/v1/**', async route => {
      const req = route.request(), url = new URL(req.url()), path = url.pathname
      const reply = data => route.fulfill({ json: { success: true, data } })
      if (req.method() !== 'GET') { writes.push(req.postDataJSON()); return reply(product) }
      if (path.endsWith('/recipe-options')) {
        if (failList) return route.fulfill({ status: 500, json: { error: 'Daftar resep gagal dimuat.' } })
        return reply(noRecipes || url.searchParams.get('outlet_id') === 'o2' ? [] : recipes)
      }
      if (path.includes('/recipe-options/')) {
        assert.equal(url.searchParams.get('outlet_id'), 'o1')
        const id = path.split('/').at(-1)
        if (id === 'r3' && delayDetail) await new Promise(resolve => { releaseDetail = resolve })
        if (failDetail) return route.fulfill({ status: 404, json: { error: 'Resep tidak tersedia untuk outlet ini.' } })
        return reply({ ...recipes.find(r => r.id === id), items: id === 'r1' ? items : items.slice(0, 2), instructions, current_cost: 5000, cost_ingredients: items.map(i => ({ ...i, cost_per_base: 100, cost: 1000 })), initial_cost: { total: 4000, captured_at: '2026-10-21T10:00:00Z', source: 'central', ingredients: items.map(i => ({ ...i, cost_per_base: 90, cost: 900 })) } })
      }
      if (path.endsWith('/products')) return reply({ items: [product], total: 1 })
      if (path.endsWith('/categories')) return reply({ items: [{ id: 'c1', name: 'Makanan' }], total: 1 })
      if (path.includes('/outlets') || path.endsWith('/my-outlets')) return reply([{ id: 'o1', name: 'Outlet Pusat' }, { id: 'o2', name: 'Outlet Timur' }])
      errors.push('Unexpected API ' + path); return route.abort()
    })
    await page.goto(base + '/tests/fixtures/product-recipes.html')
    const form = page.getByRole('dialog', { name: 'Edit Produk', exact: true })
    const picker = page.getByRole('dialog', { name: 'Pilih resep produk', exact: true })
    const use = picker.getByRole('button', { name: 'Terapkan pilihan', exact: true })
    async function openEdit() { await page.locator('.desktop-only').getByRole('button', { name: 'Edit', exact: true }).click() }
    async function openPicker() { await form.locator('#product-recipe').click(); await picker.waitFor() }
    async function ready(name) { await picker.getByRole('heading', { name, exact: true }).waitFor(); assert(await picker.getByRole('button', { name: /^(Tambahkan|Lepas) resep$/ }).isEnabled()) }
    async function choose(name) { await picker.locator('.recipe-choice').filter({ hasText: name }).click(); await ready(name) }
    async function fits(label) {
      const dimensions = await picker.evaluate(el => { const rect = el.getBoundingClientRect(), footer = el.querySelector('footer').getBoundingClientRect(); return { top: rect.top, bottom: rect.bottom, right: rect.right, width: innerWidth, height: innerHeight, footer: footer.bottom, overflow: el.scrollHeight - el.clientHeight } })
      assert(dimensions.top >= 0 && dimensions.bottom <= dimensions.height && dimensions.right <= dimensions.width && dimensions.footer <= dimensions.height && dimensions.overflow <= 1, label + JSON.stringify(dimensions))
    }
    await openEdit(); await openPicker(); await ready('Nasi Goreng Spesial')
    await picker.getByText('125,5', { exact: false }).first().waitFor()
    await picker.getByRole('button', { name: 'HPP & acuan', exact: true }).click()
    await picker.getByText('HPP bahan terkini', { exact: true }).waitFor()
    await picker.getByText('Acuan saat dibuat', { exact: true }).waitFor()
    await picker.getByRole('button', { name: 'Bahan & takaran', exact: true }).click()
    await picker.getByText('Timbang sebelum dimasak.', { exact: true }).waitFor()
    assert.equal(await picker.locator('.recipe-choice').count(), 3)
    await fits('desktop')
    await use.focus(); await page.keyboard.press('Tab')
    assert.equal(await page.evaluate(() => document.activeElement.getAttribute('aria-label')), 'Tutup pilihan resep')
    if (process.env.PRODUCT_RECIPE_SCREENSHOT) await page.screenshot({ path: process.env.PRODUCT_RECIPE_SCREENSHOT, animations: 'disabled' })
    await picker.getByRole('button', { name: /^Langkah SOP/ }).click()
    await picker.getByText('Panaskan wajan.', { exact: false }).waitFor()
    assert.equal(await picker.locator('.recipe-steps img, .recipe-steps script').count(), 0)
    assert.equal(await page.evaluate(() => window.recipeXss), undefined)
    console.log('PASS ingredients, quantities, notes and safe SOP text')
    await picker.getByRole('button', { name: 'Minuman', exact: true }).click(); await ready('Es Teh Lemon')
    assert.equal(await picker.locator('.recipe-choice').count(), 1)
    await picker.getByRole('button', { name: 'Semua', exact: true }).click()
    await picker.getByRole('searchbox', { name: 'Cari nama resep' }).fill('ayam'); await ready('Ayam Bakar Madu')
    assert.equal(await picker.locator('.recipe-choice').count(), 1)
    await picker.getByRole('searchbox', { name: 'Cari nama resep' }).fill('tidak ada')
    await picker.getByText('Resep tidak ditemukan', { exact: true }).waitFor(); assert.equal(await picker.getByRole('button', { name: /^(Tambahkan|Lepas) resep$/ }).count(), 0)
    await picker.getByRole('button', { name: 'Reset pencarian' }).click()
    await choose('Es Teh Lemon'); await picker.getByRole('button', { name: 'Tambahkan resep', exact: true }).click()
    await picker.getByRole('spinbutton', { name: 'Pengali Es Teh Lemon' }).fill('0.5')
    await picker.getByRole('button', { name: 'Batal', exact: true }).click()
    assert.equal(await form.locator('#product-recipe').textContent().then(t => t.trim()), 'Nasi Goreng Spesial')
    await page.waitForFunction(() => document.activeElement?.id === 'product-recipe')
    assert.equal(writes.length, 0)
    console.log('PASS search, category, empty result and cancel preserves selection/focus')
    await openPicker(); await choose('Es Teh Lemon')
    assert.equal(await picker.locator('.selection-item').count(), 1)
    await picker.getByRole('button', { name: 'Tambahkan resep', exact: true }).click()
    const multiplier = picker.getByRole('spinbutton', { name: 'Pengali Es Teh Lemon' })
    await multiplier.fill('0'); assert(await use.isDisabled())
    await multiplier.fill('0.5'); await use.click()
    await form.locator('#product-recipe').getByText('2 resep ditautkan').waitFor()
    assert.equal(writes.length, 0)
    await form.getByRole('button', { name: 'Simpan Perubahan' }).click(); await form.waitFor({ state: 'hidden' })
    assert.deepEqual(writes.at(-1).recipes.map(({ recipe_master_id, multiplier }) => ({ recipe_master_id, multiplier })), [{ recipe_master_id: 'r1', multiplier: 1 }, { recipe_master_id: 'r2', multiplier: 0.5 }])
    await openEdit(); await openPicker(); await picker.getByRole('button', { name: 'Tanpa resep', exact: true }).click()
    await form.locator('#product-recipe').getByText('Pilih resep').waitFor()
    await form.getByRole('button', { name: 'Simpan Perubahan' }).click(); await form.waitFor({ state: 'hidden' })
    assert.deepEqual(writes.at(-1).recipes, [])
    console.log('PASS multiple recipes, fractional multiplier, zero validation, draft cancel and optional unlink')
    await openEdit(); await openPicker(); await ready('Nasi Goreng Spesial')
    delayDetail = true
    await picker.locator('.recipe-choice').filter({ hasText: 'Ayam Bakar Madu' }).click()
    await picker.getByText('Memuat isi resep…', { exact: true }).waitFor(); assert.equal(await picker.getByRole('button', { name: /^(Tambahkan|Lepas) resep$/ }).count(), 0)
    await choose('Es Teh Lemon')
    while (!releaseDetail) await page.waitForTimeout(20)
    releaseDetail(); delayDetail = false
    await page.waitForResponse(r => r.url().includes('/recipe-options/r3'))
    await ready('Es Teh Lemon')
    failDetail = true
    await picker.locator('.recipe-choice').filter({ hasText: 'Ayam Bakar Madu' }).click()
    await picker.getByText('Resep tidak tersedia untuk outlet ini.', { exact: true }).waitFor(); assert.equal(await picker.getByRole('button', { name: /^(Tambahkan|Lepas) resep$/ }).count(), 0)
    failDetail = false; await picker.getByRole('button', { name: 'Coba lagi', exact: true }).click(); await ready('Ayam Bakar Madu')
    await page.keyboard.press('Escape'); await picker.waitFor({ state: 'hidden' }); assert(await form.isVisible())
    console.log('PASS stale requests, detail failure/retry and Escape closes only the picker')
    await page.setViewportSize({ width: 390, height: 844 })
    await form.getByRole('button', { name: 'Foto & resep', exact: true }).click(); await openPicker()
    await fits('phone list'); assert(await picker.getByRole('searchbox', { name: 'Cari nama resep' }).isVisible())
    await choose('Nasi Goreng Spesial'); await fits('phone detail')
    assert(await picker.locator('.ingredients').isVisible())
    if (process.env.PRODUCT_RECIPE_MOBILE_SCREENSHOT) await page.screenshot({ path: process.env.PRODUCT_RECIPE_MOBILE_SCREENSHOT, animations: 'disabled' })
    await page.setViewportSize({ width: 320, height: 568 }); await fits('small phone detail')
    await picker.getByRole('button', { name: 'Daftar resep', exact: true }).click(); await fits('small phone list')
    await picker.getByRole('button', { name: 'Tutup pilihan resep', exact: true }).click()
    await form.getByRole('button', { name: 'Tutup formulir produk' }).click()
    await page.setViewportSize({ width: 1366, height: 768 })
    failList = true
    await openEdit(); await openPicker(); await picker.getByText('Daftar resep gagal dimuat.', { exact: true }).waitFor(); assert.equal(await picker.getByRole('button', { name: /^(Tambahkan|Lepas) resep$/ }).count(), 0)
    failList = false; noRecipes = true; await picker.getByRole('button', { name: 'Coba lagi' }).click()
    await picker.getByText('Belum ada resep tersedia', { exact: true }).waitFor()
    await picker.getByRole('button', { name: 'Tanpa resep', exact: true }).click()
    assert.equal(await form.locator('#product-recipe').textContent().then(t => t.trim()), 'Pilih resep')
    await form.getByRole('button', { name: 'Tutup formulir produk' }).click()
    noRecipes = false
    await page.getByRole('button', { name: 'Tambah Produk', exact: true }).click()
    const create = page.getByRole('dialog', { name: 'Tambah Produk', exact: true })
    assert(await create.locator('#product-recipe').isDisabled())
    async function outlet(name) {
      await create.locator('#product-outlet').click()
      await page.getByRole('combobox', { name: 'Cari pilihan' }).fill(name)
      await page.getByRole('option', { name, exact: true }).click()
    }
    await outlet('Outlet Pusat')
    await create.locator('#product-name').fill('Teh Lemon Dingin'); await create.locator('#product-price').fill('15000')
    await create.locator('#product-recipe').click(); await choose('Es Teh Lemon'); await picker.getByRole('button', { name: 'Tambahkan resep', exact: true }).click(); await use.click()
    await create.locator('#product-recipe').getByText('Es Teh Lemon').waitFor()
    await outlet('Outlet Timur')
    await create.locator('#product-recipe').getByText('Pilih resep').waitFor()
    await create.locator('#product-recipe').click()
    await picker.getByText('Belum ada resep tersedia', { exact: true }).waitFor()
    await picker.getByRole('button', { name: 'Tanpa resep', exact: true }).click()
    await create.getByRole('button', { name: 'Tambah Produk', exact: true }).click()
    await create.waitFor({ state: 'hidden' })
    assert.equal(writes.at(-1).outlet_id, 'o2'); assert.deepEqual(writes.at(-1).recipes, [])
    console.log('PASS new products require an outlet; switching outlets clears the previous recipe')
    assert.deepEqual(errors, [])
    console.log('PASS mobile list/detail, unavailable recipes and optional recipe selection')
  } finally { await browser.close() }
})().catch(e => { console.error(e); process.exitCode = 1 })
