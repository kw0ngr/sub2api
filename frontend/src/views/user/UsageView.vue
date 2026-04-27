<template>
  <AppLayout>
    <TablePageLayout class="user-usage-layout">
      <template #actions>
        <div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
          <!-- Total Requests -->
          <div class="card p-4">
          <div class="flex items-center gap-3">
            <div class="rounded-lg bg-blue-100 p-2 dark:bg-blue-900/30">
              <Icon name="document" size="md" class="text-blue-600 dark:text-blue-400" />
            </div>
            <div>
              <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                {{ t('usage.totalRequests') }}
              </p>
              <p class="text-xl font-bold text-gray-900 dark:text-white">
                {{ usageStats?.total_requests?.toLocaleString() || '0' }}
              </p>
              <p class="text-xs text-gray-500 dark:text-gray-400">
                {{ t('usage.inSelectedRange') }}
              </p>
            </div>
          </div>
        </div>

        <!-- Total Tokens -->
        <div class="card p-4">
          <div class="flex items-center gap-3">
            <div class="rounded-lg bg-amber-100 p-2 dark:bg-amber-900/30">
              <Icon name="cube" size="md" class="text-amber-600 dark:text-amber-400" />
            </div>
            <div>
              <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                {{ t('usage.totalTokens') }}
              </p>
              <p class="text-xl font-bold text-gray-900 dark:text-white">
                {{ formatTokens(usageStats?.total_tokens || 0) }}
              </p>
              <p class="text-xs text-gray-500 dark:text-gray-400">
                {{ t('usage.in') }}: {{ formatTokens(usageStats?.total_input_tokens || 0) }} /
                {{ t('usage.out') }}: {{ formatTokens(usageStats?.total_output_tokens || 0) }}
              </p>
            </div>
          </div>
        </div>

        <!-- Total Cost -->
        <div class="card p-4">
          <div class="flex items-center gap-3">
            <div class="rounded-lg bg-green-100 p-2 dark:bg-green-900/30">
              <Icon name="dollar" size="md" class="text-green-600 dark:text-green-400" />
            </div>
            <div class="min-w-0 flex-1">
              <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                {{ t('usage.totalCost') }}
              </p>
              <p class="text-xl font-bold text-green-600 dark:text-green-400">
                ${{ (usageStats?.total_actual_cost || 0).toFixed(4) }}
              </p>
              <p class="text-xs text-gray-500 dark:text-gray-400">
                {{ t('usage.actualCost') }} /
                <span class="line-through">${{ (usageStats?.total_cost || 0).toFixed(4) }}</span>
                {{ t('usage.standardCost') }}
              </p>
            </div>
          </div>
        </div>

        <!-- Average Duration -->
        <div class="card p-4">
          <div class="flex items-center gap-3">
            <div class="rounded-lg bg-purple-100 p-2 dark:bg-purple-900/30">
              <Icon name="clock" size="md" class="text-purple-600 dark:text-purple-400" />
            </div>
            <div>
              <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                {{ t('usage.avgDuration') }}
              </p>
              <p class="text-xl font-bold text-gray-900 dark:text-white">
                {{ formatDuration(usageStats?.average_duration_ms || 0) }}
              </p>
              <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('usage.perRequest') }}</p>
            </div>
          </div>
        </div>
        </div>

        <div v-if="showSelfInsightArea" class="user-usage-insights mt-4">
          <div class="grid grid-cols-1 gap-4 lg:grid-cols-2 xl:grid-cols-3">
            <div v-if="selfInsightTips.length" class="card self-insight-summary xl:col-span-3">
              <div class="flex flex-col gap-1 sm:flex-row sm:items-end sm:justify-between">
                <div>
                  <p class="text-[11px] font-semibold uppercase tracking-[0.18em] text-blue-600 dark:text-blue-300">
                    Personal Signals
                  </p>
                  <h3 class="mt-1 text-sm font-semibold text-gray-900 dark:text-white">个人使用洞察</h3>
                </div>
                <div class="flex flex-wrap items-center gap-2 sm:justify-end">
                  <p class="text-xs text-gray-500 dark:text-gray-400">
                    基于当前时间范围，优先提示模型、工具、缓存和会话变化。
                  </p>
                  <button
                    type="button"
                    class="self-insight-copy"
                    title="复制个人使用洞察摘要"
                    :disabled="!selfInsightSummary"
                    @click="copySelfInsightSummary"
                  >
                    复制摘要
                  </button>
                </div>
              </div>
              <div class="mt-4 grid grid-cols-1 gap-3 md:grid-cols-3">
                <div
                  v-for="tip in selfInsightTips"
                  :key="tip.title"
                  class="self-insight-tip"
                  :class="`self-insight-tip-${tip.tone}`"
                >
                  <div class="flex items-start gap-3">
                    <span class="self-insight-tip-dot" />
                    <div class="min-w-0">
                      <p class="text-xs font-semibold text-gray-900 dark:text-white">{{ tip.title }}</p>
                      <p class="mt-1 text-xs leading-5 text-gray-500 dark:text-gray-400">
                        {{ tip.description }}
                      </p>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <div v-if="showSelfRecentCallsCard" class="card self-insight-card self-recent-card relative overflow-hidden p-4 xl:col-span-2">
            <div
              v-if="loading"
              class="absolute inset-0 z-10 flex items-center justify-center bg-white/50 backdrop-blur-sm dark:bg-dark-800/50"
            >
              <div class="h-5 w-5 animate-spin rounded-full border-2 border-blue-500 border-t-transparent" />
            </div>
            <div class="mb-4 flex items-start justify-between gap-3">
              <div>
                <h3 class="self-insight-title text-sm font-semibold text-gray-900 dark:text-white">近期调用</h3>
                <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                  直接展示当前筛选范围内最新请求，点击可打开 Trace。
                </p>
              </div>
              <span class="self-insight-pill rounded-full border border-sky-400/30 bg-sky-500/10 px-2 py-1 text-[11px] font-medium text-sky-600 dark:text-sky-300">
                {{ pagination.total.toLocaleString() }} 条
              </span>
            </div>
            <div v-if="recentUsageLogs.length" class="space-y-2">
              <button
                v-for="log in recentUsageLogs"
                :key="`recent-${log.id}`"
                type="button"
                class="self-recent-row w-full rounded-xl border border-gray-100 px-3 py-2 text-left dark:border-gray-700"
                @click="openTrace(log)"
              >
                <div class="flex items-start justify-between gap-3">
                  <div class="min-w-0">
                    <div class="flex min-w-0 items-center gap-2">
                      <span class="self-recent-dot" />
                      <p class="truncate text-sm font-semibold text-gray-900 dark:text-white" :title="log.model">
                        {{ log.model || '-' }}
                      </p>
                    </div>
                    <p class="mt-1 truncate text-xs text-gray-500 dark:text-gray-400" :title="formatUsageEndpoints(log)">
                      {{ log.api_key?.name || 'API Key' }} · {{ getRequestTypeLabel(log) }} · {{ formatUsageEndpoints(log) }}
                    </p>
                  </div>
                  <div class="shrink-0 text-right">
                    <p class="text-sm font-semibold text-emerald-600 dark:text-emerald-300">
                      ${{ log.actual_cost.toFixed(5) }}
                    </p>
                    <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                      {{ formatTokens(usageLogTotalTokens(log)) }} · {{ formatDuration(log.duration_ms) }}
                    </p>
                  </div>
                </div>
                <div class="mt-2 flex items-center justify-between gap-3 text-[11px] text-gray-500 dark:text-gray-400">
                  <span>{{ formatCompactDateTime(log.created_at) }}</span>
                  <span>Trace →</span>
                </div>
              </button>
            </div>
            <div v-else-if="!loading" class="flex h-28 items-center justify-center text-sm text-gray-500 dark:text-gray-400">
              暂无近期调用
            </div>
            </div>

            <div v-if="showSelfQualityCard" class="card self-insight-card self-quality-card relative overflow-hidden p-4 xl:col-span-1">
            <div
              v-if="loading"
              class="absolute inset-0 z-10 flex items-center justify-center bg-white/50 backdrop-blur-sm dark:bg-dark-800/50"
            >
              <div class="h-5 w-5 animate-spin rounded-full border-2 border-blue-500 border-t-transparent" />
            </div>
            <div class="mb-4 flex items-start justify-between gap-3">
              <div>
                <h3 class="self-insight-title text-sm font-semibold text-gray-900 dark:text-white">调用质量</h3>
                <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                  从当前页请求估算延迟、首 Token 和单次成本。
                </p>
              </div>
              <span
                class="self-insight-pill rounded-full border px-2 py-1 text-[11px] font-medium"
                :class="selfQualityStatusClass"
              >
                {{ selfQualityStatusLabel }}
              </span>
            </div>
            <div v-if="selfQualityMetrics.length" class="space-y-3">
              <div class="grid grid-cols-2 gap-2">
                <div
                  v-for="metric in selfQualityMetrics"
                  :key="metric.label"
                  class="self-insight-metric rounded-xl border border-gray-100 p-3 dark:border-gray-700"
                >
                  <p class="text-xs text-gray-500 dark:text-gray-400">{{ metric.label }}</p>
                  <p class="mt-1 text-base font-bold text-gray-900 dark:text-white">{{ metric.value }}</p>
                  <p class="mt-0.5 truncate text-xs text-gray-500 dark:text-gray-400" :title="metric.detail">
                    {{ metric.detail }}
                  </p>
                </div>
              </div>

              <div v-if="topCostModel" class="self-quality-row rounded-xl border border-gray-100 p-3 dark:border-gray-700">
                <div class="flex items-start justify-between gap-3">
                  <div class="min-w-0">
                    <p class="text-xs text-gray-500 dark:text-gray-400">当前页最高成本模型</p>
                    <p class="mt-1 truncate text-sm font-semibold text-gray-900 dark:text-white" :title="topCostModel.model">
                      {{ topCostModel.model }}
                    </p>
                  </div>
                  <p class="shrink-0 text-sm font-semibold text-emerald-600 dark:text-emerald-300">
                    {{ formatCurrency(topCostModel.cost) }}
                  </p>
                </div>
                <div class="mt-2 h-1.5 overflow-hidden rounded-full bg-gray-100 dark:bg-gray-700">
                  <div
                    class="h-full rounded-full bg-emerald-500"
                    :style="{ width: `${topCostModelShareWidth}%` }"
                  />
                </div>
              </div>

              <div
                v-if="selfSlowCallCount > 0"
                class="rounded-2xl border border-amber-400/30 bg-amber-500/10 px-3 py-2 text-xs text-amber-700 dark:text-amber-300"
              >
                <p class="font-semibold">发现慢请求</p>
                <p class="mt-1 opacity-80">
                  当前页有 {{ selfSlowCallCount }} 次请求超过 10s，建议查看 Trace 或切换上游。
                </p>
              </div>
            </div>
            <div v-else-if="!loading" class="flex h-28 items-center justify-center text-sm text-gray-500 dark:text-gray-400">
              暂无质量数据
            </div>
            </div>

            <div v-if="showSelfProfileCard" class="card self-insight-card self-insight-card-feature relative overflow-hidden p-4 xl:col-span-1">
            <div
              v-if="selfInsightsLoading"
              class="absolute inset-0 z-10 flex items-center justify-center bg-white/50 backdrop-blur-sm dark:bg-dark-800/50"
            >
              <div class="h-5 w-5 animate-spin rounded-full border-2 border-blue-500 border-t-transparent" />
            </div>
            <div class="mb-4 flex items-start justify-between gap-3">
              <div>
                <h3 class="self-insight-title text-sm font-semibold text-gray-900 dark:text-white">我的使用画像</h3>
                <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                  当前时间范围内的工具、模型和缓存效率。
                </p>
              </div>
              <span class="self-insight-pill rounded-full border border-emerald-400/30 bg-emerald-500/10 px-2 py-1 text-[11px] font-medium text-emerald-600 dark:text-emerald-300">
                缓存 {{ formatPercent(selfInsights?.cache_share || 0) }}
              </span>
            </div>
            <div v-if="selfInsights && selfInsights.total_tokens > 0" class="space-y-3">
              <div class="grid grid-cols-2 gap-2">
                <div class="self-insight-metric rounded-xl border border-gray-100 p-3 dark:border-gray-700">
                  <p class="text-xs text-gray-500 dark:text-gray-400">Token</p>
                  <p class="mt-1 text-lg font-bold text-gray-900 dark:text-white">
                    {{ formatTokens(selfInsights.total_tokens) }}
                  </p>
                </div>
                <div class="self-insight-metric rounded-xl border border-gray-100 p-3 dark:border-gray-700">
                  <p class="text-xs text-gray-500 dark:text-gray-400">请求</p>
                  <p class="mt-1 text-lg font-bold text-gray-900 dark:text-white">
                    {{ selfInsights.total_requests.toLocaleString() }}
                  </p>
                </div>
              </div>
              <div v-if="topSelfModel" class="self-insight-focus rounded-xl border border-blue-400/20 bg-blue-500/10 p-3">
                <p class="text-xs text-blue-600 dark:text-blue-300">主要模型</p>
                <p class="mt-1 truncate text-sm font-semibold text-gray-900 dark:text-white" :title="topSelfModel.model">
                  {{ topSelfModel.model }}
                </p>
                <p class="text-xs text-gray-500 dark:text-gray-400">
                  {{ formatTokens(topSelfModel.total_tokens) }} · {{ formatPercent(topSelfModel.cache_share) }} 缓存
                </p>
              </div>
            </div>
            <div v-else class="flex h-28 items-center justify-center text-sm text-gray-500 dark:text-gray-400">
              暂无画像数据
            </div>
            </div>

            <div v-if="showSelfClientCard" class="card self-insight-card relative overflow-hidden p-4">
            <div
              v-if="selfInsightsLoading"
              class="absolute inset-0 z-10 flex items-center justify-center bg-white/50 backdrop-blur-sm dark:bg-dark-800/50"
            >
              <div class="h-5 w-5 animate-spin rounded-full border-2 border-blue-500 border-t-transparent" />
            </div>
            <h3 class="self-insight-title mb-4 text-sm font-semibold text-gray-900 dark:text-white">客户端 / 工具分布</h3>
            <div v-if="selfClientItems.length" class="space-y-3">
              <div v-for="client in selfClientItems" :key="client.client" class="space-y-1.5">
                <div class="flex items-center justify-between gap-3 text-xs">
                  <span class="truncate font-medium text-gray-900 dark:text-white">
                    {{ client.client || 'Unknown' }}
                  </span>
                  <span class="shrink-0 text-gray-500 dark:text-gray-400">
                    {{ formatPercent(client.token_share) }} / {{ formatTokens(client.total_tokens) }}
                  </span>
                </div>
                <div class="self-progress-track h-2 overflow-hidden rounded-full bg-gray-100 dark:bg-gray-700">
                  <div
                    class="self-progress-fill self-progress-fill-cyan h-full rounded-full bg-cyan-500"
                    :style="{ width: `${clientShareWidth(client.token_share)}%` }"
                  />
                </div>
              </div>
            </div>
            <div v-else class="flex h-28 items-center justify-center text-sm text-gray-500 dark:text-gray-400">
              暂无客户端数据
            </div>
            </div>

            <div v-if="showSelfSessionCard" class="card self-insight-card relative overflow-hidden p-4">
            <div
              v-if="selfInsightsLoading"
              class="absolute inset-0 z-10 flex items-center justify-center bg-white/50 backdrop-blur-sm dark:bg-dark-800/50"
            >
              <div class="h-5 w-5 animate-spin rounded-full border-2 border-blue-500 border-t-transparent" />
            </div>
            <h3 class="self-insight-title mb-4 text-sm font-semibold text-gray-900 dark:text-white">请求会话</h3>
            <div v-if="selfSessionItems.length" class="space-y-2">
              <div
                v-for="session in selfSessionItems"
                :key="session.session_id"
                class="self-session-row rounded-xl border border-gray-100 px-3 py-2 dark:border-gray-700"
              >
                <div class="flex items-start justify-between gap-3">
                  <div class="min-w-0">
                    <p class="truncate text-xs font-semibold text-gray-900 dark:text-white" :title="`${session.client} / ${session.model}`">
                      {{ session.client }} / {{ session.model }}
                    </p>
                    <p class="mt-1 truncate text-xs text-gray-500 dark:text-gray-400" :title="session.api_key_name">
                      {{ session.api_key_name || 'API Key' }}
                    </p>
                  </div>
                  <div class="shrink-0 text-right text-xs text-gray-500 dark:text-gray-400">
                    <p>{{ formatTokens(session.total_tokens) }}</p>
                    <p>{{ formatCompactDateTime(session.last_seen) }}</p>
                  </div>
                </div>
              </div>
            </div>
            <div v-else class="flex h-28 items-center justify-center text-sm text-gray-500 dark:text-gray-400">
              暂无会话数据
            </div>
            </div>

            <div v-if="showSelfModelCard" class="card self-insight-card p-4">
            <div class="mb-4 flex items-center justify-between">
              <h3 class="self-insight-title text-sm font-semibold text-gray-900 dark:text-white">模型使用矩阵</h3>
              <span class="text-xs text-gray-500 dark:text-gray-400">Top {{ selfModelItems.length }}</span>
            </div>
            <div v-if="selfModelItems.length" class="space-y-2">
              <div
                v-for="model in selfModelItems"
                :key="model.model"
                class="self-model-row rounded-xl border border-gray-100 p-3 dark:border-gray-700"
              >
                <div class="flex items-center justify-between gap-3">
                  <p class="truncate text-sm font-semibold text-gray-900 dark:text-white" :title="model.model">
                    {{ model.model }}
                  </p>
                  <p class="shrink-0 text-xs text-gray-500 dark:text-gray-400">
                    {{ formatPercent(model.share_of_member) }}
                  </p>
                </div>
                <div class="self-progress-track mt-2 h-2 overflow-hidden rounded-full bg-gray-100 dark:bg-gray-700">
                  <div
                    class="self-progress-fill self-progress-fill-blue h-full rounded-full bg-blue-500"
                    :style="{ width: `${clientShareWidth(model.share_of_member)}%` }"
                  />
                </div>
                <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                  {{ formatTokens(model.total_tokens) }} · 缓存 {{ formatPercent(model.cache_share) }}
                </p>
              </div>
            </div>
            <div v-else class="flex h-28 items-center justify-center text-sm text-gray-500 dark:text-gray-400">
              暂无模型数据
            </div>
            </div>

            <div v-if="showSelfCacheCard" class="card self-insight-card p-4">
            <div class="mb-4 flex items-center justify-between">
              <h3 class="self-insight-title text-sm font-semibold text-gray-900 dark:text-white">缓存效率</h3>
              <span class="text-xs text-gray-500 dark:text-gray-400">
                读 {{ formatPercent(selfInsights?.cache_share || 0) }}
              </span>
            </div>
            <div v-if="selfCacheItems.length" class="space-y-2">
              <div
                v-for="item in selfCacheItems"
                :key="`${item.scope}-${item.model || item.label}`"
                class="self-cache-row rounded-xl border border-gray-100 p-3 dark:border-gray-700"
              >
                <div class="flex items-center justify-between gap-3">
                  <p class="truncate text-sm font-semibold text-gray-900 dark:text-white" :title="item.label || item.model">
                    {{ item.label || item.model || '-' }}
                  </p>
                  <p class="shrink-0 text-xs text-emerald-600 dark:text-emerald-400">
                    {{ formatPercent(item.cache_share) }}
                  </p>
                </div>
                <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                  读 {{ formatTokens(item.cache_read_tokens) }} / 写 {{ formatTokens(item.cache_creation_tokens) }}
                  <span v-if="item.ttl_override_count"> · TTL {{ item.ttl_override_count }}</span>
                </p>
              </div>
            </div>
            <div v-else class="flex h-28 items-center justify-center text-sm text-gray-500 dark:text-gray-400">
              暂无缓存数据
            </div>
            </div>
          </div>
        </div>
      </template>

      <template #filters>
        <div class="card">
          <div class="px-6 py-4">
          <div class="flex flex-wrap items-end gap-4">
            <!-- API Key Filter -->
            <div class="min-w-[180px]">
              <label class="input-label">{{ t('usage.apiKeyFilter') }}</label>
              <Select
                v-model="filters.api_key_id"
                :options="apiKeyOptions"
                :placeholder="t('usage.allApiKeys')"
                @change="applyFilters"
              />
            </div>

            <!-- Date Range Filter -->
            <div>
              <label class="input-label">{{ t('usage.timeRange') }}</label>
              <DateRangePicker
                v-model:start-date="startDate"
                v-model:end-date="endDate"
                @change="onDateRangeChange"
              />
            </div>

            <!-- Actions -->
            <div class="ml-auto flex items-center gap-3">
              <button @click="applyFilters" :disabled="loading" class="btn btn-secondary">
                {{ t('common.refresh') }}
              </button>
              <button @click="resetFilters" class="btn btn-secondary">
                {{ t('common.reset') }}
              </button>
              <button @click="exportToCSV" :disabled="exporting" class="btn btn-primary">
                <svg
                  v-if="exporting"
                  class="-ml-1 mr-2 h-4 w-4 animate-spin"
                  fill="none"
                  viewBox="0 0 24 24"
                >
                  <circle
                    class="opacity-25"
                    cx="12"
                    cy="12"
                    r="10"
                    stroke="currentColor"
                    stroke-width="4"
                  ></circle>
                  <path
                    class="opacity-75"
                    fill="currentColor"
                    d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                  ></path>
                </svg>
                {{ exporting ? t('usage.exporting') : t('usage.exportCsv') }}
              </button>
            </div>
          </div>
        </div>
        </div>
      </template>

      <template #table>
        <DataTable
          :columns="columns"
          :data="usageLogs"
          :loading="loading"
          :server-side-sort="true"
          default-sort-key="created_at"
          default-sort-order="desc"
          @sort="handleSort"
        >
          <template #cell-api_key="{ row }">
            <span class="text-sm text-gray-900 dark:text-white">{{
              row.api_key?.name || '-'
            }}</span>
          </template>

          <template #cell-model="{ value }">
            <span class="font-medium text-gray-900 dark:text-white">{{ value }}</span>
          </template>

          <template #cell-reasoning_effort="{ row }">
            <span class="text-sm text-gray-900 dark:text-white">
              {{ formatReasoningEffort(row.reasoning_effort) }}
            </span>
          </template>

          <template #cell-endpoint="{ row }">
            <span class="text-sm text-gray-600 dark:text-gray-300 block max-w-[320px] whitespace-normal break-all">
              {{ formatUsageEndpoints(row) }}
            </span>
          </template>

          <template #cell-stream="{ row }">
            <span
              class="inline-flex items-center rounded px-2 py-0.5 text-xs font-medium"
              :class="getRequestTypeBadgeClass(row)"
            >
              {{ getRequestTypeLabel(row) }}
            </span>
          </template>

          <template #cell-billing_mode="{ row }">
            <span class="inline-flex items-center rounded px-1.5 py-0.5 text-xs font-medium"
                  :class="getBillingModeBadgeClass(row.billing_mode)">
              {{ getBillingModeLabel(row.billing_mode, t) }}
            </span>
          </template>

          <template #cell-tokens="{ row }">
            <!-- 图片生成请求（仅按次计费时显示图片格式） -->
            <div v-if="row.image_count > 0 && row.billing_mode === 'image'" class="flex items-center gap-1.5">
              <svg
                class="h-4 w-4 text-indigo-500"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z"
                />
              </svg>
              <span class="font-medium text-gray-900 dark:text-white">{{ row.image_count }}{{ $t('usage.imageUnit') }}</span>
              <span class="text-gray-400">({{ row.image_size || '2K' }})</span>
            </div>
            <!-- Token 请求 -->
            <div v-else class="flex items-center gap-1.5">
              <div class="space-y-1.5 text-sm">
                <!-- Input / Output Tokens -->
                <div class="flex items-center gap-2">
                  <!-- Input -->
                  <div class="inline-flex items-center gap-1">
                    <Icon name="arrowDown" size="sm" class="text-emerald-500" />
                    <span class="font-medium text-gray-900 dark:text-white">{{
                      row.input_tokens.toLocaleString()
                    }}</span>
                  </div>
                  <!-- Output -->
                  <div class="inline-flex items-center gap-1">
                    <Icon name="arrowUp" size="sm" class="text-violet-500" />
                    <span class="font-medium text-gray-900 dark:text-white">{{
                      row.output_tokens.toLocaleString()
                    }}</span>
                  </div>
                </div>
                <!-- Cache Tokens (Read + Write) -->
                <div
                  v-if="row.cache_read_tokens > 0 || row.cache_creation_tokens > 0"
                  class="flex items-center gap-2"
                >
                  <!-- Cache Read -->
                  <div v-if="row.cache_read_tokens > 0" class="inline-flex items-center gap-1">
                    <Icon name="inbox" size="sm" class="text-sky-500" />
                    <span class="font-medium text-sky-600 dark:text-sky-400">{{
                      formatCacheTokens(row.cache_read_tokens)
                    }}</span>
                  </div>
                  <!-- Cache Write -->
                  <div v-if="row.cache_creation_tokens > 0" class="inline-flex items-center gap-1">
                    <Icon name="edit" size="sm" class="text-amber-500" />
                    <span class="font-medium text-amber-600 dark:text-amber-400">{{
                      formatCacheTokens(row.cache_creation_tokens)
                    }}</span>
                    <span v-if="row.cache_creation_1h_tokens > 0" class="inline-flex items-center rounded px-1 py-px text-[10px] font-medium leading-tight bg-orange-100 text-orange-600 ring-1 ring-inset ring-orange-200 dark:bg-orange-500/20 dark:text-orange-400 dark:ring-orange-500/30">1h</span>
                    <span v-if="row.cache_ttl_overridden" :title="t('usage.cacheTtlOverriddenHint')" class="inline-flex items-center rounded px-1 py-px text-[10px] font-medium leading-tight bg-rose-100 text-rose-600 ring-1 ring-inset ring-rose-200 dark:bg-rose-500/20 dark:text-rose-400 dark:ring-rose-500/30 cursor-help">R</span>
                  </div>
                </div>
              </div>
              <!-- Token Detail Tooltip -->
              <div
                class="group relative"
                @mouseenter="showTokenTooltip($event, row)"
                @mouseleave="hideTokenTooltip"
              >
                <div
                  class="flex h-4 w-4 cursor-help items-center justify-center rounded-full bg-gray-100 transition-colors group-hover:bg-blue-100 dark:bg-gray-700 dark:group-hover:bg-blue-900/50"
                >
                  <Icon
                    name="infoCircle"
                    size="xs"
                    class="text-gray-400 group-hover:text-blue-500 dark:text-gray-500 dark:group-hover:text-blue-400"
                  />
                </div>
              </div>
            </div>
          </template>

          <template #cell-cost="{ row }">
            <div class="flex items-center gap-1.5 text-sm">
              <span class="font-medium text-green-600 dark:text-green-400">
                ${{ row.actual_cost.toFixed(6) }}
              </span>
              <!-- Cost Detail Tooltip -->
              <div
                class="group relative"
                @mouseenter="showTooltip($event, row)"
                @mouseleave="hideTooltip"
              >
                <div
                  class="flex h-4 w-4 cursor-help items-center justify-center rounded-full bg-gray-100 transition-colors group-hover:bg-blue-100 dark:bg-gray-700 dark:group-hover:bg-blue-900/50"
                >
                  <Icon
                    name="infoCircle"
                    size="xs"
                    class="text-gray-400 group-hover:text-blue-500 dark:text-gray-500 dark:group-hover:text-blue-400"
                  />
                </div>
              </div>
            </div>
          </template>

          <template #cell-first_token="{ row }">
            <span
              v-if="row.first_token_ms != null"
              class="text-sm text-gray-600 dark:text-gray-400"
            >
              {{ formatDuration(row.first_token_ms) }}
            </span>
            <span v-else class="text-sm text-gray-400 dark:text-gray-500">-</span>
          </template>

          <template #cell-duration="{ row }">
            <span class="text-sm text-gray-600 dark:text-gray-400">{{
              formatDuration(row.duration_ms)
            }}</span>
          </template>

          <template #cell-created_at="{ value }">
            <span class="text-sm text-gray-600 dark:text-gray-400">{{
              formatDateTime(value)
            }}</span>
          </template>

          <template #cell-user_agent="{ row }">
            <span v-if="row.user_agent" class="text-sm text-gray-600 dark:text-gray-400 block max-w-[320px] whitespace-normal break-all" :title="row.user_agent">{{ formatUserAgent(row.user_agent) }}</span>
            <span v-else class="text-sm text-gray-400 dark:text-gray-500">-</span>
          </template>

          <template #cell-actions="{ row }">
            <button
              type="button"
              class="inline-flex items-center rounded-lg border border-sky-400/25 bg-sky-500/10 px-2.5 py-1.5 text-xs font-semibold text-sky-600 transition hover:border-sky-400/50 hover:bg-sky-500/15 dark:text-sky-300"
              @click="openTrace(row)"
            >
              Trace
            </button>
          </template>

          <template #empty>
            <EmptyState :message="t('usage.noRecords')" />
          </template>
        </DataTable>
      </template>

      <template #pagination>
        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </template>
    </TablePageLayout>
  </AppLayout>

  <!-- Token Tooltip Portal -->
  <Teleport to="body">
    <div
      v-if="tokenTooltipVisible"
      class="fixed z-[9999] pointer-events-none -translate-y-1/2"
      :style="{
        left: tokenTooltipPosition.x + 'px',
        top: tokenTooltipPosition.y + 'px'
      }"
    >
      <div
        class="whitespace-nowrap rounded-lg border border-gray-700 bg-gray-900 px-3 py-2.5 text-xs text-white shadow-xl dark:border-gray-600 dark:bg-gray-800"
      >
        <div class="space-y-1.5">
          <!-- Token Breakdown -->
          <div>
            <div class="text-xs font-semibold text-gray-300 mb-1">{{ t('usage.tokenDetails') }}</div>
            <div v-if="tokenTooltipData && tokenTooltipData.input_tokens > 0" class="flex items-center justify-between gap-4">
              <span class="text-gray-400">{{ t('admin.usage.inputTokens') }}</span>
              <span class="font-medium text-white">{{ tokenTooltipData.input_tokens.toLocaleString() }}</span>
            </div>
            <div v-if="tokenTooltipData && tokenTooltipData.output_tokens > 0" class="flex items-center justify-between gap-4">
              <span class="text-gray-400">{{ t('admin.usage.outputTokens') }}</span>
              <span class="font-medium text-white">{{ tokenTooltipData.output_tokens.toLocaleString() }}</span>
            </div>
            <div v-if="tokenTooltipData && tokenTooltipData.cache_creation_tokens > 0">
              <!-- 有 5m/1h 明细时，展开显示 -->
              <template v-if="tokenTooltipData.cache_creation_5m_tokens > 0 || tokenTooltipData.cache_creation_1h_tokens > 0">
                <div v-if="tokenTooltipData.cache_creation_5m_tokens > 0" class="flex items-center justify-between gap-4">
                  <span class="text-gray-400 flex items-center gap-1.5">
                    {{ t('admin.usage.cacheCreation5mTokens') }}
                    <span class="inline-flex items-center rounded px-1 py-px text-[10px] font-medium leading-tight bg-amber-500/20 text-amber-400 ring-1 ring-inset ring-amber-500/30">5m</span>
                  </span>
                  <span class="font-medium text-white">{{ tokenTooltipData.cache_creation_5m_tokens.toLocaleString() }}</span>
                </div>
                <div v-if="tokenTooltipData.cache_creation_1h_tokens > 0" class="flex items-center justify-between gap-4">
                  <span class="text-gray-400 flex items-center gap-1.5">
                    {{ t('admin.usage.cacheCreation1hTokens') }}
                    <span class="inline-flex items-center rounded px-1 py-px text-[10px] font-medium leading-tight bg-orange-500/20 text-orange-400 ring-1 ring-inset ring-orange-500/30">1h</span>
                  </span>
                  <span class="font-medium text-white">{{ tokenTooltipData.cache_creation_1h_tokens.toLocaleString() }}</span>
                </div>
              </template>
              <!-- 无明细时，只显示聚合值 -->
              <div v-else class="flex items-center justify-between gap-4">
                <span class="text-gray-400">{{ t('admin.usage.cacheCreationTokens') }}</span>
                <span class="font-medium text-white">{{ tokenTooltipData.cache_creation_tokens.toLocaleString() }}</span>
              </div>
            </div>
            <div v-if="tokenTooltipData && tokenTooltipData.cache_ttl_overridden" class="flex items-center justify-between gap-4">
              <span class="text-gray-400 flex items-center gap-1.5">
                {{ t('usage.cacheTtlOverriddenLabel') }}
                <span class="inline-flex items-center rounded px-1 py-px text-[10px] font-medium leading-tight bg-rose-500/20 text-rose-400 ring-1 ring-inset ring-rose-500/30">R-{{ tokenTooltipData.cache_creation_1h_tokens > 0 ? '5m' : '1H' }}</span>
              </span>
              <span class="font-medium text-rose-400">{{ tokenTooltipData.cache_creation_1h_tokens > 0 ? t('usage.cacheTtlOverridden1h') : t('usage.cacheTtlOverridden5m') }}</span>
            </div>
            <div v-if="tokenTooltipData && tokenTooltipData.cache_read_tokens > 0" class="flex items-center justify-between gap-4">
              <span class="text-gray-400">{{ t('admin.usage.cacheReadTokens') }}</span>
              <span class="font-medium text-white">{{ tokenTooltipData.cache_read_tokens.toLocaleString() }}</span>
            </div>
          </div>
          <!-- Total -->
          <div class="flex items-center justify-between gap-6 border-t border-gray-700 pt-1.5">
            <span class="text-gray-400">{{ t('usage.totalTokens') }}</span>
            <span class="font-semibold text-blue-400">{{ ((tokenTooltipData?.input_tokens || 0) + (tokenTooltipData?.output_tokens || 0) + (tokenTooltipData?.cache_creation_tokens || 0) + (tokenTooltipData?.cache_read_tokens || 0)).toLocaleString() }}</span>
          </div>
        </div>
        <!-- Tooltip Arrow (left side) -->
        <div
          class="absolute right-full top-1/2 h-0 w-0 -translate-y-1/2 border-b-[6px] border-r-[6px] border-t-[6px] border-b-transparent border-r-gray-900 border-t-transparent dark:border-r-gray-800"
        ></div>
      </div>
    </div>
  </Teleport>

  <!-- Tooltip Portal -->
  <Teleport to="body">
    <div
      v-if="tooltipVisible"
      class="fixed z-[9999] pointer-events-none -translate-y-1/2"
      :style="{
        left: tooltipPosition.x + 'px',
        top: tooltipPosition.y + 'px'
      }"
    >
      <div
        class="whitespace-nowrap rounded-lg border border-gray-700 bg-gray-900 px-3 py-2.5 text-xs text-white shadow-xl dark:border-gray-600 dark:bg-gray-800"
      >
        <div class="space-y-1.5">
          <!-- Cost Breakdown -->
          <div class="mb-2 border-b border-gray-700 pb-1.5">
            <div class="text-xs font-semibold text-gray-300 mb-1">{{ t('usage.costDetails') }}</div>
            <div v-if="tooltipData && tooltipData.input_cost > 0" class="flex items-center justify-between gap-4">
              <span class="text-gray-400">{{ t('admin.usage.inputCost') }}</span>
              <span class="font-medium text-white">${{ tooltipData.input_cost.toFixed(6) }}</span>
            </div>
            <div v-if="tooltipData && tooltipData.output_cost > 0" class="flex items-center justify-between gap-4">
              <span class="text-gray-400">{{ t('admin.usage.outputCost') }}</span>
              <span class="font-medium text-white">${{ tooltipData.output_cost.toFixed(6) }}</span>
            </div>
            <!-- Token billing: show unit prices per 1M tokens -->
            <template v-if="!tooltipData?.billing_mode || tooltipData.billing_mode === 'token'">
              <div v-if="tooltipData && tooltipData.input_tokens > 0" class="flex items-center justify-between gap-4">
                <span class="text-gray-400">{{ t('usage.inputTokenPrice') }}</span>
                <span class="font-medium text-sky-300">{{ formatTokenPricePerMillion(tooltipData.input_cost, tooltipData.input_tokens) }} {{ t('usage.perMillionTokens') }}</span>
              </div>
              <div v-if="tooltipData && tooltipData.output_tokens > 0" class="flex items-center justify-between gap-4">
                <span class="text-gray-400">{{ t('usage.outputTokenPrice') }}</span>
                <span class="font-medium text-violet-300">{{ formatTokenPricePerMillion(tooltipData.output_cost, tooltipData.output_tokens) }} {{ t('usage.perMillionTokens') }}</span>
              </div>
            </template>
            <!-- Per-request / image billing: show unit price -->
            <div v-else class="flex items-center justify-between gap-4">
              <span class="text-gray-400">{{ tooltipData.billing_mode === 'image' ? t('usage.imageUnitPrice') : t('usage.unitPrice') }}</span>
              <span class="font-medium text-sky-300">${{ tooltipData.total_cost?.toFixed(6) || '0.000000' }}</span>
            </div>
            <div v-if="tooltipData && tooltipData.cache_creation_cost > 0" class="flex items-center justify-between gap-4">
              <span class="text-gray-400">{{ t('admin.usage.cacheCreationCost') }}</span>
              <span class="font-medium text-white">${{ tooltipData.cache_creation_cost.toFixed(6) }}</span>
            </div>
            <div v-if="tooltipData && tooltipData.cache_read_cost > 0" class="flex items-center justify-between gap-4">
              <span class="text-gray-400">{{ t('admin.usage.cacheReadCost') }}</span>
              <span class="font-medium text-white">${{ tooltipData.cache_read_cost.toFixed(6) }}</span>
            </div>
          </div>
          <!-- Rate and Summary -->
          <div class="flex items-center justify-between gap-6">
            <span class="text-gray-400">{{ t('usage.serviceTier') }}</span>
            <span class="font-semibold text-cyan-300">{{ getUsageServiceTierLabel(tooltipData?.service_tier, t) }}</span>
          </div>
          <div class="flex items-center justify-between gap-6">
            <span class="text-gray-400">{{ t('usage.rate') }}</span>
            <span class="font-semibold text-blue-400"
              >{{ formatMultiplier(tooltipData?.rate_multiplier || 1) }}x</span
            >
          </div>
          <div class="flex items-center justify-between gap-6">
            <span class="text-gray-400">{{ t('usage.original') }}</span>
            <span class="font-medium text-white">${{ tooltipData?.total_cost.toFixed(6) }}</span>
          </div>
          <div class="flex items-center justify-between gap-6 border-t border-gray-700 pt-1.5">
            <span class="text-gray-400">{{ t('usage.billed') }}</span>
            <span class="font-semibold text-green-400"
              >${{ tooltipData?.actual_cost.toFixed(6) }}</span
            >
          </div>
        </div>
        <!-- Tooltip Arrow (left side) -->
        <div
          class="absolute right-full top-1/2 h-0 w-0 -translate-y-1/2 border-b-[6px] border-r-[6px] border-t-[6px] border-b-transparent border-r-gray-900 border-t-transparent dark:border-r-gray-800"
        ></div>
      </div>
    </div>
  </Teleport>

  <RequestTraceDrawer
    :show="Boolean(traceLog)"
    :log="traceLog"
    @close="closeTrace"
  />
</template>

<script setup lang="ts">
import { ref, computed, reactive, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { usageAPI, keysAPI } from '@/api'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Select from '@/components/common/Select.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import Icon from '@/components/icons/Icon.vue'
import RequestTraceDrawer from '@/components/usage/RequestTraceDrawer.vue'
import type { UsageLog, ApiKey, UsageQueryParams, UsageStatsResponse, SelfUsageInsights } from '@/types'
import type { Column } from '@/components/common/types'
import { formatDateTime, formatReasoningEffort } from '@/utils/format'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { useClipboard } from '@/composables/useClipboard'
import { formatCacheTokens, formatMultiplier } from '@/utils/formatters'
import { formatTokenPricePerMillion } from '@/utils/usagePricing'
import { getUsageServiceTierLabel } from '@/utils/usageServiceTier'
import { resolveUsageRequestType } from '@/utils/usageRequestType'
import { getBillingModeLabel, getBillingModeBadgeClass } from '@/utils/billingMode'

const { t } = useI18n()
const appStore = useAppStore()
const { copyToClipboard } = useClipboard()

let abortController: AbortController | null = null

// Tooltip state
const tooltipVisible = ref(false)
const tooltipPosition = ref({ x: 0, y: 0 })
const tooltipData = ref<UsageLog | null>(null)

// Token tooltip state
const tokenTooltipVisible = ref(false)
const tokenTooltipPosition = ref({ x: 0, y: 0 })
const tokenTooltipData = ref<UsageLog | null>(null)
const traceLog = ref<UsageLog | null>(null)

// Usage stats from API
const usageStats = ref<UsageStatsResponse | null>(null)
const selfInsights = ref<SelfUsageInsights | null>(null)
const selfInsightsLoading = ref(false)

const columns = computed<Column[]>(() => [
  { key: 'api_key', label: t('usage.apiKeyFilter'), sortable: false },
  { key: 'model', label: t('usage.model'), sortable: true },
  { key: 'reasoning_effort', label: t('usage.reasoningEffort'), sortable: false },
  { key: 'endpoint', label: t('usage.endpoint'), sortable: false },
  { key: 'stream', label: t('usage.type'), sortable: false },
  { key: 'billing_mode', label: t('admin.usage.billingMode'), sortable: false },
  { key: 'tokens', label: t('usage.tokens'), sortable: false },
  { key: 'cost', label: t('usage.cost'), sortable: false },
  { key: 'first_token', label: t('usage.firstToken'), sortable: false },
  { key: 'duration', label: t('usage.duration'), sortable: false },
  { key: 'created_at', label: t('usage.time'), sortable: true },
  { key: 'user_agent', label: t('usage.userAgent'), sortable: false },
  { key: 'actions', label: 'Trace', sortable: false }
])

const usageLogs = ref<UsageLog[]>([])
const apiKeys = ref<ApiKey[]>([])
const loading = ref(false)
const exporting = ref(false)

const apiKeyOptions = computed(() => {
  return [
    { value: null, label: t('usage.allApiKeys') },
    ...apiKeys.value.map((key) => ({
      value: key.id,
      label: key.name
    }))
  ]
})

// Helper function to format date in local timezone
const formatLocalDate = (date: Date): string => {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
}

// Initialize date range immediately
const now = new Date()
const weekAgo = new Date(now)
weekAgo.setDate(weekAgo.getDate() - 6)

// Date range state
const startDate = ref(formatLocalDate(weekAgo))
const endDate = ref(formatLocalDate(now))

const filters = ref<UsageQueryParams>({
  api_key_id: undefined,
  start_date: undefined,
  end_date: undefined
})

// Initialize filters with date range
filters.value.start_date = startDate.value
filters.value.end_date = endDate.value

// Handle date range change from DateRangePicker
const onDateRangeChange = (range: {
  startDate: string
  endDate: string
  preset: string | null
}) => {
  filters.value.start_date = range.startDate
  filters.value.end_date = range.endDate
  applyFilters()
}

const pagination = reactive({
  page: 1,
  page_size: getPersistedPageSize(),
  total: 0,
  pages: 0
})
const sortState = reactive({
  sort_by: 'created_at',
  sort_order: 'desc' as 'asc' | 'desc'
})

const formatDuration = (ms: number): string => {
  if (ms < 1000) return `${ms.toFixed(0)}ms`
  return `${(ms / 1000).toFixed(2)}s`
}

const formatUserAgent = (ua: string): string => {
  return ua
}

const getRequestTypeLabel = (log: UsageLog): string => {
  const requestType = resolveUsageRequestType(log)
  if (requestType === 'ws_v2') return t('usage.ws')
  if (requestType === 'stream') return t('usage.stream')
  if (requestType === 'sync') return t('usage.sync')
  return t('usage.unknown')
}

const getRequestTypeBadgeClass = (log: UsageLog): string => {
  const requestType = resolveUsageRequestType(log)
  if (requestType === 'ws_v2') return 'bg-violet-100 text-violet-800 dark:bg-violet-900 dark:text-violet-200'
  if (requestType === 'stream') return 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200'
  if (requestType === 'sync') return 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200'
  return 'bg-amber-100 text-amber-800 dark:bg-amber-900 dark:text-amber-200'
}


const getRequestTypeExportText = (log: UsageLog): string => {
  const requestType = resolveUsageRequestType(log)
  if (requestType === 'ws_v2') return 'WS'
  if (requestType === 'stream') return 'Stream'
  if (requestType === 'sync') return 'Sync'
  return 'Unknown'
}

const formatUsageEndpoints = (log: UsageLog): string => {
  const inbound = log.inbound_endpoint?.trim()
  return inbound || '-'
}

const openTrace = (row: UsageLog) => {
  traceLog.value = row
}

const closeTrace = () => {
  traceLog.value = null
}

const formatTokens = (value: number): string => {
  if (value >= 1_000_000_000) {
    return `${(value / 1_000_000_000).toFixed(2)}B`
  } else if (value >= 1_000_000) {
    return `${(value / 1_000_000).toFixed(2)}M`
  } else if (value >= 1_000) {
    return `${(value / 1_000).toFixed(2)}K`
  }
  return value.toLocaleString()
}

const formatCurrency = (value: number | null | undefined): string => {
  const safeValue = Number.isFinite(value) ? Number(value) : 0
  if (safeValue >= 1) return `$${safeValue.toFixed(2)}`
  if (safeValue >= 0.01) return `$${safeValue.toFixed(3)}`
  return `$${safeValue.toFixed(5)}`
}

const formatPercent = (value: number | null | undefined): string => `${((value || 0) * 100).toFixed(1)}%`

const clientShareWidth = (share: number | null | undefined): number => {
  if (!share || share <= 0) return 0
  return Math.max(4, Math.round(share * 100))
}

const formatCompactDateTime = (value: string | undefined): string => {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return date.toLocaleString(undefined, {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  })
}

const usageLogTotalTokens = (log: UsageLog): number => {
  return (
    (log.input_tokens || 0) +
    (log.output_tokens || 0) +
    (log.cache_creation_tokens || 0) +
    (log.cache_read_tokens || 0)
  )
}

const selfClientItems = computed(() => selfInsights.value?.client_distribution?.slice(0, 5) || [])
const selfModelItems = computed(() => selfInsights.value?.model_matrix?.slice(0, 6) || [])
const selfCacheItems = computed(() => selfInsights.value?.cache_efficiency?.slice(0, 5) || [])
const selfSessionItems = computed(() => selfInsights.value?.sessions?.slice(0, 5) || [])
const topSelfModel = computed(() => selfModelItems.value[0] || null)
const topSelfClient = computed(() => selfClientItems.value[0] || null)
const hasSelfProfileData = computed(() => (selfInsights.value?.total_tokens || 0) > 0)
const recentUsageLogs = computed(() => usageLogs.value.slice(0, 6))
const visibleUsageTotalCost = computed(() => usageLogs.value.reduce((sum, log) => sum + (log.actual_cost || 0), 0))
const visibleUsageTotalTokens = computed(() => usageLogs.value.reduce((sum, log) => sum + usageLogTotalTokens(log), 0))
const visibleAvgDurationMs = computed(() => {
  if (!usageLogs.value.length) return usageStats.value?.average_duration_ms || 0
  return usageLogs.value.reduce((sum, log) => sum + (log.duration_ms || 0), 0) / usageLogs.value.length
})
const visibleAvgFirstTokenMs = computed(() => {
  const values = usageLogs.value
    .map((log) => log.first_token_ms)
    .filter((value): value is number => typeof value === 'number' && Number.isFinite(value))
  if (!values.length) return 0
  return values.reduce((sum, value) => sum + value, 0) / values.length
})
const selfSlowCallCount = computed(() => usageLogs.value.filter((log) => (log.duration_ms || 0) > 10_000).length)
const visibleModelCount = computed(() => new Set(usageLogs.value.map((log) => log.model).filter(Boolean)).size)
const topCostModel = computed(() => {
  const costs = new Map<string, number>()
  for (const log of usageLogs.value) {
    const model = log.model || 'Unknown'
    costs.set(model, (costs.get(model) || 0) + (log.actual_cost || 0))
  }
  const [model, cost] = Array.from(costs.entries()).sort((a, b) => b[1] - a[1])[0] || []
  return model ? { model, cost } : null
})
const topCostModelShareWidth = computed(() => {
  if (!topCostModel.value || visibleUsageTotalCost.value <= 0) return 0
  return Math.max(4, Math.round((topCostModel.value.cost / visibleUsageTotalCost.value) * 100))
})
const selfAvgCostPerRequest = computed(() => {
  const totalRequests = usageStats.value?.total_requests || usageLogs.value.length
  const totalCost = usageStats.value?.total_actual_cost || visibleUsageTotalCost.value
  if (!totalRequests) return 0
  return totalCost / totalRequests
})
const selfQualityStatusLabel = computed(() => {
  if (selfSlowCallCount.value > 0 || visibleAvgDurationMs.value > 10_000) return '需关注'
  if (visibleAvgDurationMs.value > 5000) return '偏慢'
  return '良好'
})
const selfQualityStatusClass = computed(() => {
  if (selfSlowCallCount.value > 0 || visibleAvgDurationMs.value > 10_000) {
    return 'border-amber-400/30 bg-amber-500/10 text-amber-700 dark:text-amber-300'
  }
  if (visibleAvgDurationMs.value > 5000) {
    return 'border-cyan-400/30 bg-cyan-500/10 text-cyan-700 dark:text-cyan-300'
  }
  return 'border-emerald-400/30 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300'
})
const selfQualityMetrics = computed(() => {
  if (!usageLogs.value.length && !(usageStats.value?.total_requests || 0)) return []
  return [
    {
      label: '单次成本',
      value: formatCurrency(selfAvgCostPerRequest.value),
      detail: `${formatCurrency(visibleUsageTotalCost.value)} / 当前页`
    },
    {
      label: '平均耗时',
      value: formatDuration(visibleAvgDurationMs.value),
      detail: selfSlowCallCount.value > 0 ? `${selfSlowCallCount.value} 慢请求` : '当前页'
    },
    {
      label: '首 Token',
      value: visibleAvgFirstTokenMs.value > 0 ? formatDuration(visibleAvgFirstTokenMs.value) : '-',
      detail: '可见流式记录均值'
    },
    {
      label: '模型数',
      value: visibleModelCount.value.toLocaleString(),
      detail: `${formatTokens(visibleUsageTotalTokens.value)} Token`
    }
  ]
})
const showSelfProfileCard = computed(() => selfInsightsLoading.value || hasSelfProfileData.value)
const showSelfRecentCallsCard = computed(() => loading.value || recentUsageLogs.value.length > 0)
const showSelfQualityCard = computed(() => loading.value || selfQualityMetrics.value.length > 0)
const showSelfClientCard = computed(() => selfInsightsLoading.value || selfClientItems.value.length > 0)
const showSelfSessionCard = computed(() => selfInsightsLoading.value || selfSessionItems.value.length > 0)
const showSelfModelCard = computed(() => selfInsightsLoading.value || selfModelItems.value.length > 0)
const showSelfCacheCard = computed(() => selfInsightsLoading.value || selfCacheItems.value.length > 0)
const showSelfInsightArea = computed(
  () =>
    showSelfProfileCard.value ||
    showSelfRecentCallsCard.value ||
    showSelfQualityCard.value ||
    showSelfClientCard.value ||
    showSelfSessionCard.value ||
    showSelfModelCard.value ||
    showSelfCacheCard.value
)

type SelfInsightTip = {
  title: string
  description: string
  tone: 'blue' | 'emerald' | 'amber' | 'violet'
}

const selfInsightTips = computed<SelfInsightTip[]>(() => {
  if (!hasSelfProfileData.value) return []

  const tips: SelfInsightTip[] = []
  const model = topSelfModel.value
  const client = topSelfClient.value
  const cacheShare = selfInsights.value?.cache_share || 0

  if (model) {
    tips.push({
      title: '主要模型',
      description: `${model.model} 占本期用量 ${formatPercent(model.share_of_member)}，共 ${formatTokens(model.total_tokens)} Token。`,
      tone: 'blue'
    })
  }

  if (client) {
    tips.push({
      title: '主要工具',
      description: `${client.client || 'Unknown'} 贡献 ${formatPercent(client.token_share)} 用量，可用来判断团队客户端迁移情况。`,
      tone: 'violet'
    })
  }

  if (cacheShare >= 0.45) {
    tips.push({
      title: '缓存利用较好',
      description: `缓存占比 ${formatPercent(cacheShare)}，长上下文复用比较充分。`,
      tone: 'emerald'
    })
  } else if (cacheShare > 0) {
    tips.push({
      title: '缓存仍可优化',
      description: `缓存占比 ${formatPercent(cacheShare)}，高频提示词或长上下文可以考虑稳定复用。`,
      tone: 'amber'
    })
  }

  const session = selfSessionItems.value[0]
  if (session && tips.length < 3) {
    tips.push({
      title: '最近会话',
      description: `${session.client || 'Unknown'} / ${session.model || '-'} 最近活跃于 ${formatCompactDateTime(session.last_seen)}。`,
      tone: 'emerald'
    })
  }

  return tips.slice(0, 3)
})

const selfInsightSummary = computed(() => {
  if (!selfInsightTips.value.length || !selfInsights.value) return ''
  const { start_date, end_date } = getActiveDateRange()
  const dateRange = `${start_date} ~ ${end_date}`
  return [
    `个人使用洞察（${dateRange}）`,
    `请求 ${selfInsights.value.total_requests.toLocaleString()} / Token ${formatTokens(selfInsights.value.total_tokens)} / 缓存 ${formatPercent(selfInsights.value.cache_share)}`,
    ...selfInsightTips.value.map((tip) => `- ${tip.title}: ${tip.description}`)
  ].join('\n')
})

const copySelfInsightSummary = async () => {
  await copyToClipboard(selfInsightSummary.value, '洞察摘要已复制')
}

type UsageTableQueryParams = UsageQueryParams & {
  sort_by?: string
  sort_order?: 'asc' | 'desc'
}

const getActiveDateRange = () => ({
  start_date: filters.value.start_date || startDate.value,
  end_date: filters.value.end_date || endDate.value
})

const getSelectedApiKeyId = (): number | undefined => {
  const value = filters.value.api_key_id as number | string | null | undefined
  if (value === null || value === undefined || value === '') return undefined

  const id = Number(value)
  return Number.isFinite(id) && id > 0 ? id : undefined
}

const buildUsageQueryParams = (page: number, pageSize: number): UsageTableQueryParams => {
  const { start_date, end_date } = getActiveDateRange()
  const params: UsageTableQueryParams = {
    page,
    page_size: pageSize,
    sort_by: sortState.sort_by,
    sort_order: sortState.sort_order
  }

  if (start_date) params.start_date = start_date
  if (end_date) params.end_date = end_date

  const apiKeyId = getSelectedApiKeyId()
  if (apiKeyId !== undefined) {
    params.api_key_id = apiKeyId
  }

  return params
}

const loadUsageLogs = async () => {
  if (abortController) {
    abortController.abort()
  }
  const currentAbortController = new AbortController()
  abortController = currentAbortController
  const { signal } = currentAbortController
  loading.value = true
  try {
    const response = await usageAPI.query(
      buildUsageQueryParams(pagination.page, pagination.page_size),
      { signal }
    )
    if (signal.aborted) {
      return
    }
    usageLogs.value = response.items
    pagination.total = response.total
    pagination.pages = response.pages
  } catch (error) {
    if (signal.aborted) {
      return
    }
    const abortError = error as { name?: string; code?: string }
    if (abortError?.name === 'AbortError' || abortError?.code === 'ERR_CANCELED') {
      return
    }
    appStore.showError(t('usage.failedToLoad'))
  } finally {
    if (abortController === currentAbortController) {
      loading.value = false
    }
  }
}

const loadApiKeys = async () => {
  try {
    const response = await keysAPI.list(1, 100)
    apiKeys.value = response.items
  } catch (error) {
    console.error('Failed to load API keys:', error)
  }
}

const loadUsageStats = async () => {
  try {
    const apiKeyId = getSelectedApiKeyId()
    const { start_date, end_date } = getActiveDateRange()
    const stats = await usageAPI.getStatsByDateRange(
      start_date,
      end_date,
      apiKeyId
    )
    usageStats.value = stats
  } catch (error) {
    console.error('Failed to load usage stats:', error)
  }
}

const loadSelfInsights = async () => {
  selfInsightsLoading.value = true
  try {
    const { start_date, end_date } = getActiveDateRange()
    const response = await usageAPI.getDashboardSelfInsights({
      start_date,
      end_date,
      api_key_id: getSelectedApiKeyId(),
      limit: 8
    })
    selfInsights.value = response.insights || null
  } catch (error) {
    console.error('Failed to load self usage insights:', error)
    selfInsights.value = null
  } finally {
    selfInsightsLoading.value = false
  }
}

const applyFilters = () => {
  pagination.page = 1
  loadUsageLogs()
  loadUsageStats()
  loadSelfInsights()
}

const resetFilters = () => {
  filters.value = {
    api_key_id: undefined,
    start_date: undefined,
    end_date: undefined
  }
  // Reset date range to default (last 7 days)
  const now = new Date()
  const weekAgo = new Date(now)
  weekAgo.setDate(weekAgo.getDate() - 6)
  startDate.value = formatLocalDate(weekAgo)
  endDate.value = formatLocalDate(now)
  filters.value.start_date = startDate.value
  filters.value.end_date = endDate.value
  pagination.page = 1
  loadUsageLogs()
  loadUsageStats()
  loadSelfInsights()
}

const handlePageChange = (page: number) => {
  pagination.page = page
  loadUsageLogs()
}

const handlePageSizeChange = (pageSize: number) => {
  pagination.page_size = pageSize
  pagination.page = 1
  loadUsageLogs()
}

const handleSort = (key: string, order: 'asc' | 'desc') => {
  sortState.sort_by = key
  sortState.sort_order = order
  pagination.page = 1
  loadUsageLogs()
}

/**
 * Escape CSV value to prevent injection and handle special characters
 */
const escapeCSVValue = (value: unknown): string => {
  if (value == null) return ''

  const str = String(value)
  const escaped = str.replace(/"/g, '""')

  // Prevent formula injection by prefixing dangerous characters with single quote
  if (/^[=+\-@\t\r]/.test(str)) {
    return `"\'${escaped}"`
  }

  // Escape values containing comma, quote, or newline
  if (/[,"\n\r]/.test(str)) {
    return `"${escaped}"`
  }

  return str
}

const exportToCSV = async () => {
  if (pagination.total === 0) {
    appStore.showWarning(t('usage.noDataToExport'))
    return
  }

  exporting.value = true
  appStore.showInfo(t('usage.preparingExport'))

  try {
    const allLogs: UsageLog[] = []
    const pageSize = 100 // Use a larger page size for export to reduce requests
    const totalRequests = Math.ceil(pagination.total / pageSize)

    for (let page = 1; page <= totalRequests; page++) {
      const response = await usageAPI.query(buildUsageQueryParams(page, pageSize))
      allLogs.push(...response.items)
    }

    if (allLogs.length === 0) {
      appStore.showWarning(t('usage.noDataToExport'))
      return
    }

    const headers = [
      'Time',
      'API Key Name',
      'Model',
      'Reasoning Effort',
      'Inbound Endpoint',
      'Type',
      'Billing Mode',
      'Input Tokens',
      'Output Tokens',
      'Cache Read Tokens',
      'Cache Creation Tokens',
      'Rate Multiplier',
      'Billed Cost',
      'Original Cost',
      'First Token (ms)',
      'Duration (ms)'
    ]
    const rows = allLogs.map((log) =>
      [
        log.created_at,
        log.api_key?.name || '',
        log.model,
        formatReasoningEffort(log.reasoning_effort),
        log.inbound_endpoint || '',
        getRequestTypeExportText(log),
        getBillingModeLabel(log.billing_mode, t),
        log.input_tokens,
        log.output_tokens,
        log.cache_read_tokens,
        log.cache_creation_tokens,
        log.rate_multiplier,
        log.actual_cost.toFixed(8),
        log.total_cost.toFixed(8),
        log.first_token_ms ?? '',
        log.duration_ms
      ].map(escapeCSVValue)
    )

    const csvContent = [
      headers.map(escapeCSVValue).join(','),
      ...rows.map((row) => row.join(','))
    ].join('\n')

    const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' })
    const url = window.URL.createObjectURL(blob)
    const link = document.createElement('a')
    const { start_date, end_date } = getActiveDateRange()
    link.href = url
    link.download = `usage_${start_date}_to_${end_date}.csv`
    link.click()
    window.URL.revokeObjectURL(url)

    appStore.showSuccess(t('usage.exportSuccess'))
  } catch (error) {
    appStore.showError(t('usage.exportFailed'))
    console.error('CSV Export failed:', error)
  } finally {
    exporting.value = false
  }
}

// Tooltip functions
const showTooltip = (event: MouseEvent, row: UsageLog) => {
  const target = event.currentTarget as HTMLElement
  const rect = target.getBoundingClientRect()

  tooltipData.value = row
  // Position to the right of the icon, vertically centered
  tooltipPosition.value.x = rect.right + 8
  tooltipPosition.value.y = rect.top + rect.height / 2
  tooltipVisible.value = true
}

const hideTooltip = () => {
  tooltipVisible.value = false
  tooltipData.value = null
}

// Token tooltip functions
const showTokenTooltip = (event: MouseEvent, row: UsageLog) => {
  const target = event.currentTarget as HTMLElement
  const rect = target.getBoundingClientRect()

  tokenTooltipData.value = row
  tokenTooltipPosition.value.x = rect.right + 8
  tokenTooltipPosition.value.y = rect.top + rect.height / 2
  tokenTooltipVisible.value = true
}

const hideTokenTooltip = () => {
  tokenTooltipVisible.value = false
  tokenTooltipData.value = null
}

onMounted(() => {
  loadApiKeys()
  loadUsageLogs()
  loadUsageStats()
  loadSelfInsights()
})
</script>

<style scoped>
.user-usage-layout {
  height: auto;
  min-height: calc(100vh - 64px - 4rem);
}

@media (min-width: 1024px) {
  .user-usage-layout :deep(.layout-section-scrollable),
  .user-usage-layout :deep(.table-scroll-container) {
    min-height: 24rem;
  }
}

.user-usage-insights {
  position: relative;
}

.self-insight-card {
  position: relative;
  border-color: rgb(226 232 240 / 0.92);
  background:
    linear-gradient(180deg, rgb(255 255 255 / 0.96), rgb(248 250 252 / 0.92)),
    radial-gradient(circle at 100% 0%, rgb(59 130 246 / 0.08), transparent 34%);
  box-shadow: 0 16px 36px rgb(15 23 42 / 0.06);
  transition:
    border-color 160ms ease,
    box-shadow 180ms ease,
    transform 180ms ease;
}

.self-insight-summary {
  overflow: hidden;
  border-color: rgb(191 219 254 / 0.72);
  background:
    radial-gradient(circle at 6% 20%, rgb(59 130 246 / 0.12), transparent 28%),
    linear-gradient(135deg, rgb(255 255 255 / 0.98), rgb(248 250 252 / 0.92));
  padding: 1rem;
  box-shadow: 0 18px 42px rgb(15 23 42 / 0.07);
}

.dark .self-insight-card {
  border-color: rgb(51 65 85 / 0.88);
  background:
    linear-gradient(180deg, rgb(15 23 42 / 0.95), rgb(15 23 42 / 0.88)),
    radial-gradient(circle at 100% 0%, rgb(56 189 248 / 0.12), transparent 36%);
  box-shadow: 0 20px 46px rgb(0 0 0 / 0.22);
}

.dark .self-insight-summary {
  border-color: rgb(30 64 175 / 0.46);
  background:
    radial-gradient(circle at 6% 20%, rgb(59 130 246 / 0.16), transparent 30%),
    linear-gradient(135deg, rgb(15 23 42 / 0.96), rgb(2 6 23 / 0.92));
  box-shadow: 0 22px 52px rgb(0 0 0 / 0.25);
}

.self-insight-card:hover {
  border-color: rgb(148 163 184 / 0.86);
  box-shadow: 0 18px 42px rgb(15 23 42 / 0.09);
  transform: translateY(-1px);
}

.dark .self-insight-card:hover {
  border-color: rgb(71 85 105 / 0.95);
  box-shadow: 0 22px 52px rgb(0 0 0 / 0.28);
}

.self-insight-card-feature::before {
  content: "";
  position: absolute;
  inset: 0;
  pointer-events: none;
  background: linear-gradient(135deg, rgb(59 130 246 / 0.10), transparent 42%);
}

.self-insight-title {
  letter-spacing: -0.01em;
}

.self-insight-pill {
  box-shadow: inset 0 1px 0 rgb(255 255 255 / 0.55);
}

.self-insight-tip {
  border: 1px solid rgb(226 232 240 / 0.92);
  border-radius: 1rem;
  background: rgb(255 255 255 / 0.74);
  padding: 0.85rem;
  transition:
    border-color 150ms ease,
    background-color 150ms ease,
    transform 150ms ease;
}

.dark .self-insight-tip {
  border-color: rgb(51 65 85 / 0.82);
  background: rgb(15 23 42 / 0.64);
}

.self-insight-tip:hover {
  border-color: rgb(147 197 253 / 0.72);
  background: rgb(239 246 255 / 0.76);
  transform: translateY(-1px);
}

.dark .self-insight-tip:hover {
  border-color: rgb(96 165 250 / 0.38);
  background: rgb(30 41 59 / 0.76);
}

.self-insight-tip-dot {
  margin-top: 0.25rem;
  height: 0.55rem;
  width: 0.55rem;
  flex: none;
  border-radius: 9999px;
  background: rgb(59 130 246);
  box-shadow: 0 0 0 4px rgb(59 130 246 / 0.12);
}

.self-insight-tip-emerald .self-insight-tip-dot {
  background: rgb(16 185 129);
  box-shadow: 0 0 0 4px rgb(16 185 129 / 0.12);
}

.self-insight-tip-amber .self-insight-tip-dot {
  background: rgb(245 158 11);
  box-shadow: 0 0 0 4px rgb(245 158 11 / 0.14);
}

.self-insight-tip-violet .self-insight-tip-dot {
  background: rgb(139 92 246);
  box-shadow: 0 0 0 4px rgb(139 92 246 / 0.12);
}

.self-insight-copy {
  border-radius: 9999px;
  border: 1px solid rgb(147 197 253 / 0.56);
  background: rgb(239 246 255 / 0.82);
  padding: 0.3rem 0.7rem;
  color: rgb(37 99 235);
  font-size: 0.72rem;
  font-weight: 700;
  transition:
    background-color 150ms ease,
    border-color 150ms ease,
    transform 120ms ease;
}

.self-insight-copy:hover:not(:disabled) {
  border-color: rgb(96 165 250 / 0.86);
  background: rgb(219 234 254 / 0.92);
  transform: translateY(-1px);
}

.self-insight-copy:disabled {
  cursor: not-allowed;
  opacity: 0.52;
}

.dark .self-insight-copy {
  border-color: rgb(59 130 246 / 0.34);
  background: rgb(30 41 59 / 0.78);
  color: rgb(147 197 253);
}

.dark .self-insight-copy:hover:not(:disabled) {
  border-color: rgb(96 165 250 / 0.52);
  background: rgb(30 64 175 / 0.32);
}

.self-insight-metric,
.self-insight-focus,
.self-session-row,
.self-recent-row,
.self-quality-row,
.self-model-row,
.self-cache-row {
  background: rgb(255 255 255 / 0.72);
  transition:
    background-color 150ms ease,
    border-color 150ms ease,
    transform 150ms ease;
}

.dark .self-insight-metric,
.dark .self-insight-focus,
.dark .self-session-row,
.dark .self-recent-row,
.dark .self-quality-row,
.dark .self-model-row,
.dark .self-cache-row {
  background: rgb(15 23 42 / 0.58);
}

.self-recent-row {
  box-shadow: inset 0 1px 0 rgb(255 255 255 / 0.72);
}

.self-recent-dot {
  height: 0.55rem;
  width: 0.55rem;
  flex: none;
  border-radius: 9999px;
  background: linear-gradient(135deg, rgb(14 165 233), rgb(16 185 129));
  box-shadow: 0 0 0 4px rgb(14 165 233 / 0.10);
}

.self-session-row:hover,
.self-recent-row:hover,
.self-quality-row:hover,
.self-model-row:hover,
.self-cache-row:hover {
  border-color: rgb(96 165 250 / 0.42);
  background: rgb(239 246 255 / 0.74);
  transform: translateX(1px);
}

.dark .self-session-row:hover,
.dark .self-recent-row:hover,
.dark .self-quality-row:hover,
.dark .self-model-row:hover,
.dark .self-cache-row:hover {
  border-color: rgb(96 165 250 / 0.34);
  background: rgb(30 41 59 / 0.74);
}

.self-progress-track {
  box-shadow: inset 0 1px 2px rgb(15 23 42 / 0.10);
}

.self-progress-fill {
  min-width: 0.35rem;
  box-shadow: 0 0 14px currentColor;
}

.self-progress-fill-cyan {
  color: rgb(6 182 212 / 0.35);
}

.self-progress-fill-blue {
  color: rgb(59 130 246 / 0.35);
}
</style>
