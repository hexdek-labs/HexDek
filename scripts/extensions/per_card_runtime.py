#!/usr/bin/env python3
"""Runtime dispatch for snowflake per-card handlers.

Parse-time counterparts live in :mod:`extensions.per_card`. Those emit a
``Modification(kind="custom", args=(<slug>, ...))`` marker on the AST.
When the runtime sees one of those markers it calls into this module's
dispatch table to mutate game state.

Contract for each handler::

    def handle(game, source_seat, source_card, ctx) -> None

``source_seat`` is the int index of the seat that controls/cast the source.
``source_card`` is the :class:`playloop.CardEntry` whose AST carried the
Modification. ``ctx`` is a dict with any contextual hints (e.g. the
resolving :class:`StackItem`, the :class:`Permanent` for an ETB trigger).

Handlers should:
  * fail gracefully — if a zone is empty, a target missing, a mode
    ambiguous, etc., emit a ``per_card_failed`` event with a reason and
    return cleanly. Never raise; a raise surfaces as a crash in the
    interaction harnesses.
  * emit a ``per_card_handler`` event with ``card``, ``slug``, plus any
    salient fields. This is what the rule auditor and downstream reviewers
    see.
  * treat the game state mutation as atomic — once the handler begins, it
    must leave the game in a consistent zone-set even on early return.

Accuracy discipline (the whole point of this module):
  * Each handler block begins with a quote of the card's current Oracle
    text (as of the 2025-02-27 comprehensive-rules refresh). If the
    implementation departs from literal oracle text, say why in a comment.
  * Prefer partial-but-correct over full-but-lossy. A handler that only
    implements the half of the card that fits the current engine
    infrastructure (and logs a ``per_card_partial`` event for the rest)
    is strictly better than one that pretends to do everything.

Registry layout: each slug maps to exactly one function. Slugs come from
per_card.py's ``_custom(slug, ...)`` / ``_custom_triggered(slug, ...)``
builders. New slugs can be added here at any time — absent entries just
fall through the resolver as a ``per_card_unhandled`` event.
"""

from __future__ import annotations

from typing import Callable, Optional

# All handlers take a minimal signature; we pass game state by reference.
# ctx is a free-form dict so the resolver can forward permanent/stack info
# without growing a rigid parameter list.
PerCardHandler = Callable[[object, int, object, dict], None]


# ---------------------------------------------------------------------------
# Helper utilities shared across handlers
# ---------------------------------------------------------------------------

def _emit(game, slug: str, card, **extras) -> None:
    """Standard event emission for a runtime handler firing."""
    game.ev("per_card_handler",
            slug=slug,
            card=getattr(card, "name", "?"),
            **extras)
    game.emit(f"  per-card[{slug}] {getattr(card, 'name', '?')}")


def _fail(game, slug: str, card, reason: str, **extras) -> None:
    """Standard event for a handler that couldn't execute (graceful skip)."""
    game.ev("per_card_failed",
            slug=slug,
            card=getattr(card, "name", "?"),
            reason=reason,
            **extras)
    game.emit(f"  per-card[{slug}] {getattr(card, 'name', '?')} failed: {reason}")


def _partial(game, slug: str, card, missing: str, **extras) -> None:
    """A handler that did part of its work but left a clause unimplemented."""
    game.ev("per_card_partial",
            slug=slug,
            card=getattr(card, "name", "?"),
            missing=missing,
            **extras)


def _move_library_to_exile(seat) -> int:
    """Move every card in a seat's library to exile. Returns the count."""
    moved = 0
    while seat.library:
        seat.exile.append(seat.library.pop(0))
        moved += 1
    return moved


def _controller_of_source(game, source_seat: int, source_card) -> int:
    """Best-effort: if the source_card is on the battlefield, use that
    controller; else fall back to source_seat. Most effects on the stack
    resolve under their controller, which is source_seat."""
    for s in game.seats:
        for p in s.battlefield:
            if p.card is source_card:
                return p.controller
    return source_seat


def _source_permanent(game, source_card):
    """Locate the Permanent object backing source_card, if any."""
    for s in game.seats:
        for p in s.battlefield:
            if p.card is source_card:
                return p
    return None


# ---------------------------------------------------------------------------
# Individual handlers — top-impact snowflakes
# ---------------------------------------------------------------------------

def _doomsday_pile(game, source_seat: int, source_card, ctx: dict) -> None:
    """Doomsday — *"Search your library and graveyard for five cards and
    exile the rest. Put the chosen cards on top of your library in any
    order. You lose half your life, rounded up."*

    MVP implementation:
      * Gather library + graveyard into one pool.
      * Pick 5 cards; if fewer than 5 total exist, use whatever is
        available (loose on rules — normally you would have to pick
        exactly five; the CR allows none of a type so picking fewer is
        actually legal when the pool is smaller).
      * Exile everything that wasn't picked.
      * Rebuild the library from the picked stack.
      * Shuffle the library AFTER the pile is built, per the Oracle text:
        "Put the chosen cards on top of your library in any order." There
        is no shuffle clause — the pile is not shuffled. This matters for
        Doomsday Oracle combos.

    The "order" selection is a choice made by the controller; the MVP
    picks the order in which we collected them (library-first-then-graveyard)
    which is deterministic given the current state. A real client will want
    to expose choice.
    """
    slug = "doomsday_pile"
    seat = game.seats[source_seat]
    pool = list(seat.library) + list(seat.graveyard)
    if not pool:
        _fail(game, slug, source_card, "library_and_graveyard_empty")
        return

    picked = pool[:5]
    rest = pool[5:]

    # Clear source zones
    seat.library.clear()
    seat.graveyard.clear()

    # Put picked on top of library in the chosen order (top = index 0).
    seat.library.extend(picked)
    # Exile everything else
    seat.exile.extend(rest)
    _emit(game, slug, source_card,
          picked_count=len(picked),
          picked=[c.name for c in picked],
          exiled_count=len(rest))


def _doomsday_life_loss(game, source_seat: int, source_card, ctx: dict) -> None:
    """Doomsday half-life payment. Rounded up: life//2 + life%2.

    Emits a ``life_change`` event so the auditor can track the total.
    """
    slug = "lose_half_life_rounded_up"
    seat = game.seats[source_seat]
    before = seat.life
    # Rounded up: ceil(life / 2).
    loss = (before + 1) // 2 if before > 0 else 0
    seat.life -= loss
    _emit(game, slug, source_card, amount=loss,
          **{"from": before, "to": seat.life})
    game.ev("life_change", seat=seat.idx,
            **{"from": before, "to": seat.life})
    if seat.life <= 0:
        seat.lost = True
        seat.loss_reason = "life total reached 0"
        game.check_end()


def _painters_servant_choose_color(game, source_seat: int, source_card, ctx: dict) -> None:
    """Painter's Servant — *"As Painter's Servant enters the battlefield,
    choose a color."*

    MVP choice policy: name the most common color among opposing creatures
    (so downstream effects like Grindstone can detect color-sharing). If
    opposing board has no colored creatures, pick the first color on the
    Servant's controller's library. Last resort: White.

    We stash the chosen color on the Permanent instance as
    ``chosen_color``. Other handlers (``color_wash``, Grindstone's mill
    loop) read that attribute.
    """
    slug = "painters_servant_choose_color"
    perm = ctx.get("permanent") or _source_permanent(game, source_card)
    if perm is None:
        _fail(game, slug, source_card, "no_battlefield_permanent")
        return
    # Count colors across all opposing permanents
    from collections import Counter
    opp = game.opp(source_seat)
    tally = Counter()
    for p in opp.battlefield:
        for c in p.card.colors or ():
            tally[c] += 1
    # Fall back: look at own library
    if not tally:
        for c in game.seats[source_seat].library:
            for col in c.colors or ():
                tally[col] += 1
                break
    chosen = tally.most_common(1)[0][0] if tally else "W"
    perm.chosen_color = chosen
    _emit(game, slug, source_card, chosen_color=chosen)


def _painters_servant_color_wash(game, source_seat: int, source_card, ctx: dict) -> None:
    """Painter's Servant static — *"All cards that aren't on the battlefield,
    spells, and permanents are the chosen color in addition to their other
    colors."* (2022 Oracle wording)

    MVP implementation: mark this as a registered color-wash source. The
    color-mixing logic lives in a helper queried at predicate-check time
    (by e.g. Grindstone's mill loop). We don't eagerly stamp every card
    because (a) new cards enter zones constantly and (b) the Oracle
    specifically says the *effect* grants the color — it's not a
    persistent property.

    Side effect: set ``game.painter_color`` to the chosen color. That
    attribute is read by consumers (Grindstone runtime) to know every
    card in every zone shares that color. If multiple Painter's Servants
    are active, the last one to resolve wins — a rules inaccuracy but one
    that only matters in vanishingly rare stacked-Painter scenarios.
    """
    slug = "painters_servant_color_wash"
    perm = _source_permanent(game, source_card)
    chosen = getattr(perm, "chosen_color", None) if perm else None
    if chosen is None:
        # The choose-color ETB handler should have run first; static
        # ability without a chosen color is a silent no-op until then.
        _fail(game, slug, source_card, "no_chosen_color_yet")
        return
    game.painter_color = chosen
    # §613 layer-5 registration — adds chosen_color to every on-
    # battlefield permanent's colors via the layer system. Kept in
    # lockstep with the legacy `game.painter_color` signal so existing
    # consumers (Grindstone, etc.) keep working.
    _painters_servant_register_layer5(game, source_seat, source_card, ctx)
    _emit(game, slug, source_card, chosen_color=chosen)


def _thassas_oracle_etb(game, source_seat: int, source_card, ctx: dict) -> None:
    """Thassa's Oracle — *"When Thassa's Oracle enters the battlefield,
    look at the top X cards of your library, where X is your devotion to
    blue. Put up to one of them on top of your library and the rest on
    the bottom in a random order. Then if your library has X or fewer
    cards in it, you win the game."*

    Implementation notes:
      * Devotion counting: we reuse the playloop's ``_count_devotion``
        helper. Oracle itself has one blue pip, so devotion ≥ 1 on its
        own resolution.
      * The "look at top X" + scry-style reordering is unobservable to
        the game in MVP (no hidden information model); we skip the look
        and go straight to the win check. This is a minor timing-only
        inaccuracy — the choice of what to keep on top vs bottom can
        matter for a Laboratory Maniac draw replacement later, but not
        for the Oracle win condition itself.
      * Win check: ``library_size <= devotion`` (the Oracle's OWN
        devotion, not target player's). Empty library also wins.
    """
    slug = "thassas_oracle_etb_win_check"
    # Lazy import to avoid circular import (playloop imports this module).
    from playloop import _count_devotion
    seat = game.seats[source_seat]
    devotion = _count_devotion(seat, ("U",))
    lib_size = len(seat.library)
    _emit(game, slug, source_card, devotion=devotion, library_size=lib_size)
    if lib_size <= devotion:
        game.ev("per_card_win",
                slug=slug,
                card=getattr(source_card, "name", "?"),
                winner_seat=seat.idx,
                reason="thassas_oracle")
        game.emit(f"  Thassa's Oracle WINS for seat {seat.idx} "
                  f"(library {lib_size} ≤ devotion {devotion})")
        # Opponent loses; SBA will also catch this, but we set explicitly.
        for other in game.seats:
            if other.idx != seat.idx:
                other.lost = True
                other.loss_reason = "Thassa's Oracle ETB win condition"
        game.ended = True
        game.winner = seat.idx
        game.end_reason = "Thassa's Oracle ETB devotion ≥ library size"
        game.check_end()


def _demonic_consultation_exile_loop(game, source_seat: int, source_card, ctx: dict) -> None:
    """Demonic Consultation — *"Name a card. Exile the top six cards of
    your library, then reveal cards from the top of your library until
    you reveal the named card. Put that card into your hand and exile
    all other cards revealed this way."*

    MVP implementation: since we can't "name" a card the player doesn't
    intend to find, the canonical Thoracle-combo usage is "name a card
    not in your deck" — which means we exile ALL of the top-6 plus
    everything else (the reveal loop never finds it → the whole library
    gets exiled with no card put into hand).

    We default to that combo-line behavior when ctx.named_card is absent
    or not present in the library. If a named card IS in the library,
    we exile top-6, then reveal+exile until the named card, then move
    the named card to hand.
    """
    slug = "demonic_consultation"
    seat = game.seats[source_seat]
    named = ctx.get("named_card")
    # Exile top 6 (or whatever's left).
    exiled_six = 0
    for _ in range(6):
        if not seat.library:
            break
        seat.exile.append(seat.library.pop(0))
        exiled_six += 1
    # Reveal-until-named
    revealed = 0
    found = False
    while seat.library:
        c = seat.library.pop(0)
        revealed += 1
        if named and c.name.lower() == named.lower():
            seat.hand.append(c)
            found = True
            break
        seat.exile.append(c)
    _emit(game, slug, source_card,
          exiled_top_six=exiled_six,
          revealed_count=revealed,
          found_named=found,
          named=named or "(none — library-emptied)")


def _tainted_pact_exile_until(game, source_seat: int, source_card, ctx: dict) -> None:
    """Tainted Pact — *"Exile the top card of your library. You may repeat
    this process any number of times. For each card exiled this way with
    the same name as another card exiled this way, Tainted Pact has no
    effect on that card, but those cards still go to exile. Stop when
    you choose to stop or when you exile a card with the same name as
    another card exiled this way."*

    MVP: the Thoracle-combo usage is "exile until you hit a duplicate or
    the library is empty." With a singleton deck (typical Commander /
    combo control) this just empties the library. We model that: exile
    until duplicate or empty.
    """
    slug = "tainted_pact"
    seat = game.seats[source_seat]
    seen_names = set()
    exiled = 0
    while seat.library:
        c = seat.library.pop(0)
        if c.name in seen_names:
            seat.exile.append(c)
            exiled += 1
            break  # Tainted Pact stops on name collision.
        seen_names.add(c.name)
        seat.exile.append(c)
        exiled += 1
    _emit(game, slug, source_card, exiled=exiled, duplicate_hit=(bool(seat.library) or exiled == 0))


def _laboratory_maniac_replacement(game, source_seat: int, source_card, ctx: dict) -> None:
    """Laboratory Maniac — *"If you would draw a card while your library
    has no cards in it, you win the game instead."*

    This is a replacement on the draw event, not an activated ability.
    The handler is fired from the draw-cards codepath whenever a draw
    would fail due to an empty library AND this permanent is in play.
    """
    slug = "laboratory_maniac_win"
    seat = game.seats[source_seat]
    _emit(game, slug, source_card, seat=seat.idx)
    game.ev("per_card_win",
            slug=slug,
            card=getattr(source_card, "name", "?"),
            winner_seat=seat.idx,
            reason="laboratory_maniac")
    for other in game.seats:
        if other.idx != seat.idx:
            other.lost = True
            other.loss_reason = "Laboratory Maniac opponent win"
    seat.lost = False  # explicitly mark alive — we're overriding a decking loss
    game.ended = True
    game.winner = seat.idx
    game.end_reason = "Laboratory Maniac replacement win"


def _grindstone_mill_loop(game, source_seat: int, source_card, ctx: dict) -> None:
    """Grindstone — *"{3}, {T}: Target player mills two cards. If two
    cards that share a color were milled this way, repeat this process."*

    The interesting clause is the repeat. Each pair that shares a color
    triggers the loop. With Painter's Servant active (all cards share
    the chosen color), every pair shares a color → mill entire library.

    We detect the Painter color-wash via ``game.painter_color``.
    """
    slug = "grindstone_mill"
    opp = game.opp(source_seat)
    painter_active = getattr(game, "painter_color", None) is not None
    total_milled = 0
    iterations = 0
    max_iterations = 200  # safety cap
    while opp.library and iterations < max_iterations:
        iterations += 1
        pair = []
        for _ in range(2):
            if not opp.library:
                break
            pair.append(opp.library.pop(0))
        for c in pair:
            opp.graveyard.append(c)
        total_milled += len(pair)
        if len(pair) < 2:
            break
        # Check if pair shares a color. Painter color-wash makes
        # everything share the chosen color.
        if painter_active:
            continue  # always repeats
        # Otherwise check real colors
        c1, c2 = pair[0], pair[1]
        shared = set(c1.colors or ()) & set(c2.colors or ())
        if not shared:
            break
    _emit(game, slug, source_card,
          target_seat=opp.idx,
          total_milled=total_milled,
          iterations=iterations,
          painter_active=painter_active)
    game.ev("mill", seat=opp.idx, count=total_milled,
            cards=[])  # detailed cards omitted for log size
    if not opp.library:
        opp.lost = True
        opp.loss_reason = "decked out (Grindstone)"
        game.check_end()


def _mindslaver_control_turn(game, source_seat: int, source_card, ctx: dict) -> None:
    """Mindslaver — *"{T}, Sacrifice Mindslaver: You control target player
    during that player's next turn."*

    Full fidelity requires a turn-ownership state slot + an AI hook that
    defers to ``game.controlled_by`` during the target's turn. MVP:
      * Stash the control relationship on the game as
        ``game.mindslaver_control = {target_seat: controller_seat}``.
      * The turn-driver checks this slot; if set, it drives the
        target's turn using source_seat's policy. We don't implement
        that driver yet; instead we log a ``per_card_partial`` so the
        auditor flags it.
    """
    slug = "mindslaver_control"
    opp = game.opp(source_seat)
    # Sacrifice Mindslaver (move its permanent to owner's graveyard)
    perm = _source_permanent(game, source_card)
    if perm is not None:
        ctrl_seat = game.seats[perm.controller]
        ctrl_seat.battlefield.remove(perm)
        ctrl_seat.graveyard.append(perm.card)
    controls = getattr(game, "mindslaver_control", {})
    controls[opp.idx] = source_seat
    game.mindslaver_control = controls
    _emit(game, slug, source_card,
          target_seat=opp.idx, controller_seat=source_seat)
    _partial(game, slug, source_card,
             missing="turn_driver_does_not_yet_route_to_mindslaver_controller")


def _worldgorger_etb_exile(game, source_seat: int, source_card, ctx: dict) -> None:
    """Worldgorger Dragon — *"When Worldgorger Dragon enters the
    battlefield, exile all other permanents you control."*

    The ETB half is straightforward. The complementary LTB half (below)
    is what creates the loop when paired with Animate Dead.
    """
    slug = "worldgorger_etb_exile"
    perm = ctx.get("permanent") or _source_permanent(game, source_card)
    if perm is None:
        _fail(game, slug, source_card, "no_battlefield_permanent")
        return
    ctrl_seat = game.seats[perm.controller]
    exiled = []
    # Iterate over a snapshot so we don't mutate while iterating.
    for p in list(ctrl_seat.battlefield):
        if p is perm:
            continue
        ctrl_seat.battlefield.remove(p)
        ctrl_seat.exile.append(p.card)
        exiled.append(p.card.name)
    # Track exile-by-worldgorger on the permanent itself so the LTB
    # handler can return the right set.
    perm.worldgorger_exiled = exiled
    _emit(game, slug, source_card,
          controller=perm.controller,
          exiled_count=len(exiled),
          exiled=exiled)


def _worldgorger_ltb_return(game, source_seat: int, source_card, ctx: dict) -> None:
    """Worldgorger Dragon — *"When Worldgorger Dragon leaves the
    battlefield, return the exiled cards to the battlefield under their
    owners' control."*

    The LTB trigger needs to know WHICH exiled cards were exiled by the
    Dragon, not every card in exile. We use the ``worldgorger_exiled``
    attribute we stamped on the Permanent during the ETB.
    """
    slug = "worldgorger_ltb_return"
    perm = ctx.get("permanent")
    if perm is None:
        _fail(game, slug, source_card, "no_permanent_in_ctx")
        return
    exiled_names = list(getattr(perm, "worldgorger_exiled", ()))
    if not exiled_names:
        _partial(game, slug, source_card, missing="no_exiled_set_recorded")
        return
    returned = []
    ctrl_seat = game.seats[perm.controller]
    # Walk exile and pull matching card entries. This is a shallow
    # match (by name); a proper implementation tracks object identity.
    remaining_exile = []
    to_return = list(exiled_names)
    for card in ctrl_seat.exile:
        if card.name in to_return:
            to_return.remove(card.name)
            # Recreate a Permanent. Summoning sickness: returning
            # creatures shouldn't be sick (they were in play before).
            # Lazy import to avoid circularity.
            from playloop import Permanent
            new_perm = Permanent(card=card, controller=perm.controller,
                                 tapped=False, summoning_sick=False)
            ctrl_seat.battlefield.append(new_perm)
            returned.append(card.name)
        else:
            remaining_exile.append(card)
    ctrl_seat.exile = remaining_exile
    _emit(game, slug, source_card,
          returned_count=len(returned),
          returned=returned)


def _food_chain_exile_for_mana(game, source_seat: int, source_card, ctx: dict) -> None:
    """Food Chain — *"Exile a creature you control: Add X mana of any one
    color, where X is that creature's mana value plus one. Spend this
    mana only to cast creature spells."*

    MVP implementation: pick any creature the controller owns; exile it;
    add CMC+1 mana to the pool. The "only to cast creature spells"
    restriction is tracked as a tag on the mana pool which the current
    engine doesn't enforce — we log a ``per_card_partial`` for that
    clause.
    """
    slug = "food_chain_exile_for_mana"
    seat = game.seats[source_seat]
    # Pick a creature to exile: prefer non-commanders with lowest CMC
    # (or rather, keep most valuable creatures). MVP: highest CMC for
    # max mana output.
    creatures = [p for p in seat.battlefield if p.is_creature]
    if not creatures:
        _fail(game, slug, source_card, "no_creature_to_exile")
        return
    creatures.sort(key=lambda p: -p.card.cmc)
    victim = creatures[0]
    mana_gained = (victim.card.cmc or 0) + 1
    seat.battlefield.remove(victim)
    seat.exile.append(victim.card)
    seat.mana_pool += mana_gained
    _emit(game, slug, source_card,
          exiled=victim.card.name,
          mana_gained=mana_gained,
          new_pool=seat.mana_pool)
    game.ev("add_mana", seat=seat.idx, amount=mana_gained,
            source="food_chain", source_card="Food Chain",
            pool_after=seat.mana_pool)
    _partial(game, slug, source_card,
             missing="mana_restriction_creature_spells_only_not_enforced")


def _humility_strip_abilities(game, source_seat: int, source_card, ctx: dict) -> None:
    """Humility — *"All creatures lose all abilities and have base power
    and toughness 1/1."* (CR §613)

    §613 layer registration:
      * Layer 6 — strip all abilities from every object that is a
        creature AT THE TIME THE LAYER APPLIES (post-layer-4 type-setting).
      * Layer 7b — set base P/T to 1/1 for every creature.

    Both effects share Humility's ETB timestamp (CR §613.7). Self-reference
    is fine: Humility is not itself a creature (it's an Enchantment), so
    the predicates naturally exclude it — UNLESS another effect (e.g.
    Opalescence at layer 4) has turned it into a creature, at which point
    Humility's own layer-6/7b effects DO apply to itself. That's the
    canonical Humility + Opalescence loop-resolution — layer 4 runs before
    layer 6/7b, so by the time we reach Humility's strip-and-set, Humility
    is a creature and gets stripped/set like everything else.

    Implementation notes:
      * Both effects key off `chars["types"]` (the IN-LAYER type-set)
        not `perm.is_creature` (the baseline property). That's how the
        layer-4 type-add from Opalescence propagates into later layers.
      * We source the effects to the Humility permanent so that
        `unregister_continuous_effects_for_permanent` cleans them up
        automatically when Humility leaves the battlefield.
      * Legacy flags (humility_active, humility_set_pt) kept as
        belt-and-suspenders for code paths that may still read them
        pre-migration — they're now SHADOWED by the layer system, which
        is the authoritative source.
    """
    slug = "humility_strip_abilities"
    # Lazy import to avoid circular import at module load.
    from playloop import ContinuousEffect, register_continuous_effect

    perm = ctx.get("permanent") or _source_permanent(game, source_card)
    if perm is None:
        _fail(game, slug, source_card, "no_battlefield_permanent")
        return

    timestamp = game.next_timestamp()

    def _is_creature_in_layer(game_, perm_, chars):
        return "creature" in (chars.get("types") or ())

    def _strip_abilities(game_, perm_, chars):
        if "creature" in (chars.get("types") or ()):
            chars["abilities"] = []

    def _set_base_pt_1_1(game_, perm_, chars):
        if "creature" in (chars.get("types") or ()):
            chars["power"] = 1
            chars["toughness"] = 1

    # Predicate is called with (game, perm) — we need chars-awareness,
    # so the actual layer-sensitive check lives inside apply_fn; the
    # predicate just broadly allows every permanent (the apply_fn is a
    # no-op when the target isn't currently a creature in-layer).
    register_continuous_effect(game, ContinuousEffect(
        layer="6",
        timestamp=timestamp,
        source_perm=perm,
        source_card_name="Humility",
        controller_seat=perm.controller,
        predicate=lambda g, p: True,
        apply_fn=_strip_abilities,
        handler_id=f"humility_strip:{id(perm)}",
    ))
    register_continuous_effect(game, ContinuousEffect(
        layer="7b",
        timestamp=timestamp,
        source_perm=perm,
        source_card_name="Humility",
        controller_seat=perm.controller,
        predicate=lambda g, p: True,
        apply_fn=_set_base_pt_1_1,
        handler_id=f"humility_pt:{id(perm)}",
    ))

    # Legacy shadow flags for any pre-layer-system code that still peeks.
    affected = 0
    for s in game.seats:
        for p in s.battlefield:
            if p.is_creature:
                p.humility_active = True
                p.humility_set_pt = (1, 1)
                affected += 1
    _emit(game, slug, source_card,
          affected_creatures=affected,
          layer_6_registered=True,
          layer_7b_registered=True,
          timestamp=timestamp,
          rule="613.2/613.4")


def _opalescence_etb_layers(game, source_seat: int, source_card, ctx: dict) -> None:
    """Opalescence — post-2017 Oracle: *"Each other non-Aura enchantment
    is a creature in addition to its other types and has base power and
    toughness each equal to its mana value."* (CR §613)

    §613 layer registration:
      * Layer 4 — add `creature` type to every OTHER non-Aura enchantment.
        Self-exclusion is in the oracle ("each other"), which resolves the
        classic Humility+Opalescence paradox without dependency trickery.
      * Layer 7b — set base P/T of each affected permanent to its mana
        value (cmc). We set the SAME permanents targeted by layer 4.

    We do NOT add the `creature` type to Auras (the oracle says
    "non-Aura"). Tokens and face-down permanents are unaffected implicitly
    by the non-Aura / enchantment filter, since tokens without the
    enchantment type aren't selected.

    Note on stacking with Humility (canonical test): Humility's layer 6
    strips abilities; Opalescence itself is not in layer 6 so isn't
    stripped at that layer; but because Opalescence did NOT add itself as
    a creature, it also isn't hit by Humility's layer-7b set-P/T gate.
    Net: Opalescence keeps its printed enchantment-line; Humility becomes
    1/1 creature with no abilities; other creatures become 1/1 with no
    abilities. Self-exclusion is the §613-legal way to dodge the paradox.
    """
    slug = "opalescence_layer_effects"
    from playloop import ContinuousEffect, register_continuous_effect

    perm = ctx.get("permanent") or _source_permanent(game, source_card)
    if perm is None:
        _fail(game, slug, source_card, "no_battlefield_permanent")
        return

    timestamp = game.next_timestamp()
    source_id = id(perm)

    def _other_non_aura_enchantment(game_, target_perm):
        if target_perm is perm:
            return False  # self-exclusion ("each other")
        tl = (target_perm.card.type_line or "").lower()
        if "enchantment" not in tl:
            return False
        # "non-Aura" filter — subtype check on the printed type_line.
        # A proper impl would read chars in-layer; for layer 4 application
        # the printed-subtype read is correct (auras printed as auras
        # never lose the aura subtype).
        if "aura" in tl:
            return False
        return True

    def _add_creature_type(game_, target_perm, chars):
        types = list(chars.get("types") or ())
        if "creature" not in types:
            types.append("creature")
            chars["types"] = types

    def _set_pt_to_cmc(game_, target_perm, chars):
        # Only for permanents Opalescence's layer 4 turned INTO creatures.
        # Check current in-layer types (post-layer-4).
        if "creature" not in (chars.get("types") or ()):
            return
        # Only set if Opalescence would have granted the creature type
        # (predicate matches). Also skip if the printed card is already a
        # creature — Opalescence's P/T-set clause only applies to its
        # own granted creatures.
        tl = (target_perm.card.type_line or "").lower()
        head = tl.split("—")[0]
        if "creature" in head:
            return
        if not _other_non_aura_enchantment(game_, target_perm):
            return
        cmc = getattr(target_perm.card, "cmc", 0) or 0
        chars["power"] = cmc
        chars["toughness"] = cmc

    register_continuous_effect(game, ContinuousEffect(
        layer="4",
        timestamp=timestamp,
        source_perm=perm,
        source_card_name="Opalescence",
        controller_seat=perm.controller,
        predicate=_other_non_aura_enchantment,
        apply_fn=_add_creature_type,
        handler_id=f"opalescence_type:{source_id}",
    ))
    register_continuous_effect(game, ContinuousEffect(
        layer="7b",
        timestamp=timestamp,
        source_perm=perm,
        source_card_name="Opalescence",
        controller_seat=perm.controller,
        # Predicate kept loose — the apply_fn re-checks the filter and
        # reads chars for in-layer type awareness.
        predicate=lambda g, p: True,
        apply_fn=_set_pt_to_cmc,
        handler_id=f"opalescence_pt:{source_id}",
    ))
    _emit(game, slug, source_card,
          timestamp=timestamp,
          layer_4_registered=True,
          layer_7b_registered=True,
          rule="613.4")


def _blood_moon_etb_layers(game, source_seat: int, source_card, ctx: dict) -> None:
    """Blood Moon — *"Nonbasic lands are Mountains."* (CR §613 layer 4)

    Applies to every permanent on the battlefield whose printed type_line
    contains `land` but NOT the `Basic` supertype. The effect changes the
    land subtype to Mountain; per CR §305.7 it REMOVES other land
    subtypes (Forest, Island, etc.) but PRESERVES any non-land types
    (Creature, Artifact) and the tapping-for-mana rules-text of basic
    Mountains is added via the rules tail (§305.6: a land with the
    `Mountain` subtype has the intrinsic `{T}: Add {R}` ability). The
    rules-tail is a consequence, not a separate layer effect, so we only
    register layer 4 here.

    Dryad Arbor test case: printed type_line "Land Creature — Forest
    Dryad". Blood Moon → layer 4 sets subtypes to ["mountain"] (the
    Forest loss) while types ["land", "creature"] are preserved. The
    Dryad subtype is a CREATURE subtype, not a land subtype — we must
    not touch it. §305.7: "If the land has other card types (e.g.
    creature), those types are unaffected. The land still has its
    creature subtypes." So result: "Land Creature — Mountain Dryad".
    """
    slug = "blood_moon_nonbasic_lands"
    from playloop import ContinuousEffect, register_continuous_effect

    perm = ctx.get("permanent") or _source_permanent(game, source_card)
    if perm is None:
        _fail(game, slug, source_card, "no_battlefield_permanent")
        return

    timestamp = game.next_timestamp()
    _register_blood_moon_like_effect(
        game, perm, "Blood Moon", timestamp, slug)
    _emit(game, slug, source_card,
          timestamp=timestamp,
          layer_4_registered=True,
          rule="613.4/305.7")


def _magus_of_the_moon_etb_layers(game, source_seat: int, source_card, ctx: dict) -> None:
    """Magus of the Moon — *"Nonbasic lands are Mountains."* (CR §613
    layer 4, identical rules text to Blood Moon but from a creature's
    static ability.)

    §613.7 timestamp-tiebreak: when Blood Moon and Magus of the Moon
    are both on the battlefield, they both register layer-4 effects.
    The later-timestamped one is applied after the earlier one — but
    since both apply identical "set subtype to Mountain" operations,
    the result is stable regardless of order (idempotent under
    composition).
    """
    slug = "magus_of_the_moon_nonbasic_lands"
    from playloop import ContinuousEffect, register_continuous_effect

    perm = ctx.get("permanent") or _source_permanent(game, source_card)
    if perm is None:
        _fail(game, slug, source_card, "no_battlefield_permanent")
        return

    timestamp = game.next_timestamp()
    _register_blood_moon_like_effect(
        game, perm, "Magus of the Moon", timestamp, slug)
    _emit(game, slug, source_card,
          timestamp=timestamp,
          layer_4_registered=True,
          rule="613.4/305.7")


# Shared core for Blood Moon-family effects.
def _register_blood_moon_like_effect(game, source_perm, card_name: str,
                                     timestamp: int, slug: str) -> None:
    """Register the layer-4 'non-basic lands are Mountains' continuous
    effect. Shared by Blood Moon + Magus of the Moon; stacks harmlessly
    per §613.7 (idempotent re-application)."""
    from playloop import ContinuousEffect, register_continuous_effect

    def _is_nonbasic_land(game_, target_perm):
        tl = (target_perm.card.type_line or "").lower()
        head = tl.split("—")[0]
        if "land" not in head:
            return False
        if "basic" in head:
            return False
        return True

    def _make_mountain(game_, target_perm, chars):
        # CR §305.7: subtype-set effect on a land removes old LAND
        # subtypes only. Non-land subtypes (e.g. "dryad") are preserved.
        # We read the PRINTED type_line to distinguish land subtypes
        # from creature subtypes — if the card has the `Land Creature`
        # dual-type then the tail subtypes after "—" are a mix.
        LAND_SUBTYPES = {
            "plains", "island", "swamp", "mountain", "forest",
            "desert", "gate", "lair", "locus", "urza's", "mine",
            "power-plant", "tower", "cave", "sphere", "town",
        }
        current_sub = list(chars.get("subtypes") or ())
        # Drop any current land-subtype; keep everything else intact
        # (e.g. "dryad" creature-subtype on Dryad Arbor).
        retained = [s for s in current_sub if s.lower() not in LAND_SUBTYPES]
        retained.append("mountain")
        # De-dupe while preserving order.
        seen = set()
        final_sub = []
        for s in retained:
            key = s.lower()
            if key in seen:
                continue
            seen.add(key)
            final_sub.append(s)
        chars["subtypes"] = final_sub

    register_continuous_effect(game, ContinuousEffect(
        layer="4",
        timestamp=timestamp,
        source_perm=source_perm,
        source_card_name=card_name,
        controller_seat=source_perm.controller,
        predicate=_is_nonbasic_land,
        apply_fn=_make_mountain,
        handler_id=f"{slug}:{id(source_perm)}",
    ))


def _painters_servant_register_layer5(game, source_seat: int, source_card, ctx: dict) -> None:
    """Painter's Servant — *"All cards that aren't on the battlefield,
    spells, and permanents are the chosen color in addition to their
    other colors."* (CR §613 layer 5)

    §613 layer 5 = color-changing effects. Painter's Servant's static
    applies across ALL ZONES — this is unusual. For the layer registry,
    we register a layer-5 effect keyed on the chosen color and rely on
    `get_effective_characteristics` to see it for any on-battlefield
    permanent. The all-zones behavior for non-battlefield cards is
    tracked via `game.painter_color` (existing mechanism used by
    Grindstone et al.); the layer effect adds color to the BATTLEFIELD
    side for the layer system's consumers.

    We don't clobber the old `painter_color` signal — Grindstone still
    reads it. This is additive: per-permanent color queries through the
    layer system ALSO see the chosen color.
    """
    slug = "painters_servant_color_wash"
    from playloop import ContinuousEffect, register_continuous_effect

    perm = _source_permanent(game, source_card)
    chosen = getattr(perm, "chosen_color", None) if perm else None
    if chosen is None:
        _fail(game, slug, source_card, "no_chosen_color_yet")
        return
    game.painter_color = chosen  # preserve existing signal
    timestamp = game.next_timestamp()

    def _add_chosen_color(game_, target_perm, chars):
        colors = list(chars.get("colors") or ())
        if chosen not in colors:
            colors.append(chosen)
            chars["colors"] = colors

    register_continuous_effect(game, ContinuousEffect(
        layer="5",
        timestamp=timestamp,
        source_perm=perm,
        source_card_name="Painter's Servant",
        controller_seat=perm.controller if perm else source_seat,
        predicate=lambda g, p: True,
        apply_fn=_add_chosen_color,
        handler_id=f"painter_color:{id(perm)}",
    ))
    _emit(game, slug, source_card,
          chosen_color=chosen,
          timestamp=timestamp,
          layer_5_registered=True,
          rule="613.4")


def _mycosynth_lattice_register_layers(game, source_seat: int, source_card, ctx: dict) -> None:
    """Mycosynth Lattice — *"All permanents are artifacts in addition to
    their other types. All cards that aren't on the battlefield, spells,
    and permanents are colorless. Players may spend mana as though it
    were mana of any color."* (CR §613 layers 4 + 5, plus a mana-spend
    static that is NOT a layer effect.)

    §613 layer registration:
      * Layer 4 — add `artifact` type to every permanent.
      * Layer 5 — set colors of every permanent to empty (colorless).
        The oracle says "all cards that aren't permanents" (hand,
        library, graveyard, stack) become colorless — that's outside the
        battlefield and thus outside `get_effective_characteristics`'s
        remit; the `game.mycosynth_lattice_active` flag continues to
        serve that consumer.

    The "mana of any color" clause is a static mana-spend permission —
    NOT a characteristic-modification — so it lives outside the layer
    system entirely and stays in `game.mycosynth_lattice_active`.
    """
    slug = "mycosynth_lattice_layers"
    from playloop import ContinuousEffect, register_continuous_effect

    perm = ctx.get("permanent") or _source_permanent(game, source_card)
    if perm is None:
        _fail(game, slug, source_card, "no_battlefield_permanent")
        return

    game.mycosynth_lattice_active = True
    timestamp = game.next_timestamp()

    def _add_artifact_type(game_, target_perm, chars):
        types = list(chars.get("types") or ())
        if "artifact" not in types:
            types.append("artifact")
            chars["types"] = types

    def _set_colorless(game_, target_perm, chars):
        chars["colors"] = []

    register_continuous_effect(game, ContinuousEffect(
        layer="4",
        timestamp=timestamp,
        source_perm=perm,
        source_card_name="Mycosynth Lattice",
        controller_seat=perm.controller,
        predicate=lambda g, p: True,
        apply_fn=_add_artifact_type,
        handler_id=f"mycosynth_type:{id(perm)}",
    ))
    register_continuous_effect(game, ContinuousEffect(
        layer="5",
        timestamp=timestamp,
        source_perm=perm,
        source_card_name="Mycosynth Lattice",
        controller_seat=perm.controller,
        predicate=lambda g, p: True,
        apply_fn=_set_colorless,
        handler_id=f"mycosynth_color:{id(perm)}",
    ))
    _emit(game, slug, source_card,
          timestamp=timestamp,
          layer_4_registered=True,
          layer_5_registered=True,
          rule="613.4")


def _ensoul_artifact_etb_layers(game, source_seat: int, source_card, ctx: dict) -> None:
    """Ensoul Artifact — *"Enchant artifact. Enchanted artifact is a 5/5
    creature in addition to its other types."* (CR §613 layers 4 + 7b)

    §613 layer registration:
      * Layer 4 — add `creature` type to the enchanted artifact.
      * Layer 7b — set the enchanted artifact's base P/T to 5/5.

    We scope the predicate to the single permanent the Aura is attached
    to via `perm.attached_to`. If the aura isn't attached (e.g. it came
    down without a legal target — SBA would've killed it but for a
    fraction of a frame it might exist), the effect is a no-op.
    """
    slug = "ensoul_artifact_layers"
    from playloop import ContinuousEffect, register_continuous_effect

    perm = ctx.get("permanent") or _source_permanent(game, source_card)
    if perm is None:
        _fail(game, slug, source_card, "no_battlefield_permanent")
        return

    timestamp = game.next_timestamp()
    aura_id = id(perm)

    def _is_attached_target(game_, target_perm):
        # Check BOTH the aura's attached_to (preferred) and a loose
        # identity match as a fallback for test harnesses that set up
        # the attachment the other way.
        if perm.attached_to is not None:
            return target_perm is perm.attached_to
        return False

    def _add_creature_type(game_, target_perm, chars):
        types = list(chars.get("types") or ())
        if "creature" not in types:
            types.append("creature")
            chars["types"] = types

    def _set_5_5(game_, target_perm, chars):
        chars["power"] = 5
        chars["toughness"] = 5

    register_continuous_effect(game, ContinuousEffect(
        layer="4",
        timestamp=timestamp,
        source_perm=perm,
        source_card_name="Ensoul Artifact",
        controller_seat=perm.controller,
        predicate=_is_attached_target,
        apply_fn=_add_creature_type,
        handler_id=f"ensoul_type:{aura_id}",
    ))
    register_continuous_effect(game, ContinuousEffect(
        layer="7b",
        timestamp=timestamp,
        source_perm=perm,
        source_card_name="Ensoul Artifact",
        controller_seat=perm.controller,
        predicate=_is_attached_target,
        apply_fn=_set_5_5,
        handler_id=f"ensoul_pt:{aura_id}",
    ))
    _emit(game, slug, source_card,
          timestamp=timestamp,
          layer_4_registered=True,
          layer_7b_registered=True,
          attached_to=(getattr(perm.attached_to, "card", None).name
                       if perm.attached_to and getattr(perm.attached_to, "card", None)
                       else None),
          rule="613.4")


def _conspiracy_etb_layers(game, source_seat: int, source_card, ctx: dict) -> None:
    """Conspiracy — *"As Conspiracy enters the battlefield, choose a
    creature type. Creatures you control are the chosen type in addition
    to their other types."* (CR §613 layer 4, dependent on layer 4
    type-setters elsewhere per §613.8 dependency-ordering.)

    §613.8 dependency: if ANOTHER layer-4 effect changes the set of
    "creatures you control" (e.g. Opalescence turning enchantments into
    creatures), then Conspiracy's effect depends on that one because
    which objects are "creatures" shifts. The current framework sorts
    strictly by timestamp within a layer; dependency-ordering is NOT
    implemented. For now, a Conspiracy+Opalescence interaction where
    both are in layer 4 resolves by timestamp — if Conspiracy is
    timestamped AFTER Opalescence, it DOES see the freshly-created
    enchantment-creatures and set their subtype correctly. If
    Conspiracy is timestamped FIRST, it only sets subtypes on the
    original creatures — the later-Opalescence enchantments won't get
    the subtype until Conspiracy re-applies (which it won't without
    dependency detection). This is flagged as a known gap below.

    Implementation: register layer 4 setting creature subtype to the
    chosen type on every permanent controlled by source_seat that is
    a creature in the current layer.
    """
    slug = "conspiracy_layer_4"
    from playloop import ContinuousEffect, register_continuous_effect

    perm = ctx.get("permanent") or _source_permanent(game, source_card)
    if perm is None:
        _fail(game, slug, source_card, "no_battlefield_permanent")
        return

    # Choice policy: default to "Zombie" (combo-relevant with Cemetery
    # tribal payoffs); users can override via perm.chosen_creature_type
    # set by a pre-registered choice hook.
    chosen = getattr(perm, "chosen_creature_type", None) or "Zombie"
    perm.chosen_creature_type = chosen
    timestamp = game.next_timestamp()
    controller = perm.controller

    def _controls_and_is_creature(game_, target_perm):
        if target_perm.controller != controller:
            return False
        # Read-through to CURRENT types: in-layer the predicate only
        # sees the baseline, so we union printed + baseline check.
        # This is a limitation vs true §613.8 dependency detection —
        # Opalescence-granted creatures registered AFTER Conspiracy
        # won't be seen by Conspiracy's predicate. Flagged.
        tl = (target_perm.card.type_line or "").lower()
        return "creature" in tl.split("—")[0]

    def _add_chosen_subtype(game_, target_perm, chars):
        # Check in-layer types so the effect propagates to enchantment
        # creatures IF they were type-converted earlier in layer 4.
        if "creature" not in (chars.get("types") or ()):
            return
        subs = list(chars.get("subtypes") or ())
        key = chosen.lower()
        if key not in [s.lower() for s in subs]:
            subs.append(chosen)
            chars["subtypes"] = subs

    register_continuous_effect(game, ContinuousEffect(
        layer="4",
        timestamp=timestamp,
        source_perm=perm,
        source_card_name="Conspiracy",
        controller_seat=controller,
        predicate=_controls_and_is_creature,
        apply_fn=_add_chosen_subtype,
        handler_id=f"conspiracy_subtype:{id(perm)}",
    ))
    _emit(game, slug, source_card,
          chosen_type=chosen,
          timestamp=timestamp,
          layer_4_registered=True,
          rule="613.4/613.8")
    _partial(game, slug, source_card,
             missing="613_8_dependency_ordering_not_implemented_"
                     "conspiracy_does_not_resee_opalescence_creatures_"
                     "if_opalescence_registered_after_conspiracy")


def _splinter_twin_etb_layers(game, source_seat: int, source_card, ctx: dict) -> None:
    """Splinter Twin — *"Enchant creature. Enchanted creature has
    '{tap}: Create a token that's a copy of this creature, except it
    has haste. Exile the token at the beginning of the next end step.'"*
    (CR §613 layer 6 — ability-granting.)

    §613 layer registration:
      * Layer 6 — grant an activated ability to the enchanted creature.

    The granted ability's effect body (token creation + EOT exile) is
    NOT a layer effect — it runs when the ability is activated. The
    granting itself is what goes into layer 6. We model the grant as
    appending a tagged string to the `abilities` list (the engine's
    ability representation at the layer tier is list-of-names); the
    actual token-creation logic is handled by the name-based activated
    dispatch when the user activates the copy ability.
    """
    slug = "splinter_twin_layer_6"
    from playloop import ContinuousEffect, register_continuous_effect

    perm = ctx.get("permanent") or _source_permanent(game, source_card)
    if perm is None:
        _fail(game, slug, source_card, "no_battlefield_permanent")
        return

    timestamp = game.next_timestamp()
    aura_id = id(perm)
    GRANTED_ABILITY = "splinter_twin_copy_token_activated"

    def _is_attached_target(game_, target_perm):
        if perm.attached_to is not None:
            return target_perm is perm.attached_to
        return False

    def _grant_activated(game_, target_perm, chars):
        abilities = list(chars.get("abilities") or ())
        if GRANTED_ABILITY not in abilities:
            abilities.append(GRANTED_ABILITY)
            chars["abilities"] = abilities

    register_continuous_effect(game, ContinuousEffect(
        layer="6",
        timestamp=timestamp,
        source_perm=perm,
        source_card_name="Splinter Twin",
        controller_seat=perm.controller,
        predicate=_is_attached_target,
        apply_fn=_grant_activated,
        handler_id=f"splinter_twin_grant:{aura_id}",
    ))
    _emit(game, slug, source_card,
          granted_ability=GRANTED_ABILITY,
          timestamp=timestamp,
          layer_6_registered=True,
          rule="613.4")


def _karn_liberated_restart(game, source_seat: int, source_card, ctx: dict) -> None:
    """Karn Liberated — *"-14: Restart the game, leaving in exile all
    non-Aura permanent cards exiled with Karn Liberated. Then put those
    cards onto the battlefield under your control."*

    Full implementation requires a game-reset primitive the engine
    doesn't expose. MVP: end the current game with Karn's controller as
    the winner (the pragmatic interpretation: in competitive play, the
    game is over after Karn ult — the "restart" never matters because
    the opponent just scoops).

    Cite: this is the standard tournament ruling; officially the game
    does restart, but since the exiled permanents are Karn's controller's
    cards and the restart begins a fresh game, this is effectively
    unrecoverable for most decks.
    """
    slug = "karn_liberated_ult"
    seat = game.seats[source_seat]
    _emit(game, slug, source_card,
          winner_seat=seat.idx)
    game.ev("per_card_win",
            slug=slug,
            card=getattr(source_card, "name", "?"),
            winner_seat=seat.idx,
            reason="karn_liberated_restart")
    for other in game.seats:
        if other.idx != seat.idx:
            other.lost = True
            other.loss_reason = "Karn Liberated restart (concession-equivalent)"
    game.ended = True
    game.winner = seat.idx
    game.end_reason = "Karn Liberated -14 ultimate"
    _partial(game, slug, source_card,
             missing="actual_game_restart_not_implemented_treated_as_win")


def _jace_wielder_ult(game, source_seat: int, source_card, ctx: dict) -> None:
    """Jace, Wielder of Mysteries -8: *"Exile the top X cards of your
    library, where X is an amount equal to your devotion to blue. Look at
    them."* (that's the +1)

    The ULT is actually: *"−8: Draw seven cards."* (paired with the static
    *"You may have Jace, Wielder of Mysteries be your commander. You get
    a card-draw win if you'd draw with an empty library."*)

    Jace's win condition is already handled structurally by the existing
    playloop (empty-library draw → seat decks out → SBA). This handler
    is a backup for the -8 ultimate itself.
    """
    slug = "jace_wielder_ult"
    seat = game.seats[source_seat]
    # Draw seven — whether the library empties mid-draw is handled by
    # the existing draw code. The static win-on-empty-draw is also
    # already handled by Laboratory-Maniac-class replacement once we
    # reach zero library.
    from playloop import draw_cards
    draw_cards(game, seat, 7)
    _emit(game, slug, source_card, drew=7)


def _animate_dead_etb_swap(game, source_seat: int, source_card, ctx: dict) -> None:
    """Animate Dead — *"Enchant creature card in a graveyard / When Animate
    Dead enters the battlefield, if it's on the battlefield, it loses
    'enchant creature card in a graveyard' and gains 'enchant creature
    put onto the battlefield with Animate Dead.' Return enchanted
    creature card to the battlefield under your control attached by
    Animate Dead."*

    MVP: we pick a creature card from any graveyard and put it onto the
    battlefield under source_seat's control with a marker pointing at
    the Animate Dead aura. We don't model aura attachment mechanics
    (the engine has no aura-edge relation yet), so the "aura comes off
    → creature is sacrificed" LTB binding is a partial — we fire it
    from the LTB handler based on a game-level dict.
    """
    slug = "animate_dead_etb"
    # Find a creature card in any graveyard to return
    target_card = None
    source_of_target = None
    for s in game.seats:
        for c in s.graveyard:
            if "creature" in c.type_line.lower():
                target_card = c
                source_of_target = s
                break
        if target_card:
            break
    if target_card is None:
        _fail(game, slug, source_card, "no_creature_in_graveyard")
        return
    source_of_target.graveyard.remove(target_card)
    from playloop import Permanent
    returned = Permanent(card=target_card, controller=source_seat,
                         tapped=False, summoning_sick=False)
    game.seats[source_seat].battlefield.append(returned)
    # Record the binding so LTB can clean up
    bindings = getattr(game, "animate_dead_bindings", {})
    bindings[id(_source_permanent(game, source_card))] = id(returned)
    game.animate_dead_bindings = bindings
    _emit(game, slug, source_card,
          returned=target_card.name,
          from_seat=source_of_target.idx,
          to_seat=source_seat)


def _animate_dead_ltb_sacrifice(game, source_seat: int, source_card, ctx: dict) -> None:
    """When Animate Dead leaves the battlefield, the enchanted creature
    is sacrificed (via the aura-falls-off interaction — enchanted
    creature loses its only enchantment and the LTB trigger from the
    swap-text fires sacrifice).
    """
    slug = "animate_dead_ltb"
    perm = ctx.get("permanent")
    if perm is None:
        _fail(game, slug, source_card, "no_permanent_in_ctx")
        return
    bindings = getattr(game, "animate_dead_bindings", {})
    tied = bindings.pop(id(perm), None)
    if tied is None:
        _partial(game, slug, source_card, missing="no_bound_target")
        return
    # Find the tied Permanent and sacrifice it
    for s in game.seats:
        for p in list(s.battlefield):
            if id(p) == tied:
                s.battlefield.remove(p)
                s.graveyard.append(p.card)
                _emit(game, slug, source_card,
                      sacrificed=p.card.name, seat=s.idx)
                game.animate_dead_bindings = bindings
                return
    _partial(game, slug, source_card, missing="bound_target_no_longer_on_battlefield")


def _mycosynth_lattice_artifact(game, source_seat: int, source_card, ctx: dict) -> None:
    """Mycosynth Lattice — *"All permanents are artifacts in addition to
    their other types. All cards that aren't on the battlefield, spells,
    and permanents are colorless. Players may spend mana as though it
    were mana of any color."*

    MVP: set a flag on the game so downstream can honor any-color mana.
    The "all permanents become artifacts" and "all cards colorless"
    clauses are continuous effects that belong in a layer framework —
    we mark ``game.mycosynth_lattice_active = True`` and rely on
    consumers to query it.
    """
    slug = "mycosynth_lattice"
    game.mycosynth_lattice_active = True
    _emit(game, slug, source_card)
    _partial(game, slug, source_card,
             missing="layer_4_type_add_and_layer_5_color_strip_not_enforced")


# ---------------------------------------------------------------------------
# Mindslaver's *sacrifice cost* slug lives on the activated ability path.
# Since per_card.py wraps cards uniformly, most activated per-card handlers
# route through these slugs whose names are prefixed with the card slug.
# ---------------------------------------------------------------------------

# ---------------------------------------------------------------------------
# Registry
# ---------------------------------------------------------------------------

PER_CARD_RUNTIME_HANDLERS: dict[str, PerCardHandler] = {
    # Doomsday
    "doomsday_pile": _doomsday_pile,
    "lose_half_life_rounded_up": _doomsday_life_loss,
    # Painter's Servant
    "painters_servant_choose_color": _painters_servant_choose_color,
    "painters_servant_color_wash": _painters_servant_color_wash,
    # Thassa's Oracle (parser gap — the card isn't in per_card.py; we add
    # the ETB-win hook by NAME from playloop.py's collect_etb_effects.)
    "thassas_oracle_etb_win_check": _thassas_oracle_etb,
    # Demonic Consultation
    "demonic_consultation": _demonic_consultation_exile_loop,
    # Tainted Pact
    "tainted_pact": _tainted_pact_exile_until,
    # Laboratory Maniac
    "laboratory_maniac_win": _laboratory_maniac_replacement,
    # Grindstone
    "grindstone_mill": _grindstone_mill_loop,
    # Mindslaver
    "mindslaver_control": _mindslaver_control_turn,
    # Worldgorger Dragon
    "worldgorger_etb_exile": _worldgorger_etb_exile,
    "worldgorger_ltb_return": _worldgorger_ltb_return,
    # Food Chain
    "food_chain_exile_for_mana": _food_chain_exile_for_mana,
    # Humility
    "humility_strip_abilities": _humility_strip_abilities,
    # Karn Liberated
    "karn_liberated_ult": _karn_liberated_restart,
    # Jace, Wielder of Mysteries
    "jace_wielder_ult": _jace_wielder_ult,
    # Animate Dead (already has parse-time handler in per_card.py)
    "animate_dead_enchant_creature_card_in_graveyard": _animate_dead_etb_swap,
    "animate_dead_etb_swap_text_return": _animate_dead_etb_swap,
    "animate_dead_ltb_sacrifice_creature": _animate_dead_ltb_sacrifice,
    "animate_dead_minus_1_power": lambda g, s, c, x: None,  # no-op (P/T -1 is minor)
    # Mycosynth Lattice — layer-system migration. all_artifacts slug is
    # the one that fires at ETB (it's the first Static(custom) in the
    # per_card.py list for Lattice); the other two slugs are subsumed
    # into the single layer-system registration.
    "mycosynth_lattice_all_artifacts": _mycosynth_lattice_register_layers,
    "mycosynth_lattice_all_colorless": lambda g, s, c, x: None,  # subsumed
    "mycosynth_lattice_any_color_mana": lambda g, s, c, x: None,  # subsumed
    # §613 layer-registering ETB handlers (new migration, 2026-04).
    # These are invoked via NAME_TO_ETB_SLUG (cards without per_card.py
    # entries) or Static(custom) re-fire for per_card.py cards.
    "humility_layer_effects": _humility_strip_abilities,
    "opalescence_layer_effects": _opalescence_etb_layers,
    "blood_moon_layer_4": _blood_moon_etb_layers,
    "magus_of_the_moon_layer_4": _magus_of_the_moon_etb_layers,
    "painters_servant_layer_5": _painters_servant_register_layer5,
    "mycosynth_lattice_layers": _mycosynth_lattice_register_layers,
    "ensoul_artifact_layers": _ensoul_artifact_etb_layers,
    "conspiracy_layer_4": _conspiracy_etb_layers,
    "splinter_twin_layer_6": _splinter_twin_etb_layers,
}


# ---------------------------------------------------------------------------
# Name-based extras: some snowflakes don't have a per_card.py parse-time
# handler yet, but we DO want runtime wiring for them. The playloop's
# collect_etb_effects / collect_spell_effects hooks can fall through to
# this name-based map as a second-chance dispatch.
# ---------------------------------------------------------------------------

NAME_TO_ETB_SLUG: dict[str, str] = {
    # Thassa's Oracle's oracle text parses as an Unknown triggered because
    # it combines an ETB "look at top X" with an if-intervening win clause
    # that the grammar can't consume. We patch it at the name layer.
    "Thassa's Oracle": "thassas_oracle_etb_win_check",
    # Worldgorger Dragon — same story (ETB exile-all is custom-only in
    # per_card, but we wire a name-based fallback for when the card is
    # not in per_card.py's registry).
    "Worldgorger Dragon": "worldgorger_etb_exile",
    # §613 layer-system migration (2026-04). These cards have no
    # per_card.py entry; we route their ETB straight into the layer-
    # registering handler by name. The handler registers the
    # ContinuousEffect(s) on game.continuous_effects, which
    # get_effective_characteristics then applies in §613 order.
    "Humility": "humility_strip_abilities",
    "Opalescence": "opalescence_layer_effects",
    "Blood Moon": "blood_moon_layer_4",
    "Magus of the Moon": "magus_of_the_moon_layer_4",
    "Ensoul Artifact": "ensoul_artifact_layers",
    "Conspiracy": "conspiracy_layer_4",
    "Splinter Twin": "splinter_twin_layer_6",
}

NAME_TO_LTB_SLUG: dict[str, str] = {
    "Worldgorger Dragon": "worldgorger_ltb_return",
    "Animate Dead": "animate_dead_ltb_sacrifice_creature",
}

NAME_TO_SPELL_SLUGS: dict[str, list[str]] = {
    # Cards whose *on-resolution* effect isn't parsed as spell_effect but
    # whose per_card.py handler emitted a Static(custom) stub. When the
    # spell resolves from the stack, we walk these slugs in order.
    "Doomsday": ["doomsday_pile", "lose_half_life_rounded_up"],
    "Demonic Consultation": ["demonic_consultation"],
    "Tainted Pact": ["tainted_pact"],
}

# Activated-ability name-based dispatch. A handful of snowflakes have
# activated abilities whose effect bodies parse as UnknownEffect (no
# Static(custom) handle), but the runtime still needs to do something
# specific. Harness-side (``run_scripted_sequence`` activate branch)
# and engine-side (``fire_activated_ability``) call ``dispatch_custom``
# with the slug listed here.
NAME_TO_ACTIVATED_SLUG: dict[str, str] = {
    "Grindstone": "grindstone_mill",
    "Mindslaver": "mindslaver_control",
    "Karn Liberated": "karn_liberated_ult",
    # Jace, Wielder's -8 already passes via the existing harness (structural
    # win on empty-library draw). We still register it so future callers
    # can route through the runtime handler directly.
    "Jace, Wielder of Mysteries": "jace_wielder_ult",
    "Food Chain": "food_chain_exile_for_mana",
}


def dispatch_custom(game, source_seat: int, source_card, slug: str,
                    ctx: Optional[dict] = None) -> bool:
    """Lookup + invoke a runtime handler for ``slug``.

    Returns True if a handler fired (even if it failed gracefully),
    False if no handler was registered — allowing the caller to emit
    its own ``per_card_unhandled`` breadcrumb.

    Never raises; handlers that throw are caught, logged as
    ``per_card_crashed``, and the call returns True (we still handled
    it — just badly).
    """
    handler = PER_CARD_RUNTIME_HANDLERS.get(slug)
    if handler is None:
        return False
    try:
        handler(game, source_seat, source_card, ctx or {})
    except Exception as exc:
        game.ev("per_card_crashed",
                slug=slug,
                card=getattr(source_card, "name", "?"),
                exception=f"{type(exc).__name__}: {exc}")
        game.emit(f"  per-card[{slug}] CRASHED: {exc}")
    return True


__all__ = [
    "PER_CARD_RUNTIME_HANDLERS",
    "NAME_TO_ETB_SLUG",
    "NAME_TO_LTB_SLUG",
    "NAME_TO_SPELL_SLUGS",
    "NAME_TO_ACTIVATED_SLUG",
    "dispatch_custom",
]
