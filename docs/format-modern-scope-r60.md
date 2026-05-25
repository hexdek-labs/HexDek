# Adding Modern Format Support — Scoping Document

Status: **survey only — no engine changes yet.** This document is the
pre-implementation scope so the eventual Modern-support PR has a
clear delta to land against.

The audit covers what HexDek today assumes is Commander and what
would have to fork to support a 60-card 1v1 constructed format
(Modern as the concrete first target; the same delta unlocks
Standard / Pioneer / Legacy / Pauper with banlist swaps).

---

## Why Modern first

Modern is the highest-value non-Commander target:

- Large existing player base + deep card pool → broad coverage value
  for the AI evaluator and parser-gap audits.
- Banlist is meaningfully different from Commander (Sensei's
  Divining Top, Field of the Dead, Bridge from Below, etc.) — drives
  banlist plumbing that other formats reuse.
- 1v1 structure simplifies several engine paths (no commander tax,
  no 21-damage SBA, no command zone) — easier on-ramp than Pauper or
  Legacy, which share Modern's structural shape but have larger
  card-pool nuances.

The Modern engine work is mostly *removal* of Commander assumptions
plus *additive* sideboard/best-of-three plumbing — it does not need
a parallel rules engine.

---

## What HexDek hardcodes today

A grep of the runtime found **27 distinct Commander assumptions**
across `internal/gameengine` and `internal/tournament`. They cluster
into seven categories:

### 1. Starting life

- `gameengine/state.go:930` — fresh Seats default to `StartingLife = 20`.
- `gameengine/commander.go:107` — `SetupCommanderGame` overwrites to
  40.

**Modern requires**: 20 starting life. Default is already correct;
just need a non-Commander setup path that doesn't call
`SetupCommanderGame`.

### 2. Command zone + commanders

- `gameengine/state.go` — `Seat.CommandZone`, `Seat.CommanderNames`,
  `Seat.CommanderCastCounts`, `Seat.CommanderTax`,
  `Seat.CommanderDamage`.
- `gameengine/commander.go:199` — `registerCommanderZoneReplacement`
  installs the §903.9b zone-redirect replacement effect.
- `gameengine/sba.go:1680` — §704.6c 21+ commander-damage SBA.
- `gameengine/sba.go:1746` — §704.6d zone-return SBA.
- `gameengine/costs.go:415` — commander tax adjustment on cast.

**Modern requires**: none of this. All of it is gated on
`gs.CommanderFormat`, so leaving that flag `false` short-circuits
every commander-specific helper. The fields stay on `Seat` unused —
fine for now; later refactor can move them into a `CommanderState`
sub-struct keyed by format.

### 3. Deck construction rules

| Rule                  | Commander                       | Modern                         |
|-----------------------|---------------------------------|--------------------------------|
| Deck size             | exactly 100 (99 + 1 commander)  | minimum 60                     |
| Copy limit            | 1 of each non-basic             | up to 4 of each non-basic      |
| Color identity        | enforced from commander         | none                           |
| Sideboard             | none                            | up to 15 cards                 |
| Companion             | hand-built via `Companion:` line | sideboard-resident             |

- `deckparser/deckparser.go:328` — `TournamentDeck` has
  `CommanderName` + `CommanderCards` + `Library`. No sideboard field.
- `deckparser/deckparser.go:404` — section parser explicitly DROPS
  `sideboard`, `maybeboard`, `considering`, etc.
- No copy-count cap is enforced today (Commander's "1 of each" is
  implicit from the input format).
- No color-identity validation — `internal/moxfield/validate.go`
  reads Scryfall legality status but doesn't cross-check colors.

**Modern requires**:
- New field `TournamentDeck.Sideboard []*gameengine.Card` (size 0..15).
- Section parser stops dropping `sideboard:`; routes cards to the
  new field.
- Deck-size validator: ≥ 60 cards in main + ≤ 15 cards in sideboard.
- Copy-count validator: ≤ 4 of any non-basic across main + sideboard
  combined.

### 4. Banlist + format legality

- `oracle/scryfall.go:83` — `Card.Legalities` is a `map[string]string`
  with Scryfall-provided slugs ("legal", "banned", "not_legal",
  "restricted"). The map is populated by the corpus loader from
  Scryfall bulk data.
- `moxfield/validate.go:61` — `ValidateFormat(format, cards)` already
  walks `Legalities[format]` and returns a `FormatReport`. Supports
  any format slug Scryfall ships — "modern", "legacy", "vintage",
  "pioneer", "standard", "pauper", "historic", "alchemy" all
  recognized; no Commander-specific assumption inside.

**Modern requires**: nothing. The banlist infrastructure is
generic. The CLI / tournament runner just needs a `--format modern`
flag that calls `ValidateFormat("modern", ...)` on deck load.

### 5. Mulligan

- `tournament/turn.go:563` — `RunLondonMulligan` implements CR §103.5
  London mulligan: draw 7, mulligan optional, on keep put N cards
  on bottom where N = mulligans taken.

**Modern requires**: London mulligan is the current official rule
across all formats. **No change.**

### 6. Turn / game limits

- `tournament/config.go:194` — `defaultMaxTurns = 80` (Commander
  multiplayer cap).
- `tournament/config.go:198` — `DefaultMaxTurns` is the exported
  constant.

**Modern requires**: lower cap (~ 25–30 turns) since 1v1 games
resolve faster. Trivially configurable per `TournamentConfig`;
suggest a `DefaultMaxTurns60` constant for the new format default.

### 7. Seat count + multiplayer assumptions

- `tournament/config.go:50` — `NSeats` configurable; 4 is the
  documented Commander default.
- `gameengine/multiplayer.go` — APNAP turn ordering, multi-seat
  combat targeting, life-loss-bounded-by-opponents. All scale to
  `NSeats=2`.

**Modern requires**: `NSeats=2`. Already supported (`TestSmallTournament`
in `runner_test.go` runs 2-seat games today). Combat targeting
collapses naturally — no engine change needed.

---

## Best-of-three matches (Modern's actual play structure)

Constructed Modern tournaments are best-of-three with sideboarding
between games. HexDek today runs one game per "match" — there is no
multi-game-with-sideboarding loop, and the AI has no notion of
sideboarding decisions.

This is the **biggest scope item** by an order of magnitude.

Components:

1. **Match controller**: `RunBestOfThree(deckA, deckB) → MatchResult`.
   Plays games 1..3, alternating play/draw on the loser of the prior
   game (CR §100.5), stopping when one side wins 2.
2. **Sideboard swap interface**: between games, each AI must pick
   ≤ 15 cards to swap between main + sideboard. New
   `Hat.ChooseSideboard(deck, opponentDeck, gameResults) []SwapPair`
   method.
3. **AI sideboarding logic**: the meaningful intelligence work. Two
   tractable starting points:
   - Heuristic: predefined sideboard guides per archetype
     (graveyard hate vs reanimator, artifact hate vs affinity, etc).
     Lift from Freya's `hoserDB` which already tags hosers by
     condition.
   - Learned: collect win/loss data per sideboard-swap pattern and
     gradient-descend swap rules. Months of work post-Heuristic.
4. **Game-1 deck identification**: between games 1 and 2, the AI
   only knows what cards the opponent revealed. The heuristic-side
   loop should accept "perceived archetype" from 3rd-Eye intelligence
   as the sideboard-decision input, not the actual opponent decklist.

**Initial scope recommendation**: ship Modern with the match loop
running best-of-ONE (one game per match, no sideboarding) and add
best-of-three + sideboarding as a follow-up. The single-game path
delivers banlist + deck-size + 60-card validation immediately;
sideboarding is a separable initiative.

---

## Format abstraction surface

Rather than scatter Modern-specific conditionals through the engine,
the suggested shape is a `Format` enum + `FormatRules` struct that
encapsulates the parameters listed above:

```go
type Format string

const (
    FormatCommander Format = "commander"
    FormatModern    Format = "modern"
)

type FormatRules struct {
    StartingLife       int
    MinDeckSize        int
    MaxDeckSize        int     // 0 = no cap
    MaxCopiesPerCard   int     // 0 = unlimited (commander); 4 = modern
    AllowSideboard     bool
    MaxSideboardSize   int
    UsesCommandZone    bool
    MaxTurns           int
    MaxMatchGames      int     // 1 = best-of-one; 3 = best-of-three
    BanlistFormatSlug  string  // passed to moxfield.ValidateFormat
}

func DefaultFormatRules(f Format) FormatRules { ... }
```

`SetupCommanderGame` becomes `SetupFormatGame(gs, FormatRules,
decks)`, dispatches to either the existing commander setup or a new
`setupConstructedGame` path that just stamps life + library and
skips command-zone bookkeeping.

`TournamentConfig` gains a `Format` field; readers default to
`FormatCommander` for back-compat.

---

## Per-card engine gaps to expect

The Commander card pool overlaps Modern substantially, but Modern's
**signature mechanics** that aren't load-bearing for Commander will
likely surface parser gaps:

- Cascade (Bloomburrow + Crashing Footfalls, Living End, Violent
  Outburst) — present in engine, but heavily Modern-specific.
- Storm (Past in Flames, Grapeshot, Pyromancer Ascension chains) —
  partially covered.
- Tribal payoffs at 4-of density (Goblin Lord, Lord of Atlantis
  stacking) — Commander rarely stacks 4 copies of a single anthem.
- Modern Horizons evoke-elementals + companion abuse — companion
  validation exists but evoke + ETB cascades are not heavily fuzzed.

**Recommendation**: after the format scaffold lands, run a 5000-game
Loki gauntlet on a Modern banlist + sample tier-list and triage the
top parser gaps. Existing Loki invariants (ZoneConservation,
CardIdentity, AttachmentConsistency) work format-agnostically.

---

## Tournament runner integration

All seven bracket styles (rotate / pool / lazy-pool / round-robin /
swiss / double-elim / balanced-pool) operate on
`TournamentDeck` + per-game-driver — they don't know Commander
specifics today. The only Commander-specific call site is
`tournament/runner.go:440` which calls `RunLondonMulligan` (London
mulligan, format-neutral) followed by `SetupCommanderGame`.

The cleanest hook is one site:

```go
// In tournament/runner.go runOneGame:
if cfg.Format == FormatCommander {
    SetupCommanderGame(gs, cmdDecks)
} else {
    setupConstructedGame(gs, cfg.Format, decks)
}
```

No other tournament code needs to change. Standings, ELO, TrueSkill,
seat rotation, pod formation, ALL bracket styles work as-is.

---

## Effort estimate

Bucketed by deliverable, assuming the engine work happens before
sideboarding:

| Deliverable                            | Effort   | Blockers                       |
|----------------------------------------|----------|--------------------------------|
| Format enum + FormatRules struct       | ~1 day   | None                           |
| Constructed setup path (vs commander)  | ~1 day   | Above                          |
| 60-card deck validation                | ~1 day   | Above + sideboard parser       |
| Sideboard section parser               | ~0.5 day | None                           |
| Banlist hook in deck loader            | ~0.5 day | Above (uses ValidateFormat)    |
| Copy-count validator (≤ 4)             | ~0.5 day | Sideboard parser               |
| `--format modern` CLI flag             | ~0.5 day | All above                      |
| Test corpus: 4–8 Modern decks          | ~0.5 day | Banlist hook (legality check)  |
| Tournament smoke test (Modern decks)   | ~0.5 day | All above                      |
| Loki Modern gauntlet (parser-gap audit)| ~2 days  | Smoke test passes              |
| **MVP total (best-of-one)**            | **~7 days** |                              |
| ChooseSideboard hat interface          | ~2 days  | MVP                            |
| Heuristic sideboarding (hoserDB reuse) | ~3 days  | Above                          |
| RunBestOfThree match controller        | ~2 days  | Above                          |
| **Best-of-three total**                | **+7 days** |                              |

Realistic project envelope: **~1–1.5 weeks for MVP single-game
Modern**, plus another **~1–1.5 weeks for best-of-three +
sideboarding** as a separable follow-up.

---

## Out of scope for this PR

This document does NOT propose:
- A parallel rules engine (HexDek's engine is format-neutral
  enough; we extend, not fork).
- Pauper or Legacy support in the same scope. Pauper inherits
  Modern's frame; Legacy adds restricted-list semantics that need
  more thought.
- Limited formats (Draft, Sealed). Need a draft-pool generator,
  draft-pick AI, and Sealed pool sampling — separate initiative.
- A "format meta" surface (deck tier lists, top-decks-this-week).
  Out of engine scope; downstream content concern.
