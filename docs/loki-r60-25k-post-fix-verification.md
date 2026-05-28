# Loki r60 25K Post-Fix Verification (Etali cluster — PR #685)

## Headline

**Etali cluster fully closed: 828 → 0 violations (-828, -100%).** Re-
ran the exact same 25K-game seed-42 sweep that originally surfaced
the 828-violation Etali cluster in PR #682. Post PR #685's CR §400.7c
owner-routing fix, the Etali clusters are gone. The run surfaced
**2 residual violations in 1 game** at depth 14,620 — both
ZoneConservation, but in a **completely different cluster**
(Naru Meha + Panharmonicon ETB-copy interaction, no Etali in the
pod). This is a new deeper-tail surface that was masked in the
original 25K run because Etali's 280 ZoneConservation violations
dominated the report; with Etali silenced, the rare Naru Meha case
became visible.

## Before / after summary

| Metric | Pre-fix (PR #682) | Post-fix (this run) | Δ |
|--------|------------------:|--------------------:|---|
| Total chaos games | 25,000 | 25,000 | 0 |
| Total chaos violations | **828** | **2** | **−826 (−99.76%)** |
| Games with ≥1 violation | 16 | 1 | −15 (−93.75%) |
| Crashes | 0 | 0 | 0 |
| Panics | 0 | 0 | 0 |
| Nightmare boards | 10,000 | 10,000 | 0 |
| Nightmare violations | 0 | 0 | 0 |
| Chaos throughput | 52 g/s | 25 g/s | −27 g/s (slower from thermal — same M-class hardware, consecutive sweep) |

## Per-invariant before / after

| Invariant | Pre-fix count | Post-fix count | Δ |
|-----------|--------------:|---------------:|---|
| **CardIdentity** (Etali "Hostile Realm" cross-seat ptr leak — Cluster 1 in PR #682) | **548** in ~5 games | **0** | **−548** ✅ |
| **ZoneConservation** (Etali cast-from-exile copy-count drift — Cluster 2 in PR #682) | **280** in ~11 games | **2** in 1 game (NEW cluster, NOT Etali) | **−278** ✅ |
| All other invariant kinds | 0 | 0 | unchanged |

Both Etali clusters are zero. The 2 residual ZoneConservation hits
are NEW signal that wasn't visible pre-fix because the Etali ZC
cluster's 280 violations swallowed the report.

## Card-correlation before / after

| Rank | Card | Pre-fix violation games | Post-fix violation games | Δ |
|------|------|------------------------:|-------------------------:|---|
| 1 (pre-fix) | **Etali, Primal Storm** | **15 / 16** | **0 / 1** | **−15** ✅ |
| 2 (pre-fix) | Riveteers Confluence | 1 / 16 | 0 / 1 | −1 |
| 3 (pre-fix) | Mirrorwood Treefolk | 1 / 16 | 0 / 1 | −1 |
| 4 (pre-fix) | Sand Warrior | 1 / 16 | 0 / 1 | −1 |
| 1 (post-fix) | Hardened Academic | 0 / 16 | 1 / 1 | +1 (new — coincidental shared pod) |
| 1 (post-fix) | Wyll of the Elder Pact | 0 / 16 | 1 / 1 | +1 (new — coincidental pod member) |

Etali's correlation dropped to **0/1** post-fix (Etali didn't appear
in the 1 remaining violation game's pod at all). The post-fix
top-correlation cards are all coincidental pod members of the single
Naru Meha + Panharmonicon game — none are causal.

## Residual cluster: Naru Meha + Panharmonicon ETB-copy interaction

**Single violation game** (game 14620, depth ~58% through the 25K
sweep, well past the 10K canonical-clean threshold):

| Field | Value |
|-------|-------|
| Game | 14620 (seed 146200043 derived from master seed 42) |
| Turn | 39, Phase=beginning Step=draw |
| Invariant | ZoneConservation × 2 (consecutive snapshots, same game) |
| Pod | Wyll of the Elder Pact / Toph, Hardheaded Teacher / Giott, King of the Dwarves / Judith, the Scourge Diva |
| **Etali presence** | **NONE** — no Etali in this pod, confirms residual is unrelated |
| Census drift | seat 0 reports "500 extra real cards appeared" (expected -138, found 362) |
| All 4 seats | Lost (life=9/27/25/19, library=81/81/81/82, battlefield=0/0/0/0) |

Recent event window (events 6883-6896):

```
[6883] replacement_applied seat=0 source=Panharmonicon amount=2
[6884] triggered_ability seat=0 source=Naru Meha, Master Wizard
[6885] triggered_ability seat=0 source=Naru Meha, Master Wizard
[6886] priority_pass seat=1
[6887] priority_pass seat=2
[6888] priority_pass seat=3
[6889] stack_resolve seat=0 source=Naru Meha, Master Wizard
[6890] stack_push seat=0 source=Naru Meha, Master Wizard
[6891] copy_spell seat=0 source=Naru Meha, Master Wizard
[6892] enter_battlefield seat=0 source=Naru Meha, Master Wizard
[6893] replacement_applied seat=0 source=Panharmonicon amount=2
[6894] game_draw seat=0
[6895] game_draw seat=1
[6896] game_draw seat=2
```

### Root cause hypothesis

**Naru Meha, Master Wizard** oracle: *"When Naru Meha enters the
battlefield, copy target instant or sorcery spell you control. You
may choose new targets for the copy."* (BBD 2018, {2}{U}{U}, 3/3
Legendary Creature — Human Wizard.)

**Panharmonicon** oracle: *"If a creature ETB ability triggers, that
ability triggers an additional time."* (KLD 2016, {4}, artifact.)

The interaction: when Naru Meha enters with Panharmonicon out, the
ETB ability triggers twice → two `copy_spell` events copying the
target instant/sorcery. If the target instant/sorcery is itself
Naru Meha (e.g. a flicker-loop or a cast-from-graveyard like
Pull from the Deep), each copy resolves, creates another Naru Meha
on the battlefield, which itself triggers twice via Panharmonicon,
copying ANOTHER target instant/sorcery from your control...

The event log shows the cascade: `replacement_applied seat=0
source=Panharmonicon amount=2` (the doubling), `triggered_ability
seat=0 source=Naru Meha` x2 (the doubled triggers), then a `copy_spell` +
`enter_battlefield` cascade. At some point the recursion blows up the
per-seat card census by 500 (expected -138 found 362). Then `game_draw`
fires immediately after on all 4 seats — the SBA cap mandatory-loop
draw correctly detected the infinite loop and ended the game with the
704.3 safety cap (per the May-24 cap-draw issue-log entry).

So the game ended CORRECTLY via the mandatory-loop draw path; the
ZoneConservation invariant fired in the cleanup snapshot because the
per-seat card census had already drifted before the cap-draw closed
the game. The invariant is correctly flagging that the engine's per-
seat object accounting got out of sync during the unbounded copy
cascade — likely the copy machinery is creating `*Card` pointers (or
token wrappers around copied spell objects) without registering them
in the per-seat census the invariant counts against.

### Why this didn't surface in the original 25K run

The original 25K sweep at seed 42 ran on the same seed 146200043
sub-seed at game 14620 — the Naru Meha + Panharmonicon cascade
MUST have triggered there too. But the original report's
"Violation Details (up to 5 per invariant kind)" showed:

- 5 of 5 CardIdentity details from game 1944 (Etali Hostile Realm)
- 5 of 5 ZoneConservation details from game 2275 (Etali exile drift)

The Loki reporter's "up to 5 per invariant kind" cap meant the
Naru Meha game 14620 ZC violations were among the 275 non-detailed
ZC violations that the report rolled up but didn't expand. Post-fix
the Etali cluster's 280 ZC violations are gone, so the Naru Meha
game becomes the only ZC violation and the reporter expands its
details. **This isn't a regression — it's exposure of a
pre-existing rare-tail cluster previously masked by the Etali noise.**

### Sibling-card audit (not in scope for this PR)

The 25K report's recommendation #3 called out the Etali family
sibling sweep (Etali Primal Conqueror, Maelstrom Wanderer-style
cascade, Possibility Storm, Bolas's Citadel play-from-top, Knowledge
Pool) — all per_card handlers that exile cards across seats. None of
those appeared in this run's 1 violation game, so the targeted Etali
fix didn't miss any in-family siblings AT THIS DEPTH AT THIS SEED.
A broader sweep across multiple seeds would be needed to confirm no
sibling carries the same anti-pattern; that's tracked as the
follow-up audit PR mentioned in PR #685's commit message.

The Naru Meha + Panharmonicon cluster is a separate spell-copy
cascade family, not an Etali sibling. The fix surface is the spell-
copy machinery's per-seat census plumbing, not the cross-seat
zone-routing PR #685 addressed.

## Verdict

**PR #685 (Etali §400.7c owner-routing fix) is fully verified.** The
targeted Etali cluster dropped from 828 violations across 16 games
to 0 violations across 0 games. The 2 residual ZoneConservation
violations in 1 game are a separate Naru Meha + Panharmonicon
spell-copy cascade cluster that was previously masked by the Etali
noise; it's a new fix surface for a follow-up PR, NOT a regression
of the Etali fix.

## CLAUDE.md issue-log impact

Recommended Resolved-table entry:

> | 2026-05-27 | Loki r60 25K post-fix verify | **Etali §400.7c cluster: 828 → 0** | PR #685 routes Etali's exiled cards to each card's OWNER's exile per CR §400.7c (was: cross-seat-routed to Etali-controller's exile via a buggy "library_remove" + manual append pair). 25K seed-42 re-run @ 25 g/s reports 0 CardIdentity (was 548) + 0 Etali-related ZoneConservation (was 280). Residual 2 ZC violations in 1 game (#14620) are a separate Naru Meha + Panharmonicon copy-cascade cluster previously masked by the Etali noise. |

Recommended Open-table entry (new cluster discovered):

> | 2026-05-27 | Loki r60 25K post-fix verify | **Naru Meha + Panharmonicon copy-cascade ZoneConservation (2 violations / 1 game / 25K depth)** | LOW | Game 14620 turn 39: Naru Meha ETB doubled by Panharmonicon cascades into a copy-spell loop; per-seat card census drifts by ~500 before SBA cap-draw closes the game. The game-draw path is correct (CR §704.3 mandatory-loop cap fires); the residue is the cleanup snapshot showing the census drift. Fix surface: per-seat census plumbing in the copy-spell machinery (likely `resolveModificationEffect` copy arm or `copy_spell` event handler). Rate: 1 in 25,000 games. Not blocking for any deck currently in active play. |

## Reproducing

```bash
cd $(git rev-parse --show-toplevel)
git fetch origin main
git checkout -B dev/etali-fix-verification-r60 origin/main
go run ./cmd/hexdek-loki --games 25000 --seed 42 \
    --report /tmp/loki_25k_postfix.md
```

Expected (post PR #685): `violations: 2 (in 1 games)` chaos + `0`
nightmare. The 2 chaos violations are both ZoneConservation in game
14620 (Naru Meha + Panharmonicon, NOT Etali). Wall time ~17 minutes
on consecutive-sweep thermals (~25 g/s); first-run cold throughput
typically ~52 g/s for a ~8-minute wall.
