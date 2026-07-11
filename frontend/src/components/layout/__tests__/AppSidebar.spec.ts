import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../AppSidebar.vue')
const componentSource = readFileSync(componentPath, 'utf8')
const appLayoutPath = resolve(dirname(fileURLToPath(import.meta.url)), '../AppLayout.vue')
const appLayoutSource = readFileSync(appLayoutPath, 'utf8')
const stylePath = resolve(dirname(fileURLToPath(import.meta.url)), '../../../style.css')
const styleSource = readFileSync(stylePath, 'utf8')

describe('AppSidebar custom SVG styles', () => {
  it('does not override uploaded SVG fill or stroke colors', () => {
    expect(componentSource).toContain('.sidebar-svg-icon {')
    expect(componentSource).toContain('color: currentColor;')
    expect(componentSource).toContain('display: block;')
    expect(componentSource).not.toContain('stroke: currentColor;')
    expect(componentSource).not.toContain('fill: none;')
  })
})


describe('AppSidebar scroll position persistence', () => {
  it('binds a template ref to the sidebar nav element', () => {
    expect(componentSource).toContain('ref="sidebarNavRef"')
    expect(componentSource).toContain('sidebar-nav')
  })

  it('saves and restores scroll position through appStore', () => {
    expect(componentSource).toContain('const sidebarNavRef = ref<HTMLElement | null>(null)')
    expect(componentSource).toContain('onBeforeUnmount')
    expect(componentSource).toContain('appStore.sidebarScrollTop = sidebarNavRef.value.scrollTop')
    expect(componentSource).toContain('nextTick')
    expect(componentSource).toContain('sidebarNavRef.value.scrollTop = appStore.sidebarScrollTop')
  })
})

describe('AppSidebar header styles', () => {
  it('does not clip the version badge dropdown', () => {
    const sidebarHeaderBlockMatch = styleSource.match(/\.sidebar-header\s*\{[\s\S]*?\n {2}\}/)
    const sidebarBrandBlockMatch = componentSource.match(/\.sidebar-brand\s*\{[\s\S]*?\n\}/)

    expect(sidebarHeaderBlockMatch).not.toBeNull()
    expect(sidebarBrandBlockMatch).not.toBeNull()
    expect(sidebarHeaderBlockMatch?.[0]).not.toContain('@apply overflow-hidden;')
    expect(sidebarBrandBlockMatch?.[0]).not.toContain('overflow: hidden;')
  })
})

describe('AppSidebar anti-design palette', () => {
  it('keeps sidebar navigation out of browser-blue link styling', () => {
    expect(appLayoutSource).toContain('a:not(.dropdown-item):not(.sidebar-link)')
    expect(appLayoutSource).toContain('.app-shell.app-shell-anti .sidebar-link {')
    expect(appLayoutSource).toContain('text-decoration: none !important;')
    expect(appLayoutSource).toContain('.app-shell.app-shell-anti .sidebar-link-active')
    expect(appLayoutSource).toContain('color: var(--anti-ink) !important;')
  })
})

describe('AppSidebar admin navigation', () => {
  it('exposes the dedicated fingerprint strategy page', () => {
    expect(componentSource).toContain("path: '/admin/fingerprints'")
    expect(componentSource).toContain("label: t('nav.clientFingerprints')")
  })
})
