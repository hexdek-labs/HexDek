package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// R60 event-Kind normalization wave 3 (follow-up to PR #830 lose_game
// + PR #849 untap normalize). Per_card sibling tests to the engine-side
// pin at `internal/gameengine/discard_canonical_routing_r60_test.go`.
//
// Audit found 4 per_card sites bypassing the canonical
// `gameengine.DiscardCard` helper with raw `seat.Hand = ...` splices
// + raw `seat.Graveyard = append(...)`:
//
//   - mox_diamond.go         — discard a land cost on ETB
//   - emet_selch.go          — loot-style draw+discard
//   - smellerbee_rebel_fighter.go — discard entire hand on attack
//   - per_card_batch_k_r60.go — discard-on-draw generic
//
// Each silently bypassed CR §702.34a Madness replacement, CR §702.187
// Mayhem tracking, Necropotence skip-draw rerouting, the
// `card_discarded` trigger (Liliana's Caress / Waste Not / Tergrid),
// and Turn.Discarded stat. This file pins the post-refactor behavior
// on each path.

// discardTestMadnessCard builds a card with the madness keyword in
// the per_card test package's namespace. Mirrors the engine-side
// newMadnessCard fixture.
func discardTestMadnessCard(name string, owner, cmc int, madnessArg string) *gameengine.Card {
	args := []any{}
	if madnessArg != "" {
		args = append(args, madnessArg)
	}
	return &gameengine.Card{
		Name:  name,
		Owner: owner,
		Types: []string{"instant"},
		CMC:   cmc,
		AST: &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "madness", Args: args},
			},
		},
	}
}

// discardTestPlainCard builds a no-Madness plain card. Mirrors the
// engine-side discardTestPlainCard / newPlainCardForMadness.
func discardTestPlainCard(name string, owner int) *gameengine.Card {
	return &gameengine.Card{
		Name:  name,
		Owner: owner,
		Types: []string{"instant"},
		AST:   &gameast.CardAST{Name: name},
	}
}

// graveyardContains reports whether the named card is in `seat`'s
// graveyard. Local helper distinct from the test/_test.go conventions
// in this package; safe because it's package-private to the test.
func graveyardContains(seat *gameengine.Seat, card *gameengine.Card) bool {
	for _, c := range seat.Graveyard {
		if c == card {
			return true
		}
	}
	return false
}

func exileContains(seat *gameengine.Seat, card *gameengine.Card) bool {
	for _, c := range seat.Exile {
		if c == card {
			return true
		}
	}
	return false
}

// TestMoxDiamond_DiscardRoutesThroughCanonical pins the Mox Diamond
// refactor: discarding the land cost must now bump Turn.Discarded
// and set Flags["discarded_N"]. Pre-fix the direct MoveCard call
// bypassed both.
func TestMoxDiamond_DiscardRoutesThroughCanonical(t *testing.T) {
	gs := newGame(t, 2)
	mox := addPerm(gs, 0, "Mox Diamond", "artifact")
	// Land in hand satisfies the cost.
	land := &gameengine.Card{
		Name:  "Mountain",
		Owner: 0,
		Types: []string{"land"},
		AST:   &gameast.CardAST{Name: "Mountain"},
	}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, land)

	startingDiscarded := gs.Seats[0].Turn.Discarded
	startingFlag := gs.Flags["discarded_0"]

	gameengine.InvokeETBHook(gs, mox)

	if gs.Seats[0].Turn.Discarded <= startingDiscarded {
		t.Errorf("Turn.Discarded = %d (start %d), want > start — Mox Diamond must route through DiscardCard", gs.Seats[0].Turn.Discarded, startingDiscarded)
	}
	if gs.Flags["discarded_0"] <= startingFlag {
		t.Errorf("Flags[discarded_0] = %d (start %d), want > start — Mox Diamond must route through DiscardCard", gs.Flags["discarded_0"], startingFlag)
	}
	if !graveyardContains(gs.Seats[0], land) {
		t.Errorf("the land cost should land in graveyard (no madness on a land)")
	}
}

// TestEmetSelch_DiscardRoutesMadnessReplacement is the load-bearing
// rules pin for Emet-Selch. A madness card discarded by Emet-Selch's
// loot must be exiled and made castable. Pre-fix the direct MoveCard
// graveyarded it silently.
func TestEmetSelch_DiscardRoutesMadnessReplacement(t *testing.T) {
	gs := newGame(t, 2)
	emet := addPerm(gs, 0, "Emet-Selch, Unsundered", "creature")
	// Top up library so drawOne succeeds before the discard step.
	addLibrary(gs, 0, "Top Card")
	// Madness card in hand. Emet picks the highest-CMC card to discard;
	// give Fiery Temper CMC 4 to ensure it's the pick.
	madness := discardTestMadnessCard("Fiery Temper", 0, 4, "{R}")
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, madness)

	gameengine.InvokeETBHook(gs, emet)

	if graveyardContains(gs.Seats[0], madness) {
		t.Errorf("Madness card must NOT go to graveyard — §702.34a routes to exile via DiscardCard")
	}
	if !exileContains(gs.Seats[0], madness) {
		t.Errorf("Madness card should be in exile after Emet-Selch loot")
	}
	if !gameengine.HasOpenMadnessWindow(gs, 0, madness) {
		t.Errorf("HasOpenMadnessWindow should be true after Emet-Selch's madness discard — OnDiscardMadness must run via DiscardCard")
	}
}

// TestSmellerbee_BulkDiscardRoutesThroughCanonical pins the
// Smellerbee-discards-entire-hand path: every card must increment
// Turn.Discarded individually, and a madness card in the hand must
// be exiled (CR §702.34a), not graveyard-dumped.
func TestSmellerbee_BulkDiscardRoutesThroughCanonical(t *testing.T) {
	gs := newGame(t, 2)
	smellerbee := addPerm(gs, 0, "Smellerbee, Rebel Fighter", "creature")
	smellerbee.Flags["attacking"] = 1
	// Two more attackers to satisfy `attackers > handSize` (handSize
	// will be 2 below).
	a2 := addPerm(gs, 0, "Attacker 2", "creature")
	a2.Flags["attacking"] = 1
	a3 := addPerm(gs, 0, "Attacker 3", "creature")
	a3.Flags["attacking"] = 1

	// Hand: one plain card + one madness card.
	plain1 := discardTestPlainCard("Plain A", 0)
	madness := discardTestMadnessCard("Fiery Temper", 0, 4, "{R}")
	gs.Seats[0].Hand = []*gameengine.Card{plain1, madness}

	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": smellerbee,
		"source_perm":   smellerbee,
	})

	// Both should be removed from hand. Plain goes to graveyard,
	// madness goes to exile.
	if got := gs.Seats[0].Turn.Discarded; got < 2 {
		t.Errorf("Turn.Discarded = %d, want >= 2 — both cards must route through DiscardCard individually", got)
	}
	if !graveyardContains(gs.Seats[0], plain1) {
		t.Errorf("Plain card should be in graveyard after Smellerbee discard")
	}
	if graveyardContains(gs.Seats[0], madness) {
		t.Errorf("Madness card must NOT be in graveyard — §702.34a routes to exile via DiscardCard")
	}
	if !exileContains(gs.Seats[0], madness) {
		t.Errorf("Madness card should be in exile (%d cards present)", len(gs.Seats[0].Exile))
	}
}

// TestMoxDiamond_EmitsCanonicalDiscardEvent pins the showmatch-
// facing Kind="discard" event still fires after the refactor.
// Guards against an over-eager normalization that would drop the
// LogEvent and break the spectator log feed.
func TestMoxDiamond_EmitsCanonicalDiscardEvent(t *testing.T) {
	gs := newGame(t, 2)
	mox := addPerm(gs, 0, "Mox Diamond", "artifact")
	land := &gameengine.Card{
		Name:  "Plains",
		Owner: 0,
		Types: []string{"land"},
		AST:   &gameast.CardAST{Name: "Plains"},
	}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, land)

	gameengine.InvokeETBHook(gs, mox)

	matched := 0
	for _, ev := range gs.EventLog {
		if ev.Kind == "discard" && ev.Source == "Mox Diamond" {
			matched++
		}
	}
	if matched != 1 {
		t.Errorf("expected exactly 1 Kind=\"discard\" event with Source=\"Mox Diamond\" (showmatch spectator log feed), got %d", matched)
	}
}
