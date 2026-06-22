import { Card, CardContent } from '@/components/ui/card'
import { Home } from 'lucide-react'

export function NotFoundPage() {
  return (
    <div className="flex items-center justify-center min-h-[60vh] animate-fade-in">
      <Card className="max-w-md w-full">
        <CardContent className="text-center py-12">
          <p className="text-6xl font-bold text-[var(--text-muted)] mb-4">404</p>
          <h1 className="text-xl font-semibold text-[var(--text-primary)] mb-2">Page Not Found</h1>
          <p className="text-sm text-[var(--text-muted)] mb-6">The page you're looking for doesn't exist.</p>
          <a
            href="#/"
            className="inline-flex items-center gap-2 px-4 py-2 rounded-lg bg-[var(--accent)] text-white text-sm font-medium hover:bg-[var(--accent-hover)] transition-colors"
          >
            <Home className="h-4 w-4" />
            Back to Dashboard
          </a>
        </CardContent>
      </Card>
    </div>
  )
}
