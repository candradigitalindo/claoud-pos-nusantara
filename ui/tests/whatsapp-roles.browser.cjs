// Synthetic requests only: no actual WhatsApp messages or production data.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright')
const assert = require('node:assert/strict')
const base = process.env.WHATSAPP_ROLE_TEST_URL || 'http://127.0.0.1:4189'
;(async () => {
  const browser = await chromium.launch({ args: ['--no-sandbox'] })
  try {
    const page = await browser.newPage({ viewport: { width: 1366, height: 1000 } })
    page.setDefaultTimeout(12000)
    const errors = [], writes = [], resets = [], requests = []
    const events = [
      { key: 'sales.shift', label: 'Tutup kasir', group: 'Penjualan', scope_kind: 'outlet' },
      { key: 'sales.daily', label: 'Rekap harian gabungan', group: 'Penjualan', scope_kind: 'mixed', is_digest: true },
      { key: 'purchase.approved', label: 'Pengajuan disetujui', group: 'Pengadaan', scope_kind: 'mixed' },
      { key: 'purchase.payment_requested', label: 'Permintaan pembayaran', group: 'Pengadaan', scope_kind: 'mixed' },
      { key: 'stock.low', label: 'Stok minimum', group: 'Gudang', scope_kind: 'mixed' },
    ].map(event => ({ ...event, audience: 'internal', description: `Penjelasan ${event.label}.`, scope_description: 'Mengikuti outlet atau unit kerja yang terkait.' }))
    const allKeys = events.map(event => event.key)
    const roles = [
      { name: 'kasir', description: 'Petugas kasir outlet', scope_type: 'specific', users: 2, outlet_ids: ['o1'], outlet_names: ['Outlet Sendiri'], eligible_events: ['sales.shift'], automatic_events: ['sales.shift'], personal_events: [] },
      { name: 'purchasing', description: 'Pembelian dan pengajuan sendiri', scope_type: 'specific', users: 1, outlet_ids: ['o1'], outlet_names: ['Outlet Sendiri'], eligible_events: ['purchase.payment_requested'], automatic_events: [], personal_events: ['purchase.approved'] },
      { name: 'admin', description: 'Role sistem yang dilindungi', is_system: true, scope_type: 'all', users: 1, outlet_ids: ['o1', 'o2'], outlet_names: ['Outlet Sendiri', 'Outlet Lain'], eligible_events: allKeys, automatic_events: allKeys, personal_events: [] },
      { name: 'kepala_pusat', description: 'Kepala unit kerja pusat', scope_type: 'specific', users: 1, outlet_ids: [], outlet_names: [], central_work_unit_ids: ['wu0'], central_work_unit_names: ['Unit Administrasi Pusat'], eligible_events: [], automatic_events: [], personal_events: ['purchase.approved'] },
      { name: 'superadmin', description: 'Pemilik sistem', scope_type: 'all', users: 1, outlet_ids: ['o1', 'o2'], outlet_names: ['Outlet Sendiri', 'Outlet Lain'], eligible_events: allKeys, automatic_events: allKeys, personal_events: [] },
    ]
    const policy = role => ({ enabled: true, outlet_mode: 'role', outlet_ids: [], include_global: role.scope_type === 'all', events: {} })
    const settings = { enabled: true, events: Object.fromEntries(allKeys.map(key => [key, true])), event_roles: {}, role_notifications: { purchasing: { ...policy(roles[1]), events: { 'stock.low': 'on' } } } }
    const metadata = () => roles.map(role => ({ ...role, policy: structuredClone(settings.role_notifications[role.name] || policy(role)), has_policy: Object.hasOwn(settings.role_notifications, role.name) }))
    page.on('pageerror', error => errors.push(error.message))
    page.on('console', message => { if (message.text().includes('[Vue warn]')) errors.push(message.text()) })
    await page.route('**/api/v1/**', async route => {
      const request = route.request(), url = new URL(request.url()), path = url.pathname
      requests.push(`${request.method()} ${path}`)
      const reply = data => route.fulfill({ json: { success: true, data } })
      if (path.endsWith('/admin/roles/notification-options')) return reply({ settings, roles: metadata(), events })
      if (/\/admin\/roles\/[^/]+\/notifications$/.test(path)) {
        const role = decodeURIComponent(path.split('/').at(-2))
        if (request.method() === 'PUT') { const data = request.postDataJSON(); writes.push({ role, data }); assert(!Object.hasOwn(data, 'templates')); settings.role_notifications[role] = data }
        else if (request.method() === 'DELETE') { resets.push(role); delete settings.role_notifications[role] }
        else throw new Error('Unexpected role notification method')
        return reply({ settings, roles: metadata(), events })
      }
      if (path.endsWith('/admin/roles')) return reply({ roles: roles.filter(role => role.name !== 'superadmin').map(role => ({ ...role, redirect_to: '/' })), permissions: {}, scopes: {}, all_permissions: [] })
      if (path.endsWith('/work-units')) return reply([])
      if (path.endsWith('/whatsapp/status')) return reply({ enabled: settings.enabled, gateway: { configured: true, reachable: true, state: 'connected', connected: true, logged_in: true, jid: '62811111', events: [] }, queue: {} })
      if (path.endsWith('/whatsapp/settings')) return reply({ settings: { ...settings, templates: {}, gap_min_sec: 10, gap_max_sec: 20, quiet_start: '', quiet_end: '' }, events: events.map(event => ({ ...event, placeholders: [], default_template: '', template: '' })) })
      if (path.endsWith('/whatsapp/recipients') || path.endsWith('/whatsapp/recipients/users') || path.endsWith('/outlets')) return reply([])
      errors.push(`Unexpected API ${request.method()} ${path}`); return route.abort()
    })
    const card = role => page.locator(`[data-role="${role}"]`)
    const toggle = (role, label) => card(role).getByRole('switch', { name: `Notifikasi ${label}`, exact: true })
    async function open(role) { await page.getByRole('button', { name: `Notifikasi role ${role}`, exact: true }).click(); await card(role).getByRole('tab', { name: 'Notifikasi', exact: true }).waitFor() }
    async function save(role) {
      const response = page.waitForResponse(response => response.url().endsWith(`/roles/${role}/notifications`) && response.request().method() === 'PUT')
      await card(role).getByRole('button', { name: 'Simpan notifikasi role', exact: true }).click(); await response
      await card(role).getByText('Perubahan role ini belum disimpan.', { exact: true }).waitFor({ state: 'hidden' })
    }
    await page.goto(base + '/tests/fixtures/whatsapp-roles.html')
    await toggle('kasir', 'Tutup kasir').waitFor()
    assert.equal(await card('kasir').locator('.wa-role-event').count(), 1)
    assert.equal(await card('kasir').getByRole('combobox', { name: 'Pilih role notifikasi' }).count(), 0)
    assert.equal(await card('kasir').locator('[data-event^="purchase."]').count(), 0)
    assert.equal(await page.getByRole('link', { name: 'Log Pesan', exact: true }).count(), 0, 'Role-only admin needs no WhatsApp rights')
    assert(await toggle('kasir', 'Tutup kasir').isChecked())
    await toggle('kasir', 'Tutup kasir').uncheck(); await save('kasir')
    assert.equal(writes.at(-1).data.events['sales.shift'], 'off')
    await card('kasir').getByRole('checkbox', { name: 'Aktifkan notifikasi role', exact: true }).uncheck()
    await toggle('kasir', 'Tutup kasir').check(); await save('kasir')
    assert.equal(writes.at(-1).data.enabled, false); assert.equal(writes.at(-1).data.events['sales.shift'], 'on')
    assert(await toggle('kasir', 'Tutup kasir').isChecked(), 'Preference remains checked while role master is off')
    await card('kasir').getByRole('checkbox', { name: 'Aktifkan notifikasi role', exact: true }).check(); await save('kasir')
    console.log('PASS notifications embedded in role, relevant-only list, direct on/off and role-only authorization')

    await toggle('kasir', 'Tutup kasir').uncheck()
    await open('purchasing')
    assert.equal(await card('purchasing').locator('.wa-role-event').count(), 3, 'Eligible, personal and stale overrides retained')
    await toggle('purchasing', 'Stok minimum').uncheck()
    assert(await toggle('purchasing', 'Stok minimum').isDisabled(), 'Removed permission cannot be switched back on')
    await card('purchasing').getByRole('button', { name: 'Ikuti bawaan Stok minimum', exact: true }).click()
    assert.equal(await card('purchasing').locator('[data-event="stock.low"]').count(), 0)
    await toggle('purchasing', 'Pengajuan disetujui').uncheck(); await save('purchasing')
    assert.equal(writes.at(-1).data.events['purchase.approved'], 'off')
    await toggle('purchasing', 'Pengajuan disetujui').check()
    await toggle('purchasing', 'Permintaan pembayaran').check(); await save('purchasing')
    assert(!Object.hasOwn(writes.at(-1).data.events, 'purchase.approved'), 'Personal on uses inherited auto, never broad on')
    assert.equal(writes.at(-1).data.events['purchase.payment_requested'], 'on')
    await open('kasir'); assert(!(await toggle('kasir', 'Tutup kasir').isChecked()), 'Draft survives closing role')
    await card('kasir').getByRole('button', { name: 'Ikuti bawaan Tutup kasir', exact: true }).click(); await save('kasir')
    console.log('PASS personal messages on/off, stale preference cleanup and drafts preserved across roles')

    await open('admin')
    const adminMaster = card('admin').getByRole('checkbox', { name: 'Aktifkan notifikasi role', exact: true })
    assert(await adminMaster.isEnabled())
    await toggle('admin', 'Stok minimum').uncheck(); await save('admin')
    await open('superadmin')
    assert.equal(await card('superadmin').getByRole('button', { name: 'Edit Role', exact: true }).count(), 0)
    assert.equal(await card('superadmin').getByRole('button', { name: 'Duplikat Role', exact: true }).count(), 0)
    assert.equal(await card('superadmin').getByRole('tab', { name: 'Hak Akses', exact: true }).count(), 0)
    await card('superadmin').getByRole('radio', { name: /Pilih outlet tertentu/ }).check()
    await card('superadmin').getByRole('checkbox', { name: 'Outlet Lain', exact: true }).check()
    const global = card('superadmin').getByRole('checkbox', { name: 'Terima pesan global lintas outlet', exact: true })
    assert(await global.isDisabled()); assert(!(await global.isChecked()))
    await save('superadmin'); assert.equal(writes.at(-1).data.include_global, false)
    await open('kepala_pusat')
    await card('kepala_pusat').getByText('Unit kerja pusat: Unit Administrasi Pusat', { exact: true }).first().waitFor()
    assert(await toggle('kepala_pusat', 'Pengajuan disetujui').isChecked())
    await card('kepala_pusat').locator('[data-event="purchase.approved"]').getByText('Pengajuan sendiri:', { exact: true }).waitFor()
    console.log('PASS protected system/superadmin notifications configurable, outlet/global and central-unit protections remain')

    settings.enabled = false
    await page.goto(base + '/tests/fixtures/whatsapp-roles.html?role=kasir')
    await toggle('kasir', 'Tutup kasir').waitFor()
    assert(await toggle('kasir', 'Tutup kasir').isChecked())
    await toggle('kasir', 'Tutup kasir').uncheck(); await save('kasir')
    await toggle('kasir', 'Tutup kasir').check(); await save('kasir')
    assert(await toggle('kasir', 'Tutup kasir').isChecked())
    await card('kasir').locator('[data-event="sales.shift"]').getByText('Pengiriman WhatsApp dimatikan', { exact: false }).waitFor()
    assert.equal(writes.at(-1).data.events['sales.shift'], 'on')
    const reset = page.waitForResponse(response => response.request().method() === 'DELETE' && response.url().endsWith('/roles/kasir/notifications'))
    await card('kasir').getByRole('button', { name: 'Hapus aturan khusus', exact: true }).click(); await reset
    await card('kasir').getByRole('button', { name: 'Hapus aturan khusus', exact: true }).waitFor({ state: 'hidden' })
    assert.equal(resets.at(-1), 'kasir')
    console.log('PASS subscription switches remain usable while global delivery disabled; reset restores inheritance')

    await page.setViewportSize({ width: 390, height: 844 })
    await page.screenshot({ path: '/artifacts/roles-inline-mobile.png', fullPage: true })
    assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1))
    await page.goto(base + '/tests/fixtures/whatsapp-roles.html?readonly=1&role=kasir')
    await toggle('kasir', 'Tutup kasir').waitFor(); assert(await toggle('kasir', 'Tutup kasir').isDisabled())
    assert.equal(await card('kasir').getByRole('button', { name: 'Simpan notifikasi role', exact: true }).count(), 0)
    const beforeLegacy = requests.filter(request => request.includes('/whatsapp/')).length
    await page.goto(base + '/tests/fixtures/whatsapp-roles.html?legacy=1&role=purchasing')
    await toggle('purchasing', 'Permintaan pembayaran').waitFor()
    assert((await page.evaluate(() => notificationTestRoute())).startsWith('/roles?'))
    assert.equal(requests.filter(request => request.includes('/whatsapp/')).length, beforeLegacy, 'Legacy deep link does not require WhatsApp APIs')
    await page.goto(base + '/tests/fixtures/whatsapp-roles.html?whatsapp=1')
    await page.getByRole('button', { name: 'Koneksi', exact: true }).waitFor()
    assert.equal(await page.getByRole('button', { name: 'Notifikasi per Role', exact: true }).count(), 0)
    await page.getByRole('link', { name: 'Notifikasi setiap role →', exact: true }).waitFor()
    assert.deepEqual(errors, [])
    console.log('PASS mobile, read-only, legacy redirect, and WhatsApp page contains no duplicate role editor')
  } finally { await browser.close() }
})().catch(error => { console.error(error); process.exitCode = 1 })
