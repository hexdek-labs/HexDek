package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// -----------------------------------------------------------------------------
// Batch AK (R60) — tests for Insurrection / Word of Command / Mind's Dilation
// -----------------------------------------------------------------------------

// -----------------------------------------------------------------------------
// Registry smoke — all 3 new handlers wired and survive Reset.
// -----------------------------------------------------------------------------

func TestBatchAK_AllRegistered(t *testing.T) {
	if !HasResolve("Insurrection") {
		t.Error("Insurrection should have a Resolve handler")
	}
	if !HasResolve("Word of Command") {
		t.Error("Word of Command should have a Resolve handler")
	}
	// Mind's Dilation is trigger-only, can't check via HasResolve.
	// Indirect probe — fire the trigger and confirm at least one event.
	gs := newGame(t, 2)
	addPerm(gs, 0, "Mind's Dilation", "enchantment")
	addLibrary(gs, 1, "A")
	before := len(gs.EventLog)
	gameengine.FireCardTrigger(gs, "spell_cast_by_opponent", map[string]interface{}{
		"caster_seat": 1,
		"spell_name":  "X",
	})
	if len(gs.EventLog) <= before {
		t.Error("Mind's Dilation spell_cast_by_opponent should fire some event")
	}
}

// -----------------------------------------------------------------------------
// Insurrection — mass untap + steal-control of every creature on the table.
// -----------------------------------------------------------------------------

func TestInsurrection_UntapsAllCreatures(t *testing.T) {
	gs := newGame(t, 3)
	c0 := addPerm(gs, 0, "Caster Creature", "creature")
	c0.Tapped = true
	c1 := addPerm(gs, 1, "Opp Creature 1", "creature")
	c1.Tapped = true
	c2 := addPerm(gs, 2, "Opp Creature 2", "creature")
	c2.Tapped = true
	// Non-creature on the same battlefield should NOT be untapped or
	// stolen — the oracle is gated to creatures only.
	c1Land := addPerm(gs, 1, "Tapped Mountain", "land")
	c1Land.Tapped = true

	card := addCard(gs, 0, "Insurrection", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if c0.Tapped {
		t.Error("caster's own creature should be untapped")
	}
	if c1.Tapped {
		t.Error("opp creature 1 should be untapped")
	}
	if c2.Tapped {
		t.Error("opp creature 2 should be untapped")
	}
	if !c1Land.Tapped {
		t.Error("land (noncreature) should NOT be untapped by Insurrection")
	}
}

func TestInsurrection_StealsOpponentCreaturesToCasterBattlefield(t *testing.T) {
	gs := newGame(t, 3)
	mine := addPerm(gs, 0, "Mine", "creature")
	theirs1 := addPerm(gs, 1, "Theirs 1", "creature")
	theirs2 := addPerm(gs, 2, "Theirs 2", "creature")

	card := addCard(gs, 0, "Insurrection", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	// All 3 creatures should now be controlled by seat 0.
	if mine.Controller != 0 {
		t.Errorf("caster's own creature should still be controlled by 0; got %d", mine.Controller)
	}
	if theirs1.Controller != 0 {
		t.Errorf("opp1's creature should now be controlled by 0; got %d", theirs1.Controller)
	}
	if theirs2.Controller != 0 {
		t.Errorf("opp2's creature should now be controlled by 0; got %d", theirs2.Controller)
	}
	// Original seats' battlefields should NOT contain the stolen perms.
	for _, p := range gs.Seats[1].Battlefield {
		if p == theirs1 {
			t.Error("Theirs 1 should have left seat 1's battlefield")
		}
	}
	for _, p := range gs.Seats[2].Battlefield {
		if p == theirs2 {
			t.Error("Theirs 2 should have left seat 2's battlefield")
		}
	}
	// Caster's battlefield should now contain all 3.
	foundMine, found1, found2 := false, false, false
	for _, p := range gs.Seats[0].Battlefield {
		if p == mine {
			foundMine = true
		}
		if p == theirs1 {
			found1 = true
		}
		if p == theirs2 {
			found2 = true
		}
	}
	if !foundMine || !found1 || !found2 {
		t.Errorf("caster's battlefield missing creatures: mine=%v t1=%v t2=%v", foundMine, found1, found2)
	}
}

func TestInsurrection_GrantsHasteToAllCreatures(t *testing.T) {
	gs := newGame(t, 2)
	mine := addPerm(gs, 0, "Mine", "creature")
	mine.SummoningSick = true
	theirs := addPerm(gs, 1, "Theirs", "creature")
	theirs.SummoningSick = true

	card := addCard(gs, 0, "Insurrection", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if mine.SummoningSick {
		t.Error("Insurrection should clear summoning sickness on own creature (haste grant)")
	}
	if theirs.SummoningSick {
		t.Error("Insurrection should clear summoning sickness on stolen creature (haste grant)")
	}
	if mine.Flags["kw:haste"] != 1 {
		t.Errorf("expected kw:haste=1 on own creature; got %d", mine.Flags["kw:haste"])
	}
	if theirs.Flags["kw:haste"] != 1 {
		t.Errorf("expected kw:haste=1 on stolen creature; got %d", theirs.Flags["kw:haste"])
	}
}

func TestInsurrection_RegistersEOTReversalDelayedTrigger(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 1, "Theirs", "creature")
	before := len(gs.DelayedTriggers)

	card := addCard(gs, 0, "Insurrection", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if len(gs.DelayedTriggers) != before+1 {
		t.Errorf("expected one new DelayedTrigger registered; before=%d after=%d",
			before, len(gs.DelayedTriggers))
	}
}

func TestInsurrection_NoCreaturesNoOp(t *testing.T) {
	gs := newGame(t, 2)
	// Only a land + an artifact on the table — no creatures anywhere.
	addPerm(gs, 0, "Sol Ring", "artifact")
	addPerm(gs, 1, "Island", "land")
	before := len(gs.DelayedTriggers)

	card := addCard(gs, 0, "Insurrection", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if len(gs.DelayedTriggers) != before {
		t.Error("no creatures on table → no EOT-reversal trigger needed")
	}
	if hasEvent(gs, "per_card_handler") < 1 {
		t.Error("expected breadcrumb event even on no-op resolve")
	}
}

// -----------------------------------------------------------------------------
// Word of Command — hand reveal + force-play (with partial breadcrumb).
// -----------------------------------------------------------------------------

func TestWordOfCommand_PicksHighestCMCNonland(t *testing.T) {
	gs := newGame(t, 2)
	// Opp hand: Island (land, skipped), Counterspell (CMC 2), Force of
	// Will (CMC 5), Lightning Bolt (CMC 1). Best pick = Force of Will.
	land := &gameengine.Card{Name: "Island", Owner: 1, Types: []string{"land"}}
	cs := &gameengine.Card{Name: "Counterspell", Owner: 1, Types: []string{"instant", "cmc:2"}}
	fow := &gameengine.Card{Name: "Force of Will", Owner: 1, Types: []string{"instant", "cmc:5"}}
	bolt := &gameengine.Card{Name: "Lightning Bolt", Owner: 1, Types: []string{"instant", "cmc:1"}}
	gs.Seats[1].Hand = []*gameengine.Card{land, cs, fow, bolt}

	card := addCard(gs, 0, "Word of Command", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	// Find the per_card_handler event and confirm forced_card == "Force of Will".
	found := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "per_card_handler" && ev.Source == "Word of Command" {
			if fc, ok := ev.Details["forced_card"].(string); ok && fc == "Force of Will" {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("expected forced_card=Force of Will (highest-CMC nonland); events:\n%+v", gs.EventLog)
	}
}

func TestWordOfCommand_RevealsFullHandIntoEventDetails(t *testing.T) {
	gs := newGame(t, 2)
	a := &gameengine.Card{Name: "Card A", Owner: 1, Types: []string{"instant"}}
	b := &gameengine.Card{Name: "Card B", Owner: 1, Types: []string{"instant"}}
	gs.Seats[1].Hand = []*gameengine.Card{a, b}

	card := addCard(gs, 0, "Word of Command", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	for _, ev := range gs.EventLog {
		if ev.Kind == "per_card_handler" && ev.Source == "Word of Command" {
			if names, ok := ev.Details["revealed_hand"].([]string); ok {
				if len(names) != 2 {
					t.Errorf("expected 2 revealed cards; got %d (%v)", len(names), names)
				}
			} else {
				t.Errorf("expected revealed_hand []string in event details")
			}
		}
	}
}

func TestWordOfCommand_EmitsPartialForUnmodeledForcePlay(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[1].Hand = []*gameengine.Card{
		{Name: "Counterspell", Owner: 1, Types: []string{"instant"}},
	}

	card := addCard(gs, 0, "Word of Command", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if hasEvent(gs, "per_card_partial") < 1 {
		t.Error("Word of Command must emit per_card_partial — the force-play core is unmodeled")
	}
}

func TestWordOfCommand_NoLivingOpponentFails(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[1].Lost = true

	card := addCard(gs, 0, "Word of Command", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if hasEvent(gs, "per_card_failed") < 1 {
		t.Error("expected per_card_failed when no living opponents remain")
	}
}

func TestWordOfCommand_EmptyOppHandFails(t *testing.T) {
	gs := newGame(t, 2)
	// Opp alive but holding nothing.

	card := addCard(gs, 0, "Word of Command", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if hasEvent(gs, "per_card_failed") < 1 {
		t.Error("expected per_card_failed when opp hand is empty")
	}
}

// -----------------------------------------------------------------------------
// Mind's Dilation — opp's first spell each turn exiles top of their library;
// nonland → free cast for the Dilation controller.
// -----------------------------------------------------------------------------

func TestMindsDilation_ExilesTopOnFirstOppSpell(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Mind's Dilation", "enchantment")
	topCard := &gameengine.Card{
		Name:  "Brainstorm",
		Owner: 1,
		Types: []string{"instant", "cmc:1"},
	}
	bottomCard := &gameengine.Card{
		Name:  "Island",
		Owner: 1,
		Types: []string{"land"},
	}
	gs.Seats[1].Library = []*gameengine.Card{topCard, bottomCard}

	gameengine.FireCardTrigger(gs, "spell_cast_by_opponent", map[string]interface{}{
		"caster_seat": 1,
		"spell_name":  "Counterspell",
	})

	// Top should be exiled. The §400.7c routing means Brainstorm lands
	// in seat 1's exile (it's owned by seat 1) — that's the post-#685
	// owner-scoped MoveCard primitive at work.
	found := false
	for _, c := range gs.Seats[1].Exile {
		if c == topCard {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("top card should be in seat 1's exile (owner-scoped); got %d cards in seat 1 exile, %d in seat 0 exile",
			len(gs.Seats[1].Exile), len(gs.Seats[0].Exile))
	}
	if len(gs.Seats[1].Library) != 1 {
		t.Errorf("opp library should have 1 card left; got %d", len(gs.Seats[1].Library))
	}
}

func TestMindsDilation_FiresOnlyOnFirstOppSpellPerTurn(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Mind's Dilation", "enchantment")
	gs.Seats[1].Library = []*gameengine.Card{
		{Name: "A", Owner: 1, Types: []string{"instant"}},
		{Name: "B", Owner: 1, Types: []string{"instant"}},
		{Name: "C", Owner: 1, Types: []string{"instant"}},
	}

	// Opp casts their 1st spell — should fire.
	gameengine.FireCardTrigger(gs, "spell_cast_by_opponent", map[string]interface{}{
		"caster_seat": 1,
		"spell_name":  "X1",
	})
	// Opp casts their 2nd spell — should NOT fire (gated by first-this-turn).
	gameengine.FireCardTrigger(gs, "spell_cast_by_opponent", map[string]interface{}{
		"caster_seat": 1,
		"spell_name":  "X2",
	})

	if len(gs.Seats[1].Library) != 2 {
		t.Errorf("only one exile expected (first-spell gate); library has %d (expected 2)",
			len(gs.Seats[1].Library))
	}
	if len(gs.Seats[1].Exile) != 1 {
		t.Errorf("exactly one card should be in exile; got %d", len(gs.Seats[1].Exile))
	}
}

func TestMindsDilation_RegistersZoneCastGrantForNonland(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Mind's Dilation", "enchantment")
	nonland := &gameengine.Card{Name: "Brainstorm", Owner: 1, Types: []string{"instant"}}
	gs.Seats[1].Library = []*gameengine.Card{nonland}

	gameengine.FireCardTrigger(gs, "spell_cast_by_opponent", map[string]interface{}{
		"caster_seat": 1,
		"spell_name":  "X",
	})

	if _, ok := gs.ZoneCastGrants[nonland]; !ok {
		t.Errorf("expected ZoneCastGrant for the exiled nonland; got nil")
	}
	// Confirm the free_cast=true breadcrumb.
	saw := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "per_card_handler" && ev.Source == "Mind's Dilation" {
			if fc, ok := ev.Details["free_cast"].(bool); ok && fc {
				saw = true
			}
		}
	}
	if !saw {
		t.Error("expected free_cast=true in Mind's Dilation event details for nonland")
	}
}

func TestMindsDilation_DoesNotGrantCastForLand(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Mind's Dilation", "enchantment")
	land := &gameengine.Card{Name: "Island", Owner: 1, Types: []string{"land"}}
	gs.Seats[1].Library = []*gameengine.Card{land}

	gameengine.FireCardTrigger(gs, "spell_cast_by_opponent", map[string]interface{}{
		"caster_seat": 1,
		"spell_name":  "X",
	})

	if _, ok := gs.ZoneCastGrants[land]; ok {
		t.Error("lands should NOT get a free-cast grant; oracle gates on nonland")
	}
}

func TestMindsDilation_DoesNotFireOnOwnControllerCast(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Mind's Dilation", "enchantment")
	gs.Seats[0].Library = []*gameengine.Card{
		{Name: "Should Not Move", Owner: 0, Types: []string{"instant"}},
	}

	// Controller (seat 0) casts a spell — Dilation's trigger gates on
	// opponent, so this is a no-op.
	gameengine.FireCardTrigger(gs, "spell_cast_by_opponent", map[string]interface{}{
		"caster_seat": 0,
		"spell_name":  "X",
	})

	if len(gs.Seats[0].Library) != 1 {
		t.Error("controller's own cast should NOT trigger Mind's Dilation")
	}
}

func TestMindsDilation_EmptyOppLibraryFailsGracefully(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Mind's Dilation", "enchantment")
	// Opp library empty.

	gameengine.FireCardTrigger(gs, "spell_cast_by_opponent", map[string]interface{}{
		"caster_seat": 1,
		"spell_name":  "X",
	})

	if hasEvent(gs, "per_card_failed") < 1 {
		t.Error("expected per_card_failed when opp library is empty")
	}
}
