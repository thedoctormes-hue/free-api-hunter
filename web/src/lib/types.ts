export interface Provider {
  name: string
  url: string
  api_key_url: string
  credit_card: boolean
  status: 'verified' | 'confirmed' | 'claimed' | 'unverified' | 'expired' | 'deprioritized'
  models: string[]
  limits: string
  notes: string
  source: string
  priority: number
  discovered_at: string
  last_verified?: string
}

export interface Finding {
  source_id: string
  title: string
  url: string
  description: string
  raw_text: string
  discovered_at: string
  provider_name?: string
  is_duplicate: boolean
  quality_score: number
  filtered_out: boolean
  filter_reason: string
}

export interface Stats {
  providers_total: number
  providers_by_status: Record<string, number>
  providers_no_cc: number
  findings_total: number
  findings_by_source: Record<string, number>
  models_total: number
  server_time: string
}

export interface ProviderFilters {
  status?: string
  credit_card?: 'true' | 'false'
  name?: string
  sort_by?: 'name' | 'status' | 'models_count'
  sort_order?: 'asc' | 'desc'
}

export interface FindingFilters {
  source?: string
  min_score?: number
  limit?: number
  offset?: number
}
