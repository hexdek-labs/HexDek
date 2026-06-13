package gameengine

import "testing"

// checker_layer_awareness_r63_test.go — regression pins for the
// layer-awareness checker bug class (follow-up to the §704.5f feynman fix
// 40477cbf). Invariant checkers must decide violations from CR-613
// LAYER-APPLIED characteristics, not raw printed/base values, or they
// phantom-flag legal states (animated creatures, layer-stripped keywords,
// layer-granted keywords).

// --- checkCombatLegality: haste GRANTED by a pure layer-6 effect --------

// Toph cards grant haste via RegisterGrantKeyword (a pure layer-6 effect
// with no Flags["kw:haste"]). A summoning-sick creature with such haste can
// legally attack — the raw p.HasKeyword misses the grant, so the checker
// must also consult gs.HasKeywordOf (union).
func TestCombatLegality_LayerGrantedHaste_NotFlagged(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	gs.Phase = "combat"
	gs.Step = "declare_attackers"
	addBattlefield(gs, 1, "Bear", 2, 2, "creature")

	atk := addBattlefield(gs, 0, "Hasty Summon", 2, 2, "creature")
	atk.SummoningSick = true
	atk.Flags["attacking"] = 1

	// Sanity: summoning-sick, no haste yet → must be flagged.
	if err := checkCombatLegality(gs); err == nil {
		t.Fatal("precondition: summoning-sick attacker with no haste must be flagged")
	}

	// Pure layer-6 haste grant (Toph-style) — no Flags["kw:haste"].
	RegisterGrantKeyword(gs, atk, "haste", DurationUntilSourceLeaves, "Toph", "test")
	gs.InvalidateCharacteristicsCache()

	if atk.HasKeyword("haste") {
		t.Fatal("precondition: raw HasKeyword must NOT see the pure-layer haste grant")
	}
	if !gs.HasKeywordOf(atk, "haste") {
		t.Fatal("precondition: layer-aware read must see the granted haste")
	}
	if err := checkCombatLegality(gs); err != nil {
		t.Fatalf("layer-granted haste — attacking is legal, must NOT flag: %v", err)
	}
}

// Guardrail: a genuinely summoning-sick, hasteless attacker is STILL
// flagged — the union fix only adds recognition, it does not blind the
// checker to a real illegal attack.
func TestCombatLegality_GenuineSummoningSick_StillFlagged(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	gs.Phase = "combat"
	gs.Step = "declare_attackers"
	addBattlefield(gs, 1, "Bear", 2, 2, "creature")

	atk := addBattlefield(gs, 0, "Slowpoke", 2, 2, "creature")
	atk.SummoningSick = true
	atk.Flags["attacking"] = 1
	gs.InvalidateCharacteristicsCache()

	if err := checkCombatLegality(gs); err == nil {
		t.Fatal("a genuine summoning-sick hasteless attacker must be flagged")
	}
}

// --- checkAttachmentConsistency: equipment on an ANIMATED creature ------

// Equipment attached to a permanent that is a creature only by a layer-4
// type-add (Ensoul Artifact / March of the Machines / animated land) must
// not be phantom-flagged as "attached to a non-creature".
func TestAttachmentConsistency_EquipmentOnAnimatedCreature_NotFlagged(t *testing.T) {
	gs := newFixtureGame(t)

	// A base non-creature artifact, animated to a creature by layer 4.
	host := addBattlefield(gs, 0, "Ensouled Bell", 0, 0, "artifact")
	RegisterAddTypes(gs, host, []string{"creature"}, DurationUntilSourceLeaves, "Ensoul Artifact", "test")
	RegisterSetPT(gs, host, 5, 5, DurationUntilSourceLeaves, "Ensoul Artifact", "test")

	equip := addBattlefield(gs, 0, "Bonesplitter", 0, 0, "artifact", "equipment")
	equip.AttachedTo = host
	gs.InvalidateCharacteristicsCache()

	// Sanity: base type is non-creature, layer type is creature.
	if host.IsCreature() {
		t.Fatal("precondition: host base type must be non-creature")
	}
	if !gs.IsCreatureOf(host) {
		t.Fatal("precondition: host must be a creature by layer")
	}
	if err := checkAttachmentConsistency(gs); err != nil {
		t.Fatalf("equipment on an animated (layer) creature must NOT be flagged: %v", err)
	}
}

// Guardrail: equipment on a genuine non-creature is STILL flagged.
func TestAttachmentConsistency_EquipmentOnNonCreature_StillFlagged(t *testing.T) {
	gs := newFixtureGame(t)

	rock := addBattlefield(gs, 0, "Mana Rock", 0, 0, "artifact")
	equip := addBattlefield(gs, 0, "Bonesplitter", 0, 0, "artifact", "equipment")
	equip.AttachedTo = rock
	gs.InvalidateCharacteristicsCache()

	if err := checkAttachmentConsistency(gs); err == nil {
		t.Fatal("equipment attached to a genuine non-creature must be flagged")
	}
}
