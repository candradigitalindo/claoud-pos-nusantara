<!--
  SocialAccounts.vue — Laporan → Kinerja Markom

  Halaman ini memasok angka Instagram/TikTok untuk grafik medsos di Analisa
  Bisnis, sekaligus jadi tempat memantau apakah pasokannya sehat.

  Tiga keputusan tata letak yang disengaja:

  1. TikTok dan Instagram dipisah menjadi dua panel, tidak pernah dijumlahkan
     jadi satu garis. Keduanya mengukur hal yang berbeda — TikTok menyerahkan
     total suka, Instagram tidak menyerahkan apa pun selain pengikut dan jumlah
     postingan — jadi satu garis gabungan akan naik-turun karena komposisinya,
     bukan karena kinerjanya.

  2. Isian manual dipindah ke modal, tidak lagi menempel di bawah halaman.
     Mengetik angka adalah pekerjaan sesekali untuk SATU akun; menaruhnya
     sebagai tabel permanen membuat halaman yang tugas utamanya memantau
     terbaca seperti formulir entri.

  3. Di bawah sm, tiap tabel berganti jadi daftar kartu. Tabel enam kolom yang
     digulir menyamping di layar ponsel praktis tidak terbaca, dan halaman ini
     paling sering dibuka manajer dari ponsel.
-->
<template>
  <div class="space-y-4 sm:space-y-5">

    <!-- ── Kepala ───────────────────────────────────────────────── -->
    <AppCard>
      <div class="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div class="min-w-0 flex-1">
          <h2 class="text-lg font-semibold text-gray-900">Kinerja Markom</h2>
          <p class="mt-1 text-sm leading-relaxed text-gray-600">
            Angka Instagram dan TikTok tiap outlet, dipakai sebagai pembanding kedua di
            <RouterLink to="/business-analysis" class="font-medium text-emerald-700 hover:underline">Analisa Bisnis</RouterLink>.
            Ditarik otomatis tiap dini hari dari halaman profil publik.
          </p>
        </div>

        <div class="flex flex-wrap items-center gap-2 sm:justify-end">
          <AppSelect v-model.number="minggu" :options="OPSI_MINGGU" class="w-full sm:w-44" />
          <AppButton v-if="bolehKelola" :loading="busy.scrape" :disabled="!accounts.length"
            class="w-full sm:w-auto" @click="tarikSemua">
            {{ busy.scrape ? 'Menarik…' : 'Tarik sekarang' }}
          </AppButton>
        </div>
      </div>

      <p v-if="bolehKelola && accounts.length" class="mt-2 text-[11px] leading-relaxed text-gray-500">
        Tombol tarik untuk memastikan tautannya benar — maksimal {{ BATAS_MANUAL }} akun sekali tekan, dengan jeda.
        Pengumpulan hariannya dikerjakan penjadwal dini hari.
      </p>

      <div v-if="hasil" class="mt-4 rounded-lg border p-3.5 text-sm"
        :class="hasil.failed ? 'border-amber-200 bg-amber-50' : 'border-emerald-200 bg-emerald-50'">
        <p class="font-medium" :class="hasil.failed ? 'text-amber-900' : 'text-emerald-900'">
          {{ hasil.ok }} dari {{ hasil.attempted }} akun berhasil ditarik dalam {{ (hasil.took_ms / 1000).toFixed(1) }} detik.
        </p>
        <ul v-if="hasil.errors?.length" class="mt-2 space-y-1">
          <li v-for="(e, i) in hasil.errors" :key="i" class="flex gap-2 text-xs leading-relaxed"
            :class="hasil.failed ? 'text-amber-900' : 'text-emerald-900'">
            <span class="shrink-0 opacity-40">•</span><span class="break-words">{{ e }}</span>
          </li>
        </ul>
        <p v-if="hasil.failed" class="mt-2 text-xs leading-relaxed text-amber-800">
          Akun yang ditolak platform sengaja didiamkan beberapa jam sebelum dicoba lagi — mencoba
          berulang kali justru memperdalam pembatasannya.
        </p>
      </div>
    </AppCard>

    <AppAlert type="error" :message="errorMsg" />
    <div v-if="loading" class="flex justify-center py-16"><AppSpinner size="lg" /></div>

    <template v-if="!loading">
      <div v-if="!accounts.length">
        <AppCard>
          <div class="py-10 text-center">
            <p class="text-sm text-gray-600">Belum ada akun terdaftar.</p>
            <p class="mx-auto mt-1.5 max-w-lg text-xs leading-relaxed text-gray-500">
              Selama kosong, bagian medsos di Analisa Bisnis tidak digambar sama sekali —
              grafik kosong akan terbaca seolah medsos outlet-outlet ini memang mati.
            </p>
            <AppButton v-if="bolehKelola" class="mt-4" @click="bukaTambah">Tambah Akun</AppButton>
          </div>
        </AppCard>
      </div>

      <template v-else>
        <!-- ── Ringkasan per platform ───────────────────────────── -->
        <div class="grid gap-4 lg:grid-cols-2">
          <AppCard v-for="p in PLATFORM" :key="p.key">
            <div class="flex items-center justify-between gap-3">
              <div class="flex min-w-0 items-center gap-2.5">
                <span class="shrink-0 rounded-lg p-2" :class="p.kotak" v-html="IC[p.key]" />
                <div class="min-w-0">
                  <h3 class="truncate text-base font-semibold text-gray-900">{{ p.nama }}</h3>
                  <p class="text-[11px] text-gray-500">{{ ringkas(p.key).akun }} akun terdaftar</p>
                </div>
              </div>
              <span v-if="ringkas(p.key).bermasalah"
                class="shrink-0 rounded bg-amber-100 px-2 py-1 text-[11px] font-semibold text-amber-800">
                {{ ringkas(p.key).bermasalah }} bermasalah
              </span>
            </div>

            <dl class="mt-4 grid grid-cols-2 gap-3 sm:grid-cols-4">
              <div v-for="k in kartuRingkas(p.key)" :key="k.label" class="rounded-lg bg-gray-50 p-2.5">
                <dt class="text-[10px] font-semibold uppercase tracking-wider text-gray-500">{{ k.label }}</dt>
                <dd class="mt-0.5 text-base font-bold leading-tight tabular-nums" :class="k.warna ?? 'text-gray-900'">{{ k.nilai }}</dd>
                <dd class="text-[10px] leading-snug text-gray-400">{{ k.kaki }}</dd>
              </div>
            </dl>
          </AppCard>
        </div>

        <!-- ── Grafik ───────────────────────────────────────────── -->
        <AppCard>
          <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div class="min-w-0">
              <h3 class="text-base font-semibold text-gray-900">Perkembangan {{ metrikAktif.label }}</h3>
              <p class="mt-0.5 text-xs leading-relaxed text-gray-500">{{ metrikAktif.jelas }}</p>
            </div>
            <!-- Pemilih metrik: menggulir menyamping di ponsel, bukan membungkus
                 jadi dua baris yang menggeser grafiknya ke bawah layar. -->
            <div class="-mx-1 overflow-x-auto pb-1">
              <div class="inline-flex gap-1 px-1">
                <button v-for="m in METRIK" :key="m.key" @click="metrik = m.key"
                  class="whitespace-nowrap rounded-lg px-3 py-1.5 text-xs font-medium transition-colors"
                  :class="metrik === m.key ? 'bg-emerald-600 text-white shadow-sm' : 'bg-gray-100 text-gray-600 hover:bg-gray-200'">
                  {{ m.label }}
                </button>
              </div>
            </div>
          </div>

          <div class="mt-4 grid gap-5 lg:grid-cols-2">
            <div v-for="p in PLATFORM" :key="p.key" class="min-w-0">
              <div class="mb-1 flex items-center gap-2">
                <span class="shrink-0" :class="p.teks" v-html="IC[p.key]" />
                <h4 class="text-sm font-semibold text-gray-800">{{ p.nama }}</h4>
              </div>

              <VueApexCharts v-if="adaAngka(p.key)" :type="metrikAktif.tipe" :height="270"
                :options="opsiGrafik(p.key)" :series="seriGrafik(p.key)" />

              <!-- Kosong bukan berarti nol. Bedanya disebutkan, karena keduanya
                   menuntut tindakan yang sama sekali berbeda. -->
              <div v-else class="flex h-[270px] items-center justify-center rounded-lg border border-dashed border-gray-200 px-5">
                <p class="max-w-xs text-center text-xs leading-relaxed text-gray-500">{{ pesanKosong(p.key) }}</p>
              </div>
            </div>
          </div>
        </AppCard>

        <!-- ── Akun per outlet ──────────────────────────────────── -->
        <!--
          Satu baris = satu outlet, dua kolom platform di sampingnya. Bukan
          tabel <table>, melainkan kisi yang melipat sendiri: di bawah sm
          ketiga sel menumpuk, di atasnya berjajar. Dengan satu markup untuk
          kedua bentuk, tidak ada versi mobile yang diam-diam tertinggal ketika
          kolomnya berubah.
        -->
        <AppCard :padding="false">
          <div class="flex flex-wrap items-center justify-between gap-3 p-4 sm:p-5">
            <div class="min-w-0">
              <h3 class="text-base font-semibold text-gray-900">Akun per Outlet</h3>
              <p class="mt-0.5 text-xs text-gray-500">
                {{ jumlahTerdaftar }} dari {{ barisOutlet.length * 2 }} kemungkinan akun terdaftar.
                Kolom kosong berarti belum didaftarkan.
              </p>
            </div>
            <AppButton v-if="bolehKelola" variant="secondary" size="sm" @click="bukaTambah">Tambah Akun</AppButton>
          </div>

          <div class="hidden border-y border-gray-100 bg-gray-50/70 px-4 py-1.5 sm:grid sm:grid-cols-[minmax(7rem,1fr)_1.4fr_1.4fr] sm:gap-4">
            <span class="text-[10px] font-semibold uppercase tracking-wide text-gray-400">Outlet</span>
            <span v-for="p in PLATFORM" :key="p.key"
              class="flex items-center gap-1.5 text-[10px] font-semibold uppercase tracking-wide text-gray-400">
              <span :class="p.teks" v-html="IC[p.key]" />{{ p.nama }}
            </span>
          </div>

          <!--
            Tidak ada batas tinggi dan tidak ada gulir di dalam gulir: seluruh
            outlet tampil sekaligus. Barisnya dirampingkan sampai kesembilan
            outlet muat dalam satu layar laptop, tetapi angkanya TIDAK ikut
            dikecilkan — angka yang sulit dibaca menggagalkan tujuan bagian ini
            lebih parah daripada baris yang sedikit lebih rapat.
          -->
          <ul class="divide-y divide-gray-100 border-t border-gray-100 sm:border-t-0">
            <!-- py-1.5, bukan py-2: enam piksel per baris terdengar sepele,
                 tetapi dikalikan sembilan outlet ia yang menentukan bagian ini
                 masih muat di laptop 720p atau tidak. -->
            <li v-for="o in barisOutlet" :key="o.code"
              class="px-4 py-1.5 transition-colors hover:bg-gray-50/60">
              <div class="grid gap-x-2 gap-y-1.5 min-[360px]:grid-cols-2 sm:grid-cols-[minmax(7rem,1fr)_1.4fr_1.4fr] sm:items-center sm:gap-4">
                <div class="min-w-0 col-span-full sm:col-span-1">
                  <p class="truncate text-[13px] font-semibold leading-tight text-gray-900" :title="o.name">{{ o.name }}</p>
                  <p class="text-[10px] leading-tight text-gray-400">{{ o.code }}</p>
                </div>

                <div v-for="c in o.kolom" :key="c.platform.key" class="group/sel min-w-0 sm:max-w-[17rem]">
                  <template v-if="c.akun">
                    <div class="flex items-center gap-1.5">
                      <!-- Keadaan jadi titik; kalimatnya ada di tooltip -->
                      <span class="h-2 w-2 shrink-0 rounded-full" :class="statusAkun(c.akun).titik"
                        :title="statusJudul(c.akun)" />
                      <span class="shrink-0 sm:hidden" :class="c.platform.teks" v-html="IC[c.platform.key]" />
                      <a :href="c.akun.profile_url" target="_blank" rel="noopener"
                        class="truncate text-[12px] font-medium text-gray-600 transition-colors hover:text-emerald-700">
                        @{{ c.akun.username }}
                      </a>
                      <!-- Tombol muncul saat baris disentuh; di ponsel selalu
                           tampak karena tidak ada keadaan "disentuh" di sana. -->
                      <div v-if="bolehKelola"
                        class="ml-auto flex shrink-0 gap-0.5 transition-opacity sm:opacity-0 sm:group-hover/sel:opacity-100 sm:focus-within:opacity-100">
                        <button v-for="t in aksi(c.akun)" :key="t.judul" @click="t.aksi" :disabled="t.mati"
                          :title="t.judul" :aria-label="t.judul"
                          class="rounded p-1 transition-colors disabled:opacity-30"
                          :class="t.kelas" v-html="t.ikon" />
                      </div>
                    </div>

                    <div class="mt-px flex items-center justify-between gap-2">
                      <p class="min-w-0 truncate text-[12px] leading-tight text-gray-500">
                        <b class="text-[15px] tabular-nums text-gray-900">{{ c.akun.followers == null ? '—' : angkaID(c.akun.followers) }}</b>
                        <span v-if="c.delta != null" class="ml-1 font-medium tabular-nums"
                          :class="c.delta >= 0 ? 'text-emerald-600' : 'text-red-600'">
                          {{ c.delta >= 0 ? '+' : '−' }}{{ angkaID(Math.abs(c.delta)) }}</span>
                        <span class="ml-1.5">· {{ c.akun.posts_count == null ? '—' : angkaID(c.akun.posts_count) }} konten</span>
                      </p>
                      <!-- Jejak pengikut: angka tunggal tidak pernah menunjukkan ARAH -->
                      <svg v-if="sparkline(c.seri, 54, 16)" width="54" height="16" viewBox="0 0 54 16"
                        class="hidden shrink-0 overflow-visible min-[420px]:block" aria-hidden="true">
                        <path :d="sparkline(c.seri, 54, 16)" fill="none" :stroke="warnaOutlet(o.code)"
                          stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" />
                      </svg>
                    </div>
                  </template>

                  <button v-else-if="bolehKelola" @click="tambahUntuk(o.code, c.platform.key)"
                    class="flex w-full items-center justify-center gap-1 rounded-md border border-dashed border-gray-200 py-1.5 text-[11px] font-medium text-gray-400 transition-colors hover:border-emerald-400 hover:bg-emerald-50/50 hover:text-emerald-700">
                    <span v-html="IC_AKSI.tambah" />
                    <span>Tambah</span>
                  </button>
                  <p v-else class="py-1.5 text-center text-[11px] text-gray-300">belum terdaftar</p>
                </div>
              </div>
            </li>
          </ul>
        </AppCard>
      </template>
    </template>

    <!-- ── Tambah / ubah akun ───────────────────────────────────── -->
    <AppModal v-model="modal.open" :title="modal.id ? 'Ubah Akun' : 'Tambah Akun'">
      <div class="space-y-4">
        <AppSelect v-model="form.outlet_id" label="Outlet" :options="opsiOutlet"
          placeholder="Pilih outlet" :disabled="!!modal.id" />
        <AppSelect v-model="form.platform" label="Platform" :options="OPSI_PLATFORM" :disabled="!!modal.id" />
        <AppInput v-model="form.username" label="Tautan profil atau nama akun"
          placeholder="https://www.instagram.com/nama.akun/ atau @nama.akun" />
        <p class="text-xs leading-relaxed text-gray-500">
          Tautan boleh ditempel apa adanya dari peramban — bagian di belakang tanda tanya ikut dibuang otomatis.
        </p>

        <div class="rounded-lg border border-gray-200 p-3">
          <label class="flex cursor-pointer items-start gap-2.5">
            <input v-model="form.auto_fetch" type="checkbox"
              class="mt-0.5 h-4 w-4 rounded border-gray-300 text-emerald-600 focus:ring-emerald-500" />
            <span class="text-sm leading-relaxed text-gray-700">
              Tarik otomatis tiap dini hari
              <span class="mt-0.5 block text-xs text-gray-500">
                Kalau dimatikan, akun ini hanya memakai angka yang diketik manual —
                dan ia tidak akan pernah dilaporkan mandek.
              </span>
            </span>
          </label>
          <p class="mt-2.5 rounded-lg border border-gray-200 bg-gray-50 p-2.5 text-xs leading-relaxed text-gray-600">
            {{ KEMAMPUAN[form.platform] }}
          </p>
        </div>

        <p v-if="modal.id" class="rounded-lg border border-amber-200 bg-amber-50 p-2.5 text-xs leading-relaxed text-amber-700">
          Mengganti nama akun akan menghapus seluruh riwayat akun lama. Riwayat dua akun berbeda yang
          disambung jadi satu garis akan terbaca sebagai lonjakan pengikut yang tidak pernah terjadi.
        </p>
        <AppAlert type="error" :message="modal.error" />
      </div>
      <template #footer>
        <AppButton variant="secondary" @click="modal.open = false">Batal</AppButton>
        <AppButton :loading="busy.simpan" @click="simpanAkun">Simpan</AppButton>
      </template>
    </AppModal>

    <!-- ── Isian manual mingguan ────────────────────────────────── -->
    <AppModal v-model="manual.open" size="xl"
      :title="manual.akun ? `Isian Manual — @${manual.akun.username}` : 'Isian Manual'">
      <div class="space-y-4 sm:space-y-5">
        <p class="text-xs leading-relaxed text-gray-600">
          Untuk angka yang tidak ada di halaman publik. Jangkauan Instagram salah satunya: angka itu
          hanya hidup di layar Insights pemegang akun. Kolom yang diisi menimpa hasil tarikan pada
          minggu itu saja; yang dikosongkan tetap dari tarikan.
        </p>

        <div v-if="busy.manual" class="flex justify-center py-10"><AppSpinner /></div>

        <div v-else class="max-h-[55vh] overflow-y-auto pr-1">
          <!-- Ponsel: satu kartu per minggu -->
          <ul class="space-y-2.5 sm:hidden">
            <li v-for="r in barisMinggu" :key="r.week_start"
              class="rounded-lg border p-3" :class="r.adaManual ? 'border-sky-200 bg-sky-50/50' : 'border-gray-200'">
              <div class="flex items-center justify-between gap-2">
                <p class="text-sm font-medium text-gray-800">{{ rentangMinggu(r.week_start) }}</p>
                <span v-if="r.adaManual" class="rounded bg-sky-100 px-1.5 py-0.5 text-[10px] font-medium text-sky-800">manual</span>
              </div>
              <div class="mt-2 grid grid-cols-2 gap-2">
                <label v-for="k in KOLOM_ISI" :key="k.field" class="block">
                  <span class="mb-0.5 block text-[10px] font-medium uppercase tracking-wide text-gray-500">{{ k.teks }}</span>
                  <input v-model="r.isi[k.field]" type="number" min="0" inputmode="numeric"
                    :placeholder="r.asal[k.field]" class="isian w-full" />
                </label>
              </div>
              <button @click="simpanMinggu(r)" :disabled="r.busy || !berubah(r)"
                class="mt-2.5 w-full rounded-md bg-emerald-600 px-3 py-1.5 text-xs font-medium text-white transition-colors hover:bg-emerald-700 disabled:cursor-not-allowed disabled:opacity-30">
                {{ r.busy ? 'Menyimpan…' : 'Simpan minggu ini' }}
              </button>
            </li>
          </ul>

          <!-- Layar lebar: tabel -->
          <table class="hidden w-full text-sm sm:table">
            <thead class="sticky top-0 bg-white">
              <tr class="border-b border-gray-200">
                <th class="px-2 py-2.5 text-left text-xs font-semibold uppercase tracking-wide text-gray-500">Minggu</th>
                <th v-for="k in KOLOM_ISI" :key="k.field"
                  class="whitespace-nowrap px-2 py-2.5 text-right text-xs font-semibold uppercase tracking-wide text-gray-500">
                  {{ k.teks }}
                </th>
                <th class="px-2 py-2.5"></th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100">
              <tr v-for="r in barisMinggu" :key="r.week_start" :class="r.adaManual ? 'bg-sky-50/50' : ''">
                <td class="whitespace-nowrap px-2 py-2">
                  {{ rentangMinggu(r.week_start) }}
                  <span v-if="r.adaManual" class="ml-1.5 rounded bg-sky-100 px-1.5 py-0.5 text-[10px] font-medium text-sky-800">manual</span>
                </td>
                <td v-for="k in KOLOM_ISI" :key="k.field" class="px-2 py-2 text-right">
                  <input v-model="r.isi[k.field]" type="number" min="0" inputmode="numeric"
                    :placeholder="r.asal[k.field]" class="isian w-24 text-right" />
                </td>
                <td class="whitespace-nowrap px-2 py-2 text-right">
                  <button @click="simpanMinggu(r)" :disabled="r.busy || !berubah(r)"
                    class="text-xs font-medium text-emerald-700 hover:text-emerald-900 disabled:cursor-not-allowed disabled:opacity-30">
                    {{ r.busy ? 'Menyimpan…' : 'Simpan' }}
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <p class="text-[11px] leading-relaxed text-gray-500">
          Angka abu-abu di dalam kotak adalah hasil tarikan minggu itu — kotak yang dibiarkan kosong
          memakai angka tersebut. Kosongkan kembali sebuah kotak untuk mencabut tambalannya.
        </p>
      </div>
      <template #footer>
        <AppButton variant="secondary" @click="manual.open = false">Tutup</AppButton>
      </template>
    </AppModal>

    <AppModal v-model="hapus.open" title="Hapus akun medsos?" size="sm">
      <p class="text-sm leading-relaxed text-gray-600">
        Seluruh riwayat potret dan konten <b>@{{ hapus.akun?.username }}</b> ikut terhapus permanen,
        dan outlet ini hilang dari grafik medsos di Analisa Bisnis.
      </p>
      <template #footer>
        <AppButton variant="secondary" @click="hapus.open = false">Batal</AppButton>
        <AppButton variant="danger" :loading="busy.hapus" @click="hapusAkun">Hapus</AppButton>
      </template>
    </AppModal>
  </div>
</template>

<script setup>
import { ref, reactive, computed, watch, onMounted } from 'vue'
import { RouterLink } from 'vue-router'
import VueApexCharts from 'vue3-apexcharts'
import { socialApi } from '@/api/social.js'
import { outletsApi } from '@/api/outlets.js'
import { useToastStore } from '@/stores/toast.js'
import { useAuthStore } from '@/stores/auth.js'
import AppCard from '@/components/ui/AppCard.vue'
import AppModal from '@/components/ui/AppModal.vue'
import AppInput from '@/components/ui/AppInput.vue'
import AppSelect from '@/components/ui/AppSelect.vue'
import AppButton from '@/components/ui/AppButton.vue'
import AppAlert from '@/components/ui/AppAlert.vue'
import AppSpinner from '@/components/ui/AppSpinner.vue'

const toast = useToastStore()
const auth = useAuthStore()
const bolehKelola = computed(() => auth.hasPermission('social.manage'))

// Harus sama dengan socialBatasManual di services/social.go — dipakai hanya
// untuk menerangkan perilakunya, bukan untuk menegakkannya.
const BATAS_MANUAL = 5

// Ikon inline SVG mengikuti gaya halaman lain; emoji tampil berbeda di tiap
// sistem operasi dan tidak bisa diwarnai mengikuti keadaan barisnya.
const IC = {
  instagram: '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><rect x="2" y="2" width="20" height="20" rx="5"/><circle cx="12" cy="12" r="4"/><circle cx="17.5" cy="6.5" r="1.2" fill="currentColor" stroke="none"/></svg>',
  tiktok: '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M15 3v11.5a4 4 0 11-3-3.87"/><path d="M15 6.5a5 5 0 004.5 3"/></svg>',
}

const NAMA_PLATFORM = { instagram: 'Instagram', tiktok: 'TikTok' }
// Satu urutan platform dipakai di SELURUH halaman — kartu ringkas, panel
// grafik, kolom tabel, dan pilihan di modal. Urutan yang berpindah-pindah
// antar bagian memaksa mata mencari ulang tiap kali turun satu kartu.
const PLATFORM = [
  { key: 'instagram', nama: 'Instagram', kotak: 'bg-pink-100 text-pink-600', teks: 'text-pink-500' },
  { key: 'tiktok', nama: 'TikTok', kotak: 'bg-gray-900 text-white', teks: 'text-gray-700' },
]
const OPSI_PLATFORM = PLATFORM.map(p => ({ value: p.key, label: p.nama }))
const KEMAMPUAN = {
  tiktok: 'Dari TikTok bisa ditarik pengikut, jumlah konten, dan total suka — semuanya eksak. Konten dan interaksi mingguan dihitung dari selisih antar minggu.',
  instagram: 'Dari Instagram bisa ditarik pengikut, yang diikuti, dan jumlah postingan. Jangkauannya tidak — angka itu hanya hidup di layar Insights pemegang akun, jadi perlu diisi manual.',
}

// Tiap metrik memilih bentuknya sendiri. Pengikut adalah TINGGI PERMUKAAN pada
// satu saat, jadi digambar sebagai garis; konten, tayangan, dan interaksi
// adalah ALIRAN sepanjang satu minggu, jadi digambar sebagai batang. Menggambar
// aliran sebagai garis membuat mata menyambungkan dua minggu yang sebenarnya
// tidak bersambung.
const METRIK = [
  { key: 'followers', label: 'Pengikut', tipe: 'line', tumpuk: false,
    jelas: 'Jumlah pengikut pada akhir tiap minggu. Garis yang putus berarti minggu itu tidak ada pembacaan yang berhasil — bukan berarti pengikutnya nol.' },
  { key: 'views', label: 'Jangkauan', tipe: 'bar', tumpuk: true,
    jelas: 'Berapa kali konten ditonton. Hanya ada bila platformnya menyerahkan angka itu atau diisi manual.' },
  { key: 'engagement', label: 'Interaksi', tipe: 'bar', tumpuk: true,
    jelas: 'Suka, komentar, dan bagikan yang didapat sepanjang minggu itu.' },
  { key: 'posts', label: 'Konten', tipe: 'bar', tumpuk: true,
    jelas: 'Berapa konten terbit tiap minggu — ukuran paling langsung apakah medsosnya sedang dikerjakan.' },
]

const OPSI_MINGGU = [8, 12, 26, 52].map(n => ({ value: n, label: `${n} minggu terakhir` }))

// Aksi jadi ikon, bukan teks. Satu baris memuat dua platform yang masing-masing
// punya empat tindakan; delapan tombol berteks akan memakan lebih banyak ruang
// daripada angkanya sendiri, dan angkanyalah alasan tabel ini ada.
const IC_AKSI = {
  tarik: '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 2v6h-6"/><path d="M3 12a9 9 0 0115-6.7L21 8"/><path d="M3 22v-6h6"/><path d="M21 12a9 9 0 01-15 6.7L3 16"/></svg>',
  manual: '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>',
  ubah: '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 00.33 1.82l.06.06a2 2 0 11-2.83 2.83l-.06-.06a1.65 1.65 0 00-1.82-.33 1.65 1.65 0 00-1 1.51V21a2 2 0 11-4 0v-.09A1.65 1.65 0 008 19.4a1.65 1.65 0 00-1.82.33l-.06.06a2 2 0 11-2.83-2.83l.06-.06a1.65 1.65 0 00.33-1.82 1.65 1.65 0 00-1.51-1H2a2 2 0 110-4h.09A1.65 1.65 0 004.6 8a1.65 1.65 0 00-.33-1.82l-.06-.06a2 2 0 112.83-2.83l.06.06A1.65 1.65 0 009 3.6 1.65 1.65 0 0010 2.09V2a2 2 0 114 0v.09a1.65 1.65 0 001 1.51 1.65 1.65 0 001.82-.33l.06-.06a2 2 0 112.83 2.83l-.06.06A1.65 1.65 0 0019.4 8v0a1.65 1.65 0 001.51 1H21a2 2 0 110 4h-.09a1.65 1.65 0 00-1.51 1z"/></svg>',
  hapus: '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6m3 0V4a1 1 0 011-1h4a1 1 0 011 1v2"/></svg>',
  tambah: '<svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round"><path d="M12 5v14M5 12h14"/></svg>',
}
const KOLOM_ISI = [
  { field: 'followers', teks: 'Pengikut' },
  { field: 'posts', teks: 'Konten' },
  { field: 'views', teks: 'Jangkauan' },
  { field: 'engagement', teks: 'Interaksi' },
]

// Palet yang sama dengan Analisa Bisnis, dengan urutan yang sudah lolos uji
// buta warna (pasangan bersebelahan terburuk ΔE 8,9).
const WARNA_OUTLET = ['#0ea5e9', '#f59e0b', '#8b5cf6', '#84cc16', '#ec4899', '#06b6d4', '#f43f5e', '#14b8a6', '#6366f1']

const accounts = ref([])
const outlets = ref([])
const weekly = ref([])
const loading = ref(true)
const errorMsg = ref('')
const hasil = ref(null)
const minggu = ref(12)
const metrik = ref('followers')
const busy = reactive({ scrape: false, simpan: false, hapus: false, manual: false })

const modal = reactive({ open: false, id: '', error: '' })
const form = reactive({ outlet_id: '', platform: 'instagram', username: '', auto_fetch: true })
const manual = reactive({ open: false, akun: null })
const hapus = reactive({ open: false, akun: null })

const metrikAktif = computed(() => METRIK.find(m => m.key === metrik.value) ?? METRIK[0])
const jumlahTerdaftar = computed(() => accounts.value.length)

onMounted(muat)
watch(minggu, muatMingguan)

async function muat() {
  loading.value = true
  errorMsg.value = ''
  try {
    accounts.value = (await socialApi.list()) ?? []
    // Mendaftarkan akun medsos tidak menuntut izin melihat master outlet, jadi
    // daftar lengkapnya boleh gagal — yang tersisa outlet dalam jangkauan
    // pengguna sendiri, dan itu sudah cukup untuk mengisi pilihan di modal.
    let gerai
    try {
      gerai = await outletsApi.list()
    } catch {
      gerai = await outletsApi.myOutlets()
    }
    outlets.value = Array.isArray(gerai) ? gerai : (gerai?.data ?? [])
    await muatMingguan()
  } catch (err) {
    errorMsg.value = err?.message ?? 'Gagal memuat data medsos.'
  } finally {
    loading.value = false
  }
}

async function muatMingguan() {
  if (!accounts.value.length) { weekly.value = []; return }
  try {
    weekly.value = (await socialApi.weekly(minggu.value)) ?? []
  } catch (err) {
    errorMsg.value = err?.message ?? 'Gagal memuat ringkasan mingguan.'
  }
}

const opsiOutlet = computed(() => outlets.value.map(o => ({ value: o.id, label: `${o.name} (${o.code})` })))

// ── Bahan grafik ─────────────────────────────────────────────
const daftarMinggu = computed(() => [...new Set(weekly.value.map(r => r.week_start))].sort())

// Dikelompokkan per platform lalu per outlet. Dua platform TIDAK PERNAH
// dijumlahkan jadi satu angka: keduanya mengukur hal yang berbeda, dan garis
// gabungan akan naik-turun karena komposisinya, bukan karena kinerjanya.
const perPlatform = computed(() => {
  const hasil = {}
  for (const p of PLATFORM) hasil[p.key] = []
  const indeks = {}
  for (const r of weekly.value) {
    if (!hasil[r.platform]) continue
    const kunci = `${r.platform}|${r.outlet_code}`
    let o = indeks[kunci]
    if (!o) {
      o = { code: r.outlet_code, name: r.outlet_name, weeks: {} }
      indeks[kunci] = o
      hasil[r.platform].push(o)
    }
    const w = (o.weeks[r.week_start] ??= { followers: null, posts: 0, views: 0, engagement: 0 })
    // Pengikut dijumlah hanya bila ada pembacaannya; null tetap null supaya
    // minggu yang tak terbaca digambar putus, bukan sebagai nol.
    if (r.followers != null) w.followers = (w.followers ?? 0) + r.followers
    w.posts += r.posts
    w.views += r.views
    w.engagement += r.engagement
  }
  for (const k of Object.keys(hasil)) hasil[k].sort((a, b) => a.code.localeCompare(b.code))
  return hasil
})

const warnaOutlet = (code) => {
  const semua = [...new Set(weekly.value.map(r => r.outlet_code))].sort()
  const i = semua.indexOf(code)
  return WARNA_OUTLET[(i < 0 ? 0 : i) % WARNA_OUTLET.length]
}

function nilai(o, wk) {
  const w = o.weeks[wk]
  if (!w) return metrikAktif.value.key === 'followers' ? null : 0
  return w[metrikAktif.value.key]
}

const seriGrafik = (platform) => perPlatform.value[platform].map(o => ({
  name: o.code,
  data: daftarMinggu.value.map(wk => nilai(o, wk)),
}))

// adaAngka membedakan "belum ada apa-apa" dari "semuanya nol". Keduanya tampil
// sebagai grafik kosong, tetapi yang pertama menuntut tindakan dan yang kedua
// adalah temuan.
function adaAngka(platform) {
  const deret = perPlatform.value[platform]
  if (!deret.length) return false
  return deret.some(o => daftarMinggu.value.some(wk => {
    const v = nilai(o, wk)
    return v != null && v !== 0
  }))
}

function pesanKosong(platform) {
  if (!perPlatform.value[platform].length) {
    return `Belum ada akun ${NAMA_PLATFORM[platform]} yang didaftarkan.`
  }
  if (metrik.value === 'views') {
    return platform === 'instagram'
      ? 'Jangkauan Instagram tidak pernah ada di halaman publik — angka itu hanya hidup di layar Insights pemegang akun. Isi manual lewat tombol di daftar akun bila ingin memakainya.'
      : 'Halaman profil TikTok menyerahkan penghitung akun, bukan daftar videonya, jadi jumlah tontonan tidak ikut tertarik. Pakai metrik Interaksi, atau isi manual.'
  }
  if (metrik.value === 'engagement' && platform === 'instagram') {
    return 'Instagram tidak menyerahkan total suka di halaman profilnya. Pakai metrik Konten atau Pengikut, atau isi Interaksi manual.'
  }
  return `Belum ada angka ${metrikAktif.value.label.toLowerCase()} yang terkumpul. Perlu dua minggu pembacaan sebelum selisih mingguannya bisa dihitung.`
}

function opsiGrafik(platform) {
  const m = metrikAktif.value
  const kategori = daftarMinggu.value.map(rentangMinggu)
  const dasar = {
    chart: { toolbar: { show: false }, fontFamily: 'inherit', zoom: { enabled: false },
             stacked: m.tumpuk, animations: { enabled: false } },
    colors: perPlatform.value[platform].map(o => warnaOutlet(o.code)),
    grid: { borderColor: '#f1f5f9', padding: { left: 4, right: 4 } },
    // Legenda selalu ada supaya identitas outlet tidak pernah bergantung pada
    // warna saja; di ponsel ia otomatis membungkus ke beberapa baris.
    // Legenda selalu ada supaya identitas outlet tidak pernah bergantung pada
    // warna saja; di ponsel ia otomatis membungkus ke beberapa baris.
    legend: { position: 'bottom', fontSize: '11px', markers: { radius: 3 }, itemMargin: { horizontal: 6 } },
    // Label minggu dijarangkan, bukan diperkecil. Dua panel berdampingan hanya
    // punya separuh lebar layar; dua belas label yang dipaksa muat di situ
    // berubah jadi pagar miring yang tidak terbaca, dan tanggal persisnya toh
    // sudah muncul di tooltip saat ditunjuk.
    xaxis: {
      categories: kategori,
      tickAmount: Math.min(6, kategori.length),
      labels: { style: { fontSize: '10px' }, rotate: -40, rotateAlways: false, hideOverlappingLabels: true, trim: false },
      tooltip: { enabled: false },
    },
    yaxis: { labels: { formatter: ringkasAngka, style: { fontSize: '10px' } } },
    dataLabels: { enabled: false },
    tooltip: { shared: true, intersect: false, y: { formatter: v => (v == null ? 'belum terbaca' : angkaID(v)) } },
  }
  if (m.tipe === 'line') {
    return { ...dasar, stroke: { width: 2, curve: 'straight' }, markers: { size: 0, hover: { size: 5 } } }
  }
  return { ...dasar, plotOptions: { bar: { borderRadius: 3, columnWidth: '60%' } } }
}

// ── Ringkasan per platform ───────────────────────────────────
function ringkas(platform) {
  const akun = accounts.value.filter(a => a.platform === platform)
  return {
    akun: akun.length,
    bermasalah: akun.filter(a => statusAkun(a).buruk).length,
  }
}

function kartuRingkas(platform) {
  const deret = perPlatform.value[platform]
  const wks = daftarMinggu.value
  const total = (field) => deret.reduce((s, o) => s + wks.reduce((t, wk) => t + (o.weeks[wk]?.[field] ?? 0), 0), 0)

  // Pengikut: pembacaan terakhir yang ada, dibandingkan pembacaan pertama.
  let kini = 0, awal = 0, ada = false
  for (const o of deret) {
    const isi = wks.map(wk => o.weeks[wk]?.followers).filter(v => v != null)
    if (!isi.length) continue
    kini += isi[isi.length - 1]
    awal += isi[0]
    ada = true
  }
  const tambah = ada ? kini - awal : null

  return [
    { label: 'Pengikut', nilai: ada ? angkaID(kini) : '—', kaki: 'pembacaan terakhir' },
    { label: 'Bertambah', nilai: tambah == null ? '—' : (tambah >= 0 ? '+' : '−') + angkaID(Math.abs(tambah)),
      warna: tambah == null ? 'text-gray-400' : tambah > 0 ? 'text-emerald-600' : tambah < 0 ? 'text-red-600' : 'text-gray-600',
      kaki: `sepanjang ${wks.length} minggu` },
    { label: 'Konten', nilai: angkaID(total('posts')), kaki: 'terbit di rentang ini' },
    { label: 'Interaksi', nilai: ringkasAngka(total('engagement')), kaki: 'didapat di rentang ini' },
  ]
}

// ── Aksi per akun ────────────────────────────────────────────
function aksi(a) {
  return [
    { ikon: IC_AKSI.tarik, judul: a.auto_fetch ? 'Tarik sekarang' : 'Coba tarik — akun ini tidak ikut jadwal harian',
      mati: busy.scrape, kelas: 'text-emerald-700 hover:bg-emerald-50', aksi: () => tarikSatu(a) },
    { ikon: IC_AKSI.manual, judul: 'Isi angka manual per minggu',
      mati: false, kelas: 'text-sky-700 hover:bg-sky-50', aksi: () => bukaManual(a) },
    { ikon: IC_AKSI.ubah, judul: 'Ubah akun',
      mati: false, kelas: 'text-gray-600 hover:bg-gray-100', aksi: () => bukaUbah(a) },
    { ikon: IC_AKSI.hapus, judul: 'Hapus akun',
      mati: false, kelas: 'text-red-600 hover:bg-red-50', aksi: () => konfirmasiHapus(a) },
  ]
}

// ── Satu baris per outlet ────────────────────────────────────────────────────
//
// Sebelumnya satu baris per AKUN, sehingga satu outlet yang punya dua platform
// muncul dua kali dan tidak ada tempat untuk membandingkan keduanya. Dipivot
// jadi satu baris per outlet, pertanyaan "outlet ini di mana lebih kuat" bisa
// dijawab dengan melirik ke samping.
//
// Outlet yang BELUM punya akun sengaja ikut ditampilkan. Daftar yang hanya
// memuat yang sudah terdaftar tidak pernah bisa menunjukkan apa yang kurang —
// padahal justru itu yang perlu dikerjakan berikutnya.
const barisOutlet = computed(() => {
  const peta = new Map()
  const pastikan = (code, name) => {
    let o = peta.get(code)
    if (!o) { o = { code, name, plat: {} }; peta.set(code, o) }
    return o
  }

  for (const a of accounts.value) pastikan(a.outlet_code, a.outlet_name).plat[a.platform] = { akun: a }
  for (const g of outlets.value) {
    if (g.is_active === false) continue
    pastikan((g.code ?? '').trim(), g.name)
  }

  for (const o of peta.values()) {
    o.kolom = PLATFORM.map(p => {
      const sel = o.plat[p.key]
      if (!sel) return { platform: p, akun: null }
      const deret = perPlatform.value[p.key]?.find(x => x.code === o.code)
      const seri = deret ? daftarMinggu.value.map(wk => deret.weeks[wk]?.followers ?? null) : []
      const isi = seri.filter(v => v != null)
      return {
        platform: p,
        akun: sel.akun,
        seri,
        delta: isi.length >= 2 ? isi[isi.length - 1] - isi[0] : null,
      }
    })
  }
  return [...peta.values()].sort((a, b) => a.code.localeCompare(b.code))
})

// sparkline menggambar perjalanan pengikut sebagai jejak kecil di samping
// angkanya. Bukan hiasan: angka tunggal tidak pernah bisa menunjukkan ARAH,
// dan arah itulah yang dicari orang saat memindai daftar.
//
// Minggu yang tidak terbaca memutus jejaknya, tidak dijembatani — jembatan akan
// menggambarkan pertumbuhan mulus yang tidak pernah diukur.
function sparkline(seri, w = 68, h = 20) {
  if (!seri?.length) return ''
  const isi = seri.filter(v => v != null)
  if (isi.length < 2) return ''
  const min = Math.min(...isi)
  const span = (Math.max(...isi) - min) || 1
  const n = (seri.length - 1) || 1
  const x = (i) => (i / n) * (w - 2) + 1
  const y = (v) => h - 1 - ((v - min) / span) * (h - 2)
  let d = '', nyambung = false
  seri.forEach((v, i) => {
    if (v == null) { nyambung = false; return }
    d += `${nyambung ? 'L' : 'M'}${x(i).toFixed(1)} ${y(v).toFixed(1)} `
    nyambung = true
  })
  return d.trim()
}

// Membuka modal tambah dengan outlet dan platform yang sudah terisi, supaya
// kolom kosong di tabel bisa langsung ditindak dari tempatnya.
function tambahUntuk(code, platform) {
  const g = outlets.value.find(x => (x.code ?? '').trim() === code)
  modal.id = ''
  modal.error = ''
  Object.assign(form, { outlet_id: g?.id ?? '', platform, username: '', auto_fetch: true })
  modal.open = true
}

// ── Penarikan ────────────────────────────────────────────────
async function tarikSemua() { await jalankanTarik([]) }
async function tarikSatu(a) { await jalankanTarik([a.id]) }

async function jalankanTarik(ids) {
  busy.scrape = true
  hasil.value = null
  errorMsg.value = ''
  try {
    hasil.value = await socialApi.scrape(ids)
    accounts.value = (await socialApi.list()) ?? []
    await muatMingguan()
  } catch (err) {
    // Putaran yang bentrok bukan kerusakan — cukup diberitahu, jangan
    // ditampilkan sebagai galat merah yang memancing orang menekan lagi.
    const pesan = err?.message ?? 'Gagal menjalankan penarikan.'
    if (pesan.includes('sedang berjalan')) toast.info(pesan)
    else errorMsg.value = pesan
  } finally {
    busy.scrape = false
  }
}

// ── Tambah / ubah / hapus ────────────────────────────────────
function bukaTambah() {
  modal.id = ''
  modal.error = ''
  Object.assign(form, { outlet_id: '', platform: 'instagram', username: '', auto_fetch: true })
  modal.open = true
}

function bukaUbah(a) {
  modal.id = a.id
  modal.error = ''
  Object.assign(form, {
    outlet_id: a.outlet_id, platform: a.platform, username: a.username, auto_fetch: a.auto_fetch,
  })
  modal.open = true
}

// Ganti platform di modal ikut menyetel ulang saklar penarikan, karena
// kemampuannya memang melekat pada platformnya.
watch(() => form.platform, () => { if (!modal.id) form.auto_fetch = true })

async function simpanAkun() {
  modal.error = ''
  busy.simpan = true
  try {
    if (modal.id) {
      await socialApi.update(modal.id, {
        platform: form.platform, username: form.username, auto_fetch: form.auto_fetch,
      })
      toast.success('Akun diperbarui.')
    } else {
      await socialApi.create({ ...form })
      toast.success('Akun ditambahkan. Angkanya masuk setelah penarikan pertama.')
    }
    modal.open = false
    await muat()
  } catch (err) {
    modal.error = err?.message ?? 'Gagal menyimpan akun.'
  } finally {
    busy.simpan = false
  }
}

function konfirmasiHapus(a) {
  hapus.akun = a
  hapus.open = true
}

async function hapusAkun() {
  busy.hapus = true
  try {
    await socialApi.remove(hapus.akun.id)
    toast.success('Akun dihapus.')
    hapus.open = false
    await muat()
  } catch (err) {
    toast.error(err?.message ?? 'Gagal menghapus akun.')
  } finally {
    busy.hapus = false
  }
}

// ── Isian manual ─────────────────────────────────────────────
const barisMinggu = ref([])

async function bukaManual(a) {
  manual.akun = a
  manual.open = true
  barisMinggu.value = []
  busy.manual = true
  try {
    const [w, m] = await Promise.all([socialApi.weekly(minggu.value), socialApi.manualList(a.id)])
    rakitBaris((w ?? []).filter(r => r.account_id === a.id), m ?? [])
  } catch (err) {
    toast.error(err?.message ?? 'Gagal memuat isian manual.')
  } finally {
    busy.manual = false
  }
}

// Dirakit sekali tiap kali data dimuat, bukan dihitung ulang tiap render:
// barisnya MENYIMPAN apa yang sedang diketik, dan turunan yang melahirkan objek
// baru setiap kali dibaca akan menyapu ketikan yang belum sempat disimpan.
function rakitBaris(rows, manualRows) {
  const manualBy = Object.fromEntries(manualRows.map(m => [m.week_start, m]))
  barisMinggu.value = [...rows]
    .sort((a, b) => b.week_start.localeCompare(a.week_start))
    .map(r => {
      const m = manualBy[r.week_start]
      const isi = {
        followers: m?.followers ?? '',
        posts: m?.posts ?? '',
        views: m?.views ?? '',
        engagement: m?.engagement ?? '',
      }
      return reactive({
        week_start: r.week_start,
        adaManual: !!m,
        asal: {
          followers: r.followers == null ? '—' : String(r.followers),
          posts: String(r.posts), views: String(r.views), engagement: String(r.engagement),
        },
        isi: { ...isi },
        awal: { ...isi },
        busy: false,
      })
    })
}

const berubah = (r) => KOLOM_ISI.some(k => String(r.isi[k.field] ?? '') !== String(r.awal[k.field] ?? ''))

async function simpanMinggu(r) {
  r.busy = true
  try {
    const angka = (v) => {
      const s = String(v ?? '').trim()
      if (s === '') return null
      const n = Number(s)
      return Number.isFinite(n) && n >= 0 ? Math.round(n) : null
    }
    await socialApi.manualSave(manual.akun.id, {
      week_start: r.week_start,
      followers: angka(r.isi.followers),
      posts: angka(r.isi.posts),
      views: angka(r.isi.views),
      engagement: angka(r.isi.engagement),
    })
    toast.success(`Minggu ${rentangMinggu(r.week_start)} tersimpan.`)
    await bukaManual(manual.akun)
    await muatMingguan()
  } catch (err) {
    toast.error(err?.message ?? 'Gagal menyimpan tambalan.')
  } finally {
    r.busy = false
  }
}

// ── Format ───────────────────────────────────────────────────
const angkaID = (v) => Number(v ?? 0).toLocaleString('id-ID')

// Sumbu angka besar ditulis ringkas: "12,3 rb" terbaca sekilas, "12.300"
// memaksa mata menghitung digit.
function ringkasAngka(v) {
  const n = Number(v) || 0
  const koma = (x) => x.toFixed(1).replace('.', ',')
  if (Math.abs(n) >= 1e6) return koma(n / 1e6) + ' jt'
  if (Math.abs(n) >= 1e3) return koma(n / 1e3) + ' rb'
  return String(Math.round(n))
}

const BULAN = ['Jan', 'Feb', 'Mar', 'Apr', 'Mei', 'Jun', 'Jul', 'Agu', 'Sep', 'Okt', 'Nov', 'Des']

function rentangMinggu(weekStart) {
  if (!weekStart) return ''
  const a = new Date(`${weekStart}T00:00:00`)
  const b = new Date(a.getTime() + 6 * 86400000)
  return a.getMonth() === b.getMonth()
    ? `${a.getDate()}–${b.getDate()} ${BULAN[b.getMonth()]}`
    : `${a.getDate()} ${BULAN[a.getMonth()]}–${b.getDate()} ${BULAN[b.getMonth()]}`
}

function fmtTanggal(s) {
  if (!s) return ''
  const d = new Date(`${s}T00:00:00`)
  return `${d.getDate()} ${BULAN[d.getMonth()]}`
}

// Status penarikan dibaca dari dua tanda: kapan terakhir BERHASIL, dan apakah
// percobaan terakhir meninggalkan galat. Akun yang galat tetapi masih punya
// keberhasilan baru-baru ini tidak disebut mandek — pembacaan yang gagal
// sesekali memang wajar dan tidak perlu memancing orang bertindak.
function statusAkun(a) {
  if (!a.auto_fetch) return { teks: 'Isian manual', warna: 'text-sky-600', titik: 'bg-sky-400', buruk: false }
  if (!a.last_ok_at) {
    return a.last_error
      ? { teks: 'Belum pernah berhasil', warna: 'text-red-600', titik: 'bg-red-500', buruk: true }
      : { teks: 'Belum pernah ditarik', warna: 'text-gray-400', titik: 'bg-gray-300', buruk: false }
  }
  const umurHari = (Date.now() - new Date(a.last_ok_at).getTime()) / 86400000
  if (umurHari > 8) return { teks: 'Mandek', warna: 'text-red-600', titik: 'bg-red-500', buruk: true }
  if (a.last_error) return { teks: 'Gagal terakhir', warna: 'text-amber-600', titik: 'bg-amber-400', buruk: false }
  return { teks: 'Lancar', warna: 'text-emerald-600', titik: 'bg-emerald-500', buruk: false }
}

// Status jadi titik berwarna, bukan kalimat. Judul lengkapnya — termasuk pesan
// galat — dipindahkan ke tooltip: keadaan sehat adalah keadaan yang biasa, dan
// mengulang kata "Lancar" pada tiap sel memakan ruang untuk mengabarkan bahwa
// tidak ada yang perlu dikabarkan.
function statusJudul(a) {
  const st = statusAkun(a)
  return a.last_error ? `${st.teks} — ${a.last_error}` : st.teks
}
</script>

<style scoped>
.isian {
  border: 1px solid #e5e7eb;
  border-radius: 0.375rem;
  padding: 0.25rem 0.5rem;
  font-size: 0.875rem;
  font-variant-numeric: tabular-nums;
  transition: border-color .15s, box-shadow .15s;
}
.isian:focus {
  outline: none;
  border-color: #10b981;
  box-shadow: 0 0 0 3px rgba(16, 185, 129, .25);
}
.isian::placeholder { color: #d1d5db; }
/* Panah penambah pada input angka mempersempit kolom yang sudah sempit di
   ponsel, dan angka di sini selalu diketik, tidak pernah dinaik-turunkan. */
.isian::-webkit-outer-spin-button,
.isian::-webkit-inner-spin-button { -webkit-appearance: none; margin: 0; }
.isian[type='number'] { -moz-appearance: textfield; appearance: textfield; }
</style>
