# InstanceID Source Audit (r60)

## Summary

Per-site triage of every raw `&Card{...}` / `new(Card)` / `^Card{...}`
construction in the non-test codebase against the four canonical
InstanceID chokepoints (`MintOGInstanceID`, `MintTokenInstanceID`,
`MintCopyInstanceID`, `MintSpellCopy`, `MintTokenAsCopyOf`) plus the
defensive `EnsureTokenInstanceID` net wired into
`FirePermanentETBTriggers`.

**Result:** 42 raw construction sites across 22 files. 41 are either
(a) different-package `Card` types (oracle / moxfield / game), (b)
covered by explicit mint calls within the same function, or (c)
covered by the `EnsureTokenInstanceID` defensive net via
`FirePermanentETBTriggers`. **1 real latent bypass found and fixed**:
`loop_shortcut.go:223` (Loop Token mint).

## Methodology

```bash
grep -rn -E "&Card\{|new\(Card\)|^Card\{" internal/ cmd/ \
  --include="*.go" | grep -v "_test.go"
```

42 hits. Each was classified by:

1. Reading 60-line window around the hit.
2. Checking for any `Mint*InstanceID` / `MintSpellCopy` /
   `MintTokenAsCopyOf` / `EnsureTokenInstanceID` /
   `FirePermanentETBTriggers` call.
3. For non-engine hits, confirming the package's `Card` type is a
   distinct schema (no `InstanceID` field).

## Triage results

### (a) Non-engine Card types — 8 sites, N/A

These are unrelated `Card` structs in sibling packages — they don't
have an `InstanceID` field at all and aren't part of the InstanceID
census surface.

| Site | Package | Notes |
|------|---------|-------|
| `internal/moxfield/resolver.go:94` | `moxfield` | deck-import DTO |
| `internal/oracle/scryfall.go:568` | `oracle` | Scryfall cache row |
| `internal/oracle/scryfall.go:705` | `oracle` | Scryfall named-API response |
| `internal/oracle/scryfall.go:744` | `oracle` | DB cache scan target |
| `internal/game/engine.go:79` | `game` | commander DB row (uses `db.NewID(16)` for its own InstanceID) |
| `internal/game/engine.go:130` | `game` | library card DB row |
| `internal/game/storage.go:264` | `game` | DB scan target |
| `internal/game/storage.go:291` | `game` | DB scan target |

### (b) Explicit chokepoint coverage — 11 sites, covered

These call `MintTokenInstanceID` or `MintCopyInstanceID` directly on
the constructed `*Card` within ~15 lines of the mint site.

| Site | Chokepoint | Card kind |
|------|------------|-----------|
| `internal/gameengine/resolve.go:1921` | `MintTokenInstanceID` | generic token mint |
| `internal/gameengine/cast_counts.go:536` | `MintTokenInstanceID` | Treasure |
| `internal/gameengine/cast_counts.go:703` | `MintTokenInstanceID` | creature token |
| `internal/gameengine/tokens.go:87` | `MintTokenInstanceID` | artifact token |
| `internal/gameengine/tokens.go:145` | `MintTokenInstanceID` | generic token |
| `internal/gameengine/keywords_storm_rider.go:155` | `MintCopyInstanceID` | storm copy |
| `internal/gameengine/keywords_stubs_tail.go:229` | `MintCopyInstanceID` | tier copy |
| `internal/gameengine/keywords_batch3.go:197` | `MintCopyInstanceID` | generic copy |
| `internal/gameengine/keywords_batch6.go:890` | `MintCopyInstanceID` | gravestorm copy |
| `internal/gameengine/keywords_demonstrate.go:203` | `MintCopyInstanceID` | demonstrate copy |
| `internal/gameengine/state.go:2159` | n/a (comment) | inside docstring, not a real mint |

### (c) Defensive net coverage — 21 sites, covered

These construct a token `*Card` inside a `*Permanent` that goes through
`FirePermanentETBTriggers`, which invokes `EnsureTokenInstanceID`
before the trigger cascade. Every constructed `Card` has `Types`
containing `"token"` (the gate the defensive net checks).

| Site | Token name | ETB-dispatched |
|------|------------|----------------|
| `internal/gameengine/keywords_batch5.go:164` | Manifested Creature | yes |
| `internal/gameengine/keywords_batch5.go:617` | Manifested Creature (dread) | yes |
| `internal/gameengine/keywords_batch.go:83` | Persist return | yes |
| `internal/gameengine/keywords_batch.go:136` | Undying return | yes |
| `internal/gameengine/keywords_batch.go:692` | Spirit | yes |
| `internal/gameengine/keywords_batch.go:1129` | Incubator | yes |
| `internal/gameengine/keywords_batch3.go:568` | Squad token | yes |
| `internal/gameengine/keywords_batch6.go:1511` | Mercenary | yes |
| `internal/gameengine/keywords_combat.go:438` | myriad copy | yes |
| `internal/gameengine/keywords_manifest.go:139` | Face-Down (manifest) | yes |
| `internal/gameengine/keywords_manifest_dread.go:169` | Manifested Creature | yes |
| `internal/gameengine/keywords_misc.go:237` | Embalmed | yes |
| `internal/gameengine/keywords_misc.go:307` | Eternalized | yes |
| `internal/gameengine/keywords_misc.go:373` | Encore | yes |
| `internal/gameengine/keywords_misc.go:845` | Servo | yes |
| `internal/gameengine/keywords_misc.go:1211` | Phyrexian Germ | yes |
| `internal/gameengine/keywords_misc.go:1528` | Cloaked (face-down) | yes |
| `internal/gameengine/resolve_helpers.go:31` | Face-Down (manifest) | yes |
| `internal/gameengine/resolve_helpers.go:965` | token copy | yes |
| `internal/gameengine/resolve_helpers.go:2936` | Zombie Army | yes |
| `internal/gameengine/resolve_helpers.go:3189` | token copy of source | yes |

### (d) Safe-by-design transient — 1 site

| Site | Reason |
|------|--------|
| `internal/gameengine/keywords_tempting_offer.go:164` | `syntheticTemptingOfferSource` returns a placeholder `*Permanent` that is explicitly NOT added to any battlefield — exists only for the duration of one `ResolveEffect` call as a fake source pointer, then garbage-collected. The docstring at line 155-161 documents this. No mint required because the card never enters the InstanceID census surface. |

### (e) LATENT BYPASS — 1 site, fixed

| Site | Bug |
|------|-----|
| `internal/gameengine/loop_shortcut.go:223` | Loop Token minted directly to `s.Battlefield` via raw `s.Battlefield = append(...)`. The §727 loop-shortcut path collapses N cycles to a single state mutation, so it intentionally bypasses `FirePermanentETBTriggers` (to skip observer triggers). That also bypasses the defensive `EnsureTokenInstanceID` stamp. Worse: the original Card had `TypeLine: "Token Creature"` but `Types` was empty — so even if `EnsureTokenInstanceID` had been called manually, it would have no-op'd (the helper gates on `Types` containing `"token"`, not on TypeLine). Tokens entered the battlefield with no InstanceID at all — invisible to the census, untraceable through the InstanceID lineage system. Fix: added `Types: []string{"token", "creature"}` and an explicit `MintTokenInstanceID(gs, tokenCard, "", currentMintEnablerID(gs))` call before constructing the `*Permanent`. |

This is the only real engine bypass surfaced by the audit.

## Why this matters

The 4 canonical chokepoints + the defensive net are the entire
contract for the InstanceID census. A bypass means the census is
blind to those cards: they don't fabricate (no ID present), they
don't disappear (no ID minted to compare against), but they're also
not part of any lineage chain — a future invariant or forensics
trace can't follow them.

The loop_shortcut path is exercised whenever Loki's chaos corpus
hits a mandatory-loop scenario (Worldgorger Dragon + Animate Dead,
Heliod + Walking Ballista, etc.). The 2026-05-23 r43/game-420 ZoneConservation
cluster in CLAUDE.md (commit `c711b1a`) was the prior fire-fight
around this path's stack-evacuation. The token-mint bypass is a
sibling defect on the same path.

## Fix verification

- `go build ./...` clean.
- `go test ./internal/gameengine/...` — 14.5 s, all 4 packages pass.

A targeted Loki re-run on a mandatory-loop seed would be the
authoritative validation but is out of scope for this audit PR
(`scripts/run-forensics.sh` will catch any regression on the next
CI run).

## Recommended follow-ups

- Make `EnsureTokenInstanceID` gate on either `Types` containing
  `"token"` OR `TypeLine` starting with `"Token"`. That would have
  caught the loop_shortcut bug defensively, and adds resilience for
  any future per_card handler that populates `TypeLine` but forgets
  `Types`. Small one-line change in `instanceid_mint.go:251-258`.
- Add a `vet`-style lint check that flags any `&Card{` in
  `internal/gameengine/` not within 20 lines of a `Mint*InstanceID` /
  `EnsureTokenInstanceID` / `FirePermanentETBTriggers` / synthetic-
  transient comment. Would catch future regressions of this exact
  shape without needing a Loki run to surface them.
