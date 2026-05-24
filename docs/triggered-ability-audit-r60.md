# Triggered-Ability Cleanup Audit — R60

**Date:** 2026-05-24
**Branch:** `dev/triggered-ability-cleanup-audit-r60`
**Scope:** All transient registries in `internal/gameengine/` that depend on
either end-of-turn (`ExpireZoneCastGrants`-style) cleanup or source-LTB
(`UnregisterReplacementsForPermanent`-style) cleanup, with an eye toward
bypass paths that mirror the ZoneCastGrantExpiry residual closed in the
preceding r60 PRs.

## What was closed before this audit

| Registry | EOT sweep | Source-LTB sweep | Game-end purge |
|----------|-----------|------------------|----------------|
| `gs.ZoneCastGrants` | `ExpireZoneCastGrants` (phases.go:316) | `ExpireSourceGrants` — **hooked into 6 LTB sites in the r60 zonecast-residual PR** | flushed in `CheckEnd` (r60 zonecast-residual PR) |
| `gs.Replacements` | — | `UnregisterReplacementsForPermanent` in all 4 zone_change.go paths + both sba.go paths | n/a (lifetime ≤ source) |
| `gs.ContinuousEffects` | `ScanExpiredDurations` by `Duration` + phase/step | `UnregisterContinuousEffectsForPermanent` on LTB | n/a |

## Registries audited this round

| Registry | EOT sweep | Source-LTB sweep | Status |
|----------|-----------|------------------|--------|
| `gs.ZoneCastPolicies` | `ExpireZoneCastPoliciesByDuration` (phases.go:319) — handles `until_end_of_turn` + `until_end_of_next_turn`; **deliberately leaves `while_source_on_bf` alone** | `UnregisterZoneCastPoliciesForPermanent` — called only from per_card LTB handlers, **NOT from engine LTB pathway** | ⚠️ Confirmed leak (fixed below) |
| `gs.GraveyardFlashbackGrants` | `ExpireEOTGraveyardFlashbackGrants` + `ExpireOrphanedGraveyardFlashbackGrants` (phases.go:317–318) | `ExpireGraveyardFlashbackGrantsBySource` — called only from per_card LTB handlers (Iroh, Lier, Past in Flames family, Will of the Jeskai) | At-risk but mitigated by the orphan sweep at EOT (and by game-end purge if I extend the r60 PR pattern). No silent-leak invariant gap shown to fire in r60 fuzz. |
| `gs.PlayFromGraveyardGrants` (the bundle: per-Card grants + policy + §614 replacements + seat flag) | `ExpirePlayFromGraveyardForTurn` (phases.go:320) — turn-scoped variant only | `UnregisterPlayFromGraveyardForPermanent` — called from per_card LTB handlers (Yawgmoth's Agenda). Helper only sweeps the per-Card grants + seat flag; **the ZoneCastPolicy is NOT dropped by it, contrary to its docstring.** | ⚠️ Confirmed leak (fixed below) |
| `gs.DelayedTriggers` | self-consuming on fire; passive expiry via `gs.Turn > dt.CreatedTurn` checks in `FireDelayedTriggers` (phases.go:480+) | none — delayed triggers intentionally outlive their source per CR §603.7d | OK — by design |
| `gs.SourceLeftEffects` | `ExpireSourceLeftEffects` (phases.go:375) | n/a — fires-on-source-LTB primitive | OK |

## The confirmed leak — Yawgmoth's Agenda's ZoneCastPolicy

**Symptom:** A `RegisterPlayFromGraveyard` call with `Permanent=true`
(Yawgmoth's Agenda) registers a `ZoneCastPolicy` with `SourcePerm = perm`
and `Duration = "while_source_on_bf"` at `play_from_graveyard.go:172`.

Three cleanup paths claim to handle this policy:

1. `ExpireZoneCastPoliciesByDuration` (the EOT sweep) — its docstring
   explicitly states `while_source_on_bf` entries are **left alone**,
   handled instead by `UnregisterZoneCastPoliciesForPermanent` on LTB.
2. The engine LTB pathway in `zone_change.go` / `sba.go` — calls
   `UnregisterReplacementsForPermanent` and
   `UnregisterContinuousEffectsForPermanent`, **but NOT**
   `UnregisterZoneCastPoliciesForPermanent`.
3. The per_card LTB hook (`yawgmothsAgendaLTB`) — calls
   `UnregisterPlayFromGraveyardForPermanent`, which drops the seat
   flag + the per-Card ZoneCastGrants tied to the source's
   `Timestamp`, **but does not drop the policy**. The function's
   docstring incorrectly claims "the ZoneCastPolicy is dropped by
   UnregisterZoneCastPoliciesForPermanent" — that call is absent.

**Result:** when Yawgmoth's Agenda dies (or is exiled / bounced /
sacrificed), the policy survives. Any non-creature card entering its
controller's graveyard later in the game still matches the policy's
`Predicate` and is treated as free-castable from the graveyard by
`FindZoneCastPolicy`. There is no invariant covering this surface
(`checkZoneCastGrantExpiry` walks `gs.ZoneCastGrants` only, not
`gs.ZoneCastPolicies`), so the leak is silent.

The r60 zonecast-residual fuzz validation surfaced the per-Card grant
side (`Huatli's Final Strike` via `Yawgmoth's Agenda`, signature
`sourceTimestamp=107`) and was closed by hooking `ExpireSourceGrants`
into the engine LTB pathway. The policy side wasn't covered and
wouldn't have been flagged by the invariant — the audit caught it.

**Fix:** added `ExpireSourceBoundPolicies(p *Permanent)` in
`zone_cast_policy.go` — mirror of `ExpireSourceGrants` but for the
policy registry. Drops only policies with `SourcePerm == p` AND
`Duration == "while_source_on_bf"` (finite-duration policies stay,
because CR §603 keeps a resolved triggered ability's grant alive until
its declared expiry even if the source has since left the
battlefield). Hooked into the same 6 LTB sites as the prior r60 fix:
`DestroyPermanent`, `ExilePermanent`, `sacrificePermanentImpl`,
`BouncePermanent` in `zone_change.go`, plus `destroyPermSBA` and
`sacrificePermSBA` in `sba.go`. Also fixed the docstring on
`UnregisterPlayFromGraveyardForPermanent` to match reality and added
a `gs.UnregisterZoneCastPoliciesForPermanent(p)` call there as
defense-in-depth.

Regression test in
`internal/gameengine/yawgmoth_agenda_policy_leak_test.go`: register
Agenda, confirm a single matching policy is in the registry, destroy
Agenda, confirm the policy is gone. Fails at baseline, passes after
fix.

## At-risk surfaces noted but NOT fixed this round

- **GraveyardFlashbackGrants source-LTB**: only called from per_card
  handlers (Iroh, Lier, Past in Flames family). A future TLA card that
  grants flashback in graveyard without a per_card LTB hook would leak
  the same way. Mitigated today by `ExpireOrphanedGraveyardFlashbackGrants`
  at EOT, but that's susceptible to the same game-ends-mid-turn skip
  the r60 zonecast-residual PR closed by flushing in `CheckEnd`. Add a
  flashback-grant flush there if/when this surfaces in fuzz.
- **CheckEnd doesn't flush `gs.ZoneCastPolicies`**. The r60
  zonecast-residual PR flushes `gs.ZoneCastGrants` at game-end; the
  policy registry has the same exposure to game-ends-before-EOT-cleanup
  for `until_end_of_turn` entries. Out of scope for the
  "fix at most 1" task constraint, but worth a follow-up if the fuzz
  surfaces it.
- **The docstring on `yawgmoths_agenda.go:28`** still claims the
  engine LTB pathway invokes `UnregisterZoneCastPoliciesForPermanent`
  — it doesn't, and now doesn't need to because
  `ExpireSourceBoundPolicies` covers the `while_source_on_bf` case.
  The docstring should be updated; not done here to keep the
  per_card surface untouched.

## Anti-patterns to watch for

A registration is **leak-prone** if all three are true:

1. The state is keyed off `SourceTimestamp` or `SourcePerm`.
2. Its `Duration` is `while_source_on_bf` (or an analogous
   "lifetime-bound-to-source" semantic with no time clock).
3. Cleanup relies on a per_card LTB handler calling the
   `Unregister*ForPermanent` helper, rather than being wired into
   the engine LTB pathway.

The r60 ZoneCastGrant fix moved the per-Card grant out of that
anti-pattern by hooking `ExpireSourceGrants` into the engine LTB
pathway. This audit moved the ZoneCastPolicy out of the same
anti-pattern via `ExpireSourceBoundPolicies`. Future registries
should default to engine-side cleanup unless the per_card hook
adds value the engine can't (e.g., conditional unregister).
