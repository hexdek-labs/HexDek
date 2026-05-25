package gameengine

// r60 — pin the expiry contract on the remaining "cast-from-exile"
// grants produced by resolveModificationEffect's "heist" and
// "may_play_exiled_free" arms.
//
// Bug history (Loki r41, 8 ZoneCastGrantExpiry violations): both arms
// built a NewFreeCastFromExilePermission with no Duration / GrantTurn,
// so a grant whose oracle text declared a "until end of turn" / "this
// turn" window outlived its declared expiry. ExpireZoneCastGrants
// couldn't reclaim it and the ZoneCastGrantExpiry invariant tripped.
//
// The shared NewFreeCastFromExilePermission helper is intentionally
// duration-less — Opposition Agent, Release to the Wind, and Dauthi
// Voidwalker all use it for "as long as exiled" / "while source in
// play" semantics. Each EOT-bound caller must therefore stamp Duration
// + GrantTurn on the returned permission itself.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// TestHeist_GrantExpiresAtEndOfTurn verifies the "heist" modification
// kind produces a ZoneCastPermission bound to EOT so cleanup reclaims
// it (matches oracle text "Until end of turn, you may play that
// card.").
func TestHeist_GrantExpiresAtEndOfTurn(t *testing.T) {
	gs := r59MakeGameState(4)
	// Heist target = opponent (seat 1); seed their library top.
	victimTop := &Card{Name: "Victim Top", Owner: 1}
	gs.Seats[1].Library = []*Card{victimTop}
	src := r59NewSourcePerm(gs, 0, "Heist Source")

	resolveModificationEffect(gs, src, &gameast.ModificationEffect{ModKind: "heist"})

	if len(gs.ZoneCastGrants) != 1 {
		t.Fatalf("heist should register one grant, got %d", len(gs.ZoneCastGrants))
	}
	grant := gs.ZoneCastGrants[victimTop]
	if grant == nil {
		t.Fatalf("grant for heisted card missing from ZoneCastGrants")
	}
	if grant.Duration != "until_end_of_turn" {
		t.Errorf("Duration: want until_end_of_turn, got %q", grant.Duration)
	}
	if grant.GrantTurn != 4 {
		t.Errorf("GrantTurn: want 4, got %d", grant.GrantTurn)
	}
	if grant.RequireController != 0 {
		t.Errorf("RequireController: want 0 (heist controller), got %d", grant.RequireController)
	}

	ExpireZoneCastGrants(gs)
	if _, still := gs.ZoneCastGrants[victimTop]; still {
		t.Errorf("heist grant should be reclaimed at cleanup of its grant turn")
	}
	if err := checkZoneCastGrantExpiry(gs); err != nil {
		t.Errorf("ZoneCastGrantExpiry invariant should be clean after cleanup: %v", err)
	}
}

// TestMayPlayExiledFree_GrantExpiresAtEndOfTurn verifies the
// "may_play_exiled_free" modification kind produces an EOT-bound
// grant on the most recently exiled card (matches the "this turn"
// family of oracle wordings — the parallel "as long as it remains
// exiled" wording is routed through resolveResidualByText and was
// already pinned in r59).
func TestMayPlayExiledFree_GrantExpiresAtEndOfTurn(t *testing.T) {
	gs := r59MakeGameState(6)
	exileTop := &Card{Name: "Exile Top", Owner: 0}
	gs.Seats[0].Exile = []*Card{exileTop}
	src := r59NewSourcePerm(gs, 0, "Exile Granter")

	resolveModificationEffect(gs, src, &gameast.ModificationEffect{ModKind: "may_play_exiled_free"})

	grant := gs.ZoneCastGrants[exileTop]
	if grant == nil {
		t.Fatalf("may_play_exiled_free should register a grant on the exile-top card")
	}
	if grant.Duration != "until_end_of_turn" {
		t.Errorf("Duration: want until_end_of_turn, got %q", grant.Duration)
	}
	if grant.GrantTurn != 6 {
		t.Errorf("GrantTurn: want 6, got %d", grant.GrantTurn)
	}
	if grant.ManaCost != 0 {
		t.Errorf("ManaCost: want 0 (free cast), got %d", grant.ManaCost)
	}

	ExpireZoneCastGrants(gs)
	if _, still := gs.ZoneCastGrants[exileTop]; still {
		t.Errorf("may_play_exiled_free grant should be reclaimed at cleanup")
	}
	if err := checkZoneCastGrantExpiry(gs); err != nil {
		t.Errorf("ZoneCastGrantExpiry invariant should be clean after cleanup: %v", err)
	}
}

// TestNewFreeCastFromExilePermission_NoTurnClock pins the shared
// helper's contract: it must NOT carry a Duration on its own, so
// "as long as exiled" / "while source in play" callers (Release to
// the Wind, Opposition Agent, Dauthi Voidwalker) keep their grant
// alive across turn boundaries. Per-caller stamping is the only way
// to opt into EOT expiry.
func TestNewFreeCastFromExilePermission_NoTurnClock(t *testing.T) {
	perm := NewFreeCastFromExilePermission(2, "Release to the Wind")
	if perm.Duration != "" {
		t.Errorf("NewFreeCastFromExilePermission must default to no turn clock; got Duration=%q", perm.Duration)
	}
	if perm.GrantTurn != 0 {
		t.Errorf("NewFreeCastFromExilePermission must default GrantTurn=0; got %d", perm.GrantTurn)
	}
	if perm.RequireController != 2 {
		t.Errorf("RequireController plumbing broken: want 2, got %d", perm.RequireController)
	}
	if perm.SourceName != "Release to the Wind" {
		t.Errorf("SourceName plumbing broken: got %q", perm.SourceName)
	}
}
