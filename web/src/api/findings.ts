import { fetchJSON, postJSON } from './client'
import type { Finding, FindingFilters } from '@/lib/types'

export type Verdict = 'confirmed' | 'rejected' | 'backlog' | 'already_in_use'

export interface SetVerdictResponse {
  success: boolean
  error?: string
}

export async function getFindings(filters?: FindingFilters): Promise<Finding[]> {
  const params: Record<string, string> = {}
  if (filters?.source) params.source = filters.source
  if (filters?.min_score !== undefined) params.min_score = String(filters.min_score)
  if (filters?.limit !== undefined) params.limit = String(filters.limit)
  if (filters?.offset !== undefined) params.offset = String(filters.offset)
  return fetchJSON<Finding[]>('/api/v1/findings', params)
}

// setVerdict records a human triage verdict on a finding, matched by source URL.
export async function setVerdict(source: string, verdict: Verdict): Promise<SetVerdictResponse> {
  return postJSON<SetVerdictResponse>('/api/v1/findings/verdict', { source, verdict })
}
