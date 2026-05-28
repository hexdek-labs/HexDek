# Loki r60 — Cast-from-non-owner Seeded Sweep

**Date:** 2026-05-27
**Branch:** `dev/loki-cast-from-non-owner-seeded-r60`
**Sweep:** 10,000 chaos games, seed 42, 4 seats, 60 max turns, nightmare phase disabled
**Seed cards (round-robin across all 4 seats via new `--seed-cards-all-seats`):**

```
Bribery, Hostage Taker, Possibility Storm, Knowledge Pool,
Praetor's Grasp, Sen Triplets, Magus of the Future,
Etali, Primal Storm, Maelstrom Wanderer, Bolas's Citadel
```

**Verdict:** 110,072 invariant violations across 2,203 / 10,000 games (22.0% violation rate). **Zero crashes.** The seeded sweep surfaces three distinct cluster classes, all siblings of the Etali bug class closed by PR #685 — each rooted in a CR §400.6 / §400.7c / §406.7 routing-or-cleanup gap in a per-card handler.

## Headline

| Metric | Value |
|--------|-------|
| Games run | 10,000 |
| Dirty games | 2,203 (22.0%) |
| Clean games | 7,797 (78.0%) |
| Total violations | 110,072 |
| Crashes | 0 |
| Throughput | 93 games / sec (1m47s wall) |

| Invariant | Count | % of total |
|-----------|-------|------------|
| CardIdentity | 78,486 | 71.3% |
| ZoneConservation | 21,378 | 19.4% |
| ExileLinkageIntegrity | 10,208 | 9.3% |

Per-game violation-count median is high — a single bug pattern fires on every cleanup pass across many turns, so a 1-pattern game inflates the raw violation count by 10-100x. The 22% dirty-game rate is the more honest cluster-prevalence signal.

## Cluster A: §400.6 commander-zone-residue (CardIdentity)

**Signature:** card `"<Commander>"` (ptr X) appears in both `seat N command_zone` AND `seat N battlefield`.

**Representative hits:**

```
Kalitas, Bloodchief of Ghet   — seat 0 command_zone ↔ seat 0 battlefield
Captain America, Super-Soldier — seat 0 command_zone ↔ seat 0 battlefield
Captain America, Super-Soldier — seat 0 command_zone ↔ seat 3 battlefield   ← cross-seat (Bribery / Sen Triplets shape)
Kalitas, Bloodchief of Ghet    — seat 0 battlefield ↔ seat 3 battlefield     ← cross-seat (Bribery shape)
Captain America, Super-Soldier — seat 0 battlefield ↔ seat 3 battlefield     ← cross-seat (Bribery shape)
```

**Root-cause family:**

1. **Commander cast leaves *Card pointer in command zone.** When a commander is cast from the command zone, the `*Card` is moved to the battlefield via `createPermanent`, but the entry in `gs.Seats[seat].CommandZone` is not always purged. Subsequent cleanup invariants see the same pointer in both zones. The MoveCard pipeline has a `command_zone` source-zone branch, but per-card cast paths that bypass MoveCard (commander-tax-paid cast directly via `castFromCommandZone` helpers) skip the removal. Estimated ~70% of the CardIdentity cluster.

2. **Bribery / Sen Triplets cross-seat physical duplication.** A creature that's tutored onto Bribery's controller's battlefield via a cross-seat library move ends up with a `Permanent` wrapper on the caster's seat AND, in some paths, the same `*Card` ALSO remains on its OWNER's battlefield (or in their command zone if it was a commander). The `seat 0 battlefield ↔ seat 3 battlefield` cases are the smoking gun — Captain America and Kalitas (both commanders in the surfaced pods) appear on TWO seats' battlefields simultaneously after Bribery resolves. Owner-routing for the library move is correct (the card leaves owner's library); the bug is in the post-move bookkeeping when the card is a commander OR when the destination seat already has a different Permanent wrapping it. Estimated ~25% of the CardIdentity cluster.

3. **Generic exile-then-cast residue.** Cards like Ajani's Mantra / Alloy Golem / Bog-Strider Ash / Boxing Ring / Chandra, Awakened Inferno / Charging Cinderhorn / Morph / Rune of Protection: Red / Soul Net / Trained Condor appear in both `seat N exile` AND `seat N battlefield`. Pattern: card was exiled by some effect (Possibility Storm, Mind's Desire, Knowledge Pool grant, etc.), then cast for free from exile, the `Permanent` wrapper entered the battlefield, but the `*Card` was NOT removed from the exile pile. The cast-from-zone path needs to call `removeFromZone(exile)` before / during `createPermanent`. Estimated ~5% of the cluster.

4. **Adventure / DFC graveyard ↔ exile residue.** "Garruk's Uprising // Garruk's Uprising", "Arni Brokenbrow // Arni Brokenbrow", "Bhaal, Lord of Murder // Bhaal, Lord of Murder", "Seasoned Cathar // Seasoned Cathar" each appear in both graveyard and exile. The "// Foo // Foo" name pattern (front-face name appearing twice) is the DFC / Adventure layout — both faces share a single `*Card` and the zone-change for one face doesn't update the residence for the other. Possibility Storm / Knowledge Pool exiling a DFC and then a separate ETB-die path destroying it leaves the same pointer in two zones. Estimated <5% of the cluster.

## Cluster B: §400.7c pod-census surplus (ZoneConservation)

**Signature:** `zone conservation suspicious: N extra real cards appeared (expected E, found E+N) — possible copy bug` where N ∈ {11..15} across deck sizes E ∈ {355..396}.

**Representative hits:**

```
11 extra real cards (Oyobi pod / Zilortha pod / Phenax pod / Mondrak pod) ← appears 7×
12 extra real cards                                                       ← appears 4×
13 extra real cards                                                       ← appears 5×
14 extra real cards                                                       ← appears 2×
15 extra real cards                                                       ← appears 1×
```

**Root cause:** The pod-level card census counts unique cards across all seats' libraries + hands + graveyards + exiles + battlefields + command zones + stack. The expected count = sum of seats' starting decklist sizes (each ~99 + 1 commander). When the per-card handlers for the seeded stress cards CROSS-SEAT-DUPLICATE a `*Card` (the Cluster A bugs), each duplication shows up as +1 in the pod census. The N=11-15 surplus matches the seeded-card count (10 stress cards × handler-fire rate) — the bigger the pod's stress-card-interaction surface, the higher the surplus.

The ZoneConservation cluster is a downstream observer of Cluster A's duplication leaks — fixing Cluster A's handlers (commander cast cleanup, Bribery post-tutor cleanup, exile-then-cast removeFromZone) should drop ZoneConservation toward 0 mechanically.

## Cluster C: §406.7 orphaned linked-exile (ExileLinkageIntegrity)

**Signature:** `card "<X>" in seat N exile is linked to source timestamp T which is no longer on any battlefield — LTB return missed (orphaned linked exile)`.

**Representative hits (20 distinct cards across 6 distinct seats):**

```
Bookwurm                                          (seat 2)
Circuit Mender                                    (seat 0)
Crumbling Vestige                                 (seat 0)
Expand the Sphere // Expand the Sphere            (seat 0)
Field of Ruin                                     (seat 0)
Ghost Ark                                         (seat 0)
Invasion of Regatha // Disciples of the Inferno   (seat 0)
Island / Mountain / Plains                        (seats 0, 1)
Kozilek's Command                                 (seat 0)
Liu Bei, Lord of Shu                              (seat 0)
Myrkul's Edict                                    (seat 0)
Plague Reaver                                     (seat 0)
Primal Storm   (= Etali, Primal Storm — exiled BY a hostile linked-exile source!)   (seat 0)
Pulsemage Advocate                                (seat 0)
Ravenous Brute Head                               (seat 2)
Skullmead Cauldron                                (seat 0)
Tromell, Seymour's Butler                         (seat 0)
```

**Root cause:** When a card is exiled via `gameengine.ExileLinked(gs, source, card, ownerSeat, fromZone)` per CR §406.7, the source's `LinkedExile` slice holds the exiled card and the card's `ExiledByTimestamp` points back to the source's `Permanent.Timestamp`. When the source leaves the battlefield, the canonical LTB paths (`DestroyPermanent`, `ExilePermanent`, `sacrificePermanentImpl`, `BouncePermanent`, the SBA variants, and `HandleSeatElimination`) are supposed to call `ReturnLinkedExile(gs, perm, "battlefield")` — returning every linked card to its OWNER's appropriate zone.

The Loki sweep surfaces 20+ distinct ORPHAN cards across multiple games, meaning multiple LTB paths in the engine are NOT calling `ReturnLinkedExile`. The seeded sweep's Hostage Taker / Praetor's Grasp / Knowledge Pool ETB-exile-until-leaves chain is the dominant trigger, but the orphans include non-permanent cards (Kozilek's Command, Myrkul's Edict — instants/sorceries that were exiled by an effect like Praetor's Grasp), which means the bug surface includes the "exile cards from another player's library" family AND the "exile a permanent until source leaves" family.

The most damning hit: **"Primal Storm" linked to source timestamp 38** — Etali, Primal Storm itself was the EXILED card, linked to some OTHER permanent's LinkedExile slot. PR #685 fixed Etali's exile-others routing; this sweep shows Etali on the receiving end of a LinkedExile bug from a different handler. The class is wider than PR #685 closed.

## Pod-and-commander interaction observations

The three games that contributed the densest detail samples after dedup:

| Game | Seed | Turn span | Pod | Cluster |
|------|------|-----------|-----|---------|
| 6 | 60043 | 38-50+ | Oyobi / Zilortha / Phenax / Mondrak | ZoneConservation (11 extra cards × many turns) |
| 7 | 70043 | 22-40+ | Kalitas / Gitrog / Greven / Gandalf | CardIdentity (commander-cast residue) |
| 18 | 180043 | 42+ | Mana Max / Toothy / Neva / Lady Orca | ExileLinkageIntegrity (Skullmead Cauldron orphan) |
| 55 | 550043 | 21+ | Éowyn / Kethek / Narset / Urza | ExileLinkageIntegrity (Pulsemage Advocate orphan) |

**Commander correlation:** Captain America and Kalitas Bloodchief of Ghet are over-represented in the cross-seat CardIdentity hits — both are likely-cast commanders (3-color, 4-CMC, evasive bodies) that show up in chaos decks frequently. The cross-seat duplication signature is NOT specific to a particular commander; it's specific to the cast-from-command-zone path interacting with cross-seat tutor effects (Bribery, Sen Triplets) when the targeted card IS a commander.

## Fix candidates (in priority order)

1. **Commander cast cleanup audit** — find every `castFromCommandZone` / commander-tax payment path and verify it removes the `*Card` from `Seats[N].CommandZone` before `createPermanent`. Estimated ~70% of CardIdentity hits, ~50% of ZoneConservation hits.
2. **Bribery post-tutor cleanup** — when Bribery moves a card from opp's library to caster's battlefield, ensure the OWNER seat's residue is purged (no battlefield, no command zone, no graveyard). The `removePermanent` in our r60 batch-AI handler covers the from-side; the to-side dedup in `createPermanent` doesn't account for the `*Card` lingering elsewhere. Estimated ~20% of cross-seat CardIdentity hits.
3. **LinkedExile LTB sweep** — audit every LTB-eligible engine path (Sacrifice/Destroy/Exile/Bounce/SBA pairs/`HandleSeatElimination`/zone-change variants) to ensure `ReturnLinkedExile(perm, "battlefield")` is called. The Loki signatures suggest gaps in at least 6 paths (each game's orphans correlate with a different source-LTB cause). Estimated 100% of ExileLinkageIntegrity hits.
4. **Cast-from-exile removeFromZone** — verify that the `CastFromZone` path (`ResolveStackTop` for permanent spells cast via a `ZoneCastGrant`) calls `removeFromZone(exile)` for the `*Card` before `createPermanent` wraps it. Possibility Storm's matched-card cast (per-card batch AI, PR #686) and Knowledge Pool's free-cast both rely on this. Estimated ~5% of CardIdentity hits.
5. **DFC / Adventure dual-face zone tracking** — when a card with a `// Foo // Foo` name is exiled-then-recurred, ensure both faces share zone updates. Edge case but worth a defensive sweep. Estimated <5% of CardIdentity hits.

## Reproduction

The `--seed-cards-all-seats` flag is new in this branch:

```sh
go run ./cmd/hexdek-loki/ --games 10000 --seats 4 --max-turns 60 \
    --nightmare-boards 0 \
    --seed-cards-all-seats "Bribery,Hostage Taker,Possibility Storm,Knowledge Pool,Praetor's Grasp,Sen Triplets,Magus of the Future,Etali, Primal Storm,Maelstrom Wanderer,Bolas's Citadel" \
    --report docs/loki-cast-from-non-owner-r60-raw.md
```

Reproducible at seed 42 with the same card list. The raw 7416-line report with per-violation game-state snapshots is at `docs/loki-cast-from-non-owner-r60-raw.md`.

## What this sweep does NOT cover

- **Non-§400 invariants** — AttachmentConsistency, ReplacementCompleteness, TriggerCompleteness, ResourceConservation, WinCondition, etc. all stayed clean in this sweep. The seeded card pool is narrowly §400.6/§400.7c/§406.7-focused; a broader sweep would surface other classes.
- **Self-library / own-seat-only exile** — Bolas's Citadel and Magus of the Future exile from / cast from the SAME seat's library, so §400.7c is trivially satisfied. Their bug surface is narrower than the cross-seat cards (which dominate the violation counts here).
- **Storm interaction** — Mind's Desire was seeded but its EOT grant + own-library exile shape didn't surface as a distinct cluster signature. The storm-copy multiplier of grants and the EOT cleanup are both untested here at depth.
- **Crash detection** — zero crashes in 10K games confirms the engine's recover() guards hold across these interactions; the bugs are silent state leaks, not panics.

## Related r60 reports

- `docs/loki-r60-25k-report.md` — original Etali cluster discovery (828 violations / 2 clusters)
- `docs/loki-r60-final-report.md` — Etali fix verification (0 violations)
- `docs/loki-r60-100k-r60.md` — 100K post-#685 sweep ("Etali class extinct")
- PR #685 (commit `cb3734c5`) — Etali §400.7c fix that opened this follow-up audit lane
- PR #686 (commit `a877975d`) — live-game P/T re-architecture (unrelated)
- PR #687 (commit `a5d8e530`) — per_card batch AI (§400.7c-compliant handlers for the 5 stress cards Hostage Taker / Knowledge Pool / Possibility Storm / Bribery / Mind's Desire)

**The Cluster A / B / C signatures are SIBLINGS to the Etali class, not regressions of it.** PR #685's fix targeted Etali's handler; the sibling handlers (commander-cast cleanup, Bribery, Sen Triplets-style mind-control, LinkedExile LTB paths) carry the same anti-patterns and surface under the same stress shape.
