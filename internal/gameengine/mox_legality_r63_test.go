package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ===========================================================================
// Symptom 1 — CR §117.1a: a non-flash permanent spell follows sorcery timing
// (active player, main phase, empty stack). CastSpell is the trust boundary.
// ===========================================================================

func moxCard(seat int) *Card {
	return &Card{
		Name:           "Mox Amber",
		Owner:          seat,
		Types:          []string{"legendary", "artifact"},
		ManaCostString: "",
		AST:            &gameast.CardAST{Name: "Mox Amber"},
	}
}

func TestMoxCast_NonActiveSeatRejected(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.Active = 0
	gs.Phase, gs.Step = "main", "precombat_main"
	mox := moxCard(1) // seat 1 is NON-active
	gs.Seats[1].Hand = append(gs.Seats[1].Hand, mox)

	err := CastSpell(gs, 1, mox, nil)
	if err == nil {
		t.Fatal("§117.1a: a non-active seat must NOT be able to cast a Mox (non-flash permanent) at sorcery speed")
	}
	if ce, ok := err.(*CastError); !ok || ce.Reason != "sorcery_speed_timing" {
		t.Fatalf("expected sorcery_speed_timing, got %v", err)
	}
}

func TestMoxCast_ActiveSeatMainPhaseAllowed(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.Active = 0
	gs.Phase, gs.Step = "main", "precombat_main"
	mox := moxCard(0)
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, mox)

	err := CastSpell(gs, 0, mox, nil)
	if ce, ok := err.(*CastError); ok && ce.Reason == "sorcery_speed_timing" {
		t.Fatalf("active player casting a Mox in its main phase with an empty stack must be allowed, got %v", err)
	}
	// The card must have left the hand (the cast proceeded past the gate).
	for _, c := range gs.Seats[0].Hand {
		if c == mox {
			t.Fatal("Mox should have left hand on a legal cast")
		}
	}
}

func TestMoxCast_ActiveSeatNonEmptyStackRejected(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.Active = 0
	gs.Phase, gs.Step = "main", "precombat_main"
	gs.Stack = append(gs.Stack, &StackItem{Kind: "spell"}) // something on the stack
	mox := moxCard(0)
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, mox)

	err := CastSpell(gs, 0, mox, nil)
	if ce, ok := err.(*CastError); !ok || ce.Reason != "sorcery_speed_timing" {
		t.Fatalf("§117.1a: a Mox cast with a non-empty stack must be rejected, got %v", err)
	}
}

func TestPermanentCast_FlashCreatureNonActiveAllowed(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.Active = 0
	gs.Phase, gs.Step = "main", "precombat_main"
	// Flash creature cast by the non-active seat — legal at instant speed.
	flash := &Card{
		Name: "Ambush Viper", Owner: 1, Types: []string{"creature"},
		BasePower: 2, BaseToughness: 1, ManaCostString: "{1}{G}",
		AST: &gameast.CardAST{Name: "Ambush Viper", Abilities: []gameast.Ability{
			&gameast.Keyword{Name: "flash"},
		}},
	}
	gs.Seats[1].Mana = nil
	gs.Seats[1].ManaPool = 5
	gs.Seats[1].Hand = append(gs.Seats[1].Hand, flash)

	err := CastSpell(gs, 1, flash, nil)
	if ce, ok := err.(*CastError); ok && ce.Reason == "sorcery_speed_timing" {
		t.Fatalf("a FLASH creature cast by a non-active seat must be exempt from the sorcery-speed gate, got %v", err)
	}
}

func TestPermanentCast_EffectDrivenExempt(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.Active = 0
	gs.Phase, gs.Step = "main", "precombat_main"
	// Simulate being mid-resolution ("you may cast …" free cast): the gate
	// must NOT fire even for a non-active seat / non-empty stack.
	gs.Flags["_resolve_frame_depth"] = 1
	gs.Stack = append(gs.Stack, &StackItem{Kind: "ability"})
	mox := moxCard(1)
	gs.Seats[1].Hand = append(gs.Seats[1].Hand, mox)

	err := CastSpell(gs, 1, mox, nil)
	if ce, ok := err.(*CastError); ok && ce.Reason == "sorcery_speed_timing" {
		t.Fatalf("an effect-driven cast (resolve_frame_depth>0) must be exempt from the sorcery-speed gate, got %v", err)
	}
}

// ===========================================================================
// Symptom 2 — CR §605.1a: conditional Mox mana must NOT be fabricated when the
// condition is unmet (no legendary for Mox Amber, no imprint for Chrome Mox).
// The real production path is ApplyArtifactMana (tapAllManaSources).
// ===========================================================================

func bfArtifact(gs *GameState, seat int, name string, types ...string) *Permanent {
	p := &Permanent{
		Card:      &Card{Name: name, Owner: seat, Types: types},
		Controller: seat, Owner: seat,
		Timestamp: gs.NextTimestamp(), Counters: map[string]int{}, Flags: map[string]int{},
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

func TestMoxAmber_NoLegendaryProducesNothing(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	mox := bfArtifact(gs, 0, "Mox Amber", "legendary", "artifact")

	pips, ok := ApplyArtifactMana(gs, gs.Seats[0], mox)
	if ok || pips != 0 {
		t.Fatalf("§605.1a: Mox Amber with NO legendary must produce no mana, got pips=%d ok=%v", pips, ok)
	}
	if gs.Seats[0].ManaPool != 0 {
		t.Errorf("mana pool must stay 0 (no phantom mana), got %d", gs.Seats[0].ManaPool)
	}
	if mox.Tapped {
		t.Errorf("Mox Amber must NOT tap when it can produce nothing")
	}
}

func TestMoxAmber_WithLegendaryProduces(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	mox := bfArtifact(gs, 0, "Mox Amber", "legendary", "artifact")
	leg := bfArtifact(gs, 0, "Goblin Chieftain", "legendary", "creature")
	leg.Card.Colors = []string{"R"}

	pips, ok := ApplyArtifactMana(gs, gs.Seats[0], mox)
	if !ok || pips != 1 {
		t.Fatalf("Mox Amber WITH a legendary must produce 1 mana, got pips=%d ok=%v", pips, ok)
	}
	if gs.Seats[0].ManaPool != 1 {
		t.Errorf("mana pool should be 1, got %d", gs.Seats[0].ManaPool)
	}
	if !mox.Tapped {
		t.Errorf("Mox Amber should be tapped after producing mana")
	}
}

func TestChromeMox_NoImprintProducesNothing(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	mox := bfArtifact(gs, 0, "Chrome Mox", "artifact")

	pips, ok := ApplyArtifactMana(gs, gs.Seats[0], mox)
	if ok || pips != 0 {
		t.Fatalf("Chrome Mox with NO imprinted card must produce no mana, got pips=%d ok=%v", pips, ok)
	}
	if gs.Seats[0].ManaPool != 0 || mox.Tapped {
		t.Errorf("no phantom mana / no tap; pool=%d tapped=%v", gs.Seats[0].ManaPool, mox.Tapped)
	}
}

func TestChromeMox_WithImprintProduces(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	mox := bfArtifact(gs, 0, "Chrome Mox", "artifact")
	mox.Flags["imprinted"] = 1
	mox.Flags["imprint_color_U"] = 1

	pips, ok := ApplyArtifactMana(gs, gs.Seats[0], mox)
	if !ok || pips != 1 {
		t.Fatalf("Chrome Mox WITH an imprinted card must produce 1 mana, got pips=%d ok=%v", pips, ok)
	}
	if gs.Seats[0].ManaPool != 1 || !mox.Tapped {
		t.Errorf("pool should be 1 and tapped; pool=%d tapped=%v", gs.Seats[0].ManaPool, mox.Tapped)
	}
}
