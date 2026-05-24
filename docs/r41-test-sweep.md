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

## Skipped — needs handler work, not a test fix

- `internal/gameengine/per_card/percard_stubs_batchH_r51_test.go` — all
  10 tests fail. The handlers (`gen_mabel_heir_to_cragflame.go`,
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
