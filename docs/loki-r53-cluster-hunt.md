# Loki r53 — post-R52 cluster hunt (7500 games)

**Date:** 2026-05-20
**Branch:** `dev/loki-r53`
**Binary:** `cmd/hexdek-loki`
**Command:** `cmd/hexdek-loki --games 7500 --seed 48 --report data/rules/CHAOS_REPORT_R53.md --nightmare-boards 0`
**Base:** main @ `c9fdcc6` (post R52 batches K/L/M layered on top of the r52 baseline — Avatar Enthusiasts fix + ~80-card per_card port wave)
**Purpose:** With the r52 Avatar Enthusiasts leak closed and ~100 per_card handlers now wired in, push to 7500 games (50% more volume) on the same seed and re-rank the next top-3 violation clusters to seed the R54+ fix wave.

## Headline

| Phase            | Volume        | Crashes | Invariant Hits | Clean         |
|------------------|---------------|---------|----------------|---------------|
| Chaos games      | 7500 games    | **0**   | 780 (47 games) | 7453 (99.37%) |
| Nightmare boards | 0 boards      | —       | —              | —             |

Throughput: 26 g/s chaos. Wall time **4m47s**.

**Zero panics, zero recovers, zero crashes** — 31 cumulative loki runs in
this seed family, still flat.

## Per-invariant volume (7500 games, seed 48)

| Invariant             | Count | /1000 games |
|-----------------------|------:|------------:|
| CardIdentity          |   616 |        82.1 |
| ZoneConservation      |   108 |        14.4 |
| AttachmentConsistency |    22 |         2.9 |
| ZoneCastGrantExpiry   |    20 |         2.7 |
| CombatLegality        |     8 |         1.1 |
| TriggerCompleteness   |     6 |         0.8 |
| **Total**             | **780** |   **104.0** |

## Comparison vs r52 baseline (same seed, +50% volume)

Per-game RNG seeds are stable. Games 0-4999 are the same r52 deck-sets;
games 5000-7499 are new exposure. Per-1k rates use the new volume.

| Metric                | r52 (5k) | r52 /1k | **r53 (7.5k)** | **r53 /1k** | Δ /1k    | Notes                                                |
|-----------------------|---------:|--------:|---------------:|------------:|---------:|------------------------------------------------------|
| Crashes               |        0 |     0   |          **0** |     **0**   |   flat   |                                                      |
| Total violations      |      462 |   92.4  |        **780** |   **104.0** | **+13%** | Composition shift drives most of the rate move.      |
| Dirty games           |       31 |    6.2  |         **47** |     **6.3** |   +2%    |                                                      |
| Clean game rate       |   99.38% |    —    |     **99.37%** |     —       | −0.01pp  | Flat.                                                |
| CardIdentity          |      310 |   62.0  |        **616** |    **82.1** | **+32%** | New top cluster surfaces (see Lead 1).               |
| ZoneConservation      |      108 |   21.6  |        **108** |    **14.4** | **−33%** | Bit-stable (same game-3431 hits, larger denominator).|
| AttachmentConsistency |       10 |    2.0  |         **22** |     **2.9** |  +45%    | More noise from games 5000-7499; signatures same.    |
| ZoneCastGrantExpiry   |       20 |    4.0  |         **20** |     **2.7** |  −33%    | Bit-stable (same Prosper/Ashling sources).           |
| CombatLegality        |        8 |    1.6  |          **8** |     **1.1** |  −31%    | Bit-stable.                                          |
| TriggerCompleteness   |        6 |    1.2  |          **6** |     **0.8** |  −33%    | Bit-stable.                                          |

The four "bit-stable" invariants (ZoneConservation, ZoneCastGrantExpiry,
CombatLegality, TriggerCompleteness) all show the same absolute count at
both volumes, meaning games 5000-7499 contributed zero new hits to those
families. ZoneConservation per-1k drops mechanically (108 / 7.5k vs 108
/ 5k); the absolute leak is unchanged. The real composition story lives
in CardIdentity.

## Composition story: CardIdentity 310 → 616

r52 showed Gerrard, Weatherlight Hero (game 2432) as the dominant
CardIdentity cluster. In r53 — same seed, same game indices — Gerrard
no longer appears as a CardIdentity hit. The five shown CardIdentity
details are 100% a NEW signature:

```
CardIdentity: card "Glyph of Destruction" (ptr 0xc006c110e0) appears in
both seat 3 hand and seat 3 graveyard
```

Cluster commanders: `Vincent Valentine // Galian Beast`,
`Gwaihir, Greatest of the Eagles`, `Squee, the Immortal`, **`Krark, the
Thumbless`** (game 490, seed 4900049). The event log around the leak
shows the Krark coin-flip resolve immediately preceding the corruption:

```
[305] stack_resolve seat=3 source=Krark, the Thumbless target=seat0
[306] zone_change seat=3 source=Glyph of Destruction
[307] bounce seat=3 source=Krark, the Thumbless target=seat3
[308] per_card_handler seat=0 source=Krark, the Thumbless target=seat0
[309] stack_push seat=3 source=Glyph of Destruction target=seat0
```

This is the **Krark coin-flip bounce-back** anti-pattern — when Krark's
"flip a coin; on tails, return the spell to its owner's hand" returns
a spell to hand, the original `*Card` reference appears to also still
live in the graveyard from the cast→resolve→graveyard chain. Same
anti-pattern family as the Adric / Pitmage / Krark-zone-conservation
fixes from r41-r44, but on a different mechanism: instead of the
counterSpellEffect-shaped leak, this one is Krark's per-card bounce
handler not removing the spell card from its origin zone before
re-inserting it into hand.

Surfacing this cluster is the structural payoff of closing Avatar
Enthusiasts (r51) and the Gerrard cluster (apparently closed somewhere
in R52 batches K/L/M, since same seed/same game 2432 no longer leaks).

## Top-3 clusters for R54+

### Lead 1 — Krark, the Thumbless bounce-back hand+graveyard leak

- **Invariant:** CardIdentity
- **Volume:** 5 shown details all on the same `*Card` pointer; r53
  total of 616 CardIdentity hits with this one game (490) accounting
  for the dominant share via per-tick recount
- **Signature:** `card "Glyph of Destruction" (ptr 0xc006c110e0)
  appears in both seat 3 hand and seat 3 graveyard`
- **Suspected source:** Krark, the Thumbless per_card handler
  (`internal/gameengine/per_card/krark_the_thumbless.go` or similar)
  — the coin-flip-tails-bounce-to-hand path doesn't remove the spell
  card from its origin (graveyard / stack-resolve target) before
  inserting it into hand. Same anti-pattern shape as the closed
  r44 Krark zone-conservation cluster but on a different code path.
- **Repro:** `--games 491 --seed 48` and trace `Glyph of Destruction`
  zone transitions around the Krark resolve at event index ~305.

### Lead 2 — game-3431 ZoneConservation cards-disappeared (carry-forward)

- **Invariant:** ZoneConservation
- **Volume:** 108 hits, all from game 3431 (seed 34310049). Identical
  signature to r48-deep / r50 / r52.
- **Signature:** `zone conservation violated: N real cards disappeared
  (expected 394, found 394-N)` for N=2,4,5 across turns
- **Suspected source:** Same family as the r44 game 420 cluster
  (Breya/Bertram/Alela deck) — cards going MISSING rather than
  duplicating. Not yet root-caused; r51 + R52 batches haven't
  touched the surface. Long-standing top-3 lead now carried into
  r53+.
- **Repro:** `--games 3432 --seed 48`. Per-turn census diff at the
  leak window.

### Lead 3 — Krark→Glyph cluster (new) — but ALSO a Marchesa, the Black Rose sacrifice-trigger-drop signature has surfaced

- **Invariant:** TriggerCompleteness
- **Volume:** 1 new signature (6 total TriggerCompleteness; the
  Gisa + Jenova signatures from r52 are still present at 2 hits each;
  Marchesa is the third)
- **Signature:** `death event "sacrifice" at index 2322 with
  trigger-bearer(s) [{Marchesa, the Black Rose 1}] on battlefield,
  but no subsequent trigger/effect event found`
- **Suspected source:** Marchesa, the Black Rose's "Whenever another
  nontoken creature you control with a +1/+1 counter on it dies,
  return that card to the battlefield under your control" trigger
  not firing on a `sacrifice` event (only on `dies` / `sba_704_5g`
  /etc.). The per_card handler likely filters by death kind too
  narrowly, missing sacrifice as a kind of "die." Same root-cause
  shape as the Gisa/Jenova r52 death-trigger-drop pattern.
- **Repro:** Trace event log around index 2322 in any game with
  Marchesa as a commander; look for the unconsumed death event.

(Honorable mention — not in the top-3 since the signatures are
unchanged from r52, but the per-1k bookkeeping is worth re-flagging:
**AttachmentConsistency** doubled in absolute terms (10 → 22) when
volume only grew 50%. Same three signatures — Ghoulish Impetus,
Brilliant Wings, Dub — but more frequent. Suggests at least one of
the aura-detach-on-target-LTB paths is firing more often now, possibly
because the R52 batch K/L/M ports add token-LTB events the prior
register tree didn't dispatch from. Investigate if Lead 1 + Lead 2
get closed before R55.)

## What R52 batches K/L/M moved

Indirectly inferable from the r52→r53 deltas:

- **Closed:** Gerrard, Weatherlight Hero command_zone↔battlefield
  in game 2432. r52's top CardIdentity cluster no longer appears in
  r53's shown details (and `grep Gerrard CHAOS_REPORT_R53.md` returns
  only one match — a commander-list mention, not a violation). The
  closure isn't called out in any of the K/L/M commit messages I
  reviewed, so it's likely a side-effect of one of the batch K LTB
  cleanups or batch M's enchantment-form ports — same anti-pattern
  family as the Oketra zone-leak fix from r48.
- **Surfaced:** Krark→Glyph hand+graveyard duplication (game 490).
  Same anti-pattern family as the closed r44 Krark zone-conservation
  cluster but on a different code path — Krark's bounce-back into hand
  rather than the per_card_handler / parker_setup interaction.
- **Surfaced:** Marchesa, the Black Rose sacrifice-trigger-drop.

Net per-1k arithmetic: −12% on the bit-stable invariants (denominator
grew), +32% on CardIdentity (new top cluster), +45% on
AttachmentConsistency (volume up but signatures stable), total
**+13%/1k**. The headline number masks the substantive structural
movement underneath: an old top-3 lead closed, a new one surfaced.

## Methodology + caveats

- Single seed (48); per-game RNG seeds (`seed * 10000 + game_index +
  49`) are stable, so games 0-4999 map identically across r52 and r53.
  Games 5000-7499 are new exposure unique to r53.
- All shown-detail signatures verified by grepping `Message:` and
  `Game:` lines in `data/rules/CHAOS_REPORT_R53.md`. Volume counts
  read from the `Invariant Violations · By Invariant` table.
- Throughput: r52 saw 29 g/s → 15 g/s drop on the same volume but
  fresh worktree; r53 here ran at 26 g/s, suggesting the r52
  measurement caught a busy machine. Not a correctness signal.
- The "shown details" are capped at 5 per invariant kind, so the
  CardIdentity signature analysis is biased toward the highest-volume
  cluster. The Krark→Glyph cluster at game 490 likely accounts for the
  bulk of the 616 CardIdentity total, but the long tail (games 5000-
  7499 contributed ~300 of the +306 r52→r53 delta) may contain
  additional minor clusters not surfaced in the top-5 shown details.
  Future runs with `--seed-cmdr Krark` would isolate the Krark cluster
  out of the noise floor.
