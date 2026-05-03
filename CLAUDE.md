# CLAUDE.md — HexDek

## What is HexDek

HexDek is an open-source MTG (Magic: The Gathering) Commander game engine, AI player, and analysis platform. It simulates 4-player Commander games with AI-driven decision-making, deck analysis, and tournament systems.

- **License:** MIT
- **Language:** Go (engine, AI, tools), React/TypeScript (frontend)
- **Module:** `github.com/hexdek/hexdek`

## Architecture

```
cmd/                    # Executable entry points
  hexdek-server/      # Main API server (WebSocket game engine + REST API)
  hexdek-freya/       # Deck analysis pipeline (archetype, combos, win lines, roles)
  hexdek-heimdall/    # Post-game analytics + spectator tool
  hexdek-thor/        # Card corpus parser (Scryfall → AST)
  hexdek-import/      # Moxfield deck importer
  hexdek-tournament/  # CLI tournament runner
  hexdek-loki/        # Fuzz tester (random games, crash detection)
  hexdek-judge/       # Rules compliance checker
  hexdek-valkyrie/    # Deck effectiveness ranker
  hexdek-odin/        # Oracle text analyzer
  hexdek-parity/      # Cross-engine parity checker
  dump_drift/           # ELO drift reporter

internal/               # Core packages
  gameengine/           # Rules engine (zones, combat, triggers, SBAs, stack)
  gameast/              # Card AST (parsed oracle text → structured abilities)
  hat/                  # AI player ("YggdrasilHat" — MCTS + heuristics + Freya intelligence)
  analytics/            # Heimdall post-game analytics (card rankings, missed combos, stall detection)
  tournament/           # Tournament runner (round-robin, pool, gauntlet)
  hexapi/               # HTTP/WS API layer
  hub/                  # WebSocket hub for live spectator
  party/                # Game lobby / party system
  auth/                 # Authentication
  db/                   # SQLite persistence (ELO, game history)
  deckparser/           # Deck list parser (text → cards)
  moxfield/             # Moxfield API client
  oracle/               # Scryfall oracle corpus loader
  astload/              # AST dataset loader
  rules/                # Comprehensive Rules text parser
  mana/                 # Mana cost/pool algebra
  shuffle/              # Fisher-Yates shuffle
  trueskill/            # TrueSkill rating system
  ai/                   # LLM integration (Ollama)
  deckid/               # Deck identity hashing
  paritycheck/          # Cross-engine parity utilities
  game/                 # Game state serialization
  ws/                   # WebSocket utilities

hexdek/                 # Frontend (React + Vite)
  src/
    screens/            # Spectator, Decks, Queue, Party screens
    components/         # UI components
  public/

data/
  decks/                # Deck lists (text files)
  rules/                # Scryfall oracle data, AST datasets (gitignored, large)
```

## Tool Suite (Norse Mythology Naming)

| Tool | Purpose | Key Flags |
|------|---------|-----------|
| **Thor** | Parses Scryfall oracle corpus → AST dataset | `--oracle`, `--output` |
| **Odin** | Oracle text analysis and search | `--search`, `--pattern` |
| **Freya** | Deck analysis: archetype, combos, roles, win lines, bracket | `--deck`, `--json`, `--strategy` |
| **Loki** | Fuzz testing: random games looking for crashes | `--games`, `--timeout` |
| **Heimdall** | Post-game analytics + spectator | `--replay`, `--stats` |
| **Valkyrie** | Deck effectiveness ranking via tournament | `--decks`, `--games` |

## AI Architecture: YggdrasilHat

The AI player uses a layered evaluation system:

1. **8 Evaluation Dimensions:** BoardPresence, CardAdvantage, ManaAdvantage, LifeResource, ComboProximity, ThreatExposure, CommanderProgress, GraveyardValue
2. **Archetype-Aware Weights:** Freya provides per-deck MCTS weights (e.g., Combo decks boost ComboProximity to 2.0)
3. **Strategy Profile Bridge:** Freya's analysis (combos, roles, finishers, color demand, value engines) feeds directly into hat decisions
4. **3rd Eye Intelligence:** Opponent tracking (cards seen, perceived archetype, threat assessment)
5. **Budget System:** 0=heuristic, 1-199=evaluator-guided, 200+=rollout. Adaptive degradation on complex boards.
6. **Conviction System:** Concession detection based on relative position tracking

## Common Commands

```bash
# Build everything
go build ./...

# Run tests
go test ./internal/gameengine/... -count=1 -timeout 300s
go test ./internal/hat/... -count=1
go test ./internal/tournament/... -count=1

# Run the server (needs oracle-cards.json in data/rules/)
go run ./cmd/hexdek-server/

# Analyze a deck
go run ./cmd/hexdek-freya/ --deck data/decks/mydeck.txt

# Run a tournament
go run ./cmd/hexdek-tournament/ --decks data/decks/ --games 100

# Cross-compile for DARKSTAR (Linux deployment)
GOOS=linux GOARCH=amd64 go build -o hexdek-server-linux ./cmd/hexdek-server/

# Frontend dev
cd hexdek && npm install && npm run dev

# Frontend build
cd hexdek && VITE_API_URL="" npx vite build
```

## Deployment

- **Engine runs on DARKSTAR** (192.168.1.207:8090) — Ubuntu Linux, Ryzen 9
- **Frontend on MISTY** (192.168.1.200) — behind Caddy at hexdek.dev
- **Deploy script:** `./scripts/deploy.sh [backend|frontend|both]`
- **Manual deploy server:** cross-compile → scp to DARKSTAR → `~/hexdek/start-hexdek.sh` (port 8090)
- **Manual deploy frontend:** `cd hexdek && npm run build` → `rsync --delete hexdek/dist/ josh@...200:~/sites/hexdek/`
- **IMPORTANT:** Frontend source is `hexdek/` (Vite React). Do NOT deploy from any other directory.
- Requires WireGuard VPN when remote: `sudo wg-quick up ~/.config/wireguard/admin-vpn.conf`

## Data Files

Large data files are gitignored and must be fetched locally:
- `data/rules/oracle-cards.json` (163MB) — Scryfall bulk oracle data
- `data/rules/ast_dataset.jsonl` (46MB) — Thor's parsed AST output
- `data/rules/finetune_pairs.jsonl` (56MB) — Training data

Fetch oracle data: `scripts/fetch-oracle.sh`

## Freya Improvement Kanban

### Ready
- [ ] Fix triple combo cycle ordering (only tests 2/6 orderings — misses valid 3-card combos)

### Done (2026-04-29)
- [x] Fix combo false positives (self-exile, hand vs battlefield, attack-trigger dependency, randomness)
- [x] Wire 20 `archetypes.go` definitions into classifier (11 new fingerprints + context signals + gameplan descriptions)
- [x] Add value chain templates for Storm, Artifact, Enchantress, Counters Matter engines
- [x] Improve Frank Karsten land formula (ramp/draw discount with dork fragility weight, 28-land floor)
- [x] Fix bracket estimation to exclude banned cards from scoring
- [x] Track colorless `{C}` mana production in land analysis
- [x] Refine card role multi-tag priority ordering (strategic importance sort)
- [x] Eval weight profiles for all 22 archetypes (up from 5 generic profiles)
- [x] Mana curve shape analysis (bimodal detection, top-heavy/bottom-light warnings)
- [x] Expand KnownCombos database (58 entries, +16: Worldgorger, Nim Deathmantle, Breach, Tooth&Nail, Karmic/Reveillark, persist combos)
- [x] Commander synergy scoring (theme extraction from oracle text, 14 theme patterns, synergy % in deck profile)
- [x] Interaction quality scoring (avg CMC of interaction, cheap vs expensive breakdown)
- [x] Recursion depth scoring for value chains (infinite/deep/shallow/none loop-back detection)
- [x] Protection density analysis (built-in protection tracking for combo/threat pieces)
- [x] Mana base grading (tapland/fetch/utility scoring, A-F grade, upgrade suggestions)
- [x] Threat assessment profile (26-entry hoser database, condition-matched vulnerability report)
- [x] Opening hand simulation (10K Monte Carlo trials, keepable % and avg turn to 4 mana)
- [x] Synergy clusters (theme-grouped card packages with pairwise synergy scoring)
- [x] Meta positioning (archetype-vs-archetype matchup predictions with reasoning)
- [x] Card quality tiers (star/cuttable identification from role overlap, win line presence, CMC efficiency)
- [x] Color weight optimization (demand vs supply analysis, specific land swap suggestions)
- [x] Deck personality blurb (archetype-aware 2-3 sentence flavor description)
- [x] Power percentile within archetype (multi-factor scoring: tutors, mana base, interaction, draw, curve, hands)

### Backlog
- [ ] 4-card+ combo detection (currently capped at triples)
- [ ] Tutor inference for modal spells and complex wording
- [ ] Commander Spellbook integration (external combo DB import)
- [ ] NLP-grade oracle text parsing (replace substring matching for edge cases)
