<template>
  <div class="account-map-page space-y-6 pb-12">
    <section class="account-map-toolbar">
      <div class="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
        <div>
          <h1 class="flex items-center gap-2 text-xl font-black text-gray-900 dark:text-white">
            <svg class="h-6 w-6 text-blue-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M4 7a2 2 0 012-2h3a2 2 0 012 2v3a2 2 0 01-2 2H6a2 2 0 01-2-2V7zm9 0a2 2 0 012-2h3a2 2 0 012 2v3a2 2 0 01-2 2h-3a2 2 0 01-2-2V7zM4 16a2 2 0 012-2h3a2 2 0 012 2v1a2 2 0 01-2 2H6a2 2 0 01-2-2v-1zm9 0a2 2 0 012-2h3a2 2 0 012 2v1a2 2 0 01-2 2h-3a2 2 0 01-2-2v-1z"
              />
            </svg>
            账号池地图
          </h1>
          <div class="mt-1 flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
            <span class="flex items-center gap-1.5">
              <span class="relative inline-flex h-2 w-2 rounded-full" :class="loading ? 'bg-gray-400' : 'bg-green-500'"></span>
              {{ loading ? '加载中' : '就绪' }}
            </span>
            <span>·</span>
            <span>按平台、状态和账号池查看调度健康度</span>
            <template v-if="generatedAt">
              <span>·</span>
              <span>更新 {{ formatDate(generatedAt) }}</span>
            </template>
          </div>
        </div>
        <div class="flex flex-wrap items-center gap-3">
          <button
            type="button"
            class="account-map-secondary-button"
            @click="refresh"
          >
            {{ loading ? '刷新中...' : '刷新地图' }}
          </button>
          <button
            type="button"
            class="account-map-primary-button"
            @click="goAccounts"
          >
            账号管理
          </button>
        </div>
      </div>
    </section>

    <section class="account-map-summary-grid grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
      <div
        v-for="item in summaryCards"
        :key="item.key"
        class="account-map-stat-card"
      >
        <div class="flex items-center justify-between gap-3">
          <p class="text-[10px] font-bold uppercase tracking-wider text-gray-400">{{ item.label }}</p>
          <span class="h-2.5 w-2.5 rounded-full" :class="item.dotClass"></span>
        </div>
        <p class="mt-2 text-2xl font-black text-gray-900 dark:text-white">{{ item.value }}</p>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ item.detail }}</p>
      </div>
    </section>

    <section class="account-map-workspace grid gap-6 xl:grid-cols-[minmax(0,1fr)_380px]">
      <div class="account-map-main-column">
        <div class="account-map-filter-card">
          <div class="grid gap-3 md:grid-cols-[1.2fr_0.8fr_0.8fr_auto]">
            <label class="block">
              <span class="mb-1 block text-xs font-medium text-gray-500 dark:text-gray-400">搜索账号</span>
              <input
                v-model.trim="filters.search"
                type="search"
                class="w-full rounded-xl border border-gray-200 bg-white px-3 py-2 text-sm text-gray-900 outline-none transition focus:border-primary-400 focus:ring-4 focus:ring-primary-500/10 dark:border-dark-700 dark:bg-dark-800 dark:text-white"
                placeholder="名称、平台、分组、错误信息"
              />
            </label>
            <label class="block">
              <span class="mb-1 block text-xs font-medium text-gray-500 dark:text-gray-400">平台</span>
              <select
                v-model="filters.platform"
                class="w-full rounded-xl border border-gray-200 bg-white px-3 py-2 text-sm text-gray-900 outline-none transition focus:border-primary-400 focus:ring-4 focus:ring-primary-500/10 dark:border-dark-700 dark:bg-dark-800 dark:text-white"
              >
                <option value="all">全部平台</option>
                <option v-for="platform in platformOptions" :key="platform" :value="platform">
                  {{ platformLabel(platform) }}
                </option>
              </select>
            </label>
            <label class="block">
              <span class="mb-1 block text-xs font-medium text-gray-500 dark:text-gray-400">状态</span>
              <select
                v-model="filters.status"
                class="w-full rounded-xl border border-gray-200 bg-white px-3 py-2 text-sm text-gray-900 outline-none transition focus:border-primary-400 focus:ring-4 focus:ring-primary-500/10 dark:border-dark-700 dark:bg-dark-800 dark:text-white"
              >
                <option value="all">全部状态</option>
                <option value="healthy">健康</option>
                <option value="degraded">降级/冷却</option>
                <option value="rate_limited">限流</option>
                <option value="error">错误</option>
                <option value="disabled">停用</option>
              </select>
            </label>
            <div class="flex items-end">
              <div class="inline-flex rounded-xl border border-gray-200 bg-gray-50 p-1 dark:border-dark-700 dark:bg-dark-800">
                <button
                  v-for="mode in viewModes"
                  :key="mode.value"
                  type="button"
                  class="rounded-lg px-3 py-1.5 text-xs font-semibold transition"
                  :class="filters.view === mode.value ? 'bg-white text-primary-700 shadow-sm dark:bg-dark-700 dark:text-primary-300' : 'text-gray-500 hover:text-gray-900 dark:text-gray-400 dark:hover:text-white'"
                  @click="filters.view = mode.value"
                >
                  {{ mode.label }}
                </button>
              </div>
            </div>
          </div>
          <div
            v-if="errorMessage"
            class="mt-3 flex flex-col gap-2 rounded-xl border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-800 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-200 sm:flex-row sm:items-center sm:justify-between"
          >
            <span>{{ errorMessage }}</span>
            <button type="button" class="text-xs font-semibold underline underline-offset-4" @click="refresh">
              重试
            </button>
          </div>
          <div
            v-if="sourceInfo?.truncated"
            class="mt-3 rounded-xl border border-blue-200 bg-blue-50 px-3 py-2 text-xs text-blue-700 dark:border-blue-500/30 dark:bg-blue-500/10 dark:text-blue-200"
          >
            当前仅展示前 {{ sourceInfo.returned }} / {{ sourceInfo.total }} 个账号；可以通过搜索或平台筛选缩小范围。
          </div>
        </div>

        <div v-if="loading && !accounts.length" class="grid gap-3">
          <div v-for="idx in 3" :key="idx" class="h-32 animate-pulse rounded-2xl bg-gray-100 dark:bg-dark-800"></div>
        </div>

        <div v-else-if="!visiblePools.length" class="account-map-empty-state">
          <p class="text-base font-semibold text-gray-900 dark:text-white">没有匹配的账号池</p>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">调整筛选条件，或者先在账号管理中导入/创建上游账号。</p>
        </div>

        <div v-else class="space-y-4">
          <article
            v-for="pool in visiblePools"
            :key="pool.key"
            class="account-pool-lane rounded-2xl border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-900"
          >
            <div class="flex flex-col gap-3 border-b border-gray-100 pb-4 dark:border-dark-800 md:flex-row md:items-center md:justify-between">
              <div class="min-w-0">
                <div class="flex flex-wrap items-center gap-2">
                  <span class="rounded-full bg-gray-100 px-2.5 py-1 text-xs font-semibold text-gray-700 dark:bg-dark-800 dark:text-gray-300">
                    {{ platformLabel(pool.platform) }}
                  </span>
                  <h2 class="truncate text-lg font-bold text-gray-950 dark:text-white">
                    {{ accountTypeLabel(pool.type) }}
                  </h2>
                </div>
                <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                  {{ pool.accounts.length }} 个账号 · 健康 {{ poolHealthyCount(pool) }} · 需关注 {{ poolAttentionCount(pool) }}
                </p>
              </div>
              <div class="grid grid-cols-3 gap-2 text-right text-xs">
                <div class="rounded-xl bg-gray-50 px-3 py-2 dark:bg-dark-800">
                  <p class="text-gray-400">RPM</p>
                  <p class="font-semibold text-gray-900 dark:text-white">{{ poolRPM(pool) }}</p>
                </div>
                <div class="rounded-xl bg-gray-50 px-3 py-2 dark:bg-dark-800">
                  <p class="text-gray-400">并发</p>
                  <p class="font-semibold text-gray-900 dark:text-white">{{ poolConcurrency(pool) }}</p>
                </div>
                <div class="rounded-xl bg-gray-50 px-3 py-2 dark:bg-dark-800">
                  <p class="text-gray-400">指纹</p>
                  <p class="font-semibold text-gray-900 dark:text-white">{{ poolFingerprintCount(pool) }}</p>
                </div>
              </div>
            </div>

            <div class="mt-4 grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3 2xl:grid-cols-4">
              <button
                v-for="account in pool.accounts"
                :key="account.id"
                type="button"
                class="account-node group text-left"
                :class="[nodeClass(account), selectedAccount?.id === account.id ? 'account-node-selected' : '']"
                @click="selectAccount(account)"
              >
                <div class="flex items-start justify-between gap-2">
                  <div class="min-w-0">
                    <p class="truncate text-sm font-bold">{{ account.name }}</p>
                    <p class="mt-0.5 text-[11px] opacity-75">
                      {{ statusLabel(account) }} · {{ account.type }}
                    </p>
                  </div>
                  <span class="mt-1 h-2.5 w-2.5 shrink-0 rounded-full bg-current opacity-70"></span>
                </div>
                <div class="mt-3 flex items-center justify-between gap-2 text-[11px] opacity-80">
                  <span>RPM {{ account.current_rpm ?? 0 }}</span>
                  <span>并发 {{ account.current_concurrency ?? 0 }}</span>
                  <span v-if="account.enable_tls_fingerprint">TLS</span>
                  <span v-else>no TLS</span>
                </div>
              </button>
            </div>
          </article>
        </div>
      </div>

      <aside class="account-map-detail-rail">
        <section ref="inspectorPanel" class="account-map-inspector" aria-live="polite">
          <template v-if="selectedAccount">
            <div class="inspector-heading">
              <div class="min-w-0">
                <p class="inspector-kicker">账号详情</p>
                <h2 class="inspector-title">{{ selectedAccount.name }}</h2>
                <p class="inspector-subtitle">
                  {{ platformLabel(selectedAccount.platform) }} · {{ accountTypeLabel(selectedAccount.type) }}
                </p>
              </div>
              <button type="button" class="inspector-close" aria-label="取消选择账号" @click="clearSelection">
                ×
              </button>
            </div>

            <div class="inspector-status-card" :class="`inspector-status-${statusKind(selectedAccount)}`">
              <div>
                <span class="inspector-status-label">当前状态</span>
                <strong>{{ statusLabel(selectedAccount) }}</strong>
                <p>{{ selectedAccount.status_reason || selectedAccount.temp_unschedulable_reason || selectedAccount.error_message || '该账号当前可按调度策略参与路由。' }}</p>
              </div>
              <span class="rounded-full px-2.5 py-1 text-xs font-semibold" :class="statusBadgeClass(selectedAccount)">
                {{ selectedAccount.schedulable ? '可调度' : '不可调度' }}
              </span>
            </div>

            <div class="inspector-metric-grid">
              <MetricTile label="可调度" :value="selectedAccount.schedulable ? '是' : '否'" />
              <MetricTile label="当前并发" :value="String(selectedAccount.current_concurrency ?? 0)" />
              <MetricTile label="当前 RPM" :value="String(selectedAccount.current_rpm ?? 0)" />
              <MetricTile label="活跃会话" :value="String(selectedAccount.active_sessions ?? 0)" />
            </div>

            <div class="inspector-section">
              <div class="inspector-section-title">
                <h3>池与路由</h3>
                <span>{{ selectedPoolLabel }}</span>
              </div>
              <InspectorRow label="所在泳道" :value="selectedPoolLabel" />
              <InspectorRow label="分组" :value="groupNames(selectedAccount)" />
              <InspectorRow label="代理" :value="selectedAccount.proxy?.name || '未绑定'" />
              <InspectorRow label="同池账号" :value="`${selectedPoolPeerCount} 个`" />
            </div>

            <div class="inspector-section">
              <div class="inspector-section-title">
                <h3>客户端策略</h3>
                <span>HTTP / TLS</span>
              </div>
              <InspectorRow label="TLS 指纹" :value="selectedAccount.enable_tls_fingerprint ? `已启用 #${selectedAccount.tls_fingerprint_profile_id || '-'}` : '未启用'" />
              <InspectorRow label="缓存 TTL" :value="selectedAccount.cache_ttl_override_enabled ? `强制 ${selectedAccount.cache_ttl_override_target || '-'}` : '按上游返回'" />
              <InspectorRow label="最近使用" :value="formatDate(selectedAccount.last_used_at)" />
            </div>

            <div v-if="selectedAccount.error_message || selectedAccount.temp_unschedulable_reason" class="inspector-alert">
              <p class="font-semibold">最近问题</p>
              <p class="mt-1 line-clamp-4 text-xs">{{ selectedAccount.temp_unschedulable_reason || selectedAccount.error_message }}</p>
            </div>

            <div class="inspector-actions">
              <button type="button" class="inspector-action-primary" @click="goAccountDetail(selectedAccount)">
                打开账号管理
              </button>
              <button type="button" class="inspector-action" @click="goFingerprints">
                查看指纹策略
              </button>
              <button type="button" class="inspector-action" @click="goUsage(selectedAccount)">
                查看相关使用记录
              </button>
            </div>
          </template>

          <template v-else>
            <div class="inspector-empty-header">
              <p class="inspector-kicker">详情栏</p>
              <h2 class="inspector-title">选择账号查看调度状态</h2>
              <p class="inspector-subtitle">
                未选择时展示当前筛选范围的账号池概览和优先关注项。
              </p>
            </div>

            <div class="inspector-section">
              <div class="inspector-section-title">
                <h3>地图概览</h3>
                <span>{{ visiblePools.length }} 个泳道</span>
              </div>
              <div class="inspector-overview-grid">
                <MetricTile label="账号总数" :value="String(summary.total)" />
                <MetricTile label="健康" :value="String(summary.healthy)" />
                <MetricTile label="需关注" :value="String(summary.attention)" />
                <MetricTile label="TLS 覆盖" :value="`${summary.tls_enabled}/${summary.total || 0}`" />
              </div>
            </div>

            <div class="inspector-section">
              <div class="inspector-section-title">
                <h3>优先关注</h3>
                <span>{{ attentionAccounts.length ? `${attentionAccounts.length} 个` : '无异常' }}</span>
              </div>
              <button
                v-for="account in attentionAccounts"
                :key="account.id"
                type="button"
                class="inspector-attention-item"
                @click="selectAccount(account)"
              >
                <div class="flex items-center justify-between gap-2">
                  <p class="truncate text-sm font-semibold text-gray-900 dark:text-white">{{ account.name }}</p>
                  <span class="rounded-full px-2 py-0.5 text-[11px] font-semibold" :class="statusBadgeClass(account)">
                    {{ statusLabel(account) }}
                  </span>
                </div>
                <p class="mt-1 truncate text-xs text-gray-500 dark:text-gray-400">
                  {{ account.status_reason || account.error_message || account.temp_unschedulable_reason || '等待恢复或需要人工复核' }}
                </p>
              </button>
              <div v-if="!attentionAccounts.length" class="inspector-good-state">
                当前筛选范围内没有明显异常账号。
              </div>
            </div>
          </template>
        </section>
      </aside>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, nextTick, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { adminAPI } from '@/api/admin'
import type { Account } from '@/types'
import type {
  AccountPoolMapAccount,
  AccountPoolMapPool,
  AccountPoolMapResponse,
  AccountPoolMapStatusKind,
  AccountPoolMapSummary
} from '@/api/admin/accounts'

type ViewMode = 'status' | 'traffic' | 'error'
type AccountStatusKind = AccountPoolMapStatusKind
type AccountPool = AccountPoolMapPool

const MetricTile = defineComponent({
  props: {
    label: { type: String, required: true },
    value: { type: String, required: true }
  },
  setup(props) {
    return () => h('div', { class: 'inspector-metric' }, [
      h('p', { class: 'text-xs text-gray-500 dark:text-gray-400' }, props.label),
      h('p', { class: 'mt-1 text-base font-bold text-gray-950 dark:text-white' }, props.value)
    ])
  }
})

const InspectorRow = defineComponent({
  props: {
    label: { type: String, required: true },
    value: { type: String, required: true }
  },
  setup(props) {
    return () => h('div', { class: 'inspector-row' }, [
      h('span', { class: 'shrink-0 text-gray-500 dark:text-gray-400' }, props.label),
      h('span', { class: 'min-w-0 text-right font-medium text-gray-900 dark:text-white' }, props.value)
    ])
  }
})

const router = useRouter()
const accounts = ref<AccountPoolMapAccount[]>([])
const pools = ref<AccountPoolMapPool[]>([])
const serverSummary = ref<AccountPoolMapSummary | null>(null)
const sourceInfo = ref<AccountPoolMapResponse['source'] | null>(null)
const generatedAt = ref('')
const errorMessage = ref('')
const knownPlatforms = ref<string[]>([])
const loading = ref(false)
const selectedAccountId = ref<number | null>(null)
const inspectorPanel = ref<HTMLElement | null>(null)
let refreshTimer: ReturnType<typeof setTimeout> | null = null
let refreshSeq = 0

const filters = reactive<{
  search: string
  platform: string
  status: 'all' | AccountStatusKind
  view: ViewMode
}>({
  search: '',
  platform: 'all',
  status: 'all',
  view: 'status'
})

const viewModes: Array<{ value: ViewMode; label: string }> = [
  { value: 'status', label: '状态' },
  { value: 'traffic', label: '流量' },
  { value: 'error', label: '错误' }
]

const platformOptions = computed(() => {
  const set = new Set(knownPlatforms.value)
  if (filters.platform !== 'all') set.add(filters.platform)
  return Array.from(set).sort()
})

const selectedAccount = computed(() => accounts.value.find((item) => item.id === selectedAccountId.value) || null)

const visiblePools = computed<AccountPool[]>(() => pools.value)

const selectedPool = computed(() => {
  const selected = selectedAccount.value
  if (!selected) return null
  return visiblePools.value.find((pool) => pool.accounts.some((item) => item.id === selected.id)) || null
})

const selectedPoolLabel = computed(() => {
  const pool = selectedPool.value
  if (!pool) return '未匹配泳道'
  return `${platformLabel(pool.platform)} · ${accountTypeLabel(pool.type)}`
})

const selectedPoolPeerCount = computed(() => {
  const pool = selectedPool.value
  return pool ? Math.max(0, pool.accounts.length - 1) : 0
})

const attentionAccounts = computed(() => accounts.value
  .filter((account) => statusKind(account) !== 'healthy')
  .sort((a, b) => statusWeight(a) - statusWeight(b))
  .slice(0, 6))

const summary = computed(() => {
  return serverSummary.value || buildSummary(accounts.value, pools.value)
})

const summaryCards = computed(() => [
  {
    key: 'total',
    label: '账号总数',
    value: summary.value.total,
    detail: `${summary.value.pools || visiblePools.value.length} 个池/泳道`,
    dotClass: 'bg-gray-400'
  },
  {
    key: 'healthy',
    label: '健康可调度',
    value: summary.value.healthy,
    detail: summary.value.total ? `${Math.round((summary.value.healthy / summary.value.total) * 100)}% 可用` : '无账号',
    dotClass: 'bg-emerald-500'
  },
  {
    key: 'attention',
    label: '需要关注',
    value: summary.value.attention,
    detail: '错误、冷却、限流或停用',
    dotClass: summary.value.attention > 0 ? 'bg-amber-500' : 'bg-emerald-500'
  },
  {
    key: 'limited',
    label: '限流/停用',
    value: summary.value.rate_limited + summary.value.disabled,
    detail: `${summary.value.rate_limited} 限流 · ${summary.value.disabled} 停用`,
    dotClass: summary.value.rate_limited + summary.value.disabled > 0 ? 'bg-violet-500' : 'bg-gray-300'
  }
])

function statusKind(account: Account): AccountStatusKind {
  const mappedKind = (account as Partial<AccountPoolMapAccount>).status_kind
  if (mappedKind) return mappedKind
  return inferStatusKind(account)
}

function inferStatusKind(account: Account): AccountStatusKind {
  if (account.status === 'inactive') return 'disabled'
  if (account.status === 'error' || account.error_message) return 'error'
  if (isFuture(account.rate_limit_reset_at)) return 'rate_limited'
  if (isFuture(account.overload_until) || isFuture(account.temp_unschedulable_until) || !account.schedulable) return 'degraded'
  return 'healthy'
}

function statusWeight(account: Account): number {
  const weights: Record<AccountStatusKind, number> = {
    error: 0,
    rate_limited: 1,
    degraded: 2,
    disabled: 3,
    healthy: 4
  }
  return weights[statusKind(account)]
}

function statusLabel(account: Account): string {
  const mappedLabel = (account as Partial<AccountPoolMapAccount>).status_label
  if (mappedLabel) return mappedLabel
  return statusKindLabel(statusKind(account))
}

function statusKindLabel(kind: AccountStatusKind): string {
  const labels: Record<AccountStatusKind, string> = {
    healthy: '健康',
    degraded: '降级',
    rate_limited: '限流',
    error: '错误',
    disabled: '停用'
  }
  return labels[kind]
}

function statusBadgeClass(account: Account): string {
  const classes: Record<AccountStatusKind, string> = {
    healthy: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300',
    degraded: 'bg-amber-100 text-amber-700 dark:bg-amber-500/15 dark:text-amber-300',
    rate_limited: 'bg-violet-100 text-violet-700 dark:bg-violet-500/15 dark:text-violet-300',
    error: 'bg-rose-100 text-rose-700 dark:bg-rose-500/15 dark:text-rose-300',
    disabled: 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
  }
  return classes[statusKind(account)]
}

function nodeClass(account: Account): string {
  if (filters.view === 'traffic') {
    const load = (account.current_rpm || 0) + (account.current_concurrency || 0) + (account.active_sessions || 0)
    if (load >= 20) return 'account-node-blue-strong'
    if (load > 0) return 'account-node-blue'
    return 'account-node-muted'
  }
  if (filters.view === 'error') {
    if (statusKind(account) === 'healthy') return 'account-node-muted'
    return statusNodeClass(account)
  }
  return statusNodeClass(account)
}

function statusNodeClass(account: Account): string {
  const classes: Record<AccountStatusKind, string> = {
    healthy: 'account-node-healthy',
    degraded: 'account-node-degraded',
    rate_limited: 'account-node-limited',
    error: 'account-node-error',
    disabled: 'account-node-muted'
  }
  return classes[statusKind(account)]
}

function isFuture(value?: string | null): boolean {
  if (!value) return false
  const ts = new Date(value).getTime()
  return Number.isFinite(ts) && ts > Date.now()
}

function platformLabel(platform: string): string {
  const labels: Record<string, string> = {
    anthropic: 'Anthropic',
    openai: 'OpenAI',
    gemini: 'Gemini',
    antigravity: 'Antigravity',
    bedrock: 'Bedrock'
  }
  return labels[platform] || platform || 'Unknown'
}

function accountTypeLabel(type: string): string {
  const labels: Record<string, string> = {
    oauth: 'OAuth Pool',
    'setup-token': 'SetupToken Pool',
    apikey: 'API Key Pool',
    upstream: 'Upstream Pool',
    bedrock: 'Bedrock Pool'
  }
  return labels[type] || type || 'Unknown Pool'
}

function poolHealthyCount(pool: AccountPool): number {
  return pool.summary?.healthy ?? pool.accounts.filter((item) => statusKind(item) === 'healthy').length
}

function poolAttentionCount(pool: AccountPool): number {
  return pool.summary?.attention ?? (pool.accounts.length - poolHealthyCount(pool))
}

function poolRPM(pool: AccountPool): number {
  return pool.summary?.rpm ?? pool.accounts.reduce((sum, item) => sum + (item.current_rpm || 0), 0)
}

function poolConcurrency(pool: AccountPool): number {
  return pool.summary?.concurrency ?? pool.accounts.reduce((sum, item) => sum + (item.current_concurrency || 0), 0)
}

function poolFingerprintCount(pool: AccountPool): string {
  const enabled = pool.summary?.tls_enabled ?? pool.accounts.filter((item) => item.enable_tls_fingerprint).length
  return `${enabled}/${pool.accounts.length}`
}

function groupNames(account: Account): string {
  if (account.groups?.length) return account.groups.map((group) => group.name).join(', ')
  if (account.group_ids?.length) return account.group_ids.map((id) => `#${id}`).join(', ')
  return '未分组'
}

function formatDate(value?: string | null): string {
  if (!value) return '从未'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '未知'
  return date.toLocaleString()
}

function selectAccount(account: AccountPoolMapAccount) {
  selectedAccountId.value = account.id
  void nextTick(() => {
    if (typeof window !== 'undefined' && window.innerWidth < 1280) {
      inspectorPanel.value?.scrollIntoView({ behavior: 'smooth', block: 'start' })
    }
  })
}

function clearSelection() {
  selectedAccountId.value = null
}

async function refresh() {
  const seq = ++refreshSeq
  loading.value = true
  errorMessage.value = ''
  try {
    const response = await adminAPI.accounts.getPoolMap({
      platform: filters.platform !== 'all' ? filters.platform : undefined,
      status: filters.status !== 'all' ? filters.status : undefined,
      search: filters.search || undefined,
      limit: 3000
    })
    if (seq !== refreshSeq) return
    loadPoolMapResponse(response)
  } catch (error) {
    if (seq !== refreshSeq) return
    try {
      await loadFallbackAccounts()
      errorMessage.value = normalizeError(error) || '账号池地图接口暂不可用，已回退到账号列表聚合。'
    } catch (fallbackError) {
      errorMessage.value = normalizeError(fallbackError) || normalizeError(error) || '账号池地图加载失败。'
      accounts.value = []
      pools.value = []
      serverSummary.value = createEmptySummary()
      sourceInfo.value = null
      generatedAt.value = ''
    }
  } finally {
    if (seq === refreshSeq) {
      loading.value = false
    }
  }
}

function loadPoolMapResponse(response: AccountPoolMapResponse) {
  const nextAccounts = response.accounts || []
  accounts.value = nextAccounts
  pools.value = response.pools || groupAccounts(nextAccounts)
  serverSummary.value = response.summary || buildSummary(nextAccounts, pools.value)
  sourceInfo.value = response.source || null
  generatedAt.value = response.generated_at || ''
  mergeKnownPlatforms(nextAccounts)
  if (selectedAccountId.value && !nextAccounts.some((item) => item.id === selectedAccountId.value)) {
    selectedAccountId.value = null
  }
}

async function loadFallbackAccounts() {
  const response = await adminAPI.accounts.list(1, 1000, {
    platform: filters.platform !== 'all' ? filters.platform : undefined,
    search: filters.search || undefined,
    sort_by: 'platform',
    sort_order: 'asc'
  })
  const nextAccounts = (response.items || [])
    .map(enrichFallbackAccount)
    .filter((account) => filters.status === 'all' || statusKind(account) === filters.status)
  accounts.value = nextAccounts
  pools.value = groupAccounts(nextAccounts)
  serverSummary.value = buildSummary(nextAccounts, pools.value)
  sourceInfo.value = {
    total: response.total,
    returned: response.items?.length || 0,
    limit: 1000,
    truncated: response.total > (response.items?.length || 0)
  }
  generatedAt.value = new Date().toISOString()
  mergeKnownPlatforms(nextAccounts)
  if (selectedAccountId.value && !nextAccounts.some((item) => item.id === selectedAccountId.value)) {
    selectedAccountId.value = null
  }
}

function enrichFallbackAccount(account: Account): AccountPoolMapAccount {
  const kind = inferStatusKind(account)
  return {
    ...account,
    status_kind: kind,
    status_label: statusKindLabel(kind),
    status_reason: account.temp_unschedulable_reason || account.error_message || '',
    attention: kind !== 'healthy'
  }
}

function groupAccounts(items: AccountPoolMapAccount[]): AccountPoolMapPool[] {
  const map = new Map<string, AccountPoolMapPool>()
  for (const account of items) {
    const key = `${account.platform}:${account.type}`
    if (!map.has(key)) {
      map.set(key, {
        key,
        platform: account.platform,
        type: account.type,
        summary: createEmptySummary(),
        accounts: []
      })
    }
    const pool = map.get(key)
    if (!pool) continue
    pool.accounts.push(account)
    incrementSummary(pool.summary, account)
  }
  return Array.from(map.values())
    .map((pool) => ({
      ...pool,
      summary: { ...pool.summary, pools: 1 },
      accounts: pool.accounts.slice().sort((a, b) => statusWeight(a) - statusWeight(b) || a.name.localeCompare(b.name))
    }))
    .sort((a, b) => platformLabel(a.platform).localeCompare(platformLabel(b.platform)) || accountTypeLabel(a.type).localeCompare(accountTypeLabel(b.type)))
}

function buildSummary(items: AccountPoolMapAccount[], groupedPools: AccountPoolMapPool[]): AccountPoolMapSummary {
  const next = createEmptySummary()
  for (const account of items) {
    incrementSummary(next, account)
  }
  next.pools = groupedPools.length
  return next
}

function createEmptySummary(): AccountPoolMapSummary {
  return {
    total: 0,
    healthy: 0,
    degraded: 0,
    rate_limited: 0,
    error: 0,
    disabled: 0,
    attention: 0,
    pools: 0,
    tls_enabled: 0,
    rpm: 0,
    concurrency: 0,
    active_sessions: 0
  }
}

function incrementSummary(summary: AccountPoolMapSummary, account: AccountPoolMapAccount) {
  summary.total += 1
  if (account.status_kind === 'healthy') summary.healthy += 1
  if (account.status_kind === 'degraded') summary.degraded += 1
  if (account.status_kind === 'rate_limited') summary.rate_limited += 1
  if (account.status_kind === 'error') summary.error += 1
  if (account.status_kind === 'disabled') summary.disabled += 1
  if (account.attention) summary.attention += 1
  if (account.enable_tls_fingerprint) summary.tls_enabled += 1
  summary.rpm += account.current_rpm || 0
  summary.concurrency += account.current_concurrency || 0
  summary.active_sessions += account.active_sessions || 0
}

function mergeKnownPlatforms(items: AccountPoolMapAccount[]) {
  const set = new Set(knownPlatforms.value)
  for (const account of items) {
    if (account.platform) set.add(account.platform)
  }
  knownPlatforms.value = Array.from(set).sort()
}

function normalizeError(error: unknown): string {
  if (error && typeof error === 'object' && 'message' in error) {
    return String((error as { message?: unknown }).message || '')
  }
  return ''
}

function scheduleRefresh() {
  if (refreshTimer) clearTimeout(refreshTimer)
  refreshTimer = setTimeout(() => {
    void refresh()
  }, filters.search ? 280 : 0)
}

function goAccounts() {
  router.push('/admin/accounts')
}

function goAccountDetail(account: Account) {
  router.push({ path: '/admin/accounts', query: { search: account.name } })
}

function goFingerprints() {
  router.push('/admin/fingerprints')
}

function goUsage(account: Account) {
  router.push({ path: '/admin/usage', query: { account_id: String(account.id) } })
}

watch(
  () => [filters.search, filters.platform, filters.status],
  () => scheduleRefresh()
)

onMounted(() => {
  void refresh()
})

onUnmounted(() => {
  if (refreshTimer) clearTimeout(refreshTimer)
})
</script>

<style scoped>
.account-map-page {
  --account-map-border: rgb(17 24 39 / 0.06);
  --account-map-border-strong: rgb(229 231 235);
  --account-map-surface: rgb(255 255 255);
  --account-map-subtle: rgb(249 250 251);
  --account-map-shadow: 0 1px 2px rgb(15 23 42 / 0.05);
}

:global(.dark) .account-map-page {
  --account-map-border: rgb(55 65 81);
  --account-map-border-strong: rgb(75 85 99);
  --account-map-surface: rgb(31 41 55);
  --account-map-subtle: rgb(17 24 39 / 0.58);
  --account-map-shadow: none;
}

.account-map-toolbar,
.account-map-stat-card,
.account-map-filter-card,
.account-map-empty-state,
.account-pool-lane,
.account-map-inspector {
  border: 1px solid var(--account-map-border);
  background: var(--account-map-surface);
  box-shadow: var(--account-map-shadow);
}

.account-map-toolbar {
  border-radius: 1.5rem;
  padding: 1.5rem;
}

.account-map-summary-grid {
  align-items: stretch;
}

.account-map-stat-card {
  display: flex;
  min-height: 6.8rem;
  flex-direction: column;
  justify-content: space-between;
  border-radius: 1.25rem;
  padding: 1rem;
}

.account-map-main-column {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.account-map-filter-card {
  border-radius: 1.25rem;
  padding: 1rem;
}

.account-map-empty-state {
  border-style: dashed;
  border-radius: 1.25rem;
  padding: 2rem;
  text-align: center;
}

.account-map-workspace {
  align-items: start;
}

.account-pool-lane {
  overflow: hidden;
  box-shadow: 0 1px 2px rgb(15 23 42 / 0.04);
}

.account-map-primary-button,
.account-map-secondary-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 0.75rem;
  padding: 0.55rem 0.8rem;
  font-size: 0.875rem;
  font-weight: 700;
  transition:
    border-color 0.15s ease,
    background-color 0.15s ease,
    color 0.15s ease,
    box-shadow 0.15s ease;
}

.account-map-primary-button {
  background: rgb(37 99 235);
  color: white;
  box-shadow: 0 1px 2px rgb(37 99 235 / 0.18);
}

.account-map-primary-button:hover {
  background: rgb(29 78 216);
}

.account-map-secondary-button {
  border: 1px solid rgb(229 231 235);
  color: rgb(55 65 81);
}

.account-map-secondary-button:hover {
  border-color: rgb(191 219 254);
  color: rgb(37 99 235);
}

:global(.dark) .account-map-secondary-button {
  border-color: rgb(75 85 99);
  color: rgb(209 213 219);
}

:global(.dark) .account-map-secondary-button:hover {
  border-color: rgb(59 130 246 / 0.7);
  color: rgb(147 197 253);
}

.account-map-detail-rail {
  min-width: 0;
}

.account-map-inspector {
  position: sticky;
  top: 5.5rem;
  overflow: hidden;
  border-radius: 1.25rem;
}

.inspector-heading,
.inspector-empty-header {
  padding: 1rem;
}

.inspector-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.85rem;
  border-bottom: 1px solid rgb(243 244 246);
}

:global(.dark) .inspector-heading {
  border-bottom-color: rgb(31 41 55);
}

.inspector-kicker {
  font-size: 0.7rem;
  font-weight: 800;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  color: rgb(107 114 128);
}

:global(.dark) .inspector-kicker {
  color: rgb(156 163 175);
}

.inspector-title {
  margin-top: 0.35rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 1.05rem;
  font-weight: 800;
  line-height: 1.2;
  color: rgb(17 24 39);
}

:global(.dark) .inspector-title {
  color: rgb(255 255 255);
}

.inspector-subtitle {
  margin-top: 0.3rem;
  font-size: 0.8125rem;
  color: rgb(107 114 128);
}

:global(.dark) .inspector-subtitle {
  color: rgb(156 163 175);
}

.inspector-close {
  display: inline-flex;
  height: 2rem;
  width: 2rem;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  border-radius: 999px;
  border: 1px solid rgb(229 231 235);
  color: rgb(107 114 128);
  transition:
    border-color 0.15s ease,
    background-color 0.15s ease,
    color 0.15s ease;
}

.inspector-close:hover {
  border-color: rgb(191 219 254);
  background: rgb(239 246 255);
  color: rgb(29 78 216);
}

:global(.dark) .inspector-close {
  border-color: rgb(55 65 81);
  color: rgb(156 163 175);
}

:global(.dark) .inspector-close:hover {
  border-color: rgb(59 130 246 / 0.7);
  background: rgb(30 64 175 / 0.22);
  color: rgb(191 219 254);
}

.inspector-status-card {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  margin: 1rem;
  border-radius: 0.9rem;
  border: 1px solid var(--account-map-border-strong);
  padding: 0.85rem;
  background: var(--account-map-subtle);
}

.inspector-status-card > div {
  min-width: 0;
}

.inspector-status-label {
  display: block;
  margin-bottom: 0.3rem;
  font-size: 0.72rem;
  font-weight: 700;
  color: rgb(107 114 128);
}

.inspector-status-card strong {
  display: block;
  font-size: 1rem;
  font-weight: 800;
  color: rgb(17 24 39);
}

.inspector-status-card p {
  margin-top: 0.35rem;
  display: -webkit-box;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 3;
  font-size: 0.78rem;
  line-height: 1.45;
  color: rgb(107 114 128);
}

:global(.dark) .inspector-status-card {
  border-color: rgb(55 65 81 / 0.82);
  background: rgb(31 41 55 / 0.56);
}

:global(.dark) .inspector-status-card strong {
  color: rgb(255 255 255);
}

:global(.dark) .inspector-status-card p,
:global(.dark) .inspector-status-label {
  color: rgb(156 163 175);
}

.inspector-status-healthy {
  border-color: rgb(167 243 208);
  background: linear-gradient(135deg, rgb(236 253 245), rgb(255 255 255));
}

.inspector-status-degraded {
  border-color: rgb(253 230 138);
  background: linear-gradient(135deg, rgb(255 251 235), rgb(255 255 255));
}

.inspector-status-rate_limited {
  border-color: rgb(221 214 254);
  background: linear-gradient(135deg, rgb(245 243 255), rgb(255 255 255));
}

.inspector-status-error {
  border-color: rgb(254 205 211);
  background: linear-gradient(135deg, rgb(255 241 242), rgb(255 255 255));
}

.inspector-status-disabled {
  border-color: rgb(229 231 235);
  background: rgb(249 250 251);
}

:global(.dark) .inspector-status-healthy {
  border-color: rgb(16 185 129 / 0.45);
  background: rgb(6 78 59 / 0.22);
}

:global(.dark) .inspector-status-degraded {
  border-color: rgb(245 158 11 / 0.5);
  background: rgb(120 53 15 / 0.25);
}

:global(.dark) .inspector-status-rate_limited {
  border-color: rgb(139 92 246 / 0.5);
  background: rgb(76 29 149 / 0.25);
}

:global(.dark) .inspector-status-error {
  border-color: rgb(244 63 94 / 0.5);
  background: rgb(136 19 55 / 0.25);
}

:global(.dark) .inspector-status-disabled {
  border-color: rgb(55 65 81);
  background: rgb(31 41 55 / 0.72);
}

.inspector-metric-grid,
.inspector-overview-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.65rem;
  padding: 0 1rem 1rem;
}

.inspector-overview-grid {
  padding: 0;
}

.inspector-metric {
  border-radius: 0.9rem;
  border: 1px solid var(--account-map-border);
  background: rgb(255 255 255 / 0.72);
  padding: 0.75rem;
}

:global(.dark) .inspector-metric {
  border-color: rgb(55 65 81 / 0.74);
  background: rgb(17 24 39 / 0.46);
}

.inspector-section {
  margin: 0 1rem 1rem;
  border-radius: 1rem;
  border: 1px solid var(--account-map-border);
  background: rgb(255 255 255 / 0.72);
  padding: 0.9rem;
}

:global(.dark) .inspector-section {
  border-color: rgb(55 65 81 / 0.7);
  background: rgb(17 24 39 / 0.36);
}

.inspector-section-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  margin-bottom: 0.8rem;
}

.inspector-section-title h3 {
  font-size: 0.9rem;
  font-weight: 800;
  color: rgb(17 24 39);
}

.inspector-section-title span {
  min-width: 0;
  max-width: 55%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 0.72rem;
  font-weight: 650;
  color: rgb(107 114 128);
}

:global(.dark) .inspector-section-title h3 {
  color: rgb(255 255 255);
}

:global(.dark) .inspector-section-title span {
  color: rgb(156 163 175);
}

.inspector-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  border-bottom: 1px solid rgb(243 244 246);
  padding-bottom: 0.55rem;
  font-size: 0.875rem;
}

.inspector-row + .inspector-row {
  padding-top: 0.55rem;
}

.inspector-row:last-child {
  border-bottom: 0;
  padding-bottom: 0;
}

:global(.dark) .inspector-row {
  border-bottom-color: rgb(31 41 55);
}

.inspector-alert {
  margin: 0 1rem 1rem;
  border-radius: 1rem;
  border: 1px solid rgb(253 230 138);
  background: rgb(255 251 235);
  padding: 0.85rem;
  font-size: 0.875rem;
  color: rgb(146 64 14);
}

:global(.dark) .inspector-alert {
  border-color: rgb(245 158 11 / 0.38);
  background: rgb(245 158 11 / 0.12);
  color: rgb(253 230 138);
}

.inspector-actions {
  display: grid;
  gap: 0.55rem;
  padding: 0 1rem 1rem;
}

.inspector-empty-header {
  border-bottom: 1px solid rgb(243 244 246);
}

:global(.dark) .inspector-empty-header {
  border-bottom-color: rgb(31 41 55);
}

.inspector-attention-item {
  width: 100%;
  border-radius: 0.9rem;
  border: 1px solid var(--account-map-border);
  background: var(--account-map-subtle);
  padding: 0.75rem;
  text-align: left;
  transition:
    border-color 0.15s ease,
    background-color 0.15s ease,
    transform 0.15s ease;
}

.inspector-attention-item + .inspector-attention-item {
  margin-top: 0.55rem;
}

.inspector-attention-item:hover {
  transform: translateY(-1px);
  border-color: rgb(191 219 254);
  background: rgb(255 255 255);
}

:global(.dark) .inspector-attention-item {
  border-color: rgb(55 65 81 / 0.7);
  background: rgb(31 41 55 / 0.58);
}

:global(.dark) .inspector-attention-item:hover {
  border-color: rgb(59 130 246 / 0.58);
  background: rgb(31 41 55 / 0.82);
}

.inspector-good-state {
  border-radius: 0.9rem;
  border: 1px solid rgb(167 243 208);
  background: rgb(236 253 245);
  padding: 0.85rem;
  font-size: 0.875rem;
  color: rgb(4 120 87);
}

:global(.dark) .inspector-good-state {
  border-color: rgb(16 185 129 / 0.35);
  background: rgb(16 185 129 / 0.12);
  color: rgb(167 243 208);
}

.account-node {
  position: relative;
  min-height: 5.6rem;
  overflow: hidden;
  border: 1px solid rgb(229 231 235);
  border-radius: 1rem;
  padding: 0.85rem;
  background: rgb(255 255 255);
  color: rgb(17 24 39);
  transition:
    border-color 0.15s ease,
    box-shadow 0.15s ease,
    transform 0.15s ease,
    background-color 0.15s ease;
}

.account-node::before {
  content: '';
  position: absolute;
  inset: 0 auto 0 0;
  width: 3px;
  background: currentColor;
  opacity: 0.45;
}

.account-node > * {
  position: relative;
}

.account-node:hover {
  transform: translateY(-1px);
  background: rgb(249 250 251);
  box-shadow: 0 10px 24px rgb(15 23 42 / 0.07);
}

:global(.dark) .account-node {
  border-color: rgb(31 41 55);
  background: rgb(17 24 39 / 0.72);
  color: rgb(243 244 246);
}

:global(.dark) .account-node:hover {
  background: rgb(31 41 55 / 0.82);
  box-shadow: 0 10px 24px rgb(0 0 0 / 0.18);
}

.account-node-selected {
  border-color: rgb(37 99 235) !important;
  background: rgb(239 246 255);
  box-shadow: 0 0 0 4px rgb(59 130 246 / 0.13), 0 12px 28px rgb(15 23 42 / 0.1);
}

.account-node-healthy {
  border-color: rgb(167 243 208);
  background: rgb(255 255 255);
  color: rgb(6 95 70);
}

.account-node-degraded {
  border-color: rgb(253 230 138);
  background: rgb(255 255 255);
  color: rgb(146 64 14);
}

.account-node-limited {
  border-color: rgb(221 214 254);
  background: rgb(255 255 255);
  color: rgb(109 40 217);
}

.account-node-error {
  border-color: rgb(254 205 211);
  background: rgb(255 255 255);
  color: rgb(190 18 60);
}

.account-node-muted {
  border-color: rgb(229 231 235);
  background: rgb(249 250 251);
  color: rgb(75 85 99);
}

.account-node-blue {
  border-color: rgb(191 219 254);
  background: rgb(255 255 255);
  color: rgb(29 78 216);
}

.account-node-blue-strong {
  border-color: rgb(96 165 250);
  background: rgb(239 246 255);
  color: rgb(30 64 175);
}

:global(.dark) .account-node-healthy {
  border-color: rgb(16 185 129 / 0.45);
  background: rgb(6 78 59 / 0.22);
  color: rgb(167 243 208);
}

:global(.dark) .account-node-degraded {
  border-color: rgb(245 158 11 / 0.5);
  background: rgb(120 53 15 / 0.25);
  color: rgb(253 230 138);
}

:global(.dark) .account-node-limited {
  border-color: rgb(139 92 246 / 0.5);
  background: rgb(76 29 149 / 0.25);
  color: rgb(221 214 254);
}

:global(.dark) .account-node-error {
  border-color: rgb(244 63 94 / 0.5);
  background: rgb(136 19 55 / 0.25);
  color: rgb(254 205 211);
}

:global(.dark) .account-node-muted {
  border-color: rgb(55 65 81);
  background: rgb(31 41 55 / 0.72);
  color: rgb(156 163 175);
}

:global(.dark) .account-node-blue,
:global(.dark) .account-node-blue-strong {
  border-color: rgb(59 130 246 / 0.5);
  background: rgb(30 64 175 / 0.25);
  color: rgb(191 219 254);
}

.inspector-action,
.inspector-action-primary {
  width: 100%;
  border-radius: 0.9rem;
  padding: 0.65rem 0.8rem;
  font-size: 0.875rem;
  font-weight: 650;
  transition:
    border-color 0.15s ease,
    background-color 0.15s ease,
    color 0.15s ease,
    transform 0.15s ease;
}

.inspector-action-primary {
  background: rgb(37 99 235);
  color: white;
}

.inspector-action-primary:hover {
  background: rgb(29 78 216);
}

.inspector-action {
  border: 1px solid rgb(229 231 235);
  color: rgb(55 65 81);
}

.inspector-action:hover {
  border-color: rgb(147 197 253);
  color: rgb(37 99 235);
}

:global(.dark) .inspector-action {
  border-color: rgb(55 65 81);
  color: rgb(209 213 219);
}

:global(.dark) .inspector-action:hover {
  border-color: rgb(59 130 246 / 0.7);
  color: rgb(147 197 253);
}

:global(.app-shell.app-shell-anti) .account-map-page {
  color: var(--anti-ink);
}

:global(.app-shell.app-shell-anti) .account-map-page :is(.account-map-toolbar, .account-map-stat-card, .account-map-filter-card, .account-map-empty-state, .account-pool-lane, .account-map-inspector, .account-node, .inspector-section, .inspector-metric, .inspector-attention-item, .inspector-alert, .inspector-good-state) {
  border: 4px solid var(--anti-ink) !important;
  border-radius: 0.35rem !important;
  background: var(--anti-paper) !important;
  box-shadow: 7px 7px 0 var(--anti-ink) !important;
  color: var(--anti-ink) !important;
}

:global(.app-shell.app-shell-anti) .account-map-page .account-map-toolbar {
  background: var(--anti-yellow) !important;
  transform: rotate(-0.25deg);
}

:global(.app-shell.app-shell-anti) .account-map-page :is(h1, h2, h3, p, span, label, strong, small) {
  color: var(--anti-ink) !important;
}

:global(.app-shell.app-shell-anti) .account-map-page :is(input, select) {
  border: 3px solid var(--anti-ink) !important;
  border-radius: 0 !important;
  background: var(--anti-paper) !important;
  box-shadow: 4px 4px 0 var(--anti-blue) !important;
  color: var(--anti-ink) !important;
}

:global(.app-shell.app-shell-anti) .account-map-page button {
  border: 3px solid var(--anti-ink) !important;
  border-radius: 0.2rem !important;
  box-shadow: 4px 4px 0 var(--anti-ink) !important;
  color: var(--anti-ink) !important;
  font-weight: 950;
}

:global(.app-shell.app-shell-anti) .account-map-page button:hover {
  background: var(--anti-green) !important;
  transform: rotate(-1deg) translate(-1px, -1px);
}

:global(.app-shell.app-shell-anti) .account-map-page .account-node:nth-child(odd) {
  transform: rotate(-0.35deg);
}

:global(.app-shell.app-shell-anti) .account-map-page .account-node:nth-child(even) {
  transform: rotate(0.35deg);
}

:global(.app-shell.app-shell-anti) .account-map-page .account-node:hover,
:global(.app-shell.app-shell-anti) .account-map-page .account-node-selected {
  background: var(--anti-green) !important;
  box-shadow: 9px 9px 0 var(--anti-red) !important;
  transform: rotate(-1deg) translate(-2px, -2px);
}

:global(.app-shell.app-shell-anti) .account-map-page .inspector-status-card {
  border: 4px solid var(--anti-ink) !important;
  border-radius: 0.35rem !important;
  background: var(--anti-yellow) !important;
  box-shadow: 6px 6px 0 var(--anti-blue) !important;
  color: var(--anti-ink) !important;
}

:global(.app-shell.app-shell-anti) .account-map-page .inspector-kicker {
  display: inline-flex;
  border: 3px solid var(--anti-ink);
  background: var(--anti-red);
  color: var(--anti-paper) !important;
  padding: 0.2rem 0.35rem;
}

@media (max-width: 1279px) {
  .account-map-inspector {
    position: static;
  }
}

@media (max-width: 640px) {
  .inspector-metric-grid,
  .inspector-overview-grid {
    grid-template-columns: 1fr;
  }

  .inspector-section-title {
    align-items: flex-start;
    flex-direction: column;
    gap: 0.25rem;
  }

  .inspector-section-title span {
    max-width: 100%;
  }
}
</style>
