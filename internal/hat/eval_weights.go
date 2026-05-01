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
	},
	ArchetypeRamp: {
		BoardPresence:          0.6,
		CardAdvantage:          0.7,
		ManaAdvantage:          1.8,
		LifeResource:           0.5,
		ComboProximity:         0.3,
		ThreatExposure:         0.6,
		CommanderProgress:      0.8,
		GraveyardValue:         0.3,
		DrainEngine:            0.1,
		ArtifactSynergy:        0.5,
		EnchantmentSynergy:     0.2,
		OpponentGraveyardThreat: 0.4,
	},
	ArchetypeStax: {
		BoardPresence:          0.7,
		CardAdvantage:          1.2,
		ManaAdvantage:          1.0,
		LifeResource:           0.5,
		ComboProximity:         0.3,
		ThreatExposure:         1.5,
		CommanderProgress:      0.8,
		GraveyardValue:         0.4,
		DrainEngine:            0.2,
		ArtifactSynergy:        0.6,
		EnchantmentSynergy:     0.5,
		OpponentGraveyardThreat: 0.8,
	},
	ArchetypeReanimator: {
		BoardPresence:          0.8,
		CardAdvantage:          0.6,
		ManaAdvantage:          0.5,
		LifeResource:           0.4,
		ComboProximity:         0.6,
		ThreatExposure:         0.7,
		CommanderProgress:      0.6,
		GraveyardValue:         1.8,
		DrainEngine:            0.4,
		ArtifactSynergy:        0.2,
		EnchantmentSynergy:     0.2,
		OpponentGraveyardThreat: 0.8,
	},
	ArchetypeSpellslinger: {
		BoardPresence:          0.4,
		CardAdvantage:          1.4,
		ManaAdvantage:          0.9,
		LifeResource:           0.5,
		ComboProximity:         0.5,
		ThreatExposure:         0.8,
		CommanderProgress:      0.5,
		GraveyardValue:         0.4,
		DrainEngine:            0.1,
		ArtifactSynergy:        0.2,
		EnchantmentSynergy:     0.3,
		OpponentGraveyardThreat: 0.5,
	},
	ArchetypeTribal: {
		BoardPresence:          1.4,
		CardAdvantage:          0.6,
		ManaAdvantage:          0.5,
		LifeResource:           0.7,
		ComboProximity:         0.4,
		ThreatExposure:         0.6,
		CommanderProgress:      1.0,
		GraveyardValue:         0.6,
		DrainEngine:            0.3,
		ArtifactSynergy:        0.2,
		EnchantmentSynergy:     0.2,
		OpponentGraveyardThreat: 0.4,
	},
	ArchetypeAristocrats: {
		BoardPresence:          0.6,
		CardAdvantage:          0.7,
		ManaAdvantage:          0.5,
		LifeResource:           0.5,
		ComboProximity:         1.0,
		ThreatExposure:         0.5,
		CommanderProgress:      0.8,
		GraveyardValue:         0.8,
		DrainEngine:            2.0,
		ArtifactSynergy:        0.3,
		EnchantmentSynergy:     0.3,
		OpponentGraveyardThreat: 0.6,
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
	},
}

// DefaultWeightsForArchetype returns the tuned weight profile for the
// given archetype string. Unknown archetypes get the midrange profile.
func DefaultWeightsForArchetype(archetype string) EvalWeights {
	if w, ok := archetypeWeights[archetype]; ok {
		return w
	}
	return archetypeWeights[ArchetypeMidrange]
}
