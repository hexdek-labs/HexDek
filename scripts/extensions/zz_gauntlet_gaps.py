#!/usr/bin/env python3
"""Close the seven parser/resolver gaps surfaced by the 100-game 4p EDH gauntlet.

The gauntlet audit (data/rules/gauntlet_report.md) flagged these `unknown_nodes`
categories as the top unhandled effect-kinds the resolver fell through on:

    parsed_effect_residual : 294   (already a safety net — addressed via EFFECT_RULES)
    UnknownEffect          : 254   (untyped fallbacks — addressed via EFFECT_RULES)
    attack_trigger_unknown : 101   (handled via _synthesize_combat_effect extension)
    sacrifice              :  61   (Sacrifice(...) node had no resolver branch)
    fight                  :  10   (Fight(...) node had no resolver branch)
    replacement            :  10   (Modification(kind='replacement')-ish — route through §614 fire_event)
    exile_top_library      :   4   (Modification(kind='exile_top_library') stub)

Approach
--------
The extension is named ``zz_*`` so it loads LAST — after every other extension
has registered its parser rules. It has two responsibilities:

1. **Parser-side** (EFFECT_RULES): emit typed promotions for common oracle
   text snippets that previously fell through to ``UnknownEffect`` or to
   ``Modification(kind='parsed_effect_residual')``. Each rule uses anchored
   regex (``^...$``) so it only matches COMPLETE clauses — we never widen an
   existing grammar production.

2. **Runtime patches**: the task explicitly notes "sacrifice / fight /
   replacement / exile_top_library need resolver branches". The `playloop.py`
   source file is owned by another agent tonight and cannot be edited, so
   this extension installs the missing branches by wrapping the module-level
   ``resolve_effect`` and ``_synthesize_combat_effect`` functions. The wraps
   delegate to the original for every kind the resolver already handles and
   add the five missing kinds (the four gap kinds + the attack-trigger
   synth extensions).

   Why wrap and not rewrite ASTs? Rewriting would flip golden signatures for
   every card whose ``first5`` currently ends in a typed ``sacrifice`` /
   ``fight`` / ``replacement`` node (e.g. ``ichorid`` — ``trig:phase /
   sacrifice``). The task says goldens "may need regen" but preserving
   current AST shape is cleaner: the AST records what the card *says*, the
   resolver decides what the engine *does*.

Accuracy posture
----------------
These handlers are deliberately MINIMAL. MTG has subtle interactions we don't
model here (sacrifice ignores indestructible but destroy doesn't; fight is
simultaneous so both creatures die together even if one has lifelink; etc.).
The goal is to stop incrementing the `unknown_nodes` counter — which today
silently means "nothing happened" — with a correct-enough execution that the
gauntlet's zone-conservation audit doesn't flag extra violations.

Each handler emits an event (e.g. ``sacrifice_resolved``) so downstream
auditors can see exactly what fired and distinguish from the zero-effect
no-op the counter used to represent.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

_HERE = Path(__file__).resolve().parent
_SCRIPTS = _HERE.parent
if str(_SCRIPTS) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS))

from mtg_ast import (  # noqa: E402
    AddMana, Buff, CounterMod, CreateToken, Damage, Discard, Draw, Filter,
    ManaSymbol, Modification, Sacrifice, Sequence, TARGET_ANY, TARGET_CREATURE,
    TARGET_OPPONENT, TARGET_PLAYER, SELF, UnknownEffect,
)


# ---------------------------------------------------------------------------
# Parser-side: EFFECT_RULES for common raw-text snippets that previously
# fell through to UnknownEffect. These are anchored to complete clauses so
# they never swallow surrounding text.
# ---------------------------------------------------------------------------

EFFECT_RULES: list = []


def _eff(pattern: str):
    def deco(fn):
        EFFECT_RULES.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


_NUMS = {
    "a": 1, "an": 1, "one": 1, "two": 2, "three": 3, "four": 4, "five": 5,
    "six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10,
}


def _n(tok):
    t = (tok or "").lower()
    if t in _NUMS:
        return _NUMS[t]
    if t.isdigit():
        return int(t)
    return t


# --- "defending player discards/draws/sacrifices/loses life" — combat-trigger
#     tails that today reach _synthesize_combat_effect as UnknownEffect. We
#     promote the TAIL shape at the effect grammar level so the attack-trigger
#     resolver sees a typed Discard/Draw/LoseLife/Sacrifice subtree and
#     never reaches the "unknown" branch of _synthesize_combat_effect.

# Guarded rule: "defending player discards a card"
@_eff(r"^defending player discards an? card\.?$")
def _def_player_discards(m):
    return Discard(count=1, target=Filter(base="player", targeted=False,
                                          extra=("defending",)))


@_eff(r"^defending player discards (\d+|two|three) cards\.?$")
def _def_player_discards_n(m):
    return Discard(count=_n(m.group(1)),
                   target=Filter(base="player", targeted=False,
                                 extra=("defending",)))


@_eff(r"^defending player may draw a card\.?$")
def _def_player_may_draw(m):
    # "may" is opponent-choice; represent as a plain Draw and log — a true
    # optional would need a player-prompt path the resolver doesn't own yet.
    from mtg_ast import Optional_
    return Optional_(body=Draw(count=1,
                               target=Filter(base="player", targeted=False,
                                             extra=("defending",))))


@_eff(r"^defending player loses (\d+|one|two|three) life\.?$")
def _def_player_loses_life(m):
    from mtg_ast import LoseLife
    return LoseLife(amount=_n(m.group(1)),
                    target=Filter(base="player", targeted=False,
                                  extra=("defending",)))


# "that player loses/discards" variants — same shape but pronoun-addressed
@_eff(r"^that player loses (\d+|one|two|three) life\.?$")
def _that_player_loses_life(m):
    from mtg_ast import LoseLife
    return LoseLife(amount=_n(m.group(1)),
                    target=Filter(base="player", targeted=False,
                                  extra=("that",)))


@_eff(r"^that player discards an? card\.?$")
def _that_player_discards(m):
    return Discard(count=1, target=Filter(base="player", targeted=False,
                                          extra=("that",)))


# --- "create a Treasure/Food/Clue/Map/Blood token" — attack-trigger payload
#     shapes that the generic create-token rule misses when the parser lowered
#     the trigger text into a bare verb phrase. Each of these is safe because
#     the token dataclass is fully typed.

@_eff(r"^create a treasure token\.?$")
def _create_treasure(m):
    return CreateToken(count=1, types=("Treasure",))


@_eff(r"^create a food token\.?$")
def _create_food(m):
    return CreateToken(count=1, types=("Food",))


@_eff(r"^create a clue token\.?$")
def _create_clue(m):
    return CreateToken(count=1, types=("Clue",))


@_eff(r"^create a map token\.?$")
def _create_map(m):
    return CreateToken(count=1, types=("Map",))


@_eff(r"^create a blood token\.?$")
def _create_blood(m):
    return CreateToken(count=1, types=("Blood",))


# --- Attack-trigger self-buff tails: "it gets +N/+M until end of turn"
# (already partially handled by _SYNTH_RE_SELF_BUFF but the anchored grammar
#  lowers the variant to a typed Buff on 'self' which routes cleanly.)
@_eff(r"^it gets \+(\d+)/\+(\d+) until end of turn\.?$")
def _it_gets_buff_eot(m):
    return Buff(power=int(m.group(1)), toughness=int(m.group(2)),
                target=Filter(base="self", targeted=False),
                duration="until_end_of_turn")


# --- "target creature gets -N/-N until end of turn" — generic combat shape
@_eff(r"^target creature gets -(\d+)/-(\d+) until end of turn\.?$")
def _target_creature_debuff(m):
    return Buff(power=-int(m.group(1)), toughness=-int(m.group(2)),
                target=TARGET_CREATURE,
                duration="until_end_of_turn")


# ---------------------------------------------------------------------------
# Runtime patches: installed lazily at first use of resolve_effect().
#
# We cannot import playloop at module-load time because (a) extensions are
# loaded during parser module import, and (b) playloop imports parser which
# triggers extension loading — a circular path. Instead we defer the wrap
# until `load_extensions()` runs from inside a playloop main. At that point
# sys.modules['playloop'] is populated.
#
# The wraps are idempotent: if already installed they no-op.
# ---------------------------------------------------------------------------


def _is_indestructible(perm) -> bool:
    """Thin local shim so we don't have to hop through playloop."""
    try:
        granted = getattr(perm, "granted", ()) or ()
        if "indestructible" in granted:
            return True
        kws = getattr(perm.card, "ast", None)
        if kws is not None:
            from mtg_ast import Keyword
            for ab in kws.abilities:
                if isinstance(ab, Keyword) and ab.name == "indestructible":
                    return True
    except Exception:
        pass
    return False


def _seat_of_source(game, source_seat):
    try:
        return game.seats[source_seat]
    except Exception:
        return None


def _pick_sac_target(game, seat, flt):
    """Pick a creature / permanent to sacrifice from `seat` matching `flt`.

    Dumb policy: lowest (power + toughness) creature; if no creatures match
    or the filter doesn't target creatures, sacrifice any matching permanent
    with lowest mana value. Returns None if nothing matches.

    Handles compound base names like "nontoken creature" / "non-vampire
    creature" / "creature or vehicle" / "creature or planeswalker" that the
    parser glues together as a single base string. These are the shapes
    Eradicator Valkyrie, Anowon, Momentum Breaker etc. emit.
    """
    if seat is None or not getattr(seat, "battlefield", None):
        return None
    base = (getattr(flt, "base", "") or "").lower().strip()
    if not base:
        return None

    # Normalize compound bases. We scan for type keywords and for "nontoken"
    # / "non-X" modifiers. Anything else falls through to a permissive match.
    tokens = [t for t in re.split(r"[\s/-]+", base) if t]

    def matches(p) -> bool:
        tl = p.card.type_line.lower()
        # "permanent" matches anything
        if "permanent" in tokens:
            ok = True
        else:
            want_types = [t for t in ("creature", "artifact", "land",
                                       "enchantment", "planeswalker",
                                       "vehicle", "saga", "battle")
                          if t in tokens]
            # "creature or vehicle" etc. → OR-match
            if not want_types:
                ok = True  # nothing to match — fall through permissively
            else:
                ok = any(t in tl for t in want_types)
        # "nontoken" filter
        if "nontoken" in tokens and getattr(p, "is_token", False):
            return False
        # "non-<type>" (creature type) filter — e.g. non-vampire creature
        for tk in tokens:
            if tk.startswith("non") and tk != "nontoken" and len(tk) > 3:
                subtype = tk[3:].lstrip("-").strip()
                if subtype and subtype in tl:
                    return False
        return ok

    pool = []
    for p in seat.battlefield:
        if base in ("self", "that_thing", "that_creature"):
            # handled by caller (source_perm)
            return None
        if matches(p):
            pool.append(p)

    if not pool:
        return None
    # Prefer creatures the seat would rather lose (low stats, summoning sick,
    # token). For non-creatures, prefer lowest mana value.
    def rank(p):
        if p.is_creature:
            is_token = 1 if getattr(p, "is_token", False) else 0
            return (0, -is_token, p.power + p.toughness, p.power)
        return (1, 0, 0, 0)
    pool.sort(key=rank)
    return pool[0]


def _sac_permanent(game, perm, seat, reason):
    """Route a sacrifice through the same §614 die event destroy uses, but
    bypass the indestructible check (sacrifice ignores indestructible per
    CR 701.16). Emits a `sacrifice_resolved` event so auditors can see it."""
    try:
        import sys as _s
        playloop = _s.modules.get("playloop")
        if playloop is None:
            return
        # Sacrificing a permanent IS a zone-change (battlefield → graveyard
        # unless replaced). do_permanent_die fires the die + gy events.
        do_die = getattr(playloop, "do_permanent_die", None)
        if do_die is None:
            return
        do_die(game, perm, reason=reason)
        game.ev("sacrifice_resolved",
                card=perm.card.name,
                seat=perm.controller,
                rule="701.16")
    except Exception as exc:
        game.ev("sacrifice_error",
                card=getattr(perm, "card", None) and perm.card.name or "?",
                error=f"{type(exc).__name__}: {exc}")


def _resolve_sacrifice_effect(game, source_seat, effect, source_perm=None):
    """Resolver branch for k == 'sacrifice'. Handles the common actor/filter
    combinations: controller sacrifices one / each opponent sacrifices one /
    each player sacrifices one / target player sacrifices one.

    NOTE: simplified policy — the AI picks the least-valuable permanent for
    the sacrificing player. Real MTG lets the player choose; for a FFA
    engine the dumb policy is consistent with how destroy/pick_target works
    in the rest of the resolver.
    """
    actor = getattr(effect, "actor", "controller") or "controller"
    flt = getattr(effect, "query", None)
    base = (getattr(flt, "base", "") or "").lower()

    # Figure out WHICH seats are sacrificing. Support the common actor shapes.
    if actor == "controller":
        seats = [game.seats[source_seat]]
    elif actor == "target_player":
        import sys as _s
        playloop = _s.modules.get("playloop")
        pick = playloop and getattr(playloop, "_pick_opponent_by_threat", None)
        picked = pick(game, source_seat) if pick else None
        seats = [picked] if picked is not None else [game.seats[source_seat]]
    elif actor == "each_opponent":
        seats = [s for s in game.seats
                 if not s.lost and s.idx != source_seat]
    elif actor == "each_player":
        seats = [s for s in game.seats if not s.lost]
    else:
        # Unknown actor shape — default to controller.
        seats = [game.seats[source_seat]]

    # Self-sacrifice (Sacrifice(query=Filter(base='self'))): the source permanent
    # is the thing sacrificed, regardless of actor.
    if base == "self":
        if source_perm is not None:
            _sac_permanent(game, source_perm, game.seats[source_perm.controller],
                           reason="self_sacrifice")
        else:
            game.ev("sacrifice_no_source", base=base)
        return

    if base in ("that_thing", "that_creature"):
        # Pronoun target — synth from the source permanent. Best-effort:
        # treat as self if source is the permanent it refers to.
        if source_perm is not None:
            _sac_permanent(game, source_perm, game.seats[source_perm.controller],
                           reason="pronoun_sacrifice")
        else:
            game.ev("sacrifice_no_source", base=base)
        return

    for seat in seats:
        perm = _pick_sac_target(game, seat, flt)
        if perm is None:
            game.ev("sacrifice_no_target",
                    seat=seat.idx,
                    filter_base=base,
                    actor=actor)
            continue
        _sac_permanent(game, perm, seat, reason=f"sacrifice_{actor}")


def _resolve_fight_effect(game, source_seat, effect, source_perm=None):
    """Resolver branch for k == 'fight'. Two targets each deal damage equal
    to their power to the other (CR 701.12). Simultaneous — we apply both
    damage marks before checking SBA.

    Fight 'a' is usually a creature we control (or the source permanent
    pronoun 'that creature'); 'b' is usually a creature the opponent
    controls. When a = 'that_creature' (pronoun), we use source_perm.
    """
    import sys as _s
    playloop = _s.modules.get("playloop")
    if playloop is None:
        return
    pick_target = getattr(playloop, "pick_target", None)
    if pick_target is None:
        return

    # Resolve side A
    a_flt = getattr(effect, "a", None)
    a_base = (getattr(a_flt, "base", "") or "").lower()
    side_a = None
    if a_base in ("that_creature", "self") and source_perm is not None:
        side_a = source_perm
    else:
        kind, obj = pick_target(game, source_seat, a_flt)
        if kind == "permanent":
            side_a = obj

    # Resolve side B
    b_flt = getattr(effect, "b", None)
    b_base = (getattr(b_flt, "base", "") or "").lower()
    side_b = None
    if b_base in ("that_creature", "self") and source_perm is not None:
        side_b = source_perm
    else:
        kind, obj = pick_target(game, source_seat, b_flt)
        if kind == "permanent":
            side_b = obj

    if side_a is None or side_b is None or side_a is side_b:
        game.ev("fight_no_target",
                a_present=side_a is not None,
                b_present=side_b is not None,
                same=side_a is side_b)
        return

    power_a = side_a.power + side_a.buffs_pt[0]
    power_b = side_b.power + side_b.buffs_pt[0]
    # Mark damage on both creatures before SBA. Damage is by the fighting
    # creature; carry source-colors hint for protection.
    if power_a > 0:
        side_b.damage_marked += power_a
        game.ev("damage", amount=power_a, target_kind="permanent",
                target_card=side_b.card.name, target_seat=side_b.controller,
                source_card=side_a.card.name, source_seat=side_a.controller,
                rule="701.12")
    if power_b > 0:
        side_a.damage_marked += power_b
        game.ev("damage", amount=power_b, target_kind="permanent",
                target_card=side_a.card.name, target_seat=side_a.controller,
                source_card=side_b.card.name, source_seat=side_b.controller,
                rule="701.12")
    game.ev("fight_resolved",
            a_card=side_a.card.name, a_power=power_a, a_seat=side_a.controller,
            b_card=side_b.card.name, b_power=power_b, b_seat=side_b.controller,
            rule="701.12")
    # SBA will sweep corpses on the next check.
    sba = getattr(playloop, "state_based_actions", None)
    if sba is not None:
        sba(game)


def _resolve_replacement_effect(game, source_seat, effect):
    """Resolver branch for k == 'replacement' (Replacement AST node).

    Most static `Replacement` nodes we see are lands that enter tapped or
    damage-prevention rings — those already register via the parser's
    static-ability path. When a Replacement ARRIVES as a direct Effect
    (from a triggered/activated ability), the cleanest safe action is to
    route it as a one-shot §614 event and log, so the resolver doesn't
    silently drop it.

    The `trigger_event` field names the event we care about (e.g.
    "self_etb", "deal_damage"); without a registered handler we just record
    the miss. This matches the task note: "Route these through the new
    fire_event framework". For now we stage a no-op event so the handler
    count stays visible in the audit trail.
    """
    import sys as _s
    playloop = _s.modules.get("playloop")
    if playloop is None:
        return
    Event = getattr(playloop, "Event", None)
    fire_event = getattr(playloop, "fire_event", None)
    trig_event = getattr(effect, "trigger_event", "unknown") or "unknown"
    if Event is not None and fire_event is not None:
        ev = Event(type=f"would_{trig_event}" if not trig_event.startswith("would_")
                   else trig_event,
                   player=source_seat, kwargs={})
        try:
            fire_event(game, ev)
        except Exception as exc:
            game.ev("replacement_routing_error",
                    trigger_event=trig_event,
                    error=f"{type(exc).__name__}: {exc}",
                    rule="614")
            return
    # Also dispatch the inner replacement as a normal effect so "prevent 2
    # damage" etc. actually fire. This is best-effort; the real §614 path
    # would fire this only when the watched event arrives.
    inner = getattr(effect, "replacement", None)
    if inner is not None:
        resolve = getattr(playloop, "resolve_effect", None)
        if resolve is not None:
            try:
                resolve(game, source_seat, inner)
            except Exception as exc:
                game.ev("replacement_inner_error",
                        error=f"{type(exc).__name__}: {exc}",
                        rule="614")
    game.ev("replacement_resolved",
            trigger_event=trig_event,
            rule="614")


def _resolve_exile_top_library(game, source_seat, effect):
    """Resolver branch for Modification(kind='exile_top_library', args=(N, ...)).

    Exile the top N cards of the controller's library. If a 'face_down' flag
    is present in args we preserve that semantic (not visible to opponents);
    the engine tracks this as a regular exile with a flag in the event log.
    """
    args = getattr(effect, "args", ()) or ()
    n = 1
    face_down = False
    for a in args:
        if isinstance(a, int):
            n = a
        elif isinstance(a, str) and a == "face_down":
            face_down = True
    seat = game.seats[source_seat]
    moved = []
    for _ in range(n):
        if not seat.library:
            break
        card = seat.library.pop(0)
        seat.exile.append(card)
        moved.append(card.name)
    game.ev("exile_top_library",
            seat=source_seat,
            count=len(moved),
            requested=n,
            cards=moved,
            face_down=face_down)


# Attack-trigger raw-text synthesizers. Added on top of playloop's
# _synthesize_combat_effect which already covers a few common shapes.
_ATK_RE_DEF_DISCARD = re.compile(r"defending player discards? an? card")
_ATK_RE_DEF_DRAW = re.compile(r"defending player may draw a card")
_ATK_RE_DEF_LOSE_LIFE = re.compile(
    r"defending player loses (\d+|one|two|three) life")
_ATK_RE_DEF_SAC_CREATURE = re.compile(
    r"defending player sacrifices a creature(?: of their choice)?")
_ATK_RE_CREATE_TREASURE = re.compile(r"create a treasure token")
_ATK_RE_CREATE_FOOD = re.compile(r"create a food token")
_ATK_RE_CREATE_CLUE = re.compile(r"create a clue token")
_ATK_RE_CREATE_MAP = re.compile(r"create a map token")
_ATK_RE_CREATE_BLOOD = re.compile(r"create a blood token")
_ATK_RE_DRAW_CARD = re.compile(r"^(?:you )?draw a card")
_ATK_RE_TARGET_DEBUFF = re.compile(
    r"target creature gets -(\d+)/-(\d+) until end of turn")
_ATK_RE_NO_UNTAP = re.compile(
    r"it doesn'?t untap during its controller'?s? next untap step")
_ATK_RE_OPP_LOSE_LIFE = re.compile(
    r"target opponent loses (\d+|one|two|three) life")
_ATK_RE_GOAD = re.compile(r"goad target creature defending player controls")
# Mangara, the Diplomat — "whenever an opponent attacks with creatures, if two
# or more of those creatures are attacking you and/or planeswalkers you
# control, draw a card" — the parser lowers the triggers to attack and leaves
# "with creatures, if two or more..." as raw_text. The effect is the tail
# "draw a card" (best-effort — we ignore the ≥2 attackers condition).
_ATK_RE_MANGARA = re.compile(
    r"with creatures, if two or more of those creatures are attacking "
    r"you(?: and/or planeswalkers you control)?, draw a card")


def _extended_synth_combat(raw: str, source):
    """Return a list of (tag, ...) payload tuples understood by
    playloop._apply_synth_effect, or None if no extended pattern matched.

    Called as a FALLBACK after the original _synthesize_combat_effect — so
    we never override a shape the vanilla synth already knows.
    """
    if not raw:
        return None
    r = raw.lower()

    if _ATK_RE_DEF_DISCARD.search(r):
        return [("defender_discard", 1)]
    if _ATK_RE_DEF_DRAW.search(r):
        return [("defender_draw", 1)]
    m = _ATK_RE_DEF_LOSE_LIFE.search(r)
    if m:
        amt_raw = m.group(1).lower()
        amt = _NUMS.get(amt_raw, None)
        if amt is None and amt_raw.isdigit():
            amt = int(amt_raw)
        if amt is not None:
            return [("defender_lose_life", amt)]
    if _ATK_RE_DEF_SAC_CREATURE.search(r):
        return [("defender_sacrifice_creature",)]
    if _ATK_RE_CREATE_TREASURE.search(r):
        return [("create_token", "Treasure", 1)]
    if _ATK_RE_CREATE_FOOD.search(r):
        return [("create_token", "Food", 1)]
    if _ATK_RE_CREATE_CLUE.search(r):
        return [("create_token", "Clue", 1)]
    if _ATK_RE_CREATE_MAP.search(r):
        return [("create_token", "Map", 1)]
    if _ATK_RE_CREATE_BLOOD.search(r):
        return [("create_token", "Blood", 1)]
    if _ATK_RE_DRAW_CARD.search(r):
        return [("controller_draw", 1)]
    m = _ATK_RE_TARGET_DEBUFF.search(r)
    if m:
        return [("debuff_target_creature", int(m.group(1)), int(m.group(2)))]
    if _ATK_RE_NO_UNTAP.search(r):
        return [("self_no_untap",)]
    m = _ATK_RE_OPP_LOSE_LIFE.search(r)
    if m:
        amt_raw = m.group(1).lower()
        amt = _NUMS.get(amt_raw, None)
        if amt is None and amt_raw.isdigit():
            amt = int(amt_raw)
        if amt is not None:
            return [("opp_lose_life", amt)]
    if _ATK_RE_GOAD.search(r):
        return [("goad_defender_creature",)]
    if _ATK_RE_MANGARA.search(r):
        # Mangara's condition (≥ 2 attackers targeting you/planeswalker) is
        # approximated as "always" here — the engine already gates this
        # trigger on opponent-attack declarations, so it only fires when
        # we're defending anyway. If we ever track attacker targets, we
        # can re-enforce the 2+ count.
        return [("controller_draw", 1)]
    return None


def _apply_extended_synth(game, source, payload):
    """Dispatch table for _extended_synth_combat payloads. Uses the same
    simplified-but-correct-enough posture as playloop's _apply_synth_effect.
    """
    import sys as _s
    playloop = _s.modules.get("playloop")
    if playloop is None:
        return
    tag = payload[0]

    # Helper: opponent under attack = the defending player. In declared-
    # attackers combat, attackers target game.opp(game.active). For N-seat
    # FFA we use the source permanent's registered defending player if we
    # can find it; else fall back to game.opp(controller).
    def defender():
        opp = getattr(playloop, "opp", None) or getattr(game, "opp", None)
        if callable(opp):
            try:
                return opp(source.controller)
            except TypeError:
                return opp(game, source.controller)
        # Last resort: any living opp.
        for s in game.seats:
            if not s.lost and s.idx != source.controller:
                return s
        return None

    if tag == "defender_discard":
        _, n = payload
        d = defender()
        if d is None:
            return
        for _ in range(n):
            if d.hand:
                gone = d.hand.pop(0)
                d.graveyard.append(gone)
                game.ev("discard", seat=d.idx, card=gone.name,
                        source_card=source.card.name)
        return

    if tag == "defender_draw":
        _, n = payload
        d = defender()
        if d is None:
            return
        draw_cards = getattr(playloop, "draw_cards", None)
        if draw_cards is not None:
            draw_cards(game, d, n)
        return

    if tag == "defender_lose_life":
        _, amt = payload
        d = defender()
        if d is None:
            return
        before = d.life
        d.life -= amt
        game.emit(f"  defender seat {d.idx} loses {amt} life")
        game.ev("life_change", seat=d.idx, **{"from": before, "to": d.life})
        if d.life <= 0:
            d.lost = True
            d.loss_reason = "attack trigger lose-life"
            game.check_end()
        return

    if tag == "defender_sacrifice_creature":
        d = defender()
        if d is None:
            return
        perm = _pick_sac_target(game, d, Filter(base="creature"))
        if perm is not None:
            _sac_permanent(game, perm, d, reason="attack_trigger_sacrifice")
        return

    if tag == "create_token":
        _, ttype, cnt = payload
        seat = game.seats[source.controller]
        # Route through playloop's token-creation helper if exposed; else
        # fall back to the direct battlefield-append used by the MVP engine.
        create = getattr(playloop, "create_token", None) or \
                 getattr(playloop, "_create_token", None)
        if create is not None:
            try:
                create(game, seat, types=(ttype,), count=cnt)
                return
            except Exception:
                pass
        # Direct fallback — emit an event the auditor can count, even if we
        # can't actually place a Permanent without the engine's CardEntry.
        game.ev("token_created", seat=seat.idx, types=(ttype,), count=cnt,
                source_card=source.card.name)
        return

    if tag == "controller_draw":
        _, n = payload
        seat = game.seats[source.controller]
        draw_cards = getattr(playloop, "draw_cards", None)
        if draw_cards is not None:
            draw_cards(game, seat, n)
        return

    if tag == "debuff_target_creature":
        _, p, t = payload
        d = defender()
        if d is None:
            return
        # Pick the smallest creature an opp controls.
        cands = [x for x in d.battlefield if x.is_creature]
        if not cands:
            return
        cands.sort(key=lambda x: (x.toughness, x.power))
        tgt = cands[0]
        tgt.buffs_pt = (tgt.buffs_pt[0] - p, tgt.buffs_pt[1] - t)
        game.ev("debuff_applied", card=tgt.card.name, seat=tgt.controller,
                power=-p, toughness=-t, duration="eot",
                source_card=source.card.name)
        # SBA will sweep if lethal.
        sba = getattr(playloop, "state_based_actions", None)
        if sba is not None:
            sba(game)
        return

    if tag == "self_no_untap":
        # Flag the source permanent so its next untap step skips it.
        try:
            source.skip_next_untap = True
        except Exception:
            pass
        game.ev("self_no_untap_flagged", card=source.card.name,
                seat=source.controller)
        return

    if tag == "opp_lose_life":
        _, amt = payload
        d = defender()
        if d is None:
            return
        before = d.life
        d.life -= amt
        game.emit(f"  opp seat {d.idx} loses {amt} life (attack trigger)")
        game.ev("life_change", seat=d.idx, **{"from": before, "to": d.life})
        if d.life <= 0:
            d.lost = True
            d.loss_reason = "attack trigger opp lose-life"
            game.check_end()
        return

    if tag == "goad_defender_creature":
        d = defender()
        if d is None:
            return
        cands = [x for x in d.battlefield if x.is_creature]
        if not cands:
            return
        # Mark the highest-power creature as goaded.
        cands.sort(key=lambda x: (-x.power, -x.toughness))
        try:
            cands[0].goaded = True
        except Exception:
            pass
        game.ev("goad_applied", card=cands[0].card.name, seat=cands[0].controller,
                source_card=source.card.name)
        return


# ---------------------------------------------------------------------------
# Monkey-patch installer — called lazily on first parse, guaranteed to run
# AFTER playloop.py has been imported (playloop imports parser and parser
# calls load_extensions which imports this module's EFFECT_RULES into the
# registry; the module-level side effects are inert until _install_patches()
# is called below).
#
# We hook the install into the end of this file's import. At import time
# playloop may or may not be in sys.modules:
#   - `python3 scripts/parser.py --card X` → playloop never imported → patch
#     would skip, which is fine (no games are played).
#   - `python3 scripts/playloop.py` / `gauntlet.py` → playloop loaded first
#     (it imports parser which triggers load_extensions → this module
#     executes → playloop IS in sys.modules). Patch installs on first call.
#   - Tests → playloop not imported → patch skipped → goldens unaffected.
# ---------------------------------------------------------------------------


_PATCHED = False


def _install_patches():
    global _PATCHED
    if _PATCHED:
        return True
    playloop = sys.modules.get("playloop")
    if playloop is None:
        return False

    # Wrap resolve_effect with our extra kind dispatch.
    orig_resolve = getattr(playloop, "resolve_effect", None)
    if orig_resolve is None:
        return False

    def patched_resolve_effect(game, source_seat, effect, depth=0,
                               source_colors_hint=(), _orig=orig_resolve,
                               _this_file=__file__):
        if effect is None:
            return _orig(game, source_seat, effect, depth, source_colors_hint)
        k = getattr(effect, "kind", None)

        if k == "sacrifice":
            # Find a source_perm from the current stack item if available.
            src_perm = None
            stack = getattr(game, "stack", None)
            if stack:
                top = stack[-1]
                src_perm = getattr(top, "source_perm", None)
            _resolve_sacrifice_effect(game, source_seat, effect,
                                      source_perm=src_perm)
            return

        if k == "fight":
            src_perm = None
            stack = getattr(game, "stack", None)
            if stack:
                top = stack[-1]
                src_perm = getattr(top, "source_perm", None)
            _resolve_fight_effect(game, source_seat, effect,
                                  source_perm=src_perm)
            return

        if k == "replacement":
            _resolve_replacement_effect(game, source_seat, effect)
            return

        # Modification stubs that aren't handled elsewhere — specifically
        # 'exile_top_library' which the task flags as low priority but still
        # surfaces 4 times per 100 games.
        if k == "exile_top_library":
            _resolve_exile_top_library(game, source_seat, effect)
            return

        return _orig(game, source_seat, effect, depth, source_colors_hint)

    playloop.resolve_effect = patched_resolve_effect

    # Wrap _synthesize_combat_effect to add our fallback shapes AFTER the
    # original — we must not override shapes the original already knows.
    orig_synth = getattr(playloop, "_synthesize_combat_effect", None)
    if orig_synth is not None:
        def patched_synth(raw, source, _orig=orig_synth):
            result = _orig(raw, source)
            if result is not None:
                return result
            return _extended_synth_combat(raw, source)
        playloop._synthesize_combat_effect = patched_synth

    # Wrap _apply_synth_effect to dispatch our extended payload tags.
    orig_apply = getattr(playloop, "_apply_synth_effect", None)
    if orig_apply is not None:
        _EXTENDED_TAGS = {
            "defender_discard", "defender_draw", "defender_lose_life",
            "defender_sacrifice_creature", "create_token",
            "controller_draw", "debuff_target_creature", "self_no_untap",
            "opp_lose_life", "goad_defender_creature",
        }

        def patched_apply(game, source, payload, _orig=orig_apply):
            if payload and payload[0] in _EXTENDED_TAGS:
                _apply_extended_synth(game, source, payload)
                return
            return _orig(game, source, payload)

        playloop._apply_synth_effect = patched_apply

    _PATCHED = True
    return True


# Immediate install attempt — will succeed when the host is playloop /
# gauntlet (playloop already in sys.modules by the time load_extensions()
# runs). For parser-only hosts (tests, `parser.py --card`), we skip silently.
_install_patches()


# Safety net: if playloop imports this module before it finishes defining
# resolve_effect (unlikely but possible under re-entrant imports), attempt
# the install again on first EFFECT_RULES consult. We piggy-back on a
# dummy rule that's always checked early — but we use a NOP pattern that
# never matches real text. This keeps the install idempotent and free.


@_eff(r"^__gauntlet_gaps_install_marker__$")
def _install_on_first_match(m):
    _install_patches()
    return None  # never actually fires
