<template>
  <section class="gateway-signal-card">
    <div class="gateway-signal-heading">
      <div>
        <p class="gateway-signal-kicker">Gateway Signals</p>
        <h3>网关策略预检</h3>
      </div>
      <span class="gateway-signal-badge">当前筛选 · {{ logs.length }} 条可见样本</span>
    </div>

    <div class="gateway-signal-grid">
      <article v-for="signal in signals" :key="signal.title" class="gateway-signal-item" :class="`gateway-signal-${signal.tone}`">
        <div class="gateway-signal-item-head">
          <span>{{ signal.label }}</span>
          <strong>{{ signal.value }}</strong>
        </div>
        <h4>{{ signal.title }}</h4>
        <p>{{ signal.description }}</p>
      </article>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { AdminUsageLog, EndpointStat } from '@/types'
import type { AdminUsageStatsResponse } from '@/api/admin/usage'

type SignalTone = 'blue' | 'emerald' | 'amber' | 'violet'

type GatewaySignal = {
  label: string
  value: string
  title: string
  description: string
  tone: SignalTone
}

const props = defineProps<{
  logs: AdminUsageLog[]
  stats: AdminUsageStatsResponse | null
  endpointStats?: EndpointStat[]
}>()

const formatTokens = (value: number | null | undefined): string => {
  const n = value || 0
  if (n >= 1_000_000_000) return `${(n / 1_000_000_000).toFixed(2)}B`
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(2)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(2)}K`
  return n.toLocaleString()
}

const formatDuration = (ms: number | null | undefined): string => {
  if (ms == null) return '-'
  if (ms < 1000) return `${Math.round(ms)}ms`
  return `${(ms / 1000).toFixed(2)}s`
}

const formatPercent = (value: number): string => `${(value * 100).toFixed(1)}%`

const normalizeClient = (userAgent: string | null | undefined): string => {
  const ua = (userAgent || '').toLowerCase()
  if (!ua) return 'Unknown'
  if (ua.includes('claude-code')) return 'Claude Code'
  if (ua.includes('codex')) return 'Codex CLI'
  if (ua.includes('cline')) return 'Cline'
  if (ua.includes('cherry')) return 'Cherry Studio'
  if (ua.includes('cursor')) return 'Cursor'
  if (ua.includes('python')) return 'Python SDK'
  if (ua.includes('node') || ua.includes('axios')) return 'Node Client'
  return 'Other'
}

const slowestLog = computed(() => {
  return [...props.logs].sort((a, b) => (b.duration_ms || 0) - (a.duration_ms || 0))[0] || null
})

const mappedCount = computed(() => {
  return props.logs.filter((log) => {
    if (log.model_mapping_chain?.includes('→')) return true
    return Boolean(log.upstream_model && log.upstream_model !== log.model)
  }).length
})

const clientFamilies = computed(() => new Set(props.logs.map(log => normalizeClient(log.user_agent))).size)

const topEndpoint = computed(() => {
  const endpoints = props.endpointStats || []
  return endpoints[0]?.endpoint || ''
})

const signals = computed<GatewaySignal[]>(() => {
  const totalTokens = props.stats?.total_tokens || 0
  const cacheTokens = props.stats?.total_cache_tokens || 0
  const cacheShare = totalTokens > 0 ? cacheTokens / totalTokens : 0
  const slowest = slowestLog.value

  return [
    {
      label: 'CACHE',
      value: totalTokens > 0 ? formatPercent(cacheShare) : '-',
      title: cacheShare >= 0.5 ? '缓存命中健康' : '缓存仍有优化空间',
      description: `本范围缓存 Token ${formatTokens(cacheTokens)}。低缓存时优先检查长上下文复用、系统提示稳定性和客户端请求模式。`,
      tone: cacheShare >= 0.5 ? 'emerald' : 'amber'
    },
    {
      label: 'LATENCY',
      value: slowest ? formatDuration(slowest.duration_ms) : '-',
      title: '最慢可见请求',
      description: slowest
        ? `${slowest.model || '-'} · ${slowest.request_id || '无 request_id'}`
        : '当前页暂无可用于定位的请求样本。',
      tone: slowest && slowest.duration_ms > 30_000 ? 'amber' : 'blue'
    },
    {
      label: 'MODEL MAP',
      value: `${mappedCount.value}/${props.logs.length || 0}`,
      title: '模型映射发生次数',
      description: mappedCount.value > 0
        ? '当前页存在请求模型到上游模型的映射，可用 Trace 抽屉核对实际路由。'
        : '当前页未发现明显模型映射，路由链路相对直接。',
      tone: mappedCount.value > 0 ? 'violet' : 'blue'
    },
    {
      label: 'CLIENTS',
      value: String(clientFamilies.value || 0),
      title: '客户端族群',
      description: `可见样本覆盖 ${clientFamilies.value || 0} 类客户端。客户端过多时，建议按 User-Agent 做兼容与指纹策略核对。`,
      tone: clientFamilies.value > 3 ? 'amber' : 'emerald'
    },
    {
      label: 'UPSTREAM',
      value: topEndpoint.value ? '已识别' : '-',
      title: '主上游端点',
      description: topEndpoint.value || '当前筛选下暂无上游端点分布数据。',
      tone: 'blue'
    }
  ]
})
</script>

<style scoped>
.gateway-signal-card {
  border: 1px solid rgba(148, 163, 184, 0.18);
  border-radius: 1.25rem;
  background:
    radial-gradient(circle at 14% 0%, rgba(14, 165, 233, 0.11), transparent 26rem),
    linear-gradient(135deg, rgba(15, 23, 42, 0.055), rgba(15, 23, 42, 0.025));
  padding: 1rem;
}

:global(.dark) .gateway-signal-card {
  background:
    radial-gradient(circle at 14% 0%, rgba(14, 165, 233, 0.12), transparent 26rem),
    linear-gradient(135deg, rgba(15, 23, 42, 0.86), rgba(2, 6, 23, 0.72));
}

.gateway-signal-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  margin-bottom: 0.9rem;
}

.gateway-signal-kicker {
  font-size: 0.68rem;
  font-weight: 800;
  letter-spacing: 0.18em;
  text-transform: uppercase;
  color: #0284c7;
}

:global(.dark) .gateway-signal-kicker {
  color: #38bdf8;
}

.gateway-signal-heading h3 {
  margin-top: 0.18rem;
  font-size: 0.95rem;
  font-weight: 800;
  color: #0f172a;
}

:global(.dark) .gateway-signal-heading h3 {
  color: #f8fafc;
}

.gateway-signal-badge {
  border-radius: 999px;
  border: 1px solid rgba(14, 165, 233, 0.24);
  background: rgba(14, 165, 233, 0.08);
  padding: 0.35rem 0.6rem;
  font-size: 0.7rem;
  font-weight: 700;
  color: #0369a1;
}

:global(.dark) .gateway-signal-badge {
  color: #7dd3fc;
}

.gateway-signal-grid {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 0.75rem;
}

.gateway-signal-item {
  min-width: 0;
  border-radius: 1rem;
  border: 1px solid rgba(148, 163, 184, 0.18);
  background: rgba(255, 255, 255, 0.62);
  padding: 0.85rem;
}

:global(.dark) .gateway-signal-item {
  background: rgba(15, 23, 42, 0.62);
}

.gateway-signal-item-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}

.gateway-signal-item-head span {
  font-size: 0.64rem;
  font-weight: 800;
  letter-spacing: 0.14em;
  color: #64748b;
}

.gateway-signal-item-head strong {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 0.86rem;
  color: #0f172a;
}

:global(.dark) .gateway-signal-item-head strong {
  color: #f8fafc;
}

.gateway-signal-item h4 {
  margin-top: 0.55rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 0.82rem;
  font-weight: 800;
  color: #111827;
}

:global(.dark) .gateway-signal-item h4 {
  color: #e5e7eb;
}

.gateway-signal-item p {
  margin-top: 0.35rem;
  display: -webkit-box;
  min-height: 2.4rem;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  font-size: 0.72rem;
  line-height: 1.2rem;
  color: #64748b;
}

:global(.dark) .gateway-signal-item p {
  color: #94a3b8;
}

.gateway-signal-blue {
  box-shadow: inset 0 1px 0 rgba(59, 130, 246, 0.12);
}

.gateway-signal-emerald {
  box-shadow: inset 0 1px 0 rgba(16, 185, 129, 0.14);
}

.gateway-signal-amber {
  box-shadow: inset 0 1px 0 rgba(245, 158, 11, 0.2);
}

.gateway-signal-violet {
  box-shadow: inset 0 1px 0 rgba(139, 92, 246, 0.16);
}

@media (max-width: 1280px) {
  .gateway-signal-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 640px) {
  .gateway-signal-heading {
    flex-direction: column;
  }

  .gateway-signal-grid {
    grid-template-columns: 1fr;
  }
}
</style>
