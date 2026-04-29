"""Ramp effect patterns — cards that fetch lands out of the library.

These extend `semantic_clusters.EFFECT_PATTERNS` to cover land-tutor phrasings
the existing `tutor_land` rule misses: plural (`up to two basic land cards`),
basic-type-specific (`a Forest card`, `a Plains, Island, Swamp, or Mountain
card`), mixed destinations (`one onto the battlefield and the other into your
hand`), variable counts (`up to X basic land cards, where X is ...`), and
and/or-typed combos (`basic land cards and/or Gate cards`).

Ordering notes (specific → general, same contract as EFFECT_PATTERNS):
 1. Mixed-destination Cultivate/Kodama's Reach templates first (they also match
    the broader "onto the battlefield tapped" rule, so they'd lose detail).
 2. Count-variable `up to X` forms next (Boundless Realms / Harvest Season).
 3. Plural + basic / typed / and-or combos.
 4. Singular typed-basic (Farseek, Three Visits, Nature's Lore).
 5. A catch-all "N land cards to battlefield" at the bottom.

Canonical verb: `ramp_lands` — one handler for "lands appear", with modifiers:
  to        — "bf" (battlefield) | "hand" | "bf_and_hand" | "bf_tapped" | "graveyard"
  n         — integer | "var" (X / any number / up to that many)
  filter    — "basic" | "typed_basic" | "land" | "basic_or_nonbasic"
  tapped    — "yes" | "no" | "mixed"

Registering: append these to EFFECT_PATTERNS in semantic_clusters.py BEFORE
the existing `tutor_land` entries, or merge into the TUTOR section.
"""

PATTERNS = [
    # ------------------------------------------------------------------------
    # 1. Cultivate / Kodama's Reach — split destination (one bf, one hand)
    # ------------------------------------------------------------------------
    (
        r"search your library for up to (\w+) basic land cards?,? "
        r"reveal (?:those|them|these) cards?,? "
        r"put one onto the battlefield tapped and the other into your hand",
        "ramp_lands",
        {"to": "bf_and_hand", "n": "${1}", "filter": "basic", "tapped": "mixed"},
    ),

    # ------------------------------------------------------------------------
    # 2. Variable-count (X is ...): Boundless Realms, Harvest Season,
    #    Traverse the Outlands, Jaheira's Respite, Celebrate the Harvest,
    #    Prismatic Undercurrents, Sporocyst (Spore Chimney)
    # ------------------------------------------------------------------------
    (
        r"search your library for up to x basic land cards,? "
        r"where x is [^.]+?,?\s*(?:\.|reveal|put) [^.]*onto the battlefield tapped",
        "ramp_lands",
        {"to": "bf", "n": "var", "filter": "basic", "tapped": "yes"},
    ),
    (
        r"search your library for up to x basic land cards,? "
        r"where x is [^.]+?\.\s*reveal [^.]*put (?:them|those cards) into your hand",
        "ramp_lands",
        {"to": "hand", "n": "var", "filter": "basic"},
    ),
    (
        r"search your library for up to x basic land cards,? where x is",
        "ramp_lands",
        {"to": "bf", "n": "var", "filter": "basic", "tapped": "yes"},
    ),

    # ------------------------------------------------------------------------
    # 3. Sacrifice-based variable: Scapeshift, Rites of Spring
    # ------------------------------------------------------------------------
    (
        r"sacrifice any number of lands\.\s*search your library for up to that many "
        r"land cards,? put them onto the battlefield tapped",
        "ramp_lands",
        {"to": "bf", "n": "var", "filter": "land", "tapped": "yes", "cost": "sac_lands"},
    ),
    (
        r"discard any number of cards\.\s*search your library for up to that many "
        r"basic land cards,? reveal them,? put them into your hand",
        "ramp_lands",
        {"to": "hand", "n": "var", "filter": "basic", "cost": "discard"},
    ),

    # ------------------------------------------------------------------------
    # 4. Plural basic land cards — onto the battlefield (tapped)
    #    Covers: Explosive Vegetation, Cultivate-with-kicker, Path of the
    #    Animist, Nissa's Expedition, Vastwood Surge, Primeval Titan,
    #    Reshape the Earth, Planar Engineering, New Generation's Technique
    # ------------------------------------------------------------------------
    (
        r"search your library for up to (\w+) basic land cards?,? "
        r"put (?:them|those cards) onto the battlefield tapped",
        "ramp_lands",
        {"to": "bf", "n": "${1}", "filter": "basic", "tapped": "yes"},
    ),
    (
        r"search your library for (\w+) basic land cards?,? "
        r"put (?:them|those cards) onto the battlefield tapped",
        "ramp_lands",
        {"to": "bf", "n": "${1}", "filter": "basic", "tapped": "yes"},
    ),

    # ------------------------------------------------------------------------
    # 5. Plural basic land cards — reveal then to hand
    #    Seek the Horizon, Yavimaya Elder (trigger body), Nissa's Triumph
    # ------------------------------------------------------------------------
    (
        r"search your library for up to (\w+) basic land cards?,? "
        r"reveal (?:them|those cards)?,?\s*put (?:them|those cards) into your hand",
        "ramp_lands",
        {"to": "hand", "n": "${1}", "filter": "basic"},
    ),
    # "up to N basic land cards and reveal them. Put one/two ... onto the
    # battlefield ... the rest into your hand." — Viewpoint Synchronization,
    # Verdant Mastery, Fork in the Road. These phrasings use a period between
    # the search-and-reveal clause and the put clause, so we allow `[\s\S]`.
    (
        r"search your library for up to (\w+) basic land cards? and reveal (?:them|those)"
        r"[\s\S]*?put [^.]*?onto the battlefield[^.]*?(?:the (?:other|rest)|one(?: of them)?) into your hand",
        "ramp_lands",
        {"to": "bf_and_hand", "n": "${1}", "filter": "basic", "tapped": "mixed"},
    ),
    # Fork in the Road: one into hand, other into graveyard
    (
        r"search your library for up to (\w+) basic land cards? and reveal (?:them|those)"
        r"[\s\S]*?put one into your hand and the other into your graveyard",
        "ramp_lands",
        {"to": "hand_and_grave", "n": "${1}", "filter": "basic"},
    ),
    # Flourishing Bloom-Kin: "up to two forest cards and reveal them. put one
    # of them onto the battlefield tapped and the other into your hand"
    (
        r"search your library for up to (\w+) (?:forest|plains|island|swamp|mountain) cards? "
        r"and reveal (?:them|those)[\s\S]*?put one(?: of them)? onto the battlefield"
        r"[^.]*?the other into your hand",
        "ramp_lands",
        {"to": "bf_and_hand", "n": "${1}", "filter": "typed_basic", "tapped": "mixed"},
    ),
    # Nissa's Triumph: "up to two basic Forest cards. ... reveal those cards,
    # put them into your hand"
    (
        r"search your library for up to (\w+) basic (?:forest|plains|island|swamp|mountain) cards?"
        r"[\s\S]*?reveal (?:them|those cards),\s*put (?:them|those cards) into your hand",
        "ramp_lands",
        {"to": "hand", "n": "${1}", "filter": "typed_basic"},
    ),

    # ------------------------------------------------------------------------
    # 6. and/or typed combos — Map the Frontier, Circuitous Route,
    #    Explore the Underdark, Reach the Horizon
    # ------------------------------------------------------------------------
    (
        r"search your library for up to (\w+) basic land cards? and/or "
        r"\w+ cards?[^.]*put (?:them|those cards) onto the battlefield tapped",
        "ramp_lands",
        {"to": "bf", "n": "${1}", "filter": "basic_or_nonbasic", "tapped": "yes"},
    ),

    # ------------------------------------------------------------------------
    # 7. Plural typed-basic (Forest/Plains/etc.) cards — onto battlefield
    #    Skyshroud Claim, Ranger's Path, Flourishing Bloom-Kin, Gift of Estates,
    #    Nissa's Pilgrimage, Nissa's Triumph, Boreas Charger
    # ------------------------------------------------------------------------
    (
        r"search your library for up to (\w+) (?:basic )?(?:forest|plains|island|swamp|mountain) cards?"
        r"[^.]*?put (?:them|those cards|one[^.]*?|the rest) onto the battlefield",
        "ramp_lands",
        {"to": "bf", "n": "${1}", "filter": "typed_basic"},
    ),
    (
        r"search your library for up to (\w+) (?:basic )?(?:forest|plains|island|swamp|mountain) cards?"
        r"[^.]*?put (?:them|those cards|the rest) into your hand",
        "ramp_lands",
        {"to": "hand", "n": "${1}", "filter": "typed_basic"},
    ),
    # "a number of Plains cards equal to [...]" — Boreas Charger
    (
        r"search your library for a number of (?:forest|plains|island|swamp|mountain) cards "
        r"equal to [^.]+?put one[^.]*onto the battlefield[^.]*the rest into your hand",
        "ramp_lands",
        {"to": "bf_and_hand", "n": "var", "filter": "typed_basic", "tapped": "mixed"},
    ),

    # ------------------------------------------------------------------------
    # 8. Plural generic "land cards" — Hour of Promise, Primeval Titan,
    #    Scapeshift (post-sac), Elemental Teachings, Reshape the Earth,
    #    Realms Uncharted ("with different names ... into your hand")
    # ------------------------------------------------------------------------
    (
        r"search your library for up to (\w+) land cards?[^.]*"
        r"put (?:them|those cards|the rest|the chosen cards) onto the battlefield tapped",
        "ramp_lands",
        {"to": "bf", "n": "${1}", "filter": "land", "tapped": "yes"},
    ),
    (
        r"search your library for up to (\w+) land cards? with different names[\s\S]*?"
        r"the rest into your hand",
        "ramp_lands",
        {"to": "hand", "n": "${1}", "filter": "land_different_names"},
    ),
    (
        r"search your library for up to (\w+) land cards? with different names[\s\S]*?"
        r"the rest onto the battlefield tapped",
        "ramp_lands",
        {"to": "bf", "n": "${1}", "filter": "land_different_names", "tapped": "yes"},
    ),
    # Disorienting Choice: "up to that many land cards, put them onto the bf"
    (
        r"search your library for up to that many land cards,? "
        r"put (?:them|those cards) onto the battlefield tapped",
        "ramp_lands",
        {"to": "bf", "n": "var", "filter": "land", "tapped": "yes"},
    ),

    # ------------------------------------------------------------------------
    # 9. Singular typed-basic — Three Visits, Nature's Lore,
    #    Binding the Old Gods chapter, Yavimaya Dryad, Sylvan Primordial,
    #    Into the North (snow), Scouting Hawk, Knight of the White Orchid,
    #    Field Trip (basic), Plasmancer, Starfield Shepherd, Land Grant,
    #    Edge Rover / Emergency Eject (Lander reminder). Note: normalizer
    #    strips the card's first-word if it's a substring of "battlefield"
    #    (e.g. "Field Trip" → "battle~"), so accept that fallback too.
    # ------------------------------------------------------------------------
    (
        r"search your library for a (?:basic |snow )?(?:forest|plains|island|swamp|mountain|land) card"
        r"(?:,| and)?\s*"
        r"(?:reveal (?:it|that card),?\s*)?"
        r"put (?:it|that card) onto the (?:battlefield|battle~)(?: tapped)?",
        "ramp_lands",
        {"to": "bf", "n": "1", "filter": "typed_basic"},
    ),
    # Starfield Shepherd / Safewright modal: basic X card OR <something> — hand
    (
        r"search your library for a basic (?:forest|plains|island|swamp|mountain) card or "
        r"(?:a |an )?[^.]+?card[^.]*?(?:reveal it,?\s*)?put it into your hand",
        "ramp_lands",
        {"to": "hand", "n": "1", "filter": "typed_basic_or_other"},
    ),
    (
        r"search your library for a (?:basic |snow )?(?:forest|plains|island|swamp|mountain|land) card,? "
        r"reveal (?:it|that card),?\s*put it into your hand",
        "ramp_lands",
        {"to": "hand", "n": "1", "filter": "typed_basic"},
    ),

    # ------------------------------------------------------------------------
    # 10. Two-color / multi-type choice: Farseek ("Plains, Island, Swamp, or
    #     Mountain"), Spoils of Victory (all five), Safewright Quest
    # ------------------------------------------------------------------------
    (
        r"search your library for a (?:plains|island|swamp|mountain|forest)"
        r"(?:[, ]+(?:plains|island|swamp|mountain|forest)){1,4}"
        r"[^.]*card[^.]*put (?:it|that card) onto the battlefield",
        "ramp_lands",
        {"to": "bf", "n": "1", "filter": "typed_basic_choice"},
    ),
    (
        r"search your library for a (?:forest|plains|island|swamp|mountain) or "
        r"(?:forest|plains|island|swamp|mountain) card,? reveal it,?\s*put it into your hand",
        "ramp_lands",
        {"to": "hand", "n": "1", "filter": "typed_basic_choice"},
    ),

    # ------------------------------------------------------------------------
    # 11. Search for Tomorrow pattern — name gets stripped to "~" so the
    #     word "search" is gone. Handle the degraded form.
    # ------------------------------------------------------------------------
    (
        r"~ your library for a basic land card,? put it onto the battlefield",
        "ramp_lands",
        {"to": "bf", "n": "1", "filter": "basic", "tapped": "no"},
    ),

    # ------------------------------------------------------------------------
    # 12. Singular basic land — existing tutor_land handles to-hand, but the
    #     Dig Up cleave / "[basic land]" bracket form is new, and "onto the
    #     battlefield" (not "tapped") needs its own mod.
    # ------------------------------------------------------------------------
    (
        r"search your library for a \[?basic land\]? card,? "
        r"\[?reveal it,?\]?\s*put it into your hand",
        "ramp_lands",
        {"to": "hand", "n": "1", "filter": "basic"},
    ),

    # ------------------------------------------------------------------------
    # 13. Any number of land cards → exile (Mana Severance, Trench Gorger,
    #     Scouting Trek bottom-of-library). These are thinning/utility rather
    #     than ramp-to-mana, but share the "search lands" primitive.
    # ------------------------------------------------------------------------
    (
        r"search your library for any number of (?:basic )?land cards,? exile them",
        "ramp_lands",
        {"to": "exile", "n": "var", "filter": "land"},
    ),
    (
        r"search your library for any number of basic land cards,? reveal "
        r"(?:them|those cards),?\s*then shuffle and put them on top",
        "ramp_lands",
        {"to": "library_top", "n": "var", "filter": "basic"},
    ),

    # ------------------------------------------------------------------------
    # 14. Shard Convergence-style multi-card tutor (one of each basic)
    # ------------------------------------------------------------------------
    (
        r"search your library for a plains card, an island card, a swamp card, "
        r"and a mountain card",
        "ramp_lands",
        {"to": "hand", "n": "4", "filter": "one_of_each_basic"},
    ),
]
