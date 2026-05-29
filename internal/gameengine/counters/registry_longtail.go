package counters

// Counter DB Phase 5 — long-tail counter types per
// docs/counter-db-implementation-plan-r60.md §6, sourced from the Probe F
// catalog at docs/counter-types-catalog-r60.md (252 types catalogued).
//
// This file extends the registry beyond the Phase 1 + 2 + 4 set (the 10
// most common stat/storage shapes, 12 ability-granting keyword counters,
// and the 3 player-counter kinds) with the remaining ~230 entries:
// the P/T variant shapes (§704.5r non-pairing), the §306/§310/§714
// resource counters on planeswalkers / battles / Sagas (defense), the
// remaining §122.1c keyword counters (haste, shadow, decayed, phyresis,
// strike), the countdown family (fade, age) per §702.24 / §702.32, the
// §614 replacement-effect counters (finality, mannequin, incarnation,
// isolation, paralyzation, pin, echo), the alternate-win / threshold
// counters (quest, level, verse, study, tower, intervention, filibuster,
// luck), and the long-tail storage / resource / card-specific markers
// (charge-counter siblings on artifacts, lands, enchantments —
// brick / storage / hour / dream / coin / eon / point / ticket / wish /
// stash / book / page / ki / spore / ice / soul / bounty / hit / acorn /
// egg / divinity / fate / etc., and the 130+ single-card markers).
//
// Joke-set / un-set counters that have no rules function (stroopwafel,
// bargle, shoe, twenty, glass, traffic, keyword, token, spooky, milk,
// third-degree-burn, art, curse from Blue Screen of Death) are
// intentionally skipped per the design doc §10 out-of-scope list. They
// rely on generic §122 plumbing — Lookup returns nil and the engine
// degrades gracefully via ErrUnknownCounterType. The skip list is
// recorded in the comment block at the bottom of this file so future
// sweeps don't re-add them. The "everything" / "sickness" entries from
// the Probe F catalog are regex artifacts (possessive forms of
// "counters" / "summoning sickness") and are likewise skipped.
//
// Each entry's Notes carries a CR §-citation per the registry
// invariant. ValidTargets is taken from the Probe F per-card mechanic
// review. DoublingApplies follows §122.1g — true for any counter placed
// on a permanent controlled by the doubling player; false for player
// counters (engine ruling, see Phase 4 note) and lore (engine ruling at
// registry_init.go Phase 1). Proliferate is true for all §122 counters
// per §701.27 except energy (already excluded at Phase 4). Stacking
// behavior is NoPair for everything except the +1/+1 / -1/-1 family
// already wired at Phase 1 — §704.5r is the only counter-pair cancel
// in the Comprehensive Rules.

func init() {
	registerPhase5StatModifierVariants()
	registerPhase5ResourceSubstance()
	registerPhase5RemainingKeywordCounters()
	registerPhase5Countdowns()
	registerPhase5Replacements()
	registerPhase5Thresholds()
	registerPhase5MultiCardStorage()
	registerPhase5SingleCardMarkers()
}

// ---------------------------------------------------------------------------
// Helper constructors. Each fills the §122 defaults so the per-section blocks
// stay readable. Override fields after the helper returns when a counter has
// non-default semantics (e.g. defense uses PlaceEnterCondition + cost; level
// counters proliferate but don't double under §122.1g per the Doubling-Season
// historical ruling on level-up creatures).
// ---------------------------------------------------------------------------

// permanentTracker builds the canonical §122 storage-counter shape: lives on
// the listed permanent targets, doubled by §122.1g, proliferate-eligible per
// §701.27, no pair-cancel, no granted ability. Used for the bulk of the
// long-tail markers.
func permanentTracker(name string, targets []TargetType, notes string) *CounterTypeDef {
	return &CounterTypeDef{
		Name:             name,
		Category:         OtherTracker,
		ValidTargets:     targets,
		Placement:        PlaceEnterCondition | PlaceAbilityCounter | PlaceProliferateOnly,
		DoublingApplies:  true,
		Proliferate:      true,
		StackingBehavior: NoPair,
		Notes:            notes,
	}
}

// resourceMarker is the §614-driven replacement-effect family (stun-shaped):
// the counter's presence drives a replacement, removal is a cost paid by the
// replacement firing. DoublingApplies stays true so the §122.1g pipeline
// modifies how many counters land at ETB.
func resourceMarker(name string, targets []TargetType, notes string) *CounterTypeDef {
	return &CounterTypeDef{
		Name:             name,
		Category:         ResourceMarker,
		ValidTargets:     targets,
		Placement:        PlaceEnterCondition | PlaceAbilityCounter | PlaceProliferateOnly,
		DoublingApplies:  true,
		Proliferate:      true,
		StackingBehavior: NoPair,
		Notes:            notes,
	}
}

// statModifier is the §704.5r non-pairing P/T variant shape. The +1/+1 and
// -1/-1 entries already live in registry_init.go with PairsWith metadata;
// every other shape is NoPair per §704.5r ("Only +1/+1 and -1/-1 cancel").
func statModifier(name string, target TargetType, notes string) *CounterTypeDef {
	return &CounterTypeDef{
		Name:             name,
		Category:         StatModifier,
		ValidTargets:     []TargetType{target},
		Placement:        PlaceEnterCondition | PlaceAbilityCounter | PlaceProliferateOnly,
		DoublingApplies:  true,
		Proliferate:      true,
		StackingBehavior: NoPair,
		Notes:            notes,
	}
}

// keywordGrant is the §122.1c presence-grants-keyword shape. Only the
// remaining §122.1c keywords not covered by Phase 2 register here.
func keywordGrant(name, keyword, crCite string) *CounterTypeDef {
	return &CounterTypeDef{
		Name:             name,
		Category:         KeywordGrant,
		ValidTargets:     []TargetType{TargetCreature},
		Placement:        PlaceAbilityCounter | PlaceProliferateOnly,
		DoublingApplies:  true,
		Proliferate:      true,
		StackingBehavior: NoPair,
		GrantedAbility:   &AbilityRef{Keyword: keyword},
		Notes:            "CR §122.1c keyword counter; " + crCite,
	}
}

// countdown is the upkeep-ticking shape used by fade / age / time. age
// accumulates (+1 each upkeep, sacrifice on unpaid cumulative-upkeep);
// fade decrements (-1, sacrifice when can't). Both honor §122.1g doubling
// at ETB.
func countdown(name string, targets []TargetType, notes string) *CounterTypeDef {
	return &CounterTypeDef{
		Name:             name,
		Category:         TimeCounter,
		ValidTargets:     targets,
		Placement:        PlaceEnterCondition | PlaceAbilityCounter | PlaceAutoUpkeep,
		DoublingApplies:  true,
		Proliferate:      true,
		StackingBehavior: NoPair,
		Notes:            notes,
	}
}

// thresholdCounter is the discrete-effect-at-threshold shape (quest, level,
// verse, study, tower, intervention, filibuster, luck). Doubling applies per
// §122.1g, proliferate is on per §701.27, no pair-cancel.
func thresholdCounter(name string, targets []TargetType, notes string) *CounterTypeDef {
	return &CounterTypeDef{
		Name:             name,
		Category:         OtherTracker,
		ValidTargets:     targets,
		Placement:        PlaceAbilityCounter | PlaceProliferateOnly,
		DoublingApplies:  true,
		Proliferate:      true,
		StackingBehavior: NoPair,
		Notes:            notes,
	}
}

// ---------------------------------------------------------------------------
// §704.5r non-pairing P/T variants (Section A of the Probe F catalog).
// All ten extra shapes sit independently and modify P/T additively. None
// cancel against any other shape — only +1/+1 / -1/-1 do per §704.5r.
// ---------------------------------------------------------------------------

func registerPhase5StatModifierVariants() {
	registerDefinition(statModifier("+1/+0", TargetCreature,
		"CR §122 stat modifier; Clockwork Avian/Beast/Steed/Swarm battery, Balduvian Hydra"))
	registerDefinition(statModifier("+2/+2", TargetCreature,
		"CR §122 stat modifier; Baron Sengir, Autumn Willow, Brass-Talon Chimera"))
	registerDefinition(statModifier("+0/+1", TargetCreature,
		"CR §122 stat modifier; Living Armor, Coral Reef polyp output, Sacred Boon"))
	registerDefinition(statModifier("+1/+2", TargetCreature,
		"CR §122 stat modifier; Experiment Five, Armor Thrull"))
	registerDefinition(statModifier("+0/+2", TargetCreature,
		"CR §122 stat modifier; Frankenstein's Monster"))
	registerDefinition(statModifier("-0/-1", TargetCreature,
		"CR §122 stat modifier; Krovikan Plague, Lesser Werewolf, Essence Flare"))
	registerDefinition(statModifier("-0/-2", TargetCreature,
		"CR §122 stat modifier; Greater Werewolf, Spirit Shackle"))
	registerDefinition(statModifier("-1/-0", TargetCreature,
		"CR §122 stat modifier; Jabari's Influence"))
	registerDefinition(statModifier("-2/-1", TargetCreature,
		"CR §122 stat modifier; Contagion"))
	registerDefinition(statModifier("-2/-2", TargetCreature,
		"CR §122 stat modifier; Ebon Praetor"))
}

// ---------------------------------------------------------------------------
// CR §310 defense counters (battles). Defense IS the battle's resource value
// per §306.5b / §310.5; ETB at printed defense, removed 1:1 by damage per
// §310.7, defeated at 0 per §310.10. Doubling Season applies per §122.1g.
// ---------------------------------------------------------------------------

func registerPhase5ResourceSubstance() {
	registerDefinition(&CounterTypeDef{
		Name:             "defense",
		Category:         LoyaltyCounter, // resource-substance category mirror — battle's loyalty-analogue
		ValidTargets:     []TargetType{TargetBattle},
		Placement:        PlaceEnterCondition | PlaceAbilityCost,
		DoublingApplies:  true,
		Proliferate:      true,
		StackingBehavior: NoPair,
		Notes:            "CR §310.5 (battles); ETB at printed defense, §310.7 damage-removal, §310.10 defeat at 0",
	})
}

// ---------------------------------------------------------------------------
// Remaining §122.1c keyword counters (Section C). Phase 2 covered the 12
// most common; this block fills the long tail (haste, shadow, decayed,
// phyresis) plus the legacy "strike" alias. CR §122.1c grants the keyword
// while the counter is present per §122.6.
// ---------------------------------------------------------------------------

func registerPhase5RemainingKeywordCounters() {
	registerDefinition(keywordGrant("haste", "haste", "CR §702.10 haste grant"))
	registerDefinition(keywordGrant("shadow", "shadow", "CR §702.27 shadow grant (combat-block restriction)"))
	registerDefinition(keywordGrant("decayed", "decayed",
		"CR §702.146 decayed grant (can't block; sacrifice after attacking) — Rot-Curse Rakshasa counter form"))
	registerDefinition(keywordGrant("phyresis", "infect",
		"CR §702.91 infect grant — Weatherlight Compleated phyresis counter"))
	// CR §122.1c legacy "strike counter" — errata'd as "first strike counter" per
	// the Probe F catalog. Aliased so old-frame oracle text resolves correctly.
	registerDefinition(&CounterTypeDef{
		Name:             "strike",
		Aliases:          []string{"strike counter"},
		Category:         KeywordGrant,
		ValidTargets:     []TargetType{TargetCreature},
		Placement:        PlaceAbilityCounter | PlaceProliferateOnly,
		DoublingApplies:  true,
		Proliferate:      true,
		StackingBehavior: NoPair,
		GrantedAbility:   &AbilityRef{Keyword: "first strike"},
		Notes:            "CR §122.1c keyword counter; legacy alias of first strike per WotC errata",
	})
}

// ---------------------------------------------------------------------------
// CR §702.24 (cumulative upkeep / age) and §702.32 (fading). The §702.62
// (suspend) / §702.63 (vanishing) `time` counter already lives in Phase 1.
// ---------------------------------------------------------------------------

func registerPhase5Countdowns() {
	registerDefinition(countdown("fade",
		[]TargetType{TargetCreature, TargetArtifact, TargetEnchantment, TargetLand},
		"CR §702.32 (fading); ETB with N, -1 each upkeep, sacrifice when can't remove"))
	registerDefinition(countdown("age",
		[]TargetType{TargetCreature, TargetArtifact, TargetEnchantment, TargetLand},
		"CR §702.24 (cumulative upkeep); +1 each upkeep (accumulates), pay {age N} or sacrifice"))
}

// ---------------------------------------------------------------------------
// CR §614 replacement-effect counters (Section G). Each drives a §614
// replacement: would-untap → consume a stun counter, would-destroy /
// would-take-damage → consume a shield counter (Phase 1), etc. ETB with N
// per the card's printed value; §122.1g doubling applies on placement.
// ---------------------------------------------------------------------------

func registerPhase5Replacements() {
	registerDefinition(resourceMarker("finality",
		[]TargetType{TargetCreature},
		"CR §614 death-replacement; would-die → exile instead. Death-replacement counter."))
	registerDefinition(resourceMarker("mannequin",
		[]TargetType{TargetCreature},
		"CR §614 + per-card; Makeshift Mannequin sacrifice-on-targeted handle, single-use"))
	registerDefinition(resourceMarker("incarnation",
		[]TargetType{TargetPlayer},
		"CR §614 + per-card; Nine Lives damage-prevention + alt-loss at 9 incarnation counters"))
	registerDefinition(resourceMarker("isolation",
		[]TargetType{TargetEnchantment},
		"CR §614 + per-card; Quarantine Field spend-to-return-exiled-permanent"))
	registerDefinition(resourceMarker("paralyzation",
		[]TargetType{TargetCreature},
		"CR §614 + per-card; Dread Wight pre-§701.50 untap-prevention analogue"))
	registerDefinition(resourceMarker("pin",
		[]TargetType{TargetArtifact},
		"CR §614 + per-card; Voodoo Doll damage scaling on pin counter"))
	registerDefinition(resourceMarker("echo",
		[]TargetType{TargetPlayer},
		"CR §614 + per-card; Soul Echo damage-replacement removes counter (distinct from the Echo keyword cost)"))
}

// ---------------------------------------------------------------------------
// Alternate-win / threshold counters (Section F). Each triggers a discrete
// effect at a specific count. Catalog entries: quest, level, verse, study,
// tower, intervention, filibuster, luck.
//
// Level counters are special: CR §711 defines per-level brackets statically
// (P/T + abilities by count). They DO get doubled by §122.1g (Doubling Season
// + level up makes the creature level up faster) and ARE proliferate-eligible
// per §701.27.
// ---------------------------------------------------------------------------

func registerPhase5Thresholds() {
	registerDefinition(thresholdCounter("quest",
		[]TargetType{TargetEnchantment, TargetCreature},
		"CR §122; Quest cycle (Beastmaster Ascension 7, Bloodchief Ascension 4, Archmage Ascension 6) — threshold flips static effect"))
	registerDefinition(&CounterTypeDef{
		Name:             "level",
		Category:         OtherTracker,
		ValidTargets:     []TargetType{TargetCreature},
		Placement:        PlaceAbilityCounter | PlaceProliferateOnly,
		DoublingApplies:  true,
		Proliferate:      true,
		StackingBehavior: NoPair,
		Notes:            "CR §711 (level up); per-level brackets define P/T + abilities, accumulate via level-up activation",
	})
	registerDefinition(thresholdCounter("verse",
		[]TargetType{TargetEnchantment},
		"CR §122; Aria of Flame / Crescendo of War / Lilting Refrain scaling-per-count"))
	registerDefinition(thresholdCounter("study",
		[]TargetType{TargetEnchantment, TargetCreature},
		"CR §122; Class enchantment progression, Lattice Library, Vhal Eager Scholar, Imbraham"))
	registerDefinition(thresholdCounter("tower",
		[]TargetType{TargetEnchantment},
		"CR §122; Helix Pinnacle alt-win at 100 tower counters"))
	registerDefinition(thresholdCounter("intervention",
		[]TargetType{TargetEnchantment},
		"CR §122; Divine Intervention countdown from 2 → 0 = game drawn"))
	registerDefinition(thresholdCounter("filibuster",
		[]TargetType{TargetCreature},
		"CR §122; Azor's Elocutors alt-win at 5 filibuster counters"))
	registerDefinition(thresholdCounter("luck",
		[]TargetType{TargetEnchantment, TargetCreature, TargetLand},
		"CR §122; Chance Encounter (10 → win), As Luck Would Have It, Gemstone Caverns"))
}

// ---------------------------------------------------------------------------
// Long-tail multi-card storage / accumulator counters (Section H.1, ≥2 cards
// each). All follow the standard §122 plumbing: §122.6 persistence, §122.1g
// doubling at placement, §701.27 proliferate-eligible.
//
// Targets are taken from each entry's Probe F mechanic notes — when a counter
// appears on multiple permanent types across its source cards, all are
// listed in ValidTargets so the engine accepts the placement on any of them.
// ---------------------------------------------------------------------------

func registerPhase5MultiCardStorage() {
	allPerm := []TargetType{TargetCreature, TargetArtifact, TargetEnchantment, TargetLand, TargetPlaneswalker}
	artifactsOnly := []TargetType{TargetArtifact}
	creatureOnly := []TargetType{TargetCreature}
	enchOnly := []TargetType{TargetEnchantment}
	landOnly := []TargetType{TargetLand}
	artCre := []TargetType{TargetArtifact, TargetCreature}
	artEnch := []TargetType{TargetArtifact, TargetEnchantment}
	artLand := []TargetType{TargetArtifact, TargetLand}

	registerDefinition(permanentTracker("brick", artifactsOnly,
		"CR §122; Pyramid of the Pantheon / Oracle's Vault / Edifice of Authority — tap-to-add, threshold flips mode"))
	registerDefinition(permanentTracker("depletion", landOnly,
		"CR §122; Mirage storage-land cycle (Hickory Woodlot, Lava Tubes, Land Cap, Decree of Silence)"))
	registerDefinition(permanentTracker("storage", artLand,
		"CR §122; storage-land cycle (Bottomless Vault, Calciform Pools, Crucible of the Spirit Dragon, Saltcrusted Steppe)"))
	registerDefinition(permanentTracker("hour", artEnch,
		"CR §122; Midnight Clock / Midnight Oil / Rusko Clockmaker (12 → big effect)"))
	registerDefinition(permanentTracker("dream", creatureOnly,
		"CR §122; Rasputin Dreamweaver / Goliath Daydreamer — ETB with 7, spend for mana"))
	registerDefinition(permanentTracker("coin", artEnch,
		"CR §122; Athreos Shroud-Veiled / Noble's Purse / Wishing Well — activation spend"))
	registerDefinition(permanentTracker("eon", landOnly,
		"CR §122; Magosi the Waterveil / Out of the Tombs — 2 → extra turn"))
	registerDefinition(permanentTracker("point", allPerm,
		"CR §122; Strixhaven Stadium / Contested Game Ball / Brave Falconhawk threshold mechanics"))
	registerDefinition(permanentTracker("ticket", artifactsOnly,
		"CR §122; Blorbian Buddy / Ticket Bucket-Bot / Ticket Turbotubes storage"))
	registerDefinition(permanentTracker("wish", artifactsOnly,
		"CR §122; Wishclaw Talisman / Ring of Three Wishes / Djinn of Wishes — ETB with 3, activated spend"))
	registerDefinition(permanentTracker("stash", artEnch,
		"CR §122; Glittering Stockpile / Hoarder's Overflow / Tinybones Bauble Burglar"))
	registerDefinition(permanentTracker("book", artifactsOnly,
		"CR §122; Spell Satchel (and Alchemy variant)"))
	registerDefinition(permanentTracker("page", artifactsOnly,
		"CR §122; Mazemind Tome / Barrin's Codex / Autograph Book / Diary of Dreams"))
	registerDefinition(permanentTracker("ki", allPerm,
		"CR §122; Kamigawa Spirit/Arcane cast trigger adds; threshold flips"))
	registerDefinition(permanentTracker("soot", artifactsOnly,
		"CR §122; Smokestack upkeep accumulator → mass sacrifice"))
	registerDefinition(permanentTracker("gold", allPerm,
		"CR §122; counter form (distinct from Gold tokens) — Aurification, Dragon's Hoard"))
	registerDefinition(permanentTracker("treasure", artifactsOnly,
		"CR §122; counter form (distinct from Treasure tokens) — Legacy's Allure"))
	registerDefinition(permanentTracker("food", artifactsOnly,
		"CR §122; counter form (distinct from Food tokens) — rare corpus appearance"))
	registerDefinition(permanentTracker("clue", artifactsOnly,
		"CR §122; counter form (distinct from Clue tokens) — rare corpus appearance"))
	registerDefinition(permanentTracker("map", artifactsOnly,
		"CR §122; counter form (distinct from Map tokens) — rare corpus appearance"))
	registerDefinition(permanentTracker("landmark", artifactsOnly,
		"CR §122; Treasure Map flipside Treasure Cove threshold"))
	registerDefinition(permanentTracker("component", artifactsOnly,
		"CR §122; Component Pouch storage"))
	registerDefinition(permanentTracker("cube", artifactsOnly,
		"CR §122; Delif's Cube storage"))
	registerDefinition(permanentTracker("currency", artifactsOnly,
		"CR §122; Trade Caravan land-storage"))
	registerDefinition(permanentTracker("delay", artEnch,
		"CR §122; Delaying Shield / Ertai's Meddling countdown"))
	registerDefinition(permanentTracker("fuse", artifactsOnly,
		"CR §122; Powder Keg / Goblin Bomb / Bomb Squad / Incendiary countdown-to-detonate"))
	registerDefinition(permanentTracker("doom", artEnch,
		"CR §122; Armageddon Clock / Baron Von Count / Eye of Doom / Lavabrink Floodgates countdown-to-mass-effect"))
	registerDefinition(permanentTracker("omen", enchOnly,
		"CR §122; Celestial Convergence / Foreboding Statue / Soulcipher Board countdown"))
	registerDefinition(permanentTracker("flood", landOnly,
		"CR §122; Aquitect's Will / Bounty of the Luxa / Quicksilver Fountain water-cycle"))
	registerDefinition(permanentTracker("tide", creatureOnly,
		"CR §122; Homarid / Tidal Influence 1→4→reset cycle"))
	registerDefinition(permanentTracker("wind", enchOnly,
		"CR §122; Freyalise's Winds / Cyclone untap-replacement-style"))
	registerDefinition(permanentTracker("flame", artCre,
		"CR §122; Flame Channeler / Kardum / Managorger Phoenix / Naar Isle scaling"))
	registerDefinition(permanentTracker("growth", enchOnly,
		"CR §122; Simic Ascendancy alt-win at 20 / Paradox Zone / Comforting Counsel"))
	registerDefinition(permanentTracker("pressure", artLand,
		"CR §122; Magma Mine / Hellion Crucible / Exploding Barrel / Mount Keralia"))
	registerDefinition(permanentTracker("mining", landOnly,
		"CR §122; Gemstone Mine ETB with 3, tap removes, sacrifice at 0"))
	registerDefinition(permanentTracker("mire", artifactsOnly,
		"CR §122; Cyclopean Tomb land-transformation engine"))
	registerDefinition(permanentTracker("polyp", creatureOnly,
		"CR §122; Coral Reef polyp → +0/+1 counter production"))
	registerDefinition(permanentTracker("slumber", creatureOnly,
		"CR §122; Arixmethes Slumbering Isle — ETB with 5, attacks/triggers remove, 0 → 12/12"))
	registerDefinition(permanentTracker("slime", creatureOnly,
		"CR §122; Gutter Grime / Sludge Monster / Toxrill the Corrosive Ooze-token scaling"))
	registerDefinition(permanentTracker("spore", creatureOnly,
		"CR §122; Thallid family (Deathspore Thallid, Elvish Farmer, Mycologist) — upkeep adds, remove 3 → Saproling"))
	registerDefinition(permanentTracker("ice", allPerm,
		"CR §122; Iceberg / Rimefeather Owl / Draugr Necromancer / Dark Depths"))
	registerDefinition(permanentTracker("soul", artCre,
		"CR §122; Soulcatchers' Aerie / Malefic Scythe / Hostile Hostel / Netherborn Altar creature-death feed"))
	registerDefinition(permanentTracker("bounty", creatureOnly,
		"CR §122; Bounty Board / Bounty Hunter / Chevill / Mathas / Aragorn — marker, death of marked → reward"))
	registerDefinition(permanentTracker("corpse", artCre,
		"CR §122; Crowded Crypt / From the Catacombs / Isareth — graveyard tracker / sac fodder"))
	registerDefinition(permanentTracker("hit", creatureOnly,
		"CR §122; Etrata the Silencer / Mari the Killing Quill — 3 hits → exile/effect"))
	registerDefinition(permanentTracker("acorn", artifactsOnly,
		"CR §122; Chitterspitter / Acornelia / Acorn Stash storage (distinct from Acorn joke-set counter)"))
	registerDefinition(permanentTracker("egg", creatureOnly,
		"CR §122; Darigaaz Reincarnated / Xira the Golden Sting growth"))
	registerDefinition(permanentTracker("hatchling", artCre,
		"CR §122; Ludevic's Test Subject / Triassic Egg / Eumidian Hatchery threshold-to-transform"))
	registerDefinition(permanentTracker("shell", creatureOnly,
		"CR §122; Roc Hatchling growth"))
	registerDefinition(permanentTracker("pupa", creatureOnly,
		"CR §122; Cocoon growth"))
	registerDefinition(permanentTracker("tribute", creatureOnly,
		"CR §122 + ETB-choice tribute mechanic — opponent chooses to add baseline counter"))
	registerDefinition(permanentTracker("divinity", creatureOnly,
		"CR §122; Myojin cycle / Kindred Boon — ETB with 1, spend for one-time effect"))
	registerDefinition(permanentTracker("fate", artCre,
		"CR §122; Triad of Fates / Norn's Dominion / Oblivion Stone"))
	registerDefinition(permanentTracker("feather", creatureOnly,
		"CR §122; Aven Mimeomancer / Kangee Aerie Keeper / Soulcatchers' Aerie flying-tribal"))
	registerDefinition(permanentTracker("fetch", creatureOnly,
		"CR §122; Pako Arcane Retriever / Haldan Avid Arcanist / Hex Kellan's Companion fetched-card storage"))
	registerDefinition(permanentTracker("dread", creatureOnly,
		"CR §122; Grasping Shadows rare scaling"))
	registerDefinition(permanentTracker("void", creatureOnly,
		"CR §122; Dauthi Voidwalker / Sphere of Annihilation exile-tracking"))
	registerDefinition(permanentTracker("velocity", artCre,
		"CR §122; Daredevil Dragster / Tornado scaling"))
	registerDefinition(permanentTracker("infection", creatureOnly,
		"CR §122; Diseased Vermin / Festering Wound / Genestealer Patriarch Phyrexia-flavor"))
	registerDefinition(permanentTracker("plague", artEnch,
		"CR §122; Plague Boiler / Traveling Plague / Withering Hex scaling"))
	registerDefinition(permanentTracker("fungus", creatureOnly,
		"CR §122; Mindbender Spores / Sporogenesis Thallid-variant"))
	registerDefinition(permanentTracker("burden", artifactsOnly,
		"CR §122; The One Ring / A-The One Ring — upkeep adds, take damage = burden count"))
	registerDefinition(permanentTracker("cage", allPerm,
		"CR §122; Mairsil the Pretender exile-grant / Seer of the Bright Side"))
	registerDefinition(permanentTracker("collection", artifactsOnly,
		"CR §122; Charitable Levy / Evelyn the Covetous rare scaling"))
	registerDefinition(permanentTracker("net", artifactsOnly,
		"CR §122; Braided Net / Merseine rare"))
	registerDefinition(permanentTracker("death", creatureOnly,
		"CR §122; Bogardan Phoenix / Necropotence Avatar rare"))
	registerDefinition(permanentTracker("devotion", creatureOnly,
		"CR §122; Bloodthirsty Ogre / Pious Kitsune rare"))
	registerDefinition(permanentTracker("aim", artCre,
		"CR §122; Hankyu / Haphazard Bombardment rare"))
	registerDefinition(permanentTracker("blight", creatureOnly,
		"CR §122; Rottenmouth Viper / Ultima Origin of Oblivion rare"))
	registerDefinition(permanentTracker("blaze", artCre,
		"CR §122; Five-Alarm Fire / Obsidian Fireheart rare"))
	registerDefinition(permanentTracker("scream", enchOnly,
		"CR §122; All Hallow's Eve / Endless Scream threshold"))
	registerDefinition(permanentTracker("fire", artEnch,
		"CR §122; Fated Firepower / War Balloon rare"))
	registerDefinition(permanentTracker("healing", enchOnly,
		"CR §122; Fylgja / Ursine Fylgja damage-prevent allocation"))
	registerDefinition(permanentTracker("fellowship", artifactsOnly,
		"CR §122; Banner of Kinship scaling anthem"))
}

// ---------------------------------------------------------------------------
// Single-card resource / storage / mechanic markers. Each appears on exactly
// one card per the Probe F sweep. They all follow §122 generic plumbing —
// the per-card handler reads CounterCount and the engine clears them on LTB
// (§122.6 persistence holds for in-zone characteristic changes only).
//
// Targets default to the permanent type the source card belongs to. When a
// catalog entry's mechanic is unclear from the table alone, the safer choice
// is broader ValidTargets (e.g. [TargetCreature, TargetArtifact]) — overly
// strict targeting risks ErrInvalidTarget at placement time on legitimate
// per_card calls. The §704.5r invariant is unaffected either way.
// ---------------------------------------------------------------------------

func registerPhase5SingleCardMarkers() {
	cre := []TargetType{TargetCreature}
	art := []TargetType{TargetArtifact}
	ench := []TargetType{TargetEnchantment}
	land := []TargetType{TargetLand}

	// H.1 (single-card storage / marker tail)
	registerDefinition(permanentTracker("petal", ench, "CR §122; Lotus Blossom single growth-spend"))
	registerDefinition(permanentTracker("knowledge", art, "CR §122; The Magic Mirror storage/scaling"))
	registerDefinition(permanentTracker("eyeball", art, "CR §122; Jar of Eyeballs storage"))
	registerDefinition(permanentTracker("eyestalk", cre, "CR §122; Underdark Beholder storage"))
	registerDefinition(permanentTracker("bloodline", art, "CR §122; Edgar Markov's Coffin vampire-tribal"))
	registerDefinition(permanentTracker("croak", cre, "CR §122; Grolnok the Omnivore frog-tribal"))
	registerDefinition(permanentTracker("funk", ench, "CR §122; Temp of the Damned"))
	registerDefinition(permanentTracker("fury", cre, "CR §122; Charging Cinderhorn trampling-fury build-up"))
	registerDefinition(permanentTracker("harmony", art, "CR §122; Instrument of the Bards bard-tribal"))
	registerDefinition(permanentTracker("hoofprint", ench, "CR §122; Hoofprints of the Stag — 4 → 4/4 stag token"))
	registerDefinition(permanentTracker("muster", ench, "CR §122; Assemble the Legion upkeep-adds → token scale"))
	registerDefinition(permanentTracker("valor", cre, "CR §122; Intrepid Adversary champion-style scaling"))
	registerDefinition(permanentTracker("vitality", art, "CR §122; Living Artifact damage-replacement → vitality; remove for life"))
	registerDefinition(permanentTracker("vortex", ench, "CR §122; Energy Vortex damage tracker"))
	registerDefinition(permanentTracker("wage", cre, "CR §122; Rogue Skycaptain upkeep-adds threshold flips controller"))
	registerDefinition(permanentTracker("unity", ench, "CR §122; Call for Unity upkeep-adds scaling anthem"))
	registerDefinition(permanentTracker("phylactery", art, "CR §122; Phylactery Lich — 0 → die"))
	registerDefinition(permanentTracker("prey", cre, "CR §122; Tetzimoc Primal Death mark mechanic"))
	registerDefinition(permanentTracker("bait", art, "CR §122; Fishing Pole rare"))
	registerDefinition(permanentTracker("chip", art, "CR §122; B-I-N-G-O rare"))
	registerDefinition(permanentTracker("chorus", cre, "CR §122; Malcolm Alluring Scoundrel rare"))
	registerDefinition(permanentTracker("credit", cre, "CR §122; Icatian Moneychanger rare"))
	registerDefinition(permanentTracker("crystal", art, "CR §122; Prism Array rare"))
	registerDefinition(permanentTracker("cell", cre, "CR §122; Sephiroth Fallen Hero (Final Fantasy)"))
	registerDefinition(permanentTracker("brain", cre, "CR §122; Rex Cyber-Hound rare"))
	registerDefinition(permanentTracker("gem", art, "CR §122; Briber's Purse storage"))
	registerDefinition(permanentTracker("intel", cre, "CR §122; Flamewar Brash Veteran (Transformers)"))
	registerDefinition(permanentTracker("kick", cre, "CR §122; Zethi Arcane Blademaster rare"))
	registerDefinition(permanentTracker("matrix", art, "CR §122; Life Matrix rare"))
	registerDefinition(permanentTracker("memory", art, "CR §122; Altaïr Ibn-La'Ahad / The Animus rare"))
	registerDefinition(permanentTracker("nest", cre, "CR §122; Twitching Doll rare"))
	registerDefinition(permanentTracker("night", art, "CR §122; Replicating Ring rare"))
	registerDefinition(permanentTracker("ore", land, "CR §122; Orcish Mine rare"))
	registerDefinition(permanentTracker("pain", art, "CR §122; Torture Chamber rare"))
	registerDefinition(permanentTracker("palliation", ench, "CR §122; Palliation Accord rare"))
	registerDefinition(permanentTracker("pause", cre, "CR §122; Grand Marshal Macie rare"))
	registerDefinition(permanentTracker("petrification", cre, "CR §122; Xathrid Gorgon stone-counter freezes target"))
	registerDefinition(permanentTracker("plan", ench, "CR §122; Doom Reigns Supreme rare"))
	registerDefinition(permanentTracker("plot", ench, "CR §122; Deadly Designs rare"))
	registerDefinition(permanentTracker("possession", art, "CR §122; Unwilling Vessel rare"))
	registerDefinition(permanentTracker("rally", ench, "CR §122; Aligned Heart rare"))
	registerDefinition(permanentTracker("rejection", ench, "CR §122; Tolarian Contempt rare"))
	registerDefinition(permanentTracker("release", ench, "CR §122; The Heron Moon rare"))
	registerDefinition(permanentTracker("reprieve", cre, "CR §122; Magnanimous Magistrate rare"))
	registerDefinition(permanentTracker("resonance", ench, "CR §122; Fifth Stage of Magic Design rare"))
	registerDefinition(permanentTracker("rev", art, "CR §122; Chainsaw equipment vehicle-style cost"))
	registerDefinition(permanentTracker("revival", art, "CR §122; Nine-Lives Familiar rare"))
	registerDefinition(permanentTracker("ribbon", art, "CR §122; Prize Pig rare"))
	registerDefinition(permanentTracker("ritual", art, "CR §122; Heirloom Mirror rare"))
	registerDefinition(permanentTracker("scroll", land, "CR §122; Aretopolis rare"))
	registerDefinition(permanentTracker("shred", cre, "CR §122; Cephalid Vandal rare"))
	registerDefinition(permanentTracker("silver", cre, "CR §122; Karn Scion of Urza construct-token base"))
	registerDefinition(permanentTracker("skewer", art, "CR §122; Rotisserie Elemental rare"))
	registerDefinition(permanentTracker("sleep", cre, "CR §122; Venarian Gold rare"))
	registerDefinition(permanentTracker("spark", cre, "CR §122; Blood Poet rare"))
	registerDefinition(permanentTracker("spite", ench, "CR §122; Curse of Vengeance rare"))
	registerDefinition(permanentTracker("stall", cre, "CR §122; Vamping Vampire rare"))
	registerDefinition(permanentTracker("story", art, "CR §122; Staff of the Storyteller rare"))
	registerDefinition(permanentTracker("strife", ench, "CR §122; Crescendo of War rare"))
	registerDefinition(permanentTracker("supply", ench, "CR §122; Stocking the Pantry rare"))
	registerDefinition(permanentTracker("suspect", art, "CR §122; Investigator's Journal rare"))
	registerDefinition(permanentTracker("takeover", ench, "CR §122; The Master Formed Anew rare"))
	registerDefinition(permanentTracker("task", ench, "CR §122; Heliod's Punishment rare"))
	registerDefinition(permanentTracker("taste", art, "CR §122; Moth Herb Elixir rare"))
	registerDefinition(permanentTracker("theft", ench, "CR §122; Night Dealings storage; spend → tutor"))
	registerDefinition(permanentTracker("unlock", art, "CR §122; Cryptex rare"))
	registerDefinition(permanentTracker("vow", ench, "CR §122; Promise of Loyalty rare"))
	registerDefinition(permanentTracker("voyage", cre, "CR §122; Cosima God of the Voyage countdown"))
	registerDefinition(permanentTracker("winch", art, "CR §122; Mercadian Lift rare"))
	registerDefinition(permanentTracker("wreck", ench, "CR §122; Spectacle of Destruction rare"))
	registerDefinition(permanentTracker("aegis", cre, "CR §122; Livio Oathsworn Sentinel damage-prevent"))
	registerDefinition(permanentTracker("arrow", ench, "CR §122; Archery Training rare"))
	registerDefinition(permanentTracker("arrowhead", art, "CR §122; Serrated Arrows — ETB 3, remove 1 → -1/-1"))
	registerDefinition(permanentTracker("awakening", land, "CR §122; Liege of the Tangle land-animate"))
	registerDefinition(permanentTracker("blessing", ench, "CR §122; Boon of the Spirit Realm rare"))
	registerDefinition(permanentTracker("bloodstain", ench, "CR §122; Blood Spatter Analysis rare"))
	registerDefinition(permanentTracker("bore", art, "CR §122; Brass's Tunnel-Grinder rare"))
	registerDefinition(permanentTracker("bribery", cre, "CR §122; Gwafa Hazid Profiteer rare"))
	registerDefinition(permanentTracker("carrion", cre, "CR §122; Osai Vultures rare"))
	registerDefinition(permanentTracker("conqueror", cre, "CR §122; Zhao the Moon Slayer rare"))
	registerDefinition(permanentTracker("contested", ench, "CR §122; Turf War rare"))
	registerDefinition(permanentTracker("corruption", cre, "CR §122; Geyadrone Dihada rare"))
	registerDefinition(permanentTracker("day", cre, "CR §122; The Knight of Weeks rare"))
	registerDefinition(permanentTracker("descent", ench, "CR §122; Descent into Avernus rare"))
	registerDefinition(permanentTracker("despair", ench, "CR §122; Descent into Madness rare"))
	registerDefinition(permanentTracker("discovery", cre, "CR §122; Lara Croft Tomb Raider rare"))
	registerDefinition(permanentTracker("duty", ench, "CR §122; Immortal Obligation rare"))
	registerDefinition(permanentTracker("elixir", art, "CR §122; Essence Bottle rare"))
	registerDefinition(permanentTracker("ember", cre, "CR §122; Smoldering Egg rare"))
	registerDefinition(permanentTracker("enlightened", art, "CR §122; The Book of Exalted Deeds rare"))
	registerDefinition(permanentTracker("eruption", land, "CR §122; Pompeii rare"))
	registerDefinition(permanentTracker("exalted", cre, "CR §122; Emissary of Soulfire rare"))
	registerDefinition(permanentTracker("exposure", art, "CR §122; Aplan Mortarium rare"))
	registerDefinition(permanentTracker("fear", cre, "CR §122; rare corpus appearance"))
	registerDefinition(permanentTracker("feeding", cre, "CR §122; Nazar the Velvet Fang rare"))
	registerDefinition(permanentTracker("film", art, "CR §122; Peter Parker's Camera (Marvel)"))
	registerDefinition(permanentTracker("foreshadow", ench, "CR §122; Ominous Seas rare"))
	registerDefinition(permanentTracker("ghostform", []TargetType{TargetPlaneswalker}, "CR §122; Kaya the Inexorable emblem-style"))
	registerDefinition(permanentTracker("glyph", ench, "CR §122; Glyph of Delusion rare"))
	registerDefinition(permanentTracker("hope", ench, "CR §122; Dawn of a New Age rare"))
	registerDefinition(permanentTracker("hourglass", art, "CR §122; Temporal Distortion countdown"))
	registerDefinition(permanentTracker("hunger", ench, "CR §122; Fasting rare"))
	registerDefinition(permanentTracker("husk", land, "CR §122; Necropolis of Azar rare"))
	registerDefinition(permanentTracker("impostor", ench, "CR §122; Illicit Masquerade rare"))
	registerDefinition(permanentTracker("incubation", cre, "CR §122; Drake Hatcher growth"))
	registerDefinition(permanentTracker("influence", art, "CR §122; Palantír of Orthanc damage-burn per count"))
	registerDefinition(permanentTracker("ingenuity", cre, "CR §122; Jhoira Ageless Innovator rare"))
	registerDefinition(permanentTracker("invitation", ench, "CR §122; Wedding Announcement rare"))
	registerDefinition(permanentTracker("javelin", cre, "CR §122; Icatian Javelineers rare"))
	registerDefinition(permanentTracker("judgment", cre, "CR §122; Faithbound Judge rare"))
	registerDefinition(permanentTracker("magnet", art, "CR §122; Magnetic Web equipment-attach"))
	registerDefinition(permanentTracker("manifestation", cre, "CR §122; Arbiter of the Ideal rare"))
	registerDefinition(permanentTracker("mask", art, "CR §122; Illusionary Mask family hidden-info"))
	registerDefinition(permanentTracker("mine", land, "CR §122; Mine Layer rare"))
	registerDefinition(permanentTracker("hack", cre, "CR §122; Truss Chief Engineer (Acquisitions Inc)"))
	registerDefinition(permanentTracker("hole", cre, "CR §122; Impressive Rat rare"))
	registerDefinition(permanentTracker("knickknack", cre, "CR §122; Tchotchke Elemental rare"))
	registerDefinition(permanentTracker("rebuilding", cre, "CR §122; Slobad Actually Just Fine rare"))
	registerDefinition(permanentTracker("midway", cre, "CR §122; Myra the Magnificent rare"))
	registerDefinition(permanentTracker("shy", land, "CR §122; Shy Town rare"))

}

// ---------------------------------------------------------------------------
// Skipped counter types — explicit out-of-scope list per design doc §10.
//
// Joke / un-set / silver-border counters that have no rules function in the
// HexDek engine. They are intentionally NOT registered; the engine returns
// ErrUnknownCounterType on placement, which the per-card layer can no-op or
// surface to logs. Listed here so future sweeps don't re-add them and so
// auditors can see the deliberate exclusion:
//
//   - stroopwafel  (Bag of Stroopwafels — joke set)
//   - bargle       (Happy Yargle Day! — joke set)
//   - shoe         (Shoe Tree — joke set)
//   - twenty       (Randy Buehler Bio — joke set)
//   - glass        (Sky Deck — joke set)
//   - traffic      (Spaghetti Junction — joke set)
//   - keyword      (Mad Labs — joke set)
//   - token        (The Tokenator — joke set)
//   - spooky       (Sorin's Remastered Manor — joke set)
//   - milk         (Dairy Cow — joke set)
//   - third-degree-burn (Red-Hot Hottie — joke set)
//   - art          (Famous Museum — joke set)
//   - curse        (Blue Screen of Death — joke set)
//
// Regex artifacts from the Probe F oracle sweep also skipped:
//
//   - everything   (Omo Queen of Vesuva — likely possessive-form artifact)
//   - sickness     (likely "summoning sickness" verb-form artifact)
//
// "energy" is intentionally absent from the registry per CR §106.11: energy
// is a resource pool denoted {E}, NOT a §122 counter. Proliferate cannot add
// energy. The Phase 4 commit documented this exclusion in registry_init.go.
