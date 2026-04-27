<template>
  <AppLayout>
    <div class="fingerprint-page space-y-6">
      <section class="fingerprint-hero">
        <div>
          <p class="fingerprint-eyebrow">Gateway identity layer</p>
          <h1>{{ t('admin.fingerprintStrategy.title') }}</h1>
          <p>{{ t('admin.fingerprintStrategy.description') }}</p>
        </div>
        <div class="flex flex-wrap gap-3">
          <button class="btn btn-secondary" :disabled="loading" @click="loadData">
            {{ t('common.refresh') }}
          </button>
          <button class="btn btn-primary" :disabled="saving || loading" @click="saveSettings">
            {{ saving ? t('common.saving') : t('common.save') }}
          </button>
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

      <section class="grid gap-6 xl:grid-cols-[minmax(0,0.95fr)_minmax(0,1.05fr)]">
        <div class="fingerprint-panel space-y-5">
          <div>
            <h2>全局转发策略</h2>
            <p>这组开关会影响共享同一上游 OAuth 账号时的客户端指纹一致性。</p>
          </div>

          <div class="space-y-4">
            <label class="fingerprint-switch">
              <Toggle v-model="form.enable_fingerprint_unification" />
              <span>
                <strong>统一 X-Stainless-* 指纹</strong>
                <small>共享 OAuth 账号时复用稳定请求头，减少客户端差异造成的上游判定抖动。</small>
              </span>
            </label>
            <label class="fingerprint-switch">
              <Toggle v-model="form.enable_metadata_passthrough" />
              <span>
                <strong>metadata.user_id 透传</strong>
                <small>保留客户端原始 metadata.user_id；适合追求上游缓存命中，但会暴露更多客户端差异。</small>
              </span>
            </label>
            <label class="fingerprint-switch">
              <Toggle v-model="form.enable_cch_signing" />
              <span>
                <strong>CCH Billing Header 签名</strong>
                <small>对 Claude Code billing 占位 header 做稳定签名，建议保持开启。</small>
              </span>
            </label>
          </div>

          <div class="grid gap-4 sm:grid-cols-2">
            <label class="fingerprint-field">
              <span>Claude Code 最低版本</span>
              <input
                v-model.trim="form.min_claude_code_version"
                class="input"
                placeholder="例如 2.1.63"
              />
            </label>
            <label class="fingerprint-field">
              <span>Claude Code 最高版本</span>
              <input
                v-model.trim="form.max_claude_code_version"
                class="input"
                placeholder="例如 2.5.0"
              />
            </label>
          </div>
        </div>

        <div class="fingerprint-panel">
          <div class="mb-5 flex flex-wrap items-start justify-between gap-3">
            <div>
              <h2>客户端策略矩阵</h2>
              <p>把之前散在账号、系统设置里的伪装策略收拢成一个可读总览。</p>
            </div>
            <span class="fingerprint-badge">{{ profiles.length }} TLS profiles</span>
          </div>

          <div class="grid gap-4 md:grid-cols-2">
            <article v-for="strategy in strategies" :key="strategy.name" class="fingerprint-strategy-card">
              <div class="mb-3 flex items-center justify-between gap-3">
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
        </div>
      </section>

      <section class="fingerprint-panel">
        <div class="mb-5 flex flex-wrap items-start justify-between gap-3">
          <div>
            <h2>TLS 指纹模板</h2>
            <p>账号侧可绑定固定模板或随机模板；这里展示当前可用模板并跳转到管理弹窗。</p>
          </div>
          <button class="btn btn-secondary" @click="showTLSModal = true">
            管理 TLS 模板
          </button>
        </div>

        <div v-if="profiles.length === 0" class="fingerprint-empty">
          暂无 TLS 指纹模板。建议先采集 Claude Code / Node.js 风格模板，再绑定到敏感上游账号。
        </div>
        <div v-else class="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
          <article v-for="profile in profiles" :key="profile.id" class="fingerprint-profile-card">
            <div class="flex items-center justify-between gap-3">
              <strong>{{ profile.name }}</strong>
              <span>{{ profile.enable_grease ? 'GREASE' : 'NO GREASE' }}</span>
            </div>
            <p>{{ profile.description || '未填写描述' }}</p>
            <div class="fingerprint-profile-meta">
              <span>ALPN {{ profile.alpn_protocols?.join(', ') || '-' }}</span>
              <span>{{ profile.supported_versions?.length || 0 }} TLS versions</span>
            </div>
          </article>
        </div>
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
import AppLayout from '@/components/layout/AppLayout.vue'
import Toggle from '@/components/common/Toggle.vue'
import TLSFingerprintProfilesModal from '@/components/admin/TLSFingerprintProfilesModal.vue'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const saving = ref(false)
const showTLSModal = ref(false)
const profiles = ref<TLSFingerprintProfile[]>([])

const form = reactive({
  min_claude_code_version: '',
  max_claude_code_version: '',
  enable_fingerprint_unification: true,
  enable_metadata_passthrough: false,
  enable_cch_signing: true
})

const summaryCards = computed(() => [
  {
    label: '指纹统一化',
    value: form.enable_fingerprint_unification ? 'ON' : 'OFF',
    enabled: form.enable_fingerprint_unification,
    hint: '控制 X-Stainless-* 是否按账号统一。'
  },
  {
    label: 'Metadata 策略',
    value: form.enable_metadata_passthrough ? 'PASSTHROUGH' : 'REWRITE',
    enabled: !form.enable_metadata_passthrough,
    hint: '默认重写更稳定，透传更利于上游缓存归因。'
  },
  {
    label: 'CCH 签名',
    value: form.enable_cch_signing ? 'SIGNED' : 'RAW',
    enabled: form.enable_cch_signing,
    hint: '建议保持签名，降低 Claude Code 伪装漂移。'
  },
  {
    label: 'TLS 模板',
    value: String(profiles.value.length),
    enabled: profiles.value.length > 0,
    hint: '可在账号侧选择固定或随机模板。'
  }
])

const strategies = computed(() => [
  {
    name: 'Claude Code',
    scope: 'Anthropic / Claude 兼容入口',
    status: form.enable_fingerprint_unification ? '统一中' : '透传中',
    tone: form.enable_fingerprint_unification ? 'good' : 'warn',
    items: [
      '应用 Claude Code UA / beta / metadata 结构',
      '支持最低/最高版本门禁',
      '建议绑定 Node.js / Claude Code TLS 模板'
    ]
  },
  {
    name: 'Codex CLI',
    scope: 'OpenAI OAuth / Responses / Compact',
    status: '策略化',
    tone: 'good',
    items: [
      '保留 Codex CLI only 账号标记',
      '统一远程 compact 与 responses 链路观测',
      '与 OpenAI OAuth TLS 模板独立配置'
    ]
  },
  {
    name: 'Cline / Cherry Studio',
    scope: '通用 API 客户端',
    status: form.enable_metadata_passthrough ? '客户端优先' : '网关优先',
    tone: form.enable_metadata_passthrough ? 'warn' : 'good',
    items: [
      '默认保留请求兼容性，不强行套 Claude Code',
      '可通过账号 TLS 模板模拟固定桌面客户端',
      '使用记录中保留客户端/工具分布'
    ]
  },
  {
    name: 'Unknown / Custom',
    scope: '未知脚本、SDK、自研工具',
    status: '隔离观察',
    tone: 'neutral',
    items: [
      '不破坏原始请求语义',
      '建议配合分组/Key 限额护栏',
      '后续可加入按客户端策略覆盖'
    ]
  }
])

async function loadData() {
  loading.value = true
  try {
    const [settings, tlsProfiles] = await Promise.all([
      adminAPI.settings.getSettings(),
      adminAPI.tlsFingerprintProfiles.list()
    ])
    form.min_claude_code_version = settings.min_claude_code_version || ''
    form.max_claude_code_version = settings.max_claude_code_version || ''
    form.enable_fingerprint_unification = settings.enable_fingerprint_unification !== false
    form.enable_metadata_passthrough = settings.enable_metadata_passthrough === true
    form.enable_cch_signing = settings.enable_cch_signing !== false
    profiles.value = tlsProfiles
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
.fingerprint-panel,
.fingerprint-stat,
.fingerprint-strategy-card,
.fingerprint-profile-card {
  border: 1px solid rgb(226 232 240);
  border-radius: 1.25rem;
  background: rgb(255 255 255 / 0.86);
  box-shadow: 0 18px 60px rgb(15 23 42 / 0.08);
}

:global(.dark) .fingerprint-hero,
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

.fingerprint-eyebrow {
  margin-bottom: 0.35rem;
  color: rgb(14 165 233);
  font-size: 0.72rem;
  font-weight: 800;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.fingerprint-hero h1,
.fingerprint-panel h2 {
  color: rgb(15 23 42);
  font-size: 1.45rem;
  font-weight: 850;
}

:global(.dark) .fingerprint-hero h1,
:global(.dark) .fingerprint-panel h2 {
  color: white;
}

.fingerprint-hero p,
.fingerprint-panel p,
.fingerprint-stat p,
.fingerprint-strategy-card p,
.fingerprint-profile-card p {
  color: rgb(100 116 139);
}

:global(.dark) .fingerprint-hero p,
:global(.dark) .fingerprint-panel p,
:global(.dark) .fingerprint-stat p,
:global(.dark) .fingerprint-strategy-card p,
:global(.dark) .fingerprint-profile-card p {
  color: rgb(148 163 184);
}

.fingerprint-panel,
.fingerprint-stat {
  padding: 1.25rem;
}

.fingerprint-stat span,
.fingerprint-field span {
  display: block;
  color: rgb(100 116 139);
  font-size: 0.78rem;
  font-weight: 750;
}

.fingerprint-stat strong {
  display: block;
  margin-top: 0.45rem;
  font-size: 1.5rem;
  font-weight: 900;
}

.fingerprint-stat p {
  margin-top: 0.35rem;
  font-size: 0.78rem;
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

.fingerprint-badge,
.fingerprint-status,
.fingerprint-profile-card span {
  border-radius: 999px;
  background: rgb(241 245 249);
  color: rgb(51 65 85);
  padding: 0.35rem 0.65rem;
  font-size: 0.72rem;
  font-weight: 800;
}

:global(.dark) .fingerprint-badge,
:global(.dark) .fingerprint-status,
:global(.dark) .fingerprint-profile-card span {
  background: rgb(30 41 59);
  color: rgb(203 213 225);
}

.fingerprint-status.good {
  background: rgb(209 250 229);
  color: rgb(4 120 87);
}

.fingerprint-status.warn {
  background: rgb(254 243 199);
  color: rgb(180 83 9);
}

.fingerprint-status.neutral {
  background: rgb(219 234 254);
  color: rgb(29 78 216);
}

.fingerprint-strategy-card,
.fingerprint-profile-card {
  padding: 1rem;
}

.fingerprint-strategy-card h3,
.fingerprint-profile-card strong {
  color: rgb(15 23 42);
  font-size: 1rem;
  font-weight: 850;
}

:global(.dark) .fingerprint-strategy-card h3,
:global(.dark) .fingerprint-profile-card strong {
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

.fingerprint-empty {
  border: 1px dashed rgb(148 163 184);
  border-radius: 1rem;
  color: rgb(100 116 139);
  padding: 1rem;
}

.fingerprint-profile-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  margin-top: 0.8rem;
}

@media (max-width: 768px) {
  .fingerprint-hero {
    flex-direction: column;
  }
}
</style>
