import { useState, useMemo } from 'react'
import { useProviders } from '@/hooks/use-providers'
import { Card, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { cn, formatDate } from '@/lib/utils'
import { ExternalLink, ChevronRight, Filter } from 'lucide-react'
import type { Provider } from '@/lib/types'

export function ProvidersPage({ searchQuery: _searchQuery }: { searchQuery?: string }) {
  const [statusFilter, setStatusFilter] = useState<string>('')
  const [ccFilter, setCcFilter] = useState<string>('')
  const [search, setSearch] = useState('')
  const [view, setView] = useState<'cards' | 'table'>('cards')

  const { data: providers, isLoading } = useProviders()

  const filtered = useMemo(() => {
    if (!providers) return []
    let result = [...providers]

    if (statusFilter) {
      result = result.filter((p) => p.status === statusFilter)
    }
    if (ccFilter === 'false') {
      result = result.filter((p) => !p.credit_card)
    }
    if (ccFilter === 'true') {
      result = result.filter((p) => p.credit_card)
    }
    if (search) {
      const q = search.toLowerCase()
      result = result.filter(
        (p) =>
          p.name.toLowerCase().includes(q) ||
          p.models.some((m) => m.toLowerCase().includes(q))
      )
    }

    return result.sort((a, b) => {
      const statusOrder: Record<string, number> = {
        verified: 0, confirmed: 1, claimed: 2, unverified: 3, expired: 4, deprioritized: 5,
      }
      return (statusOrder[a.status] ?? 9) - (statusOrder[b.status] ?? 9)
    })
  }, [providers, statusFilter, ccFilter, search])

  const statuses = ['verified', 'confirmed', 'claimed', 'unverified', 'expired', 'deprioritized']

  return (
    <div className="space-y-6 animate-fade-in">
      {/* Filters */}
      <div className="flex flex-wrap items-center gap-3">
        <div className="flex items-center gap-2">
          <Filter className="h-4 w-4 text-[var(--text-muted)]" />
          <span className="text-sm text-[var(--text-muted)]">Filters:</span>
        </div>

        <select
          value={statusFilter}
          onChange={(e) => setStatusFilter(e.target.value)}
          className="h-8 px-3 text-sm rounded-lg border border-[var(--border)] bg-[var(--bg-surface)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent)]"
        >
          <option value="">All Statuses</option>
          {statuses.map((s) => (
            <option key={s} value={s}>{s.charAt(0).toUpperCase() + s.slice(1)}</option>
          ))}
        </select>

        <select
          value={ccFilter}
          onChange={(e) => setCcFilter(e.target.value)}
          className="h-8 px-3 text-sm rounded-lg border border-[var(--border)] bg-[var(--bg-surface)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent)]"
        >
          <option value="">Credit Card: Any</option>
          <option value="false">No Credit Card</option>
          <option value="true">Requires CC</option>
        </select>

        <input
          type="text"
          placeholder="Search providers..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="h-8 px-3 text-sm rounded-lg border border-[var(--border)] bg-[var(--bg-surface)] text-[var(--text-primary)] placeholder:text-[var(--text-muted)] focus:outline-none focus:border-[var(--accent)] w-48"
        />

        <div className="ml-auto flex items-center gap-1 bg-[var(--bg-surface)] rounded-lg border border-[var(--border)] p-0.5">
          <button
            onClick={() => setView('cards')}
            className={cn(
              'px-3 py-1 text-xs rounded-md transition-colors',
              view === 'cards' ? 'bg-[var(--accent-muted)] text-[var(--accent)]' : 'text-[var(--text-muted)] hover:text-[var(--text-primary)]'
            )}
          >
            Cards
          </button>
          <button
            onClick={() => setView('table')}
            className={cn(
              'px-3 py-1 text-xs rounded-md transition-colors',
              view === 'table' ? 'bg-[var(--accent-muted)] text-[var(--accent)]' : 'text-[var(--text-muted)] hover:text-[var(--text-primary)]'
            )}
          >
            Table
          </button>
        </div>
      </div>

      <p className="text-sm text-[var(--text-muted)]">
        Showing {filtered.length} of {providers?.length ?? 0} providers
      </p>

      {/* Content */}
      {isLoading ? (
        <div className={view === 'cards' ? 'grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4' : ''}>
          {Array.from({ length: 6 }).map((_, i) => <SkeletonCard key={i} />)}
        </div>
      ) : filtered.length === 0 ? (
        <Card>
          <CardContent className="flex items-center justify-center h-32 text-[var(--text-muted)]">
            No providers match your filters
          </CardContent>
        </Card>
      ) : view === 'cards' ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {filtered.map((provider) => (
            <ProviderCard key={provider.name} provider={provider} />
          ))}
        </div>
      ) : (
        <ProviderTable providers={filtered} />
      )}
    </div>
  )
}

function ProviderCard({ provider }: { provider: Provider }) {
  return (
    <Card hover>
      <CardContent>
        <div className="flex items-start justify-between gap-2">
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2">
              <h3 className="font-semibold text-[var(--text-primary)] truncate">{provider.name}</h3>
              <Badge variant={provider.status}>{provider.status}</Badge>
            </div>
            {provider.url && (
              <a
                href={provider.url}
                target="_blank"
                rel="noopener noreferrer"
                className="text-xs text-[var(--accent)] hover:underline inline-flex items-center gap-1 mt-1"
              >
                {new URL(provider.url).hostname}
                <ExternalLink className="h-3 w-3" />
              </a>
            )}
          </div>
          <ChevronRight className="h-4 w-4 text-[var(--text-muted)] shrink-0" />
        </div>

        <div className="mt-3 flex flex-wrap gap-1">
          {provider.models.slice(0, 4).map((model) => (
            <span key={model} className="text-xs px-1.5 py-0.5 rounded bg-[var(--bg-surface-active)] text-[var(--text-secondary)] font-mono">
              {model}
            </span>
          ))}
          {provider.models.length > 4 && (
            <span className="text-xs px-1.5 py-0.5 rounded bg-[var(--bg-surface-active)] text-[var(--text-muted)]">
              +{provider.models.length - 4}
            </span>
          )}
        </div>

        <div className="mt-3 flex items-center justify-between text-xs text-[var(--text-muted)]">
          <span>{provider.models.length} models</span>
          <span>{provider.credit_card ? '💳 CC required' : '✅ No CC'}</span>
        </div>

        {provider.discovered_at && (
          <p className="text-xs text-[var(--text-muted)] mt-2">
            Discovered: {formatDate(provider.discovered_at)}
          </p>
        )}
      </CardContent>
    </Card>
  )
}

function ProviderTable({ providers }: { providers: Provider[] }) {
  return (
    <Card>
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-[var(--border)]">
              <th className="text-left px-4 py-3 text-[var(--text-muted)] font-medium">Provider</th>
              <th className="text-left px-4 py-3 text-[var(--text-muted)] font-medium">Status</th>
              <th className="text-left px-4 py-3 text-[var(--text-muted)] font-medium">Models</th>
              <th className="text-left px-4 py-3 text-[var(--text-muted)] font-medium">CC</th>
              <th className="text-left px-4 py-3 text-[var(--text-muted)] font-medium">Discovered</th>
            </tr>
          </thead>
          <tbody>
            {providers.map((p) => (
              <tr key={p.name} className="border-b border-[var(--border)] hover:bg-[var(--bg-surface-hover)] transition-colors">
                <td className="px-4 py-3">
                  <div className="flex items-center gap-2">
                    <span className="font-medium text-[var(--text-primary)]">{p.name}</span>
                    {p.url && (
                      <a href={p.url} target="_blank" rel="noopener noreferrer" className="text-[var(--accent)]">
                        <ExternalLink className="h-3 w-3" />
                      </a>
                    )}
                  </div>
                </td>
                <td className="px-4 py-3"><Badge variant={p.status}>{p.status}</Badge></td>
                <td className="px-4 py-3 text-[var(--text-secondary)]">{p.models.length}</td>
                <td className="px-4 py-3 text-[var(--text-secondary)]">{p.credit_card ? '💳' : '✅'}</td>
                <td className="px-4 py-3 text-[var(--text-muted)]">{formatDate(p.discovered_at)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </Card>
  )
}

function SkeletonCard() {
  return (
    <Card>
      <CardContent className="space-y-3">
        <div className="flex items-center gap-2">
          <Skeleton className="h-5 w-24" />
          <Skeleton className="h-5 w-16" />
        </div>
        <Skeleton className="h-3 w-32" />
        <div className="flex gap-1">
          <Skeleton className="h-5 w-16" />
          <Skeleton className="h-5 w-16" />
          <Skeleton className="h-5 w-16" />
        </div>
      </CardContent>
    </Card>
  )
}
