import { Search, Moon, Sun, Menu } from 'lucide-react'
import { cn } from '@/lib/utils'
import { useTheme } from '@/contexts/theme'
import { useAppStore } from '@/lib/store'

export function Header() {
  const { theme, toggleTheme } = useTheme()
  const { toggleSidebar, searchQuery, setSearchQuery } = useAppStore()

  return (
    <header className="sticky top-0 z-50 border-b border-[var(--border)] bg-[var(--bg-base)]/80 backdrop-blur-md">
      <div className="container flex h-14 items-center gap-4">
        {/* Mobile menu */}
        <button
          onClick={toggleSidebar}
          className="md:hidden p-2 rounded-lg hover:bg-[var(--bg-surface-hover)] transition-colors"
          aria-label="Toggle menu"
        >
          <Menu className="h-5 w-5 text-[var(--text-secondary)]" />
        </button>

        {/* Logo */}
        <a href="#/" className="flex items-center gap-2 font-bold text-lg text-[var(--text-primary)] shrink-0">
          <span className="text-xl">🔍</span>
          <span className="hidden sm:inline">Free API Hunter</span>
        </a>

        {/* Search */}
        <div className="flex-1 max-w-md mx-auto">
          <div className="relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-[var(--text-muted)]" />
            <input
              type="text"
              placeholder="Search providers..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="w-full h-9 pl-9 pr-4 text-sm rounded-lg border border-[var(--border)] bg-[var(--bg-surface)] text-[var(--text-primary)] placeholder:text-[var(--text-muted)] focus:outline-none focus:border-[var(--accent)] transition-colors"
            />
          </div>
        </div>

        {/* Theme toggle */}
        <button
          onClick={toggleTheme}
          className={cn(
            'p-2 rounded-lg hover:bg-[var(--bg-surface-hover)] transition-colors'
          )}
          aria-label="Toggle theme"
        >
          {theme === 'dark' ? (
            <Sun className="h-5 w-5 text-[var(--text-secondary)]" />
          ) : (
            <Moon className="h-5 w-5 text-[var(--text-secondary)]" />
          )}
        </button>
      </div>
    </header>
  )
}
