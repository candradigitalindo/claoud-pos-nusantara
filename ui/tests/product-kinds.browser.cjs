// Synthetic catalog only. Every API request is intercepted; no production writes.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright')
const assert = require('node:assert/strict')
const base = process.env.PRODUCT_KIND_TEST_URL || 'http://127.0.0.1:4189'
;(async () => {
  const browser = await chromium.launch({ args: ['--no-sandbox'] })
  try {
    const page = await browser.newPage({ viewport: { width: 1366, height: 900 } })
    const errors = [], productWrites = [], applyWrites = [], previews = [], lists = []
    let revision = 1, staleApply = true
    const catalog = Array.from({ length: 883 }, (_, index) => ({
      id: `p${index}`, outlet_id: index % 2 ? 'o2' : 'o1', outlet_name: index % 2 ? 'Outlet Timur' : 'Outlet Pusat',
      name: `Produk ${String(index).padStart(3, '0')}`, code: `SKU-${index}`, category_id: 'c1', category_name: 'Kategori Asli', price: 15000,
      menu_kind: index % 5 === 0 ? 'food' : '', suggested_kind: index % 5 === 0 || index % 4 === 3 ? '' : ['food', 'beverage', 'activity'][index % 3],
      stock_type: 'none', recipes: [],
    }))
    const scoped = outlet => catalog.filter(row => !outlet || row.outlet_id === outlet)
    function report(outlet) {
      const rows = scoped(outlet).map(product => ({
        product_id: product.id, outlet_id: product.outlet_id, outlet_name: product.outlet_name, name: product.name, category_name: product.category_name,
        current_kind: product.menu_kind, suggested_kind: product.menu_kind ? '' : product.suggested_kind,
        status: product.menu_kind ? 'existing' : product.suggested_kind ? 'suggested' : 'needs_review',
        confidence: product.menu_kind || product.suggested_kind ? 'high' : 'none',
        reason: product.menu_kind ? 'Jenis produk sudah ditentukan.' : product.suggested_kind ? 'Bukti nama dan kategori sesuai.' : 'Nama dan kategori belum memberikan bukti yang cukup.',
        fingerprint: `fp-${revision}-${product.id}`,
      }))
      const totals = { total: rows.length, existing: 0, suggested: 0, needs_review: 0, food: 0, beverage: 0, activity: 0 }
      for (const row of rows) { totals[row.status]++; const kind = row.current_kind || row.suggested_kind; if (kind) totals[kind]++ }
      return { rows, totals }
    }
    page.on('pageerror', error => errors.push(error.message))
    page.on('console', message => { if (message.text().includes('[Vue warn]')) errors.push(message.text()) })
    await page.route('**/api/v1/**', async route => {
      const request = route.request(), url = new URL(request.url()), path = url.pathname
      const reply = data => route.fulfill({ json: { success: true, data } })
      if (path.endsWith('/products/classification')) {
        const outlet = url.searchParams.get('outlet_id') || ''
        if (request.method() === 'GET') { previews.push(Object.fromEntries(url.searchParams)); return reply(report(outlet)) }
        const payload = request.postDataJSON()
        applyWrites.push({ outlet, ...payload })
        assert(payload.items.every(item => Object.keys(item).sort().join(',') === 'fingerprint,product_id,suggested_kind'))
        if (staleApply) return route.fulfill({ status: 409, json: { error: 'Preview sudah berubah' } })
        const latest = report(outlet)
        for (const item of payload.items) {
          const row = latest.rows.find(row => row.product_id === item.product_id)
          assert(row && row.status === 'suggested' && row.confidence === 'high')
          assert.equal(item.fingerprint, row.fingerprint)
          assert.equal(item.suggested_kind, row.suggested_kind)
          catalog.find(product => product.id === item.product_id).menu_kind = item.suggested_kind
        }
        revision++
        return reply({ applied: payload.items.length })
      }
      if (path.endsWith('/admin/products') && request.method() === 'GET') {
        lists.push(Object.fromEntries(url.searchParams))
        const kind = url.searchParams.get('menu_kind'), search = url.searchParams.get('search') || ''
        let rows = scoped(url.searchParams.get('outlet_id')).filter(row => row.name.toLowerCase().includes(search.toLowerCase()))
        if (kind) rows = rows.filter(row => kind === 'unclassified' ? !row.menu_kind : row.menu_kind === kind)
        const limit = +(url.searchParams.get('limit') || 20), offset = (+(url.searchParams.get('page') || 1) - 1) * limit
        return reply({ items: rows.slice(offset, offset + limit), total: rows.length })
      }
      if (/\/admin\/products(?:\/p\d+)?$/.test(path) && request.method() !== 'GET') {
        const payload = request.postDataJSON(); productWrites.push(payload)
        return reply({ id: 'created-product', ...payload })
      }
      if (path.endsWith('/categories')) return reply({ items: [{ id: 'c1', name: 'Kategori Asli' }], total: 1 })
      if (path.endsWith('/recipe-options')) return reply([])
      if (path.endsWith('/my-outlets')) return reply([{ id: 'o1', name: 'Outlet Pusat' }, { id: 'o2', name: 'Outlet Timur' }])
      errors.push(`Unexpected API ${request.method()} ${path}`)
      return route.abort()
    })
    await page.goto(base + '/tests/fixtures/product-kinds.html')
    await page.locator('.desktop-only .tr-data').first().waitFor()
    const filter = page.getByRole('combobox', { name: 'Filter jenis produk', exact: true })
    for (const kind of ['food', 'beverage', 'activity', 'unclassified']) {
      const response = page.waitForResponse(response => new URL(response.url()).searchParams.get('menu_kind') === kind)
      await filter.selectOption(kind); await response
      assert.equal(lists.at(-1).menu_kind, kind)
      assert.equal(lists.at(-1).page, '1')
    }
    const resetFilter = page.waitForResponse(response => { const url = new URL(response.url()); return url.pathname.endsWith('/admin/products') && !url.searchParams.has('menu_kind') })
    await filter.selectOption(''); await resetFilter
    await page.locator('.desktop-only .tr-data').first().waitFor()
    await page.locator('.desktop-only').getByRole('button', { name: 'Edit', exact: true }).first().click()
    const edit = page.getByRole('dialog', { name: 'Edit Produk', exact: true })
    assert.equal(await edit.locator('#product-kind').inputValue(), 'food')
    await edit.locator('#product-name').fill('Nama produk diperbarui')
    await edit.getByRole('button', { name: 'Simpan Perubahan' }).click(); await edit.waitFor({ state: 'hidden' })
    assert(!Object.hasOwn(productWrites.at(-1), 'menu_kind'), 'Unrelated edits must preserve newer shared F&B/classifier type')
    assert.equal(productWrites.at(-1).category_id, 'c1')
    await page.locator('.desktop-only').getByRole('button', { name: 'Edit', exact: true }).first().click()
    await edit.locator('#product-kind').selectOption('activity')
    await edit.getByRole('button', { name: 'Simpan Perubahan' }).click(); await edit.waitFor({ state: 'hidden' })
    assert.equal(productWrites.at(-1).menu_kind, 'activity')
    assert.equal(productWrites.at(-1).category_id, 'c1')
    assert.equal(productWrites.at(-1).category_name, 'Kategori Asli')
    assert.equal(productWrites.at(-1).price, 15000)
    assert(!Object.hasOwn(productWrites.at(-1), 'recipes'))
    console.log('PASS server menu_kind filters, edit type payload, category/price/recipe preservation')

    await page.getByRole('button', { name: 'Tambah Produk', exact: true }).click()
    const create = page.getByRole('dialog', { name: 'Tambah Produk', exact: true })
    await create.locator('#product-outlet').click()
    await page.getByRole('option', { name: 'Outlet Pusat', exact: true }).click()
    await create.locator('#product-name').fill('Aktivitas Uji')
    await create.locator('#product-price').fill('50000')
    await create.locator('#product-kind').selectOption('activity')
    await create.locator('#product-category').click()
    await page.getByRole('option', { name: 'Kategori Asli', exact: true }).click()
    await create.getByRole('button', { name: 'Tambah Produk', exact: true }).click(); await create.waitFor({ state: 'hidden' })
    assert.equal(productWrites.at(-1).menu_kind, 'activity'); assert.equal(productWrites.at(-1).category_id, 'c1'); assert.equal(productWrites.at(-1).outlet_id, 'o1')
    console.log('PASS create type and category remain separate')

    await page.getByRole('button', { name: 'Analisis jenis', exact: true }).click()
    const modal = page.getByRole('dialog', { name: 'Analisis jenis produk', exact: true })
    await modal.locator('.classification-row').first().waitFor()
    assert.deepEqual(previews.at(-1), {})
    assert.equal(await modal.locator('.classification-counts > div').first().locator('strong').textContent(), '883')
    assert.equal(await modal.locator('.classification-row').count(), 40)
    const suggested = report('').rows.filter(row => row.status === 'suggested')
    await modal.getByRole('button', { name: `Terapkan ${suggested.length} saran`, exact: true }).waitFor()
    await modal.getByRole('searchbox', { name: 'Cari produk dalam analisis' }).fill('Produk 882')
    assert.equal(await modal.locator('.classification-row').count(), 1)
    await modal.locator('[data-product-id="p882"]').waitFor()
    await modal.getByRole('searchbox', { name: 'Cari produk dalam analisis' }).fill('')
    await modal.getByRole('combobox', { name: 'Filter status analisis' }).selectOption('needs_review')
    await modal.locator('.classification-row.uncertain').first().waitFor()
    assert.equal(await modal.getByRole('checkbox').count(), 0)
    assert.equal(await modal.locator('.classification-counts > div').last().locator('strong').textContent(), String(report('').totals.needs_review))
    await modal.getByRole('combobox', { name: 'Filter status analisis' }).selectOption('')
    await modal.getByRole('combobox', { name: 'Filter jenis dalam analisis' }).selectOption('activity')
    await modal.locator('.classification-row').first().waitFor()
    assert((await modal.locator('.product-kind').allTextContents()).every(label => label === 'Aktivitas'))
    await modal.getByRole('combobox', { name: 'Filter jenis dalam analisis' }).selectOption('')
    await modal.getByRole('button', { name: 'Next ›', exact: true }).click()
    await modal.locator('[data-product-id="p40"]').waitFor()
    console.log('PASS complete 883-row report, page-independent search, type filter and uncertain review')

    async function fits(label) {
      const box = await modal.evaluate(element => {
        const rect = element.getBoundingClientRect(), footer = element.querySelector('footer').getBoundingClientRect()
        return { top: rect.top, right: rect.right, bottom: rect.bottom, footer: footer.bottom, width: innerWidth, height: innerHeight, overflow: element.scrollWidth - element.clientWidth }
      })
      assert(box.top >= 0 && box.right <= box.width && box.bottom <= box.height && box.footer <= box.height && box.overflow <= 1, label + JSON.stringify(box))
    }
    await fits('desktop')
    await page.setViewportSize({ width: 320, height: 568 }); await fits('small phone')
    await page.setViewportSize({ width: 390, height: 844 }); await fits('phone')
    if (process.env.PRODUCT_KIND_SCREENSHOT) await page.screenshot({ path: process.env.PRODUCT_KIND_SCREENSHOT, animations: 'disabled' })
    await page.setViewportSize({ width: 1366, height: 900 })
    await modal.getByRole('button', { name: `Terapkan ${suggested.length} saran`, exact: true }).click()
    assert.equal(applyWrites.length, 0)
    await modal.getByRole('button', { name: 'Ya, terapkan jenis', exact: true }).click()
    await modal.getByRole('alert').filter({ hasText: 'Katalog berubah sejak analisis' }).waitFor()
    assert.equal(applyWrites[0].items.length, suggested.length)
    assert(applyWrites[0].items.some(item => item.product_id === 'p882'))
    assert(await modal.getByRole('button', { name: `Terapkan ${suggested.length} saran`, exact: true }).isDisabled())
    revision++; staleApply = false
    await modal.getByRole('button', { name: 'Muat ulang analisis', exact: true }).click()
    await modal.locator('.classification-row').first().waitFor()
    await modal.getByRole('button', { name: 'Kosongkan pilihan', exact: true }).click()
    await modal.getByRole('checkbox').first().check()
    await modal.getByRole('button', { name: 'Terapkan 1 saran', exact: true }).click()
    await modal.getByRole('button', { name: 'Ya, terapkan jenis', exact: true }).click()
    await modal.getByRole('status').filter({ hasText: '1 produk berhasil ditentukan jenisnya.' }).waitFor()
    assert.equal(applyWrites.at(-1).items.length, 1)
    assert(applyWrites.at(-1).items[0].fingerprint.startsWith('fp-2-'))
    assert.equal(applyWrites.at(-1).outlet, '')
    await modal.getByRole('button', { name: 'Tutup analisis jenis' }).click()
    console.log('PASS explicit confirmation, exact eligible fingerprints, stale409 lock and reload-before-apply')

    await page.locator('.outlet-select-wrap .ss-trigger').click()
    await page.getByRole('option', { name: 'Outlet Timur', exact: true }).click()
    await page.getByRole('button', { name: 'Analisis jenis', exact: true }).click()
    await modal.locator('.classification-row').first().waitFor()
    assert.deepEqual(previews.at(-1), { outlet_id: 'o2' })
    const scopedSuggested = report('o2').totals.suggested
    await modal.getByRole('button', { name: `Terapkan ${scopedSuggested} saran`, exact: true }).click()
    await modal.getByRole('button', { name: 'Ya, terapkan jenis', exact: true }).click()
    await modal.getByRole('status').filter({ hasText: `${scopedSuggested} produk berhasil ditentukan jenisnya.` }).waitFor()
    assert.equal(applyWrites.at(-1).outlet, 'o2')
    assert(applyWrites.at(-1).items.every(item => catalog.find(row => row.id === item.product_id).outlet_id === 'o2'))
    await modal.getByRole('button', { name: 'Tutup analisis jenis' }).click()
    await page.setViewportSize({ width: 390, height: 844 })
    assert(await page.locator('.mobile-cards .product-kind').first().isVisible())
    console.log('PASS outlet-scoped review/apply and mobile type badge')

    await page.goto(base + '/tests/fixtures/product-kinds.html?outlet=o2&readonly')
    const outletFiltered = page.waitForResponse(response => { const url = new URL(response.url()); return url.pathname.endsWith('/admin/products') && url.searchParams.get('menu_kind') === 'activity' && url.searchParams.get('outlet_id') === 'o2' })
    await page.getByRole('combobox', { name: 'Filter jenis produk outlet' }).selectOption('activity')
    await outletFiltered
    assert.equal(lists.at(-1).outlet_id, 'o2')
    await page.getByRole('button', { name: 'Analisis jenis', exact: true }).click()
    await modal.locator('.classification-row').first().waitFor()
    assert.deepEqual(previews.at(-1), { outlet_id: 'o2' })
    assert.equal(await modal.getByRole('checkbox').count(), 0)
    assert.equal(await modal.getByRole('button', { name: /^Terapkan / }).count(), 0)
    await modal.getByText(/Akses lihat saja/).waitFor()
    assert.equal(await modal.locator('.classification-counts > div').first().locator('strong').textContent(), String(scoped('o2').length))
    await fits('outlet mobile read-only')
    console.log('PASS outlet page server filter, whole-outlet analysis and products.view read-only permissions')

    // More than the API's atomic 1,000-row limit must remain visibly bounded.
    catalog.push(...Array.from({ length: 1100 }, (_, index) => ({
      id: `extra${index}`, outlet_id: 'o1', outlet_name: 'Outlet Pusat',
      name: `Saran Tambahan ${String(index).padStart(4, '0')}`, code: `EXTRA-${index}`, category_id: 'c1', category_name: 'Kategori Asli', price: 10000,
      menu_kind: '', suggested_kind: 'activity', stock_type: 'none', recipes: [],
    })))
    await page.setViewportSize({ width: 1366, height: 900 })
    await page.goto(base + '/tests/fixtures/product-kinds.html')
    await page.getByRole('button', { name: 'Analisis jenis', exact: true }).click()
    await modal.locator('.classification-row').first().waitFor()
    const largeSuggestions = report('').rows.filter(row => row.status === 'suggested')
    assert(largeSuggestions.length > 1000)
    await modal.getByRole('status').filter({ hasText: 'Maksimal 1.000 saran per penerapan' }).waitFor()
    await modal.getByRole('button', { name: 'Terapkan 1000 saran', exact: true }).waitFor()
    const firstEligible = largeSuggestions[0], lastEligible = largeSuggestions.at(-1)
    const analysisSearch = modal.getByRole('searchbox', { name: 'Cari produk dalam analisis' })
    await analysisSearch.fill(lastEligible.name)
    assert(await modal.getByRole('checkbox', { name: `Pilih ${lastEligible.name}`, exact: true }).isDisabled())
    await analysisSearch.fill(firstEligible.name)
    await modal.getByRole('checkbox', { name: `Pilih ${firstEligible.name}`, exact: true }).uncheck()
    await analysisSearch.fill(lastEligible.name)
    await modal.getByRole('checkbox', { name: `Pilih ${lastEligible.name}`, exact: true }).check()
    const beforeApply = applyWrites.length
    await modal.getByRole('button', { name: 'Terapkan 1000 saran', exact: true }).click()
    assert.equal(applyWrites.length, beforeApply)
    await modal.getByRole('button', { name: 'Ya, terapkan jenis', exact: true }).click()
    await modal.getByRole('status').filter({ hasText: '1000 produk berhasil ditentukan jenisnya.' }).waitFor()
    assert.equal(applyWrites.length, beforeApply + 1, 'One explicit atomic apply, no automatic chunks')
    assert.equal(applyWrites.at(-1).items.length, 1000)
    assert(!applyWrites.at(-1).items.some(item => item.product_id === firstEligible.product_id))
    assert(applyWrites.at(-1).items.some(item => item.product_id === lastEligible.product_id))
    assert(report('').totals.suggested > 0, 'Remaining suggestions need another review and confirmation')
    assert.deepEqual(errors, [])
    console.log('PASS visible 1000-row cap, replace selected row, single atomic apply and remaining review')
  } finally { await browser.close() }
})().catch(error => { console.error(error); process.exitCode = 1 })
