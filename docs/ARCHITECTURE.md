# HexDek Architecture

A 30-minute tour for new contributors. Reading this + the [README](../README.md) should be enough to understand the system end-to-end. Per-system deep dives live in `docs/architecture/` and are linked inline.

---

## The big picture

```
┌──────────────┐   ┌──────────────┐   ┌──────────────┐   ┌──────────────┐
│ Scryfall     │──▶│ Thor parser  │──▶│ AST corpus   │──▶│ Game engine  │
│ oracle text  │   │ (Python)     │   │ (jsonl)      │   │ (Go)         │
└──────────────┘   └──────────────┘   └──────────────┘   └──────┬───────┘
                                                                │
                ┌───────────────────────────────────────────────┘
                ▼
       ┌──────────────┐      ┌──────────────┐      ┌──────────────┐
       │  YggdrasilHat│◀────▶│  Tournament  │─────▶│  TrueSkill   │
       │  (AI player) │      │  runner      │      │  + Comp Prior│
       └──────────────┘      └──────┬───────┘      └──────────────┘
                                    │                      ▲
                                    ▼                      │
                             ┌──────────────┐      ┌──────────────┐
                             │  Heimdall    │      │  Freya       │
                             │  (analytics) │      │  (analysis)  │
                             └──────────────┘      └──────────────┘
                                    │
                                    ▼
                             ┌──────────────┐
                             │  hexapi + WS │──▶ React frontend
                             └──────────────┘
```

**One paragraph**: every printed Magic card is parsed once into a typed AST (`internal/gameast`). At runtime the game engine (`internal/gameengine`) reads the AST per card to execute Magic's rules — stack, priority, combat, triggers, replacements, state-based actions. An AI player called YggdrasilHat (`internal/hat`) makes decisions for each seat. The tournament runner (`internal/tournament`) plays thousands of games per minute, feeding TrueSkill ratings (`internal/trueskill`) and Heimdall analytics (`internal/analytics`). Freya (`cmd/hexdek-freya`) reads each deck once and produces deep analysis (archetype, bracket, S/A/B/C/D card tiers, win-line graph, synergy clusters) that flows back into both Hat (per-archetype weight profiles) and TrueSkill (composition matchup prior). The whole stack speaks REST + WebSocket via `internal/hexapi` + `internal/ws` to a React frontend at hexdek.dev.

---

## 1. Engine layers (`internal/gameengine`)

The game engine is the rules layer. It's the largest and most invariant-protected part of the codebase — a stress cycle ran to 100K games per canonical seed at zero invariant violations.

### Layer stack

```
gameengine/per_card     ← ~600 snowflake handlers (Athreos, Gisa, Zidane, ...)
       ↑
gameengine             ← rules layer (stack, combat, SBAs, replacements, zones)
       ↑
gameast                ← AST node types (Triggered, Static, Activated, Modification)
       ↑
astload                ← reads ast_dataset.jsonl into *Card
```

### Key files and what they own

| File | Owns |
|------|------|
| `state.go` | `GameState`, `Seat`, `Permanent`, `Card`, zone slices, NextTimestamp |
| `stack.go` | `ResolveStackTop`, `DrainStack`, `PushTriggeredAbility`, loop detection |
| `phases.go` | turn loop, phase transitions, delayed-trigger firing |
| `combat.go` | combat phases, attacker/blocker assignment, damage assignment, combat-damage SBAs |
| `sba.go` | `StateBasedActions` — 25 SBA helpers (`sba704_5a` through `sba704_6d`), 40-pass cap with mandatory-loop-draw cleanup |
| `replacement.go` | `gs.Replacements`, `FireEvent`, `pickReplacement`, §614 ordering |
| `zone_change.go` | `DestroyPermanent` / `ExilePermanent` / `sacrificePermanentImpl` / `BouncePermanent` — every canonical LTB path |
| `multiplayer.go` | `HandleSeatElimination` — §800.4a seat-loss cleanup |
| `resolve.go` + `resolve_helpers.go` | spell + ability resolution, modal effects, residual-text parsers |
| `zone_cast.go` | flashback / escape / cast-from-exile / library-cast permissions |
| `layers.go` | §613 continuous effects (layers 1-7e), `IsCreatureOf`, `PowerOf`, `ToughnessOf` |
| `trigger_batch.go` | CR §603.3b trigger batching (`BeginTriggerBatch` / `EndTriggerBatch` / `pendingTriggers`) |
| `invariants.go` | runtime invariant checks (SBACompleteness, CardIdentity, ZoneConservation, etc.) |

### Lifecycle invariants

The engine has 14 runtime invariants exposed via `gameengine.AllInvariants()`. The Loki fuzzer (`cmd/hexdek-loki`) runs them after every turn and reports any violations. R60-era closures landed defensive backstops at the canonical paths:

- **HandleSeatElimination** → calls `ExpireSourceGrants` + `UnregisterReplacementsForPermanent` + purges `pendingTriggers` for the leaving seat (CR §800.4a)
- **pickReplacement** → skips replacements whose `SourcePerm` has left play through non-canonical paths (defensive backstop)
- **SBA cap path** → marks all non-Lost seats Lost under CR §104.4b mandatory-loop-draw

Per-card handlers in `gameengine/per_card/` are the largest non-engine surface (~600 cards with handcrafted Go). They follow a registry pattern: `OnETB("Card Name", handler)`, `OnTrigger("Card Name", "creature_dies", handler)`, etc.

---

## 2. Hat / AI player (`internal/hat`)

YggdrasilHat is the AI. It picks every choice every player makes in a game — what to cast, what to attack, what to counter, what to block, what to scry-keep, what counters to put where.

### Decision-making layers

```
ChooseAction
    │
    ▼
┌─────────────────────┐
│ Budget System       │  0 = heuristic only
│                     │  1-199 = evaluator-guided
│                     │  200+ = full rollout MCTS
└─────────────────────┘
    │
    ▼
┌─────────────────────┐
│ Evaluator           │  20 dimensions:
│ (per_archetype)     │  BoardPresence, CardAdvantage, ManaAdvantage,
│                     │  LifeResource, ComboProximity, ThreatExposure,
│                     │  CommanderProgress, GraveyardValue, DrainEngine,
│                     │  ArtifactSynergy, EnchantmentSynergy,
│                     │  OpponentGraveyardThreat, PartnerSynergy,
│                     │  ActivationTempo, ToolboxBreadth,
│                     │  ThreatTrajectory, StackInteraction,
│                     │  PlaneswalkerProgress, ExileZoneAssets,
│                     │  StaxLockProgress
└─────────────────────┘
```

### Budget system

`internal/hat/budget.go` controls how much compute the Hat spends per decision:

- **Budget 0** — pure heuristic, no rollout. ~10K decisions/sec. Used for chaos-game fuzzing and mass-tournament sweeps.
- **Budget 1-199** — evaluator-guided. Scores each legal action against the 20-dim evaluator, picks the highest. ~1K decisions/sec.
- **Budget 200+** — full rollout MCTS via `internal/hat/mcts.go`. Builds an information-set tree, runs determinizations, backs scores up. ~100 decisions/sec at budget=500.

The budget adapts under load (`AdaptiveBudgetDegradation` in `budget.go`): if the board is complex enough that even budget=200 takes longer than a wallclock cap, the Hat falls back to a lower budget level for the rest of the turn.

### 22 archetype weight profiles

`internal/hat/eval_weights.go` ships 22 custom-tuned weight sets — one per Freya-recognized archetype. Each profile anchors the archetype's signature dimensions at the 2.0 mark and tunes the rest relative to a midrange baseline:

| Archetype | Signature dims |
|:---|:---|
| Aggro | BoardPresence 2.0 / LifeResource 1.6 |
| Control | CardAdvantage 1.6 / StackInteraction 1.5 / ManaAdvantage 1.3 |
| Burn | LifeResource 1.8 / ThreatExposure 1.4 |
| Combo | ComboProximity 2.0 |
| Storm | ComboProximity 2.0 / ManaAdvantage 1.5 |
| Reanimator | GraveyardValue 2.0 |
| Aristocrats | DrainEngine 2.0 |
| Voltron | CommanderProgress 2.0 / ThreatExposure 1.4 |
| Group Hug | CardAdvantage 2.0 / StackInteraction 0.9 / LifeResource 0.3 |
| Enchantress | EnchantmentSynergy 1.8 / CardAdvantage 1.4 |
| Artifacts | ArtifactSynergy 1.8 / ActivationTempo 1.2 |
| Stax | StaxLockProgress 1.6 / ThreatExposure 1.0 |
| ... (10 more) | |

When the Hat boots for a deck, Freya runs first (or its analysis is loaded from disk), and the archetype tag selects the weight set. Decks Freya can't classify fall back to the midrange profile.

### Self-trigger response matrix

When the Hat sees its own trigger about to fire and the resolution would be lethal to itself, it counters the trigger. The matrix (`internal/hat/poker.go::ChooseResponse`) covers 4 lethal-state classes:

- Mill (own deck would be empty + Lab Maniac dead)
- Life-loss (own life would go to 0)
- Damage (own creatures would take lethal)
- Commander damage (would self-Voltron over 21)

Validation gauntlet: 2,500 games × 5 seeds = 0 self-harm fires.

### Other AI components

- **Genetic Curse** (`internal/hat/curse.go`) — per-deck DNA evolution, 7-param genome, population of 8.
- **Shannon Entropy** (`internal/hat/shannon.go`) — opponent hand probability model.
- **Neural Evaluator** (`internal/hat/neural.go`) — 92-dim MLP trained via self-play.
- **Strategy Profile Bridge** — Freya's win-lines / value-engines / finishers / synergy clusters flow into Hat decisions via `StrategyProfile`.

---

## 3. Freya — deck analysis (`cmd/hexdek-freya`)

Freya is the deck-analysis pipeline. Run once per deck, output is a `DeckProfile` consumed by:
- The Hat (selects archetype weight profile, surfaces win lines)
- TrueSkill (matchup matrix → CompositionPrior)
- The Decks app frontend (renders archetype tag, S/A/B/C/D tiers, bracket, suggestions)

### Pipeline

```
deck.txt
    │
    ├─▶ deckparser (text → []Card)
    │
    ├─▶ archetype.go::ClassifyArchetype (22 fingerprints)
    │
    ├─▶ estimateBracket (B1 casual → B5 cEDH with rationale)
    │
    ├─▶ valueChains (Storm / Enchantress / Artifact / Counters engines)
    │
    ├─▶ KnownCombos check (58 curated + Commander Spellbook import)
    │
    ├─▶ computeCardPower (0-100 per card, S/A/B/C/D tier)
    │
    ├─▶ computePetCards (off-archetype flavor picks)
    │
    ├─▶ computeSynergyClusters (death_value / etb_value with chain bonus)
    │
    ├─▶ Opening hand simulation (10K Monte Carlo)
    │
    └─▶ DeckProfile JSON (consumed by Hat, TrueSkill, frontend)
```

### Card power tiers

Every non-land card gets a 0-100 power score with 3 explicit components:

- **ArchetypeFit (0-40)** — role-vs-fingerprint match against the deck's archetype
- **CMCEfficiency (0-20)** — absolute curve band (CMC≤1: 20, CMC2: 18, ..., CMC6+: 2)
- **SynergyContribution (0-40)** — wincon piece (+25), bridge (+20), step (+10), finisher (+12), cluster member (+6), with penalties

The score buckets into absolute S/A/B/C/D tier bands (70/55/38/25 thresholds calibrated against a 300-deck moxfield corpus to hit ~7/18/35/30/10%). Each card gets a one-line WHY explanation: `★ [S 84] Thassa's Oracle — wincon piece + 3-role at CMC 2 + Combo fit (Tutor/Combo)`.

### Bracket B5 cEDH gate

WotC's bracket system has 5 levels (B1 casual → B5 cEDH). Freya scores the deck and applies a **B5 confirmation gate**: bracket=5 only if (`freeInteractionCount >= 2` OR `tutorDensity >= 0.12` OR `gameChangerCount >= 8`) AND `avgCMC < 2.8`. Otherwise demoted to B4.

Every bracket call ships with a **rationale table** showing per-signal contribution + evidence cards.

### Meta-positioning matchup matrix

22 archetypes × 22 archetypes matchup matrix with a reciprocity invariant (`matchup(A,B) + matchup(B,A) = 0` for symmetric matchups). This matrix is the input to TrueSkill's `CompositionPrior`.

---

## 4. TrueSkill + CompositionPrior (`internal/trueskill`)

The rating system. Bayesian μ/σ per deck, 4-player FFA + 1v1 support.

### Standard TrueSkill pipeline

```
Game result (ranks[])
    │
    ▼
UpdateMultiplayer(cfg, ratings[], ranks[])
    │
    ├─▶ Sort by rank
    ├─▶ Accumulate pairwise update deltas
    └─▶ Apply to History + Ratings map
```

### CompositionPrior wave (R60)

```
[seat0_archetype, seat1_archetype, seat2_archetype, seat3_archetype]
    │
    ▼
CompositionPrior.Score(archetypes)
    │
    ├─▶ Look up each seat-pair in Freya matchup matrix
    ├─▶ Apply Wilson 95% confidence (low-data cells shrink to 50/50)
    └─▶ Return expected-winrate vector

    │
    ▼
TrueSkillRatings.Update(participantNames, ranks, prior)
    │
    ├─▶ Shrink mu delta when matchup predicted the result
    └─▶ Amplify mu delta when result surprised the prior
```

Validation: **+1.4pp accuracy / +0.036 log-loss** vs no-prior baseline. Monitoring lives in Heimdall analytics; `cmd/hexdek-composition-replay` is the offline what-if CLI.

### Drift / smurf detection

`internal/trueskill/drift.go` walks each deck's rating history to flag suspiciously rapid mu shifts (smurf detection, sandbagging, post-update outlier games).

---

## 5. Heimdall analytics (`internal/analytics`)

Heimdall is the post-game analytics. After every game, it reads the event log + final state and produces:

- **Per-card stats** — how often each card was cast, drawn, sacrificed, won-the-game-on
- **Win-line analysis** — which combo path the winner used, which pieces showed up
- **Resource tracking** — turn-by-turn mana production, draws, ramp curves
- **Missed combos** — pieces that came together but never executed
- **Stall detection** — games that decision-loop in the same state
- **Causal pivots** (via Tesla) — the decisive turn/action
- **Three-act narrative** (via Ive) — setup / conflict / resolution

### GameSummary infrastructure

R60 shipped a per-game `GameSummary` data structure that consolidates all the analytics into a single serializable JSON payload — the basis for:

- Share-link infrastructure (a game's summary URL is stable, opens in any browser)
- Archive view (historical games filterable by deck, archetype, outcome)
- Composition-effect surfacing (shows how much of the rating delta came from the CompositionPrior vs the actual upset)
- PDF export (full game summary as a printable post-game report)

---

## 6. Tournament runner (`internal/tournament`)

The bulk-game driver. Supports several modes:

- **Round-robin** — every deck plays every other deck N times
- **Balanced pool** — rating-similar decks paired
- **Swiss** — pairing by current standings
- **Double elimination** — bracket-style
- **Gauntlet** — fixed deck vs random pool

Single-machine throughput: ~500 games/sec for chaos fuzzing (budget=0 Hat), ~50-100 games/sec for full Hat (budget=50 default).

The tournament runner emits per-game GameSummary records via Heimdall, updates TrueSkill ratings via the composition-prior path, and writes results to SQLite (`internal/db`).

### Loki fuzzer

`cmd/hexdek-loki` is the dedicated chaos-game runner. Phase 1 plays N random 4-player Commander games with random decks and runs invariants after every turn. Phase 2 builds N random nightmare board states (5 random permanents per seat, no library context) and runs invariants once. The fuzzer reports any invariant violation, including the game ID, seed, turn, phase, recent events, and full state snapshot — making every residual deterministically reproducible.

R60 final stats: 100K canonical chaos games + 100K nightmare boards = **0 violations**.

---

## 7. hexapi + WebSocket stack (`internal/hexapi` + `internal/ws`)

The serving layer.

### REST API (`internal/hexapi`)

- `/decks/*` — deck CRUD, archetype tagging, version history, save views
- `/games/*` — game summary, archive, share-links
- `/tournaments/*` — tournament status, leaderboards, history
- `/composition/*` — CompositionPrior debug endpoints + replay CLI backend
- `/freya/*` — deck analysis endpoints (archetype, brackets, tiers)
- `/auth/*` — Firebase auth

### WebSocket (`internal/ws` + `internal/hub` + `internal/party`)

- **Spectator** — live game streaming (event log + state diffs to subscribed clients)
- **Party** — lobby system: 4 humans + AIs assemble a pod
- **Queue** — matchmaking queue, rating-aware pod assembly

### Frontend (`hexdek/`)

React + Vite. Brutalist design system. Main screens:

- **Spectator** — live game view with event stream
- **Decks** — deck list with system-tagged archetypes, S/A/B/C/D tier rendering, recent games inline
- **Queue** — matchmaking lobby
- **Party** — pre-game lobby
- **Heimdall** — post-game analytics, archive, share-links

---

## Data flow at runtime: one game

```
1. Tournament runner picks 4 decks from the pool
2. Freya loads each deck's pre-computed DeckProfile from disk
3. Hat boots one per seat, selects per-archetype weight profile
4. CompositionPrior is computed from the 4-archetype tuple
5. Game loop:
     for each turn:
       Phase / step transitions (gameengine/phases.go)
       Hat picks an action (internal/hat::Choose*)
       gameengine executes (stack.go / resolve.go / combat.go)
       SBAs run (sba.go)
       Loki invariants check (only during fuzz runs)
6. CheckEnd → winner detected
7. Heimdall reads event log → builds GameSummary
8. TrueSkill.Update applies the result with CompositionPrior weighting
9. SQLite persists Game + GameSummary
10. WebSocket broadcasts the final state to subscribed spectators
```

---

## Where to start as a contributor

If you want to **add a new card handler**:
- Read `internal/gameengine/per_card/athreos_shroud_veiled.go` (representative)
- Register via `r.OnETB("Card Name", handler)` in the file's `register*` function
- Add an entry to the registry init (`registry.go` calls every `register*` function)
- Write a test in `internal/gameengine/per_card/` matching the existing patterns

If you want to **fix a Loki violation**:
- Run `go run ./cmd/hexdek-loki --games N --seed S --invariant <kind>` to capture details
- Read the violation's recent events + state snapshot from `data/rules/CHAOS_REPORT.md`
- Add a regression test pinning the bit-stable shape
- Walk the suspected code path and add the missing call / guard
- See `CLAUDE.md` Issue Log Resolved for ~12 reference closures

If you want to **add a new Freya signal**:
- Read `cmd/hexdek-freya/advanced.go::computeCardPower` for the established pattern
- Plumb the new field through `DeckProfile` → `jsonDeckProfile` → text report
- Add tests in `cmd/hexdek-freya/*_test.go`
- Wire into the Hat's `StrategyProfile` if the signal should influence AI decisions

If you want to **understand a specific subsystem deeply**:
- Engine layers: read `internal/gameengine/state.go` + `stack.go` + `sba.go` in order
- Hat: read `internal/hat/poker.go::ChooseAction` + `budget.go` + `eval_weights.go`
- TrueSkill: read `internal/trueskill/trueskill.go::Update` + the new `CompositionPrior` files
- Tournament: read `internal/tournament/balanced_pool.go` as the canonical pattern

### Building and running

```bash
# Build everything
go build ./...

# Run tests
go test ./internal/gameengine/... -count=1 -timeout 300s

# Loki fuzz one seed
go run ./cmd/hexdek-loki --games 1000 --seed 42

# Freya-analyze a deck
go run ./cmd/hexdek-freya/ --deck data/decks/test/mydeck.txt

# Run a tournament
go run ./cmd/hexdek-tournament/ --decks data/decks/ --games 100

# Start the server (needs data/rules/oracle-cards.json)
go run ./cmd/hexdek-server/

# Frontend dev
cd hexdek && npm install && npm run dev
```

---

## Per-system deep dives

For full depth on any of the above subsystems, see the per-system docs in `docs/architecture/`:

- [Card AST and Parser](architecture/Card%20AST%20and%20Parser.md)
- [Engine Architecture](architecture/Engine%20Architecture.md)
- [Zone Changes](architecture/Zone%20Changes.md)
- [Layer System](architecture/Layer%20System.md)
- [Stack and Priority](architecture/Stack%20and%20Priority.md)
- [Trigger Dispatch](architecture/Trigger%20Dispatch.md)
- [Mana System](architecture/Mana%20System.md)
- [Combat Phases](architecture/Combat%20Phases.md)
- [Replacement Effects](architecture/Replacement%20Effects.md)
- [State-Based Actions](architecture/State-Based%20Actions.md)
- [YggdrasilHat](architecture/YggdrasilHat.md)
- [Hat State Machine](architecture/Hat%20State%20Machine.md)
- [MCTS and Yggdrasil](architecture/MCTS%20and%20Yggdrasil.md)
- [Eval Weights and Archetypes](architecture/Eval%20Weights%20and%20Archetypes.md)
- [Tool - Freya](architecture/Tool%20-%20Freya.md)
- [Tool - Heimdall](architecture/Tool%20-%20Heimdall.md)
- [Tool - Loki](architecture/Tool%20-%20Loki.md)
- [Tool Suite](architecture/Tool%20Suite.md)

And for the R60 stress-cycle narrative + 12-fix audit trail: [release-notes-r60.md](release-notes-r60.md).
