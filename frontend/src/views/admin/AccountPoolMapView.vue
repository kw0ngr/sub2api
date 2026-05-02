<template>
  <AppLayout>
    <div class="account-map-page space-y-5 pb-12">
      <section class="account-map-toolbar">
        <div class="account-map-toolbar-copy">
          <div class="min-w-0">
            <p class="account-map-eyebrow">Account pool map</p>
            <h1>
              <svg class="h-5 w-5 text-sky-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M4 7a2 2 0 012-2h3a2 2 0 012 2v3a2 2 0 01-2 2H6a2 2 0 01-2-2V7zm9 0a2 2 0 012-2h3a2 2 0 012 2v3a2 2 0 01-2 2h-3a2 2 0 01-2-2V7zM4 16a2 2 0 012-2h3a2 2 0 012 2v1a2 2 0 01-2 2H6a2 2 0 01-2-2v-1zm9 0a2 2 0 012-2h3a2 2 0 012 2v1a2 2 0 01-2 2h-3a2 2 0 01-2-2v-1z"
                />
              </svg>
              账号池地图
            </h1>
            <div class="account-map-meta">
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
          <div class="account-map-toolbar-actions">
            <button
              type="button"
              class="account-map-secondary-button"
              :disabled="checkingHealth || loading"
              @click="runHealthCheck"
            >
              {{ checkingHealth ? '巡检中...' : '健康检测' }}
            </button>
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
          :class="`account-map-stat-${item.tone}`"
        >
          <div class="flex items-center justify-between gap-3">
            <p>{{ item.label }}</p>
            <span class="h-2.5 w-2.5 rounded-full" :class="item.dotClass"></span>
          </div>
          <strong>{{ item.value }}</strong>
          <small>{{ item.detail }}</small>
        </div>
      </section>

      <section class="account-map-workspace grid gap-6 xl:grid-cols-[minmax(0,1fr)_440px]">
        <div class="account-map-main-column">
        <div class="account-map-filter-card">
          <div class="account-map-filter-header">
            <div>
              <h2>筛选账号池</h2>
              <p>先缩小范围，再展开查看具体账号，避免异常账号一次性淹没页面。</p>
            </div>
          </div>
          <div class="grid gap-3 md:grid-cols-[1.2fr_0.8fr_0.8fr_auto]">
            <label class="account-map-field">
              <span>搜索账号</span>
              <input
                v-model.trim="filters.search"
                type="search"
                class="account-map-control"
                placeholder="名称、平台、分组、错误信息"
              />
            </label>
            <label class="account-map-field">
              <span>平台</span>
              <select
                v-model="filters.platform"
                class="account-map-control"
              >
                <option value="all">全部平台</option>
                <option v-for="platform in platformOptions" :key="platform" :value="platform">
                  {{ platformLabel(platform) }}
                </option>
              </select>
            </label>
            <label class="account-map-field">
              <span>状态</span>
              <select
                v-model="filters.status"
                class="account-map-control"
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
              <div class="account-map-segmented">
                <button
                  v-for="mode in viewModes"
                  :key="mode.value"
                  type="button"
                  :class="{ active: filters.view === mode.value }"
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
            class="account-pool-lane"
          >
            <div class="account-pool-lane-header">
              <div class="min-w-0">
                <div class="flex flex-wrap items-center gap-2">
                  <span class="account-pool-platform">
                    {{ platformLabel(pool.platform) }}
                  </span>
                  <h2>
                    {{ accountTypeLabel(pool.type) }}
                  </h2>
                </div>
                <p>
                  {{ pool.accounts.length }} 个账号 · 健康 {{ poolHealthyCount(pool) }} · 需关注 {{ poolAttentionCount(pool) }}
                </p>
              </div>
              <div class="account-pool-mini-metrics">
                <div>
                  <span>RPM</span>
                  <strong>{{ poolRPM(pool) }}</strong>
                </div>
                <div>
                  <span>并发</span>
                  <strong>{{ poolConcurrency(pool) }}</strong>
                </div>
                <div>
                  <span>指纹</span>
                  <strong>{{ poolFingerprintCount(pool) }}</strong>
                </div>
                <div>
                  <span>额度</span>
                  <strong>{{ poolQuotaSignalCount(pool) }}</strong>
                </div>
              </div>
            </div>

            <div class="account-node-grid">
              <button
                v-for="account in visiblePoolAccounts(pool)"
                :key="account.id"
                type="button"
                class="account-node group text-left"
                :class="[nodeClass(account), selectedAccount?.id === account.id ? 'account-node-selected' : '']"
                @click="selectAccount(account)"
              >
                <div class="flex items-start justify-between gap-2">
                  <div class="min-w-0">
                    <p class="truncate text-sm font-bold">{{ account.name }}</p>
                    <p class="account-node-subtitle">
                      <span class="account-node-status">{{ statusLabel(account) }}</span>
                      <span>{{ account.type }}</span>
                    </p>
                  </div>
                  <span class="mt-1 h-2.5 w-2.5 shrink-0 rounded-full bg-current opacity-70"></span>
                </div>
                <div class="account-node-meta">
                  <span>RPM {{ account.current_rpm ?? 0 }}</span>
                  <span>并发 {{ account.current_concurrency ?? 0 }}</span>
                  <span v-if="account.enable_tls_fingerprint">TLS</span>
                  <span v-else>no TLS</span>
                </div>
                <div v-if="quotaBrief(account)" class="account-node-quota">
                  <span>{{ quotaBrief(account) }}</span>
                  <small>{{ quotaSourceLabel(quotaSnapshot(account)) }}</small>
                </div>
              </button>
            </div>

            <div v-if="poolHiddenCount(pool) > 0 || isPoolExpanded(pool)" class="account-pool-footer">
              <span v-if="poolHiddenCount(pool) > 0">
                已优先显示 {{ visiblePoolAccounts(pool).length }} 个账号，剩余 {{ poolHiddenCount(pool) }} 个折叠。
              </span>
              <span v-else>
                已展开全部 {{ pool.accounts.length }} 个账号。
              </span>
              <button type="button" class="account-map-link-button" @click="togglePoolExpanded(pool)">
                {{ isPoolExpanded(pool) ? '收起' : '展开全部' }}
              </button>
            </div>
          </article>
        </div>
      </div>

      <aside class="account-map-detail-rail">
        <section ref="inspectorPanel" class="account-map-inspector" aria-live="polite">
          <template v-if="selectedAccount">
            <div class="inspector-profile-card">
              <button type="button" class="inspector-close" aria-label="取消选择账号" @click="clearSelection">
                ×
              </button>
              <p class="inspector-kicker">账号详情</p>
              <h2 class="inspector-title">{{ selectedAccount.name }}</h2>
              <div class="inspector-profile-meta">
                <span>{{ platformLabel(selectedAccount.platform) }}</span>
                <span>{{ accountTypeLabel(selectedAccount.type) }}</span>
                <span>{{ selectedPoolPeerCount + 1 }} 个同池账号</span>
              </div>

              <div class="inspector-status-card" :class="`inspector-status-${statusKind(selectedAccount)}`">
                <div>
                  <span class="inspector-status-label">调度判断</span>
                  <strong>{{ statusLabel(selectedAccount) }}</strong>
                  <p>{{ selectedAccount.status_reason || selectedAccount.temp_unschedulable_reason || selectedAccount.error_message || '当前可按调度策略参与路由。' }}</p>
                </div>
                <span class="inspector-status-pill" :class="statusBadgeClass(selectedAccount)">
                  {{ selectedAccount.schedulable ? '可调度' : '暂停' }}
                </span>
              </div>
            </div>

            <div class="inspector-snapshot-grid">
              <MetricTile label="并发" :value="String(selectedAccount.current_concurrency ?? 0)" />
              <MetricTile label="RPM" :value="String(selectedAccount.current_rpm ?? 0)" />
              <MetricTile label="会话" :value="String(selectedAccount.active_sessions ?? 0)" />
              <MetricTile label="额度" :value="quotaBrief(selectedAccount) || '未探测'" />
            </div>

            <div class="inspector-section inspector-quota-section" :class="quotaPanelClass(quotaSnapshot(selectedAccount))">
              <div class="inspector-section-title">
                <div>
                  <h3>额度探测</h3>
                  <p>{{ quotaSummaryText(quotaSnapshot(selectedAccount)) }}</p>
                </div>
                <span>{{ quotaUpdatedText(quotaSnapshot(selectedAccount)) }}</span>
              </div>

              <div v-if="quotaStatItems(quotaSnapshot(selectedAccount)).length" class="quota-stat-grid">
                <div
                  v-for="item in quotaStatItems(quotaSnapshot(selectedAccount))"
                  :key="item.label"
                  class="quota-stat-card"
                >
                  <span>{{ item.label }}</span>
                  <strong>{{ item.value }}</strong>
                  <small>{{ item.hint }}</small>
                </div>
              </div>
              <div v-else class="quota-empty-state">
                <span>{{ quotaCapabilityLabel(quotaSnapshot(selectedAccount)) }}</span>
                <strong>{{ quotaEmptyTitle(quotaSnapshot(selectedAccount)) }}</strong>
                <p>{{ quotaGuidance(quotaSnapshot(selectedAccount)) }}</p>
              </div>

              <div v-if="quotaDetailRows(quotaSnapshot(selectedAccount)).length" class="quota-fact-list">
                <InspectorRow
                  v-for="row in quotaDetailRows(quotaSnapshot(selectedAccount))"
                  :key="row.label"
                  :label="row.label"
                  :value="row.value"
                />
              </div>
            </div>

            <div class="inspector-section-grid">
              <div class="inspector-section">
                <div class="inspector-section-title">
                  <div>
                    <h3>池与路由</h3>
                    <p>{{ selectedPoolLabel }}</p>
                  </div>
                </div>
                <InspectorRow label="分组" :value="groupNames(selectedAccount)" />
                <InspectorRow label="代理" :value="selectedAccount.proxy?.name || '未绑定'" />
                <InspectorRow label="同池账号" :value="`${selectedPoolPeerCount} 个`" />
              </div>

              <div class="inspector-section">
                <div class="inspector-section-title">
                  <div>
                    <h3>客户端策略</h3>
                    <p>HTTP / TLS</p>
                  </div>
                </div>
                <InspectorRow label="TLS 指纹" :value="selectedAccount.enable_tls_fingerprint ? `已启用 #${selectedAccount.tls_fingerprint_profile_id || '-'}` : '未启用'" />
                <InspectorRow label="缓存 TTL" :value="selectedAccount.cache_ttl_override_enabled ? `强制 ${selectedAccount.cache_ttl_override_target || '-'}` : '按上游返回'" />
                <InspectorRow label="最近使用" :value="formatDate(selectedAccount.last_used_at)" />
              </div>
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
                <MetricTile label="额度信号" :value="String(summary.quota_signals || 0)" />
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
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, nextTick, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { adminAPI } from '@/api/admin'
import AppLayout from '@/components/layout/AppLayout.vue'
import type { Account } from '@/types'
import type {
  AccountPoolMapAccount,
  AccountPoolMapPool,
  AccountPoolMapResponse,
  AccountPoolMapStatusKind,
  AccountPoolMapSummary,
  APIKeyProbeQuotaSnapshot
} from '@/api/admin/accounts'

type ViewMode = 'status' | 'traffic' | 'error'
type AccountStatusKind = AccountPoolMapStatusKind
type AccountPool = AccountPoolMapPool
type DetailRow = { label: string; value: string }
type QuotaStatItem = { label: string; value: string; hint: string }

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
const expandedPoolKeys = ref<Set<string>>(new Set())
const checkingHealth = ref(false)
let refreshTimer: ReturnType<typeof setTimeout> | null = null
let healthPollTimer: ReturnType<typeof setTimeout> | null = null
let refreshSeq = 0
const POOL_PREVIEW_LIMIT = 12

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
    dotClass: 'bg-slate-400',
    tone: 'neutral'
  },
  {
    key: 'healthy',
    label: '健康可调度',
    value: summary.value.healthy,
    detail: summary.value.total ? `${Math.round((summary.value.healthy / summary.value.total) * 100)}% 可用` : '无账号',
    dotClass: 'bg-emerald-500',
    tone: 'good'
  },
  {
    key: 'attention',
    label: '需要关注',
    value: summary.value.attention,
    detail: '错误、冷却、限流或停用',
    dotClass: summary.value.attention > 0 ? 'bg-amber-500' : 'bg-emerald-500',
    tone: summary.value.attention > 0 ? 'warn' : 'good'
  },
  {
    key: 'quota',
    label: '额度信号',
    value: summary.value.quota_signals || 0,
    detail: `${summary.value.rate_limited} 限流 · ${summary.value.disabled} 停用`,
    dotClass: summary.value.quota_signals > 0 ? 'bg-sky-500' : 'bg-slate-300',
    tone: summary.value.quota_signals > 0 ? 'accent' : 'neutral'
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
    openrouter: 'OpenRouter',
    deepseek: 'DeepSeek',
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

function poolQuotaSignalCount(pool: AccountPool): number {
  return pool.summary?.quota_signals ?? pool.accounts.filter((item) => quotaHasAnySignal(quotaSnapshot(item))).length
}

function quotaSnapshot(account: Account | AccountPoolMapAccount | null | undefined): APIKeyProbeQuotaSnapshot | null {
  if (!account) return null
  const direct = (account as Partial<AccountPoolMapAccount>).api_key_probe_quota
  if (direct) return direct
  return normalizeProbeQuota((account.extra as Record<string, unknown> | undefined)?.apikey_probe_quota)
}

function normalizeProbeQuota(raw: unknown): APIKeyProbeQuotaSnapshot | null {
  if (!raw || typeof raw !== 'object') return null
  const payload = raw as Record<string, unknown>
  const provider = stringValue(payload.provider)
  if (!provider) return null
  return {
    provider,
    supported: booleanValue(payload.supported),
    source: stringValue(payload.source) || '',
    scope: stringValue(payload.scope),
    updated_at: stringValue(payload.updated_at) || '',
    status_code: numberValue(payload.status_code),
    model: stringValue(payload.model),
    requests_limit: stringValue(payload.requests_limit),
    requests_remaining: stringValue(payload.requests_remaining),
    requests_reset: stringValue(payload.requests_reset),
    tokens_limit: stringValue(payload.tokens_limit),
    tokens_remaining: stringValue(payload.tokens_remaining),
    tokens_reset: stringValue(payload.tokens_reset),
    input_tokens_limit: stringValue(payload.input_tokens_limit),
    input_tokens_remaining: stringValue(payload.input_tokens_remaining),
    input_tokens_reset: stringValue(payload.input_tokens_reset),
    output_tokens_limit: stringValue(payload.output_tokens_limit),
    output_tokens_remaining: stringValue(payload.output_tokens_remaining),
    output_tokens_reset: stringValue(payload.output_tokens_reset),
    retry_after: stringValue(payload.retry_after),
    rate_limit_policy: stringValue(payload.rate_limit_policy),
    quota_project: stringValue(payload.quota_project),
    balance: stringValue(payload.balance),
    credits_total: stringValue(payload.credits_total),
    credits_used: stringValue(payload.credits_used),
    credits_remaining: stringValue(payload.credits_remaining),
    currency: stringValue(payload.currency),
    note: stringValue(payload.note),
    has_rate_limit_header_signal: booleanValue(payload.has_rate_limit_header_signal),
    has_balance_signal: booleanValue(payload.has_balance_signal)
  }
}

function stringValue(value: unknown): string | undefined {
  if (value === null || value === undefined) return undefined
  const next = String(value).trim()
  return next || undefined
}

function booleanValue(value: unknown): boolean {
  if (typeof value === 'boolean') return value
  if (typeof value === 'string') return value.toLowerCase() === 'true'
  return false
}

function numberValue(value: unknown): number | undefined {
  if (value === null || value === undefined || value === '') return undefined
  const next = Number(value)
  return Number.isFinite(next) ? next : undefined
}

function quotaHasHeaderSignal(snapshot: APIKeyProbeQuotaSnapshot | null): boolean {
  if (!snapshot) return false
  return Boolean(
    snapshot.has_rate_limit_header_signal ||
    snapshot.requests_limit ||
    snapshot.requests_remaining ||
    snapshot.requests_reset ||
    snapshot.tokens_limit ||
    snapshot.tokens_remaining ||
    snapshot.tokens_reset ||
    snapshot.input_tokens_limit ||
    snapshot.input_tokens_remaining ||
    snapshot.input_tokens_reset ||
    snapshot.output_tokens_limit ||
    snapshot.output_tokens_remaining ||
    snapshot.output_tokens_reset ||
    snapshot.rate_limit_policy ||
    snapshot.retry_after
  )
}

function quotaHasBalanceSignal(snapshot: APIKeyProbeQuotaSnapshot | null): boolean {
  if (!snapshot) return false
  return Boolean(
    snapshot.has_balance_signal ||
    snapshot.balance ||
    snapshot.credits_remaining ||
    snapshot.credits_total ||
    snapshot.credits_used ||
    snapshot.currency
  )
}

function quotaHasAnySignal(snapshot: APIKeyProbeQuotaSnapshot | null): boolean {
  return quotaHasHeaderSignal(snapshot) || quotaHasBalanceSignal(snapshot)
}

function quotaBrief(account: Account | AccountPoolMapAccount | null | undefined): string {
  const snapshot = quotaSnapshot(account)
  if (!snapshot) return ''
  if (snapshot.balance) return `余额 ${snapshot.balance}`
  if (snapshot.credits_remaining) return `余额 $${snapshot.credits_remaining}`
  if (snapshot.tokens_remaining || snapshot.tokens_limit) {
    return `Token ${formatQuotaPair(snapshot.tokens_remaining, snapshot.tokens_limit)}`
  }
  if (snapshot.input_tokens_remaining || snapshot.input_tokens_limit || snapshot.output_tokens_remaining || snapshot.output_tokens_limit) {
    const input = formatQuotaPair(snapshot.input_tokens_remaining, snapshot.input_tokens_limit)
    const output = formatQuotaPair(snapshot.output_tokens_remaining, snapshot.output_tokens_limit)
    return `I/O ${input} / ${output}`
  }
  if (snapshot.requests_remaining || snapshot.requests_limit) {
    return `请求 ${formatQuotaPair(snapshot.requests_remaining, snapshot.requests_limit)}`
  }
  if (!snapshot.supported && snapshot.provider === 'gemini') return '项目级额度'
  if (!quotaHasAnySignal(snapshot)) return '未返回额度'
  return '已采集额度'
}

function quotaSourceLabel(snapshot: APIKeyProbeQuotaSnapshot | null): string {
  if (!snapshot) return '未采集'
  if (snapshot.source === 'headers') return '响应头采集'
  if (snapshot.source === 'balance_api') return '余额接口'
  if (snapshot.source === 'missing_balance') return '余额未返回'
  if (snapshot.source === 'missing_headers') return '响应头未返回'
  if (snapshot.source === 'project_quota') return '项目级额度'
  return snapshot.source || '探测记录'
}

function quotaScopeLabel(snapshot: APIKeyProbeQuotaSnapshot | null): string {
  if (!snapshot) return '未采集'
  if (snapshot.scope === 'account') return '账号余额'
  if (snapshot.scope === 'project') return 'Project'
  if (snapshot.scope === 'response_headers') return '响应头'
  return snapshot.scope || '上游'
}

function quotaCapabilityLabel(snapshot: APIKeyProbeQuotaSnapshot | null): string {
  if (!snapshot) return '尚未探测'
  if (quotaHasBalanceSignal(snapshot)) return '已查询余额'
  if (quotaHasHeaderSignal(snapshot)) return '已捕获响应头'
  if (snapshot.provider === 'gemini') return '项目级限额'
  return '无实时额度头'
}

function quotaEmptyTitle(snapshot: APIKeyProbeQuotaSnapshot | null): string {
  if (!snapshot) return '等待健康检查'
  if (snapshot.provider === 'gemini') return 'Gemini 未返回每 Key 剩余额度'
  if (snapshot.source === 'missing_balance') return '余额接口未返回明细'
  return '上游未返回可读额度'
}

function quotaGuidance(snapshot: APIKeyProbeQuotaSnapshot | null): string {
  if (!snapshot) return '运行一次 API Key 健康检查后，这里会显示最近一次真实上游探测结果。'
  if (quotaHasBalanceSignal(snapshot)) return '已通过上游余额接口获取可读余额，可用于账号池地图判断剩余额度。'
  if (snapshot.provider === 'gemini' && !quotaHasHeaderSignal(snapshot)) {
    return 'Gemini API 的限额主要按 Google Cloud Project 与模型维度管控。当前探测请求已记录，但本次响应没有可读的剩余请求或剩余 Token 响应头。'
  }
  if (!quotaHasHeaderSignal(snapshot)) {
    return '本次探测已完成，但上游没有返回 rate-limit header；后续健康检查如果捕获到响应头会自动补齐。'
  }
  return '已从最近一次真实上游响应中捕获 rate-limit header。'
}

function quotaSummaryText(snapshot: APIKeyProbeQuotaSnapshot | null): string {
  if (!snapshot) return '运行健康检查后显示上游返回的额度信号。'
  const source = quotaSourceLabel(snapshot)
  const scope = quotaScopeLabel(snapshot)
  if (quotaHasBalanceSignal(snapshot)) return `${source} · ${scope}`
  if (quotaHasHeaderSignal(snapshot)) return `${source} · ${scope} · ${snapshot.model || '未记录模型'}`
  if (snapshot.provider === 'gemini') return `Gemini ${scope} 级探测 · 无每 Key 剩余量`
  return `${source} · ${snapshot.model || '未记录模型'}`
}

function quotaPanelClass(snapshot: APIKeyProbeQuotaSnapshot | null): string {
  if (!snapshot) return 'quota-panel-missing'
  if (quotaHasBalanceSignal(snapshot)) return 'quota-panel-balance'
  if (quotaHasHeaderSignal(snapshot)) return 'quota-panel-live'
  if (snapshot.provider === 'gemini') return 'quota-panel-project'
  return 'quota-panel-muted'
}

function quotaUpdatedText(snapshot: APIKeyProbeQuotaSnapshot | null): string {
  if (!snapshot?.updated_at) return '未记录时间'
  return formatDate(snapshot.updated_at)
}

function formatQuotaPair(remaining?: string, limit?: string): string {
  const left = remaining || '-'
  const right = limit || '-'
  if (left === '-' && right === '-') return '-'
  return `${left} / ${right}`
}

function quotaResetText(snapshot: APIKeyProbeQuotaSnapshot | null): string {
  if (!snapshot) return '-'
  return snapshot.tokens_reset || snapshot.input_tokens_reset || snapshot.output_tokens_reset || snapshot.requests_reset || snapshot.retry_after || '-'
}

function quotaStatItems(snapshot: APIKeyProbeQuotaSnapshot | null): QuotaStatItem[] {
  if (!snapshot) return []
  const items: QuotaStatItem[] = []
  if (snapshot.balance || snapshot.credits_remaining) {
    items.push({
      label: '余额',
      value: snapshot.balance || `$${snapshot.credits_remaining}`,
      hint: snapshot.provider === 'openrouter' ? 'OpenRouter credits' : '账户余额'
    })
  }
  if (snapshot.credits_total || snapshot.credits_used) {
    items.push({
      label: 'Credits',
      value: formatQuotaPair(snapshot.credits_remaining, snapshot.credits_total),
      hint: snapshot.credits_used ? `已用 ${snapshot.credits_used}` : 'OpenRouter credit 窗口'
    })
  }
  if (snapshot.tokens_remaining || snapshot.tokens_limit) {
    items.push({
      label: 'Token',
      value: formatQuotaPair(snapshot.tokens_remaining, snapshot.tokens_limit),
      hint: snapshot.tokens_reset ? `重置 ${snapshot.tokens_reset}` : '总 Token 窗口'
    })
  }
  if (snapshot.requests_remaining || snapshot.requests_limit) {
    items.push({
      label: '请求',
      value: formatQuotaPair(snapshot.requests_remaining, snapshot.requests_limit),
      hint: snapshot.requests_reset ? `重置 ${snapshot.requests_reset}` : '请求窗口'
    })
  }
  if (snapshot.input_tokens_remaining || snapshot.input_tokens_limit) {
    items.push({
      label: '输入',
      value: formatQuotaPair(snapshot.input_tokens_remaining, snapshot.input_tokens_limit),
      hint: snapshot.input_tokens_reset ? `重置 ${snapshot.input_tokens_reset}` : '输入 Token'
    })
  }
  if (snapshot.output_tokens_remaining || snapshot.output_tokens_limit) {
    items.push({
      label: '输出',
      value: formatQuotaPair(snapshot.output_tokens_remaining, snapshot.output_tokens_limit),
      hint: snapshot.output_tokens_reset ? `重置 ${snapshot.output_tokens_reset}` : '输出 Token'
    })
  }
  return items
}

function quotaDetailRows(snapshot: APIKeyProbeQuotaSnapshot | null): DetailRow[] {
  if (!snapshot) return []
  const rows: DetailRow[] = [
    { label: '来源', value: quotaSourceLabel(snapshot) },
    { label: '范围', value: quotaScopeLabel(snapshot) }
  ]
  if (snapshot.model) rows.push({ label: '模型', value: snapshot.model })
  if (snapshot.status_code) rows.push({ label: '状态码', value: String(snapshot.status_code) })
  if (snapshot.quota_project) rows.push({ label: 'Quota Project', value: snapshot.quota_project })
  if (snapshot.balance) rows.push({ label: '余额', value: snapshot.balance })
  if (snapshot.currency) rows.push({ label: '币种', value: snapshot.currency })
  if (snapshot.credits_total) rows.push({ label: 'Credits 总量', value: snapshot.credits_total })
  if (snapshot.credits_used) rows.push({ label: 'Credits 已用', value: snapshot.credits_used })
  if (snapshot.credits_remaining) rows.push({ label: 'Credits 剩余', value: snapshot.credits_remaining })
  if (snapshot.rate_limit_policy) rows.push({ label: '策略', value: snapshot.rate_limit_policy })
  const reset = quotaResetText(snapshot)
  if (reset !== '-') rows.push({ label: '重置/重试', value: reset })
  return rows
}

function visiblePoolAccounts(pool: AccountPool): AccountPoolMapAccount[] {
  if (isPoolExpanded(pool) || pool.accounts.length <= POOL_PREVIEW_LIMIT) {
    return pool.accounts
  }
  return pool.accounts.slice(0, POOL_PREVIEW_LIMIT)
}

function poolHiddenCount(pool: AccountPool): number {
  if (isPoolExpanded(pool)) return 0
  return Math.max(0, pool.accounts.length - POOL_PREVIEW_LIMIT)
}

function isPoolExpanded(pool: AccountPool): boolean {
  return expandedPoolKeys.value.has(pool.key)
}

function togglePoolExpanded(pool: AccountPool) {
  const next = new Set(expandedPoolKeys.value)
  if (next.has(pool.key)) {
    next.delete(pool.key)
  } else {
    next.add(pool.key)
  }
  expandedPoolKeys.value = next
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

function clearHealthPollTimer() {
  if (healthPollTimer) {
    clearTimeout(healthPollTimer)
    healthPollTimer = null
  }
}

async function runHealthCheck() {
  if (checkingHealth.value) return
  checkingHealth.value = true
  clearHealthPollTimer()
  try {
    const started = await adminAPI.accounts.startAPIKeysHealthCheck()
    errorMessage.value = `健康检测已启动：${started.total} 个原始 Key 账号`
    await pollHealthCheckStatus()
  } catch (error) {
    checkingHealth.value = false
    errorMessage.value = normalizeError(error) || '健康检测启动失败。'
  }
}

async function pollHealthCheckStatus() {
  try {
    const status = await adminAPI.accounts.getAPIKeysHealthStatus()
    if (status.status === 'running') {
      const checked = status.result?.checked ?? 0
      const total = status.result?.total ?? 0
      errorMessage.value = total > 0 ? `健康检测中：${checked}/${total}` : '健康检测中...'
      healthPollTimer = setTimeout(() => {
        void pollHealthCheckStatus()
      }, 1500)
      return
    }

    checkingHealth.value = false
    clearHealthPollTimer()
    if (status.status === 'completed') {
      const result = status.result
      errorMessage.value = result
        ? `健康检测完成：有效 ${result.valid}，禁用 ${result.invalid_disabled}，失败 ${result.failed}`
        : '健康检测完成。'
      await refresh()
      return
    }
    if (status.status === 'failed') {
      errorMessage.value = status.error || '健康检测失败。'
    }
  } catch (error) {
    checkingHealth.value = false
    clearHealthPollTimer()
    errorMessage.value = normalizeError(error) || '健康检测状态获取失败。'
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
    active_sessions: 0,
    quota_signals: 0
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
  if (quotaHasAnySignal(quotaSnapshot(account))) summary.quota_signals += 1
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
  clearHealthPollTimer()
})
</script>

<style scoped>
.account-map-page {
  --account-map-border: rgb(226 232 240);
  --account-map-border-strong: rgb(203 213 225);
  --account-map-ink: rgb(15 23 42);
  --account-map-muted: rgb(100 116 139);
  --account-map-faint: rgb(148 163 184);
  --account-map-surface: rgb(255 255 255);
  --account-map-subtle: rgb(248 250 252);
  --account-map-soft: rgb(241 245 249);
  --account-map-accent: rgb(14 165 233);
  --account-map-accent-strong: rgb(37 99 235);
  --account-map-good: rgb(16 185 129);
  --account-map-warn: rgb(245 158 11);
  --account-map-danger: rgb(225 29 72);
  --account-map-shadow: 0 10px 30px rgb(15 23 42 / 0.05);
  max-width: 1480px;
  margin: 0 auto;
}

:global(.dark) .account-map-page {
  --account-map-border: rgb(51 65 85);
  --account-map-border-strong: rgb(71 85 105);
  --account-map-ink: rgb(248 250 252);
  --account-map-muted: rgb(148 163 184);
  --account-map-faint: rgb(100 116 139);
  --account-map-surface: rgb(15 23 42 / 0.94);
  --account-map-subtle: rgb(30 41 59 / 0.7);
  --account-map-soft: rgb(30 41 59 / 0.82);
  --account-map-shadow: none;
}

:global(.dark) .account-map-page :is(.account-map-toolbar, .account-map-stat-card, .account-map-filter-card, .account-map-empty-state, .account-pool-lane, .account-map-inspector) {
  border-color: rgb(51 65 85);
  background: var(--account-map-surface);
  color: var(--account-map-ink);
}

:global(.dark) .account-map-page :is(.account-map-stat-card p, .account-map-stat-card small, .account-pool-lane p, .account-node-meta, .account-map-meta) {
  color: var(--account-map-muted);
}

:global(.dark) .account-map-page :is(.account-map-stat-card strong, .account-pool-lane h2, .account-node p:first-child, .inspector-title) {
  color: var(--account-map-ink);
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
  border-radius: 1.25rem;
  padding: 1.15rem 1.25rem;
}

.account-map-toolbar-copy {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
}

.account-map-toolbar h1 {
  display: flex;
  align-items: center;
  gap: 0.55rem;
  color: var(--account-map-ink);
  font-size: 1.35rem;
  font-weight: 850;
  line-height: 1.2;
}

.account-map-eyebrow {
  margin-bottom: 0.3rem;
  color: var(--account-map-accent);
  font-size: 0.72rem;
  font-weight: 800;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.account-map-meta {
  margin-top: 0.55rem;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.45rem;
  color: var(--account-map-muted);
  font-size: 0.78rem;
}

.account-map-toolbar-actions {
  display: flex;
  flex: none;
  flex-wrap: wrap;
  align-items: center;
  justify-content: flex-end;
  gap: 0.65rem;
}

.account-map-summary-grid {
  align-items: stretch;
}

.account-map-stat-card {
  display: flex;
  min-height: 5.85rem;
  flex-direction: column;
  justify-content: space-between;
  border-radius: 1.15rem;
  padding: 0.95rem 1rem;
}

.account-map-stat-card p {
  color: var(--account-map-faint);
  font-size: 0.72rem;
  font-weight: 780;
  letter-spacing: 0.05em;
  text-transform: uppercase;
}

.account-map-stat-card strong {
  display: block;
  margin-top: 0.5rem;
  color: var(--account-map-ink);
  font-size: 1.55rem;
  font-weight: 900;
  line-height: 1;
}

.account-map-stat-card small {
  display: block;
  margin-top: 0.45rem;
  color: var(--account-map-muted);
  font-size: 0.76rem;
}

.account-map-stat-good {
  border-color: rgb(16 185 129 / 0.24);
}

.account-map-stat-warn {
  border-color: rgb(245 158 11 / 0.25);
}

.account-map-stat-accent {
  border-color: rgb(139 92 246 / 0.24);
}

.account-map-main-column {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.account-map-filter-card {
  border-radius: 1.15rem;
  padding: 1rem;
}

.account-map-filter-header {
  margin-bottom: 0.9rem;
}

.account-map-filter-header h2 {
  color: var(--account-map-ink);
  font-size: 0.98rem;
  font-weight: 850;
}

.account-map-filter-header p {
  margin-top: 0.25rem;
  color: var(--account-map-muted);
  font-size: 0.78rem;
}

.account-map-field {
  display: block;
}

.account-map-field > span {
  margin-bottom: 0.35rem;
  display: block;
  color: var(--account-map-muted);
  font-size: 0.72rem;
  font-weight: 720;
}

.account-map-control {
  width: 100%;
  border: 1px solid var(--account-map-border);
  border-radius: 0.85rem;
  background: var(--account-map-subtle);
  padding: 0.68rem 0.8rem;
  color: var(--account-map-ink);
  font-size: 0.88rem;
  outline: none;
  transition:
    border-color 0.15s ease,
    box-shadow 0.15s ease,
    background-color 0.15s ease;
}

.account-map-control::placeholder {
  color: var(--account-map-faint);
}

.account-map-control:focus {
  border-color: rgb(14 165 233 / 0.62);
  background: var(--account-map-surface);
  box-shadow: 0 0 0 4px rgb(14 165 233 / 0.11);
}

.account-map-segmented {
  display: inline-flex;
  border: 1px solid var(--account-map-border);
  border-radius: 0.9rem;
  background: var(--account-map-subtle);
  padding: 0.25rem;
}

.account-map-segmented button {
  border-radius: 0.68rem;
  padding: 0.48rem 0.75rem;
  color: var(--account-map-muted);
  font-size: 0.78rem;
  font-weight: 760;
  transition:
    background-color 0.15s ease,
    color 0.15s ease,
    box-shadow 0.15s ease;
}

.account-map-segmented button:hover {
  color: var(--account-map-ink);
}

.account-map-segmented button.active {
  background: var(--account-map-surface);
  color: var(--account-map-accent-strong);
  box-shadow: 0 1px 2px rgb(15 23 42 / 0.06);
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
  border-radius: 1.15rem;
  padding: 1rem;
}

.account-pool-lane-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  border-bottom: 1px solid var(--account-map-border);
  padding-bottom: 0.95rem;
}

.account-pool-platform {
  border-radius: 999px;
  background: var(--account-map-soft);
  padding: 0.34rem 0.62rem;
  color: var(--account-map-ink);
  font-size: 0.76rem;
  font-weight: 800;
}

.account-pool-lane h2 {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--account-map-ink);
  font-size: 1.02rem;
  font-weight: 850;
}

.account-pool-lane-header p {
  margin-top: 0.45rem;
  color: var(--account-map-muted);
  font-size: 0.82rem;
}

.account-pool-mini-metrics {
  display: grid;
  min-width: min(100%, 18rem);
  grid-template-columns: repeat(4, minmax(3.4rem, 1fr));
  gap: 0.45rem;
}

.account-pool-mini-metrics div {
  border: 1px solid var(--account-map-border);
  border-radius: 0.85rem;
  background: var(--account-map-subtle);
  padding: 0.55rem 0.65rem;
  text-align: right;
}

.account-pool-mini-metrics span {
  display: block;
  color: var(--account-map-faint);
  font-size: 0.68rem;
  font-weight: 760;
}

.account-pool-mini-metrics strong {
  display: block;
  margin-top: 0.15rem;
  color: var(--account-map-ink);
  font-size: 0.92rem;
  font-weight: 850;
}

.account-node-grid {
  margin-top: 1rem;
  display: grid;
  grid-template-columns: repeat(1, minmax(0, 1fr));
  gap: 0.75rem;
}

@media (min-width: 640px) {
  .account-node-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (min-width: 1024px) {
  .account-node-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

@media (min-width: 1536px) {
  .account-node-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

.account-pool-footer {
  margin-top: 0.95rem;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  border-top: 1px solid var(--account-map-border);
  padding-top: 0.85rem;
  color: var(--account-map-muted);
  font-size: 0.78rem;
}

.account-map-link-button {
  color: var(--account-map-accent-strong);
  font-size: 0.78rem;
  font-weight: 800;
}

.account-map-link-button:hover {
  color: var(--account-map-accent);
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
  background: var(--account-map-accent-strong);
  color: white;
  box-shadow: 0 1px 2px rgb(37 99 235 / 0.18);
}

.account-map-primary-button:hover {
  background: rgb(29 78 216);
}

.account-map-secondary-button {
  border: 1px solid var(--account-map-border);
  background: var(--account-map-surface);
  color: var(--account-map-ink);
}

.account-map-secondary-button:hover {
  border-color: rgb(14 165 233 / 0.45);
  color: var(--account-map-accent-strong);
}

.account-map-primary-button:disabled,
.account-map-secondary-button:disabled {
  cursor: not-allowed;
  opacity: 0.58;
}

:global(.dark) .account-map-secondary-button {
  border-color: var(--account-map-border);
  color: var(--account-map-ink);
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
  border-radius: 1.15rem;
}

.inspector-profile-card {
  position: relative;
  border-bottom: 1px solid var(--account-map-border);
  padding: 1.05rem;
}

.inspector-profile-card .inspector-close {
  position: absolute;
  right: 1rem;
  top: 1rem;
}

.inspector-profile-card > .inspector-kicker,
.inspector-profile-card > .inspector-title,
.inspector-profile-card > .inspector-profile-meta {
  padding-right: 2.75rem;
}

.inspector-profile-meta {
  margin-top: 0.65rem;
  display: flex;
  min-width: 0;
  flex-wrap: wrap;
  gap: 0.4rem;
}

.inspector-profile-meta span {
  border-radius: 999px;
  background: var(--account-map-soft);
  padding: 0.24rem 0.5rem;
  color: var(--account-map-muted);
  font-size: 0.7rem;
  font-weight: 750;
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
  border-bottom: 1px solid var(--account-map-border);
}

:global(.dark) .inspector-heading {
  border-bottom-color: var(--account-map-border);
}

.inspector-kicker {
  font-size: 0.7rem;
  font-weight: 800;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  color: var(--account-map-muted);
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
  color: var(--account-map-ink);
}

:global(.dark) .inspector-title {
  color: rgb(255 255 255);
}

.inspector-subtitle {
  margin-top: 0.3rem;
  font-size: 0.8125rem;
  color: var(--account-map-muted);
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
  margin-top: 0.95rem;
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

.inspector-status-pill {
  flex: none;
  border-radius: 999px;
  padding: 0.34rem 0.55rem;
  font-size: 0.72rem;
  font-weight: 800;
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
.inspector-overview-grid,
.inspector-snapshot-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.65rem;
  padding: 0 1rem 1rem;
}

.inspector-snapshot-grid {
  padding-top: 1rem;
}

.inspector-overview-grid {
  padding: 0;
}

.inspector-metric {
  border-radius: 0.9rem;
  border: 1px solid var(--account-map-border);
  background: var(--account-map-subtle);
  padding: 0.75rem;
}

.inspector-metric p:first-child {
  color: var(--account-map-muted) !important;
  font-size: 0.72rem;
  font-weight: 720;
}

.inspector-metric p:last-child {
  color: var(--account-map-ink) !important;
  font-size: 1rem;
  font-weight: 850;
}

:global(.dark) .inspector-metric {
  border-color: rgb(55 65 81 / 0.74);
  background: rgb(17 24 39 / 0.46);
}

.inspector-section {
  margin: 0 1rem 1rem;
  border-radius: 1rem;
  border: 1px solid var(--account-map-border);
  background: var(--account-map-subtle);
  padding: 0.9rem;
}

.inspector-section-grid {
  display: grid;
  gap: 0;
}

.inspector-quota-section {
  overflow: hidden;
  background: var(--account-map-subtle);
}

.quota-panel-live {
  border-color: rgb(14 165 233 / 0.28);
  background: linear-gradient(135deg, rgb(240 249 255), rgb(255 255 255));
}

.quota-panel-balance {
  border-color: rgb(16 185 129 / 0.3);
  background: linear-gradient(135deg, rgb(236 253 245), rgb(255 255 255));
}

.quota-panel-project {
  border-color: rgb(251 191 36 / 0.32);
  background: linear-gradient(135deg, rgb(255 251 235), rgb(255 255 255));
}

.quota-panel-muted,
.quota-panel-missing {
  border-color: var(--account-map-border);
}

.inspector-section-title {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  margin-bottom: 0.8rem;
}

.inspector-section-title h3 {
  font-size: 0.9rem;
  font-weight: 800;
  color: var(--account-map-ink);
}

.inspector-section-title p {
  margin-top: 0.24rem;
  color: var(--account-map-muted);
  font-size: 0.72rem;
  line-height: 1.35;
}

.inspector-section-title span {
  min-width: 0;
  max-width: 55%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 0.72rem;
  font-weight: 650;
  color: var(--account-map-muted);
}

:global(.dark) .inspector-section-title h3 {
  color: rgb(255 255 255);
}

:global(.dark) .inspector-section-title p,
:global(.dark) .inspector-section-title span {
  color: rgb(156 163 175);
}

.quota-stat-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.65rem;
}

.quota-stat-card {
  border-radius: 0.85rem;
  border: 1px solid rgb(186 230 253);
  background: rgb(255 255 255 / 0.74);
  padding: 0.72rem;
}

.quota-stat-card span,
.quota-empty-state span {
  display: block;
  color: var(--account-map-muted);
  font-size: 0.7rem;
  font-weight: 780;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.quota-stat-card strong,
.quota-empty-state strong {
  display: block;
  margin-top: 0.35rem;
  color: var(--account-map-ink);
  font-size: 0.98rem;
  font-weight: 900;
  line-height: 1.2;
}

.quota-stat-card small {
  display: block;
  margin-top: 0.25rem;
  color: var(--account-map-muted);
  font-size: 0.7rem;
}

.quota-empty-state {
  border-radius: 0.95rem;
  border: 1px dashed rgb(251 191 36 / 0.65);
  background: rgb(255 251 235 / 0.78);
  padding: 0.85rem;
}

.quota-empty-state p {
  margin-top: 0.4rem;
  color: rgb(146 64 14);
  font-size: 0.78rem;
  line-height: 1.5;
}

.quota-fact-list {
  margin-top: 0.85rem;
  border-top: 1px solid rgb(226 232 240 / 0.9);
  padding-top: 0.2rem;
}

:global(.dark) .inspector-section {
  border-color: rgb(55 65 81 / 0.7);
  background: rgb(17 24 39 / 0.36);
}

:global(.dark) .quota-panel-live {
  border-color: rgb(56 189 248 / 0.38);
  background: rgb(12 74 110 / 0.18);
}

:global(.dark) .quota-panel-balance {
  border-color: rgb(52 211 153 / 0.4);
  background: rgb(6 78 59 / 0.22);
}

:global(.dark) .quota-panel-project {
  border-color: rgb(245 158 11 / 0.38);
  background: rgb(120 53 15 / 0.18);
}

:global(.dark) .quota-stat-card {
  border-color: rgb(56 189 248 / 0.28);
  background: rgb(15 23 42 / 0.5);
}

:global(.dark) .quota-empty-state {
  border-color: rgb(245 158 11 / 0.45);
  background: rgb(120 53 15 / 0.24);
}

:global(.dark) .quota-empty-state p {
  color: rgb(253 230 138);
}

:global(.dark) .quota-fact-list {
  border-top-color: rgb(51 65 85);
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
  min-height: 5.45rem;
  overflow: hidden;
  border: 1px solid var(--account-map-border);
  border-radius: 0.95rem;
  padding: 0.82rem 0.86rem 0.82rem 0.95rem;
  background: var(--account-map-surface);
  color: var(--account-map-ink);
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
  opacity: 0.5;
}

.account-node > * {
  position: relative;
}

.account-node:hover {
  transform: translateY(-1px);
  background: var(--account-map-subtle);
  box-shadow: 0 10px 24px rgb(15 23 42 / 0.07);
}

:global(.dark) .account-node {
  border-color: rgb(51 65 85);
  background: rgb(15 23 42 / 0.72);
  color: var(--account-map-ink);
}

:global(.dark) .account-node:hover {
  background: rgb(30 41 59 / 0.82);
  box-shadow: 0 10px 24px rgb(0 0 0 / 0.18);
}

.account-node-selected {
  border-color: rgb(14 165 233 / 0.75) !important;
  background: rgb(240 249 255);
  box-shadow: 0 0 0 4px rgb(59 130 246 / 0.13), 0 12px 28px rgb(15 23 42 / 0.1);
}

.account-node-healthy {
  color: var(--account-map-good);
}

.account-node-degraded {
  color: var(--account-map-warn);
}

.account-node-limited {
  color: rgb(124 58 237);
}

.account-node-error {
  color: var(--account-map-danger);
}

.account-node-muted {
  color: var(--account-map-faint);
}

.account-node-blue {
  color: var(--account-map-accent-strong);
}

.account-node-blue-strong {
  border-color: rgb(14 165 233 / 0.42);
  background: rgb(240 249 255);
  color: var(--account-map-accent-strong);
}

:global(.dark) .account-node-healthy {
  color: rgb(167 243 208);
}

:global(.dark) .account-node-degraded {
  color: rgb(253 230 138);
}

:global(.dark) .account-node-limited {
  color: rgb(221 214 254);
}

:global(.dark) .account-node-error {
  color: rgb(254 205 211);
}

:global(.dark) .account-node-muted {
  color: rgb(156 163 175);
}

:global(.dark) .account-node-blue,
:global(.dark) .account-node-blue-strong {
  color: rgb(191 219 254);
}

.account-node p:first-child {
  color: var(--account-map-ink);
}

.account-node-subtitle {
  margin-top: 0.35rem;
  display: flex;
  min-width: 0;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.35rem;
  color: var(--account-map-muted);
  font-size: 0.72rem;
}

.account-node-status {
  border-radius: 999px;
  background: rgb(226 232 240);
  padding: 0.12rem 0.42rem;
  color: rgb(71 85 105);
  font-weight: 780;
}

.account-node-healthy .account-node-status {
  background: rgb(209 250 229);
  color: rgb(4 120 87);
}

.account-node-degraded .account-node-status {
  background: rgb(254 243 199);
  color: rgb(180 83 9);
}

.account-node-limited .account-node-status {
  background: rgb(237 233 254);
  color: rgb(109 40 217);
}

.account-node-error .account-node-status {
  background: rgb(255 228 230);
  color: rgb(190 18 60);
}

:global(.dark) .account-node-status {
  background: rgb(51 65 85);
  color: rgb(203 213 225);
}

:global(.dark) .account-node-healthy .account-node-status {
  background: rgb(6 78 59 / 0.55);
  color: rgb(167 243 208);
}

:global(.dark) .account-node-degraded .account-node-status {
  background: rgb(120 53 15 / 0.5);
  color: rgb(253 230 138);
}

:global(.dark) .account-node-limited .account-node-status {
  background: rgb(76 29 149 / 0.5);
  color: rgb(221 214 254);
}

:global(.dark) .account-node-error .account-node-status {
  background: rgb(136 19 55 / 0.5);
  color: rgb(254 205 211);
}

.account-node-meta {
  margin-top: 0.85rem;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.45rem;
  color: var(--account-map-muted);
  font-size: 0.72rem;
}

.account-node-quota {
  margin-top: 0.65rem;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  border-radius: 0.65rem;
  background: rgb(240 249 255);
  padding: 0.4rem 0.5rem;
  color: rgb(3 105 161);
  font-size: 0.72rem;
  font-weight: 760;
}

.account-node-quota small {
  flex: none;
  color: rgb(100 116 139);
  font-size: 0.66rem;
  font-weight: 650;
}

.account-node-quota span {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

:global(.dark) .account-node-quota {
  background: rgb(12 74 110 / 0.26);
  color: rgb(125 211 252);
}

:global(.dark) .account-node-quota small {
  color: rgb(148 163 184);
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
  width: 100%;
  max-width: min(1480px, 100%);
  margin-inline: auto;
  transform: none !important;
  --account-map-border: var(--anti-ink);
  --account-map-border-strong: var(--anti-ink);
  --account-map-ink: var(--anti-ink);
  --account-map-muted: #334155;
  --account-map-faint: #475569;
  --account-map-surface: var(--anti-paper);
  --account-map-subtle: #fffef2;
  --account-map-soft: #fff9b8;
  --account-map-shadow: none;
}

:global(.app-shell.app-shell-anti) .account-map-page :is(.account-map-toolbar, .account-map-filter-card, .account-map-empty-state, .account-pool-lane, .account-map-inspector, .inspector-profile-card) {
  border: 3px solid var(--anti-ink) !important;
  border-radius: 0.55rem !important;
  background: var(--anti-paper) !important;
  box-shadow: 6px 6px 0 var(--anti-ink) !important;
  color: var(--anti-ink) !important;
  transform: none !important;
}

:global(.app-shell.app-shell-anti) .account-map-page :is(.account-map-stat-card, .inspector-section, .inspector-metric, .inspector-attention-item, .inspector-alert, .inspector-good-state, .account-node-quota, .quota-stat-card, .quota-empty-state),
:global(.app-shell.app-shell-anti) .account-map-page .account-pool-mini-metrics div {
  border: 2px solid var(--anti-ink) !important;
  border-radius: 0.45rem !important;
  background: var(--anti-paper) !important;
  box-shadow: 3px 3px 0 var(--anti-ink) !important;
  color: var(--anti-ink) !important;
}

:global(.app-shell.app-shell-anti) .account-map-page .account-map-toolbar {
  background: var(--anti-yellow) !important;
  transform: none !important;
}

:global(.app-shell.app-shell-anti) .account-map-page .account-map-stat-card {
  min-height: 5.4rem;
}

:global(.app-shell.app-shell-anti) .account-map-eyebrow {
  display: inline-flex;
  border: 3px solid var(--anti-ink);
  background: var(--anti-red);
  color: var(--anti-paper) !important;
  padding: 0.2rem 0.35rem;
}

:global(.app-shell.app-shell-anti) .account-map-page :is(h1, h2, h3, p, label, strong, small) {
  color: var(--anti-ink) !important;
}

:global(.app-shell.app-shell-anti) .account-map-page :is(input, select) {
  border: 3px solid var(--anti-ink) !important;
  border-radius: 0 !important;
  background: var(--anti-paper) !important;
  box-shadow: 4px 4px 0 var(--anti-blue) !important;
  color: var(--anti-ink) !important;
}

:global(.app-shell.app-shell-anti) .account-map-page button:not(.account-node):not(.inspector-attention-item):not(.inspector-close) {
  border: 3px solid var(--anti-ink) !important;
  border-radius: 0.2rem !important;
  box-shadow: 4px 4px 0 var(--anti-ink) !important;
  color: var(--anti-ink) !important;
  font-weight: 950;
}

:global(.app-shell.app-shell-anti) .account-map-page button:not(.account-node):not(.inspector-attention-item):not(.inspector-close):hover {
  background: var(--anti-green) !important;
  transform: rotate(-1deg) translate(-1px, -1px);
}

:global(.app-shell.app-shell-anti) .account-map-page .account-node {
  border: 2px solid var(--anti-ink) !important;
  border-radius: 0.5rem !important;
  background: var(--anti-paper) !important;
  box-shadow: 3px 3px 0 var(--anti-ink) !important;
  transform: none !important;
}

:global(.app-shell.app-shell-anti) .account-map-page .account-node p:first-child,
:global(.app-shell.app-shell-anti) .account-map-page .account-node-meta,
:global(.app-shell.app-shell-anti) .account-map-page .account-node-subtitle,
:global(.app-shell.app-shell-anti) .account-map-page .account-node-quota,
:global(.app-shell.app-shell-anti) .account-map-page .account-node-quota small,
:global(.app-shell.app-shell-anti) .account-map-page .quota-empty-state p,
:global(.app-shell.app-shell-anti) .account-map-page .quota-stat-card small,
:global(.app-shell.app-shell-anti) .account-map-page .inspector-profile-meta span {
  color: var(--anti-ink) !important;
}

:global(.app-shell.app-shell-anti) .account-map-page .account-node-status {
  border: 2px solid var(--anti-ink);
  border-radius: 0.2rem;
  background: var(--anti-yellow) !important;
  color: var(--anti-ink) !important;
}

:global(.app-shell.app-shell-anti) .account-map-page .account-node-healthy {
  color: #007f3b;
}

:global(.app-shell.app-shell-anti) .account-map-page .account-node-degraded {
  color: #a16207;
}

:global(.app-shell.app-shell-anti) .account-map-page :is(.account-node-limited, .account-node-blue, .account-node-blue-strong) {
  color: var(--anti-blue);
}

:global(.app-shell.app-shell-anti) .account-map-page .account-node-error {
  color: var(--anti-red);
}

:global(.app-shell.app-shell-anti) .account-map-page .account-node-muted {
  color: var(--anti-muted);
}

:global(.app-shell.app-shell-anti) .account-map-page .account-node:hover,
:global(.app-shell.app-shell-anti) .account-map-page .account-node-selected {
  background: #f6ff57 !important;
  box-shadow: 5px 5px 0 var(--anti-red) !important;
  transform: translate(-1px, -1px) !important;
}

:global(.app-shell.app-shell-anti) .account-map-page .inspector-status-card {
  border: 3px solid var(--anti-ink) !important;
  border-radius: 0.5rem !important;
  background: var(--anti-yellow) !important;
  box-shadow: 4px 4px 0 var(--anti-blue) !important;
  color: var(--anti-ink) !important;
}

:global(.app-shell.app-shell-anti) .account-map-page .inspector-kicker {
  display: inline-flex;
  border: 3px solid var(--anti-ink);
  background: var(--anti-red);
  color: var(--anti-paper) !important;
  padding: 0.2rem 0.35rem;
}

:global(.dark .app-shell.app-shell-anti) .account-map-page {
  --anti-paper: #0b1020;
  --anti-ink: #f8fafc;
  --anti-muted: #94a3b8;
  --account-map-border: #e2e8f0;
  --account-map-border-strong: #f8fafc;
  --account-map-ink: #f8fafc;
  --account-map-muted: #cbd5e1;
  --account-map-faint: #94a3b8;
  --account-map-surface: #0f172a;
  --account-map-subtle: #111827;
  --account-map-soft: #1e293b;
}

:global(.dark .app-shell.app-shell-anti) .account-map-page :is(.account-map-toolbar, .account-map-filter-card, .account-map-empty-state, .account-pool-lane, .account-map-inspector, .inspector-profile-card) {
  background: linear-gradient(135deg, #101827, #0b1020) !important;
  box-shadow: 6px 6px 0 #334155 !important;
}

:global(.dark .app-shell.app-shell-anti) .account-map-page :is(.account-map-stat-card, .inspector-section, .inspector-metric, .inspector-attention-item, .inspector-alert, .inspector-good-state, .account-node-quota, .quota-stat-card, .quota-empty-state),
:global(.dark .app-shell.app-shell-anti) .account-map-page .account-pool-mini-metrics div {
  background: #111827 !important;
  box-shadow: 3px 3px 0 #334155 !important;
}

:global(.dark .app-shell.app-shell-anti) .account-map-page .account-map-toolbar {
  background:
    linear-gradient(90deg, rgb(250 204 21 / 0.22), transparent 46%),
    linear-gradient(135deg, #111827, #0b1020) !important;
}

:global(.dark .app-shell.app-shell-anti) .account-map-page :is(.account-map-eyebrow, .inspector-kicker) {
  border-color: #f8fafc;
  background: #ef4444;
  color: #ffffff !important;
}

:global(.dark .app-shell.app-shell-anti) .account-map-page :is(input, select) {
  background: #0f172a !important;
  box-shadow: 4px 4px 0 #2563eb !important;
  color: #f8fafc !important;
}

:global(.dark .app-shell.app-shell-anti) .account-map-page :is(input, select)::placeholder {
  color: #94a3b8 !important;
}

:global(.dark .app-shell.app-shell-anti) .account-map-page .account-map-secondary-button,
:global(.dark .app-shell.app-shell-anti) .account-map-page .account-map-segmented button,
:global(.dark .app-shell.app-shell-anti) .account-map-page .inspector-action {
  background: #0f172a !important;
  color: #f8fafc !important;
}

:global(.dark .app-shell.app-shell-anti) .account-map-page .account-map-primary-button,
:global(.dark .app-shell.app-shell-anti) .account-map-page .inspector-action-primary {
  background: #2563eb !important;
  color: #ffffff !important;
}

:global(.dark .app-shell.app-shell-anti) .account-map-page .account-map-segmented button.active {
  background: #facc15 !important;
  color: #111827 !important;
}

:global(.dark .app-shell.app-shell-anti) .account-map-page .account-node {
  background: #101827 !important;
  box-shadow: 3px 3px 0 #334155 !important;
}

:global(.dark .app-shell.app-shell-anti) .account-map-page .account-node:hover,
:global(.dark .app-shell.app-shell-anti) .account-map-page .account-node-selected {
  background: #172554 !important;
  box-shadow: 5px 5px 0 #facc15 !important;
}

:global(.dark .app-shell.app-shell-anti) .account-map-page .account-node-status {
  border-color: #f8fafc;
  background: #facc15 !important;
  color: #111827 !important;
}

:global(.dark .app-shell.app-shell-anti) .account-map-page .inspector-status-card {
  background: #111827 !important;
  box-shadow: 4px 4px 0 #2563eb !important;
}

@media (max-width: 1279px) {
  .account-map-inspector {
    position: static;
  }
}

@media (max-width: 768px) {
  .account-map-toolbar-copy,
  .account-pool-lane-header {
    align-items: stretch;
    flex-direction: column;
  }

  .account-map-toolbar-actions {
    justify-content: flex-start;
  }

  .account-pool-mini-metrics {
    width: 100%;
    min-width: 0;
  }

  .account-map-segmented {
    width: 100%;
  }

  .account-map-segmented button {
    flex: 1;
  }
}

@media (max-width: 640px) {
  .account-pool-mini-metrics {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

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

<style>
/* Keep account-map theme selectors unscoped: Vue scoped :global() drops the local tail. */
.dark .account-map-page {
  --account-map-border: rgb(51 65 85);
  --account-map-border-strong: rgb(71 85 105);
  --account-map-ink: rgb(248 250 252);
  --account-map-muted: rgb(148 163 184);
  --account-map-faint: rgb(100 116 139);
  --account-map-surface: rgb(15 23 42 / 0.94);
  --account-map-subtle: rgb(30 41 59 / 0.7);
  --account-map-soft: rgb(30 41 59 / 0.82);
  --account-map-shadow: none;
}

.dark .account-map-page :is(.account-map-toolbar, .account-map-stat-card, .account-map-filter-card, .account-map-empty-state, .account-pool-lane, .account-map-inspector) {
  border-color: rgb(51 65 85);
  background: var(--account-map-surface);
  color: var(--account-map-ink);
}

.dark .account-map-page :is(.account-map-stat-card p, .account-map-stat-card small, .account-pool-lane p, .account-node-meta, .account-map-meta, .account-map-field > span, .inspector-subtitle, .inspector-section-title p, .inspector-section-title span) {
  color: var(--account-map-muted);
}

.dark .account-map-page :is(.account-map-stat-card strong, .account-pool-lane h2, .account-node p:first-child, .account-map-filter-header h2, .inspector-title, .inspector-section-title h3, .inspector-metric p:last-child) {
  color: var(--account-map-ink);
}

.dark .account-map-page :is(.account-map-control, .account-map-segmented, .account-pool-mini-metrics div, .inspector-metric, .inspector-section, .inspector-attention-item) {
  border-color: rgb(55 65 81 / 0.74);
  background: var(--account-map-subtle);
  color: var(--account-map-ink);
}

.dark .account-map-page .account-map-control:focus {
  background: rgb(15 23 42 / 0.98);
}

.dark .account-map-page .account-map-secondary-button {
  border-color: var(--account-map-border);
  background: rgb(15 23 42 / 0.92);
  color: var(--account-map-ink);
}

.dark .account-map-page .account-map-secondary-button:hover {
  border-color: rgb(59 130 246 / 0.7);
  color: rgb(147 197 253);
}

.dark .account-map-page .account-map-segmented button.active {
  background: rgb(15 23 42 / 0.96);
  color: rgb(147 197 253);
}

.dark .account-map-page .account-node {
  border-color: rgb(51 65 85);
  background: rgb(15 23 42 / 0.72);
  color: var(--account-map-ink);
}

.dark .account-map-page .account-node:hover {
  background: rgb(30 41 59 / 0.82);
  box-shadow: 0 10px 24px rgb(0 0 0 / 0.18);
}

.dark .account-map-page .account-node-status {
  background: rgb(51 65 85);
  color: rgb(203 213 225);
}

.dark .account-map-page .account-node-healthy .account-node-status {
  background: rgb(6 78 59 / 0.55);
  color: rgb(167 243 208);
}

.dark .account-map-page .account-node-degraded .account-node-status {
  background: rgb(120 53 15 / 0.5);
  color: rgb(253 230 138);
}

.dark .account-map-page .account-node-limited .account-node-status {
  background: rgb(76 29 149 / 0.5);
  color: rgb(221 214 254);
}

.dark .account-map-page .account-node-error .account-node-status {
  background: rgb(136 19 55 / 0.5);
  color: rgb(254 205 211);
}

.dark .account-map-page .account-node-quota {
  background: rgb(12 74 110 / 0.26);
  color: rgb(125 211 252);
}

.dark .account-map-page .account-node-quota small {
  color: rgb(148 163 184);
}

.dark .account-map-page :is(.inspector-heading, .inspector-empty-header, .account-pool-lane-header, .quota-fact-list, .inspector-row) {
  border-color: rgb(51 65 85);
}

.dark .account-map-page .inspector-status-card {
  border-color: rgb(55 65 81 / 0.82);
  background: rgb(31 41 55 / 0.56);
}

.dark .account-map-page .inspector-status-card strong {
  color: rgb(255 255 255);
}

.dark .account-map-page :is(.inspector-status-card p, .inspector-status-label, .inspector-kicker, .inspector-metric p:first-child) {
  color: rgb(156 163 175);
}

.dark .account-map-page .inspector-status-healthy {
  border-color: rgb(16 185 129 / 0.45);
  background: rgb(6 78 59 / 0.22);
}

.dark .account-map-page .inspector-status-degraded {
  border-color: rgb(245 158 11 / 0.5);
  background: rgb(120 53 15 / 0.25);
}

.dark .account-map-page .inspector-status-rate_limited {
  border-color: rgb(139 92 246 / 0.5);
  background: rgb(76 29 149 / 0.25);
}

.dark .account-map-page .inspector-status-error {
  border-color: rgb(244 63 94 / 0.5);
  background: rgb(136 19 55 / 0.25);
}

.dark .account-map-page .inspector-status-disabled {
  border-color: rgb(55 65 81);
  background: rgb(31 41 55 / 0.72);
}

.dark .account-map-page .quota-panel-live {
  border-color: rgb(56 189 248 / 0.38);
  background: rgb(12 74 110 / 0.18);
}

.dark .account-map-page .quota-panel-balance {
  border-color: rgb(52 211 153 / 0.4);
  background: rgb(6 78 59 / 0.22);
}

.dark .account-map-page .quota-panel-project {
  border-color: rgb(245 158 11 / 0.38);
  background: rgb(120 53 15 / 0.18);
}

.dark .account-map-page .quota-stat-card {
  border-color: rgb(56 189 248 / 0.28);
  background: rgb(15 23 42 / 0.5);
}

.dark .account-map-page .quota-empty-state {
  border-color: rgb(245 158 11 / 0.45);
  background: rgb(120 53 15 / 0.24);
}

.dark .account-map-page .quota-empty-state p {
  color: rgb(253 230 138);
}

.dark .account-map-page .inspector-alert {
  border-color: rgb(245 158 11 / 0.38);
  background: rgb(245 158 11 / 0.12);
  color: rgb(253 230 138);
}

.dark .account-map-page .inspector-good-state {
  border-color: rgb(16 185 129 / 0.35);
  background: rgb(16 185 129 / 0.12);
  color: rgb(167 243 208);
}

.dark .account-map-page .inspector-action {
  border-color: rgb(55 65 81);
  color: rgb(209 213 219);
}

.dark .account-map-page .inspector-action:hover {
  border-color: rgb(59 130 246 / 0.7);
  color: rgb(147 197 253);
}

.app-shell.app-shell-anti {
  display: block !important;
  width: auto !important;
  min-height: 100vh !important;
  max-width: none !important;
  margin: 0 !important;
  border: 0 !important;
  border-radius: 0 !important;
  padding: 0 !important;
  box-shadow: none !important;
  transform: none !important;
  background:
    radial-gradient(circle at 10% 10%, rgb(255 0 0 / 0.16) 0 7rem, transparent 7.2rem),
    radial-gradient(circle at 88% 8%, rgb(0 255 0 / 0.16) 0 7.5rem, transparent 7.7rem),
    repeating-linear-gradient(135deg, rgb(5 5 5 / 0.055) 0 10px, transparent 10px 24px),
    linear-gradient(135deg, #fff038 0%, #ffffff 52%, #eaffff 100%) !important;
}

.dark .app-shell.app-shell-anti {
  background:
    radial-gradient(circle at 10% 10%, rgb(255 0 0 / 0.22) 0 7rem, transparent 7.2rem),
    radial-gradient(circle at 88% 8%, rgb(0 255 0 / 0.20) 0 7.5rem, transparent 7.7rem),
    repeating-linear-gradient(135deg, rgb(255 255 255 / 0.08) 0 10px, transparent 10px 24px),
    linear-gradient(135deg, #160016 0%, #2d0a3d 52%, #3b3300 100%) !important;
}

.app-shell.app-shell-anti .account-map-page {
  color: var(--anti-ink);
  width: 100%;
  max-width: min(1480px, 100%);
  margin-inline: auto;
  transform: none !important;
  --account-map-border: var(--anti-ink);
  --account-map-border-strong: var(--anti-ink);
  --account-map-ink: var(--anti-ink);
  --account-map-muted: #334155;
  --account-map-faint: #475569;
  --account-map-surface: var(--anti-paper);
  --account-map-subtle: #fffef2;
  --account-map-soft: #fff9b8;
  --account-map-shadow: none;
}

.app-shell.app-shell-anti .account-map-page :is(.account-map-toolbar, .account-map-filter-card, .account-map-empty-state, .account-pool-lane, .account-map-inspector, .inspector-profile-card) {
  border: 3px solid var(--anti-ink) !important;
  border-radius: 0.55rem !important;
  background: var(--anti-paper) !important;
  box-shadow: 6px 6px 0 var(--anti-ink) !important;
  color: var(--anti-ink) !important;
  transform: none !important;
}

.app-shell.app-shell-anti .account-map-page :is(.account-map-stat-card, .inspector-section, .inspector-metric, .inspector-attention-item, .inspector-alert, .inspector-good-state, .account-node-quota, .quota-stat-card, .quota-empty-state),
.app-shell.app-shell-anti .account-map-page .account-pool-mini-metrics div {
  border: 2px solid var(--anti-ink) !important;
  border-radius: 0.45rem !important;
  background: var(--anti-paper) !important;
  box-shadow: 3px 3px 0 var(--anti-ink) !important;
  color: var(--anti-ink) !important;
}

.app-shell.app-shell-anti .account-map-page .account-map-toolbar {
  background: var(--anti-yellow) !important;
}

.app-shell.app-shell-anti .account-map-page :is(.account-map-eyebrow, .inspector-kicker) {
  display: inline-flex;
  border: 3px solid var(--anti-ink);
  background: var(--anti-red);
  color: var(--anti-paper) !important;
  padding: 0.2rem 0.35rem;
}

.app-shell.app-shell-anti .account-map-page :is(h1, h2, h3, p, label, strong, small),
.app-shell.app-shell-anti .account-map-page :is(.account-node-meta, .account-node-subtitle, .inspector-profile-meta span) {
  color: var(--anti-ink) !important;
}

.app-shell.app-shell-anti .account-map-page :is(input, select) {
  border: 3px solid var(--anti-ink) !important;
  border-radius: 0 !important;
  background: var(--anti-paper) !important;
  box-shadow: 4px 4px 0 var(--anti-blue) !important;
  color: var(--anti-ink) !important;
}

.app-shell.app-shell-anti .account-map-page button:not(.account-node):not(.inspector-attention-item):not(.inspector-close) {
  border: 3px solid var(--anti-ink) !important;
  border-radius: 0.2rem !important;
  box-shadow: 4px 4px 0 var(--anti-ink) !important;
  color: var(--anti-ink) !important;
  font-weight: 950;
}

.app-shell.app-shell-anti .account-map-page button:not(.account-node):not(.inspector-attention-item):not(.inspector-close):hover {
  background: var(--anti-green) !important;
  transform: rotate(-1deg) translate(-1px, -1px);
}

.app-shell.app-shell-anti .account-map-page .account-node {
  border: 2px solid var(--anti-ink) !important;
  border-radius: 0.5rem !important;
  background: var(--anti-paper) !important;
  box-shadow: 3px 3px 0 var(--anti-ink) !important;
  transform: none !important;
}

.app-shell.app-shell-anti .account-map-page .account-node-status {
  border: 2px solid var(--anti-ink);
  border-radius: 0.2rem;
  background: var(--anti-yellow) !important;
  color: var(--anti-ink) !important;
}

.app-shell.app-shell-anti .account-map-page :is(.account-node:hover, .account-node-selected) {
  background: #f6ff57 !important;
  box-shadow: 5px 5px 0 var(--anti-red) !important;
  transform: translate(-1px, -1px) !important;
}

.app-shell.app-shell-anti .account-map-page .inspector-status-card {
  border: 3px solid var(--anti-ink) !important;
  border-radius: 0.5rem !important;
  background: var(--anti-yellow) !important;
  box-shadow: 4px 4px 0 var(--anti-blue) !important;
  color: var(--anti-ink) !important;
}

.dark .app-shell.app-shell-anti .account-map-page {
  --anti-paper: #0b1020;
  --anti-ink: #f8fafc;
  --anti-muted: #94a3b8;
  --account-map-border: #e2e8f0;
  --account-map-border-strong: #f8fafc;
  --account-map-ink: #f8fafc;
  --account-map-muted: #cbd5e1;
  --account-map-faint: #94a3b8;
  --account-map-surface: #0f172a;
  --account-map-subtle: #111827;
  --account-map-soft: #1e293b;
}

.dark .app-shell.app-shell-anti .account-map-page :is(.account-map-toolbar, .account-map-filter-card, .account-map-empty-state, .account-pool-lane, .account-map-inspector, .inspector-profile-card) {
  background: linear-gradient(135deg, #101827, #0b1020) !important;
  box-shadow: 6px 6px 0 #334155 !important;
}

.dark .app-shell.app-shell-anti .account-map-page :is(.account-map-stat-card, .inspector-section, .inspector-metric, .inspector-attention-item, .inspector-alert, .inspector-good-state, .account-node-quota, .quota-stat-card, .quota-empty-state),
.dark .app-shell.app-shell-anti .account-map-page .account-pool-mini-metrics div {
  background: #111827 !important;
  box-shadow: 3px 3px 0 #334155 !important;
}

.dark .app-shell.app-shell-anti .account-map-page .account-map-toolbar {
  background:
    linear-gradient(90deg, rgb(250 204 21 / 0.22), transparent 46%),
    linear-gradient(135deg, #111827, #0b1020) !important;
}

.dark .app-shell.app-shell-anti .account-map-page .account-node {
  background: #101827 !important;
  box-shadow: 3px 3px 0 #334155 !important;
}

.dark .app-shell.app-shell-anti .account-map-page :is(.account-node:hover, .account-node-selected) {
  background: #172554 !important;
  box-shadow: 5px 5px 0 #facc15 !important;
}

.dark .app-shell.app-shell-anti .account-map-page .account-node-status {
  border-color: #f8fafc;
  background: #facc15 !important;
  color: #111827 !important;
}

.dark .app-shell.app-shell-anti .account-map-page .account-map-segmented button.active {
  background: #facc15 !important;
  color: #111827 !important;
}
</style>
