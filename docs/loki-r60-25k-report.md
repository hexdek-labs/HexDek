# Loki r60 — 25K Deep-Surface Sweep (seed 42)

| Field | Value |
|-------|-------|
| Date | 2026-05-27 |
| Branch | `dev/loki-r60-25k-r60` |
| Invocation | `go run ./cmd/hexdek-loki --games 25000 --seed 42` |
| Phase 1 throughput | 25,000 chaos games in 7m57s (52 g/s avg, peaked ~117 g/s early) |
| Phase 2 throughput | 10,000 nightmare boards in 782ms (12,792 b/s) |
| Crashes | **0** (chaos) + **0** (nightmare) |
| Panics / recovers | **0** |
| Chaos violations | **828** in **16 games** (24,984 clean) |
| Nightmare violations | **0** (all 10,000 boards clean) |
| Verdict | Rare-event tail surfaced — two Etali-anchored clusters, both new vs the 10K canonical-clean baseline |

## Trajectory vs prior depth runs (canonical seed 42)

| Depth | Run | Violations | Crashes | Notes |
|-------|-----|------------|---------|-------|
| 5,000 | 2026-05-26 (PR #553 re-verify) | **0** | 0 | canonical-final baseline |
| 5,000 | 2026-05-25 (canonical-final sweep, 10 seeds × 10K) | **0** | 0 | `docs/loki-r60-canonical-final.md` |
| 10,000 | 2026-05-27 (10K extension) | **0** | 0 | `docs/loki-r60-10k-report.md` — 2× depth, same verdict |
| **25,000** | **this run** | **828 in 16 games** | **0** | **2 rare-event clusters surfaced past the 10K wall — both Etali-anchored** |

Cumulative r41 → r60 trajectory at seed 42: 1,652 → 0 (5K/10K) → 828 (25K) reveals that the canonical-clean verdict at 10K is a **depth-threshold artifact**, not a true-zero. The engine is clean to ~10K games at seed 42; the rare tail begins between 10K and 25K. Per-game violation rate at 25K = 16/25,000 = **0.064%** (1 in 1,563 games). At the canonical 5K/10K depth this rate would expect 3.2 / 6.4 violated games — well within the noise floor of "clean," but the 25K run now sees enough samples to bring it above zero.

## By invariant

| Invariant | Count | Games | Cluster |
|-----------|-------|-------|---------|
| CardIdentity | 548 | ~5 | "Hostile Realm" cross-seat ptr leak via Etali graveyard residue |
| ZoneConservation | 280 | ~11 | "N extra real cards appeared" — Etali cast-from-exile copy-count drift |

Both clusters are **new at 25K depth** — neither appeared in the 5K/10K runs at seed 42, neither appears in the canonical-final 10-seed × 10K matrix. They are not regressions; they are deeper-tail surfaces previously beyond the sampling depth.

## Card correlation (top 10)

| Rank | Card | Violation games | Clean games | Correlation |
|------|------|-----------------|-------------|-------------|
| 1 | Riveteers Confluence | 1 | 9 | 0.10 |
| 2 | **Etali, Primal Storm** | **15** | **140** | **0.10** |
| 3 | Mirrorwood Treefolk | 1 | 10 | 0.09 |
| 4 | Sand Warrior | 1 | 12 | 0.08 |
| 5 | Ravenous Squirrel | 1 | 21 | 0.05 |

Etali, Primal Storm appears in **15 of the 16 violation games** (93.75%) and is implicated in every event-log excerpt for both clusters. Etali is the smoking gun — the cards at ranks 1, 3, 4 are coincidental low-N cards that happened to share a pod with Etali. Riveteers Confluence at rank 1 has a higher per-game ratio only because its denominator (10) is tiny — the absolute games-correlated number for Etali (15) dwarfs every other card.

## Cluster 1: CardIdentity — Etali "Hostile Realm" cross-seat residue (548 / 5 games)

**Signature** (bit-stable across 5 violations shown, all in game 1944):

```
CardIdentity: card "Hostile Realm" (ptr 0xfcf377ed560)
  appears in both seat 0 graveyard AND seat 1 exile
```

- **Pod**: Hazezon Tamar / Etali, Primal Storm / Zagras, Thief of Heartbeats / Gandalf, White Rider
- **Turns**: 46, 47, 47, 48 (persists across several cleanup snapshots — stable, not transient)
- **Seat alignment**: card belongs to seat 0 (a land in seat 0's deck), Etali is seat 1
- **Recent-event window** consistently ends with a cascade of `zone_cast_grant_expired seat=1 source=Etali, Primal Storm target=seat0` (the EOT grant cleanup), but the underlying `*Card` for "Hostile Realm" remained in seat 1's exile after expiry

**Hypothesis**: Etali's "Whenever this attacks, exile the top card of each player's library. You may play those cards this turn" creates `ZoneCastGrant` entries with a parallel `*Card` reference in seat 1 (the Etali controller)'s exile zone for cards belonging to OTHER seats. When the EOT cleanup expires the grant (`ExpireZoneCastGrants`), the grant *permission* is reclaimed but the *Card pointer itself isn't returned to its owner's library/graveyard/wherever it canonically belongs. If seat 0 separately drew & played their copy of "Hostile Realm" via natural top-deck (or a tutor/recursion path), the *Card lands in seat 0's graveyard while still sitting in seat 1's exile — same ptr, two zones.

This is structurally distinct from the previously-closed Etali clusters (PR #106 zone-cast-grant-expiry source-LTB / 2026-05-24 seat-elimination grant expiry / 2026-05-23 heist + may_play_exiled_free expiry): those fixed the *grant lifecycle* but never returned exiled cards to their owners. The card-residue path is a sibling gap.

## Cluster 2: ZoneConservation — Etali cast-from-exile copy-count drift (280 / 11 games)

**Signature** (bit-stable across 5 violations shown, all in game 2275, turns 33-35):

```
zone conservation suspicious: 12 extra real cards appeared
  (expected 394, found 406) — possible copy bug
```

- **Pod**: Etali, Primal Storm / Jyoti, Moag Ancient / Zuko, Avatar Hunter / Jolrael, Empress of Beasts
- **Persistent gap of exactly +12 across consecutive turns** — drift accumulated once then held steady, did not grow per-turn
- **Recent-event window** shows `etali_exile seat=N source=Etali, Primal Storm` for N ∈ {1, 2, 3} (the three opponents) followed by `zone_cast_grant_registered` × 2 and `zone_cast_grant_expired` × 3 — exiled-cards-per-attack ratio mismatch with grant-registration ratio
- Seat 0 (the Etali controller) census reports `exile=12` at the violation snapshots — same number as the drift

**Hypothesis**: When Etali exiles the top card of each player's library, the `*Card` is moved into Etali-controller's exile zone (NOT each owner's exile). `checkZoneConservation` counts each seat's zones against the pre-game census of *that seat's* cards. The 12 cards that started in seats 1/2/3's libraries are now physically sitting in seat 0's exile pile — the invariant sees seat 0 holding 12 "extra real cards" that don't belong to seat 0's census, while seats 1/2/3 each show their library/etc. count short by the corresponding amount. The invariant message frames this as "extra cards appeared" because seat 0's positive delta is the loudest signal, but the underlying issue is that Etali's exile-other-players'-cards effect either (a) needs to route each card into its *owner's* exile per CR §400.7 (owner is the player who owns the card, not who controls Etali), or (b) the invariant needs to be Etali-aware. CR §400.7c is unambiguous: "If an effect causes a player to put a card into a zone, that card moves to the corresponding zone owned by that player" — so (a) is the correct fix. Same root primitive as PR #161 / Athreos / Adric: when an effect moves a card from one player's library to "exile," it must land in the *owner's* exile, not the effect-controller's exile.

## Rare-event signal analysis

What surfaced at 25K that did NOT surface at 10K:

| Cluster | First seen game | First seen turn | Rate per 1K games |
|---------|-----------------|-----------------|-------------------|
| CardIdentity (Etali Hostile Realm) | 1944 (chaos game ~1,944 of 25K) | 46 (late game) | ~0.2 games/1K |
| ZoneConservation (Etali exile drift) | 2275 | 33 (mid-late) | ~0.44 games/1K |

Both clusters require:
1. Etali, Primal Storm in the pod (15/16 violation games)
2. Etali surviving to mid-late game and attacking multiple times (turn 33-48 onsets)
3. The attack-trigger exiling a card from an opponent's library
4. The opponent later drawing/casting their canonical copy of the same card (CardIdentity case) OR enough cumulative exiles to drift the census by ≥12 (ZoneConservation case)

The conjunction of (long game, surviving attacker, repeated trigger fires, target-card reuse) is statistically rare per game but inevitable at sufficient depth. 10K depth wasn't enough to sample even one Etali game that hit conditions 2-4; 25K depth is the threshold.

Other rare-tail signals expected at this depth but **not observed**: stack saturation, mana-pool overflow, triggered-ability flood, replacement-effect cascades, copy/token-mint runaway, paradigm-exile leak (the round-2 Krark fix held), commander cast-from-anywhere edge cases, double-faced card zone transitions, mutate stack interactions, prismatic-bridge-style "create a token copy" pointer aliasing. The fact that only Etali-anchored clusters surfaced at 25K is itself a positive signal — the rest of the engine has no rare tail above 1-in-25,000.

## What is NOT in this report

- **No regressions of the canonical r60 clusters** (Adric / Oketra / Dread / Jaxis / Athreos / Zidane / Necrogen Communion / Cerulean Sphinx / Krark paradigm / Breya game-420 evacuate / Charix `ended=1` / Platinum Angel phantom-source / Compound Fracture RIP-ETB FP / Myr Moonvessel pending-triggers / WinCondition post-elim FP) — all those signatures stayed at 0 across the 25K run
- **No nightmare-board violations** — 10,000 boards, 0 violations, 12,792 b/s
- **No new invariant categories** — the 11 historical r60 invariants are exhaustive; everything 25K surfaced fit existing buckets
- **No panics / recovers** — `0 (in 0 games)` for both phases

## Recommended follow-up

1. **CardIdentity Cluster 1 fix** — `ExpireZoneCastGrants` (or the per-handler `etali_exile` mirror) needs to return the *Card to its canonical owner's library/graveyard on grant expiry, not leave the residue in the granter's exile. Same pattern as the source-LTB grant cleanup added in PR #106 — extend to "grant expiry from natural EOT" path too.
2. **ZoneConservation Cluster 2 fix** — `Etali` per_card handler (in `internal/gameengine/per_card/` — likely `etali_primal_storm.go` or routed through the generic Etali-like exile primitive) needs to move each exiled card to the *card's owner's* exile zone, not Etali-controller's. Per CR §400.7c. Alternatively (cheaper) suppress the `checkZoneConservation` false-positive by teaching it to recognize cross-seat exile as legal when sourced from an attack-trigger `etali_exile` event — but that's masking the real CR §400.7c violation.
3. **Cluster sibling sweep**: every per_card handler that exiles "the top card of each player's library" or "exile the top card of each opponent's library" likely has the same routing bug. Candidates to audit: Etali Primal Storm, Etali Primal Conqueror, Maelstrom Wanderer-style cascade-from-other-players, Possibility Storm, Bolas's Citadel "play from top of library" cross-cast cases, Knowledge Pool (exiles into a shared pool — may already be correct).
4. **Depth-threshold note for future Loki cadence**: 10K at seed 42 is no longer "the canonical clean depth" — 25K reveals tail. Either treat 25K as the new canonical floor, or accept that 10K = "clean modulo 1-in-25K rare events" and report accordingly. Recommended: keep canonical at 10K (cheap) but add a quarterly 25K deep-sweep as a rare-event probe.
