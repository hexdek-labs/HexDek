package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// sba_type_layer_aware_r63_test.go — pins the CONFIRMED engine-side gap
// proven in /tmp/fable-review/checker-layer-awareness-r63.md: the
// type-based state-based actions bucketed permanents by their RAW printed
// types instead of the §613 LAYER-APPLIED types, so a permanent whose
// type is changed by a continuous effect was mis-handled. This is the
// type-predicate sibling of the r60 sba704_5f (toughness) layer-aware fix.
//
// Both directions of the §704.5n equipment SBA are pinned:
//   - OVER-application: equipment on a host that is a creature ONLY by a
//     layer-4 type-add (Ensoul Artifact) must STAY attached.
//   - UNDER-application: equipment on a host whose creature type is
//     STRIPPED by a layer-4 effect (Song of the Dryads) must UNATTACH.
// Plus guardrails that the genuine cases still fire / still skip.

// registerTypeAdd registers a layer-4 effect that APPENDS `addType` to
// `perm`'s types (Ensoul Artifact / March of the Machines shape).
func registerTypeAdd(gs *GameState, perm *Permanent, name, addType string) {
	gs.RegisterContinuousEffect(&ContinuousEffect{
		Layer: LayerType, Timestamp: gs.NextTimestamp(),
		SourcePerm: perm, SourceCardName: name,
		ControllerSeat: perm.Controller,
		HandlerID:      "test_type_add_" + addType,
		ApplyFn: func(_ *GameState, q *Permanent, chars *Characteristics) {
			if q == perm {
				chars.Types = append(chars.Types, addType)
			}
		},
	})
	gs.InvalidateCharacteristicsCache()
}

// registerTypeReplace registers a layer-4 effect that REPLACES `perm`'s
// types with `newTypes` (Song of the Dryads → land shape).
func registerTypeReplace(gs *GameState, perm *Permanent, name string, newTypes ...string) {
	gs.RegisterContinuousEffect(&ContinuousEffect{
		Layer: LayerType, Timestamp: gs.NextTimestamp(),
		SourcePerm: perm, SourceCardName: name,
		ControllerSeat: perm.Controller,
		HandlerID:      "test_type_replace",
		ApplyFn: func(_ *GameState, q *Permanent, chars *Characteristics) {
			if q == perm {
				chars.Types = append([]string{}, newTypes...)
			}
		},
	})
	gs.InvalidateCharacteristicsCache()
}

// PROVEN over-application: §704.5n unattached an Equipment from a host
// that's a creature only by layers. The fix keeps it attached.
func TestSBA704_5n_EquipmentStaysOnLayerAnimatedHost(t *testing.T) {
	gs := newFixtureGame(t)
	host := addBattlefield(gs, 0, "Ornithopter", 0, 2, "artifact") // NOT a creature by print
	equip := addBattlefield(gs, 0, "Bonesplitter", 0, 0, "artifact", "equipment")
	equip.AttachedTo = host

	registerTypeAdd(gs, host, "Ensoul Artifact", "creature")

	// Precondition: raw type-blind sees a non-creature; layers see a creature.
	if host.IsCreature() {
		t.Fatal("fixture: host should NOT be a printed creature")
	}
	if !gs.IsCreatureOf(host) {
		t.Fatal("fixture: host SHOULD be a creature by layer (Ensoul)")
	}

	StateBasedActions(gs)

	if equip.AttachedTo != host {
		t.Fatalf("§704.5n WRONGLY unattached equipment from a layer-animated creature host (over-application)")
	}
}

// PROVEN under-application: §704.5n failed to unattach an Equipment from a
// host whose creature type was STRIPPED by layers. The fix unattaches it.
func TestSBA704_5n_EquipmentFallsOffTypeStrippedHost(t *testing.T) {
	gs := newFixtureGame(t)
	host := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")
	equip := addBattlefield(gs, 0, "Bonesplitter", 0, 0, "artifact", "equipment")
	equip.AttachedTo = host

	registerTypeReplace(gs, host, "Song of the Dryads", "land") // no longer a creature

	if !host.IsCreature() {
		t.Fatal("fixture: host should still be a printed creature")
	}
	if gs.IsCreatureOf(host) {
		t.Fatal("fixture: host should be a NON-creature by layer (Song of the Dryads)")
	}

	StateBasedActions(gs)

	if equip.AttachedTo != nil {
		t.Fatalf("§704.5n FAILED to unattach equipment from a host that is no longer a creature (under-application)")
	}
}

// Guardrail: equipment on a genuine non-creature (no layer rescue) STILL
// unattaches — the fix didn't blind the SBA.
func TestSBA704_5n_EquipmentFallsOffGenuineNonCreature(t *testing.T) {
	gs := newFixtureGame(t)
	host := addBattlefield(gs, 0, "Sol Ring", 0, 0, "artifact") // plain non-creature artifact
	equip := addBattlefield(gs, 0, "Bonesplitter", 0, 0, "artifact", "equipment")
	equip.AttachedTo = host

	StateBasedActions(gs)

	if equip.AttachedTo != nil {
		t.Fatalf("§704.5n must still unattach equipment from a genuine non-creature")
	}
}

// Guardrail: equipment correctly attached to a printed creature stays put.
func TestSBA704_5n_EquipmentStaysOnPlainCreature(t *testing.T) {
	gs := newFixtureGame(t)
	host := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")
	equip := addBattlefield(gs, 0, "Bonesplitter", 0, 0, "artifact", "equipment")
	equip.AttachedTo = host

	StateBasedActions(gs)

	if equip.AttachedTo != host {
		t.Fatalf("§704.5n wrongly unattached equipment from a normal creature host")
	}
}

// §704.5m aura enchant-clause is layer-aware too: an "Enchant creature"
// aura on a host whose creature type is stripped is put into the
// graveyard (§303.4f), and one on a layer-animated creature stays.
func TestSBA704_5m_EnchantClauseLayerAware(t *testing.T) {
	// Stripped host → aura dies.
	gs := newFixtureGame(t)
	host := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")
	aura := addBattlefield(gs, 0, "Pacifism", 0, 0, "enchantment", "aura")
	aura.Card.AST = &gameast.CardAST{
		Name:      "Pacifism",
		Abilities: []gameast.Ability{&gameast.Static{Raw: "Enchant creature. Enchanted creature can't attack or block."}},
	}
	aura.AttachedTo = host

	registerTypeReplace(gs, host, "Song of the Dryads", "land")
	StateBasedActions(gs)

	if onBattlefield(gs, aura) {
		t.Fatalf("§704.5m: 'Enchant creature' aura should fall off a host stripped of creature-ness")
	}
}

// Guardrail for §704.5m: the same aura on a normal creature host survives.
func TestSBA704_5m_EnchantClauseAuraOnCreatureSurvives(t *testing.T) {
	gs := newFixtureGame(t)
	host := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")
	aura := addBattlefield(gs, 0, "Pacifism", 0, 0, "enchantment", "aura")
	aura.Card.AST = &gameast.CardAST{
		Name:      "Pacifism",
		Abilities: []gameast.Ability{&gameast.Static{Raw: "Enchant creature. Enchanted creature can't attack or block."}},
	}
	aura.AttachedTo = host

	StateBasedActions(gs)

	if !onBattlefield(gs, aura) {
		t.Fatalf("§704.5m: aura on a normal creature must survive")
	}
}
