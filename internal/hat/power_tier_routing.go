package hat

// ApplyPowerTierRouting adjusts a deck's eval weights based on the
// cEDH power-tier classification (Freya PRs #714/#715). At cEDH tier
// (T5) the hat prioritizes combo execution + tutor chains + stack
// interaction over board development; at casual tier (T1/T2) it
// prioritizes board presence + commander damage + life pressure;
// at upgraded precon (T3) weights are unchanged (neutral midline);
// at high power (T4) it tilts toward cEDH-shape but less aggressively.
//
// Multipliers (per dimension, per tier):
//
//	Dimension              T1/T2  T3  T4   T5
//	---------------------------------------------
//	ComboProximity          0.6   1.0 1.25 1.6
//	StackInteraction        0.5   1.0 1.3  1.6
//	ActivationTempo         0.7   1.0 1.2  1.4
//	ToolboxBreadth          0.7   1.0 1.15 1.3
//	BoardPresence           1.4   1.0 0.85 0.6
//	CommanderProgress       1.4   1.0 0.85 0.65
//	LifeResource            1.2   1.0 0.85 0.7
//
// Rationale: cEDH games are won by assembling 2-3 card combos through
// tutor chains while interacting on the stack to protect the line —
// board development is a means to that end, not the win condition.
// Casual games are won (or lost) on the board over many turns —
// commander damage, creature attrition, and life-total pressure are
// the dominant axes. T3 is the neutral midline where the hat applies
// its archetype-tuned weights without tier-based tilt.
//
// No-op when PowerTier == 0 (Freya didn't classify) or when sp.Weights
// is nil (no weights to multiply). When PowerTier is out of band
// (negative or > 5), no-op rather than panic — defensive default for
// future schema changes.
func ApplyPowerTierRouting(sp *StrategyProfile) {
	if sp == nil || sp.Weights == nil {
		return
	}
	if sp.PowerTier < 1 || sp.PowerTier > 5 {
		return
	}
	m := powerTierMultipliers(sp.PowerTier)
	if m == nil {
		return
	}
	w := sp.Weights
	w.ComboProximity *= m.ComboProximity
	w.StackInteraction *= m.StackInteraction
	w.ActivationTempo *= m.ActivationTempo
	w.ToolboxBreadth *= m.ToolboxBreadth
	w.BoardPresence *= m.BoardPresence
	w.CommanderProgress *= m.CommanderProgress
	w.LifeResource *= m.LifeResource
}

// powerTierMultipliers returns the per-dimension multiplier table for
// a given tier, or nil for T3 (neutral — no multipliers applied).
// Returning nil for the neutral case lets ApplyPowerTierRouting skip
// the multiply loop entirely, avoiding floating-point noise from
// six ×1.0 operations.
func powerTierMultipliers(tier int) *powerTierMult {
	switch tier {
	case 1, 2:
		return &powerTierMult{
			ComboProximity:    0.6,
			StackInteraction:  0.5,
			ActivationTempo:   0.7,
			ToolboxBreadth:    0.7,
			BoardPresence:     1.4,
			CommanderProgress: 1.4,
			LifeResource:      1.2,
		}
	case 3:
		return nil // neutral midline
	case 4:
		return &powerTierMult{
			ComboProximity:    1.25,
			StackInteraction:  1.30,
			ActivationTempo:   1.20,
			ToolboxBreadth:    1.15,
			BoardPresence:     0.85,
			CommanderProgress: 0.85,
			LifeResource:      0.85,
		}
	case 5:
		return &powerTierMult{
			ComboProximity:    1.60,
			StackInteraction:  1.60,
			ActivationTempo:   1.40,
			ToolboxBreadth:    1.30,
			BoardPresence:     0.60,
			CommanderProgress: 0.65,
			LifeResource:      0.70,
		}
	}
	return nil
}

// powerTierMult is the per-dimension multiplier shape for a single
// tier. Internal-only — consumers go through ApplyPowerTierRouting.
type powerTierMult struct {
	ComboProximity    float64
	StackInteraction  float64
	ActivationTempo   float64
	ToolboxBreadth    float64
	BoardPresence     float64
	CommanderProgress float64
	LifeResource      float64
}
