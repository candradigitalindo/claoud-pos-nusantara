export function defaultRoleNotificationPolicy(role) {
  return { enabled: true, outlet_mode: 'role', outlet_ids: [], include_global: role?.scope_type !== 'specific', events: {} }
}

export function normalizeRoleNotificationPolicy(policy, role) {
  const defaults = defaultRoleNotificationPolicy(role)
  return {
    enabled: policy?.enabled ?? defaults.enabled,
    outlet_mode: policy?.outlet_mode === 'selected' ? 'selected' : 'role',
    outlet_ids: [...(policy?.outlet_ids || [])],
    include_global: role?.scope_type === 'specific' || policy?.outlet_mode === 'selected' ? false : (policy?.include_global ?? defaults.include_global),
    events: { ...(policy?.events || {}) },
  }
}

export function roleNotificationOutlets(role) {
  return (role?.outlet_ids || []).map((id, index) => ({ id, name: role?.outlet_names?.[index] || id }))
}

export function selectedNotificationOutlets(role, policy) {
  const allowed = roleNotificationOutlets(role)
  return policy?.outlet_mode === 'selected' ? allowed.filter(outlet => (policy.outlet_ids || []).includes(outlet.id)) : allowed
}

export function roleNotificationEventState(event, role, policy, settings) {
  const key = event.key
  const eligible = (role?.eligible_events || []).includes(key)
  const personal = (role?.personal_events || []).includes(key)
  const choice = policy?.events?.[key] || 'auto'
  const automatic = (role?.automatic_events || []).includes(key)
  const stopped = reason => ({ active: false, label: 'Nonaktif', reason, eligible, personal })
  if (!settings?.enabled) return stopped('Pengiriman WhatsApp dimatikan pada pengaturan umum.')
  if (settings?.events?.[key] === false) return stopped('Jenis pesan ini dimatikan untuk seluruh sistem pada pengaturan umum.')
  if (policy?.enabled === false) return stopped('Seluruh notifikasi untuk role ini dimatikan, termasuk pengajuan milik sendiri.')
  if (choice === 'off') return stopped('Dimatikan khusus untuk role ini, termasuk pesan pengajuan milik sendiri.')
  if (!eligible && !personal) return stopped('Hak akses role ini belum sesuai dengan jenis pesan ini.')
  const digest = event.is_digest === true
  if (digest && role?.scope_type !== 'specific' && policy?.outlet_mode === 'role' && !policy?.include_global) {
    return stopped('Rekap gabungan nonaktif. Pilih outlet tertentu untuk rekap terbatas, atau aktifkan pesan global.')
  }
  if (event.scope_kind === 'global' && (role?.scope_type === 'specific' || policy?.outlet_mode === 'selected' || !policy?.include_global)) {
    return stopped('Pesan global memuat data lintas outlet. Cakupan global untuk role ini tidak aktif.')
  }
  const ownCentralUnit = key.startsWith('purchase.') && policy?.outlet_mode === 'role' && (role?.central_work_unit_ids || []).length > 0
  if (!ownCentralUnit && event.scope_kind !== 'global' && (policy?.outlet_mode === 'selected' || role?.scope_type === 'specific') && !selectedNotificationOutlets(role, policy).length) {
    // A mixed event can still reach an all-access role through its global lane.
    if (!(event.scope_kind === 'mixed' && role?.scope_type !== 'specific' && policy?.include_global)) {
      return stopped('Belum ada outlet yang dipilih untuk notifikasi role ini.')
    }
  }
  if (personal && (!eligible || (choice === 'auto' && !automatic))) return {
    active: true, label: 'Pengajuan sendiri', eligible, personal,
    reason: 'Dikirim hanya saat pengguna ini menjadi pengaju dokumen. Pengajuan pengguna lain tidak ikut dikirim.',
  }
  if (choice === 'auto' && !automatic && !personal) {
    return stopped((settings?.event_roles?.[key] || []).length
      ? 'Mengikuti daftar penerima event yang sudah ditetapkan. Role ini belum termasuk; pilih Aktif bila hak aksesnya sesuai.'
      : 'Aturan otomatis belum menempatkan role ini sebagai penerima jenis pesan ini.')
  }
  let reason = choice === 'on' ? 'Diaktifkan khusus untuk role ini, tetap mengikuti hak akses dan cakupan outletnya.' : 'Mengikuti aturan otomatis berdasarkan hak akses dan daftar penerima event.'
  if (event.scope_kind === 'mixed' && !policy?.include_global) reason += ' Bagian global atau rekap lintas outlet tidak dikirim.'
  return { active: true, label: 'Aktif', reason, eligible, personal }
}
