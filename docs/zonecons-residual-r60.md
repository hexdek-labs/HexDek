# ZoneConservation Residual Classification — Phase G (post-#855)

**Date:** 2026-05-30
**Branch:** `dev/zonecons-residual-classification-r60` (cut from `origin/main` @ `6f3e912e`)
**Command:** `go run ./cmd/hexdek-loki --games 5000 --seed 42 --violations-dump …`
**Sample:** 5,000 chaos games + 10,000 nightmare boards, seed 42.

## Headline

**Pre-Phase-G state: 34 ZoneConservation violations in 1 game / 0 crashes / 0 nightmare hits.**

Every single one of the 34 residual hits is the SAME fabrication
signature — one InstanceID, one game, repeating across turns 44–60.
This is a **(b) structural** residual class, not an (a) per-card
weird-interaction tail edge.

## Per-signature breakdown

| Signature | Hits | Games | InstanceID | Card | Cause |
|-----------|-----:|------:|------------|------|-------|
| fabrication / Lash Out | 34 | 1 (game 2762) | `h1OGVR200056` | Lash Out | Aziza, Mage Tower Captain's spell-copy handler aliases the source `*Card` pointer directly into the §707.2 StackItem. On §707.10 cease at `stack.go:1312`, `MarkInstanceIDCeased(item.Card.InstanceID)` retires the SOURCE's OG ID. Every subsequent invariant tick walks seat 1's hand/graveyard, finds Lash Out still present, and flags it. |

That's the entire residual. No disappearance arm, no
ExileLinkageIntegrity, no AttachmentConsistency, no second per_card
signature.

## Classification

This is the SAME `stack.go:1312` cease-the-source mechanism Phase F's
`MintSpellCopy` chokepoint targeted (10 sites closed: `alania` / `zada` /
`krark` / `mica` / `mendicant` / `rootha` / `kalamax` / `ivy` /
`fire_lord_azula` / `ulalek`). Phase F audit caught Aziza but
intentionally deferred it because the trace showed Phase F's primary
fix (CleanupHandSize LeftGame skip) was the active cause of the dominant
game-411 / Distemper cluster, not Aziza.

The Aziza site is **structurally worse** than the 10 Phase F sites: they
called `DeepCopy()` first (sharing the ID via field inheritance); Aziza
aliases the pointer outright (`Card: castCard`). Both shapes drive the
same cease-the-source mechanism on resolve, with the alias shape being a
strict superset of the bugs the DeepCopy-share shape produced.

A repo-wide grep for the same anti-pattern surfaces **two** call sites
in total:

```
$ grep -rn "Card:\s*castCard\|Card:\s*spellCard\|Card:\s*item\.Card\b" \
    internal/gameengine/per_card/ internal/gameengine/ --include="*.go" \
    | grep -v "_test\.go" | grep -v "MintSpellCopy\|DeepCopy"

internal/gameengine/per_card/aziza_mage_tower_captain.go:96:		Card:       castCard,
internal/gameengine/keywords_batch4.go:562:		Card:       item.Card,
```

- **Aziza** (Mage Tower Captain) — actively causing the 34-hit Loki
  residual on this seed.
- **Conspire** (keyword §702.78, `keywords_batch4.go`) — same bug
  shape, dormant on seed-42 chaos (no Conspire card surfaced into the
  copy path at this depth), but a real correctness bug that would fire
  the same cease-the-source leak when a Conspire-keyword spell gets
  cast with two color-matching untapped creatures available.

Other sites with `IsCopy: true` in the engine (`keywords_storm_rider.go`,
`keywords_stubs_tail.go`, `resolve.go`'s canonical `resolveCopySpell`)
either build a fresh `*Card` struct from scratch or already route through
`MintCopyInstanceID` — they don't share the alias bug.

## Recommended path: (b) structural fix

Both call sites collapse to the same one-line patch — route through the
existing `gameengine.MintSpellCopy` chokepoint:

```go
copyCard := gameengine.MintSpellCopy(gs, sourceCard)
copyItem := &gameengine.StackItem{
    Card: copyCard,
    ...
}
```

`MintSpellCopy` (added in Phase F #851) handles the full canonical
sequence: `DeepCopy` the source, clear the inherited `InstanceID`, stamp
`IsCopy=true`, mint a fresh CP-provenance ID with `SourceInstanceID`
recording the lineage. When the copy resolves and `stack.go:1312` ceases
the copy's CP ID, the source's OG ID is untouched.

Per_card audit confirms Aziza is the LONE per_card site with this exact
anti-pattern (the other 10 Phase F sites all used `DeepCopy` first; the
Phase F sweep closed them). The engine-side Conspire site is the only
non-per_card sibling.

## Phase G scope (this PR)

1. **Aziza fix** — `internal/gameengine/per_card/aziza_mage_tower_captain.go:96`
   route through `MintSpellCopy`.
2. **Conspire fix** — `internal/gameengine/keywords_batch4.go:562` same
   one-line patch.
3. **Existing Aziza test update** — `TestAziza_R51_CopiesSpellOnInstantCast`
   used to ASSERT `top.Card == castCard` (pinning the bug). Flipped to
   pin `top.Card != castCard` plus the InstanceID + IsCopy invariants
   `MintSpellCopy` guarantees.
4. **New regression** — `TestAziza_PhaseG_SourceInstanceIDSurvivesCopyResolution`
   pins the end-to-end shape: build an OG-minted source, fire Aziza,
   simulate §707.10 cease on the copy's CP ID, assert the source's OG
   ID is NOT ceased.

## Verification

- Engine + per_card + counters + instanceid suites: PASS.
- Loki 5K seed-42 post-fix run on this branch:

  ```
  === CHAOS GAMES COMPLETE ===
    games:           5000
    duration:        5m43.13s
    throughput:      14 games/sec   (system was running 3 parallel Loki workers)
    crashes:         0 (in 0 games)
    violations:      0 (in 0 games)
    clean games:     5000

  === NIGHTMARE BOARDS COMPLETE ===
    boards:          10000
    duration:        2.333s
    crashes:         0
    violations:      0
    clean boards:    10000
  ```

  **34 → 0 ZoneConservation violations.** The Aziza-driven Lash Out
  fabrication is gone. Engine + nightmare both clean across the full
  seed-42 chaos surface at 5K depth.

## Verdict

Single-signature, single-cause structural residual. Path **(b)**:
implement the fix this PR. No per-card weird-interaction tail edges
remain at this seed/depth.

— Phase G / 2026-05-30
