# mtgsquad Architecture

One-page map of how oracle text becomes executable game state.

```
                 ┌──────────────────────────────────────────────────────────────┐
                 │  Scryfall bulk dump → data/rules/oracle-cards.json (31,639)  │
                 └─────────────────────────────┬────────────────────────────────┘
                                               │
                                               ▼
     ┌───────────────────────────────────────────────────────────────────────────┐
     │  scripts/parser.py                                                        │
     │    normalize(card) → split_abilities(text) → parse_ability(chunk)         │
     │           │                                       │                       │
     │           │                ┌──────────────────────┼────────────────────┐  │
     │           │                ▼                      ▼                    ▼  │
     │           │         try Keyword           try Triggered         try Static│
     │           │         try Activated         spell fallback        (+extensions)
     │           │                                                               │
     │           └──────────────────→  typed AST (scripts/mtg_ast.py)            │
     └───────────────────────────────────────┬───────────────────────────────────┘
                                             │
                ┌────────────────────────────┼────────────────────────────┐
                ▼                            ▼                            ▼
    ┌───────────────────┐     ┌────────────────────────┐     ┌────────────────────────┐
    │ coverage_honest.py│     │ layer_harness.py       │     │ runtime engine         │
    │ dual-metric       │     │ tag Modifications with │     │ internal/game/ (Go)    │
    │ report            │     │ §613 layers            │     │ (consumes AST; future) │
    └───────────────────┘     └────────────────────────┘     └────────────────────────┘
```

---

## 1. Parser pipeline

Source: `scripts/parser.py` (~98K lines counting inlined grammar tables).

The parser is a recursive-descent grammar with a registry of regex-keyed effect rules. For each card it runs roughly:

1. **`normalize(card)`** — pull `oracle_text`, concatenate `card_faces` on DFCs, lowercase, strip reminder text, normalize em-dashes, replace the card's own name with `~`.
2. **`split_abilities(text)`** — break on canonical delimiters (newlines, known keyword lists, ability words, modal bullets). Emits a list of ability-sized chunks.
3. **`parse_ability(chunk)`** — try each production in precedence order:
    - **Keyword** shorthand lookup (`Flying`, `Flashback`, `Cycling`, …). Keywords that expand to full abilities still emit a `Keyword` node; the runtime is responsible for the expansion so the AST stays a stable interface.
    - **Triggered** — anchored at `When / Whenever / At`. Grammar for trigger event + intervening `if` clause + effect body.
    - **Activated** — `cost:effect` split at the unescaped colon; cost parsed via `parse_cost`, effect via the effect-rule registry.
    - **Static** — anything left that matches a `STATIC_PATTERN`. Includes continuous effects, replacement effects, timing restrictions.
    - **Spell fallback** — instants/sorceries whose body is just an effect clause are wrapped as a single activated-like `Effect` at the card level.
4. **Unconsumed text is recorded** in `parse_errors` so the coverage report can pinpoint exactly which grammar production is missing. This is what made 100% syntactic coverage tractable — every gap is a concrete, named grammar bug.

Entry points:

```bash
python3 scripts/parser.py                 # full report → data/rules/parser_coverage.md
python3 scripts/parser.py --card "Snapcaster Mage"   # dump one card's AST
python3 scripts/parser.py --errors-top 40            # show top-N unconsumed fragments
```

---

## 2. AST schema

Source: `scripts/mtg_ast.py`. All nodes are `@dataclass(frozen=True)` so the AST is hashable, comparable, and safe to cache.

Per comp rules §113, a card's abilities are one of four top-level kinds:

| Node | Shape | Example |
|---|---|---|
| **`Static`** | `(condition?, modification?, raw)` | `Creatures you control get +1/+1` |
| **`Activated`** | `(cost, effect, timing_restriction?, raw)` | `{T}: Add {G}` |
| **`Triggered`** | `(trigger, effect, intervening_if?, raw)` | `When ~ enters, draw a card` |
| **`Keyword`** | `(name, args, raw)` | `Flying`, `Flashback {2}{U}` |

Supporting types:

- **`ManaSymbol` / `ManaCost`** — `{U}`, `{2}`, `{U/B}`, `{X}`, `{S}`, `{U/P}`.
- **`Filter`** — a structural target spec (`target creature you control`, `each opponent`, `a basic Plains card`). Two filters with the same shape compare equal.
- **`Trigger`** — event slug (`etb`, `die`, `attack`, `cast`, `phase`, …) plus optional actor/target filters and phase.
- **`Cost`** — a composite (mana, tap, untap, sacrifice, discard, pay-life, exile-self, return-self, counter-removal, extra).
- **`Condition`** — a typed boolean (`you_control`, `life_threshold`, `card_count_zone`, `tribal`, …).

**Effect nodes** are the recursive part. Control-flow nodes (`Sequence`, `Choice`, `Optional_`, `Conditional`) wrap leaf effects. Every leaf is a comp-rules-defined effect type, each carrying the parameters that structurally distinguish it:

`Damage · Draw · Discard · Mill · Scry · Surveil · CounterSpell · Destroy · Exile · Bounce · Tutor · Reanimate · Recurse · GainLife · LoseLife · SetLife · Sacrifice · CreateToken · CounterMod · Buff · GrantAbility · TapEffect · UntapEffect · AddMana · GainControl · CopySpell · CopyPermanent · Fight · Reveal · LookAt · Shuffle · ExtraTurn · ExtraCombat · WinGame · LoseGame · Replacement · Prevent · UnknownEffect`

**`Modification`** is the static-ability body (anthems, restrictions, type-adds, replacement effects). It carries a `kind` slug, `args`, and a `layer` tag (see §4).

**`CardAST`** is the top-level container: `(name, abilities, parse_errors, fully_parsed)`. `signature(ast)` produces a hashable structural fingerprint used to cluster functionally equivalent cards.

---

## 3. Extension ecosystem

The parser's grammar rules are registered at import time through a module-local decorator plus an auto-loader. Source: `scripts/extensions/` (~50 modules).

On startup `parser.py` calls `load_extensions()`, which `importlib`-loads every non-underscore `.py` under `scripts/extensions/` and merges four registries:

| Registry | Where it feeds | Used for |
|---|---|---|
| `EFFECT_RULES` | effect-rule table | new effect grammars (`@rule(pattern)` decorators) |
| `STATIC_PATTERNS` | static-ability matcher | anthems, continuous effects, restrictions |
| `TRIGGER_PATTERNS` | trigger matcher | new trigger events |
| `PER_CARD_HANDLERS` | per-card table checked first in `parse_card` | 1,079 snowflake cards routed to named handlers |

The extensions are organized by concern: `replacements.py`, `combat_triggers.py`, `sagas_adventures.py`, `vehicles_mounts.py`, `quoted_abilities.py`, `equipment_aura.py`, `stack_timing.py`, `pronoun_chains.py`, `villainous_choice.py`, and so on. A family of `partial_scrubber_*` and `unparsed_residual*` modules exist specifically to pick up the long tail of grammar variations that the core productions miss.

**Design intent**: the core parser is the stable grammar. Extensions are where new mechanics land when a set introduces them, without having to touch the main parse loop. A contributor adding Bloomburrow's *Offspring* keyword writes a new extension module; they do not edit `parser.py`.

`per_card.py` is the escape hatch for cards that defeat any reasonable grammar (Karn Liberated's ultimate, Shahrazad, cards with nested quoted abilities that mutate across game state). These emit stub `Modification(kind="custom", args=(slug,))` placeholders that the runtime engine must resolve by slug dispatch.

---

## 4. Layer harness

Source: `scripts/layer_harness.py`. Output: `data/rules/layer_harness.md`.

MTG's continuous-effects system (comp rules §613) resolves effects in a strict layer order:

| Layer | Purpose |
|---|---|
| 1 | copy effects |
| 2 | control-changing |
| 3 | text-changing |
| 4 | type / subtype / supertype changes |
| 5 | color-changing |
| 6 | ability add / remove |
| 7a | characteristic-defining P/T |
| 7b | P/T set ("becomes 1/1") |
| 7c | P/T modify (anthems, +N/+N) |
| 7d | counters |
| 7e | P/T switching |

The layer harness walks every card's AST, inspects every `Static` ability's `Modification`, and assigns a layer label based on the modification's `kind` slug plus, where relevant, the effect node types reachable inside triggered/activated bodies. Non-layered effects (activated costs, stack triggers, one-shot spell effects, timing restrictions) get `layer=None`.

**Why this matters for the runtime**: when the engine resolves continuous effects, it cannot derive layers on the fly — that is quadratic and error-prone. Pre-tagging at parse time means the runtime's continuous-effects resolver sees a layer-labeled stream and can apply effects in the canonical §613 order deterministically. It also surfaces cards that span multiple layers (Humility, Blood Moon, Opalescence) as explicit multi-layer entries rather than edge cases the engine discovers at runtime.

Current distribution is summarized in `data/rules/layer_harness.md`. The biggest cohorts are always layer 6 (ability add/remove) and layer 7c (P/T modification); multi-layer cards (Humility, Blood Moon, Opalescence) are called out explicitly.

---

## 5. Engine work owed

From `data/rules/coverage_honest.md` (regenerate for live numbers):

- **~24% of cards are engine-executable today.** Their AST is composed of typed leaf nodes (`Damage`, `Buff`, `Tutor`, `Destroy`, …) and a runtime interpreter can execute them by dispatching on `kind` alone.
- **~76% parse but carry stubs.** Their AST contains `Modification(kind="custom", args=(slug,))` nodes, or per-card stubs from `extensions/per_card.py`. These are recognized — the parser did not fail — but the runtime cannot execute them until a hand-coded resolver exists for the slug.

The runtime engine (`internal/game/` in Go) therefore has two integration points against the AST:

1. **A typed dispatcher** — one Go function per leaf effect kind (`executeDamage`, `executeBuff`, `executeTutor`, …). This mechanically covers the structural slice.
2. **A slug registry** — a `map[string]Resolver` keyed by custom-modification slug and by card name for per-card handlers. Adding a resolver to this registry is how engine coverage grows toward 100%.

The parser's job is done. The next build is working through the custom-slug and per-card-handler list in priority order. `scripts/cluster_priority.py` ranks the outstanding work by how frequently each stub appears across real decklists, so effort lands on the slugs that matter for actual play first.

---

## 6. Python self-play loop (`scripts/playloop.py`)

A Python-only structural simulator that proves the parser AST can drive an
end-to-end game. It deliberately duplicates runtime work on the Python side
(rather than wire Python→Go immediately) so the AST can be exercised against
live turn state the same day a new effect kind lands in the parser.

**Turn pipeline:** untap → upkeep (triggers) → draw → main1 → **combat** →
main2 → end. The combat phase runs declare-attackers → declare-blockers →
first-strike damage → regular damage, with state-based actions between damage
steps.

**Combat keyword coverage today:** flying, reach, first strike, double strike,
trample (spillover to player), deathtouch, lifelink, vigilance, menace (2+
blockers), defender, haste.

**Dispatch pattern:** the loop walks `Activated.effect` / `Triggered.effect`
bodies and dispatches on Effect node kinds (`Damage`, `Draw`, `Buff`,
`CreateToken`, `AddMana`, `Tutor`, `Reanimate`, etc.). `UnknownEffect` nodes
are counted and surfaced in the per-matchup report.

**Report:** `data/rules/playloop_report.md` (per-matchup win rates, turn
distribution, unknown-node rollup, deck lists). Sample per-matchup games are
logged to `data/rules/playloop_sample_log.txt` in JSONL form suitable for the
browser replay viewer.

## 7. Replay viewer (`web/replay/`)

Zero-dependency static HTML/CSS/JS. Consumes the JSONL event stream emitted by
`playloop.py` and replays it step-by-step (scrub, prev/next, autoplay) against
a two-seat board visualisation. No game logic runs in the viewer — it only
applies logged state deltas. Drop-in for humans auditing a self-play session
or for shipping post-mortems of a bot-vs-bot match.

## 8. Combo detector (`scripts/combo_detector.py`)

Static-analysis scanner over the parsed AST. Surfaces six families of
candidates — mana-positive engines, untap triggers, storm engines, iterative
draw engines, doublers (replacement effects on resources), and activated-untap
permanents — then runs a simple 2-card pair search keyed on untap-trigger ×
mana-positive to flag infinite-mana candidates.

**Heuristic, not a solver.** Every entry is a candidate for human review; false
positives are expected. Outputs:

- `data/rules/combo_detections.md` — human-readable tables grouped by family
- `data/rules/combo_detections.json` — programmatic export

The same static lens is useful during engine buildout: the same AST shapes
that make a combo detectable are the shapes that make a runtime interpreter
work.

## 9. AST dataset + finetune pairs

`scripts/export_ast_dataset.py` and `scripts/export_finetune_pairs.py` dump the
typed AST for every card in two shapes:

- **Raw JSONL** (`data/rules/ast_dataset.jsonl`) — one `{name, oracle_text,
  type_line, mana_cost, cmc, colors, ast}` row per card. Every dataclass-backed
  node carries an `__ast_type__` string so consumers can discriminate
  structurally-similar nodes (e.g. `Damage` vs `Draw` vs `UnknownEffect`) at
  the dataset layer.
- **Instruction pairs** (`data/rules/finetune_pairs.jsonl`) — Alpaca-style
  `{instruction, input, output}` triples for oracle-text → AST-JSON
  finetuning (LoRA or full-parameter).

These are the entry point for finetuning a model that learns the parser's
grammar directly and can be used to bootstrap new card-set expansions.

## 10. Test suite (`tests/`)

Pytest with two tiers:

- **Golden regression** (`test_parser_regression.py`) — 201 parametrized tests,
  one per curated card across 8 mechanic families (cEDH staples, triggers,
  keywords, layer-7 effects, modal, ramp, removal, vanilla). Each card's AST
  is compressed to a compact structural signature (`ability_count`,
  `parse_error_count`, per-ability `kind` + first-5 DFS node kinds) and
  compared byte-for-byte against `tests/golden/<card>.json`. A regression
  lights up exactly the failing card(s), not the whole suite. Regenerate
  intentionally with `python3 tests/generate_golden.py`.
- **Whole-corpus coverage** (`test_parser_coverage.py`, marked `slow`) — two
  asserts that run across all 31,639 real cards: `green_count == 31639` and
  every card parses without raising.

## 11. Runtime engine (today vs. target)

Source: `internal/game/`. Files: `engine.go`, `combat.go`, `types.go`, `storage.go`, `json_helpers.go`.

**Today the engine handles:**
- Turn structure (untap, upkeep, draw, main, combat, end).
- Priority passing.
- Mana pool and payment.
- Casting basic spells.
- Basic battlefield state transitions.
- SQLite-backed ephemeral game state (`internal/db/`).
- WebSocket hub for multi-client sessions (`internal/ws/`).

**Today the engine does not:**
- Consume the Python AST directly. AST export to a Go-readable form (JSON / protobuf) is not yet wired.
- Resolve the stack against AST effects.
- Apply §613 continuous-effects layers.
- Handle the custom-slug or per-card-handler registry.
- Expose an AI-playable surface.

**Target runtime flow** (not yet implemented):

```
Go engine boot → load AST JSON emitted by parser.py
             → build (kind → handler) and (slug → resolver) dispatch tables
             → game loop resolves each stack object by walking its CardAST abilities
             → continuous-effects pass applies Modifications in §613 layer order
             → state diffs stream to clients (humans via UI, AI via JSON API)
```

---

## Relevant files

| File | Purpose |
|---|---|
| `scripts/parser.py` | Oracle text → CardAST |
| `scripts/mtg_ast.py` | Typed frozen-dataclass AST schema (§113) |
| `scripts/extensions/` | ~50 grammar-extension modules |
| `scripts/extensions/per_card.py` | 1,079 snowflake per-card handlers |
| `scripts/layer_harness.py` | §613 layer tagger |
| `scripts/coverage_honest.py` | Dual-metric coverage reporter |
| `scripts/rules_coverage.py` | Detailed parser diagnostics |
| `scripts/cluster_priority.py` | Prioritize engine work by deck-frequency |
| `scripts/semantic_clusters.py` | AST-signature clustering |
| `scripts/combo_detector.py` | Static-analysis infinite-combo scanner |
| `scripts/playloop.py` | Python self-play loop (3 matchups, full combat) |
| `scripts/export_ast_dataset.py` | Dumps typed AST to JSONL |
| `scripts/export_finetune_pairs.py` | Alpaca-style instruction pairs |
| `tests/` | 203-test pytest suite (goldens + whole-corpus coverage) |
| `web/replay/` | Static browser replay viewer for playloop logs |
| `data/rules/oracle-cards.json` | Scryfall bulk dump (input) |
| `data/rules/MagicCompRules-20260227.txt` | Source of truth spec |
| `data/rules/parser_coverage.md` | Parser report |
| `data/rules/coverage_honest.md` | Dual-metric report |
| `data/rules/layer_harness.md` | §613 layer distribution |
| `data/rules/cluster_priority.md` | Stub work prioritized by usage |
| `data/rules/combo_detections.{md,json}` | Static combo/loop candidates |
| `data/rules/playloop_report.md` | Self-play matchup results |
| `data/rules/ast_dataset.jsonl` | Every card's typed AST (~40 MiB) |
| `data/rules/finetune_pairs.jsonl` | Alpaca-style instruction pairs |
| `internal/game/` | Go runtime engine (basic today) |
| `cmd/mtgsquad-server/` | HTTP/WebSocket entry point |
