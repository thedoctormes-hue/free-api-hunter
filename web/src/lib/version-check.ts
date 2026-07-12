import { BUILD_VERSION } from './version.generated'

const VERSION_URL = '/version.json'
const CHECK_INTERVAL_MS = 60_000

async function checkVersion(): Promise<void> {
  try {
    const res = await fetch(VERSION_URL, { cache: 'no-store' })
    if (!res.ok) return
    const data = (await res.json()) as { version?: string }
    if (data.version && data.version !== BUILD_VERSION) {
      window.location.reload()
    }
  } catch {
    // сеть недоступна — игнорируем, проверим в следующий раз
  }
}

export function startVersionWatch(): void {
  void checkVersion()
  window.setInterval(() => void checkVersion(), CHECK_INTERVAL_MS)
  document.addEventListener('visibilitychange', () => {
    if (document.visibilityState === 'visible') void checkVersion()
  })
  window.addEventListener('focus', () => void checkVersion())
}
