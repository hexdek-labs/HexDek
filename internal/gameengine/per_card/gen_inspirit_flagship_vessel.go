package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerInspiritFlagshipVessel wires Inspirit, Flagship Vessel.
//
// Oracle text (Scryfall, verified):
//
//	Station (Tap another creature you control: Put charge counters
//	equal to its power on this Spacecraft. Station only as a sorcery.
//	It's an artifact creature at 8+.)
//	1+ | At the beginning of combat on your turn, put your choice of
//	a +1/+1 counter or two charge counters on up to one other target
//	artifact.
//	8+ | Flying
//	Other artifacts you control have hexproof and indestructible.
//
// Implementation (R46 stub port):
//   - Static "other artifacts you control have hexproof and
//     indestructible": OnETB walks the controller's battlefield and
//     stamps kw:hexproof + kw:indestructible on every other artifact
//     perm. Refresh hook on nonland_permanent_etb catches newly-
//     entering artifacts so the grant stays applied. The grant is
//     additive (we don't strip on Inspirit LTB, matching the conservative
//     "until Phase 8 layers" stance other static-grant ports take).
//   - 1+ combat-begin trigger: OnTrigger("combat_begin") gated on
//     active_seat == controller. AI policy is +1/+1 (the more durable
//     stat-pump) on the highest-power non-Inspirit own artifact.
//   - Station activated ability (tap another creature for power-many
//     charge counters) and the 8+ flying / artifact-creature
//     Spacecraft transition: emitPartial. Activated-ability surface
//     for Station is doable separately; the type-change rider needs
//     Phase 8 layers.
func registerInspiritFlagshipVessel(r *Registry) {
	r.OnETB("Inspirit, Flagship Vessel", inspiritFlagshipVesselETB)
	r.OnTrigger("Inspirit, Flagship Vessel", "nonland_permanent_etb", inspiritRefreshArtifactGrants)
	r.OnTrigger("Inspirit, Flagship Vessel", "combat_begin", inspiritCombatBeginCounter)
	// R55: Spacecraft 8+ → flying artifact creature, via R54 Layer 4
	// (add types) + Layer 6 (kw:flying) primitives. The Spacecraft
	// threshold check fires on counter_placed (charge counters accrue
	// through Station activations) and on ETB.
	r.OnTrigger("Inspirit, Flagship Vessel", "counter_placed", inspiritCheckSpacecraftThreshold)
}

func inspiritFlagshipVesselETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "inspirit_flagship_vessel_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	stamped := inspiritStampArtifactGrants(gs, perm)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":           perm.Controller,
		"artifacts_buff": stamped,
	})
	// R55: Station activated ability is the engine's standard activated
	// dispatch (caller goes through Station tap-creature → charge
	// counter mechanic in the AST keyword pipeline). 8+ Spacecraft
	// threshold wired via inspiritCheckSpacecraftThreshold on
	// counter_placed.
	inspiritCheckSpacecraftThreshold(gs, perm, map[string]interface{}{
		"perm": perm,
	})
}

// inspiritCheckSpacecraftThreshold wires the Spacecraft transition:
// once Inspirit has 8+ charge counters, it becomes a flying artifact
// creature. R55 uses the Layer-4 add-types + Layer-6 grant-keyword
// primitives added in R54.
func inspiritCheckSpacecraftThreshold(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	if perm.Counters == nil {
		return
	}
	if perm.Counters["charge"] < 8 {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	if perm.Flags["inspirit_spacecraft_active"] == 1 {
		return // already crossed the threshold; layered effects are registered
	}
	perm.Flags["inspirit_spacecraft_active"] = 1
	gameengine.RegisterAddTypes(gs, perm, []string{"creature"},
		gameengine.DurationUntilSourceLeaves,
		"Inspirit, Flagship Vessel (Spacecraft 8+)",
		"inspirit_spacecraft_creature")
	gameengine.RegisterGrantKeyword(gs, perm, "flying",
		gameengine.DurationUntilSourceLeaves,
		"Inspirit, Flagship Vessel (Spacecraft 8+)",
		"inspirit_spacecraft_flying")
	emit(gs, "inspirit_spacecraft_threshold_crossed", perm.Card.DisplayName(), map[string]interface{}{
		"seat":             perm.Controller,
		"charge_counters":  perm.Counters["charge"],
	})
}

func inspiritRefreshArtifactGrants(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	entering, _ := ctx["perm"].(*gameengine.Permanent)
	if entering == nil || entering.Card == nil {
		return
	}
	if entering.Controller != perm.Controller {
		return
	}
	if entering == perm {
		return
	}
	if !entering.IsArtifact() {
		return
	}
	if entering.Flags == nil {
		entering.Flags = map[string]int{}
	}
	entering.Flags["kw:hexproof"] = 1
	entering.Flags["kw:indestructible"] = 1
}

// inspiritStampArtifactGrants walks the controller's battlefield and
// sets kw:hexproof + kw:indestructible on every artifact other than
// Inspirit himself. Returns the count of artifacts touched.
func inspiritStampArtifactGrants(gs *gameengine.GameState, perm *gameengine.Permanent) int {
	if gs == nil || perm == nil || perm.Controller < 0 || perm.Controller >= len(gs.Seats) {
		return 0
	}
	s := gs.Seats[perm.Controller]
	if s == nil {
		return 0
	}
	count := 0
	for _, p := range s.Battlefield {
		if p == nil || p == perm || p.Card == nil {
			continue
		}
		if !p.IsArtifact() {
			continue
		}
		if p.Flags == nil {
			p.Flags = map[string]int{}
		}
		p.Flags["kw:hexproof"] = 1
		p.Flags["kw:indestructible"] = 1
		count++
	}
	return count
}

func inspiritCombatBeginCounter(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "inspirit_combat_begin_counter"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	s := gs.Seats[perm.Controller]
	if s == nil {
		return
	}
	var pick *gameengine.Permanent
	bestPower := -1
	for _, p := range s.Battlefield {
		if p == nil || p == perm || p.Card == nil {
			continue
		}
		if !p.IsArtifact() {
			continue
		}
		if p.Power() > bestPower {
			bestPower = p.Power()
			pick = p
		}
	}
	if pick == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_other_artifact_target", map[string]interface{}{
			"seat": perm.Controller,
		})
		return
	}
	pick.AddCounter("+1/+1", 1)
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"target": pick.Card.DisplayName(),
		"mode":   "+1/+1",
	})
}
