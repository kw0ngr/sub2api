<template>
  <AppLayout>
    <div class="fingerprint-page space-y-6">
      <section class="fingerprint-hero">
        <div class="min-w-0">
          <p class="fingerprint-eyebrow">Simple identity presets</p>
          <h1>{{ t('admin.fingerprintStrategy.title') }}</h1>
          <p>
            不需要理解 headers、TLS、metadata。先选一个使用模式，保存即可；高级项默认收起。
          </p>
        </div>
        <div class="fingerprint-actions">
          <button class="btn btn-secondary" :disabled="loading || saving" @click="applyRecommendedPreset">
            一键推荐
          </button>
          <button class="btn btn-secondary" :disabled="loading" @click="loadData">
            {{ t('common.refresh') }}
          </button>
          <button class="btn btn-primary" :disabled="saving || loading" @click="saveSettings">
            {{ saving ? t('common.saving') : t('common.save') }}
          </button>
        </div>
      </section>

      <section class="fingerprint-guide">
        <div class="fingerprint-guide-main">
          <span class="fingerprint-guide-icon">1</span>
          <div>
            <h2>新手推荐：稳定模式</h2>
            <p>
              适合大多数内部团队：网关统一关键客户端特征，Claude Code 相关请求更稳定，
              同时减少共享 OAuth 账号时的指纹漂移。
            </p>
          </div>
        </div>
        <div class="fingerprint-guide-result">
          <span>当前模式</span>
          <strong>{{ activePresetLabel }}</strong>
        </div>
      </section>

      <section class="grid gap-4 lg:grid-cols-4">
        <div v-for="card in summaryCards" :key="card.label" class="fingerprint-stat">
          <span>{{ card.label }}</span>
          <strong :class="card.enabled ? 'text-emerald-500' : 'text-amber-500'">
            {{ card.value }}
          </strong>
          <p>{{ card.hint }}</p>
        </div>
      </section>

      <section class="grid gap-6 xl:grid-cols-[minmax(0,0.9fr)_minmax(0,1.1fr)]">
        <div class="fingerprint-panel">
          <div class="mb-5">
            <h2>选择工作模式</h2>
            <p>不知道选什么就用“稳定模式”。出问题时再切到“兼容模式”排查。</p>
          </div>

          <div class="space-y-3">
            <button
              v-for="preset in presets"
              :key="preset.id"
              type="button"
              class="fingerprint-preset-card"
              :class="{ active: activePreset === preset.id }"
              @click="applyPreset(preset.id)"
            >
              <span class="fingerprint-preset-mark" :class="preset.tone">{{ preset.badge }}</span>
              <span class="min-w-0">
                <strong>{{ preset.title }}</strong>
                <small>{{ preset.description }}</small>
              </span>
            </button>
          </div>
        </div>

        <div class="fingerprint-panel">
          <div class="mb-5 flex flex-wrap items-start justify-between gap-3">
            <div>
              <h2>保存后会发生什么</h2>
              <p>把专家术语翻译成人话，方便团队成员知道这些开关的影响。</p>
            </div>
            <span class="fingerprint-badge">{{ profiles.length }} 个 TLS 模板</span>
          </div>

          <div class="fingerprint-outcome-list">
            <div v-for="item in outcomeItems" :key="item.title" class="fingerprint-outcome">
              <span :class="['fingerprint-outcome-dot', item.tone]" />
              <div>
                <strong>{{ item.title }}</strong>
                <p>{{ item.description }}</p>
              </div>
            </div>
          </div>
        </div>
      </section>

      <section class="fingerprint-panel">
        <div class="mb-5">
          <h2>工具适配说明</h2>
          <p>这里只做总览，不要求新手逐项配置。实际策略由上面的工作模式统一控制。</p>
        </div>

        <div class="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
          <article v-for="strategy in strategies" :key="strategy.name" class="fingerprint-strategy-card">
            <div class="mb-3 flex items-start justify-between gap-3">
              <div>
                <h3>{{ strategy.name }}</h3>
                <p>{{ strategy.scope }}</p>
              </div>
              <span :class="['fingerprint-status', strategy.tone]">{{ strategy.status }}</span>
            </div>
            <ul>
              <li v-for="item in strategy.items" :key="item">{{ item }}</li>
            </ul>
          </article>
        </div>
      </section>

      <section class="grid gap-6 xl:grid-cols-[minmax(0,0.95fr)_minmax(0,1.05fr)]">
        <div class="fingerprint-panel">
          <div class="mb-5 flex flex-wrap items-start justify-between gap-3">
            <div>
              <h2>推荐配置清单</h2>
              <p>不追求玄学参数，核心是“少漂移、先学习、再固定”。</p>
            </div>
            <span class="fingerprint-badge">小白照抄</span>
          </div>

          <div class="fingerprint-check-list">
            <div v-for="item in recommendedSetup" :key="item.title" class="fingerprint-check-item">
              <span class="fingerprint-check">{{ item.mark }}</span>
              <div>
                <strong>{{ item.title }}</strong>
                <p>{{ item.description }}</p>
              </div>
            </div>
          </div>
        </div>

        <div class="fingerprint-panel">
          <div class="mb-5 flex flex-wrap items-start justify-between gap-3">
            <div>
              <h2>抓取真实 Claude Code 指纹</h2>
              <p>HTTP 指纹会自动学习；TLS 指纹需要用采集器导入模板。</p>
            </div>
            <span class="fingerprint-badge">真实优先</span>
          </div>

          <div class="fingerprint-capture-flow">
            <div v-for="step in captureFlow" :key="step.step" class="fingerprint-capture-step">
              <span class="fingerprint-step-number">{{ step.step }}</span>
              <div>
                <strong>{{ step.title }}</strong>
                <p>{{ step.description }}</p>
              </div>
            </div>
          </div>

          <div class="fingerprint-command">
            <div>
              <span>Claude Code 预热命令</span>
              <code>{{ captureCommand }}</code>
            </div>
            <button class="btn btn-secondary" type="button" @click="copyCaptureCommand">
              复制
            </button>
          </div>

          <div class="mt-4 flex flex-wrap gap-3">
            <a
              class="fingerprint-link-button"
              href="https://tls.sub2api.org"
              target="_blank"
              rel="noopener noreferrer"
            >
              打开 TLS 采集器
            </a>
            <button class="btn btn-secondary" type="button" @click="showTLSModal = true">
              粘贴/管理 TLS 模板
            </button>
          </div>
        </div>
      </section>

      <section class="fingerprint-panel">
        <div class="mb-5 flex flex-wrap items-start justify-between gap-3">
          <div>
            <h2>已捕获 HTTP 指纹样本库</h2>
            <p>团队成员用真实 Claude Code 走一次网关后，这里会自动出现可复用样本；只保存非凭据 headers，不保存 Auth、Cookie 或 API Key。</p>
          </div>
          <span class="fingerprint-badge">{{ httpFingerprintProfiles.length }} 个样本</span>
        </div>

        <div class="fingerprint-library-toolbar">
          <label class="fingerprint-field min-w-0 flex-1">
            <span>当前使用</span>
            <select v-model="activeHTTPFingerprintID" class="input">
              <option value="">自动学习（按上游账号缓存）</option>
              <option v-for="profile in httpFingerprintProfiles" :key="profile.id" :value="profile.id">
                {{ profile.name }}
              </option>
            </select>
          </label>
          <div class="fingerprint-library-actions">
            <button class="btn btn-secondary" type="button" :disabled="savingHTTPFingerprint" @click="clearActiveHTTPFingerprint">
              恢复自动
            </button>
            <button class="btn btn-primary" type="button" :disabled="savingHTTPFingerprint" @click="saveActiveHTTPFingerprint">
              {{ savingHTTPFingerprint ? '保存中' : '应用选择' }}
            </button>
          </div>
        </div>

        <div v-if="httpFingerprintProfiles.length === 0" class="fingerprint-empty">
          暂无样本。让任意团队成员用真实 Claude Code 通过网关调用一次，系统会自动保存 User-Agent、X-App、Anthropic-* 和 X-Stainless-* 等非凭据特征。
        </div>
        <div v-else class="mt-4 grid gap-3 lg:grid-cols-2">
          <article
            v-for="profile in httpFingerprintProfiles"
            :key="profile.id"
            class="fingerprint-http-card"
            :class="{ active: profile.id === activeHTTPFingerprintID }"
          >
            <div class="mb-3 flex flex-wrap items-start justify-between gap-3">
              <div class="min-w-0">
                <h3>{{ profile.name }}</h3>
                <p>{{ profile.description || '自动捕获样本' }}</p>
              </div>
              <span class="fingerprint-status good" v-if="profile.id === activeHTTPFingerprintID">使用中</span>
            </div>
            <dl class="fingerprint-http-meta">
              <div>
                <dt>完整度</dt>
                <dd>{{ profile.completeness_score || 0 }}%</dd>
              </div>
              <div>
                <dt>User-Agent</dt>
                <dd>{{ profile.user_agent }}</dd>
              </div>
              <div>
                <dt>App / Version</dt>
                <dd>{{ profile.x_app || '-' }} / {{ profile.anthropic_version || '-' }}</dd>
              </div>
              <div>
                <dt>Beta</dt>
                <dd>{{ formatBetaTokens(profile.anthropic_beta) }}</dd>
              </div>
              <div>
                <dt>Runtime</dt>
                <dd>{{ profile.stainless_runtime || '-' }} {{ profile.stainless_runtime_version || '' }}</dd>
              </div>
              <div>
                <dt>OS / Arch</dt>
                <dd>{{ profile.stainless_os || '-' }} / {{ profile.stainless_arch || '-' }}</dd>
              </div>
              <div>
                <dt>Retry / Timeout</dt>
                <dd>{{ profile.stainless_retry_count || '-' }} / {{ profile.stainless_timeout || '-' }}</dd>
              </div>
              <div>
                <dt>最近捕获</dt>
                <dd>{{ formatFingerprintTime(profile.last_seen_at) }} · {{ profile.seen_count }} 次</dd>
              </div>
            </dl>
            <div class="mt-3 flex flex-wrap gap-2">
              <button class="btn btn-secondary btn-sm" type="button" @click="selectHTTPFingerprint(profile.id)">
                选择
              </button>
              <button class="btn btn-secondary btn-sm" type="button" @click="deleteHTTPFingerprint(profile.id)">
                删除
              </button>
            </div>
          </article>
        </div>
      </section>

      <section class="grid gap-6 xl:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
        <div class="fingerprint-panel fingerprint-drift-panel">
          <div class="mb-5 flex flex-wrap items-start justify-between gap-3">
            <div>
              <h2>Outgoing 指纹巡检</h2>
              <p>展示最近一次真实转发前的最终 HTTP 指纹，直接核对样本是否生效。</p>
            </div>
            <span :class="['fingerprint-status', driftStatusTone]">
              {{ driftStatusLabel }} · {{ driftStatus?.score || 0 }}%
            </span>
          </div>

          <div class="fingerprint-drift-cards">
            <div v-for="item in driftChecklist" :key="item.label" class="fingerprint-drift-card" :class="{ ok: item.ok }">
              <span>{{ item.label }}</span>
              <strong>{{ item.value }}</strong>
              <p>{{ item.hint }}</p>
            </div>
          </div>

          <div class="mt-4 fingerprint-drift-issues">
            <div v-for="row in driftIssueRows" :key="row.title" :class="['fingerprint-drift-issue', row.tone]">
              <strong>{{ row.title }}</strong>
              <p>{{ row.detail }}</p>
            </div>
          </div>

          <dl v-if="driftStatus?.outgoing_header_summary" class="fingerprint-header-summary">
            <div v-for="(value, key) in driftStatus.outgoing_header_summary" :key="key">
              <dt>{{ key }}</dt>
              <dd>{{ value }}</dd>
            </div>
          </dl>
        </div>

        <div class="fingerprint-panel fingerprint-binding-panel">
          <div class="mb-5 flex flex-wrap items-start justify-between gap-3">
            <div>
              <h2>HTTP / TLS 绑定推荐</h2>
              <p>把选中的 Claude Code HTTP 样本和 TLS 模板放在一起看，避免一个像 macOS Node，一个像随机 TLS。</p>
            </div>
            <span :class="['fingerprint-status', tlsBindingRecommendation.tone]">
              一致性 {{ tlsBindingRecommendation.level }}
            </span>
          </div>

          <div class="fingerprint-binding-summary">
            <div>
              <span>HTTP 样本</span>
              <strong>{{ tlsBindingRecommendation.httpSummary }}</strong>
            </div>
            <div>
              <span>推荐 TLS 模板</span>
              <strong>{{ tlsBindingRecommendation.profileName }}</strong>
              <p>{{ tlsBindingRecommendation.profileHint }}</p>
            </div>
          </div>

          <div class="mt-4 fingerprint-binding-meter">
            <span>一致性评分</span>
            <strong>{{ tlsBindingRecommendation.score }}%</strong>
            <i :style="{ width: `${tlsBindingRecommendation.score}%` }" />
          </div>

          <div class="mt-4 fingerprint-check-list">
            <div v-for="reason in tlsBindingRecommendation.reasons" :key="reason" class="fingerprint-check-item compact">
              <span class="fingerprint-check">✓</span>
              <div>
                <strong>判断依据</strong>
                <p>{{ reason }}</p>
              </div>
            </div>
          </div>
        </div>
      </section>

      <section class="fingerprint-panel">
        <details class="fingerprint-advanced">
          <summary>
            <span>
              <strong>高级配置</strong>
              <small>只有在 Claude Code 仍然报客户端限制、或你要调试上游行为时才需要打开。</small>
            </span>
            <span class="fingerprint-chevron">⌄</span>
          </summary>

          <div class="mt-5 grid gap-5 xl:grid-cols-[minmax(0,0.9fr)_minmax(0,1.1fr)]">
            <div class="space-y-4">
              <label class="fingerprint-switch">
                <Toggle v-model="form.enable_fingerprint_unification" />
                <span>
                  <strong>统一客户端指纹</strong>
                  <small>推荐开启。共享上游账号时，网关会统一关键请求头，降低“像不同客户端”的风险。</small>
                </span>
              </label>
              <label class="fingerprint-switch">
                <Toggle v-model="form.enable_metadata_passthrough" />
                <span>
                  <strong>透传 metadata.user_id</strong>
                  <small>默认关闭。开启后更接近原客户端，但稳定性和隐私边界更依赖调用方。</small>
                </span>
              </label>
              <label class="fingerprint-switch">
                <Toggle v-model="form.enable_cch_signing" />
                <span>
                  <strong>Claude Code Billing Header 签名</strong>
                  <small>推荐开启。让 Claude Code 相关占位 header 更接近真实客户端行为。</small>
                </span>
              </label>
            </div>

            <div class="space-y-4">
              <div class="grid gap-4 sm:grid-cols-2">
                <label class="fingerprint-field">
                  <span>Claude Code 最低版本</span>
                  <input
                    v-model.trim="form.min_claude_code_version"
                    class="input"
                    placeholder="留空表示不限制"
                  />
                </label>
                <label class="fingerprint-field">
                  <span>Claude Code 最高版本</span>
                  <input
                    v-model.trim="form.max_claude_code_version"
                    class="input"
                    placeholder="留空表示不限制"
                  />
                </label>
              </div>

              <div class="fingerprint-tls-box">
                <div class="flex flex-wrap items-start justify-between gap-3">
                  <div>
                    <h3>TLS 模板</h3>
                    <p>默认不用管。只有需要模拟特定桌面客户端 TLS 握手时，再到账号侧绑定模板。</p>
                  </div>
                  <button class="btn btn-secondary" @click="showTLSModal = true">
                    管理模板
                  </button>
                </div>

                <div v-if="profiles.length === 0" class="fingerprint-empty">
                  暂无模板。没有模板时系统会使用内置默认行为。
                </div>
                <div v-else class="mt-4 grid gap-3 sm:grid-cols-2">
                  <article v-for="profile in visibleProfiles" :key="profile.id" class="fingerprint-profile-card">
                    <div class="flex items-center justify-between gap-3">
                      <strong>{{ profile.name }}</strong>
                      <span>{{ profile.enable_grease ? 'GREASE' : '普通' }}</span>
                    </div>
                    <p>{{ profile.description || '未填写描述' }}</p>
                  </article>
                </div>
                <p v-if="profiles.length > visibleProfiles.length" class="mt-3 text-xs text-slate-500 dark:text-slate-400">
                  其余 {{ profiles.length - visibleProfiles.length }} 个模板可在管理弹窗中查看。
                </p>
              </div>
            </div>
          </div>
        </details>
      </section>
    </div>
  </AppLayout>

  <TLSFingerprintProfilesModal :show="showTLSModal" @close="handleTLSModalClose" />
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { TLSFingerprintProfile } from '@/api/admin'
import type { ClaudeCodeFingerprintDriftStatus, ClaudeCodeFingerprintProfile } from '@/api/admin/settings'
import AppLayout from '@/components/layout/AppLayout.vue'
import Toggle from '@/components/common/Toggle.vue'
import TLSFingerprintProfilesModal from '@/components/admin/TLSFingerprintProfilesModal.vue'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const saving = ref(false)
const savingHTTPFingerprint = ref(false)
const showTLSModal = ref(false)
const profiles = ref<TLSFingerprintProfile[]>([])
const httpFingerprintProfiles = ref<ClaudeCodeFingerprintProfile[]>([])
const activeHTTPFingerprintID = ref('')
const fingerprintDrift = ref<ClaudeCodeFingerprintDriftStatus | null>(null)

type PresetID = 'stable' | 'compatible' | 'debug'

const form = reactive({
  min_claude_code_version: '',
  max_claude_code_version: '',
  enable_fingerprint_unification: true,
  enable_metadata_passthrough: false,
  enable_cch_signing: true
})

const presets = [
  {
    id: 'stable',
    badge: '推荐',
    title: '稳定模式',
    description: '大多数团队直接选它。统一关键指纹，减少 Claude Code 客户端限制和共享账号漂移。',
    tone: 'good'
  },
  {
    id: 'compatible',
    badge: '兼容',
    title: '兼容模式',
    description: '尽量尊重原客户端 metadata，适合 Cline、Cherry Studio、自研脚本混用时排查问题。',
    tone: 'neutral'
  },
  {
    id: 'debug',
    badge: '排查',
    title: '调试模式',
    description: '尽量少改请求，只保留必要签名。适合短时间定位上游为什么拒绝请求。',
    tone: 'warn'
  }
] as const

const activePreset = computed<PresetID | 'custom'>(() => {
  if (
    form.enable_fingerprint_unification &&
    !form.enable_metadata_passthrough &&
    form.enable_cch_signing
  ) {
    return 'stable'
  }
  if (
    form.enable_fingerprint_unification &&
    form.enable_metadata_passthrough &&
    form.enable_cch_signing
  ) {
    return 'compatible'
  }
  if (
    !form.enable_fingerprint_unification &&
    form.enable_metadata_passthrough &&
    form.enable_cch_signing
  ) {
    return 'debug'
  }
  return 'custom'
})

const activePresetLabel = computed(() => {
  const preset = presets.find((item) => item.id === activePreset.value)
  return preset?.title || '自定义模式'
})

const visibleProfiles = computed(() => profiles.value.slice(0, 4))

const activeHTTPFingerprint = computed(() => {
  if (!activeHTTPFingerprintID.value) {
    return null
  }
  return httpFingerprintProfiles.value.find((profile) => profile.id === activeHTTPFingerprintID.value) || null
})

const driftStatus = computed(() => fingerprintDrift.value)

const driftStatusLabel = computed(() => {
  const status = driftStatus.value?.status
  if (status === 'ok') return '一致'
  if (status === 'warning') return '有偏差'
  return '待采集'
})

const driftStatusTone = computed(() => {
  const status = driftStatus.value?.status
  if (status === 'ok') return 'good'
  if (status === 'warning') return 'warn'
  return 'neutral'
})

const driftChecklist = computed(() => {
  const drift = driftStatus.value
  return [
    {
      label: '样本生效',
      value: drift?.sample_applied ? '已生效' : '未确认',
      ok: drift?.sample_applied === true,
      hint: drift?.active_profile_name || '未选择固定 HTTP 样本'
    },
    {
      label: 'Header 覆盖',
      value: `${drift?.default_overwrites?.length || 0} 项`,
      ok: (drift?.default_overwrites?.length || 0) === 0,
      hint: (drift?.default_overwrites?.length || 0) === 0 ? '未发现默认值回写' : '有样本字段被内置默认值覆盖'
    },
    {
      label: 'Beta Token',
      value: `${(drift?.beta_missing?.length || 0) + (drift?.beta_unexpected?.length || 0)} 处`,
      ok: (drift?.beta_missing?.length || 0) === 0,
      hint: (drift?.beta_missing?.length || 0) === 0 ? '没有缺少样本 beta' : '缺少样本中的 beta token'
    },
    {
      label: 'cc_version',
      value: drift?.cc_version_matches ? '匹配' : '待确认',
      ok: drift?.cc_version_matches === true,
      hint: `${drift?.cc_version_from_ua || '-'} / ${drift?.cc_version_from_billing || '-'}`
    }
  ]
})

const driftIssueRows = computed(() => {
  const drift = driftStatus.value
  if (!drift) return []
  const rows: Array<{ title: string; detail: string; tone: string }> = []
  if (drift.missing_headers?.length) {
    rows.push({
      title: '缺失 Header',
      detail: drift.missing_headers.join(', '),
      tone: 'warn'
    })
  }
  if (drift.default_overwrites?.length) {
    rows.push({
      title: '默认值覆盖',
      detail: drift.default_overwrites.map((item) => `${item.header}: ${item.actual}`).join(' / '),
      tone: 'warn'
    })
  }
  if (drift.header_mismatches?.length) {
    rows.push({
      title: '字段不一致',
      detail: drift.header_mismatches.map((item) => `${item.header}: ${item.actual || '-'}`).join(' / '),
      tone: 'neutral'
    })
  }
  if (drift.beta_missing?.length || drift.beta_unexpected?.length) {
    rows.push({
      title: 'Beta 差异',
      detail: [
        drift.beta_missing?.length ? `缺少 ${drift.beta_missing.join(', ')}` : '',
        drift.beta_unexpected?.length ? `额外 ${drift.beta_unexpected.join(', ')}` : ''
      ].filter(Boolean).join('；'),
      tone: drift.beta_missing?.length ? 'warn' : 'neutral'
    })
  }
  if (rows.length === 0) {
    rows.push({
      title: '暂无明显偏差',
      detail: drift.message || '等待下一次请求刷新巡检结果。',
      tone: drift.status === 'ok' ? 'good' : 'neutral'
    })
  }
  return rows
})

const tlsBindingRecommendation = computed(() => buildTLSBindingRecommendation(activeHTTPFingerprint.value, profiles.value))

const summaryCards = computed(() => [
  {
    label: '当前模式',
    value: activePresetLabel.value,
    enabled: activePreset.value === 'stable',
    hint: activePreset.value === 'stable' ? '新手推荐，适合长期使用。' : '已偏离推荐，可用于兼容或排查。'
  },
  {
    label: 'Claude Code',
    value: form.enable_fingerprint_unification ? '更稳定' : '更原始',
    enabled: form.enable_fingerprint_unification,
    hint: '控制共享账号时是否统一关键客户端特征。'
  },
  {
    label: 'Metadata',
    value: form.enable_metadata_passthrough ? '透传' : '网关处理',
    enabled: !form.enable_metadata_passthrough,
    hint: '默认由网关处理，减少客户端差异。'
  },
  {
    label: 'TLS 模板',
    value: profiles.value.length ? `${profiles.value.length} 个` : '内置默认',
    enabled: true,
    hint: '普通用户不用改，账号侧可按需绑定。'
  }
])

const outcomeItems = computed(() => [
  {
    title: form.enable_fingerprint_unification ? '网关会统一关键客户端特征' : '网关会尽量保留客户端原样',
    description: form.enable_fingerprint_unification
      ? '适合 Claude Code / OAuth 账号共享，减少同一上游账号被不同工具来回切换的痕迹。'
      : '适合短时间排查兼容性问题，但长期使用时更容易出现客户端差异。',
    tone: form.enable_fingerprint_unification ? 'good' : 'warn'
  },
  {
    title: form.enable_metadata_passthrough ? 'metadata.user_id 会透传' : 'metadata.user_id 默认由网关处理',
    description: form.enable_metadata_passthrough
      ? '更接近调用方原始请求，适合调试；但不同客户端会暴露更多差异。'
      : '更适合小团队统一管理，也更容易维持共享账号的稳定表现。',
    tone: form.enable_metadata_passthrough ? 'neutral' : 'good'
  },
  {
    title: form.enable_cch_signing ? 'Claude Code 签名保持开启' : 'Claude Code 签名已关闭',
    description: form.enable_cch_signing
      ? '这是推荐状态，能让 Claude Code 相关 billing 占位 header 更稳定。'
      : '除非明确知道原因，否则不建议关闭。',
    tone: form.enable_cch_signing ? 'good' : 'warn'
  }
])

const strategies = computed(() => [
  {
    name: 'Claude Code',
    scope: '最需要稳定指纹',
    status: form.enable_fingerprint_unification ? '推荐状态' : '排查状态',
    tone: form.enable_fingerprint_unification ? 'good' : 'warn',
    items: [
      '稳定模式会自动统一关键特征',
      '版本限制默认可留空',
      '报客户端限制时优先检查这里'
    ]
  },
  {
    name: 'Codex CLI',
    scope: 'OpenAI / Codex 链路',
    status: '自动识别',
    tone: 'good',
    items: [
      '无需手动配置 headers',
      '继续使用系统模型映射',
      '适合和 Claude Code 分开观察'
    ]
  },
  {
    name: 'Cline / Cherry',
    scope: '通用桌面客户端',
    status: form.enable_metadata_passthrough ? '兼容优先' : '网关优先',
    tone: form.enable_metadata_passthrough ? 'neutral' : 'good',
    items: [
      '一般不强套 Claude Code',
      '兼容模式更适合排查',
      '用量页会保留工具分布'
    ]
  },
  {
    name: '自研脚本',
    scope: 'SDK / curl / Unknown',
    status: '保持可用',
    tone: 'neutral',
    items: [
      '默认不破坏请求语义',
      '建议配合 Key 限额',
      '异常时先看 Trace 和用量记录'
    ]
  }
])

const recommendedSetup = computed(() => [
  {
    mark: '1',
    title: '全局使用稳定模式',
    description: activePreset.value === 'stable'
      ? '当前已是推荐状态：统一指纹开启、metadata 由网关处理、CCH 签名开启。'
      : '点击“一键推荐”恢复稳定模式。自定义状态可以用，但建议只在排查时短期开启。'
  },
  {
    mark: '2',
    title: 'OAuth 账号启用 TLS 指纹',
    description: profiles.value.length
      ? '已有 TLS 模板。给重要 Claude Code 上游账号绑定固定模板，不要频繁随机切换。'
      : '没有模板也能用内置默认；如果仍遇到客户端限制，再用采集器导入真实模板。'
  },
  {
    mark: '3',
    title: '先让真实 Claude Code 跑一次',
    description: '网关会从真实请求学习 User-Agent 和 X-Stainless-* 头，之后同一上游账号复用这组指纹。'
  },
  {
    mark: '4',
    title: '版本限制默认留空',
    description: '不要随手设置最低/最高版本。只有确认某个 Claude Code 版本有兼容问题时再限制。'
  }
])

const captureFlow = [
  {
    step: '01',
    title: 'HTTP 指纹自动学习',
    description: '用真实 Claude Code 通过网关请求一次，系统会按上游账号缓存 UA、X-Stainless、runtime 等字段。'
  },
  {
    step: '02',
    title: 'TLS 指纹用采集器导入',
    description: '在同一台机器或同一网络环境访问采集器，把得到的 YAML 粘贴到 TLS 模板，再绑定到 OAuth 账号。'
  },
  {
    step: '03',
    title: '固定模板，不要乱换',
    description: '稳定比“看起来很新”更重要。重要账号建议绑定固定模板，避免频繁变化造成识别漂移。'
  }
]

const captureCommand = computed(() => {
  const origin = typeof window === 'undefined' ? 'https://你的网关域名' : window.location.origin
  return `ANTHROPIC_BASE_URL="${origin}" ANTHROPIC_AUTH_TOKEN="sk-你的网关密钥" claude -p "ping"`
})

function describeHTTPFingerprint(profile: ClaudeCodeFingerprintProfile | null) {
  if (!profile) {
    return '未选择固定 HTTP 样本'
  }
  const version = extractClaudeCLIVersion(profile.user_agent) || '未知版本'
  const os = profile.stainless_os || '未知 OS'
  const arch = profile.stainless_arch || '未知架构'
  const runtime = [profile.stainless_runtime || 'runtime?', profile.stainless_runtime_version || ''].filter(Boolean).join(' ')
  return `Claude Code ${version} / ${os} / ${arch} / ${runtime}`
}

function extractClaudeCLIVersion(userAgent: string | undefined) {
  const match = (userAgent || '').match(/\/(\d+\.\d+\.\d+)/)
  return match?.[1] || ''
}

function buildTLSBindingRecommendation(
  httpProfile: ClaudeCodeFingerprintProfile | null,
  tlsProfiles: TLSFingerprintProfile[]
) {
  const httpSummary = describeHTTPFingerprint(httpProfile)
  if (!httpProfile) {
    return {
      httpSummary,
      profileName: '先选择 HTTP 样本',
      profileHint: '让真实 Claude Code 请求一次并选中样本后，再给账号绑定 TLS 模板。',
      level: '低',
      tone: 'warn',
      score: 0,
      reasons: ['缺少固定 HTTP 样本，无法判断 HTTP/TLS 是否同源。']
    }
  }

  const candidates = tlsProfiles.map((profile) => ({
    name: profile.name,
    description: profile.description || '',
    enableGrease: profile.enable_grease,
    alpnProtocols: profile.alpn_protocols || [],
    supportedVersions: profile.supported_versions || [],
    extensions: profile.extensions || [],
    score: scoreTLSCandidate(httpProfile, profile),
    reasons: describeTLSCandidateReasons(httpProfile, profile)
  }))
  candidates.push({
    name: 'Built-in Default (Node.js 24.x)',
    description: '没有绑定模板时的内置 Node.js 24.x / Claude Code 默认 TLS 指纹。',
    enableGrease: false,
    alpnProtocols: ['http/1.1'],
    supportedVersions: [772, 771],
    extensions: [],
    score: scoreBuiltInTLSCandidate(httpProfile),
    reasons: ['内置模板面向 Node.js 24.x，适合没有自采集 TLS 模板时作为保守默认。']
  })
  candidates.sort((a, b) => b.score - a.score)
  const best = candidates[0]
  const level = best.score >= 75 ? '高' : best.score >= 55 ? '中' : '低'
  const tone = best.score >= 75 ? 'good' : best.score >= 55 ? 'neutral' : 'warn'
  return {
    httpSummary,
    profileName: best.name,
    profileHint: best.description || '建议在账号侧固定绑定该模板，不建议长期使用随机模板。',
    level,
    tone,
    score: best.score,
    reasons: best.reasons
  }
}

function scoreTLSCandidate(httpProfile: ClaudeCodeFingerprintProfile, profile: TLSFingerprintProfile) {
  const text = `${profile.name} ${profile.description || ''}`.toLowerCase()
  let score = 25
  if (text.includes('claude')) score += 18
  if (text.includes('node')) score += 18
  if (text.includes('24') || text.includes('v24')) score += 12
  const httpOS = (httpProfile.stainless_os || '').toLowerCase()
  if (httpOS && (text.includes(httpOS) || (httpOS.includes('darwin') && text.includes('mac')))) score += 10
  const httpArch = (httpProfile.stainless_arch || '').toLowerCase()
  if (httpArch && text.includes(httpArch)) score += 8
  if (!profile.enable_grease) score += 8
  if ((profile.alpn_protocols || []).includes('http/1.1')) score += 8
  if ((profile.supported_versions || []).includes(772)) score += 6
  if ((profile.extensions || []).length > 0) score += 5
  return Math.min(100, score)
}

function scoreBuiltInTLSCandidate(httpProfile: ClaudeCodeFingerprintProfile) {
  const runtime = `${httpProfile.stainless_runtime || ''} ${httpProfile.stainless_runtime_version || ''}`.toLowerCase()
  let score = 62
  if (runtime.includes('node')) score += 12
  if (runtime.includes('v24')) score += 12
  if ((httpProfile.stainless_os || '').toLowerCase().includes('darwin')) score += 4
  return Math.min(92, score)
}

function describeTLSCandidateReasons(httpProfile: ClaudeCodeFingerprintProfile, profile: TLSFingerprintProfile) {
  const reasons: string[] = []
  const text = `${profile.name} ${profile.description || ''}`.toLowerCase()
  if (text.includes('claude') || text.includes('node')) {
    reasons.push('模板名称/描述与 Claude Code / Node.js 方向一致。')
  }
  if (!profile.enable_grease) {
    reasons.push('GREASE 关闭，和当前内置 Node.js 24 模板更接近。')
  }
  if ((profile.alpn_protocols || []).includes('http/1.1')) {
    reasons.push('ALPN 包含 http/1.1，适合当前上游请求链路。')
  }
  const httpOS = (httpProfile.stainless_os || '').toLowerCase()
  if (httpOS && text.includes(httpOS)) {
    reasons.push(`模板描述包含 ${httpProfile.stainless_os}，和 HTTP 样本 OS 对齐。`)
  }
  if (reasons.length === 0) {
    reasons.push('可用，但和当前 HTTP 样本的系统/运行时特征没有明显绑定证据。')
  }
  return reasons
}

async function copyCaptureCommand() {
  try {
    await navigator.clipboard.writeText(captureCommand.value)
    appStore.showSuccess('采集命令已复制')
  } catch {
    appStore.showError('复制失败，请手动复制命令')
  }
}

function applyFingerprintLibrary(library: { profiles?: ClaudeCodeFingerprintProfile[]; active_id?: string }) {
  httpFingerprintProfiles.value = library.profiles || []
  activeHTTPFingerprintID.value = library.active_id || ''
}

function selectHTTPFingerprint(id: string) {
  activeHTTPFingerprintID.value = id
}

function clearActiveHTTPFingerprint() {
  activeHTTPFingerprintID.value = ''
  void saveActiveHTTPFingerprint()
}

async function saveActiveHTTPFingerprint() {
  savingHTTPFingerprint.value = true
  try {
    const library = await adminAPI.settings.setActiveClaudeCodeFingerprint(activeHTTPFingerprintID.value)
    applyFingerprintLibrary(library)
    appStore.showSuccess(activeHTTPFingerprintID.value ? 'Claude Code 指纹样本已应用' : '已恢复按账号自动学习')
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, '保存指纹样本失败'))
  } finally {
    savingHTTPFingerprint.value = false
  }
}

async function deleteHTTPFingerprint(id: string) {
  savingHTTPFingerprint.value = true
  try {
    const library = await adminAPI.settings.deleteClaudeCodeFingerprint(id)
    applyFingerprintLibrary(library)
    appStore.showSuccess('指纹样本已删除')
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, '删除指纹样本失败'))
  } finally {
    savingHTTPFingerprint.value = false
  }
}

function formatFingerprintTime(timestamp: number) {
  if (!timestamp) {
    return '-'
  }
  return new Date(timestamp * 1000).toLocaleString()
}

function formatBetaTokens(beta: string | undefined) {
  if (!beta) {
    return '-'
  }
  const tokens = beta.split(',').map((item) => item.trim()).filter(Boolean)
  if (tokens.length <= 3) {
    return tokens.join(', ')
  }
  return `${tokens.slice(0, 3).join(', ')} +${tokens.length - 3}`
}

function applyPreset(preset: PresetID) {
  if (preset === 'stable') {
    form.enable_fingerprint_unification = true
    form.enable_metadata_passthrough = false
    form.enable_cch_signing = true
    return
  }
  if (preset === 'compatible') {
    form.enable_fingerprint_unification = true
    form.enable_metadata_passthrough = true
    form.enable_cch_signing = true
    return
  }
  form.enable_fingerprint_unification = false
  form.enable_metadata_passthrough = true
  form.enable_cch_signing = true
}

function applyRecommendedPreset() {
  applyPreset('stable')
}

async function loadData() {
  loading.value = true
  try {
    const [settings, tlsProfiles, fingerprintLibrary, drift] = await Promise.all([
      adminAPI.settings.getSettings(),
      adminAPI.tlsFingerprintProfiles.list(),
      adminAPI.settings.getClaudeCodeFingerprints(),
      adminAPI.settings.getClaudeCodeFingerprintDrift()
    ])
    form.min_claude_code_version = settings.min_claude_code_version || ''
    form.max_claude_code_version = settings.max_claude_code_version || ''
    form.enable_fingerprint_unification = settings.enable_fingerprint_unification !== false
    form.enable_metadata_passthrough = settings.enable_metadata_passthrough === true
    form.enable_cch_signing = settings.enable_cch_signing !== false
    profiles.value = tlsProfiles
    applyFingerprintLibrary(fingerprintLibrary)
    fingerprintDrift.value = drift
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, '加载指纹策略失败'))
  } finally {
    loading.value = false
  }
}

async function saveSettings() {
  saving.value = true
  try {
    await adminAPI.settings.updateSettings({
      min_claude_code_version: form.min_claude_code_version,
      max_claude_code_version: form.max_claude_code_version,
      enable_fingerprint_unification: form.enable_fingerprint_unification,
      enable_metadata_passthrough: form.enable_metadata_passthrough,
      enable_cch_signing: form.enable_cch_signing
    })
    appStore.showSuccess('指纹策略已保存')
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, '保存指纹策略失败'))
  } finally {
    saving.value = false
  }
}

async function handleTLSModalClose() {
  showTLSModal.value = false
  await loadData()
}

onMounted(loadData)
</script>

<style scoped>
.fingerprint-page {
  max-width: 1480px;
  margin: 0 auto;
}

.fingerprint-hero,
.fingerprint-guide,
.fingerprint-panel,
.fingerprint-stat,
.fingerprint-strategy-card,
.fingerprint-profile-card {
  border: 1px solid rgb(226 232 240);
  border-radius: 1.25rem;
  background: rgb(255 255 255 / 0.88);
  box-shadow: 0 18px 60px rgb(15 23 42 / 0.08);
}

:global(.dark) .fingerprint-hero,
:global(.dark) .fingerprint-guide,
:global(.dark) .fingerprint-panel,
:global(.dark) .fingerprint-stat,
:global(.dark) .fingerprint-strategy-card,
:global(.dark) .fingerprint-profile-card {
  border-color: rgb(51 65 85 / 0.9);
  background: rgb(15 23 42 / 0.78);
  box-shadow: 0 18px 60px rgb(0 0 0 / 0.28);
}

.fingerprint-hero {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  padding: 1.5rem;
}

.fingerprint-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
  justify-content: flex-end;
}

.fingerprint-eyebrow {
  margin-bottom: 0.35rem;
  color: rgb(14 165 233);
  font-size: 0.72rem;
  font-weight: 800;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.fingerprint-hero h1,
.fingerprint-guide h2,
.fingerprint-panel h2 {
  color: rgb(15 23 42);
  font-size: 1.45rem;
  font-weight: 850;
}

:global(.dark) .fingerprint-hero h1,
:global(.dark) .fingerprint-guide h2,
:global(.dark) .fingerprint-panel h2 {
  color: white;
}

.fingerprint-hero p,
.fingerprint-guide p,
.fingerprint-panel p,
.fingerprint-stat p,
.fingerprint-strategy-card p,
.fingerprint-profile-card p {
  color: rgb(100 116 139);
}

:global(.dark) .fingerprint-hero p,
:global(.dark) .fingerprint-guide p,
:global(.dark) .fingerprint-panel p,
:global(.dark) .fingerprint-stat p,
:global(.dark) .fingerprint-strategy-card p,
:global(.dark) .fingerprint-profile-card p {
  color: rgb(148 163 184);
}

.fingerprint-guide {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 1.25rem;
  background:
    radial-gradient(circle at 0% 0%, rgb(16 185 129 / 0.13), transparent 32%),
    rgb(255 255 255 / 0.9);
}

:global(.dark) .fingerprint-guide {
  background:
    radial-gradient(circle at 0% 0%, rgb(16 185 129 / 0.18), transparent 34%),
    rgb(15 23 42 / 0.78);
}

.fingerprint-guide-main {
  display: flex;
  align-items: flex-start;
  gap: 1rem;
}

.fingerprint-guide-icon {
  display: grid;
  width: 2.35rem;
  height: 2.35rem;
  flex: none;
  place-items: center;
  border-radius: 999px;
  background: rgb(16 185 129);
  color: white;
  font-weight: 900;
}

.fingerprint-guide-result {
  flex: none;
  min-width: 10rem;
  border: 1px solid rgb(16 185 129 / 0.28);
  border-radius: 1rem;
  background: rgb(236 253 245 / 0.74);
  padding: 0.85rem 1rem;
}

:global(.dark) .fingerprint-guide-result {
  background: rgb(6 78 59 / 0.24);
}

.fingerprint-guide-result span,
.fingerprint-stat span,
.fingerprint-field span {
  display: block;
  color: rgb(100 116 139);
  font-size: 0.78rem;
  font-weight: 750;
}

.fingerprint-guide-result strong {
  display: block;
  margin-top: 0.25rem;
  color: rgb(5 150 105);
  font-size: 1.25rem;
  font-weight: 900;
}

.fingerprint-panel,
.fingerprint-stat {
  padding: 1.25rem;
}

.fingerprint-stat strong {
  display: block;
  margin-top: 0.45rem;
  font-size: 1.35rem;
  font-weight: 900;
}

.fingerprint-stat p {
  margin-top: 0.35rem;
  font-size: 0.78rem;
}

.fingerprint-preset-card {
  display: flex;
  width: 100%;
  align-items: flex-start;
  gap: 0.9rem;
  border: 1px solid rgb(226 232 240);
  border-radius: 1rem;
  padding: 1rem;
  text-align: left;
  transition:
    border-color 160ms ease,
    background-color 160ms ease,
    box-shadow 160ms ease,
    transform 160ms ease;
}

.fingerprint-preset-card:hover {
  border-color: rgb(125 211 252);
  background: rgb(240 249 255 / 0.82);
  transform: translateY(-1px);
}

.fingerprint-preset-card.active {
  border-color: rgb(14 165 233);
  background: rgb(240 249 255);
  box-shadow: 0 14px 32px rgb(14 165 233 / 0.12);
}

:global(.dark) .fingerprint-preset-card {
  border-color: rgb(51 65 85);
}

:global(.dark) .fingerprint-preset-card:hover,
:global(.dark) .fingerprint-preset-card.active {
  border-color: rgb(56 189 248 / 0.72);
  background: rgb(8 47 73 / 0.28);
}

.fingerprint-preset-card strong {
  display: block;
  color: rgb(15 23 42);
  font-size: 0.98rem;
  font-weight: 850;
}

:global(.dark) .fingerprint-preset-card strong {
  color: white;
}

.fingerprint-preset-card small {
  display: block;
  margin-top: 0.25rem;
  color: rgb(100 116 139);
  line-height: 1.55;
}

:global(.dark) .fingerprint-preset-card small {
  color: rgb(148 163 184);
}

.fingerprint-preset-mark,
.fingerprint-badge,
.fingerprint-status,
.fingerprint-profile-card span {
  border-radius: 999px;
  background: rgb(241 245 249);
  color: rgb(51 65 85);
  padding: 0.35rem 0.65rem;
  font-size: 0.72rem;
  font-weight: 850;
}

.fingerprint-preset-mark {
  flex: none;
}

:global(.dark) .fingerprint-preset-mark,
:global(.dark) .fingerprint-badge,
:global(.dark) .fingerprint-status,
:global(.dark) .fingerprint-profile-card span {
  background: rgb(30 41 59);
  color: rgb(203 213 225);
}

.fingerprint-preset-mark.good,
.fingerprint-status.good {
  background: rgb(209 250 229);
  color: rgb(4 120 87);
}

.fingerprint-preset-mark.warn,
.fingerprint-status.warn {
  background: rgb(254 243 199);
  color: rgb(180 83 9);
}

.fingerprint-preset-mark.neutral,
.fingerprint-status.neutral {
  background: rgb(219 234 254);
  color: rgb(29 78 216);
}

.fingerprint-outcome-list {
  display: grid;
  gap: 0.85rem;
}

.fingerprint-outcome {
  display: flex;
  gap: 0.85rem;
  border: 1px solid rgb(226 232 240);
  border-radius: 1rem;
  padding: 1rem;
}

:global(.dark) .fingerprint-outcome {
  border-color: rgb(51 65 85);
}

.fingerprint-outcome strong {
  display: block;
  color: rgb(15 23 42);
  font-weight: 850;
}

:global(.dark) .fingerprint-outcome strong {
  color: white;
}

.fingerprint-outcome p {
  margin-top: 0.25rem;
  font-size: 0.84rem;
  line-height: 1.55;
}

.fingerprint-outcome-dot {
  margin-top: 0.35rem;
  width: 0.6rem;
  height: 0.6rem;
  flex: none;
  border-radius: 999px;
  background: rgb(59 130 246);
  box-shadow: 0 0 0 4px rgb(59 130 246 / 0.12);
}

.fingerprint-outcome-dot.good {
  background: rgb(16 185 129);
  box-shadow: 0 0 0 4px rgb(16 185 129 / 0.12);
}

.fingerprint-outcome-dot.warn {
  background: rgb(245 158 11);
  box-shadow: 0 0 0 4px rgb(245 158 11 / 0.14);
}

.fingerprint-outcome-dot.neutral {
  background: rgb(59 130 246);
  box-shadow: 0 0 0 4px rgb(59 130 246 / 0.12);
}

.fingerprint-check-list,
.fingerprint-capture-flow {
  display: grid;
  gap: 0.85rem;
}

.fingerprint-check-item,
.fingerprint-capture-step,
.fingerprint-command {
  display: flex;
  align-items: flex-start;
  gap: 0.85rem;
  border: 1px solid rgb(226 232 240);
  border-radius: 1rem;
  background:
    linear-gradient(135deg, rgb(248 250 252 / 0.86), rgb(255 255 255 / 0.72));
  padding: 1rem;
}

:global(.dark) .fingerprint-check-item,
:global(.dark) .fingerprint-capture-step,
:global(.dark) .fingerprint-command {
  border-color: rgb(51 65 85);
  background:
    linear-gradient(135deg, rgb(30 41 59 / 0.72), rgb(15 23 42 / 0.5));
}

.fingerprint-check,
.fingerprint-step-number {
  display: grid;
  width: 2rem;
  height: 2rem;
  flex: none;
  place-items: center;
  border-radius: 0.8rem;
  background: rgb(14 165 233);
  color: white;
  font-size: 0.78rem;
  font-weight: 900;
  box-shadow: 0 10px 24px rgb(14 165 233 / 0.18);
}

.fingerprint-check-item strong,
.fingerprint-capture-step strong {
  display: block;
  color: rgb(15 23 42);
  font-weight: 850;
}

:global(.dark) .fingerprint-check-item strong,
:global(.dark) .fingerprint-capture-step strong {
  color: white;
}

.fingerprint-check-item p,
.fingerprint-capture-step p {
  margin-top: 0.25rem;
  font-size: 0.84rem;
  line-height: 1.55;
}

.fingerprint-command {
  margin-top: 1rem;
  align-items: center;
  justify-content: space-between;
}

.fingerprint-command span {
  display: block;
  color: rgb(100 116 139);
  font-size: 0.78rem;
  font-weight: 800;
}

.fingerprint-command code {
  display: block;
  margin-top: 0.35rem;
  color: rgb(15 23 42);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', monospace;
  font-size: 0.78rem;
  line-height: 1.45;
  word-break: break-all;
}

:global(.dark) .fingerprint-command span {
  color: rgb(148 163 184);
}

:global(.dark) .fingerprint-command code {
  color: rgb(226 232 240);
}

.fingerprint-link-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 1px solid rgb(14 165 233 / 0.4);
  border-radius: 0.9rem;
  background: rgb(14 165 233 / 0.1);
  color: rgb(2 132 199);
  padding: 0.62rem 0.95rem;
  font-size: 0.88rem;
  font-weight: 850;
  transition:
    border-color 160ms ease,
    background-color 160ms ease,
    color 160ms ease,
    transform 160ms ease;
}

.fingerprint-link-button:hover {
  border-color: rgb(14 165 233);
  background: rgb(14 165 233);
  color: white;
  transform: translateY(-1px);
}

:global(.dark) .fingerprint-link-button {
  border-color: rgb(56 189 248 / 0.42);
  background: rgb(8 47 73 / 0.35);
  color: rgb(125 211 252);
}

.fingerprint-library-toolbar {
  display: flex;
  align-items: end;
  gap: 1rem;
}

.fingerprint-library-actions {
  display: flex;
  flex: none;
  flex-wrap: wrap;
  gap: 0.75rem;
}

.fingerprint-http-card {
  border: 1px solid rgb(226 232 240);
  border-radius: 1rem;
  background:
    radial-gradient(circle at 100% 0%, rgb(14 165 233 / 0.1), transparent 30%),
    rgb(248 250 252 / 0.76);
  padding: 1rem;
  transition:
    border-color 160ms ease,
    box-shadow 160ms ease,
    transform 160ms ease;
}

.fingerprint-http-card:hover,
.fingerprint-http-card.active {
  border-color: rgb(14 165 233 / 0.68);
  box-shadow: 0 16px 34px rgb(14 165 233 / 0.12);
  transform: translateY(-1px);
}

:global(.dark) .fingerprint-http-card {
  border-color: rgb(51 65 85);
  background:
    radial-gradient(circle at 100% 0%, rgb(14 165 233 / 0.13), transparent 34%),
    rgb(15 23 42 / 0.5);
}

:global(.dark) .fingerprint-http-card:hover,
:global(.dark) .fingerprint-http-card.active {
  border-color: rgb(56 189 248 / 0.64);
}

.fingerprint-http-card h3 {
  color: rgb(15 23 42);
  font-size: 1rem;
  font-weight: 850;
}

:global(.dark) .fingerprint-http-card h3 {
  color: white;
}

.fingerprint-http-meta {
  display: grid;
  gap: 0.65rem;
}

.fingerprint-http-meta div {
  min-width: 0;
}

.fingerprint-http-meta dt {
  color: rgb(100 116 139);
  font-size: 0.72rem;
  font-weight: 850;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.fingerprint-http-meta dd {
  margin-top: 0.18rem;
  overflow-wrap: anywhere;
  color: rgb(30 41 59);
  font-size: 0.82rem;
  line-height: 1.45;
}

:global(.dark) .fingerprint-http-meta dt {
  color: rgb(148 163 184);
}

:global(.dark) .fingerprint-http-meta dd {
  color: rgb(226 232 240);
}

.fingerprint-strategy-card,
.fingerprint-profile-card {
  padding: 1rem;
}

.fingerprint-strategy-card h3,
.fingerprint-profile-card strong,
.fingerprint-tls-box h3 {
  color: rgb(15 23 42);
  font-size: 1rem;
  font-weight: 850;
}

:global(.dark) .fingerprint-strategy-card h3,
:global(.dark) .fingerprint-profile-card strong,
:global(.dark) .fingerprint-tls-box h3 {
  color: white;
}

.fingerprint-strategy-card ul {
  display: grid;
  gap: 0.55rem;
  margin-top: 0.85rem;
  color: rgb(71 85 105);
  font-size: 0.85rem;
}

:global(.dark) .fingerprint-strategy-card ul {
  color: rgb(203 213 225);
}

.fingerprint-strategy-card li {
  position: relative;
  padding-left: 1rem;
}

.fingerprint-strategy-card li::before {
  content: '';
  position: absolute;
  left: 0;
  top: 0.55em;
  width: 0.42rem;
  height: 0.42rem;
  border-radius: 999px;
  background: rgb(14 165 233);
}

.fingerprint-advanced {
  border: 0;
}

.fingerprint-advanced summary {
  display: flex;
  cursor: pointer;
  list-style: none;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
}

.fingerprint-advanced summary::-webkit-details-marker {
  display: none;
}

.fingerprint-advanced summary strong {
  display: block;
  color: rgb(15 23 42);
  font-size: 1.05rem;
  font-weight: 850;
}

:global(.dark) .fingerprint-advanced summary strong {
  color: white;
}

.fingerprint-advanced summary small {
  display: block;
  margin-top: 0.2rem;
  color: rgb(100 116 139);
}

:global(.dark) .fingerprint-advanced summary small {
  color: rgb(148 163 184);
}

.fingerprint-chevron {
  display: grid;
  width: 2rem;
  height: 2rem;
  flex: none;
  place-items: center;
  border-radius: 999px;
  background: rgb(241 245 249);
  color: rgb(71 85 105);
  transition: transform 160ms ease;
}

:global(.dark) .fingerprint-chevron {
  background: rgb(30 41 59);
  color: rgb(203 213 225);
}

.fingerprint-advanced[open] .fingerprint-chevron {
  transform: rotate(180deg);
}

.fingerprint-switch {
  display: flex;
  gap: 0.85rem;
  border: 1px solid rgb(226 232 240);
  border-radius: 1rem;
  padding: 1rem;
}

:global(.dark) .fingerprint-switch {
  border-color: rgb(51 65 85);
}

.fingerprint-switch strong {
  display: block;
  color: rgb(15 23 42);
}

:global(.dark) .fingerprint-switch strong {
  color: white;
}

.fingerprint-switch small {
  display: block;
  margin-top: 0.25rem;
  color: rgb(100 116 139);
  line-height: 1.5;
}

.fingerprint-field {
  display: grid;
  gap: 0.5rem;
}

.fingerprint-tls-box {
  border: 1px solid rgb(226 232 240);
  border-radius: 1rem;
  padding: 1rem;
}

:global(.dark) .fingerprint-tls-box {
  border-color: rgb(51 65 85);
}

.fingerprint-empty {
  margin-top: 1rem;
  border: 1px dashed rgb(148 163 184);
  border-radius: 1rem;
  color: rgb(100 116 139);
  padding: 1rem;
}

.fingerprint-drift-panel,
.fingerprint-binding-panel {
  overflow: hidden;
}

.fingerprint-drift-cards {
  display: grid;
  gap: 0.85rem;
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.fingerprint-drift-card,
.fingerprint-binding-summary > div {
  border: 1px solid rgb(226 232 240);
  border-radius: 1rem;
  background:
    linear-gradient(135deg, rgb(248 250 252 / 0.86), rgb(255 255 255 / 0.72));
  padding: 1rem;
}

:global(.dark) .fingerprint-drift-card,
:global(.dark) .fingerprint-binding-summary > div {
  border-color: rgb(51 65 85);
  background:
    linear-gradient(135deg, rgb(30 41 59 / 0.72), rgb(15 23 42 / 0.5));
}

.fingerprint-drift-card.ok {
  border-color: rgb(16 185 129 / 0.35);
  background:
    radial-gradient(circle at 100% 0%, rgb(16 185 129 / 0.11), transparent 45%),
    rgb(255 255 255 / 0.82);
}

:global(.dark) .fingerprint-drift-card.ok {
  background:
    radial-gradient(circle at 100% 0%, rgb(16 185 129 / 0.16), transparent 48%),
    rgb(15 23 42 / 0.55);
}

.fingerprint-drift-card span,
.fingerprint-binding-summary span,
.fingerprint-binding-meter span {
  display: block;
  color: rgb(100 116 139);
  font-size: 0.75rem;
  font-weight: 850;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.fingerprint-drift-card strong,
.fingerprint-binding-summary strong,
.fingerprint-binding-meter strong {
  display: block;
  margin-top: 0.28rem;
  color: rgb(15 23 42);
  font-size: 1.02rem;
  font-weight: 900;
}

:global(.dark) .fingerprint-drift-card strong,
:global(.dark) .fingerprint-binding-summary strong,
:global(.dark) .fingerprint-binding-meter strong {
  color: white;
}

.fingerprint-drift-card p,
.fingerprint-binding-summary p {
  margin-top: 0.28rem;
  font-size: 0.8rem;
  line-height: 1.45;
}

.fingerprint-drift-issues {
  display: grid;
  gap: 0.75rem;
}

.fingerprint-drift-issue {
  border: 1px solid rgb(226 232 240);
  border-radius: 0.95rem;
  padding: 0.85rem 1rem;
}

.fingerprint-drift-issue.good {
  border-color: rgb(16 185 129 / 0.34);
  background: rgb(236 253 245 / 0.58);
}

.fingerprint-drift-issue.warn {
  border-color: rgb(245 158 11 / 0.36);
  background: rgb(255 251 235 / 0.7);
}

.fingerprint-drift-issue.neutral {
  border-color: rgb(59 130 246 / 0.28);
  background: rgb(239 246 255 / 0.58);
}

:global(.dark) .fingerprint-drift-issue.good {
  background: rgb(6 78 59 / 0.18);
}

:global(.dark) .fingerprint-drift-issue.warn {
  background: rgb(120 53 15 / 0.18);
}

:global(.dark) .fingerprint-drift-issue.neutral {
  background: rgb(30 58 138 / 0.16);
}

.fingerprint-drift-issue strong {
  color: rgb(15 23 42);
  font-weight: 850;
}

:global(.dark) .fingerprint-drift-issue strong {
  color: white;
}

.fingerprint-drift-issue p {
  margin-top: 0.25rem;
  overflow-wrap: anywhere;
  font-size: 0.8rem;
  line-height: 1.45;
}

.fingerprint-header-summary {
  display: grid;
  margin-top: 1rem;
  max-height: 14rem;
  overflow: auto;
  border: 1px solid rgb(226 232 240);
  border-radius: 1rem;
}

:global(.dark) .fingerprint-header-summary {
  border-color: rgb(51 65 85);
}

.fingerprint-header-summary div {
  display: grid;
  gap: 0.5rem;
  grid-template-columns: minmax(8rem, 0.45fr) minmax(0, 1fr);
  border-bottom: 1px solid rgb(226 232 240);
  padding: 0.72rem 0.9rem;
}

.fingerprint-header-summary div:last-child {
  border-bottom: 0;
}

:global(.dark) .fingerprint-header-summary div {
  border-color: rgb(51 65 85);
}

.fingerprint-header-summary dt {
  color: rgb(100 116 139);
  font-size: 0.72rem;
  font-weight: 850;
}

.fingerprint-header-summary dd {
  min-width: 0;
  overflow-wrap: anywhere;
  color: rgb(30 41 59);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', monospace;
  font-size: 0.72rem;
}

:global(.dark) .fingerprint-header-summary dd {
  color: rgb(226 232 240);
}

.fingerprint-binding-summary {
  display: grid;
  gap: 0.85rem;
}

.fingerprint-binding-meter {
  position: relative;
  overflow: hidden;
  border: 1px solid rgb(226 232 240);
  border-radius: 1rem;
  padding: 1rem;
}

:global(.dark) .fingerprint-binding-meter {
  border-color: rgb(51 65 85);
}

.fingerprint-binding-meter i {
  display: block;
  height: 0.55rem;
  margin-top: 0.85rem;
  border-radius: 999px;
  background: linear-gradient(90deg, rgb(14 165 233), rgb(16 185 129));
  box-shadow: 0 8px 20px rgb(14 165 233 / 0.18);
}

.fingerprint-check-item.compact {
  padding: 0.85rem;
}

@media (max-width: 768px) {
  .fingerprint-hero,
  .fingerprint-guide {
    flex-direction: column;
    align-items: stretch;
  }

  .fingerprint-actions {
    justify-content: flex-start;
  }

  .fingerprint-guide-result {
    min-width: 0;
  }

  .fingerprint-command {
    align-items: stretch;
    flex-direction: column;
  }

  .fingerprint-library-toolbar {
    align-items: stretch;
    flex-direction: column;
  }

  .fingerprint-library-actions {
    flex-direction: column;
  }

  .fingerprint-drift-cards,
  .fingerprint-header-summary div {
    grid-template-columns: 1fr;
  }
}
</style>
