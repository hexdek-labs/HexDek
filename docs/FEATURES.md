# HexDek Features — what the system does today

*A readable, code-grounded tour of HexDek's working surfaces, for a new
contributor or an owner catching up. Each feature is a couple of
sentences (what it is, what it does) with a pointer to the package that
implements it — not exhaustive API docs. Maturity is called out honestly
where it matters. Snapshot: r63 (2026-06-13).*

For the higher-level "why" see [THESIS-SNAPSHOT.md](THESIS-SNAPSHOT.md);
for the data-flow trace of a single deck see
[deck-lifecycle.md](deck-lifecycle.md); for known bugs see
[BUG-LEDGER.md](BUG-LEDGER.md).

---

## 1 · The game engine (`internal/gameengine`)

The rules engine simulates 4-player Commander end-to-end. It is the
largest and most mature surface in the project.

- **Zones & game state** (`state.go`) — every seat carries Library,
  Hand, Graveyard, Exile, Battlefield, and Command Zone; objects move
  through one universal entry point (`zone_move.go` / `zone_change.go`)
  so replacement effects, redirects, and zone-change triggers all fire
  from a single chokepoint.
- **Stack & priority** (`stack.go`) — CR §117 APNAP priority polling,
  §601 casting (`CastSpell`), §608 resolution (`ResolveStackTop`),
  split-second handling. The stack is a real LIFO with per-item IDs and
  copy semantics (§707.10).
- **Combat** (`combat.go`) — the full §506–§510 combat phase: declare
  attackers/blockers, a first-strike damage step plus the regular step,
  first strike / double strike / deathtouch / trample / lifelink, and
  end-of-combat cleanup. The AI's block plan (chump vs. lethal trade)
  lives here too.
- **Triggered abilities** (`triggers.go`, `trigger_batch.go`,
  `attack_trigger_dispatch.go`) — §603.2 put-on-stack, simultaneous-
  trigger batching, and §603.3b APNAP ordering (active player's
  triggers stack first, resolve last). Intervening-if (§603.4),
  reflexive (§603.12), and delayed (§603.7) triggers are implemented.
- **State-based actions** (`sba.go`) — the §704.3 SBA loop (bounded at
  40 passes) covering player loss, zero-toughness/lethal-damage death,
  legend rule, token/copy cessation, aura/equipment legality, plus the
  Commander variants (§704.6c 21-damage loss, §704.6d graveyard/exile
  return).
- **Layer system** (`layers.go`) — the full §613 continuous-effects
  stack: copy → control → text → type → color → ability → power/
  toughness (7a–7d), timestamp ordering, and §613.8 dependency cycle-
  breaking. Counters apply in their own post-layer pass.
- **Replacement effects** (`replacement.go`, `damage_replacement.go`) —
  §614 event wrappers with §616.1 category ordering and the applied-
  once rule, covering the canonical "would" events (draw, gain/lose
  life, damage, counters, tokens, die, leave-game, win/lose).
- **Mana** (`mana.go`) — typed colored pools (WUBRG + colorless + any +
  *restricted* mana with spend-time constraints), §106 empty-on-step-
  boundary draining, and §605 mana-ability detection.
- **Commander rules** (`commander.go`) — command zone, commander tax
  (§903.8, +{2} per prior command-zone cast), the §903.9b cast-from-
  command-zone redirect, commander damage (§903.10), 40 starting life,
  and the §800.4a player-leaves-the-game cleanup (the engine's hardest
  edge — see the BUG-LEDGER's open §800.4a items).
- **Per-card handlers** (`internal/gameengine/per_card/`) — cards too
  weird for the generic AST get hand-written hooks (ETB / cast /
  resolve / activated / trigger), registered into a `per_card` registry
  with double-fire guards so an AST trigger and a per-card hook never
  both fire. This is how the long tail of "snowflake" cards is handled.

The card text these run on is the **AST** (`internal/gameast`), the
structured parse of every card's oracle text produced by Thor.

## 2 · The Hex Judge — correctness as a faculty (`internal/judge`)

The defining build-out of the r60→r63 era: correctness stopped being a
pile of separate tools and became one engine-embedded faculty. There is
one violation type (`ValidationViolation`) and one router (`LogViolation`
in `router.go`); checks are registered by **dimension**.

- **LEGALITY** — was each action allowed? Deck-construction checks
  (count, color identity, singleton, banlist, commander eligibility)
  plus ride-along action legality (target / timing / cost / priority).
- **CONSERVATION** — are objects conserved by identity? A strict
  InstanceID census (`gameengine/invariants.go`) that subtracts legal
  §800.4 departures by construction, so a hit is a real leak.
- **STATE-INTEGRITY** — is state internally consistent? End-of-game SBA
  compliance, exactly-one-winner, stack/turn/layer/attachment integrity,
  and a per-seat win/loss self-checker (`seat_outcome.go`).
- **PROGRESSION** (`judge/progression/`) — does the game advance right?
  Re-derives each trigger's expected firing from the AST + the stimulus
  and observes it as a state delta: correct fire = one delta, zero =
  phantom, 2× = double-fire, plus APNAP order and intervening-if.
- **OUTCOME** (`judge/outcome/`) — did each effect produce the *right*
  result? Its distinctive mechanism is a **second, independent AST
  interpreter** that computes the expected post-resolution state delta
  and asserts it against what the engine actually did — parity against
  *intent*, per effect, at scale. This closed the engine's biggest blind
  spot (we used to verify "something changed," never "the right thing").
- **LIVENESS** (`judge/liveness.go`) — does the game terminate? Catches
  wall-clock non-termination, turn overrun, event-flood, and the
  cap-contract breach (a loop guard fired but the game didn't end).

Supporting it:

- **The correctness score** (`cmd/hexdek-correctness`) — runs the
  dimensions over a controlled game sweep + AST corpus and prints a
  per-dimension, reproducible topline (currently ~99.8%, deliberately
  not rounded to 100 — widening coverage is preferred over a prettier
  number).
- **The grinder watchdog** (`internal/hexapi`, gated by the
  `HEXDEK_JUDGE_SAMPLE` env var, default off) — a sampled live Judge
  over the production grinder; violations stream out for triage.
- **The CI gate** (`.github/workflows/judge.yml`) — the Judge runs in
  `--run` mode against a committed baseline so a correctness regression
  fails the build.

## 3 · The AI player — YggdrasilHat (`internal/hat`)

A layered evaluator, not a single MCTS. It plays all four seats in
self-play and pilots decks in the gauntlet.

- **Evaluation dimensions** (`eval_weights.go`) — the 8 named pillars
  (BoardPresence, CardAdvantage, ManaAdvantage, LifeResource,
  ComboProximity, ThreatExposure, CommanderProgress, GraveyardValue)
  are the foundation of a 20-axis vector (the extra 12 cover drain,
  artifact/enchantment synergy, stax-lock progress, stack interaction,
  etc.). Weights are tuned per archetype across 22 archetypes.
- **Archetype-aware weights** (`strategy_loader.go`, `evaluator.go`) —
  Freya's per-deck analysis ships an `eval_weights` profile in
  `strategy.json`; the evaluator uses it when present, else falls back
  to the archetype default, with a cEDH/casual power-tier routing pass.
- **The strategy-profile bridge** (`strategy.go`) — Freya's combos,
  tutor targets, value-engine keys, finishers, card roles, synergy
  clusters, and color demand flow directly into hat decisions (combo
  sequencing, tutor routing, what to protect, mana sequencing).
- **"3rd Eye" opponent tracking** (`yggdrasil.go`, `opponent_profile.go`)
  — per-opponent rolling intelligence: cards seen, inferred archetype +
  confidence, threat level and trajectory, and a political damage graph
  across all seat pairs. It informs attack targeting (kingmaker
  avoidance), interaction timing, and counter allocation.
- **Budget system** (`yggdrasil.go`, `rollout.go`) — 0 = pure
  heuristic, 1–199 = evaluator-guided, 200+ = rollout simulation, with
  *adaptive degradation*: on boards past ~60 permanents the budget
  tapers (to a 15% floor) so a complex board degrades search depth
  rather than hanging.
- **Conviction / concession** (`conviction.go`) — *currently
  diagnostic.* It tracks relative position over a sliding window and
  logs whether a concession trigger *would* have fired, but
  `ShouldConcede` always returns false — a deliberately non-acting
  instrument while the triggers are validated against real outcomes.
- **The "Curse" pool** (`curse.go`) — a per-deck genetic algorithm over
  personality DNA (aggression, combo patience, threat paranoia, …). It
  evolves every 100 games toward the profile that wins most with a given
  decklist, persisted via `SavePool` and reloaded at gauntlet start.

## 4 · The learning loop — Huginn (`internal/huginn`)

Huginn turns played games into opponent-prediction intelligence.
Heimdall feeds observed co-trigger pairs into `raw_observations.json`;
`Ingest` (wired into the tournament runner, `tournament/runner.go`)
graduates them through a three-tier confidence ladder (observed →
recurring → confirmed) and exports the confirmed pairs and inferred 3–5
card chains to `tier3_for_freya.json` for Freya to fold back into
analysis. The graduation half of the loop is closed as of r63; the
remaining seam is the deck-submission/Freya *analyze* path, which does
not yet auto-re-read the tier-3 file (see the honest caveat in
[deck-lifecycle.md](deck-lifecycle.md)). The through-line: play →
observe → graduate → analyze → better play.

## 5 · Freya — deck analysis (`cmd/hexdek-freya`)

Freya reads a decklist + the oracle corpus and writes a deep analysis:
a human `_freya.md` report, a hat-consumable `strategy.json`, and a full
`profile.json`. What it computes today (`main.go`, `archetypes.go`,
`deckprofile.go`, `advanced.go`, `roles.go`):

- **Archetype classification** — deck fingerprint → one of 20+
  archetypes with primary/secondary blend and confidence.
- **Combo detection** — a curated KnownCombos database matches pairs,
  triples, and triple-cycle loops, classified infinite / determined /
  finisher / synergy.
- **Card roles** — every card tagged (ramp, draw, removal, threat,
  combo, protection, utility, …) with priority ordering.
- **Win lines / finishers** — concrete game-closing sequences with
  per-line confidence and tutor-path resolution.
- **Bracket / power estimation** — a B1–B5 bracket synthesized from
  game-changers, win-line quality, protection density, mana base, and
  commander synergy (banned cards excluded from scoring).
- **Mana-base grade** — color fixing / tapland / fetch / utility-land
  scoring to an A–F grade with swap suggestions.
- **Opening-hand Monte Carlo** — 10K mulligan trials → keepable %, a
  commander-centric *adjusted* keepable % for Voltron/engine-commander
  decks, and average turn-to-commander.
- **Plus** mana-curve shape (bimodal / top-heavy warnings), synergy
  clusters, interaction quality (cheap vs. expensive), threat
  assessment (hoser database), meta positioning (archetype matchups),
  color-weight demand-vs-supply optimization, a deck-personality blurb,
  card quality tiers (star / solid / cuttable), and a power percentile
  within the archetype.

> Note: deck *consults* are a user-facing surface; they intentionally
> don't drive engine/AI priorities (see the project's working notes).

## 6 · The tooling suite (Norse naming, post-consolidation)

The r63 cleanup deleted dead tools and folded standalone validators into
the Judge. What's live:

- **Thor** (`cmd/hexdek-thor`) — parses the Scryfall oracle corpus into
  the AST dataset and stress-tests per-card interactions (destroy /
  exile / bounce / counter / copy / layer / APNAP / zone-chain
  batteries). Its old "goldilocks" dead-effect sweep was **retired into
  the Judge's OUTCOME dimension**; the file remains only for shared
  keyword-observability scaffolding.
- **Loki** (`cmd/hexdek-loki`) — the chaos fuzz tester: random 4-seat
  games over the full deck corpus, checking invariants after every move
  and flagging crashes + violations (the source of most of the
  BUG-LEDGER's zone-conservation finds).
- **Heimdall** (`cmd/hexdek-heimdall`) — post-game analytics and the
  spectator backend: per-card and per-player contribution, why-X-won
  analysis, and the co-trigger observations that feed Huginn.
- **Muninn** (`cmd/hexdek-muninn`) — now the **Hex Judge's triage
  clerk**: it consumes the grinder violation stream, dedupes by
  fingerprint, classifies by dimension, and files a triage report.
- **Tournament** (`cmd/hexdek-tournament`, `internal/tournament`) — the
  parallel match runner (round-robin / Swiss / double-elimination /
  pool), with Heimdall feedback for self-tuning.
- **Import** (`cmd/hexdek-import`, `internal/moxfield`) — the CLI deck
  importer (Moxfield / Archidekt → HexDek `.txt`).

(Retired in r63: Odin, Valkyrie, and several one-off CLIs — see
[LEGACY-EOL.md](LEGACY-EOL.md). `internal/huginn` lives on; only its CLI
wrapper was deleted.)

## 7 · The platform (`internal/hexapi`, `internal/hub`, `internal/db`)

The server (`cmd/hexdek-server`) exposes the engine, AI, and analysis
over HTTP/WS.

- **Gauntlet** (`hexapi/showmatch.go`, `RunGauntlet`) — a deck plays N
  games against random opponents in 4-player FFA; returns per-game logs
  and ELO deltas. This is the primary "how good is this deck" loop.
- **Rating: ELO + TrueSkill** (`tournament/elo.go`,
  `internal/trueskill`) — a multiplayer ELO plus a 4-player-FFA-tuned
  TrueSkill (mu/sigma); both update per game (`updateELO`) and TrueSkill
  mu/sigma persist to SQLite (as of r62).
- **Live spectator** (`internal/hub`, `hexapi/spectate_rooms.go`) — a
  WebSocket hub broadcasts game snapshots (seat state, mana,
  battlefield, loss reason) to viewers in a room, with idle timeout.
- **Deck import** — from Moxfield (`POST /api/import/moxfield`, fetches
  api2.moxfield.com and normalizes) and from pasted text / file upload;
  decks are stored as plain-text decklists with a version lineage and
  SQLite metadata (see [deck-lifecycle.md](deck-lifecycle.md) for the
  full flow).
- **Deck editor** — *MVP.* The card-search backend ships in r63
  (`hexapi/cards.go`: fuzzy name, type-line, oracle-text, and
  color-identity §903.4 filters). The full search/edit UX is staged —
  sweeping frontend changes go through a whitelisted staging review
  before production, so this surface is intentionally not fully rolled
  out yet.
- **Temporal Pincer analytics** (`internal/pincer`) — privacy-
  respecting page analytics: an anonymous per-browser UUID with **no
  PII**, recording coarse events (page viewed, deck viewed, game
  watched, import, login). On login the prior anonymous events are
  *stitched* to the user id.
- **Auth** (`internal/auth`) — device-token sessions with TOTP 2FA and
  API keys.
- **Persistence** (`internal/db`) — SQLite for decks, ELO/TrueSkill
  ratings, game history, and the import log.

## 8 · The frontend (`hexdek/`)

A React/Vite app — spectator, decks, queue, and party screens —
deployed to MISTY behind Caddy at hexdek.dev. The engine runs on
DARKSTAR (Windows 11) as a scheduled-task service on :8090. Backend
ships freely; sweeping UX changes go through the staging review noted
above.

---

*This file describes behavior, not API contracts — for endpoints see
[API.md](API.md); for system layout see [ARCHITECTURE.md](ARCHITECTURE.md).
When a feature here disagrees with the code, the code wins; please update
this file in the same change.*
