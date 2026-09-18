# CR Aug-2026 impact — build scope (branch `dev/cr-aug-impact`)

Companion to the Aug 7 2026 Comprehensive Rules adoption. **A-items are built +
tested + staged on this branch** (commit `cc388ce1`). **B-items are scoped
below** for the modular, rule-focused build, staged with A for one push.

## A — DRIFTED rules (BUILT, staged)

| # | Rule | Change | Fix | Status |
|---|------|--------|-----|--------|
| A1 | §605.1a | mana ability disqualified if cost/effect moves a card to/from a library | `IsManaAbility` library-movement gate (mill in Cost.Extra, or Draw/Mill/Surveil/Tutor in effect; Scry excluded) | ✅ built + test |
| A2 | §714.3a | Saga's first lore counter = intrinsic enters-with-counter replacement (§614.1c) | route through `PutCountersTriggered` (event + doubler chain) | ✅ built + test — **OPEN FORK: DS doubles starting lore (chapter 2). Needs ruling.** |
| A3 | §702.122c | "crewed by" is a defined relationship | `Permanent.CrewersThisTurn` + `CrewedByThisTurn` (mirrors Saddle) | ✅ built + test |

## B — NEW mechanics (scoped; modular reuse)

Ordered by reuse-confidence / effort.

### LOW effort — reuse an existing primitive

| Keyword | Rule | Meaning | Engine seam / reuse | Notes |
|---------|------|---------|---------------------|-------|
| **Storied** | §702.195 | 3+ artifact/Saga/legendary permanents → "enduring story" designation (perm.) | **reuse Ascend**: `keywords_misc.go` sets `citys_blessing` at 10+ perms + `HasCitysBlessing()`. Generalize to a filtered-count threshold → `enduring_story` flag + `HasEnduringStory()` | per-card `grub_storied_matriarch.go` already exists to validate against |
| **Heal** | §701.69 | remove marked damage from a permanent | new `RemoveMarkedDamage(perm)` — we already track marked damage | trivial primitive |
| **Recruit** | §701.70 | loot; if you discarded a nonland → 1/1 W Human Soldier | compose existing loot (draw+discard) + `CreateCreatureToken` | conditional on discarded-card type |
| **worthy creatures** | §700.16 | legendary && not Villain && (red and/or white) | predicate helper `IsWorthy(card)` | a matcher, no state |
| **Teamwork** | §702.194 | additional cost to cast: tap creatures total power ≥ N | **reuse crew/saddle** tap-for-total-power primitive as a §601.2f spell additional cost | same shape as Saddle/Crew, different attach point (cast, not activate) |

### MED effort — modest new plumbing (design choice flagged)

| Keyword | Rule | Meaning | Engine seam | Design note |
|---------|------|---------|-------------|-------------|
| **Increment** | §702.191 | on cast, if this is a creature and mana spent > its power OR toughness → +1/+1 counter | per-permanent triggered ability on `cast`; place via `PutCountersTriggered` | needs **"mana spent to cast"** on the cast event — check whether the cast pipeline already records it; if not, that's the one new datum |
| **Power-up** | §702.193 | rider on an activated ability: if permanent entered this turn, cost −(its mana cost); activate only once | cost-modifier + once-per-turn/permanent marker; reuse cost-reduction machinery | "entered this turn" flag + a per-ability once marker |
| **Paradigm** | §702.192 | first resolution: each precombat main, copy in exile, cast free; else exile this spell | **reuse existing ParadigmExile machinery** (Loki r43: `stack.go`/`phases.go`) — wire it as a keyword rather than per-card | mostly a generalization of code we already run |

### NEEDS INVESTIGATION first

| Area | Rules | Note |
|------|-------|------|
| **Battles restructure** | §310.9x / §310.12x added, §310.8/§310.11 removed | Assess current battles/Siege support before scoping — this is a rework of an area we may only partially implement. Separate scoping pass. |

## Build order proposed
1. LOW block (Storied, Heal, Recruit, worthy, Teamwork) — clean reuse, high confidence.
2. MED block (Increment, Power-up, Paradigm) — after confirming the design notes above.
3. Battles — investigate current support, then scope separately.

All staged on `dev/cr-aug-impact`; single push once the batch is green and the
A2 DS-lore fork + the MED design notes are confirmed.
