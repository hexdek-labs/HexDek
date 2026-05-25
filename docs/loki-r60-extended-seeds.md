# Loki R60 Extended-Seeds Validation

Date: 2026-05-25
Branch: `dev/loki-r60-extended-seeds-r60`
Goal: with #377's 4-residual catalog fully closed (Resource via PR
#402, SBA via PR #407, RIP closed as side effect, ZoneConservation
deferred), run 2 additional fresh seeds (3141, 2718) at 10K games
each. Declare fully clean if both return 0.

## TL;DR

**1 of 2 seeds clean. Seed 2718 surfaced a NEW residual signature
not in #377's catalog (WinCondition / poison-loss).**

| Seed | Chaos (10K games) | Nightmare (10K boards) | Verdict |
|:----:|:-----------------:|:----------------------:|:-------:|
| 3141 | **0** | **0** | ✅ clean |
| 2718 | **2 in 1 game** (WinCondition) | **0** | ⚠️ NEW residual |

Aggregate: 20,000 chaos games + 20,000 nightmare boards, 2 violations
across 1 game (1 distinct signature), 0 nightmare, 0 crashes.

**Not officially clean at the extended-seeds level.** The "wide-seed
validation" claim is bounded; the next residual surface (WinCondition
poison-loss false-positive) is the chase target.

## Seed 3141 — fully clean

10K chaos games + 10K nightmare boards, 0 violations, 0 crashes.

## Seed 2718 — 1 game / 1 new signature

### Game 3428 — WinCondition x2

- **Turn**: 39 end_of_combat
- **Active**: seat 3 (Hapatra, Vizier of Poisons)
- **Pod**: Genevieve Conniving Dragon / Bartolomé del Presidio /
  Zndrsplt Eye of Wisdom / Hapatra Vizier of Poisons
- **Final state**: seat 3 [WON], seats 0/1/2 all [LOST]
- **Signature**: `WinCondition: seat 0 lost via poison but has only
  2 poison counters (< 10)`

Per CR §704.5c, a player with 10 or more poison counters loses the
game. The invariant correctly flagged that seat 0 is marked Lost
with "via poison" reason at only 2 counters — far below the §704.5c
threshold.

### Recent events trail

```
[1209] triggered_ability seat=3 source=Hapatra, Vizier of Poisons → seat 0
[1210] stack_push seat=3 source=Hapatra
[1211] priority_pass seat=1
[1212] stack_resolve seat=3 source=Hapatra → seat 0
[1213] counter_mod seat=0 source=Hapatra amount=1   ← +1 poison
[1214] poison seat=3 source=Blightwidow amount=2 → seat 1
[1215] damage seat=3 source=Dakmor Scorpion amount=2 → seat 1
[1216] destroy seat=3 source=Broodguard Elite
[1217] sba_704_5f seat=3 source=Broodguard Elite
[1218] zone_change seat=3 source=Broodguard Elite
[1219] triggered_ability seat=3 source=Broodguard Elite → seat 0
[1220] stack_push seat=3 source=Broodguard Elite
[1221] triggers_ordered seat=3
```

The poison count on seat 0 visible in events is **at most 1** (the
single `counter_mod amount=1` at [1213]). Yet the loss reason
attributes "via poison" — which suggests EITHER:

1. **sba704_5c poison threshold misread** — the SBA might be using
   `counters_total >= 10` against the wrong counter key, OR comparing
   ints across signed/unsigned boundaries, OR triggering on
   `had_poison_event_this_turn` rather than `cumulative_poison >= 10`.
2. **LossReason mis-attributed** — the seat was Lost via some OTHER
   path (life=0 from chip damage? seat-0 has graveyard=1, exile=1,
   battlefield=0 — looks like a non-combat death) but the LossReason
   field carries a stale "via poison" string from an earlier event.

Hypothesis #2 fits the state better: seat 0 has life=16 in the
report. Life=16 doesn't trigger SBA 704.5a, so the loss must be
from another source. Yet battlefield=0, hand=4 (cards gone). Most
likely path: a Broodguard Elite "when this dies, target opponent
loses the game" trigger or similar instakill ability, with the
LossReason carrying poison-flavored text from a different code path.

### Likely root cause

The `LossReason` field is set by various SBA / loss paths and may
have a stale or mis-attributed "via poison" string when the actual
loss reason is something else (instakill, mill, etc.). Either:

- The invariant should match the loss path against the counter state
  (if the reason mentions poison, counters must be ≥10)
- The loss-reason emitter should use accurate strings (not "via
  poison" for non-poison losses)

Single signature, single game out of 20,000 — bounded, but the
WinCondition family is high-severity (incorrect game-end attribution
affects ELO / TrueSkill audit trails).

## Conclusion

**Not officially clean at the extended-seeds level.** Seed 3141
confirms the post-#407 engine is bit-stable on a fresh seed; seed
2718 surfaces 1 new long-tail signature.

Status of #377's catalog:
- ZoneConservation x2 (seed 42 at 15K) — STILL OPEN (deferred)
- ResourceConservation x2 — CLOSED (PR #402)
- SBACompleteness x6 — CLOSED (PR #407)
- ReplacementCompleteness x1 — CLOSED (side effect of #402 or #407)

New residual surfaced this run:
- WinCondition x2 (seed 2718 game 3428) — Hapatra/Broodguard pod,
  poison-loss-attribution mismatch.

Open residual count: 2 (ZoneConservation #377-deferred + WinCondition
new).

## How to reproduce

```bash
go run ./cmd/hexdek-loki --games 10000 --seed 3141   # expect: 0/0
go run ./cmd/hexdek-loki --games 10000 --seed 2718   # expect: 2 chaos in game 3428
```

Narrow seed-2718 reproducer:

```bash
go run ./cmd/hexdek-loki --games 3430 --seed 2718 --invariant win-condition
```
