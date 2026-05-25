# Freya — Commander Spellbook Import

> **TL;DR — Freya's combo database imports the full Commander Spellbook variants JSON, dedupes against the 58 hand-curated entries (curated wins on conflict), and surfaces the merged set through a single `allKnownCombos()` accessor. CLI flags `--spellbook` (cache path) and `--spellbook-fetch` (URL refresh). All offline by default — Freya runs fine without a fetch and never auto-refreshes during deck analysis.**

| | |
|---|---|
| **Feature branch** | `dev/freya-spellbook-import-r60` |
| **Code** | `cmd/hexdek-freya/spellbook_import.go` (308 lines) |
| **Tests** | `cmd/hexdek-freya/spellbook_import_test.go` (341 lines, 11 regressions, all passing) |
| **CLI flags** | `--spellbook <cache>` (default `data/rules/spellbook.json`), `--spellbook-fetch <url>` (default empty — no auto-fetch) |
| **Default URL** | `https://json.commanderspellbook.com/variants.json` |
| **Curated baseline** | 58 hand-authored `KnownCombos` entries in `cmd/hexdek-freya/analysis.go` |
| **Curated entries always win** on dedupe conflicts |
| **Cache file** | `data/rules/spellbook.json` (gitignored alongside the AST corpus + oracle bulk data) |

---

## Why this exists

Freya's curated `KnownCombos` is the 58-entry hand-authored list of the most-impactful Commander combos — Worldgorger Dragon + Animate Dead, Thassa's Oracle + Demonic Consultation, Heliod + Walking Ballista, etc. The curation captures rich semantics (outlets, stops, color identity, classification) that informs the Hat's strategy bridge and Freya's deck-coaching layer.

But 58 entries is a small fraction of the ~3,000 catalogued combos in Commander Spellbook's public dataset. For long-tail combo recognition — recognizing that a specific deck packs a Pili-Pala loop or a Devoted Druid line even if the curated list doesn't include it — Freya needs the broader database.

The Spellbook import bridges the two: the curated set drives the high-precision signals; the imported set fills in the long tail. Dedup is by canonical card-set key so the curated entries' richer metadata is never overwritten.

---

## How it works

### Pipeline

```
                                       ┌──────────────┐
https://json.commanderspellbook.com/   │ Spellbook    │
            variants.json   ─ fetch ─▶ │ JSON cache   │ (data/rules/spellbook.json)
                                       └──────┬───────┘
                                              │
                                              ▼ ParseSpellbookJSON
                                       ┌──────────────┐
                                       │ Imported     │
                                       │ KnownCombos  │
                                       │ (deduped     │
                                       │  within file)│
                                       └──────┬───────┘
                                              │
                                              ▼ MergeKnownCombos
                                       ┌──────────────┐
                            curated ──▶│   merged     │──▶ allKnownCombos()
                            (58)       │   set        │    (analysis.go uses this)
                                       └──────────────┘
```

### Filtering

The parser keeps a variant ONLY if:
- `status` is one of `""`, `"OK"`, `"EXAMPLE"` — anything else (`"DRAFT"`, `"NEEDS_REVIEW"`, `"NOT_WORKING"`) is rejected so Freya never recognizes an unfinished combo
- `uses[].card.name` resolves to ≥ 2 non-empty card names (variants below 2 cards are warned and skipped)
- The canonical card-set key hasn't already been imported (intra-file dedupe — Spellbook can list the same combo under multiple variant IDs with different prerequisite phrasings)

### Type inference

The `Produces[].Feature.Name` strings drive the loop classification:

| Feature pattern | Inferred type |
|:---|:---|
| Contains `"Win the game"` or `"Lose the game"` | `true_infinite` |
| Contains `"infinite"` (any variant) | `true_infinite` |
| Anything else | `determined` |

Two passes — the win/lose pattern takes priority over the generic infinite-resource pattern.

### Dedup against curated

`canonicalComboKey(pieces)` lowercases each card name, trims whitespace, sorts the slice, and joins with `|`. Order-independent and case-insensitive. Punctuation (apostrophes, commas, hyphens) passes through unchanged — Spellbook and Scryfall agree on canonical card names so byte-equality after lowercase is sufficient.

`MergeKnownCombos(curated, imported)` walks the imported slice, checks each entry's canonical key against the curated set, and:
- **Drops** the imported entry if a curated entry exists with the same key (curated always wins — it carries the hand-authored richer metadata)
- **Drops** the imported entry if another imported entry already claimed the key (intra-import dedupe handles cases that slipped past the parser-level intra-file dedupe)
- **Keeps** the imported entry otherwise

Returns `(merged, dropped)` so the import-time CLI output can report how many imported combos collided with curated ones.

### Caching strategy

`FetchSpellbook` downloads the JSON and writes it atomically (`.tmp` → rename). Freya does NOT auto-refresh — the cache is the source of truth at deck-analysis time. Users opt into a refresh by passing `--spellbook-fetch <url>`. Without the fetch flag, Freya loads the cache if present and runs fine without a Spellbook database at all (no panic, no warning — just a smaller `allKnownCombos()` set).

---

## CLI usage

```bash
# Default — no Spellbook integration, just curated set
go run ./cmd/hexdek-freya/ --deck data/decks/mydeck.txt

# Load cached Spellbook DB if present (silent no-op if data/rules/spellbook.json missing)
go run ./cmd/hexdek-freya/ --deck data/decks/mydeck.txt --spellbook data/rules/spellbook.json

# Fetch fresh from upstream, then use it
go run ./cmd/hexdek-freya/ --deck data/decks/mydeck.txt \
  --spellbook data/rules/spellbook.json \
  --spellbook-fetch https://json.commanderspellbook.com/variants.json
```

The `--spellbook` flag defaults to `data/rules/spellbook.json` (`DefaultSpellbookCache`), so passing `--spellbook-fetch` alone is sufficient if you accept the default cache path.

---

## Test coverage

11 regression tests in `cmd/hexdek-freya/spellbook_import_test.go`. All passing.

| Test | What it pins |
|:---|:---|
| `TestParseSpellbookJSON_BasicShape` | end-to-end parse of a realistic 7-variant fixture; correct combo count, name format, intra-file dedupe |
| `TestParseSpellbookJSON_TypeInference` | "Win the game" → `true_infinite`; "Infinite mana" → `true_infinite`; plain "Card draw" → `determined` |
| `TestParseSpellbookJSON_StatusFilter` | "DRAFT" / "NEEDS_REVIEW" rejected; "OK" / "EXAMPLE" / "" accepted |
| `TestCanonicalComboKey_OrderIndependent` | `[A, B]` and `[B, a]` and `["  B  ", "A"]` collapse to the same key |
| `TestMergeKnownCombos_CuratedWinsOnConflict` | imported entry colliding with a curated key is dropped; curated metadata preserved verbatim |
| `TestMergeKnownCombos_DedupesAgainstRealCurated` | runs the merge against the actual 58 curated entries with a hand-built imported list designed to collide with several — confirms dedupe behavior against production data |
| `TestLoadSpellbookCache_MissingFileReturnsNil` | absent cache file returns `(nil, nil, nil)` — no error, no panic |
| `TestLoadSpellbookCache_RoundTrip` | write fixture JSON to disk, load via `LoadSpellbookCache`, verify combos match `ParseSpellbookJSON` output |
| `TestParseSpellbookJSON_BadJSONErrors` | malformed JSON returns an error, not panic, not partial state |
| `TestAllKnownCombos_FallsBackToCuratedWhenNoImport` | when `ImportedCombos` is empty, `allKnownCombos()` returns the curated slice directly (no merge cost) |
| `TestAllKnownCombos_MergesImportedAndDedupes` | when `ImportedCombos` is populated, returns the merged set with conflicts resolved curated-wins |

Run them:

```bash
go test ./cmd/hexdek-freya -run "TestParseSpellbookJSON|TestMergeKnownCombos|TestCanonicalComboKey|TestLoadSpellbookCache|TestAllKnownCombos" -v
```

---

## Verification on real Spellbook data

To verify the merge against the production Spellbook dataset:

```bash
# One-time fetch
go run ./cmd/hexdek-freya/ \
  --deck data/decks/test/cedh_combo_partner_b5_kraum_tymna.txt \
  --spellbook data/rules/spellbook.json \
  --spellbook-fetch https://json.commanderspellbook.com/variants.json
```

The CLI logs the import summary: `loaded N variants, M dropped (collided with curated), K kept` (or similar). Inspect `data/rules/spellbook.json` to see the cached payload; subsequent runs load directly from cache without re-fetching.

---

## Design choices

- **Fail-open on missing cache** — Freya never errors when the Spellbook cache is absent. The curated 58 are always available; the import is purely additive.
- **No auto-refresh** — the cache is the source of truth at deck-analysis time. Users explicitly opt into a refresh via `--spellbook-fetch`. Avoids surprise network calls during batch tournament analysis.
- **Curated always wins on conflict** — the 58 hand-authored entries carry richer outlet/stop/classification metadata than Spellbook's structured-but-thin records. Letting Spellbook overwrite would silently lose precision.
- **Type inference from feature names only** — Spellbook's structured Produces feature names are the most reliable signal. We don't parse the freeform description (would risk hallucinating loop types from prose).
- **Status-based filter** — only `"OK"` / `"EXAMPLE"` / `""` variants are imported. Excludes draft + broken combos so Freya never flags an unfinished line.
- **Order-independent canonical key** — combos are unordered sets of cards. The key is built by lowercasing each name, sorting, joining with `|`. Two prerequisites differing in piece order canonicalize to the same key.
- **Atomic cache writes** — `FetchSpellbook` writes to `.tmp` then renames, so a partial download never corrupts the cache.

---

## See also

- `cmd/hexdek-freya/analysis.go` — the `KnownCombos` curated baseline (58 entries) plus the `allKnownCombos()` callers
- `cmd/hexdek-freya/combo_class.go` — the loop class hierarchy (true_infinite / soft_lock / value_engine / win_condition) that imported combos flow into via `ClassifySpellbookImported`
- `docs/release-notes-r60.md` — r60 release notes section on Freya combo work
- [Commander Spellbook](https://commanderspellbook.com/) — the upstream data source
