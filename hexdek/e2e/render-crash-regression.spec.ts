import { test, expect, type Page } from '@playwright/test'

// Render-crash regression spec (r61) — pins the bug class behind the
// `pls` ReferenceError (commit 55386105 removed the plays_like binding
// but left four live reads in DeckArchive + SharePreview, killing both
// pages at render) and the earlier cardSearchOpen TDZ (44550a26).
//
// Stricter than smoke.spec.ts BY DESIGN: the smoke suite treats the
// error-boundary fallback ("SOMETHING BROKE") as a successful mount and
// allowlists "cannot access before initialization" console noise — which
// is exactly how both crashes shipped. Here the fallback IS the failure,
// and any ReferenceError / TDZ pageerror fails the test.
//
// Hermetic: uses a deliberately nonexistent owner/deck so no backend
// data is required. Both screens render their full page shell (hero,
// vital signs, meta tags) from the URL slug when every fetch fails —
// which is precisely the path that evaluated `pls` and crashed.

function collectCrashErrors(page: Page): string[] {
  const crashes: string[] = []
  page.on('pageerror', (err: Error) => {
    const msg = String(err?.message || err)
    if (/is not defined|before initialization|ReferenceError/i.test(msg)) {
      crashes.push(msg)
    }
  })
  return crashes
}

test('DeckArchive renders its page shell without a render crash', async ({ page }) => {
  const crashes = collectCrashErrors(page)
  await page.goto('/decks/regression-owner/regression_deck_b3')
  // The real container must mount — the error-boundary fallback is a FAIL.
  await expect(page.locator('.deck-archive-page')).toBeVisible({ timeout: 20_000 })
  await expect(page.locator('text=SOMETHING BROKE')).toHaveCount(0)
  // The POWER LEVEL vital-signs cell is the surface that read `pls`.
  await expect(page.locator('text=POWER LEVEL')).toBeVisible()
  expect(crashes, `ReferenceError/TDZ page errors on DeckArchive`).toEqual([])
})

test('SharePreview renders its page shell without a render crash', async ({ page }) => {
  const crashes = collectCrashErrors(page)
  await page.goto('/share/regression-owner/regression_deck_b3')
  await expect(page.locator('.deck-archive-page')).toBeVisible({ timeout: 20_000 })
  await expect(page.locator('text=SOMETHING BROKE')).toHaveCount(0)
  // SharePreview's hero meta row + POWER LEVEL cell both read `pls`.
  await expect(page.locator('text=POWER LEVEL')).toBeVisible()
  expect(crashes, `ReferenceError/TDZ page errors on SharePreview`).toEqual([])
})
