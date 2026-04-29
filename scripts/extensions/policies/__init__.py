"""Pluggable AI decision policies for the mtgsquad engine.

Architectural directive (7174n1c):
    "D shouldn't be anything more than a hat that gets swapped out as we
    change AI pilot policy. I don't want it dug into a piece a code that
    when it changes we got to unspaghetti it"

All decision-making — mulligans, which spell to cast, which attacker to
declare, which target to pick, whether to counter a spell, which mode of
a modal spell to choose, etc. — lives BEHIND the ``Hat`` Protocol. The
engine calls ``seat.policy.method(...)`` at each decision site and never
inspects the concrete hat class. Hats carry their own internal state
(mode, memory, model references, anything) — the engine is ignorant of
it.

Swap at game setup::

    seat.policy = GreedyHat()      # current default
    seat.policy = PokerHat()       # adaptive HOLD/CALL/RAISE
    seat.policy = LLMHat(api_key=...)   # future
    seat.policy = HumanUIHat(ws)        # future

Current implementations
-----------------------
- :class:`GreedyHat`  — heuristic baseline ported from playloop.py
- :class:`PokerHat`   — HOLD/CALL/RAISE adaptive with event-driven
                        mode transitions and hysteresis.

Interface contract
------------------
See :class:`Hat` for the complete method list. A hat need only implement
the methods it wants to override — :class:`GreedyHat` serves as a
defaultable base class for subclasses that only customize a few
decisions.

Backward-compatible aliases
---------------------------
The previous names ``PlayerPolicy``, ``GreedyPolicy``, and ``PokerPolicy``
are still exported as deprecated aliases so external code keeps working.
Prefer the ``Hat`` vocabulary for new code.
"""

from __future__ import annotations

from typing import Any, Optional, Protocol, runtime_checkable


@runtime_checkable
class Hat(Protocol):
    """Protocol every AI pilot ("hat") must satisfy.

    Every method gets the full ``game`` + ``seat`` context. Hats are
    free to read any public state on those objects. They MUST NOT mutate
    game state — the engine owns mutation. Hat decisions come back
    through the return value and the engine applies them.

    Methods are invoked only at *decision sites* — points where, per CR,
    the player has a genuine choice. Deterministic resolution (triggers
    firing, SBAs, damage assignment given declared blockers, etc.)
    belongs in the engine, not the hat.
    """

    # -- Game-start / hand-selection ------------------------------------

    def choose_mulligan(self, game, seat, hand) -> bool:
        """Return True to mulligan, False to keep (CR 103.4).

        MVP: engine doesn't implement Paris/London mulligans yet, but the
        hook is in place so future mulligan-capable engines can route
        through the hat without refactoring.
        """
        ...

    # -- Main phase actions ---------------------------------------------

    def choose_land_to_play(self, game, seat, lands_in_hand) -> Optional[Any]:
        """Return the ``CardEntry`` to play as the land drop this turn,
        or ``None`` to skip it. CR §305.2 — one land per turn."""
        ...

    def choose_cast_from_hand(self, game, seat,
                              castable_cards) -> Optional[Any]:
        """Return the ``CardEntry`` from hand to cast next, or ``None`` to
        stop casting. Called in a loop until it returns ``None``.
        ``castable_cards`` is the list the engine has already filtered
        for mana-affordability and speed restrictions."""
        ...

    def choose_activation(self, game, seat, activatable) -> Optional[tuple]:
        """Pick an activated ability to fire, or ``None`` to pass.

        ``activatable`` is a list of ``(permanent, ability_index)`` tuples
        the engine has determined are legal to activate right now.
        Return one of those tuples or ``None``.
        """
        ...

    # -- Combat ----------------------------------------------------------

    def declare_attackers(self, game, seat, legal_attackers) -> list:
        """Return the subset of ``legal_attackers`` to declare as
        attackers (CR §508). Empty list = no attack."""
        ...

    def declare_attack_target(self, game, seat, attacker,
                              legal_defenders) -> int:
        """For multi-opponent games, which defending seat index this
        attacker is attacking. Only called when the list has >1 entry."""
        ...

    def declare_blockers(self, game, seat, attackers) -> dict:
        """Return ``{id(attacker): [blocker, ...]}`` assignment dict
        (CR §509). Empty list for an attacker = it's unblocked."""
        ...

    # -- Targeting / choices --------------------------------------------

    def choose_target(self, game, seat, filter_spec, legal_targets) -> Any:
        """Pick a target for a targeted effect given the parser's
        ``Filter`` spec and the list of legal objects/players the engine
        resolved. Return one of ``legal_targets`` (or a 2-tuple in
        the ``(kind, target)`` format the legacy ``pick_target`` used,
        for compatibility with existing resolvers)."""
        ...

    def respond_to_stack_item(self, game, seat,
                              stack_item) -> Optional[Any]:
        """Priority response. Return a ``CardEntry`` to cast in response
        (typically a counterspell) or ``None`` to pass priority.
        CR §117 / §702.13 (counters)."""
        ...

    def choose_mode(self, game, seat, spell, modal_choices) -> list:
        """Pick mode indices for a modal spell (CR §700.2d). Return a
        list of 0-based indices into ``modal_choices``."""
        ...

    def order_replacements(self, game, seat, candidates) -> list:
        """Order simultaneous replacement effects (CR §616.1). The
        affected player orders; returns the ``candidates`` list in the
        desired application order."""
        ...

    def choose_discard(self, game, seat, hand, n) -> list:
        """Pick ``n`` cards from ``hand`` to discard (cleanup hand-size
        overflow, forced discard effects, Mind Rot, etc.)."""
        ...

    def choose_distribution(self, game, seat, n, targets) -> dict:
        """Distribute ``n`` units (damage, counters, etc.) across
        ``targets``. Return ``{target: amount}`` with sum == n."""
        ...

    # -- Passive observation --------------------------------------------

    def observe_event(self, game, seat, event) -> None:
        """Called by the engine after EVERY ``game.ev()`` fire.

        Hat may update internal state (track opponent mana curves,
        learn tempo, count opp's land drops, whatever). This hook is the
        only way a hat is told what happened — the engine never
        inspects what the hat did with the information.

        The engine guarantees ``observe_event`` is called on EVERY seat's
        hat, not just the acting seat — so defender hats can see
        the active player's plays.
        """
        ...


# Re-exports for convenience.
from .greedy import GreedyHat           # noqa: E402
from .poker import PokerHat, PlayerMode  # noqa: E402
from .octo import OctoHat                # noqa: E402

# Backward-compat aliases — the old names still work so external code
# (and anything we missed in the rename) keeps functioning. Prefer the
# Hat vocabulary for new code.
PlayerPolicy = Hat           # deprecated alias, use Hat
GreedyPolicy = GreedyHat     # deprecated alias, use GreedyHat
PokerPolicy = PokerHat       # deprecated alias, use PokerHat
OctoPolicy = OctoHat         # deprecated alias, use OctoHat

__all__ = [
    "Hat",
    "GreedyHat",
    "PokerHat",
    "OctoHat",
    "PlayerMode",
    # Backward-compat exports.
    "PlayerPolicy",
    "GreedyPolicy",
    "PokerPolicy",
    "OctoPolicy",
]
