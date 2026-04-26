<template>
  <div class="space-y-6">
    <!-- Date Range Filter -->
    <div class="card p-4">
      <div class="flex flex-wrap items-center gap-4">
        <div class="flex items-center gap-2">
          <span class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('dashboard.timeRange') }}:</span>
          <DateRangePicker :start-date="startDate" :end-date="endDate" @update:startDate="$emit('update:startDate', $event)" @update:endDate="$emit('update:endDate', $event)" @change="$emit('dateRangeChange', $event)" />
        </div>
        <button @click="$emit('refresh')" :disabled="loading" class="btn btn-secondary">
          {{ t('common.refresh') }}
        </button>
        <div class="ml-auto flex items-center gap-2">
          <span class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('dashboard.granularity') }}:</span>
          <div class="w-28">
            <Select :model-value="granularity" :options="[{value:'day', label:t('dashboard.day')}, {value:'hour', label:t('dashboard.hour')}]" @update:model-value="$emit('update:granularity', $event)" @change="$emit('granularityChange')" />
          </div>
        </div>
      </div>
    </div>

    <!-- Charts Grid -->
    <div class="grid grid-cols-1 gap-6 lg:grid-cols-2">
      <!-- Model Distribution Chart -->
      <div class="card relative overflow-hidden p-4">
        <div v-if="loading" class="absolute inset-0 z-10 flex items-center justify-center bg-white/50 backdrop-blur-sm dark:bg-dark-800/50">
          <LoadingSpinner size="md" />
        </div>
        <h3 class="mb-4 text-sm font-semibold text-gray-900 dark:text-white">{{ t('dashboard.modelDistribution') }}</h3>
        <div class="flex items-center gap-6">
          <div class="h-48 w-48">
            <Doughnut v-if="modelData" :data="modelData" :options="doughnutOptions" />
            <div v-else class="flex h-full items-center justify-center text-sm text-gray-500 dark:text-gray-400">{{ t('dashboard.noDataAvailable') }}</div>
          </div>
          <div class="max-h-48 flex-1 overflow-y-auto">
            <table class="w-full text-xs">
              <thead>
                <tr class="text-gray-500 dark:text-gray-400">
                  <th class="pb-2 text-left">{{ t('dashboard.model') }}</th>
                  <th class="pb-2 text-right">{{ t('dashboard.requests') }}</th>
                  <th class="pb-2 text-right">{{ t('dashboard.tokens') }}</th>
                  <th class="pb-2 text-right">{{ t('dashboard.actual') }}</th>
                  <th class="pb-2 text-right">{{ t('dashboard.standard') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="model in models" :key="model.model" class="border-t border-gray-100 dark:border-gray-700">
                  <td class="max-w-[100px] truncate py-1.5 font-medium text-gray-900 dark:text-white" :title="model.model">{{ model.model }}</td>
                  <td class="py-1.5 text-right text-gray-600 dark:text-gray-400">{{ formatNumber(model.requests) }}</td>
                  <td class="py-1.5 text-right text-gray-600 dark:text-gray-400">{{ formatTokens(model.total_tokens) }}</td>
                  <td class="py-1.5 text-right text-green-600 dark:text-green-400">${{ formatCost(model.actual_cost) }}</td>
                  <td class="py-1.5 text-right text-gray-400 dark:text-gray-500">${{ formatCost(model.cost) }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>

      <!-- Token Usage Trend Chart -->
      <TokenUsageTrend :trend-data="trend" :loading="loading" />
    </div>

    <div class="grid grid-cols-1 gap-6 lg:grid-cols-2">
      <!-- Project Distribution -->
      <div class="card relative overflow-hidden p-4">
        <div v-if="loading" class="absolute inset-0 z-10 flex items-center justify-center bg-white/50 backdrop-blur-sm dark:bg-dark-800/50">
          <LoadingSpinner size="md" />
        </div>
        <h3 class="mb-4 text-sm font-semibold text-gray-900 dark:text-white">{{ t('dashboard.projectDistribution') }}</h3>
        <div v-if="projects?.length" class="space-y-3">
          <div v-for="project in topProjects" :key="project.project_key" class="space-y-1">
            <div class="flex items-center justify-between gap-3 text-xs">
              <span class="truncate font-medium text-gray-900 dark:text-white" :title="project.project_label">{{ project.project_label || project.project_key }}</span>
              <span class="shrink-0 text-gray-500 dark:text-gray-400">{{ formatTokens(project.total_tokens) }}</span>
            </div>
            <div class="h-2 overflow-hidden rounded-full bg-gray-100 dark:bg-gray-700">
              <div class="h-full rounded-full bg-cyan-500" :style="{ width: `${projectShare(project.total_tokens)}%` }" />
            </div>
          </div>
          <table class="mt-4 w-full text-xs">
            <thead>
              <tr class="text-gray-500 dark:text-gray-400">
                <th class="pb-2 text-left">{{ t('dashboard.project') }}</th>
                <th class="pb-2 text-right">{{ t('dashboard.requests') }}</th>
                <th class="pb-2 text-right">{{ t('dashboard.tokens') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="project in topProjects" :key="`row-${project.project_key}`" class="border-t border-gray-100 dark:border-gray-700">
                <td class="max-w-[160px] truncate py-1.5 font-medium text-gray-900 dark:text-white" :title="project.project_label">{{ project.project_label || project.project_key }}</td>
                <td class="py-1.5 text-right text-gray-600 dark:text-gray-400">{{ formatNumber(project.requests) }}</td>
                <td class="py-1.5 text-right text-gray-600 dark:text-gray-400">{{ formatTokens(project.total_tokens) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-else class="flex h-40 items-center justify-center text-sm text-gray-500 dark:text-gray-400">{{ t('dashboard.noDataAvailable') }}</div>
      </div>

      <!-- Usage Insights -->
      <div class="card relative overflow-hidden p-4">
        <div v-if="loading" class="absolute inset-0 z-10 flex items-center justify-center bg-white/50 backdrop-blur-sm dark:bg-dark-800/50">
          <LoadingSpinner size="md" />
        </div>
        <h3 class="mb-4 text-sm font-semibold text-gray-900 dark:text-white">{{ t('dashboard.usageInsights') }}</h3>
        <div v-if="insights && insights.total_tokens > 0" class="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <div class="rounded-lg border border-gray-100 p-3 dark:border-gray-700">
            <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('dashboard.topModel') }}</p>
            <p class="truncate text-sm font-semibold text-gray-900 dark:text-white" :title="insights.top_model">{{ insights.top_model || '-' }}</p>
            <p class="text-xs text-blue-600 dark:text-blue-400">{{ formatPercent(insights.top_model_share) }} · {{ formatTokens(insights.top_model_tokens) }}</p>
          </div>
          <div class="rounded-lg border border-gray-100 p-3 dark:border-gray-700">
            <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('dashboard.topProject') }}</p>
            <p class="truncate text-sm font-semibold text-gray-900 dark:text-white" :title="insights.top_project_label">{{ insights.top_project_label || '-' }}</p>
            <p class="text-xs text-cyan-600 dark:text-cyan-400">{{ formatPercent(insights.top_project_share) }} · {{ formatTokens(insights.top_project_tokens) }}</p>
          </div>
          <div class="rounded-lg border border-gray-100 p-3 dark:border-gray-700">
            <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('dashboard.cacheShare') }}</p>
            <p class="text-sm font-semibold text-gray-900 dark:text-white">{{ formatPercent(insights.cache_share) }}</p>
            <p class="text-xs text-amber-600 dark:text-amber-400">{{ formatTokens(insights.cache_tokens) }}</p>
          </div>
          <div class="rounded-lg border border-gray-100 p-3 dark:border-gray-700">
            <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('dashboard.variety') }}</p>
            <p class="text-sm font-semibold text-gray-900 dark:text-white">{{ insights.model_count }} {{ t('dashboard.model') }} / {{ insights.project_count }} {{ t('dashboard.project') }}</p>
            <p class="text-xs text-gray-500 dark:text-gray-400">{{ formatNumber(insights.requests) }} {{ t('dashboard.requests') }}</p>
          </div>
        </div>
        <div v-else class="flex h-40 items-center justify-center text-sm text-gray-500 dark:text-gray-400">{{ t('dashboard.noDataAvailable') }}</div>
      </div>
    </div>

    <!-- Hourly Activity Heatmap -->
    <div class="card relative overflow-hidden p-4">
      <div v-if="loading" class="absolute inset-0 z-10 flex items-center justify-center bg-white/50 backdrop-blur-sm dark:bg-dark-800/50">
        <LoadingSpinner size="md" />
      </div>
      <h3 class="mb-4 text-sm font-semibold text-gray-900 dark:text-white">{{ t('dashboard.hourlyActivity') }}</h3>
      <div v-if="hourlyActivity?.length" class="overflow-x-auto">
        <div class="grid min-w-[720px] grid-cols-[48px_repeat(24,minmax(20px,1fr))] gap-1 text-[10px]">
          <div />
          <div v-for="hour in hours" :key="`h-${hour}`" class="text-center text-gray-400">{{ hour % 3 === 0 ? hour.toString().padStart(2, '0') : '' }}</div>
          <template v-for="weekday in weekdays" :key="`weekday-${weekday.value}`">
            <div class="flex items-center text-gray-500 dark:text-gray-400">{{ weekday.label }}</div>
            <div
              v-for="hour in hours"
              :key="`${weekday.value}-${hour}`"
              :data-testid="`hourly-activity-cell-${weekday.value}-${hour}`"
              class="h-5 rounded"
              :class="heatmapCellClass(getHeatmapCell(weekday.value, hour)?.total_tokens || 0)"
              :title="heatmapTitle(weekday.value, hour)"
            />
          </template>
        </div>
      </div>
      <div v-else class="flex h-32 items-center justify-center text-sm text-gray-500 dark:text-gray-400">{{ t('dashboard.noDataAvailable') }}</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import Select from '@/components/common/Select.vue'
import { Doughnut } from 'vue-chartjs'
import TokenUsageTrend from '@/components/charts/TokenUsageTrend.vue'
import type { TrendDataPoint, ModelStat, ProjectStat, UsageInsightSummary, HourlyActivityHeatmapCell } from '@/types'
import { formatCostFixed as formatCost, formatNumberLocaleString as formatNumber, formatTokensK as formatTokens } from '@/utils/format'
import { Chart as ChartJS, CategoryScale, LinearScale, PointElement, LineElement, ArcElement, Title, Tooltip, Legend, Filler } from 'chart.js'
ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, ArcElement, Title, Tooltip, Legend, Filler)

const props = defineProps<{ loading: boolean, startDate: string, endDate: string, granularity: string, trend: TrendDataPoint[], models: ModelStat[], projects?: ProjectStat[], insights?: UsageInsightSummary | null, hourlyActivity?: HourlyActivityHeatmapCell[] }>()
defineEmits(['update:startDate', 'update:endDate', 'update:granularity', 'dateRangeChange', 'granularityChange', 'refresh'])
const { t } = useI18n()

const modelData = computed(() => !props.models?.length ? null : {
  labels: props.models.map((m: ModelStat) => m.model),
  datasets: [{
    data: props.models.map((m: ModelStat) => m.total_tokens),
    backgroundColor: ['#3b82f6', '#10b981', '#f59e0b', '#ef4444', '#8b5cf6', '#ec4899', '#06b6d4', '#84cc16']
  }]
})

const doughnutOptions = {
  responsive: true,
  maintainAspectRatio: false,
  plugins: {
    legend: { display: false },
    tooltip: {
      callbacks: {
        label: (context: any) => `${context.label}: ${formatTokens(context.parsed)} tokens`
      }
    }
  }
}

const topProjects = computed(() => (props.projects || []).slice(0, 8))
const totalProjectTokens = computed(() => topProjects.value.reduce((sum, p) => sum + p.total_tokens, 0))
const projectShare = (tokens: number) => totalProjectTokens.value > 0 ? Math.max(4, Math.round((tokens / totalProjectTokens.value) * 100)) : 0
const formatPercent = (value: number) => `${(value * 100).toFixed(1)}%`

const hours = Array.from({ length: 24 }, (_, index) => index)
const weekdays = [
  { value: 0, label: 'Sun' },
  { value: 1, label: 'Mon' },
  { value: 2, label: 'Tue' },
  { value: 3, label: 'Wed' },
  { value: 4, label: 'Thu' },
  { value: 5, label: 'Fri' },
  { value: 6, label: 'Sat' }
]
const maxHourlyTokens = computed(() => Math.max(0, ...(props.hourlyActivity || []).map(cell => cell.total_tokens)))
const getHeatmapCell = (weekday: number, hour: number) => (props.hourlyActivity || []).find(cell => cell.weekday === weekday && cell.hour === hour)
const heatmapCellClass = (tokens: number) => {
  if (!tokens || maxHourlyTokens.value <= 0) return 'bg-gray-100 dark:bg-gray-800'
  const ratio = tokens / maxHourlyTokens.value
  if (ratio >= 0.75) return 'bg-emerald-600'
  if (ratio >= 0.5) return 'bg-emerald-500'
  if (ratio >= 0.25) return 'bg-emerald-400'
  return 'bg-emerald-200 dark:bg-emerald-700'
}
const heatmapTitle = (weekday: number, hour: number) => {
  const cell = getHeatmapCell(weekday, hour)
  const tokens = cell?.total_tokens || 0
  const requests = cell?.requests || 0
  return `${weekdays[weekday]?.label || weekday} ${hour.toString().padStart(2, '0')}:00 · ${formatTokens(tokens)} tokens · ${formatNumber(requests)} requests`
}
</script>
