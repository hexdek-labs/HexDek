package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// R60 follow-up to #118 (would_fire_etb_trigger wired into the ETB
// pipeline). These tests verify the two flagship ETB-doublers — Yarok,
// the Desecrated and Cloud, Midgar Mercenary — work end-to-end through
// the real per_card ETB hooks (InvokeETBHook → registerXxx →
// gameengine.RegisterXxx → replacement registry) and the real ETB
// firing pipeline (FirePermanentETBTriggers).
//
// Yarok's printed text: "If a permanent entering the battlefield causes
// a triggered ability of a permanent you control to trigger, that
// ability triggers an additional time."
//
// Cloud's printed text: "As long as Cloud is equipped, if a triggered
// ability of Cloud or an Equipment attached to it triggers, that ability
// triggers an additional time."
//
// Both register replacements on would_fire_etb_trigger. Pre-#118 the
// replacements were inert. Post-#118 they should double; this is the
// verification.

// drawTriggerAST builds a card with a single ETB triggered ability that
// draws one card. Used as the source-permanent whose trigger Yarok /
// Cloud should double.
func drawTriggerAST(name string) *gameast.CardAST {
	return &gameast.CardAST{
		Name: name,
		Abilities: []gameast.Ability{
			&gameast.Triggered{
				Trigger: gameast.Trigger{Event: "etb"},
				Effect: &gameast.Draw{
					Count:  *gameast.NumInt(1),
					Target: gameast.Filter{Base: "controller"},
				},
			},
		},
	}
}

// triggerPushCount mirrors the helper from gameengine/panharmonicon_etb_wireup_r60_test.go.
// PushTriggeredAbility increments gs.Flags["_trigger_fires_this_turn"]
// per push — regardless of whether the resulting StackItem subsequently
// resolves and pops off gs.Stack — so it's the cleanest signal that
// the doubler ran.
func triggerPushCount(gs *gameengine.GameState) int {
	if gs == nil || gs.Flags == nil {
		return 0
	}
	return gs.Flags["_trigger_fires_this_turn"]
}

// -----------------------------------------------------------------------------
// Yarok, the Desecrated
// -----------------------------------------------------------------------------

// TestYarok_DoublesCreatureETB pins the iconic Yarok behavior: a
// creature ETBs on Yarok's controller's side, its ETB trigger fires
// TWICE.
func TestYarok_DoublesCreatureETB(t *testing.T) {
	gs := newGame(t, 2)

	// Yarok on the battlefield, ETB hook registers the replacement.
	yarok := addPerm(gs, 0, "Yarok, the Desecrated", "creature")
	gameengine.InvokeETBHook(gs, yarok)

	// Another friendly creature ETBs with a draw trigger.
	mulldrifter := addPerm(gs, 0, "Mulldrifter", "creature")
	mulldrifter.Card.AST = drawTriggerAST("Mulldrifter")

	addLibrary(gs, 0, "L1", "L2", "L3", "L4")
	gameengine.FirePermanentETBTriggers(gs, mulldrifter)

	if n := triggerPushCount(gs); n != 2 {
		t.Fatalf("Yarok should double the ETB trigger: want 2 pushes, got %d", n)
	}
}

// TestYarok_DoublesLandETB extends the test to LAND ETB triggers — the
// semantic difference from Panharmonicon. Yarok's text says "a
// permanent entering", Panharmonicon says "an artifact or creature
// entering". Verifying the wider scope catches it.
func TestYarok_DoublesLandETB(t *testing.T) {
	gs := newGame(t, 2)

	yarok := addPerm(gs, 0, "Yarok, the Desecrated", "creature")
	gameengine.InvokeETBHook(gs, yarok)

	// A "landfall" land with an ETB draw trigger (proxy for any land
	// with an ETB effect — Lotus Vale, Shivan Gorge, etc.).
	land := addPerm(gs, 0, "Triggered Land", "land")
	land.Card.AST = drawTriggerAST("Triggered Land")

	addLibrary(gs, 0, "L1", "L2", "L3", "L4")
	gameengine.FirePermanentETBTriggers(gs, land)

	if n := triggerPushCount(gs); n != 2 {
		t.Fatalf("Yarok should double LAND ETB trigger: want 2 pushes, got %d", n)
	}
}

// TestYarok_DoesNotDoubleOpponent guards the controller filter. Yarok
// only doubles triggers on permanents YOU control.
func TestYarok_DoesNotDoubleOpponent(t *testing.T) {
	gs := newGame(t, 2)

	yarok := addPerm(gs, 0, "Yarok, the Desecrated", "creature")
	gameengine.InvokeETBHook(gs, yarok)

	// Opponent's creature ETBs — Yarok should NOT double.
	oppCreature := addPerm(gs, 1, "Opp Mulldrifter", "creature")
	oppCreature.Card.AST = drawTriggerAST("Opp Mulldrifter")

	addLibrary(gs, 1, "L1", "L2", "L3", "L4")
	gameengine.FirePermanentETBTriggers(gs, oppCreature)

	if n := triggerPushCount(gs); n != 1 {
		t.Fatalf("Yarok must not double opponent's ETB: want 1 push, got %d", n)
	}
}

// TestYarok_StacksWithPanharmonicon — two doublers in play, each apply
// independently per §616.1f iterate-until-no-applicable. Expected push
// count: 3 (base 1 + Panharmonicon +1 + Yarok +1).
func TestYarok_StacksWithPanharmonicon(t *testing.T) {
	gs := newGame(t, 2)

	yarok := addPerm(gs, 0, "Yarok, the Desecrated", "creature")
	gameengine.InvokeETBHook(gs, yarok)

	panh := addPerm(gs, 0, "Panharmonicon", "artifact")
	gameengine.RegisterPanharmonicon(gs, panh)

	mulldrifter := addPerm(gs, 0, "Mulldrifter", "creature")
	mulldrifter.Card.AST = drawTriggerAST("Mulldrifter")

	addLibrary(gs, 0, "L1", "L2", "L3", "L4", "L5", "L6")
	gameengine.FirePermanentETBTriggers(gs, mulldrifter)

	if n := triggerPushCount(gs); n != 3 {
		t.Fatalf("Yarok + Panharmonicon should triple ETB trigger: want 3 pushes, got %d", n)
	}
}

// -----------------------------------------------------------------------------
// Cloud, Midgar Mercenary
// -----------------------------------------------------------------------------

// TestCloudMidgar_RegistersReplacementAndTutors pins the ETB tutor +
// replacement-registration behavior the per_card ETB hook performs.
func TestCloudMidgar_RegistersReplacementAndTutors(t *testing.T) {
	gs := newGame(t, 2)

	// Seed library with two equipment cards + filler so the tutor has
	// targets and we can confirm the highest-CMC one is fetched. cardCMC
	// reads the test-friendly "cmc:N" token from Card.Types (see
	// per_card/helpers.go::cardCMC), not Card.CMC.
	smallEq := &gameengine.Card{Name: "Bone Saw", Owner: 0, Types: []string{"artifact", "equipment", "cmc:0"}}
	bigEq := &gameengine.Card{Name: "Argentum Armor", Owner: 0, Types: []string{"artifact", "equipment", "cmc:6"}}
	gs.Seats[0].Library = append(gs.Seats[0].Library, smallEq, bigEq, &gameengine.Card{Name: "Filler", Owner: 0, Types: []string{"creature"}})

	cloud := addPerm(gs, 0, "Cloud, Midgar Mercenary", "creature")
	gameengine.InvokeETBHook(gs, cloud)

	// The tutor moves the highest-CMC equipment (Argentum Armor) to hand.
	found := false
	for _, c := range gs.Seats[0].Hand {
		if c != nil && c.Name == "Argentum Armor" {
			found = true
		}
	}
	if !found {
		t.Errorf("Argentum Armor (highest CMC) should have been tutored to hand; hand=%v", handNames(gs.Seats[0].Hand))
	}

	// The cloud_trigger_doubler_when_equipped flag is set.
	if cloud.Flags["cloud_trigger_doubler_when_equipped"] != 1 {
		t.Errorf("cloud_trigger_doubler_when_equipped flag not set")
	}
}

// TestCloudMidgar_UnequippedDoesNotDouble verifies the cloudIsEquipped
// gate: without an Equipment attached, Cloud's own ETB-source triggers
// do NOT double.
func TestCloudMidgar_UnequippedDoesNotDouble(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Library = append(gs.Seats[0].Library, &gameengine.Card{Name: "Filler Equipment", Owner: 0, Types: []string{"artifact", "equipment"}, CMC: 1})

	cloud := addPerm(gs, 0, "Cloud, Midgar Mercenary", "creature")
	gameengine.InvokeETBHook(gs, cloud)

	// Attach a draw-ETB trigger AST to Cloud retroactively (simulating
	// Cloud as the trigger-source permanent). Without equipment
	// attached, the doubler should NOT apply.
	cloud.Card.AST = drawTriggerAST("Cloud, Midgar Mercenary")

	// Reset the trigger counter so we measure only this fire.
	gs.Flags["_trigger_fires_this_turn"] = 0

	addLibrary(gs, 0, "L1", "L2", "L3", "L4")
	gameengine.FirePermanentETBTriggers(gs, cloud)

	if n := triggerPushCount(gs); n != 1 {
		t.Fatalf("Cloud unequipped: must not double its own trigger (want 1 push), got %d", n)
	}
}

// TestCloudMidgar_EquippedDoublesOwnTrigger verifies the doubler ACTIVATES
// when an Equipment is attached to Cloud.
func TestCloudMidgar_EquippedDoublesOwnTrigger(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Library = append(gs.Seats[0].Library, &gameengine.Card{Name: "Filler Equipment", Owner: 0, Types: []string{"artifact", "equipment"}, CMC: 1})

	cloud := addPerm(gs, 0, "Cloud, Midgar Mercenary", "creature")
	gameengine.InvokeETBHook(gs, cloud)

	// Equip Cloud — attach a stand-in equipment permanent.
	equip := addPerm(gs, 0, "Argentum Armor", "artifact", "equipment")
	equip.AttachedTo = cloud

	// Now give Cloud an ETB trigger and fire it.
	cloud.Card.AST = drawTriggerAST("Cloud, Midgar Mercenary")
	gs.Flags["_trigger_fires_this_turn"] = 0
	addLibrary(gs, 0, "L1", "L2", "L3", "L4")
	gameengine.FirePermanentETBTriggers(gs, cloud)

	if n := triggerPushCount(gs); n != 2 {
		t.Fatalf("Cloud equipped: must double its own trigger (want 2 pushes), got %d", n)
	}
}

// TestCloudMidgar_EquippedDoublesAttachedEquipmentTrigger covers the
// second half of Cloud's text: triggered abilities of EQUIPMENT
// attached to Cloud also double.
func TestCloudMidgar_EquippedDoublesAttachedEquipmentTrigger(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Library = append(gs.Seats[0].Library, &gameengine.Card{Name: "Filler Equipment", Owner: 0, Types: []string{"artifact", "equipment"}, CMC: 1})

	cloud := addPerm(gs, 0, "Cloud, Midgar Mercenary", "creature")
	gameengine.InvokeETBHook(gs, cloud)

	// Attach an equipment that itself has an ETB trigger.
	equip := addPerm(gs, 0, "Cranial Plating", "artifact", "equipment")
	equip.AttachedTo = cloud
	equip.Card.AST = drawTriggerAST("Cranial Plating")

	gs.Flags["_trigger_fires_this_turn"] = 0
	addLibrary(gs, 0, "L1", "L2", "L3", "L4")
	gameengine.FirePermanentETBTriggers(gs, equip)

	if n := triggerPushCount(gs); n != 2 {
		t.Fatalf("Cloud equipped: attached equipment trigger must double (want 2 pushes), got %d", n)
	}
}

// TestCloudMidgar_EquippedDoesNotDoubleUnrelated guards the source
// filter: even with Cloud equipped, an UNRELATED creature's ETB trigger
// does not double (Cloud's text is scoped to Cloud + attached equipment).
func TestCloudMidgar_EquippedDoesNotDoubleUnrelated(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Library = append(gs.Seats[0].Library, &gameengine.Card{Name: "Filler Equipment", Owner: 0, Types: []string{"artifact", "equipment"}, CMC: 1})

	cloud := addPerm(gs, 0, "Cloud, Midgar Mercenary", "creature")
	gameengine.InvokeETBHook(gs, cloud)

	equip := addPerm(gs, 0, "Argentum Armor", "artifact", "equipment")
	equip.AttachedTo = cloud

	// An unrelated creature ETBs — not Cloud, not attached to Cloud.
	other := addPerm(gs, 0, "Mulldrifter", "creature")
	other.Card.AST = drawTriggerAST("Mulldrifter")
	gs.Flags["_trigger_fires_this_turn"] = 0
	addLibrary(gs, 0, "L1", "L2", "L3", "L4")
	gameengine.FirePermanentETBTriggers(gs, other)

	if n := triggerPushCount(gs); n != 1 {
		t.Fatalf("Cloud doubler must not affect unrelated permanents (want 1 push), got %d", n)
	}
}
