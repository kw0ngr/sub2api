import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import AccountBulkActionsBar from '../AccountBulkActionsBar.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key
  })
}))

describe('AccountBulkActionsBar', () => {
  it('emits check-health for selected accounts', async () => {
    const wrapper = mount(AccountBulkActionsBar, {
      props: {
        selectedIds: [1784]
      }
    })

    const healthButton = wrapper
      .findAll('button')
      .find(button => button.text().includes('admin.accounts.bulkActions.checkHealth'))
    expect(healthButton).toBeTruthy()
    await healthButton?.trigger('click')

    expect(wrapper.emitted('check-health')).toHaveLength(1)
  })

  it('disables check-health while a check is running', () => {
    const wrapper = mount(AccountBulkActionsBar, {
      props: {
        selectedIds: [1784],
        healthChecking: true
      }
    })

    const healthButton = wrapper
      .findAll('button')
      .find(button => button.text().includes('admin.accounts.apiKeyHealthChecking'))
    expect(healthButton?.attributes('disabled')).toBeDefined()
  })
})
