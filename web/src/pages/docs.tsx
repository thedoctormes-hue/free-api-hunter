import { useState } from 'react'
import { ChevronRight, ChevronDown, Copy, Check, Zap } from 'lucide-react'
import { cn } from '@/lib/utils'
import { API_BASE } from '@/api/config'

interface Endpoint {
  method: 'GET' | 'POST'
  path: string
  summary: string
  description?: string
  params?: { name: string; type: string; description: string; required?: boolean }[]
  responseExample?: string
}

const endpoints: Endpoint[] = [
  {
    method: 'GET',
    path: '/health',
    summary: 'Health Check',
    description: 'Проверка работоспособности сервера. Публичный эндпоинт без авторизации.',
    responseExample: `{
  "success": true,
  "data": {
    "status": "ok",
    "time": "2026-06-29T01:00:00Z"
  }
}`,
  },
  {
    method: 'GET',
    path: '/api/v1/providers',
    summary: 'Список провайдеров',
    description: 'Получить список всех обнаруженных LLM-провайдеров с фильтрацией.',
    params: [
      { name: 'status', type: 'string', description: 'Фильтр по статусу: verified, confirmed, claimed, unverified, expired, deprioritized' },
      { name: 'credit_card', type: 'string', description: 'Фильтр по наличию кредитной карты: "true" или "false"' },
    ],
    responseExample: `{
  "success": true,
  "data": [
    {
      "name": "OpenRouter",
      "url": "https://openrouter.ai",
      "api_key_url": "https://openrouter.ai/keys",
      "credit_card": false,
      "status": "verified",
      "models": ["gpt-4", "claude-3"],
      "limits": "100 req/day free",
      "notes": "",
      "source": "manual",
      "priority": 1,
      "discovered_at": "2026-01-15T10:00:00Z",
      "last_verified": "2026-06-28T12:00:00Z"
    }
  ],
  "meta": { "count": 1, "version": "0.1.0" }
}`,
  },
  {
    method: 'GET',
    path: '/api/v1/providers/{id}',
    summary: 'Провайдер по ID',
    description: 'Получить детальную информацию о конкретном провайдере по его имени.',
    params: [
      { name: 'id', type: 'string', description: 'Имя провайдера (из поля name)', required: true },
    ],
    responseExample: `{
  "success": true,
  "data": {
    "name": "OpenRouter",
    "url": "https://openrouter.ai",
    "status": "verified",
    "models": ["gpt-4", "claude-3"]
  },
  "meta": { "count": 1, "version": "0.1.0" }
}`,
  },
  {
    method: 'GET',
    path: '/api/v1/findings',
    summary: 'Список находок',
    description: 'Получить список обнаруженных источников API с фильтрацией по источнику и лимиту.',
    params: [
      { name: 'source', type: 'string', description: 'Фильтр по источнику (например: hackernews, reddit, github)' },
      { name: 'limit', type: 'integer', description: 'Максимальное количество результатов' },
    ],
    responseExample: `{
  "success": true,
  "data": [
    {
      "source_id": "hackernews",
      "title": "Free GPT-4 API Alternative",
      "url": "https://example.com/api",
      "description": "Unofficial wrapper...",
      "raw_text": "Check out this free API...",
      "discovered_at": "2026-06-20T15:30:00Z",
      "is_duplicate": false,
      "quality_score": 0.85,
      "filtered_out": false,
      "filter_reason": ""
    }
  ],
  "meta": { "count": 1, "version": "0.1.0" }
}`,
  },
  {
    method: 'GET',
    path: '/api/v1/stats',
    summary: 'Статистика',
    description: 'Агрегированная статистика по провайдерам, находкам и моделям.',
    responseExample: `{
  "success": true,
  "data": {
    "providers_total": 42,
    "providers_by_status": {"verified": 10, "confirmed": 8},
    "providers_no_cc": 15,
    "findings_total": 128,
    "findings_by_source": {"hackernews": 45},
    "models_total": 256,
    "server_time": "2026-06-29T01:00:00Z"
  },
  "meta": { "count": 0, "version": "0.1.0" }
}`,
  },
  {
    method: 'GET',
    path: '/api/v1/scan-history',
    summary: 'История сканирований',
    description: 'Получить историю запусков сканирования.',
    params: [
      { name: 'limit', type: 'integer', description: 'Количество записей (по умолчанию 20)' },
    ],
    responseExample: `{
  "success": true,
  "data": [
    {
      "started_at": "2026-06-28T10:00:00Z",
      "duration_ms": 12500,
      "sources_scanned": 5,
      "findings_count": 3,
      "status": "completed"
    }
  ],
  "meta": { "count": 1, "version": "0.1.0" }
}`,
  },
  {
    method: 'POST',
    path: '/api/v1/scan',
    summary: 'Запустить сканирование',
    description: 'Запускает процесс сканирования источников. Пока не реализовано (заглушка).',
    responseExample: `{
  "success": true,
  "data": {
    "status": "not_implemented",
    "message": "Scan trigger via API is not yet implemented. Use CLI mode."
  }
}`,
  },
  {
    method: 'GET',
    path: '/api/v1/tts/providers',
    summary: 'TTS провайдеры',
    description: 'Получить список провайдеров Text-to-Speech с результатами верификации.',
    responseExample: `{
  "success": true,
  "data": [
    {
      "name": "ElevenLabs",
      "url": "https://elevenlabs.io",
      "free_tier": { "char_limit": 10000 },
      "voices": ["Rachel", "Adam"]
    }
  ],
  "meta": { "count": 1, "version": "0.1.0" }
}`,
  },
  {
    method: 'GET',
    path: '/api/v1/tts/providers/{id}',
    summary: 'TTS провайдер по ID',
    description: 'Получить информацию о конкретном TTS-провайдере по имени.',
    params: [
      { name: 'id', type: 'string', description: 'Имя TTS-провайдера', required: true },
    ],
    responseExample: `{
  "success": true,
  "data": {
    "name": "ElevenLabs",
    "url": "https://elevenlabs.io",
    "voices": ["Rachel", "Adam"]
  },
  "meta": { "count": 1, "version": "0.1.0" }
}`,
  },
  {
    method: 'GET',
    path: '/api/v1/tts/stats',
    summary: 'TTS статистика',
    description: 'Агрегированная статистика по TTS-провайдерам: количество, активные, бесплатные тарифы.',
    responseExample: `{
  "success": true,
  "data": {
    "providers_total": 8,
    "active_count": 6,
    "free_tier_count": 5,
    "total_voices": 42,
    "updated_at": "2026-06-28T12:00:00Z"
  },
  "meta": { "count": 0, "version": "0.1.0" }
}`,
  },
]

const methodColors: Record<string, string> = {
  GET: 'bg-emerald-500/15 text-emerald-400 border border-emerald-500/30',
  POST: 'bg-amber-500/15 text-amber-400 border border-amber-500/30',
}

function EndpointDoc({ endpoint }: { endpoint: Endpoint }) {
  const [expanded, setExpanded] = useState(false)
  const [copied, setCopied] = useState(false)

  const fullPath = `${API_BASE}${endpoint.path}`

  const handleCopy = async () => {
    await navigator.clipboard.writeText(`curl "${fullPath}"`)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <div className="border border-[var(--border)] rounded-xl overflow-hidden">
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center gap-3 px-4 py-3 text-left hover:bg-[var(--bg-surface-hover)] transition-colors"
      >
        <span className={cn('px-2 py-0.5 text-xs font-bold rounded-md', methodColors[endpoint.method])}>
          {endpoint.method}
        </span>
        <code className="text-sm font-mono text-[var(--text-primary)] flex-1 truncate">
          {endpoint.path}
        </code>
        <span className="text-xs text-[var(--text-muted)] hidden sm:inline">{endpoint.summary}</span>
        {expanded ? (
          <ChevronDown className="h-4 w-4 text-[var(--text-muted)]" />
        ) : (
          <ChevronRight className="h-4 w-4 text-[var(--text-muted)]" />
        )}
      </button>

      {expanded && (
        <div className="px-4 pb-4 space-y-4 border-t border-[var(--border)] bg-[var(--bg-surface)]/50">
          {endpoint.description && (
            <p className="text-sm text-[var(--text-secondary)] pt-3">{endpoint.description}</p>
          )}

          {endpoint.params && endpoint.params.length > 0 && (
            <div>
              <h4 className="text-xs font-semibold text-[var(--text-muted)] uppercase tracking-wide mb-2">Параметры</h4>
              <div className="space-y-2">
                {endpoint.params.map((param) => (
                  <div key={param.name} className="flex items-start gap-3 text-sm">
                    <code className="px-2 py-0.5 rounded bg-[var(--bg-base)] text-[var(--accent)] text-xs font-mono shrink-0">
                      {param.name}
                    </code>
                    <span className="text-[var(--text-muted)] text-xs shrink-0">({param.type})</span>
                    <span className="text-[var(--text-secondary)] text-xs">{param.description}</span>
                  </div>
                ))}
              </div>
            </div>
          )}

          {endpoint.responseExample && (
            <div>
              <div className="flex items-center justify-between mb-2">
                <h4 className="text-xs font-semibold text-[var(--text-muted)] uppercase tracking-wide">Пример ответа</h4>
                <button
                  onClick={handleCopy}
                  className="inline-flex items-center gap-1 text-xs text-[var(--text-muted)] hover:text-[var(--accent)] transition-colors"
                >
                  {copied ? <Check className="h-3 w-3" /> : <Copy className="h-3 w-3" />}
                  {copied ? 'Скопировано' : 'Копировать curl'}
                </button>
              </div>
              <pre className="p-3 rounded-lg bg-[var(--bg-base)] border border-[var(--border)] text-xs font-mono text-[var(--text-secondary)] overflow-x-auto">
                {endpoint.responseExample}
              </pre>
            </div>
          )}

          <div className="flex items-center gap-2 text-xs text-[var(--text-muted)]">
            <Zap className="h-3 w-3" />
            <span>URL: <code className="text-[var(--text-secondary)]">{fullPath}</code></span>
          </div>
        </div>
      )}
    </div>
  )
}

export function DocsPage() {
  const [filter, setFilter] = useState<'all' | 'GET' | 'POST'>('all')

  const filtered = filter === 'all'
    ? endpoints
    : endpoints.filter((e) => e.method === filter)

  return (
    <div className="space-y-6 animate-fade-in">
      <div>
        <h1 className="text-2xl font-bold text-[var(--text-primary)]">API Documentation</h1>
        <p className="text-sm text-[var(--text-muted)] mt-1">
          Интерактивная документация всех эндпоинтов Free API Hunter
        </p>
      </div>

      {/* Filters */}
      <div className="flex items-center gap-2">
        {(['all', 'GET', 'POST'] as const).map((f) => (
          <button
            key={f}
            onClick={() => setFilter(f)}
            className={cn(
              'px-3 py-1.5 text-xs rounded-lg border transition-colors',
              filter === f
                ? 'bg-[var(--accent-muted)] border-[var(--accent)]/30 text-[var(--accent)]'
                : 'border-[var(--border)] text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-surface-hover)]'
            )}
          >
            {f === 'all' ? 'Все' : f}
          </button>
        ))}
        <span className="text-xs text-[var(--text-muted)] ml-2">
          {filtered.length} эндпоинтов
        </span>
      </div>

      {/* Endpoints */}
      <div className="space-y-3">
        {filtered.map((endpoint) => (
          <EndpointDoc key={`${endpoint.method}-${endpoint.path}`} endpoint={endpoint} />
        ))}
      </div>

      {/* Info */}
      <div className="border border-[var(--border)] rounded-xl p-4 bg-[var(--bg-surface)]/50">
        <h3 className="text-sm font-semibold text-[var(--text-primary)] mb-2">Общие правила</h3>
        <ul className="space-y-1 text-xs text-[var(--text-secondary)]">
          <li>• Все ответы в формате JSON с обёрткой <code className="text-[var(--accent)]">{'{ success, data, error, meta }'}</code></li>
          <li>• Базовый URL API: <code className="text-[var(--accent)]">{API_BASE || window.location.origin}</code></li>
          <li>• Content-Type: <code className="text-[var(--accent)]">application/json; charset=utf-8</code></li>
          <li>• Эндпоинты <code className="text-[var(--accent)]">/api/v1/*</code> защищены rate-limiting</li>
        </ul>
      </div>
    </div>
  )
}
