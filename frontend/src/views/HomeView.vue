<template>
  <!-- Custom Home Content: Full Page Mode -->
  <div v-if="homeContent" class="min-h-screen">
    <!-- iframe mode -->
    <iframe
      v-if="isHomeContentUrl"
      :src="homeContent.trim()"
      class="h-screen w-full border-0"
      allowfullscreen
    ></iframe>
    <!-- HTML mode - SECURITY: homeContent is admin-only setting, XSS risk is acceptable -->
    <div v-else v-html="homeContent"></div>
  </div>

  <!-- Default Home Page: Anti-design mode is the default public personality. -->
  <div v-else class="anti-home">
    <div class="anti-home-bg" aria-hidden="true">
      <span class="anti-home-orb anti-home-orb-red"></span>
      <span class="anti-home-orb anti-home-orb-green"></span>
      <span class="anti-home-sticker anti-home-sticker-top">NO RULES / JUST ROUTES</span>
      <span class="anti-home-sticker anti-home-sticker-bottom">CACHE IS A WEAPON</span>
    </div>

    <header class="anti-home-header">
      <nav class="anti-home-nav">
        <div class="anti-home-brand">
          <div class="anti-home-logo">
            <img :src="siteLogo || '/logo.png'" alt="Logo" />
          </div>
          <div class="anti-home-brand-text">
            <span>{{ siteName }}</span>
            <small>API GATEWAY / UNSAFE LOOK, SAFE ROUTES</small>
          </div>
        </div>

        <div class="anti-home-actions">
          <LocaleSwitcher />

          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="anti-nav-button"
            :title="t('home.viewDocs')"
          >
            <Icon name="book" size="md" />
            <span>{{ t('home.docs') }}</span>
          </a>

          <button
            class="anti-nav-button"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
            @click="toggleTheme"
          >
            <Icon v-if="isDark" name="sun" size="md" />
            <Icon v-else name="moon" size="md" />
            <span>{{ isDark ? 'LIGHT' : 'DARK' }}</span>
          </button>

          <router-link
            v-if="isAuthenticated"
            :to="dashboardPath"
            class="anti-nav-button anti-nav-button-primary"
          >
            <span class="anti-user-dot">{{ userInitial }}</span>
            <span>{{ t('home.dashboard') }}</span>
          </router-link>
          <router-link v-else to="/login" class="anti-nav-button anti-nav-button-primary">
            {{ t('home.login') }}
          </router-link>
        </div>
      </nav>
    </header>

    <main class="anti-home-main">
      <section class="anti-hero">
        <div class="anti-hero-copy">
          <div class="anti-kicker">
            <span>MODEL CHAOS</span>
            <span>TEAM CONTROL</span>
            <span>KEYS ON LEASH</span>
          </div>

          <h1 class="anti-title">
            <span>{{ siteName }}</span>
            <strong>BREAK THE API WALL</strong>
          </h1>

          <p class="anti-subtitle">
            {{ siteSubtitle }}
          </p>
          <p class="anti-manifesto">
            把 Claude、OpenAI、Gemini、内部 key 池、缓存账本和团队用量揉成一个入口。
            外表反叛一点，路由、审计和限额要稳得像铁皮柜。
          </p>

          <div class="anti-cta-row">
            <router-link
              :to="isAuthenticated ? dashboardPath : '/login'"
              class="anti-cta anti-cta-primary"
            >
              {{ isAuthenticated ? t('home.goToDashboard') : t('home.getStarted') }}
              <Icon name="arrowRight" size="md" :stroke-width="2" />
            </router-link>
            <a
              v-if="docUrl"
              :href="docUrl"
              target="_blank"
              rel="noopener noreferrer"
              class="anti-cta anti-cta-secondary"
            >
              READ THE WIRES
            </a>
          </div>
        </div>

        <div class="anti-hero-console" aria-label="Gateway routing preview">
          <div class="anti-console-label">LIVE GATEWAY NOISE</div>
          <div class="terminal-container">
            <div class="terminal-window">
              <div class="terminal-header">
                <div class="terminal-buttons">
                  <span class="btn-close"></span>
                  <span class="btn-minimize"></span>
                  <span class="btn-maximize"></span>
                </div>
                <span class="terminal-title">gateway://route-board</span>
              </div>
              <div class="terminal-body">
                <div class="code-line line-1">
                  <span class="code-prompt">$</span>
                  <span class="code-cmd">POST</span>
                  <span class="code-url">/v1/messages</span>
                  <span class="code-flag">model=claude-code</span>
                </div>
                <div class="code-line line-2">
                  <span class="code-comment">fingerprint=claude-code · key=healthy · cache=hot</span>
                </div>
                <div class="code-line line-3">
                  <span class="code-success">200 OK</span>
                  <span class="code-response">route: openai-compatible -> upstream pool #3</span>
                </div>
                <div class="code-line line-4">
                  <span class="code-prompt">$</span>
                  <span class="cursor"></span>
                </div>
              </div>
            </div>
          </div>

          <div class="anti-console-metrics">
            <div v-for="metric in gatewayMetrics" :key="metric.label">
              <span>{{ metric.label }}</span>
              <strong>{{ metric.value }}</strong>
            </div>
          </div>
        </div>
      </section>

      <section class="anti-tag-wall" aria-label="Gateway capabilities">
        <div
          v-for="tag in gatewayTags"
          :key="tag"
          class="anti-tag"
        >
          {{ tag }}
        </div>
      </section>

      <section class="anti-feature-grid" aria-label="API gateway modules">
        <article
          v-for="(item, index) in gatewayHighlights"
          :key="item.title"
          class="anti-feature-card"
          :class="`anti-feature-card-${index + 1}`"
        >
          <div class="anti-feature-index">{{ item.code }}</div>
          <h2>{{ item.title }}</h2>
          <p>{{ item.body }}</p>
          <span>{{ item.signal }}</span>
        </article>
      </section>

      <section class="anti-flow-board">
        <div class="anti-flow-title">
          <span>REQUEST PATH / 可控混乱</span>
          <strong>1 个入口，N 个上游，所有痕迹都要留下。</strong>
        </div>
        <div class="anti-flow-steps">
          <div
            v-for="(step, index) in gatewayFlow"
            :key="step.title"
            class="anti-flow-step"
          >
            <span>{{ String(index + 1).padStart(2, '0') }}</span>
            <h3>{{ step.title }}</h3>
            <p>{{ step.body }}</p>
          </div>
        </div>
      </section>

      <section class="anti-provider-zone" aria-label="Supported providers">
        <div class="anti-section-heading">
          <h2>{{ t('home.providers.title') }}</h2>
          <p>{{ t('home.providers.description') }}</p>
        </div>
        <div class="anti-provider-grid">
          <div
            v-for="provider in providerCards"
            :key="provider.name"
            class="anti-provider-card"
            :class="provider.tone"
          >
            <strong>{{ provider.mark }}</strong>
            <span>{{ provider.name }}</span>
            <em>{{ provider.status }}</em>
          </div>
        </div>
      </section>
    </main>

    <footer class="anti-home-footer">
      <p>&copy; {{ currentYear }} {{ siteName }}. {{ t('home.footer.allRightsReserved') }}</p>
      <div>
        <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer">
          {{ t('home.docs') }}
        </a>
        <a :href="githubUrl" target="_blank" rel="noopener noreferrer">GitHub</a>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore, useAppStore } from '@/stores'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import { sanitizeUrl } from '@/utils/url'

const { t } = useI18n()

const authStore = useAuthStore()
const appStore = useAppStore()

// Site settings - directly from appStore (already initialized from injected config)
const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'Sub2API')
const siteLogo = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || 'AI API Gateway Platform')
const docUrl = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.doc_url || appStore.docUrl || ''))
const homeContent = computed(() => appStore.cachedPublicSettings?.home_content || '')

// Check if homeContent is a URL (for iframe display)
const isHomeContentUrl = computed(() => {
  const content = homeContent.value.trim()
  return content.startsWith('http://') || content.startsWith('https://')
})

// Theme
const isDark = ref(document.documentElement.classList.contains('dark'))

// GitHub URL
const githubUrl = 'https://github.com/Wei-Shaw/sub2api'

// Auth state
const isAuthenticated = computed(() => authStore.isAuthenticated)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => isAdmin.value ? '/admin/dashboard' : '/dashboard')
const userInitial = computed(() => {
  const user = authStore.user
  if (!user || !user.email) return ''
  return user.email.charAt(0).toUpperCase()
})

const gatewayMetrics = [
  { label: 'MODEL MAP', value: 'OpenAI / Claude / Gemini' },
  { label: 'CACHE READ', value: 'VISIBLE' },
  { label: 'KEY HEALTH', value: 'AUTO POLL' }
]

const gatewayTags = [
  'OPENAI-COMPATIBLE',
  'CLAUDE CODE FINGERPRINT',
  'RAW KEY POOL',
  'HEALTH CHECK',
  'TEAM USAGE MATRIX',
  'CACHE LEDGER',
  'MODEL ALIAS ROUTING',
  'FAILOVER TRACE'
]

const gatewayHighlights = [
  {
    code: '01',
    title: '模型别名与协议翻译',
    body: '把不同供应商、客户端和模型命名压到一个兼容入口，减少团队脚本到处改参数。',
    signal: 'OpenAI-style in / mixed upstream out'
  },
  {
    code: '02',
    title: 'Key 池健康与轮询',
    body: '原始 key、OAuth 账号、代理通道都应该能被检测、打分、降级和自动避让。',
    signal: 'bad key should not poison the queue'
  },
  {
    code: '03',
    title: '团队用量画像',
    body: '成员、工具、模型、缓存和会话聚合不是报表装饰，是内部排障和额度分配的现场地图。',
    signal: 'who used what, from where, and why'
  },
  {
    code: '04',
    title: '缓存效率看板',
    body: '缓存读写、TTL、命中率和模型矩阵放在一起，才能看出哪里在烧 token，哪里在省钱。',
    signal: 'cache hit rate is a product signal'
  },
  {
    code: '05',
    title: '客户端指纹与兼容层',
    body: 'Claude Code、Codex CLI、Cherry Studio 等工具应有明确识别和可控伪装策略。',
    signal: 'client identity is routing metadata'
  },
  {
    code: '06',
    title: '审计与回放',
    body: '每次失败都要能看到请求路径、上游、key、模型映射和错误透传，方便复现而不是猜。',
    signal: 'no black-box failures'
  }
]

const gatewayFlow = [
  {
    title: '识别客户端',
    body: '抓住工具指纹、用户、key、模型和会话，先给请求贴标签。'
  },
  {
    title: '选择路径',
    body: '按模型映射、健康分、限额、缓存策略和组策略选择上游。'
  },
  {
    title: '失败避让',
    body: '错误分类、降级、重试和熔断分开处理，避免一个坏 key 拖垮队列。'
  },
  {
    title: '写入画像',
    body: '把 token、缓存、耗时、会话和工具分布写回看板，下一次调度更聪明。'
  }
]

const providerCards = computed(() => [
  {
    mark: 'C',
    name: t('home.providers.claude'),
    status: t('home.providers.supported'),
    tone: 'anti-provider-orange'
  },
  {
    mark: 'O',
    name: 'OpenAI',
    status: t('home.providers.supported'),
    tone: 'anti-provider-green'
  },
  {
    mark: 'G',
    name: t('home.providers.gemini'),
    status: t('home.providers.supported'),
    tone: 'anti-provider-blue'
  },
  {
    mark: 'A',
    name: t('home.providers.antigravity'),
    status: t('home.providers.supported'),
    tone: 'anti-provider-red'
  },
  {
    mark: '+',
    name: t('home.providers.more'),
    status: t('home.providers.soon'),
    tone: 'anti-provider-muted'
  }
])

// Current year for footer
const currentYear = computed(() => new Date().getFullYear())

// Toggle theme
function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}

// Initialize theme
function initTheme() {
  const savedTheme = localStorage.getItem('theme')
  if (
    savedTheme === 'dark' ||
    (!savedTheme && window.matchMedia('(prefers-color-scheme: dark)').matches)
  ) {
    isDark.value = true
    document.documentElement.classList.add('dark')
  }
}

onMounted(() => {
  initTheme()

  // Check auth state
  authStore.checkAuth()

  // Ensure public settings are loaded (will use cache if already loaded from injected config)
  if (!appStore.publicSettingsLoaded) {
    appStore.fetchPublicSettings()
  }
})
</script>

<style scoped>
.anti-home {
  --anti-ink: #050505;
  --anti-paper: #ffffff;
  --anti-yellow: #ffeb3b;
  --anti-red: #ff0000;
  --anti-blue: #0000ff;
  --anti-green: #00ff00;
  --anti-cyan: #00e5ff;
  position: relative;
  min-height: 100vh;
  overflow: hidden;
  color: var(--anti-ink);
  background:
    radial-gradient(circle at 14% 12%, rgb(255 0 0 / 0.28) 0 8rem, transparent 8.2rem),
    radial-gradient(circle at 88% 18%, rgb(0 255 0 / 0.30) 0 9rem, transparent 9.2rem),
    repeating-linear-gradient(135deg, rgb(5 5 5 / 0.12) 0 10px, transparent 10px 22px),
    linear-gradient(135deg, #ffeb3b 0%, #fff 44%, #00e5ff 100%);
  font-family: Impact, "Arial Black", "Comic Sans MS", ui-sans-serif, system-ui, sans-serif;
  letter-spacing: -0.02em;
}

:global(.dark) .anti-home {
  background:
    radial-gradient(circle at 12% 14%, rgb(255 0 0 / 0.38) 0 8rem, transparent 8.2rem),
    radial-gradient(circle at 84% 16%, rgb(0 255 0 / 0.32) 0 9rem, transparent 9.2rem),
    repeating-linear-gradient(135deg, rgb(255 255 255 / 0.10) 0 10px, transparent 10px 22px),
    linear-gradient(135deg, #080008 0%, #ff00ff 42%, #ffeb3b 100%);
}

.anti-home-bg {
  pointer-events: none;
  position: absolute;
  inset: 0;
  overflow: hidden;
}

.anti-home-bg::before {
  content: "";
  position: absolute;
  inset: 2rem;
  border: 7px solid var(--anti-ink);
  box-shadow: 14px 14px 0 var(--anti-blue);
  opacity: 0.14;
  transform: rotate(-1.6deg);
}

.anti-home-bg::after {
  content: "";
  position: absolute;
  inset: 0;
  background:
    repeating-linear-gradient(90deg, rgb(5 5 5 / 0.13) 0 12px, transparent 12px 28px),
    repeating-linear-gradient(0deg, rgb(0 0 255 / 0.11) 0 2px, transparent 2px 52px);
  mix-blend-mode: multiply;
}

.anti-home-orb,
.anti-home-sticker {
  position: absolute;
  z-index: 1;
}

.anti-home-orb {
  border: 8px solid var(--anti-ink);
  box-shadow: 13px 13px 0 var(--anti-ink);
}

.anti-home-orb-red {
  right: -4rem;
  top: 14rem;
  height: 12rem;
  width: 12rem;
  background: var(--anti-red);
  transform: rotate(18deg);
}

.anti-home-orb-green {
  bottom: 10rem;
  left: -3rem;
  height: 9rem;
  width: 18rem;
  background: var(--anti-green);
  transform: rotate(-12deg);
}

.anti-home-sticker {
  border: 6px solid var(--anti-ink);
  background: var(--anti-green);
  box-shadow: 9px 9px 0 var(--anti-blue);
  padding: 0.4rem 0.7rem;
  color: var(--anti-ink);
  font-size: clamp(1rem, 2vw, 1.8rem);
  line-height: 0.92;
  transform: rotate(5deg);
}

.anti-home-sticker-top {
  right: 7vw;
  top: 6.2rem;
}

.anti-home-sticker-bottom {
  bottom: 5rem;
  left: 10vw;
  background: var(--anti-red);
  color: var(--anti-paper);
  transform: rotate(-4deg);
}

.anti-home-header,
.anti-home-main,
.anti-home-footer {
  position: relative;
  z-index: 2;
}

.anti-home-header {
  padding: 1.4rem clamp(1rem, 3vw, 2rem);
}

.anti-home-nav {
  display: flex;
  max-width: 82rem;
  margin: 0 auto;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  border: 6px solid var(--anti-ink);
  background: var(--anti-paper);
  box-shadow: 10px 10px 0 var(--anti-ink);
  padding: 0.75rem;
  transform: rotate(-0.7deg);
}

.anti-home-brand {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 0.85rem;
}

.anti-home-logo {
  display: grid;
  height: 3.25rem;
  width: 3.25rem;
  flex: 0 0 auto;
  place-items: center;
  border: 5px solid var(--anti-ink);
  background: var(--anti-yellow);
  box-shadow: 5px 5px 0 var(--anti-blue);
  transform: rotate(-8deg);
}

.anti-home-logo img {
  height: 2.4rem;
  width: 2.4rem;
  object-fit: contain;
  filter: contrast(1.15) saturate(1.25);
}

.anti-home-brand-text {
  display: grid;
  min-width: 0;
}

.anti-home-brand-text span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: clamp(1.2rem, 2.4vw, 2rem);
  line-height: 0.92;
  text-transform: uppercase;
}

.anti-home-brand-text small {
  margin-top: 0.2rem;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 0.68rem;
  letter-spacing: -0.04em;
}

.anti-home-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: flex-end;
  gap: 0.55rem;
}

.anti-home-actions :deep(button),
.anti-home-actions :deep(.locale-switcher),
.anti-nav-button,
.anti-cta {
  border: 4px solid var(--anti-ink) !important;
  border-radius: 0 !important;
  background: var(--anti-yellow) !important;
  box-shadow: 5px 5px 0 var(--anti-ink);
  color: var(--anti-ink) !important;
  font-weight: 950;
  text-transform: uppercase;
}

.anti-nav-button {
  display: inline-flex;
  min-height: 2.5rem;
  align-items: center;
  gap: 0.45rem;
  padding: 0.42rem 0.7rem;
  font-size: 0.78rem;
  text-decoration: none;
}

.anti-nav-button-primary {
  background: var(--anti-red) !important;
  color: var(--anti-paper) !important;
  transform: rotate(2deg);
}

.anti-user-dot {
  display: inline-grid;
  height: 1.35rem;
  width: 1.35rem;
  place-items: center;
  border: 2px solid var(--anti-paper);
  background: var(--anti-blue);
  color: var(--anti-paper);
  font-size: 0.7rem;
}

.anti-home-main {
  max-width: 82rem;
  margin: 0 auto;
  padding: clamp(2rem, 5vw, 4.8rem) clamp(1rem, 3vw, 2rem) 4rem;
}

.anti-hero {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: 2rem;
  align-items: center;
}

.anti-hero-copy {
  position: relative;
  border: 7px solid var(--anti-ink);
  background: var(--anti-paper);
  box-shadow: 13px 13px 0 var(--anti-red);
  padding: clamp(1.2rem, 3vw, 2.2rem);
  transform: rotate(-1deg);
}

.anti-kicker {
  display: flex;
  flex-wrap: wrap;
  gap: 0.45rem;
  margin-bottom: 1.1rem;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
}

.anti-kicker span,
.anti-tag,
.anti-flow-title span {
  border: 3px solid var(--anti-ink);
  background: var(--anti-green);
  box-shadow: 4px 4px 0 var(--anti-ink);
  padding: 0.2rem 0.45rem;
  font-size: 0.72rem;
  font-weight: 900;
  letter-spacing: -0.04em;
}

.anti-kicker span:nth-child(2) {
  background: var(--anti-cyan);
  transform: rotate(2deg);
}

.anti-kicker span:nth-child(3) {
  background: var(--anti-yellow);
  transform: rotate(-2deg);
}

.anti-title {
  display: grid;
  gap: 0.45rem;
  margin: 0;
  color: var(--anti-ink);
  font-size: clamp(3.4rem, 9vw, 8.3rem);
  letter-spacing: -0.085em;
  line-height: 0.78;
  text-transform: uppercase;
}

.anti-title span {
  max-width: 100%;
  overflow-wrap: anywhere;
}

.anti-title strong {
  display: inline-block;
  width: fit-content;
  max-width: 100%;
  border: 6px solid var(--anti-ink);
  background: var(--anti-blue);
  box-shadow: 9px 9px 0 var(--anti-ink);
  color: var(--anti-paper);
  padding: 0.2rem 0.45rem 0.28rem;
  transform: rotate(1.5deg);
}

.anti-subtitle {
  margin: 1.6rem 0 0;
  max-width: 46rem;
  font-size: clamp(1.25rem, 2.5vw, 2.1rem);
  line-height: 1.05;
  text-transform: uppercase;
}

.anti-manifesto {
  max-width: 44rem;
  margin: 1rem 0 0;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 0.98rem;
  font-weight: 800;
  letter-spacing: -0.05em;
  line-height: 1.55;
}

.anti-cta-row {
  display: flex;
  flex-wrap: wrap;
  gap: 0.85rem;
  margin-top: 1.6rem;
}

.anti-cta {
  display: inline-flex;
  min-height: 3.25rem;
  align-items: center;
  gap: 0.6rem;
  padding: 0.75rem 1rem;
  color: var(--anti-ink);
  text-decoration: none;
}

.anti-cta-primary {
  background: var(--anti-red) !important;
  color: var(--anti-paper) !important;
  box-shadow: 7px 7px 0 var(--anti-blue);
  transform: rotate(-2deg);
}

.anti-cta-secondary {
  background: var(--anti-green) !important;
  transform: rotate(2deg);
}

.anti-hero-console {
  position: relative;
  border: 7px solid var(--anti-ink);
  background: var(--anti-yellow);
  box-shadow: 13px 13px 0 var(--anti-blue);
  padding: clamp(1rem, 2.2vw, 1.5rem);
  transform: rotate(1.4deg);
}

.anti-console-label {
  position: absolute;
  right: 1rem;
  top: -1.1rem;
  border: 5px solid var(--anti-ink);
  background: var(--anti-red);
  box-shadow: 6px 6px 0 var(--anti-ink);
  color: var(--anti-paper);
  padding: 0.18rem 0.5rem;
  font-size: 1.1rem;
  transform: rotate(5deg);
}

.terminal-container {
  position: relative;
}

.terminal-window {
  overflow: hidden;
  width: min(100%, 35rem);
  border: 6px solid var(--anti-ink);
  background: #050505;
  box-shadow: 9px 9px 0 var(--anti-ink);
  color: var(--anti-paper);
}

.terminal-header {
  display: flex;
  align-items: center;
  border-bottom: 5px solid var(--anti-ink);
  background: var(--anti-paper);
  padding: 0.7rem 0.85rem;
}

.terminal-buttons {
  display: flex;
  gap: 0.45rem;
}

.terminal-buttons span {
  width: 0.85rem;
  height: 0.85rem;
  border: 3px solid var(--anti-ink);
}

.btn-close {
  background: var(--anti-red);
}

.btn-minimize {
  background: var(--anti-yellow);
}

.btn-maximize {
  background: var(--anti-green);
}

.terminal-title {
  flex: 1;
  margin-right: 3rem;
  color: var(--anti-ink);
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 0.74rem;
  font-weight: 900;
  text-align: center;
}

.terminal-body {
  display: grid;
  gap: 0.6rem;
  padding: 1rem;
  font-family: ui-monospace, "Fira Code", monospace;
  font-size: clamp(0.78rem, 1.8vw, 0.92rem);
  line-height: 1.5;
}

.code-line {
  display: flex;
  flex-wrap: wrap;
  gap: 0.45rem;
  opacity: 0;
  animation: line-appear 0.45s steps(2, end) forwards;
}

.line-1 {
  animation-delay: 0.25s;
}

.line-2 {
  animation-delay: 0.8s;
}

.line-3 {
  animation-delay: 1.35s;
}

.line-4 {
  animation-delay: 1.9s;
}

.code-prompt,
.code-success {
  color: var(--anti-green);
}

.code-cmd {
  color: var(--anti-cyan);
}

.code-flag {
  color: #ff7aff;
}

.code-url,
.code-response {
  color: var(--anti-yellow);
}

.code-comment {
  color: #cbd5e1;
}

.code-success {
  border: 2px solid var(--anti-green);
  padding: 0 0.3rem;
  font-weight: 900;
}

.cursor {
  display: inline-block;
  width: 0.65rem;
  height: 1rem;
  background: var(--anti-green);
  animation: blink 0.8s step-end infinite;
}

.anti-console-metrics {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0.65rem;
  margin-top: 1rem;
}

.anti-console-metrics div {
  border: 4px solid var(--anti-ink);
  background: var(--anti-paper);
  box-shadow: 5px 5px 0 var(--anti-ink);
  padding: 0.6rem;
}

.anti-console-metrics span {
  display: block;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 0.62rem;
  font-weight: 900;
}

.anti-console-metrics strong {
  display: block;
  margin-top: 0.2rem;
  font-size: 0.9rem;
  line-height: 0.95;
  text-transform: uppercase;
}

.anti-tag-wall {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
  margin: 2rem 0;
  transform: rotate(-0.6deg);
}

.anti-tag:nth-child(2n) {
  background: var(--anti-red);
  color: var(--anti-paper);
  transform: rotate(2deg);
}

.anti-tag:nth-child(3n) {
  background: var(--anti-blue);
  color: var(--anti-paper);
  transform: rotate(-2deg);
}

.anti-feature-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: 1.1rem;
  margin-top: 2rem;
}

.anti-feature-card,
.anti-flow-board,
.anti-provider-zone {
  border: 6px solid var(--anti-ink);
  background: var(--anti-paper);
  box-shadow: 10px 10px 0 var(--anti-ink);
}

.anti-feature-card {
  position: relative;
  min-height: 16rem;
  padding: 1rem;
  transition:
    transform 150ms steps(2, end),
    box-shadow 150ms steps(2, end),
    background-color 150ms steps(2, end);
}

.anti-feature-card-1,
.anti-feature-card-5 {
  background: var(--anti-yellow);
  transform: rotate(-1.2deg);
}

.anti-feature-card-2,
.anti-feature-card-6 {
  background: var(--anti-cyan);
  transform: rotate(1.1deg) translateY(0.35rem);
}

.anti-feature-card-3 {
  background: var(--anti-green);
  transform: rotate(-0.6deg);
}

.anti-feature-card-4 {
  background: var(--anti-red);
  color: var(--anti-paper);
  transform: rotate(1.6deg);
}

.anti-feature-index {
  position: absolute;
  right: 0.75rem;
  top: 0.65rem;
  border: 4px solid var(--anti-ink);
  background: var(--anti-paper);
  box-shadow: 5px 5px 0 var(--anti-ink);
  color: var(--anti-ink);
  padding: 0.15rem 0.45rem;
  font-size: 1.6rem;
  line-height: 0.9;
  transform: rotate(6deg);
}

.anti-feature-card h2 {
  max-width: 76%;
  margin: 0 0 1rem;
  font-size: clamp(1.6rem, 4vw, 2.65rem);
  line-height: 0.9;
  text-transform: uppercase;
}

.anti-feature-card p {
  margin: 0;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 0.92rem;
  font-weight: 900;
  letter-spacing: -0.05em;
  line-height: 1.55;
}

.anti-feature-card span {
  display: inline-block;
  margin-top: 1rem;
  border: 3px solid currentColor;
  background: rgb(255 255 255 / 0.7);
  padding: 0.24rem 0.42rem;
  font-size: 0.75rem;
  text-transform: uppercase;
}

.anti-flow-board {
  margin-top: 2.4rem;
  padding: clamp(1rem, 3vw, 1.6rem);
  transform: rotate(0.4deg);
}

.anti-flow-title {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  margin-bottom: 1rem;
}

.anti-flow-title strong {
  max-width: 34rem;
  font-size: clamp(1.2rem, 2.4vw, 2rem);
  line-height: 0.95;
  text-align: right;
  text-transform: uppercase;
}

.anti-flow-steps {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: 0.9rem;
}

.anti-flow-step {
  border: 5px solid var(--anti-ink);
  background: var(--anti-yellow);
  box-shadow: 6px 6px 0 var(--anti-ink);
  padding: 0.85rem;
}

.anti-flow-step:nth-child(even) {
  background: var(--anti-green);
  transform: rotate(1deg);
}

.anti-flow-step span {
  display: inline-block;
  margin-bottom: 0.5rem;
  border: 3px solid var(--anti-ink);
  background: var(--anti-ink);
  color: var(--anti-yellow);
  padding: 0.1rem 0.35rem;
  line-height: 1;
}

.anti-flow-step h3 {
  margin: 0 0 0.45rem;
  font-size: 1.35rem;
  line-height: 0.95;
}

.anti-flow-step p {
  margin: 0;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 0.86rem;
  font-weight: 850;
  letter-spacing: -0.045em;
  line-height: 1.45;
}

.anti-provider-zone {
  margin-top: 2.4rem;
  padding: clamp(1rem, 3vw, 1.6rem);
  transform: rotate(-0.45deg);
}

.anti-section-heading {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  gap: 1rem;
  margin-bottom: 1rem;
}

.anti-section-heading h2 {
  margin: 0;
  font-size: clamp(2rem, 5vw, 4rem);
  line-height: 0.82;
  text-transform: uppercase;
}

.anti-section-heading p {
  max-width: 30rem;
  margin: 0;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-weight: 850;
  letter-spacing: -0.045em;
}

.anti-provider-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 0.8rem;
}

.anti-provider-card {
  display: flex;
  min-height: 4.8rem;
  min-width: 12rem;
  align-items: center;
  gap: 0.6rem;
  border: 5px solid var(--anti-ink);
  background: var(--anti-paper);
  box-shadow: 6px 6px 0 var(--anti-ink);
  padding: 0.55rem;
  transform: rotate(-1deg);
}

.anti-provider-card strong {
  display: grid;
  height: 2.9rem;
  width: 2.9rem;
  flex: 0 0 auto;
  place-items: center;
  border: 4px solid var(--anti-ink);
  background: var(--anti-yellow);
  color: var(--anti-ink);
  font-size: 1.6rem;
  line-height: 1;
}

.anti-provider-card span {
  font-size: 1.15rem;
  line-height: 0.9;
}

.anti-provider-card em {
  margin-left: auto;
  border: 3px solid var(--anti-ink);
  background: var(--anti-green);
  padding: 0.1rem 0.35rem;
  font-size: 0.7rem;
  font-style: normal;
}

.anti-provider-orange strong {
  background: #ff7a00;
}

.anti-provider-green strong {
  background: var(--anti-green);
}

.anti-provider-blue strong {
  background: var(--anti-blue);
  color: var(--anti-paper);
}

.anti-provider-red strong {
  background: var(--anti-red);
  color: var(--anti-paper);
}

.anti-provider-muted {
  opacity: 0.72;
  transform: rotate(2deg);
}

.anti-home-footer {
  display: flex;
  max-width: 82rem;
  margin: 0 auto;
  padding: 1.5rem clamp(1rem, 3vw, 2rem) 2rem;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 0.82rem;
  font-weight: 900;
  letter-spacing: -0.04em;
}

.anti-home-footer p {
  margin: 0;
}

.anti-home-footer div {
  display: flex;
  gap: 0.8rem;
}

.anti-home-footer a {
  border: 3px solid var(--anti-ink);
  background: var(--anti-paper);
  box-shadow: 4px 4px 0 var(--anti-ink);
  color: var(--anti-ink);
  padding: 0.25rem 0.45rem;
  text-decoration: none;
  text-transform: uppercase;
}

@keyframes line-appear {
  from {
    opacity: 0;
    transform: translate(-0.35rem, 0.2rem);
  }
  to {
    opacity: 1;
    transform: translate(0, 0);
  }
}

@keyframes blink {
  0%,
  50% {
    opacity: 1;
  }
  51%,
  100% {
    opacity: 0;
  }
}

@media (hover: hover) {
  .anti-nav-button:hover,
  .anti-cta:hover,
  .anti-feature-card:hover,
  .anti-provider-card:hover,
  .anti-home-footer a:hover {
    background: var(--anti-green) !important;
    box-shadow: 12px 12px 0 var(--anti-red);
    transform: rotate(-2deg) translate(-2px, -2px);
  }

  .terminal-window:hover {
    transform: rotate(-1deg) translate(-2px, -2px);
  }
}

@media (min-width: 768px) {
  .anti-feature-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .anti-flow-steps {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (min-width: 1024px) {
  .anti-hero {
    grid-template-columns: minmax(0, 1.05fr) minmax(22rem, 0.95fr);
  }

  .anti-feature-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

@media (max-width: 760px) {
  .anti-home-header {
    padding: 1rem;
  }

  .anti-home-nav,
  .anti-home-footer {
    align-items: stretch;
    flex-direction: column;
  }

  .anti-home-actions {
    justify-content: flex-start;
  }

  .anti-title {
    font-size: clamp(3rem, 18vw, 5.4rem);
  }

  .anti-console-metrics {
    grid-template-columns: minmax(0, 1fr);
  }

  .anti-home-sticker {
    display: none;
  }

  .anti-flow-title strong {
    text-align: left;
  }
}

@media (prefers-reduced-motion: reduce) {
  .code-line,
  .cursor {
    animation-duration: 1ms;
    animation-iteration-count: 1;
  }

  .anti-nav-button,
  .anti-cta,
  .anti-feature-card,
  .anti-provider-card,
  .terminal-window {
    transition-duration: 1ms;
  }
}
</style>
