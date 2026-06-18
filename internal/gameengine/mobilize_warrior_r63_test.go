package gameengine

// r63 mechanic-probe (MOBILIZE, CR §702.181). The generic ApplyMobilize path
// minted "Mercenary" tokens and EXILED them at end of turn — both wrong per
// CR §702.181a ("N 1/1 red Warrior creature tokens ... Sacrifice them at the
// beginning of the next end step"). It also hand-rolled the token append,
// bypassing the §614 token-doubler chain, and double-fired alongside the
// per_card Zurgo handlers (which carry the same parsed `mobilize` keyword).
//
// These regressions pin: (1) Warrior subtype tokens are minted through the
// canonical doubling chokepoint so Doubling Season doubles them; (2) the
// generic path stands down when a per_card handler owns the card's
// creature_attacks trigger (no double tokens); (3) the canonical token path
// runs (create_token event fires, marking the doubler/token-matters wiring).

import "testing"

// (1) Token doublers double the Warrior tokens (CR §702.181a + §614).
func TestApplyMobilize_DoublingSeasonDoublesWarriors(t *testing.T) {
	gs := newAetherGame4P(t)
	addDoublerSource(gs, 0, "Doubling Season", RegisterDoublingSeason)
	atk := mobilizeAttacker(gs, 0, 1, 2) // Mobilize 2

	ApplyMobilize(gs, atk, 0)

	tokens := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Flags != nil && p.Flags["mobilize_token"] == 1 {
			tokens++
		}
	}
	// Mobilize 2 under Doubling Season → 4 Warrior tokens.
	if tokens != 4 {
		t.Fatalf("Mobilize 2 under Doubling Season should mint 4 Warrior tokens, got %d", tokens)
	}
}

// (2) The generic path stands down when a per_card handler owns the card's
// creature_attacks trigger — otherwise Zurgo Stormrender / Zurgo, Thunder's
// Decree would get DOUBLE the tokens (per_card Warriors + generic tokens).
func TestFireMobilizeTriggers_PerCardOwnedStandsDown(t *testing.T) {
	gs := newAetherGame4P(t)
	// mobilizeAttacker names the card "Trial Driver"; pretend a per_card
	// handler owns its attack trigger.
	oldHook := HasTriggerHook
	HasTriggerHook = func(cardName, event string) bool {
		return cardName == "Trial Driver" && event == "creature_attacks"
	}
	defer func() { HasTriggerHook = oldHook }()

	carrier := mobilizeAttacker(gs, 0, 1, 1)
	FireMobilizeTriggers(gs, 0, []*Permanent{carrier})

	generic := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Flags != nil && p.Flags["mobilize_token"] == 1 {
			generic++
		}
	}
	if generic != 0 {
		t.Fatalf("generic mobilize must not fire when a per_card handler owns "+
			"creature_attacks (would double the tokens), got %d generic tokens", generic)
	}

	// Control: with no per_card owner the generic path DOES fire (proving the
	// gate, not an inert setup, is what suppressed it above).
	HasTriggerHook = func(cardName, event string) bool { return false }
	gs2 := newAetherGame4P(t)
	carrier2 := mobilizeAttacker(gs2, 0, 1, 1)
	FireMobilizeTriggers(gs2, 0, []*Permanent{carrier2})
	ungated := 0
	for _, p := range gs2.Seats[0].Battlefield {
		if p != nil && p.Flags != nil && p.Flags["mobilize_token"] == 1 {
			ungated++
		}
	}
	if ungated != 1 {
		t.Fatalf("ungated generic mobilize 1 should mint exactly 1 token, got %d", ungated)
	}
}

// (3) Tokens are minted through the canonical CreateCreatureToken path (fires
// create_token), not a raw append — the wiring token-doublers and
// token-matters payoffs depend on.
func TestApplyMobilize_UsesCanonicalTokenPath(t *testing.T) {
	gs := newAetherGame4P(t)
	atk := mobilizeAttacker(gs, 0, 1, 2) // Mobilize 2

	ApplyMobilize(gs, atk, 0)

	createEvents := 0
	for _, ev := range gs.EventLog {
		if ev.Kind == "create_token" {
			createEvents++
		}
	}
	if createEvents < 2 {
		t.Fatalf("expected >=2 create_token events from the canonical token path, got %d", createEvents)
	}
	// And the seat's per-turn creature-entered counter reflects both tokens.
	if got := gs.Seats[0].Turn.CreaturesEntered; got < 2 {
		t.Fatalf("expected CreaturesEntered >= 2 from mobilize 2, got %d", got)
	}
}
