// All requests are intercepted; no production records are read or written.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright')
const assert = require('node:assert/strict')
;(async () => {
  const browser = await chromium.launch({ args: ['--no-sandbox'] })
  try {
    const page = await browser.newPage({ viewport: { width: 1440, height: 1000 }, acceptDownloads: true })
    const errors = [], downloads = []
    let failDownload = false
    let docs = [{ id: 'd1', document_number: 'SOP/OPS/001', about: 'Panduan pembukaan outlet', file_name: 'Panduan outlet.pdf', file_size: 51200, uploaded_at: '2026-10-22T03:30:00Z', uploaded_by: 'Candra', download_count: 0 }]
    page.on('pageerror', e => errors.push(e.message))
    page.on('console', m => { if (m.text().includes('[Vue warn]')) errors.push(m.text()) })
    await page.addInitScript(() => {
      localStorage.setItem('cloud_pos_token', 'uji-dokumen')
      localStorage.setItem('cloud_pos_admin', JSON.stringify({ username: 'uji', role: 'uji', name: 'Pengguna Dokumen' }))
      localStorage.setItem('cloud_pos_timezone', 'Asia/Jakarta')
      if (!localStorage.getItem('cloud_pos_permissions')) localStorage.setItem('cloud_pos_permissions', JSON.stringify(['documents.view', 'documents.create', 'documents.download']))
    })
    await page.route('**/api/v1/**', async route => {
      const req = route.request(), url = new URL(req.url())
      assert.equal(req.headers().authorization, 'Bearer uji-dokumen')
      assert(url.pathname.startsWith('/api/v1/admin/documents'))
      if (url.pathname.endsWith('/download')) {
        assert.equal(req.method(), 'POST')
        if (failDownload) return route.fulfill({ status: 404, json: { success: false, error: 'Berkas hilang' } })
        const doc = docs.find(d => url.pathname.includes('/' + d.id + '/'))
        doc.download_count++
        downloads.push(doc.id)
        return route.fulfill({ contentType: 'application/octet-stream', body: '%PDF-1.7\nDokumen uji' })
      }
      if (req.method() === 'POST') {
        const body = req.postDataBuffer().toString()
        const field = name => body.match(new RegExp('name="' + name + '"\r\n\r\n([^\r]*)'))?.[1]
        const number = field('document_number')
        assert(number && field('about'))
        assert(body.includes('%PDF-1.7'))
        if (docs.some(d => d.document_number === number)) return route.fulfill({ status: 409, json: { success: false, error: 'Nomor dokumen sudah digunakan. Gunakan nomor lain.' } })
        const doc = { ...docs[0], id: 'd2', document_number: number, about: field('about'), file_name: 'baru.pdf', download_count: 0 }
        docs.unshift(doc)
        return route.fulfill({ status: 201, json: { success: true, data: doc } })
      }
      assert.equal(req.method(), 'GET')
      const search = (url.searchParams.get('search') || '').toLowerCase()
      const filtered = docs.filter(d => (d.document_number + d.about + d.file_name).toLowerCase().includes(search))
      return route.fulfill({ json: { success: true, data: { items: filtered, total: filtered.length } } })
    })
    await page.goto((process.env.DOCUMENTS_TEST_URL || 'http://127.0.0.1:4190') + '/tests/fixtures/documents.html')
    await page.getByRole('cell', { name: 'SOP/OPS/001', exact: true }).waitFor()
    assert(await page.getByRole('link', { name: 'Dokumen', exact: true }).isVisible())
    await page.getByRole('columnheader', { name: 'Waktu unggah (Timestamp)' }).waitFor()
    await page.getByRole('cell', { name: '22-10-2026, 10:30 Candra' }).waitFor()
    const search = page.getByRole('searchbox', { name: 'Cari dokumen' })
    await search.fill('tidak ditemukan')
    await page.getByRole('cell', { name: 'Tidak ada dokumen yang cocok dengan pencarian.' }).waitFor()
    await search.fill('')
    await page.getByRole('cell', { name: 'SOP/OPS/001', exact: true }).waitFor()
    console.log('PASS sidebar, metadata, timezone and search')

    await page.getByRole('button', { name: 'Unggah dokumen', exact: true }).click()
    const modal = page.getByRole('dialog', { name: 'Unggah dokumen', exact: true })
    await modal.getByLabel('Nomor dokumen').fill('SOP/OPS/001')
    await modal.getByLabel('Tentang', { exact: true }).fill('SOP pelayanan pelanggan')
    await modal.getByLabel('Berkas dokumen').setInputFiles({ name: 'baru.pdf', mimeType: 'application/pdf', buffer: Buffer.from('%PDF-1.7\nbaru') })
    await modal.getByRole('button', { name: 'Simpan dokumen' }).click()
    await modal.getByText('Nomor dokumen sudah digunakan. Gunakan nomor lain.').waitFor()
    assert.equal(await modal.getByLabel('Tentang', { exact: true }).inputValue(), 'SOP pelayanan pelanggan')
    await modal.getByLabel('Nomor dokumen').fill('SOP/OPS/002')
    await modal.getByRole('button', { name: 'Simpan dokumen' }).click()
    await modal.waitFor({ state: 'hidden' })
    await page.getByRole('cell', { name: 'SOP/OPS/002', exact: true }).waitFor()
    const downloadEvent = page.waitForEvent('download')
    await page.getByRole('button', { name: 'Unduh SOP/OPS/002', exact: true }).click()
    const download = await downloadEvent
    assert.equal(download.suggestedFilename(), 'baru.pdf')
    await page.getByRole('cell', { name: '1 kali', exact: true }).waitFor()
    assert.deepEqual(downloads, ['d2'])
    failDownload = true
    await page.getByRole('button', { name: 'Unduh SOP/OPS/002', exact: true }).click()
    await page.getByText('Dokumen atau berkas tidak ditemukan.', { exact: true }).waitFor()
    assert.deepEqual(downloads, ['d2'])
    console.log('PASS upload, duplicate retry, download filename/counter and failed download')

    if (process.env.DOCUMENTS_SCREENSHOT) await page.screenshot({ path: process.env.DOCUMENTS_SCREENSHOT, fullPage: true })
    await page.setViewportSize({ width: 390, height: 844 })
    await page.getByRole('heading', { name: 'Dokumen', exact: true }).waitFor()
    assert(await page.getByRole('button', { name: 'Unduh SOP/OPS/002', exact: true }).isVisible())
    assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth))
    await page.getByRole('button', { name: 'Open menu' }).click()
    assert(await page.getByRole('link', { name: 'Dokumen', exact: true }).isVisible())
    await page.getByRole('link', { name: 'Dokumen', exact: true }).click()
    await page.evaluate(() => localStorage.setItem('cloud_pos_permissions', JSON.stringify(['documents.view'])))
    await page.reload()
    await page.getByText('SOP/OPS/002', { exact: true }).last().waitFor()
    assert.equal(await page.getByRole('button', { name: 'Unggah dokumen', exact: true }).count(), 0)
    assert.equal(await page.getByRole('button', { name: /^Unduh SOP/ }).count(), 0)
    assert.deepEqual(errors, [])
    console.log('PASS mobile layout, sidebar and read-only permissions')
  } finally { await browser.close() }
})().catch(error => { console.error(error); process.exitCode = 1 })
