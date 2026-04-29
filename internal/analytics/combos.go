package analytics

// ComboDefinition describes a known deterministic combo.
type ComboDefinition struct {
	Name        string   // human-readable name
	Pieces      []string // card names required on battlefield
	ManaCost    int      // minimum mana needed to start the loop
	WinType     string   // "infinite_damage", "infinite_mana", "win_game", "infinite_tokens"
	Description string   // what happens when it fires
}

// KnownCombos is the database of recognized combo patterns.
var KnownCombos = []ComboDefinition{
	{
		Name:        "Thoracle Win",
		Pieces:      []string{"Thassa's Oracle"},
		ManaCost:    2, // need UU for Oracle + combo spell in hand
		WinType:     "win_game",
		Description: "Thassa's Oracle + Demonic Consultation/Tainted Pact — exile library, win on ETB",
	},
	{
		Name:        "Ragost Strongbull Loop",
		Pieces:      []string{"Penregon Strongbull", "Crime Novelist", "Nuka-Cola Vending Machine"},
		ManaCost:    1,
		WinType:     "infinite_damage",
		Description: "Sac artifact with Strongbull, Novelist makes {R}, Nuka-Cola makes Treasure, repeat",
	},
	{
		Name:        "Sanguine Bond + Exquisite Blood",
		Pieces:      []string{"Sanguine Bond", "Exquisite Blood"},
		ManaCost:    0,
		WinType:     "infinite_damage",
		Description: "Any life gain or opponent life loss triggers infinite drain loop",
	},
	{
		Name:        "Basalt Monolith + Kinnan",
		Pieces:      []string{"Basalt Monolith", "Kinnan, Bonder Prodigy"},
		ManaCost:    3,
		WinType:     "infinite_mana",
		Description: "Tap Basalt for 3+1 (Kinnan), pay 3 to untap, net +1 each cycle",
	},
	{
		Name:        "Isochron Scepter + Dramatic Reversal",
		Pieces:      []string{"Isochron Scepter"},
		ManaCost:    2,
		WinType:     "infinite_mana",
		Description: "Imprint Dramatic Reversal, untap all nonland permanents including Scepter",
	},
	{
		Name:        "Walking Ballista + Heliod",
		Pieces:      []string{"Walking Ballista", "Heliod, Sun-Crowned"},
		ManaCost:    4,
		WinType:     "infinite_damage",
		Description: "Ballista with lifelink from Heliod, remove counter to deal 1, Heliod adds counter back",
	},
	{
		Name:        "Aetherflux Reservoir Storm",
		Pieces:      []string{"Aetherflux Reservoir"},
		ManaCost:    0,
		WinType:     "infinite_damage",
		Description: "At 51+ life, pay 50 to deal 50 to each opponent",
	},
	{
		Name:        "Dockside + Temur Sabertooth",
		Pieces:      []string{"Dockside Extortionist", "Temur Sabertooth"},
		ManaCost:    3,
		WinType:     "infinite_mana",
		Description: "Bounce Dockside with Sabertooth, recast for treasures, repeat if opponents have 3+ artifacts/enchantments",
	},
	{
		Name:        "Food Chain + Eternal Creature",
		Pieces:      []string{"Food Chain"},
		ManaCost:    0,
		WinType:     "infinite_mana",
		Description: "Exile creature from command zone for mana, recast from exile, repeat",
	},
	{
		Name:        "Phyrexian Altar + Gravecrawler",
		Pieces:      []string{"Phyrexian Altar", "Gravecrawler"},
		ManaCost:    1,
		WinType:     "infinite_mana",
		Description: "Sac Gravecrawler for {B}, recast from graveyard with Zombie on board",
	},
}
