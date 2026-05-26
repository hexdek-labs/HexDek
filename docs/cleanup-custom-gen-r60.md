# Cleanup — `custom_X.go` + `gen_X.go` Shadow-Pair Sweep (R60 Versailles Phase 2I)

**Date:** 2026-05-25
**Branch:** `dev/cleanup-custom-gen-shadow-r60`
**Source audit:** `docs/audit-percard-registry-r60.md` (PR #477)
**Scope:** every `custom_X.go` + `gen_X.go` pair under
`internal/gameengine/per_card/`. For each: determine which file owns
the live registration, delete the redundant side (or merge if both
contribute unique logic).

## Pair inventory

**Total pairs found: 32.** (The PR #477 audit listed 14; the gap is
mostly Class A delegators and Class B no-op stubs that the audit
didn't enumerate.)

Classification:

| Class | Description | Count | This PR |
|---|---|---|---|
| **B** | gen is a neutered no-op (`_ = r`) — `custom_*Custom` is wired elsewhere | 14 | **deleted** |
| **A** | gen is a single-line delegator (`registerXCustom(r)`) | 5 | left in place — addressed in follow-up |
| **C** | gen and custom both register real handlers (potential double-fire or complementary split) | 13 | **deferred** to per-card triage |

## Class B — deleted (14 files)

These gen files contained ONLY a `register*` function with `_ = r`
body. Each was a relic of an earlier sweep that neutered an
auto-gen breadcrumb but couldn't delete the file because
`batch_generated.go` was still calling the function. The custom
counterpart was already registered from elsewhere
(`registry.go::registerDefaults` or one of the `zz_*_register.go`
files), so deleting the gen file + its `batch_generated.go` call is
a pure no-behavior-change cleanup.

| Card | Where custom is registered |
|---|---|
| Asmoranomardicadaistinaculdacar | `registry.go:1849` |
| Ellie, Vengeful Hunter | `zz_handler_q45_register.go:34` |
| Galazeth Prismari | `registry.go:1846` |
| Ghen, Arcanum Weaver | `zz_activated_stubs_register.go:35` |
| Gyruda, Doom of Depths | `zz_handler_q45_register.go:31` |
| Jhoira, Ageless Innovator | `zz_activated_stubs_register.go:32` |
| Mister Negative | `zz_handler_q45_register.go:36` |
| Obeka, Brute Chronologist | `zz_activated_stubs_register.go:31` |
| Phenax, God of Deception | `zz_activated_stubs_register.go:30` |
| Quandrix, the Proof | `registry.go:1852` |
| Shadowheart, Dark Justiciar | `zz_activated_stubs_register.go:33` |
| Silverquill, the Disputant | `registry.go:1851` |
| Splinter, Radical Rat | `zz_activated_stubs_register.go:34` |
| Ureni, the Song Unending | `zz_handler_q45_register.go:33` |

**Side-cleanups in the same PR:**

- 14 lines removed from `batch_generated.go::registerGeneratedHandlers`.
- 4 stale doc comments in `custom_*.go` siblings updated — they referenced "the matching `gen_*.go` remains in place" / "both handlers fire (its body only emits a partial)", which is no longer true now that the gen sides are deleted.
- 2 test files (`percard_stub_batch_c_r49_test.go`, `percard_stub_batch_g_r50_test.go`) had `TestBatch_NeutersAreCallableNoOps` tests that directly called the now-deleted gen `register*` functions. Removed; `TestBatch_CustomHandlersStillRegisteredAfterReset` retained (it pins the canonical "custom is wired" coverage, which is exactly what survives this PR).

## Class A — left in place (5 files)

Gen file is a 9–10-line delegator:

```go
func registerX(r *Registry) {
    registerXCustom(r)
}
```

Cards: Avacyn (Angel of Hope), Feather (the Redeemed), Hogaak
(Arisen Necropolis), Kess (Dissident Mage), Maelstrom Wanderer.

These are ambiguity-only — no double-fire, no broken behavior, just
noise from a one-line indirection. Could be flattened (rename the
custom function to drop the `Custom` suffix, delete the gen file,
update `batch_generated.go` to call the new name), but the rename
ripples into the `custom_*.go` doc comments and any test that
references the `registerXCustom` symbol. Out of scope for this PR
to keep the diff scoped to "delete-without-rename."

## Class C — deferred to per-card triage (13 files)

Both gen and custom register real handlers, with mixed overlap:

| Card | gen registers | custom registers | Overlap risk |
|---|---|---|---|
| Commander Mustard | `OnActivated`, `OnTrigger("creature_attacks")` | `OnETB`, `OnTrigger("permanent_etb"/"_ltb")` | None (split — both needed) |
| Firesong and Sunspeaker | `OnETB`, `OnTrigger("life_gained")` | (per-card sibling file) | Need to compare custom |
| Kolodin, Triumph Caster | `OnETB`, `OnTrigger("permanent_etb")` | — | Listed in PR #477 audit's dup table |
| Lier, Disciple of the Drowned | `OnTrigger("spell_cast")`, `OnETB` | — | Custom is the flashback-grant primitive |
| Mabel, Heir to Cragflame | `OnETB` | — | Both ETBs would stack |
| Mendicant Core, Guidelight | `OnETB`, `OnTrigger` ×3 | `OnTrigger("spell_cast")` | `spell_cast` overlap |
| Morlun, Devourer of Spiders | `OnCast`, `OnETB` | — | — |
| Old One Eye | `OnETB` | — | — |
| Rienne, Angel of Rebirth | `OnETB`, `OnTrigger("creature_dies")` | `OnETB`, `OnTrigger("permanent_etb"/"_ltb")` | OnETB overlap |
| Sandman, Shifting Scoundrel | `OnETB`, `OnActivated` | — | — |
| The Locust God | `OnETB`, `OnTrigger("card_drawn")` | `OnActivated` | None (split — both needed) |
| Toxrill, the Corrosive | `OnTrigger("end_step")`, `OnTrigger("creature_dies")` | `OnTrigger("end_step")`, `OnActivated` | `end_step` overlap; **gen body is incorrect (1/1 token + draw)** |
| Yorion, Sky Nomad | `OnETB` (real impl) | `_ = r` (neutered) | None — reverse pattern; custom is the no-op |

Per-card resolution is non-trivial:

- Some pairs are genuine **functional splits** (Mustard, Locust
  God): gen owns one half, custom owns the other. Resolution: either
  merge into one file or document the split as intentional.
- Some pairs are **partial overlaps** (Mendicant, Rienne, Toxrill):
  one trigger fires in both handlers, which is a real double-execute
  bug. Resolution: delete the duplicate registration in the wrong
  file.
- Yorion is **reversed** (custom is neutered, gen is live).
  Resolution: delete the custom side or rename.

Each of these needs careful inspection (the gen body for Toxrill
literally draws a card on every end_step + makes a token labeled
"1/1 Token Token" — that's broken behavior actively firing today).
Out of scope for THIS PR to keep the diff small and auditable; a
dedicated PR per Class C card (or a careful batch sweep) is the
right next step.

## Verification

- `go build ./...` clean.
- `go test -short ./internal/gameengine/... ./internal/gameengine/per_card/... ./internal/tournament/... ./internal/hat/...` clean.
- No test was relying on the deleted gen `register*` functions
  except the two "neuter is callable" tests, which were updated
  to remove their now-meaningless body and retain the
  "custom is wired" companion test.

## Net diff

20 files changed (14 deletions + 6 edits):

- **Deleted (14):** `gen_{asmoranomardicadaistinaculdacar, ellie_vengeful_hunter, galazeth_prismari, ghen_arcanum_weaver, gyruda_doom_of_depths, jhoira_ageless_innovator, mister_negative, obeka_brute_chronologist, phenax_god_of_deception, quandrix_the_proof, shadowheart_dark_justiciar, silverquill_the_disputant, splinter_radical_rat, ureni_the_song_unending}.go`
- **Edited (6):** `batch_generated.go` (−14 lines); `percard_stub_batch_c_r49_test.go` (delete dead test fn); `percard_stub_batch_g_r50_test.go` (delete dead test fn); `custom_{asmoranomardicadaistinaculdacar, galazeth_prismari, quandrix_the_proof, silverquill_the_disputant}.go` (stale doc-comment refresh, 4 sites)
