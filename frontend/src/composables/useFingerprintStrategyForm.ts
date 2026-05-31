import { computed, reactive } from 'vue'
import type { SystemSettings } from '@/api/admin/settings'
import {
  FINGERPRINT_STRATEGY_PRESETS,
  applyFingerprintPreset,
  applyFingerprintSettings,
  buildFingerprintSettingsPayload,
  createDefaultFingerprintStrategyForm,
  detectFingerprintPreset,
  fingerprintPresetChanges,
  fingerprintPresetLabel,
  validateFingerprintStrategyForm,
  type FingerprintPresetID
} from '@/utils/fingerprintStrategy'

export function useFingerprintStrategyForm() {
  const form = reactive(createDefaultFingerprintStrategyForm())

  const activePreset = computed(() => detectFingerprintPreset(form))
  const activePresetLabel = computed(() => fingerprintPresetLabel(activePreset.value))
  const activePresetChanges = computed(() => fingerprintPresetChanges(activePreset.value, form))
  const validationErrors = computed(() => validateFingerprintStrategyForm(form))

  function applyPreset(preset: FingerprintPresetID) {
    applyFingerprintPreset(form, preset)
  }

  function applyRecommendedPreset() {
    applyPreset('stable')
  }

  function loadSettings(settings: SystemSettings) {
    applyFingerprintSettings(form, settings)
  }

  return {
    form,
    presets: FINGERPRINT_STRATEGY_PRESETS,
    activePreset,
    activePresetLabel,
    activePresetChanges,
    validationErrors,
    applyPreset,
    applyRecommendedPreset,
    loadSettings,
    buildPayload: () => buildFingerprintSettingsPayload(form)
  }
}
