"""Poker-inspired HOLD/CALL/RAISE adaptive hat — v2.

The mode is INTERNAL hat state — the engine does not know it exists
and must never branch on it. Swapping this hat for another is a
one-line change: ``seat.policy = PokerHat()``.

v2 redesign (2026-04-15)
------------------------
v1 read only ``seat.battlefield`` for threat assessment, which made
graveyard-value decks (Coram), reanimator (Sin), and lifegain (Oloro)
invisible to the hat. This rewrite widens the aperture.

Seven-dimensional threat_score
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
The policy now computes ``_seven_dim_threat(game, source_seat,
target_seat)`` which sums weighted contributions from:

    1. board_power       (power of living creatures)
    2. graveyard_value   (recursion/reanimator ammunition)
    3. hand_size         (5+ cards = active draw engine)
    4. command_zone      (uncast commander = deferred threat)
    5. ramp_signal       (lands + mana rocks; ramp decks telegraph here)
    6. library_pressure  (small library → combo/self-mill warning)
    7. archetype_bonus   (detected deck type: combo/reanimator/etc.)

Each dimension is surfaced via ``_threat_breakdown`` so debug logs and
``player_mode_change`` events can cite the driving signal.

Mode semantics (v2)
-------------------
- ``HOLD`` — *rebuild* mode (NOT pure defense). Just got wiped /
  answered / mana-starved. Prioritize ramp, tutors, card draw,
  recursion. Targeted removal is fine. Meaningful attacks are fine —
  swing safely for chip damage, don't trade into walls. Actively
  setting up the next engaged phase.
- ``CALL`` — *default* engaged mode. T1-T3 fast plays, aggressive
  counters/removal early. Attack OPEN targets (low blockers, low life,
  vulnerable walkers) — not just highest-threat seat. Matches energy:
  if the table escalates, so do we.
- ``RAISE`` — *press to win* or respond to another seat's RAISE.
  "I can end the game this turn or next." All-in attacks, combos
  fire, commit the board. Also triggered reactively by an opponent's
  ``player_mode_change to=raise`` event — the mutual-RAISE cascade
  the paper-EDH signal ("sudden draw that makes a player perk up")
  produces at the table.

RAISE cascade
~~~~~~~~~~~~~
``observe_event`` now catches ``player_mode_change`` events emitted by
OTHER seats' hats. If another seat transitions to RAISE and the
observing seat has pressurable assets (combo pieces in hand, board
power >= 10, or is in imminent danger), it also transitions to RAISE.
This produces the all-4-seats-in-RAISE closing exchanges the game
design targets.

Default mode
~~~~~~~~~~~~
The hat initializes to ``CALL`` (the paper-EDH default for a seat
"playing normally"). A seat with nothing to do T1 can drop to HOLD
on the first re-evaluate.

Pluggability
~~~~~~~~~~~~
INHERITS from :class:`GreedyHat` so every method we don't override
falls through to the heuristic baseline. Only decisions whose outcome
should change with mode are overridden. The engine never inspects
``self.mode``.
"""

from __future__ import annotations

from enum import Enum
from typing import Any, Optional

from .greedy import GreedyHat


class PlayerMode(str, Enum):
    HOLD = "hold"
    CALL = "call"
    RAISE = "raise"


# =====================================================================
# Tuning constants — exposed so tests / ablations can tweak.
# =====================================================================

# HOLD ↔ CALL hysteresis on self-threat. HOLD→CALL must be strictly
# greater than CALL→HOLD so the mode doesn't chatter on the boundary.
_HOLD_TO_CALL_THRESHOLD = 12
_CALL_TO_HOLD_THRESHOLD = 8

# RAISE trigger thresholds.
_RAISE_LIFE_THRESHOLD = 10        # life <= this → defensive RAISE
_RAISE_BOARD_POWER_FLOOR = 14     # our board power >= this → offensive RAISE
_RAISE_COMBO_MANA_FLOOR = 4
_RAISE_CASCADE_BOARD_POWER = 10   # follow another seat's RAISE if we have this much

# Anti-chatter: a mode change "owes" N observed events before another
# mode change is allowed. Prevents ping-pong on boundary.
_MODE_CHANGE_COOLDOWN_EVENTS = 3

# Dimension weights for the 7-dim threat score. These sum to ~1 across
# a "normal" mid-game seat and make each dimension contributive.
_W_BOARD_POWER = 1.0
_W_GRAVEYARD = 0.8
_W_HAND = 0.6
_W_COMMANDER = 0.7
_W_RAMP = 0.5
_W_LIBRARY_PRESSURE = 0.6
_W_ARCHETYPE = 1.2


# =====================================================================
# Card-effect classification (read oracle-text AST)
# =====================================================================


def _effect_kinds(card) -> set[str]:
    """Return the set of top-level effect ``kind`` strings the card's
    spell-side resolves. Used to bucket cards into ramp/draw/tutor/etc.
    Robust to parser misses — swallows exceptions, returns empty set."""
    try:
        import playloop as _pl  # noqa: WPS433
        effs = _pl.collect_spell_effects(card.ast)
        return {getattr(e, "kind", "") for e in effs if getattr(e, "kind", None)}
    except Exception:
        return set()


def _is_ramp(card) -> bool:
    """Ramp = adds mana, puts a land into play, or cheap mana rock/dork.

    Heuristics (in priority):
      - AddMana effect (includes Sol Ring, Signets, mana dorks).
      - Tutor destination = "battlefield" targeting a land (Cultivate,
        Rampant Growth, Farseek — synthesized as a land-tutor).
      - Creature with a {T}: Add mana activated ability (mana dork).
      - Artifact with cmc <= 3 and "mana" in oracle (mana rock proxy).
    """
    kinds = _effect_kinds(card)
    if "add_mana" in kinds:
        return True
    # Land-ramp tutors: tutor to battlefield where target is a land.
    try:
        import playloop as _pl  # noqa: WPS433
        for eff in _pl.collect_spell_effects(card.ast):
            if getattr(eff, "kind", None) == "tutor":
                dest = getattr(eff, "destination", "")
                q = getattr(eff, "query", None)
                if "battlefield" in dest and q is not None:
                    base = getattr(q, "base", "")
                    if "land" in base:
                        return True
        # Mana-dork creature (tap-for-mana activated ability).
        if "creature" in (card.type_line or "").lower():
            if _pl.get_tap_mana_ability(card.ast) is not None:
                return True
    except Exception:
        pass
    # Cheap artifact with "mana" in oracle_text is a rock (Sol Ring
    # pattern even if AddMana wasn't parsed).
    tl = (card.type_line or "").lower()
    ot = (card.oracle_text or "").lower()
    if "artifact" in tl and (card.cmc or 99) <= 3 and "mana" in ot:
        return True
    return False


def _is_card_draw(card) -> bool:
    return "draw" in _effect_kinds(card)


def _is_tutor(card) -> bool:
    kinds = _effect_kinds(card)
    # Land-tutors that are really ramp get sent to _is_ramp first —
    # but a "Demonic Tutor" hand-tutor is still a tutor here.
    return "tutor" in kinds


def _is_recursion(card) -> bool:
    """Recursion = get a card back from graveyard (reanimate or recurse)."""
    kinds = _effect_kinds(card)
    return bool(kinds & {"reanimate", "recurse"})


def _is_targeted_removal(card) -> bool:
    """Targeted removal = destroy/exile/bounce/damage a single thing.
    (Wraths that target 'each creature' are mass removal, not targeted —
    we filter those out by looking at the Filter's base/scope.)"""
    try:
        import playloop as _pl  # noqa: WPS433
        effs = _pl.collect_spell_effects(card.ast)
    except Exception:
        return False
    targeted_kinds = {"destroy", "exile", "bounce", "damage"}
    for eff in effs:
        if getattr(eff, "kind", None) not in targeted_kinds:
            continue
        tgt = getattr(eff, "target", None)
        if tgt is None:
            continue
        base = getattr(tgt, "base", "")
        # Singular-target filters: creature / player / opponent / any_target /
        # planeswalker. Reject "each" / "all" scopes.
        if base in ("creature", "planeswalker", "any_target",
                    "player", "opponent", "artifact",
                    "enchantment", "nonland_permanent"):
            return True
    return False


def _is_mass_removal(card) -> bool:
    ot = (card.oracle_text or "").lower()
    # Cheap text-side signal — the parser's Filter base doesn't always
    # distinguish "each creature" reliably.
    return (
        "destroy all" in ot
        or "exile all" in ot
        or "each creature" in ot and ("destroy" in ot or "damage" in ot)
        or "wrath" in ot  # cards named wrath-of-god-like
    )


def _is_counterspell(card) -> bool:
    try:
        import playloop as _pl  # noqa: WPS433
        return _pl._card_has_counterspell(card)
    except Exception:
        return False


def _is_cheap_threat(card) -> bool:
    """Cheap creature (CMC ≤ 3) that we can drop to rebuild the board
    during HOLD without overcommitting."""
    tl = (card.type_line or "").lower()
    if "creature" not in tl:
        return False
    return (card.cmc or 99) <= 3


# =====================================================================
# Deck archetype detection
# =====================================================================


class _ArchetypeObservation:
    """Per-seat running tally of observed plays. The PokerHat
    instance attached to one seat keeps a dict of these keyed by the
    OTHER seats, so we can classify each opponent's deck over time.
    """

    __slots__ = (
        "ramp_spent", "counters_cast", "creatures_cast",
        "cards_cast_total", "graveyard_high_water",
        "commander_cast_count",
    )

    def __init__(self) -> None:
        self.ramp_spent: int = 0
        self.counters_cast: int = 0
        self.creatures_cast: int = 0
        self.cards_cast_total: int = 0
        self.graveyard_high_water: int = 0
        self.commander_cast_count: int = 0


# Archetype labels.
ARCHETYPE_UNKNOWN = "unknown"
ARCHETYPE_AGGRO = "aggro"
ARCHETYPE_MIDRANGE = "midrange"
ARCHETYPE_CONTROL = "control"
ARCHETYPE_RAMP = "ramp"
ARCHETYPE_COMBO = "combo"


def _classify_archetype(obs: _ArchetypeObservation, turn: int) -> str:
    """Classify a seat's deck based on cumulative observations.
    Heuristic thresholds mirror the spec:
      - creatures > 40% of cast spells => aggro/midrange (aggro if
        avg creature cmc is low).
      - 6+ counterspells cast => control
      - 8+ mana spent on ramp by T5 => ramp
      - graveyard > 10 by T6 => combo/value
    """
    total = obs.cards_cast_total
    if total < 3:
        return ARCHETYPE_UNKNOWN
    if obs.graveyard_high_water >= 10 and turn <= 6:
        return ARCHETYPE_COMBO
    if obs.counters_cast >= 6:
        return ARCHETYPE_CONTROL
    if obs.ramp_spent >= 8 and turn <= 5:
        return ARCHETYPE_RAMP
    if total > 0 and obs.creatures_cast / total >= 0.4:
        return ARCHETYPE_AGGRO
    return ARCHETYPE_MIDRANGE


def _archetype_bonus(archetype: str, target_seat) -> float:
    """Score bonus added to threat_score for target seats we've
    classified. Combo gets the highest bonus because its gameplan
    is often invisible on the battlefield (Sin / Coram problem)."""
    if archetype == ARCHETYPE_COMBO:
        return 6.0
    if archetype == ARCHETYPE_CONTROL:
        return 3.0
    if archetype == ARCHETYPE_RAMP:
        return 2.5
    if archetype == ARCHETYPE_AGGRO:
        return 2.0
    if archetype == ARCHETYPE_MIDRANGE:
        return 1.5
    return 0.0


# =====================================================================
# Seven-dimensional threat assessment
# =====================================================================


def _count_mana_rocks_and_lands(seat) -> int:
    """Land count + artifact-rock count on battlefield."""
    n = 0
    for p in seat.battlefield:
        tl = (p.card.type_line or "").lower()
        if "land" in tl:
            n += 1
        elif "artifact" in tl and (p.card.cmc or 99) <= 3:
            ot = (p.card.oracle_text or "").lower()
            if "mana" in ot or "{t}" in ot:
                n += 1
    return n


def _board_power(seat) -> float:
    return float(sum(
        max(0, p.power) for p in seat.battlefield
        if getattr(p, "is_creature", False)
    ))


def _high_cmc_permanents(seat) -> int:
    return sum(1 for p in seat.battlefield if (p.card.cmc or 0) >= 5)


def _threat_breakdown(game, source_seat_idx: int, target_seat,
                      archetype_observations: dict) -> dict:
    """Compute the 7-dimensional threat score for ``target_seat`` as
    observed by ``source_seat_idx``. Returns a dict with each dimension
    contribution AND a final ``score`` so debug logs can cite the
    dominant signal.

    ``archetype_observations`` is the PokerHat's per-other-seat
    obs dict (used for archetype classification).
    """
    if target_seat.lost:
        return {"score": float("-inf")}

    # Dimension 1 — board power + high-CMC proxy + low-life leverage.
    bp = _board_power(target_seat)
    hc = _high_cmc_permanents(target_seat)
    dim_board = _W_BOARD_POWER * (bp + hc)
    if target_seat.life <= 10:
        dim_board += 5
    if target_seat.life <= 5:
        dim_board += 5
    # Commander damage leverage (source's commander has already dealt X).
    src = game.seats[source_seat_idx]
    dmg_leverage = 0.0
    for nm in getattr(src, "commander_names", []) or []:
        dmg_leverage += 2 * target_seat.commander_damage.get(nm, 0)
    dim_board += dmg_leverage

    # Dimension 2 — graveyard value. Count cards with recursion
    # relevance: creatures, and cards that themselves are recursion
    # targets. Heuristic: every card in yard contributes 0.3; bonus
    # for creatures and high-cmc cards.
    gy = getattr(target_seat, "graveyard", []) or []
    gy_raw = len(gy)
    gy_bonus = 0.0
    for c in gy:
        tl = (c.type_line or "").lower()
        if "creature" in tl:
            gy_bonus += 0.5
            if (c.power or 0) >= 3:
                gy_bonus += 0.3
        if (c.cmc or 0) >= 4:
            gy_bonus += 0.2
    dim_graveyard = _W_GRAVEYARD * (0.3 * gy_raw + gy_bonus)

    # Dimension 3 — hand size. 5+ is scary (draw engine active).
    hand_n = len(getattr(target_seat, "hand", []) or [])
    if hand_n >= 5:
        dim_hand = _W_HAND * (hand_n - 4) * 1.5
    elif hand_n >= 3:
        dim_hand = _W_HAND * 1.0
    else:
        dim_hand = 0.0

    # Dimension 4 — command zone: uncast commander = deferred threat.
    # Also weigh commander tax paid (cast count) as commitment signal.
    cmd_zone = getattr(target_seat, "command_zone", []) or []
    cmd_cast_count = sum(getattr(target_seat, "commander_tax", {}).values())
    dim_commander = _W_COMMANDER * (
        2.0 * len(cmd_zone)             # still in zone → looming threat
        + 1.0 * cmd_cast_count          # more recasts = committed
    )

    # Dimension 5 — ramp / mana density. Land count is visible.
    # 5+ lands by itself isn't threatening, but add mana rocks and
    # it's a ramp-deck tell.
    rocks_lands = _count_mana_rocks_and_lands(target_seat)
    dim_ramp = _W_RAMP * max(0, rocks_lands - 3) * 0.7

    # Dimension 6 — library pressure / deck-out looming.
    # Small library = either self-milled combo or long game nearly over.
    lib_n = len(getattr(target_seat, "library", []) or [])
    if lib_n < 20:
        dim_lib = _W_LIBRARY_PRESSURE * (20 - lib_n) * 0.25
    elif lib_n < 40:
        dim_lib = _W_LIBRARY_PRESSURE * 0.5
    else:
        dim_lib = 0.0

    # Dimension 7 — archetype bonus. Uses observed cast history.
    archetype = ARCHETYPE_UNKNOWN
    obs = archetype_observations.get(target_seat.idx)
    if obs is not None:
        archetype = _classify_archetype(obs, game.turn)
    dim_arch = _W_ARCHETYPE * _archetype_bonus(archetype, target_seat)

    score = (dim_board + dim_graveyard + dim_hand + dim_commander
             + dim_ramp + dim_lib + dim_arch)

    return {
        "score": score,
        "board": dim_board,
        "graveyard": dim_graveyard,
        "hand": dim_hand,
        "commander": dim_commander,
        "ramp": dim_ramp,
        "library": dim_lib,
        "archetype": dim_arch,
        "archetype_label": archetype,
    }


# =====================================================================
# Open-target attack selection
# =====================================================================


def _is_open_for_attacker(attacker, defender_seat) -> bool:
    """Can ``attacker`` swing at ``defender_seat`` mostly unblocked?

    An opp is "open" for this attacker if the defender's pool of
    legal untapped blockers is thin:
      - Flying attacker: defender has no flying/reach blocker.
      - Menace attacker: defender has <= 1 untapped creature.
      - Unblockable attacker (protection / shadow / etc., approximated
        by keyword checks): always open.
      - Otherwise: no untapped creature with power >= attacker.power.
    """
    try:
        import playloop as _pl  # noqa: WPS433
    except Exception:
        return False

    if _pl.kw(attacker, "unblockable"):
        return True
    # Shadow / Fear / Intimidate are rare in current cardpool; use
    # keyword proxy. Skip if unknown.

    untapped_creatures = [
        b for b in defender_seat.battlefield
        if getattr(b, "is_creature", False) and not b.tapped
    ]
    if not untapped_creatures:
        return True

    if _pl.kw(attacker, "flying"):
        # Can only be blocked by flying or reach.
        flyblockers = [
            b for b in untapped_creatures
            if _pl.kw(b, "flying") or _pl.kw(b, "reach")
        ]
        if not flyblockers:
            return True
        return False

    if _pl.kw(attacker, "menace"):
        if len(untapped_creatures) <= 1:
            return True

    # Vanilla: open if no blocker can profitably kill us.
    ap = max(0, getattr(attacker, "power", 0))
    threatening = [b for b in untapped_creatures
                   if getattr(b, "power", 0) >= ap]
    if not threatening:
        return True
    return False


def _pick_attacker_target(game, seat, attacker,
                           archetype_observations) -> int:
    """Per-attacker target selection. Preference order:
        1. open target (can swing through mostly unblocked)
        2. lowest-life opp
        3. highest-threat opp (via 7-dim score)
        4. first living opp
    Returns a seat index.
    """
    living = [s for s in game.seats
              if not s.lost and s.idx != seat.idx]
    if not living:
        return seat.idx
    if len(living) == 1:
        return living[0].idx

    # Stage 1: open targets.
    open_opps = [o for o in living if _is_open_for_attacker(attacker, o)]
    if open_opps:
        # Tie-break by lowest life among open opps.
        open_opps.sort(key=lambda o: o.life)
        return open_opps[0].idx

    # Stage 2: lowest-life opp.
    if any(o.life <= 15 for o in living):
        by_life = sorted(living, key=lambda o: o.life)
        return by_life[0].idx

    # Stage 3: highest-threat opp.
    ranked = sorted(
        living,
        key=lambda o: -_threat_breakdown(
            game, seat.idx, o, archetype_observations)["score"],
    )
    return ranked[0].idx


# =====================================================================
# Self-threat (for CALL↔HOLD hysteresis)
# =====================================================================


def _self_threat_score(game, seat, archetype_observations) -> float:
    """Score our own board/hand density — "how scary do we look?"
    Used for HOLD↔CALL transitions. Mirrors the 7-dim threat but
    pointed inward. We want a single scalar here, not a breakdown.
    """
    # Re-use the threat breakdown in third-person — compute as if we
    # were looking at ourselves as a target. Treat "source" as the
    # next living opp (any — the result is symmetric for our purposes).
    other = None
    for s in game.seats:
        if s.idx != seat.idx and not s.lost:
            other = s
            break
    if other is None:
        return 0.0
    bd = _threat_breakdown(game, other.idx, seat, archetype_observations)
    return bd.get("score", 0.0)


# =====================================================================
# RAISE trigger detection
# =====================================================================


def _available_mana_estimate(seat) -> int:
    try:
        import playloop as _pl  # noqa: WPS433
        return _pl._available_mana(seat)
    except Exception:
        pool = getattr(seat, "mana_pool", 0)
        lands = sum(
            1 for p in seat.battlefield
            if getattr(p, "is_land", False) and not p.tapped
        )
        return pool + lands


def _combo_pieces_in_hand(seat) -> int:
    """Count of cards in hand that look like "combo pieces" — tutors,
    card draw engines, recursion, win conditions, or high-cmc haymakers.
    Rough — this is a gate, not a solver.
    """
    n = 0
    for c in seat.hand:
        if _is_tutor(c):
            n += 1
        elif _is_card_draw(c) and (c.cmc or 0) >= 3:
            n += 1
        elif _is_recursion(c):
            n += 1
        elif "win the game" in (c.oracle_text or "").lower():
            n += 2
        elif (c.cmc or 0) >= 6:
            n += 1
    return n


def _combo_ready(game, seat) -> bool:
    """True iff we look like we can pop off: enough mana plus at least
    one castable payoff / combo piece in hand."""
    available = _available_mana_estimate(seat)
    if available < _RAISE_COMBO_MANA_FLOOR:
        return False
    if _combo_pieces_in_hand(seat) == 0:
        return False
    return True


def _opponent_one_turn_from_win(game, seat) -> bool:
    """Is an opp positioned to lethal us next turn?"""
    try:
        for opp in game.seats:
            if opp.lost or opp.idx == seat.idx:
                continue
            untapped_power = sum(
                max(0, p.power) for p in opp.battlefield
                if getattr(p, "is_creature", False) and not p.tapped
            )
            if untapped_power >= seat.life:
                return True
            for nm in getattr(opp, "commander_names", []) or []:
                if seat.commander_damage.get(nm, 0) >= 15:
                    return True
    except Exception:
        return False
    return False


# =====================================================================
# PokerHat
# =====================================================================


class PokerHat(GreedyHat):
    """Adaptive HOLD/CALL/RAISE hat. Subclasses GreedyHat so
    anything we don't override falls through to the heuristic baseline
    — the "poker" signal only shows up where mode should change outcomes
    (casts, attacks, targeting, combat, stack response).
    """

    def __init__(self, initial_mode: PlayerMode = PlayerMode.CALL) -> None:
        self.mode: PlayerMode = initial_mode
        self.last_mode_change_seq: int = 0
        self._events_seen: int = 0
        # Per-OTHER-seat observation tally. Keyed by seat.idx. We
        # initialize lazily on first observed event from that seat.
        self._obs: dict = {}
        # Snapshot of the last breakdown we computed for our own
        # self-threat; used for trace/debug emission.
        self._last_self_threat_breakdown: dict = {}

    # ------------------------------------------------------------------
    # Internal: mode transitions
    # ------------------------------------------------------------------

    def _transition(self, game, seat, new_mode: PlayerMode,
                    reason: str) -> None:
        if new_mode == self.mode:
            return
        if (self._events_seen - self.last_mode_change_seq
                < _MODE_CHANGE_COOLDOWN_EVENTS):
            return
        old = self.mode
        self.mode = new_mode
        self.last_mode_change_seq = self._events_seen
        try:
            game.ev(
                "player_mode_change",
                seat_idx=seat.idx,
                from_mode=old.value,
                to_mode=new_mode.value,
                reason=reason,
            )
        except Exception:
            pass

    def _re_evaluate(self, game, seat) -> None:
        """Decide whether the current mode should change. Priority:

            1. Emergency RAISE  (low life, opp one turn from lethal)
            2. Offensive RAISE  (big board + combo-ready hand)
            3. CALL ↔ HOLD      (hysteresis on self-threat)
            4. RAISE decay      (stabilized: drop back to CALL)
        """
        # -- 1. Emergency RAISE --------------------------------------
        if (seat.life <= _RAISE_LIFE_THRESHOLD
                and self.mode != PlayerMode.RAISE):
            self._transition(game, seat, PlayerMode.RAISE,
                             f"life={seat.life}<={_RAISE_LIFE_THRESHOLD}")
            return
        if _opponent_one_turn_from_win(game, seat):
            self._transition(game, seat, PlayerMode.RAISE,
                             "opp_one_turn_from_win")
            return

        # -- 2. Offensive RAISE --------------------------------------
        our_board = _board_power(seat)
        if (our_board >= _RAISE_BOARD_POWER_FLOOR
                and _combo_ready(game, seat)
                and self.mode != PlayerMode.RAISE):
            self._transition(
                game, seat, PlayerMode.RAISE,
                f"offensive: board={our_board:.0f} + combo_ready",
            )
            return

        # -- 3. CALL ↔ HOLD hysteresis --------------------------------
        score = _self_threat_score(game, seat, self._obs)
        if self.mode == PlayerMode.HOLD and score >= _HOLD_TO_CALL_THRESHOLD:
            self._transition(
                game, seat, PlayerMode.CALL,
                f"self_threat={score:.1f}>={_HOLD_TO_CALL_THRESHOLD}",
            )
            return
        if self.mode == PlayerMode.CALL and score <= _CALL_TO_HOLD_THRESHOLD:
            self._transition(
                game, seat, PlayerMode.HOLD,
                f"self_threat={score:.1f}<={_CALL_TO_HOLD_THRESHOLD}",
            )
            return

        # -- 4. RAISE decay ------------------------------------------
        if (self.mode == PlayerMode.RAISE
                and seat.life > _RAISE_LIFE_THRESHOLD
                and not _opponent_one_turn_from_win(game, seat)):
            # Stabilized; drop back to CALL (not HOLD — we still have
            # board most likely).
            self._transition(game, seat, PlayerMode.CALL, "raise_cooldown")
            return

    # ------------------------------------------------------------------
    # Internal: observation accounting (archetype detection)
    # ------------------------------------------------------------------

    def _obs_for(self, seat_idx: int) -> _ArchetypeObservation:
        obs = self._obs.get(seat_idx)
        if obs is None:
            obs = _ArchetypeObservation()
            self._obs[seat_idx] = obs
        return obs

    def _tally_cast(self, game, acting_seat_idx: int, event) -> None:
        """A ``cast`` event fired. Update archetype tally for the
        acting seat. Does NOT tally our own casts — archetype detection
        is for OTHER seats only.
        """
        if acting_seat_idx is None:
            return
        obs = self._obs_for(acting_seat_idx)
        obs.cards_cast_total += 1
        card_name = event.get("card")
        cmc = event.get("cmc", 0) or 0
        # Look up the card on the acting seat's battlefield / graveyard
        # to re-parse its AST. Best effort — card objects are mutable.
        card = None
        try:
            for pool in (
                game.seats[acting_seat_idx].graveyard,
                game.seats[acting_seat_idx].battlefield,
            ):
                for entry in pool:
                    candidate = entry.card if hasattr(entry, "card") else entry
                    if getattr(candidate, "name", None) == card_name:
                        card = candidate
                        break
                if card is not None:
                    break
        except Exception:
            card = None
        if card is None:
            # Can't classify — bump generic count only.
            return
        if _is_ramp(card):
            obs.ramp_spent += cmc
        if _is_counterspell(card):
            obs.counters_cast += 1
        if "creature" in (card.type_line or "").lower():
            obs.creatures_cast += 1

    def _tally_graveyard(self, game, seat_idx: int) -> None:
        try:
            gy = len(game.seats[seat_idx].graveyard or [])
            obs = self._obs_for(seat_idx)
            if gy > obs.graveyard_high_water:
                obs.graveyard_high_water = gy
        except Exception:
            pass

    # ------------------------------------------------------------------
    # Event observer — the only state-update hook
    # ------------------------------------------------------------------

    def observe_event(self, game, seat, event) -> None:
        self._events_seen += 1
        if not isinstance(event, dict):
            return
        etype = event.get("type")
        if etype is None:
            return

        # --- RAISE CASCADE -----------------------------------------
        # If ANOTHER seat transitioned to RAISE, consider matching.
        if etype == "player_mode_change":
            other_idx = event.get("seat_idx")
            to_mode = event.get("to_mode")
            if (other_idx is not None
                    and other_idx != seat.idx
                    and to_mode == PlayerMode.RAISE.value):
                self._consider_cascade_raise(game, seat)
                return   # cascade already handled re-eval

        # --- Archetype accounting (for OTHER seats only) ------------
        if etype == "cast":
            acting = event.get("seat")
            if acting is not None and acting != seat.idx:
                self._tally_cast(game, acting, event)
            # Also refresh graveyard high-water for every seat on cast.
            for s in game.seats:
                if s.idx != seat.idx:
                    self._tally_graveyard(game, s.idx)

        # Trigger re-evaluate on relevant events.
        trigger_types = {
            "game_start",
            "turn_start",
            "draw",
            "damage",
            "life_change",
            "cast",
            "attackers",
            "blockers",
            "sba_704_5a",
            "seat_eliminated",
            "combat_damage",
        }
        if etype in trigger_types:
            self._re_evaluate(game, seat)

    def _consider_cascade_raise(self, game, seat) -> None:
        """Another seat just RAISEd. Should we match?

        Thresholds (from spec):
            - 2+ combo pieces in hand    → yes
            - board power >= 10          → yes
            - facing imminent loss       → yes
            - otherwise stay in current mode (don't chase an empty bluff).
        """
        if self.mode == PlayerMode.RAISE:
            return  # already raising
        combo_count = _combo_pieces_in_hand(seat)
        board_power = _board_power(seat)
        imminent = _opponent_one_turn_from_win(game, seat) or \
            seat.life <= _RAISE_LIFE_THRESHOLD
        if (combo_count >= 2
                or board_power >= _RAISE_CASCADE_BOARD_POWER
                or imminent):
            reason = (
                f"cascade: combo={combo_count} board={board_power:.0f} "
                f"imminent={imminent}"
            )
            self._transition(game, seat, PlayerMode.RAISE, reason)

    # ------------------------------------------------------------------
    # Overridden decisions (mode-sensitive)
    # ------------------------------------------------------------------

    def choose_cast_from_hand(self, game, seat, castable_cards):
        """Mode-sensitive cast priority.

        HOLD (rebuild mode) priority order:
            1. Tutors
            2. Card draw
            3. Recursion
            4. Ramp
            5. Targeted removal IF a threat merits it
            6. Cheap threats (CMC <= 3) to rebuild board
            7. Skip expensive haymakers

        CALL / RAISE: defer to greedy (biggest affordable non-counter)
        so the combo pieces actually resolve on the turn we decided to
        pop off.
        """
        import playloop as _pl  # noqa: WPS433

        # Baseline filter: greedy's own "skip counters, we save them"
        # check. We keep counters in hand for respond_to_stack_item.
        hand_playable = [
            c for c in castable_cards
            if not _pl._card_has_counterspell(c)
        ]
        if not hand_playable:
            return None

        if self.mode == PlayerMode.HOLD:
            return self._choose_cast_hold(game, seat, hand_playable)
        # CALL / RAISE: greedy's biggest-affordable-first is fine.
        return super().choose_cast_from_hand(game, seat, castable_cards)

    def _choose_cast_hold(self, game, seat, cards):
        """HOLD-mode cast priority. Return first card from the highest-
        priority bucket that has any candidates, else None.
        """
        # Bucket every castable in priority order.
        buckets: dict[int, list] = {i: [] for i in range(7)}
        for c in cards:
            if _is_tutor(c):
                buckets[0].append(c)
            elif _is_card_draw(c):
                buckets[1].append(c)
            elif _is_recursion(c):
                buckets[2].append(c)
            elif _is_ramp(c):
                buckets[3].append(c)
            elif _is_targeted_removal(c):
                buckets[4].append(c)
            elif _is_cheap_threat(c):
                buckets[5].append(c)
            else:
                # Expensive haymakers / mass removal — skip during HOLD.
                buckets[6].append(c)

        # Removal needs a worthy target. If we can't name a threat
        # score >= 10, drop removal back to bucket 6 (skip).
        if buckets[4]:
            has_big_threat = False
            for s in game.seats:
                if s.lost or s.idx == seat.idx:
                    continue
                bd = _threat_breakdown(game, seat.idx, s, self._obs)
                if bd["score"] >= 10:
                    has_big_threat = True
                    break
            if not has_big_threat:
                # No worthy target; defer until CALL.
                buckets[4] = []

        for i in range(6):  # skip bucket 6 (haymakers)
            pool = buckets[i]
            if not pool:
                continue
            # Within a bucket, cheaper first (rebuild efficiently).
            pool.sort(key=lambda c: (c.cmc or 0, c.name))
            return pool[0]
        return None

    def declare_attackers(self, game, seat, legal_attackers):
        """Mode-sensitive attack declaration.

        HOLD: swing only with creatures that can hit safely — evasion
              (flying/menace/unblockable) OR the attacker has an open
              target where we can swing past all blockers. Chip damage
              is welcome; trades are not.

        CALL: attack with deadliest-first 70% of our attackers, but
              only if they have open targets or a 2-for-1 block isn't
              imminent. Preserves some blocking capacity.

        RAISE: all-in.
        """
        import playloop as _pl  # noqa: WPS433

        if not legal_attackers:
            return []

        if self.mode == PlayerMode.RAISE:
            return list(legal_attackers)

        # HOLD: only safe attackers.
        if self.mode == PlayerMode.HOLD:
            safe: list = []
            for a in legal_attackers:
                has_evasion = (
                    _pl.kw(a, "flying")
                    or _pl.kw(a, "menace")
                    or _pl.kw(a, "unblockable")
                    or _pl.kw(a, "shadow")
                    or _pl.kw(a, "skulk")
                )
                # Or: an open target exists for this attacker.
                open_exists = any(
                    _is_open_for_attacker(a, o)
                    for o in game.seats
                    if not o.lost and o.idx != seat.idx
                )
                if has_evasion or open_exists:
                    safe.append(a)
            return safe

        # CALL: send deadliest-first 70% that have open targets OR no
        # profitable trade. Reserve some blocking capacity.
        def _rank(a):
            dt = 1 if _pl.kw(a, "deathtouch") else 0
            ds = 1 if _pl.kw(a, "double strike") else 0
            return -(a.power + dt * 5 + ds * 3)
        ranked = sorted(legal_attackers, key=_rank)
        # Filter for a target worth swinging at (open or lowest-life).
        def _has_worthwhile_target(a) -> bool:
            for o in game.seats:
                if o.lost or o.idx == seat.idx:
                    continue
                if _is_open_for_attacker(a, o):
                    return True
                if o.life <= 15:
                    return True
            return False
        ranked = [a for a in ranked if _has_worthwhile_target(a)]
        if not ranked:
            return []
        keep = max(1, (len(ranked) * 7 + 9) // 10)  # ceil(n * 0.7)
        return ranked[:keep]

    def declare_attack_target(self, game, seat, attacker,
                               legal_defenders) -> int:
        """Per-attacker target selection with OPEN-TARGET preference.
        This method exists on the Protocol; when the engine starts
        routing multi-defender damage through it, the 7-dim threat
        + open-target algorithm takes effect automatically.
        """
        if not legal_defenders:
            return seat.idx
        if len(legal_defenders) == 1:
            return legal_defenders[0]

        # Filter to legal defender seats (they're idx or Seat?).
        def _idx(x) -> int:
            return x if isinstance(x, int) else getattr(x, "idx", 0)

        idxs = [_idx(d) for d in legal_defenders]

        # Map idx → Seat for convenience.
        seats_by_idx = {s.idx: s for s in game.seats}
        living = [seats_by_idx[i] for i in idxs
                  if i in seats_by_idx and not seats_by_idx[i].lost
                  and i != seat.idx]
        if not living:
            return seat.idx
        # Prefer open target.
        for o in living:
            if _is_open_for_attacker(attacker, o):
                return o.idx
        # Then lowest-life.
        if any(o.life <= 15 for o in living):
            return min(living, key=lambda o: o.life).idx
        # Then highest 7-dim threat.
        ranked = sorted(
            living,
            key=lambda o: -_threat_breakdown(
                game, seat.idx, o, self._obs)["score"],
        )
        return ranked[0].idx

    def declare_blockers(self, game, seat, attackers) -> dict:
        """RAISE: decline to block unless lethal is on the stack.
        HOLD / CALL: defer to greedy's deadliest-first heuristic.
        """
        if self.mode == PlayerMode.RAISE:
            import playloop as _pl  # noqa: WPS433
            incoming = sum(
                a.power * (2 if _pl.kw(a, "double strike") else 1)
                for a in attackers
            )
            if incoming < seat.life:
                return {id(a): [] for a in attackers}
        return super().declare_blockers(game, seat, attackers)

    def respond_to_stack_item(self, game, seat, stack_item):
        import playloop as _pl  # noqa: WPS433

        if stack_item.controller == seat.idx:
            return None
        if stack_item.countered:
            return None
        if _pl._split_second_active(game):
            return None
        if _pl._opp_restricts_defender_to_sorcery_speed(game, seat.idx):
            return None

        # Mode-specific threshold.
        if self.mode == PlayerMode.RAISE:
            # Pressing to win — save mana for our plays. Only counter a
            # win-game trigger or a board wipe.
            score = _pl._stack_item_threat_score(stack_item)
            if score < 8:
                return None
            return _pl._find_counter_in_hand(game, seat)

        if self.mode == PlayerMode.HOLD:
            # Rebuild mode: counter things aimed at us more
            # aggressively than CALL (we can't afford to lose our
            # fragile comeback).
            if _pl._stack_item_threat_score(stack_item) < 3:
                return None
            return _pl._find_counter_in_hand(game, seat)

        # CALL — aggressive early, counter biggest threats.
        if _pl._stack_item_threat_score(stack_item) < 4:
            return None
        return _pl._find_counter_in_hand(game, seat)

    def choose_target(self, game, seat, filter_spec, legal_targets):
        """Targeting. HOLD prefers self-targets for buffs/draw; CALL
        and RAISE want to hit the highest 7-dim-threat OPEN opp.
        """
        if self.mode == PlayerMode.HOLD and filter_spec.base in (
                "any_target", "player"):
            return "player", game.seats[seat.idx]

        # CALL / RAISE: route to our 7-dim threat ranker for player
        # targets (the greedy baseline uses flat threat_score via
        # _pick_opponent_by_threat; we want the widened version).
        if filter_spec.base in ("player", "opponent", "any_target"):
            best = self._pick_best_player_target(game, seat)
            if best is not None:
                return "player", best
        return super().choose_target(game, seat, filter_spec, legal_targets)

    def _pick_best_player_target(self, game, seat):
        """Pick the highest 7-dim threat living opponent."""
        living = [s for s in game.seats
                  if not s.lost and s.idx != seat.idx]
        if not living:
            return None
        ranked = sorted(
            living,
            key=lambda s: -_threat_breakdown(
                game, seat.idx, s, self._obs)["score"],
        )
        return ranked[0]

    # ------------------------------------------------------------------
    # Introspection — not used by engine, but useful for tests/debug
    # ------------------------------------------------------------------

    def threat_breakdown_for(self, game, seat, target) -> dict:
        """Exposed for debug/logging consumers. Returns the same dict
        the internal scorer produces so tests / the rule auditor can
        inspect dimension contributions.
        """
        return _threat_breakdown(game, seat.idx, target, self._obs)

    def __repr__(self) -> str:
        return f"PokerHat(mode={self.mode.value})"


# Backward-compat alias — the old name still works so external code
# (and anything we missed in the rename) keeps functioning. Prefer
# ``PokerHat`` for new code.
PokerPolicy = PokerHat
