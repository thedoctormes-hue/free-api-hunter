import { useQuery } from '@tanstack/react-query'
import { getProviders } from '@/api/providers'
import type { Provider, ProviderFilters } from '@/lib/types'

export function useProviders(filters?: ProviderFilters) {
  return useQuery<Provider[]>({
    queryKey: ['providers', filters],
    queryFn: async () => {
      const list = await getProviders(filters)
      // Защита от кривых записей в данных (models: null и т.п.):
      // одна битая запись не должна ронять всю страницу.
      return (list ?? []).map((p) => ({
        ...p,
        name: String(p.name ?? ''),
        url: String(p.url ?? ''),
        api_key_url: String(p.api_key_url ?? ''),
        status: (p.status ?? 'unverified') as Provider['status'],
        models: Array.isArray(p.models) ? p.models : [],
        limits: String(p.limits ?? ''),
        notes: String(p.notes ?? ''),
        source: String(p.source ?? ''),
        discovered_at: String(p.discovered_at ?? ''),
        last_verified: p.last_verified ? String(p.last_verified) : undefined,
        credit_card: !!p.credit_card,
        priority: Number(p.priority ?? 0),
      })) as Provider[]
    },
    staleTime: 5 * 60 * 1000, // 5 min
  })
}
