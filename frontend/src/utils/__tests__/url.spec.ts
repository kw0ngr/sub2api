import { describe, expect, it } from 'vitest'
import { sanitizeUrl } from '@/utils/url'

describe('sanitizeUrl', () => {
  it('allows http and https URLs', () => {
    expect(sanitizeUrl('https://docs.example.com/path')).toBe('https://docs.example.com/path')
    expect(sanitizeUrl('http://docs.example.com/path')).toBe('http://docs.example.com/path')
  })

  it('rejects javascript and protocol-relative URLs', () => {
    expect(sanitizeUrl('javascript:alert(1)')).toBe('')
    expect(sanitizeUrl('//evil.example/logo.png', { allowRelative: true })).toBe('')
  })

  it('only allows relative and image data URLs when explicitly enabled', () => {
    expect(sanitizeUrl('/logo.png')).toBe('')
    expect(sanitizeUrl('/logo.png', { allowRelative: true })).toBe('/logo.png')
    expect(sanitizeUrl('data:image/png;base64,AAAA')).toBe('')
    expect(sanitizeUrl('data:image/png;base64,AAAA', { allowDataUrl: true })).toBe('data:image/png;base64,AAAA')
  })
})
