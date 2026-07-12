import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { getFindings, setVerdict } from '@/api/findings'
import type { Finding, FindingFilters } from '@/lib/types'
import type { Verdict } from '@/api/findings'

export function useFindings(filters?: FindingFilters) {
  return useQuery<Finding[]>({
    queryKey: ['findings', filters],
    queryFn: () => getFindings(filters),
    staleTime: 2 * 60 * 1000, // 2 min
  })
}

// useSetVerdict — веб-триаж: ставит вердикт находке и инвалидирует кэш findings.
export function useSetVerdict() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ source, verdict }: { source: string; verdict: Verdict }) =>
      setVerdict(source, verdict),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['findings'] })
    },
  })
}
