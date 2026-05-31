import { computed, ref } from 'vue'
import { adminAPI } from '@/api/admin'
import type { Account } from '@/types'
import type {
  AccountPoolMapAccount,
  AccountPoolMapPool,
  AccountPoolMapResponse,
  AccountPoolMapStatusKind,
  AccountPoolMapSummary
} from '@/api/admin/accounts'

export interface AccountPoolMapRefreshFilters {
  search: string
  platform: string
  status: 'all' | AccountPoolMapStatusKind
}

export type AccountPoolMapDataSourceMode = 'pool-map' | 'fallback' | 'empty'

interface UseAccountPoolMapDataOptions {
  groupAccounts: (accounts: AccountPoolMapAccount[]) => AccountPoolMapPool[]
  buildSummary: (accounts: AccountPoolMapAccount[], pools: AccountPoolMapPool[]) => AccountPoolMapSummary
  createEmptySummary: () => AccountPoolMapSummary
  enrichFallbackAccount: (account: Account) => AccountPoolMapAccount
  normalizeError: (error: unknown) => string
  afterLoad?: (accounts: AccountPoolMapAccount[], pools: AccountPoolMapPool[]) => void
}

export function useAccountPoolMapData(options: UseAccountPoolMapDataOptions) {
  const accounts = ref<AccountPoolMapAccount[]>([])
  const pools = ref<AccountPoolMapPool[]>([])
  const serverSummary = ref<AccountPoolMapSummary | null>(null)
  const sourceInfo = ref<AccountPoolMapResponse['source'] | null>(null)
  const generatedAt = ref('')
  const errorMessage = ref('')
  const knownPlatforms = ref<string[]>([])
  const loading = ref(false)
  const sourceMode = ref<AccountPoolMapDataSourceMode>('empty')
  let refreshSeq = 0

  const usingFallback = computed(() => sourceMode.value === 'fallback')

  async function refresh(filters: AccountPoolMapRefreshFilters) {
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
      sourceMode.value = 'pool-map'
    } catch (error) {
      if (seq !== refreshSeq) return
      try {
        await loadFallbackAccounts(filters)
        if (seq !== refreshSeq) return
        sourceMode.value = 'fallback'
        errorMessage.value = options.normalizeError(error) || '账号池地图接口暂不可用，已回退到账号列表聚合。'
      } catch (fallbackError) {
        if (seq !== refreshSeq) return
        sourceMode.value = 'empty'
        errorMessage.value = options.normalizeError(fallbackError) || options.normalizeError(error) || '账号池地图加载失败。'
        accounts.value = []
        pools.value = []
        serverSummary.value = options.createEmptySummary()
        sourceInfo.value = null
        generatedAt.value = ''
        options.afterLoad?.(accounts.value, pools.value)
      }
    } finally {
      if (seq === refreshSeq) {
        loading.value = false
      }
    }
  }

  function loadPoolMapResponse(response: AccountPoolMapResponse) {
    const nextAccounts = response.accounts || []
    const nextPools = response.pools || options.groupAccounts(nextAccounts)
    accounts.value = nextAccounts
    pools.value = nextPools
    serverSummary.value = response.summary || options.buildSummary(nextAccounts, nextPools)
    sourceInfo.value = response.source || null
    generatedAt.value = response.generated_at || ''
    mergeKnownPlatforms(nextAccounts)
    options.afterLoad?.(nextAccounts, nextPools)
  }

  async function loadFallbackAccounts(filters: AccountPoolMapRefreshFilters) {
    const response = await adminAPI.accounts.list(1, 1000, {
      platform: filters.platform !== 'all' ? filters.platform : undefined,
      search: filters.search || undefined,
      sort_by: 'platform',
      sort_order: 'asc'
    })
    const nextAccounts = (response.items || [])
      .map(options.enrichFallbackAccount)
      .filter((account) => filters.status === 'all' || account.status_kind === filters.status)
    const nextPools = options.groupAccounts(nextAccounts)
    accounts.value = nextAccounts
    pools.value = nextPools
    serverSummary.value = options.buildSummary(nextAccounts, nextPools)
    sourceInfo.value = {
      total: response.total,
      returned: response.items?.length || 0,
      limit: 1000,
      truncated: response.total > (response.items?.length || 0)
    }
    generatedAt.value = new Date().toISOString()
    mergeKnownPlatforms(nextAccounts)
    options.afterLoad?.(nextAccounts, nextPools)
  }

  function mergeKnownPlatforms(items: AccountPoolMapAccount[]) {
    const set = new Set(knownPlatforms.value)
    for (const account of items) {
      if (account.platform) set.add(account.platform)
    }
    knownPlatforms.value = Array.from(set).sort()
  }

  return {
    accounts,
    pools,
    serverSummary,
    sourceInfo,
    generatedAt,
    errorMessage,
    knownPlatforms,
    loading,
    sourceMode,
    usingFallback,
    refresh
  }
}
