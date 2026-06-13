package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// manland_animate_r63_test.go — pins manland animation: the
// animate_subject shape (24 iconic manlands that were inert) and the
// type-revert at §514.2 cleanup (animated lands stop being creatures).

func mlLand(gs *GameState, seat int, name string) *Permanent {
	p := &Permanent{
		Card:       &Card{Name: name, Owner: seat, Types: []string{"land"}},
		Controller: seat, Owner: seat,
		Flags: map[string]int{}, Counters: map[string]int{},
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

func animateSubjectArgs(pt, descr, typ string) []interface{} {
	return []interface{}{
		[]interface{}{"subj", "this land"},
		[]interface{}{"pt", pt},
		[]interface{}{"descr", descr},
		[]interface{}{"type", typ},
	}
}

func TestManland_AnimateSubject_ExplicitPT_FaerieConclave(t *testing.T) {
	gs := newTestGameState(2)
	land := mlLand(gs, 0, "Faerie Conclave")
	resolveModificationEffect(gs, land, &gameast.ModificationEffect{
		ModKind: "animate_subject", Args: animateSubjectArgs("2/1", "blue faerie", "creature"),
	})
	if !land.IsCreature() {
		t.Fatalf("Faerie Conclave should be a creature after animation")
	}
	if land.Power() != 2 || land.Toughness() != 1 {
		t.Errorf("expected 2/1, got %d/%d", land.Power(), land.Toughness())
	}
	if !land.hasType("land") {
		t.Errorf("animated manland must still be a land (CR §706)")
	}
}

func TestManland_AnimateSubject_PTInDescr_Mutavault(t *testing.T) {
	gs := newTestGameState(2)
	land := mlLand(gs, 0, "Mutavault")
	// Mutavault carries P/T in descr with an empty pt arg.
	resolveModificationEffect(gs, land, &gameast.ModificationEffect{
		ModKind: "animate_subject", Args: animateSubjectArgs("", "2/2", "creature"),
	})
	if !land.IsCreature() || land.Power() != 2 || land.Toughness() != 2 {
		t.Fatalf("Mutavault should be a 2/2 creature, got creature=%v %d/%d",
			land.IsCreature(), land.Power(), land.Toughness())
	}
}

func TestManland_AnimateSubject_ArtifactCreature_Inkmoth(t *testing.T) {
	gs := newTestGameState(2)
	land := mlLand(gs, 0, "Inkmoth Nexus")
	resolveModificationEffect(gs, land, &gameast.ModificationEffect{
		ModKind: "animate_subject", Args: animateSubjectArgs("1/1", "phyrexian blinkmoth", "artifact creature"),
	})
	if !land.IsCreature() || !land.hasType("artifact") {
		t.Fatalf("Inkmoth Nexus should be an artifact creature, got types %v", land.Card.Types)
	}
	if !land.hasType("land") {
		t.Errorf("must still be a land")
	}
}

func TestManland_AnimateSubject_RevertsAtCleanup(t *testing.T) {
	gs := newTestGameState(2)
	land := mlLand(gs, 0, "Mutavault")
	resolveModificationEffect(gs, land, &gameast.ModificationEffect{
		ModKind: "animate_subject", Args: animateSubjectArgs("", "2/2", "creature"),
	})
	if !land.IsCreature() {
		t.Fatalf("should be a creature mid-turn")
	}
	// §514.2 cleanup.
	ScanExpiredDurations(gs, "ending", "cleanup")
	if land.IsCreature() {
		t.Errorf("animated land must stop being a creature at cleanup")
	}
	if !land.hasType("land") {
		t.Errorf("land type must persist through cleanup")
	}
	if land.Power() != 0 || land.Toughness() != 0 {
		t.Errorf("P/T modification must be removed at cleanup, got %d/%d", land.Power(), land.Toughness())
	}
	if len(land.AnimatedAddedTypes) != 0 {
		t.Errorf("AnimatedAddedTypes must be cleared, got %v", land.AnimatedAddedTypes)
	}
}

// The plain `animate` shape (Restless cycle) also reverts now.
func TestManland_Animate_RevertsAtCleanup(t *testing.T) {
	gs := newTestGameState(2)
	land := mlLand(gs, 0, "Creeping Tar Pit")
	resolveModificationEffect(gs, land, &gameast.ModificationEffect{
		ModKind: "animate", Args: []interface{}{3, 2, "elemental"},
	})
	if !land.IsCreature() || land.Power() != 3 || land.Toughness() != 2 {
		t.Fatalf("expected 3/2 creature, got creature=%v %d/%d", land.IsCreature(), land.Power(), land.Toughness())
	}
	ScanExpiredDurations(gs, "ending", "cleanup")
	if land.IsCreature() {
		t.Errorf("Restless-cycle land must stop being a creature at cleanup")
	}
	if !land.hasType("land") {
		t.Errorf("land type must persist")
	}
}

// Animating twice (two turns) must not accumulate duplicate types.
func TestManland_Animate_TwiceNoTypeAccumulation(t *testing.T) {
	gs := newTestGameState(2)
	land := mlLand(gs, 0, "Mutavault")
	for i := 0; i < 2; i++ {
		resolveModificationEffect(gs, land, &gameast.ModificationEffect{
			ModKind: "animate_subject", Args: animateSubjectArgs("", "2/2", "creature"),
		})
		ScanExpiredDurations(gs, "ending", "cleanup")
	}
	creatureCount := 0
	for _, tp := range land.Card.Types {
		if tp == "creature" {
			creatureCount++
		}
	}
	if creatureCount != 0 {
		t.Errorf("after cleanup land should have no creature type, got types %v", land.Card.Types)
	}
}
