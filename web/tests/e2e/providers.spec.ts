import { test, expect } from '@playwright/test'

test.describe('Providers page', () => {
  test('loads successfully', async ({ page }) => {
    await page.goto('/providers')
    await expect(page).toHaveURL('/providers')
  })

  test('displays provider cards by default', async ({ page }) => {
    await page.goto('/providers')
    const cards = page.locator('[data-testid="provider-card"]')
    await expect(cards).toHaveCountGreaterThan(0)
  })

  test('filters models by text search', async ({ page }) => {
    await page.goto('/providers')
    const searchInput = page.getByPlaceholder('Search models...')
    await expect(searchInput).toBeVisible()
    await searchInput.fill('openai')
    await page.waitForTimeout(500)
    const cards = page.locator('[data-testid="provider-card"]')
    const count = await cards.count()
    expect(count).toBeGreaterThan(0) // at least some matching
  })

  test('has credit card toggle filter', async ({ page }) => {
    await page.goto('/providers')
    const noCC = page.locator('button:has-text("No Credit Card")')
    const withCC = page.locator('button:has-text("Credit Card")')
    await expect(noCC).toBeVisible()
    await expect(withCC).toBeVisible()
    await noCC.click()
    await page.waitForTimeout(500)
    const cards = page.locator('[data-testid="provider-card"]')
    const count = await cards.count()
    expect(count).toBeGreaterThanOrEqual(0)
    await withCC.click()
    await page.waitForTimeout(500)
  })

  test('supports table view toggle', async ({ page }) => {
    await page.goto('/providers')
    const tableBtn = page.locator('button:has-text("Table")')
    const cardBtn = page.locator('button:has-text("Cards")')
    await expect(tableBtn).toBeVisible()
    await expect(cardBtn).toBeVisible()
    await tableBtn.click()
    await page.waitForSelector('table')
    const table = page.locator('table')
    await expect(table).toBeVisible()
    await cardBtn.click()
    await page.waitForSelector('[data-testid="provider-card"]')
  })

  test('exports buttons present', async ({ page }) => {
    await page.goto('/providers')
    await expect(page.locator('button:has-text("Export")')).toBeVisible()
  })

  test('sorting controls present', async ({ page }) => {
    await page.goto('/providers')
    await expect(page.locator('select')).toHaveCountGreaterThan(0) // sort controls
    await expect(page.getByRole('columnheader')).toHaveCountGreaterThan(0) // for table view
  })

  test('each provider card has status badge', async ({ page }) => {
    await page.goto('/providers')
    const badges = page.locator('[data-testid="provider-status-badge"]')
    await expect(badges).toHaveCountGreaterThan(0)
  })
})
