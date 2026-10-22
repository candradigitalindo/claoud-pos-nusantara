// Mocked API: this test does not read or modify production data.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright')
const assert = require('node:assert/strict')
;(async () => {
  const browser = await chromium.launch({ args: ['--no-sandbox'] })
  try {
    const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } })
    const errors = [], project = { id: 'p1', name: 'Renovasi outlet', project_number: 'PRJ001', status: 'berjalan' }
    let report, mode = 'creator', used = 0
    const material = () => ({ id: 'm1', name: 'Semen', unit: 'sak', qty_received: 100, qty_used: used, qty_returned: 0, qty_wasted: 0, remaining: 100 - used, unit_cost: 50000, request_number: 'PR001' })
    const overview = () => ({ project, materials: [material()], summary: { lines: 1, value_received: 5000000, value_used: used * 50000, value_remaining: (100 - used) * 50000, unsettled_lines: 1 }, pending_reports: report && report.status !== 'verified' ? 1 : 0, assets: [{ id: 'a1', number: 'AST001', name: 'Bor listrik', qty: 1, unit: 'unit', condition: 'baik', location: 'Gudang projek', pic: 'Budi', evidence: ['/uploads/test/proof.png'] }] })
    const proof = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aJ9sAAAAASUVORK5CYII=', 'base64')
    page.on('pageerror', e => errors.push(e.message))
    page.on('console', m => { if (m.text().includes('[Vue warn]')) errors.push(m.text()) })
    await page.addInitScript(() => {
      if (!localStorage.getItem('cloud_pos_token')) {
        localStorage.setItem('cloud_pos_token', 'test')
        localStorage.setItem('cloud_pos_admin', JSON.stringify({ id: 'creator', role: 'tester', name: 'Petugas', username: 'petugas' }))
        localStorage.setItem('cloud_pos_permissions', JSON.stringify(['assets.usage.view', 'assets.usage.create']))
      }
    })
    await page.route('**/uploads/**', r => r.fulfill({ contentType: 'image/png', body: proof }))
    await page.route('**/api/v1/**', async route => {
      const req = route.request(), path = new URL(req.url()).pathname
      const ok = data => route.fulfill({ json: { success: true, data } })
      if (path === '/api/v1/admin/project-usage/projects') return ok([project])
      if (path === '/api/v1/admin/project-usage/projects/p1') return ok(overview())
      if (path.endsWith('/materials')) return ok({ materials: [material()], summary: overview().summary })
      if (path.endsWith('/logs')) return ok([])
      if (path === '/api/v1/admin/upload') return ok({ url: '/uploads/test/proof.png' })
      if (path === '/api/v1/admin/project-usage-export') return route.fulfill({ contentType: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet', body: 'mock-excel' })
      if (path === '/api/v1/admin/project-usage' && req.method() === 'GET') return ok({ items: report ? [report] : [], total: report ? 1 : 0 })
      if (path === '/api/v1/admin/project-usage' && req.method() === 'POST') {
        const input = req.postDataJSON()
        assert.equal(input.items[0].qty, 60); assert.equal(input.evidence.length, 1)
        report = { ...input, id: 'r1', report_number: 'PBG001', project_name: project.name, status: 'draft', version: 1, created_by: 'Petugas', created_by_id: 'creator', created_at: '2026-10-22T06:00:00Z', events: [], items: [{ ...input.items[0], name: 'Semen', unit: 'sak', request_number: 'PR001' }] }
        return ok(report)
      }
      if (path.endsWith('/actions')) {
        const input = req.postDataJSON(); assert.equal(input.version, report.version)
        if (input.action === 'submit') { report.status = 'submitted'; report.submitted_by_id = 'creator'; used = report.items[0].qty }
        if (input.action === 'revise') { assert.equal(mode, 'reviewer'); report.status = 'revision'; report.review_note = input.note }
        if (input.action === 'verify') { assert.equal(mode, 'reviewer'); report.status = 'verified'; report.reviewed_by = 'Atasan'; report.reviewed_at = '2026-10-22T06:20:00Z' }
        report.version++; return ok(report)
      }
      if (path.endsWith('/r1') && req.method() === 'PUT') {
        const input = req.postDataJSON(); assert.equal(input.items[0].qty, 55)
        report = { ...report, ...input, version: report.version + 1 }; return ok(report)
      }
      if (path.endsWith('/r1')) return ok(report)
      if (path.endsWith('/my-outlets')) return ok({ outlets: [] })
      if (path.endsWith('/asset-categories')) return ok([])
      throw new Error(`Unexpected API ${req.method()} ${path}`)
    })
    const url = (process.env.PROJECT_USAGE_TEST_URL || 'http://127.0.0.1:4191') + '/tests/fixtures/project-usage.html'
    await page.goto(url)
    await page.getByRole('heading', { name: 'Penggunaan Barang Projek', exact: true }).waitFor()
    await page.getByRole('button', { name: 'Buat laporan', exact: true }).click()
    const form = page.getByRole('dialog', { name: 'Laporan penggunaan barang', exact: true })
    await form.getByLabel('Lokasi pekerjaan').fill('Dapur lantai 1')
    await form.getByLabel('Pelaksana', { exact: true }).fill('Tukang')
    await form.getByLabel('PIC yang mengonfirmasi penggunaan').fill('Budi')
    await form.getByLabel('Pekerjaan / alasan susut').fill('Pengecoran lantai')
    await form.getByLabel('Barang 1', { exact: true }).selectOption('m1')
    await form.getByLabel('Jumlah 1', { exact: true }).fill('60')
    await form.getByLabel('Bukti penggunaan', { exact: true }).setInputFiles({ name: 'bukti.png', mimeType: 'image/png', buffer: proof })
    await form.getByRole('button', { name: 'Hapus bukti 1' }).waitFor()
    await form.getByRole('button', { name: 'Simpan draft / perbaikan' }).click()
    let detail = page.getByRole('dialog', { name: 'PBG001', exact: true })
    await detail.getByRole('button', { name: 'Ajukan laporan', exact: true }).click()
    await detail.getByText('Diajukan', { exact: true }).waitFor()
    assert.equal(await detail.getByRole('button', { name: 'Verifikasi laporan' }).count(), 0)
    assert.equal(used, 60)
    console.log('PASS create, proof upload, submit and creator cannot verify')
    async function switchRole(next) {
      mode = next
      await page.evaluate(next => {
        localStorage.setItem('cloud_pos_admin', JSON.stringify({ id: next, name: next, role: 'tester' }))
        localStorage.setItem('cloud_pos_permissions', JSON.stringify(['assets.usage.view', ...(next === 'creator' ? ['assets.usage.create'] : next === 'reviewer' ? ['assets.usage.verify'] : [])]))
      }, next)
      await page.reload(); await page.getByRole('button', { name: 'Lihat laporan', exact: true }).click()
      detail = page.getByRole('dialog', { name: 'PBG001', exact: true })
    }
    await switchRole('reviewer')
    await detail.getByLabel('Catatan pemeriksaan / alasan pembatalan').fill('Jumlah sebenarnya 55 sak')
    await detail.getByRole('button', { name: 'Minta perbaikan' }).click()
    await detail.getByText('Perlu perbaikan', { exact: true }).waitFor()
    await switchRole('creator')
    await detail.getByRole('button', { name: 'Perbaiki laporan' }).click()
    await form.getByLabel('Jumlah 1', { exact: true }).fill('55')
    await form.getByRole('button', { name: 'Simpan draft / perbaikan' }).click()
    await detail.getByRole('button', { name: 'Ajukan laporan', exact: true }).click()
    await detail.getByText('Diajukan', { exact: true }).waitFor()
    assert.equal(used, 55)
    await switchRole('reviewer')
    await detail.getByRole('button', { name: 'Verifikasi laporan', exact: true }).click()
    await detail.getByText('Diverifikasi', { exact: true }).waitFor()
    await detail.getByRole('button', { name: 'Tutup', exact: true }).last().click()
    await page.getByRole('button', { name: 'Peralatan & PIC', exact: true }).click()
    await page.getByText('AST001 · Bor listrik', { exact: true }).waitFor()
    await page.getByRole('button', { name: 'Saldo material', exact: true }).click()
    await page.getByText('Semen', { exact: true }).waitFor()
    console.log('PASS revision, resubmit, verification, material and equipment views')
    await page.getByRole('button', { name: 'Laporan & bukti', exact: true }).click()
    const downloadEvent = page.waitForEvent('download')
    await page.getByRole('button', { name: 'Unduh Excel' }).click()
    assert.equal((await downloadEvent).suggestedFilename(), 'penggunaan-barang-projek.xlsx')
    await page.screenshot({ path: '/check/usage-desktop.png', fullPage: true })
    await page.setViewportSize({ width: 390, height: 844 })
    assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth))
    await page.getByRole('button', { name: 'Open menu' }).click()
    const group = page.getByRole('button', { name: 'Perlengkapan', exact: true })
    if (await group.count()) await group.click()
    assert(await page.getByRole('link', { name: 'Penggunaan Barang Projek', exact: true }).isVisible())
    await page.getByRole('link', { name: 'Penggunaan Barang Projek', exact: true }).click()
    await switchRole('viewer')
    assert.equal(await detail.getByRole('button', { name: 'Ajukan laporan' }).count(), 0)
    assert.equal(await detail.getByRole('button', { name: 'Verifikasi laporan' }).count(), 0)
    assert.equal(await page.getByRole('button', { name: 'Buat laporan', exact: true }).count(), 0)
    assert.deepEqual(errors, [])
    console.log('PASS export, mobile sidebar, read-only role and no browser errors')
  } finally { await browser.close() }
})().catch(e => { console.error(e); process.exitCode = 1 })
