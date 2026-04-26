import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import type { DashboardStats } from '@/types'
import DashboardView from '../DashboardView.vue'

const { getSnapshotV2, getUserUsageTrend, getUserSpendingRanking, getProjectStats, getUsageInsights, getHourlyActivity } = vi.hoisted(() => ({
  getSnapshotV2: vi.fn(),
  getUserUsageTrend: vi.fn(),
  getUserSpendingRanking: vi.fn(),
  getProjectStats: vi.fn(),
  getUsageInsights: vi.fn(),
  getHourlyActivity: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    dashboard: {
      getSnapshotV2,
      getUserUsageTrend,
      getUserSpendingRanking,
      getProjectStats,
      getUsageInsights,
      getHourlyActivity
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn()
  })
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({
    push: vi.fn()
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

const formatLocalDate = (date: Date): string => {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

const createDashboardStats = (): DashboardStats => ({
  total_users: 0,
  today_new_users: 0,
  active_users: 0,
  hourly_active_users: 0,
  stats_updated_at: '',
  stats_stale: false,
  total_api_keys: 0,
  active_api_keys: 0,
  total_accounts: 0,
  normal_accounts: 0,
  error_accounts: 0,
  ratelimit_accounts: 0,
  overload_accounts: 0,
  total_requests: 0,
  total_input_tokens: 0,
  total_output_tokens: 0,
  total_cache_creation_tokens: 0,
  total_cache_read_tokens: 0,
  total_tokens: 0,
  total_cost: 0,
  total_actual_cost: 0,
  total_account_cost: 0,
  today_requests: 0,
  today_input_tokens: 0,
  today_output_tokens: 0,
  today_cache_creation_tokens: 0,
  today_cache_read_tokens: 0,
  today_tokens: 0,
  today_cost: 0,
  today_actual_cost: 0,
  today_account_cost: 0,
  average_duration_ms: 0,
  uptime: 0,
  rpm: 0,
  tpm: 0
})

describe('admin DashboardView', () => {
  beforeEach(() => {
    getSnapshotV2.mockReset()
    getUserUsageTrend.mockReset()
    getUserSpendingRanking.mockReset()
    getProjectStats.mockReset()
    getUsageInsights.mockReset()
    getHourlyActivity.mockReset()

    getSnapshotV2.mockResolvedValue({
      stats: createDashboardStats(),
      trend: [],
      models: []
    })
    getUserUsageTrend.mockResolvedValue({
      trend: [],
      start_date: '',
      end_date: '',
      granularity: 'hour'
    })
    getUserSpendingRanking.mockResolvedValue({
      ranking: [],
      total_actual_cost: 0,
      total_requests: 0,
      total_tokens: 0,
      start_date: '',
      end_date: ''
    })
    getProjectStats.mockResolvedValue({
      projects: [{
        project_key: 'p1',
        project_label: 'internal-tools',
        requests: 2,
        input_tokens: 10,
        output_tokens: 5,
        cache_creation_tokens: 0,
        cache_read_tokens: 0,
        total_tokens: 15,
        cost: 0,
        actual_cost: 0,
        account_cost: 0
      }],
      start_date: '',
      end_date: ''
    })
    getUsageInsights.mockResolvedValue({
      insights: {
        requests: 2,
        input_tokens: 10,
        output_tokens: 5,
        cache_creation_tokens: 0,
        cache_read_tokens: 0,
        cache_tokens: 0,
        total_tokens: 15,
        input_share: 0.6667,
        output_share: 0.3333,
        cache_creation_share: 0,
        cache_read_share: 0,
        cache_share: 0,
        model_count: 1,
        project_count: 1,
        top_model: 'gpt-5.1',
        top_model_tokens: 15,
        top_model_share: 1,
        top_project_key: 'p1',
        top_project_label: 'internal-tools',
        top_project_tokens: 15,
        top_project_share: 1
      },
      start_date: '',
      end_date: ''
    })
    getHourlyActivity.mockResolvedValue({
      hourly_activity: Array.from({ length: 168 }, (_, index) => ({
        weekday: Math.floor(index / 24),
        hour: index % 24,
        requests: index === 9 ? 2 : 0,
        input_tokens: index === 9 ? 10 : 0,
        output_tokens: index === 9 ? 5 : 0,
        cache_creation_tokens: 0,
        cache_read_tokens: 0,
        total_tokens: index === 9 ? 15 : 0,
        cost: 0,
        actual_cost: 0
      })),
      start_date: '',
      end_date: ''
    })
  })

  it('uses last 24 hours as default dashboard range', async () => {
    mount(DashboardView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          LoadingSpinner: true,
          Icon: true,
          DateRangePicker: true,
          Select: true,
          ModelDistributionChart: true,
          TokenUsageTrend: true,
          Line: true
        }
      }
    })

    await flushPromises()

    const now = new Date()
    const yesterday = new Date(now.getTime() - 24 * 60 * 60 * 1000)

    expect(getSnapshotV2).toHaveBeenCalledTimes(1)
    expect(getSnapshotV2).toHaveBeenCalledWith(expect.objectContaining({
      start_date: formatLocalDate(yesterday),
      end_date: formatLocalDate(now),
      granularity: 'hour'
    }))
  })

  it('renders admin project insights and hourly activity widgets', async () => {
    const wrapper = mount(DashboardView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          LoadingSpinner: true,
          Icon: true,
          DateRangePicker: true,
          Select: true,
          ModelDistributionChart: true,
          TokenUsageTrend: true,
          Line: true
        }
      }
    })

    await flushPromises()

    expect(getProjectStats).toHaveBeenCalledTimes(1)
    expect(getUsageInsights).toHaveBeenCalledTimes(1)
    expect(getHourlyActivity).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('admin.dashboard.projectDistribution')
    expect(wrapper.text()).toContain('internal-tools')
    expect(wrapper.text()).toContain('admin.dashboard.usageInsights')
    expect(wrapper.text()).toContain('gpt-5.1')
    expect(wrapper.find('[data-testid="admin-hourly-activity-cell-0-9"]').attributes('title')).toContain('15')
  })
})
