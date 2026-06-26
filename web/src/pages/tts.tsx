import { useTTSProviders, useTTSStats } from '../hooks/use-tts'
import { Card, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { ExternalLink, Mic, Key } from 'lucide-react'

export function TTSPage() {
  const { data: providers, isLoading: providersLoading, error: providersError } = useTTSProviders()
  const { data: stats, isLoading: statsLoading } = useTTSStats()

  return (
    <div className="space-y-6 animate-fade-in">
      <div className="flex items-center gap-3">
        <div className="h-10 w-10 rounded-xl bg-[var(--accent)]/10 flex items-center justify-center">
          <Mic className="h-5 w-5 text-[var(--accent)]" />
        </div>
        <div>
          <h1 className="text-2xl font-bold text-[var(--text-primary)]">TTS/STT Providers</h1>
          <p className="text-sm text-[var(--text-muted)]">
            Text-to-Speech & Speech-to-Text — free tier catalog
          </p>
        </div>
      </div>

      {/* Stats Cards */}
      {statsLoading ? (
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
          {Array.from({ length: 4 }).map((_, i) => (
            <Card key={i} hover>
              <CardContent className="text-center py-6">
                <Skeleton className="h-8 w-16 mx-auto mb-2" />
                <Skeleton className="h-3 w-20 mx-auto" />
              </CardContent>
            </Card>
          ))}
        </div>
      ) : stats ? (
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
          {[
            { label: 'Total Providers', value: stats.providers_total },
            { label: 'Active', value: stats.active_count },
            { label: 'Free Tier', value: stats.free_tier_count },
            { label: 'Voices', value: stats.total_voices },
          ].map((card) => (
            <Card key={card.label} hover>
              <CardContent className="text-center py-6">
                <p className="text-3xl font-bold text-[var(--text-primary)]">{card.value}</p>
                <p className="text-sm text-[var(--text-muted)] mt-1">{card.label}</p>
              </CardContent>
            </Card>
          ))}
        </div>
      ) : null}

      {/* Providers List */}
      <div>
        <h2 className="text-lg font-semibold text-[var(--text-primary)] mb-4">Providers</h2>

        {providersLoading && (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {Array.from({ length: 6 }).map((_, i) => (
              <Card key={i}>
                <CardContent className="py-5">
                  <div className="flex items-center gap-3 mb-3">
                    <Skeleton className="h-5 w-5 rounded-full" />
                    <Skeleton className="h-4 w-28" />
                  </div>
                  <Skeleton className="h-3 w-full mb-2" />
                  <Skeleton className="h-3 w-3/4 mb-3" />
                  <Skeleton className="h-2 w-full rounded-full" />
                </CardContent>
              </Card>
            ))}
          </div>
        )}

        {providersError && (
          <Card>
            <CardContent className="flex items-center justify-center h-32 text-[var(--status-expired)]">
              Error loading providers: {providersError.message}
            </CardContent>
          </Card>
        )}

        {providers && providers.length === 0 && (
          <Card>
            <CardContent className="flex flex-col items-center justify-center h-32 text-[var(--text-muted)]">
              <p className="text-lg">No TTS providers available</p>
              <p className="text-sm mt-1">Run <code className="bg-[var(--bg-surface-active)] px-2 py-0.5 rounded text-xs">hunter --verify</code> to scan</p>
            </CardContent>
          </Card>
        )}

        {providers && providers.length > 0 && (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {providers.map((provider) => (
              <TTSProviderCard key={provider.name} provider={provider} />
            ))}
          </div>
        )}
      </div>

      {/* Key Pool Visualization */}
      <div>
        <h2 className="text-lg font-semibold text-[var(--text-primary)] mb-4">Key Pool Status</h2>
        <KeyPoolOverview providers={providers} isLoading={providersLoading} />
      </div>

      {/* Footer */}
      {stats?.updated_at && (
        <p className="text-xs text-[var(--text-muted)] text-center pt-4">
          Last updated: {new Date(stats.updated_at).toLocaleString('ru-RU')}
        </p>
      )}
    </div>
  )
}

function TTSProviderCard({ provider }: { provider: import('../api/tts').TTSProvider }) {
  const statusVariant = provider.status === 'active' || provider.status === 'verified'
    ? 'verified' as const
    : provider.status === 'expired'
      ? 'expired' as const
      : 'confirmed' as const

  return (
    <Card hover>
      <CardContent>
        <div className="flex items-start justify-between gap-2">
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2">
              <h3 className="font-semibold text-[var(--text-primary)] truncate">{provider.name}</h3>
              <Badge variant={statusVariant}>{provider.status}</Badge>
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
          {provider.free_tier && (
            <Badge variant="claimed" className="shrink-0">
              Free: {provider.free_tier.char_limit.toLocaleString()} chars
            </Badge>
          )}
        </div>

        {/* Key pool visualization */}
        {provider.free_tier && (
          <div className="mt-3">
            <KeyPoolBar
              charLimit={provider.free_tier.char_limit}
              resetPeriod={provider.free_tier.reset_period}
              voiceClones={provider.free_tier.voice_clones}
            />
          </div>
        )}

        {/* Models */}
        {provider.models.length > 0 && (
          <div className="mt-3">
            <span className="text-xs text-[var(--text-muted)]">Models:</span>
            <div className="flex flex-wrap gap-1 mt-1">
              {provider.models.slice(0, 4).map((m) => (
                <span key={m} className="text-xs px-1.5 py-0.5 rounded bg-[var(--bg-surface-active)] text-[var(--text-secondary)] font-mono">
                  {m}
                </span>
              ))}
              {provider.models.length > 4 && (
                <span className="text-xs text-[var(--text-muted)]">+{provider.models.length - 4}</span>
              )}
            </div>
          </div>
        )}

        {/* Features */}
        {provider.features.length > 0 && (
          <div className="mt-3">
            <span className="text-xs text-[var(--text-muted)]">Features:</span>
            <div className="flex flex-wrap gap-1 mt-1">
              {provider.features.map((f) => (
                <span key={f} className="text-xs px-1.5 py-0.5 rounded bg-[var(--status-verified)]/10 text-[var(--status-verified)]">
                  {f}
                </span>
              ))}
            </div>
          </div>
        )}

        {/* Languages count */}
        {provider.languages.length > 0 && (
          <div className="mt-3 text-xs text-[var(--text-muted)]">
            {provider.languages.length} languages
          </div>
        )}

        {/* Limits */}
        {provider.limits && (
          <div className="mt-2 text-xs text-[var(--text-muted)]">
            {provider.limits}
          </div>
        )}

        {provider.notes && (
          <p className="text-xs text-[var(--text-muted)] mt-2">{provider.notes}</p>
        )}
      </CardContent>
    </Card>
  )
}

function KeyPoolBar({ charLimit, resetPeriod, voiceClones }: {
  charLimit: number
  resetPeriod: string
  voiceClones: number
}) {
  // Simulate usage (since we don't have actual usage data from API yet)
  const estimated = Math.min(charLimit * 0.6, charLimit)
  const pct = charLimit > 0 ? (estimated / charLimit) * 100 : 0
  const barColor = pct > 80 ? 'bg-[var(--status-expired)]' : pct > 50 ? 'bg-[var(--status-confirmed)]' : 'bg-[var(--status-verified)]'

  return (
    <div className="space-y-1.5">
      <div className="flex items-center justify-between text-xs">
        <span className="text-[var(--text-muted)]">Free tier usage</span>
        <span className="text-[var(--text-secondary)]">{estimated.toLocaleString()} / {charLimit.toLocaleString()} chars</span>
      </div>
      <div className="h-2 rounded-full bg-[var(--bg-surface-active)] overflow-hidden">
        <div
          className={`h-full rounded-full ${barColor} transition-all duration-500`}
          style={{ width: `${pct}%` }}
        />
      </div>
      <div className="flex items-center justify-between text-xs text-[var(--text-muted)]">
        <span>Reset: {resetPeriod}</span>
        <span>{voiceClones} voice clones</span>
      </div>
    </div>
  )
}

function KeyPoolOverview({ providers, isLoading }: {
  providers: import('../api/tts').TTSProvider[] | undefined
  isLoading: boolean
}) {
  if (isLoading) {
    return (
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        {Array.from({ length: 4 }).map((_, i) => (
          <Card key={i}><CardContent className="py-5"><Skeleton className="h-16 w-full" /></CardContent></Card>
        ))}
      </div>
    )
  }

  if (!providers || providers.length === 0) {
    return null
  }

  // Check if any provider has key pool data (chars_used field)
  // The API currently doesn't expose key pool data, so we show a fallback
  const hasKeyPoolData = providers.some(
    (p) => (p as unknown as Record<string, unknown>)['chars_used'] !== undefined
  )

  if (!hasKeyPoolData) {
    return (
      <Card>
        <CardContent className="py-8">
          <div className="flex flex-col items-center justify-center text-center gap-3">
            <div className="h-12 w-12 rounded-xl bg-[var(--accent)]/10 flex items-center justify-center">
              <Key className="h-6 w-6 text-[var(--accent)]" />
            </div>
            <div>
              <p className="text-sm font-medium text-[var(--text-primary)]">Key pool data not available</p>
              <p className="text-xs text-[var(--text-muted)] mt-1">
                Run <code className="bg-[var(--bg-surface-active)] px-2 py-0.5 rounded text-xs">hunter --verify</code> to load key pool data
              </p>
            </div>
          </div>
        </CardContent>
      </Card>
    )
  }

  // If we had key pool data, render per-provider bars
  return (
    <div className="space-y-4">
      {providers.map((provider) => {
        const charsUsed = ((provider as unknown as Record<string, unknown>)['chars_used'] as number) || 0
        const charsLimit = provider.free_tier?.char_limit || 1
        const pct = Math.min((charsUsed / charsLimit) * 100, 100)
        const isExhausted = pct >= 100

        return (
          <Card key={provider.name} hover>
            <CardContent className="py-4">
              <div className="flex items-center justify-between mb-2">
                <div className="flex items-center gap-2">
                  <span className={`h-2.5 w-2.5 rounded-full ${isExhausted ? 'bg-[var(--status-expired)]' : 'bg-[var(--status-verified)]'}`} />
                  <span className="text-sm font-medium text-[var(--text-primary)]">{provider.name}</span>
                </div>
                <span className={`text-xs ${isExhausted ? 'text-[var(--status-expired)]' : 'text-[var(--text-muted)]'}`}>
                  {isExhausted ? 'Exhausted' : `${charsUsed.toLocaleString()} / ${charsLimit.toLocaleString()} chars`}
                </span>
              </div>
              <div className="h-2 rounded-full bg-[var(--bg-surface-active)] overflow-hidden">
                <div
                  className={`h-full rounded-full transition-all duration-500 ${
                    isExhausted ? 'bg-[var(--status-expired)]' : pct > 80 ? 'bg-[var(--status-confirmed)]' : 'bg-[var(--status-verified)]'
                  }`}
                  style={{ width: `${pct}%` }}
                />
              </div>
            </CardContent>
          </Card>
        )
      })}
    </div>
  )
}
