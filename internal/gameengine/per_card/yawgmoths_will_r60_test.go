package per_card

import (
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// -----------------------------------------------------------------------------
// R60 — Yawgmoth's Will family
// -----------------------------------------------------------------------------

// addCardToGraveyard appends a card to a seat's graveyard.
func addCardToGraveyard(gs *gameengine.GameState, seat int, name string, types ...string) *gameengine.Card {
	c := &gameengine.Card{Name: name, Owner: seat, Types: append([]string{}, types...)}
	gs.Seats[seat].Graveyard = append(gs.Seats[seat].Graveyard, c)
	return c
}

// invokeSorceryResolve manufactures a StackItem and runs the resolve hook
// so we can exercise OnResolve handlers directly. Mirrors how the stack
// invokes per_card resolvers.
func invokeSorceryResolve(gs *gameengine.GameState, controller int, name string) *gameengine.StackItem {
	item := &gameengine.StackItem{
		Card:       &gameengine.Card{Name: name, Owner: controller, Types: []string{"sorcery"}},
		Controller: controller,
	}
	gameengine.InvokeResolveHook(gs, item)
	return item
}

func TestYawgmothsWill_GrantsZoneCastAndExileReplacement(t *testing.T) {
	gs := newGame(t, 2)
	// Seat 0 has three nonland cards in the graveyard ready to be
	// replayed for free post-Will, plus a land that should NOT get a
	// cast grant (lands are played, not cast).
	lotus := addCardToGraveyard(gs, 0, "Lotus Petal", "artifact")
	wheel := addCardToGraveyard(gs, 0, "Wheel of Fortune", "sorcery")
	led := addCardToGraveyard(gs, 0, "Lion's Eye Diamond", "artifact")
	plains := addCardToGraveyard(gs, 0, "Plains", "land")

	invokeSorceryResolve(gs, 0, "Yawgmoth's Will")

	// Per-Card ZoneCastGrants registered on every nonland card.
	for _, c := range []*gameengine.Card{lotus, wheel, led} {
		grant := gameengine.GetZoneCastGrant(gs, c)
		if grant == nil {
			t.Fatalf("expected ZoneCastGrant for %s after Will resolves", c.Name)
		}
		if grant.Zone != gameengine.ZoneGraveyard {
			t.Errorf("%s: expected Zone=graveyard, got %s", c.Name, grant.Zone)
		}
		if !grant.ExileOnResolve {
			t.Errorf("%s: expected ExileOnResolve=true", c.Name)
		}
		if grant.RequireController != 0 {
			t.Errorf("%s: expected RequireController=0, got %d", c.Name, grant.RequireController)
		}
		if grant.Duration != "until_end_of_turn" {
			t.Errorf("%s: expected Duration=until_end_of_turn, got %s", c.Name, grant.Duration)
		}
	}
	if g := gameengine.GetZoneCastGrant(gs, plains); g != nil {
		t.Errorf("expected no cast grant on the land card, got %+v", g)
	}
	// ZoneCastPolicy registered for late arrivals.
	policyFound := false
	for _, p := range gs.ZoneCastPolicies {
		if p != nil && strings.HasPrefix(p.HandlerID, "play_from_graveyard:") {
			policyFound = true
			if p.OwnerScope != "self" || p.CasterScope != "controller" {
				t.Errorf("policy scope wrong: owner=%s caster=%s", p.OwnerScope, p.CasterScope)
			}
			if !p.ExileOnResolve {
				t.Errorf("policy ExileOnResolve should be true")
			}
		}
	}
	if !policyFound {
		t.Error("expected a play_from_graveyard ZoneCastPolicy to be registered")
	}
	// §614 GY→exile replacement registered for both event types.
	gotDie, gotGY := 0, 0
	for _, re := range gs.Replacements {
		if re == nil {
			continue
		}
		if strings.HasPrefix(re.HandlerID, "play_from_graveyard_die_redirect:") {
			gotDie++
		}
		if strings.HasPrefix(re.HandlerID, "play_from_graveyard_gy_redirect:") {
			gotGY++
		}
	}
	if gotDie != 1 || gotGY != 1 {
		t.Errorf("expected 1 die + 1 gy redirect replacement, got die=%d gy=%d", gotDie, gotGY)
	}
	// Land-play seat flag set.
	if gs.Seats[0].Flags["play_lands_from_graveyard_eot_seat_0"] == 0 {
		t.Errorf("expected play_lands_from_graveyard_eot_seat_0 flag set")
	}
	if hasEvent(gs, "play_from_graveyard_granted") < 1 {
		t.Errorf("expected play_from_graveyard_granted event")
	}
}

func TestYawgmothsWill_GraveyardToExileReplacementRedirectsControllersCards(t *testing.T) {
	gs := newGame(t, 2)
	addCardToGraveyard(gs, 0, "Dark Ritual", "instant") // seed
	invokeSorceryResolve(gs, 0, "Yawgmoth's Will")

	// Simulate "would_be_put_into_graveyard" for a seat-0 card —
	// must redirect to exile.
	ev := gameengine.NewReplEvent("would_be_put_into_graveyard")
	ev.TargetSeat = 0
	ev.Payload["to_zone"] = "graveyard"
	gameengine.FireEvent(gs, ev)
	if got := ev.String("to_zone"); got != "exile" {
		t.Errorf("seat-0 card should redirect to exile, got %s", got)
	}

	// Opponent's card should NOT be redirected.
	ev2 := gameengine.NewReplEvent("would_be_put_into_graveyard")
	ev2.TargetSeat = 1
	ev2.Payload["to_zone"] = "graveyard"
	gameengine.FireEvent(gs, ev2)
	if got := ev2.String("to_zone"); got != "graveyard" {
		t.Errorf("seat-1 card should stay in graveyard, got %s", got)
	}
}

func TestYawgmothsWill_ExpiresAtEndOfTurnCleanup(t *testing.T) {
	gs := newGame(t, 2)
	gs.Turn = 5
	gs.Active = 0
	addCardToGraveyard(gs, 0, "Ponder", "sorcery")
	invokeSorceryResolve(gs, 0, "Yawgmoth's Will")

	// Sanity: replacement is live this turn.
	ev := gameengine.NewReplEvent("would_be_put_into_graveyard")
	ev.TargetSeat = 0
	ev.Payload["to_zone"] = "graveyard"
	gameengine.FireEvent(gs, ev)
	if ev.String("to_zone") != "exile" {
		t.Fatalf("preflight: GY→exile should still fire on the activating turn")
	}

	// Run the EOT sweep (this is what phases.go invokes during cleanup).
	gameengine.ExpirePlayFromGraveyardForTurn(gs)
	gameengine.ExpireZoneCastGrants(gs)
	gameengine.ExpireZoneCastPoliciesByDuration(gs)

	// Replacement must be gone.
	for _, re := range gs.Replacements {
		if re != nil && strings.HasPrefix(re.HandlerID, "play_from_graveyard_") {
			t.Errorf("replacement still present after EOT sweep: %s", re.HandlerID)
		}
	}
	// Seat flag must be cleared.
	if gs.Seats[0].Flags["play_lands_from_graveyard_eot_seat_0"] != 0 {
		t.Errorf("seat flag not cleared after EOT sweep")
	}
	// Re-firing the would_be_put_into_graveyard event must NOT now redirect.
	ev2 := gameengine.NewReplEvent("would_be_put_into_graveyard")
	ev2.TargetSeat = 0
	ev2.Payload["to_zone"] = "graveyard"
	gameengine.FireEvent(gs, ev2)
	if ev2.String("to_zone") != "graveyard" {
		t.Errorf("after sweep, GY redirect should not fire; got %s", ev2.String("to_zone"))
	}
}

func TestGaeasWill_SharesYawgmothsWillBody(t *testing.T) {
	gs := newGame(t, 2)
	ponder := addCardToGraveyard(gs, 0, "Ponder", "sorcery")
	invokeSorceryResolve(gs, 0, "Gaea's Will")

	if g := gameengine.GetZoneCastGrant(gs, ponder); g == nil {
		t.Fatalf("Gaea's Will should grant graveyard cast just like Yawgmoth's Will")
	}
	if gs.Seats[0].Flags["play_lands_from_graveyard_eot_seat_0"] == 0 {
		t.Errorf("Gaea's Will should set the seat-flag for land-play permission")
	}
}

func TestYawgmothsAgenda_ETBPermanentDurationAndLTBCleanup(t *testing.T) {
	gs := newGame(t, 2)
	gs.Turn = 3
	addCardToGraveyard(gs, 0, "Demonic Tutor", "sorcery")
	agenda := addPerm(gs, 0, "Yawgmoth's Agenda", "enchantment")

	gameengine.InvokeETBHook(gs, agenda)

	// While source is on battlefield: seat flag set with "perm" suffix.
	if gs.Seats[0].Flags["play_lands_from_graveyard_perm_seat_0"] == 0 {
		t.Errorf("Agenda should set the perm-variant seat flag")
	}
	// The §614 replacement should be tied to the source permanent.
	foundAnchored := false
	for _, re := range gs.Replacements {
		if re != nil && strings.HasPrefix(re.HandlerID, "play_from_graveyard_gy_redirect:") {
			if re.SourcePerm == agenda {
				foundAnchored = true
			}
		}
	}
	if !foundAnchored {
		t.Errorf("Agenda's GY-redirect replacement should anchor to the source permanent")
	}

	// LTB cleanup: simulate the engine's LTB pathway by calling
	// UnregisterReplacementsForPermanent + the per-card LTB.
	gs.UnregisterReplacementsForPermanent(agenda)
	gs.UnregisterZoneCastPoliciesForPermanent(agenda)
	gameengine.UnregisterPlayFromGraveyardForPermanent(gs, agenda)

	if gs.Seats[0].Flags["play_lands_from_graveyard_perm_seat_0"] != 0 {
		t.Errorf("Agenda LTB should clear the perm-variant seat flag")
	}
	for _, re := range gs.Replacements {
		if re != nil && re.SourcePerm == agenda {
			t.Errorf("LTB should drop replacement anchored to Agenda; left %s", re.HandlerID)
		}
	}
}

func TestMagusOfTheWill_ActivationPaysCostExilesSelfAndGrantsRecursion(t *testing.T) {
	gs := newGame(t, 2)
	magus := addPerm(gs, 0, "Magus of the Will", "creature")
	magus.SummoningSick = false
	gs.Seats[0].ManaPool = 5
	addCardToGraveyard(gs, 0, "Brainstorm", "instant")
	addCardToGraveyard(gs, 0, "Lotus Petal", "artifact")

	gameengine.InvokeActivatedHook(gs, magus, 0, nil)

	// Cost paid: 3 mana spent, Magus tapped, Magus moved to exile.
	if gs.Seats[0].ManaPool != 2 {
		t.Errorf("expected mana pool = 2 (5 - 3), got %d", gs.Seats[0].ManaPool)
	}
	// After ExilePermanent, Magus should be in seat-0 exile.
	foundExile := false
	for _, c := range gs.Seats[0].Exile {
		if c != nil && c == magus.Card {
			foundExile = true
		}
	}
	if !foundExile {
		t.Errorf("Magus should be exiled by its own activation cost; exile=%+v", gs.Seats[0].Exile)
	}
	// Grants are turn-scoped (SourcePerm=nil after self-exile).
	if gs.Seats[0].Flags["play_lands_from_graveyard_eot_seat_0"] == 0 {
		t.Errorf("Magus activation should set the EOT seat flag")
	}
}

func TestMagusOfTheWill_RejectsTappedOrSummoningSick(t *testing.T) {
	gs := newGame(t, 2)
	magus := addPerm(gs, 0, "Magus of the Will", "creature")
	magus.SummoningSick = true
	gs.Seats[0].ManaPool = 5

	gameengine.InvokeActivatedHook(gs, magus, 0, nil)

	if gs.Seats[0].ManaPool != 5 {
		t.Errorf("summoning-sick Magus should not spend mana, pool=%d", gs.Seats[0].ManaPool)
	}
	if gs.Seats[0].Flags["play_lands_from_graveyard_eot_seat_0"] != 0 {
		t.Errorf("summoning-sick Magus should not grant recursion")
	}
}
