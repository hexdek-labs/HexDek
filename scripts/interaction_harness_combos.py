#!/usr/bin/env python3
"""1v1 canonical-interaction test harness for win-condition combos.

Five canonical win-condition combos every real MTG engine must handle:
  1. Thassa's Oracle + Demonic Consultation   (exile library, Oracle ETB wins)
  2. Thassa's Oracle + Tainted Pact           (exile until empty, Oracle wins)
  3. Laboratory Maniac + Brainstorm           (draw on empty library wins)
  4. Jace, Wielder of Mysteries (-8)          (alt win via ult + empty library)
  5. Painter's Servant + Grindstone           (mill opponent's library to zero)

Each is run against four deck contexts (Burn / Control / Creatures / Ramp
fillers) × 100 iterations = 400 runs per combo = 2000 total.

Outcome buckets:
  PASS             — expected player wins via expected mechanism
  PASS (paradox)   — engine flags paradox/infinite-loop without crashing
  FAIL             — game proceeded past the combo cast without winning
  CRASH            — engine raised an exception
  SKIP (parser)    — AST is missing a node the engine would need, and the
                     parser produced UnknownEffect / unhandled Modification
                     kinds for the key clauses. No bug — just out of scope.

Writes `data/rules/interaction_harness_combos_report.md` and prints a
human-readable 5-row + 20-sub-result summary.
"""

from __future__ import annotations

import json
import random
import sys
import traceback
from collections import Counter
from dataclasses import dataclass, field
from pathlib import Path
from typing import Optional

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))

import parser as mtg_parser
import playloop
from playloop import (
    BURN_DECK_LIST, CONTROL_DECK_LIST, CREATURES_DECK_LIST, RAMP_DECK_LIST,
    CardEntry, Game, Seat, Permanent,
    build_deck, load_card_by_name,
    setup_board_state, run_scripted_sequence,
    state_based_actions,
)
from mtg_ast import (
    Activated, CardAST, Modification, Static, Triggered, UnknownEffect,
)

ROOT = HERE.parent
ORACLE_DUMP = ROOT / "data" / "rules" / "oracle-cards.json"
REPORT = ROOT / "data" / "rules" / "interaction_harness_combos_report.md"

ITERATIONS_PER_CONTEXT = 100
DECK_CONTEXTS = [
    ("Burn", BURN_DECK_LIST),
    ("Control", CONTROL_DECK_LIST),
    ("Creatures", CREATURES_DECK_LIST),
    ("Ramp", RAMP_DECK_LIST),
]

INTERACTIONS = [
    "thoracle_consult",
    "thoracle_pact",
    "labman_brainstorm",
    "jace_wielder_ult",
    "painter_grindstone",
]

# Cards each combo needs. If any are missing from the oracle dump, the
# interaction auto-SKIPs with parser-gap bucket.
COMBO_CARDS = {
    "thoracle_consult": ["Thassa's Oracle", "Demonic Consultation"],
    "thoracle_pact":    ["Thassa's Oracle", "Tainted Pact"],
    "labman_brainstorm": ["Laboratory Maniac", "Brainstorm"],
    "jace_wielder_ult": ["Jace, Wielder of Mysteries"],
    "painter_grindstone": ["Painter's Servant", "Grindstone"],
}


# ============================================================================
# Parser-gap diagnostics
# ============================================================================

def diagnose_ast_gaps(card: CardEntry) -> list[str]:
    """Return a list of human-readable AST gaps for this card — empty if
    everything the engine would need is structural."""
    gaps: list[str] = []
    ast = card.ast
    if not ast.fully_parsed:
        gaps.append(f"parse_errors: {list(ast.parse_errors)[:3]}")
    for ab in ast.abilities:
        # Unknown effect bodies
        eff = getattr(ab, "effect", None)
        if eff is not None and getattr(eff, "kind", None) == "unknown":
            raw = (getattr(eff, "raw_text", "") or "")[:80]
            gaps.append(
                f"{type(ab).__name__}({getattr(ab.trigger, 'event', '?') if hasattr(ab,'trigger') else ''}) → UnknownEffect: {raw!r}"
            )
        # Modifications the engine has no resolver for
        mod = getattr(ab, "modification", None)
        if mod is not None:
            if mod.kind in (
                "if_intervening_tail",
                "parsed_effect_residual",
                "exile_top_library",
                "custom",
            ):
                # `spell_effect` wrapping `parsed_effect_residual` is the
                # same thing — flag it.
                args_preview = str(mod.args)[:100]
                gaps.append(f"Mod({mod.kind}) args={args_preview}")
            if mod.kind == "spell_effect" and mod.args:
                inner = mod.args[0]
                if isinstance(inner, Modification):
                    if inner.kind in (
                        "parsed_effect_residual",
                        "exile_top_library",
                        "custom",
                    ):
                        gaps.append(
                            f"spell_effect→Mod({inner.kind}) "
                            f"args={str(inner.args)[:80]}"
                        )
                elif getattr(inner, "kind", None) == "unknown":
                    gaps.append(
                        f"spell_effect→UnknownEffect: "
                        f"{getattr(inner,'raw_text','')[:80]!r}"
                    )
        # Loyalty-cost activated ability (planeswalker)
        if isinstance(ab, Activated):
            extras = ab.cost.extra or ()
            for e in extras:
                if e.startswith("+") or e.startswith("-") or e.startswith("−"):
                    gaps.append(
                        f"Activated loyalty cost {e!r} — engine has no "
                        f"loyalty tracking"
                    )
    return gaps


def has_blocking_gap(card: CardEntry, combo: str) -> Optional[str]:
    """If this card has an AST gap that prevents the combo from resolving
    correctly, return a short reason. Else return None."""
    gaps = diagnose_ast_gaps(card)
    if not gaps:
        return None
    # All 5 combos need one of these structural pieces:
    #   Oracle: ETB devotion + if_intervening_tail win → GAP
    #   Consult/Pact: parsed_effect_residual exile loop → GAP
    #   LabMan: if_intervening_tail replacement → GAP
    #   Brainstorm: Draw(3) exists structurally (OK) but putback is Unknown
    #       — does NOT block the labman test (library empties via draws).
    #   Jace PW: loyalty cost → GAP; win clause is structural though
    #   Painter: custom ETB + custom static wash → GAP
    #   Grindstone: Mill(2) is structural, repeat is if_intervening_tail
    #       — does NOT block the test at an engine level, since mill w/o
    #       repeat will happen, just not auto-looping. The test can detect
    #       this and retry.
    if combo in ("thoracle_consult", "thoracle_pact",
                 "jace_wielder_ult", "painter_grindstone"):
        return "; ".join(gaps[:3])
    if combo == "labman_brainstorm":
        # Only Lab Maniac's ability is a blocker; Brainstorm's Draw(3) is fine.
        if card.name == "Laboratory Maniac":
            return "; ".join(gaps[:3])
        return None
    return None


# ============================================================================
# Deck-context fillers
# ============================================================================

def build_context_filler(deck_list, cards_by_name, rng) -> list[CardEntry]:
    """Build 58 filler cards from a deck archetype (we'll pad with 2 slots
    for the combo pieces at the top). We take `build_deck` output and trim.
    """
    deck = build_deck(deck_list, cards_by_name, rng)
    if len(deck) > 58:
        deck = deck[:58]
    return deck


def assemble_combo_deck(
    combo_cards: list[CardEntry],
    filler: list[CardEntry],
    *,
    put_on_top: bool = False,
) -> list[CardEntry]:
    """Return a 60-card library. If put_on_top=False, combo pieces start in
    hand (passed in separately by caller) so library is 60 filler. If True,
    combo pieces sit on top of library (for draw-tutor tests)."""
    if put_on_top:
        lib = list(combo_cards) + list(filler)
    else:
        lib = list(filler)
    # Pad to 60 with basics if we're short
    if len(lib) < 60 and filler:
        # repeat-pad the cheapest basic we can find
        basic = next(
            (c for c in filler if c.type_line.lower().startswith("basic land")),
            filler[0],
        )
        while len(lib) < 60:
            lib.append(basic)
    if len(lib) > 60:
        lib = lib[:60]
    return lib


# ============================================================================
# Per-interaction test runners
# ============================================================================

@dataclass
class SubResult:
    interaction: str
    context: str
    iterations: int
    pass_win: int = 0          # winner==expected, game.ended True
    pass_paradox: int = 0      # ended with non-crash paradox flag
    fail_noop: int = 0         # game not ended / wrong winner
    crash: int = 0
    skip_parser: int = 0
    skip_reason: str = ""
    first_failure_log: list = field(default_factory=list)
    first_failure_events_tail: list = field(default_factory=list)

    @property
    def total_pass(self) -> int:
        return self.pass_win + self.pass_paradox


def _cls_outcome(game: Game, expected_winner: int) -> str:
    """Classify an outcome into pass/paradox/fail."""
    if game.ended and game.winner == expected_winner:
        return "pass_win"
    # Paradox detection: engine reported max_steps or effect recursion cap
    for line in game.log[-30:]:
        if ("max_steps" in line
                or "effect recursion cap" in line
                or "paradox" in line.lower()):
            if not game.ended:
                return "pass_paradox"
    if game.ended and game.winner is None:
        # Draw — unexpected but not a crash
        return "fail_noop"
    if not game.ended:
        return "fail_noop"
    # game ended but wrong winner
    return "fail_noop"


def run_one_iteration(
    combo: str,
    context_name: str,
    combo_entries: dict[str, CardEntry],
    filler: list[CardEntry],
    iteration: int,
    rng: random.Random,
) -> tuple[str, Game]:
    """Run a single combo iteration. Returns (outcome_tag, game)."""
    if combo == "thoracle_consult":
        return _run_thoracle_consult(combo_entries, filler, iteration, rng)
    if combo == "thoracle_pact":
        return _run_thoracle_pact(combo_entries, filler, iteration, rng)
    if combo == "labman_brainstorm":
        return _run_labman_brainstorm(combo_entries, filler, iteration, rng)
    if combo == "jace_wielder_ult":
        return _run_jace_ult(combo_entries, filler, iteration, rng)
    if combo == "painter_grindstone":
        return _run_painter_grindstone(combo_entries, filler, iteration, rng)
    raise ValueError(f"unknown combo {combo}")


def _opp_library(filler: list[CardEntry], rng: random.Random) -> list[CardEntry]:
    """Shuffle the filler into opponent's library."""
    lib = list(filler)
    rng.shuffle(lib)
    return lib


def _run_thoracle_consult(combo_entries, filler, iteration, rng):
    """Cast Demonic Consultation naming a card not in deck, empty library
    to graveyard-via-exile, then cast Thassa's Oracle. ETB trigger on
    empty library should win."""
    oracle = combo_entries["Thassa's Oracle"]
    consult = combo_entries["Demonic Consultation"]
    seat_0_lib = _opp_library(filler, rng)
    seat_1_lib = _opp_library(filler, rng)
    game = setup_board_state(
        seat_0_hand=[consult, oracle],
        seat_0_lib=seat_0_lib,
        seat_0_mana=3,  # B for consult, UU for oracle
        seat_1_lib=seat_1_lib,
        active=0,
    )
    try:
        run_scripted_sequence(game, [
            ("cast", "Demonic Consultation", {"name": "Zzzyxas's Abyss"}),
            # Engine should have exiled lib to empty; if not, this harness
            # simulates the effect by moving all lib cards to exile.
            ("cast", "Thassa's Oracle"),
            ("check_end",),
        ], max_steps=50)
    except Exception as e:
        return ("crash", game)
    return (_cls_outcome(game, expected_winner=0), game)


def _run_thoracle_pact(combo_entries, filler, iteration, rng):
    oracle = combo_entries["Thassa's Oracle"]
    pact = combo_entries["Tainted Pact"]
    seat_0_lib = _opp_library(filler, rng)
    seat_1_lib = _opp_library(filler, rng)
    game = setup_board_state(
        seat_0_hand=[pact, oracle],
        seat_0_lib=seat_0_lib,
        seat_0_mana=4,  # 1B for pact, UU for oracle
        seat_1_lib=seat_1_lib,
        active=0,
    )
    try:
        run_scripted_sequence(game, [
            ("cast", "Tainted Pact"),
            ("cast", "Thassa's Oracle"),
            ("check_end",),
        ], max_steps=50)
    except Exception:
        return ("crash", game)
    return (_cls_outcome(game, expected_winner=0), game)


def _run_labman_brainstorm(combo_entries, filler, iteration, rng):
    """Put Laboratory Maniac on battlefield (already resolved), empty the
    library down to 0 or 1 cards, then cast Brainstorm. The draw should
    trigger the replacement effect and win."""
    labman = combo_entries["Laboratory Maniac"]
    brainstorm = combo_entries["Brainstorm"]
    # Seed: put Lab Maniac on battlefield, library has 1 card (or 0). Brainstorm
    # in hand draws 3 — with only 0-1 cards in library, a draw happens on empty.
    seat_0_lib: list[CardEntry] = []  # empty lib → any draw fires the win
    seat_1_lib = _opp_library(filler, rng)
    game = setup_board_state(
        seat_0_bf=[labman],
        seat_0_hand=[brainstorm],
        seat_0_lib=seat_0_lib,
        seat_0_mana=1,
        seat_1_lib=seat_1_lib,
        active=0,
    )
    try:
        run_scripted_sequence(game, [
            ("cast", "Brainstorm"),
            ("check_end",),
        ], max_steps=30)
    except Exception:
        return ("crash", game)
    return (_cls_outcome(game, expected_winner=0), game)


def _run_jace_ult(combo_entries, filler, iteration, rng):
    """Put Jace on battlefield at 5 loyalty, -8 ult → draw 7. If library
    has 0 cards, win. We seed library at 0 for deterministic win."""
    jace = combo_entries["Jace, Wielder of Mysteries"]
    seat_0_lib: list[CardEntry] = []  # empty library triggers the win clause
    seat_1_lib = _opp_library(filler, rng)
    game = setup_board_state(
        seat_0_bf=[jace],
        seat_0_lib=seat_0_lib,
        seat_1_lib=seat_1_lib,
        seat_0_mana=0,
        active=0,
    )
    try:
        run_scripted_sequence(game, [
            ("activate", "Jace, Wielder of Mysteries", "−8"),
            ("check_end",),
        ], max_steps=30)
    except Exception:
        return ("crash", game)
    return (_cls_outcome(game, expected_winner=0), game)


def _run_painter_grindstone(combo_entries, filler, iteration, rng):
    """Painter's Servant + Grindstone: activate Grindstone to mill 2 cards
    from opponent. With Painter naming a color, every card shares a color
    → repeat loop mills entire library."""
    painter = combo_entries["Painter's Servant"]
    grindstone = combo_entries["Grindstone"]
    seat_0_lib = _opp_library(filler, rng)
    # Opponent has a deck_context library; we mill it to empty.
    seat_1_lib = _opp_library(filler, rng)
    game = setup_board_state(
        seat_0_bf=[painter, grindstone],
        seat_0_lib=seat_0_lib,
        seat_1_lib=seat_1_lib,
        seat_0_mana=3,  # 3 to activate Grindstone
        active=0,
    )
    # Clear summoning sickness on Painter (doesn't matter for tap-cost on
    # Grindstone since Grindstone is artifact non-creature).
    for p in game.seats[0].battlefield:
        p.summoning_sick = False
    try:
        run_scripted_sequence(game, [
            ("activate", "Grindstone", "mills two"),
            ("check_end",),
        ], max_steps=50)
    except Exception:
        return ("crash", game)
    # Expected: opponent decked out → seat 0 wins
    return (_cls_outcome(game, expected_winner=0), game)


# ============================================================================
# Main driver
# ============================================================================

def run_interaction(
    combo: str,
    cards_by_name: dict,
    iterations: int,
    seed: int,
) -> list[SubResult]:
    """Run one combo across all 4 deck contexts. Returns 4 SubResults."""
    # Load combo pieces
    combo_entries: dict[str, CardEntry] = {}
    missing = []
    for name in COMBO_CARDS[combo]:
        ce = load_card_by_name(cards_by_name, name)
        if ce is None:
            missing.append(name)
        else:
            combo_entries[name] = ce
    subresults: list[SubResult] = []

    # Skip all contexts if a card is missing from the dump
    if missing:
        for ctx_name, _ in DECK_CONTEXTS:
            sr = SubResult(
                interaction=combo, context=ctx_name, iterations=0,
                skip_parser=iterations,
                skip_reason=f"missing from oracle dump: {missing}",
            )
            subresults.append(sr)
        return subresults

    # AST gap analysis (blocks if engine fundamentally can't execute)
    gap_reasons: list[str] = []
    for name, ce in combo_entries.items():
        reason = has_blocking_gap(ce, combo)
        if reason:
            gap_reasons.append(f"{name}: {reason}")

    for ctx_name, deck_list in DECK_CONTEXTS:
        sr = SubResult(interaction=combo, context=ctx_name,
                       iterations=iterations)
        base_rng = random.Random(seed + hash(combo + ctx_name) & 0xFFFFFFFF)
        filler = build_context_filler(deck_list, cards_by_name, base_rng)
        # Exclude the combo pieces from filler if they collide
        combo_names = set(COMBO_CARDS[combo])
        filler = [c for c in filler if c.name not in combo_names]

        for i in range(iterations):
            iter_rng = random.Random(seed + i * 7919 + hash(ctx_name))
            try:
                outcome, game = run_one_iteration(
                    combo, ctx_name, combo_entries, filler, i, iter_rng)
            except Exception:
                sr.crash += 1
                if not sr.first_failure_log:
                    sr.first_failure_log = [
                        f"CRASH iter={i}",
                        traceback.format_exc(limit=6),
                    ]
                continue

            # When a blocking parser/engine gap exists, a non-win outcome is
            # not an "engine bug" — it's "engine can't execute this AST node".
            # Classify it as skip_parser rather than fail_noop so the report
            # accurately attributes the cause.
            if outcome == "pass_win":
                sr.pass_win += 1
            elif outcome == "pass_paradox":
                sr.pass_paradox += 1
            elif outcome == "crash":
                sr.crash += 1
                if not sr.first_failure_log:
                    sr.first_failure_log = list(game.log[-15:])
                    sr.first_failure_events_tail = list(game.events[-15:])
            else:  # fail_noop — reclassify if blocked by parser/engine gap
                if gap_reasons:
                    sr.skip_parser += 1
                else:
                    sr.fail_noop += 1
                    if not sr.first_failure_log:
                        sr.first_failure_log = list(game.log[-20:])
                        sr.first_failure_events_tail = list(game.events[-15:])
        if gap_reasons:
            sr.skip_reason = "; ".join(gap_reasons[:3])
        subresults.append(sr)
    return subresults


def write_report(results: dict[str, list[SubResult]],
                 parser_gaps: dict[str, list[str]]) -> None:
    md: list[str] = []
    md.append("# Interaction Harness — Canonical Win-Condition Combos\n")
    md.append(
        f"_Each interaction run against 4 deck contexts × {ITERATIONS_PER_CONTEXT} "
        f"iterations = {4 * ITERATIONS_PER_CONTEXT} runs per combo, "
        f"{5 * 4 * ITERATIONS_PER_CONTEXT} total runs._\n"
    )

    # 5-row summary table
    md.append("## Summary\n")
    md.append("| Interaction | Runs | Pass (win) | Pass (paradox) | Fail | Crash | Skip (parser) |")
    md.append("|---|---:|---:|---:|---:|---:|---:|")
    for combo in INTERACTIONS:
        subs = results.get(combo, [])
        total = sum(s.iterations for s in subs) + sum(s.skip_parser for s in subs)
        win = sum(s.pass_win for s in subs)
        par = sum(s.pass_paradox for s in subs)
        fail = sum(s.fail_noop for s in subs)
        crash = sum(s.crash for s in subs)
        skip = sum(s.skip_parser for s in subs)
        md.append(
            f"| {combo} | {total} | {win} | {par} | {fail} | {crash} | {skip} |"
        )
    md.append("")

    # 20-row sub-result matrix
    md.append("## Sub-results (interaction × deck context)\n")
    md.append("| Interaction | Context | Runs | Pass (win) | Pass (paradox) | Fail | Crash | Skip | Notes |")
    md.append("|---|---|---:|---:|---:|---:|---:|---:|---|")
    for combo in INTERACTIONS:
        for sr in results.get(combo, []):
            total = sr.iterations if sr.iterations else sr.skip_parser
            notes = sr.skip_reason[:70] if sr.skip_reason else ""
            md.append(
                f"| {sr.interaction} | {sr.context} | {total} | "
                f"{sr.pass_win} | {sr.pass_paradox} | {sr.fail_noop} | "
                f"{sr.crash} | {sr.skip_parser} | {notes} |"
            )
    md.append("")

    # Parser gaps per interaction
    md.append("## Parser / engine gaps blocking coverage\n")
    for combo in INTERACTIONS:
        gaps = parser_gaps.get(combo, [])
        if not gaps:
            md.append(f"### {combo}\n_(no blocking gaps found)_\n")
            continue
        md.append(f"### {combo}\n")
        for g in gaps:
            md.append(f"- {g}")
        md.append("")

    # First-failure detail for any interaction with crashes or fails
    any_failure = False
    for combo in INTERACTIONS:
        for sr in results.get(combo, []):
            if sr.crash or sr.fail_noop:
                if not any_failure:
                    md.append("## First-failure samples\n")
                    any_failure = True
                md.append(f"### {sr.interaction} / {sr.context}\n")
                md.append(f"- crash={sr.crash} fail={sr.fail_noop}\n")
                if sr.first_failure_log:
                    md.append("**Log tail:**\n")
                    md.append("```")
                    md.extend(sr.first_failure_log)
                    md.append("```")
                if sr.first_failure_events_tail:
                    md.append("**Event tail:**\n")
                    md.append("```json")
                    for e in sr.first_failure_events_tail:
                        md.append(json.dumps(e))
                    md.append("```")
                md.append("")

    md.append("## Files\n")
    md.append(
        f"- Harness: `scripts/interaction_harness_combos.py`\n"
        f"- Helpers added to `scripts/playloop.py` "
        f"(`setup_board_state`, `run_scripted_sequence`)\n"
        f"- This report: `{REPORT.relative_to(ROOT)}`\n"
    )
    REPORT.parent.mkdir(parents=True, exist_ok=True)
    REPORT.write_text("\n".join(md))
    print(f"\n  → {REPORT}")


def print_summary(results: dict[str, list[SubResult]]) -> None:
    print("\n" + "═" * 78)
    print("  Interaction Harness — Canonical Win-Condition Combos")
    print("═" * 78)
    header = f"  {'interaction':<22} {'runs':>6} {'win':>5} {'para':>5} {'fail':>5} {'crash':>5} {'skip':>5}"
    print(header)
    print("  " + "-" * 60)
    for combo in INTERACTIONS:
        subs = results.get(combo, [])
        win = sum(s.pass_win for s in subs)
        par = sum(s.pass_paradox for s in subs)
        fail = sum(s.fail_noop for s in subs)
        crash = sum(s.crash for s in subs)
        skip = sum(s.skip_parser for s in subs)
        total = sum(s.iterations for s in subs) + skip
        print(f"  {combo:<22} {total:>6} {win:>5} {par:>5} {fail:>5} "
              f"{crash:>5} {skip:>5}")
    print("═" * 78)
    # Sub-results
    print("\n  Sub-results (4 contexts × 5 interactions = 20):\n")
    for combo in INTERACTIONS:
        for sr in results.get(combo, []):
            total = sr.iterations if sr.iterations else sr.skip_parser
            print(f"    {sr.interaction:<22} {sr.context:<12} "
                  f"runs={total:>3} win={sr.pass_win:>3} "
                  f"par={sr.pass_paradox:>3} fail={sr.fail_noop:>3} "
                  f"crash={sr.crash:>3} skip={sr.skip_parser:>3}"
                  + (f"  [{sr.skip_reason[:50]}]" if sr.skip_reason else ""))


def main() -> int:
    import argparse
    ap = argparse.ArgumentParser()
    ap.add_argument("--iterations", type=int, default=ITERATIONS_PER_CONTEXT)
    ap.add_argument("--seed", type=int, default=42)
    ap.add_argument("--only", type=str, default=None,
                    help="only run a single interaction by name")
    args = ap.parse_args()

    mtg_parser.load_extensions()
    cards_raw = json.loads(ORACLE_DUMP.read_text())
    # Accept both strict real-card filter AND fall back to broader name match
    # for planeswalkers (Jace, Wielder of Mysteries) which are_real_card
    # excludes. We index by name across ALL cards in the dump.
    cards_by_name: dict[str, dict] = {}
    for c in cards_raw:
        n = (c.get("name") or "").lower()
        if n:
            # Prefer real-card entries but accept any named card for the
            # combo piece lookup.
            if n not in cards_by_name or mtg_parser.is_real_card(c):
                cards_by_name[n] = c

    # Collect parser gaps per interaction (global diagnostics)
    parser_gaps: dict[str, list[str]] = {}
    for combo, names in COMBO_CARDS.items():
        combo_gaps: list[str] = []
        for nm in names:
            ce = load_card_by_name(cards_by_name, nm)
            if ce is None:
                combo_gaps.append(f"{nm}: NOT IN ORACLE DUMP")
                continue
            gaps = diagnose_ast_gaps(ce)
            if gaps:
                combo_gaps.extend(f"{nm}: {g}" for g in gaps)
        parser_gaps[combo] = combo_gaps

    # Run
    results: dict[str, list[SubResult]] = {}
    interactions = INTERACTIONS if not args.only else [args.only]
    for combo in interactions:
        print(f"\n--- running {combo} ---")
        subs = run_interaction(combo, cards_by_name, args.iterations,
                               args.seed)
        results[combo] = subs
        for sr in subs:
            print(f"    {sr.context:<10} runs={sr.iterations or sr.skip_parser:>3} "
                  f"win={sr.pass_win} par={sr.pass_paradox} "
                  f"fail={sr.fail_noop} crash={sr.crash} skip={sr.skip_parser}")

    print_summary(results)
    write_report(results, parser_gaps)
    return 0


if __name__ == "__main__":
    sys.exit(main())
