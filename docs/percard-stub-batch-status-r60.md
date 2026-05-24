# Per-card stub-batch status — R60 (2026-05-24)

Findings from auditing batches I, L, and O for "missing handler"
work. **All three already shipped on `main`; no implementation needed.**
Recorded so future workers don't repeat the spelunk.

## Method

For each of the four R5x stub-batch test files
(`internal/gameengine/per_card/percard_stub_batch_{g_r50,i_r51,l_r52,o_r53}_test.go`),
read the file's docstring header to enumerate the picks, then run the
relevant test names against the package. A "missing handler" surface
would manifest as either (a) a compile failure naming an unresolved
helper symbol or (b) a runtime test failure asserting flag-clear /
registry-presence behavior the handler is supposed to provide.

## Results

| Batch | File | Picks | Test run | Status |
|-------|------|------:|----------|--------|
| I | `percard_stub_batch_i_r51_test.go` | 10 trigger-only LTB cleanups: Zaffai and the Tempests, Nadu Winged Wisdom, Neriv Heart of the Storm, Cecily Haunted Mage, The Second Doctor, Kuja Genome Sorcerer, Samut the Driving Force, Sen Triplets, Storm Force of Nature, Lightning Army of One | `go test ./internal/gameengine/per_card/ -run "TestZaffai\|TestNadu\|TestNeriv\|TestCecily\|TestSecondDoctor\|TestKuja\|TestSamut\|TestSenTriplets\|TestStorm\|TestLightning\|TestBatchI"` | **PASS** — all 10 LTB helpers wired |
| L | `percard_stub_batch_l_r52_test.go` | 3 picks: Yorion Sky Nomad dedup neuter, The Reality Chip LTB clear, Zethi Arcane Blademaster LTB sweep | `go test ./internal/gameengine/per_card/ -run "TestYorion\|TestRealityChip\|TestZethi"` | **PASS** |
| O | `percard_stub_batch_o_r53_test.go` | 3 picks: Cecil DFC face-name aliases (6 new alias registrations), Reaper King once-per-turn gate moved to seat (flicker bypass fix), Jolly Balloon Man ETB hook drop (false parser_gap signal) | `go test ./internal/gameengine/per_card/ -run "TestCecil\|TestReaperKing\|TestJollyBalloon"` | **PASS** |

Full `go test ./internal/gameengine/per_card/ -count=1` also green
(1.05s).

## Why these are all closed

Git log corroborates the test result — each batch landed as its own
feat commit and is in `main`:

```
072ae44 feat(per_card): R53 stub-batch O port — 3 picks (Cecil face aliases + Reaper flicker bypass + Balloon Man partial noise)
1fb67e2 feat(per_card): R52 stub-batch L port — 3 picks (Yorion dedup + 2 LTB cleanups)
d438a34 feat(per_card): R51 stub-batch I port — 10 trigger-only LTB cleanups
```

(Plus duplicate-author commits `2f889d6` / `37e771d` from a parallel
worker — same content, same hash root.)

This matches the broader pattern from `docs/percard-stub-census-r56.md`
and `docs/percard-stub-census-r58.md`: by mid-R5x the per_card stub
pool is heavily thinned and the few `emitPartial` breadcrumbs remaining
in `gen_*.go` handlers are flagging **engine-deep gaps** —
`cost_modifiers` replacement effects, AST keyword pipeline coverage,
`while_source_on_bf` duration on continuous-effect grants — that
correctly live on the engine side and need no per_card change. The
batch L docstring is explicit about this:

> Picks are sparse this round — the per_card stub pool has been
> thinned across batches A–K and the remaining 27 emitPartial-bearing
> handlers are mostly substantively ported with emitPartial breadcrumbs
> flagging legitimate engine-deep gaps … that are correctly engine-side
> and need no per_card change.

## What this means for future workers

- **Do not re-port batches A–O.** They are all shipped. Re-running the
  picks against a fresh registry will assert green every time; there
  is no implementation work to do in `internal/gameengine/per_card/`
  under the "stub-batch port" framing.
- **`emitPartial` is not always a per_card bug.** Census-driven sweeps
  that grep for `emitPartial` will surface the engine-deep gaps too.
  Before opening a per_card PR for a partial, confirm the missing
  capability is per-card-shaped and not a generic AST/resolve gap.
- **The next productive surface for per_card work** (per the R58
  census and the post-Muninn 2026-05-17 saturation report) is
  event-driven: a specific card flagged by Goldilocks, Loki, or a
  tournament replay. The "wave / batch NNN-MMM" cadence has been
  formally retired in `docs/muninn-saturation-report-2026-05-17.md`
  for the same reason — it produces 0-2 handler PRs that thrash the
  gap-log without moving coverage.

## What might still warrant per_card attention

Not from these three batches, but adjacent surfaces flagged by other
R60 work that *would* take per_card edits if they regress:

- The Iroh-pattern continuous-effect grant family (TLA flashback-grant
  sweep — closed for mass-grant in R60 main, single-target shape has a
  partial sibling sweep already shipped in PR `f73c5d1`; remaining
  callouts in CLAUDE.md 2026-05-23 row are Archmage's Newt, The
  Fugitive Doctor, Lost in Memories).
- Per_card handlers that route through the package-local
  `removePermanent` instead of `gameengine.ExilePermanent` — these
  bypass §614 replacements, §903.9b commander redirect, and aura
  `detachAll`. Six were flagged in the May-11 nil-deref forensics
  (etrata, zabaz, zimone+dina, bilbo, thassa). The Abdel Adrian fix
  in commit `b348f4a` patched the crash symptom but the API misuse
  in the siblings is still standing.

Neither belongs to the stub-batch porting line. Both are documented
elsewhere; this note is purely to close the "did anyone try batch
I/L/O lately?" loop.
