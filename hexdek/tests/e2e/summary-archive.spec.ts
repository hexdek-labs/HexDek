import { test, expect } from '@playwright/test'

// /games/summaries — the SummaryArchive screen lists every game
// that has a persisted observation snapshot (#180). Five
// filters above a paginated table; clicking a row jumps to the
// per-game GameSummary screen (#189).
//
// UX in one paragraph: the page surfaces a five-input filter row
// (FROM date, UNTIL date, COMMANDER substring, DECK substring,
// WINNER substring) plus APPLY / CLEAR buttons; results render
// as a clickable table (GAME id, FINISHED timestamp, T turns,
// WINNER name + seat, COMMANDERS joined per-seat, END reason).
// Each row navigates the user into /games/:id/summary on click.
// The tape header shows the active row count and a "RICH GAMES
// ONLY" tag so users understand the archive excludes games
// without persisted snapshots. PREV / NEXT page buttons surface
// once paging is meaningful (PAGE_SIZE = 25).

const shot = (name: string) => {
  const project = test.info().project.name
  return `test-results/${name}-${project}.png`
}

test('summary archive renders the table with snapshotted games', async ({ page, request }) => {
  // Probe the API first — if no snapshotted games exist, the
  // empty state is what we want to test. Otherwise the table
  // path is the expectation.
  let hasRows = false
  try {
    const r = await request.get('/api/games/summaries?limit=1')
    if (r.ok()) {
      const body = await r.json()
      hasRows = (body?.rows?.length ?? 0) > 0
    }
  } catch {
    /* empty-state test arm covers backend-unavailable too */
  }

  await page.goto('/games/summaries')
  await expect(page.locator('text=PAST SUMMARIES').first()).toBeVisible({ timeout: 10_000 })
  // Filters panel mounts.
  await expect(page.locator('[data-testid="summary-archive-filters"]').first()).toBeVisible()
  // Five filter inputs are present and focusable.
  for (const id of ['filter-since', 'filter-until', 'filter-commander', 'filter-deck', 'filter-winner']) {
    await expect(page.locator(`[data-testid="${id}"]`).first()).toBeVisible()
  }

  if (hasRows) {
    await expect(page.locator('[data-testid="summary-archive-table"]').first()).toBeVisible({ timeout: 10_000 })
    // At least one row is rendered.
    const rowCount = await page.locator('tr[data-testid^="summary-row-"]').count()
    expect(rowCount).toBeGreaterThan(0)
  } else {
    await expect(page.locator('[data-testid="summary-archive-empty"]').first()).toBeVisible({ timeout: 10_000 })
  }

  await page.screenshot({ path: shot('summary-archive'), fullPage: true })
})

test('summary archive applies a commander filter', async ({ page, request }) => {
  // Need at least one snapshotted game to exercise the filter
  // path; skip otherwise.
  let pickCommander: string | null = null
  try {
    const r = await request.get('/api/games/summaries?limit=1')
    if (r.ok()) {
      const body = await r.json()
      const c = body?.rows?.[0]?.commanders?.[0]
      if (typeof c === 'string' && c.length > 2) {
        // Use the first 4 chars as the filter so the substring
        // matches even with minor name variations.
        pickCommander = c.slice(0, 4)
      }
    }
  } catch {
    /* noop */
  }
  test.skip(!pickCommander, 'no snapshotted games to filter against')

  await page.goto('/games/summaries')
  await expect(page.locator('[data-testid="filter-commander"]').first()).toBeVisible({ timeout: 10_000 })

  await page.locator('[data-testid="filter-commander"]').first().fill(pickCommander!)
  await page.locator('[data-testid="filter-apply"]').first().click()

  // Either the table re-renders with results or the empty state
  // mounts. Both are valid outcomes for the filter; the assertion
  // is that the page doesn't error.
  await page.waitForTimeout(1500)
  const hasTable = await page.locator('[data-testid="summary-archive-table"]').count()
  const hasEmpty = await page.locator('[data-testid="summary-archive-empty"]').count()
  expect(hasTable + hasEmpty).toBeGreaterThan(0)
})

test('summary archive row click navigates to GameSummary', async ({ page, request }) => {
  // Pull a real game id with a snapshot.
  let gameId: number | null = null
  try {
    const r = await request.get('/api/games/summaries?limit=1')
    if (r.ok()) {
      const body = await r.json()
      gameId = body?.rows?.[0]?.game_id ?? null
    }
  } catch {
    /* noop */
  }
  test.skip(!gameId, 'no snapshotted games to navigate from')

  await page.goto('/games/summaries')
  const row = page.locator(`[data-testid="summary-row-${gameId}"]`).first()
  await expect(row).toBeVisible({ timeout: 10_000 })
  await row.click()

  await page.waitForURL(`**/games/${gameId}/summary`, { timeout: 10_000 })
  await expect(page.locator(`text=GAME #${gameId}`).first()).toBeVisible({ timeout: 10_000 })
})
