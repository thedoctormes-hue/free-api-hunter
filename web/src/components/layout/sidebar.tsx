import { LayoutDashboard, Server, Search, BarChart3, Mic } from 'lucide-react'
import { cn } from '@/lib/utils'

interface SidebarProps {
  currentPath: string
  isOpen: boolean
  onClose: () => void
}

const navItems = [
  { path: '/', label: 'Dashboard', icon: LayoutDashboard },
  { path: '/providers', label: 'Providers', icon: Server },
  { path: '/findings', label: 'Findings', icon: Search },
  { path: '/stats', label: 'Statistics', icon: BarChart3 },
  { path: '/tts', label: 'TTS/STT', icon: Mic },
]

export function Sidebar({ currentPath, isOpen, onClose }: SidebarProps) {
  return (
    <>
      {/* Mobile overlay */}
      {isOpen && (
        <div
          className="fixed inset-0 z-40 bg-black/50 md:hidden"
          onClick={onClose}
        />
      )}

      {/* Sidebar */}
      <aside
        className={cn(
          'fixed top-14 left-0 z-40 h-[calc(100vh-3.5rem)] w-56 border-r border-[var(--border)] bg-[var(--bg-base)] transition-transform duration-200 md:translate-x-0',
          isOpen ? 'translate-x-0' : '-translate-x-full'
        )}
      >
        <nav className="p-3 space-y-1">
          {navItems.map((item) => {
            const Icon = item.icon
            const isActive = currentPath === item.path || (item.path !== '/' && currentPath.startsWith(item.path))
            return (
              <a
                key={item.path}
                href={item.path}
                onClick={onClose}
                className={cn(
                  'flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium transition-colors',
                  isActive
                    ? 'bg-[var(--accent-muted)] text-[var(--accent)]'
                    : 'text-[var(--text-secondary)] hover:bg-[var(--bg-surface-hover)] hover:text-[var(--text-primary)]'
                )}
              >
                <Icon className="h-4 w-4 shrink-0" />
                {item.label}
              </a>
            )
          })}
        </nav>

        {/* Footer */}
        <div className="absolute bottom-0 left-0 right-0 p-4 border-t border-[var(--border)]">
          <p className="text-xs text-[var(--text-muted)]">
            Free API Hunter v0.7.0
          </p>
          <p className="text-xs text-[var(--text-muted)] mt-1">
            LabDoctorM
          </p>
        </div>
      </aside>
    </>
  )
}
