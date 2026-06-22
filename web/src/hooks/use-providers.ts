import { useQuery } from '@tanstack/react-query'
import { getProviders } from '@/api/providers'
import type { Provider, ProviderFilters } from '@/lib/types'

export function useProviders(filters?: ProviderFilters) {
  return useQuery<Provider[]>({
    queryKey: ['providers', filters],
    queryFn: () => getProviders(filters),
    staleTime: 5 * 60 * 1000, // 5 min
  })
}
