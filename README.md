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
| Engine throughput | **500+ games/sec** on a single machine |
| Rating system | TrueSkill (Bayesian μ/σ) + standard ELO |
| AI | YggdrasilHat — 8-dimensional evaluator, political multiplayer |
| Format | Commander (4-player pods), 1v1, Archenemy |

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
```

The engine is written in Go for performance. The frontend is React + Vite with a custom brutalist design system. Communication happens over WebSocket for live spectating and REST for everything else.

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
| **YggdrasilHat** | Political AI: 8-dim board evaluator, threat scoring, grudge memory, budget system | [YggdrasilHat](docs/architecture/YggdrasilHat.md) |
| **MCTS** | Monte Carlo tree search with configurable depth and budget | [MCTS and Yggdrasil](docs/architecture/MCTS%20and%20Yggdrasil.md) |
| **Eval Weights** | Per-archetype tuning: aggro, combo, control, midrange, stax | [Eval Weights](docs/architecture/Eval%20Weights%20and%20Archetypes.md) |
| **Greedy Hat** | Fast fallback AI for bulk simulation | [Greedy Hat](docs/architecture/Greedy%20Hat.md) |

### Analytics & tools

| Tool | What it does | Docs |
|------|-------------|------|
| **Thor** | Bulk tournament runner, ELO/TrueSkill ratings | [Tool - Thor](docs/architecture/Tool%20-%20Thor.md) |
| **Odin** | Invariant checker, engine correctness verification | [Tool - Odin](docs/architecture/Tool%20-%20Odin.md) |
| **Freya** | Deck strategy analyzer (curve, ratios, win lines, archetypes) | [Tool - Freya](docs/architecture/Tool%20-%20Freya.md) |
| **Heimdall** | Game analytics, ELO tracking, card performance | [Tool - Heimdall](docs/architecture/Tool%20-%20Heimdall.md) |
| **Loki** | Fuzzer, edge-case discovery, chaos testing | [Tool - Loki](docs/architecture/Tool%20-%20Loki.md) |
| **Valkyrie** | Cross-compile and deploy automation | [Tool - Valkyrie](docs/architecture/Tool%20-%20Valkyrie.md) |
| **Huginn** | Emergent interaction discovery (parser gap → handler graduation) | [Tool Suite](docs/architecture/Tool%20Suite.md) |
| **Muninn** | Persistent crash/gap telemetry (append-only memory) | [Tool Suite](docs/architecture/Tool%20Suite.md) |

Full tool reference: **[Tool Suite](docs/architecture/Tool%20Suite.md)**

---

## Repository layout

```
HexDek/
├── cmd/                          # CLI entry points
│   ├── hexdek-server/            # HTTP/WebSocket API server
│   ├── hexdek-thor/              # Tournament runner
│   ├── hexdek-freya/             # Deck analyzer
│   ├── hexdek-odin/              # Invariant checker
│   ├── hexdek-heimdall/          # Analytics/ELO tracker
│   ├── hexdek-loki/              # Fuzzer
│   ├── hexdek-judge/             # Rules compliance checker
│   ├── hexdek-import/            # Bulk deck importer
│   ├── hexdek-tournament/        # Full tournament orchestrator
│   ├── hexdek-valkyrie/          # Deploy tool
│   ├── hexdek-huginn/            # Interaction discovery
│   ├── hexdek-muninn/            # Crash telemetry
│   └── hexdek-parity/            # Cross-engine parity checker
├── internal/                     # Core engine (Go)
│   ├── gameengine/               # Game state, turns, combat, SBAs
│   ├── astload/                  # AST loader from Scryfall corpus
│   ├── deckparser/               # Deck file parsing
│   ├── hat/                      # AI system (Yggdrasil, Greedy, strategy)
│   ├── tournament/               # Tournament runner, round-robin, swiss
│   ├── analytics/                # Rivalry, threat graph, resource tracking
│   ├── versioning/               # Deck version DAG (content-addressable)
│   ├── matchmaking/              # Rating-aware pod assembly
│   ├── hexapi/                   # REST API handlers
│   ├── ws/                       # WebSocket hub (live spectating)
│   ├── moxfield/                 # Moxfield deck import
│   ├── oracle/                   # Scryfall card lookup
│   ├── db/                       # SQLite game state
│   └── auth/                     # Firebase auth middleware
├── hexdek/                       # Frontend (React + Vite)
│   └── src/
│       ├── screens/              # Pages (Splash, Dashboard, Spectator, etc.)
│       ├── components/           # UI components (chrome design system)
│       ├── hooks/                # useLiveSocket, useAnimatedCounter
│       ├── context/              # AuthContext (Firebase)
│       └── services/             # API client, mock data
├── docs/                         # Documentation
│   └── architecture/             # Engine, AI, tools, systems (you are here)
├── data/
│   ├── decks/                    # Deck files (owner/deck.json)
│   └── rules/                    # Scryfall data, comp rules, coverage reports
├── scripts/                      # Parser (Python), build scripts, extensions
│   └── extensions/               # Per-card handlers, parser extensions
├── tests/                        # Go + Python test suites
└── web/                          # Vanilla JS tools (leaderboard, drilldown)
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

---

## Legal

This project ships no card images and no copyrighted card text. Oracle text is pulled at runtime from Scryfall's bulk-data dump — the same pattern every major MTG tool (Scryfall, Moxfield, EDHREC, Archidekt) operates under. The engine implements the comprehensive rules as software, not as Wizards IP.

Wizards of the Coast, *Magic: The Gathering*, card names, and card artwork are property of Wizards of the Coast LLC. This project is not affiliated with or endorsed by Wizards.

---

## License

MIT. No ads. No paywalls. No premium tiers. Donations only.
