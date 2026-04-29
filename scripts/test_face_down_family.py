"""Face-down family regression tests.

Covers Morph / Megamorph / Manifest / Manifest-Dread / Cloak / Disguise /
Cybermen — all the mechanics defined by CR §702.37, §702.37b, §701.40,
§701.62, §701.58, §702.168 + the custom Cybermen keyword. The common
thread is CR §708: face-down permanents have the characteristics assigned
by the ability or rule that allowed them to be face down.

Tests are written for pytest and can be run stand-alone:
    python3 -m pytest scripts/test_face_down_family.py -v

They use the hand-rolled fixture style established in
tests/test_engine_layers_and_durations.py (no oracle dump required).
"""

from __future__ import annotations

import sys
from pathlib import Path

import pytest

# Ensure scripts/ is importable even when pytest is invoked from the repo
# root. conftest.py handles this for tests/ but not for scripts/-hosted
# tests; we replicate the path setup here.
_HERE = Path(__file__).resolve().parent
if str(_HERE) not in sys.path:
    sys.path.insert(0, str(_HERE))

import parser as mtg_parser  # noqa: E402
# Load the ~50 extensions so the face-down family's EFFECT_RULES /
# STATIC_PATTERNS / POST_PARSE_HOOKS are registered before any card parses.
mtg_parser.load_extensions()

import playloop  # noqa: E402
from mtg_ast import (  # noqa: E402
    CardAST, Keyword, ManaCost, ManaSymbol, Modification, Static,
    FACE_DOWN_VANILLA, FACE_DOWN_WARD_2, FaceDownCharacteristics,
    TurnFaceUp, Cost,
)
from parser import parse_card  # noqa: E402
from playloop import (  # noqa: E402
    CardEntry, Game, Permanent, Seat,
    cast_face_down, manifest, cloak, manifest_dread, turn_face_up,
    face_down_characteristics,
    get_effective_characteristics,
)


# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------


def _empty_game() -> Game:
    seats = [Seat(idx=0), Seat(idx=1)]
    g = Game(seats=seats)
    g.set_phase_step("precombat_main", "", legacy_phase="main1")
    return g


def _card(name: str, type_line: str, *,
          power=None, toughness=None, cmc=0, colors=(),
          mana_cost: str = "",
          abilities=(),
          morph_cost: "ManaCost | None" = None,
          disguise_cost: "ManaCost | None" = None,
          has_morph: bool = False,
          has_megamorph: bool = False,
          has_disguise: bool = False) -> CardEntry:
    ast = CardAST(
        name=name, abilities=tuple(abilities),
        parse_errors=(), fully_parsed=True,
        morph_cost=morph_cost,
        disguise_cost=disguise_cost,
        has_morph=has_morph,
        has_megamorph=has_megamorph,
        has_disguise=has_disguise,
    )
    return CardEntry(
        name=name, mana_cost=mana_cost, cmc=cmc, type_line=type_line,
        oracle_text="", power=power, toughness=toughness, ast=ast,
        colors=tuple(colors),
    )


def _mc(raw: str) -> ManaCost:
    return playloop._parse_mana_cost_str(raw)


# ===========================================================================
# Part A — AST schema + parser extension tests
# ===========================================================================


class TestASTSchema:
    def test_face_down_characteristics_defaults(self):
        """CR §708.2a: face-down is a 2/2 nameless colorless creature with
        no text, no subtypes, no mana cost."""
        fd = FACE_DOWN_VANILLA
        assert fd.power == 2
        assert fd.toughness == 2
        assert fd.name == ""
        assert fd.colors == ()
        assert fd.subtypes == ()
        assert fd.abilities == ()

    def test_face_down_ward_2_variant(self):
        """CR §702.168a / §701.58a: Disguise and Cloak both grant the
        face-down permanent ward {2}."""
        fd = FACE_DOWN_WARD_2
        assert fd.power == 2
        assert fd.toughness == 2
        assert "ward_2" in fd.abilities

    def test_face_down_characteristics_hashable(self):
        """FaceDownCharacteristics must be hashable so AST signature
        comparison and deduplication work."""
        assert hash(FACE_DOWN_VANILLA)
        assert hash(FACE_DOWN_WARD_2)
        s = {FACE_DOWN_VANILLA, FACE_DOWN_WARD_2}
        assert len(s) == 2

    def test_turn_face_up_node_defaults(self):
        """TurnFaceUp is the typed special-action effect."""
        tfu = TurnFaceUp()
        assert tfu.kind == "turn_face_up"
        assert tfu.cost is None
        assert tfu.megamorph is False

    def test_card_ast_morph_cost_default(self):
        """CardAST stays backward-compatible — morph_cost defaults None."""
        c = CardAST(name="test")
        assert c.morph_cost is None
        assert c.disguise_cost is None
        assert c.has_morph is False
        assert c.manifest_token is False


class TestParserExtension:
    """Validate the face_down_family parser extension emits the right
    ast enrichment for oracle-corpus cards (if available) OR via hand-
    crafted strings."""

    def test_morph_cost_extracted_for_morph_card(self):
        """Willbender-style card (Morph {1}{U}) ends up with morph_cost
        populated on its CardAST."""
        card = {
            "name": "Test Morph Card",
            "oracle_text": "Morph {1}{U}",
            "type_line": "Creature — Human",
            "mana_cost": "{2}{U}",
            "power": 2,
            "toughness": 2,
        }
        ast = parse_card(card)
        assert ast.has_morph is True
        assert ast.morph_cost is not None
        assert ast.morph_cost.cmc == 2  # {1}{U}

    def test_megamorph_flag_set(self):
        """Den Protector-style card gets has_megamorph=True AND
        has_morph=True (per CR §702.37b: megamorph is a variant of
        morph)."""
        card = {
            "name": "Test Megamorph",
            "oracle_text": "Megamorph {1}{G}",
            "type_line": "Creature — Human Druid",
            "mana_cost": "{1}{G}",
            "power": 2,
            "toughness": 1,
        }
        ast = parse_card(card)
        assert ast.has_megamorph is True
        assert ast.has_morph is True
        assert ast.morph_cost is not None
        assert ast.morph_cost.cmc == 2

    def test_disguise_cost_extracted(self):
        """Boltbender-style card (Disguise {1}{R}) populates
        disguise_cost and has_disguise flag."""
        card = {
            "name": "Test Disguise",
            "oracle_text": "Disguise {1}{R}",
            "type_line": "Creature — Human",
            "mana_cost": "{3}{R}",
            "power": 3,
            "toughness": 2,
        }
        ast = parse_card(card)
        assert ast.has_disguise is True
        assert ast.disguise_cost is not None
        assert ast.disguise_cost.cmc == 2

    def test_face_down_copy_effect_static_added(self):
        """Every morph/disguise card gains a face_down_copy_effect Static
        ability (the §613.2b layer-1b override marker)."""
        card = {
            "name": "Test Morph Card 2",
            "oracle_text": "Morph {B}",
            "type_line": "Creature",
            "mana_cost": "{2}{B}",
            "power": 3,
            "toughness": 3,
        }
        ast = parse_card(card)
        found = False
        for ab in ast.abilities:
            if (isinstance(ab, Static) and ab.modification
                    and ab.modification.kind == "face_down_copy_effect"):
                found = True
                # Variant should be "vanilla" (morph, not disguise)
                assert ab.modification.args[0] == "vanilla"
                assert ab.modification.layer == "1"
        assert found, "face_down_copy_effect Static must be emitted for morph"

    def test_disguise_emits_ward_2_variant(self):
        """Disguise cards get variant="ward_2" per CR §702.168a."""
        card = {
            "name": "Test Disguise Card",
            "oracle_text": "Disguise {1}{R}",
            "type_line": "Creature",
            "mana_cost": "{2}{R}",
            "power": 2,
            "toughness": 3,
        }
        ast = parse_card(card)
        for ab in ast.abilities:
            if (isinstance(ab, Static) and ab.modification
                    and ab.modification.kind == "face_down_copy_effect"):
                assert ab.modification.args[0] == "ward_2"
                return
        pytest.fail("expected face_down_copy_effect Static on disguise card")

    def test_manifest_action_tagged(self):
        """A spell that manifests the top card of the library gets
        manifest_token=True on its CardAST."""
        card = {
            "name": "Test Soul Summons",
            "oracle_text": "Manifest the top card of your library.",
            "type_line": "Sorcery",
            "mana_cost": "{1}{W}",
            "power": None,
            "toughness": None,
        }
        ast = parse_card(card)
        assert ast.manifest_token is True, (
            "cards that produce face-down permanents must flag manifest_token"
        )

    def test_vanilla_card_untouched(self):
        """A plain non-face-down card is unchanged by the enrichment hook."""
        card = {
            "name": "Grizzly Bears",
            "oracle_text": "",
            "type_line": "Creature — Bear",
            "mana_cost": "{1}{G}",
            "power": 2,
            "toughness": 2,
        }
        ast = parse_card(card)
        assert ast.morph_cost is None
        assert ast.disguise_cost is None
        assert ast.has_morph is False
        assert ast.manifest_token is False
        # No face_down_copy_effect Static added
        for ab in ast.abilities:
            if isinstance(ab, Static) and ab.modification:
                assert ab.modification.kind != "face_down_copy_effect"


# ===========================================================================
# Part B — Engine face-down state + §613 integration
# ===========================================================================


class TestFaceDownCharacteristics:
    def test_face_down_returns_2_2_vanilla(self):
        """CR §708.2a: face-down vanilla is 2/2 with no name/subtypes."""
        g = _empty_game()
        true_card = _card("Ainok Survivalist Stand-In",
                          "Creature — Human Scout",
                          power=3, toughness=2, cmc=3,
                          morph_cost=_mc("{1}{G}"),
                          has_morph=True, has_megamorph=True)
        g.seats[0].hand.append(true_card)
        fd = cast_face_down(g, 0, true_card,
                            flip_cost=_mc("{1}{G}"),
                            variant="vanilla",
                            is_megamorph=True)
        chars = face_down_characteristics(fd)
        assert chars["power"] == 2
        assert chars["toughness"] == 2
        assert chars["name"] == ""
        assert chars["colors"] == []
        assert chars["abilities"] == []
        # subtypes empty = no Human, no Scout
        assert chars["subtypes"] == []

    def test_face_down_returns_ward_2_for_disguise(self):
        """CR §702.168a: face-down disguise permanent has ward {2}."""
        g = _empty_game()
        true_card = _card("Disguised Card",
                          "Creature — Human", power=3, toughness=3,
                          cmc=4, disguise_cost=_mc("{1}{R}"),
                          has_disguise=True)
        g.seats[0].hand.append(true_card)
        fd = cast_face_down(g, 0, true_card,
                            flip_cost=_mc("{1}{R}"),
                            variant="ward_2",
                            is_megamorph=False)
        chars = face_down_characteristics(fd)
        assert chars["power"] == 2
        assert chars["toughness"] == 2
        assert "ward_2" in chars["abilities"]

    def test_layer_1_override_replaces_true_characteristics(self):
        """§613.2b: layer-1b override replaces ALL copiable values on the
        face-down permanent. get_effective_characteristics reports the
        override, NOT the true card's 7/7 red abilities."""
        g = _empty_game()
        akroma = _card("Akroma Stand-In",
                       "Legendary Creature — Angel",
                       power=6, toughness=6, cmc=6, colors=("R",),
                       morph_cost=_mc("{3}{R}{R}{R}"),
                       has_morph=True)
        g.seats[0].hand.append(akroma)
        fd = cast_face_down(g, 0, akroma,
                            flip_cost=_mc("{3}{R}{R}{R}"),
                            variant="vanilla", is_megamorph=False)
        chars = get_effective_characteristics(g, fd)
        # §708.2a: 2/2, no name, no color
        assert chars["power"] == 2
        assert chars["toughness"] == 2
        assert chars["name"] == ""
        assert chars["colors"] == []

    def test_layer_7d_counter_still_applies_over_face_down(self):
        """CR §613.1g: layer 7 applies AFTER layer 1. A face-down 2/2 + a
        +1/+1 counter = 3/3 (per judge-grade interaction)."""
        g = _empty_game()
        true_card = _card("MorphCard", "Creature — Goblin",
                          power=5, toughness=5, cmc=5,
                          morph_cost=_mc("{1}{G}"),
                          has_morph=True)
        g.seats[0].hand.append(true_card)
        fd = cast_face_down(g, 0, true_card,
                            flip_cost=_mc("{1}{G}"),
                            variant="vanilla", is_megamorph=False)
        fd.counters["+1/+1"] = 1
        # Invalidate cache so the layer resolver re-computes
        g.invalidate_characteristics_cache()
        chars = get_effective_characteristics(g, fd)
        assert chars["power"] == 3, (
            f"face-down 2/2 + +1/+1 counter should be 3/3, got "
            f"{chars['power']}/{chars['toughness']}"
        )
        assert chars["toughness"] == 3


class TestMorph:
    def test_cast_face_down_puts_on_battlefield(self):
        """CR §702.37c: casting face-down via morph puts the permanent
        onto the battlefield face down."""
        g = _empty_game()
        card = _card("Test Morph", "Creature — Human",
                     power=4, toughness=4, cmc=4,
                     morph_cost=_mc("{U}"), has_morph=True)
        g.seats[0].hand.append(card)
        assert len(g.seats[0].battlefield) == 0

        fd = cast_face_down(g, 0, card, flip_cost=_mc("{U}"),
                            variant="vanilla", is_megamorph=False)
        assert fd in g.seats[0].battlefield
        assert fd.face_down is True
        assert fd.original_card is card
        assert card not in g.seats[0].hand

    def test_turn_face_up_pays_flip_cost(self):
        """CR §702.37e: turning face up pays the morph cost."""
        g = _empty_game()
        card = _card("Test Morph 2", "Creature — Wizard",
                     power=4, toughness=4, cmc=4,
                     morph_cost=_mc("{2}{U}"), has_morph=True)
        g.seats[0].hand.append(card)
        fd = cast_face_down(g, 0, card, flip_cost=_mc("{2}{U}"),
                            variant="vanilla", is_megamorph=False)
        g.seats[0].mana_pool = 5  # enough for {2}{U}
        assert fd.face_down is True
        ok = turn_face_up(g, fd, paying=True)
        assert ok is True
        assert fd.face_down is False
        assert g.seats[0].mana_pool == 2  # 5 - 3

    def test_turn_face_up_fails_without_mana(self):
        """If the controller can't pay the flip cost, the special action
        fails and the permanent stays face down."""
        g = _empty_game()
        card = _card("Test Morph 3", "Creature",
                     power=4, toughness=4, cmc=4,
                     morph_cost=_mc("{4}{U}"), has_morph=True)
        g.seats[0].hand.append(card)
        fd = cast_face_down(g, 0, card, flip_cost=_mc("{4}{U}"),
                            variant="vanilla", is_megamorph=False)
        g.seats[0].mana_pool = 1  # not enough
        ok = turn_face_up(g, fd, paying=True)
        assert ok is False
        assert fd.face_down is True

    def test_face_up_restores_true_characteristics(self):
        """After turning face up, the permanent's true card takes over."""
        g = _empty_game()
        card = _card("Willbender Stand-In",
                     "Creature — Shapeshifter Wizard",
                     power=1, toughness=2, cmc=3, colors=("U",),
                     morph_cost=_mc("{1}{U}"), has_morph=True)
        g.seats[0].hand.append(card)
        fd = cast_face_down(g, 0, card, flip_cost=_mc("{1}{U}"),
                            variant="vanilla", is_megamorph=False)
        g.seats[0].mana_pool = 5
        turn_face_up(g, fd, paying=True)
        chars = get_effective_characteristics(g, fd)
        # Now the true card's values show through
        assert chars["power"] == 1
        assert chars["toughness"] == 2
        assert chars["name"] == "Willbender Stand-In"


class TestMegamorph:
    def test_flip_places_p1p1_counter(self):
        """CR §702.37b: megamorph adds a +1/+1 counter on turn-face-up."""
        g = _empty_game()
        card = _card("Den Protector Stand-In",
                     "Creature — Human Druid",
                     power=2, toughness=1, cmc=2,
                     morph_cost=_mc("{1}{G}"),
                     has_megamorph=True, has_morph=True)
        g.seats[0].hand.append(card)
        fd = cast_face_down(g, 0, card, flip_cost=_mc("{1}{G}"),
                            variant="vanilla", is_megamorph=True)
        assert fd.counters.get("+1/+1", 0) == 0
        g.seats[0].mana_pool = 3
        turn_face_up(g, fd, paying=True)
        assert fd.face_down is False
        assert fd.counters.get("+1/+1", 0) == 1, (
            "megamorph must place a +1/+1 counter on flip (CR §702.37b)"
        )


class TestDisguise:
    def test_face_down_disguise_has_ward_2(self):
        """CR §702.168a: face-down disguise permanent has ward {2}."""
        g = _empty_game()
        card = _card("Boltbender Stand-In", "Creature — Human",
                     power=3, toughness=2, cmc=4, colors=("R",),
                     disguise_cost=_mc("{1}{R}"), has_disguise=True)
        g.seats[0].hand.append(card)
        fd = cast_face_down(g, 0, card, flip_cost=_mc("{1}{R}"),
                            variant="ward_2", is_megamorph=False)
        chars = get_effective_characteristics(g, fd)
        assert "ward_2" in chars["abilities"], (
            "face-down disguise must carry ward {2} per CR §702.168a"
        )

    def test_flip_removes_ward_2(self):
        """After flipping, the face-up form's true abilities apply —
        ward {2} from the face-down override is gone."""
        g = _empty_game()
        card = _card("Flipping Disguise", "Creature",
                     power=2, toughness=2, cmc=3,
                     disguise_cost=_mc("{1}{R}"), has_disguise=True)
        g.seats[0].hand.append(card)
        fd = cast_face_down(g, 0, card, flip_cost=_mc("{1}{R}"),
                            variant="ward_2", is_megamorph=False)
        g.seats[0].mana_pool = 5
        turn_face_up(g, fd, paying=True)
        chars = get_effective_characteristics(g, fd)
        # Face-up card has no ward — the face-down override is gone
        assert "ward_2" not in chars["abilities"]


class TestManifest:
    def test_manifest_top_of_library(self):
        """CR §701.40a: manifest puts the TOP of library face down as
        a 2/2 creature."""
        g = _empty_game()
        creature_card = _card("Top Card", "Creature — Elf",
                              power=3, toughness=3, cmc=2,
                              mana_cost="{1}{G}")
        g.seats[0].library.append(creature_card)
        assert len(g.seats[0].battlefield) == 0
        perms = manifest(g, 0, count=1)
        assert len(perms) == 1
        fd = perms[0]
        assert fd.face_down is True
        assert fd.original_card is creature_card
        # Face-down is always 2/2 even though Top Card is 3/3
        chars = get_effective_characteristics(g, fd)
        assert chars["power"] == 2
        assert chars["toughness"] == 2

    def test_manifest_creature_flippable(self):
        """CR §701.40a: if the manifested card is a creature, it can
        be turned face up for its mana cost."""
        g = _empty_game()
        creature = _card("Top Creature", "Creature — Elf",
                         power=3, toughness=3, cmc=2,
                         mana_cost="{1}{G}")
        g.seats[0].library.append(creature)
        perms = manifest(g, 0, count=1)
        fd = perms[0]
        assert fd.turn_face_up_cost is not None
        assert fd.turn_face_up_cost.cmc == 2
        g.seats[0].mana_pool = 3
        ok = turn_face_up(g, fd, paying=True)
        assert ok is True
        assert fd.face_down is False

    def test_manifest_noncreature_cannot_flip(self):
        """CR §701.40a (reading): "you may turn it face up any time for
        its mana cost if it's a creature card." Non-creature manifests
        stay face-down permanently (engine tracks via
        turn_face_up_cost=None)."""
        g = _empty_game()
        sorcery = _card("Top Sorcery", "Sorcery",
                        power=None, toughness=None, cmc=1,
                        mana_cost="{R}")
        g.seats[0].library.append(sorcery)
        perms = manifest(g, 0, count=1)
        fd = perms[0]
        assert fd.turn_face_up_cost is None
        ok = turn_face_up(g, fd, paying=True)
        assert ok is False  # can't flip a non-creature manifest


class TestManifestDread:
    def test_look_two_manifest_one_mill_other(self):
        """CR §701.62a: look at top 2, manifest 1, put the other into
        graveyard."""
        g = _empty_game()
        c1 = _card("First Card", "Creature — Bear",
                   power=2, toughness=2, cmc=2, mana_cost="{1}{G}")
        c2 = _card("Second Card", "Sorcery", cmc=1, mana_cost="{R}")
        g.seats[0].library.append(c1)
        g.seats[0].library.append(c2)
        assert len(g.seats[0].library) == 2
        assert len(g.seats[0].graveyard) == 0

        fd = manifest_dread(g, 0)
        assert fd is not None
        # Greedy policy picks the creature card (can flip later)
        assert fd.original_card is c1
        # The other one gets milled
        assert c2 in g.seats[0].graveyard
        assert len(g.seats[0].graveyard) == 1

    def test_manifest_dread_with_empty_library(self):
        """Manifest-dread with 0 library cards returns None (no face-
        down permanent created), doesn't raise."""
        g = _empty_game()
        result = manifest_dread(g, 0)
        assert result is None

    def test_manifest_dread_with_single_card_library(self):
        """Per §701.62a language, we look at TOP TWO. With only 1 card
        in library, we still manifest it (there's nothing to mill)."""
        g = _empty_game()
        only = _card("Only", "Creature — Ox",
                     power=4, toughness=5, cmc=4, mana_cost="{2}{G}{G}")
        g.seats[0].library.append(only)
        fd = manifest_dread(g, 0)
        assert fd is not None
        assert fd.original_card is only
        assert len(g.seats[0].graveyard) == 0


class TestCloak:
    def test_cloak_top_of_library_has_ward_2(self):
        """CR §701.58a: cloaked top card becomes 2/2 face-down with
        ward {2}."""
        g = _empty_game()
        c = _card("Cloaked", "Creature — Human",
                  power=5, toughness=5, cmc=5, mana_cost="{3}{W}{W}")
        g.seats[0].library.append(c)
        perms = cloak(g, 0, count=1)
        assert len(perms) == 1
        fd = perms[0]
        chars = get_effective_characteristics(g, fd)
        assert chars["power"] == 2
        assert chars["toughness"] == 2
        assert "ward_2" in chars["abilities"]

    def test_cloak_multiple(self):
        """Cloak N works by cloaking N cards one at a time."""
        g = _empty_game()
        c1 = _card("C1", "Creature — Elf", power=1, toughness=1,
                   cmc=1, mana_cost="{G}")
        c2 = _card("C2", "Creature — Wolf", power=2, toughness=2,
                   cmc=2, mana_cost="{1}{G}")
        c3 = _card("C3", "Creature — Bear", power=2, toughness=2,
                   cmc=2, mana_cost="{1}{G}")
        for c in (c1, c2, c3):
            g.seats[0].library.append(c)
        perms = cloak(g, 0, count=3)
        assert len(perms) == 3
        for p in perms:
            assert p.face_down is True
            assert p.face_down_variant == "ward_2"


class TestFaceDownInterference:
    def test_face_down_akroma_bypasses_protection(self):
        """CR §708.2a: a face-down permanent has no text (and therefore
        no abilities). Akroma, Angel of Fury cast face-down has NO
        protection from white/blue while face down — it's just a
        2/2 colorless nameless creature."""
        g = _empty_game()
        akroma = _card("Akroma Stand-In",
                       "Legendary Creature — Angel",
                       power=6, toughness=6, cmc=6, colors=("R",),
                       mana_cost="{5}{R}{R}{R}",
                       abilities=(
                           Keyword(name="flying"),
                           Keyword(name="trample"),
                           Keyword(name="protection"),
                       ),
                       morph_cost=_mc("{3}{R}{R}{R}"),
                       has_morph=True)
        g.seats[0].hand.append(akroma)
        fd = cast_face_down(g, 0, akroma, flip_cost=_mc("{3}{R}{R}{R}"),
                            variant="vanilla", is_megamorph=False)
        chars = get_effective_characteristics(g, fd)
        # No flying, no trample, no protection — face-down is vanilla
        assert "flying" not in chars["abilities"]
        assert "trample" not in chars["abilities"]
        assert "protection" not in chars["abilities"]

    def test_face_down_has_no_color_or_name(self):
        """CR §708.2a: face-down is colorless and nameless. A "destroy
        target black creature" spell can't target a face-down creature
        whose TRUE form is black, because its copiable color is now
        colorless."""
        g = _empty_game()
        true_card = _card("Black Creature", "Creature — Zombie",
                          power=4, toughness=4, cmc=3, colors=("B",),
                          mana_cost="{1}{B}{B}",
                          morph_cost=_mc("{B}"), has_morph=True)
        g.seats[0].hand.append(true_card)
        fd = cast_face_down(g, 0, true_card, flip_cost=_mc("{B}"),
                            variant="vanilla", is_megamorph=False)
        chars = get_effective_characteristics(g, fd)
        assert chars["colors"] == []  # colorless per §708.2a
        assert chars["name"] == ""     # nameless per §708.2a

    def test_face_down_leaves_battlefield_revealed(self):
        """CR §708.4 / §708.11: a face-down permanent leaving the
        battlefield is revealed — the TRUE card goes to the graveyard,
        not the face-down shell."""
        g = _empty_game()
        true_card = _card("Revealed On Death",
                          "Creature — Wizard",
                          power=3, toughness=2, cmc=4,
                          mana_cost="{2}{U}{U}",
                          morph_cost=_mc("{1}{U}"), has_morph=True)
        g.seats[0].hand.append(true_card)
        fd = cast_face_down(g, 0, true_card, flip_cost=_mc("{1}{U}"),
                            variant="vanilla", is_megamorph=False)
        # Kill it via _destroy_perm (direct removal path, simulates a
        # sweeper effect — exercises the "destination = graveyard" code)
        playloop._destroy_perm(g, fd, reason="test_destroy",
                               rule="704.5f")
        # The TRUE card is in the graveyard, not the face-down shell
        assert true_card in g.seats[0].graveyard, (
            "face-down leaving battlefield must reveal the true card "
            "in the graveyard (CR §708.4)"
        )
        # The shell's fake name "Face-Down Creature" is NOT in the
        # graveyard
        gy_names = [c.name for c in g.seats[0].graveyard]
        assert "Face-Down Creature" not in gy_names


class TestFaceDownDoubleInvariants:
    def test_face_down_cannot_be_re_flipped_to_face_down(self):
        """CR §708.2b: can't turn a face-down permanent face down; the
        effect does nothing. We enforce that turn_face_up on a face-up
        permanent fails cleanly (symmetric read — the flow doesn't
        re-flip)."""
        g = _empty_game()
        true_card = _card("x", "Creature",
                          power=1, toughness=1, cmc=1, mana_cost="{G}",
                          morph_cost=_mc("{G}"), has_morph=True)
        g.seats[0].hand.append(true_card)
        fd = cast_face_down(g, 0, true_card, flip_cost=_mc("{G}"),
                            variant="vanilla", is_megamorph=False)
        g.seats[0].mana_pool = 2
        turn_face_up(g, fd, paying=True)
        # Try again — should fail because it's no longer face-down
        ok = turn_face_up(g, fd, paying=True)
        assert ok is False

    def test_timestamp_updates_on_flip(self):
        """CR §613.7f: a permanent receives a new timestamp each time
        it turns face up or face down."""
        g = _empty_game()
        true_card = _card("TS Test", "Creature",
                          power=3, toughness=3, cmc=2,
                          mana_cost="{1}{G}",
                          morph_cost=_mc("{G}"), has_morph=True)
        g.seats[0].hand.append(true_card)
        fd = cast_face_down(g, 0, true_card, flip_cost=_mc("{G}"),
                            variant="vanilla", is_megamorph=False)
        ts_before = fd.timestamp
        g.seats[0].mana_pool = 3
        turn_face_up(g, fd, paying=True)
        ts_after = fd.timestamp
        assert ts_after > ts_before, (
            "timestamp must increase when turning face up (CR §613.7f)"
        )


# ===========================================================================
# Part C — Real-corpus integration spot checks
# ===========================================================================


class TestCorpusCards:
    """These depend on oracle-cards.json. If the dump is missing, skip."""

    @pytest.fixture(scope="class")
    def oracle(self):
        import json
        dump_path = Path(__file__).resolve().parents[1] / "data" / "rules" / "oracle-cards.json"
        if not dump_path.exists():
            pytest.skip("oracle-cards.json missing")
        with open(dump_path, encoding="utf-8") as f:
            cards = json.load(f)
        return {c["name"]: c for c in cards}

    def test_ainok_survivalist_parses(self, oracle):
        """Ainok Survivalist: Megamorph {1}{G} → morph_cost populated,
        has_megamorph True, face_down_copy_effect Static present."""
        card = oracle.get("Ainok Survivalist")
        if card is None:
            pytest.skip("Ainok Survivalist not in oracle dump")
        ast = parse_card(card)
        assert ast.has_megamorph is True
        assert ast.has_morph is True
        assert ast.morph_cost is not None
        assert ast.morph_cost.cmc == 2

    def test_willbender_parses(self, oracle):
        """Willbender: Morph {1}{U} — morph_cost populated."""
        card = oracle.get("Willbender")
        if card is None:
            pytest.skip("Willbender not in oracle dump")
        ast = parse_card(card)
        assert ast.has_morph is True
        assert ast.morph_cost is not None

    def test_akroma_angel_of_fury_parses(self, oracle):
        """Akroma, Angel of Fury: has morph; bypasses its own
        protection when face-down (§708.2a)."""
        card = oracle.get("Akroma, Angel of Fury")
        if card is None:
            pytest.skip("Akroma, Angel of Fury not in oracle dump")
        ast = parse_card(card)
        assert ast.has_morph is True
        # Akroma's morph cost is {3}{R}{R}{R} = CMC 6
        assert ast.morph_cost is not None
        assert ast.morph_cost.cmc == 6

    def test_den_protector_parses(self, oracle):
        """Den Protector: Megamorph {1}{G}."""
        card = oracle.get("Den Protector")
        if card is None:
            pytest.skip("Den Protector not in oracle dump")
        ast = parse_card(card)
        assert ast.has_megamorph is True

    def test_boltbender_disguise_parses(self, oracle):
        """Boltbender: Disguise {1}{R} → disguise_cost set."""
        card = oracle.get("Boltbender")
        if card is None:
            pytest.skip("Boltbender not in oracle dump")
        ast = parse_card(card)
        assert ast.has_disguise is True
        assert ast.disguise_cost is not None
        assert ast.disguise_cost.cmc == 2

    def test_whisperwood_elemental_manifest_flag(self, oracle):
        """Whisperwood Elemental manifests top of library — flagged."""
        card = oracle.get("Whisperwood Elemental")
        if card is None:
            pytest.skip("Whisperwood Elemental not in oracle dump")
        ast = parse_card(card)
        assert ast.manifest_token is True

    def test_hauntwoods_shrieker_manifest_dread(self, oracle):
        """Hauntwoods Shrieker manifests dread — flagged."""
        card = oracle.get("Hauntwoods Shrieker")
        if card is None:
            pytest.skip("Hauntwoods Shrieker not in oracle dump")
        ast = parse_card(card)
        assert ast.manifest_token is True


if __name__ == "__main__":
    pytest.main([__file__, "-v"])
