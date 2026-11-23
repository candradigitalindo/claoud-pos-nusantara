<!--
  OutletDetail.vue — Outlet detail shell with tab navigation (Tahoe Glass)
  Route: /outlets/:id   meta: { title: 'Detail Outlet' }
-->
<template>
  <div class="detail-root">

    <!-- Header -->
    <div class="detail-header">
      <RouterLink to="/outlets" class="back-link">
        <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
          <polyline points="15 18 9 12 15 6"/>
        </svg>
        Outlet
      </RouterLink>
      <span class="hd-sep">/</span>

      <div v-if="loadingOutlet" class="hd-skel" />
      <template v-else>
        <div class="hd-avatar">{{ outlet?.name?.charAt(0)?.toUpperCase() ?? 'O' }}</div>
        <div class="hd-info">
          <h1 class="hd-name">{{ outlet?.name ?? `Outlet #${outletId}` }}</h1>
          <span class="hd-code">{{ outlet?.code ?? '' }}</span>
        </div>
        <span :class="['status-badge', outlet?.is_active ? 'status-active' : 'status-inactive']">
          <span class="status-dot" />{{ outlet?.is_active ? 'Aktif' : 'Nonaktif' }}
        </span>
      </template>
    </div>

    <!-- Tab bar -->
    <div class="tab-bar">
      <RouterLink
        v-for="tab in TAB_ITEMS"
        :key="tab.path"
        :to="`/outlets/${outletId}/${tab.path}`"
        :class="['tab-link', isTabActive(tab.path) ? 'tab-link--active' : '']"
      >{{ tab.label }}</RouterLink>
    </div>

    <!-- Child page -->
    <RouterView :outlet="outlet" :outletId="outletId" @refresh="fetchOutlet" />

  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute, RouterLink, RouterView } from 'vue-router'
import { outletsApi } from '@/api/outlets.js'
import { useRealtime } from '@/utils/realtime.js'

const route    = useRoute()
const outletId = computed(() => route.params.id)

const outlet        = ref(null)
const loadingOutlet = ref(true)

async function fetchOutlet({ silent = false } = {}) {
  if (!silent) loadingOutlet.value = true
  try { outlet.value = await outletsApi.get(outletId.value) } catch {}
  finally { if (!silent) loadingOutlet.value = false }
}

onMounted(fetchOutlet)
useRealtime(['transaction', 'order', 'cashier_shift', 'device'], (ev) => {
  if (!ev?.outlet_id || ev.outlet_id === outletId.value) fetchOutlet({ silent: true })
})

function isTabActive(path) {
  return route.path.includes(`/outlets/${outletId.value}/${path}`)
}

const TAB_ITEMS = [
  { path: 'info',         label: 'Info & API Key' },
  { path: 'transactions', label: 'Transaksi'      },
  { path: 'orders',       label: 'Order'          },
  { path: 'products',     label: 'Produk'         },
  { path: 'categories',   label: 'Kategori'       },
  { path: 'printers',     label: 'Printer'        },
  { path: 'sync-logs',    label: 'Sync Log'       },
  { path: 'conflicts',    label: 'Konflik'        },
  { path: 'procurement',  label: 'Pengadaan'      },
]
</script>

<style scoped>
.detail-root { display: flex; flex-direction: column; gap: 1.1rem; }

.detail-header {
  display: flex; align-items: center; gap: .6rem; flex-wrap: wrap;
  padding: .85rem 1.25rem; border-radius: 1rem;
  background: rgba(255,255,255,.78); backdrop-filter: blur(20px) saturate(170%);
  border: 1px solid rgba(255,255,255,.7); box-shadow: 0 2px 12px rgba(0,0,0,.06);
}
.back-link {
  display: inline-flex; align-items: center; gap: .25rem;
  font-size: .78rem; font-weight: 600; color: #5a7866; text-decoration: none; transition: color .13s;
}
.back-link:hover { color: #1a5c38; }
.hd-sep { color: #cad5cc; font-size: .85rem; }
.hd-skel { height: 18px; width: 140px; border-radius: 5px; background: linear-gradient(90deg,#e5eae7 25%,#f0f4f1 50%,#e5eae7 75%); background-size: 200% 100%; animation: shimmer 1.4s infinite; }
@keyframes shimmer { 0%{background-position:200% 0} 100%{background-position:-200% 0} }
.hd-avatar {
  width: 32px; height: 32px; border-radius: .55rem; flex-shrink: 0;
  background: linear-gradient(135deg, #1a5c38, #0f4226); color: #fff;
  font-size: .85rem; font-weight: 800; display: flex; align-items: center; justify-content: center;
  box-shadow: 0 2px 6px rgba(15,66,38,.28);
}
.hd-info { display: flex; align-items: baseline; gap: .45rem; flex: 1; min-width: 0; }
.hd-name { font-size: .95rem; font-weight: 800; color: #0f2d1d; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.hd-code { font-size: .68rem; font-family: monospace; font-weight: 700; color: #2d8f56; background: rgba(45,143,86,.1); padding: .1rem .42rem; border-radius: .35rem; white-space: nowrap; }

.status-badge { display: inline-flex; align-items: center; gap: .28rem; padding: .18rem .55rem; border-radius: 999px; font-size: .65rem; font-weight: 700; flex-shrink: 0; }
.status-active   { background: rgba(22,163,74,.13); color: #16a34a; }
.status-inactive { background: rgba(107,114,128,.11); color: #6b7280; }
.status-dot { width: 5px; height: 5px; border-radius: 50%; background: currentColor; }
.status-active .status-dot { animation: pulse-dot 2s ease-in-out infinite; }
@keyframes pulse-dot { 0%,100%{opacity:1} 50%{opacity:.4} }

.tab-bar {
  display: flex; gap: .2rem; overflow-x: auto; padding: .3rem;
  border-radius: .85rem; background: rgba(255,255,255,.72); backdrop-filter: blur(16px) saturate(160%);
  border: 1px solid rgba(255,255,255,.65); box-shadow: 0 1px 8px rgba(0,0,0,.05);
  scrollbar-width: none;
}
.tab-bar::-webkit-scrollbar { display: none; }
.tab-link {
  flex-shrink: 0; padding: .38rem .85rem; border-radius: .6rem;
  font-size: .78rem; font-weight: 600; color: #5a7866; text-decoration: none;
  transition: background .14s, color .14s; white-space: nowrap;
}
.tab-link:hover { background: rgba(45,143,86,.07); color: #2d5c40; }
.tab-link--active { background: linear-gradient(135deg, #1a5c38, #0f4226); color: #fff; box-shadow: 0 2px 8px rgba(15,66,38,.28); }
</style>
