<template>
  <Teleport to="body">
    <Transition name="request-facts">
      <div v-if="show" class="fixed inset-0 z-[10000]" @keydown.esc="close">
        <button class="absolute inset-0 bg-slate-950/60 backdrop-blur-sm" type="button" aria-label="关闭事实日志" @click="close" />
        <aside class="absolute bottom-0 right-0 top-0 flex w-[min(58rem,calc(100vw-1rem))] flex-col overflow-hidden border-l border-slate-700/70 bg-slate-950 text-slate-100 shadow-2xl" role="dialog" aria-modal="true">
          <header class="border-b border-slate-800 px-5 py-4">
            <div class="flex items-start justify-between gap-4">
              <div class="min-w-0">
                <p class="text-[11px] font-black uppercase tracking-[0.22em] text-sky-300">Request Facts</p>
                <h2 class="mt-1 truncate text-lg font-black text-white">请求事实日志</h2>
                <p class="mt-1 break-all font-mono text-xs text-slate-400">{{ requestLabel }}</p>
              </div>
              <button class="rounded-lg border border-slate-700 px-2.5 py-1 text-lg leading-none text-slate-300 hover:bg-slate-800" type="button" @click="close">×</button>
            </div>
          </header>

          <div class="min-h-0 flex-1 overflow-y-auto p-5">
            <div v-if="loading" class="rounded-2xl border border-slate-800 bg-slate-900/70 p-8 text-center text-sm text-slate-400">加载事实日志...</div>
            <div v-else class="space-y-4">
              <section class="grid gap-3 md:grid-cols-4">
                <FactTile label="平台" :value="summary.platform" />
                <FactTile label="模型" :value="summary.model" />
                <FactTile label="账号" :value="summary.account" />
                <FactTile label="状态" :value="summary.status" />
              </section>

              <section class="rounded-2xl border border-slate-800 bg-slate-900/70 p-4">
                <div class="mb-3 flex items-center justify-between gap-3">
                  <h3 class="text-sm font-black text-white">时间线</h3>
                  <button class="rounded-lg border border-slate-700 px-2.5 py-1 text-xs font-bold text-slate-300 hover:bg-slate-800" type="button" @click="copyBundle">复制证据包</button>
                </div>
                <div v-if="timeline.length === 0" class="text-sm text-slate-500">没有关联到请求事件。</div>
                <div v-else class="space-y-2">
                  <div v-for="item in timeline" :key="item.key" class="rounded-xl border border-slate-800 bg-slate-950/70 p-3">
                    <div class="flex flex-wrap items-center gap-2 text-xs">
                      <span class="font-mono text-slate-500">{{ formatTime(item.time) }}</span>
                      <span class="rounded-full px-2 py-0.5 font-bold" :class="kindClass(item.kind)">{{ item.kind }}</span>
                      <span class="font-semibold text-slate-100">{{ item.title }}</span>
                    </div>
                    <p v-if="item.detail" class="mt-1 whitespace-pre-wrap break-words text-xs text-slate-300">{{ item.detail }}</p>
                  </div>
                </div>
              </section>

              <section class="grid gap-4 lg:grid-cols-2">
                <FactPanel title="请求/调度">
                  <FactRow label="request_id" :value="requestId || '-'" mono />
                  <FactRow label="client_request_id" :value="clientRequestId || '-'" mono />
                  <FactRow label="入口端点" :value="firstError?.inbound_endpoint || firstDebug?.inbound_endpoint || '-'" />
                  <FactRow label="上游端点" :value="firstError?.upstream_endpoint || firstDebug?.upstream_endpoint || '-'" />
                  <FactRow label="耗时" :value="formatMs(firstRequest?.duration_ms ?? firstError?.response_latency_ms)" />
                  <FactRow label="TTFT" :value="formatMs(firstError?.time_to_first_token_ms ?? firstDebug?.time_to_first_token_ms)" />
                </FactPanel>

                <FactPanel title="错误原文">
                  <pre class="max-h-80 overflow-auto whitespace-pre-wrap break-words rounded-xl bg-slate-950 p-3 text-xs text-slate-200">{{ primaryPayload }}</pre>
                </FactPanel>
              </section>

              <FactPanel title="系统日志">
                <div v-if="systemLogs.length === 0" class="text-sm text-slate-500">无关联系统日志。</div>
                <div v-else class="space-y-2">
                  <details v-for="log in systemLogs" :key="log.id" class="rounded-xl border border-slate-800 bg-slate-950/70 p-3">
                    <summary class="cursor-pointer text-xs text-slate-300">
                      <span class="font-mono text-slate-500">{{ formatTime(log.created_at) }}</span>
                      <span class="ml-2 font-bold uppercase" :class="logLevelClass(log.level)">{{ log.level }}</span>
                      <span class="ml-2">{{ log.message || log.component }}</span>
                    </summary>
                    <pre class="mt-3 max-h-72 overflow-auto whitespace-pre-wrap break-words rounded-lg bg-slate-900 p-3 text-xs text-slate-200">{{ pretty(log) }}</pre>
                  </details>
                </div>
              </FactPanel>

              <FactPanel title="Debug Trace">
                <div v-if="debugTraces.length === 0" class="text-sm text-slate-500">无内存调试 Trace，可能已过 TTL。</div>
                <div v-else class="space-y-2">
                  <details v-for="trace in debugTraces" :key="trace.id" class="rounded-xl border border-slate-800 bg-slate-950/70 p-3">
                    <summary class="cursor-pointer text-xs text-slate-300">
                      <span class="font-mono text-slate-500">{{ formatTime(trace.created_at) }}</span>
                      <span class="ml-2">{{ trace.reason_code || trace.error_code || trace.status_code || 'trace' }}</span>
                    </summary>
                    <pre class="mt-3 max-h-96 overflow-auto whitespace-pre-wrap break-words rounded-lg bg-slate-900 p-3 text-xs text-slate-200">{{ pretty(trace) }}</pre>
                  </details>
                </div>
              </FactPanel>
            </div>
          </div>
        </aside>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, ref, watch } from 'vue'
import { opsAPI, type OpsDebugTrace, type OpsErrorDetail, type OpsRequestDetail, type OpsSystemLog } from '@/api/admin/ops'
import { formatDateTime } from '@/utils/format'

const props = defineProps<{
  show: boolean
  requestId?: string
  clientRequestId?: string
  errorId?: number | null
}>()

const emit = defineEmits<{ close: [] }>()

const loading = ref(false)
const requests = ref<OpsRequestDetail[]>([])
const requestErrors = ref<OpsErrorDetail[]>([])
const upstreamErrors = ref<OpsErrorDetail[]>([])
const systemLogs = ref<OpsSystemLog[]>([])
const debugTraces = ref<OpsDebugTrace[]>([])

const requestId = computed(() => String(props.requestId || '').trim())
const clientRequestId = computed(() => String(props.clientRequestId || '').trim())
const firstRequest = computed(() => requests.value[0] || null)
const firstError = computed(() => requestErrors.value[0] || upstreamErrors.value[0] || null)
const firstDebug = computed(() => debugTraces.value[0] || null)

const requestLabel = computed(() => requestId.value || clientRequestId.value || '未指定 request_id')
const summary = computed(() => ({
  platform: firstRequest.value?.platform || firstError.value?.platform || firstDebug.value?.platform || '-',
  model: firstRequest.value?.model || firstError.value?.upstream_model || firstError.value?.model || firstDebug.value?.model || '-',
  account: firstError.value?.account_name || (firstError.value?.account_id ? `#${firstError.value.account_id}` : firstDebug.value?.account_id ? `#${firstDebug.value.account_id}` : '-'),
  status: String(firstRequest.value?.status_code ?? firstError.value?.status_code ?? firstDebug.value?.status_code ?? '-')
}))

type TimelineItem = { key: string; time: string; kind: string; title: string; detail: string }
const timeline = computed<TimelineItem[]>(() => {
  const items: TimelineItem[] = []
  for (const row of requests.value) {
    items.push({ key: `req-${row.request_id}-${row.created_at}`, time: row.created_at, kind: row.kind, title: `${row.platform || '-'} ${row.model || '-'}`, detail: `status=${row.status_code ?? '-'} duration=${formatMs(row.duration_ms)} error_id=${row.error_id ?? '-'}` })
  }
  for (const row of requestErrors.value) {
    items.push({ key: `err-${row.id}`, time: row.created_at, kind: 'request-error', title: row.message || row.type || 'request error', detail: `${row.phase || '-'} ${row.error_owner || '-'} status=${row.status_code}` })
  }
  for (const row of upstreamErrors.value) {
    items.push({ key: `up-${row.id}`, time: row.created_at, kind: 'upstream-error', title: row.message || row.upstream_error_message || 'upstream error', detail: `${row.account_name || row.account_id || '-'} status=${row.status_code}` })
  }
  for (const row of debugTraces.value) {
    items.push({ key: `dbg-${row.id}`, time: row.created_at, kind: 'debug', title: row.reason_code || row.error_code || row.path || 'debug trace', detail: row.reason_hint || row.error_message || '' })
  }
  return items.sort((a, b) => new Date(a.time).getTime() - new Date(b.time).getTime())
})

const primaryPayload = computed(() => {
  const err = firstError.value
  const trace = firstDebug.value
  return prettyText(err?.upstream_error_detail || err?.upstream_errors || err?.error_body || trace?.response_body_preview || trace?.request_body_preview_json || '')
})

const FactTile = defineComponent({
  props: { label: { type: String, required: true }, value: { type: String, required: true } },
  setup(p) {
    return () => h('div', { class: 'rounded-2xl border border-slate-800 bg-slate-900/70 p-3' }, [
      h('div', { class: 'text-[11px] font-bold uppercase text-slate-500' }, p.label),
      h('div', { class: 'mt-1 truncate font-mono text-sm font-bold text-slate-100', title: p.value }, p.value)
    ])
  }
})

const FactPanel = defineComponent({
  props: { title: { type: String, required: true } },
  setup(p, ctx) {
    return () => h('section', { class: 'rounded-2xl border border-slate-800 bg-slate-900/70 p-4' }, [
      h('h3', { class: 'mb-3 text-sm font-black text-white' }, p.title),
      ctx.slots.default?.()
    ])
  }
})

const FactRow = defineComponent({
  props: { label: { type: String, required: true }, value: { type: String, required: true }, mono: { type: Boolean, default: false } },
  setup(p) {
    return () => h('div', { class: 'grid grid-cols-[8rem_1fr] gap-3 border-t border-slate-800 py-2 first:border-t-0' }, [
      h('span', { class: 'text-xs text-slate-500' }, p.label),
      h('span', { class: ['break-all text-xs text-slate-200', p.mono ? 'font-mono' : ''], title: p.value }, p.value)
    ])
  }
})

function close() {
  emit('close')
}

async function loadFacts() {
  if (!props.show) return
  const rid = requestId.value
  const crid = clientRequestId.value
  if (!rid && !crid && !props.errorId) return

  loading.value = true
  requests.value = []
  requestErrors.value = []
  upstreamErrors.value = []
  systemLogs.value = []
  debugTraces.value = []

  try {
    const jobs: Promise<void>[] = []
    if (rid) {
      jobs.push(opsAPI.listRequestDetails({ request_id: rid, time_range: '24h', kind: 'all', page: 1, page_size: 20 }).then(res => { requests.value = res.items || [] }))
      jobs.push(opsAPI.listDebugTraces({ request_id: rid, limit: 20, only_errors: false }).then(res => { debugTraces.value = res.items || [] }))
    }
    if (rid || crid) {
      jobs.push(opsAPI.listSystemLogs({ request_id: rid || undefined, client_request_id: rid ? undefined : crid || undefined, time_range: '24h', page: 1, page_size: 50 }).then(res => { systemLogs.value = res.items || [] }))
    }
    if (props.errorId) {
      jobs.push(opsAPI.getRequestErrorDetail(props.errorId).then(res => { requestErrors.value = [res] }))
      jobs.push(opsAPI.listRequestErrorUpstreamErrors(props.errorId, { page: 1, page_size: 100, view: 'all' }, { include_detail: true }).then(res => { upstreamErrors.value = res.items || [] }))
    }
    await Promise.allSettled(jobs)
  } finally {
    loading.value = false
  }
}

watch(() => [props.show, props.requestId, props.clientRequestId, props.errorId] as const, loadFacts, { immediate: true })

function formatTime(value?: string): string {
  if (!value) return '-'
  return formatDateTime(value)
}

function formatMs(value: number | null | undefined): string {
  if (value == null) return '-'
  return value < 1000 ? `${Math.round(value)} ms` : `${(value / 1000).toFixed(2)} s`
}

function pretty(value: unknown): string {
  return JSON.stringify(value, null, 2)
}

function prettyText(raw: string): string {
  if (!raw) return 'N/A'
  try { return JSON.stringify(JSON.parse(raw), null, 2) } catch { return raw }
}

function kindClass(kind: string): string {
  if (kind.includes('error')) return 'bg-red-500/15 text-red-300'
  if (kind === 'success') return 'bg-emerald-500/15 text-emerald-300'
  return 'bg-sky-500/15 text-sky-300'
}

function logLevelClass(level: string): string {
  const v = String(level || '').toLowerCase()
  if (v === 'error' || v === 'fatal') return 'text-red-300'
  if (v === 'warn') return 'text-amber-300'
  return 'text-sky-300'
}

async function copyBundle() {
  if (typeof navigator === 'undefined' || !navigator.clipboard) return
  await navigator.clipboard.writeText(pretty({ request_id: requestId.value, client_request_id: clientRequestId.value, summary: summary.value, timeline: timeline.value }))
}
</script>
