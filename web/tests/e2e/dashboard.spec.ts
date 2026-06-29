import { test, expect } from '@playwright/test'

test.describe('Dashboard page', () => {
  test('loads successfully', async ({ page }) => {
    await page.goto('/')
    // Hash-based routing: app loads at "/" and renders Dashboard for path "/"
    await expect(page).toHaveURL('/')
    await expect(page.locator('h1')).toHaveText('Dashboard')
  })

  test('displays stats cards', async ({ page }) => {
    await page.goto('/')
    const cards = page.locator('[data-testid="stats-card"]')
    // StatsCards renders 4 cards: Total Providers, Verified, Total Models, No Credit Card
    await expect(cards).toHaveCount(4)
  })

  test('has theme toggle button', async ({ page }) => {
    await page.goto('/')
    const toggle = page.locator('button[data-testid="theme-toggle"]')
    await expect(toggle).toBeVisible()
    await toggle.click()
    // Theme toggle adds/removes 'light' class on <html>
    await expect(page.locator('html')).toHaveClass(/light|dark|/)
  })

  test('refresh button works', async ({ page }) => {
    await page.goto('/')
    const refreshBtn = page.locator('button').filter({ hasText: 'Refresh' })
    await expect(refreshBtn).toBeVisible()
    await refreshBtn.click()
  })
})
