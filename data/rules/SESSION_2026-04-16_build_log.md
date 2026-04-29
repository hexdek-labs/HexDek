# mtgsquad Session Log — 2026-04-16

**Who:** Josh Wiedeman (direction), 7174n1c (rules expertise / QA), Hex (engineering).
**What:** From "parser at 97.74% GREEN" to "judge-grade 4-player EDH engine running personal decks at 0 crashes / 100 games."
**Why captured:** We kicked a fat rock and only found a big worm — the engine came out cleaner than predicted. This log is "the picture" so the wins don't evaporate.

---

## Headline achievements (in rough order)

1. **Parser landed at 100% GREEN** — 31,943 cards typed to AST, zero errors. Up from 97.74% at session start.
2. **100% GREEN → 31,965 cards** after planeswalker-filter bug fix (`'plane' in 'planeswalker'` substring match was excluding every planeswalker silently for weeks).
3. **§704 state-based actions** — 15 of 24 fully implemented, 9 stubbed with CR citations. Up from 3.
4. **§614 replacement effects framework** — 8 event types, 15 handlers, **APNAP ordering working** (Hardened Scales + Doubling Season correctly produces 4 counters when HS is older, 3 if DS is older).
5. **§613 layer system enforcement** — query path built, ContinuousEffect register/unregister wired, caching + timestamp ordering. Per-card migration in progress.
6. **Stack + priority + counterspells** — Control archetype went from 1% aggregate winrate to 29% after stack landed.
7. **Commander format** — §903 complete: command zone + tax + commander damage + Gilded Drake owner/controller distinction + 40 life + zone-change replacement (§903.9b) + SBA return (§903.9a).
8. **Multiplayer N-seat engine** — 2p → N-seat generalization. APNAP priority, threat-score targeting, §800.4a seat elimination, each-opponent effect fan-outs.
9. **Per-card handler dispatch bridge** — 20 snowflake cards runtime-handled. Combos harness went from 1/5 → 4/5 passing.
10. **Duration tracking** — 9 duration kinds, scan_expired_durations at every phase/step boundary.
11. **Delayed trigger queue** — 7 trigger_at kinds, Sneak Attack / Pact / end-step effects.
12. **Combat damage per-instance triggers** — double-strike fires triggers twice correctly.
13. **Enters-tapped-and-attacking distinction** — Hero of Bladehold tokens don't re-trigger attacks.
14. **Additional combat phases** — Seize the Day / Aggravated Assault multi-combat support.
15. **Stun counter untap replacement** — §614-integrated. Kaldheim+ era cards supported.
16. **Sundial of the Infinite** — end_the_turn() correctly skips end-step triggers while still expiring "until end of turn" effects.
17. **Lasagna event schema** — game_id + turn + phase_kind + step_kind + priority_round + timestamp + layer + target + depends_on. Additive upgrade.
18. **Rule auditor** — 9 invariants, 0 violations across 260,819 events (pre-gauntlet).
19. **Interaction harnesses** — 4 harnesses × 400-2000 reps each. Win-combos 4/5, infinites 5/5, paradoxes 5/5, timing 2/5.
20. **AST dataset exported** — 31,639 rows × typed AST as JSONL, HF-ready.
21. **Combo detector** — static analysis over AST, found Dockside+Sabertooth, Painter+Grindstone, doublers, storm engines.
22. **Replay viewer** — static HTML/JS UI for scrubbing turn-by-turn.
23. **Golden-file test suite** — 203 pytest tests, now 223 after §613 work.
24. **Documentation** — README, ARCHITECTURE.md, PROJECT_AUDIT.md, edge_case_wishlist.md, this file.
25. **Personal deck gauntlet** — 100 games, 4-player EDH, 0 crashes, 16.4 avg turns, diverse archetype winrates.

## Gauntlet results (THE milestone)

**Setup:** Sin / Oloro / Ragost / Coram × 100 games × seed=42 × 4-player FFA × standard EDH.

| Commander | Archetype | Wins | Win % | First-out % |
|---|---|---:|---:|---:|
| Ragost, Deft Gastronaut | Boros artifacts | 34 | 34% | 12% |
| Sin, Spira's Punishment | Sultai landfall | 33 | 33% | 39% 🎲 |
| Oloro, Ageless Ascetic | Esper lifegain | 26 | 26% | 6% 🛡️ |
| Coram, the Undertaker | Jund graveyard | 7 | 7% | 43% 💀 |

**Engine health:**
- 100 games completed, 0 crashed, 0 turn-cap hits
- Avg 16.4 turns (median 16, min 11, max 24)
- 123 audit violations across entire run (~1.23/game)
- 7 distinct parser gap snippets identified

**Archetype dynamics observed (emergent, not coded):**
- Coram-as-archenemy: graveyard decks get targeted. 43% first-out rate.
- Sin-as-glass-cannon: 39% first-out AND 33% winrate. Bimodal outcome.
- Oloro-as-tank: only 6% first-out. Lifegain + §614 doublers (Boon Reflection + Rhox Faithmender) make it survive to close.
- Ragost-as-aggro-closer: treasure engine + damage-pingers. Most reliable.

---

## 7174n1c's rules nightmares (contributed during session, memory-worthy)

These are the edge cases 7174n1c raised that got filed into test cases or design. Each one represents judge-level rules competence that took him thousands of games to build.

1. **Rule 716 infinite loop classification** — optional vs mandatory, one-sided vs two-sided, CR §731.4 draw vs CR §104.3a controller-loses.
2. **Sundial of the Infinite** — damage wears off via any turn-end path (not specifically cleanup), but end-step triggers skipped.
3. **Duration kinds (9)** — end_of_turn, until_your_next_turn, until_end_of_your_next_turn, until_next_end_step, until_your_next_end_step, until_next_upkeep, until_source_leaves, until_condition_changes, permanent.
4. **Sacrifice-at-end-of-turn delayed triggers** — Sneak Attack, Through the Breach pattern.
5. **Stun counters** — §614 replacement on untap event. Neon Dynasty onwards.
6. **Combat damage per-instance triggers** — double-strike + "whenever deals combat damage" fires twice (Goldspan + Fireshrieker = 2 Treasures).
7. **"Enters tapped and attacking" vs "declared attacking"** — Hero of Bladehold tokens ARE attacking but didn't go through declare-attackers, so their own "whenever attacks" triggers don't fire.
8. **Additional combat phases** — Seize the Day, Aggravated Assault; each combat phase re-fires begin-of-combat triggers.
9. **Combat priority windows** — 6 windows: begin-of-combat, declare-attackers, declare-blockers, first-strike-damage, combat-damage, end-of-combat.
10. **Commander zone rules** — owner's replacement right, not controller's. Ownership change on battlefield doesn't trigger zone replacement. Commander tax only on command-zone casts.
11. **Multiplayer poker AI policy** — HOLD / CALL / RAISE with event-driven adaptive updates + hysteresis + explainable transitions.
12. **Grappling Kraken + Scapeshift + Stun Counters + Seize the Day scenario** — full scenario trace showing what's wired vs what's gap.
13. **XMage bug-tracker scrape suggestion** — use their issue tracker as our test case source (they did the hard work for us).

## Josh's direction moments (priorities + decisions)

1. **"No half work"** — pursue full coverage, don't stop at convenient milestones
2. **"Accurate accuracy over all else"** — never compromise rules correctness for speed/convenience
3. **"Go backed for 50k rounds/sec, judge-perfect accuracy"** — the Go engine roadmap with parity testing
4. **"This is OSS work, have fun"** — agent prompting style lesson
5. **"Emotional arcs vs technical barking"** — how to direct subagents for quality
6. **"Drop jar jar, practical TLDR"** — changelog voice correction mid-session
7. **"Fire fire fire"** + immediately after "don't immediately run the gauntlet, give me a minute to breathe" — trigger and pause dynamic
8. **"7174n1c owner authority"** — delegation of command-level direction
9. **"Push beyond XMage / find rules bugs in printed cards"** — engine ambition ceiling
10. **"Dante's Inferno plan"** — 1B-game phase transition detection + 5k Moxfield deck comparison as post-Go-engine research plan

## Agent summary (in rough order)

Round 1 (parallel): combo detector, AST dataset export, replay viewer, MVP play loop, golden-file test suite.
Round 2: polish + review, scrubber consolidation, README/ARCHITECTURE docs, stack + counterspells, combat phase, tournament + rule auditor.
Round 3: conjunction fix, UnknownEffect pattern hunt, tutor fix, per-card handler bridge.
Round 4: §704 SBAs, §614 replacement effects framework.
Round 5: §613 layers + combat/timing omnibus (9 scope items), commander format + command zone.
Round 6: multiplayer N-seat extension, deck parser + gauntlet runner.
Round 7 (in flight): zone_conservation audit polish, parser gap extensions wave 2, §613 per-card migration, poker AI policy.

Each agent verified 0 audit violations + pytest passing + GREEN preserved before declaring done.

## Files that matter (snapshot)

**Engineering:**
- `scripts/parser.py` (~2000 LOC, 100% GREEN)
- `scripts/mtg_ast.py` (schema with ScalingAmount, ContinuousEffect, DelayedTrigger, Event, ReplacementEffect)
- `scripts/playloop.py` (~6600 LOC — stack, priority, combat, §614, §613, multiplayer, commander, duration, delayed triggers)
- `scripts/extensions/` (~50+ files, parser extension rules)
- `scripts/extensions/per_card.py` (1,079 snowflake card handlers)
- `scripts/extensions/per_card_runtime.py` (911 LOC, 20 runtime handlers + name-based fallbacks)
- `scripts/gauntlet.py` (~800 LOC, 4p EDH gauntlet runner with 2p fallback)
- `scripts/rule_auditor.py` (9 invariants)

**Docs + data:**
- `README.md`, `ARCHITECTURE.md`, `PROJECT_AUDIT.md`
- `data/rules/edge_case_wishlist.md` (7174n1c's canonical test cases)
- `data/rules/gauntlet_report.md` (100-game results)
- `data/rules/gauntlet_audit_events.jsonl` (full event stream)
- `data/rules/SESSION_2026-04-16_build_log.md` (this file)
- `data/decks/personal/*.txt` (sin, oloro, ragost, coram decklists)

**Tests:**
- `tests/` (223 passing pytest)
- `scripts/test_commander_zone.py` (26/26)
- `scripts/test_multiplayer.py` (42/42)
- `scripts/test_engine_layers_and_durations.py` (22/22)
- 4 interaction harnesses (combos, infinites, paradoxes, timing)

## Memory files touched this session

- `project_hexdek_parser.md` — parser state
- `project_hexdek_architecture.md` — architecture decisions
- `project_hexdek_inferno_plan.md` — post-Go-engine research plan
- `feedback_7174n1c_authority.md` — owner-level delegation
- `feedback_agent_prompting.md` — emotional arcs vs technical
- `feedback_changelog_voice.md` — voice conventions (jar jar revoked)
- `user_mtg_7174n1c.md`, `user_mtg_wiedeman.md` — deck list memory

---

## What's next (per 7174n1c's directive at 09:03 UTC)

**In flight (round 7):**
1. Zone conservation audit polish (tighten the 84 noise violations)
2. Parser gap extensions wave 2 (type the 7 remaining unknown snippets)
3. §613 per-card continuous effect migration (Humility / Opalescence / Blood Moon / Painter / etc. registering through the layer framework)
4. Poker AI policy (HOLD / CALL / RAISE with event-driven adaptive behavior)

**Deferred (for tomorrow or later):**
- 1000-game runs (statistical power)
- Add Lobster deck + 5-commander pod
- Go engine port (50k games/sec target)
- XMage bug-tracker scrape

---

**"We kicked a fat rock and only found a big worm."** The engine is cleaner than predicted. This log is the picture.

Big night. One-sitting work. Gold logged.
