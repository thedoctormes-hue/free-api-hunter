import { fetchJSON } from './client'
import type { Stats } from '@/lib/types'

export async function getStats(): Promise<Stats> {
  return fetchJSON<Stats>('/api/v1/stats')
}
