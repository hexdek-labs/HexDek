# Wave 2 Final Status — Deferred Sites Reasoning (R60)

**Date:** 2026-05-31
**Branches consumed:** #924 (12 single-step), #944 (6 library-reorder), #952 (7 sac/graveyard), #964 (6 hand-cheat + library-walk), and this PR (5 final).
**Total Wave 2 migrations:** 36 single-step + multi-step sites across 5 PRs.

This doc inventories the sites still carrying `seat.<Zone> = append(...)` patterns AFTER Wave 2 completes, with the reason each is deferred. Grep across `internal/gameengine/per_card/` will continue to surface ~24 files as "manual append present" because the within-zone library reorders introduced in #944's canonical migration intentionally retain that shape (CR §400.7 — library top↔bottom is not a zone change). The actual surface remaining is:

- **6 files retain INTENTIONAL within-zone reorders** (no zone change; the manual append is correct per CR).
- **18 files remain genuinely deferred** for one of the four reasons below.

## Intentional within-zone library reorders (NOT a bug)

The canonical migration for "look at top N, pick some to move, rest stay in library" routes moved cards through `MoveCard` and reorders the remaining cards by `seat.Library = seat.Library[1:]; seat.Library = append(seat.Library, c)` (a positional shuffle within the library zone). Per CR §400.7, a card moving from library top to library bottom is NOT a zone change, so this is not an anti-pattern — it's the canonical idiom.

Files in this category: `star_charter.go`, `birthing_ritual.go`, `svella_ice_shaper.go`, `gen_toph_greatest_earthbender.go`, `ayesha_tanaka_armorer.go`, `esika.go`, `custom_mayael_the_anima.go`. Wave 2's migrations are complete for these files.

## Genuinely deferred — 18 sites by category

### Category D — Multi-card iteration with sequenced moves (10 files)

These handlers walk a list and perform multiple zone changes in a specific order where the order matters (each move depends on the previous having completed). Migrating each one requires careful sequencing through `MoveCard` to preserve the dependency graph, plus tests that exercise the full sequence. Not a one-line drop.

- `chaos_cascade.go` — multi-card exile cascade with conditional play
- `gen_the_capitoline_triad.go` — graveyard → exile + graveyard → bf
- `hurkyl_master_wizard.go` — bounce-N batch with cross-perm dependencies
- `commanders_batch.go` — multi-trigger batch handler
- `grub_storied_matriarch.go` — encounter-trigger batch
- `gen_sandman_shifting_scoundrel.go` — discard + look batch
- `gen_raph_mikey_troublemakers.go` — multi-card pick+play loop
- `etb_tribe_gate_family.go` — multi-card library reshuffle
- `runo_stromkirk.go` — library reorder with conditional swap
- `batch17_sweep.go` — sweep utility with mass reshuffle

### Category E — Cross-seat state movement (3 files)

These move cards between owners' private zones. `MoveCard` handles cross-seat owner-routing via the §400.7 owner-redirect path, but the per_card handlers also coordinate trigger state across seats (control-change semantics, "you may put" prompts on opponents). Migrating requires deeper §400.7c modeling.

- `custom_selenia_dark_angel.go` — opponent steal-and-hand
- `gisa_glorious_resurrector.go` — opp-creature-dies steal
- `custom_yorion_sky_nomad.go` — flicker-to-exile with controller-side state retention

### Category F — Custom *Permanent construction (3 files)

These build `*Permanent` manually rather than going through `createPermanent`. Migration needs routing through `enterBattlefieldWithETB` plus a careful mirror of their side-effects (sometimes they set custom `Counters` or `Flags` that need to fire in a specific order around ETB triggers).

- `atraxa_grand.go` — proliferate-style draw stamping
- `custom_jadzi_oracle_of_arcavios.go` — magic-prowess return chain
- `obeka_support.go` — extra-phase scheduler with permanent flag

### Category G — Within-zone reorders not yet migrated (2 files)

These follow the same library-reorder pattern Wave 2 canonized but were lower-priority and not part of the Wave 2 batches. A future PR can apply the same `read top N → MoveCard the picks → rotate rest top→bottom` shape used in #944.

- `zurgo_ojutai.go` — reveal-N picker with hand+library bottom
- `satoru_umezawa.go` — top-N reorder with picked-to-hand

## Recommended next steps

Wave 2 is structurally complete: the canonical chokepoints (`MoveCard`, `DiscardCard`, `enterBattlefieldWithETB`, `MintSpellCopy`, `MintTokenAsCopyOf`) cover every clean migration target. The 18 deferred files split as:

1. **Category D (10 files)** — biggest remaining surface. Each is a per-handler migration; recommend tackling 2-3 per follow-up PR with full integration tests covering the sequenced moves.
2. **Category E (3 files)** — needs §400.7c modeling for cross-seat state coordination. Recommend a dedicated cross-seat-state PR that lifts the shared pattern into a helper.
3. **Category F (3 files)** — needs careful per-handler test setup. Manual `*Permanent` construction is rare; recommend tackling individually as the cards come up in tournament regressions.
4. **Category G (2 files)** — copy/paste the #944 canonical pattern. Easiest follow-up — a single small PR could close both.

## Verification of Wave 2 completion

```
$ go test ./internal/gameengine/... -count=1
ok  github.com/hexdek/hexdek/internal/gameengine    19.0s
ok  github.com/hexdek/hexdek/internal/gameengine/counters    0.8s
ok  github.com/hexdek/hexdek/internal/gameengine/instanceid    1.2s
ok  github.com/hexdek/hexdek/internal/gameengine/per_card    6.4s
```

All 5 Wave 2 PRs merged; 36 per_card handlers + 1 engine-side spell-copy chokepoint (`MintSpellCopy`) routed through canonical helpers. The structural zone-helper coverage gap that Wave 2 was designed to close is now closed for every shape encountered. Remaining 18 sites are documented above with their per-category migration recipes.

— Wave 2 / 2026-05-31
