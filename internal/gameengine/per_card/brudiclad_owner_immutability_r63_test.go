package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// TestBrudiclad_CopyPreservesTokenOwner pins the OwnerImmutability fix
// (firespot r63 — "Phyrexian Myr Token on seat 3 has Permanent.Owner=1
// diverging from Card.Owner=3"). Brudiclad's combat trigger makes every
// OTHER token its controller controls become a copy of the chosen token.
// The rewrite previously stamped the new Card's Owner to the controller
// (`newCard.Owner = seat`), which is wrong for a token the seat CONTROLS
// but does not OWN (e.g. one stolen via Mob Rule): a copy effect changes
// characteristics, not ownership (CR §108.3 / §110.5a). That clobbered
// Card.Owner away from the immutable owner and diverged from the
// untouched Permanent.Owner.
//
// Setup: seat 3 controls Brudiclad and a 0/1 token it controls but does
// NOT own (owner = seat 1, the original creator). Brudiclad mints its
// 1/1 "Phyrexian Myr Token" (the best token, so it's the copy template)
// and rewrites the stolen token into a copy of it. After the trigger the
// stolen token must still be owned by seat 1 on BOTH fields.
func TestBrudiclad_CopyPreservesTokenOwner(t *testing.T) {
	gs := newGame(t, 4)
	gs.Active = 3
	gs.Turn = 1

	// Brudiclad under seat 3's control (owner == controller == 3).
	brudiclad := addPerm(gs, 3, "Brudiclad, Telchor Engineer",
		"legendary", "creature", "artifact")

	// A token seat 3 CONTROLS but seat 1 OWNS (stolen). Built by hand so
	// the two owner fields start consistent at the true owner (seat 1).
	stolen := &gameengine.Permanent{
		Card: &gameengine.Card{
			Name:          "Goblin Token",
			Owner:         1,
			InstanceID:    "t1OGgoblin01",
			Types:         []string{"token", "creature", "goblin"},
			BasePower:     0,
			BaseToughness: 1,
		},
		Controller: 3,
		Owner:      1,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}
	gs.Seats[3].Battlefield = append(gs.Seats[3].Battlefield, stolen)

	// Sanity: pre-trigger the stolen token's owner fields agree at seat 1.
	if stolen.Owner != 1 || stolen.Card.Owner != 1 {
		t.Fatalf("pre-state: stolen token owner should be seat 1, got perm=%d card=%d",
			stolen.Owner, stolen.Card.Owner)
	}

	gameengine.FireCardTrigger(gs, "combat_begin", map[string]interface{}{
		"active_seat": 3,
	})

	// The stolen token should now be a copy of the 1/1 Phyrexian Myr Token
	// (Brudiclad's minted, highest-statline template).
	if stolen.Card.DisplayName() != "Phyrexian Myr Token" {
		t.Errorf("expected stolen token rewritten to Phyrexian Myr Token, got %q",
			stolen.Card.DisplayName())
	}

	// The fix: owner is immutable. Both fields must still read seat 1 (its
	// creator), NOT seat 3 (the controller). Permanent.Owner == Card.Owner.
	if stolen.Card.Owner != 1 {
		t.Errorf("Card.Owner clobbered: want 1 (true owner), got %d", stolen.Card.Owner)
	}
	if stolen.Owner != 1 {
		t.Errorf("Permanent.Owner mutated: want 1, got %d", stolen.Owner)
	}
	if stolen.Owner != stolen.Card.Owner {
		t.Errorf("OwnerImmutability divergence: Permanent.Owner=%d != Card.Owner=%d",
			stolen.Owner, stolen.Card.Owner)
	}

	_ = brudiclad
}
