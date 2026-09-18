<template>
  <div class="rp">
    <!-- Atmospheric ornaments (seperti Login) -->
    <div class="rp-atmo" aria-hidden="true">
      <div class="orb o1" /><div class="orb o2" /><div class="orb o3" />
      <svg class="topo" viewBox="0 0 800 800" fill="none" preserveAspectRatio="xMidYMid slice">
        <g stroke="rgba(126,184,154,.06)" stroke-width="1" fill="none">
          <ellipse cx="400" cy="400" rx="140" ry="90" /><ellipse cx="400" cy="400" rx="285" ry="196" />
          <ellipse cx="400" cy="400" rx="450" ry="325" /><ellipse cx="400" cy="400" rx="636" ry="476" />
        </g>
      </svg>
    </div>

    <header class="rp-head">
      <div class="rp-head-in">
        <div class="rp-brand">
          <div class="rp-logo">
            <svg viewBox="0 0 32 32" fill="none"><rect x="2" y="2" width="13" height="13" rx="3" fill="rgba(126,184,154,.9)"/><rect x="17" y="2" width="13" height="13" rx="3" fill="rgba(126,184,154,.55)"/><rect x="2" y="17" width="13" height="13" rx="3" fill="rgba(126,184,154,.55)"/><rect x="17" y="17" width="13" height="13" rx="3" fill="rgba(126,184,154,.75)"/></svg>
          </div>
          <div>
            <p class="rp-eyebrow">Reservasi Online</p>
            <h1 class="rp-title">{{ status?.outlet_name || menu?.outlet_name || 'Memuat…' }}</h1>
          </div>
        </div>
      </div>
    </header>

    <div v-if="loading" class="rp-state">Memuat menu…</div>
    <div v-else-if="loadError" class="rp-state">{{ loadError }}</div>

    <!-- Status reservasi: dipakai setelah kirim DAN saat dibuka lewat link ?id= -->
    <div v-else-if="status" class="rp-wrap">
      <div class="glass rp-success">
        <div class="rp-check"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6L9 17l-5-5"/></svg></div>
        <h2>{{ justCreated ? 'Reservasi terkirim!' : 'Status Reservasi' }}</h2>
        <p v-if="justCreated">Terima kasih, {{ status.customer_name }}. Simpan tautan di bawah untuk memantau reservasi dan mengunggah bukti DP.</p>
        <p v-else>Halo {{ status.customer_name }}, ini keadaan reservasi Anda.</p>
        <span :class="['rp-badge', 'rp-badge--' + status.status]">{{ STATUS_LABEL[status.status] || status.status }}</span>
        <div class="rp-recap">
          <div><span>Jadwal</span><b>{{ status.reservation_date ? fmtDate(status.reservation_date) : '-' }} {{ status.reservation_time }}</b></div>
          <div><span>Tamu</span><b>{{ status.pax }} orang</b></div>
          <div><span>Total</span><b>{{ rupiah(status.total) }}</b></div>
          <div v-if="status.down_payment > 0"><span>DP diminta</span><b>{{ rupiah(status.down_payment) }}</b></div>
          <div><span>Sudah tervalidasi</span><b>{{ rupiah(status.paid_amount) }}</b></div>
          <div v-if="status.pending_amount > 0"><span>Menunggu validasi</span><b>{{ rupiah(status.pending_amount) }}</b></div>
          <div><span>Sisa</span><b>{{ rupiah(status.remaining) }}</b></div>
        </div>
        <div class="rp-link">
          <span>Tautan cek status</span>
          <input :value="statusLink" readonly @focus="$event.target.select()" />
          <button type="button" class="rp-btn-ghost" @click="copyStatusLink">{{ copied ? 'Tersalin' : 'Salin' }}</button>
        </div>
      </div>

      <!-- Bayar DP / pelunasan: bukti lahir 'menunggu validasi' sampai admin mengesahkannya. -->
      <section v-if="canPay" class="glass rp-pay">
        <h3 class="rp-section">{{ status.dp_paid ? 'Pelunasan' : 'Bayar DP' }}</h3>
        <p class="rp-pay-hint">
          <template v-if="!status.dp_paid">Reservasi dikonfirmasi setelah DP <b>{{ rupiah(dpDue) }}</b> kami terima dan validasi.</template>
          <template v-else>Sisa <b>{{ rupiah(status.remaining) }}</b> bisa dilunasi sekarang atau di kasir saat datang.</template>
        </p>
        <div v-if="status.bank_accounts?.length" class="rp-banks">
          <div v-for="b in status.bank_accounts" :key="b.id" class="rp-bank">
            <b>{{ b.bank_name }}</b><span>{{ b.account_number }}</span><small>a.n. {{ b.account_holder }}</small>
          </div>
        </div>
        <p v-else class="rp-pay-hint">Hubungi kami untuk rekening tujuan, lalu unggah buktinya di sini.</p>
        <div class="rp-fields">
          <div><label>Nominal transfer <span>*</span></label><input v-model.number="pay.amount" type="number" inputmode="numeric" min="1" /></div>
          <div v-if="status.bank_accounts?.length"><label>Transfer ke rekening</label>
            <select v-model="pay.bank_account_id"><option value="">— pilih —</option><option v-for="b in status.bank_accounts" :key="b.id" :value="b.id">{{ b.bank_name }} {{ b.account_number }}</option></select></div>
          <div><label>Tanggal transfer</label><input v-model="pay.paid_at" type="date" /></div>
          <div><label>Foto bukti transfer <span>*</span></label><input type="file" accept="image/*" @change="onProof" /></div>
          <div><label>Catatan</label><input v-model="pay.notes" placeholder="opsional" /></div>
        </div>
        <p v-if="payError" class="rp-err">{{ payError }}</p>
        <p v-if="payMsg" class="rp-ok">{{ payMsg }}</p>
        <button class="rp-next" :disabled="paySubmitting" @click="submitPay">{{ paySubmitting ? 'Mengirim…' : 'Kirim Bukti Pembayaran' }}</button>
      </section>

      <section v-if="status.payments?.length" class="glass rp-pay">
        <h3 class="rp-section">Riwayat Pembayaran</h3>
        <ul class="rp-order-list">
          <li v-for="p in status.payments" :key="p.id">
            <span class="rp-oi-name">{{ PAY_TYPE[p.type] || p.type }} · {{ p.paid_at }}</span>
            <span :class="['rp-badge', 'rp-badge--' + p.status]">{{ PAY_STATUS[p.status] || p.status }}</span>
            <span class="rp-oi-sub">{{ rupiah(p.amount) }}</span>
          </li>
        </ul>
        <p v-for="p in status.payments.filter(x => x.status === 'rejected' && x.rejected_reason)" :key="'r' + p.id" class="rp-err">Ditolak: {{ p.rejected_reason }}</p>
      </section>

      <div class="rp-actions"><button class="rp-btn-ghost" @click="resetAll">Buat reservasi lain</button></div>
    </div>

    <template v-else-if="menu">
      <!-- STEP 1: Data Reservasi -->
      <div v-if="step === 1" class="rp-wrap">
        <section class="glass rp-form">
          <h3 class="rp-section">Data Reservasi</h3>
          <div class="rp-fields">
            <div><label>Nama <span>*</span></label><input v-model="f.customer_name" placeholder="Nama Anda" /></div>
            <div><label>No. HP / WhatsApp</label><input v-model="f.customer_phone" placeholder="08xxxx" inputmode="tel" /></div>
            <div class="rp-row3">
              <div><label>Tamu</label><input v-model.number="f.pax" type="number" min="1" /></div>
              <div><label>Tanggal</label><input v-model="f.reservation_date" type="date" /></div>
              <div><label>Jam</label><input v-model="f.reservation_time" type="time" /></div>
            </div>
            <div><label>Catatan</label><textarea v-model="f.notes" rows="2" placeholder="Permintaan khusus (opsional)"></textarea></div>
          </div>
          <p v-if="formError" class="rp-err">{{ formError }}</p>
          <button type="button" class="rp-next" @click="goMenu">Pilih Menu →</button>
        </section>
      </div>

      <!-- STEP 2: Pilih Menu -->
      <template v-else>
        <!-- Category chips (sticky) -->
        <div class="rp-cats">
          <div class="rp-cats-in">
            <button :class="['rp-chip', activeCat === '' && 'rp-chip--on']" @click="activeCat = ''">Semua <span class="rp-chip-n">{{ totalProducts }}</span></button>
            <button v-for="c in menu.categories" :key="c.name" :class="['rp-chip', activeCat === c.name && 'rp-chip--on']" @click="activeCat = c.name">{{ c.name }} <span class="rp-chip-n">{{ c.products.length }}</span></button>
          </div>
        </div>

        <div class="rp-wrap">
          <button type="button" class="rp-back" @click="step = 1">← Ubah data reservasi</button>

          <!-- Search -->
          <div class="rp-search glass">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>
            <input v-model="searchQuery" type="search" placeholder="Cari produk…" />
          </div>

          <!-- Flat product grid (no category headers) -->
          <div v-if="!filteredProducts.length" class="rp-empty">Tidak ada menu yang cocok.</div>
          <div v-else class="rp-grid">
            <article v-for="p in filteredProducts" :key="p.id" :class="['glass rp-card', qty[p.id] && 'rp-card--active']">
              <div class="rp-photo">
                <img v-if="p.photo_url" :src="p.photo_url" :alt="p.name" loading="lazy" />
                <div v-else class="rp-initials" :style="initialStyle(p.name)">{{ initials(p.name) }}</div>
                <span v-if="qty[p.id]" class="rp-qty-badge">{{ qty[p.id] }}</span>
              </div>
              <div class="rp-card-body">
                <p class="rp-name">{{ p.name }}</p>
                <p class="rp-price">{{ rupiah(p.price) }}</p>
                <div v-if="!qty[p.id]" class="rp-add" @click="inc(p)">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg> Tambah
                </div>
                <div v-else class="rp-stepper">
                  <button type="button" @click="dec(p)">−</button><span>{{ qty[p.id] }}</span><button type="button" @click="inc(p)">+</button>
                </div>
              </div>
            </article>
          </div>

          <!-- Order summary -->
          <section v-if="cartItems.length" class="glass rp-order">
            <h3 class="rp-section">Pesanan Anda</h3>
            <ul class="rp-order-list">
              <li v-for="it in cartItems" :key="it.id">
                <span class="rp-oi-name">{{ it.name }}</span>
                <span class="rp-oi-qty">{{ it.qty }}×</span>
                <span class="rp-oi-sub">{{ rupiah(it.price * it.qty) }}</span>
                <button class="rp-oi-del" @click="remove(it.id)">×</button>
              </li>
            </ul>
            <div class="rp-order-total"><span>Total</span><b>{{ rupiah(total) }}</b></div>
          </section>
        </div>
      </template>
    </template>

    <!-- Sticky summary bar (step 2 only) -->
    <div v-if="menu && !status && step === 2" class="rp-bar">
      <div class="rp-bar-in">
        <div class="rp-bar-info"><span class="rp-bar-count">{{ itemCount }} item</span><span class="rp-bar-total">{{ rupiah(total) }}</span></div>
        <button class="rp-btn" :disabled="submitting" @click="submit">{{ submitting ? 'Mengirim…' : 'Kirim Reservasi' }}</button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { publicApi } from '@/api/public.js'

const route = useRoute()
const slug = route.params.slug

const menu = ref(null)
const loading = ref(true)
const loadError = ref('')
const qty = reactive({})
const submitting = ref(false)
const formError = ref('')
// status = keadaan reservasi dari server (setelah kirim, atau dibuka lewat ?id=).
const status = ref(null)
const justCreated = ref(false)
const copied = ref(false)
const pay = reactive({ amount: 0, bank_account_id: '', paid_at: new Date().toISOString().slice(0, 10), notes: '', file: null })
const paySubmitting = ref(false)
const payError = ref('')
const payMsg = ref('')
const STATUS_LABEL = { pending: 'Menunggu DP', confirmed: 'Dikonfirmasi', done: 'Selesai', cancelled: 'Dibatalkan' }
const PAY_TYPE = { dp: 'DP', pelunasan: 'Pelunasan', refund: 'Refund' }
const PAY_STATUS = { pending: 'Menunggu validasi', validated: 'Tervalidasi', rejected: 'Ditolak' }
const statusLink = computed(() => status.value ? `${window.location.origin}/r/${slug}?id=${status.value.id}` : '')
const dpDue = computed(() => Math.max((status.value?.down_payment || 0) - (status.value?.paid_amount || 0), 0))
const canPay = computed(() => !!status.value && ['pending', 'confirmed'].includes(status.value.status)
  && (status.value.remaining - status.value.pending_amount) > 0)
const activeCat = ref('')
const searchQuery = ref('')
const step = ref(1) // 1 = isi data, 2 = pilih menu

const f = reactive({ customer_name: '', customer_phone: '', pax: 2, reservation_date: '', reservation_time: '', notes: '' })

const allProducts = computed(() => menu.value?.categories.flatMap(c => c.products) || [])
const totalProducts = computed(() => allProducts.value.length)
const baseProducts = computed(() => activeCat.value ? (menu.value.categories.find(c => c.name === activeCat.value)?.products || []) : allProducts.value)
const filteredProducts = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  return [...baseProducts.value]
    .filter(p => !q || p.name.toLowerCase().includes(q))
    .sort((a, b) => b.price - a.price) // termahal lebih dulu
})
const nameById = computed(() => Object.fromEntries(allProducts.value.map(p => [p.id, p.name])))
const priceById = computed(() => Object.fromEntries(allProducts.value.map(p => [p.id, p.price])))
const itemCount = computed(() => Object.values(qty).reduce((s, q) => s + (q || 0), 0))
const total = computed(() => Object.entries(qty).reduce((s, [id, q]) => s + (priceById.value[id] || 0) * (q || 0), 0))
const cartItems = computed(() => Object.entries(qty).filter(([, q]) => q > 0).map(([id, q]) => ({ id, name: nameById.value[id], price: priceById.value[id], qty: q })))

const PALETTE = [
  ['rgba(126,184,154,.25)', '#cdeede'], ['rgba(96,165,250,.22)', '#cfe4ff'], ['rgba(251,191,36,.22)', '#ffe9b0'],
  ['rgba(244,114,182,.22)', '#ffd6ec'], ['rgba(167,139,250,.22)', '#e4ddff'], ['rgba(45,212,191,.22)', '#bff5ec'],
]
function initials(name) { return (name || '?').trim().slice(0, 2).toUpperCase() }
function initialStyle(name) {
  let h = 0
  for (let i = 0; i < (name || '').length; i++) h = (h * 31 + name.charCodeAt(i)) >>> 0
  const [bg, fg] = PALETTE[h % PALETTE.length]
  return { background: bg, color: fg }
}

function rupiah(v) { return 'Rp ' + new Intl.NumberFormat('id-ID').format(Math.round(v || 0)) }
function fmtDate(s) { try { return new Date(s).toLocaleDateString('id-ID', { day: '2-digit', month: 'short', year: 'numeric' }) } catch { return s } }
function inc(p) { qty[p.id] = (qty[p.id] || 0) + 1 }
function dec(p) { if (qty[p.id] > 1) qty[p.id]--; else delete qty[p.id] }
function remove(id) { delete qty[id] }

async function loadMenu() {
  loading.value = true; loadError.value = ''
  try {
    const res = await publicApi.menu(slug)
    if (!res.success) throw new Error(res.error || 'Gagal')
    menu.value = res.data
    document.title = `Reservasi — ${res.data.outlet_name}`
  } catch (e) {
    loadError.value = e?.response?.data?.error || 'Outlet tidak ditemukan.'
  } finally { loading.value = false }
}
function goMenu() {
  formError.value = ''
  if (!f.customer_name.trim()) { formError.value = 'Nama wajib diisi dulu.'; return }
  step.value = 2
  window.scrollTo({ top: 0, behavior: 'smooth' })
}
async function loadStatus(id, created = false) {
  const res = await publicApi.status(slug, id)
  if (!res.success) throw new Error(res.error || 'Gagal')
  status.value = res.data
  justCreated.value = created
  pay.amount = status.value.dp_paid ? Math.max(status.value.remaining - status.value.pending_amount, 0) : dpDue.value
  pay.bank_account_id = status.value.bank_accounts?.[0]?.id || ''
  pay.file = null
  // Alamat halaman menjadi tautan status supaya bisa disimpan/dibagikan.
  window.history.replaceState(null, '', `/r/${slug}?id=${id}`)
}
function onProof(e) { pay.file = e.target.files?.[0] || null }
async function submitPay() {
  payError.value = ''; payMsg.value = ''
  if (!(pay.amount > 0)) { payError.value = 'Isi nominal yang ditransfer.'; return }
  if (!pay.file) { payError.value = 'Unggah foto bukti transfer.'; return }
  const fd = new FormData()
  fd.append('file', pay.file)
  fd.append('amount', String(pay.amount))
  fd.append('bank_account_id', pay.bank_account_id || '')
  fd.append('paid_at', pay.paid_at || '')
  fd.append('notes', pay.notes || '')
  paySubmitting.value = true
  try {
    const res = await publicApi.pay(slug, status.value.id, fd)
    if (!res.success) throw new Error(res.error || 'Gagal')
    payMsg.value = res.message || 'Bukti diterima, menunggu validasi.'
    await loadStatus(status.value.id)
  } catch (e) {
    payError.value = e?.response?.data?.error || e?.message || 'Gagal mengirim bukti.'
  } finally { paySubmitting.value = false }
}
async function copyStatusLink() {
  try { await navigator.clipboard.writeText(statusLink.value); copied.value = true; setTimeout(() => copied.value = false, 1500) } catch {}
}

async function submit() {
  formError.value = ''
  if (!f.customer_name.trim()) { step.value = 1; formError.value = 'Nama wajib diisi.'; return }
  const items = cartItems.value.map(it => ({ product_id: it.id, qty: it.qty }))
  if (!items.length) { formError.value = 'Pilih minimal satu menu.'; return }
  submitting.value = true
  try {
    const res = await publicApi.reserve(slug, { ...f, items })
    if (!res.success) throw new Error(res.error || 'Gagal')
    await loadStatus(res.data.id, true)
    window.scrollTo({ top: 0, behavior: 'smooth' })
  } catch (e) {
    formError.value = e?.response?.data?.error || 'Gagal mengirim reservasi.'
  } finally { submitting.value = false }
}
function resetAll() {
  status.value = null; justCreated.value = false; payMsg.value = ''; payError.value = ''
  window.history.replaceState(null, '', `/r/${slug}`)
  Object.keys(qty).forEach(k => delete qty[k])
  f.customer_name = ''; f.customer_phone = ''; f.pax = 2; f.reservation_date = ''; f.reservation_time = ''; f.notes = ''
  activeCat.value = ''; searchQuery.value = ''; step.value = 1
}

onMounted(async () => {
  await loadMenu()
  // Dibuka lewat tautan status (?id=): langsung tampilkan keadaan reservasi.
  const id = route.query.id
  if (id && !loadError.value) {
    try { await loadStatus(String(id)) } catch { loadError.value = 'Reservasi tidak ditemukan.' }
  }
})
</script>

<style scoped>
.rp { position: relative; min-height: 100vh; overflow: hidden; padding-bottom: 5.5rem; color: rgba(255,255,255,.92);
  background:
    radial-gradient(ellipse 110% 70% at 20% 100%, #253f2d 0%, transparent 55%),
    radial-gradient(ellipse 90% 60% at 85% 0%, #1e3427 0%, transparent 55%),
    radial-gradient(ellipse 70% 50% at 50% 50%, #1e3228 0%, transparent 60%),
    #182b20;
}
.rp-atmo { position: absolute; inset: 0; pointer-events: none; overflow: hidden; }
.orb { position: absolute; border-radius: 50%; filter: blur(80px); }
.o1 { width: 560px; height: 480px; top: -120px; left: -80px; background: radial-gradient(circle at 40% 40%, rgba(74,130,100,.16), rgba(50,100,78,.04)); }
.o2 { width: 420px; height: 420px; bottom: -100px; right: -60px; background: radial-gradient(circle at 60% 60%, rgba(126,184,154,.13), rgba(50,100,78,.03)); }
.o3 { width: 280px; height: 280px; top: 45%; left: 38%; background: radial-gradient(circle, rgba(120,175,145,.09), transparent 70%); }
.topo { position: absolute; inset: 0; width: 100%; height: 100%; opacity: .6; }

/* Glass surface */
.glass {
  background: linear-gradient(145deg, rgba(255,255,255,.12), rgba(255,255,255,.04)), rgba(126,184,154,.06);
  backdrop-filter: blur(22px) saturate(150%); -webkit-backdrop-filter: blur(22px) saturate(150%);
  border: 1px solid rgba(255,255,255,.16);
  box-shadow: 0 12px 36px rgba(0,0,0,.32), inset 0 1px 0 rgba(255,255,255,.18);
}

.rp-head { position: relative; }
.rp-head-in { max-width: 760px; margin: 0 auto; padding: 1.4rem 1.1rem 1.2rem; }
.rp-brand { display: flex; align-items: center; gap: .8rem; }
.rp-logo { width: 2.6rem; height: 2.6rem; border-radius: .7rem; display: flex; align-items: center; justify-content: center; background: rgba(126,184,154,.12); border: 1px solid rgba(255,255,255,.14); }
.rp-logo svg { width: 1.5rem; height: 1.5rem; }
.rp-eyebrow { font-size: .66rem; font-weight: 800; letter-spacing: .14em; text-transform: uppercase; color: rgba(168,203,191,.8); }
.rp-title { font-size: 1.45rem; font-weight: 900; line-height: 1.1; letter-spacing: -.02em; }
.rp-state { position: relative; max-width: 760px; margin: 2rem auto; text-align: center; color: rgba(255,255,255,.6); padding: 0 1rem; }

/* Category chips */
.rp-cats { position: sticky; top: 0; z-index: 20; background: rgba(24,43,32,.72); backdrop-filter: blur(14px); border-bottom: 1px solid rgba(255,255,255,.08); }
.rp-cats-in { max-width: 760px; margin: 0 auto; padding: .7rem 1.1rem; display: flex; gap: .5rem; overflow-x: auto; scrollbar-width: none; }
.rp-cats-in::-webkit-scrollbar { display: none; }
.rp-chip { flex-shrink: 0; display: inline-flex; align-items: center; gap: .35rem; padding: .45rem .85rem; border-radius: 999px; border: 1px solid rgba(255,255,255,.16); background: rgba(255,255,255,.06); font-size: .82rem; font-weight: 600; color: rgba(255,255,255,.78); cursor: pointer; transition: all .15s; }
.rp-chip--on { background: rgba(126,184,154,.9); color: #14271d; border-color: rgba(126,184,154,.9); box-shadow: 0 3px 12px rgba(126,184,154,.3); }
.rp-chip-n { font-size: .67rem; font-weight: 700; background: rgba(255,255,255,.14); border-radius: 999px; padding: .02rem .4rem; }
.rp-chip--on .rp-chip-n { background: rgba(20,39,29,.25); }

.rp-wrap { position: relative; max-width: 760px; margin: 0 auto; padding: 1.1rem; }
.rp-section { font-size: 1rem; font-weight: 800; margin: 0 0 .8rem; color: #fff; }
.rp-back { display: inline-flex; align-items: center; gap: .3rem; background: none; border: none; color: #9fd4ba; font-size: .85rem; font-weight: 700; cursor: pointer; padding: 0; margin-bottom: 1rem; }

/* Search */
.rp-search { display: flex; align-items: center; gap: .55rem; padding: .65rem .85rem; border-radius: .85rem; margin-bottom: 1.1rem; }
.rp-search svg { width: 1.1rem; height: 1.1rem; color: rgba(168,203,191,.8); flex-shrink: 0; }
.rp-search input { flex: 1; background: transparent; border: none; outline: none; color: #fff; font-size: .9rem; }
.rp-search input::placeholder { color: rgba(255,255,255,.4); }

.rp-empty { text-align: center; color: rgba(255,255,255,.5); padding: 2rem 0; font-size: .9rem; }
.rp-grid { display: grid; grid-template-columns: repeat(2, 1fr); gap: .8rem; }
@media (min-width: 560px) { .rp-grid { grid-template-columns: repeat(3, 1fr); } }
.rp-card { border-radius: 1rem; overflow: hidden; display: flex; flex-direction: column; transition: box-shadow .15s, border-color .15s; }
.rp-card--active { border-color: rgba(126,184,154,.7); box-shadow: 0 8px 22px rgba(126,184,154,.22), inset 0 1px 0 rgba(255,255,255,.2); }
.rp-photo { position: relative; aspect-ratio: 1/1; background: rgba(255,255,255,.05); }
.rp-photo img { width: 100%; height: 100%; object-fit: cover; }
.rp-initials { width: 100%; height: 100%; display: flex; align-items: center; justify-content: center; font-size: 2.2rem; font-weight: 900; letter-spacing: .03em; }
.rp-qty-badge { position: absolute; top: .5rem; right: .5rem; min-width: 1.5rem; height: 1.5rem; padding: 0 .4rem; border-radius: 999px; background: #7eb89a; color: #14271d; font-size: .8rem; font-weight: 800; display: flex; align-items: center; justify-content: center; box-shadow: 0 2px 6px rgba(0,0,0,.3); }
.rp-card-body { padding: .6rem .65rem .65rem; display: flex; flex-direction: column; gap: .2rem; flex: 1; }
.rp-name { font-size: .82rem; font-weight: 600; line-height: 1.25; color: rgba(255,255,255,.92); }
.rp-price { font-size: .85rem; font-weight: 800; color: #9fd4ba; margin-top: auto; }
.rp-add { margin-top: .45rem; display: flex; align-items: center; justify-content: center; gap: .25rem; padding: .42rem; border-radius: .6rem; background: rgba(126,184,154,.18); color: #cdeede; font-size: .8rem; font-weight: 700; cursor: pointer; user-select: none; }
.rp-add:hover { background: rgba(126,184,154,.28); }
.rp-add svg { width: .9rem; height: .9rem; }
.rp-stepper { margin-top: .45rem; display: flex; align-items: center; justify-content: space-between; background: rgba(0,0,0,.2); border-radius: .6rem; padding: .15rem; }
.rp-stepper button { width: 1.9rem; height: 1.9rem; border: none; background: rgba(255,255,255,.12); border-radius: .45rem; font-size: 1.1rem; font-weight: 700; color: #cdeede; cursor: pointer; }
.rp-stepper span { font-size: .9rem; font-weight: 700; min-width: 1.5rem; text-align: center; }

/* Order summary */
.rp-order { border-radius: 1rem; padding: 1.1rem; margin-top: 1.1rem; }
.rp-order-list { list-style: none; margin: 0; padding: 0; }
.rp-order-list li { display: flex; align-items: center; gap: .6rem; padding: .5rem 0; border-bottom: 1px dashed rgba(255,255,255,.12); font-size: .88rem; }
.rp-oi-name { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.rp-oi-qty { color: rgba(255,255,255,.6); font-weight: 600; }
.rp-oi-sub { font-weight: 700; min-width: 5rem; text-align: right; }
.rp-oi-del { width: 1.5rem; height: 1.5rem; border: none; background: rgba(239,68,68,.22); color: #fca5a5; border-radius: 50%; font-size: 1rem; cursor: pointer; line-height: 1; }
.rp-order-total { display: flex; justify-content: space-between; align-items: center; padding-top: .7rem; font-size: 1rem; }
.rp-order-total b { font-weight: 900; color: #fff; }

/* Form */
.rp-form { border-radius: 1rem; padding: 1.1rem; }
.rp-fields { display: flex; flex-direction: column; gap: .8rem; }
.rp-fields label { display: block; font-size: .72rem; font-weight: 700; color: rgba(168,203,191,.85); margin-bottom: .25rem; }
.rp-fields label span { color: #fca5a5; }
.rp-fields input, .rp-fields textarea { width: 100%; padding: .65rem .75rem; border-radius: .65rem; border: 1px solid rgba(255,255,255,.16); font-size: .9rem; outline: none; background: rgba(255,255,255,.07); color: #fff; }
.rp-fields input::placeholder, .rp-fields textarea::placeholder { color: rgba(255,255,255,.38); }
.rp-fields input:focus, .rp-fields textarea:focus { border-color: rgba(126,184,154,.6); box-shadow: 0 0 0 3px rgba(126,184,154,.18); }
.rp-row3 { display: grid; grid-template-columns: repeat(3, 1fr); gap: .6rem; }
.rp-err { color: #fca5a5; font-size: .8rem; margin-top: .7rem; }
.rp-next { width: 100%; margin-top: 1rem; background: linear-gradient(145deg, #7eb89a, #5d9b78); color: #14271d; border: none; padding: .85rem; border-radius: .8rem; font-size: .95rem; font-weight: 800; cursor: pointer; box-shadow: 0 6px 18px rgba(126,184,154,.3); }

/* Sticky bar */
.rp-bar { position: fixed; bottom: 0; left: 0; right: 0; z-index: 30; background: rgba(24,43,32,.82); backdrop-filter: blur(16px); border-top: 1px solid rgba(255,255,255,.1); }
.rp-bar-in { max-width: 760px; margin: 0 auto; padding: .7rem 1.1rem; display: flex; align-items: center; gap: 1rem; }
.rp-bar-info { display: flex; flex-direction: column; line-height: 1.1; }
.rp-bar-count { font-size: .72rem; color: rgba(255,255,255,.6); font-weight: 600; }
.rp-bar-total { font-size: 1.05rem; font-weight: 900; color: #fff; }
.rp-btn { margin-left: auto; background: linear-gradient(145deg, #7eb89a, #5d9b78); color: #14271d; border: none; padding: .75rem 1.4rem; border-radius: .75rem; font-size: .92rem; font-weight: 800; cursor: pointer; box-shadow: 0 4px 14px rgba(126,184,154,.3); }
.rp-btn:disabled { opacity: .6; }

/* Status / uang muka */
.rp-badge { display: inline-block; margin-top: .6rem; padding: .18rem .65rem; border-radius: 999px; font-size: .72rem; font-weight: 800; letter-spacing: .02em; background: rgba(255,255,255,.12); color: #fff; }
.rp-badge--pending { background: rgba(245,158,11,.25); color: #ffe9b0; }
.rp-badge--confirmed, .rp-badge--validated { background: rgba(126,184,154,.3); color: #cdeede; }
.rp-badge--done { background: rgba(126,184,154,.9); color: #14271d; }
.rp-badge--cancelled, .rp-badge--rejected { background: rgba(239,68,68,.25); color: #fecaca; }
.rp-link { margin-top: 1rem; display: flex; flex-wrap: wrap; align-items: center; gap: .5rem; text-align: left; }
.rp-link span { width: 100%; font-size: .72rem; font-weight: 700; color: rgba(168,203,191,.85); }
.rp-link input { flex: 1; min-width: 0; padding: .55rem .7rem; border-radius: .6rem; border: 1px solid rgba(255,255,255,.16); background: rgba(255,255,255,.07); color: #fff; font-size: .78rem; font-family: ui-monospace, monospace; }
.rp-pay { border-radius: 1rem; padding: 1.1rem; margin-top: 1.1rem; }
.rp-pay-hint { font-size: .86rem; color: rgba(255,255,255,.7); margin: -.3rem 0 .9rem; }
.rp-pay-hint b { color: #fff; }
.rp-banks { display: grid; gap: .5rem; margin-bottom: 1rem; }
.rp-bank { display: flex; flex-wrap: wrap; align-items: baseline; gap: .4rem .7rem; padding: .6rem .8rem; border-radius: .7rem; background: rgba(126,184,154,.12); border: 1px solid rgba(126,184,154,.25); }
.rp-bank b { color: #fff; }
.rp-bank span { font-family: ui-monospace, monospace; font-size: .95rem; font-weight: 700; color: #cdeede; letter-spacing: .04em; }
.rp-bank small { width: 100%; font-size: .74rem; color: rgba(255,255,255,.6); }
.rp-fields select { width: 100%; padding: .65rem .75rem; border-radius: .65rem; border: 1px solid rgba(255,255,255,.16); font-size: .9rem; outline: none; background: rgba(255,255,255,.07); color: #fff; }
.rp-fields select option { color: #111; }
.rp-fields input[type=file] { padding: .5rem; font-size: .82rem; }
.rp-ok { color: #cdeede; font-size: .85rem; margin-top: .7rem; }
.rp-actions { display: flex; justify-content: center; margin-top: 1.1rem; }

/* Success */
.rp-success { border-radius: 1rem; padding: 2rem 1.3rem; text-align: center; max-width: 760px; margin: 1.1rem auto; }
.rp-check { width: 4rem; height: 4rem; border-radius: 50%; background: rgba(126,184,154,.2); color: #9fd4ba; display: flex; align-items: center; justify-content: center; margin: 0 auto 1rem; }
.rp-check svg { width: 2rem; height: 2rem; }
.rp-success h2 { font-size: 1.25rem; font-weight: 800; color: #fff; }
.rp-success p { font-size: .9rem; color: rgba(255,255,255,.6); margin-top: .4rem; }
.rp-recap { margin: 1.2rem 0; display: flex; flex-direction: column; gap: .5rem; }
.rp-recap div { display: flex; justify-content: space-between; font-size: .9rem; border-bottom: 1px dashed rgba(255,255,255,.12); padding-bottom: .4rem; }
.rp-recap span { color: rgba(255,255,255,.6); }
.rp-recap b { color: #fff; }
.rp-btn-ghost { background: rgba(255,255,255,.1); border: 1px solid rgba(255,255,255,.16); color: #fff; padding: .65rem 1.2rem; border-radius: .7rem; font-weight: 700; font-size: .88rem; cursor: pointer; }
</style>
