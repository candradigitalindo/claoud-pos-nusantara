<!--
  Login.vue — macOS Tahoe 26 "Liquid Glass" login card
  ───────────────────────────────────────────────────────────────
  Route: /login   (wrapped in AuthLayout)
-->
<template>
  <div class="glass-card">

    <!-- ── Top specular highlight bar ── -->
    <div class="specular" aria-hidden="true" />

    <!-- ── Brand header ── -->
    <div class="text-center mb-7">
      <div class="app-icon mx-auto mb-4">
        <svg class="w-8 h-8" viewBox="0 0 32 32" fill="none">
          <rect x="2"  y="2"  width="13" height="13" rx="3" fill="rgba(126,184,154,.9)" />
          <rect x="17" y="2"  width="13" height="13" rx="3" fill="rgba(126,184,154,.55)" />
          <rect x="2"  y="17" width="13" height="13" rx="3" fill="rgba(126,184,154,.55)" />
          <rect x="17" y="17" width="13" height="13" rx="3" fill="rgba(126,184,154,.75)" />
        </svg>
      </div>
      <h1 class="card-title">Cloud POS</h1>
      <p class="card-sub">Management Dashboard</p>
    </div>

    <!-- ── Form ── -->
    <form class="space-y-4" @submit.prevent="submit">

      <!-- Pesan sesi: mengapa pengguna kembali ke halaman ini (password
           diganti, sesi dicabut server, dsb. — lihat utils/session.js). -->
      <Transition name="slide-down">
        <div v-if="notice && !errorMsg" class="notice-toast" :class="`notice-toast--${notice.type}`" role="status">
          <svg class="w-3.5 h-3.5 flex-shrink-0 mt-px" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <circle cx="12" cy="12" r="10" /><line x1="12" y1="16" x2="12" y2="12" /><line x1="12" y1="8" x2="12.01" y2="8" />
          </svg>
          <span>{{ notice.message }}</span>
        </div>
      </Transition>

      <!-- Error toast -->
      <Transition name="slide-down">
        <div v-if="errorMsg" class="err-toast">
          <svg class="w-3.5 h-3.5 flex-shrink-0" fill="currentColor" viewBox="0 0 20 20">
            <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd"/>
          </svg>
          {{ errorMsg }}
        </div>
      </Transition>

      <!-- Username -->
      <div class="field">
        <label class="field-label">Username</label>
        <div class="glass-input" :class="{ 'is-error': fieldErrors.username }">
          <svg class="f-icon" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.6"
              d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"/>
          </svg>
          <input
            v-model="form.username"
            type="text" placeholder="your_username"
            autocomplete="username"
            :disabled="loading"
            class="f-input"
          />
        </div>
        <p v-if="fieldErrors.username" class="f-err">{{ fieldErrors.username }}</p>
      </div>

      <!-- Password -->
      <div class="field">
        <label class="field-label">Password</label>
        <div class="glass-input" :class="{ 'is-error': fieldErrors.password }">
          <svg class="f-icon" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.6"
              d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"/>
          </svg>
          <input
            v-model="form.password"
            :type="showPassword ? 'text' : 'password'"
            placeholder="••••••••"
            autocomplete="current-password"
            :disabled="loading"
            class="f-input"
          />
          <button type="button" class="eye-btn" @click="showPassword = !showPassword" tabindex="-1">
            <svg v-if="!showPassword" class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.6" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"/>
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.6" d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"/>
            </svg>
            <svg v-else class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.6" d="M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.543-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.88 9.88l-3.29-3.29m7.532 7.532l3.29 3.29M3 3l3.59 3.59m0 0A9.953 9.953 0 0112 5c4.478 0 8.268 2.943 9.543 7a10.025 10.025 0 01-4.132 5.411m0 0L21 21"/>
            </svg>
          </button>
        </div>
        <p v-if="fieldErrors.password" class="f-err">{{ fieldErrors.password }}</p>
      </div>

      <!-- Submit -->
      <button type="submit" :disabled="loading" class="liquid-btn">
        <span class="liquid-shine" aria-hidden="true" />
        <span v-if="loading" class="btn-spinner" />
        <span v-else class="flex items-center gap-2 relative z-10">
          Sign in
          <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 7l5 5m0 0l-5 5m5-5H6"/>
          </svg>
        </span>
      </button>
    </form>

    <!-- Footer pill -->
    <div class="footer-pill">
      Nusantara POS &nbsp;&middot;&nbsp; Cloud Edition
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth.js'
import { takeLoginNotice } from '@/utils/session.js'

const router    = useRouter()
const authStore = useAuthStore()

const form         = reactive({ username: '', password: '' })
const fieldErrors  = reactive({ username: '', password: '' })
const loading      = ref(false)
const errorMsg     = ref('')
const showPassword = ref(false)
// Pesan titipan (ditampilkan sekali), mis. "Password diganti, silakan login ulang".
const notice       = ref(null)
onMounted(() => { notice.value = takeLoginNotice() })

async function submit() {
  fieldErrors.username = form.username ? '' : 'Username wajib diisi'
  fieldErrors.password = form.password ? '' : 'Password wajib diisi'
  errorMsg.value = ''
  if (fieldErrors.username || fieldErrors.password) return
  notice.value = null

  loading.value = true
  try {
    await authStore.login(form.username, form.password)
    router.push(authStore.redirectTo || '/')
  } catch (err) {
    errorMsg.value = err?.message || 'Username atau password salah.'
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
/* ───────────────────────────────────────────────────────
   Liquid Glass Card  —  macOS Tahoe 26 aesthetic
   Key:  heavy blur + translucent tint + specular edge
─────────────────────────────────────────────────────── */
.glass-card {
  position: relative;
  width: 100%;
  max-width: 380px;
  background:
    linear-gradient(
      145deg,
      rgba(255,255,255,.18) 0%,
      rgba(255,255,255,.08) 40%,
      rgba(126,184,154,.1) 100%
    );
  backdrop-filter: blur(36px) saturate(160%) brightness(120%);
  -webkit-backdrop-filter: blur(36px) saturate(160%) brightness(120%);
  border-radius: 2rem;
  border: 1px solid rgba(255,255,255,.22);
  padding: 2.25rem 2rem 2rem;
  box-shadow:
    0 32px 80px rgba(0,0,0,.55),
    0 8px  24px rgba(0,0,0,.3),
    inset 0 0 60px rgba(126,184,154,.05),
    0 1px 0 rgba(126,184,154,.15) inset;
  animation: card-rise .5s cubic-bezier(.22,.68,0,1.2) both;
}
@keyframes card-rise {
  from { opacity: 0; transform: translateY(28px) scale(.96); }
  to   { opacity: 1; transform: translateY(0)    scale(1); }
}

/* Specular highlight */
.specular {
  position: absolute;
  top: 0; left: 12%; right: 12%;
  height: 1px;
  background: linear-gradient(90deg,
    transparent,
    rgba(255,255,255,.55) 30%,
    rgba(255,255,255,.7)  50%,
    rgba(255,255,255,.55) 70%,
    transparent
  );
  border-radius: 0 0 4px 4px;
}

/* App icon */
.app-icon {
  width: 60px; height: 60px;
  border-radius: 16px;
  background:
    linear-gradient(145deg, rgba(255,255,255,.18), rgba(255,255,255,.05)),
    rgba(126,184,154,.15);
  border: 1px solid rgba(255,255,255,.2);
  display: flex; align-items: center; justify-content: center;
  box-shadow:
    0 8px 24px rgba(0,0,0,.3),
    inset 0 1px 0 rgba(255,255,255,.25);
}

/* Typography */
.card-title {
  font-size: 1.5rem; font-weight: 700;
  letter-spacing: -.025em; color: rgba(255,255,255,.92);
}
.card-sub {
  font-size: .7rem; letter-spacing: .12em; text-transform: uppercase;
  color: rgba(255,255,255,.35); margin-top: .25rem;
}

/* Fields */
.field { display: flex; flex-direction: column; gap: .3rem; }
.field-label {
  font-size: .7rem; font-weight: 600; color: rgba(168,203,191,.7);
  letter-spacing: .1em; text-transform: uppercase; padding-left: .25rem;
}

/* Glass input */
.glass-input {
  display: flex; align-items: center;
  background: linear-gradient(160deg, rgba(255,255,255,.17), rgba(255,255,255,.03));
  border: 1px solid rgba(255,255,255,.2);
  border-radius: .875rem;
  backdrop-filter: blur(8px);
  transition: border-color .2s, box-shadow .2s, background .2s;
}
.glass-input:focus-within {
  border-color: rgba(126,184,154,.5);
  background: linear-gradient(160deg, rgba(255,255,255,.12), rgba(126,184,154,.06));
  box-shadow: 0 0 0 3px rgba(126,184,154,.12), inset 0 1px 0 rgba(255,255,255,.12);
}
.glass-input.is-error {
  border-color: rgba(252,165,165,.45);
  box-shadow: 0 0 0 3px rgba(252,165,165,.1);
}

.f-icon {
  width: 1rem; height: 1rem; color: rgba(126,184,154,.55);
  margin-left: .875rem; flex-shrink: 0; transition: color .2s;
}
.glass-input:focus-within .f-icon { color: rgba(126,184,154,.9); }

.f-input {
  flex: 1; background: transparent; border: none; outline: none;
  padding: .72rem .75rem; font-size: .875rem;
  color: rgba(255,255,255,.88); caret-color: #7eb89a;
}
.f-input::placeholder { color: rgba(255,255,255,.22); }
.f-input:disabled { opacity: .4; cursor: not-allowed; }

.eye-btn {
  padding: .5rem .875rem; color: rgba(255,255,255,.28);
  background: none; border: none; cursor: pointer; transition: color .15s;
}
.eye-btn:hover { color: rgba(126,184,154,.8); }

.f-err { font-size: .7rem; color: #fca5a5; padding-left: .25rem; }

/* Error toast */
.err-toast {
  display: flex; align-items: center; gap: .5rem;
  background: rgba(239,68,68,.15); border: 1px solid rgba(239,68,68,.3);
  backdrop-filter: blur(8px); border-radius: .75rem;
  padding: .625rem .875rem; font-size: .8rem; color: #fca5a5;
}
/* Pesan sesi (info / berhasil / peringatan) */
.notice-toast {
  display: flex; align-items: flex-start; gap: .5rem;
  backdrop-filter: blur(8px); border-radius: .75rem;
  padding: .625rem .875rem; font-size: .8rem; line-height: 1.35;
  background: rgba(126,184,154,.14); border: 1px solid rgba(126,184,154,.35); color: #c9ead8;
}
.notice-toast--success { background: rgba(16,185,129,.15); border-color: rgba(16,185,129,.35); color: #a7f3d0; }
.notice-toast--warning { background: rgba(245,158,11,.14); border-color: rgba(245,158,11,.35); color: #fde68a; }
.slide-down-enter-active, .slide-down-leave-active { transition: all .25s cubic-bezier(.22,.68,0,1.2); }
.slide-down-enter-from, .slide-down-leave-to       { opacity: 0; transform: translateY(-8px); }

/* Liquid button */
.liquid-btn {
  position: relative; overflow: hidden;
  width: 100%; padding: .8rem;
  border-radius: .875rem; font-size: .9rem; font-weight: 700;
  color: rgba(255,255,255,.92); cursor: pointer;
  background: linear-gradient(135deg, rgba(126,184,154,.35) 0%, rgba(90,145,112,.5) 100%);
  border: 1px solid rgba(126,184,154,.4);
  box-shadow: 0 4px 20px rgba(126,184,154,.2), inset 0 1px 0 rgba(255,255,255,.15);
  transition: transform .15s, box-shadow .15s, filter .15s;
}
.liquid-btn:hover:not(:disabled) {
  filter: brightness(1.1);
  box-shadow: 0 6px 28px rgba(126,184,154,.3), inset 0 1px 0 rgba(255,255,255,.15);
  transform: translateY(-1px);
}
.liquid-btn:active:not(:disabled) { transform: translateY(0); }
.liquid-btn:disabled { opacity: .5; cursor: not-allowed; }

/* Shimmer sweep */
.liquid-shine {
  position: absolute; inset: 0;
  background: linear-gradient(105deg, transparent 35%, rgba(255,255,255,.18) 50%, transparent 65%);
  background-size: 200% 100%; background-position: -100% 0;
  pointer-events: none;
  transition: background-position .6s ease;
}
.liquid-btn:hover .liquid-shine { background-position: 200% 0; }

/* Spinner */
.btn-spinner {
  display: block; margin: 0 auto;
  width: 18px; height: 18px;
  border: 2px solid rgba(255,255,255,.3); border-top-color: rgba(255,255,255,.85);
  border-radius: 50%; animation: spin .7s linear infinite;
}
@keyframes spin { to { transform: rotate(360deg); } }

/* Footer pill */
.footer-pill {
  margin-top: 1.5rem; text-align: center;
  font-size: .7rem; color: rgba(255,255,255,.2); letter-spacing: .04em;
}
</style>
