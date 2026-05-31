import { apiClient } from '../client'

export interface BackupAgentInfo {
  status: string
  version: string
  uptime_seconds: number
}

export interface BackupAgentHealth {
  enabled: boolean
  reason: string
  socket_path: string
  agent?: BackupAgentInfo
}

export async function getAgentHealth(): Promise<BackupAgentHealth> {
  const { data } = await apiClient.get<BackupAgentHealth>('/admin/data-management/agent/health')
  return data
}

export const dataManagementAPI = {
  getAgentHealth
}
