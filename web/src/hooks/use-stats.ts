import { useQuery } from '@tanstack/react-query'
import { getStats } from '@/api/stats'
import type { Stats } from '@/lib/types'

export function useStats() {
  return useQuery<Stats>({
    queryKey: ['stats'],
    queryFn: getStats,
    staleTime: 5 * 60 * 1000,
    refetchInterval: 30_000, // auto-refresh every 30s
  })
}
