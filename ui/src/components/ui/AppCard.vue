<!--
  AppCard.vue — White panel with soft shadow
  ───────────────────────────────────────────────────────────────
  Slots:
    default  — card content
    header   — optional header row (title + actions)
    footer   — optional footer row

  Props:
    padding  : boolean  — add p-6 if true (default true)

  Usage:
    <AppCard>
      <template #header>
        <h2 class="text-lg font-semibold">Title</h2>
        <AppButton size="sm">New</AppButton>
      </template>
      ... content ...
    </AppCard>
-->
<template>
  <div :class="['tahoe-card min-w-0 max-w-full', padding ? 'p-3.5 sm:p-5' : '']">
    <!-- Specular top edge -->
    <div class="card-specular" aria-hidden="true" />

    <!-- Optional header slot -->
    <div
      v-if="$slots.header"
      class="flex flex-wrap items-center justify-between gap-2 mb-4"
    >
      <slot name="header" />
    </div>

    <slot />

    <!-- Optional footer slot -->
    <div
      v-if="$slots.footer"
      class="mt-4 pt-4 border-t border-glass flex flex-wrap items-center justify-end gap-2"
    >
      <slot name="footer" />
    </div>
  </div>
</template>

<script setup>
defineProps({
  padding: { type: Boolean, default: true },
})
</script>

<style scoped>
.tahoe-card {
  position: relative;
  border-radius: 1.1rem;
  background: rgba(255,255,255,.78);
  backdrop-filter: blur(22px) saturate(170%);
  -webkit-backdrop-filter: blur(22px) saturate(170%);
  border: 1px solid rgba(255,255,255,.68);
  box-shadow:
    0 2px 16px rgba(0,0,0,.07),
    0 1px 0 rgba(255,255,255,.92) inset;
  transition: box-shadow .18s ease, transform .18s ease;
}
.tahoe-card:hover {
  box-shadow:
    0 6px 28px rgba(0,0,0,.10),
    0 1px 0 rgba(255,255,255,.92) inset;
}
/* top-left glare shimmer */
.card-specular {
  display: block;
  position: absolute;
  inset: 0;
  border-radius: inherit;
  pointer-events: none;
  background: linear-gradient(135deg, rgba(255,255,255,.48) 0%, rgba(255,255,255,0) 52%);
}
</style>
