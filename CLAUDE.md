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

### Done (2026-05-24)
- [x] Commander Spellbook import — `cmd/hexdek-freya/spellbook_import.go` parses the public variants JSON, infers loop type from features (`Win the game` / `Infinite *` → true_infinite), and dedupes against the 58 curated entries via canonical (lowercased + sorted) card-name keys; curated entries always win conflicts (richer outlets/stops). CLI flags `--spellbook <cache>` and `--spellbook-fetch <url>`; cache file gitignored. 11 regressions in `cmd/hexdek-freya/spellbook_import_test.go` (parse, status filter, type inference, key normalization, curated-wins dedupe, real-curated dedupe, intra-import dedupe, missing-cache no-op, round-trip, bad JSON, accessor merge)

### Done (2026-05-23)
- [x] 4-card combo detection — `checkQuadCombo` enumerates all 24 permutations via nested distinct-index loops (structurally generated, not hand-listed); prefiltered to flow-active candidates and capped at 70 to bound runtime at ~3s worst case. Regressions in `cmd/hexdek-freya/quad_cycle_test.go`; runtime tradeoff in `docs/freya-4card-runtime.md`

### Done (2026-05-22)
- [x] Fix triple combo cycle ordering — enumerate all 6 permutations in `checkTripleCombo` (was 2/6); regressions in `cmd/hexdek-freya/triple_cycle_test.go`

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
- [ ] 5-card+ combo detection (would need graph-walk, not brute-force per `docs/freya-4card-runtime.md`)
- [ ] Tutor inference for modal spells and complex wording
- [ ] NLP-grade oracle text parsing (replace substring matching for edge cases)

## Issue Log

> **Rule:** ANY time a test, audit, Goldilocks run, Loki fuzz, or manual investigation surfaces a bug, invariant violation, or unexpected behavior — log it here IMMEDIATELY. No exceptions. Format: date, source, description, status.

### Open

| Date | Source | Issue | Severity | Notes |
|------|--------|-------|----------|-------|
| 2026-05-08 | Corpus Audit | **4,190 unbucketed condition/trigger nodes** across all 4 eras (33.9% of 12,363 total) | Info | Era 1 biggest gap (3,281), Era 3 highest % (76.2%). 19 new scaffold kinds proposed from Era 4 audit |

### Resolved

| Date | Source | Issue | Resolution |
|------|--------|-------|------------|
| 2026-05-24 | Loki r41/r60 | **TriggerCompleteness (8) — trigger batch opened but never drained** (originally suspected as a missing-defer bypass). Root cause was actually two collaborating bugs: (a) the per-card dispatcher in `per_card/registry.go`'s `fireTrigger` increments a process-lifetime `gs.Flags["trigger_total"]` counter and silently short-circuits past 2000. With no per-turn reset the counter saturated by turn 40+ (Jaxis game 109 hit at turn 48 / 4357 events), permanently muting every subsequent dispatch — so creature-dies bearers like Jaxis emitted no `trigger_evaluated` follow-up event and the invariant flagged each remaining death. (b) The invariant itself treated every `sacrifice` event as a creature-dies candidate, so sacrificing a Plains / artifact / enchantment under a controller who happened to own a creature_dies bearer (Sefris, Syr Vondam Sunstar Exemplar) false-positived even when no trigger was expected. **Fix:** (1) `per_card/registry.go` — when the `trigger_depth > 8` or `trigger_total > 2000` guard short-circuits, emit a synthetic `trigger_evaluated` event with `capped="trigger_depth"`/`"trigger_total"` so the invariant's follow-up scan succeeds. (2) `phases.go` cleanup step (line ~324) — `delete(gs.Flags, "trigger_total")` so the counter is the intended per-turn runaway budget rather than an accidental lifetime cap. (3) `zone_change.go` `sacrificePermanentImpl` — add `was_creature: perm.IsCreature()` to the sacrifice event Details. (4) `invariants.go` `checkTriggerCompleteness` — skip `sacrifice` events whose Details say `was_creature=false`. 5 regressions in `internal/gameengine/trigger_cap_r60_test.go` (cleanup reset, non-cleanup persistence, cap-breaks-after-cleanup, invariant skips non-creature sacrifice, invariant still flags creature sacrifice). Loki re-run @ 2000 games / seed 41: TriggerCompleteness 8 → **0** (all 8 cleared in two passes: -2 from the per-turn reset, -6 from the type-aware filter). Goldilocks + full engine suite clean. |
| 2026-05-23 | Loki r41/r57 | **AttachmentConsistency (14→23 then bit-stable at 22 across r53→r57)** — Aura/Equipment `AttachedTo` pointing at a permanent no longer on any battlefield. Three bit-stable signatures: `"Ghoulish Impetus" attached to "creature token black zombie giant Token"`, `"Brilliant Wings" attached to "Tidal Warrior"`, `"Dub" attached to "creature token phyrexian mite Token"`. Root cause: several engine + per_card removal paths call `gs.removePermanent` / `removePermanentFromBattlefield` directly without `detachAll`, leaving auras/equipment with stale `AttachedTo` references in the window before SBA §704.5m/§704.5n run. The window is observed by `checkAttachmentConsistency` after `TakeTurn` returns but before the next `StateBasedActions` pass. **Fix:** (1) added `detachAll` inside `removePermanentFromBattlefield` (covers Craft, Meld, exile-self activation cost, Aura swap); (2) added it to the sweep alt-cost return path in `keywords_sweep.go` and both mutate arms in `keywords_batch6.go`; (3) exported `DetachAll` in `sba.go` for per_card callers; (4) added `gameengine.DetachAll` calls to the per_card flicker/blink/exile-self handlers that route through the package-local `removePermanent` helper: Displacer Kitten, Emiel the Blessed, Deadeye Navigator, Brago King Eternal, Thassa Deep-Dwelling, Yorion Sky Nomad, Etrata the Silencer, Wan Shi Tong, Jadzi Oracle of Arcavios, Bilbo Birthday Celebrant, Lord Xander (era3). Gain-control paths (`resolve.go:1585`, `resolve_helpers.go:4827`, Gilded Drake in `commander_staples.go`) intentionally do NOT detach — control change is not a zone change. 4 regressions in `internal/gameengine/attachment_consistency_r60_test.go` pin the `removePermanentFromBattlefield`, sweep-return, gain-control, and public-`DetachAll` paths. Loki re-run @ 500 games / seed 48 (and seed 41): AttachmentConsistency 22 → **0**, both seeds. Goldilocks: clean (15 ZoneCastGrantExpiry unrelated). |
| 2026-05-23 | Loki r41 | **ZoneCastGrantExpiry (8) — impulse-play / cast-from-exile grant outlived its declared expiry**. r59 already pinned the two `impulse_play` sites (the structured `resolveModificationEffect` arm at `resolve_helpers.go:1506` and the `resolveResidualByText` "you may play that card this turn" branch at `resolve_helpers.go:4760`). The remaining 8 violations came from two sibling arms in `resolveModificationEffect` that built `NewFreeCastFromExilePermission(...)` and left it duration-less: the **`heist`** arm (`resolve_helpers.go:354`, oracle "Until end of turn, you may play that card.") and the **`may_play_exiled_free`** arm (`resolve_helpers.go:550`, the "this turn" family — the parallel "as long as it remains exiled" wording is routed through `resolveResidualByText` and was already pinned). Shared helper `NewFreeCastFromExilePermission` is intentionally duration-less because Opposition Agent, Release to the Wind, and Dauthi Voidwalker all need "as long as exiled" / "while source in play" semantics, so the fix is per-call-site: stamp `Duration = "until_end_of_turn"` + `GrantTurn = gs.Turn` on the returned permission. 3 regressions in `internal/gameengine/zonecast_cast_from_exile_expiry_r60_test.go` pin both grants reclaim at `ExpireZoneCastGrants` cleanup, the `ZoneCastGrantExpiry` invariant stays clean, and the shared helper continues to default to no turn clock. See branch `dev/zonecast-grant-expiry-r60` |
| 2026-05-23 | Loki r44 / game 420 | **ZoneConservation "real cards disappeared"** — 28 hits in game 420 (Breya / Baron Bertram / Alela / SP//dr deck) at turn 10+, seat 3 census reads 4 cards low. Cards going MISSING (not duplicating), distinct from the r41–r43 paradigm-copy and Cerulean Sphinx duplication clusters. | Root cause was an Unholy Indenture self-reanimate driving SBAs past the pass cap, setting `gs.Flags["game_draw"]=1` (CR §104.4b mandatory-loop draw); `CheckEnd` did not react to `game_draw`, so the chaos turn loop pushed a fresh permanent spell onto `gs.Stack`, `ResolveStackTop` short-circuited on the `ended` flag, and the §727 no-op detector eventually fired `projectAndApply` — which truncated the stack via `gs.Stack = gs.Stack[:0]`, silently vanishing the `*Card`. **Fix (r59, commit `c711b1a`):** `evacuateStackSpellsToGraveyard` in `internal/gameengine/loop_shortcut.go` now routes the underlying `*Card` of each non-copy non-ability StackItem to its OWNER's graveyard (per CR §608.2g) before `projectAndApply` truncates `gs.Stack`. Regressions: `internal/gameengine/loki_r59_loop_evacuate_test.go` (4 tests pinning the primitive, owner-vs-controller routing, copy/ability skip, and end-to-end no-op-loop preservation). **Verification (r60, 2026-05-23):** Loki re-run @ 5000 games / seed 41 reports 0 ZoneConservation violations (was 102 in the original cluster, including the 28 game-420 hits); narrow reproduction `--games 421 --seed 41` confirms game 420 clean. Subsumed under the r59 merge `b186f89` (`-102 ZoneConservation`). |
| 2026-05-23 | Forge gauntlet | **TLA flashback-grant unimplemented (Iroh + sweep of similar patterns)** — Iroh's "during your turn, each non-Lesson i/s in your graveyard has flashback; Lessons have flashback {1}" was inert because the AST parser dumped the whole clause into a `phase_scoped_static` raw-text arg. Resolved in two steps: (r59) added the `GraveyardFlashbackGrant` predicate-style continuous-effect primitive (`internal/gameengine/keywords_flashback_grant.go`) registered against `CastFlashback`'s permission pipeline, with timestamp-driven LTB cleanup and an EOT orphan-source sweep — wired Iroh as the first consumer. (r60) one-shot variant `RegisterEOTGraveyardFlashbackGrant` for spells with no battlefield permanent — wired Past in Flames, Will of the Jeskai, the Flashback instant, and Recoup. **r60 TLA sweep** (this fix) ran the same pattern across the AST corpus, found 4 additional mass-grant cards with no per_card handler: Lier, Disciple of the Drowned (continuous always-on), A-Lier (continuous OnlyActiveTurn), Return the Past (Enchantment, continuous OnlyActiveTurn), Backdraft Hellkite (attack-trigger EOT mass grant). All four reuse `PrintedMassFlashbackCost` and the existing primitives — no further engine extensions needed. Lier's ETB+LTB lives in `custom_lier_disciple_of_the_drowned.go`; the other three in `internal/gameengine/per_card/tla_flashback_grants_r60.go`. 12 regressions in `tla_flashback_grants_r60_test.go` (ETB grant shape, active-turn gating where applicable, CastFlashback success, LTB / EOT-sweep cleanup, Backdraft attacker_perm filter). Single-target attack/ETB-triggered grants (Snapcaster, Katilda and Lier, Sphinx of Forgotten Lore, Slickshot Lockpicker, Archmage's Newt, The Fugitive Doctor, Lost in Memories) are a separate systemic pattern — primitive (`GrantFlashbackUntilEOT`) already exists; per_card wiring is a follow-up sweep |
| 2026-05-20 | Loki r47 / game 333 | **God-Eternal Oketra CardIdentity zone-leak (5 of 5 shown details in r47, dominant new-after-Adric cluster)** — card "God-Eternal Oketra" appears in both seat 2 library and seat 2 graveyard. Root cause: the `god_eternal_tuck` arm of `resolveModificationEffect` (`internal/gameengine/resolve_helpers.go:4236`) called `removePermanent(src)` and inserted `src.Card` into the library at index 2. Oketra's "When this dies or is put into exile from anywhere, put it into your owner's library third from the top" fires AFTER the zone change, so by resolve time the card is already in the graveyard (or exile); `removePermanent` no-op'd against the now-off-battlefield Permanent and the card got duplicated into the library. Same anti-pattern as the Dread fix (2026-05-08) and Adric fix (commit `7e782cf`). **Fix:** mirror `shuffle_pronoun_into_owner_library`'s shape — capture `fromZone`, try `removePermanent` first, fall through to `removeFromZone` across each seat's graveyard → exile → hand, then insert at index 2 (clamped to tail when library < 3) and call `FireZoneChangeTriggers` with the real `fromZone`. 5 regressions in `internal/gameengine/god_eternal_tuck_r48_test.go` (battlefield, graveyard, exile, short-library, empty-library paths). Loki re-run @ 5000 games / seed 42: total 380 → **330** (−13%), CardIdentity 346 → **296** (−50, all Oketra hits gone), game 333 cleared. Next CardIdentity offender now Archon's Glory (1 game / 5 details). See commit on `dev/oketra-zone-fix-r48` |
| 2026-05-08 | Freya | **Commander-centric hand evaluation gap**: keepable hand heuristic didn't account for decks where the gameplan IS the commander (Voltron, high-synergy engine commanders) | Two-part fix in `cmd/hexdek-freya/advanced.go`. (1) Rewrote `computeOpeningHandSim` to model the deck with per-slot category flags (land/ramp/synergy/action) padded to `report.TotalCards` so basic lands actually appear in hand — the old simulation only sampled nonbasic profiles, depressing every keepable %. Standard "keepable" now also requires an actual action card (threat/removal/draw/combo/tutor/wipe/counter) instead of merely "any non-land". (2) Added `detectCommanderCentric` (Voltron archetype, ≥45% commander synergy, or commander oracle text containing ≥2 engine phrases) and a parallel `KeepableHandPctAdjusted` that accepts hands satisfied by ramp, a commander-synergy enabler, or natural land drops to commander CMC. Reports both metrics + avg turn to commander; exposed via deckprofile JSON and strategy JSON for hat consumption. Voltron Uril now reads 63% standard / 80% adjusted (was 37%) |
| 2026-05-08 | Hat | **Graveyard recursion intelligence gated behind ArchetypeReanimator**: non-reanimator decks with graveyard synergy (Szarekh, surveil, self-mill) got no strategic graveyard play | Added deck-agnostic `hasGraveyardRecursionValue` (flashback/unearth/escape/disturb/embalm/eternalize/encore/jump-start/aftermath/retrace/dredge) and `hasGraveyardRecursionEnabler` (Sun-Titan/Muldrotha-style) helpers; both ChooseDiscard and ChooseSurveil now bias recursion-bearing cards toward the graveyard regardless of archetype |
| 2026-05-08 | Goldilocks | **Dread — CardIdentity invariant violation**: card appeared in both library and graveyard after death-trigger shuffled it into owner's library | `shuffle_pronoun_into_owner_library` handler in `resolve_helpers.go` only removed the source from the battlefield. For "put into graveyard from anywhere" triggers (Dread, Purity, Guile, Vigor, Worldspine Wurm, etc.) the card was already in the graveyard at resolve time, so the library move left a stale graveyard reference. Handler now falls through to remove the card from graveyard / exile / hand if not on the battlefield, and threads the actual `from_zone` into `FireZoneChangeTriggers`. Verified clean on Dread + 8 related cards |
| 2026-05-08 | Goldilocks | **1,795 keyword_dead failures**: combat keywords scaffolded but no observable change | Two bugs in `cmd/hexdek-thor/goldilocks.go`. (1) `makeKeywordGameState` left `RetainEvents` at default false, so every `LogEvent` call (including the default-case `keyword_test` fallback) was dropped to `gs.lastEvent` and never reached `EventLog` — leaving `verifyKeyword`'s event-log probe blind. (2) Combat-relevant keywords (flying, trample, deathtouch, lifelink, first strike, double strike, vigilance, haste, menace, reach, defender, protection, infect, toxic, fear, intimidate, shroud, hexproof, indestructible, ward, horsemanship, flanking, skulk, all *walk variants) had no scaffold at all, so they fell through to a default that placed the card on the battlefield without exercising combat. Added `RetainEvents: true` plus a combat scaffold that puts the source on seat 0 attacking seat 1 with a 2/2 blocker, runs both first-strike and regular `DealCombatDamageStep`, then resets phase + clears combat flags so post-test `CombatLegality` doesn't fire on walls/defenders being tested. Result: keyword_dead 1,795 → 0, total goldilocks failures 1,915 → 54 |
| 2026-05-16 | Muninn | **May 11 grinder flood — 324 nil-deref panics in one run, identical signature**: `abdelAdrianETB → moveCardBetweenZones("battlefield"→"exile") → MoveCard → FireZoneChangeTriggers(perm=nil) → perm.Controller`. Symptom already patched in commit `b348f4a` (2026-05-08) by adding `perm != nil` guard around the LTB `FireCardTrigger`, but the API misuse remained: `abdel_adrian.go` was bypassing §614 would-be-exiled replacements, §903.9b commander redirect, aura `detachAll`, and replacement-effect unregistering. Root-cause fix in `internal/gameengine/per_card/abdel_adrian.go` — exile each pick through `gameengine.ExilePermanent(gs, p, perm)` (the canonical battlefield-exit API), drive token count off the survived-§614 count, drop the manual `removePermanent + moveCardBetweenZones` pair. Full forensic write-up at `docs/may11-nil-deref-forensics.md`, which also flags 6 sibling handlers (etrata, zabaz, zimone+dina, bilbo, thassa) carrying the same anti-pattern — they don't crash post-`b348f4a` but bypass the same machinery and should be swept in a follow-up. Same audit shows the 466 May 5 crashes (`ExtractPivot` at `tesla.go:55`, `winnerSeat=-1`) were already closed by commit `199837e` on 2026-05-05 — left documented to prevent re-investigation |
| 2026-05-19 | Loki r44 / game 404 | **Disruptive Pitmage hand+battlefield duplication (18 of r43's 410 CardIdentity hits)** — same anti-pattern as the Adric leak (commit `7e782cf`): `{T}: Counter target spell unless its controller pays {1}` made the creature look like a hand-castable counter response to `counterSpellEffect`, and `PriorityRound`'s response-cast path didn't remove the card from hand after paying cost. Independently rediscovered while investigating game 404; the Adric r44 fix landed in main during diagnosis and already covered both surfaces. **Additive refinement** on `dev/game-404-fix-r44`: `counterSpellEffect`'s Layout-1 (Activated AST) match now also requires `isEmptyActivationCost(a.Cost)` — defense-in-depth for the hypothetical instant/sorcery with a real-cost activated counter (`isPermanentSpell` already covers Pitmage; the empty-cost gate covers parser-artifact instants whose body is an Activated node, mirroring `collectSpellEffect`'s r41 follow-up gate). Plus 3 regression tests pinning the new gate. Loki re-run with both fixes (seed 41, 5000 games): game 404 18 → **0** violations |
| 2026-05-19 | Loki r43 / Krark zone-conservation | **Paradigm copy resolution leaks `*Card` refs into `gs.ParadigmExile` every tick — 824 ZoneConservation "extra real cards appeared" hits in r41 game 181 (Decorum Dissertation in a Krark, the Thumbless deck, seed 41 / turn 47 / 11 extras)**: Two bugs. (1) `ResolveParadigmCopies` in `internal/gameengine/phases.go` pushed the copy StackItem without `IsCopy: true`, so when the copy resolved as a non-permanent spell `ResolveStackTop` routed the copy `*Card` to the graveyard instead of letting CR §707.10 cease it. (2) The per-card OnResolve handlers for paradigm cards (Decorum Dissertation, Echocasting Symposium, Germination Practicum, Improvisation Capstone, Restoration Seminar — all 5 use the shared `paradigmExileItem` helper in `internal/gameengine/per_card/paradigm_echocasting_symposium.go`) ran a fresh `paradigmExileItem` call on every paradigm copy too, re-appending the copy's `*Card` to `gs.ParadigmExile` and re-flipping `CostMeta["exile_on_resolve"]=true`. `checkZoneConservation` counts `gs.ParadigmExile` contents as real cards, so each tick inflated the total by one. **Fix:** `IsCopy: true` on the copy StackItem so CR §707.10 ceasing applies; `paradigmExileItem` short-circuits when `item.IsCopy` (the original is the only Card paradigm-tracked; copies are transient). Regressions: `internal/gameengine/paradigm_r43_zone_test.go` + `internal/gameengine/per_card/paradigm_r43_test.go` (4 tests pinning both surfaces). Loki re-run: game 181 86 → **0** violations; full 5000-game fuzz: 1,255 → 1,113 (-11% vs r41 follow-up baseline / -33% vs r41 raw). ZoneConservation specifically: 824 → 664 (-19%). See commit on `dev/krark-zone-conservation-r43` |
| 2026-05-19 | Loki r41 follow-up | **Cerulean Sphinx zone-leak (1,622 of 1,652 r41 violations)**: `*Card` pointer duplicating across "owner's library" + "controller's battlefield" / "another seat's library". Two collaborating bugs. (1) `collectSpellEffect` (stack.go:604) returned the first Activated ability's effect as the cast-time spell body for ANY card with an Activated AST node, including permanent spells. Cerulean Sphinx's `{U}: This creature's owner shuffles it into their library` therefore ran at CAST resolution — putting the card into the owner's library — before `resolvePermanentSpellETB` then wrapped the same `*Card` in a fresh Permanent on the controller's battlefield. (2) The synthetic transient `Permanent` built in `ResolveStackTop` (stack.go:1109) and `resolveActivatedAbility` (activation.go:543) for handlers that need a `*Permanent` source omitted the `Owner` field, defaulting to seat 0 — so even when the card was owned by seat 1+, `shuffle_into_owner_library` routed to seat 0's library. Fix in `internal/gameengine/stack.go` + `activation.go`: `collectSpellEffect` now returns nil for permanent spells per CR §112.6/§603.5, and for instants/sorceries only matches `Activated` nodes with an empty cost (the parser-artifact spell-body shape that Summon the School / Divergent Growth / Eldrazi Confluence emit); both synthetic-Permanent sites now thread `Owner: item.Card.Owner`. Regression: `internal/gameengine/loki_r41_followup_test.go` (4 tests pinning both surfaces). Loki re-run: game 137 → 0 violations; full 5000-game fuzz: 1,652 → 1,255 (-24%, all remaining hits are unrelated token/copy and equipment-attachment clusters). See `docs/loki-r41-followup-report.md` |
| 2026-05-17 | Per-card tests | **`TestSai_ArtifactCastCreatesThopter` test pollution — Sai handler permanently stripped by `Reset()`**: bisected to `TestAllRegisteredTriggersAreDispatched` calling `per_card.Reset()`, which rebuilt the global registry from `registerDefaults()` only. Sai was registered via `tribal_lords.go`'s `init()`, which Go runs exactly once per process, so post-Reset the registry permanently lost Sai (plus ~50 other handlers wired via sibling `init()` functions in `obeka_support.go`, `combat_restrictions.go`, `batch17_sweep.go`, and four `zz_*_register.go` files). The `zz_*` files already exposed `Register*` functions but no test was wired to invoke them after Reset. **Fix:** added `AddResetHook(fn)` in `registry.go`; `Reset()` now re-invokes every registered hook on the fresh registry. Each init() that populates the registry now adds a hook (extracted bodies into named `registerXxx` functions where needed). Sibling latent bug surfaced + fixed: `custom_saheeli_radiant_creator.go` registered `OnTrigger("...", "begin_combat", ...)` but the engine fires `combat_begin` — added `"begin_combat": {"combat_begin"}` alias in `event_aliases.go` |
