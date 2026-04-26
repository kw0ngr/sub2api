import { describe, expect, it, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { nextTick } from 'vue'

import UsageView from '../UsageView.vue'

const { query, getStatsByDateRange, getDashboardSelfInsights, list, showError, showWarning, showSuccess, showInfo, copyToClipboard } = vi.hoisted(() => ({
  query: vi.fn(),
  getStatsByDateRange: vi.fn(),
  getDashboardSelfInsights: vi.fn(),
  list: vi.fn(),
  showError: vi.fn(),
  showWarning: vi.fn(),
  showSuccess: vi.fn(),
  showInfo: vi.fn(),
  copyToClipboard: vi.fn(),
}))

const messages: Record<string, string> = {
  'usage.costDetails': 'Cost Breakdown',
  'admin.usage.inputCost': 'Input Cost',
  'admin.usage.outputCost': 'Output Cost',
  'admin.usage.cacheCreationCost': 'Cache Creation Cost',
  'admin.usage.cacheReadCost': 'Cache Read Cost',
  'usage.inputTokenPrice': 'Input price',
  'usage.outputTokenPrice': 'Output price',
  'usage.perMillionTokens': '/ 1M tokens',
  'usage.serviceTier': 'Service tier',
  'usage.serviceTierPriority': 'Fast',
  'usage.serviceTierFlex': 'Flex',
  'usage.serviceTierStandard': 'Standard',
  'usage.rate': 'Rate',
  'usage.original': 'Original',
  'usage.billed': 'Billed',
  'usage.allApiKeys': 'All API Keys',
  'usage.apiKeyFilter': 'API Key',
  'usage.model': 'Model',
  'usage.reasoningEffort': 'Reasoning Effort',
  'usage.type': 'Type',
  'usage.tokens': 'Tokens',
  'usage.cost': 'Cost',
  'usage.firstToken': 'First Token',
  'usage.duration': 'Duration',
  'usage.time': 'Time',
  'usage.userAgent': 'User Agent',
  'usage.noRecords': 'No usage records',
}

vi.mock('@/api', () => ({
  usageAPI: {
    query,
    getStatsByDateRange,
    getDashboardSelfInsights,
  },
  keysAPI: {
    list,
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError, showWarning, showSuccess, showInfo }),
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({ copyToClipboard }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => messages[key] ?? key,
    }),
  }
})

const AppLayoutStub = { template: '<div><slot /></div>' }
const TablePageLayoutStub = {
  template:
    '<div><slot name="actions" /><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>',
}
const DataTableStub = {
  props: ['columns', 'data', 'loading'],
  template: `
    <div data-testid="usage-data-table">
      <div v-if="loading" data-testid="usage-loading">Loading</div>
      <slot v-else-if="!data || data.length === 0" name="empty" />
      <div v-else>
        <div v-for="row in data" :key="row.request_id || row.id" data-testid="usage-row">
          <slot name="cell-api_key" :row="row" :value="row.api_key" />
          <slot name="cell-model" :row="row" :value="row.model">{{ row.model }}</slot>
          <slot name="cell-created_at" :row="row" :value="row.created_at">{{ row.created_at }}</slot>
        </div>
      </div>
    </div>
  `,
}
const PaginationStub = {
  props: ['page', 'total', 'pageSize'],
  template: '<div data-testid="usage-pagination">{{ total }}</div>',
}

const mountUsageView = () => mount(UsageView, {
  global: {
    stubs: {
      AppLayout: AppLayoutStub,
      TablePageLayout: TablePageLayoutStub,
      DataTable: DataTableStub,
      Pagination: PaginationStub,
      EmptyState: true,
      Select: true,
      DateRangePicker: true,
      Icon: true,
      Teleport: true,
    },
  },
})

const usageLog = (overrides: Record<string, unknown> = {}) => ({
  request_id: 'req-user-1',
  actual_cost: 0.092883,
  total_cost: 0.092883,
  rate_multiplier: 1,
  service_tier: 'priority',
  input_cost: 0.020285,
  output_cost: 0.00303,
  cache_creation_cost: 0,
  cache_read_cost: 0.069568,
  input_tokens: 4057,
  output_tokens: 101,
  cache_creation_tokens: 0,
  cache_read_tokens: 278272,
  cache_creation_5m_tokens: 0,
  cache_creation_1h_tokens: 0,
  image_count: 0,
  image_size: null,
  first_token_ms: null,
  duration_ms: 1,
  created_at: '2026-03-08T00:00:00Z',
  model: 'gpt-5.4',
  api_key: { name: 'work-key' },
  ...overrides,
})

const mockEmptyStats = () => {
  getStatsByDateRange.mockResolvedValue({
    total_requests: 0,
    total_tokens: 0,
    total_cost: 0,
    avg_duration_ms: 0,
  })
}

describe('user UsageView tooltip', () => {
  beforeEach(() => {
    query.mockReset()
    getStatsByDateRange.mockReset()
    getDashboardSelfInsights.mockReset()
    list.mockReset()
    showError.mockReset()
    showWarning.mockReset()
    showSuccess.mockReset()
    showInfo.mockReset()
    copyToClipboard.mockReset()
    copyToClipboard.mockResolvedValue(true)

    vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockReturnValue({
      x: 0,
      y: 0,
      top: 20,
      left: 20,
      right: 120,
      bottom: 40,
      width: 100,
      height: 20,
      toJSON: () => ({}),
    } as DOMRect)

    ;(globalThis as any).ResizeObserver = class {
      observe() {}
      disconnect() {}
    }

    getDashboardSelfInsights.mockResolvedValue({
      insights: {
        total_requests: 1,
        total_tokens: 100,
        cache_tokens: 20,
        cache_share: 0.2,
        client_distribution: [],
        model_matrix: [],
        cache_efficiency: [],
        sessions: [],
      },
      start_date: '',
      end_date: '',
    })
  })

  it('calls the personal usage list API on mount and renders returned records', async () => {
    query.mockResolvedValue({
      items: [
        usageLog({
          request_id: 'req-visible',
          model: 'claude-sonnet-4-6',
          api_key: { name: 'personal-key' },
        }),
      ],
      total: 1,
      pages: 1,
    })
    mockEmptyStats()
    list.mockResolvedValue({ items: [] })

    const wrapper = mountUsageView()

    await flushPromises()

    expect(query).toHaveBeenCalledTimes(1)
    const [params, config] = query.mock.calls[0]
    expect(params).toEqual(expect.objectContaining({
      page: 1,
      page_size: expect.any(Number),
      sort_by: 'created_at',
      sort_order: 'desc',
      start_date: expect.stringMatching(/^\d{4}-\d{2}-\d{2}$/),
      end_date: expect.stringMatching(/^\d{4}-\d{2}-\d{2}$/),
    }))
    expect(params).not.toHaveProperty('api_key_id')
    expect(config).toEqual(expect.objectContaining({ signal: expect.any(AbortSignal) }))
    expect(wrapper.findAll('[data-testid="usage-row"]')).toHaveLength(1)
    expect(wrapper.text()).toContain('claude-sonnet-4-6')
    expect(wrapper.text()).toContain('personal-key')
    expect(wrapper.find('[data-testid="usage-pagination"]').exists()).toBe(true)
  })

  it('omits the api_key_id filter when all API keys are selected', async () => {
    query.mockResolvedValue({
      items: [],
      total: 0,
      pages: 0,
    })
    mockEmptyStats()
    list.mockResolvedValue({ items: [] })

    const wrapper = mountUsageView()
    await flushPromises()

    query.mockClear()
    getStatsByDateRange.mockClear()
    const setupState = (wrapper.vm as any).$?.setupState
    const filters = setupState.filters?.value ?? setupState.filters
    filters.api_key_id = null

    setupState.applyFilters()
    await flushPromises()

    const params = query.mock.calls[0][0] as Record<string, unknown>
    expect(params).not.toHaveProperty('api_key_id')
    expect(getStatsByDateRange).toHaveBeenLastCalledWith(
      expect.stringMatching(/^\d{4}-\d{2}-\d{2}$/),
      expect.stringMatching(/^\d{4}-\d{2}-\d{2}$/),
      undefined
    )
  })

  it('shows fast service tier and unit prices in user tooltip', async () => {
    query.mockResolvedValue({
      items: [
        {
          request_id: 'req-user-1',
          actual_cost: 0.092883,
          total_cost: 0.092883,
          rate_multiplier: 1,
          service_tier: 'priority',
          input_cost: 0.020285,
          output_cost: 0.00303,
          cache_creation_cost: 0,
          cache_read_cost: 0.069568,
          input_tokens: 4057,
          output_tokens: 101,
          cache_creation_tokens: 0,
          cache_read_tokens: 278272,
          cache_creation_5m_tokens: 0,
          cache_creation_1h_tokens: 0,
          image_count: 0,
          image_size: null,
          first_token_ms: null,
          duration_ms: 1,
          created_at: '2026-03-08T00:00:00Z',
        },
      ],
      total: 1,
      pages: 1,
    })
    getStatsByDateRange.mockResolvedValue({
      total_requests: 1,
      total_tokens: 100,
      total_cost: 0.1,
      avg_duration_ms: 1,
    })
    list.mockResolvedValue({ items: [] })

    const wrapper = mountUsageView()

    await flushPromises()
    await nextTick()

    const setupState = (wrapper.vm as any).$?.setupState
    setupState.tooltipData = {
      request_id: 'req-user-1',
      actual_cost: 0.092883,
      total_cost: 0.092883,
      rate_multiplier: 1,
      service_tier: 'priority',
      input_cost: 0.020285,
      output_cost: 0.00303,
      cache_creation_cost: 0,
      cache_read_cost: 0.069568,
      input_tokens: 4057,
      output_tokens: 101,
    }
    setupState.tooltipVisible = true
    await nextTick()

    const text = wrapper.text()
    expect(text).toContain('Service tier')
    expect(text).toContain('Fast')
    expect(text).toContain('Rate')
    expect(text).toContain('1.00x')
    expect(text).toContain('Billed')
    expect(text).toContain('$0.092883')
    expect(text).toContain('$5.0000 / 1M tokens')
    expect(text).toContain('$30.0000 / 1M tokens')
  })

  it('hides optional self insight cards when selected range has no insight data', async () => {
    query.mockResolvedValue({
      items: [],
      total: 0,
      pages: 0,
    })
    getStatsByDateRange.mockResolvedValue({
      total_requests: 0,
      total_tokens: 0,
      total_cost: 0,
      avg_duration_ms: 0,
    })
    getDashboardSelfInsights.mockResolvedValue({
      insights: {
        total_requests: 0,
        total_tokens: 0,
        cache_tokens: 0,
        cache_share: 0,
        client_distribution: [],
        model_matrix: [],
        cache_efficiency: [],
        sessions: [],
      },
      start_date: '',
      end_date: '',
    })
    list.mockResolvedValue({ items: [] })

    const wrapper = mountUsageView()

    await flushPromises()

    const text = wrapper.text()
    expect(text).not.toContain('我的使用画像')
    expect(text).not.toContain('客户端 / 工具分布')
    expect(text).not.toContain('请求会话')
    expect(text).not.toContain('模型使用矩阵')
    expect(text).not.toContain('缓存效率')
    expect(text).not.toContain('个人使用洞察')
  })

  it('renders self-service insight cards when data exists', async () => {
    query.mockResolvedValue({
      items: [],
      total: 0,
      pages: 0,
    })
    getStatsByDateRange.mockResolvedValue({
      total_requests: 3,
      total_tokens: 1234,
      total_cost: 0,
      avg_duration_ms: 0,
    })
    getDashboardSelfInsights.mockResolvedValue({
      insights: {
        total_requests: 3,
        total_tokens: 1234,
        cache_tokens: 234,
        cache_share: 0.19,
        client_distribution: [
          {
            client: 'Claude Code',
            total_requests: 2,
            total_tokens: 900,
            token_share: 0.73,
          },
        ],
        model_matrix: [
          {
            member: 'me@example.com',
            model: 'claude-sonnet-4-6',
            total_requests: 2,
            total_tokens: 900,
            share_of_member: 0.73,
            cache_tokens: 200,
            cache_share: 0.22,
          },
        ],
        cache_efficiency: [
          {
            scope: 'model',
            model: 'claude-sonnet-4-6',
            label: 'claude-sonnet-4-6',
            total_requests: 2,
            total_tokens: 900,
            cache_read_tokens: 160,
            cache_creation_tokens: 40,
            cache_tokens: 200,
            cache_share: 0.22,
            ttl_override_count: 1,
          },
        ],
        sessions: [
          {
            session_id: 'session-1',
            client: 'Claude Code',
            model: 'claude-sonnet-4-6',
            api_key_id: 1,
            api_key_name: 'work-key',
            total_requests: 2,
            total_tokens: 900,
            first_seen: '2026-04-26T00:00:00Z',
            last_seen: '2026-04-26T00:05:00Z',
          },
        ],
      },
      start_date: '',
      end_date: '',
    })
    list.mockResolvedValue({ items: [] })

    const wrapper = mountUsageView()

    await flushPromises()

    const text = wrapper.text()
    expect(text).toContain('我的使用画像')
    expect(text).toContain('客户端 / 工具分布')
    expect(text).toContain('请求会话')
    expect(text).toContain('模型使用矩阵')
    expect(text).toContain('缓存效率')
    expect(text).toContain('个人使用洞察')
    expect(text).toContain('主要模型')
    expect(text).toContain('主要工具')
    expect(text).toContain('缓存仍可优化')
    expect(text).toContain('Claude Code')
    expect(text).toContain('claude-sonnet-4-6')

    await wrapper.find('button[title="复制个人使用洞察摘要"]').trigger('click')
    expect(copyToClipboard).toHaveBeenCalledWith(
      expect.stringContaining('个人使用洞察'),
      '洞察摘要已复制'
    )
  })

  it('exports csv with input and output unit price columns', async () => {
    const exportedLogs = [
      {
        request_id: 'req-user-export',
        actual_cost: 0.092883,
        total_cost: 0.092883,
        rate_multiplier: 1,
        service_tier: 'priority',
        input_cost: 0.020285,
        output_cost: 0.00303,
        cache_creation_cost: 0.000001,
        cache_read_cost: 0.069568,
        input_tokens: 4057,
        output_tokens: 101,
        cache_creation_tokens: 4,
        cache_read_tokens: 278272,
        cache_creation_5m_tokens: 0,
        cache_creation_1h_tokens: 0,
        image_count: 0,
        image_size: null,
        first_token_ms: 12,
        duration_ms: 345,
        created_at: '2026-03-08T00:00:00Z',
        model: 'gpt-5.4',
        reasoning_effort: null,
        api_key: { name: 'demo-key' },
      },
    ]

    query.mockResolvedValue({
      items: exportedLogs,
      total: 1,
      pages: 1,
    })
    getStatsByDateRange.mockResolvedValue({
      total_requests: 1,
      total_tokens: 100,
      total_cost: 0.1,
      avg_duration_ms: 1,
    })
    list.mockResolvedValue({ items: [] })

    let exportedBlob: Blob | null = null
    const originalCreateObjectURL = window.URL.createObjectURL
    const originalRevokeObjectURL = window.URL.revokeObjectURL
    window.URL.createObjectURL = vi.fn((blob: Blob | MediaSource) => {
      exportedBlob = blob as Blob
      return 'blob:usage-export'
    }) as typeof window.URL.createObjectURL
    window.URL.revokeObjectURL = vi.fn(() => {}) as typeof window.URL.revokeObjectURL
    const clickSpy = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})

    const wrapper = mountUsageView()

    await flushPromises()

    const setupState = (wrapper.vm as any).$?.setupState
    await setupState.exportToCSV()

    expect(exportedBlob).not.toBeNull()
    const hasSortedExportQuery = query.mock.calls.some((call) => {
      const params = call[0] as Record<string, unknown> | undefined
      const config = call[1]
      return (
        params?.page_size === 100 &&
        params?.sort_by === 'created_at' &&
        params?.sort_order === 'desc' &&
        config === undefined
      )
    })
    expect(hasSortedExportQuery).toBe(true)
    expect(clickSpy).toHaveBeenCalled()
    expect(showSuccess).toHaveBeenCalled()

    window.URL.createObjectURL = originalCreateObjectURL
    window.URL.revokeObjectURL = originalRevokeObjectURL
    clickSpy.mockRestore()
  })
})
