package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// newTestPlaneswalker builds a planeswalker with the given loyalty and
// the given loyalty-cost Extra strings as its activated abilities
// (Thor's encoding: "+1" / "−3" / "0"), placed on seat 0's battlefield.
func newTestPlaneswalker(gs *GameState, loyalty int, loyaltyCosts ...string) *Permanent {
	abilities := make([]gameast.Ability, 0, len(loyaltyCosts))
	for _, lc := range loyaltyCosts {
		abilities = append(abilities, &gameast.Activated{
			Cost: gameast.Cost{Extra: []string{lc}},
			Raw:  lc + ": test loyalty ability",
		})
	}
	card := &Card{
		Name:  "Test Walker",
		Types: []string{"legendary", "planeswalker"},
		CMC:   4, Owner: 0,
		AST: &gameast.CardAST{Name: "Test Walker", Abilities: abilities},
	}
	perm := &Permanent{
		Card: card, Controller: 0, Owner: 0,
		Counters: map[string]int{"loyalty": loyalty},
		Flags:    map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)
	return perm
}

// TestLoyaltyPlusAbilityAddsCounters: a +1 ability adds one loyalty
// counter and charges no life.
func TestLoyaltyPlusAbilityAddsCounters(t *testing.T) {
	gs := newTestGameState(4)
	gs.Active = 0
	gs.Phase = "precombat_main"
	pw := newTestPlaneswalker(gs, 4, "+1")
	lifeBefore := gs.Seats[0].Life

	if err := ActivateAbility(gs, 0, pw, 0, nil); err != nil {
		t.Fatalf("+1 activation failed: %v", err)
	}
	if got := pw.Counters["loyalty"]; got != 5 {
		t.Errorf("loyalty after +1 = %d, want 5", got)
	}
	if gs.Seats[0].Life != lifeBefore {
		t.Errorf("life changed on +1: %d -> %d", lifeBefore, gs.Seats[0].Life)
	}
	if pw.Flags["loyalty_used_this_turn"] != 1 {
		t.Error("loyalty_used_this_turn not set after activation")
	}
}

// TestLoyaltyMinusAbilityRemovesCounters: a −3 (U+2212, as Thor emits)
// removes three loyalty counters and charges no life.
func TestLoyaltyMinusAbilityRemovesCounters(t *testing.T) {
	gs := newTestGameState(4)
	gs.Active = 0
	gs.Phase = "precombat_main"
	pw := newTestPlaneswalker(gs, 5, "−3")
	lifeBefore := gs.Seats[0].Life

	if err := ActivateAbility(gs, 0, pw, 0, nil); err != nil {
		t.Fatalf("-3 activation failed: %v", err)
	}
	if got := pw.Counters["loyalty"]; got != 2 {
		t.Errorf("loyalty after -3 = %d, want 2", got)
	}
	if gs.Seats[0].Life != lifeBefore {
		t.Errorf("life changed on -3: %d -> %d", lifeBefore, gs.Seats[0].Life)
	}
}

// TestLoyaltyMinusIllegalBelowCost: a −3 with only 2 loyalty is rejected
// before any state changes (CR §606.5).
func TestLoyaltyMinusIllegalBelowCost(t *testing.T) {
	gs := newTestGameState(4)
	gs.Active = 0
	gs.Phase = "precombat_main"
	pw := newTestPlaneswalker(gs, 2, "−3")
	lifeBefore := gs.Seats[0].Life

	err := ActivateAbility(gs, 0, pw, 0, nil)
	if err == nil {
		t.Fatal("-3 with 2 loyalty was allowed; want insufficient_loyalty")
	}
	if ce, ok := err.(*CastError); !ok || ce.Reason != "insufficient_loyalty" {
		t.Errorf("error = %v, want CastError{insufficient_loyalty}", err)
	}
	if got := pw.Counters["loyalty"]; got != 2 {
		t.Errorf("loyalty changed on rejected activation: %d, want 2", got)
	}
	if gs.Seats[0].Life != lifeBefore {
		t.Errorf("life changed on rejected activation: %d -> %d", lifeBefore, gs.Seats[0].Life)
	}
	if pw.Flags["loyalty_used_this_turn"] != 0 {
		t.Error("loyalty_used_this_turn set by a rejected activation")
	}
}

// TestLoyaltyLethalMinusDestroysViaSBA: paying exactly all remaining
// loyalty drops the walker to 0; the §704.5i SBA (run by the activation
// path's DrainStack) puts it into its owner's graveyard. No life charged.
func TestLoyaltyLethalMinusDestroysViaSBA(t *testing.T) {
	gs := newTestGameState(4)
	gs.Active = 0
	gs.Phase = "precombat_main"
	pw := newTestPlaneswalker(gs, 3, "−3")
	lifeBefore := gs.Seats[0].Life

	if err := ActivateAbility(gs, 0, pw, 0, nil); err != nil {
		t.Fatalf("lethal -3 activation failed: %v", err)
	}
	for _, p := range gs.Seats[0].Battlefield {
		if p == pw {
			t.Fatalf("walker still on battlefield at loyalty %d; §704.5i SBA did not fire",
				pw.Counters["loyalty"])
		}
	}
	inGrave := false
	for _, c := range gs.Seats[0].Graveyard {
		if c == pw.Card {
			inGrave = true
		}
	}
	if !inGrave {
		t.Error("walker card not in owner's graveyard after lethal minus")
	}
	if gs.Seats[0].Life != lifeBefore {
		t.Errorf("life changed on lethal minus: %d -> %d", lifeBefore, gs.Seats[0].Life)
	}
}

// TestLoyaltyOncePerTurn: the second loyalty activation in a turn is
// rejected without paying (CR §606.3).
func TestLoyaltyOncePerTurn(t *testing.T) {
	gs := newTestGameState(4)
	gs.Active = 0
	gs.Phase = "precombat_main"
	pw := newTestPlaneswalker(gs, 4, "+1")

	if err := ActivateAbility(gs, 0, pw, 0, nil); err != nil {
		t.Fatalf("first +1 activation failed: %v", err)
	}
	err := ActivateAbility(gs, 0, pw, 0, nil)
	if err == nil {
		t.Fatal("second loyalty activation in one turn was allowed")
	}
	if ce, ok := err.(*CastError); !ok || ce.Reason != "loyalty_already_used" {
		t.Errorf("error = %v, want CastError{loyalty_already_used}", err)
	}
	if got := pw.Counters["loyalty"]; got != 5 {
		t.Errorf("loyalty after rejected second activation = %d, want 5", got)
	}
}

// TestLoyaltyLegacyPayLifeEncodingRoutesToLoyalty: older datasets parsed
// "−N" into Cost.PayLife. On a planeswalker that must spend loyalty, not
// the player's life.
func TestLoyaltyLegacyPayLifeEncodingRoutesToLoyalty(t *testing.T) {
	gs := newTestGameState(4)
	gs.Active = 0
	gs.Phase = "precombat_main"
	pw := newTestPlaneswalker(gs, 5) // no Extra-encoded abilities
	three := 3
	pw.Card.AST.Abilities = []gameast.Ability{
		&gameast.Activated{Cost: gameast.Cost{PayLife: &three}, Raw: "-3: legacy encoding"},
	}
	lifeBefore := gs.Seats[0].Life

	if err := ActivateAbility(gs, 0, pw, 0, nil); err != nil {
		t.Fatalf("legacy PayLife minus activation failed: %v", err)
	}
	if got := pw.Counters["loyalty"]; got != 2 {
		t.Errorf("loyalty after legacy -3 = %d, want 2", got)
	}
	if gs.Seats[0].Life != lifeBefore {
		t.Errorf("legacy PayLife on a planeswalker charged life: %d -> %d", lifeBefore, gs.Seats[0].Life)
	}
}

// TestParseLoyaltyDelta pins the Cost.Extra loyalty encoding: explicit
// +/−/- signed integers and bare "0" parse; rider tags and channel costs
// must not.
func TestParseLoyaltyDelta(t *testing.T) {
	cases := []struct {
		in   string
		want int
		ok   bool
	}{
		{"+1", 1, true},
		{"+12", 12, true},
		{"−3", -3, true}, // U+2212 as Thor emits
		{"-7", -7, true}, // ASCII fallback
		{"0", 0, true},
		{"+X", 0, false},
		{"−X", 0, false},
		{"2", 0, false}, // unsigned: not a loyalty cost
		{"hellbent_rider", 0, false},
		{"discard this card", 0, false},
		{"-1/-1", 0, false},
		{"", 0, false},
	}
	for _, c := range cases {
		got, ok := parseLoyaltyDelta(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("parseLoyaltyDelta(%q) = (%d, %v), want (%d, %v)", c.in, got, ok, c.want, c.ok)
		}
	}
}
