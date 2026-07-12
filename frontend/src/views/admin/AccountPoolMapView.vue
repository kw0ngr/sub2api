<template>
  <AppLayout>
    <div class="account-map-page account-map-console pb-12">
      <section class="account-map-toolbar">
        <div class="account-map-toolbar-copy">
          <div class="min-w-0">
            <h1>
              <svg class="h-5 w-5 text-sky-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M4 7a2 2 0 012-2h3a2 2 0 012 2v3a2 2 0 01-2 2H6a2 2 0 01-2-2V7zm9 0a2 2 0 012-2h3a2 2 0 012 2v3a2 2 0 01-2 2h-3a2 2 0 01-2-2V7zM4 16a2 2 0 012-2h3a2 2 0 012 2v1a2 2 0 01-2 2H6a2 2 0 01-2-2v-1zm9 0a2 2 0 012-2h3a2 2 0 012 2v1a2 2 0 01-2 2h-3a2 2 0 01-2-2v-1z"
                />
              </svg>
              账号池地图 / Account Pool Map
            </h1>
            <div class="account-map-meta">
              <span class="account-map-live-state">
                <span class="relative inline-flex h-2 w-2 rounded-full" :class="loading ? 'bg-gray-400' : 'bg-green-500'"></span>
                {{ loading ? '加载中' : '就绪' }}
              </span>
              <span>按平台、状态和账号池查看调度健康度</span>
              <template v-if="generatedAt">
                <span>Updated {{ formatDate(generatedAt) }}</span>
              </template>
            </div>
          </div>
          <div class="account-map-toolbar-actions">
            <button
              type="button"
              class="account-map-secondary-button"
              :disabled="loading"
              @click="refresh"
            >
              <svg aria-hidden="true" focusable="false" width="15" height="15" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
                <path d="M17 3v5h-5" />
                <path d="M3 17v-5h5" />
                <path d="M16.1 8A6.5 6.5 0 0 0 5 5.4L3 7.2" />
                <path d="M3.9 12A6.5 6.5 0 0 0 15 14.6l2-1.8" />
              </svg>
              {{ loading ? '刷新中' : '刷新' }}
            </button>
            <label class="account-map-auto-toggle">
              <span>Auto refresh</span>
              <input v-model="autoRefreshEnabled" type="checkbox" />
              <i aria-hidden="true"></i>
            </label>
            <span class="account-map-toolbar-divider" aria-hidden="true"></span>
            <span class="account-map-updated">
              Updated {{ generatedAt ? formatDate(generatedAt) : '未同步' }}
            </span>
            <label class="account-map-search">
              <svg aria-hidden="true" focusable="false" width="16" height="16" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round">
                <circle cx="9" cy="9" r="5.5" />
                <path d="m13.2 13.2 3.3 3.3" />
              </svg>
              <input
                v-model.trim="filters.search"
                type="search"
                placeholder="Search account / model / error"
              />
            </label>
          </div>
        </div>
      </section>

      <section class="account-map-summary-grid">
        <div
          v-for="item in summaryCards"
          :key="item.key"
          class="account-map-stat-card"
          :class="`account-map-stat-${item.tone}`"
        >
          <div class="account-map-stat-head">
            <span class="account-map-stat-icon" :class="item.dotClass">
              <svg aria-hidden="true" focusable="false" width="18" height="18" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
                <path :d="item.iconPath" />
              </svg>
            </span>
            <p>{{ item.label }}</p>
            <span class="account-map-stat-info">i</span>
          </div>
          <strong>{{ item.value }}</strong>
          <small>{{ item.detail }}</small>
          <div class="account-map-stat-spark" aria-hidden="true"></div>
          <div v-if="item.metrics?.length" class="account-map-stat-metrics">
            <span v-for="metric in item.metrics" :key="metric.label">
              <b>{{ metric.value }}</b>
              {{ metric.label }}
            </span>
          </div>
        </div>
      </section>

      <section
        v-if="loading || errorMessage || usingFallback || sourceInfo?.truncated || (!loading && !accounts.length)"
        class="account-map-state-strip"
        aria-live="polite"
      >
        <div v-if="loading" class="account-map-state-pill account-map-state-loading">
          <span class="account-map-state-pulse"></span>
          正在同步账号池地图数据
        </div>
        <div v-if="errorMessage" class="account-map-state-pill account-map-state-error">
          <span class="account-map-status-dot"></span>
          <span>{{ errorMessage }}</span>
          <button type="button" :disabled="loading" @click="refresh">重试</button>
        </div>
        <div v-if="usingFallback" class="account-map-state-pill account-map-state-fallback">
          <span class="account-map-line-swatch"></span>
          使用账号列表聚合数据；账号池地图 API 恢复后会自动回到服务端聚合结果。
        </div>
        <div v-if="sourceInfo?.truncated" class="account-map-state-pill account-map-state-info">
          当前展示 {{ sourceInfo.returned }} / {{ sourceInfo.total }} 个账号，建议用搜索或平台筛选缩小范围。
        </div>
        <div v-if="!loading && !accounts.length" class="account-map-state-pill account-map-state-info">
          暂无可展示账号；创建或导入上游账号后，地图会自动生成泳道。
        </div>
      </section>

      <section class="account-map-workspace">
        <aside class="account-map-left-rail" aria-label="账号池筛选">
          <div class="account-map-filter-card">
            <div class="account-map-filter-header">
              <div>
                <h2>平台筛选</h2>
              </div>
            </div>
            <div class="account-map-chip-grid">
              <button
                type="button"
                class="account-map-filter-chip"
                :class="{ active: filters.platform === 'all' }"
                @click="filters.platform = 'all'"
              >
                <span class="account-map-platform-mark">ALL</span>
                全部平台
              </button>
              <button
                v-for="platform in platformOptions"
                :key="platform"
                type="button"
                class="account-map-filter-chip"
                :class="{ active: filters.platform === platform }"
                @click="filters.platform = platform"
              >
                <span class="account-map-platform-mark">{{ platformInitial(platform) }}</span>
                {{ platformLabel(platform) }}
              </button>
            </div>

            <div class="account-map-filter-header account-map-filter-header-spaced">
              <div>
                <h2>状态筛选</h2>
              </div>
            </div>
            <div class="account-map-chip-grid account-map-status-chip-grid">
              <button
                v-for="status in statusFilterOptions"
                :key="status.value"
                type="button"
                class="account-map-filter-chip account-map-status-filter-chip"
                :class="[
                  `account-map-status-filter-${status.tone}`,
                  filters.status === status.value ? 'active' : ''
                ]"
                @click="filters.status = status.value"
              >
                <span class="account-map-status-dot"></span>
                {{ status.label }}
              </button>
            </div>

            <div
              v-if="errorMessage"
              class="account-map-rail-notice account-map-rail-notice-warn"
            >
              <span>{{ errorMessage }}</span>
              <button type="button" @click="refresh">重试</button>
            </div>
            <div
              v-if="sourceInfo?.truncated"
              class="account-map-rail-notice account-map-rail-notice-info"
            >
              当前仅展示前 {{ sourceInfo.returned }} / {{ sourceInfo.total }} 个账号；可通过搜索或平台筛选缩小范围。
            </div>
            <div
              v-if="usingFallback"
              class="account-map-rail-notice account-map-rail-notice-info"
            >
              当前使用账号列表聚合数据；账号池地图 API 恢复后会自动回到服务端聚合结果。
            </div>
          </div>

          <div class="account-map-group-panel">
            <div class="account-map-group-panel-head">
              <h2>分组列表</h2>
              <button type="button" aria-label="刷新分组列表" @click="refresh">
                <svg aria-hidden="true" focusable="false" width="15" height="15" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
                  <path d="M17 3v5h-5" />
                  <path d="M3 17v-5h5" />
                  <path d="M16.1 8A6.5 6.5 0 0 0 5 5.4L3 7.2" />
                  <path d="M3.9 12A6.5 6.5 0 0 0 15 14.6l2-1.8" />
                </svg>
              </button>
            </div>
            <div class="account-map-group-table-head">
              <span>分组名称</span>
              <span>平台</span>
              <span>可用 / 总数</span>
              <span>风险</span>
              <span>健康度</span>
            </div>
            <div class="account-map-group-list">
              <button
                v-for="pool in visiblePools"
                :key="pool.key"
                type="button"
                class="account-map-group-row"
                :class="[
                  `account-map-group-row-${poolStatusTone(pool)}`,
                  selectedPoolForDetail?.key === pool.key ? 'active' : ''
                ]"
                :title="poolDisplayName(pool)"
                @click="selectPool(pool)"
              >
                <span class="account-map-group-name">{{ poolDisplayName(pool) }}</span>
                <span class="account-map-group-platform">
                  <span class="account-map-platform-mark">{{ platformInitial(pool.platform) }}</span>
                </span>
                <span class="account-map-group-count">{{ poolHealthyCount(pool) }} / {{ pool.accounts.length }}</span>
                <span class="account-map-risk-chip" :class="`account-map-risk-${poolStatusTone(pool)}`">
                  {{ poolRiskLabel(pool) }}
                </span>
                <span class="account-map-health-cell" :title="`${poolHealthPercent(pool)}%`">
                  <span class="account-map-health-track">
                    <span :style="{ width: `${poolHealthPercent(pool)}%` }"></span>
                  </span>
                  <small>{{ poolHealthPercent(pool) }}%</small>
                </span>
              </button>
            </div>
          </div>

        </aside>

        <div class="account-map-main-column">
          <section class="account-map-table-panel" aria-label="账号池总览">
            <div v-if="loading && !accounts.length" class="account-map-loading-shell">
              <div v-for="idx in 3" :key="idx" class="account-map-skeleton-card"></div>
            </div>

            <div v-else-if="!visiblePools.length" class="account-map-empty-state">
              <p class="text-base font-semibold text-gray-900 dark:text-white">没有匹配的账号池</p>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">调整筛选条件，或者先在账号管理中导入/创建上游账号。</p>
            </div>

            <div v-else class="account-map-table-card">
              <div class="account-map-table-title">
                <h2>账号池总览</h2>
                <span>{{ visiblePools.length }} groups · {{ accounts.length }} accounts</span>
              </div>
              <div class="account-map-table-scroll">
                <table class="account-map-data-table account-map-pool-table">
                  <thead>
                    <tr>
                      <th>Group</th>
                      <th>Platform</th>
                      <th>Usable</th>
                      <th>Cooldown</th>
                      <th>Error</th>
                      <th>Quota</th>
                      <th>Latency</th>
                      <th>Models</th>
                      <th>Status</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr
                      v-for="pool in visiblePools"
                      :key="pool.key"
                      :class="{ active: selectedPoolForDetail?.key === pool.key }"
                      tabindex="0"
                      @click="selectPool(pool)"
                      @keydown.enter.prevent="selectPool(pool)"
                      @keydown.space.prevent="selectPool(pool)"
                    >
                      <td>
                        <button type="button" class="account-map-table-primary" :title="poolDisplayName(pool)" @click.stop="selectPool(pool)">
                          {{ platformLabel(pool.platform) }} {{ accountTypeLabel(pool.type) }}
                        </button>
                      </td>
                      <td>
                        <span class="account-map-platform-pill" :title="platformLabel(pool.platform)">{{ platformLabel(pool.platform) }}</span>
                      </td>
                      <td>{{ poolHealthyCount(pool) }} / {{ pool.accounts.length }}</td>
                      <td class="account-map-number-warn">{{ poolCooldownAccounts(pool).length }}</td>
                      <td :class="pool.summary?.error ? 'account-map-number-danger' : 'account-map-number-good'">
                        {{ pool.summary?.error || 0 }}
                      </td>
                      <td>
                        <span class="account-map-quota-cell">
                          <span>{{ quotaPercentText(poolQuotaPercent(pool)) }}</span>
                          <span class="account-map-mini-bar" :class="quotaBarClass(poolQuotaPercent(pool))">
                            <i :style="{ width: quotaBarWidth(poolQuotaPercent(pool)) }"></i>
                          </span>
                        </span>
                      </td>
                      <td :class="poolLatencyClass(pool)">{{ poolLatencyText(pool) }}</td>
                      <td>{{ poolModelCount(pool) }}</td>
                      <td>
                        <span class="account-map-status-label" :class="`account-map-status-label-${poolStatusTone(pool)}`">
                          {{ poolStatusChipText(pool) }}
                        </span>
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>
          </section>

          <section class="account-map-table-panel account-map-account-detail-panel" aria-label="选中池账号明细">
            <div v-if="loading && !accounts.length" class="account-map-loading-shell account-map-loading-shell-compact">
              <div v-for="idx in 2" :key="idx" class="account-map-skeleton-card"></div>
            </div>

            <div v-else-if="!activeDetailPool" class="account-map-empty-state account-map-empty-state-compact">
              <p class="text-base font-semibold text-gray-900 dark:text-white">选择一个账号池查看账号明细</p>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">点击左侧分组或上方总览表格中的行后，这里会展示池内账号。</p>
            </div>

            <div v-else class="account-map-table-card">
              <div class="account-map-table-title">
                <h2 :title="`${poolDisplayName(activeDetailPool)} · 账号明细`">{{ poolDisplayName(activeDetailPool) }} · 账号明细</h2>
                <span>{{ poolHealthyCount(activeDetailPool) }} / {{ activeDetailPool.accounts.length }} usable</span>
              </div>
              <div class="account-map-table-scroll">
                <table class="account-map-data-table account-map-account-table">
                  <thead>
                    <tr>
                      <th>Account</th>
                      <th>Status</th>
                      <th>Models</th>
                      <th>RPM</th>
                      <th>Tokens</th>
                      <th>Quota</th>
                      <th>Last error</th>
                      <th>Cooldown until</th>
                      <th>Action</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr
                      v-for="account in visiblePoolAccounts(activeDetailPool)"
                      :key="account.id"
                      :class="{ active: selectedAccount?.id === account.id }"
                      tabindex="0"
                      @click="selectAccount(account)"
                      @keydown.enter.prevent="selectAccount(account)"
                      @keydown.space.prevent="selectAccount(account)"
                    >
                      <td>
                        <button type="button" class="account-map-table-primary" :title="account.name" @click.stop="selectAccount(account)">
                          {{ account.name }}
                        </button>
                      </td>
                      <td>
                        <span class="account-map-account-status" :class="nodeClass(account)">
                          <i></i>{{ statusLabel(account) }}
                        </span>
                      </td>
                      <td>{{ accountModelNames(account).length || '-' }}</td>
                      <td>{{ account.current_rpm ?? 0 }}</td>
                      <td :title="accountTokenMeta(account) || '-'">{{ accountTokenMeta(account) || '-' }}</td>
                      <td>
                        <span class="account-map-quota-cell">
                          <span>{{ quotaPercentText(accountQuotaPercent(account)) }}</span>
                          <span class="account-map-mini-bar" :class="quotaBarClass(accountQuotaPercent(account))">
                            <i :style="{ width: quotaBarWidth(accountQuotaPercent(account)) }"></i>
                          </span>
                        </span>
                      </td>
                      <td :class="statusKind(account) === 'error' ? 'account-map-number-danger' : ''" :title="accountLastError(account)">
                        {{ accountLastError(account) }}
                      </td>
                      <td :title="accountCooldownUntilText(account)">{{ accountCooldownUntilText(account) }}</td>
                      <td>
                        <button type="button" class="account-map-row-action" @click.stop="selectAccount(account)">
                          查看
                        </button>
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
              <div v-if="poolHiddenCount(activeDetailPool) > 0" class="account-map-table-footnote">
                <span class="account-map-status-info-dot">i</span>
                当前显示前 {{ visiblePoolAccounts(activeDetailPool).length }} 个账号；可展开查看剩余 {{ poolHiddenCount(activeDetailPool) }} 个。
                <button type="button" @click="togglePoolExpanded(activeDetailPool)">展开全部</button>
              </div>
              <div v-else class="account-map-table-footnote">
                <span class="account-map-status-info-dot">i</span>
                已显示全部账号
              </div>
            </div>
          </section>
        </div>
        <aside class="account-map-right-rail" aria-label="账号池详情检查器">
          <section class="account-map-inspector" aria-live="polite">
          <template v-if="selectedAccount">
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
              <p class="mt-1 line-clamp-4 text-xs" :title="selectedAccount.temp_unschedulable_reason || selectedAccount.error_message || ''">{{ selectedAccount.temp_unschedulable_reason || selectedAccount.error_message }}</p>
            </div>

            <div class="inspector-actions">
              <button type="button" class="inspector-action-primary" @click="goAccountDetail(selectedAccount)">
                打开账号管理
              </button>
              <button
                v-if="isAPIKeyHealthSupportedAccount(selectedAccount)"
                type="button"
                class="inspector-action"
                :disabled="checkingHealth"
                @click="runHealthCheck([selectedAccount.id])"
              >
                {{ checkingHealth ? '检测中' : '检测此 Key' }}
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
            <div class="inspector-drawer-header">
              <div class="inspector-drawer-header-left">
                <span class="inspector-kicker">Pool inspector</span>
                <h2 class="inspector-title">{{ poolDisplayName(selectedPoolForDetail) }}</h2>
                <p class="inspector-subtitle">
                  <span class="account-map-node-status-dot" :class="`inspector-dot-${poolStatusTone(selectedPoolForDetail)}`"></span>
                  {{ poolStatusText(selectedPoolForDetail) }}
                </p>
              </div>
              <div class="inspector-selection-actions">
                <span class="inspector-selected-badge">已选中</span>
                <button type="button" class="inspector-close" aria-label="取消选择账号池" @click="clearPoolSelection">
                  <svg aria-hidden="true" focusable="false" width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
                    <path d="M1 1l12 12M13 1L1 13"/>
                  </svg>
                </button>
              </div>
            </div>

            <div class="inspector-diagnosis-card" :class="`inspector-diagnosis-${poolStatusTone(selectedPoolForDetail)}`">
              <div class="inspector-diagnosis-head">
                <span class="account-map-status-label" :class="`account-map-status-label-${poolStatusTone(selectedPoolForDetail)}`">
                  {{ poolStatusChipText(selectedPoolForDetail) }}
                </span>
                <strong>{{ poolHealthyCount(selectedPoolForDetail) }} / {{ selectedPoolForDetail.accounts.length }} usable</strong>
              </div>
              <h3>当前判断</h3>
              <p>{{ poolDiagnosisText(selectedPoolForDetail) }}</p>
            </div>

            <div class="inspector-action-grid">
              <button type="button" class="inspector-action" @click="runHealthCheck()">
                {{ checkingHealth ? '重试健康检查中' : '重试健康检查' }}
              </button>
              <button type="button" class="inspector-action" @click="goAccounts">
                打开账号管理
              </button>
              <button type="button" class="inspector-action" @click="goPoolUsage(selectedPoolForDetail)">
                查看请求记录
              </button>
              <button type="button" class="inspector-action" @click="goFingerprints">
                查看指纹策略
              </button>
            </div>

            <div class="inspector-section">
              <div class="inspector-section-title">
                <div>
                  <h3>可用模型</h3>
                  <p>来自账号模型限流配置与最近额度探测模型</p>
                </div>
              </div>
              <div v-if="poolModelRows(selectedPoolForDetail).length" class="inspector-model-chips">
                <span v-for="model in poolModelRows(selectedPoolForDetail)" :key="model.name">
                  {{ model.name }}
                </span>
              </div>
              <p v-else class="inspector-empty-line">暂无模型映射</p>
            </div>

            <div class="inspector-section">
              <div class="inspector-section-title">
                <div>
                  <h3>不可用原因</h3>
                  <p>按当前账号状态与配额信号聚合</p>
                </div>
                <span>{{ poolUnavailableReasons(selectedPoolForDetail).length }} 类</span>
              </div>
              <div v-if="poolUnavailableReasons(selectedPoolForDetail).length" class="inspector-reason-table">
                <div
                  v-for="item in poolUnavailableReasons(selectedPoolForDetail)"
                  :key="item.reason"
                  class="inspector-reason-row"
                  :class="`inspector-reason-${item.tone}`"
                >
                  <span>{{ item.reason }}</span>
                  <strong>{{ item.count }}</strong>
                </div>
              </div>
              <p v-else class="inspector-empty-line">当前未发现不可用账号</p>
            </div>

            <div class="inspector-section inspector-policy-section">
              <div class="inspector-section-title">
                <div>
                  <h3>调度策略</h3>
                  <p>{{ selectedPoolForDetail.platform }} · {{ selectedPoolForDetail.type }}</p>
                </div>
              </div>
              <InspectorRow
                v-for="row in poolPolicyRows(selectedPoolForDetail)"
                :key="row.label"
                :label="row.label"
                :value="row.value"
              />
            </div>

            <div class="inspector-section">
              <div class="inspector-section-title">
                <div>
                  <h3>最近错误</h3>
                  <p>基于账号状态、错误消息和调度暂停原因</p>
                </div>
                <span>{{ poolRecentErrors(selectedPoolForDetail).length }} 条</span>
              </div>
              <div v-if="poolRecentErrors(selectedPoolForDetail).length" class="inspector-event-list">
                <button
                  v-for="item in poolRecentErrors(selectedPoolForDetail)"
                  :key="item.account.id"
                  type="button"
                  class="inspector-event-row"
                  :class="nodeClass(item.account)"
                  @click="selectAccount(item.account)"
                >
                  <span class="inspector-account-dot"></span>
                  <span>
                    <b>{{ item.account.name }}</b>
                    <small>{{ item.reason }}</small>
                  </span>
                  <em>{{ statusLabel(item.account) }}</em>
                </button>
              </div>
              <p v-else class="inspector-empty-line">暂无最近错误</p>
            </div>

            <div class="inspector-section">
              <div class="inspector-section-title">
                <div>
                  <h3>最近 10 次请求</h3>
                  <p>无请求日志字段时展示当前 RPM/并发/会话状态</p>
                </div>
                <span>{{ poolRequestSignals(selectedPoolForDetail).length }} 条</span>
              </div>
              <div v-if="poolRequestSignals(selectedPoolForDetail).length" class="inspector-request-list">
                <button
                  v-for="item in poolRequestSignals(selectedPoolForDetail)"
                  :key="item.account.id"
                  type="button"
                  class="inspector-request-row"
                  :class="nodeClass(item.account)"
                  @click="selectAccount(item.account)"
                >
                  <span>{{ item.account.name }}</span>
                  <span>{{ statusLabel(item.account) }}</span>
                  <span>{{ accountModelMeta(item.account) }}</span>
                  <span>{{ item.signal }}</span>
                </button>
              </div>
              <p v-else class="inspector-empty-line">暂无最近请求信号</p>
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
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { adminAPI } from '@/api/admin'
import AppLayout from '@/components/layout/AppLayout.vue'
import { useAccountPoolMapData } from '@/composables/useAccountPoolMapData'
import type { Account, AccountPlatform } from '@/types'
import type {
  AccountPoolMapAccount,
  AccountPoolMapPool,
  AccountPoolMapStatusKind,
  AccountPoolMapSummary,
  APIKeyProbeQuotaSnapshot
} from '@/api/admin/accounts'

type AccountStatusKind = AccountPoolMapStatusKind
type AccountPool = AccountPoolMapPool
type DetailRow = { label: string; value: string }
type QuotaStatItem = { label: string; value: string; hint: string }
type StatusFilterOption = {
  value: 'all' | AccountStatusKind
  label: string
  tone: 'all' | AccountStatusKind
}
type PlatformHealthRow = {
  platform: string
  total: number
  healthy: number
  attention: number
  quota: number
  healthyPercent: number
}
type PoolModelRow = { name: string; supported: number; total: number }
type PoolIssueRow = { account: AccountPoolMapAccount; reason: string }
type PoolCooldownRow = { account: AccountPoolMapAccount; resetText: string; reason: string }
type PoolRequestSignal = { account: AccountPoolMapAccount; signal: string }
type PoolUnavailableReason = { reason: string; count: number; tone: AccountStatusKind }

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
const autoRefreshEnabled = ref(true)
let refreshTimer: ReturnType<typeof setTimeout> | null = null
let healthPollTimer: ReturnType<typeof setTimeout> | null = null
let autoRefreshTimer: ReturnType<typeof setInterval> | null = null
const POOL_PREVIEW_LIMIT = 12
const AUTO_REFRESH_INTERVAL_MS = 60_000
const API_KEY_HEALTH_PLATFORMS = new Set<AccountPlatform>([
  'anthropic',
  'openai',
  'gemini',
  'openrouter',
  'deepseek',
  'glm',
  'grok'
])

const filters = reactive<{
  search: string
  platform: string
  status: 'all' | AccountStatusKind
}>({
  search: '',
  platform: 'all',
  status: 'all'
})

const statusFilterOptions: StatusFilterOption[] = [
  { value: 'all', label: '全部', tone: 'all' },
  { value: 'healthy', label: 'Healthy', tone: 'healthy' },
  { value: 'degraded', label: 'Cooldown', tone: 'degraded' },
  { value: 'error', label: 'Error', tone: 'error' },
  { value: 'disabled', label: 'Disabled', tone: 'disabled' },
  { value: 'rate_limited', label: 'Quota Risk', tone: 'rate_limited' }
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

const activeDetailPool = computed(() => selectedPoolForDetail.value)

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

const summaryCards = computed(() => {
  const cooldown = (summary.value.degraded || 0) + (summary.value.rate_limited || 0)
  const riskyPools = visiblePools.value.filter((pool) => poolStatusTone(pool) !== 'healthy').length
  return [
    {
      key: 'healthy',
      label: '可用账号',
      value: summary.value.healthy,
      detail: summary.value.total ? `${Math.round((summary.value.healthy / summary.value.total) * 100)}% 可调度` : '无账号',
      dotClass: 'account-map-stat-icon-good',
      tone: 'good',
      iconPath: 'M6.5 10.2 8.8 12.5 13.8 7.5 M10 2.5a7.5 7.5 0 1 0 0 15 7.5 7.5 0 0 0 0-15',
      metrics: [
        { label: '总数', value: String(summary.value.total || 0) },
        { label: 'TLS', value: `${summary.value.tls_enabled}/${summary.value.total || 0}` }
      ]
    },
    {
      key: 'cooldown',
      label: '临时冷却',
      value: cooldown,
      detail: `${summary.value.degraded || 0} 降级 · ${summary.value.rate_limited || 0} 限流`,
      dotClass: cooldown > 0 ? 'account-map-stat-icon-warn' : 'account-map-stat-icon-good',
      tone: cooldown > 0 ? 'warn' : 'good',
      iconPath: 'M10 3v7l3.8 2.1 M10 17.5a7.5 7.5 0 1 0 0-15 7.5 7.5 0 0 0 0 15',
      metrics: [
        { label: 'RPM', value: String(summary.value.rpm || 0) },
        { label: '并发', value: String(summary.value.concurrency || 0) }
      ]
    },
    {
      key: 'error',
      label: '异常账号',
      value: summary.value.error || 0,
      detail: `${summary.value.attention || 0} 个账号需要关注`,
      dotClass: summary.value.error > 0 ? 'account-map-stat-icon-danger' : 'account-map-stat-icon-good',
      tone: summary.value.error > 0 ? 'danger' : 'good',
      iconPath: 'M10 5.2v5.2 M10 13.8h.01 M8.8 2.8 2.3 16.4h13.8L11.2 2.8a1.4 1.4 0 0 0-2.4 0',
      metrics: [
        { label: '停用', value: String(summary.value.disabled || 0) },
        { label: '限流', value: String(summary.value.rate_limited || 0) }
      ]
    },
    {
      key: 'risk',
      label: '高风险分组',
      value: riskyPools,
      detail: `${visiblePools.value.length} 个池/泳道已载入`,
      dotClass: riskyPools > 0 ? 'account-map-stat-icon-orange' : 'account-map-stat-icon-good',
      tone: riskyPools > 0 ? 'orange' : 'good',
      iconPath: 'M10 3.2 15.8 5.5v4.3c0 3.8-2.4 6.2-5.8 7-3.4-.8-5.8-3.2-5.8-7V5.5L10 3.2Z M10 7v4',
      metrics: [
        { label: '平台', value: String(platformHealthRows.value.length) },
        { label: '额度', value: String(summary.value.quota_signals || 0) }
      ]
    },
    {
      key: 'sessions',
      label: '活动会话',
      value: summary.value.active_sessions || 0,
      detail: `${summary.value.rpm || 0} RPM · ${summary.value.concurrency || 0} 并发`,
      dotClass: 'account-map-stat-icon-accent',
      tone: 'accent',
      iconPath: 'M3.5 10a6.5 6.5 0 0 1 13 0 M6.2 10a3.8 3.8 0 0 1 7.6 0 M10 10v4 M5 14h10',
      metrics: [
        { label: '池', value: String(summary.value.pools || visiblePools.value.length) },
        { label: '回退', value: usingFallback.value ? '是' : '否' }
      ]
    }
  ]
})

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
  const classes: Record<AccountStatusKind, string> = {
    healthy: 'account-map-account-status-healthy',
    degraded: 'account-map-account-status-degraded',
    rate_limited: 'account-map-account-status-rate_limited',
    error: 'account-map-account-status-error',
    disabled: 'account-map-account-status-disabled'
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

function platformInitial(platform: string): string {
  const label = platformLabel(platform)
  if (!label || label === 'Unknown') return 'UN'
  return label
    .split(/[\s_-]+/)
    .map((part) => part.slice(0, 1))
    .join('')
    .slice(0, 3)
    .toUpperCase()
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

function poolRiskLabel(pool: AccountPool): string {
  const tone = poolStatusTone(pool)
  if (tone === 'healthy') return '低'
  if (tone === 'degraded' || tone === 'rate_limited') return '中'
  return '高'
}

function poolHealthPercent(pool: AccountPool): number {
  if (!pool.accounts.length) return 0
  return Math.round((poolHealthyCount(pool) / pool.accounts.length) * 100)
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

function poolDisplayName(pool: AccountPool): string {
  return `${platformLabel(pool.platform)} ${accountTypeLabel(pool.type)}`
}

function poolStatusText(pool: AccountPool): string {
  const tone = poolStatusTone(pool)
  const reason = poolPrimaryReason(pool)
  return `${statusKindLabel(tone)} · ${reason}`
}

function poolStatusChipText(pool: AccountPool): string {
  const tone = poolStatusTone(pool)
  if (tone === 'healthy') return 'Healthy'
  if (tone === 'degraded') return 'Cooldown'
  if (tone === 'rate_limited') return 'Quota Risk'
  if (tone === 'error') return 'Error'
  return 'Disabled'
}

function poolDiagnosisText(pool: AccountPool): string {
  const unavailable = pool.accounts.length - poolHealthyCount(pool)
  if (!unavailable) return '当前账号池可参与调度，未发现错误、冷却或额度风险。'
  const cooldown = poolCooldownAccounts(pool).length
  const error = pool.summary?.error || 0
  const quota = pool.summary?.rate_limited || poolQuotaSignalCount(pool)
  const parts = [`${unavailable} 个账号不可用或需关注`]
  if (cooldown) parts.push(`${cooldown} 个账号处于冷却窗口`)
  if (error) parts.push(`${error} 个账号存在错误`)
  if (quota) parts.push(`${quota} 个账号存在额度或限流信号`)
  return `${parts.join('，')}。`
}

function poolPrimaryReason(pool: AccountPool): string {
  const issue = pool.accounts.find((account) => statusKind(account) !== 'healthy')
  if (!issue) return '当前账号池可参与调度'
  return accountIssueReason(issue)
}

function accountIssueReason(account: AccountPoolMapAccount): string {
  return truncateText(
    account.status_reason ||
      account.temp_unschedulable_reason ||
      account.error_message ||
      quotaBrief(account) ||
      statusKindLabel(statusKind(account)),
    44
  )
}

function poolPolicyRows(pool: AccountPool): DetailRow[] {
  const policy = pool.accounts.some((account) => account.priority !== 0) ? '按优先级与健康度调度' : '按健康度与可用负载调度'
  const distribution = poolRPM(pool) > 0 || poolConcurrency(pool) > 0 ? `RPM ${poolRPM(pool)} · 并发 ${poolConcurrency(pool)}` : '暂无实时负载'
  const failThreshold = `${pool.summary?.error || 0} 错误 · ${pool.summary?.rate_limited || 0} 限流`
  const cooldown = poolCooldownAccounts(pool)[0]?.resetText || '暂无冷却窗口'
  return [
    { label: '策略类型', value: policy },
    { label: '负载分配', value: distribution },
    { label: '失败阈值', value: failThreshold },
    { label: '冷却时间', value: cooldown },
    { label: '健康检查', value: poolQuotaSignalCount(pool) ? `${poolQuotaSignalCount(pool)} 个额度信号` : '等待额度探测' },
    { label: 'TLS 覆盖', value: poolFingerprintCount(pool) }
  ]
}

function poolModelRows(pool: AccountPool): PoolModelRow[] {
  const modelMap = new Map<string, Set<number>>()
  for (const account of pool.accounts) {
    for (const model of accountModelNames(account)) {
      const accountSet = modelMap.get(model) || new Set<number>()
      accountSet.add(account.id)
      modelMap.set(model, accountSet)
    }
  }
  return Array.from(modelMap.entries())
    .map(([name, accountIds]) => ({
      name,
      supported: accountIds.size,
      total: pool.accounts.length
    }))
    .sort((a, b) => b.supported - a.supported || a.name.localeCompare(b.name))
    .slice(0, 10)
}

function poolRecentErrors(pool: AccountPool): PoolIssueRow[] {
  return pool.accounts
    .filter((account) => statusKind(account) === 'error' || Boolean(account.error_message))
    .map((account) => ({ account, reason: accountIssueReason(account) }))
    .slice(0, 6)
}

function poolCooldownAccounts(pool: AccountPool): PoolCooldownRow[] {
  return pool.accounts
    .filter((account) => {
      const kind = statusKind(account)
      return kind === 'degraded' || kind === 'rate_limited' || isFuture(account.rate_limit_reset_at) || isFuture(account.overload_until) || isFuture(account.temp_unschedulable_until)
    })
    .map((account) => ({
      account,
      resetText: accountCooldownText(account),
      reason: accountIssueReason(account)
    }))
    .slice(0, 6)
}

function poolUnavailableReasons(pool: AccountPool): PoolUnavailableReason[] {
  const reasonMap = new Map<string, PoolUnavailableReason>()
  for (const account of pool.accounts) {
    const kind = statusKind(account)
    if (kind === 'healthy') continue
    const reason = accountIssueReason(account)
    const existing = reasonMap.get(reason) || { reason, count: 0, tone: kind }
    existing.count += 1
    reasonMap.set(reason, existing)
  }
  return Array.from(reasonMap.values()).sort((a, b) => b.count - a.count || a.reason.localeCompare(b.reason)).slice(0, 6)
}

function accountCooldownText(account: AccountPoolMapAccount): string {
  const resetAt = account.rate_limit_reset_at || account.overload_until || account.temp_unschedulable_until
  if (resetAt) return formatDate(resetAt)
  const quotaReset = quotaResetText(quotaSnapshot(account))
  return quotaReset === '-' ? '未记录' : quotaReset
}

function accountCooldownUntilText(account: AccountPoolMapAccount): string {
  const text = accountCooldownText(account)
  return text === '未记录' ? '-' : text
}

function poolRequestSignals(pool: AccountPool): PoolRequestSignal[] {
  return pool.accounts
    .map((account) => ({
      account,
      signal: accountRuntimeSignal(account)
    }))
    .filter((item) => item.signal !== 'idle')
    .sort((a, b) => accountLoadScore(b.account) - accountLoadScore(a.account) || statusWeight(a.account) - statusWeight(b.account))
    .slice(0, 10)
}

function accountRuntimeSignal(account: AccountPoolMapAccount): string {
  const parts: string[] = []
  if (account.current_rpm) parts.push(`${account.current_rpm} RPM`)
  if (account.current_concurrency) parts.push(`${account.current_concurrency} 并发`)
  if (account.active_sessions) parts.push(`${account.active_sessions} 会话`)
  if (statusKind(account) !== 'healthy') parts.push(accountIssueReason(account))
  return parts.length ? parts.join(' · ') : 'idle'
}

function accountLoadScore(account: AccountPoolMapAccount): number {
  return (account.current_rpm || 0) + (account.current_concurrency || 0) * 5 + (account.active_sessions || 0) * 2
}

function poolModelCount(pool: AccountPool): number {
  const models = new Set<string>()
  for (const account of pool.accounts) {
    for (const model of accountModelNames(account)) {
      models.add(model)
    }
  }
  return models.size
}

function accountModelNames(account: AccountPoolMapAccount): string[] {
  const models = new Set<string>()
  const modelLimits = account.extra?.model_rate_limits
  if (modelLimits && typeof modelLimits === 'object') {
    for (const model of Object.keys(modelLimits)) models.add(model)
  }
  const snapshotModel = quotaSnapshot(account)?.model
  if (snapshotModel) models.add(snapshotModel)
  return Array.from(models)
}

function accountModelMeta(account: AccountPoolMapAccount): string {
  const models = accountModelNames(account)
  if (models.length > 1) return `${models.length} models`
  if (models.length === 1) return models[0]
  return accountTypeLabel(account.type)
}

function accountTokenMeta(account: AccountPoolMapAccount): string {
  const snapshot = quotaSnapshot(account)
  if (!snapshot) return ''
  if (snapshot.tokens_remaining || snapshot.tokens_limit) {
    return `Tok ${formatQuotaPair(snapshot.tokens_remaining, snapshot.tokens_limit)}`
  }
  if (snapshot.input_tokens_remaining || snapshot.input_tokens_limit || snapshot.output_tokens_remaining || snapshot.output_tokens_limit) {
    return `Tok ${formatQuotaPair(snapshot.input_tokens_remaining, snapshot.input_tokens_limit)} / ${formatQuotaPair(snapshot.output_tokens_remaining, snapshot.output_tokens_limit)}`
  }
  if (snapshot.requests_remaining || snapshot.requests_limit) {
    return `Req ${formatQuotaPair(snapshot.requests_remaining, snapshot.requests_limit)}`
  }
  return ''
}

function truncateText(value: string, maxLength: number): string {
  if (value.length <= maxLength) return value
  return `${value.slice(0, maxLength - 1)}…`
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

function quotaIsGLMFiveHour(snapshot: APIKeyProbeQuotaSnapshot | null): boolean {
  if (!snapshot) return false
  return snapshot.provider === 'glm' && snapshot.source === 'glm_quota_api'
}

function quotaBrief(account: Account | AccountPoolMapAccount | null | undefined): string {
  const snapshot = quotaSnapshot(account)
  if (!snapshot) return ''
  if (quotaIsGLMFiveHour(snapshot)) {
    return `5h ${formatQuotaPair(snapshot.tokens_remaining, snapshot.tokens_limit)}`
  }
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

function quotaPercentText(value: number | null): string {
  return value === null ? '-' : `${value}%`
}

function quotaBarWidth(value: number | null): string {
  return `${Math.max(6, Math.min(100, value ?? 0))}%`
}

function quotaBarClass(value: number | null): string {
  if (value === null) return 'account-map-mini-bar-muted'
  if (value >= 85) return 'account-map-mini-bar-danger'
  if (value >= 70) return 'account-map-mini-bar-warn'
  return 'account-map-mini-bar-good'
}

function poolQuotaPercent(pool: AccountPool): number | null {
  const values = pool.accounts.map((account) => accountQuotaPercent(account)).filter((value): value is number => value !== null)
  if (!values.length) return null
  return Math.round(values.reduce((sum, value) => sum + value, 0) / values.length)
}

function accountQuotaPercent(account: AccountPoolMapAccount): number | null {
  const direct = quotaUsagePercent(account.quota_used, account.quota_limit)
  if (direct !== null) return direct
  const daily = quotaUsagePercent(account.quota_daily_used, account.quota_daily_limit)
  if (daily !== null) return daily
  const weekly = quotaUsagePercent(account.quota_weekly_used, account.quota_weekly_limit)
  if (weekly !== null) return weekly
  return quotaSnapshotPercent(quotaSnapshot(account))
}

function quotaUsagePercent(used?: number | null, limit?: number | null): number | null {
  if (!limit || limit <= 0 || used === null || used === undefined) return null
  return Math.round(Math.max(0, Math.min(100, (used / limit) * 100)))
}

function quotaSnapshotPercent(snapshot: APIKeyProbeQuotaSnapshot | null): number | null {
  if (!snapshot) return null
  const tokenPercent = remainingLimitPercent(snapshot.tokens_remaining, snapshot.tokens_limit)
  if (tokenPercent !== null) return tokenPercent
  const requestPercent = remainingLimitPercent(snapshot.requests_remaining, snapshot.requests_limit)
  if (requestPercent !== null) return requestPercent
  const inputPercent = remainingLimitPercent(snapshot.input_tokens_remaining, snapshot.input_tokens_limit)
  const outputPercent = remainingLimitPercent(snapshot.output_tokens_remaining, snapshot.output_tokens_limit)
  if (inputPercent !== null && outputPercent !== null) return Math.round((inputPercent + outputPercent) / 2)
  return inputPercent ?? outputPercent
}

function remainingLimitPercent(remaining?: string, limit?: string): number | null {
  const remainingValue = numericStringValue(remaining)
  const limitValue = numericStringValue(limit)
  if (remainingValue === null || limitValue === null || limitValue <= 0) return null
  return Math.round(Math.max(0, Math.min(100, ((limitValue - remainingValue) / limitValue) * 100)))
}

function numericStringValue(value?: string): number | null {
  if (!value) return null
  const next = Number(value.replace(/,/g, ''))
  return Number.isFinite(next) ? next : null
}

function accountLatencyMs(account: AccountPoolMapAccount): number | null {
  const extra = account.extra as Record<string, unknown> | undefined
  return numberValue(extra?.latency_ms) ?? numberValue(extra?.base_latency_ms) ?? numberValue(extra?.average_duration_ms) ?? null
}

function poolLatencyMs(pool: AccountPool): number | null {
  const values = pool.accounts.map(accountLatencyMs).filter((value): value is number => value !== null)
  if (!values.length) return null
  return Math.round(values.reduce((sum, value) => sum + value, 0) / values.length)
}

function poolLatencyText(pool: AccountPool): string {
  const latency = poolLatencyMs(pool)
  if (latency === null) return '-'
  if (latency >= 1000) return `${(latency / 1000).toFixed(2)}s`
  return `${latency}ms`
}

function poolLatencyClass(pool: AccountPool): string {
  const latency = poolLatencyMs(pool)
  if (latency === null) return ''
  if (latency >= 1000) return 'account-map-number-danger'
  if (latency >= 800) return 'account-map-number-warn'
  return 'account-map-number-good'
}

function accountLastError(account: AccountPoolMapAccount): string {
  if (statusKind(account) === 'healthy') return '-'
  return accountIssueReason(account)
}

function quotaSourceLabel(snapshot: APIKeyProbeQuotaSnapshot | null): string {
  if (!snapshot) return '未采集'
  if (snapshot.source === 'headers') return '响应头采集'
  if (snapshot.source === 'balance_api') return '余额接口'
  if (snapshot.source === 'missing_balance') return '余额未返回'
  if (snapshot.source === 'missing_headers') return '响应头未返回'
  if (snapshot.source === 'project_quota') return '项目级额度'
  if (snapshot.source === 'glm_quota_api') return 'GLM 用量接口'
  if (snapshot.source === 'missing_glm_quota') return 'GLM 用量未返回'
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
  if (quotaIsGLMFiveHour(snapshot)) return '已查询 5h 窗口'
  if (quotaHasBalanceSignal(snapshot)) return '已查询余额'
  if (quotaHasHeaderSignal(snapshot)) return '已捕获响应头'
  if (snapshot.provider === 'gemini') return '项目级限额'
  return '无实时额度头'
}

function quotaEmptyTitle(snapshot: APIKeyProbeQuotaSnapshot | null): string {
  if (!snapshot) return '等待健康检查'
  if (snapshot.provider === 'glm') return 'GLM 未返回可读 5h 用量'
  if (snapshot.provider === 'gemini') return 'Gemini 未返回每 Key 剩余额度'
  if (snapshot.source === 'missing_balance') return '余额接口未返回明细'
  return '上游未返回可读额度'
}

function quotaGuidance(snapshot: APIKeyProbeQuotaSnapshot | null): string {
  if (!snapshot) return '运行一次 API Key 健康检查后，这里会显示最近一次真实上游探测结果。'
  if (quotaIsGLMFiveHour(snapshot)) {
    return '已通过官方 GLM quota/limit 接口读取 5 小时 Token 窗口，可用于判断哪个 Key 接近上游重置窗口。'
  }
  if (snapshot.provider === 'glm') {
    return 'GLM 官方直连账号会尝试读取 quota/limit；中转站或未返回明细时仍保留健康检查结果。'
  }
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
  if (quotaIsGLMFiveHour(snapshot)) {
    const reset = quotaResetText(snapshot)
    return `${source} · 5 小时 Token 窗口${reset !== '-' ? ` · 重置 ${reset}` : ''}`
  }
  if (quotaHasBalanceSignal(snapshot)) return `${source} · ${scope}`
  if (quotaHasHeaderSignal(snapshot)) return `${source} · ${scope} · ${snapshot.model || '未记录模型'}`
  if (snapshot.provider === 'gemini') return `Gemini ${scope} 级探测 · 无每 Key 剩余量`
  return `${source} · ${snapshot.model || '未记录模型'}`
}

function quotaPanelClass(snapshot: APIKeyProbeQuotaSnapshot | null): string {
  if (!snapshot) return 'quota-panel-missing'
  if (quotaIsGLMFiveHour(snapshot)) return 'quota-panel-glm'
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
    const reset = snapshot.tokens_reset ? `重置 ${snapshot.tokens_reset}` : '等待上游重置'
    items.push({
      label: quotaIsGLMFiveHour(snapshot) ? '5h Token' : 'Token',
      value: formatQuotaPair(snapshot.tokens_remaining, snapshot.tokens_limit),
      hint: quotaIsGLMFiveHour(snapshot) ? `${snapshot.rate_limit_policy || 'GLM 5h 窗口'} · ${reset}` : reset
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
  if (snapshot.note) rows.push({ label: '说明', value: snapshot.note })
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

function clearAutoRefreshTimer() {
  if (autoRefreshTimer) {
    clearInterval(autoRefreshTimer)
    autoRefreshTimer = null
  }
}

function syncAutoRefreshTimer() {
  clearAutoRefreshTimer()
  if (!autoRefreshEnabled.value) return
  autoRefreshTimer = setInterval(() => {
    if (!loading.value) void refresh()
  }, AUTO_REFRESH_INTERVAL_MS)
}

function isAPIKeyHealthSupportedAccount(account: Account) {
  return account.type === 'apikey' && API_KEY_HEALTH_PLATFORMS.has(account.platform)
}

async function runHealthCheck(accountIds?: number[]) {
  if (checkingHealth.value) return
  checkingHealth.value = true
  clearHealthPollTimer()
  try {
    const started = await adminAPI.accounts.startAPIKeysHealthCheck(accountIds)
    const scope = accountIds && accountIds.length > 0 ? '选中 Key 账号' : '原始 Key 账号'
    errorMessage.value = `健康检测已启动：${started.total} 个${scope}`
    await pollHealthCheckStatus(started.job_id)
  } catch (error) {
    checkingHealth.value = false
    errorMessage.value = normalizeError(error) || '健康检测启动失败。'
  }
}

async function pollHealthCheckStatus(jobID?: string) {
  try {
    const status = await adminAPI.accounts.getAPIKeysHealthStatus()
    if (jobID && status.job_id && status.job_id !== jobID) {
      healthPollTimer = setTimeout(() => {
        void pollHealthCheckStatus(jobID)
      }, 1500)
      return
    }
    if (status.status === 'running') {
      const checked = status.result?.checked ?? 0
      const total = status.result?.total ?? 0
      errorMessage.value = total > 0 ? `健康检测中：${checked}/${total}` : '健康检测中...'
      healthPollTimer = setTimeout(() => {
        void pollHealthCheckStatus(jobID)
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

function goPoolUsage(pool: AccountPool) {
  const first = pool.accounts[0]
  if (first) {
    goUsage(first)
    return
  }
  router.push('/admin/usage')
}

watch(
  () => [filters.search, filters.platform, filters.status],
  () => scheduleRefresh()
)

watch(autoRefreshEnabled, syncAutoRefreshTimer)

onMounted(() => {
  void refresh()
  syncAutoRefreshTimer()
  document.addEventListener('keydown', handleKeydown)
})

onUnmounted(() => {
  if (refreshTimer) clearTimeout(refreshTimer)
  clearAutoRefreshTimer()
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

:global(.dark) .account-map-page :is(.account-map-toolbar, .account-map-stat-card, .account-map-filter-card, .account-map-empty-state, .account-map-inspector) {
  border-color: rgb(51 65 85);
  background: var(--account-map-surface);
  color: var(--account-map-ink);
}

:global(.dark) .account-map-page :is(.account-map-stat-card p, .account-map-stat-card small, .account-map-meta, .inspector-account-main small, .inspector-account-quota) {
  color: var(--account-map-muted);
}

:global(.dark) .account-map-page :is(.account-map-stat-card strong, .inspector-title, .inspector-account-main b) {
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

.account-map-secondary-button {
  border: 1px solid var(--account-map-border);
  background: var(--account-map-surface);
  color: var(--account-map-ink);
}

.account-map-secondary-button:hover {
  border-color: rgb(14 165 233 / 0.45);
  color: var(--account-map-accent-strong);
}

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

.quota-panel-glm {
  border-color: rgb(52 211 153 / 0.34);
  background: linear-gradient(135deg, rgb(236 253 245), rgb(239 246 255));
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

:global(.dark) .quota-panel-glm {
  border-color: rgb(52 211 153 / 0.42);
  background: rgb(6 78 59 / 0.24);
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

:global(.app-shell.app-shell-anti) .account-map-page :is(.account-map-toolbar, .account-map-filter-card, .account-map-empty-state, .account-map-inspector, .inspector-profile-card) {
  border: 3px solid var(--anti-ink) !important;
  border-radius: 0.55rem !important;
  background: var(--anti-paper) !important;
  box-shadow: 6px 6px 0 var(--anti-ink) !important;
  color: var(--anti-ink) !important;
  transform: none !important;
}

:global(.app-shell.app-shell-anti) .account-map-page :is(.account-map-stat-card, .account-map-stat-metrics span, .inspector-section, .inspector-metric, .inspector-account-row, .inspector-alert, .quota-stat-card, .quota-empty-state, .inspector-account-quota) {
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

:global(.app-shell.app-shell-anti) .account-map-page button:not(.inspector-account-row):not(.inspector-close) {
  border: 3px solid var(--anti-ink) !important;
  border-radius: 0.2rem !important;
  box-shadow: 4px 4px 0 var(--anti-ink) !important;
  color: var(--anti-ink) !important;
  font-weight: 950;
}

:global(.app-shell.app-shell-anti) .account-map-page button:not(.inspector-account-row):not(.inspector-close):hover {
  background: var(--anti-green) !important;
  transform: rotate(-1deg) translate(-1px, -1px);
}

:global(.app-shell.app-shell-anti) .account-map-page .quota-empty-state p,
:global(.app-shell.app-shell-anti) .account-map-page .quota-stat-card small,
:global(.app-shell.app-shell-anti) .account-map-page .inspector-profile-meta span {
  color: var(--anti-ink) !important;
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

:global(.dark .app-shell.app-shell-anti) .account-map-page :is(.account-map-toolbar, .account-map-filter-card, .account-map-empty-state, .account-map-inspector, .inspector-profile-card) {
  background: linear-gradient(135deg, #101827, #0b1020) !important;
  box-shadow: 6px 6px 0 #334155 !important;
}

:global(.dark .app-shell.app-shell-anti) .account-map-page :is(.account-map-stat-card, .account-map-stat-metrics span, .inspector-section, .inspector-metric, .inspector-account-row, .inspector-alert, .quota-stat-card, .quota-empty-state, .inspector-account-quota) {
  background: #111827 !important;
  box-shadow: 3px 3px 0 #334155 !important;
}

:global(.dark .app-shell.app-shell-anti) .account-map-page .account-map-toolbar {
  background:
    linear-gradient(90deg, rgb(250 204 21 / 0.22), transparent 46%),
    linear-gradient(135deg, #111827, #0b1020) !important;
}

:global(.dark .app-shell.app-shell-anti) .account-map-page .inspector-kicker {
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
:global(.dark .app-shell.app-shell-anti) .account-map-page .inspector-action {
  background: #0f172a !important;
  color: #f8fafc !important;
}

:global(.dark .app-shell.app-shell-anti) .account-map-page .inspector-action-primary {
  background: #2563eb !important;
  color: #ffffff !important;
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

.account-map-workspace {
  display: block;
}

.account-map-main-column {
  width: 100%;
}

.account-map-console {
  --account-map-border: rgb(44 62 79);
  --account-map-border-strong: rgb(68 88 105);
  --account-map-ink: rgb(235 242 248);
  --account-map-muted: rgb(155 168 181);
  --account-map-faint: rgb(103 119 135);
  --account-map-surface: rgb(18 35 47 / 0.88);
  --account-map-subtle: rgb(12 27 38 / 0.82);
  --account-map-soft: rgb(26 45 59 / 0.9);
  --account-map-shadow: 0 22px 60px rgb(0 0 0 / 0.28);
  display: grid;
  width: 100%;
  max-width: min(1760px, calc(100vw - 2rem));
  gap: 0.7rem;
  border-radius: 0;
  color: var(--account-map-ink);
  font-variant-numeric: tabular-nums;
}

.account-map-console::before {
  content: '';
  position: fixed;
  inset: 0;
  z-index: -1;
  background:
    radial-gradient(circle at 18% -8%, rgb(32 125 166 / 0.18), transparent 24rem),
    radial-gradient(circle at 82% 10%, rgb(55 86 115 / 0.16), transparent 22rem),
    linear-gradient(135deg, rgb(8 17 25), rgb(8 28 39) 48%, rgb(6 23 34));
}

.account-map-console .account-map-toolbar,
.account-map-console .account-map-stat-card,
.account-map-console .account-map-filter-card,
.account-map-console .account-map-group-panel,
.account-map-console .account-map-table-panel,
.account-map-console .account-map-inspector,
.account-map-console .account-map-empty-state {
  border: 1px solid var(--account-map-border);
  background:
    linear-gradient(180deg, rgb(22 42 56 / 0.92), rgb(12 29 40 / 0.9)),
    var(--account-map-surface);
  box-shadow: inset 0 1px 0 rgb(255 255 255 / 0.04);
}

.account-map-console .account-map-toolbar {
  border-radius: 0.35rem;
  padding: 0.75rem 0.9rem;
  background:
    radial-gradient(circle at 0 0, rgb(32 125 166 / 0.15), transparent 14rem),
    linear-gradient(180deg, rgb(16 32 44 / 0.96), rgb(9 23 34 / 0.94));
}

.account-map-console .account-map-toolbar-copy {
  align-items: center;
  gap: 1rem;
}

.account-map-console .account-map-toolbar h1 {
  gap: 0.45rem;
  color: var(--account-map-ink);
  font-size: 1.25rem;
  font-weight: 800;
  letter-spacing: 0;
}

.account-map-console .account-map-toolbar h1 svg {
  color: rgb(145 165 183);
}

.account-map-console .account-map-meta {
  margin-top: 0.25rem;
  gap: 0.7rem;
  color: rgb(132 148 163);
  font-size: 0.8rem;
}

.account-map-live-state {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
}

.account-map-console .account-map-toolbar-actions {
  flex-wrap: nowrap;
  gap: 0.65rem;
}

.account-map-console .account-map-secondary-button {
  min-height: 2rem;
  gap: 0.38rem;
  border-radius: 0.35rem;
  padding: 0.42rem 0.66rem;
  font-size: 0.8rem;
  font-weight: 650;
}

.account-map-console .account-map-secondary-button {
  border-color: rgb(65 83 99);
  background: rgb(13 27 38 / 0.8);
  color: rgb(221 231 240);
}

.account-map-console .account-map-secondary-button:hover {
  border-color: rgb(68 145 220);
  color: rgb(147 197 253);
}

.account-map-auto-toggle {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  color: rgb(224 232 240);
  font-size: 0.78rem;
  white-space: nowrap;
}

.account-map-auto-toggle input {
  position: absolute;
  opacity: 0;
  pointer-events: none;
}

.account-map-auto-toggle i {
  position: relative;
  display: inline-flex;
  width: 2rem;
  height: 1.1rem;
  border: 1px solid rgb(58 78 95);
  border-radius: 999px;
  background: rgb(28 44 58);
  transition: background-color 0.16s ease, border-color 0.16s ease;
}

.account-map-auto-toggle i::after {
  content: '';
  position: absolute;
  top: 0.12rem;
  left: 0.13rem;
  width: 0.76rem;
  height: 0.76rem;
  border-radius: 999px;
  background: rgb(148 163 184);
  transition: transform 0.16s ease, background-color 0.16s ease;
}

.account-map-auto-toggle input:checked + i {
  border-color: rgb(68 145 220);
  background: rgb(37 99 235);
}

.account-map-auto-toggle input:checked + i::after {
  transform: translateX(0.86rem);
  background: white;
}

.account-map-toolbar-divider {
  width: 1px;
  height: 1.35rem;
  background: rgb(67 83 99);
}

.account-map-updated {
  color: rgb(132 148 163);
  font-size: 0.76rem;
  white-space: nowrap;
}

.account-map-search {
  display: inline-flex;
  min-width: 230px;
  align-items: center;
  gap: 0.55rem;
  border: 1px solid rgb(48 67 83);
  border-radius: 0.35rem;
  background: rgb(8 20 30 / 0.78);
  padding: 0.42rem 0.6rem;
  color: rgb(116 132 148);
}

.account-map-search input {
  min-width: 0;
  width: 100%;
  border: 0;
  background: transparent;
  color: var(--account-map-ink);
  font-size: 0.78rem;
  outline: none;
}

.account-map-search input::placeholder {
  color: rgb(103 119 135);
}

.account-map-search kbd {
  flex: none;
  border: 1px solid rgb(58 76 92);
  border-radius: 0.26rem;
  background: rgb(25 41 54 / 0.82);
  padding: 0.04rem 0.32rem;
  color: rgb(141 158 174);
  font-size: 0.68rem;
  font-weight: 760;
  line-height: 1.35;
}

.account-map-console .account-map-summary-grid {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr)) !important;
  gap: 0.7rem;
}

.account-map-console .account-map-stat-card {
  position: relative;
  min-height: 5.4rem;
  overflow: hidden;
  border-radius: 0.35rem;
  padding: 0.75rem 0.85rem 0.68rem;
}

.account-map-stat-head {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 0.6rem;
}

.account-map-stat-head p {
  min-width: 0;
  overflow: hidden;
  color: rgb(226 236 244);
  font-size: 0.82rem;
  font-weight: 680;
  letter-spacing: 0;
  text-overflow: ellipsis;
  text-transform: none;
  white-space: nowrap;
}

.account-map-stat-info {
  display: inline-flex;
  width: 0.9rem;
  height: 0.9rem;
  align-items: center;
  justify-content: center;
  border: 1px solid rgb(93 111 128);
  border-radius: 999px;
  color: rgb(133 150 166);
  font-size: 0.58rem;
  font-style: normal;
  font-weight: 700;
}

.account-map-stat-icon {
  display: inline-flex;
  width: 2rem;
  height: 2rem;
  align-items: center;
  justify-content: center;
  border-radius: 0.4rem;
  border: 1px solid currentColor;
  background: color-mix(in srgb, currentColor 13%, transparent);
  color: currentColor;
}

.account-map-stat-icon-good { color: rgb(55 199 119); }
.account-map-stat-icon-warn { color: rgb(224 179 48); }
.account-map-stat-icon-danger { color: rgb(232 86 76); }
.account-map-stat-icon-orange { color: rgb(231 112 39); }
.account-map-stat-icon-accent { color: rgb(68 145 220); }

.account-map-console .account-map-stat-card strong {
  margin-top: 0.45rem;
  color: rgb(245 249 252);
  font-size: 1.8rem;
  font-weight: 780;
  letter-spacing: 0;
}

.account-map-console .account-map-stat-card small {
  margin-top: 0.28rem;
  color: rgb(143 158 173);
  font-size: 0.72rem;
}

.account-map-stat-spark {
  position: absolute;
  right: 0.72rem;
  bottom: 0.68rem;
  width: 5rem;
  height: 1.15rem;
  opacity: 0.9;
  background:
    linear-gradient(135deg, transparent 0 14%, currentColor 14% 18%, transparent 18% 32%, currentColor 32% 36%, transparent 36% 48%, currentColor 48% 52%, transparent 52% 63%, currentColor 63% 67%, transparent 67% 82%, currentColor 82% 86%, transparent 86%),
    linear-gradient(90deg, transparent, color-mix(in srgb, currentColor 22%, transparent), transparent);
  color: rgb(70 177 122);
  clip-path: polygon(0 64%, 12% 70%, 22% 58%, 34% 62%, 46% 42%, 58% 54%, 70% 35%, 84% 50%, 100% 28%, 100% 100%, 0 100%);
}

.account-map-stat-warn .account-map-stat-spark { color: rgb(224 179 48); }
.account-map-stat-danger .account-map-stat-spark { color: rgb(232 86 76); }
.account-map-stat-orange .account-map-stat-spark { color: rgb(231 112 39); }
.account-map-stat-accent .account-map-stat-spark { color: rgb(68 145 220); }

.account-map-console .account-map-stat-metrics {
  display: none;
}

.account-map-state-strip {
  display: flex;
  min-width: 0;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem;
  border: 1px solid var(--account-map-border);
  border-radius: 0.35rem;
  background:
    linear-gradient(180deg, rgb(16 32 44 / 0.9), rgb(8 23 34 / 0.92)),
    var(--account-map-surface);
  padding: 0.58rem 0.68rem;
  box-shadow: inset 0 1px 0 rgb(255 255 255 / 0.04);
}

.account-map-state-pill {
  display: inline-flex;
  min-width: 0;
  align-items: center;
  gap: 0.45rem;
  border: 1px solid rgb(51 70 86);
  border-radius: 0.34rem;
  background: rgb(11 25 36 / 0.72);
  padding: 0.4rem 0.55rem;
  color: rgb(190 205 218);
  font-size: 0.72rem;
  font-weight: 680;
  line-height: 1.35;
}

.account-map-state-pill > span:not(.account-map-status-dot):not(.account-map-line-swatch):not(.account-map-state-pulse) {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.account-map-state-pill button {
  flex: none;
  color: rgb(147 197 253);
  font-weight: 820;
}

.account-map-state-pill button:disabled {
  cursor: not-allowed;
  opacity: 0.55;
}

.account-map-state-loading {
  border-color: rgb(68 145 220 / 0.48);
  background: rgb(12 74 110 / 0.18);
  color: rgb(191 219 254);
}

.account-map-state-error {
  border-color: rgb(248 113 113 / 0.48);
  background: rgb(127 29 29 / 0.22);
  color: rgb(254 202 202);
}

.account-map-state-fallback,
.account-map-state-info {
  border-color: rgb(68 145 220 / 0.42);
  background: rgb(12 74 110 / 0.15);
}

.account-map-state-pulse {
  width: 0.55rem;
  height: 0.55rem;
  flex: none;
  border-radius: 999px;
  background: rgb(96 165 250);
  box-shadow: 0 0 0 0 rgb(96 165 250 / 0.45);
  animation: account-map-pulse 1.35s ease-out infinite;
}

@keyframes account-map-pulse {
  0% { box-shadow: 0 0 0 0 rgb(96 165 250 / 0.45); }
  100% { box-shadow: 0 0 0 0.55rem rgb(96 165 250 / 0); }
}

.account-map-console .account-map-workspace {
  display: grid;
  grid-template-columns: minmax(300px, 310px) minmax(0, 1fr) minmax(315px, 340px);
  gap: 0.7rem;
  align-items: stretch;
  height: clamp(34rem, calc(100dvh - 13.8rem), 54rem);
  min-height: 0;
}

.account-map-right-rail {
  min-width: 0;
  min-height: 0;
}

.account-map-left-rail {
  display: grid;
  min-width: 0;
  min-height: 0;
  grid-template-rows: auto minmax(0, 1fr);
  gap: 0.7rem;
}

.account-map-console .account-map-filter-card,
.account-map-console .account-map-group-panel,
.account-map-console .account-map-table-panel,
.account-map-console .account-map-inspector {
  border-radius: 0.35rem;
}

.account-map-console .account-map-filter-card {
  padding: 0.85rem;
}

.account-map-console .account-map-filter-header {
  margin-bottom: 0.65rem;
}

.account-map-console .account-map-filter-header h2,
.account-map-group-panel-head h2 {
  color: rgb(235 242 248);
  font-size: 0.86rem;
  font-weight: 760;
  letter-spacing: 0;
}

.account-map-filter-header-spaced {
  margin-top: 0.9rem;
}

.account-map-chip-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.45rem;
}

.account-map-filter-chip {
  display: inline-flex;
  min-width: 0;
  min-height: 2rem;
  align-items: center;
  gap: 0.45rem;
  border: 1px solid rgb(51 70 86);
  border-radius: 0.32rem;
  background: rgb(13 29 41 / 0.78);
  padding: 0.42rem 0.55rem;
  color: rgb(214 224 233);
  font-size: 0.76rem;
  font-weight: 620;
  text-align: left;
  transition: border-color 0.16s ease, background-color 0.16s ease, color 0.16s ease;
}

.account-map-filter-chip:hover,
.account-map-filter-chip.active {
  border-color: rgb(68 145 220);
  background: rgb(23 49 68);
  color: white;
}

.account-map-platform-mark {
  display: inline-flex;
  width: 1.42rem;
  height: 1.42rem;
  flex: none;
  align-items: center;
  justify-content: center;
  border-radius: 0.32rem;
  border: 1px solid rgb(68 145 220 / 0.45);
  background: rgb(23 70 122 / 0.38);
  color: rgb(139 194 248);
  font-size: 0.58rem;
  font-weight: 850;
}

.account-map-status-chip-grid {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.account-map-status-filter-chip {
  gap: 0.42rem;
}

.account-map-status-dot {
  display: inline-flex;
  width: 0.55rem;
  height: 0.55rem;
  flex: none;
  border-radius: 999px;
  background: currentColor;
  box-shadow: 0 0 0 3px color-mix(in srgb, currentColor 17%, transparent);
}

.account-map-status-filter-all { color: rgb(148 163 184); }
.account-map-status-filter-healthy { color: rgb(52 211 153); }
.account-map-status-filter-degraded { color: rgb(250 204 21); }
.account-map-status-filter-rate_limited { color: rgb(251 146 60); }
.account-map-status-filter-error { color: rgb(248 113 113); }
.account-map-status-filter-disabled { color: rgb(116 132 148); }

.account-map-rail-notice {
  margin-top: 0.7rem;
  border-radius: 0.32rem;
  border: 1px solid rgb(51 70 86);
  padding: 0.55rem 0.6rem;
  color: rgb(192 207 221);
  font-size: 0.72rem;
  line-height: 1.4;
}

.account-map-rail-notice button {
  margin-left: 0.45rem;
  color: rgb(147 197 253);
  font-weight: 760;
}

.account-map-rail-notice-warn {
  border-color: rgb(245 158 11 / 0.45);
  background: rgb(120 53 15 / 0.2);
}

.account-map-rail-notice-info {
  border-color: rgb(68 145 220 / 0.45);
  background: rgb(12 74 110 / 0.18);
}

.account-map-group-panel {
  display: flex;
  min-height: 0;
  flex-direction: column;
  overflow: hidden;
}

.account-map-group-panel-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid var(--account-map-border);
  padding: 0.78rem 0.85rem;
}

.account-map-group-panel-head button {
  display: inline-flex;
  width: 1.65rem;
  height: 1.65rem;
  align-items: center;
  justify-content: center;
  border: 1px solid rgb(67 86 102);
  border-radius: 0.32rem;
  color: rgb(154 171 187);
}

.account-map-group-table-head,
.account-map-group-row {
  display: grid;
  grid-template-columns: minmax(5.6rem, 1fr) 1.8rem 3.1rem 3.2rem 2.7rem;
  gap: 0.28rem;
  align-items: center;
}

.account-map-group-table-head {
  padding: 0.72rem 0.85rem 0.45rem;
  color: rgb(122 138 153);
  font-size: 0.68rem;
  font-weight: 700;
}

.account-map-group-list {
  display: grid;
  min-height: 0;
  overflow-y: auto;
  padding: 0 0.45rem 0.7rem;
}

.account-map-group-row {
  border: 1px solid transparent;
  border-radius: 0.35rem;
  padding: 0.58rem 0.4rem;
  color: rgb(215 226 236);
  font-size: 0.76rem;
  text-align: left;
  transition: border-color 0.16s ease, background-color 0.16s ease;
}

.account-map-group-row:hover,
.account-map-group-row.active {
  border-color: rgb(68 145 220);
  background: rgb(23 71 122 / 0.38);
}

.account-map-group-name,
.account-map-group-count {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.account-map-group-count {
  color: rgb(197 211 224);
}

.account-map-risk-chip {
  display: inline-flex;
  width: max-content;
  max-width: 100%;
  border-radius: 0.28rem;
  padding: 0.18rem 0.36rem;
  font-size: 0.68rem;
  font-weight: 800;
}

.account-map-risk-healthy {
  background: rgb(52 211 153 / 0.15);
  color: rgb(74 222 128);
}

.account-map-risk-degraded,
.account-map-risk-rate_limited {
  background: rgb(245 158 11 / 0.18);
  color: rgb(251 191 36);
}

.account-map-risk-error,
.account-map-risk-disabled {
  background: rgb(239 68 68 / 0.18);
  color: rgb(248 113 113);
}

.account-map-health-cell {
  display: grid;
  min-width: 0;
  grid-template-columns: minmax(0, 1fr);
  align-items: center;
  gap: 0.35rem;
}

.account-map-health-track {
  height: 0.32rem;
  overflow: hidden;
  border-radius: 999px;
  background: rgb(37 52 66);
}

.account-map-health-track span {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: linear-gradient(90deg, rgb(74 222 128), rgb(125 211 252));
}

.account-map-health-cell small {
  display: none;
}

.account-map-line-swatch {
  display: inline-flex;
  width: 1.45rem;
  height: 0;
  border-top: 2px solid rgb(45 143 255);
}

.account-map-line-swatch-dashed {
  border-top-style: dashed;
  border-top-color: rgb(151 164 176);
}

.account-map-console .account-map-main-column {
  width: 100%;
  min-height: 0;
  gap: 0.7rem;
}

.account-map-console .account-map-table-panel {
  display: flex;
  min-height: 0;
  flex-direction: column;
  overflow: hidden;
  background:
    linear-gradient(180deg, rgb(16 34 46 / 0.96), rgb(8 24 35 / 0.98)),
    var(--account-map-surface);
}

.account-map-console .account-map-account-detail-panel {
  flex: 1 1 0;
  margin-top: 0;
}

.account-map-table-card {
  display: flex;
  min-width: 0;
  min-height: 0;
  flex: 1 1 auto;
  flex-direction: column;
}

.account-map-table-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  border-bottom: 1px solid var(--account-map-border);
  padding: 0.9rem 1rem;
}

.account-map-table-title h2 {
  min-width: 0;
  overflow: hidden;
  color: rgb(245 249 252);
  font-size: 1rem;
  font-weight: 780;
  line-height: 1.2;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.account-map-table-title span {
  flex: none;
  color: rgb(143 158 173);
  font-size: 0.72rem;
  font-weight: 680;
}

.account-map-table-scroll {
  min-width: 0;
  min-height: 0;
  overflow: auto;
}

.account-map-data-table {
  width: 100%;
  min-width: 100%;
  border-collapse: collapse;
  table-layout: fixed;
  color: rgb(224 234 242);
  font-size: 0.72rem;
  font-variant-numeric: tabular-nums;
}

.account-map-pool-table {
  min-width: 54rem;
}

.account-map-account-table {
  min-width: 100%;
}

.account-map-pool-table :is(th, td):nth-child(1) { width: 10rem; }
.account-map-pool-table :is(th, td):nth-child(2) { width: 6.1rem; }
.account-map-pool-table :is(th, td):nth-child(3) { width: 5.3rem; }
.account-map-pool-table :is(th, td):nth-child(4) { width: 4.7rem; }
.account-map-pool-table :is(th, td):nth-child(5) { width: 4.1rem; }
.account-map-pool-table :is(th, td):nth-child(6) { width: 6.2rem; }
.account-map-pool-table :is(th, td):nth-child(7) { width: 5.2rem; }
.account-map-pool-table :is(th, td):nth-child(8) { width: 4.5rem; }
.account-map-pool-table :is(th, td):nth-child(9) { width: 5.6rem; }

.account-map-account-table :is(th, td):nth-child(1) { width: 8.6rem; }
.account-map-account-table :is(th, td):nth-child(2) { width: 5.6rem; }
.account-map-account-table :is(th, td):nth-child(3) { width: 4.5rem; }
.account-map-account-table :is(th, td):nth-child(4) { width: 4rem; }
.account-map-account-table :is(th, td):nth-child(5) { width: 6.2rem; }
.account-map-account-table :is(th, td):nth-child(6) { width: 6rem; }
.account-map-account-table :is(th, td):nth-child(7) { width: 9rem; }
.account-map-account-table :is(th, td):nth-child(8) { width: 7.4rem; }
.account-map-account-table :is(th, td):nth-child(9) { width: 4.4rem; }

.account-map-account-table :is(th, td):nth-child(7),
.account-map-account-table :is(th, td):nth-child(8) {
  white-space: normal;
}

.account-map-account-table td:nth-child(7),
.account-map-account-table td:nth-child(8) {
  line-height: 1.35;
}

.account-map-data-table th,
.account-map-data-table td {
  border-bottom: 1px solid rgb(39 56 72 / 0.92);
  overflow: hidden;
  padding: 0.68rem 0.5rem;
  text-align: left;
  text-overflow: ellipsis;
  vertical-align: middle;
  white-space: nowrap;
}

.account-map-data-table th {
  background: rgb(19 36 49 / 0.68);
  color: rgb(177 191 204);
  font-size: 0.68rem;
  font-weight: 720;
}

.account-map-data-table tbody tr {
  cursor: pointer;
  background: rgb(11 26 38 / 0.18);
  transition: background-color 0.14s ease, box-shadow 0.14s ease;
}

.account-map-data-table tbody tr:hover,
.account-map-data-table tbody tr:focus-visible,
.account-map-data-table tbody tr.active {
  background: rgb(23 71 122 / 0.28);
  box-shadow: inset 2px 0 0 rgb(45 143 255), inset -1px 0 0 rgb(45 143 255 / 0.5);
  outline: none;
}

.account-map-table-primary {
  max-width: 100%;
  overflow: hidden;
  color: rgb(239 246 252);
  font-weight: 760;
  text-align: left;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.account-map-table-primary:hover {
  color: rgb(125 188 255);
}

.account-map-platform-pill,
.account-map-status-label,
.account-map-row-action {
  display: inline-flex;
  align-items: center;
  border-radius: 0.34rem;
  border: 1px solid rgb(67 86 102);
  background: rgb(13 29 41 / 0.75);
  max-width: 100%;
  overflow: hidden;
  padding: 0.2rem 0.4rem;
  color: rgb(218 230 240);
  font-size: 0.72rem;
  font-weight: 720;
}

.account-map-status-label-healthy {
  border-color: rgb(52 211 153 / 0.28);
  background: rgb(22 101 52 / 0.24);
  color: rgb(74 222 128);
}

.account-map-status-label-degraded,
.account-map-status-label-rate_limited {
  border-color: rgb(245 158 11 / 0.34);
  background: rgb(120 53 15 / 0.26);
  color: rgb(251 191 36);
}

.account-map-status-label-error,
.account-map-status-label-disabled {
  border-color: rgb(248 113 113 / 0.34);
  background: rgb(127 29 29 / 0.24);
  color: rgb(248 113 113);
}

.account-map-number-good { color: rgb(52 211 153); }
.account-map-number-warn { color: rgb(250 204 21); }
.account-map-number-danger { color: rgb(248 113 113); }

.account-map-quota-cell {
  display: inline-grid;
  min-width: 0;
  width: 100%;
  grid-template-columns: 1.8rem minmax(1.9rem, 1fr);
  align-items: center;
  gap: 0.28rem;
}

.account-map-mini-bar {
  display: inline-flex;
  height: 0.34rem;
  overflow: hidden;
  border-radius: 999px;
  background: rgb(36 51 65);
}

.account-map-mini-bar i {
  display: block;
  height: 100%;
  border-radius: inherit;
}

.account-map-mini-bar-good i { background: rgb(52 211 153); }
.account-map-mini-bar-warn i { background: rgb(250 204 21); }
.account-map-mini-bar-danger i { background: rgb(248 113 113); }
.account-map-mini-bar-muted i { background: rgb(100 116 139); }

.account-map-account-status {
  display: inline-flex;
  align-items: center;
  gap: 0.38rem;
  color: rgb(203 213 225);
  font-weight: 720;
}

.account-map-account-status i {
  width: 0.48rem;
  height: 0.48rem;
  flex: none;
  border-radius: 999px;
  background: currentColor;
}

.account-map-account-status-healthy {
  color: rgb(52 211 153);
}

.account-map-account-status-degraded {
  color: rgb(250 204 21);
}

.account-map-account-status-rate_limited {
  color: rgb(251 146 60);
}

.account-map-account-status-error {
  color: rgb(248 113 113);
}

.account-map-account-status-disabled {
  color: rgb(116 132 148);
}

.account-map-row-action {
  border-color: transparent;
  background: transparent;
  color: rgb(96 165 250);
  padding-inline: 0;
}

.account-map-row-action:hover {
  color: rgb(147 197 253);
}

.account-map-table-footnote {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin: 0.72rem 0.86rem 0.86rem;
  border: 1px solid rgb(45 62 78);
  border-radius: 0.34rem;
  background: rgb(20 36 49 / 0.55);
  padding: 0.6rem 0.7rem;
  color: rgb(162 177 191);
  font-size: 0.72rem;
}

.account-map-table-footnote button {
  color: rgb(96 165 250);
  font-weight: 780;
}

.account-map-status-info-dot {
  display: inline-flex;
  width: 0.9rem;
  height: 0.9rem;
  flex: none;
  align-items: center;
  justify-content: center;
  border: 1px solid rgb(113 128 143);
  border-radius: 999px;
  color: rgb(175 189 201);
  font-size: 0.58rem;
  font-weight: 760;
}

.account-map-empty-state-compact,
.account-map-loading-shell-compact {
  min-height: 16rem;
}

.account-map-console .account-map-inspector {
  position: sticky;
  top: 0.75rem;
  height: 100%;
  max-height: none;
  overflow-y: auto;
  border: 1px solid var(--account-map-border);
  background:
    linear-gradient(180deg, rgb(18 35 47 / 0.96), rgb(8 24 35 / 0.96)),
    var(--account-map-surface);
  box-shadow: inset 0 1px 0 rgb(255 255 255 / 0.04);
}

.account-map-console .inspector-drawer-header {
  border-bottom-color: var(--account-map-border);
  background: rgb(13 29 41 / 0.94);
  padding: 0.72rem 0.82rem;
}

.account-map-console .inspector-drawer-header-left .inspector-title {
  color: rgb(245 249 252);
  font-size: 0.98rem;
  letter-spacing: 0;
}

.account-map-console .inspector-kicker {
  color: rgb(148 163 184);
  font-size: 0.68rem;
  letter-spacing: 0;
  text-transform: none;
}

.account-map-console .inspector-subtitle {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  color: rgb(156 172 187);
  font-size: 0.72rem;
  line-height: 1.35;
}

.inspector-selection-actions {
  display: inline-flex;
  flex: none;
  align-items: center;
  gap: 0.45rem;
}

.inspector-selected-badge {
  border-radius: 0.3rem;
  background: rgb(37 99 235 / 0.32);
  padding: 0.16rem 0.46rem;
  color: rgb(147 197 253);
  font-size: 0.66rem;
  font-weight: 780;
}

.account-map-console .inspector-close {
  width: 1.8rem;
  height: 1.8rem;
  border-color: rgb(64 84 101);
  border-radius: 0.32rem;
  color: rgb(148 163 184);
}

.account-map-console .inspector-close:hover {
  background: rgb(20 46 63);
  color: rgb(226 238 248);
  transform: none;
}

.inspector-dot-healthy { color: rgb(52 211 153); }
.inspector-dot-degraded,
.inspector-dot-rate_limited { color: rgb(250 204 21); }
.inspector-dot-error { color: rgb(248 113 113); }
.inspector-dot-disabled { color: rgb(116 132 148); }

.inspector-pool-summary {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.48rem;
  padding: 0.72rem 0.82rem 0.18rem;
}

.inspector-pool-summary div {
  min-width: 0;
  border: 1px solid rgb(56 77 94);
  border-radius: 0.38rem;
  background: rgb(9 25 37 / 0.72);
  padding: 0.55rem;
}

.inspector-pool-summary span,
.inspector-empty-line,
.inspector-model-row strong,
.inspector-event-row small,
.inspector-event-row em,
.inspector-cooldown-head,
.inspector-cooldown-row,
.inspector-request-row {
  color: rgb(152 168 183);
  font-size: 0.7rem;
}

.inspector-pool-summary strong {
  display: block;
  margin-top: 0.22rem;
  overflow: hidden;
  color: rgb(241 247 252);
  font-size: 0.96rem;
  font-weight: 820;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.inspector-diagnosis-card {
  margin: 0.7rem 0.82rem 0;
  border: 1px solid rgb(56 77 94);
  border-radius: 0.36rem;
  background: rgb(10 26 38 / 0.72);
  padding: 0.72rem;
}

.inspector-diagnosis-card h3 {
  margin-top: 0.58rem;
  color: rgb(235 242 248);
  font-size: 0.78rem;
  font-weight: 760;
}

.inspector-diagnosis-card p {
  margin-top: 0.35rem;
  color: rgb(176 190 203);
  font-size: 0.72rem;
  line-height: 1.55;
}

.inspector-diagnosis-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.7rem;
}

.inspector-diagnosis-head strong {
  flex: none;
  color: rgb(207 220 231);
  font-size: 0.72rem;
  font-weight: 720;
}

.inspector-diagnosis-healthy {
  border-color: rgb(52 211 153 / 0.32);
}

.inspector-diagnosis-degraded,
.inspector-diagnosis-rate_limited {
  border-color: rgb(245 158 11 / 0.36);
  background: rgb(120 53 15 / 0.18);
}

.inspector-diagnosis-error,
.inspector-diagnosis-disabled {
  border-color: rgb(248 113 113 / 0.36);
  background: rgb(127 29 29 / 0.2);
}

.inspector-action-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.5rem;
  padding: 0.72rem 0.82rem 0.1rem;
}

.inspector-model-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 0.42rem;
}

.inspector-model-chips span {
  max-width: 100%;
  overflow: hidden;
  border: 1px solid rgb(62 82 98);
  border-radius: 0.32rem;
  background: rgb(12 29 41 / 0.74);
  padding: 0.3rem 0.48rem;
  color: rgb(220 232 241);
  font-size: 0.72rem;
  font-weight: 720;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.inspector-reason-table {
  overflow: hidden;
  border: 1px solid rgb(49 69 86);
  border-radius: 0.34rem;
}

.inspector-reason-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 0.5rem;
  align-items: center;
  border-bottom: 1px solid rgb(39 56 72);
  background: rgb(12 29 41 / 0.56);
  padding: 0.38rem 0.5rem;
  color: rgb(184 199 212);
  font-size: 0.72rem;
}

.inspector-reason-row:last-child {
  border-bottom: 0;
}

.inspector-reason-row span {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.inspector-reason-row strong {
  color: rgb(231 241 248);
  font-weight: 820;
}

.inspector-reason-healthy { color: rgb(52 211 153); }
.inspector-reason-degraded,
.inspector-reason-rate_limited { color: rgb(250 204 21); }
.inspector-reason-error { color: rgb(248 113 113); }
.inspector-reason-disabled { color: rgb(148 163 184); }

.account-map-console .inspector-section {
  margin: 0.62rem 0.82rem 0;
  border-color: rgb(54 74 90);
  border-radius: 0.36rem;
  background: rgb(10 26 38 / 0.62);
  padding: 0.72rem;
}

.account-map-console .inspector-section-title {
  margin-bottom: 0.58rem;
}

.account-map-console .inspector-section-title h3 {
  color: rgb(235 242 248);
  font-size: 0.8rem;
  font-weight: 760;
}

.account-map-console .inspector-section-title p,
.account-map-console .inspector-section-title span {
  color: rgb(138 154 169);
  font-size: 0.68rem;
}

.account-map-console .inspector-row {
  border-bottom-color: rgb(44 62 79);
  color: rgb(211 224 235);
  font-size: 0.74rem;
}

.account-map-console .inspector-row span:first-child {
  color: rgb(132 148 163) !important;
}

.account-map-console .inspector-row span:last-child {
  color: rgb(226 238 248) !important;
}

.inspector-model-list,
.inspector-event-list,
.inspector-cooldown-table,
.inspector-request-list,
.inspector-account-list {
  display: grid;
  gap: 0.42rem;
}

.inspector-model-row,
.inspector-event-row,
.inspector-cooldown-row,
.inspector-request-row {
  min-width: 0;
  border: 1px solid rgb(49 69 86);
  border-radius: 0.34rem;
  background: rgb(12 29 41 / 0.72);
}

.inspector-model-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.55rem;
  padding: 0.36rem 0.48rem;
}

.inspector-model-row span {
  min-width: 0;
  overflow: hidden;
  color: rgb(221 233 243);
  font-size: 0.72rem;
  font-weight: 720;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.inspector-empty-line {
  border: 1px dashed rgb(56 77 94);
  border-radius: 0.34rem;
  padding: 0.55rem;
}

.inspector-event-row {
  display: grid;
  width: 100%;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: start;
  gap: 0.48rem;
  padding: 0.48rem;
  text-align: left;
}

.inspector-event-row b,
.inspector-event-row small,
.inspector-event-row em,
.inspector-cooldown-row span,
.inspector-request-row span {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.inspector-event-row b {
  display: block;
  color: rgb(232 242 250);
  font-size: 0.72rem;
}

.inspector-event-row small {
  display: block;
  margin-top: 0.2rem;
}

.inspector-event-row em {
  border-radius: 0.24rem;
  background: color-mix(in srgb, currentColor 15%, transparent);
  padding: 0.12rem 0.32rem;
  color: currentColor;
  font-style: normal;
  font-weight: 780;
}

.inspector-cooldown-head,
.inspector-cooldown-row,
.inspector-request-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(4.2rem, 0.8fr) minmax(0, 1.25fr);
  gap: 0.42rem;
  align-items: center;
  padding: 0.4rem 0.46rem;
  text-align: left;
}

.inspector-cooldown-head {
  border-bottom: 1px solid rgb(44 62 79);
  font-weight: 780;
}

.inspector-cooldown-row {
  width: 100%;
}

.inspector-cooldown-row:hover,
.inspector-event-row:hover,
.inspector-request-row:hover {
  border-color: rgb(68 145 220 / 0.7);
  background: rgb(18 43 58 / 0.88);
}

.inspector-failover-chain {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.38rem;
}

.inspector-failover-chain button {
  min-width: 0;
  max-width: 100%;
  border: 1px solid rgb(54 74 90);
  border-radius: 0.34rem;
  background: rgb(12 29 41 / 0.72);
  padding: 0.34rem 0.52rem;
  color: rgb(181 197 212);
  font-size: 0.7rem;
  font-weight: 760;
}

.inspector-failover-chain button::after {
  content: '>';
  margin-left: 0.38rem;
  color: rgb(85 103 119);
}

.inspector-failover-chain button:last-child::after {
  content: '';
  margin-left: 0;
}

.inspector-failover-chain button.active {
  border-color: rgb(68 145 220);
  background: rgb(23 71 122 / 0.42);
  color: rgb(191 219 254);
}

.inspector-request-row {
  width: 100%;
  grid-template-columns: minmax(0, 1fr) 3.2rem minmax(0, 0.8fr) minmax(0, 1fr);
}

.account-map-node-status-dot {
  height: 0.48rem;
  width: 0.48rem;
  flex: none;
  border-radius: 999px;
  background: currentColor;
  box-shadow: 0 0 0 3px color-mix(in srgb, currentColor 18%, transparent);
}

.account-map-console .account-pool-platform {
  border-color: rgb(57 77 93);
  background: rgb(11 28 40 / 0.78);
  color: rgb(200 214 227);
}

.account-map-loading-shell {
  display: grid;
  gap: 0.75rem;
  min-height: 36rem;
  align-content: center;
}

.account-map-skeleton-card {
  min-height: 8rem;
  border-radius: 0.55rem;
  background: linear-gradient(90deg, rgb(22 42 56), rgb(35 58 73), rgb(22 42 56));
  background-size: 220% 100%;
  animation: account-map-skeleton 1.2s ease-in-out infinite;
}

@keyframes account-map-skeleton {
  0% { background-position: 100% 0; }
  100% { background-position: -100% 0; }
}

@media (max-width: 1260px) {
  .account-map-console .account-map-summary-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr)) !important;
  }

  .account-map-console .account-map-workspace {
    grid-template-columns: 1fr;
    height: auto;
  }

  .account-map-left-rail {
    grid-template-columns: minmax(0, 0.95fr) minmax(0, 1.05fr);
    align-items: start;
    grid-template-rows: none;
  }

  .account-map-console .account-map-inspector {
    position: static;
    height: auto;
    max-height: none;
  }
}

@media (max-width: 900px) {
  .account-map-console .account-map-toolbar-copy {
    align-items: stretch;
    flex-direction: column;
  }

  .account-map-console .account-map-toolbar-actions {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    flex-wrap: wrap;
    justify-content: flex-start;
  }

  .account-map-console .account-map-toolbar-actions > * {
    min-width: 0;
  }

  .account-map-search {
    grid-column: 1 / -1;
    min-width: 0;
    width: 100%;
  }

  .account-map-updated {
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .account-map-toolbar-divider {
    display: none;
  }
}

@media (max-width: 700px) {
  .account-map-console {
    max-width: calc(100vw - 1rem);
    gap: 0.55rem;
  }

  .account-map-console .account-map-summary-grid {
    grid-template-columns: 1fr !important;
  }

  .account-map-console .account-map-toolbar-actions,
  .account-map-left-rail {
    grid-template-columns: 1fr;
  }

  .account-map-console .account-map-secondary-button,
  .account-map-auto-toggle,
  .account-map-search {
    width: 100%;
  }

  .account-map-auto-toggle {
    justify-content: space-between;
    border: 1px solid rgb(51 70 86);
    border-radius: 0.34rem;
    background: rgb(11 25 36 / 0.72);
    padding: 0.42rem 0.55rem;
  }

  .account-map-state-strip,
  .account-map-state-pill {
    align-items: stretch;
    flex-direction: column;
  }

  .account-map-state-pill > span:not(.account-map-status-dot):not(.account-map-line-swatch):not(.account-map-state-pulse) {
    white-space: normal;
  }

  .account-map-chip-grid,
  .account-map-status-chip-grid {
    grid-template-columns: 1fr;
  }

  .account-map-group-table-head {
    display: none;
  }

  .account-map-group-row {
    grid-template-columns: minmax(0, 1fr) auto;
  }

  .account-map-group-platform,
  .account-map-group-count,
  .account-map-risk-chip {
    display: none;
  }

  .account-map-health-cell {
    width: 5.5rem;
  }

  .inspector-cooldown-head,
  .inspector-cooldown-row,
  .inspector-request-row {
    grid-template-columns: 1fr;
  }

  .inspector-pool-summary,
  .quota-stat-grid,
  .inspector-snapshot-grid {
    grid-template-columns: 1fr;
  }

  .account-map-console .inspector-section {
    margin-inline: 0.62rem;
  }
}

@media (max-width: 430px) {
  .account-map-console {
    max-width: calc(100vw - 0.5rem);
  }

  .account-map-console .account-map-toolbar,
  .account-map-console .account-map-filter-card,
  .account-map-console .account-map-group-panel,
  .account-map-console .account-map-inspector {
    border-radius: 0.28rem;
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

.dark .account-map-page :is(.account-map-toolbar, .account-map-stat-card, .account-map-filter-card, .account-map-empty-state, .account-map-inspector) {
  border-color: rgb(51 65 85);
  background: var(--account-map-surface);
  color: var(--account-map-ink);
}

.dark .account-map-page .inspector-drawer-header {
  background: rgb(15 23 42 / 0.97);
  border-bottom-color: rgb(51 65 85);
}

.dark .account-map-page :is(.account-map-stat-card p, .account-map-stat-card small, .account-map-meta, .account-map-field > span, .inspector-subtitle, .inspector-section-title p, .inspector-section-title span, .inspector-account-main small, .inspector-account-quota) {
  color: var(--account-map-muted);
}

.dark .account-map-page :is(.account-map-stat-card strong, .account-map-filter-header h2, .inspector-title, .inspector-section-title h3, .inspector-metric p:last-child, .inspector-account-main b) {
  color: var(--account-map-ink);
}

.dark .account-map-page :is(.account-map-control, .account-map-stat-metrics span, .inspector-metric, .inspector-section, .inspector-account-row, .inspector-account-quota) {
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

.app-shell.app-shell-anti .account-map-page :is(.account-map-toolbar, .account-map-filter-card, .account-map-empty-state, .account-map-inspector, .inspector-profile-card) {
  border: 3px solid var(--anti-ink) !important;
  border-radius: 0.55rem !important;
  background: var(--anti-paper) !important;
  box-shadow: 6px 6px 0 var(--anti-ink) !important;
  color: var(--anti-ink) !important;
  transform: none !important;
}

.app-shell.app-shell-anti .account-map-page :is(.account-map-stat-card, .account-map-stat-metrics span, .inspector-section, .inspector-metric, .inspector-account-row, .inspector-alert, .quota-stat-card, .quota-empty-state, .inspector-account-quota) {
  border: 2px solid var(--anti-ink) !important;
  border-radius: 0.45rem !important;
  background: var(--anti-paper) !important;
  box-shadow: 3px 3px 0 var(--anti-ink) !important;
  color: var(--anti-ink) !important;
}

.app-shell.app-shell-anti .account-map-page .account-map-toolbar {
  background: var(--anti-yellow) !important;
}

.app-shell.app-shell-anti .account-map-page .inspector-kicker {
  display: inline-flex;
  border: 3px solid var(--anti-ink);
  background: var(--anti-red);
  color: var(--anti-paper) !important;
  padding: 0.2rem 0.35rem;
}

.app-shell.app-shell-anti .account-map-page :is(h1, h2, h3, p, label, strong, small),
.app-shell.app-shell-anti .account-map-page .inspector-profile-meta span {
  color: var(--anti-ink) !important;
}

.app-shell.app-shell-anti .account-map-page :is(input, select) {
  border: 3px solid var(--anti-ink) !important;
  border-radius: 0 !important;
  background: var(--anti-paper) !important;
  box-shadow: 4px 4px 0 var(--anti-blue) !important;
  color: var(--anti-ink) !important;
}

.app-shell.app-shell-anti .account-map-page button:not(.inspector-account-row):not(.inspector-close) {
  border: 3px solid var(--anti-ink) !important;
  border-radius: 0.2rem !important;
  box-shadow: 4px 4px 0 var(--anti-ink) !important;
  color: var(--anti-ink) !important;
  font-weight: 950;
}

.app-shell.app-shell-anti .account-map-page button:not(.inspector-account-row):not(.inspector-close):hover {
  background: var(--anti-green) !important;
  transform: rotate(-1deg) translate(-1px, -1px);
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

.dark .app-shell.app-shell-anti .account-map-page :is(.account-map-toolbar, .account-map-filter-card, .account-map-empty-state, .account-map-inspector, .inspector-profile-card) {
  background: linear-gradient(135deg, #101827, #0b1020) !important;
  box-shadow: 6px 6px 0 #334155 !important;
}

.dark .app-shell.app-shell-anti .account-map-page :is(.account-map-stat-card, .account-map-stat-metrics span, .inspector-section, .inspector-metric, .inspector-account-row, .inspector-alert, .quota-stat-card, .quota-empty-state, .inspector-account-quota) {
  background: #111827 !important;
  box-shadow: 3px 3px 0 #334155 !important;
}

.dark .app-shell.app-shell-anti .account-map-page .account-map-toolbar {
  background:
    linear-gradient(90deg, rgb(250 204 21 / 0.22), transparent 46%),
    linear-gradient(135deg, #111827, #0b1020) !important;
}

.app-shell.app-shell-anti .account-map-page .account-map-platform-row {
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

.dark .app-shell.app-shell-anti .account-map-page .account-map-platform-row {
  background: #111827 !important;
  box-shadow: 2px 2px 0 #334155 !important;
}
</style>
