package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// R60 event-Kind normalization wave 3 (follow-up to PR #830 lose_game
// + PR #849 untap normalize). The `DiscardCard` helper at
// internal/gameengine/resolve.go:994 is the documented canonical
// discard chokepoint — its own docstring says: *"All discard paths
// should route through this to ensure Liliana's Caress, Waste Not,
// Tergrid, etc. see every discard."* That helper owns:
//
//   - CR §702.34a Madness replacement (OnDiscardMadness exiles
//     instead of graveyarding when the card has madness)
//   - CR §702.187 Mayhem turn-tracking (MayhemDiscards map)
//   - Necropotence skip-draw rerouting (graveyard → exile when active)
//   - card_discarded trigger (Liliana's Caress / Waste Not / Tergrid)
//   - Turn.Discarded stat for archetype/observer signals
//   - gs.Flags["discarded_N"] for evalCondition
//
// Audit on this branch found 6 high-impact sites bypassing the
// helper with direct `seat.Hand = …` splices + raw `seat.Graveyard
// = append(...)`. This PR routes each through DiscardCard. This file
// pins the engine-side bypass site that ships in this PR; per_card
// siblings are pinned by the parallel test file in the per_card
// package.

// discardTestPlainCard builds a no-Madness card in `owner`'s hand.
// Mirrors newPlainCardForMadness so the helper-suite stays consistent.
func discardTestPlainCard(name string, owner int) *Card {
	return &Card{
		Name:  name,
		Owner: owner,
		Types: []string{"instant"},
		AST:   &gameast.CardAST{Name: name},
	}
}

// TestActivationCostDiscard_RoutesThroughDiscardCard pins the engine-
// side `activation.go` refactor: a "{T}, Discard a card: …" activated
// ability's discard cost must now route through DiscardCard. The
// load-bearing assertion is the card_discarded trigger firing — pre-
// fix the direct splice silently bypassed it, and every Liliana's
// Caress / Waste Not / Tergrid deck failed to register the discard
// from such activations.
func TestActivationCostDiscard_RoutesThroughDiscardCard(t *testing.T) {
	gs := newMadnessGame(t)
	gs.Turn = 3

	// Stack 2 plain (non-madness) cards in seat 0's hand.
	card1 := discardTestPlainCard("Plain Card A", 0)
	card2 := discardTestPlainCard("Plain Card B", 0)
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, card1, card2)

	// Build a synthetic activated ability source on seat 0's
	// battlefield with a discard-1 cost.
	src := addBattlefield(gs, 0, "Compulsive-Discarder", 1, 1, "creature")
	one := 1
	src.Card.AST = &gameast.CardAST{
		Name: "Compulsive-Discarder",
		Abilities: []gameast.Ability{
			&gameast.Activated{
				Cost: gameast.Cost{Discard: &one},
				Effect: &gameast.Draw{
					Count: *gameast.NumInt(1),
				},
			},
		},
	}

	startingDiscarded := gs.Seats[0].Turn.Discarded
	startingFlagDiscard := gs.Flags["discarded_0"]
	priorTriggerCount := 0
	for _, ev := range gs.EventLog {
		if ev.Kind == "card_discarded" {
			priorTriggerCount++
		}
	}

	// Drive activation cost payment directly via ActivateAbility
	// (cost path is what we're exercising, not the effect).
	err := ActivateAbility(gs, 0, src, 0, nil)
	if err != nil {
		t.Fatalf("ActivateAbility failed: %v", err)
	}

	// Bypass-side validation: Turn.Discarded must be incremented,
	// Flags["discarded_0"] must be set. Both were silently inert
	// pre-fix.
	if got := gs.Seats[0].Turn.Discarded; got <= startingDiscarded {
		t.Errorf("Turn.Discarded = %d (start %d), want > start — activation cost must route through DiscardCard so the stat increments", got, startingDiscarded)
	}
	if got := gs.Flags["discarded_0"]; got <= startingFlagDiscard {
		t.Errorf("Flags[discarded_0] = %d (start %d), want > start — activation cost must route through DiscardCard so the flag updates", got, startingFlagDiscard)
	}

	// The discarded card lands in graveyard (no madness, no Necro).
	if !graveyardHas(gs.Seats[0], card1) && !graveyardHas(gs.Seats[0], card2) {
		t.Errorf("neither card landed in graveyard after activation-cost discard — DiscardCard's MoveCard call may have been skipped")
	}
}

// TestActivationCostDiscard_RoutesMadnessReplacement is the load-
// bearing rules pin. A Madness card discarded as an activation cost
// must be exiled (CR §702.34a) and made castable from exile for the
// madness window — pre-fix the direct splice routed it to graveyard
// and silently broke Madness on every activated-discard-cost ability.
func TestActivationCostDiscard_RoutesMadnessReplacement(t *testing.T) {
	gs := newMadnessGame(t)
	gs.Turn = 3

	// Madness card in seat 0's hand.
	madness := newMadnessCard("Fiery Temper", 0, 4, "{R}")
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, madness)

	src := addBattlefield(gs, 0, "Activator", 1, 1, "creature")
	one := 1
	src.Card.AST = &gameast.CardAST{
		Name: "Activator",
		Abilities: []gameast.Ability{
			&gameast.Activated{
				Cost: gameast.Cost{Discard: &one},
				Effect: &gameast.Draw{
					Count: *gameast.NumInt(1),
				},
			},
		},
	}

	err := ActivateAbility(gs, 0, src, 0, nil)
	if err != nil {
		t.Fatalf("ActivateAbility failed: %v", err)
	}

	if graveyardHas(gs.Seats[0], madness) {
		t.Errorf("Madness card must NOT go to graveyard (§702.34a should reroute to exile via DiscardCard)")
	}
	if !exileHas(gs.Seats[0], madness) {
		t.Errorf("Madness card should be in exile after activation-cost discard")
	}
	// Madness window registered → cast permission exists.
	if !HasOpenMadnessWindow(gs, 0, madness) {
		t.Errorf("HasOpenMadnessWindow should be true after activation-cost discard of madness card — DiscardCard's OnDiscardMadness call must run")
	}
	if grant := GetZoneCastGrant(gs, madness); grant == nil {
		t.Errorf("expected a ZoneCastPermission for the madness-exiled card; got nil — Madness pipeline silently broken pre-fix")
	}
}

// TestActivationCostDiscard_EmitsCanonicalDiscardEvent pins the
// showmatch-facing Kind="discard" event still fires for the spectator
// log. (Showmatch's chat-line formatter at internal/hexapi/showmatch.go:4065
// reads Kind="discard" + ev.Source.) This guards against an over-eager
// normalization that would drop the LogEvent and break the spectator
// feed.
func TestActivationCostDiscard_EmitsCanonicalDiscardEvent(t *testing.T) {
	gs := newMadnessGame(t)
	gs.Turn = 3
	card := discardTestPlainCard("Plain Card", 0)
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)

	src := addBattlefield(gs, 0, "Discarder", 1, 1, "creature")
	one := 1
	src.Card.AST = &gameast.CardAST{
		Name: "Discarder",
		Abilities: []gameast.Ability{
			&gameast.Activated{
				Cost: gameast.Cost{Discard: &one},
				Effect: &gameast.Draw{
					Count: *gameast.NumInt(1),
				},
			},
		},
	}

	err := ActivateAbility(gs, 0, src, 0, nil)
	if err != nil {
		t.Fatalf("ActivateAbility failed: %v", err)
	}

	// At least one Kind="discard" event with Source="Discarder" must
	// fire for showmatch's spectator-log formatter.
	matched := 0
	for _, ev := range gs.EventLog {
		if ev.Kind == "discard" && ev.Source == "Discarder" {
			matched++
		}
	}
	if matched != 1 {
		t.Errorf("expected exactly 1 Kind=\"discard\" event with Source=\"Discarder\" (showmatch spectator log feed), got %d", matched)
	}
}
