# HexDek

An open-source Magic: The Gathering Commander engine. Typed AST compiled from every printed card, political AI that plays multiplayer like a human, and a live tournament forge that runs thousands of games per minute.

**[hexdek.dev](https://hexdek.dev)** · [Discord](https://discord.gg/Mz2ueRFXds) · [Donate](https://hexdek.dev/donations) · [Bug Report](https://hexdek.dev/feedback) · MIT License

---

## What HexDek does

HexDek parses every printed Magic card into a typed abstract syntax tree, then runs full Commander games against that AST — complete with the stack, priority, combat, triggers, replacement effects, and state-based actions. An AI system called **Yggdrasil** plays each deck with political awareness: threat assessment, grudge tracking, alliance evaluation, and budget-controlled search depth.

The engine powers a live tournament forge that simulates tens of thousands of games, producing ELO/TrueSkill ratings, win-line analysis, mana curve diagnostics, and matchup data for every deck in the pool.

### Key numbers

| Metric | Value |
|--------|-------|
| Cards parsed | **50,000+** (100% of Scryfall bulk, zero parse errors) |
| Engine cleanliness | **0 invariant violations / 0 crashes** across 100,000 canonical-seed games (r60 final) |
| Engine throughput | **500+ games/sec** on a single machine |
| Rating system | TrueSkill (Bayesian μ/σ) + **CompositionPrior** (+1.4pp accuracy, +0.036 log-loss) |
| AI | YggdrasilHat — 20-dim evaluator, **22 archetype-tuned weight profiles**, self-trigger response matrix |
| Deck coaching | Freya — **S/A/B/C/D power tiers**, B5 cEDH bracket gate, pet-card detection, synergy clusters |
| Format | Commander (4-player pods), 1v1, Archenemy |

For the deeper picture see **[ARCHITECTURE.md](docs/ARCHITECTURE.md)** — a 30-minute read covering engine layers, Hat MCTS budgeting, Freya's analysis pipeline, Heimdall analytics, tournament runner, and the hexapi+ws stack.

### R60 highlights

The R60 cycle (2026-04 → 2026-05) closed a multi-week stress-discovery push and shipped several user-facing systems. Full audit trail in [release-notes-r60.md](docs/release-notes-r60.md).

- **Engine officially clean** — 12 lifecycle/invariant/per_card fixes shipped during the cycle. 100,000 canonical-final games = 0 violations, 0 crashes.
- **AI archetype-aware** — every one of the 22 Freya-classified archetypes gets a custom-tuned weight profile (Aggro anchors LifeResource, Control anchors CardAdvantage + StackInteraction, Burn anchors LifeResource + ThreatExposure, etc.).
- **TrueSkill composition-aware** — the new `CompositionPrior` reads Freya's archetype matchup matrix and folds matchup expectation into rating updates. Validated at +1.4pp accuracy / +0.036 log-loss with Wilson 95% confidence intervals.
- **Freya deck coaching** — absolute S/A/B/C/D tier bands per card with WHY explanations, B5 cEDH detection with free-interaction gate, pet-card flagging for off-archetype flavor picks, synergy clusters with chain-depth scoring, Commander Spellbook import.
- **Decks app archetype loop** — Freya auto-tags decks, user confirms or corrects in the UI, corrections feed back to retrain Freya.
- **Heimdall GameSummary** — per-game summary infrastructure with composition-effect surfacing, archive view, share-link infrastructure.
- **Parser corpus** — 4-era audit fully scaffolded; condition gap down from 76% (era 3 peak) to 14.8% global; era 2 at 0%.

---

## Use it

**[hexdek.dev](https://hexdek.dev)** — import your decks, watch live simulations, and dig into the analytics. No install required.

The source is open for reading, learning, and auditing. You *can* build and run it locally — it's standard Go and Node — but we ship features daily and the engine moves fast. If you're forking to tinker, expect to rebase often.

---

## Architecture

```
Oracle Text → Parser → Typed AST → Game Engine → Tournament Runner → Analytics
                                        ↑
                                   YggdrasilHat (political AI)
                                        ↓
                              TrueSkill + CompositionPrior
```

The engine is written in Go for performance. The frontend is React + Vite with a custom brutalist design system. Communication happens over WebSocket for live spectating and REST for everything else.

**Detailed architecture** lives in [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md). Per-system deep dives in `docs/architecture/`.

### Core systems

| System | What it does | Docs |
|--------|-------------|------|
| **Parser** | Compiles oracle text into typed AST nodes per CR §113 | [Card AST and Parser](docs/architecture/Card%20AST%20and%20Parser.md) |
| **Game Engine** | Full Commander rules: stack, priority, combat, SBAs, triggers | [Engine Architecture](docs/architecture/Engine%20Architecture.md) |
| **Zone System** | Universal MoveCard with replacement effect interception | [Zone Changes](docs/architecture/Zone%20Changes.md) |
| **Layer System** | §613 continuous effects resolution (layers 1–7e) | [Layer System](docs/architecture/Layer%20System.md) |
| **Stack & Priority** | APNAP ordering, split-second, modal spells | [Stack and Priority](docs/architecture/Stack%20and%20Priority.md) |
| **Trigger Dispatch** | Zone-change, ETB/LTB, cast, damage, state triggers | [Trigger Dispatch](docs/architecture/Trigger%20Dispatch.md) |
| **Mana System** | 6-type pool, hybrid/phyrexian, convoke, treasure | [Mana System](docs/architecture/Mana%20System.md) |
| **Combat** | Full combat phases, first/double strike, trample, menace | [Combat Phases](docs/architecture/Combat%20Phases.md) |
| **Replacement Effects** | Self-replacement, interaction ordering, shield counters | [Replacement Effects](docs/architecture/Replacement%20Effects.md) |
| **State-Based Actions** | Lethal damage, 0 toughness, legend rule, +1/+1 and -1/-1 | [State-Based Actions](docs/architecture/State-Based%20Actions.md) |

### AI system

| Component | What it does | Docs |
|-----------|-------------|------|
| **YggdrasilHat** | Political AI: 20-dim board evaluator, threat scoring, grudge memory, budget system | [YggdrasilHat](docs/architecture/YggdrasilHat.md) |
| **22 Archetype Profiles** | Per-archetype eval weight tuning (Aggro / Control / Burn / Combo / 18 more) | [Eval Weights](docs/architecture/Eval%20Weights%20and%20Archetypes.md) |
| **Self-Trigger Matrix** | Counter own trigger when self-harm would be lethal (mill / life / damage / commander damage) | — |
| **Hat State Machine** | 6 game plans (Develop/Assemble/Execute/Disrupt/Pivot/Defend) with transition rules | [Hat State Machine](docs/architecture/Hat%20State%20Machine.md) |
| **Combo Sequencer** | SAT constraint solver for multi-card combo execution | [Hat State Machine](docs/architecture/Hat%20State%20Machine.md) |
| **Genetic Curse** | Per-deck DNA evolution — 7-param genome, population of 8, persisted to disk | [Genetic Curse](docs/architecture/Genetic%20Curse.md) |
| **IS-MCTS** | Information-Set Monte Carlo tree search with determinization | [MCTS and Yggdrasil](docs/architecture/MCTS%20and%20Yggdrasil.md) |
| **Shannon Entropy** | Opponent hand probability model, LikelyHasAnswer heuristic | [Shannon Entropy](docs/architecture/Shannon%20Entropy.md) |
| **Neural Evaluator** | 92-dim MLP position evaluator, trained via self-play | [Neural Evaluator](docs/architecture/Neural%20Evaluator.md) |
| **Self-Play Loop** | Automated training: sample collection → PyTorch → hot-reload | [Self-Play Loop](docs/architecture/Self-Play%20Loop.md) |

### Rating & matchmaking

| Component | What it does | Docs |
|-----------|-------------|------|
| **TrueSkill** | Bayesian μ/σ ratings, 4-player FFA + 1v1 support, drift detection | [Tool - Thor](docs/architecture/Tool%20-%20Thor.md) |
| **CompositionPrior** | Reads Freya's matchup matrix, folds composition expectation into rating updates (+1.4pp acc) | — |
| **Matchmaking** | Rating-aware pod assembly with similarity-based pairing | — |

### Analytics & tools

| Tool | What it does | Docs |
|------|-------------|------|
| **Thor** | Bulk tournament runner, ELO/TrueSkill ratings | [Tool - Thor](docs/architecture/Tool%20-%20Thor.md) |
| **Odin** | Oracle text analyzer / pattern search | [Tool - Odin](docs/architecture/Tool%20-%20Odin.md) |
| **Freya** | Deck strategy analyzer — archetype, bracket, S/A/B/C/D tiers, clusters, win lines | [Tool - Freya](docs/architecture/Tool%20-%20Freya.md) |
| **Heimdall** | Post-game analytics, ELO tracking, GameSummary, archive, share-links | [Tool - Heimdall](docs/architecture/Tool%20-%20Heimdall.md) |
| **Loki** | Fuzzer, edge-case discovery, chaos + nightmare board fuzzing | [Tool - Loki](docs/architecture/Tool%20-%20Loki.md) |
| **Valkyrie** | Deck effectiveness ranker via tournament | [Tool - Valkyrie](docs/architecture/Tool%20-%20Valkyrie.md) |
| **Huginn** | Emergent interaction discovery (parser gap → handler graduation) | [Tool - Huginn](docs/architecture/Tool%20-%20Huginn.md) |
| **Muninn** | Persistent crash/gap telemetry (append-only memory) | [Tool - Muninn](docs/architecture/Tool%20-%20Muninn.md) |
| **Composition Replay** | Offline what-if CLI: re-rate games without the composition prior, show the delta | — |
| **Feynman Oracle** | Post-game invariant checker (zone accounting, SBA compliance, winner validation) | [Feynman Oracle](docs/architecture/Feynman%20Oracle.md) |
| **Tesla** | Causal pivot extraction — identifies the decisive turn/action per game | [Tesla Causal Pivots](docs/architecture/Tesla%20Causal%20Pivots.md) |
| **Ive** | Three-act narrative spectator (setup/conflict/resolution from causal pivots) | [Ive Spectator](docs/architecture/Ive%20Spectator.md) |

Full tool reference: **[Tool Suite](docs/architecture/Tool%20Suite.md)**

---

## Repository layout

```
HexDek/
├── cmd/                          # CLI entry points
│   ├── hexdek-server/            # HTTP/WebSocket API server
│   ├── hexdek-thor/              # AST corpus parser
│   ├── hexdek-freya/             # Deck analyzer (tier system, brackets, coaching)
│   ├── hexdek-odin/              # Oracle text analyzer
│   ├── hexdek-heimdall/          # Post-game analytics, GameSummary, archive
│   ├── hexdek-loki/              # Fuzz tester (chaos games + nightmare boards)
│   ├── hexdek-judge/             # Rules compliance checker
│   ├── hexdek-import/            # Moxfield deck importer
│   ├── hexdek-tournament/        # Full tournament orchestrator
│   ├── hexdek-valkyrie/          # Deck effectiveness ranker
│   ├── hexdek-composition-replay/ # CompositionPrior what-if CLI
│   ├── hexdek-huginn/            # Interaction discovery
│   ├── hexdek-muninn/            # Crash telemetry
│   └── hexdek-parity/            # Cross-engine parity checker
├── internal/                     # Core engine (Go)
│   ├── gameengine/               # Game state, turns, combat, SBAs, replacements
│   │   └── per_card/             # ~600 per-card snowflake handlers
│   ├── gameast/                  # Typed AST node definitions
│   ├── astload/                  # AST corpus loader
│   ├── deckparser/               # Deck file parsing
│   ├── hat/                      # AI system (Yggdrasil, archetype weights, MCTS)
│   ├── tournament/               # Tournament runner, round-robin, swiss
│   ├── trueskill/                # Bayesian ratings + CompositionPrior
│   ├── analytics/                # Heimdall post-game analytics
│   ├── matchmaking/              # Rating-aware pod assembly
│   ├── hexapi/                   # REST API handlers + GameSummary endpoints
│   ├── ws/                       # WebSocket hub (live spectating)
│   ├── hub/                      # WS session management
│   ├── party/                    # Lobby / party system
│   ├── deckid/                   # Content-addressable deck hashing
│   ├── moxfield/                 # Moxfield deck import
│   ├── oracle/                   # Scryfall card lookup
│   ├── db/                       # SQLite persistence
│   └── auth/                     # Auth middleware
├── hexdek/                       # Frontend (React + Vite)
│   └── src/
│       ├── screens/              # Pages (Splash, Spectator, Decks, Queue, Party)
│       ├── components/           # UI components
│       └── services/             # API client
├── docs/
│   ├── ARCHITECTURE.md           # 30-min architecture onboarding
│   ├── release-notes-r60.md      # R60 cycle public release notes
│   ├── architecture/             # Per-system deep dives
│   └── loki-*.md                 # Loki stress-run reports
├── data/
│   ├── decks/                    # Deck files (owner/deck.json)
│   └── rules/                    # Scryfall data, AST corpus
├── scripts/                      # Parser (Python), build scripts
└── tests/                        # Go + Python test suites
```

---

## Get involved

HexDek is open source but opinionated. We handle the development — the engine is deeply Go-native and moves at high velocity with AI-assisted tooling. Outside PRs aren't the bottleneck; good feedback is.

**What helps most:**
- **Bug reports** — [hexdek.dev/feedback](https://hexdek.dev/feedback) or open a GitHub issue
- **Feature requests** — tell us what analysis you want to see for your decks
- **Import your decks** — more replays = better AI. The forge gets smarter with volume
- **Donate** — no ads, no paywalls. Costs are transparent at [hexdek.dev/donations](https://hexdek.dev/donations)

**Want to write code?** If you're a strong Go developer and this system speaks to you, reach out. We'll add you to the Discord, walk through the architecture, and get you set up with our dev process. We don't do drive-by PRs, but we're happy to onboard people who want to go deep.

New contributors: read [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) first. It's a 30-minute tour.

---

## Legal

This project ships no card images and no copyrighted card text. Oracle text is pulled at runtime from Scryfall's bulk-data dump — the same pattern every major MTG tool (Scryfall, Moxfield, EDHREC, Archidekt) operates under. The engine implements the comprehensive rules as software, not as Wizards IP.

Wizards of the Coast, *Magic: The Gathering*, card names, and card artwork are property of Wizards of the Coast LLC. This project is not affiliated with or endorsed by Wizards.

---

## License

MIT. No ads. No paywalls. No premium tiers. Donations only.
