<template>
  <div class="account-map-page space-y-5">
    <section class="overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-900">
      <div class="flex flex-col gap-4 border-b border-gray-100 p-5 dark:border-dark-800 lg:flex-row lg:items-center lg:justify-between">
        <div>
          <p class="text-xs font-semibold uppercase tracking-[0.22em] text-primary-600 dark:text-primary-400">
            Gateway Lab
          </p>
          <h1 class="mt-2 text-2xl font-bold text-gray-950 dark:text-white">
            账号池地图
          </h1>
          <p class="mt-1 max-w-3xl text-sm text-gray-500 dark:text-gray-400">
            用泳道地图查看账号池健康、调度状态和指纹覆盖情况。默认隐藏空泳道，右侧始终给出选中账号或全局建议，避免控制台留白。
          </p>
        </div>
        <div class="flex flex-wrap gap-2">
          <button
            type="button"
            class="rounded-xl border border-gray-200 px-3 py-2 text-sm font-medium text-gray-700 transition hover:border-primary-300 hover:text-primary-600 dark:border-dark-700 dark:text-gray-300 dark:hover:border-primary-500 dark:hover:text-primary-300"
            @click="refresh"
          >
            {{ loading ? '刷新中...' : '刷新地图' }}
          </button>
          <button
            type="button"
            class="rounded-xl bg-primary-600 px-3 py-2 text-sm font-semibold text-white shadow-sm transition hover:bg-primary-700"
            @click="goAccounts"
          >
            账号管理
          </button>
        </div>
      </div>

      <div class="grid gap-3 p-4 sm:grid-cols-2 lg:grid-cols-4">
        <div
          v-for="item in summaryCards"
          :key="item.key"
          class="rounded-2xl border border-gray-100 bg-gray-50/70 p-4 dark:border-dark-800 dark:bg-dark-800/50"
        >
          <div class="flex items-center justify-between gap-3">
            <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ item.label }}</p>
            <span class="h-2.5 w-2.5 rounded-full" :class="item.dotClass"></span>
          </div>
          <p class="mt-3 text-2xl font-bold text-gray-950 dark:text-white">{{ item.value }}</p>
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ item.detail }}</p>
        </div>
      </div>
    </section>

    <section class="grid gap-5 xl:grid-cols-[minmax(0,1fr)_380px]">
      <div class="space-y-4">
        <div class="rounded-2xl border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-900">
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
        </div>

        <div v-if="loading && !accounts.length" class="grid gap-3">
          <div v-for="idx in 3" :key="idx" class="h-32 animate-pulse rounded-2xl bg-gray-100 dark:bg-dark-800"></div>
        </div>

        <div v-else-if="!visiblePools.length" class="rounded-2xl border border-dashed border-gray-300 bg-white p-8 text-center dark:border-dark-700 dark:bg-dark-900">
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

      <aside class="space-y-4">
        <section class="sticky top-24 rounded-2xl border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-900">
          <template v-if="selectedAccount">
            <div class="flex items-start justify-between gap-3">
              <div class="min-w-0">
                <p class="text-xs font-semibold uppercase tracking-[0.18em] text-gray-400">Inspector</p>
                <h2 class="mt-2 truncate text-xl font-bold text-gray-950 dark:text-white">{{ selectedAccount.name }}</h2>
                <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                  {{ platformLabel(selectedAccount.platform) }} · {{ selectedAccount.type }}
                </p>
              </div>
              <span class="rounded-full px-2.5 py-1 text-xs font-semibold" :class="statusBadgeClass(selectedAccount)">
                {{ statusLabel(selectedAccount) }}
              </span>
            </div>

            <div class="mt-5 grid grid-cols-2 gap-3">
              <MetricTile label="可调度" :value="selectedAccount.schedulable ? '是' : '否'" />
              <MetricTile label="当前并发" :value="String(selectedAccount.current_concurrency ?? 0)" />
              <MetricTile label="当前 RPM" :value="String(selectedAccount.current_rpm ?? 0)" />
              <MetricTile label="活跃会话" :value="String(selectedAccount.active_sessions ?? 0)" />
            </div>

            <div class="mt-5 space-y-3">
              <InspectorRow label="分组" :value="groupNames(selectedAccount)" />
              <InspectorRow label="代理" :value="selectedAccount.proxy?.name || '未绑定'" />
              <InspectorRow label="TLS 指纹" :value="selectedAccount.enable_tls_fingerprint ? `已启用 #${selectedAccount.tls_fingerprint_profile_id || '-'}` : '未启用'" />
              <InspectorRow label="缓存 TTL" :value="selectedAccount.cache_ttl_override_enabled ? `强制 ${selectedAccount.cache_ttl_override_target || '-'}` : '按上游返回'" />
              <InspectorRow label="最近使用" :value="formatDate(selectedAccount.last_used_at)" />
            </div>

            <div v-if="selectedAccount.error_message || selectedAccount.temp_unschedulable_reason" class="mt-5 rounded-xl border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-200">
              <p class="font-semibold">最近问题</p>
              <p class="mt-1 line-clamp-4 text-xs">{{ selectedAccount.temp_unschedulable_reason || selectedAccount.error_message }}</p>
            </div>

            <div class="mt-5 grid gap-2">
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
            <p class="text-xs font-semibold uppercase tracking-[0.18em] text-gray-400">Inspector</p>
            <h2 class="mt-2 text-xl font-bold text-gray-950 dark:text-white">全局建议</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              选择左侧账号节点查看调度、TLS、缓存和错误状态。当前视图下最需要关注的账号已在下方列出。
            </p>

            <div class="mt-5 space-y-2">
              <button
                v-for="account in attentionAccounts"
                :key="account.id"
                type="button"
                class="w-full rounded-xl border border-gray-100 bg-gray-50 p-3 text-left transition hover:border-primary-200 hover:bg-white dark:border-dark-800 dark:bg-dark-800/70 dark:hover:border-primary-500/40"
                @click="selectAccount(account)"
              >
                <div class="flex items-center justify-between gap-2">
                  <p class="truncate text-sm font-semibold text-gray-900 dark:text-white">{{ account.name }}</p>
                  <span class="rounded-full px-2 py-0.5 text-[11px] font-semibold" :class="statusBadgeClass(account)">
                    {{ statusLabel(account) }}
                  </span>
                </div>
                <p class="mt-1 truncate text-xs text-gray-500 dark:text-gray-400">
                  {{ account.error_message || account.temp_unschedulable_reason || '等待恢复或需要人工复核' }}
                </p>
              </button>
              <div v-if="!attentionAccounts.length" class="rounded-xl border border-emerald-100 bg-emerald-50 p-4 text-sm text-emerald-700 dark:border-emerald-500/30 dark:bg-emerald-500/10 dark:text-emerald-200">
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
import { computed, defineComponent, h, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { adminAPI } from '@/api/admin'
import type { Account } from '@/types'

type ViewMode = 'status' | 'traffic' | 'error'
type AccountStatusKind = 'healthy' | 'degraded' | 'rate_limited' | 'error' | 'disabled'

interface AccountPool {
  key: string
  platform: string
  type: string
  accounts: Account[]
}

const MetricTile = defineComponent({
  props: {
    label: { type: String, required: true },
    value: { type: String, required: true }
  },
  setup(props) {
    return () => h('div', { class: 'rounded-xl bg-gray-50 p-3 dark:bg-dark-800' }, [
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
    return () => h('div', { class: 'flex items-start justify-between gap-4 border-b border-gray-100 pb-2 text-sm last:border-b-0 dark:border-dark-800' }, [
      h('span', { class: 'shrink-0 text-gray-500 dark:text-gray-400' }, props.label),
      h('span', { class: 'min-w-0 text-right font-medium text-gray-900 dark:text-white' }, props.value)
    ])
  }
})

const router = useRouter()
const accounts = ref<Account[]>([])
const loading = ref(false)
const selectedAccountId = ref<number | null>(null)

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

const platformOptions = computed(() => Array.from(new Set(accounts.value.map((item) => item.platform))).sort())

const selectedAccount = computed(() => accounts.value.find((item) => item.id === selectedAccountId.value) || null)

const filteredAccounts = computed(() => {
  const query = filters.search.toLowerCase()
  return accounts.value.filter((account) => {
    if (filters.platform !== 'all' && account.platform !== filters.platform) return false
    if (filters.status !== 'all' && statusKind(account) !== filters.status) return false
    if (!query) return true
    const haystack = [
      account.name,
      account.platform,
      account.type,
      account.error_message || '',
      account.temp_unschedulable_reason || '',
      ...(account.groups || []).map((group) => group.name)
    ].join(' ').toLowerCase()
    return haystack.includes(query)
  })
})

const visiblePools = computed<AccountPool[]>(() => {
  const map = new Map<string, AccountPool>()
  for (const account of filteredAccounts.value) {
    const key = `${account.platform}:${account.type}`
    if (!map.has(key)) {
      map.set(key, {
        key,
        platform: account.platform,
        type: account.type,
        accounts: []
      })
    }
    map.get(key)?.accounts.push(account)
  }
  return Array.from(map.values())
    .map((pool) => ({
      ...pool,
      accounts: pool.accounts.slice().sort((a, b) => statusWeight(a) - statusWeight(b) || a.name.localeCompare(b.name))
    }))
    .sort((a, b) => platformLabel(a.platform).localeCompare(platformLabel(b.platform)) || accountTypeLabel(a.type).localeCompare(accountTypeLabel(b.type)))
})

const attentionAccounts = computed(() => filteredAccounts.value
  .filter((account) => statusKind(account) !== 'healthy')
  .sort((a, b) => statusWeight(a) - statusWeight(b))
  .slice(0, 6))

const summary = computed(() => {
  const total = filteredAccounts.value.length
  const healthy = filteredAccounts.value.filter((item) => statusKind(item) === 'healthy').length
  const rateLimited = filteredAccounts.value.filter((item) => statusKind(item) === 'rate_limited').length
  const disabled = filteredAccounts.value.filter((item) => statusKind(item) === 'disabled').length
  const attention = total - healthy
  return { total, healthy, rateLimited, disabled, attention }
})

const summaryCards = computed(() => [
  {
    key: 'total',
    label: '账号总数',
    value: summary.value.total,
    detail: `${visiblePools.value.length} 个池/泳道`,
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
    value: summary.value.rateLimited + summary.value.disabled,
    detail: `${summary.value.rateLimited} 限流 · ${summary.value.disabled} 停用`,
    dotClass: summary.value.rateLimited + summary.value.disabled > 0 ? 'bg-violet-500' : 'bg-gray-300'
  }
])

function statusKind(account: Account): AccountStatusKind {
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
  const labels: Record<AccountStatusKind, string> = {
    healthy: '健康',
    degraded: '降级',
    rate_limited: '限流',
    error: '错误',
    disabled: '停用'
  }
  return labels[statusKind(account)]
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
  return pool.accounts.filter((item) => statusKind(item) === 'healthy').length
}

function poolAttentionCount(pool: AccountPool): number {
  return pool.accounts.length - poolHealthyCount(pool)
}

function poolRPM(pool: AccountPool): number {
  return pool.accounts.reduce((sum, item) => sum + (item.current_rpm || 0), 0)
}

function poolConcurrency(pool: AccountPool): number {
  return pool.accounts.reduce((sum, item) => sum + (item.current_concurrency || 0), 0)
}

function poolFingerprintCount(pool: AccountPool): string {
  const enabled = pool.accounts.filter((item) => item.enable_tls_fingerprint).length
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

function selectAccount(account: Account) {
  selectedAccountId.value = account.id
}

async function refresh() {
  loading.value = true
  try {
    const response = await adminAPI.accounts.list(1, 1000, {
      sort_by: 'platform',
      sort_order: 'asc'
    })
    accounts.value = response.items || []
    if (selectedAccountId.value && !accounts.value.some((item) => item.id === selectedAccountId.value)) {
      selectedAccountId.value = null
    }
  } finally {
    loading.value = false
  }
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

onMounted(() => {
  void refresh()
})
</script>

<style scoped>
.account-node {
  min-height: 5.6rem;
  border: 1px solid rgb(229 231 235);
  border-radius: 1rem;
  padding: 0.85rem;
  background: rgb(249 250 251);
  color: rgb(17 24 39);
  transition:
    border-color 0.15s ease,
    box-shadow 0.15s ease,
    transform 0.15s ease,
    background-color 0.15s ease;
}

.account-node:hover {
  transform: translateY(-1px);
  box-shadow: 0 10px 30px rgb(15 23 42 / 0.08);
}

.dark .account-node {
  border-color: rgb(31 41 55);
  background: rgb(17 24 39 / 0.72);
  color: rgb(243 244 246);
}

.account-node-selected {
  border-color: rgb(37 99 235) !important;
  box-shadow: 0 0 0 4px rgb(59 130 246 / 0.13), 0 14px 34px rgb(15 23 42 / 0.12);
}

.account-node-healthy {
  border-color: rgb(167 243 208);
  background: linear-gradient(135deg, rgb(236 253 245), rgb(255 255 255));
  color: rgb(6 95 70);
}

.account-node-degraded {
  border-color: rgb(253 230 138);
  background: linear-gradient(135deg, rgb(255 251 235), rgb(255 255 255));
  color: rgb(146 64 14);
}

.account-node-limited {
  border-color: rgb(221 214 254);
  background: linear-gradient(135deg, rgb(245 243 255), rgb(255 255 255));
  color: rgb(109 40 217);
}

.account-node-error {
  border-color: rgb(254 205 211);
  background: linear-gradient(135deg, rgb(255 241 242), rgb(255 255 255));
  color: rgb(190 18 60);
}

.account-node-muted {
  border-color: rgb(229 231 235);
  background: rgb(249 250 251);
  color: rgb(75 85 99);
}

.account-node-blue {
  border-color: rgb(191 219 254);
  background: linear-gradient(135deg, rgb(239 246 255), rgb(255 255 255));
  color: rgb(29 78 216);
}

.account-node-blue-strong {
  border-color: rgb(96 165 250);
  background: linear-gradient(135deg, rgb(219 234 254), rgb(239 246 255));
  color: rgb(30 64 175);
}

.dark .account-node-healthy {
  border-color: rgb(16 185 129 / 0.45);
  background: rgb(6 78 59 / 0.22);
  color: rgb(167 243 208);
}

.dark .account-node-degraded {
  border-color: rgb(245 158 11 / 0.5);
  background: rgb(120 53 15 / 0.25);
  color: rgb(253 230 138);
}

.dark .account-node-limited {
  border-color: rgb(139 92 246 / 0.5);
  background: rgb(76 29 149 / 0.25);
  color: rgb(221 214 254);
}

.dark .account-node-error {
  border-color: rgb(244 63 94 / 0.5);
  background: rgb(136 19 55 / 0.25);
  color: rgb(254 205 211);
}

.dark .account-node-muted {
  border-color: rgb(55 65 81);
  background: rgb(31 41 55 / 0.72);
  color: rgb(156 163 175);
}

.dark .account-node-blue,
.dark .account-node-blue-strong {
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

.dark .inspector-action {
  border-color: rgb(55 65 81);
  color: rgb(209 213 219);
}

.dark .inspector-action:hover {
  border-color: rgb(59 130 246 / 0.7);
  color: rgb(147 197 253);
}
</style>
