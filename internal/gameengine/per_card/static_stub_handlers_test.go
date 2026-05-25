package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// Tests for the static-stub-handlers batch. Focused per-handler
// behaviors — registry smoke tests + the headline mechanic each card
// brings.

// ---------------------------------------------------------------------
// Avacyn, Angel of Hope — indestructible anthem
// ---------------------------------------------------------------------

func TestAvacyn_GrantsIndestructibleToOtherPermanents(t *testing.T) {
	gs := newGame(t, 2)
	avacyn := addPerm(gs, 0, "Avacyn, Angel of Hope", "creature", "angel")
	bear := addPerm(gs, 0, "Grizzly Bears", "creature")
	enemy := addPerm(gs, 1, "Goblin", "creature")

	avacynGrantIndestructible(gs, avacyn)

	if !bear.IsIndestructible() {
		t.Fatalf("Avacyn should grant indestructible to other permanents we control")
	}
	if enemy.IsIndestructible() {
		t.Fatalf("Avacyn should NOT grant indestructible to opponents' permanents")
	}
}

func TestAvacyn_DoesNotRegrantToSelf(t *testing.T) {
	gs := newGame(t, 2)
	avacyn := addPerm(gs, 0, "Avacyn, Angel of Hope", "creature", "angel")
	avacynGrantIndestructible(gs, avacyn)
	// The grant predicate excludes the source — verifying via the
	// applied flag that we don't double-stamp.
	chars := gameengine.GetEffectiveCharacteristics(gs, avacyn)
	count := 0
	if chars != nil {
		for _, k := range chars.Keywords {
			if k == "indestructible" {
				count++
			}
		}
	}
	if count > 1 {
		t.Fatalf("Avacyn shouldn't double-grant indestructible to herself; got %d", count)
	}
}

// ---------------------------------------------------------------------
// Maelstrom Wanderer — second cascade + haste anthem
// ---------------------------------------------------------------------

func TestMaelstromWanderer_ETBFiresSecondCascadeAndGrantsHaste(t *testing.T) {
	gs := newGame(t, 2)
	// Empty library — cascade whiffs, but the haste anthem grant
	// should still fire. (A non-empty library would resolve a
	// cascade-cast spell back through the engine, which has
	// downstream effects we don't want this focused test to depend
	// on.)
	mw := addPerm(gs, 0, "Maelstrom Wanderer", "creature", "elemental")
	mw.Card.CMC = 8

	bear := addPerm(gs, 0, "Grizzly Bears", "creature")
	bear.SummoningSick = true

	maelstromWandererETB(gs, mw)

	// Cascade event should be in the log (whiff is fine — we just
	// want to verify the second cascade fired).
	if hasEvent(gs, "cascade_trigger")+hasEvent(gs, "cascade_hit")+hasEvent(gs, "cascade_whiff") == 0 {
		t.Fatalf("Maelstrom Wanderer ETB should fire a cascade event")
	}

	// Haste grant — bear should have kw:haste flag and not be sick.
	if bear.Flags["kw:haste"] != 1 {
		t.Fatalf("Maelstrom should grant haste to creatures we control; flags=%v", bear.Flags)
	}
	if bear.SummoningSick {
		t.Fatalf("Maelstrom haste should clear summoning sickness on creatures we control")
	}
}

func TestMaelstromWanderer_DoesNotGrantHasteToOpponents(t *testing.T) {
	gs := newGame(t, 2)
	addLibrary(gs, 0, "Forest")
	mw := addPerm(gs, 0, "Maelstrom Wanderer", "creature")
	mw.Card.CMC = 8
	enemy := addPerm(gs, 1, "Goblin", "creature")
	enemy.SummoningSick = true

	maelstromWandererETB(gs, mw)

	if enemy.Flags["kw:haste"] == 1 {
		t.Fatalf("Maelstrom should NOT grant haste to opponents' creatures")
	}
}

// ---------------------------------------------------------------------
// Feather, the Redeemed — exile-on-resolve + return at end step
// ---------------------------------------------------------------------

func TestFeather_StampsExileOnResolveWhenSpellTargetsOurCreature(t *testing.T) {
	gs := newGame(t, 2)
	feather := addPerm(gs, 0, "Feather, the Redeemed", "creature", "angel")
	target := addPerm(gs, 0, "Bear", "creature")

	bolt := &gameengine.Card{Name: "Lightning Helix", Owner: 0, Types: []string{"instant"}}
	item := &gameengine.StackItem{
		Controller: 0,
		Card:       bolt,
		Kind:       "spell",
		Targets:    []gameengine.Target{{Kind: gameengine.TargetKindPermanent, Permanent: target, Seat: 0}},
	}
	gs.Stack = append(gs.Stack, item)

	featherExileAndReturn(gs, feather, map[string]interface{}{
		"caster_seat": 0,
		"card":        bolt,
	})

	v, ok := item.CostMeta["exile_on_resolve"].(bool)
	if !ok || !v {
		t.Fatalf("Feather should stamp exile_on_resolve=true on the spell; got %v", item.CostMeta)
	}
	if len(gs.DelayedTriggers) == 0 {
		t.Fatalf("Feather should register a delayed trigger to return the card at next end step")
	}
}

func TestFeather_DoesNotStampWhenSpellTargetsOpponentCreature(t *testing.T) {
	gs := newGame(t, 2)
	feather := addPerm(gs, 0, "Feather, the Redeemed", "creature")
	enemy := addPerm(gs, 1, "Goblin", "creature")

	bolt := &gameengine.Card{Name: "Lightning Bolt", Owner: 0, Types: []string{"instant"}}
	item := &gameengine.StackItem{
		Controller: 0,
		Card:       bolt,
		Kind:       "spell",
		Targets:    []gameengine.Target{{Kind: gameengine.TargetKindPermanent, Permanent: enemy, Seat: 1}},
	}
	gs.Stack = append(gs.Stack, item)

	featherExileAndReturn(gs, feather, map[string]interface{}{
		"caster_seat": 0,
		"card":        bolt,
	})

	if v, _ := item.CostMeta["exile_on_resolve"].(bool); v {
		t.Fatalf("Feather should NOT stamp exile_on_resolve when the target is an opponent's creature")
	}
}

// ---------------------------------------------------------------------
// Kess, Dissident Mage — once-per-turn cast-from-graveyard
//
// R60 rewired Kess onto the once_per_turn_cast_from_graveyard primitive
// in gameengine/zone_cast.go. The handler no longer pushes a synthetic
// stack item from an activated hook — it registers ZoneCastGrants and
// the engine's CastFromZone consumes them. These tests verify the new
// contract: ETB seeds grants for every instant/sorcery in the
// controller's graveyard with the right shape (exile-on-resolve,
// once-per-turn-per-source, source-pinned to this Kess).
// ---------------------------------------------------------------------

func TestKess_ETBGrantsExileOnResolveForGraveyardInstantsAndSorceries(t *testing.T) {
	gs := newGame(t, 2)
	kess := addPerm(gs, 0, "Kess, Dissident Mage", "creature")
	kess.Flags = map[string]int{}

	bolt := &gameengine.Card{Name: "Lightning Bolt", Owner: 0, Types: []string{"instant", "cmc:1"}}
	wrath := &gameengine.Card{Name: "Wrath of God", Owner: 0, Types: []string{"sorcery", "cmc:4"}}
	bear := &gameengine.Card{Name: "Grizzly Bears", Owner: 0, Types: []string{"creature"}}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, bolt, wrath, bear)

	kessETB(gs, kess)

	pBolt := gameengine.GetZoneCastGrant(gs, bolt)
	pWrath := gameengine.GetZoneCastGrant(gs, wrath)
	pBear := gameengine.GetZoneCastGrant(gs, bear)

	if pBolt == nil || pWrath == nil {
		t.Fatalf("Kess ETB must grant cast-from-graveyard to instant + sorcery (bolt=%v wrath=%v)", pBolt, pWrath)
	}
	if pBear != nil {
		t.Fatalf("Kess ETB must NOT grant cast permission to a creature (got %+v)", pBear)
	}
	if !pBolt.ExileOnResolve || !pWrath.ExileOnResolve {
		t.Fatalf("Kess grants must be exile-on-resolve per oracle text")
	}
	if !pBolt.OncePerTurnPerSource || pBolt.SourceTimestamp != kess.Timestamp {
		t.Fatalf("Kess grant must be once-per-turn pinned to this Kess (got src=%d, want %d, opt=%v)",
			pBolt.SourceTimestamp, kess.Timestamp, pBolt.OncePerTurnPerSource)
	}
	if pBolt.RequireController != 0 {
		t.Fatalf("Kess grant should restrict casting to Kess's controller")
	}
}

func TestKess_RefreshGrantsNewlyMilledInstants(t *testing.T) {
	gs := newGame(t, 2)
	kess := addPerm(gs, 0, "Kess, Dissident Mage", "creature")
	kess.Flags = map[string]int{}
	kessETB(gs, kess)

	// New instant lands in the graveyard mid-turn (mill/discard/etc.).
	freshBolt := &gameengine.Card{Name: "Counterspell", Owner: 0, Types: []string{"instant"}}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, freshBolt)
	kessRefreshGrants(gs, kess, nil)

	if g := gameengine.GetZoneCastGrant(gs, freshBolt); g == nil {
		t.Fatalf("refresh must pick up instants that landed in the graveyard after ETB")
	}
}

func TestKess_GrantExpiresWhenSourceLeavesBattlefield(t *testing.T) {
	// We don't hook LTB directly — the engine's EOT cleanup
	// (ExpireZoneCastGrants) prunes "while_source_on_bf" grants when no
	// permanent with the source's Timestamp is on the battlefield. This
	// test simulates Kess leaving by removing her permanent, then runs
	// the cleanup and verifies the grant is gone.
	gs := newGame(t, 2)
	kess := addPerm(gs, 0, "Kess, Dissident Mage", "creature")
	kess.Flags = map[string]int{}
	bolt := &gameengine.Card{Name: "Lightning Bolt", Owner: 0, Types: []string{"instant"}}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, bolt)
	kessETB(gs, kess)

	if gameengine.GetZoneCastGrant(gs, bolt) == nil {
		t.Fatalf("precondition: Kess should have granted Lightning Bolt a cast permission")
	}

	// Yank Kess off the battlefield.
	bf := gs.Seats[0].Battlefield[:0]
	for _, p := range gs.Seats[0].Battlefield {
		if p != kess {
			bf = append(bf, p)
		}
	}
	gs.Seats[0].Battlefield = bf

	gameengine.ExpireZoneCastGrants(gs)
	if gameengine.GetZoneCastGrant(gs, bolt) != nil {
		t.Fatalf("EOT cleanup must revoke grants whose source has left the battlefield")
	}
}

// ---------------------------------------------------------------------
// Hogaak, Arisen Necropolis — cast-restriction flags
// ---------------------------------------------------------------------

func TestHogaak_ETBSetsCastRestrictionFlags(t *testing.T) {
	gs := newGame(t, 2)
	hog := addPerm(gs, 0, "Hogaak, Arisen Necropolis", "creature", "avatar")

	hogaakRegisterCastFlags(gs, hog)

	if gs.Flags["hogaak_graveyard_castable_seat"] != 1 {
		t.Fatalf("Hogaak ETB should set graveyard_castable_seat=controller+1; got %d",
			gs.Flags["hogaak_graveyard_castable_seat"])
	}
	if gs.Flags["hogaak_no_mana_cast_seat"] != 1 {
		t.Fatalf("Hogaak ETB should set no_mana_cast_seat=controller+1; got %d",
			gs.Flags["hogaak_no_mana_cast_seat"])
	}
}
