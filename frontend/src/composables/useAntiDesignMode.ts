import { readonly, ref } from 'vue'

export const antiDesignStorageKey = 'sub2api.ui.antiDesign'
const legacyAntiDesignStorageKey = 'sub2api.admin.dashboard.antiDesign'

const antiDesignMode = ref(false)

const canUseDOM = () => typeof window !== 'undefined' && typeof document !== 'undefined'

const readStoredPreference = (): boolean => {
  if (typeof window === 'undefined') return false

  const current = window.localStorage.getItem(antiDesignStorageKey)
  if (current !== null) return current === '1'

  return window.localStorage.getItem(legacyAntiDesignStorageKey) === '1'
}

const applyDocumentClass = (enabled: boolean) => {
  if (!canUseDOM()) return

  document.documentElement.classList.toggle('anti-design', enabled)
  document.body?.classList.toggle('anti-design-body', enabled)
}

const persistPreference = (enabled: boolean) => {
  if (typeof window === 'undefined') return

  window.localStorage.setItem(antiDesignStorageKey, enabled ? '1' : '0')
  window.localStorage.setItem(legacyAntiDesignStorageKey, enabled ? '1' : '0')
}

export const syncAntiDesignModeFromStorage = () => {
  antiDesignMode.value = readStoredPreference()
  applyDocumentClass(antiDesignMode.value)
  return antiDesignMode.value
}

export const setAntiDesignMode = (enabled: boolean) => {
  antiDesignMode.value = enabled
  persistPreference(enabled)
  applyDocumentClass(enabled)
}

export const toggleAntiDesignMode = () => {
  setAntiDesignMode(!antiDesignMode.value)
}

export const useAntiDesignMode = () => {
  syncAntiDesignModeFromStorage()

  return {
    antiDesignMode: readonly(antiDesignMode),
    setAntiDesignMode,
    syncAntiDesignModeFromStorage,
    toggleAntiDesignMode
  }
}
