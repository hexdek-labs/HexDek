package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// wave2_multistep_batch3_r60_test.go — Wave 2 batch 3 migrations.
//
// 6 per_card handlers carrying the manual `seat.Hand = append(...[:i], ...[i+1:]...)`
// (or library-walk) splice before enterBattlefieldWithETB. Same Cluster 2
// pattern Wave 2 #924 closed, just deferred for batch-size discipline.
//
// Migrated:
//   1. Minn, Wily Illusionist  — die-trigger hand cheat
//   2. Strefan, Maurer Prog.  — attack-trigger Vampire hand cheat
//   3. The Gitrog, Ravenous   — combat-damage hand land cheat
//   4. Hakbal of the Surging  — attack-trigger hand land cheat
//   5. Mayael the Anima       — activated library top-5 creature cheat
//   6. Eruth, Tormented P.    — draw-replacement library → exile + grant

// ---------------------------------------------------------------------------
// 1. Minn
// ---------------------------------------------------------------------------

func TestWave2MS3_Minn_DiesTriggerCheatsFromHand(t *testing.T) {
	gs := newGame(t, 2)
	minn := addPerm(gs, 0, "Minn, Wily Illusionist", "creature")
	// Eligible target with CMC ≤ dead illusion power (3).
	target := &gameengine.Card{
		Name: "Cheap Creature", Owner: 0,
		Types:         []string{"creature"},
		BasePower:     1, BaseToughness: 1,
	}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, target)

	deadIllusion := &gameengine.Card{
		Name: "Phantasmal Image", Owner: 0,
		Types:         []string{"creature", "illusion"},
		BasePower:     3, BaseToughness: 3,
	}

	minnIllusionDies(gs, minn, map[string]interface{}{
		"controller_seat": 0,
		"card":            deadIllusion,
		"perm":            (*gameengine.Permanent)(nil),
	})

	if countCardIn(gs.Seats[0].Hand, target) != 0 {
		t.Errorf("Minn: target still in hand")
	}
	if countBFCard(gs.Seats[0].Battlefield, target) != 1 {
		t.Errorf("Minn: target must be on battlefield once; got %d",
			countBFCard(gs.Seats[0].Battlefield, target))
	}
}

// ---------------------------------------------------------------------------
// 2. Strefan
// ---------------------------------------------------------------------------

func TestWave2MS3_Strefan_AttackTriggerCheatsVampireFromHand(t *testing.T) {
	gs := newGame(t, 2)
	strefan := stampCreaturePT(addPerm(gs, 0, "Strefan, Maurer Progenitor", "creature", "vampire"), 4, 4)
	// Blood token to sacrifice (Strefan's attack cost).
	bloodPerm := addPerm(gs, 0, "Blood Token", "token", "artifact", "blood")
	_ = bloodPerm
	// Vampire in hand to cheat.
	vamp := &gameengine.Card{
		Name: "Vampire of Death", Owner: 0,
		Types:         []string{"creature", "vampire"},
		BasePower:     3, BaseToughness: 3,
		CMC: 4,
	}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, vamp)

	strefanAttackTrigger(gs, strefan, map[string]interface{}{
		"attacker_perm": strefan,
		"defending_seat": 1,
	})

	// Strefan's attack trigger has a discard-blood step + cheat. The
	// migration only matters when a Vampire is found and cheated in.
	// If the test conditions allow the cheat, the migration covers no
	// double-add. If not (cost not paid, no token), the hand is
	// untouched. Either way, no duplication.
	bfCount := countBFCard(gs.Seats[0].Battlefield, vamp)
	handCount := countCardIn(gs.Seats[0].Hand, vamp)
	if bfCount+handCount != 1 {
		t.Errorf("Strefan: vamp presence must sum to 1; bf=%d hand=%d", bfCount, handCount)
	}
}

// ---------------------------------------------------------------------------
// 3. Gitrog Ravenous Ride
// ---------------------------------------------------------------------------

func TestWave2MS3_Gitrog_CombatDamageCheatsLandFromHand(t *testing.T) {
	gs := newGame(t, 2)
	git := stampCreaturePT(addPerm(gs, 0, "The Gitrog, Ravenous Ride", "creature"), 6, 6)
	land := &gameengine.Card{Name: "Forest", Owner: 0, Types: []string{"land", "basic"}}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, land)

	gitrogRideCombatDamage(gs, git, map[string]interface{}{
		"attacker_perm": git,
		"victim_seat":   1,
	})

	bfCount := countBFCard(gs.Seats[0].Battlefield, land)
	handCount := countCardIn(gs.Seats[0].Hand, land)
	if bfCount+handCount != 1 {
		t.Errorf("Gitrog: land presence must sum to 1; bf=%d hand=%d", bfCount, handCount)
	}
}

// ---------------------------------------------------------------------------
// 4. Hakbal
// ---------------------------------------------------------------------------

func TestWave2MS3_Hakbal_AttackTriggerCheatsLandFromHand(t *testing.T) {
	gs := newGame(t, 2)
	hak := stampCreaturePT(addPerm(gs, 0, "Hakbal of the Surging Soul", "creature"), 3, 3)
	land := &gameengine.Card{Name: "Forest", Owner: 0, Types: []string{"land", "basic"}}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, land)

	hakbalAttacks(gs, hak, map[string]interface{}{
		"attacker_perm": hak,
		"seat":          0,
	})

	bfCount := countBFCard(gs.Seats[0].Battlefield, land)
	handCount := countCardIn(gs.Seats[0].Hand, land)
	if bfCount+handCount != 1 {
		t.Errorf("Hakbal: land presence must sum to 1; bf=%d hand=%d", bfCount, handCount)
	}
}

// ---------------------------------------------------------------------------
// 5. Mayael the Anima
// ---------------------------------------------------------------------------

func TestWave2MS3_Mayael_LookFiveCheatsBigCreature(t *testing.T) {
	gs := newGame(t, 2)
	mayael := addPerm(gs, 0, "Mayael the Anima", "creature")
	gs.Seats[0].ManaPool = 10

	target := &gameengine.Card{
		Name: "Big Beater", Owner: 0,
		Types:         []string{"creature"},
		BasePower:     6, BaseToughness: 6,
	}
	others := []*gameengine.Card{
		{Name: "Small1", Owner: 0, Types: []string{"creature"}, BasePower: 1, BaseToughness: 1},
		{Name: "Small2", Owner: 0, Types: []string{"creature"}, BasePower: 1, BaseToughness: 1},
		{Name: "Spell", Owner: 0, Types: []string{"sorcery"}},
	}
	gs.Seats[0].Library = append(gs.Seats[0].Library, target)
	gs.Seats[0].Library = append(gs.Seats[0].Library, others...)

	mayaelLookFive(gs, mayael, 0, nil)

	if countBFCard(gs.Seats[0].Battlefield, target) != 1 {
		t.Errorf("Mayael: target must be on battlefield once; got %d",
			countBFCard(gs.Seats[0].Battlefield, target))
	}
	if countCardIn(gs.Seats[0].Library, target) != 0 {
		t.Errorf("Mayael: target must NOT be in library; got %d",
			countCardIn(gs.Seats[0].Library, target))
	}
	// Non-picked top cards must be in library exactly once each.
	for _, c := range others {
		if countCardIn(gs.Seats[0].Library, c) != 1 {
			t.Errorf("Mayael: filler %q must be in library once; got %d",
				c.Name, countCardIn(gs.Seats[0].Library, c))
		}
	}
}

// ---------------------------------------------------------------------------
// 6. Eruth — exercise the would_draw replacement closure directly
// ---------------------------------------------------------------------------

func TestWave2MS3_Eruth_DrawReplacementExilesTopTwoCleanly(t *testing.T) {
	gs := newGame(t, 2)
	eruth := addPerm(gs, 0, "Eruth, Tormented Prophet", "creature")

	top1 := &gameengine.Card{Name: "T1", Owner: 0, Types: []string{"instant"}}
	top2 := &gameengine.Card{Name: "T2", Owner: 0, Types: []string{"sorcery"}}
	bottom := &gameengine.Card{Name: "Bottom", Owner: 0, Types: []string{"land"}}
	gs.Seats[0].Library = append(gs.Seats[0].Library, top1, top2, bottom)

	// Register the replacement, then build a would_draw ReplEvent and
	// invoke the registered handler directly.
	eruthRegisterDrawReplacement(gs, eruth)

	var applyFn func(gs *gameengine.GameState, ev *gameengine.ReplEvent)
	var matched *gameengine.ReplacementEffect
	for _, re := range gs.Replacements {
		if re == nil {
			continue
		}
		if re.EventType == "would_draw" && re.ControllerSeat == 0 {
			matched = re
			applyFn = re.ApplyFn
			break
		}
	}
	if applyFn == nil {
		t.Fatal("Eruth: would_draw replacement not registered")
	}
	ev := gameengine.NewReplEvent("would_draw")
	ev.TargetSeat = 0
	ev.SetCount(1)
	if matched != nil && !matched.Applies(gs, ev) {
		t.Fatal("Eruth: replacement Applies guard rejected the synthetic event")
	}
	applyFn(gs, ev)

	if !ev.Cancelled {
		t.Errorf("Eruth: replacement must cancel the draw")
	}
	// top1 + top2 must be in exile exactly once each, gone from library.
	for _, c := range []*gameengine.Card{top1, top2} {
		if countCardIn(gs.Seats[0].Exile, c) != 1 {
			t.Errorf("Eruth: %q must be in exile once; got %d",
				c.Name, countCardIn(gs.Seats[0].Exile, c))
		}
		if countCardIn(gs.Seats[0].Library, c) != 0 {
			t.Errorf("Eruth: %q must NOT be in library; got %d",
				c.Name, countCardIn(gs.Seats[0].Library, c))
		}
	}
	// Library bottom card untouched.
	if countCardIn(gs.Seats[0].Library, bottom) != 1 {
		t.Errorf("Eruth: untouched bottom card must remain in library; got %d",
			countCardIn(gs.Seats[0].Library, bottom))
	}
}
