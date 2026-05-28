package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// TestZoneCastGrantExpiry_HeistGrantStampsSourceTimestamp
// reproduces the ZoneCastGrantExpiry leak surfaced by the layer-
// stress 1000-game seed-42 sweep (PR #735): the heist arm at
// resolve_helpers.go:357 registered a "until end of turn" exile-
// cast grant via NewFreeCastFromExilePermission but never stamped
// SourceTimestamp on the returned ZoneCastPermission. When the
// source permanent (the heist effect's resolver — Cruelclaw, etc.)
// leaves the battlefield before EOT (destroyed, exiled, bounced),
// ExpireSourceGrants can't reap the grant (it short-circuits on
// sourceTimestamp == 0) and the only remaining cleanup path is
// EOT — which is skipped on mandatory-loop draw, mid-combat
// game-end, and the SBA-cap path.
//
// Fix: stamp perm.SourceTimestamp = src.Timestamp on the grant,
// mirroring the structured impulse_play arms at lines 1571 +
// 4857 that already do this.
//
// This test:
//   1. Builds a synthetic heist resolution against an opponent's
//      library
//   2. Asserts the resulting ZoneCastGrant carries
//      SourceTimestamp == src.Timestamp
//   3. Drives ExpireSourceGrants(src.Timestamp) and asserts the
//      grant is reaped (which it could not be pre-fix)
func TestZoneCastGrantExpiry_HeistGrantStampsSourceTimestamp(t *testing.T) {
	gs := NewGameState(2, nil, nil)

	// Source: the heist-resolving permanent on seat 0
	src := &Permanent{
		Card:       &Card{Name: "Cruelclaw, the Heister", Types: []string{"creature"}, Owner: 0},
		Controller: 0,
		Owner:      0,
		Timestamp:  gs.NextTimestamp(),
		Flags:      map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)

	// Opponent's library needs at least one card for the heist
	// resolution to pick the top card and grant a cast permission.
	heistTarget := &Card{Name: "Lightning Bolt", Types: []string{"instant"}, Owner: 1}
	gs.Seats[1].Library = append([]*Card{heistTarget}, gs.Seats[1].Library...)

	// Drive the heist resolution via resolveModificationEffect
	// with ModKind="heist".
	resolveModificationEffect(gs, src, &gameast.ModificationEffect{
		ModKind: "heist",
	})

	// The heist target should now be in seat 1's exile (moved
	// there by the resolution), and a ZoneCastGrant should be
	// registered against it.
	grant, ok := gs.ZoneCastGrants[heistTarget]
	if !ok {
		t.Fatal("heist resolution should have registered a ZoneCastGrant for the exiled target, got none")
	}
	if grant.Duration != "until_end_of_turn" {
		t.Errorf("grant Duration: want until_end_of_turn, got %q", grant.Duration)
	}
	if grant.GrantTurn != gs.Turn {
		t.Errorf("grant GrantTurn: want %d, got %d", gs.Turn, grant.GrantTurn)
	}
	// The canonical regression: SourceTimestamp must be stamped
	// so ExpireSourceGrants can reap on source-LTB.
	if grant.SourceTimestamp != src.Timestamp {
		t.Errorf("grant SourceTimestamp: want %d (src.Timestamp), got %d — without this stamp ExpireSourceGrants short-circuits and the grant survives source-LTB",
			src.Timestamp, grant.SourceTimestamp)
	}

	// Drive the canonical leak scenario: source leaves the
	// battlefield (e.g. destroyed) mid-turn. ExpireSourceGrants
	// should reap the heist grant. Pre-fix this is a no-op
	// because SourceTimestamp == 0 short-circuits the function.
	ExpireSourceGrants(gs, src.Timestamp)
	if _, stillThere := gs.ZoneCastGrants[heistTarget]; stillThere {
		t.Errorf("heist grant should have been reaped by ExpireSourceGrants(src.Timestamp), but it survived — the grant would now leak past EOT and trip ZoneCastGrantExpiry on next cleanup")
	}
}

// TestZoneCastGrantExpiry_MayPlayExiledFreeStampsSourceTimestamp
// covers the sister leak in the may_play_exiled_free arm at
// resolve_helpers.go:560 — same NewFreeCastFromExilePermission
// path, same SourceTimestamp-missing bug, fixed in the same PR.
func TestZoneCastGrantExpiry_MayPlayExiledFreeStampsSourceTimestamp(t *testing.T) {
	gs := NewGameState(2, nil, nil)

	src := &Permanent{
		Card:       &Card{Name: "Granting Source", Types: []string{"enchantment"}, Owner: 0},
		Controller: 0,
		Owner:      0,
		Timestamp:  gs.NextTimestamp(),
		Flags:      map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)

	// Put a card in the seat's own exile so may_play_exiled_free
	// has a target.
	exileTop := &Card{Name: "Counterspell", Types: []string{"instant"}, Owner: 0}
	gs.Seats[0].Exile = append(gs.Seats[0].Exile, exileTop)

	resolveModificationEffect(gs, src, &gameast.ModificationEffect{
		ModKind: "may_play_exiled_free",
	})

	grant, ok := gs.ZoneCastGrants[exileTop]
	if !ok {
		t.Fatal("may_play_exiled_free should have registered a grant for the exiled card, got none")
	}
	if grant.SourceTimestamp != src.Timestamp {
		t.Errorf("grant SourceTimestamp: want %d (src.Timestamp), got %d — missing stamp would let the grant survive source-LTB past EOT",
			src.Timestamp, grant.SourceTimestamp)
	}

	ExpireSourceGrants(gs, src.Timestamp)
	if _, stillThere := gs.ZoneCastGrants[exileTop]; stillThere {
		t.Errorf("may_play_exiled_free grant should have been reaped by ExpireSourceGrants, but it survived")
	}
}
