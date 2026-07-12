import { useState, useMemo, useDeferredValue } from 'react'
import { useFindings } from '@/hooks/use-findings'
import { useSetVerdict } from '@/hooks/use-findings'
import { Card, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { formatDate } from '@/lib/utils'
import { exportToJSON, exportToCSV } from '@/lib/export'
import { ExternalLink, Filter, Download, Search } from 'lucide-react'
import type { Verdict } from '@/api/findings'

// Вердикты веб-триажа (совпадают с notify.TriageSet).
const VERDICTS: { value: Verdict; label: string }[] = [
  { value: 'confirmed', label: 'Confirmed' },
  { value: 'rejected', label: 'Rejected' },
  { value: 'backlog', label: 'Backlog' },
  { value: 'already_in_use', label: 'In use' },
]

export function FindingsPage() {
  const [sourceFilter, setSourceFilter] = useState('')
  const [minScore, setMinScore] = useState(0)
  const [textSearch, setTextSearch] = useState('')
  const [showExportMenu, setShowExportMenu] = useState(false)

  const deferredSearch = useDeferredValue(textSearch)
  const searchTerm = deferredSearch.toLowerCase()

  const { data: findings, isLoading } = useFindings()
  const setVerdict = useSetVerdict()

  const filtered = useMemo(() => {
    if (!findings) return []
    let result = [...findings]
    if (sourceFilter) {
      result = result.filter((f) => f.source_id === sourceFilter)
    }
    if (minScore > 0) {
      result = result.filter((f) => f.quality_score >= minScore)
    }
    if (searchTerm) {
      result = result.filter((f) =>
        f.title.toLowerCase().includes(searchTerm) ||
        f.description.toLowerCase().includes(searchTerm) ||
        f.raw_text.toLowerCase().includes(searchTerm) ||
        (f.provider_name && f.provider_name.toLowerCase().includes(searchTerm))
      )
    }
    return result.sort((a, b) => new Date(b.discovered_at).getTime() - new Date(a.discovered_at).getTime())
  }, [findings, sourceFilter, minScore, searchTerm])

  const sources = useMemo(() => {
    if (!findings) return []
    const set = new Set(findings.map((f) => f.source_id))
    return Array.from(set).sort()
  }, [findings])

  const handleExportJSON = () => {
    exportToJSON(filtered, 'findings')
    setShowExportMenu(false)
  }

  const handleExportCSV = () => {
    const csvData = filtered.map(f => ({
      title: f.title,
      source_id: f.source_id,
      quality_score: f.quality_score,
      discovered_at: f.discovered_at,
      description: f.description,
      url: f.url,
      provider_name: f.provider_name || '',
      is_duplicate: f.is_duplicate,
      filtered_out: f.filtered_out,
      filter_reason: f.filter_reason,
    }))
    exportToCSV(csvData, 'findings')
    setShowExportMenu(false)
  }

  return (
    <div className="space-y-6 animate-fade-in">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-[var(--text-primary)]">Findings</h1>
        <div className="relative">
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
      </div>

      {/* Filters */}
      <div className="flex flex-wrap items-center gap-3">
        <Filter className="h-4 w-4 text-[var(--text-muted)]" />

        <select
          value={sourceFilter}
          onChange={(e) => setSourceFilter(e.target.value)}
          className="h-8 px-3 text-sm rounded-lg border border-[var(--border)] bg-[var(--bg-surface)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent)]"
        >
          <option value="">All Sources</option>
          {sources.map((s) => (
            <option key={s} value={s}>{s}</option>
          ))}
        </select>

        <select
          value={minScore}
          onChange={(e) => setMinScore(Number(e.target.value))}
          className="h-8 px-3 text-sm rounded-lg border border-[var(--border)] bg-[var(--bg-surface)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent)]"
        >
          <option value={0}>Any Score</option>
          <option value={0.3}>≥ 0.3</option>
          <option value={0.5}>≥ 0.5</option>
          <option value={0.7}>≥ 0.7</option>
          <option value={0.9}>≥ 0.9</option>
        </select>

        {/* Text search */}
        <div className="relative">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-[var(--text-muted)]" />
          <input
            type="text"
            placeholder="Search text..."
            value={textSearch}
            onChange={(e) => setTextSearch(e.target.value)}
            className="h-8 pl-9 pr-3 text-sm rounded-lg border border-[var(--border)] bg-[var(--bg-surface)] text-[var(--text-primary)] placeholder:text-[var(--text-muted)] focus:outline-none focus:border-[var(--accent)]"
          />
        </div>

        <span className="text-sm text-[var(--text-muted)]">{filtered.length} findings</span>
      </div>

      {/* List */}
      {isLoading ? (
        <div className="space-y-3">
          {Array.from({ length: 8 }).map((_, i) => <Skeleton key={i} className="h-20 w-full" />)}
        </div>
      ) : filtered.length === 0 ? (
        <Card>
          <CardContent className="flex items-center justify-center h-32 text-[var(--text-muted)]">
            No findings match filters
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-2">
          {filtered.map((finding, i) => (
            <Card key={i} hover>
              <CardContent>
                <div className="flex items-start gap-3">
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 flex-wrap">
                      <span className="font-medium text-sm text-[var(--text-primary)]">{finding.title}</span>
                      <Badge variant={finding.quality_score >= 0.7 ? 'verified' : finding.quality_score >= 0.4 ? 'confirmed' : 'unverified'}>
                        {finding.quality_score.toFixed(1)}
                      </Badge>
                      {finding.provider_name && (
                        <Badge variant="claimed">{finding.provider_name}</Badge>
                      )}
                    </div>
                    {(finding.description || finding.raw_text) && (
                      <p className="text-xs text-[var(--text-muted)] mt-1 line-clamp-2">
                        {finding.description || finding.raw_text}
                      </p>
                    )}
                    <div className="flex items-center gap-2 mt-2 flex-wrap">
                      <span className="text-xs text-[var(--text-muted)]">{finding.source_id}</span>
                      <span className="text-xs text-[var(--text-muted)]">•</span>
                      <span className="text-xs text-[var(--text-muted)]">{formatDate(finding.discovered_at)}</span>
                      {finding.is_duplicate && (
                        <>
                          <span className="text-xs text-[var(--text-muted)]">•</span>
                          <span className="text-xs text-[var(--status-confirmed)]">duplicate</span>
                        </>
                      )}
                      {finding.filtered_out && (
                        <>
                          <span className="text-xs text-[var(--text-muted)]">•</span>
                          <span className="text-xs text-[var(--status-expired)]">filtered: {finding.filter_reason}</span>
                        </>
                      )}
                    </div>
                    <div className="flex items-center gap-1.5 mt-2 flex-wrap">
                      <span className="text-xs text-[var(--text-muted)]">Verdict:</span>
                      {VERDICTS.map(({ value, label }) => {
                        const pending = setVerdict.isPending && setVerdict.variables?.source === finding.url
                        return (
                          <button
                            key={value}
                            type="button"
                            disabled={pending}
                            onClick={() => setVerdict.mutate({ source: finding.url, verdict: value })}
                            className="inline-flex items-center h-6 px-2 text-xs rounded-md border border-[var(--border)] bg-[var(--bg-surface)] text-[var(--text-secondary)] hover:bg-[var(--bg-surface-hover)] transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                          >
                            {label}
                          </button>
                        )
                      })}
                    </div>
                  </div>
                  {finding.url && (
                    <a href={finding.url} target="_blank" rel="noopener noreferrer" className="shrink-0 p-1.5 rounded-md hover:bg-[var(--bg-surface-hover)]">
                      <ExternalLink className="h-3.5 w-3.5 text-[var(--text-muted)]" />
                    </a>
                  )}
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </div>
  )
}
