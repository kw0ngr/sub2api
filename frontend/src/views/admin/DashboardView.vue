<template>
  <AppLayout>
    <div class="space-y-6">
      <!-- Loading State -->
      <div v-if="loading" class="flex items-center justify-center py-12">
        <LoadingSpinner />
      </div>

      <template v-else-if="stats">
        <!-- Row 1: Core Stats -->
        <div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
          <!-- Total API Keys -->
          <div class="card p-4">
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
          <div class="card p-4">
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
          <div class="card p-4">
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
          <div class="card p-4">
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
        <div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
          <!-- Today Tokens -->
          <div class="card p-4">
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
          <div class="card p-4">
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
          <div class="card p-4">
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
          <div class="card p-4">
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
          <div class="card p-4">
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
          <div class="grid grid-cols-1 gap-6 lg:grid-cols-2">
            <ModelDistributionChart
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
            <TokenUsageTrend :trend-data="trendData" :loading="chartsLoading" />
          </div>

          <!-- Team Member and Insight Widgets -->
          <div class="grid grid-cols-1 gap-6 lg:grid-cols-2 lg:items-start">
            <div class="card relative overflow-hidden p-4">
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
                            {{ formatPercent(topUserTokenShare) }}
                          </p>
                          <p class="text-[11px] text-gray-500 dark:text-gray-400">
                            {{ t('admin.dashboard.topMemberShare') }}
                          </p>
                        </div>
                      </div>
                      <div class="mt-3 grid grid-cols-2 gap-3 text-xs">
                        <div>
                          <p class="text-gray-500 dark:text-gray-400">{{ t('admin.dashboard.tokens') }}</p>
                          <p class="font-semibold text-gray-900 dark:text-white">{{ formatTokens(topUser.tokens) }}</p>
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
                      {{ t('admin.dashboard.totalTeamTokens') }}
                    </p>
                    <p class="mt-1 text-lg font-bold text-gray-900 dark:text-white">
                      {{ formatTokens(rankingTotalTokens) }}
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
                        {{ formatPercent(userTokenShare(item.tokens)) }} / {{ formatTokens(item.tokens) }}
                      </span>
                    </div>
                    <div class="h-2 overflow-hidden rounded-full bg-gray-100 dark:bg-gray-700">
                      <div
                        class="h-full rounded-full"
                        :class="userBarClass(index, item.isOther)"
                        :style="{ width: `${userBarWidth(item.tokens)}%` }"
                      />
                    </div>
                  </div>
                </div>

                <table class="mt-2 w-full text-xs">
                  <thead>
                    <tr class="text-gray-500 dark:text-gray-400">
                      <th class="pb-2 text-left">{{ t('admin.dashboard.member') }}</th>
                      <th class="pb-2 text-right">{{ t('admin.dashboard.requests') }}</th>
                      <th class="pb-2 text-right">{{ t('admin.dashboard.tokens') }}</th>
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
                        {{ formatTokens(item.tokens) }}
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
              <div
                v-else
                class="flex h-40 items-center justify-center text-sm text-gray-500 dark:text-gray-400"
              >
                {{ t('admin.dashboard.noDataAvailable') }}
              </div>
            </div>

            <div class="space-y-6">
              <div class="card relative overflow-hidden p-4">
              <div
                v-if="chartsLoading"
                class="absolute inset-0 z-10 flex items-center justify-center bg-white/50 backdrop-blur-sm dark:bg-dark-800/50"
              >
                <LoadingSpinner size="md" />
              </div>
              <h3 class="mb-4 text-sm font-semibold text-gray-900 dark:text-white">
                {{ t('admin.dashboard.usageInsights') }}
              </h3>
              <div
                v-if="usageInsights && usageInsights.total_tokens > 0"
                class="space-y-4"
              >
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
                      {{ formatPercent(topUserTokenShare) }} /
                      {{ formatTokens(topUser?.tokens || 0) }}
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
                v-else
                class="flex h-40 items-center justify-center text-sm text-gray-500 dark:text-gray-400"
              >
                {{ t('admin.dashboard.noDataAvailable') }}
              </div>
            </div>

            <div class="card relative overflow-hidden p-4">
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
                      {{ t('admin.dashboard.topThreeShare') }}
                    </p>
                    <p class="mt-1 text-lg font-bold text-gray-900 dark:text-white">
                      {{ formatPercent(topThreeTokenShare) }}
                    </p>
                  </div>
                  <div class="rounded-xl border border-gray-100 p-3 dark:border-gray-700">
                    <p class="text-xs text-gray-500 dark:text-gray-400">
                      {{ t('admin.dashboard.avgTokensPerRequest') }}
                    </p>
                    <p class="mt-1 text-lg font-bold text-gray-900 dark:text-white">
                      {{ formatTokens(avgTokensPerRequest) }}
                    </p>
                  </div>
                  <div class="rounded-xl border border-gray-100 p-3 dark:border-gray-700">
                    <p class="text-xs text-gray-500 dark:text-gray-400">
                      {{ t('admin.dashboard.avgTokensPerMember') }}
                    </p>
                    <p class="mt-1 text-lg font-bold text-gray-900 dark:text-white">
                      {{ formatTokens(avgTokensPerMember) }}
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
                            {{ formatPercent(userTokenShare(item.tokens)) }}
                          </p>
                        </div>
                        <div class="mt-1 h-1.5 overflow-hidden rounded-full bg-gray-100 dark:bg-gray-700">
                          <div
                            class="h-full rounded-full"
                            :class="userBarClass(index)"
                            :style="{ width: `${userBarWidth(item.tokens)}%` }"
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
                v-else
                class="flex h-32 items-center justify-center text-sm text-gray-500 dark:text-gray-400"
              >
                {{ t('admin.dashboard.noDataAvailable') }}
              </div>
              </div>
            </div>
          </div>

          <!-- Hourly Activity Heatmap -->
          <div class="card relative overflow-hidden p-4">
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
            <div v-if="hourlyActivity.length" class="overflow-x-auto">
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
              v-else
              class="flex h-32 items-center justify-center text-sm text-gray-500 dark:text-gray-400"
            >
              {{ t('admin.dashboard.noDataAvailable') }}
            </div>
          </div>

          <!-- User Usage Trend (Full Width) -->
          <div class="card p-4">
            <h3 class="mb-4 text-sm font-semibold text-gray-900 dark:text-white">
              {{ t('admin.dashboard.recentUsage') }} (Top 12)
            </h3>
            <div class="h-64">
              <div v-if="userTrendLoading" class="flex h-full items-center justify-center">
                <LoadingSpinner size="md" />
              </div>
              <Line v-else-if="userTrendChartData" :data="userTrendChartData" :options="lineOptions" />
              <div
                v-else
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
  UserSpendingRankingItem
} from '@/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Icon from '@/components/icons/Icon.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import Select from '@/components/common/Select.vue'
import ModelDistributionChart from '@/components/charts/ModelDistributionChart.vue'
import TokenUsageTrend from '@/components/charts/TokenUsageTrend.vue'

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

const formatPercent = (value: number | null | undefined): string => `${((value || 0) * 100).toFixed(1)}%`

type UserContributionItem = UserSpendingRankingItem & { isOther?: boolean }

const userDisplayName = (item: Pick<UserSpendingRankingItem, 'user_id' | 'email'>): string => {
  const email = item.email?.trim()
  if (email) return email
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

const activeUsageUserCount = computed(() => {
  return rankingTotalUsers.value || rankingItems.value.length
})

const topUser = computed<UserSpendingRankingItem | null>(() => rankingItems.value[0] || null)

const topUserDisplayName = computed(() => {
  return topUser.value ? userDisplayName(topUser.value) : '-'
})

const userTokenShare = (tokens: number): number => {
  if (rankingTotalTokens.value <= 0) return 0
  return tokens / rankingTotalTokens.value
}

const userBarWidth = (tokens: number): number => {
  if (rankingTotalTokens.value <= 0) return 0
  return Math.max(4, Math.round(userTokenShare(tokens) * 100))
}

const topUserTokenShare = computed(() => {
  return topUser.value ? userTokenShare(topUser.value.tokens) : 0
})

const memberPulseItems = computed(() => rankingItems.value.slice(0, 5))

const topThreeTokenShare = computed(() => {
  if (rankingTotalTokens.value <= 0) return 0
  const topThreeTokens = rankingItems.value.slice(0, 3).reduce((sum, item) => sum + item.tokens, 0)
  return topThreeTokens / rankingTotalTokens.value
})

const avgTokensPerRequest = computed(() => {
  if (rankingTotalRequests.value <= 0) return 0
  return Math.round(rankingTotalTokens.value / rankingTotalRequests.value)
})

const avgTokensPerMember = computed(() => {
  if (activeUsageUserCount.value <= 0) return 0
  return Math.round(rankingTotalTokens.value / activeUsageUserCount.value)
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
  if (topUserTokenShare.value >= 0.6) return 'amber'
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

const otherUserContribution = computed<UserContributionItem | null>(() => {
  const otherTokens = Math.max(rankingTotalTokens.value - rankedUserTokens.value, 0)
  const otherRequests = Math.max(
    rankingTotalRequests.value - rankingItems.value.reduce((sum, item) => sum + item.requests, 0),
    0
  )
  const otherActualCost = Math.max(
    rankingTotalActualCost.value - rankingItems.value.reduce((sum, item) => sum + item.actual_cost, 0),
    0
  )
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
    const [snapshotResult, insightsResult, activityResult] = await Promise.allSettled([
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
</style>
