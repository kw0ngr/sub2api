<template>
  <AppLayout>
    <div class="admin-dashboard-polish space-y-6" :class="{ 'admin-dashboard-anti': antiDesignMode }">
      <!-- Loading State -->
      <div v-if="loading" class="flex items-center justify-center py-12">
        <LoadingSpinner />
      </div>

      <template v-else-if="stats">
        <!-- Row 1: Core Stats -->
        <div class="dashboard-stat-grid">
          <!-- Total API Keys -->
          <div class="card dashboard-stat-card p-4">
            <div class="flex items-center gap-3">
              <div class="rounded-lg bg-blue-100 p-2 dark:bg-blue-900/30">
                <Icon name="key" size="md" class="text-blue-600 dark:text-blue-400" :stroke-width="2" />
              </div>
              <div>
                <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                  {{ t('admin.dashboard.apiKeys') }}
                </p>
                <p class="text-xl font-bold text-gray-900 dark:text-white">
                  {{ stats.total_api_keys }}
                </p>
                <p class="text-xs text-green-600 dark:text-green-400">
                  {{ stats.active_api_keys }} {{ t('common.active') }}
                </p>
              </div>
            </div>
          </div>

          <!-- Service Accounts -->
          <div class="card dashboard-stat-card p-4">
            <div class="flex items-center gap-3">
              <div class="rounded-lg bg-purple-100 p-2 dark:bg-purple-900/30">
                <Icon name="server" size="md" class="text-purple-600 dark:text-purple-400" :stroke-width="2" />
              </div>
              <div>
                <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                  {{ t('admin.dashboard.accounts') }}
                </p>
                <p class="text-xl font-bold text-gray-900 dark:text-white">
                  {{ stats.total_accounts }}
                </p>
                <p class="text-xs">
                  <span class="text-green-600 dark:text-green-400"
                    >{{ stats.normal_accounts }} {{ t('common.active') }}</span
                  >
                  <span v-if="stats.error_accounts > 0" class="ml-1 text-red-500"
                    >{{ stats.error_accounts }} {{ t('common.error') }}</span
                  >
                </p>
              </div>
            </div>
          </div>

          <!-- Today Requests -->
          <div class="card dashboard-stat-card p-4">
            <div class="flex items-center gap-3">
              <div class="rounded-lg bg-green-100 p-2 dark:bg-green-900/30">
                <Icon name="chart" size="md" class="text-green-600 dark:text-green-400" :stroke-width="2" />
              </div>
              <div>
                <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                  {{ t('admin.dashboard.todayRequests') }}
                </p>
                <p class="text-xl font-bold text-gray-900 dark:text-white">
                  {{ stats.today_requests }}
                </p>
                <p class="text-xs text-gray-500 dark:text-gray-400">
                  {{ t('common.total') }}: {{ formatNumber(stats.total_requests) }}
                </p>
              </div>
            </div>
          </div>

          <!-- New Users Today -->
          <div class="card dashboard-stat-card p-4">
            <div class="flex items-center gap-3">
              <div class="rounded-lg bg-emerald-100 p-2 dark:bg-emerald-900/30">
                <Icon name="userPlus" size="md" class="text-emerald-600 dark:text-emerald-400" :stroke-width="2" />
              </div>
              <div>
                <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                  {{ t('admin.dashboard.users') }}
                </p>
                <p class="text-xl font-bold text-emerald-600 dark:text-emerald-400">
                  +{{ stats.today_new_users }}
                </p>
                <p class="text-xs text-gray-500 dark:text-gray-400">
                  {{ t('common.total') }}: {{ formatNumber(stats.total_users) }}
                </p>
              </div>
            </div>
          </div>
        </div>

        <!-- Row 2: Token Stats -->
        <div class="dashboard-stat-grid">
          <!-- Today Tokens -->
          <div class="card dashboard-stat-card p-4">
            <div class="flex items-center gap-3">
              <div class="rounded-lg bg-amber-100 p-2 dark:bg-amber-900/30">
                <Icon name="cube" size="md" class="text-amber-600 dark:text-amber-400" :stroke-width="2" />
              </div>
              <div>
                <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                  {{ t('admin.dashboard.todayTokens') }}
                </p>
                <p class="text-xl font-bold text-gray-900 dark:text-white">
                  {{ formatTokens(stats.today_tokens) }}
                </p>
                <p class="text-xs">
                  <span
                    class="text-green-600 dark:text-green-400"
                    :title="t('admin.dashboard.actual')"
                    >${{ formatCost(stats.today_actual_cost) }}</span
                  >
                  <span class="text-gray-400 dark:text-gray-500"> / </span>
                  <span
                    class="text-orange-500 dark:text-orange-400"
                    :title="t('admin.dashboard.accountCost')"
                    >${{ formatCost(stats.today_account_cost) }}</span
                  >
                  <span class="text-gray-400 dark:text-gray-500"> / </span>
                  <span
                    class="text-gray-400 dark:text-gray-500"
                    :title="t('admin.dashboard.standard')"
                    >${{ formatCost(stats.today_cost) }}</span
                  >
                </p>
              </div>
            </div>
          </div>

          <!-- Total Tokens -->
          <div class="card dashboard-stat-card p-4">
            <div class="flex items-center gap-3">
              <div class="rounded-lg bg-indigo-100 p-2 dark:bg-indigo-900/30">
                <Icon name="database" size="md" class="text-indigo-600 dark:text-indigo-400" :stroke-width="2" />
              </div>
              <div>
                <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                  {{ t('admin.dashboard.totalTokens') }}
                </p>
                <p class="text-xl font-bold text-gray-900 dark:text-white">
                  {{ formatTokens(stats.total_tokens) }}
                </p>
                <p class="text-xs">
                  <span
                    class="text-green-600 dark:text-green-400"
                    :title="t('admin.dashboard.actual')"
                    >${{ formatCost(stats.total_actual_cost) }}</span
                  >
                  <span class="text-gray-400 dark:text-gray-500"> / </span>
                  <span
                    class="text-orange-500 dark:text-orange-400"
                    :title="t('admin.dashboard.accountCost')"
                    >${{ formatCost(stats.total_account_cost) }}</span
                  >
                  <span class="text-gray-400 dark:text-gray-500"> / </span>
                  <span
                    class="text-gray-400 dark:text-gray-500"
                    :title="t('admin.dashboard.standard')"
                    >${{ formatCost(stats.total_cost) }}</span
                  >
                </p>
              </div>
            </div>
          </div>

          <!-- Performance (RPM/TPM) -->
          <div class="card dashboard-stat-card p-4">
            <div class="flex items-center gap-3">
              <div class="rounded-lg bg-violet-100 p-2 dark:bg-violet-900/30">
                <Icon name="bolt" size="md" class="text-violet-600 dark:text-violet-400" :stroke-width="2" />
              </div>
              <div class="flex-1">
                <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                  {{ t('admin.dashboard.performance') }}
                </p>
                <div class="flex items-baseline gap-2">
                  <p class="text-xl font-bold text-gray-900 dark:text-white">
                    {{ formatTokens(stats.rpm) }}
                  </p>
                  <span class="text-xs text-gray-500 dark:text-gray-400">RPM</span>
                </div>
                <div class="flex items-baseline gap-2">
                  <p class="text-sm font-semibold text-violet-600 dark:text-violet-400">
                    {{ formatTokens(stats.tpm) }}
                  </p>
                  <span class="text-xs text-gray-500 dark:text-gray-400">TPM</span>
                </div>
              </div>
            </div>
          </div>

          <!-- Avg Response Time -->
          <div class="card dashboard-stat-card p-4">
            <div class="flex items-center gap-3">
              <div class="rounded-lg bg-rose-100 p-2 dark:bg-rose-900/30">
                <Icon name="clock" size="md" class="text-rose-600 dark:text-rose-400" :stroke-width="2" />
              </div>
              <div>
                <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                  {{ t('admin.dashboard.avgResponse') }}
                </p>
                <p class="text-xl font-bold text-gray-900 dark:text-white">
                  {{ formatDuration(stats.average_duration_ms) }}
                </p>
                <p class="text-xs text-gray-500 dark:text-gray-400">
                  {{ stats.active_users }} {{ t('admin.dashboard.activeUsers') }}
                </p>
              </div>
            </div>
          </div>
        </div>

        <!-- Charts Section -->
        <div class="space-y-6">
          <!-- Date Range Filter -->
          <div class="card dashboard-filter-panel p-4">
            <div class="flex flex-wrap items-center gap-4">
              <div class="flex items-center gap-2">
                <span class="text-sm font-medium text-gray-700 dark:text-gray-300"
                  >{{ t('admin.dashboard.timeRange') }}:</span
                >
                <DateRangePicker
                  v-model:start-date="startDate"
                  v-model:end-date="endDate"
                  @change="onDateRangeChange"
                />
              </div>
              <button @click="loadDashboardStats" :disabled="chartsLoading" class="btn btn-secondary">
                {{ t('common.refresh') }}
              </button>
              <div class="ml-auto flex items-center gap-2">
                <span class="text-sm font-medium text-gray-700 dark:text-gray-300"
                  >{{ t('admin.dashboard.granularity') }}:</span
                >
                <div class="w-28">
                  <Select
                    v-model="granularity"
                    :options="granularityOptions"
                    @change="loadChartData"
                  />
                </div>
              </div>
            </div>
          </div>

          <!-- Charts Grid -->
          <div v-if="showChartCards" class="dashboard-chart-grid" :class="chartGridClass">
            <ModelDistributionChart
              v-if="showModelDistributionCard"
              class="dashboard-analytics-card"
              :model-stats="modelStats"
              :enable-ranking-view="true"
              :ranking-items="rankingItems"
              :ranking-total-actual-cost="rankingTotalActualCost"
              :ranking-total-requests="rankingTotalRequests"
              :ranking-total-tokens="rankingTotalTokens"
              :loading="chartsLoading"
              :ranking-loading="rankingLoading"
              :ranking-error="rankingError"
              :start-date="startDate"
              :end-date="endDate"
              @ranking-click="goToUserUsage"
            />
            <TokenUsageTrend
              v-if="showTrendCard"
              class="dashboard-analytics-card"
              :trend-data="trendData"
              :loading="chartsLoading"
            />
          </div>

          <!-- Team Member and Insight Widgets -->
          <div
            v-if="showInsightCards"
            class="dashboard-masonry"
            :class="{ 'dashboard-masonry--single': insightCardCount <= 1 }"
          >
            <div
              v-if="showUserContributionCard"
              class="card dashboard-analytics-card dashboard-masonry-item dashboard-masonry-item-wide relative overflow-hidden p-4"
            >
              <div
                v-if="rankingLoading"
                class="absolute inset-0 z-10 flex items-center justify-center bg-white/50 backdrop-blur-sm dark:bg-dark-800/50"
              >
                <LoadingSpinner size="md" />
              </div>
              <h3 class="mb-4 text-sm font-semibold text-gray-900 dark:text-white">
                {{ t('admin.dashboard.userDistribution') }}
              </h3>
              <div
                v-if="rankingError"
                class="flex h-40 items-center justify-center text-sm text-gray-500 dark:text-gray-400"
              >
                {{ t('admin.dashboard.failedToLoad') }}
              </div>
              <div v-else-if="userContributionItems.length" class="space-y-4">
                <button
                  v-if="topUser"
                  type="button"
                  class="w-full rounded-2xl border border-blue-400/30 bg-blue-500/10 p-4 text-left transition-colors hover:border-blue-400/50 hover:bg-blue-500/15"
                  @click="goToUserUsage(topUser)"
                >
                  <div class="flex items-start gap-3">
                    <div class="flex h-11 w-11 shrink-0 items-center justify-center rounded-full bg-blue-500 text-sm font-bold text-white">
                      {{ userInitials(topUser) }}
                    </div>
                    <div class="min-w-0 flex-1">
                      <div class="flex items-start justify-between gap-3">
                        <div class="min-w-0">
                          <p class="text-xs font-medium uppercase tracking-wide text-blue-600 dark:text-blue-300">
                            #1 {{ t('admin.dashboard.topMember') }}
                          </p>
                          <p class="mt-1 truncate text-lg font-bold text-gray-900 dark:text-white" :title="userDisplayName(topUser)">
                            {{ userDisplayName(topUser) }}
                          </p>
                        </div>
                        <div class="shrink-0 rounded-full border border-blue-400/30 px-3 py-1.5 text-right">
                          <p class="text-sm font-bold text-blue-600 dark:text-blue-300">
                            {{ formatPercent(topUserCostShare) }}
                          </p>
                          <p class="text-[11px] text-gray-500 dark:text-gray-400">
                            {{ t('admin.dashboard.topMemberShare') }}
                          </p>
                        </div>
                      </div>
                      <div class="mt-3 grid grid-cols-2 gap-3 text-xs">
                        <div>
                          <p class="text-gray-500 dark:text-gray-400">{{ t('admin.dashboard.spendingRankingSpend') }}</p>
                          <p class="font-semibold text-gray-900 dark:text-white">{{ formatCurrency(topUser.actual_cost) }}</p>
                        </div>
                        <div>
                          <p class="text-gray-500 dark:text-gray-400">{{ t('admin.dashboard.requests') }}</p>
                          <p class="font-semibold text-gray-900 dark:text-white">{{ formatNumber(topUser.requests) }}</p>
                        </div>
                      </div>
                      <p class="mt-3 text-xs text-blue-600 dark:text-blue-300">
                        {{ t('admin.dashboard.viewUserUsage') }}
                      </p>
                    </div>
                  </div>
                </button>

                <div class="grid grid-cols-3 gap-2">
                  <div class="rounded-xl border border-gray-100 p-3 dark:border-gray-700">
                    <p class="text-xs text-gray-500 dark:text-gray-400">
                      {{ t('admin.dashboard.activeMembers') }}
                    </p>
                    <p class="mt-1 text-lg font-bold text-gray-900 dark:text-white">
                      {{ formatNumber(activeUsageUserCount) }}
                    </p>
                  </div>
                  <div class="rounded-xl border border-gray-100 p-3 dark:border-gray-700">
                    <p class="text-xs text-gray-500 dark:text-gray-400">
                      {{ t('admin.dashboard.totalTeamSpend') }}
                    </p>
                    <p class="mt-1 text-lg font-bold text-gray-900 dark:text-white">
                      {{ formatCurrency(rankingTotalActualCost) }}
                    </p>
                  </div>
                  <div class="rounded-xl border border-gray-100 p-3 dark:border-gray-700">
                    <p class="text-xs text-gray-500 dark:text-gray-400">
                      {{ t('admin.dashboard.requests') }}
                    </p>
                    <p class="mt-1 text-lg font-bold text-gray-900 dark:text-white">
                      {{ formatNumber(rankingTotalRequests) }}
                    </p>
                  </div>
                </div>

                <div class="space-y-2">
                  <p class="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">
                    {{ t('admin.dashboard.memberContribution') }}
                  </p>
                  <div
                    v-for="(item, index) in userContributionItems"
                    :key="item.isOther ? 'other-members' : `user-${item.user_id}`"
                    class="space-y-1 rounded-lg px-2 py-1.5 transition-colors"
                    :class="item.isOther ? 'bg-gray-50 dark:bg-gray-800/40' : 'cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-800/50'"
                    @click="item.isOther ? undefined : goToUserUsage(item)"
                  >
                    <div class="flex items-center justify-between gap-3 text-xs">
                      <div class="flex min-w-0 items-center gap-2">
                        <span class="w-6 shrink-0 text-[11px] font-semibold text-gray-500 dark:text-gray-400">
                          {{ item.isOther ? 'Σ' : `#${index + 1}` }}
                        </span>
                        <span class="truncate font-medium text-gray-900 dark:text-white" :title="contributionDisplayName(item)">
                          {{ contributionDisplayName(item) }}
                        </span>
                      </div>
                      <span class="shrink-0 text-gray-500 dark:text-gray-400">
                        {{ formatPercent(userCostShare(item.actual_cost)) }} / {{ formatCurrency(item.actual_cost) }}
                      </span>
                    </div>
                    <div class="h-2 overflow-hidden rounded-full bg-gray-100 dark:bg-gray-700">
                      <div
                        class="h-full rounded-full"
                        :class="userBarClass(index, item.isOther)"
                        :style="{ width: `${userCostBarWidth(item.actual_cost)}%` }"
                      />
                    </div>
                  </div>
                </div>

                <table class="mt-2 w-full text-xs">
                  <thead>
                    <tr class="text-gray-500 dark:text-gray-400">
                      <th class="pb-2 text-left">{{ t('admin.dashboard.member') }}</th>
                      <th class="pb-2 text-right">{{ t('admin.dashboard.requests') }}</th>
                      <th class="pb-2 text-right">{{ t('admin.dashboard.spendingRankingSpend') }}</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr
                      v-for="item in userContributionItems"
                      :key="`row-${item.isOther ? 'other-members' : item.user_id}`"
                      class="border-t border-gray-100 transition-colors dark:border-gray-700"
                      :class="item.isOther ? '' : 'cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-800/50'"
                      @click="item.isOther ? undefined : goToUserUsage(item)"
                    >
                      <td
                        class="max-w-[180px] truncate py-1.5 font-medium text-gray-900 dark:text-white"
                        :title="contributionDisplayName(item)"
                      >
                        {{ contributionDisplayName(item) }}
                      </td>
                      <td class="py-1.5 text-right text-gray-600 dark:text-gray-400">
                        {{ formatNumber(item.requests) }}
                      </td>
                      <td class="py-1.5 text-right text-gray-600 dark:text-gray-400">
                        {{ formatCurrency(item.actual_cost) }}
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
              <div
                v-else-if="!rankingLoading"
                class="flex h-40 items-center justify-center text-sm text-gray-500 dark:text-gray-400"
              >
                {{ t('admin.dashboard.noDataAvailable') }}
              </div>
            </div>

            <div
              v-if="showCompactInsightSection"
              class="dashboard-masonry-item dashboard-masonry-item-wide dashboard-insight-columns"
            >
              <div class="dashboard-insight-stack">
                <div
                  v-if="showUsageInsightsCard"
                  class="card dashboard-analytics-card relative overflow-hidden p-4"
                >
                  <div
                    v-if="chartsLoading"
                    class="absolute inset-0 z-10 flex items-center justify-center bg-white/50 backdrop-blur-sm dark:bg-dark-800/50"
                  >
                    <LoadingSpinner size="md" />
                  </div>
                  <h3 class="mb-4 text-sm font-semibold text-gray-900 dark:text-white">
                    {{ t('admin.dashboard.usageInsights') }}
                  </h3>
                  <div v-if="usageInsights && hasUsageInsightsData" class="space-y-4">
                    <div>
                      <div class="mb-2 flex items-center justify-between">
                        <p class="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">
                          {{ t('admin.dashboard.tokenComposition') }}
                        </p>
                        <p class="text-xs text-gray-500 dark:text-gray-400">
                          {{ formatTokens(usageInsights.total_tokens) }}
                        </p>
                      </div>
                      <div class="flex h-3 overflow-hidden rounded-full bg-gray-100 dark:bg-gray-800">
                        <div
                          v-for="segment in tokenCompositionSegments"
                          :key="segment.key"
                          class="h-full"
                          :class="segment.class"
                          :style="{ width: `${segment.width}%` }"
                          :title="`${segment.label}: ${formatTokens(segment.tokens)} (${formatPercent(segment.share)})`"
                        />
                      </div>
                      <div class="mt-3 grid grid-cols-2 gap-2">
                        <div
                          v-for="segment in tokenCompositionSegments"
                          :key="`legend-${segment.key}`"
                          class="rounded-lg border border-gray-100 p-2 dark:border-gray-700"
                        >
                          <div class="flex items-center justify-between gap-2">
                            <span class="flex items-center gap-1 text-xs text-gray-500 dark:text-gray-400">
                              <span class="h-2 w-2 rounded-full" :class="segment.class" />
                              {{ segment.label }}
                            </span>
                            <span class="text-xs font-semibold text-gray-900 dark:text-white">
                              {{ formatPercent(segment.share) }}
                            </span>
                          </div>
                          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                            {{ formatTokens(segment.tokens) }}
                          </p>
                        </div>
                      </div>
                    </div>

                    <div
                      v-if="hasRankingSpendData"
                      class="dashboard-cost-summary rounded-2xl border border-gray-100 p-3 dark:border-gray-700"
                    >
                      <div class="mb-3 flex items-center justify-between gap-3">
                        <div>
                          <p class="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">
                            {{ t('admin.dashboard.nonAdminSpendSummary') }}
                          </p>
                          <p class="mt-1 text-lg font-bold text-gray-900 dark:text-white">
                            {{ formatCurrency(rankingTotalActualCost) }}
                          </p>
                        </div>
                        <span class="shrink-0 rounded-full border border-emerald-400/30 bg-emerald-500/10 px-2 py-1 text-[11px] font-semibold text-emerald-600 dark:text-emerald-300">
                          {{ formatNumber(activeUsageUserCount) }} {{ t('admin.dashboard.member') }}
                        </span>
                      </div>
                      <div class="grid grid-cols-3 gap-2 text-xs">
                        <div>
                          <p class="text-gray-500 dark:text-gray-400">{{ t('admin.dashboard.avgSpendPerRequest') }}</p>
                          <p class="mt-1 font-semibold text-gray-900 dark:text-white">{{ formatCurrency(avgSpendPerRequest) }}</p>
                        </div>
                        <div>
                          <p class="text-gray-500 dark:text-gray-400">{{ t('admin.dashboard.avgSpendPerMember') }}</p>
                          <p class="mt-1 font-semibold text-gray-900 dark:text-white">{{ formatCurrency(avgSpendPerMember) }}</p>
                        </div>
                        <div>
                          <p class="text-gray-500 dark:text-gray-400">{{ t('admin.dashboard.topMemberShare') }}</p>
                          <p class="mt-1 font-semibold text-gray-900 dark:text-white">{{ formatPercent(topUserCostShare) }}</p>
                        </div>
                      </div>
                    </div>

                    <div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
                      <div class="rounded-lg border border-gray-100 p-3 dark:border-gray-700">
                        <p class="text-xs text-gray-500 dark:text-gray-400">
                          {{ t('admin.dashboard.topModel') }}
                        </p>
                        <p
                          class="truncate text-sm font-semibold text-gray-900 dark:text-white"
                          :title="usageInsights.top_model"
                        >
                          {{ usageInsights.top_model || '-' }}
                        </p>
                        <p class="text-xs text-blue-600 dark:text-blue-400">
                          {{ formatPercent(usageInsights.top_model_share) }} /
                          {{ formatTokens(usageInsights.top_model_tokens) }}
                        </p>
                      </div>
                      <div class="rounded-lg border border-gray-100 p-3 dark:border-gray-700">
                        <p class="text-xs text-gray-500 dark:text-gray-400">
                          {{ t('admin.dashboard.topMember') }}
                        </p>
                        <p
                          class="truncate text-sm font-semibold text-gray-900 dark:text-white"
                          :title="topUserDisplayName"
                        >
                          {{ topUserDisplayName }}
                        </p>
                        <p class="text-xs text-cyan-600 dark:text-cyan-400">
                          {{ formatPercent(topUserCostShare) }} /
                          {{ formatCurrency(topUser?.actual_cost || 0) }}
                        </p>
                      </div>
                      <div class="rounded-lg border border-gray-100 p-3 dark:border-gray-700">
                        <p class="text-xs text-gray-500 dark:text-gray-400">
                          {{ t('admin.dashboard.variety') }}
                        </p>
                        <p class="text-sm font-semibold text-gray-900 dark:text-white">
                          {{ usageInsights.model_count }} {{ t('admin.dashboard.model') }} /
                          {{ activeUsageUserCount }} {{ t('admin.dashboard.member') }}
                        </p>
                        <p class="text-xs text-gray-500 dark:text-gray-400">
                          {{ formatNumber(usageInsights.requests) }} {{ t('admin.dashboard.requests') }}
                        </p>
                      </div>
                    </div>
                  </div>
                  <div
                    v-else-if="!chartsLoading"
                    class="flex h-40 items-center justify-center text-sm text-gray-500 dark:text-gray-400"
                  >
                    {{ t('admin.dashboard.noDataAvailable') }}
                  </div>
                </div>

                <div
                  v-if="showGatewayBriefCard"
                  class="card dashboard-analytics-card dashboard-gateway-brief-card relative overflow-hidden p-4"
                >
                  <div class="mb-4 flex items-start justify-between gap-3">
                    <div>
                      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
                        {{ t('admin.dashboard.gatewayBrief') }}
                      </h3>
                      <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                        {{ t('admin.dashboard.gatewayBriefHint') }}
                      </p>
                    </div>
                    <span
                      class="shrink-0 rounded-full border px-2 py-1 text-[11px] font-semibold"
                      :class="gatewayBriefStatusClass"
                    >
                      {{ gatewayBriefStatusLabel }}
                    </span>
                  </div>

                  <div class="grid grid-cols-2 gap-2">
                    <div
                      v-for="metric in gatewayBriefMetrics"
                      :key="metric.key"
                      class="dashboard-brief-metric rounded-xl border border-gray-100 p-3 dark:border-gray-700"
                    >
                      <p class="text-xs text-gray-500 dark:text-gray-400">{{ metric.label }}</p>
                      <p class="mt-1 text-lg font-bold text-gray-900 dark:text-white">{{ metric.value }}</p>
                      <p class="mt-0.5 truncate text-xs text-gray-500 dark:text-gray-400" :title="metric.detail">
                        {{ metric.detail }}
                      </p>
                    </div>
                  </div>

                  <div class="mt-4 space-y-2">
                    <button
                      v-for="alert in gatewayBriefAlerts"
                      :key="alert.title"
                      type="button"
                      class="w-full rounded-2xl border px-3 py-2 text-left text-xs"
                      :class="gatewayBriefAlertClass(alert.tone)"
                      @click="goToAdminPath(alert.path)"
                    >
                      <div class="flex items-start justify-between gap-3">
                        <div class="min-w-0">
                          <p class="font-semibold">{{ alert.title }}</p>
                          <p class="mt-1 leading-5 opacity-80">{{ alert.description }}</p>
                        </div>
                        <span class="shrink-0 rounded-full bg-white/55 px-2 py-0.5 text-[10px] font-semibold dark:bg-black/20">
                          {{ alert.actionLabel }}
                        </span>
                      </div>
                    </button>
                  </div>

                  <div class="mt-4 grid grid-cols-2 gap-2">
                    <button
                      type="button"
                      class="dashboard-brief-action rounded-xl border border-gray-100 px-3 py-2 text-xs font-semibold text-gray-700 hover:text-blue-600 dark:border-gray-700 dark:text-gray-200 dark:hover:text-blue-300"
                      @click="goToAdminPath('/admin/accounts')"
                    >
                      {{ t('admin.dashboard.gatewayBriefAccounts') }}
                    </button>
                    <button
                      type="button"
                      class="dashboard-brief-action rounded-xl border border-gray-100 px-3 py-2 text-xs font-semibold text-gray-700 hover:text-blue-600 dark:border-gray-700 dark:text-gray-200 dark:hover:text-blue-300"
                      @click="goToAdminPath('/admin/channels/monitor')"
                    >
                      {{ t('admin.dashboard.gatewayBriefMonitor') }}
                    </button>
                    <button
                      type="button"
                      class="dashboard-brief-action rounded-xl border border-gray-100 px-3 py-2 text-xs font-semibold text-gray-700 hover:text-blue-600 dark:border-gray-700 dark:text-gray-200 dark:hover:text-blue-300"
                      @click="goToAdminPath('/admin/usage')"
                    >
                      {{ t('admin.dashboard.gatewayBriefUsage') }}
                    </button>
                    <button
                      type="button"
                      class="dashboard-brief-action rounded-xl border border-gray-100 px-3 py-2 text-xs font-semibold text-gray-700 hover:text-blue-600 dark:border-gray-700 dark:text-gray-200 dark:hover:text-blue-300"
                      @click="goToAdminPath('/admin/fingerprints')"
                    >
                      {{ t('admin.dashboard.gatewayBriefFingerprints') }}
                    </button>
                  </div>
                </div>
              </div>

              <div class="dashboard-insight-stack">
                <div
                  v-if="showMemberPulseCard"
                  class="card dashboard-analytics-card relative overflow-hidden p-4"
                >
                  <div
                    v-if="rankingLoading"
                    class="absolute inset-0 z-10 flex items-center justify-center bg-white/50 backdrop-blur-sm dark:bg-dark-800/50"
                  >
                    <LoadingSpinner size="md" />
                  </div>
                  <div class="mb-4 flex items-start justify-between gap-3">
                    <div>
                      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
                        {{ t('admin.dashboard.memberPulse') }}
                      </h3>
                      <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                        {{ t('admin.dashboard.memberPulseHint') }}
                      </p>
                    </div>
                    <span class="shrink-0 rounded-full border border-cyan-400/30 bg-cyan-500/10 px-2 py-1 text-[11px] font-medium text-cyan-600 dark:text-cyan-300">
                      {{ t('admin.dashboard.nonAdminOnly') }}
                    </span>
                  </div>

                  <div v-if="memberPulseItems.length" class="space-y-4">
                    <div class="grid grid-cols-2 gap-2">
                      <div class="rounded-xl border border-gray-100 p-3 dark:border-gray-700">
                        <p class="text-xs text-gray-500 dark:text-gray-400">
                          {{ t('admin.dashboard.topThreeSpendShare') }}
                        </p>
                        <p class="mt-1 text-lg font-bold text-gray-900 dark:text-white">
                          {{ formatPercent(topThreeCostShare) }}
                        </p>
                      </div>
                      <div class="rounded-xl border border-gray-100 p-3 dark:border-gray-700">
                        <p class="text-xs text-gray-500 dark:text-gray-400">
                          {{ t('admin.dashboard.avgSpendPerRequest') }}
                        </p>
                        <p class="mt-1 text-lg font-bold text-gray-900 dark:text-white">
                          {{ formatCurrency(avgSpendPerRequest) }}
                        </p>
                      </div>
                      <div class="rounded-xl border border-gray-100 p-3 dark:border-gray-700">
                        <p class="text-xs text-gray-500 dark:text-gray-400">
                          {{ t('admin.dashboard.avgSpendPerMember') }}
                        </p>
                        <p class="mt-1 text-lg font-bold text-gray-900 dark:text-white">
                          {{ formatCurrency(avgSpendPerMember) }}
                        </p>
                      </div>
                      <div class="rounded-xl border border-gray-100 p-3 dark:border-gray-700">
                        <p class="text-xs text-gray-500 dark:text-gray-400">
                          {{ t('admin.dashboard.longTailMembers') }}
                        </p>
                        <p class="mt-1 text-lg font-bold text-gray-900 dark:text-white">
                          {{ formatNumber(longTailMemberCount) }}
                        </p>
                      </div>
                    </div>

                    <div class="rounded-2xl border border-gray-100 p-3 dark:border-gray-700">
                      <div class="mb-3 flex items-center justify-between">
                        <p class="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">
                          {{ t('admin.dashboard.nonAdminLeaderboard') }}
                        </p>
                        <p class="text-xs text-gray-500 dark:text-gray-400">
                          Top {{ memberPulseItems.length }}
                        </p>
                      </div>
                      <button
                        v-for="(item, index) in memberPulseItems"
                        :key="`pulse-${item.user_id}`"
                        type="button"
                        class="w-full rounded-xl px-2 py-2 text-left transition-colors hover:bg-gray-50 dark:hover:bg-gray-800/50"
                        @click="goToUserUsage(item)"
                      >
                        <div class="flex items-center gap-3">
                          <span
                            class="flex h-7 w-7 shrink-0 items-center justify-center rounded-full text-[11px] font-bold text-white"
                            :class="memberPulseAvatarClass(index)"
                          >
                            {{ index + 1 }}
                          </span>
                          <div class="min-w-0 flex-1">
                            <div class="flex items-center justify-between gap-2">
                              <p class="truncate text-sm font-semibold text-gray-900 dark:text-white" :title="userDisplayName(item)">
                                {{ userDisplayName(item) }}
                              </p>
                              <p class="shrink-0 text-xs text-gray-500 dark:text-gray-400">
                                {{ formatPercent(userCostShare(item.actual_cost)) }}
                              </p>
                            </div>
                            <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                              {{ formatCurrency(item.actual_cost) }} · {{ formatNumber(item.requests) }} {{ t('admin.dashboard.requests') }}
                            </p>
                            <div class="mt-1 h-1.5 overflow-hidden rounded-full bg-gray-100 dark:bg-gray-700">
                              <div
                                class="h-full rounded-full"
                                :class="userBarClass(index)"
                                :style="{ width: `${userCostBarWidth(item.actual_cost)}%` }"
                              />
                            </div>
                          </div>
                        </div>
                      </button>
                    </div>

                    <div
                      class="rounded-2xl border px-4 py-3 text-xs"
                      :class="memberPulseAdviceClass"
                    >
                      <p class="font-semibold">
                        {{ memberPulseAdviceTitle }}
                      </p>
                      <p class="mt-1 opacity-80">
                        {{ memberPulseAdviceBody }}
                      </p>
                    </div>
                  </div>
                  <div
                    v-else-if="!rankingLoading"
                    class="flex h-32 items-center justify-center text-sm text-gray-500 dark:text-gray-400"
                  >
                    {{ t('admin.dashboard.noDataAvailable') }}
                  </div>
                </div>

                <div
                  v-if="showGatewayRadarCard"
                  class="card dashboard-analytics-card dashboard-gateway-radar-card relative overflow-hidden p-4"
                >
                  <div class="mb-4 flex items-start justify-between gap-3">
                    <div>
                      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
                        {{ t('admin.dashboard.gatewayRadar') }}
                      </h3>
                      <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                        {{ t('admin.dashboard.gatewayRadarHint') }}
                      </p>
                    </div>
                    <span
                      class="shrink-0 rounded-full border px-2 py-1 text-[11px] font-semibold"
                      :class="gatewayRadarStatusClass"
                    >
                      {{ gatewayRadarStatusLabel }}
                    </span>
                  </div>

                  <div class="space-y-2.5">
                    <button
                      v-for="item in gatewayRadarItems"
                      :key="item.key"
                      type="button"
                      class="dashboard-radar-row w-full rounded-2xl border border-gray-100 px-3 py-2 text-left transition-colors hover:border-blue-200 hover:bg-blue-50/50 dark:border-gray-700 dark:hover:border-blue-400/30 dark:hover:bg-blue-500/10"
                      @click="goToAdminPath(item.path)"
                    >
                      <div class="mb-1.5 flex items-start justify-between gap-3">
                        <div class="min-w-0">
                          <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ item.label }}</p>
                          <p class="mt-0.5 truncate text-sm font-semibold text-gray-900 dark:text-white" :title="item.detail">
                            {{ item.value }}
                          </p>
                        </div>
                        <p class="shrink-0 text-right text-xs text-gray-500 dark:text-gray-400">
                          {{ item.detail }}
                        </p>
                      </div>
                      <div class="h-1.5 overflow-hidden rounded-full bg-gray-100 dark:bg-gray-700">
                        <div
                          class="h-full rounded-full"
                          :class="gatewayRadarToneClass(item.tone)"
                          :style="{ width: `${item.width}%` }"
                        />
                      </div>
                    </button>
                  </div>
                </div>
              </div>
            </div>

            <div
              v-if="showTeamUsageProfileCard"
              class="card dashboard-analytics-card dashboard-masonry-item dashboard-masonry-item-wide dashboard-team-profile-card relative overflow-hidden p-4"
            >
                <div
                  v-if="chartsLoading"
                  class="absolute inset-0 z-10 flex items-center justify-center bg-white/50 backdrop-blur-sm dark:bg-dark-800/50"
                >
                  <LoadingSpinner size="md" />
                </div>
                <div class="mb-4 flex items-start justify-between gap-3">
                  <div>
                    <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
                      团队使用画像
                    </h3>
                    <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                      工具、缓存、成员画像和近期会话，默认不统计管理员。
                    </p>
                  </div>
                  <span class="shrink-0 rounded-full border border-violet-400/30 bg-violet-500/10 px-2 py-1 text-[11px] font-medium text-violet-600 dark:text-violet-300">
                    {{ formatTokens(teamInsights?.total_tokens || 0) }}
                  </span>
                </div>

                <div v-if="teamInsights && hasTeamInsightsData" class="space-y-5">
                  <div v-if="teamSignalTips.length" class="dashboard-team-signal-grid">
                    <div
                      v-for="tip in teamSignalTips"
                      :key="tip.title"
                      class="rounded-2xl border px-3 py-2 text-xs"
                      :class="teamSignalToneClass(tip.tone)"
                    >
                      <div class="flex items-start justify-between gap-3">
                        <div class="min-w-0">
                          <p class="font-semibold">{{ tip.title }}</p>
                          <p class="mt-1 leading-5 opacity-80">{{ tip.description }}</p>
                        </div>
                        <span class="shrink-0 rounded-full bg-white/50 px-2 py-0.5 text-[10px] font-semibold dark:bg-black/20">
                          {{ tip.badge }}
                        </span>
                      </div>
                    </div>
                  </div>

                  <div class="dashboard-team-metrics-grid">
                    <div class="rounded-xl border border-gray-100 p-3 dark:border-gray-700">
                      <p class="text-xs text-gray-500 dark:text-gray-400">团队缓存率</p>
                      <p class="mt-1 text-lg font-bold text-emerald-600 dark:text-emerald-400">
                        {{ formatPercent(teamInsights.cache_share) }}
                      </p>
                      <p class="text-xs text-gray-500 dark:text-gray-400">
                        {{ formatTokens(teamInsights.cache_tokens) }}
                      </p>
                    </div>
                    <div class="rounded-xl border border-gray-100 p-3 dark:border-gray-700">
                      <p class="text-xs text-gray-500 dark:text-gray-400">工具 / 客户端</p>
                      <p class="mt-1 text-lg font-bold text-gray-900 dark:text-white">
                        {{ formatNumber(teamClientItems.length) }}
                      </p>
                      <p class="text-xs text-gray-500 dark:text-gray-400">
                        {{ formatNumber(teamInsights.total_requests) }} 请求
                      </p>
                    </div>
                  </div>

                  <div class="dashboard-team-profile-grid">
                    <div v-if="teamProfileItems.length" class="dashboard-team-profile-section space-y-2">
                      <p class="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">
                        成员画像
                      </p>
                      <button
                        v-for="profile in teamProfileItems"
                        :key="`profile-${profile.user_id}`"
                        type="button"
                        class="w-full rounded-xl border border-gray-100 p-3 text-left transition-colors hover:bg-gray-50 dark:border-gray-700 dark:hover:bg-gray-800/50"
                        @click="goToMemberUsage(profile.user_id)"
                      >
                        <div class="flex items-start justify-between gap-3">
                          <div class="min-w-0">
                            <p class="truncate text-sm font-semibold text-gray-900 dark:text-white" :title="memberDisplayName(profile)">
                              {{ memberDisplayName(profile) }}
                            </p>
                            <p class="mt-1 truncate text-xs text-gray-500 dark:text-gray-400" :title="`${profile.top_client || '-'} / ${profile.top_model || '-'}`">
                              {{ profile.top_client || '-' }} / {{ profile.top_model || '-' }}
                            </p>
                          </div>
                          <div class="shrink-0 text-right text-xs">
                            <p class="font-semibold text-gray-900 dark:text-white">
                              {{ formatTokens(profile.total_tokens) }}
                            </p>
                            <p class="text-emerald-600 dark:text-emerald-400">
                              缓存 {{ formatPercent(profile.cache_share) }}
                            </p>
                          </div>
                        </div>
                      </button>
                    </div>

                    <div v-if="teamClientItems.length" class="dashboard-team-profile-section space-y-2">
                      <p class="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">
                        客户端 / 工具分布
                      </p>
                      <div v-for="client in teamClientItems" :key="client.client" class="space-y-1">
                        <div class="flex items-center justify-between gap-3 text-xs">
                          <span class="truncate font-medium text-gray-900 dark:text-white">
                            {{ client.client || 'Unknown' }}
                          </span>
                          <span class="shrink-0 text-gray-500 dark:text-gray-400">
                            {{ formatPercent(client.token_share) }} / {{ formatTokens(client.total_tokens) }}
                          </span>
                        </div>
                        <div class="h-2 overflow-hidden rounded-full bg-gray-100 dark:bg-gray-700">
                          <div
                            class="h-full rounded-full bg-cyan-500"
                            :style="{ width: `${shareBarWidth(client.token_share)}%` }"
                          />
                        </div>
                      </div>
                    </div>

                    <div v-if="teamCacheItems.length" class="dashboard-team-profile-section space-y-2">
                      <p class="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">
                        缓存效率看板
                      </p>
                      <div
                        v-for="item in teamCacheItems"
                        :key="`${item.scope}-${item.user_id || item.model || item.label}`"
                        class="rounded-xl border border-gray-100 px-3 py-2 dark:border-gray-700"
                      >
                        <div class="flex items-center justify-between gap-3">
                          <p class="truncate text-xs font-semibold text-gray-900 dark:text-white" :title="item.label">
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

                    <div v-if="teamSessionItems.length" class="dashboard-team-profile-section space-y-2">
                      <p class="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">
                        近期会话聚合
                      </p>
                      <button
                        v-for="session in teamSessionItems"
                        :key="session.session_id"
                        type="button"
                        class="w-full rounded-xl border border-gray-100 px-3 py-2 text-left transition-colors hover:bg-gray-50 dark:border-gray-700 dark:hover:bg-gray-800/50"
                        @click="goToMemberUsage(session.user_id)"
                      >
                        <div class="flex items-start justify-between gap-3">
                          <div class="min-w-0">
                            <p class="truncate text-xs font-semibold text-gray-900 dark:text-white" :title="sessionDisplayName(session)">
                              {{ sessionDisplayName(session) }}
                            </p>
                            <p class="mt-1 truncate text-xs text-gray-500 dark:text-gray-400" :title="`${session.client} / ${session.model}`">
                              {{ session.client }} / {{ session.model }}
                            </p>
                          </div>
                          <div class="shrink-0 text-right text-xs text-gray-500 dark:text-gray-400">
                            <p>{{ formatTokens(session.total_tokens) }}</p>
                            <p>{{ formatCompactDateTime(session.last_seen) }}</p>
                          </div>
                        </div>
                      </button>
                    </div>
                  </div>
                </div>
                <div
                  v-else-if="!chartsLoading"
                  class="flex h-40 items-center justify-center text-sm text-gray-500 dark:text-gray-400"
                >
                  {{ t('admin.dashboard.noDataAvailable') }}
                </div>
            </div>
          </div>

          <!-- Member x Model Matrix -->
          <div v-if="showMatrixCard" class="card dashboard-analytics-card relative overflow-hidden p-4">
            <div
              v-if="chartsLoading"
              class="absolute inset-0 z-10 flex items-center justify-center bg-white/50 backdrop-blur-sm dark:bg-dark-800/50"
            >
              <LoadingSpinner size="md" />
            </div>
            <div class="mb-4 flex flex-wrap items-start justify-between gap-3">
              <div>
                <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
                  成员 × 模型矩阵
                </h3>
                <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                  观察团队成员在不同模型上的使用偏好与缓存占比。
                </p>
              </div>
              <span class="rounded-full border border-cyan-400/30 bg-cyan-500/10 px-2 py-1 text-[11px] font-medium text-cyan-600 dark:text-cyan-300">
                {{ matrixMembers.length }} 成员 / {{ matrixModels.length }} 模型
              </span>
            </div>
            <div v-if="hasMatrixData" class="overflow-x-auto">
              <table class="min-w-[860px] w-full text-xs">
                <thead>
                  <tr class="border-b border-gray-100 text-gray-500 dark:border-gray-700 dark:text-gray-400">
                    <th class="w-56 pb-2 text-left font-medium">成员</th>
                    <th
                      v-for="model in matrixModels"
                      :key="`matrix-head-${model}`"
                      class="min-w-32 pb-2 text-left font-medium"
                    >
                      <span class="block truncate" :title="model">{{ model }}</span>
                    </th>
                  </tr>
                </thead>
                <tbody>
                  <tr
                    v-for="member in matrixMembers"
                    :key="`matrix-member-${member.user_id}`"
                    class="border-b border-gray-100 dark:border-gray-700"
                  >
                    <td class="py-2 pr-3">
                      <button
                        type="button"
                        class="max-w-52 truncate text-left font-semibold text-gray-900 hover:text-blue-600 dark:text-white dark:hover:text-blue-300"
                        :title="memberDisplayName(member)"
                        @click="goToMemberUsage(member.user_id)"
                      >
                        {{ memberDisplayName(member) }}
                      </button>
                      <p class="text-[11px] text-gray-500 dark:text-gray-400">
                        {{ formatTokens(member.total_tokens) }}
                      </p>
                    </td>
                    <td
                      v-for="model in matrixModels"
                      :key="`matrix-cell-${member.user_id}-${model}`"
                      class="py-2 pr-3"
                    >
                      <div
                        v-if="matrixCell(member.user_id, model)"
                        class="relative overflow-hidden rounded-lg border border-gray-100 px-2 py-1.5 dark:border-gray-700"
                      >
                        <div
                          class="absolute inset-y-0 left-0 bg-blue-500/10"
                          :style="{ width: `${matrixCellWidth(matrixCell(member.user_id, model)?.total_tokens || 0)}%` }"
                        />
                        <div class="relative">
                          <p class="font-semibold text-gray-900 dark:text-white">
                            {{ formatTokens(matrixCell(member.user_id, model)?.total_tokens || 0) }}
                          </p>
                          <p class="text-[11px] text-gray-500 dark:text-gray-400">
                            {{ formatPercent(matrixCell(member.user_id, model)?.share_of_member || 0) }}
                            · 缓存 {{ formatPercent(matrixCell(member.user_id, model)?.cache_share || 0) }}
                          </p>
                        </div>
                      </div>
                      <span v-else class="text-gray-300 dark:text-gray-600">-</span>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
            <div
              v-else-if="!chartsLoading"
              class="flex h-32 items-center justify-center text-sm text-gray-500 dark:text-gray-400"
            >
              {{ t('admin.dashboard.noDataAvailable') }}
            </div>
          </div>

          <!-- Hourly Activity Heatmap -->
          <div v-if="showHourlyActivityCard" class="card dashboard-analytics-card relative overflow-hidden p-4">
            <div
              v-if="chartsLoading"
              class="absolute inset-0 z-10 flex items-center justify-center bg-white/50 backdrop-blur-sm dark:bg-dark-800/50"
            >
              <LoadingSpinner size="md" />
            </div>
            <h3 class="mb-4 text-sm font-semibold text-gray-900 dark:text-white">
              {{ t('admin.dashboard.hourlyActivity') }}
            </h3>
            <div
              v-if="peakActivityCell"
              class="mb-4 flex flex-wrap items-center justify-between gap-3 rounded-xl border border-emerald-500/20 bg-emerald-500/10 px-4 py-3"
            >
              <div>
                <p class="text-xs font-medium uppercase tracking-wide text-emerald-600 dark:text-emerald-300">
                  {{ t('admin.dashboard.peakActivity') }}
                </p>
                <p class="text-lg font-semibold text-gray-900 dark:text-white">
                  {{ peakActivityLabel }}
                </p>
              </div>
              <div class="text-right text-xs text-gray-500 dark:text-gray-400">
                <p>
                  {{ formatTokens(peakActivityCell.total_tokens) }}
                  {{ t('admin.dashboard.tokens') }}
                </p>
                <p>
                  {{ formatNumber(peakActivityCell.requests) }}
                  {{ t('admin.dashboard.requests') }}
                </p>
              </div>
            </div>
            <div v-if="hasHourlyActivityData" class="overflow-x-auto">
              <div class="grid min-w-[720px] grid-cols-[48px_repeat(24,minmax(20px,1fr))] gap-1 text-[10px]">
                <div />
                <div
                  v-for="hour in hours"
                  :key="`h-${hour}`"
                  class="text-center text-gray-400"
                >
                  {{ hour % 3 === 0 ? hour.toString().padStart(2, '0') : '' }}
                </div>
                <template v-for="weekday in weekdays" :key="`weekday-${weekday.value}`">
                  <div class="flex items-center text-gray-500 dark:text-gray-400">
                    {{ weekday.label }}
                  </div>
                  <div
                    v-for="hour in hours"
                    :key="`${weekday.value}-${hour}`"
                    :data-testid="`admin-hourly-activity-cell-${weekday.value}-${hour}`"
                    class="h-5 rounded"
                    :class="heatmapCellClass(getHeatmapCell(weekday.value, hour)?.total_tokens || 0)"
                    :title="heatmapTitle(weekday.value, hour)"
                  />
                </template>
              </div>
            </div>
            <div
              v-else-if="!chartsLoading"
              class="flex h-32 items-center justify-center text-sm text-gray-500 dark:text-gray-400"
            >
              {{ t('admin.dashboard.noDataAvailable') }}
            </div>
          </div>

          <!-- User Usage Trend (Full Width) -->
          <div v-if="showUserTrendCard" class="card dashboard-analytics-card p-4">
            <h3 class="mb-4 text-sm font-semibold text-gray-900 dark:text-white">
              {{ t('admin.dashboard.recentUsage') }} (Top 12)
            </h3>
            <div class="h-64">
              <div v-if="userTrendLoading" class="flex h-full items-center justify-center">
                <LoadingSpinner size="md" />
              </div>
              <Line v-else-if="userTrendChartData" :data="userTrendChartData" :options="lineOptions" />
              <div
                v-else-if="!userTrendLoading"
                class="flex h-full items-center justify-center text-sm text-gray-500 dark:text-gray-400"
              >
                {{ t('admin.dashboard.noDataAvailable') }}
              </div>
            </div>
          </div>
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { useAppStore } from '@/stores/app'

const { t } = useI18n()
import { adminAPI } from '@/api/admin'
import type {
  DashboardStats,
  TrendDataPoint,
  ModelStat,
  UsageInsightSummary,
  HourlyActivityHeatmapCell,
  UserUsageTrendPoint,
  UserSpendingRankingItem,
  TeamUsageInsights,
  MemberModelMatrixRow,
  MemberUsageProfile
} from '@/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Icon from '@/components/icons/Icon.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import Select from '@/components/common/Select.vue'
import ModelDistributionChart from '@/components/charts/ModelDistributionChart.vue'
import TokenUsageTrend from '@/components/charts/TokenUsageTrend.vue'
import { useAntiDesignMode } from '@/composables/useAntiDesignMode'

import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Tooltip,
  Legend,
  Filler
} from 'chart.js'
import { Line } from 'vue-chartjs'

// Register Chart.js components
ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Tooltip,
  Legend,
  Filler
)

const appStore = useAppStore()
const router = useRouter()
const stats = ref<DashboardStats | null>(null)
const loading = ref(false)
const chartsLoading = ref(false)
const userTrendLoading = ref(false)
const rankingLoading = ref(false)
const rankingError = ref(false)

// Chart data
const trendData = ref<TrendDataPoint[]>([])
const modelStats = ref<ModelStat[]>([])
const usageInsights = ref<UsageInsightSummary | null>(null)
const teamInsights = ref<TeamUsageInsights | null>(null)
const hourlyActivity = ref<HourlyActivityHeatmapCell[]>([])
const userTrend = ref<UserUsageTrendPoint[]>([])
const rankingItems = ref<UserSpendingRankingItem[]>([])
const rankingTotalActualCost = ref(0)
const rankingTotalRequests = ref(0)
const rankingTotalTokens = ref(0)
const rankingTotalUsers = ref(0)
let chartLoadSeq = 0
let usersTrendLoadSeq = 0
let rankingLoadSeq = 0
const rankingLimit = 12
const { antiDesignMode } = useAntiDesignMode()

// Helper function to format date in local timezone
const formatLocalDate = (date: Date): string => {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
}

const getLast24HoursRangeDates = (): { start: string; end: string } => {
  const end = new Date()
  const start = new Date(end.getTime() - 24 * 60 * 60 * 1000)
  return {
    start: formatLocalDate(start),
    end: formatLocalDate(end)
  }
}

const dashboardTimezone = (): string | undefined => {
  return Intl.DateTimeFormat().resolvedOptions().timeZone || undefined
}

// Date range
const granularity = ref<'day' | 'hour'>('hour')
const defaultRange = getLast24HoursRangeDates()
const startDate = ref(defaultRange.start)
const endDate = ref(defaultRange.end)

// Granularity options for Select component
const granularityOptions = computed(() => [
  { value: 'day', label: t('admin.dashboard.day') },
  { value: 'hour', label: t('admin.dashboard.hour') }
])

// Dark mode detection
const isDarkMode = computed(() => {
  return document.documentElement.classList.contains('dark')
})

// Chart colors
const chartColors = computed(() => ({
  text: isDarkMode.value ? '#e5e7eb' : '#374151',
  grid: isDarkMode.value ? '#374151' : '#e5e7eb'
}))

// Line chart options (for user trend chart)
const lineOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  interaction: {
    intersect: false,
    mode: 'index' as const
  },
  plugins: {
    legend: {
      position: 'top' as const,
      labels: {
        color: chartColors.value.text,
        usePointStyle: true,
        pointStyle: 'circle',
        padding: 15,
        font: {
          size: 11
        }
      }
    },
    tooltip: {
      itemSort: (a: any, b: any) => {
        const aValue = typeof a?.raw === 'number' ? a.raw : Number(a?.parsed?.y ?? 0)
        const bValue = typeof b?.raw === 'number' ? b.raw : Number(b?.parsed?.y ?? 0)
        return bValue - aValue
      },
      callbacks: {
        label: (context: any) => {
          return `${context.dataset.label}: ${formatTokens(context.raw)}`
        }
      }
    }
  },
  scales: {
    x: {
      grid: {
        color: chartColors.value.grid
      },
      ticks: {
        color: chartColors.value.text,
        font: {
          size: 10
        }
      }
    },
    y: {
      grid: {
        color: chartColors.value.grid
      },
      ticks: {
        color: chartColors.value.text,
        font: {
          size: 10
        },
        callback: (value: string | number) => formatTokens(Number(value))
      }
    }
  }
}))

// User trend chart data
const userTrendChartData = computed(() => {
  if (!userTrend.value?.length) return null

  const getDisplayName = (point: UserUsageTrendPoint): string => {
    const username = point.username?.trim()
    if (username) {
      return username
    }

    const email = point.email?.trim()
    if (email) {
      return email
    }

    return t('admin.redeem.userPrefix', { id: point.user_id })
  }

  // Group by user_id to avoid merging different users with the same display name
  const userGroups = new Map<number, { name: string; data: Map<string, number> }>()
  const allDates = new Set<string>()

  userTrend.value.forEach((point) => {
    allDates.add(point.date)
    const key = point.user_id
    if (!userGroups.has(key)) {
      userGroups.set(key, { name: getDisplayName(point), data: new Map() })
    }
    userGroups.get(key)!.data.set(point.date, point.tokens)
  })

  const sortedDates = Array.from(allDates).sort()
  const colors = [
    '#3b82f6',
    '#10b981',
    '#f59e0b',
    '#ef4444',
    '#8b5cf6',
    '#ec4899',
    '#14b8a6',
    '#f97316',
    '#6366f1',
    '#84cc16',
    '#06b6d4',
    '#a855f7'
  ]

  const datasets = Array.from(userGroups.values()).map((group, idx) => ({
    label: group.name,
    data: sortedDates.map((date) => group.data.get(date) || 0),
    borderColor: colors[idx % colors.length],
    backgroundColor: `${colors[idx % colors.length]}20`,
    fill: false,
    tension: 0.3
  }))

  return {
    labels: sortedDates,
    datasets
  }
})

// Format helpers
const formatTokens = (value: number | undefined): string => {
  if (value === undefined || value === null) return '0'
  if (value >= 1_000_000_000) {
    return `${(value / 1_000_000_000).toFixed(2)}B`
  } else if (value >= 1_000_000) {
    return `${(value / 1_000_000).toFixed(2)}M`
  } else if (value >= 1_000) {
    return `${(value / 1_000).toFixed(2)}K`
  }
  return value.toLocaleString()
}

const formatNumber = (value: number): string => {
  return value.toLocaleString()
}

const formatCost = (value: number | null | undefined): string => {
  const safeValue = Number.isFinite(value) ? Number(value) : 0
  if (safeValue >= 1000) {
    return (safeValue / 1000).toFixed(2) + 'K'
  } else if (safeValue >= 1) {
    return safeValue.toFixed(2)
  } else if (safeValue >= 0.01) {
    return safeValue.toFixed(3)
  }
  return safeValue.toFixed(4)
}

const formatCurrency = (value: number | null | undefined): string => `$${formatCost(value)}`

const formatDuration = (ms: number): string => {
  if (ms >= 1000) {
    return `${(ms / 1000).toFixed(2)}s`
  }
  return `${Math.round(ms)}ms`
}

const goToUserUsage = (item: UserSpendingRankingItem) => {
  void router.push({
    path: '/admin/usage',
    query: {
      user_id: String(item.user_id),
      start_date: startDate.value,
      end_date: endDate.value
    }
  })
}

const goToMemberUsage = (userID: number) => {
  if (!userID) return
  void router.push({
    path: '/admin/usage',
    query: {
      user_id: String(userID),
      start_date: startDate.value,
      end_date: endDate.value
    }
  })
}

const goToAdminPath = (path: string) => {
  void router.push({ path })
}

const formatPercent = (value: number | null | undefined): string => `${((value || 0) * 100).toFixed(1)}%`

const shareBarWidth = (share: number | null | undefined): number => {
  if (!share || share <= 0) return 0
  return Math.max(4, Math.round(share * 100))
}

type UserContributionItem = UserSpendingRankingItem & { isOther?: boolean }

const userDisplayName = (item: Pick<UserSpendingRankingItem, 'user_id' | 'email'>): string => {
  const email = item.email?.trim()
  if (email) return email
  return t('admin.redeem.userPrefix', { id: item.user_id })
}

const memberDisplayName = (
  item: Pick<MemberUsageProfile | MemberModelMatrixRow, 'user_id' | 'email' | 'username'>
): string => {
  const email = item.email?.trim()
  if (email) return email
  const username = item.username?.trim()
  if (username) return username
  return t('admin.redeem.userPrefix', { id: item.user_id })
}

const userInitials = (item: Pick<UserSpendingRankingItem, 'user_id' | 'email'>): string => {
  const name = userDisplayName(item)
  const first = name.trim().charAt(0)
  return first ? first.toUpperCase() : '#'
}

const rankedUserTokens = computed(() => {
  return rankingItems.value.reduce((sum, item) => sum + item.tokens, 0)
})

const rankedUserCost = computed(() => {
  return rankingItems.value.reduce((sum, item) => sum + item.actual_cost, 0)
})

const activeUsageUserCount = computed(() => {
  return rankingTotalUsers.value || rankingItems.value.length
})

const topUser = computed<UserSpendingRankingItem | null>(() => rankingItems.value[0] || null)

const topUserDisplayName = computed(() => {
  return topUser.value ? userDisplayName(topUser.value) : '-'
})

const userCostShare = (cost: number): number => {
  if (rankingTotalActualCost.value <= 0) return 0
  return cost / rankingTotalActualCost.value
}

const userCostBarWidth = (cost: number): number => {
  if (rankingTotalActualCost.value <= 0) return 0
  return Math.max(4, Math.round(userCostShare(cost) * 100))
}

const topUserCostShare = computed(() => {
  return topUser.value ? userCostShare(topUser.value.actual_cost) : 0
})

const memberPulseItems = computed(() => rankingItems.value.slice(0, 5))

const topThreeCostShare = computed(() => {
  if (rankingTotalActualCost.value <= 0) return 0
  const topThreeCost = rankingItems.value.slice(0, 3).reduce((sum, item) => sum + item.actual_cost, 0)
  return topThreeCost / rankingTotalActualCost.value
})

const avgSpendPerRequest = computed(() => {
  if (rankingTotalRequests.value <= 0) return 0
  return rankingTotalActualCost.value / rankingTotalRequests.value
})

const avgSpendPerMember = computed(() => {
  if (activeUsageUserCount.value <= 0) return 0
  return rankingTotalActualCost.value / activeUsageUserCount.value
})

type GatewayBriefMetric = {
  key: string
  label: string
  value: string
  detail: string
}

type GatewayBriefAlert = {
  title: string
  description: string
  actionLabel: string
  path: string
  tone: 'blue' | 'emerald' | 'amber' | 'red'
}

type GatewayRadarItem = {
  key: string
  label: string
  value: string
  detail: string
  path: string
  width: number
  tone: 'blue' | 'emerald' | 'amber' | 'red' | 'cyan'
}

const gatewayAccountIssueCount = computed(() => {
  const current = stats.value
  if (!current) return 0
  return (current.error_accounts || 0) + (current.ratelimit_accounts || 0) + (current.overload_accounts || 0)
})

const gatewayCacheShare = computed(() => {
  return usageInsights.value?.cache_share || teamInsights.value?.cache_share || 0
})

const gatewayCacheTokens = computed(() => {
  return usageInsights.value?.cache_tokens || teamInsights.value?.cache_tokens || 0
})

const gatewayRequestCount = computed(() => {
  return rankingTotalRequests.value || usageInsights.value?.requests || stats.value?.today_requests || 0
})

const gatewayBriefStatusLabel = computed(() => {
  if (gatewayAccountIssueCount.value > 0) return t('admin.dashboard.gatewayBriefNeedsAttention')
  if (topUserCostShare.value >= 0.6) return t('admin.dashboard.gatewayBriefWatch')
  return t('admin.dashboard.gatewayBriefHealthy')
})

const gatewayBriefStatusClass = computed(() => {
  if (gatewayAccountIssueCount.value > 0) {
    return 'border-amber-400/30 bg-amber-500/10 text-amber-700 dark:text-amber-300'
  }
  if (topUserCostShare.value >= 0.6) {
    return 'border-cyan-400/30 bg-cyan-500/10 text-cyan-700 dark:text-cyan-300'
  }
  return 'border-emerald-400/30 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300'
})

const gatewayBriefMetrics = computed<GatewayBriefMetric[]>(() => {
  const current = stats.value
  const accountTotal = current?.total_accounts || 0
  const normalAccounts = current?.normal_accounts || 0
  const requestCount = gatewayRequestCount.value
  return [
    {
      key: 'spend',
      label: t('admin.dashboard.gatewayBriefSpend'),
      value: formatCurrency(rankingTotalActualCost.value || current?.today_actual_cost || 0),
      detail: `${formatCurrency(avgSpendPerRequest.value)} / ${t('admin.dashboard.requestsShort')}`
    },
    {
      key: 'health',
      label: t('admin.dashboard.gatewayBriefHealth'),
      value: gatewayAccountIssueCount.value > 0
        ? `${gatewayAccountIssueCount.value} ${t('common.error')}`
        : t('admin.dashboard.gatewayBriefHealthOk'),
      detail: `${normalAccounts} / ${accountTotal} ${t('common.active')}`
    },
    {
      key: 'cache',
      label: t('admin.dashboard.gatewayBriefCache'),
      value: formatPercent(gatewayCacheShare.value),
      detail: formatTokens(gatewayCacheTokens.value)
    },
    {
      key: 'traffic',
      label: t('admin.dashboard.gatewayBriefTraffic'),
      value: formatNumber(requestCount),
      detail: `${formatNumber(activeUsageUserCount.value)} ${t('admin.dashboard.member')}`
    }
  ]
})

const gatewayBriefAlerts = computed<GatewayBriefAlert[]>(() => {
  const alerts: GatewayBriefAlert[] = []
  const current = stats.value

  if (gatewayAccountIssueCount.value > 0 && current) {
    alerts.push({
      title: t('admin.dashboard.gatewayAlertAccountIssues'),
      description: t('admin.dashboard.gatewayAlertAccountIssuesDesc', {
        count: gatewayAccountIssueCount.value,
        total: current.total_accounts || 0
      }),
      actionLabel: t('admin.dashboard.gatewayBriefAccounts'),
      path: '/admin/accounts',
      tone: 'amber'
    })
  }

  if (topUserCostShare.value >= 0.6 && topUser.value) {
    alerts.push({
      title: t('admin.dashboard.gatewayAlertSpendConcentration'),
      description: t('admin.dashboard.gatewayAlertSpendConcentrationDesc', {
        member: topUserDisplayName.value,
        share: formatPercent(topUserCostShare.value)
      }),
      actionLabel: t('admin.dashboard.gatewayBriefUsage'),
      path: '/admin/usage',
      tone: 'blue'
    })
  }

  if (gatewayCacheShare.value > 0 && gatewayCacheShare.value < 0.25) {
    alerts.push({
      title: t('admin.dashboard.gatewayAlertLowCache'),
      description: t('admin.dashboard.gatewayAlertLowCacheDesc', {
        share: formatPercent(gatewayCacheShare.value)
      }),
      actionLabel: t('admin.dashboard.gatewayBriefUsage'),
      path: '/admin/usage',
      tone: 'amber'
    })
  }

  if (!alerts.length) {
    alerts.push({
      title: t('admin.dashboard.gatewayAlertHealthy'),
      description: t('admin.dashboard.gatewayAlertHealthyDesc'),
      actionLabel: t('admin.dashboard.gatewayBriefMonitor'),
      path: '/admin/channels/monitor',
      tone: 'emerald'
    })
  }

  return alerts.slice(0, 3)
})

const clampPercent = (value: number): number => {
  if (!Number.isFinite(value)) return 0
  return Math.max(4, Math.min(100, Math.round(value)))
}

const showGatewayRadarCard = computed(() => Boolean(stats.value))

const gatewayRadarStatusLabel = computed(() => {
  if (gatewayAccountIssueCount.value > 0) return t('admin.dashboard.gatewayRadarNeedsAttention')
  if (stats.value && stats.value.average_duration_ms > 10_000) return t('admin.dashboard.gatewayRadarSlow')
  if (topUserCostShare.value >= 0.6) return t('admin.dashboard.gatewayRadarConcentrated')
  return t('admin.dashboard.gatewayRadarStable')
})

const gatewayRadarStatusClass = computed(() => {
  if (gatewayAccountIssueCount.value > 0 || (stats.value && stats.value.average_duration_ms > 10_000)) {
    return 'border-amber-400/30 bg-amber-500/10 text-amber-700 dark:text-amber-300'
  }
  if (topUserCostShare.value >= 0.6) {
    return 'border-cyan-400/30 bg-cyan-500/10 text-cyan-700 dark:text-cyan-300'
  }
  return 'border-emerald-400/30 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300'
})

const gatewayRadarItems = computed<GatewayRadarItem[]>(() => {
  const current = stats.value
  if (!current) return []

  const totalAccounts = current.total_accounts || 0
  const normalAccounts = current.normal_accounts || 0
  const accountHealth = totalAccounts > 0 ? (normalAccounts / totalAccounts) * 100 : 0
  const latencyScore = current.average_duration_ms > 0
    ? Math.max(0, 100 - (current.average_duration_ms / 12_000) * 100)
    : 100
  const throughputScore = Math.min(100, Math.max(current.rpm || 0, current.tpm ? current.tpm / 100_000 : 0))
  const cacheScore = gatewayCacheShare.value * 100
  const concentrationScore = Math.max(0, 100 - topUserCostShare.value * 100)

  return [
    {
      key: 'accounts',
      label: t('admin.dashboard.gatewayRadarAccounts'),
      value: `${normalAccounts} / ${totalAccounts}`,
      detail: gatewayAccountIssueCount.value > 0
        ? t('admin.dashboard.gatewayRadarIssues', { count: gatewayAccountIssueCount.value })
        : t('admin.dashboard.gatewayRadarAllNormal'),
      path: '/admin/accounts',
      width: clampPercent(accountHealth),
      tone: gatewayAccountIssueCount.value > 0 ? 'amber' : 'emerald'
    },
    {
      key: 'latency',
      label: t('admin.dashboard.gatewayRadarLatency'),
      value: formatDuration(current.average_duration_ms || 0),
      detail: current.average_duration_ms > 10_000
        ? t('admin.dashboard.gatewayRadarCheck')
        : t('admin.dashboard.gatewayRadarAcceptable'),
      path: '/admin/channels/monitor',
      width: clampPercent(latencyScore),
      tone: current.average_duration_ms > 10_000 ? 'amber' : 'blue'
    },
    {
      key: 'throughput',
      label: t('admin.dashboard.gatewayRadarThroughput'),
      value: `${formatTokens(current.rpm)} RPM`,
      detail: `${formatTokens(current.tpm)} TPM`,
      path: '/admin/usage',
      width: clampPercent(throughputScore),
      tone: 'cyan'
    },
    {
      key: 'cache',
      label: t('admin.dashboard.gatewayRadarCache'),
      value: formatPercent(gatewayCacheShare.value),
      detail: formatTokens(gatewayCacheTokens.value),
      path: '/admin/usage',
      width: clampPercent(cacheScore),
      tone: gatewayCacheShare.value >= 0.45 ? 'emerald' : 'amber'
    },
    {
      key: 'concentration',
      label: t('admin.dashboard.gatewayRadarDistribution'),
      value: topUser.value ? formatPercent(1 - topUserCostShare.value) : '-',
      detail: topUser.value
        ? `Top ${formatPercent(topUserCostShare.value)}`
        : t('admin.dashboard.gatewayRadarNoRanking'),
      path: '/admin/usage',
      width: clampPercent(concentrationScore),
      tone: topUserCostShare.value >= 0.6 ? 'amber' : 'blue'
    }
  ]
})

const longTailMemberCount = computed(() => {
  return Math.max(activeUsageUserCount.value - Math.min(activeUsageUserCount.value, 3), 0)
})

const memberPulseAvatarClass = (index: number): string => {
  const colors = [
    'bg-blue-500',
    'bg-emerald-500',
    'bg-amber-500',
    'bg-violet-500',
    'bg-pink-500'
  ]
  return colors[index % colors.length]
}

const memberPulseAdviceTone = computed<'amber' | 'emerald' | 'cyan' | 'blue'>(() => {
  if (activeUsageUserCount.value <= 1) return 'blue'
  if (topUserCostShare.value >= 0.6) return 'amber'
  if ((usageInsights.value?.cache_share || 0) >= 0.7) return 'emerald'
  return 'cyan'
})

const memberPulseAdviceClass = computed(() => {
  switch (memberPulseAdviceTone.value) {
    case 'amber':
      return 'border-amber-400/30 bg-amber-500/10 text-amber-700 dark:text-amber-300'
    case 'emerald':
      return 'border-emerald-400/30 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300'
    case 'blue':
      return 'border-blue-400/30 bg-blue-500/10 text-blue-700 dark:text-blue-300'
    default:
      return 'border-cyan-400/30 bg-cyan-500/10 text-cyan-700 dark:text-cyan-300'
  }
})

const memberPulseAdviceTitle = computed(() => {
  switch (memberPulseAdviceTone.value) {
    case 'amber':
      return t('admin.dashboard.memberPulseConcentratedTitle')
    case 'emerald':
      return t('admin.dashboard.memberPulseCacheTitle')
    case 'blue':
      return t('admin.dashboard.memberPulseSparseTitle')
    default:
      return t('admin.dashboard.memberPulseBalancedTitle')
  }
})

const memberPulseAdviceBody = computed(() => {
  switch (memberPulseAdviceTone.value) {
    case 'amber':
      return t('admin.dashboard.memberPulseConcentratedBody')
    case 'emerald':
      return t('admin.dashboard.memberPulseCacheBody')
    case 'blue':
      return t('admin.dashboard.memberPulseSparseBody')
    default:
      return t('admin.dashboard.memberPulseBalancedBody')
  }
})

const teamClientItems = computed(() => teamInsights.value?.client_distribution?.slice(0, 6) || [])
const teamProfileItems = computed(() => teamInsights.value?.member_profiles?.slice(0, 4) || [])
const teamCacheItems = computed(() => teamInsights.value?.cache_efficiency?.slice(0, 5) || [])
const teamSessionItems = computed(() => teamInsights.value?.sessions?.slice(0, 5) || [])

type TeamSignalTip = {
  title: string
  description: string
  badge: string
  tone: 'blue' | 'emerald' | 'amber' | 'violet'
}

const teamSignalTips = computed<TeamSignalTip[]>(() => {
  if (!hasTeamInsightsData.value || !teamInsights.value) return []

  const tips: TeamSignalTip[] = []
  const topMember = teamProfileItems.value[0]
  const topClient = teamClientItems.value[0]
  const cacheShare = teamInsights.value.cache_share || 0
  const latestSession = teamSessionItems.value[0]

  if (topMember) {
    const concentrated = topMember.token_share >= 0.55
    tips.push({
      title: concentrated ? '成员用量集中' : '最高贡献成员',
      description: `${memberDisplayName(topMember)} 占团队 ${formatPercent(topMember.token_share)}，主要使用 ${topMember.top_model || '-'}。`,
      badge: formatTokens(topMember.total_tokens),
      tone: concentrated ? 'amber' : 'blue'
    })
  }

  if (topClient) {
    tips.push({
      title: '主要客户端',
      description: `${topClient.client || 'Unknown'} 覆盖 ${formatNumber(topClient.member_count)} 名成员，占 ${formatPercent(topClient.token_share)} 用量。`,
      badge: formatTokens(topClient.total_tokens),
      tone: 'violet'
    })
  }

  if (cacheShare >= 0.45) {
    tips.push({
      title: '缓存效率健康',
      description: `团队缓存占比 ${formatPercent(cacheShare)}，长上下文复用情况较好。`,
      badge: formatTokens(teamInsights.value.cache_tokens),
      tone: 'emerald'
    })
  } else if (cacheShare > 0) {
    tips.push({
      title: '缓存效率可优化',
      description: `团队缓存占比 ${formatPercent(cacheShare)}，建议关注高频提示词和上下文复用策略。`,
      badge: formatTokens(teamInsights.value.cache_tokens),
      tone: 'amber'
    })
  }

  if (latestSession && tips.length < 3) {
    tips.push({
      title: '最近活跃会话',
      description: `${sessionDisplayName(latestSession)} 通过 ${latestSession.client || 'Unknown'} 使用 ${latestSession.model || '-'}。`,
      badge: formatCompactDateTime(latestSession.last_seen),
      tone: 'emerald'
    })
  }

  return tips.slice(0, 3)
})

const teamSignalToneClass = (tone: TeamSignalTip['tone']): string => {
  switch (tone) {
    case 'amber':
      return 'border-amber-400/30 bg-amber-500/10 text-amber-700 dark:text-amber-300'
    case 'emerald':
      return 'border-emerald-400/30 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300'
    case 'violet':
      return 'border-violet-400/30 bg-violet-500/10 text-violet-700 dark:text-violet-300'
    default:
      return 'border-blue-400/30 bg-blue-500/10 text-blue-700 dark:text-blue-300'
  }
}

const gatewayBriefAlertClass = (tone: GatewayBriefAlert['tone']): string => {
  switch (tone) {
    case 'red':
      return 'border-red-400/30 bg-red-500/10 text-red-700 hover:border-red-400/50 dark:text-red-300'
    case 'amber':
      return 'border-amber-400/30 bg-amber-500/10 text-amber-700 hover:border-amber-400/50 dark:text-amber-300'
    case 'emerald':
      return 'border-emerald-400/30 bg-emerald-500/10 text-emerald-700 hover:border-emerald-400/50 dark:text-emerald-300'
    default:
      return 'border-blue-400/30 bg-blue-500/10 text-blue-700 hover:border-blue-400/50 dark:text-blue-300'
  }
}

const gatewayRadarToneClass = (tone: GatewayRadarItem['tone']): string => {
  switch (tone) {
    case 'red':
      return 'bg-red-500'
    case 'amber':
      return 'bg-amber-500'
    case 'emerald':
      return 'bg-emerald-500'
    case 'cyan':
      return 'bg-cyan-500'
    default:
      return 'bg-blue-500'
  }
}

type MatrixMember = {
  user_id: number
  email: string
  username: string
  total_tokens: number
}

const matrixMembers = computed<MatrixMember[]>(() => {
  const members = new Map<number, MatrixMember>()
  for (const row of teamInsights.value?.member_model_matrix || []) {
    const current = members.get(row.user_id)
    const total = Math.max(row.member_total_tokens || 0, row.total_tokens || 0)
    if (!current || total > current.total_tokens) {
      members.set(row.user_id, {
        user_id: row.user_id,
        email: row.email,
        username: row.username,
        total_tokens: total
      })
    }
  }
  return Array.from(members.values()).sort((a, b) => b.total_tokens - a.total_tokens).slice(0, 8)
})

const matrixModels = computed(() => {
  const totals = new Map<string, number>()
  for (const row of teamInsights.value?.member_model_matrix || []) {
    totals.set(row.model, (totals.get(row.model) || 0) + row.total_tokens)
  }
  return Array.from(totals.entries())
    .sort((a, b) => b[1] - a[1])
    .slice(0, 8)
    .map(([model]) => model)
})

const matrixMaxTokens = computed(() => {
  return Math.max(0, ...(teamInsights.value?.member_model_matrix || []).map((row) => row.total_tokens))
})

const matrixCell = (userID: number, model: string): MemberModelMatrixRow | undefined => {
  return teamInsights.value?.member_model_matrix?.find(
    (row) => row.user_id === userID && row.model === model
  )
}

const matrixCellWidth = (tokens: number): number => {
  if (!tokens || matrixMaxTokens.value <= 0) return 0
  return Math.max(8, Math.round((tokens / matrixMaxTokens.value) * 100))
}

const sessionDisplayName = (session: { user_id: number; email: string; username: string }): string => {
  return memberDisplayName(session)
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

const otherUserContribution = computed<UserContributionItem | null>(() => {
  const otherTokens = Math.max(rankingTotalTokens.value - rankedUserTokens.value, 0)
  const otherRequests = Math.max(
    rankingTotalRequests.value - rankingItems.value.reduce((sum, item) => sum + item.requests, 0),
    0
  )
  const otherActualCost = Math.max(rankingTotalActualCost.value - rankedUserCost.value, 0)
  if (otherTokens <= 0 && otherRequests <= 0 && otherActualCost <= 0.000001) return null
  return {
    user_id: 0,
    email: '',
    actual_cost: otherActualCost,
    requests: otherRequests,
    tokens: otherTokens,
    isOther: true
  }
})

const userContributionItems = computed<UserContributionItem[]>(() => {
  const items: UserContributionItem[] = rankingItems.value.slice(0, rankingLimit)
  return otherUserContribution.value ? [...items, otherUserContribution.value] : items
})

const contributionDisplayName = (item: UserContributionItem): string => {
  if (item.isOther) return t('admin.dashboard.otherMembers')
  return userDisplayName(item)
}

const userBarClass = (index: number, isOther?: boolean): string => {
  if (isOther) return 'bg-slate-400 dark:bg-slate-500'
  const colors = [
    'bg-blue-500',
    'bg-emerald-500',
    'bg-amber-500',
    'bg-violet-500',
    'bg-pink-500',
    'bg-cyan-500'
  ]
  return colors[index % colors.length]
}

const tokenCompositionSegments = computed(() => {
  const insight = usageInsights.value
  if (!insight || insight.total_tokens <= 0) return []

  const segments = [
    {
      key: 'input',
      label: t('admin.dashboard.input'),
      tokens: insight.input_tokens,
      share: insight.input_share,
      class: 'bg-blue-500'
    },
    {
      key: 'output',
      label: t('admin.dashboard.output'),
      tokens: insight.output_tokens,
      share: insight.output_share,
      class: 'bg-violet-500'
    },
    {
      key: 'cache_creation',
      label: t('admin.dashboard.cacheCreation'),
      tokens: insight.cache_creation_tokens,
      share: insight.cache_creation_share,
      class: 'bg-amber-500'
    },
    {
      key: 'cache_read',
      label: t('admin.dashboard.cacheRead'),
      tokens: insight.cache_read_tokens,
      share: insight.cache_read_share,
      class: 'bg-emerald-500'
    }
  ]

  return segments.map((segment) => ({
    ...segment,
    width: segment.tokens > 0 ? Math.max(segment.share * 100, 3) : 0
  }))
})

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
const maxHourlyTokens = computed(() => Math.max(0, ...hourlyActivity.value.map((cell) => cell.total_tokens)))
const getHeatmapCell = (weekday: number, hour: number) => {
  return hourlyActivity.value.find((cell) => cell.weekday === weekday && cell.hour === hour)
}
const heatmapCellClass = (tokens: number): string => {
  if (!tokens || maxHourlyTokens.value <= 0) return 'bg-gray-100 dark:bg-gray-800'
  const ratio = tokens / maxHourlyTokens.value
  if (ratio >= 0.75) return 'bg-emerald-600'
  if (ratio >= 0.5) return 'bg-emerald-500'
  if (ratio >= 0.25) return 'bg-emerald-400'
  return 'bg-emerald-200 dark:bg-emerald-700'
}
const heatmapTitle = (weekday: number, hour: number): string => {
  const cell = getHeatmapCell(weekday, hour)
  const tokens = cell?.total_tokens || 0
  const requests = cell?.requests || 0
  return `${weekdays[weekday]?.label || weekday} ${hour.toString().padStart(2, '0')}:00 - ${formatTokens(tokens)} tokens - ${formatNumber(requests)} requests`
}
const peakActivityCell = computed(() => {
  const peak = hourlyActivity.value.reduce<HourlyActivityHeatmapCell | null>((currentPeak, cell) => {
    if (!currentPeak || cell.total_tokens > currentPeak.total_tokens) {
      return cell
    }
    return currentPeak
  }, null)
  return peak && peak.total_tokens > 0 ? peak : null
})
const peakActivityLabel = computed(() => {
  const cell = peakActivityCell.value
  if (!cell || cell.total_tokens <= 0) return '-'
  const weekday = weekdays.find((item) => item.value === cell.weekday)?.label || String(cell.weekday)
  return `${weekday} ${cell.hour.toString().padStart(2, '0')}:00`
})

const hasUsageValue = (...values: Array<number | null | undefined>): boolean => {
  return values.some((value) => (value || 0) > 0)
}

const hasModelDistributionData = computed(() => {
  return modelStats.value.some((item) => hasUsageValue(
    item.requests,
    item.total_tokens,
    item.actual_cost,
    item.account_cost,
    item.cost
  ))
})

const hasTrendData = computed(() => {
  return trendData.value.some((item) => hasUsageValue(
    item.requests,
    item.total_tokens,
    item.actual_cost,
    item.cost
  ))
})

const hasRankingData = computed(() => {
  return hasUsageValue(
    rankingTotalRequests.value,
    rankingTotalTokens.value,
    rankingTotalActualCost.value
  ) || rankingItems.value.some((item) => hasUsageValue(item.requests, item.tokens, item.actual_cost))
})

const hasRankingSpendData = computed(() => {
  return hasUsageValue(rankingTotalActualCost.value) ||
    rankingItems.value.some((item) => hasUsageValue(item.actual_cost))
})

const hasMemberPulseData = computed(() => {
  return memberPulseItems.value.some((item) => hasUsageValue(item.requests, item.actual_cost))
})

const hasUsageInsightsData = computed(() => {
  return hasUsageValue(usageInsights.value?.requests, usageInsights.value?.total_tokens)
})

const hasTeamInsightsData = computed(() => {
  return hasUsageValue(teamInsights.value?.total_requests, teamInsights.value?.total_tokens)
})

const hasMatrixData = computed(() => {
  return matrixMembers.value.length > 0 &&
    matrixModels.value.length > 0 &&
    (teamInsights.value?.member_model_matrix || []).some((item) => hasUsageValue(item.requests, item.total_tokens))
})

const hasHourlyActivityData = computed(() => {
  return hourlyActivity.value.some((item) => hasUsageValue(item.requests, item.total_tokens))
})

const hasUserTrendData = computed(() => {
  return userTrend.value.some((item) => hasUsageValue(item.requests, item.tokens, item.actual_cost, item.cost))
})

const showModelDistributionCard = computed(() => chartsLoading.value || hasModelDistributionData.value)
const showTrendCard = computed(() => chartsLoading.value || hasTrendData.value)
const showChartCards = computed(() => showModelDistributionCard.value || showTrendCard.value)
const showUserContributionCard = computed(() => rankingLoading.value || rankingError.value || hasRankingData.value)
const showUsageInsightsCard = computed(() => chartsLoading.value || hasUsageInsightsData.value)
const showMemberPulseCard = computed(() => rankingLoading.value || hasMemberPulseData.value)
const showTeamUsageProfileCard = computed(() => chartsLoading.value || hasTeamInsightsData.value)
const showGatewayBriefCard = computed(() => Boolean(stats.value) || hasRankingData.value || hasUsageInsightsData.value)
const showCompactInsightSection = computed(() => (
  showUsageInsightsCard.value ||
  showGatewayBriefCard.value ||
  showMemberPulseCard.value ||
  showGatewayRadarCard.value
))
const chartGridClass = computed(() => {
  return showModelDistributionCard.value && showTrendCard.value
    ? 'dashboard-chart-grid--split'
    : 'dashboard-chart-grid--single'
})
const insightCardCount = computed(() => [
  showUserContributionCard.value,
  showUsageInsightsCard.value,
  showGatewayBriefCard.value,
  showMemberPulseCard.value,
  showTeamUsageProfileCard.value
].filter(Boolean).length)
const showInsightCards = computed(() => {
  return insightCardCount.value > 0
})
const showMatrixCard = computed(() => chartsLoading.value || hasMatrixData.value)
const showHourlyActivityCard = computed(() => chartsLoading.value || hasHourlyActivityData.value)
const showUserTrendCard = computed(() => userTrendLoading.value || hasUserTrendData.value)

// Date range change handler
const onDateRangeChange = (range: {
  startDate: string
  endDate: string
  preset: string | null
}) => {
  // Auto-select granularity based on date range
  const start = new Date(range.startDate)
  const end = new Date(range.endDate)
  const daysDiff = Math.ceil((end.getTime() - start.getTime()) / (1000 * 60 * 60 * 24))

  // If range is 1 day, use hourly granularity
  if (daysDiff <= 1) {
    granularity.value = 'hour'
  } else {
    granularity.value = 'day'
  }

  loadChartData()
}

// Load data
const loadDashboardSnapshot = async (includeStats: boolean) => {
  const currentSeq = ++chartLoadSeq
  if (includeStats && !stats.value) {
    loading.value = true
  }
  chartsLoading.value = true
  try {
    const params = {
      start_date: startDate.value,
      end_date: endDate.value
    }
    const [snapshotResult, insightsResult, activityResult, teamInsightsResult] = await Promise.allSettled([
      adminAPI.dashboard.getSnapshotV2({
        ...params,
        granularity: granularity.value,
        include_stats: includeStats,
        include_trend: true,
        include_model_stats: true,
        include_group_stats: false,
        include_users_trend: false
      }),
      adminAPI.dashboard.getUsageInsights(params),
      adminAPI.dashboard.getHourlyActivity({
        ...params,
        timezone: dashboardTimezone()
      }),
      adminAPI.dashboard.getTeamUsageInsights({
        ...params,
        limit: rankingLimit
      })
    ])

    if (currentSeq !== chartLoadSeq) return
    if (snapshotResult.status === 'rejected') {
      throw snapshotResult.reason
    }

    const response = snapshotResult.value
    if (insightsResult.status === 'fulfilled') {
      usageInsights.value = insightsResult.value.insights || null
    } else {
      usageInsights.value = null
      console.error('Error loading admin usage insights:', insightsResult.reason)
    }
    if (activityResult.status === 'fulfilled') {
      hourlyActivity.value = activityResult.value.hourly_activity || []
    } else {
      hourlyActivity.value = []
      console.error('Error loading admin hourly activity:', activityResult.reason)
    }
    if (teamInsightsResult.status === 'fulfilled') {
      teamInsights.value = teamInsightsResult.value.insights || null
    } else {
      teamInsights.value = null
      console.error('Error loading admin team usage insights:', teamInsightsResult.reason)
    }

    if (includeStats && response.stats) {
      stats.value = response.stats
    }
    trendData.value = response.trend || []
    modelStats.value = response.models || []
  } catch (error) {
    if (currentSeq !== chartLoadSeq) return
    appStore.showError(t('admin.dashboard.failedToLoad'))
    console.error('Error loading dashboard snapshot:', error)
  } finally {
    if (currentSeq === chartLoadSeq) {
      loading.value = false
      chartsLoading.value = false
    }
  }
}

const loadUsersTrend = async () => {
  const currentSeq = ++usersTrendLoadSeq
  userTrendLoading.value = true
  try {
    const response = await adminAPI.dashboard.getUserUsageTrend({
      start_date: startDate.value,
      end_date: endDate.value,
      granularity: granularity.value,
      limit: 12
    })
    if (currentSeq !== usersTrendLoadSeq) return
    userTrend.value = response.trend || []
  } catch (error) {
    if (currentSeq !== usersTrendLoadSeq) return
    console.error('Error loading users trend:', error)
    userTrend.value = []
  } finally {
    if (currentSeq === usersTrendLoadSeq) {
      userTrendLoading.value = false
    }
  }
}

const loadUserSpendingRanking = async () => {
  const currentSeq = ++rankingLoadSeq
  rankingLoading.value = true
  rankingError.value = false
  try {
    const response = await adminAPI.dashboard.getUserSpendingRanking({
      start_date: startDate.value,
      end_date: endDate.value,
      limit: rankingLimit
    })
    if (currentSeq !== rankingLoadSeq) return
    rankingItems.value = response.ranking || []
    rankingTotalActualCost.value = response.total_actual_cost || 0
    rankingTotalRequests.value = response.total_requests || 0
    rankingTotalTokens.value = response.total_tokens || 0
    rankingTotalUsers.value = response.total_users || rankingItems.value.length
  } catch (error) {
    if (currentSeq !== rankingLoadSeq) return
    console.error('Error loading user spending ranking:', error)
    rankingItems.value = []
    rankingTotalActualCost.value = 0
    rankingTotalRequests.value = 0
    rankingTotalTokens.value = 0
    rankingTotalUsers.value = 0
    rankingError.value = true
  } finally {
    if (currentSeq === rankingLoadSeq) {
      rankingLoading.value = false
    }
  }
}

const loadDashboardStats = async () => {
  await Promise.all([
    loadDashboardSnapshot(true),
    loadUsersTrend(),
    loadUserSpendingRanking()
  ])
}

const loadChartData = async () => {
  await Promise.all([
    loadDashboardSnapshot(false),
    loadUsersTrend(),
    loadUserSpendingRanking()
  ])
}

onMounted(() => {
  loadDashboardStats()
})
</script>

<style scoped>
.admin-dashboard-polish {
  --dashboard-accent: 37 99 235;
  --dashboard-accent-soft: 59 130 246;
  --dashboard-surface: 255 255 255;
  --dashboard-surface-soft: 248 250 252;
  --dashboard-surface-muted: 241 245 249;
  --dashboard-border: 226 232 240;
  --dashboard-border-strong: 203 213 225;
  --dashboard-ink-shadow: 15 23 42;
  --dashboard-ring: 59 130 246;
  --metric-accent: var(--dashboard-accent);
  position: relative;
  isolation: isolate;
  padding-bottom: 0.25rem;
}

.dark .admin-dashboard-polish {
  --dashboard-surface: 15 23 42;
  --dashboard-surface-soft: 30 41 59;
  --dashboard-surface-muted: 51 65 85;
  --dashboard-border: 51 65 85;
  --dashboard-border-strong: 71 85 105;
  --dashboard-ink-shadow: 0 0 0;
}

.admin-dashboard-polish::before {
  content: "";
  position: absolute;
  inset: -1.5rem -1rem auto;
  z-index: -1;
  height: 16rem;
  pointer-events: none;
  background:
    radial-gradient(circle at 78% 0%, rgb(var(--dashboard-accent) / 0.08), transparent 34%),
    linear-gradient(180deg, rgb(var(--dashboard-surface-soft) / 0.54), transparent 78%);
  opacity: 0.9;
}

.dark .admin-dashboard-polish::before {
  background:
    radial-gradient(circle at 78% 0%, rgb(var(--dashboard-accent-soft) / 0.13), transparent 34%),
    linear-gradient(180deg, rgb(15 23 42 / 0.74), transparent 78%);
  opacity: 0.72;
}

.admin-dashboard-polish :deep(.card) {
  border-color: rgb(var(--dashboard-border) / 0.72);
  box-shadow:
    0 1px 2px rgb(var(--dashboard-ink-shadow) / 0.035),
    0 16px 34px -30px rgb(var(--dashboard-ink-shadow) / 0.28);
}

.dark .admin-dashboard-polish :deep(.card) {
  border-color: rgb(var(--dashboard-border) / 0.68);
  box-shadow:
    0 1px 0 rgb(255 255 255 / 0.035) inset,
    0 16px 36px -30px rgb(0 0 0 / 0.55);
}

.dashboard-stat-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(min(100%, 13.25rem), 1fr));
  gap: clamp(0.75rem, 1.3vw, 1rem);
  align-items: stretch;
}

.dashboard-stat-card {
  --metric-accent: var(--dashboard-accent);
  position: relative;
  isolation: isolate;
  overflow: hidden;
  min-height: 6.75rem;
  padding: clamp(0.85rem, 1.2vw, 1rem);
  background: linear-gradient(180deg, rgb(var(--dashboard-surface) / 0.98), rgb(var(--dashboard-surface-soft) / 0.86));
  box-shadow:
    0 1px 2px rgb(var(--dashboard-ink-shadow) / 0.04),
    0 14px 30px -26px rgb(var(--dashboard-ink-shadow) / 0.32),
    inset 0 1px 0 rgb(255 255 255 / 0.70);
  transform: translateZ(0);
  transition:
    border-color 180ms ease,
    box-shadow 180ms ease,
    transform 180ms ease,
    background-color 180ms ease;
}

.dashboard-stat-grid .dashboard-stat-card:nth-child(1) {
  --metric-accent: 37 99 235;
}

.dashboard-stat-grid .dashboard-stat-card:nth-child(2) {
  --metric-accent: 124 58 237;
}

.dashboard-stat-grid .dashboard-stat-card:nth-child(3) {
  --metric-accent: 22 163 74;
}

.dashboard-stat-grid .dashboard-stat-card:nth-child(4) {
  --metric-accent: 5 150 105;
}

.dashboard-stat-grid:nth-of-type(2) .dashboard-stat-card:nth-child(1) {
  --metric-accent: 217 119 6;
}

.dashboard-stat-grid:nth-of-type(2) .dashboard-stat-card:nth-child(2) {
  --metric-accent: 79 70 229;
}

.dashboard-stat-grid:nth-of-type(2) .dashboard-stat-card:nth-child(3) {
  --metric-accent: 124 58 237;
}

.dashboard-stat-grid:nth-of-type(2) .dashboard-stat-card:nth-child(4) {
  --metric-accent: 225 29 72;
}

.dark .dashboard-stat-card {
  background: linear-gradient(180deg, rgb(var(--dashboard-surface-soft) / 0.74), rgb(var(--dashboard-surface) / 0.62));
  box-shadow:
    0 1px 0 rgb(255 255 255 / 0.04) inset,
    0 16px 34px -28px rgb(0 0 0 / 0.62);
}

.dashboard-stat-card::before {
  content: "";
  position: absolute;
  inset: 0 0 auto;
  height: 2px;
  background: linear-gradient(90deg, rgb(var(--metric-accent) / 0.62), transparent 72%);
  opacity: 0.58;
}

.dashboard-stat-card::after {
  content: "";
  position: absolute;
  inset: 0;
  z-index: -1;
  pointer-events: none;
  background:
    radial-gradient(circle at 100% 0%, rgb(var(--metric-accent) / 0.075), transparent 42%),
    linear-gradient(180deg, transparent, rgb(var(--dashboard-surface-muted) / 0.18));
  opacity: 0.86;
  transition: opacity 180ms ease;
}

.dashboard-stat-card:hover {
  border-color: rgb(var(--metric-accent) / 0.28);
  box-shadow:
    0 1px 2px rgb(var(--dashboard-ink-shadow) / 0.045),
    0 18px 36px -28px rgb(var(--dashboard-ink-shadow) / 0.40),
    inset 0 1px 0 rgb(255 255 255 / 0.76);
  transform: translateY(-1px);
}

.dark .dashboard-stat-card:hover {
  border-color: rgb(var(--metric-accent) / 0.34);
  box-shadow:
    0 1px 0 rgb(255 255 255 / 0.055) inset,
    0 18px 38px -28px rgb(0 0 0 / 0.68);
}

.dashboard-stat-card:hover::after {
  opacity: 1;
}

.dashboard-stat-card > .flex > div:first-child {
  display: flex;
  width: 2.75rem;
  height: 2.75rem;
  align-items: center;
  justify-content: center;
  border: 1px solid rgb(var(--metric-accent) / 0.14);
  background: rgb(var(--metric-accent) / 0.095) !important;
  box-shadow:
    inset 0 1px 0 rgb(255 255 255 / 0.62),
    0 1px 2px rgb(var(--dashboard-ink-shadow) / 0.04);
}

.dark .dashboard-stat-card > .flex > div:first-child {
  border-color: rgb(var(--metric-accent) / 0.24);
  background: rgb(var(--metric-accent) / 0.14) !important;
  box-shadow: inset 0 1px 0 rgb(255 255 255 / 0.07);
}

.dashboard-stat-card > .flex > div:last-child {
  min-width: 0;
}

.dashboard-stat-card :is(.text-xl, .text-lg, .text-sm.font-semibold) {
  font-variant-numeric: tabular-nums;
  letter-spacing: -0.025em;
}

.dashboard-stat-card .text-xs {
  line-height: 1.35;
}

.dashboard-filter-panel {
  position: relative;
  z-index: 30;
  overflow: visible;
  border-color: rgb(var(--dashboard-border) / 0.78);
  background: rgb(var(--dashboard-surface) / 0.88);
  box-shadow:
    0 1px 2px rgb(var(--dashboard-ink-shadow) / 0.035),
    0 14px 34px -30px rgb(var(--dashboard-ink-shadow) / 0.32),
    inset 0 1px 0 rgb(255 255 255 / 0.72);
  backdrop-filter: blur(14px);
  transition:
    border-color 180ms ease,
    box-shadow 180ms ease;
}

.dark .dashboard-filter-panel {
  background: rgb(var(--dashboard-surface-soft) / 0.62);
  box-shadow:
    0 1px 0 rgb(255 255 255 / 0.045) inset,
    0 14px 34px -28px rgb(0 0 0 / 0.58);
}

.dashboard-filter-panel:focus-within {
  z-index: 70;
  border-color: rgb(var(--dashboard-ring) / 0.34);
  box-shadow:
    0 0 0 3px rgb(var(--dashboard-ring) / 0.08),
    0 14px 34px -30px rgb(var(--dashboard-ink-shadow) / 0.34),
    inset 0 1px 0 rgb(255 255 255 / 0.72);
}

.dashboard-filter-panel::before {
  content: "";
  position: absolute;
  inset: 0 auto 0 0;
  width: 3px;
  pointer-events: none;
  border-radius: inherit;
  background: linear-gradient(180deg, rgb(var(--dashboard-accent) / 0.66), rgb(var(--dashboard-accent-soft) / 0.12));
  opacity: 0.74;
}

.dashboard-filter-panel > * {
  position: relative;
  z-index: 1;
}

.dashboard-filter-panel :deep(.date-picker-dropdown) {
  z-index: 1000;
}

.dashboard-filter-panel :deep(.date-picker-trigger),
.dashboard-filter-panel :deep(.select-trigger),
.dashboard-filter-panel .btn-secondary {
  min-height: 2.5rem;
  border-radius: 0.75rem;
  border-color: rgb(var(--dashboard-border) / 0.86);
  background: rgb(var(--dashboard-surface) / 0.82);
  box-shadow: none;
  transition:
    border-color 160ms ease,
    background-color 160ms ease,
    box-shadow 160ms ease,
    transform 120ms ease;
}

.dashboard-filter-panel :deep(.date-picker-trigger:hover),
.dashboard-filter-panel :deep(.select-trigger:hover),
.dashboard-filter-panel .btn-secondary:hover {
  border-color: rgb(var(--dashboard-border-strong) / 0.95);
  background: rgb(var(--dashboard-surface-soft) / 0.74);
}

.dashboard-filter-panel .btn-secondary:active {
  transform: translateY(1px);
}

.dashboard-analytics-card {
  min-width: 0;
  border-radius: 1.25rem;
  border-color: rgb(var(--dashboard-border) / 0.76);
  background: linear-gradient(180deg, rgb(var(--dashboard-surface) / 0.98), rgb(var(--dashboard-surface-soft) / 0.82));
  box-shadow:
    0 1px 2px rgb(var(--dashboard-ink-shadow) / 0.04),
    0 16px 36px -30px rgb(var(--dashboard-ink-shadow) / 0.30),
    inset 0 1px 0 rgb(255 255 255 / 0.74);
  transition:
    border-color 180ms ease,
    box-shadow 180ms ease,
    transform 180ms ease,
    background-color 180ms ease;
}

.dark .dashboard-analytics-card {
  border-color: rgb(var(--dashboard-border) / 0.68);
  background: linear-gradient(180deg, rgb(var(--dashboard-surface-soft) / 0.68), rgb(var(--dashboard-surface) / 0.58));
  box-shadow:
    0 1px 0 rgb(255 255 255 / 0.04) inset,
    0 16px 36px -28px rgb(0 0 0 / 0.58);
}

.dashboard-analytics-card:hover {
  border-color: rgb(var(--dashboard-border-strong) / 0.84);
  box-shadow:
    0 1px 2px rgb(var(--dashboard-ink-shadow) / 0.045),
    0 16px 34px -30px rgb(var(--dashboard-ink-shadow) / 0.30),
    inset 0 1px 0 rgb(255 255 255 / 0.78);
}

.dark .dashboard-analytics-card:hover {
  border-color: rgb(var(--dashboard-border-strong) / 0.80);
  box-shadow:
    0 1px 0 rgb(255 255 255 / 0.055) inset,
    0 16px 34px -28px rgb(0 0 0 / 0.62);
}

.dashboard-analytics-card :deep(h3) {
  letter-spacing: -0.01em;
}

.dashboard-analytics-card :is(.rounded-lg.border, .rounded-xl.border, .rounded-2xl.border):not([class*="bg-"]) {
  border-color: rgb(var(--dashboard-border) / 0.68);
  background: linear-gradient(180deg, rgb(var(--dashboard-surface) / 0.56), rgb(var(--dashboard-surface-soft) / 0.42));
  box-shadow: inset 0 1px 0 rgb(255 255 255 / 0.54);
}

.dark .dashboard-analytics-card :is(.rounded-lg.border, .rounded-xl.border, .rounded-2xl.border):not([class*="bg-"]) {
  border-color: rgb(var(--dashboard-border) / 0.66);
  background: linear-gradient(180deg, rgb(var(--dashboard-surface-soft) / 0.40), rgb(var(--dashboard-surface) / 0.30));
  box-shadow: inset 0 1px 0 rgb(255 255 255 / 0.04);
}

.dashboard-analytics-card :is(.rounded-lg.border, .rounded-xl.border, .rounded-2xl.border) p {
  line-height: 1.35;
}

.dashboard-analytics-card button {
  transition:
    border-color 160ms ease,
    background-color 160ms ease,
    color 160ms ease,
    transform 120ms ease;
}

.dashboard-analytics-card button:not([class*="border-blue"]):not([class*="border-cyan"]):not([class*="border-violet"]):not([class*="border-amber"]):not([class*="border-emerald"]):hover {
  border-color: rgb(var(--dashboard-border-strong) / 0.88);
}

.dashboard-analytics-card button:active {
  transform: translateY(1px);
}

.dashboard-analytics-card button:focus-visible {
  outline: none;
  box-shadow: 0 0 0 3px rgb(var(--dashboard-ring) / 0.14);
}

.dashboard-chart-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: clamp(1rem, 1.8vw, 1.5rem);
  align-items: stretch;
}

.dashboard-chart-grid > * {
  min-width: 0;
}

.dashboard-analytics-card button.w-full:hover {
  background-color: rgb(var(--dashboard-surface-muted) / 0.34);
}

.dark .dashboard-analytics-card button.w-full:hover {
  background-color: rgb(var(--dashboard-surface-soft) / 0.44);
}

.dashboard-analytics-card table {
  border-collapse: separate;
  border-spacing: 0;
  font-variant-numeric: tabular-nums;
}

.dashboard-analytics-card table th {
  font-weight: 600;
  letter-spacing: 0.01em;
}

.dashboard-analytics-card table :is(th, td) {
  line-height: 1.45;
}

.dashboard-analytics-card tbody tr {
  transition: background-color 140ms ease;
}

.dashboard-analytics-card tbody tr:hover {
  background-color: rgb(var(--dashboard-surface-muted) / 0.28);
}

.dark .dashboard-analytics-card tbody tr:hover {
  background-color: rgb(var(--dashboard-surface-soft) / 0.32);
}

.dashboard-analytics-card :is(.h-2, .h-1\.5).overflow-hidden.rounded-full {
  background: rgb(var(--dashboard-surface-muted) / 0.68);
  box-shadow: inset 0 1px 1px rgb(var(--dashboard-ink-shadow) / 0.06);
}

.dark .dashboard-analytics-card :is(.h-2, .h-1\.5).overflow-hidden.rounded-full {
  background: rgb(var(--dashboard-surface-muted) / 0.54);
  box-shadow: inset 0 1px 1px rgb(0 0 0 / 0.28);
}

.dashboard-analytics-card td .relative.overflow-hidden.rounded-lg.border {
  transition:
    border-color 140ms ease,
    background-color 140ms ease;
}

.dashboard-analytics-card td .relative.overflow-hidden.rounded-lg.border:hover {
  border-color: rgb(var(--dashboard-ring) / 0.28);
  background: rgb(var(--dashboard-surface-soft) / 0.64);
}

.dark .dashboard-analytics-card td .relative.overflow-hidden.rounded-lg.border:hover {
  background: rgb(var(--dashboard-surface-soft) / 0.46);
}

.dashboard-analytics-card [data-testid^="admin-hourly-activity-cell"] {
  box-shadow: inset 0 0 0 1px rgb(255 255 255 / 0.20);
  transition:
    transform 120ms ease,
    box-shadow 120ms ease,
    filter 120ms ease;
}

.dashboard-analytics-card [data-testid^="admin-hourly-activity-cell"]:hover {
  filter: saturate(1.06) brightness(1.04);
  transform: scale(1.035);
  box-shadow:
    inset 0 0 0 1px rgb(255 255 255 / 0.34),
    0 2px 8px rgb(var(--dashboard-ink-shadow) / 0.10);
}

.dashboard-masonry {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: clamp(1rem, 1.8vw, 1.5rem);
  align-items: start;
}

.dashboard-masonry-item {
  width: 100%;
  align-self: start;
}

.dashboard-masonry-item-wide {
  grid-column: 1 / -1;
}

.dashboard-insight-columns {
  display: block;
  columns: 1;
  column-gap: clamp(1rem, 1.8vw, 1.5rem);
}

.dashboard-insight-stack {
  display: contents;
}

.dashboard-insight-columns .dashboard-analytics-card {
  width: 100%;
  margin: 0 0 clamp(1rem, 1.8vw, 1.5rem);
  break-inside: avoid;
  page-break-inside: avoid;
}

.dashboard-team-signal-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: 0.5rem;
}

.dashboard-team-metrics-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.5rem;
}

.dashboard-team-profile-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(min(100%, 14rem), 1fr));
  gap: 1rem;
  align-items: start;
}

.dashboard-team-profile-section {
  min-width: 0;
}

.dashboard-cost-summary {
  background:
    radial-gradient(circle at 100% 0%, rgb(16 185 129 / 0.10), transparent 36%),
    linear-gradient(180deg, rgb(var(--dashboard-surface) / 0.66), rgb(var(--dashboard-surface-soft) / 0.48));
}

.dashboard-gateway-brief-card {
  min-height: 0;
}

.dashboard-gateway-brief-card::before {
  content: "";
  position: absolute;
  inset: 0 0 auto;
  height: 2px;
  pointer-events: none;
  background: linear-gradient(90deg, rgb(14 165 233 / 0.58), rgb(16 185 129 / 0.18), transparent);
}

.dashboard-brief-metric,
.dashboard-brief-action,
.dashboard-radar-row {
  min-width: 0;
}

.dashboard-gateway-radar-card::before {
  content: "";
  position: absolute;
  inset: 0 auto 0 0;
  width: 2px;
  pointer-events: none;
  background: linear-gradient(180deg, rgb(59 130 246 / 0.56), rgb(14 165 233 / 0.20), transparent);
}

.admin-dashboard-anti {
  --dashboard-accent: 255 0 0;
  --dashboard-accent-soft: 0 0 255;
  --dashboard-surface: 255 235 59;
  --dashboard-surface-soft: 255 255 255;
  --dashboard-surface-muted: 0 255 0;
  --dashboard-border: 0 0 0;
  --dashboard-border-strong: 0 0 0;
  --dashboard-ink-shadow: 0 0 0;
  --dashboard-ring: 255 0 0;
  padding: 1rem;
  color: #050505;
  font-family: Impact, "Arial Black", "Comic Sans MS", ui-sans-serif, system-ui, sans-serif;
  letter-spacing: -0.015em;
  background:
    repeating-linear-gradient(135deg, rgb(0 0 0 / 0.09) 0 10px, transparent 10px 22px),
    radial-gradient(circle at 82% 4%, #00ff00 0 9rem, transparent 9.2rem),
    linear-gradient(135deg, #ffeb3b 0%, #ffffff 48%, #00ffff 100%);
}

.dark .admin-dashboard-anti {
  --dashboard-surface: 255 235 59;
  --dashboard-surface-soft: 255 255 255;
  --dashboard-surface-muted: 0 255 0;
  --dashboard-border: 0 0 0;
  --dashboard-border-strong: 0 0 0;
  color: #050505;
  background:
    repeating-linear-gradient(135deg, rgb(255 255 255 / 0.10) 0 10px, transparent 10px 22px),
    radial-gradient(circle at 82% 4%, #00ff00 0 9rem, transparent 9.2rem),
    linear-gradient(135deg, #120012 0%, #ff00ff 46%, #ffeb3b 100%);
}

.admin-dashboard-anti::before {
  inset: -1rem;
  height: 18rem;
  background:
    repeating-linear-gradient(90deg, #000 0 12px, transparent 12px 24px),
    radial-gradient(circle at 12% 30%, #ff0000 0 5.5rem, transparent 5.7rem);
  opacity: 0.18;
  transform: rotate(-2deg);
}

.admin-dashboard-anti::after {
  content: "NO RULES / LIVE DATA / DON'T PANIC";
  position: absolute;
  top: 0.25rem;
  right: 1.25rem;
  z-index: 0;
  border: 6px solid #000;
  background: #00ff00;
  box-shadow: 9px 9px 0 #0000ff;
  color: #000;
  font-size: clamp(1.1rem, 2.4vw, 2.25rem);
  font-weight: 950;
  letter-spacing: -0.05em;
  line-height: 0.9;
  padding: 0.35rem 0.6rem;
  pointer-events: none;
  transform: rotate(4deg);
}

.admin-dashboard-anti .dashboard-stat-grid {
  gap: 1.1rem;
}

.admin-dashboard-anti .dashboard-stat-grid:nth-of-type(1) {
  transform: rotate(-0.7deg);
}

.admin-dashboard-anti .dashboard-stat-grid:nth-of-type(2) {
  transform: rotate(0.55deg);
}

.admin-dashboard-anti :deep(.card),
.admin-dashboard-anti .dashboard-stat-card,
.admin-dashboard-anti .dashboard-filter-panel,
.admin-dashboard-anti .dashboard-analytics-card {
  border: 6px solid #050505;
  border-radius: 0.45rem;
  background: #fff;
  box-shadow: 10px 10px 0 #050505;
  color: #050505;
}

.admin-dashboard-anti .dashboard-stat-card {
  min-height: 7.15rem;
  background:
    linear-gradient(90deg, rgb(var(--metric-accent) / 0.20), transparent 62%),
    #fff;
}

.admin-dashboard-anti .dashboard-stat-grid .dashboard-stat-card:nth-child(odd) {
  transform: rotate(-1.4deg);
}

.admin-dashboard-anti .dashboard-stat-grid .dashboard-stat-card:nth-child(even) {
  transform: rotate(1.1deg) translateY(0.15rem);
}

.admin-dashboard-anti .dashboard-stat-card:hover,
.admin-dashboard-anti .dashboard-analytics-card:hover {
  border-color: #050505;
  box-shadow: 14px 14px 0 #ff0000;
  transform: rotate(-0.7deg) translate(-2px, -2px);
}

.admin-dashboard-anti .dashboard-stat-card::before {
  height: 8px;
  background: repeating-linear-gradient(90deg, #050505 0 14px, #ffeb3b 14px 26px, #ff0000 26px 38px);
  opacity: 1;
}

.admin-dashboard-anti .dashboard-stat-card::after {
  background: radial-gradient(circle at 100% 10%, rgb(var(--metric-accent) / 0.25), transparent 44%);
  opacity: 1;
}

.admin-dashboard-anti .dashboard-stat-card > .flex > div:first-child {
  border: 4px solid #050505;
  border-radius: 0;
  background: #ffeb3b !important;
  box-shadow: 5px 5px 0 #050505;
  transform: rotate(-5deg);
}

.admin-dashboard-anti .dashboard-stat-card :is(.text-xl, .text-lg, .text-sm.font-semibold) {
  color: #050505 !important;
  font-size: 1.55rem;
  letter-spacing: -0.08em;
  text-transform: uppercase;
}

.admin-dashboard-anti .dashboard-stat-card p:first-child,
.admin-dashboard-anti h3,
.admin-dashboard-anti .text-xs.font-medium.uppercase {
  color: #050505 !important;
  font-family: Impact, "Arial Black", ui-sans-serif, system-ui, sans-serif;
  text-transform: uppercase;
}

.admin-dashboard-anti .dashboard-filter-panel {
  background: #fff;
  transform: rotate(-0.55deg);
}

.admin-dashboard-anti .dashboard-filter-panel::before {
  width: 14px;
  border-radius: 0;
  background: repeating-linear-gradient(180deg, #ff0000 0 16px, #0000ff 16px 32px, #00ff00 32px 48px);
  opacity: 1;
}

.admin-dashboard-anti .dashboard-filter-panel :deep(.date-picker-trigger),
.admin-dashboard-anti .dashboard-filter-panel :deep(.select-trigger),
.admin-dashboard-anti .dashboard-filter-panel .btn-secondary {
  border: 4px solid #050505;
  border-radius: 0;
  background: #ffeb3b;
  box-shadow: 5px 5px 0 #050505;
  color: #050505;
  font-weight: 950;
  text-transform: uppercase;
}

.admin-dashboard-anti .dashboard-filter-panel .btn-secondary:hover {
  background: #00ff00;
  transform: rotate(-2deg) translate(-1px, -1px);
}

.admin-dashboard-anti .dashboard-filter-panel :deep(.date-picker-dropdown) {
  border: 6px solid #050505;
  border-radius: 0;
  background: #fff;
  box-shadow: 14px 14px 0 #0000ff;
  color: #050505;
}

.admin-dashboard-anti .dashboard-analytics-card {
  background:
    linear-gradient(180deg, #fff, #fff 64%, #ffeb3b 64%, #ffeb3b 100%);
}

.admin-dashboard-anti .dashboard-cost-summary {
  background:
    repeating-linear-gradient(135deg, rgb(0 0 0 / 0.18) 0 8px, transparent 8px 16px),
    #00ffff;
  box-shadow: 8px 8px 0 #ff0000;
}

.admin-dashboard-anti .dashboard-gateway-brief-card {
  background:
    repeating-linear-gradient(90deg, rgb(255 0 0 / 0.18) 0 12px, transparent 12px 24px),
    #fff;
}

.admin-dashboard-anti .dashboard-gateway-radar-card {
  background:
    repeating-linear-gradient(135deg, rgb(0 0 255 / 0.16) 0 10px, transparent 10px 20px),
    #fff;
}

.admin-dashboard-anti .dashboard-gateway-brief-card::before {
  height: 8px;
  background: repeating-linear-gradient(90deg, #0000ff 0 16px, #00ff00 16px 32px, #ff0000 32px 48px);
}

.admin-dashboard-anti .dashboard-gateway-radar-card::before {
  width: 8px;
  background: repeating-linear-gradient(180deg, #ff0000 0 16px, #00ff00 16px 32px, #0000ff 32px 48px);
}

.admin-dashboard-anti .dashboard-brief-action {
  border: 4px solid #050505;
  border-radius: 0;
  background: #ffeb3b;
  box-shadow: 4px 4px 0 #050505;
  color: #050505 !important;
}

.admin-dashboard-anti .dashboard-masonry {
  gap: 2rem;
}

.admin-dashboard-anti .dashboard-insight-columns .dashboard-analytics-card {
  margin-bottom: 2rem;
}

.admin-dashboard-anti .dashboard-masonry-item:nth-child(odd) {
  transform: rotate(-0.75deg);
}

.admin-dashboard-anti .dashboard-masonry-item:nth-child(even) {
  transform: rotate(0.85deg);
}

.admin-dashboard-anti .dashboard-analytics-card :is(.rounded-lg.border, .rounded-xl.border, .rounded-2xl.border):not([class*="bg-"]) {
  border: 4px solid #050505;
  border-radius: 0;
  background: #fff;
  box-shadow: 6px 6px 0 #00ff00;
}

.admin-dashboard-anti .dashboard-analytics-card button {
  border-width: 4px;
  border-radius: 0.15rem;
}

.admin-dashboard-anti .dashboard-analytics-card button:hover {
  background: #ffeb3b !important;
  color: #050505;
  transform: rotate(-1.5deg) translate(-1px, -1px);
}

.admin-dashboard-anti .dashboard-analytics-card button:active {
  transform: rotate(1deg) translate(2px, 2px);
}

.admin-dashboard-anti .dashboard-analytics-card table {
  border: 4px solid #050505;
  background: #fff;
}

.admin-dashboard-anti .dashboard-analytics-card table th {
  background: #050505;
  color: #ffeb3b !important;
  text-transform: uppercase;
}

.admin-dashboard-anti .dashboard-analytics-card table td {
  border-top: 3px solid #050505;
  color: #050505 !important;
}

.admin-dashboard-anti .dashboard-analytics-card tbody tr:hover {
  background: #00ff00;
}

.admin-dashboard-anti .dashboard-analytics-card :is(.h-2, .h-1\.5).overflow-hidden.rounded-full,
.admin-dashboard-anti .dashboard-analytics-card .flex.h-3.overflow-hidden.rounded-full {
  border: 3px solid #050505;
  border-radius: 0;
  background: #fff !important;
  box-shadow: none;
}

.admin-dashboard-anti .dashboard-analytics-card td .relative.overflow-hidden.rounded-lg.border {
  border: 4px solid #050505;
  border-radius: 0;
  background: #fff;
  box-shadow: 5px 5px 0 #ffeb3b;
}

.admin-dashboard-anti .dashboard-analytics-card [data-testid^="admin-hourly-activity-cell"] {
  border-radius: 0;
  border: 2px solid #050505;
  box-shadow: none;
}

.admin-dashboard-anti .dashboard-analytics-card [data-testid^="admin-hourly-activity-cell"]:hover {
  filter: none;
  transform: rotate(7deg) scale(1.12);
  box-shadow: 4px 4px 0 #ff0000;
}

@media (min-width: 1024px) {
  .dashboard-chart-grid--split {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .dashboard-chart-grid--single {
    grid-template-columns: minmax(0, 1fr);
  }

  .dashboard-masonry {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .dashboard-masonry--single {
    grid-template-columns: minmax(0, 1fr);
  }

  .dashboard-insight-columns {
    columns: 2 28rem;
  }

  .dashboard-team-signal-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  .dashboard-team-profile-grid {
    grid-template-columns: repeat(auto-fit, minmax(min(100%, 14.5rem), 1fr));
  }
}

@media (min-width: 1280px) {
  .dashboard-stat-grid {
    grid-template-columns: repeat(4, minmax(0, 1fr));
  }

  .dashboard-team-profile-grid {
    grid-template-columns: repeat(auto-fit, minmax(min(100%, 15rem), 1fr));
  }
}

@media (min-width: 1536px) {
  .dashboard-insight-columns {
    columns: 2 31rem;
  }
}

@media (max-width: 640px) {
  .admin-dashboard-polish {
    gap: 1rem;
  }

  .dashboard-stat-card > .flex {
    align-items: flex-start;
  }

  .dashboard-stat-card > .flex > div:first-child {
    width: 2.35rem;
    height: 2.35rem;
  }

  .admin-dashboard-anti .dashboard-stat-grid,
  .admin-dashboard-anti .dashboard-masonry-item,
  .admin-dashboard-anti .dashboard-filter-panel {
    transform: none !important;
  }
}

@media (prefers-reduced-motion: reduce) {
  .admin-dashboard-polish::before,
  .dashboard-stat-card,
  .dashboard-stat-card::before,
  .dashboard-stat-card::after,
  .dashboard-filter-panel,
  .dashboard-filter-panel :deep(.date-picker-trigger),
  .dashboard-filter-panel :deep(.select-trigger),
  .dashboard-filter-panel .btn-secondary,
  .dashboard-analytics-card,
  .dashboard-analytics-card button,
  .dashboard-analytics-card [data-testid^="admin-hourly-activity-cell"] {
    transition-duration: 1ms;
  }

  .dashboard-stat-card:hover,
  .dashboard-analytics-card:hover,
  .dashboard-analytics-card button:active,
  .dashboard-analytics-card [data-testid^="admin-hourly-activity-cell"]:hover {
    transform: none;
  }
}
</style>
