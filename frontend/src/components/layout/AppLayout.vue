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
  --anti-blue: #1b00ff;
  --anti-green: #00ff00;
  --anti-cyan: #00e5ff;
  --anti-muted: #475569;
  --anti-link: #1428ff;
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

.app-shell.app-shell-anti a:not(.dropdown-item):not(.sidebar-link) {
  color: var(--anti-link) !important;
  text-decoration: underline;
  text-decoration-thickness: 3px;
  text-underline-offset: 3px;
}

.app-shell.app-shell-anti .sidebar {
  background:
    linear-gradient(90deg, transparent calc(100% - 8px), var(--anti-ink) calc(100% - 8px)),
    var(--anti-paper) !important;
}

.app-shell.app-shell-anti .sidebar-header {
  border-bottom: 4px solid var(--anti-ink) !important;
  background: linear-gradient(90deg, #ffffff 0%, #fff9b8 100%) !important;
}

.app-shell.app-shell-anti .sidebar-logo {
  border: 3px solid var(--anti-ink) !important;
  border-radius: 50% !important;
  background: var(--anti-yellow) !important;
  box-shadow: 4px 4px 0 var(--anti-ink) !important;
}

.app-shell.app-shell-anti .sidebar-brand-title {
  color: var(--anti-ink) !important;
  text-shadow: 2px 2px 0 var(--anti-cyan);
}

.app-shell.app-shell-anti .sidebar-brand button {
  border: 2px solid var(--anti-ink) !important;
  border-radius: 0 !important;
  background: var(--anti-paper) !important;
  box-shadow: 3px 3px 0 var(--anti-ink) !important;
  color: var(--anti-ink) !important;
  text-transform: uppercase;
}

.app-shell.app-shell-anti .sidebar-nav {
  background:
    repeating-linear-gradient(135deg, rgb(255 235 59 / 0.10) 0 10px, transparent 10px 22px),
    transparent !important;
}

.app-shell.app-shell-anti .sidebar-section-title {
  color: var(--anti-muted) !important;
  letter-spacing: 0.08em;
}

.app-shell.app-shell-anti .sidebar-section-title-text {
  background: var(--anti-paper);
  color: var(--anti-muted) !important;
}

.app-shell.app-shell-anti .sidebar-link {
  position: relative;
  border: 3px solid transparent !important;
  border-radius: 0.2rem !important;
  color: var(--anti-ink) !important;
  font-weight: 950;
  letter-spacing: 0.01em;
  text-decoration: none !important;
  text-transform: uppercase;
}

.app-shell.app-shell-anti .sidebar-link :is(svg, .sidebar-svg-icon) {
  color: var(--anti-blue) !important;
  filter: drop-shadow(2px 2px 0 rgb(255 235 59 / 0.92));
}

.app-shell.app-shell-anti .sidebar-link:hover {
  border-color: var(--anti-ink) !important;
  background: var(--anti-yellow) !important;
  box-shadow: 4px 4px 0 var(--anti-ink) !important;
  color: var(--anti-ink) !important;
  transform: rotate(-0.9deg) translate(-1px, -1px);
}

.app-shell.app-shell-anti .sidebar-link-active {
  border-color: var(--anti-ink) !important;
  background:
    linear-gradient(90deg, rgb(0 255 0 / 0.18), rgb(255 255 255 / 0.92) 55%),
    var(--anti-paper) !important;
  box-shadow: 5px 5px 0 var(--anti-red) !important;
  color: var(--anti-ink) !important;
  text-decoration: none !important;
}

.app-shell.app-shell-anti .sidebar-link-active :is(svg, .sidebar-svg-icon) {
  color: var(--anti-red) !important;
  filter: drop-shadow(2px 2px 0 var(--anti-yellow));
}

.app-shell.app-shell-anti .sidebar-link-active::after {
  content: '!';
  position: absolute;
  right: 0.55rem;
  top: 50%;
  display: grid;
  width: 1rem;
  height: 1rem;
  place-items: center;
  border: 2px solid var(--anti-ink);
  background: var(--anti-yellow);
  color: var(--anti-ink);
  font-size: 0.72rem;
  line-height: 1;
  transform: translateY(-50%) rotate(8deg);
}

.app-shell.app-shell-anti .sidebar-link-collapsed.sidebar-link-active::after {
  right: 0.25rem;
  top: 0.22rem;
  transform: rotate(8deg);
}

.app-shell.app-shell-anti .sidebar > .mt-auto {
  border-top: 4px solid var(--anti-ink) !important;
  background: linear-gradient(90deg, #ffffff 0%, #efffff 100%) !important;
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
