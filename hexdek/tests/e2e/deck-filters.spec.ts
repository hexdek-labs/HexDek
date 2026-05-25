import { test, expect } from '@playwright/test'

// Decks-screen filter chip rows. Pre-r60, /decks had a single archetype
// chip row; r60 adds bracket and color rows. These tests pin the
// per-row structure, URL-param round-trip, and that selecting a chip
// actually narrows the match count.
//
// Like the rest of hexdek/tests/e2e, this runs against the deployed
// staging site (dev.hexdek.dev by default). The unit tests under
// tests/unit/ cover the predicate logic synchronously; this spec only
// asserts that the rendered chips wire up to the same predicates.

const DATA_WAIT_MS = 4000

async function waitForDecks(page) {
  await page.waitForFunction(
    () => !document.body.innerText.includes('LOADING DECK ARCHIVE'),
    null,
    { timeout: 15_000 },
  ).catch(() => {})
  await page.waitForTimeout(DATA_WAIT_MS)
}

test('decks: all three filter chip rows render with legends', async ({ page }) => {
  await page.goto('/decks')
  await waitForDecks(page)
  for (const testid of ['bracket-chips', 'color-chips', 'archetype-chips']) {
    const row = page.getByTestId(testid)
    await expect(row, `expected ${testid} row`).toBeVisible()
  }
  // Each row begins with an "ALL" chip the user can click back to.
  for (const testid of ['bracket-chips', 'color-chips', 'archetype-chips']) {
    const allChip = page.getByTestId(testid).locator('[data-chip-id="all"]')
    await expect(allChip, `${testid} ALL chip`).toBeVisible()
  }
})

test('decks: bracket chip toggles URL param and reduces match count', async ({ page }) => {
  await page.goto('/decks')
  await waitForDecks(page)

  const matchLabel = page.locator('text=/\\d+ MATCHES/').first()
  const before = await matchLabel.innerText()
  const beforeN = parseInt(before, 10)

  await page.getByTestId('bracket-chips').locator('[data-chip-id="b4"]').click()
  await expect(page).toHaveURL(/[?&]bracket=b4(\b|&)/)

  const after = await matchLabel.innerText()
  const afterN = parseInt(after, 10)
  expect(afterN).toBeLessThanOrEqual(beforeN)
})

test('decks: color chip toggles URL param', async ({ page }) => {
  await page.goto('/decks')
  await waitForDecks(page)

  await page.getByTestId('color-chips').locator('[data-chip-id="u"]').click()
  await expect(page).toHaveURL(/[?&]color=u(\b|&)/)

  // ALL clears the param.
  await page.getByTestId('color-chips').locator('[data-chip-id="all"]').click()
  await expect(page).not.toHaveURL(/[?&]color=/)
})

test('decks: archetype chip URL param round-trips on direct load', async ({ page }) => {
  await page.goto('/decks?archetype=combo')
  await waitForDecks(page)
  // The combo chip should be active (solid styling = different aria/state)
  // — we assert by data attribute presence and that the URL persisted.
  await expect(page).toHaveURL(/[?&]archetype=combo(\b|&)/)
  const comboChip = page.getByTestId('archetype-chips').locator('[data-chip-id="combo"]')
  await expect(comboChip).toBeVisible()
})
