# PARTIAL Bucket Diagnostic — Structural Analysis

Pool: **2,607 PARTIAL cards** (8.24% of 31,639 real cards) / **2,932 unparsed ability
fragments**. Cards where some abilities parsed but at least one ability remained as
raw text. After 8 `partial_scrubber` passes the long tail is structural, not
pattern-shaped — the top-frequency error prefix repeats only 2–3× per key, and the
fragment space holds 2,838 unique 10-word prefixes against 2,932 total failures.
Adding more regex patterns will yield sub-linear returns.

The categorization below is primary-bucket (one per card, picked from its first
non-`misc` failing ability). The long `misc` bucket is then drilled into.

## Primary-bucket breakdown (cards, non-overlapping)

| Cards | %    | Bucket                          |
|------:|-----:|---------------------------------|
| 1,141 | 43.8 | misc (verb-led effect, fails on target/filter/rider) |
|   358 | 13.7 | novel_trigger                   |
|   146 |  5.6 | variable_amount (`for each X` / `equal to Y`) |
|   135 |  5.2 | you_may_untemplated             |
|   133 |  5.1 | pile_recent_zone (`milled/revealed/exiled this way`, `from among them`) |
|   121 |  4.6 | each_player_distributive        |
|   109 |  4.2 | anthem_filtered (`creatures you control with X have Y`, `enchanted/equipped …`) |
|   104 |  4.0 | demonstrative (`that card/creature/player…` leading) |
|    87 |  3.3 | target_with_adjectives (`target green creature you control gets +2/+2 …`) |
|    85 |  3.3 | pronoun_it_they                 |
|    54 |  2.1 | quoted_grant (`"…embedded ability…"`) |
|    42 |  1.6 | tiered_speed (`1–9 | effect`, Station / Speed / Siege-Rhino-rank format) |
|    26 |  1.0 | bullet_modal (orphan `•` clause) |
|    19 |  0.7 | replacement_instead             |
|    19 |  0.7 | until_prefix                    |
|    15 |  0.6 | spells_static                   |
|    13 |  0.5 | global_player_restriction       |

---

## Root cause taxonomy (what kind of fix does each bucket need?)

### 1. `parse_filter` is shallower than the text demands — **core fix, huge leverage**

The biggest structural weakness. The EFFECT_RULES grammar pairs a verb with a
hand-rolled regex whose object slot is a narrow prefix (`^target creature gets
+N/+N until end of turn`). When the filter grows beyond `creature` — adjective
(`green`), possessor (`you control`), zone (`in an opponent's graveyard`), type
conjunction (`artifact or creature`), or mana-value (`with mana value 3 or
less`) — the effect rule no longer matches even though `parse_filter` itself
would have handled the phrase.

- **Affected cards:** target_with_adjectives (87) + most of misc-verb
  (`return ~`, `destroy ~`, `exile ~`, `put ~`, `choose ~`, `create ~`) where
  the grammar failure is the filter slot, not the verb. A conservative estimate
  is that **~700–900 PARTIAL cards** (roughly 30% of the whole bucket) have at
  least one ability that fails solely because the filter slot was too narrow.
- **Fix shape:** refactor EFFECT_RULES to accept a filter-shaped capture group
  parsed by `parse_filter`, not a flat `[^.]+?` regex. In practice: replace each
  `target creature gets +N/+N` style rule with one that splits `target <filter>
  gets +N/+N <duration>` using a filter-aware sub-parser. Parallel work on
  `parse_filter` to cover compound types (`creature or planeswalker`), zone
  qualifiers (`in an opponent's graveyard`), and cross-zone filters (`card in
  <player>'s graveyard`).
- **Effort:** medium-large. ~400–600 LOC in `parser.py`. Touches every
  verb-rule (destroy, exile, return, bounce, buff, tap, debuff, counters,
  reanimate, tutor). No AST-schema change needed; `Filter` already carries
  enough fields.
- **Examples:** Hunt the Hunter, Custodi Squire, Mythos of Brokkos,
  Phantom Blade, Consign to Dust, Jailbreak, Duneblast, Cytoshape,
  Nissa's Judgment, Rakalite.

### 2. Trigger grammar doesn't generate novel shapes — **core fix, large leverage**

The `_TRIGGER_PATTERNS` list is a fixed enumeration of ~45 handwritten regexes.
Every new Scryfall release adds novel trigger shapes that don't match any of
them. Sampling the 376 novel_trigger cards, we see:
- Filtered actors (`whenever a suspected creature an opponent controls dies
  while ~ is in your graveyard`)
- Compound actors (`whenever this creature or another modified creature you
  control dies`)
- Filtered events (`whenever an opponent discards a noncreature, nonland card`)
- Composite events (`whenever you create or sacrifice a token`, `when you
  remove a counter this way`, `whenever you become the monarch`,
  `when you proliferate`, `whenever you discover`)
- Intervening-if inside the trigger (`whenever an opponent mills a nonland
  card, if this creature is in your graveyard, …`)

- **Affected cards:** **358 cards**, distributed as a long tail (most novel
  trigger prefixes appear on 1–3 cards each).
- **Fix shape:** treat the trigger grammar as productions rather than literal
  patterns. A core `parse_trigger(text) -> (Trigger, rest)` that walks:
  `(when|whenever|at the beginning) <actor-phrase> <event-verb> <event-object>
  [<while-clause>|<intervening-if>]`. Reuse `parse_filter` for actor and
  event-object. Add ~30 missing event verbs (`discovers`, `proliferates`,
  `becomes the monarch`, `sacrifices`, `creates a token`, etc.) as small
  enumerations. Add `Trigger.intervening_if` population (already in schema).
- **Effort:** medium. ~200–300 LOC; one extension file + parser-core trigger
  loop refactor. No AST-schema change (Trigger already has `condition`).
- **Examples:** Waste Not, Mirkwood Bats, Divine Intervention, Snarlfang
  Vermin, One with the Kami, A-Phylath World Sculptor, Costume Closet.

### 3. Pronoun / demonstrative / cross-sentence antecedent — **core grammar,
   large leverage**

PARTIAL fragments that start with `it`, `they`, `them`, `those`, `that
card/creature/player`, `the token`, `the chosen`, `its owner/controller` —
**234 cards**. These sentences were split out from a prior sentence by
`split_abilities`, and now they stand alone. `parse_effect` has ad-hoc rules for
a handful (`it gains X until end of turn`, `return it to …`), but the general
mechanism is missing.

- **Affected cards:** **234 cards** (pronoun_it_they 85 + demonstrative 104 +
  overlap in pile_recent_zone 133 that uses "those cards").
- **Fix shape:** **antecedent tracking**. When a card's abilities are parsed,
  maintain a rolling "most recent noun" stack (`IT=last singular permanent`,
  `THEY=last group`, `THAT_CARD=last zone-card reference`, `THAT_PLAYER=last
  player mentioned`). The split_abilities pre-pass is the natural place — it
  already re-attaches saga chapter prefixes to continuation sentences. Extend
  the same mechanism: when a sentence starts with a demonstrative, either
  (a) substitute the antecedent into the sentence before parsing, OR (b) flag
  the continuation so `parse_effect` takes a context arg. Option (a) is simpler
  and loses no information (the AST stores the original raw).
- **Effort:** medium. ~150–250 LOC in `parser.py` (`split_abilities` extension
  plus a new `_antecedent_stack` class). Light AST touch — add an optional
  `antecedent: str` arg on continuation-like effects (or bake into raw).
- **Examples:** Gluntch the Bestower, Eluge the Shoreless Sea, Unwilling
  Recruit, Spider-Man No More, Storage Matrix, Atomic Microsizer,
  Psychotic Episode (1st sentence parses, 2nd "that player reveals…" doesn't).

### 4. `variable_amount` sub-expressions aren't first-class — **AST + parser
   medium fix**

`for each X` and `equal to Y` are not parser nodes. Rules hard-code them into
specific contexts (there's a `.gets +N/+N for each [^.]+` in the buff rule, but
not in damage, token-count, life-loss, mill, discard, draw, scry, or add-mana
rules). Result: any novel pairing of a standard verb with a variable amount
fails.

- **Affected cards:** **146 cards** primary, 213 cards have at least one
  variable-amount fragment.
- **Fix shape:** introduce an `Amount` AST node (tagged union: `Fixed(int)`,
  `X`, `Variable(source_filter, quantifier)`, `EqualToExpr(expression)`).
  Replace `Union[int, str]` on every effect that carries `count/amount` in
  `mtg_ast.py` with `Amount`. Then write ONE `parse_amount(text)` that
  recognizes the common shapes (`a/one/N/X`, `that much`, `equal to the
  <noun-phrase>`, `for each <filter>`, `half that many rounded up/down`). All
  existing effect rules consume it instead of regexing digits.
- **Effort:** medium-large. Schema change ripples to every numeric leaf
  (Damage.amount, Draw.count, Mill.count, LoseLife.amount, GainLife.amount,
  CounterMod.count, CreateToken.count, Buff.power/toughness — ~15 nodes).
  ~300 LOC new + refactor of ~20 rules. But most of the refactor is mechanical.
- **Examples:** Vial Smasher the Fierce, Essence Pulse, Entrapment Maneuver,
  Goblin Tinkerer, Pest Infestation (`twice x tokens`), Hit // Run,
  A-Phylath World Sculptor.

### 5. Effect-modifier / rider clauses aren't composable — **core fix, medium
   leverage**

Effects accept a single trailing clause today (duration = `until end of turn`
is the only one normalized). Oracle text routinely stacks riders: `… until end
of turn. That creature gets haste`, `… under your control`, `… with a +1/+1
counter on it`, `… tapped and attacking`, `… at the beginning of the next end
step, return it to the battlefield`. When the rider appears, the verb rule's
regex's `(?:\.|$)` terminator bails before matching, so the whole ability fails.

- **Affected cards:** distributed across many buckets; conservatively ~200
  cards where rider clauses are the sole failure.
- **Fix shape:** post-verb rider parser. After a verb rule successfully
  matches its core, keep parsing a rider tail: `[duration]? [controller-flip]?
  [with-counter]? [and <sub-effect>]? [then <sub-effect>]?`. Normalize into
  optional fields on the effect node (`Buff.duration`, new `Reanimate.with_mods`
  already present but under-used, new `Bounce.destination_rider`).
- **Effort:** medium. ~150–250 LOC. Small additive schema fields. Touches the
  verb-rules directly.
- **Examples:** Dance of the Dead (`put creature card onto the battlefield
  tapped under your control and attach this aura to it`), Breaking // Entering,
  Jailbreak, Command the Dreadhorde.

### 6. Recursive "quoted ability" gaps beyond the recent fix — **small fix, small
   leverage**

The recent `parse_quoted_ability` extension handles the common case
(`"Keyword" / "At the beginning of your upkeep, …"`), but **54 PARTIAL cards**
still fail because:
- The quoted body contains multiple sentences (`"When this token dies, you gain
  1 life."` inside a `CreateToken.keywords` slot that isn't schema-supported)
- Multi-clause grants (`gains haste, "this spell costs {1} less to cast," and
  "at the beginning of your end step, sacrifice this creature"`) — a LIST of
  heterogeneous abilities
- Grants attached to tokens at creation time, not to a target (`create a 4/4
  robot token with "this token can't block."`)

- **Affected cards:** **54 cards**.
- **Fix shape:** extend `parse_quoted_ability` to return a LIST of abilities
  (already infrastructure in `parse_ability` that returns list for keyword
  lists). Enrich `CreateToken` with a `granted_abilities: tuple[Ability, ...]`
  field, and `GrantAbility` with `granted_abilities` instead of just
  `ability_name: str`.
- **Effort:** small-medium. Schema touch on 2 AST nodes. ~100 LOC.
- **Examples:** Rat Out, Cybernetica Datasmith, Meandered Towershell,
  Sliver Weftwinder, Takklemaggot, Goblin Trapfinder.

### 7. Tiered / modal-integer-prefixed sub-abilities (Station, Speed, Rad,
   Oathbreaker mission) — **snowflake with schema support**

`1–9 | effect`, `10–19 | effect`, `20 | effect` and friends (Station counters,
Speed mechanic, Mission objectives, Planar Dungeon). The split_abilities step
keeps each tier as a separate "ability," so each becomes its own failed parse.
These aren't standalone abilities — they're clauses inside a single mechanic.

- **Affected cards:** **42 cards** primary (but 103 fragments because each card
  has 3+ tiers).
- **Fix shape:** new AST node `TieredClause(threshold_lo, threshold_hi,
  body: Effect)` and a new ability type `Tiered(resource: str, clauses:
  tuple[TieredClause, ...])`. Pre-pass in `split_abilities` recognizes tiered
  blocks and consolidates.
- **Effort:** small-medium. ~80 LOC + schema additions.
- **Examples:** Earth-Cult Elemental, Diviner's Portent, Arcane Investigator,
  Megaton's Fate, every Station / Speed / Siege-objective card from FIN, BRO,
  MH3, BOT.

### 8. Distributive / `each player` orchestration effects — **medium fix**

`each player searches their library for X, puts it onto the battlefield, then
shuffles` — the shape is a full mini-sequence scoped to `each player`. Current
parser has a couple of rules like `each player draws N cards`, but it can't
recognize "each player does <arbitrary sequence>". Similarly: `each player may
put a land card from their hand onto the battlefield`, `each opponent who can't
discards a card`, `each creature you control enters with an additional +1/+1
counter`.

- **Affected cards:** **121 cards** primary + ~40 additional overlap.
- **Fix shape:** generalize the `each <filter>` quantifier so the BODY is
  `parse_effect(rest)` wrapped in a `ForEach(scope: Filter, body: Effect)` AST
  node (doesn't exist yet). Schema addition: `ForEach` with quantifier and
  inner effect.
- **Effort:** medium. New AST node + ~80 LOC parser. Low risk because
  `parse_filter` already handles `each <filter>` fine.
- **Examples:** Collective Voyage, Pir's Whim, Momentum Breaker, Oversimplify,
  Mystic Barrier.

---

## Top 3 highest-leverage structural fixes

| # | Fix                                             | Est. cards unlocked | Effort |
|---|-------------------------------------------------|--------------------:|--------|
| 1 | `parse_filter`-aware EFFECT_RULES (objects w/ modifiers) | 700–900 | ~500 LOC, parser-core |
| 2 | Productive trigger grammar (actor + event + qualifier + intervening-if) | 350–400 | ~250 LOC, ~1 extension + core |
| 3 | Antecedent tracking for demonstrative/pronoun continuations | 220–250 | ~200 LOC, split_abilities extension |

Together these three would move coverage from 91.75% → **~97.5% GREEN** (roughly
1,400+ cards), without adding a single new effect-shape pattern. They are
structural: each one replaces a class of regex-per-phrase work with one
principled grammar production.

The remaining ~1,200 PARTIAL cards after the top-3 split roughly as: 150 variable
amount (#4), 150 rider-clause (#5), 100 pile-from-recent-zone (a variant of #3),
60 quoted grants (#6), 120 each-player distributive (#8), 42 tiered (#7), plus a
few hundred true snowflakes (Command mechanics, Planar Dungeons, novel 2026
keywords) that will remain per-card handlers.

## Observations / side notes

- **No UNPARSED drift.** The UNPARSED bucket is 3 cards. All failures here are
  PARTIAL — the parser recognizes some abilities on every card. This is good
  news: the mechanisms are intact, only coverage is missing.
- **`split_abilities` is the right place for several fixes.** It's the only
  global-context pass. Antecedent tracking, tiered consolidation, and bulleted
  modal re-consolidation all belong there, not in per-rule regex.
- **The 8 `partial_scrubber` extensions show diminishing returns.** Each new
  scrubber pass captures 50–150 fewer cards than the one before; scrubber_7
  and scrubber_8 together probably netted < 200 cards. Pattern-writing is
  sub-linear at this coverage level. Structural fixes are superlinear.
- **Schema is already mostly adequate.** The AST node set covers 80%+ of what's
  needed. Missing: `Amount` (fix #4), `ForEach` (fix #8), `Tiered` (fix #7),
  and richer `CreateToken.granted_abilities` (fix #6). That's ~4 schema
  changes, not a rewrite.
