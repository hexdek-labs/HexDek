package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// ---------------------------------------------------------------------------
// Resource types -- what cards actively produce or consume through abilities.
//
// IMPORTANT: These represent ACTIVE resource generation/consumption, not
// passive properties. A creature card does NOT "produce creatures" -- only
// cards that CREATE creature tokens or REANIMATE creatures produce them.
// ---------------------------------------------------------------------------

type ResourceType string

const (
	ResMana          ResourceType = "mana"
	ResToken         ResourceType = "token"
	ResCard          ResourceType = "card"
	ResLife          ResourceType = "life"
	ResCounter       ResourceType = "counter"
	ResGraveyard     ResourceType = "graveyard"      // graveyard recursion
	ResUntap         ResourceType = "untap"          // untap effects
	ResLand          ResourceType = "land"           // land enters/leaves
	ResGraveyardFill ResourceType = "graveyard_fill" // intentional self-mill
	ResLandfall      ResourceType = "landfall"       // land-enters triggers
	ResReanimate     ResourceType = "reanimate"      // return from graveyard
	ResDamage        ResourceType = "damage"         // deals damage (feeds counter/life loops)
)

// ---------------------------------------------------------------------------
// Card profile -- the semantic fingerprint of a card for combo detection.
// ---------------------------------------------------------------------------

type CardProfile struct {
	Name     string
	TypeLine string
	ManaCost string         // raw mana cost string, e.g. "{2}{U}{B}"
	CMC      int            // converted mana cost
	Produces []ResourceType // what this card actively generates
	Consumes []ResourceType // what this card requires as a cost
	Triggers []string       // what events trigger this card ("whenever X")
	Effects  []string       // what this card does (damage, draw, mill, etc)

	LandColors []string // colors this land produces (W/U/B/R/G), empty for non-lands

	// Tribal detection
	CreatureTypes  []string // creature types this card has or references
	IsTribalPayoff bool     // triggers on creatures of a specific type
	IsTribalLord   bool     // grants bonuses to creatures of a type

	IsOutlet    bool // can sacrifice/use OTHER permanents
	IsTutor     bool // searches library
	IsRemoval   bool // destroys/exiles targets
	IsWinCon    bool // can win the game directly
	IsMassWipe  bool // destroys/exiles all
	IsRecursion bool // returns cards from graveyard
	IsLand      bool // card is a land

	// Blink/Flicker detection
	IsBlinker   bool // exiles and returns permanents (blink/flicker effect)
	HasValueETB bool // ETB produces something worth blinking for
	HasManaETB  bool // ETB specifically produces mana (strongest blink combo)

	// Lifegain ↔ Lifeloss loop detection
	LifegainToDrain bool // "whenever you gain life, opponent loses life" (Sanguine Bond pattern)
	LifelossToPump  bool // "whenever opponent loses life, you gain life" (Exquisite Blood pattern)

	// Counter ↔ Damage loop detection
	CounterToDamage bool // "whenever counters are placed, deal damage" (Shalai and Hallar pattern)
	DamageToCounter bool // "whenever a source deals damage, put a counter" (The Red Terror pattern)

	// cEDH win condition pattern flags
	WinsWithEmptyLib    bool // Lab Man / Thassa's Oracle / Jace Wielder -- wins with empty library
	EmptiesLibrary      bool // Demonic Consultation / Tainted Pact / Hermit Druid / Doomsday
	IsManaPayoff        bool // X-cost damage/drain sink for infinite mana (Walking Ballista, etc)
	HasETBDamage        bool // "whenever a creature enters" + damage/drain (Purphoros pattern)
	HasDeathDrain       bool // "whenever a creature dies" + drain (Blood Artist pattern)
	MakesInfiniteTokens bool // repeatable token copy engine (goes infinite with ETB damage)
	UntapsAll           bool // "untap all nonland permanents" (Dramatic Reversal pattern)

	MandatoryTriggers bool // all triggers are "whenever" (mandatory), not "you may"

	IsFlingEffect      bool // deals damage equal to a creature's power (Fling, Chandra's Ignition, Jarad)
	HasVariablePower   bool // power scales with game state (Lord of Extinction, Consuming Aberration)
	HasHighPower       bool // base power >= 7 or commander with pump (lethal threat for fling/Ignition)

	// Expanded finisher flags
	IsMassPump       bool // gives all your creatures +X/+X and/or keywords (Craterhoof, Akroma's Will)
	IsHasteEnabler   bool // gives all your creatures haste (Anger, Fervor, Concordant Crossroads)
	IsExtraCombat    bool // grants additional combat phase (Aggravated Assault, Aurelia)
	IsDamageDoubler  bool // doubles damage output (Dictate of the Twin Gods, Furnace of Rath)
	IsXBurn          bool // X-cost direct burn/drain to opponents (Torment of Hailfire, Exsanguinate)
	IsMassTokens     bool // creates many tokens at once (Avenger of Zendikar, Army of the Damned)
	HasInfect        bool // infect or gives infect (Triumph of the Hordes, Blightsteel Colossus)
	IsStormFinisher  bool // storm + damage/drain/mill (Grapeshot, Brain Freeze, Tendrils of Agony)
	IsCmdDamageThreat bool // legendary creature with high power + evasion (2-3 swing commander kill)

	SelfExilesOnDeath  bool   // card exiles itself from graveyard on death (breaks graveyard recursion)
	RecursionDest      string // where recursion sends cards: "hand", "battlefield", "top", ""
	RequiresCombat     bool   // triggers require combat (attack/combat damage)
	HasRandomSelection bool   // selects targets "at random"

	ZoneFlows []ZoneFlow // zone transitions this card enables (for value chain detection)
}

// ---------------------------------------------------------------------------
// Combo result -- a detected interaction between cards.
// ---------------------------------------------------------------------------

type ComboResult struct {
	Cards            []string
	LoopType         string // "determined", "true_infinite", "finisher", "synergy"
	Resources        string // what the loop produces
	Description      string
	Confirmed        bool // true if matched from KnownCombos database
	NonDeterministic bool // loop depends on random selection
}

// ---------------------------------------------------------------------------
// Freya report -- the full analysis output.
// ---------------------------------------------------------------------------

type FreyaReport struct {
	DeckName      string
	DeckPath      string
	Commander     string
	TotalCards    int
	Profiles      []CardProfile
	TrueInfinites []ComboResult
	Determined    []ComboResult
	Finishers     []ComboResult
	Synergies     []ComboResult

	ComboNotes []string // partial combo piece warnings

	// Legality validation (runs before all other phases)
	Legality *LegalityReport

	TutorCount   int
	RemovalCount int
	OutletCount  int
	WinConCount  int

	// Mana curve
	ManaCurve  [8]int // index 0-6 = CMC 0-6, index 7 = CMC 7+
	AvgCMC     float64
	CurveShape string // "aggro", "midrange", "control"

	// Color analysis
	ColorDemand   map[string]int // W/U/B/R/G -> total pips needed
	ColorSupply   map[string]int // W/U/B/R/G -> total sources available
	ColorMismatch []string       // warnings about under/over-represented colors
	LandCount     int
	NonlandCount  int

	// Phase 1 statistics module
	Stats *DeckStatistics

	// Phase 2 role tagging
	Roles *RoleAnalysis

	// Phase 3 archetype classification
	Archetype *ArchetypeClassification

	// Phase 4 win line mapping
	WinLines *WinLineAnalysis

	// Phase 5 unified deck profile
	Profile *DeckProfile

	// Value chain detection
	ValueChains []ValueChain
}

// ---------------------------------------------------------------------------
// ClassifyCard -- scan oracle text to build a CardProfile.
//
// Classification is deliberately conservative: we only tag resources that
// the card ACTIVELY produces/consumes through abilities, not passive
// properties derived from its type line. This keeps combo detection
// focused on real interactions.
// ---------------------------------------------------------------------------

func ClassifyCard(name, oracleText, typeLine, manaCost string, cmc int, power string) CardProfile {
	p := CardProfile{Name: name, TypeLine: typeLine, ManaCost: manaCost, CMC: cmc}
	ot := strings.ToLower(oracleText)
	tl := strings.ToLower(typeLine)

	// Detect lands and their color production.
	if strings.Contains(tl, "land") {
		p.IsLand = true
		// Determine colors from basic land types in the type line.
		if strings.Contains(tl, "plains") {
			p.LandColors = append(p.LandColors, "W")
		}
		if strings.Contains(tl, "island") {
			p.LandColors = append(p.LandColors, "U")
		}
		if strings.Contains(tl, "swamp") {
			p.LandColors = append(p.LandColors, "B")
		}
		if strings.Contains(tl, "mountain") {
			p.LandColors = append(p.LandColors, "R")
		}
		if strings.Contains(tl, "forest") {
			p.LandColors = append(p.LandColors, "G")
		}
		// Check oracle text for "any color" / "add {X}" patterns.
		if containsAny(ot, "any color", "any one color", "any combination of colors") {
			p.LandColors = []string{"W", "U", "B", "R", "G"}
		}
		// If no colors detected from types/text but the land adds mana,
		// check oracle text for specific color symbols.
		if len(p.LandColors) == 0 {
			if strings.Contains(ot, "add {w}") || strings.Contains(ot, "{w}") && strings.Contains(ot, "add") {
				p.LandColors = append(p.LandColors, "W")
			}
			if strings.Contains(ot, "add {u}") || strings.Contains(ot, "{u}") && strings.Contains(ot, "add") {
				p.LandColors = append(p.LandColors, "U")
			}
			if strings.Contains(ot, "add {b}") || strings.Contains(ot, "{b}") && strings.Contains(ot, "add") {
				p.LandColors = append(p.LandColors, "B")
			}
			if strings.Contains(ot, "add {r}") || strings.Contains(ot, "{r}") && strings.Contains(ot, "add") {
				p.LandColors = append(p.LandColors, "R")
			}
			if strings.Contains(ot, "add {g}") || strings.Contains(ot, "{g}") && strings.Contains(ot, "add") {
				p.LandColors = append(p.LandColors, "G")
			}
		}
	}

	// ---------------------------------------------------------------
	// PRODUCES -- active resource generation only
	// ---------------------------------------------------------------

	// Mana production: "add {X}" or treasure/mana generation.
	if strings.Contains(ot, "add {") || strings.Contains(ot, "add one mana") ||
		strings.Contains(ot, "add two mana") || strings.Contains(ot, "adds one mana") {
		p.Produces = append(p.Produces, ResMana)
	}
	// Treasure token creation implies mana production too.
	if containsAny(ot, "create a treasure", "create two treasure", "create three treasure",
		"creates a treasure", "create treasure token") {
		p.Produces = append(p.Produces, ResMana)
		p.Produces = append(p.Produces, ResToken)
	}

	// Token creation (non-treasure).
	if strings.Contains(ot, "create") && strings.Contains(ot, "token") &&
		!strings.Contains(ot, "treasure") {
		p.Produces = append(p.Produces, ResToken)
	}

	// Card draw.
	if containsAny(ot, "draw a card", "draw two", "draw three", "draw cards",
		"draws a card", "draw x") && !strings.Contains(ot, "withdraw") {
		p.Produces = append(p.Produces, ResCard)
	}

	// Graveyard recursion: return from graveyard.
	if strings.Contains(ot, "return") && strings.Contains(ot, "from") &&
		strings.Contains(ot, "graveyard") &&
		(strings.Contains(ot, "battlefield") || strings.Contains(ot, "to your hand") ||
			strings.Contains(ot, "to its owner")) {
		p.Produces = append(p.Produces, ResGraveyard)
		p.IsRecursion = true
	}

	// Life gain.
	if strings.Contains(ot, "gain") && strings.Contains(ot, "life") {
		p.Produces = append(p.Produces, ResLife)
	}

	// Counter placement.
	if containsAny(ot, "put a +1/+1 counter", "put two +1/+1", "put a charge counter",
		"put a -1/-1 counter", "gets +1/+1") {
		p.Produces = append(p.Produces, ResCounter)
	}

	// Untap effects.
	if containsAny(ot, "untap target", "untap all", "untap it", "untap each") {
		p.Produces = append(p.Produces, ResUntap)
	}

	// ---------------------------------------------------------------
	// CONSUMES -- resource costs for abilities
	// ---------------------------------------------------------------

	// Sacrifice as a cost: "sacrifice a/an <type>" (not just mentioning sacrifice).
	if containsAny(ot, "sacrifice a ", "sacrifice an ", "sacrifice another",
		"sacrifice it", "sacrifice two", "sacrifice three", "sacrifice x") {
		p.IsOutlet = true
		if containsAny(ot, "sacrifice an artifact", "sacrifice a food",
			"sacrifice a treasure", "sacrifice a clue", "sacrifice a blood") {
			p.Consumes = append(p.Consumes, ResToken)
		}
		if containsAny(ot, "sacrifice a creature", "sacrifice another creature",
			"sacrifice two creatures", "sacrifice a nontoken") {
			p.Consumes = append(p.Consumes, ResToken) // tokens can be sacrificed too
		}
	}

	// Discard as a cost.
	if containsAny(ot, "discard a card", "discard two", "discard your hand",
		"discard a ", "as an additional cost") && strings.Contains(ot, "discard") {
		p.Consumes = append(p.Consumes, ResCard)
	}

	// Life payment.
	if strings.Contains(ot, "pay") && strings.Contains(ot, "life") {
		p.Consumes = append(p.Consumes, ResLife)
	}

	// Exile from graveyard.
	if strings.Contains(ot, "exile") && containsAny(ot, "from your graveyard",
		"from a graveyard", "cards from your graveyard") {
		p.Consumes = append(p.Consumes, ResGraveyard)
	}

	// Counter removal.
	if strings.Contains(ot, "remove") && strings.Contains(ot, "counter") {
		p.Consumes = append(p.Consumes, ResCounter)
	}

	// ---------------------------------------------------------------
	// TRIGGERS -- "whenever" / "when" clauses
	// ---------------------------------------------------------------

	// Only consider "whenever" for repeatable triggers. "When" is
	// typically a one-shot ETB and less combo-relevant.
	if strings.Contains(ot, "whenever") {
		if containsAny(ot, "whenever you sacrifice", "whenever a creature you control is sacrificed",
			"whenever a permanent is sacrificed", "whenever an artifact is sacrificed",
			"whenever another creature is sacrificed") {
			p.Triggers = append(p.Triggers, "sacrifice")
		}
		if containsAny(ot, "whenever a creature dies", "whenever another creature dies",
			"whenever a nontoken creature dies", "whenever a creature you control dies",
			"whenever a creature an opponent controls dies") {
			p.Triggers = append(p.Triggers, "dies")
		}
		if containsAny(ot, "whenever a creature enters", "whenever another creature enters",
			"whenever a nontoken creature enters", "whenever a creature you control enters",
			"whenever an artifact enters", "whenever a token enters",
			"whenever another permanent enters", "whenever a permanent enters") {
			p.Triggers = append(p.Triggers, "etb")
		}
		if containsAny(ot, "whenever you cast", "whenever a player casts",
			"whenever an opponent casts") {
			p.Triggers = append(p.Triggers, "cast")
		}
		if strings.Contains(ot, "whenever") && strings.Contains(ot, "attacks") {
			p.Triggers = append(p.Triggers, "attacks")
		}
		if containsAny(ot, "whenever a source deals damage", "whenever this creature deals",
			"whenever a creature deals damage", "whenever a creature you control deals damage") {
			p.Triggers = append(p.Triggers, "damage")
		}
		if containsAny(ot, "whenever you gain life", "whenever a player gains life") {
			p.Triggers = append(p.Triggers, "lifegain")
		}
		if containsAny(ot, "whenever you lose life", "whenever a player loses life",
			"whenever an opponent loses life") {
			p.Triggers = append(p.Triggers, "lifeloss")
		}
		if containsAny(ot, "whenever a token", "whenever one or more tokens") {
			p.Triggers = append(p.Triggers, "token_created")
		}
		if strings.Contains(ot, "whenever") && strings.Contains(ot, "leaves the battlefield") {
			p.Triggers = append(p.Triggers, "ltb")
		}
	}

	// Lifegain ↔ Lifeloss loop pattern detection
	// "Whenever you gain life, [opponent loses / deal damage]" = Sanguine Bond pattern
	if containsAny(ot, "whenever you gain life") &&
		containsAny(ot, "loses that much", "loses life", "deals that much", "deals damage") {
		p.LifegainToDrain = true
	}
	// "Whenever an opponent loses life, you gain [that much]" = Exquisite Blood pattern
	if containsAny(ot, "whenever an opponent loses life", "whenever a player loses life") &&
		containsAny(ot, "you gain that much", "you gain life", "gain that much life") {
		p.LifelossToPump = true
	}

	// Counter → Damage: "whenever counters are put on, deal damage"
	// Shalai and Hallar, Hallar the Firefletcher, Xyris, etc.
	if containsAny(ot, "whenever one or more +1/+1 counters", "whenever a +1/+1 counter",
		"whenever you put one or more counters", "whenever a counter is placed",
		"whenever one or more counters are put") &&
		containsAny(ot, "deals that much damage", "deals damage", "deal damage",
			"loses that much life", "loses life") {
		p.CounterToDamage = true
		p.Produces = append(p.Produces, ResDamage)
	}

	// Damage → Counter: "whenever a source deals damage, put a counter"
	// The Red Terror, Managorger Hydra (from spells), etc.
	if containsAny(ot, "whenever a red source", "whenever a source you control deals damage",
		"whenever this creature deals damage", "whenever a creature you control deals damage",
		"whenever a creature deals damage") &&
		containsAny(ot, "put a +1/+1 counter", "put a counter", "+1/+1 counter on") {
		p.DamageToCounter = true
		p.Produces = append(p.Produces, ResCounter)
		p.Triggers = append(p.Triggers, "damage")
	}

	// Also detect damage production from counter-triggered effects.
	if p.CounterToDamage {
		p.Triggers = append(p.Triggers, "counter_placed")
	}

	// ---------------------------------------------------------------
	// cEDH WIN CONDITION PATTERN FLAGS
	// ---------------------------------------------------------------
	nameLower := strings.ToLower(name)

	// Win-the-game with empty library (Lab Man line)
	if containsAny(ot, "you win the game") && containsAny(ot, "library", "devotion to blue") {
		p.WinsWithEmptyLib = true
	}
	// Catch specific cards by name since oracle text varies
	if containsAny(nameLower, "laboratory maniac", "thassa's oracle", "jace, wielder") {
		p.WinsWithEmptyLib = true
	}

	// Empties your library (Consultation / Tainted Pact / Doomsday / Hermit Druid)
	if containsAny(ot, "exile your library", "exile all cards from your library") {
		p.EmptiesLibrary = true
	}
	// "search your library for a card" + exile (without targeting opponents)
	// Demonic Consultation / Tainted Pact style -- exile until you find something
	if containsAny(ot, "search your library for a card") && strings.Contains(ot, "exile") &&
		!strings.Contains(ot, "target opponent") {
		p.EmptiesLibrary = true
	}
	// Doomsday: exile all but 5 cards, then 5-card pile with Oracle on top
	if containsAny(nameLower, "doomsday") {
		p.EmptiesLibrary = true
	}
	// Hermit Druid: put top card into graveyard, repeat if not a basic land
	// With no basics = mills entire library
	if containsAny(nameLower, "hermit druid") {
		p.EmptiesLibrary = true
	}
	// Tainted Pact / Demonic Consultation by name
	if containsAny(nameLower, "tainted pact", "demonic consultation") {
		p.EmptiesLibrary = true
	}
	// "exile cards from the top of your library" pattern (generic)
	if strings.Contains(ot, "exile") && containsAny(ot, "cards from the top of your library", "from the top of your library until") {
		p.EmptiesLibrary = true
	}

	// Infinite mana sink (X cost damage/drain/draw/destroy)
	if strings.Contains(ot, "{x}") && containsAny(ot, "damage", "loses life", "destroy", "draw") {
		p.IsManaPayoff = true
	}
	if containsAny(ot, "pay any amount of life", "pay x life") {
		p.IsManaPayoff = true
	}
	// Specific mana sinks by name
	if containsAny(nameLower, "walking ballista", "staff of domination", "finale of devastation") {
		p.IsManaPayoff = true
	}

	// ETB damage (Purphoros / Impact Tremors pattern)
	if containsAny(ot, "whenever a creature enters") && containsAny(ot, "damage", "loses life", "loses 1 life") {
		p.HasETBDamage = true
	}
	if containsAny(ot, "whenever another creature you control enters") && containsAny(ot, "damage", "loses") {
		p.HasETBDamage = true
	}
	// Specific ETB damage cards by name
	if containsAny(nameLower, "purphoros, god of the forge", "impact tremors", "terror of the peaks",
		"warstorm surge", "goblin bombardment") {
		p.HasETBDamage = true
	}

	// Death drain (Blood Artist / Zulaport Cutthroat pattern)
	if containsAny(ot, "whenever a creature dies", "whenever another creature",
		"whenever a creature you control dies") &&
		containsAny(ot, "loses life", "loses 1 life", "deals 1 damage", "each opponent loses") {
		p.HasDeathDrain = true
	}
	// Specific death drain cards by name
	if containsAny(nameLower, "blood artist", "zulaport cutthroat", "bastion of remembrance",
		"cruel celebrant", "vindictive vampire", "falkenrath noble") {
		p.HasDeathDrain = true
	}

	// Infinite token generator (creates copies repeatedly -- goes infinite with ETB damage)
	if containsAny(ot, "create a copy", "create a token that's a copy") &&
		containsAny(ot, "whenever", "each") {
		p.MakesInfiniteTokens = true
	}

	// Untaps everything (Dramatic Reversal / Turnabout pattern)
	if containsAny(ot, "untap all nonland permanents", "untap all creatures", "untap each creature") {
		p.UntapsAll = true
	}

	// ---------------------------------------------------------------
	// EXPANDED FINISHER FLAGS
	// ---------------------------------------------------------------

	// Mass pump: gives all your creatures +X/+X (stat buff required for generic match)
	if containsAny(ot, "creatures you control get +", "each creature you control gets +") {
		p.IsMassPump = true
	}
	// Keyword-only grants count as mass pump only if they include power multipliers
	if containsAny(ot, "creatures you control have", "creatures you control gain") &&
		containsAny(ot, "trample", "double strike", "flying", "indestructible", "lifelink") &&
		containsAny(ot, "get +", "+1/+1", "double strike") {
		p.IsMassPump = true
	}
	if containsAny(nameLower, "craterhoof behemoth", "akroma's will", "overwhelming stampede",
		"triumph of the hordes", "end-raze forerunners", "pathbreaker ibex",
		"overrun", "decimator of the provinces", "finale of devastation") {
		p.IsMassPump = true
	}

	// Mass haste enabler
	if containsAny(ot, "creatures you control have haste", "creatures you control gain haste",
		"other creatures you control have haste", "each creature you control has haste") {
		p.IsHasteEnabler = true
	}
	if containsAny(nameLower, "concordant crossroads", "anger", "fervor",
		"fires of yavimaya", "hammer of purphoros", "urabrask the hidden",
		"rising of the day", "temur ascendancy", "maelstrom wanderer",
		"samut, voice of dissent") {
		p.IsHasteEnabler = true
	}

	// Extra combat steps
	if containsAny(ot, "additional combat phase", "additional combat step") {
		p.IsExtraCombat = true
	}
	if containsAny(nameLower, "aggravated assault", "aurelia, the warleader",
		"najeela, the blade-blossom", "combat celebrant", "moraug, fury of akoum",
		"hellkite charger", "godo, bandit warlord", "port razer",
		"karlach, fury of avernus", "breath of fury") {
		p.IsExtraCombat = true
	}

	// Damage doublers
	if containsAny(ot, "damage would be dealt") && containsAny(ot, "double that", "twice that", "deals double", "deals twice") {
		p.IsDamageDoubler = true
	}
	if containsAny(ot, "deals double that damage", "deals twice that", "double the damage") {
		p.IsDamageDoubler = true
	}
	if containsAny(nameLower, "dictate of the twin gods", "furnace of rath",
		"gratuitous violence", "gisela, blade of goldnight", "angrath's marauders",
		"fiendish duo", "solphim, mayhem dominus", "obosh, the preypiercer",
		"twinflame tyrant", "curse of bloodletting", "fire servant") {
		p.IsDamageDoubler = true
	}

	// X-cost burn/drain to opponents
	if strings.Contains(ot, "{x}") &&
		containsAny(ot, "each opponent", "any target", "target player", "each player") &&
		containsAny(ot, "damage", "loses life", "sacrifices", "discards") {
		p.IsXBurn = true
	}
	if containsAny(nameLower, "torment of hailfire", "exsanguinate", "comet storm",
		"villainous wealth", "cut // ribbons", "debt to the deathless",
		"banefire", "jaya's immolating inferno", "crackle with power") {
		p.IsXBurn = true
	}

	// Mass token creation (5+ tokens or scales with board/resources)
	if containsAny(ot, "create x ", "create x ") && strings.Contains(ot, "token") {
		p.IsMassTokens = true
	}
	if containsAny(ot, "for each land you control") && strings.Contains(ot, "token") {
		p.IsMassTokens = true
	}
	if containsAny(ot, "for each creature") && containsAny(ot, "create a token", "create that many") {
		p.IsMassTokens = true
	}
	if containsAny(nameLower, "avenger of zendikar", "army of the damned",
		"scute swarm", "mycoloth", "martial coup", "secure the wastes",
		"finale of glory", "white sun's zenith", "deploy to the front",
		"chancellor of the forge", "storm herd", "adrix and nev") {
		p.IsMassTokens = true
	}

	// Infect / grants infect
	if containsAny(ot, "infect", "poison counter") && !strings.Contains(ot, "remove a poison") {
		p.HasInfect = true
	}
	if containsAny(ot, "creatures you control have infect", "creatures you control gain infect") {
		p.HasInfect = true
	}
	if containsAny(nameLower, "triumph of the hordes", "blightsteel colossus",
		"grafted exoskeleton", "tainted strike", "phyresis",
		"skithiryx, the blight dragon", "inexorable tide") {
		p.HasInfect = true
	}

	// Storm finisher
	if strings.Contains(ot, "storm") &&
		containsAny(ot, "damage", "loses life", "copy of this spell", "mills") {
		p.IsStormFinisher = true
	}
	if containsAny(nameLower, "grapeshot", "brain freeze", "tendrils of agony",
		"temporal fissure", "empty the warrens", "aetherflux reservoir") {
		p.IsStormFinisher = true
	}

	// Commander damage threat: legendary creature, high power + evasion
	typeLower := strings.ToLower(typeLine)
	if strings.Contains(typeLower, "legendary") && strings.Contains(typeLower, "creature") {
		pwr := 0
		if pw, err := strconv.Atoi(power); err == nil {
			pwr = pw
		}
		if pwr >= 6 && containsAny(ot, "trample", "double strike", "flying",
			"can't be blocked", "unblockable", "menace", "fear", "shadow") {
			p.IsCmdDamageThreat = true
		}
		if pwr >= 10 {
			p.IsCmdDamageThreat = true
		}
	}

	// ---------------------------------------------------------------
	// EFFECTS -- what this card does to the game state
	// ---------------------------------------------------------------
	if strings.Contains(ot, "deals") && strings.Contains(ot, "damage") {
		p.Effects = append(p.Effects, "damage")
	}
	if containsAny(ot, "destroy target", "destroy all", "destroy each") {
		p.Effects = append(p.Effects, "destroy")
	}
	if containsAny(ot, "exile target", "exile all", "exile each") {
		p.Effects = append(p.Effects, "exile")
	}
	if strings.Contains(ot, "mill") {
		p.Effects = append(p.Effects, "mill")
	}
	if containsAny(ot, "each opponent loses", "each opponent lose") {
		p.Effects = append(p.Effects, "drain")
	}
	if p.IsOutlet {
		p.Effects = append(p.Effects, "sacrifice")
	}
	if strings.Contains(ot, "create") && strings.Contains(ot, "token") {
		p.Effects = append(p.Effects, "create_token")
	}

	// ---------------------------------------------------------------
	// LAND & GRAVEYARD STRATEGY DETECTION
	// ---------------------------------------------------------------

	// Landfall triggers
	if strings.Contains(ot, "landfall") ||
		(strings.Contains(ot, "whenever") && strings.Contains(ot, "land") &&
			(strings.Contains(ot, "enters") || strings.Contains(ot, "enter"))) {
		p.Triggers = append(p.Triggers, "landfall")
	}

	// Self-mill (intentional graveyard filling)
	if strings.Contains(ot, "mill") && !strings.Contains(ot, "opponent") {
		p.Produces = append(p.Produces, ResGraveyardFill)
		p.Effects = append(p.Effects, "self_mill")
	}

	// Mass reanimation
	if (strings.Contains(ot, "return all") || strings.Contains(ot, "return each")) &&
		strings.Contains(ot, "graveyard") &&
		(strings.Contains(ot, "battlefield") || strings.Contains(ot, "hand")) {
		p.Produces = append(p.Produces, ResReanimate)
		p.Effects = append(p.Effects, "mass_reanimate")
		p.IsRecursion = true
	}

	// Land-specific reanimation
	if strings.Contains(ot, "return") && strings.Contains(ot, "land") &&
		strings.Contains(ot, "graveyard") {
		p.Produces = append(p.Produces, ResLand)
		p.Produces = append(p.Produces, ResReanimate)
		p.Effects = append(p.Effects, "land_reanimate")
		p.IsRecursion = true
	}

	// Land sacrifice (Scapeshift pattern) -- must specifically sacrifice lands,
	// not just mention sacrifice and land in the same card text.
	if containsAny(ot, "sacrifice a land", "sacrifice any number of land",
		"sacrifice two land", "sacrifice three land", "sacrifice x land",
		"sacrifices a land", "sacrifice all land") {
		p.Consumes = append(p.Consumes, ResLand)
		p.Effects = append(p.Effects, "land_sacrifice")
	}

	// Land searching/fetching
	if strings.Contains(ot, "search") && strings.Contains(ot, "land") &&
		strings.Contains(ot, "battlefield") {
		p.Produces = append(p.Produces, ResLand)
		p.Effects = append(p.Effects, "land_fetch")
	}

	// Graveyard curation (exile from your graveyard for value)
	if strings.Contains(ot, "exile") && strings.Contains(ot, "your graveyard") &&
		!strings.Contains(ot, "return") {
		p.Consumes = append(p.Consumes, ResGraveyardFill)
		p.Effects = append(p.Effects, "graveyard_curate")
	}

	// Delve (uses graveyard as fuel)
	if strings.Contains(ot, "delve") {
		p.Consumes = append(p.Consumes, ResGraveyardFill)
	}

	// Dredge (self-mill + recursion)
	if strings.Contains(ot, "dredge") {
		p.Produces = append(p.Produces, ResGraveyardFill)
		p.Produces = append(p.Produces, ResCard)
	}

	// ---------------------------------------------------------------
	// SPECIAL FLAGS
	// ---------------------------------------------------------------
	if containsAny(ot, "you win the game", "that player loses the game",
		"target player loses the game", "each opponent loses the game") {
		p.IsWinCon = true
	}
	if strings.Contains(ot, "search your library") {
		isCyclingSearch := (strings.Contains(ot, "cycling") || strings.Contains(ot, "landcycling") ||
			strings.Contains(ot, "swampcycling") || strings.Contains(ot, "forestcycling") ||
			strings.Contains(ot, "mountaincycling") || strings.Contains(ot, "islandcycling") ||
			strings.Contains(ot, "plainscycling")) &&
			!strings.Contains(ot, "search your library for a card")
		if !isCyclingSearch {
			p.IsTutor = true
		}
	}
	if containsAny(ot, "destroy target", "exile target") {
		p.IsRemoval = true
	}
	if containsAny(ot, "destroy all creatures", "destroy all permanents",
		"destroy all artifacts", "destroy all enchantments", "destroy all nonland",
		"exile all creatures", "exile all permanents", "exile all nonland",
		"exile all cards", "exile all graveyards",
		"destroy each creature", "destroy each nonland", "destroy each permanent",
		"exile each creature", "exile each nonland", "exile each permanent",
		"all creatures get -", "deals damage to each creature",
		"deals damage to each opponent") {
		p.IsMassWipe = true
		p.IsRemoval = true
	}

	// Check if triggers are mandatory or optional.
	// "you may" in oracle text near a "whenever" = optional trigger.
	p.MandatoryTriggers = true // assume mandatory until proven otherwise
	if strings.Contains(ot, "you may") {
		p.MandatoryTriggers = false
	}
	// Activated abilities ("{cost}: effect") are always optional.
	if strings.Contains(ot, ": ") && (strings.Contains(ot, "{t}") || strings.Contains(ot, "sacrifice") || strings.Contains(ot, "pay")) {
		p.MandatoryTriggers = false
	}

	// ---------------------------------------------------------------
	// FLING / POWER-BASED FINISHER DETECTION
	// ---------------------------------------------------------------

	// Fling-type effects: deals damage equal to a creature's power
	if containsAny(ot, "deals damage equal to its power",
		"deals damage equal to the sacrificed creature's power",
		"deals damage equal to that creature's power",
		"each opponent loses life equal to the sacrificed creature's power",
		"each opponent loses life equal to its power",
		"deals damage equal to target creature's power") {
		p.IsFlingEffect = true
	}

	// Variable-power creatures: power scales with game state
	if strings.Contains(tl, "creature") {
		if containsAny(ot, "power and toughness are each equal to",
			"power is equal to", "gets +1/+1 for each",
			"has base power and toughness") {
			p.HasVariablePower = true
		}
	}

	// ---------------------------------------------------------------
	// FALSE POSITIVE AWARENESS FLAGS
	// ---------------------------------------------------------------

	// Self-exile on death: "when ~ dies, exile it" breaks graveyard recursion loops.
	// Use ~ replacement: oracle text uses the card's own name or "it" in death triggers.
	if (strings.Contains(ot, "when") && strings.Contains(ot, "dies")) ||
		(strings.Contains(ot, "when") && strings.Contains(ot, "is put into a graveyard")) {
		if containsAny(ot, "exile it", "exile "+strings.ToLower(name), "exile ~") {
			p.SelfExilesOnDeath = true
		}
	}

	// Recursion destination: where does the graveyard return go?
	if p.IsRecursion {
		if strings.Contains(ot, "return") && strings.Contains(ot, "graveyard") {
			if strings.Contains(ot, "to your hand") || strings.Contains(ot, "to its owner's hand") ||
				strings.Contains(ot, "to hand") {
				p.RecursionDest = "hand"
			} else if strings.Contains(ot, "to the battlefield") || strings.Contains(ot, "onto the battlefield") {
				p.RecursionDest = "battlefield"
			} else if strings.Contains(ot, "on top of") || strings.Contains(ot, "top of your library") ||
				strings.Contains(ot, "top of its owner's library") {
				p.RecursionDest = "top"
			}
		}
	}

	// Combat-dependent triggers: require attacking or combat damage.
	if containsAny(ot, "whenever "+strings.ToLower(name)+" attacks",
		"whenever ~ attacks", "whenever this creature attacks") {
		p.RequiresCombat = true
	}
	if strings.Contains(ot, "whenever") && (strings.Contains(ot, "attacks") || strings.Contains(ot, "you attack")) &&
		!strings.Contains(ot, "whenever a creature") && !strings.Contains(ot, "whenever another") {
		p.RequiresCombat = true
	}
	if containsAny(ot, "whenever "+strings.ToLower(name)+" deals combat damage",
		"whenever ~ deals combat damage", "whenever this creature deals combat damage") {
		p.RequiresCombat = true
	}
	if containsAny(ot, "at the beginning of combat") {
		p.RequiresCombat = true
	}

	// Random selection: "at random" in oracle text.
	if strings.Contains(ot, "at random") {
		p.HasRandomSelection = true
	}

	// ---------------------------------------------------------------
	// ARCHETYPE 1: TRIBAL / CREATURE TYPE SYNERGIES
	// ---------------------------------------------------------------
	p.CreatureTypes = extractCreatureTypes(ot, tl)

	// Lord detection: "<type> you control get/have/gain"
	tribalTypes := []string{
		"zombie", "elf", "goblin", "wizard", "warrior", "dragon", "angel",
		"vampire", "merfolk", "soldier", "knight", "beast", "elemental",
		"rogue", "cleric", "shaman", "spirit", "dinosaur", "human", "faerie",
		"ninja", "werewolf", "wolf", "sliver", "pirate", "demon", "cat",
		"bird", "rat", "insect", "fungus", "treefolk", "horror", "wraith",
		"nazgul", "orc", "halfling", "dwarf", "phyrexian",
	}
	for _, t := range tribalTypes {
		// Plural forms
		plural := t + "s"
		if strings.HasSuffix(t, "f") {
			// elf -> elves, dwarf -> dwarves, werewolf -> werewolves
			plural = t[:len(t)-1] + "ves"
		}
		// Check "X you control get/have/gain"
		if strings.Contains(ot, plural+" you control get") ||
			strings.Contains(ot, plural+" you control have") ||
			strings.Contains(ot, plural+" you control gain") ||
			strings.Contains(ot, t+" creatures you control get") ||
			strings.Contains(ot, t+" creatures you control have") ||
			strings.Contains(ot, t+" creatures you control gain") {
			p.IsTribalLord = true
			break
		}
		// Also catch "other X get" pattern (e.g. "other Zombies you control")
		if strings.Contains(ot, "other "+plural) && containsAny(ot, "get +", "have ", "gain ") {
			p.IsTribalLord = true
			break
		}
	}

	// Payoff detection: "whenever a/an/another <type>"
	for _, t := range tribalTypes {
		if strings.Contains(ot, "whenever a "+t) ||
			strings.Contains(ot, "whenever an "+t) ||
			strings.Contains(ot, "whenever another "+t) ||
			strings.Contains(ot, "each "+t) && (strings.Contains(ot, "deals") || strings.Contains(ot, "gets") || strings.Contains(ot, "enters")) {
			p.IsTribalPayoff = true
			break
		}
	}

	// ---------------------------------------------------------------
	// ARCHETYPE 2: +1/+1 COUNTER SYNERGIES
	// ---------------------------------------------------------------
	if strings.Contains(ot, "+1/+1 counter") {
		if strings.Contains(ot, "put") || strings.Contains(ot, "enter") {
			p.Effects = append(p.Effects, "counter_add")
			p.Produces = append(p.Produces, ResCounter)
		}
		if strings.Contains(ot, "remove") || strings.Contains(ot, "move") {
			p.Effects = append(p.Effects, "counter_move")
		}
	}
	if strings.Contains(ot, "proliferate") {
		p.Effects = append(p.Effects, "proliferate")
		p.Produces = append(p.Produces, ResCounter)
	}
	// Cards that care about counters being present
	if strings.Contains(ot, "with a +1/+1 counter") || strings.Contains(ot, "has a +1/+1 counter") ||
		strings.Contains(ot, "number of +1/+1 counters") || strings.Contains(ot, "for each +1/+1 counter") ||
		strings.Contains(ot, "with +1/+1 counters") || strings.Contains(ot, "have +1/+1 counters") {
		p.Triggers = append(p.Triggers, "counter_matters")
	}

	// ---------------------------------------------------------------
	// ARCHETYPE 3: DAY/NIGHT / TRANSFORM
	// ---------------------------------------------------------------
	if strings.Contains(ot, "daybound") || strings.Contains(ot, "nightbound") {
		p.Triggers = append(p.Triggers, "daynight")
		p.Effects = append(p.Effects, "transform")
	}
	if strings.Contains(ot, "transform") || strings.Contains(ot, "transforms") {
		p.Effects = append(p.Effects, "transform")
	}
	if strings.Contains(ot, "it becomes night") || strings.Contains(ot, "it becomes day") ||
		strings.Contains(ot, "becomes night") || strings.Contains(ot, "becomes day") {
		p.Effects = append(p.Effects, "daynight_toggle")
	}

	// ---------------------------------------------------------------
	// ARCHETYPE 4: EXILE MATTERS
	// ---------------------------------------------------------------
	if strings.Contains(ot, "exile") && (strings.Contains(ot, "you may cast") || strings.Contains(ot, "you may play")) {
		p.Produces = append(p.Produces, ResCard)
		p.Effects = append(p.Effects, "exile_cast")
	}
	if strings.Contains(ot, "exiled") && strings.Contains(ot, "return") {
		p.Effects = append(p.Effects, "exile_return")
	}
	// Impulsive draw (exile top, may play this turn)
	if strings.Contains(ot, "exile the top") && (strings.Contains(ot, "may play") || strings.Contains(ot, "may cast")) {
		p.Effects = append(p.Effects, "impulse_draw")
		p.Produces = append(p.Produces, ResCard)
	}

	// ---------------------------------------------------------------
	// ARCHETYPE 5: FACE-DOWN MATTERS
	// ---------------------------------------------------------------
	// Distinguish between:
	// - "facedown_create": card itself enters face-down (morph/disguise)
	// - "facedown_enabler": puts OTHER cards face-down (manifest, Scroll of Fate)
	// - "facedown" trigger: card cares about OTHER face-down creatures existing
	// - "face_up" effect: card rewards things being turned face up (not just self)
	hasMorphKeyword := strings.Contains(ot, "morph") || strings.Contains(ot, "megamorph") ||
		strings.Contains(ot, "disguise")

	if hasMorphKeyword {
		p.Effects = append(p.Effects, "facedown_create")
	}

	// Enablers: cards that put OTHER things face-down (manifest, Scroll of Fate, etc)
	if strings.Contains(ot, "manifest") && !strings.Contains(ot, "manifested") {
		p.Effects = append(p.Effects, "facedown_enabler")
		p.Effects = append(p.Effects, "facedown_create")
	}

	// Face-down payoffs: cards that specifically care about face-down creatures
	// existing on the battlefield (not just their own morph ability).
	// Pattern: "face-down creatures you control" or "whenever a face-down creature"
	// or "face-down permanents" -- these are the REAL payoffs.
	if containsAny(ot, "face-down creature", "face down creature",
		"face-down permanent", "face down permanent") {
		p.Triggers = append(p.Triggers, "facedown")
	}
	// "whenever" + face-down/face up referencing other creatures
	if strings.Contains(ot, "whenever") &&
		(strings.Contains(ot, "turned face up") || strings.Contains(ot, "turns face up")) &&
		!hasMorphKeyword {
		// This card triggers when OTHER things flip (e.g., Trail of Mystery)
		p.Triggers = append(p.Triggers, "facedown")
		p.Effects = append(p.Effects, "face_up")
	}
	// Cards with morph that ALSO have a "when turned face up" are just morph creatures --
	// they have face_up as a self-trigger but are NOT face-down payoffs.
	if hasMorphKeyword && (strings.Contains(ot, "turned face up") || strings.Contains(ot, "turns face up")) {
		p.Effects = append(p.Effects, "face_up")
	}

	// ---------------------------------------------------------------
	// ARCHETYPE 6: TOP-OF-LIBRARY MATTERS
	// ---------------------------------------------------------------
	if strings.Contains(ot, "top of") && strings.Contains(ot, "library") {
		if strings.Contains(ot, "reveal") || strings.Contains(ot, "look at") {
			p.Effects = append(p.Effects, "topdeck_reveal")
		}
		if strings.Contains(ot, "put") && (strings.Contains(ot, "on top") || strings.Contains(ot, "top of your library")) {
			p.Effects = append(p.Effects, "topdeck_manipulate")
		}
	}
	if strings.Contains(ot, "scry") || strings.Contains(ot, "surveil") {
		p.Effects = append(p.Effects, "topdeck_manipulate")
	}

	// ---------------------------------------------------------------
	// ARCHETYPE 7: STAX/TAX
	// ---------------------------------------------------------------
	if strings.Contains(ot, "costs") && (strings.Contains(ot, "more to cast") ||
		strings.Contains(ot, "{1} more") || strings.Contains(ot, "{2} more") ||
		strings.Contains(ot, "{3} more") || strings.Contains(ot, "more to activate")) {
		p.Effects = append(p.Effects, "tax")
	}
	if strings.Contains(ot, "can't cast") || strings.Contains(ot, "can't be cast") ||
		strings.Contains(ot, "can't play") {
		p.Effects = append(p.Effects, "lock")
	}
	if strings.Contains(ot, "each opponent") && (strings.Contains(ot, "loses") ||
		strings.Contains(ot, "discards") || strings.Contains(ot, "sacrifices")) {
		p.Effects = append(p.Effects, "symmetric_pain")
		p.Triggers = append(p.Triggers, "opponent_pain")
	}

	// ---------------------------------------------------------------
	// ARCHETYPE 8: BLINK / FLICKER
	// ---------------------------------------------------------------

	// Blink/Flicker detection
	// True blink exiles a permanent and returns it (or another) to battlefield in one effect.
	// Exclude: unearth (self-return from GY), death triggers that exile self and return others
	// from GY, hideaway (exile from library), self-sacrifice-to-return patterns.
	isSelfReturn := strings.Contains(ot, "return this") || strings.Contains(ot, "return it from your graveyard") ||
		strings.Contains(ot, "return "+strings.ToLower(name))
	isFromGraveyard := strings.Contains(ot, "from your graveyard") || strings.Contains(ot, "from a graveyard")
	isHideaway := strings.Contains(ot, "hideaway")
	isUnearth := strings.Contains(ot, "unearth")
	isDiesExile := strings.Contains(ot, "dies") && strings.Contains(ot, "exile it")
	selfSacToReturn := strings.Contains(ot, "sacrifice") && strings.Contains(ot, "return") &&
		(strings.Contains(ot, "sacrifice "+strings.ToLower(name)) ||
			strings.Contains(ot, "sacrifice this") ||
			strings.Contains(ot, "sacrifice ~"))
	blinkExcluded := selfSacToReturn || isSelfReturn || isFromGraveyard || isHideaway || isUnearth || isDiesExile
	p.IsBlinker = false
	if !blinkExcluded {
		// Specific blink patterns (high confidence)
		if strings.Contains(ot, "flicker") || strings.Contains(ot, "exile, then return") ||
			(strings.Contains(ot, "exile target") && strings.Contains(ot, "return it")) ||
			(strings.Contains(ot, "exile up to one") && strings.Contains(ot, "return")) ||
			(strings.Contains(ot, "exile another") && strings.Contains(ot, "return")) {
			p.IsBlinker = true
			p.Effects = append(p.Effects, "blink")
		}
		// General pattern: only if not excluded and exile+return are clearly about the same permanent
		if !p.IsBlinker && strings.Contains(ot, "exile") && strings.Contains(ot, "return") &&
			(strings.Contains(ot, "return it") || strings.Contains(ot, "return that card") ||
				strings.Contains(ot, "return the exiled") || strings.Contains(ot, "returns it")) {
			p.IsBlinker = true
			p.Effects = append(p.Effects, "blink")
		}
	}

	// ETB value detection -- cards whose ETB produces something worth blinking
	p.HasValueETB = false
	if strings.Contains(ot, "when") && strings.Contains(ot, "enters") {
		exileIsGYHate := strings.Contains(ot, "exile target") &&
			strings.Contains(ot, "graveyard") &&
			!strings.Contains(ot, "exile target creature") &&
			!strings.Contains(ot, "exile target permanent") &&
			!strings.Contains(ot, "exile target artifact") &&
			!strings.Contains(ot, "exile target enchantment")
		if strings.Contains(ot, "add {") || strings.Contains(ot, "add one mana") ||
			strings.Contains(ot, "untap") || strings.Contains(ot, "create") ||
			strings.Contains(ot, "draw") || strings.Contains(ot, "destroy") ||
			(strings.Contains(ot, "exile target") && !exileIsGYHate) ||
			strings.Contains(ot, "search your library") ||
			strings.Contains(ot, "each opponent loses") || strings.Contains(ot, "deals") {
			p.HasValueETB = true
		}
	}

	// Mana-producing ETB specifically (strongest blink combo potential)
	// Split oracle into sentences to check if "add {" is in the ETB clause, not a tap ability.
	p.HasManaETB = false
	if strings.Contains(ot, "when") && strings.Contains(ot, "enters") {
		etbClause := ""
		for _, sentence := range strings.Split(ot, "\n") {
			if strings.Contains(sentence, "when") && strings.Contains(sentence, "enters") {
				etbClause = sentence
				break
			}
		}
		if strings.Contains(etbClause, "add {") || strings.Contains(etbClause, "add one mana") ||
			strings.Contains(etbClause, "create a treasure") || strings.Contains(etbClause, "create treasure") ||
			(strings.Contains(etbClause, "untap") && strings.Contains(etbClause, "land")) {
			p.HasManaETB = true
		}
	}

	// Deduplicate resource slices.
	p.Produces = uniqueResources(p.Produces)
	p.Consumes = uniqueResources(p.Consumes)
	p.Triggers = uniqueStrings(p.Triggers)
	p.Effects = uniqueStrings(p.Effects)

	// Zone flow classification for value chain detection.
	p.ZoneFlows = classifyZoneFlows(ot, tl, &p)

	return p
}

// ---------------------------------------------------------------------------
// AnalyzeDeck -- run the full analysis pipeline on a set of card profiles.
// ---------------------------------------------------------------------------

func AnalyzeDeck(profiles []CardProfile, deckName, deckPath, commander string) *FreyaReport {
	report := &FreyaReport{
		DeckName:    deckName,
		DeckPath:    deckPath,
		Commander:   commander,
		TotalCards:  len(profiles),
		Profiles:    profiles,
		ColorDemand: make(map[string]int),
		ColorSupply: make(map[string]int),
	}

	// Count utility cards and compute mana curve.
	for _, p := range profiles {
		if p.IsTutor {
			report.TutorCount++
		}
		if p.IsRemoval {
			report.RemovalCount++
		}
		if p.IsOutlet {
			report.OutletCount++
		}
		if p.IsWinCon {
			report.WinConCount++
		}

		// Mana curve and color analysis.
		if p.IsLand {
			report.LandCount++
			countLandColors(p, report.ColorSupply)
		} else {
			report.NonlandCount++
			cmc := p.CMC
			if cmc >= 7 {
				report.ManaCurve[7]++
			} else if cmc >= 0 {
				report.ManaCurve[cmc]++
			}
			countManaCostPips(p.ManaCost, report.ColorDemand)
		}
	}

	// Average CMC (excluding lands).
	totalCMC := 0
	for i, count := range report.ManaCurve {
		if i == 7 {
			totalCMC += count * 7 // approximate 7+ as 7
		} else {
			totalCMC += count * i
		}
	}
	if report.NonlandCount > 0 {
		report.AvgCMC = float64(totalCMC) / float64(report.NonlandCount)
	}

	// Curve shape classification.
	peak := 0
	peakCMC := 0
	for i, count := range report.ManaCurve {
		if count > peak {
			peak = count
			peakCMC = i
		}
	}
	switch {
	case peakCMC <= 2 && report.AvgCMC < 2.5:
		report.CurveShape = "aggro"
	case peakCMC <= 3 && report.AvgCMC < 3.5:
		report.CurveShape = "midrange"
	case report.AvgCMC >= 3.5:
		report.CurveShape = "control"
	default:
		report.CurveShape = "midrange"
	}

	// Color mismatch detection.
	totalDemand := 0
	totalSupply := 0
	for _, v := range report.ColorDemand {
		totalDemand += v
	}
	for _, v := range report.ColorSupply {
		totalSupply += v
	}
	if totalDemand > 0 && totalSupply > 0 {
		for _, color := range []string{"W", "U", "B", "R", "G"} {
			demandPct := float64(report.ColorDemand[color]) / float64(totalDemand) * 100
			supplyPct := float64(report.ColorSupply[color]) / float64(totalSupply) * 100
			if demandPct-supplyPct > 5 {
				report.ColorMismatch = append(report.ColorMismatch,
					fmt.Sprintf("%s underrepresented: %.0f%% demand vs %.0f%% supply", color, demandPct, supplyPct))
			}
		}
	}

	// ── Check known combos first (100% confidence) ──
	deckCardNames := map[string]bool{}
	for _, p := range profiles {
		deckCardNames[p.Name] = true
	}

	knownComboKeys := map[string]bool{} // track confirmed combos to avoid heuristic dupes
	for _, known := range KnownCombos {
		allPresent := true
		for _, piece := range known.Pieces {
			if !deckCardNames[piece] {
				allPresent = false
				break
			}
		}
		if !allPresent {
			continue
		}

		combo := ComboResult{
			Cards:       known.Pieces,
			LoopType:    known.Type,
			Description: known.Description,
			Confirmed:   true,
		}

		// Add outlet info
		var deckOutlets []string
		for _, outlet := range known.Outlets {
			if deckCardNames[outlet] {
				deckOutlets = append(deckOutlets, outlet)
			}
		}
		if len(deckOutlets) > 0 {
			combo.Description += " | OUTLETS IN DECK: " + strings.Join(deckOutlets, ", ")
		}

		switch known.Type {
		case "true_infinite":
			report.TrueInfinites = append(report.TrueInfinites, combo)
		case "synergy":
			report.Synergies = append(report.Synergies, combo)
		default:
			report.Determined = append(report.Determined, combo)
		}

		// Build a key for this combo to suppress heuristic duplicates.
		sorted := make([]string, len(known.Pieces))
		copy(sorted, known.Pieces)
		for si := 0; si < len(sorted); si++ {
			for sj := si + 1; sj < len(sorted); sj++ {
				if sorted[si] > sorted[sj] {
					sorted[si], sorted[sj] = sorted[sj], sorted[si]
				}
			}
		}
		knownComboKeys[strings.Join(sorted, "|")] = true
	}

	// ── Flag individual combo pieces (partial matches) ──
	for _, known := range KnownCombos {
		var presentPieces []string
		var missingPieces []string
		for _, piece := range known.Pieces {
			if deckCardNames[piece] {
				presentPieces = append(presentPieces, piece)
			} else {
				missingPieces = append(missingPieces, piece)
			}
		}
		// If we have SOME but not ALL pieces, note the potential.
		if len(presentPieces) > 0 && len(missingPieces) > 0 {
			report.ComboNotes = append(report.ComboNotes, fmt.Sprintf(
				"%s: have %s, missing %s for %s",
				known.Name, strings.Join(presentPieces, " + "),
				strings.Join(missingPieces, " + "), known.Type))
		}
	}

	// Run all detectors.
	loops := FindLoops(profiles)
	for _, combo := range loops {
		// Skip if already confirmed from known database.
		sorted := make([]string, len(combo.Cards))
		copy(sorted, combo.Cards)
		for si := 0; si < len(sorted); si++ {
			for sj := si + 1; sj < len(sorted); sj++ {
				if sorted[si] > sorted[sj] {
					sorted[si], sorted[sj] = sorted[sj], sorted[si]
				}
			}
		}
		if knownComboKeys[strings.Join(sorted, "|")] {
			continue
		}

		switch combo.LoopType {
		case "true_infinite":
			report.TrueInfinites = append(report.TrueInfinites, combo)
		case "determined":
			report.Determined = append(report.Determined, combo)
		default:
			report.Synergies = append(report.Synergies, combo)
		}
	}

	// Find outlets for each true infinite loop.
	for i, combo := range report.TrueInfinites {
		outlets := FindOutletsForInfinite(combo, profiles)
		if len(outlets) > 0 {
			report.TrueInfinites[i].Description += fmt.Sprintf(" | OUTLETS: %s",
				strings.Join(outletNames(outlets), ", "))
		} else {
			report.TrueInfinites[i].Description += " | NO OUTLET -- draws the game"
		}
	}

	finishers := FindFinishers(profiles)
	report.Finishers = append(report.Finishers, finishers...)

	synergies := FindSynergies(profiles)
	for _, combo := range synergies {
		sorted := make([]string, len(combo.Cards))
		copy(sorted, combo.Cards)
		for si := 0; si < len(sorted); si++ {
			for sj := si + 1; sj < len(sorted); sj++ {
				if sorted[si] > sorted[sj] {
					sorted[si], sorted[sj] = sorted[sj], sorted[si]
				}
			}
		}
		if knownComboKeys[strings.Join(sorted, "|")] {
			continue
		}
		switch combo.LoopType {
		case "true_infinite":
			report.TrueInfinites = append(report.TrueInfinites, combo)
		case "determined":
			report.Determined = append(report.Determined, combo)
		default:
			report.Synergies = append(report.Synergies, combo)
		}
	}

	// Deduplicate synergies by card-pair key.
	report.Synergies = deduplicateCombos(report.Synergies)

	// Value chain detection.
	report.ValueChains = DetectValueChains(profiles)

	return report
}

// ---------------------------------------------------------------------------
// Outlet detection -- for true infinites, find cards in the deck that can
// convert the infinite loop into a win condition.
// ---------------------------------------------------------------------------

// OutletInfo describes a card that can serve as an outlet for an infinite loop.
type OutletInfo struct {
	CardName   string
	OutletType string // "damage_on_etb", "drain_on_death", "sacrifice_outlet", etc.
	Converts   string // what the outlet converts the infinite into
}

// FindOutletsForInfinite scans all card profiles for compatible outlets
// that can convert a true infinite combo into a win condition.
func FindOutletsForInfinite(combo ComboResult, allProfiles []CardProfile) []OutletInfo {
	var outlets []OutletInfo

	// Determine what the infinite loop produces.
	producesETBs := false
	producesTokens := false

	comboCards := map[string]bool{}
	for _, c := range combo.Cards {
		comboCards[c] = true
	}

	for _, cardName := range combo.Cards {
		for _, p := range allProfiles {
			if p.Name == cardName {
				if containsRes(p.Produces, ResToken) {
					producesTokens = true
					producesETBs = true
				}
				for _, t := range p.Triggers {
					if t == "etb" {
						producesETBs = true
					}
				}
			}
		}
	}

	// If the loop description mentions triggers, infer ETB production.
	if strings.Contains(strings.ToLower(combo.Description), "trigger") {
		producesETBs = true
	}

	// Scan all other cards in deck for compatible outlets.
	for _, p := range allProfiles {
		if comboCards[p.Name] {
			continue // skip combo pieces themselves
		}

		// ETB outlets (Warstorm Surge, Purphoros, Impact Tremors, Terror of the Peaks).
		if producesETBs {
			for _, t := range p.Triggers {
				if t == "etb" {
					for _, e := range p.Effects {
						if e == "damage" || e == "drain" {
							outlets = append(outlets, OutletInfo{
								CardName:   p.Name,
								OutletType: "damage_on_etb",
								Converts:   "infinite ETBs -> infinite damage",
							})
							goto doneETB // one outlet type per card is enough
						}
					}
				}
			}
		}
	doneETB:

		// Sacrifice outlets (can break the loop + get value).
		if producesTokens && p.IsOutlet {
			// Check if the outlet also has a damage/drain effect.
			hasDmg := false
			for _, e := range p.Effects {
				if e == "damage" || e == "drain" {
					hasDmg = true
				}
			}
			if hasDmg {
				outlets = append(outlets, OutletInfo{
					CardName:   p.Name,
					OutletType: "sacrifice_outlet_damage",
					Converts:   "infinite tokens -> sacrifice -> infinite damage",
				})
			} else {
				outlets = append(outlets, OutletInfo{
					CardName:   p.Name,
					OutletType: "sacrifice_outlet",
					Converts:   "infinite tokens -> sacrifice value",
				})
			}
		}

		// Death triggers with tokens.
		if producesTokens && !comboCards[p.Name] {
			for _, t := range p.Triggers {
				if t == "dies" || t == "sacrifice" {
					for _, e := range p.Effects {
						if e == "damage" || e == "drain" {
							outlets = append(outlets, OutletInfo{
								CardName:   p.Name,
								OutletType: "death_trigger_damage",
								Converts:   "infinite tokens -> sacrifice -> infinite damage",
							})
							goto doneDeath
						}
					}
				}
			}
		}
	doneDeath:
	}

	// Deduplicate outlets by card name.
	seen := map[string]bool{}
	var deduped []OutletInfo
	for _, o := range outlets {
		if !seen[o.CardName] {
			seen[o.CardName] = true
			deduped = append(deduped, o)
		}
	}

	return deduped
}

// outletNames extracts card names from a slice of OutletInfo.
func outletNames(outlets []OutletInfo) []string {
	names := make([]string, len(outlets))
	for i, o := range outlets {
		names[i] = fmt.Sprintf("%s (%s)", o.CardName, o.Converts)
	}
	return names
}

// ---------------------------------------------------------------------------
// FindLoops -- detect cycles in the resource graph.
//
// A real combo loop requires ACTIVE resource flow: card A actively produces
// resource X which card B actively consumes, and card B actively produces
// resource Y which card A actively consumes. This creates a repeatable
// cycle.
// ---------------------------------------------------------------------------

func FindLoops(profiles []CardProfile) []ComboResult {
	var results []ComboResult

	// Check all pairs.
	for i := 0; i < len(profiles); i++ {
		for j := i + 1; j < len(profiles); j++ {
			if combo := checkPairCombo(profiles[i], profiles[j]); combo != nil {
				results = append(results, *combo)
			}
		}
	}

	// Check all triples -- only if the deck is small enough (< 120 cards)
	// to keep runtime reasonable. For a 100-card EDH deck: C(100,3) = 161,700.
	if len(profiles) <= 120 {
		for i := 0; i < len(profiles); i++ {
			for j := i + 1; j < len(profiles); j++ {
				for k := j + 1; k < len(profiles); k++ {
					if combo := checkTripleCombo(profiles[i], profiles[j], profiles[k]); combo != nil {
						results = append(results, *combo)
					}
				}
			}
		}
	}

	return results
}

func checkPairCombo(a, b CardProfile) *ComboResult {
	// A produces what B consumes AND B produces what A consumes = loop.
	aProducesForB := resourceOverlap(a.Produces, b.Consumes)
	bProducesForA := resourceOverlap(b.Produces, a.Consumes)

	if len(aProducesForB) > 0 && len(bProducesForA) > 0 {
		// Filter out trivial overlaps (e.g. both produce/consume life
		// but in different contexts). Require at least one "interesting"
		// resource in the loop.
		if isInterestingLoop(aProducesForB) || isInterestingLoop(bProducesForA) {
			// Verify trigger chain — does B's trigger actually fire from A's production?
			if !verifyTriggerChain(a, b) && !verifyTriggerChain(b, a) {
				// Resource overlap exists but triggers don't chain — downgrade to synergy.
				return &ComboResult{
					Cards:    []string{a.Name, b.Name},
					LoopType: "synergy",
					Resources: fmt.Sprintf("%v <-> %v",
						resourceNames(aProducesForB), resourceNames(bProducesForA)),
					Description: fmt.Sprintf("%s and %s share resources but triggers don't chain",
						a.Name, b.Name),
				}
			}

			loopType := classifyLoop(a, b)
			combo := &ComboResult{
				Cards:    []string{a.Name, b.Name},
				LoopType: loopType,
				Resources: fmt.Sprintf("%v <-> %v",
					resourceNames(aProducesForB), resourceNames(bProducesForA)),
				Description: fmt.Sprintf("%s produces %v for %s, %s produces %v back",
					a.Name, resourceNames(aProducesForB),
					b.Name, b.Name, resourceNames(bProducesForA)),
			}
			if a.HasRandomSelection || b.HasRandomSelection {
				combo.NonDeterministic = true
			}
			return combo
		}
	}

	// Check for mutual trigger loops even without explicit resource flow.
	// If A triggers on B's effect AND B triggers on A's effect, that's
	// potentially infinite.
	if hasMutualTriggerLoop(a, b) {
		loopType := classifyLoop(a, b)
		combo := &ComboResult{
			Cards:     []string{a.Name, b.Name},
			LoopType:  loopType,
			Resources: "trigger loop",
			Description: fmt.Sprintf("%s and %s trigger each other in a loop",
				a.Name, b.Name),
		}
		if a.HasRandomSelection || b.HasRandomSelection {
			combo.NonDeterministic = true
		}
		return combo
	}

	return nil
}

// verifyTriggerChain checks whether the consumer card's triggers would
// actually fire from the producer card's output. This prevents false
// positives where cards share resource types but don't actually chain.
func verifyTriggerChain(producer, consumer CardProfile) bool {
	// Does the consumer have a trigger that fires from the producer's effects?
	for _, prodEffect := range producer.Effects {
		for _, consTrigger := range consumer.Triggers {
			if triggerMatchesEffect(consTrigger, prodEffect) {
				return true
			}
		}
	}
	// Also check: does producer create something that consumer consumes?
	for _, res := range producer.Produces {
		for _, cons := range consumer.Consumes {
			if res == cons {
				// Check that the consumer's trigger fires from this resource arrival.
				if res == ResToken {
					for _, t := range consumer.Triggers {
						if t == "etb" || t == "cast" || t == "sacrifice" || t == "dies" {
							return true
						}
					}
					// If consumer is an outlet that sacrifices, that's a valid chain.
					if consumer.IsOutlet {
						return true
					}
				}
				if res == ResCard {
					for _, t := range consumer.Triggers {
						if t == "draw" || t == "discard" {
							return true
						}
					}
					// If consumer has discard as a cost, producing cards feeds it.
					return true
				}
				if res == ResMana {
					return true // mana loops are always real if resources cycle
				}
				if res == ResGraveyard {
					return true // graveyard recursion loops are always real
				}
				if res == ResUntap {
					return true // untap loops are always real
				}
				if res == ResCounter {
					return true // counter loops are real (e.g. Walking Ballista + Heliod)
				}
				if res == ResLand {
					// Land loops are only real if at least one card has recursion
					// (e.g. Splendid Reclamation + Scapeshift). Two one-shot fetch
					// effects don't create actual loops.
					if producer.IsRecursion || consumer.IsRecursion {
						return true
					}
				}
				if res == ResGraveyardFill {
					return true // graveyard-fill loops are real
				}
				if res == ResReanimate {
					return true // reanimate resource loops are real
				}
			}
		}
	}
	return false
}

// classifyLoop determines the loop type for a set of cards.
//
// Classification:
//
//	"true_infinite" = all triggers are mandatory AND no kill condition in the loop.
//	                  This is an unbreakable loop that draws the game without an outlet.
//	"determined"    = the loop has a KILL CONDITION (damage, drain, mill) OR
//	                  triggers are optional (player chooses how many iterations).
//	"synergy"       = resource interaction but not self-sustaining enough for infinite.
func classifyLoop(cards ...CardProfile) string {
	hasKillOutput := false
	allMandatory := true
	producesInfiniteResource := false

	for i := 0; i < len(cards); i++ {
		for j := i + 1; j < len(cards); j++ {
			if hasDamageOutput(cards[i], cards[j]) ||
				hasDrainOutput(cards[i], cards[j]) ||
				hasMillOutput(cards[i], cards[j]) {
				hasKillOutput = true
			}
		}
		if !cards[i].MandatoryTriggers {
			allMandatory = false
		}
		if containsRes(cards[i].Produces, ResMana) ||
			containsRes(cards[i].Produces, ResToken) ||
			containsRes(cards[i].Produces, ResCard) ||
			containsRes(cards[i].Produces, ResUntap) {
			producesInfiniteResource = true
		}
	}

	// False positive checks: these conditions break the loop assumption.
	loopUsesGraveyard := false
	for _, c := range cards {
		if containsRes(c.Produces, ResGraveyard) || containsRes(c.Produces, ResReanimate) ||
			c.IsRecursion {
			loopUsesGraveyard = true
			break
		}
	}

	// Self-exile on death: card removes itself from graveyard, breaking recursion.
	if loopUsesGraveyard {
		for _, c := range cards {
			if c.SelfExilesOnDeath {
				return "synergy"
			}
		}
	}

	// Hand-only recursion: all recursion pieces return to hand, requiring recast.
	if loopUsesGraveyard {
		allToHand := true
		hasAnyRecursion := false
		for _, c := range cards {
			if c.IsRecursion {
				hasAnyRecursion = true
				if c.RecursionDest != "hand" {
					allToHand = false
				}
			}
		}
		if hasAnyRecursion && allToHand {
			return "synergy"
		}
	}

	// Combat-dependent: any piece requires attacking, capping at 1 iteration per turn.
	for _, c := range cards {
		if c.RequiresCombat {
			return "synergy"
		}
	}

	raw := "synergy"
	if allMandatory && !hasKillOutput {
		raw = "true_infinite" // mandatory loop, no kill = draws the game without outlet
	} else if allMandatory && hasKillOutput {
		raw = "determined" // mandatory but has kill -- terminates at opponent death
	} else if hasKillOutput {
		raw = "determined" // has a kill condition, player controls termination
	} else if producesInfiniteResource {
		if allMandatory {
			raw = "true_infinite"
		} else {
			raw = "determined" // optional = player chooses how many times
		}
	}

	return raw
}

// hasMutualTriggerLoop checks if A triggers on B's effects AND B triggers
// on A's effects, creating a potentially infinite trigger chain.
func hasMutualTriggerLoop(a, b CardProfile) bool {
	aTrigFromB := false
	bTrigFromA := false
	for _, trig := range a.Triggers {
		for _, eff := range b.Effects {
			if triggerMatchesEffect(trig, eff) {
				aTrigFromB = true
			}
		}
	}
	for _, trig := range b.Triggers {
		for _, eff := range a.Effects {
			if triggerMatchesEffect(trig, eff) {
				bTrigFromA = true
			}
		}
	}
	return aTrigFromB && bTrigFromA
}

func checkTripleCombo(a, b, c CardProfile) *ComboResult {
	// Try all 3 cycle orderings: A->B->C->A, A->C->B->A.
	if combo := tryTripleCycle(a, b, c); combo != nil {
		return combo
	}
	if combo := tryTripleCycle(a, c, b); combo != nil {
		return combo
	}
	return nil
}

func tryTripleCycle(a, b, c CardProfile) *ComboResult {
	abFlow := resourceOverlap(a.Produces, b.Consumes)
	bcFlow := resourceOverlap(b.Produces, c.Consumes)
	caFlow := resourceOverlap(c.Produces, a.Consumes)

	if len(abFlow) > 0 && len(bcFlow) > 0 && len(caFlow) > 0 {
		if !isInterestingLoop(abFlow) && !isInterestingLoop(bcFlow) && !isInterestingLoop(caFlow) {
			return nil
		}

		loopType := classifyLoop(a, b, c)

		combo := &ComboResult{
			Cards:    []string{a.Name, b.Name, c.Name},
			LoopType: loopType,
			Resources: fmt.Sprintf("%v -> %v -> %v -> loop",
				resourceNames(abFlow), resourceNames(bcFlow), resourceNames(caFlow)),
			Description: fmt.Sprintf("%s -> %s -> %s -> loop (%v -> %v -> %v)",
				a.Name, b.Name, c.Name,
				resourceNames(abFlow), resourceNames(bcFlow), resourceNames(caFlow)),
		}
		if a.HasRandomSelection || b.HasRandomSelection || c.HasRandomSelection {
			combo.NonDeterministic = true
		}
		return combo
	}

	return nil
}

// isInterestingLoop returns true if a resource overlap contains something
// more specific than just life or counters (which are very common and noisy).
func isInterestingLoop(rs []ResourceType) bool {
	for _, r := range rs {
		switch r {
		case ResMana, ResToken, ResCard, ResGraveyard, ResUntap, ResLand, ResGraveyardFill, ResReanimate:
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// FindFinishers -- detect game-ending effects.
// ---------------------------------------------------------------------------

func FindFinishers(profiles []CardProfile) []ComboResult {
	var results []ComboResult
	for _, p := range profiles {
		if p.IsWinCon {
			results = append(results, ComboResult{
				Cards:       []string{p.Name},
				LoopType:    "finisher",
				Description: "Direct win condition",
			})
		}
		if p.IsMassWipe && !p.IsWinCon {
			results = append(results, ComboResult{
				Cards:       []string{p.Name},
				LoopType:    "finisher",
				Description: "Board wipe / mass removal",
			})
		}
	}

	// Fling + big creature / variable power = lethal finisher combo
	// Prioritize variable-power creatures (Lord of Extinction, Consuming Aberration) and the commander.
	// For static-power creatures, only flag the top 3 by CMC to avoid noise.
	var flings []CardProfile
	var varPowerThreats []CardProfile
	var staticThreats []CardProfile
	for _, p := range profiles {
		if p.IsFlingEffect {
			flings = append(flings, p)
		}
		if !strings.Contains(strings.ToLower(p.TypeLine), "creature") {
			continue
		}
		if p.HasVariablePower {
			varPowerThreats = append(varPowerThreats, p)
		} else if p.CMC >= 7 {
			staticThreats = append(staticThreats, p)
		}
	}
	sort.Slice(staticThreats, func(i, j int) bool { return staticThreats[i].CMC > staticThreats[j].CMC })
	if len(staticThreats) > 3 {
		staticThreats = staticThreats[:3]
	}
	bigThreats := append(varPowerThreats, staticThreats...)
	for _, fling := range flings {
		for _, threat := range bigThreats {
			if fling.Name == threat.Name {
				continue
			}
			results = append(results, ComboResult{
				Cards:    []string{threat.Name, fling.Name},
				LoopType: "finisher",
				Description: fmt.Sprintf("%s + %s — power-based lethal damage",
					threat.Name, fling.Name),
			})
		}
	}

	// Mass reanimate as finisher (Living Death, Rise of the Dark Realms)
	for _, p := range profiles {
		ot := strings.ToLower(p.Name)
		_ = ot
		hasEffect := false
		for _, e := range p.Effects {
			if e == "mass_reanimate" {
				hasEffect = true
			}
		}
		if hasEffect && !p.IsWinCon {
			results = append(results, ComboResult{
				Cards:       []string{p.Name},
				LoopType:    "finisher",
				Description: fmt.Sprintf("%s — mass reanimation finisher", p.Name),
			})
		}
	}

	// ---------------------------------------------------------------
	// EXPANDED FINISHER CATEGORIES
	// ---------------------------------------------------------------

	// Collect enablers for cross-category combos
	var massPumps, hasteEnablers, extraCombats, damageDbs []CardProfile
	var xBurns, massTokenMakers, infectCards, stormCards, cmdThreats []CardProfile

	for _, p := range profiles {
		if p.IsMassPump {
			massPumps = append(massPumps, p)
		}
		if p.IsHasteEnabler {
			hasteEnablers = append(hasteEnablers, p)
		}
		if p.IsExtraCombat {
			extraCombats = append(extraCombats, p)
		}
		if p.IsDamageDoubler {
			damageDbs = append(damageDbs, p)
		}
		if p.IsXBurn {
			xBurns = append(xBurns, p)
		}
		if p.IsMassTokens {
			massTokenMakers = append(massTokenMakers, p)
		}
		if p.HasInfect {
			infectCards = append(infectCards, p)
		}
		if p.IsStormFinisher {
			stormCards = append(stormCards, p)
		}
		if p.IsCmdDamageThreat {
			cmdThreats = append(cmdThreats, p)
		}
	}

	// Solo finishers: cards that are finishers on their own
	for _, p := range massPumps {
		results = append(results, ComboResult{
			Cards:       []string{p.Name},
			LoopType:    "finisher",
			Description: fmt.Sprintf("%s — mass pump finisher", p.Name),
		})
	}
	for _, p := range xBurns {
		results = append(results, ComboResult{
			Cards:       []string{p.Name},
			LoopType:    "finisher",
			Description: fmt.Sprintf("%s — X-cost burn/drain finisher", p.Name),
		})
	}
	for _, p := range stormCards {
		results = append(results, ComboResult{
			Cards:       []string{p.Name},
			LoopType:    "finisher",
			Description: fmt.Sprintf("%s — storm finisher", p.Name),
		})
	}
	for _, p := range cmdThreats {
		results = append(results, ComboResult{
			Cards:       []string{p.Name},
			LoopType:    "finisher",
			Description: fmt.Sprintf("%s — commander damage threat", p.Name),
		})
	}
	for _, p := range infectCards {
		results = append(results, ComboResult{
			Cards:       []string{p.Name},
			LoopType:    "finisher",
			Description: fmt.Sprintf("%s — infect/poison finisher", p.Name),
		})
	}

	// Enabler finishers: cards that multiply existing board presence
	for _, p := range extraCombats {
		results = append(results, ComboResult{
			Cards:       []string{p.Name},
			LoopType:    "finisher",
			Description: fmt.Sprintf("%s — extra combat finisher", p.Name),
		})
	}
	for _, p := range damageDbs {
		results = append(results, ComboResult{
			Cards:       []string{p.Name},
			LoopType:    "finisher",
			Description: fmt.Sprintf("%s — damage doubler", p.Name),
		})
	}

	// Cross-category combos: mass tokens + pump/haste/damage doubler
	for _, tok := range massTokenMakers {
		for _, pump := range massPumps {
			if tok.Name == pump.Name {
				continue
			}
			results = append(results, ComboResult{
				Cards:    []string{tok.Name, pump.Name},
				LoopType: "finisher",
				Description: fmt.Sprintf("%s + %s — token army + mass pump",
					tok.Name, pump.Name),
			})
		}
		for _, haste := range hasteEnablers {
			if tok.Name == haste.Name {
				continue
			}
			results = append(results, ComboResult{
				Cards:    []string{tok.Name, haste.Name},
				LoopType: "finisher",
				Description: fmt.Sprintf("%s + %s — token army + haste",
					tok.Name, haste.Name),
			})
		}
	}

	// Extra combat + damage doubler = multiplicative damage
	for _, ec := range extraCombats {
		for _, dd := range damageDbs {
			if ec.Name == dd.Name {
				continue
			}
			results = append(results, ComboResult{
				Cards:    []string{ec.Name, dd.Name},
				LoopType: "finisher",
				Description: fmt.Sprintf("%s + %s — extra combat × doubled damage",
					ec.Name, dd.Name),
			})
		}
	}

	// Haste + extra combat = immediate multi-swing
	for _, haste := range hasteEnablers {
		for _, ec := range extraCombats {
			if haste.Name == ec.Name {
				continue
			}
			results = append(results, ComboResult{
				Cards:    []string{haste.Name, ec.Name},
				LoopType: "finisher",
				Description: fmt.Sprintf("%s + %s — haste into extra combats",
					haste.Name, ec.Name),
			})
		}
	}

	return results
}

// ---------------------------------------------------------------------------
// FindSynergies -- detect strong 2-card trigger interactions.
// ---------------------------------------------------------------------------

func FindSynergies(profiles []CardProfile) []ComboResult {
	var results []ComboResult

	// ---------------------------------------------------------------
	// Special synergy patterns: land-matters and graveyard-matters
	// These run FIRST so they get priority over generic trigger matching
	// during deduplication (better descriptions).
	// ---------------------------------------------------------------
	for i := 0; i < len(profiles); i++ {
		for j := i + 1; j < len(profiles); j++ {
			a, b := profiles[i], profiles[j]

			// Self-mill + mass reanimate = POWER SYNERGY
			// Require that one card has self_mill WITHOUT mass_reanimate and the
			// other has mass_reanimate, to avoid pairing two self-millers.
			if profileHasEffect(a, "self_mill") && profileHasEffect(b, "mass_reanimate") &&
				!(profileHasEffect(a, "mass_reanimate") && profileHasEffect(b, "self_mill")) {
				results = append(results, ComboResult{
					Cards:       []string{a.Name, b.Name},
					LoopType:    "synergy",
					Description: fmt.Sprintf("%s fills graveyard, %s reanimates en masse — graveyard engine", a.Name, b.Name),
				})
			} else if profileHasEffect(b, "self_mill") && profileHasEffect(a, "mass_reanimate") &&
				!(profileHasEffect(b, "mass_reanimate") && profileHasEffect(a, "self_mill")) {
				results = append(results, ComboResult{
					Cards:       []string{a.Name, b.Name},
					LoopType:    "synergy",
					Description: fmt.Sprintf("%s fills graveyard, %s reanimates en masse — graveyard engine", b.Name, a.Name),
				})
			}

			// Land sacrifice + land reanimate = repeatable value
			if (profileHasEffect(a, "land_sacrifice") && profileHasEffect(b, "land_reanimate")) ||
				(profileHasEffect(b, "land_sacrifice") && profileHasEffect(a, "land_reanimate")) {
				sac, rec := pickByEffect(a, b, "land_sacrifice", "land_reanimate")
				results = append(results, ComboResult{
					Cards:       []string{a.Name, b.Name},
					LoopType:    "synergy",
					Description: fmt.Sprintf("%s sacrifices lands, %s brings them back — repeatable land value", sac, rec),
				})
			}

			// Land reanimate + landfall = game-ending burst
			if (profileHasEffect(a, "land_reanimate") && profileHasTrigger(b, "landfall")) ||
				(profileHasEffect(b, "land_reanimate") && profileHasTrigger(a, "landfall")) {
				var reanimName, landfallName string
				if profileHasEffect(a, "land_reanimate") {
					reanimName = a.Name
					landfallName = b.Name
				} else {
					reanimName = b.Name
					landfallName = a.Name
				}
				results = append(results, ComboResult{
					Cards:       []string{a.Name, b.Name},
					LoopType:    "synergy",
					Description: fmt.Sprintf("%s returns lands en masse, %s triggers on each — landfall avalanche", reanimName, landfallName),
				})
			}

			// Land fetch + landfall = reliable triggers
			if (profileHasEffect(a, "land_fetch") && profileHasTrigger(b, "landfall")) ||
				(profileHasEffect(b, "land_fetch") && profileHasTrigger(a, "landfall")) {
				var fetchName, landfallName string
				if profileHasEffect(a, "land_fetch") {
					fetchName = a.Name
					landfallName = b.Name
				} else {
					fetchName = b.Name
					landfallName = a.Name
				}
				results = append(results, ComboResult{
					Cards:       []string{a.Name, b.Name},
					LoopType:    "synergy",
					Description: fmt.Sprintf("%s fetches lands, %s triggers on entry — landfall enabler", fetchName, landfallName),
				})
			}

			// Graveyard fill + graveyard payoff (delve, recursion, etc)
			if (containsRes(a.Produces, ResGraveyardFill) && containsRes(b.Consumes, ResGraveyardFill)) ||
				(containsRes(b.Produces, ResGraveyardFill) && containsRes(a.Consumes, ResGraveyardFill)) {
				var fillName, useName string
				if containsRes(a.Produces, ResGraveyardFill) {
					fillName = a.Name
					useName = b.Name
				} else {
					fillName = b.Name
					useName = a.Name
				}
				results = append(results, ComboResult{
					Cards:       []string{a.Name, b.Name},
					LoopType:    "synergy",
					Description: fmt.Sprintf("%s fills graveyard, %s uses it as fuel", fillName, useName),
				})
			}

			// Self-mill + single-card reanimate (non-mass)
			if (profileHasEffect(a, "self_mill") && a.IsRecursion && !profileHasEffect(a, "mass_reanimate")) ||
				(profileHasEffect(b, "self_mill") && b.IsRecursion && !profileHasEffect(b, "mass_reanimate")) {
				// Covered by the generic trigger synergies, skip to avoid duplication.
			} else if (profileHasEffect(a, "self_mill") && b.IsRecursion && !profileHasEffect(b, "mass_reanimate")) ||
				(profileHasEffect(b, "self_mill") && a.IsRecursion && !profileHasEffect(a, "mass_reanimate")) {
				var millName, recurName string
				if profileHasEffect(a, "self_mill") {
					millName = a.Name
					recurName = b.Name
				} else {
					millName = b.Name
					recurName = a.Name
				}
				results = append(results, ComboResult{
					Cards:       []string{a.Name, b.Name},
					LoopType:    "synergy",
					Description: fmt.Sprintf("%s mills into graveyard, %s recurs the best targets", millName, recurName),
				})
			}
		}
	}

	// ---------------------------------------------------------------
	// ARCHETYPE 1: TRIBAL SYNERGIES
	// Lords/payoffs paired with creatures sharing their type.
	// Limit: 1 synergy per lord+creature pair to avoid N*M explosion.
	// ---------------------------------------------------------------
	tribalSeen := map[string]bool{}
	for i := 0; i < len(profiles); i++ {
		if !profiles[i].IsTribalLord && !profiles[i].IsTribalPayoff {
			continue
		}
		for j := 0; j < len(profiles); j++ {
			if i == j {
				continue
			}
			common := commonType(profiles[i], profiles[j])
			if common == "" {
				continue
			}
			// Deduplicate: one entry per lord+creature pair
			key := profiles[i].Name + "|" + profiles[j].Name
			if tribalSeen[key] {
				continue
			}
			tribalSeen[key] = true

			desc := ""
			if profiles[i].IsTribalLord {
				desc = fmt.Sprintf("%s boosts %s (shared type: %s)", profiles[i].Name, profiles[j].Name, common)
			} else {
				desc = fmt.Sprintf("%s triggers from %s (shared type: %s)", profiles[i].Name, profiles[j].Name, common)
			}
			results = append(results, ComboResult{
				Cards:       []string{profiles[i].Name, profiles[j].Name},
				LoopType:    "synergy",
				Description: desc,
			})
		}
	}

	// ---------------------------------------------------------------
	// ARCHETYPE 2: +1/+1 COUNTER SYNERGIES
	// counter_add + counter_matters, proliferate + any counter effect
	// ---------------------------------------------------------------
	for i := 0; i < len(profiles); i++ {
		for j := i + 1; j < len(profiles); j++ {
			a, b := profiles[i], profiles[j]

			// counter_add + counter_matters
			if profileHasEffect(a, "counter_add") && profileHasTrigger(b, "counter_matters") {
				results = append(results, ComboResult{
					Cards:       []string{a.Name, b.Name},
					LoopType:    "synergy",
					Description: fmt.Sprintf("%s adds +1/+1 counters, %s cares about them", a.Name, b.Name),
				})
			} else if profileHasEffect(b, "counter_add") && profileHasTrigger(a, "counter_matters") {
				results = append(results, ComboResult{
					Cards:       []string{b.Name, a.Name},
					LoopType:    "synergy",
					Description: fmt.Sprintf("%s adds +1/+1 counters, %s cares about them", b.Name, a.Name),
				})
			}

			// proliferate + counter_add or counter_matters
			if profileHasEffect(a, "proliferate") && (profileHasEffect(b, "counter_add") || profileHasTrigger(b, "counter_matters")) {
				results = append(results, ComboResult{
					Cards:       []string{a.Name, b.Name},
					LoopType:    "synergy",
					Description: fmt.Sprintf("%s proliferates counters that %s uses", a.Name, b.Name),
				})
			} else if profileHasEffect(b, "proliferate") && (profileHasEffect(a, "counter_add") || profileHasTrigger(a, "counter_matters")) {
				results = append(results, ComboResult{
					Cards:       []string{b.Name, a.Name},
					LoopType:    "synergy",
					Description: fmt.Sprintf("%s proliferates counters that %s uses", b.Name, a.Name),
				})
			}

			// counter_move + counter_add = counter manipulation engine
			if profileHasEffect(a, "counter_move") && profileHasEffect(b, "counter_add") {
				results = append(results, ComboResult{
					Cards:       []string{a.Name, b.Name},
					LoopType:    "synergy",
					Description: fmt.Sprintf("%s moves counters, %s adds them — counter manipulation engine", a.Name, b.Name),
				})
			} else if profileHasEffect(b, "counter_move") && profileHasEffect(a, "counter_add") {
				results = append(results, ComboResult{
					Cards:       []string{b.Name, a.Name},
					LoopType:    "synergy",
					Description: fmt.Sprintf("%s moves counters, %s adds them — counter manipulation engine", b.Name, a.Name),
				})
			}
		}
	}

	// ---------------------------------------------------------------
	// ARCHETYPE 3: DAY/NIGHT SYNERGIES
	// Daynight cards synergize with each other and with daynight_toggle
	// ---------------------------------------------------------------
	for i := 0; i < len(profiles); i++ {
		for j := i + 1; j < len(profiles); j++ {
			a, b := profiles[i], profiles[j]

			// daynight_toggle + daynight trigger
			if profileHasEffect(a, "daynight_toggle") && profileHasTrigger(b, "daynight") {
				results = append(results, ComboResult{
					Cards:       []string{a.Name, b.Name},
					LoopType:    "synergy",
					Description: fmt.Sprintf("%s toggles day/night, triggering %s transform", a.Name, b.Name),
				})
			} else if profileHasEffect(b, "daynight_toggle") && profileHasTrigger(a, "daynight") {
				results = append(results, ComboResult{
					Cards:       []string{b.Name, a.Name},
					LoopType:    "synergy",
					Description: fmt.Sprintf("%s toggles day/night, triggering %s transform", b.Name, a.Name),
				})
			}

			// Two daynight cards synergize (they transform together)
			if profileHasTrigger(a, "daynight") && profileHasTrigger(b, "daynight") {
				results = append(results, ComboResult{
					Cards:       []string{a.Name, b.Name},
					LoopType:    "synergy",
					Description: fmt.Sprintf("%s and %s transform together on day/night cycle", a.Name, b.Name),
				})
			}
		}
	}

	// ---------------------------------------------------------------
	// ARCHETYPE 4: EXILE MATTERS SYNERGIES
	// exile_cast + impulse_draw, exile_return combos
	// ---------------------------------------------------------------
	for i := 0; i < len(profiles); i++ {
		for j := i + 1; j < len(profiles); j++ {
			a, b := profiles[i], profiles[j]

			// Multiple impulse draw effects synergize with exile-cast payoffs
			if profileHasEffect(a, "impulse_draw") && profileHasEffect(b, "exile_cast") {
				results = append(results, ComboResult{
					Cards:       []string{a.Name, b.Name},
					LoopType:    "synergy",
					Description: fmt.Sprintf("%s exiles for cards, %s casts from exile — exile value engine", a.Name, b.Name),
				})
			} else if profileHasEffect(b, "impulse_draw") && profileHasEffect(a, "exile_cast") {
				results = append(results, ComboResult{
					Cards:       []string{b.Name, a.Name},
					LoopType:    "synergy",
					Description: fmt.Sprintf("%s exiles for cards, %s casts from exile — exile value engine", b.Name, a.Name),
				})
			}

			// exile_return + exile effects (flicker-style value)
			if profileHasEffect(a, "exile_return") && (profileHasEffect(b, "exile") || profileHasEffect(b, "exile_cast")) {
				results = append(results, ComboResult{
					Cards:       []string{a.Name, b.Name},
					LoopType:    "synergy",
					Description: fmt.Sprintf("%s returns exiled cards, %s exiles for value", a.Name, b.Name),
				})
			} else if profileHasEffect(b, "exile_return") && (profileHasEffect(a, "exile") || profileHasEffect(a, "exile_cast")) {
				results = append(results, ComboResult{
					Cards:       []string{b.Name, a.Name},
					LoopType:    "synergy",
					Description: fmt.Sprintf("%s returns exiled cards, %s exiles for value", b.Name, a.Name),
				})
			}
		}
	}

	// ---------------------------------------------------------------
	// ARCHETYPE 5: FACE-DOWN SYNERGIES
	// Only pair enablers/payoffs with morph creatures, not morph-morph pairs.
	// Enablers: cards with facedown_enabler (manifest others) or facedown trigger
	// Payoffs: cards with face_up effects or facedown trigger
	// ---------------------------------------------------------------
	for i := 0; i < len(profiles); i++ {
		for j := i + 1; j < len(profiles); j++ {
			a, b := profiles[i], profiles[j]

			// facedown enabler (manifest/etc) + morph creature
			if profileHasEffect(a, "facedown_enabler") && profileHasEffect(b, "facedown_create") {
				results = append(results, ComboResult{
					Cards:       []string{a.Name, b.Name},
					LoopType:    "synergy",
					Description: fmt.Sprintf("%s manifests creatures, %s has morph to flip for value", a.Name, b.Name),
				})
			} else if profileHasEffect(b, "facedown_enabler") && profileHasEffect(a, "facedown_create") {
				results = append(results, ComboResult{
					Cards:       []string{b.Name, a.Name},
					LoopType:    "synergy",
					Description: fmt.Sprintf("%s manifests creatures, %s has morph to flip for value", b.Name, a.Name),
				})
			}

			// facedown payoff (trigger on facedown) + morph creature
			if profileHasTrigger(a, "facedown") && profileHasEffect(b, "facedown_create") && !profileHasTrigger(b, "facedown") {
				results = append(results, ComboResult{
					Cards:       []string{a.Name, b.Name},
					LoopType:    "synergy",
					Description: fmt.Sprintf("%s triggers from face-down creatures, %s provides them", a.Name, b.Name),
				})
			} else if profileHasTrigger(b, "facedown") && profileHasEffect(a, "facedown_create") && !profileHasTrigger(a, "facedown") {
				results = append(results, ComboResult{
					Cards:       []string{b.Name, a.Name},
					LoopType:    "synergy",
					Description: fmt.Sprintf("%s triggers from face-down creatures, %s provides them", b.Name, a.Name),
				})
			}

			// face_up payoff + facedown_create (flip value)
			if profileHasEffect(a, "face_up") && profileHasEffect(b, "facedown_create") && !profileHasEffect(a, "facedown_create") {
				results = append(results, ComboResult{
					Cards:       []string{a.Name, b.Name},
					LoopType:    "synergy",
					Description: fmt.Sprintf("%s triggers on flip, %s provides face-down targets", a.Name, b.Name),
				})
			} else if profileHasEffect(b, "face_up") && profileHasEffect(a, "facedown_create") && !profileHasEffect(b, "facedown_create") {
				results = append(results, ComboResult{
					Cards:       []string{b.Name, a.Name},
					LoopType:    "synergy",
					Description: fmt.Sprintf("%s triggers on flip, %s provides face-down targets", b.Name, a.Name),
				})
			}
		}
	}

	// ---------------------------------------------------------------
	// ARCHETYPE 6: TOP-OF-LIBRARY SYNERGIES
	// topdeck_manipulate + topdeck_reveal
	// ---------------------------------------------------------------
	for i := 0; i < len(profiles); i++ {
		for j := i + 1; j < len(profiles); j++ {
			a, b := profiles[i], profiles[j]

			if profileHasEffect(a, "topdeck_manipulate") && profileHasEffect(b, "topdeck_reveal") {
				results = append(results, ComboResult{
					Cards:       []string{a.Name, b.Name},
					LoopType:    "synergy",
					Description: fmt.Sprintf("%s sets up top of library, %s reveals for value", a.Name, b.Name),
				})
			} else if profileHasEffect(b, "topdeck_manipulate") && profileHasEffect(a, "topdeck_reveal") {
				results = append(results, ComboResult{
					Cards:       []string{b.Name, a.Name},
					LoopType:    "synergy",
					Description: fmt.Sprintf("%s sets up top of library, %s reveals for value", b.Name, a.Name),
				})
			}
		}
	}

	// ---------------------------------------------------------------
	// ARCHETYPE 7: STAX/TAX SYNERGIES
	// tax + tax = stax package, lock + lock = hard lock
	// symmetric_pain + symmetric_pain = pain package
	// ---------------------------------------------------------------
	for i := 0; i < len(profiles); i++ {
		for j := i + 1; j < len(profiles); j++ {
			a, b := profiles[i], profiles[j]

			// Two tax effects = stax synergy
			if profileHasEffect(a, "tax") && profileHasEffect(b, "tax") {
				results = append(results, ComboResult{
					Cards:       []string{a.Name, b.Name},
					LoopType:    "synergy",
					Description: fmt.Sprintf("%s and %s stack tax effects — stax package", a.Name, b.Name),
				})
			}

			// Lock + lock = hard lock
			if profileHasEffect(a, "lock") && profileHasEffect(b, "lock") {
				results = append(results, ComboResult{
					Cards:       []string{a.Name, b.Name},
					LoopType:    "synergy",
					Description: fmt.Sprintf("%s and %s layer restrictions — hard lock", a.Name, b.Name),
				})
			}

			// Tax + lock = stax/lock synergy
			if profileHasEffect(a, "tax") && profileHasEffect(b, "lock") {
				results = append(results, ComboResult{
					Cards:       []string{a.Name, b.Name},
					LoopType:    "synergy",
					Description: fmt.Sprintf("%s taxes, %s locks — layered denial", a.Name, b.Name),
				})
			} else if profileHasEffect(b, "tax") && profileHasEffect(a, "lock") {
				results = append(results, ComboResult{
					Cards:       []string{b.Name, a.Name},
					LoopType:    "synergy",
					Description: fmt.Sprintf("%s taxes, %s locks — layered denial", b.Name, a.Name),
				})
			}

			// Symmetric pain pairing
			if profileHasEffect(a, "symmetric_pain") && profileHasEffect(b, "symmetric_pain") {
				results = append(results, ComboResult{
					Cards:       []string{a.Name, b.Name},
					LoopType:    "synergy",
					Description: fmt.Sprintf("%s and %s compound opponent pain", a.Name, b.Name),
				})
			}
		}
	}

	// ---------------------------------------------------------------
	// ARCHETYPE 8: BLINK COMBO DETECTION
	// Repeatable blink + mana ETB = infinite mana potential
	// Repeatable blink + damage ETB = infinite damage potential
	// Repeatable blink + value ETB = repeatable value engine
	// ---------------------------------------------------------------

	blinkers := []CardProfile{}
	manaETBs := []CardProfile{}
	damageETBs := []CardProfile{}
	valueETBs := []CardProfile{}

	for _, p := range profiles {
		if p.IsBlinker {
			blinkers = append(blinkers, p)
		}
		if p.HasManaETB {
			manaETBs = append(manaETBs, p)
		}
		if p.HasValueETB {
			// Check if the ETB deals damage
			hasDamageETB := false
			for _, e := range p.Effects {
				if e == "damage" || e == "drain" {
					hasDamageETB = true
				}
			}
			if hasDamageETB {
				damageETBs = append(damageETBs, p)
			}
			valueETBs = append(valueETBs, p)
		}
	}

	// Flag blink + mana ETB as determined loop potential
	for _, blinker := range blinkers {
		for _, manaETB := range manaETBs {
			if blinker.Name == manaETB.Name {
				continue
			}
			results = append(results, ComboResult{
				Cards:    []string{blinker.Name, manaETB.Name},
				LoopType: "determined",
				Description: fmt.Sprintf("BLINK COMBO: %s blinks %s for repeatable mana — infinite mana potential",
					blinker.Name, manaETB.Name),
			})
		}
		// Flag blink + damage ETB as determined loop (has kill output)
		for _, dmgETB := range damageETBs {
			if blinker.Name == dmgETB.Name {
				continue
			}
			// Don't duplicate if already flagged as mana combo
			alreadyFlagged := false
			for _, m := range manaETBs {
				if m.Name == dmgETB.Name {
					alreadyFlagged = true
					break
				}
			}
			if alreadyFlagged {
				continue
			}
			results = append(results, ComboResult{
				Cards:    []string{blinker.Name, dmgETB.Name},
				LoopType: "determined",
				Description: fmt.Sprintf("BLINK COMBO: %s blinks %s for repeatable damage/drain",
					blinker.Name, dmgETB.Name),
			})
		}
		// Flag blink + value ETB as synergy (not infinite but strong)
		// Limit to 3 best value ETBs to avoid noise
		valueCount := 0
		for _, valETB := range valueETBs {
			if blinker.Name == valETB.Name {
				continue
			}
			if valETB.HasManaETB {
				continue
			} // already flagged above
			hasDmg := false
			for _, e := range valETB.Effects {
				if e == "damage" || e == "drain" {
					hasDmg = true
				}
			}
			if hasDmg {
				continue
			} // already flagged above
			if valueCount >= 3 {
				break
			} // limit noise
			results = append(results, ComboResult{
				Cards:    []string{blinker.Name, valETB.Name},
				LoopType: "synergy",
				Description: fmt.Sprintf("BLINK VALUE: %s blinks %s for repeatable ETB value",
					blinker.Name, valETB.Name),
			})
			valueCount++
		}
	}

	// ---------------------------------------------------------------
	// Lifegain ↔ Lifeloss infinite loop pattern detection
	// Any "lifegain → opponent loses life" + "opponent loses life → you gain life"
	// = mandatory infinite drain loop (true infinite, kills the table)
	// ---------------------------------------------------------------
	for i := 0; i < len(profiles); i++ {
		for j := i + 1; j < len(profiles); j++ {
			a, b := profiles[i], profiles[j]
			if (a.LifegainToDrain && b.LifelossToPump) || (b.LifegainToDrain && a.LifelossToPump) {
				drainer := a.Name
				pumper := b.Name
				if b.LifegainToDrain {
					drainer = b.Name
					pumper = a.Name
				}
				results = append(results, ComboResult{
					Cards:    []string{drainer, pumper},
					LoopType: "true_infinite",
					Description: fmt.Sprintf("DRAIN LOOP: %s converts lifegain→damage, %s converts damage→lifegain. Mandatory infinite drain — kills the table.",
						drainer, pumper),
					Confirmed: false,
				})
			}
		}
	}

	// ---------------------------------------------------------------
	// PATTERN: Counter → Damage → Counter loop
	// "whenever counters placed, deal damage" + "whenever damage, place counter"
	// = mandatory infinite damage (Shalai and Hallar + The Red Terror)
	// ---------------------------------------------------------------
	for i := 0; i < len(profiles); i++ {
		for j := i + 1; j < len(profiles); j++ {
			a, b := profiles[i], profiles[j]
			if (a.CounterToDamage && b.DamageToCounter) || (b.CounterToDamage && a.DamageToCounter) {
				damager := a.Name
				grower := b.Name
				if b.CounterToDamage {
					damager = b.Name
					grower = a.Name
				}
				results = append(results, ComboResult{
					Cards:    []string{damager, grower},
					LoopType: "true_infinite",
					Description: fmt.Sprintf("COUNTER-DAMAGE LOOP: %s converts +1/+1 counters→damage, %s converts damage→+1/+1 counters. Any counter placement starts infinite damage.",
						damager, grower),
					Confirmed: false,
				})
			}
		}
	}

	// ---------------------------------------------------------------
	// PATTERN: Self-Mill Win (Lab Man line)
	// "Win with empty library" + "empties library" = instant win
	// ---------------------------------------------------------------
	for i := 0; i < len(profiles); i++ {
		for j := i + 1; j < len(profiles); j++ {
			a, b := profiles[i], profiles[j]
			if (a.WinsWithEmptyLib && b.EmptiesLibrary) || (b.WinsWithEmptyLib && a.EmptiesLibrary) {
				winner := a.Name
				enabler := b.Name
				if b.WinsWithEmptyLib {
					winner = b.Name
					enabler = a.Name
				}
				results = append(results, ComboResult{
					Cards:    []string{winner, enabler},
					LoopType: "determined",
					Description: fmt.Sprintf("WIN COMBO: %s wins the game, %s empties the library",
						winner, enabler),
				})
			}
		}
	}

	// ---------------------------------------------------------------
	// PATTERN: Infinite Mana + Mana Sink
	// Any infinite mana combo + X-cost payoff = kill
	// ---------------------------------------------------------------
	for i := 0; i < len(profiles); i++ {
		if !profiles[i].IsManaPayoff {
			continue
		}
		// Check if any determined/true_infinite loop in the deck produces mana
		for _, combo := range results {
			if combo.LoopType != "determined" && combo.LoopType != "true_infinite" {
				continue
			}
			if strings.Contains(strings.ToLower(combo.Description), "mana") {
				// Avoid listing the payoff if it's already part of this combo
				alreadyInCombo := false
				for _, c := range combo.Cards {
					if c == profiles[i].Name {
						alreadyInCombo = true
						break
					}
				}
				if alreadyInCombo {
					continue
				}
				results = append(results, ComboResult{
					Cards:    append(append([]string{}, combo.Cards...), profiles[i].Name),
					LoopType: "determined",
					Description: fmt.Sprintf("MANA SINK: %s converts infinite mana from %s into a win",
						profiles[i].Name, strings.Join(combo.Cards, " + ")),
				})
				break // one sink per payoff
			}
		}
	}

	// Check for infinite mana combos WITHOUT a sink — flag as a gap
	hasManaPayoff := false
	for _, p := range profiles {
		if p.IsManaPayoff {
			hasManaPayoff = true
			break
		}
	}
	for idx := range results {
		combo := &results[idx]
		if (combo.LoopType == "determined" || combo.LoopType == "true_infinite") &&
			strings.Contains(strings.ToLower(combo.Description), "mana") &&
			!strings.Contains(combo.Description, "MANA SINK") &&
			!strings.Contains(combo.Description, "WIN COMBO") &&
			!strings.Contains(combo.Description, "DRAIN LOOP") {
			if !hasManaPayoff {
				combo.Description += " | ⚠️ NO MANA SINK IN DECK — infinite mana with nothing to spend it on"
			}
		}
	}

	// ---------------------------------------------------------------
	// PATTERN: Infinite ETB + ETB Damage
	// Token/blink loop + Purphoros/Impact Tremors = kill
	// ---------------------------------------------------------------
	for i := 0; i < len(profiles); i++ {
		if !profiles[i].HasETBDamage {
			continue
		}
		for j := 0; j < len(profiles); j++ {
			if i == j {
				continue
			}
			if profiles[j].MakesInfiniteTokens || profiles[j].IsBlinker {
				results = append(results, ComboResult{
					Cards:    []string{profiles[i].Name, profiles[j].Name},
					LoopType: "determined",
					Description: fmt.Sprintf("ETB KILL: %s deals damage on ETB, %s provides repeatable ETBs",
						profiles[i].Name, profiles[j].Name),
				})
			}
		}
	}

	// ---------------------------------------------------------------
	// PATTERN: Sacrifice Loop + Death Drain
	// Sac outlet + death trigger drain = kill engine
	// ---------------------------------------------------------------
	for i := 0; i < len(profiles); i++ {
		if !profiles[i].HasDeathDrain {
			continue
		}
		for j := 0; j < len(profiles); j++ {
			if i == j {
				continue
			}
			if profiles[j].IsOutlet {
				results = append(results, ComboResult{
					Cards:    []string{profiles[i].Name, profiles[j].Name},
					LoopType: "synergy",
					Description: fmt.Sprintf("DEATH ENGINE: %s drains on death, %s provides sacrifice outlet",
						profiles[i].Name, profiles[j].Name),
				})
			}
		}
	}

	// ---------------------------------------------------------------
	// Generic trigger-effect synergies (after special patterns).
	// ---------------------------------------------------------------
	for i := 0; i < len(profiles); i++ {
		for j := i + 1; j < len(profiles); j++ {
			synergies := findPairSynergies(profiles[i], profiles[j])
			results = append(results, synergies...)
		}
	}

	return results
}

func findPairSynergies(a, b CardProfile) []ComboResult {
	var results []ComboResult

	// Check if A's trigger matches B's effect type.
	for _, trig := range a.Triggers {
		for _, eff := range b.Effects {
			if triggerMatchesEffect(trig, eff) {
				results = append(results, ComboResult{
					Cards:    []string{a.Name, b.Name},
					LoopType: "synergy",
					Description: fmt.Sprintf("%s triggers on '%s' which %s provides",
						a.Name, trig, b.Name),
				})
				goto doneAtoB // one synergy per direction is enough
			}
		}
	}
doneAtoB:

	// Check the reverse direction.
	for _, trig := range b.Triggers {
		for _, eff := range a.Effects {
			if triggerMatchesEffect(trig, eff) {
				results = append(results, ComboResult{
					Cards:    []string{b.Name, a.Name},
					LoopType: "synergy",
					Description: fmt.Sprintf("%s triggers on '%s' which %s provides",
						b.Name, trig, a.Name),
				})
				goto doneBtoA
			}
		}
	}
doneBtoA:

	return results
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// resourceOverlap returns the resources that appear in both produces and consumes.
func resourceOverlap(produces, consumes []ResourceType) []ResourceType {
	var overlap []ResourceType
	seen := map[ResourceType]bool{}
	for _, p := range produces {
		for _, c := range consumes {
			if p == c && !seen[p] {
				overlap = append(overlap, p)
				seen[p] = true
			}
		}
	}
	return overlap
}

// containsRes checks if a resource slice contains a specific type.
func containsRes(rs []ResourceType, target ResourceType) bool {
	for _, r := range rs {
		if r == target {
			return true
		}
	}
	return false
}

// hasDamageOutput checks if either card deals damage.
func hasDamageOutput(a, b CardProfile) bool {
	return containsEffect(a.Effects, "damage") || containsEffect(b.Effects, "damage")
}

// hasDrainOutput checks if either card drains opponents.
func hasDrainOutput(a, b CardProfile) bool {
	return containsEffect(a.Effects, "drain") || containsEffect(b.Effects, "drain")
}

// hasMillOutput checks if either card mills opponents.
func hasMillOutput(a, b CardProfile) bool {
	return containsEffect(a.Effects, "mill") || containsEffect(b.Effects, "mill")
}

// containsEffect checks if an effects list has a particular effect.
func containsEffect(effects []string, target string) bool {
	for _, e := range effects {
		if e == target {
			return true
		}
	}
	return false
}

// profileHasEffect checks if a CardProfile has a specific effect.
func profileHasEffect(p CardProfile, eff string) bool {
	for _, e := range p.Effects {
		if e == eff {
			return true
		}
	}
	return false
}

// profileHasTrigger checks if a CardProfile has a specific trigger.
func profileHasTrigger(p CardProfile, trig string) bool {
	for _, t := range p.Triggers {
		if t == trig {
			return true
		}
	}
	return false
}

// pickByEffect returns the names of two cards based on which has effectA vs effectB.
// If a has effectA, returns (a.Name, b.Name), otherwise (b.Name, a.Name).
func pickByEffect(a, b CardProfile, effectA, effectB string) (string, string) {
	if profileHasEffect(a, effectA) {
		return a.Name, b.Name
	}
	return b.Name, a.Name
}

// triggerMatchesEffect maps trigger keywords to effect keywords.
func triggerMatchesEffect(trigger, effect string) bool {
	switch trigger {
	case "sacrifice":
		return effect == "sacrifice" || effect == "create_token" ||
			effect == "land_reanimate" || effect == "mass_reanimate"
	case "dies":
		return effect == "sacrifice" || effect == "destroy" || effect == "mass_reanimate"
	case "etb":
		return effect == "create_token" || effect == "self_mill"
	case "damage":
		return effect == "damage"
	case "lifegain":
		return effect == "drain" // drain causes life loss for opponents + life gain for you
	case "lifeloss":
		return effect == "drain" || effect == "damage"
	case "token_created":
		return effect == "create_token"
	case "ltb":
		return effect == "sacrifice" || effect == "exile"
	case "landfall":
		return effect == "land_reanimate" || effect == "land_fetch" || effect == "land_sacrifice"
	}
	return false
}

// resourceNames converts a slice of ResourceType to a readable string.
func resourceNames(rs []ResourceType) string {
	if len(rs) == 0 {
		return "none"
	}
	names := make([]string, len(rs))
	for i, r := range rs {
		names[i] = string(r)
	}
	return strings.Join(names, ", ")
}

// deduplicateCombos removes duplicate combos by card set.
func deduplicateCombos(combos []ComboResult) []ComboResult {
	seen := map[string]bool{}
	var result []ComboResult
	for _, c := range combos {
		// Sort card names for consistent keying.
		sorted := make([]string, len(c.Cards))
		copy(sorted, c.Cards)
		for i := 0; i < len(sorted); i++ {
			for j := i + 1; j < len(sorted); j++ {
				if sorted[i] > sorted[j] {
					sorted[i], sorted[j] = sorted[j], sorted[i]
				}
			}
		}
		key := strings.Join(sorted, "|")
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, c)
	}
	return result
}

// containsAny checks if s contains any of the given substrings.
func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// uniqueResources deduplicates a ResourceType slice.
func uniqueResources(rs []ResourceType) []ResourceType {
	if len(rs) <= 1 {
		return rs
	}
	seen := map[ResourceType]bool{}
	var result []ResourceType
	for _, r := range rs {
		if !seen[r] {
			seen[r] = true
			result = append(result, r)
		}
	}
	return result
}

// uniqueStrings deduplicates a string slice.
func uniqueStrings(ss []string) []string {
	if len(ss) <= 1 {
		return ss
	}
	seen := map[string]bool{}
	var result []string
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

// ---------------------------------------------------------------------------
// Mana curve / color helpers
// ---------------------------------------------------------------------------

// countManaCostPips counts colored pips (W/U/B/R/G) in a mana cost string
// like "{2}{U}{B}" or "{W}{W}{U}".
func countManaCostPips(manaCost string, demand map[string]int) {
	for _, c := range manaCost {
		switch c {
		case 'W':
			demand["W"]++
		case 'U':
			demand["U"]++
		case 'B':
			demand["B"]++
		case 'R':
			demand["R"]++
		case 'G':
			demand["G"]++
		}
	}
}

// countLandColors adds a land card's color production to the supply map.
func countLandColors(p CardProfile, supply map[string]int) {
	if len(p.LandColors) > 0 {
		for _, c := range p.LandColors {
			supply[c]++
		}
		return
	}
	// Fallback: if the land produces mana but we couldn't determine colors,
	// count it as colorless (doesn't add to any color supply).
}

// ---------------------------------------------------------------------------
// Tribal helpers
// ---------------------------------------------------------------------------

// knownCreatureTypes is the master list of creature types we detect.
var knownCreatureTypes = []string{
	"advisor", "aetherborn", "ally", "angel", "antelope", "ape", "archer",
	"archon", "army", "artificer", "assassin", "assembly-worker", "atog",
	"aurochs", "avatar", "azra", "badger", "barbarian", "bard", "basilisk",
	"bat", "bear", "beast", "beholder", "berserker", "bird", "blinkmoth",
	"boar", "bringer", "brushwagg", "camarid", "camel", "caribou", "carrier",
	"cat", "centaur", "cephalid", "changeling", "chimera", "citizen", "cleric",
	"cockatrice", "construct", "coward", "crab", "crocodile", "cyclops",
	"dauthi", "demigod", "demon", "deserter", "devil", "dinosaur", "djinn",
	"dog", "dragon", "drake", "dreadnought", "drone", "druid", "dryad",
	"dwarf", "efreet", "egg", "elder", "eldrazi", "elemental", "elephant",
	"elf", "elk", "eye", "faerie", "ferret", "fish", "flagbearer", "fox",
	"frog", "fungus", "gargoyle", "germ", "giant", "gnoll", "gnome", "goat",
	"goblin", "god", "golem", "gorgon", "graveborn", "gremlin", "griffin",
	"hag", "halfling", "hamster", "harpy", "hellion", "hippo", "hippogriff",
	"homarid", "homunculus", "horror", "horse", "human", "hydra", "hyena",
	"illusion", "imp", "incarnation", "insect", "jackal", "jellyfish",
	"juggernaut", "kavu", "kirin", "kithkin", "knight", "kobold", "kor",
	"kraken", "lamia", "lammasu", "leech", "leviathan", "lhurgoyf",
	"licid", "lizard", "manticore", "masticore", "mercenary", "merfolk",
	"metathran", "minion", "minotaur", "mole", "monger", "mongoose", "monk",
	"monkey", "moonfolk", "mouse", "mutant", "myr", "mystic", "naga",
	"nautilus", "nazgul", "nephilim", "nightmare", "nightstalker", "ninja",
	"noble", "noggle", "nomad", "nymph", "octopus", "ogre", "ooze", "orb",
	"orc", "orgg", "otter", "ox", "oyster", "pangolin", "peasant", "pegasus",
	"pentavite", "pest", "phelddagrif", "phoenix", "phyrexian", "pilot",
	"pincher", "pirate", "plant", "praetor", "prism", "processor",
	"rabbit", "raccoon", "ranger", "rat", "rebel", "reflection", "rhino",
	"rigger", "rogue", "sable", "salamander", "samurai", "sand", "saproling",
	"satyr", "scarecrow", "scion", "scorpion", "scout", "sculpture",
	"serf", "serpent", "servo", "shade", "shaman", "shapeshifter", "shark",
	"sheep", "siren", "skeleton", "slith", "sliver", "slug", "snake",
	"soldier", "soltari", "spawn", "specter", "spellshaper", "sphinx",
	"spider", "spike", "spirit", "splinter", "sponge", "squid", "squirrel",
	"starfish", "surrakar", "survivor", "tentacle", "tetravite", "thalakos",
	"thopter", "thrull", "tiefling", "treefolk", "trilobite", "troll",
	"turtle", "unicorn", "vampire", "vedalken", "viashino", "volver",
	"wall", "warlock", "warrior", "weird", "werewolf", "whale", "wizard",
	"wolf", "wolverine", "wombat", "worm", "wraith", "wurm", "yeti",
	"zombie", "zubera",
}

// extractCreatureTypes extracts creature types from the type line and
// oracle text references. Returns lowercase type names.
func extractCreatureTypes(oracleText, typeLine string) []string {
	var types []string
	seen := map[string]bool{}

	// Parse type line: everything after " — " (or " - ") is subtypes.
	// Creature subtypes are the creature types.
	dashIdx := strings.Index(typeLine, " — ")
	if dashIdx < 0 {
		dashIdx = strings.Index(typeLine, " - ")
	}
	if dashIdx >= 0 {
		subtypePart := strings.ToLower(typeLine[dashIdx+3:])
		// Also strip the dash variant
		subtypePart = strings.TrimPrefix(subtypePart, " ")
		words := strings.Fields(subtypePart)
		for _, w := range words {
			w = strings.TrimRight(w, ",.")
			for _, ct := range knownCreatureTypes {
				if w == ct {
					if !seen[ct] {
						seen[ct] = true
						types = append(types, ct)
					}
					break
				}
			}
		}
	}

	// Also check oracle text for type references (for tribal spells/enchantments)
	for _, ct := range knownCreatureTypes {
		if seen[ct] {
			continue
		}
		// Look for the type being specifically referenced in oracle text
		// (e.g., "Zombies you control", "each Zombie", "target Zombie")
		if strings.Contains(oracleText, ct+" ") || strings.Contains(oracleText, ct+"s") ||
			strings.Contains(oracleText, "target "+ct) || strings.Contains(oracleText, "each "+ct) {
			// Avoid false positives from common English words
			if ct == "eye" || ct == "egg" || ct == "wall" || ct == "sand" || ct == "bear" {
				continue // too many false positives
			}
			seen[ct] = true
			types = append(types, ct)
		}
	}

	return types
}

// commonType returns the first shared creature type between two profiles,
// or empty string if none.
func commonType(a, b CardProfile) string {
	for _, ta := range a.CreatureTypes {
		for _, tb := range b.CreatureTypes {
			if ta == tb {
				return ta
			}
		}
	}
	return ""
}
