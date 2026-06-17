package gameengine

// eliminated_seat_token_orphan_r63_test.go — CONSERVATION eliminated-seat
// token-orphan class (r63). A token created for a seat that has already
// left the game (LeftGame=true) lands on that seat's battlefield, which the
// ZoneConservation census + orphan sweep both SKIP (invariants.go:218,
// instanceid_orphan_sweep.go:191). The token's InstanceID is therefore
// minted but never present and never ceased — a permanent "minted-not-ceased
// but absent from every zone" orphan (strict-census disappearance). The
// trigger that mints it (e.g. a Weirding Shaman "whenever a player casts a
// spell, create a 1/1 black Goblin Rogue" token) can resolve AFTER
// HandleSeatElimination's cleanup sweep, so cleanup can't retroactively cease
// it. CR §800.4a: a player who has left the game is no longer in the game and
// cannot create or control new objects. Fix: gate token creation on
// SeatHasLeftGame at both creation chokepoints (FireCreateTokenEvent +
// CreateCreatureToken), mirroring the established mana/trigger/zone-move
// LeftGame gates — refuse the token before it is ever minted.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// CreateCreatureToken (the non-AST path, bypasses FireCreateTokenEvent) must
// refuse to mint for a departed seat: nil return, no new minted ID, no token
// on the battlefield, census clean.
func TestEliminatedSeat_CreateCreatureTokenRefused(t *testing.T) {
	gs := newFixtureGame(t)
	// A live, minted card so the census actually engages (non-empty Minted set).
	live := &Card{Name: "Grizzly Bears", Owner: 1, Types: []string{"creature"}}
	MintOGInstanceID(gs, live)
	gs.Seats[1].Hand = append(gs.Seats[1].Hand, live)

	gs.Seats[0].LeftGame = true
	mintedBefore := len(gs.MintedInstanceIDs)

	perm := CreateCreatureToken(gs, 0, "Goblin Rogue", []string{"creature"}, 1, 1)
	if perm != nil {
		t.Fatal("CreateCreatureToken must return nil for a LeftGame seat (§800.4a)")
	}
	if len(gs.MintedInstanceIDs) != mintedBefore {
		t.Errorf("no token ID may be minted for a departed seat: before=%d after=%d",
			mintedBefore, len(gs.MintedInstanceIDs))
	}
	if n := len(gs.Seats[0].Battlefield); n != 0 {
		t.Errorf("departed seat must gain no token permanent: battlefield has %d", n)
	}
	if err, ok := ZoneConservationStrict(gs); !ok || err != nil {
		t.Errorf("strict census must be clean: ok=%v err=%v", ok, err)
	}
}

// FireCreateTokenEvent (the chokepoint for resolveCreateToken /
// resolveCreateTokenCopy / CreateDoubledTokens) must report cancelled for a
// departed seat.
func TestEliminatedSeat_FireCreateTokenEventCancelled(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Seats[0].LeftGame = true

	n, cancelled := FireCreateTokenEvent(gs, 0, 3, nil)
	if !cancelled || n != 0 {
		t.Errorf("FireCreateTokenEvent for a LeftGame seat must cancel: n=%d cancelled=%v", n, cancelled)
	}

	// A living seat is unaffected.
	if n, cancelled := FireCreateTokenEvent(gs, 1, 2, nil); cancelled || n != 2 {
		t.Errorf("living seat token creation must pass through: n=%d cancelled=%v", n, cancelled)
	}
}

// The AST path (resolveCreateToken — what a parsed "create a token" ability
// uses) must mint nothing when its controller has left, leaving the census
// clean. This is the actual leak shape: a token-making trigger controlled by a
// seat that has already been eliminated.
func TestEliminatedSeat_ASTTokenCreationNoOrphan(t *testing.T) {
	gs := newFixtureGame(t)
	live := &Card{Name: "Grizzly Bears", Owner: 1, Types: []string{"creature"}}
	MintOGInstanceID(gs, live)
	gs.Seats[1].Hand = append(gs.Seats[1].Hand, live)

	// The token-maker (e.g. Weirding Shaman) is controlled by the now-departed
	// seat 0. Even though the source permanent reference still exists, §800.4a
	// means its controller is gone and the token must not be created.
	src := &Permanent{
		Card:       &Card{Name: "Weirding Shaman", Owner: 0, Types: []string{"creature"}},
		Controller: 0,
		Owner:      0,
	}
	gs.Seats[0].LeftGame = true
	mintedBefore := len(gs.MintedInstanceIDs)

	resolveCreateToken(gs, src, &gameast.CreateToken{
		PT:    &[2]int{1, 1},
		Types: []string{"creature"},
		Color: []string{"black"},
	})

	if len(gs.MintedInstanceIDs) != mintedBefore {
		t.Errorf("AST token creation for a departed seat must mint nothing: before=%d after=%d",
			mintedBefore, len(gs.MintedInstanceIDs))
	}
	if n := len(gs.Seats[0].Battlefield); n != 0 {
		t.Errorf("departed seat must gain no token: battlefield has %d", n)
	}
	if err, ok := ZoneConservationStrict(gs); !ok || err != nil {
		t.Errorf("strict census must be clean: ok=%v err=%v", ok, err)
	}
}

// End-to-end window: a real seat is eliminated via HandleSeatElimination, then
// a trigger (resolving after the cleanup sweep) tries to mint a token for that
// now-departed seat. The token must be refused and conservation must hold —
// this is the precise sequence that produced the live "Goblin Rogue token
// present-but-not-in-(Minted-Ceased)" orphan.
func TestEliminatedSeat_PostEliminationTokenTrigger_NoConservationLeak(t *testing.T) {
	gs := newFixtureGame(t)

	// Seat 1 has a real creature + a card in hand, all minted.
	creature := addBattlefield(gs, 1, "Weirding Shaman", 2, 2, "creature")
	MintOGInstanceID(gs, creature.Card)
	held := &Card{Name: "Mountain", Owner: 1, Types: []string{"land"}}
	MintOGInstanceID(gs, held)
	gs.Seats[1].Hand = append(gs.Seats[1].Hand, held)

	// Seat 0 (the one that will leave) has a minted card so the elimination
	// cleanup has owned objects to cease.
	doomed := &Card{Name: "Llanowar Elves", Owner: 0, Types: []string{"creature"}}
	MintOGInstanceID(gs, doomed)
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, doomed)

	if err, ok := ZoneConservationStrict(gs); !ok || err != nil {
		t.Fatalf("fixture census not clean before elimination: ok=%v err=%v", ok, err)
	}

	HandleSeatElimination(gs, 0)
	if !gs.Seats[0].LeftGame {
		t.Fatal("fixture failure: seat 0 not marked LeftGame")
	}

	// A trigger resolving AFTER the cleanup sweep tries to make a token for the
	// departed seat. Both the AST and direct paths must no-op.
	if p := CreateCreatureToken(gs, 0, "Goblin Rogue", []string{"creature"}, 1, 1); p != nil {
		t.Error("post-elimination token (direct path) must be refused")
	}
	resolveCreateToken(gs, &Permanent{
		Card:       &Card{Name: "Weirding Shaman", Owner: 0},
		Controller: 0, Owner: 0,
	}, &gameast.CreateToken{PT: &[2]int{1, 1}, Types: []string{"creature"}})

	if err, ok := ZoneConservationStrict(gs); !ok || err != nil {
		t.Errorf("strict census must stay clean through the post-elimination token window: ok=%v err=%v", ok, err)
	}
}
