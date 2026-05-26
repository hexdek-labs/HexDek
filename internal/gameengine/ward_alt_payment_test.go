package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// newWardAltGame builds a 2-seat fixture for alt-payment ward tests.
// Seat 0 controls the warded permanent (Sauron / Saruman / Auntie Ool);
// seat 1 is the opponent who'd cast a targeting spell.
func newWardAltGame(t *testing.T) *GameState {
	t.Helper()
	rng := rand.New(rand.NewSource(7))
	return NewGameState(2, rng, nil)
}

// wardedPerm builds a permanent with the "ward" AST keyword and stamps
// the alt-payment flags onto it. Lives on seat 0's battlefield.
func wardedPerm(gs *GameState, name string, kind int, filter int, types ...string) *Permanent {
	c := &Card{
		Name:  name,
		Owner: 0,
		Types: types,
		AST: &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "ward"},
			},
		},
	}
	p := &Permanent{
		Card:       c,
		Controller: 0,
		Owner:      0,
		Flags: map[string]int{
			"kw:ward":         1,
			"ward_alt_kind":   kind,
			"ward_alt_filter": filter,
		},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, p)
	return p
}

func targetingItem(perm *Permanent) *StackItem {
	return &StackItem{
		Kind:       "spell",
		Controller: 1, // opponent's spell
		Card:       &Card{Name: "Doom Blade", Owner: 1, Types: []string{"instant"}},
		Targets:    []Target{{Kind: TargetKindPermanent, Permanent: perm}},
	}
}

// TestWardAlt_SacrificeLegendary_Pays — Sauron's ward shape.
// Opponent (caster) has a legendary creature; ward pays via
// sacrificing it; the targeting spell is NOT countered.
func TestWardAlt_SacrificeLegendary_Pays(t *testing.T) {
	gs := newWardAltGame(t)
	sauron := wardedPerm(gs, "Sauron, the Dark Lord",
		WardAltKindSacrificeLegendary, 0, "creature", "legendary")

	// Caster has a legendary creature available to sacrifice.
	legendary := &Permanent{
		Card: &Card{
			Name:          "Talrand, Sky Summoner",
			Owner:         1,
			Types:         []string{"creature", "legendary"},
			BasePower:     2,
			BaseToughness: 2,
		},
		Controller: 1,
		Owner:      1,
	}
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, legendary)

	item := targetingItem(sauron)
	CheckWardOnTargeting(gs, item)

	if item.Countered {
		t.Fatal("ward should have been paid by sacrificing the legendary creature")
	}
	// Talrand should be in seat 1's graveyard.
	if len(gs.Seats[1].Graveyard) != 1 || gs.Seats[1].Graveyard[0].Name != "Talrand, Sky Summoner" {
		t.Errorf("legendary creature should be sacrificed to graveyard, got %v", gs.Seats[1].Graveyard)
	}
	// Confirm ward_alt_paid event was logged.
	sawPaid := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "ward_alt_paid" {
			sawPaid = true
			break
		}
	}
	if !sawPaid {
		t.Error("expected a ward_alt_paid event")
	}
}

// TestWardAlt_SacrificeLegendary_CountersWhenNoLegendary — opponent
// has no legendary on the battlefield; ward can't be paid; spell is
// countered per CR §702.21c.
func TestWardAlt_SacrificeLegendary_CountersWhenNoLegendary(t *testing.T) {
	gs := newWardAltGame(t)
	sauron := wardedPerm(gs, "Sauron, the Dark Lord",
		WardAltKindSacrificeLegendary, 0, "creature", "legendary")

	// Caster controls only a non-legendary creature.
	plain := &Permanent{
		Card: &Card{
			Name:  "Grizzly Bears",
			Owner: 1,
			Types: []string{"creature"},
		},
		Controller: 1,
		Owner:      1,
	}
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, plain)

	item := targetingItem(sauron)
	CheckWardOnTargeting(gs, item)

	if !item.Countered {
		t.Fatal("ward should counter the spell — no legendary available to sacrifice")
	}
	// Plain creature should still be on the battlefield (not sacrificed).
	if len(gs.Seats[1].Graveyard) != 0 {
		t.Errorf("non-legendary should NOT be sacrificed; got %v in graveyard", gs.Seats[1].Graveyard)
	}
}

// TestWardAlt_DiscardInstSorcEnch_Pays — Saruman's ward shape.
// Opponent has a matching card in hand; ward is paid via discard;
// spell resolves.
func TestWardAlt_DiscardInstSorcEnch_Pays(t *testing.T) {
	gs := newWardAltGame(t)
	saruman := wardedPerm(gs, "Saruman of Many Colors",
		WardAltKindDiscardInstSorcEnch, 0, "creature", "legendary")

	// Caster has an instant in hand to discard.
	matching := &Card{Name: "Counterspell", Owner: 1, Types: []string{"instant", "cost:2"}}
	nonmatching := &Card{Name: "Grizzly Bears", Owner: 1, Types: []string{"creature"}}
	gs.Seats[1].Hand = []*Card{nonmatching, matching}

	item := targetingItem(saruman)
	CheckWardOnTargeting(gs, item)

	if item.Countered {
		t.Fatal("ward should have been paid by discarding the instant")
	}
	// Instant should be in graveyard; creature should still be in hand.
	if len(gs.Seats[1].Hand) != 1 || gs.Seats[1].Hand[0].Name != "Grizzly Bears" {
		t.Errorf("non-matching card should remain in hand, got %v", gs.Seats[1].Hand)
	}
	if len(gs.Seats[1].Graveyard) != 1 || gs.Seats[1].Graveyard[0].Name != "Counterspell" {
		t.Errorf("instant should be discarded to graveyard, got %v", gs.Seats[1].Graveyard)
	}
}

// TestWardAlt_DiscardInstSorcEnch_CountersWhenNoMatch — opponent
// has only creatures in hand; ward can't be paid; spell countered.
func TestWardAlt_DiscardInstSorcEnch_CountersWhenNoMatch(t *testing.T) {
	gs := newWardAltGame(t)
	saruman := wardedPerm(gs, "Saruman of Many Colors",
		WardAltKindDiscardInstSorcEnch, 0, "creature", "legendary")

	gs.Seats[1].Hand = []*Card{
		{Name: "Grizzly Bears", Owner: 1, Types: []string{"creature"}},
		{Name: "Forest", Owner: 1, Types: []string{"land", "basic"}},
	}

	item := targetingItem(saruman)
	CheckWardOnTargeting(gs, item)

	if !item.Countered {
		t.Fatal("ward should counter — no instant/sorcery/enchantment to discard")
	}
	// Hand should be unchanged.
	if len(gs.Seats[1].Hand) != 2 {
		t.Errorf("hand should be unchanged when ward counters, got %d cards", len(gs.Seats[1].Hand))
	}
}

// TestWardAlt_Blight_PutsCountersOnLowestToughnessCreature —
// Auntie Ool's ward shape. Opponent has 2 creatures; the lower-
// toughness one takes the 2 -1/-1 counters.
func TestWardAlt_Blight_PutsCountersOnLowestToughnessCreature(t *testing.T) {
	gs := newWardAltGame(t)
	auntie := wardedPerm(gs, "Auntie Ool, Cursewretch",
		WardAltKindBlight, 2 /* counters */, "creature", "legendary")

	// Caster has two creatures: a 4/4 and a 1/1.
	weenie := &Permanent{
		Card: &Card{
			Name: "Llanowar Elves", Owner: 1,
			Types:         []string{"creature"},
			BasePower:     1,
			BaseToughness: 1,
		},
		Controller: 1, Owner: 1,
		Flags: map[string]int{},
	}
	beater := &Permanent{
		Card: &Card{
			Name: "Phyrexian Obliterator", Owner: 1,
			Types:         []string{"creature"},
			BasePower:     5,
			BaseToughness: 5,
		},
		Controller: 1, Owner: 1,
		Flags: map[string]int{},
	}
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, beater, weenie)

	item := targetingItem(auntie)
	CheckWardOnTargeting(gs, item)

	if item.Countered {
		t.Fatal("ward should have been paid by blighting a creature")
	}
	if weenie.Counters["-1/-1"] != 2 {
		t.Errorf("expected 2 -1/-1 counters on Llanowar Elves (lowest toughness), got %d", weenie.Counters["-1/-1"])
	}
	if beater.Counters["-1/-1"] != 0 {
		t.Errorf("expected 0 -1/-1 counters on Obliterator (higher toughness), got %d", beater.Counters["-1/-1"])
	}
}

// TestWardAlt_Blight_CountersWhenNoCreature — opponent has no
// creatures; ward can't be paid; spell countered.
func TestWardAlt_Blight_CountersWhenNoCreature(t *testing.T) {
	gs := newWardAltGame(t)
	auntie := wardedPerm(gs, "Auntie Ool, Cursewretch",
		WardAltKindBlight, 2, "creature", "legendary")
	// Caster controls only a land.
	land := &Permanent{
		Card:       &Card{Name: "Forest", Owner: 1, Types: []string{"land", "basic"}},
		Controller: 1, Owner: 1,
	}
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, land)

	item := targetingItem(auntie)
	CheckWardOnTargeting(gs, item)

	if !item.Countered {
		t.Fatal("ward should counter — no creature to blight")
	}
}

// TestWardAlt_NoAltKindFallsThroughToManaWard — when ward_alt_kind
// is zero, the engine should fall through to the existing mana-ward
// pipeline (regression guard for the new branch not stealing flow
// from the mana path).
func TestWardAlt_NoAltKindFallsThroughToManaWard(t *testing.T) {
	gs := newWardAltGame(t)
	p := &Permanent{
		Card: &Card{
			Name:  "Generic Ward Creature",
			Owner: 0,
			Types: []string{"creature"},
			AST: &gameast.CardAST{
				Name: "Generic",
				Abilities: []gameast.Ability{
					&gameast.Keyword{Name: "ward"},
				},
			},
		},
		Controller: 0,
		Owner:      0,
		Flags: map[string]int{
			"kw:ward":   1,
			"ward_cost": 2, // mana ward {2}, NOT alt-payment
		},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, p)
	gs.Seats[1].ManaPool = 3

	item := targetingItem(p)
	CheckWardOnTargeting(gs, item)

	if item.Countered {
		t.Fatal("ward {2} with caster having {3} should be paid, not countered")
	}
	// Caster mana should be 3 - 2 = 1.
	if gs.Seats[1].ManaPool != 1 {
		t.Errorf("caster mana should be 1 after paying ward {2}, got %d", gs.Seats[1].ManaPool)
	}
}
