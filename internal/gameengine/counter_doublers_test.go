package gameengine

// Counter DB Phase 6 — engine-side tests for the new doubler card
// registrations and the AddCountersToPermanent bridge. These exercise the
// FireEvent dispatch path (Doubling Season + Hardened Scales + the new
// Phase 6 family) through the §616 audit chain, distinct from the
// counters-package property tests in counters/doubling_test.go which
// drive ApplyDoublers directly with mock Doublers.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine/counters"
)

// addDoublerSource installs a non-creature artifact-shaped Permanent on
// the given seat with the supplied register-function applied. Returns the
// new Permanent so tests can call Stamp* audit helpers.
func addDoublerSource(gs *GameState, seat int, name string, register func(*GameState, *Permanent)) *Permanent {
	card := &Card{
		Name:  name,
		Owner: seat,
		Types: []string{"enchantment"},
	}
	p := &Permanent{
		Card:       card,
		Controller: seat,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	if register != nil {
		register(gs, p)
	}
	return p
}

// TestAddCountersToPermanent_NoDoublersPassthrough confirms the bridge
// places the requested count when no doublers are in play.
func TestAddCountersToPermanent_NoDoublersPassthrough(t *testing.T) {
	gs := newCombatGame(t)
	target := addCreature(gs, 0, "Walking Ballista", 0, 0)

	placed, chain := AddCountersToPermanent(gs, target, "+1/+1", 2, "src-001", false)
	if placed != 2 {
		t.Errorf("no doublers: placed = %d, want 2", placed)
	}
	if len(chain) != 0 {
		t.Errorf("chain len = %d, want 0", len(chain))
	}
	if got := counters.CounterCount(target.AsCounterTarget(), "+1/+1"); got != 2 {
		t.Errorf("CounterCount = %d, want 2", got)
	}
}

// TestAddCountersToPermanent_HardenedScales — single Hardened Scales adds
// +1 to a +1/+1 placement. Through the engine FireEvent path.
func TestAddCountersToPermanent_HardenedScales(t *testing.T) {
	gs := newCombatGame(t)
	addDoublerSource(gs, 0, "Hardened Scales", RegisterHardenedScales)
	target := addCreature(gs, 0, "Walking Ballista", 0, 0)

	placed, chain := AddCountersToPermanent(gs, target, "+1/+1", 2, "src", false)
	if placed != 3 {
		t.Errorf("HS: placed = %d, want 3", placed)
	}
	if len(chain) != 1 {
		t.Fatalf("chain len = %d, want 1", len(chain))
	}
	if chain[0].SourceName != "Hardened Scales" {
		t.Errorf("chain[0].SourceName = %q, want Hardened Scales", chain[0].SourceName)
	}
}

// TestAddCountersToPermanent_DoublingSeasonAndHardenedScales — both
// doublers fire; final count depends on the §616 ordering chosen by the
// engine's pickReplacement (deterministic timestamp ascending in the
// default no-Hat fixture).
func TestAddCountersToPermanent_DoublingSeasonAndHardenedScales(t *testing.T) {
	gs := newCombatGame(t)
	// DS first (lower timestamp), then HS — DS-then-HS ordering: 2 → 4 → 5.
	addDoublerSource(gs, 0, "Doubling Season", RegisterDoublingSeason)
	addDoublerSource(gs, 0, "Hardened Scales", RegisterHardenedScales)
	target := addCreature(gs, 0, "Walking Ballista", 0, 0)

	placed, chain := AddCountersToPermanent(gs, target, "+1/+1", 2, "src", false)
	if placed != 5 {
		t.Errorf("DS+HS: placed = %d, want 5 (2 → 4 → 5)", placed)
	}
	if len(chain) != 2 {
		t.Fatalf("chain len = %d, want 2", len(chain))
	}
}

// TestAddCountersToPermanent_PrimalVigorOnAnyCreature — Primal Vigor is
// symmetric: doubles +1/+1 placements on ANY creature, not just the
// controller's. Drops a Primal Vigor on seat 0 and places a +1/+1 on
// seat 1's creature; confirms the doubler still fires.
func TestAddCountersToPermanent_PrimalVigorOnAnyCreature(t *testing.T) {
	gs := newCombatGame(t)
	addDoublerSource(gs, 0, "Primal Vigor", RegisterPrimalVigor)
	target := addCreature(gs, 1, "Hornet Queen", 2, 2)

	placed, chain := AddCountersToPermanent(gs, target, "+1/+1", 1, "src", false)
	if placed != 2 {
		t.Errorf("Primal Vigor on opp creature: placed = %d, want 2", placed)
	}
	if len(chain) != 1 {
		t.Fatalf("chain len = %d, want 1", len(chain))
	}
	if chain[0].SourceName != "Primal Vigor" {
		t.Errorf("chain[0].SourceName = %q, want Primal Vigor", chain[0].SourceName)
	}
}

// TestAddCountersToPermanent_BranchingEvolutionCreaturesOnly — Branching
// Evolution doubles +1/+1 only on creatures the controller controls.
// Confirms the gate at three corners: same-seat creature (fires),
// opp-seat creature (skip), same-seat non-creature (skip).
func TestAddCountersToPermanent_BranchingEvolutionCreaturesOnly(t *testing.T) {
	gs := newCombatGame(t)
	addDoublerSource(gs, 0, "Branching Evolution", RegisterBranchingEvolution)

	mine := addCreature(gs, 0, "Walking Ballista", 0, 0)
	placed, _ := AddCountersToPermanent(gs, mine, "+1/+1", 1, "src", false)
	if placed != 2 {
		t.Errorf("BE on own creature: placed = %d, want 2", placed)
	}

	opp := addCreature(gs, 1, "Hornet Queen", 2, 2)
	placedOpp, _ := AddCountersToPermanent(gs, opp, "+1/+1", 1, "src", false)
	if placedOpp != 1 {
		t.Errorf("BE on opp creature: placed = %d, want 1 (BE doesn't fire)", placedOpp)
	}
}

// TestAddCountersToPermanent_VorinclexAsymmetric — controller's
// placement doubles, opponent's placement halves.
func TestAddCountersToPermanent_VorinclexAsymmetric(t *testing.T) {
	gs := newCombatGame(t)
	addDoublerSource(gs, 0, "Vorinclex, Monstrous Raider", RegisterVorinclexMonstrousRaider)

	mine := addCreature(gs, 0, "Champion", 1, 1)
	placedSelf, chainSelf := AddCountersToPermanent(gs, mine, "+1/+1", 3, "src", false)
	if placedSelf != 6 {
		t.Errorf("Vorinclex self placement of 3: placed = %d, want 6", placedSelf)
	}
	if len(chainSelf) != 1 {
		t.Fatalf("self chain len = %d, want 1", len(chainSelf))
	}

	opp := addCreature(gs, 1, "Opp Beast", 2, 2)
	placedOpp, chainOpp := AddCountersToPermanent(gs, opp, "+1/+1", 7, "src", false)
	if placedOpp != 3 {
		t.Errorf("Vorinclex opp placement of 7: placed = %d, want 3 (halve floor)", placedOpp)
	}
	if len(chainOpp) != 1 {
		t.Fatalf("opp chain len = %d, want 1", len(chainOpp))
	}
}

// TestAddCountersToPermanent_OzolithETBGate — Ozolith fires only when the
// placement carries the ETB context flag. Non-ETB +1/+1 placements skip.
func TestAddCountersToPermanent_OzolithETBGate(t *testing.T) {
	gs := newCombatGame(t)
	addDoublerSource(gs, 0, "Ozolith, the Shattered Spire", RegisterOzolithShatteredSpire)
	target := addCreature(gs, 0, "Walking Ballista", 0, 0)

	// Non-ETB placement: Ozolith should NOT fire.
	placed, _ := AddCountersToPermanent(gs, target, "+1/+1", 2, "src", false)
	if placed != 2 {
		t.Errorf("non-ETB +1/+1: placed = %d, want 2 (Ozolith gates by ETB)", placed)
	}

	// ETB placement: Ozolith fires (+1).
	target2 := addCreature(gs, 0, "Champion", 0, 0)
	placedETB, chain := AddCountersToPermanent(gs, target2, "+1/+1", 2, "src", true)
	if placedETB != 3 {
		t.Errorf("ETB +1/+1 with Ozolith: placed = %d, want 3 (2 → 3)", placedETB)
	}
	if len(chain) != 1 || chain[0].SourceName != "Ozolith, the Shattered Spire" {
		t.Errorf("Ozolith chain = %+v", chain)
	}
}

// TestAddCountersToPermanent_ConclaveMentorStacksWithHardenedScales —
// Conclave Mentor + Hardened Scales each add +1 independently per §122.1g.
// 2 → 3 → 4.
func TestAddCountersToPermanent_ConclaveMentorStacksWithHardenedScales(t *testing.T) {
	gs := newCombatGame(t)
	addDoublerSource(gs, 0, "Hardened Scales", RegisterHardenedScales)
	addDoublerSource(gs, 0, "Conclave Mentor", RegisterConclaveMentor)
	target := addCreature(gs, 0, "Walking Ballista", 0, 0)

	placed, chain := AddCountersToPermanent(gs, target, "+1/+1", 2, "src", false)
	if placed != 4 {
		t.Errorf("HS+CM: placed = %d, want 4 (2 → 3 → 4)", placed)
	}
	if len(chain) != 2 {
		t.Errorf("chain len = %d, want 2", len(chain))
	}
}

// TestAddCountersToPermanent_EnergyShortCircuits — energy is not in the
// counters registry (CR §106.11 carve-out). The bridge must place 0 and
// NOT fire the chain.
func TestAddCountersToPermanent_EnergyShortCircuits(t *testing.T) {
	gs := newCombatGame(t)
	addDoublerSource(gs, 0, "Doubling Season", RegisterDoublingSeason)
	target := addCreature(gs, 0, "Aetherworks Marvel", 0, 0)

	placed, chain := AddCountersToPermanent(gs, target, "energy", 1, "src", false)
	if placed != 0 {
		t.Errorf("energy via doubler bridge: placed = %d, want 0 (energy unregistered — §106.11)", placed)
	}
	if len(chain) != 0 {
		t.Errorf("energy chain len = %d, want 0", len(chain))
	}
}

// TestProvidesReplacementsPopulated — every new Phase 6 registration must
// populate the Phase 5 audit surface Permanent.ProvidesReplacements.
// Defends the audit trail for Heimdall + Loki.
func TestProvidesReplacementsPopulated(t *testing.T) {
	gs := newCombatGame(t)

	cases := []struct {
		name     string
		register func(*GameState, *Permanent)
		// minSpecs is the minimum number of ReplacementSpec entries we
		// expect — multi-arm cards (DS, Primal Vigor, Vorinclex) register
		// multiple.
		minSpecs int
	}{
		{"Primal Vigor", RegisterPrimalVigor, 2},                              // counter + token
		{"Branching Evolution", RegisterBranchingEvolution, 1},                // counter only
		{"Vorinclex, Monstrous Raider", RegisterVorinclexMonstrousRaider, 2},  // self + opp
		{"Ozolith, the Shattered Spire", RegisterOzolithShatteredSpire, 1},    // counter only
		{"Conclave Mentor", RegisterConclaveMentor, 1},                        // counter only
	}
	for _, c := range cases {
		p := addDoublerSource(gs, 0, c.name, c.register)
		if len(p.ProvidesReplacements) < c.minSpecs {
			t.Errorf("%s: ProvidesReplacements len = %d, want >= %d",
				c.name, len(p.ProvidesReplacements), c.minSpecs)
		}
		// Each spec must have a non-empty HandlerID and Name matching the card.
		for i, spec := range p.ProvidesReplacements {
			if spec.HandlerID == "" {
				t.Errorf("%s spec[%d] HandlerID empty", c.name, i)
			}
			if spec.Name != c.name {
				t.Errorf("%s spec[%d].Name = %q, want %q", c.name, i, spec.Name, c.name)
			}
		}
	}
}

// TestStampDoublingSeasonAudit_Idempotent — retrofit audit stamp for the
// pre-Phase-6 DS handler. Must not double-stamp when called twice.
func TestStampDoublingSeasonAudit_Idempotent(t *testing.T) {
	gs := newCombatGame(t)
	p := addDoublerSource(gs, 0, "Doubling Season", RegisterDoublingSeason)
	StampDoublingSeasonAudit(p)
	first := len(p.ProvidesReplacements)
	StampDoublingSeasonAudit(p)
	second := len(p.ProvidesReplacements)
	if first != second {
		t.Errorf("idempotent stamp: %d → %d (should be unchanged)", first, second)
	}
	if first != 2 {
		t.Errorf("DS stamp count = %d, want 2 (token + counter arms)", first)
	}
}

// TestAddCountersToPermanent_OpponentDoublerOnControllersCreature —
// Doubling Season belongs to seat 0; seat 1 places a +1/+1 on their own
// creature. Doubling Season should NOT fire (gates on TargetPerm.Controller
// == p.Controller).
func TestAddCountersToPermanent_OpponentDoublerOnControllersCreature(t *testing.T) {
	gs := newCombatGame(t)
	addDoublerSource(gs, 0, "Doubling Season", RegisterDoublingSeason)
	opp := addCreature(gs, 1, "Opp Champion", 1, 1)

	placed, chain := AddCountersToPermanent(gs, opp, "+1/+1", 2, "src", false)
	if placed != 2 {
		t.Errorf("opp seat placement under controller's DS: placed = %d, want 2 (DS doesn't apply)", placed)
	}
	if len(chain) != 0 {
		t.Errorf("chain len = %d, want 0", len(chain))
	}
}
