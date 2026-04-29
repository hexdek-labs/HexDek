package main

// Archetype definitions for EDH/Commander deck classification.
// Freya matches a deck's card profiles against these signatures
// to identify primary and secondary archetypes.

type ArchetypeDef struct {
	Name        string
	Description string
	Keywords    []string // oracle text patterns that signal this archetype
	KeyCards    []string // specific cards strongly associated
	Triggers    []string // trigger types that appear frequently
	Effects     []string // effect types that appear frequently
	Weight      int      // base weight for matching (higher = more cards needed)
}

var Archetypes = []ArchetypeDef{
	// ── Aggro / Combat ──
	{
		Name:        "Voltron",
		Description: "Suit up one creature (usually commander) with equipment/auras and win through commander damage",
		Keywords:    []string{"equip", "aura", "attach", "equipped creature", "enchanted creature", "commander damage", "hexproof", "indestructible", "double strike"},
		KeyCards:    []string{"Colossus Hammer", "Sigarda's Aid", "Puresteel Paladin", "Sram, Senior Edificer", "Ardenn, Intrepid Archaeologist", "Hammer of Nazahn", "Shadowspear", "Sword of Feast and Famine"},
		Triggers:    []string{"attacks", "damage"},
		Effects:     []string{"equip", "attach"},
		Weight:      5,
	},
	{
		Name:        "Aggro / Go Wide",
		Description: "Create many tokens, buff them, swing for lethal",
		Keywords:    []string{"create", "token", "all creatures you control get", "anthem", "overrun", "go wide"},
		KeyCards:    []string{"Craterhoof Behemoth", "Coat of Arms", "Beastmaster Ascension", "Triumph of the Hordes", "Shared Animosity", "Finale of Devastation"},
		Triggers:    []string{"etb", "attacks"},
		Effects:     []string{"token_create", "buff"},
		Weight:      5,
	},
	{
		Name:        "Extra Combats",
		Description: "Take additional combat phases to multiply damage output",
		Keywords:    []string{"additional combat", "extra combat", "additional combat phase", "untap all creatures"},
		KeyCards:    []string{"Aggravated Assault", "Aurelia, the Warleader", "Moraug, Fury of Akoum", "Savage Ventmaw", "Hellkite Charger", "Port Razer"},
		Triggers:    []string{"attacks", "damage"},
		Effects:     []string{"extra_combat"},
		Weight:      3,
	},
	// ── Combo ──
	{
		Name:        "Combo / Infinite",
		Description: "Assemble specific card combinations for deterministic or infinite loops",
		Keywords:    []string{"untap", "add {", "whenever you cast", "copy", "return to hand", "exile your library"},
		KeyCards:    []string{"Thassa's Oracle", "Demonic Consultation", "Tainted Pact", "Isochron Scepter", "Dramatic Reversal", "Basalt Monolith", "Walking Ballista", "Food Chain"},
		Triggers:    []string{"cast", "etb"},
		Effects:     []string{"untap", "mana_add"},
		Weight:      3,
	},
	{
		Name:        "Storm",
		Description: "Cast many spells in one turn, win via storm count or cumulative triggers",
		Keywords:    []string{"storm", "whenever you cast", "cost reduction", "draw a card", "add {", "ritual", "each instant and sorcery"},
		KeyCards:    []string{"Aetherflux Reservoir", "Bolas's Citadel", "Sensei's Divining Top", "Birgi, God of Storytelling", "Thousand-Year Storm", "Grapeshot", "Tendrils of Agony"},
		Triggers:    []string{"cast"},
		Effects:     []string{"draw", "mana_add"},
		Weight:      4,
	},
	// ── Control ──
	{
		Name:        "Control",
		Description: "Answer threats, deny resources, win in the late game",
		Keywords:    []string{"counter target", "destroy target", "exile target", "return target", "draw a card", "can't be countered"},
		KeyCards:    []string{"Cyclonic Rift", "Counterspell", "Swan Song", "Fierce Guardianship", "Mana Drain", "Rhystic Study", "Mystic Remora"},
		Triggers:    []string{"cast", "etb"},
		Effects:     []string{"counter", "destroy", "exile", "draw"},
		Weight:      6,
	},
	{
		Name:        "Stax",
		Description: "Deploy tax and lock effects to prevent opponents from playing the game",
		Keywords:    []string{"costs {1} more", "costs {2} more", "can't cast", "can't attack", "can't untap", "each upkeep", "sacrifice a creature", "nonland permanents don't untap"},
		KeyCards:    []string{"Thalia, Guardian of Thraben", "Trinisphere", "Winter Orb", "Smokestack", "Stasis", "Drannith Magistrate", "Opposition Agent", "Collector Ouphe"},
		Triggers:    []string{"upkeep"},
		Effects:     []string{"tax", "lock"},
		Weight:      4,
	},
	// ── Value / Midrange ──
	{
		Name:        "Aristocrats",
		Description: "Sacrifice creatures for value, drain opponents with death triggers",
		Keywords:    []string{"sacrifice a creature", "whenever a creature dies", "whenever another creature", "each opponent loses", "you gain", "blood artist", "drain"},
		KeyCards:    []string{"Blood Artist", "Zulaport Cutthroat", "Viscera Seer", "Phyrexian Altar", "Grave Pact", "Dictate of Erebos", "Pitiless Plunderer", "Syr Konrad, the Grim"},
		Triggers:    []string{"dies", "sacrifice"},
		Effects:     []string{"drain", "damage", "token_create"},
		Weight:      4,
	},
	{
		Name:        "Artifacts",
		Description: "Artifact synergies, artifact tokens, artifact sacrifice engines",
		Keywords:    []string{"artifact", "treasure", "whenever an artifact", "sacrifice an artifact", "artifact you control", "metalcraft", "affinity"},
		KeyCards:    []string{"Dockside Extortionist", "Smothering Tithe", "Mystic Forge", "Krark-Clan Ironworks", "Urza, Lord High Artificer", "Academy Manufactor"},
		Triggers:    []string{"sacrifice", "etb"},
		Effects:     []string{"token_create", "mana_add"},
		Weight:      5,
	},
	{
		Name:        "Enchantress",
		Description: "Cast enchantments for card advantage, build a pillowfort or value engine",
		Keywords:    []string{"enchantment", "whenever you cast an enchantment", "constellation", "enchantment you control", "aura"},
		KeyCards:    []string{"Enchantress's Presence", "Mesa Enchantress", "Sigil of the Empty Throne", "Sphere of Safety", "Replenish", "Open the Vaults"},
		Triggers:    []string{"cast", "etb"},
		Effects:     []string{"draw"},
		Weight:      5,
	},
	// ── Graveyard ──
	{
		Name:        "Reanimator",
		Description: "Fill graveyard, reanimate high-value creatures",
		Keywords:    []string{"return from graveyard", "reanimate", "put into your graveyard", "mill", "unearth", "dredge", "flashback", "escape"},
		KeyCards:    []string{"Reanimate", "Animate Dead", "Living Death", "Entomb", "Buried Alive", "Muldrotha, the Gravetide", "Meren of Clan Nel Toth"},
		Triggers:    []string{"dies", "etb"},
		Effects:     []string{"self_mill", "mass_reanimate", "reanimate"},
		Weight:      4,
	},
	{
		Name:        "Lands Matter",
		Description: "Land-based value engine — landfall, land sacrifice, land reanimation",
		Keywords:    []string{"landfall", "whenever a land", "sacrifice a land", "return all land", "land enters", "explore", "dredge"},
		KeyCards:    []string{"Splendid Reclamation", "Scapeshift", "Crucible of Worlds", "Ramunap Excavator", "The Gitrog Monster", "Omnath, Locus of Creation", "Azusa, Lost but Seeking"},
		Triggers:    []string{"landfall"},
		Effects:     []string{"land_reanimate", "land_fetch", "land_sacrifice", "self_mill"},
		Weight:      4,
	},
	// ── Tribal ──
	{
		Name:        "Tribal",
		Description: "Creature type synergies — lords, type-matters triggers, type-based payoffs",
		Keywords:    []string{"you control get", "all zombies", "all elves", "all goblins", "all dragons", "changeling", "kindred", "choose a creature type"},
		KeyCards:    []string{"Coat of Arms", "Shared Animosity", "Vanquisher's Banner", "Herald's Horn", "Icon of Ancestry", "Door of Destinies"},
		Triggers:    []string{"etb", "dies", "cast"},
		Effects:     []string{"buff", "token_create", "draw"},
		Weight:      5,
	},
	// ── Specialty ──
	{
		Name:        "Superfriends",
		Description: "Planeswalker-heavy strategy with proliferate and doubling effects",
		Keywords:    []string{"planeswalker", "loyalty", "proliferate", "doubling season", "each planeswalker"},
		KeyCards:    []string{"Doubling Season", "Vorinclex, Monstrous Raider", "The Chain Veil", "Oath of Teferi", "Deepglow Skate"},
		Triggers:    []string{"etb"},
		Effects:     []string{"proliferate", "counter_add"},
		Weight:      4,
	},
	{
		Name:        "Mill",
		Description: "Win by emptying opponent libraries",
		Keywords:    []string{"mill", "puts cards from the top", "into their graveyard", "exile top", "cards in graveyard"},
		KeyCards:    []string{"Bruvac the Grandiloquent", "Maddening Cacophony", "Traumatize", "Fleet Swallower", "Altar of Dementia", "Mindcrank"},
		Triggers:    []string{"mill"},
		Effects:     []string{"mill"},
		Weight:      4,
	},
	{
		Name:        "Lifegain",
		Description: "Gain life for value, win via life-matters payoffs",
		Keywords:    []string{"gain life", "whenever you gain life", "you gain that much life", "life total", "pay life"},
		KeyCards:    []string{"Oloro, Ageless Ascetic", "Aetherflux Reservoir", "Bolas's Citadel", "Vito, Thorn of the Dusk Rose", "Serra Ascendant", "Heliod, Sun-Crowned"},
		Triggers:    []string{"lifegain"},
		Effects:     []string{"drain", "draw"},
		Weight:      4,
	},
	{
		Name:        "Discard / Hand Attack",
		Description: "Force opponents to discard, benefit from their empty hands",
		Keywords:    []string{"discard", "each opponent discards", "whenever an opponent discards", "no maximum hand size", "madness"},
		KeyCards:    []string{"Tergrid, God of Fright", "Tinybones, Trinket Thief", "Waste Not", "Megrim", "Words of Waste", "Liliana's Caress"},
		Triggers:    []string{"discard"},
		Effects:     []string{"discard_force", "drain"},
		Weight:      4,
	},
	{
		Name:        "Blink / Flicker",
		Description: "Exile and return permanents to re-trigger ETB abilities",
		Keywords:    []string{"exile, then return", "flicker", "blink", "exile target creature you control", "enters the battlefield"},
		KeyCards:    []string{"Deadeye Navigator", "Thassa, Deep-Dwelling", "Conjurer's Closet", "Restoration Angel", "Panharmonicon", "Yarok, the Desecrated", "Ghostly Flicker", "Ephemerate"},
		Triggers:    []string{"etb"},
		Effects:     []string{"exile_return", "draw", "token_create"},
		Weight:      4,
	},
	{
		Name:        "Spellslinger",
		Description: "Instants and sorceries matter — copy, recurse, benefit from casting",
		Keywords:    []string{"instant or sorcery", "whenever you cast an instant", "copy target", "flashback", "overload", "magecraft"},
		KeyCards:    []string{"Archmage Emeritus", "Guttersnipe", "Talrand, Sky Summoner", "Thousand-Year Storm", "Mizzix's Mastery", "Narset's Reversal"},
		Triggers:    []string{"cast"},
		Effects:     []string{"draw", "damage", "copy"},
		Weight:      5,
	},
	{
		Name:        "Counters Matter",
		Description: "+1/+1 counters, proliferate, counter distribution and payoffs",
		Keywords:    []string{"+1/+1 counter", "proliferate", "counter on", "counters on", "number of counters", "modified"},
		KeyCards:    []string{"Doubling Season", "Hardened Scales", "Ozolith, the Shattered Spire", "Vorinclex, Monstrous Raider", "Atraxa, Praetors' Voice", "Winding Constrictor"},
		Triggers:    []string{"counter_matters"},
		Effects:     []string{"counter_add", "proliferate", "counter_move"},
		Weight:      4,
	},
	{
		Name:        "Theft / Clone",
		Description: "Steal or copy opponents' permanents and spells",
		Keywords:    []string{"gain control", "copy target", "clone", "becomes a copy", "control of target", "opponent controls"},
		KeyCards:    []string{"Bribery", "Treachery", "Gilded Drake", "Empress Galina", "Sakashima of a Thousand Faces", "Clever Impersonator"},
		Triggers:    []string{"etb", "cast"},
		Effects:     []string{"control_change", "copy"},
		Weight:      4,
	},
	{
		Name:        "Ninjutsu / Evasion",
		Description: "Unblockable attackers, ninjutsu, combat damage triggers",
		Keywords:    []string{"ninjutsu", "can't be blocked", "unblockable", "deals combat damage to a player", "flying", "shadow"},
		KeyCards:    []string{"Yuriko, the Tiger's Shadow", "Ingenious Infiltrator", "Tetsuko Umezawa, Fugitive", "Higure, the Still Wind", "Sakashima's Student"},
		Triggers:    []string{"damage", "attacks"},
		Effects:     []string{"draw", "topdeck_reveal"},
		Weight:      4,
	},
}

type ArchetypeMatch struct {
	Name          string
	Description   string
	Score         int
	KeyCardsFound []string
}
