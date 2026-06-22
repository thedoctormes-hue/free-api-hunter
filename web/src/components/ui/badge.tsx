import { cn } from '@/lib/utils'
import type { ReactNode } from 'react'

interface BadgeProps {
  children: ReactNode
  variant?: 'default' | 'verified' | 'confirmed' | 'claimed' | 'unverified' | 'expired' | 'deprioritized'
  className?: string
}

const variantClasses: Record<string, string> = {
  default: 'bg-gray-500/15 text-gray-400 border-gray-500/30',
  verified: 'bg-green-500/15 text-green-400 border-green-500/30',
  confirmed: 'bg-amber-500/15 text-amber-400 border-amber-500/30',
  claimed: 'bg-blue-500/15 text-blue-400 border-blue-500/30',
  unverified: 'bg-gray-500/15 text-gray-400 border-gray-500/30',
  expired: 'bg-red-500/15 text-red-400 border-red-500/30',
  deprioritized: 'bg-gray-500/10 text-gray-500 border-gray-500/20 line-through',
}

export function Badge({ children, variant = 'default', className }: BadgeProps) {
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1 px-2 py-0.5 text-xs font-medium rounded-full border',
        variantClasses[variant],
        className
      )}
    >
      {children}
    </span>
  )
}
