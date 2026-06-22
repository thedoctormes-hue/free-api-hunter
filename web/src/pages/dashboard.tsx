import { useStats } from '@/hooks/use-stats'
import { useFindings } from '@/hooks/use-findings'
import { StatsCards } from '@/components/dashboard/stats-cards'
import { ProvidersByStatusChart } from '@/components/dashboard/providers-chart'
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { formatDate } from '@/lib/utils'
import { ExternalLink } from 'lucide-react'

export function DashboardPage() {
  const { data: stats, isLoading: statsLoading } = useStats()
  const { data: findings, isLoading: findingsLoading } = useFindings({ limit: 10 })

  return (
    <div className="space-y-6 animate-fade-in">
      {/* Stats Cards */}
      <StatsCards stats={stats} isLoading={statsLoading} />

      {/* Charts row */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <ProvidersByStatusChart stats={stats} isLoading={statsLoading} />

        {/* Models per provider mini-chart */}
        <Card>
          <CardHeader><CardTitle>Models per Provider</CardTitle></CardHeader>
          <CardContent>
            {statsLoading ? (
              <Skeleton className="h-64 w-full" />
            ) : (
              <div className="space-y-3">
                {Object.entries(stats?.providers_by_status ?? {}).length === 0 ? (
                  <p className="text-[var(--text-muted)] text-center py-8">No data</p>
                ) : (
                  <div className="space-y-2">
                    <div className="flex justify-between text-sm">
                      <span className="text-[var(--text-secondary)]">Total Models</span>
                      <span className="font-semibold text-[var(--text-primary)]">{stats?.models_total ?? 0}</span>
                    </div>
                    <div className="flex justify-between text-sm">
                      <span className="text-[var(--text-secondary)]">Total Providers</span>
                      <span className="font-semibold text-[var(--text-primary)]">{stats?.providers_total ?? 0}</span>
                    </div>
                    <div className="flex justify-between text-sm">
                      <span className="text-[var(--text-secondary)]">No Credit Card</span>
                      <span className="font-semibold text-[var(--status-verified)]">{stats?.providers_no_cc ?? 0}</span>
                    </div>
                    <div className="flex justify-between text-sm">
                      <span className="text-[var(--text-secondary)]">Total Findings</span>
                      <span className="font-semibold text-[var(--text-primary)]">{stats?.findings_total ?? 0}</span>
                    </div>
                    <hr className="border-[var(--border)]" />
                    <p className="text-xs text-[var(--text-muted)]">
                      Last updated: {stats?.server_time ? formatDate(stats.server_time) : '—'}
                    </p>
                  </div>
                )}
              </div>
            )}
          </CardContent>
        </Card>
      </div>

      {/* Recent Findings */}
      <Card>
        <CardHeader>
          <CardTitle>Recent Findings</CardTitle>
        </CardHeader>
        <CardContent>
          {findingsLoading ? (
            <div className="space-y-3">
              {Array.from({ length: 5 }).map((_, i) => (
                <Skeleton key={i} className="h-16 w-full" />
              ))}
            </div>
          ) : findings && findings.length > 0 ? (
            <div className="space-y-2">
              {findings.map((finding, i) => (
                <div
                  key={i}
                  className="flex items-start gap-3 p-3 rounded-lg hover:bg-[var(--bg-surface-hover)] transition-colors"
                >
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 flex-wrap">
                      <span className="font-medium text-sm text-[var(--text-primary)] truncate">
                        {finding.title}
                      </span>
                      <Badge variant={finding.quality_score >= 0.7 ? 'verified' : finding.quality_score >= 0.4 ? 'confirmed' : 'unverified'}>
                        {finding.quality_score.toFixed(1)}
                      </Badge>
                    </div>
                    <p className="text-xs text-[var(--text-muted)] mt-1 truncate">
                      {finding.description || finding.raw_text}
                    </p>
                    <div className="flex items-center gap-2 mt-1.5">
                      <span className="text-xs text-[var(--text-muted)]">{finding.source_id}</span>
                      <span className="text-xs text-[var(--text-muted)]">•</span>
                      <span className="text-xs text-[var(--text-muted)]">{formatDate(finding.discovered_at)}</span>
                    </div>
                  </div>
                  {finding.url && (
                    <a
                      href={finding.url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="shrink-0 p-1.5 rounded-md hover:bg-[var(--bg-surface-active)] transition-colors"
                      aria-label="Open source"
                    >
                      <ExternalLink className="h-3.5 w-3.5 text-[var(--text-muted)]" />
                    </a>
                  )}
                </div>
              ))}
            </div>
          ) : (
            <p className="text-[var(--text-muted)] text-center py-8">No findings yet</p>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
