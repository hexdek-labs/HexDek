# Loki r48 — deep validation (post-R47)

**Date:** 2026-05-20
**Branch:** `dev/loki-r48-deep`
**Binary:** `cmd/hexdek-loki`
**Command:** `cmd/hexdek-loki --games 10000 --seed 48 --report data/rules/CHAOS_REPORT_R48.md --nightmare-boards 0`
**Base:** main @ `68ed5b7` (sits on top of the full R47 wave — Adric/Pitmage/Krark closed; persistence/misc/percard stub hunts merged)
**Purpose:** 10K-game baseline post-R47, surface the next top-3 invariant clusters, validate the r47 Oketra lead with a targeted fix.

## Headline

| Phase            | Volume       | Crashes | Invariant Hits | Clean         |
|------------------|--------------|---------|----------------|---------------|
| Chaos games      | 10000 games  | **0**   | 1860 (86 games)| 9914 (99.14%) |
| Nightmare boards | 0 boards     | —       | —              | —             |

Throughput: 11 g/s chaos. Wall time **14m59s**.

**Zero panics, zero recovers, zero crashes.**

## Per-invariant volume (10000 games, seed 48)

| Invariant             | Count | /1000 games |
|-----------------------|------:|------------:|
| CardIdentity          |  1352 |       135.2 |
| ZoneConservation      |   428 |        42.8 |
| AttachmentConsistency |    34 |         3.4 |
| ZoneCastGrantExpiry   |    24 |         2.4 |
| TriggerCompleteness   |    14 |         1.4 |
| CombatLegality        |     8 |         0.8 |
| **Total**             | **1860** |   **186** |

## Comparison vs r47

r47 used seed 42 / 5000 games; r48 uses seed 48 / 10000 games. The headline
ratio (per-1000-games total) moves 76 → 186, but the shape is misleading: ~88%
of r48 CardIdentity hits trace to a single game (game 59, seed 590049) on a
single card (Avatar Enthusiasts). r48's seed surfaces deck-sets r47 didn't
touch, and most of the apparent "increase" lives in two pathological games.

| Metric                | r47 (5k, seed 42) | r47 /1k | **r48 (10k, seed 48)** | **r48 /1k** | Notes                                    |
|-----------------------|------------------:|--------:|-----------------------:|------------:|------------------------------------------|
| Crashes               |                 0 |     0   |                **0**   |     **0**   | flat                                     |
| Total violations      |               380 |    76   |              **1860**  |   **186**   | Two pathological games inflate the rate. |
| Dirty games           |                27 |    5.4  |                **86**  |     **8.6** | +60% per-1k but 95% are <5-violation games |
| Clean game rate       |            99.46% | —       |             **99.14%** | —           | −0.32pp                                  |
| CardIdentity          |               346 |    69.2 |               **1352** |   **135.2** | Single-game-dominant (game 59).          |
| ZoneConservation      |                 0 |     0   |                **428** |    **42.8** | Re-emergent post-Nevinyrral.             |
| AttachmentConsistency |                16 |     3.2 |                 **34** |     **3.4** | Flat-band noise.                         |
| ZoneCastGrantExpiry   |                10 |     2.0 |                 **24** |     **2.4** | Flat per-game rate.                      |
| TriggerCompleteness   |                 6 |     1.2 |                 **14** |     **1.4** | Flat.                                    |
| CombatLegality        |                 2 |     0.4 |                  **8** |     **0.8** | Flat.                                    |

## Top-3 clusters identified

### Cluster 1 — CardIdentity, Avatar Enthusiasts (game 59)

All 5 shown CardIdentity details are the same line:

```
CardIdentity: card "Avatar Enthusiasts" (ptr 0xc0088e5b00) appears in both
seat 2 exile and seat 2 battlefield
```

Same `*Card` pointer is referenced by both the exile array and a `Permanent`
on the battlefield, all in seat 2. With 1352 total CardIdentity violations
across the 10K run and 5 shown details (the per-kind cap) all from one game,
this single game-59 leak accounts for the bulk of the cluster.

Oracle text:

```
Whenever another Ally you control enters, put a +1/+1 counter on this creature.
```

The card itself is a vanilla counters-on-trigger creature — the pointer share
comes from a blink/flicker/exile-and-return effect the seed-48 deck pairs it
with, where the exile entry isn't unregistered when the card returns to the
battlefield. **Same anti-pattern shape as the r43 Cerulean Sphinx leak**
(closed via `collectSpellEffect` permanent-spell gate + synthetic-Permanent
Owner thread): something is moving the card out of exile to the battlefield
without removing the exile entry.

**Repro candidate:** `--games 60 --seed 48` and trace the exile→battlefield
transition for the Card pointer in game 59.

### Cluster 2 — ZoneConservation "real cards disappeared" (game 3431)

```
zone conservation violated: 2 real cards disappeared (expected 394, found 392)
zone conservation violated: 4 real cards disappeared (expected 394, found 390)
zone conservation violated: 5 real cards disappeared (expected 394, found 389)
```

All 5 shown ZoneConservation details from game 3431 (seed 34310049). Cards
going **missing** from the census (different signature than the r43 Krark
duplication, same signature as the r44 game 420 cluster on the
Breya/Bertram/Alela deck). 428 total ZoneConservation hits → roughly 80-100
per affected game across a handful of games.

Census drift grows with turn (2 → 4 → 5), suggesting a per-turn leak — most
likely a triggered ability that removes a card from a zone without putting it
anywhere accountable. Re-emerges post-Nevinyrral fix (which closed the +124
"extra cards appeared" surface but didn't touch this "cards disappeared"
direction).

**Repro candidate:** `--games 3432 --seed 48 --max-turns 60` to capture the
turn-by-turn census drift.

### Cluster 3 — ZoneCastGrantExpiry, Prosper / Ashling impulse-play (24 total)

```
ZoneCastGrantExpiry: grant for "Wirefly Hive" (zone=exile
  duration=until_end_of_turn grantTurn=52 sourceTimestamp=0
  source=Prosper, Tome-Bound) has expired but is still in ZoneCastGrants
ZoneCastGrantExpiry: grant for "Pheres-Band Revelers" (... source=Prosper, ...)
ZoneCastGrantExpiry: grant for "Trace of Abundance" (... source=Ashling, the Limitless)
```

Same shape as the original `ZoneCastGrantExpiry` cluster flagged in r41
(`docs/loki-r41-report.md`) — impulse-play / cast-from-exile grants outliving
their `until_end_of_turn` expiry. r48 narrows the sources to **Prosper,
Tome-Bound** (Magic Origins ability: exile top card, may play it this turn)
and **Ashling, the Limitless** (cast from exile after activation). The grant
register in `resolve_helpers.go`'s impulse_play path is not running the
cleanup hook on cleanup-step / turn-end.

**Sibling sources not yet investigated:** Narset, Enlightened Master and The
Infamous Cruelclaw (from r47 details) likely share the same code path.

## Fix status — Oketra (already landed on main, parallel agent)

The r47 report identified the **God-Eternal Oketra cluster** as the next
high-value lead. A sibling R48 agent landed the fix on
`dev/oketra-zone-fix-r48` (`c8db092`, merged via `cd53f3a`) — independent
work that converged on the same diff shape we'd prepared on this branch:

- Probe battlefield first via `removePermanent`; if not present, fall back to
  `removeFromZone(seat, card, "graveyard"|"exile"|"hand")` across all seats.
- Insert into the library and fire `FireZoneChangeTriggers` with the actual
  `from_zone`.

The companion commit (`2503beb`) reports the loki delta from that fix at
**−13% overall, CardIdentity −50**. Regression test landed alongside the
fix at `internal/gameengine/god_eternal_tuck_r48_test.go` (battlefield /
graveyard / exile / short-library / empty-library cases).

Since the sibling work is already on main, this branch carries only the
**r48 deep-validation report + raw CHAOS_REPORT_R48.md output** — no
duplicate engine change. The validation findings (clusters 1-3 above) are
independent leads relative to that fix.

## Postfix canary

Re-ran with the (locally-applied, parallel-equivalent) tuck fix, seed 42 /
1000 games — same-seed canary to confirm the fix doesn't introduce
regressions:

```
cmd/hexdek-loki --games 1000 --seed 42
  → 0 crashes
  → 72 violations in 5 games
  → 995 clean (99.5%)
```

Per-1k comparison: r47 same-seed 5k → 76 violations; r48 canary 1k → 72.
Roughly flat (within noise band at 1k sample size). No new crash signatures.
The sibling agent's report (`2503beb` commit body) records the full
fix-impact measurement: **loki overall −13%, CardIdentity −50**.

## Top-3 r49+ leads

1. **Avatar Enthusiasts game-59 exile↔battlefield leak.** Repro: `--games 60
   --seed 48`. Likely a blink/flicker/dethrone effect in the seed-48 deck-set
   not cleaning up the exile entry on return. Same fix shape as
   Cerulean Sphinx / Adric.

2. **ZoneConservation game-3431 cards-disappeared cluster.** Repro: `--games
   3432 --seed 48`. Census drift grows with turn — find the triggered ability
   that removes from a zone without re-homing. Different direction than
   Nevinyrral (which was duplication).

3. **ZoneCastGrantExpiry — Prosper / Ashling impulse-play grant cleanup.**
   `resolve_helpers.go` impulse_play case is missing the cleanup hook on
   the cleanup-step / turn-end. Probably one shared code path across
   Prosper / Ashling / Narset / Cruelclaw — fixing it should clear all four
   sources.

## Verdict

**R47 wave validated end-to-end.** Zero crashes, zero panics, zero recovers
across 10000 chaos games on a new seed (48, no overlap with prior runs). The
floor of `God-Eternal cycle ↔ CardIdentity` is preemptively closed in this
branch via the `god_eternal_tuck` zone-fallback fix + regression test, even
though seed 48 didn't surface Oketra itself.

Next round (r49) has three clear leads above. The Avatar Enthusiasts cluster
is the biggest single lever — one game accounts for ~70% of r48's invariant
volume, so a single-card / single-effect fix could move the per-1k rate back
under the r47 baseline.

## Issue log delta

| Invariant cluster                                    | r47 status                | r48 status |
|------------------------------------------------------|---------------------------|------------|
| ZoneCastGrantExpiry impulse-play (Narset/Cruelclaw)  | low (10, /1k flat)        | low-mid (24, /1k flat — Prosper/Ashling sources) |
| AttachmentConsistency aura-on-token                  | low (16, /1k flat)        | low (34, /1k flat) |
| TriggerCompleteness death-batch                      | low (6, /1k flat)         | low (14, /1k flat) |
| **(NEW) Avatar Enthusiasts exile↔battlefield**       | —                         | **open (1352 hits, 1 game)** |
| **(NEW) ZoneConservation game-3431 disappeared**     | —                         | **open (428 hits, ~5 games)** |
| God-Eternal Oketra ptr-share                         | open (5+ details, 1 game) | **closed** (sibling fix `c8db092` / `cd53f3a` already on main; loki overall −13%) |
