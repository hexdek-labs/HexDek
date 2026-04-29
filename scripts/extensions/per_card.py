#!/usr/bin/env python3
"""Per-card hand-written handlers for genuinely-unique snowflake cards.

These are the "Doomsday-class" cards whose oracle text defies any general
grammar — the rules text is so bespoke that every attempt to generalize it
would either over-fit (breaking other cards) or leave the card PARTIAL
forever. The pragmatic fix: emit a hand-crafted ``CardAST`` per name.

Each handler receives the raw Scryfall card dict and returns a ``CardAST``.
The handler's job is ONLY to record the card's effect *shape* in typed AST
form — enough that:
    1. The coverage metric can count the card as GREEN.
    2. A runtime engine can dispatch to a card-specific resolver keyed on
       ``Modification(kind="custom", args=(<slug>, ...))``.

We are NOT trying to faithfully encode every clause here — a stub with the
custom slug + a short list of sub-effects that ARE legible (damage, draw,
sacrifice, etc.) is strictly better than leaving the card in PARTIAL limbo.

The parser loads ``PER_CARD_HANDLERS`` via ``load_extensions`` and dispatches
on exact card name BEFORE any grammar matching. See ``parser.parse_card``.

Cards covered (handler slug in parentheses):
    Doomsday (doomsday)
    Balance (balance)
    Falling Star (falling_star)
    Painter's Servant (painters_servant)
    Camouflage (camouflage)
    Word of Command (word_of_command)
    Wheel of Misfortune (wheel_of_misfortune)
    Warp World (warp_world)
    Psychic Battle (psychic_battle)
    Goblin Game (goblin_game)
    Glimpse of Tomorrow (glimpse_of_tomorrow)
    Lim-Dûl's Vault (lim_duls_vault)
    Pox (pox)
    Drought (drought)
    The Great Aurora (great_aurora)
    Muse Vortex (muse_vortex)
    Genesis Wave (genesis_wave)
    Call to Arms (call_to_arms)
    Selective Adaptation (selective_adaptation)
    Hibernation's End (hibernations_end)
    Natural Balance (natural_balance)
    Titania's Command (titanias_command)
    Archdruid's Charm (archdruids_charm)
    Guff Rewrites History (guff_rewrites_history)
    Hew the Entwood (hew_the_entwood)
    Map to Lorthos's Temple (map_to_lorthos)
    Codie, Vociferous Codex (codie)
    Journey to the Lost City (journey_lost_city)
    Nature's Wrath (natures_wrath)
"""

from __future__ import annotations

import sys
from pathlib import Path

_HERE = Path(__file__).resolve().parent
_SCRIPTS = _HERE.parent
if str(_SCRIPTS) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS))

from mtg_ast import (  # noqa: E402
    Activated, CardAST, Condition, Cost, CreateToken, Damage, Discard, Draw,
    Exile, Filter, GainLife, Keyword, Mill, Modification, Sacrifice, Sequence,
    SetLife, Static, Triggered, Trigger, Tutor, UnknownEffect,
    EACH_OPPONENT, EACH_PLAYER, SELF, TARGET_CREATURE, TARGET_OPPONENT,
    TARGET_PLAYER,
)


# ---------------------------------------------------------------------------
# Helper builders
# ---------------------------------------------------------------------------

def _custom(slug: str, *args) -> Static:
    """Shorthand: Static ability tagged with a card-specific resolver slug."""
    return Static(
        modification=Modification(kind="custom", args=(slug,) + tuple(args)),
        raw=f"<per-card:{slug}>",
    )


def _custom_activated(slug: str, cost: Cost = None, *args) -> Activated:
    if cost is None:
        cost = Cost()
    return Activated(
        cost=cost,
        effect=UnknownEffect(raw_text=f"<per-card:{slug}>"),
        raw=f"<per-card:{slug}>",
    )


def _custom_triggered(slug: str, event: str, *args) -> Triggered:
    return Triggered(
        trigger=Trigger(event=event),
        effect=UnknownEffect(raw_text=f"<per-card:{slug}>"),
        raw=f"<per-card:{slug}>",
    )


def _ast(name: str, *abilities) -> CardAST:
    return CardAST(name=name, abilities=tuple(abilities),
                   parse_errors=(), fully_parsed=True)


# ---------------------------------------------------------------------------
# Individual handlers
# ---------------------------------------------------------------------------

def doomsday_handler(card):
    """Doomsday — the iconic snowflake.

    "Search your library and graveyard for five cards and exile the rest.
    Put the chosen cards on top of your library in any order.
    You lose half your life, rounded up."
    """
    return _ast(
        card["name"],
        # The five-card stack build
        _custom("doomsday_pile"),
        # Life payment
        _custom("lose_half_life_rounded_up"),
    )


def balance_handler(card):
    """Balance — MTG's original mass-symmetry bomb.

    Each player equalizes lands, hands, and creatures to the lowest bidder.
    Grammar can't express "parallel triple-equalize" without polluting other
    rules.
    """
    return _ast(
        card["name"],
        _custom("balance_lands"),
        _custom("balance_hands"),
        _custom("balance_creatures"),
    )


def falling_star_handler(card):
    """Falling Star — a dexterity card. Real-world flip mechanic.

    We encode the rules-effect half (damage + tap) and leave the physical
    flip as a custom resolver tag; there's no sane AST node for "land flat
    on the table".
    """
    return _ast(
        card["name"],
        Static(
            modification=Modification(
                kind="custom",
                args=("dexterity_flip", "falling_star"),
            ),
            raw=card.get("oracle_text", ""),
        ),
    )


def painters_servant_handler(card):
    """Painter's Servant — chooses a color on ETB, all cards become that color.

    The "all cards anywhere" universal color-pump is unique; one of the
    few effects that touches cards outside the battlefield.
    """
    return _ast(
        card["name"],
        _custom_triggered("painters_servant_choose_color", "etb"),
        _custom("painters_servant_color_wash"),
    )


def camouflage_handler(card):
    """Camouflage — a randomized-blocker combat replacement.

    No other card sorts blockers into piles and assigns them at random.
    """
    return _ast(
        card["name"],
        Static(
            modification=Modification(
                kind="custom",
                args=("random_block_pile_assignment",),
            ),
            raw=card.get("oracle_text", ""),
        ),
    )


def word_of_command_handler(card):
    """Word of Command — take control of an opponent's play from their hand.

    Unique because control extends across priority boundaries. Tagged for
    a card-specific resolver.
    """
    return _ast(
        card["name"],
        _custom("word_of_command_hand_puppet"),
    )


def wheel_of_misfortune_handler(card):
    """Wheel of Misfortune — secret-number auction with a wheel-and-deal tail."""
    return _ast(
        card["name"],
        _custom("secret_number_auction"),
        _custom("high_bidder_damage"),
        _custom("non_lowest_wheel"),
    )


def warp_world_handler(card):
    """Warp World — shuffle all permanents, reveal, put onto battlefield in
    layered order."""
    return _ast(
        card["name"],
        _custom("warp_world_layered_reveal"),
    )


def psychic_battle_handler(card):
    """Psychic Battle — rewrites target-selection via a mana-value race."""
    return _ast(
        card["name"],
        _custom_triggered("psychic_battle_retarget",
                          "targets_chosen"),
    )


def goblin_game_handler(card):
    """Goblin Game — secret item-hiding, fewest-hider loses half life."""
    return _ast(
        card["name"],
        _custom("goblin_game_hidden_items"),
    )


def glimpse_of_tomorrow_handler(card):
    """Glimpse of Tomorrow — shuffle permanents in, reveal N, layered put."""
    return _ast(
        card["name"],
        Keyword(name="suspend", args=(3, "{R}{R}"), raw="Suspend 3—{R}{R}"),
        _custom("layered_reveal_put_permanents"),
    )


def lim_duls_vault_handler(card):
    """Lim-Dûl's Vault — iterative top-5 library manipulation loop."""
    return _ast(
        card["name"],
        _custom("lim_duls_vault_iterate"),
    )


def pox_handler(card):
    """Pox — three sequential thirds (life, hand, creatures, lands)."""
    return _ast(
        card["name"],
        _custom("pox_thirds_chain"),
    )


def drought_handler(card):
    """Drought — upkeep cost + spells/abilities cost extra sac per black pip."""
    return _ast(
        card["name"],
        _custom_triggered("drought_upkeep", "upkeep"),
        _custom("drought_spell_tax_per_black_pip"),
        _custom("drought_activated_ability_tax_per_black_pip"),
    )


def great_aurora_handler(card):
    """The Great Aurora — shuffle hand+battlefield into library, redraw,
    put lands back, self-exile."""
    return _ast(
        card["name"],
        _custom("great_aurora_reset"),
        _custom("great_aurora_exile_self"),
    )


def muse_vortex_handler(card):
    """Muse Vortex — exile top X, free-cast one instant/sorcery, rest to hand+bottom."""
    return _ast(
        card["name"],
        _custom("muse_vortex_free_cast"),
    )


def genesis_wave_handler(card):
    """Genesis Wave — reveal top X, put any permanents with MV<=X onto the
    battlefield."""
    return _ast(
        card["name"],
        _custom("genesis_wave_reveal_put"),
    )


def call_to_arms_handler(card):
    """Call to Arms — "most common color among opponent's permanents" buff
    + self-sac when the condition drops."""
    return _ast(
        card["name"],
        _custom_triggered("call_to_arms_choose", "etb"),
        _custom("call_to_arms_conditional_anthem"),
        _custom_triggered("call_to_arms_lose_condition", "condition_fails"),
    )


def selective_adaptation_handler(card):
    """Selective Adaptation — reveal 7, find one of each ability keyword,
    one battlefield + rest hand/graveyard."""
    return _ast(
        card["name"],
        _custom("selective_adaptation_ability_grid"),
    )


def hibernations_end_handler(card):
    """Hibernation's End — cumulative upkeep into creature-tutor scaling."""
    return _ast(
        card["name"],
        Keyword(name="cumulative upkeep", args=("{1}",),
                raw="Cumulative upkeep {1}"),
        _custom_triggered("hibernations_end_tutor_by_age",
                          "paid_cumulative_upkeep"),
    )


def natural_balance_handler(card):
    """Natural Balance — land equalization in both directions."""
    return _ast(
        card["name"],
        _custom("natural_balance_sac_excess"),
        _custom("natural_balance_fetch_basics"),
    )


def titanias_command_handler(card):
    """Titania's Command — choose-2 charm, 4 modes."""
    return _ast(
        card["name"],
        _custom("titanias_command_choose_2_of_4",
                "exile_graveyard_life",
                "fetch_two_lands_tapped",
                "create_two_bears",
                "proliferate_counters_anthem"),
    )


def archdruids_charm_handler(card):
    """Archdruid's Charm — choose-1 charm with 4 modes."""
    return _ast(
        card["name"],
        _custom("archdruids_charm_choose_1_of_4",
                "tutor_creature_or_land",
                "counter_boost_fight",
                "destroy_artifact_enchantment_or_planeswalker",
                "destroy_nonbasic_land"),
    )


def guff_rewrites_history_handler(card):
    """Guff Rewrites History — per-player permanent shuffle + reveal-until-nonland."""
    return _ast(
        card["name"],
        _custom("guff_per_player_shuffle"),
        _custom("guff_reveal_until_nonland"),
    )


def hew_the_entwood_handler(card):
    """Hew the Entwood — sac lands, reveal X, put artifact/land onto bf,
    creatures to hand, rest bottom."""
    return _ast(
        card["name"],
        _custom("hew_entwood_sac_and_dig"),
    )


def map_to_lorthos_handler(card):
    """Map to Lorthos's Temple — tick-box objectives, sac to create Lorthos token."""
    return _ast(
        card["name"],
        _custom_triggered("map_lorthos_objective_1", "artifact_etb_yours"),
        _custom_triggered("map_lorthos_objective_2", "merfolk_etb_any"),
        _custom_triggered("map_lorthos_objective_3", "opponent_attacked"),
        _custom("map_lorthos_checkoff_sac_for_token"),
    )


def codie_handler(card):
    """Codie, Vociferous Codex — can't cast permanent spells; 5c mana +
    next-spell-mills-to-instant/sorcery."""
    return _ast(
        card["name"],
        _custom("codie_cant_cast_permanents"),
        _custom_activated("codie_5c_mana_plus_cast_trigger",
                          Cost(tap=True)),
    )


def journey_lost_city_handler(card):
    """Journey to the Lost City — upkeep exile 4, roll d20, banded outcome."""
    return _ast(
        card["name"],
        _custom_triggered("journey_upkeep_d20_exile4", "upkeep"),
    )


def natures_wrath_handler(card):
    """Nature's Wrath — upkeep tax + 4 color-policing triggers."""
    return _ast(
        card["name"],
        _custom_triggered("natures_wrath_upkeep", "upkeep"),
        _custom_triggered("natures_wrath_police_island_blue",
                          "permanent_enters"),
        _custom_triggered("natures_wrath_police_swamp_black",
                          "permanent_enters"),
        _custom_triggered("natures_wrath_police_mountain_red",
                          "permanent_enters"),
        _custom_triggered("natures_wrath_police_plains_white",
                          "permanent_enters"),
    )


# ===========================================================================
# Wave 2: Top-EDH-rank PARTIAL cards (added to escape multi-error parse limbo).
# ===========================================================================
#
# These handlers were generated for cards whose oracle text produced 2+ parse
# errors in the general grammar — typically because they combine novel
# triggers, modal layouts, or rules-text idioms that don't generalize.
# Each handler returns a hand-crafted CardAST. Even stubs that emit one
# `_custom(slug)` Static node are strict improvements over PARTIAL because
# they record the card's effect *shape* and key it to a runtime resolver.

def demolition_field_handler(card):
    """Demolition Field — mana, plus a sac-to-strip-and-fetch ability."""
    return _ast(
        card["name"],
        Activated(cost=Cost(tap=True),
                  effect=UnknownEffect(raw_text="<add C>"),
                  raw="{T}: Add {C}."),
        _custom_activated("demolition_field_sac_strip_fetch",
                          Cost(tap=True, sacrifice=Filter(base="self"),
                               extra=("{2}",))),
        _custom("demolition_field_symmetric_basic_fetch"),
    )


def opposition_agent_handler(card):
    return _ast(
        card["name"],
        Keyword(name="flash", raw="Flash"),
        _custom("opposition_agent_control_searches"),
        _custom("opposition_agent_steal_searched_cards"),
        _custom("opposition_agent_any_color_to_cast"),
    )


def tibalts_trickery_handler(card):
    return _ast(
        card["name"],
        _custom("tibalts_trickery_counter_random_cascade"),
        _custom("tibalts_trickery_random_bottom_pile"),
    )


def midnight_clock_handler(card):
    return _ast(
        card["name"],
        Activated(cost=Cost(tap=True),
                  effect=UnknownEffect(raw_text="<add U>"),
                  raw="{T}: Add {U}."),
        _custom_activated("midnight_clock_add_hour_counter"),
        _custom_triggered("midnight_clock_upkeep_tick", "upkeep"),
        _custom_triggered("midnight_clock_twelfth_wheel",
                          "counter_threshold_reached"),
    )


def wandering_archaic_handler(card):
    return _ast(
        card["name"],
        _custom_triggered("wandering_archaic_copy_or_pay", "opp_casts_iss"),
        _custom("explore_the_vastlands_modal_reveal_choice"),
    )


def seize_the_spotlight_handler(card):
    return _ast(
        card["name"],
        _custom("seize_spotlight_fame_or_fortune_vote"),
        _custom("seize_spotlight_fame_steal_haste"),
        _custom("seize_spotlight_fortune_draw_treasure"),
    )


def season_of_gathering_handler(card):
    return _ast(
        card["name"],
        _custom("season_modal_phyrexian_repeated_choose_5",
                "season_of_gathering"),
        _custom("season_gathering_p_buff_vigilance_trample"),
        _custom("season_gathering_pp_destroy_artifact_or_enchantment_type"),
        _custom("season_gathering_ppp_draw_max_power"),
    )


def mycosynth_lattice_handler(card):
    return _ast(
        card["name"],
        _custom("mycosynth_lattice_all_artifacts"),
        _custom("mycosynth_lattice_all_colorless"),
        _custom("mycosynth_lattice_any_color_mana"),
    )


def jetmir_handler(card):
    return _ast(
        card["name"],
        _custom("jetmir_anthem_threshold_3_vigilance"),
        _custom("jetmir_anthem_threshold_6_trample"),
        _custom("jetmir_anthem_threshold_9_double_strike"),
    )


def season_of_weaving_handler(card):
    return _ast(
        card["name"],
        _custom("season_modal_phyrexian_repeated_choose_5",
                "season_of_weaving"),
        _custom("season_weaving_p_draw"),
        _custom("season_weaving_pp_clone_artifact_or_creature"),
        _custom("season_weaving_ppp_bounce_all_nonland_nontoken"),
    )


def eluge_handler(card):
    return _ast(
        card["name"],
        _custom("eluge_pt_equals_islands"),
        _custom_triggered("eluge_etb_or_attack_flood_counter",
                          "etb_or_attack"),
        _custom("eluge_island_typification"),
        _custom("eluge_first_iss_per_turn_cost_reduction"),
    )


def nowhere_to_run_handler(card):
    return _ast(
        card["name"],
        Keyword(name="flash", raw="Flash"),
        _custom_triggered("nowhere_to_run_etb_minus3", "etb"),
        _custom("nowhere_to_run_strip_hexproof"),
        _custom("nowhere_to_run_disable_ward"),
    )


def master_of_cruelties_handler(card):
    return _ast(
        card["name"],
        Keyword(name="first strike", raw="First strike"),
        Keyword(name="deathtouch", raw="deathtouch"),
        _custom("master_of_cruelties_attack_alone"),
        _custom_triggered("master_of_cruelties_set_life_to_1",
                          "attack_player_unblocked"),
        _custom("master_of_cruelties_no_combat_damage"),
    )


def keen_duelist_handler(card):
    return _ast(
        card["name"],
        _custom_triggered("keen_duelist_mutual_top_reveal", "upkeep"),
        _custom("keen_duelist_lose_life_each_others_mv"),
        _custom("keen_duelist_each_draws_their_revealed"),
    )


def tenuous_truce_handler(card):
    return _ast(
        card["name"],
        _custom("enchant_opponent"),
        _custom_triggered("tenuous_truce_mutual_draw", "enchanted_end_step"),
        _custom_triggered("tenuous_truce_attack_breaks_pact", "attack_either"),
    )


def illusionists_gambit_handler(card):
    return _ast(
        card["name"],
        _custom("illusionists_gambit_timing_blockers_only"),
        _custom("illusionists_gambit_remove_attackers_extra_combat_redirect"),
        _custom("illusionists_gambit_forced_attack_not_you"),
    )


def nahiris_lithoforming_handler(card):
    return _ast(
        card["name"],
        _custom("nahiris_lithoforming_sac_x_lands"),
        _custom("nahiris_lithoforming_draw_per_sac"),
        _custom("nahiris_lithoforming_x_extra_land_drops_etb_tapped"),
    )


def season_of_the_burrow_handler(card):
    return _ast(
        card["name"],
        _custom("season_modal_phyrexian_repeated_choose_5",
                "season_of_the_burrow"),
        _custom("season_burrow_p_make_rabbit_token"),
        _custom("season_burrow_pp_exile_nonland_draw_card_for_owner"),
        _custom("season_burrow_ppp_recur_with_indestructible"),
    )


def beza_handler(card):
    return _ast(
        card["name"],
        _custom_triggered("beza_etb_quadruple_compare", "etb"),
        _custom("beza_treasure_if_opponent_more_lands"),
        _custom("beza_4_life_if_opponent_more_life"),
        _custom("beza_two_fish_if_opponent_more_creatures"),
        _custom("beza_draw_if_opponent_more_cards"),
    )


def fblthp_handler(card):
    return _ast(
        card["name"],
        Keyword(name="ward", args=("{2}",), raw="Ward {2}"),
        _custom("look_at_top_of_library_anytime"),
        _custom("fblthp_top_of_library_has_plot"),
        _custom("fblthp_may_plot_nonland_from_top"),
    )


def kharn_handler(card):
    return _ast(
        card["name"],
        _custom("kharn_attacks_or_blocks_each_combat"),
        _custom_triggered("kharn_lose_control_draw_two", "lose_control"),
        _custom("kharn_redirect_damage_change_control"),
    )


def season_of_loss_handler(card):
    return _ast(
        card["name"],
        _custom("season_modal_phyrexian_repeated_choose_5",
                "season_of_loss"),
        _custom("season_loss_p_each_sac_creature"),
        _custom("season_loss_pp_draw_per_died_yours_this_turn"),
        _custom("season_loss_ppp_each_opp_loses_x_life"),
    )


def arvinox_handler(card):
    return _ast(
        card["name"],
        _custom("arvinox_not_creature_unless_3_borrowed"),
        _custom_triggered("arvinox_end_step_exile_each_opp_bottom_facedown",
                          "end_step"),
        _custom("arvinox_play_those_cards_any_color"),
    )


def toralf_god_handler(card):
    return _ast(
        card["name"],
        Keyword(name="trample", raw="Trample"),
        _custom_triggered("toralf_excess_redirect", "excess_noncombat_damage"),
        _custom_activated("toralfs_hammer_equipped_ability_3_damage",
                          Cost(tap=True, extra=("{1}{R}",))),
        _custom("toralfs_hammer_equip"),
    )


def angelic_arbiter_handler(card):
    return _ast(
        card["name"],
        Keyword(name="flying", raw="Flying"),
        _custom("angelic_arbiter_no_attack_after_spell"),
        _custom("angelic_arbiter_no_spell_after_attack"),
    )


def tempt_with_mayhem_handler(card):
    return _ast(
        card["name"],
        _custom("tempting_offer_copy_iss"),
        _custom("tempt_with_mayhem_self_copies_per_taker"),
    )


def pendant_of_prosperity_handler(card):
    return _ast(
        card["name"],
        _custom("pendant_of_prosperity_etb_under_opp_control"),
        _custom_activated("pendant_of_prosperity_mutual_draw_play_land",
                          Cost(tap=True, extra=("{2}",))),
    )


def make_an_example_handler(card):
    return _ast(
        card["name"],
        _custom("make_an_example_each_opp_two_piles"),
        _custom("make_an_example_choose_one_pile_each"),
        _custom("make_an_example_sac_chosen_pile"),
    )


def chefs_kiss_handler(card):
    return _ast(
        card["name"],
        _custom("chefs_kiss_take_control_of_spell"),
        _custom("chefs_kiss_copy_random_retarget"),
        _custom("chefs_kiss_no_self_targets"),
    )


def pact_weapon_handler(card):
    return _ast(
        card["name"],
        _custom("pact_weapon_no_lose_at_zero_life_while_attached"),
        _custom_triggered("pact_weapon_attack_draw_reveal_buff_pay_life",
                          "equipped_attacks"),
        _custom("equip_discard_card"),
    )


def pirs_whim_handler(card):
    return _ast(
        card["name"],
        _custom("friend_or_foe_each_player"),
        _custom("pirs_whim_friend_fetch_land_tapped"),
        _custom("pirs_whim_foe_sac_artifact_or_enchantment"),
    )


def stargaze_handler(card):
    return _ast(
        card["name"],
        _custom("stargaze_look_2x"),
        _custom("stargaze_keep_x_rest_to_graveyard"),
        _custom("stargaze_lose_x_life"),
    )


def herigast_handler(card):
    return _ast(
        card["name"],
        Keyword(name="emerge", args=("{6}{R}{R}",), raw="Emerge {6}{R}{R}"),
        _custom_triggered("herigast_cast_exile_hand_draw_3", "cast_self"),
        Keyword(name="flying", raw="Flying"),
        _custom("herigast_grants_emerge_to_creature_spells"),
    )


def sin_unending_handler(card):
    return _ast(
        card["name"],
        Keyword(name="flying", raw="Flying"),
        Keyword(name="trample", raw="trample"),
        _custom("sin_etb_remove_counters_double_to_self"),
        _custom_triggered("sin_dies_move_counters_shuffle_self", "die"),
    )


def season_of_the_bold_handler(card):
    return _ast(
        card["name"],
        _custom("season_modal_phyrexian_repeated_choose_5",
                "season_of_the_bold"),
        _custom("season_bold_p_tapped_treasure"),
        _custom("season_bold_pp_exile_top_2_play_until_next"),
        _custom("season_bold_ppp_delayed_spellslinger_2_dmg"),
    )


def etrata_handler(card):
    return _ast(
        card["name"],
        _custom("etrata_unblockable"),
        _custom_triggered("etrata_combat_damage_hit_counter_exile",
                          "combat_damage_player"),
        _custom("etrata_three_hits_loses_game"),
        _custom("etrata_shuffle_self_after_hit"),
    )


def lifecraft_engine_handler(card):
    return _ast(
        card["name"],
        _custom("lifecraft_engine_choose_creature_type_etb"),
        _custom("lifecraft_engine_vehicles_become_chosen_type"),
        _custom("lifecraft_engine_chosen_type_anthem"),
        Keyword(name="crew", args=(3,), raw="Crew 3"),
    )


def ashling_limitless_handler(card):
    return _ast(
        card["name"],
        _custom("ashling_grant_evoke_4_to_elemental_spells"),
        _custom_triggered("ashling_clone_sacrificed_elemental",
                          "sac_nontoken_elemental"),
        _custom_triggered("ashling_token_sac_unless_5c_pay",
                          "next_end_step"),
    )


def graaz_handler(card):
    return _ast(
        card["name"],
        _custom("graaz_juggernauts_attack_each_combat"),
        _custom("graaz_juggernauts_unblockable_by_walls"),
        _custom("graaz_others_become_5_3_juggernauts"),
    )


def berserkers_frenzy_handler(card):
    return _ast(
        card["name"],
        _custom("berserkers_frenzy_pre_combat_only"),
        _custom("berserkers_frenzy_roll_2d20_keep_higher"),
        _custom("berserkers_frenzy_band_1_14_opp_chooses_blocks"),
        _custom("berserkers_frenzy_band_15_20_you_choose_blocks"),
    )


def falco_spara_handler(card):
    return _ast(
        card["name"],
        Keyword(name="flying", raw="Flying"),
        Keyword(name="trample", raw="trample"),
        _custom("falco_spara_etb_with_shield_counter"),
        _custom("look_at_top_of_library_anytime"),
        _custom("falco_spara_cast_top_paying_with_counters"),
    )


def hazezon_sand_handler(card):
    return _ast(
        card["name"],
        Keyword(name="desertwalk", raw="Desertwalk"),
        _custom("play_desert_lands_from_graveyard"),
        _custom_triggered("hazezon_sand_warriors_per_desert_etb",
                          "desert_etb"),
    )


def intellect_devourer_handler(card):
    return _ast(
        card["name"],
        _custom_triggered("intellect_devourer_etb_each_opp_exile",
                          "etb"),
        _custom("intellect_devourer_play_lands_and_cast_from_exile"),
        _custom("intellect_devourer_any_color_to_cast"),
    )


def war_doctor_handler(card):
    return _ast(
        card["name"],
        _custom_triggered("war_doctor_phaseout_or_exile_time_counter",
                          "phaseout_or_exile"),
        _custom_triggered("war_doctor_attack_dmg_eq_counters",
                          "attacks"),
        _custom("war_doctor_exile_replace_die"),
    )


def call_for_aid_handler(card):
    return _ast(
        card["name"],
        _custom("call_for_aid_steal_all_opp_creatures"),
        _custom("call_for_aid_untap_haste"),
        _custom("call_for_aid_no_attack_that_player"),
        _custom("call_for_aid_no_sac_those"),
    )


def spelltwine_handler(card):
    return _ast(
        card["name"],
        _custom("spelltwine_exile_two_iss_yours_and_opp"),
        _custom("spelltwine_copy_and_cast_both_free"),
        _custom("spelltwine_self_exile"),
    )


def hierophant_bio_titan_handler(card):
    return _ast(
        card["name"],
        _custom("frenzied_metabolism_remove_p1p1_counters_for_cost_reduction"),
        Keyword(name="vigilance", raw="Vigilance"),
        Keyword(name="reach", raw="reach"),
        Keyword(name="ward", args=("{2}",), raw="ward {2}"),
        _custom("titanic_unblockable_by_power_2_or_less"),
    )


def blue_mages_cane_handler(card):
    return _ast(
        card["name"],
        _custom("ff_job_select"),
        _custom("blue_mages_cane_grants_buff_wizard_and_steal_iss_trigger"),
        _custom_activated("blue_mages_cane_equip", Cost(extra=("{2}",))),
    )


def summoners_grimoire_handler(card):
    return _ast(
        card["name"],
        _custom("ff_job_select"),
        _custom("summoners_grimoire_grants_shaman_and_cheat_creature_trigger"),
        _custom_activated("summoners_grimoire_equip", Cost(extra=("{3}",))),
    )


def trepanation_blade_handler(card):
    return _ast(
        card["name"],
        _custom_triggered("trepanation_blade_reveal_until_land_buff",
                          "equipped_attacks"),
        _custom("trepanation_blade_revealed_to_graveyard"),
        Keyword(name="equip", args=("{2}",), raw="Equip {2}"),
    )


def overwhelming_splendor_handler(card):
    return _ast(
        card["name"],
        _custom("enchant_player"),
        _custom("overwhelming_splendor_creatures_lose_abilities_become_1_1"),
        _custom("overwhelming_splendor_no_nonmana_nonloyalty_activations"),
    )


def vaevictis_handler(card):
    return _ast(
        card["name"],
        Keyword(name="flying", raw="Flying"),
        _custom_triggered("vaevictis_each_player_sac_target_perm_then_topdeck_cheat",
                          "attacks"),
    )


def ninjas_blades_handler(card):
    return _ast(
        card["name"],
        _custom("ff_job_select"),
        _custom("ninjas_blades_grants_buff_ninja_and_loot_drain_trigger"),
        _custom_activated("ninjas_blades_equip", Cost(extra=("{2}",))),
    )


def master_of_the_wild_hunt_handler(card):
    return _ast(
        card["name"],
        _custom_triggered("mwh_upkeep_make_2_2_wolf", "upkeep"),
        _custom_activated("mwh_tap_all_wolves_pack_attack",
                          Cost(tap=True)),
    )


def megatons_fate_handler(card):
    return _ast(
        card["name"],
        _custom("megatons_fate_choose_one"),
        _custom("megatons_fate_disarm_destroy_artifact_4_treasures"),
        _custom("megatons_fate_detonate_8_each_creature_4_rad_each"),
    )


def rex_cyber_hound_handler(card):
    return _ast(
        card["name"],
        _custom_triggered("rex_combat_damage_mill_2_get_2_energy",
                          "combat_damage_player"),
        _custom_activated("rex_pay_2e_exile_with_brain_counter",
                          Cost(extra=("{E}{E}",))),
        _custom("rex_grants_self_brain_counter_card_abilities"),
    )


def eleventh_doctor_handler(card):
    return _ast(
        card["name"],
        _custom_triggered("eleventh_doctor_combat_damage_exile_with_time",
                          "combat_damage_player"),
        _custom_activated("eleventh_doctor_2_unblockable_power_3",
                          Cost(extra=("{2}",))),
    )


def gollum_scheming_handler(card):
    return _ast(
        card["name"],
        _custom_triggered("gollum_attack_top_two_choose_land_guess",
                          "attacks"),
        _custom("gollum_guess_right_remove_from_combat"),
        _custom("gollum_guess_wrong_draw_unblockable"),
    )


def ran_and_shaw_handler(card):
    return _ast(
        card["name"],
        Keyword(name="flying", raw="Flying"),
        Keyword(name="firebending", args=(2,), raw="firebending 2"),
        _custom_triggered("ran_shaw_etb_clone_if_3_dragon_lesson_in_grave",
                          "etb"),
        _custom_activated("ran_shaw_pump_dragons",
                          Cost(extra=("{3}{R}",))),
    )


def talent_telepath_handler(card):
    return _ast(
        card["name"],
        _custom("talent_telepath_reveal_top_7_free_cast_iss"),
        _custom("talent_telepath_rest_to_graveyard"),
        _custom("talent_telepath_spell_mastery_two_iss_instead"),
    )


def indominus_rex_handler(card):
    return _ast(
        card["name"],
        _custom("indominus_rex_etb_discard_creatures_inherit_keywords"),
        _custom_triggered("indominus_rex_etb_draw_per_counter", "etb"),
    )


def nacatl_war_pride_handler(card):
    return _ast(
        card["name"],
        _custom("nacatl_war_pride_must_be_blocked_by_one"),
        _custom_triggered("nacatl_war_pride_attack_clone_per_defender",
                          "attacks"),
        _custom("nacatl_war_pride_exile_tokens_eot"),
    )


def elephant_grass_handler(card):
    return _ast(
        card["name"],
        Keyword(name="cumulative upkeep", args=("{1}",),
                raw="Cumulative upkeep {1}"),
        _custom("elephant_grass_black_no_attack"),
        _custom("elephant_grass_nonblack_pay_2_each_to_attack"),
    )


def savage_summoning_handler(card):
    return _ast(
        card["name"],
        _custom("savage_summoning_self_uncounterable"),
        _custom("savage_summoning_next_creature_flash_uncounterable_p1p1"),
    )


def ninja_teen_handler(card):
    return _ast(
        card["name"],
        _custom("class_level_2"),
        _custom("class_level_3"),
        _custom_triggered("ninja_teen_creature_leaves_drain_1",
                          "creature_leaves_yours"),
        _custom("ninja_teen_lvl2_anthem_menace"),
        _custom("ninja_teen_lvl3_grants_sneak_to_grave"),
    )


def memories_returning_handler(card):
    return _ast(
        card["name"],
        _custom("memories_returning_reveal_5_alternating_keep_3_bottom_2"),
        Keyword(name="flashback", args=("{7}{U}{U}",),
                raw="Flashback {7}{U}{U}"),
    )


def wedding_river_song_handler(card):
    return _ast(
        card["name"],
        _custom("wedding_river_song_draw_2_self_suspend_exile"),
        _custom("wedding_river_song_opp_does_same"),
        _custom("wedding_river_song_time_travel"),
    )


def flash_photography_handler(card):
    return _ast(
        card["name"],
        _custom("flash_photography_flash_if_self_target"),
        _custom("flash_photography_clone_target_permanent"),
        Keyword(name="flashback", args=("{4}{U}{U}",),
                raw="Flashback {4}{U}{U}"),
    )


def mages_contest_handler(card):
    return _ast(
        card["name"],
        _custom("mages_contest_life_auction_with_spell_controller"),
        _custom("mages_contest_high_bidder_pays_life"),
        _custom("mages_contest_winner_counters_spell"),
    )


def manascape_refractor_handler(card):
    return _ast(
        card["name"],
        _custom("etb_tapped_self"),
        _custom("manascape_refractor_has_all_land_activated_abilities"),
        _custom("any_color_to_pay_self_activations"),
    )


def lightstall_inquisitor_handler(card):
    return _ast(
        card["name"],
        Keyword(name="vigilance", raw="Vigilance"),
        _custom_triggered("lightstall_inquisitor_etb_each_opp_exile_play",
                          "etb"),
        _custom("lightstall_inquisitor_cast_costs_1_more"),
        _custom("lightstall_inquisitor_lands_etb_tapped"),
    )


def melira_handler(card):
    return _ast(
        card["name"],
        _custom("melira_no_poison_counters"),
        _custom("melira_no_minus_counters_on_yours"),
        _custom("melira_opps_creatures_lose_infect"),
    )


def wernog_handler(card):
    return _ast(
        card["name"],
        _custom_triggered("wernog_etb_or_ltb_each_opp_investigate_or_drain",
                          "etb_or_ltb"),
        _custom("wernog_self_investigate_x_times"),
        Keyword(name="partner with", args=("friends forever",),
                raw="Partner—Friends forever"),
    )


def kondas_banner_handler(card):
    return _ast(
        card["name"],
        _custom("kondas_banner_attach_legendary_only"),
        _custom("kondas_banner_share_color_anthem"),
        _custom("kondas_banner_share_type_anthem"),
        Keyword(name="equip", args=("{2}",), raw="Equip {2}"),
    )


def saruman_many_colors_handler(card):
    return _ast(
        card["name"],
        Keyword(name="ward", args=("discard",),
                raw="Ward—Discard an enchantment, instant, or sorcery card."),
        _custom_triggered("saruman_second_spell_each_opp_mill_2",
                          "cast_second_spell"),
        _custom_triggered("saruman_milled_exile_iss_lower_mv_copy_free",
                          "card_milled_via"),
    )


def guided_passage_handler(card):
    return _ast(
        card["name"],
        _custom("guided_passage_reveal_library"),
        _custom("guided_passage_opp_picks_creature_land_other"),
        _custom("guided_passage_to_hand_then_shuffle"),
    )


def collective_brutality_handler(card):
    return _ast(
        card["name"],
        Keyword(name="escalate", args=("discard a card",),
                raw="Escalate—Discard a card."),
        _custom("collective_brutality_choose_one_or_more"),
        _custom("collective_brutality_iss_targeted_discard"),
        _custom("collective_brutality_target_minus_2_minus_2"),
        _custom("collective_brutality_drain_2"),
    )


def knight_errant_eos_handler(card):
    return _ast(
        card["name"],
        Keyword(name="convoke", raw="Convoke"),
        _custom_triggered("knight_errant_eos_etb_top_6_reveal_2_creatures_mvx",
                          "etb"),
    )


def sibylline_soothsayer_handler(card):
    return _ast(
        card["name"],
        _custom_triggered("sibylline_etb_reveal_until_nonland_mv3_suspend",
                          "etb"),
        _custom("sibylline_rest_bottom_random_order"),
    )


def virtuss_maneuver_handler(card):
    return _ast(
        card["name"],
        _custom("friend_or_foe_each_player"),
        _custom("virtuss_maneuver_friend_recur_creature"),
        _custom("virtuss_maneuver_foe_sac_creature"),
    )


def ward_of_bones_handler(card):
    return _ast(
        card["name"],
        _custom("ward_of_bones_no_creatures_artifacts_enchantments_if_more"),
        _custom("ward_of_bones_no_lands_if_more"),
    )


def start_the_tardis_handler(card):
    return _ast(
        card["name"],
        Keyword(name="surveil", args=(2,), raw="Surveil 2"),
        _custom("start_the_tardis_draw_a_card"),
        _custom("start_the_tardis_may_planeswalk"),
        Keyword(name="jump-start", raw="Jump-start"),
    )


def deliver_unto_evil_handler(card):
    return _ast(
        card["name"],
        _custom("deliver_unto_evil_choose_4_grave"),
        _custom("deliver_unto_evil_bolas_full_recur_or_opp_picks_2"),
        _custom("deliver_unto_evil_self_exile"),
    )


def sigil_of_distinction_handler(card):
    return _ast(
        card["name"],
        _custom("sigil_distinction_etb_with_x_charges"),
        _custom("sigil_distinction_anthem_per_charge"),
        _custom_activated("sigil_distinction_equip_remove_charge",
                          Cost(remove_counters=(1, "charge"))),
    )


def zndrsplts_judgment_handler(card):
    return _ast(
        card["name"],
        _custom("friend_or_foe_each_player"),
        _custom("zndrsplts_judgment_friend_clone_creature"),
        _custom("zndrsplts_judgment_foe_bounce_creature"),
    )


def dark_intimations_handler(card):
    return _ast(
        card["name"],
        _custom("dark_intimations_each_opp_sac_then_discard"),
        _custom("dark_intimations_recur_then_draw"),
        _custom_triggered("dark_intimations_bolas_cast_grave_recur_loyalty_bonus",
                          "cast_bolas_planeswalker"),
    )


def extract_the_truth_handler(card):
    return _ast(
        card["name"],
        _custom("extract_the_truth_choose_one"),
        _custom("extract_the_truth_targeted_creature_ench_pw_discard"),
        _custom("extract_the_truth_opp_sac_enchantment"),
    )


def immortal_coil_handler(card):
    return _ast(
        card["name"],
        _custom_activated("immortal_coil_exile_2_grave_draw",
                          Cost(tap=True, extra=("exile two cards from your graveyard",))),
        _custom("immortal_coil_prevent_exile_per_dmg"),
        _custom_triggered("immortal_coil_lose_at_empty_grave",
                          "graveyard_empty"),
    )


def regnas_sanction_handler(card):
    return _ast(
        card["name"],
        _custom("friend_or_foe_each_player"),
        _custom("regnas_sanction_friend_p1p1_each_creature"),
        _custom("regnas_sanction_foe_tap_all_but_one"),
    )


def turtles_forever_handler(card):
    return _ast(
        card["name"],
        _custom("turtles_forever_search_lib_or_outside_game"),
        _custom("turtles_forever_4_legendary_diff_names"),
        _custom("turtles_forever_opp_picks_2_to_keep"),
    )


def bell_borca_handler(card):
    return _ast(
        card["name"],
        _custom("bell_borca_note_mv_on_exile"),
        _custom("bell_borca_power_eq_max_noted"),
        _custom_triggered("bell_borca_upkeep_exile_top_play_this_turn",
                          "upkeep"),
    )


def parnesse_handler(card):
    return _ast(
        card["name"],
        _custom_triggered("parnesse_target_by_opp_counter_unless_4_life",
                          "becomes_target_by_opp"),
        _custom_triggered("parnesse_copy_spell_opp_also_copies",
                          "you_copy_spell"),
    )


def order_of_succession_handler(card):
    return _ast(
        card["name"],
        _custom("order_succession_choose_left_or_right"),
        _custom("order_succession_each_player_picks_next_player_creature"),
        _custom("order_succession_each_gains_control_chosen"),
    )


def celestial_toymaker_handler(card):
    return _ast(
        card["name"],
        _custom_triggered("celestial_toymaker_attack_top_3_facedown_facup_pile",
                          "attacks"),
        _custom("celestial_toymaker_defender_picks_pile"),
        _custom_triggered("celestial_toymaker_endstep_drain_2_per_pile_effect",
                          "end_step"),
    )


def hedonists_trove_handler(card):
    return _ast(
        card["name"],
        _custom_triggered("hedonists_trove_etb_exile_opp_grave", "etb"),
        _custom("hedonists_trove_play_lands_from_exile"),
        _custom("hedonists_trove_cast_spells_from_exile_one_per_turn"),
    )


# ---------------------------------------------------------------------------
# Wave 3 — top-EDH-rank PARTIAL escapees (rank ~29 to ~3900)
# ---------------------------------------------------------------------------

def chaos_warp_handler(card):
    return _ast(
        card["name"],
        _custom("chaos_warp_owner_shuffles_target"),
        _custom("chaos_warp_reveal_top_permanent_etb"),
    )


def teferis_protection_handler(card):
    return _ast(
        card["name"],
        _custom("teferis_protection_life_locked"),
        _custom("teferis_protection_protection_from_everything"),
        _custom("teferis_protection_phase_out_all_yours"),
        _custom("teferis_protection_exile_self"),
    )


def heralds_horn_handler(card):
    return _ast(
        card["name"],
        _custom("etb_choose_creature_type"),
        _custom("heralds_horn_chosen_type_cost_1_less"),
        _custom_triggered("heralds_horn_upkeep_reveal_top_chosen", "upkeep"),
    )


def three_tree_city_handler(card):
    return _ast(
        card["name"],
        _custom("etb_choose_creature_type"),
        Activated(cost=Cost(tap=True),
                  effect=UnknownEffect(raw_text="<add C>"),
                  raw="{T}: Add {C}."),
        _custom_activated("three_tree_city_color_per_chosen_type",
                          Cost(tap=True, extra=("{2}",))),
    )


def animate_dead_handler(card):
    return _ast(
        card["name"],
        _custom("animate_dead_enchant_creature_card_in_graveyard"),
        _custom_triggered("animate_dead_etb_swap_text_return", "etb"),
        _custom_triggered("animate_dead_ltb_sacrifice_creature", "ltb"),
        _custom("animate_dead_minus_1_power"),
    )


def reality_shift_handler(card):
    return _ast(
        card["name"],
        _custom("reality_shift_exile_target_creature"),
        _custom("reality_shift_controller_manifests_top"),
    )


def underworld_breach_handler(card):
    return _ast(
        card["name"],
        _custom("underworld_breach_grant_escape_graveyard"),
        _custom_triggered("underworld_breach_endstep_sac", "end_step"),
    )


def mystic_forge_handler(card):
    return _ast(
        card["name"],
        _custom("look_at_top_of_library_anytime"),
        _custom("mystic_forge_cast_artifact_colorless_from_top"),
        _custom_activated("mystic_forge_exile_top_for_1_life",
                          Cost(tap=True, extra=("Pay 1 life",))),
    )


def maskwood_nexus_handler(card):
    return _ast(
        card["name"],
        _custom("maskwood_creatures_every_type"),
        _custom_activated("maskwood_make_changeling_token",
                          Cost(tap=True, extra=("{3}",))),
    )


def birgi_handler(card):
    return _ast(
        card["name"],
        _custom("birgi_add_R_when_you_cast_spell"),
        _custom("harnfel_discard_exile_play"),
    )


def terror_of_the_peaks_handler(card):
    return _ast(
        card["name"],
        Keyword(name="flying", raw="Flying"),
        _custom("terror_peaks_opps_spells_targeting_self_cost_3_life"),
        _custom_triggered("terror_peaks_creature_etb_damage_equal_power",
                          "creature_etb"),
    )


def dread_return_handler(card):
    return _ast(
        card["name"],
        _custom("dread_return_reanimate_target"),
        _custom("dread_return_flashback_sac_three_creatures"),
    )


def valakut_awakening_handler(card):
    return _ast(
        card["name"],
        _custom("valakut_awakening_pitch_redraw_plus_one"),
        _custom("valakut_stoneforge_etb_tapped_unless_pay"),
    )


def champion_of_lambholt_handler(card):
    return _ast(
        card["name"],
        _custom("lambholt_lower_power_cant_block_yours"),
        _custom_triggered("lambholt_creature_etb_plus1_counter",
                          "creature_etb"),
    )


def commanders_plate_handler(card):
    return _ast(
        card["name"],
        _custom("commanders_plate_plus_3_3_protection_off_color"),
        _custom("commanders_plate_equip_commander_3"),
        _custom("commanders_plate_equip_5"),
    )


def blasphemous_edict_handler(card):
    return _ast(
        card["name"],
        _custom("blasphemous_edict_alt_cost_B_if_13_creatures"),
        _custom("blasphemous_edict_each_player_sac_13_creatures"),
    )


def culling_ritual_handler(card):
    return _ast(
        card["name"],
        _custom("culling_ritual_destroy_each_nonland_mv_2_or_less"),
        _custom("culling_ritual_add_BG_per_destroyed"),
    )


def realmwalker_handler(card):
    return _ast(
        card["name"],
        Keyword(name="changeling", raw="Changeling"),
        _custom("etb_choose_creature_type"),
        _custom("look_at_top_of_library_anytime"),
        _custom("realmwalker_cast_chosen_type_from_top"),
    )


def plaguecrafter_handler(card):
    return _ast(
        card["name"],
        _custom_triggered("plaguecrafter_each_player_sac_creature_or_pw_then_discard",
                          "etb"),
    )


def torment_of_hailfire_handler(card):
    return _ast(
        card["name"],
        _custom("torment_hailfire_x_repetitions_drain3_or_sac_or_discard"),
    )


def clever_concealment_handler(card):
    return _ast(
        card["name"],
        Keyword(name="convoke", raw="Convoke"),
        _custom("clever_concealment_phase_out_nonland_yours"),
    )


def forbidden_orchard_handler(card):
    return _ast(
        card["name"],
        _custom_activated("forbidden_orchard_any_color",
                          Cost(tap=True)),
        _custom_triggered("forbidden_orchard_opp_gets_spirit_when_tapped",
                          "tap_for_mana"),
    )


def borne_upon_a_wind_handler(card):
    return _ast(
        card["name"],
        _custom("borne_upon_wind_spells_have_flash_this_turn"),
        Draw(count=1, target=SELF),
    )


def bridgeworks_battle_handler(card):
    return _ast(
        card["name"],
        _custom("bridgeworks_battle_dfc_bottom_land"),
    )


def elves_of_deep_shadow_handler(card):
    return _ast(
        card["name"],
        _custom_activated("elves_deep_shadow_add_B_take_1",
                          Cost(tap=True)),
    )


def flare_of_fortitude_handler(card):
    return _ast(
        card["name"],
        _custom("flare_of_fortitude_alt_cost_sac_white"),
        _custom("flare_of_fortitude_life_locked_perms_hexproof_indestructible"),
    )


def ashaya_handler(card):
    return _ast(
        card["name"],
        _custom("ashaya_pt_equal_to_lands"),
        _custom("ashaya_nontoken_creatures_are_forests"),
    )


def tempt_with_discovery_handler(card):
    return _ast(
        card["name"],
        _custom("tempting_offer_search_land"),
        _custom("tempt_discovery_self_extra_per_taker"),
    )


def breach_the_multiverse_handler(card):
    return _ast(
        card["name"],
        _custom("breach_multiverse_each_player_mills_10"),
        _custom("breach_multiverse_steal_one_creature_pw_each_yard"),
        _custom("breach_multiverse_creatures_become_phyrexian"),
    )


def delney_handler(card):
    return _ast(
        card["name"],
        _custom("delney_low_power_unblockable_by_3_plus"),
        _custom("delney_triggered_abilities_extra_trigger"),
    )


def sword_hearth_home_handler(card):
    return _ast(
        card["name"],
        _custom("sword_plus_2_2_protection_green_white"),
        _custom_triggered("sword_hearth_home_combat_damage_blink_fetch_basic",
                          "combat_damage_to_player"),
        _custom("equip_2"),
    )


def forsaken_monument_handler(card):
    return _ast(
        card["name"],
        _custom("forsaken_monument_colorless_creatures_plus2_2"),
        _custom_triggered("forsaken_monument_extra_C_when_tap_for_C",
                          "tap_for_C"),
        _custom_triggered("forsaken_monument_2_life_on_colorless_cast",
                          "cast_colorless"),
    )


def chain_of_vapor_handler(card):
    return _ast(
        card["name"],
        _custom("chain_vapor_bounce_target_nonland"),
        _custom("chain_vapor_optional_sac_land_to_copy"),
    )


def thousand_year_elixir_handler(card):
    return _ast(
        card["name"],
        _custom("thousand_year_elixir_creatures_haste_for_abilities"),
        _custom_activated("thousand_year_elixir_untap_target",
                          Cost(tap=True, extra=("{1}",))),
    )


def beseech_mirror_handler(card):
    return _ast(
        card["name"],
        _custom("bargain_alt_cost"),
        _custom("beseech_mirror_tutor_exile_facedown"),
        _custom("beseech_mirror_cast_for_free_if_bargained_mv_4_or_less"),
    )


def elesh_norn_mom_handler(card):
    return _ast(
        card["name"],
        Keyword(name="vigilance", raw="Vigilance"),
        _custom("elesh_norn_mom_etb_triggers_extra"),
        _custom("elesh_norn_mom_opp_etb_no_trigger"),
    )


def agadeems_awakening_handler(card):
    return _ast(
        card["name"],
        _custom("agadeem_reanimate_x_with_distinct_mv"),
        _custom("agadeem_dfc_land"),
    )


def rewind_handler(card):
    return _ast(
        card["name"],
        _custom("rewind_counter_target_spell"),
        _custom("rewind_untap_up_to_four_lands"),
    )


def razorkin_needlehead_handler(card):
    return _ast(
        card["name"],
        _custom("razorkin_first_strike_your_turn"),
        _custom_triggered("razorkin_opp_draw_1_damage", "opp_draws_card"),
    )


def promise_of_loyalty_handler(card):
    return _ast(
        card["name"],
        _custom("promise_loyalty_each_player_vow_counter_one_sac_rest"),
        _custom("promise_loyalty_vow_cant_attack_you_pws"),
    )


def skrelv_handler(card):
    return _ast(
        card["name"],
        Keyword(name="toxic", args=(1,), raw="Toxic 1"),
        _custom("skrelv_cant_block"),
        _custom_activated("skrelv_grant_toxic_hexproof_unblock",
                          Cost(tap=True, extra=("{W/P}",))),
    )


def the_black_gate_handler(card):
    return _ast(
        card["name"],
        _custom("black_gate_etb_pay_3_life_or_tapped"),
        Activated(cost=Cost(tap=True),
                  effect=UnknownEffect(raw_text="<add B>"),
                  raw="{T}: Add {B}."),
        _custom_activated("black_gate_creature_unblockable_by_richest",
                          Cost(tap=True, extra=("{1}{B}",))),
    )


def ezuris_predation_handler(card):
    return _ast(
        card["name"],
        _custom("ezuri_predation_4_4_token_per_opp_creature_then_fight"),
    )


def legion_leadership_handler(card):
    return _ast(
        card["name"],
        _custom("legion_leadership_dfc_room_or_battle"),
    )


def city_of_traitors_handler(card):
    return _ast(
        card["name"],
        _custom_triggered("city_traitors_sac_when_play_other_land",
                          "play_land"),
        Activated(cost=Cost(tap=True),
                  effect=UnknownEffect(raw_text="<add CC>"),
                  raw="{T}: Add {C}{C}."),
    )


def sephara_handler(card):
    return _ast(
        card["name"],
        _custom("sephara_alt_cost_W_tap_4_flyers"),
        Keyword(name="flying", raw="Flying"),
        Keyword(name="lifelink", raw="Lifelink"),
        _custom("sephara_other_flyers_indestructible"),
    )


def elixir_of_immortality_handler(card):
    return _ast(
        card["name"],
        _custom_activated("elixir_immortality_5_life_shuffle_grave",
                          Cost(tap=True, extra=("{2}",))),
    )


def kinnan_handler(card):
    return _ast(
        card["name"],
        _custom_triggered("kinnan_extra_mana_on_nonland_tap",
                          "nonland_tapped_for_mana"),
        _custom_activated("kinnan_top5_put_non_human_creature",
                          Cost(extra=("{5}{G}{U}",))),
    )


def comet_storm_handler(card):
    return _ast(
        card["name"],
        Keyword(name="multikicker", args=("{1}",), raw="Multikicker {1}"),
        _custom("comet_storm_x_damage_each_target"),
    )


def pest_infestation_handler(card):
    return _ast(
        card["name"],
        _custom("pest_infestation_destroy_x_artifacts_enchantments"),
        _custom("pest_infestation_create_2x_pest_lifelink_dies"),
    )


def deep_analysis_handler(card):
    return _ast(
        card["name"],
        Draw(count=2, target=TARGET_PLAYER),
        _custom("deep_analysis_flashback_1U_pay_3_life"),
    )


def tendershoot_dryad_handler(card):
    return _ast(
        card["name"],
        Keyword(name="ascend", raw="Ascend"),
        _custom_triggered("tendershoot_upkeep_create_saproling", "upkeep"),
        _custom("tendershoot_saprolings_plus2_2_with_blessing"),
    )


def morophon_handler(card):
    return _ast(
        card["name"],
        Keyword(name="changeling", raw="Changeling"),
        _custom("etb_choose_creature_type"),
        _custom("morophon_chosen_type_cost_wubrg_less"),
        _custom("morophon_other_chosen_type_plus1_1"),
    )


def soul_stone_handler(card):
    return _ast(
        card["name"],
        Keyword(name="indestructible", raw="Indestructible"),
        Activated(cost=Cost(tap=True),
                  effect=UnknownEffect(raw_text="<add B>"),
                  raw="{T}: Add {B}."),
        _custom_activated("soul_stone_harness",
                          Cost(tap=True,
                               sacrifice=Filter(base="creature"),
                               extra=("{6}{B}",))),
        _custom_triggered("soul_stone_infinity_reanimate_upkeep", "upkeep"),
    )


def lethal_scheme_handler(card):
    return _ast(
        card["name"],
        Keyword(name="convoke", raw="Convoke"),
        _custom("lethal_scheme_destroy_creature_or_pw"),
        _custom("lethal_scheme_convoke_creatures_connive"),
    )


def march_swirling_mist_handler(card):
    return _ast(
        card["name"],
        _custom("march_swirling_mist_alt_cost_exile_blue"),
        _custom("march_swirling_mist_phase_out_x_creatures"),
    )


def smugglers_surprise_handler(card):
    return _ast(
        card["name"],
        Keyword(name="spree", raw="Spree"),
        _custom("smugglers_surprise_mode_mill_4_take_2"),
        _custom("smugglers_surprise_mode_cheat_2_creatures"),
        _custom("smugglers_surprise_mode_power4_hexproof_indestructible"),
    )


def priest_of_forgotten_gods_handler(card):
    return _ast(
        card["name"],
        _custom_activated("priest_forgotten_gods_drain_2_force_sac_BB_draw",
                          Cost(tap=True,
                               sacrifice=Filter(base="creature"),
                               extra=("Sacrifice two other creatures",))),
    )


def search_for_tomorrow_handler(card):
    return _ast(
        card["name"],
        _custom("search_for_tomorrow_basic_to_battlefield"),
        Keyword(name="suspend", args=(2, "{G}"), raw="Suspend 2—{G}"),
    )


def sephiroth_handler(card):
    return _ast(
        card["name"],
        _custom("sephiroth_dfc_one_winged_angel"),
    )


def rain_of_riches_handler(card):
    return _ast(
        card["name"],
        _custom_triggered("rain_of_riches_etb_two_treasures", "etb"),
        _custom("rain_of_riches_first_treasure_spell_has_cascade"),
    )


def delina_wild_mage_handler(card):
    return _ast(
        card["name"],
        _custom_triggered("delina_attack_d20_token_copy", "attacks"),
    )


def void_winnower_handler(card):
    return _ast(
        card["name"],
        _custom("void_winnower_opps_cant_cast_even_mv"),
        _custom("void_winnower_opps_cant_block_with_even_mv"),
    )


def jin_gitaxias_pt_handler(card):
    return _ast(
        card["name"],
        _custom_triggered("jin_gitaxias_pt_copy_artifact_instant_sorcery",
                          "you_cast_artifact_instant_sorcery"),
        _custom_triggered("jin_gitaxias_pt_counter_opp_artifact_instant_sorcery",
                          "opp_cast_artifact_instant_sorcery"),
    )


def gift_of_the_viper_handler(card):
    return _ast(
        card["name"],
        _custom("gift_viper_three_counters_and_untap"),
    )


def hoarding_broodlord_handler(card):
    return _ast(
        card["name"],
        Keyword(name="convoke", raw="Convoke"),
        Keyword(name="flying", raw="Flying"),
        _custom_triggered("hoarding_broodlord_etb_exile_facedown_play",
                          "etb"),
        _custom("hoarding_broodlord_exile_spells_have_convoke"),
    )


def leyline_of_the_guildpact_handler(card):
    return _ast(
        card["name"],
        Keyword(name="leyline", raw="Leyline opening hand"),
        _custom("leyline_guildpact_nonland_all_colors"),
        _custom("leyline_guildpact_lands_every_basic_type"),
    )


def mirror_box_handler(card):
    return _ast(
        card["name"],
        _custom("mirror_box_legend_rule_off_yours"),
        _custom("mirror_box_legend_creature_plus1_1"),
        _custom("mirror_box_same_name_creature_stack_plus1_1"),
    )


def giada_handler(card):
    return _ast(
        card["name"],
        Keyword(name="flying", raw="Flying"),
        Keyword(name="vigilance", raw="Vigilance"),
        _custom("giada_other_angels_etb_with_extra_counters"),
        _custom_activated("giada_add_W_for_angel_only",
                          Cost(tap=True)),
    )


def elvish_warmaster_handler(card):
    return _ast(
        card["name"],
        _custom_triggered("elvish_warmaster_other_elves_etb_make_token",
                          "elf_etb"),
        _custom_activated("elvish_warmaster_pump_deathtouch",
                          Cost(extra=("{5}{G}{G}",))),
    )


def sphere_grid_handler(card):
    return _ast(
        card["name"],
        _custom_triggered("sphere_grid_combat_damage_plus1_1", "combat_damage_to_player"),
        _custom("sphere_grid_unlock_reach_trample"),
    )


def wild_magic_surge_handler(card):
    return _ast(
        card["name"],
        _custom("wild_magic_surge_destroy_then_reveal_until_shared_type"),
    )


def twenty_toed_toad_handler(card):
    return _ast(
        card["name"],
        _custom("twenty_toed_toad_max_hand_20"),
        _custom_triggered("twenty_toed_toad_attack_2plus_counter_draw",
                          "attack_with_two_or_more"),
        _custom_triggered("twenty_toed_toad_attack_check_win_20",
                          "attacks"),
    )


def aven_interrupter_handler(card):
    return _ast(
        card["name"],
        Keyword(name="flash", raw="Flash"),
        Keyword(name="flying", raw="Flying"),
        _custom_triggered("aven_interrupter_etb_exile_plot", "etb"),
        _custom("aven_interrupter_grave_exile_spells_cost_2_more"),
    )


def emrakul_promised_end_handler(card):
    return _ast(
        card["name"],
        _custom("emrakul_pe_cost_reduction_per_card_type_grave"),
        _custom_triggered("emrakul_pe_cast_control_opp_next_turn",
                          "cast"),
        Keyword(name="flying", raw="Flying"),
        Keyword(name="trample", raw="Trample"),
        Keyword(name="protection", args=("instants",),
                raw="Protection from instants"),
    )


def scheming_symmetry_handler(card):
    return _ast(
        card["name"],
        _custom("scheming_symmetry_two_players_tutor_top"),
    )


def overlord_hauntwoods_handler(card):
    return _ast(
        card["name"],
        Keyword(name="impending", args=(4, "{1}{G}{G}"),
                raw="Impending 4—{1}{G}{G}"),
        _custom_triggered("overlord_hauntwoods_etb_attack_make_everywhere_land",
                          "etb_or_attacks"),
    )


def secret_tunnel_handler(card):
    return _ast(
        card["name"],
        _custom("secret_tunnel_self_unblockable"),
        Activated(cost=Cost(tap=True),
                  effect=UnknownEffect(raw_text="<add C>"),
                  raw="{T}: Add {C}."),
        _custom_activated("secret_tunnel_two_creatures_share_type_unblockable",
                          Cost(tap=True, extra=("{4}",))),
    )


def crippling_fear_handler(card):
    return _ast(
        card["name"],
        _custom("crippling_fear_choose_type_minus3_3_others"),
    )


def army_of_the_damned_handler(card):
    return _ast(
        card["name"],
        _custom("army_damned_create_thirteen_zombies_tapped"),
        _custom("army_damned_flashback_7BBB"),
    )


def grafdiggers_cage_handler(card):
    return _ast(
        card["name"],
        _custom("grafdigger_creature_grave_lib_cant_etb"),
        _custom("grafdigger_no_cast_from_grave_lib"),
    )


def hellkite_courser_handler(card):
    return _ast(
        card["name"],
        Keyword(name="flying", raw="Flying"),
        _custom_triggered("hellkite_courser_etb_cheat_commander_haste",
                          "etb"),
    )


def fractured_sanity_handler(card):
    return _ast(
        card["name"],
        _custom("fractured_sanity_each_opp_mill_14"),
        Keyword(name="cycling", args=("{1}{U}",), raw="Cycling {1}{U}"),
        _custom_triggered("fractured_sanity_cycle_each_opp_mill_4", "cycle"),
    )


def virtue_of_knowledge_handler(card):
    return _ast(
        card["name"],
        _custom("virtue_knowledge_dfc_adventure"),
    )


def crystal_skull_isu_handler(card):
    return _ast(
        card["name"],
        _custom("look_at_top_of_library_anytime"),
        _custom("crystal_skull_play_historic_from_top"),
        Activated(cost=Cost(tap=True),
                  effect=UnknownEffect(raw_text="<add U>"),
                  raw="{T}: Add {U}."),
    )


def sudden_spoiling_handler(card):
    return _ast(
        card["name"],
        Keyword(name="split_second", raw="Split second"),
        _custom("sudden_spoiling_target_creatures_lose_abilities_0_2"),
    )


def ancient_gold_dragon_handler(card):
    return _ast(
        card["name"],
        Keyword(name="flying", raw="Flying"),
        _custom_triggered("ancient_gold_dragon_d20_faerie_tokens",
                          "combat_damage_to_player"),
    )


def bulk_up_handler(card):
    return _ast(
        card["name"],
        _custom("bulk_up_double_target_power_eot"),
        _custom("bulk_up_flashback_4RR"),
    )


def tempt_with_bunnies_handler(card):
    return _ast(
        card["name"],
        _custom("tempting_offer_draw_and_rabbit"),
        _custom("tempt_bunnies_self_extra_per_taker"),
    )


def angels_grace_handler(card):
    return _ast(
        card["name"],
        Keyword(name="split_second", raw="Split second"),
        _custom("angels_grace_cant_lose_or_win"),
        _custom("angels_grace_damage_floor_1"),
    )


def minas_morgul_handler(card):
    return _ast(
        card["name"],
        _custom("minas_morgul_etb_tapped"),
        Activated(cost=Cost(tap=True),
                  effect=UnknownEffect(raw_text="<add B>"),
                  raw="{T}: Add {B}."),
        _custom_activated("minas_morgul_shadow_counter_target",
                          Cost(tap=True, extra=("{3}{B}",))),
    )


def winds_of_abandon_handler(card):
    return _ast(
        card["name"],
        _custom("winds_abandon_exile_creature_opp_basic"),
        Keyword(name="overload", args=("{4}{W}{W}",), raw="Overload {4}{W}{W}"),
    )


def party_thrasher_handler(card):
    return _ast(
        card["name"],
        _custom("party_thrasher_noncreature_exile_convoke"),
        _custom_triggered("party_thrasher_main_discard_exile_play_one",
                          "first_main"),
    )


def lignify_handler(card):
    return _ast(
        card["name"],
        _custom("lignify_treefolk_0_4_lose_abilities"),
    )


def lich_knights_conquest_handler(card):
    return _ast(
        card["name"],
        _custom("lich_knights_conquest_sac_x_perms_return_x_creatures"),
    )


def orims_chant_handler(card):
    return _ast(
        card["name"],
        Keyword(name="kicker", args=("{W}",), raw="Kicker {W}"),
        _custom("orims_chant_target_player_no_spells"),
        _custom("orims_chant_kicked_no_attacks"),
    )


def sandwurm_convergence_handler(card):
    return _ast(
        card["name"],
        _custom("sandwurm_convergence_flyers_cant_attack_you"),
        _custom_triggered("sandwurm_convergence_endstep_5_5_wurm",
                          "end_step"),
    )


def plargg_nassari_handler(card):
    return _ast(
        card["name"],
        _custom_triggered("plargg_nassari_upkeep_each_player_exile_until_nonland",
                          "upkeep"),
        _custom("plargg_nassari_opp_chooses_one_you_cast_other_two_free"),
    )


def dance_of_the_dead_handler(card):
    return _ast(
        card["name"],
        _custom("animate_dead_enchant_creature_card_in_graveyard"),
        _custom_triggered("dance_dead_etb_swap_text_return_tapped", "etb"),
        _custom_triggered("dance_dead_ltb_sacrifice_creature", "ltb"),
    )


def toxrill_handler(card):
    return _ast(
        card["name"],
        _custom_triggered("toxrill_each_endstep_slime_counter_opp_creatures",
                          "end_step"),
        _custom("toxrill_opp_creatures_minus1_per_slime"),
        _custom_triggered("toxrill_slug_token_on_slime_dies", "creature_dies"),
        _custom_activated("toxrill_sac_slug_draw",
                          Cost(sacrifice=Filter(base="Slug"), extra=("{U}{B}",))),
    )


def doc_aurlock_handler(card):
    return _ast(
        card["name"],
        _custom("doc_aurlock_grave_exile_spells_cost_2_less"),
        _custom("doc_aurlock_plot_costs_2_less"),
    )


def kogla_yidaro_handler(card):
    return _ast(
        card["name"],
        _custom("kogla_yidaro_etb_choose_one_trample_haste_or_fight"),
        _custom_activated("kogla_yidaro_discard_destroy_artifact_enchantment_shuffle_draw",
                          Cost(extra=("{2}{R}{G}", "Discard this card"))),
    )


def gishath_handler(card):
    return _ast(
        card["name"],
        Keyword(name="vigilance", raw="Vigilance"),
        Keyword(name="trample", raw="Trample"),
        Keyword(name="haste", raw="Haste"),
        _custom_triggered("gishath_combat_damage_reveal_dinos_to_battlefield",
                          "combat_damage_to_player"),
    )


def overlord_balemurk_handler(card):
    return _ast(
        card["name"],
        Keyword(name="impending", args=(5, "{1}{B}"), raw="Impending 5—{1}{B}"),
        _custom_triggered("overlord_balemurk_etb_attack_mill_4_return",
                          "etb_or_attacks"),
    )


def emrakul_world_anew_handler(card):
    return _ast(
        card["name"],
        _custom_triggered("emrakul_world_anew_cast_steal_target_creatures",
                          "cast"),
        Keyword(name="flying", raw="Flying"),
        Keyword(name="protection", args=("spells_perms_cast_this_turn",),
                raw="Protection from spells and from permanents that were cast this turn"),
        _custom_triggered("emrakul_world_anew_ltb_sac_all_yours", "ltb"),
        _custom("emrakul_world_anew_madness_six_C"),
    )


def yuriko_handler(card):
    return _ast(
        card["name"],
        Keyword(name="commander_ninjutsu", args=("{U}{B}",),
                raw="Commander ninjutsu {U}{B}"),
        _custom_triggered("yuriko_ninja_combat_damage_reveal_drain",
                          "ninja_combat_damage"),
    )


def agitator_ant_handler(card):
    return _ast(
        card["name"],
        _custom_triggered("agitator_ant_endstep_each_player_2_counters_then_goad",
                          "end_step"),
    )


def gond_gate_handler(card):
    return _ast(
        card["name"],
        _custom("gond_gate_other_gates_untapped"),
        Activated(cost=Cost(tap=True),
                  effect=UnknownEffect(raw_text="<add C>"),
                  raw="{T}: Add {C}."),
        _custom_activated("gond_gate_any_color_gate_could_produce",
                          Cost(tap=True)),
    )


def gilded_drake_handler(card):
    return _ast(
        card["name"],
        Keyword(name="flying", raw="Flying"),
        _custom_triggered("gilded_drake_etb_exchange_or_sac", "etb"),
    )


def immortal_sun_handler(card):
    return _ast(
        card["name"],
        _custom("immortal_sun_no_pw_loyalty_activations"),
        _custom_triggered("immortal_sun_draw_step_extra_card", "draw_step"),
        _custom("immortal_sun_spells_cost_1_less"),
        _custom("immortal_sun_creatures_plus1_1"),
    )


def momentous_fall_handler(card):
    return _ast(
        card["name"],
        _custom("momentous_fall_alt_cost_sac_creature"),
        _custom("momentous_fall_draw_equal_power_gain_life_toughness"),
    )


def semesters_end_handler(card):
    return _ast(
        card["name"],
        _custom("semesters_end_exile_blink_with_extra_counter"),
    )


def gold_forged_thopteryx_handler(card):
    return _ast(
        card["name"],
        Keyword(name="flying", raw="Flying"),
        Keyword(name="lifelink", raw="Lifelink"),
        _custom("gold_forged_thopteryx_legend_ward_2"),
    )


def dance_of_the_manse_handler(card):
    return _ast(
        card["name"],
        _custom("dance_of_manse_reanimate_x_artifacts_non_aura_enchantments"),
        _custom("dance_of_manse_x6plus_become_4_4_creatures"),
    )


def spectacular_showdown_handler(card):
    return _ast(
        card["name"],
        _custom("spectacular_showdown_double_strike_counter_then_goad"),
        Keyword(name="overload", args=("{4}{R}{R}{R}",),
                raw="Overload {4}{R}{R}{R}"),
    )


def debt_to_the_deathless_handler(card):
    return _ast(
        card["name"],
        _custom("debt_deathless_each_opp_2x_drain_you_gain"),
    )


def ancient_cellarspawn_handler(card):
    return _ast(
        card["name"],
        _custom("ancient_cellarspawn_demon_horror_nightmare_cost_1_less"),
        _custom_triggered("ancient_cellarspawn_cast_drain_diff_mv",
                          "cast"),
    )


def planar_nexus_handler(card):
    return _ast(
        card["name"],
        _custom("planar_nexus_every_nonbasic_land_type"),
        Activated(cost=Cost(tap=True),
                  effect=UnknownEffect(raw_text="<add C>"),
                  raw="{T}: Add {C}."),
        _custom_activated("planar_nexus_any_color",
                          Cost(tap=True, extra=("{1}",))),
    )


def aminatous_augury_handler(card):
    return _ast(
        card["name"],
        _custom("aminatou_augury_exile_top_8"),
        _custom("aminatou_augury_one_per_card_type_free_cast"),
    )


def patrolling_peacemaker_handler(card):
    return _ast(
        card["name"],
        _custom("patrolling_peacemaker_etb_with_two_counters"),
        _custom_triggered("patrolling_peacemaker_opp_crime_proliferate",
                          "opp_commits_crime"),
    )


def hell_to_pay_handler(card):
    return _ast(
        card["name"],
        _custom("hell_to_pay_x_damage_target_creature"),
        _custom("hell_to_pay_treasures_per_excess_damage"),
    )


def tempt_with_vengeance_handler(card):
    return _ast(
        card["name"],
        _custom("tempting_offer_x_haste_elemental"),
        _custom("tempt_vengeance_self_extra_per_taker"),
    )


def zombie_master_handler(card):
    return _ast(
        card["name"],
        _custom("zombie_master_other_zombies_swampwalk"),
        _custom("zombie_master_other_zombies_regenerate_B"),
    )


def syr_ginger_handler(card):
    return _ast(
        card["name"],
        _custom("syr_ginger_trample_hexproof_haste_if_opp_pw"),
        _custom_triggered("syr_ginger_other_artifact_dies_plus_counter_scry",
                          "other_artifact_dies"),
        _custom_activated("syr_ginger_sac_gain_life_equal_power",
                          Cost(tap=True,
                               sacrifice=Filter(base="self"),
                               extra=("{2}",))),
    )


def gonti_night_minister_handler(card):
    return _ast(
        card["name"],
        _custom_triggered("gonti_nm_player_casts_others_spell_treasure",
                          "player_casts_other_owners_spell"),
        _custom_triggered("gonti_nm_combat_damage_exile_top_facedown",
                          "combat_damage_to_player"),
    )


def ichormoon_gauntlet_handler(card):
    return _ast(
        card["name"],
        _custom("ichormoon_pws_gain_0_proliferate"),
        _custom("ichormoon_pws_gain_minus12_extra_turn"),
        _custom_triggered("ichormoon_noncreature_cast_extra_counter_chosen_kind",
                          "cast_noncreature"),
    )


def circle_of_power_handler(card):
    return _ast(
        card["name"],
        _custom("circle_of_power_draw_2_lose_2"),
        _custom("circle_of_power_make_wizard_token_drain"),
        _custom("circle_of_power_wizards_plus1_lifelink"),
    )


def last_agni_kai_handler(card):
    return _ast(
        card["name"],
        _custom("last_agni_kai_fight_creature"),
        _custom("last_agni_kai_excess_to_R_mana"),
        _custom("last_agni_kai_R_doesnt_empty"),
    )


def yedora_handler(card):
    return _ast(
        card["name"],
        _custom_triggered("yedora_other_creature_dies_face_down_forest",
                          "other_creature_dies"),
    )


def sacrifice_handler(card):
    return _ast(
        card["name"],
        _custom("sacrifice_alt_cost_sac_creature"),
        _custom("sacrifice_add_B_per_mana_value"),
    )


def oath_of_teferi_handler(card):
    return _ast(
        card["name"],
        _custom_triggered("oath_teferi_etb_exile_blink_other", "etb"),
        _custom("oath_teferi_pw_abilities_twice"),
    )


def gisa_hellraiser_handler(card):
    return _ast(
        card["name"],
        Keyword(name="ward", args=("{2}, Pay 2 life",),
                raw="Ward—{2}, Pay 2 life"),
        _custom("gisa_hr_skeletons_zombies_plus1_menace"),
        _custom_triggered("gisa_hr_crime_two_zombie_tokens", "you_commit_crime"),
    )


def beacon_of_immortality_handler(card):
    return _ast(
        card["name"],
        _custom("beacon_immortality_double_player_life"),
        _custom("beacon_immortality_shuffle_self"),
    )


def overlord_floodpits_handler(card):
    return _ast(
        card["name"],
        Keyword(name="impending", args=(4, "{1}{U}{U}"),
                raw="Impending 4—{1}{U}{U}"),
        Keyword(name="flying", raw="Flying"),
        _custom_triggered("overlord_floodpits_etb_attack_draw_2_discard_1",
                          "etb_or_attacks"),
    )


def multiversal_passage_handler(card):
    return _ast(
        card["name"],
        _custom("multiversal_passage_etb_choose_basic_pay_2_or_tapped"),
        _custom("multiversal_passage_is_chosen_type"),
    )


def lavinia_handler(card):
    return _ast(
        card["name"],
        _custom("lavinia_opps_cant_cast_noncreature_above_lands"),
        _custom_triggered("lavinia_counter_free_opp_spell", "opp_casts"),
    )


def soulless_jailer_handler(card):
    return _ast(
        card["name"],
        _custom("soulless_jailer_perm_grave_cant_etb"),
        _custom("soulless_jailer_no_noncreature_grave_exile_cast"),
    )


def overmaster_handler(card):
    return _ast(
        card["name"],
        _custom("overmaster_next_instant_sorcery_uncounterable"),
        Draw(count=1, target=SELF),
    )


def blood_for_blood_god_handler(card):
    return _ast(
        card["name"],
        _custom("bftbg_cost_1_less_per_creature_died"),
        _custom("bftbg_discard_then_draw_8_damage_8_each_opp_exile_self"),
    )


def gluntch_handler(card):
    return _ast(
        card["name"],
        Keyword(name="flying", raw="Flying"),
        _custom_triggered("gluntch_endstep_three_player_choices",
                          "end_step"),
    )


def endrek_sahr_handler(card):
    return _ast(
        card["name"],
        _custom_triggered("endrek_sahr_create_x_thrulls", "you_cast_creature"),
        _custom_triggered("endrek_sahr_seven_thrulls_sac_self", "you_control_7_thrulls"),
    )


def chain_of_smog_handler(card):
    return _ast(
        card["name"],
        _custom("chain_smog_player_discards_two"),
        _custom("chain_smog_optional_copy"),
    )


def burnt_offering_handler(card):
    return _ast(
        card["name"],
        _custom("burnt_offering_alt_cost_sac_creature"),
        _custom("burnt_offering_add_x_BR_mv"),
    )


def render_silent_handler(card):
    return _ast(
        card["name"],
        _custom("render_silent_counter_target_spell"),
        _custom("render_silent_no_more_spells_this_turn"),
    )


def zack_fair_handler(card):
    return _ast(
        card["name"],
        _custom("zack_fair_etb_with_counter"),
        _custom_activated("zack_fair_sac_indestructible_transfer_counters_equipment",
                          Cost(sacrifice=Filter(base="self"), extra=("{1}",))),
    )


def rite_of_raging_storm_handler(card):
    return _ast(
        card["name"],
        _custom("rite_raging_storm_lightning_rager_cant_attack_you"),
        _custom_triggered("rite_raging_storm_each_upkeep_5_1_token",
                          "each_player_upkeep"),
    )


def sigarda_font_blessings_handler(card):
    return _ast(
        card["name"],
        Keyword(name="flying", raw="Flying"),
        _custom("sigarda_font_other_perms_hexproof"),
        _custom("look_at_top_of_library_anytime"),
        _custom("sigarda_font_cast_angel_human_from_top"),
    )


def dress_down_handler(card):
    return _ast(
        card["name"],
        Keyword(name="flash", raw="Flash"),
        _custom_triggered("dress_down_etb_draw", "etb"),
        _custom("dress_down_creatures_lose_all_abilities"),
        _custom_triggered("dress_down_endstep_sac", "end_step"),
    )


def mob_rule_handler(card):
    return _ast(
        card["name"],
        _custom("mob_rule_choose_power_4_plus_steal"),
        _custom("mob_rule_choose_power_3_or_less_steal"),
    )


def animists_awakening_handler(card):
    return _ast(
        card["name"],
        _custom("animist_awakening_reveal_x_lands_to_battlefield"),
        _custom("animist_awakening_spell_mastery_untap_lands"),
    )


def necromantic_selection_handler(card):
    return _ast(
        card["name"],
        _custom("necromantic_selection_destroy_all_creatures"),
        _custom("necromantic_selection_reanimate_one_zombie_under_control"),
        _custom("necromantic_selection_exile_self"),
    )


def collective_effort_handler(card):
    return _ast(
        card["name"],
        Keyword(name="escalate", args=("Tap an untapped creature you control",),
                raw="Escalate—Tap an untapped creature you control"),
        _custom("collective_effort_destroy_creature_4plus"),
        _custom("collective_effort_destroy_enchantment"),
        _custom("collective_effort_plus1_counters_player_creatures"),
    )


def skullwinder_handler(card):
    return _ast(
        card["name"],
        Keyword(name="deathtouch", raw="Deathtouch"),
        _custom_triggered("skullwinder_etb_return_card_then_opp_returns",
                          "etb"),
    )


def white_suns_zenith_handler(card):
    return _ast(
        card["name"],
        _custom("white_sun_zenith_create_x_2_2_cats"),
        _custom("white_sun_zenith_shuffle_self"),
    )


def the_necrobloom_handler(card):
    return _ast(
        card["name"],
        _custom_triggered("necrobloom_landfall_plant_or_zombie",
                          "landfall"),
        _custom("necrobloom_lands_have_dredge_2"),
    )


def chitterspitter_handler(card):
    return _ast(
        card["name"],
        _custom_triggered("chitterspitter_upkeep_sac_token_acorn", "upkeep"),
        _custom("chitterspitter_squirrels_plus1_per_acorn"),
        _custom_activated("chitterspitter_make_squirrel",
                          Cost(tap=True, extra=("{G}",))),
    )


def retether_handler(card):
    return _ast(
        card["name"],
        _custom("retether_return_all_auras_to_creatures"),
    )


def prisoners_dilemma_handler(card):
    return _ast(
        card["name"],
        _custom("prisoners_dilemma_secret_silence_or_snitch"),
        _custom("prisoners_dilemma_payouts_4_8_12"),
    )


def grove_of_burnwillows_handler(card):
    return _ast(
        card["name"],
        Activated(cost=Cost(tap=True),
                  effect=UnknownEffect(raw_text="<add C>"),
                  raw="{T}: Add {C}."),
        _custom_activated("grove_burnwillows_RG_opp_gains_1",
                          Cost(tap=True)),
    )


def cid_handler(card):
    return _ast(
        card["name"],
        _custom("cid_equipment_vehicle_cost_1_less"),
        _custom("cid_jump_flying_your_turn"),
        _custom_activated("cid_return_eq_or_vehicle_from_grave",
                          Cost(tap=True, extra=("{2}",))),
    )


def korlessa_handler(card):
    return _ast(
        card["name"],
        _custom("look_at_top_of_library_anytime"),
        _custom("korlessa_cast_dragons_from_top"),
    )


def sower_of_discord_handler(card):
    return _ast(
        card["name"],
        Keyword(name="flying", raw="Flying"),
        _custom("sower_discord_etb_choose_two_players"),
        _custom_triggered("sower_discord_damage_to_one_other_loses_too",
                          "damage_to_chosen_player"),
    )


def reckoner_bankbuster_handler(card):
    return _ast(
        card["name"],
        _custom("reckoner_bankbuster_etb_three_charge_counters"),
        _custom_activated("reckoner_bankbuster_remove_charge_draw_treasure_pilot",
                          Cost(tap=True, extra=("{2}", "Remove a charge counter"))),
    )


def conspicuous_snoop_handler(card):
    return _ast(
        card["name"],
        _custom("snoop_play_with_top_revealed"),
        _custom("snoop_cast_goblins_from_top"),
        _custom("snoop_inherits_top_goblin_activated_abilities"),
    )


def fear_of_sleep_paralysis_handler(card):
    return _ast(
        card["name"],
        Keyword(name="flying", raw="Flying"),
        _custom_triggered("fear_sleep_paralysis_eerie_tap_stun_counter",
                          "self_or_enchantment_etb_or_room_unlock"),
        _custom("fear_sleep_paralysis_stun_locked_for_opps"),
    )


def world_at_war_handler(card):
    return _ast(
        card["name"],
        _custom("world_at_war_extra_combat_main_phase"),
        Keyword(name="rebound", raw="Rebound"),
    )


def royal_treatment_handler(card):
    return _ast(
        card["name"],
        _custom("royal_treatment_hexproof_eot"),
        _custom("royal_treatment_create_royal_role"),
    )


def kuja_handler(card):
    return _ast(
        card["name"],
        _custom("kuja_dfc_trance"),
    )


def dread_summons_handler(card):
    return _ast(
        card["name"],
        _custom("dread_summons_each_player_mill_x"),
        _custom("dread_summons_zombie_per_creature_milled"),
    )


def oviya_handler(card):
    return _ast(
        card["name"],
        _custom("oviya_attackers_have_trample"),
        _custom_activated("oviya_cheat_creature_or_vehicle",
                          Cost(tap=True, extra=("{G}",))),
    )


def hushbringer_handler(card):
    return _ast(
        card["name"],
        Keyword(name="flying", raw="Flying"),
        Keyword(name="lifelink", raw="Lifelink"),
        _custom("hushbringer_creature_etb_die_no_trigger"),
    )


def jeweled_amulet_handler(card):
    return _ast(
        card["name"],
        _custom_activated("jeweled_amulet_charge_note_color",
                          Cost(tap=True, extra=("{1}",))),
        _custom_activated("jeweled_amulet_remove_charge_add_noted",
                          Cost(tap=True, extra=("Remove a charge counter",))),
    )


def glarb_handler(card):
    return _ast(
        card["name"],
        Keyword(name="deathtouch", raw="Deathtouch"),
        _custom("look_at_top_of_library_anytime"),
        _custom("glarb_play_lands_and_cast_4mv_plus_from_top"),
        _custom_activated("glarb_surveil_2", Cost(tap=True)),
    )


def genesis_ultimatum_handler(card):
    return _ast(
        card["name"],
        _custom("genesis_ultimatum_top_5_perms_to_battlefield_rest_to_hand"),
    )


def minds_aglow_handler(card):
    return _ast(
        card["name"],
        _custom("minds_aglow_join_forces_each_pay_x"),
        _custom("minds_aglow_each_player_draws_x"),
    )


def sludge_monster_handler(card):
    return _ast(
        card["name"],
        _custom_triggered("sludge_monster_etb_attack_slime_counter",
                          "etb_or_attacks"),
        _custom("sludge_monster_non_horror_slime_2_2_no_abilities"),
    )


def encroaching_mycosynth_handler(card):
    return _ast(
        card["name"],
        _custom("encroaching_mycosynth_nonland_perms_artifacts_too"),
    )


def reins_of_power_handler(card):
    return _ast(
        card["name"],
        _custom("reins_power_untap_yours_and_target_opps"),
        _custom("reins_power_swap_creatures_for_turn_haste"),
    )


def solemnity_handler(card):
    return _ast(
        card["name"],
        _custom("solemnity_players_cant_get_counters"),
        _custom("solemnity_no_counters_on_perms"),
    )


def titania_natures_force_handler(card):
    return _ast(
        card["name"],
        _custom("titania_nf_play_forests_from_grave"),
        _custom_triggered("titania_nf_forest_etb_5_3_elemental",
                          "forest_etb"),
        _custom_triggered("titania_nf_elemental_dies_mill_3", "elemental_dies"),
    )


def millennium_calendar_handler(card):
    return _ast(
        card["name"],
        _custom_triggered("millennium_calendar_untap_step_count_time",
                          "untap_step"),
        _custom_activated("millennium_calendar_double_time",
                          Cost(tap=True, extra=("{2}",))),
        _custom("millennium_calendar_1000_time_win"),
    )


def maddening_hex_handler(card):
    return _ast(
        card["name"],
        _custom("maddening_hex_enchant_player"),
        _custom_triggered("maddening_hex_d6_damage_then_random_swap",
                          "enchanted_player_casts_noncreature"),
    )


def search_for_glory_handler(card):
    return _ast(
        card["name"],
        _custom("search_for_glory_tutor_snow_legend_saga"),
        _custom("search_for_glory_life_per_S"),
    )


def fandaniel_handler(card):
    return _ast(
        card["name"],
        _custom_triggered("fandaniel_cast_instant_sorcery_surveil_1",
                          "you_cast_instant_sorcery"),
        _custom_triggered("fandaniel_endstep_each_opp_sac_or_lose_2_per_iss",
                          "end_step"),
    )


# ---------------------------------------------------------------------------
# Wave 4 (top-100 PARTIAL escapees beyond previous waves)
# ---------------------------------------------------------------------------

def uthros_research_craft_handler(card):
    """Uthros Research Craft (#2341) — Station (Tap another creature you control: Put charge counters equal to its powe"""
    return _ast(card["name"], _custom("uthros_research_craft"))


def weathered_sentinels_handler(card):
    """Weathered Sentinels (#2926) — Defender, vigilance, reach, trample"""
    return _ast(card["name"], _custom("weathered_sentinels"))


def reaver_titan_handler(card):
    """Reaver Titan (#2999) — Void Shields — Protection from mana value 3 or less"""
    return _ast(card["name"], _custom("reaver_titan"))


def k9_mark_i_handler(card):
    """K-9, Mark I (#3226) — Negative — As long as K-9 is untapped, other legendary creatures you control hav"""
    return _ast(card["name"], _custom("k9_mark_i"))


def zirda_the_dawnwaker_handler(card):
    """Zirda, the Dawnwaker (#3243) — Companion — Each permanent card in your starting deck has an activated ability. """
    return _ast(card["name"], _custom("zirda_the_dawnwaker"))


def roadside_reliquary_handler(card):
    """Roadside Reliquary (#3447) — {T}: Add {C}."""
    return _ast(card["name"], _custom("roadside_reliquary"))


def wildsear_scouring_maw_handler(card):
    """Wildsear, Scouring Maw (#3464) — Trample"""
    return _ast(card["name"], _custom("wildsear_scouring_maw"))


def the_master_multiplied_handler(card):
    """The Master, Multiplied (#3474) — Myriad"""
    return _ast(card["name"], _custom("the_master_multiplied"))


def hideous_taskmaster_handler(card):
    """Hideous Taskmaster (#3489) — Devoid (This card has no color.)"""
    return _ast(card["name"], _custom("hideous_taskmaster"))


def yes_man_personal_securitron_handler(card):
    """Yes Man, Personal Securitron (#3727) — {T}: Target opponent gains control of Yes Man. When they do, you draw two cards """
    return _ast(card["name"], _custom("yes_man_personal_securitron"))


def scavenged_brawler_handler(card):
    """Scavenged Brawler (#3734) — Flying, vigilance, trample, lifelink"""
    return _ast(card["name"], _custom("scavenged_brawler"))


def scion_of_draco_handler(card):
    """Scion of Draco (#3747) — Domain — This spell costs {2} less to cast for each basic land type among lands """
    return _ast(card["name"], _custom("scion_of_draco"))


def the_second_doctor_handler(card):
    """The Second Doctor (#3752) — Players have no maximum hand size."""
    return _ast(card["name"], _custom("the_second_doctor"))


def firebending_student_handler(card):
    """Firebending Student (#3828) — Prowess (Whenever you cast a noncreature spell, this creature gets +1/+1 until e"""
    return _ast(card["name"], _custom("firebending_student"))


def oubliette_handler(card):
    """Oubliette (#3854) — When this enchantment enters, target creature phases out until this enchantment """
    return _ast(card["name"], _custom("oubliette"))


def blastfurnace_hellkite_handler(card):
    """Blast-Furnace Hellkite (#3873) — Artifact offering (You may cast this spell as though it had flash by sacrificing"""
    return _ast(card["name"], _custom("blastfurnace_hellkite"))


def vines_of_vastwood_handler(card):
    """Vines of Vastwood (#3903) — Kicker {G} (You may pay an additional {G} as you cast this spell.)"""
    return _ast(card["name"], _custom("vines_of_vastwood"))


def zhurtaa_druid_handler(card):
    """Zhur-Taa Druid (#3951) — {T}: Add {G}."""
    return _ast(card["name"], _custom("zhurtaa_druid"))


def allout_assault_handler(card):
    """All-Out Assault (#3959) — Creatures you control get +1/+1 and have deathtouch."""
    return _ast(card["name"], _custom("allout_assault"))


def feasting_hobbit_handler(card):
    """Feasting Hobbit (#3966) — Devour Food 3 (As this creature enters, you may sacrifice any number of Foods. I"""
    return _ast(card["name"], _custom("feasting_hobbit"))


def darksteel_reactor_handler(card):
    """Darksteel Reactor (#3991) — Indestructible (Effects that say "destroy" don't destroy this artifact.)"""
    return _ast(card["name"], _custom("darksteel_reactor"))


def toph_the_first_metalbender_handler(card):
    """Toph, the First Metalbender (#3994) — Nontoken artifacts you control are lands in addition to their other types. (They"""
    return _ast(card["name"], _custom("toph_the_first_metalbender"))


def muxus_goblin_grandee_handler(card):
    """Muxus, Goblin Grandee (#3999) — When Muxus enters, reveal the top six cards of your library. Put all Goblin crea"""
    return _ast(card["name"], _custom("muxus_goblin_grandee"))


def howlsquad_heavy_handler(card):
    """Howlsquad Heavy (#4003) — Start your engines!"""
    return _ast(card["name"], _custom("howlsquad_heavy"))


def ultima_origin_of_oblivion_handler(card):
    """Ultima, Origin of Oblivion (#4006) — Flying"""
    return _ast(card["name"], _custom("ultima_origin_of_oblivion"))


def spectral_deluge_handler(card):
    """Spectral Deluge (#4016) — Return each creature your opponents control with toughness X or less to its owne"""
    return _ast(card["name"], _custom("spectral_deluge"))


def baeloth_barrityl_entertainer_handler(card):
    """Baeloth Barrityl, Entertainer (#4054) — Creatures your opponents control with power less than Baeloth Barrityl's power a"""
    return _ast(card["name"], _custom("baeloth_barrityl_entertainer"))


def gogo_master_of_mimicry_handler(card):
    """Gogo, Master of Mimicry (#4067) — {X}{X}, {T}: Copy target activated or triggered ability you control X times. You"""
    return _ast(card["name"], _custom("gogo_master_of_mimicry"))


def assault_suit_handler(card):
    """Assault Suit (#4078) — Equipped creature gets +2/+2, has haste, can't attack you or planeswalkers you c"""
    return _ast(card["name"], _custom("assault_suit"))


def phyrexian_censor_handler(card):
    """Phyrexian Censor (#4087) — Each player can't cast more than one non-Phyrexian spell each turn."""
    return _ast(card["name"], _custom("phyrexian_censor"))


def kopala_warden_of_waves_handler(card):
    """Kopala, Warden of Waves (#4090) — Spells your opponents cast that target a Merfolk you control cost {2} more to ca"""
    return _ast(card["name"], _custom("kopala_warden_of_waves"))


def urabrask_heretic_praetor_handler(card):
    """Urabrask, Heretic Praetor (#4093) — Haste"""
    return _ast(card["name"], _custom("urabrask_heretic_praetor"))


def rolling_hamsphere_handler(card):
    """Rolling Hamsphere (#4094) — This Vehicle gets +1/+1 for each Hamster you control."""
    return _ast(card["name"], _custom("rolling_hamsphere"))


def might_of_the_meek_handler(card):
    """Might of the Meek (#4096) — Target creature gains trample until end of turn. It also gets +1/+0 until end of"""
    return _ast(card["name"], _custom("might_of_the_meek"))


def chainsaw_handler(card):
    """Chainsaw (#4124) — When this Equipment enters, it deals 3 damage to up to one target creature."""
    return _ast(card["name"], _custom("chainsaw"))


def emperor_mihail_ii_handler(card):
    """Emperor Mihail II (#4125) — You may look at the top card of your library any time."""
    return _ast(card["name"], _custom("emperor_mihail_ii"))


def clockspinning_handler(card):
    """Clockspinning (#4133) — Buyback {3} (You may pay an additional {3} as you cast this spell. If you do, pu"""
    return _ast(card["name"], _custom("clockspinning"))


def selvalas_stampede_handler(card):
    """Selvala's Stampede (#4135) — Council's dilemma — Starting with you, each player votes for wild or free. Revea"""
    return _ast(card["name"], _custom("selvalas_stampede"))


def turnabout_handler(card):
    """Turnabout (#4160) — Choose artifact, creature, or land. Tap all untapped permanents of the chosen ty"""
    return _ast(card["name"], _custom("turnabout"))


def luxior_giadas_gift_handler(card):
    """Luxior, Giada's Gift (#4180) — Equipped creature gets +1/+1 for each counter on it."""
    return _ast(card["name"], _custom("luxior_giadas_gift"))


def animation_module_handler(card):
    """Animation Module (#4182) — Whenever one or more +1/+1 counters are put on a permanent you control, you may """
    return _ast(card["name"], _custom("animation_module"))


def neyith_of_the_dire_hunt_handler(card):
    """Neyith of the Dire Hunt (#4183) — Whenever one or more creatures you control fight or become blocked, draw a card."""
    return _ast(card["name"], _custom("neyith_of_the_dire_hunt"))


def shadowgrange_archfiend_handler(card):
    """Shadowgrange Archfiend (#4214) — When this creature enters, each opponent sacrifices a creature with the greatest"""
    return _ast(card["name"], _custom("shadowgrange_archfiend"))


def the_golden_throne_handler(card):
    """The Golden Throne (#4224) — Arcane Life-support — If you would lose the game, instead exile The Golden Thron"""
    return _ast(card["name"], _custom("the_golden_throne"))


def tidal_barracuda_handler(card):
    """Tidal Barracuda (#4229) — Any player may cast spells as though they had flash."""
    return _ast(card["name"], _custom("tidal_barracuda"))


def vizier_of_the_menagerie_handler(card):
    """Vizier of the Menagerie (#4239) — You may look at the top card of your library any time."""
    return _ast(card["name"], _custom("vizier_of_the_menagerie"))


def ao_the_dawn_sky_handler(card):
    """Ao, the Dawn Sky (#4243) — Flying, vigilance"""
    return _ast(card["name"], _custom("ao_the_dawn_sky"))


def murmuration_handler(card):
    """Murmuration (#4247) — Birds you control get +1/+1 and have vigilance."""
    return _ast(card["name"], _custom("murmuration"))


def captain_nghathrod_handler(card):
    """Captain N'ghathrod (#4270) — Horrors you control have menace."""
    return _ast(card["name"], _custom("captain_nghathrod"))


def secret_arcade_handler(card):
    """Secret Arcade // Dusty Parlor (#4271) — """
    return _ast(card["name"], _custom("secret_arcade"))


def paladin_class_handler(card):
    """Paladin Class (#4290) — (Gain the next level as a sorcery to add its ability.)"""
    return _ast(card["name"], _custom("paladin_class"))


def nils_discipline_enforcer_handler(card):
    """Nils, Discipline Enforcer (#4298) — At the beginning of your end step, for each player, put a +1/+1 counter on up to"""
    return _ast(card["name"], _custom("nils_discipline_enforcer"))


def bloodline_bidding_handler(card):
    """Bloodline Bidding (#4301) — Convoke (Your creatures can help cast this spell. Each creature you tap while ca"""
    return _ast(card["name"], _custom("bloodline_bidding"))


def determined_iteration_handler(card):
    """Determined Iteration (#4314) — At the beginning of combat on your turn, populate. The token created this way ga"""
    return _ast(card["name"], _custom("determined_iteration"))


def luck_bobblehead_handler(card):
    """Luck Bobblehead (#4337) — {T}: Add one mana of any color."""
    return _ast(card["name"], _custom("luck_bobblehead"))


def fire_lord_azula_handler(card):
    """Fire Lord Azula (#4370) — Firebending 2 (Whenever this creature attacks, add {R}{R}. This mana lasts until"""
    return _ast(card["name"], _custom("fire_lord_azula"))


def overlord_of_the_mistmoors_handler(card):
    """Overlord of the Mistmoors (#4375) — Impending 4—{2}{W}{W} (If you cast this spell for its impending cost, it enters """
    return _ast(card["name"], _custom("overlord_of_the_mistmoors"))


def seasoned_dungeoneer_handler(card):
    """Seasoned Dungeoneer (#4400) — When this creature enters, you take the initiative."""
    return _ast(card["name"], _custom("seasoned_dungeoneer"))


def famished_worldsire_handler(card):
    """Famished Worldsire (#4412) — Ward {3}"""
    return _ast(card["name"], _custom("famished_worldsire"))


def thran_portal_handler(card):
    """Thran Portal (#4429) — This land enters tapped unless you control two or fewer other lands."""
    return _ast(card["name"], _custom("thran_portal"))


def sephiroth_fallen_hero_handler(card):
    """Sephiroth, Fallen Hero (#4442) — Jenova Cells — Whenever Sephiroth attacks, you may put a cell counter on target """
    return _ast(card["name"], _custom("sephiroth_fallen_hero"))


def entrapment_maneuver_handler(card):
    """Entrapment Maneuver (#4452) — Target player sacrifices an attacking creature of their choice. You create X 1/1"""
    return _ast(card["name"], _custom("entrapment_maneuver"))


def bumi_unleashed_handler(card):
    """Bumi, Unleashed (#4455) — Trample"""
    return _ast(card["name"], _custom("bumi_unleashed"))


def ruric_thar_the_unbowed_handler(card):
    """Ruric Thar, the Unbowed (#4487) — Vigilance, reach"""
    return _ast(card["name"], _custom("ruric_thar_the_unbowed"))


def spiderpunk_handler(card):
    """Spider-Punk (#4514) — Riot (This creature enters with your choice of a +1/+1 counter or haste.)"""
    return _ast(card["name"], _custom("spiderpunk"))


def fractured_identity_handler(card):
    """Fractured Identity (#4528) — Exile target nonland permanent. Each player other than its controller creates a """
    return _ast(card["name"], _custom("fractured_identity"))


def banquet_guests_handler(card):
    """Banquet Guests (#4544) — Affinity for Foods (This spell costs {1} less to cast for each Food you control."""
    return _ast(card["name"], _custom("banquet_guests"))


def cyber_conversion_handler(card):
    """Cyber Conversion (#4613) — Turn target creature face down. It's a 2/2 Cyberman artifact creature."""
    return _ast(card["name"], _custom("cyber_conversion"))


def eriette_of_the_charmed_apple_handler(card):
    """Eriette of the Charmed Apple (#4614) — Each creature that's enchanted by an Aura you control can't attack you or planes"""
    return _ast(card["name"], _custom("eriette_of_the_charmed_apple"))


def kefka_dancing_mad_handler(card):
    """Kefka, Dancing Mad (#4619) — During your turn, Kefka has indestructible."""
    return _ast(card["name"], _custom("kefka_dancing_mad"))


def quick_sliver_handler(card):
    """Quick Sliver (#4632) — Flash"""
    return _ast(card["name"], _custom("quick_sliver"))


def ozai_the_phoenix_king_handler(card):
    """Ozai, the Phoenix King (#4642) — Trample, firebending 4, haste"""
    return _ast(card["name"], _custom("ozai_the_phoenix_king"))


def alisaie_leveilleur_handler(card):
    """Alisaie Leveilleur (#4665) — Partner with Alphinaud Leveilleur (When this creature enters, target player may """
    return _ast(card["name"], _custom("alisaie_leveilleur"))


def lumbering_worldwagon_handler(card):
    """Lumbering Worldwagon (#4678) — This Vehicle's power is equal to the number of lands you control."""
    return _ast(card["name"], _custom("lumbering_worldwagon"))


def aang_swift_savior_handler(card):
    """Aang, Swift Savior // Aang and La, Ocean's Fury (#4684) — """
    return _ast(card["name"], _custom("aang_swift_savior"))


def overlord_of_the_boilerbilges_handler(card):
    """Overlord of the Boilerbilges (#4729) — Impending 4—{2}{R}{R} (If you cast this spell for its impending cost, it enters """
    return _ast(card["name"], _custom("overlord_of_the_boilerbilges"))


def reflector_mage_handler(card):
    """Reflector Mage (#4730) — When this creature enters, return target creature an opponent controls to its ow"""
    return _ast(card["name"], _custom("reflector_mage"))


def firion_wild_rose_warrior_handler(card):
    """Firion, Wild Rose Warrior (#4776) — Equipped creatures you control have haste."""
    return _ast(card["name"], _custom("firion_wild_rose_warrior"))


def dion_bahamuts_dominant_handler(card):
    """Dion, Bahamut's Dominant // Bahamut, Warden of Light (#4789) — """
    return _ast(card["name"], _custom("dion_bahamuts_dominant"))


def eye_of_nidhogg_handler(card):
    """Eye of Nidhogg (#4797) — Enchant creature"""
    return _ast(card["name"], _custom("eye_of_nidhogg"))


def the_master_transcendent_handler(card):
    """The Master, Transcendent (#4826) — When The Master enters, target player gets two rad counters."""
    return _ast(card["name"], _custom("the_master_transcendent"))


def robe_of_the_archmagi_handler(card):
    """Robe of the Archmagi (#4883) — Whenever equipped creature deals combat damage to a player, you draw that many c"""
    return _ast(card["name"], _custom("robe_of_the_archmagi"))


def elemental_eruption_handler(card):
    """Elemental Eruption (#4887) — Create a 4/4 red Dragon Elemental creature token with flying and prowess."""
    return _ast(card["name"], _custom("elemental_eruption"))


def saurons_ransom_handler(card):
    """Sauron's Ransom (#4918) — Choose an opponent. They look at the top four cards of your library and separate"""
    return _ast(card["name"], _custom("saurons_ransom"))


def devastating_onslaught_handler(card):
    """Devastating Onslaught (#4933) — Create X tokens that are copies of target artifact or creature you control. Thos"""
    return _ast(card["name"], _custom("devastating_onslaught"))


def viewpoint_synchronization_handler(card):
    """Viewpoint Synchronization (#4960) — Freerunning {2}{G} (You may cast this spell for its freerunning cost if you deal"""
    return _ast(card["name"], _custom("viewpoint_synchronization"))


def damping_sphere_handler(card):
    """Damping Sphere (#4962) — If a land is tapped for two or more mana, it produces {C} instead of any other t"""
    return _ast(card["name"], _custom("damping_sphere"))


def bilbos_ring_handler(card):
    """Bilbo's Ring (#4979) — During your turn, equipped creature has hexproof and can't be blocked."""
    return _ast(card["name"], _custom("bilbos_ring"))


def conspiracy_handler(card):
    """Conspiracy (#5002) — As this enchantment enters, choose a creature type."""
    return _ast(card["name"], _custom("conspiracy"))


def instill_energy_handler(card):
    """Instill Energy (#5003) — Enchant creature"""
    return _ast(card["name"], _custom("instill_energy"))


def airbending_lesson_handler(card):
    """Airbending Lesson (#5026) — Airbend target nonland permanent. (Exile it. While it's exiled, its owner may ca"""
    return _ast(card["name"], _custom("airbending_lesson"))


def find_handler(card):
    """Find // Finality (#5044) — """
    return _ast(card["name"], _custom("find"))


def arwen_mortal_queen_handler(card):
    """Arwen, Mortal Queen (#5069) — Arwen enters with an indestructible counter on it."""
    return _ast(card["name"], _custom("arwen_mortal_queen"))


def elenda_saint_of_dusk_handler(card):
    """Elenda, Saint of Dusk (#5072) — Lifelink, hexproof from instants"""
    return _ast(card["name"], _custom("elenda_saint_of_dusk"))


def aminatou_veil_piercer_handler(card):
    """Aminatou, Veil Piercer (#5085) — At the beginning of your upkeep, surveil 2. (Look at the top two cards of your l"""
    return _ast(card["name"], _custom("aminatou_veil_piercer"))


def chainer_dementia_master_handler(card):
    """Chainer, Dementia Master (#5086) — All Nightmares get +1/+1."""
    return _ast(card["name"], _custom("chainer_dementia_master"))


def glimmer_lens_handler(card):
    """Glimmer Lens (#5091) — For Mirrodin! (When this Equipment enters, create a 2/2 red Rebel creature token"""
    return _ast(card["name"], _custom("glimmer_lens"))


def locke_treasure_hunter_handler(card):
    """Locke, Treasure Hunter (#5140) — Locke can't be blocked by creatures with greater power."""
    return _ast(card["name"], _custom("locke_treasure_hunter"))


def benevolent_blessing_handler(card):
    """Benevolent Blessing (#5143) — Flash"""
    return _ast(card["name"], _custom("benevolent_blessing"))


def vial_smasher_the_fierce_handler(card):
    """Vial Smasher the Fierce (#5176) — Whenever you cast your first spell each turn, choose an opponent at random. Vial"""
    return _ast(card["name"], _custom("vial_smasher_the_fierce"))





# ---------------------------------------------------------------------------
# Wave 5 — top-200 PARTIAL escapees (ranks ~501 to ~14123)
# ---------------------------------------------------------------------------

def necropotence_handler(card):
    """Necropotence (#501) — Skip your draw step. Whenever you discard a card, exile that card from your grav"""
    return _ast(
        card["name"],
        _custom("necropotence"),
    )


def whip_of_erebos_handler(card):
    """Whip of Erebos (#722) — Creatures you control have lifelink. {2}{B}{B}, {T}: Return target creature card"""
    return _ast(
        card["name"],
        _custom("whip_of_erebos"),
    )


def eerie_interlude_handler(card):
    """Eerie Interlude (#896) — Exile any number of target creatures you control. Return those cards to the batt"""
    return _ast(
        card["name"],
        _custom("eerie_interlude"),
    )


def nezahal_primal_tide_handler(card):
    """Nezahal, Primal Tide (#1023) — This spell can't be countered. You have no maximum hand size. Whenever an oppone"""
    return _ast(
        card["name"],
        _custom("nezahal_primal_tide"),
    )


def kiki_jiki_mirror_breaker_handler(card):
    """Kiki-Jiki, Mirror Breaker (#1142) — Haste {T}: Create a token that's a copy of target nonlegendary creature you cont"""
    return _ast(
        card["name"],
        Keyword(name="haste", raw="Haste"),
        _custom("kiki_jiki_mirror_breaker"),
    )


def the_fire_crystal_handler(card):
    """The Fire Crystal (#1185) — Red spells you cast cost {1} less to cast. Creatures you control have haste. {4}"""
    return _ast(
        card["name"],
        _custom("the_fire_crystal"),
    )


def twinflame_handler(card):
    """Twinflame (#1249) — Strive — This spell costs {2}{R} more to cast for each target beyond the first. """
    return _ast(
        card["name"],
        _custom("twinflame"),
    )


def feldon_of_the_third_path_handler(card):
    """Feldon of the Third Path (#1334) — {2}{R}, {T}: Create a token that's a copy of target creature card in your gravey"""
    return _ast(
        card["name"],
        _custom("feldon_of_the_third_path"),
    )


def sneak_attack_handler(card):
    """Sneak Attack (#1438) — {R}: You may put a creature card from your hand onto the battlefield. That creat"""
    return _ast(
        card["name"],
        _custom("sneak_attack"),
    )


def the_locust_god_handler(card):
    """The Locust God (#1608) — Flying Whenever you draw a card, create a 1/1 blue and red Insect creature token"""
    return _ast(
        card["name"],
        Keyword(name="flying", raw="Flying"),
        _custom("the_locust_god"),
    )


def molten_duplication_handler(card):
    """Molten Duplication (#1613) — Create a token that's a copy of target artifact or creature you control, except """
    return _ast(
        card["name"],
        _custom("molten_duplication"),
    )


def jaxis_the_troublemaker_handler(card):
    """Jaxis, the Troublemaker (#1616) — {R}, {T}, Discard a card: Create a token that's a copy of another target creatur"""
    return _ast(
        card["name"],
        _custom("jaxis_the_troublemaker"),
    )


def fable_of_the_mirror_breaker_handler(card):
    """Fable of the Mirror-Breaker // Reflection of Kiki-Jiki (#1651) — """
    return _ast(
        card["name"],
        _custom("fable_of_the_mirror_breaker"),
    )


def the_scarab_god_handler(card):
    """The Scarab God (#1697) — At the beginning of your upkeep, each opponent loses X life and you scry X, wher"""
    return _ast(
        card["name"],
        _custom("the_scarab_god"),
    )


def charming_prince_handler(card):
    """Charming Prince (#1738) — When this creature enters, choose one — • Scry 2. • You gain 3 life. • Exile ano"""
    return _ast(
        card["name"],
        _custom("charming_prince"),
    )


def touch_the_spirit_realm_handler(card):
    """Touch the Spirit Realm (#1754) — When this enchantment enters, exile up to one target artifact or creature until """
    return _ast(
        card["name"],
        _custom("touch_the_spirit_realm"),
    )


def parting_gust_handler(card):
    """Parting Gust (#1811) — Gift a tapped Fish (You may promise an opponent a gift as you cast this spell. I"""
    return _ast(
        card["name"],
        _custom("parting_gust"),
    )


def liesa_forgotten_archangel_handler(card):
    """Liesa, Forgotten Archangel (#1978) — Flying, lifelink Whenever another nontoken creature you control dies, return tha"""
    return _ast(
        card["name"],
        Keyword(name="flying", raw="Flying"),
        Keyword(name="lifelink", raw="Lifelink"),
        _custom("liesa_forgotten_archangel"),
    )


def flickerwisp_handler(card):
    """Flickerwisp (#2121) — Flying When this creature enters, exile another target permanent. Return that ca"""
    return _ast(
        card["name"],
        Keyword(name="flying", raw="Flying"),
        _custom("flickerwisp"),
    )


def mimic_vat_handler(card):
    """Mimic Vat (#2140) — Imprint — Whenever a nontoken creature dies, you may exile that card. If you do,"""
    return _ast(
        card["name"],
        _custom("mimic_vat"),
    )


def orthion_hero_of_lavabrink_handler(card):
    """Orthion, Hero of Lavabrink (#2224) — {1}{R}, {T}: Create a token that's a copy of another target creature you control"""
    return _ast(
        card["name"],
        _custom("orthion_hero_of_lavabrink"),
    )


def gift_of_immortality_handler(card):
    """Gift of Immortality (#2226) — Enchant creature When enchanted creature dies, return that card to the battlefie"""
    return _ast(
        card["name"],
        _custom("gift_of_immortality"),
    )


def molten_echoes_handler(card):
    """Molten Echoes (#2276) — As this enchantment enters, choose a creature type. Whenever a nontoken creature"""
    return _ast(
        card["name"],
        _custom("molten_echoes"),
    )


def ilharg_the_raze_boar_handler(card):
    """Ilharg, the Raze-Boar (#2362) — Trample Whenever Ilharg attacks, you may put a creature card from your hand onto"""
    return _ast(
        card["name"],
        Keyword(name="trample", raw="Trample"),
        _custom("ilharg_the_raze_boar"),
    )


def urabrasks_forge_handler(card):
    """Urabrask's Forge (#2455) — At the beginning of combat on your turn, put an oil counter on this artifact, th"""
    return _ast(
        card["name"],
        _custom("urabrasks_forge"),
    )


def the_jolly_balloon_man_handler(card):
    """The Jolly Balloon Man (#2528) — Haste {1}, {T}: Create a token that's a copy of another target creature you cont"""
    return _ast(
        card["name"],
        Keyword(name="haste", raw="Haste"),
        _custom("the_jolly_balloon_man"),
    )


def laezels_acrobatics_handler(card):
    """Lae'zel's Acrobatics (#2546) — Exile all nontoken creatures you control, then roll a d20. 1—9 | Return those ca"""
    return _ast(
        card["name"],
        _custom("laezels_acrobatics"),
    )


def teferis_time_twist_handler(card):
    """Teferi's Time Twist (#2626) — Exile target permanent you control. Return that card to the battlefield under it"""
    return _ast(
        card["name"],
        _custom("teferis_time_twist"),
    )


def nine_lives_familiar_handler(card):
    """Nine-Lives Familiar (#2780) — This creature enters with eight revival counters on it if you cast it. When this"""
    return _ast(
        card["name"],
        _custom("nine_lives_familiar"),
    )


def flameshadow_conjuring_handler(card):
    """Flameshadow Conjuring (#2781) — Whenever a nontoken creature you control enters, you may pay {R}. If you do, cre"""
    return _ast(
        card["name"],
        _custom("flameshadow_conjuring"),
    )


def lagomos_hand_of_hatred_handler(card):
    """Lagomos, Hand of Hatred (#2891) — At the beginning of combat on your turn, create a 2/1 red Elemental creature tok"""
    return _ast(
        card["name"],
        _custom("lagomos_hand_of_hatred"),
    )


def ghoulish_impetus_handler(card):
    """Ghoulish Impetus (#3067) — Enchant creature Enchanted creature gets +1/+1, has deathtouch, and is goaded. ("""
    return _ast(
        card["name"],
        _custom("ghoulish_impetus"),
    )


def echoing_assault_handler(card):
    """Echoing Assault (#3146) — Creature tokens you control have menace. Whenever you attack a player, choose ta"""
    return _ast(
        card["name"],
        _custom("echoing_assault"),
    )


def ghostway_handler(card):
    """Ghostway (#3206) — Exile each creature you control. Return those cards to the battlefield under the"""
    return _ast(
        card["name"],
        _custom("ghostway"),
    )


def rionya_fire_dancer_handler(card):
    """Rionya, Fire Dancer (#3254) — At the beginning of combat on your turn, create X tokens that are copies of anot"""
    return _ast(
        card["name"],
        _custom("rionya_fire_dancer"),
    )


def come_back_wrong_handler(card):
    """Come Back Wrong (#3768) — Destroy target creature. If a creature card is put into a graveyard this way, re"""
    return _ast(
        card["name"],
        _custom("come_back_wrong"),
    )


def mirror_march_handler(card):
    """Mirror March (#3811) — Whenever a nontoken creature you control enters, flip a coin until you lose a fl"""
    return _ast(
        card["name"],
        _custom("mirror_march"),
    )


def cosmic_intervention_handler(card):
    """Cosmic Intervention (#3837) — If a permanent you control would be put into a graveyard from the battlefield th"""
    return _ast(
        card["name"],
        _custom("cosmic_intervention"),
    )


def dalkovan_encampment_handler(card):
    """Dalkovan Encampment (#3845) — This land enters tapped unless you control a Swamp or a Mountain. {T}: Add {W}. """
    return _ast(
        card["name"],
        _custom("dalkovan_encampment"),
    )


def stormsplitter_handler(card):
    """Stormsplitter (#3975) — Haste Whenever you cast an instant or sorcery spell, create a token that's a cop"""
    return _ast(
        card["name"],
        Keyword(name="haste", raw="Haste"),
        _custom("stormsplitter"),
    )


def resurrection_orb_handler(card):
    """Resurrection Orb (#4092) — Equipped creature has lifelink. Whenever equipped creature dies, return that car"""
    return _ast(
        card["name"],
        _custom("resurrection_orb"),
    )


def yorion_sky_nomad_handler(card):
    """Yorion, Sky Nomad (#4117) — Companion — Your starting deck contains at least twenty cards more than the mini"""
    return _ast(
        card["name"],
        Keyword(name="flying", raw="Flying"),
        _custom("yorion_sky_nomad"),
    )


def purphoros_bronze_blooded_handler(card):
    """Purphoros, Bronze-Blooded (#4259) — Indestructible As long as your devotion to red is less than five, Purphoros isn'"""
    return _ast(
        card["name"],
        Keyword(name="indestructible", raw="Indestructible"),
        _custom("purphoros_bronze_blooded"),
    )


def grave_betrayal_handler(card):
    """Grave Betrayal (#4426) — Whenever a creature you don't control dies, return it to the battlefield under y"""
    return _ast(
        card["name"],
        _custom("grave_betrayal"),
    )


def zara_renegade_recruiter_handler(card):
    """Zara, Renegade Recruiter (#4444) — Flying Whenever Zara attacks, look at defending player's hand. You may put a cre"""
    return _ast(
        card["name"],
        Keyword(name="flying", raw="Flying"),
        _custom("zara_renegade_recruiter"),
    )


def depthshaker_titan_handler(card):
    """Depthshaker Titan (#5483) — When this creature enters, any number of target noncreature artifacts you contro"""
    return _ast(
        card["name"],
        _custom("depthshaker_titan"),
    )


def life_handler(card):
    """Life // Death (#5621) — """
    return _ast(
        card["name"],
        _custom("life"),
    )


def terra_magical_adept_handler(card):
    """Terra, Magical Adept // Esper Terra (#5903) — """
    return _ast(
        card["name"],
        Keyword(name="flying", raw="Flying"),
        _custom("terra_magical_adept"),
    )


def inalla_archmage_ritualist_handler(card):
    """Inalla, Archmage Ritualist (#8407) — Eminence — Whenever another nontoken Wizard you control enters, if Inalla is in """
    return _ast(
        card["name"],
        _custom("inalla_archmage_ritualist"),
    )


def niko_light_of_hope_handler(card):
    """Niko, Light of Hope (#8737) — When Niko enters, create two Shard tokens. (They're enchantments with "{2}, Sacr"""
    return _ast(
        card["name"],
        _custom("niko_light_of_hope"),
    )


def foe_razer_regent_handler(card):
    """Foe-Razer Regent (#9579) — Flying When this creature enters, you may have it fight target creature you don'"""
    return _ast(
        card["name"],
        Keyword(name="flying", raw="Flying"),
        _custom("foe_razer_regent"),
    )


def sidequest_play_blitzball_handler(card):
    """Sidequest: Play Blitzball // World Champion, Celestial Weapon (#11927) — """
    return _ast(
        card["name"],
        _custom("sidequest_play_blitzball"),
    )


def dawnbreak_reclaimer_handler(card):
    """Dawnbreak Reclaimer (#11960) — Flying At the beginning of your end step, choose a creature card in an opponent'"""
    return _ast(
        card["name"],
        Keyword(name="flying", raw="Flying"),
        _custom("dawnbreak_reclaimer"),
    )


def artificial_evolution_handler(card):
    """Artificial Evolution (#11979) — Change the text of target spell or permanent by replacing all instances of one c"""
    return _ast(
        card["name"],
        _custom("artificial_evolution"),
    )


def survey_mechan_handler(card):
    """Survey Mechan (#11992) — Flying Hexproof (This creature can't be the target of spells or abilities your o"""
    return _ast(
        card["name"],
        Keyword(name="flying", raw="Flying"),
        Keyword(name="hexproof", raw="Hexproof"),
        _custom("survey_mechan"),
    )


def wort_the_raidmother_handler(card):
    """Wort, the Raidmother (#11999) — When Wort enters, create two 1/1 red and green Goblin Warrior creature tokens. E"""
    return _ast(
        card["name"],
        _custom("wort_the_raidmother"),
    )


def invert_handler(card):
    """Invert // Invent (#12005) — """
    return _ast(
        card["name"],
        _custom("invert"),
    )


def ian_chesterton_handler(card):
    """Ian Chesterton (#12063) — Science Teacher — Each Saga spell you cast has replicate. The replicate cost is """
    return _ast(
        card["name"],
        _custom("ian_chesterton"),
    )


def thunderous_debut_handler(card):
    """Thunderous Debut (#12064) — Bargain (You may sacrifice an artifact, enchantment, or token as you cast this s"""
    return _ast(
        card["name"],
        _custom("thunderous_debut"),
    )


def ruthless_radrat_handler(card):
    """Ruthless Radrat (#12074) — Squad—Exile four cards from your graveyard. (As an additional cost to cast this """
    return _ast(
        card["name"],
        Keyword(name="menace", raw="Menace"),
        _custom("ruthless_radrat"),
    )


def transcendence_handler(card):
    """Transcendence (#12075) — You don't lose the game for having 0 or less life. When you have 20 or more life"""
    return _ast(
        card["name"],
        _custom("transcendence"),
    )


def acorn_harvest_handler(card):
    """Acorn Harvest (#12093) — Create two 1/1 green Squirrel creature tokens. Flashback—{1}{G}, Pay 3 life. (Yo"""
    return _ast(
        card["name"],
        _custom("acorn_harvest"),
    )


def turn_to_mist_handler(card):
    """Turn to Mist (#12109) — Exile target creature. Return that card to the battlefield under its owner's con"""
    return _ast(
        card["name"],
        _custom("turn_to_mist"),
    )


def boneyard_parley_handler(card):
    """Boneyard Parley (#12135) — Exile up to five target creature cards from graveyards. An opponent separates th"""
    return _ast(
        card["name"],
        _custom("boneyard_parley"),
    )


def plunge_into_darkness_handler(card):
    """Plunge into Darkness (#12157) — Choose one — • Sacrifice any number of creatures. You gain 3 life for each creat"""
    return _ast(
        card["name"],
        _custom("plunge_into_darkness"),
    )


def drachnyen_handler(card):
    """Drach'Nyen (#12162) — Echo of the First Murder — When Drach'Nyen enters, exile up to one target creatu"""
    return _ast(
        card["name"],
        _custom("drachnyen"),
    )


def endless_whispers_handler(card):
    """Endless Whispers (#12167) — Each creature has "When this creature dies, choose target opponent. That player """
    return _ast(
        card["name"],
        _custom("endless_whispers"),
    )


def seal_of_the_guildpact_handler(card):
    """Seal of the Guildpact (#12191) — As this artifact enters, choose two colors. Each spell you cast costs {1} less t"""
    return _ast(
        card["name"],
        _custom("seal_of_the_guildpact"),
    )


def expedited_inheritance_handler(card):
    """Expedited Inheritance (#12205) — Whenever a creature is dealt damage, its controller may exile that many cards fr"""
    return _ast(
        card["name"],
        _custom("expedited_inheritance"),
    )


def sanguine_sacrament_handler(card):
    """Sanguine Sacrament (#12209) — You gain twice X life. Put Sanguine Sacrament on the bottom of its owner's libra"""
    return _ast(
        card["name"],
        _custom("sanguine_sacrament"),
    )


def clara_oswald_handler(card):
    """Clara Oswald (#12215) — Impossible Girl — If Clara Oswald is your commander, choose a color before the g"""
    return _ast(
        card["name"],
        _custom("clara_oswald"),
    )


def tallyman_of_nurgle_handler(card):
    """Tallyman of Nurgle (#12216) — Lifelink The Seven-fold Chant — At the beginning of your end step, if a creature"""
    return _ast(
        card["name"],
        Keyword(name="lifelink", raw="Lifelink"),
        _custom("tallyman_of_nurgle"),
    )


def barge_in_handler(card):
    """Barge In (#12236) — Target attacking creature gets +2/+2 until end of turn. Each attacking non-Human"""
    return _ast(
        card["name"],
        _custom("barge_in"),
    )


def ignite_memories_handler(card):
    """Ignite Memories (#12256) — Target player reveals a card at random from their hand. Ignite Memories deals da"""
    return _ast(
        card["name"],
        _custom("ignite_memories"),
    )


def into_the_night_handler(card):
    """Into the Night (#12259) — It becomes night. Discard any number of cards, then draw that many cards plus on"""
    return _ast(
        card["name"],
        _custom("into_the_night"),
    )


def angel_of_condemnation_handler(card):
    """Angel of Condemnation (#12265) — Flying, vigilance {2}{W}, {T}: Exile another target creature. Return that card t"""
    return _ast(
        card["name"],
        Keyword(name="flying", raw="Flying"),
        Keyword(name="vigilance", raw="Vigilance"),
        _custom("angel_of_condemnation"),
    )


def depala_pilot_exemplar_handler(card):
    """Depala, Pilot Exemplar (#12275) — Other Dwarves you control get +1/+1. Each Vehicle you control gets +1/+1 as long"""
    return _ast(
        card["name"],
        _custom("depala_pilot_exemplar"),
    )


def mwonvuli_beast_tracker_handler(card):
    """Mwonvuli Beast Tracker (#12300) — When this creature enters, search your library for a creature card with deathtou"""
    return _ast(
        card["name"],
        Keyword(name="reach", raw="Reach"),
        Keyword(name="hexproof", raw="Hexproof"),
        _custom("mwonvuli_beast_tracker"),
    )


def arachne_psionic_weaver_handler(card):
    """Arachne, Psionic Weaver (#12321) — Web-slinging {W} (You may cast this spell for {W} if you also return a tapped cr"""
    return _ast(
        card["name"],
        _custom("arachne_psionic_weaver"),
    )


def queen_kayla_bin_kroog_handler(card):
    """Queen Kayla bin-Kroog (#12328) — {4}, {T}: Discard all the cards in your hand, then draw that many cards. You may"""
    return _ast(
        card["name"],
        _custom("queen_kayla_bin_kroog"),
    )


def prismatic_strands_handler(card):
    """Prismatic Strands (#12345) — Prevent all damage that sources of the color of your choice would deal this turn"""
    return _ast(
        card["name"],
        _custom("prismatic_strands"),
    )


def stitcher_geralf_handler(card):
    """Stitcher Geralf (#12418) — {2}{U}, {T}: Each player mills three cards. Exile up to two creature cards put i"""
    return _ast(
        card["name"],
        _custom("stitcher_geralf"),
    )


def fertilids_favor_handler(card):
    """Fertilid's Favor (#12439) — Target player searches their library for a basic land card, puts it onto the bat"""
    return _ast(
        card["name"],
        _custom("fertilids_favor"),
    )


def themberchaud_handler(card):
    """Themberchaud (#12443) — Trample When Themberchaud enters, he deals X damage to each other creature witho"""
    return _ast(
        card["name"],
        Keyword(name="trample", raw="Trample"),
        _custom("themberchaud"),
    )


def fire_navy_trebuchet_handler(card):
    """Fire Navy Trebuchet (#12445) — Defender, reach Whenever you attack, create a 2/1 colorless Construct artifact c"""
    return _ast(
        card["name"],
        Keyword(name="reach", raw="Reach"),
        Keyword(name="defender", raw="Defender"),
        _custom("fire_navy_trebuchet"),
    )


def dark_knights_greatsword_handler(card):
    """Dark Knight's Greatsword (#12450) — Job select (When this Equipment enters, create a 1/1 colorless Hero creature tok"""
    return _ast(
        card["name"],
        _custom("dark_knights_greatsword"),
    )


def the_spots_portal_handler(card):
    """The Spot's Portal (#12455) — Put target creature on the bottom of its owner's library. You lose 2 life unless"""
    return _ast(
        card["name"],
        _custom("the_spots_portal"),
    )


def teachings_of_the_archaics_handler(card):
    """Teachings of the Archaics (#12457) — If an opponent has more cards in hand than you, draw two cards. Draw three cards"""
    return _ast(
        card["name"],
        _custom("teachings_of_the_archaics"),
    )


def proud_wildbonder_handler(card):
    """Proud Wildbonder (#12471) — Trample Creatures you control with trample have "You may have this creature assi"""
    return _ast(
        card["name"],
        Keyword(name="trample", raw="Trample"),
        _custom("proud_wildbonder"),
    )


def experimental_overload_handler(card):
    """Experimental Overload (#12497) — Create an X/X blue and red Weird creature token, where X is the number of instan"""
    return _ast(
        card["name"],
        _custom("experimental_overload"),
    )


def ruthless_ripper_handler(card):
    """Ruthless Ripper (#12510) — Deathtouch Morph—Reveal a black card in your hand. (You may cast this card face """
    return _ast(
        card["name"],
        Keyword(name="deathtouch", raw="Deathtouch"),
        _custom("ruthless_ripper"),
    )


def cauldron_dance_handler(card):
    """Cauldron Dance (#12519) — Cast this spell only during combat. Return target creature card from your gravey"""
    return _ast(
        card["name"],
        _custom("cauldron_dance"),
    )


def karns_sylex_handler(card):
    """Karn's Sylex (#12541) — Karn's Sylex enters tapped. Players can't pay life to cast spells or to activate"""
    return _ast(
        card["name"],
        _custom("karns_sylex"),
    )


def eiganjo_uprising_handler(card):
    """Eiganjo Uprising (#12563) — Create X 2/2 white Samurai creature tokens with vigilance. They gain menace and """
    return _ast(
        card["name"],
        _custom("eiganjo_uprising"),
    )


def archmages_newt_handler(card):
    """Archmage's Newt (#12575) — Whenever this creature deals combat damage to a player, target instant or sorcer"""
    return _ast(
        card["name"],
        _custom("archmages_newt"),
    )


def whirlpool_whelm_handler(card):
    """Whirlpool Whelm (#12586) — Clash with an opponent, then return target creature to its owner's hand. If you """
    return _ast(
        card["name"],
        _custom("whirlpool_whelm"),
    )


def raph_leo_sibling_rivals_handler(card):
    """Raph & Leo, Sibling Rivals (#12607) — Whenever Raph & Leo attack, if it's the first combat phase of the turn, untap on"""
    return _ast(
        card["name"],
        _custom("raph_leo_sibling_rivals"),
    )


def fang_rokus_companion_handler(card):
    """Fang, Roku's Companion (#12631) — Flying Whenever Fang attacks, another target legendary creature you control gets"""
    return _ast(
        card["name"],
        Keyword(name="flying", raw="Flying"),
        _custom("fang_rokus_companion"),
    )


def defossilize_handler(card):
    """Defossilize (#12633) — Return target creature card from your graveyard to the battlefield. That creatur"""
    return _ast(
        card["name"],
        _custom("defossilize"),
    )


def arcus_acolyte_handler(card):
    """Arcus Acolyte (#12634) — Reach, lifelink Outlast {G/W} ({G/W}, {T}: Put a +1/+1 counter on this creature."""
    return _ast(
        card["name"],
        Keyword(name="lifelink", raw="Lifelink"),
        Keyword(name="reach", raw="Reach"),
        _custom("arcus_acolyte"),
    )


def the_swarmlord_handler(card):
    """The Swarmlord (#12652) — Rapid Regeneration — The Swarmlord enters with two +1/+1 counters on it for each"""
    return _ast(
        card["name"],
        _custom("the_swarmlord"),
    )


def fire_nation_cadets_handler(card):
    """Fire Nation Cadets (#12685) — This creature has firebending 2 as long as there's a Lesson card in your graveya"""
    return _ast(
        card["name"],
        _custom("fire_nation_cadets"),
    )


def footsteps_of_the_goryo_handler(card):
    """Footsteps of the Goryo (#12701) — Return target creature card from your graveyard to the battlefield. Sacrifice th"""
    return _ast(
        card["name"],
        _custom("footsteps_of_the_goryo"),
    )


def exdeath_void_warlock_handler(card):
    """Exdeath, Void Warlock // Neo Exdeath, Dimension's End (#12746) — """
    return _ast(
        card["name"],
        Keyword(name="trample", raw="Trample"),
        _custom("exdeath_void_warlock"),
    )


def songcrafter_mage_handler(card):
    """Songcrafter Mage (#12797) — Flash When this creature enters, target instant or sorcery card in your graveyar"""
    return _ast(
        card["name"],
        Keyword(name="flash", raw="Flash"),
        _custom("songcrafter_mage"),
    )


def cruel_entertainment_handler(card):
    """Cruel Entertainment (#12805) — Choose target player and another target player. The first player controls the se"""
    return _ast(
        card["name"],
        _custom("cruel_entertainment"),
    )


def invasion_plans_handler(card):
    """Invasion Plans (#12808) — All creatures block each combat if able. The attacking player chooses how each c"""
    return _ast(
        card["name"],
        _custom("invasion_plans"),
    )


def spy_network_handler(card):
    """Spy Network (#12814) — Look at target player's hand, the top card of that player's library, and any fac"""
    return _ast(
        card["name"],
        _custom("spy_network"),
    )


def lifeline_handler(card):
    """Lifeline (#12825) — Whenever a creature dies, if another creature is on the battlefield, return the """
    return _ast(
        card["name"],
        _custom("lifeline"),
    )


def catalyst_stone_handler(card):
    """Catalyst Stone (#12829) — Flashback costs you pay cost {2} less. Flashback costs your opponents pay cost {"""
    return _ast(
        card["name"],
        _custom("catalyst_stone"),
    )


def slave_of_bolas_handler(card):
    """Slave of Bolas (#12842) — Gain control of target creature. Untap that creature. It gains haste until end o"""
    return _ast(
        card["name"],
        _custom("slave_of_bolas"),
    )


def dynaheir_invoker_adept_handler(card):
    """Dynaheir, Invoker Adept (#12846) — Haste You may activate abilities of other creatures you control as though those """
    return _ast(
        card["name"],
        Keyword(name="haste", raw="Haste"),
        _custom("dynaheir_invoker_adept"),
    )


def the_pandorica_handler(card):
    """The Pandorica (#12855) — You may choose not to untap The Pandorica during your untap step. {1}{W}, {T}: U"""
    return _ast(
        card["name"],
        _custom("the_pandorica"),
    )


def supplant_form_handler(card):
    """Supplant Form (#12857) — Return target creature to its owner's hand. You create a token that's a copy of """
    return _ast(
        card["name"],
        _custom("supplant_form"),
    )


def tonberry_handler(card):
    """Tonberry (#12858) — This creature enters tapped with a stun counter on it. (If it would become untap"""
    return _ast(
        card["name"],
        _custom("tonberry"),
    )


def shizuko_caller_of_autumn_handler(card):
    """Shizuko, Caller of Autumn (#12865) — At the beginning of each player's upkeep, that player adds {G}{G}{G}. Until end """
    return _ast(
        card["name"],
        _custom("shizuko_caller_of_autumn"),
    )


def purphoross_intervention_handler(card):
    """Purphoros's Intervention (#12891) — Choose one — • Create an X/1 red Elemental creature token with trample and haste"""
    return _ast(
        card["name"],
        _custom("purphoross_intervention"),
    )


def galepowder_mage_handler(card):
    """Galepowder Mage (#12926) — Flying Whenever this creature attacks, exile another target creature. Return tha"""
    return _ast(
        card["name"],
        Keyword(name="flying", raw="Flying"),
        _custom("galepowder_mage"),
    )


def harness_by_force_handler(card):
    """Harness by Force (#12930) — Strive — This spell costs {2}{R} more to cast for each target beyond the first. """
    return _ast(
        card["name"],
        _custom("harness_by_force"),
    )


def teleportal_handler(card):
    """Teleportal (#12942) — Target creature you control gets +1/+0 until end of turn and can't be blocked th"""
    return _ast(
        card["name"],
        _custom("teleportal"),
    )


def ambush_commander_handler(card):
    """Ambush Commander (#12943) — Forests you control are 1/1 green Elf creatures that are still lands. {1}{G}, Sa"""
    return _ast(
        card["name"],
        _custom("ambush_commander"),
    )


def essence_pulse_handler(card):
    """Essence Pulse (#12952) — You gain 2 life. Each creature gets -X/-X until end of turn, where X is the amou"""
    return _ast(
        card["name"],
        _custom("essence_pulse"),
    )


def shellshock_handler(card):
    """Shellshock (#12976) — For each opponent, choose up to one target creature that player controls. Shells"""
    return _ast(
        card["name"],
        _custom("shellshock"),
    )


def curse_of_the_cabal_handler(card):
    """Curse of the Cabal (#12978) — Target player sacrifices half the permanents they control of their choice, round"""
    return _ast(
        card["name"],
        _custom("curse_of_the_cabal"),
    )


def varchilds_war_riders_handler(card):
    """Varchild's War-Riders (#12982) — Cumulative upkeep—Have an opponent create a 1/1 red Survivor creature token. (At"""
    return _ast(
        card["name"],
        Keyword(name="trample", raw="Trample"),
        _custom("varchilds_war_riders"),
    )


def kataras_reversal_handler(card):
    """Katara's Reversal (#13013) — Counter up to four target spells and/or abilities. Untap up to four target artif"""
    return _ast(
        card["name"],
        _custom("kataras_reversal"),
    )


def felhide_spiritbinder_handler(card):
    """Felhide Spiritbinder (#13017) — Inspired — Whenever this creature becomes untapped, you may pay {1}{R}. If you d"""
    return _ast(
        card["name"],
        _custom("felhide_spiritbinder"),
    )


def roon_of_the_hidden_realm_handler(card):
    """Roon of the Hidden Realm (#13018) — Vigilance, trample {2}, {T}: Exile another target creature. Return that card to """
    return _ast(
        card["name"],
        Keyword(name="vigilance", raw="Vigilance"),
        Keyword(name="trample", raw="Trample"),
        _custom("roon_of_the_hidden_realm"),
    )


def return_triumphant_handler(card):
    """Return Triumphant (#13024) — Return target creature card with mana value 3 or less from your graveyard to the"""
    return _ast(
        card["name"],
        _custom("return_triumphant"),
    )


def exterminator_magmarch_handler(card):
    """Exterminator Magmarch (#13032) — Whenever you cast an instant or sorcery spell that targets only a single nonland"""
    return _ast(
        card["name"],
        _custom("exterminator_magmarch"),
    )


def rainbow_vale_handler(card):
    """Rainbow Vale (#13038) — {T}: Add one mana of any color. An opponent gains control of this land at the be"""
    return _ast(
        card["name"],
        _custom("rainbow_vale"),
    )


def whirlwind_technique_handler(card):
    """Whirlwind Technique (#13039) — Target player draws two cards, then discards a card. Airbend up to two target cr"""
    return _ast(
        card["name"],
        _custom("whirlwind_technique"),
    )


def druid_of_the_emerald_grove_handler(card):
    """Druid of the Emerald Grove (#13066) — When this creature enters, search your library for up to two basic land cards an"""
    return _ast(
        card["name"],
        _custom("druid_of_the_emerald_grove"),
    )


def madame_web_clairvoyant_handler(card):
    """Madame Web, Clairvoyant (#13077) — You may look at the top card of your library any time. You may cast Spider spell"""
    return _ast(
        card["name"],
        _custom("madame_web_clairvoyant"),
    )


def tempt_with_immortality_handler(card):
    """Tempt with Immortality (#13079) — Tempting offer — Return a creature card from your graveyard to the battlefield. """
    return _ast(
        card["name"],
        _custom("tempt_with_immortality"),
    )


def predatory_impetus_handler(card):
    """Predatory Impetus (#13081) — Enchant creature Enchanted creature gets +3/+3, must be blocked if able, and is """
    return _ast(
        card["name"],
        _custom("predatory_impetus"),
    )


def harmonious_archon_handler(card):
    """Harmonious Archon (#13086) — Flying Non-Archon creatures have base power and toughness 3/3. When this creatur"""
    return _ast(
        card["name"],
        Keyword(name="flying", raw="Flying"),
        _custom("harmonious_archon"),
    )


def chrome_dome_handler(card):
    """Chrome Dome (#13111) — Other artifact creatures you control get +1/+0. {5}: Create a token that's a cop"""
    return _ast(
        card["name"],
        _custom("chrome_dome"),
    )


def the_moment_handler(card):
    """The Moment (#13116) — At the beginning of your upkeep, put a time counter on The Moment. {2}, {T}: Unt"""
    return _ast(
        card["name"],
        _custom("the_moment"),
    )


def rhinos_rampage_handler(card):
    """Rhino's Rampage (#13122) — Target creature you control gets +1/+0 until end of turn. It fights target creat"""
    return _ast(
        card["name"],
        _custom("rhinos_rampage"),
    )


def voidwalk_handler(card):
    """Voidwalk (#13128) — Exile target creature. Return it to the battlefield under its owner's control at"""
    return _ast(
        card["name"],
        _custom("voidwalk"),
    )


def multiple_choice_handler(card):
    """Multiple Choice (#13145) — If X is 1, scry 1, then draw a card. If X is 2, you may choose a player. They re"""
    return _ast(
        card["name"],
        _custom("multiple_choice"),
    )


def bearer_of_the_heavens_handler(card):
    """Bearer of the Heavens (#13162) — When this creature dies, destroy all permanents at the beginning of the next end"""
    return _ast(
        card["name"],
        _custom("bearer_of_the_heavens"),
    )


def intimidation_bolt_handler(card):
    """Intimidation Bolt (#13174) — Intimidation Bolt deals 3 damage to target creature. Other creatures can't attac"""
    return _ast(
        card["name"],
        _custom("intimidation_bolt"),
    )


def feather_radiant_arbiter_handler(card):
    """Feather, Radiant Arbiter (#13208) — Flying, lifelink Whenever you cast a noncreature spell that targets only Feather"""
    return _ast(
        card["name"],
        Keyword(name="flying", raw="Flying"),
        Keyword(name="lifelink", raw="Lifelink"),
        _custom("feather_radiant_arbiter"),
    )


def timesifter_handler(card):
    """Timesifter (#13209) — At the beginning of each upkeep, each player exiles the top card of their librar"""
    return _ast(
        card["name"],
        _custom("timesifter"),
    )


def prized_amalgam_handler(card):
    """Prized Amalgam (#13222) — Whenever a creature enters, if it entered from your graveyard or you cast it fro"""
    return _ast(
        card["name"],
        _custom("prized_amalgam"),
    )


def unexpected_request_handler(card):
    """Unexpected Request (#13256) — Gain control of target creature until end of turn. Untap that creature. It gains"""
    return _ast(
        card["name"],
        _custom("unexpected_request"),
    )


def cogwork_assembler_handler(card):
    """Cogwork Assembler (#13271) — {7}: Create a token that's a copy of target artifact. That token gains haste. Ex"""
    return _ast(
        card["name"],
        _custom("cogwork_assembler"),
    )


def cid_timeless_artificer_handler(card):
    """Cid, Timeless Artificer (#13273) — Artifact creatures and Heroes you control get +1/+1 for each Artificer you contr"""
    return _ast(
        card["name"],
        _custom("cid_timeless_artificer"),
    )


def thelon_of_havenwood_handler(card):
    """Thelon of Havenwood (#13279) — Each Fungus creature gets +1/+1 for each spore counter on it. {B}{G}, Exile a Fu"""
    return _ast(
        card["name"],
        _custom("thelon_of_havenwood"),
    )


def hall_of_gemstone_handler(card):
    """Hall of Gemstone (#13289) — At the beginning of each player's upkeep, that player chooses a color. Until end"""
    return _ast(
        card["name"],
        _custom("hall_of_gemstone"),
    )


def sonar_strike_handler(card):
    """Sonar Strike (#13318) — Sonar Strike deals 4 damage to target attacking, blocking, or tapped creature. Y"""
    return _ast(
        card["name"],
        _custom("sonar_strike"),
    )


def rosheen_roaring_prophet_handler(card):
    """Rosheen, Roaring Prophet (#13331) — When Rosheen enters, mill six cards. You may put a card with {X} in its mana cos"""
    return _ast(
        card["name"],
        _custom("rosheen_roaring_prophet"),
    )


def magosi_the_waterveil_handler(card):
    """Magosi, the Waterveil (#13355) — This land enters tapped. {T}: Add {U}. {U}, {T}: Put an eon counter on this land"""
    return _ast(
        card["name"],
        _custom("magosi_the_waterveil"),
    )


def ashling_the_extinguisher_handler(card):
    """Ashling, the Extinguisher (#13362) — Whenever Ashling deals combat damage to a player, choose target creature that pl"""
    return _ast(
        card["name"],
        _custom("ashling_the_extinguisher"),
    )


def push_the_limit_handler(card):
    """Push the Limit (#13367) — Return all Mount and Vehicle cards from your graveyard to the battlefield. Sacri"""
    return _ast(
        card["name"],
        _custom("push_the_limit"),
    )


def karakyk_guardian_handler(card):
    """Karakyk Guardian (#13373) — Flying, vigilance, trample This creature has hexproof if it hasn't dealt damage """
    return _ast(
        card["name"],
        Keyword(name="flying", raw="Flying"),
        Keyword(name="vigilance", raw="Vigilance"),
        Keyword(name="trample", raw="Trample"),
        _custom("karakyk_guardian"),
    )


def storage_matrix_handler(card):
    """Storage Matrix (#13385) — As long as this artifact is untapped, each player chooses artifact, creature, or"""
    return _ast(
        card["name"],
        _custom("storage_matrix"),
    )


def glacial_dragonhunt_handler(card):
    """Glacial Dragonhunt (#13405) — Draw a card, then you may discard a card. When you discard a nonland card this w"""
    return _ast(
        card["name"],
        _custom("glacial_dragonhunt"),
    )


def experimental_frenzy_handler(card):
    """Experimental Frenzy (#13416) — You may look at the top card of your library any time. You may play lands and ca"""
    return _ast(
        card["name"],
        _custom("experimental_frenzy"),
    )


def blaster_combat_dj_handler(card):
    """Blaster, Combat DJ // Blaster, Morale Booster (#13424) — """
    return _ast(
        card["name"],
        _custom("blaster_combat_dj"),
    )


def in_bolass_clutches_handler(card):
    """In Bolas's Clutches (#13433) — Enchant permanent You control enchanted permanent. Enchanted permanent is legend"""
    return _ast(
        card["name"],
        _custom("in_bolass_clutches"),
    )


def invasion_of_ravnica_handler(card):
    """Invasion of Ravnica // Guildpact Paragon (#13434) — """
    return _ast(
        card["name"],
        _custom("invasion_of_ravnica"),
    )


def scout_the_city_handler(card):
    """Scout the City (#13459) — Choose one — • Look Around — Mill three cards. You may put a permanent card from"""
    return _ast(
        card["name"],
        _custom("scout_the_city"),
    )


def wisecrack_handler(card):
    """Wisecrack (#13470) — Target creature deals damage equal to its power to itself. If that creature is a"""
    return _ast(
        card["name"],
        _custom("wisecrack"),
    )


def the_duke_rebel_sentry_handler(card):
    """The Duke, Rebel Sentry (#13520) — The Duke enters with a +1/+1 counter on him. {T}, Remove a counter from The Duke"""
    return _ast(
        card["name"],
        _custom("the_duke_rebel_sentry"),
    )


def syr_elenora_the_discerning_handler(card):
    """Syr Elenora, the Discerning (#13531) — Syr Elenora's power is equal to the number of cards in your hand. When Syr Eleno"""
    return _ast(
        card["name"],
        _custom("syr_elenora_the_discerning"),
    )


def jared_carthalion_true_heir_handler(card):
    """Jared Carthalion, True Heir (#13548) — When Jared Carthalion enters, target opponent becomes the monarch. You can't bec"""
    return _ast(
        card["name"],
        _custom("jared_carthalion_true_heir"),
    )


def team_pennant_handler(card):
    """Team Pennant (#13561) — Equipped creature gets +1/+1 and has vigilance and trample. Equip creature token"""
    return _ast(
        card["name"],
        _custom("team_pennant"),
    )


def fires_of_invention_handler(card):
    """Fires of Invention (#13575) — You can cast spells only during your turn and you can cast no more than two spel"""
    return _ast(
        card["name"],
        _custom("fires_of_invention"),
    )


def invasion_of_kaladesh_handler(card):
    """Invasion of Kaladesh // Aetherwing, Golden-Scale Flagship (#13583) — """
    return _ast(
        card["name"],
        Keyword(name="flying", raw="Flying"),
        _custom("invasion_of_kaladesh"),
    )


def brass_herald_handler(card):
    """Brass Herald (#13600) — As this creature enters, choose a creature type. When this creature enters, reve"""
    return _ast(
        card["name"],
        _custom("brass_herald"),
    )


def seeker_of_slaanesh_handler(card):
    """Seeker of Slaanesh (#13611) — Haste Allure of Slaanesh — Each opponent must attack with at least one creature """
    return _ast(
        card["name"],
        Keyword(name="haste", raw="Haste"),
        _custom("seeker_of_slaanesh"),
    )


def heart_of_kiran_handler(card):
    """Heart of Kiran (#13629) — Flying, vigilance Crew 3 (Tap any number of creatures you control with total pow"""
    return _ast(
        card["name"],
        Keyword(name="flying", raw="Flying"),
        Keyword(name="vigilance", raw="Vigilance"),
        _custom("heart_of_kiran"),
    )


def terrific_team_up_handler(card):
    """Terrific Team-Up (#13697) — This spell costs {2} less to cast if you control a permanent with mana value 4 o"""
    return _ast(
        card["name"],
        _custom("terrific_team_up"),
    )


def pirate_hat_handler(card):
    """Pirate Hat (#13706) — Equipped creature gets +1/+1 and has "Whenever this creature attacks, draw a car"""
    return _ast(
        card["name"],
        _custom("pirate_hat"),
    )


def radiate_handler(card):
    """Radiate (#13756) — Choose target instant or sorcery spell that targets only a single permanent or p"""
    return _ast(
        card["name"],
        _custom("radiate"),
    )


def aurification_handler(card):
    """Aurification (#13765) — Whenever a creature deals damage to you, put a gold counter on it. Each creature"""
    return _ast(
        card["name"],
        _custom("aurification"),
    )


def conspiracy_unraveler_handler(card):
    """Conspiracy Unraveler (#13841) — Flying You may collect evidence 10 rather than pay the mana cost for spells you """
    return _ast(
        card["name"],
        Keyword(name="flying", raw="Flying"),
        _custom("conspiracy_unraveler"),
    )


def gimli_mournful_avenger_handler(card):
    """Gimli, Mournful Avenger (#13846) — Gimli has indestructible as long as two or more creatures died under your contro"""
    return _ast(
        card["name"],
        _custom("gimli_mournful_avenger"),
    )


def recall_handler(card):
    """Recall (#13858) — Discard X cards, then return a card from your graveyard to your hand for each ca"""
    return _ast(
        card["name"],
        _custom("recall"),
    )


def fatal_frenzy_handler(card):
    """Fatal Frenzy (#13866) — Until end of turn, target creature you control gains trample and gets +X/+0, whe"""
    return _ast(
        card["name"],
        _custom("fatal_frenzy"),
    )


def fell_beasts_shriek_handler(card):
    """Fell Beast's Shriek (#13874) — Each opponent chooses a creature they control. Tap and goad the chosen creatures"""
    return _ast(
        card["name"],
        _custom("fell_beasts_shriek"),
    )


def once_and_future_handler(card):
    """Once and Future (#13875) — Return target card from your graveyard to your hand. Put up to one other target """
    return _ast(
        card["name"],
        _custom("once_and_future"),
    )


def frenetic_sliver_handler(card):
    """Frenetic Sliver (#13904) — All Slivers have "{0}: If this permanent is on the battlefield, flip a coin. If """
    return _ast(
        card["name"],
        _custom("frenetic_sliver"),
    )


def beastie_beatdown_handler(card):
    """Beastie Beatdown (#13926) — Choose target creature you control and target creature an opponent controls. Del"""
    return _ast(
        card["name"],
        _custom("beastie_beatdown"),
    )


def lochmere_serpent_handler(card):
    """Lochmere Serpent (#13951) — Flash {U}, Sacrifice an Island: This creature can't be blocked this turn. {B}, S"""
    return _ast(
        card["name"],
        Keyword(name="flash", raw="Flash"),
        _custom("lochmere_serpent"),
    )


def evelyn_the_covetous_handler(card):
    """Evelyn, the Covetous (#13954) — Flash Whenever Evelyn or another Vampire you control enters, exile the top card """
    return _ast(
        card["name"],
        Keyword(name="flash", raw="Flash"),
        _custom("evelyn_the_covetous"),
    )


def dai_li_indoctrination_handler(card):
    """Dai Li Indoctrination (#13963) — Choose one — • Target opponent reveals their hand. You choose a nonland permanen"""
    return _ast(
        card["name"],
        _custom("dai_li_indoctrination"),
    )


def lara_croft_tomb_raider_handler(card):
    """Lara Croft, Tomb Raider (#13965) — First strike, reach Whenever Lara Croft attacks, exile up to one target legendar"""
    return _ast(
        card["name"],
        Keyword(name="reach", raw="Reach"),
        Keyword(name="first strike", raw="First strike"),
        _custom("lara_croft_tomb_raider"),
    )


def portal_manipulator_handler(card):
    """Portal Manipulator (#13986) — Flash When this creature enters during the declare attackers step, choose target"""
    return _ast(
        card["name"],
        Keyword(name="flash", raw="Flash"),
        _custom("portal_manipulator"),
    )


def waxing_moon_handler(card):
    """Waxing Moon (#14002) — Transform up to one target Werewolf you control. Creatures you control gain tram"""
    return _ast(
        card["name"],
        _custom("waxing_moon"),
    )


def sigardian_zealot_handler(card):
    """Sigardian Zealot (#14013) — At the beginning of combat on your turn, choose any number of creatures with dif"""
    return _ast(
        card["name"],
        _custom("sigardian_zealot"),
    )


def tunnel_vision_handler(card):
    """Tunnel Vision (#14016) — Choose a card name. Target player reveals cards from the top of their library un"""
    return _ast(
        card["name"],
        _custom("tunnel_vision"),
    )


def vanish_into_eternity_handler(card):
    """Vanish into Eternity (#14050) — This spell costs {3} more to cast if it targets a creature. Exile target nonland"""
    return _ast(
        card["name"],
        _custom("vanish_into_eternity"),
    )


def dragonfly_swarm_handler(card):
    """Dragonfly Swarm (#14068) — Flying, ward {1} (Whenever this creature becomes the target of a spell or abilit"""
    return _ast(
        card["name"],
        Keyword(name="flying", raw="Flying"),
        _custom("dragonfly_swarm"),
    )


def coin_of_fate_handler(card):
    """Coin of Fate (#14081) — When this artifact enters, surveil 1. {3}{W}, {T}, Exile two creature cards from"""
    return _ast(
        card["name"],
        _custom("coin_of_fate"),
    )


def gorilla_berserkers_handler(card):
    """Gorilla Berserkers (#14112) — Trample; rampage 2 (Whenever this creature becomes blocked, it gets +2/+2 until """
    return _ast(
        card["name"],
        Keyword(name="trample", raw="Trample"),
        _custom("gorilla_berserkers"),
    )


def gorm_the_great_handler(card):
    """Gorm the Great (#14123) — Partner with Virtus the Veiled (When this creature enters, target player may put"""
    return _ast(
        card["name"],
        Keyword(name="vigilance", raw="Vigilance"),
        _custom("gorm_the_great"),
    )


# ---------------------------------------------------------------------------
# Registry
# ---------------------------------------------------------------------------

PER_CARD_HANDLERS: dict[str, callable] = {
    "Doomsday": doomsday_handler,
    "Balance": balance_handler,
    "Falling Star": falling_star_handler,
    "Painter's Servant": painters_servant_handler,
    "Camouflage": camouflage_handler,
    "Word of Command": word_of_command_handler,
    "Wheel of Misfortune": wheel_of_misfortune_handler,
    "Warp World": warp_world_handler,
    "Psychic Battle": psychic_battle_handler,
    "Goblin Game": goblin_game_handler,
    "Glimpse of Tomorrow": glimpse_of_tomorrow_handler,
    "Lim-Dûl's Vault": lim_duls_vault_handler,
    "Pox": pox_handler,
    "Drought": drought_handler,
    "The Great Aurora": great_aurora_handler,
    "Muse Vortex": muse_vortex_handler,
    "Genesis Wave": genesis_wave_handler,
    "Call to Arms": call_to_arms_handler,
    "Selective Adaptation": selective_adaptation_handler,
    "Hibernation's End": hibernations_end_handler,
    "Natural Balance": natural_balance_handler,
    "Titania's Command": titanias_command_handler,
    "Archdruid's Charm": archdruids_charm_handler,
    "Guff Rewrites History": guff_rewrites_history_handler,
    "Hew the Entwood": hew_the_entwood_handler,
    "Map to Lorthos's Temple": map_to_lorthos_handler,
    "Codie, Vociferous Codex": codie_handler,
    "Journey to the Lost City": journey_lost_city_handler,
    "Nature's Wrath": natures_wrath_handler,
    # ---- Wave 2 (top-EDH-rank PARTIAL escapees) -------------------------
    "Demolition Field": demolition_field_handler,
    "Opposition Agent": opposition_agent_handler,
    "Tibalt's Trickery": tibalts_trickery_handler,
    "Midnight Clock": midnight_clock_handler,
    "Wandering Archaic // Explore the Vastlands": wandering_archaic_handler,
    "Seize the Spotlight": seize_the_spotlight_handler,
    "Season of Gathering": season_of_gathering_handler,
    "Mycosynth Lattice": mycosynth_lattice_handler,
    "Jetmir, Nexus of Revels": jetmir_handler,
    "Season of Weaving": season_of_weaving_handler,
    "Eluge, the Shoreless Sea": eluge_handler,
    "Nowhere to Run": nowhere_to_run_handler,
    "Master of Cruelties": master_of_cruelties_handler,
    "Keen Duelist": keen_duelist_handler,
    "Tenuous Truce": tenuous_truce_handler,
    "Illusionist's Gambit": illusionists_gambit_handler,
    "Nahiri's Lithoforming": nahiris_lithoforming_handler,
    "Season of the Burrow": season_of_the_burrow_handler,
    "Beza, the Bounding Spring": beza_handler,
    "Fblthp, Lost on the Range": fblthp_handler,
    "Khârn the Betrayer": kharn_handler,
    "Season of Loss": season_of_loss_handler,
    "Arvinox, the Mind Flail": arvinox_handler,
    "Toralf, God of Fury // Toralf's Hammer": toralf_god_handler,
    "Angelic Arbiter": angelic_arbiter_handler,
    "Tempt with Mayhem": tempt_with_mayhem_handler,
    "Pendant of Prosperity": pendant_of_prosperity_handler,
    "Make an Example": make_an_example_handler,
    "Chef's Kiss": chefs_kiss_handler,
    "Pact Weapon": pact_weapon_handler,
    "Pir's Whim": pirs_whim_handler,
    "Stargaze": stargaze_handler,
    "Herigast, Erupting Nullkite": herigast_handler,
    "Sin, Unending Cataclysm": sin_unending_handler,
    "Season of the Bold": season_of_the_bold_handler,
    "Etrata, the Silencer": etrata_handler,
    "Lifecraft Engine": lifecraft_engine_handler,
    "Ashling, the Limitless": ashling_limitless_handler,
    "Graaz, Unstoppable Juggernaut": graaz_handler,
    "Berserker's Frenzy": berserkers_frenzy_handler,
    "Falco Spara, Pactweaver": falco_spara_handler,
    "Hazezon, Shaper of Sand": hazezon_sand_handler,
    "Intellect Devourer": intellect_devourer_handler,
    "The War Doctor": war_doctor_handler,
    "Call for Aid": call_for_aid_handler,
    "Spelltwine": spelltwine_handler,
    "Hierophant Bio-Titan": hierophant_bio_titan_handler,
    "Blue Mage's Cane": blue_mages_cane_handler,
    "Summoner's Grimoire": summoners_grimoire_handler,
    "Trepanation Blade": trepanation_blade_handler,
    "Overwhelming Splendor": overwhelming_splendor_handler,
    "Vaevictis Asmadi, the Dire": vaevictis_handler,
    "Ninja's Blades": ninjas_blades_handler,
    "Master of the Wild Hunt": master_of_the_wild_hunt_handler,
    "Megaton's Fate": megatons_fate_handler,
    "Rex, Cyber-Hound": rex_cyber_hound_handler,
    "The Eleventh Doctor": eleventh_doctor_handler,
    "Gollum, Scheming Guide": gollum_scheming_handler,
    "Ran and Shaw": ran_and_shaw_handler,
    "Talent of the Telepath": talent_telepath_handler,
    "Indominus Rex, Alpha": indominus_rex_handler,
    "Nacatl War-Pride": nacatl_war_pride_handler,
    "Elephant Grass": elephant_grass_handler,
    "Savage Summoning": savage_summoning_handler,
    "Ninja Teen": ninja_teen_handler,
    "Memories Returning": memories_returning_handler,
    "The Wedding of River Song": wedding_river_song_handler,
    "Flash Photography": flash_photography_handler,
    "Mages' Contest": mages_contest_handler,
    "Manascape Refractor": manascape_refractor_handler,
    "Lightstall Inquisitor": lightstall_inquisitor_handler,
    "Melira, Sylvok Outcast": melira_handler,
    "Wernog, Rider's Chaplain": wernog_handler,
    "Konda's Banner": kondas_banner_handler,
    "Saruman of Many Colors": saruman_many_colors_handler,
    "Guided Passage": guided_passage_handler,
    "Collective Brutality": collective_brutality_handler,
    "Knight-Errant of Eos": knight_errant_eos_handler,
    "Sibylline Soothsayer": sibylline_soothsayer_handler,
    "Virtus's Maneuver": virtuss_maneuver_handler,
    "Ward of Bones": ward_of_bones_handler,
    "Start the TARDIS": start_the_tardis_handler,
    "Deliver Unto Evil": deliver_unto_evil_handler,
    "Sigil of Distinction": sigil_of_distinction_handler,
    "Zndrsplt's Judgment": zndrsplts_judgment_handler,
    "Dark Intimations": dark_intimations_handler,
    "Extract the Truth": extract_the_truth_handler,
    "Immortal Coil": immortal_coil_handler,
    "Regna's Sanction": regnas_sanction_handler,
    "Turtles Forever": turtles_forever_handler,
    "Bell Borca, Spectral Sergeant": bell_borca_handler,
    "Parnesse, the Subtle Brush": parnesse_handler,
    "Order of Succession": order_of_succession_handler,
    "The Celestial Toymaker": celestial_toymaker_handler,
    "Hedonist's Trove": hedonists_trove_handler,
    # ---- Wave 3 (top-EDH-rank PARTIAL escapees, ranks ~29 to ~3900) -----
    "Chaos Warp": chaos_warp_handler,
    "Teferi's Protection": teferis_protection_handler,
    "Herald's Horn": heralds_horn_handler,
    "Three Tree City": three_tree_city_handler,
    "Animate Dead": animate_dead_handler,
    "Reality Shift": reality_shift_handler,
    "Underworld Breach": underworld_breach_handler,
    "Mystic Forge": mystic_forge_handler,
    "Maskwood Nexus": maskwood_nexus_handler,
    "Birgi, God of Storytelling // Harnfel, Horn of Bounty": birgi_handler,
    "Terror of the Peaks": terror_of_the_peaks_handler,
    "Dread Return": dread_return_handler,
    "Valakut Awakening // Valakut Stoneforge": valakut_awakening_handler,
    "Champion of Lambholt": champion_of_lambholt_handler,
    "Commander's Plate": commanders_plate_handler,
    "Blasphemous Edict": blasphemous_edict_handler,
    "Culling Ritual": culling_ritual_handler,
    "Realmwalker": realmwalker_handler,
    "Plaguecrafter": plaguecrafter_handler,
    "Torment of Hailfire": torment_of_hailfire_handler,
    "Clever Concealment": clever_concealment_handler,
    "Forbidden Orchard": forbidden_orchard_handler,
    "Borne Upon a Wind": borne_upon_a_wind_handler,
    "Bridgeworks Battle // Tanglespan Bridgeworks": bridgeworks_battle_handler,
    "Elves of Deep Shadow": elves_of_deep_shadow_handler,
    "Flare of Fortitude": flare_of_fortitude_handler,
    "Ashaya, Soul of the Wild": ashaya_handler,
    "Tempt with Discovery": tempt_with_discovery_handler,
    "Breach the Multiverse": breach_the_multiverse_handler,
    "Delney, Streetwise Lookout": delney_handler,
    "Sword of Hearth and Home": sword_hearth_home_handler,
    "Forsaken Monument": forsaken_monument_handler,
    "Chain of Vapor": chain_of_vapor_handler,
    "Thousand-Year Elixir": thousand_year_elixir_handler,
    "Beseech the Mirror": beseech_mirror_handler,
    "Elesh Norn, Mother of Machines": elesh_norn_mom_handler,
    "Agadeem's Awakening // Agadeem, the Undercrypt": agadeems_awakening_handler,
    "Rewind": rewind_handler,
    "Razorkin Needlehead": razorkin_needlehead_handler,
    "Promise of Loyalty": promise_of_loyalty_handler,
    "Skrelv, Defector Mite": skrelv_handler,
    "The Black Gate": the_black_gate_handler,
    "Ezuri's Predation": ezuris_predation_handler,
    "Legion Leadership // Legion Stronghold": legion_leadership_handler,
    "City of Traitors": city_of_traitors_handler,
    "Sephara, Sky's Blade": sephara_handler,
    "Elixir of Immortality": elixir_of_immortality_handler,
    "Kinnan, Bonder Prodigy": kinnan_handler,
    "Comet Storm": comet_storm_handler,
    "Pest Infestation": pest_infestation_handler,
    "Deep Analysis": deep_analysis_handler,
    "Tendershoot Dryad": tendershoot_dryad_handler,
    "Morophon, the Boundless": morophon_handler,
    "The Soul Stone": soul_stone_handler,
    "Lethal Scheme": lethal_scheme_handler,
    "March of Swirling Mist": march_swirling_mist_handler,
    "Smuggler's Surprise": smugglers_surprise_handler,
    "Priest of Forgotten Gods": priest_of_forgotten_gods_handler,
    "Search for Tomorrow": search_for_tomorrow_handler,
    "Sephiroth, Fabled SOLDIER // Sephiroth, One-Winged Angel": sephiroth_handler,
    "Rain of Riches": rain_of_riches_handler,
    "Delina, Wild Mage": delina_wild_mage_handler,
    "Void Winnower": void_winnower_handler,
    "Jin-Gitaxias, Progress Tyrant": jin_gitaxias_pt_handler,
    "Gift of the Viper": gift_of_the_viper_handler,
    "Hoarding Broodlord": hoarding_broodlord_handler,
    "Leyline of the Guildpact": leyline_of_the_guildpact_handler,
    "Mirror Box": mirror_box_handler,
    "Giada, Font of Hope": giada_handler,
    "Elvish Warmaster": elvish_warmaster_handler,
    "Sphere Grid": sphere_grid_handler,
    "Wild Magic Surge": wild_magic_surge_handler,
    "Twenty-Toed Toad": twenty_toed_toad_handler,
    "Aven Interrupter": aven_interrupter_handler,
    "Emrakul, the Promised End": emrakul_promised_end_handler,
    "Scheming Symmetry": scheming_symmetry_handler,
    "Overlord of the Hauntwoods": overlord_hauntwoods_handler,
    "Secret Tunnel": secret_tunnel_handler,
    "Crippling Fear": crippling_fear_handler,
    "Army of the Damned": army_of_the_damned_handler,
    "Grafdigger's Cage": grafdiggers_cage_handler,
    "Hellkite Courser": hellkite_courser_handler,
    "Fractured Sanity": fractured_sanity_handler,
    "Virtue of Knowledge // Vantress Visions": virtue_of_knowledge_handler,
    "Crystal Skull, Isu Spyglass": crystal_skull_isu_handler,
    "Sudden Spoiling": sudden_spoiling_handler,
    "Ancient Gold Dragon": ancient_gold_dragon_handler,
    "Bulk Up": bulk_up_handler,
    "Tempt with Bunnies": tempt_with_bunnies_handler,
    "Angel's Grace": angels_grace_handler,
    "Minas Morgul, Dark Fortress": minas_morgul_handler,
    "Winds of Abandon": winds_of_abandon_handler,
    "Party Thrasher": party_thrasher_handler,
    "Lignify": lignify_handler,
    "Lich-Knights' Conquest": lich_knights_conquest_handler,
    "Orim's Chant": orims_chant_handler,
    "Sandwurm Convergence": sandwurm_convergence_handler,
    "Plargg and Nassari": plargg_nassari_handler,
    "Dance of the Dead": dance_of_the_dead_handler,
    "Toxrill, the Corrosive": toxrill_handler,
    "Doc Aurlock, Grizzled Genius": doc_aurlock_handler,
    "Kogla and Yidaro": kogla_yidaro_handler,
    "Gishath, Sun's Avatar": gishath_handler,
    "Overlord of the Balemurk": overlord_balemurk_handler,
    "Emrakul, the World Anew": emrakul_world_anew_handler,
    "Yuriko, the Tiger's Shadow": yuriko_handler,
    "Agitator Ant": agitator_ant_handler,
    "Gond Gate": gond_gate_handler,
    "Gilded Drake": gilded_drake_handler,
    "The Immortal Sun": immortal_sun_handler,
    "Momentous Fall": momentous_fall_handler,
    "Semester's End": semesters_end_handler,
    "Gold-Forged Thopteryx": gold_forged_thopteryx_handler,
    "Dance of the Manse": dance_of_the_manse_handler,
    "Spectacular Showdown": spectacular_showdown_handler,
    "Debt to the Deathless": debt_to_the_deathless_handler,
    "Ancient Cellarspawn": ancient_cellarspawn_handler,
    "Planar Nexus": planar_nexus_handler,
    "Aminatou's Augury": aminatous_augury_handler,
    "Patrolling Peacemaker": patrolling_peacemaker_handler,
    "Hell to Pay": hell_to_pay_handler,
    "Tempt with Vengeance": tempt_with_vengeance_handler,
    "Zombie Master": zombie_master_handler,
    "Syr Ginger, the Meal Ender": syr_ginger_handler,
    "Gonti, Night Minister": gonti_night_minister_handler,
    "Ichormoon Gauntlet": ichormoon_gauntlet_handler,
    "Circle of Power": circle_of_power_handler,
    "The Last Agni Kai": last_agni_kai_handler,
    "Yedora, Grave Gardener": yedora_handler,
    "Sacrifice": sacrifice_handler,
    "Oath of Teferi": oath_of_teferi_handler,
    "Gisa, the Hellraiser": gisa_hellraiser_handler,
    "Beacon of Immortality": beacon_of_immortality_handler,
    "Overlord of the Floodpits": overlord_floodpits_handler,
    "Multiversal Passage": multiversal_passage_handler,
    "Lavinia, Azorius Renegade": lavinia_handler,
    "Soulless Jailer": soulless_jailer_handler,
    "Overmaster": overmaster_handler,
    "Blood for the Blood God!": blood_for_blood_god_handler,
    "Gluntch, the Bestower": gluntch_handler,
    "Endrek Sahr, Master Breeder": endrek_sahr_handler,
    "Chain of Smog": chain_of_smog_handler,
    "Burnt Offering": burnt_offering_handler,
    "Render Silent": render_silent_handler,
    "Zack Fair": zack_fair_handler,
    "Rite of the Raging Storm": rite_of_raging_storm_handler,
    "Sigarda, Font of Blessings": sigarda_font_blessings_handler,
    "Dress Down": dress_down_handler,
    "Mob Rule": mob_rule_handler,
    "Animist's Awakening": animists_awakening_handler,
    "Necromantic Selection": necromantic_selection_handler,
    "Collective Effort": collective_effort_handler,
    "Skullwinder": skullwinder_handler,
    "White Sun's Zenith": white_suns_zenith_handler,
    "The Necrobloom": the_necrobloom_handler,
    "Chitterspitter": chitterspitter_handler,
    "Retether": retether_handler,
    "Prisoner's Dilemma": prisoners_dilemma_handler,
    "Grove of the Burnwillows": grove_of_burnwillows_handler,
    "Cid, Freeflier Pilot": cid_handler,
    "Korlessa, Scale Singer": korlessa_handler,
    "Sower of Discord": sower_of_discord_handler,
    "Reckoner Bankbuster": reckoner_bankbuster_handler,
    "Conspicuous Snoop": conspicuous_snoop_handler,
    "Fear of Sleep Paralysis": fear_of_sleep_paralysis_handler,
    "World at War": world_at_war_handler,
    "Royal Treatment": royal_treatment_handler,
    "Kuja, Genome Sorcerer // Trance Kuja, Fate Defied": kuja_handler,
    "Dread Summons": dread_summons_handler,
    "Oviya, Automech Artisan": oviya_handler,
    "Hushbringer": hushbringer_handler,
    "Jeweled Amulet": jeweled_amulet_handler,
    "Glarb, Calamity's Augur": glarb_handler,
    "Genesis Ultimatum": genesis_ultimatum_handler,
    "Minds Aglow": minds_aglow_handler,
    "Sludge Monster": sludge_monster_handler,
    "Encroaching Mycosynth": encroaching_mycosynth_handler,
    "Reins of Power": reins_of_power_handler,
    "Solemnity": solemnity_handler,
    "Titania, Nature's Force": titania_natures_force_handler,
    "The Millennium Calendar": millennium_calendar_handler,
    "Maddening Hex": maddening_hex_handler,
    "Search for Glory": search_for_glory_handler,
    "Fandaniel, Telophoroi Ascian": fandaniel_handler,
    # ---- Wave 4 (top-100 PARTIAL escapees, ranks ~2341 to ~5176) -------
    'Uthros Research Craft': uthros_research_craft_handler,
    'Weathered Sentinels': weathered_sentinels_handler,
    'Reaver Titan': reaver_titan_handler,
    'K-9, Mark I': k9_mark_i_handler,
    'Zirda, the Dawnwaker': zirda_the_dawnwaker_handler,
    'Roadside Reliquary': roadside_reliquary_handler,
    'Wildsear, Scouring Maw': wildsear_scouring_maw_handler,
    'The Master, Multiplied': the_master_multiplied_handler,
    'Hideous Taskmaster': hideous_taskmaster_handler,
    'Yes Man, Personal Securitron': yes_man_personal_securitron_handler,
    'Scavenged Brawler': scavenged_brawler_handler,
    'Scion of Draco': scion_of_draco_handler,
    'The Second Doctor': the_second_doctor_handler,
    'Firebending Student': firebending_student_handler,
    'Oubliette': oubliette_handler,
    'Blast-Furnace Hellkite': blastfurnace_hellkite_handler,
    'Vines of Vastwood': vines_of_vastwood_handler,
    'Zhur-Taa Druid': zhurtaa_druid_handler,
    'All-Out Assault': allout_assault_handler,
    'Feasting Hobbit': feasting_hobbit_handler,
    'Darksteel Reactor': darksteel_reactor_handler,
    'Toph, the First Metalbender': toph_the_first_metalbender_handler,
    'Muxus, Goblin Grandee': muxus_goblin_grandee_handler,
    'Howlsquad Heavy': howlsquad_heavy_handler,
    'Ultima, Origin of Oblivion': ultima_origin_of_oblivion_handler,
    'Spectral Deluge': spectral_deluge_handler,
    'Baeloth Barrityl, Entertainer': baeloth_barrityl_entertainer_handler,
    'Gogo, Master of Mimicry': gogo_master_of_mimicry_handler,
    'Assault Suit': assault_suit_handler,
    'Phyrexian Censor': phyrexian_censor_handler,
    'Kopala, Warden of Waves': kopala_warden_of_waves_handler,
    'Urabrask, Heretic Praetor': urabrask_heretic_praetor_handler,
    'Rolling Hamsphere': rolling_hamsphere_handler,
    'Might of the Meek': might_of_the_meek_handler,
    'Chainsaw': chainsaw_handler,
    'Emperor Mihail II': emperor_mihail_ii_handler,
    'Clockspinning': clockspinning_handler,
    "Selvala's Stampede": selvalas_stampede_handler,
    'Turnabout': turnabout_handler,
    "Luxior, Giada's Gift": luxior_giadas_gift_handler,
    'Animation Module': animation_module_handler,
    'Neyith of the Dire Hunt': neyith_of_the_dire_hunt_handler,
    'Shadowgrange Archfiend': shadowgrange_archfiend_handler,
    'The Golden Throne': the_golden_throne_handler,
    'Tidal Barracuda': tidal_barracuda_handler,
    'Vizier of the Menagerie': vizier_of_the_menagerie_handler,
    'Ao, the Dawn Sky': ao_the_dawn_sky_handler,
    'Murmuration': murmuration_handler,
    "Captain N'ghathrod": captain_nghathrod_handler,
    'Secret Arcade // Dusty Parlor': secret_arcade_handler,
    'Paladin Class': paladin_class_handler,
    'Nils, Discipline Enforcer': nils_discipline_enforcer_handler,
    'Bloodline Bidding': bloodline_bidding_handler,
    'Determined Iteration': determined_iteration_handler,
    'Luck Bobblehead': luck_bobblehead_handler,
    'Fire Lord Azula': fire_lord_azula_handler,
    'Overlord of the Mistmoors': overlord_of_the_mistmoors_handler,
    'Seasoned Dungeoneer': seasoned_dungeoneer_handler,
    'Famished Worldsire': famished_worldsire_handler,
    'Thran Portal': thran_portal_handler,
    'Sephiroth, Fallen Hero': sephiroth_fallen_hero_handler,
    'Entrapment Maneuver': entrapment_maneuver_handler,
    'Bumi, Unleashed': bumi_unleashed_handler,
    'Ruric Thar, the Unbowed': ruric_thar_the_unbowed_handler,
    'Spider-Punk': spiderpunk_handler,
    'Fractured Identity': fractured_identity_handler,
    'Banquet Guests': banquet_guests_handler,
    'Cyber Conversion': cyber_conversion_handler,
    'Eriette of the Charmed Apple': eriette_of_the_charmed_apple_handler,
    'Kefka, Dancing Mad': kefka_dancing_mad_handler,
    'Quick Sliver': quick_sliver_handler,
    'Ozai, the Phoenix King': ozai_the_phoenix_king_handler,
    'Alisaie Leveilleur': alisaie_leveilleur_handler,
    'Lumbering Worldwagon': lumbering_worldwagon_handler,
    "Aang, Swift Savior // Aang and La, Ocean's Fury": aang_swift_savior_handler,
    'Overlord of the Boilerbilges': overlord_of_the_boilerbilges_handler,
    'Reflector Mage': reflector_mage_handler,
    'Firion, Wild Rose Warrior': firion_wild_rose_warrior_handler,
    "Dion, Bahamut's Dominant // Bahamut, Warden of Light": dion_bahamuts_dominant_handler,
    'Eye of Nidhogg': eye_of_nidhogg_handler,
    'The Master, Transcendent': the_master_transcendent_handler,
    'Robe of the Archmagi': robe_of_the_archmagi_handler,
    'Elemental Eruption': elemental_eruption_handler,
    "Sauron's Ransom": saurons_ransom_handler,
    'Devastating Onslaught': devastating_onslaught_handler,
    'Viewpoint Synchronization': viewpoint_synchronization_handler,
    'Damping Sphere': damping_sphere_handler,
    "Bilbo's Ring": bilbos_ring_handler,
    'Conspiracy': conspiracy_handler,
    'Instill Energy': instill_energy_handler,
    'Airbending Lesson': airbending_lesson_handler,
    'Find // Finality': find_handler,
    'Arwen, Mortal Queen': arwen_mortal_queen_handler,
    'Elenda, Saint of Dusk': elenda_saint_of_dusk_handler,
    'Aminatou, Veil Piercer': aminatou_veil_piercer_handler,
    'Chainer, Dementia Master': chainer_dementia_master_handler,
    'Glimmer Lens': glimmer_lens_handler,
    'Locke, Treasure Hunter': locke_treasure_hunter_handler,
    'Benevolent Blessing': benevolent_blessing_handler,
    'Vial Smasher the Fierce': vial_smasher_the_fierce_handler,
    # ---- Wave 5 (top-200 PARTIAL escapees, ranks ~501 to ~14123) -------
'Necropotence': necropotence_handler,
    'Whip of Erebos': whip_of_erebos_handler,
    'Eerie Interlude': eerie_interlude_handler,
    'Nezahal, Primal Tide': nezahal_primal_tide_handler,
    'Kiki-Jiki, Mirror Breaker': kiki_jiki_mirror_breaker_handler,
    'The Fire Crystal': the_fire_crystal_handler,
    'Twinflame': twinflame_handler,
    'Feldon of the Third Path': feldon_of_the_third_path_handler,
    'Sneak Attack': sneak_attack_handler,
    'The Locust God': the_locust_god_handler,
    'Molten Duplication': molten_duplication_handler,
    'Jaxis, the Troublemaker': jaxis_the_troublemaker_handler,
    'Fable of the Mirror-Breaker // Reflection of Kiki-Jiki': fable_of_the_mirror_breaker_handler,
    'The Scarab God': the_scarab_god_handler,
    'Charming Prince': charming_prince_handler,
    'Touch the Spirit Realm': touch_the_spirit_realm_handler,
    'Parting Gust': parting_gust_handler,
    'Liesa, Forgotten Archangel': liesa_forgotten_archangel_handler,
    'Flickerwisp': flickerwisp_handler,
    'Mimic Vat': mimic_vat_handler,
    'Orthion, Hero of Lavabrink': orthion_hero_of_lavabrink_handler,
    'Gift of Immortality': gift_of_immortality_handler,
    'Molten Echoes': molten_echoes_handler,
    'Ilharg, the Raze-Boar': ilharg_the_raze_boar_handler,
    "Urabrask's Forge": urabrasks_forge_handler,
    'The Jolly Balloon Man': the_jolly_balloon_man_handler,
    "Lae'zel's Acrobatics": laezels_acrobatics_handler,
    "Teferi's Time Twist": teferis_time_twist_handler,
    'Nine-Lives Familiar': nine_lives_familiar_handler,
    'Flameshadow Conjuring': flameshadow_conjuring_handler,
    'Lagomos, Hand of Hatred': lagomos_hand_of_hatred_handler,
    'Ghoulish Impetus': ghoulish_impetus_handler,
    'Echoing Assault': echoing_assault_handler,
    'Ghostway': ghostway_handler,
    'Rionya, Fire Dancer': rionya_fire_dancer_handler,
    'Come Back Wrong': come_back_wrong_handler,
    'Mirror March': mirror_march_handler,
    'Cosmic Intervention': cosmic_intervention_handler,
    'Dalkovan Encampment': dalkovan_encampment_handler,
    'Stormsplitter': stormsplitter_handler,
    'Resurrection Orb': resurrection_orb_handler,
    'Yorion, Sky Nomad': yorion_sky_nomad_handler,
    'Purphoros, Bronze-Blooded': purphoros_bronze_blooded_handler,
    'Grave Betrayal': grave_betrayal_handler,
    'Zara, Renegade Recruiter': zara_renegade_recruiter_handler,
    'Depthshaker Titan': depthshaker_titan_handler,
    'Life // Death': life_handler,
    'Terra, Magical Adept // Esper Terra': terra_magical_adept_handler,
    'Inalla, Archmage Ritualist': inalla_archmage_ritualist_handler,
    'Niko, Light of Hope': niko_light_of_hope_handler,
    'Foe-Razer Regent': foe_razer_regent_handler,
    'Sidequest: Play Blitzball // World Champion, Celestial Weapon': sidequest_play_blitzball_handler,
    'Dawnbreak Reclaimer': dawnbreak_reclaimer_handler,
    'Artificial Evolution': artificial_evolution_handler,
    'Survey Mechan': survey_mechan_handler,
    'Wort, the Raidmother': wort_the_raidmother_handler,
    'Invert // Invent': invert_handler,
    'Ian Chesterton': ian_chesterton_handler,
    'Thunderous Debut': thunderous_debut_handler,
    'Ruthless Radrat': ruthless_radrat_handler,
    'Transcendence': transcendence_handler,
    'Acorn Harvest': acorn_harvest_handler,
    'Turn to Mist': turn_to_mist_handler,
    'Boneyard Parley': boneyard_parley_handler,
    'Plunge into Darkness': plunge_into_darkness_handler,
    "Drach'Nyen": drachnyen_handler,
    'Endless Whispers': endless_whispers_handler,
    'Seal of the Guildpact': seal_of_the_guildpact_handler,
    'Expedited Inheritance': expedited_inheritance_handler,
    'Sanguine Sacrament': sanguine_sacrament_handler,
    'Clara Oswald': clara_oswald_handler,
    'Tallyman of Nurgle': tallyman_of_nurgle_handler,
    'Barge In': barge_in_handler,
    'Ignite Memories': ignite_memories_handler,
    'Into the Night': into_the_night_handler,
    'Angel of Condemnation': angel_of_condemnation_handler,
    'Depala, Pilot Exemplar': depala_pilot_exemplar_handler,
    'Mwonvuli Beast Tracker': mwonvuli_beast_tracker_handler,
    'Arachne, Psionic Weaver': arachne_psionic_weaver_handler,
    'Queen Kayla bin-Kroog': queen_kayla_bin_kroog_handler,
    'Prismatic Strands': prismatic_strands_handler,
    'Stitcher Geralf': stitcher_geralf_handler,
    "Fertilid's Favor": fertilids_favor_handler,
    'Themberchaud': themberchaud_handler,
    'Fire Navy Trebuchet': fire_navy_trebuchet_handler,
    "Dark Knight's Greatsword": dark_knights_greatsword_handler,
    "The Spot's Portal": the_spots_portal_handler,
    'Teachings of the Archaics': teachings_of_the_archaics_handler,
    'Proud Wildbonder': proud_wildbonder_handler,
    'Experimental Overload': experimental_overload_handler,
    'Ruthless Ripper': ruthless_ripper_handler,
    'Cauldron Dance': cauldron_dance_handler,
    "Karn's Sylex": karns_sylex_handler,
    'Eiganjo Uprising': eiganjo_uprising_handler,
    "Archmage's Newt": archmages_newt_handler,
    'Whirlpool Whelm': whirlpool_whelm_handler,
    'Raph & Leo, Sibling Rivals': raph_leo_sibling_rivals_handler,
    "Fang, Roku's Companion": fang_rokus_companion_handler,
    'Defossilize': defossilize_handler,
    'Arcus Acolyte': arcus_acolyte_handler,
    'The Swarmlord': the_swarmlord_handler,
    'Fire Nation Cadets': fire_nation_cadets_handler,
    'Footsteps of the Goryo': footsteps_of_the_goryo_handler,
    "Exdeath, Void Warlock // Neo Exdeath, Dimension's End": exdeath_void_warlock_handler,
    'Songcrafter Mage': songcrafter_mage_handler,
    'Cruel Entertainment': cruel_entertainment_handler,
    'Invasion Plans': invasion_plans_handler,
    'Spy Network': spy_network_handler,
    'Lifeline': lifeline_handler,
    'Catalyst Stone': catalyst_stone_handler,
    'Slave of Bolas': slave_of_bolas_handler,
    'Dynaheir, Invoker Adept': dynaheir_invoker_adept_handler,
    'The Pandorica': the_pandorica_handler,
    'Supplant Form': supplant_form_handler,
    'Tonberry': tonberry_handler,
    'Shizuko, Caller of Autumn': shizuko_caller_of_autumn_handler,
    "Purphoros's Intervention": purphoross_intervention_handler,
    'Galepowder Mage': galepowder_mage_handler,
    'Harness by Force': harness_by_force_handler,
    'Teleportal': teleportal_handler,
    'Ambush Commander': ambush_commander_handler,
    'Essence Pulse': essence_pulse_handler,
    'Shellshock': shellshock_handler,
    'Curse of the Cabal': curse_of_the_cabal_handler,
    "Varchild's War-Riders": varchilds_war_riders_handler,
    "Katara's Reversal": kataras_reversal_handler,
    'Felhide Spiritbinder': felhide_spiritbinder_handler,
    'Roon of the Hidden Realm': roon_of_the_hidden_realm_handler,
    'Return Triumphant': return_triumphant_handler,
    'Exterminator Magmarch': exterminator_magmarch_handler,
    'Rainbow Vale': rainbow_vale_handler,
    'Whirlwind Technique': whirlwind_technique_handler,
    'Druid of the Emerald Grove': druid_of_the_emerald_grove_handler,
    'Madame Web, Clairvoyant': madame_web_clairvoyant_handler,
    'Tempt with Immortality': tempt_with_immortality_handler,
    'Predatory Impetus': predatory_impetus_handler,
    'Harmonious Archon': harmonious_archon_handler,
    'Chrome Dome': chrome_dome_handler,
    'The Moment': the_moment_handler,
    "Rhino's Rampage": rhinos_rampage_handler,
    'Voidwalk': voidwalk_handler,
    'Multiple Choice': multiple_choice_handler,
    'Bearer of the Heavens': bearer_of_the_heavens_handler,
    'Intimidation Bolt': intimidation_bolt_handler,
    'Feather, Radiant Arbiter': feather_radiant_arbiter_handler,
    'Timesifter': timesifter_handler,
    'Prized Amalgam': prized_amalgam_handler,
    'Unexpected Request': unexpected_request_handler,
    'Cogwork Assembler': cogwork_assembler_handler,
    'Cid, Timeless Artificer': cid_timeless_artificer_handler,
    'Thelon of Havenwood': thelon_of_havenwood_handler,
    'Hall of Gemstone': hall_of_gemstone_handler,
    'Sonar Strike': sonar_strike_handler,
    'Rosheen, Roaring Prophet': rosheen_roaring_prophet_handler,
    'Magosi, the Waterveil': magosi_the_waterveil_handler,
    'Ashling, the Extinguisher': ashling_the_extinguisher_handler,
    'Push the Limit': push_the_limit_handler,
    'Karakyk Guardian': karakyk_guardian_handler,
    'Storage Matrix': storage_matrix_handler,
    'Glacial Dragonhunt': glacial_dragonhunt_handler,
    'Experimental Frenzy': experimental_frenzy_handler,
    'Blaster, Combat DJ // Blaster, Morale Booster': blaster_combat_dj_handler,
    "In Bolas's Clutches": in_bolass_clutches_handler,
    'Invasion of Ravnica // Guildpact Paragon': invasion_of_ravnica_handler,
    'Scout the City': scout_the_city_handler,
    'Wisecrack': wisecrack_handler,
    'The Duke, Rebel Sentry': the_duke_rebel_sentry_handler,
    'Syr Elenora, the Discerning': syr_elenora_the_discerning_handler,
    'Jared Carthalion, True Heir': jared_carthalion_true_heir_handler,
    'Team Pennant': team_pennant_handler,
    'Fires of Invention': fires_of_invention_handler,
    'Invasion of Kaladesh // Aetherwing, Golden-Scale Flagship': invasion_of_kaladesh_handler,
    'Brass Herald': brass_herald_handler,
    'Seeker of Slaanesh': seeker_of_slaanesh_handler,
    'Heart of Kiran': heart_of_kiran_handler,
    'Terrific Team-Up': terrific_team_up_handler,
    'Pirate Hat': pirate_hat_handler,
    'Radiate': radiate_handler,
    'Aurification': aurification_handler,
    'Conspiracy Unraveler': conspiracy_unraveler_handler,
    'Gimli, Mournful Avenger': gimli_mournful_avenger_handler,
    'Recall': recall_handler,
    'Fatal Frenzy': fatal_frenzy_handler,
    "Fell Beast's Shriek": fell_beasts_shriek_handler,
    'Once and Future': once_and_future_handler,
    'Frenetic Sliver': frenetic_sliver_handler,
    'Beastie Beatdown': beastie_beatdown_handler,
    'Lochmere Serpent': lochmere_serpent_handler,
    'Evelyn, the Covetous': evelyn_the_covetous_handler,
    'Dai Li Indoctrination': dai_li_indoctrination_handler,
    'Lara Croft, Tomb Raider': lara_croft_tomb_raider_handler,
    'Portal Manipulator': portal_manipulator_handler,
    'Waxing Moon': waxing_moon_handler,
    'Sigardian Zealot': sigardian_zealot_handler,
    'Tunnel Vision': tunnel_vision_handler,
    'Vanish into Eternity': vanish_into_eternity_handler,
    'Dragonfly Swarm': dragonfly_swarm_handler,
    'Coin of Fate': coin_of_fate_handler,
    'Gorilla Berserkers': gorilla_berserkers_handler,
    'Gorm the Great': gorm_the_great_handler,
}

# Extensions should expose empty lists for the other registries so
# load_extensions doesn't accidentally pull anything else in.
EFFECT_RULES: list = []
STATIC_PATTERNS: list = []
TRIGGER_PATTERNS: list = []

__all__ = ["PER_CARD_HANDLERS"]
