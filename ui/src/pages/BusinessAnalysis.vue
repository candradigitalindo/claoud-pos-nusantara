<!--
  BusinessAnalysis.vue — Laporan → Analisa Bisnis

  Menjawab satu pertanyaan: penurunan penjualan ini masalah outlet (manajer)
  atau masalah pasar/Markom?

  Halaman ini sengaja tidak memuat kalimat penjelas apa pun. Judul bagian,
  penjelasan grafik, temuan, arti istilah, dan saran tindakan semuanya dirakit
  di backend dari angka periode yang sedang dilihat (services/business_analysis_
  narrative.go), lalu di sini hanya digambar. Kalimat tetap yang ditanam di UI
  akan terus berbunyi sama ketika keadaannya sudah berubah — dan pembaca
  kehilangan kepercayaan pada seluruh halaman begitu menemukan satu kalimat
  yang tidak cocok dengan angkanya.

  Yang tinggal di berkas ini hanya perkakas tampilan: warna, ikon, tata letak,
  judul kolom, dan label tombol.
-->
<template>
  <div class="space-y-5">

    <!-- ── Kendali ─────────────────────────────────────────────── -->
    <AppCard>
      <div class="flex flex-wrap items-end gap-3">
        <div class="flex flex-col gap-1">
          <label class="text-sm font-medium text-gray-700">Rentang Analisa</label>
          <AppSelect v-model="weeks" :options="WEEK_OPTIONS" class="min-w-45" />
        </div>

        <button @click="fetchData" :disabled="loading"
          class="px-4 py-2 bg-emerald-600 text-white text-sm font-medium rounded-lg hover:bg-emerald-700 transition-colors shadow-sm disabled:opacity-60">
          Tampilkan
        </button>

        <button @click="downloadExcel" :disabled="!data || busy.excel"
          class="inline-flex items-center gap-2 px-4 py-2 bg-white border border-emerald-600 text-emerald-700 text-sm font-medium rounded-lg hover:bg-emerald-50 transition-colors shadow-sm disabled:opacity-60 disabled:cursor-not-allowed">
          <svg v-if="!busy.excel" class="w-4 h-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" d="M4 16v2a2 2 0 002 2h12a2 2 0 002-2v-2M12 4v12m0 0l-4-4m4 4l4-4" />
          </svg>
          <AppSpinner v-else size="sm" />
          {{ busy.excel ? 'Menyiapkan…' : 'Excel' }}
        </button>

        <button @click="downloadPDF" :disabled="!data || busy.pdf"
          class="inline-flex items-center gap-2 px-4 py-2 bg-white border border-red-500 text-red-600 text-sm font-medium rounded-lg hover:bg-red-50 transition-colors shadow-sm disabled:opacity-60 disabled:cursor-not-allowed">
          <svg v-if="!busy.pdf" class="w-4 h-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" d="M7 21h10a2 2 0 002-2V9.4a2 2 0 00-.6-1.4l-4.4-4.4A2 2 0 0012.6 3H7a2 2 0 00-2 2v14a2 2 0 002 2zM9 13h6M9 17h4" />
          </svg>
          <AppSpinner v-else size="sm" />
          {{ busy.pdf ? 'Menyusun…' : 'PDF + Grafik' }}
        </button>

        <p v-if="data" class="text-xs text-gray-500 ml-auto text-right leading-relaxed">
          {{ data.weeks_count }} minggu penuh<br>
          {{ fmtDate(data.period_from) }} – {{ fmtDate(data.period_to) }}
        </p>
      </div>
    </AppCard>

    <AppAlert type="error" :message="errorMsg" />
    <div v-if="loading" class="flex justify-center py-16"><AppSpinner size="lg" /></div>

    <template v-if="!loading && data">

      <!-- ── Kesimpulan utama ──────────────────────────────────── -->
      <div class="rounded-xl border p-5 flex items-start gap-4" :class="vonis.box">
        <span class="shrink-0 mt-0.5" :class="vonis.icon" v-html="vonis.svg" />
        <div class="min-w-0 flex-1">
          <p class="text-lg font-bold leading-snug" :class="vonis.judul">{{ data.headline }}</p>
          <p class="mt-1.5 text-sm leading-relaxed" :class="vonis.teks">{{ data.verdict_text }}</p>
        </div>
      </div>

      <!-- ── Temuan ────────────────────────────────────────────── -->
      <div v-if="temuanUtama.length" class="grid gap-3" :class="temuanUtama.length > 2 ? 'md:grid-cols-3' : 'md:grid-cols-2'">
        <div v-for="(t, i) in temuanUtama" :key="i"
          class="rounded-xl border p-4" :class="TEMUAN[t.kind].box">
          <div class="flex items-start gap-2.5">
            <span class="shrink-0 mt-0.5" :class="TEMUAN[t.kind].icon" v-html="TEMUAN[t.kind].svg" />
            <p class="text-sm font-semibold leading-snug flex-1" :class="TEMUAN[t.kind].judul">{{ t.title }}</p>
          </div>
          <p class="mt-2 text-[13px] leading-relaxed text-gray-600">{{ t.body }}</p>
          <div v-if="t.outlets?.length" class="mt-2.5 flex flex-wrap gap-1">
            <button v-for="k in t.outlets" :key="k" @click="bukaOutlet(k)"
              class="text-[11px] font-semibold px-1.5 py-0.5 rounded bg-white/80 hover:bg-white transition-colors"
              :class="TEMUAN[t.kind].judul">{{ k }} ›</button>
          </div>
        </div>
      </div>

      <!-- Catatan konteks: tidak menuntut tindakan, jadi tidak perlu sebesar kartu -->
      <details v-if="temuanCatatan.length" class="rounded-lg border border-gray-200 bg-white">
        <summary class="px-4 py-2.5 text-xs font-medium text-gray-500 cursor-pointer select-none hover:text-gray-700">
          {{ temuanCatatan.length }} catatan konteks
        </summary>
        <ul class="px-4 pb-3 space-y-2">
          <li v-for="(t, i) in temuanCatatan" :key="i" class="text-xs text-gray-500 leading-relaxed">
            <span class="font-semibold text-gray-700">{{ t.title }}.</span> {{ t.body }}
          </li>
        </ul>
      </details>

      <!-- ── Angka ringkas ─────────────────────────────────────── -->
      <div class="rounded-xl border border-gray-200 bg-white grid grid-cols-2 lg:grid-cols-4 divide-x divide-y lg:divide-y-0 divide-gray-100">
        <div v-for="k in kartu" :key="k.label" class="px-4 py-3">
          <p class="text-[11px] font-semibold uppercase tracking-wider" :class="k.warnaLabel">{{ k.label }}</p>
          <p class="text-xl font-bold mt-0.5 tabular-nums" :class="k.warnaNilai">{{ k.nilai }}</p>
          <p class="text-[11px] text-gray-400 mt-0.5 leading-snug">{{ k.kaki }}</p>
        </div>
      </div>

      <!-- ── Grafik ────────────────────────────────────────────── -->
      <AppCard v-for="g in GRAFIK" :key="g.key">
        <SectionHead :section="bagian(g.key)" />
        <VueApexCharts :type="g.type" :height="g.height" :options="g.opts.value" :series="g.series.value" />
      </AppCard>

      <!-- ── Tabel rincian ─────────────────────────────────────── -->
      <AppCard>
        <SectionHead :section="bagian('rincian')" />
        <div class="overflow-x-auto">
          <table class="w-full text-sm">
            <thead>
              <tr class="border-b border-gray-200">
                <th v-for="h in KOLOM" :key="h.teks"
                  class="py-3 px-3 text-xs font-semibold text-gray-500 uppercase tracking-wide whitespace-nowrap"
                  :class="h.kanan ? 'text-right' : 'text-left'">{{ h.teks }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100">
              <tr v-for="o in data.outlets" :key="o.outlet_id"
                class="align-top cursor-pointer transition-colors"
                :class="o.code === outletTerpilih ? 'bg-emerald-50/60' : 'hover:bg-gray-50'"
                @click="bukaOutlet(o.code)">
                <td class="py-2.5 px-3">
                  <div class="font-medium text-gray-900 truncate max-w-52" :title="o.name">{{ o.name }}</div>
                  <div class="text-[11px] text-gray-400">{{ o.code }}</div>
                </td>
                <td class="py-2.5 px-3 text-right tabular-nums text-gray-700 whitespace-nowrap">{{ formatRupiah(o.net) }}</td>
                <td class="py-2.5 px-3 text-right tabular-nums whitespace-nowrap">
                  <span class="font-medium" :class="numClass(o.growth4)">{{ pp(o.growth4, '%') }}</span>
                  <span v-if="o.peer_growth4 != null" class="block text-[11px] text-gray-400">
                    lainnya {{ pp(o.peer_growth4, '%') }}
                  </span>
                </td>
                <td class="py-2.5 px-3 text-right tabular-nums whitespace-nowrap">
                  <span class="font-semibold" :class="rgiClass(o)">{{ pp(o.rgi4, '') }}</span>
                  <span v-if="o.threshold != null" class="block text-[11px] text-gray-400">batas ±{{ n1(o.threshold) }}</span>
                </td>
                <td class="py-2.5 px-3 text-right tabular-nums whitespace-nowrap"
                  :class="o.gap_net == null ? 'text-gray-300' : (o.gap_net >= 0 ? 'text-emerald-600' : 'text-red-600')">
                  {{ o.gap_net == null ? '—' : (o.gap_net >= 0 ? '+' : '−') + formatRupiah(Math.abs(o.gap_net)) }}
                </td>
                <td class="py-2.5 px-3">
                  <span class="text-[11px] font-semibold px-2 py-1 rounded whitespace-nowrap" :class="DIAG[o.diagnosis].chip">
                    {{ o.diagnosis_label }}
                  </span>
                </td>
                <td class="py-2.5 px-3">
                  <span class="text-xs font-medium whitespace-nowrap" :class="o.owner === '—' ? 'text-gray-300' : 'text-red-600'">{{ o.owner }}</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </AppCard>

      <!-- ── Laporan detail per outlet ─────────────────────────── -->
      <AppCard id="detail-outlet">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <SectionHead :section="bagian('detail')" class="flex-1 min-w-60" />
          <AppSelect v-model="outletTerpilih" :options="opsiOutlet" class="min-w-64" />
        </div>

        <div v-if="detail" class="space-y-5 mt-4">
          <div class="pb-4 border-b border-gray-100">
            <div class="flex flex-wrap items-center gap-2.5">
              <p class="text-lg font-semibold text-gray-900">{{ detail.name }}</p>
              <span class="text-xs text-gray-400">{{ detail.code }}</span>
              <span class="text-xs font-semibold px-2.5 py-1 rounded" :class="DIAG[detail.diagnosis].chip">
                {{ detail.diagnosis_label }}
              </span>
              <span v-if="detail.owner !== '—'" class="text-xs font-medium px-2.5 py-1 rounded bg-gray-100 text-gray-700">
                {{ detail.owner }}
              </span>
            </div>
            <p class="text-sm text-gray-600 leading-relaxed mt-2">{{ detail.note }}</p>
          </div>

          <div class="grid grid-cols-2 lg:grid-cols-4 gap-3">
            <div v-for="a in angkaPokok" :key="a.label" class="rounded-lg bg-gray-50 p-3">
              <p class="text-xs text-gray-500">{{ a.label }}</p>
              <p class="text-lg font-bold tabular-nums" :class="a.warna ?? 'text-gray-900'">{{ a.nilai }}</p>
            </div>
          </div>

          <div v-if="barisBanding.length" class="rounded-lg border border-gray-200 overflow-hidden">
            <table class="w-full text-sm">
              <tbody class="divide-y divide-gray-100">
                <tr v-for="b in barisBanding" :key="b.label" :class="b.sorot ? 'bg-amber-50/60' : ''">
                  <td class="py-2.5 px-4 text-gray-600" :class="b.tebal ? 'font-medium text-gray-800' : ''">{{ b.label }}</td>
                  <td class="py-2.5 px-4 text-right tabular-nums whitespace-nowrap"
                    :class="[b.tebal ? 'font-bold' : 'font-medium', b.warna ?? 'text-gray-900']">{{ b.nilai }}</td>
                  <td class="py-2.5 px-4 text-right tabular-nums text-gray-400 text-xs whitespace-nowrap">{{ b.kanan ?? '' }}</td>
                </tr>
              </tbody>
            </table>
          </div>

          <div class="grid gap-3 md:grid-cols-2">
            <div v-for="c in ceritaDetail" :key="c.judul" class="rounded-lg border p-4" :class="c.box">
              <p class="text-[11px] font-semibold uppercase tracking-wider mb-1.5" :class="c.judulWarna">{{ c.judul }}</p>
              <p class="text-sm leading-relaxed" :class="c.teksWarna">{{ c.isi }}</p>
            </div>
          </div>

          <div class="overflow-x-auto">
            <table class="w-full text-sm">
              <thead>
                <tr class="border-b border-gray-200">
                  <th v-for="h in KOLOM_MINGGU" :key="h.teks"
                    class="py-2.5 px-3 text-xs font-semibold text-gray-500 uppercase tracking-wide whitespace-nowrap"
                    :class="h.kanan ? 'text-right' : 'text-left'">{{ h.teks }}</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-100">
                <tr v-for="w in detail.weeks" :key="w.week_start" class="hover:bg-gray-50">
                  <td class="py-2.5 px-3 whitespace-nowrap">
                    {{ weekRange(w.week_start) }}
                    <span v-if="mingguTidakBiasa.has(w.label)"
                      class="ml-1.5 text-[10px] font-medium px-1.5 py-0.5 rounded bg-amber-100 text-amber-800">!</span>
                  </td>
                  <td class="py-2.5 px-3 text-right tabular-nums text-gray-900">{{ formatRupiah(w.net) }}</td>
                  <td class="py-2.5 px-3 text-right tabular-nums text-gray-500">{{ w.trx.toLocaleString('id-ID') }}</td>
                  <td class="py-2.5 px-3 text-right tabular-nums text-gray-500">{{ w.trx ? formatRupiah(w.net / w.trx) : '—' }}</td>
                  <td class="py-2.5 px-3 text-right tabular-nums" :class="numClass(w.growth)">{{ pp(w.growth, '%') }}</td>
                  <td class="py-2.5 px-3 text-right tabular-nums" :class="numClass(w.rgi)">{{ pp(w.rgi, '') }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </AppCard>

      <!-- ── Arti istilah ──────────────────────────────────────── -->
      <AppCard v-if="data.glossary?.length" class="!p-0">
        <details>
          <summary class="px-5 py-4 cursor-pointer select-none hover:bg-gray-50/60 transition-colors">
            <span class="text-base font-semibold text-gray-900">{{ bagian('istilah').title }}</span>
            <span class="block text-sm text-gray-500 mt-0.5">{{ bagian('istilah').lead }}</span>
          </summary>
        <dl class="grid gap-x-8 gap-y-4 md:grid-cols-2 px-5 pb-5">
          <div v-for="k in data.glossary" :key="k.term">
            <dt class="text-sm font-semibold text-gray-800">{{ k.term }}</dt>
            <dd class="text-sm text-gray-600 leading-relaxed mt-0.5">{{ k.meaning }}</dd>
            <dd v-if="k.example" class="text-sm text-emerald-800 bg-emerald-50/70 border-l-2 border-emerald-300 pl-2.5 py-1 mt-1.5 leading-relaxed">
              {{ k.example }}
            </dd>
          </div>
        </dl>
        </details>
      </AppCard>

      <!-- ── Cara menghitung ───────────────────────────────────── -->
      <AppCard class="!p-0">
        <details>
          <summary class="px-5 py-4 cursor-pointer select-none hover:bg-gray-50/60 transition-colors">
            <span class="text-base font-semibold text-gray-900">{{ bagian('metode').title }}</span>
            <span class="block text-sm text-gray-500 mt-0.5">{{ bagian('metode').lead }}</span>
          </summary>
          <ul class="space-y-1.5 px-5 pb-5">
            <li v-for="(n, i) in data.notes" :key="i" class="text-xs text-gray-500 leading-relaxed flex gap-2">
              <span class="text-gray-300 shrink-0">•</span><span>{{ n }}</span>
            </li>
          </ul>
        </details>
      </AppCard>

    </template>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, h } from 'vue'
import VueApexCharts from 'vue3-apexcharts'
import { apiClient } from '@/api/client.js'
import { formatRupiah, formatDateStr } from '@/utils/format.js'
import AppCard from '@/components/ui/AppCard.vue'
import AppAlert from '@/components/ui/AppAlert.vue'
import AppSpinner from '@/components/ui/AppSpinner.vue'
import AppSelect from '@/components/ui/AppSelect.vue'

// Tiap grafik diberi id supaya gambarnya bisa diambil untuk PDF
// (ApexCharts.exec(id, 'dataURI')).
const CHART_ID = { grup: 'ba-grup', rgi: 'ba-rgi', tren: 'ba-tren', peta: 'ba-peta' }
// Kunci bagian dari backend -> id kanvas, supaya gambar untuk PDF bisa diambil
// berdasarkan bagian yang sama dengan yang dipakai judulnya.
const CHART_ID_BY_KEY = { pasar: 'grup', selisih: 'rgi', perjalanan: 'tren', peta: 'peta' }

const WEEK_OPTIONS = [
  { value: 8,  label: '8 minggu terakhir' },
  { value: 12, label: '12 minggu terakhir' },
  { value: 26, label: '26 minggu terakhir' },
  { value: 52, label: '52 minggu terakhir' },
]

const data     = ref(null)
const loading  = ref(false)
const errorMsg = ref('')
const weeks    = ref(12)
const busy     = ref({ excel: false, pdf: false })

onMounted(fetchData)

async function fetchData() {
  loading.value = true
  errorMsg.value = ''
  try {
    // apiClient sudah membuka envelope { success, data } → ini langsung payload.
    data.value = await apiClient.get('/admin/business-analysis', { params: { weeks: weeks.value } })
    pilihOutletAwal()
  } catch (err) {
    errorMsg.value = err?.message ?? 'Gagal memuat analisa bisnis.'
  } finally {
    loading.value = false
  }
}

// ── Format ───────────────────────────────────────────────────
// Angka ditulis gaya Indonesia (koma desimal), sama seperti formatRupiah.
function n1(v) { return Number(v).toFixed(1).replace('.', ',') }
function pp(v, suffix) {
  if (v == null) return '—'
  return `${v > 0 ? '+' : ''}${n1(v)}${suffix}`
}
function numClass(v) {
  if (v == null) return 'text-gray-400'
  return v > 0 ? 'text-emerald-600' : v < 0 ? 'text-red-600' : 'text-gray-600'
}
function fmtDate(s) { return s ? formatDateStr(s) : '' }

// Label minggu untuk sumbu grafik: "13–19 Jul" jauh lebih mudah dibaca daripada
// notasi ISO "2026-W29". Notasi ISO tetap dipakai di catatan kaki.
const BULAN = ['Jan', 'Feb', 'Mar', 'Apr', 'Mei', 'Jun', 'Jul', 'Agu', 'Sep', 'Okt', 'Nov', 'Des']
function weekRange(weekStart) {
  if (!weekStart) return ''
  const a = new Date(`${weekStart}T00:00:00`)
  const b = new Date(a.getTime() + 6 * 86400000)
  const bulanSama = a.getMonth() === b.getMonth()
  return bulanSama
    ? `${a.getDate()}–${b.getDate()} ${BULAN[b.getMonth()]}`
    : `${a.getDate()} ${BULAN[a.getMonth()]}–${b.getDate()} ${BULAN[b.getMonth()]}`
}

// Warna angka selisih mengikuti KESIMPULAN, bukan tanda angkanya: selisih yang
// masih di dalam batas wajar tidak boleh tampil merah/hijau seolah sudah pasti.
function rgiClass(o) {
  if (o.rgi4 == null) return 'text-gray-400'
  if (o.diagnosis === 'TERTINGGAL') return 'text-red-600'
  if (o.diagnosis === 'UNGGUL') return 'text-emerald-600'
  return 'text-gray-500'
}

// Peta warna diagnosis. Hanya warna — teksnya datang dari backend lewat
// diagnosis_label, supaya tidak ada dua sumber kata yang bisa saling menyimpang.
const DIAG = {
  UNGGUL:       { chip: 'bg-emerald-100 text-emerald-700', color: '#059669' },
  TERTINGGAL:   { chip: 'bg-red-100 text-red-700',         color: '#dc2626' },
  SEIRAMA:      { chip: 'bg-gray-100 text-gray-600',       color: '#9ca3af' },
  SINYAL_LEMAH: { chip: 'bg-amber-100 text-amber-700',     color: '#d97706' },
  DATA_KURANG:  { chip: 'bg-gray-100 text-gray-500',       color: '#d1d5db' },
}

// Judul bagian: judul, penjelasan, dan cara membaca semuanya dari backend.
// Judul + satu kalimat inti. "Cara membaca" dilipat: berguna sekali saat
// pertama, jadi derau pada pembacaan berikutnya.
const SectionHead = (props) => {
  const b = props.section ?? {}
  return h('div', null, [
    h('h3', { class: 'text-base font-semibold text-gray-900' }, b.title ?? ''),
    b.lead ? h('p', { class: 'text-sm text-gray-600 mt-1 leading-relaxed' }, b.lead) : null,
    b.hint
      ? h('details', { class: 'mt-1.5 group' }, [
          h('summary', { class: 'text-xs text-gray-500 cursor-pointer select-none hover:text-emerald-700 inline-flex items-center gap-1 mt-0.5' }, '› Cara membaca'),
          h('p', { class: 'text-xs text-gray-500 mt-1 leading-relaxed' }, b.hint),
        ])
      : null,
  ])
}
SectionHead.props = ['section']
const bagian = (key) => (data.value?.sections ?? []).find(x => x.key === key) ?? {}

// ── Vonis ────────────────────────────────────────────────────
const IC = {
  outlet: '<svg width="26" height="26" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.7"><path stroke-linecap="round" stroke-linejoin="round" d="M12 9v4m0 4h.01M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z"/></svg>',
  market: '<svg width="26" height="26" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.7"><path stroke-linecap="round" stroke-linejoin="round" d="M3 17l6-6 4 4 8-8m0 0h-5m5 0v5"/></svg>',
  ok:     '<svg width="26" height="26" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.7"><path stroke-linecap="round" stroke-linejoin="round" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>',
  both:   '<svg width="26" height="26" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.7"><path stroke-linecap="round" stroke-linejoin="round" d="M3 7l6 6 4-4 8 8m0 0h-5m5 0v-5M12 20H4a1 1 0 01-1-1v-8"/></svg>',
  none:   '<svg width="26" height="26" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.7"><path stroke-linecap="round" stroke-linejoin="round" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>',
}
// Warna dan ikon tiap vonis. Judulnya sendiri dirakit backend (headline),
// jadi di sini tidak ada kalimat apa pun.
const VONIS = {
  OUTLET:           { svg: IC.outlet, box: 'bg-red-50 border-red-200',        icon: 'text-red-600',     judul: 'text-red-800',     teks: 'text-red-900' },
  PASAR_MARKOM:     { svg: IC.market, box: 'bg-amber-50 border-amber-200',    icon: 'text-amber-600',   judul: 'text-amber-800',   teks: 'text-amber-900' },
  OUTLET_DAN_PASAR: { svg: IC.both,   box: 'bg-orange-50 border-orange-300',  icon: 'text-orange-600',  judul: 'text-orange-800',  teks: 'text-orange-900' },
  NORMAL:           { svg: IC.ok,     box: 'bg-emerald-50 border-emerald-200', icon: 'text-emerald-600', judul: 'text-emerald-800', teks: 'text-emerald-900' },
  DATA_KURANG:      { svg: IC.none,   box: 'bg-gray-50 border-gray-200',      icon: 'text-gray-500',    judul: 'text-gray-700',    teks: 'text-gray-700' },
}
const vonis = computed(() => VONIS[data.value?.verdict] ?? VONIS.DATA_KURANG)

// Warna dan ikon tiap jenis temuan.
const TEMUAN = {
  MASALAH:   { svg: IC.outlet, box: 'bg-red-50 border-red-200',         icon: 'text-red-500',     judul: 'text-red-800' },
  PELUANG:   { svg: IC.market, box: 'bg-emerald-50 border-emerald-200', icon: 'text-emerald-600', judul: 'text-emerald-800' },
  PERHATIAN: { svg: IC.none,   box: 'bg-amber-50 border-amber-200',     icon: 'text-amber-600',   judul: 'text-amber-800' },
  NETRAL:    { svg: IC.none,   box: 'bg-white border-gray-200',         icon: 'text-gray-400',    judul: 'text-gray-700' },
}

// ── Laporan detail per outlet ────────────────────────────────
// Manajer outlet hanya perlu halamannya sendiri, jadi detailnya dipilih satu
// per satu. Yang menandai outlet bermasalah dipilihkan lebih dulu supaya yang
// paling perlu dibaca muncul tanpa dicari.
const outletTerpilih = ref('')
const opsiOutlet = computed(() => (data.value?.outlets ?? []).map(o => ({
  value: o.code,
  label: `${o.name} (${o.code}) — ${o.diagnosis_label}`,
})))
const detail = computed(() =>
  (data.value?.outlets ?? []).find(o => o.code === outletTerpilih.value) ?? null)
const mingguTidakBiasa = computed(() => new Set(data.value?.group_events ?? []))

function pilihOutletAwal() {
  const list = data.value?.outlets ?? []
  if (!list.length) { outletTerpilih.value = ''; return }
  if (list.some(o => o.code === outletTerpilih.value)) return
  const perlu = list.find(o => o.diagnosis === 'TERTINGGAL') ?? list[0]
  outletTerpilih.value = perlu.code
}

// "naik 12,3%" lebih mudah dibaca daripada "+12,3%" bagi pembaca non-analis.
function naikTurun(v) {
  if (v == null) return '—'
  if (v > 0) return `naik ${n1(v)}%`
  if (v < 0) return `turun ${n1(-v)}%`
  return 'tidak berubah'
}

// ── Grafik 1: growth grup per minggu ─────────────────────────
const groupSeries = computed(() => [{
  name: 'Growth grup',
  data: (data.value?.group ?? []).filter(g => g.growth != null).map(g => ({ x: weekRange(g.week_start), y: g.growth })),
}])
const groupOpts = computed(() => {
  const pts = (data.value?.group ?? []).filter(g => g.growth != null)
  return {
    chart: { id: CHART_ID.grup, toolbar: { show: false }, fontFamily: 'inherit' },
    plotOptions: { bar: { borderRadius: 3, columnWidth: '55%', distributed: true } },
    // Minggu peristiwa grup diberi warna berbeda: pada minggu itu seluruh grup
    // bergerak di luar kebiasaan, jadi angkanya bukan cermin kerja manajer.
    colors: pts.map(g => g.is_group_event ? '#f59e0b' : (g.growth >= 0 ? '#10b981' : '#ef4444')),
    legend: { show: false },
    dataLabels: {
      enabled: true,
      formatter: v => `${v > 0 ? '+' : ''}${n1(v)}%`,
      style: { fontSize: '11px', colors: ['#374151'] },
      offsetY: -18,
    },
    grid: { borderColor: '#f1f5f9', padding: { top: 10 } },
    xaxis: { labels: { style: { fontSize: '11px' } } },
    yaxis: { labels: { formatter: v => `${v.toFixed(0)}%`, style: { fontSize: '11px' } } },
    tooltip: {
      y: {
        formatter: (v, { dataPointIndex }) => {
          const g = pts[dataPointIndex]
          const tag = g?.is_group_event ? ' · minggu tidak normal' : ''
          return `${v > 0 ? '+' : ''}${n1(v)}%${tag}`
        },
      },
    },
  }
})

// ── Grafik 2: RGI per outlet + rentang derau ─────────────────
const rgiSeries = computed(() => [{
  name: 'Selisih dengan outlet lain',
  data: (data.value?.outlets ?? []).filter(o => o.rgi4 != null).map(o => ({
    x: o.code,
    y: o.rgi4,
    fillColor: DIAG[o.diagnosis]?.color ?? '#9ca3af',
    // Rentang derau outlet ini — selisih di dalamnya belum bisa disimpulkan.
    goals: o.threshold == null ? [] : [
      { name: 'Batas derau atas',  value: o.threshold,  strokeColor: '#94a3b8', strokeWidth: 2, strokeDashArray: 3 },
      { name: 'Batas derau bawah', value: -o.threshold, strokeColor: '#94a3b8', strokeWidth: 2, strokeDashArray: 3 },
    ],
  })),
}])
const rgiOpts = computed(() => ({
  chart: { id: CHART_ID.rgi, toolbar: { show: false }, fontFamily: 'inherit' },
  plotOptions: { bar: { horizontal: true, borderRadius: 3, barHeight: '55%' } },
  dataLabels: {
    enabled: true,
    formatter: v => `${v > 0 ? '+' : ''}${n1(v)}`,
    style: { fontSize: '11px', colors: ['#1f2937'] },
    offsetX: 34,
  },
  legend: { show: false },
  grid: { borderColor: '#f1f5f9' },
  xaxis: {
    title: { text: '← lebih lambat dari outlet lain     ·     lebih cepat →', style: { fontSize: '11px', fontWeight: 500, color: '#6b7280' } },
    labels: { formatter: v => `${Number(v).toFixed(0)}`, style: { fontSize: '11px' } },
  },
  yaxis: { labels: { style: { fontSize: '12px', fontWeight: 600 } } },
  tooltip: {
    y: {
      formatter: (v, { dataPointIndex }) => {
        const o = (data.value?.outlets ?? []).filter(x => x.rgi4 != null)[dataPointIndex]
        return `${v > 0 ? '+' : ''}${n1(v)} · ${o?.diagnosis_label ?? ''}`
      },
    },
  },
}))

// ── Grafik 3: tren indeks outlet vs grup ─────────────────────
const OUTLET_COLORS = ['#0ea5e9', '#8b5cf6', '#f59e0b', '#ec4899', '#14b8a6', '#84cc16', '#f43f5e', '#6366f1', '#06b6d4']
// Minggu sebelum garis pasar terbentuk dipotong: pada minggu itu belum ada
// titik sandar, jadi seluruh kolomnya kosong dan hanya terbaca sebagai cacat.
const trendWeeks = computed(() => {
  const g = data.value?.group ?? []
  const mulai = g.findIndex(w => w.index != null)
  return mulai < 0 ? [] : g.slice(mulai)
})
const trendSeries = computed(() => {
  if (!data.value) return []
  const labels = trendWeeks.value.map(g => g.label)
  const byLabel = (weeksArr) => {
    const m = Object.fromEntries(weeksArr.map(w => [w.label, w.index]))
    return labels.map(l => m[l] ?? null)
  }
  return [
    { name: 'PASAR', data: trendWeeks.value.map(g => g.index ?? null), type: 'line' },
    ...data.value.outlets.map(o => ({ name: o.code, data: byLabel(o.weeks), type: 'line' })),
  ]
})
const trendOpts = computed(() => ({
  chart: { id: CHART_ID.tren, toolbar: { show: false }, fontFamily: 'inherit', zoom: { enabled: false } },
  colors: ['#111827', ...OUTLET_COLORS],
  // Garis grup dibuat tebal & putus-putus supaya terbaca sebagai pembanding,
  // bukan sebagai salah satu outlet.
  // Sembilan garis dengan bobot sama menghasilkan benang kusut. Garis pasar
  // dan outlet yang sedang dibuka ditebalkan; sisanya tetap ada tetapi mundur
  // ke latar supaya bentuknya masih terbaca sebagai rombongan.
  stroke: {
    width: [3.5, ...(data.value?.outlets ?? []).map(o => o.code === outletTerpilih.value ? 3 : 1.5)],
    dashArray: [6, ...(data.value?.outlets ?? []).map(() => 0)],
    curve: 'straight',
  },
  fill: {
    opacity: [1, ...(data.value?.outlets ?? []).map(o => o.code === outletTerpilih.value ? 1 : 0.35)],
  },
  markers: { size: 0, hover: { size: 4 } },
  legend: { position: 'bottom', fontSize: '12px', markers: { radius: 3 } },
  grid: { borderColor: '#f1f5f9' },
  xaxis: { categories: trendWeeks.value.map(g => weekRange(g.week_start)), labels: { style: { fontSize: '11px' } } },
  yaxis: {
    title: { text: 'Penjualan (awal periode = 100)', style: { fontSize: '11px', fontWeight: 500, color: '#6b7280' } },
    labels: { formatter: v => Number(v).toFixed(0), style: { fontSize: '11px' } },
  },
  annotations: { yaxis: [{ y: 100, borderColor: '#cbd5e1', strokeDashArray: 4 }] },
  tooltip: { shared: true, y: { formatter: v => v == null ? '—' : v.toFixed(0) } },
}))

// ── Grafik 4: matriks diagnosis ──────────────────────────────
const matrixSeries = computed(() => {
  const groups = {}
  for (const o of data.value?.outlets ?? []) {
    if (o.rgi4 == null || o.growth4 == null) continue
    const label = o.diagnosis_label
    ;(groups[label] ??= []).push({ x: o.rgi4, y: o.growth4, code: o.code })
  }
  return Object.entries(groups).map(([name, points]) => ({ name, data: points }))
})
const matrixOpts = computed(() => {
  const order = Object.keys(
    (data.value?.outlets ?? []).filter(o => o.rgi4 != null && o.growth4 != null)
      .reduce((acc, o) => ({ ...acc, [o.diagnosis_label]: 1 }), {}),
  )
  const colorByLabel = Object.fromEntries(Object.values(DIAG).map(d => [d.label, d.color]))
  return {
    chart: { id: CHART_ID.peta, toolbar: { show: false }, fontFamily: 'inherit', zoom: { enabled: false } },
    colors: order.map(l => colorByLabel[l] ?? '#9ca3af'),
    markers: { size: 11, strokeWidth: 0, hover: { sizeOffset: 3 } },
    dataLabels: {
      enabled: true,
      formatter: (_v, { seriesIndex, dataPointIndex, w }) =>
        w.config.series[seriesIndex].data[dataPointIndex]?.code ?? '',
      offsetY: -14,
      style: { fontSize: '11px', fontWeight: 600, colors: ['#374151'] },
      background: { enabled: false },
    },
    legend: { position: 'bottom', fontSize: '12px', markers: { radius: 12 } },
    grid: { borderColor: '#f1f5f9' },
    xaxis: {
      type: 'numeric', tickAmount: 6,
      title: { text: '← tertinggal dari outlet lain     ·     lebih unggul →', style: { fontSize: '11px', fontWeight: 500, color: '#6b7280' } },
      labels: { formatter: v => Number(v).toFixed(0), style: { fontSize: '11px' } },
    },
    yaxis: {
      tickAmount: 5,
      title: { text: 'Penjualan outlet naik / turun (%)', style: { fontSize: '11px', fontWeight: 500, color: '#6b7280' } },
      labels: { formatter: v => Number(v).toFixed(0), style: { fontSize: '11px' } },
    },
    annotations: {
      xaxis: [{ x: 0, borderColor: '#94a3b8', strokeDashArray: 4, label: { text: 'sama dengan outlet lain', orientation: 'horizontal', style: { fontSize: '10px', color: '#64748b', background: 'transparent' } } }],
      yaxis: [{ y: 0, borderColor: '#94a3b8', strokeDashArray: 4 }],
    },
    tooltip: {
      custom: ({ seriesIndex, dataPointIndex, w }) => {
        const p = w.config.series[seriesIndex].data[dataPointIndex]
        return `<div class="px-2 py-1 text-xs">
          <b>${p.code}</b><br>Selisih dgn outlet lain: ${p.x > 0 ? '+' : ''}${n1(p.x)}<br>Penjualan: ${p.y > 0 ? '+' : ''}${n1(p.y)}%
        </div>`
      },
    },
  }
})

// ── Bahan tampilan yang dirakit dari data ────────────────────
// Tidak ada angka maupun daftar yang ditulis tetap di sini: semuanya diturunkan
// dari isi laporan, sehingga ikut berubah kalau outletnya bertambah/berkurang.

// Hanya temuan yang menuntut tindakan yang pantas memakai ruang kartu;
// sisanya konteks, cukup jadi baris yang dilipat.
const temuanUtama = computed(() =>
  (data.value?.insights ?? []).filter(t => t.kind !== 'NETRAL').slice(0, 3))
const temuanCatatan = computed(() => {
  const semua = data.value?.insights ?? []
  const utama = new Set(temuanUtama.value)
  return semua.filter(t => !utama.has(t))
})

const kartu = computed(() => {
  const d = data.value
  if (!d) return []
  const n = d.panel_codes?.length ?? 0
  return [
    { label: 'Gerak Pasar', nilai: pp(d.group_growth4, '%'),
      warnaLabel: 'text-gray-500', warnaNilai: numClass(d.group_growth4),
      kaki: `${d.block_weeks} minggu terakhir vs ${d.block_weeks} sebelumnya`
            + (d.market_band != null ? ` · batas ±${n1(d.market_band)}` : '') },
    { label: 'Lebih Baik', nilai: d.leading.length,
      warnaLabel: 'text-emerald-600', warnaNilai: 'text-emerald-600',
      kaki: d.leading.join(', ') || 'tidak ada' },
    { label: 'Tertinggal', nilai: d.lagging.length,
      warnaLabel: 'text-red-600', warnaNilai: 'text-red-600',
      kaki: d.lagging.join(', ') || 'tidak ada' },
    { label: 'Outlet Pembanding', nilai: n,
      warnaLabel: 'text-gray-500', warnaNilai: 'text-gray-900',
      kaki: n ? `tiap outlet dinilai lawan ${n - 1} lainnya` : 'belum ada' },
  ]
})

// Judul kolom = perkakas tampilan, bukan penjelasan; isinya tetap.
// Tujuh kolom. Patokan outlet lain dan batas wajar tetap tampil, tetapi
// sebagai subteks di kolom yang memang membutuhkannya — sepuluh kolom membuat
// tabel meluber ke samping dan kolom terakhir terpotong.
const KOLOM = [
  { teks: 'Outlet' },
  { teks: 'Penjualan', kanan: true },
  { teks: 'Naik / Turun', kanan: true },
  { teks: 'Selisih', kanan: true },
  { teks: 'Selisih Rupiah', kanan: true },
  { teks: 'Kesimpulan' },
  { teks: 'Yang Bertindak' },
]
const KOLOM_MINGGU = [
  { teks: 'Minggu' },
  { teks: 'Penjualan', kanan: true },
  { teks: 'Struk', kanan: true },
  { teks: 'Rata-rata/Struk', kanan: true },
  { teks: 'Dibanding Minggu Lalu', kanan: true },
  { teks: 'Dibanding Outlet Lain', kanan: true },
]

// Membuka outlet tertentu di bagian detail — dipakai dari kartu temuan maupun
// dari baris tabel, supaya temuan bisa langsung ditelusuri ke angkanya.
function bukaOutlet(code) {
  if (!(data.value?.outlets ?? []).some(o => o.code === code)) return
  outletTerpilih.value = code
  document.getElementById('detail-outlet')?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

const angkaPokok = computed(() => {
  const o = detail.value
  if (!o) return []
  return [
    { label: 'Penjualan seluruh periode', nilai: formatRupiah(o.net) },
    { label: 'Jumlah struk', nilai: o.trx.toLocaleString('id-ID') },
    { label: 'Rata-rata belanja per struk', nilai: formatRupiah(o.atv) },
    { label: 'Selisih dengan outlet lain', nilai: pp(o.rgi4, ''), warna: rgiClass(o) },
  ]
})

const barisBanding = computed(() => {
  const o = detail.value, d = data.value
  if (!o || !d || !(o.recent_net > 0)) return []
  const r = [
    { label: `${d.block_weeks} minggu sebelumnya`, nilai: formatRupiah(o.prev_net),
      kanan: `${o.prev_trx.toLocaleString('id-ID')} struk` },
    { label: `${d.block_weeks} minggu terakhir`, nilai: formatRupiah(o.recent_net),
      kanan: `${o.recent_trx.toLocaleString('id-ID')} struk` },
    { label: 'Jadi outlet ini', nilai: naikTurun(o.growth4), warna: numClass(o.growth4) },
    { label: `Sementara ${o.peer_count} outlet lain`, nilai: naikTurun(o.peer_growth4), warna: numClass(o.peer_growth4) },
  ]
  if (o.expected_net != null) {
    r.push({ label: 'Kalau ikut bergerak seperti mereka', nilai: formatRupiah(o.expected_net), sorot: true })
  }
  if (o.gap_net != null) {
    r.push({ label: 'Selisih dengan kenyataan', sorot: true, tebal: true,
      nilai: (o.gap_net >= 0 ? 'lebih ' : 'kurang ') + formatRupiah(Math.abs(o.gap_net)),
      warna: o.gap_net >= 0 ? 'text-emerald-700' : 'text-red-700' })
  }
  return r
})

const ceritaDetail = computed(() => {
  const o = detail.value
  if (!o) return []
  // Catatan kesimpulan sudah tampil sebagai kalimat di bawah nama outlet;
  // mengulanginya sebagai kotak membuat angka yang sama muncul tiga kali.
  return [
    { judul: 'Dari mana perubahannya', isi: o.breakdown,
      box: 'bg-violet-50 border-violet-100', judulWarna: 'text-violet-800', teksWarna: 'text-violet-900' },
    { judul: 'Yang perlu dilakukan', isi: o.advice,
      box: 'bg-emerald-50 border-emerald-100', judulWarna: 'text-emerald-800', teksWarna: 'text-emerald-900' },
  ].filter(x => x.isi)
})

// Daftar grafik: kunci bagiannya dipakai mengambil judul & penjelasan dari
// backend, jadi menambah grafik tidak perlu menambah kalimat di berkas ini.
const GRAFIK = [
  { key: 'pasar',      type: 'bar',     height: 260, opts: groupOpts,  series: groupSeries },
  { key: 'selisih',    type: 'bar',     height: 320, opts: rgiOpts,    series: rgiSeries },
  { key: 'perjalanan', type: 'line',    height: 340, opts: trendOpts,  series: trendSeries },
  { key: 'peta',       type: 'scatter', height: 340, opts: matrixOpts, series: matrixSeries },
]

// ── Unduh Excel ──────────────────────────────────────────────
// File dibuat di server (excelize) supaya grafiknya grafik Excel asli —
// bisa diklik, diubah rentangnya, dan ditelusuri sampai ke sel.
async function downloadExcel() {
  if (busy.value.excel) return
  busy.value.excel = true
  errorMsg.value = ''
  try {
    const blob = await apiClient.get('/admin/business-analysis/export', {
      params: { weeks: weeks.value },
      responseType: 'blob',
      timeout: 120000,
    })
    saveBlob(blob, `Analisa-Bisnis_${data.value.period_from}_sd_${data.value.period_to}.xlsx`)
  } catch (err) {
    errorMsg.value = err?.message ?? 'Gagal mengunduh file Excel.'
  } finally {
    busy.value.excel = false
  }
}

function saveBlob(blob, filename) {
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}

// ── Unduh PDF ────────────────────────────────────────────────
// Grafik diambil dari ApexCharts yang sudah tergambar di layar
// (dataURI → PNG), jadi isi PDF persis sama dengan yang dilihat pengguna.
// jsPDF di-import saat dipakai supaya tidak menambah berat halaman.
// Grafik yang ikut ke PDF. Hanya pemetaan id kanvas ke kunci bagian — judul
// dan keterangannya diambil dari backend, sama persis dengan yang di layar,
// supaya PDF tidak pernah memuat kalimat yang sudah usang.
const PDF_SECTIONS = GRAFIK.map(g => ({ id: CHART_ID_BY_KEY[g.key], key: g.key }))

// jsPDF memakai encoding WinAnsi; karakter panah tidak ada di dalamnya dan akan
// tercetak jadi sampah, jadi diterjemahkan ke kata biasa lebih dulu.
// PNG dari ApexCharts berukuran besar (4 grafik bisa 8 MB). Grafik warna rata
// seperti ini nyaris tidak terlihat bedanya sebagai JPEG, tapi ukurannya turun
// jauh sehingga PDF-nya enak dikirim lewat email atau WhatsApp.
function toJpeg(dataURI, quality = 0.92) {
  return new Promise((resolve) => {
    const img = new Image()
    img.onload = () => {
      const c = document.createElement('canvas')
      c.width = img.width
      c.height = img.height
      const ctx = c.getContext('2d')
      ctx.fillStyle = '#ffffff'          // JPEG tak punya transparansi
      ctx.fillRect(0, 0, c.width, c.height)
      ctx.drawImage(img, 0, 0)
      resolve(c.toDataURL('image/jpeg', quality))
    }
    img.onerror = () => resolve(dataURI)  // gagal konversi: pakai PNG apa adanya
    img.src = dataURI
  })
}

function pdfText(t) {
  return String(t ?? '')
    .replace(/←/g, '<-').replace(/→/g, '->')
    .replace(/–/g, '-').replace(/·/g, '-')
}

async function downloadPDF() {
  if (busy.value.pdf || !data.value) return
  busy.value.pdf = true
  errorMsg.value = ''
  try {
    const [{ jsPDF }, autoTableMod, apexMod] = await Promise.all([
      import('jspdf'),
      import('jspdf-autotable'),
      import('apexcharts'),
    ])
    const autoTable = autoTableMod.default
    const ApexCharts = apexMod.default
    const d = data.value

    const doc = new jsPDF({ orientation: 'portrait', unit: 'mm', format: 'a4' })
    const W = doc.internal.pageSize.getWidth()
    const M = 14
    let y = M

    // Kop
    doc.setFont('helvetica', 'bold'); doc.setFontSize(16)
    doc.text('Analisa Bisnis - Outlet atau Pasar?', M, y); y += 7
    doc.setFont('helvetica', 'normal'); doc.setFontSize(9); doc.setTextColor(110)
    doc.text(pdfText(`Periode ${fmtDate(d.period_from)} - ${fmtDate(d.period_to)} (${d.weeks_count} minggu penuh) | dibuat ${new Date().toLocaleString('id-ID')}`), M, y)
    y += 8

    // Kotak kesimpulan
    const vb = VONIS[d.verdict] ?? VONIS.DATA_KURANG
    const warna = { OUTLET: [220, 38, 38], PASAR_MARKOM: [217, 119, 6], OUTLET_DAN_PASAR: [234, 88, 12], NORMAL: [5, 150, 105], DATA_KURANG: [107, 114, 128] }[d.verdict] ?? [107, 114, 128]
    const teks = doc.splitTextToSize(pdfText(d.verdict_text), W - 2 * M - 14)
    const tinggi = 13 + teks.length * 4.6
    doc.setFillColor(250, 250, 250); doc.setDrawColor(...warna); doc.setLineWidth(0.5)
    doc.roundedRect(M, y, W - 2 * M, tinggi, 2, 2, 'FD')
    doc.setTextColor(...warna); doc.setFont('helvetica', 'bold'); doc.setFontSize(10)
    doc.text(pdfText(d.headline.toUpperCase()), M + 6, y + 6)
    doc.setTextColor(40); doc.setFont('helvetica', 'normal'); doc.setFontSize(9.5)
    doc.text(teks, M + 6, y + 12)
    y += tinggi + 7

    // Angka utama
    autoTable(doc, {
      startY: y,
      theme: 'grid',
      styles: { fontSize: 9, cellPadding: 2 },
      headStyles: { fillColor: [5, 150, 105], textColor: 255 },
      head: [['Gerak Pasar', 'Batas Wajar Pasar', 'Lebih Baik', 'Tertinggal', 'Outlet Pembanding', 'Cara Membandingkan']],
      body: [[
        pp(d.group_growth4, '%'),
        d.market_band == null ? '-' : `+/-${n1(d.market_band)}`,
        d.leading.join(', ') || '-',
        d.lagging.join(', ') || '-',
        `${d.panel_codes.length} lengkap, tiap outlet vs ${Math.max(d.panel_codes.length - 1, 0)} lainnya`,
        `${d.block_weeks} minggu vs ${d.block_weeks} minggu`,
      ]],
    })
    y = doc.lastAutoTable.finalY + 8

    // Temuan — sudah terurut kepentingan dari backend
    if (d.insights?.length) {
      autoTable(doc, {
        startY: y, theme: 'grid',
        styles: { fontSize: 8, cellPadding: 2, valign: 'top', overflow: 'linebreak' },
        headStyles: { fillColor: [5, 150, 105], textColor: 255, fontSize: 8 },
        columnStyles: { 0: { cellWidth: 22 }, 1: { cellWidth: 52, fontStyle: 'bold' } },
        head: [['Jenis', 'Temuan', 'Keterangan']],
        body: d.insights.map(t => [t.kind, pdfText(t.title), pdfText(t.body)]),
      })
      y = doc.lastAutoTable.finalY + 8
    }

    // Grafik — diambil dari yang sudah tergambar di layar
    for (const sec of PDF_SECTIONS) {
      let img
      try {
        const png = (await ApexCharts.exec(CHART_ID[sec.id], 'dataURI', { width: 1200 }))?.imgURI
        img = png ? await toJpeg(png) : null
      } catch { img = null }
      if (!img) continue

      const imgW = W - 2 * M
      const props = doc.getImageProperties(img)
      const imgH = (props.height * imgW) / props.width

      if (y + imgH + 16 > doc.internal.pageSize.getHeight() - M) { doc.addPage(); y = M }
      doc.setFont('helvetica', 'bold'); doc.setFontSize(11); doc.setTextColor(30)
      doc.text(pdfText(bagian(sec.key).title ?? ''), M, y); y += 5
      doc.setFont('helvetica', 'normal'); doc.setFontSize(8.5); doc.setTextColor(120)
      const ket = doc.splitTextToSize(
        pdfText([bagian(sec.key).lead, bagian(sec.key).hint].filter(Boolean).join(' ')), imgW)
      doc.text(ket, M, y); y += ket.length * 3.6 + 2
      doc.addImage(img, 'JPEG', M, y, imgW, imgH, undefined, 'FAST')
      y += imgH + 9
    }

    // Rincian per outlet — hanya pindah halaman kalau sisa ruang tidak cukup,
    // supaya tidak meninggalkan halaman yang separuhnya kosong.
    if (y > doc.internal.pageSize.getHeight() - 70) { doc.addPage(); y = M }
    doc.setFont('helvetica', 'bold'); doc.setFontSize(13); doc.setTextColor(30)
    doc.text('Rincian per Outlet', M, y); y += 6
    autoTable(doc, {
      startY: y,
      theme: 'grid',
      styles: { fontSize: 7.5, cellPadding: 1.8, valign: 'top', overflow: 'linebreak' },
      headStyles: { fillColor: [5, 150, 105], textColor: 255, fontSize: 7.5 },
      columnStyles: { 0: { cellWidth: 28 }, 7: { cellWidth: 46 } },
      head: [['Outlet', 'Penjualan', 'Naik/Turun', 'Outlet Lain', 'Selisih', 'Batas Wajar', 'Kesimpulan', 'Penjelasan']],
      body: d.outlets.map(o => [
        `${o.name}\n(${o.code})`,
        formatRupiah(o.net),
        pp(o.growth4, '%'),
        pp(o.peer_growth4, '%'),
        pp(o.rgi4, ''),
        o.threshold == null ? '-' : `+/-${n1(o.threshold)}`,
        `${o.diagnosis_label}${o.owner !== '—' ? `\n${o.owner}` : ''}`,
        pdfText(o.note),
      ]),
    })
    y = doc.lastAutoTable.finalY + 8

    // Lembar detail per outlet — dibuat supaya bisa disobek dan diberikan ke
    // tiap manajer: angkanya rupiah, penjelasannya kalimat biasa.
    for (const o of d.outlets) {
      doc.addPage(); y = M
      doc.setFont('helvetica', 'bold'); doc.setFontSize(14); doc.setTextColor(30)
      doc.text(pdfText(`${o.name} (${o.code})`), M, y); y += 6
      doc.setFont('helvetica', 'normal'); doc.setFontSize(9); doc.setTextColor(110)
      doc.text(pdfText(`${o.diagnosis_label}${o.owner !== '—' ? ` · ${o.owner}` : ''}`), M, y)
      y += 7

      const baris = [
        ['Penjualan seluruh periode', formatRupiah(o.net)],
        ['Jumlah struk', o.trx.toLocaleString('id-ID')],
        ['Rata-rata belanja per struk', formatRupiah(o.atv)],
      ]
      if (o.recent_net > 0) {
        baris.push(
          [`${d.block_weeks} minggu sebelumnya`, `${formatRupiah(o.prev_net)}  (${o.prev_trx.toLocaleString('id-ID')} struk)`],
          [`${d.block_weeks} minggu terakhir`, `${formatRupiah(o.recent_net)}  (${o.recent_trx.toLocaleString('id-ID')} struk)`],
          ['Jadi outlet ini', naikTurun(o.growth4)],
          [`Sementara ${o.peer_count} outlet lain`, naikTurun(o.peer_growth4)],
        )
      }
      if (o.expected_net != null) baris.push(['Kalau ikut bergerak seperti mereka', formatRupiah(o.expected_net)])
      if (o.gap_net != null) {
        baris.push(['Selisih dengan kenyataan',
          `${o.gap_net >= 0 ? 'lebih ' : 'kurang '}${formatRupiah(Math.abs(o.gap_net))}`])
      }
      autoTable(doc, {
        startY: y, theme: 'plain',
        styles: { fontSize: 9, cellPadding: 1.6 },
        columnStyles: { 0: { cellWidth: 70, textColor: 90 }, 1: { fontStyle: 'bold' } },
        body: baris,
      })
      y = doc.lastAutoTable.finalY + 5

      for (const [judul, isi] of [
        ['Kesimpulannya', o.note],
        ['Dari mana perubahannya', o.breakdown],
        ['Yang perlu dilakukan', o.advice],
      ]) {
        if (!isi) continue
        if (y > doc.internal.pageSize.getHeight() - 30) { doc.addPage(); y = M }
        doc.setFont('helvetica', 'bold'); doc.setFontSize(9); doc.setTextColor(60)
        doc.text(judul, M, y); y += 4.5
        doc.setFont('helvetica', 'normal'); doc.setTextColor(80)
        const t = doc.splitTextToSize(pdfText(isi), W - 2 * M)
        doc.text(t, M, y); y += t.length * 3.9 + 3
      }

      if (y > doc.internal.pageSize.getHeight() - 40) { doc.addPage(); y = M }
      autoTable(doc, {
        startY: y, theme: 'grid',
        styles: { fontSize: 7.5, cellPadding: 1.6 },
        headStyles: { fillColor: [5, 150, 105], textColor: 255, fontSize: 7.5 },
        head: [['Minggu', 'Penjualan', 'Struk', 'Rata-rata/Struk', 'Dibanding Minggu Lalu', 'Dibanding Outlet Lain']],
        body: o.weeks.map(w => [
          weekRange(w.week_start),
          formatRupiah(w.net),
          w.trx.toLocaleString('id-ID'),
          w.trx ? formatRupiah(w.net / w.trx) : '-',
          pp(w.growth, '%'),
          pp(w.rgi, ''),
        ]),
      })
      y = doc.lastAutoTable.finalY + 8
    }

    // Catatan metodologi
    if (y > doc.internal.pageSize.getHeight() - 40) { doc.addPage(); y = M }
    doc.setFont('helvetica', 'bold'); doc.setFontSize(10); doc.setTextColor(30)
    doc.text('Cara angka ini dihitung', M, y); y += 5
    doc.setFont('helvetica', 'normal'); doc.setFontSize(8); doc.setTextColor(110)
    for (const n of d.notes) {
      const baris = doc.splitTextToSize('- ' + pdfText(n), W - 2 * M)
      if (y + baris.length * 3.6 > doc.internal.pageSize.getHeight() - M) { doc.addPage(); y = M }
      doc.text(baris, M, y); y += baris.length * 3.6 + 1.5
    }

    // Nomor halaman
    const total = doc.internal.getNumberOfPages()
    for (let i = 1; i <= total; i++) {
      doc.setPage(i)
      doc.setFontSize(8); doc.setTextColor(150)
      doc.text(`${i} / ${total}`, W - M, doc.internal.pageSize.getHeight() - 6, { align: 'right' })
    }

    doc.save(`Analisa-Bisnis_${d.period_from}_sd_${d.period_to}.pdf`)
  } catch (err) {
    errorMsg.value = err?.message ?? 'Gagal menyusun PDF.'
  } finally {
    busy.value.pdf = false
  }
}
</script>
