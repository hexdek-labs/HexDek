# Freya Export API

External-integration documentation for HexDek's per-deck Freya analysis
endpoint. If you're building a deck-builder, recommender, EDHrec-style
aggregator, or research tool that wants HexDek's archetype + combo +
power-tier + win-line analysis, this is the surface.

For internal frontend consumption, `/api/decks/{owner}/{id}/analysis`
already returns raw `strategy.json`. The endpoint described here adds
deck context, schema versioning, and R60 classification extras
specifically so external tools don't have to make 3-5 calls to
reconstruct one deck's full picture.

- **Endpoint**: `GET /api/decks/{owner}/{id}/freya-export`
- **Auth**: none — public read
- **Stability**: schema-versioned; current version is `1.0`
- **Side effects**: none — safe to scrape

---

## Quick start

```bash
curl https://hexdek.dev/api/decks/7174n1c/god_save_the_queen/freya-export | jq .schema_version
# "1.0"

curl -s https://hexdek.dev/api/decks/7174n1c/god_save_the_queen/freya-export \
  | jq '{commander: .deck.commander_card, archetype: .analysis.deck_profile.primary_archetype, bracket: .analysis.deck_profile.bracket}'
# {
#   "commander": "Queen Marchesa",
#   "archetype": "Pillowfort",
#   "bracket": 3
# }
```

---

## Envelope schema (v1.0)

The response is a single JSON object with five top-level keys:

```json
{
  "schema_version": "1.0",
  "generated_at": "2026-05-25T17:42:18Z",
  "source": "hexdek.dev/freya",
  "deck":     { ... },
  "analysis": { ... },
  "extras":   { ... }
}
```

| Field            | Type   | Stability   | Meaning                                                       |
| ---------------- | ------ | ----------- | ------------------------------------------------------------- |
| `schema_version` | string | locked      | Bumped MAJOR on breaking shape changes; current `1.0`         |
| `generated_at`   | string | locked      | RFC 3339 UTC instant the response was rendered                |
| `source`         | string | locked      | Always `"hexdek.dev/freya"`                                   |
| `deck`           | object | locked      | Deck identity + commander + card list (see below)             |
| `analysis`       | object | **upstream** | Verbatim `strategy.json` — schema owned by Freya, not this API |
| `extras`         | object | locked      | R60-era extras outside `strategy.json` (see below)            |

The **envelope** (`schema_version`, `generated_at`, `source`, `deck`,
`extras`) is what `1.0` pins down. The **`analysis` blob** is a
pass-through of `strategy.json`; new Freya fields land in it without an
envelope bump, but no existing field will disappear silently. See
[Versioning](#versioning) for the contract.

### `deck` sub-object

```json
{
  "owner": "7174n1c",
  "id": "god_save_the_queen",
  "commander": "QUEEN MARCHESA",
  "commander_card": "Queen Marchesa",
  "color_identity": "?",
  "card_count": 100,
  "cards": [
    { "name": "Sol Ring",        "quantity": 1 },
    { "name": "Plains",          "quantity": 8 },
    { "name": "Anguished Unmaking", "quantity": 1 }
  ]
}
```

| Field            | Type    | Note                                                                                                                |
| ---------------- | ------- | ------------------------------------------------------------------------------------------------------------------- |
| `owner`          | string  | URL path component, ASCII-only                                                                                       |
| `id`             | string  | URL path component (the deck slug)                                                                                   |
| `commander`      | string  | Upper-case display label derived from `COMMANDER:` line or filename                                                  |
| `commander_card` | string  | The exact card name as written in the deck file                                                                      |
| `color_identity` | string  | WUBRG letters (e.g. `"WBR"`) when the parser computed it; `"?"` when not yet computed — treat empty/`?` as unknown   |
| `card_count`     | int     | Quantity-weighted sum (a 4× Lightning Bolt counts as 4, not 1)                                                       |
| `cards`          | array   | `[{name, quantity}, ...]` — commander is auto-appended if not already present in the body, so this is the FULL list |

### `analysis` sub-object (upstream — `strategy.json` pass-through)

A non-exhaustive map of major sections you'll find under `analysis`.
**This list documents what's likely present, not what's guaranteed
to be present.** Always feature-detect rather than assuming a field
exists.

```json
{
  "deck_profile": {
    "primary_archetype": "Pillowfort",
    "secondary_archetype": "Control",
    "bracket": 3,
    "bracket_label": "Optimized",
    "power_percentile": 67,
    "gameplan_summary": "..."
  },
  "archetype": {
    "fingerprints": [
      { "name": "Pillowfort", "score": 18, "evidence": ["Ghostly Prison", "Propaganda"] }
    ],
    "bracket_rationale": { "raw_score": 14, "signals": [...] }
  },
  "win_lines": {
    "primary": "...",
    "finishers": [...]
  },
  "true_infinites":   [ { "cards": [...], "loop_type": "...", "description": "..." } ],
  "determined_loops": [ ... ],
  "finishers":        [ ... ],
  "synergies":        [ ... ],
  "mana_curve": {
    "distribution": [4, 12, 18, 16, 9, 6, 2, 1],
    "avg_cmc": 2.7,
    "curve_shape": "bimodal",
    "land_count": 37
  },
  "color_balance": {
    "demand": { "W": 22, "B": 18, "R": 14 },
    "supply": { "W": 19, "B": 16, "R": 12 }
  },
  "roles": {
    "ramp": 8, "draw": 9, "removal": 7, "wipe": 4, "tutors": 3, "protection": 5
  },
  "value_chains": [ { "name": "Monarchy + Council's Judgment", "depth": 3, "members": [...] } ],
  "statistics": {
    "tutors": 6, "non_land_tutors": 3, "removal": 10, "sacrifice_outlets": 0
  },
  "unified_profile": {
    "power_tier_counts": { "S": 2, "A": 7, "B": 38, "C": 28, "D": 8 },
    "card_power_levels": [
      { "name": "Queen Marchesa", "power": 87, "tier": "S", "explanation": "..." }
    ],
    "star_cards":    [ ... ],
    "solid_cards":   [ ... ],
    "cuttable_cards":[ ... ],
    "synergy_clusters": [ ... ],
    "meta_positioning": [ ... ],
    "interaction_quality": { "avg_cmc_interaction": 2.4, "cheap_interaction": 6 },
    "opening_hand_sim": { "keepable_pct_standard": 64.2, "keepable_pct_adjusted": 71.8 }
  }
}
```

**Why this is "pass-through" and not formally documented**: Freya's
schema evolves frequently — recent additions include the
S/A/B/C/D power-tier grading system (PR #308), pet-card detection
(PR #380), and per-card power explanations + corpus aggregates from
adjacent work. Pinning the analysis shape in this doc would freeze
external tooling against a moving target. Instead the envelope
guarantees the **wrapper** is stable, and the analysis-blob fields
follow Freya's own evolution.

For the authoritative source-of-truth field list, read
[`cmd/hexdek-freya/report.go`](../cmd/hexdek-freya/report.go) (the
`jsonStrategy` struct and its descendants).

### `extras` sub-object

R60-era classification metadata that lives outside `strategy.json`.
Surfaced here so external consumers see the **full** classification
context — what Freya thought, what the user thought, and which deck
version both applied to.

```json
{
  "system_tags": ["archetype:pillowfort"],
  "user_archetype_feedback": "control",
  "version_hash": "a7f3c91b...",
  "version_number": 4
}
```

| Field                      | Type     | Meaning                                                                                                                                                                                                                |
| -------------------------- | -------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `system_tags`              | string[] | Freya-derived tags, prefixed `archetype:`. One entry today; the slice shape allows future system tags (bracket, color identity, power tier) to join.                                                                   |
| `user_archetype_feedback`  | string   | Empty/absent = no opinion. `"confirmed"` = user agreed with Freya. Anything else = the normalized archetype name the user prefers (e.g. `"stax"`, `"lands-matter"`). This is the OVERRIDE — Freya says X, user says Y. |
| `version_hash`             | string   | Content-addressable hash of the current deck version (from the versioning DAG)                                                                                                                                          |
| `version_number`           | int      | Incrementing version count (1, 2, 3...) for this deck                                                                                                                                                                  |

---

## Response codes

| Status | Meaning                                                                                                                                                              |
| ------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 200    | Envelope returned                                                                                                                                                    |
| 400    | Invalid `owner` or `id` (path-traversal-shaped, non-ASCII, etc.)                                                                                                     |
| 404    | Deck not found OR `strategy.json` not yet generated. Body hints at `POST /api/decks/{owner}/{id}/analyze` so the caller knows how to trigger analysis if they opt in |
| 500    | IO failure reading the deck or analysis file, or `strategy.json` exists but is malformed                                                                             |

**`/freya-export` is intentionally read-only** — unlike
`/api/decks/{owner}/{id}/analysis`, it will not auto-trigger a Freya
run when analysis is missing. External integrators scraping the corpus
need a clear deterministic failure, not a "come back in 30 seconds"
side-effect. If you want to trigger analysis on missing decks, POST
`/api/decks/{owner}/{id}/analyze` explicitly.

---

## Integration recipes

### EDHrec-style aggregator

You're maintaining a per-commander archetype distribution and want to
include HexDek-classified decks in the rollup.

```python
import requests

def freya_archetype(owner, deck_id):
    r = requests.get(f"https://hexdek.dev/api/decks/{owner}/{deck_id}/freya-export")
    if r.status_code == 404:
        return None  # analysis not yet run; skip or POST /analyze
    r.raise_for_status()
    data = r.json()

    # Schema-version guard — refuse to read fields you don't recognize.
    if not data["schema_version"].startswith("1."):
        raise RuntimeError(f"unsupported schema: {data['schema_version']}")

    return {
        "commander": data["deck"]["commander_card"],
        "freya_archetype": data["analysis"]["deck_profile"]["primary_archetype"],
        # Honour the user's override when they disagreed with Freya.
        "user_archetype": data["extras"].get("user_archetype_feedback") or None,
        "card_count": data["deck"]["card_count"],
        "deck_version": data["extras"].get("version_number"),
    }
```

**Why `extras.user_archetype_feedback` matters**: when present and not
`"confirmed"`, it's the deck owner's correction to Freya's call. For
aggregator accuracy, prefer the user override — Freya is an automated
fingerprint match, the owner has direct knowledge of the deck's intent.

### Deck-builder upgrade suggestions

Your deck-builder wants to suggest which cards to cut and which slots
to fill, anchored against HexDek's role/cluster analysis.

```javascript
async function suggestCuts(owner, deckId) {
  const r = await fetch(`https://hexdek.dev/api/decks/${owner}/${deckId}/freya-export`);
  if (r.status === 404) return { error: "analysis not yet run" };
  const { analysis } = await r.json();

  // unified_profile is the R60 power-tier surface.
  const cuttable = analysis.unified_profile?.cuttable_cards ?? [];
  const stars    = analysis.unified_profile?.star_cards ?? [];

  return {
    cuts:  cuttable.map(c => ({ name: c.name, reason: c.power_explanation })),
    keeps: stars.map(c => ({ name: c.name, reason: c.power_explanation })),
    archetype: analysis.deck_profile?.primary_archetype,
    // Use synergy_clusters to suggest "you have 3 of 4 pieces of the
    // sacrifice-engine chain — adding a fourth would close the loop."
    incomplete_clusters: (analysis.unified_profile?.synergy_clusters ?? [])
      .filter(c => c.completeness < 1.0),
  };
}
```

### Power-level / matchup recommender

You're building a recommender that helps users find decks at a
specific power level or with a specific matchup profile.

```javascript
const decks = await Promise.all(deckKeys.map(async key => {
  const [owner, id] = key.split('/');
  const r = await fetch(`https://hexdek.dev/api/decks/${owner}/${id}/freya-export`);
  return r.ok ? r.json() : null;
}));

const matchups = decks.filter(Boolean).map(d => ({
  deck: `${d.deck.owner}/${d.deck.id}`,
  bracket: d.analysis.deck_profile?.bracket,             // 1–5
  primary_archetype: d.analysis.deck_profile?.primary_archetype,
  power_distribution: d.analysis.unified_profile?.power_tier_counts,
  predicted_matchups: d.analysis.unified_profile?.meta_positioning,  // [{vs, score, reason}]
  // Trust the user override for archetype matching:
  effective_archetype:
    (d.extras.user_archetype_feedback && d.extras.user_archetype_feedback !== "confirmed")
      ? d.extras.user_archetype_feedback
      : d.analysis.deck_profile?.primary_archetype,
}));
```

---

## Versioning

The envelope follows a strict additive-or-bump policy:

- **Additive change** (new field, new enum value, new `extras.*` key)
  → no version bump. Consumers should ignore unrecognized fields and
  continue.
- **Breaking change** (removed field, renamed field, type change,
  semantics change) → `schema_version` MAJOR bump (e.g. `1.0` → `2.0`).
  The previous-version endpoint **may** remain available at a versioned
  URL for a grace period; check this doc when you see the constant
  bump.

The `analysis` blob is exempt — its schema is owned by Freya proper
and evolves independently. New `analysis` fields land without an
envelope version bump.

**Recommended consumer policy**: pin against the MAJOR version. Refuse
unrecognized MAJOR values; warn-and-continue on unrecognized MINOR.
Feature-detect every `analysis.*` field before reading.

---

## Rate limiting and scraping

The endpoint is not currently rate-limited per-key, but corpus-wide
scraping should be polite:

- **Cache the envelope** for at least the deck's typical edit
  cadence (~hours). The `generated_at` timestamp + the
  `extras.version_hash` together identify whether the underlying deck
  has changed; re-fetching for an unchanged hash is wasted load.
- **Discover decks via `/api/decks?owner=...`** rather than guessing
  IDs. The listing endpoint is the canonical inventory.
- **Skip 404s** rather than retrying — a 404 here means "no
  analysis", which a single retry won't fix. Trigger `/analyze`
  explicitly if you want the underlying deck classified.
- **If you're building a heavy integration**, file an issue at
  https://github.com/hexdek-labs/HexDek/issues so we can pin SLAs and
  potentially provision an API key tier.

---

## Field provenance

For HexDek contributors — where each piece of the envelope comes from.

| Field                            | Source                                                                      |
| -------------------------------- | --------------------------------------------------------------------------- |
| `schema_version`                 | `FreyaExportSchemaVersion` constant in `internal/hexapi/freya_export.go`     |
| `generated_at`                   | `time.Now().UTC().Format(time.RFC3339)` in handler                           |
| `source`                         | Hardcoded `"hexdek.dev/freya"`                                              |
| `deck.commander`                 | `parseDeckFilename` + `extractCommander` (file-name suffix or COMMANDER line) |
| `deck.color_identity`            | `parseDeckFilename` — currently hardcoded `"?"`; real WUBRG extraction is a follow-up |
| `deck.card_count` / `deck.cards` | `parseDeckList` (commander auto-appended when not in body)                  |
| `analysis`                       | `data/decks/{owner}/freya/{id}.strategy.json` — verbatim pass-through       |
| `extras.system_tags`             | `loadFreyaSystemTags` → reads `primary_archetype` from `strategy.json`      |
| `extras.user_archetype_feedback` | `deck_meta.archetype_feedback` column (PR #421)                              |
| `extras.version_hash` / `_number`| `versioning.DeckDAG.GetHead` (decks/.versions/ DAG, PR #425)                 |

---

## Related PRs (R60 lineage)

The `extras` block consolidates signals from a chain of recent
R60 work. Listed here for HexDek contributors tracing field
provenance:

- **PR #416** — system-assigned archetype tags from Freya
  (`extras.system_tags`)
- **PR #421** — user confirm/correct UI + `archetype_feedback_log`
  training-signal table (`extras.user_archetype_feedback`)
- **PR #425** — archetype change history across deck versions
  (`extras.version_hash`, `extras.version_number`)
- **PR #429** — training-signal endpoint
  (`/api/freya/training-signal` — out of scope for this export but
  consumes the same `archetype_feedback_log` table)
- **PR #432** — this export endpoint

## Changelog

- **`1.0`** (2026-05-25) — initial release. PR #432.
