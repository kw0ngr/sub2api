import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const testDir = dirname(fileURLToPath(import.meta.url))
const srcRoot = resolve(testDir, '../../..')
const read = (path: string) => readFileSync(resolve(srcRoot, path), 'utf8')

describe('public settings URL rendering', () => {
  it('sanitizes documentation URLs before binding href attributes', () => {
    expect(read('components/layout/AppHeader.vue')).toContain("sanitizeUrl(appStore.docUrl || '')")
    expect(read('views/HomeView.vue')).toContain("const docUrl = computed(() => sanitizeUrl(")
    expect(read('views/KeyUsageView.vue')).toContain("const docUrl = computed(() => sanitizeUrl(")
  })

  it('sanitizes custom site logos while preserving relative/image data logo support', () => {
    const sidebar = read('components/layout/AppSidebar.vue')
    const home = read('views/HomeView.vue')
    const keyUsage = read('views/KeyUsageView.vue')

    for (const source of [sidebar, home, keyUsage]) {
      expect(source).toContain("sanitizeUrl(")
      expect(source).toContain('allowRelative: true')
      expect(source).toContain('allowDataUrl: true')
    }
  })
})
