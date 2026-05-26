# Audit: Engine Dead Branches (R60 Phase 1D)

Static analysis of `internal/gameengine` to surface dead-branch candidates without runtime
instrumentation. **Findings-only — no source files were modified.**

## Headline

| Metric | Value |
|---|---:|
| Declarations scanned | 5064 |
| Identifier references counted | 64166 |
| `exported_but_test_only` findings | 495 |
| `unused_switch_case_literals` findings | 140 |

## `exported_but_test_only` — 495 findings

Exported function or method declared in non-test code whose only
references outside the declaring file are in `_test.go` files.
Strong signal that the API surface exists only to support tests —
either the production caller was deleted (dead code) or the helper
should be unexported and moved next to the consumer.

_Note: methods are matched by simple name; if two unrelated types
expose the same method name, references conflate. Verify before acting._

| # | Symbol | Receiver | File:Line |
|---:|---|---|---|
| 1 | `WasDeclaredAttacker` | `Permanent` | `internal/gameengine/combat.go:178` |
| 2 | `CheckNinjutsu` | `—` | `internal/gameengine/combat.go:2049` |
| 3 | `DFCFrontFaceName` | `—` | `internal/gameengine/commander.go:498` |
| 4 | `CommanderCastCost` | `—` | `internal/gameengine/commander.go:607` |
| 5 | `DeclareCompanion` | `—` | `internal/gameengine/companion.go:23` |
| 6 | `MoveCompanionToHand` | `—` | `internal/gameengine/companion.go:45` |
| 7 | `ApplyCostModifiers` | `—` | `internal/gameengine/cost_modifiers.go:138` |
| 8 | `ScanCostModifiers` | `—` | `internal/gameengine/cost_modifiers.go:209` |
| 9 | `CanPayAlternativeCost` | `—` | `internal/gameengine/costs.go:210` |
| 10 | `ControlsCommander` | `—` | `internal/gameengine/costs.go:414` |
| 11 | `BargainAdditionalCost` | `—` | `internal/gameengine/costs.go:884` |
| 12 | `InitDFCFaces` | `—` | `internal/gameengine/dfc.go:360` |
| 13 | `SwapToBackFace` | `—` | `internal/gameengine/dfc.go:415` |
| 14 | `EnsureMDFCBackFaceForBattlefield` | `—` | `internal/gameengine/dfc.go:515` |
| 15 | `StripAdventureHalfTypes` | `—` | `internal/gameengine/dfc.go:575` |
| 16 | `CastWithAftermath` | `—` | `internal/gameengine/keywords_aftermath_cast.go:69` |
| 17 | `IsAftermathCast` | `—` | `internal/gameengine/keywords_aftermath_cast.go:182` |
| 18 | `SpellAftermathCastThisTurn` | `—` | `internal/gameengine/keywords_aftermath_cast.go:196` |
| 19 | `HasBargain` | `—` | `internal/gameengine/keywords_bargain.go:67` |
| 20 | `CanBargain` | `—` | `internal/gameengine/keywords_bargain.go:80` |
| 21 | `EligibleBargainTargets` | `—` | `internal/gameengine/keywords_bargain.go:101` |
| 22 | `CastWithBargain` | `—` | `internal/gameengine/keywords_bargain.go:165` |
| 23 | `IsPaired` | `—` | `internal/gameengine/keywords_batch2.go:42` |
| 24 | `HasReplicate` | `—` | `internal/gameengine/keywords_batch3.go:131` |
| 25 | `ReplicateCost` | `—` | `internal/gameengine/keywords_batch3.go:136` |
| 26 | `ApplyReplicate` | `—` | `internal/gameengine/keywords_batch3.go:151` |
| 27 | `ActivateSaddle` | `—` | `internal/gameengine/keywords_batch4.go:20` |
| 28 | `HasMaxSpeedKeyword` | `—` | `internal/gameengine/keywords_batch6.go:1098` |
| 29 | `HasMaxSpeed` | `—` | `internal/gameengine/keywords_batch6.go:1111` |
| 30 | `ApplyStartYourEngines` | `—` | `internal/gameengine/keywords_batch6.go:1137` |
| 31 | `HasHarmonize` | `—` | `internal/gameengine/keywords_batch6.go:1267` |
| 32 | `HasHarmonizeCard` | `—` | `internal/gameengine/keywords_batch6.go:1279` |
| 33 | `HarmonizeActivate` | `—` | `internal/gameengine/keywords_batch6.go:1305` |
| 34 | `HasMobilize` | `—` | `internal/gameengine/keywords_batch6.go:1412` |
| 35 | `MobilizeCount` | `—` | `internal/gameengine/keywords_batch6.go:1419` |
| 36 | `ApplyMobilize` | `—` | `internal/gameengine/keywords_batch6.go:1436` |
| 37 | `HasWarp` | `—` | `internal/gameengine/keywords_batch6.go:1583` |
| 38 | `CastWarp` | `—` | `internal/gameengine/keywords_batch6.go:1605` |
| 39 | `NewWarpCastFromExilePermission` | `—` | `internal/gameengine/keywords_batch6.go:1693` |
| 40 | `SpellWarpedThisTurn` | `—` | `internal/gameengine/keywords_batch6.go:1860` |
| 41 | `HasBattalion` | `—` | `internal/gameengine/keywords_battalion.go:46` |
| 42 | `PermanentHasBattalion` | `—` | `internal/gameengine/keywords_battalion.go:63` |
| 43 | `BattleDefenseCounters` | `—` | `internal/gameengine/keywords_battle.go:93` |
| 44 | `AddDefenseCounters` | `—` | `internal/gameengine/keywords_battle.go:113` |
| 45 | `RemoveDefenseCounters` | `—` | `internal/gameengine/keywords_battle.go:147` |
| 46 | `FireBattleZeroDefense` | `—` | `internal/gameengine/keywords_battle.go:199` |
| 47 | `IsBattleDefeated` | `—` | `internal/gameengine/keywords_battle.go:235` |
| 48 | `SetAttackerDefenderBattle` | `—` | `internal/gameengine/keywords_battle.go:258` |
| 49 | `Behold` | `—` | `internal/gameengine/keywords_behold.go:94` |
| 50 | `BeholdRevealFromHand` | `—` | `internal/gameengine/keywords_behold.go:146` |
| 51 | `BeholdChoosePermanent` | `—` | `internal/gameengine/keywords_behold.go:190` |
| 52 | `HasBeheld` | `—` | `internal/gameengine/keywords_behold.go:221` |
| 53 | `BeheldCount` | `—` | `internal/gameengine/keywords_behold.go:228` |
| 54 | `HasBloodrush` | `—` | `internal/gameengine/keywords_bloodrush.go:55` |
| 55 | `BloodrushCost` | `—` | `internal/gameengine/keywords_bloodrush.go:85` |
| 56 | `BloodrushPump` | `—` | `internal/gameengine/keywords_bloodrush.go:110` |
| 57 | `ActivateBloodrush` | `—` | `internal/gameengine/keywords_bloodrush.go:205` |
| 58 | `HasBuyback` | `—` | `internal/gameengine/keywords_buyback.go:49` |
| 59 | `BuybackCost` | `—` | `internal/gameengine/keywords_buyback.go:58` |
| 60 | `IsBoughtBack` | `—` | `internal/gameengine/keywords_buyback.go:93` |
| 61 | `CastBuyback` | `—` | `internal/gameengine/keywords_buyback.go:140` |
| 62 | `HasChannel` | `—` | `internal/gameengine/keywords_channel.go:48` |
| 63 | `ActivateChannel` | `—` | `internal/gameengine/keywords_channel.go:87` |
| 64 | `HasClass` | `—` | `internal/gameengine/keywords_class.go:70` |
| 65 | `ClassLevel` | `—` | `internal/gameengine/keywords_class.go:93` |
| 66 | `MaxClassLevel` | `—` | `internal/gameengine/keywords_class.go:115` |
| 67 | `LevelUpClass` | `—` | `internal/gameengine/keywords_class.go:178` |
| 68 | `ClassLevelStaticActive` | `—` | `internal/gameengine/keywords_class.go:291` |
| 69 | `ActiveClassBrackets` | `—` | `internal/gameengine/keywords_class.go:303` |
| 70 | `HasCleave` | `—` | `internal/gameengine/keywords_cleave.go:51` |
| 71 | `CleaveCost` | `—` | `internal/gameengine/keywords_cleave.go:61` |
| 72 | `BaseSpellEffect` | `—` | `internal/gameengine/keywords_cleave.go:68` |
| 73 | `CleaveEffect` | `—` | `internal/gameengine/keywords_cleave.go:81` |
| 74 | `CastWithCleave` | `—` | `internal/gameengine/keywords_cleave.go:133` |
| 75 | `SpellCleaveThisTurn` | `—` | `internal/gameengine/keywords_cleave.go:227` |
| 76 | `CanBlockIntimidate` | `—` | `internal/gameengine/keywords_combat.go:68` |
| 77 | `CanBlockFear` | `—` | `internal/gameengine/keywords_combat.go:99` |
| 78 | `CanBlockShadow` | `—` | `internal/gameengine/keywords_combat.go:126` |
| 79 | `CanBlockSkulk` | `—` | `internal/gameengine/keywords_combat.go:148` |
| 80 | `CanBlockDaunt` | `—` | `internal/gameengine/keywords_combat.go:165` |
| 81 | `GetRampageN` | `—` | `internal/gameengine/keywords_combat.go:301` |
| 82 | `ApplyRampage` | `—` | `internal/gameengine/keywords_combat.go:322` |
| 83 | `FireRampageTriggers` | `—` | `internal/gameengine/keywords_combat.go:354` |
| 84 | `ApplyBattleCry` | `—` | `internal/gameengine/keywords_combat.go:373` |
| 85 | `ApplyMyriad` | `—` | `internal/gameengine/keywords_combat.go:423` |
| 86 | `ApplyMelee` | `—` | `internal/gameengine/keywords_combat.go:508` |
| 87 | `ApplyAnnihilator` | `—` | `internal/gameengine/keywords_combat.go:577` |
| 88 | `GetAfflictN` | `—` | `internal/gameengine/keywords_combat.go:654` |
| 89 | `ApplyAfflict` | `—` | `internal/gameengine/keywords_combat.go:675` |
| 90 | `FireAfflictTriggers` | `—` | `internal/gameengine/keywords_combat.go:688` |
| 91 | `ApplyProvoke` | `—` | `internal/gameengine/keywords_combat.go:717` |
| 92 | `HasTrampleOverPlaneswalkers` | `—` | `internal/gameengine/keywords_combat.go:789` |
| 93 | `HasImprovise` | `—` | `internal/gameengine/keywords_combat.go:889` |
| 94 | `ImproviseCostReduction` | `—` | `internal/gameengine/keywords_combat.go:895` |
| 95 | `HasAssist` | `—` | `internal/gameengine/keywords_combat.go:917` |
| 96 | `HasUndaunted` | `—` | `internal/gameengine/keywords_combat.go:935` |
| 97 | `UndauntedReduction` | `—` | `internal/gameengine/keywords_combat.go:941` |
| 98 | `HasOffering` | `—` | `internal/gameengine/keywords_combat.go:958` |
| 99 | `OfferingReduction` | `—` | `internal/gameengine/keywords_combat.go:964` |
| 100 | `HasMiracle` | `—` | `internal/gameengine/keywords_combat.go:1034` |

_… 395 more (run with `--top 0` to dump all)._

## `unused_switch_case_literals` — 140 findings

Switch case arm whose string-literal value appears nowhere else
in the codebase (every other string literal in `internal/gameengine` was searched).
Strong signal that the matching emitter was removed without the
consumer; the case is unreachable.

**False-positive sources** to verify before acting:
- **Card-name switches** (`switch name { case "Storm-Kiln Artist": ... }`):
  the literal is a card name compared against `p.Card.Name`, which the
  engine reads from the JSON oracle dataset, not from Go source. Every
  such case will appear here even when the per-card handler is live.
  Verify by checking the switch's tag column: tags like `name`, `Card.Name`,
  or `p.Card.Name` strongly suggest a data-driven dispatch, not a dead case.
- **AST modification-kind switches** (`switch mod.ModKind { case "tri_tribe_anthem": ... }`):
  the literal is emitted by the Python parser (`scripts/mtg_ast.py`),
  not by Go code. Cross-check against the parser's emitter table.
- **Runtime-constructed strings** (`fmt.Sprintf`, `event.Kind = base + "x"`)
  can produce values this scan misses.
- **References from outside the module** (data dumps, generated configs)
  aren't scanned.

### By switch tag

| Switch tag | Count | Likely interpretation |
|---|---:|---|
| `e.ModKind` | 35 | AST enum from `scripts/mtg_ast.py` — cross-check parser, expected false positive |
| `base` | 21 | AST enum from `scripts/mtg_ast.py` — cross-check parser, expected false positive |
| `f.Base` | 18 | AST enum from `scripts/mtg_ast.py` — cross-check parser, expected false positive |
| `name` | 18 | card-name dispatch — values come from JSON data, expected false positive |
| `exLow` | 11 | verify the emitter is genuinely absent |
| `mod.ModKind` | 10 | AST enum from `scripts/mtg_ast.py` — cross-check parser, expected false positive |
| `sa.ScalingKind` | 6 | AST enum from `scripts/mtg_ast.py` — cross-check parser, expected false positive |
| `p.Card.DisplayName(…)` | 5 | card-name dispatch — values come from JSON data, expected false positive |
| `prefix` | 3 | verify the emitter is genuinely absent |
| `r` | 3 | verify the emitter is genuinely absent |
| `e.Actor` | 2 | verify the emitter is genuinely absent |
| `op` | 2 | verify the emitter is genuinely absent |
| `st.Modification.ModKind` | 2 | AST enum from `scripts/mtg_ast.py` — cross-check parser, expected false positive |
| `ctrl` | 1 | verify the emitter is genuinely absent |
| `e.Query.Base` | 1 | AST enum from `scripts/mtg_ast.py` — cross-check parser, expected false positive |
| `f` | 1 | verify the emitter is genuinely absent |
| `t` | 1 | verify the emitter is genuinely absent |

### Per-case detail

| # | Literal value | Switched on | File:Line |
|---:|---|---|---|
| 1 | `Storm-Kiln Artist` | `name` | `internal/gameengine/cast_counts.go:221` |
| 2 | `Third Path Iconoclast` | `name` | `internal/gameengine/cast_counts.go:248` |
| 3 | `Monastery Mentor` | `name` | `internal/gameengine/cast_counts.go:262` |
| 4 | `Vryn Wingmare` | `name` | `internal/gameengine/cost_modifiers.go:476` |
| 5 | `Thorn of Amethyst` | `name` | `internal/gameengine/cost_modifiers.go:476` |
| 6 | `Glowrider` | `name` | `internal/gameengine/cost_modifiers.go:476` |
| 7 | `Lodestone Golem` | `name` | `internal/gameengine/cost_modifiers.go:520` |
| 8 | `Baral, Chief of Compliance` | `name` | `internal/gameengine/cost_modifiers.go:540` |
| 9 | `Jet Medallion` | `name` | `internal/gameengine/cost_modifiers.go:635` |
| 10 | `Ruby Medallion` | `name` | `internal/gameengine/cost_modifiers.go:643` |
| 11 | `Pearl Medallion` | `name` | `internal/gameengine/cost_modifiers.go:651` |
| 12 | `Emerald Medallion` | `name` | `internal/gameengine/cost_modifiers.go:659` |
| 13 | `Edgewalker` | `name` | `internal/gameengine/cost_modifiers.go:668` |
| 14 | `Nightscape Familiar` | `name` | `internal/gameengine/cost_modifiers.go:714` |
| 15 | `green creature` | `f` | `internal/gameengine/costs.go:549` |
| 16 | `activated_ability` | `base` | `internal/gameengine/counter_resolve.go:167` |
| 17 | `or` | `base` | `internal/gameengine/counter_resolve.go:256` |
| 18 | `nontoken_yours_anthem` | `mod.ModKind` | `internal/gameengine/layers.go:2330` |
| 19 | `other_creatures_global_pt` | `mod.ModKind` | `internal/gameengine/layers.go:2337` |
| 20 | `tri_tribe_anthem` | `mod.ModKind` | `internal/gameengine/layers.go:2358` |
| 21 | `tribe_global_pt` | `mod.ModKind` | `internal/gameengine/layers.go:2365` |
| 22 | `non_type_global_pt` | `mod.ModKind` | `internal/gameengine/layers.go:2371` |
| 23 | `opp_creatures_base_pt` | `mod.ModKind` | `internal/gameengine/layers.go:2377` |
| 24 | `commander_anthem` | `mod.ModKind` | `internal/gameengine/layers.go:2384` |
| 25 | `tribe_yours_anthem_have` | `mod.ModKind` | `internal/gameengine/layers.go:2391` |
| 26 | `tribe_anthem_have` | `mod.ModKind` | `internal/gameengine/layers.go:2391` |
| 27 | `enchanted_creature_pt` | `mod.ModKind` | `internal/gameengine/layers.go:2413` |
| 28 | `non_creature_activation_only` | `r` | `internal/gameengine/mana.go:212` |
| 29 | `noncreature_activation_only` | `r` | `internal/gameengine/mana.go:213` |
| 30 | `instant_or_sorcery_only` | `r` | `internal/gameengine/mana.go:218` |
| 31 | `misthollow griffin` | `name` | `internal/gameengine/per_card/food_chain.go:179` |
| 32 | `eternal scourge` | `name` | `internal/gameengine/per_card/food_chain.go:180` |
| 33 | `squee the immortal` | `name` | `internal/gameengine/per_card/food_chain.go:181` |
| 34 | `torrent elemental` | `name` | `internal/gameengine/per_card/food_chain.go:182` |
| 35 | `pip:C` | `t` | `internal/gameengine/per_card/hashaton.go:87` |
| 36 | `Genesis` | `p.Card.DisplayName(…)` | `internal/gameengine/per_card/sac_outlets.go:587` |
| 37 | `The Cauldron of Eternity` | `p.Card.DisplayName(…)` | `internal/gameengine/per_card/sac_outlets.go:590` |
| 38 | `Sheoldred, Whispering One` | `p.Card.DisplayName(…)` | `internal/gameengine/per_card/sac_outlets.go:591` |
| 39 | `Liliana, Death's Majesty` | `p.Card.DisplayName(…)` | `internal/gameengine/per_card/sac_outlets.go:592` |
| 40 | `Whisper, Blood Liturgist` | `p.Card.DisplayName(…)` | `internal/gameengine/per_card/sac_outlets.go:593` |
| 41 | `active_player` | `ctrl` | `internal/gameengine/phases.go:188` |
| 42 | `!=` | `op` | `internal/gameengine/resolve.go:565` |
| 43 | `that_thing` | `e.Query.Base` | `internal/gameengine/resolve.go:1657` |
| 44 | `each_opponent` | `e.Actor` | `internal/gameengine/resolve.go:1681` |
| 45 | `that_player_choice` | `e.Actor` | `internal/gameengine/resolve.go:1692` |
| 46 | `activation_restriction` | `e.ModKind` | `internal/gameengine/resolve_helpers.go:1777` |
| 47 | `face_down_copy_effect` | `e.ModKind` | `internal/gameengine/resolve_helpers.go:1777` |
| 48 | `this_spell_colored_cost_reduce` | `e.ModKind` | `internal/gameengine/resolve_helpers.go:1778` |
| 49 | `for_each_rider` | `e.ModKind` | `internal/gameengine/resolve_helpers.go:1779` |
| 50 | `mana_restriction` | `e.ModKind` | `internal/gameengine/resolve_helpers.go:1780` |
| 51 | `typed_you_control_have` | `e.ModKind` | `internal/gameengine/resolve_helpers.go:1780` |
| 52 | `equip_buff_grant` | `e.ModKind` | `internal/gameengine/resolve_helpers.go:1780` |
| 53 | `copy_retarget` | `e.ModKind` | `internal/gameengine/resolve_helpers.go:1781` |
| 54 | `aura_grant` | `e.ModKind` | `internal/gameengine/resolve_helpers.go:1781` |
| 55 | `type_add_still` | `e.ModKind` | `internal/gameengine/resolve_helpers.go:1782` |
| 56 | `etb_tapped_unless` | `e.ModKind` | `internal/gameengine/resolve_helpers.go:1782` |
| 57 | `colored_cost_reduce` | `e.ModKind` | `internal/gameengine/resolve_helpers.go:1782` |
| 58 | `cast_without_paying_static` | `e.ModKind` | `internal/gameengine/resolve_helpers.go:1783` |
| 59 | `no_regen_tail_it` | `e.ModKind` | `internal/gameengine/resolve_helpers.go:1784` |
| 60 | `equip_grant` | `e.ModKind` | `internal/gameengine/resolve_helpers.go:1785` |
| 61 | `until_next_turn` | `e.ModKind` | `internal/gameengine/resolve_helpers.go:1785` |
| 62 | `aura_no_untap` | `e.ModKind` | `internal/gameengine/resolve_helpers.go:1786` |
| 63 | `group_quoted_ability_grant` | `e.ModKind` | `internal/gameengine/resolve_helpers.go:1787` |
| 64 | `orphan_choice` | `e.ModKind` | `internal/gameengine/resolve_helpers.go:1788` |
| 65 | `no_untap_conditional` | `e.ModKind` | `internal/gameengine/resolve_helpers.go:1788` |
| 66 | `aura_restriction` | `e.ModKind` | `internal/gameengine/resolve_helpers.go:1788` |
| 67 | `etb_may_copy` | `e.ModKind` | `internal/gameengine/resolve_helpers.go:1789` |
| 68 | `inline_modal_with_bullets` | `e.ModKind` | `internal/gameengine/resolve_helpers.go:1790` |
| 69 | `modal_header_orphan` | `e.ModKind` | `internal/gameengine/resolve_helpers.go:1790` |
| 70 | `play_those_this_turn` | `e.ModKind` | `internal/gameengine/resolve_helpers.go:1791` |
| 71 | `cast_restriction` | `e.ModKind` | `internal/gameengine/resolve_helpers.go:1791` |
| 72 | `extra_block` | `e.ModKind` | `internal/gameengine/resolve_helpers.go:1791` |
| 73 | `when_you_do_p1p1` | `e.ModKind` | `internal/gameengine/resolve_helpers.go:1792` |
| 74 | `optional_skip_untap_self` | `e.ModKind` | `internal/gameengine/resolve_helpers.go:1792` |
| 75 | `during_turn_self_static` | `e.ModKind` | `internal/gameengine/resolve_helpers.go:1793` |
| 76 | `fetch_land_tail` | `e.ModKind` | `internal/gameengine/resolve_helpers.go:1793` |
| 77 | `reanimate_that_card_tail` | `e.ModKind` | `internal/gameengine/resolve_helpers.go:1794` |
| 78 | `stun_target` | `e.ModKind` | `internal/gameengine/resolve_helpers.go:1794` |
| 79 | `mana_retention` | `e.ModKind` | `internal/gameengine/resolve_helpers.go:1795` |
| 80 | `opp_choice_card_pick` | `e.ModKind` | `internal/gameengine/resolve_helpers.go:1795` |
| 81 | `literal` | `sa.ScalingKind` | `internal/gameengine/scaling.go:97` |
| 82 | `permanents_you_control` | `sa.ScalingKind` | `internal/gameengine/scaling.go:121` |
| 83 | `cards_in_zone` | `sa.ScalingKind` | `internal/gameengine/scaling.go:130` |
| 84 | `counters_on_self` | `sa.ScalingKind` | `internal/gameengine/scaling.go:137` |
| 85 | `count_filter` | `sa.ScalingKind` | `internal/gameengine/scaling.go:206` |
| 86 | `tapped_creatures_you_control` | `sa.ScalingKind` | `internal/gameengine/scaling.go:243` |
| 87 | `cast_timing_opp_sorcery` | `st.Modification.ModKind` | `internal/gameengine/stack.go:1748` |
| 88 | `opp_only_sorcery_speed` | `st.Modification.ModKind` | `internal/gameengine/stack.go:1749` |
| 89 | `equipped_creature` | `f.Base` | `internal/gameengine/targets.go:75` |
| 90 | `enchanted_creature` | `f.Base` | `internal/gameengine/targets.go:76` |
| 91 | `enchanted_permanent` | `f.Base` | `internal/gameengine/targets.go:77` |
| 92 | `enchanted_land` | `f.Base` | `internal/gameengine/targets.go:77` |
| 93 | `them` | `f.Base` | `internal/gameengine/targets.go:97` |
| 94 | `that_thing` | `f.Base` | `internal/gameengine/targets.go:97` |
| 95 | `them` | `f.Base` | `internal/gameengine/targets.go:125` |
| 96 | `that_thing` | `f.Base` | `internal/gameengine/targets.go:125` |
| 97 | `equipped_creature` | `f.Base` | `internal/gameengine/targets.go:128` |
| 98 | `enchanted_creature` | `f.Base` | `internal/gameengine/targets.go:129` |
| 99 | `enchanted_permanent` | `f.Base` | `internal/gameengine/targets.go:130` |
| 100 | `enchanted_land` | `f.Base` | `internal/gameengine/targets.go:130` |

_… 40 more (run with `--top 0` to dump all)._

## Methodology

- `go/ast` parses every `.go` file under `internal/gameengine`. Test files (`*_test.go`)
  participate in reference counting but their declarations are NOT
  collected — dead-branch findings about test helpers aren't actionable.
- References are collected as both `*ast.Ident` and `*ast.SelectorExpr.Sel`,
  matching by simple name. Receiver-qualified disambiguation isn't
  attempted; the over-counting bias is conservative (less likely to
  flag a real dead branch).
- Self-references (a declaration's name appearing in its own declaring
  file) are filtered before classification.
- Switch case arms are extracted via `*ast.SwitchStmt`. Type-switch
  arms (which switch on types, not string literals) are out of scope.

## Reproducing this report

```
go run ./cmd/audit-engine-dead --dir internal/gameengine --out docs/audit-engine-dead-branches-r60.md --top 100
```
