# Go Port Plan — mtgsquad Rules Engine

> **Author:** Survey pass, 2026-04-15
> **Goal:** Achieve the Python reference engine's rules fidelity in Go, reusing the existing Go infrastructure, with a 50k games/sec benchmark target.
> **Source of truth:** `scripts/playloop.py` (~8,133 LOC) + `scripts/mtg_ast.py` (~718 LOC) + `scripts/parser.py` (~2,627 LOC) + `scripts/extensions/policies/*.py` (~1,629 LOC).

---

## Survey findings (Phase 1 answers)

### Q1 — What does the existing Go engine cover vs Python?

| Feature | Python (`playloop.py`) | Go (`internal/game/*.go`) |
|---|---|---|
| §500 phase/step model (5 phases, 11 sub-steps) | Full — `set_phase_step`, `phase_kind`, `step_kind`, `priority_round` | Flat 8-element `PhaseOrder` enum only; no sub-steps |
| §117 priority + §608 stack resolution | Full — `StackItem`, `resolve_effect`, priority loop, split-second, counterspells | **NONE** — `CastSpell` moves cards straight to battlefield / graveyard, no stack |
| §614 replacement-effect framework | Full — `fire_event`, `ReplacementEffect`, §614.5 applied-ids tracking, §616.1 sub-category ordering, APNAP tiebreak | **NONE** |
| §613 continuous effects (layers 1-7e) | Full — `ContinuousEffect`, `Modifier`, `get_effective_characteristics`, `_char_cache`, layer system smoke test | **NONE** |
| §704 SBAs (all 24 sub-rules 5a-5z) | Full — `_sba_704_5a` through `_sba_704_5z` + §704.6c/d commander SBAs | **NONE** |
| §508/509/510 combat (declare, block, damage, first strike, deathtouch, trample) | Full — `combat_phase`, `_deal_combat_damage_step`, first-strike/regular split | Partial — `DeclareAttackers`/`DeclareBlockers`/damage assignment, but no first strike, no trample overflow, no deathtouch, no combat-damage triggers |
| §903 Commander format (tax §903.8, 21-damage §704.6c, zone return §903.9) | Full — `setup_commander_game`, `_sba_704_6c/d`, `fire_zone_change`, tax tracking | **NONE** (ignores commander mechanically; command zone is purely a storage zone) |
| §800.4 leave-the-game cleanup | Full — `_handle_seat_elimination`, OWNER vs CONTROLLER distinction | **NONE** — no elimination logic, game just runs turns |
| N-seat multiplayer (APNAP, "each opponent", etc.) | Full — `apnap_order`, `living_opponents`, `opponents(seat_idx)`, fan-out on each-player effects | Partial — supports 2+ seats in data schema, but combat targets seat integers, no APNAP ordering |
| `resolve_effect` dispatch table | **32 effect kinds** (damage, draw, discard, mill, destroy, exile, bounce, gain_life, lose_life, buff, create_token, add_mana, tutor, reanimate, recurse, scry, surveil, reveal, look_at, shuffle, counter_spell, tap, untap, gain_control, counter_mod, prevent, extra_turn, win_game, lose_game, sequence, choice, optional, conditional, unknown) | **NONE** — Go has no AST interpreter; card text is never read |
| PlayerPolicy abstraction | `seat.policy.method(...)` at every decision site; GreedyPolicy + PokerPolicy implemented | Dead-simple `internal/ai/autopilot.go` that only calls `AdvancePhase` — "no card play, no combat decisions" per its own comment |
| Scaling amounts (`ScalingAmount`) | Full — `resolve_amount` handles devotion, creatures_you_control, cards_in_zone, x, counters_on_self, etc. | **NONE** |
| Per-card snowflake dispatch (1079 handlers) | Full — `_dispatch_per_card` lookup via `extensions/per_card_runtime.py` | **NONE** |
| §613.7 timestamps (Legend Rule, layer ordering) | Full — `_timestamp_counter`, `next_timestamp()` on Permanent | **NONE** |
| §603.7 delayed triggers | Full — `DelayedTrigger`, `_fire_delayed_triggers`, phase-boundary drain | **NONE** |
| §514.2 "until end of turn" durations | Full — `Modifier`, `scan_expired_durations`, `_revert_modifier` | **NONE** |
| Mana system | Generic-only MVP (single `mana_pool: int`), plus `fill_mana_pool` heuristic | Full 6-color pool (WUBRGC), `mana.Parse`, `Pool.CanPay`/`Pool.Pay` — **better than Python here** |

**Summary:** Go's `internal/game/` is a **"state-tracker / referee assistant"** (per the top-of-file comment on `types.go`: *"State-tracking, not a rules engine: players announce intents via the WebSocket API, and the server records the resulting state changes."*). It's a fine shell for human multiplayer where humans adjudicate rules, but it is **NOT** a rules engine. The Python engine is the rules engine.

### Q2 — Reusable vs replaceable

**Keep (plumbing — no game-rule logic):**
- `internal/auth/` — session/JWT auth middleware
- `internal/db/` — SQLite setup, schema, UUID helpers, deck CRUD
- `internal/hub/` — WebSocket connection hub (party-keyed broadcast)
- `internal/ws/` — WebSocket envelope/transport (ping, chat, state_update routing)
- `internal/party/` — HTTP endpoints for device/party/deck/game-start (LOBBY layer)
- `internal/moxfield/` — deck-format parsing (Moxfield JSON, text lists, Scryfall resolver)
- `internal/oracle/` — Scryfall card-data fetch/cache
- `internal/shuffle/` — crypto/rand Fisher-Yates (**keep — Python uses `random.Random` which is weaker for trustless games**)
- `internal/mana/` — WUBRGC mana pool + parser (**better than Python's MVP; keep and use as the Go engine's mana layer**)
- `cmd/mtgsquad-server/main.go` — server entry point

**Replace (game logic — currently a thin state-tracker; needs rules engine):**
- `internal/game/engine.go` — core game ops (StartGame, DrawCards, PlayLand, CastSpell, YurikoReveal, etc.) — move to `internal/gameengine/` as a new package, keep old `internal/game/` as the state-tracker shim so the live websocket server still works
- `internal/game/combat.go` — combat declarations
- `internal/game/types.go` — card/player/zone types
- `internal/ai/autopilot.go` — phase-only autopilot; replace with PolicyRunner

**Add (brand-new packages):**
- `internal/gameast/` — typed AST types mirroring `mtg_ast.py`
- `internal/astload/` — loader for `ast_dataset.jsonl`
- `internal/gameengine/` — the ported rules engine (stack, priority, SBAs, §613, §614, resolve_effect, combat, commander, N-seat)
- `internal/policy/` — PlayerPolicy interface + GreedyPolicy + PokerPolicy
- `internal/tournament/` — goroutine-parallel tournament runner
- `internal/paritycheck/` — Python↔Go event-stream differ
- `cmd/mtgsquad-sim/` — headless simulator binary (not the websocket server)

### Q3 — Data formats

Python exports `ast_dataset.jsonl` (~40 MB, one JSON object per card) via `export_ast_dataset.py`. Every AST node carries an `__ast_type__` field naming the Python dataclass (`"CardAST"`, `"Static"`, `"Damage"`, `"Sequence"`, etc.), so Go can discriminate without re-parsing. Numeric fields (`amount`, `count`) may hold either an `int`, a string (`"x"`, `"var"`), or a nested `{"__ast_type__": "ScalingAmount", ...}` object. Tuples serialize as JSON arrays.

**Go does NOT currently read this file.** Go deck JSON (`data/decks/yuriko_v1.json`) has its own Moxfield-shaped schema — card name, mana_cost, cmc, types, subtypes — but no oracle text and no AST. For the port:

- **MVP:** Go loads `ast_dataset.jsonl` at boot and keeps it in an in-memory `map[string]*CardAST` (keyed by card name). ~40 MB of JSON → maybe 60-80 MB of Go structs, acceptable for a server.
- **Later:** Precompile to flatbuffer / MessagePack / protobuf for faster cold-start, but NOT for MVP.
- **No re-parsing in Go.** Porting the Python parser (~2,627 LOC) is out of scope for the first wave. The AST dataset is the contract.

### Q4 — Where does the Go AI pilot sit?

`internal/ai/autopilot.go` is NOT comparable to Python's GreedyPolicy. It's a 129-line "phase advancer": whenever an AI-controlled seat has active priority, call `AdvancePhase`. No card plays, no casts, no combat. The top comment says: *"This is intentionally not 'smart' — no card play, no combat decisions. Future work: add a behavior policy..."*. It IS the hook where a real policy plugs in — but the policy interface itself doesn't exist yet.

The Python `PlayerPolicy` protocol (14 methods: `choose_mulligan`, `choose_land_to_play`, `choose_cast_from_hand`, `choose_activation`, `declare_attackers`, `declare_attack_target`, `declare_blockers`, `choose_target`, `respond_to_stack_item`, `choose_mode`, `order_replacements`, `choose_discard`, `choose_distribution`, `observe_event`) is the interface Go needs.

---

## Part A — What to preserve (as-is)

| Package | LOC | Why it stays |
|---|---|---|
| `internal/auth/` | ~300 | Session auth is orthogonal to game rules |
| `internal/db/` | ~500 | SQLite CRUD for deck/device/party |
| `internal/hub/` | 146 | WebSocket registry, no game logic |
| `internal/ws/` | 593 | Wire envelope, no rules logic (only routes to `internal/game/*` methods) |
| `internal/party/` | 633 | HTTP endpoints for lobby; doesn't touch rules |
| `internal/moxfield/` | ~400 | Deck format parsing |
| `internal/oracle/` | ~400 | Scryfall card-data cache |
| `internal/shuffle/` | ~100 | Crypto-grade Fisher-Yates — KEEP AS-IS, better than Python's MT19937 |
| `internal/mana/` | ~400 | WUBRGC mana pool + parser — already 6-color, better than Python's MVP |

**Total preserved: ~3,500 LOC (~55% of the Go codebase).**

The live websocket server keeps running throughout the port. `internal/game/` stays as the state-tracker shim until the new `internal/gameengine/` is ready to replace it piece by piece. That minimizes rework.

## Part B — What to replace

| Package | Current LOC | Action |
|---|---|---|
| `internal/game/engine.go` | 719 | Rewrite as thin adapter that calls `internal/gameengine/` |
| `internal/game/combat.go` | 327 | Replace with `internal/gameengine/combat.go` port |
| `internal/game/types.go` | 137 | Keep for the websocket state shim; engine types live in `internal/gameengine/types.go` (richer) |
| `internal/game/storage.go` | 295 | KEEP — used by ws layer for state persistence; engine is in-memory and separate |
| `internal/ai/autopilot.go` | 129 | Replace with `internal/policy/runner.go` + `internal/policy/greedy.go` + `internal/policy/poker.go` |

**Total replaced: ~1,511 LOC → grows to ~8-10K LOC in the new engine.**

## Part C — What's missing (Python has, Go doesn't)

1. **§614 replacement-effect framework** — `fire_event`, `ReplacementEffect`, `EventType` enum, applied-ids tracking, §616.1 sub-category ordering (self_replacement < control_etb < copy_etb < back_face_up < other), APNAP tiebreak, depth-cap safety net.
2. **§613 continuous effects** — `ContinuousEffect`, `Modifier`, layer system (1, 2, 3, 4, 5, 6, 7a, 7b, 7c, 7d, 7e), `get_effective_characteristics` + cache, §613.7a timestamps, §613.8 dependency resolution.
3. **§704 state-based actions** — 24 sub-rules (5a-5z), loop until no more changes, commander SBAs (704.6c, 704.6d).
4. **§608 stack + §117 priority** — `StackItem` struct, priority pass, split-second check, counterspell routing, LIFO resolution.
5. **§500 phase/step granularity** — 5 canonical phases × 11 sub-steps, `set_phase_step` atomic update, phase-boundary hooks for duration expiry + delayed triggers.
6. **§506 combat sub-phases** — begin_of_combat, declare_attackers, declare_blockers, combat_damage (with first-strike split), end_of_combat.
7. **§903 Commander format** — starting life=40, commander tax, 21-damage SBA, zone-change replacement (§903.9a), partner-architected commander_names list.
8. **§800.4 leave-the-game** — eliminate seat, exile controlled stack items, uproot owned permanents, fire seat_eliminated event.
9. **PlayerPolicy abstraction** — 14-method interface, decision-site plumbing, observe_event hook.
10. **ScalingAmount resolution** — `resolve_amount` kinds: devotion, creatures_you_control, permanents_you_control, cards_in_zone, life_lost_this_way, life_gained_this_turn, counters_on_self, x, literal, raw.
11. **Targeting pipeline** — `Filter` evaluation (you_control, opponent_controls, nontoken, creature_types, color_filter, mana_value_op, etc.), `pick_target`, threat-score ranking.
12. **Per-card snowflake dispatch** — 1,079 per-card runtime handlers. The Go engine MUST either:
    - **Option A (MVP):** Load Python-authored snowflake slugs from `ast_dataset.jsonl` and skip execution — cards with snowflake-only rules will behave as "vanilla" (accurate for ~80% of the card pool but wrong for the 20% that rely on snowflakes).
    - **Option B (post-MVP):** Port per-card handlers. 1,079 × ~50 LOC each ≈ 54,000 LOC. This is the long tail and will fill in over years.
13. **Event stream (`game.ev(type_, **fields)`)** — structured Lasagna-schema event emitter. Required for the parity checker (Part F).

## Part D — Data bridge design

**Decision: MVP loads `ast_dataset.jsonl` once at process start.**

```
data/rules/ast_dataset.jsonl      ← Python exports this (40 MB)
      │
      │  Go loads at boot (~1-2s cold start)
      ▼
internal/astload/loader.go        ← streaming JSONL decoder
      │
      │  returns map[CardName]*CardAST
      ▼
internal/gameengine/              ← consumes AST during gameplay
```

**Decoder strategy:**
- Use `encoding/json` streaming decoder (`json.Decoder.Decode` per line).
- Discriminate node types via the `__ast_type__` field.
- Custom `UnmarshalJSON` methods on each AST node type implementing a two-pass approach:
  1. Decode into a `map[string]json.RawMessage` to read `__ast_type__`.
  2. Switch on `__ast_type__` and decode into the concrete struct.
- Numeric-or-string-or-ScalingAmount fields (`amount`, `count`) use a `NumberOrRef` union type that decodes to int / string / `*ScalingAmount` based on JSON shape.

**Memory budget:** ~60-80 MB resident after load, acceptable. 32k unique cards × ~2 KB/AST avg = ~64 MB. Re-parsing each game is wasteful; load once, pass `*CardAST` pointers into game state.

**Alternative considered and rejected:** Port `parser.py` to Go. The Python parser is 2,627 LOC of recursive-descent regex-heavy text matching. Porting is expensive and duplicates the source of truth. The Python parser is still the oracle-text compiler; Go is its downstream consumer.

## Part E — Sequential port phases

Each phase is a **single agent's work unit** — one context, one commit (once we start committing). Phases are ordered so each builds on the prior.

| # | Phase | Deliverable | Est. hours | Depends on |
|---|---|---|---|---|
| 1 | **Go AST types** | `internal/gameast/` — Go structs mirroring `mtg_ast.py` dataclasses. All 40+ node kinds. Type-safe discriminated unions via interface. Scaling amount support. | 3-4 | — |
| 2 | **AST loader** | `internal/astload/` — streaming JSONL decoder with polymorphic `__ast_type__` dispatch. Loads `ast_dataset.jsonl` into `map[string]*gameast.CardAST`. Includes smoke test: load all 31,943 rows, verify zero decode errors, spot-check Lightning Bolt / Doubling Season / Yuriko. | 4-6 | #1 |
| 3 | **resolve_effect port** | `internal/gameengine/resolve.go` — Go port of `playloop.resolve_effect` covering all 32 kinds. Includes `resolve_amount`, filter evaluation, target-picking stub that delegates to policy. No §613/§614 yet (effect dispatch, not continuous-effect machinery). | 12-16 | #1 |
| 4 | **Combat phase** | `internal/gameengine/combat.go` — `combat_phase`, declare_attackers, declare_blockers, first-strike split, damage assignment with trample/deathtouch, Yuriko-style combat-damage triggers. | 8-10 | #3 |
| 5 | **Stack + priority** | `internal/gameengine/stack.go` — `StackItem`, `cast_spell`, priority pass, split-second check, counterspell routing, LIFO resolve loop. Integrates with #3 (stack items emit Effect nodes that run through resolve_effect). | 8-10 | #3 |
| 6 | **SBA loop** | `internal/gameengine/sba.go` — port all 24 `_sba_704_5x` handlers plus the outer loop. Includes poison, lethal damage, attempted-draw-from-empty-library, aura/equipment attachment legality, planeswalker loyalty, legend rule, world rule. | 10-14 | #3 |
| 7 | **§614 replacement framework** | `internal/gameengine/replacement.go` — `Event`, `ReplacementEffect`, `fire_event`, applied-ids tracking, §616.1 sub-category ordering, APNAP tiebreak. Plus hardcoded handlers for the baseline set (Rest in Peace, Leyline of the Void, Doubling Season, Hardened Scales, Panharmonicon, Platinum Angel, Anafenza, Alhammarret's Archive, Rhox Faithmender, Laboratory Maniac). | 12-16 | #3, #6 |
| 8 | **§613 layer system** | `internal/gameengine/layers.go` — `ContinuousEffect`, `Modifier`, `get_effective_characteristics` with 7-layer resolution and cache, §613.7a timestamps, dependency handling for the common cases (Humility, Blood Moon, Painter's Servant, Opalescence). Duration tracking and cleanup-step expiry. | 14-18 | #7 |
| 9 | **Multiplayer + commander** | `internal/gameengine/multiplayer.go` + `commander.go` — APNAP ordering, `opponents()`, `living_opponents()`, `living_seats()`, §800.4 leave-the-game cleanup. Commander setup, §903.8 tax, §704.6c 21-damage SBA, §704.6d zone-change return, commander_damage tracking. | 8-10 | #6, #7, #8 |
| 10 | **PlayerPolicy + GreedyPolicy + PokerPolicy** | `internal/policy/` — 14-method Go interface, GreedyPolicy port (~300 LOC), PokerPolicy port with 7-dim threat score + HOLD/CALL/RAISE mode machine (~1,100 LOC). Replaces `internal/ai/autopilot.go`. | 10-14 | #9 |
| 11 | **Tournament runner** | `internal/tournament/` — goroutine-parallel runner. `runtime.NumCPU()` workers, each pulls matchups from a channel, runs games in-process (no SQLite persistence — pure in-memory), emits results. `cmd/mtgsquad-sim/main.go` CLI. | 6-8 | #10 |
| 12 | **Python↔Go parity tester** | `internal/paritycheck/` — given `(seed, deck_a, deck_b)`, run Python and Go engines with identical RNG state, compare event streams. Tolerance: byte-identical on structural events (zone_change, damage_dealt, seat_eliminated); allow cosmetic diffs in log strings. Start with a 100-game corpus; expand. | 10-14 | #11 |
| 13 | **50k games/sec benchmark** | `bench/` — `go test -bench` harness. Start with 2-player Lightning Bolt goldfish (simplest case), scale up to 4-player EDH gauntlet. Profile (`pprof`), optimize allocation hot path (likely: AST node allocation per resolution, map operations in effective-characteristics cache). Target 50k/sec @ 2-player, stretch goal 15k/sec @ 4-player EDH. | 8-12 | #12 |

**Total estimated effort: 113-152 hours** (roughly **3-4 weeks of focused work** at 40 hrs/wk). Phase 12 + 13 are where the benchmark number is proven or adjusted.

## Part F — Parity strategy

**Hypothesis:** same deck + same seed + same policy → same event stream, byte-identical on structural events.

**Setup:**
1. Python engine adds a `--deterministic --seed N` flag that seeds `game.rng = random.Random(N)` and uses ONLY that RNG for all draws, shuffles, coin flips. Already supported.
2. Go engine accepts a `Seed int64` in `gameengine.Config`, threads it through a single `*rand.Rand` instance wired to every randomized decision.
3. Both engines use the SAME policy (GreedyPolicy; later PokerPolicy). The policy must be deterministic given the event stream — no wall-clock, no un-seeded `math/rand`.
4. Both engines emit the same event stream schema:
   ```
   {seq, game_id, turn, phase, phase_kind, step_kind, priority_round, seat, type, ...type-specific fields}
   ```

**Differ:**
- Structural events must be byte-identical in JSON form: `zone_change`, `damage_dealt`, `life_change`, `card_drawn`, `spell_cast`, `permanent_etb`, `permanent_ltb`, `sba_fired`, `replacement_applied`, `seat_eliminated`.
- Log-string events (`"T3 P0 [main1] casts Lightning Bolt"`) allow cosmetic diffs but must be the same COUNT and roughly the same ORDER.
- Timestamp fields normalized to monotonic seq-based values before diff.

**Sample size: 1,000 games per test run.** First parity milestone: pass 100 Yuriko vs Yuriko games. Final: pass all 16 matchups of the 4-deck commander gauntlet × 1,000 games each = 16,000 games per parity run. CI budget: 16,000 games × 0.5s/game = 2.2 hours per parity pass — tolerable for a nightly gate.

**Tolerance knobs:**
- Hash the event-stream's structural projection (drop `log`, keep `type` + `seat` + `turn` + `phase`). Match by hash.
- Allow 0.1% divergence-rate per 1,000 games for "acceptable fuzz" during development; require 100% match at v1 release.

## Part G — Estimated effort per phase

See Part E table. **Total: 113-152 hours.**

| Agent skill tier | Phases they can own |
|---|---|
| Junior Go | Phases 1, 2, 11 (mechanical porting) |
| Mid-level Go with MTG knowledge | Phases 3, 4, 5, 6 (rules interpretation) |
| Senior Go + MTG + systems | Phases 7, 8, 9 (§613/§614/multiplayer — the hard ones) |
| Go + ML/policy | Phase 10 (policy port) |
| Go + testing/benchmarking | Phases 12, 13 (parity + perf) |

## Part H — Risks and gotchas

1. **Lazy evaluation in Python vs explicit types in Go.** Python tolerates `amount: Union[int, str]` and then stuffs `ScalingAmount` in there at runtime. Go needs an explicit discriminated union (`NumberOrRef{Int, Str, Scaling}`). Every place the Python code writes `amount = 3` OR `amount = "x"` OR `amount = ScalingAmount(...)` becomes a tagged-union constructor in Go.
2. **AST schema evolution.** The Python AST ships as `ast_dataset.jsonl` committed to `data/rules/`. If Python adds a new node type (say `Manifest`, `Morph`, `Bestow`), the Go loader MUST: (a) not crash on unknown `__ast_type__`, (b) log a warning, (c) decode to a `*UnknownNode` catch-all. Backward-compat is the Go loader's responsibility.
3. **Concurrency safety.** Python engine uses no threading; game state is a single-threaded Python object. Go engine goes parallel (Phase 11). Each game MUST be a hermetic `*Game` value with no shared state between goroutines; only the deck AST lookup table is shared (read-only after load). This is enforced by making all `*CardAST` pointers immutable post-load.
4. **Per-card snowflake coverage gap.** Python has 1,079 snowflake handlers (`extensions/per_card.py` = 192,648 bytes). MVP Go engine will NOT port these. Any card whose gameplay relies on a snowflake (estimated 20% of the pool) will play as "vanilla with stats". The gauntlet's 4 curated decks were chosen so the snowflakes they rely on are the ones parsed structurally — so the gauntlet should parity-check clean; random commander decks from the wider pool will NOT.
5. **§613 dependency resolution.** §613.8 dependency resolution is the hardest MTG rule to implement correctly. Python `playloop.py` handles the common cases (Humility, Opalescence, Conspiracy) but a judge could construct corner cases that break both engines identically. Acceptable divergence from CR, parity between engines preserved.
6. **Floating-point / map-ordering nondeterminism.** Go's map iteration order is randomized per-run. Any `resolve_effect` path that iterates a map MUST sort the keys first for determinism. Python dict iteration is insertion-order since 3.7 — Go's equivalent is explicit sorted iteration over `[]string` key slices.
7. **Event-stream ordering across goroutines.** The parity checker compares one game's stream to another game's stream (same seed). Within a game, events are serial (single-threaded per game). Between games, goroutines run in parallel. Parity is per-game, not cross-game. This constraint must be enforced in the tournament runner — each game's events stream to a per-game `[]Event` slice, never a shared channel in the hot path.
8. **Python `random.Random` vs Go `math/rand`.** These are DIFFERENT RNGs (MT19937 vs PCG-based). Same seed → different sequence. Parity requires: either (a) Go uses a Go port of MT19937 (`github.com/seehuhn/mt19937` exists), or (b) both engines use the same cryptographic seed-derived stream (chacha20 KDF → identical stream). Pick one early — this is a Phase 11 blocker.
9. **SQLite persistence overhead.** The current `internal/game/engine.go` persists every action via `AppendActionLog`, `CreateGameCard`, `UpdateGamePlayer`. That's SQLite writes on every operation — unusable for 50k games/sec. The new `internal/gameengine/` MUST be purely in-memory. Persistence is an optional adapter applied post-game (flush final state to SQLite) or during human-visible games only.
10. **"Vanilla fallback" for un-handled cards.** Python's engine has `UnknownEffect` that logs and skips. Go needs the same graceful degradation. Every AST node kind the engine doesn't handle must produce a `ev("unknown_effect", ...)` event and move on — never panic.

---

## Recommendation: which phase NEXT

**Phase 1 (Go AST types) is already started by this agent** — see below.

**Next agent should tackle Phase 2 (AST loader).** It's the smallest standalone unit that produces immediate value (proves the Go types round-trip against the Python export), it's the data contract that everything else depends on, and it's well-scoped for a single agent's context window.

After Phase 2, the critical-path to first-gameplay is:
Phase 1 (done) → Phase 2 → Phase 3 (resolve_effect) → Phase 5 (stack) → Phase 6 (SBAs) → Phase 4 (combat) → first vanilla games running in Go.

The order 3 → 5 → 6 → 4 is deliberate: effect resolution comes first (you can't cast a spell until you can resolve it), stack wraps it, SBAs run the game forward, combat is last because it depends on the effect machinery for first-strike / deathtouch / trample triggers.

§613/§614 (Phases 7 + 8) come after because they require the event plumbing from Phase 3 to be in place.

---

## Part I — Phase 1 implementation (completed by this agent)

See `internal/gameast/` for the Go AST type definitions. Files created:

| File | Purpose |
|---|---|
| `internal/gameast/mana.go` | `ManaSymbol`, `ManaCost` |
| `internal/gameast/filter.go` | `Filter` + built-in shorthand constants |
| `internal/gameast/trigger.go` | `Trigger`, `Condition`, `Cost` |
| `internal/gameast/effects.go` | All 40+ effect node types + `Effect` interface + `ScalingAmount` + `NumberOrRef` |
| `internal/gameast/ability.go` | `Static`, `Activated`, `Triggered`, `Keyword`, `Ability` interface, `Modification` |
| `internal/gameast/card.go` | `CardAST` top-level struct, `Signature()` helper |

The types are pure data structures — no methods beyond accessor helpers. The loader (Phase 2) will populate them; the engine (Phase 3+) will read them.
