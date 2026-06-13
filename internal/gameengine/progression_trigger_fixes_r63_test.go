package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// progression_trigger_fixes_r63_test.go — regressions for the engine
// trigger-dispatch fixes the r63 PROGRESSION sweep surfaced.

// ---------------------------------------------------------------------------
// etb_or_another — "whenever ~ or another creature you control enters"
// was absent from the event alias table, so EventEquals(event,"etb") was
// false and BOTH the self cascade and the observer walk skipped it: the
// trigger was fully inert (Ambuscade Shaman, Kor Celebrant, Kapsho
// Kitefins, …). It must fire on the bearer's OWN ETB and on an ally's ETB.
// ---------------------------------------------------------------------------

func TestEtbOrAnother_Aliased(t *testing.T) {
	if !EventEquals("etb_or_another", "etb") {
		t.Fatal("etb_or_another must normalize to etb")
	}
	if !dualSelfObserverEvents["etb_or_another"] {
		t.Fatal("etb_or_another must be a dual self+observer event")
	}
}

func etbOrAnotherGainLifeCreature(seat int, gs *GameState) *Permanent {
	c := &Card{
		Name:  "Lifegain Sentinel",
		Owner: seat,
		Types: []string{"creature"},
		AST: &gameast.CardAST{Abilities: []gameast.Ability{
			&gameast.Triggered{
				Trigger: gameast.Trigger{Event: "etb_or_another"},
				Effect: &gameast.GainLife{
					Amount: gameast.NumberOrRef{IsInt: true, Int: 1},
					Target: gameast.Filter{Base: "self"},
				},
				Raw: "whenever this creature or another creature you control enters, you gain 1 life",
			},
		}},
		BasePower: 2, BaseToughness: 2,
	}
	return &Permanent{
		Card: c, Controller: seat, Owner: seat,
		Counters: map[string]int{}, Flags: map[string]int{},
		Timestamp: gs.NextTimestamp(),
	}
}

func TestEtbOrAnother_FiresOnSelfETB(t *testing.T) {
	gs := newTestGame(t, 2)
	before := gs.Seats[0].Life
	bearer := etbOrAnotherGainLifeCreature(0, gs)
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, bearer)
	FirePermanentETBTriggers(gs, bearer)
	if got := gs.Seats[0].Life - before; got != 1 {
		t.Fatalf("self-ETB: expected +1 life, got %+d", got)
	}
}

func TestEtbOrAnother_FiresOnAllyETB(t *testing.T) {
	gs := newTestGame(t, 2)
	// Bearer already on the battlefield.
	bearer := etbOrAnotherGainLifeCreature(0, gs)
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, bearer)
	before := gs.Seats[0].Life
	// An ally creature enters.
	ally := addTestPerm(gs, 0, "Ally Vanilla", "creature")
	FirePermanentETBTriggers(gs, ally)
	if got := gs.Seats[0].Life - before; got != 1 {
		t.Fatalf("ally-ETB: expected +1 life from observer, got %+d", got)
	}
}

func TestEtbOrAnother_DoesNotFireOnOpponentETB(t *testing.T) {
	gs := newTestGame(t, 2)
	bearer := etbOrAnotherGainLifeCreature(0, gs)
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, bearer)
	before := gs.Seats[0].Life
	// Opponent's creature enters — "you control" gate must keep it silent.
	opp := addTestPerm(gs, 1, "Opp Vanilla", "creature")
	FirePermanentETBTriggers(gs, opp)
	if got := gs.Seats[0].Life - before; got != 0 {
		t.Fatalf("opponent-ETB: expected no life change, got %+d", got)
	}
}

// ---------------------------------------------------------------------------
// Self-recursion die trigger — "when ~ dies, return this card to your
// hand" resolves after the source has left the battlefield (the card is in
// the graveyard). resolveBounce must move it graveyard→hand, not no-op.
// ---------------------------------------------------------------------------

func TestDieReturnSelf_GraveyardToHand(t *testing.T) {
	gs := newTestGame(t, 2)
	c := &Card{
		Name:  "Returning Brute",
		Owner: 0,
		Types: []string{"creature"},
		AST: &gameast.CardAST{Abilities: []gameast.Ability{
			&gameast.Triggered{
				Trigger: gameast.Trigger{Event: "die"},
				Effect:  &gameast.Bounce{Target: gameast.Filter{Base: "self"}, To: "owners_hand"},
				Raw:     "when this creature dies, return this card to your hand",
			},
		}},
		BasePower: 4, BaseToughness: 4,
	}
	bearer := &Permanent{
		Card: c, Controller: 0, Owner: 0,
		Counters: map[string]int{}, Flags: map[string]int{},
		Timestamp: gs.NextTimestamp(),
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, bearer)

	DestroyPermanent(gs, bearer, nil)

	inHand := false
	for _, hc := range gs.Seats[0].Hand {
		if hc == c {
			inHand = true
		}
	}
	if !inHand {
		t.Fatalf("expected the card in hand after death, hand=%v gy=%v", names(gs.Seats[0].Hand), names(gs.Seats[0].Graveyard))
	}
	for _, gc := range gs.Seats[0].Graveyard {
		if gc == c {
			t.Fatalf("card must not remain in the graveyard (CardIdentity): gy=%v", names(gs.Seats[0].Graveyard))
		}
	}
}

// ---------------------------------------------------------------------------
// distribute -1/-1 counters — "when ~ dies, distribute three -1/-1
// counters among one, two, or three target creatures". The verbose target
// base must resolve to a real on-battlefield creature, not self-fallback
// onto the dead source.
// ---------------------------------------------------------------------------

func TestSimplifyDistributeCreatureFilter(t *testing.T) {
	cases := []struct {
		base       string
		wantOK     bool
		wantYouCon bool
	}{
		{"one, two, or three target creatures", true, false},
		{"up to three target creatures", true, false},
		{"any number of target creatures you control", true, true},
		{"target creatures", true, false},
		{"target creatures you control", true, true},
		{"any number of target elves", false, false}, // typed — leave alone
		{"one or two other target vehicles and/or creatures you control", false, false},
	}
	for _, c := range cases {
		nf, ok := simplifyDistributeCreatureFilter(gameast.Filter{Base: c.base, Targeted: true})
		if ok != c.wantOK {
			t.Errorf("base=%q: ok=%v want %v", c.base, ok, c.wantOK)
			continue
		}
		if ok {
			if nf.Base != "creature" {
				t.Errorf("base=%q: simplified base=%q want creature", c.base, nf.Base)
			}
			if nf.YouControl != c.wantYouCon {
				t.Errorf("base=%q: YouControl=%v want %v", c.base, nf.YouControl, c.wantYouCon)
			}
		}
	}
}

// A "distribute N -1/-1 counters among <verbose> target creatures" effect
// resolving after the source has left the battlefield (Shambling Swarm's
// dies-trigger) must land its counters on a real on-battlefield creature —
// NOT no-op by self-falling-back onto the dead source.
func TestDistributeMinusCounters_DeadSourceHitsBattlefieldCreature(t *testing.T) {
	gs := newTestGame(t, 2)
	victim := addTestPerm(gs, 1, "Hapless Bear", "creature")
	victim.Card.BasePower = 4
	victim.Card.BaseToughness = 4

	src := addTestPerm(gs, 0, "Swarming Horror", "creature")
	src.Card.BasePower = 4
	src.Card.BaseToughness = 4
	// Source has already left the battlefield (it died) by the time the
	// distribute resolves — the §603.10 look-back condition.
	gs.removePermanent(src)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, src.Card)

	resolveCounterMod(gs, src, &gameast.CounterMod{
		Op:          "distribute",
		Count:       gameast.NumberOrRef{IsInt: true, Int: 3},
		CounterKind: "-1/-1",
		Target:      gameast.Filter{Base: "one, two, or three target creatures", Targeted: true},
	})

	if got := victim.Counters["-1/-1"]; got != 3 {
		t.Fatalf("expected 3 -1/-1 counters on the battlefield creature, got %d", got)
	}
	if got := src.Counters["-1/-1"]; got != 0 {
		t.Fatalf("dead source must not receive the counters (self-fallback guard), got %d", got)
	}
}

func names(cards []*Card) []string {
	out := make([]string, 0, len(cards))
	for _, c := range cards {
		if c != nil {
			out = append(out, c.Name)
		}
	}
	return out
}
