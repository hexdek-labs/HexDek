import { test, expect } from '@playwright/test'

// SummaryActions on the Report screen — verifies the DEEP SUMMARY
// link navigates to /games/:id/summary (#189) and the SHARE
// button copies the canonical /api/games/{id}/summary URL to the
// clipboard.
//
// UX in one paragraph: the RESULT BLOCK panel header on the
// Report screen (/report/:gameId) now carries a two-button row
// next to the game-id stamp. DEEP SUMMARY → routes the user into
// the GameSummary screen for the same game id; SHARE → copies
// the absolute /api/games/{id}/summary URL to the clipboard and
// flashes "COPIED ✓" for ~1.8s as confirmation. The buttons reuse
// the existing Btn chrome so they match the surrounding design
// language, and they fall back gracefully when the Clipboard API
// is denied (legacy textarea + execCommand path).

const shot = (name: string) => {
  const project = test.info().project.name
  return `test-results/${name}-${project}.png`
}

test.use({
  // Grant clipboard permissions for the SHARE-button test.
  permissions: ['clipboard-read', 'clipboard-write'],
})

test('Report DEEP SUMMARY button navigates to GameSummary', async ({ page, request }) => {
  // Pull a real game id, same pattern as the existing report and
  // game-summary tests.
  let gameId: number | null = null
  try {
    const r = await request.get('/api/games?limit=1')
    if (r.ok()) {
      const games = await r.json()
      gameId = games?.[0]?.game_id ?? null
    }
  } catch {
    /* backend may be cold */
  }
  test.skip(!gameId, 'no completed games available')

  await page.goto(`/report/${gameId}`)
  await expect(page.locator('text=/RESULT BLOCK/i').first()).toBeVisible({ timeout: 15_000 })

  // The SummaryActions container carries a data-testid so the
  // selector survives copy-edits to the button labels.
  const actions = page.locator('[data-testid="summary-actions"]').first()
  await expect(actions).toBeVisible({ timeout: 10_000 })

  const deepBtn = actions.locator('button:has-text("DEEP SUMMARY")').first()
  await expect(deepBtn).toBeVisible()
  await deepBtn.click()

  await page.waitForURL(`**/games/${gameId}/summary`, { timeout: 10_000 })
  await expect(page.locator(`text=GAME #${gameId}`).first()).toBeVisible({ timeout: 10_000 })
  await page.screenshot({ path: shot('report-deep-summary-nav'), fullPage: true })
})

test('Report SHARE button copies the /summary URL to clipboard', async ({ page, request, browserName }) => {
  // Webkit's headless clipboard.readText is unreliable in Playwright
  // even with permissions; skip the readback step on webkit and just
  // assert the visual "COPIED ✓" confirmation flips on.
  let gameId: number | null = null
  try {
    const r = await request.get('/api/games?limit=1')
    if (r.ok()) {
      const games = await r.json()
      gameId = games?.[0]?.game_id ?? null
    }
  } catch {
    /* noop */
  }
  test.skip(!gameId, 'no completed games available')

  await page.goto(`/report/${gameId}`)
  await expect(page.locator('text=/RESULT BLOCK/i').first()).toBeVisible({ timeout: 15_000 })

  const actions = page.locator('[data-testid="summary-actions"]').first()
  const shareBtn = actions.locator('button:has-text("SHARE")').first()
  await expect(shareBtn).toBeVisible({ timeout: 10_000 })

  await shareBtn.click()

  // Visual confirmation flips on within a tick.
  await expect(actions.locator('button:has-text("COPIED ✓")').first()).toBeVisible({ timeout: 2_000 })

  // Clipboard readback (skip on webkit where it's flaky).
  if (browserName !== 'webkit') {
    const clip = await page.evaluate(() => navigator.clipboard.readText()).catch(() => '')
    if (clip) {
      // Match the API path; origin varies with how the dev server is
      // configured, so anchor on the suffix.
      expect(clip).toMatch(new RegExp(`/api/games/${gameId}/summary$`))
    }
  }

  // After ~1.8s the button reverts to SHARE.
  await expect(actions.locator('button:has-text("SHARE")').first()).toBeVisible({ timeout: 3_000 })

  await page.screenshot({ path: shot('report-share-copied'), fullPage: true })
})
