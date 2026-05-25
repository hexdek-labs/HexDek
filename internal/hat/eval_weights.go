package hat

// EvalWeights controls the relative importance of each scoring dimension
// in the GameStateEvaluator. Freya computes deck-specific weights from
// analysis; when nil on a StrategyProfile, the evaluator falls back to
// DefaultWeightsForArchetype.
type EvalWeights struct {
	BoardPresence      float64 `json:"board_presence"`
	CardAdvantage      float64 `json:"card_advantage"`
	ManaAdvantage      float64 `json:"mana_advantage"`
	LifeResource       float64 `json:"life_resource"`
	ComboProximity     float64 `json:"combo_proximity"`
	ThreatExposure     float64 `json:"threat_exposure"`
	CommanderProgress  float64 `json:"commander_progress"`
	GraveyardValue     float64 `json:"graveyard_value"`
	DrainEngine        float64 `json:"drain_engine"`
	ArtifactSynergy    float64 `json:"artifact_synergy"`
	EnchantmentSynergy      float64 `json:"enchantment_synergy"`
	OpponentGraveyardThreat float64 `json:"opponent_graveyard_threat"`
	PartnerSynergy     float64 `json:"partner_synergy"`
	ActivationTempo    float64 `json:"activation_tempo"`
	ToolboxBreadth     float64 `json:"toolbox_breadth"`
	ThreatTrajectory      float64 `json:"threat_trajectory"`
	StackInteraction      float64 `json:"stack_interaction"`
	PlaneswalkerProgress  float64 `json:"planeswalker_progress"`
	ExileZoneAssets       float64 `json:"exile_zone_assets"`
	StaxLockProgress      float64 `json:"stax_lock_progress"`
}

const NumDimensions = 20

func (w EvalWeights) AsArray() [NumDimensions]float64 {
	return [NumDimensions]float64{
		w.BoardPresence, w.CardAdvantage, w.ManaAdvantage,
		w.LifeResource, w.ComboProximity, w.ThreatExposure,
		w.CommanderProgress, w.GraveyardValue, w.DrainEngine,
		w.ArtifactSynergy, w.EnchantmentSynergy, w.OpponentGraveyardThreat,
		w.PartnerSynergy, w.ActivationTempo, w.ToolboxBreadth,
		w.ThreatTrajectory, w.StackInteraction, w.PlaneswalkerProgress,
		w.ExileZoneAssets, w.StaxLockProgress,
	}
}

var archetypeWeights = map[string]EvalWeights{
	ArchetypeAggro: {
		BoardPresence:          1.5,
		CardAdvantage:          0.4,
		ManaAdvantage:          0.3,
		LifeResource:           0.8,
		ComboProximity:         0.1,
		ThreatExposure:         0.6,
		CommanderProgress:      0.9,
		GraveyardValue:         0.2,
		DrainEngine:            0.2,
		ArtifactSynergy:        0.2,
		EnchantmentSynergy:     0.2,
		OpponentGraveyardThreat: 0.3,
		PartnerSynergy:         0.4,
		ActivationTempo:        0.2,
		ToolboxBreadth:         0.2,
		ThreatTrajectory:       0.3,
		StackInteraction:       0.2,
		PlaneswalkerProgress:   0.4,
		ExileZoneAssets:        0.6,
		StaxLockProgress:       0.1,
	},
	ArchetypeCombo: {
		BoardPresence:          0.4,
		CardAdvantage:          0.8,
		ManaAdvantage:          0.7,
		LifeResource:           0.3,
		ComboProximity:         2.0,
		ThreatExposure:         0.5,
		CommanderProgress:      0.6,
		GraveyardValue:         0.5,
		DrainEngine:            0.3,
		ArtifactSynergy:        0.4,
		EnchantmentSynergy:     0.3,
		OpponentGraveyardThreat: 0.5,
		PartnerSynergy:         0.2,
		ActivationTempo:        0.3,
		ToolboxBreadth:         0.6,
		ThreatTrajectory:       0.5,
		StackInteraction:       0.6,
		PlaneswalkerProgress:   0.3,
		ExileZoneAssets:        0.4,
		StaxLockProgress:       0.3,
	},
	ArchetypeControl: {
		BoardPresence:          0.5,
		CardAdvantage:          1.5,
		ManaAdvantage:          0.8,
		LifeResource:           0.6,
		ComboProximity:         0.4,
		ThreatExposure:         1.2,
		CommanderProgress:      0.5,
		GraveyardValue:         0.4,
		DrainEngine:            0.2,
		ArtifactSynergy:        0.3,
		EnchantmentSynergy:     0.3,
		OpponentGraveyardThreat: 1.0,
		PartnerSynergy:         0.4,
		ActivationTempo:        0.7,
		ToolboxBreadth:         0.7,
		ThreatTrajectory:       0.8,
		StackInteraction:       1.5,
		PlaneswalkerProgress:   0.8,
		ExileZoneAssets:        0.4,
		StaxLockProgress:       0.6,
	},
	ArchetypeMidrange: {
		BoardPresence:          1.0,
		CardAdvantage:          1.0,
		ManaAdvantage:          0.8,
		LifeResource:           0.7,
		ComboProximity:         0.5,
		ThreatExposure:         0.8,
		CommanderProgress:      0.7,
		GraveyardValue:         0.5,
		DrainEngine:            0.3,
		ArtifactSynergy:        0.3,
		EnchantmentSynergy:     0.3,
		OpponentGraveyardThreat: 0.6,
		PartnerSynergy:         0.5,
		ActivationTempo:        0.4,
		ToolboxBreadth:         0.5,
		ThreatTrajectory:       0.5,
		StackInteraction:       0.7,
		PlaneswalkerProgress:   0.6,
		ExileZoneAssets:        0.5,
		StaxLockProgress:       0.2,
	},
	ArchetypeRamp: {
		// R60 round 4 audit retune. ManaAdvantage=1.8 was correctly the
		// highest dimension, but the profile was indistinguishable from a
		// generic "we play lands" deck on the other axes. Three changes:
		//   - ManaAdvantage: 1.8 → 2.0 to anchor the signature dimension
		//     at the standard 2.0 used by every other custom profile
		//     (Storm/Mill/Voltron/Aristocrats/Reanimator/Spellslinger/
		//     Tribal/Stax/Selfmill/Enchantress/Artifacts/Lifegain all
		//     anchor their highest at 2.0). 1.8 was the same under-anchor
		//     bug Lifegain had pre-R60.
		//   - BoardPresence: 0.6 → 1.1 to reflect big_creature_density.
		//     Ramp's whole point is dumping fatties early — Craterhoof
		//     Behemoth, Vorinclex Voice of Hunger, Avenger of Zendikar,
		//     Eldrazi titans (Ulamog/Kozilek/Emrakul), Worldspine Wurm,
		//     End-Raze Forerunners, Hornet Queen, Apex Devastator. A ramp
		//     deck with 9 mana and no body on the board has failed its
		//     gameplan. 0.6 ranked board presence BELOW even the Combo
		//     profile's 0.4-tier creature contempt, which was wrong: ramp
		//     cares about the board even though it doesn't care about
		//     individual small creatures.
		//   - LifeResource: 0.5 → 0.7 to land at midrange-neutral. The
		//     prior 0.5 made ramp eager to trade life for mana, but ramp
		//     decks aren't Storm/Mill — they need to SURVIVE the early
		//     turns where they're tapped out ramping, then sit behind a
		//     fatty. Life equals time-to-fatty. Neutral midrange-tier
		//     (0.7) is the correct read.
		BoardPresence:          1.1, // was 0.6 — big_creature_density (Craterhoof / Vorinclex / Eldrazi)
		CardAdvantage:          0.7,
		ManaAdvantage:          2.0, // was 1.8 — anchor signature at standard 2.0
		LifeResource:           0.7, // was 0.5 — neutral (life = time-to-fatty)
		ComboProximity:         0.3,
		ThreatExposure:         0.6,
		CommanderProgress:      0.8,
		GraveyardValue:         0.3,
		DrainEngine:            0.1,
		ArtifactSynergy:        0.5,
		EnchantmentSynergy:     0.2,
		OpponentGraveyardThreat: 0.4,
		PartnerSynergy:         0.3,
		ActivationTempo:        0.4,
		ToolboxBreadth:         0.3,
		ThreatTrajectory:       0.4,
		StackInteraction:       0.3,
		PlaneswalkerProgress:   0.5,
		ExileZoneAssets:        0.3,
		StaxLockProgress:       0.1,
	},
	ArchetypeStax: {
		// R60 round 4 audit retune. StaxLockProgress=2.0 correctly
		// highest, but two dimensions ignored what Stax actually IS:
		//   - ArtifactSynergy: Winter Orb, Static Orb, Stasis,
		//     Smokestack, Sphere of Resistance, Tangle Wire, Trinisphere,
		//     Chalice of the Void, Tabernacle (well, land — but you get
		//     the point), Mana Vault. Most stax pieces are artifacts;
		//     0.6 ranked them below Voltron's equipment package.
		//   - StackInteraction: lock pieces eat removal and counters
		//     constantly — protecting the soft-lock is the difference
		//     between winning a long game and losing a few turns later.
		BoardPresence:          0.7,
		CardAdvantage:          1.2,
		ManaAdvantage:          1.0,
		LifeResource:           0.5,
		ComboProximity:         0.3,
		ThreatExposure:         1.5,
		CommanderProgress:      0.8,
		GraveyardValue:         0.4,
		DrainEngine:            0.2,
		ArtifactSynergy:        1.2, // was 0.6 — Winter Orb / Static Orb / Stasis
		EnchantmentSynergy:     0.5,
		OpponentGraveyardThreat: 0.8,
		PartnerSynergy:         0.3,
		ActivationTempo:        0.5,
		ToolboxBreadth:         0.4,
		ThreatTrajectory:       0.7,
		StackInteraction:       1.1, // was 0.8 — protect lock pieces
		PlaneswalkerProgress:   0.5,
		ExileZoneAssets:        0.2,
		StaxLockProgress:       2.0,
	},
	ArchetypeReanimator: {
		// R60 round 3 audit retune. GraveyardValue=1.8 correctly highest,
		// but the profile under-weighted Reanimator's combo and single-
		// threat realities:
		//   - ComboProximity: Reanimator IS combo-shaped — Animate Dead +
		//     Worldgorger Dragon (infinite mana), Karmic Guide + Reveillark,
		//     Karador + persist creatures. 0.6 weighted these as
		//     Midrange-tier value rather than win-line proximity.
		//   - ThreatExposure: same problem as Voltron — putting a single
		//     fatty back into play means losing it to Path / Swords / Anguished
		//     Unmaking is the whole game. 0.7 was creature-deck noise.
		//   - ActivationTempo: activated reanimators (Sheoldred The
		//     Apocalypse, Volrath, Karador, Lord Windgrace) + dredge /
		//     surveil triggers all live on the activation axis.
		BoardPresence:          0.8,
		CardAdvantage:          0.6,
		ManaAdvantage:          0.5,
		LifeResource:           0.4,
		ComboProximity:         1.1, // was 0.6
		ThreatExposure:         1.2, // was 0.7
		CommanderProgress:      0.6,
		GraveyardValue:         1.8,
		DrainEngine:            0.4,
		ArtifactSynergy:        0.2,
		EnchantmentSynergy:     0.2,
		OpponentGraveyardThreat: 0.8,
		PartnerSynergy:         0.3,
		ActivationTempo:        0.6, // was 0.3
		ToolboxBreadth:         0.4,
		ThreatTrajectory:       0.5,
		StackInteraction:       0.4,
		PlaneswalkerProgress:   0.3,
		ExileZoneAssets:        0.2,
		StaxLockProgress:       0.2,
	},
	ArchetypeSpellslinger: {
		// R60 round 3 audit retune. CardAdvantage=1.4 correctly highest
		// (Niv-Mizzet / Mizzix / Kalamax draw triggers), but three
		// dimensions sat below midrange despite being core to the
		// gameplan:
		//   - ComboProximity: Mizzix-Aetherflux storm lines, Niv-Mizzet
		//     parun infinite loops, Dramatic Reversal + Isochron Scepter
		//     all live in this archetype. 0.5 was lower than Selfmill's
		//     0.7 — wrong direction.
		//   - ManaAdvantage: casting many spells per turn needs ritual /
		//     cost-reduction support (Goblin Electromancer, Birgi,
		//     Baral, Jhoira). 0.9 missed how much the deck depends on
		//     mana acceleration to actually CHAIN spells.
		//   - ActivationTempo: Isochron Scepter is the canonical
		//     spellslinger artifact — its imprint+activate loop is a
		//     core line. 0.3 ignored it entirely.
		BoardPresence:          0.4,
		CardAdvantage:          1.4,
		ManaAdvantage:          1.1, // was 0.9
		LifeResource:           0.5,
		ComboProximity:         1.0, // was 0.5
		ThreatExposure:         0.8,
		CommanderProgress:      0.5,
		GraveyardValue:         0.4,
		DrainEngine:            0.1,
		ArtifactSynergy:        0.2,
		EnchantmentSynergy:     0.3,
		OpponentGraveyardThreat: 0.5,
		PartnerSynergy:         0.3,
		ActivationTempo:        0.7, // was 0.3 — Isochron Scepter
		ToolboxBreadth:         0.6,
		ThreatTrajectory:       0.5,
		StackInteraction:       1.0,
		PlaneswalkerProgress:   0.4,
		ExileZoneAssets:        0.6,
		StaxLockProgress:       0.2,
	},
	ArchetypeTribal: {
		// R60 round 3 audit retune. BoardPresence=1.4 correctly highest
		// (creature-flood is the gameplan), but three dimensions ignored
		// what makes tribal DIFFERENT from a generic creature midrange:
		//   - CardAdvantage: Vanquisher's Banner / Herald's Horn / Beast
		//     Whisperer / Kindred Discovery turn every tribal cast into
		//     a draw. 0.6 was below midrange (1.0).
		//   - PartnerSynergy: lord effects (+1/+1 to other Goblins/Elves/
		//     Slivers) ARE the tribal payoff. PartnerSynergy is the
		//     creature-vs-creature mutual-buff dial; 0.4 was way too
		//     low for a deck whose whole point is creatures-help-each-
		//     other.
		//   - CommanderProgress: tribal commanders (Edgar Markov, Reaper
		//     King, Karador, Slimefoot, Krenko) are usually the win
		//     condition. 1.0 was equal-with-extra-combats; bumped to
		//     match the deck's actual reliance on the commander engine.
		BoardPresence:          1.4,
		CardAdvantage:          1.0, // was 0.6
		ManaAdvantage:          0.5,
		LifeResource:           0.7,
		ComboProximity:         0.4,
		ThreatExposure:         0.6,
		CommanderProgress:      1.2, // was 1.0
		GraveyardValue:         0.6,
		DrainEngine:            0.3,
		ArtifactSynergy:        0.2,
		EnchantmentSynergy:     0.2,
		OpponentGraveyardThreat: 0.4,
		PartnerSynergy:         0.8, // was 0.4 — lord effects
		ActivationTempo:        0.3,
		ToolboxBreadth:         0.3,
		ThreatTrajectory:       0.4,
		StackInteraction:       0.3,
		PlaneswalkerProgress:   0.3,
		ExileZoneAssets:        0.3,
		StaxLockProgress:       0.1,
	},
	ArchetypeAristocrats: {
		// R60 round 2 audit retune. DrainEngine=2.0 correctly highest
		// (Blood Artist / Zulaport Cutthroat / Marionette Master), but
		// three load-bearing dimensions were under-weighted:
		//   - BoardPresence: every sac trigger needs a body. Aristocrats
		//     decks spew tokens (Pawn of Ulamog, Bitterblossom, Ophiomancer)
		//     specifically to keep the sac engine fueled.
		//   - ActivationTempo: sac outlets ARE activated abilities
		//     (Carrion Feeder, Viscera Seer, Yawgmoth Thran Physician,
		//     Goblin Bombardment, Phyrexian Altar). The whole gameplan
		//     loop is "make body, tap/activate sac outlet, repeat."
		//   - GraveyardValue: Karmic Guide / Reveillark / Sun Titan /
		//     Marionette Master / Skullclamp loop the same creatures
		//     turn after turn. Treating GY as a thin resource (0.8)
		//     missed that recursion IS the deck.
		BoardPresence:          1.1, // was 0.6 — sac engine needs MORE bodies than midrange
		CardAdvantage:          0.7,
		ManaAdvantage:          0.5,
		LifeResource:           0.5,
		ComboProximity:         1.0,
		ThreatExposure:         0.5,
		CommanderProgress:      0.8,
		GraveyardValue:         1.2, // was 0.8
		DrainEngine:            2.0,
		ArtifactSynergy:        0.3,
		EnchantmentSynergy:     0.3,
		OpponentGraveyardThreat: 0.6,
		PartnerSynergy:         0.4,
		ActivationTempo:        0.9, // was 0.5
		ToolboxBreadth:         0.3,
		ThreatTrajectory:       0.4,
		StackInteraction:       0.4,
		PlaneswalkerProgress:   0.3,
		ExileZoneAssets:        0.3,
		StaxLockProgress:       0.2,
	},
	ArchetypeSelfmill: {
		BoardPresence:          0.5,
		CardAdvantage:          0.6,
		ManaAdvantage:          0.6,
		LifeResource:           0.4,
		ComboProximity:         0.7,
		ThreatExposure:         0.6,
		CommanderProgress:      0.7,
		GraveyardValue:         2.0,
		DrainEngine:            0.3,
		ArtifactSynergy:        0.2,
		EnchantmentSynergy:     0.2,
		OpponentGraveyardThreat: 0.5,
		PartnerSynergy:         0.3,
		ActivationTempo:        0.3,
		ToolboxBreadth:         0.3,
		ThreatTrajectory:       0.4,
		StackInteraction:       0.3,
		PlaneswalkerProgress:   0.2,
		ExileZoneAssets:        0.2,
		StaxLockProgress:       0.1,
	},
	ArchetypeEnchantress: {
		BoardPresence:          0.5,
		CardAdvantage:          1.3,
		ManaAdvantage:          0.7,
		LifeResource:           0.6,
		ComboProximity:         0.5,
		ThreatExposure:         0.7,
		CommanderProgress:      0.7,
		GraveyardValue:         0.3,
		DrainEngine:            0.2,
		ArtifactSynergy:        0.2,
		EnchantmentSynergy:     2.0,
		OpponentGraveyardThreat: 0.4,
		PartnerSynergy:         0.3,
		ActivationTempo:        0.4,
		ToolboxBreadth:         0.4,
		ThreatTrajectory:       0.5,
		StackInteraction:       0.4,
		PlaneswalkerProgress:   0.3,
		ExileZoneAssets:        0.3,
		StaxLockProgress:       0.4,
	},
	ArchetypeArtifacts: {
		BoardPresence:          0.7,
		CardAdvantage:          1.0,
		ManaAdvantage:          1.1,
		LifeResource:           0.5,
		ComboProximity:         0.8,
		ThreatExposure:         0.7,
		CommanderProgress:      0.7,
		GraveyardValue:         0.4,
		DrainEngine:            0.3,
		ArtifactSynergy:        2.0,
		EnchantmentSynergy:     0.2,
		OpponentGraveyardThreat: 0.5,
		PartnerSynergy:         0.3,
		ActivationTempo:        0.6,
		ToolboxBreadth:         0.4,
		ThreatTrajectory:       0.5,
		StackInteraction:       0.5,
		PlaneswalkerProgress:   0.3,
		ExileZoneAssets:        0.3,
		StaxLockProgress:       0.5,
	},
	ArchetypeLifegain: {
		// R60 follow-up tune. LifeResource was already highest at 1.8 and
		// DrainEngine at 1.2 covered the Sanguine Bond / Vito drain wins,
		// but three dimensions under-weighted what Lifegain ACTUALLY does
		// on the board:
		//   - LifeResource: bumped 1.8 → 2.0 to match the "signature
		//     dimension at the 2.0 anchor" pattern used by every other
		//     custom profile (Storm/Mill/Voltron/Aristocrats/Reanimator/
		//     Spellslinger/Tribal/Stax/Selfmill/Enchantress/Artifacts all
		//     anchor their highest at 2.0). Lifegain's 1.8 was the only
		//     custom profile under-anchored.
		//   - BoardPresence: Ajani's Pridemate / Karlov of the Ghost
		//     Council / Heliod Sun-Crowned grow as the deck gains life;
		//     the board scales WITH the life total, so creatures aren't
		//     incidental — they're the closer. 0.9 was midrange-tier.
		//   - ThreatExposure: lifelink creatures (Soul Warden, Heliod,
		//     Bishop of Wings, Trelasarra, Crested Sunmare) ARE the life-
		//     gain engine. Removing one means the deck stops gaining life
		//     means Pridemate stops growing means the wincon stalls. 0.6
		//     ignored the chained dependency.
		BoardPresence:          1.2, // was 0.9 — Pridemate / Karlov / Heliod grow
		CardAdvantage:          0.7,
		ManaAdvantage:          0.5,
		LifeResource:           2.0, // was 1.8 — anchor signature at standard 2.0
		ComboProximity:         0.8,
		ThreatExposure:         1.0, // was 0.6 — protect the lifelink engine
		CommanderProgress:      0.6,
		GraveyardValue:         0.3,
		DrainEngine:            1.2,
		ArtifactSynergy:        0.2,
		EnchantmentSynergy:     0.4,
		OpponentGraveyardThreat: 0.4,
		PartnerSynergy:         0.3,
		ActivationTempo:        0.4,
		ToolboxBreadth:         0.3,
		ThreatTrajectory:       0.4,
		StackInteraction:       0.4,
		PlaneswalkerProgress:   0.3,
		ExileZoneAssets:        0.2,
		StaxLockProgress:       0.2,
	},
	ArchetypeVoltron: {
		// R60 round 2 audit retune. CommanderProgress=2.0 correctly
		// highest (Voltron's win condition is commander damage), but
		// four dimensions were under-weighted given the deck's whole
		// gameplan revolves around equipping/enchanting one creature:
		//   - ThreatExposure: a Swords to Plowshares on the commander
		//     is a tempo + card disaster — the deck has no plan B. The
		//     0.9 weight made it equal to a midrange creature deck;
		//     bumped to put it above ThreatExposure for Control (1.2).
		//   - ArtifactSynergy: Hammer of Nazahn, Embercleave, Colossus
		//     Hammer, Lightning Greaves, Swiftfoot Boots, Skullclamp —
		//     equipment IS the deck. 0.6 was the same weight as a
		//     midrange-y "we have some artifacts" rating.
		//   - EnchantmentSynergy: Eldrazi Conscription, All That
		//     Glitters, Shielded by Faith, Daybreak Coronet, Ethereal
		//     Armor — voltron-aura subarchetype is half the deck list.
		//   - StackInteraction: Heroic Intervention / Boros Charm /
		//     Teferi's Protection to save the commander before it eats
		//     removal is the difference between game-over and recover.
		BoardPresence:          0.8,
		CardAdvantage:          0.5,
		ManaAdvantage:          0.5,
		LifeResource:           0.6,
		ComboProximity:         0.2,
		ThreatExposure:         1.4, // was 0.9
		CommanderProgress:      2.0,
		GraveyardValue:         0.3,
		DrainEngine:            0.1,
		ArtifactSynergy:        1.1, // was 0.6
		EnchantmentSynergy:     0.9, // was 0.5
		OpponentGraveyardThreat: 0.3,
		PartnerSynergy:         0.3,
		ActivationTempo:        0.5,
		ToolboxBreadth:         0.4,
		ThreatTrajectory:       0.6,
		StackInteraction:       0.8, // was 0.4
		PlaneswalkerProgress:   0.2,
		ExileZoneAssets:        0.2,
		StaxLockProgress:       0.1,
	},
	ArchetypeLandsMatter: {
		// R60 round 4 audit retune. ManaAdvantage=1.8 correctly highest
		// (lands ARE mana), but the profile under-weighted the deck's
		// graveyard-as-second-hand reality:
		//   - GraveyardValue: Crucible of Worlds / Ramunap Excavator /
		//     Splendid Reclamation / World Shaper / Titania, Protector
		//     of Argoth / Worm Harvest / Lord Windgrace -3 / The Gitrog
		//     Monster — recurring lands from GY IS the gameplan. 0.8
		//     ranked it as midrange-noise.
		//   - ComboProximity: Lotus Field + Mystic Sanctuary loops,
		//     Field of the Dead, Scapeshift, Valakut, Dakmor Salvage +
		//     Gitrog Monster combo. 0.4 hid these wins behind the
		//     "we ramp" headline.
		BoardPresence:          0.7,
		CardAdvantage:          0.7,
		ManaAdvantage:          1.8,
		LifeResource:           0.6,
		ComboProximity:         0.7, // was 0.4 — Lotus Field / Scapeshift / Gitrog
		ThreatExposure:         0.5,
		CommanderProgress:      0.7,
		GraveyardValue:         1.3, // was 0.8 — Crucible / Excavator / Titania
		DrainEngine:            0.2,
		ArtifactSynergy:        0.2,
		EnchantmentSynergy:     0.3,
		OpponentGraveyardThreat: 0.4,
		PartnerSynergy:         0.3,
		ActivationTempo:        0.5,
		ToolboxBreadth:         0.4,
		ThreatTrajectory:       0.4,
		StackInteraction:       0.4,
		PlaneswalkerProgress:   0.3,
		ExileZoneAssets:        0.2,
		StaxLockProgress:       0.2,
	},
	ArchetypeCountersMatter: {
		BoardPresence:          1.2,
		CardAdvantage:          0.7,
		ManaAdvantage:          0.6,
		LifeResource:           0.6,
		ComboProximity:         0.5,
		ThreatExposure:         0.7,
		CommanderProgress:      0.9,
		GraveyardValue:         0.3,
		DrainEngine:            0.2,
		ArtifactSynergy:        0.3,
		EnchantmentSynergy:     0.3,
		OpponentGraveyardThreat: 0.4,
		PartnerSynergy:         0.4,
		ActivationTempo:        0.5,
		ToolboxBreadth:         0.3,
		ThreatTrajectory:       0.5,
		StackInteraction:       0.4,
		PlaneswalkerProgress:   0.4,
		ExileZoneAssets:        0.3,
		StaxLockProgress:       0.2,
	},
	ArchetypeMill: {
		// R60 audit retune. Pre-fix Mill weighted like a thin combo deck
		// (ComboProximity high, everything else minimal). In practice Mill
		// is a Control variant whose win condition is exhausting opp
		// libraries — it needs to DRAW the mill spells (CardAdvantage),
		// PROTECT the wincon piece (Bruvac/Tasha → StackInteraction), and
		// LEAN ON wheel-style draw-then-mill engines (Notion Thief /
		// Consecrated Sphinx / Hullbreacher → DrainEngine).
		//
		// R60 follow-up (PR #194 gauntlet): Phenax (Mill) regressed 3.7%
		// after the cross-cutting late-game LifeResource bump (lateFactor
		// *0.15) merged in #191 because Mill decks WANT to trade life
		// for mill progress — taking damage to keep mana up for another
		// Maddening Cacophony / Glimpse the Unthinkable / Bruvac trigger
		// is correct play. The global bump pushed Mill's effective
		// late-game LifeResource from 0.5 → 0.575, making the hat refuse
		// trades it should take. Dropping the baseline to 0.3 lets the
		// global bump land Mill at 0.345 late-game — comfortably below
		// midrange's 0.575 — restoring the "life is cheaper for Mill
		// than for midrange" property the deck depends on.
		BoardPresence:          0.4,
		CardAdvantage:          1.4, // was 0.8
		ManaAdvantage:          0.6,
		LifeResource:           0.3, // was 0.5 — life is tradeable for mill progress
		ComboProximity:         1.4,
		ThreatExposure:         0.8,
		CommanderProgress:      0.5,
		GraveyardValue:         0.3,
		DrainEngine:            0.8, // was 0.4
		ArtifactSynergy:        0.2,
		EnchantmentSynergy:     0.2,
		OpponentGraveyardThreat: 0.3,
		PartnerSynergy:         0.3,
		ActivationTempo:        0.4,
		ToolboxBreadth:         0.7, // was 0.4
		ThreatTrajectory:       0.4,
		StackInteraction:       1.1, // was 0.6
		PlaneswalkerProgress:   0.3,
		ExileZoneAssets:        0.3,
		StaxLockProgress:       0.2,
	},
	ArchetypeStorm: {
		// R60 audit retune. ComboProximity (1.8) was correctly highest,
		// but ActivationTempo at 0.3 ignored Storm's gameplan — Dark
		// Ritual / Cabal Ritual / LED / Aetherflux Reservoir are
		// ACTIVATIONS and UNTAPS, not raw mana. StackInteraction at 0.8
		// underweighted protection: the spell chain dies to a single
		// resolved counter mid-storm. GraveyardValue at 0.3 ignored
		// Past in Flames / Yawgmoth's Will second-storm lines.
		BoardPresence:          0.2,
		CardAdvantage:          1.3,
		ManaAdvantage:          1.3, // was 1.5 — rituals matter more than raw lands
		LifeResource:           0.2,
		ComboProximity:         1.8,
		ThreatExposure:         0.3,
		CommanderProgress:      0.4,
		GraveyardValue:         0.6, // was 0.3 — Past in Flames lines
		DrainEngine:            0.4,
		ArtifactSynergy:        0.3,
		EnchantmentSynergy:     0.2,
		OpponentGraveyardThreat: 0.3,
		PartnerSynergy:         0.2,
		ActivationTempo:        1.2, // was 0.3 — rituals, Aetherflux, LED
		ToolboxBreadth:         0.5,
		ThreatTrajectory:       0.3,
		StackInteraction:       1.4, // was 0.8 — counter war over the chain
		PlaneswalkerProgress:   0.2,
		ExileZoneAssets:        0.3,
		StaxLockProgress:       0.1,
	},
	ArchetypeSuperfriends: {
		BoardPresence:          0.9,
		CardAdvantage:          1.2,
		ManaAdvantage:          0.7,
		LifeResource:           0.8,
		ComboProximity:         0.4,
		ThreatExposure:         1.0,
		CommanderProgress:      0.4,
		GraveyardValue:         0.3,
		DrainEngine:            0.2,
		ArtifactSynergy:        0.3,
		EnchantmentSynergy:     0.3,
		OpponentGraveyardThreat: 0.5,
		PartnerSynergy:         0.3,
		ActivationTempo:        0.4,
		ToolboxBreadth:         0.5,
		ThreatTrajectory:       0.6,
		StackInteraction:       0.7,
		PlaneswalkerProgress:   2.0,
		ExileZoneAssets:        0.3,
		StaxLockProgress:       0.3,
	},
	ArchetypeBlink: {
		// R60 round 4 audit retune. CardAdvantage=1.2 correctly highest
		// (re-trigger ETBs = re-trigger draws), but two dimensions
		// missed the deck's exile-as-engine and combo realities:
		//   - ExileZoneAssets: Yorion Sky Nomad's exile pile is the
		//     deck's resource; Brago King Eternal exiles and returns;
		//     Soulherder accumulates +1/+1 from exiled-and-returned
		//     creatures. 0.5 hid this from the evaluator — it was lower
		//     than Spellslinger's ExileZoneAssets (0.6) despite Blink
		//     literally living in exile.
		//   - ComboProximity: Felidar Guardian + Saheeli Rai (infinite
		//     tokens), Restoration Angel + Kiki-Jiki (infinite haste
		//     copies), Emiel the Blessed + Tundra Wolves family,
		//     Deadeye Navigator + Peregrine Drake — Blink decks have
		//     two-card infinites stapled onto every ETB creature.
		BoardPresence:          0.8,
		CardAdvantage:          1.2,
		ManaAdvantage:          0.7,
		LifeResource:           0.6,
		ComboProximity:         1.0, // was 0.6 — Felidar+Saheeli, Resto+Kiki
		ThreatExposure:         0.7,
		CommanderProgress:      0.7,
		GraveyardValue:         0.3,
		DrainEngine:            0.3,
		ArtifactSynergy:        0.3,
		EnchantmentSynergy:     0.3,
		OpponentGraveyardThreat: 0.4,
		PartnerSynergy:         0.4,
		ActivationTempo:        0.8,
		ToolboxBreadth:         0.7,
		ThreatTrajectory:       0.4,
		StackInteraction:       0.4,
		PlaneswalkerProgress:   0.3,
		ExileZoneAssets:        1.0, // was 0.5 — Yorion pile / Soulherder / Brago
		StaxLockProgress:       0.2,
	},
	ArchetypeExtraCombats: {
		BoardPresence:          1.3,
		CardAdvantage:          0.5,
		ManaAdvantage:          0.6,
		LifeResource:           0.7,
		ComboProximity:         0.4,
		ThreatExposure:         0.7,
		CommanderProgress:      1.0,
		GraveyardValue:         0.3,
		DrainEngine:            0.2,
		ArtifactSynergy:        0.4,
		EnchantmentSynergy:     0.3,
		OpponentGraveyardThreat: 0.3,
		PartnerSynergy:         0.3,
		ActivationTempo:        0.4,
		ToolboxBreadth:         0.3,
		ThreatTrajectory:       0.5,
		StackInteraction:       0.3,
		PlaneswalkerProgress:   0.3,
		ExileZoneAssets:        0.2,
		StaxLockProgress:       0.1,
	},
}

// LegacyMidrangeOnly, when true, makes DefaultWeightsForArchetype return
// the midrange profile for every archetype — collapsing archetype-specific
// dispatch. Set by the tournament CLI's --legacy-hat-weights flag to
// produce a pre-archetype-tuning baseline for A/B winrate comparisons.
// Set once before games start; not safe to flip concurrently.
var LegacyMidrangeOnly bool

// DefaultWeightsForArchetype returns the tuned weight profile for the
// given archetype string. Unknown archetypes get the midrange profile.
func DefaultWeightsForArchetype(archetype string) EvalWeights {
	if LegacyMidrangeOnly {
		return archetypeWeights[ArchetypeMidrange]
	}
	if w, ok := archetypeWeights[archetype]; ok {
		return w
	}
	return archetypeWeights[ArchetypeMidrange]
}
