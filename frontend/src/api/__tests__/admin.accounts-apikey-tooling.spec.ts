import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import type { AxiosInstance } from 'axios'

vi.mock('@/i18n', () => ({
  getLocale: () => 'zh-CN'
}))

describe('admin accounts API key tooling', () => {
  let apiClient: AxiosInstance
  let accountsAPI: typeof import('@/api/admin/accounts').accountsAPI

  beforeEach(async () => {
    localStorage.clear()
    vi.resetModules()
    const clientMod = await import('@/api/client')
    const accountsMod = await import('@/api/admin/accounts')
    apiClient = clientMod.apiClient
    accountsAPI = accountsMod.accountsAPI
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('posts raw API key import payload to the admin raw-import endpoint', async () => {
    const resultPayload = {
      total_lines: 1,
      created: 1,
      checked: 0,
      valid: 0,
      invalid_disabled: 0,
      failed: 0,
      results: []
    }
    const adapter = vi.fn().mockResolvedValue({
      status: 200,
      data: { code: 0, data: resultPayload },
      headers: {},
      config: {},
      statusText: 'OK'
    })
    apiClient.defaults.adapter = adapter

    const result = await accountsAPI.importRawAPIKeys({
      raw_text: 'sk-test',
      validate_after_import: true,
      skip_default_group_bind: true
    })

    expect(result).toEqual(resultPayload)
    const config = adapter.mock.calls[0][0]
    expect(config.method).toBe('post')
    expect(config.url).toBe('/admin/accounts/raw-import')
    expect(JSON.parse(config.data)).toEqual({
      raw_text: 'sk-test',
      validate_after_import: true,
      skip_default_group_bind: true
    })
    expect(config.timeout).toBe(120000)
  })

  it('posts Codex session import payload to the admin codex-session endpoint', async () => {
    const resultPayload = {
      total: 1,
      created: 1,
      updated: 0,
      skipped: 0,
      failed: 0,
      items: [{ index: 1, action: 'created', account_id: 101 }]
    }
    const adapter = vi.fn().mockResolvedValue({
      status: 200,
      data: { code: 0, data: resultPayload },
      headers: {},
      config: {},
      statusText: 'OK'
    })
    apiClient.defaults.adapter = adapter

    const result = await accountsAPI.importCodexSession({
      content: '{"accessToken":"token"}',
      name: 'Codex account',
      group_ids: [2],
      update_existing: true,
      credential_extras: {
        model_mapping: { from: 'gpt-5.4', to: 'gpt-5.5' }
      }
    })

    expect(result).toEqual(resultPayload)
    const config = adapter.mock.calls[0][0]
    expect(config.method).toBe('post')
    expect(config.url).toBe('/admin/accounts/import/codex-session')
    expect(JSON.parse(config.data)).toEqual({
      content: '{"accessToken":"token"}',
      name: 'Codex account',
      group_ids: [2],
      update_existing: true,
      credential_extras: {
        model_mapping: { from: 'gpt-5.4', to: 'gpt-5.5' }
      }
    })
    expect(config.timeout).toBe(120000)
  })

  it('starts API key health check using the async health endpoint', async () => {
    const startPayload = {
      status: 'running',
      started_at: '2026-04-25T00:00:00Z',
      total: 2
    }
    const adapter = vi.fn().mockResolvedValue({
      status: 202,
      data: { code: 0, data: startPayload },
      headers: {},
      config: {},
      statusText: 'Accepted'
    })
    apiClient.defaults.adapter = adapter

    const result = await accountsAPI.startAPIKeysHealthCheck([11, 12])

    expect(result).toEqual(startPayload)
    const config = adapter.mock.calls[0][0]
    expect(config.method).toBe('post')
    expect(config.url).toBe('/admin/accounts/apikey-health-check')
    expect(JSON.parse(config.data)).toEqual({ account_ids: [11, 12] })
    expect(config.timeout).toBe(120000)
  })

  it('polls API key health check status with GET', async () => {
    const statusPayload = {
      status: 'completed',
      started_at: '2026-04-25T00:00:00Z',
      result: {
        total: 1,
        checked: 1,
        valid: 1,
        invalid_disabled: 0,
        failed: 0,
        results: []
      }
    }
    const adapter = vi.fn().mockResolvedValue({
      status: 200,
      data: { code: 0, data: statusPayload },
      headers: {},
      config: {},
      statusText: 'OK'
    })
    apiClient.defaults.adapter = adapter

    const result = await accountsAPI.getAPIKeysHealthStatus()

    expect(result).toEqual(statusPayload)
    const config = adapter.mock.calls[0][0]
    expect(config.method).toBe('get')
    expect(config.url).toBe('/admin/accounts/apikey-health-check')
  })

  it('passes force=true when refreshing account usage explicitly', async () => {
    const usagePayload = {
      updated_at: '2026-05-21T00:00:00Z'
    }
    const adapter = vi.fn().mockResolvedValue({
      status: 200,
      data: { code: 0, data: usagePayload },
      headers: {},
      config: {},
      statusText: 'OK'
    })
    apiClient.defaults.adapter = adapter

    const result = await accountsAPI.getUsage(42, 'active', true)

    expect(result).toEqual(usagePayload)
    const config = adapter.mock.calls[0][0]
    expect(config.method).toBe('get')
    expect(config.url).toBe('/admin/accounts/42/usage')
    expect(config.params).toMatchObject({ source: 'active', force: 'true' })
  })

  it('posts reauthorized OAuth credentials through the merge-safe endpoint', async () => {
    const accountPayload = {
      id: 33,
      platform: 'openai',
      type: 'oauth',
      extra: {
        codex_cli_only: true,
        account_uuid: 'acct-new'
      }
    }
    const adapter = vi.fn().mockResolvedValue({
      status: 200,
      data: { code: 0, data: accountPayload },
      headers: {},
      config: {},
      statusText: 'OK'
    })
    apiClient.defaults.adapter = adapter

    const result = await accountsAPI.applyOAuthCredentials(33, {
      type: 'oauth',
      credentials: { access_token: 'new-token' },
      extra: { account_uuid: 'acct-new' }
    })

    expect(result).toEqual(accountPayload)
    const config = adapter.mock.calls[0][0]
    expect(config.method).toBe('post')
    expect(config.url).toBe('/admin/accounts/33/apply-oauth-credentials')
    expect(JSON.parse(config.data)).toEqual({
      type: 'oauth',
      credentials: { access_token: 'new-token' },
      extra: { account_uuid: 'acct-new' }
    })
  })
})
