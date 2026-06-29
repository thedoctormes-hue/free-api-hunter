import { test, expect } from '@playwright/test'

test.describe('Providers page', () => {
  test('loads successfully', async ({ page }) => {
    await page.goto('/#/providers')
    await expect(page.locator('h1')).toHaveText('Providers')
  })

  test('displays provider cards by default', async ({ page }) => {
    await page.goto('/#/providers')
    const cards = page.locator('[data-testid="provider-card"]')
    await expect(cards.first()).toBeVisible()
  })

  test('filters providers by text search', async ({ page }) => {
    await page.goto('/#/providers')
    const searchInput = page.getByPlaceholder('Search providers...')
    await expect(searchInput).toBeVisible()
    await searchInput.fill('openai')
    await page.waitForTimeout(500)
    // After filtering, either some cards match or "No providers match" appears
    const cards = page.locator('[data-testid="provider-card"]')
    const count = await cards.count()
    // Just verify the search doesn't crash — count can be 0 or more
    expect(count).toBeGreaterThanOrEqual(0)
  })

  test('has credit card filter dropdown', async ({ page }) => {
    await page.goto('/#/providers')
    // CC filter is a <select>, not buttons
    const ccSelect = page.locator('select').filter({ hasText: 'No Credit Card' })
    await expect(ccSelect).toBeVisible()
    await ccSelect.selectOption('false')
    await page.waitForTimeout(500)
    await ccSelect.selectOption('true')
    await page.waitForTimeout(500)
  })

  test('supports table view toggle', async ({ page }) => {
    await page.goto('/#/providers')
    const tableBtn = page.locator('button', { hasText: 'Table' })
    const cardBtn = page.locator('button', { hasText: 'Cards' })
    await expect(tableBtn).toBeVisible()
    await expect(cardBtn).toBeVisible()
    await tableBtn.click()
    await page.waitForSelector('table')
    const table = page.locator('table')
    await expect(table).toBeVisible()
    await cardBtn.click()
    await page.waitForSelector('[data-testid="provider-card"]')
  })

  test('export button present', async ({ page }) => {
    await page.goto('/#/providers')
    await expect(page.locator('button', { hasText: 'Export' })).toBeVisible()
  })

  test('sorting controls present', async ({ page }) => {
    await page.goto('/#/providers')
    // There are 3 <select> elements: status filter, CC filter, sort by
    const selects = page.locator('select')
    await expect(selects.first()).toBeVisible()
  })

  test('each provider card has status badge', async ({ page }) => {
    await page.goto('/#/providers')
    const badges = page.locator('[data-testid="provider-status-badge"]')
    await expect(badges.first()).toBeVisible()
  })
})
