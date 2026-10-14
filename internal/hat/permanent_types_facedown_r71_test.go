package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// r71 — the permanent_types check must not flag a FACE-DOWN permanent.
//
// Live grinder output carried 91 critical "instants and sorceries
// cannot be permanents" violations naming ordinary spells — Village
// Rites, Disallow, Tale's End, Crop Rotation, Rampant Growth. None of
// them is a bug: manifest (CR §701.40) puts ANY card onto the
// battlefield face down as a 2/2 creature, and a face-down permanent
// has no types other than creature (CR §707.2). The check was reading
// the PRINTED type line of a card whose printed characteristics do not
// apply.
//
// The risk in fixing this is over-suppression, so the face-up case is
// pinned just as hard as the face-down one.

func r71Perm(gs *gameengine.GameState, name string, faceDown, manifested bool) *gameengine.Permanent {
	c := &gameengine.Card{
		Name:     name,
		Types:    []string{"sorcery"},
		TypeLine: "Sorcery",
		FaceDown: faceDown,
	}
	p := &gameengine.Permanent{Card: c, Controller: 0, Owner: 0, Flags: map[string]int{}}
	if manifested {
		p.Flags["manifested"] = 1
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, p)
	return p
}

func r71TypeHits(gs *gameengine.GameState) int {
	n := 0
	for _, v := range CheckGame(gs).Violations {
		if v.Name == "permanent_types" {
			n++
		}
	}
	return n
}

// A manifested sorcery is a legal 2/2 face-down creature.
func TestPermanentTypesR71_ManifestedSorceryIsNotAViolation(t *testing.T) {
	gs := gameengine.NewGameState(2, nil, nil)
	r71Perm(gs, "Rampant Growth", true, true)
	if hits := r71TypeHits(gs); hits != 0 {
		t.Errorf("a manifested sorcery is legal under CR §701.40/§707.2; got %d permanent_types violation(s)", hits)
	}
}

// Morph-style face-down (no manifest marker) is equally legal — the
// §707.2 shape comes from being face down, not from how it got there.
func TestPermanentTypesR71_AnyFaceDownIsNotAViolation(t *testing.T) {
	gs := gameengine.NewGameState(2, nil, nil)
	r71Perm(gs, "Village Rites", true, false)
	if hits := r71TypeHits(gs); hits != 0 {
		t.Errorf("a face-down permanent has no printed types to violate; got %d", hits)
	}
}

// THE CASE THE CHECK EXISTS FOR. A face-up instant or sorcery on the
// battlefield is impossible under any effect and must stay critical.
func TestPermanentTypesR71_FaceUpSorceryStillFlaggedCritical(t *testing.T) {
	gs := gameengine.NewGameState(2, nil, nil)
	r71Perm(gs, "Rampant Growth", false, false)

	hits, critical := 0, 0
	for _, v := range CheckGame(gs).Violations {
		if v.Name == "permanent_types" {
			hits++
			if v.Severity == "critical" {
				critical++
			}
		}
	}
	if hits == 0 {
		t.Fatal("a FACE-UP sorcery on the battlefield must still be flagged")
	}
	if critical == 0 {
		t.Error("the face-up case must stay critical severity")
	}
}

// A face-up permanent that is genuinely legal must stay clean — guards
// against the skip being widened into "never check anything".
func TestPermanentTypesR71_OrdinaryCreatureStaysClean(t *testing.T) {
	gs := gameengine.NewGameState(2, nil, nil)
	c := &gameengine.Card{Name: "Grizzly Bears", Types: []string{"creature"}, TypeLine: "Creature — Bear"}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield,
		&gameengine.Permanent{Card: c, Controller: 0, Owner: 0})
	if hits := r71TypeHits(gs); hits != 0 {
		t.Errorf("an ordinary creature must not be flagged; got %d", hits)
	}
}
