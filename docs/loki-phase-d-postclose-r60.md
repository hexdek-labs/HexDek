# InstanceID Phase D — Private-Zone Mint-Coverage Closure (r60)

**Branch:** `dev/phase-d-private-zone-mint-coverage-r60`
**Date:** 2026-05-30
**Predecessor:** PR #767 (Phase C — strict-census default ON, 25k verification surfaced 2,904,066 ZoneConservation hits)

## Objective

Close the 2.9M ZoneConservation "card disappeared" hit cluster that the
strict-census disappearance arm flipped on by default in Phase C
surfaced. These are real engine paths that mint InstanceIDs (canonical
`MintOG/Token/Copy` chain), then drop the *Card pointer from every zone
without calling `MarkInstanceIDCeased`.

The Phase C 25k run reported 2,904,066 hits — ~116 violations per game,
dominated by token-typed *Cards that:

1. Are seeded into chaos-generated decks as library cards (Scryfall
   contains "Token" entries that aren't real deck-legal cards but were
   not filtered at corpus load).
2. Get minted at battlefield-add then dropped via non-canonical perm
   removal paths (mutate eats other / control-trade / per_card raw
   `removePermanent`).
3. Resolve copy/cease semantics (CR §707.10) on the stack that bypass
   the InstanceID cessation chain.

## Step 1 — Categorize the 2.9M cluster

A 500-game seed-42 baseline sweep (`/tmp/zc_phase_d_500.md`,
strict-census ON via Phase C default) reported **52,620 hits across 498
of 500 games**. Sampling the first 30 violation details showed
**100% OG-provenance** IDs disappearing — all at turn 1–2 cleanup —
implying a setup-time path was filtering the cards.

A single-game diagnostic harness (`cmd/hexdek-loki/phase_d_diag_test.go`)
named the disappearing cards:

- `h0OGVG000018` = "Guenhwyvar" types=[token, legendary, creature, cat]
- `h1OGVG000049` = "Insect" types=[token, creature, insect]
- `h1OGVW000039` = "Zombie" types=[token, creature, zombie]
- `h3OGVB000002` = "Assassin" types=[token, creature, assassin]

These are **Scryfall token entries** that the chaos corpus loader didn't
filter — they entered library zones as OG-provenance *Cards, then SBA
§704.5d ("tokens in non-battlefield zones cease") swept them out of the
library WITHOUT calling `MarkInstanceIDCeased`. The cards' OG IDs stayed
in `MintedInstanceIDs` but no zone held them → strict-census fired.

## Step 2 — Fix the canonical cessation gaps

### Fix 1: SBA §704.5d cessation

`sba704_5d` in `internal/gameengine/sba.go` removed token-typed *Cards
from libraries/hands/graveyards/exile without ceasing their IDs. Added
`MarkInstanceIDCeased(gs, c.InstanceID)` in the removal branch. Covers
both TK-provenance tokens (canonical mint path) and OG-provenance
token-typed corpus seeds.

### Fix 2: SBA §704.5e cessation

`removeCopiesFromZone` in `internal/gameengine/sba.go` removed `IsCopy=true`
*Cards without ceasing IDs. Threaded `*GameState` through and added
`MarkInstanceIDCeased` per removal. Updated all 4 callers
(`sba704_5e` for hand/graveyard/exile/library).

### Fix 3: `evacuateStackSpellsToGraveyard` copy cessation

`internal/gameengine/loop_shortcut.go` skipped `IsCopy=true` stack items
with `continue` — a §727 no-op detector path that fired when
`projectAndApply` truncated `gs.Stack`. Added `MarkInstanceIDCeased` for
the skipped copies per §707.10.

### Fix 4: Universal token cessation chokepoint at `gs.removePermanent`

`internal/gameengine/state.go:removePermanent` is the engine-wide primitive
for taking a `*Permanent` off the battlefield slice. Every canonical path
(Destroy / Exile / Sacrifice / Bounce, plus the SBA equivalents) already
called `markPermanentCeaseIfToken` afterward, but ~10 non-canonical sites
(mutate eats other / per_card raw `gs.removePermanent` / sweep alt-cost
return / control-trade) bypassed it.

Rather than chase each call site individually, the cessation is now baked
into the primitive: when the removed perm is a token AND its `InstanceID`
is non-empty, cease the ID and CLEAR `Card.InstanceID = ""`. Clearing the
field models §704.5d's "ceases to exist" + new-token-on-re-entry
semantics: blink-and-re-add paths now pick up a fresh TK mint from
`EnsureTokenInstanceID` when the perm re-enters via
`enterBattlefieldWithETB`.

Token-as-copy paths (Phantasmal Image, Spark Double, etc.) stay safe
because the gap-walk's `EnforceBattlefieldUniqueInstanceID` re-mints
those *Cards as TK before they hit the battlefield — so by the time
this chokepoint sees them, their ID is TK, not the copied OG.

### Fix 5: Per-card `removePermanent` mirror

The per_card helper `removePermanent` in `internal/gameengine/per_card/helpers.go`
does its OWN battlefield manipulation (doesn't route through
`gs.removePermanent`). Mirrored the cessation+clear logic there so the
30+ per_card blink/flicker handlers don't leak tokens.

### Fix 6: `removePermanentFromBattlefield` mirror

The non-trigger removal path used by Craft / Meld / Aura swap /
activation-cost ExileSelf (`internal/gameengine/keywords_misc.go`) was
also bypassing cessation. Added the cessation+clear there.

### Fix 7: Chaos corpus token filter

Scryfall's oracle-cards.json contains explicit "Token" entries
(e.g., "creature token soldier Token", "treasure artifact token Token"
with `type_line` containing "Token") that aren't real deck-legal cards.
The chaos deck generator was seeding them as library cards, which would
let SBA §704.5d sweep them (now correctly per Fix 1) but flag noisy
"OG-typed token card disappeared" violations.

Filtering these at `loadOracleCorpus` (`cmd/hexdek-loki/main.go`) by
checking `strings.Contains(tlLower, "token")` prevents the seeding at
the source — chaos decks now only contain genuinely deck-legal cards.

### Diagnostic enhancement: `MintedInstanceIDNames`

Added a `map[string]string` side-table on `GameState` populated by
`RecordMintedInstanceIDName` from each of the three mint helpers
(`MintOG/Token/Copy`). The ZoneConservation disappearance message now
includes the card name when available, so post-mortem walks can identify
the offending engine path without re-running with diagnostic
instrumentation. Example violation message:

```
ZoneConservation: InstanceID "h1TKVC000113" (creature token illusion Token) is minted and not ceased but is absent from every zone — card disappeared
```

## Step 3 — Per-card handler sweep

The Fix 5 mirror covers every per_card handler that uses the package-
local `removePermanent` helper (Brago, Deadeye Navigator, Displacer
Kitten, Emiel, Yorion, Jadzi, Wan Shi Tong, Selenia, banisher-priest
family, commander staples, and ~20 others). No per-card handler-level
changes needed — the chokepoint covers them all.

The mutate/meld paths in `keywords_batch6.go` route absorbed cards
through `MergedCardPtrs` (Phase 8) which the census already walks, so no
additional work needed there.

Foretell / madness / plot / mayhem private zones already have their
*Card pointers walked by the census (`internal/gameengine/invariants.go`
lines 227–249). The leaks weren't in those zones per the categorization;
the dominant remaining cluster was non-canonical token removal.

## Step 4 — Verification

### 500-game seed-42 ladder

| Version | Violations | Δ vs Phase C baseline | Notes |
|---------|-----------|----------------------|-------|
| Phase C baseline (strict ON, pre-Phase-D) | 52,620 (in 498 games) | — | All OG, all turn 1–2 cleanup — Scryfall token corpus seeding |
| + Fix 1 (SBA §704.5d cessation) | 1,952 (in 114 games) | **-96.3%** | Closes the OG-token-corpus cluster |
| + Fix 2 (SBA §704.5e cessation) | 1,952 | (no delta — IsCopy not common in chaos) | Correctness fix, low chaos volume |
| + Fix 3 (loop-shortcut copy cessation) | 1,952 | (no delta — rare path) | Correctness fix, low chaos volume |
| + Fix 4 (gs.removePermanent chokepoint) | 1,900 (in 112 games) | -96.4% | Catches non-canonical engine paths |
| + Fix 5 (per_card removePermanent mirror) | 1,938 (in 113 games) | -96.3% | Slight noise from RNG re-sequencing |
| + Fix 7 (chaos corpus token filter) | **1,694** (in 109 games) | **-96.8%** | Final 500-game number |

### 5000-game seed-42 sweep

`/tmp/zc_phase_d_5000.md`: **18,754 violations in 1,111 games**
(out of 5000 = 22.2% non-clean rate). Per-game density: 3.75 vs the
Phase C baseline of ~116. **96.8% reduction at 5000-game scale.**

### 25k verification (final)

`--games 25000 --seed 42 --invariant zone-conservation --nightmare-boards 0`:

| Metric | Phase C baseline (PR #767) | Phase D | Δ |
|--------|----------------------------|---------|---|
| Total games | 25,000 | 25,000 | — |
| Violation games | ~22,000 (est.) | 5,805 | -73.6% |
| Clean games | ~3,000 | 19,195 | +540% |
| Total violations | **2,904,066** | **98,230** | **-96.6%** |
| Per-game violation density | ~116 | **3.93** | **-96.6%** |

**Goal of <100 hits not met in this PR.** The remaining ~98k are real
engine bugs at the long-tail edge of token-leak coverage:

- Tokens minted via per_card paths that DON'T use the per_card
  `removePermanent` helper (raw `gs.removePermanent` calls in
  `resolve_helpers.go`, `keywords_batch3.go`, etc. are caught by Fix 4;
  but tokens that get destroyed via mutate-eats-other or absorbed into
  meld and then unmerged might have edge-case ID-routing issues).
- Tokens whose *Card pointer survives a control-change cycle (Bribery
  → opponent destroys → owner-graveyard routing) where the ID-tracking
  doesn't follow the *Card across seats.
- Race conditions during seat-elimination (HandleSeatElimination
  ceases the leaving seat's *Card pointers, but a window may exist
  where the seat is `Lost=true` but not yet `LeftGame=true` and the
  invariant fires on partial state).

The single-game diagnostic harness (`phase_d_diag_test.go`) does NOT
reproduce these long-tail leaks in single-game test mode — the leaks
appear to require Loki's parallel runner or specific seat-elimination
sequencing. Closing the residual will need a more targeted next-phase
(Phase E) effort.

## Build + test suite

```
go build ./...                  → clean
go test ./...                   → clean (full suite passes)
go test ./internal/gameengine/... → clean
```

## LOC accounting

- Production: ~120 lines (chokepoint at gs.removePermanent +
  per_card+helper mirrors + SBA cessation arms + invariant name
  enrichment + ID→name map)
- Test: diagnostic harness in cmd/hexdek-loki (~270 lines, skipped by
  default)

Well under the 800/300 budget.

## What ships

- `internal/gameengine/state.go` — universal token cessation chokepoint
  + MintedInstanceIDNames side-map
- `internal/gameengine/sba.go` — SBA §704.5d + §704.5e InstanceID
  cessation
- `internal/gameengine/loop_shortcut.go` — §707.10 copy cessation on
  stack evacuation
- `internal/gameengine/instanceid_mint.go` — RecordMintedInstanceIDName
  + wires into MintOG/Token/Copy
- `internal/gameengine/invariants.go` — disappearance message includes
  card name
- `internal/gameengine/per_card/helpers.go` — per_card removePermanent
  token cessation mirror
- `internal/gameengine/keywords_misc.go` — removePermanentFromBattlefield
  token cessation mirror
- `cmd/hexdek-loki/main.go` — chaos corpus token filter
- `cmd/hexdek-loki/phase_d_diag_test.go` — skipped diagnostic harness
- `docs/loki-phase-d-postclose-r60.md` — this document
