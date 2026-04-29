"""Tribal anthem patterns — lord effects and tribal-typed static buffs/abilities.

The "lord" shape — `Other Goblins get +1/+1` — is one of the oldest evergreen
static effects in the game (Lord of Atlantis, 1993) and every set ships new
versions of it. At the engine level there are only ~6 distinct mutations:

  - `tribal_anthem`     flat +N/+N to a tribal subset you control
  - `tribal_keyword`    grant a keyword (flying, islandwalk, haste, ...)
  - `tribal_cost_red`   tribal spells cost {N} less to cast
  - `tribal_etb_trig`   "whenever a <tribe> you control enters" trigger
  - `tribal_attack`     "whenever a <tribe> you control attacks" trigger
  - `tribal_chosen_type` "as this enters, choose a creature type" + anthem/etc.

All variations (Elf vs Goblin vs Sliver vs Spider) collapse onto the same
handler — the **tribe is a modifier**, not a distinct mechanic. We capture
the tribe name into the `tribe` modifier so cluster signatures stay canonical
(`tribal_anthem(tribe=goblin,p=1,t=1)` and `tribal_anthem(tribe=elf,p=1,t=1)`
cluster apart only by tribe, and the merge pass in the engine treats them as
same-handler with different template parameters).

Phrasings covered (from the Scryfall bulk, ~900+ cards):

  "Other Goblins you control get +1/+1"             → tribal_anthem
  "Other Merfolk get +1/+1 and have islandwalk"     → tribal_anthem + tribal_keyword
  "Elf creatures you control get +1/+1"             → tribal_anthem_self (includes ~)
  "Sliver creatures you control have first strike"  → tribal_keyword_self
  "Warrior spells you cast cost {1} less to cast"   → tribal_cost_red
  "Whenever a Dragon you control attacks"           → tribal_attack
  "Whenever another Goblin you control enters"      → tribal_etb_trig
  "As ~ enters, choose a creature type."            → choose_creature_type
  "Creatures you control of the chosen type get +1/+1" → chosen_type_anthem
  "Each other Elf you control gets +1/+1"           → tribal_anthem (each_other form)
  "Each creature gets +1/+1 for each other creature that shares a type" → coat_of_arms

Ordering: specific (with-keyword-grant) → anthem-only → keyword-only → trigger
forms → chosen-type variants. First-match-wins per canonical verb so the
combined "anthem+keyword" form claims `tribal_anthem` before bare anthem runs.

Registering: append to EFFECT_PATTERNS in semantic_clusters.py, preferably
before the generic `tribal_anthem` catch-all currently at
`"other [^.]+ you control (?:have|get) [^.]+"` so these more-specific captures
fire first and populate real `tribe` / `p` / `t` modifiers.

Modifier schema:
  tribe     string  — creature type (e.g. "goblin", "elf", "sliver")
                      or "creature" for colorless "other creatures" form
                      or "chosen_type" for Metallic-Mimic-style templates
  p, t      int     — power / toughness bonus
  keyword   string  — the granted keyword (flying, islandwalk, haste, etc.)
                      multi-keyword grants use "multi"
  n         int     — cost reduction amount
  trigger   string  — "enters" | "attacks" | "dies"
  scope     string  — "other" (excludes self) | "self" (includes this) |
                      "each_other" (applies per-creature)
"""

# Shared fragment: a tribe token. Covers single-word tribes (goblin, elf,
# merfolk, sliver, dragon, spider) and compound tokens that the engine may
# tokenize as one (e.g. "eldrazi scion", "zombie army"). We keep it cheap:
# one to three hyphen-aware words before the "you control" / "creatures"
# marker. Over-greedy captures (e.g. matching "target nonland" as a tribe)
# are filtered in the engine by cross-checking the creature-type vocabulary.
_TRIBE = r"([a-z][a-z' \-]*?)"
_TRIBE_WORD = r"([a-z][a-z'\-]+)"  # single-token tribe for landwalk-grant forms

PATTERNS = [
    # ------------------------------------------------------------------------
    # 1. Classic lord — "Other <tribe>s you control get +N/+N AND have <kw>"
    #    Lord of Atlantis, Goblin King, Elvish Champion, Merfolk Sovereign,
    #    Zombie Master, Master of the Pearl Trident, Timber Protector,
    #    Coralhelm Commander. These are the highest-value: anthem + keyword
    #    in a single text, so they collapse a lot of variance.
    # ------------------------------------------------------------------------
    (
        r"other " + _TRIBE + r"s?(?: creatures?)?(?: you control)? "
        r"get \+(\d+)/\+(\d+) and have ",
        "tribal_anthem_keyword",
        {"tribe": "${1}", "p": "${2}", "t": "${3}", "scope": "other"},
    ),

    # ------------------------------------------------------------------------
    # 2. Classic lord — "Other <tribe>s (you control) get +N/+N" (no keyword)
    #    Feline Sovereign, Midnight Entourage, Regal Caracal, Gallows Warden,
    #    Nightmare Shepherd, Emmara Tandris (creature-typed). Name may or may
    #    not include the word "creatures" after the tribe noun.
    # ------------------------------------------------------------------------
    (
        r"other " + _TRIBE + r"s?(?: creatures?)?(?: you control)? "
        r"get \+(\d+)/\+(\d+)",
        "tribal_anthem",
        {"tribe": "${1}", "p": "${2}", "t": "${3}", "scope": "other"},
    ),

    # ------------------------------------------------------------------------
    # 3. "Other <tribe>s you control have <keyword>" — keyword-only grant,
    #    no +N/+N. Crested Sunmare (horses get indestructible), Regisaur
    #    Alpha (dinos have haste), Spider-Punk (spiders have reach), Vihaan
    #    (outlaws have deathtouch), Aggressive Mammoth (elephants trample).
    # ------------------------------------------------------------------------
    (
        r"other " + _TRIBE + r"s?(?: creatures?)?(?: you control)? have ",
        "tribal_keyword",
        {"tribe": "${1}", "scope": "other"},
    ),

    # ------------------------------------------------------------------------
    # 4. Including-self anthem — "<tribe> creatures you control get +N/+N"
    #    Crucible of Fire (dragons), Master of Waves (elementals), Brighthearth
    #    Banneret (warriors anthem side), Bramblewood Paragon, Battle Frenzy
    #    (one-shot instant — clustered as the same mutation). Note: some
    #    "creatures" here is qualifier not a type (attacking, modified, green).
    # ------------------------------------------------------------------------
    (
        r"(?<!other )" + _TRIBE + r" creatures? you control get \+(\d+)/\+(\d+)",
        "tribal_anthem",
        {"tribe": "${1}", "p": "${2}", "t": "${3}", "scope": "self"},
    ),

    # ------------------------------------------------------------------------
    # 5. Including-self keyword — "<tribe> creatures you control have <kw>"
    #    Fangorn Tree Shepherd (treefolk vigilance), Nadu (flying creatures
    #    get a second effect), Groundshaker Sliver, Brawn (trample), Rage
    #    Reflection (double strike). Also catches "slivers you control have
    #    X" since "slivers" is the tribal noun even without "creatures".
    # ------------------------------------------------------------------------
    (
        r"(?<!other )" + _TRIBE + r" creatures? you control have ",
        "tribal_keyword",
        {"tribe": "${1}", "scope": "self"},
    ),
    (
        # No "creatures" — "slivers you control have double strike"
        # "treefolk you control have vigilance", "vehicles you control have X"
        r"(?<!other )" + _TRIBE_WORD + r"s you control have ",
        "tribal_keyword",
        {"tribe": "${1}", "scope": "self"},
    ),

    # ------------------------------------------------------------------------
    # 6. Tribal cost reduction — "<tribe> spells you cast cost {N} less"
    #    Urza's Incubator, Herald's Horn, Heraldic Banner, Oketra's Monument
    #    (cycle), Brighthearth Banneret, Bonders' Enclave? Also catches
    #    non-creature cycles like "enchantment spells cost {1} less"
    #    (Starnheim Courser) — still a cost_reduction mutation, same handler.
    # ------------------------------------------------------------------------
    (
        r"(?<!other )" + _TRIBE + r" (?:creature )?spells you cast cost "
        r"\{(\d+)\} less to cast",
        "tribal_cost_red",
        {"tribe": "${1}", "n": "${2}"},
    ),

    # ------------------------------------------------------------------------
    # 7. "Each other <tribe> you control gets +N/+N" — Metallic Mimic family
    #    phrasing (but note Mimic uses ETB-counter, not static anthem).
    #    Muxus Goblin Grandee, Vren Relentless, Tam Mindful First-Year,
    #    Vampirism, Locthwain Scorn, Bramblewood Paragon (rhyme variant).
    # ------------------------------------------------------------------------
    (
        r"each other " + _TRIBE + r" you control gets? \+(\d+)/\+(\d+)",
        "tribal_anthem",
        {"tribe": "${1}", "p": "${2}", "t": "${3}", "scope": "each_other"},
    ),

    # ------------------------------------------------------------------------
    # 8. Tribal ETB trigger — "Whenever a/another <tribe> you control enters"
    #    Utvara Hellkite, Herald's Horn (trigger on cast), Thunderfoot Baloth,
    #    Metastatic Evangel, Yorvo, Minion Reflector. The trigger *body* is
    #    highly variable; we only claim the trigger shape here.
    # ------------------------------------------------------------------------
    (
        r"^whenever (?:a|an|another) " + _TRIBE + r"(?: creature)? you control enters",
        "tribal_etb_trig",
        {"tribe": "${1}", "trigger": "enters"},
    ),

    # ------------------------------------------------------------------------
    # 9. Tribal attack trigger — "Whenever a <tribe> you control attacks"
    #    Utvara Hellkite (dragons), Tolsimir (wolves), Putrefying Rotboar
    #    (boars), Dragon Tempest, Reyhan's, Goblin Chieftain? (haste grant
    #    is anthem; different card). Same rationale as ETB: claim the shape.
    # ------------------------------------------------------------------------
    (
        r"^whenever (?:a|an|another) " + _TRIBE + r"(?: creature)? you control attacks",
        "tribal_attack",
        {"tribe": "${1}", "trigger": "attacks"},
    ),

    # ------------------------------------------------------------------------
    # 10. Tribal dies trigger — "Whenever a <tribe> you control dies"
    #     Grim Haruspex (nontoken creatures), Dictate of Erebos family cross
    #     into this shape; less common pure tribal but exists (Shadowborn
    #     Apostle engines, Rat Colony? no). Include for completeness.
    # ------------------------------------------------------------------------
    (
        r"^whenever (?:a|an|another) " + _TRIBE + r"(?: creature)? you control dies",
        "tribal_death",
        {"tribe": "${1}", "trigger": "dies"},
    ),

    # ------------------------------------------------------------------------
    # 11. Chosen-type lords — "As ~ enters, choose a creature type."
    #     Adaptive Automaton, Metallic Mimic, Door of Destinies, Roaming
    #     Throne, Coalition Construct, Vanquisher's Banner, Obelisk of Urd,
    #     Shared Triumph, Kindred Discovery, Radiant Destiny. Separate verb
    #     for the *choice* itself; the anthem/counter/trigger that follows
    #     is captured by subsequent patterns.
    # ------------------------------------------------------------------------
    (
        r"as (?:this creature|this artifact|this enchantment|~) enters,"
        r" choose a creature type",
        "choose_creature_type",
        {},
    ),

    # ------------------------------------------------------------------------
    # 12. Chosen-type anthem — "creatures you control of the chosen type
    #     get +N/+N" (Adaptive Automaton, Obelisk of Urd, Etchings of the
    #     Chosen, Radiant Destiny, Icon of Ancestry, Shared Triumph,
    #     Vanquisher's Banner). Also the "of the chosen type" form without
    #     the "creatures" noun (rare but exists).
    # ------------------------------------------------------------------------
    (
        r"(?:other )?creatures? you control of the chosen type get \+(\d+)/\+(\d+)",
        "tribal_anthem",
        {"tribe": "chosen_type", "p": "${1}", "t": "${2}", "scope": "chosen"},
    ),
    (
        r"(?:other )?creatures? you control of the chosen type have ",
        "tribal_keyword",
        {"tribe": "chosen_type", "scope": "chosen"},
    ),

    # ------------------------------------------------------------------------
    # 13. Chosen-type ETB counter — Metallic Mimic unique shape: "Each other
    #     creature you control of the chosen type enters with an additional
    #     +1/+1 counter on it." This is *not* an anthem — it's an ETB-
    #     replacement that scales over time. Separate handler.
    # ------------------------------------------------------------------------
    (
        r"each other creature you control of the chosen type enters with "
        r"an additional \+(\d+)/\+(\d+) counter",
        "tribal_etb_counter",
        {"tribe": "chosen_type", "p": "${1}", "t": "${2}"},
    ),

    # ------------------------------------------------------------------------
    # 14. Chosen-type cast trigger — "Whenever you cast a spell of the chosen
    #     type, ..." (Vanquisher's Banner draws, Door of Destinies charges up,
    #     Reflections of Littjara copies, Kindred Discovery scries/draws).
    # ------------------------------------------------------------------------
    (
        r"whenever you cast a (?:creature )?spell of the chosen type",
        "tribal_cast_trig",
        {"tribe": "chosen_type", "trigger": "cast"},
    ),

    # ------------------------------------------------------------------------
    # 15. Coat of Arms — per-creature scaling anthem based on shared types.
    #     "Each creature gets +1/+1 for each other creature on the battlefield
    #     that shares a type with it." This is *global* (both players'
    #     creatures) and combinatorial — distinct mutation from tribal_anthem.
    # ------------------------------------------------------------------------
    (
        r"each creature gets \+(\d+)/\+(\d+) for each other creature "
        r"(?:on the battlefield )?that shares",
        "coat_of_arms",
        {"p": "${1}", "t": "${2}", "scope": "global"},
    ),

    # ------------------------------------------------------------------------
    # NOTE: intentionally NOT added here:
    #
    #   - "Other elf creatures get +1/+1" without "you control" (Elvish
    #     Champion / Lord of Atlantis pre-errata form). Post-2007 errata
    #     rewrote these to "other elves you control get +1/+1", so pattern 2
    #     already catches the modern oracle text. Scryfall serves the
    #     erratified version, so a pre-errata variant is unreachable.
    #
    #   - Landwalk-grant specific pattern ("...and have forestwalk"). This
    #     is absorbed by pattern 1 (tribal_anthem_keyword), which captures
    #     the shape. The specific keyword text lives beyond the regex match
    #     boundary and is extracted by the engine's body-parser pass.
    #
    #   - Door of Destinies-style "+N/+N for each charge counter on ~".
    #     That's a *scaling* anthem driven by a counter; handled by the
    #     separate `counter_scaling_anthem` family in counter patterns.
    #
    #   - "If a <tribe> would <event>, <alt-event> instead" (Lin Sivvi
    #     substitution). Only ~5 cards have this exact shape and they're
    #     too heterogeneous to template safely — left in UNPARSED for a
    #     dedicated replacement-effect pass later.
    #
    #   - "Creatures of the chosen type have protection from X" (rare —
    #     Zur's Weirding ain't it). Catches in pattern 12 variant 2 via
    #     the general "have" rule.
    # ------------------------------------------------------------------------
]
