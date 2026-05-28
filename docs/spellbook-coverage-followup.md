# Commander Spellbook Coverage Follow-up

## Headline

Re-fetched the live Commander Spellbook variants JSON 22 hours after
the PR #563 baseline import. **161 new variants added, 0 removed**
across that window; net corpus growth from 89,396 → 89,557. Both
snapshots ingest cleanly through the PR #563 parser — no schema drift,
no parse-warning regressions, same 35 curated-conflict dedupes, same
7 parse warnings on both sides.

## Cache snapshots

| Field | OLD (PR #563 baseline) | NEW (this re-fetch) | Δ |
|-------|------------------------|---------------------|---|
| Local file timestamp | 2026-05-26 18:26 | 2026-05-27 17:55 | +23h 29m |
| Upstream `timestamp` (in JSON) | 2026-05-27 01:25:06 UTC | 2026-05-27 23:24:26 UTC | +22h 0m |
| Upstream `version` | 5.4.10 | 5.4.10 | unchanged |
| File size (bytes) | 507,759,602 | 508,577,370 | +817,768 (+0.16%) |
| Variant count | 89,396 | 89,557 | **+161** |
| Status="OK" variants | 89,396 | 89,557 | +161 (all-OK on both sides) |
| Variant IDs ADDED (new − old) | — | **161** | — |
| Variant IDs REMOVED (old − new) | — | 0 | — |
| Variant IDs STABLE | — | 89,396 | — |

Freya end-to-end on a representative deck (Blast from the Past, the
PR #563 verification case):

| Field | OLD | NEW |
|-------|----:|----:|
| Loaded combos | 89,322 | 89,483 |
| Skipped as duplicates of curated entries | 35 | 35 |
| Parse warnings | 7 | 7 |
| Imported delta (NEW − OLD) | — | **+161** |

The +161 imported delta exactly matches the +161 raw ID delta, so
every new upstream variant survives the parser, status filter, AND
the canonical-key intra-import dedupe (the 74-entry intra-import
collapse held constant — Spellbook's "same card set under multiple
variant IDs" pattern produces no new collisions in this 161-deck
window).

## What the 161 new combos look like

All 161 are **3-card combos** — Spellbook's curators didn't ship any
new 2-card or 4+-card additions in the 22-hour window.

### By primary outcome class

| Outcome | Count | Notes |
|---------|------:|-------|
| Infinite ETB | **110** | Dominant cluster — the new Ratadrabik of Urborg + Leyline of Singularity package |
| Infinite mana | 23 | Mostly the Eirdu / Isilu + Vizier of Remedies family |
| Lockdown | 17 | Agatha of the Vile Cauldron-anchored hard locks |
| Infinite damage | 10 | Existing damage shells with new third-piece carveouts |
| Infinite mill | 1 | One-off addition |
| **Total** | **161** | |

### By color identity (top 10)

| Identity | Count |
|----------|------:|
| WUB | 43 |
| WB | 31 |
| RG | 21 |
| WU | 15 |
| WUBR | 12 |
| GWUB | 10 |
| RWB | 8 |
| WUBRG | 7 |
| BRGW | 5 |
| WBG | 5 |

White / blue heavy — consistent with the Ratadrabik/Leyline package
landing in Esper-class color shells and the Eirdu+Vizier mana cluster
sitting in Bant-class.

### Top driver cards

| Card | Combo count | Likely role |
|------|------------:|-------------|
| Ratadrabik of Urborg | 65 | Legendary token doubler (DMU); enables the new "legendary creature dies → token copy" infinite ETB family |
| Leyline of Singularity | 65 | Removes the legendary rule's "destroy" clause from the Ratadrabik token, locking the ETB loop open |
| Eirdu, Carrier of Dawn // Isilu, Carrier of Twilight | 56 | MDFC enabler in the infinite-mana cluster |
| Vizier of Remedies | 56 | Replaces "this creature dies" with persist returns — pairs with the Eirdu line |
| Agatha of the Vile Cauldron | 23 | New lockdown anchor; tax effect on activated abilities |
| Silvanus's Invoker | 23 | Pairs with Agatha for the mass-untap lock |
| Timestream Navigator | 17 | Newly indexed extra-turn enabler in 3-piece lines |
| Recruiter of the Guard | 17 | Tutor that finds the new 2-power utility creatures |
| King Darien XLVIII / Basal Sliver | 2 each | Auxiliary third pieces in the Ratadrabik shells |

The Ratadrabik + Leyline of Singularity package is the headline new
combo family — 65 of the 161 new variants (40%) involve at least one
of those two cards, and most of the rest are auxiliary third pieces
paired around the same core engine.

## Schema-drift check

The PR #563 fixture-based schema-drift canary
(`TestParseSpellbookJSON_LivePayloadSchemaSmoke` in
`cmd/hexdek-freya/spellbook_import_test.go`) was added precisely to
catch upstream field renames. **It still passes** on the new
spellbook.json — the upstream payload shape (`variants[].id`,
`variants[].status`, `variants[].uses[].card.name`,
`variants[].produces[].feature.name`, `variants[].description`,
`variants[].identity`) is unchanged.

The `version: 5.4.10` stamp matches both snapshots, so the upstream
API contract is bit-stable across this window.

## Reproducing

```bash
cd $(git rev-parse --show-toplevel)
git fetch origin main
git checkout -B dev/freya-spellbook-update-r60 origin/main

# Snapshot any existing cache for diffing.
cp data/rules/spellbook.json /tmp/spellbook_old.json

# Re-fetch the latest variants JSON (~510MB, takes ~10s on a fast link).
curl -sf -o data/rules/spellbook.json \
    https://json.commanderspellbook.com/variants.json --max-time 120

# Confirm Freya ingests cleanly + count delta.
go run ./cmd/hexdek-freya/ --deck \
    data/decks/wizards/blast_from_the_past_doctor_who_commander_precon_decklist.txt \
    2>&1 | grep -i spellbook
# Expected: "loaded 89,557 Spellbook combos ... 35 skipped as duplicates ... 7 parse warnings"
```

## Open follow-up

- **Cache file is gitignored** (`/data/rules/spellbook.json` per
  `.gitignore`) and we don't auto-refresh on every freya invocation
  by design. The 22-hour delta producing 161 new variants gives a
  rough rate-of-change baseline; if upstream remains at ~7
  combos/hour, a weekly re-fetch yields ~1,200 new combos per cycle,
  which is well within the parser's existing throughput (~89K
  variants ingested in <1s during freya startup).
- **No re-classification needed**: the new variants slot into the
  existing 5 outcome classes (infinite ETB / mana / damage / mill /
  lockdown) — no new combo-class enum extensions warranted.
- **The Ratadrabik + Leyline of Singularity package warrants a
  curated KnownCombos entry** so the canonical-key dedupe path
  treats it as a first-class combo rather than an imported variant.
  This is a small docs-and-list change that would lift the 65
  Ratadrabik-family Spellbook variants out of the "imported"
  bucket; left for a follow-up PR to keep this report focused on
  the coverage delta itself.
