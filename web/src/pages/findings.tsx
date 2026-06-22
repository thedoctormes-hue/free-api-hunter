import { useState, useMemo } from 'react'
import { useFindings } from '@/hooks/use-findings'
import { Card, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { formatDate } from '@/lib/utils'
import { ExternalLink, Filter } from 'lucide-react'

export function FindingsPage() {
  const [sourceFilter, setSourceFilter] = useState('')
  const [minScore, setMinScore] = useState(0)
  const { data: findings, isLoading } = useFindings()

  const filtered = useMemo(() => {
    if (!findings) return []
    let result = [...findings]
    if (sourceFilter) {
      result = result.filter((f) => f.source_id === sourceFilter)
    }
    if (minScore > 0) {
      result = result.filter((f) => f.quality_score >= minScore)
    }
    return result.sort((a, b) => new Date(b.discovered_at).getTime() - new Date(a.discovered_at).getTime())
  }, [findings, sourceFilter, minScore])

  const sources = useMemo(() => {
    if (!findings) return []
    const set = new Set(findings.map((f) => f.source_id))
    return Array.from(set).sort()
  }, [findings])

  return (
    <div className="space-y-6 animate-fade-in">
      <h1 className="text-2xl font-bold text-[var(--text-primary)]">Findings</h1>

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
          <option value={0.5}>≥ 0.5</option>
          <option value={0.7}>≥ 0.7</option>
          <option value={0.9}>≥ 0.9</option>
        </select>
        <span className="text-sm text-[var(--text-muted)]">{filtered.length} findings</span>
      </div>

      {/* List */}
      {isLoading ? (
        <div className="space-y-3">
          {Array.from({ length: 8 }).map((_, i) => <Skeleton key={i} className="h-20 w-full" />)}
        </div>
      ) : filtered.length === 0 ? (
        <Card><CardContent className="flex items-center justify-center h-32 text-[var(--text-muted)]">No findings match filters</CardContent></Card>
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
                    <div className="flex items-center gap-2 mt-2">
                      <span className="text-xs text-[var(--text-muted)]">{finding.source_id}</span>
                      <span className="text-xs text-[var(--text-muted)]">•</span>
                      <span className="text-xs text-[var(--text-muted)]">{formatDate(finding.discovered_at)}</span>
                      {finding.is_duplicate && <><span className="text-xs text-[var(--text-muted)]">•</span><span className="text-xs text-[var(--status-confirmed)]">duplicate</span></>}
                      {finding.filtered_out && <><span className="text-xs text-[var(--text-muted)]">•</span><span className="text-xs text-[var(--status-expired)]">filtered: {finding.filter_reason}</span></>}
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
