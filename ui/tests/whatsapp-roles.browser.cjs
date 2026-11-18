// Every request is intercepted; this suite never sends WhatsApp messages.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright')
const assert = require('node:assert/strict')
const base = process.env.WHATSAPP_ROLE_TEST_URL || 'http://127.0.0.1:4189'
;(async () => {
  const browser = await chromium.launch({ args: ['--no-sandbox'] })
  try {
    const page = await browser.newPage({ viewport: { width: 1366, height: 1000 } })
    page.setDefaultTimeout(12000)
    page.on('console', message => { if (message.type() === 'error') console.log('BROWSER', message.text()) })
    const errors = [], writes = [], resets = []
    const allOutlets = [{ id: 'o1', name: 'Outlet Sendiri' }, { id: 'o2', name: 'Outlet Lain' }]
    const event = (key, label, group, scope_kind, audience = 'internal') => ({ key, label, group, audience, scope_kind, is_digest: key === 'sales.daily', scope_description: scope_kind === 'global' ? 'Ringkasan yang memuat data seluruh outlet.' : 'Kejadian dari outlet yang terkait dokumen.', description: `Penjelasan ${label}.`, route_desc: 'Penerima sesuai hak akses.', placeholders: [], default_template: 'Contoh pesan', template: 'Contoh pesan', enabled: true })
    const events = [event('sales.shift_closed', 'Tutup kasir', 'Penjualan', 'outlet'), event('sales.daily', 'Rekap harian gabungan', 'Penjualan', 'mixed'), event('purchase.approved', 'Pengajuan disetujui', 'Pengadaan', 'mixed'), event('purchase.payment_requested', 'Permintaan pembayaran', 'Pengadaan', 'mixed'), event('stock.low', 'Stok minimum', 'Gudang', 'mixed'), event('customer.confirmed', 'Konfirmasi pelanggan', 'Pelanggan', 'outlet', 'customer')]
    const settings = { enabled: true, gap_min_sec: 10, gap_max_sec: 20, daily_limit: 200, quiet_start: '21:00', quiet_end: '07:00', report_hour: 20, reminder_hour: 9, device_offline_min: 10, public_base_url: 'https://example.test', templates: {}, events: Object.fromEntries(events.map(event => [event.key, event.key !== 'stock.low'])), event_roles: { 'purchase.payment_requested': ['keuangan'] }, role_notifications: {} }
    const defaultPolicy = role => ({ enabled: true, outlet_mode: 'role', outlet_ids: [], include_global: role.scope_type === 'all', events: {} })
    const roles = [{ name: 'manager_outlet', description: 'Manajer outlet sendiri', scope_type: 'specific', users: 2, requester_only: true, outlet_ids: ['o1'], outlet_names: ['Outlet Sendiri'], eligible_events: ['sales.shift_closed', 'sales.daily', 'stock.low'], automatic_events: ['sales.shift_closed', 'sales.daily'], personal_events: ['purchase.approved'] },
      { name: 'manajer_operasional', description: 'Pantau seluruh outlet', scope_type: 'all', users: 1, requester_only: false, outlet_ids: ['o1', 'o2'], outlet_names: ['Outlet Sendiri', 'Outlet Lain'], eligible_events: events.filter(event => event.audience === 'internal').map(event => event.key), automatic_events: ['sales.shift_closed', 'sales.daily', 'purchase.approved'], personal_events: ['purchase.approved'] },
      { name: 'kepala_pusat', description: 'Kepala unit kerja pusat', scope_type: 'specific', users: 1, requester_only: true, outlet_ids: [], outlet_names: [], central_work_unit_ids: ['wu0'], central_work_unit_names: ['Unit Administrasi Pusat'], eligible_events: [], automatic_events: [], personal_events: ['purchase.approved'] }]
    const roleResponse = () => roles.map(role => ({ ...role, policy: structuredClone(settings.role_notifications[role.name] || defaultPolicy(role)), has_policy: Object.hasOwn(settings.role_notifications, role.name) }))
    page.on('pageerror', error => errors.push(error.message))
    page.on('console', message => { if (message.text().includes('[Vue warn]')) errors.push(message.text()) })
    await page.route('**/api/v1/**', async route => {
      const request = route.request(), url = new URL(request.url()), path = url.pathname
      const reply = data => route.fulfill({ json: { success: true, data } })
      if (path.endsWith('/whatsapp/status')) return reply({ enabled: settings.enabled, gateway: { configured: true, reachable: true, state: 'connected', connected: true, logged_in: true, jid: '62811111', push_name: 'Nomor Uji', events: [] }, queue: { pending: 0, sent_today: 0, failed: 0, daily_limit: 200 } })
      if (path.endsWith('/whatsapp/settings')) {
        if (request.method() === 'PUT') { const data = request.postDataJSON(); writes.push({ general: true, data }); assert(!Object.hasOwn(data, 'role_notifications')); Object.assign(settings, data) }
        return reply({ settings, events })
      }
      if (path.endsWith('/whatsapp/roles')) return reply(roleResponse())
      if (/\/whatsapp\/roles\/[^/]+\/notifications$/.test(path)) {
        const roleName = decodeURIComponent(path.split('/').at(-2))
        if (request.method() === 'PUT') { const data = request.postDataJSON(); writes.push({ role: roleName, data }); assert(!Object.hasOwn(data, 'gap_min_sec')); settings.role_notifications[roleName] = data }
        else if (request.method() === 'DELETE') { resets.push(roleName); delete settings.role_notifications[roleName] }
        else errors.push(`Unexpected method ${request.method()}`)
        return reply({ settings, roles: roleResponse() })
      }
      if (path.endsWith('/whatsapp/recipients/users')) return reply(roles.map(role => {
        const policy = settings.role_notifications[role.name] || defaultPolicy(role)
        const names = policy.outlet_mode === 'selected' ? allOutlets.filter(outlet => policy.outlet_ids.includes(outlet.id)).map(outlet => outlet.name) : role.outlet_names
        return { id: role.name, name: `User ${role.name}`, username: role.name, role: role.name, wa_phone: '628123456', wa_notify: true, is_active: true, scope_type: role.scope_type, outlet_names: role.outlet_names, events: [], role_notifications_enabled: policy.enabled, notification_scope_type: policy.enabled ? (policy.outlet_mode === 'selected' || role.scope_type === 'specific' ? 'specific' : 'all') : 'none', notification_outlet_names: names, notification_work_unit_names: policy.outlet_mode === 'role' ? (role.central_work_unit_names || []) : [], include_global: policy.include_global }
      }))
      if (path.endsWith('/whatsapp/recipients')) return reply([])
      if (path.endsWith('/outlets')) return reply(allOutlets)
      if (path.endsWith('/admin/roles')) return reply({ roles: roles.map(role => ({ ...role, redirect_to: '/', is_system: false })), permissions: {}, scopes: {}, all_permissions: [] })
      if (path.endsWith('/work-units')) return reply([])
      errors.push(`Unexpected API ${request.method()} ${path}`); return route.abort()
    })
    const picker = () => page.getByRole('combobox', { name: 'Pilih role notifikasi', exact: true })
    async function openPicker(locator) {
      await locator.evaluate(element => element.scrollIntoView({ block: 'center', behavior: 'instant' }))
      await page.waitForTimeout(120)
      await locator.click()
    }
    async function chooseRole(name) {
      await openPicker(picker()); await page.getByRole('combobox', { name: 'Cari pilihan', exact: true }).fill(name)
      await page.getByRole('option').click()
    }
    async function chooseEvent(label, option) {
      await openPicker(page.getByRole('combobox', { name: `Aturan pesan ${label}`, exact: true }))
      await page.getByRole('option', { name: option, exact: true }).click()
    }
    async function saveRole() {
      const response = page.waitForResponse(response => response.url().endsWith('/notifications') && response.request().method() === 'PUT')
      await page.getByRole('button', { name: 'Simpan notifikasi role', exact: true }).click()
      await response
      await page.getByText('Perubahan role ini belum disimpan.', { exact: true }).waitFor({ state: 'hidden' })
    }
    await page.goto(base + '/tests/fixtures/whatsapp-roles.html')
    await picker().waitFor()
    await page.getByText('2 akun aktif siap menerima', { exact: true }).waitFor()
    await page.getByText('Pesan global tidak tersedia untuk role ini.', { exact: false }).waitFor()
    assert.equal(await page.getByRole('checkbox', { name: 'Terima pesan global lintas outlet', exact: true }).count(), 0)
    await page.getByRole('radio', { name: /Pilih outlet tertentu/ }).check()
    await page.getByRole('checkbox', { name: 'Outlet Sendiri', exact: true }).check()
    assert.equal(await page.getByRole('checkbox', { name: 'Outlet Lain', exact: true }).count(), 0, 'Other outlets are never offered to scoped role')
    await page.locator('[data-event="purchase.approved"]').getByText('Pengajuan sendiri:', { exact: true }).waitFor()
    await openPicker(page.getByRole('combobox', { name: 'Aturan pesan Pengajuan disetujui', exact: true }))
    assert.equal(await page.getByRole('option', { name: 'Aktif', exact: true }).getAttribute('aria-disabled'), 'true')
    await page.getByRole('option', { name: 'Nonaktif', exact: true }).click()
    console.log('PASS own-outlet controls exclude other outlets/global data; personal procurement supports auto/off only')

    await chooseRole('manajer_operasional')
    await page.getByRole('checkbox', { name: 'Terima pesan global lintas outlet', exact: true }).waitFor()
    await page.getByText('Perubahan belum disimpan untuk:', { exact: false }).waitFor()
    await page.getByRole('checkbox', { name: 'Terima pesan global lintas outlet', exact: true }).uncheck()
    await page.locator('[data-event="sales.daily"]').getByText('Rekap gabungan nonaktif.', { exact: false }).waitFor()
    await page.getByRole('radio', { name: /Pilih outlet tertentu/ }).check()
    const globalToggle = page.getByRole('checkbox', { name: 'Terima pesan global lintas outlet', exact: true })
    assert(await globalToggle.isDisabled()); assert(!(await globalToggle.isChecked()))
    await page.getByRole('checkbox', { name: 'Outlet Lain', exact: true }).first().check()
    await chooseEvent('Permintaan pembayaran', 'Aktif')
    await saveRole()
    await page.getByRole('button', { name: 'Simpan notifikasi role', exact: true }).waitFor()
    assert.equal(writes.at(-1).role, 'manajer_operasional')
    assert.deepEqual(writes.at(-1).data.outlet_ids, ['o2'])
    assert.equal(writes.at(-1).data.include_global, false)
    assert.equal(writes.at(-1).data.events['purchase.payment_requested'], 'on')
    console.log('PASS all-access role can narrow outlets; narrowing clears global; role save uses isolated endpoint')

    await chooseRole('manager_outlet')
    assert.equal(await page.getByRole('combobox', { name: 'Aturan pesan Pengajuan disetujui', exact: true }).textContent().then(value => value.trim()), 'Nonaktif')
    await saveRole()
    await page.getByRole('checkbox', { name: 'Aktifkan notifikasi role', exact: true }).uncheck()
    await page.getByText('Semua pesan untuk role ini nonaktif, termasuk pengajuan pribadi.', { exact: true }).waitFor()
    await saveRole()
    await page.waitForTimeout(100)
    assert.equal(settings.role_notifications.manager_outlet.enabled, false)
    await page.getByRole('button', { name: 'Penerima', exact: true }).click()
    const auditOps = page.getByRole('row').filter({ hasText: 'User manajer_operasional' })
    await auditOps.getByText('Outlet Lain', { exact: true }).waitFor()
    await auditOps.getByText('Hak akses: semua outlet', { exact: true }).waitFor()
    await auditOps.getByText('Tanpa pesan global / lintas outlet', { exact: true }).waitFor()
    await page.getByRole('row').filter({ hasText: 'User manager_outlet' }).getByText('Notifikasi role nonaktif', { exact: true }).waitFor()
    await page.getByRole('button', { name: 'Notifikasi per Role', exact: true }).click()
    await page.getByRole('button', { name: 'Hapus aturan khusus', exact: true }).click()
    await page.waitForTimeout(100)
    assert.equal(resets.at(-1), 'manager_outlet')
    assert(!Object.hasOwn(settings.role_notifications, 'manager_outlet'))
    assert(await page.getByRole('checkbox', { name: 'Aktifkan notifikasi role', exact: true }).isChecked())
    console.log('PASS unsaved drafts survive role changes; off mutes personal messages; reset restores defaults')
    await chooseRole('kepala_pusat')
    await page.getByText('Unit kerja pusat: Unit Administrasi Pusat', { exact: true }).first().waitFor()
    await page.locator('[data-event="purchase.approved"]').getByText('Pengajuan sendiri:', { exact: true }).waitFor()
    await page.getByRole('radio', { name: /Pilih outlet tertentu/ }).check()
    await page.locator('[data-event="purchase.approved"]').getByText('Belum ada outlet yang dipilih', { exact: false }).waitFor()
    await page.getByRole('radio', { name: /Sesuai cakupan role/ }).check()
    await page.locator('[data-event="purchase.approved"]').getByText('Pengajuan sendiri:', { exact: true }).waitFor()
    await chooseRole('manager_outlet')
    console.log('PASS assigned central work unit receives own documents with global off; selected-outlet mode excludes central unit')

    await page.getByRole('button', { name: 'Pengaturan Umum & Template', exact: true }).click()
    const legacyEvent = page.locator('[data-template-event="purchase.payment_requested"]')
    await legacyEvent.getByRole('button', { name: 'Ubah template', exact: true }).click()
    await legacyEvent.getByRole('button', { name: 'Gunakan penerima otomatis', exact: true }).click()
    assert.deepEqual(settings.event_roles['purchase.payment_requested'], ['keuangan'], 'Legacy reset remains a draft until general save')
    await page.getByRole('checkbox', { name: 'Pengiriman aktif', exact: true }).uncheck()
    await page.getByRole('button', { name: 'Notifikasi per Role', exact: true }).click()
    assert.equal(await page.getByText('Pengiriman WhatsApp umum sedang nonaktif.', { exact: false }).count(), 0, 'Unsaved general draft does not change effective role preview')
    await page.getByRole('button', { name: 'Pengaturan Umum & Template', exact: true }).click()
    await page.getByRole('button', { name: 'Simpan pengaturan & template', exact: true }).click()
    await page.waitForTimeout(100)
    assert(writes.at(-1).general)
    assert(!Object.hasOwn(writes.at(-1).data, 'role_notifications'))
    assert.deepEqual(writes.at(-1).data.event_roles['purchase.payment_requested'], [])
    await page.getByRole('button', { name: 'Notifikasi per Role', exact: true }).click()
    await page.getByText('Pengiriman WhatsApp umum sedang nonaktif.', { exact: false }).waitFor()
    await page.getByRole('searchbox', { name: 'Cari jenis pesan role', exact: true }).fill('kasir')
    assert.equal(await page.locator('.wa-role-event').count(), 1)
    await page.getByRole('searchbox', { name: 'Cari jenis pesan role', exact: true }).fill('')
    assert.equal(await page.locator('.wa-role-event').count(), 5, 'Customer events are separate')
    console.log('PASS global settings omit role policy; global-disabled explanation and event search are visible')

    await page.setViewportSize({ width: 390, height: 844 })
    await page.screenshot({ path: '/artifacts/whatsapp-role-mobile.png', fullPage: true })
    assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1), 'Mobile page fits viewport')
    await page.goto(base + '/tests/fixtures/whatsapp-roles.html?readonly=1')
    await picker().waitFor()
    assert(await page.getByRole('checkbox', { name: 'Aktifkan notifikasi role', exact: true }).isDisabled())
    assert.equal(await page.getByRole('button', { name: 'Simpan notifikasi role', exact: true }).count(), 0)
    await page.goto(base + '/tests/fixtures/whatsapp-roles.html?roles=1')
    const link = page.getByRole('link', { name: 'Notifikasi WhatsApp role manager_outlet', exact: true })
    await link.waitFor()
    assert((await link.getAttribute('href')).includes('/whatsapp?tab=role&role=manager_outlet'))
    await link.click(); await picker().waitFor()
    assert((await picker().textContent()).includes('manager_outlet'))
    assert.deepEqual(errors, [])
    console.log('PASS mobile layout, read-only access, and Roles-to-notification deep link')
  } finally { await browser.close() }
})().catch(error => { console.error(error); process.exitCode = 1 })
