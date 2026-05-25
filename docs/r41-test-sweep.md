# R41 Test Files Sweep (2026-05-23)

Five untracked test files dating from the r41 era were swept on branch
`dev/r41-test-files-sweep-r60`. Four passed and were committed; one was
skipped because every test asserts behavior the corresponding per_card
handlers were explicitly written *not* to implement.

## Committed (passing)

- `internal/gameengine/combat_damage_edge_r41_test.go` — Humility +
  Opalescence termination / cache churn / register churn / deep-stack /
  bounded-stack-growth regressions.
- `internal/gameengine/mana_ability_605_r41_test.go` — CR §605 mana
  ability semantics (no stack, can't be responded to, mana produced
  during cost payment, Priest of Urabrask triggered-not-mana).
- `internal/gameengine/replacement_loop_r41_test.go` — combat damage
  edge cases (multi-blocker order, first-strike + deathtouch mirror,
  trample + deathtouch carry to player, double-strike lifelink, menace,
  protection-on-attacker).
- `internal/gameengine/targeting_protection_r41_test.go` — hexproof /
  shroud / protection-from-color / ward targeting protections.

## Resolved (2026-05-24, dev/percard-batchH-followup-r60)

The deferred test file is now adopted. All 10 handlers were extended
from `emitPartial` breadcrumbs to the actual gameplay-impacting
branches the tests assert:

- **Mabel** — captures Cragflame perm from `enterBattlefieldWithETB`,
  picks best friendly Mouse by base power (tie → non-Mabel), sets
  `AttachedTo` + stamps `kw:vigilance` / `kw:trample` / `kw:haste`.
- **Raphael** — ETB sets `must_be_blocked` perm flag.
- **The Twelfth Doctor** — ETB arms
  `twelfth_doctor_demonstrate_pending` seat flag; new `spell_cast`
  trigger consumes it when `cast_zone != "hand"`.
- **Storm, Force of Nature** — new `spell_cast` trigger consumes
  `storm_grant_pending` when the cast card is instant or sorcery.
- **Cloud, Midgar Mercenary** — registers `would_fire_etb_trigger`
  ReplacementEffect that increments count when Cloud has any
  attached perm (`p.AttachedTo == cloud` scan); LTB unregisters.
- **Lightning, Army of One** — adds parallel
  `would_be_dealt_damage` ReplacementEffect (sibling to the existing
  combat-path `DamageReplacement`) so `FireDamageEvent` (which only
  walks `gs.Replacements`) sees the doubling; sets
  `gs.Flags["lightning_stagger_seat<N>_until_turn"]` breadcrumb.
- **The Master of Keys** — ETB sets
  `master_of_keys_enchantment_escape_grant` seat flag (load-bearing
  ZoneCastPolicy was already there); LTB clears it.
- **Aminatou, Veil Piercer** — new case in
  `gameengine/cost_modifiers.go` switch: enchantment spells by
  Aminatou's controller get `CostModReduction Amount: 4` (the
  miracle math, gating on first-card-drawn left to the cast pipeline).
- **Jasmine Boreal** — `{T}` activation now actually taps, adds 2
  to `seat.ManaPool`, and increments
  `jasmine_no_ability_creature_mana` breadcrumb flag.
- **Commander Mustard** — new `creature_attacks` trigger; when the
  per-seat `mustard_soldier_attack_ping` flag is armed, soldier
  attackers reduce defender life by 1 (non-soldiers ignored).

Repro: `go test ./internal/gameengine/per_card -run '^TestStubsBatchH_' -count=1`
(all 10 pass).

## Skipped — needs handler work, not a test fix

- ~~`internal/gameengine/per_card/percard_stubs_batchH_r51_test.go`~~
  — RESOLVED above. Original notes preserved for context:

- The handlers (`gen_mabel_heir_to_cragflame.go`,
  `gen_raphael_ninja_destroyer.go`, `gen_the_twelfth_doctor.go`,
  `gen_storm_force_of_nature.go`, `gen_cloud_midgar_mercenary.go`,
  `gen_lightning_army_of_one.go`, `gen_the_master_of_keys.go`,
  `gen_aminatou_veil_piercer.go`, `gen_jasmine_boreal_of_the_seven.go`,
  `gen_commander_mustard.go`) all currently `emitPartial(...)` the
  gameplay-impacting branch and stop. e.g. Mabel creates the Cragflame
  token but explicitly does not auto-equip
  (`cragflame_equip_grant_not_auto_attached_engine_lacks_equip_planner`).
  The tests instead assert end-to-end behavior:
  auto-attachment, ETB flag stamping, demonstrate grant arming,
  storm-grant consumption, equipped-trigger doubling, stagger arming,
  escape-grant flag, miracle cost reduction, activated tap producing
  mana + restriction flag, soldier-attack pings.

  Filing this as a Batch-H follow-up: either flesh out each
  `emitPartial` to actual gameplay, or scope each test down to assert
  only the partial-emit side and reinstate.

  Repro:
  `go test ./internal/gameengine/per_card -run '^TestStubsBatchH_' -count=1`
