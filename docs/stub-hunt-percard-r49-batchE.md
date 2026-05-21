# Stub Hunt — per_card (R49 batch E, defensive utility)

Scope: per_card stubs ported to real handlers, filtered to defensive
utility — protection grants, removal, replacement-effect-driven
prevention, and counter denial.

## Method

Two pass survey of `internal/gameengine/per_card/gen_*.go`:

1. **Filter out cards with `custom_*.go` overrides** — the registry
   routes those to the real handler at dispatch time anyway, so the
   gen breadcrumb is dead code.
2. **Read remaining gen files and group by oracle text**, keeping only
   ones whose primary effect is defensive utility. The pool we'll
   actually port has both pure stubs (handler body is just
   `emit`+`emitPartial`) AND "promote" candidates (handler stamps a
   breadcrumb flag instead of registering a real continuous-effect or
   replacement).

## The 8 ports — defensive utility

| # | Card | Category | What ships | Pattern / precedent |
|---|------|----------|------------|---------------------|
| 1 | **Maha, Its Feathers Night** | Mass-toughness removal | Promote the R37 `gs.Flags["maha_base_tough_one_active"]` breadcrumb to a real `RegisterContinuousEffect` at layer 7b (§613.4b base-PT SET) whose predicate matches opponents' creatures; ApplyFn writes `Toughness=1` / `BaseToughness=1`. Legacy flag kept as breadcrumb + the R37 ETB/LTB function names kept as shims so existing tests keep compiling. | Kudo (gen_kudo_king_among_bears.go) |
| 2 | **Sokrates, Athenian Teacher** | Conditional self-hexproof | Promote one-shot `Flags["kw:hexproof"]` stamp at ETB to a trigger-driven sync: `permanent_tapped` clears the flag, `upkeep` re-stamps if Sokrates is untapped. Existing Sokratic Dialogue activated kept (with inline flag-clear at tap-cost). | Lyse Hext flag pattern + Emmara `permanent_tapped` hook |
| 3 | **Thrun, Breaker of Silence** | Conditional self-indestructible | Promote `emitPartial` breadcrumb to ETB stamp + `upkeep` re-sync of `Flags["kw:indestructible"]` based on `gs.Active == src.Controller`. Cast-uncounterable hook preserved. | Ozai's conditional refresh, broadened to listen on every upkeep regardless of active seat |
| 4 | **Ozai, the Phoenix King** | Conditional self protection | Broaden the R37 `upkeep_controller`-only refresh to `upkeep` + `end_step` so a mid-turn mana spend cleans up the ≥6-mana flying+indestructible grant before the next SBA check. | Same handler, broader trigger set |
| 5 | **Progenitus** | Graveyard-shuffle replacement | Promote post-death reactive rescue to a true `RegisterReplacement(EventType="would_be_put_into_graveyard")` that places the Card in the owner's library, shuffles, and cancels the event (so the downstream `moveToZone` doesn't double-place). Reactive `creature_dies` path kept as idempotent fallback for code paths that bypass §614. `prot:*` static flag preserved. | Rest in Peace (replacement.go:687) shape; library-redirect handled inline because the engine's repl-payload pipeline doesn't natively support `to_zone=library` |
| 6 | **Wilson, Refined Grizzly** | Cast-uncounterable | Add `OnCast` hook stamping `StackItem.CostMeta["cannot_be_countered"]=true`. counter_resolve.go's spellCannotBeCountered then refuses to counter the spell. | Thrun cast hook |
| 7 | **Erebos, God of the Dead** | Opponent lifegain denial | Register §614 `would_gain_life` replacement on ETB; `Applies` filters to opponents, `ApplyFn` zeros `ev.Count()`. Existing `{1}{B}, Pay 2 life: Draw a card` activation untouched. | Bilbo Birthday Celebrant (gen_bilbo_birthday_celebrant.go) replacement shape, opposite direction (zero vs +1) |
| 8 | **Lier, Disciple of the Drowned** | Counter denial (controller-wide) | Port: OnTrigger("spell_cast") gates by `caster_seat == perm.Controller`, walks `gs.Stack` from top to find the matching StackItem (just pushed by PushStackItem; spell_cast fires AFTER push), stamps `CostMeta["cannot_be_countered"]=true`. Counter resolver refuses every controller spell while Lier is on the battlefield. | Thrun cast-uncounterable, broadened to controller-wide via spell_cast trigger |

## Two overlap collisions with concurrent batch B (Silvar, Zidane)

The initial batch-E candidate set was 10 cards. Two of those —
**Silvar, Devourer of the Free** and **Zidane, Tantalus Thief** — were
landed independently by R49 batch B (`516645d`/`b0d0223`) while batch E
was in flight. Both batch B implementations cover the defensive utility
ports batch E had drafted (Silvar: sac-Human → +1/+1 + indestructible
UEOT; Zidane: control-steal UEOT + lifelink + haste + EOT restoration
delayed trigger). On merge into main, batch E adopts batch B's versions
of those two files unchanged — there is nothing to add. The remaining
8 ports above are batch E's contribution.

## Tests

`internal/gameengine/per_card/percard_stub_batch_r49_e_test.go` — 16
new tests pin happy paths and negative-case guards for each of the 8
cards. Full `go test ./...` stays green across cmd/, internal/gameengine
(+ per_card), internal/hat, internal/heimdall, internal/hexapi,
internal/muninn, internal/tournament, etc.

## Out-of-scope (intentionally skipped)

- **Cards already done via `custom_*.go`** — Asmoranomardicadaistinaculdacar,
  Ellie, Splinter, Ureni, Mister Negative, Firesong & Sunspeaker,
  Mendicant Core, Old One Eye, Sandman, The Locust God, and others now
  have custom overrides; their gen breadcrumb is dead.
- **Cards covered by concurrent batches A/B/D** — Silvar, Zidane,
  Karlov, Tajic, Skullbriar, Ty Lee, Breya, Raphael, Alisaie, Lyse,
  Magnus, The Earth King, June Bounty Hunter, Kwain, Minwu, Oona,
  Rat King, The Destined White Mage, The Wandering Minstrel.
- **Lara Croft (attack-trigger discovery counters)** — needs a
  discovery-counter + play-from-exile pipeline that doesn't exist; the
  Raid Treasure half is already ported.
- **Kruphix (unspent-mana-becomes-colorless)** — engine-deep ManaEmpty
  replacement hook not in place.
- **Lier's flashback grant** — AST keyword-pipeline territory; the
  per_card layer would just shadow the real machinery.
- **Thrun's "can't be the target of nongreen opponents" protection** —
  §702.16 protection-from machinery owns this surface; per_card would
  shadow it incorrectly.
