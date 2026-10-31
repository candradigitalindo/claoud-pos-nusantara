// Synthetic files and intercepted API only; never uploads to or deletes from production.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright')
const assert = require('node:assert/strict')
const base = process.env.APP_POS_TEST_URL || 'http://127.0.0.1:4189'
const latest = {
  name: 'app-universal-release-v10.18.apk',
  url: '/uploads/apps/app-universal-release-v10.18.apk',
  size: 64178671,
  uploaded_at: '2026-10-31T04:45:16Z',
}
const old = {
  name: 'app-arm64-v8a-release-v10.17.apk',
  url: '/uploads/apps/app-arm64-v8a-release-v10.17.apk',
  size: 23945270,
  uploaded_at: '2026-09-04T08:29:08Z',
}
const bundle = {
  name: 'windows-tools-v20.zip',
  url: '/uploads/apps/windows-tools-v20.zip',
  size: 12000,
  uploaded_at: '2026-11-03T04:00:00Z',
}
const uploaded = {
  name: 'app-universal-release-v11.apk',
  url: '/uploads/apps/app-universal-release-v11.apk',
  size: 64,
  uploaded_at: '2026-11-02T04:00:00Z',
}

;(async () => {
  const browser = await chromium.launch({ args: ['--no-sandbox'] })
  try {
    const page = await browser.newPage({ viewport: { width: 1366, height: 950 } })
    page.setDefaultTimeout(10000)
    const errors = [], uploads = []
    let files = [old, bundle, latest], listError = false
    page.on('pageerror', error => errors.push(error.message))
    page.on('console', message => { if (message.text().includes('[Vue warn]')) errors.push(message.text()) })
    await page.route('**/api/v1/**', async route => {
      const request = route.request(), path = new URL(request.url()).pathname
      if (path === '/api/v1/admin/app-files' && request.method() === 'GET') {
        return route.fulfill({ status: listError ? 503 : 200, json: listError
          ? { success: false, error: 'Daftar file sedang tidak tersedia' }
          : { success: true, data: files } })
      }
      if (path === '/api/v1/admin/app-files' && request.method() === 'POST') {
        uploads.push(request.postDataBuffer())
        assert(request.headers()['content-type'].startsWith('multipart/form-data; boundary='))
        assert(uploads.at(-1).includes(Buffer.from('filename="app-universal-release-v11.apk"')))
        files = [uploaded, ...files]
        return route.fulfill({ status: 201, json: { success: true, data: uploaded } })
      }
      errors.push(`Unexpected API ${request.method()} ${path}`)
      return route.abort()
    })

    const url = base + '/tests/fixtures/app-pos.html'
    const hero = page.getByRole('region', { name: 'APK terbaru', exact: true })
    const archive = page.getByRole('region', { name: 'Arsip aplikasi', exact: true })
    const search = page.getByRole('searchbox', { name: 'Cari file aplikasi', exact: true })
    async function format(label, query = label) {
      const trigger = page.getByRole('combobox', { name: 'Filter format', exact: true })
      await trigger.scrollIntoViewIfNeeded()
      // Let resize/autoscroll finish before opening a picker that intentionally
      // closes when its containing page scrolls.
      await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))))
      await trigger.click()
      await page.getByRole('combobox', { name: 'Cari pilihan', exact: true }).fill(query)
      await page.getByRole('option', { name: label, exact: true }).click()
    }
    await page.goto(url)
    await hero.getByRole('link', { name: /Unduh/ }).waitFor()
    assert((await hero.textContent()).includes('v10.18'))
    assert((await hero.textContent()).includes('Android 7.0'))
    assert((await page.textContent('body')).includes('Makanan'))
    assert((await page.textContent('body')).includes('Minuman'))
    assert((await page.textContent('body')).includes('Aktivitas'))
    assert((await page.textContent('body')).includes('Lainnya'))
    assert.equal(await hero.getByRole('link', { name: /Unduh/ }).getAttribute('href'), base + latest.url)
    assert(!(await hero.textContent()).includes(bundle.name))
    console.log('PASS newest Android APK is featured despite newer non-APK; v10.18 notes and download destination match')

    await search.fill('  V10.17  ')
    await archive.getByText(old.name, { exact: true }).first().waitFor({ state: 'attached' })
    assert.equal(await archive.getByText(latest.name, { exact: true }).count(), 0)
    await search.fill('nothing-matches-this-file')
    await archive.getByText(/Tidak ada file yang cocok|Tidak ditemukan/).waitFor()
    await search.fill('')
    await format('ZIP')
    await archive.getByText(bundle.name, { exact: true }).first().waitFor({ state: 'attached' })
    assert.equal(await archive.getByText(latest.name, { exact: true }).count(), 0)
    await format('Semua format', 'semua')
    console.log('PASS archive search and searchable format filtering')

    await page.getByText('Kelola aplikasi · Unggah versi baru', { exact: true }).click()
    const fileInput = page.getByLabel('File aplikasi', { exact: true })
    const uploadButton = page.getByRole('button', { name: 'Unggah aplikasi', exact: true })
    await fileInput.setInputFiles({ name: 'blocked.txt', mimeType: 'text/plain', buffer: Buffer.from('not an app') })
    await page.getByRole('alert').filter({ hasText: /Format/ }).first().waitFor()
    assert(await uploadButton.isDisabled())
    assert.equal(uploads.length, 0)
    await fileInput.evaluate(element => {
      const file = new File(['x'], 'oversized.apk', { type: 'application/vnd.android.package-archive' })
      Object.defineProperty(file, 'size', { value: 200 * 1024 * 1024 + 1 })
      const transfer = new DataTransfer()
      transfer.items.add(file)
      element.files = transfer.files
      element.dispatchEvent(new Event('change', { bubbles: true }))
    })
    await page.getByRole('alert').filter({ hasText: /200/ }).first().waitFor()
    assert(await uploadButton.isDisabled())
    assert.equal(uploads.length, 0)
    await fileInput.setInputFiles({ name: uploaded.name, mimeType: 'application/vnd.android.package-archive', buffer: Buffer.alloc(64) })
    await uploadButton.click()
    await hero.getByRole('link', { name: /Unduh/ }).waitFor()
    await page.waitForFunction(name => document.querySelector('[aria-label="APK terbaru"]').textContent.includes(name), 'v11')
    assert.equal(uploads.length, 1)
    assert.equal(await hero.getByRole('link', { name: /Unduh/ }).getAttribute('href'), base + uploaded.url)
    assert(!(await page.textContent('body')).includes('Makanan'), 'Unknown APK must not inherit v10.18 release notes')
    assert(!(await hero.textContent()).includes('Android 7.0'), 'Unknown APK must not inherit verified v10.18 compatibility')
    assert.equal(await fileInput.inputValue(), '')
    console.log('PASS invalid/oversized files never upload; valid mocked upload refreshes newest APK without stale release claims')

    files = [{ ...latest, size: 64 }]
    await page.goto(url + '?readonly')
    await hero.getByRole('link', { name: /Unduh/ }).waitFor()
    assert(!(await page.textContent('body')).includes('Makanan'), 'A different file with the known name must not inherit release notes')
    assert(!(await hero.textContent()).includes('Paket rilis terverifikasi'))
    assert(!(await hero.textContent()).includes('Android 7.0'))
    console.log('PASS release evidence requires both exact filename and expected size')

    files = [old, bundle, latest]
    await page.goto(url + '?readonly')
    await hero.getByRole('link', { name: /Unduh/ }).waitFor()
    assert.equal(await page.locator('input[type="file"]').count(), 0)
    assert.equal(await page.getByRole('button', { name: /Hapus/ }).count(), 0)
    for (const viewport of [{ width: 390, height: 844 }, { width: 320, height: 568 }]) {
      await page.setViewportSize(viewport)
      assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), `Page overflows at ${viewport.width}px`)
      await hero.getByRole('link', { name: /Unduh/ }).waitFor()
      await format('APK')
      assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth))
      await format('Semua format', 'semua')
    }
    if (process.env.APP_POS_SCREENSHOT) await page.screenshot({ path: process.env.APP_POS_SCREENSHOT, fullPage: true, animations: 'disabled' })
    console.log('PASS read-only permission controls and phone layouts')

    files = []
    await page.goto(url + '?readonly')
    await page.getByText(/Belum ada file aplikasi|Belum ada aplikasi/).first().waitFor()
    assert.equal(await hero.count(), 0)
    assert.equal(await page.getByRole('link', { name: /Unduh/ }).count(), 0)
    listError = true
    await page.reload()
    await page.getByRole('alert').filter({ hasText: 'Layanan sementara terganggu, coba lagi' }).waitFor()
    assert.equal(await hero.count(), 0)
    assert.deepEqual(errors, [])
    console.log('PASS empty/error states and no Vue/runtime errors')
  } finally {
    await browser.close()
  }
})().catch(error => { console.error(error); process.exitCode = 1 })
