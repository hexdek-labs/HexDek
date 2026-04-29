"""OctoHat — stress-test policy.

Design intent (from 7174n1c, 2026-04-16):

    "Octopus-like Hat policy that will encourage each player to cast and
    activate all available abilities each turn. This might screw the
    winrate metrics and pilots will play unoptimized, using every available
    resource that they can cast with valid targets. This OctoHat should
    just try to force every nook and cranny edge case to happen."

OctoHat is NOT an optimizing policy. It intentionally plays badly to
maximize interaction coverage for the rules engine. Use when you want to
stress:

  * trigger ordering + reactive firing ("bless you" observer triggers)
  * cast-count scaling (storm observers fire on every cast)
  * mana pool churn (every available mana gets spent)
  * "may" clause optionality (OctoHat always says yes)
  * multi-mode spell resolution (OctoHat picks ALL modes, not just one)
  * activated ability dispatch (every activatable thing gets activated)
  * stack depth + loop detection (many simultaneous casts → many triggers)
  * counter-war dynamics (OctoHat casts counterspells even on its own spells)

Rules of thumb:
  - Cast every castable spell, cheap first (more casts per turn = more events)
  - Activate every activatable ability with a legal target
  - Say YES to every "may" decision
  - Never hold mana, never reserve for counters
  - Pick ALL modes on modal spells (where legal)
  - Never mulligan (keep every opener, more cards = more casts)
  - Discard LOWEST cmc when forced (protect big plays that trigger storm)

Do NOT use OctoHat to measure deck strength. Winrate with OctoHat vs
GreedyHat is noise by design.
"""

from __future__ import annotations

from typing import Any, Optional


class OctoHat:
    """Stress-test policy. Casts everything, activates everything,
    declines nothing. Winrate is meaningless; interaction coverage is
    maximized.

    Per-phase activation cap (v2 update 2026-04-16): without a limit on
    repeated activations of the same card, OctoHat hit 588x Greater Good
    + 532x Chainer, Nightmare Adept in a single game — because the
    engine's activation loop keeps calling choose_activation up to 50
    times per phase and OctoHat always returns index 0. The cap prevents
    these looping cards from consuming all 50 slots.
    """

    # Max times any single card may be activated in a single phase.
    # Set low enough to prevent infinite loops (Greater Good, Chainer,
    # Sensei's Top-rebuy, Thrasios draw-engine), high enough to allow
    # reasonable strategic activation.
    ACTIVATION_CAP_PER_CARD_PER_PHASE = 3

    def __init__(self):
        # Per-phase activation counter: (seat_idx, card_name) -> count.
        # Cleared on every phase_step_change event observed.
        self._activations_this_phase: dict = {}

    # -- Mulligan --------------------------------------------------------

    def choose_mulligan(self, game, seat, hand) -> bool:
        # Never mulligan — more cards = more casts = more interactions.
        return False

    # -- Land drop -------------------------------------------------------

    def choose_land_to_play(self, game, seat, lands_in_hand):
        if not lands_in_hand:
            return None
        if seat.lands_played_this_turn >= 1:
            return None
        # Drop a land every turn. Prefer non-basic lands first (more
        # abilities to trigger). Tie-break alphabetically for determinism.
        non_basics = [c for c in lands_in_hand
                      if "basic" not in c.type_line.lower()]
        pool = non_basics or lands_in_hand
        pool.sort(key=lambda c: c.name)
        return pool[0]

    # -- Main-phase casting ---------------------------------------------

    def choose_cast_from_hand(self, game, seat, castable_cards):
        """Cast anything castable. CHEAP FIRST so we fit more casts per
        turn — unlike GreedyHat which prefers big expensive bombs. Does
        NOT skip counterspells — cast them too, even at sorcery speed
        against an empty stack (engine will just resolve them as null-ops
        but every cast feeds cast-count observers)."""
        if not castable_cards:
            return None
        # Sort by (+cmc, name) — cheap first, alphabetic tiebreak.
        ordered = sorted(castable_cards, key=lambda c: (c.cmc, c.name))
        return ordered[0]

    # -- Activations ----------------------------------------------------

    def choose_activation(self, game, seat, activatable):
        """Activate any activatable ability — but respect a per-card
        per-phase cap to prevent runaway loops on recursive activations
        (Greater Good, Chainer, Sensei's Top, Thrasios).

        Walk the list, find the first card that hasn't hit the cap this
        phase, increment its counter + return it. If every option is
        capped, return None (signals engine to exit activation loop)."""
        if not activatable:
            return None
        for choice in activatable:
            # activatable entries may be (permanent, ability_idx) tuples
            # or just permanents — support both.
            perm = choice[0] if isinstance(choice, tuple) else choice
            card_name = getattr(getattr(perm, "card", None), "name", None)
            if card_name is None:
                return choice
            key = (seat.idx, card_name)
            current = self._activations_this_phase.get(key, 0)
            if current < self.ACTIVATION_CAP_PER_CARD_PER_PHASE:
                self._activations_this_phase[key] = current + 1
                return choice
        # All options have hit the cap — stop activating.
        return None

    # -- Combat ---------------------------------------------------------

    def declare_attackers(self, game, seat, legal_attackers):
        """Attack with EVERY creature. Even suicide attacks — the engine
        needs to resolve combat damage paths for as many creatures as
        possible."""
        return list(legal_attackers)

    def declare_attack_target(self, game, seat, attacker,
                              legal_defenders) -> int:
        """Rotate targets across opponents — DON'T focus on one seat.
        Deterministic rotation via slot-id so OctoHat spreads pressure
        evenly. This exposes more commander-damage-to-different-seats
        interactions."""
        if not legal_defenders:
            return seat.idx
        if len(legal_defenders) == 1:
            return legal_defenders[0]
        # Rotate based on attacker's permanent-id modulo number of opps.
        attacker_id = id(attacker)
        idx = attacker_id % len(legal_defenders)
        return legal_defenders[idx]

    def declare_blockers(self, game, seat, attackers) -> dict:
        """Block EVERY attacker with available creatures. Even with
        chumps that immediately die. This maximizes combat-damage
        resolutions per turn (both attacker→creature and
        blocker→attacker events fire)."""
        import playloop as _pl  # noqa: WPS433

        available = [p for p in seat.battlefield
                     if p.is_creature and not p.tapped]
        assignment: dict = {id(a): [] for a in attackers}

        for atk in attackers:
            if not available:
                break
            legal = [b for b in available if _pl.can_block(b, atk)]
            if not legal:
                continue
            # Assign up to 2 blockers per attacker to force multi-block
            # damage-distribution code paths.
            blockers_for_this = legal[:2]
            for b in blockers_for_this:
                if b in available:
                    available.remove(b)
            assignment[id(atk)] = blockers_for_this

        return assignment

    # -- Targeting ------------------------------------------------------

    def choose_target(self, game, seat, filter_spec, legal_targets):
        """Pick the FIRST legal target without threat-scoring. We want
        targets hit, not smart targets hit."""
        import playloop as _pl  # noqa: WPS433

        base = filter_spec.base
        if base in ("player", "opponent", "any_target"):
            opps = [s for s in game.seats
                    if not s.lost and s.idx != seat.idx]
            if opps:
                return "player", opps[0]
            return "player", game.seats[seat.idx]
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
                return "permanent", pool[0]
            return "none", None
        if base == "spell":
            return "none", None
        if base == "creature_card":
            return "none", None
        return "none", None

    # -- Stack response -------------------------------------------------

    def respond_to_stack_item(self, game, seat, stack_item):
        """Fire every counterspell in hand against every opponent stack
        item, regardless of threat score. We want counter-war resolution
        paths to actually fire. Still skips our own items (can't
        counter ourselves mid-chain without engine weirdness)."""
        import playloop as _pl  # noqa: WPS433

        if stack_item.controller == seat.idx:
            return None
        if stack_item.countered:
            return None
        if _pl._split_second_active(game):
            return None
        if _pl._opp_restricts_defender_to_sorcery_speed(game, seat.idx):
            return None
        # No threat-score gate — any opponent stack item triggers our
        # counter if we have one. OctoHat wants counter-wars.
        return _pl._find_counter_in_hand(game, seat)

    # -- Modal spells ---------------------------------------------------

    def choose_mode(self, game, seat, spell, modal_choices) -> list:
        """Pick ALL modes (where the spell allows multi-mode, like
        Charm spells allowing 1-of-3 or Entwine cards forcing all).
        For single-mode-only spells, return mode 0.

        The engine's resolution should enforce legal counts — OctoHat
        just wants to force the multi-mode path to run."""
        if not modal_choices:
            return []
        # Return all mode indices. If the engine rejects "too many
        # modes" it'll fall back to a legal count, which still
        # exercises more resolution paths than a single-mode choice.
        return list(range(len(modal_choices)))

    # -- Replacements ---------------------------------------------------

    def order_replacements(self, game, seat, candidates) -> list:
        """Same as GreedyHat: self-controlled first. Order doesn't
        matter for OctoHat's stress-test goal since all replacements
        fire regardless of order."""
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
        """Discard LOWEST cmc first — protect the expensive bombs that
        generate the most event volume when cast."""
        if n <= 0 or not hand:
            return []
        ranked = sorted(hand, key=lambda c: c.cmc)  # ascending
        return ranked[:min(n, len(ranked))]

    # -- Distribution ---------------------------------------------------

    def choose_distribution(self, game, seat, n, targets) -> dict:
        """Pile all damage/counters on FIRST target. This stresses
        single-target accumulation paths (commander damage threshold,
        lethal damage SBA, counter overflow, etc.) more than even-split."""
        if not targets or n <= 0:
            return {}
        out = {t: 0 for t in targets}
        out[targets[0]] = n
        return out

    # -- Observation: reset per-phase activation counter ----------------

    def observe_event(self, game, seat, event) -> None:
        # Clear the per-phase activation counter at phase boundaries.
        # CR §106.4 drains mana pools at phase ends; we similarly reset
        # our activation bookkeeping so each new phase starts fresh.
        if event is None:
            return
        etype = event.get("type", "") if isinstance(event, dict) else ""
        if etype in ("phase_step_change", "turn_start"):
            self._activations_this_phase.clear()


# Backward-compat alias pattern matching GreedyHat/PokerHat.
OctoPolicy = OctoHat
