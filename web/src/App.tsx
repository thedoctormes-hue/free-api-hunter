import { useState, useEffect, lazy, Suspense } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Header } from '@/components/layout/header'
import { Sidebar } from '@/components/layout/sidebar'
import { ThemeProvider } from '@/contexts/theme'
import { ErrorBoundary } from '@/components/shared/error-boundary'
import { useAppStore } from '@/lib/store'
import { Skeleton } from '@/components/ui/skeleton'

const DashboardPage = lazy(() => import('@/pages/dashboard').then(m => ({ default: m.DashboardPage })))
const ProvidersPage = lazy(() => import('@/pages/providers').then(m => ({ default: m.ProvidersPage })))
const FindingsPage = lazy(() => import('@/pages/findings').then(m => ({ default: m.FindingsPage })))
const StatsPage = lazy(() => import('@/pages/stats').then(m => ({ default: m.StatsPage })))
const TTSPage = lazy(() => import('@/pages/tts').then(m => ({ default: m.TTSPage })))
const DocsPage = lazy(() => import('@/pages/docs').then(m => ({ default: m.DocsPage })))
const NotFoundPage = lazy(() => import('@/pages/not-found').then(m => ({ default: m.NotFoundPage })))

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 3,
      refetchOnWindowFocus: true,
      staleTime: 5 * 60 * 1000,
      gcTime: 10 * 60 * 1000,
    },
  },
})

function PageSkeleton() {
  return (
    <div className="space-y-6">
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        {Array.from({ length: 4 }).map((_, i) => (
          <div key={i} className="rounded-xl border border-[var(--border)] bg-[var(--bg-surface)] p-5">
            <Skeleton className="h-20 w-full" />
          </div>
        ))}
      </div>
      <Skeleton className="h-64 w-full" />
    </div>
  )
}

function AppContent() {
  const [currentPath, setCurrentPath] = useState('/')
  const { sidebarOpen, closeSidebar, searchQuery } = useAppStore()

  useEffect(() => {
    const handleHash = () => {
      const hash = window.location.hash.slice(1) || '/'
      setCurrentPath(hash)
    }
    handleHash()
    window.addEventListener('hashchange', handleHash)
    return () => window.removeEventListener('hashchange', handleHash)
  }, [])

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
      case '/docs':
        return <DocsPage />
      default:
        return <NotFoundPage />
    }
  }

  return (
    <div className="min-h-screen bg-[var(--bg-base)] text-[var(--text-primary)] transition-colors duration-300">
      <Header />
      <Sidebar
        currentPath={currentPath}
        isOpen={sidebarOpen}
        onClose={closeSidebar}
      />
      <main className="md:ml-56 pt-6 pb-12">
        <div className="container">
          <ErrorBoundary>
            <Suspense fallback={<PageSkeleton />}>
              {renderPage()}
            </Suspense>
          </ErrorBoundary>
        </div>
      </main>
    </div>
  )
}

function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <ThemeProvider>
        <AppContent />
      </ThemeProvider>
    </QueryClientProvider>
  )
}

export default App
