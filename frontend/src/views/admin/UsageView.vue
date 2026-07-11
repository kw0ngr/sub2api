<template>
  <AppLayout>
    <div class="space-y-6">
      <UsageStatsCards :stats="usageStats" />
      <!-- Charts Section -->
      <div class="space-y-4">
        <div class="card p-4">
          <div class="flex flex-wrap items-center gap-4">
            <div class="flex items-center gap-2">
              <span class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('admin.dashboard.timeRange') }}:</span>
              <DateRangePicker
                v-model:start-date="startDate"
                v-model:end-date="endDate"
                @change="onDateRangeChange"
              />
            </div>
            <div class="ml-auto flex items-center gap-2">
              <span class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('admin.dashboard.granularity') }}:</span>
              <div class="w-28">
                <Select v-model="granularity" :options="granularityOptions" @change="loadChartData" />
              </div>
            </div>
          </div>
        </div>
        <div class="grid grid-cols-1 gap-6 lg:grid-cols-2">
          <ModelDistributionChart
            v-model:source="modelDistributionSource"
            v-model:metric="modelDistributionMetric"
            :model-stats="requestedModelStats"
            :upstream-model-stats="upstreamModelStats"
            :mapping-model-stats="mappingModelStats"
            :loading="modelStatsLoading"
            :show-source-toggle="true"
            :show-metric-toggle="true"
            :start-date="startDate"
            :end-date="endDate"
            :filters="breakdownFilters"
          />
          <GroupDistributionChart
            v-model:metric="groupDistributionMetric"
            :group-stats="groupStats"
            :loading="chartsLoading"
            :show-metric-toggle="true"
            :start-date="startDate"
            :end-date="endDate"
            :filters="breakdownFilters"
          />
        </div>
        <div class="grid grid-cols-1 gap-6 lg:grid-cols-2">
          <EndpointDistributionChart
            v-model:source="endpointDistributionSource"
            v-model:metric="endpointDistributionMetric"
            :endpoint-stats="inboundEndpointStats"
            :upstream-endpoint-stats="upstreamEndpointStats"
            :endpoint-path-stats="endpointPathStats"
            :loading="endpointStatsLoading"
            :show-source-toggle="true"
            :show-metric-toggle="true"
            :title="t('usage.endpointDistribution')"
            :start-date="startDate"
            :end-date="endDate"
            :filters="breakdownFilters"
          />
          <TokenUsageTrend :trend-data="trendData" :loading="chartsLoading" />
        </div>
      </div>
      <UsageFilters v-model="filters" :start-date="startDate" :end-date="endDate" :exporting="exporting" :model-options="modelNameOptions" @change="applyFilters" @refresh="refreshData" @reset="resetFilters" @cleanup="openCleanupDialog" @export="exportToExcel">
        <template #after-reset>
          <div class="relative" ref="columnDropdownRef">
            <button
              @click="showColumnDropdown = !showColumnDropdown"
              class="btn btn-secondary px-2 md:px-3"
              :title="t('admin.users.columnSettings')"
            >
              <svg class="h-4 w-4 md:mr-1.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5">
                <path stroke-linecap="round" stroke-linejoin="round" d="M9 4.5v15m6-15v15m-10.875 0h15.75c.621 0 1.125-.504 1.125-1.125V5.625c0-.621-.504-1.125-1.125-1.125H4.125C3.504 4.5 3 5.004 3 5.625v12.75c0 .621.504 1.125 1.125 1.125z" />
              </svg>
              <span class="hidden md:inline">{{ t('admin.users.columnSettings') }}</span>
            </button>
            <div
              v-if="showColumnDropdown"
              class="absolute right-0 top-full z-50 mt-1 max-h-80 w-48 overflow-y-auto rounded-lg border border-gray-200 bg-white py-1 shadow-lg dark:border-dark-600 dark:bg-dark-800"
            >
              <button
                v-for="col in toggleableColumns"
                :key="col.key"
                @click="toggleColumn(col.key)"
                class="flex w-full items-center justify-between px-4 py-2 text-left text-sm text-gray-700 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-dark-700"
              >
                <span>{{ col.label }}</span>
                <Icon
                  v-if="isColumnVisible(col.key)"
                  name="check"
                  size="sm"
                  class="text-primary-500"
                  :stroke-width="2"
                />
              </button>
            </div>
          </div>
        </template>
      </UsageFilters>
      <GatewaySignalPreview
        v-if="showGatewaySignalPreview"
        :logs="usageLogs"
        :stats="usageStats"
        :endpoint-stats="upstreamEndpointStats"
      />
      <div class="card overflow-hidden">
        <div class="border-b border-gray-100 px-4 py-3 dark:border-dark-700/50 sm:px-6">
          <h2 class="text-sm font-semibold text-gray-800 dark:text-gray-100">
            {{ t('admin.usage.tokenRanking.title') }}
          </h2>
        </div>
        <UserTokenRanking
          ref="userTokenRankingRef"
          :start-date="startDate"
          :end-date="endDate"
          :filters="breakdownFilters"
          :model="filters.model"
          @select-user="handleRankingSelectUser"
        />
      </div>
      <UsageTable
        :data="usageLogs"
        :loading="loading"
        :columns="visibleColumns"
        :server-side-sort="true"
        :default-sort-key="'created_at'"
        :default-sort-order="'desc'"
        @sort="handleSort"
        @userClick="handleUserClick"
        @replay="openReplayLab"
      />
      <Pagination v-if="pagination.total > 0" :page="pagination.page" :total="pagination.total" :page-size="pagination.page_size" @update:page="handlePageChange" @update:pageSize="handlePageSizeChange" />
    </div>
  </AppLayout>
  <UsageExportProgress :show="exportProgress.show" :progress="exportProgress.progress" :current="exportProgress.current" :total="exportProgress.total" :estimated-time="exportProgress.estimatedTime" @cancel="cancelExport" />
  <UsageCleanupDialog
    :show="cleanupDialogVisible"
    :filters="filters"
    :start-date="startDate"
    :end-date="endDate"
    @close="cleanupDialogVisible = false"
  />
  <!-- Balance history modal triggered from usage table user click -->
  <UserBalanceHistoryModal
    :show="showBalanceHistoryModal"
    :user="balanceHistoryUser"
    :hide-actions="true"
    @close="showBalanceHistoryModal = false; balanceHistoryUser = null"
  />
  <Teleport to="body">
    <div
      v-if="replayLog"
      class="fixed inset-0 z-[1000] flex justify-end bg-black/35"
      @click.self="closeReplayLab"
    >
      <aside class="h-full w-full max-w-xl overflow-y-auto bg-white shadow-2xl dark:bg-dark-900">
        <div class="border-b border-gray-100 p-5 dark:border-dark-800">
          <div class="flex items-start justify-between gap-4">
            <div>
              <p class="text-xs font-semibold uppercase tracking-[0.2em] text-violet-600 dark:text-violet-400">Replay Lab</p>
              <h2 class="mt-2 text-xl font-bold text-gray-950 dark:text-white">回放预检</h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                当前版本先做低风险 dry-run 摘要，不直接重放请求体，避免误计费和敏感内容落库。
              </p>
              <p v-if="replayLoading" class="mt-2 text-xs font-medium text-violet-600 dark:text-violet-300">
                正在加载后端诊断包...
              </p>
              <p v-else-if="replayError" class="mt-2 text-xs font-medium text-amber-600 dark:text-amber-300">
                {{ replayError }}
              </p>
            </div>
            <button class="rounded-lg p-2 text-gray-400 transition hover:bg-gray-100 hover:text-gray-700 dark:hover:bg-dark-800 dark:hover:text-gray-200" @click="closeReplayLab">
              ✕
            </button>
          </div>
        </div>

        <div class="space-y-4 p-5">
          <div class="rounded-2xl border border-gray-100 bg-gray-50 p-4 dark:border-dark-800 dark:bg-dark-800/60">
            <div class="flex flex-wrap items-center justify-between gap-3">
              <div>
                <p class="text-sm font-semibold text-gray-900 dark:text-white">{{ replayLog.model }}</p>
                <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">Request #{{ replayLog.request_id || replayLog.id }}</p>
              </div>
              <span class="rounded-full bg-violet-100 px-2.5 py-1 text-xs font-semibold text-violet-700 dark:bg-violet-500/15 dark:text-violet-300">
                {{ replayLog.stream ? 'stream' : 'sync' }}
              </span>
            </div>
          </div>

          <div class="grid gap-3 sm:grid-cols-2">
            <ReplayTile label="用户" :value="replayLog.user?.email || `#${replayLog.user_id}`" />
            <ReplayTile label="API Key" :value="replayLog.api_key?.name || `#${replayLog.api_key_id}`" />
            <ReplayTile label="上游账号" :value="replayLog.account?.name || (replayLog.account_id ? `#${replayLog.account_id}` : '-')" />
            <ReplayTile label="耗时" :value="`${replayLog.duration_ms || 0}ms`" />
          </div>

          <div v-if="replayPackage" class="rounded-2xl border border-violet-100 bg-violet-50/60 p-4 dark:border-violet-500/20 dark:bg-violet-500/10">
            <div class="flex flex-wrap items-center justify-between gap-3">
              <div>
                <h3 class="font-semibold text-gray-950 dark:text-white">路由解释</h3>
                <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                  {{ replayPackage.route.request_type }} · {{ replayPackage.route.inbound_endpoint || 'unknown endpoint' }}
                </p>
              </div>
              <span class="rounded-full bg-white px-2.5 py-1 text-xs font-semibold text-violet-700 shadow-sm dark:bg-dark-900 dark:text-violet-300">
                {{ replayPackage.safety.risk_level }}
              </span>
            </div>
            <div class="mt-4 grid gap-2 text-sm sm:grid-cols-2">
              <ReplayTile label="入口 endpoint" :value="replayPackage.route.inbound_endpoint || '-'" />
              <ReplayTile label="上游 endpoint" :value="replayPackage.route.upstream_endpoint || '-'" />
              <ReplayTile label="请求模型" :value="replayPackage.route.requested_model || '-'" />
              <ReplayTile label="上游模型" :value="replayPackage.route.upstream_model || '-'" />
              <ReplayTile label="映射链" :value="replayPackage.route.model_mapping_chain || '未映射'" />
              <ReplayTile label="原账号状态" :value="formatReplayAccountState" />
            </div>
          </div>

          <div v-if="replayPackage?.checks?.length" class="rounded-2xl border border-gray-100 p-4 dark:border-dark-800">
            <h3 class="font-semibold text-gray-950 dark:text-white">预检项</h3>
            <div class="mt-3 space-y-2">
              <div
                v-for="check in replayPackage.checks"
                :key="check.key"
                class="flex items-start gap-3 rounded-xl bg-gray-50 p-3 dark:bg-dark-800/70"
              >
                <span class="mt-0.5 rounded-full px-2 py-0.5 text-[11px] font-bold uppercase" :class="replayCheckClass(check.status)">
                  {{ check.status }}
                </span>
                <div class="min-w-0">
                  <p class="text-sm font-semibold text-gray-900 dark:text-white">{{ check.label }}</p>
                  <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{{ check.message }}</p>
                </div>
              </div>
            </div>
          </div>

          <div class="rounded-2xl border border-gray-100 p-4 dark:border-dark-800">
            <h3 class="font-semibold text-gray-950 dark:text-white">Dry-run 结论</h3>
            <ul class="mt-3 space-y-2 text-sm text-gray-600 dark:text-gray-300">
              <template v-if="replayPackage?.safety?.reasons?.length">
                <li v-for="reason in replayPackage.safety.reasons" :key="reason">· {{ reason }}</li>
              </template>
              <template v-else>
                <li>· 可基于这条日志定位用户、Key、模型、上游账号和 endpoint。</li>
                <li>· 当前日志没有保存原始请求体，暂不直接真实回放，避免敏感内容和重复费用。</li>
                <li>· 下一步会接入 snapshot 后支持“原路径回放 / 换健康账号回放 / 生成 curl”。</li>
              </template>
            </ul>
          </div>

          <div v-if="replayPackage?.curl_template" class="rounded-2xl border border-gray-100 p-4 dark:border-dark-800">
            <div class="flex flex-wrap items-center justify-between gap-3">
              <h3 class="font-semibold text-gray-950 dark:text-white">curl 模板</h3>
              <button class="text-xs font-semibold text-violet-600 hover:text-violet-700 dark:text-violet-300" type="button" @click="copyReplayCurl">
                复制模板
              </button>
            </div>
            <pre class="mt-3 max-h-72 overflow-auto rounded-xl bg-gray-950 p-3 text-xs text-gray-100">{{ replayPackage.curl_template }}</pre>
          </div>

          <div class="rounded-2xl border border-gray-100 p-4 dark:border-dark-800">
            <h3 class="font-semibold text-gray-950 dark:text-white">诊断摘要</h3>
            <pre class="mt-3 max-h-56 overflow-auto rounded-xl bg-gray-950 p-3 text-xs text-gray-100">{{ replaySummary }}</pre>
          </div>

          <div class="grid gap-2">
            <button class="btn btn-primary" type="button" @click="copyReplaySummary">
              复制诊断摘要
            </button>
            <button class="btn btn-secondary" type="button" @click="closeReplayLab">
              关闭
            </button>
          </div>
        </div>
      </aside>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { defineComponent, h, ref, reactive, computed, onMounted, onUnmounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { saveAs } from 'file-saver'
import { useRoute } from 'vue-router'
import { useAppStore } from '@/stores/app'; import { adminAPI } from '@/api/admin'; import { adminUsageAPI } from '@/api/admin/usage'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { formatReasoningEffort } from '@/utils/format'
import { resolveUsageRequestType, requestTypeToLegacyStream } from '@/utils/usageRequestType'
import AppLayout from '@/components/layout/AppLayout.vue'; import Pagination from '@/components/common/Pagination.vue'; import Select from '@/components/common/Select.vue'; import DateRangePicker from '@/components/common/DateRangePicker.vue'
import UsageStatsCards from '@/components/admin/usage/UsageStatsCards.vue'; import UsageFilters from '@/components/admin/usage/UsageFilters.vue'
import UsageTable from '@/components/admin/usage/UsageTable.vue'; import UsageExportProgress from '@/components/admin/usage/UsageExportProgress.vue'
import UserTokenRanking from '@/components/admin/usage/UserTokenRanking.vue'
import UsageCleanupDialog from '@/components/admin/usage/UsageCleanupDialog.vue'
import GatewaySignalPreview from '@/components/admin/usage/GatewaySignalPreview.vue'
import UserBalanceHistoryModal from '@/components/admin/user/UserBalanceHistoryModal.vue'
import ModelDistributionChart from '@/components/charts/ModelDistributionChart.vue'; import GroupDistributionChart from '@/components/charts/GroupDistributionChart.vue'; import TokenUsageTrend from '@/components/charts/TokenUsageTrend.vue'
import EndpointDistributionChart from '@/components/charts/EndpointDistributionChart.vue'
import Icon from '@/components/icons/Icon.vue'
import type { AdminUsageLog, TrendDataPoint, ModelStat, GroupStat, EndpointStat, AdminUser } from '@/types'; import type { AdminUsageStatsResponse, AdminUsageQueryParams, ReplayLabPackage } from '@/api/admin/usage'

const { t } = useI18n()
const appStore = useAppStore()
const ReplayTile = defineComponent({
  props: {
    label: { type: String, required: true },
    value: { type: String, required: true }
  },
  setup(props) {
    return () => h('div', { class: 'rounded-2xl border border-gray-100 bg-white p-4 dark:border-dark-800 dark:bg-dark-900' }, [
      h('p', { class: 'text-xs text-gray-500 dark:text-gray-400' }, props.label),
      h('p', { class: 'mt-1 truncate text-sm font-semibold text-gray-950 dark:text-white', title: props.value }, props.value)
    ])
  }
})
type DistributionMetric = 'tokens' | 'actual_cost'
type EndpointSource = 'inbound' | 'upstream' | 'path'
type ModelDistributionSource = 'requested' | 'upstream' | 'mapping'
const route = useRoute()
const usageStats = ref<AdminUsageStatsResponse | null>(null); const usageLogs = ref<AdminUsageLog[]>([]); const loading = ref(false); const exporting = ref(false)
const trendData = ref<TrendDataPoint[]>([]); const requestedModelStats = ref<ModelStat[]>([]); const upstreamModelStats = ref<ModelStat[]>([]); const mappingModelStats = ref<ModelStat[]>([]); const groupStats = ref<GroupStat[]>([]); const chartsLoading = ref(false); const modelStatsLoading = ref(false); const granularity = ref<'day' | 'hour'>('hour')
const modelDistributionMetric = ref<DistributionMetric>('tokens')
const modelDistributionSource = ref<ModelDistributionSource>('requested')
const loadedModelSources = reactive<Record<ModelDistributionSource, boolean>>({
  requested: false,
  upstream: false,
  mapping: false,
})
const groupDistributionMetric = ref<DistributionMetric>('tokens')
const endpointDistributionMetric = ref<DistributionMetric>('tokens')
const endpointDistributionSource = ref<EndpointSource>('inbound')
const inboundEndpointStats = ref<EndpointStat[]>([])
const upstreamEndpointStats = ref<EndpointStat[]>([])
const endpointPathStats = ref<EndpointStat[]>([])
const endpointStatsLoading = ref(false)
let abortController: AbortController | null = null; let exportAbortController: AbortController | null = null
let chartReqSeq = 0
let statsReqSeq = 0
let modelStatsReqSeq = 0
const exportProgress = reactive({ show: false, progress: 0, current: 0, total: 0, estimatedTime: '' })
const cleanupDialogVisible = ref(false)
const replayLog = ref<AdminUsageLog | null>(null)
const replayPackage = ref<ReplayLabPackage | null>(null)
const replayLoading = ref(false)
const replayError = ref('')
// Balance history modal state
const showBalanceHistoryModal = ref(false)
const balanceHistoryUser = ref<AdminUser | null>(null)
const showGatewaySignalPreview = computed(() => {
  return Boolean((usageStats.value?.total_requests || 0) > 0 || usageLogs.value.length > 0)
})

const replaySummary = computed(() => {
  if (replayPackage.value) {
    return JSON.stringify({
      summary: replayPackage.value.summary,
      route: replayPackage.value.route,
      safety: replayPackage.value.safety,
      checks: replayPackage.value.checks,
      generated_at: replayPackage.value.generated_at
    }, null, 2)
  }
  if (!replayLog.value) return ''
  return JSON.stringify({
    usage_log_id: replayLog.value.id,
    request_id: replayLog.value.request_id,
    user_id: replayLog.value.user_id,
    api_key_id: replayLog.value.api_key_id,
    account_id: replayLog.value.account_id,
    model: replayLog.value.model,
    upstream_model: replayLog.value.upstream_model,
    inbound_endpoint: replayLog.value.inbound_endpoint,
    upstream_endpoint: replayLog.value.upstream_endpoint,
    stream: replayLog.value.stream,
    request_type: replayLog.value.request_type,
    duration_ms: replayLog.value.duration_ms,
    first_token_ms: replayLog.value.first_token_ms,
    user_agent: replayLog.value.user_agent,
    ip_address: replayLog.value.ip_address,
    created_at: replayLog.value.created_at
  }, null, 2)
})

const formatReplayAccountState = computed(() => {
  const route = replayPackage.value?.route
  if (!route) return '-'
  const parts = [
    route.account_name || (route.account_id ? `#${route.account_id}` : '-'),
    route.account_platform,
    route.account_status,
    route.account_schedulable === false ? '不可调度' : route.account_schedulable === true ? '可调度' : ''
  ].filter(Boolean)
  return parts.join(' · ')
})

const breakdownFilters = computed(() => {
  const f: Record<string, any> = {}
  if (filters.value.user_id) f.user_id = filters.value.user_id
  if (filters.value.api_key_id) f.api_key_id = filters.value.api_key_id
  if (filters.value.account_id) f.account_id = filters.value.account_id
  if (filters.value.group_id) f.group_id = filters.value.group_id
  if (filters.value.request_type != null) f.request_type = filters.value.request_type
  if (filters.value.billing_type != null) f.billing_type = filters.value.billing_type
  return f
})

const modelNameOptions = computed(() =>
  Array.from(new Set(requestedModelStats.value.map((m) => m.model).filter(Boolean))).sort()
)

const handleUserClick = async (userId: number) => {
  try {
    const user = await adminAPI.users.getById(userId, true)
    balanceHistoryUser.value = user
    showBalanceHistoryModal.value = true
  } catch {
    appStore.showError(t('admin.usage.failedToLoadUser'))
  }
}

const handleRankingSelectUser = (userId: number) => {
  filters.value = { ...filters.value, user_id: userId }
  applyFilters()
}

async function openReplayLab(row: AdminUsageLog) {
  replayLog.value = row
  replayPackage.value = null
  replayError.value = ''
  replayLoading.value = true
  try {
    const usageLogID = row.id
    const pkg = await adminUsageAPI.getReplayPackage(usageLogID)
    if (replayLog.value?.id !== usageLogID) return
    replayPackage.value = pkg
    if (pkg.usage) {
      replayLog.value = pkg.usage
    }
  } catch (error) {
    if (replayLog.value?.id !== row.id) return
    replayError.value = normalizeReplayError(error) || '后端诊断包加载失败，当前展示列表内已有摘要。'
  } finally {
    if (replayLog.value?.id === row.id) {
      replayLoading.value = false
    }
  }
}

function closeReplayLab() {
  replayLog.value = null
  replayPackage.value = null
  replayError.value = ''
  replayLoading.value = false
}

async function copyReplaySummary() {
  try {
    await navigator.clipboard.writeText(replaySummary.value)
    appStore.showSuccess('Replay 诊断摘要已复制')
  } catch {
    appStore.showError('复制失败，请手动复制摘要')
  }
}

async function copyReplayCurl() {
  if (!replayPackage.value?.curl_template) return
  try {
    await navigator.clipboard.writeText(replayPackage.value.curl_template)
    appStore.showSuccess('Replay curl 模板已复制')
  } catch {
    appStore.showError('复制失败，请手动复制 curl 模板')
  }
}

function replayCheckClass(status: string): string {
  if (status === 'ok') return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300'
  if (status === 'blocked') return 'bg-rose-100 text-rose-700 dark:bg-rose-500/15 dark:text-rose-300'
  return 'bg-amber-100 text-amber-700 dark:bg-amber-500/15 dark:text-amber-300'
}

function normalizeReplayError(error: unknown): string {
  if (error && typeof error === 'object' && 'message' in error) {
    return String((error as { message?: unknown }).message || '')
  }
  return ''
}

const granularityOptions = computed(() => [{ value: 'day', label: t('admin.dashboard.day') }, { value: 'hour', label: t('admin.dashboard.hour') }])
// Use local timezone to avoid UTC timezone issues
const formatLD = (d: Date) => {
  const year = d.getFullYear()
  const month = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}
const getLast24HoursRangeDates = (): { start: string; end: string } => {
  const end = new Date()
  const start = new Date(end.getTime() - 24 * 60 * 60 * 1000)
  return {
    start: formatLD(start),
    end: formatLD(end)
  }
}
const getGranularityForRange = (start: string, end: string): 'day' | 'hour' => {
  const startTime = new Date(`${start}T00:00:00`).getTime()
  const endTime = new Date(`${end}T00:00:00`).getTime()
  const daysDiff = Math.ceil((endTime - startTime) / (1000 * 60 * 60 * 24))
  return daysDiff <= 1 ? 'hour' : 'day'
}
const defaultRange = getLast24HoursRangeDates()
const startDate = ref(defaultRange.start); const endDate = ref(defaultRange.end)
const filters = ref<AdminUsageQueryParams>({ user_id: undefined, model: undefined, group_id: undefined, request_type: undefined, billing_type: null, start_date: startDate.value, end_date: endDate.value })
const pagination = reactive({ page: 1, page_size: getPersistedPageSize(), total: 0 })
const sortState = reactive({
  sort_by: 'created_at',
  sort_order: 'desc' as 'asc' | 'desc'
})

const getSingleQueryValue = (value: string | null | Array<string | null> | undefined): string | undefined => {
  if (Array.isArray(value)) return value.find((item): item is string => typeof item === 'string' && item.length > 0)
  return typeof value === 'string' && value.length > 0 ? value : undefined
}

const getNumericQueryValue = (value: string | null | Array<string | null> | undefined): number | undefined => {
  const raw = getSingleQueryValue(value)
  if (!raw) return undefined
  const parsed = Number(raw)
  return Number.isFinite(parsed) ? parsed : undefined
}

const applyRouteQueryFilters = () => {
  const queryStartDate = getSingleQueryValue(route.query.start_date)
  const queryEndDate = getSingleQueryValue(route.query.end_date)
  const queryUserId = getNumericQueryValue(route.query.user_id)

  if (queryStartDate) {
    startDate.value = queryStartDate
  }
  if (queryEndDate) {
    endDate.value = queryEndDate
  }

  filters.value = {
    ...filters.value,
    user_id: queryUserId,
    start_date: startDate.value,
    end_date: endDate.value
  }
  granularity.value = getGranularityForRange(startDate.value, endDate.value)
}

const onDateRangeChange = (range: { startDate: string; endDate: string; preset: string | null }) => {
  startDate.value = range.startDate
  endDate.value = range.endDate
  filters.value = {
    ...filters.value,
    start_date: range.startDate,
    end_date: range.endDate
  }
  granularity.value = getGranularityForRange(range.startDate, range.endDate)
  applyFilters()
}

const buildUsageListParams = (
  page: number,
  pageSize: number,
  exactTotal: boolean
): AdminUsageQueryParams => {
  const requestType = filters.value.request_type
  const legacyStream = requestType ? requestTypeToLegacyStream(requestType) : filters.value.stream
  return {
    page,
    page_size: pageSize,
    exact_total: exactTotal,
    ...filters.value,
    stream: legacyStream === null ? undefined : legacyStream,
    sort_by: sortState.sort_by,
    sort_order: sortState.sort_order
  }
}

const loadLogs = async () => {
  abortController?.abort(); const c = new AbortController(); abortController = c; loading.value = true
  try {
    const res = await adminAPI.usage.list(
      buildUsageListParams(pagination.page, pagination.page_size, false),
      { signal: c.signal }
    )
    if(!c.signal.aborted) { usageLogs.value = res.items; pagination.total = res.total }
  } catch (error: any) { if(error?.name !== 'AbortError') console.error('Failed to load usage logs:', error) } finally { if(abortController === c) loading.value = false }
}
const loadStats = async (force = false) => {
  const seq = ++statsReqSeq
  endpointStatsLoading.value = true
  try {
    const requestType = filters.value.request_type
    const legacyStream = requestType ? requestTypeToLegacyStream(requestType) : filters.value.stream
    const s = await adminAPI.usage.getStats({
      ...filters.value,
      stream: legacyStream === null ? undefined : legacyStream,
      ...(force ? { nocache: 1 } : {})
    })
    if (seq !== statsReqSeq) return
    usageStats.value = s
    inboundEndpointStats.value = s.endpoints || []
    upstreamEndpointStats.value = s.upstream_endpoints || []
    endpointPathStats.value = s.endpoint_paths || []
  } catch (error) {
    if (seq !== statsReqSeq) return
    console.error('Failed to load usage stats:', error)
    inboundEndpointStats.value = []
    upstreamEndpointStats.value = []
    endpointPathStats.value = []
  } finally {
    if (seq === statsReqSeq) endpointStatsLoading.value = false
  }
}

const invalidateModelStatsCache = () => {
  loadedModelSources.requested = false
  loadedModelSources.upstream = false
  loadedModelSources.mapping = false
}

const loadModelStats = async (source: ModelDistributionSource, force = false) => {
  if (!force && loadedModelSources[source]) {
    return
  }

  const seq = ++modelStatsReqSeq
  modelStatsLoading.value = true
  try {
    const requestType = filters.value.request_type
    const legacyStream = requestType ? requestTypeToLegacyStream(requestType) : filters.value.stream
    const baseParams = {
      start_date: filters.value.start_date || startDate.value,
      end_date: filters.value.end_date || endDate.value,
      user_id: filters.value.user_id,
      model: filters.value.model,
      api_key_id: filters.value.api_key_id,
      account_id: filters.value.account_id,
      group_id: filters.value.group_id,
      request_type: requestType,
      stream: legacyStream === null ? undefined : legacyStream,
      billing_type: filters.value.billing_type,
    }

    const response = await adminAPI.dashboard.getModelStats({ ...baseParams, model_source: source })

    if (seq !== modelStatsReqSeq) return

    const models = response.models || []
    if (source === 'requested') {
      requestedModelStats.value = models
    } else if (source === 'upstream') {
      upstreamModelStats.value = models
    } else {
      mappingModelStats.value = models
    }
    loadedModelSources[source] = true
  } catch (error) {
    if (seq !== modelStatsReqSeq) return
    console.error('Failed to load model stats:', error)
    if (source === 'requested') {
      requestedModelStats.value = []
    } else if (source === 'upstream') {
      upstreamModelStats.value = []
    } else {
      mappingModelStats.value = []
    }
    loadedModelSources[source] = false
  } finally {
    if (seq === modelStatsReqSeq) modelStatsLoading.value = false
  }
}

const loadChartData = async () => {
  const seq = ++chartReqSeq
  chartsLoading.value = true
  try {
    const requestType = filters.value.request_type
    const legacyStream = requestType ? requestTypeToLegacyStream(requestType) : filters.value.stream
    const snapshot = await adminAPI.dashboard.getSnapshotV2({
      start_date: filters.value.start_date || startDate.value,
      end_date: filters.value.end_date || endDate.value,
      granularity: granularity.value,
      user_id: filters.value.user_id,
      model: filters.value.model,
      api_key_id: filters.value.api_key_id,
      account_id: filters.value.account_id,
      group_id: filters.value.group_id,
      request_type: requestType,
      stream: legacyStream === null ? undefined : legacyStream,
      billing_type: filters.value.billing_type,
      include_stats: false,
      include_trend: true,
      include_model_stats: false,
      include_group_stats: true,
      include_users_trend: false
    })
    if (seq !== chartReqSeq) return
    trendData.value = snapshot.trend || []
    groupStats.value = snapshot.groups || []
  } catch (error) { console.error('Failed to load chart data:', error) } finally { if (seq === chartReqSeq) chartsLoading.value = false }
}
const applyFilters = () => {
  pagination.page = 1
  invalidateModelStatsCache()
  loadLogs()
  loadStats()
  loadModelStats(modelDistributionSource.value, true)
  loadChartData()
}
const userTokenRankingRef = ref<{ reload: () => void } | null>(null)

const refreshData = () => {
  invalidateModelStatsCache()
  loadLogs()
  loadStats(true)
  loadModelStats(modelDistributionSource.value, true)
  loadChartData()
  userTokenRankingRef.value?.reload()
}
const resetFilters = () => {
  const range = getLast24HoursRangeDates()
  startDate.value = range.start
  endDate.value = range.end
  filters.value = { start_date: startDate.value, end_date: endDate.value, request_type: undefined, billing_type: null, billing_mode: undefined }
  granularity.value = getGranularityForRange(startDate.value, endDate.value)
  applyFilters()
}
const handlePageChange = (p: number) => { pagination.page = p; loadLogs() }
const handlePageSizeChange = (s: number) => { pagination.page_size = s; pagination.page = 1; loadLogs() }
const handleSort = (key: string, order: 'asc' | 'desc') => {
  sortState.sort_by = key
  sortState.sort_order = order
  pagination.page = 1
  loadLogs()
}
const cancelExport = () => exportAbortController?.abort()
const openCleanupDialog = () => { cleanupDialogVisible.value = true }
const getRequestTypeLabel = (log: AdminUsageLog): string => {
  const requestType = resolveUsageRequestType(log)
  if (requestType === 'ws_v2') return t('usage.ws')
  if (requestType === 'stream') return t('usage.stream')
  if (requestType === 'sync') return t('usage.sync')
  return t('usage.unknown')
}

const exportToExcel = async () => {
  if (exporting.value) return; exporting.value = true; exportProgress.show = true
  const c = new AbortController(); exportAbortController = c
  try {
    let p = 1; let total = pagination.total; let exportedCount = 0
    const XLSX = await import('xlsx')
    const headers = [
      t('usage.time'), t('admin.usage.user'), t('usage.apiKeyFilter'),
      t('admin.usage.account'), t('usage.model'), t('usage.upstreamModel'), t('usage.reasoningEffort'), t('admin.usage.group'),
      t('usage.inboundEndpoint'), t('usage.upstreamEndpoint'),
      t('usage.type'),
      t('admin.usage.inputTokens'), t('admin.usage.outputTokens'),
      t('admin.usage.cacheReadTokens'), t('admin.usage.cacheCreationTokens'),
      t('admin.usage.inputCost'), t('admin.usage.outputCost'),
      t('admin.usage.cacheReadCost'), t('admin.usage.cacheCreationCost'),
      t('usage.rate'), t('usage.accountMultiplier'), t('usage.original'), t('usage.userBilled'), t('usage.accountBilled'),
      t('usage.firstToken'), t('usage.duration'),
      t('admin.usage.requestId'), t('usage.userAgent'), t('admin.usage.ipAddress')
    ]
    const ws = XLSX.utils.aoa_to_sheet([headers])
    while (true) {
      const res = await adminUsageAPI.list(
        buildUsageListParams(p, 100, true),
        { signal: c.signal }
      )
      if (c.signal.aborted) break; if (p === 1) { total = res.total; exportProgress.total = total }
      const rows = (res.items || []).map((log: AdminUsageLog) => [
        log.created_at, log.user?.email || '', log.api_key?.name || '', log.account?.name || '', log.model,
        log.upstream_model || '', formatReasoningEffort(log.reasoning_effort), log.group?.name || '',
        log.inbound_endpoint || '', log.upstream_endpoint || '', getRequestTypeLabel(log),
        log.input_tokens, log.output_tokens, log.cache_read_tokens, log.cache_creation_tokens,
        log.input_cost?.toFixed(6) || '0.000000', log.output_cost?.toFixed(6) || '0.000000',
        log.cache_read_cost?.toFixed(6) || '0.000000', log.cache_creation_cost?.toFixed(6) || '0.000000',
        log.rate_multiplier?.toPrecision(4) || '1.00', (log.account_rate_multiplier ?? 1).toPrecision(4),
        log.total_cost?.toFixed(6) || '0.000000', log.actual_cost?.toFixed(6) || '0.000000',
        ((log.account_stats_cost ?? log.total_cost) * (log.account_rate_multiplier ?? 1)).toFixed(6), log.first_token_ms ?? '', log.duration_ms,
        log.request_id || '', log.user_agent || '', log.ip_address || ''
      ])
      if (rows.length) {
        XLSX.utils.sheet_add_aoa(ws, rows, { origin: -1 })
      }
      exportedCount += rows.length
      exportProgress.current = exportedCount
      exportProgress.progress = total > 0 ? Math.min(100, Math.round(exportedCount / total * 100)) : 0
      if (exportedCount >= total || res.items.length < 100) break; p++
    }
    if(!c.signal.aborted) {
      const wb = XLSX.utils.book_new()
      XLSX.utils.book_append_sheet(wb, ws, 'Usage')
      saveAs(new Blob([XLSX.write(wb, { bookType: 'xlsx', type: 'array' })], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' }), `usage_${filters.value.start_date}_to_${filters.value.end_date}.xlsx`)
      appStore.showSuccess(t('usage.exportSuccess'))
    }
  } catch (error) { console.error('Failed to export:', error); appStore.showError('Export Failed') }
  finally { if(exportAbortController === c) { exportAbortController = null; exporting.value = false; exportProgress.show = false } }
}

// Column visibility
const ALWAYS_VISIBLE = ['user', 'created_at', 'actions']
const DEFAULT_HIDDEN_COLUMNS = ['reasoning_effort', 'user_agent']
const HIDDEN_COLUMNS_KEY = 'usage-hidden-columns'

const allColumns = computed(() => [
  { key: 'user', label: t('admin.usage.user'), sortable: false },
  { key: 'api_key', label: t('usage.apiKeyFilter'), sortable: false },
  { key: 'account', label: t('admin.usage.account'), sortable: false },
  { key: 'model', label: t('usage.model'), sortable: true },
  { key: 'reasoning_effort', label: t('usage.reasoningEffort'), sortable: false },
  { key: 'endpoint', label: t('usage.endpoint'), sortable: false },
  { key: 'group', label: t('admin.usage.group'), sortable: false },
  { key: 'stream', label: t('usage.type'), sortable: false },
  { key: 'billing_mode', label: t('admin.usage.billingMode'), sortable: false },
  { key: 'tokens', label: t('usage.tokens'), sortable: false },
  { key: 'cost', label: t('usage.cost'), sortable: false },
  { key: 'first_token', label: t('usage.firstToken'), sortable: false },
  { key: 'duration', label: t('usage.duration'), sortable: false },
  { key: 'created_at', label: t('usage.time'), sortable: true },
  { key: 'user_agent', label: t('usage.userAgent'), sortable: false },
  { key: 'ip_address', label: t('admin.usage.ipAddress'), sortable: false },
  { key: 'actions', label: 'Trace', sortable: false }
])

const hiddenColumns = reactive<Set<string>>(new Set())

const toggleableColumns = computed(() =>
  allColumns.value.filter(col => !ALWAYS_VISIBLE.includes(col.key))
)

const visibleColumns = computed(() =>
  allColumns.value.filter(col =>
    ALWAYS_VISIBLE.includes(col.key) || !hiddenColumns.has(col.key)
  )
)

const isColumnVisible = (key: string) => !hiddenColumns.has(key)

const toggleColumn = (key: string) => {
  if (hiddenColumns.has(key)) {
    hiddenColumns.delete(key)
  } else {
    hiddenColumns.add(key)
  }
  try {
    localStorage.setItem(HIDDEN_COLUMNS_KEY, JSON.stringify([...hiddenColumns]))
  } catch (e) {
    console.error('Failed to save columns:', e)
  }
}

const loadSavedColumns = () => {
  try {
    const saved = localStorage.getItem(HIDDEN_COLUMNS_KEY)
    if (saved) {
      (JSON.parse(saved) as string[]).forEach((key) => {
        hiddenColumns.add(key)
      })
    } else {
      DEFAULT_HIDDEN_COLUMNS.forEach((key) => {
        hiddenColumns.add(key)
      })
    }
  } catch {
    DEFAULT_HIDDEN_COLUMNS.forEach((key) => {
      hiddenColumns.add(key)
    })
  }
}

const showColumnDropdown = ref(false)
const columnDropdownRef = ref<HTMLElement | null>(null)

const handleColumnClickOutside = (event: MouseEvent) => {
  if (columnDropdownRef.value && !columnDropdownRef.value.contains(event.target as HTMLElement)) {
    showColumnDropdown.value = false
  }
}

onMounted(() => {
  applyRouteQueryFilters()
  loadLogs()
  loadStats()
  loadModelStats(modelDistributionSource.value, true)
  window.setTimeout(() => {
    void loadChartData()
  }, 120)
  loadSavedColumns()
  document.addEventListener('click', handleColumnClickOutside)
})
onUnmounted(() => { abortController?.abort(); exportAbortController?.abort(); document.removeEventListener('click', handleColumnClickOutside) })

watch(modelDistributionSource, (source) => {
  void loadModelStats(source)
})

defineExpose({ requestedModelStats, refreshData })
</script>
