import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import AccountActionMenu from '../AccountActionMenu.vue'
import type { Account } from '@/types'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key
  })
}))

const makeAccount = (type: Account['type']): Account => ({
  id: 1784,
  name: 'GLM 官方',
  platform: 'glm',
  type,
  proxy_id: null,
  concurrency: 1,
  priority: 0,
  status: 'active',
  error_message: null,
  last_used_at: null,
  expires_at: null,
  auto_pause_on_expired: false,
  created_at: '2026-06-24T00:00:00Z',
  updated_at: '2026-06-24T00:00:00Z',
  schedulable: true,
  rate_limited_at: null,
  rate_limit_reset_at: null,
  overload_until: null,
  temp_unschedulable_until: null,
  temp_unschedulable_reason: null,
  session_window_start: null,
  session_window_end: null,
  session_window_status: null
})

describe('AccountActionMenu', () => {
  it('emits check-health for supported API key accounts', async () => {
    const account = makeAccount('apikey')
    const wrapper = mount(AccountActionMenu, {
      props: {
        show: true,
        account,
        position: { top: 10, left: 10 }
      },
      global: {
        stubs: {
          Teleport: true,
          Icon: true
        }
      }
    })

    const healthButton = wrapper
      .findAll('button')
      .find(button => button.text().includes('admin.accounts.apiKeyHealthCheckOne'))
    expect(healthButton).toBeTruthy()
    await healthButton?.trigger('click')

    expect(wrapper.emitted('check-health')).toEqual([[account]])
    expect(wrapper.emitted('close')).toBeTruthy()
  })

  it('hides check-health for non API key accounts', () => {
    const wrapper = mount(AccountActionMenu, {
      props: {
        show: true,
        account: makeAccount('oauth'),
        position: { top: 10, left: 10 }
      },
      global: {
        stubs: {
          Teleport: true,
          Icon: true
        }
      }
    })

    expect(wrapper.text()).not.toContain('admin.accounts.apiKeyHealthCheckOne')
  })
})
