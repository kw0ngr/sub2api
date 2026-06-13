import type { AdminGroup, SelectOption } from '@/types'

export interface ApiKeyGroupFilterLabels {
  all: string
  exclusive: string
  public: string
  subscription: string
  disabled: string
}

export interface ApiKeyGroupFilterOption extends SelectOption {
  value: number | null
  label: string
  disabled?: boolean
  kind?: 'group'
}

const groupHeaders = {
  exclusive: -1,
  public: -2,
  subscription: -3,
  disabled: -4,
} as const

export function buildApiKeyGroupFilterOptions(
  groups: AdminGroup[],
  labels: ApiKeyGroupFilterLabels
): ApiKeyGroupFilterOption[] {
  const sections = {
    exclusive: [] as AdminGroup[],
    public: [] as AdminGroup[],
    subscription: [] as AdminGroup[],
    disabled: [] as AdminGroup[],
  }

  for (const group of groups) {
    if (group.status !== 'active') {
      sections.disabled.push(group)
      continue
    }
    if (group.subscription_type === 'subscription') {
      sections.subscription.push(group)
      continue
    }
    if (group.is_exclusive) {
      sections.exclusive.push(group)
    } else {
      sections.public.push(group)
    }
  }

  const options: ApiKeyGroupFilterOption[] = [{ value: null, label: labels.all }]
  const appendSection = (key: keyof typeof sections, label: string) => {
    const section = sections[key]
    if (section.length === 0) return
    options.push({ value: groupHeaders[key], label, disabled: true, kind: 'group' })
    for (const group of section) {
      options.push({ value: group.id, label: group.name })
    }
  }

  appendSection('exclusive', labels.exclusive)
  appendSection('public', labels.public)
  appendSection('subscription', labels.subscription)
  appendSection('disabled', labels.disabled)

  return options
}
