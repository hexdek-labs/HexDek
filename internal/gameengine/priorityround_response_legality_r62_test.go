package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// -----------------------------------------------------------------------------
// r62 — PriorityRound response-cast blind spot (legality-validator
// follow-up #4). Response casts bypass CastSpell entirely; until r62
// they paid bare manaCostOf (no cost statics), accepted whatever the
// policy returned (no timing gate), never validated announced targets,
// and never touched the legality validator's Begin/Finish hooks — the
// validator could not see these casts at all.
// -----------------------------------------------------------------------------

func respCounterCard(owner int, costType string, filterBase string) *Card {
	types := []string{"instant"}
	if costType != "" {
		types = append(types, costType)
	}
	return &Card{
		Name:  "Test Counter",
		Owner: owner,
		Types: types,
		AST: &gameast.CardAST{
			Name: "Test Counter",
			Abilities: []gameast.Ability{
				&gameast.Static{
					Modification: &gameast.Modification{
						ModKind: "spell_effect",
						Args: []interface{}{
							&gameast.CounterSpell{
								Target: gameast.Filter{Base: filterBase, Targeted: true},
							},
						},
					},
				},
			},
		},
	}
}

func pushIncomingSpell(gs *GameState, controller int, name string, types ...string) *StackItem {
	if len(types) == 0 {
		types = []string{"instant"}
	}
	item := &StackItem{
		Kind:       "spell",
		Controller: controller,
		Card:       &Card{Name: name, Owner: controller, Types: types},
	}
	gs.Stack = append(gs.Stack, item)
	return item
}

// (1) Cost statics now apply to response casts. Thalia taxes the
// noncreature counter +1; with exactly the printed cost in pool the
// response must be DECLINED (pre-r62 it cast for the bare printed
// cost), and with printed+1 it casts, draining the pool to zero.
func TestPriorityRound_Response_CostStaticsApply(t *testing.T) {
	build := func(pool int) (*GameState, *Card) {
		gs := newFixtureGame(t)
		gs.Active = 0
		thalia := &Permanent{
			Card:       &Card{Name: "Thalia, Guardian of Thraben", Types: []string{"creature"}},
			Controller: 0,
			Owner:      0,
		}
		gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, thalia)
		pushIncomingSpell(gs, 0, "Lightning Bolt")
		counter := respCounterCard(1, "cost:2", "spell")
		gs.Seats[1].Hand = append(gs.Seats[1].Hand, counter)
		gs.Seats[1].ManaPool = pool
		EnsureTypedPool(gs.Seats[1])
		return gs, counter
	}

	// Pool == printed cost (2) but Thalia makes the real cost 3: decline.
	gs, counter := build(2)
	PriorityRound(gs)
	inHand := false
	for _, c := range gs.Seats[1].Hand {
		if c == counter {
			inHand = true
		}
	}
	if !inHand {
		t.Fatal("response cast for its unmodified cost — Thalia's tax was ignored (pre-r62 bug)")
	}
	if gs.Seats[1].ManaPool != 2 {
		t.Fatalf("declined response must not spend mana, pool = %d", gs.Seats[1].ManaPool)
	}

	// Pool == modified cost (3): casts, pays 3.
	gs, counter = build(3)
	PriorityRound(gs)
	for _, c := range gs.Seats[1].Hand {
		if c == counter {
			t.Fatal("affordable response (with tax) was not cast")
		}
	}
	if gs.Seats[1].ManaPool != 0 {
		t.Fatalf("response should pay the MODIFIED cost 3, pool = %d (want 0)", gs.Seats[1].ManaPool)
	}
}

// (2) The legality validator now sees response casts: the Begin/Finish
// bracket fires (probe check observes the cast) and an honest response
// produces ZERO violations — in particular cost-paid (601.2f-h)
// reconciles because announcement and payment both use
// CalculateTotalCost now.
func TestPriorityRound_Response_VisibleToLegalityValidator(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	v := NewLegalityValidator(42)
	var seen []string
	v.Register(LegalityCheck{
		Name: "probe",
		Fn: func(_ *GameState, obs *LegalityObservation) []LegalityViolation {
			seen = append(seen, obs.ActionLabel())
			return nil
		},
	})
	gs.Legality = v

	thalia := &Permanent{
		Card:       &Card{Name: "Thalia, Guardian of Thraben", Types: []string{"creature"}},
		Controller: 0,
		Owner:      0,
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, thalia)
	pushIncomingSpell(gs, 0, "Lightning Bolt")
	counter := respCounterCard(1, "cost:2", "spell")
	gs.Seats[1].Hand = append(gs.Seats[1].Hand, counter)
	gs.Seats[1].ManaPool = 5
	EnsureTypedPool(gs.Seats[1])

	PriorityRound(gs)

	found := false
	for _, s := range seen {
		if s == "cast:Test Counter" {
			found = true
		}
	}
	if !found {
		t.Fatalf("legality validator never observed the response cast (saw %v) — the Begin/Finish bracket is missing", seen)
	}
	for _, viol := range v.Violations {
		t.Errorf("honest response cast flagged: %s", viol.String())
	}
	// The announced target must be stamped and visible on the pushed item.
	top := gs.Stack[len(gs.Stack)-1]
	if top.Card != counter {
		t.Fatalf("expected counter on top of stack, got %v", top.Card)
	}
	if len(top.Targets) != 1 || top.Targets[0].Kind != TargetKindStackItem || top.Targets[0].Stack == nil {
		t.Fatalf("response counter should carry its announced stack-item target, got %+v", top.Targets)
	}
}

// (3) CR 601.2c — a counter whose filter doesn't accept the incoming
// spell must be declined at announcement ("counter target creature
// spell" at a noncreature spell). The hat path never checked this.
func TestPriorityRound_Response_CounterFilterMismatchDeclined(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	pushIncomingSpell(gs, 0, "Lightning Bolt") // instant, not a creature spell
	counter := respCounterCard(1, "cost:1", "creature")
	gs.Seats[1].Hand = append(gs.Seats[1].Hand, counter)
	gs.Seats[1].ManaPool = 5
	EnsureTypedPool(gs.Seats[1])

	PriorityRound(gs)

	inHand := false
	for _, c := range gs.Seats[1].Hand {
		if c == counter {
			inHand = true
		}
	}
	if !inHand {
		t.Fatal("creature-spell counter was announced at a noncreature spell — 601.2c filter not enforced")
	}
	if gs.Seats[1].ManaPool != 5 {
		t.Fatalf("declined response must not spend mana, pool = %d", gs.Seats[1].ManaPool)
	}
	if len(gs.Stack) != 1 {
		t.Fatalf("stack should be unchanged, depth = %d", len(gs.Stack))
	}
}

// sorceryResponseHat returns a sorcery as its "response" — an illegal
// play the policy layer should never make, which the engine must now
// reject instead of pushing.
type sorceryResponseHat struct {
	GreedyHatStub
	sorcery *Card
}

func (h *sorceryResponseHat) ChooseResponse(gs *GameState, seatIdx int, top *StackItem) *StackItem {
	return &StackItem{Kind: "spell", Controller: seatIdx, Card: h.sorcery}
}

// (4) CR 117.1a/307.1 — a sorcery (or non-flash permanent) returned by
// the response policy is rejected; pre-r62 PriorityRound pushed
// whatever the hat returned with no timing gate.
func TestPriorityRound_Response_SorcerySpeedRejected(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	pushIncomingSpell(gs, 0, "Lightning Bolt")
	sorc := &Card{Name: "Lava Spike", Owner: 1, Types: []string{"sorcery", "cost:1"}}
	gs.Seats[1].Hand = append(gs.Seats[1].Hand, sorc)
	gs.Seats[1].Hat = &sorceryResponseHat{sorcery: sorc}
	gs.Seats[1].ManaPool = 5
	EnsureTypedPool(gs.Seats[1])

	PriorityRound(gs)

	if len(gs.Stack) != 1 {
		t.Fatalf("sorcery response was pushed mid-stack (depth %d) — 117.1a gate missing", len(gs.Stack))
	}
	if gs.Seats[1].ManaPool != 5 {
		t.Fatalf("rejected response must not spend mana, pool = %d", gs.Seats[1].ManaPool)
	}
}

// (5) No-false-positive pin: a plain legal counter response with no cost
// statics and no validator attached behaves exactly as before (casts,
// pays printed cost, removed from hand). Guards the bounded gates
// against over-rejection.
func TestPriorityRound_Response_PlainCounterStillWorks(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	incoming := pushIncomingSpell(gs, 0, "Lightning Bolt")
	counter := respCounterCard(1, "cost:2", "spell")
	gs.Seats[1].Hand = append(gs.Seats[1].Hand, counter)
	gs.Seats[1].ManaPool = 2
	EnsureTypedPool(gs.Seats[1])

	PriorityRound(gs)

	for _, c := range gs.Seats[1].Hand {
		if c == counter {
			t.Fatal("plain legal counter response was not cast — bounded gates over-reject")
		}
	}
	if gs.Seats[1].ManaPool != 0 {
		t.Fatalf("printed cost 2 should be paid in full, pool = %d", gs.Seats[1].ManaPool)
	}
	top := gs.Stack[len(gs.Stack)-1]
	if top.Card != counter {
		t.Fatalf("counter should top the stack")
	}
	if len(top.Targets) != 1 || top.Targets[0].Stack != incoming {
		t.Fatalf("announced target should reference the incoming spell, got %+v", top.Targets)
	}
}
