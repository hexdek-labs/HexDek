"""Curated card list for the golden-file regression suite.

Grouped by mechanic family so that when a group regresses we know which
extension to blame. The entire list is flattened into CARDS for easy
consumption by the generator + the pytest suite.

Target totals:
    cedh        40
    triggers    30
    keywords    30
    layer7      30
    modal       20
    ramp        20
    removal     20
    vanilla     10
    ------------
    total      200
"""

from __future__ import annotations

# cEDH staples — the most-played, most-referenced cards in the format.
# If ANY of these regress, cEDH players will scream. Priority #1 to keep green.
CEDH = [
    "Sol Ring",
    "Counterspell",
    "Brainstorm",
    "Rhystic Study",
    "Mystic Remora",
    "Cyclonic Rift",
    "Fierce Guardianship",
    "Deflecting Swat",
    "Path to Exile",
    "Swords to Plowshares",
    "Reanimate",
    "Animate Dead",
    "Necropotence",
    "Demonic Consultation",
    "Thassa's Oracle",
    "Doomsday",
    "Mana Crypt",
    "Mana Vault",
    "Chrome Mox",
    "Mox Diamond",
    "Jeweled Lotus",
    "Ad Nauseam",
    "Yawgmoth's Will",
    "Dockside Extortionist",
    "Birds of Paradise",
    "Noble Hierarch",
    "Mystic Confluence",
    "Force of Will",
    "Force of Negation",
    "Pact of Negation",
    "Vampiric Tutor",
    "Demonic Tutor",
    "Mystical Tutor",
    "Worldly Tutor",
    "Enlightened Tutor",
    "Imperial Seal",
    "Gitaxian Probe",
    "Preordain",
    "Ponder",
    "Timetwister",
]

# Trigger-family: one representative per common trigger slot.
TRIGGERS = [
    # ETB
    "Solemn Simulacrum",
    "Mulldrifter",
    "Reclamation Sage",
    "Eternal Witness",
    "Acidic Slime",
    # Death
    "Blood Artist",
    "Zulaport Cutthroat",
    "Grim Haruspex",
    "Midnight Reaper",
    # Attack
    "Hellrider",
    "Edric, Spymaster of Trest",
    "Bloodthirster",
    # Block
    "Thorn Lieutenant",
    # Combat damage
    "Coastal Piracy",
    "Bident of Thassa",
    "Ophidian",
    # Upkeep
    "Howling Mine",
    "Phyrexian Arena",
    "Dark Confidant",
    # End step
    "Nettlecyst",
    # Cast
    "Guttersnipe",
    "Kalamax, the Stormsire",
    "Thousand-Year Storm",
    # Cast from graveyard / other
    "Snapcaster Mage",
    # LTB
    "Fleshbag Marauder",
    # Sac trigger
    "Viscera Seer",
    # Untap
    "Seedborn Muse",
    # Token create
    "Avenger of Zendikar",
    # Draw trigger
    "Consecrated Sphinx",
    # Cast noncreature
    "Young Pyromancer",
]

# Keyword-family: samples across evergreen, mechanic-specific, deciduous.
KEYWORDS = [
    "Serra Angel",              # Flying, Vigilance
    "Rumbling Baloth",          # Trample
    "Kitesail Freebooter",      # Flying
    "Shivan Dragon",            # Flying, firebreathing
    "Tormented Hero",           # Heroic
    "Dauntless Bodyguard",      # Protect
    "Resolute Archangel",       # Flying
    "Dromad Purebred",          # Cycling
    "Decree of Justice",        # Cycling w/ effect
    "Rift Bolt",                # Suspend
    "Ancestral Vision",         # Suspend
    "Lingering Souls",          # Flashback
    "Fires of Yavimaya",        # Haste anthem
    "Vexing Devil",             # Choice
    "Bogardan Hellkite",        # ETB damage
    "Vengevine",                # Recursion
    "Misthollow Griffin",       # Exile cast
    "Fireblast",                # Alternate cost
    "Great Furnace",            # Artifact land
    "Changeling Outcast",       # Changeling
    "Nameless Inversion",       # Tribal Changeling
    "Thought-Knot Seer",        # Devoid
    "Dismember",                # Phyrexian mana
    "Basking Rootwalla",        # Madness
    "Circular Logic",           # Madness
    "Rukh Egg",                 # Echo
    "Keldon Champion",          # Echo
    "Rolling Thunder",          # Buyback/X
    "Snapback",                 # Buyback
    "Ichorid",                  # Intrinsic recursion
]

# Layer-7 P/T — anthems, -X/-X, base-P/T-sets, counters.
LAYER7 = [
    "Glorious Anthem",
    "Dictate of Heliod",
    "Honor of the Pure",
    "Lord of Atlantis",
    "Goblin King",
    "Elvish Archdruid",
    "Coat of Arms",
    "Intangible Virtue",
    "Unholy Strength",
    "Bonesplitter",
    "Humility",                  # layer-7 set-base-P/T
    "Opalescence",
    "Blood Moon",                # layer-4 but also textbox rewrite
    "Urborg, Tomb of Yawgmoth",  # type-add
    "Mirrorworks",
    "Corpsejack Menace",         # doubling counters
    "Hardened Scales",
    "Winding Constrictor",
    "Master Biomancer",
    "Juniper Order Ranger",
    "Scute Swarm",
    "Cathars' Crusade",
    "Parallel Lives",
    "Doubling Season",
    "Primal Vigor",
    "Anointed Procession",
    "Leonin Warleader",
    "Hero of Bladehold",
    "Elspeth, Sun's Champion",
    "Phyrexian Obliterator",     # strict P/T
]

# Modal commands / charms.
MODAL = [
    "Cryptic Command",
    "Kolaghan's Command",
    "Primal Command",
    "Bant Charm",
    "Esper Charm",
    "Naya Charm",
    "Grixis Charm",
    "Jund Charm",
    "Boros Charm",
    "Azorius Charm",
    "Dimir Charm",
    "Izzet Charm",
    "Golgari Charm",
    "Gruul Charm",
    "Selesnya Charm",
    "Fire // Ice",
    "Wear // Tear",
    "Abzan Charm",
    "Temur Charm",
    "Atarka's Command",
]

# Ramp / tutors-for-lands.
RAMP = [
    "Cultivate",
    "Kodama's Reach",
    "Rampant Growth",
    "Farseek",
    "Nature's Lore",
    "Three Visits",
    "Wood Elves",
    "Sakura-Tribe Elder",
    "Yavimaya Elder",
    "Skyshroud Claim",
    "Explosive Vegetation",
    "Circuitous Route",
    "Harrow",
    "Search for Tomorrow",
    "Arcane Signet",
    "Talisman of Dominance",
    "Simic Signet",
    "Llanowar Elves",
    "Elvish Mystic",
    "Fyndhorn Elves",
]

# Removal — destroy / exile / bounce with varied filters.
REMOVAL = [
    "Doom Blade",
    "Go for the Throat",
    "Hero's Downfall",
    "Murderous Cut",
    "Vindicate",
    "Anguished Unmaking",
    "Utter End",
    "Assassin's Trophy",
    "Putrefy",
    "Mortify",
    "Oblivion Ring",
    "Banishing Light",
    "Grasp of Fate",
    "Chaos Warp",
    "Generous Gift",
    "Beast Within",
    "Pongify",
    "Rapid Hybridization",
    "Resculpt",
    "Reality Shift",
]

# Vanilla / French-vanilla — sanity-check. No abilities, or one keyword.
VANILLA = [
    "Grizzly Bears",
    "Hill Giant",
    "Savannah Lions",
    "Gray Ogre",
    "War Mammoth",
    "Scathe Zombies",
    "Mons's Goblin Raiders",
    "Elvish Warrior",
    "Goblin Piker",
    "Trained Armodon",
]


GROUPS: dict[str, list[str]] = {
    "cedh": CEDH,
    "triggers": TRIGGERS,
    "keywords": KEYWORDS,
    "layer7": LAYER7,
    "modal": MODAL,
    "ramp": RAMP,
    "removal": REMOVAL,
    "vanilla": VANILLA,
}


def all_cards() -> list[tuple[str, str]]:
    """Return [(group, card_name), ...] flattened, preserving group tag."""
    out = []
    for group, names in GROUPS.items():
        for n in names:
            out.append((group, n))
    return out
