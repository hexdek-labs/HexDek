import { test, expect } from '@playwright/test'

// /games/:gameId/summary — the new R60 GameSummary screen that
// consumes /api/games/{id}/summary (PRs #177 / #180 / #183 / #186).
// Renders four panels (commander zone visits, regret cards, MVP
// cards, turning points) plus a data-source tape that
// discriminates between "rich" (snapshot persisted) and
// "db_only" (game metadata only).
//
// Screenshot description (game-summary-{project}.png):
//   ┌─────────────────────────────────────────────────────────────┐
//   │ TAPE: GAME #N · K TURNS · WINNER SEAT W · END_REASON · TAG  │
//   ├─────────────────────────────────────────────────────────────┤
//   │ Panel GS.A COMMANDER ZONE VISITS                            │
//   │   commander_name (SEAT N) ...... Nx visited · cast Kx ...   │
//   │   ...                                                       │
//   │ Panel GS.B REGRET CARDS                                     │
//   │   card_name (SEAT N) ........... CMC X · stranded_in_hand   │
//   │   ... (or "no stranded cards" empty state)                  │
//   │ Panel GS.C MVP CARDS                                        │
//   │   card_name [CMDR] (SEAT N) .... CMC X · score Y · ...      │
//   │   ...                                                       │
//   │ Panel GS.D TURNING POINTS                                   │
//   │   1. T4  COMMANDER FIRST CAST  seat 1 cast "X" on turn 4    │
//   │   2. T9  GAME END              seat 1 won on turn 9         │
//   └─────────────────────────────────────────────────────────────┘
//
// The data-source tag in the tape reads RICH (green/solid) when a
// snapshot has been persisted via #186, METADATA ONLY (warn-amber)
// when the endpoint fell back to the db_only path, or the raw
// source string for forward-compatibility with future modes.

const DATA_WAIT_MS = 3000

const shot = (name: string) => {
  const project = test.info().project.name
  return `test-results/${name}-${project}.png`
}

test('game summary screen renders for a real game id', async ({ page, request }) => {
  // Pull the most recent finished game from /api/games — same
  // pattern as the report-screen test, which already exists in
  // smoke.spec.ts.
  let gameId: number | null = null
  try {
    const r = await request.get('/api/games?limit=1')
    if (r.ok()) {
      const games = await r.json()
      gameId = games?.[0]?.game_id ?? null
    }
  } catch {
    /* backend may be cold; the skip below handles it */
  }
  test.skip(!gameId, 'no completed games available to summarize')

  await page.goto(`/games/${gameId}/summary`)
  // Tape reads "GAME #{id}" on success. Loading variant reads
  // "GAME SUMMARY ... LOADING ... GAME #{id}", so we anchor on the
  // game-id portion which is present in both states.
  await expect(page.locator(`text=GAME #${gameId}`).first()).toBeVisible({ timeout: 15_000 })
  await page.waitForTimeout(DATA_WAIT_MS)

  // All four panels should be in the DOM regardless of which path
  // the endpoint took — the db_only fallback still mounts the
  // panels with their empty-state copy.
  await expect(page.locator('text=COMMANDER ZONE VISITS').first()).toBeVisible()
  await expect(page.locator('text=REGRET CARDS').first()).toBeVisible()
  await expect(page.locator('text=MVP CARDS').first()).toBeVisible()
  await expect(page.locator('text=TURNING POINTS').first()).toBeVisible()

  // The data-source tag is either RICH or METADATA ONLY.
  const dataSource = await page.locator('.tag').first().innerText()
  expect(['RICH', 'METADATA ONLY']).toContain(dataSource.trim())

  await page.screenshot({ path: shot('game-summary'), fullPage: true })
})

test('game summary screen shows error state for unknown game', async ({ page }) => {
  // Pick an id far outside the autoincrement range so the backend
  // returns 404 deterministically.
  await page.goto('/games/9999999/summary')
  await expect(page.locator(`text=GAME #9999999`).first()).toBeVisible({ timeout: 10_000 })
  // Either an error tape or a load-failed panel surfaces.
  await expect(page.locator('text=/ERROR|LOAD FAILED/i').first()).toBeVisible({ timeout: 10_000 })
  await page.screenshot({ path: shot('game-summary-404'), fullPage: true })
})
