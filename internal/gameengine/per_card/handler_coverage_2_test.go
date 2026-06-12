package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// dev/handler-coverage-2 test suite. One focused test per handler.

// ---------------------------------------------------------------------
// Arcades, the Strategist
// ---------------------------------------------------------------------

func TestArcades_DefenderETBDrawsCard(t *testing.T) {
	gs := newGame(t, 2)
	arcades := addPerm(gs, 0, "Arcades, the Strategist", "creature")
	addLibrary(gs, 0, "A", "B")

	wall := addPerm(gs, 0, "Tree of Redemption", "creature")
	wall.Flags["kw:defender"] = 1

	arcadesDefenderETBDraw(gs, arcades, map[string]interface{}{
		"perm":            wall,
		"controller_seat": 0,
		"card":            wall.Card,
	})

	if len(gs.Seats[0].Hand) != 1 {
		t.Fatalf("Arcades should have drawn 1 card on defender ETB; hand=%d", len(gs.Seats[0].Hand))
	}
}

func TestArcades_NondefenderDoesNotDraw(t *testing.T) {
	gs := newGame(t, 2)
	arcades := addPerm(gs, 0, "Arcades, the Strategist", "creature")
	addLibrary(gs, 0, "A")

	bear := addPerm(gs, 0, "Grizzly Bears", "creature")
	arcadesDefenderETBDraw(gs, arcades, map[string]interface{}{
		"perm":            bear,
		"controller_seat": 0,
		"card":            bear.Card,
	})

	if len(gs.Seats[0].Hand) != 0 {
		t.Fatalf("Arcades should not draw on a non-defender ETB; hand=%d", len(gs.Seats[0].Hand))
	}
}

// ---------------------------------------------------------------------
// Tifa Lockhart
// ---------------------------------------------------------------------

func TestTifa_LandfallDoublesPower(t *testing.T) {
	gs := newGame(t, 2)
	tifa := addPerm(gs, 0, "Tifa Lockhart", "creature")
	tifa.Card.BasePower = 4

	land := addPerm(gs, 0, "Forest", "land", "basic")
	tifaLockhartLandfall(gs, tifa, map[string]interface{}{
		"perm":            land,
		"controller_seat": 0,
		"card":            land.Card,
	})

	if tifa.Power() != 8 {
		t.Fatalf("Tifa should have power 8 (4 → 8) after landfall; got %d", tifa.Power())
	}
}

// ---------------------------------------------------------------------
// Veyran, Voice of Duality
// ---------------------------------------------------------------------

func TestVeyran_MagecraftAddsPlusOnePlusOne(t *testing.T) {
	gs := newGame(t, 2)
	veyran := addPerm(gs, 0, "Veyran, Voice of Duality", "creature")
	veyran.Card.BasePower = 2
	veyran.Card.BaseToughness = 2

	veyranMagecraftPump(gs, veyran, map[string]interface{}{
		"caster_seat": 0,
		"spell_name":  "Brainstorm",
	})

	if veyran.Power() != 3 {
		t.Fatalf("Veyran should be 3/3 after magecraft; got power %d", veyran.Power())
	}
}

// ---------------------------------------------------------------------
// Shadow the Hedgehog
// ---------------------------------------------------------------------

func TestShadow_FlashOrHasteDeathDraws(t *testing.T) {
	gs := newGame(t, 2)
	shadow := addPerm(gs, 0, "Shadow the Hedgehog", "creature")
	addLibrary(gs, 0, "A")

	dying := addPerm(gs, 0, "Goblin Guide", "creature")
	dying.Flags["kw:haste"] = 1
	shadowOnCreatureDies(gs, shadow, map[string]interface{}{
		"perm":            dying,
		"card":            dying.Card,
		"controller_seat": 0,
	})

	if len(gs.Seats[0].Hand) != 1 {
		t.Fatalf("Shadow should draw on haste-creature death; hand=%d", len(gs.Seats[0].Hand))
	}
}

func TestShadow_VanillaDeathDoesNothing(t *testing.T) {
	gs := newGame(t, 2)
	shadow := addPerm(gs, 0, "Shadow the Hedgehog", "creature")
	addLibrary(gs, 0, "A")

	dying := addPerm(gs, 0, "Grizzly Bears", "creature")
	shadowOnCreatureDies(gs, shadow, map[string]interface{}{
		"perm":            dying,
		"card":            dying.Card,
		"controller_seat": 0,
	})

	if len(gs.Seats[0].Hand) != 0 {
		t.Fatalf("Shadow should not draw on vanilla death; hand=%d", len(gs.Seats[0].Hand))
	}
}

// ---------------------------------------------------------------------
// Eriette of the Charmed Apple
// ---------------------------------------------------------------------

func TestEriette_DrainScalesWithAuras(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Eriette of the Charmed Apple", "creature")
	addPerm(gs, 0, "Curiosity", "enchantment", "aura")
	addPerm(gs, 0, "Pacifism", "enchantment", "aura")
	addPerm(gs, 0, "Pacifism", "enchantment", "aura")
	gs.Seats[0].Life = 30
	gs.Seats[1].Life = 30

	erietteEndStepDrain(gs, 0)

	if gs.Seats[0].Life != 33 {
		t.Errorf("seat 0 should gain 3 life (3 auras); got %d", gs.Seats[0].Life)
	}
	if gs.Seats[1].Life != 27 {
		t.Errorf("seat 1 should lose 3 life (3 auras); got %d", gs.Seats[1].Life)
	}
}

// ---------------------------------------------------------------------
// Inalla, Archmage Ritualist
// ---------------------------------------------------------------------

func TestInalla_WizardETBSpawnsHasteToken(t *testing.T) {
	// We exercise the handler directly to isolate the eminence-copy
	// behaviour from the engine's stack/priority round (which can
	// cascade into trigger-cap territory if ETB self-recursion isn't
	// guarded against in test fixtures).
	gs := newGame(t, 2)
	inalla := addPerm(gs, 0, "Inalla, Archmage Ritualist", "creature")
	gs.Seats[0].ManaPool = 2

	wiz := addPerm(gs, 0, "Azami, Lady of Scrolls", "creature", "wizard")
	bfBefore := len(gs.Seats[0].Battlefield)

	inallaEminenceCopy(gs, inalla, map[string]interface{}{
		"perm":            wiz,
		"controller_seat": 0,
		"card":            wiz.Card,
	})

	if len(gs.Seats[0].Battlefield) != bfBefore+1 {
		t.Errorf("Inalla should have spawned a token; battlefield grew by %d",
			len(gs.Seats[0].Battlefield)-bfBefore)
	}
	if gs.Seats[0].ManaPool != 1 {
		t.Errorf("Inalla should have spent 1 mana on the copy; pool=%d", gs.Seats[0].ManaPool)
	}
}

func TestInalla_TapFiveDrains7(t *testing.T) {
	gs := newGame(t, 2)
	inalla := addPerm(gs, 0, "Inalla, Archmage Ritualist", "creature")
	for i := 0; i < 5; i++ {
		addPerm(gs, 0, "Wizard Pawn", "creature", "wizard")
	}
	gs.Seats[1].Life = 40

	inallaTapFiveDrain(gs, inalla, 0, map[string]interface{}{"target_seat": 1})

	if gs.Seats[1].Life != 33 {
		t.Errorf("Inalla activation should drain 7 life from seat 1; got %d", gs.Seats[1].Life)
	}
}

// ---------------------------------------------------------------------
// Eddie Brock
// ---------------------------------------------------------------------

func TestEddieBrock_ETBReanimatesMV1Creature(t *testing.T) {
	gs := newGame(t, 2)
	target := &gameengine.Card{
		Name:      "Stitcher's Supplier",
		Owner:     0,
		Types:     []string{"creature", "cost:1"},
		BasePower: 1,
	}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, target)
	bfBefore := len(gs.Seats[0].Battlefield)

	eddie := addPerm(gs, 0, "Eddie Brock // Venom, Lethal Protector", "creature")
	eddieBrockETB(gs, eddie)

	if len(gs.Seats[0].Battlefield) <= bfBefore {
		t.Errorf("Eddie should have brought back a creature; before=%d after=%d",
			bfBefore, len(gs.Seats[0].Battlefield))
	}
	if len(gs.Seats[0].Graveyard) != 0 {
		t.Errorf("graveyard should be empty after reanimate; got %d", len(gs.Seats[0].Graveyard))
	}
}

// ---------------------------------------------------------------------
// Tiamat
// ---------------------------------------------------------------------

func TestTiamat_ETBPullsUpToFiveDifferentDragons(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Library = []*gameengine.Card{
		{Name: "Niv-Mizzet, the Firemind", Owner: 0, Types: []string{"creature", "dragon"}},
		{Name: "Niv-Mizzet, the Firemind", Owner: 0, Types: []string{"creature", "dragon"}}, // dupe — skipped
		{Name: "Atarka, World Render", Owner: 0, Types: []string{"creature", "dragon"}},
		{Name: "Tiamat", Owner: 0, Types: []string{"creature", "dragon"}}, // self — skipped
		{Name: "Dragonlord Atarka", Owner: 0, Types: []string{"creature", "dragon"}},
		{Name: "Lightning Bolt", Owner: 0, Types: []string{"instant"}}, // not a dragon
	}

	tiamat := addPerm(gs, 0, "Tiamat", "creature", "dragon")
	tiamat.Flags["was_cast"] = 1 // intervening-if: "if you cast it"
	// Dispatch through fireETB (not a direct handler call) so this test
	// also pins single-fire composite behavior — the r61 C-1 bug was two
	// stacked registrations that only dispatch-level tests could see.
	fireETB(gs, tiamat)

	if len(gs.Seats[0].Hand) != 3 {
		t.Errorf("Tiamat should have pulled 3 different-named non-Tiamat dragons; hand=%d", len(gs.Seats[0].Hand))
	}
}

func TestTiamat_ETBNoTutorWhenNotCast(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Library = []*gameengine.Card{
		{Name: "Niv-Mizzet, the Firemind", Owner: 0, Types: []string{"creature", "dragon"}},
		{Name: "Atarka, World Render", Owner: 0, Types: []string{"creature", "dragon"}},
	}

	// Reanimated / blinked Tiamat — no was_cast flag.
	tiamat := addPerm(gs, 0, "Tiamat", "creature", "dragon")
	fireETB(gs, tiamat)

	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("Tiamat should not tutor when not cast; hand=%d", len(gs.Seats[0].Hand))
	}
}

// ---------------------------------------------------------------------
// Mayael the Anima
// ---------------------------------------------------------------------

func TestMayael_PutsPower5CreatureOnBattlefield(t *testing.T) {
	gs := newGame(t, 2)
	mayael := addPerm(gs, 0, "Mayael the Anima", "creature")
	gs.Seats[0].Library = []*gameengine.Card{
		{Name: "Llanowar Elves", Owner: 0, Types: []string{"creature"}, BasePower: 1},
		{Name: "Birds of Paradise", Owner: 0, Types: []string{"creature"}, BasePower: 0},
		{Name: "Avenger of Zendikar", Owner: 0, Types: []string{"creature"}, BasePower: 5},
		{Name: "Forest", Owner: 0, Types: []string{"land"}},
		{Name: "Elvish Mystic", Owner: 0, Types: []string{"creature"}, BasePower: 1},
	}
	gs.Seats[0].ManaPool = 6 // {3}{R}{G}{W}
	bfBefore := len(gs.Seats[0].Battlefield)

	mayaelLookFive(gs, mayael, 0, nil)

	if len(gs.Seats[0].Battlefield) <= bfBefore {
		t.Errorf("Mayael should have put one creature on battlefield; before=%d after=%d",
			bfBefore, len(gs.Seats[0].Battlefield))
	}
}

func TestMayael_NoQualifyingCreatureBottoms(t *testing.T) {
	gs := newGame(t, 2)
	mayael := addPerm(gs, 0, "Mayael the Anima", "creature")
	gs.Seats[0].Library = []*gameengine.Card{
		{Name: "Llanowar Elves", Owner: 0, Types: []string{"creature"}, BasePower: 1},
		{Name: "Birds of Paradise", Owner: 0, Types: []string{"creature"}, BasePower: 0},
		{Name: "Forest", Owner: 0, Types: []string{"land"}},
		{Name: "Mountain", Owner: 0, Types: []string{"land"}},
		{Name: "Plains", Owner: 0, Types: []string{"land"}},
	}
	gs.Seats[0].ManaPool = 6
	libBefore := len(gs.Seats[0].Library)

	mayaelLookFive(gs, mayael, 0, nil)

	if len(gs.Seats[0].Library) != libBefore {
		t.Errorf("library should be unchanged in size when no creature qualifies; before=%d after=%d",
			libBefore, len(gs.Seats[0].Library))
	}
}

// ---------------------------------------------------------------------
// Giada, Font of Hope
// ---------------------------------------------------------------------

func TestGiada_AngelETBGetsCounterPerExistingAngel(t *testing.T) {
	gs := newGame(t, 2)
	giada := addPerm(gs, 0, "Giada, Font of Hope", "creature", "angel")
	addPerm(gs, 0, "Serra Angel", "creature", "angel")
	addPerm(gs, 0, "Lyra Dawnbringer", "creature", "angel")

	newAngel := addPerm(gs, 0, "Bishop of Wings", "creature", "angel")
	giadaAngelCounters(gs, giada, map[string]interface{}{
		"perm":            newAngel,
		"controller_seat": 0,
		"card":            newAngel.Card,
	})

	// Giada + Serra + Lyra = 3 angels already in play — Bishop gets 3 counters.
	if newAngel.Counters["+1/+1"] != 3 {
		t.Errorf("Bishop should have 3 +1/+1 counters; got %d", newAngel.Counters["+1/+1"])
	}
}

// ---------------------------------------------------------------------
// Ghyrson Starn, Kelermorph
// ---------------------------------------------------------------------

func TestGhyrsonStarn_OneDamageTriggersTwo(t *testing.T) {
	gs := newGame(t, 2)
	ghyrson := addPerm(gs, 0, "Ghyrson Starn, Kelermorph", "creature")
	gs.Seats[1].Life = 20
	source := addPerm(gs, 0, "Walking Ballista", "creature", "artifact")

	ghyrsonOnNoncombatPlayer(gs, ghyrson, map[string]interface{}{
		"source_perm": source,
		"target_seat": 1,
		"amount":      1,
	})

	if gs.Seats[1].Life != 18 {
		t.Errorf("Ghyrson should pile +2 damage on a 1-damage trigger; expected 18, got %d", gs.Seats[1].Life)
	}
}

func TestGhyrsonStarn_TwoDamageDoesNotTrigger(t *testing.T) {
	gs := newGame(t, 2)
	ghyrson := addPerm(gs, 0, "Ghyrson Starn, Kelermorph", "creature")
	gs.Seats[1].Life = 20
	source := addPerm(gs, 0, "Walking Ballista", "creature", "artifact")

	ghyrsonOnNoncombatPlayer(gs, ghyrson, map[string]interface{}{
		"source_perm": source,
		"target_seat": 1,
		"amount":      2,
	})

	if gs.Seats[1].Life != 20 {
		t.Errorf("Ghyrson should NOT trigger on 2 damage; expected 20, got %d", gs.Seats[1].Life)
	}
}

// ---------------------------------------------------------------------
// Choco, Seeker of Paradise
// ---------------------------------------------------------------------

func TestChoco_LandfallPumpsOne(t *testing.T) {
	gs := newGame(t, 2)
	choco := addPerm(gs, 0, "Choco, Seeker of Paradise", "creature", "bird")
	choco.Card.BasePower = 2

	land := addPerm(gs, 0, "Forest", "land", "basic")
	chocoLandfall(gs, choco, map[string]interface{}{
		"perm":            land,
		"controller_seat": 0,
		"card":            land.Card,
	})

	if choco.Power() != 3 {
		t.Errorf("Choco should be +1/+0 after landfall; got power %d", choco.Power())
	}
}

// ---------------------------------------------------------------------
// Isshin, Two Heavens as One
// ---------------------------------------------------------------------

func TestIsshin_AttackSetsActiveSeatFlag(t *testing.T) {
	gs := newGame(t, 2)
	isshin := addPerm(gs, 0, "Isshin, Two Heavens as One", "creature")

	atk := addPerm(gs, 0, "Goblin Guide", "creature")
	isshinOnAttack(gs, isshin, map[string]interface{}{
		"attacker_perm": atk,
		"attacker_seat": 0,
		"attacker_card": atk.Card,
	})

	if gs.Flags["isshin_active_seat"] != 1 {
		t.Errorf("isshin_active_seat should be 1 (controller 0 + 1); got %d", gs.Flags["isshin_active_seat"])
	}
}

// ---------------------------------------------------------------------
// Registry smoke check — every dev/handler-coverage-2 commander has an
// entry on the registry after init().
// ---------------------------------------------------------------------

func TestHandlerCoverage2_AllRegistered(t *testing.T) {
	// Ensure registrations are present even if a prior test called Reset().
	RegisterHandlerCoverage2(Global())
	cards := []string{
		"Arcades, the Strategist",
		"Tifa Lockhart",
		"Veyran, Voice of Duality",
		"Shadow the Hedgehog",
		"Eriette of the Charmed Apple",
		"Inalla, Archmage Ritualist",
		"Eddie Brock // Venom, Lethal Protector",
		"Tiamat",
		"Mayael the Anima",
		"Giada, Font of Hope",
		"Ghyrson Starn, Kelermorph",
		"Choco, Seeker of Paradise",
		"Isshin, Two Heavens as One",
	}
	for _, name := range cards {
		hasAny := HasETB(name) || HasResolve(name) || HasActivated(name) || hasAnyTrigger(name)
		if !hasAny {
			t.Errorf("%q should have at least one registered handler", name)
		}
	}
}

func hasAnyTrigger(name string) bool {
	reg := Global()
	reg.mu.RLock()
	defer reg.mu.RUnlock()
	byEvent, ok := reg.onTrigger[normalizeName(name)]
	if !ok {
		return false
	}
	for _, hs := range byEvent {
		if len(hs) > 0 {
			return true
		}
	}
	return false
}
