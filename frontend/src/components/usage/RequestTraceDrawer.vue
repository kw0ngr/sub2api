<template>
  <Teleport to="body">
    <Transition name="request-trace">
      <div v-if="show && log" class="request-trace-shell" @keydown.esc="emit('close')">
        <button class="request-trace-backdrop" type="button" aria-label="Close trace drawer" @click="emit('close')" />
        <aside class="request-trace-panel" role="dialog" aria-modal="true" aria-labelledby="request-trace-title">
          <header class="request-trace-header">
            <div class="min-w-0">
              <p class="request-trace-kicker">Gateway Trace</p>
              <h2 id="request-trace-title" class="request-trace-title">
                {{ log.model || '-' }}
              </h2>
              <p class="request-trace-subtitle">
                {{ formatDateTime(log.created_at) }} · {{ requestTypeLabel }}
              </p>
            </div>
            <button class="request-trace-close" type="button" aria-label="Close trace drawer" @click="emit('close')">
              ×
            </button>
          </header>

          <section class="request-trace-hero">
            <div>
              <span class="request-trace-label">总 Token</span>
              <strong>{{ formatTokens(totalTokens) }}</strong>
            </div>
            <div>
              <span class="request-trace-label">实际成本</span>
              <strong>${{ formatMoney(log.actual_cost) }}</strong>
            </div>
            <div>
              <span class="request-trace-label">耗时</span>
              <strong>{{ formatDuration(log.duration_ms) }}</strong>
            </div>
          </section>

          <section class="request-trace-section">
            <div class="request-trace-section-title">
              <h3>路由链路</h3>
              <span>{{ admin ? 'Admin' : 'User' }}</span>
            </div>
            <div class="request-trace-route">
              <TraceRow label="入口端点" :value="log.inbound_endpoint || '-'" />
              <TraceRow v-if="admin" label="上游端点" :value="adminLog.upstream_endpoint || '-'" />
              <TraceRow v-if="admin" label="请求模型" :value="log.model || '-'" />
              <TraceRow v-if="admin && adminLog.upstream_model" label="上游模型" :value="adminLog.upstream_model" />
              <TraceRow v-if="admin && adminLog.model_mapping_chain" label="模型映射" :value="adminLog.model_mapping_chain" />
              <TraceRow label="API Key" :value="log.api_key?.name || `#${log.api_key_id || '-'}`" />
              <TraceRow v-if="admin" label="账号" :value="adminLog.account?.name || (adminLog.account_id ? `#${adminLog.account_id}` : '-')" />
              <TraceRow v-if="admin" label="用户" :value="log.user?.email || (log.user_id ? `#${log.user_id}` : '-')" />
              <TraceRow v-if="log.project_label || log.project_key" label="项目" :value="log.project_label || log.project_key || '-'" />
            </div>
          </section>

          <section class="request-trace-section">
            <div class="request-trace-section-title">
              <h3>Token / 成本拆解</h3>
              <span>可用于定位缓存与倍率</span>
            </div>
            <div class="request-trace-meter">
              <div
                v-for="part in tokenParts"
                :key="part.label"
                class="request-trace-meter-segment"
                :style="{ width: `${part.width}%`, background: part.color }"
                :title="`${part.label}: ${formatTokens(part.value)}`"
              />
            </div>
            <div class="request-trace-grid">
              <TraceMetric label="输入" :value="formatTokens(log.input_tokens)" :sub-value="`$${formatMoney(log.input_cost)}`" tone="blue" />
              <TraceMetric label="输出" :value="formatTokens(log.output_tokens)" :sub-value="`$${formatMoney(log.output_cost)}`" tone="violet" />
              <TraceMetric label="缓存写入" :value="formatTokens(log.cache_creation_tokens)" :sub-value="`$${formatMoney(log.cache_creation_cost)}`" tone="amber" />
              <TraceMetric label="缓存读取" :value="formatTokens(log.cache_read_tokens)" :sub-value="`$${formatMoney(log.cache_read_cost)}`" tone="emerald" />
            </div>
            <div class="request-trace-route mt-3">
              <TraceRow label="用户倍率" :value="`${formatMultiplier(log.rate_multiplier || 1)}x`" />
              <TraceRow v-if="admin" label="账号倍率" :value="`${formatMultiplier(adminLog.account_rate_multiplier ?? 1)}x`" />
              <TraceRow v-if="log.cache_ttl_overridden" label="缓存 TTL" value="已被请求覆盖" />
              <TraceRow v-if="log.service_tier" label="服务层级" :value="log.service_tier" />
            </div>
          </section>

          <section class="request-trace-section">
            <div class="request-trace-section-title">
              <h3>客户端与定位</h3>
              <span>排查指纹 / 客户端异常</span>
            </div>
            <div class="request-trace-route">
              <TraceRow label="Request ID" :value="log.request_id || '-'" monospace />
              <TraceRow v-if="admin" label="IP" :value="adminLog.ip_address || '-'" monospace />
              <TraceRow label="User-Agent" :value="log.user_agent || '-'" />
              <TraceRow label="首 Token" :value="log.first_token_ms == null ? '-' : formatDuration(log.first_token_ms)" />
              <TraceRow label="创建时间" :value="formatDateTime(log.created_at)" />
            </div>
          </section>

          <footer class="request-trace-footer">
            <button
              type="button"
              class="request-trace-copy"
              :disabled="!log.request_id"
              @click="copyRequestId"
            >
              复制 Request ID
            </button>
            <button type="button" class="request-trace-done" @click="emit('close')">完成</button>
          </footer>
        </aside>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, defineComponent, h } from 'vue'
import { formatDateTime } from '@/utils/format'
import { formatMultiplier } from '@/utils/formatters'
import { resolveUsageRequestType } from '@/utils/usageRequestType'
import type { AdminUsageLog, UsageLog } from '@/types'

const props = withDefaults(defineProps<{
  show: boolean
  log: UsageLog | AdminUsageLog | null
  admin?: boolean
}>(), {
  admin: false
})

const emit = defineEmits<{
  close: []
}>()

const TraceRow = defineComponent({
  name: 'TraceRow',
  props: {
    label: { type: String, required: true },
    value: { type: String, required: true },
    monospace: { type: Boolean, default: false }
  },
  setup(rowProps) {
    return () => h('div', { class: 'request-trace-row' }, [
      h('span', { class: 'request-trace-row-label' }, rowProps.label),
      h('span', {
        class: ['request-trace-row-value', rowProps.monospace ? 'request-trace-row-mono' : ''],
        title: rowProps.value
      }, rowProps.value)
    ])
  }
})

const TraceMetric = defineComponent({
  name: 'TraceMetric',
  props: {
    label: { type: String, required: true },
    value: { type: String, required: true },
    subValue: { type: String, required: true },
    tone: { type: String, default: 'blue' }
  },
  setup(metricProps) {
    return () => h('div', { class: ['request-trace-metric', `request-trace-metric-${metricProps.tone}`] }, [
      h('span', metricProps.label),
      h('strong', metricProps.value),
      h('small', metricProps.subValue)
    ])
  }
})

const adminLog = computed(() => (props.log || {}) as AdminUsageLog)

const totalTokens = computed(() => {
  const log = props.log
  if (!log) return 0
  return (
    (log.input_tokens || 0) +
    (log.output_tokens || 0) +
    (log.cache_creation_tokens || 0) +
    (log.cache_read_tokens || 0)
  )
})

const requestTypeLabel = computed(() => {
  if (!props.log) return '-'
  const type = resolveUsageRequestType(props.log)
  if (type === 'ws_v2') return 'WS'
  if (type === 'stream') return 'Stream'
  if (type === 'sync') return 'Sync'
  return 'Unknown'
})

const tokenParts = computed(() => {
  const log = props.log
  const total = totalTokens.value || 1
  const parts = [
    { label: '输入', value: log?.input_tokens || 0, color: '#3b82f6' },
    { label: '输出', value: log?.output_tokens || 0, color: '#8b5cf6' },
    { label: '缓存写入', value: log?.cache_creation_tokens || 0, color: '#f59e0b' },
    { label: '缓存读取', value: log?.cache_read_tokens || 0, color: '#10b981' }
  ]
  return parts
    .filter(part => part.value > 0)
    .map(part => ({
      ...part,
      width: Math.max(4, (part.value / total) * 100)
    }))
})

const formatTokens = (value: number | null | undefined): string => {
  const n = value || 0
  if (n >= 1_000_000_000) return `${(n / 1_000_000_000).toFixed(2)}B`
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(2)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(2)}K`
  return n.toLocaleString()
}

const formatMoney = (value: number | null | undefined): string => (value || 0).toFixed(6)

const formatDuration = (ms: number | null | undefined): string => {
  if (ms == null) return '-'
  if (ms < 1000) return `${Math.round(ms)}ms`
  return `${(ms / 1000).toFixed(2)}s`
}

const copyRequestId = async () => {
  const requestId = props.log?.request_id
  if (!requestId || typeof navigator === 'undefined' || !navigator.clipboard) return
  await navigator.clipboard.writeText(requestId)
}
</script>

<style scoped>
.request-trace-shell {
  position: fixed;
  inset: 0;
  z-index: 10000;
}

.request-trace-backdrop {
  position: absolute;
  inset: 0;
  cursor: default;
  background: rgba(2, 6, 23, 0.52);
  backdrop-filter: blur(5px);
}

.request-trace-panel {
  position: absolute;
  bottom: 0;
  right: 0;
  top: 0;
  display: flex;
  width: min(31rem, calc(100vw - 1rem));
  flex-direction: column;
  overflow-y: auto;
  border-left: 1px solid rgba(148, 163, 184, 0.28);
  background:
    radial-gradient(circle at 20% 0%, rgba(59, 130, 246, 0.12), transparent 34rem),
    linear-gradient(180deg, rgba(15, 23, 42, 0.98), rgba(2, 6, 23, 0.98));
  box-shadow: -28px 0 70px rgba(2, 6, 23, 0.42);
  color: #e5e7eb;
}

.request-trace-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  padding: 1.25rem;
}

.request-trace-kicker {
  font-size: 0.68rem;
  font-weight: 800;
  letter-spacing: 0.18em;
  text-transform: uppercase;
  color: #38bdf8;
}

.request-trace-title {
  margin-top: 0.25rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 1.15rem;
  font-weight: 800;
  color: #f8fafc;
}

.request-trace-subtitle {
  margin-top: 0.25rem;
  font-size: 0.78rem;
  color: #94a3b8;
}

.request-trace-close {
  display: inline-flex;
  height: 2.25rem;
  width: 2.25rem;
  align-items: center;
  justify-content: center;
  border-radius: 999px;
  border: 1px solid rgba(148, 163, 184, 0.25);
  background: rgba(15, 23, 42, 0.72);
  color: #cbd5e1;
  font-size: 1.4rem;
  line-height: 1;
  transition: transform 160ms ease, border-color 160ms ease, background 160ms ease;
}

.request-trace-close:hover {
  transform: translateY(-1px);
  border-color: rgba(125, 211, 252, 0.45);
  background: rgba(30, 41, 59, 0.9);
}

.request-trace-hero {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0.65rem;
  padding: 0 1.25rem 1rem;
}

.request-trace-hero > div {
  min-width: 0;
  border-radius: 1rem;
  border: 1px solid rgba(148, 163, 184, 0.18);
  background: rgba(15, 23, 42, 0.66);
  padding: 0.85rem;
}

.request-trace-label,
.request-trace-hero span {
  display: block;
  font-size: 0.68rem;
  color: #94a3b8;
}

.request-trace-hero strong {
  margin-top: 0.35rem;
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 0.95rem;
  color: #f8fafc;
}

.request-trace-section {
  margin: 0 1.25rem 1rem;
  border-radius: 1.1rem;
  border: 1px solid rgba(148, 163, 184, 0.2);
  background: rgba(15, 23, 42, 0.58);
  padding: 1rem;
}

.request-trace-section-title {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 1rem;
  margin-bottom: 0.8rem;
}

.request-trace-section-title h3 {
  font-size: 0.85rem;
  font-weight: 800;
  color: #f8fafc;
}

.request-trace-section-title span {
  font-size: 0.68rem;
  color: #64748b;
}

.request-trace-route {
  display: grid;
  gap: 0.55rem;
}

:deep(.request-trace-row) {
  display: grid;
  grid-template-columns: 5.5rem minmax(0, 1fr);
  gap: 0.8rem;
  align-items: start;
}

:deep(.request-trace-row-label) {
  color: #94a3b8;
  font-size: 0.74rem;
}

:deep(.request-trace-row-value) {
  min-width: 0;
  overflow-wrap: anywhere;
  color: #e5e7eb;
  font-size: 0.78rem;
}

:deep(.request-trace-row-mono) {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace;
}

.request-trace-meter {
  display: flex;
  height: 0.65rem;
  overflow: hidden;
  border-radius: 999px;
  background: rgba(51, 65, 85, 0.85);
}

.request-trace-meter-segment {
  min-width: 0.35rem;
}

.request-trace-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.65rem;
  margin-top: 0.8rem;
}

:deep(.request-trace-metric) {
  border-radius: 0.9rem;
  border: 1px solid rgba(148, 163, 184, 0.16);
  background: rgba(2, 6, 23, 0.35);
  padding: 0.75rem;
}

:deep(.request-trace-metric span),
:deep(.request-trace-metric small) {
  display: block;
  color: #94a3b8;
  font-size: 0.68rem;
}

:deep(.request-trace-metric strong) {
  display: block;
  margin-top: 0.25rem;
  color: #f8fafc;
  font-size: 0.92rem;
}

:deep(.request-trace-metric-blue strong) {
  color: #93c5fd;
}

:deep(.request-trace-metric-violet strong) {
  color: #c4b5fd;
}

:deep(.request-trace-metric-amber strong) {
  color: #fcd34d;
}

:deep(.request-trace-metric-emerald strong) {
  color: #6ee7b7;
}

.request-trace-footer {
  position: sticky;
  bottom: 0;
  display: flex;
  gap: 0.75rem;
  margin-top: auto;
  border-top: 1px solid rgba(148, 163, 184, 0.16);
  background: rgba(2, 6, 23, 0.88);
  padding: 1rem 1.25rem;
  backdrop-filter: blur(12px);
}

.request-trace-copy,
.request-trace-done {
  flex: 1;
  border-radius: 0.85rem;
  padding: 0.72rem 0.9rem;
  font-size: 0.82rem;
  font-weight: 700;
  transition: transform 160ms ease, opacity 160ms ease, background 160ms ease;
}

.request-trace-copy {
  border: 1px solid rgba(148, 163, 184, 0.25);
  background: rgba(15, 23, 42, 0.8);
  color: #cbd5e1;
}

.request-trace-done {
  background: linear-gradient(135deg, #06b6d4, #2563eb);
  color: white;
}

.request-trace-copy:hover,
.request-trace-done:hover {
  transform: translateY(-1px);
}

.request-trace-copy:disabled {
  cursor: not-allowed;
  opacity: 0.45;
}

.request-trace-enter-active,
.request-trace-leave-active {
  transition: opacity 180ms ease;
}

.request-trace-enter-from,
.request-trace-leave-to {
  opacity: 0;
}

.request-trace-enter-active .request-trace-panel,
.request-trace-leave-active .request-trace-panel {
  transition: transform 220ms cubic-bezier(0.2, 0.8, 0.2, 1);
}

.request-trace-enter-from .request-trace-panel,
.request-trace-leave-to .request-trace-panel {
  transform: translateX(1.5rem);
}

@media (max-width: 640px) {
  .request-trace-panel {
    width: 100vw;
  }

  .request-trace-hero,
  .request-trace-grid {
    grid-template-columns: 1fr;
  }
}
</style>
