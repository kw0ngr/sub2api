import { describe, expect, it } from 'vitest'
import type { AdminGroup } from '@/types'
import { buildApiKeyGroupFilterOptions } from '../apiKeyGroupFilterOptions'

const labels = {
  all: 'All',
  exclusive: 'Exclusive',
  public: 'Public',
  subscription: 'Subscription',
  disabled: 'Disabled',
}

function group(partial: Partial<AdminGroup>): AdminGroup {
  return {
    id: 0,
    name: '',
    status: 'active',
    is_exclusive: false,
    subscription_type: 'standard',
    ...partial,
  } as AdminGroup
}

describe('buildApiKeyGroupFilterOptions', () => {
  it('partitions active groups by API-key group type', () => {
    const options = buildApiKeyGroupFilterOptions(
      [
        group({ id: 1, name: 'Exclusive', is_exclusive: true }),
        group({ id: 2, name: 'Public', is_exclusive: false }),
        group({ id: 3, name: 'Subscription', subscription_type: 'subscription' }),
      ],
      labels
    )

    expect(options).toEqual([
      { value: null, label: 'All' },
      { value: -1, label: 'Exclusive', disabled: true, kind: 'group' },
      { value: 1, label: 'Exclusive' },
      { value: -2, label: 'Public', disabled: true, kind: 'group' },
      { value: 2, label: 'Public' },
      { value: -3, label: 'Subscription', disabled: true, kind: 'group' },
      { value: 3, label: 'Subscription' },
    ])
  })

  it('keeps disabled groups visible in a dedicated section', () => {
    const options = buildApiKeyGroupFilterOptions(
      [
        group({ id: 1, name: 'Active Exclusive', is_exclusive: true }),
        group({ id: 2, name: 'Disabled Exclusive', is_exclusive: true, status: 'inactive' }),
      ],
      labels
    )

    expect(options).toContainEqual({ value: -4, label: 'Disabled', disabled: true, kind: 'group' })
    expect(options).toContainEqual({ value: 2, label: 'Disabled Exclusive' })
    expect(options.findIndex((option) => option.value === -1)).toBeLessThan(
      options.findIndex((option) => option.value === -4)
    )
  })

  it('uses unique negative header values to avoid duplicate Select keys', () => {
    const options = buildApiKeyGroupFilterOptions(
      [
        group({ id: 1, name: 'Exclusive', is_exclusive: true }),
        group({ id: 2, name: 'Public' }),
        group({ id: 3, name: 'Subscription', subscription_type: 'subscription' }),
        group({ id: 4, name: 'Disabled', status: 'inactive' }),
      ],
      labels
    )

    const headerValues = options.filter((option) => option.kind === 'group').map((option) => option.value)
    expect(new Set(headerValues).size).toBe(headerValues.length)
    expect(headerValues.every((value) => typeof value === 'number' && value < 0)).toBe(true)
  })

  it('omits empty sections', () => {
    const options = buildApiKeyGroupFilterOptions([group({ id: 2, name: 'Public' })], labels)

    expect(options).toEqual([
      { value: null, label: 'All' },
      { value: -2, label: 'Public', disabled: true, kind: 'group' },
      { value: 2, label: 'Public' },
    ])
  })
})
