"""Spot-check tests for Wave 1a parser phrase promotions.

Each test below exercises one of the promoted phrase families from
``scripts/extensions/a_wave1a_promotions.py``. These are NOT golden-file
tests — they assert structural AST properties (kind, counts, target
filter base) so a future parser refactor that preserves semantics but
shifts signatures keeps passing.

Pick-set philosophy: one representative card per promoted shape, so a
breakage triggers a specific red signal (e.g. the dual-mana rule broke,
or the Optional_(Draw) rule broke) rather than one big blob.
"""

from __future__ import annotations

import sys
from pathlib import Path

import pytest

_ROOT = Path(__file__).resolve().parents[1]
_SCRIPTS = _ROOT / "scripts"
sys.path.insert(0, str(_SCRIPTS))

import parser as _parser_module  # noqa: E402

_parser_module.load_extensions()
from parser import parse_effect  # noqa: E402

from mtg_ast import (  # noqa: E402
    AddMana,
    Bounce,
    Choice,
    Conditional,
    CreateToken,
    Damage,
    Destroy,
    Draw,
    Fight,
    GrantAbility,
    LoseGame,
    Mill,
    Modification,
    Optional_,
    Reanimate,
    Recurse,
    Reveal,
    Sacrifice,
    Sequence,
    Tutor,
    UnknownEffect,
)


# ----------------------------------------------------------------------
# Group A — token creation
# ----------------------------------------------------------------------

def test_create_tapped_token_with_keywords():
    """Lingering Souls tail: ``create a tapped 1/1 white spirit token
    with flying``. Promoted rule must capture color, keyword, tapped."""
    e = parse_effect(
        "create a tapped 1/1 white spirit creature token with flying"
    )
    assert isinstance(e, CreateToken)
    assert e.count == 1
    assert e.pt == (1, 1)
    assert e.color == ("W",)
    assert "spirit" in e.types
    assert "flying" in e.keywords
    assert e.tapped is True


def test_create_typed_token_with_keywords_no_tapped():
    """Rukh Egg tail: ``create a 4/4 red dragon creature token with
    flying``. Non-tapped variant with keywords — must fire through new
    rule (not the built-in bare rule)."""
    e = parse_effect("create a 4/4 red dragon creature token with flying")
    assert isinstance(e, CreateToken)
    assert e.pt == (4, 4)
    assert e.color == ("R",)
    assert "dragon" in e.types
    assert "flying" in e.keywords


def test_create_token_copy_of_that_creature():
    e = parse_effect("create a token that's a copy of that creature")
    assert isinstance(e, CreateToken)
    assert e.is_copy_of is not None
    assert e.is_copy_of.base == "that_creature"


# ----------------------------------------------------------------------
# Group B — mana choice (dual / triple)
# ----------------------------------------------------------------------

def test_add_two_color_choice():
    """Timber Gorge's ``Add {R} or {G}``: Choice(pick=1, [AddMana(R),
    AddMana(G)])."""
    e = parse_effect("add {r} or {g}")
    assert isinstance(e, Choice)
    assert e.pick == 1
    assert len(e.options) == 2
    for opt in e.options:
        assert isinstance(opt, AddMana)
        assert len(opt.pool) == 1
    colors = {opt.pool[0].color[0] for opt in e.options}
    assert colors == {"R", "G"}


def test_add_three_color_choice():
    """Arcane Sanctum's ``Add {W}, {U}, or {B}``."""
    e = parse_effect("add {w}, {u}, or {b}")
    assert isinstance(e, Choice)
    assert e.pick == 1
    assert len(e.options) == 3
    colors = {opt.pool[0].color[0] for opt in e.options}
    assert colors == {"W", "U", "B"}


# ----------------------------------------------------------------------
# Group C — you lose the game
# ----------------------------------------------------------------------

def test_you_lose_the_game():
    e = parse_effect("you lose the game")
    assert isinstance(e, LoseGame)


# ----------------------------------------------------------------------
# Group D — each-player verbs
# ----------------------------------------------------------------------

def test_each_player_mill():
    e = parse_effect("each player mills three cards")
    assert isinstance(e, Mill)
    assert e.count == 3
    assert e.target.base == "player"
    assert e.target.quantifier == "each"


def test_each_player_reveal_top():
    e = parse_effect("each player reveals the top card of their library")
    assert isinstance(e, Reveal)
    assert e.actor == "each_player"
    assert e.source == "top_of_library"


# ----------------------------------------------------------------------
# Group E — Optional_(<typed>) promotions
# ----------------------------------------------------------------------

def test_may_draw_card():
    e = parse_effect("you may draw a card")
    assert isinstance(e, Optional_)
    assert isinstance(e.body, Draw)
    assert e.body.count == 1


def test_may_draw_two_cards():
    e = parse_effect("you may draw two cards")
    assert isinstance(e, Optional_)
    assert isinstance(e.body, Draw)
    assert e.body.count == 2


def test_may_destroy_target_art_or_enc():
    e = parse_effect("you may destroy target artifact or enchantment")
    assert isinstance(e, Optional_)
    assert isinstance(e.body, Destroy)
    assert e.body.target.base == "artifact_or_enchantment"


def test_may_recurse_creature_card():
    e = parse_effect(
        "you may return target creature card from your graveyard to your hand"
    )
    assert isinstance(e, Optional_)
    assert isinstance(e.body, Recurse)
    assert e.body.query.base == "creature"
    assert e.body.from_zone == "your_graveyard"
    assert e.body.destination == "hand"


def test_may_bounce_creature():
    e = parse_effect("you may return target creature to its owner's hand")
    assert isinstance(e, Optional_)
    assert isinstance(e.body, Bounce)


def test_may_search_basic_land():
    e = parse_effect(
        "you may search your library for a basic land card, put it onto the battlefield tapped, then shuffle"
    )
    assert isinstance(e, Optional_)
    assert isinstance(e.body, Tutor)
    assert e.body.query.base == "basic_land_card"
    assert e.body.destination == "battlefield_tapped"


# ----------------------------------------------------------------------
# Group H — this-creature effects
# ----------------------------------------------------------------------

def test_this_creature_damage_each_opponent():
    e = parse_effect("this creature deals 2 damage to each opponent")
    assert isinstance(e, Damage)
    assert e.amount == 2
    assert e.target.base == "opponent"
    assert e.target.quantifier == "each"


def test_this_creature_phases_out():
    e = parse_effect("this creature phases out")
    assert isinstance(e, Modification)
    assert e.kind == "phase_out_self"


def test_it_fights_up_to_one_opp():
    e = parse_effect("it fights up to one target creature you don't control")
    assert isinstance(e, Fight)
    assert e.b.opponent_controls is True
    assert e.b.quantifier == "up_to_n"


# ----------------------------------------------------------------------
# Group I — self keyword grants EOT
# ----------------------------------------------------------------------

def test_self_gain_flying_eot():
    e = parse_effect("~ gains flying until end of turn")
    assert isinstance(e, GrantAbility)
    assert e.ability_name == "flying"
    assert e.duration == "until_end_of_turn"


def test_self_gain_first_strike_eot():
    e = parse_effect("~ gains first strike until end of turn")
    assert isinstance(e, GrantAbility)
    assert e.ability_name == "first strike"


# ----------------------------------------------------------------------
# Group K — conditional effects
# ----------------------------------------------------------------------

def test_if_power_ge_draw():
    e = parse_effect(
        "if you control a creature with power 4 or greater, draw a card"
    )
    assert isinstance(e, Conditional)
    assert e.condition.kind == "you_control_creature_power_ge"
    assert e.condition.args == (4,)
    assert isinstance(e.body, Draw)


def test_if_creature_died_p1p1():
    e = parse_effect(
        "if a creature died this turn, put a +1/+1 counter on this creature"
    )
    assert isinstance(e, Conditional)
    assert e.condition.kind == "creature_died_this_turn"


# ----------------------------------------------------------------------
# Group L — misc
# ----------------------------------------------------------------------

def test_those_creatures_fight_each_other():
    e = parse_effect("those creatures fight each other")
    assert isinstance(e, Modification)
    assert e.kind == "fight_each_other"


def test_it_doesnt_untap_controller_next():
    e = parse_effect("it doesn't untap during its controller's next untap step")
    assert isinstance(e, Modification)
    assert e.kind == "stun_target_next_untap"


def test_reveal_top_put_hand():
    """Dark Confidant / Dark Tutelage: promoted to Sequence(Reveal, put_card_into_hand)."""
    e = parse_effect(
        "reveal the top card of your library and put that card into your hand"
    )
    assert isinstance(e, Sequence)
    assert len(e.items) == 2
    assert isinstance(e.items[0], Reveal)


def test_return_exiled_card_battlefield():
    e = parse_effect(
        "return the exiled card to the battlefield under its owner's control"
    )
    assert isinstance(e, Reanimate)
    assert e.from_zone == "exile"
    assert e.destination == "battlefield"


def test_goad_target_that_player():
    e = parse_effect("goad target creature that player controls")
    assert isinstance(e, Modification)
    assert e.kind == "goad"


def test_investigate_twice():
    e = parse_effect("investigate twice")
    assert isinstance(e, Modification)
    assert e.kind == "investigate"
    assert e.args == (2,)


# ----------------------------------------------------------------------
# Regression guard: promoted phrases must NOT be UnknownEffect anymore.
# ----------------------------------------------------------------------

_PROMOTED_SHOULD_NOT_BE_UNKNOWN = [
    "add {r} or {g}",
    "add {u}, {b}, or {r}",
    "you lose the game",
    "you may draw a card",
    "you may draw two cards",
    "you may destroy target artifact or enchantment",
    "you may destroy target artifact",
    "you may return target creature card from your graveyard to your hand",
    "you may return target card from your graveyard to your hand",
    "you may return target creature to its owner's hand",
    "you may return this creature to its owner's hand",
    "you may sacrifice a creature",
    "you may return target enchantment card from your graveyard to the battlefield",
    "each player mills three cards",
    "each player reveals the top card of their library",
    "each other player sacrifices a creature of their choice",
    "this creature deals 2 damage to each opponent",
    "this creature phases out",
    "this creature loses defender until end of turn",
    "this creature can attack this turn as though it didn't have defender",
    "create a tapped 1/1 white spirit creature token with flying",
    "create a 4/4 red dragon creature token with flying",
    "create a token that's a copy of that creature",
    "~ gains flying until end of turn",
    "~ gains first strike until end of turn",
    "it fights up to one target creature you don't control",
    "those creatures fight each other",
    "it doesn't untap during its controller's next untap step",
    "you may search your library for a basic land card, reveal it, put it into your hand, then shuffle",
    "you may search your library for a basic land card, put it onto the battlefield tapped, then shuffle",
    "you may search your library for a card named ~, reveal it, put it into your hand, then shuffle",
    "if a creature died this turn, put a +1/+1 counter on this creature",
    "if you control a creature with power 4 or greater, draw a card",
    "return the exiled card to the battlefield under its owner's control",
    "return that card to the battlefield",
    "goad target creature that player controls",
    "heist target opponent's library",
    "suspect it",
    "it becomes plotted",
    "investigate twice",
    "copy that ability",
    "reveal the top card of your library and put that card into your hand",
]


@pytest.mark.parametrize("phrase", _PROMOTED_SHOULD_NOT_BE_UNKNOWN)
def test_phrase_is_not_unknown(phrase):
    e = parse_effect(phrase)
    assert e is not None, f"parse_effect returned None for {phrase!r}"
    assert not isinstance(e, UnknownEffect), (
        f"Phrase {phrase!r} still parses as UnknownEffect "
        f"(expected a typed node). "
        f"Got raw_text={e.raw_text!r}"
    )
