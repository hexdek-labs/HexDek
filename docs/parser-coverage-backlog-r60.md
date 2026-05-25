# Parser Coverage Backlog — R60

High-impact uncovered cards prioritized by deck-frequency across the 1,616
deck files under `data/decks/**`. Cross-references `parser-coverage`'s
classification (MISSING / EMPTY_AST / PARTIAL) with how often each card
appears in real decks, then runs `--action-list` per card to produce a
concrete scaffold checklist.

This is a **backlog**, not a fix: each card here is something the parser
or per-card layer needs to land work for. No code changes ship with this
doc.

## Methodology

1. `parser-coverage` classifies every Scryfall oracle entry against
   `data/rules/ast_dataset.jsonl`.
2. A one-off cross-ref script (`scripts/find_high_freq_uncovered/`) walks
   every deck file, counts how many decks each uncovered card appears
   in, sorts by frequency, and emits the top N.
3. For each, `go run ./cmd/parser-coverage --action-list "<name>"`
   produces the per-card TODO checklist captured below.

Reproduce:

```sh
go run ./scripts/find_high_freq_uncovered --top 30
go run ./cmd/parser-coverage --action-list "Underground Sea"
```

## Headline

| Metric | Value |
|---|---:|
| Decks scanned | 1,616 |
| Uncovered cards (any class) in oracle corpus | 3,758 |
| Uncovered cards appearing in ≥1 deck | **12** |
| Coverage of deck-relevant cards | very high (only 12 cards in real decks miss) |

The deck corpus's parser coverage is in good shape — out of thousands of
unique cards across 1,616 decks, only twelve uncovered cards show up at
all, and ten of them collapse to a single scaffold pattern (ABU dual
lands with reminder-text-only oracle bodies).

## Top 12 uncovered cards by deck frequency

| Decks | Class | Card | Scaffold group |
|---:|---|---|---|
| 103 | EMPTY_AST | Underground Sea | A — Dual lands |
| 94  | EMPTY_AST | Volcanic Island | A — Dual lands |
| 91  | EMPTY_AST | Tropical Island | A — Dual lands |
| 78  | EMPTY_AST | Bayou           | A — Dual lands |
| 73  | EMPTY_AST | Tundra          | A — Dual lands |
| 72  | EMPTY_AST | Scrubland       | A — Dual lands |
| 69  | EMPTY_AST | Badlands        | A — Dual lands |
| 58  | EMPTY_AST | Taiga           | A — Dual lands |
| 53  | EMPTY_AST | Savannah        | A — Dual lands |
| 49  | EMPTY_AST | Plateau         | A — Dual lands |
| 43  | EMPTY_AST | Dryad Arbor     | B — Creature land |
| 1   | MISSING   | Ormacar, Relic Wraith | C — Missing ingest |

## Scaffold groups

### Group A — ABU Dual Lands (10 cards, 740 deck-appearances)

All ten Revised dual lands share the same oracle-text shape: a single
parenthetical reminder line documenting the mana ability they pick up
intrinsically from their basic-land subtypes (CR 305.6).

Example (`Underground Sea`):

> Oracle text: `({T}: Add {U} or {B}.)`
> Type: `Land — Island Swamp`

The Python parser correctly drops parenthetical reminder text, leaving
zero abilities in the AST. The reason these classify EMPTY_AST instead
of OK_VANILLA is that `parser-coverage`'s vanilla check requires
`type_line` to contain "Basic" — duals are `Land — Island Swamp`, not
`Basic Land — Island`, so they fall through to EMPTY_AST.

**Scaffold need (shared across all 10):**

- [ ] Decide whether duals should classify as OK_VANILLA (treat
      reminder-text-only lands with intrinsic-type mana as covered by
      basic-land scaffolding) or whether the AST should explicitly
      emit a `MultiColorBasicTypeMana` ability node.
- [ ] If the former: extend `parser-coverage`'s vanilla check to also
      accept "Land — X Y" where X and Y are basic land subtypes.
- [ ] If the latter: extend the Python parser (`scripts/mtg_ast.py`)
      to emit a synthetic mana ability for any land whose type line
      contains one or more basic land subtypes, even when oracle text
      is only reminder.
- [ ] Verify the engine already grants intrinsic mana from
      basic-land subtypes (CR 305.6) — if so, the AST gap is purely
      cosmetic and the right fix is the classifier.

**Per-card action lists** (all 10 produce the same TODO list; only
the matched snippet varies by color pair):

#### Underground Sea (`Land — Island Swamp`)

> Oracle text: `({T}: Add {U} or {B}.)`

- [ ] Add activated-ability handler (`{cost}: effect`)
  - _matched:_ `({T}: Add {U} or {B}.)`
- [ ] Add mana-ability handler (`{T}: Add …`)
  - _matched:_ `{T}: Add`
- [ ] Extend Python parser to produce non-empty abilities for this oracle text
  - _matched:_ EMPTY_AST: AST entry exists but contains zero abilities despite non-trivial oracle text

#### Volcanic Island (`Land — Island Mountain`)

> Oracle text: `({T}: Add {U} or {R}.)`

Same scaffold as Underground Sea.

#### Tropical Island (`Land — Forest Island`)

> Oracle text: `({T}: Add {G} or {U}.)`

Same scaffold as Underground Sea.

#### Bayou (`Land — Swamp Forest`)

> Oracle text: `({T}: Add {B} or {G}.)`

Same scaffold as Underground Sea.

#### Tundra (`Land — Plains Island`)

> Oracle text: `({T}: Add {W} or {U}.)`

Same scaffold as Underground Sea.

#### Scrubland (`Land — Plains Swamp`)

> Oracle text: `({T}: Add {W} or {B}.)`

Same scaffold as Underground Sea.

#### Badlands (`Land — Swamp Mountain`)

> Oracle text: `({T}: Add {B} or {R}.)`

Same scaffold as Underground Sea.

#### Taiga (`Land — Mountain Forest`)

> Oracle text: `({T}: Add {R} or {G}.)`

Same scaffold as Underground Sea.

#### Savannah (`Land — Forest Plains`)

> Oracle text: `({T}: Add {G} or {W}.)`

Same scaffold as Underground Sea.

#### Plateau (`Land — Mountain Plains`)

> Oracle text: `({T}: Add {R} or {W}.)`

Same scaffold as Underground Sea.

### Group B — Creature lands (1 card, 43 deck-appearances)

#### Dryad Arbor (`Land Creature — Forest Dryad`)

> Oracle text: `(This land isn't a spell, it's affected by summoning sickness, and it has "{T}: Add {G}.")`

- [ ] Add activated-ability handler (`{cost}: effect`)
  - _matched:_ `(This land isn't a spell, it's affected by summoning sickness, and it has "{T}: Add {G}.")`
- [ ] Add mana-ability handler (`{T}: Add …`)
  - _matched:_ `{T}: Add`
- [ ] Extend Python parser to produce non-empty abilities for this oracle text
  - _matched:_ EMPTY_AST: AST entry exists but contains zero abilities despite non-trivial oracle text

**Scaffold need:**

Dryad Arbor is doubly anomalous: its only oracle text is a parenthetical
reminder, AND it has the dual `Land Creature` type. The fix is the same
as Group A — either classify as OK_VANILLA via an extended type-line
heuristic, or emit a synthetic mana ability node from the Forest
subtype. The "Land Creature" supertype combination should already be
handled by the engine (summoning-sickness application + tap-for-mana),
so this is a parser-output-only gap.

### Group C — Newly printed cards missing from ingest (1 card, 1 deck-appearance)

#### Ormacar, Relic Wraith (`Legendary Creature — Elf Wraith`)

> Oracle text:
> ```
> Vigilance, menace, lifelink
> Precious (You can have two commanders if the other one is a legendary noncreature artifact.)
> As long as you control your Precious, Ormacar gets +X/+X, where X is the mana value of your Precious.
> ```

- [ ] Re-ingest card via Thor (no entry in ast_dataset.jsonl)
  - _matched:_ MISSING: parser pipeline never produced an AST for this card

**Scaffold need:**

Single MISSING card — likely a set printed after the cached AST dataset
was built. The action-list tool emits only the "Re-ingest" item because
the MISSING arm short-circuits its pattern detection on type_line +
oracle text. Manually inspecting the oracle reveals three real scaffold
needs the dev should expect once Thor re-runs:

- [ ] Keyword-stew handler for the first line (`Vigilance, menace,
      lifelink`) — three printed keywords.
- [ ] **New mechanic: Precious.** This is a "partner-like" alternate
      commander variant ("two commanders if the other is a legendary
      noncreature artifact"). Engine needs a Precious-keyword analogue
      to Partner / Friends Forever / Choose a Background — likely a
      new entry in `internal/gameengine/partner_variants.go` (or
      wherever the partner family lives), plus a commander-pair
      legality check that accepts a legendary noncreature artifact as
      the "Precious" slot.
- [ ] Static buff with X derived from commander mana value
      (`gets +X/+X, where X is the mana value of your Precious`) —
      similar shape to commander-tax X buffs; reuse whichever existing
      primitive computes "mana value of named permanent".

The Precious mechanic is the most interesting backlog item in this
batch — it's a real engine extension, not just a parser gap.

## Action-list tool — known blind spots surfaced by this batch

Running `--action-list` against MISSING-class cards short-circuits at
the "Re-ingest" item even when the oracle text would otherwise trigger
pattern detection. Ormacar's case (above) shows that the developer must
read the oracle text manually for MISSING cards. Two improvements would
help here, but they're tool improvements rather than parser/engine work:

1. For MISSING cards, still run the scaffold pattern detector so the
   developer sees what the engine will need _once_ Thor re-runs.
2. Add a static-buff detector (`gets +N/+N`, `gets +X/+X`) — currently
   the rule catalog covers triggers, activated, modal, replacement,
   damage, lifegain, etc., but not power/toughness modification.

Logging these here so they don't get lost; not in scope for this
backlog doc.

## Priority order

1. **Group A** — 740 deck-appearances behind a single classifier fix.
   Highest-leverage win in the batch. Recommended first work item.
2. **Group B** — Dryad Arbor is one card but uses every Group A
   primitive plus the creature-land tap-rule check, so it's a useful
   sanity case once Group A lands.
3. **Group C** — Ormacar is the only deck-relevant MISSING card and
   would unblock Precious-commander deck imports. Engine work is the
   dominant cost; parser side is just a `thor` re-run.

## Reproducing

```sh
# 1. Refresh the data files (or symlink them in from the main repo).
ls data/rules/oracle-cards.json data/rules/ast_dataset.jsonl

# 2. Recompute the high-frequency uncovered list.
go run ./scripts/find_high_freq_uncovered --top 30

# 3. Per-card action lists.
for c in "Underground Sea" "Dryad Arbor" "Ormacar, Relic Wraith"; do
    go run ./cmd/parser-coverage --action-list "$c"
done
```
