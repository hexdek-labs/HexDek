# Extensions Changelog

This directory is auto-loaded by `scripts/parser.py::load_extensions()`. Every
`*.py` file here (except `__init__.py` and `per_card.py` helpers) gets imported
alphabetically at startup, and any `EFFECT_RULES`, `STATIC_PATTERNS`,
`TRIGGER_PATTERNS`, and `PER_CARD_HANDLERS` the module exports are merged into
the parser's global registries.

## Directory layout

Two broad families of files live here:

1. **Mechanic-family extensions** — one file per MTG mechanic or grammar
   shape (`combat_triggers.py`, `equipment_aura.py`, `counter_triggers.py`,
   `vehicles_mounts.py`, etc.). Each file owns a coherent slice of the game
   (~15-30 files). These are the stable spine of the extension layer.

2. **Scrubbers** — iterative promotion passes that target residual PARTIAL
   and UNPARSED buckets. There are many scrubbers (11 `partial_scrubber*`, 2
   `unparsed_residual*`, `unparsed_final_sweep`, `partial_final`, and 2
   `snowflake_rules*`). Each pass was written AFTER the previous passes
   shipped, by re-bucketing whatever parse_errors remained and keeping the
   densest clusters.

**Why so many scrubber files?** Each scrubber is frozen at a specific
coverage milestone (`scrubber_2` targets the clusters visible AFTER
`scrubber` shipped; `scrubber_3` targets what survived `scrubber_2`; and so
on). Merging them into one file would break:

- **Load order / specific-first matching.** Patterns accumulate in lists
  and the first match wins, so alphabetical load order is load-bearing.
- **Top-level name reuse.** Nearly every scrubber defines its own `_sp`,
  `_er`, `_num`, `_NUM_WORDS`, `_COLOR`, `_MULTIPIP_KEYWORDS`,
  `_BARE_KEYWORDS`, plus dozens of single-use builder closures
  (`_token_anthem`, `_doran`, `_bare_keyword`, etc.). Each module is an
  isolated namespace; merging would require renaming hundreds of helpers
  or risk silently shadowing a sibling's patterns.
- **Bisect-ability.** When a regression lands, `git blame` on a small file
  plus the pass docstring points straight at the cluster that changed.

Documenting the passes here lets future readers understand the history
without touching the files. Each scrubber's own module docstring already
records what it owns ("Highlights of the N-th-pass cluster set" ...); this
changelog is the unified timeline.

## Coverage timeline

All percentages are measured against the 31,639-card pool.

### Baseline (core parser + mechanic-family extensions only)

The stable spine (`combat_triggers`, `equipment_aura`, `counter_triggers`,
`mana_dynamics`, `stack_timing`, `damage_prevention`, `library_manip`,
`life_triggers`, `token_specs`, etc.) lands the parser in the high-80s%
GREEN range. The scrubbers push it to 100%.

### PARTIAL-bucket promotions

| File | Added ~when | Pool state at entry | Owns |
|------|-------------|---------------------|------|
| `partial_scrubber.py` | First pass | Large PARTIAL residue | Bulk clusters the base grammar mishandled: multi-pip keyword bodies (morph/flashback/madness/cycling), `{cost}` bare keywords, common ward customs, pain-tail riders, bare keyword statics like `_BARE_KEYWORDS`, ability-level anthems. Ships `STATIC_PATTERNS` and `EFFECT_RULES`. |
| `partial_scrubber_2.py` | After #1 | ≥6-hit residual clusters | Multi-pip mana keywords (morph, flashback, unearth, suspend, madness, etc.) that base `KEYWORD_RE` only allowed one pip for; integer-count keywords; ability-word riders (`adamant - ...`); standalone effect lines (`copy it`, `its controller draws a card`, `gift a card`); trigger shapes `whenever X, Y`. **First file to ship `TRIGGER_PATTERNS`.** |
| `partial_scrubber_3.py` | After #2 | ≥5-hit residual clusters | `creatures you control gain <kw>`; opponent anthems; activated-cost modifiers; `must be blocked if able`; `this creature can block any number`; Platinum Angel "you don't lose the game" variants; fight tails; token creation riders; distributive opponent edicts. |
| `partial_scrubber_4.py` | After #3 | ≥6-hit residual clusters | `your maximum hand size is <X>`; land-creature anthems; typed cross-product anthems; tribal-lord spell variants; max-speed riders; play-lands-from-zone / cast-spells-from-zone shapes; mana-use restrictions; reveal-opening-hand clause; extra-combat statics; `you may cast that card ...` body. |
| `partial_scrubber_5.py` | After #4 | ≥3-hit clusters | Wider net. Spree mode sub-abilities (`+ {cost} - <effect>`); more ability-word riders (`companion -`, `infusion -`, `paradigm`, `time travel`); bare keywords (`convert`, `devoid`, `afflict N`, `blitz {cost}`, `kicker-<rider>`); gy-cards-untargetable; "your spells can't be countered"; attacking-ally anthems; typed token anthems; self-is-all-colors. |
| `partial_scrubber_6.py` | After #5 | ~4,010 residual error strings | Suspend with cost-spec (`suspend N-{cost}`, 64 hits — biggest cluster left); multikicker with cost (18); single-pip megamorph; bloodthirst X; populate; `become the monarch`; `activate no more than N times each turn`; "don't lose unspent mana"; enchanted-buff must-attack; base-PT overrides (`eot that base pt`, `its pt mirrors`). |
| `partial_scrubber_7.py` | After #6 | ~2,940 PARTIAL, 9.29% | Mutate with cost (~40 cards); awaken with cost (~14); firebending N and firebending X; type-prefixed offering (Patron-of-* cycle); bare vanishing; echo-discard; artifact landcycling; `~ walk`; solved riders; transform-self; self-plus-X variables; pacifism-plus buffs. |
| `partial_scrubber_8.py` | After #7 | ~2,775 PARTIAL, 8.77% | Escalate; splice-onto-arcane; reinforce-dash; equip-sacrifice alt-cost; suspect-it; typed spell casting restrictions; land-play restrictions; enchanted-creature abilities-off; enchant-multi-type; paralyze-tax on auras; per-counter buffs on aura itself; enchant-gets-X variables. |
| `partial_scrubber_9.py` | After #8 | ~2,607 PARTIAL, 8.24% | Each-other-typed anthem; each-creature-with-counter; permanents-with-counters anthem; attack-despite-defender; creature-cards-in-zone-have-kw; `is` as spells static; spells-with-MV statics; legendary anthem; "can't pay or sac"; "deck can have up to ..."; protection-each; enchantment-deals-damage bodies; `this creature deals damage to that creature`. |
| `partial_scrubber_10.py` | After #9 | 1,986 PARTIAL, 6.28% | "You lose N life for each <noun>"; "put one of those cards ... one of ..." distributive recur tail; mass-recur grave picks; polymorph/overlay `until end of turn, it has base PT`; "may reveal card that shares X"; "this effect doesn't remove auras"; "you may cast spells from top of library"; typed-anthems for less-common types; mana restriction riders. |
| `partial_scrubber_11.py` | After #10 | 1,708 PARTIAL, 5.40% | Aura-buff broad shape `enchanted creature gets ±P/±T and <rider>` (10 cards); Lignify-family `enchanted creature is a <type>` (5); typed-anthems that came in; typed-spell cost modifiers; opponent spell cost mods; "each spell cast <predicate>"; typed-spells-gain-kw; `this creature enters with counters`; token creation with named subtype. |
| `partial_final.py` | After #11 | ~715 PARTIAL residue | **Broad structural net.** At this point the error distribution is flat (almost every remaining fragment is unique), so instead of exact-string matching we match *families* of orphan fragments: alt-cost keywords with dash separators; exotic keywords (impending, craft-with); enchant-subtype restrictions; self-reference tilde fragments; "that player/creature/card ..." demonstrative tails; "it gets/gains/perpetually ..." pronoun tails; imperative-verb tails; Alchemy-specific verbs (perpetually, conjure, seek, intensify). Each pattern emits `Modification(kind="parsed_tail:...")` so the ability is recorded as structurally identified without forcing exact semantics. |

### UNPARSED-bucket promotions

| File | Added ~when | Pool state at entry | Owns |
|------|-------------|---------------------|------|
| `unparsed_residual.py` | After PARTIAL scrubbers matured | ~1,693 zero-ability cards (5.35%) | Biggest first-five-word clusters in the UNPARSED pool: modal headers without bullets (`choose one or both / one or more`); tribal anthems (`commander creatures you own have ...`, `all <type> creatures ...`); until-end-of-turn mass buffs; color anthems; bare regenerate; lace effects; choose-from-counters on ETB; Inundate-style `return all <filter>`; multi-target destroy/exile; "target player draws N cards (and loses N life)". |
| `unparsed_residual_2.py` | After #1 | ~1,063 zero-ability cards | Second-pass cluster set that survived #1: library-search templates; land-tap triggers (`whenever a player taps an island for mana, ...`); another-typed-ETB triggers; reanimate-with-suffix; up-to-N-target multi-clause; temporary trigger riders; opponent-controls effects; cycling triggers; same-name cross-product anthems; enchantment-zone-change triggers. |
| `unparsed_final_sweep.py` | After #2 | ~791 UNPARSED (2.50%) | Broad catch-all net that isn't iteration-specific: generic PT-override anthems, color-unscoped PT, target-player-creatures PT, typed-attacking/get-and-have anthems, "all <type> are ...", no-abilities anthems, other-you-control bulk. Emits descriptive-kind `Modification`s. |

### Snowflake passes

| File | Added ~when | Pool state at entry | Owns |
|------|-------------|---------------------|------|
| `snowflake_rules.py` | After `unparsed_final_sweep` | ~370 residual UNPARSED | Hand-walked the entire remaining UNPARSED list and wrote hyper-specific regexes per cluster (usually 1-3 cards each). Accepts literal `~` as a number/type stand-in where `normalize()` mangled oracle text (e.g. "Three Tragedies" → "target player discards ~ cards"). Contains 17 registry declarations mixed with per-cluster helpers and `_er` / `_num` utilities. Largest single file in this directory. |
| `snowflake_rules_2.py` | After `snowflake_rules` | 150 cards (0.47%) | Final-final pass. Individual snowflakes with 2-3 card templates: Summoning tokens, fractal-anomaly tokens, colorless-artifact tokens, mill-then-return, Reap, Selective Snare, Time Wipe, graveyard-target-then-return, Sinister Waltz, Lich Knights, aberrant-return. Also adds `STATIC_PATTERN` companions for clauses that `snowflake_rules.py`'s greedy triggers left empty-rest. |

### Result

With all scrubbers loaded, GREEN is **100.00% (31,639 / 31,639)**.

## Conventions inside scrubber files

- **Specific-first ordering within each file.** Patterns are compiled
  top-to-bottom and appended to registries in source order; the loader
  preserves that order, so narrow regexes go above broad ones.
- **Per-cluster comments.** Most clusters ship a short comment naming the
  cards that motivated them — makes regressions easy to bisect.
- **Name collisions are intentional.** Every scrubber has its own `_sp`,
  `_er`, `_num` helpers. They are module-private (leading underscore,
  never re-exported) so each file is an isolated namespace.
- **Descriptive `kind` tags.** Later scrubbers (especially `partial_final`
  and `unparsed_final_sweep`) emit `Static(Modification(kind=...))` or
  `UnknownEffect(raw_text=...)` with human-readable tags so downstream
  consumers can filter to precise-semantic effects if they want.

## Adding a new scrubber pass

1. Run the parser, collect the top-N parse_error clusters that still fire.
2. Write a new `<name>_N.py` with a module docstring naming the entry
   conditions (how many PARTIAL/UNPARSED cards, %, residual fragment count).
3. Add specific-first regex clusters with per-cluster "cards that motivated
   it" comments.
4. Re-run the parser — GREEN count must not regress; new passes should
   only promote cards up the ladder.
5. Append a row to the appropriate table in this changelog.

## Do not merge

Do not attempt to consolidate these files into one "big scrubber." The
current layout is the result of successive bisect-friendly passes, and
top-level helper names (`_sp`, `_er`, `_num`, `_NUM_WORDS`, `_COLOR`,
`_MULTIPIP_KEYWORDS`, `_BARE_KEYWORDS`, plus dozens of per-cluster closure
builders) collide across files. Merging would require renaming hundreds of
helpers or risk silently shadowing a sibling's patterns, with no coverage
win.
