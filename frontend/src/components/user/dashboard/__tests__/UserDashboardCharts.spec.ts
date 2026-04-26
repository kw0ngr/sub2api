import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import UserDashboardCharts from '../UserDashboardCharts.vue'

const messages: Record<string, string> = {
  'dashboard.timeRange': 'Time range',
  'dashboard.granularity': 'Granularity',
  'dashboard.day': 'Day',
  'dashboard.hour': 'Hour',
  'dashboard.modelDistribution': 'Model Distribution',
  'dashboard.projectDistribution': 'Project Distribution',
  'dashboard.usageInsights': 'Usage Insights',
  'dashboard.hourlyActivity': 'Hourly Activity',
  'dashboard.model': 'Model',
  'dashboard.project': 'Project',
  'dashboard.requests': 'Requests',
  'dashboard.tokens': 'Tokens',
  'dashboard.actual': 'Actual',
  'dashboard.standard': 'Standard',
  'dashboard.topModel': 'Top model',
  'dashboard.topProject': 'Top project',
  'dashboard.cacheShare': 'Cache share',
  'dashboard.variety': 'Variety',
  'dashboard.noDataAvailable': 'No data available',
  'common.refresh': 'Refresh',
}

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => messages[key] ?? key,
    }),
  }
})

vi.mock('vue-chartjs', () => ({
  Doughnut: {
    props: ['data'],
    template: '<div class="chart-data">{{ JSON.stringify(data) }}</div>',
  },
}))

describe('UserDashboardCharts TokenArena-inspired widgets', () => {
  it('renders project distribution, insight summary and heatmap cells', () => {
    const wrapper = mount(UserDashboardCharts, {
      props: {
        loading: false,
        startDate: '2026-04-20',
        endDate: '2026-04-26',
        granularity: 'day',
        trend: [],
        models: [
          {
            model: 'gpt-5.1',
            requests: 2,
            input_tokens: 10,
            output_tokens: 5,
            cache_creation_tokens: 0,
            cache_read_tokens: 0,
            total_tokens: 15,
            cost: 0,
            actual_cost: 0,
            account_cost: 0,
          },
        ],
        projects: [
          {
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
            account_cost: 0,
          },
        ],
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
          top_project_share: 1,
        },
        hourlyActivity: Array.from({ length: 168 }, (_, index) => ({
          weekday: Math.floor(index / 24),
          hour: index % 24,
          requests: index === 9 ? 2 : 0,
          input_tokens: index === 9 ? 10 : 0,
          output_tokens: index === 9 ? 5 : 0,
          cache_creation_tokens: 0,
          cache_read_tokens: 0,
          total_tokens: index === 9 ? 15 : 0,
        })),
      },
      global: {
        stubs: {
          DateRangePicker: true,
          LoadingSpinner: true,
          Select: true,
          TokenUsageTrend: true,
        },
      },
    })

    expect(wrapper.text()).toContain('Project Distribution')
    expect(wrapper.text()).toContain('internal-tools')
    expect(wrapper.text()).toContain('Usage Insights')
    expect(wrapper.text()).toContain('gpt-5.1')
    expect(wrapper.text()).toContain('Hourly Activity')
    expect(wrapper.find('[data-testid="hourly-activity-cell-0-9"]').attributes('title')).toContain('15')
  })
})
