import { beforeEach, describe, expect, it } from 'vitest'

import { useAntiDesignMode } from '../useAntiDesignMode'

describe('useAntiDesignMode', () => {
  beforeEach(() => {
    window.localStorage.clear()
    document.documentElement.classList.remove('anti-design')
    document.body.classList.remove('anti-design-body')
  })

  it('toggles and persists the backend-wide anti-design mode', () => {
    const { antiDesignMode, toggleAntiDesignMode } = useAntiDesignMode()

    expect(antiDesignMode.value).toBe(false)

    toggleAntiDesignMode()

    expect(antiDesignMode.value).toBe(true)
    expect(window.localStorage.getItem('sub2api.ui.antiDesign')).toBe('1')
    expect(window.localStorage.getItem('sub2api.admin.dashboard.antiDesign')).toBe('1')
    expect(document.documentElement.classList.contains('anti-design')).toBe(true)
  })

  it('keeps compatibility with the old dashboard-only preference key', () => {
    window.localStorage.setItem('sub2api.admin.dashboard.antiDesign', '1')

    const { antiDesignMode } = useAntiDesignMode()

    expect(antiDesignMode.value).toBe(true)
    expect(document.documentElement.classList.contains('anti-design')).toBe(true)
  })
})
