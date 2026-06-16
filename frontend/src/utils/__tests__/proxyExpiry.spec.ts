import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { daysUntil, proxyExpiryBadgeClass, proxyExpiryLabelKey } from '../proxyExpiry'

const NOW = new Date('2026-06-15T00:00:00Z')
const isoInDays = (days: number): string =>
  new Date(NOW.getTime() + days * 86400000).toISOString()

beforeEach(() => {
  vi.useFakeTimers()
  vi.setSystemTime(NOW)
})

afterEach(() => {
  vi.useRealTimers()
})

describe('daysUntil', () => {
  it('returns whole days relative to now', () => {
    expect(daysUntil(isoInDays(10))).toBe(10)
    expect(daysUntil(isoInDays(-3))).toBe(-3)
  })
})

describe('proxyExpiryBadgeClass', () => {
  it('uses danger for explicit expired status', () => {
    expect(proxyExpiryBadgeClass(isoInDays(30), 'expired')).toBe('badge badge-danger')
  })

  it('uses danger when expiry is within three days', () => {
    expect(proxyExpiryBadgeClass(isoInDays(2), 'active')).toBe('badge badge-danger')
    expect(proxyExpiryBadgeClass(isoInDays(3), 'active')).toBe('badge badge-danger')
  })

  it('uses warning between four and seven days', () => {
    expect(proxyExpiryBadgeClass(isoInDays(5), 'active')).toBe('badge badge-warning')
    expect(proxyExpiryBadgeClass(isoInDays(7), 'active')).toBe('badge badge-warning')
  })

  it('uses muted text for long-lived proxies', () => {
    expect(proxyExpiryBadgeClass(isoInDays(30), 'active')).toBe('text-gray-500')
  })
})

describe('proxyExpiryLabelKey', () => {
  it('returns expired label for explicit expired status', () => {
    expect(proxyExpiryLabelKey(isoInDays(30), 'expired')).toEqual({
      key: 'admin.proxies.expired'
    })
  })

  it('returns overdue label for past expiry', () => {
    expect(proxyExpiryLabelKey(isoInDays(-3), 'active')).toEqual({
      key: 'admin.proxies.overdueDays',
      params: { days: 3 }
    })
  })

  it('returns expiring label within warning window', () => {
    expect(proxyExpiryLabelKey(isoInDays(5), 'active')).toEqual({
      key: 'admin.proxies.expiringInDays',
      params: { days: 5 }
    })
  })

  it('returns remaining label outside warning window', () => {
    expect(proxyExpiryLabelKey(isoInDays(30), 'active')).toEqual({
      key: 'admin.proxies.remainingDays',
      params: { days: 30 }
    })
  })
})
