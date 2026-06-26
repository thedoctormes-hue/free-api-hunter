import { create } from 'zustand'

interface AppState {
  sidebarOpen: boolean
  searchQuery: string
  setSidebarOpen: (open: boolean) => void
  toggleSidebar: () => void
  closeSidebar: () => void
  setSearchQuery: (q: string) => void
}

export const useAppStore = create<AppState>((set) => ({
  sidebarOpen: false,
  searchQuery: '',
  setSidebarOpen: (open) => set({ sidebarOpen: open }),
  toggleSidebar: () => set((s) => ({ sidebarOpen: !s.sidebarOpen })),
  closeSidebar: () => set({ sidebarOpen: false }),
  setSearchQuery: (q) => set({ searchQuery: q }),
}))
