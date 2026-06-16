export const EXPIRY_WARN_DAYS = 7
export const EXPIRY_DANGER_DAYS = 3

export const daysUntil = (iso: string): number =>
  Math.ceil((new Date(iso).getTime() - Date.now()) / 86400000)

export function proxyExpiryBadgeClass(expiresAt: string | null | undefined, status?: string): string {
  if (status === 'expired') return 'badge badge-danger'
  const days = expiresAt ? daysUntil(expiresAt) : Infinity
  if (days <= EXPIRY_DANGER_DAYS) return 'badge badge-danger'
  if (days <= EXPIRY_WARN_DAYS) return 'badge badge-warning'
  return 'text-gray-500'
}

export function proxyExpiryLabelKey(
  expiresAt: string | null | undefined,
  status?: string
): { key: string; params?: { days: number } } {
  if (status === 'expired') return { key: 'admin.proxies.expired' }
  const days = expiresAt ? daysUntil(expiresAt) : Infinity
  if (days < 0) return { key: 'admin.proxies.overdueDays', params: { days: Math.abs(days) } }
  if (days <= EXPIRY_WARN_DAYS) {
    return { key: 'admin.proxies.expiringInDays', params: { days } }
  }
  return { key: 'admin.proxies.remainingDays', params: { days } }
}
