<template>
  <div class="space-y-4 sm:space-y-5">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div class="min-w-0">
        <h1 class="text-lg sm:text-xl font-bold text-gray-900">WhatsApp</h1>
        <p class="mt-0.5 text-sm text-gray-500">
          Nomor WhatsApp biasa dipasangkan lewat QR (seperti WhatsApp Web) dan dipakai mengirim
          notifikasi otomatis, pesan ke pelanggan, dan broadcast — gratis, dijalankan di server sendiri.
        </p>
      </div>
      <RouterLink to="/whatsapp/messages" class="text-xs font-semibold text-emerald-700 hover:underline">Lihat log pesan →</RouterLink>
    </div>

    <AppAlert type="error" :message="errorMsg" />
    <AppAlert v-if="status && !status.gateway.configured" type="warning"
      message="Gateway belum dikonfigurasi: isi WA_GATEWAY_URL & WA_GATEWAY_TOKEN pada layanan app di docker-compose, lalu jalankan ulang." />
    <AppAlert v-else-if="status && !status.gateway.reachable" type="warning"
      :message="`Layanan wa-gateway tidak merespons (${status.gateway.error || 'tidak diketahui'}). Pastikan container 'wa' berjalan.`" />
    <AppAlert v-else-if="status && !status.enabled" type="warning"
      message="Pengiriman sedang dimatikan dari pengaturan — pesan tetap masuk antrean tetapi tidak dikirim." />

    <!-- Ringkasan -->
    <div class="grid grid-cols-2 gap-3 lg:grid-cols-4">
      <AppCard class="min-w-0">
        <p class="stat-lbl">Koneksi</p>
        <p class="stat-val" :class="stateColor">{{ stateLabel }}</p>
        <p class="stat-sub">{{ status?.gateway.push_name || status?.gateway.jid ? `${status.gateway.push_name || ''} +${status.gateway.jid}` : 'belum ada nomor' }}</p>
      </AppCard>
      <AppCard class="min-w-0">
        <p class="stat-lbl">Antrean</p>
        <p class="stat-val" :class="status?.queue.pending ? 'text-amber-600' : ''">{{ status?.queue.pending ?? 0 }}</p>
        <p class="stat-sub">menunggu dikirim</p>
      </AppCard>
      <AppCard class="min-w-0">
        <p class="stat-lbl">Terkirim hari ini</p>
        <p class="stat-val">{{ status?.queue.sent_today ?? 0 }} <small class="text-xs font-medium text-gray-400">/ {{ status?.queue.daily_limit ?? '–' }}</small></p>
        <p class="stat-sub">{{ status?.queue.last_sent_at ? 'terakhir ' + status.queue.last_sent_at : 'belum ada' }}</p>
      </AppCard>
      <AppCard class="min-w-0">
        <p class="stat-lbl">Gagal</p>
        <p class="stat-val" :class="status?.queue.failed ? 'text-red-600' : ''">{{ status?.queue.failed ?? 0 }}</p>
        <p class="stat-sub">{{ status?.queue.failed_today ?? 0 }} hari ini</p>
      </AppCard>
    </div>

    <!-- Tab -->
    <div class="flex flex-wrap gap-2">
      <button v-for="t in TABS" :key="t.key" class="px-4 py-2 text-sm font-medium rounded-lg transition-colors"
        :class="tab === t.key ? 'bg-emerald-600 text-white shadow-sm' : 'bg-white border border-gray-200 text-gray-600 hover:bg-gray-50'"
        @click="tab = t.key">{{ t.label }}</button>
    </div>

    <!-- ── Koneksi ── -->
    <div v-show="tab === 'koneksi'" class="grid gap-4 lg:grid-cols-2">
      <AppCard>
        <h2 class="sec-title">Perangkat tertaut</h2>
        <div v-if="loggedIn" class="mt-3 space-y-3">
          <div class="flex items-center gap-3 rounded-xl bg-emerald-50 p-3">
            <svg class="h-6 w-6 shrink-0 text-emerald-600" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"/></svg>
            <div class="min-w-0 text-sm">
              <p class="font-semibold text-emerald-800">Terhubung sebagai {{ status.gateway.push_name || 'nomor ini' }} (+{{ status.gateway.jid }})</p>
              <p class="text-xs text-emerald-700">{{ status.gateway.note }} · {{ status.gateway.sent_count }} pesan sejak gateway menyala</p>
            </div>
          </div>
          <p class="text-xs text-gray-500">
            Sesi tersimpan di database dan bertahan walau server/container dimulai ulang. Nomor ini juga
            muncul di HP pada WhatsApp → Perangkat Tertaut; mengeluarkannya dari sana memutus gateway.
          </p>
          <div v-if="canManage" class="flex justify-end">
            <AppButton variant="danger" size="sm" :loading="busy.logout" @click="logout">Lepas nomor (logout)</AppButton>
          </div>
        </div>
        <div v-else class="mt-3 space-y-3">
          <p class="text-sm text-gray-600">{{ status?.gateway.note || 'Memuat status…' }}</p>
          <div v-if="qrDataUrl" class="flex flex-col items-center gap-2">
            <img :src="qrDataUrl" alt="QR WhatsApp" class="h-64 w-64 rounded-xl border border-gray-200 bg-white p-2" />
            <p class="text-xs text-gray-500">QR berganti otomatis · berlaku {{ qrSecondsLeft }} dtk</p>
          </div>
          <ol class="list-decimal space-y-1 pl-5 text-xs text-gray-600">
            <li>Buka WhatsApp di HP dengan nomor khusus untuk toko (disarankan bukan nomor pribadi).</li>
            <li>Menu ⋮ → <b>Perangkat tertaut</b> → <b>Tautkan perangkat</b>.</li>
            <li>Pindai QR di layar ini. Setelah tersambung, kirim pesan uji.</li>
          </ol>
          <div v-if="canManage" class="flex flex-wrap justify-end gap-2">
            <AppButton size="sm" variant="secondary" :loading="busy.status" @click="refreshStatus(true)">Segarkan</AppButton>
            <AppButton size="sm" :loading="busy.login" :disabled="!status?.gateway.reachable" @click="login">
              {{ qrDataUrl ? 'Minta QR baru' : 'Tampilkan QR' }}
            </AppButton>
          </div>
        </div>
      </AppCard>

      <AppCard>
        <h2 class="sec-title">Kirim pesan uji</h2>
        <p class="mt-1 text-xs text-gray-500">Dikirim langsung tanpa antrean; hasilnya tercatat di log pesan.</p>
        <form class="mt-3 space-y-3" @submit.prevent="sendTest">
          <div>
            <label class="lbl">Nomor tujuan</label>
            <input v-model="test.to" class="form-input" placeholder="08xx atau 628xx" :disabled="!canManage" />
          </div>
          <div>
            <label class="lbl">Pesan</label>
            <textarea v-model="test.message" rows="3" class="form-input" placeholder="Pesan uji dari Cloud POS" :disabled="!canManage" />
          </div>
          <div class="flex justify-end">
            <AppButton type="submit" size="sm" :loading="busy.test" :disabled="!canManage || !loggedIn">Kirim uji</AppButton>
          </div>
          <p v-if="testResult" class="text-xs" :class="testResult.ok ? 'text-emerald-700' : 'text-red-600'">{{ testResult.msg }}</p>
        </form>

        <div class="mt-4 rounded-lg bg-amber-50 p-3 text-xs text-amber-800">
          <p class="font-semibold">Jalur tidak resmi — jaga ritme pengiriman.</p>
          <p class="mt-1">
            Gateway ini memakai protokol WhatsApp Web (whatsmeow), bukan WhatsApp Business API berbayar.
            WhatsApp bisa memblokir nomor yang mengirim beruntun ke banyak orang asing. Karena itu pesan
            dikirim satu per satu dengan jeda acak, ada batas harian dan jam tenang (tab Notifikasi), dan
            pakailah nomor khusus toko yang sudah "hangat" (pernah chat normal), bukan nomor baru.
          </p>
        </div>
      </AppCard>
    </div>

    <!-- ── Notifikasi ── -->
    <div v-show="tab === 'notifikasi'" class="space-y-4">
      <AppCard v-if="settings">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <h2 class="sec-title">Pengaturan pengiriman</h2>
          <label class="flex items-center gap-2 text-sm font-medium" :class="settings.enabled ? 'text-emerald-700' : 'text-gray-500'">
            <input v-model="settings.enabled" type="checkbox" class="h-4 w-4 accent-emerald-600" :disabled="!canManage" />
            Pengiriman aktif
          </label>
        </div>
        <div class="mt-3 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          <div>
            <label class="lbl">Jeda antar pesan (detik)</label>
            <div class="flex items-center gap-2">
              <input v-model.number="settings.gap_min_sec" type="number" min="1" max="300" class="form-input" :disabled="!canManage" />
              <span class="text-xs text-gray-400">–</span>
              <input v-model.number="settings.gap_max_sec" type="number" min="1" max="600" class="form-input" :disabled="!canManage" />
            </div>
            <p class="hint">Acak di rentang ini; makin lama makin aman dari blokir.</p>
          </div>
          <div>
            <label class="lbl">Batas pesan per hari</label>
            <input v-model.number="settings.daily_limit" type="number" min="1" max="10000" class="form-input" :disabled="!canManage" />
            <p class="hint">Sisa antrean ditunda ke besok pukul 07:00.</p>
          </div>
          <div>
            <label class="lbl">Jam tenang (pesan ke pelanggan)</label>
            <div class="flex items-center gap-2">
              <input v-model="settings.quiet_start" type="time" class="form-input" :disabled="!canManage" />
              <span class="text-xs text-gray-400">–</span>
              <input v-model="settings.quiet_end" type="time" class="form-input" :disabled="!canManage" />
            </div>
            <p class="hint">Kosongkan keduanya untuk mematikan. Notifikasi internal tidak terpengaruh.</p>
          </div>
          <div>
            <label class="lbl">Alamat publik (tautan status reservasi)</label>
            <input v-model="settings.public_base_url" class="form-input" placeholder="https://pos.nbp.co.id" :disabled="!canManage" />
          </div>
          <div>
            <label class="lbl">Jam rekap penjualan harian</label>
            <input v-model.number="settings.report_hour" type="number" min="0" max="23" class="form-input" :disabled="!canManage" />
          </div>
          <div>
            <label class="lbl">Jam pengingat reservasi H-1</label>
            <input v-model.number="settings.reminder_hour" type="number" min="0" max="23" class="form-input" :disabled="!canManage" />
          </div>
          <div>
            <label class="lbl">Perangkat dianggap offline setelah (menit)</label>
            <input v-model.number="settings.device_offline_min" type="number" min="5" max="1440" class="form-input" :disabled="!canManage" />
          </div>
        </div>
      </AppCard>

      <AppCard v-for="grp in eventGroups" :key="grp.name">
        <h2 class="sec-title">{{ grp.name }}</h2>
        <p class="mt-0.5 text-xs text-gray-500">{{ GROUP_DESC[grp.name] }}</p>
        <div class="mt-3 divide-y divide-gray-100">
          <div v-for="ev in grp.events" :key="ev.key" class="py-3">
            <div class="flex flex-wrap items-start gap-3">
              <label class="mt-0.5 flex items-center">
                <input v-model="settings.events[ev.key]" type="checkbox" class="h-4 w-4 accent-emerald-600" :disabled="!canManage" />
              </label>
              <div class="min-w-0 flex-1">
                <div class="flex flex-wrap items-center gap-2">
                  <span class="text-sm font-semibold text-gray-800">{{ ev.label }}</span>
                  <span class="chip" :class="ev.audience === 'customer' ? 'chip--cust' : 'chip--int'">{{ ev.audience === 'customer' ? 'ke pelanggan' : 'ke penerima internal' }}</span>
                  <span v-if="ev.scheduled" class="chip chip--sched">terjadwal</span>
                  <span v-if="settings.templates[ev.key] && settings.templates[ev.key] !== ev.default_template" class="chip chip--custom">template diubah</span>
                </div>
                <p v-if="ev.description" class="mt-0.5 text-xs text-gray-500">{{ ev.description }}</p>
                <p v-if="ev.audience === 'internal'" class="mt-0.5 text-[11px] text-gray-500">
                  <span class="font-semibold text-gray-600">Penerima:</span>{{ ' ' }}
                  <template v-if="(settings.event_roles[ev.key] || []).length">posisi {{ (settings.event_roles[ev.key] || []).join(', ') }} (menimpa aturan otomatis)</template>
                  <template v-else>{{ ev.route_desc }}</template>
                </p>
              </div>
              <button type="button" class="text-xs font-semibold text-emerald-700 hover:underline" @click="toggleEdit(ev.key)">
                {{ editing[ev.key] ? 'Tutup' : 'Ubah template' }}
              </button>
            </div>
            <div v-if="editing[ev.key]" class="mt-3 grid gap-3 lg:grid-cols-2">
              <div>
                <textarea v-model="settings.templates[ev.key]" rows="10" class="form-input font-mono text-xs" :disabled="!canManage" />
                <div class="mt-2 flex flex-wrap gap-1">
                  <button v-for="p in ev.placeholders" :key="p" type="button" class="ph" @click="insertPlaceholder(ev.key, p)">{{ '{' + p + '}' }}</button>
                </div>
                <div class="mt-2 flex flex-wrap gap-2">
                  <AppButton size="sm" variant="secondary" @click="preview(ev)">Pratinjau</AppButton>
                  <AppButton size="sm" variant="ghost" :disabled="!canManage" @click="settings.templates[ev.key] = ev.default_template">Kembalikan bawaan</AppButton>
                </div>
                <div v-if="ev.audience === 'internal'" class="mt-3 rounded-lg bg-gray-50 p-3">
                  <p class="lbl">Posisi penerima</p>
                  <p class="hint mb-2">Kosong = otomatis: {{ ev.route_desc }}. Centang role untuk menimpa aturan itu (batasan outlet role tetap berlaku).</p>
                  <div class="flex flex-wrap gap-1.5">
                    <label v-for="r in roleOptions" :key="r.name" class="rolechip" :class="{ 'rolechip--on': (settings.event_roles[ev.key] || []).includes(r.name) }">
                      <input type="checkbox" class="sr-only" :checked="(settings.event_roles[ev.key] || []).includes(r.name)" :disabled="!canManage" @change="toggleRole(ev.key, r.name)" />
                      {{ r.name }}<span class="opacity-60"> · {{ r.users }} org</span>
                    </label>
                  </div>
                </div>
              </div>
              <div>
                <p class="lbl">Pratinjau (data contoh)</p>
                <div class="bubble">{{ previews[ev.key] || 'Klik Pratinjau untuk melihat hasilnya.' }}</div>
              </div>
            </div>
          </div>
        </div>
      </AppCard>

      <div v-if="settings && canManage" class="sticky bottom-3 flex justify-end">
        <AppButton :loading="busy.save" @click="saveSettings">Simpan pengaturan & template</AppButton>
      </div>
    </div>

    <!-- ── Penerima ── -->
    <div v-show="tab === 'penerima'" class="space-y-4">
      <AppCard>
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h2 class="sec-title">Berdasarkan posisi (akun pengguna)</h2>
            <p class="mt-0.5 text-xs text-gray-500">
              Setiap pengguna hanya menerima event yang berkaitan dengan posisinya: ditentukan dari hak akses
              role-nya dan dibatasi outlet role tersebut. Nomor WhatsApp diatur di menu
              <RouterLink to="/admins" class="font-semibold text-emerald-700 hover:underline">Pengguna</RouterLink>;
              aturan per event bisa ditimpa di tab Notifikasi (Posisi penerima).
            </p>
          </div>
          <button class="text-xs font-semibold text-emerald-700 hover:underline" @click="loadRecipientUsers">Segarkan</button>
        </div>
        <div class="mt-3 overflow-x-auto">
          <table class="w-full text-sm">
            <thead class="bg-gray-50 text-left text-xs uppercase tracking-wide text-gray-500">
              <tr>
                <th class="px-3 py-2">Pengguna</th>
                <th class="px-3 py-2">Role</th>
                <th class="px-3 py-2">Nomor WA</th>
                <th class="px-3 py-2">Outlet</th>
                <th class="px-3 py-2">Event yang diterima</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100">
              <tr v-if="!recipientUsers.length"><td colspan="5" class="px-3 py-8 text-center text-gray-400">Belum ada pengguna.</td></tr>
              <tr v-for="u in recipientUsers" :key="u.id" :class="{ 'opacity-50': !u.is_active }">
                <td class="px-3 py-2">
                  <p class="font-medium text-gray-800">{{ u.name }}</p>
                  <p class="text-xs text-gray-400">{{ u.username }}<span v-if="!u.is_active"> · nonaktif</span></p>
                </td>
                <td class="px-3 py-2"><span class="chip chip--int">{{ u.role }}</span></td>
                <td class="px-3 py-2 text-xs">
                  <template v-if="u.wa_phone">
                    <span class="font-mono">+{{ u.wa_phone }}</span>
                    <span v-if="!u.wa_notify" class="chip chip--off ml-1">notifikasi mati</span>
                  </template>
                  <span v-else class="italic text-gray-400">belum diisi</span>
                </td>
                <td class="px-3 py-2 text-xs text-gray-600">{{ u.scope_type === 'specific' ? (u.outlet_names.join(', ') || '(tidak ada)') : 'Semua' }}</td>
                <td class="px-3 py-2 text-xs">
                  <template v-if="u.events.length">
                    <span v-for="k in u.events.slice(0, expandedUser[u.id] ? 99 : 4)" :key="k" class="chip chip--sched mb-1 mr-1">{{ eventLabel(k) }}</span>
                    <button v-if="u.events.length > 4" class="text-[11px] font-semibold text-emerald-700 hover:underline" @click="expandedUser[u.id] = !expandedUser[u.id]">
                      {{ expandedUser[u.id] ? 'ringkas' : `+${u.events.length - 4} lagi` }}
                    </button>
                  </template>
                  <span v-else class="text-gray-400">—</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </AppCard>

      <AppCard>
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h2 class="sec-title">Penerima tambahan (grup WhatsApp & nomor di luar akun)</h2>
            <p class="mt-0.5 text-xs text-gray-500">
              Untuk grup WhatsApp manajemen atau nomor yang bukan akun pengguna. Pilih event dan outlet
              yang relevan supaya tidak menerima hal yang tidak berkaitan.
            </p>
          </div>
          <AppButton v-if="canManage" size="sm" @click="openRecipient()">+ Tambah penerima</AppButton>
        </div>
        <div class="mt-3 overflow-x-auto">
          <table class="w-full text-sm">
            <thead class="bg-gray-50 text-left text-xs uppercase tracking-wide text-gray-500">
              <tr>
                <th class="px-3 py-2">Nama</th>
                <th class="px-3 py-2">Tujuan</th>
                <th class="px-3 py-2">Event</th>
                <th class="px-3 py-2">Outlet</th>
                <th class="px-3 py-2">Status</th>
                <th class="px-3 py-2 text-right">Aksi</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100">
              <tr v-if="!recipients.length"><td colspan="6" class="px-3 py-8 text-center text-gray-400">Belum ada penerima. Tambahkan nomor pemilik atau grup manajer.</td></tr>
              <tr v-for="r in recipients" :key="r.id">
                <td class="px-3 py-2 font-medium text-gray-800">{{ r.name }}<p v-if="r.notes" class="text-xs font-normal text-gray-400">{{ r.notes }}</p></td>
                <td class="px-3 py-2 text-gray-600">
                  <span class="chip" :class="r.kind === 'group' ? 'chip--sched' : 'chip--int'">{{ r.kind === 'group' ? 'grup' : 'nomor' }}</span>
                  <span class="ml-1 font-mono text-xs">{{ r.kind === 'group' ? r.target.replace('@g.us', '') : '+' + r.target }}</span>
                </td>
                <td class="px-3 py-2 text-xs text-gray-600">{{ eventsLabel(r.events) }}</td>
                <td class="px-3 py-2 text-xs text-gray-600">{{ outletsLabel(r.outlet_ids) }}</td>
                <td class="px-3 py-2"><span class="chip" :class="r.is_active ? 'chip--ok' : 'chip--off'">{{ r.is_active ? 'aktif' : 'nonaktif' }}</span></td>
                <td class="px-3 py-2 text-right whitespace-nowrap">
                  <template v-if="canManage">
                    <button class="text-xs font-semibold text-emerald-700 hover:underline" @click="openRecipient(r)">Ubah</button>
                    <button class="ml-3 text-xs font-semibold text-red-600 hover:underline" @click="deleteRecipient(r)">Hapus</button>
                  </template>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </AppCard>
    </div>

    <!-- Modal penerima -->
    <AppModal v-model="showRecipient" :title="recipientForm.id ? 'Ubah Penerima' : 'Tambah Penerima'" size="lg">
      <form class="space-y-4" @submit.prevent="saveRecipient">
        <div class="grid gap-3 sm:grid-cols-2">
          <div>
            <label class="lbl">Nama</label>
            <input v-model="recipientForm.name" class="form-input" placeholder="Pak Budi (pemilik)" />
          </div>
          <div>
            <label class="lbl">Jenis</label>
            <div class="flex gap-4 pt-2 text-sm">
              <label class="flex items-center gap-1.5"><input v-model="recipientForm.kind" type="radio" value="phone" class="accent-emerald-600" /> Nomor HP</label>
              <label class="flex items-center gap-1.5"><input v-model="recipientForm.kind" type="radio" value="group" class="accent-emerald-600" /> Grup WhatsApp</label>
            </div>
          </div>
        </div>
        <div v-if="recipientForm.kind === 'phone'">
          <label class="lbl">Nomor HP</label>
          <input v-model="recipientForm.target" class="form-input" placeholder="08xx atau 628xx" />
        </div>
        <div v-else>
          <label class="lbl">Grup</label>
          <div class="flex gap-2">
            <select v-model="recipientForm.target" class="form-input">
              <option value="">— pilih grup yang diikuti nomor gateway —</option>
              <option v-for="g in groups" :key="g.jid" :value="g.jid">{{ g.name }} ({{ g.participants }} anggota)</option>
            </select>
            <AppButton type="button" size="sm" variant="secondary" :loading="busy.groups" @click="loadGroups">Muat grup</AppButton>
          </div>
          <p class="hint">Nomor gateway harus sudah menjadi anggota grup itu. Hanya tersedia saat tersambung.</p>
        </div>
        <div>
          <label class="lbl">Event yang diterima</label>
          <label class="flex items-center gap-2 text-sm"><input v-model="recipientAll" type="checkbox" class="accent-emerald-600" /> Semua notifikasi internal</label>
          <div v-if="!recipientAll" class="mt-2 grid gap-1 sm:grid-cols-2">
            <label v-for="ev in internalEvents" :key="ev.key" class="flex items-center gap-2 text-xs text-gray-700">
              <input v-model="recipientForm.events" type="checkbox" :value="ev.key" class="accent-emerald-600" /> {{ ev.group }} · {{ ev.label }}
            </label>
          </div>
        </div>
        <div>
          <label class="lbl">Batasi outlet (kosong = semua)</label>
          <div class="grid gap-1 sm:grid-cols-2">
            <label v-for="o in outlets" :key="o.id" class="flex items-center gap-2 text-xs text-gray-700">
              <input v-model="recipientForm.outlet_ids" type="checkbox" :value="o.id" class="accent-emerald-600" /> {{ o.name }}
            </label>
          </div>
          <p class="hint">Notifikasi yang tidak terkait outlet tertentu (mis. peringatan stok gudang) tetap dikirim.</p>
        </div>
        <div class="grid gap-3 sm:grid-cols-2">
          <div>
            <label class="lbl">Catatan</label>
            <input v-model="recipientForm.notes" class="form-input" />
          </div>
          <label class="flex items-center gap-2 pt-6 text-sm"><input v-model="recipientForm.is_active" type="checkbox" class="accent-emerald-600" /> Aktif</label>
        </div>
        <div class="flex justify-end gap-2 pt-2">
          <AppButton type="button" variant="secondary" @click="showRecipient = false">Batal</AppButton>
          <AppButton type="submit" :loading="busy.recipient">Simpan</AppButton>
        </div>
      </form>
    </AppModal>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import QRCode from 'qrcode'
import AppAlert from '@/components/ui/AppAlert.vue'
import AppButton from '@/components/ui/AppButton.vue'
import AppCard from '@/components/ui/AppCard.vue'
import AppModal from '@/components/ui/AppModal.vue'
import { whatsappApi } from '@/api/whatsapp.js'
import { outletsApi } from '@/api/outlets.js'
import { useAuthStore } from '@/stores/auth.js'
import { useToastStore } from '@/stores/toast.js'

const auth = useAuthStore()
const toast = useToastStore()
// Cek eksplisit: aturan lama hasPermission menganggap '.manage' terpenuhi oleh
// izin modul apa pun (mis. whatsapp.broadcast), padahal server menuntut kuncinya persis.
const canManage = computed(() => auth.isSuperadmin || (auth.permissions || []).includes('whatsapp.manage'))

const TABS = [
  { key: 'koneksi', label: 'Koneksi' },
  { key: 'notifikasi', label: 'Notifikasi & Template' },
  { key: 'penerima', label: 'Penerima' },
]
const GROUP_DESC = {
  Reservasi: 'Dikirim ke penerima internal saat ada reservasi baru, bukti bayar masuk, konfirmasi, atau pembatalan.',
  Pelanggan: 'Dikirim ke nomor HP pelanggan yang tercantum pada reservasi. Tunduk jam tenang.',
  Penjualan: 'Laporan tutup kasir dari app POS dan rekap harian terjadwal.',
  Pengadaan: 'Perjalanan status pengajuan pengadaan.',
  Gudang: 'Evaluasi PPIC harian: batch kedaluwarsa dan stok di bawah titik pesan ulang.',
  Aset: 'Work order perawatan yang diterbitkan penjadwal.',
  Perangkat: 'Tablet kasir yang berhenti mengirim heartbeat.',
}

const tab = ref('koneksi')
const status = ref(null)
const settings = ref(null)
const events = ref([])
const recipients = ref([])
const recipientUsers = ref([])
const roleOptions = ref([])
const expandedUser = ref({})
const groups = ref([])
const outlets = ref([])
const errorMsg = ref('')
const busy = ref({ status: false, login: false, logout: false, test: false, save: false, groups: false, recipient: false })
const test = ref({ to: '', message: '' })
const testResult = ref(null)
const editing = ref({})
const previews = ref({})
const qrDataUrl = ref('')
const qrSecondsLeft = ref(0)
let lastQr = ''
let pollTimer = null
let qrTimer = null

const loggedIn = computed(() => !!(status.value?.gateway.logged_in && status.value?.gateway.connected))
const STATE_LABEL = {
  connected: 'Tersambung', connecting: 'Menyambung…', pairing: 'Menunggu QR dipindai', qr_timeout: 'QR kedaluwarsa',
  logged_out: 'Belum dipasang', offline: 'Terputus', banned: 'Diblokir sementara', error: 'Bermasalah',
  unreachable: 'Gateway mati', unconfigured: 'Belum dikonfigurasi',
}
const stateLabel = computed(() => STATE_LABEL[status.value?.gateway.state] || status.value?.gateway.state || '…')
const stateColor = computed(() => {
  const s = status.value?.gateway.state
  if (s === 'connected') return 'text-emerald-700'
  if (['banned', 'error', 'unreachable'].includes(s)) return 'text-red-600'
  return 'text-amber-600'
})

const eventGroups = computed(() => {
  const map = new Map()
  for (const ev of events.value) {
    if (!map.has(ev.group)) map.set(ev.group, [])
    map.get(ev.group).push(ev)
  }
  return [...map.entries()].map(([name, evs]) => ({ name, events: evs }))
})
const internalEvents = computed(() => events.value.filter(e => e.audience === 'internal'))

// ── Status & QR ──
async function refreshStatus(force = false) {
  busy.value.status = true
  try {
    status.value = await whatsappApi.status(force)
    await renderQr(status.value.gateway.qr || '')
    if (loggedIn.value) stopPolling()
  } catch (e) { errorMsg.value = e?.message || 'Gagal memuat status' } finally { busy.value.status = false }
}

async function renderQr(code) {
  if (!code) { qrDataUrl.value = ''; lastQr = ''; return }
  if (code === lastQr) return
  lastQr = code
  try { qrDataUrl.value = await QRCode.toDataURL(code, { width: 320, margin: 1 }) } catch { qrDataUrl.value = '' }
}

function startPolling() {
  stopPolling()
  pollTimer = setInterval(() => refreshStatus(true), 2500)
  qrTimer = setInterval(() => {
    const exp = status.value?.gateway.qr_expires_at
    qrSecondsLeft.value = exp ? Math.max(0, Math.round((new Date(exp) - Date.now()) / 1000)) : 0
  }, 1000)
}
function stopPolling() {
  clearInterval(pollTimer); clearInterval(qrTimer); pollTimer = qrTimer = null
}

async function login() {
  busy.value.login = true
  try {
    const gw = await whatsappApi.login()
    status.value = { ...(status.value || { queue: {}, enabled: true }), gateway: gw }
    await renderQr(gw.qr || '')
    startPolling()
  } catch (e) { toast.error(e?.message || 'Gagal memulai pemasangan') } finally { busy.value.login = false }
}

async function logout() {
  if (!confirm('Lepas nomor dari gateway? Notifikasi berhenti sampai QR dipindai lagi.')) return
  busy.value.logout = true
  try {
    await whatsappApi.logout()
    toast.success('Nomor dilepas')
    await refreshStatus(true)
  } catch (e) { toast.error(e?.message || 'Gagal melepas nomor') } finally { busy.value.logout = false }
}

async function sendTest() {
  busy.value.test = true
  testResult.value = null
  try {
    const m = await whatsappApi.sendTest(test.value)
    testResult.value = { ok: true, msg: `Terkirim ke +${m.target} (id ${m.wa_message_id || '-'})` }
    refreshStatus(true)
  } catch (e) { testResult.value = { ok: false, msg: e?.message || 'Gagal' } } finally { busy.value.test = false }
}

// ── Pengaturan & template ──
async function loadSettings() {
  try {
    const res = await whatsappApi.getSettings()
    const s = res.settings
    if (s.quiet_start === '-') { s.quiet_start = ''; s.quiet_end = '' }
    s.templates = s.templates || {}
    s.event_roles = s.event_roles || {}
    for (const ev of res.events) if (!s.templates[ev.key]) s.templates[ev.key] = ev.template
    settings.value = s
    events.value = res.events
  } catch (e) { errorMsg.value = e?.message || 'Gagal memuat pengaturan' }
}

function toggleEdit(key) { editing.value[key] = !editing.value[key] }

function insertPlaceholder(key, p) {
  settings.value.templates[key] = (settings.value.templates[key] || '') + '{' + p + '}'
}

async function preview(ev) {
  try {
    const r = await whatsappApi.preview({ event: ev.key, template: settings.value.templates[ev.key] })
    previews.value[ev.key] = r.preview
  } catch (e) { toast.error(e?.message || 'Gagal membuat pratinjau') }
}

async function saveSettings() {
  busy.value.save = true
  try {
    const res = await whatsappApi.updateSettings(settings.value)
    toast.success('Pengaturan WhatsApp disimpan')
    const s = res.settings
    if (s.quiet_start === '-') { s.quiet_start = ''; s.quiet_end = '' }
    s.templates = s.templates || {}
    s.event_roles = s.event_roles || {}
    for (const ev of res.events) if (!s.templates[ev.key]) s.templates[ev.key] = ev.template
    settings.value = s
    events.value = res.events
    loadRecipientUsers()
    refreshStatus(true)
  } catch (e) { toast.error(e?.message || 'Gagal menyimpan') } finally { busy.value.save = false }
}

// ── Penerima ──
const showRecipient = ref(false)
const recipientForm = ref(emptyRecipient())
const recipientAll = ref(true)
function emptyRecipient() {
  return { id: '', name: '', target: '', kind: 'phone', events: [], outlet_ids: [], is_active: true, notes: '' }
}
async function loadRecipientUsers() {
  try { recipientUsers.value = await whatsappApi.recipientUsers() } catch (e) { errorMsg.value = e?.message || 'Gagal memuat pengguna' }
}
async function loadRoles() {
  try { roleOptions.value = await whatsappApi.roles() } catch { roleOptions.value = [] }
}
function eventLabel(key) {
  return events.value.find(e => e.key === key)?.label || key
}
function toggleRole(eventKey, role) {
  const cur = settings.value.event_roles[eventKey] || []
  settings.value.event_roles[eventKey] = cur.includes(role) ? cur.filter(r => r !== role) : [...cur, role]
}
async function loadRecipients() {
  try { recipients.value = await whatsappApi.listRecipients() } catch (e) { errorMsg.value = e?.message || 'Gagal memuat penerima' }
}
async function loadOutlets() {
  try {
    const res = await outletsApi.list()
    outlets.value = (Array.isArray(res) ? res : (res?.data || [])).map(o => ({ id: (o.id || '').trim(), name: o.name }))
  } catch { outlets.value = [] }
}
async function loadGroups() {
  busy.value.groups = true
  try { groups.value = await whatsappApi.groups() } catch (e) { toast.error(e?.message || 'Gagal memuat grup') } finally { busy.value.groups = false }
}
function openRecipient(r) {
  recipientForm.value = r ? { ...r, events: [...(r.events || [])], outlet_ids: [...(r.outlet_ids || [])] } : emptyRecipient()
  recipientAll.value = !r || !r.events?.length || r.events.includes('*')
  if (recipientAll.value) recipientForm.value.events = []
  showRecipient.value = true
  if (recipientForm.value.kind === 'group' && !groups.value.length) loadGroups()
}
watch(() => recipientForm.value.kind, (k) => { if (k === 'group' && !groups.value.length && showRecipient.value) loadGroups() })
async function saveRecipient() {
  busy.value.recipient = true
  try {
    const payload = { ...recipientForm.value, events: recipientAll.value ? ['*'] : recipientForm.value.events }
    if (payload.id) await whatsappApi.updateRecipient(payload.id, payload)
    else await whatsappApi.createRecipient(payload)
    toast.success('Penerima disimpan')
    showRecipient.value = false
    await loadRecipients()
  } catch (e) { toast.error(e?.message || 'Gagal menyimpan penerima') } finally { busy.value.recipient = false }
}
async function deleteRecipient(r) {
  if (!confirm(`Hapus penerima "${r.name}"?`)) return
  try { await whatsappApi.deleteRecipient(r.id); toast.success('Penerima dihapus'); await loadRecipients() }
  catch (e) { toast.error(e?.message || 'Gagal menghapus') }
}
function eventsLabel(evs) {
  if (!evs?.length || evs.includes('*')) return 'Semua'
  return `${evs.length} event`
}
function outletsLabel(ids) {
  if (!ids?.length) return 'Semua'
  const names = ids.map(id => outlets.value.find(o => o.id === id)?.name || id)
  return names.join(', ')
}

onMounted(async () => {
  await Promise.all([refreshStatus(true), loadSettings(), loadRecipients(), loadRecipientUsers(), loadRoles(), loadOutlets()])
  if (status.value?.gateway.state === 'pairing') startPolling()
})
onUnmounted(stopPolling)
</script>

<style scoped>
.form-input {
  width: 100%; padding: .5rem .7rem; border-radius: .6rem; font-size: .85rem;
  border: 1px solid rgba(0,0,0,.14); background: #fff; color: #111827; outline: none;
}
.form-input:focus { border-color: rgba(5,150,105,.5); box-shadow: 0 0 0 3px rgba(5,150,105,.12); }
.form-input:disabled { background: #f9fafb; color: #6b7280; }
.lbl { display: block; font-size: .72rem; font-weight: 700; color: #4b5563; margin-bottom: .25rem; }
.hint { margin-top: .25rem; font-size: .68rem; color: #6b7280; }
.sec-title { font-size: .95rem; font-weight: 700; color: #111827; }
.stat-lbl { font-size: .68rem; font-weight: 700; color: #9ca3af; text-transform: uppercase; letter-spacing: .02em; }
.stat-val { margin-top: .15rem; font-size: 1rem; font-weight: 700; color: #111827; }
.stat-sub { margin-top: .1rem; font-size: .7rem; color: #6b7280; overflow-wrap: anywhere; }
.chip { display: inline-block; padding: .05rem .45rem; border-radius: 999px; font-size: .65rem; font-weight: 600; }
.chip--int { background: #ecfdf5; color: #047857; }
.chip--cust { background: #eff6ff; color: #1d4ed8; }
.chip--sched { background: #f5f3ff; color: #6d28d9; }
.chip--custom { background: #fffbeb; color: #b45309; }
.chip--ok { background: #ecfdf5; color: #047857; }
.chip--off { background: #f3f4f6; color: #6b7280; }
.rolechip { display: inline-flex; align-items: center; padding: .15rem .55rem; border-radius: 999px; font-size: .68rem; font-weight: 600; border: 1px solid #d1d5db; color: #374151; background: #fff; cursor: pointer; }
.rolechip--on { background: #059669; border-color: #059669; color: #fff; }
.ph { padding: .1rem .4rem; border-radius: .4rem; background: #f3f4f6; font-family: ui-monospace, monospace; font-size: .65rem; color: #374151; }
.ph:hover { background: #d1fae5; color: #065f46; }
.bubble {
  white-space: pre-wrap; overflow-wrap: anywhere; font-size: .8rem; line-height: 1.4; color: #111827;
  background: #dcf8c6; border-radius: .8rem; padding: .6rem .8rem; min-height: 6rem;
}
</style>
