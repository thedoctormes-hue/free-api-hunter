import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function formatDate(dateStr: string): string {
  if (!dateStr) return '—'
  const date = new Date(dateStr)
  return date.toLocaleDateString('ru-RU', {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
  })
}

export function formatDateTime(dateStr: string): string {
  if (!dateStr) return '—'
  const date = new Date(dateStr)
  return date.toLocaleString('ru-RU', {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

export function formatNumber(num: number): string {
  if (num >= 1000000) return `${(num / 1000000).toFixed(1)}M`
  if (num >= 1000) return `${(num / 1000).toFixed(1)}K`
  return num.toString()
}

export function formatContextLength(tokens: number): string {
  if (tokens >= 1000000) return `${(tokens / 1000000).toFixed(1)}M`
  if (tokens >= 1000) return `${(tokens / 1000).toFixed(0)}K`
  return tokens.toString()
}

export function getStatusColor(status: string): string {
  const colors: Record<string, string> = {
    verified: 'var(--status-verified)',
    confirmed: 'var(--status-confirmed)',
    claimed: 'var(--status-claimed)',
    unverified: 'var(--status-unverified)',
    expired: 'var(--status-expired)',
    deprioritized: 'var(--status-deprioritized)',
  }
  return colors[status] || 'var(--status-unverified)'
}

export function getStatusBgClass(status: string): string {
  const classes: Record<string, string> = {
    verified: 'bg-green-500/15 text-green-400 border-green-500/30',
    confirmed: 'bg-amber-500/15 text-amber-400 border-amber-500/30',
    claimed: 'bg-blue-500/15 text-blue-400 border-blue-500/30',
    unverified: 'bg-gray-500/15 text-gray-400 border-gray-500/30',
    expired: 'bg-red-500/15 text-red-400 border-red-500/30',
    deprioritized: 'bg-gray-500/10 text-gray-500 border-gray-500/20 line-through',
  }
  return classes[status] || classes.unverified
}

export function debounce<T extends (...args: unknown[]) => void>(
  fn: T,
  delay: number
): (...args: Parameters<T>) => void {
  let timeoutId: ReturnType<typeof setTimeout>
  return (...args: Parameters<T>) => {
    clearTimeout(timeoutId)
    timeoutId = setTimeout(() => fn(...args), delay)
  }
}
