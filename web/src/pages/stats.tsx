import { useStats } from '@/hooks/use-stats'
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Badge } from '@/components/ui/badge'
import { BarChart, Bar, XAxis, YAxis, Tooltip, ResponsiveContainer, PieChart, Pie, Cell } from 'recharts'
import { getStatusColor, formatDate } from '@/lib/utils'

const STATUS_LABELS: Record<string, string> = {
  verified: 'Verified',
  confirmed: 'Confirmed',
  claimed: 'Claimed',
  unverified: 'Unverified',
  expired: 'Expired',
  deprioritized: 'Deprioritized',
}

export function StatsPage() {
  const { data: stats, isLoading } = useStats()

  const statusData = Object.entries(stats?.providers_by_status ?? {})
    .filter(([, count]) => count > 0)
    .map(([status, count]) => ({
      name: STATUS_LABELS[status] || status,
      value: count,
      color: getStatusColor(status),
    }))

  const sourceData = Object.entries(stats?.findings_by_source ?? {})
    .sort(([, a], [, b]) => b - a)
    .map(([source, count]) => ({ name: source, value: count }))

  return (
    <div className="space-y-6 animate-fade-in">
      <h1 className="text-2xl font-bold text-[var(--text-primary)]">Statistics</h1>

      {/* Overview */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        {[
          { label: 'Providers', value: stats?.providers_total ?? 0, color: 'text-[var(--accent)]' },
          { label: 'Models', value: stats?.models_total ?? 0, color: 'text-purple-400' },
          { label: 'Findings', value: stats?.findings_total ?? 0, color: 'text-[var(--status-confirmed)]' },
          { label: 'No CC', value: stats?.providers_no_cc ?? 0, color: 'text-[var(--status-verified)]' },
        ].map((item) => (
          <Card key={item.label}>
            <CardContent className="text-center py-6">
              {isLoading ? (
                <Skeleton className="h-8 w-16 mx-auto" />
              ) : (
                <p className={`text-3xl font-bold ${item.color}`}>{item.value}</p>
              )}
              <p className="text-sm text-[var(--text-muted)] mt-1">{item.label}</p>
            </CardContent>
          </Card>
        ))}
      </div>

      {/* Charts */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Providers by Status */}
        <Card>
          <CardHeader><CardTitle>Providers by Status</CardTitle></CardHeader>
          <CardContent>
            {isLoading ? (
              <Skeleton className="h-64 w-full" />
            ) : (
              <ResponsiveContainer width="100%" height={280}>
                <PieChart>
                  <Pie data={statusData} cx="50%" cy="50%" outerRadius={100} dataKey="value" stroke="none">
                    {statusData.map((entry, i) => <Cell key={i} fill={entry.color} />)}
                  </Pie>
                  <Tooltip contentStyle={{ backgroundColor: 'var(--bg-surface)', border: '1px solid var(--border)', borderRadius: '8px', color: 'var(--text-primary)', fontSize: '13px' }} />
                </PieChart>
              </ResponsiveContainer>
            )}
            <div className="flex flex-wrap gap-2 mt-4">
              {statusData.map((d) => (
                <Badge key={d.name} variant={d.name.toLowerCase() as 'verified' | 'confirmed' | 'claimed' | 'unverified' | 'expired' | 'deprioritized'}>
                  {d.name}: {d.value}
                </Badge>
              ))}
            </div>
          </CardContent>
        </Card>

        {/* Findings by Source */}
        <Card>
          <CardHeader><CardTitle>Findings by Source</CardTitle></CardHeader>
          <CardContent>
            {isLoading ? (
              <Skeleton className="h-64 w-full" />
            ) : sourceData.length > 0 ? (
              <ResponsiveContainer width="100%" height={280}>
                <BarChart data={sourceData} layout="vertical">
                  <XAxis type="number" tick={{ fill: 'var(--text-muted)', fontSize: 12 }} axisLine={{ stroke: 'var(--border)' }} />
                  <YAxis type="category" dataKey="name" tick={{ fill: 'var(--text-secondary)', fontSize: 12 }} axisLine={{ stroke: 'var(--border)' }} width={100} />
                  <Tooltip contentStyle={{ backgroundColor: 'var(--bg-surface)', border: '1px solid var(--border)', borderRadius: '8px', color: 'var(--text-primary)', fontSize: '13px' }} />
                  <Bar dataKey="value" fill="var(--accent)" radius={[0, 4, 4, 0]} />
                </BarChart>
              </ResponsiveContainer>
            ) : (
              <p className="text-[var(--text-muted)] text-center py-8">No data</p>
            )}
          </CardContent>
        </Card>
      </div>

      {/* Server info */}
      <Card>
        <CardHeader><CardTitle>Server Info</CardTitle></CardHeader>
        <CardContent>
          <div className="grid grid-cols-2 md:grid-cols-3 gap-4 text-sm">
            <div>
              <p className="text-[var(--text-muted)]">Server Time</p>
              <p className="text-[var(--text-primary)] font-mono">{stats?.server_time ? formatDate(stats.server_time) : '—'}</p>
            </div>
            <div>
              <p className="text-[var(--text-muted)]">API Version</p>
              <p className="text-[var(--text-primary)] font-mono">v0.1.0</p>
            </div>
            <div>
              <p className="text-[var(--text-muted)]">Frontend</p>
              <p className="text-[var(--text-primary)] font-mono">v0.7.0</p>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
