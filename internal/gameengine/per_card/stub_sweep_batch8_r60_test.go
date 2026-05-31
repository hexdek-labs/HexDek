package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// TestUrtet_WUBRGActivationStampsThreeCountersOnEachMyr pins the
// activated ability: tap Urtet, put 3 +1/+1 counters on each Myr we
// control. Mana cost ({W}{U}{B}{R}{G}) is enforced upstream.
func TestUrtet_WUBRGActivationStampsThreeCountersOnEachMyr(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 0
	urtet := addPerm(gs, 0, "Urtet, Remnant of Memnarch", "legendary", "artifact", "creature", "myr")
	urtet.Card.BasePower = 0
	urtet.Card.BaseToughness = 4
	myr1 := addPerm(gs, 0, "Myr Token", "token", "artifact", "creature", "myr")
	myr1.Card.BasePower = 1
	myr1.Card.BaseToughness = 1
	myr2 := addPerm(gs, 0, "Myr Token", "token", "artifact", "creature", "myr")
	myr2.Card.BasePower = 1
	myr2.Card.BaseToughness = 1
	nonMyr := addPerm(gs, 0, "Sol Ring", "artifact")

	gameengine.InvokeActivatedHook(gs, urtet, 0, nil)

	if !urtet.Tapped {
		t.Errorf("Urtet should be tapped after activation")
	}
	if myr1.Counters["+1/+1"] != 3 {
		t.Errorf("myr1 should have 3 +1/+1 counters; got %d", myr1.Counters["+1/+1"])
	}
	if myr2.Counters["+1/+1"] != 3 {
		t.Errorf("myr2 should have 3 +1/+1 counters; got %d", myr2.Counters["+1/+1"])
	}
	// Urtet IS a Myr — should also get counters.
	if urtet.Counters["+1/+1"] != 3 {
		t.Errorf("Urtet (also a Myr) should have 3 +1/+1 counters; got %d", urtet.Counters["+1/+1"])
	}
	if nonMyr.Counters["+1/+1"] != 0 {
		t.Errorf("non-Myr artifact should not get counters; got %d", nonMyr.Counters["+1/+1"])
	}
}

// TestUrtet_FailsOnOpponentTurn pins the "Activate only during your
// turn" gate.
func TestUrtet_FailsOnOpponentTurn(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 1
	urtet := addPerm(gs, 0, "Urtet, Remnant of Memnarch", "legendary", "creature", "myr")
	myr := addPerm(gs, 0, "Myr Token", "creature", "myr")

	gameengine.InvokeActivatedHook(gs, urtet, 0, nil)

	if urtet.Tapped {
		t.Errorf("Urtet should not tap on opponent's turn")
	}
	if myr.Counters["+1/+1"] != 0 {
		t.Errorf("no counters should land on opponent's turn; got %d", myr.Counters["+1/+1"])
	}
}

// TestGolos_ActivatedExilesTopThreeAndRegistersFreeCastGrants pins
// the 5-color activated: exile top 3 + register free-cast grants.
func TestGolos_ActivatedExilesTopThreeAndRegistersFreeCastGrants(t *testing.T) {
	gs := newGame(t, 2)
	golos := addPerm(gs, 0, "Golos, Tireless Pilgrim", "legendary", "creature", "scout")
	golos.Card.BasePower = 3
	golos.Card.BaseToughness = 5
	c1 := addCard(gs, 0, "Top Card 1", "instant")
	c2 := addCard(gs, 0, "Top Card 2", "sorcery")
	c3 := addCard(gs, 0, "Top Card 3", "creature")
	gs.Seats[0].Library = append([]*gameengine.Card{c1, c2, c3}, gs.Seats[0].Library...)

	gameengine.InvokeActivatedHook(gs, golos, 0, nil)

	// Verify all 3 cards in exile.
	exileCount := 0
	for _, c := range gs.Seats[0].Exile {
		if c == c1 || c == c2 || c == c3 {
			exileCount++
		}
	}
	if exileCount != 3 {
		t.Errorf("expected 3 cards in exile; got %d", exileCount)
	}
	// Verify grants registered with ManaCost=0.
	for _, c := range []*gameengine.Card{c1, c2, c3} {
		grant, ok := gs.ZoneCastGrants[c]
		if !ok {
			t.Errorf("expected grant for %s; got none", c.DisplayName())
			continue
		}
		if grant.ManaCost != 0 {
			t.Errorf("%s: expected ManaCost=0 (free); got %d", c.DisplayName(), grant.ManaCost)
		}
		if grant.Duration != "until_end_of_turn" {
			t.Errorf("%s: expected Duration=until_end_of_turn; got %q", c.DisplayName(), grant.Duration)
		}
		if grant.SourceTimestamp != golos.Timestamp {
			t.Errorf("%s: expected SourceTimestamp=Golos.Timestamp; got %d vs %d",
				c.DisplayName(), grant.SourceTimestamp, golos.Timestamp)
		}
	}
}

// TestPestfinderPest_DiesGainsOneLife pins the wired Pest Token
// "When this token dies, you gain 1 life" trigger (Pestfinder-spawned
// Pests carry the pestfinder_die_lifegain flag).
func TestPestfinderPest_DiesGainsOneLife(t *testing.T) {
	gs := newGame(t, 2)
	startLife := gs.Seats[0].Life
	pest := addPerm(gs, 0, "Pest Token", "creature", "pest")
	// Real P/T so SBA 704.5f doesn't auto-kill the pest and re-fire
	// creature_dies via the destroy path — that would double-trigger
	// the lifegain via the per_card dispatcher's leaving-perm fallback.
	pest.Card.BasePower = 1
	pest.Card.BaseToughness = 1
	pest.Flags["pestfinder_die_lifegain"] = 1

	gameengine.FireCardTrigger(gs, "creature_dies", map[string]interface{}{
		"perm":            pest,
		"card":            pest.Card,
		"controller_seat": 0,
	})

	if gs.Seats[0].Life != startLife+1 {
		t.Errorf("expected life +1 after Pestfinder Pest dies; before=%d after=%d", startLife, gs.Seats[0].Life)
	}
}

// TestPestfinderPest_NonPestfinderPestNoLifegain pins the Flags gate:
// a Pest without the pestfinder_die_lifegain flag (e.g. Lluwen's
// attack-lifegain Pest) doesn't trigger.
func TestPestfinderPest_NonPestfinderPestNoLifegain(t *testing.T) {
	gs := newGame(t, 2)
	startLife := gs.Seats[0].Life
	pest := addPerm(gs, 0, "Pest Token", "creature", "pest")
	// No pestfinder_die_lifegain flag.

	gameengine.FireCardTrigger(gs, "creature_dies", map[string]interface{}{
		"perm":            pest,
		"card":            pest.Card,
		"controller_seat": 0,
	})

	if gs.Seats[0].Life != startLife {
		t.Errorf("non-Pestfinder pest must not gain life on dies; before=%d after=%d", startLife, gs.Seats[0].Life)
	}
}

// TestMizzixsMastery_NoStaleHatConsumePartial pins the cleanup: the
// resolve no longer emits the "hat_must_consume_grants" partial.
func TestMizzixsMastery_NoStaleHatConsumePartial(t *testing.T) {
	gs := newGame(t, 2)
	mastery := &gameengine.Card{Name: "Mizzix's Mastery", Owner: 0, Types: []string{"sorcery"}}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard,
		addCard(gs, 0, "Bolt", "instant"),
		addCard(gs, 0, "Wrath", "sorcery"),
	)
	item := &gameengine.StackItem{
		Controller: 0,
		Card:       mastery,
		Kind:       "spell",
	}
	gameengine.InvokeResolveHook(gs, item)

	for _, ev := range gs.EventLog {
		if ev.Kind == "per_card_partial" && ev.Source == "Mizzix's Mastery" {
			t.Errorf("expected no per_card_partial after batch 8 cleanup; got %+v", ev)
		}
	}
	// Verify the substantive behavior survives: grants were registered.
	grantCount := 0
	for c := range gs.ZoneCastGrants {
		if c != nil {
			for _, t := range c.Types {
				if t == "instant" || t == "sorcery" {
					grantCount++
					break
				}
			}
		}
	}
	if grantCount < 2 {
		t.Errorf("expected ≥2 grants registered; got %d", grantCount)
	}
}
