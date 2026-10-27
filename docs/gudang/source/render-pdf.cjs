const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const fs = require('fs'), path = require('path'), {pathToFileURL} = require('url');
const out = process.env.DOC_OUTPUT || path.resolve(__dirname, '..');
(async () => {
  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage({ viewport: { width: 1000, height: 1200 } });
    await page.goto(pathToFileURL(path.join(out, 'panduan-operasional-gudang.html')).href);
    await page.evaluate(() => document.fonts.ready);
    await page.waitForFunction(() => [...document.images].every(i => i.complete && i.naturalWidth));
    const overflow = await page.locator('.page').evaluateAll(pages => pages.map((p, i) => ({page: i + 1, gap: p.querySelector('.footer').getBoundingClientRect().top - p.querySelector('.content').getBoundingClientRect().bottom})).filter(p => p.gap < 6));
    if (overflow.length) throw new Error('Isi melewati batas halaman: ' + JSON.stringify(overflow));
    await page.pdf({ path: path.join(out, 'Panduan-Operasional-Gudang-Cloud-POS.pdf'), printBackground: true, preferCSSPageSize: true, tagged: true, outline: true });
    console.log('PDF selesai dibuat.');
  } finally { await browser.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
