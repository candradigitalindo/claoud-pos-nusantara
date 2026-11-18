// Start Vite, then: RAB_TEST_URL=http://127.0.0.1:4195 node tests/project-rab-editor.browser.cjs
// Requires Playwright with Chromium. The fixture never connects to an API/database.
//
// Regression: the editor echoed its own v-model update back as "new" data (the
// parent's ref() returns a reactive proxy), rebuilt every row and emitted again
// without end. Production builds have no recursion guard, so the browser tab
// froze and crashed as soon as one character was typed into the RAB editor.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright')
const assert = require('node:assert/strict')
const base = process.env.RAB_TEST_URL || 'http://127.0.0.1:4195'

;(async () => {
  const browser = await chromium.launch({ args: ['--no-sandbox'] })
  try {
    const page = await browser.newPage({ viewport: { width: 1280, height: 900 } })
    page.setDefaultTimeout(10000)
    const errors = []
    page.on('pageerror', error => errors.push(error.message))
    page.on('console', message => {
      if (message.text().includes('[Vue warn]')) errors.push(message.text())
    })
    await page.goto(`${base}/tests/fixtures/project-rab-editor.html`)
    const state = async () => JSON.parse(await page.locator('#state').textContent())
    const names = page.locator('input[placeholder^="mis. Pasang keramik"]:visible')

    await names.first().click()
    await page.keyboard.press('End')
    await page.keyboard.type(' area', { delay: 30 })
    assert.equal(await page.evaluate(() => document.activeElement?.value), 'Pembersihan lokasi area')
    const typed = await state()
    assert.equal(typed.draft[0].name, 'Pembersihan lokasi area')
    assert.ok(typed.updates >= 1 && typed.updates <= 5, `satu pembaruan per ketukan, didapat ${typed.updates}`)
    assert.deepEqual(errors, [])
    console.log('PASS mengetik tidak memicu loop pembaruan dan fokus tetap di input')

    await page.locator('input[type=number]:visible').first().fill('3')
    const price = page.locator('input[inputmode=decimal]:visible').first()
    await price.fill('')
    await price.pressSequentially('125000', { delay: 20 })
    const edited = await state()
    assert.equal(edited.draft[0].qty, 3)
    assert.equal(edited.draft[0].unit_price, 125000)
    assert.equal(edited.draft[1].id, 'r2')
    assert.match(await page.getByText(/^Total RAB:/).innerText(), /Rp\s?1\.875\.000/)
    console.log('PASS volume dan harga satuan sampai ke induk, total ikut berubah')

    await page.getByRole('button', { name: 'Ganti dari induk', exact: true }).click()
    assert.equal(await names.count(), 1)
    assert.equal(await names.first().inputValue(), 'Rangka baja ringan')
    assert.equal((await state()).draft[0].id, 'r9')
    assert.deepEqual(errors, [])
    console.log('PASS perubahan nyata dari induk tetap membangun ulang editor')
  } finally {
    await browser.close()
  }
})().catch(error => {
  console.error(error)
  process.exit(1)
})
