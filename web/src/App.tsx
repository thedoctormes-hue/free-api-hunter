import { useState, useEffect } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Header } from '@/components/layout/header'
import { Sidebar } from '@/components/layout/sidebar'
import { DashboardPage } from '@/pages/dashboard'
import { ProvidersPage } from '@/pages/providers'
import { FindingsPage } from '@/pages/findings'
import { StatsPage } from '@/pages/stats'
import { TTSPage } from '@/pages/tts'
import { NotFoundPage } from '@/pages/not-found'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 2,
      refetchOnWindowFocus: false,
      staleTime: 5 * 60 * 1000,
    },
  },
})

function App() {
  const [dark, setDark] = useState(true)
  const [sidebarOpen, setSidebarOpen] = useState(false)
  const [searchQuery, setSearchQuery] = useState('')
  const [currentPath, setCurrentPath] = useState('/')

  useEffect(() => {
    if (dark) {
      document.documentElement.classList.remove('light')
    } else {
      document.documentElement.classList.add('light')
    }
  }, [dark])

  useEffect(() => {
    const handleHash = () => {
      const hash = window.location.hash.slice(1) || '/'
      setCurrentPath(hash)
    }
    handleHash()
    window.addEventListener('hashchange', handleHash)
    return () => window.removeEventListener('hashchange', handleHash)
  }, [])

  const toggleTheme = () => setDark((d) => !d)
  const toggleSidebar = () => setSidebarOpen((o) => !o)
  const closeSidebar = () => setSidebarOpen(false)

  const renderPage = () => {
    switch (currentPath) {
      case '/':
        return <DashboardPage />
      case '/providers':
        return <ProvidersPage searchQuery={searchQuery} />
      case '/findings':
        return <FindingsPage />
      case '/stats':
        return <StatsPage />
      case '/tts':
        return <TTSPage />
      default:
        return <NotFoundPage />
    }
  }

  return (
    <QueryClientProvider client={queryClient}>
      <div className="min-h-screen bg-[var(--bg-base)] text-[var(--text-primary)]">
        <Header
          dark={dark}
          onToggleTheme={toggleTheme}
          onToggleSidebar={toggleSidebar}
          searchQuery={searchQuery}
          onSearchChange={setSearchQuery}
        />
        <Sidebar
          currentPath={currentPath}
          isOpen={sidebarOpen}
          onClose={closeSidebar}
        />
        <main className="md:ml-56 pt-6 pb-12">
          <div className="container">
            {renderPage()}
          </div>
        </main>
      </div>
    </QueryClientProvider>
  )
}

export default App
