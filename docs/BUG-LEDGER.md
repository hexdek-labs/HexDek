# HexDek Bug Ledger

A durable, honest record of every known bug worth remembering, sorted by
*resolution kind* rather than by date:

- **FIXED** — a real defect, root-caused and corrected. One line of root
  cause + fix.
- **WORKED-AROUND** — mitigated so it's no longer user-fatal, but the
  underlying root cause is still open.
- **WAS-A-BUG-THEN-BECAME-INTENDED** — logged as a violation, turned out
  the engine was right and the *check* (or our expectation) was wrong.
- **OPEN** — known, unresolved, reproducible (or measured) today.

Each entry: **date · area · one-line · status**. Sources: CLAUDE.md
Issue Log (Open + Resolved), the r63 correctness/Loki reports under
`docs/` and the fable-review set, and `git log`. Newest-first within
each group. This ledger is a map of conclusions — for the full forensic
write-up of any entry, follow the round-stamped report named in it.

---

## FIXED

Real defects, root cause → fix in one line.

| Date | Area | Root cause → fix |
|------|------|------------------|
| 2026-06-12 | engine · CONSERVATION §800.4a | Eliminated-player residual class (ZoneConservation/CardIdentity): §800.4a didn't reach every card container and zone-movers didn't refuse departed cards → fixed across merged-limbo drain, CastSpell transit-window census gap, survivor-zone sweep, and dead-owner guards on death-trigger graveyard lifts (Gisa +3 siblings). 0 hits / 6,000 strict-census games. (`d84bc71a..67810ac9`) |
| 2026-06-12 | engine · CONSERVATION | Athreos returned an *eliminated* player's creature via `createPermanent` with no §800.4a guard → guard added; CONSERVATION 99.67%→100%. (#1062) |
| 2026-06-12 | engine · CONSERVATION | Knowledge Pool granted a free cast of an eliminated player's already-ceased card (§800.4a) → ceased-card cast gate. (#1044) |
| 2026-06-12 | engine · CONSERVATION | OGVC fabrication: eliminated owner's card re-materialized through a LeftGame zone-move → §800.4a LeftGame zone-move guard. (#1041) |
| 2026-06-12 | engine · CONSERVATION | Stolen permanents of an eliminated player vanished from all zones (prod zone_accounting light-seats) + mid-resolution limbo window left an in-flight spell un-ceased → orphan sweep ceases in-flight spell; elimination no longer misses the caster's own resolving card. (`1229264f`, #1043) |
| 2026-06-12 | engine · LIVENESS | Plargg / Possibility Storm / Chaos Wand spun an infinite upkeep loop on an eliminated seat's *retained* library → per-card loop guards on eliminated-seat libraries. (#1065) |
| 2026-06-12 | engine · PROGRESSION | `gain_life` and `becomes_tapped` AST triggers were ALL silent — the event alias mapped to a name no dispatch walk consulted → `FireLifeGainedASTTriggers` / `FireTapEventASTTriggers` at the GainLife + four tap-transition chokepoints. (#1068) |
| 2026-06-12 | engine · PROGRESSION | The whole impulse-exile trigger family ("exile the top card of your library") was countered at resolution — the §608.2b fizzle gate treated the `library_top` zone selector as a required battlefield target → zone-selector bases are never required targets. (#1068) |
| 2026-06-12 | engine · PROGRESSION | "At the beginning of EACH upkeep" cards fired only on their controller's upkeep — parser emits Controller=None and the matcher defaulted "" to controller-gated → raw-aware each/your matcher. (phase-2 / #1057) |
| 2026-06-12 | engine · PROGRESSION | §603.3b APNAP violation: simultaneous deaths in one SBA sweep resolved the ACTIVE player's trigger first (backwards) — `sba704_5f/5g/5h` ran with no trigger-batch frame → standard `BeginTriggerBatch/EndTriggerBatch` idiom orders them APNAP. (`cd04b3fc`) |
| 2026-06-12 | engine · OUTCOME | Six per-effect engine no-ops caught by the second AST interpreter: tutor-whiff, distribute-counters no-op, own-land bounce policy, impulse-exile no-op, duplicate ETB counters + Dina phantom drain, up_to_n self-targeting removal → each fixed at its resolution site; OUTCOME 100%. (`3eb60fe1`, `4b0fa7ca`, `0c5e9572`, `671aa6ff`, `9e39042e`, `2155ddef`) |
| 2026-06-12 | engine · STATE-INTEGRITY | MDFC back-face land carried the front-face *spell* type — the correctness baseline's lone STATE-INTEGRITY offender → face-type derivation fixed. (`851f1870`) |
| 2026-06-11 | engine · LEGALITY | §704.6d commander redirect left a linked-exile claim dangling (seed-42 game 486, the 1-in-500 strict-census residual) → claim released on redirect. (#1049) |
| 2026-06-11 | engine · LEGALITY | Knights memorabilia §601.2f-h: elimination pool-clear misread as a payment → 3 pool-hygiene desyncs fixed + loki memorabilia filter. (`2010297a`) |
| 2026-06-10 | engine · LEGALITY | r62 casting/cost/mana sweep: Esper Sentinel / Rhystic Study / Mystic Remora double-tax from duplicate Wave-1b inline observers; restricted-mana under-pay (`SyncManaAfterSpend` stranded restricted mana); defensive-gate double-pay + 21-site pool drift + madness credit; hats illegally blocking landwalkers → each closed at its chokepoint. (#1030, #1032, #1034/#1036, `e34c8e4a`, `0f136f01`) |
| 2026-06-?? | engine · LEGALITY | **Sorceries uncastable** — the era's most humbling bug, surfaced by external review: an entire card class could not be cast → fixed and attributed honestly in the changelog. (#1011) |
| 2026-05-20 | engine · CardIdentity | God-Eternal Oketra appeared in both library and graveyard — `god_eternal_tuck` called `removePermanent` AFTER the death zone-change so it no-op'd and the card duplicated into the library → capture `fromZone`, fall through GY→exile→hand removal, fire real zone-change triggers. CardIdentity −50, game 333 cleared. (`god_eternal_tuck_r48`) |
| 2026-05-19 | engine · ZoneConservation | Krark/paradigm copy resolution leaked `*Card` into `gs.ParadigmExile` every tick (824 hits) — copy StackItem lacked `IsCopy` and `paradigmExileItem` re-appended on copies → `IsCopy: true` so §707.10 ceasing applies; helper short-circuits on copies. game 181 86→0. |
| 2026-05-19 | engine · CardIdentity | Cerulean Sphinx zone-leak (1,622 of 1,652) — `collectSpellEffect` ran an activated-ability body at CAST time + synthetic Permanent omitted `Owner` (defaulted seat 0) → return nil for permanent spells; thread `Owner: item.Card.Owner`. game 137→0. |
| 2026-05-19 | engine · CardIdentity | Disruptive Pitmage hand+battlefield duplication — `{T}:` activated counter looked hand-castable to `counterSpellEffect` → empty-activation-cost gate (defense-in-depth over the Adric r44 fix). game 404 18→0. |
| 2026-05-16 | per_card · crash | 324 nil-deref panics: `abdelAdrianETB` bypassed §614 / §903.9b / aura detach by hand-rolling `removePermanent + moveCardBetweenZones` → route each exile through the canonical `ExilePermanent`. (`may11-nil-deref-forensics.md`) |
| 2026-05-17 | per_card · tests | Test pollution: `Reset()` rebuilt the registry from `registerDefaults()` only, permanently dropping ~50 `init()`-registered handlers (Sai et al.) → `AddResetHook`; every init re-registers on the fresh registry. |
| 2026-05-08 | engine · CardIdentity | Dread (and 8 "put into graveyard from anywhere" siblings) duplicated into owner's library — `shuffle_pronoun_into_owner_library` only removed from the battlefield → fall through GY/exile/hand removal + thread the real `from_zone`. |
| 2026-05-08 | thor · Goldilocks | 1,795 `keyword_dead` failures — `RetainEvents` default-false dropped every event, and combat keywords had no scaffold → `RetainEvents: true` + a combat scaffold; 1,795→0 (total 1,915→54). |
| 2026-05-08 | freya | Commander-centric decks read as unkeepable — keepable-hand heuristic only sampled nonbasic profiles and required "any non-land" → per-slot category sim + `detectCommanderCentric` + `KeepableHandPctAdjusted` (Voltron Uril 37%→63/80%). |

## WORKED-AROUND

Mitigated — no longer user-fatal — but the root cause is still open.

| Date | Area | Mitigation shipped / root cause still open |
|------|------|--------------------------------------------|
| 2026-06-08 | engine · §704.5y | **sba704_5y Role-uniqueness ↔ trigger-resolution mutual recursion.** Unbounded SBA↔stack re-entrancy overflows the stack (and sometimes OOMs — a 54 GB alloc seen). **Mitigation:** depth-guard at the re-entrancy boundary logs a loop anomaly and ENDS THE GAME AS A DRAW instead of crashing the process (PR #1012); `run-server.bat` self-heal relaunch loop is a second backstop. **Root cause OPEN:** the §704.5y Role-uniqueness loop itself (a death/LTB trigger re-granting a Role each resolution) — needs a captured `josh/cleohaus_tergrid` repro. See [[project_sba704_5y_stack_overflow]]. |
| 2026-06-12 | engine · LIVENESS | The LIVENESS watchdog (#1067) is a *standing* mitigation: any non-terminating game is aborted by a sampled production watchdog + uniform `loop_guard_fired` event. It bounds the symptom class generally; individual loops (e.g. §704.5y above) still want root-causing. |

## WAS-A-BUG-THEN-BECAME-INTENDED

Logged as a violation; the engine turned out correct and the *check* (or
our expectation) was wrong. The fix was to the validator, not the engine.

| Date | Area | What was flagged → why it was actually correct |
|------|------|------------------------------------------------|
| 2026-06-12 | hat/judge · zone_accounting | The Feynman count-heuristic flagged seats "-5..-20 cards-light" on every elimination game → those were legal §800.4 departures (cards ceasing across all zones); counts fundamentally can't model the departure flow. The InstanceID strict census is the real authority (499/500 clean) → heuristic demoted to unminted-state fallback. (`d486fff3`) |
| 2026-06-11 | engine · invariants | "Winner at negative life" flagged as an invariant violation (seed 8675309 game 209) → a legal Platinum Angel win; the winner legitimately survived at negative life. False positive removed. (`c24b27af`) |
| 2026-06-11 | engine · LEGALITY | §614.1a "stamp-class" replacement flagged as illegal → legal: replacements that redirect a zone change correctly declare `RedirectsZone`. Check taught the legal shape. (`449b85c9`) |
| 2026-06-12 | judge · PROGRESSION | The trigger-correctness audit's "phantom fires" (engine fired Beastmaster on another creature's attack; refrained on Scaleguard without a counter) → the ENGINE was right; the AST drops the actor restriction, so the CHECKER's bare self-trigger assumption was wrong. Scope narrowed; parser gap handed to the corpus lane. |

## OPEN

Known, unresolved, and measured/reproducible today.

### Correctness-score residuals (400-game seed-42 sweep, r63)

| Date | Area | One-line | Status |
|------|------|----------|--------|
| 2026-06-12 | judge · PROGRESSION | **28 trigger-dispatch bugs** surfaced when coverage widened ~68% (now ~3,026 triggers measured) — always there, newly *visible*. Dominated by the ally-ETB contextual-ref plumbing cluster (~10: "that creature gets +X" / Cathars'/Goldnight anthem-count), the "you may sacrifice ANOTHER creature" parser-drop class (3: Caesar, Surtland Flinger, Terror Ballista), plus singles (Michelangelo, Shambling Swarm, Broodguard, Havengul/Dream Pillager sum-accounting). PROGRESSION 99.07%. Fix order: ally_etb/etb cluster first. | OPEN |
| 2026-06-13 | engine · STATE-INTEGRITY | **1 game** of 400 fails on `seat_outcome/winner_count` (a winner-count edge in the per-seat win/loss self-checker). STATE-INTEGRITY 99.75%. | OPEN |
| 2026-06-12 | engine · STATE-INTEGRITY | **Heliod, Sun-Crowned ExileLinkageIntegrity** — orphaned linked exile, seed 31337 game 1196; the documented §406.7 deferred-return TODO. | OPEN |
| 2026-06-12 | engine · CONSERVATION | **Orphan-sweep silent-mop population** — the orphan sweep retires a real leak population (~20 games / 2,000 surface if the sweep is delayed; proven by a rejected turn-grace A/B). Never delay the sweep (grace regressed CONSERVATION 300/300→297/300). | OPEN |

### Loki / Issue-Log carry-overs (CLAUDE.md)

| Date | Area | One-line | Status |
|------|------|----------|--------|
| 2026-05-19 | Loki r41 · AttachmentConsistency | Aura/equipment attached state diverges from controller's battlefield — likely stale `AttachedTo` after a cross-seat control change. | OPEN (Low) |
| 2026-05-19 | Loki r41 · TriggerCompleteness | Trigger batch opened but never drained — likely a `BeginTriggerBatch` whose `defer EndTriggerBatch` was bypassed by an early return. | OPEN (Low) |
| 2026-05-19 | Loki r41 · ZoneCastGrantExpiry | Impulse-play / cast-from-exile grant outlived its declared expiry — suspect `resolveResidualByText` impulse_play grants lacking a cleanup hook. | OPEN (Low) |
| 2026-05-19 | Loki r44 · ZoneConservation (game 420) | "Real cards disappeared" (28 hits, Breya/Alela deck, turn 10+) — *likely covered* by the r63 CONSERVATION residual-class closure (0/6,000 strict games) but not yet struck from the Issue Log. | OPEN (Med — verify) |
| 2026-05-22 | Forge · TLA flashback-grant | `Iroh, Grand Lotus`'s graveyard-flashback-grant parses to a raw-text scaffold; engine never grants flashback (Iroh ran 19% with his recursion engine inert). Representative of TLA complex-ability cards. Needs a `grant_flashback_in_graveyard` primitive. | OPEN (Med) |
| 2026-05-08 | Corpus Audit | 4,190 unbucketed condition/trigger AST nodes (33.9% of 12,363) across all four eras — Era 1 biggest gap, Era 3 highest %. | OPEN (Info) |

### Parser-fidelity frontier (handed to the corpus/parser lane)

| Date | Area | One-line | Status |
|------|------|----------|--------|
| 2026-06-12 | parser · trigger actors | The AST DROPS trigger actor restrictions — "a creature you control [with X] attacks", "one or more OTHER creatures die", bolster's "your creature with the least toughness" all flatten to bare events; the engine dispatches with more nuance than the AST carries (a parity risk worth its own audit). | OPEN |
| 2026-06-12 | parser · "another" / intervening-if | "you may sacrifice ANOTHER creature" drops "another" (picker can choose the source); conditional phase triggers lose their if-clause; the corpus emits ZERO `intervening_if` nodes (§603.4 now implemented engine-side, lights up when the parser emits the clause). | OPEN |
