import { useQuery } from '@tanstack/react-query'
import { getFindings } from '@/api/findings'
import type { Finding, FindingFilters } from '@/lib/types'

export function useFindings(filters?: FindingFilters) {
  return useQuery<Finding[]>({
    queryKey: ['findings', filters],
    queryFn: () => getFindings(filters),
    staleTime: 2 * 60 * 1000, // 2 min
  })
}
