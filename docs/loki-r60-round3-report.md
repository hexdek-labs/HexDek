# Loki R60 Round 3 Report

Date: 2026-05-24
Branch: `dev/loki-residual-round3-r60`
Comparison against: round 1 (402) and round 2 (52 → 10 → 0 on seed 41).

## TL;DR

- **Seed 41** (the seed Round 2's fixes targeted): **0 violations**
  across 5,000 chaos games + 10,000 nightmare boards. The night's
  fixes (game-420 ZoneConservation, TriggerCompleteness lifetime-cap
  + sacrifice-type filter, ZoneCastGrantExpiry same-turn false-positive,
  command_zone idempotent inserts, The Reaper / §704.6d commander
  steal race) drove seed 41 to fully clean.
- **Seed 42** (fresh seed): **6 violations across 3 games**, evenly
  split across three previously-unseen residual clusters. Loki's
  coverage is seed-dependent — driving one seed to zero does not
  guarantee zero for the next. None of the seed-41 clusters re-emerged
  on 42; all three new clusters are new bug surfaces.

## Trend across rounds

| Round | Seed | Violations | Notes |
|------:|:----:|:----------:|:------|
| 1 | 41 | **402** | Initial baseline (dominant: Cerulean Sphinx zone-leak 1,622 hits in r41 raw, ZoneConservation paradigm-copy leaks, AttachmentConsistency cluster, abdelAdrian nil-derefs). |
| 2 (pre-fix) | 41 | **52** | After R59 fixes — Cerulean Sphinx fixed, paradigm-copies fixed, abdelAdrian fixed, attachment cluster fixed. |
| 2 (post-fix) | 41 | **10** | After night's first batch — game 404 Pitmage + earlier r60 work. Cluster I picked up: 10 hits split across TriggerCompleteness 8 + ZoneCastGrantExpiry 2. |
| 2 (round 2 final) | 41 | **0** | After tonight's r60 commits: trigger lifetime-cap fix + Jaxis sacrifice-type filter + ZoneCastGrantExpiry same-turn fix + command-zone idempotency + The Reaper §704.6d race fix. |
| 3 (this round) | 42 | **6** | NEW seed surfaces three new clusters: 2 TriggerCompleteness (Rest in Peace false-positive), 2 CardIdentity (Abuelo cross-seat), 2 CombatLegality (summoning-sick attacker). |

Headline: **−98.5% from round 1 (402 → 6)**, but the residual surface
has shifted from "many duplicated hits of a few bugs" to "a few hits
each of distinct bugs."

## Round 3 residual breakdown (seed 42, 5,000 games / 10,000 boards)

| Invariant | Count | Games |
|---|---:|---|
| TriggerCompleteness | 2 | 4512 |
| CardIdentity | 2 | 1173 |
| CombatLegality | 2 | 1611 |

No single dominant — three independent clusters, each at the floor.

## Cluster 1 — TriggerCompleteness (game 4512, Gerrard + Rest in Peace)

- Game: 4512, seed 45120043, turn ≈ 5, event index 567
- Pod: Dragonlord Silumgar, Gnostro Voice of the Crags, Erayo
  Soratami Ascendant, Gerrard Weatherlight Hero
- Message: `death event "sba_704_5g" at index 567 with trigger-bearer(s)
  [{Gerrard, Weatherlight Hero 3}] on battlefield, but no subsequent
  trigger/effect event found`

Event log around the violation:
```
[565] replacement_applied seat=0 source=Rest in Peace target=seat0
[566] destroy seat=3 source=Firemane Commando
[567] sba_704_5g seat=3 source=Firemane Commando
[568] zone_change seat=3 source=Firemane Commando
[569] sba_cycle_complete seat=-1
```

**Root cause: false-positive.** Rest in Peace's §614 replacement
redirected Firemane Commando's "would be put into graveyard" to exile
(event 565). The creature was destroyed, but since CR §700.4 "dies"
requires battlefield → graveyard specifically, Gerrard's `Whenever
another creature you control dies` trigger correctly does not fire.
The TriggerCompleteness invariant only inspects the death-event side
(`sba_704_5g`) and the trigger-bearer's on-battlefield presence; it
doesn't know the destination zone was exile, not graveyard.

Same class as the previously-fixed
`sacrifice-of-non-creature` false-positive: the invariant lacks
destination-zone awareness. Fix shape: extend `checkTriggerCompleteness`
to inspect the `zone_change` event immediately following each death
event; if `to_zone != "graveyard"`, skip the bearer check.

## Cluster 2 — CardIdentity (game 1173, Abuelo cross-seat)

- Game: 1173, seed 11730043
- Pod: Lurrus of the Dream-Den (seat 0), Abuelo Ancestral Echo (seat 1),
  Shirei Shizo's Caretaker (seat 2), Gisa Glorious Resurrector (seat 3)
- Message: `card "Abuelo, Ancestral Echo" (ptr 0xc003a3aea0) appears
  in both seat 1 command_zone and seat 3 exile`

The duplicate is **cross-seat** (seat 1 command_zone + seat 3 exile),
the first such signature this round. Tail-event context shows Gisa and
Shirei resolving simultaneously through the same SBA cycle:

```
[3228] triggered_ability seat=2 source=Shirei, Shizo's Caretaker
[3229] trigger_evaluated seat=3 source=Gisa, Glorious Resurrector
[3231] triggered_ability seat=3 source=Gisa, Glorious Resurrector
[3235] stack_resolve seat=3 source=Gisa, Glorious Resurrector
[3236] per_card_handler seat=0 source=Gisa, Glorious Resurrector
[3242] stack_resolve seat=2 source=Shirei, Shizo's Caretaker
```

Hypothesis: Gisa's "If a non-token creature an opponent controls would
die, exile it instead, then create a 2/2 black Zombie under your
control" runs on Abuelo dying. The exile-instead replacement places
Abuelo's `*Card` in seat 3's exile (Gisa's controller). Then SBA
§903.9a/§704.6d should lift the commander out of exile back to its
owner's (seat 1's) command zone — but if §704.6d only sweeps
`s.Graveyard` / `s.Exile` for cards whose name matches `s.CommanderNames`,
the exile-side sweep would catch Abuelo in seat 3's exile and APPEND
to seat 1's command_zone (commander's owner), leaving seat 3's exile
copy intact. Loki r60's command-zone idempotency guard added last
night doesn't help — the duplicate is across DIFFERENT seats with
DIFFERENT slices.

Fix shape: §704.6d sweep should REMOVE the card from whichever public
zone it found it in before appending to the owner's command zone. The
existing sba.go:1738+ uses an in-place filter (`kept := s.Graveyard[:0]`
then `continue` to skip the commander) for the SAME-seat sweep, but
needs to handle the cross-seat exile case too. The Lurrus + Gisa
reanimation interaction makes this hot — commanders flow between exile
piles (Gisa's exile) and command zones (owner's) frequently.

## Cluster 3 — CombatLegality (game 1611, summoning-sick attacker)

- Game: 1611, seed 16110043
- Pod: Raph & Mikey Troublemakers (seat 0), King Macar (seat 1),
  A-Queza (seat 2), Omnath Locus of All (seat 3)
- Message: `"Behemoth of Vault 0" (seat 0) is attacking with summoning
  sickness and no haste`

Tail context shows seat 0 swinging with 7+ attackers (alpha strike)
including Behemoth of Vault 0 (8/8). Raph & Mikey's `triggered_ability`
at index 2622 (before declare_attackers) suggests an ETB or
phase-trigger created Behemoth on the same turn it then attacked.
Likely root cause: a "create a token" / "put onto battlefield" effect
that doesn't grant haste failed to set the `SummoningSick` flag
correctly, OR the hat's attack-picking heuristic ignored the sickness
constraint for a fresh permanent.

Fix shape: needs hat-side attack legality check pre-declare (cheap
SBA-equivalent), or engine-side rejection during the declare_attackers
flow. Either way, single-card fingerprint (Behemoth of Vault 0 only).
Low frequency suggests an edge case in token-creation P/T handlers.

## What's recommended next

1. **TriggerCompleteness Rest-in-Peace filter** — cheapest fix, single
   invariant edit, mirrors the sacrifice-type-filter pattern. Likely
   clears 2 hits per run AND prevents future RIP/Anafenza/Leyline
   false-positives.
2. **§704.6d cross-seat sweep** — investigate Gisa / Lurrus / Shirei
   commander flow. The exile-side sweep needs to remove from the
   opponent's exile when moving back to owner's command zone.
3. **Hat declare-attackers sickness gate** — defensive check during
   attacker picks, cheap and ELO-positive (the hat shouldn't waste
   a tap on an illegal attacker).

Round 4 candidate baseline: re-run with `--games 5000 --seed 42`
after fixing cluster 1 (the easy one) — expect 4 violations remaining.
Run additional seeds (43, 44, 45) to characterize the long-tail
residual surface before chasing further single-card bugs.
