# Fail-closed `evalCondition`: the `if` and `conditional` kinds — census and verdict (r178-a1)

**Date:** 2026-10-18 · **Scope:** AST `Condition` kinds `if` (888 cards) and `conditional` (498 cards).
**Nature:** measurement only. No production code was changed.

---

## 0. Headline

| | |
|---|---|
| **`if` — premise HOLDS.** | 888 cards carry it; every occurrence routes to `evalCondition`, which fails closed. **785 of 888 cards (88.4%) are genuinely dead.** |
| **`conditional` — premise is FALSE.** | 498 cards carry it, but the engine **never passes it to `evalCondition` at all**. Those cards are inert for a *different* reason (an unimplemented `conditional_static` modification), and they contribute **zero** to the `condunk` counter. The fail-closed tail is not their bug. |

Combined, the honest number is **785 genuinely-dead cards out of the 1,377 in scope**, not 1,386 — and the 498 `conditional` cards need to be re-filed against a different root cause.

---

## 1. Calibration — reproduced exactly

Corpus: the **live** dataset pulled from `josh@192.168.1.206:~/hexdek/data/rules/ast_dataset.jsonl`
(51,272,120 bytes, 32,418 lines, manifest `export_date 2026-10-16T09:07:45+0000`, `entry_count 32417`).
The repo's local copy was not used. Traversal keys on `__ast_type__ == "Condition"`.

| kind | cards | occurrences | target | match |
|---|---|---|---|---|
| `if` | **888** | 921 | 888 | ✅ |
| `conditional` | **498** | 525 | 498 | ✅ |
| union | 1,377 | — | — | (9 cards carry both) |

Full unhandled-kind census (all condition kinds in the corpus, for context — only 20 kinds are handled, plus `paid_optional_cost` handled upstream in `resolveConditional`):

```
if                             occ=921  cards=888    UNHANDLED
paid_optional_cost             occ=717  cards=705    handled upstream (resolve.go:359)
conditional                    occ=525  cards=498    UNHANDLED (but never reaches evalCondition — §3)
for_each                       occ=268  cards=251    UNHANDLED
did_prior_action               occ=191  cards=190    UNHANDLED
etb_tapped_unless              occ=106  cards=106    UNHANDLED
threshold                      occ= 73  cards= 73    UNHANDLED
kicked                         occ= 50  cards= 47    handled
delirium                       occ= 41  cards= 41    UNHANDLED
no_spells_cast_last_turn       occ= 32  cards= 32    UNHANDLED
two_plus_spells_cast_last_turn occ= 32  cards= 32    UNHANDLED
metalcraft                     occ= 24  cards= 24    UNHANDLED
etb_if                         occ= 22  cards= 22    UNHANDLED
domain                         occ= 21  cards= 21    UNHANDLED
… (31 further kinds, ≤15 occurrences each)
```

### Live confirmation (VERIFIED, not inferred)

The current production grinder line on .206 (`server.log`, tail):

```
condunk=for_each:136303,if:115473,paid_optional_cost:114916,did_prior_action:6862,
        no_spells_cast_last_turn:3541,two_plus_spells_cast_last_turn:3541,
        you_control_creature_power_ge:2919,gained_life_this_turn:1557
```

`if` has fired **115,473** fail-closed evaluations in the live process. **`conditional` does not appear in the counter at all** — which independently confirms the code read in §3 below: that kind never reaches `evalCondition`.

---

## 2. Where these nodes actually sit (verified by traversal)

The split is total and clean — no mixing:

| kind | AST slot | occurrences |
|---|---|---|
| `if` | `Conditional.condition` (the `Conditional` **effect** node) | 921 / 921 |
| `conditional` | `Static.condition` (a **Static ability**) | 525 / 525 |

`Conditional.condition` is read by `resolveConditional` (`internal/gameengine/resolve.go:371`) → `evalCondition` → fail closed → **`Body` skipped**, and `ElseBody` executed if present.

Enclosing-ability chain for the 921 `if` nodes (so: are they reachable at all?):

```
608  Triggered                                 (590 cards)
151  Triggered/Sequence                        (145)
 88  Static/Modification/Triggered             ( 88)   — ability_word wrappers
 32  Static/Modification/Sequence              ( 32)
 19  Activated/Sequence                        ( 19)
  9  Activated                                 (  8)
  7  Static/Modification/Triggered/Sequence    (  6)
  7  Static/Modification                       (  7)
```

~97% sit inside a `Triggered` or `Activated` effect body — i.e. the ordinary `ResolveEffect` path that definitely calls `evalCondition` when the ability resolves.

---

## 3. The `conditional` half: PREMISE FALSE — stop and read this

`Static.Condition` has exactly **two** production readers in the whole repo:

- `internal/gameengine/etb_static_counters.go:72`
- `internal/gameengine/legality.go:1294`

and **both are gated on `st.Modification.ModKind == "etb_with_counters"`** before they look at the condition.

Meanwhile, **100% of the 525 `conditional` Condition nodes sit on a Static whose `modification.kind` is `conditional_static`** — never `etb_with_counters`. Therefore:

1. The condition is **never evaluated**. It is not fail-closed; it is ignored.
2. `conditional_static` is itself inert:
   - `internal/gameengine/resolve_helpers.go:1837` lists it in the log-only "Static/structural kinds" case — `gs.LogEvent(...)` and nothing else.
   - It appears **zero times** in `internal/gameengine/layers.go`, and all 689 corpus `conditional_static` Modifications carry `layer: null`, so `registerASTStaticEffects` skips them at `layers.go:2663` (`Layer == "" && !staticKindAllowedLayerless(...)` → `continue`).
3. The arg string is the **whole clause**, condition *and* effect — e.g. Static Orb's is
   `"as long as this artifact is untapped, players can't untap more than two permanents during their untap steps"`.
   `conditional_static` is a parser give-up bucket, not a gate.

**Conclusion for this half (VERIFIED by code read + the live counter's silence):** the 498 `conditional` cards' clauses are inert, but the cause is an unimplemented `conditional_static` modification kind. Fixing `evalCondition` would change **nothing** for them. This row should be re-filed against the `conditional_static` / residual-static-text bug class.

Of the 498, **24 cards have at least one `per_card` handler registered** (name match against the live registry dump; **registration-only, I did not read these 24 handler bodies** — the half was out of scope once the premise failed). The remaining **474** have no handler at all.

---

## 4. The `if` half: per-bucket verdict

**Denominator: 888 cards carrying `Condition{kind:"if"}`.**

| bucket | cards | % of 888 | basis |
|---|---|---|---|
| **GENUINELY DEAD** | **785** | **88.4%** | 772 with no handler registered at all (verified registry dump + verified no re-parser path); 13 with a handler that covers a *different* ability (verified by handler-body read) |
| **RESCUED by a `per_card` handler** | **103** | 11.6% | handler body read and confirmed to implement the gated ability *and* an equivalent gate |
| **RESCUED by a runtime re-parser** | **0** | 0% | verified: no call path exists (§5) |
| **NOT LOAD-BEARING** | **0** | 0% | one structural test applied (§6); it produced no card |
| **UNCLASSIFIED** | **0** | 0% | every card landed in a bucket — but see §7 for what that confidence does *not* cover |

### How the rescue set was established

The registry was dumped **programmatically**, not grepped: a throwaway `main.go` called
`per_card.Global().RegisteredCardNames()` after all `init()` registration, yielding **1,672** normalized
names. (The throwaway command was deleted; nothing was committed.) Name matching replicates
`normalizeName` (`per_card/registry.go:173`) and the `"Front // Back"` front-face fallback from
`lookupCandidates`.

- 888 `if` cards × registry → **115** name hits, **773** misses. The DFC front-face fallback found **2** more among the misses.
- Of the 115, **104** had a literal `On*("<Card Name>", …)` registration; the other **11** are table-driven family registrations (`etb_tribe_gate_family.go`, `land_tax_family.go`, `evoke_color_gate.go`, `gated_etb_effect_family.go`, `end_step_intervening_if_family.go`, `fetchlands.go`, `replacement_lifegain_ifwould_r63.go`).
- **All 115 + the 2 DFC candidates were resolved by reading handler gate logic.** None was acquitted on registration alone. (The 104 were read in four parallel passes; the 11 family entries and the 2 DFC entries I read directly.)

**Verdict: 103 RESCUED, 13 NOT_RESCUED.**

### The 13 cards that have a handler and are still dead (verified, high-confidence)

| card | the dead `if` clause | why the handler doesn't save it |
|---|---|---|
| Avatar Aang // Aang, Master of Elements | "you've done all four this turn" | `avatarAangStub` is a pure `emitPartial`; no bend tracking, no transform (`avatar_aang.go:29`) |
| Darigaaz Reincarnated | "exiled with an egg counter" / "no egg counters" | only the dies→exile redirect exists; nothing ever decrements the egg counters (`darigaaz_reincarnated.go:86`) |
| Golbez, Crystal Collector | "four or more artifacts" / "eight or more artifacts" | `golbez.go` implements **a different card's text entirely** — see §8 |
| Jace, Wielder of Mysteries | "your library has no cards in it" | the `if` is in the −8 ultimate; only the §614 draw replacement is wired (`jace_wielder_of_mysteries.go:38`) |
| Kami of Transience | "this card is in your graveyard" | end-step self-recursion explicitly `emitPartial`-declared unimplemented (`kami_of_transience.go:47`) |
| Lord of the Nazgûl | "you control nine or more Wraiths" | `nazgulTrigger` only amasses; never counts Wraiths, never applies the 9/9 (`commanders_batch.go:309`) |
| Master of Death | "this card is in your graveyard" | `emitPartial` names the missing graveyard-phase trigger dispatch (`master_of_death.go:62`) |
| Oloro, Ageless Ascetic | "Oloro is in the command zone" | handler is an `OnTrigger` on a battlefield permanent — structurally cannot fire from the command zone (`commanders_batch.go:625`) |
| Orvar, the All-Form | "it targets one or more permanents you control" | `orvarSpellCast` calls `emitPartial` and creates nothing (`orvar_all_form.go:43`) |
| Ragost, Deft Gastronaut | "you gained life this turn" | handler implements a different (mis-transcribed) ability (`commanders_batch.go:653`) |
| Tinybones, Trinket Thief | "an opponent discarded a card this turn" | handler writes `gs.Flags["tinybones_opp_discarded"]`, which **nothing reads** — inert-Flags bug class (`discard_spells.go:383`) |
| Warlock Class | "a creature died this turn" | handler covers only the level-2 dig (`class_level_up_consumers_r60.go:134`) |
| Pest Rescuer | "you don't control a Pest creature token" | handler is the life-gain `+1` replacement, a different ability (`replacement_lifegain_ifwould_r63.go:46`) |

### Reviewable list of the genuinely-dead cards

- `.scratch/dead_list.txt` — 798 rows / **772 cards** (no handler registered), `name | if <condition> | body=<kind> [| ELSE=<kind>]`
- `.scratch/dead_list_handler.txt` — 15 rows / **13 cards** (handler exists, covers another ability)

Representative head of the no-handler list:

```
A Killer Among Us | if target attacking creature token is the chosen type | body=CounterMod
A-Cosmos Elixir | if your life total is greater than your starting life total | body=Draw | ELSE=GainLife
A-Death-Priest of Myrkul | if a creature died this turn | body=CreateToken
Acrobatic Cheerleader | …
Agent of Treachery | …
```

**Worse than dead — 32 cards run the WRONG branch.** 32 of the 921 `if` occurrences have a non-null `else_body`. Fail-closed on those does not mean "nothing happens"; it means `ElseBody` executes unconditionally. (Example: `A-Cosmos Elixir` always gains life and never draws.)

Reachability of the dead bodies — the hosting trigger event, for the 772 no-handler cards:

```
293 phase        223 etb         62 (none: Activated/Static)   48 attack
 24 die           16 combat_damage_player   15 cast_filtered
 15 beginning_of_ordinal_step   10 tribe_you_control_etb   … 35 further events
```

The two dominant hosts (`phase`, `etb`) are core, continuously-fired events, so the dead bodies are on hot paths rather than exotic ones.

---

## 5. Runtime re-parser: zero rescues (VERIFIED)

There **is** a raw-text condition re-parser — `parseConditionText` in `internal/gameengine/conditional_parser.go:147`. Its only caller is `resolveConditionalEffect` (`:46`), whose only caller is the `case "conditional_effect":` arm of `ResolveModification` (`resolve_helpers.go:2245`). That is a **Modification kind**, reached from a different node type. **No path converts a `Condition{kind:"if"}` into a re-parsed condition.** `resolveConditional` calls `evalCondition` directly with no raw-text fallback.

### But the grammar already exists — and this is the actionable part

I replayed the **actual regexes** from `parseConditionText` against the **real argument strings** of all 921 `if` occurrences:

```
would map to an ALREADY-HANDLED kind: 405 / 921 occurrences (44.0%)

  raw (→ default true)         occ=516  cards=500
  you_control                  occ=176  cards=171
  kicked                       occ= 66  cards= 60
  self_is_type                 occ= 46  cards= 43
  self_was_cast                occ= 39  cards= 39
  card_count_zone              occ= 15  cards= 15
  you_attacked_this_turn       occ= 14  cards= 14
  permanent_left_this_turn     occ= 11  cards= 11
  life_threshold               occ=  9  cards=  9
  counter_on_self              occ=  8  cards=  8
  you_dont_control             occ=  5  cards=  5
  descended_this_turn          occ=  5  cards=  5
  not_your_turn                occ=  4  cards=  4
  first_combat_phase           occ=  4  cards=  4
  self_was_not_cast            occ=  2  cards=  2
  opponent_discarded_this_turn occ=  1  cards=  1
```

Routing the `if` kind's raw arg through the existing `parseConditionText` would give 44% of occurrences a structured condition **with no new grammar at all**.

⚠️ **Caveat on that 44%, stated plainly:** "matches a regex" ≠ "evaluates correctly". `reYouControl` is `^you control (.+)$` — greedy. It happily matches `"you control three or more creatures with different powers"` and hands `countControlledByType` the filter string `"three or more creatures with different powers"`, which will count 0 and return false. The 176 `you_control` hits are therefore an **upper bound** on correct rescues, not a count of them. The genuinely clean families are `kicked` (66), `self_was_cast`/`self_was_not_cast` (41), `you_attacked_this_turn` (14), `permanent_left_this_turn` (11), `counter_on_self` (8), `descended_this_turn` (5), `not_your_turn`/`first_combat_phase` (8) — ~153 occurrences that would work today.

---

## 6. Argument-shape frequency tables

### `if` — 921 occurrences, 538 distinct argument strings

Top exact strings:

```
  52  it was kicked
  24  you cast it
  22  a creature died this turn
  14  you cast it from your hand
  14  you attacked this turn
  11  a permanent left the battlefield under your control this turn
  11  you control three or more creatures with different powers
  11  an opponent lost life this turn
  10  you're the monarch
  10  you gained life this turn
   9  you've cast a noncreature spell this turn
   8  this creature is tapped
   8  an opponent controls more lands than you
   8  it had counters on it
   8  tribute wasn't paid
   7  you have no cards in hand
   7  there are four or more card types among cards in your graveyard
   7  this card is in your graveyard
   7  you control your commander
   6  it has three or more +1/+1 counters on it
   6  it was bargained
   6  you control two or more gates
   6  you have the city's blessing
   5  you control three or more artifacts
   5  there are three or more creature cards in your graveyard
   5  you control two or more tapped creatures
   5  you descended this turn
   4  it's the first combat phase of the turn
   4  at least four mana was spent to cast it
   4  you haven't cast a spell from your hand this turn
   4  an opponent controls more creatures than you
   4  creatures you control have total power 8 or greater
   4  you control a desert or there is a desert card in your graveyard
```

Leading-3-word phrase buckets — **this is the grammar `evalCondition` needs to be taught**:

```
  64  it was kicked          41  you cast it           40  you control a
  26  you control three      26  a creature died       22  you control two
  21  this creature is       19  you control another   18  you control no
  16  there are three        14  you control an        14  this card is
  14  you attacked this      14  an opponent lost      12  there are four
  12  an opponent controls   11  a permanent left      10  you've cast a
  10  you're the monarch     10  you gained life        8  it has three
   8  it had counters         8  tribute wasn't paid    8  you have the
   7  you have no             7  you haven't cast       7  you control your
   6  it was bargained        6  you have more          5  you don't control
   5  one or more             5  there are five         5  it's the first
   5  this creature didn't    5  that creature has      5  it had a
   5  creatures you control   5  you have fewer         5  you control seven
```

Distilled: the dominant families are **you-control-N-of-a-type** (~190 occ), **kicked/bargained/tribute/alternative-cost-paid** (~80), **was-this-spell-cast / cast-from-where** (~45), **something-happened-this-turn** (died / attacked / gained life / lost life / left-the-battlefield / descended ≈ 90), **graveyard counting** (~40), **self-state** (tapped / counters / is-a-type ≈ 60), **monarch / city's blessing / commander** (~25).

### `conditional` — 525 occurrences, 512 distinct argument strings

Essentially all unique. Leading-phrase buckets show the shape immediately:

```
 374  as long as …        (71% — continuous-effect clauses, verbatim)
  42  if it's a …          12  if it shares …     7  if this land …
   6  if that card …        5  if this spell …    5  if you control …
   5  if it was …           4  if you would …   … 25 further 1–3-count heads
```

Examples (each the **whole clause**, condition + effect, stuffed into one string arg):

```
as long as this artifact is untapped, players can't untap more than two permanents during their untap steps
as long as enchanted creature is white, it gets +1/+1 and has lifelink
as long as you have the city's blessing, detectives you control can't be blocked
if it's a land card, put it onto the battlefield. otherwise, put it into your hand
```

This is a **parser output-shape** problem, not a condition-evaluation problem: the grammar needed here is one that *splits* the clause into a condition and a continuous effect in the first place, so `conditional_static` stops being a dumping ground.

---

## 7. What makes my number shakier than it looks

Stated separately, as asked. **Verified** = I read code or ran a probe. **Inferred** = reasoning.

1. **(Inferred) Per-card reachability of the 772 no-handler cards is not individually proven.** I verified the mechanism (fail-closed evaluation) and that no handler is registered. I did **not** prove, card by card, that the enclosing trigger event actually fires in a real game. If some of those 772 hang off trigger events the engine never emits, they are dead for *two* reasons and fixing `evalCondition` alone won't revive them. The event histogram in §4 argues most are on hot paths (`phase` 293, `etb` 223), but that is an aggregate argument, not 772 proofs.
2. **(Verified but delegated)** The 104 handler-body reads were done by four parallel readers against the same written brief. I read the 11 family registrations and the 2 DFC candidates myself and spot-checked `etb_tribe_gate_family.go`, `land_tax_family.go`, `end_step_intervening_if_family.go`, `gated_etb_effect_family.go` and `fetchlands.go` directly. The individual citations are the readers'; I did not re-derive all 104.
3. **(Verified) Several "RESCUED" verdicts are rescued-but-lossy, and I counted them as rescued.** Flagging the known ones so nobody reads 103 as 103 correct cards:
   - **Fabled Passage** — the `if you control four or more lands` gate is *not implemented*; `fetchlands.go:54` says "MVP: we treat it as untapped". The effect happens, the gate does not. Counted RESCUED because the body is not dead.
   - **Vibrance** (×2 clauses) and **Deceit/Wistfulness** family — gated on `was_cast` rather than the hybrid pip actually paid, so they **over-fire** on every cast.
   - **Breaching Leviathan**, **Grave Scrabbler**, **Sunderflock**, **Tiamat**, **Yathan Roadwatcher**, **Transcendent Dragon**, **Unstable Glyphbridge**, **Bringer of the Last Gift**, **Eon Frolicker** — all gate on `Flags["was_cast"]`, which does not distinguish cast-from-hand; a graveyard/exile cast wrongly fires them.
   - **Mana Vault** — registered on `upkeep_controller`; the printed trigger is the **draw step**. Fires one step early.
   - **Raphael, Fiendish Savior** — gate narrows "put into your graveyard from anywhere" to "died".
   - **Rocco, Cabaretti Caterer** — tutor silently no-ops when `x_paid` isn't threaded.
   - **Chronozoa** — "copies" are hardcoded 3/3 Illusion tokens.
   - **Lighthouse Chronologist** — fires at any level; the level-7 gate is not modeled.
4. **(Inferred) "NOT LOAD-BEARING = 0" rests on one test only.** I tested exactly one structural criterion: is the `if`-body itself a Modification whose kind sits in the log-only case list at `resolve_helpers.go:1837`? Exactly one occurrence matched (Flubs, the Fool / `cast_trigger_tail`), and that card is rescued by `per_card` anyway. I did **not** test "the body duplicates an effect that happens elsewhere on the card", which would be a genuine not-load-bearing case and would shave the dead count. If you want that, it needs a per-card oracle read.
5. **(Verified) 7 of the 772 dead cards are Alchemy rebalances (`A-` prefix)** and will never appear in a paper Commander deck. Trivial, but it's 7 off any "real impact" count.
6. **(Verified) The 24 `conditional` cards with a `per_card` handler were registration-checked only.** I did not read those bodies. That half's root cause changed (§3), which made the per-card question moot for the verdict — but if someone re-files the `conditional_static` row, those 24 need a real read.
7. **(Verified) Corpus ≠ deck pool.** Every count here is over the 32,417-card oracle corpus. None of it is weighted by how often these cards appear in the decks actually played. `if:115473` on the live counter is the only play-weighted number in this document, and it is an occurrence count across all unhandled-`if` cards, not a per-card breakdown.

---

## 8. Incidental findings worth filing separately

Surfaced while reading handlers; not part of the count.

1. **`Golbez, Crystal Collector` is implemented as the wrong card.** `per_card/golbez.go` implements "whenever you cast a spell from graveyard/exile, create a Treasure" plus "{2}{B}: exile target card from a graveyard, you may cast it" — text that is **not on the card**. The real Golbez is "whenever an artifact you control enters, surveil 1" plus the end-step 4+/8+ artifact recursion/drain. Both `if` clauses are dead *and* the handler adds two phantom abilities.
2. **`Lord of the Nazgûl`'s token clause is also wrong** — `nazgulTrigger` amasses 1 instead of creating a 3/3 black Wraith with menace.
3. **`Mana Vault` fires its tapped-damage at upkeep, not the draw step** (`commander_staples.go:146`).
4. **`Tinybones, Trinket Thief`** writes `gs.Flags["tinybones_opp_discarded"]` which has **no reader anywhere in the repo** — another instance of the known inert-`Flags` bug class.
5. **`Oloro, Ageless Ascetic`** exposes a structural gap: a command-zone triggered ability has no `*Permanent` for `fireTrigger`'s battlefield walk to find, so no `per_card` `OnTrigger` can ever implement one.

---

## 9. Recommended re-filing

- **Keep High severity for `if`** — 785 dead cards out of 888, 115k live fail-closed evaluations, 32 cards running the wrong branch. The cheapest first move is routing `Condition{kind:"if"}`'s raw arg through the existing `parseConditionText`, which already covers ~153 occurrences cleanly and ~405 nominally; then tighten `reYouControl`, then add grammar for the families in §6.
- **Split `conditional` out into its own row** against `conditional_static` / residual-static-text, severity to be set on that basis. It is not an `evalCondition` bug and the current row over-counts the fail-closed blast radius by 498 cards.

---

### Artifacts (scratch, not committed)

`/private/tmp/claude-501/hexdek-a1/.scratch/` — `ast_live.jsonl` (the live corpus), `dead_list.txt`,
`dead_list_handler.txt`, `if.json`, `cond.json`, `if_full.json`, `xref.json`, `percard_names.txt`
(programmatic registry dump), `handler_report.txt`, `group1-4.txt`, `scripts/`.
