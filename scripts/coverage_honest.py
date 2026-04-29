#!/usr/bin/env python3
"""Dual-metric coverage reporter — separates "structurally typed AST" cards
from "custom-stub" cards to give an honest picture of what the parser
actually produces vs. what's been tagged for future engine work.

The "100% GREEN" number from `parser.py` is real in the sense that every
card returns an AST without parse errors. But some of that AST is:
  - typed nodes (Damage, Buff, Tutor, Destroy, etc.) that an engine can
    execute directly
  - Modification(kind="custom", args=("slug",)) stubs that say "this card
    has an effect; look up the slug in the runtime registry"
  - per-card handlers from per_card.py that intentionally emit
    custom(slug) placeholders for snowflake cards

This report splits them so external readers see the honest two numbers:
  - STRUCTURAL: % of cards whose AST is primarily typed engine-executable nodes
  - STUB: % of cards whose AST contains one or more custom(slug) markers

Usage: python3 scripts/coverage_honest.py
"""

from __future__ import annotations

import json
import sys
from collections import Counter
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import parser as p
from mtg_ast import (
    Modification, Static, Activated, Triggered, Keyword,
    Sequence, Choice, Optional_, Conditional, UnknownEffect,
)


ROOT = Path(__file__).resolve().parents[1]
REPORT = ROOT / "data" / "rules" / "coverage_honest.md"


# Modification kinds that are intentional stubs (engine-needs-slug-lookup)
_STUB_KINDS = {
    "custom",
    "spell_effect",          # fallback: "this is a spell, the rest of the oracle text becomes one Effect"
    "ability_word",          # just tags an ability-word label
    "saga_chapter",          # chapter text preserved verbatim
    "class_level_band",      # class level header text
    "orphan_choice",
    "modal_header_orphan",
    "inline_modal_with_bullets",
    "delayed_trigger",
    "delayed_eot_trigger",
    "conditional_static",    # generic "as long as X / if X" fallback
    "once_per_turn",
    "once_per_turn_may",
    "activation_restriction",
    "timing_restriction",
    "cast_restriction",
    "cast_from_gy",
    "fetch_land_tail",
    "reanimate_that_card_tail",
    "reanimate_it_tail",
    "etb_p1p1_counter",
    "during_turn_self_static",
    "trigger_restriction",
    "copy_retarget",
    "additional_cost",
    "mana_restriction",
    "modes_repeatable",
    "ability_grant",         # verbatim text stashed in args
    "pronoun_grant",
    "tribal_anthem_keyword",
    "aura_keyword",
    "equip_keyword",
    "static_keyword_self",
    "tribal_keyword",
    "tribal_anthem",
    "tribal_anthem_get",
    "tribal_anthem_have",
    "tribal_etb_trig",
    "tribal_cost_red",
    "parsed_tail",
    "self_keyword",
    "static_creatures",
    "until_next_turn",
    "cost_reduction",
    "cost_reduce_self",
    "variable_cost_reduce",
    "play_those_this_turn",
    "play_exiled_card_this_turn",
    "when_you_do_p1p1",
    "exert",
    "extra_block",
    "block_only_filter",
    "combat_restriction",
    "restriction",
    "immunity",
    "grant_flash_self",
    "no_max_hand",
    "pronoun_tap",
    "painland_tail",
    "stun_target",
    "no_regen_tail",
    "no_regen_tail_it",
    "no_untap",
    "no_untap_self",
    "aura_no_untap",
    "optional_skip_untap",
    "optional_skip_untap_self",
    "mana_retention",
    "activation_rights",
    "temp_control",
    "etb_tapped",
    "etb_with_counters",
    "must_attack",
    "library_bottom",
    "saga_chapter_final_tail",
    "modal_bullet_effect",
    "modal_bullet",
    "conditional_buff_self",
    "conditional_buff_target",
    "conditional_debuff_self",
    "counter_scoped_anthem",
    "pronoun_verb",
    "still_type",
    "is_also",
    "still_a",
    "is_every_creature_type",
    "token_type_def",
    "perpetually",
    "perpetual_mod",
    "villainous_choice",
    "opp_choice_card_pick",
    "enters_prepared",
    "replacement_static",
    "pariah_redirect",
    "worship_life_loss",
    "counter_anthem",
    "chosen_name_uncastable",
    "pithing_needle_chosen",
    "aura_trigger",
    "aura_upkeep_trigger",
    "aura_eot_trigger",
    "aura_buff",
    "chain_copy",
    "self_copy_retarget",
    "then_if_rider",
    "then_clause",
    "when_you_do",
    "unknown",
}


def count_effects(effect) -> tuple[int, int]:
    """Walk a (possibly nested) Effect node. Return (structural_count, unknown_count)."""
    if effect is None:
        return (0, 0)
    kind = getattr(effect, "kind", None)
    if kind == "unknown":
        return (0, 1)
    if kind == "sequence":
        s, u = 0, 0
        for item in effect.items:
            ss, uu = count_effects(item)
            s += ss; u += uu
        return (s, u)
    if kind == "choice":
        s, u = 0, 0
        for opt in effect.options:
            ss, uu = count_effects(opt)
            s += ss; u += uu
        return (s, u)
    if kind == "optional":
        return count_effects(effect.body)
    if kind == "conditional":
        s1, u1 = count_effects(effect.body)
        s2, u2 = count_effects(effect.else_body) if effect.else_body else (0, 0)
        return (s1 + s2, u1 + u2)
    if kind is not None:
        return (1, 0)  # typed leaf effect
    return (0, 0)


def classify_card(ast) -> str:
    """Return one of 'structural', 'stub', 'mixed', 'vanilla'.

    - 'vanilla': no abilities (token reminder-only, blank cards, etc.)
    - 'structural': every ability maps to a typed AST node (Damage, Buff, etc.)
      or a keyword. No Modification with a stub kind.
    - 'stub': at least one ability is a Modification with a stub kind,
      OR contains an UnknownEffect.
    - 'mixed': has BOTH typed nodes and stubs.
    """
    if not ast.abilities:
        return "vanilla"

    has_structural = False
    has_stub = False

    for ab in ast.abilities:
        if isinstance(ab, Keyword):
            has_structural = True
            continue
        if isinstance(ab, Static):
            mod = ab.modification
            if mod is None:
                continue
            if mod.kind in _STUB_KINDS:
                has_stub = True
            else:
                has_structural = True
            continue
        if isinstance(ab, Triggered):
            s, u = count_effects(ab.effect)
            if s > 0:
                has_structural = True
            if u > 0:
                has_stub = True
            continue
        if isinstance(ab, Activated):
            s, u = count_effects(ab.effect)
            if s > 0:
                has_structural = True
            if u > 0:
                has_stub = True
            continue

    if has_structural and has_stub:
        return "mixed"
    if has_structural:
        return "structural"
    if has_stub:
        return "stub"
    return "vanilla"


def main():
    p.load_extensions()
    cards = json.loads((ROOT / "data" / "rules" / "oracle-cards.json").read_text())
    real = [c for c in cards if p.is_real_card(c)]

    buckets = Counter()
    per_card_handled = 0

    for c in real:
        ast = p.parse_card(c)
        cls = classify_card(ast)
        buckets[cls] += 1
        if c["name"] in p.PER_CARD_HANDLERS:
            per_card_handled += 1

    total = len(real)
    pct = lambda n: f"{100 * n / total:.2f}%"

    structural = buckets["structural"]
    mixed = buckets["mixed"]
    stub = buckets["stub"]
    vanilla = buckets["vanilla"]

    engine_ready = structural + vanilla  # vanilla is "no work needed, nothing to execute"
    engine_partial = mixed
    engine_stubs = stub

    report = f"""# Honest Coverage Report

**Parser status: 100% GREEN** (every card returns an AST without parse errors).

But GREEN is two things, and the distinction matters for what the runtime engine
will actually be able to execute. This report splits them.

## Three honest numbers

| Category | Cards | % | What it means |
|---|---:|---:|---|
| **Structural** | {structural:,} | {pct(structural)} | Every ability maps to a typed AST node (Damage, Buff, Tutor, Destroy, etc.) that the engine can execute directly. |
| **Mixed** | {mixed:,} | {pct(mixed)} | Some abilities are typed, others are stubs waiting for engine-side custom resolvers. Playable but incomplete. |
| **Stub** | {stub:,} | {pct(stub)} | AST contains only stub Modifications (`custom(slug)` or similar placeholders). Card is recognized; engine needs a hand-coded resolver. |
| **Vanilla** | {vanilla:,} | {pct(vanilla)} | No oracle text (vanilla creatures, tokens with no abilities). Trivially executable. |

## Per-card handler stats

- Per-card handlers in `per_card.py`: **{len(p.PER_CARD_HANDLERS):,}** named cards
- Of those, cards that actually hit the handler (i.e., are in the oracle dump): **{per_card_handled:,}**

Per-card handlers are intentionally emitting stub placeholders for snowflake
cards. They are NOT the same as structural coverage — they're a work queue
for the runtime engine's custom-resolver dispatch.

## The honest framing

- **"100% GREEN" = 100% of cards parse without error.** This is real.
- **"Engine-executable today" = Structural + Vanilla = {engine_ready:,} ({pct(engine_ready)}).**
  For these cards, the AST is fully typed and a runtime interpreter can execute
  them based on the node types alone.
- **"Engine work owed" = Stub + Mixed = {engine_stubs + engine_partial:,} ({pct(engine_stubs + engine_partial)}).**
  These cards parse, but the runtime engine would need custom-resolver code
  keyed by slug or by card name to actually play them.

## What to show externally

When describing this project honestly:

> "The parser reaches syntactic coverage of every printed Magic card (31,639 cards,
> 100%). Of those, {pct(engine_ready)} produce a fully-typed AST that a runtime
> engine can execute from the node types alone. The remaining {pct(engine_stubs + engine_partial)}
> are recognized but carry stub modifications that will need hand-coded resolvers
> in the engine layer. This is the parser — the runtime engine is the next build."

That framing is both impressive and accurate. "Parsed every magic card" is
legitimately a thing no public FOSS project has cleanly accomplished. But
"can play every magic card" is not yet true, and this report preserves the
distinction.
"""

    REPORT.write_text(report)

    print(f"\n{'═' * 60}")
    print(f"  Honest coverage — {total:,} cards")
    print(f"{'═' * 60}\n")
    print(f"  Structural (typed AST throughout): {structural:>6,}  {pct(structural)}")
    print(f"  Mixed (some typed, some stubs):    {mixed:>6,}  {pct(mixed)}")
    print(f"  Stub (all stubs, needs resolver):  {stub:>6,}  {pct(stub)}")
    print(f"  Vanilla (no oracle text):          {vanilla:>6,}  {pct(vanilla)}")
    print()
    print(f"  Engine-executable today:           {engine_ready:>6,}  {pct(engine_ready)}")
    print(f"  Engine work owed:                  {engine_stubs + engine_partial:>6,}  {pct(engine_stubs + engine_partial)}")
    print()
    print(f"  Per-card handlers: {len(p.PER_CARD_HANDLERS):,} registered, {per_card_handled:,} in oracle pool")
    print(f"\n  → {REPORT}")


if __name__ == "__main__":
    main()
