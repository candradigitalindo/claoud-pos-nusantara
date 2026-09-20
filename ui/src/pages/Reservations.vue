<template>
  <div class="space-y-5">
    <div class="flex items-start justify-between flex-wrap gap-3">
      <div>
        <h1 class="text-xl font-bold text-gray-900">Reservasi</h1>
        <p class="text-sm text-gray-500 mt-0.5">Tamu, menu dipesan, DP yang diminta, uang yang sudah tervalidasi, dan sisanya.</p>
      </div>
      <AppButton v-if="canCreate" @click="openCreate">+ Reservasi Baru</AppButton>
    </div>

    <AppAlert type="error" :message="errorMsg" />

    <!-- Filters + public link -->
    <AppCard>
      <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
        <SearchSelect v-model="filterOutlet" :options="outletFilterOptions" placeholder="Semua outlet" searchPlaceholder="Cari outlet…" @change="load" />
        <select v-model="filterStatus" @change="load" class="form-input">
          <option value="">Semua status</option>
          <option v-for="(l,k) in STATUS" :key="k" :value="k">{{ l }}</option>
        </select>
        <DateRangePicker v-model="range" clearable />
      </div>
      <div v-if="selectedOutletObj?.slug" class="mt-3 flex items-center gap-2 flex-wrap text-xs">
        <span class="text-gray-500">Link reservasi publik:</span>
        <input :value="publicUrl" readonly class="flex-1 min-w-[200px] font-mono text-gray-600 bg-gray-50 border border-gray-200 rounded px-2 py-1" />
        <button @click="copyLink" class="px-2 py-1 rounded font-medium" :class="copied ? 'text-emerald-600 bg-emerald-50' : 'text-gray-600 bg-gray-100'">{{ copied ? 'Tersalin!' : 'Salin' }}</button>
        <a :href="publicUrl" target="_blank" rel="noopener" class="px-2 py-1 rounded font-medium text-emerald-700 bg-emerald-50">Buka</a>
      </div>
      <!-- Kebijakan DP untuk reservasi dari halaman publik; admin bisa mengubah per reservasi. -->
      <div v-if="canUpdate" class="mt-3 flex items-center gap-2 flex-wrap text-xs border-t border-gray-100 pt-3">
        <span class="text-gray-500">DP yang diminta dari halaman publik:</span>
        <input v-model.number="dpPercentInput" type="number" min="0" max="100" class="form-input !w-20 !py-1 text-right" /> <span class="text-gray-500">% dari total</span>
        <button @click="saveSettings" :disabled="savingSettings" class="px-2 py-1 rounded font-medium text-emerald-700 bg-emerald-50 hover:bg-emerald-100">Simpan</button>
        <span v-if="!settings.bank_accounts?.length" class="text-amber-700">Belum ada rekening aktif di Keuangan → Rekening Bank; pelanggan tidak tahu harus transfer ke mana.</span>
      </div>
    </AppCard>

    <!-- List -->
    <AppCard :padding="false">
      <!-- Mobile cards -->
      <div class="sm:hidden">
        <div v-if="loading" class="p-6 text-center text-sm text-gray-400">Memuat…</div>
        <div v-else-if="!rows.length" class="p-6 text-center text-sm text-gray-400">Belum ada reservasi.</div>
        <ul v-else class="divide-y divide-gray-100">
          <li v-for="r in rows" :key="r.id" class="p-4 space-y-2">
            <div class="flex items-start justify-between gap-2">
              <div class="min-w-0">
                <p class="font-semibold text-gray-900 break-words">{{ r.customer_name }}</p>
                <p class="text-xs text-gray-500">{{ r.outlet_name }} · {{ r.pax }} tamu<span v-if="r.customer_phone"> · {{ r.customer_phone }}</span></p>
              </div>
              <span class="st-badge shrink-0" :class="stCls(r.status)">{{ STATUS[r.status] || r.status }}</span>
            </div>
            <p class="text-xs text-gray-500">📅 {{ r.reservation_date ? formatDateStr(r.reservation_date) : '—' }} {{ r.reservation_time }}</p>
            <p class="text-xs text-gray-600">Total <b>{{ formatRupiah(r.total) }}</b> · Dibayar {{ formatRupiah(r.paid_amount) }} · Sisa <b class="text-amber-600">{{ formatRupiah(r.remaining) }}</b></p>
            <p v-if="r.pending_amount > 0" class="text-xs font-semibold text-amber-700">Bukti {{ formatRupiah(r.pending_amount) }} menunggu validasi</p>
            <div class="flex gap-2 pt-1">
              <button @click="openEdit(r)" class="flex-1 text-center text-xs font-medium px-2 py-1.5 rounded-lg bg-gray-100 text-gray-700">Detail</button>
              <button v-if="canDelete" @click="confirmDelete(r)" class="text-center text-xs font-medium px-3 py-1.5 rounded-lg bg-red-50 text-red-600">Hapus</button>
            </div>
          </li>
        </ul>
      </div>

      <!-- Desktop table -->
      <AppTable class="hidden sm:block" :columns="COLUMNS" :rows="rows" :loading="loading" emptyText="Belum ada reservasi.">
        <template #cell-customer_name="{ row }">
          <div><p class="font-medium text-gray-900">{{ row.customer_name }}</p><p v-if="row.customer_phone" class="text-xs text-gray-400">{{ row.customer_phone }}</p></div>
        </template>
        <template #cell-schedule="{ row }">
          <span class="text-sm">{{ row.reservation_date ? formatDateStr(row.reservation_date) : '—' }}</span>
          <span class="text-xs text-gray-400 block">{{ row.reservation_time }}</span>
        </template>
        <template #cell-total="{ row }">{{ formatRupiah(row.total) }}</template>
        <template #cell-paid="{ row }">
          <span>{{ formatRupiah(row.paid_amount) }}</span>
          <span v-if="row.pending_amount > 0" class="block text-[11px] font-semibold text-amber-700">+{{ formatRupiah(row.pending_amount) }} menunggu</span>
        </template>
        <template #cell-remaining="{ row }"><span :class="row.remaining > 0 ? 'text-amber-600 font-semibold' : 'text-gray-400'">{{ formatRupiah(row.remaining) }}</span></template>
        <template #cell-status="{ row }"><span class="st-badge" :class="stCls(row.status)">{{ STATUS[row.status] || row.status }}</span></template>
        <template #cell-source="{ row }"><span class="text-xs" :class="row.source==='public' ? 'text-emerald-600' : 'text-gray-400'">{{ row.source==='public' ? 'Publik' : 'Admin' }}</span></template>
        <template #cell-actions="{ row }">
          <div class="flex items-center gap-1 justify-end">
            <button @click="openEdit(row)" class="text-gray-600 hover:text-gray-900 text-xs font-medium px-2 py-1 rounded hover:bg-gray-100">Detail</button>
            <button v-if="canDelete" @click="confirmDelete(row)" class="text-red-600 hover:text-red-800 text-xs font-medium px-2 py-1 rounded hover:bg-red-50">Hapus</button>
          </div>
        </template>
      </AppTable>
    </AppCard>

    <!-- ── Public reservation URLs per outlet (sesuai role) ── -->
    <AppCard :padding="false">
      <div class="px-4 pt-4 pb-2">
        <h3 class="text-sm font-semibold text-gray-700">URL Reservasi per Outlet</h3>
        <p class="text-xs text-gray-500 mt-0.5">Bagikan link ini ke pelanggan untuk reservasi online (sesuai outlet yang Anda kelola).</p>
      </div>
      <div v-if="!outletsWithSlug.length" class="p-6 text-center text-sm text-gray-400">Belum ada outlet dengan link reservasi.</div>
      <ul v-else class="divide-y divide-gray-100">
        <li v-for="o in outletsWithSlug" :key="o.id" class="px-4 py-3 flex items-center gap-2 flex-wrap">
          <span class="font-medium text-gray-900 w-full sm:w-44 shrink-0 truncate">{{ o.name }}</span>
          <input :value="urlFor(o)" readonly @focus="$event.target.select()" class="flex-1 min-w-[160px] font-mono text-xs text-gray-600 bg-gray-50 border border-gray-200 rounded px-2 py-1" />
          <div class="flex gap-1.5 shrink-0">
            <button @click="copyOutlet(o)" class="px-2.5 py-1 rounded text-xs font-medium" :class="copiedId === o.id ? 'text-emerald-600 bg-emerald-50' : 'text-gray-600 bg-gray-100 hover:bg-gray-200'">{{ copiedId === o.id ? 'Tersalin!' : 'Salin' }}</button>
            <a :href="urlFor(o)" target="_blank" rel="noopener" class="px-2.5 py-1 rounded text-xs font-medium text-emerald-700 bg-emerald-50 hover:bg-emerald-100">Buka</a>
          </div>
        </li>
      </ul>
    </AppCard>

    <!-- ── Reservation Modal ── -->
    <AppModal v-model="modal" :title="editing ? `Reservasi · ${editing.customer_name}` : 'Reservasi Baru'" size="2xl">
      <div class="space-y-4">
        <!-- Ringkasan status (mode detail) -->
        <div v-if="editing" class="rounded-xl border border-gray-200 bg-gray-50 p-3 space-y-2">
          <div class="flex flex-wrap items-center gap-2">
            <span class="st-badge" :class="stCls(editing.status)">{{ STATUS[editing.status] || editing.status }}</span>
            <span v-if="editing.outlet_name" class="meta-chip">{{ editing.outlet_name }}</span>
            <span class="meta-chip">{{ editing.source === 'public' ? 'Dari halaman publik' : 'Dibuat admin' }}</span>
            <span v-if="editing.status === 'cancelled' && editing.cancel_disposition" class="meta-chip meta-chip--red">Uang muka {{ editing.cancel_disposition === 'refund' ? 'dikembalikan (refund)' : 'hangus' }}</span>
            <a v-if="editing.source === 'public' && statusLinkFor(editing)" :href="statusLinkFor(editing)" target="_blank" rel="noopener" class="sm:ml-auto text-xs font-semibold text-emerald-700 underline">Link status pelanggan</a>
          </div>
          <p class="text-[11px] leading-relaxed text-gray-500">
            {{ statusHint(editing) }}
            <template v-if="editing.confirmed_at"> Dikonfirmasi {{ editing.confirmed_at }}.</template>
            <template v-if="editing.settled_at"> Selesai {{ editing.settled_at }}<template v-if="editing.pos_transaction_id"> (transaksi POS {{ editing.pos_transaction_id }})</template>.</template>
          </p>
        </div>

        <form id="resv-form" class="space-y-4" @submit.prevent="save">
          <!-- 1. Outlet & jadwal -->
          <section class="sec">
            <header class="sec-hd">
              <span class="sec-no">1</span>
              <div class="min-w-0">
                <h3 class="sec-title">Outlet & Jadwal</h3>
                <p class="sec-sub">Menu yang bisa dipilih dan link status pelanggan mengikuti outlet ini.</p>
              </div>
            </header>
            <div v-if="!editing">
              <label class="lbl">Outlet <span class="text-red-500">*</span></label>
              <SearchSelect v-model="form.outlet_id" :options="outlets" placeholder="Pilih outlet…" searchPlaceholder="Cari outlet…" @change="onOutletChange" />
            </div>
            <div class="grid grid-cols-2 sm:grid-cols-3 gap-3">
              <div>
                <label class="lbl">Tanggal</label>
                <input v-model="form.reservation_date" type="date" class="form-input" :disabled="locked" />
              </div>
              <div>
                <label class="lbl">Jam</label>
                <input v-model="form.reservation_time" type="time" class="form-input" :disabled="locked" />
              </div>
              <div class="col-span-2 sm:col-span-1">
                <label class="lbl">Jumlah Tamu</label>
                <input v-model.number="form.pax" type="number" min="1" inputmode="numeric" class="form-input" :disabled="locked" />
              </div>
            </div>
          </section>

          <!-- 2. Pemesan -->
          <section class="sec">
            <header class="sec-hd">
              <span class="sec-no">2</span>
              <div class="min-w-0">
                <h3 class="sec-title">Pemesan</h3>
                <p class="sec-sub">No. HP dipakai untuk mencocokkan data pelanggan dan menghubungi tamu.</p>
              </div>
            </header>
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <div>
                <label class="lbl">Nama Pemesan <span class="text-red-500">*</span></label>
                <input v-model="form.customer_name" class="form-input" required :disabled="locked" placeholder="Nama tamu / penanggung jawab" />
              </div>
              <div>
                <label class="lbl">No. HP</label>
                <input v-model="form.customer_phone" class="form-input" inputmode="tel" :disabled="locked" placeholder="08xxxxxxxxxx" />
              </div>
            </div>
          </section>

          <!-- 3. Menu -->
          <section class="sec">
            <header class="sec-hd">
              <span class="sec-no">3</span>
              <div class="min-w-0">
                <h3 class="sec-title">Menu Dipesan</h3>
                <p class="sec-sub">Total dan DP dihitung dari menu di sini. Pesanan tambahan saat tamu datang dicatat di POS.</p>
              </div>
              <span v-if="form.items?.length" class="sec-badge">{{ form.items.length }} item</span>
            </header>
            <SearchSelect v-if="form.outlet_id && !locked" v-model="pickProduct" :options="productOptions" placeholder="+ Tambah produk…" searchPlaceholder="Cari produk…" @change="addProduct" />
            <p v-if="!form.outlet_id" class="empty-hint">Pilih outlet dulu untuk memuat menu.</p>
            <ul v-else-if="form.items.length" class="divide-y divide-gray-100 border border-gray-200 rounded-lg">
              <li v-for="(it,i) in form.items" :key="i" class="flex flex-wrap sm:flex-nowrap items-center gap-x-2 gap-y-1 px-3 py-2">
                <span class="w-full sm:w-auto sm:flex-1 min-w-0 text-sm">
                  <span class="block truncate">{{ it.product_name }}</span>
                  <span class="text-xs text-gray-400">{{ formatRupiah(it.price) }} / porsi</span>
                </span>
                <label class="flex items-center gap-1 text-xs text-gray-500">Qty
                  <input v-model.number="it.qty" type="number" min="1" inputmode="numeric" class="form-input !w-16 !py-1 text-center" :disabled="locked" />
                </label>
                <span class="ml-auto sm:ml-0 sm:w-28 text-right text-sm font-semibold">{{ formatRupiah(it.price * it.qty) }}</span>
                <button v-if="!locked" type="button" @click="form.items.splice(i,1)" class="px-1 text-lg leading-none text-red-400 hover:text-red-600" aria-label="Hapus item">×</button>
              </li>
            </ul>
            <p v-else class="empty-hint">Belum ada menu dipilih. Reservasi tanpa menu tetap bisa dibuat; DP diisi manual.</p>
          </section>

          <!-- 4. Uang muka & ringkasan -->
          <section class="sec">
            <header class="sec-hd">
              <span class="sec-no">4</span>
              <div class="min-w-0">
                <h3 class="sec-title">Uang Muka (DP)</h3>
                <p class="sec-sub">DP yang diminta hanya angka target. Uang yang benar-benar masuk dicatat di bagian <b>Uang Muka &amp; Pelunasan</b> setelah reservasi dibuat.</p>
              </div>
            </header>
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <div>
                <label class="lbl">DP yang diminta</label>
                <div class="flex gap-2">
                  <input :value="dpDisplay" @input="onDpInput" type="text" inputmode="numeric" class="form-input" placeholder="Rp 0" :disabled="locked" />
                  <button v-if="!locked" type="button" class="btn-chip" :disabled="!computedTotal" :title="computedTotal ? `Isi ${settings.dp_percent}% dari total` : 'Pilih menu dulu agar total terhitung'" @click="applyDpPolicy">{{ settings.dp_percent }}%</button>
                </div>
                <p class="hint">Kebijakan: {{ settings.dp_percent }}% dari total<template v-if="computedTotal"> = {{ formatRupiah(policyDp) }}</template>. Tekan tombol persen untuk mengisi otomatis.</p>
              </div>
              <div v-if="!editing">
                <label class="lbl">Status awal</label>
                <select v-model="form.status" class="form-input">
                  <option value="pending">Menunggu DP</option>
                  <option value="confirmed" :disabled="(form.down_payment || 0) > 0">Langsung dikonfirmasi (tanpa DP)</option>
                </select>
                <p class="hint">{{ form.status === 'confirmed' ? 'Tanpa DP: meja dianggap pasti walau belum ada uang masuk.' : 'Otomatis menjadi Dikonfirmasi begitu DP tervalidasi.' }}</p>
              </div>
            </div>
            <div class="sum">
              <div class="sum-row"><span>Total menu</span><b>{{ formatRupiah(computedTotal) }}</b></div>
              <div class="sum-row"><span>DP diminta</span><span>{{ formatRupiah(form.down_payment || 0) }}</span></div>
              <template v-if="editing">
                <div class="sum-row"><span>Tervalidasi</span><span class="text-emerald-700">− {{ formatRupiah(editing.paid_amount) }}</span></div>
                <div v-if="editing.pending_amount > 0" class="sum-row"><span>Menunggu validasi</span><span class="text-amber-700">{{ formatRupiah(editing.pending_amount) }}</span></div>
                <div class="sum-row sum-row--total"><span>Sisa tagihan</span><b class="text-amber-600">{{ formatRupiah(editing.remaining) }}</b></div>
              </template>
              <div v-else class="sum-row sum-row--total"><span>Sisa setelah DP</span><b class="text-amber-600">{{ formatRupiah(Math.max(0, computedTotal - (form.down_payment||0))) }}</b></div>
              <p class="hint">Sisa dilunasi saat tamu datang: lewat POS (metode <i>DP Reservasi</i>) atau dicatat manual di bagian Uang Muka &amp; Pelunasan.</p>
            </div>
          </section>

          <div>
            <label class="lbl">Catatan</label>
            <textarea v-model="form.notes" rows="2" class="form-input" placeholder="Permintaan khusus, nomor meja, dll. (opsional)" :disabled="locked"></textarea>
          </div>
        </form>

        <!-- ── Uang muka & pelunasan (mode detail) ── -->
        <section v-if="editing" class="sec sec--pay">
          <header class="sec-hd">
            <span class="sec-no sec-no--pay">Rp</span>
            <div class="min-w-0">
              <h3 class="sec-title">Uang Muka &amp; Pelunasan</h3>
              <p class="sec-sub">Bukti dari pelanggan menunggu validasi admin. Pembayaran yang dicatat admin langsung tervalidasi.</p>
            </div>
            <button v-if="canUpdate && canRecordPayment && !showPay" type="button" class="btn-chip shrink-0" @click="openPay">+ Catat</button>
          </header>
          <ul v-if="editing.payments?.length" class="divide-y divide-gray-100 border border-gray-200 rounded-lg">
            <li v-for="p in editing.payments" :key="p.id" class="px-3 py-2 text-sm">
              <div class="flex flex-wrap items-center gap-x-2 gap-y-1">
                <span class="font-semibold">{{ PAY_TYPE[p.type] || p.type }}</span>
                <span class="text-gray-800">{{ formatRupiah(p.amount) }}</span>
                <span class="st-badge" :class="payCls(p.status)">{{ PAY_STATUS[p.status] || p.status }}</span>
                <a v-if="p.proof_url" :href="p.proof_url" target="_blank" rel="noopener" class="text-xs text-emerald-700 underline">bukti</a>
              </div>
              <p class="text-xs text-gray-400 mt-0.5">
                {{ p.method }}<template v-if="p.bank_label"> · {{ p.bank_label }}</template> · {{ p.paid_at }}
                <template v-if="p.status === 'pending'"> · dari {{ p.submitted_by }}</template>
                <template v-else-if="p.validated_by"> · oleh {{ p.validated_by }}</template>
              </p>
              <p v-if="p.notes" class="text-xs text-gray-500 mt-0.5">{{ p.notes }}</p>
              <p v-if="p.rejected_reason" class="text-xs text-red-600 mt-0.5">Ditolak: {{ p.rejected_reason }}</p>
              <div v-if="p.status === 'pending' && canUpdate" class="mt-1.5 flex gap-2">
                <button type="button" class="flex-1 sm:flex-none text-xs font-semibold px-3 py-1.5 rounded-lg bg-emerald-600 text-white" :disabled="acting" @click="validatePayment(p)">Validasi</button>
                <button type="button" class="flex-1 sm:flex-none text-xs font-semibold px-3 py-1.5 rounded-lg bg-red-50 text-red-700" :disabled="acting" @click="rejectPayment(p)">Tolak</button>
              </div>
            </li>
          </ul>
          <p v-else class="empty-hint">Belum ada uang masuk yang tercatat.</p>

          <div v-if="showPay" class="rounded-lg border border-emerald-200 bg-emerald-50/40 p-3 space-y-3">
            <p class="text-xs font-bold uppercase tracking-wide text-emerald-800">Catat pembayaran</p>
            <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
              <div>
                <label class="lbl">Jenis</label>
                <select v-model="payForm.type" class="form-input">
                  <option v-if="editing.status !== 'cancelled'" value="dp">DP</option>
                  <option v-if="editing.status !== 'cancelled'" value="pelunasan">Pelunasan</option>
                  <option v-if="editing.status === 'cancelled' && editing.cancel_disposition === 'refund'" value="refund">Refund ke pelanggan</option>
                </select>
              </div>
              <div>
                <label class="lbl">Nominal</label>
                <input v-model.number="payForm.amount" type="number" inputmode="numeric" min="1" class="form-input" />
              </div>
              <div>
                <label class="lbl">Metode</label>
                <select v-model="payForm.method" class="form-input">
                  <option value="transfer">Transfer</option><option value="qris">QRIS</option><option value="cash">Tunai</option><option value="lainnya">Lainnya</option>
                </select>
              </div>
              <div>
                <label class="lbl">Rekening</label>
                <select v-model="payForm.bank_account_id" class="form-input">
                  <option value="">—</option>
                  <option v-for="b in settings.bank_accounts" :key="b.id" :value="b.id">{{ b.bank_name }} {{ b.account_number }}</option>
                </select>
              </div>
              <div>
                <label class="lbl">Tanggal bayar</label>
                <input v-model="payForm.paid_at" type="date" class="form-input" />
              </div>
              <div>
                <label class="lbl">Catatan</label>
                <input v-model="payForm.notes" class="form-input" placeholder="opsional" />
              </div>
            </div>
            <PhotoCapture v-model="payForm.proof_url" label="Bukti pembayaran" :required="payForm.method !== 'cash'"
              hint="Wajib untuk transfer/QRIS. Untuk tunai boleh dikosongkan." />
            <div class="flex flex-col-reverse sm:flex-row sm:justify-end gap-2">
              <button type="button" class="btn-ghost" @click="showPay = false">Batal</button>
              <AppButton :loading="acting" @click="addPayment">Simpan sebagai tervalidasi</AppButton>
            </div>
          </div>
        </section>

        <!-- ── Batalkan: nasib uang muka ── -->
        <div v-if="cancelChoice" class="rounded-xl border border-red-200 bg-red-50 p-3 text-sm space-y-2">
          <p class="font-semibold text-red-800">Sudah ada uang masuk {{ formatRupiah(editing.paid_amount) }}. Apa nasibnya?</p>
          <div class="flex flex-col sm:flex-row sm:flex-wrap gap-2">
            <button type="button" class="text-xs font-semibold px-3 py-2 rounded-lg bg-white border border-red-200 text-red-700" :disabled="acting" @click="setStatus('cancelled', 'refund')">Dikembalikan (catat refund setelah ini)</button>
            <button type="button" class="text-xs font-semibold px-3 py-2 rounded-lg bg-red-600 text-white" :disabled="acting" @click="setStatus('cancelled', 'hangus')">Hangus (jadi pendapatan lain)</button>
            <button type="button" class="text-xs font-semibold px-3 py-2 rounded-lg text-gray-600" @click="cancelChoice = false">Batal</button>
          </div>
        </div>
      </div>

      <template #footer>
        <div class="flex w-full flex-col-reverse gap-2 sm:flex-row sm:items-center">
          <button type="button" class="btn-ghost w-full sm:w-auto sm:mr-auto" @click="modal=false">Tutup</button>
          <div v-if="editing && canUpdate && !locked" class="flex gap-2">
            <button type="button" class="flex-1 sm:flex-none text-xs font-semibold px-3 py-2 rounded-lg bg-red-50 text-red-700" :disabled="acting" @click="askCancel">Batalkan</button>
            <button v-if="editing.status === 'pending'" type="button" class="flex-1 sm:flex-none text-xs font-semibold px-3 py-2 rounded-lg bg-blue-600 text-white disabled:opacity-50" :disabled="acting || !editing.dp_paid" :title="editing.dp_paid ? '' : 'DP belum tervalidasi'" @click="setStatus('confirmed')">Konfirmasi</button>
            <button type="button" class="flex-1 sm:flex-none text-xs font-semibold px-3 py-2 rounded-lg bg-emerald-600 text-white" :disabled="acting" @click="setStatus('done')">Selesai</button>
          </div>
          <AppButton v-if="!locked && (editing ? canUpdate : canCreate)" form="resv-form" type="submit" :loading="saving" class="w-full sm:w-auto">{{ editing ? 'Simpan Perubahan' : 'Buat Reservasi' }}</AppButton>
        </div>
      </template>
    </AppModal>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { reservationsApi } from '@/api/reservations.js'
import { productsApi } from '@/api/products.js'
import { outletsApi } from '@/api/outlets.js'
import { useToastStore } from '@/stores/toast.js'
import { useAuthStore } from '@/stores/auth.js'
import { formatRupiah, formatDateStr, todayDateString } from '@/utils/format.js'
import AppCard from '@/components/ui/AppCard.vue'
import AppTable from '@/components/ui/AppTable.vue'
import AppAlert from '@/components/ui/AppAlert.vue'
import AppButton from '@/components/ui/AppButton.vue'
import AppModal from '@/components/ui/AppModal.vue'
import SearchSelect from '@/components/ui/SearchSelect.vue'
import DateRangePicker from '@/components/ui/DateRangePicker.vue'
import PhotoCapture from '@/components/PhotoCapture.vue'

const toast = useToastStore()
const auth = useAuthStore()
const canCreate = auth.hasPermission('reservations.create')
const canUpdate = auth.hasPermission('reservations.update')
const canDelete = auth.hasPermission('reservations.delete')

// Status mengikuti pembayaran: 'pending' = menunggu DP, 'confirmed' = DP tervalidasi.
const STATUS = { pending: 'Menunggu DP', confirmed: 'Dikonfirmasi', done: 'Selesai', cancelled: 'Dibatalkan' }
const PAY_TYPE = { dp: 'DP', pelunasan: 'Pelunasan', refund: 'Refund' }
const PAY_STATUS = { pending: 'Menunggu validasi', validated: 'Tervalidasi', rejected: 'Ditolak' }
function stCls(s) { return { 'st-pending': s==='pending', 'st-confirmed': s==='confirmed', 'st-done': s==='done', 'st-cancelled': s==='cancelled' } }
function payCls(s) { return { 'st-pending': s==='pending', 'st-done': s==='validated', 'st-cancelled': s==='rejected' } }

const COLUMNS = [
  { key: 'customer_name', label: 'Pemesan' },
  { key: 'outlet_name',   label: 'Outlet' },
  { key: 'pax',           label: 'Tamu' },
  { key: 'schedule',      label: 'Jadwal' },
  { key: 'total',         label: 'Total' },
  { key: 'paid',          label: 'Dibayar' },
  { key: 'remaining',     label: 'Sisa' },
  { key: 'status',        label: 'Status' },
  { key: 'source',        label: 'Sumber' },
  { key: 'actions',       label: '' },
]

const rows = ref([])
const outlets = ref([])
const loading = ref(false)
const errorMsg = ref('')
const filterOutlet = ref('')
const filterStatus = ref('')
const dateFrom = ref('')
const dateTo = ref('')
const range = ref({ from: '', to: '', label: 'Semua Tanggal' })
watch(range, (r) => { dateFrom.value = r.from; dateTo.value = r.to; load() })
const copied = ref(false)

const outletFilterOptions = computed(() => [{ id: '', name: 'Semua outlet' }, ...outlets.value])
const selectedOutletObj = computed(() => outlets.value.find(o => o.id === filterOutlet.value))
const publicUrl = computed(() => selectedOutletObj.value?.slug ? `${window.location.origin}/r/${selectedOutletObj.value.slug}` : '')
const outletsWithSlug = computed(() => outlets.value.filter(o => o.slug))
const copiedId = ref('')
function urlFor(o) { return `${window.location.origin}/r/${o.slug}` }
function statusLinkFor(r) {
  const o = outlets.value.find(x => x.id === r.outlet_id)
  return o?.slug ? `${window.location.origin}/r/${o.slug}?id=${r.id}` : ''
}
async function copyOutlet(o) {
  try { await navigator.clipboard.writeText(urlFor(o)); copiedId.value = o.id; setTimeout(() => { if (copiedId.value === o.id) copiedId.value = '' }, 1500) } catch {}
}

function asArray(d) { return Array.isArray(d) ? d : (d?.data || []) }
function asObject(d) { return d?.data ?? d }

// Kebijakan DP + rekening tujuan (dipakai form catat pembayaran).
const settings = ref({ dp_percent: 50, bank_accounts: [] })
const dpPercentInput = ref(50)
const savingSettings = ref(false)
async function loadSettings() {
  try { settings.value = asObject(await reservationsApi.settings()) || settings.value; dpPercentInput.value = settings.value.dp_percent } catch {}
}
async function saveSettings() {
  savingSettings.value = true
  try { settings.value = asObject(await reservationsApi.updateSettings({ dp_percent: Number(dpPercentInput.value) })); toast.success('Kebijakan DP disimpan') }
  catch (e) { toast.error(e?.message || 'Gagal menyimpan') } finally { savingSettings.value = false }
}

async function load() {
  loading.value = true; errorMsg.value = ''
  try {
    rows.value = asArray(await reservationsApi.list({
      outlet_id: filterOutlet.value || undefined,
      status: filterStatus.value || undefined,
      date_from: dateFrom.value || undefined,
      date_to: dateTo.value || undefined,
    }))
  } catch (e) { errorMsg.value = e?.message || 'Gagal memuat reservasi' } finally { loading.value = false }
}
async function loadOutlets() {
  try { const d = await outletsApi.myOutlets(); outlets.value = d?.outlets ?? d ?? [] } catch { outlets.value = [] }
}
async function copyLink() {
  try { await navigator.clipboard.writeText(publicUrl.value); copied.value = true; setTimeout(()=>copied.value=false, 1500) } catch {}
}

// ── Modal ──
const modal = ref(false)
const editing = ref(null)
const saving = ref(false)
const acting = ref(false)
const form = ref({})
const pickProduct = ref('')
const products = ref([])
const dpDisplay = ref('')
const showPay = ref(false)
const cancelChoice = ref(false)
const payForm = ref({})

// Reservasi selesai/batal dikunci server; layar mengikuti.
const locked = computed(() => !!editing.value && (editing.value.status === 'done' || editing.value.status === 'cancelled'))
const canRecordPayment = computed(() => {
  const e = editing.value
  if (!e) return false
  if (e.status === 'pending' || e.status === 'confirmed') return e.remaining > 0
  return e.status === 'cancelled' && e.cancel_disposition === 'refund' && e.paid_amount > 0
})

function fmtRupiahInput(v) { const d = String(v ?? '').replace(/[^\d]/g, ''); return d ? 'Rp ' + new Intl.NumberFormat('id-ID').format(Number(d)) : '' }
function onDpInput(e) {
  const d = e.target.value.replace(/[^\d]/g, '')
  const num = d ? parseInt(d, 10) : 0
  form.value.down_payment = num
  dpDisplay.value = num ? fmtRupiahInput(num) : ''
  if (num > 0 && form.value.status === 'confirmed') form.value.status = 'pending'
}

const productOptions = computed(() => products.value.map(p => ({ id: p.id, name: `${p.name} — ${formatRupiah(p.price)}` })))
const computedTotal = computed(() => form.value.items?.reduce((s, it) => s + it.price * (it.qty || 0), 0) || 0)
// DP menurut kebijakan (persen dari total menu); tombol "%" mengisinya ke form.
const policyDp = computed(() => Math.round(computedTotal.value * (Number(settings.value.dp_percent) || 0) / 100))
function applyDpPolicy() {
  const num = policyDp.value
  form.value.down_payment = num
  dpDisplay.value = num ? fmtRupiahInput(num) : ''
  if (num > 0 && form.value.status === 'confirmed') form.value.status = 'pending'
}
function statusHint(r) {
  switch (r.status) {
    case 'pending':   return r.pending_amount > 0 ? 'Ada bukti pembayaran menunggu validasi di bawah.' : 'Menunggu DP. Pelanggan mengunggah bukti lewat link status; admin memvalidasinya di bagian Uang Muka & Pelunasan.'
    case 'confirmed': return 'DP tervalidasi, meja dipastikan. Sisa dibayar saat tamu datang.'
    case 'done':      return 'Tamu sudah dilayani; reservasi dikunci.'
    case 'cancelled': return 'Reservasi dibatalkan dan dikunci.'
    default:          return ''
  }
}

function blank() { return { outlet_id: filterOutlet.value || '', customer_name: '', customer_phone: '', pax: 1, reservation_date: '', reservation_time: '', items: [], down_payment: 0, status: 'pending', notes: '' } }

async function loadProducts(outletId) {
  products.value = []
  if (!outletId) return
  try { const res = await productsApi.listProducts({ outlet_id: outletId, limit: 500 }); products.value = res.items ?? [] } catch { products.value = [] }
}
function onOutletChange() { loadProducts(form.value.outlet_id) }
function addProduct() {
  const p = products.value.find(x => x.id === pickProduct.value)
  pickProduct.value = ''
  if (!p) return
  const ex = form.value.items.find(i => i.product_id === p.id)
  if (ex) ex.qty++
  else form.value.items.push({ product_id: p.id, product_name: p.name, price: p.price, qty: 1 })
}

async function openCreate() {
  editing.value = null; form.value = blank(); pickProduct.value = ''; dpDisplay.value = ''
  showPay.value = false; cancelChoice.value = false
  await loadProducts(form.value.outlet_id)
  modal.value = true
}
function fillForm(r) {
  form.value = { outlet_id: r.outlet_id, customer_name: r.customer_name, customer_phone: r.customer_phone, pax: r.pax, reservation_date: r.reservation_date || '', reservation_time: r.reservation_time || '', items: (r.items||[]).map(i=>({...i})), down_payment: r.down_payment, status: r.status, notes: r.notes }
  dpDisplay.value = r.down_payment ? fmtRupiahInput(r.down_payment) : ''
}
async function openEdit(r) {
  showPay.value = false; cancelChoice.value = false
  // Detail membawa riwayat pembayaran; baris daftar tidak.
  try { editing.value = asObject(await reservationsApi.get(r.id)) } catch { editing.value = r }
  fillForm(editing.value)
  await loadProducts(editing.value.outlet_id)
  modal.value = true
}
async function refreshDetail() {
  if (!editing.value) return
  editing.value = asObject(await reservationsApi.get(editing.value.id))
  fillForm(editing.value)
  await load()
}
async function save() {
  if (!form.value.customer_name?.trim()) { toast.error('Nama pemesan wajib diisi'); return }
  if (!editing.value && !form.value.outlet_id) { toast.error('Pilih outlet'); return }
  saving.value = true
  try {
    const payload = { ...form.value, items: form.value.items.map(i => ({ product_id: i.product_id, qty: i.qty })) }
    if (editing.value) { await reservationsApi.update(editing.value.id, payload); await refreshDetail(); toast.success('Reservasi diperbarui') }
    else { await reservationsApi.create(payload); toast.success('Reservasi dibuat'); modal.value = false; await load() }
  } catch (e) { toast.error(e?.message || 'Gagal menyimpan') } finally { saving.value = false }
}

// ── Uang muka ──
function openPay() {
  const e = editing.value
  const isRefund = e.status === 'cancelled'
  const dpDue = Math.max((e.down_payment || 0) - (e.paid_amount || 0), 0)
  payForm.value = {
    type: isRefund ? 'refund' : (dpDue > 0 ? 'dp' : 'pelunasan'),
    amount: isRefund ? e.paid_amount : (dpDue > 0 ? Math.min(dpDue, e.remaining) : Math.max(e.remaining - e.pending_amount, 0)),
    method: 'transfer', bank_account_id: settings.value.bank_accounts?.[0]?.id || '',
    paid_at: todayDateString(), proof_url: '', notes: '',
  }
  showPay.value = true
}
async function addPayment() {
  if (!(payForm.value.amount > 0)) { toast.error('Nominal harus lebih dari 0'); return }
  if (payForm.value.method !== 'cash' && !payForm.value.proof_url) { toast.error('Bukti pembayaran wajib untuk transfer/QRIS'); return }
  acting.value = true
  try {
    await reservationsApi.addPayment(editing.value.id, payForm.value)
    showPay.value = false
    await refreshDetail()
    toast.success('Pembayaran dicatat')
  } catch (e) { toast.error(e?.message || 'Gagal mencatat pembayaran') } finally { acting.value = false }
}
async function validatePayment(p) {
  acting.value = true
  try { await reservationsApi.validatePayment(editing.value.id, p.id); await refreshDetail(); toast.success('Pembayaran tervalidasi') }
  catch (e) { toast.error(e?.message || 'Gagal memvalidasi') } finally { acting.value = false }
}
async function rejectPayment(p) {
  const reason = window.prompt('Alasan penolakan (dibaca pelanggan):') || ''
  if (!reason.trim()) return
  acting.value = true
  try { await reservationsApi.rejectPayment(editing.value.id, p.id, reason); await refreshDetail(); toast.success('Bukti ditolak') }
  catch (e) { toast.error(e?.message || 'Gagal menolak') } finally { acting.value = false }
}

// ── Status ──
function askCancel() {
  if (editing.value.paid_amount > 0) { cancelChoice.value = true; return }
  if (window.confirm('Batalkan reservasi ini?')) setStatus('cancelled')
}
async function setStatus(status, disposition = '') {
  if (status === 'done' && !window.confirm('Tandai selesai (tamu sudah dilayani)? Biasanya ini ditutup otomatis oleh transaksi POS.')) return
  acting.value = true
  try {
    await reservationsApi.setStatus(editing.value.id, status, disposition)
    cancelChoice.value = false
    await refreshDetail()
    toast.success('Status diperbarui')
  } catch (e) { toast.error(e?.message || 'Gagal mengubah status') } finally { acting.value = false }
}

async function confirmDelete(r) {
  if (!window.confirm(`Hapus reservasi "${r.customer_name}"?`)) return
  try { await reservationsApi.remove(r.id); toast.success('Reservasi dihapus'); await load() }
  catch (e) { toast.error(e?.message || 'Gagal menghapus') }
}

onMounted(async () => { await loadOutlets(); await Promise.all([load(), loadSettings()]) })
</script>

<style scoped>
.form-input { width: 100%; padding: .5rem .7rem; border-radius: .6rem; font-size: .85rem; border: 1px solid rgba(0,0,0,.14); background: #fff; color: #111827; outline: none; }
.form-input:focus { border-color: rgba(5,150,105,.5); box-shadow: 0 0 0 3px rgba(5,150,105,.12); }
.form-input:disabled { background: #f9fafb; color: #6b7280; }
.lbl { display: block; font-size: .72rem; font-weight: 700; color: #4b5563; margin-bottom: .25rem; }
.btn-ghost { padding: .5rem 1rem; border-radius: .6rem; font-size: .85rem; font-weight: 600; color: #374151; background: #f3f4f6; }
.btn-ghost:hover { background: #e5e7eb; }

/* Bagian bernomor di modal */
.sec { display: flex; flex-direction: column; gap: .75rem; padding: .75rem; border: 1px solid #e5e7eb; border-radius: .75rem; background: #fff; }
@media (min-width: 640px) { .sec { padding: 1rem; } }
.sec--pay { border-color: #a7f3d0; background: #f0fdf4; }
.sec-hd { display: flex; align-items: flex-start; gap: .6rem; }
.sec-no { flex-shrink: 0; width: 1.5rem; height: 1.5rem; border-radius: 999px; background: #ecfdf5; color: #047857; font-size: .72rem; font-weight: 800; display: inline-flex; align-items: center; justify-content: center; }
.sec-no--pay { width: auto; padding: 0 .5rem; background: #d1fae5; }
.sec-title { margin: 0; font-size: .82rem; font-weight: 700; color: #111827; }
.sec-sub { margin: .1rem 0 0; font-size: .7rem; line-height: 1.4; color: #6b7280; }
.sec-badge { margin-left: auto; flex-shrink: 0; padding: .15rem .5rem; border-radius: 999px; background: #f3f4f6; color: #374151; font-size: .66rem; font-weight: 700; white-space: nowrap; }
.hint { margin-top: .3rem; font-size: .7rem; line-height: 1.4; color: #6b7280; }
.empty-hint { padding: .6rem .75rem; border: 1px dashed #e5e7eb; border-radius: .5rem; background: #f9fafb; color: #9ca3af; font-size: .75rem; text-align: center; }
.btn-chip { flex-shrink: 0; padding: .35rem .6rem; border-radius: .6rem; border: 1px solid #a7f3d0; background: #ecfdf5; color: #047857; font-size: .75rem; font-weight: 700; white-space: nowrap; }
.btn-chip:hover:not(:disabled) { background: #d1fae5; }
.btn-chip:disabled { opacity: .5; cursor: not-allowed; }
.sum { display: flex; flex-direction: column; gap: .25rem; padding: .6rem .75rem; border-radius: .6rem; background: #f9fafb; font-size: .85rem; }
.sum-row { display: flex; justify-content: space-between; gap: .5rem; color: #4b5563; }
.sum-row--total { margin-top: .15rem; padding-top: .3rem; border-top: 1px solid #e5e7eb; color: #111827; font-weight: 600; }
.meta-chip { padding: .12rem .5rem; border: 1px solid #e5e7eb; border-radius: 999px; background: #fff; color: #374151; font-size: .68rem; font-weight: 600; }
.meta-chip--red { border-color: #fecaca; background: #fef2f2; color: #b91c1c; }
.st-badge { display: inline-block; padding: .12rem .5rem; border-radius: 999px; font-size: .68rem; font-weight: 700; white-space: nowrap; }
.st-pending { background: rgba(245,158,11,.15); color: #b45309; }
.st-confirmed { background: rgba(59,130,246,.13); color: #1d4ed8; }
.st-done { background: rgba(16,185,129,.14); color: #047857; }
.st-cancelled { background: rgba(239,68,68,.13); color: #b91c1c; }
</style>
