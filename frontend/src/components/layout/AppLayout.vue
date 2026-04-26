<template>
  <div
    class="app-shell min-h-screen bg-gray-50 dark:bg-dark-950"
    :class="{ 'app-shell-anti': antiDesignMode }"
  >
    <!-- Background Decoration -->
    <div class="pointer-events-none fixed inset-0 bg-mesh-gradient"></div>

    <!-- Sidebar -->
    <AppSidebar />

    <!-- Main Content Area -->
    <div
      class="relative min-h-screen transition-all duration-300"
      :class="[sidebarCollapsed ? 'lg:ml-[72px]' : 'lg:ml-64']"
    >
      <!-- Header -->
      <AppHeader />

      <!-- Main Content -->
      <main class="p-4 md:p-6 lg:p-8">
        <slot />
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import '@/styles/onboarding.css'
import { computed, onMounted } from 'vue'
import { useAppStore } from '@/stores'
import { useAuthStore } from '@/stores/auth'
import { useAntiDesignMode } from '@/composables/useAntiDesignMode'
import { useOnboardingTour } from '@/composables/useOnboardingTour'
import { useOnboardingStore } from '@/stores/onboarding'
import AppSidebar from './AppSidebar.vue'
import AppHeader from './AppHeader.vue'

const appStore = useAppStore()
const authStore = useAuthStore()
const { antiDesignMode } = useAntiDesignMode()
const sidebarCollapsed = computed(() => appStore.sidebarCollapsed)
const isAdmin = computed(() => authStore.user?.role === 'admin')

const { replayTour } = useOnboardingTour({
  storageKey: isAdmin.value ? 'admin_guide' : 'user_guide',
  autoStart: true
})

const onboardingStore = useOnboardingStore()

onMounted(() => {
  onboardingStore.setReplayCallback(replayTour)
})

defineExpose({ replayTour })
</script>

<style>
.app-shell {
  isolation: isolate;
}

.app-shell.app-shell-anti {
  --anti-ink: #050505;
  --anti-paper: #ffffff;
  --anti-yellow: #ffeb3b;
  --anti-red: #ff0000;
  --anti-blue: #0000ff;
  --anti-green: #00ff00;
  color: var(--anti-ink);
  background:
    radial-gradient(circle at 12% 18%, rgb(255 0 0 / 0.22) 0 8rem, transparent 8.2rem),
    radial-gradient(circle at 84% 12%, rgb(0 255 0 / 0.25) 0 9rem, transparent 9.2rem),
    repeating-linear-gradient(135deg, rgb(5 5 5 / 0.08) 0 10px, transparent 10px 22px),
    linear-gradient(135deg, #ffeb3b 0%, #ffffff 48%, #00ffff 100%) !important;
  font-family: Impact, "Arial Black", "Comic Sans MS", ui-sans-serif, system-ui, sans-serif;
  letter-spacing: -0.015em;
}

.dark .app-shell.app-shell-anti {
  background:
    radial-gradient(circle at 12% 18%, rgb(255 0 0 / 0.28) 0 8rem, transparent 8.2rem),
    radial-gradient(circle at 84% 12%, rgb(0 255 0 / 0.28) 0 9rem, transparent 9.2rem),
    repeating-linear-gradient(135deg, rgb(255 255 255 / 0.10) 0 10px, transparent 10px 22px),
    linear-gradient(135deg, #120012 0%, #ff00ff 46%, #ffeb3b 100%) !important;
}

.app-shell.app-shell-anti .bg-mesh-gradient {
  background:
    repeating-linear-gradient(90deg, rgb(5 5 5 / 0.18) 0 12px, transparent 12px 24px),
    repeating-linear-gradient(0deg, rgb(0 0 255 / 0.12) 0 18px, transparent 18px 36px) !important;
  opacity: 0.48;
}

.app-shell.app-shell-anti :is(header.glass, aside, .card, .dropdown, .modal, .drawer, .table-card) {
  border: 4px solid var(--anti-ink) !important;
  border-radius: 0.45rem !important;
  background: var(--anti-paper) !important;
  box-shadow: 8px 8px 0 var(--anti-ink) !important;
  color: var(--anti-ink) !important;
  backdrop-filter: none !important;
}

.app-shell.app-shell-anti header.glass {
  z-index: 50;
  transform: rotate(-0.25deg);
}

.app-shell.app-shell-anti aside {
  transform: rotate(0.15deg);
}

.app-shell.app-shell-anti :is(.btn, .btn-primary, .btn-secondary, .btn-ghost, .dropdown-item, button:not(.chartjs-render-monitor)) {
  border-radius: 0.2rem !important;
  font-weight: 950;
  text-transform: uppercase;
}

.app-shell.app-shell-anti :is(.btn-primary, .btn-secondary, .btn-ghost, .dropdown-item) {
  border: 3px solid var(--anti-ink) !important;
  background: var(--anti-yellow) !important;
  box-shadow: 4px 4px 0 var(--anti-ink) !important;
  color: var(--anti-ink) !important;
}

.app-shell.app-shell-anti :is(.btn-primary, .btn-secondary, .btn-ghost, .dropdown-item):hover {
  background: var(--anti-green) !important;
  transform: rotate(-1.5deg) translate(-1px, -1px);
}

.app-shell.app-shell-anti :is(input, textarea, select, .select-trigger, .date-picker-trigger) {
  border: 3px solid var(--anti-ink) !important;
  border-radius: 0 !important;
  background: var(--anti-paper) !important;
  box-shadow: 4px 4px 0 var(--anti-blue) !important;
  color: var(--anti-ink) !important;
}

.app-shell.app-shell-anti :is(.text-gray-900, .text-gray-800, .text-gray-700, .text-gray-600, .text-gray-500, .text-dark-400, .dark\:text-white, .dark\:text-gray-300, .dark\:text-gray-400, .dark\:text-dark-400) {
  color: var(--anti-ink) !important;
}

.app-shell.app-shell-anti :is(.bg-gray-50, .bg-white, .dark\:bg-dark-800, .dark\:bg-dark-900) {
  background-color: var(--anti-paper) !important;
}

.app-shell.app-shell-anti a:not(.dropdown-item) {
  color: var(--anti-blue) !important;
  text-decoration: underline;
  text-decoration-thickness: 3px;
  text-underline-offset: 3px;
}

.app-shell.app-shell-anti table {
  border: 3px solid var(--anti-ink);
  background: var(--anti-paper);
}

.app-shell.app-shell-anti th {
  background: var(--anti-ink) !important;
  color: var(--anti-yellow) !important;
}

.app-shell.app-shell-anti td {
  border-top: 2px solid var(--anti-ink) !important;
}

@media (prefers-reduced-motion: no-preference) {
  .app-shell.app-shell-anti :is(.card, .dropdown, .btn, .dropdown-item, button) {
    transition:
      transform 140ms ease,
      box-shadow 140ms ease,
      background-color 140ms ease;
  }
}
</style>
