package gameengine

import (
	"testing"
)

// exile_linked_commander_unlink_r63_test.go — seed-42 game-486 leak
// (the 1-in-500 strict-census residual).
//
// Banisher Priest linked-exiled seat 1's COMMANDER. The §704.6d/§903.9a
// SBA then moved the commander exile → command zone by direct slice
// splice, leaving the Priest's ExiledByMe/LinkedExile claiming a card no
// longer in any exile — 24 ExileLinkageIntegrity violations until the
// Priest left play, and the owner re-cast the commander while the Priest
// still "held" it.
//
// Fixes pinned here:
//  1. sba704_6d severs the card-side linkage (UnlinkExiledCard) when it
//     moves a commander out of exile (or graveyard) — §406.8: an
//     "until ~ leaves" effect loses track of a card that changes zones.
//  2. ExileLinked stamps linkage only when the card actually LANDS in
//     exile (a §614 replacement may redirect the destination).

// commanderInExileShape: seat 1's commander, linked-exiled by seat 0's
// Banisher Priest shape. Returns (priest, commanderCard).
func commanderInExileShape(t *testing.T, gs *GameState) (*Permanent, *Card) {
	t.Helper()
	gs.CommanderFormat = true
	priest := banisherPriestShape(t, gs, 0)
	prey := targetCardOnBattlefield(t, gs, 1)
	gs.Seats[1].CommanderNames = []string{prey.Card.DisplayName()}

	gs.Seats[1].Battlefield = gs.Seats[1].Battlefield[:0]
	ExileLinked(gs, priest, prey.Card, prey.Owner, "battlefield")

	if len(priest.ExiledByMe) != 1 {
		t.Fatalf("setup: linkage did not stamp (ExiledByMe=%v)", priest.ExiledByMe)
	}
	if len(gs.Seats[1].Exile) != 1 {
		t.Fatalf("setup: commander not in owner's exile")
	}
	return priest, prey.Card
}

// TestSBA704_6d_UnlinksCommanderFromExiler is the game-486 pin: after
// the SBA moves the linked-exiled commander to the command zone, the
// source must no longer claim it and the invariant must be clean.
// FAILS pre-fix (ExiledByMe retained → ExileLinkageIntegrity error).
func TestSBA704_6d_UnlinksCommanderFromExiler(t *testing.T) {
	gs := newPhase3GameState(t)
	priest, cmdr := commanderInExileShape(t, gs)

	if !sba704_6d(gs) {
		t.Fatalf("sba704_6d did not act on the exiled commander")
	}

	if len(gs.Seats[1].CommandZone) != 1 || gs.Seats[1].CommandZone[0] != cmdr {
		t.Fatalf("commander not moved to command zone")
	}
	if len(gs.Seats[1].Exile) != 0 {
		t.Fatalf("commander still in exile after §704.6d")
	}
	if len(priest.ExiledByMe) != 0 || len(priest.LinkedExile) != 0 {
		t.Fatalf("source still claims the redirected commander: ExiledByMe=%v LinkedExile=%d",
			priest.ExiledByMe, len(priest.LinkedExile))
	}
	if cmdr.ExiledByTimestamp != 0 {
		t.Fatalf("ExiledByTimestamp not reset: %d", cmdr.ExiledByTimestamp)
	}
	if err := checkExileLinkageIntegrity(gs); err != nil {
		t.Fatalf("ExileLinkageIntegrity after §704.6d redirect: %v", err)
	}
}

// TestSBA704_6d_NonCommanderLinkageUntouched is the control: a linked-
// exiled NON-commander stays in exile with its linkage intact.
func TestSBA704_6d_NonCommanderLinkageUntouched(t *testing.T) {
	gs := newPhase3GameState(t)
	gs.CommanderFormat = true
	priest := banisherPriestShape(t, gs, 0)
	prey := targetCardOnBattlefield(t, gs, 1)
	// seat 1's commander is some OTHER name.
	gs.Seats[1].CommanderNames = []string{"Klauth, Unrivaled Ancient"}

	gs.Seats[1].Battlefield = gs.Seats[1].Battlefield[:0]
	ExileLinked(gs, priest, prey.Card, prey.Owner, "battlefield")

	sba704_6d(gs)

	if len(gs.Seats[1].Exile) != 1 {
		t.Fatalf("non-commander left exile")
	}
	if len(priest.ExiledByMe) != 1 || prey.Card.ExiledByTimestamp != priest.Timestamp {
		t.Fatalf("guard over-fired: non-commander linkage severed")
	}
	if err := checkExileLinkageIntegrity(gs); err != nil {
		t.Fatalf("invariant: %v", err)
	}
}

// TestExileLinked_RedirectedExileStampsNoLinkage pins the ExileLinked
// hardening: when a §614 replacement redirects the exile to another
// zone, no linkage may form (the card was never in exile).
func TestExileLinked_RedirectedExileStampsNoLinkage(t *testing.T) {
	gs := newPhase3GameState(t)
	priest := banisherPriestShape(t, gs, 0)
	prey := targetCardOnBattlefield(t, gs, 1)

	// Register a §614 replacement: this card's exile becomes graveyard
	// (the Leyline-of-the-Void-inverse shape; stands in for any
	// destination-rewriting replacement).
	gs.RegisterReplacement(&ReplacementEffect{
		EventType: "would_change_zone",
		HandlerID: "test_exile_redirect",
		Timestamp: gs.NextTimestamp(),
		Category:  CategoryOther,
		Applies: func(gs *GameState, ev *ReplEvent) bool {
			return ev.String("card_name") == prey.Card.DisplayName() &&
				ev.String("to_zone") == "exile"
		},
		ApplyFn: func(gs *GameState, ev *ReplEvent) {
			ev.Payload["to_zone"] = "graveyard"
		},
	})

	gs.Seats[1].Battlefield = gs.Seats[1].Battlefield[:0]
	ExileLinked(gs, priest, prey.Card, prey.Owner, "battlefield")

	if len(gs.Seats[1].Graveyard) != 1 {
		t.Fatalf("redirect did not route the card to the graveyard")
	}
	if len(priest.ExiledByMe) != 0 || len(priest.LinkedExile) != 0 {
		t.Fatalf("linkage stamped for a card that never reached exile: ExiledByMe=%v", priest.ExiledByMe)
	}
	if prey.Card.ExiledByTimestamp != 0 {
		t.Fatalf("ExiledByTimestamp stamped on redirected card")
	}
	if err := checkExileLinkageIntegrity(gs); err != nil {
		t.Fatalf("invariant: %v", err)
	}
}
