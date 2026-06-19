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
          <div v-if="item.metrics?.length" class="account-map-stat-metrics">
            <span v-for="metric in item.metrics" :key="metric.label">
              <b>{{ metric.value }}</b>
              {{ metric.label }}
            </span>
          </div>
        </div>
      </section>

      <section class="account-map-workspace">
        <div class="account-map-main-column">
        <div class="account-map-filter-card">
          <div class="account-map-filter-header">
            <div>
              <h2>筛选账号池</h2>
              <p>先缩小范围，再展开查看具体账号，避免异常账号一次性淹没页面。</p>
            </div>
          </div>
          <div class="account-map-filter-grid">
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
          <div
            v-if="usingFallback"
            class="mt-3 rounded-xl border border-sky-200 bg-sky-50 px-3 py-2 text-xs text-sky-700 dark:border-sky-500/30 dark:bg-sky-500/10 dark:text-sky-200"
          >
            当前使用账号列表聚合数据；账号池地图 API 恢复后会自动回到服务端聚合结果。
          </div>
        </div>

        <div v-if="loading && !accounts.length" class="grid gap-3">
          <div v-for="idx in 3" :key="idx" class="h-32 animate-pulse rounded-2xl bg-gray-100 dark:bg-dark-800"></div>
        </div>

        <div v-else-if="!visiblePools.length" class="account-map-empty-state">
          <p class="text-base font-semibold text-gray-900 dark:text-white">没有匹配的账号池</p>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">调整筛选条件，或者先在账号管理中导入/创建上游账号。</p>
        </div>

        <div v-else class="account-pool-card-grid">
          <article
            v-for="pool in visiblePools"
            :key="pool.key"
            role="button"
            tabindex="0"
            class="account-pool-card"
            :class="[
              `account-pool-card-${poolStatusTone(pool)}`,
              selectedPoolForDetail?.key === pool.key ? 'account-pool-card-active' : ''
            ]"
            @click="selectPool(pool)"
            @keydown.enter.prevent="selectPool(pool)"
            @keydown.space.prevent="selectPool(pool)"
          >
            <div class="account-pool-card-head">
              <div class="account-pool-card-title">
                <span class="account-pool-platform">{{ platformLabel(pool.platform) }}</span>
                <h2>{{ accountTypeLabel(pool.type) }}</h2>
              </div>
              <span class="account-pool-status-chip" :class="`account-pool-status-chip-${poolStatusTone(pool)}`">
                {{ statusKindLabel(poolStatusTone(pool)) }}
              </span>
            </div>
            <div class="account-pool-card-main">
              <div class="account-pool-card-count">
                <strong>{{ pool.accounts.length }}</strong>
                <span>账号</span>
              </div>
              <p>{{ poolSummaryText(pool) }}</p>
            </div>
            <div class="account-pool-card-metrics">
              <span><b>{{ poolHealthyCount(pool) }}</b>健康</span>
              <span><b>{{ poolAttentionCount(pool) }}</b>关注</span>
              <span><b>{{ poolRPM(pool) }}</b>RPM</span>
              <span><b>{{ poolConcurrency(pool) }}</b>并发</span>
              <span><b>{{ poolFingerprintCount(pool) }}</b>TLS</span>
              <span><b>{{ poolQuotaSignalCount(pool) }}</b>额度</span>
            </div>
            <div class="account-pool-card-signal">
              <span>{{ poolQuotaSummary(pool) }}</span>
              <strong :title="poolPrimaryIssue(pool)">{{ poolPrimaryIssue(pool) }}</strong>
            </div>
            <div class="account-pool-card-preview">
              <span
                v-for="account in poolPreviewAccounts(pool)"
                :key="account.id"
                class="account-pool-avatar"
                :class="nodeClass(account)"
                :title="account.name"
              >
                {{ accountInitial(account) }}
              </span>
              <span v-if="pool.accounts.length > poolPreviewAccounts(pool).length" class="account-pool-avatar account-pool-avatar-more">
                +{{ pool.accounts.length - poolPreviewAccounts(pool).length }}
              </span>
            </div>
            <div class="account-pool-card-foot">
              <span>{{ poolTrafficSummary(pool) }}</span>
              <button type="button" class="account-pool-detail-button" @click.stop="selectPool(pool)">
                查看详情
              </button>
            </div>
          </article>
        </div>
        </div>
      </section>

      <div
        v-if="detailDrawerOpen"
        class="account-map-drawer-layer"
        role="presentation"
        @click.self="closeDrawer"
      >
        <aside
          class="account-map-drawer"
          role="dialog"
          aria-modal="true"
          :aria-label="selectedAccount ? '账号详情' : '账号池详情'"
        >
        <section class="account-map-inspector" aria-live="polite">
          <template v-if="selectedAccount">
            <!-- Drawer sticky header -->
            <div class="inspector-drawer-header">
              <div class="inspector-drawer-header-left">
                <span class="inspector-kicker">账号详情</span>
                <h2 class="inspector-title">{{ selectedAccount.name }}</h2>
              </div>
              <button type="button" class="inspector-close" aria-label="关闭账号详情" @click="closeDrawer">
                <svg aria-hidden="true" focusable="false" width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
                  <path d="M1 1l12 12M13 1L1 13"/>
                </svg>
              </button>
            </div>
            <div class="inspector-profile-card">
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

          <template v-else-if="selectedPoolForDetail">
            <!-- Drawer sticky header -->
            <div class="inspector-drawer-header">
              <div class="inspector-drawer-header-left">
                <span class="inspector-kicker">池详情</span>
                <h2 class="inspector-title">{{ platformLabel(selectedPoolForDetail.platform) }} · {{ accountTypeLabel(selectedPoolForDetail.type) }}</h2>
              </div>
              <button type="button" class="inspector-close" aria-label="取消选择账号池" @click="clearPoolSelection">
                <svg aria-hidden="true" focusable="false" width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
                  <path d="M1 1l12 12M13 1L1 13"/>
                </svg>
              </button>
            </div>
            <div class="inspector-profile-card inspector-pool-card">
              <div class="inspector-profile-meta">
                <span>{{ selectedPoolForDetail.accounts.length }} 个账号</span>
                <span>健康 {{ poolHealthyCount(selectedPoolForDetail) }}</span>
                <span>需关注 {{ poolAttentionCount(selectedPoolForDetail) }}</span>
              </div>
            </div>

            <div class="inspector-snapshot-grid">
              <MetricTile label="RPM" :value="String(poolRPM(selectedPoolForDetail))" />
              <MetricTile label="并发" :value="String(poolConcurrency(selectedPoolForDetail))" />
              <MetricTile label="TLS 覆盖" :value="poolFingerprintCount(selectedPoolForDetail)" />
              <MetricTile label="额度信号" :value="String(poolQuotaSignalCount(selectedPoolForDetail))" />
            </div>

            <div class="inspector-section">
              <div class="inspector-section-title">
                <div>
                  <h3>池内账号</h3>
                  <p>{{ poolSummaryText(selectedPoolForDetail) }}</p>
                </div>
                <button
                  v-if="poolHiddenCount(selectedPoolForDetail) > 0 || isPoolExpanded(selectedPoolForDetail)"
                  type="button"
                  class="account-map-link-button"
                  @click="togglePoolExpanded(selectedPoolForDetail)"
                >
                  {{ isPoolExpanded(selectedPoolForDetail) ? '收起' : `展开 ${poolHiddenCount(selectedPoolForDetail)} 个` }}
                </button>
              </div>

              <div class="inspector-account-list">
                <button
                  v-for="account in visiblePoolAccounts(selectedPoolForDetail)"
                  :key="account.id"
                  type="button"
                  class="inspector-account-row"
                  :class="nodeClass(account)"
                  @click="selectAccount(account)"
                >
                  <span class="inspector-account-dot"></span>
                  <span class="inspector-account-main">
                    <b>{{ account.name }}</b>
                    <small>{{ statusLabel(account) }} · RPM {{ account.current_rpm ?? 0 }} · 并发 {{ account.current_concurrency ?? 0 }}</small>
                  </span>
                  <span class="inspector-account-quota">{{ quotaBrief(account) || '未探测' }}</span>
                  <svg class="inspector-account-chevron" aria-hidden="true" focusable="false" width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
                    <path d="M4 2l4 4-4 4"/>
                  </svg>
                </button>
              </div>
            </div>
          </template>

          <template v-else>
            <div class="inspector-drawer-header inspector-drawer-header-empty">
              <div class="inspector-drawer-header-left">
                <span class="inspector-kicker">详情栏</span>
                <h2 class="inspector-title">选择账号池查看细节</h2>
              </div>
            </div>
            <div class="inspector-empty-header">
              <p class="inspector-subtitle">
                左侧卡片是当前筛选范围内的调度入口，点击池子后在这里聚焦账号列表。
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

              <div v-if="platformHealthRows.length" class="account-map-platform-list">
                <div
                  v-for="row in platformHealthRows"
                  :key="row.platform"
                  class="account-map-platform-row"
                >
                  <div class="account-map-platform-row-head">
                    <span>{{ platformLabel(row.platform) }}</span>
                    <strong>{{ row.healthy }}/{{ row.total }}</strong>
                  </div>
                  <div class="account-map-platform-track">
                    <span :style="{ width: `${row.healthyPercent}%` }"></span>
                  </div>
                  <p>{{ row.attention }} 需关注 · {{ row.quota }} 额度信号</p>
                </div>
              </div>
            </div>
          </template>
        </section>
        </aside>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { adminAPI } from '@/api/admin'
import AppLayout from '@/components/layout/AppLayout.vue'
import { useAccountPoolMapData } from '@/composables/useAccountPoolMapData'
import type { Account } from '@/types'
import type {
  AccountPoolMapAccount,
  AccountPoolMapPool,
  AccountPoolMapStatusKind,
  AccountPoolMapSummary,
  APIKeyProbeQuotaSnapshot
} from '@/api/admin/accounts'

type ViewMode = 'status' | 'traffic' | 'error'
type AccountStatusKind = AccountPoolMapStatusKind
type AccountPool = AccountPoolMapPool
type DetailRow = { label: string; value: string }
type QuotaStatItem = { label: string; value: string; hint: string }
type PlatformHealthRow = {
  platform: string
  total: number
  healthy: number
  attention: number
  quota: number
  healthyPercent: number
}

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
const selectedAccountId = ref<number | null>(null)
const selectedPoolKey = ref<string | null>(null)
const expandedPoolKeys = ref<Set<string>>(new Set())
const checkingHealth = ref(false)
let refreshTimer: ReturnType<typeof setTimeout> | null = null
let healthPollTimer: ReturnType<typeof setTimeout> | null = null
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

const {
  accounts,
  pools,
  serverSummary,
  sourceInfo,
  generatedAt,
  errorMessage,
  knownPlatforms,
  loading,
  usingFallback,
  refresh: refreshPoolMap
} = useAccountPoolMapData({
  groupAccounts,
  buildSummary,
  createEmptySummary,
  enrichFallbackAccount,
  normalizeError,
  afterLoad: pruneSelection
})

const platformOptions = computed(() => {
  const set = new Set(knownPlatforms.value)
  if (filters.platform !== 'all') set.add(filters.platform)
  return Array.from(set).sort()
})

const selectedAccount = computed(() => accounts.value.find((item) => item.id === selectedAccountId.value) || null)

const visiblePools = computed<AccountPool[]>(() => pools.value)

const selectedPoolForDetail = computed(() => {
  if (!selectedPoolKey.value) return null
  return visiblePools.value.find((pool) => pool.key === selectedPoolKey.value) || null
})

const detailDrawerOpen = computed(() => Boolean(selectedAccount.value || selectedPoolForDetail.value))

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

const platformHealthRows = computed<PlatformHealthRow[]>(() => {
  const map = new Map<string, PlatformHealthRow>()
  for (const account of accounts.value) {
    const platform = account.platform || 'unknown'
    const row = map.get(platform) || {
      platform,
      total: 0,
      healthy: 0,
      attention: 0,
      quota: 0,
      healthyPercent: 0
    }
    row.total += 1
    if (statusKind(account) === 'healthy') row.healthy += 1
    else row.attention += 1
    if (quotaHasAnySignal(quotaSnapshot(account))) row.quota += 1
    map.set(platform, row)
  }
  return Array.from(map.values())
    .map((row) => ({
      ...row,
      healthyPercent: row.total ? Math.max(4, Math.round((row.healthy / row.total) * 100)) : 0
    }))
    .sort((a, b) => b.total - a.total || platformLabel(a.platform).localeCompare(platformLabel(b.platform)))
    .slice(0, 5)
})

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
    tone: 'neutral',
    metrics: [
      { label: '平台', value: String(platformHealthRows.value.length) },
      { label: 'TLS', value: `${summary.value.tls_enabled}/${summary.value.total || 0}` }
    ]
  },
  {
    key: 'healthy',
    label: '健康可调度',
    value: summary.value.healthy,
    detail: summary.value.total ? `${Math.round((summary.value.healthy / summary.value.total) * 100)}% 可用` : '无账号',
    dotClass: 'bg-emerald-500',
    tone: 'good',
    metrics: [
      { label: 'RPM', value: String(summary.value.rpm || 0) },
      { label: '并发', value: String(summary.value.concurrency || 0) }
    ]
  },
  {
    key: 'attention',
    label: '需要关注',
    value: summary.value.attention,
    detail: '错误、冷却、限流或停用',
    dotClass: summary.value.attention > 0 ? 'bg-amber-500' : 'bg-emerald-500',
    tone: summary.value.attention > 0 ? 'warn' : 'good',
    metrics: [
      { label: '错误', value: String(summary.value.error || 0) },
      { label: '冷却/限流', value: String((summary.value.degraded || 0) + (summary.value.rate_limited || 0)) }
    ]
  },
  {
    key: 'quota',
    label: '额度信号',
    value: summary.value.quota_signals || 0,
    detail: `${summary.value.rate_limited} 限流 · ${summary.value.disabled} 停用`,
    dotClass: summary.value.quota_signals > 0 ? 'bg-sky-500' : 'bg-slate-300',
    tone: summary.value.quota_signals > 0 ? 'accent' : 'neutral',
    metrics: [
      { label: '会话', value: String(summary.value.active_sessions || 0) },
      { label: '停用', value: String(summary.value.disabled || 0) }
    ]
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
    glm: 'GLM',
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

function poolStatusTone(pool: AccountPool): AccountStatusKind {
  const summary = pool.summary
  if ((summary?.error || 0) > 0) return 'error'
  if ((summary?.rate_limited || 0) > 0) return 'rate_limited'
  if ((summary?.degraded || 0) > 0) return 'degraded'
  if ((summary?.disabled || 0) > 0 && poolHealthyCount(pool) === 0) return 'disabled'
  return 'healthy'
}

function poolSummaryText(pool: AccountPool): string {
  const attention = poolAttentionCount(pool)
  const limited = pool.summary?.rate_limited || 0
  const disabled = pool.summary?.disabled || 0
  const parts = [`健康 ${poolHealthyCount(pool)}`, `需关注 ${attention}`]
  if (limited > 0) parts.push(`限流 ${limited}`)
  if (disabled > 0) parts.push(`停用 ${disabled}`)
  return parts.join(' · ')
}

function poolTrafficSummary(pool: AccountPool): string {
  const sessions = pool.summary?.active_sessions ?? pool.accounts.reduce((sum, item) => sum + (item.active_sessions || 0), 0)
  return `${poolRPM(pool)} RPM · ${poolConcurrency(pool)} 并发 · ${sessions} 会话`
}

function poolQuotaSummary(pool: AccountPool): string {
  const quotaSignals = poolQuotaSignalCount(pool)
  const total = pool.accounts.length
  if (quotaSignals > 0) return `额度信号 ${quotaSignals}/${total}`
  if (pool.platform === 'gemini') return 'Gemini 多为项目级额度'
  return '额度未探测'
}

function poolPrimaryIssue(pool: AccountPool): string {
  const summary = pool.summary
  if ((summary?.error || 0) > 0) return `${summary?.error || 0} 个错误账号`
  if ((summary?.rate_limited || 0) > 0) return `${summary?.rate_limited || 0} 个限流中`
  if ((summary?.degraded || 0) > 0) return `${summary?.degraded || 0} 个冷却/降级`
  if ((summary?.disabled || 0) > 0) return `${summary?.disabled || 0} 个已停用`
  if (poolHealthyCount(pool) === pool.accounts.length) return '全部可调度'
  return `${poolAttentionCount(pool)} 个需关注`
}

function poolPreviewAccounts(pool: AccountPool): AccountPoolMapAccount[] {
  return pool.accounts.slice(0, 5)
}

function accountInitial(account: AccountPoolMapAccount): string {
  const text = (account.name || account.platform || '?').trim()
  return text.slice(0, 1).toUpperCase() || '?'
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

function selectPool(pool: AccountPool) {
  selectedPoolKey.value = pool.key
  selectedAccountId.value = null
}

function selectAccount(account: AccountPoolMapAccount) {
  selectedAccountId.value = account.id
  const pool = visiblePools.value.find((item) => item.accounts.some((poolAccount) => poolAccount.id === account.id))
  if (pool) selectedPoolKey.value = pool.key
}

function clearPoolSelection() {
  selectedAccountId.value = null
  selectedPoolKey.value = null
}

function closeDrawer() {
  clearPoolSelection()
}

function handleKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape' && detailDrawerOpen.value) {
    closeDrawer()
  }
}

async function refresh() {
  await refreshPoolMap({
    search: filters.search,
    platform: filters.platform,
    status: filters.status
  })
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

function pruneSelection(nextAccounts: AccountPoolMapAccount[], nextPools: AccountPoolMapPool[]) {
  if (selectedAccountId.value && !nextAccounts.some((item) => item.id === selectedAccountId.value)) {
    selectedAccountId.value = null
  }
  if (selectedPoolKey.value && !nextPools.some((pool) => pool.key === selectedPoolKey.value)) {
    selectedPoolKey.value = null
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
  document.addEventListener('keydown', handleKeydown)
})

onUnmounted(() => {
  if (refreshTimer) clearTimeout(refreshTimer)
  clearHealthPollTimer()
  document.removeEventListener('keydown', handleKeydown)
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

:global(.dark) .account-map-page :is(.account-map-toolbar, .account-map-stat-card, .account-map-filter-card, .account-map-empty-state, .account-pool-card, .account-map-inspector) {
  border-color: rgb(51 65 85);
  background: var(--account-map-surface);
  color: var(--account-map-ink);
}

:global(.dark) .account-map-page :is(.account-map-stat-card p, .account-map-stat-card small, .account-pool-card-main p, .account-pool-card-count span, .account-pool-card-foot, .account-node-meta, .account-map-meta, .inspector-account-main small, .inspector-account-quota) {
  color: var(--account-map-muted);
}

:global(.dark) .account-map-page :is(.account-map-stat-card strong, .account-pool-card-main h2, .account-pool-card-count strong, .account-node p:first-child, .inspector-title, .inspector-account-main b) {
  color: var(--account-map-ink);
}

.account-map-toolbar,
.account-map-stat-card,
.account-map-filter-card,
.account-map-empty-state,
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

.account-pool-platform {
  border-radius: 999px;
  background: var(--account-map-soft);
  padding: 0.34rem 0.62rem;
  color: var(--account-map-ink);
  font-size: 0.76rem;
  font-weight: 800;
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

/* Drawer sticky header: title + close in a fixed top bar */
.inspector-drawer-header {
  position: sticky;
  top: 0;
  z-index: 10;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  border-bottom: 1px solid var(--account-map-border);
  background: var(--account-map-surface);
  padding: 0.85rem 1rem;
  backdrop-filter: blur(10px);
}

.inspector-drawer-header-left {
  min-width: 0;
  flex: 1;
}

.inspector-drawer-header-left .inspector-kicker {
  display: block;
}

.inspector-drawer-header-left .inspector-title {
  margin-top: 0.2rem;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.inspector-drawer-header-empty {
  border-bottom: 0;
  padding-bottom: 0.6rem;
}

.inspector-profile-card {
  padding: 0.85rem 1rem 1rem;
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
  padding: 0.85rem 1rem;
}

.inspector-empty-header {
  /* subtitle only — no title, handled by drawer header */
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
  height: 2.25rem;
  width: 2.25rem;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  border-radius: 999px;
  border: 1px solid rgb(229 231 235);
  color: rgb(107 114 128);
  transition:
    border-color 0.15s ease,
    background-color 0.15s ease,
    color 0.15s ease,
    transform 0.15s ease;
}

.inspector-close:hover {
  border-color: rgb(191 219 254);
  background: rgb(239 246 255);
  color: rgb(29 78 216);
  transform: rotate(90deg);
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
  padding-bottom: 0;
  border-bottom: 1px solid var(--account-map-border);
  margin-bottom: 0.25rem;
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
  margin: 0.75rem 1rem 0;
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
  margin: 0.75rem 1rem 0;
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
  padding: 0.85rem 1rem 1.25rem;
}

.inspector-empty-header {
  border-bottom: 1px solid rgb(243 244 246);
}

:global(.dark) .inspector-empty-header {
  border-bottom-color: rgb(31 41 55);
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

:global(.app-shell.app-shell-anti) .account-map-page :is(.account-map-toolbar, .account-map-filter-card, .account-map-empty-state, .account-pool-card, .account-map-inspector, .inspector-profile-card) {
  border: 3px solid var(--anti-ink) !important;
  border-radius: 0.55rem !important;
  background: var(--anti-paper) !important;
  box-shadow: 6px 6px 0 var(--anti-ink) !important;
  color: var(--anti-ink) !important;
  transform: none !important;
}

:global(.app-shell.app-shell-anti) .account-map-page :is(.account-map-stat-card, .account-map-stat-metrics span, .inspector-section, .inspector-metric, .inspector-account-row, .inspector-alert, .account-node-quota, .quota-stat-card, .quota-empty-state),
:global(.app-shell.app-shell-anti) .account-map-page :is(.account-pool-card-metrics span, .inspector-account-quota) {
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

:global(.app-shell.app-shell-anti) .account-map-page button:not(.account-node):not(.inspector-account-row):not(.inspector-close) {
  border: 3px solid var(--anti-ink) !important;
  border-radius: 0.2rem !important;
  box-shadow: 4px 4px 0 var(--anti-ink) !important;
  color: var(--anti-ink) !important;
  font-weight: 950;
}

:global(.app-shell.app-shell-anti) .account-map-page button:not(.account-node):not(.inspector-account-row):not(.inspector-close):hover {
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

:global(.dark .app-shell.app-shell-anti) .account-map-page :is(.account-map-toolbar, .account-map-filter-card, .account-map-empty-state, .account-pool-card, .account-map-inspector, .inspector-profile-card) {
  background: linear-gradient(135deg, #101827, #0b1020) !important;
  box-shadow: 6px 6px 0 #334155 !important;
}

:global(.dark .app-shell.app-shell-anti) .account-map-page :is(.account-map-stat-card, .account-map-stat-metrics span, .inspector-section, .inspector-metric, .inspector-account-row, .inspector-alert, .account-node-quota, .quota-stat-card, .quota-empty-state),
:global(.dark .app-shell.app-shell-anti) .account-map-page :is(.account-pool-card-metrics span, .inspector-account-quota) {
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

@media (max-width: 980px) {
  .account-map-inspector {
    position: static;
  }
}

@media (max-width: 768px) {
  .account-map-toolbar-copy {
    align-items: stretch;
    flex-direction: column;
  }

  .account-map-toolbar-actions {
    justify-content: flex-start;
  }

  .account-map-segmented {
    width: 100%;
  }

  .account-map-segmented button {
    flex: 1;
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

/* Account map composition pass: full-width operations layout, fewer dead zones. */
.account-map-page {
  width: 100%;
  max-width: min(1680px, calc(100vw - 1.5rem));
}

.account-map-toolbar {
  background:
    radial-gradient(circle at 2rem 0, rgb(14 165 233 / 0.10), transparent 8rem),
    linear-gradient(135deg, var(--account-map-surface), var(--account-map-subtle));
}

.account-map-summary-grid {
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)) !important;
  gap: 0.9rem;
}

.account-map-stat-card {
  min-height: 5.45rem;
  padding: 0.9rem 0.95rem;
}

.account-map-stat-metrics {
  margin-top: 0.8rem;
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.45rem;
}

.account-map-stat-metrics span {
  min-width: 0;
  border-radius: 0.7rem;
  background: var(--account-map-subtle);
  padding: 0.38rem 0.48rem;
  color: var(--account-map-muted);
  font-size: 0.68rem;
  font-weight: 760;
}

.account-map-stat-metrics b {
  margin-right: 0.22rem;
  color: var(--account-map-ink);
  font-size: 0.78rem;
  font-weight: 900;
  font-variant-numeric: tabular-nums;
}

.account-map-workspace {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(300px, 340px);
  gap: 1rem;
}

.account-map-filter-grid {
  display: grid;
  gap: 0.75rem;
  grid-template-columns: minmax(0, 2fr) minmax(0, 1fr) minmax(0, 1fr) auto;
  align-items: end;
}

@media (max-width: 860px) {
  .account-map-filter-grid {
    grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  }
  .account-map-filter-grid .flex.items-end {
    grid-column: 1 / -1;
  }
}

@media (max-width: 560px) {
  .account-map-filter-grid {
    grid-template-columns: 1fr;
  }
}

.account-pool-card-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(min(100%, 204px), 1fr));
  gap: 0.9rem;
}

.account-pool-card {
  position: relative;
  display: grid;
  min-height: 15.25rem;
  cursor: pointer;
  grid-template-rows: auto 1fr auto auto;
  overflow: hidden;
  border: 1px solid var(--account-map-border);
  border-radius: 1.15rem;
  background: var(--account-map-surface);
  box-shadow: var(--account-map-shadow);
  color: var(--account-map-good);
  outline: none;
  padding: 0.9rem;
  transition:
    border-color 0.16s ease,
    box-shadow 0.16s ease,
    transform 0.16s ease,
    background-color 0.16s ease;
}

.account-pool-card:hover,
.account-pool-card:focus-visible {
  transform: translateY(-2px);
  border-color: rgb(14 165 233 / 0.46);
  box-shadow: 0 16px 34px rgb(15 23 42 / 0.10);
}

.account-pool-card-active {
  border-color: rgb(14 165 233 / 0.72);
  box-shadow: 0 0 0 4px rgb(14 165 233 / 0.12), 0 18px 38px rgb(15 23 42 / 0.12);
}

.account-pool-card-degraded {
  color: var(--account-map-warn);
}

.account-pool-card-rate_limited {
  color: rgb(124 58 237);
}

.account-pool-card-error {
  color: var(--account-map-danger);
}

.account-pool-card-disabled {
  color: var(--account-map-faint);
}

.account-pool-card-head,
.account-pool-card-foot,
.account-pool-card-preview {
  position: relative;
  display: flex;
  align-items: center;
}

.account-pool-card-head,
.account-pool-card-foot {
  justify-content: space-between;
  gap: 0.65rem;
}

.account-pool-status-dot {
  height: 0.62rem;
  width: 0.62rem;
  flex: none;
  border-radius: 999px;
  background: currentColor;
  box-shadow: 0 0 0 4px color-mix(in srgb, currentColor 13%, transparent);
}

.account-pool-card-main {
  position: relative;
  margin-top: 0.95rem;
  min-width: 0;
}

.account-pool-card-main h2 {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--account-map-ink);
  font-size: 0.98rem;
  font-weight: 880;
}

.account-pool-card-count {
  margin-top: 0.8rem;
  display: flex;
  align-items: baseline;
  gap: 0.38rem;
  color: var(--account-map-ink);
}

.account-pool-card-count strong {
  color: var(--account-map-ink);
  font-size: 2.35rem;
  font-weight: 950;
  letter-spacing: -0.05em;
  line-height: 0.9;
}

.account-pool-card-count span {
  color: var(--account-map-muted);
  font-size: 0.78rem;
  font-weight: 760;
}

.account-pool-card-main p {
  margin-top: 0.65rem;
  color: var(--account-map-muted);
  font-size: 0.74rem;
  line-height: 1.35;
}

.account-pool-card-metrics {
  position: relative;
  margin-top: 0.9rem;
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.45rem;
}

.account-pool-card-metrics span {
  min-width: 0;
  border-radius: 0.72rem;
  background: var(--account-map-subtle);
  padding: 0.42rem 0.5rem;
  color: var(--account-map-muted);
  font-size: 0.68rem;
  font-weight: 760;
}

.account-pool-card-metrics b {
  margin-right: 0.22rem;
  color: var(--account-map-ink);
  font-size: 0.76rem;
  font-weight: 900;
  font-variant-numeric: tabular-nums;
}

.account-pool-card-preview {
  margin-top: 0.85rem;
  min-height: 2rem;
}

.account-pool-avatar {
  display: inline-flex;
  height: 2rem;
  width: 2rem;
  align-items: center;
  justify-content: center;
  border: 2px solid var(--account-map-surface);
  border-radius: 999px;
  background: var(--account-map-subtle);
  color: currentColor;
  font-size: 0.72rem;
  font-weight: 900;
  box-shadow: 0 1px 2px rgb(15 23 42 / 0.08);
}

.account-pool-avatar + .account-pool-avatar {
  margin-left: -0.42rem;
}

.account-pool-avatar-more {
  color: var(--account-map-muted);
}

.account-pool-card-foot {
  margin-top: 0.85rem;
  border-top: 1px solid var(--account-map-border);
  padding-top: 0.75rem;
  color: var(--account-map-muted);
  font-size: 0.72rem;
  font-weight: 800;
}

.account-map-detail-rail {
  align-self: start;
  min-height: calc(100vh - 6.25rem);
}

.account-map-inspector {
  height: calc(100vh - 6.25rem);
  max-height: calc(100vh - 6.25rem);
  overflow-y: auto;
  scrollbar-gutter: stable;
}

.account-map-inspector::-webkit-scrollbar {
  width: 8px;
}

.account-map-inspector::-webkit-scrollbar-thumb {
  border: 2px solid transparent;
  border-radius: 999px;
  background: rgb(148 163 184 / 0.55);
  background-clip: content-box;
}

.inspector-section {
  margin-inline: 0.9rem;
}

.inspector-pool-card {
  background:
    radial-gradient(circle at 90% 16%, rgb(14 165 233 / 0.15), transparent 4.8rem),
    var(--account-map-surface);
}

.inspector-account-list {
  display: grid;
  gap: 0.55rem;
}

.inspector-account-row {
  display: grid;
  width: 100%;
  grid-template-columns: auto minmax(0, 1fr) minmax(4.5rem, auto) 1.25rem;
  align-items: center;
  gap: 0.6rem;
  border: 1px solid var(--account-map-border);
  border-radius: 0.9rem;
  background: var(--account-map-surface);
  padding: 0.72rem 0.68rem;
  text-align: left;
  transition:
    border-color 0.15s ease,
    background-color 0.15s ease,
    transform 0.15s ease;
}

.inspector-account-row:hover {
  transform: translateX(2px);
  border-color: rgb(14 165 233 / 0.38);
  background: var(--account-map-subtle);
}

.inspector-account-chevron {
  flex-shrink: 0;
  color: var(--account-map-faint);
  transition: color 0.15s ease, transform 0.15s ease;
}

.inspector-account-row:hover .inspector-account-chevron {
  color: var(--account-map-accent);
  transform: translateX(2px);
}

.inspector-account-dot {
  height: 0.72rem;
  width: 0.72rem;
  border-radius: 999px;
  background: currentColor;
}

.inspector-account-main {
  min-width: 0;
}

.inspector-account-main b,
.inspector-account-main small {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.inspector-account-main b {
  color: var(--account-map-ink);
  font-size: 0.82rem;
  font-weight: 850;
}

.inspector-account-main small {
  margin-top: 0.16rem;
  color: var(--account-map-muted);
  font-size: 0.7rem;
}

.inspector-account-quota {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  border-radius: 999px;
  background: var(--account-map-subtle);
  padding: 0.25rem 0.48rem;
  color: var(--account-map-muted);
  font-size: 0.68rem;
  font-weight: 780;
}

.inspector-overview-grid,
.inspector-snapshot-grid {
  gap: 0.55rem;
}

.account-map-platform-list {
  margin-top: 0.85rem;
  display: grid;
  gap: 0.55rem;
}

.account-map-platform-row {
  border-radius: 0.85rem;
  border: 1px solid var(--account-map-border);
  background: var(--account-map-surface);
  padding: 0.7rem;
}

.account-map-platform-row-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  color: var(--account-map-ink);
  font-size: 0.78rem;
  font-weight: 800;
}

.account-map-platform-row-head strong {
  font-variant-numeric: tabular-nums;
}

.account-map-platform-track {
  margin-top: 0.48rem;
  height: 0.42rem;
  overflow: hidden;
  border-radius: 999px;
  background: var(--account-map-soft);
}

.account-map-platform-track span {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: linear-gradient(90deg, var(--account-map-good), var(--account-map-accent));
}

.account-map-platform-row p {
  margin-top: 0.42rem;
  color: var(--account-map-muted);
  font-size: 0.72rem;
}

@media (max-width: 980px) {
  .account-map-workspace {
    grid-template-columns: minmax(0, 1fr);
  }

  .account-map-inspector {
    height: auto;
    max-height: none;
    min-height: 0;
  }
}

@media (max-width: 640px) {
  .account-map-page {
    max-width: 100%;
  }

}

/* Drawer pass: the map stays scan-friendly; deep inspection moves to a focused side panel. */
.account-map-workspace {
  display: block;
}

.account-map-main-column {
  width: 100%;
}

.account-pool-card-grid {
  grid-template-columns: repeat(auto-fill, minmax(min(100%, 320px), 1fr));
  gap: 1rem;
}

.account-pool-card {
  min-height: 0;
  grid-template-rows: auto auto auto auto auto auto;
  padding: 1rem;
  background: linear-gradient(180deg, var(--account-map-surface), color-mix(in srgb, var(--account-map-subtle) 44%, var(--account-map-surface)));
}

.account-pool-card-head {
  align-items: flex-start;
}

.account-pool-card-title {
  min-width: 0;
}

.account-pool-card-title h2 {
  margin-top: 0.55rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--account-map-ink);
  font-size: 1rem;
  font-weight: 880;
}

.account-pool-status-chip {
  flex: none;
  display: inline-flex;
  align-items: center;
  gap: 0.32rem;
  border-radius: 999px;
  border: 1px solid color-mix(in srgb, currentColor 32%, transparent);
  background: color-mix(in srgb, currentColor 12%, var(--account-map-surface));
  padding: 0.28rem 0.62rem 0.28rem 0.48rem;
  color: currentColor;
  font-size: 0.68rem;
  font-weight: 860;
}

.account-pool-status-chip::before {
  content: '';
  display: inline-block;
  width: 0.48rem;
  height: 0.48rem;
  border-radius: 999px;
  background: currentColor;
  flex-shrink: 0;
}

.account-pool-card-main {
  margin-top: 0.75rem;
}

.account-pool-card-count {
  margin-top: 0;
}

.account-pool-card-count strong {
  font-size: 2.15rem;
}

.account-pool-card-main p {
  margin-top: 0.5rem;
}

.account-pool-card-metrics {
  grid-template-columns: repeat(3, minmax(0, 1fr));
  margin-top: 0.85rem;
}

.account-pool-card-metrics span {
  min-height: 2.55rem;
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 0.12rem;
}

.account-pool-card-metrics b {
  display: block;
  margin-right: 0;
  font-size: 0.92rem;
}

.account-pool-card-signal {
  position: relative;
  margin-top: 0.8rem;
  display: grid;
  grid-template-columns: minmax(0, 0.95fr) minmax(0, 1.05fr);
  gap: 0.5rem;
}

.account-pool-card-signal span,
.account-pool-card-signal strong {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  border-radius: 0.78rem;
  border: 1px solid var(--account-map-border);
  background: var(--account-map-surface);
  padding: 0.5rem 0.58rem;
  font-size: 0.7rem;
  line-height: 1.1;
}

.account-pool-card-signal span {
  color: var(--account-map-muted);
  font-weight: 760;
}

.account-pool-card-signal strong {
  color: var(--account-map-ink);
  font-weight: 880;
}

.account-pool-card-preview {
  margin-top: 0.85rem;
}

.account-pool-card-foot {
  margin-top: 0.85rem;
  align-items: center;
}

.account-pool-detail-button {
  flex: none;
  border-radius: 999px;
  border: 1px solid rgb(14 165 233 / 0.26);
  background: rgb(240 249 255);
  padding: 0.38rem 0.68rem;
  color: var(--account-map-accent-strong);
  font-size: 0.7rem;
  font-weight: 860;
  transition:
    border-color 0.15s ease,
    background-color 0.15s ease,
    color 0.15s ease,
    transform 0.15s ease;
}

.account-pool-detail-button:hover {
  transform: translateY(-1px);
  border-color: rgb(14 165 233 / 0.46);
  background: rgb(224 242 254);
}

.account-map-drawer-layer {
  position: fixed;
  inset: 0;
  z-index: 80;
  display: flex;
  justify-content: flex-end;
  background: rgb(15 23 42 / 0.4);
  padding: 1rem;
  backdrop-filter: blur(8px);
  animation: drawer-overlay-in 0.2s ease-out;
}

@keyframes drawer-overlay-in {
  from { opacity: 0; }
  to { opacity: 1; }
}

.account-map-drawer {
  width: min(560px, calc(100vw - 2rem));
  height: 100%;
  max-height: calc(100vh - 2rem);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  border: 1px solid var(--account-map-border);
  border-radius: 1.35rem;
  background: var(--account-map-surface);
  box-shadow: 0 32px 80px rgb(15 23 42 / 0.22), 0 0 0 1px rgb(255 255 255 / 0.06);
  animation: account-map-drawer-in 0.2s cubic-bezier(0.22, 1, 0.36, 1);
}

.account-map-drawer .account-map-inspector {
  position: static;
  flex: 1;
  height: auto;
  max-height: none;
  overflow-y: auto;
  border: 0;
  border-radius: 0;
  box-shadow: none;
}

@keyframes account-map-drawer-in {
  from {
    opacity: 0;
    transform: translateX(1.5rem) scale(0.98);
  }

  to {
    opacity: 1;
    transform: translateX(0) scale(1);
  }
}

@media (max-width: 760px) {
  .account-map-drawer-layer {
    padding: 0;
  }

  .account-map-drawer {
    width: 100vw;
    max-height: 100vh;
    border-radius: 0;
  }

  .account-pool-card-grid {
    grid-template-columns: 1fr;
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

.dark .account-map-page :is(.account-map-toolbar, .account-map-stat-card, .account-map-filter-card, .account-map-empty-state, .account-pool-card, .account-map-inspector) {
  border-color: rgb(51 65 85);
  background: var(--account-map-surface);
  color: var(--account-map-ink);
}

.dark .account-map-page .inspector-drawer-header {
  background: rgb(15 23 42 / 0.97);
  border-bottom-color: rgb(51 65 85);
}

.dark .account-map-page :is(.account-map-stat-card p, .account-map-stat-card small, .account-pool-card-main p, .account-pool-card-count span, .account-pool-card-foot, .account-node-meta, .account-map-meta, .account-map-field > span, .inspector-subtitle, .inspector-section-title p, .inspector-section-title span, .inspector-account-main small, .inspector-account-quota) {
  color: var(--account-map-muted);
}

.dark .account-map-page :is(.account-map-stat-card strong, .account-pool-card-main h2, .account-pool-card-count strong, .account-node p:first-child, .account-map-filter-header h2, .inspector-title, .inspector-section-title h3, .inspector-metric p:last-child, .inspector-account-main b) {
  color: var(--account-map-ink);
}

.dark .account-map-page :is(.account-map-control, .account-map-segmented, .account-map-stat-metrics span, .account-pool-card-metrics span, .inspector-metric, .inspector-section, .inspector-account-row, .inspector-account-quota) {
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

.dark .account-map-page :is(.inspector-heading, .inspector-empty-header, .quota-fact-list, .inspector-row) {
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

.app-shell.app-shell-anti .account-map-page :is(.account-map-toolbar, .account-map-filter-card, .account-map-empty-state, .account-pool-card, .account-map-drawer, .account-map-inspector, .inspector-profile-card) {
  border: 3px solid var(--anti-ink) !important;
  border-radius: 0.55rem !important;
  background: var(--anti-paper) !important;
  box-shadow: 6px 6px 0 var(--anti-ink) !important;
  color: var(--anti-ink) !important;
  transform: none !important;
}

.app-shell.app-shell-anti .account-map-page :is(.account-map-stat-card, .account-map-stat-metrics span, .inspector-section, .inspector-metric, .inspector-account-row, .inspector-alert, .account-node-quota, .quota-stat-card, .quota-empty-state),
.app-shell.app-shell-anti .account-map-page :is(.account-pool-card-metrics span, .account-pool-card-signal span, .account-pool-card-signal strong, .account-pool-status-chip, .inspector-account-quota) {
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

.app-shell.app-shell-anti .account-map-page button:not(.account-node):not(.inspector-account-row):not(.inspector-close) {
  border: 3px solid var(--anti-ink) !important;
  border-radius: 0.2rem !important;
  box-shadow: 4px 4px 0 var(--anti-ink) !important;
  color: var(--anti-ink) !important;
  font-weight: 950;
}

.app-shell.app-shell-anti .account-map-page button:not(.account-node):not(.inspector-account-row):not(.inspector-close):hover {
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

.app-shell.app-shell-anti .account-map-page .account-map-drawer-layer {
  background: rgb(5 5 5 / 0.52) !important;
  backdrop-filter: blur(3px) contrast(1.12);
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

.dark .app-shell.app-shell-anti .account-map-page :is(.account-map-toolbar, .account-map-filter-card, .account-map-empty-state, .account-pool-card, .account-map-drawer, .account-map-inspector, .inspector-profile-card) {
  background: linear-gradient(135deg, #101827, #0b1020) !important;
  box-shadow: 6px 6px 0 #334155 !important;
}

.dark .app-shell.app-shell-anti .account-map-page :is(.account-map-stat-card, .account-map-stat-metrics span, .inspector-section, .inspector-metric, .inspector-account-row, .inspector-alert, .account-node-quota, .quota-stat-card, .quota-empty-state),
.dark .app-shell.app-shell-anti .account-map-page :is(.account-pool-card-metrics span, .account-pool-card-signal span, .account-pool-card-signal strong, .account-pool-status-chip, .inspector-account-quota) {
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

.app-shell.app-shell-anti .account-map-page :is(.account-map-platform-row, .account-node-meta span) {
  border: 2px solid var(--anti-ink) !important;
  border-radius: 0.35rem !important;
  background: var(--anti-paper) !important;
  box-shadow: 2px 2px 0 var(--anti-ink) !important;
  color: var(--anti-ink) !important;
}

.app-shell.app-shell-anti .account-map-page .account-map-platform-track {
  border: 2px solid var(--anti-ink);
  border-radius: 0;
  background: var(--anti-paper);
}

.app-shell.app-shell-anti .account-map-page .account-map-platform-track span {
  background: var(--anti-green);
}

.dark .app-shell.app-shell-anti .account-map-page :is(.account-map-platform-row, .account-node-meta span) {
  background: #111827 !important;
  box-shadow: 2px 2px 0 #334155 !important;
}
</style>
