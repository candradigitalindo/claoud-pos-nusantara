// Run against the Vite dev server; requests are limited to the isolated fixture.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright')
const assert = require('node:assert/strict')
const base = process.env.ACTION_TEST_URL || 'http://127.0.0.1:4189'

;(async () => {
  const browser = await chromium.launch({ args: ['--no-sandbox'] })
  try {
    const page = await browser.newPage({ viewport: { width: 1200, height: 1000 } })
    const errors = []
    page.on('pageerror', error => errors.push(error.message))
    page.on('console', message => { if (message.text().includes('[Vue warn]')) errors.push(message.text()) })
    await page.goto(`${base}/tests/fixtures/action-buttons.html`)
    await page.getByRole('button', { name: 'Edit produk', exact: true }).waitFor()
    const state = async () => JSON.parse(await page.locator('#state').textContent())
    const icons = await page.locator('.action-button').evaluateAll(buttons => buttons.map(button => {
      const style = getComputedStyle(button)
      return { text: button.textContent.trim(), label: button.getAttribute('aria-label'), title: button.title,
        paths: button.querySelectorAll('svg path').length, border: style.borderTopWidth,
        width: button.getBoundingClientRect().width, height: button.getBoundingClientRect().height }
    }))
    assert(icons.length > 40)
    for (const icon of icons) {
      assert.equal(icon.text, '')
      assert(icon.label && icon.title === icon.label)
      assert(icon.paths > 0)
      assert.equal(icon.border, '1px')
      assert.equal(icon.width, icon.height)
      assert.equal(icon.width, 36)
    }
    console.log('PASS SVG, border, ukuran seragam, tooltip, dan nama aksesibel tanpa teks visual')
    await page.getByRole('button', { name: 'Edit produk', exact: true }).focus()
    await page.keyboard.press('Enter')
    assert.equal((await state()).edits, 1)
    assert.equal((await state()).rowClicks, 0)
    assert.equal((await state()).submits, 0)
    for (const name of ['Hapus produk', 'Setujui pengajuan']) {
      const button = page.getByRole('button', { name, exact: true })
      assert(await button.isDisabled())
      await button.dispatchEvent('click')
    }
    assert.equal((await state()).deletes, 0)
    assert.equal((await state()).approvals, 0)
    assert.equal(await page.getByRole('button', { name: 'Setujui pengajuan' }).getAttribute('aria-busy'), 'true')
    await page.getByRole('button', { name: 'Simpan', exact: true }).click()
    assert.equal((await state()).submits, 1)
    assert.equal((await page.getByRole('button', { name: 'Batal', exact: true }).textContent()).trim(), 'Batal')
    console.log('PASS keyboard, stopPropagation, disabled/loading, serta tombol form tetap berteks')
    const downloadPromise = page.waitForEvent('download')
    await page.getByRole('link', { name: 'Unduh bukti', exact: true }).click()
    assert.equal((await downloadPromise).suggestedFilename(), 'bukti.txt')
    await page.getByRole('link', { name: 'Buka detail', exact: true }).click()
    await page.waitForFunction(() => new URL(location.href).searchParams.get('detail') === '1')
    const previous = page.url()
    await page.getByRole('link', { name: 'Akses ditolak', exact: true }).dispatchEvent('click')
    await page.getByRole('link', { name: 'Unduhan nonaktif', exact: true }).dispatchEvent('click')
    assert.equal(page.url(), previous)
    console.log('PASS unduhan, navigasi internal, dan tautan yang dinonaktifkan')
    if (process.env.ACTION_SCREENSHOT) await page.screenshot({ path: process.env.ACTION_SCREENSHOT, fullPage: true })
    const mobile = await browser.newPage({ viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true })
    await mobile.goto(`${base}/tests/fixtures/action-buttons.html`)
    const button = mobile.getByRole('button', { name: 'Edit produk', exact: true })
    await button.waitFor()
    assert.equal((await button.boundingBox()).width, 44)
    assert(await mobile.evaluate(() => document.documentElement.scrollWidth <= innerWidth))
    assert.deepEqual(errors, [])
    console.log('PASS target sentuh ponsel dan tanpa galat Vue')
  } finally { await browser.close() }
})().catch(error => { console.error(error); process.exitCode = 1 })
