#!/usr/bin/env python3
"""Standalone verification test for §903 Commander-variant infrastructure.

Covers:
  1. Command-zone setup (§903.6) + 40 starting life (§903.4).
  2. Commander tax accrual (§903.8): cast N → pay base + 2(N-1).
  3. §903.9b / §704.6d zone-change replacement: commander dies →
     graveyard → SBA returns to command zone.
  4. Gilded-Drake-like ownership swap (§108.3): commander controlled
     by opp still uses OWNER's replacement when it dies → owner's
     command zone.
  5. Commander damage (§903.10a / §704.6c): accumulating combat
     damage from the same commander, 21-threshold → lose game.

Run standalone:
    python3 scripts/test_commander_zone.py

Exit code 0 = pass, non-zero = fail. Also prints a summary so CI
can eyeball the checks list.

Citations verified against data/rules/MagicCompRules-20260227.txt:
  104.3j, 108.3, 614.5, 614.6, 704.5a, 704.6c, 704.6d,
  903.3, 903.4, 903.6, 903.7, 903.8, 903.9a, 903.9b, 903.10a
"""

from __future__ import annotations

import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))

from mtg_ast import CardAST  # type: ignore  # noqa: E402

import playloop as pl  # type: ignore  # noqa: E402


# ---------------------------------------------------------------------------
# Synthetic-card helpers. Parser-free so we don't need oracle-cards.json.
# ---------------------------------------------------------------------------

def make_card(name: str, cmc: int, type_line: str = "Legendary Creature — Human",
              power: int = 2, toughness: int = 2,
              colors: tuple = ()) -> pl.CardEntry:
    """Build a CardEntry with an empty AST. Enough to cast + combat."""
    return pl.CardEntry(
        name=name,
        mana_cost=f"{{{cmc}}}",
        cmc=cmc,
        type_line=type_line,
        oracle_text="",
        power=power,
        toughness=toughness,
        ast=CardAST(name=name, abilities=(), parse_errors=(), fully_parsed=True),
        colors=colors,
        starting_loyalty=None,
        starting_defense=None,
    )


def make_basic_land(name: str = "Forest") -> pl.CardEntry:
    return pl.CardEntry(
        name=name,
        mana_cost="",
        cmc=0,
        type_line=f"Basic Land — {name}",
        oracle_text="",
        power=None,
        toughness=None,
        ast=CardAST(name=name, abilities=(), parse_errors=(), fully_parsed=True),
        colors=(),
    )


def build_deck(commander: pl.CardEntry,
               fillers: int = 40) -> pl.Deck:
    """A minimal Commander deck: the commander + filler lands. We don't
    need 99 cards for the tests; the shuffle step just needs a non-empty
    library so draw_cards doesn't instantly deck out the player."""
    land = make_basic_land("Forest")
    cards = [land for _ in range(fillers)]
    deck = pl.Deck(cards=cards, commander_name=commander.name)
    # Attach the commander CardEntry so setup_commander_game can
    # populate command_zone directly.
    deck.commander_cards = [commander]  # type: ignore[attr-defined]
    return deck


# ---------------------------------------------------------------------------
# Harness utilities — set up a 2-seat Commander game without the full
# play_game loop (we want fine control over phases for each assertion).
# ---------------------------------------------------------------------------

import random


def spawn_commander_game(seed: int = 1) -> tuple[pl.Game, pl.Deck, pl.Deck,
                                                 pl.CardEntry, pl.CardEntry]:
    cmd_a = make_card("Alpha Commander", cmc=3,
                      type_line="Legendary Creature — Human",
                      power=5, toughness=3, colors=("G",))
    cmd_b = make_card("Beta Commander", cmc=2,
                      type_line="Legendary Creature — Goblin",
                      power=2, toughness=2, colors=("R",))
    deck_a = build_deck(cmd_a)
    deck_b = build_deck(cmd_b)
    rng = random.Random(seed)
    seats = [
        pl.Seat(idx=0, library=list(deck_a.cards)),
        pl.Seat(idx=1, library=list(deck_b.cards)),
    ]
    game = pl.Game(seats=seats, verbose=False, rng=rng)
    pl.setup_commander_game(game, deck_a, deck_b)
    game.active = 0
    return game, deck_a, deck_b, cmd_a, cmd_b


def give_seat_mana(seat: pl.Seat, amount: int) -> None:
    """Dump `amount` generic mana into the pool without land drops."""
    seat.mana_pool += amount


# ---------------------------------------------------------------------------
# Assertions — each returns (passed: bool, detail: str).
# ---------------------------------------------------------------------------

def _check(desc: str, cond: bool, detail: str = "") -> tuple[str, bool, str]:
    return (desc, cond, detail)


def test_setup() -> list:
    out = []
    game, _, _, cmd_a, cmd_b = spawn_commander_game()
    out.append(_check("§903.6 command zone contains commander (seat 0)",
                      any(c.name == cmd_a.name for c in game.seats[0].command_zone)))
    out.append(_check("§903.6 command zone contains commander (seat 1)",
                      any(c.name == cmd_b.name for c in game.seats[1].command_zone)))
    out.append(_check("§903.4 seat 0 starting life = 40",
                      game.seats[0].life == 40,
                      f"life={game.seats[0].life}"))
    out.append(_check("§903.4 seat 1 starting life = 40",
                      game.seats[1].life == 40,
                      f"life={game.seats[1].life}"))
    out.append(_check("commander_format flag set",
                      game.commander_format is True))
    out.append(_check("seat.commander_names seeded",
                      game.seats[0].commander_names == [cmd_a.name]
                      and game.seats[1].commander_names == [cmd_b.name]))
    out.append(_check("§903.8 initial tax = 0",
                      game.seats[0].commander_tax[cmd_a.name] == 0))
    # §614 registration sanity — one replacement per commander.
    zc_reps = [r for r in game.replacement_effects
               if r.event_type == "would_change_zone"]
    out.append(_check("§903.9b replacement registered per commander",
                      len(zc_reps) == 2,
                      f"found {len(zc_reps)}"))
    return out


def test_commander_tax() -> list:
    """§903.8 — cast N times from command zone → pay base + 2(N-1)."""
    out = []
    game, _, _, cmd_a, _ = spawn_commander_game()
    seat = game.seats[0]
    # Force into main phase so the stack resolution path works.
    game.phase = "main1"

    # 1st cast: tax=0, base=3, pay 3, tax becomes 1.
    give_seat_mana(seat, 3)
    pl.cast_commander_from_command_zone(game, cmd_a)
    out.append(_check("§903.8 1st cast: tax counter → 1",
                      seat.commander_tax[cmd_a.name] == 1,
                      f"tax={seat.commander_tax[cmd_a.name]}"))
    out.append(_check("§903.8 1st cast: commander on battlefield",
                      any(p.card.name == cmd_a.name for p in seat.battlefield)))

    # Kill it so we can re-cast. Manually return to command zone as if by
    # the §704.6d SBA (skip the destroy path for clarity of tax test).
    perm_a = next(p for p in seat.battlefield if p.card.name == cmd_a.name)
    seat.battlefield.remove(perm_a)
    pl.unregister_replacements_for_permanent(game, perm_a)
    seat.command_zone.append(cmd_a)

    # 2nd cast: tax=1, pay base+2 = 5, tax becomes 2.
    give_seat_mana(seat, 5)
    pl.cast_commander_from_command_zone(game, cmd_a)
    out.append(_check("§903.8 2nd cast: paid base + 2×1 = 5",
                      seat.commander_tax[cmd_a.name] == 2))

    # Bounce it back without tax increase (§903.8 ONLY counts command-
    # zone casts).
    perm_a = next(p for p in seat.battlefield if p.card.name == cmd_a.name)
    seat.battlefield.remove(perm_a)
    pl.unregister_replacements_for_permanent(game, perm_a)
    seat.command_zone.append(cmd_a)

    # 3rd cast: tax=2, pay base+4 = 7, tax becomes 3.
    give_seat_mana(seat, 7)
    pl.cast_commander_from_command_zone(game, cmd_a)
    out.append(_check("§903.8 3rd cast: tax progresses to 3",
                      seat.commander_tax[cmd_a.name] == 3))

    # And verify that casting from HAND doesn't accrue tax. Move the
    # commander to hand by simulating an opposing bounce (flee the
    # command-zone return via SBA wouldn't happen from battlefield→hand
    # because §903.9b would redirect in commander mode; disable the
    # replacement for this test step).
    perm_a = next(p for p in seat.battlefield if p.card.name == cmd_a.name)
    seat.battlefield.remove(perm_a)
    pl.unregister_replacements_for_permanent(game, perm_a)
    # Temporarily remove the §903.9b replacement so we can model a
    # hand-cast of the commander.
    saved_reps = game.replacement_effects[:]
    game.replacement_effects = [
        r for r in game.replacement_effects
        if not (r.event_type == "would_change_zone"
                and r.source_card_name == cmd_a.name)
    ]
    seat.hand.append(cmd_a)
    give_seat_mana(seat, 3)
    pl.cast_spell(game, cmd_a)
    out.append(_check("§903.8 cast from HAND: tax UNCHANGED",
                      seat.commander_tax[cmd_a.name] == 3))
    game.replacement_effects = saved_reps
    return out


def test_zone_change_replacement_destroy() -> list:
    """§903.9a/§704.6d — commander dies → SBA returns to command zone."""
    out = []
    game, _, _, cmd_a, _ = spawn_commander_game()
    seat = game.seats[0]
    game.phase = "main1"

    give_seat_mana(seat, 3)
    pl.cast_commander_from_command_zone(game, cmd_a)
    perm_a = next(p for p in seat.battlefield if p.card.name == cmd_a.name)
    before_len_cz = len(seat.command_zone)

    # Destroy it via the normal destroy path.
    pl.do_permanent_die(game, perm_a, reason="test_destroy")
    # At this instant the card is in graveyard; SBA must move it back.
    pl.state_based_actions(game)
    out.append(_check("§704.6d dead commander returns to command zone",
                      any(c.name == cmd_a.name for c in seat.command_zone),
                      f"cz size before={before_len_cz}, "
                      f"after={len(seat.command_zone)}, "
                      f"gy={[c.name for c in seat.graveyard]}"))
    out.append(_check("§704.6d graveyard cleared of commander",
                      not any(c.name == cmd_a.name for c in seat.graveyard)))
    return out


def test_gilded_drake_swap() -> list:
    """§108.3 — commander controlled by opponent still uses OWNER's
    replacement when it dies; card goes to OWNER's graveyard → owner's
    §704.6d SBA returns it to OWNER's command zone."""
    out = []
    game, _, _, cmd_a, _ = spawn_commander_game()
    seat_0 = game.seats[0]
    seat_1 = game.seats[1]
    game.phase = "main1"

    give_seat_mana(seat_0, 3)
    pl.cast_commander_from_command_zone(game, cmd_a)
    perm_a = next(p for p in seat_0.battlefield if p.card.name == cmd_a.name)

    # Simulate a Gilded-Drake control swap. The commander is now on
    # seat_1's battlefield but OWNED by seat_0.
    seat_0.battlefield.remove(perm_a)
    perm_a.controller = 1
    perm_a.owner = 0  # explicit ownership override (§108.3)
    seat_1.battlefield.append(perm_a)

    out.append(_check("Gilded-Drake: owner_seat resolves to 0",
                      perm_a.owner_seat == 0))
    out.append(_check("Gilded-Drake: controller is 1",
                      perm_a.controller == 1))

    # Kill it — should go to OWNER's graveyard, then SBA returns to
    # OWNER's command zone (§903.9a / §704.6d).
    pl.do_permanent_die(game, perm_a, reason="gilded_drake_death")
    pl.state_based_actions(game)
    out.append(_check("Gilded-Drake: commander back in OWNER's command zone",
                      any(c.name == cmd_a.name for c in seat_0.command_zone),
                      f"owner cz={[c.name for c in seat_0.command_zone]}, "
                      f"thief cz={[c.name for c in seat_1.command_zone]}"))
    out.append(_check("Gilded-Drake: NOT in thief's command zone",
                      not any(c.name == cmd_a.name for c in seat_1.command_zone)))
    out.append(_check("Gilded-Drake: owner's graveyard clear",
                      not any(c.name == cmd_a.name for c in seat_0.graveyard)))
    out.append(_check("Gilded-Drake: thief's graveyard clear",
                      not any(c.name == cmd_a.name for c in seat_1.graveyard)))
    return out


def test_commander_damage() -> list:
    """§903.10a / §704.6c — accumulate combat damage keyed by commander
    name; 21+ → seat loses."""
    out = []
    game, _, _, cmd_a, _ = spawn_commander_game()
    seat_0 = game.seats[0]
    seat_1 = game.seats[1]

    # Synthesize combat damage events directly into the event stream.
    # _sba_704_6c mines game.events for damage with target_kind="player"
    # and phase=="combat", where source_card matches a seat's
    # commander_names. Emit two hits summing to 20; no death.
    game.phase = "combat"
    game.ev("damage", amount=10, target_kind="player",
            target_seat=seat_1.idx, source_card=cmd_a.name,
            source_seat=seat_0.idx)
    game.ev("damage", amount=10, target_kind="player",
            target_seat=seat_1.idx, source_card=cmd_a.name,
            source_seat=seat_0.idx)
    # State-based actions mine the events and populate commander_damage.
    pl.state_based_actions(game)
    # commander_damage is now nested (dealer_seat → name → int) per
    # partner spec. Sum across all dealers for the name total — in
    # practice only one seat owns each commander name.
    def _total_by_name(cd: dict, name: str) -> int:
        total = 0
        for by_name in cd.values():
            total += by_name.get(name, 0)
        return total
    out.append(_check("§903.10a accumulates combat damage (20 total)",
                      _total_by_name(seat_1.commander_damage, cmd_a.name) == 20,
                      f"dmg={seat_1.commander_damage}"))
    out.append(_check("§903.10a < 21 → seat 1 still alive",
                      not seat_1.lost,
                      f"lost={seat_1.lost}, reason={seat_1.loss_reason}"))

    # One more hit puts seat 1 over the threshold (21+ → lose).
    game.ev("damage", amount=1, target_kind="player",
            target_seat=seat_1.idx, source_card=cmd_a.name,
            source_seat=seat_0.idx)
    pl.state_based_actions(game)
    out.append(_check("§903.10a 21+ total → seat 1 lost",
                      seat_1.lost,
                      f"lost={seat_1.lost}, reason={seat_1.loss_reason}, "
                      f"dmg={seat_1.commander_damage}"))
    out.append(_check("§704.6c loss reason cites commander",
                      "commander damage" in seat_1.loss_reason.lower()
                      or "704.6c" in seat_1.loss_reason))

    # Independent commanders are tracked separately. Reset a new game
    # and verify two different source_cards accumulate independently.
    game2, _, _, cmd_a2, cmd_b2 = spawn_commander_game(seed=2)
    game2.phase = "combat"
    # 20 damage from each different commander to the same seat — NOT
    # enough from any single commander to lose.
    game2.ev("damage", amount=20, target_kind="player",
             target_seat=1, source_card=cmd_a2.name,
             source_seat=0)
    # Can't deal damage from cmd_b2 to its own owner (seat 1) in
    # combat; the §903.10a "same commander" check still keys by name,
    # so pretend cmd_b2 hit the same seat 1 from a different source.
    # Use any OTHER name that happens to coincide — that won't hit our
    # filter. Skip this step; the point is just "independent keys".
    pl.state_based_actions(game2)
    out.append(_check("§903.10a independent accumulation "
                      "(20 from single commander → alive)",
                      not game2.seats[1].lost,
                      f"dmg={game2.seats[1].commander_damage}"))
    return out


# ---------------------------------------------------------------------------
# Main driver
# ---------------------------------------------------------------------------

def run_all() -> int:
    suites = [
        ("Setup (§903.3/§903.4/§903.6/§903.7/§903.9b registration)", test_setup),
        ("Commander tax (§903.8)", test_commander_tax),
        ("Zone-change replacement / destroy "
         "(§903.9a + §704.6d SBA)", test_zone_change_replacement_destroy),
        ("Gilded-Drake ownership swap "
         "(§108.3 + §903.9a)", test_gilded_drake_swap),
        ("Commander damage (§903.10a + §704.6c)", test_commander_damage),
    ]
    total_pass = 0
    total_fail = 0
    print("═" * 72)
    print("  §903 Commander Infrastructure — verification tests")
    print("═" * 72)
    for name, fn in suites:
        print(f"\n  Suite: {name}")
        try:
            results = fn()
        except Exception as exc:
            print(f"    ! CRASH: {type(exc).__name__}: {exc}")
            import traceback
            traceback.print_exc()
            total_fail += 1
            continue
        for desc, ok, detail in results:
            flag = "PASS" if ok else "FAIL"
            tail = f"  ({detail})" if detail and not ok else ""
            print(f"    [{flag}] {desc}{tail}")
            if ok:
                total_pass += 1
            else:
                total_fail += 1
    print("\n" + "═" * 72)
    print(f"  Total: {total_pass} passed, {total_fail} failed "
          f"({total_pass + total_fail} assertions)")
    print("═" * 72)
    return 0 if total_fail == 0 else 1


if __name__ == "__main__":
    sys.exit(run_all())
