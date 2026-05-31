# Huginn 2.0 Guide

> **Audience:** users + operators of the Huginn interaction-discovery system.
> **Status:** Production. Reflects the surface at r60 after PRs #927 (partners + extend), #928 (predict), #932 (Muninn 2.0 hookup), #933 (reverse index), #936 (cycle detection), #937 (Freya integration).
> **Companion docs:** [`docs/architecture/Tool - Huginn.md`](architecture/Tool%20-%20Huginn.md) (1.0 overview), [`docs/architecture/Tool - Freya.md`](architecture/Tool%20-%20Freya.md) (consumer side).

Huginn watches what happens during tournament games and learns which card combinations actually produce meaningful synergies — without anyone hardcoding them. Huginn 2.0 adds three query surfaces (`--partners`, `--extend`, `--predict`) so the learned corpus can answer deck-building questions in real time, plus a reverse-index + cycle-observation layer that lets the system reason about specific card instances rather than just oracle identities.

---

## Architecture

```
┌─────────────────────┐
│ Tournament Runner   │  cmd/hexdek-tournament
│ (Heimdall analytics)│
└──────────┬──────────┘
           │ CoTriggerObservation, CoTriggerNTuple, CycleObservation
           ▼
┌─────────────────────────────────────────────────┐
│            data/huginn/                          │
│   raw_observations.json   (pairwise raw input)  │
│   raw_ntuples.json        (N-card raw input)    │
│   raw_cycles.json         (cycle raw input)     │
│   instance_observations.json   (reverse index)  │
│   instance_interaction_edges.json (reverse idx) │
└──────────┬──────────────────────────────────────┘
           │ huginn --ingest
           ▼
┌─────────────────────────────────────────────────┐
│  Pattern normalization (NormalizePattern)       │
│  card names stripped, verb/resource flow kept   │
└──────────┬──────────────────────────────────────┘
           │
           ▼
┌─────────────────────────────────────────────────┐
│  Tier graduation                                 │
│  Tier 1 → Tier 2 → Tier 3 (CONFIRMED)           │
└──────────┬──────────────────────────────────────┘
           │
           ▼
┌─────────────────────────────────────────────────┐
│  data/huginn/learned_interactions.json (pairs)  │
│  data/huginn/learned_ntuples.json      (N-cards)│
│  data/huginn/tier3_for_freya.json     (export)  │
│  data/huginn/tier3_ntuples_for_freya.json       │
└──────────┬──────────────────────────────────────┘
           │
           ├──────────► Freya (combo + synergy detection)
           │
           ├──────────► --partners <card>     (sister cards)
           │
           ├──────────► --extend <deck.json>  (recommended adds)
           │
           └──────────► --predict <deck.json> (possibility space)
```

**The core pipeline (Huginn 1.0):**

1. Heimdall observes co-triggers + N-tuples + cycles during games and emits them on the game-analysis output.
2. The tournament runner calls `huginn.PersistRawObservations` / `PersistRawNTuples` / `PersistRawCycles` to append to the raw-observation files.
3. An operator (or post-tournament hook) runs `huginn --ingest`. Huginn normalizes each observation to a card-name-free resource-flow `Pattern`, aggregates raw observations sharing the same Pattern into a single `LearnedInteraction`, runs tier promotion, and writes back to `learned_interactions.json`.
4. Promoted Tier 3 interactions are re-exported to `tier3_for_freya.json` for Freya to consume on its next deck analysis.

**Huginn 2.0 additions:**

- **Query layer** (`--partners`, `--extend`, `--predict`) — read-only queries against the learned corpus. No ingest, no mutation.
- **Reverse index** (`reverse_index.go`) — a Provenance lineage from card instance ID → oracle ID → games-of-origin, so the system can answer "which specific shuffle history produced this Doubling Season" beyond "Doubling Season generally does X".
- **Cycle observation persistence** (`raw_cycles.json`) — multi-step interaction loops captured as one observation rather than a string of pairwise pings.
- **Prediction outcomes** (`prediction_outcomes.jsonl`) — Huginn writes per-game predictions out, then ingests the actual outcomes to feed back into pattern confidence.

---

## CLI Reference

The binary lives at `cmd/hexdek-huginn`. Build with `go build ./cmd/hexdek-huginn/` or run with `go run ./cmd/hexdek-huginn/ --<flag>`.

### Maintenance commands (Huginn 1.0)

| Flag | Purpose |
|------|---------|
| `--ingest` | Process new raw observations through the pipeline. Aggregates, normalizes, promotes tiers, and writes back to `learned_interactions.json` + `learned_ntuples.json` + the Freya export files. Idempotent — running twice with no new raw input is a no-op. |
| `--list` | Print every learned interaction grouped by tier. Defaults to top 30 per section (override with `--top`). |
| `--candidates` | Print interactions one observation away from a tier promotion. Useful for predicting "what does Huginn think it'll learn next?" |
| `--stats` | Per-tier counts. |
| `--prune` | Garbage-collect stale Tier 1 / Tier 2 entries (Tier 3 is permanent). Tier 1 entries unused for 200+ games are dropped; Tier 2 entries unused for 500+ games are dropped. Pass `--games-since N` to advance the age counter. |
| `--top N` | Cap per-section render depth on `--list` (default 30). |
| `--dir <path>` | Data directory. Default `data/huginn`. |

Calling Huginn with no flags is equivalent to `--stats --list` (the default discovery view).

### Query commands (Huginn 2.0)

#### `--partners "<card name>"`

> "Which cards has the corpus seen co-firing with X?"

Scans every Tier ≥ `--min-tier` `LearnedInteraction` and pulls the OTHER card from any pair whose card-set contains the query. Ranked by `sum(tier × avg_impact)` across all shared patterns — a card that pairs with the query at multiple Tier 3 patterns scores higher than a card that pairs once.

```
hexdek-huginn --partners "Blood Artist" --min-tier 2 --top 10
```

Output (per hit): partner name, best tier across shared patterns, score, pair count (distinct patterns the pair shares), top 3 patterns.

`--min-tier` defaults to `2` (Tier 2 RECURRING). Drop to `1` to include single-observation noise; raise to `3` for engine-trusted hits only.

`--json` emits the structured `[]PartnerHit` shape (see [Data Model](#data-model)).

#### `--extend <deck.json>`

> "Which cards outside my deck would increase its interaction density?"

Loads a deck JSON (see [Deck JSON shapes](#deck-json-shapes)), then scans every Tier ≥ `--min-tier` learned interaction looking for cards that pair with deck cards. Returns candidates ranked by `sum(tier × avg_impact)` across all pairings, with the count of distinct deck cards each candidate would pair with.

```
hexdek-huginn --extend my_deck.json --min-tier 2 --top 15
```

Output (per hit): candidate card, score, best tier, pair count, the deck cards it would pair with (capped at 5 displayed), top patterns (capped at 3).

`--extend` skips cards already in the deck — it's a recommender, not a coverage report.

#### `--predict <deck.json>`

> "What combos does my deck already fire, and what's it close to firing?"

Different shape than `--extend`: returns a multi-section possibility-space report rather than a flat ranked list.

```
hexdek-huginn --predict my_deck.json --predict-format text
hexdek-huginn --predict my_deck.json --predict-format json | jq .
```

Four sections:

| Section | What it means |
|---------|---------------|
| **Direct pairs** | Tier 3 CONFIRMED pair interactions where BOTH cards are in the deck. These already fire. |
| **Indirect chains** | Tier 3 N-tuples (3+ cards) where the deck has N-1 cards and is missing exactly 1. "1 card away" combo lines. |
| **Cross-tier exposure** | Tier 1 / Tier 2 pair interactions present in the deck. Lower-confidence signal — the engine has seen these fire but doesn't yet trust them. |
| **Tutor reach** | Indirect chains whose missing card appears in the deck's optional `tutor_targets` set. 1 tutor activation away. |

The 2-of-3 / 3-of-4 partial-match threshold is N-1 specifically. Wider gaps (2-of-4, 3-of-5) would flood the report with low-signal noise.

`--predict` short-circuits the other modes — if you pass `--predict` alongside `--ingest`, only `--predict` runs.

### Deck JSON shapes

Both `--extend` and `--predict` accept JSON deck files. The `--predict` shape is documented inline at `cmd/hexdek-huginn/predict.go`:

```json
{
  "deck_name":     "yawgmoth_aristocrats",
  "commander":     "Yawgmoth, Thran Physician",
  "cards":         ["Blood Artist", "Phyrexian Altar", "Gravecrawler", ...],
  "tutor_targets": ["Worldgorger Dragon", "Demonic Tutor"]
}
```

- `cards` is the only required field.
- `commander` surfaces in the report header.
- `tutor_targets` (optional) is the set of cards the deck can reach via its tutor package. Without it, the tutor-reach section stays silent.

`--extend` accepts a slightly looser shape (`huginn.DeckJSON` in `internal/huginn/recommender.go`):

```json
{
  "name":      "yawgmoth_aristocrats",
  "cards":     ["Blood Artist", "Phyrexian Altar", ...],
  "library":   ["..."],           // alternative to cards
  "commander": "Yawgmoth, Thran Physician"
}
```

`LoadDeckJSON` accepts either `cards` OR `library`; one or the other must be populated. The commander is folded into the name set automatically.

---

## Data Model

### Tier Lifecycle

| Tier | Constant | Promotion criteria | Pruning |
|------|----------|--------------------|---------|
| 1 OBSERVED | `huginn.TierObserved` (=1) | First observation of a normalized pattern | Dropped after 200 games without recurrence |
| 2 RECURRING | `huginn.TierRecurring` (=2) | ≥3 observations AND ≥2 unique decks | Dropped after 500 games without recurrence |
| 3 CONFIRMED | `huginn.TierConfirmed` (=3) | ≥5 observations AND avg impact ≥5.0 | Permanent. Re-exported on every ingest. |

Promotion is **never demotion**. Once an interaction reaches a tier, it stays at or above that tier for the rest of its lifetime.

Tier 3 is the export gate. Only Tier 3 interactions appear in `tier3_for_freya.json` and `tier3_ntuples_for_freya.json` — Freya intentionally doesn't see noise.

### Core types

Defined in `internal/huginn/huginn.go` + `internal/huginn/ntuple.go`:

```go
type LearnedInteraction struct {
    Pattern          string   // normalized resource flow ("creature_etb_into_opp_lose_life")
    ExampleCards     []string // up to 10 example pairs, "CardA + CardB" format
    ObservationCount int
    UniqueDeckCount  int
    AvgImpactScore   float64
    TotalImpact      float64
    FirstSeen, LastSeen string
    Tier             int
    GamesSinceLastSeen int
}

type LearnedNTuple struct {
    Cards            []string // sorted unique cards in the combo
    NormalizedKey    string   // canonical cards key for dedup
    ObservationCount int
    UniqueDeckCount  int
    AvgImpactScore   float64
    TotalImpact      float64
    Tier             int
    GamesSinceLastSeen int
}
```

### On-disk files

All under `data/huginn/` (configurable via `--dir`):

| File | Contents | Writer | Reader |
|------|----------|--------|--------|
| `raw_observations.json` | Append-only pairwise co-trigger observations | Tournament runner (Heimdall) | `--ingest` |
| `raw_ntuples.json` | Append-only N-card co-firing observations | Tournament runner | `--ingest` |
| `raw_cycles.json` | Append-only multi-step cycle observations | Tournament runner | (cycle ingest) |
| `learned_interactions.json` | Tier-graduated pairwise patterns | `--ingest` | `--list`, `--partners`, `--extend`, `--predict`, Freya |
| `learned_ntuples.json` | Tier-graduated N-card patterns | `--ingest` | `--list`, `--predict`, Freya |
| `tier3_for_freya.json` | Tier 3 pairs + inferred chains for Freya | `--ingest` (Tier 3 hook) | Freya |
| `tier3_ntuples_for_freya.json` | Tier 3 N-tuples for Freya | `--ingest` | Freya |
| `instance_observations.json` | Per-instance Provenance lineage (reverse index) | `huginn.AppendInstanceObservations` | `LookupReverseIndex` |
| `instance_interaction_edges.json` | Per-instance interaction edges | `huginn.AppendInstanceInteractionEdges` | reverse-index queries |
| `prediction_outcomes.jsonl` | One outcome per prediction, fired vs. miss | `huginn.RecordPredictionOutcomes` | confidence calibration |

The `.jsonl` extension on `prediction_outcomes` is deliberate — that file is append-only newline-delimited so writes are non-blocking and crash-safe.

### Pattern normalization

`huginn.NormalizePattern(effectPattern)` strips concrete card names and keeps only the resource-flow shape. Example transformations:

| Raw effect pattern | Normalized pattern |
|--------------------|--------------------|
| `Blood Artist enters_battlefield, each opponent loses 1 life` | `creature_etb_into_opp_lose_life` |
| `Phyrexian Altar tap, add one mana` | `artifact_tap_into_mana` |

Two interactions involving completely different cards but the same resource flow collapse into the same `LearnedInteraction` — that's the abstraction that lets Huginn graduate "patterns of play" rather than specific card pairings. The pair-specific information lives in `ExampleCards`.

---

## Freya Integration

Freya consumes Huginn output at deck-analysis time:

1. Freya reads `data/huginn/tier3_for_freya.json` (pair-level) and `data/huginn/tier3_ntuples_for_freya.json` (N-card).
2. Pairs and N-tuples are merged into Freya's combo + synergy detection pipeline alongside the curated `KnownCombos` database.
3. Freya's analysis surfaces Huginn-derived synergies in `report.Synergies`, `report.Determined`, and the deck-profile's `SynergyClusters`.
4. The Decks-screen panel renders Huginn-derived synergies with a distinct tag so users can see which combos came from the curated set vs. emergent observation.

PR #937 (HUGINN 2.0 Worker F) wired the reverse-index and cycle observations into Freya's strategy-bridge layer, so the Hat MCTS evaluator can weight Tier 3 N-tuples by AvgImpactScore when scoring board states.

---

## Troubleshooting

### "Tier 3 promotions look stuck"

Confirm raw observations are actually arriving. From the data dir:

```
jq '. | length' data/huginn/raw_observations.json
```

If that's flat across multiple tournament runs, the tournament runner isn't persisting observations — check that the tournament was launched with `--analytics-enabled` (the flag that gates `analytics.GameAnalysis` population). Without it, `CoTriggerObservations` is empty and Huginn has nothing to ingest.

### "Partners returns zero hits for a card I know is in the corpus"

Three common causes:

1. **Casing.** Huginn matches case-insensitively, but `--partners` echoes back whatever string you passed. If the query is `"blood artist"` and partners is empty, try `"Blood Artist"` and see if Huginn 1.0 (`--list`) actually contains it.
2. **Tier floor.** `--min-tier` defaults to 2. If the card has only Tier 1 observations, drop the floor: `--partners "Card" --min-tier 1`.
3. **Pattern doesn't include this card.** Huginn matches against `ExampleCards`, which is capped at 10 examples per LearnedInteraction. A card that pairs at a pattern with 50+ other example pairs may not appear in the first 10. The cap is intentional (keeps the corpus bounded) — `--candidates` is the right tool for "is this pattern observed at all".

### "Predict surfaces direct pairs but no indirect chains"

Indirect chains come from `learned_ntuples.json`, NOT `learned_interactions.json`. If your tournament runner doesn't emit `CoTriggerNTuple` observations (because Heimdall wasn't configured to capture them), the N-tuple file stays empty and only Direct + Cross-tier sections populate.

Check:

```
jq '. | length' data/huginn/learned_ntuples.json
```

If 0, the N-tuple pipeline is dark. Re-run a tournament with the `--ntuples-enabled` Heimdall flag.

### "Ingest crashes or produces zero promotions"

Run `--stats` first. If `Total: 0`, the learned file is empty — first-time ingest with zero raw observations is a no-op (not an error). If the learned file has entries but no Tier 3, increase observation count: Tier 3 requires ≥5 observations + avg impact ≥5.0. The impact floor catches "two cards happened to share a window but didn't change game state" false positives.

For impact-floor calibration, the relevant constants are `tier3MinImpact = 5.0` in `internal/huginn/huginn.go`. Lowering catches lower-impact synergies (Soldier of the Pantheon hate, marginal removal); raising prunes noise.

### "Predict shows tutor reach entries but the missing card isn't in my tutor package"

`tutor_targets` in the deck JSON is whatever you pass it — Huginn doesn't validate against the deck's actual tutors. If you're populating it from Freya's tutor analysis, make sure the Freya analysis is current; an old tutor list will report stale reach.

### "Reverse index lookups return empty for instance IDs I just saw"

`LookupReverseIndex` lazily builds the index on first call. If you've appended observations since the last lookup, the cached index is stale. Call `huginn.ResetReverseIndex()` between writes and reads in tests; in production the call is one-shot per process.

### "After --prune, Tier 2 entries I expected to keep are gone"

Pruning uses `GamesSinceLastSeen` as the age signal, which advances by the `--games-since` argument on each ingest. If `--games-since` has been over-counting (e.g. ingest run with the full corpus game count when it should have been the delta-since-last-ingest), Tier 2 entries age out faster than they should. The fix is operational, not code — make sure `--games-since` reflects games played BETWEEN ingests, not total games ever.

---

## Where to look next

| Need | File |
|------|------|
| Tier promotion algorithm | `internal/huginn/huginn.go` → `Ingest()` |
| Pair query implementation | `internal/huginn/recommender.go` → `partnersFromInteractions()` |
| Predict implementation | `cmd/hexdek-huginn/predict.go` → `Predict()` |
| Pattern normalization rules | `internal/huginn/huginn.go` → `NormalizePattern()` |
| Reverse-index provenance | `internal/huginn/reverse_index.go` |
| Cycle observation persistence | `internal/huginn/huginn.go` → `PersistRawCycles()` |
| Freya integration test fixture | `cmd/hexdek-huginn/integration_test.go` |
| Architecture overview (1.0) | `docs/architecture/Tool - Huginn.md` |
