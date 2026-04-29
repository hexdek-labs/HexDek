# MTG Rules Edge-Case Wishlist

**Purpose:** Running list of canonical rules-nightmare test cases to encode into the interaction harness. Each case is either a known judge-call ambiguity, a documented XMage/Forge bug, or a complex interaction that breaks naive engines. Getting these right is the moat.

**Maintainers:** 7174n1c (primary, veteran MTG judge-adjacent), Hex (runtime), Josh (direction).

---

## Sundial of the Infinite — Turn-ending primitive

- Damage wears off even when turn ends via Sundial (damage is tied to turn-end, not cleanup-step)
- "Until end of turn" effects expire via any turn-end path
- End-step triggers DO NOT fire (skipped phase)
- Spells on stack are exiled, not graveyarded
- Yawgmoth's Will + Sundial = exile YW mid-stack, rest-of-turn cast effects continue next turn
- Necropotence + Sundial = skip discard step
- Sundial during combat = ends combat mid-step, attackers state?
- Sundial during opponent's turn (shouldn't work — "your turn only" restriction)

## Duration kinds (from §514, §601, §613)

- `end_of_turn` — clears via any turn-end path (including Sundial)
- `until_your_next_turn` — lasts through opp's turn
- `until_end_of_your_next_turn` — even longer
- `until_next_end_step` — clears at beginning of end step, pre-cleanup
- `until_your_next_end_step` — specifically yours, not opp's
- `until_next_upkeep` — Ashiok-style
- `until_source_leaves` — aura-style contingent
- `until_conditioning_changes` — "as long as" clauses
- `permanent` — continuous effect stays until source gone or replaced
- `one_shot` — not a duration, immediate resolution

## Layer system edge cases (§613)

- Humility + Opalescence — classic layer-4/layer-6 paradox, post-2017 oracle self-exclusion
- Blood Moon + Dryad Arbor — type-line preservation (creature-land stays a creature)
- Magus of the Moon + Blood Moon both on battlefield — layer-4 stacking
- Mycosynth Lattice + Blood Moon — "all permanents are artifacts" + "all lands are mountains" interaction
- Cytoshape / Clone replication with additional abilities
- Conspiracy (the card — Legacy legal only) — type-setting with dependency on other type-changers per §613.8
- Painter's Servant color-wash — layer 5 effect, applies to ALL cards in all zones
- Opalescence on itself (if it said "each non-Aura enchantment") — pre-2017 oracle paradox
- Lignify (lose all abilities, become 0/4 Treefolk) — layer 4 type change + layer 7b P/T set + layer 6 ability strip
- Perplexing Chimera (change controller of spells) — §613 layer 2 on stack objects

## Untap-step replacement effects

- **Stun counters** — "if permanent with stun counter would untap, remove a stun counter instead" (Neon Dynasty onwards). Counter tracked but not enforced currently. Post-2022 cards rely on this heavily.
- **"Doesn't untap during your untap step"** — Frost Titan-style, Icy Manipulator aftermath
- **"Skip your untap step"** — Stasis, Winter Orb (if Howling Mine-tied), Null Profusion
- **"Untap up to N permanents"** — Deep Analysis-style limited untap
- **Vigilance-adjacent: "doesn't untap during opp's untap step"** — obscure corner case

## Delayed triggered abilities (§603.7)

- "sacrifice ~ at end of turn" — Sneak Attack, Through the Breach
- "exile ~ at end of turn" — Mirage Mirror
- "sacrifice ~ at end of combat" — older goblin / temporary-steal effects
- "return ~ to hand at end of turn" — Release to the Wind
- "at beginning of next upkeep, X" — Pact cycle (Pact of Negation, Pact of the Titan), Slaughter Pact
- "at beginning of next end step, X" — various exile-return effects
- **Engine needs: `game.delayed_triggers` queue, phase-boundary firing, registry per trigger source**
- Currently handled ad-hoc per card (Worldgorger LTB, Animate Dead LTB). General framework missing.
- **§614 replacement interaction**: Rest in Peace + Sneak-Attacked creature → creature goes to exile not graveyard at end-of-turn sacrifice. Delayed trigger must respect replacement effects at fire time.

## Replacement effects (§614)

- Rest in Peace — "exile instead of graveyard" blanket
- Leyline of the Void — same, opponent-only
- Anafenza the Foremost — "exile instead of die" for opp's creatures (token question)
- Doubling Season — counter doubler + token doubler, interacts with Hardened Scales ordering
- Hardened Scales + Doubling Season — ORDER MATTERS: +1 → ×2 = 4 (correct) vs ×2 → +1 = 3 (wrong). APNAP chooses.
- Panharmonicon — ETB doubler (§603 technically but functionally §614)
- Strionic Resonator — trigger copy
- Alhammarret's Archive — draw + life doubler
- Boon Reflection / Rhox Faithmender — life gain doubler
- Sanguine Bond + Exquisite Blood — not technically replacement but cascade chain
- Worldgorger Dragon + Animate Dead — replacement-triggered loop
- Laboratory Maniac / Thassa's Oracle — draw-from-empty replacement

## Combat priority windows (§506)

- Beginning of combat — priority opens
- Declare attackers — priority after attackers declared, before damage
- Declare blockers — priority after blockers declared and damage ordered
- First strike damage step — priority after damage dealt
- Combat damage step — priority after damage dealt
- End of combat — priority opens
- **Each step has its own priority window** — essential for combat instants, removal mid-combat, pump spells

## Combat trigger + phase edge cases (§506-§511)

### "At beginning of combat" (§507) vs "whenever attacks" (§508)
- "at beginning of combat" triggers fire at beginning-of-combat STEP, before declare-attackers
- "whenever ~ attacks" triggers fire in declare-attackers step when creature is chosen as attacker
- **these are different timing windows** with different response opportunities

### "Declared attacking" vs "enters tapped and attacking"
- **Declared attacking** = went through declare-attackers step, was chosen as attacker. "whenever ~ attacks" triggers fire.
- **Enters tapped and attacking** = token/permanent appears on battlefield already tapped and marked as attacking, BYPASSING declare-attackers step.
- **Rules consequence**: "whenever ~ attacks" triggers DO NOT fire for enters-tapped-and-attacking creatures. They ARE "attacking creatures" (§506.2) but weren't "declared as attackers."
- **Canonical test cards**: Hero of Bladehold, Brimaz (King of Oreskos), Goblin Rabblemaster, Deepfire Elemental.
- **Known XMage/Forge bug area** — engines that conflate "is attacking" with "was declared as attacker" produce infinite-cascade false positives.

### Additional combat phases (Seize the Day, Aggravated Assault, Relentless Assault, Savage Beating)
- Each combat phase re-runs the full §506-§511 sequence
- "at beginning of combat" triggers fire PER PHASE
- "whenever ~ attacks" triggers fire each time creature is declared attacker (can be multiple times if creature untapped between phases)
- "until end of turn" effects span both phases, don't refresh
- Tapped creatures from phase 1 can re-attack in phase 2 only if untapped (via Aggravated Assault's untap, or vigilance)
- Enters-tapped-and-attacking tokens from phase 1 are tapped at start of phase 2 — can't attack unless untapped
- Aggravated Assault + Sword of Feast and Famine = canonical infinite combats, must handle correctly

## Combat damage triggers (§510.2) — CR-correct per-instance firing

- **Double strike + "whenever deals combat damage" triggers** — MUST fire TWICE (once per damage step). Most engines get this wrong. Example: Goldspan Dragon + Fireshrieker = 2 Treasures per swing.
- **Lifelink** — triggers per damage INSTANCE, not per combat. Double strike lifelink = 2 life-gain events.
- **Deathtouch** — applies per damage event. Double-strike deathtouch kills blocker in first-strike step, then trample spillover in regular step.
- **Protection edge cases** — protection from color X on a first-strike blocker vs color-X double-strike attacker = blocker survives (no damage in either step).
- **"When X attacks" vs "When X deals combat damage" triggers** — attack triggers fire once at declare-attackers, damage triggers fire per damage-step.
- **Trample with first strike** — excess damage calculated per step, can trample twice if double strike and creature blocker dies to first-strike damage (0 toughness left → regular strike full damage to player).
- **"If damage would be dealt" replacement effects** — apply per damage event, so need to check per step in dual-damage combat.

## Stack and timing nightmares

- Stifle + fetchland — counter the trigger, not the spell (works in our engine ✓)
- Counterspell + split-second spell — split-second bypasses priority (works after patch ✓)
- Teferi, Time Raveler — opponents can only cast at sorcery speed (works after patch ✓)
- Mindslaver — control target player's next turn (partially implemented)
- Academy Ruins + Mindslaver loop — recurse top-of-library interaction (recurse works, control-turn partial)
- Panglacial Wurm — cast from library mid-tutor (priority window during search)
- Torment of Hailfire — X-variable damage doubler interaction
- Chains of Mephistopheles — replacement on draw triggered by discard, nearly broken in most engines
- Mindslaver + opponent has Teferi priest ability — can controlled player use hexproof/protection?

## State-based actions (§704)

- Legend rule with timestamps (which copy survives?)
- +1/+1 vs -1/-1 annihilation (ordering with Hardened Scales)
- Saga sacrifice at final chapter (what if chapter trigger is countered?)
- Planeswalker zero loyalty
- World rule (only one "world" permanent; ties kill all)
- Battle defense counter = 0 + protector
- Role uniqueness (only one Role per creature, timestamps for which stays)
- Token ceases-to-exist if left battlefield (affects copy effects)
- Dungeon completion (if we ever implement dungeons)

## Morph / face-down / manifest

- Morph creature — turn face-up for its morph cost
- Megamorph — additional +1/+1 counter when turned face-up
- Manifest — play face-down, can be turned up if it's a creature for its cost
- Ixidron — turns face-down
- Face-down cards and color-sensitive effects (protection from white on morph?)
- Face-down creatures in graveyard (retain face-down? become face-up? rule: face-up in graveyard)

## Commander-specific

- Commander tax (2 per cast from command zone)
- Commander damage (21 damage from one commander = loss, §903)
- Partner commanders (both legal, +1 to deck size per partner)
- Companion (pre-game effect, 3-cost tax to cast from sideboard once)
- Commander replacement zone choice on death/exile

## Delayed triggers

- "At the beginning of your next upkeep, X" — fires at your upkeep, even if source is gone
- "The next time X happens" — one-shot delayed replacement
- Delayed trigger + Sundial — does it still fire the NEXT turn (yes, per delayed-trigger rules)
- Delayed trigger with condition — evaluates when it fires, not when created

## Multiplayer AI policy — poker framing (7174n1c, 2026-04-16)

**Three-state behavioral policy for multiplayer EDH AI:**

- **HOLD** — neutral pressure, no attacks, reactive presence, combo pieces in hand only
- **CALL** — focused attacks on highest-threat opp, combo pieces onto battlefield, reactive + accumulate
- **RAISE** — combos fire, all-in swings, pump-and-dump, resource dump to close

**Transitions:**
- HOLD → CALL: opp threat exceeds threshold, combo piece available, you drew 2+ pieces
- CALL → RAISE: combo ready to fire, life ≤ 10 (forced commit), opp 1 turn from win, library ≤ 5
- HOLD → RAISE: burst window (top-decked one-shot)
- RAISE → HOLD/CALL: generally one-way once committed; only if combo fizzles

**Per-archetype defaults:**
- Burn: start CALL → RAISE by T4-5
- Control (Oloro): start HOLD, CALL when archenemy identified, RAISE on win-con firing
- Creatures (Sin/Ragost): start HOLD, CALL when mana ≥ 6 + threats visible, RAISE on payoff landing
- Combo (Coram): start HOLD, CALL when 1 combo piece down, RAISE when assembled

**Implementation plan:**
- Add `Seat.player_mode: PlayerMode` to dataclass
- `update_player_mode(game, seat)` called on event-stream triggers (NOT just upkeep):
  - opp_draw, opp_cast, opp_resolve, opp_attack, opp_eliminated
  - self_draw, self_damage
  - combat_damage, game_start
- Hysteresis thresholds (HOLD→CALL at score≥12, CALL→HOLD at score≤8) to avoid oscillation
- Cooldown: can't flip HOLD↔RAISE in same turn unless forced
- Event batching: update once after stack resolution, not per-event in a chain
- Route `declare_attackers` + `main_phase` cast decisions through mode-aware policy
- Log mode transitions as `player_mode_change` events with reason + trigger_event_seq (explainable transitions)

**Adaptive feel** (7174n1c refinement): "seat 2 watches seat 1 cast Doubling Season mid-seat-1's-turn → seat 2's mode shifts HOLD→CALL immediately, ready for when they get priority next." Organic feel via event-driven updates.

**Why:** 7174n1c's poker framing (hold/call/raise) maps elegantly to MTG multiplayer decision theory. Both are sequential-imperfect-information multi-agent games with resource commitment. Each player has a *stance* that evolves over the game, matching how humans actually play EDH.

## XMage bug-log candidates (Josh's suggestion — scrape XMage's issue tracker)

**TODO:** Pull XMage's GitHub issues, filter by label "rules bug" / "rules compliance" / "incorrect interaction", triage into test cases. If XMage gets it wrong and we get it right, that's the moat demonstrated.

Known XMage historical bugs to check:
- Humility + Opalescence deadlock (fixed in some version? verify current behavior)
- Painter's Servant color-wash timing
- Worldgorger Dragon infinite detection
- Layered ability grants (e.g., Crystalline Nautilus returning to hand interaction)
- Certain Cascade edge cases

---

## Harness integration plan

- Encode each case as a standalone function in the appropriate `interaction_harness_*.py`
- Each case: board state setup + scripted sequence + assertion on outcome (win / paradox / specific effect / no-op)
- Run at 100 reps × 4 deck contexts for regression signal
- Failures = either engine bug OR rules ambiguity — triage explicitly

**Status:** living document. Add entries whenever 7174n1c flags a new nightmare or XMage scrape surfaces a new rules bug.
