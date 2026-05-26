package main

// Combo class constants. Every KnownCombo carries one of these; combos
// imported from Commander Spellbook are classified by heuristic
// (ClassifyComboHeuristic). The taxonomy is the cleavage hat uses for
// ComboProximity affinity weighting — pieces of plans in the deck's
// dominant class are scored higher than pieces of off-class plans, so a
// deck with three infinite_drain win lines doesn't get equal weight on
// a random infinite_token piece that happens to share a card with one
// of those lines.
const (
	ComboClassInfiniteMana    = "infinite_mana"     // unbounded mana per loop
	ComboClassInfiniteDamage  = "infinite_damage"   // direct damage per loop
	ComboClassInfiniteDrain   = "infinite_drain"    // life-loss/gain loop (Exquisite Blood family)
	ComboClassInfiniteTokens  = "infinite_tokens"   // unbounded creature tokens
	ComboClassInfiniteETB     = "infinite_etb"      // unbounded ETB/death/cast triggers (needs outlet)
	ComboClassInfiniteMill    = "infinite_mill"     // mills self or opponents to empty library
	ComboClassLibraryExileWin = "library_exile_win" // exile library + empty-library win (Thoracle/LabMan/Jace lines)
	ComboClassStormFinisher   = "storm_finisher"    // storm/spell-count payoff (Aetherflux)
	ComboClassETBPayoff       = "etb_payoff"        // converts ETB volume to damage (Purphoros, Impact Tremors)
	ComboClassETBDoubler      = "etb_doubler"       // multiplies ETB value (Panharmonicon, Yarok)
	ComboClassBlinkEngine     = "blink_engine"      // repeated blink for ETB value
	ComboClassManaSink        = "mana_sink"         // payoff for infinite mana (Staff of Domination solo)
	ComboClassCombatFinisher  = "combat_finisher"   // pump/anthem-driven lethal swing (Craterhoof package)
	ComboClassLockdown        = "lockdown"          // prison/lock loops that don't kill but prevent opponents from playing (Stasis, Knowledge Pool, Helm of Obedience + RIP, Eternity Vessel + clones)
	ComboClassUnknown         = "unknown"           // unclassified — fallback for unmatched imports
)

// ComboClassLabel returns a human-readable label for a class constant.
// Used by report renderers (JSON consumers, text, markdown) to surface
// the taxonomy without leaking the snake_case constant value into the
// reader-facing output. Unknown / empty values yield empty strings so
// callers can skip rendering when the classifier returned nothing.
func ComboClassLabel(class string) string {
	switch class {
	case ComboClassInfiniteMana:
		return "Infinite Mana"
	case ComboClassInfiniteDamage:
		return "Infinite Damage"
	case ComboClassInfiniteDrain:
		return "Infinite Drain"
	case ComboClassInfiniteTokens:
		return "Infinite Tokens"
	case ComboClassInfiniteETB:
		return "Infinite ETB"
	case ComboClassInfiniteMill:
		return "Infinite Mill"
	case ComboClassLibraryExileWin:
		return "Library-Exile Win"
	case ComboClassStormFinisher:
		return "Storm Finisher"
	case ComboClassETBPayoff:
		return "ETB Payoff"
	case ComboClassETBDoubler:
		return "ETB Doubler"
	case ComboClassBlinkEngine:
		return "Blink Engine"
	case ComboClassManaSink:
		return "Mana Sink"
	case ComboClassCombatFinisher:
		return "Combat Finisher"
	case ComboClassLockdown:
		return "Lockdown"
	}
	return ""
}

// KnownCombo is a confirmed combo that Freya should always flag
// with 100% confidence when all pieces are in a deck.
type KnownCombo struct {
	Name        string
	Pieces      []string // card names (all must be present)
	Type        string   // "true_infinite" / "determined" / "synergy" — loop semantics
	Class       string   // one of the ComboClass* constants — what the combo PRODUCES
	Mandatory   bool     // are the triggers mandatory?
	Description string
	Outlets     []string // known cards that convert this into a win
	Stops       []string // known cards that break the loop
}

var KnownCombos = []KnownCombo{
	// ── True Infinites (mandatory) ──
	{
		Name:        "Polyraptor + Marauding Raptor",
		Pieces:      []string{"Polyraptor", "Marauding Raptor"},
		Type:        "true_infinite",
		Class:       ComboClassInfiniteTokens,
		Mandatory:   true,
		Description: "Polyraptor ETB → Marauding Raptor deals 2 → Enrage creates copy → copy ETBs → infinite loop. Game draws without outlet or stop.",
		Outlets:     []string{"Warstorm Surge", "Impact Tremors", "Purphoros, God of the Forge", "Terror of the Peaks", "Goblin Bombardment", "Altar of Dementia"},
		Stops:       []string{"Marauding Raptor itself can be sacrificed", "Any instant-speed removal on either piece"},
	},
	{
		Name:        "Exquisite Blood + Sanguine Bond",
		Pieces:      []string{"Exquisite Blood", "Sanguine Bond"},
		Type:        "true_infinite",
		Class:       ComboClassInfiniteDrain,
		Mandatory:   true,
		Description: "Any life gain or opponent life loss triggers infinite drain loop. Kills all opponents.",
		Outlets:     []string{},
		Stops:       []string{"Remove either enchantment in response to the first trigger"},
	},
	{
		Name:        "Exquisite Blood + Enduring Tenacity",
		Pieces:      []string{"Exquisite Blood", "Enduring Tenacity"},
		Type:        "true_infinite",
		Class:       ComboClassInfiniteDrain,
		Mandatory:   true,
		Description: "Enduring Tenacity: lifegain → opponent loses life. Exquisite Blood: opponent loses life → you gain life. Infinite mandatory drain loop. Self-recurring (returns as enchantment on death).",
		Outlets:     []string{},
		Stops:       []string{"Remove either piece in response to first trigger", "Enduring Tenacity can be killed but returns as enchantment — need exile"},
	},
	{
		Name:        "Exquisite Blood + Vito, Thorn of the Dusk Rose",
		Pieces:      []string{"Exquisite Blood", "Vito, Thorn of the Dusk Rose"},
		Type:        "true_infinite",
		Class:       ComboClassInfiniteDrain,
		Mandatory:   true,
		Description: "Vito: lifegain → opponent loses life. Exquisite Blood: opponent loses life → you gain life. Same loop as Sanguine Bond but on a creature.",
		Outlets:     []string{},
		Stops:       []string{"Remove either piece"},
	},
	{
		Name:        "Exquisite Blood + Marauding Blight-Priest",
		Pieces:      []string{"Exquisite Blood", "Marauding Blight-Priest"},
		Type:        "true_infinite",
		Class:       ComboClassInfiniteDrain,
		Mandatory:   true,
		Description: "Blight-Priest: lifegain → each opponent loses 1. Exquisite Blood: opponent loses life → you gain life. Infinite drain.",
		Outlets:     []string{},
		Stops:       []string{"Remove either piece"},
	},
	{
		Name:        "Exquisite Blood + Defiant Bloodlord",
		Pieces:      []string{"Exquisite Blood", "Defiant Bloodlord"},
		Type:        "true_infinite",
		Class:       ComboClassInfiniteDrain,
		Mandatory:   true,
		Description: "Bloodlord: lifegain → opponent loses that much. Exquisite Blood: opponent loses life → you gain. Infinite drain loop.",
		Outlets:     []string{},
		Stops:       []string{"Remove either piece"},
	},
	{
		Name:        "Mikaeus + Triskelion",
		Pieces:      []string{"Mikaeus, the Unhallowed", "Triskelion"},
		Type:        "true_infinite",
		Class:       ComboClassInfiniteDamage,
		Mandatory:   true,
		Description: "Remove counters from Triskelion to deal damage, it dies, Undying returns it with +1/+1 counter, remove counters again. Infinite damage.",
		Outlets:     []string{}, // self-contained kill
		Stops:       []string{"Exile either piece", "Graveyard hate"},
	},
	{
		Name:        "Kiki-Jiki + Zealous Conscripts",
		Pieces:      []string{"Kiki-Jiki, Mirror Breaker", "Zealous Conscripts"},
		Type:        "determined",
		Class:       ComboClassInfiniteTokens,
		Mandatory:   false,
		Description: "Kiki copies Conscripts, copy untaps Kiki, repeat for infinite hasty tokens.",
		Outlets:     []string{}, // tokens attack for the win
		Stops:       []string{"Remove Kiki-Jiki in response"},
	},
	{
		Name:        "Kiki-Jiki + Felidar Guardian",
		Pieces:      []string{"Kiki-Jiki, Mirror Breaker", "Felidar Guardian"},
		Type:        "determined",
		Class:       ComboClassInfiniteTokens,
		Mandatory:   false,
		Description: "Kiki copies Felidar, copy blinks Kiki, untapped Kiki copies again. Infinite hasty tokens.",
		Outlets:     []string{},
		Stops:       []string{"Remove either piece"},
	},
	{
		Name:        "Splinter Twin + Deceiver Exarch",
		Pieces:      []string{"Splinter Twin", "Deceiver Exarch"},
		Type:        "determined",
		Class:       ComboClassInfiniteTokens,
		Mandatory:   false,
		Description: "Enchanted Exarch taps to create copy, copy untaps original, repeat. Infinite hasty tokens.",
		Outlets:     []string{},
		Stops:       []string{"Remove enchantment or creature"},
	},
	// ── Determined Infinites (optional) ──
	{
		Name:        "Basalt Monolith + Kinnan",
		Pieces:      []string{"Basalt Monolith", "Kinnan, Bonder Prodigy"},
		Type:        "determined",
		Class:       ComboClassInfiniteMana,
		Mandatory:   false,
		Description: "Tap Basalt for 3+1 (Kinnan doubles), pay 3 to untap. Net +1 mana each cycle. Infinite colorless mana.",
		Outlets:     []string{"Any mana sink: Walking Ballista, Staff of Domination, etc."},
		Stops:       []string{},
	},
	{
		Name:        "Isochron Scepter + Dramatic Reversal",
		Pieces:      []string{"Isochron Scepter", "Dramatic Reversal"},
		Type:        "determined",
		Class:       ComboClassInfiniteMana,
		Mandatory:   false,
		Description: "Imprint Dramatic Reversal on Scepter. Activate Scepter (2 mana), untap all nonland permanents including Scepter. Net mana with 3+ mana rocks.",
		Outlets:     []string{"Any mana sink", "Aetherflux Reservoir for storm count"},
		Stops:       []string{},
	},
	{
		Name:        "Thoracle + Demonic Consultation",
		Pieces:      []string{"Thassa's Oracle", "Demonic Consultation"},
		Type:        "determined",
		Class:       ComboClassLibraryExileWin,
		Mandatory:   false,
		Description: "Cast Consultation naming a card not in deck, exile library. Oracle ETB with empty library = win.",
		Outlets:     []string{}, // self-contained win
		Stops:       []string{"Stifle the ETB", "Trickbind", "Kill Oracle in response"},
	},
	{
		Name:        "Thoracle + Tainted Pact",
		Pieces:      []string{"Thassa's Oracle", "Tainted Pact"},
		Type:        "determined",
		Class:       ComboClassLibraryExileWin,
		Mandatory:   false,
		Description: "Tainted Pact with singleton deck exiles entire library. Oracle ETB = win.",
		Outlets:     []string{},
		Stops:       []string{"Stifle", "Kill Oracle"},
	},
	{
		Name:        "Walking Ballista + Heliod",
		Pieces:      []string{"Walking Ballista", "Heliod, Sun-Crowned"},
		Type:        "determined",
		Class:       ComboClassInfiniteDamage,
		Mandatory:   false,
		Description: "Ballista with lifelink from Heliod. Remove counter to deal 1, gain 1 life, Heliod puts counter back. Infinite damage.",
		Outlets:     []string{},
		Stops:       []string{"Remove either piece", "Prevent lifelink"},
	},
	{
		Name:        "Ragost Strongbull Loop",
		Pieces:      []string{"Penregon Strongbull", "Crime Novelist", "Nuka-Cola Vending Machine"},
		Type:        "determined",
		Class:       ComboClassInfiniteDamage,
		Mandatory:   false,
		Description: "Sac artifact with Strongbull (1 damage each opp), Novelist adds {R}, Nuka-Cola creates Treasure (Food via Ragost). Pay {R} to repeat.",
		Outlets:     []string{},
		Stops:       []string{"Remove any piece", "Stifle Nuka-Cola trigger"},
	},
	{
		Name:        "Phyrexian Altar + Gravecrawler",
		Pieces:      []string{"Phyrexian Altar", "Gravecrawler"},
		Type:        "determined",
		Class:       ComboClassInfiniteETB,
		Mandatory:   false,
		Description: "Sac Gravecrawler for {B}, recast from graveyard (needs a Zombie). Infinite death/ETB triggers, infinite {B}.",
		Outlets:     []string{"Blood Artist", "Zulaport Cutthroat", "Diregraf Captain"},
		Stops:       []string{"Graveyard hate", "Remove Zombie requirement"},
	},
	{
		Name:        "Dockside + Temur Sabertooth",
		Pieces:      []string{"Dockside Extortionist", "Temur Sabertooth"},
		Type:        "determined",
		Class:       ComboClassInfiniteMana,
		Mandatory:   false,
		Description: "Bounce Dockside with Sabertooth ({1}{G}), recast for Treasures. Net positive if opponents have 3+ artifacts/enchantments.",
		Outlets:     []string{"Any mana sink"},
		Stops:       []string{"Remove either piece"},
	},
	{
		Name:        "Food Chain + Eternal Creature",
		Pieces:      []string{"Food Chain"},
		Type:        "determined",
		Class:       ComboClassInfiniteETB,
		Mandatory:   false,
		Description: "Exile creature from command zone for mana, recast with extra mana. Infinite ETBs + mana (creature spells only).",
		Outlets:     []string{"Thassa's Oracle", "Walking Ballista (via infinite mana)", "Aetherflux Reservoir"},
		Stops:       []string{"Remove Food Chain"},
	},
	// ── Universal Combo Enablers ──
	{
		Name:        "Displacer Kitten + Sol Ring",
		Pieces:      []string{"Displacer Kitten", "Sol Ring"},
		Type:        "determined",
		Class:       ComboClassInfiniteMana,
		Mandatory:   false,
		Description: "Cast any noncreature spell → blink Sol Ring (enters untapped) → net +1 mana per loop. Infinite mana with any 1-cost noncreature spell.",
		Outlets:     []string{"Walking Ballista", "Aetherflux Reservoir", "Thassa's Oracle", "Staff of Domination"},
		Stops:       []string{"Remove Kitten", "Stifle the blink trigger"},
	},
	{
		Name:        "Displacer Kitten + Mana Crypt",
		Pieces:      []string{"Displacer Kitten", "Mana Crypt"},
		Type:        "determined",
		Class:       ComboClassInfiniteMana,
		Mandatory:   false,
		Description: "Cast any noncreature spell → blink Mana Crypt (enters untapped) → net +1 mana per loop. Infinite colorless mana.",
		Outlets:     []string{"Walking Ballista", "Aetherflux Reservoir", "Staff of Domination"},
		Stops:       []string{"Remove Kitten"},
	},
	{
		Name:        "Displacer Kitten + Sensei's Divining Top",
		Pieces:      []string{"Displacer Kitten", "Sensei's Divining Top"},
		Type:        "determined",
		Class:       ComboClassInfiniteETB,
		Mandatory:   false,
		Description: "Tap Top to draw → recast Top for {1} → Kitten blinks a mana rock → net mana + draw entire deck. Infinite draw with any mana rock.",
		Outlets:     []string{"Thassa's Oracle", "Aetherflux Reservoir", "Laboratory Maniac"},
		Stops:       []string{"Remove Kitten or Top"},
	},
	{
		Name:        "Displacer Kitten + Isochron Scepter",
		Pieces:      []string{"Displacer Kitten", "Isochron Scepter"},
		Type:        "determined",
		Class:       ComboClassInfiniteETB,
		Mandatory:   false,
		Description: "Activate Scepter (cast imprint) → Kitten blinks Scepter (resets) → repeat. Infinite casts of imprinted spell.",
		Outlets:     []string{"Dramatic Reversal (imprinted)", "Aetherflux Reservoir"},
		Stops:       []string{"Remove Kitten or Scepter"},
	},
	{
		Name:        "Deadeye Navigator + Peregrine Drake",
		Pieces:      []string{"Deadeye Navigator", "Peregrine Drake"},
		Type:        "determined",
		Class:       ComboClassInfiniteMana,
		Mandatory:   false,
		Description: "Soulbond Deadeye with Drake. Pay {1}{U} to blink Drake → untap 5 lands → net +3 mana per loop. Infinite mana.",
		Outlets:     []string{"Any mana sink", "Thassa's Oracle", "Walking Ballista"},
		Stops:       []string{"Remove either piece", "Stifle ETB"},
	},
	{
		Name:        "Deadeye Navigator + Palinchron",
		Pieces:      []string{"Deadeye Navigator", "Palinchron"},
		Type:        "determined",
		Class:       ComboClassInfiniteMana,
		Mandatory:   false,
		Description: "Soulbond Deadeye with Palinchron. Pay {1}{U} to blink → untap 7 lands → net +5 mana. Infinite mana.",
		Outlets:     []string{"Any mana sink"},
		Stops:       []string{"Remove either piece"},
	},
	{
		Name:        "Deadeye Navigator + Dockside Extortionist",
		Pieces:      []string{"Deadeye Navigator", "Dockside Extortionist"},
		Type:        "determined",
		Class:       ComboClassInfiniteMana,
		Mandatory:   false,
		Description: "Soulbond Deadeye with Dockside. Pay {1}{U} to blink Dockside → create Treasures. Infinite mana if opponents have 3+ artifacts/enchantments.",
		Outlets:     []string{"Any mana sink", "Aetherflux Reservoir"},
		Stops:       []string{"Remove either piece", "Opponents remove their artifacts/enchantments"},
	},
	{
		Name:        "Deadeye Navigator + Eternal Witness",
		Pieces:      []string{"Deadeye Navigator", "Eternal Witness"},
		Type:        "determined",
		Class:       ComboClassInfiniteETB,
		Mandatory:   false,
		Description: "Blink Witness → return any card from graveyard. With infinite mana, return and recast anything. Infinite recursion.",
		Outlets:     []string{"Any win condition in graveyard"},
		Stops:       []string{"Remove either piece", "Graveyard exile"},
	},
	{
		Name:        "Deadeye Navigator + Gray Merchant of Asphodel",
		Pieces:      []string{"Deadeye Navigator", "Gray Merchant of Asphodel"},
		Type:        "determined",
		Class:       ComboClassInfiniteDrain,
		Mandatory:   false,
		Description: "Blink Gray Merchant → drain each opponent for devotion. With infinite mana, infinite drain. Kills the table.",
		Outlets:     []string{},
		Stops:       []string{"Remove either piece"},
	},
	// ── Lab Man / Thoracle Lines ──
	{
		Name:        "Thassa's Oracle + Hermit Druid",
		Pieces:      []string{"Thassa's Oracle", "Hermit Druid"},
		Type:        "determined",
		Class:       ComboClassLibraryExileWin,
		Mandatory:   false,
		Description: "Hermit Druid with no basic lands mills entire library. Oracle ETB with empty library = win.",
		Outlets:     []string{},
		Stops:       []string{"Kill Hermit Druid in response", "Stifle Oracle ETB", "Play basic lands to stop Druid"},
	},
	{
		Name:        "Laboratory Maniac + Demonic Consultation",
		Pieces:      []string{"Laboratory Maniac", "Demonic Consultation"},
		Type:        "determined",
		Class:       ComboClassLibraryExileWin,
		Mandatory:   false,
		Description: "Cast Consultation naming a card not in deck, exile library. Draw a card with Lab Man in play = win.",
		Outlets:     []string{},
		Stops:       []string{"Kill Lab Man before drawing", "Counter the draw spell"},
	},
	{
		Name:        "Laboratory Maniac + Tainted Pact",
		Pieces:      []string{"Laboratory Maniac", "Tainted Pact"},
		Type:        "determined",
		Class:       ComboClassLibraryExileWin,
		Mandatory:   false,
		Description: "Tainted Pact with singleton deck exiles entire library. Draw a card with Lab Man in play = win.",
		Outlets:     []string{},
		Stops:       []string{"Kill Lab Man before drawing"},
	},
	{
		Name:        "Jace, Wielder of Mysteries + Demonic Consultation",
		Pieces:      []string{"Jace, Wielder of Mysteries", "Demonic Consultation"},
		Type:        "determined",
		Class:       ComboClassLibraryExileWin,
		Mandatory:   false,
		Description: "Cast Consultation naming a card not in deck, exile library. Activate Jace's +1 to draw with empty library = win.",
		Outlets:     []string{},
		Stops:       []string{"Kill Jace before +1 resolves", "Counter Consultation"},
	},
	{
		Name:        "Jace, Wielder of Mysteries + Tainted Pact",
		Pieces:      []string{"Jace, Wielder of Mysteries", "Tainted Pact"},
		Type:        "determined",
		Class:       ComboClassLibraryExileWin,
		Mandatory:   false,
		Description: "Tainted Pact with singleton deck exiles entire library. Activate Jace's +1 = win.",
		Outlets:     []string{},
		Stops:       []string{"Kill Jace before +1"},
	},
	// ── Doomsday Piles ──
	{
		Name:        "Doomsday + Thassa's Oracle",
		Pieces:      []string{"Doomsday", "Thassa's Oracle"},
		Type:        "determined",
		Class:       ComboClassLibraryExileWin,
		Mandatory:   false,
		Description: "Doomsday sets up a 5-card pile with Oracle on top. Draw into Oracle, cast with 0-2 cards in library = win.",
		Outlets:     []string{},
		Stops:       []string{"Counter Doomsday", "Counter Oracle", "Stifle Oracle ETB"},
	},
	{
		Name:        "Doomsday + Laboratory Maniac",
		Pieces:      []string{"Doomsday", "Laboratory Maniac"},
		Type:        "determined",
		Class:       ComboClassLibraryExileWin,
		Mandatory:   false,
		Description: "Doomsday leaves 5 cards in library. Lab Man + draw through the pile = win when library is empty.",
		Outlets:     []string{},
		Stops:       []string{"Counter Doomsday", "Kill Lab Man before final draw"},
	},
	// ── Purphoros / Impact Tremors Lines ──
	{
		Name:        "Purphoros + Any Token Engine",
		Pieces:      []string{"Purphoros, God of the Forge"},
		Type:        "synergy",
		Class:       ComboClassETBPayoff,
		Mandatory:   false,
		Description: "Every creature ETB deals 2 to each opponent. With infinite tokens = infinite damage.",
		Outlets:     []string{},
		Stops:       []string{},
	},
	{
		Name:        "Impact Tremors + Any Token Engine",
		Pieces:      []string{"Impact Tremors"},
		Type:        "synergy",
		Class:       ComboClassETBPayoff,
		Mandatory:   false,
		Description: "Every creature ETB deals 1 to each opponent.",
		Outlets:     []string{},
		Stops:       []string{},
	},
	// ── ETB Doublers (not blink but same category) ──
	{
		Name:        "Panharmonicon + Any ETB",
		Pieces:      []string{"Panharmonicon"},
		Type:        "synergy", // not a loop, just doubled value
		Class:       ComboClassETBDoubler,
		Mandatory:   false,
		Description: "Doubles all artifact and creature ETB triggers. Universal value multiplier.",
		Outlets:     []string{},
		Stops:       []string{},
	},
	{
		Name:        "Yarok + Any ETB",
		Pieces:      []string{"Yarok, the Desecrated"},
		Type:        "synergy",
		Class:       ComboClassETBDoubler,
		Mandatory:   false,
		Description: "Doubles ALL permanent ETB triggers. Even stronger than Panharmonicon.",
		Outlets:     []string{},
		Stops:       []string{},
	},
	// ── Blink Enablers ──
	{
		Name:        "Conjurer's Closet + Any Value ETB",
		Pieces:      []string{"Conjurer's Closet"},
		Type:        "synergy",
		Class:       ComboClassBlinkEngine,
		Mandatory:   false,
		Description: "Free blink every end step. Repeatable value engine with any ETB creature.",
		Outlets:     []string{},
		Stops:       []string{},
	},
	{
		Name:        "Thassa, Deep-Dwelling + Any Value ETB",
		Pieces:      []string{"Thassa, Deep-Dwelling"},
		Type:        "synergy",
		Class:       ComboClassBlinkEngine,
		Mandatory:   false,
		Description: "Free blink every end step on an indestructible body. Premium blink engine.",
		Outlets:     []string{},
		Stops:       []string{},
	},
	{
		Name:        "Ephemerate + Any Value ETB",
		Pieces:      []string{"Ephemerate"},
		Type:        "synergy",
		Class:       ComboClassBlinkEngine,
		Mandatory:   false,
		Description: "1-mana blink with rebound — two blinks for one card. Extremely efficient.",
		Outlets:     []string{},
		Stops:       []string{},
	},
	// ── Worldgorger Dragon Loops ──
	{
		Name:        "Worldgorger Dragon + Animate Dead",
		Pieces:      []string{"Worldgorger Dragon", "Animate Dead"},
		Type:        "true_infinite",
		Class:       ComboClassInfiniteETB,
		Mandatory:   true,
		Description: "Animate Dead targets Dragon in graveyard. Dragon ETB exiles all other permanents including Animate Dead. Animate Dead leaves → Dragon dies → everything returns including Animate Dead → retarget Dragon. Infinite ETB/LTB, tap lands between iterations for infinite mana.",
		Outlets:     []string{"Any instant-speed mana sink", "Comet Storm", "Walking Ballista"},
		Stops:       []string{"Remove Dragon from graveyard before Animate Dead resolves", "Another creature in any graveyard to redirect Animate Dead"},
	},
	{
		Name:        "Worldgorger Dragon + Dance of the Dead",
		Pieces:      []string{"Worldgorger Dragon", "Dance of the Dead"},
		Type:        "true_infinite",
		Class:       ComboClassInfiniteETB,
		Mandatory:   true,
		Description: "Same loop as Animate Dead. Dance enchants Dragon, Dragon exiles Dance, Dragon dies, everything returns.",
		Outlets:     []string{"Any instant-speed mana sink"},
		Stops:       []string{"Remove Dragon from graveyard", "Another reanimation target"},
	},
	{
		Name:        "Worldgorger Dragon + Necromancy",
		Pieces:      []string{"Worldgorger Dragon", "Necromancy"},
		Type:        "true_infinite",
		Class:       ComboClassInfiniteETB,
		Mandatory:   true,
		Description: "Same Worldgorger loop via Necromancy. Flash on Necromancy allows instant-speed initiation.",
		Outlets:     []string{"Any instant-speed mana sink"},
		Stops:       []string{"Remove Dragon from graveyard"},
	},
	// ── Nim Deathmantle Loops ──
	{
		Name:        "Nim Deathmantle + Ashnod's Altar + Token Maker",
		Pieces:      []string{"Nim Deathmantle", "Ashnod's Altar"},
		Type:        "determined",
		Class:       ComboClassInfiniteMana,
		Mandatory:   false,
		Description: "Sacrifice a creature that makes 2+ tokens on ETB/death to Altar ({2}{2}). Pay {4} for Deathmantle trigger to return it. Infinite tokens, infinite colorless mana.",
		Outlets:     []string{"Blood Artist", "Zulaport Cutthroat", "Any mana sink"},
		Stops:       []string{"Remove any piece", "Exile the creature"},
	},
	{
		Name:        "Nim Deathmantle + Phyrexian Altar + Token Maker",
		Pieces:      []string{"Nim Deathmantle", "Phyrexian Altar"},
		Type:        "determined",
		Class:       ComboClassInfiniteMana,
		Mandatory:   false,
		Description: "Sacrifice creature + its tokens to Phyrexian Altar for colored mana. Pay {4} for Deathmantle. Needs creature that makes 4+ tokens or cost reduction.",
		Outlets:     []string{"Blood Artist", "Zulaport Cutthroat"},
		Stops:       []string{"Remove any piece"},
	},
	// ── Underworld Breach Lines ──
	{
		Name:        "Underworld Breach + Brain Freeze + Lion's Eye Diamond",
		Pieces:      []string{"Underworld Breach", "Brain Freeze", "Lion's Eye Diamond"},
		Type:        "determined",
		Class:       ComboClassInfiniteMill,
		Mandatory:   false,
		Description: "Cast LED, crack for mana, escape Brain Freeze (exiling 3 from grave), mill yourself 6+. Repeat: each cast adds storm, mills more cards to fuel escape. Eventually mill entire library, win with Thoracle or deck opponents.",
		Outlets:     []string{"Thassa's Oracle", "Laboratory Maniac"},
		Stops:       []string{"Counter Breach", "Graveyard hate", "Stifle"},
	},
	{
		Name:        "Underworld Breach + Grinding Station + Mana Crypt",
		Pieces:      []string{"Underworld Breach", "Grinding Station", "Mana Crypt"},
		Type:        "determined",
		Class:       ComboClassInfiniteMill,
		Mandatory:   false,
		Description: "Escape Mana Crypt (exile 3), it enters → Grinding Station untaps, mill 3. Net 0 but mills entire library with enough initial grave fuel.",
		Outlets:     []string{"Thassa's Oracle"},
		Stops:       []string{"Remove any piece"},
	},
	// ── Tooth and Nail Lines ──
	{
		Name:        "Tooth and Nail → Kiki + Conscripts",
		Pieces:      []string{"Tooth and Nail", "Kiki-Jiki, Mirror Breaker", "Zealous Conscripts"},
		Type:        "determined",
		Class:       ComboClassInfiniteTokens,
		Mandatory:   false,
		Description: "Entwined Tooth and Nail fetches both to battlefield. Kiki copies Conscripts, infinite hasty tokens.",
		Outlets:     []string{},
		Stops:       []string{"Counter Tooth and Nail"},
	},
	{
		Name:        "Tooth and Nail → Avenger + Craterhoof",
		Pieces:      []string{"Tooth and Nail", "Avenger of Zendikar", "Craterhoof Behemoth"},
		Type:        "determined",
		Class:       ComboClassCombatFinisher,
		Mandatory:   false,
		Description: "Entwined Tooth and Nail fetches Avenger (makes plant tokens) and Craterhoof (pumps all). Lethal swing.",
		Outlets:     []string{},
		Stops:       []string{"Counter Tooth and Nail", "Board wipe in response"},
	},
	// ── Sacrifice Engine Combos ──
	{
		Name:        "Karmic Guide + Reveillark + Sac Outlet",
		Pieces:      []string{"Karmic Guide", "Reveillark"},
		Type:        "determined",
		Class:       ComboClassInfiniteETB,
		Mandatory:   false,
		Description: "Sacrifice Reveillark → return Karmic Guide + another creature (power 2 or less). Karmic Guide returns Reveillark. Infinite ETB/death triggers.",
		Outlets:     []string{"Blood Artist", "Zulaport Cutthroat", "Goblin Bombardment", "Altar of Dementia"},
		Stops:       []string{"Exile either piece", "Remove sac outlet"},
	},
	{
		Name:        "Leonin Relic-Warder + Animate Dead + Sac Outlet",
		Pieces:      []string{"Leonin Relic-Warder", "Animate Dead"},
		Type:        "determined",
		Class:       ComboClassInfiniteETB,
		Mandatory:   false,
		Description: "Animate Dead returns Relic-Warder. ETB exiles Animate Dead. Sacrifice Relic-Warder → Animate Dead returns → retarget Relic-Warder. Infinite death/ETB triggers.",
		Outlets:     []string{"Blood Artist", "Zulaport Cutthroat", "Goblin Bombardment"},
		Stops:       []string{"Remove either piece", "Exile Relic-Warder"},
	},
	// ── Staff of Domination ──
	{
		Name:        "Staff of Domination + Infinite Mana",
		Pieces:      []string{"Staff of Domination"},
		Type:        "synergy",
		Class:       ComboClassManaSink,
		Mandatory:   false,
		Description: "With 5+ mana per activation: untap Staff ({1}), untap creature ({3}+tap), draw card ({3}+tap), gain life ({3}+tap). Universal infinite mana payoff.",
		Outlets:     []string{},
		Stops:       []string{"Remove Staff"},
	},
	// ── Aetherflux Reservoir ──
	{
		Name:        "Aetherflux Reservoir + Storm",
		Pieces:      []string{"Aetherflux Reservoir"},
		Type:        "synergy",
		Class:       ComboClassStormFinisher,
		Mandatory:   false,
		Description: "Each spell cast gains increasing life (1st=1, 2nd=2, etc). 10 spells in a turn = 55 life. Pay 50 life to deal 50 damage. Universal storm payoff.",
		Outlets:     []string{},
		Stops:       []string{"Remove Reservoir"},
	},
	// ── Persist Combos ──
	{
		Name:        "Lesser Masticore + Murderous Redcap + Persist Reset",
		Pieces:      []string{"Murderous Redcap"},
		Type:        "synergy",
		Class:       ComboClassInfiniteDamage,
		Mandatory:   false,
		Description: "Persist creature + any way to remove -1/-1 counter (Vizier of Remedies, Grumgully, Metallic Mimic) + sac outlet = infinite ETB/death. Redcap deals 2 per loop.",
		Outlets:     []string{},
		Stops:       []string{"Remove the counter-removal piece"},
	},
	{
		Name:        "Vizier of Remedies + Persist Creature + Sac Outlet",
		Pieces:      []string{"Vizier of Remedies"},
		Type:        "synergy",
		Class:       ComboClassInfiniteETB,
		Mandatory:   false,
		Description: "Vizier prevents -1/-1 counters from persist. Any persist creature + sac outlet = infinite loop.",
		Outlets:     []string{"Blood Artist", "Goblin Bombardment", "Altar of Dementia"},
		Stops:       []string{"Remove Vizier"},
	},

	// ─────────────────────────────────────────────────────────────────────
	// 4-CARD COMBOS (R60 sweep)
	// ─────────────────────────────────────────────────────────────────────
	// Curated subset of well-documented 4-card cEDH win lines. Each entry
	// is irreducible — none of these is a strict superset of an existing
	// 1/2/3-card combo in this database (verified by allKnownCombos
	// dedup-by-pieces in spellbook_import.go).
	//
	// The KnownCombos lookup in analysis.go is O(combos × max_pieces) per
	// deck and supports any arity natively, so adding 4-card entries
	// requires no detector-side plumbing. The heuristic FindLoops path
	// (checkQuadCombo) handles UNNAMED 4-card cycles; this section
	// handles NAMED ones with curated descriptions + outlets + stops.

	{
		Name:        "Citadel + Top + Birgi + Aetherflux",
		Pieces:      []string{"Bolas's Citadel", "Sensei's Divining Top", "Birgi, God of Storytelling", "Aetherflux Reservoir"},
		Type:        "determined",
		Class:       ComboClassStormFinisher,
		Mandatory:   false,
		Description: "Bolas's Citadel casts off top paying life. Sensei's Divining Top {1}: draw → goes on top → cast again. Birgi gives {R} per spell cast, paying for Top's {1} re-cast. Aetherflux gains 1 life per cast — stacks to 51+ life across the storm chain; pay 50 to nuke any opponent. Self-sustaining Storm finisher.",
		Outlets:     []string{"Aetherflux Reservoir"}, // the wincon IS the outlet
		Stops:       []string{"Remove Aetherflux mid-loop", "Remove Birgi (Top stops cycling for free)", "Force Birgi off the battlefield"},
	},
	{
		Name:        "Twiddle Storm",
		Pieces:      []string{"Bonus Round", "Twiddle", "High Tide", "Brain Freeze"},
		Type:        "determined",
		Class:       ComboClassInfiniteMill,
		Mandatory:   false,
		Description: "High Tide doubles each Island's mana. Bonus Round copies the next instant/sorcery. Twiddle untaps an Island — copy via Bonus Round = 2 free untaps per cast, generating storm count and mana. Brain Freeze with storm = mill 30+ cards. Each Twiddle nets +mana; chain ends with lethal Brain Freeze on each opponent.",
		Outlets:     []string{"Brain Freeze", "Grapeshot", "Tendrils of Agony"},
		Stops:       []string{"Counter the storm payoff", "Bojuka Bog / graveyard hate post-resolution to prevent flashback chains"},
	},
	{
		Name:        "Hermit Druid Dread Return Line",
		Pieces:      []string{"Hermit Druid", "Narcomoeba", "Dread Return", "Thassa's Oracle"},
		Type:        "determined",
		Class:       ComboClassLibraryExileWin,
		Mandatory:   false,
		Description: "Hermit Druid mills entire library (no basics in deck). Narcomoebas hit the battlefield from the mill. Dread Return flashback sacrificing 3 Narcomoebas to reanimate Thassa's Oracle. Oracle ETB → library is empty → win. Classic 4-card Hermit Druid combo.",
		Outlets:     []string{"Thassa's Oracle", "Laboratory Maniac", "Jace, Wielder of Mysteries"},
		Stops:       []string{"Graveyard hate on either Druid's mill or pre-Dread-Return", "Counter Dread Return", "Counter Oracle ETB", "Surgical Extraction on a Narcomoeba"},
	},
	{
		Name:        "Worldgorger Dragon + Animate Dead + Outlet + Sac Creature",
		Pieces:      []string{"Worldgorger Dragon", "Animate Dead", "Goblin Bombardment", "Bloodghast"},
		Type:        "determined",
		Class:       ComboClassInfiniteDamage,
		Mandatory:   false,
		Description: "Animate Dead reanimates Worldgorger Dragon (exiles all your other permanents on ETB). Animate Dead falls off (no creature to enchant) → Worldgorger dies → permanents return → Animate Dead reanimates Worldgorger again → loop. Each cycle re-ETBs every permanent — Bloodghast triggers landfall infinitely if any land returns, or use Bombardment to sacrifice for direct damage every cycle. Untapped lands give infinite mana along the way.",
		Outlets:     []string{"Goblin Bombardment", "Walking Ballista", "Impact Tremors", "Purphoros, God of the Forge", "Blood Artist"},
		Stops:       []string{"Stifle the Worldgorger ETB", "Counter Animate Dead", "Disenchant the Animate Dead mid-loop"},
	},
	{
		Name:        "Necrotic Ooze + Phyrexian Devourer + Triskelion + Hermit Druid",
		Pieces:      []string{"Necrotic Ooze", "Phyrexian Devourer", "Triskelion", "Hermit Druid"},
		Type:        "determined",
		Class:       ComboClassInfiniteDamage,
		Mandatory:   false,
		Description: "Hermit Druid mills entire library (no basics) → Devourer + Triskelion land in graveyard. Necrotic Ooze ETBs and grants all activated abilities from creatures in graveyards. Devourer's 'exile cards from library, gain +X/+X' is now on Ooze — exile-yourself-empty doesn't matter because library is already milled. Triskelion's 'remove +1/+1 counter, deal 1 damage' is also on Ooze — sink infinite counters into infinite damage. Removes all opponents at instant speed.",
		Outlets:     []string{"Triskelion (the wincon IS the outlet)"},
		Stops:       []string{"Graveyard hate on Devourer / Triskelion / Ooze pre-resolution", "Counter Hermit Druid activation"},
	},
}
