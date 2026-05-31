import type { SystemSettings, UpdateSettingsRequest } from '@/api/admin/settings'

export type FingerprintPresetID = 'stable' | 'compatible' | 'debug'
export type ActiveFingerprintPresetID = FingerprintPresetID | 'custom'

export interface FingerprintStrategyForm {
  min_claude_code_version: string
  max_claude_code_version: string
  enable_fingerprint_unification: boolean
  enable_metadata_passthrough: boolean
  enable_cch_signing: boolean
}

export interface FingerprintStrategyPreset {
  id: FingerprintPresetID
  badge: string
  title: string
  description: string
  tone: 'good' | 'neutral' | 'warn'
  changes: string[]
}

export const FINGERPRINT_STRATEGY_PRESETS: readonly FingerprintStrategyPreset[] = [
  {
    id: 'stable',
    badge: '推荐',
    title: '稳定模式',
    description: '大多数团队直接选它。统一关键指纹，减少 Claude Code 客户端限制和共享账号漂移。',
    tone: 'good',
    changes: ['开启统一客户端指纹', '关闭 metadata.user_id 透传', '开启 Claude Code Billing Header 签名']
  },
  {
    id: 'compatible',
    badge: '兼容',
    title: '兼容模式',
    description: '尽量尊重原客户端 metadata，适合 Cline、Cherry Studio、自研脚本混用时排查问题。',
    tone: 'neutral',
    changes: ['开启统一客户端指纹', '透传 metadata.user_id', '保持 Claude Code Billing Header 签名开启']
  },
  {
    id: 'debug',
    badge: '排查',
    title: '调试模式',
    description: '尽量少改请求，只保留必要签名。适合短时间定位上游为什么拒绝请求。',
    tone: 'warn',
    changes: ['关闭统一客户端指纹', '透传 metadata.user_id', '仅保留 Claude Code Billing Header 签名']
  }
] as const

export function createDefaultFingerprintStrategyForm(): FingerprintStrategyForm {
  return {
    min_claude_code_version: '',
    max_claude_code_version: '',
    enable_fingerprint_unification: true,
    enable_metadata_passthrough: false,
    enable_cch_signing: true
  }
}

export function detectFingerprintPreset(form: FingerprintStrategyForm): ActiveFingerprintPresetID {
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
}

export function fingerprintPresetLabel(presetID: ActiveFingerprintPresetID): string {
  const preset = FINGERPRINT_STRATEGY_PRESETS.find((item) => item.id === presetID)
  return preset?.title || '自定义模式'
}

export function fingerprintPresetChanges(presetID: ActiveFingerprintPresetID, form: FingerprintStrategyForm): string[] {
  const preset = FINGERPRINT_STRATEGY_PRESETS.find((item) => item.id === presetID)
  if (preset) {
    return preset.changes
  }
  return [
    form.enable_fingerprint_unification ? '统一客户端指纹：开启' : '统一客户端指纹：关闭',
    form.enable_metadata_passthrough ? 'metadata.user_id：透传' : 'metadata.user_id：网关处理',
    form.enable_cch_signing ? 'Claude Code Billing Header 签名：开启' : 'Claude Code Billing Header 签名：关闭'
  ]
}

export function applyFingerprintPreset(form: FingerprintStrategyForm, presetID: FingerprintPresetID) {
  if (presetID === 'stable') {
    form.enable_fingerprint_unification = true
    form.enable_metadata_passthrough = false
    form.enable_cch_signing = true
    return
  }
  if (presetID === 'compatible') {
    form.enable_fingerprint_unification = true
    form.enable_metadata_passthrough = true
    form.enable_cch_signing = true
    return
  }
  form.enable_fingerprint_unification = false
  form.enable_metadata_passthrough = true
  form.enable_cch_signing = true
}

export function applyFingerprintSettings(form: FingerprintStrategyForm, settings: Pick<SystemSettings,
  | 'min_claude_code_version'
  | 'max_claude_code_version'
  | 'enable_fingerprint_unification'
  | 'enable_metadata_passthrough'
  | 'enable_cch_signing'
>) {
  form.min_claude_code_version = settings.min_claude_code_version || ''
  form.max_claude_code_version = settings.max_claude_code_version || ''
  form.enable_fingerprint_unification = settings.enable_fingerprint_unification !== false
  form.enable_metadata_passthrough = settings.enable_metadata_passthrough === true
  form.enable_cch_signing = settings.enable_cch_signing !== false
}

export function buildFingerprintSettingsPayload(form: FingerprintStrategyForm): UpdateSettingsRequest {
  return {
    min_claude_code_version: form.min_claude_code_version.trim(),
    max_claude_code_version: form.max_claude_code_version.trim(),
    enable_fingerprint_unification: form.enable_fingerprint_unification,
    enable_metadata_passthrough: form.enable_metadata_passthrough,
    enable_cch_signing: form.enable_cch_signing
  }
}

export function validateFingerprintStrategyForm(form: FingerprintStrategyForm): string[] {
  const errors: string[] = []
  const min = parseSemverPrefix(form.min_claude_code_version)
  const max = parseSemverPrefix(form.max_claude_code_version)
  if (min && max && compareSemverTuple(min, max) > 0) {
    errors.push('Claude Code 最低版本不能高于最高版本。')
  }
  return errors
}

function parseSemverPrefix(value: string): [number, number, number] | null {
  const trimmed = value.trim()
  if (!trimmed) return null
  const match = trimmed.match(/^v?(\d+)(?:\.(\d+))?(?:\.(\d+))?/)
  if (!match) return null
  return [
    Number(match[1] || 0),
    Number(match[2] || 0),
    Number(match[3] || 0)
  ]
}

function compareSemverTuple(left: [number, number, number], right: [number, number, number]): number {
  for (let idx = 0; idx < left.length; idx += 1) {
    if (left[idx] !== right[idx]) {
      return left[idx] - right[idx]
    }
  }
  return 0
}
