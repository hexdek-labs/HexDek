package per_card

import (
	"strconv"
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// R60 follow-up to the lose_game pipeline canonicalization (PR #770
// batch_aj + sibling sweep): the 8 per_card handlers that route their
// effect-driven losses through gameengine.MarkSeatLostByEffect were
// emitting their own ad-hoc audit events on top of the canonical
// `lose_game` event that the helper already fires. Specifically:
//
//   - atemsis_all_seeing.go duplicated as Kind="player_loses"
//   - demonic_pact.go    duplicated as Kind="seat_lost"
//
// Both duplicates carried the same Seat + Source + reason info as
// the helper's canonical Event, just under non-canonical Kind names.
// (The sba.go `seat_lost` emission is a different semantic — CR
// §104.4b mandatory-loop draw — and is correctly left alone.)
//
// This file pins the post-normalization shape on each path:
//
//  1. Exactly ONE `lose_game` Event fires (from the helper).
//  2. NO `seat_lost` / `player_loses` Event fires from per_card.
//  3. The canonical Event carries Source = card display name (plus
//     mode/mechanism suffix for paths where the previous duplicate
//     event carried extra reason detail — Atemsis "— six distinct
//     mana values…", Demonic Pact "(final mode — …)").
//  4. The seat is marked Lost and LossReason is stamped.

// findSingleEvent returns the single Event with the given Kind, or
// fails the test if zero/multiple matches are found. Centralizes the
// "exactly one canonical emit" check across both subtests.
func findSingleEvent(t *testing.T, gs *gameengine.GameState, kind string) gameengine.Event {
	t.Helper()
	var match []gameengine.Event
	for _, ev := range gs.EventLog {
		if ev.Kind == kind {
			match = append(match, ev)
		}
	}
	if len(match) != 1 {
		t.Fatalf("expected exactly 1 %q event, got %d (events: %+v)", kind, len(match), gs.EventLog)
	}
	return match[0]
}

// TestDemonicPact_LoseGameEventCanonical pins the demonic_pact.go
// normalization: the helper's canonical `lose_game` Event fires
// exactly once, NO `seat_lost` duplicate is emitted by the handler,
// and the source-name suffix carries the "final mode" detail that
// the previous duplicate event's Details["reason"] used to hold.
func TestDemonicPact_LoseGameEventCanonical(t *testing.T) {
	gs := newGame(t, 2)
	pact := addPerm(gs, 0, "Demonic Pact", "enchantment")
	pact.Flags = map[string]int{
		"pact_mode_damage_chosen":  1,
		"pact_mode_discard_chosen": 1,
		"pact_mode_draw_chosen":    1,
	}

	gameengine.FireCardTrigger(gs, "upkeep", map[string]interface{}{
		"active_seat": 0,
	})

	if !gs.Seats[0].Lost {
		t.Fatalf("Demonic Pact's 4th mode should mark controller Lost")
	}
	if gs.Seats[0].LossReason == "" {
		t.Errorf("expected LossReason stamped on seat 0, got empty")
	}

	// Canonical event fires exactly once.
	ev := findSingleEvent(t, gs, "lose_game")
	if ev.Seat != 0 {
		t.Errorf("lose_game.Seat = %d, want 0", ev.Seat)
	}
	if !strings.Contains(ev.Source, "Demonic Pact") {
		t.Errorf("lose_game.Source = %q, want substring \"Demonic Pact\"", ev.Source)
	}
	if !strings.Contains(ev.Source, "final mode") {
		t.Errorf("lose_game.Source = %q, want substring \"final mode\" (normalization preserved mode detail in source-name suffix)", ev.Source)
	}

	// Non-canonical duplicate is gone.
	if c := hasEvent(gs, "seat_lost"); c != 0 {
		t.Errorf("expected 0 `seat_lost` events from Demonic Pact (normalized to `lose_game`), got %d", c)
	}
}

// TestAtemsis_LoseGameEventCanonical pins the atemsis_all_seeing.go
// normalization: 6+ distinct mana values → helper fires canonical
// `lose_game`, NO `player_loses` duplicate, source name carries the
// "six distinct mana values" detail.
//
// Quirk: the current atemsis_all_seeing.go handler reads
// `gs.Seats[perm.Controller].Hand` (the controller's hand) rather
// than the defender's. Per Scryfall oracle this should be the
// DEFENDER's hand. That's a separate latent bug outside this
// normalization PR's scope — recorded under "Open" in the issue log
// follow-up. To exercise the loss path while the bug persists, we
// stack the cards in the controller's hand (seat 0); the event
// shape we're pinning here is independent of which seat's hand the
// check reads.
func TestAtemsis_LoseGameEventCanonical(t *testing.T) {
	gs := newGame(t, 2)
	atemsis := addPerm(gs, 0, "Atemsis, All-Seeing", "creature")

	// Build seat-0 hand with 6 distinct mana values. manaCostOf reads
	// from card.Types looking for a "cost:N" prefix token, so we
	// encode each card's CMC there.
	for i, name := range []string{"a", "b", "c", "d", "e", "f"} {
		c := &gameengine.Card{
			Name:  name,
			Owner: 0,
			Types: []string{"creature", "cost:" + strconv.Itoa(i+1)},
		}
		gs.Seats[0].Hand = append(gs.Seats[0].Hand, c)
	}

	gameengine.FireCardTrigger(gs, "combat_damage_player", map[string]interface{}{
		"source_perm": atemsis,
		"target_seat": 1,
	})

	if !gs.Seats[1].Lost {
		t.Fatalf("Atemsis 6+ distinct mana values → opp should be Lost; events: %+v", gs.EventLog)
	}
	if gs.Seats[1].LossReason == "" {
		t.Errorf("expected LossReason stamped on seat 1, got empty")
	}

	// Canonical event fires exactly once.
	ev := findSingleEvent(t, gs, "lose_game")
	if ev.Seat != 1 {
		t.Errorf("lose_game.Seat = %d, want 1", ev.Seat)
	}
	if !strings.Contains(ev.Source, "Atemsis") {
		t.Errorf("lose_game.Source = %q, want substring \"Atemsis\"", ev.Source)
	}
	if !strings.Contains(ev.Source, "six distinct mana values") {
		t.Errorf("lose_game.Source = %q, want substring \"six distinct mana values\" (normalization preserved mechanism detail in source-name suffix)", ev.Source)
	}

	// Non-canonical duplicate is gone.
	if c := hasEvent(gs, "player_loses"); c != 0 {
		t.Errorf("expected 0 `player_loses` events from Atemsis (normalized to `lose_game`), got %d", c)
	}
}

// TestAtemsis_BelowThresholdNoLoseGameEvent pins the negative: 5
// distinct mana values is not enough, so NO `lose_game` and NO
// duplicate events fire. Defends against an over-broad normalization
// that would always emit a `lose_game` Event regardless of threshold.
func TestAtemsis_BelowThresholdNoLoseGameEvent(t *testing.T) {
	gs := newGame(t, 2)
	atemsis := addPerm(gs, 0, "Atemsis, All-Seeing", "creature")
	// Same controller-hand quirk as the happy path above — see that
	// test's docstring. Stack in seat 0.
	for i, name := range []string{"a", "b", "c", "d", "e"} {
		c := &gameengine.Card{
			Name:  name,
			Owner: 0,
			Types: []string{"creature", "cost:" + strconv.Itoa(i+1)},
		}
		gs.Seats[0].Hand = append(gs.Seats[0].Hand, c)
	}

	gameengine.FireCardTrigger(gs, "combat_damage_player", map[string]interface{}{
		"source_perm": atemsis,
		"target_seat": 1,
	})

	if gs.Seats[1].Lost {
		t.Errorf("Atemsis 5 distinct mana values → opp should NOT be Lost")
	}
	if c := hasEvent(gs, "lose_game"); c != 0 {
		t.Errorf("expected 0 `lose_game` events below threshold, got %d", c)
	}
	if c := hasEvent(gs, "player_loses"); c != 0 {
		t.Errorf("expected 0 `player_loses` events below threshold, got %d", c)
	}
}

// TestAtemsis_PlatinumAngelCancelsLoss is the load-bearing pin on
// the order-flip embedded in this PR. Pre-fix the Atemsis handler
// called emitWin BEFORE MarkSeatLostByEffect — emitWin direct-set
// s.Lost=true on every non-winner seat, so by the time the helper
// ran the seat was already Lost and the helper short-circuited at
// its `if s.Lost { return false }` guard. That bypassed the §614
// would_lose_game replacement chain entirely — Platinum Angel was
// inert on Atemsis. Post-fix the helper runs FIRST so §614 fires;
// emitWin only runs if the loss applied. Mirrors PR #770's
// Triskaidekaphobia cancel test pattern.
func TestAtemsis_PlatinumAngelCancelsLoss(t *testing.T) {
	gs := newGame(t, 2)
	atemsis := addPerm(gs, 0, "Atemsis, All-Seeing", "creature")

	// 6 distinct mana values in controller's hand (matches the
	// happy-path test setup — see the Atemsis-controller-hand quirk).
	for i, name := range []string{"a", "b", "c", "d", "e", "f"} {
		c := &gameengine.Card{
			Name:  name,
			Owner: 0,
			Types: []string{"creature", "cost:" + strconv.Itoa(i+1)},
		}
		gs.Seats[0].Hand = append(gs.Seats[0].Hand, c)
	}

	// Platinum Angel on the defender side + its §614 cancel handler.
	pa := addPerm(gs, 1, "Platinum Angel", "creature")
	gameengine.RegisterPlatinumAngel(gs, pa)

	gameengine.FireCardTrigger(gs, "combat_damage_player", map[string]interface{}{
		"source_perm": atemsis,
		"target_seat": 1,
	})

	if gs.Seats[1].Lost {
		t.Error("seat 1 must NOT be Lost — Platinum Angel cancels the Atemsis loss via §614")
	}
	if gs.Seats[1].LossReason != "" {
		t.Errorf("LossReason must stay empty when §614 cancels, got %q", gs.Seats[1].LossReason)
	}
	// Helper emits a `lose_game_replaced` Event when §614 cancels.
	if c := hasEvent(gs, "lose_game_replaced"); c != 1 {
		t.Errorf("expected 1 `lose_game_replaced` event (§614 cancel), got %d", c)
	}
	if c := hasEvent(gs, "lose_game"); c != 0 {
		t.Errorf("expected 0 `lose_game` events when §614 cancels, got %d", c)
	}
	// emitWin must NOT fire when the loss was cancelled — otherwise the
	// controller would win despite the defender surviving.
	if c := hasEvent(gs, "per_card_win"); c != 0 {
		t.Errorf("expected 0 `per_card_win` events when §614 cancels Atemsis (controller does not win if defender doesn't lose), got %d", c)
	}
	if gs.Seats[0].Won {
		t.Error("seat 0 must NOT be Won — defender survived via Platinum Angel, controller does not win")
	}
}

// TestDemonicPact_NonFinalModeNoLoseGameEvent pins the negative for
// Demonic Pact: when ANY of the first 3 modes is still available,
// the upkeep trigger should NOT route through MarkSeatLostByEffect
// and therefore emit zero `lose_game` events.
func TestDemonicPact_NonFinalModeNoLoseGameEvent(t *testing.T) {
	gs := newGame(t, 2)
	pact := addPerm(gs, 0, "Demonic Pact", "enchantment")
	// Only the damage mode has been chosen — discard/draw/lose still
	// available. The handler should pick the next-available mode
	// (discard or draw), not "lose".
	pact.Flags = map[string]int{
		"pact_mode_damage_chosen": 1,
	}

	gameengine.FireCardTrigger(gs, "upkeep", map[string]interface{}{
		"active_seat": 0,
	})

	if gs.Seats[0].Lost {
		t.Errorf("Demonic Pact non-final mode should NOT mark controller Lost")
	}
	if c := hasEvent(gs, "lose_game"); c != 0 {
		t.Errorf("expected 0 `lose_game` events when non-final mode chosen, got %d", c)
	}
	if c := hasEvent(gs, "seat_lost"); c != 0 {
		t.Errorf("expected 0 `seat_lost` events when non-final mode chosen, got %d", c)
	}
}
