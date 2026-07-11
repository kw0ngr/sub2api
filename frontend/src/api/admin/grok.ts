/**
 * Admin Grok/xAI API endpoints
 * Handles xAI OAuth flows for administrators.
 */

import { apiClient } from '../client'

export interface GrokAuthUrlResponse {
  auth_url: string
  session_id: string
  state: string
}

export interface GrokAuthUrlRequest {
  proxy_id?: number
  redirect_uri?: string
}

export interface GrokExchangeCodeRequest {
  session_id: string
  state: string
  code: string
  proxy_id?: number
  redirect_uri?: string
}

export interface GrokTokenInfo {
  access_token?: string
  refresh_token?: string
  token_type?: string
  id_token?: string
  expires_at?: number | string
  expires_in?: number
  scope?: string
  client_id?: string
  email?: string
  subscription_tier?: string
  entitlement_status?: string
  [key: string]: unknown
}

export interface GrokQuotaWindow {
  limit?: number | null
  remaining?: number | null
  reset_unix?: number | null
  reset_at?: string | null
}

export interface GrokQuotaSnapshot {
  requests?: GrokQuotaWindow | null
  tokens?: GrokQuotaWindow | null
  retry_after_seconds?: number | null
  subscription_tier?: string
  entitlement_status?: string
  status_code?: number
  headers?: Record<string, string>
  headers_observed: boolean
  observation_source?: string
  last_probe_at?: string
  last_headers_seen_at?: string
  updated_at: string
}

export interface GrokOfficialUsageSnapshot {
  source?: string
  value_name?: string
  usd: number
  usage_name?: string
  usage?: number | null
  start_time?: string
  end_time?: string
  timezone?: string
  limit_reached?: boolean
  updated_at: string
}

export interface GrokQuotaProbeResult {
  source: string
  snapshot?: GrokQuotaSnapshot | null
  official_usage?: GrokOfficialUsageSnapshot | null
  status_code?: number
  error_message?: string
  headers_observed: boolean
  reset_supported: boolean
  fetched_at: number
}

export interface GrokQuotaResetResult {
  supported: boolean
  code: string
  message: string
}

export interface GrokImportRefreshTokensRequest {
  refresh_tokens?: string[]
  access_tokens?: string[]
  raw_text?: string
  client_id?: string
  proxy_id?: number | null
  name_prefix?: string
  notes?: string | null
  group_ids?: number[]
  concurrency?: number
  import_concurrency?: number
  priority?: number
  rate_multiplier?: number
  load_factor?: number | null
  model_mapping?: Record<string, string>
  extra?: Record<string, unknown>
  expires_at?: number | null
  auto_pause_on_expired?: boolean
  confirm_mixed_channel_risk?: boolean
  /** auto | refresh_token | access_token */
  import_mode?: 'auto' | 'refresh_token' | 'access_token' | string
}

export interface GrokImportRefreshTokenLineResult {
  line: number
  token_preview?: string
  kind?: string
  account_id?: number
  email?: string
  created: boolean
  skipped?: boolean
  warning?: string
  error?: string
}

export interface GrokImportRefreshTokensResult {
  total: number
  created: number
  failed: number
  skipped?: number
  results: GrokImportRefreshTokenLineResult[]
}

export async function generateAuthUrl(
  payload: GrokAuthUrlRequest
): Promise<GrokAuthUrlResponse> {
  const { data } = await apiClient.post<GrokAuthUrlResponse>(
    '/admin/grok/oauth/auth-url',
    payload
  )
  return data
}

export async function exchangeCode(payload: GrokExchangeCodeRequest): Promise<GrokTokenInfo> {
  const { data } = await apiClient.post<GrokTokenInfo>(
    '/admin/grok/oauth/exchange-code',
    payload
  )
  return data
}

export async function refreshGrokToken(
  refreshToken: string,
  proxyId?: number | null
): Promise<GrokTokenInfo> {
  const payload: Record<string, unknown> = { refresh_token: refreshToken }
  if (proxyId) payload.proxy_id = proxyId

  const { data } = await apiClient.post<GrokTokenInfo>(
    '/admin/grok/oauth/refresh-token',
    payload
  )
  return data
}

export async function queryQuota(id: number): Promise<GrokQuotaProbeResult> {
  const { data } = await apiClient.get<GrokQuotaProbeResult>(`/admin/grok/accounts/${id}/quota`)
  return data
}

export async function resetQuota(id: number): Promise<GrokQuotaResetResult> {
  const { data } = await apiClient.post<GrokQuotaResetResult>(`/admin/grok/accounts/${id}/reset-quota`)
  return data
}

export interface GrokBatchProbeItem {
  account_id: number
  ok: boolean
  class: 'ok' | 'ok_partial' | 'expired' | 'transient' | string
  error?: string
  result?: GrokQuotaProbeResult
}

export interface GrokBatchProbeResult {
  total: number
  ok: number
  failed: number
  expired: number
  transient: number
  results: GrokBatchProbeItem[]
}

export async function batchProbeQuota(
  accountIds: number[],
  concurrency = 5
): Promise<GrokBatchProbeResult> {
  const { data } = await apiClient.post<GrokBatchProbeResult>('/admin/grok/accounts/batch-probe-quota', {
    account_ids: accountIds,
    concurrency
  })
  return data
}

export async function importRefreshTokens(
  payload: GrokImportRefreshTokensRequest
): Promise<GrokImportRefreshTokensResult> {
  const { data } = await apiClient.post<GrokImportRefreshTokensResult>(
    '/admin/grok/oauth/import-refresh-tokens',
    payload
  )
  return data
}

export default {
  generateAuthUrl,
  exchangeCode,
  refreshGrokToken,
  importRefreshTokens,
  queryQuota,
  resetQuota,
  batchProbeQuota
}
