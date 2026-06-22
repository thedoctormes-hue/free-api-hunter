import { PieChart, Pie, Cell, ResponsiveContainer, Tooltip, Legend } from 'recharts'
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { getStatusColor } from '@/lib/utils'
import type { Stats } from '@/lib/types'

interface ProvidersChartProps {
  stats: Stats | undefined
  isLoading: boolean
}

const STATUS_LABELS: Record<string, string> = {
  verified: 'Verified',
  confirmed: 'Confirmed',
  claimed: 'Claimed',
  unverified: 'Unverified',
  expired: 'Expired',
  deprioritized: 'Deprioritized',
}

export function ProvidersByStatusChart({ stats, isLoading }: ProvidersChartProps) {
  if (isLoading) {
    return (
      <Card>
        <CardHeader><CardTitle>Providers by Status</CardTitle></CardHeader>
        <CardContent><Skeleton className="h-64 w-full" /></CardContent>
      </Card>
    )
  }

  const data = Object.entries(stats?.providers_by_status ?? {})
    .filter(([, count]) => count > 0)
    .map(([status, count]) => ({
      name: STATUS_LABELS[status] || status,
      value: count,
      color: getStatusColor(status),
    }))

  if (data.length === 0) {
    return (
      <Card>
        <CardHeader><CardTitle>Providers by Status</CardTitle></CardHeader>
        <CardContent className="flex items-center justify-center h-64 text-[var(--text-muted)]">
          No data available
        </CardContent>
      </Card>
    )
  }

  return (
    <Card>
      <CardHeader><CardTitle>Providers by Status</CardTitle></CardHeader>
      <CardContent>
        <ResponsiveContainer width="100%" height={280}>
          <PieChart>
            <Pie
              data={data}
              cx="50%"
              cy="50%"
              innerRadius={60}
              outerRadius={100}
              paddingAngle={3}
              dataKey="value"
              stroke="none"
            >
              {data.map((entry, index) => (
                <Cell key={index} fill={entry.color} />
              ))}
            </Pie>
            <Tooltip
              contentStyle={{
                backgroundColor: 'var(--bg-surface)',
                border: '1px solid var(--border)',
                borderRadius: 'var(--radius-md)',
                color: 'var(--text-primary)',
                fontSize: '13px',
              }}
            />
            <Legend
              formatter={(value) => <span style={{ color: 'var(--text-secondary)', fontSize: '12px' }}>{value}</span>}
            />
          </PieChart>
        </ResponsiveContainer>
      </CardContent>
    </Card>
  )
}
