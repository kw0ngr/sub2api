import { afterEach, describe, expect, it } from 'vitest'
import { showBootstrapFailure } from '../bootstrapFallback'

afterEach(() => {
  document.body.innerHTML = ''
})

describe('bootstrap fallback', () => {
  it('shows a visible failure instead of leaving the initial page blank', () => {
    document.body.innerHTML = '<div id="app"><p data-bootstrap-message>正在加载页面，请稍候…</p><a href="">刷新重试</a></div>'
    showBootstrapFailure()
    expect(document.querySelector('[data-bootstrap-message]')?.textContent).toBe('页面加载失败，请刷新重试。')
    expect(document.querySelector('a')?.getAttribute('href')).toBe('')
  })

  it('does not change the application after the loading placeholder is removed', () => {
    document.body.innerHTML = '<div id="app">系统设置</div>'
    showBootstrapFailure()
    expect(document.querySelector('#app')?.textContent).toBe('系统设置')
  })
})
