import { describe, expect, it } from 'vitest'
import { sanitizeSvg } from '../sanitize'

describe('sanitizeSvg', () => {
  it('preserves SVG shapes and filters', () => {
    const result = sanitizeSvg('<svg xmlns="http://www.w3.org/2000/svg"><filter id="blur"><feGaussianBlur stdDeviation="2"/></filter><path d="M0 0h10v10z" filter="url(#blur)"/></svg>')
    expect(result).toContain('<path')
    expect(result).toContain('<filter')
    expect(result).toContain('feGaussianBlur')
  })

  it('removes scripts, event handlers and executable links', () => {
    const result = sanitizeSvg('<svg onload="alert(1)"><script>alert(1)</script><a href="javascript:alert(1)"><path d="M0 0"/></a></svg>')
    expect(result).not.toMatch(/script|onload|javascript:/i)
    expect(result).toContain('<path')
  })

  it('accepts empty input', () => {
    expect(sanitizeSvg('')).toBe('')
  })
})
