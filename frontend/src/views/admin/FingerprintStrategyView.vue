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
}
</style>
