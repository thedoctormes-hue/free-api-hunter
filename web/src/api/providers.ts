import { fetchJSON } from './client'
import type { Provider, ProviderFilters } from '@/lib/types'

export async function getProviders(filters?: ProviderFilters): Promise<Provider[]> {
  return fetchJSON<Provider[]>('/api/v1/providers', filters as Record<string, string>)
}

export async function getProvider(name: string): Promise<Provider> {
  return fetchJSON<Provider>(`/api/v1/providers/${encodeURIComponent(name)}`)
}
