"""Modal-spell pattern family for the semantic clusterer.

Owned phrasings: "Choose one —", "Choose two —", "Choose one or both —",
multi-mode entwine/escalate cards, charms, commands, confluences, and the
bullet-point sub-modes (• X, • Y) each of those surface as.

Patterns are tuples of (regex, canonical_verb, modifier_template) matching
the EFFECT_PATTERNS format in `scripts/semantic_clusters.py`. They can be
merged directly into that list (see `EXTENDS` constant below).

The existing clusterer already catches bullet sub-effects because
`normalize_text()` collapses the whole oracle text into one string and the
first-match-wins extractor sweeps across all of it — so `• Destroy target
artifact.` gets picked up by the existing `destroy(target=artifact)` rule.
What was missing was the *modal shell* itself: the "choose N" headers, the
entwine/escalate/spree alternate-mode mechanics, the confluence repeatable-
mode clause, and "each mode must target a different player" targeting
constraints. Those headers are what tell the handler "this is a modal spell;
its bullets are mode options, not a sequenced effect list."

Also emits a `modal_bullet` verb so every card with • sub-modes is flagged
as a bullet-mode spell regardless of which specific "choose N" header
phrasing kicked off the block — useful for the engine to switch into modal
resolution without re-parsing headers.
"""

from __future__ import annotations

# ---------------------------------------------------------------------------
# PATTERNS — (regex, canonical_verb, modifier_template)
# ---------------------------------------------------------------------------
# Conventions:
#   * All regexes run case-insensitive (re.I is applied by the clusterer).
#   * Dashes are normalized to "-" before matching (see normalize_text).
#   * First-match-wins PER verb, so the most specific regex for each verb
#     should come FIRST (e.g. choose_one_or_both before modal_more).
#   * `${N}` pulls regex capture group N at match time.

PATTERNS: list[tuple[str, str, dict]] = [

    # ---- MODAL HEADERS (one verb per shape, most-specific first) ----
    # "Choose one or both —" (tamiyo-style two-pick optional)
    (r"choose one or both\s*[-]",                    "modal_one_or_both",  {}),
    # "Choose one or more —" (escalate / rankle / balor family)
    (r"choose one or more\s*[-]",                    "modal_one_or_more",  {}),
    # "Choose two or more —" (strive-ish, rare)
    (r"choose two or more\s*[-]",                    "modal_two_or_more",  {}),
    # "Choose any number —"  (all-modes-optional)
    (r"choose any number\s*[-]",                     "modal_any_number",   {}),
    # "Choose any number of target ..." — modal X targeting (Soulfire
    # Eruption, Command the Dreadhorde, Cauldron Haze, Solidarity of Heroes)
    (r"choose any number of target [^.]+",           "modal_any_targets",  {}),
    # "Choose three. You may choose the same mode more than once." (Confluences)
    (r"choose three\. you may choose the same mode more than once",
                                                      "modal_three_repeat", {}),
    # "Choose X. You may choose the same mode more than once." (Doomsday Confluence)
    (r"choose x\. you may choose the same mode more than once",
                                                      "modal_x_repeat",     {}),
    # "Choose three —" (Mishra-style N=3 without repeat clause)
    (r"choose three\s*[-]",                          "modal_three",        {}),
    # "Choose two —" (commands, Cryptic Command, Kolaghan's Command)
    (r"choose two\s*[-]",                            "modal_two",          {}),
    # "Choose one —" (charms — Esper/Bant/Naya/Dromar's, Charming Prince)
    (r"choose one\s*[-]",                            "modal_one",          {}),
    # Fallback: headerless modal re-entry ("Choose one." with period, no dash)
    (r"choose one\.",                                "modal_one",          {}),
    (r"choose two\.",                                "modal_two",          {}),

    # ---- MODE-COUNT MODIFIERS (escalate gates, conditional extra modes) ----
    # "...you may choose two instead." (Flame of Anor, Weave the Nightmare —
    # conditional upgrade from one mode to two)
    (r"you may choose two instead",                  "modal_upgrade_two",  {}),
    (r"you may choose (?:one or more|two or more) instead",
                                                      "modal_upgrade_more", {}),
    # Shadrix-style: ability grants a modal choice on trigger
    (r"you may choose two\. each mode",              "modal_trigger_two",  {}),
    (r"you may choose one or more\. each mode",      "modal_trigger_more", {}),

    # ---- TARGETING CONSTRAINTS ON MODES ----
    # "Each mode must target a different player." (Shadrix, Chaos Balor,
    # Vindictive Lich, Mikey & Mona)
    (r"each mode must target a different player",    "modal_distinct_players", {}),
    (r"each mode must target a different [^.]+",     "modal_distinct_targets", {}),

    # ---- ENTWINE — pay extra to get all modes ----
    # "Entwine {cost}" — choose both (always 2-mode cards)
    (r"\bentwine \{([^}]+)\}",                       "modal_entwine",      {"cost": "${1}"}),
    # "Entwine—<non-mana cost>" (Solar Tide: sacrifice two lands; Collective
    # Effort: tap an untapped creature)
    (r"\bentwine\s*[-][^.]+",                        "modal_entwine_alt",  {}),

    # ---- ESCALATE — pay per extra mode ----
    (r"\bescalate \{([^}]+)\}",                      "modal_escalate",     {"cost": "${1}"}),
    # "Escalate—<non-mana cost>" (Collective Brutality: discard a card;
    # Collective Effort: tap an untapped creature)
    (r"\bescalate\s*[-][^.]+",                       "modal_escalate_alt", {}),

    # ---- SPREE — pay per mode, no default free mode (FIN / 2025+) ----
    # "Spree (Choose one or more additional costs.)" header
    (r"\bspree\b(?:\s*\([^)]*\))?",                  "modal_spree",        {}),
    # Spree mode line: "+ {cost} - <effect>"
    (r"\+\s*\{([^}]+)\}\s*[-]\s*",                   "modal_spree_mode",   {"cost": "${1}"}),

    # ---- BULLET SUB-MODES ----
    # Presence of bullet(s) in normalized text = modal/charm/command/spree shell.
    # This fires on ANY card whose text contains " • " (the clusterer's
    # normalize step collapses whitespace so multi-line bulleted lists end up
    # with " • " delimiters on a single line).
    (r"\s\u2022\s",                                  "modal_bullet",       {}),
    # ASCII-fallback bullet (if a card's oracle uses "* " or "- •" already
    # normalized). Keep after the unicode variant so it doesn't over-match.
    (r"(?:^|\s)\u2022\s",                            "modal_bullet",       {}),

    # ---- SPECIAL: CHARM / COMMAND / CONFLUENCE ARCHETYPE TAGS ----
    # Self-referential name-based tags won't fire because ~ has replaced the
    # name, but the shapes are covered by modal_one / modal_two / modal_three
    # above plus the bullet verb. No extra regex needed here.
]


# ---------------------------------------------------------------------------
# EXTENDS — merge hint for semantic_clusters.py
# ---------------------------------------------------------------------------
# Insert PATTERNS into EFFECT_PATTERNS BEFORE the existing "MODAL / CHOICE"
# block (the three lines starting with `choose one` / `choose two`). The
# existing entries can then be removed, since every shape they matched is
# covered here with more precision.

EXTENDS = "modal_spells"


if __name__ == "__main__":
    # Self-check: verify every regex compiles.
    import re
    for pat, verb, mods in PATTERNS:
        re.compile(pat)
    print(f"modal.py: {len(PATTERNS)} patterns compiled OK")
