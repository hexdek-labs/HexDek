"""Greedy / heuristic baseline ``Hat``.

Ports the decision logic that previously lived inline in
``scripts/playloop.py`` — the engine now calls
``seat.policy.method(...)`` at each decision site and ``GreedyHat``
encapsulates the original behavior so pre-existing tests and gauntlet
baselines are unaffected.

Port table
----------
================================  ================================================
Hat method                        Original call site in playloop.py
================================  ================================================
``choose_mulligan``               (no mulligan logic yet — always keeps)
``choose_land_to_play``           ``can_play_land`` (playloop.py:5677)
``choose_cast_from_hand``         cast-decision block inside ``main_phase``
                                  (playloop.py:6264-6288): sort hand by
                                  descending CMC, skip counterspells
``choose_activation``             (no activation logic on engine side yet;
                                  policy returns None so engine passes)
``declare_attackers``             ``declare_attackers`` (playloop.py:6436-6464):
                                  attack with every creature that ``can_attack``
``declare_attack_target``         ``_pick_opponent_by_threat`` (3197) —
                                  threat_score-ranked opponent
``declare_blockers``              ``declare_blockers`` (playloop.py:6699-6778):
                                  deadliest-first assignment, chump iff
                                  defender would otherwise die this turn
``choose_target``                 ``pick_target`` (playloop.py:3215-3255) and
                                  ``_pick_opponent_by_threat`` (3197-3212)
``respond_to_stack_item``         ``_get_response`` (playloop.py:5921-5945) +
                                  ``_find_counter_in_hand`` (5838)
``choose_mode``                   Choice-resolution (playloop.py:3843-3848) —
                                  "Pick first option (dumb policy)"
``order_replacements``            ``_pick_replacement`` (playloop.py:1020) —
                                  self-controlled first
``choose_discard``                cleanup hand-size (playloop.py:7069-7076) —
                                  highest-CMC first
``choose_distribution``           (new; even-split baseline)
``observe_event``                 no-op (stateless baseline)
================================  ================================================

Every method is pure — reads game/seat state, returns a decision. No
mutation. The engine applies the returned choice.
"""

from __future__ import annotations

from typing import Any, Optional


class GreedyHat:
    """Heuristic baseline. Stateless."""

    # -- Mulligan --------------------------------------------------------

    def choose_mulligan(self, game, seat, hand) -> bool:
        # Current engine doesn't implement mulligans. Keep every opener.
        # When mulligan support lands, this is where the London/Paris
        # heuristic plugs in (e.g. keep if 2 <= lands <= 5).
        return False

    # -- Land drop -------------------------------------------------------

    def choose_land_to_play(self, game, seat, lands_in_hand):
        if not lands_in_hand:
            return None
        if seat.lands_played_this_turn >= 1:
            return None
        # Original behavior: take the first land in hand.
        return lands_in_hand[0]

    # -- Main-phase casting ---------------------------------------------

    def choose_cast_from_hand(self, game, seat, castable_cards):
        """Port of the inner loop in ``main_phase``: drop the most
        expensive affordable non-counterspell first. Counterspells are
        held for opponents' stack items (the engine budgets for them
        via ``_counterspell_reserve``)."""
        # Import here to avoid circular import at module load.
        import playloop  # noqa: WPS433

        hand_playable = [
            c for c in castable_cards
            if not playloop._card_has_counterspell(c)
        ]
        if not hand_playable:
            return None
        hand_playable.sort(key=lambda c: (-c.cmc, c.name))
        return hand_playable[0]

    # -- Activations ----------------------------------------------------

    def choose_activation(self, game, seat, activatable):
        # MVP: engine doesn't surface activated-ability choices through
        # the main loop (most are in per_card handlers or resolved
        # inline). Returning None keeps the old "no activation" behavior.
        return None

    # -- Combat ---------------------------------------------------------

    def declare_attackers(self, game, seat, legal_attackers):
        """Greedy: attack with every creature that passes can_attack().
        This matches the original inline declare_attackers loop."""
        return list(legal_attackers)

    def declare_attack_target(self, game, seat, attacker,
                              legal_defenders) -> int:
        """Port of ``_pick_opponent_by_threat``. Returns a seat index."""
        import playloop  # noqa: WPS433

        if not legal_defenders:
            return seat.idx  # degenerate — caller should have filtered
        if len(legal_defenders) == 1:
            return legal_defenders[0]
        # Sort by (-threat_score, apnap_distance).
        n = len(game.seats)

        def _key(dseat_idx: int) -> tuple:
            dseat = game.seats[dseat_idx]
            dist = (dseat_idx - seat.idx) % n
            return (-playloop.threat_score(game, seat.idx, dseat), dist)

        ordered = sorted(legal_defenders, key=_key)
        return ordered[0]

    def declare_blockers(self, game, seat, attackers) -> dict:
        """Port of original ``declare_blockers`` heuristic (playloop.py
        lines 6699-6778). Deadliest attacker first; block with smallest
        survivor, else chump only if defender would otherwise die."""
        import playloop as _pl  # noqa: WPS433

        available = [p for p in seat.battlefield
                     if p.is_creature and not p.tapped]

        def atk_priority(a) -> tuple:
            dt = 1 if _pl.kw(a, "deathtouch") else 0
            ds = 1 if _pl.kw(a, "double strike") else 0
            return (-(a.power + dt * 10 + ds * 5), -a.power)

        ordered = sorted(attackers, key=atk_priority)
        incoming = sum(
            a.power * (2 if _pl.kw(a, "double strike") else 1)
            for a in attackers
        )
        life = seat.life
        assignment: dict = {id(a): [] for a in attackers}

        for atk in ordered:
            if not available:
                break
            menace = _pl.kw(atk, "menace")
            legal = [b for b in available if _pl.can_block(b, atk)]
            if not legal:
                continue
            atk_dmg = atk.power * (2 if _pl.kw(atk, "double strike") else 1)
            will_die_if_unblocked = (life - incoming <= 0)
            if _pl.kw(atk, "deathtouch"):
                survivors = []
            else:
                survivors = [
                    b for b in legal
                    if _pl._eff_toughness_remaining(b) > atk.power
                ]
            survivors.sort(key=lambda b: (b.power + b.toughness, b.toughness))

            chosen = []
            if survivors:
                chosen.append(survivors[0])
            elif will_die_if_unblocked:
                chosen.append(min(
                    legal,
                    key=lambda b: (b.power + b.toughness, b.toughness),
                ))
            if menace and chosen:
                extras = [b for b in legal if b not in chosen]
                if extras:
                    extras.sort(key=lambda b: (b.power + b.toughness))
                    chosen.append(extras[0])
                else:
                    chosen = []
            if chosen:
                for b in chosen:
                    available.remove(b)
                assignment[id(atk)] = chosen
                if _pl.kw(atk, "trample"):
                    total_t = sum(
                        _pl._eff_toughness_remaining(b) for b in chosen
                    )
                    leak = max(0, atk_dmg - total_t)
                    incoming -= (atk_dmg - leak)
                else:
                    incoming -= atk_dmg
        return assignment

    # -- Targeting ------------------------------------------------------

    def choose_target(self, game, seat, filter_spec, legal_targets):
        """Port of ``pick_target`` (playloop.py:3215). Returns the
        ``(kind, target)`` tuple the resolver expects.

        ``legal_targets`` is the engine-supplied short-list; if the
        engine passes an empty list, the policy still has access to
        ``filter_spec`` and must synthesize a choice using the same
        rules the old inline code applied.
        """
        import playloop as _pl  # noqa: WPS433

        base = filter_spec.base
        if base in ("player", "opponent", "any_target"):
            picked = _pl._pick_opponent_by_threat(game, seat.idx)
            if picked is None:
                return "player", game.seats[seat.idx]
            return "player", picked
        if base == "self":
            return "player", game.seats[seat.idx]
        if base == "creature":
            opps = [s for s in game.seats
                    if not s.lost and s.idx != seat.idx]
            pool = []
            for opp in opps:
                pool.extend(p for p in opp.battlefield if p.is_creature)
            if filter_spec.targeted:
                pool = [p for p in pool if not _pl.is_hexproof(p)]
            if pool:
                pool.sort(key=lambda p: (p.toughness, p.power))
                return "permanent", pool[0]
            return "none", None
        if base == "spell":
            return "none", None
        if base == "creature_card":
            return "none", None
        return "none", None

    # -- Stack response -------------------------------------------------

    def respond_to_stack_item(self, game, seat, stack_item):
        """Port of ``_get_response`` + ``_find_counter_in_hand``. Returns
        a ``CardEntry`` to cast in response, or None."""
        import playloop as _pl  # noqa: WPS433

        if stack_item.controller == seat.idx:
            return None
        if stack_item.countered:
            return None
        if _pl._split_second_active(game):
            return None
        if _pl._opp_restricts_defender_to_sorcery_speed(game, seat.idx):
            return None
        if _pl._stack_item_threat_score(stack_item) < 3:
            return None
        return _pl._find_counter_in_hand(game, seat)

    # -- Modal spells ---------------------------------------------------

    def choose_mode(self, game, seat, spell, modal_choices) -> list:
        """"Pick first option" — matches the legacy Choice resolver
        (playloop.py:3843-3848). A smarter policy would evaluate each
        mode's utility; baseline stays naive for reproducibility."""
        if not modal_choices:
            return []
        return [0]

    # -- Replacements ---------------------------------------------------

    def order_replacements(self, game, seat, candidates) -> list:
        """Port of ``_pick_replacement`` heuristic: self-controlled
        replacements first. The chooser is the affected-player / object
        controller per CR §616.1."""
        if not candidates:
            return []
        own = [
            r for r in candidates
            if getattr(r, "controller_seat", None) == seat.idx
        ]
        other = [r for r in candidates if r not in own]
        return own + other

    # -- Discard --------------------------------------------------------

    def choose_discard(self, game, seat, hand, n) -> list:
        """Cleanup hand-size heuristic from playloop.py:7070-7076 —
        discard highest-CMC first. Also used for any forced discard
        that doesn't specify "opponent chooses"."""
        if n <= 0 or not hand:
            return []
        ranked = sorted(hand, key=lambda c: -c.cmc)
        return ranked[:min(n, len(ranked))]

    # -- Distribution ---------------------------------------------------

    def choose_distribution(self, game, seat, n, targets) -> dict:
        """Distribute n units across targets. Baseline: even split with
        remainder dumped on the first target. Engine call sites that
        need distribution (damage from Chandra's Incinerator, counters
        from Blessing of Frost, etc.) can override per-effect later."""
        if not targets or n <= 0:
            return {}
        base, rem = divmod(n, len(targets))
        out = {t: base for t in targets}
        if rem and targets:
            out[targets[0]] += rem
        return out

    # -- Observation (no-op for stateless baseline) ---------------------

    def observe_event(self, game, seat, event) -> None:
        return None


# Backward-compat alias — the old name still works so external code
# (and anything we missed in the rename) keeps functioning. Prefer
# ``GreedyHat`` for new code.
GreedyPolicy = GreedyHat
