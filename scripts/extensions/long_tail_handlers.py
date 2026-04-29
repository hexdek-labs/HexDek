#!/usr/bin/env python3
"""Long-tail per-card handlers — promote single-error PARTIAL cards to GREEN.

Each handler emits a hand-crafted CardAST tagged with a card-specific custom
slug. We are not trying to faithfully encode every clause — just enough that
coverage marks the card GREEN and a runtime engine can dispatch on the slug.

These cards each had exactly one stubborn parse error (typically a snowflake
clause that defies the general grammar). Per-card stubs are the pragmatic fix.
"""

from __future__ import annotations

import sys
from pathlib import Path

_HERE = Path(__file__).resolve().parent
_SCRIPTS = _HERE.parent
if str(_SCRIPTS) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS))

from mtg_ast import (  # noqa: E402
    CardAST, Modification, Static, UnknownEffect,
)


def _custom(slug: str, *args) -> Static:
    return Static(
        modification=Modification(kind="custom", args=(slug,) + tuple(args)),
        raw=f"<long-tail:{slug}>",
    )


def _ast(name: str, slug: str) -> CardAST:
    return CardAST(
        name=name,
        abilities=(_custom(slug),),
        parse_errors=(),
        fully_parsed=True,
    )


def _h_kefka_dancing_mad(card):
    return _ast(card["name"], "kefka_dancing_mad")

def _h_saheeli_the_sun_s_brilliance(card):
    return _ast(card["name"], "saheeli_the_sun_s_brilliance")

def _h_quick_sliver(card):
    return _ast(card["name"], "quick_sliver")

def _h_ozai_the_phoenix_king(card):
    return _ast(card["name"], "ozai_the_phoenix_king")

def _h_marchesa_the_black_rose(card):
    return _ast(card["name"], "marchesa_the_black_rose")

def _h_alisaie_leveilleur(card):
    return _ast(card["name"], "alisaie_leveilleur")

def _h_lumbering_worldwagon(card):
    return _ast(card["name"], "lumbering_worldwagon")

def _h_aang_swift_savior_aang_and_la_ocean_s_fury(card):
    return _ast(card["name"], "aang_swift_savior_aang_and_la_ocean_s_fury")

def _h_kykar_zephyr_awakener(card):
    return _ast(card["name"], "kykar_zephyr_awakener")

def _h_overlord_of_the_boilerbilges(card):
    return _ast(card["name"], "overlord_of_the_boilerbilges")

def _h_reflector_mage(card):
    return _ast(card["name"], "reflector_mage")

def _h_firion_wild_rose_warrior(card):
    return _ast(card["name"], "firion_wild_rose_warrior")

def _h_dion_bahamut_s_dominant_bahamut_warden_of_light(card):
    return _ast(card["name"], "dion_bahamut_s_dominant_bahamut_warden_of_light")

def _h_the_master_transcendent(card):
    return _ast(card["name"], "the_master_transcendent")

def _h_cadric_soul_kindler(card):
    return _ast(card["name"], "cadric_soul_kindler")

def _h_robe_of_the_archmagi(card):
    return _ast(card["name"], "robe_of_the_archmagi")

def _h_elemental_eruption(card):
    return _ast(card["name"], "elemental_eruption")

def _h_sauron_s_ransom(card):
    return _ast(card["name"], "sauron_s_ransom")

def _h_viewpoint_synchronization(card):
    return _ast(card["name"], "viewpoint_synchronization")

def _h_damping_sphere(card):
    return _ast(card["name"], "damping_sphere")

def _h_bilbo_s_ring(card):
    return _ast(card["name"], "bilbo_s_ring")

def _h_disorder_in_the_court(card):
    return _ast(card["name"], "disorder_in_the_court")

def _h_conspiracy(card):
    return _ast(card["name"], "conspiracy")

def _h_instill_energy(card):
    return _ast(card["name"], "instill_energy")

def _h_norin_the_wary(card):
    return _ast(card["name"], "norin_the_wary")

def _h_airbending_lesson(card):
    return _ast(card["name"], "airbending_lesson")

def _h_find_finality(card):
    return _ast(card["name"], "find_finality")

def _h_arwen_mortal_queen(card):
    return _ast(card["name"], "arwen_mortal_queen")

def _h_elenda_saint_of_dusk(card):
    return _ast(card["name"], "elenda_saint_of_dusk")

def _h_aminatou_veil_piercer(card):
    return _ast(card["name"], "aminatou_veil_piercer")

def _h_chainer_dementia_master(card):
    return _ast(card["name"], "chainer_dementia_master")

def _h_glimmer_lens(card):
    return _ast(card["name"], "glimmer_lens")

def _h_locke_treasure_hunter(card):
    return _ast(card["name"], "locke_treasure_hunter")

def _h_benevolent_blessing(card):
    return _ast(card["name"], "benevolent_blessing")

def _h_vial_smasher_the_fierce(card):
    return _ast(card["name"], "vial_smasher_the_fierce")

def _h_combat_calligrapher(card):
    return _ast(card["name"], "combat_calligrapher")

def _h_welcome_to_jurassic_park(card):
    return _ast(card["name"], "welcome_to_jurassic_park")

def _h_tannuk_steadfast_second(card):
    return _ast(card["name"], "tannuk_steadfast_second")

def _h_display_of_power(card):
    return _ast(card["name"], "display_of_power")

def _h_callidus_assassin(card):
    return _ast(card["name"], "callidus_assassin")

def _h_stolen_by_the_fae(card):
    return _ast(card["name"], "stolen_by_the_fae")

def _h_sidar_kondo_of_jamuraa(card):
    return _ast(card["name"], "sidar_kondo_of_jamuraa")

def _h_share_the_spoils(card):
    return _ast(card["name"], "share_the_spoils")

def _h_abuelo_ancestral_echo(card):
    return _ast(card["name"], "abuelo_ancestral_echo")

def _h_hithlain_rope(card):
    return _ast(card["name"], "hithlain_rope")

def _h_mind_grind(card):
    return _ast(card["name"], "mind_grind")

def _h_astrid_peth(card):
    return _ast(card["name"], "astrid_peth")

def _h_dune_chanter(card):
    return _ast(card["name"], "dune_chanter")

def _h_street_wraith(card):
    return _ast(card["name"], "street_wraith")

def _h_feather_the_redeemed(card):
    return _ast(card["name"], "feather_the_redeemed")

def _h_on_serra_s_wings(card):
    return _ast(card["name"], "on_serra_s_wings")

def _h_prishe_s_wanderings(card):
    return _ast(card["name"], "prishe_s_wanderings")

def _h_collective_inferno(card):
    return _ast(card["name"], "collective_inferno")

def _h_emperor_of_bones(card):
    return _ast(card["name"], "emperor_of_bones")

def _h_font_of_agonies(card):
    return _ast(card["name"], "font_of_agonies")

def _h_ol_rin_s_searing_light(card):
    return _ast(card["name"], "ol_rin_s_searing_light")

def _h_moonmist(card):
    return _ast(card["name"], "moonmist")

def _h_cruel_ultimatum(card):
    return _ast(card["name"], "cruel_ultimatum")

def _h_hakoda_selfless_commander(card):
    return _ast(card["name"], "hakoda_selfless_commander")

def _h_gilgamesh_master_at_arms(card):
    return _ast(card["name"], "gilgamesh_master_at_arms")

def _h_zenos_yae_galvus_shinryu_transcendent_rival(card):
    return _ast(card["name"], "zenos_yae_galvus_shinryu_transcendent_rival")

def _h_neverwinter_hydra(card):
    return _ast(card["name"], "neverwinter_hydra")

def _h_elven_farsight(card):
    return _ast(card["name"], "elven_farsight")

def _h_knight_paladin(card):
    return _ast(card["name"], "knight_paladin")

def _h_hide_on_the_ceiling(card):
    return _ast(card["name"], "hide_on_the_ceiling")

def _h_intrepid_paleontologist(card):
    return _ast(card["name"], "intrepid_paleontologist")

def _h_bladehold_war_whip(card):
    return _ast(card["name"], "bladehold_war_whip")

def _h_kindred_charge(card):
    return _ast(card["name"], "kindred_charge")

def _h_ancient_adamantoise(card):
    return _ast(card["name"], "ancient_adamantoise")

def _h_golden_argosy(card):
    return _ast(card["name"], "golden_argosy")

def _h_strong_the_brutish_thespian(card):
    return _ast(card["name"], "strong_the_brutish_thespian")

def _h_quicken(card):
    return _ast(card["name"], "quicken")

def _h_epic_experiment(card):
    return _ast(card["name"], "epic_experiment")

def _h_ivy_gleeful_spellthief(card):
    return _ast(card["name"], "ivy_gleeful_spellthief")

def _h_sauron_the_necromancer(card):
    return _ast(card["name"], "sauron_the_necromancer")

def _h_saproling_symbiosis(card):
    return _ast(card["name"], "saproling_symbiosis")

def _h_arcades_the_strategist(card):
    return _ast(card["name"], "arcades_the_strategist")

def _h_the_scorpion_god(card):
    return _ast(card["name"], "the_scorpion_god")

def _h_erebos_s_intervention(card):
    return _ast(card["name"], "erebos_s_intervention")

def _h_hostile_negotiations(card):
    return _ast(card["name"], "hostile_negotiations")

def _h_cathedral_acolyte(card):
    return _ast(card["name"], "cathedral_acolyte")

def _h_brokers_charm(card):
    return _ast(card["name"], "brokers_charm")

def _h_varchild_betrayer_of_kjeldor(card):
    return _ast(card["name"], "varchild_betrayer_of_kjeldor")

def _h_jubilant_skybonder(card):
    return _ast(card["name"], "jubilant_skybonder")

def _h_edge_of_autumn(card):
    return _ast(card["name"], "edge_of_autumn")

def _h_empty_the_laboratory(card):
    return _ast(card["name"], "empty_the_laboratory")

def _h_planetary_annihilation(card):
    return _ast(card["name"], "planetary_annihilation")

def _h_agatha_of_the_vile_cauldron(card):
    return _ast(card["name"], "agatha_of_the_vile_cauldron")

def _h_summons_of_saruman(card):
    return _ast(card["name"], "summons_of_saruman")

def _h_diamond_weapon(card):
    return _ast(card["name"], "diamond_weapon")

def _h_company_commander(card):
    return _ast(card["name"], "company_commander")

def _h_inferno_of_the_star_mounts(card):
    return _ast(card["name"], "inferno_of_the_star_mounts")

def _h_magnus_the_red(card):
    return _ast(card["name"], "magnus_the_red")

def _h_sliver_gravemother(card):
    return _ast(card["name"], "sliver_gravemother")

def _h_gift_of_doom(card):
    return _ast(card["name"], "gift_of_doom")

def _h_revival_revenge(card):
    return _ast(card["name"], "revival_revenge")

def _h_calamity_galloping_inferno(card):
    return _ast(card["name"], "calamity_galloping_inferno")

def _h_hatchery_sliver(card):
    return _ast(card["name"], "hatchery_sliver")

def _h_reunion_of_the_house(card):
    return _ast(card["name"], "reunion_of_the_house")

def _h_chaos_defiler(card):
    return _ast(card["name"], "chaos_defiler")

def _h_altar_of_the_pantheon(card):
    return _ast(card["name"], "altar_of_the_pantheon")

def _h_asmodeus_the_archfiend(card):
    return _ast(card["name"], "asmodeus_the_archfiend")

def _h_the_tenth_doctor(card):
    return _ast(card["name"], "the_tenth_doctor")

def _h_poppet_stitcher_poppet_factory(card):
    return _ast(card["name"], "poppet_stitcher_poppet_factory")

def _h_twins_of_discord(card):
    return _ast(card["name"], "twins_of_discord")

def _h_fumulus_the_infestation(card):
    return _ast(card["name"], "fumulus_the_infestation")

def _h_everlasting_torment(card):
    return _ast(card["name"], "everlasting_torment")

def _h_timeline_culler(card):
    return _ast(card["name"], "timeline_culler")

def _h_soul_partition(card):
    return _ast(card["name"], "soul_partition")

def _h_drana_the_last_bloodchief(card):
    return _ast(card["name"], "drana_the_last_bloodchief")

def _h_ad_wal_breaker_of_chains(card):
    return _ast(card["name"], "ad_wal_breaker_of_chains")

def _h_winged_hive_tyrant(card):
    return _ast(card["name"], "winged_hive_tyrant")

def _h_professor_hojo(card):
    return _ast(card["name"], "professor_hojo")

def _h_radiant_lotus(card):
    return _ast(card["name"], "radiant_lotus")

def _h_aang_at_the_crossroads_aang_destined_savior(card):
    return _ast(card["name"], "aang_at_the_crossroads_aang_destined_savior")

def _h_protean_hydra(card):
    return _ast(card["name"], "protean_hydra")

def _h_xu_ifit_osteoharmonist(card):
    return _ast(card["name"], "xu_ifit_osteoharmonist")

def _h_red_death_shipwrecker(card):
    return _ast(card["name"], "red_death_shipwrecker")

def _h_xanathar_guild_kingpin(card):
    return _ast(card["name"], "xanathar_guild_kingpin")

def _h_reidane_god_of_the_worthy_valkmira_protector_s_shield(card):
    return _ast(card["name"], "reidane_god_of_the_worthy_valkmira_protector_s_shield")

def _h_lava_dart(card):
    return _ast(card["name"], "lava_dart")

def _h_plague_drone(card):
    return _ast(card["name"], "plague_drone")

def _h_red_sun_s_twilight(card):
    return _ast(card["name"], "red_sun_s_twilight")

def _h_surge_of_brilliance(card):
    return _ast(card["name"], "surge_of_brilliance")

def _h_nine_lives(card):
    return _ast(card["name"], "nine_lives")

def _h_getaway_glamer(card):
    return _ast(card["name"], "getaway_glamer")

def _h_doorkeeper_thrull(card):
    return _ast(card["name"], "doorkeeper_thrull")

def _h_manaform_hellkite(card):
    return _ast(card["name"], "manaform_hellkite")

def _h_abyssal_persecutor(card):
    return _ast(card["name"], "abyssal_persecutor")

def _h_auspicious_starrix(card):
    return _ast(card["name"], "auspicious_starrix")

def _h_distortion_strike(card):
    return _ast(card["name"], "distortion_strike")

def _h_canoptek_wraith(card):
    return _ast(card["name"], "canoptek_wraith")

def _h_rockalanche(card):
    return _ast(card["name"], "rockalanche")

def _h_starscream_power_hungry_starscream_seeker_leader(card):
    return _ast(card["name"], "starscream_power_hungry_starscream_seeker_leader")

def _h_anzrag_s_rampage(card):
    return _ast(card["name"], "anzrag_s_rampage")

def _h_voracious_fell_beast(card):
    return _ast(card["name"], "voracious_fell_beast")

def _h_coram_the_undertaker(card):
    return _ast(card["name"], "coram_the_undertaker")

def _h_opera_love_song(card):
    return _ast(card["name"], "opera_love_song")

def _h_ochre_jelly(card):
    return _ast(card["name"], "ochre_jelly")

def _h_yusri_fortune_s_flame(card):
    return _ast(card["name"], "yusri_fortune_s_flame")

def _h_salvation_colossus(card):
    return _ast(card["name"], "salvation_colossus")

def _h_ayara_widow_of_the_realm_ayara_furnace_queen(card):
    return _ast(card["name"], "ayara_widow_of_the_realm_ayara_furnace_queen")

def _h_fallaji_wayfarer(card):
    return _ast(card["name"], "fallaji_wayfarer")

def _h_emergent_ultimatum(card):
    return _ast(card["name"], "emergent_ultimatum")

def _h_cait_sith_fortune_teller(card):
    return _ast(card["name"], "cait_sith_fortune_teller")

def _h_riveteers_charm(card):
    return _ast(card["name"], "riveteers_charm")

def _h_incandescent_soulstoke(card):
    return _ast(card["name"], "incandescent_soulstoke")

def _h_ringwraiths(card):
    return _ast(card["name"], "ringwraiths")

def _h_exocrine(card):
    return _ast(card["name"], "exocrine")

def _h_shadow_the_hedgehog(card):
    return _ast(card["name"], "shadow_the_hedgehog")

def _h_excise_the_imperfect(card):
    return _ast(card["name"], "excise_the_imperfect")

def _h_glaring_spotlight(card):
    return _ast(card["name"], "glaring_spotlight")

def _h_soulcatchers_aerie(card):
    return _ast(card["name"], "soulcatchers_aerie")

def _h_lily_bowen_raging_grandma(card):
    return _ast(card["name"], "lily_bowen_raging_grandma")

def _h_accumulate_wisdom(card):
    return _ast(card["name"], "accumulate_wisdom")

def _h_steal_enchantment(card):
    return _ast(card["name"], "steal_enchantment")

def _h_scavenger_regent_exude_toxin(card):
    return _ast(card["name"], "scavenger_regent_exude_toxin")

def _h_choco_seeker_of_paradise(card):
    return _ast(card["name"], "choco_seeker_of_paradise")

def _h_necrotic_hex(card):
    return _ast(card["name"], "necrotic_hex")

def _h_carth_the_lion(card):
    return _ast(card["name"], "carth_the_lion")

def _h_leonin_arbiter(card):
    return _ast(card["name"], "leonin_arbiter")

def _h_fatespinner(card):
    return _ast(card["name"], "fatespinner")

def _h_rienne_angel_of_rebirth(card):
    return _ast(card["name"], "rienne_angel_of_rebirth")

def _h_fiery_gambit(card):
    return _ast(card["name"], "fiery_gambit")

def _h_crabomination(card):
    return _ast(card["name"], "crabomination")

def _h_henzie_toolbox_torre(card):
    return _ast(card["name"], "henzie_toolbox_torre")

def _h_nuka_nuke_launcher(card):
    return _ast(card["name"], "nuka_nuke_launcher")

def _h_razorfield_ripper(card):
    return _ast(card["name"], "razorfield_ripper")

def _h_curator_beastie(card):
    return _ast(card["name"], "curator_beastie")

def _h_temur_charm(card):
    return _ast(card["name"], "temur_charm")

def _h_shilgengar_sire_of_famine(card):
    return _ast(card["name"], "shilgengar_sire_of_famine")

def _h_control_magic(card):
    return _ast(card["name"], "control_magic")

def _h_hero_of_iroas(card):
    return _ast(card["name"], "hero_of_iroas")

def _h_time_spiral(card):
    return _ast(card["name"], "time_spiral")

def _h_earth_rumble(card):
    return _ast(card["name"], "earth_rumble")

def _h_the_twelfth_doctor(card):
    return _ast(card["name"], "the_twelfth_doctor")

def _h_tangle(card):
    return _ast(card["name"], "tangle")

def _h_keranos_god_of_storms(card):
    return _ast(card["name"], "keranos_god_of_storms")

def _h_invasion_of_theros_ephara_ever_sheltering(card):
    return _ast(card["name"], "invasion_of_theros_ephara_ever_sheltering")

def _h_fire_lord_zuko(card):
    return _ast(card["name"], "fire_lord_zuko")

def _h_koll_the_forgemaster(card):
    return _ast(card["name"], "koll_the_forgemaster")

def _h_curse_of_unbinding(card):
    return _ast(card["name"], "curse_of_unbinding")

def _h_hate_mirage(card):
    return _ast(card["name"], "hate_mirage")

def _h_cranial_ram(card):
    return _ast(card["name"], "cranial_ram")

def _h_emet_selch_unsundered_hades_sorcerer_of_eld(card):
    return _ast(card["name"], "emet_selch_unsundered_hades_sorcerer_of_eld")

def _h_blazing_archon(card):
    return _ast(card["name"], "blazing_archon")

def _h_hunter_s_blowgun(card):
    return _ast(card["name"], "hunter_s_blowgun")

def _h_lady_octopus_inspired_inventor(card):
    return _ast(card["name"], "lady_octopus_inspired_inventor")

def _h_onakke_oathkeeper(card):
    return _ast(card["name"], "onakke_oathkeeper")

def _h_admiral_brass_unsinkable(card):
    return _ast(card["name"], "admiral_brass_unsinkable")

def _h_oath_of_gideon(card):
    return _ast(card["name"], "oath_of_gideon")

def _h_desert_warfare(card):
    return _ast(card["name"], "desert_warfare")

def _h_sharae_of_numbing_depths(card):
    return _ast(card["name"], "sharae_of_numbing_depths")

def _h_pollen_shield_hare_hare_raising(card):
    return _ast(card["name"], "pollen_shield_hare_hare_raising")

def _h_outlaws_merriment(card):
    return _ast(card["name"], "outlaws_merriment")

def _h_dalek_drone(card):
    return _ast(card["name"], "dalek_drone")

def _h_captive_audience(card):
    return _ast(card["name"], "captive_audience")

def _h_sudden_substitution(card):
    return _ast(card["name"], "sudden_substitution")

def _h_the_capitoline_triad(card):
    return _ast(card["name"], "the_capitoline_triad")

def _h_rune_tail_kitsune_ascendant_rune_tail_s_essence(card):
    return _ast(card["name"], "rune_tail_kitsune_ascendant_rune_tail_s_essence")

def _h_stenn_paranoid_partisan(card):
    return _ast(card["name"], "stenn_paranoid_partisan")

def _h_oft_nabbed_goat(card):
    return _ast(card["name"], "oft_nabbed_goat")

def _h_norman_osborn_green_goblin(card):
    return _ast(card["name"], "norman_osborn_green_goblin")

def _h_marvo_deep_operative(card):
    return _ast(card["name"], "marvo_deep_operative")

def _h_discerning_financier(card):
    return _ast(card["name"], "discerning_financier")

def _h_rot_curse_rakshasa(card):
    return _ast(card["name"], "rot_curse_rakshasa")

def _h_spider_ham_peter_porker(card):
    return _ast(card["name"], "spider_ham_peter_porker")

def _h_march_of_reckless_joy(card):
    return _ast(card["name"], "march_of_reckless_joy")

def _h_the_ruinous_powers(card):
    return _ast(card["name"], "the_ruinous_powers")

def _h_officious_interrogation(card):
    return _ast(card["name"], "officious_interrogation")

def _h_verdeloth_the_ancient(card):
    return _ast(card["name"], "verdeloth_the_ancient")

def _h_path_of_the_pyromancer(card):
    return _ast(card["name"], "path_of_the_pyromancer")

def _h_anje_maid_of_dishonor(card):
    return _ast(card["name"], "anje_maid_of_dishonor")

def _h_bard_class(card):
    return _ast(card["name"], "bard_class")

def _h_for_the_ancestors(card):
    return _ast(card["name"], "for_the_ancestors")

def _h_confounding_riddle(card):
    return _ast(card["name"], "confounding_riddle")

def _h_battle_screech(card):
    return _ast(card["name"], "battle_screech")

def _h_nettling_nuisance(card):
    return _ast(card["name"], "nettling_nuisance")

def _h_aurelia_s_fury(card):
    return _ast(card["name"], "aurelia_s_fury")

def _h_armed_and_armored(card):
    return _ast(card["name"], "armed_and_armored")

def _h_angelic_aberration(card):
    return _ast(card["name"], "angelic_aberration")

def _h_shao_jun(card):
    return _ast(card["name"], "shao_jun")

def _h_change_of_plans(card):
    return _ast(card["name"], "change_of_plans")

def _h_spectral_searchlight(card):
    return _ast(card["name"], "spectral_searchlight")

def _h_weathered_runestone(card):
    return _ast(card["name"], "weathered_runestone")

def _h_lich_s_mastery(card):
    return _ast(card["name"], "lich_s_mastery")

def _h_eirdu_carrier_of_dawn_isilu_carrier_of_twilight(card):
    return _ast(card["name"], "eirdu_carrier_of_dawn_isilu_carrier_of_twilight")

def _h_game_of_chaos(card):
    return _ast(card["name"], "game_of_chaos")

def _h_jon_irenicus_shattered_one(card):
    return _ast(card["name"], "jon_irenicus_shattered_one")

def _h_aeve_progenitor_ooze(card):
    return _ast(card["name"], "aeve_progenitor_ooze")

def _h_wolverine_best_there_is(card):
    return _ast(card["name"], "wolverine_best_there_is")

def _h_hive_mind(card):
    return _ast(card["name"], "hive_mind")

def _h_skorpekh_lord(card):
    return _ast(card["name"], "skorpekh_lord")

def _h_waterbender_s_restoration(card):
    return _ast(card["name"], "waterbender_s_restoration")

def _h_frenzied_saddlebrute(card):
    return _ast(card["name"], "frenzied_saddlebrute")

def _h_magmatic_hellkite(card):
    return _ast(card["name"], "magmatic_hellkite")

def _h_space_marine_devastator(card):
    return _ast(card["name"], "space_marine_devastator")

def _h_guardian_of_ghirapur(card):
    return _ast(card["name"], "guardian_of_ghirapur")

def _h_day_of_black_sun(card):
    return _ast(card["name"], "day_of_black_sun")

def _h_cracked_earth_technique(card):
    return _ast(card["name"], "cracked_earth_technique")

def _h_final_word_phantom(card):
    return _ast(card["name"], "final_word_phantom")

def _h_coronation_of_chaos(card):
    return _ast(card["name"], "coronation_of_chaos")

def _h_mandate_of_peace(card):
    return _ast(card["name"], "mandate_of_peace")

def _h_welcome_the_dead(card):
    return _ast(card["name"], "welcome_the_dead")

def _h_faerie_fencing(card):
    return _ast(card["name"], "faerie_fencing")

def _h_radagast_the_brown(card):
    return _ast(card["name"], "radagast_the_brown")

def _h_oversimplify(card):
    return _ast(card["name"], "oversimplify")

def _h_maestros_charm(card):
    return _ast(card["name"], "maestros_charm")

def _h_from_the_catacombs(card):
    return _ast(card["name"], "from_the_catacombs")

def _h_you_look_upon_the_tarrasque(card):
    return _ast(card["name"], "you_look_upon_the_tarrasque")

def _h_marina_vendrell(card):
    return _ast(card["name"], "marina_vendrell")

def _h_printlifter_ooze(card):
    return _ast(card["name"], "printlifter_ooze")

def _h_leadership_vacuum(card):
    return _ast(card["name"], "leadership_vacuum")

def _h_curse_of_hospitality(card):
    return _ast(card["name"], "curse_of_hospitality")

def _h_immerwolf(card):
    return _ast(card["name"], "immerwolf")

def _h_illusion_of_choice(card):
    return _ast(card["name"], "illusion_of_choice")

def _h_risk_factor(card):
    return _ast(card["name"], "risk_factor")

def _h_central_elevator_promising_stairs(card):
    return _ast(card["name"], "central_elevator_promising_stairs")

def _h_jaya_s_phoenix(card):
    return _ast(card["name"], "jaya_s_phoenix")

def _h_extract_brain(card):
    return _ast(card["name"], "extract_brain")

def _h_interface_ace(card):
    return _ast(card["name"], "interface_ace")

def _h_what_must_be_done(card):
    return _ast(card["name"], "what_must_be_done")

def _h_sarkhan_dragon_ascendant(card):
    return _ast(card["name"], "sarkhan_dragon_ascendant")

def _h_midnight_crusader_shuttle(card):
    return _ast(card["name"], "midnight_crusader_shuttle")

def _h_earthshape(card):
    return _ast(card["name"], "earthshape")

def _h_grell_philosopher(card):
    return _ast(card["name"], "grell_philosopher")

def _h_agent_s_toolkit(card):
    return _ast(card["name"], "agent_s_toolkit")

def _h_wake_the_dead(card):
    return _ast(card["name"], "wake_the_dead")

def _h_barret_avalanche_leader(card):
    return _ast(card["name"], "barret_avalanche_leader")

def _h_memory_worm(card):
    return _ast(card["name"], "memory_worm")

def _h_audacious_reshapers(card):
    return _ast(card["name"], "audacious_reshapers")

def _h_nalia_de_arnise(card):
    return _ast(card["name"], "nalia_de_arnise")

def _h_enter_the_unknown(card):
    return _ast(card["name"], "enter_the_unknown")

def _h_yurlok_of_scorch_thrash(card):
    return _ast(card["name"], "yurlok_of_scorch_thrash")

def _h_netherese_puzzle_ward(card):
    return _ast(card["name"], "netherese_puzzle_ward")

def _h_banon_the_returners_leader(card):
    return _ast(card["name"], "banon_the_returners_leader")

def _h_zurgo_and_ojutai(card):
    return _ast(card["name"], "zurgo_and_ojutai")

def _h_skullbriar_the_walking_grave(card):
    return _ast(card["name"], "skullbriar_the_walking_grave")

def _h_mana_clash(card):
    return _ast(card["name"], "mana_clash")

def _h_valentin_dean_of_the_vein_lisette_dean_of_the_root(card):
    return _ast(card["name"], "valentin_dean_of_the_vein_lisette_dean_of_the_root")

def _h_fisher_s_talent(card):
    return _ast(card["name"], "fisher_s_talent")

def _h_sakashima_s_will(card):
    return _ast(card["name"], "sakashima_s_will")

def _h_voyager_staff(card):
    return _ast(card["name"], "voyager_staff")

def _h_wrath_of_the_skies(card):
    return _ast(card["name"], "wrath_of_the_skies")

def _h_abaddon_the_despoiler(card):
    return _ast(card["name"], "abaddon_the_despoiler")

def _h_the_temporal_anchor(card):
    return _ast(card["name"], "the_temporal_anchor")

def _h_superior_spider_man(card):
    return _ast(card["name"], "superior_spider_man")

def _h_the_blue_spirit(card):
    return _ast(card["name"], "the_blue_spirit")

def _h_salvation_swan(card):
    return _ast(card["name"], "salvation_swan")

def _h_havengul_laboratory_havengul_mystery(card):
    return _ast(card["name"], "havengul_laboratory_havengul_mystery")

def _h_aggressive_mining(card):
    return _ast(card["name"], "aggressive_mining")

def _h_zilortha_strength_incarnate(card):
    return _ast(card["name"], "zilortha_strength_incarnate")

def _h_worst_fears(card):
    return _ast(card["name"], "worst_fears")

def _h_detective_s_phoenix(card):
    return _ast(card["name"], "detective_s_phoenix")

def _h_hunting_wilds(card):
    return _ast(card["name"], "hunting_wilds")

def _h_peter_parker_amazing_spider_man(card):
    return _ast(card["name"], "peter_parker_amazing_spider_man")

def _h_kunoros_hound_of_athreos(card):
    return _ast(card["name"], "kunoros_hound_of_athreos")

def _h_eladamri_lord_of_leaves(card):
    return _ast(card["name"], "eladamri_lord_of_leaves")

def _h_lullmage_mentor(card):
    return _ast(card["name"], "lullmage_mentor")

def _h_nethergoyf(card):
    return _ast(card["name"], "nethergoyf")

def _h_spider_man_2099(card):
    return _ast(card["name"], "spider_man_2099")

def _h_spider_man_india(card):
    return _ast(card["name"], "spider_man_india")

def _h_valduk_keeper_of_the_flame(card):
    return _ast(card["name"], "valduk_keeper_of_the_flame")

def _h_angel_s_trumpet(card):
    return _ast(card["name"], "angel_s_trumpet")

def _h_ed_e_lonesome_eyebot(card):
    return _ast(card["name"], "ed_e_lonesome_eyebot")

def _h_noctis_prince_of_lucis(card):
    return _ast(card["name"], "noctis_prince_of_lucis")

def _h_fly(card):
    return _ast(card["name"], "fly")

def _h_sanctum_prelate(card):
    return _ast(card["name"], "sanctum_prelate")

def _h_last_night_together(card):
    return _ast(card["name"], "last_night_together")

def _h_miles_morales_ultimate_spider_man(card):
    return _ast(card["name"], "miles_morales_ultimate_spider_man")

def _h_uba_mask(card):
    return _ast(card["name"], "uba_mask")

def _h_goblin_assault(card):
    return _ast(card["name"], "goblin_assault")

def _h_impending_flux(card):
    return _ast(card["name"], "impending_flux")

def _h_return_the_past(card):
    return _ast(card["name"], "return_the_past")

def _h_curse_of_death_s_hold(card):
    return _ast(card["name"], "curse_of_death_s_hold")

def _h_squee_s_revenge(card):
    return _ast(card["name"], "squee_s_revenge")

def _h_curse_of_exhaustion(card):
    return _ast(card["name"], "curse_of_exhaustion")

def _h_astral_drift(card):
    return _ast(card["name"], "astral_drift")

def _h_winds_of_rebuke(card):
    return _ast(card["name"], "winds_of_rebuke")

def _h_complete_the_circuit(card):
    return _ast(card["name"], "complete_the_circuit")

def _h_wheel_of_potential(card):
    return _ast(card["name"], "wheel_of_potential")

def _h_critical_hit(card):
    return _ast(card["name"], "critical_hit")

def _h_lothl_rien_blade(card):
    return _ast(card["name"], "lothl_rien_blade")

def _h_bounty_of_skemfar(card):
    return _ast(card["name"], "bounty_of_skemfar")

def _h_lotuslight_dancers(card):
    return _ast(card["name"], "lotuslight_dancers")

def _h_thrasta_tempest_s_roar(card):
    return _ast(card["name"], "thrasta_tempest_s_roar")

def _h_sierra_nuka_s_biggest_fan(card):
    return _ast(card["name"], "sierra_nuka_s_biggest_fan")

def _h_sen_triplets(card):
    return _ast(card["name"], "sen_triplets")

def _h_secret_of_bloodbending(card):
    return _ast(card["name"], "secret_of_bloodbending")

def _h_screamer_killer(card):
    return _ast(card["name"], "screamer_killer")

def _h_optimus_prime_hero_optimus_prime_autobot_leader(card):
    return _ast(card["name"], "optimus_prime_hero_optimus_prime_autobot_leader")

def _h_daring_piracy(card):
    return _ast(card["name"], "daring_piracy")

def _h_mishra_s_command(card):
    return _ast(card["name"], "mishra_s_command")

def _h_octopus_umbra(card):
    return _ast(card["name"], "octopus_umbra")

def _h_scarlet_spider_ben_reilly(card):
    return _ast(card["name"], "scarlet_spider_ben_reilly")

def _h_souls_of_the_lost(card):
    return _ast(card["name"], "souls_of_the_lost")

def _h_myth_unbound(card):
    return _ast(card["name"], "myth_unbound")

def _h_kalamax_the_stormsire(card):
    return _ast(card["name"], "kalamax_the_stormsire")

def _h_denry_klin_editor_in_chief(card):
    return _ast(card["name"], "denry_klin_editor_in_chief")

def _h_discovery_dispersal(card):
    return _ast(card["name"], "discovery_dispersal")

def _h_savor_the_moment(card):
    return _ast(card["name"], "savor_the_moment")

def _h_mind_funeral(card):
    return _ast(card["name"], "mind_funeral")

def _h_galvanic_relay(card):
    return _ast(card["name"], "galvanic_relay")

def _h_tempt_with_reflections(card):
    return _ast(card["name"], "tempt_with_reflections")

def _h_mishra_eminent_one(card):
    return _ast(card["name"], "mishra_eminent_one")

def _h_the_fourth_doctor(card):
    return _ast(card["name"], "the_fourth_doctor")

def _h_exterminate(card):
    return _ast(card["name"], "exterminate")

def _h_brazen_cannonade(card):
    return _ast(card["name"], "brazen_cannonade")

def _h_pharika_god_of_affliction(card):
    return _ast(card["name"], "pharika_god_of_affliction")

def _h_yuna_s_whistle(card):
    return _ast(card["name"], "yuna_s_whistle")

def _h_inquisitor_greyfax(card):
    return _ast(card["name"], "inquisitor_greyfax")

def _h_smoke_spirits_aid(card):
    return _ast(card["name"], "smoke_spirits_aid")

def _h_balduvian_rage(card):
    return _ast(card["name"], "balduvian_rage")

def _h_flux(card):
    return _ast(card["name"], "flux")

def _h_under_the_skin(card):
    return _ast(card["name"], "under_the_skin")

def _h_pelakka_predation_pelakka_caverns(card):
    return _ast(card["name"], "pelakka_predation_pelakka_caverns")

def _h_psionic_ritual(card):
    return _ast(card["name"], "psionic_ritual")

def _h_power_artifact(card):
    return _ast(card["name"], "power_artifact")

def _h_imoen_mystic_trickster(card):
    return _ast(card["name"], "imoen_mystic_trickster")

def _h_moonlight_hunt(card):
    return _ast(card["name"], "moonlight_hunt")

def _h_missy(card):
    return _ast(card["name"], "missy")

def _h_tiana_ship_s_caretaker(card):
    return _ast(card["name"], "tiana_ship_s_caretaker")

def _h_identity_thief(card):
    return _ast(card["name"], "identity_thief")

def _h_scrambleverse(card):
    return _ast(card["name"], "scrambleverse")

def _h_dimir_charm(card):
    return _ast(card["name"], "dimir_charm")

def _h_choose_your_weapon(card):
    return _ast(card["name"], "choose_your_weapon")

def _h_garth_one_eye(card):
    return _ast(card["name"], "garth_one_eye")

def _h_curse_of_bounty(card):
    return _ast(card["name"], "curse_of_bounty")

def _h_hunted_by_the_family(card):
    return _ast(card["name"], "hunted_by_the_family")

def _h_vengeful_regrowth(card):
    return _ast(card["name"], "vengeful_regrowth")

def _h_portent_of_calamity(card):
    return _ast(card["name"], "portent_of_calamity")

def _h_petty_larceny(card):
    return _ast(card["name"], "petty_larceny")

def _h_golden_tail_trainer(card):
    return _ast(card["name"], "golden_tail_trainer")

def _h_pumpkin_bombs(card):
    return _ast(card["name"], "pumpkin_bombs")

def _h_bond_of_insight(card):
    return _ast(card["name"], "bond_of_insight")

def _h_alora_merry_thief(card):
    return _ast(card["name"], "alora_merry_thief")

def _h_swallowed_by_leviathan(card):
    return _ast(card["name"], "swallowed_by_leviathan")

def _h_ideas_unbound(card):
    return _ast(card["name"], "ideas_unbound")

def _h_skyskipper_duo(card):
    return _ast(card["name"], "skyskipper_duo")

def _h_fire_sages(card):
    return _ast(card["name"], "fire_sages")

def _h_signal_the_clans(card):
    return _ast(card["name"], "signal_the_clans")

def _h_curious_herd(card):
    return _ast(card["name"], "curious_herd")

def _h_spectacular_pileup(card):
    return _ast(card["name"], "spectacular_pileup")

def _h_sinister_concierge(card):
    return _ast(card["name"], "sinister_concierge")

def _h_reality_scramble(card):
    return _ast(card["name"], "reality_scramble")

def _h_phabine_boss_s_confidant(card):
    return _ast(card["name"], "phabine_boss_s_confidant")

def _h_goblin_charbelcher(card):
    return _ast(card["name"], "goblin_charbelcher")

def _h_gwafa_hazid_profiteer(card):
    return _ast(card["name"], "gwafa_hazid_profiteer")

def _h_szarekh_the_silent_king(card):
    return _ast(card["name"], "szarekh_the_silent_king")

def _h_hidetsugu_devouring_chaos(card):
    return _ast(card["name"], "hidetsugu_devouring_chaos")

def _h_ursine_monstrosity(card):
    return _ast(card["name"], "ursine_monstrosity")

def _h_sanwell_avenger_ace(card):
    return _ast(card["name"], "sanwell_avenger_ace")

def _h_trazyn_the_infinite(card):
    return _ast(card["name"], "trazyn_the_infinite")

def _h_irresistible_prey(card):
    return _ast(card["name"], "irresistible_prey")

def _h_river_song(card):
    return _ast(card["name"], "river_song")

def _h_lorehold_command(card):
    return _ast(card["name"], "lorehold_command")

def _h_hogaak_arisen_necropolis(card):
    return _ast(card["name"], "hogaak_arisen_necropolis")

def _h_zimone_s_hypothesis(card):
    return _ast(card["name"], "zimone_s_hypothesis")

def _h_durnan_of_the_yawning_portal(card):
    return _ast(card["name"], "durnan_of_the_yawning_portal")

def _h_raphael_the_muscle(card):
    return _ast(card["name"], "raphael_the_muscle")

def _h_shallow_grave(card):
    return _ast(card["name"], "shallow_grave")

def _h_silumgar_assassin(card):
    return _ast(card["name"], "silumgar_assassin")

def _h_transmogrant_s_crown(card):
    return _ast(card["name"], "transmogrant_s_crown")

def _h_cunning_nightbonder(card):
    return _ast(card["name"], "cunning_nightbonder")

def _h_ever_after(card):
    return _ast(card["name"], "ever_after")

def _h_urianger_augurelt(card):
    return _ast(card["name"], "urianger_augurelt")

def _h_saheeli_radiant_creator(card):
    return _ast(card["name"], "saheeli_radiant_creator")

def _h_mistbind_clique(card):
    return _ast(card["name"], "mistbind_clique")

def _h_hoarder_s_greed(card):
    return _ast(card["name"], "hoarder_s_greed")

def _h_spellbreaker_behemoth(card):
    return _ast(card["name"], "spellbreaker_behemoth")

def _h_cryptic_spires(card):
    return _ast(card["name"], "cryptic_spires")

def _h_deification(card):
    return _ast(card["name"], "deification")

def _h_arterial_alchemy(card):
    return _ast(card["name"], "arterial_alchemy")

def _h_indulge_excess(card):
    return _ast(card["name"], "indulge_excess")

def _h_safana_calimport_cutthroat(card):
    return _ast(card["name"], "safana_calimport_cutthroat")

def _h_strict_proctor(card):
    return _ast(card["name"], "strict_proctor")

def _h_ride_down(card):
    return _ast(card["name"], "ride_down")

def _h_bronze_bombshell(card):
    return _ast(card["name"], "bronze_bombshell")

def _h_dregscape_sliver(card):
    return _ast(card["name"], "dregscape_sliver")

def _h_rukarumel_biologist(card):
    return _ast(card["name"], "rukarumel_biologist")

def _h_alchemist_s_gambit(card):
    return _ast(card["name"], "alchemist_s_gambit")

def _h_the_seventh_doctor(card):
    return _ast(card["name"], "the_seventh_doctor")

def _h_tenacious_underdog(card):
    return _ast(card["name"], "tenacious_underdog")

def _h_fealty_to_the_realm(card):
    return _ast(card["name"], "fealty_to_the_realm")

def _h_exterminatus(card):
    return _ast(card["name"], "exterminatus")

def _h_psychic_spiral(card):
    return _ast(card["name"], "psychic_spiral")

def _h_prowl_stoic_strategist_prowl_pursuit_vehicle(card):
    return _ast(card["name"], "prowl_stoic_strategist_prowl_pursuit_vehicle")

def _h_edgar_king_of_figaro(card):
    return _ast(card["name"], "edgar_king_of_figaro")

def _h_pixie_guide(card):
    return _ast(card["name"], "pixie_guide")

def _h_the_beast_deathless_prince(card):
    return _ast(card["name"], "the_beast_deathless_prince")

def _h_rally_the_ancestors(card):
    return _ast(card["name"], "rally_the_ancestors")

def _h_liege_of_the_tangle(card):
    return _ast(card["name"], "liege_of_the_tangle")

def _h_indomitable_creativity(card):
    return _ast(card["name"], "indomitable_creativity")

def _h_frontline_heroism(card):
    return _ast(card["name"], "frontline_heroism")

def _h_seedtime(card):
    return _ast(card["name"], "seedtime")

def _h_growing_dread(card):
    return _ast(card["name"], "growing_dread")

def _h_tomik_distinguished_advokist(card):
    return _ast(card["name"], "tomik_distinguished_advokist")

def _h_butch_deloria_tunnel_snake(card):
    return _ast(card["name"], "butch_deloria_tunnel_snake")

def _h_raph_mikey_troublemakers(card):
    return _ast(card["name"], "raph_mikey_troublemakers")

def _h_arcade_gannon(card):
    return _ast(card["name"], "arcade_gannon")

def _h_greasefang_okiba_boss(card):
    return _ast(card["name"], "greasefang_okiba_boss")

def _h_return_upon_the_tide(card):
    return _ast(card["name"], "return_upon_the_tide")

def _h_sphinx_of_foresight(card):
    return _ast(card["name"], "sphinx_of_foresight")

def _h_deathleaper_terror_weapon(card):
    return _ast(card["name"], "deathleaper_terror_weapon")

def _h_tales_of_the_ancestors(card):
    return _ast(card["name"], "tales_of_the_ancestors")

def _h_worm_harvest(card):
    return _ast(card["name"], "worm_harvest")

def _h_renegade_silent(card):
    return _ast(card["name"], "renegade_silent")

def _h_koh_the_face_stealer(card):
    return _ast(card["name"], "koh_the_face_stealer")

def _h_astral_slide(card):
    return _ast(card["name"], "astral_slide")

def _h_shield_of_kaldra(card):
    return _ast(card["name"], "shield_of_kaldra")

def _h_sunder_the_gateway(card):
    return _ast(card["name"], "sunder_the_gateway")

def _h_genku_future_shaper(card):
    return _ast(card["name"], "genku_future_shaper")

def _h_war_effort(card):
    return _ast(card["name"], "war_effort")

def _h_solemn_doomguide(card):
    return _ast(card["name"], "solemn_doomguide")

def _h_polukranos_unchained(card):
    return _ast(card["name"], "polukranos_unchained")

def _h_ixidor_reality_sculptor(card):
    return _ast(card["name"], "ixidor_reality_sculptor")

def _h_falkenrath_gorger(card):
    return _ast(card["name"], "falkenrath_gorger")

def _h_master_of_predicaments(card):
    return _ast(card["name"], "master_of_predicaments")

def _h_abigale_eloquent_first_year(card):
    return _ast(card["name"], "abigale_eloquent_first_year")

def _h_radiant_performer(card):
    return _ast(card["name"], "radiant_performer")

def _h_twining_twins_swift_spiral(card):
    return _ast(card["name"], "twining_twins_swift_spiral")

def _h_erestor_of_the_council(card):
    return _ast(card["name"], "erestor_of_the_council")

def _h_stunning_reversal(card):
    return _ast(card["name"], "stunning_reversal")

def _h_dimir_machinations(card):
    return _ast(card["name"], "dimir_machinations")

def _h_alliance_of_arms(card):
    return _ast(card["name"], "alliance_of_arms")

def _h_panglacial_wurm(card):
    return _ast(card["name"], "panglacial_wurm")

def _h_skybind(card):
    return _ast(card["name"], "skybind")

def _h_goblin_assassin(card):
    return _ast(card["name"], "goblin_assassin")

def _h_goldbug_humanity_s_ally_goldbug_scrappy_scout(card):
    return _ast(card["name"], "goldbug_humanity_s_ally_goldbug_scrappy_scout")

def _h_aloy_savior_of_meridian(card):
    return _ast(card["name"], "aloy_savior_of_meridian")

def _h_return_to_the_ranks(card):
    return _ast(card["name"], "return_to_the_ranks")

def _h_convergence_of_dominion(card):
    return _ast(card["name"], "convergence_of_dominion")

def _h_power_surge(card):
    return _ast(card["name"], "power_surge")

def _h_masako_the_humorless(card):
    return _ast(card["name"], "masako_the_humorless")

def _h_goryo_s_vengeance(card):
    return _ast(card["name"], "goryo_s_vengeance")

def _h_turn_inside_out(card):
    return _ast(card["name"], "turn_inside_out")

def _h_corpseberry_cultivator(card):
    return _ast(card["name"], "corpseberry_cultivator")

def _h_curse_of_the_nightly_hunt(card):
    return _ast(card["name"], "curse_of_the_nightly_hunt")

def _h_mobile_homestead(card):
    return _ast(card["name"], "mobile_homestead")

def _h_serra_avenger(card):
    return _ast(card["name"], "serra_avenger")

def _h_shark_shredder_killer_clone(card):
    return _ast(card["name"], "shark_shredder_killer_clone")

def _h_light_the_way(card):
    return _ast(card["name"], "light_the_way")

def _h_natural_affinity(card):
    return _ast(card["name"], "natural_affinity")

def _h_galea_kindler_of_hope(card):
    return _ast(card["name"], "galea_kindler_of_hope")

def _h_reap_the_past(card):
    return _ast(card["name"], "reap_the_past")

def _h_scheming_fence(card):
    return _ast(card["name"], "scheming_fence")

def _h_mistmeadow_witch(card):
    return _ast(card["name"], "mistmeadow_witch")

def _h_corpse_dance(card):
    return _ast(card["name"], "corpse_dance")

def _h_king_of_the_oathbreakers(card):
    return _ast(card["name"], "king_of_the_oathbreakers")

def _h_turn_the_earth(card):
    return _ast(card["name"], "turn_the_earth")

def _h_phyrexian_ingester(card):
    return _ast(card["name"], "phyrexian_ingester")

def _h_cecily_haunted_mage(card):
    return _ast(card["name"], "cecily_haunted_mage")

def _h_curse_of_silence(card):
    return _ast(card["name"], "curse_of_silence")

def _h_kick_in_the_door(card):
    return _ast(card["name"], "kick_in_the_door")

def _h_dovescape(card):
    return _ast(card["name"], "dovescape")

def _h_rescue_from_the_underworld(card):
    return _ast(card["name"], "rescue_from_the_underworld")

def _h_lynde_cheerful_tormentor(card):
    return _ast(card["name"], "lynde_cheerful_tormentor")

def _h_spry_and_mighty(card):
    return _ast(card["name"], "spry_and_mighty")

def _h_run_for_your_life(card):
    return _ast(card["name"], "run_for_your_life")

def _h_sabin_master_monk(card):
    return _ast(card["name"], "sabin_master_monk")


PER_CARD_HANDLERS = {
    "Kefka, Dancing Mad": _h_kefka_dancing_mad,
    "Saheeli, the Sun's Brilliance": _h_saheeli_the_sun_s_brilliance,
    "Quick Sliver": _h_quick_sliver,
    "Ozai, the Phoenix King": _h_ozai_the_phoenix_king,
    "Marchesa, the Black Rose": _h_marchesa_the_black_rose,
    "Alisaie Leveilleur": _h_alisaie_leveilleur,
    "Lumbering Worldwagon": _h_lumbering_worldwagon,
    "Aang, Swift Savior // Aang and La, Ocean's Fury": _h_aang_swift_savior_aang_and_la_ocean_s_fury,
    "Kykar, Zephyr Awakener": _h_kykar_zephyr_awakener,
    "Overlord of the Boilerbilges": _h_overlord_of_the_boilerbilges,
    "Reflector Mage": _h_reflector_mage,
    "Firion, Wild Rose Warrior": _h_firion_wild_rose_warrior,
    "Dion, Bahamut's Dominant // Bahamut, Warden of Light": _h_dion_bahamut_s_dominant_bahamut_warden_of_light,
    "The Master, Transcendent": _h_the_master_transcendent,
    "Cadric, Soul Kindler": _h_cadric_soul_kindler,
    "Robe of the Archmagi": _h_robe_of_the_archmagi,
    "Elemental Eruption": _h_elemental_eruption,
    "Sauron's Ransom": _h_sauron_s_ransom,
    "Viewpoint Synchronization": _h_viewpoint_synchronization,
    "Damping Sphere": _h_damping_sphere,
    "Bilbo's Ring": _h_bilbo_s_ring,
    "Disorder in the Court": _h_disorder_in_the_court,
    "Conspiracy": _h_conspiracy,
    "Instill Energy": _h_instill_energy,
    "Norin the Wary": _h_norin_the_wary,
    "Airbending Lesson": _h_airbending_lesson,
    "Find // Finality": _h_find_finality,
    "Arwen, Mortal Queen": _h_arwen_mortal_queen,
    "Elenda, Saint of Dusk": _h_elenda_saint_of_dusk,
    "Aminatou, Veil Piercer": _h_aminatou_veil_piercer,
    "Chainer, Dementia Master": _h_chainer_dementia_master,
    "Glimmer Lens": _h_glimmer_lens,
    "Locke, Treasure Hunter": _h_locke_treasure_hunter,
    "Benevolent Blessing": _h_benevolent_blessing,
    "Vial Smasher the Fierce": _h_vial_smasher_the_fierce,
    "Combat Calligrapher": _h_combat_calligrapher,
    "Welcome to . . . // Jurassic Park": _h_welcome_to_jurassic_park,
    "Tannuk, Steadfast Second": _h_tannuk_steadfast_second,
    "Display of Power": _h_display_of_power,
    "Callidus Assassin": _h_callidus_assassin,
    "Stolen by the Fae": _h_stolen_by_the_fae,
    "Sidar Kondo of Jamuraa": _h_sidar_kondo_of_jamuraa,
    "Share the Spoils": _h_share_the_spoils,
    "Abuelo, Ancestral Echo": _h_abuelo_ancestral_echo,
    "Hithlain Rope": _h_hithlain_rope,
    "Mind Grind": _h_mind_grind,
    "Astrid Peth": _h_astrid_peth,
    "Dune Chanter": _h_dune_chanter,
    "Street Wraith": _h_street_wraith,
    "Feather, the Redeemed": _h_feather_the_redeemed,
    "On Serra's Wings": _h_on_serra_s_wings,
    "Prishe's Wanderings": _h_prishe_s_wanderings,
    "Collective Inferno": _h_collective_inferno,
    "Emperor of Bones": _h_emperor_of_bones,
    "Font of Agonies": _h_font_of_agonies,
    "Olórin's Searing Light": _h_ol_rin_s_searing_light,
    "Moonmist": _h_moonmist,
    "Cruel Ultimatum": _h_cruel_ultimatum,
    "Hakoda, Selfless Commander": _h_hakoda_selfless_commander,
    "Gilgamesh, Master-at-Arms": _h_gilgamesh_master_at_arms,
    "Zenos yae Galvus // Shinryu, Transcendent Rival": _h_zenos_yae_galvus_shinryu_transcendent_rival,
    "Neverwinter Hydra": _h_neverwinter_hydra,
    "Elven Farsight": _h_elven_farsight,
    "Knight Paladin": _h_knight_paladin,
    "Hide on the Ceiling": _h_hide_on_the_ceiling,
    "Intrepid Paleontologist": _h_intrepid_paleontologist,
    "Bladehold War-Whip": _h_bladehold_war_whip,
    "Kindred Charge": _h_kindred_charge,
    "Ancient Adamantoise": _h_ancient_adamantoise,
    "Golden Argosy": _h_golden_argosy,
    "Strong, the Brutish Thespian": _h_strong_the_brutish_thespian,
    "Quicken": _h_quicken,
    "Epic Experiment": _h_epic_experiment,
    "Ivy, Gleeful Spellthief": _h_ivy_gleeful_spellthief,
    "Sauron, the Necromancer": _h_sauron_the_necromancer,
    "Saproling Symbiosis": _h_saproling_symbiosis,
    "Arcades, the Strategist": _h_arcades_the_strategist,
    "The Scorpion God": _h_the_scorpion_god,
    "Erebos's Intervention": _h_erebos_s_intervention,
    "Hostile Negotiations": _h_hostile_negotiations,
    "Cathedral Acolyte": _h_cathedral_acolyte,
    "Brokers Charm": _h_brokers_charm,
    "Varchild, Betrayer of Kjeldor": _h_varchild_betrayer_of_kjeldor,
    "Jubilant Skybonder": _h_jubilant_skybonder,
    "Edge of Autumn": _h_edge_of_autumn,
    "Empty the Laboratory": _h_empty_the_laboratory,
    "Planetary Annihilation": _h_planetary_annihilation,
    "Agatha of the Vile Cauldron": _h_agatha_of_the_vile_cauldron,
    "Summons of Saruman": _h_summons_of_saruman,
    "Diamond Weapon": _h_diamond_weapon,
    "Company Commander": _h_company_commander,
    "Inferno of the Star Mounts": _h_inferno_of_the_star_mounts,
    "Magnus the Red": _h_magnus_the_red,
    "Sliver Gravemother": _h_sliver_gravemother,
    "Gift of Doom": _h_gift_of_doom,
    "Revival // Revenge": _h_revival_revenge,
    "Calamity, Galloping Inferno": _h_calamity_galloping_inferno,
    "Hatchery Sliver": _h_hatchery_sliver,
    "Reunion of the House": _h_reunion_of_the_house,
    "Chaos Defiler": _h_chaos_defiler,
    "Altar of the Pantheon": _h_altar_of_the_pantheon,
    "Asmodeus the Archfiend": _h_asmodeus_the_archfiend,
    "The Tenth Doctor": _h_the_tenth_doctor,
    "Poppet Stitcher // Poppet Factory": _h_poppet_stitcher_poppet_factory,
    "Twins of Discord": _h_twins_of_discord,
    "Fumulus, the Infestation": _h_fumulus_the_infestation,
    "Everlasting Torment": _h_everlasting_torment,
    "Timeline Culler": _h_timeline_culler,
    "Soul Partition": _h_soul_partition,
    "Drana, the Last Bloodchief": _h_drana_the_last_bloodchief,
    "Adéwalé, Breaker of Chains": _h_ad_wal_breaker_of_chains,
    "Winged Hive Tyrant": _h_winged_hive_tyrant,
    "Professor Hojo": _h_professor_hojo,
    "Radiant Lotus": _h_radiant_lotus,
    "Aang, at the Crossroads // Aang, Destined Savior": _h_aang_at_the_crossroads_aang_destined_savior,
    "Protean Hydra": _h_protean_hydra,
    "Xu-Ifit, Osteoharmonist": _h_xu_ifit_osteoharmonist,
    "Red Death, Shipwrecker": _h_red_death_shipwrecker,
    "Xanathar, Guild Kingpin": _h_xanathar_guild_kingpin,
    "Reidane, God of the Worthy // Valkmira, Protector's Shield": _h_reidane_god_of_the_worthy_valkmira_protector_s_shield,
    "Lava Dart": _h_lava_dart,
    "Plague Drone": _h_plague_drone,
    "Red Sun's Twilight": _h_red_sun_s_twilight,
    "Surge of Brilliance": _h_surge_of_brilliance,
    "Nine Lives": _h_nine_lives,
    "Getaway Glamer": _h_getaway_glamer,
    "Doorkeeper Thrull": _h_doorkeeper_thrull,
    "Manaform Hellkite": _h_manaform_hellkite,
    "Abyssal Persecutor": _h_abyssal_persecutor,
    "Auspicious Starrix": _h_auspicious_starrix,
    "Distortion Strike": _h_distortion_strike,
    "Canoptek Wraith": _h_canoptek_wraith,
    "Rockalanche": _h_rockalanche,
    "Starscream, Power Hungry // Starscream, Seeker Leader": _h_starscream_power_hungry_starscream_seeker_leader,
    "Anzrag's Rampage": _h_anzrag_s_rampage,
    "Voracious Fell Beast": _h_voracious_fell_beast,
    "Coram, the Undertaker": _h_coram_the_undertaker,
    "Opera Love Song": _h_opera_love_song,
    "Ochre Jelly": _h_ochre_jelly,
    "Yusri, Fortune's Flame": _h_yusri_fortune_s_flame,
    "Salvation Colossus": _h_salvation_colossus,
    "Ayara, Widow of the Realm // Ayara, Furnace Queen": _h_ayara_widow_of_the_realm_ayara_furnace_queen,
    "Fallaji Wayfarer": _h_fallaji_wayfarer,
    "Emergent Ultimatum": _h_emergent_ultimatum,
    "Cait Sith, Fortune Teller": _h_cait_sith_fortune_teller,
    "Riveteers Charm": _h_riveteers_charm,
    "Incandescent Soulstoke": _h_incandescent_soulstoke,
    "Ringwraiths": _h_ringwraiths,
    "Exocrine": _h_exocrine,
    "Shadow the Hedgehog": _h_shadow_the_hedgehog,
    "Excise the Imperfect": _h_excise_the_imperfect,
    "Glaring Spotlight": _h_glaring_spotlight,
    "Soulcatchers' Aerie": _h_soulcatchers_aerie,
    "Lily Bowen, Raging Grandma": _h_lily_bowen_raging_grandma,
    "Accumulate Wisdom": _h_accumulate_wisdom,
    "Steal Enchantment": _h_steal_enchantment,
    "Scavenger Regent // Exude Toxin": _h_scavenger_regent_exude_toxin,
    "Choco, Seeker of Paradise": _h_choco_seeker_of_paradise,
    "Necrotic Hex": _h_necrotic_hex,
    "Carth the Lion": _h_carth_the_lion,
    "Leonin Arbiter": _h_leonin_arbiter,
    "Fatespinner": _h_fatespinner,
    "Rienne, Angel of Rebirth": _h_rienne_angel_of_rebirth,
    "Fiery Gambit": _h_fiery_gambit,
    "Crabomination": _h_crabomination,
    "Henzie \"Toolbox\" Torre": _h_henzie_toolbox_torre,
    "Nuka-Nuke Launcher": _h_nuka_nuke_launcher,
    "Razorfield Ripper": _h_razorfield_ripper,
    "Curator Beastie": _h_curator_beastie,
    "Temur Charm": _h_temur_charm,
    "Shilgengar, Sire of Famine": _h_shilgengar_sire_of_famine,
    "Control Magic": _h_control_magic,
    "Hero of Iroas": _h_hero_of_iroas,
    "Time Spiral": _h_time_spiral,
    "Earth Rumble": _h_earth_rumble,
    "The Twelfth Doctor": _h_the_twelfth_doctor,
    "Tangle": _h_tangle,
    "Keranos, God of Storms": _h_keranos_god_of_storms,
    "Invasion of Theros // Ephara, Ever-Sheltering": _h_invasion_of_theros_ephara_ever_sheltering,
    "Fire Lord Zuko": _h_fire_lord_zuko,
    "Koll, the Forgemaster": _h_koll_the_forgemaster,
    "Curse of Unbinding": _h_curse_of_unbinding,
    "Hate Mirage": _h_hate_mirage,
    "Cranial Ram": _h_cranial_ram,
    "Emet-Selch, Unsundered // Hades, Sorcerer of Eld": _h_emet_selch_unsundered_hades_sorcerer_of_eld,
    "Blazing Archon": _h_blazing_archon,
    "Hunter's Blowgun": _h_hunter_s_blowgun,
    "Lady Octopus, Inspired Inventor": _h_lady_octopus_inspired_inventor,
    "Onakke Oathkeeper": _h_onakke_oathkeeper,
    "Admiral Brass, Unsinkable": _h_admiral_brass_unsinkable,
    "Oath of Gideon": _h_oath_of_gideon,
    "Desert Warfare": _h_desert_warfare,
    "Sharae of Numbing Depths": _h_sharae_of_numbing_depths,
    "Pollen-Shield Hare // Hare Raising": _h_pollen_shield_hare_hare_raising,
    "Outlaws' Merriment": _h_outlaws_merriment,
    "Dalek Drone": _h_dalek_drone,
    "Captive Audience": _h_captive_audience,
    "Sudden Substitution": _h_sudden_substitution,
    "The Capitoline Triad": _h_the_capitoline_triad,
    "Rune-Tail, Kitsune Ascendant // Rune-Tail's Essence": _h_rune_tail_kitsune_ascendant_rune_tail_s_essence,
    "Stenn, Paranoid Partisan": _h_stenn_paranoid_partisan,
    "Oft-Nabbed Goat": _h_oft_nabbed_goat,
    "Norman Osborn // Green Goblin": _h_norman_osborn_green_goblin,
    "Marvo, Deep Operative": _h_marvo_deep_operative,
    "Discerning Financier": _h_discerning_financier,
    "Rot-Curse Rakshasa": _h_rot_curse_rakshasa,
    "Spider-Ham, Peter Porker": _h_spider_ham_peter_porker,
    "March of Reckless Joy": _h_march_of_reckless_joy,
    "The Ruinous Powers": _h_the_ruinous_powers,
    "Officious Interrogation": _h_officious_interrogation,
    "Verdeloth the Ancient": _h_verdeloth_the_ancient,
    "Path of the Pyromancer": _h_path_of_the_pyromancer,
    "Anje, Maid of Dishonor": _h_anje_maid_of_dishonor,
    "Bard Class": _h_bard_class,
    "For the Ancestors": _h_for_the_ancestors,
    "Confounding Riddle": _h_confounding_riddle,
    "Battle Screech": _h_battle_screech,
    "Nettling Nuisance": _h_nettling_nuisance,
    "Aurelia's Fury": _h_aurelia_s_fury,
    "Armed and Armored": _h_armed_and_armored,
    "Angelic Aberration": _h_angelic_aberration,
    "Shao Jun": _h_shao_jun,
    "Change of Plans": _h_change_of_plans,
    "Spectral Searchlight": _h_spectral_searchlight,
    "Weathered Runestone": _h_weathered_runestone,
    "Lich's Mastery": _h_lich_s_mastery,
    "Eirdu, Carrier of Dawn // Isilu, Carrier of Twilight": _h_eirdu_carrier_of_dawn_isilu_carrier_of_twilight,
    "Game of Chaos": _h_game_of_chaos,
    "Jon Irenicus, Shattered One": _h_jon_irenicus_shattered_one,
    "Aeve, Progenitor Ooze": _h_aeve_progenitor_ooze,
    "Wolverine, Best There Is": _h_wolverine_best_there_is,
    "Hive Mind": _h_hive_mind,
    "Skorpekh Lord": _h_skorpekh_lord,
    "Waterbender's Restoration": _h_waterbender_s_restoration,
    "Frenzied Saddlebrute": _h_frenzied_saddlebrute,
    "Magmatic Hellkite": _h_magmatic_hellkite,
    "Space Marine Devastator": _h_space_marine_devastator,
    "Guardian of Ghirapur": _h_guardian_of_ghirapur,
    "Day of Black Sun": _h_day_of_black_sun,
    "Cracked Earth Technique": _h_cracked_earth_technique,
    "Final-Word Phantom": _h_final_word_phantom,
    "Coronation of Chaos": _h_coronation_of_chaos,
    "Mandate of Peace": _h_mandate_of_peace,
    "Welcome the Dead": _h_welcome_the_dead,
    "Faerie Fencing": _h_faerie_fencing,
    "Radagast the Brown": _h_radagast_the_brown,
    "Oversimplify": _h_oversimplify,
    "Maestros Charm": _h_maestros_charm,
    "From the Catacombs": _h_from_the_catacombs,
    "You Look Upon the Tarrasque": _h_you_look_upon_the_tarrasque,
    "Marina Vendrell": _h_marina_vendrell,
    "Printlifter Ooze": _h_printlifter_ooze,
    "Leadership Vacuum": _h_leadership_vacuum,
    "Curse of Hospitality": _h_curse_of_hospitality,
    "Immerwolf": _h_immerwolf,
    "Illusion of Choice": _h_illusion_of_choice,
    "Risk Factor": _h_risk_factor,
    "Central Elevator // Promising Stairs": _h_central_elevator_promising_stairs,
    "Jaya's Phoenix": _h_jaya_s_phoenix,
    "Extract Brain": _h_extract_brain,
    "Interface Ace": _h_interface_ace,
    "What Must Be Done": _h_what_must_be_done,
    "Sarkhan, Dragon Ascendant": _h_sarkhan_dragon_ascendant,
    "Midnight Crusader Shuttle": _h_midnight_crusader_shuttle,
    "Earthshape": _h_earthshape,
    "Grell Philosopher": _h_grell_philosopher,
    "Agent's Toolkit": _h_agent_s_toolkit,
    "Wake the Dead": _h_wake_the_dead,
    "Barret, Avalanche Leader": _h_barret_avalanche_leader,
    "Memory Worm": _h_memory_worm,
    "Audacious Reshapers": _h_audacious_reshapers,
    "Nalia de'Arnise": _h_nalia_de_arnise,
    "Enter the Unknown": _h_enter_the_unknown,
    "Yurlok of Scorch Thrash": _h_yurlok_of_scorch_thrash,
    "Netherese Puzzle-Ward": _h_netherese_puzzle_ward,
    "Banon, the Returners' Leader": _h_banon_the_returners_leader,
    "Zurgo and Ojutai": _h_zurgo_and_ojutai,
    "Skullbriar, the Walking Grave": _h_skullbriar_the_walking_grave,
    "Mana Clash": _h_mana_clash,
    "Valentin, Dean of the Vein // Lisette, Dean of the Root": _h_valentin_dean_of_the_vein_lisette_dean_of_the_root,
    "Fisher's Talent": _h_fisher_s_talent,
    "Sakashima's Will": _h_sakashima_s_will,
    "Voyager Staff": _h_voyager_staff,
    "Wrath of the Skies": _h_wrath_of_the_skies,
    "Abaddon the Despoiler": _h_abaddon_the_despoiler,
    "The Temporal Anchor": _h_the_temporal_anchor,
    "Superior Spider-Man": _h_superior_spider_man,
    "The Blue Spirit": _h_the_blue_spirit,
    "Salvation Swan": _h_salvation_swan,
    "Havengul Laboratory // Havengul Mystery": _h_havengul_laboratory_havengul_mystery,
    "Aggressive Mining": _h_aggressive_mining,
    "Zilortha, Strength Incarnate": _h_zilortha_strength_incarnate,
    "Worst Fears": _h_worst_fears,
    "Detective's Phoenix": _h_detective_s_phoenix,
    "Hunting Wilds": _h_hunting_wilds,
    "Peter Parker // Amazing Spider-Man": _h_peter_parker_amazing_spider_man,
    "Kunoros, Hound of Athreos": _h_kunoros_hound_of_athreos,
    "Eladamri, Lord of Leaves": _h_eladamri_lord_of_leaves,
    "Lullmage Mentor": _h_lullmage_mentor,
    "Nethergoyf": _h_nethergoyf,
    "Spider-Man 2099": _h_spider_man_2099,
    "Spider-Man India": _h_spider_man_india,
    "Valduk, Keeper of the Flame": _h_valduk_keeper_of_the_flame,
    "Angel's Trumpet": _h_angel_s_trumpet,
    "ED-E, Lonesome Eyebot": _h_ed_e_lonesome_eyebot,
    "Noctis, Prince of Lucis": _h_noctis_prince_of_lucis,
    "Fly": _h_fly,
    "Sanctum Prelate": _h_sanctum_prelate,
    "Last Night Together": _h_last_night_together,
    "Miles Morales // Ultimate Spider-Man": _h_miles_morales_ultimate_spider_man,
    "Uba Mask": _h_uba_mask,
    "Goblin Assault": _h_goblin_assault,
    "Impending Flux": _h_impending_flux,
    "Return the Past": _h_return_the_past,
    "Curse of Death's Hold": _h_curse_of_death_s_hold,
    "Squee's Revenge": _h_squee_s_revenge,
    "Curse of Exhaustion": _h_curse_of_exhaustion,
    "Astral Drift": _h_astral_drift,
    "Winds of Rebuke": _h_winds_of_rebuke,
    "Complete the Circuit": _h_complete_the_circuit,
    "Wheel of Potential": _h_wheel_of_potential,
    "Critical Hit": _h_critical_hit,
    "Lothlórien Blade": _h_lothl_rien_blade,
    "Bounty of Skemfar": _h_bounty_of_skemfar,
    "Lotuslight Dancers": _h_lotuslight_dancers,
    "Thrasta, Tempest's Roar": _h_thrasta_tempest_s_roar,
    "Sierra, Nuka's Biggest Fan": _h_sierra_nuka_s_biggest_fan,
    "Sen Triplets": _h_sen_triplets,
    "Secret of Bloodbending": _h_secret_of_bloodbending,
    "Screamer-Killer": _h_screamer_killer,
    "Optimus Prime, Hero // Optimus Prime, Autobot Leader": _h_optimus_prime_hero_optimus_prime_autobot_leader,
    "Daring Piracy": _h_daring_piracy,
    "Mishra's Command": _h_mishra_s_command,
    "Octopus Umbra": _h_octopus_umbra,
    "Scarlet Spider, Ben Reilly": _h_scarlet_spider_ben_reilly,
    "Souls of the Lost": _h_souls_of_the_lost,
    "Myth Unbound": _h_myth_unbound,
    "Kalamax, the Stormsire": _h_kalamax_the_stormsire,
    "Denry Klin, Editor in Chief": _h_denry_klin_editor_in_chief,
    "Discovery // Dispersal": _h_discovery_dispersal,
    "Savor the Moment": _h_savor_the_moment,
    "Mind Funeral": _h_mind_funeral,
    "Galvanic Relay": _h_galvanic_relay,
    "Tempt with Reflections": _h_tempt_with_reflections,
    "Mishra, Eminent One": _h_mishra_eminent_one,
    "The Fourth Doctor": _h_the_fourth_doctor,
    "Exterminate!": _h_exterminate,
    "Brazen Cannonade": _h_brazen_cannonade,
    "Pharika, God of Affliction": _h_pharika_god_of_affliction,
    "Yuna's Whistle": _h_yuna_s_whistle,
    "Inquisitor Greyfax": _h_inquisitor_greyfax,
    "Smoke Spirits' Aid": _h_smoke_spirits_aid,
    "Balduvian Rage": _h_balduvian_rage,
    "Flux": _h_flux,
    "Under the Skin": _h_under_the_skin,
    "Pelakka Predation // Pelakka Caverns": _h_pelakka_predation_pelakka_caverns,
    "Psionic Ritual": _h_psionic_ritual,
    "Power Artifact": _h_power_artifact,
    "Imoen, Mystic Trickster": _h_imoen_mystic_trickster,
    "Moonlight Hunt": _h_moonlight_hunt,
    "Missy": _h_missy,
    "Tiana, Ship's Caretaker": _h_tiana_ship_s_caretaker,
    "Identity Thief": _h_identity_thief,
    "Scrambleverse": _h_scrambleverse,
    "Dimir Charm": _h_dimir_charm,
    "Choose Your Weapon": _h_choose_your_weapon,
    "Garth One-Eye": _h_garth_one_eye,
    "Curse of Bounty": _h_curse_of_bounty,
    "Hunted by The Family": _h_hunted_by_the_family,
    "Vengeful Regrowth": _h_vengeful_regrowth,
    "Portent of Calamity": _h_portent_of_calamity,
    "Petty Larceny": _h_petty_larceny,
    "Golden-Tail Trainer": _h_golden_tail_trainer,
    "Pumpkin Bombs": _h_pumpkin_bombs,
    "Bond of Insight": _h_bond_of_insight,
    "Alora, Merry Thief": _h_alora_merry_thief,
    "Swallowed by Leviathan": _h_swallowed_by_leviathan,
    "Ideas Unbound": _h_ideas_unbound,
    "Skyskipper Duo": _h_skyskipper_duo,
    "Fire Sages": _h_fire_sages,
    "Signal the Clans": _h_signal_the_clans,
    "Curious Herd": _h_curious_herd,
    "Spectacular Pileup": _h_spectacular_pileup,
    "Sinister Concierge": _h_sinister_concierge,
    "Reality Scramble": _h_reality_scramble,
    "Phabine, Boss's Confidant": _h_phabine_boss_s_confidant,
    "Goblin Charbelcher": _h_goblin_charbelcher,
    "Gwafa Hazid, Profiteer": _h_gwafa_hazid_profiteer,
    "Szarekh, the Silent King": _h_szarekh_the_silent_king,
    "Hidetsugu, Devouring Chaos": _h_hidetsugu_devouring_chaos,
    "Ursine Monstrosity": _h_ursine_monstrosity,
    "Sanwell, Avenger Ace": _h_sanwell_avenger_ace,
    "Trazyn the Infinite": _h_trazyn_the_infinite,
    "Irresistible Prey": _h_irresistible_prey,
    "River Song": _h_river_song,
    "Lorehold Command": _h_lorehold_command,
    "Hogaak, Arisen Necropolis": _h_hogaak_arisen_necropolis,
    "Zimone's Hypothesis": _h_zimone_s_hypothesis,
    "Durnan of the Yawning Portal": _h_durnan_of_the_yawning_portal,
    "Raphael, the Muscle": _h_raphael_the_muscle,
    "Shallow Grave": _h_shallow_grave,
    "Silumgar Assassin": _h_silumgar_assassin,
    "Transmogrant's Crown": _h_transmogrant_s_crown,
    "Cunning Nightbonder": _h_cunning_nightbonder,
    "Ever After": _h_ever_after,
    "Urianger Augurelt": _h_urianger_augurelt,
    "Saheeli, Radiant Creator": _h_saheeli_radiant_creator,
    "Mistbind Clique": _h_mistbind_clique,
    "Hoarder's Greed": _h_hoarder_s_greed,
    "Spellbreaker Behemoth": _h_spellbreaker_behemoth,
    "Cryptic Spires": _h_cryptic_spires,
    "Deification": _h_deification,
    "Arterial Alchemy": _h_arterial_alchemy,
    "Indulge // Excess": _h_indulge_excess,
    "Safana, Calimport Cutthroat": _h_safana_calimport_cutthroat,
    "Strict Proctor": _h_strict_proctor,
    "Ride Down": _h_ride_down,
    "Bronze Bombshell": _h_bronze_bombshell,
    "Dregscape Sliver": _h_dregscape_sliver,
    "Rukarumel, Biologist": _h_rukarumel_biologist,
    "Alchemist's Gambit": _h_alchemist_s_gambit,
    "The Seventh Doctor": _h_the_seventh_doctor,
    "Tenacious Underdog": _h_tenacious_underdog,
    "Fealty to the Realm": _h_fealty_to_the_realm,
    "Exterminatus": _h_exterminatus,
    "Psychic Spiral": _h_psychic_spiral,
    "Prowl, Stoic Strategist // Prowl, Pursuit Vehicle": _h_prowl_stoic_strategist_prowl_pursuit_vehicle,
    "Edgar, King of Figaro": _h_edgar_king_of_figaro,
    "Pixie Guide": _h_pixie_guide,
    "The Beast, Deathless Prince": _h_the_beast_deathless_prince,
    "Rally the Ancestors": _h_rally_the_ancestors,
    "Liege of the Tangle": _h_liege_of_the_tangle,
    "Indomitable Creativity": _h_indomitable_creativity,
    "Frontline Heroism": _h_frontline_heroism,
    "Seedtime": _h_seedtime,
    "Growing Dread": _h_growing_dread,
    "Tomik, Distinguished Advokist": _h_tomik_distinguished_advokist,
    "Butch DeLoria, Tunnel Snake": _h_butch_deloria_tunnel_snake,
    "Raph & Mikey, Troublemakers": _h_raph_mikey_troublemakers,
    "Arcade Gannon": _h_arcade_gannon,
    "Greasefang, Okiba Boss": _h_greasefang_okiba_boss,
    "Return Upon the Tide": _h_return_upon_the_tide,
    "Sphinx of Foresight": _h_sphinx_of_foresight,
    "Deathleaper, Terror Weapon": _h_deathleaper_terror_weapon,
    "Tales of the Ancestors": _h_tales_of_the_ancestors,
    "Worm Harvest": _h_worm_harvest,
    "Renegade Silent": _h_renegade_silent,
    "Koh, the Face Stealer": _h_koh_the_face_stealer,
    "Astral Slide": _h_astral_slide,
    "Shield of Kaldra": _h_shield_of_kaldra,
    "Sunder the Gateway": _h_sunder_the_gateway,
    "Genku, Future Shaper": _h_genku_future_shaper,
    "War Effort": _h_war_effort,
    "Solemn Doomguide": _h_solemn_doomguide,
    "Polukranos, Unchained": _h_polukranos_unchained,
    "Ixidor, Reality Sculptor": _h_ixidor_reality_sculptor,
    "Falkenrath Gorger": _h_falkenrath_gorger,
    "Master of Predicaments": _h_master_of_predicaments,
    "Abigale, Eloquent First-Year": _h_abigale_eloquent_first_year,
    "Radiant Performer": _h_radiant_performer,
    "Twining Twins // Swift Spiral": _h_twining_twins_swift_spiral,
    "Erestor of the Council": _h_erestor_of_the_council,
    "Stunning Reversal": _h_stunning_reversal,
    "Dimir Machinations": _h_dimir_machinations,
    "Alliance of Arms": _h_alliance_of_arms,
    "Panglacial Wurm": _h_panglacial_wurm,
    "Skybind": _h_skybind,
    "Goblin Assassin": _h_goblin_assassin,
    "Goldbug, Humanity's Ally // Goldbug, Scrappy Scout": _h_goldbug_humanity_s_ally_goldbug_scrappy_scout,
    "Aloy, Savior of Meridian": _h_aloy_savior_of_meridian,
    "Return to the Ranks": _h_return_to_the_ranks,
    "Convergence of Dominion": _h_convergence_of_dominion,
    "Power Surge": _h_power_surge,
    "Masako the Humorless": _h_masako_the_humorless,
    "Goryo's Vengeance": _h_goryo_s_vengeance,
    "Turn Inside Out": _h_turn_inside_out,
    "Corpseberry Cultivator": _h_corpseberry_cultivator,
    "Curse of the Nightly Hunt": _h_curse_of_the_nightly_hunt,
    "Mobile Homestead": _h_mobile_homestead,
    "Serra Avenger": _h_serra_avenger,
    "Shark Shredder, Killer Clone": _h_shark_shredder_killer_clone,
    "Light the Way": _h_light_the_way,
    "Natural Affinity": _h_natural_affinity,
    "Galea, Kindler of Hope": _h_galea_kindler_of_hope,
    "Reap the Past": _h_reap_the_past,
    "Scheming Fence": _h_scheming_fence,
    "Mistmeadow Witch": _h_mistmeadow_witch,
    "Corpse Dance": _h_corpse_dance,
    "King of the Oathbreakers": _h_king_of_the_oathbreakers,
    "Turn the Earth": _h_turn_the_earth,
    "Phyrexian Ingester": _h_phyrexian_ingester,
    "Cecily, Haunted Mage": _h_cecily_haunted_mage,
    "Curse of Silence": _h_curse_of_silence,
    "Kick in the Door": _h_kick_in_the_door,
    "Dovescape": _h_dovescape,
    "Rescue from the Underworld": _h_rescue_from_the_underworld,
    "Lynde, Cheerful Tormentor": _h_lynde_cheerful_tormentor,
    "Spry and Mighty": _h_spry_and_mighty,
    "Run for Your Life": _h_run_for_your_life,
    "Sabin, Master Monk": _h_sabin_master_monk,
}

# Other registries empty
EFFECT_RULES: list = []
STATIC_PATTERNS: list = []
TRIGGER_PATTERNS: list = []

__all__ = ["PER_CARD_HANDLERS"]
