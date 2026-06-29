import { Server, ShieldCheck, Cpu, CreditCard } from 'lucide-react'
import { Card, CardContent } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { formatNumber } from '@/lib/utils'
import type { Stats } from '@/lib/types'

interface StatsCardsProps {
  stats: Stats | undefined
  isLoading: boolean
}

export function StatsCards({ stats, isLoading }: StatsCardsProps) {
  const cards = [
    {
      label: 'Total Providers',
      value: stats?.providers_total ?? 0,
      icon: Server,
      color: 'text-[var(--accent)]',
      bg: 'bg-[var(--accent)]/10',
    },
    {
      label: 'Verified',
      value: stats?.providers_by_status?.verified ?? 0,
      icon: ShieldCheck,
      color: 'text-[var(--status-verified)]',
      bg: 'bg-[var(--status-verified)]/10',
    },
    {
      label: 'Total Models',
      value: stats?.models_total ?? 0,
      icon: Cpu,
      color: 'text-purple-400',
      bg: 'bg-purple-500/10',
    },
    {
      label: 'No Credit Card',
      value: stats?.providers_no_cc ?? 0,
      icon: CreditCard,
      color: 'text-[var(--status-confirmed)]',
      bg: 'bg-[var(--status-confirmed)]/10',
    },
  ]

  return (
    <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
      {cards.map((card) => {
        const Icon = card.icon
        return (
          <Card key={card.label} hover data-testid="stats-card">
            <CardContent className="flex items-center gap-4">
              {isLoading ? (
                <Skeleton className="h-12 w-12 rounded-xl" />
              ) : (
                <div className={`h-12 w-12 rounded-xl ${card.bg} flex items-center justify-center`}>
                  <Icon className={`h-5 w-5 ${card.color}`} />
                </div>
              )}
              <div>
                {isLoading ? (
                  <>
                    <Skeleton className="h-6 w-12 mb-1" />
                    <Skeleton className="h-3 w-16" />
                  </>
                ) : (
                  <>
                    <p className="text-2xl font-bold text-[var(--text-primary)]">
                      {formatNumber(card.value)}
                    </p>
                    <p className="text-xs text-[var(--text-muted)]">{card.label}</p>
                  </>
                )}
              </div>
            </CardContent>
          </Card>
        )
      })}
    </div>
  )
}
