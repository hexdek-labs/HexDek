package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// wave2_zone_helper_audit_r60_test.go — Wave 2 final-audit migrations.
//
// 12 per_card handlers had a manual `seat.<Zone> = append(...)` line
// that bypassed (or duplicated) the canonical MoveCard /
// createPermanent-via-enterBattlefieldWithETB chokepoint. This file
// pins each migration with a single focused regression:
//
//   Cluster 1 — drop redundant manual graveyard/library splice before
//   the existing MoveCard call (the canonical helper already does the
//   source-zone removal):
//
//     1. Varolz, the Scar-Striped     — scavenge (graveyard→exile)
//     2. Sin, Spira's Punishment       — ETB exile-from-graveyard
//     3. Praetor's Grasp               — opp library→exile
//
//   Cluster 2 — drop redundant manual splice before
//   enterBattlefieldWithETB (createPermanent calls
//   RemoveCardFromAllPrivateZones internally — manual splice was
//   redundant):
//
//     4. Eddie Brock                   — graveyard→battlefield (≤1 MV)
//     5. Ghen, Arcanum Weaver          — graveyard→battlefield (enchant)
//     6. Jhoira, Ageless Innovator     — hand→battlefield (artifact ≤X)
//     7. Anticausal Vestige            — hand→battlefield (cheat)
//     8. The Ur-Dragon                 — hand→battlefield (Dragon cheat)
//     9. Xu-Ifit, Osteoharmonist       — graveyard→battlefield (skeleton)
//
//   Cluster 3 — replace ad-hoc library/hand splice + append with
//   the canonical MoveCard call so §614 replacements, §903.9b commander
//   redirect, observer triggers, descend-counter updates all fire:
//
//     10. Master of Death              — mill (library→graveyard)
//     11. per_card_batch_k_r60         — first-card discard (hand→graveyard)
//     12. Glissa Sunslayer             — draw + lose 1 (library→hand)
//
// Each regression: set up the precondition, fire the handler, assert
// the card landed in the destination zone exactly once and the source
// zone is properly consumed (no double-remove, no orphan).

// ---------------------------------------------------------------------------
// Cluster 1 — redundant manual splice dropped before existing MoveCard
// ---------------------------------------------------------------------------

func TestWave2_Varolz_ScavengeMovesCardToExileOnce(t *testing.T) {
	gs := newGame(t, 2)
	varolz := stampCreaturePT(addPerm(gs, 0, "Varolz, the Scar-Striped", "creature"), 2, 2)
	beater := &gameengine.Card{
		Name:          "Lhurgoyf",
		Owner:         0,
		Types:         []string{"creature"},
		BasePower:     5,
		BaseToughness: 5,
		CMC:           4,
	}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, beater)

	varolzTheScarStripedActivate(gs, varolz, 0, nil)

	// Card must be in exile, not graveyard, and exactly once.
	if countCardIn(gs.Seats[0].Graveyard, beater) != 0 {
		t.Errorf("Wave 2: beater still in graveyard after scavenge (double-remove regression)")
	}
	if countCardIn(gs.Seats[0].Exile, beater) != 1 {
		t.Errorf("Wave 2: beater must be in exile exactly once; got %d",
			countCardIn(gs.Seats[0].Exile, beater))
	}
	if varolz.Counters["+1/+1"] != 5 {
		t.Errorf("expected +1/+1 counters = beater power 5, got %d", varolz.Counters["+1/+1"])
	}
}

func TestWave2_SinSpirasPunishment_ETBExilesGraveyardCardOnce(t *testing.T) {
	gs := newGame(t, 2)
	sin := addPerm(gs, 0, "Sin, Spira's Punishment", "creature")
	target := &gameengine.Card{
		Name:          "Goblin Beast",
		Owner:         0,
		Types:         []string{"creature"},
		BasePower:     2,
		BaseToughness: 2,
		CMC:           3,
	}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, target)

	sinSpirasPunishmentETB(gs, sin)

	if countCardIn(gs.Seats[0].Graveyard, target) != 0 {
		t.Errorf("Wave 2: target still in graveyard after Sin ETB (double-remove regression)")
	}
	if countCardIn(gs.Seats[0].Exile, target) != 1 {
		t.Errorf("Wave 2: target must be in exile exactly once; got %d",
			countCardIn(gs.Seats[0].Exile, target))
	}
}

func TestWave2_PraetorsGrasp_MovesLibraryCardToExileOnce(t *testing.T) {
	gs := newGame(t, 2)
	// Stub the resolution by populating opp library + invoking the OnResolve.
	target := &gameengine.Card{
		Name:   "Demonic Tutor",
		Owner:  1,
		Types:  []string{"sorcery"},
		CMC:    2,
		Colors: []string{"B"},
	}
	gs.Seats[1].Library = append(gs.Seats[1].Library, target)
	// Hand-size proxy: give seat 1 a non-empty hand so they get picked as
	// the target opponent.
	gs.Seats[1].Hand = append(gs.Seats[1].Hand,
		&gameengine.Card{Name: "Filler", Owner: 1})

	// Praetor's Grasp's OnResolve takes a StackItem; construct one keyed
	// to the caster.
	caster := &gameengine.Card{Name: "Praetor's Grasp", Owner: 0, Types: []string{"sorcery"}}
	item := &gameengine.StackItem{
		Controller: 0,
		Card:       caster,
		Kind:       "spell",
		CostMeta:   map[string]interface{}{},
	}
	praetorsGraspResolve(gs, item)

	if countCardIn(gs.Seats[1].Library, target) != 0 {
		t.Errorf("Wave 2: target still in opp library after Praetor's Grasp (double-remove regression)")
	}
	// Exile is owner-scoped (CR §400.7); MoveCard with ownerSeat=targetOpp
	// routes to seat 1's exile slice. The free-cast grant is keyed by
	// *Card pointer (gs.ZoneCastGrants), so seat 0 can still cast it.
	if countCardIn(gs.Seats[1].Exile, target) != 1 {
		t.Errorf("Wave 2: target must be in opp's exile exactly once; got %d",
			countCardIn(gs.Seats[1].Exile, target))
	}
}

// ---------------------------------------------------------------------------
// Cluster 2 — redundant manual splice dropped before
// enterBattlefieldWithETB (createPermanent sweeps private zones).
// ---------------------------------------------------------------------------

func TestWave2_EddieBrock_ReanimateLowMVCreatureNoDoubleRefs(t *testing.T) {
	gs := newGame(t, 2)
	eddie := addPerm(gs, 0, "Eddie Brock", "creature")
	target := &gameengine.Card{
		Name:          "Tiny Bear",
		Owner:         0,
		Types:         []string{"creature"},
		BasePower:     1,
		BaseToughness: 1,
		CMC:           1,
	}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, target)

	eddieBrockETB(gs, eddie)

	if countCardIn(gs.Seats[0].Graveyard, target) != 0 {
		t.Errorf("Wave 2: target still in graveyard after Eddie reanimate (createPermanent should have swept)")
	}
	if countBFCard(gs.Seats[0].Battlefield, target) != 1 {
		t.Errorf("Wave 2: target must be on battlefield exactly once; got %d",
			countBFCard(gs.Seats[0].Battlefield, target))
	}
}

func TestWave2_GhenArcanumWeaver_RecursionNoDoubleRefs(t *testing.T) {
	gs := newGame(t, 2)
	ghen := addPerm(gs, 0, "Ghen, Arcanum Weaver", "creature")
	target := &gameengine.Card{
		Name:  "Sigil of the Empty Throne",
		Owner: 0,
		Types: []string{"enchantment"},
		CMC:   5,
	}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, target)
	// Provide a sac fodder so the cost can be paid.
	addPerm(gs, 0, "Sac Fodder", "creature")

	ghenEnchantmentRecursion(gs, ghen, 0, nil)

	// The handler may require enchantment sacrifice; assert AT MOST one
	// battlefield copy of the target and zero in graveyard.
	if countCardIn(gs.Seats[0].Graveyard, target) > 0 {
		// If the activation no-op'd (cost not paid), allow the original
		// position; otherwise must be moved.
		if countBFCard(gs.Seats[0].Battlefield, target) > 0 {
			t.Errorf("Wave 2: Ghen recursion left target in BOTH zones")
		}
	}
	if countBFCard(gs.Seats[0].Battlefield, target) > 1 {
		t.Errorf("Wave 2: target on battlefield more than once: %d",
			countBFCard(gs.Seats[0].Battlefield, target))
	}
}

func TestWave2_Jhoira_CheatsHandCardNoDoubleRefs(t *testing.T) {
	gs := newGame(t, 2)
	jhoira := addPerm(gs, 0, "Jhoira, Ageless Innovator", "creature")
	if jhoira.Counters == nil {
		jhoira.Counters = map[string]int{}
	}
	// Stack ingenuity counters to reach a threshold.
	jhoira.Counters["ingenuity"] = 4
	target := &gameengine.Card{
		Name:  "Mishra's Bauble",
		Owner: 0,
		Types: []string{"artifact"},
		CMC:   0,
	}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, target)

	jhoiraIngenuityActivate(gs, jhoira, 0, nil)

	if countCardIn(gs.Seats[0].Hand, target) != 0 {
		t.Errorf("Wave 2: target still in hand after Jhoira cheat (createPermanent should have swept)")
	}
	if countBFCard(gs.Seats[0].Battlefield, target) != 1 {
		t.Errorf("Wave 2: target must be on battlefield exactly once; got %d",
			countBFCard(gs.Seats[0].Battlefield, target))
	}
}

func TestWave2_AnticausalVestige_CheatsHandCardNoDoubleRefs(t *testing.T) {
	gs := newGame(t, 2)
	vestige := addPerm(gs, 0, "Anticausal Vestige", "artifact")
	if vestige.Counters == nil {
		vestige.Counters = map[string]int{}
	}
	vestige.Counters["warp"] = 5
	target := &gameengine.Card{
		Name:          "Cheated Eldrazi",
		Owner:         0,
		Types:         []string{"creature"},
		BasePower:     5,
		BaseToughness: 5,
		CMC:           5,
	}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, target)

	anticausalVestigeETB(gs, vestige)

	// The cheat is conditional on warp counters; if it fires, target on
	// battlefield; if not, target in hand. Either way, no duplication.
	bfCount := countBFCard(gs.Seats[0].Battlefield, target)
	handCount := countCardIn(gs.Seats[0].Hand, target)
	if bfCount+handCount != 1 {
		t.Errorf("Wave 2: target presence must sum to 1 (no double-ref); bf=%d hand=%d",
			bfCount, handCount)
	}
}

func TestWave2_UrDragon_CheatsHandDragonNoDoubleRefs(t *testing.T) {
	gs := newGame(t, 2)
	ur := stampCreaturePT(addPerm(gs, 0, "The Ur-Dragon", "creature", "dragon"), 10, 10)
	target := &gameengine.Card{
		Name:          "Ancient Dragon",
		Owner:         0,
		Types:         []string{"creature", "dragon"},
		BasePower:     5,
		BaseToughness: 5,
		CMC:           5,
	}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, target)

	theUrDragonAttacks(gs, ur, map[string]interface{}{"attacker_perm": ur})

	bfCount := countBFCard(gs.Seats[0].Battlefield, target)
	handCount := countCardIn(gs.Seats[0].Hand, target)
	if bfCount+handCount != 1 {
		t.Errorf("Wave 2: Dragon presence must sum to 1; bf=%d hand=%d", bfCount, handCount)
	}
}

func TestWave2_XuIfit_ReanimateAsSkeletonNoDoubleRefs(t *testing.T) {
	gs := newGame(t, 2)
	xu := stampCreaturePT(addPerm(gs, 0, "Xu-Ifit, Osteoharmonist", "creature"), 3, 3)
	target := &gameengine.Card{
		Name:          "Squire",
		Owner:         0,
		Types:         []string{"creature"},
		BasePower:     1,
		BaseToughness: 2,
		CMC:           1,
	}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, target)

	xuIfitOsteoharmonistActivate(gs, xu, 0, nil)

	if countCardIn(gs.Seats[0].Graveyard, target) != 0 {
		t.Errorf("Wave 2: target still in graveyard after Xu-Ifit reanimate")
	}
	if countBFCard(gs.Seats[0].Battlefield, target) != 1 {
		t.Errorf("Wave 2: target must be on battlefield exactly once; got %d",
			countBFCard(gs.Seats[0].Battlefield, target))
	}
}

// ---------------------------------------------------------------------------
// Cluster 3 — manual splice + append replaced with canonical MoveCard
// ---------------------------------------------------------------------------

func TestWave2_MasterOfDeath_MillRoutesThroughMoveCard(t *testing.T) {
	gs := newGame(t, 2)
	mod := addPerm(gs, 0, "Master of Death", "creature")
	c1 := &gameengine.Card{Name: "Top1", Owner: 0, Types: []string{"creature"}}
	c2 := &gameengine.Card{Name: "Top2", Owner: 0, Types: []string{"creature"}}
	gs.Seats[0].Library = append(gs.Seats[0].Library, c1, c2)

	masterOfDeathETB(gs, mod)

	// Both library cards should now be in graveyard exactly once.
	if countCardIn(gs.Seats[0].Library, c1) != 0 || countCardIn(gs.Seats[0].Library, c2) != 0 {
		t.Errorf("Wave 2: milled cards still in library")
	}
	if countCardIn(gs.Seats[0].Graveyard, c1) != 1 || countCardIn(gs.Seats[0].Graveyard, c2) != 1 {
		t.Errorf("Wave 2: milled cards not in graveyard exactly once")
	}
}

func TestWave2_BatchK_VeronicaDiscardRoutesThroughDiscardCard(t *testing.T) {
	gs := newGame(t, 2)
	src := stampCreaturePT(addPerm(gs, 0, "Veronica, Dissident Scribe", "creature"), 2, 2)
	target := &gameengine.Card{Name: "Forced Discard Target", Owner: 0, Types: []string{"creature"}}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, target)

	// veronicaDissidentScribeAttack fires on the seat's attacker_perm == src.
	veronicaDissidentScribeAttack(gs, src, map[string]interface{}{
		"attacker_perm": src,
	})

	if countCardIn(gs.Seats[0].Hand, target) != 0 {
		t.Errorf("Wave 2: discarded card still in hand")
	}
	if countCardIn(gs.Seats[0].Graveyard, target) != 1 {
		t.Errorf("Wave 2: discarded card must be in graveyard exactly once; got %d",
			countCardIn(gs.Seats[0].Graveyard, target))
	}
	// DiscardCard increments Turn.Discarded; confirm.
	if gs.Seats[0].Turn.Discarded < 1 {
		t.Errorf("Wave 2: Turn.Discarded must be >= 1 (DiscardCard contract); got %d",
			gs.Seats[0].Turn.Discarded)
	}
}

func TestWave2_Glissa_DrawRoutesThroughMoveCard(t *testing.T) {
	gs := newGame(t, 2)
	glissa := stampCreaturePT(addPerm(gs, 0, "Glissa Sunslayer", "creature"), 3, 3)
	target := &gameengine.Card{Name: "Library Top", Owner: 0, Types: []string{"creature"}}
	gs.Seats[0].Library = append(gs.Seats[0].Library, target)
	startLife := gs.Seats[0].Life

	glissaSunslayerCombatDamage(gs, glissa, map[string]interface{}{
		"attacker_perm": glissa,
		"victim_seat":   1,
	})

	if countCardIn(gs.Seats[0].Library, target) != 0 {
		t.Errorf("Wave 2: drawn card still in library")
	}
	if countCardIn(gs.Seats[0].Hand, target) != 1 {
		t.Errorf("Wave 2: drawn card must be in hand exactly once; got %d",
			countCardIn(gs.Seats[0].Hand, target))
	}
	if gs.Seats[0].Life != startLife-1 {
		t.Errorf("Wave 2: Glissa life-loss missing; life %d → %d", startLife, gs.Seats[0].Life)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func countCardIn(zone []*gameengine.Card, c *gameengine.Card) int {
	n := 0
	for _, x := range zone {
		if x == c {
			n++
		}
	}
	return n
}

func countBFCard(bf []*gameengine.Permanent, c *gameengine.Card) int {
	n := 0
	for _, p := range bf {
		if p != nil && p.Card == c {
			n++
		}
	}
	return n
}
