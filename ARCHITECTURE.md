# HexDek Architecture

One-page map of how oracle text becomes a 4-player Commander game with political AI.

```
Scryfall bulk (50K+ cards)
    │
    ▼
Parser (Python) ──→ Typed AST (CardAST per card)
    │                     │
    ▼                     ▼
Coverage reports    Go Engine (internal/gameengine)
                         │
            ┌────────────┼────────────────┐
            ▼            ▼                ▼
    Layer System    Zone System     Trigger Dispatch
    (§613 L1-7e)   (MoveCard)     (ETB/LTB/cast/dmg)
            │            │                │
            └────────────┼────────────────┘
                         ▼
               YggdrasilHat (AI)
               ┌─────────┼──────────┐
               ▼         ▼          ▼
          20-dim Eval  IS-MCTS   State Machine
          + Neural     + Shannon  (6 game plans)
               │         │          │
               └─────────┼──────────┘
                         ▼
              Tournament Runner (Thor)
              ┌──────────┼──────────────┐
              ▼          ▼              ▼
          Grinder    Bracket Pod    Showmatch
          (bulk)     (rated)        (spectated)
              │          │              │
              └──────────┼──────────────┘
                         ▼
              Heimdall (observation routing)
              ┌──────┬───┼───┬──────────┐
              ▼      ▼   ▼   ▼          ▼
          Huginn  Muninn Tesla Feynman  GA4
          (synergy)(gaps)(pivot)(oracle) (telemetry)
              │      │    │      │
              ▼      ▼    ▼      ▼
            Freya ←─────────── Learning Loop
          (strategy)      Self-Play + Neural Training
```

---

## 1. Parser pipeline

Source: `scripts/parser.py` (~98K lines counting inlined grammar tables).

Recursive-descent parser with ~50 extension modules. For each card:

1. **`normalize(card)`** — pull `oracle_text`, concatenate `card_faces` on DFCs, lowercase, strip reminder text, replace card's own name with `~`.
2. **`split_abilities(text)`** — break on canonical delimiters (newlines, keyword lists, modal bullets).
3. **`parse_ability(chunk)`** — try each production: Keyword → Triggered → Activated → Static → Spell fallback.
4. Unconsumed text recorded in `parse_errors` for coverage tracking.

100% syntactic coverage across all 50K+ Scryfall cards. Zero parse failures.

AST schema (`scripts/mtg_ast.py`): frozen dataclasses implementing CR §113.

| Node | Shape | Example |
|---|---|---|
| **`Static`** | `(condition?, modification?, raw)` | `Creatures you control get +1/+1` |
| **`Activated`** | `(cost, effect, timing_restriction?, raw)` | `{T}: Add {G}` |
| **`Triggered`** | `(trigger, effect, intervening_if?, raw)` | `When ~ enters, draw a card` |
| **`Keyword`** | `(name, args, raw)` | `Flying`, `Flashback {2}{U}` |

Extensions in `scripts/extensions/` organized by concern. `per_card.py` handles snowflake cards that defeat any reasonable grammar.

---

## 2. Go game engine

Source: `internal/gameengine/` — the production backbone.

Full Commander rules implementation:

- **Turn structure**: untap → upkeep → draw → main1 → combat → main2 → end step → cleanup
- **Stack & priority**: APNAP ordering (§603), split-second, modal/kicker/X spells
- **Combat**: 5-step combat, first/double strike, trample, menace, provoke, goad
- **Zone system**: universal `MoveCard` (returns `MoveResult{FinalZone, Permanent}`) with replacement effect interception
- **Cast-from-zone systems**: 6 structural patterns for non-hand casting:
  - `ZoneCastPermission` — flashback, escape, adventure, impulse draw, static exile-cast (with Duration/GrantTurn/SourceTimestamp/SpendAnyColor)
  - `AdditionalCost` — sacrifice, bargain, exile-from-graveyard (escape's exile-N cost)
  - `ExileLinked` — O-Ring/Fiend Hunter/Knowledge Pool "exile until source leaves" (CR §406.7)
  - `Prepared` — Strixhaven DFC state flag + copy-cast from battlefield
  - `ParadigmExile` — recurring free copy-cast from exile at first main phase
  - Duration lifecycle: `ExpireZoneCastGrants` at end-of-turn, `ExpireSourceGrants` on LTB
- **Layer system**: §613 continuous effects (layers 1–7e), dependency ordering, cache invalidation
- **Trigger dispatch**: zone-change, ETB/LTB, cast, damage, state-based triggers + CI lint guard
- **State-based actions**: full §704 (lethal damage, 0 toughness, legend rule, poison, commander damage, etc.)
- **Mana system**: 6-type pool, hybrid/phyrexian, convoke, treasure, restricted mana
- **Replacement effects**: §614/§616 ordering, shield counters, self-replacement
- **Per-card handlers**: 750+ named handlers covering all 652 deck-pool commanders

Detailed docs: `docs/architecture/`

---

## 3. AI system — YggdrasilHat

Source: `internal/hat/` — single unified brain, every decision flows through one evaluation pipeline.

### Evolution levels (all implemented)

| Level | System | What it does |
|---|---|---|
| **1** | YggdrasilHat core | 20-dim board evaluator, political multiplayer, budget-controlled search |
| **2** | Combo Sequencer | SAT constraint solver for multi-card combo execution |
| **2.5** | Hat State Machine | 6 game plans (Develop/Assemble/Execute/Disrupt/Pivot/Defend) with transitions |
| **3** | Genetic Curse | Per-deck DNA evolution — 7-param genome, population of 8, persisted per deck |
| **4** | IS-MCTS | Information-Set Monte Carlo tree search with determinization, 3 rollouts/candidate |
| **5** | Neural Evaluator | 92-dim MLP, trained via self-play, 80/20 heuristic/neural blend |
| **6** | Self-Play Loop | Auto training: sample threshold → PyTorch → hot-reload Go inference |

Supporting systems:

- **Shannon Entropy**: opponent hand probability model, `LikelyHasAnswer` heuristic
- **Lovelace Composer**: star card boost + commander theme keyword matching
- **Watts Soul Layer**: bracket-aware confidence dial (B1=0.3 / B5=0.9)
- **UCB1**: exploration factor per archetype/turn (Aggro=1.0 → Combo=1.8)

### 20 evaluator dimensions

BoardPresence, ManaAdvantage, CardAdvantage, LifeTotal, CommanderPresence, GraveyardValue, ComboProgress, ThreatLevel, BoardWipeVulnerability, TempoAdvantage, ManaEfficiency, PoliticalStanding, ResourceDiversity, SynergyDensity, TutorAccess, LandCount, StackInteraction, PlaneswalkerProgress, ExileZoneAssets, StaxLockProgress

Each dimension has per-archetype weights (13 archetypes), stage scaling, and position scaling via `rescaleWeights`.

Detailed docs: [YggdrasilHat](docs/architecture/YggdrasilHat.md), [Hat AI System](docs/architecture/Hat%20AI%20System.md)

---

## 4. Tournament forge

Source: `internal/tournament/`, `internal/hexapi/`

Three game paths, all routed through Heimdall observation:

| Path | Purpose | Workers |
|---|---|---|
| **Grinder** | Bulk unrated games, maximal throughput | 12 parallel |
| **Bracket Pod** | Rated 4-player pods, TrueSkill + HexELO updates | On-demand |
| **Showmatch** | Spectated games, WebSocket broadcast, narrative arc | Single |

Rating systems: TrueSkill (Bayesian μ/σ, bracket-seeded) + HexELO (traditional). Bracket classification (B1–B5) via Freya strategy analysis.

---

## 5. Learning loop

Source: `internal/heimdall/`, `internal/muninn/`, `internal/hat/selfplay.go`

```
Game completes
    │
    ▼
Heimdall Observer (singleton)
    ├── Seeds → disk (JSONL, 1000-entry ring buffer)
    ├── Co-triggers → Huginn (synergy discovery, tier graduation)
    ├── Parser gaps → Muninn (persistent gap memory)
    ├── Dead triggers → Muninn (handler audit)
    ├── Crashes → Muninn (panic recovery)
    ├── Causal pivot → Tesla (decisive turn extraction)
    ├── Eval snapshots → TrainingCollector (every 5 turns per seat)
    └── Health pulse → GA4 telemetry

Huginn graduates Tier 3 patterns → exports to Freya
Freya reads patterns → refines strategy profiles → feeds back to hat construction
TrainingCollector → samples.jsonl → PyTorch training → model.json → hot-reload
```

Post-game validation: **Feynman Oracle** runs 8 invariant checks (§704.5a/c/f/v, zone accounting, winner count, turn bounds, negative counters) on every completed game.

---

## 6. Frontend

Source: `hexdek/` — React + Vite with custom brutalist design system.

- **Splash**: deck import, live stats
- **Dashboard**: deck library, Freya analysis, mana curves
- **Spectator**: live game view with turn bar, card reveals, combat animations
- **Leaderboard**: bracket-filterable ELO rankings with band labels
- **Deck drilldown**: per-card roles, matchup data, win lines

Communication: WebSocket for live spectating + ELO updates, REST for everything else.

---

## 7. Infrastructure

| Binary | Purpose |
|---|---|
| `hexdek-server` | HTTP/WebSocket API, grinder, matchmaking |
| `hexdek-thor` | Bulk tournament runner |
| `hexdek-freya` | Deck strategy analyzer |
| `hexdek-loki` | Chaos/edge-case fuzzer |
| `hexdek-heimdall` | Analytics tracker |
| `hexdek-muninn` | Crash/gap telemetry |
| `hexdek-judge` | Interactive rules REPL |
| `hexdek-import` | Bulk deck importer |
| `hexdek-tournament` | Full tournament orchestrator |
| `hexdek-parity` | Cross-engine parity checker |
| `gen-handlers` | AST-driven handler code generator |

Production deployment: DARKSTAR (Ryzen 9 9950X, 64GB) runs the engine. MISTY hosts the frontend via Caddy.

---

## Relevant files

| Path | Purpose |
|---|---|
| `scripts/parser.py` | Oracle text → CardAST |
| `scripts/mtg_ast.py` | Typed frozen-dataclass AST schema |
| `scripts/extensions/` | ~50 grammar extension modules |
| `internal/gameengine/` | Go game engine (production) |
| `internal/hat/` | AI system (Yggdrasil, Curse, neural eval) |
| `internal/tournament/` | Tournament runner, round-robin, swiss |
| `internal/heimdall/` | Observation routing, seed buffering |
| `internal/muninn/` | Persistent gap/crash memory |
| `internal/hexapi/` | REST + WebSocket API |
| `cmd/` | 14 CLI entry points |
| `hexdek/` | React + Vite frontend |
| `docs/architecture/` | 39 architecture documents |
| `data/decks/` | Deck files (owner/deck.json) |
| `data/rules/` | Scryfall data, comp rules, coverage reports |
| `data/training/` | Neural evaluator training data + model |
| `data/curse/` | Per-deck Curse DNA pools |
