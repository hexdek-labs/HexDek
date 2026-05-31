package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// TestAnafenzaTheForemost_OppCreatureExileReplacementRegistered pins
// the §614 "if a nontoken creature an opponent controls would die,
// exile it instead" replacement: it must be registered against
// gs.Replacements when Anafenza enters play via the canonical
// RegisterReplacementsForPermanent dispatch. The per_card handler
// itself dropped its stale "not implemented" emitPartial in batch 3
// because the framework wiring already exists at replacement.go:857.
func TestAnafenzaTheForemost_OppCreatureExileReplacementRegistered(t *testing.T) {
	gs := newGame(t, 2)
	anafenza := addPerm(gs, 0, "Anafenza, the Foremost", "legendary", "creature", "human", "soldier")
	gameengine.RegisterReplacementsForPermanent(gs, anafenza)

	// Scan replacement slots for the Anafenza handler.
	found := false
	for _, r := range gs.Replacements {
		if r == nil {
			continue
		}
		if r.EventType == "would_die" && r.SourcePerm == anafenza {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected would_die replacement registered for Anafenza; gs.Replacements=%+v", gs.Replacements)
	}
}

// TestCuriousHomunculus_TransformsWhenGraveyardHasThreeInstantSorcery
// pins the upkeep transform: ≥3 instant/sorcery cards in graveyard →
// TransformPermanent fires.
func TestCuriousHomunculus_TransformsWhenGraveyardHasThreeInstantSorcery(t *testing.T) {
	gs := newGame(t, 2)
	homunc := addPerm(gs, 0, "Curious Homunculus", "creature", "homunculus")
	// Put 3 instants in graveyard.
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard,
		addCard(gs, 0, "Bolt 1", "instant"),
		addCard(gs, 0, "Bolt 2", "instant"),
		addCard(gs, 0, "Wrath 1", "sorcery"),
	)

	gameengine.FireCardTrigger(gs, "upkeep_controller", map[string]interface{}{
		"active_seat": 0,
	})

	// TransformPermanent fires; without a DFC back face wired the engine
	// emits transform_noop (cause:not_dfc) — either way confirms the
	// primitive was invoked, which is the behavior the batch 3 fix added
	// (the prior stub only emitted a partial flag and never called the
	// transform machinery at all).
	if hasEvent(gs, "transform")+hasEvent(gs, "transform_noop") < 1 {
		t.Errorf("expected TransformPermanent to be invoked; events=%+v", gs.EventLog)
	}
	_ = homunc
}

// TestCuriousHomunculus_DoesNotTransformBelowThreshold pins the gate:
// fewer than 3 I/S cards → no transform.
func TestCuriousHomunculus_DoesNotTransformBelowThreshold(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Curious Homunculus", "creature", "homunculus")
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard,
		addCard(gs, 0, "Bolt 1", "instant"),
		addCard(gs, 0, "Bolt 2", "instant"),
	)

	gameengine.FireCardTrigger(gs, "upkeep_controller", map[string]interface{}{
		"active_seat": 0,
	})

	if c := hasEvent(gs, "transform"); c != 0 {
		t.Errorf("expected no transform event below threshold; got %d", c)
	}
}

// TestBreOfClanStoutarm_ActivatedGrantsFlyingLifelink pins the
// {1}{W}, {T} activated ability: tap, target another creature we
// control, stamp kw:flying + kw:lifelink until EOT.
func TestBreOfClanStoutarm_ActivatedGrantsFlyingLifelink(t *testing.T) {
	gs := newGame(t, 2)
	bre := addPerm(gs, 0, "Bre of Clan Stoutarm", "legendary", "creature", "human")
	target := addPerm(gs, 0, "Vanilla Bear", "creature", "bear")

	gameengine.InvokeActivatedHook(gs, bre, 0, map[string]interface{}{
		"target_perm": target,
	})

	if !bre.Tapped {
		t.Errorf("Bre should be tapped after activation")
	}
	if target.Flags["kw:flying"] != 1 {
		t.Errorf("target should have kw:flying; flags=%+v", target.Flags)
	}
	if target.Flags["kw:lifelink"] != 1 {
		t.Errorf("target should have kw:lifelink; flags=%+v", target.Flags)
	}
}

// TestBreOfClanStoutarm_GrantStripsAtEndStep pins the EOT cleanup:
// next_end_step delayed trigger fires → granted flags strip.
func TestBreOfClanStoutarm_GrantStripsAtEndStep(t *testing.T) {
	gs := newGame(t, 2)
	bre := addPerm(gs, 0, "Bre of Clan Stoutarm", "legendary", "creature", "human")
	target := addPerm(gs, 0, "Vanilla Bear", "creature", "bear")

	gameengine.InvokeActivatedHook(gs, bre, 0, map[string]interface{}{
		"target_perm": target,
	})

	// Sanity: grant present.
	if target.Flags["kw:flying"] != 1 {
		t.Fatalf("setup: target should have kw:flying after activation")
	}

	// Fire end_step → delayed-trigger cleanup runs.
	gameengine.FireDelayedTriggers(gs, "ending", "end_step")

	if target.Flags["kw:flying"] == 1 {
		t.Errorf("kw:flying should strip at EOT; flags=%+v", target.Flags)
	}
	if target.Flags["kw:lifelink"] == 1 {
		t.Errorf("kw:lifelink should strip at EOT; flags=%+v", target.Flags)
	}
}

// TestBreOfClanStoutarm_GrantTargetSelfFails pins the "another" guard:
// activation with target_perm=Bre herself should fail without consuming
// the tap.
func TestBreOfClanStoutarm_GrantTargetSelfFails(t *testing.T) {
	gs := newGame(t, 2)
	bre := addPerm(gs, 0, "Bre of Clan Stoutarm", "legendary", "creature", "human")

	gameengine.InvokeActivatedHook(gs, bre, 0, map[string]interface{}{
		"target_perm": bre,
	})

	// Tap happens via fallback target search (no other creature available
	// for fallback either, so fail; activation aborts before tap).
	if bre.Tapped {
		t.Errorf("Bre should not be tapped if no valid target (must be 'another' creature)")
	}
}

// TestFearlessSwashbuckler_VehiclesGetHasteAnthemOnETB pins the
// "Vehicles you control have haste" anthem: ETB sweep stamps kw:haste
// on every Vehicle we control.
func TestFearlessSwashbuckler_VehiclesGetHasteAnthemOnETB(t *testing.T) {
	gs := newGame(t, 2)
	veh1 := addPerm(gs, 0, "Smuggler's Copter", "artifact", "vehicle")
	veh2 := addPerm(gs, 0, "Heart of Kiran", "artifact", "vehicle")
	nonVehicle := addPerm(gs, 0, "Sol Ring", "artifact")
	swash := addPerm(gs, 0, "Fearless Swashbuckler", "creature", "fish", "pirate")

	gameengine.InvokeETBHook(gs, swash)

	if veh1.Flags["kw:haste"] != 1 {
		t.Errorf("vehicle 1 should have kw:haste; flags=%+v", veh1.Flags)
	}
	if veh2.Flags["kw:haste"] != 1 {
		t.Errorf("vehicle 2 should have kw:haste; flags=%+v", veh2.Flags)
	}
	if nonVehicle.Flags["kw:haste"] == 1 {
		t.Errorf("non-vehicle artifact should not have kw:haste; flags=%+v", nonVehicle.Flags)
	}
}

// TestFearlessSwashbuckler_LTBStripsGrantedHasteOnly pins the per-flag
// strip: when Swashbuckler leaves, only flags we granted are removed.
func TestFearlessSwashbuckler_LTBStripsGrantedHasteOnly(t *testing.T) {
	gs := newGame(t, 2)
	grantedVeh := addPerm(gs, 0, "Smuggler's Copter", "artifact", "vehicle")
	nativeHasteVeh := addPerm(gs, 0, "Native Haste Vehicle", "artifact", "vehicle")
	nativeHasteVeh.Flags["kw:haste"] = 1 // native, pre-Swashbuckler
	swash := addPerm(gs, 0, "Fearless Swashbuckler", "creature", "fish", "pirate")

	gameengine.InvokeETBHook(gs, swash)

	// Sanity: granted vehicle picked up the grant marker.
	if grantedVeh.Flags["fearless_swashbuckler_haste_grant"] != 1 {
		t.Fatalf("setup: granted vehicle should carry the marker; flags=%+v", grantedVeh.Flags)
	}

	gameengine.FireCardTrigger(gs, "permanent_ltb", map[string]interface{}{
		"perm":            swash,
		"card":            swash.Card,
		"to_zone":         "graveyard",
		"controller_seat": 0,
	})

	if grantedVeh.Flags["kw:haste"] == 1 {
		t.Errorf("granted vehicle should lose kw:haste on LTB; flags=%+v", grantedVeh.Flags)
	}
	if nativeHasteVeh.Flags["kw:haste"] != 1 {
		t.Errorf("native-haste vehicle must keep kw:haste through LTB; flags=%+v", nativeHasteVeh.Flags)
	}
}
