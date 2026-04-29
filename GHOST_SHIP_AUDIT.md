# Ghost-Ship Audit — mtgsquad AST Corpus

**Generated:** 2026-04-25 (post zone-change codemod)
**Authorized by:** 7174n1c, in HexDek Discord thread
**Method:** Static analysis of `data/rules/ast_dataset.jsonl` (32K cards) cross-referenced against engine event/handler surface

---

## TL;DR

The mtgsquad AST corpus contains **substantial ghost-ship population** —
cards whose AST parses but silently no-ops at runtime due to one of:

1. Triggered ability with `phase=null` and/or `event=null`
2. Triggered ability whose `event` string isn't dispatched by the engine
3. Effect stored as `conditional_effect` Modification with raw text in args (parser
   couldn't decompose the conditional structurally)

The codemod that shipped overnight (`db31cfaba`) fixed the **engine event-firing**
layer for zone changes. This audit measures the **next layer** — parser/AST
coherence with the engine's trigger-name vocabulary and effect-handler registry.

## Headline numbers

| Metric | Value |
|---|---|
| Cards in AST corpus | 31,965 |
| Cards with ≥1 ability | 31,604 |
| Cards with ≥1 Triggered ability | 12,430 |
| Total Triggered abilities | 14,418 |
| Total Static abilities | 28,877 |
| Total Activated abilities | 10,684 |
| Total Keyword abilities | 15,510 |

## Ghost classes (confidence-tiered)

### High-confidence ghosts (definite no-ops)

**`ghost.trigger.phase_null` — 1,419 abilities, 1,393 unique cards (4.4% of cards w/ abilities)**

Triggered abilities whose `event` is phase-style (`"phase"`, `"end_step"`,
`"upkeep"`, `"draw"`) but whose `phase` field is null. The engine's
`triggerMatchesPhaseStep` cannot match these — they silently never fire.

**Zoyowa Lava-Tongue** is in this bucket. Sample: A Girl and Her Dogs,
A-Alrund God of the Cosmos, A-Cosmos Elixir, A-Death-Priest of Myrkul,
A-Dreamshackle Geist, A-Geology Enthusiast, A-Iridescent Hornbeetle,
A-Minsc & Boo.

**`ghost.effect.raw_conditional` — 790 abilities, 767 unique cards (2.4%)**

Effects stored as `Modification(kind="conditional_effect", args=["raw text"])`.
Engine has no evaluator for this kind that parses the arg string. Conditional
fires silently as no-op even if the trigger itself were wired correctly.

Sample: A-Acererak the Archlich, A-Death-Priest of Myrkul, A-Geological
Appraiser, A-Guildsworn Prowler, A-Minsc & Boo, A-Sigil of Myrkul,
A-Sprouting Goblin, A-The One Ring.

### Lower-confidence ghosts (may have alternate dispatch paths)

**`ghost.trigger.unknown_event` — 11,101 abilities, 9,904 cards (31.3%)**

Triggered abilities whose `event` string isn't in my known-events set
(extracted from `FireCardTrigger` call sites + `zoneChangeToTriggerEvents`
+ `triggerMatchesPhaseStep` recognized phases).

**CAVEAT:** This is an upper bound. The engine has alternate dispatch
paths beyond `FireCardTrigger` — e.g., `stack.go:1089` scans for
`"etb"`-event Triggered abilities at the moment a permanent resolves,
`combat.go:625` matches `"attack"` events at combat-declaration. So
"etb" and "attack" Triggered abilities ARE handled in their primary
self-trigger context, but **observer triggers** keyed on those same
events (cards that watch other permanents entering/attacking) likely
don't get notified consistently.

The corpus's top "unknown" events:
- `etb` — 4,173 abilities (works as self-trigger via stack.go; observer behavior partial)
- `attack` — 1,089 (works via combat.go for self; observer behavior partial)
- `cast_filtered` — 491 (filtered cast triggers — engine only knows `spell_cast` family)
- `etb_as` — 255 (ETB-as-X / "enters as a copy of" patterns)
- `when_you_do` — 232 (cascading-trigger pattern)
- `cast_any` — 205 (broader cast trigger)
- `beginning_of_ordinal_step` — 144
- `enter_or_attack` — 128
- `another_typed_enters` — 124
- `self_and_another` — 114
- `turned_face_up` — 113
- `type_leaves_battlefield` — 111

**Total of 401 distinct "unknown" event names** across the corpus.
This is the parser-engine vocabulary mismatch problem — parser emits
fine-grained event names; engine dispatches on a smaller, coarser set.
A normalization layer (event-alias table) would close most of this gap
without per-event handler work.

## Modification.kind histogram (top 30)

Where the parser packages effect bodies. Engine handlers for these are
mostly switch-cases; ones without handlers silently no-op.

| Count | Cards | Kind | Likely Handler? |
|---|---|---|---|
| 5,641 | 5,239 | `parsed_effect_residual` | ❌ catchall — parser fallback |
| 1,635 | 1,456 | `add_mana` | ✅ |
| 1,334 | 1,265 | `untyped_effect` | ❌ catchall |
| 1,331 | 1,299 | `sequence` | ✅ (probably) |
| 1,311 | 1,251 | `counter_mod` | ✅ |
| 1,156 | 1,114 | `buff` | ✅ |
| 1,092 | 1,073 | `optional` | partial |
| 1,019 | 982 | `create_token` | ✅ |
| 825 | 795 | `conditional_effect` | ❌ raw text |
| 792 | 783 | `draw` | ✅ |
| 611 | 585 | `grant_ability` | partial |
| 525 | 497 | `damage` | ✅ |
| 496 | 490 | `destroy` | ✅ |
| 445 | 437 | `gain_life` | ✅ |
| 418 | 416 | `bounce` | ✅ |
| 409 | 406 | `choice` | partial |
| 376 | 366 | `exile` | ✅ |
| 305 | 302 | `tap` | ✅ |
| 282 | 247 | `conditional` | partial |
| 243 | 241 | `cast_trigger_tail` | ❌ probably |
| 212 | 211 | `reanimate` | partial |
| 209 | 207 | `tutor` | partial |
| 209 | 209 | `look_at` | partial |
| 204 | 203 | `regenerate` | partial |
| 198 | 195 | `untap` | ✅ |
| 192 | 192 | `optional_effect` | partial |
| 189 | 189 | `mill` | ✅ (post-codemod) |
| 178 | 178 | `lose_life` | ✅ |
| 161 | 161 | `sacrifice` | ✅ |
| 157 | 157 | `scry` | ✅ |

The big offenders are `parsed_effect_residual` (5,641 — anything the parser
gave up on), `untyped_effect` (1,334), and `conditional_effect` (825). These
three alone account for ~7,800 ability bodies that don't have specific engine
evaluators.

## Recommended next-step work order

1. **Event-name normalization layer.** Add an alias map: parser-emitted event names → engine-canonical event names. Closes most of the 11,101 "unknown event" matches without per-event handler work. Big-bang win.

2. **`conditional_effect` evaluator.** Parse the raw conditional string into structured Condition + sub-effect at runtime (or push parser to emit structured Conditions). Unlocks the 790 `raw_conditional` ghosts including Zoyowa.

3. **`phase_null` parser fix.** When the parser detects a phase-style trigger, populate `Trigger.phase` properly. ~1,400 ghost cards become testable.

4. **`parsed_effect_residual` triage.** This is the parser's "I gave up" bucket. 5,641 abilities. Probably worth a separate audit pass focused specifically on what's in the residual text and whether targeted parser extensions could decompose it.

## Estimated unlock per fix

| Fix | Cards unlocked (est.) | Cost (est.) |
|---|---|---|
| Event-name alias table | 9,900 (high-confidence subset of ghost.unknown_event) | small — one engine table + lookup |
| `conditional_effect` evaluator | 770 | medium — needs runtime conditional-string parser OR parser refactor |
| `phase_null` parser fix | 1,400 | small-medium — parser-side regex discipline |
| `parsed_effect_residual` triage | up to 5,200 | large — case-by-case parser extension authoring |

Combined naive ceiling: ~14K-17K cards unlocked. With overlap (a card may
sit in multiple buckets) the true unique unlock is likely 8K-12K cards.

## What the codemod did NOT fix (visible now)

The overnight codemod fixed the engine's **event firing** layer for
zone-changes. It did not fix:
- Parser-engine event-name vocabulary mismatch (the 31% upper-bound bucket)
- `conditional_effect` raw-text evaluation
- `phase=null` / `event=null` Triggered abilities
- The `parsed_effect_residual` parser-fallback bucket

The codemod was a necessary prerequisite. Phase 3 (corpus audit + ghost-fixing)
is the multi-week bulk of the remaining work.

---

*Audit script: `/tmp/ghost_ship_audit.py` (transient — re-runnable against any updated AST dataset).*
