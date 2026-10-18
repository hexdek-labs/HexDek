# r178 — `evalCondition` fail-closed: `for_each` and `paid_optional_cost` classified

**Status:** classification only. No production code changed. Not committed.
**Worktree:** `/private/tmp/claude-501/hexdek-a2` (`dev/r178-a2`, base `d56a023a`).
**Corpus:** live `~/hexdek/data/rules/ast_dataset.jsonl` on 192.168.1.206, pulled read-only
(51,272,120 bytes, mtime 2026-10-16 09:15, 32,418 lines = 1 `DatasetManifest` + **32,417 card rows**).
The repo's local copy was NOT used.

---

## 0. Headline — the premise is half wrong, and the half that is wrong matters more than the counts

Two corrections, both load-bearing, both verified:

**(1) 139 of the 251 `for_each` cards never reach `evalCondition` at all.**
`for_each` appears in the AST in two unrelated positions. Only one of them is a
`Conditional.condition`, which is the only position `resolveConditional` →
`evalCondition` ever sees. The other 153 occurrences sit on a `Static.condition`
whose sibling `Modification.kind` is **always** `for_each_rider` (153/153), and
`Static.Condition` is read in **exactly two** places in the engine —
`internal/gameengine/etb_static_counters.go:72` and
`internal/gameengine/legality.go:1294` — **both of which `continue` unless
`st.Modification.ModKind == "etb_with_counters"` first.** `layers.go` never calls
`evalCondition` (verified: all six production call sites enumerated in §2).
So the fail-closed tail is irrelevant to those cards. They are inert, but for a
different reason: `for_each_rider` is a member of the log-only "static/structural"
no-op case list at `internal/gameengine/resolve_helpers.go:1840`.
**Teaching `evalCondition` a `for_each` case fixes zero of them.**

**(2) For the 112 that *do* reach it, `for_each` is not a boolean gate — it is a
repetition count, and returning `true` would also be wrong.**
`Conditional{condition: for_each(X), body: E}` is the parser's encoding of
"for each X, E" — Bloom Tender's `For each color among permanents you control,
add one mana of that color`; Adeline's `for each opponent, create a 1/1`. The
correct semantics is *run `E` once per `X`*, not *run `E` iff `X`*. A new
`case "for_each": return true` would make Adeline mint **one** token instead of
three and Bloom Tender add **one** mana instead of up to five. The fix is a
count/loop construct in the resolver (and a `count` the parser can hand it),
not a 21st condition kind. **This is a different shape of fix from the rest of
the fail-closed burn-down list and should not be scheduled with it.**

Corollary worth stating plainly: `paid_optional_cost` is *also* not really a
"broken gate". 704 of its 705 cards say "you may …". Fail-closed means the
engine **always declines** every optional cost. Declining a "may" is a legal
choice under CR §601.2b, so this is a **play-strength / card-inertness** defect,
not a rules violation — with 12 measured exceptions where the engine ends up
*too generous* (§5.3).

---

## 1. Calibration (required before anything else)

Traversal: every JSON node in `ast`, keyed on **`__ast_type__`** (not `node`),
selecting `__ast_type__ == "Condition"` with `kind` in the target set.

| kind | cards found | cards expected | occurrences |
|---|---|---|---|
| `for_each` | **251** | 251 | 268 |
| `paid_optional_cost` | **705** | 705 | 717 |

Exact match on both. Denominator for both: **32,417 card rows**.
Neither kind appears anywhere in the corpus as a `Modification.kind` or any
other node type — only as `Condition`.

---

## 2. Every production path into `evalCondition` (so "reachable" is a fact, not a guess)

`grep -rn 'evalCondition(' --include='*.go' internal/` → 6 non-test sites:

| site | guard before the call | can it see our kinds? |
|---|---|---|
| `resolve.go:371` (`resolveConditional`) | none | **YES** — this is the only live path |
| `conditional_parser.go:65` | runtime re-parse of a `conditional_effect` raw string; `parseConditionText` emits 20 kinds, **never** `for_each`/`paid_optional_cost` | no |
| `stack.go:238` (intervening-if at push) | `Triggered.intervening_if` | no — corpus scan shows neither kind ever occupies that field |
| `stack.go:2151` (intervening-if at resolution, CR §603.4) | same | no |
| `etb_static_counters.go:72` | `st.Modification.ModKind == "etb_with_counters"` | no |
| `legality.go:1294` | `st.Modification.ModKind == "etb_with_counters"` | no |

`resolveConditional` (`resolve.go:351`) has **no raw-text fallback**: on a false
condition it runs `ElseBody` if present and otherwise returns. The runtime
re-parser (`resolveConditionalEffect`, reached from
`resolve_helpers.go:2247`) handles the `conditional_effect` **Modification**
kind — a different node — and therefore **cannot rescue a `Conditional` node**.
*The "RESCUED by a runtime re-parser" bucket is therefore empty for both kinds,
and that is a structural fact, not a sampling result.*

The only in-engine rescue specific to these kinds is
`resolvePaidOptionalCost` (`resolve.go:380`), which returns `handled=true`
for exactly one argument value: `"forage"`.

---

## 3. Verdicts

### 3.1 `for_each` — denominator 251 cards

| bucket | cards | % of 251 | how decided |
|---|---|---|---|
| **NOT LOAD-BEARING (for this bug)** — `Static.condition` + `for_each_rider`, never evaluated | **139** | 55.4% | §0(1); both `Static.Condition` readers gate on `etb_with_counters` first |
| **GENUINELY DEAD** — `Conditional.condition`, no rescue | **106** | 42.2% | reachable per §2; no handler; no re-parser path |
| **RESCUED by a `per_card` handler** (gate logic read, not just registration) | **6** | 2.4% | §3.3 |
| RESCUED by a runtime re-parser | 0 | — | structurally impossible, §2 |
| UNCLASSIFIED | 0 | — | — |

(139 + 112 = 251; 3 cards carry the condition in **both** positions and are
counted in the 112 live set, so the "never evaluated" row is 142 − 3 = 139.)

Caveat I will not paper over: the 139 are **not fine**. They are dead too, via
`for_each_rider` being a log-only no-op. They are simply *not this bug*, and
10 of them carry a `per_card` registration (Atraxa, Braids, Hurkyl, K'rrik,
Mizzix's Mastery, Niv-Mizzet Reborn, Sylvan Library, Tasha, Tivit, Valki) whose
gate logic I did **not** read — that row is out of scope and its true
dead-count is **inferred, not verified**.

### 3.2 `paid_optional_cost` — denominator 705 cards

| bucket | cards | % of 705 | how decided |
|---|---|---|---|
| **GENUINELY DEAD** (the "if you do" branch never runs) | **670** | 95.0% | reachable; no handler covering the clause |
| **RESCUED by a `per_card` handler** (handler body read) | **33** | 4.7% | §3.4 |
| **RESCUED by the `forage` carve-out** in `resolvePaidOptionalCost` | **2** | 0.3% | Bushy Bodyguard, Treetop Sentries — both cards' *only* hits are `forage` |
| RESCUED by a runtime re-parser | 0 | — | structurally impossible, §2 |
| UNCLASSIFIED | 0 | — | — |

**Severity re-rating inside the 670** (this is the part that should change how
it is prioritised):

| sub-bucket | cards | consequence |
|---|---|---|
| has an `ElseBody` in the AST | **84** | the "if you don't" branch **does** fire — e.g. all ten shocklands correctly enter tapped. Rules-legal; the option is simply never offered. |
| no `ElseBody`, oracle has no "if you don't"/"unless" | **574** (621 − 12, minus 35 rescued) | nothing happens = the engine declined. **CR-legal**, but the card is inert. |
| no `ElseBody` **and** the oracle does carry an "if you don't"/"unless" penalty | **12** | the penalty never lands → **engine is too generous**. The real rules-violating set. Listed in §5.3. |

5 of the 38 cards with a registration are **REGISTRATION-ONLY** — the handler
implements a *different* clause and the optional branch is still dead:
**Avatar Aang // Aang, Master of Elements** (`avatar_aang.go:29` is a bare
`emitPartial` stub), **Mana Vault** (`commander_staples.go:136` only does the
draw-step damage; the pay-{4}-untap is modelled as activated `abilityIdx 1`,
but Vault's only printed activated ability is `{T}: Add {C}{C}{C}` at idx 0, so
the case is unreachable — **Mana Vault never untaps**), **Oloro, Ageless
Ascetic** (`commanders_batch.go:625` only the upkeep gain-2; no `life_gained`
handler), **Sygg, Wanderwine Wisdom // Sygg, Wanderbrine Shield**
(`sygg_wanderwine_wisdom.go:63` literally emits
`first_main_phase_optional_transform_partial`; no trigger registered), and
functionally **Vraska, the Silencer** (`vraska_the_silencer.go:29` neither pays
{1} nor returns the dying card — it mints a Treasure instead). Those 5 are
inside the 670.

### 3.3 The 6 `for_each` acquittals — all verified by reading the handler body

| card | handler | what it actually does |
|---|---|---|
| Acererak the Archlich | `per_card/era3_batch.go:436` | loops `gs.LivingOpponents`, mints one 2/2 Zombie each |
| Adeline, Resplendent Cathar | `per_card/adeline_resplendent_cathar.go:64` | loops live seats ≠ controller, mints one 1/1 Human each, once-per-turn dedup |
| Altaïr Ibn-La'Ahad | `per_card/altair_ibn_la_ahad.go:51` | loops tagged exiled creature cards, mints a tapped-attacking copy of each |
| Bladewing, Deathless Tyrant | `per_card/bladewing_deathless_tyrant.go:19` | counts creature cards in graveyard, mints that many 2/2 menace Zombies |
| Sovereign Okinec Ahau | `per_card/sovereign_okinec_ahau.go:55` | walks own battlefield, snapshots `Power() − BasePower` diffs, applies counters |
| Unstable Glyphbridge // Sandswirl Wanderglyph | `per_card/unstable_glyphbridge.go:48` | loops all live seats, picks a ≤2-power survivor per seat, destroys the rest |

All six genuinely implement the per-X loop. **None is registration-only.**

### 3.4 The 33 `paid_optional_cost` acquittals — handler bodies read, 9 with a material partial

Full RESCUED list: Alesha Who Smiles at Death; Aphelia Viper Whisperer; Arahbo
Roar of the World; Ashling Rekindled//Rimebound; Aziza Mage Tower Captain;
Drake Haven; Dyadrine Synthesis Amalgam; Eye of Vecna; Faith of the Devoted;
Grub Storied Matriarch//Notorious Auntie; Hakbal of the Surging Soul; Hashaton
Scarab's Fist; Isochron Scepter; Katara Waterbending Master; Lightning Rift;
Niambi Esteemed Speaker; Panoptic Mirror; Perplexing Chimera; Quintorius
History Chaser; Raff Weatherlight Stalwart; Riku of Two Reflections; Rith the
Awakener; Sauron the Dark Lord; Shanna Purifying Blade; Smellerbee Rebel
Fighter; Strefan Maurer Progenitor; Sylvan Library; The Mimeoplasm; Toph
Hardheaded Teacher; Treva the Renewer; Ulalek Fused Atrocity; Veronica
Dissident Scribe; Winter Cynical Opportunist.

Acquitted **but materially partial** (counted as RESCUED — the gated effect
happens — with the defect recorded here so nobody re-acquits them later):

* **Rith the Awakener, Treva the Renewer, Shanna Purifying Blade** — effect
  fires, cost is **not deducted** (explicit `emitPartial` in each).
* **Isochron Scepter** (`isochron_scepter.go:137`), **Panoptic Mirror**
  (`game_changers.go:1644`) — the copy only does something if the imprinted
  card has its own per_card `ResolveHandler`; otherwise it resolves to nothing.
* **Drake Haven**, **Faith of the Devoted** (`card_cycled_consumers_r60.go:205,240`)
  — the *cycling* half is wired, the *discard* half has no `card_discarded` handler.
* **Sauron the Dark Lord** — only fires off Sauron's own army-combat-damage path.
* **Ulalek Fused Atrocity** — copies only the triggering Eldrazi spell, not
  "all spells you control, then all other activated and triggered abilities".

### 3.5 Dispatch semantics — does a `per_card` handler suppress the AST path?

Per-(card, event), never blanket per-card:

* **Spell resolve: suppresses, all-or-nothing.** `stack.go:2375` —
  `snowflakeFired := InvokeResolveHook(gs, item)`, then
  `if item.Effect != nil && snowflakeFired == 0 { ResolveEffect(...) }`.
* **Activated abilities: NO suppression — both run.** `activation.go:1107` calls
  `InvokeActivatedHook` and then resolves the AST effect anyway. (The comment
  there claims the handler is authoritative; the code does not act on it.) So
  for Mana Vault / Isochron Scepter / Quintorius the dead AST
  `paid_optional_cost` branch runs *in addition* and silently fails closed.
* **Self-ETB: suppresses only with an explicit `r.OwnsETBTrigger(name)` opt-in**
  (`etb_dispatch.go:216` → `OwnsETB`, `registry.go:1068`). A bare `OnETB` does not.
* **Named game-event triggers: suppresses per (card, event)** via
  `PerCardOwnsTrigger` (`per_card_hooks.go:71`).

For this exercise it is mostly moot: a dead AST branch does nothing whether or
not it is suppressed. It matters only for `OnResolve` cards, where a handler
silently voids the rest of the AST spell body.

---

## 4. Why `for_each` out-ranks `paid_optional_cost` on live frequency

### 4.1 The counter, with its denominator

Taken from the same `grinder:` line on `.206:~/hexdek/server.log`, so the
numerator and denominator are from one sample, not two:

```
2026/10/18 12:17:39 grinder: 31369 games ...
  condunk=for_each:136303, if:115473, paid_optional_cost:114916,
          did_prior_action:6862, no_spells_cast_last_turn:3541,
          two_plus_spells_cast_last_turn:3541,
          you_control_creature_power_ge:2919, gained_life_this_turn:1557
```

| | for_each | paid_optional_cost |
|---|---|---|
| evaluations | 136,303 | 114,916 |
| **per game (÷31,369)** | **4.34** | **3.66** |
| corpus cards | 251 | 705 |
| **cards that can actually reach `evalCondition`** | **112** | **705** |
| evals ÷ reachable cards | **1,217** | **163** |
| deck-slots across the live 1,758 deck files | 580 | 5,073 |
| **evals ÷ deck-slot** | **235** | **22.6** |

### 4.2 The explanation, with the evidence

**It is not a loop and it is not a characteristics-read re-evaluation.** The
hypothesis in the brief — that `for_each` is a scaling construct re-read on
every characteristics computation — would require `evalCondition` to be
reachable from the layer system. It is not: §2 enumerates all six call sites
and none is in `layers.go`; `AvailableColoredManaEstimate` (`hat.go:150`) does
not call it either. And at **4.34 evaluations per game** the absolute rate is
mundane. `for_each` ranks #1 only relative to a small field.

The asymmetry is **population × repeatability**, measured two ways:

1. **Reachability halves the `for_each` denominator.** All 136,303 evaluations
   come from the 112 `Conditional`-parent cards; the other 139 contribute
   literally zero. That alone turns a 251-vs-705 card ratio into 112-vs-705 and
   raises per-card frequency 7.5×.

2. **The two kinds live on opposite ends of the repeatability axis.** Counting
   name occurrences across all 1,758 `.txt` deck files on `.206`:

   | `for_each` top deck-slots | n | ability shape |
   |---|---|---|
   | Bloom Tender | 107 | **repeatable activated mana ability** (`{T}: for each color…, add`) |
   | Faeburrow Elder | 51 | same |
   | Adeline, Resplendent Cathar | 38 | per-combat attack trigger |
   | Ocelot Pride | 22 | per-turn end-step trigger |
   | Ainok Strike Leader | 21 | attack trigger |

   | `paid_optional_cost` top deck-slots | n | ability shape |
   |---|---|---|
   | Watery Grave | 223 | **once-per-game ETB replacement** |
   | Steam Vents | 219 | same |
   | Breeding Pool | 210 | same |
   | Overgrown Tomb | 199 | same |
   | Blood Crypt / Godless Shrine | 196 / 191 | same |

   The ten shocklands alone are **1,872 of `paid_optional_cost`'s 5,073
   deck-slots (37%)**, and each fires **exactly once per copy per game**.
   `for_each`'s two biggest populations are mana dorks whose ability is
   activated on most turns of most games.

   Net: **235 evaluations per deck-slot for `for_each` vs 22.6 for
   `paid_optional_cost` — 10.4×.** That ratio is the whole story.

3. **Cross-check on the host-ability mix** (occurrence-weighted, from the AST):
   `for_each`'s live occurrences are 21 `Activated` + 11 `attack` + 10 `phase`
   + 2 `you_attack` triggers; `paid_optional_cost`'s largest single host bucket
   is `etb_as` (49) plus `etb` (80), i.e. one-shot entry replacements.

### 4.3 What this means for the fix order

Bloom Tender is in **107 of 1,758 deck files** and currently **produces no mana
at all** — `Conditional{for_each(color among permanents you control),
AddMana}` → condition false → `resolveAddMana` never called. Faeburrow Elder
(51) is identical. That is the single highest-impact item in either kind, and
it is a *count* bug, not a *gate* bug.

---

## 5. Argument-shape tables (the grammar a fixer needs)

### 5.1 `for_each`, live (`Conditional`-parent) — 115 occurrences, 58 distinct

| n | argument |
|---|---|
| 34 | `opponent` |
| 10 | `player` |
| 8 | `nonland card revealed this way` |
| 3 | `creature card in your graveyard` |
| 2 | `color among permanents you control` |
| 2 | `kind of counter on target permanent or player` |
| 2 | `card exiled this way` |
| 2 | `creature token you control` |
| 2 | `token you control that entered this turn` |
| 2 | `kind of counter on target permanent` |
| 1 ×48 | long tail: `land destroyed this way`, `of those colors`, `shrine you control`, `blight counter on it`, `1 life you lost`, `attacking modified creature you control`, `defending player`, `counter removed`, `land target player controls in excess of the number you control`, … |

**Grammar shape:** 44/115 (38%) are a bare **player iteration** (`opponent`,
`player`, `defending player`) — a `for each player` loop plus a per-iteration
target binding covers more than a third of the live set outright. The next
family, 20+ occurrences, is **"… this way"** back-references to a count
produced earlier in the same effect (`nonland card revealed this way`,
`card exiled this way`, `land destroyed this way`, `creature tapped this way`)
— those need the resolver to carry a per-resolution counter, which is a
genuinely new capability. The remainder are ordinary battlefield/graveyard/
counter counts that `countControlledByType` and `countCardsInZone` could
already supply if the node returned an int instead of a bool.

### 5.2 `paid_optional_cost` — 717 occurrences, 302 distinct

| family | n | share |
|---|---|---|
| `pay {mana}` | 276 | 38.5% |
| other (long tail) | 110 | 15.3% |
| `discard …` | 80 | 11.2% |
| `exile …` | 53 | 7.4% |
| `put …` | 40 | 5.6% |
| `pay N life` | 36 | 5.0% |
| `draw …` | 35 | 4.9% |
| `reveal …` | 31 | 4.3% |
| `sacrifice …` | 30 | 4.2% |
| `tap`/`untap …` | 15 | 2.1% |
| `return …` | 11 | 1.5% |

Top exact strings: `discard a card` 66, `pay {1}` 57, `pay {2}` 29,
`draw a card` 24, `pay {e}{e}` 15, `pay 2 life` 15, `sacrifice it` 15,
`pay 3 life` 14, `exile it` 12, `pay {3}` 11, `pay {b}` 11, `pay {w}` 10.

**Grammar shape:** a mana-cost parser plus five verbs (`discard`, `exile`,
`sacrifice`, `pay N life`, `draw`) covers **~66%** of all occurrences. Unlike
`for_each`, this one *is* the shape the brief assumed — a payable-cost
predicate wired the way `resolvePaidOptionalCost` already wires `forage`, with
an AI decision hook replacing today's unconditional decline.

### 5.3 The 12 where fail-closed makes the engine *too generous*

No `ElseBody`, but the oracle carries an "if you don't"/"unless" penalty that
therefore never lands. These are the only rules-violating members of the
`paid_optional_cost` set, and some are weak matches (`Aegis Sculptor` matched
on Ward reminder text) — treat the list as a review queue, not a verdict:

Oblivious Bookworm · Aegis Sculptor · **Flash** · Dragon's Disciple ·
Meathook Massacre II · Moon-Circuit Hacker · Tilonalli's Summoner ·
Giant Albatross · Squealing Devil · Island Fish Jasconius ·
Lazotep Chancellor · Leviathan

`Flash` is the sharp one: *"You may put a creature card from your hand onto the
battlefield. If you do, sacrifice it unless you pay its mana cost reduced by
{2}."* — fail-closed means the put never happens, so it is dead rather than
generous. Verify each individually before acting.

---

## 6. Reviewable lists of the genuinely-dead cards

Full tables, sorted by live deck-slot count, with the gated clause and the body
node type for each, are in **`.scratch/tables.md`** in this worktree
(not committed; regenerate with `.scratch/bin/final.py`).

* **`for_each` GENUINELY DEAD — 106 cards**, 514 deck-slots, **56 of the 106
  appear in at least one of the 1,758 live decks.** Head of the list:
  Bloom Tender (107) · Faeburrow Elder (51) · Ocelot Pride (22) ·
  Ainok Strike Leader (21) · Kambal Profiteering Mayor (20) ·
  Renewed Solidarity (19) · Sepulchral Primordial (18) · Tragic Arrogance (18) ·
  Grasp of Fate (17) · Selvala Explorer Returned (16) · The Eternal Wanderer (14).
* **`paid_optional_cost` GENUINELY DEAD — 670 cards**, 4,761 deck-slots,
  **311 of the 670 appear in at least one live deck.** Head of the list:
  the ten shocklands (223 → 137 each, all in the harmless `ElseBody` sub-bucket) ·
  Mana Vault (191) · Sink into Stupor//Soporific Springs (122) ·
  Fell the Profane//Fell Mire (103) · the Snarl/Slowland cycles.

---

## 7. What makes my own number shakier than it looks

1. **"GENUINELY DEAD" for `paid_optional_cost` is an argument from absence.**
   I searched `internal/gameengine/per_card/` for the *card name* and found 38
   of 705. If a handler exists under a different display name (DFC face spelled
   differently, an alias, or a name-normalising registration form I did not
   match), I will have mis-filed it as dead. My name extraction matched only
   `.On{Resolve,Trigger,ETB,Dies,LTB,Attack,Activate,Cast,Static,Upkeep,Replacement}("…")`
   — 1,416 names across 1,530 files. A map-literal or generated registration
   form would be invisible to it. **Direction of error: the dead count is an
   upper bound.**
2. **I read 44 handler bodies, not 705 cards' worth of engine behaviour.**
   The 670 are "no handler found", not "proven inert in a game". None of the
   verdicts in §3.1–3.2 is backed by an executed repro.
3. **The 139 never-evaluated `for_each` cards are classified on static reading
   of two call sites.** If some other code path reflects over `Static.Condition`
   in a way my `grep -rn '\.Condition\b'` sweep missed, that row moves.
4. **Deck-slot counts are a crude line-match** over 1,758 `.txt` files,
   lowercased, with a leading quantity and a trailing `(SET) n` stripped. DFC
   names with ` // ` and names carrying diacritics will under-count; 2 of the
   112 live `for_each` names did not resolve against the pool at all. This only
   affects §4's ratios, not the card counts.
5. **The 31,369-game denominator is the grinder's own counter; the `condunk`
   counter is process-global.** If anything other than the grinder ran games in
   that process, the per-game rates in §4.1 are slightly high. The *ratio*
   between the two kinds is unaffected.
6. **The AST dataset is a snapshot (2026-10-16).** A parser change re-shapes
   these counts; the `parser_source_hash` in the manifest is
   `72e89f15…3be54793`.
7. **The 10 `per_card`-registered cards inside the 139 never-evaluated
   `for_each` set were not read.** That sub-bucket's true inert count is
   inferred.
8. **`UNCLASSIFIED` is zero, and that should be read as a claim, not a
   comfort.** I had a structural reason to empty the re-parser bucket (§2) and
   read every handler in the two registration sets; anything I got wrong will
   surface as a mis-bucketed card, not as an acknowledged unknown.

---

## 8. Recommended Issue Log amendment

The existing High-severity framing ("N cards gated on an unrecognised condition
do nothing") should be split:

* **`for_each` — re-file as a *parser/resolver shape* defect, not a
  fail-closed defect.** 139/251 never touch `evalCondition`; the other 112 need
  a repetition primitive, and `case "for_each": return true` would be a *new*
  bug. Highest-impact single card: **Bloom Tender** (107/1,758 decks, produces
  no mana).
* **`paid_optional_cost` — downgrade from rules-violation to AI-decision gap.**
  670/705 dead, but 704/705 are "you may", so always-declining is CR-legal. The
  84 with an `ElseBody` (all shocklands included) already behave correctly. The
  genuinely wrong set is the **12** in §5.3. The fix is the
  `resolvePaidOptionalCost` switch plus a Hat decision hook, and the grammar in
  §5.2 says five verbs plus a mana parser gets ~66% of it.
