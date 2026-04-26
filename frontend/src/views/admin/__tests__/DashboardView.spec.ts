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
      ranking: [{
        user_id: 2,
        email: 'alice@example.com',
        actual_cost: 1.2,
        requests: 8,
        tokens: 900
      }, {
        user_id: 3,
        email: 'bob@example.com',
        actual_cost: 0.4,
        requests: 4,
        tokens: 100
      }],
      total_actual_cost: 1.6,
      total_requests: 12,
      total_tokens: 1000,
      total_users: 2,
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

  it('renders admin member insights and hourly activity widgets', async () => {
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

    expect(getProjectStats).not.toHaveBeenCalled()
    expect(getUsageInsights).toHaveBeenCalledTimes(1)
    expect(getHourlyActivity).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('admin.dashboard.userDistribution')
    expect(wrapper.text()).toContain('admin.dashboard.memberContribution')
    expect(wrapper.text()).toContain('admin.dashboard.topMember')
    expect(wrapper.text()).toContain('alice@example.com')
    expect(wrapper.text()).not.toContain('admin.dashboard.projectDistribution')
    expect(wrapper.text()).toContain('admin.dashboard.tokenComposition')
    expect(wrapper.text()).toContain('admin.dashboard.usageInsights')
    expect(wrapper.text()).toContain('admin.dashboard.peakActivity')
    expect(wrapper.text()).toContain('gpt-5.1')
    expect(wrapper.find('[data-testid="admin-hourly-activity-cell-0-9"]').attributes('title')).toContain('15')
  })

  it('keeps unattributed project data out of the admin team summary', async () => {
    getProjectStats.mockResolvedValueOnce({
      projects: [{
        project_key: '__unattributed__',
        project_label: 'Unattributed',
        requests: 724,
        input_tokens: 100,
        output_tokens: 200,
        cache_creation_tokens: 10,
        cache_read_tokens: 690,
        total_tokens: 1000,
        cost: 0,
        actual_cost: 0,
        account_cost: 0
      }],
      start_date: '',
      end_date: ''
    })
    getUsageInsights.mockResolvedValueOnce({
      insights: {
        requests: 724,
        input_tokens: 100,
        output_tokens: 200,
        cache_creation_tokens: 10,
        cache_read_tokens: 690,
        cache_tokens: 700,
        total_tokens: 1000,
        input_share: 0.1,
        output_share: 0.2,
        cache_creation_share: 0.01,
        cache_read_share: 0.69,
        cache_share: 0.7,
        model_count: 8,
        project_count: 1,
        top_model: 'claude-opus-4-6',
        top_model_tokens: 479,
        top_model_share: 0.479,
        top_project_key: '__unattributed__',
        top_project_label: 'Unattributed',
        top_project_tokens: 1000,
        top_project_share: 1
      },
      start_date: '',
      end_date: ''
    })

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

    expect(getProjectStats).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('admin.dashboard.userDistribution')
    expect(wrapper.text()).toContain('alice@example.com')
    expect(wrapper.text()).not.toContain('admin.dashboard.unattributedProject')
    expect(wrapper.text()).not.toContain('admin.dashboard.attributionQuality')
    expect(wrapper.text()).not.toContain('admin.dashboard.configureProjectHint')
    expect(wrapper.text()).not.toContain('Unattributed')
  })

  it('renders token composition and peak activity summary', async () => {
    getUsageInsights.mockResolvedValueOnce({
      insights: {
        requests: 10,
        input_tokens: 100,
        output_tokens: 200,
        cache_creation_tokens: 50,
        cache_read_tokens: 650,
        cache_tokens: 700,
        total_tokens: 1000,
        input_share: 0.1,
        output_share: 0.2,
        cache_creation_share: 0.05,
        cache_read_share: 0.65,
        cache_share: 0.7,
        model_count: 2,
        project_count: 1,
        top_model: 'claude-opus-4-6',
        top_model_tokens: 800,
        top_model_share: 0.8,
        top_project_key: 'p1',
        top_project_label: 'internal-tools',
        top_project_tokens: 1000,
        top_project_share: 1
      },
      start_date: '',
      end_date: ''
    })
    getHourlyActivity.mockResolvedValueOnce({
      hourly_activity: Array.from({ length: 168 }, (_, index) => ({
        weekday: Math.floor(index / 24),
        hour: index % 24,
        requests: index === 34 ? 7 : 0,
        input_tokens: index === 34 ? 100 : 0,
        output_tokens: index === 34 ? 200 : 0,
        cache_creation_tokens: 0,
        cache_read_tokens: index === 34 ? 700 : 0,
        total_tokens: index === 34 ? 1000 : 0,
        cost: 0,
        actual_cost: 0
      })),
      start_date: '',
      end_date: ''
    })

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

    expect(wrapper.text()).toContain('admin.dashboard.tokenComposition')
    expect(wrapper.text()).toContain('admin.dashboard.cacheRead')
    expect(wrapper.text()).toContain('admin.dashboard.cacheCreation')
    expect(wrapper.text()).toContain('admin.dashboard.topMember')
    expect(wrapper.text()).toContain('admin.dashboard.peakActivity')
    expect(wrapper.text()).toContain('Mon 10:00')
    expect(wrapper.find('[data-testid="admin-hourly-activity-cell-1-10"]').attributes('title')).toContain('1.00K')
  })
})
