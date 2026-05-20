# Stub Hunt — per_card (R46)

Scope: `internal/gameengine/per_card/` — identify auto-generated handlers that
log-and-bail (only call `emit` / `emitPartial`, do no real state work) so we
can port the highest-value ones into real handlers.

## Method

Surveyed all `gen_*.go` handlers that contain `emitPartial`, filtered out files
that already have a `custom_*.go` override (those route to a real handler at
dispatch time anyway), then ranked the remainder by how much engine-state
mutation appears in the file body. Anything with zero mutator calls is a
"pure stub" — the entire body is a breadcrumb.

A pure stub is what we're hunting: registered but inert. A "partial" handler
(does the easy clauses, emits `parser_gap` for the hard ones) is already
giving us most of the value and is lower priority.

## Pure stubs (12)

Sorted by porting tractability — i.e. how cleanly the missing behavior maps
onto existing engine primitives.

| # | Card | Oracle clause(s) | Why it's a stub | Port complexity |
|---|------|-------------------|-----------------|------------------|
| 1 | **Eruth, Tormented Prophet** (`gen_eruth_tormented_prophet.go`) | If you would draw a card, exile the top two cards of your library instead. You may play those cards this turn. | Pure breadcrumb. Replacement on `would_draw`. | Low — `gs.RegisterReplacement(EventType="would_draw")` + grant `ZoneCastPermission` on the exiled cards. Lab Maniac is the precedent. |
| 2 | **Kudo, King Among Bears** (`gen_kudo_king_among_bears.go`) | Other creatures have base power and toughness 2/2 and are Bears in addition to their other types. | Pure breadcrumb. Layer 7b base-PT plus type-add. | Medium — stamp `kudo_base_2_2`+`bear` on others at ETB and on `permanent_etb` for newcomers; sweep on LTB. Type-add via `Card.Types`. Not layer-perfect but the runtime fast-path is fine. |
| 3 | **Clara Oswald** (`gen_clara_oswald.go`) | If a triggered ability of a Doctor you control triggers, that ability triggers an additional time. | Pure breadcrumb. Filtered Panharmonicon. | Medium — `gs.RegisterReplacement(EventType="would_fire_etb_trigger")` with a Doctor-subtype filter. Covers ETB triggers only; non-ETB triggers stay partial (same constraint as Yarok/Panharmonicon infra). |
| 4 | **Katara, the Fearless** (`gen_katara_the_fearless.go`) | If a triggered ability of an Ally you control triggers, that ability triggers an additional time. | Pure breadcrumb. Filtered Panharmonicon. | Medium — same shape as Clara, filter on Ally subtype. |
| 5 | **The Twelfth Doctor** (`gen_the_twelfth_doctor.go`) | The first spell you cast from anywhere other than your hand each turn has demonstrate. Whenever you copy a spell, put a +1/+1 counter on The Twelfth Doctor. | Pure breadcrumb. Demonstrate-grant + counter trigger. | Low (partial) — OnTrigger("spell_copied") adds the +1/+1. Demonstrate-grant stays partial. |
| 6 | **Toph, the First Metalbender** (`gen_toph_the_first_metalbender.go`) | Nontoken artifacts you control are lands in addition to their other types. At the beginning of your end step, earthbend 2. | Pure breadcrumb. Earthbend at EOT + static type-grant. | Low — OnTrigger("end_step") picks a land, stamps `earthbent`/`temp_haste`/+1/+1 counters (mirrors `toph_earthbending_master.go`). Type-grant stays partial. |
| 7 | **Bello, Bard of the Brambles** (`gen_bello_bard_of_the_brambles.go`) | Non-Equipment artifacts / non-Aura enchantments with MV ≥ 4 you control are 4/4 Elementals with indestructible, haste, and damage-to-player → draw. | Pure breadcrumb. Static type-grant + keyword set. | High — Layer 4 + 6 + 7 grant on a filtered set. Skip; AST keyword pipeline owns this. |
| 8 | **Hamza, Guardian of Arashin** (`gen_hamza_guardian_of_arashin.go`) | Cost reduction. | Engine handles via `cost_modifiers.go`. | Skip — already real (engine-side). |
| 9 | **Rakdos, Lord of Riots** (`gen_rakdos_lord_of_riots.go`) | Cost reduction + cast restriction. | Engine handles via `cost_modifiers.go`. | Skip. |
| 10 | **Jasmine Boreal of the Seven** (`gen_jasmine_boreal_of_the_seven.go`) | Restricted mana ability + block restriction. | Engine-side mana tag + combat-blocker layer. | Skip. |
| 11 | **The Master, Multiplied** (`gen_the_master_multiplied.go`) | Myriad + token legend exception + triggered-ability-can't-sac-tokens. | Mostly engine-side (myriad keyword + SBA). | Skip — sac/exile rider is a dispatch-time predicate, not a per_card mutation. |
| 12 | **Ivy, Gleeful Spellthief** (`gen_ivy_gleeful_spellthief.go`) | Whenever a player casts a spell that targets only a single creature other than Ivy, you may copy that spell, copy targets Ivy. | Identifies the trigger but doesn't push the copy. | Medium — push a `StackItem` copy onto the stack with `IsCopy: true` and rewrite target to Ivy. Strionic Resonator is the precedent. |

## Top 10 ports — R46 batch (final scope)

Nine pure-stub → real-handler ports in per_card/, plus one small additive
engine surface (FireCardTrigger("spell_copied") in resolve.go's spell-copy
path) needed to dispatch The Twelfth Doctor's trigger. Counts as the 10th
mechanical change. Read the final landed code as the source of truth — the
table below records what shipped.

| # | Card | What ships | Verified by |
|---|------|------------|--------------|
| 1 | **Eruth, Tormented Prophet** | OnETB registers `would_draw` replacement: cancels draw, exiles top 2 to controller's exile, grants each card an `until_end_of_turn` `ZoneCastPermission` from exile at normal mana cost. `UnregisterReplacementsForPermanent` auto-cleans on LTB. | `TestEruth_WouldDrawExilesTopTwoAndCancels`, `TestEruth_ReplacementUnregistersOnLTB` |
| 2 | **Kudo, King Among Bears** | Two continuous effects: layer 7b SET base P/T 2/2 on other creatures; layer 4 ADD `bear` subtype on other creatures. SourcePerm scoping → LTB clears via `UnregisterContinuousEffectsForPermanent`. | `TestKudo_OtherCreaturesGetBase2_2AndBear` |
| 3 | **Clara Oswald** | `would_fire_etb_trigger` replacement filtered by Doctor subtype on `ev.Source.Card` + controller match. Non-ETB triggers stay partial (engine only exposes the ETB-trigger event today). | `TestClara_DoublesDoctorETBTrigger`, `TestClara_DoesNotDoubleNonDoctor`, `TestClara_IgnoresOpponentDoctor` |
| 4 | **Katara, the Fearless** | Same shape as Clara — Ally subtype filter. | `TestKatara_DoublesAllyETBTrigger`, `TestKatara_DoesNotDoubleNonAlly` |
| 5 | **The Twelfth Doctor** | OnTrigger("spell_copied") with caster filter → `AddCounter("+1/+1", 1)`. Requires the engine surface in row 10. Demonstrate-grant clause stays partial. | `TestTwelfthDoctor_AddsCounterOnSpellCopy`, `TestTwelfthDoctor_OpponentCopyDoesNotCount` |
| 6 | **Toph, the First Metalbender** | OnTrigger("end_step") gated on active_seat == controller: pick a non-creature land we control, stamp `earthbent`/`temp_haste`/2x +1/+1 counters, fire engine `Earthbend()`. | `TestToph1stMB_EndStepStampsLand`, `TestToph1stMB_DoesNotFireOnOpponentTurn` |
| 7 | **Aloy, Savior of Meridian** | Promote partial: existing artifact-attack max-power calculation now calls `gameengine.ApplyDiscover(gs, controller, X)` when X > 0. | `TestAloy_PromotedHandlerCallsApplyDiscover` |
| 8 | **Ivy, Gleeful Spellthief** | Promote partial: locate originating spell on stack by Card pointer, push `StackItem` copy with `IsCopy: true` and `Targets = [Ivy]`. Strionic-Resonator-style §707.2 primitive. | `TestIvy_PushesCopyRetargetingIvy`, `TestIvy_DoesNotFireOnNonCreatureTarget`, `TestIvy_DoesNotFireOnIvyHerself` |
| 9 | **Bello, Bard of the Brambles** | Four continuous effects scoped to the qualifying predicate (non-Equipment artifact OR non-Aura enchantment, MV ≥ 4, controller match, `gs.Active == controller`): layer 4 add creature + elemental subtype; layer 7b SET base P/T 4/4 (only when our layer-4 add brought it into creature-hood); layer 6 grant indestructible; layer 6 grant haste. OnTrigger("combat_damage_player") also routes the draw rider when a qualifying creature deals combat damage. | `TestBello_4PlusArtifactBecomes4_4ElementalDuringOwnerTurn`, `TestBello_DoesNotApplyOnOpponentTurn`, `TestBello_SubMV4ArtifactNotAffected`, `TestBello_EquipmentExcluded` |
| 10 | **engine surface — `FireCardTrigger("spell_copied")`** in `resolve.go` after the CR §707.2 push | Single additive line after the existing `FireMagecraftTriggers` call. No existing card listens for this event today (per pre-port grep), so it's a green-field hook. Per-card-driven copies (Kalamax, Alania, Riku) don't reach this dispatch — partial breadcrumbed in Twelfth Doctor's ETB. | implicitly verified by Twelfth Doctor tests (which would otherwise have no dispatch path) |

All twenty new tests pass; full `go test ./...` stays green (engine, per_card,
analytics, hat, hexapi, muninn, tournament, etc.).

## Out-of-scope (intentionally skipped)

- Cards whose stub is engine-side correct (`bello`, `hamza`, `rakdos`,
  `jasmine`, `the_master_multiplied`) — the breadcrumb is doing the right
  thing pointing at the real implementation site.
- Partial handlers with 100+ lines of real logic and one `emitPartial` clause
  pointing at deep-engine territory (e.g. `sokrates_athenian_teacher`,
  `progenitus`, `lyse_hext`, `galea_kindler_of_hope`). Those are real ports
  with one missing rider — not stub hunt fodder.
