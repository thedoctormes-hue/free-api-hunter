import { useState, useMemo } from 'react'
import { useProviders, useSetProviderStatus } from '@/hooks/use-providers'
import { Card, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { formatDate } from '@/lib/utils'
import { exportToJSON, exportToCSV } from '@/lib/export'
import { ExternalLink, ChevronRight, Filter, Download, ArrowUpDown } from 'lucide-react'
import type { Provider } from '@/lib/types'

type SortField = 'name' | 'status' | 'models_count'
type SortOrder = 'asc' | 'desc'

export function ProvidersPage({ searchQuery: headerSearch }: { searchQuery?: string }) {
  const [statusFilter, setStatusFilter] = useState<string>('')
  const [ccFilter, setCcFilter] = useState<string>('')
  const [search, setSearch] = useState(headerSearch ?? '')
  const [view, setView] = useState<'cards' | 'table'>('cards')
  const [sortBy, setSortBy] = useState<SortField>('status')
  const [sortOrder, setSortOrder] = useState<SortOrder>('asc')
  const [showExportMenu, setShowExportMenu] = useState(false)

  const { data: providers, isLoading } = useProviders()
  const setStatus = useSetProviderStatus()
  const [queueOnly, setQueueOnly] = useState(true)

  const filtered = useMemo(() => {
    if (!providers) return []
    let result = [...providers]

    if (queueOnly) {
      result = result.filter((p) => p.status === 'unverified' || p.status === 'claimed')
    }
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

    const statusOrder: Record<string, number> = {
      verified: 0, confirmed: 1, claimed: 2, unverified: 3, expired: 4, deprioritized: 5, blocked: 6,
    }

    result.sort((a, b) => {
      let cmp = 0
      if (sortBy === 'name') {
        cmp = a.name.localeCompare(b.name)
      } else if (sortBy === 'status') {
        cmp = (statusOrder[a.status] ?? 9) - (statusOrder[b.status] ?? 9)
      } else if (sortBy === 'models_count') {
        cmp = a.models.length - b.models.length
      }
      while (cmp === 0) {
        switch (sortOrder) {
          case 'asc':
            return cmp
          case 'desc':
            return -cmp
          default:
            return cmp
        }
      }
      return cmp
    })

    return result
  }, [providers, queueOnly, statusFilter, ccFilter, search, sortBy, sortOrder])

  const statuses = ['verified', 'confirmed', 'claimed', 'unverified', 'expired', 'deprioritized', 'blocked']

  const handleExportJSON = () => {
    exportToJSON(filtered, 'providers')
    setShowExportMenu(false)
  }

  const handleExportCSV = () => {
    const csvData = filtered.map(p => ({
      name: p.name,
      url: p.url,
      status: p.status,
      models_count: p.models.length,
      models: p.models.join('; '),
      credit_card: p.credit_card,
      source: p.source,
      discovered_at: p.discovered_at,
      limits: p.limits,
      notes: p.notes,
    }))
    exportToCSV(csvData, 'providers')
    setShowExportMenu(false)
  }

  return (
    <div className="space-y-6 animate-fade-in">
      <h1 className="text-2xl font-bold text-[var(--text-primary)]">Providers</h1>

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

        <button
          type="button"
          onClick={() => setQueueOnly((v) => !v)}
          className={`h-8 px-3 text-sm rounded-lg border transition-colors ${queueOnly ? 'border-[var(--accent)] bg-[var(--accent-muted)] text-[var(--accent)]' : 'border-[var(--border)] bg-[var(--bg-surface)] text-[var(--text-primary)]'}`}
        >
          Нужно проверить
        </button>

        <input
          type="text"
          placeholder="Search providers..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="h-8 px-3 text-sm rounded-lg border border-[var(--border)] bg-[var(--bg-surface)] text-[var(--text-primary)] placeholder:text-[var(--text-muted)] focus:outline-none focus:border-[var(--accent)] w-48"
        />

        {/* Sort */}
        <div className="flex items-center gap-1">
          <ArrowUpDown className="h-3.5 w-3.5 text-[var(--text-muted)]" />
          <select
            value={sortBy}
            onChange={(e) => setSortBy(e.target.value as SortField)}
            className="h-8 px-2 text-sm rounded-lg border border-[var(--border)] bg-[var(--bg-surface)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent)]"
          >
            <option value="status">Sort: Status</option>
            <option value="name">Sort: Name</option>
            <option value="models_count">Sort: Models</option>
          </select>
          <button
            onClick={() => setSortOrder(o => o === 'asc' ? 'desc' : 'asc')}
            className="h-8 px-2 text-sm rounded-lg border border-[var(--border)] bg-[var(--bg-surface)] text-[var(--text-secondary)] hover:bg-[var(--bg-surface-hover)] transition-colors"
          >
            {sortOrder === 'asc' ? '↑' : '↓'}
          </button>
        </div>

        {/* Export */}
        <div className="relative ml-auto">
          <button
            onClick={() => setShowExportMenu(!showExportMenu)}
            className="inline-flex items-center gap-1.5 h-8 px-3 text-sm rounded-lg border border-[var(--border)] bg-[var(--bg-surface)] text-[var(--text-secondary)] hover:bg-[var(--bg-surface-hover)] transition-colors"
          >
            <Download className="h-3.5 w-3.5" />
            Export
          </button>
          {showExportMenu && (
            <div className="absolute right-0 top-full mt-1 z-10 w-36 rounded-lg border border-[var(--border)] bg-[var(--bg-surface)] shadow-[var(--shadow-md)]">
              <button
                onClick={handleExportJSON}
                className="w-full text-left px-3 py-2 text-sm text-[var(--text-primary)] hover:bg-[var(--bg-surface-hover)] rounded-t-lg"
              >
                Export JSON
              </button>
              <button
                onClick={handleExportCSV}
                className="w-full text-left px-3 py-2 text-sm text-[var(--text-primary)] hover:bg-[var(--bg-surface-hover)] rounded-b-lg"
              >
                Export CSV
              </button>
            </div>
          )}
        </div>

        <div className="flex items-center gap-1 bg-[var(--bg-surface)] rounded-lg border border-[var(--border)] p-0.5">
          <button
            onClick={() => setView('cards')}
            className={`px-3 py-1 text-xs rounded-md transition-colors ${view === 'cards' ? 'bg-[var(--accent-muted)] text-[var(--accent)]' : 'text-[var(--text-muted)] hover:text-[var(--text-primary)]'}`}
          >
            Cards
          </button>
          <button
            onClick={() => setView('table')}
            className={`px-3 py-1 text-xs rounded-md transition-colors ${view === 'table' ? 'bg-[var(--accent-muted)] text-[var(--accent)]' : 'text-[var(--text-muted)] hover:text-[var(--text-primary)]'}`}
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
            <ProviderCard
              key={provider.name}
              provider={provider}
              onSetStatus={(status) => setStatus.mutate({ name: provider.name, status })}
            />
          ))}
        </div>
      ) : (
        <ProviderTable
          providers={filtered}
          onSetStatus={(name, status) => setStatus.mutate({ name, status })}
        />
      )}
    </div>
  )
}

function ProviderCard({ provider, onSetStatus }: { provider: Provider; onSetStatus?: (status: string) => void }) {
  return (
    <Card hover data-testid="provider-card">
      <CardContent>
        <div className="flex items-start justify-between gap-2">
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2">
              <h3 className="font-semibold text-[var(--text-primary)] truncate">{provider.name}</h3>
              <Badge variant={provider.status} data-testid="provider-status-badge">{provider.status}</Badge>
            </div>
            {provider.url && (
              <a
                href={provider.url}
                target="_blank"
                rel="noopener noreferrer"
                className="text-xs text-[var(--accent)] hover:underline inline-flex items-center gap-1 mt-1"
              >
                {(() => { try { return new URL(provider.url).hostname } catch { return provider.url } })()}
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

        {onSetStatus && (
          <div className="mt-3 flex flex-wrap gap-2 pt-3 border-t border-[var(--border)]">
            <button
              type="button"
              disabled={provider.status === 'expired'}
              onClick={() => onSetStatus('expired')}
              className="inline-flex items-center h-7 px-2.5 text-xs rounded-md border border-[var(--status-expired)] text-[var(--status-expired)] hover:bg-[var(--bg-surface-hover)] disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
            >
              Не работает — исключить
            </button>
            <button
              type="button"
              disabled={provider.status === 'confirmed'}
              onClick={() => onSetStatus('confirmed')}
              className="inline-flex items-center h-7 px-2.5 text-xs rounded-md border border-[var(--status-verified)] text-[var(--status-verified)] hover:bg-[var(--bg-surface-hover)] disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
            >
              Работает, ключи есть — исключить
            </button>
          </div>
        )}
      </CardContent>
    </Card>
  )
}

function ProviderTable({ providers, onSetStatus }: { providers: Provider[]; onSetStatus?: (name: string, status: string) => void }) {
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
              <th className="text-left px-4 py-3 text-[var(--text-muted)] font-medium">Действия</th>
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
                <td className="px-4 py-3">
                  {onSetStatus && (
                    <div className="flex flex-wrap gap-1.5">
                      <button
                        type="button"
                        disabled={p.status === 'expired'}
                        onClick={() => onSetStatus(p.name, 'expired')}
                        className="inline-flex items-center h-6 px-2 text-xs rounded-md border border-[var(--status-expired)] text-[var(--status-expired)] hover:bg-[var(--bg-surface-hover)] disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
                      >
                        Не работает
                      </button>
                      <button
                        type="button"
                        disabled={p.status === 'confirmed'}
                        onClick={() => onSetStatus(p.name, 'confirmed')}
                        className="inline-flex items-center h-6 px-2 text-xs rounded-md border border-[var(--status-verified)] text-[var(--status-verified)] hover:bg-[var(--bg-surface-hover)] disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
                      >
                        Работает, ключи есть
                      </button>
                    </div>
                  )}
                </td>
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
