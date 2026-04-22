<template>
  <div class="card">
    <div class="flex items-center justify-between border-b border-gray-100 px-6 py-4 dark:border-dark-700">
      <div>
        <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
          {{ t('admin.ops.debugTraces.title') }}
        </h3>
        <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
          {{ t('admin.ops.debugTraces.subtitle') }}
        </p>
      </div>
      <div class="flex items-center gap-2">
        <button
          type="button"
          class="rounded-lg px-3 py-1.5 text-xs font-medium transition-colors"
          :class="onlyFallback ? 'bg-primary-600 text-white' : 'bg-gray-100 text-gray-700 hover:bg-gray-200 dark:bg-dark-700 dark:text-gray-200 dark:hover:bg-dark-600'"
          @click="toggleFallbackOnly"
        >
          {{ t('admin.ops.debugTraces.onlyFallback') }}
        </button>
        <button
          type="button"
          class="btn btn-secondary text-xs"
          :disabled="loading"
          @click="fetchRows"
        >
          {{ t('common.refresh') }}
        </button>
      </div>
    </div>

    <div class="flex items-center gap-3 border-b border-gray-100 px-6 py-3 dark:border-dark-700">
      <input
        v-model="requestId"
        type="text"
        class="input h-9 max-w-xs text-sm"
        :placeholder="t('admin.ops.debugTraces.requestIdPlaceholder')"
        @keyup.enter="applyFilters"
      >
      <input
        v-model="pathFilter"
        type="text"
        class="input h-9 max-w-xs text-sm"
        :placeholder="t('admin.ops.debugTraces.pathPlaceholder')"
        @keyup.enter="applyFilters"
      >
      <button
        type="button"
        class="rounded-lg bg-gray-100 px-3 py-1.5 text-xs font-medium text-gray-700 transition-colors hover:bg-gray-200 dark:bg-dark-700 dark:text-gray-200 dark:hover:bg-dark-600"
        @click="applyFilters"
      >
        {{ t('common.search') }}
      </button>
      <button
        v-if="requestId || pathFilter || onlyFallback"
        type="button"
        class="rounded-lg bg-gray-50 px-3 py-1.5 text-xs font-medium text-gray-500 transition-colors hover:bg-gray-100 dark:bg-dark-800 dark:text-gray-300 dark:hover:bg-dark-700"
        @click="resetFilters"
      >
        {{ t('common.reset') }}
      </button>
    </div>

    <div v-if="loading" class="flex items-center justify-center py-10">
      <div class="h-8 w-8 animate-spin rounded-full border-b-2 border-primary-600"></div>
    </div>

    <div v-else class="overflow-x-auto">
      <table class="min-w-full border-separate border-spacing-0">
        <thead class="bg-gray-50 dark:bg-dark-800">
          <tr>
            <th class="border-b border-gray-200 px-4 py-2.5 text-left text-[11px] font-bold uppercase tracking-wider text-gray-500 dark:border-dark-700 dark:text-dark-400">
              {{ t('admin.ops.debugTraces.time') }}
            </th>
            <th class="border-b border-gray-200 px-4 py-2.5 text-left text-[11px] font-bold uppercase tracking-wider text-gray-500 dark:border-dark-700 dark:text-dark-400">
              {{ t('admin.ops.debugTraces.reason') }}
            </th>
            <th class="border-b border-gray-200 px-4 py-2.5 text-left text-[11px] font-bold uppercase tracking-wider text-gray-500 dark:border-dark-700 dark:text-dark-400">
              {{ t('admin.ops.debugTraces.route') }}
            </th>
            <th class="border-b border-gray-200 px-4 py-2.5 text-left text-[11px] font-bold uppercase tracking-wider text-gray-500 dark:border-dark-700 dark:text-dark-400">
              {{ t('admin.ops.debugTraces.target') }}
            </th>
            <th class="border-b border-gray-200 px-4 py-2.5 text-left text-[11px] font-bold uppercase tracking-wider text-gray-500 dark:border-dark-700 dark:text-dark-400">
              {{ t('admin.ops.debugTraces.status') }}
            </th>
            <th class="border-b border-gray-200 px-4 py-2.5 text-left text-[11px] font-bold uppercase tracking-wider text-gray-500 dark:border-dark-700 dark:text-dark-400">
              {{ t('admin.ops.debugTraces.body') }}
            </th>
            <th class="border-b border-gray-200 px-4 py-2.5 text-right text-[11px] font-bold uppercase tracking-wider text-gray-500 dark:border-dark-700 dark:text-dark-400">
              {{ t('admin.ops.debugTraces.action') }}
            </th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
          <tr v-if="rows.length === 0">
            <td colspan="7" class="py-12 text-center text-sm text-gray-400 dark:text-dark-500">
              {{ t('common.noData') }}
            </td>
          </tr>

          <tr
            v-for="row in rows"
            :key="row.id"
            class="cursor-pointer transition-colors hover:bg-gray-50/80 dark:hover:bg-dark-800/50"
            @click="openDetail(row.id)"
          >
            <td class="whitespace-nowrap px-4 py-2 text-xs font-medium text-gray-900 dark:text-gray-200">
              {{ formatDateTime(row.created_at) }}
            </td>
            <td class="px-4 py-2">
              <div class="flex flex-col gap-1">
                <span class="text-xs font-semibold text-gray-900 dark:text-white">
                  {{ row.reason_code || row.error_type || '-' }}
                </span>
                <span class="max-w-[280px] truncate text-[11px] text-gray-500 dark:text-dark-400" :title="row.reason_hint || row.error_message || ''">
                  {{ row.reason_hint || row.error_message || '-' }}
                </span>
              </div>
            </td>
            <td class="px-4 py-2">
              <div class="flex flex-col gap-1">
                <span class="font-mono text-[11px] text-gray-700 dark:text-gray-300">
                  {{ row.path || '-' }}
                </span>
                <span class="font-mono text-[11px] text-gray-400 dark:text-dark-500">
                  {{ row.request_id || row.client_request_id || '-' }}
                </span>
              </div>
            </td>
            <td class="px-4 py-2">
              <div class="flex flex-col gap-1">
                <span class="text-xs font-medium text-gray-900 dark:text-white">
                  {{ row.platform || '-' }} / {{ row.account_id ?? '-' }}
                </span>
                <span class="max-w-[220px] truncate font-mono text-[11px] text-gray-500 dark:text-dark-400" :title="row.upstream_model || row.model || ''">
                  {{ row.upstream_model || row.model || '-' }}
                </span>
                <span v-if="row.fallback_triggered || row.account_switch_count" class="text-[11px] text-amber-600 dark:text-amber-400">
                  fallback={{ row.fallback_triggered ? 'yes' : 'no' }}, switch={{ row.account_switch_count ?? 0 }}
                </span>
              </div>
            </td>
            <td class="px-4 py-2">
              <div class="flex items-center gap-2">
                <span
                  :class="[
                    'inline-flex items-center rounded px-1.5 py-0.5 text-[10px] font-bold ring-1 ring-inset',
                    row.status_code >= 500
                      ? 'bg-red-50 text-red-700 ring-red-200 dark:bg-red-900/20 dark:text-red-300 dark:ring-red-800/60'
                      : row.status_code >= 400
                        ? 'bg-amber-50 text-amber-700 ring-amber-200 dark:bg-amber-900/20 dark:text-amber-300 dark:ring-amber-800/60'
                        : 'bg-emerald-50 text-emerald-700 ring-emerald-200 dark:bg-emerald-900/20 dark:text-emerald-300 dark:ring-emerald-800/60'
                  ]"
                >
                  {{ row.status_code }}
                </span>
                <span v-if="row.upstream_status_code" class="text-[11px] text-gray-500 dark:text-dark-400">
                  upstream {{ row.upstream_status_code }}
                </span>
              </div>
            </td>
            <td class="px-4 py-2">
              <div class="flex flex-col gap-1 text-[11px] text-gray-500 dark:text-dark-400">
                <span>{{ formatBytes(row.request_body_bytes) }}</span>
                <span v-if="row.request_body_preview_truncated" class="text-amber-600 dark:text-amber-400">
                  {{ t('admin.ops.debugTraces.truncated') }}
                </span>
              </div>
            </td>
            <td class="px-4 py-2 text-right">
              <button
                type="button"
                class="text-xs font-bold text-primary-600 hover:text-primary-700 dark:text-primary-400"
                @click.stop="openDetail(row.id)"
              >
                {{ t('admin.ops.errorLog.details') }}
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <BaseDialog
      :show="showDetail"
      :title="detailTitle"
      width="full"
      :close-on-click-outside="true"
      @close="closeDetail"
    >
      <div v-if="detailLoading" class="flex items-center justify-center py-16">
        <div class="h-8 w-8 animate-spin rounded-full border-b-2 border-primary-600"></div>
      </div>

      <div v-else-if="!selectedTrace" class="py-10 text-center text-sm text-gray-500 dark:text-dark-400">
        {{ t('common.noData') }}
      </div>

      <div v-else class="space-y-6 p-6">
        <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <div class="rounded-xl bg-gray-50 p-4 dark:bg-dark-900">
            <div class="text-xs font-bold uppercase tracking-wider text-gray-400">{{ t('admin.ops.debugTraces.reason') }}</div>
            <div class="mt-1 text-sm font-medium text-gray-900 dark:text-white">{{ selectedTrace.reason_code || selectedTrace.error_type || '-' }}</div>
          </div>
          <div class="rounded-xl bg-gray-50 p-4 dark:bg-dark-900">
            <div class="text-xs font-bold uppercase tracking-wider text-gray-400">{{ t('admin.ops.debugTraces.target') }}</div>
            <div class="mt-1 text-sm font-medium text-gray-900 dark:text-white">{{ selectedTrace.platform || '-' }} / {{ selectedTrace.account_id ?? '-' }}</div>
          </div>
          <div class="rounded-xl bg-gray-50 p-4 dark:bg-dark-900">
            <div class="text-xs font-bold uppercase tracking-wider text-gray-400">{{ t('admin.ops.debugTraces.scheduler') }}</div>
            <div class="mt-1 text-sm font-medium text-gray-900 dark:text-white">{{ selectedTrace.scheduler_layer || '-' }}</div>
          </div>
          <div class="rounded-xl bg-gray-50 p-4 dark:bg-dark-900">
            <div class="text-xs font-bold uppercase tracking-wider text-gray-400">{{ t('admin.ops.debugTraces.latency') }}</div>
            <div class="mt-1 text-sm font-medium text-gray-900 dark:text-white">
              {{ selectedTrace.upstream_latency_ms ?? '-' }}ms / {{ selectedTrace.time_to_first_token_ms ?? '-' }}ms
            </div>
          </div>
        </div>

        <div class="rounded-xl bg-gray-50 p-6 dark:bg-dark-900">
          <h4 class="text-sm font-black uppercase tracking-wider text-gray-900 dark:text-white">
            {{ t('admin.ops.debugTraces.requestPreview') }}
          </h4>
          <div v-if="selectedTrace.reason_hint" class="mt-3 rounded-lg bg-amber-50 p-3 text-sm text-amber-700 dark:bg-amber-900/20 dark:text-amber-300">
            {{ selectedTrace.reason_hint }}
          </div>
          <pre class="mt-4 max-h-[420px] overflow-auto rounded-xl border border-gray-200 bg-white p-4 text-xs text-gray-800 dark:border-dark-700 dark:bg-dark-800 dark:text-gray-100"><code>{{ prettyJSON(selectedTrace.request_body_preview_json || '') }}</code></pre>
          <div v-if="selectedTrace.request_body_truncated_paths?.length" class="mt-3 text-xs text-gray-500 dark:text-dark-400">
            {{ t('admin.ops.debugTraces.truncatedPaths') }}: {{ selectedTrace.request_body_truncated_paths.join(', ') }}
          </div>
        </div>

        <div class="rounded-xl bg-gray-50 p-6 dark:bg-dark-900">
          <h4 class="text-sm font-black uppercase tracking-wider text-gray-900 dark:text-white">
            {{ t('admin.ops.debugTraces.responsePreview') }}
          </h4>
          <pre class="mt-4 max-h-[280px] overflow-auto rounded-xl border border-gray-200 bg-white p-4 text-xs text-gray-800 dark:border-dark-700 dark:bg-dark-800 dark:text-gray-100"><code>{{ prettyJSON(selectedTrace.response_body_preview || '') }}</code></pre>
        </div>

        <div v-if="selectedTrace.upstream_errors?.length" class="rounded-xl bg-gray-50 p-6 dark:bg-dark-900">
          <h4 class="text-sm font-black uppercase tracking-wider text-gray-900 dark:text-white">
            {{ t('admin.ops.errorDetails.upstreamErrors') }}
          </h4>
          <div class="mt-4 space-y-3">
            <div
              v-for="(item, idx) in selectedTrace.upstream_errors"
              :key="`${selectedTrace.id}-${idx}`"
              class="rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800"
            >
              <div class="text-xs font-semibold text-gray-900 dark:text-white">
                #{{ idx + 1 }} {{ item.kind || '-' }} / {{ item.upstream_status_code ?? '-' }}
              </div>
              <div class="mt-2 text-sm text-gray-700 dark:text-gray-200">
                {{ item.message || item.detail || '-' }}
              </div>
              <pre v-if="item.upstream_response_body" class="mt-3 max-h-[180px] overflow-auto rounded-xl border border-gray-200 bg-gray-50 p-3 text-xs text-gray-800 dark:border-dark-700 dark:bg-dark-900 dark:text-gray-100"><code>{{ prettyJSON(item.upstream_response_body) }}</code></pre>
            </div>
          </div>
        </div>
      </div>
    </BaseDialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { opsAPI, type OpsDebugTrace } from '@/api/admin/ops'
import { useAppStore } from '@/stores'
import { formatDateTime } from '@/utils/format'

const props = withDefaults(defineProps<{
  platformFilter?: string
  refreshToken?: number
}>(), {
  platformFilter: '',
  refreshToken: 0
})

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const rows = ref<OpsDebugTrace[]>([])
const onlyFallback = ref(false)
const requestId = ref('')
const pathFilter = ref('/responses')

const showDetail = ref(false)
const detailLoading = ref(false)
const selectedTrace = ref<OpsDebugTrace | null>(null)

const detailTitle = computed(() => {
  if (!selectedTrace.value) return t('admin.ops.debugTraces.detailTitle')
  return `${t('admin.ops.debugTraces.detailTitle')} · ${selectedTrace.value.id}`
})

const buildQuery = () => {
  const query: Record<string, any> = {
    limit: 20,
    only_errors: true,
    only_fallback: onlyFallback.value
  }
  if (props.platformFilter?.trim()) query.platform = props.platformFilter.trim()
  if (requestId.value.trim()) query.request_id = requestId.value.trim()
  if (pathFilter.value.trim()) query.path = pathFilter.value.trim()
  return query
}

const fetchRows = async () => {
  loading.value = true
  try {
    const res = await opsAPI.listDebugTraces(buildQuery())
    rows.value = res.items || []
  } catch (err: any) {
    console.error('[OpsDebugTracePanel] Failed to load debug traces', err)
    appStore.showError(err?.response?.data?.detail || 'Debug traces 加载失败')
  } finally {
    loading.value = false
  }
}

const openDetail = async (id: string) => {
  if (!id) return
  detailLoading.value = true
  showDetail.value = true
  try {
    selectedTrace.value = await opsAPI.getDebugTrace(id)
  } catch (err: any) {
    console.error('[OpsDebugTracePanel] Failed to load debug trace detail', err)
    appStore.showError(err?.response?.data?.detail || 'Debug trace 详情加载失败')
    showDetail.value = false
  } finally {
    detailLoading.value = false
  }
}

const closeDetail = () => {
  showDetail.value = false
  selectedTrace.value = null
}

const toggleFallbackOnly = () => {
  onlyFallback.value = !onlyFallback.value
  fetchRows()
}

const applyFilters = () => {
  fetchRows()
}

const resetFilters = () => {
  requestId.value = ''
  pathFilter.value = '/responses'
  onlyFallback.value = false
  fetchRows()
}

const formatBytes = (value?: number | null) => {
  if (!value || value <= 0) return '-'
  if (value >= 1024 * 1024) return `${(value / (1024 * 1024)).toFixed(2)} MB`
  if (value >= 1024) return `${(value / 1024).toFixed(1)} KB`
  return `${value} B`
}

const prettyJSON = (value: string) => {
  const trimmed = String(value || '').trim()
  if (!trimmed) return ''
  try {
    return JSON.stringify(JSON.parse(trimmed), null, 2)
  } catch {
    return trimmed
  }
}

watch(() => props.refreshToken, () => {
  fetchRows()
})

watch(() => props.platformFilter, () => {
  fetchRows()
})

onMounted(() => {
  fetchRows()
})
</script>
