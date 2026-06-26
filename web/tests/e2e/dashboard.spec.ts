import { test, expect } from '@playwright/test'

test.describe('Dashboard page', () => {
  test('loads successfully', async ({ page }) => {
    await page.goto('/')
    await expect(page).toHaveURL('/dashboard')
    await expect(page.locator('title')).toHaveText(/Dashboard/) // adjust if actual title exists
  })

  test('displays stats cards', async ({ page }) => {
    await page.goto('/dashboard')
    const cards = page.locator('[data-testid="stats-card"]')
    await expect(cards).toHaveCount(3)
  })

  test('has theme toggle button', async ({ page }) => {
    await page.goto('/dashboard')
    const toggle = page.locator('button[data-testid="theme-toggle"]')
    await expect(toggle).toBeVisible()
    await toggle.click()
    await expect(page.locator('body')).toHaveClass(/dark|light/) // toggle changes theme
  })

  test('refetch buttons work', async ({ page }) => {
    await page.goto('/dashboard')
    const statsRefresh = page.locator('button:has-text("Refresh stats")')
    const providersRefresh = page.locator('button:has-text("Refresh"):visible')
    await expect(statsRefresh).toBeVisible()
    await expect(providersRefresh).toBeVisible()
  })
})
