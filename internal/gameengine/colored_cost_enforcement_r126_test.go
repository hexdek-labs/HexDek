package gameengine

import "testing"

// r126 — CR §202.1a: a cost's coloured pips must be matched in type.
//
// Before r126 the cast path compared one integer against another, so a
// mono-green pool could cast {U}{U}. r124 is what made this testable at
// all: it got the printed cost onto the runtime Card.
//
// The permissive carve-outs are as load-bearing as the enforcement — a
// false negative blocks a legal cast, which is strictly worse than the
// bug — so each is pinned here with its reason.

func r126Card(cost string) *Card {
	return &Card{Name: "r126 " + cost, ManaCostString: cost, Types: []string{"instant"}}
}

func r126Pool(w, u, b, r, g, c, any int) *ColoredManaPool {
	return &ColoredManaPool{W: w, U: u, B: b, R: r, G: g, C: c, Any: any}
}

func TestColoredCostR126_WrongColourIsRejected(t *testing.T) {
	// Five green mana cannot cast a {U}{U} counterspell.
	if CanSupplyColoredPips(r126Pool(0, 0, 0, 0, 5, 0, 0), r126Card("{U}{U}")) {
		t.Error("{U}{U} payable from five GREEN mana — §202.1a requires matching the type")
	}
	// ...and four generic-worth of green cannot cast {3}{U} either.
	if CanSupplyColoredPips(r126Pool(0, 0, 0, 0, 4, 0, 0), r126Card("{3}{U}")) {
		t.Error("{3}{U} payable with no blue source")
	}
}

func TestColoredCostR126_RightColourIsAccepted(t *testing.T) {
	for _, tc := range []struct {
		cost string
		pool *ColoredManaPool
	}{
		{"{U}{U}", r126Pool(0, 2, 0, 0, 0, 0, 0)},          // exact
		{"{U}{U}", r126Pool(0, 1, 0, 0, 0, 0, 1)},          // one Any covers the second
		{"{U}{U}", r126Pool(0, 0, 0, 0, 0, 0, 2)},          // all Any
		{"{3}{U}", r126Pool(0, 1, 0, 0, 3, 0, 0)},          // generic is NOT this check's job
		{"{C}", r126Pool(0, 0, 0, 0, 0, 1, 0)},             // colorless pip, §106.1b
		{"{W}{U}{B}{R}{G}", r126Pool(1, 1, 1, 1, 1, 0, 0)}, // five colours
	} {
		if !CanSupplyColoredPips(tc.pool, r126Card(tc.cost)) {
			t.Errorf("%s wrongly REJECTED from pool %+v", tc.cost, tc.pool)
		}
	}
}

// Hybrid {W/U} is payable with either half — and the most-constrained-first
// order must not let a two-colour pip eat the only source a pure pip needed.
func TestColoredCostR126_Hybrid(t *testing.T) {
	if !CanSupplyColoredPips(r126Pool(1, 0, 0, 0, 0, 0, 0), r126Card("{W/U}")) {
		t.Error("{W/U} not payable with white")
	}
	if !CanSupplyColoredPips(r126Pool(0, 1, 0, 0, 0, 0, 0), r126Card("{W/U}")) {
		t.Error("{W/U} not payable with blue")
	}
	if CanSupplyColoredPips(r126Pool(0, 0, 1, 0, 0, 0, 0), r126Card("{W/U}")) {
		t.Error("{W/U} payable with BLACK")
	}
	// One white, one blue: {W}{W/U} must work (hybrid must take the blue).
	if !CanSupplyColoredPips(r126Pool(1, 1, 0, 0, 0, 0, 0), r126Card("{W}{W/U}")) {
		t.Error("{W}{W/U} rejected with exactly one W and one U — the hybrid should take the U")
	}
}

// THE CARVE-OUTS. Each of these would be a wrongly-blocked legal cast.
func TestColoredCostR126_PermissiveCarveOuts(t *testing.T) {
	// Phyrexian: {W/P} is payable with 2 life, no white required. 40 cards.
	if !CanSupplyColoredPips(r126Pool(0, 0, 0, 0, 0, 0, 0), r126Card("{1}{W/P}")) {
		t.Error("{1}{W/P} blocked with no white — Phyrexian is payable with life (§107.4f)")
	}
	// Twobrid: {2/W} is payable with 2 generic, no white required. 22 cards.
	if !CanSupplyColoredPips(r126Pool(0, 0, 0, 0, 0, 0, 0), r126Card("{2/W}")) {
		t.Error("{2/W} blocked with no white — twobrid is payable with generic (§107.4e)")
	}
	// No printed cost (tokens / engine-minted) — nothing to enforce.
	if !CanSupplyColoredPips(r126Pool(0, 0, 0, 0, 0, 0, 0), &Card{Name: "token"}) {
		t.Error("a card with no printed cost must never be blocked")
	}
	// Restricted mana (§106.4a) may legally pay this spell and this
	// function cannot tell, so it declines to judge.
	restricted := r126Pool(0, 0, 0, 0, 0, 0, 0)
	restricted.Restricted = []RestrictedMana{{Amount: 2, Restriction: "creature_spell_only"}}
	if !CanSupplyColoredPips(restricted, r126Card("{U}{U}")) {
		t.Error("a pool holding restricted mana must not be judged by this check")
	}
	// Nil pool / nil card are not a licence to block.
	if !CanSupplyColoredPips(nil, r126Card("{U}")) || !CanSupplyColoredPips(r126Pool(0, 0, 0, 0, 0, 0, 0), nil) {
		t.Error("nil inputs must be permissive")
	}
}
