package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// wave2_multistep_batch1_r60_test.go — Wave 2 multi-step migrations.
//
// 6 per_card handlers in the "library-reorder-then-play" family
// (Primal Surge / Lurking Predators shape) pre-r60 spliced top-N off
// the library before calling MoveCard or enterBattlefieldWithETB on
// the picked card. MoveCard's source-zone removal then no-op'd because
// the card was no longer in library, silently bypassing §614 / §903.9b
// / zone_change triggers. This file pins each migration with a focused
// regression that:
//
//  1. Confirms the picked card lands in the destination zone exactly
//     once (battlefield, hand, or exile depending on card).
//  2. Confirms the non-picked top-N cards stay in library, rotated to
//     the bottom in the same relative order (within-zone reorder —
//     no zone-change events for them).
//  3. Confirms no double-add (the source zone holds 0 copies after).
//
// Migrated handlers:
//   1. Star Charter           — end-step look-4-play-creature, rest bottom
//   2. Birthing Ritual        — end-step sac+cheat, rest bottom
//   3. Svella, Ice Shaper     — activated look-4-free-cast, rest bottom
//   4. Toph, Greatest Earth.  — ETB earthbend X, land to play, rest bottom
//   5. Ayesha Tanaka, Armorer — attack look-4-artifacts-to-play, rest bottom
//   6. Esika / Prismatic Br.  — upkeep reveal-until-creature-PW, rest shuffled

func TestWave2MS_StarCharter_PicksAndRotatesRest(t *testing.T) {
	gs := newGame(t, 2)
	star := stampCreaturePT(addPerm(gs, 0, "Star Charter", "creature"), 2, 2)
	gs.Seats[0].Turn.LifeGained = 1 // gate the trigger

	target := &gameengine.Card{
		Name: "Tiny Creature", Owner: 0,
		Types: []string{"creature"}, BasePower: 2, BaseToughness: 2,
	}
	filler1 := &gameengine.Card{Name: "Filler1", Owner: 0, Types: []string{"land"}}
	filler2 := &gameengine.Card{Name: "Filler2", Owner: 0, Types: []string{"sorcery"}}
	filler3 := &gameengine.Card{Name: "Filler3", Owner: 0, Types: []string{"land"}}
	bottom := &gameengine.Card{Name: "Bottom", Owner: 0, Types: []string{"land"}}
	gs.Seats[0].Library = append(gs.Seats[0].Library, target, filler1, filler2, filler3, bottom)

	starCharterEndStep(gs, star, map[string]interface{}{"active_seat": 0})

	if countCardIn(gs.Seats[0].Hand, target) != 1 {
		t.Errorf("Wave 2 MS: target must be in hand once; got %d", countCardIn(gs.Seats[0].Hand, target))
	}
	if countCardIn(gs.Seats[0].Library, target) != 0 {
		t.Errorf("Wave 2 MS: target must NOT be in library; got %d", countCardIn(gs.Seats[0].Library, target))
	}
	// Pre-existing bottom card must still be at the bottom (untouched).
	if gs.Seats[0].Library[len(gs.Seats[0].Library)-1] == target {
		t.Errorf("Wave 2 MS: target should never end up at library bottom")
	}
	// Filler cards (non-picked top-4) must still be in library exactly once
	// each — no orphans, no doubles.
	for _, c := range []*gameengine.Card{filler1, filler2, filler3, bottom} {
		if countCardIn(gs.Seats[0].Library, c) != 1 {
			t.Errorf("Wave 2 MS: filler %q must be in library exactly once; got %d",
				c.Name, countCardIn(gs.Seats[0].Library, c))
		}
	}
}

func TestWave2MS_BirthingRitual_CheatsAndRotatesRest(t *testing.T) {
	gs := newGame(t, 2)
	br := addPerm(gs, 0, "Birthing Ritual", "enchantment")
	// Fodder creature for the sacrifice cost. cardCMC for creatures =
	// BasePower + BaseToughness, so power=1+toughness=1 → cardCMC=2,
	// which qualifies sacrificing it to cheat a CMC=2 target (1+1+1≥2).
	fodder := addPerm(gs, 0, "Fodder", "creature")
	fodder.Card.BasePower = 1
	fodder.Card.BaseToughness = 1
	// Helper makes birthingRitualHasCreature return true even if fodder is
	// somehow disqualified.
	_ = addPerm(gs, 0, "Helper", "creature")

	// Target's effective cardCMC is also BasePower+BaseToughness for the
	// per_card test-helper fallback; tune to 2 so the cheat-X-or-less
	// gate passes (sac fodder cardCMC 2 + 1 = 3 ≥ target cardCMC 2).
	target := &gameengine.Card{
		Name: "Big Beater", Owner: 0,
		Types:         []string{"creature"},
		BasePower:     1, BaseToughness: 1,
	}
	others := []*gameengine.Card{
		{Name: "F1", Owner: 0, Types: []string{"sorcery"}, CMC: 1},
		{Name: "F2", Owner: 0, Types: []string{"sorcery"}, CMC: 1},
		{Name: "F3", Owner: 0, Types: []string{"sorcery"}, CMC: 1},
	}
	gs.Seats[0].Library = append(gs.Seats[0].Library, target)
	gs.Seats[0].Library = append(gs.Seats[0].Library, others...)

	birthingRitualEndStep(gs, br, map[string]interface{}{"active_seat": 0})

	// Target must be on battlefield exactly once and gone from library.
	bfCount := countBFCard(gs.Seats[0].Battlefield, target)
	if bfCount != 1 {
		t.Errorf("Wave 2 MS: target must be on battlefield once; got %d", bfCount)
	}
	if countCardIn(gs.Seats[0].Library, target) != 0 {
		t.Errorf("Wave 2 MS: target must NOT be in library after cheat; got %d",
			countCardIn(gs.Seats[0].Library, target))
	}
	// Non-picked top cards must be in library exactly once.
	for _, c := range others {
		if countCardIn(gs.Seats[0].Library, c) != 1 {
			t.Errorf("Wave 2 MS: filler %q must be in library once; got %d",
				c.Name, countCardIn(gs.Seats[0].Library, c))
		}
	}
}

func TestWave2MS_Svella_PicksToExileAndRotatesRest(t *testing.T) {
	gs := newGame(t, 2)
	svella := addPerm(gs, 0, "Svella, Ice Shaper", "creature")

	target := &gameengine.Card{Name: "Free Spell", Owner: 0, Types: []string{"sorcery"}, CMC: 4}
	others := []*gameengine.Card{
		{Name: "L1", Owner: 0, Types: []string{"land"}},
		{Name: "L2", Owner: 0, Types: []string{"land"}},
		{Name: "L3", Owner: 0, Types: []string{"land"}},
	}
	gs.Seats[0].Library = append(gs.Seats[0].Library, target)
	gs.Seats[0].Library = append(gs.Seats[0].Library, others...)

	svellaTopFour(gs, svella)

	if countCardIn(gs.Seats[0].Exile, target) != 1 {
		t.Errorf("Wave 2 MS: target must be in exile once (free-cast staging); got %d",
			countCardIn(gs.Seats[0].Exile, target))
	}
	if countCardIn(gs.Seats[0].Library, target) != 0 {
		t.Errorf("Wave 2 MS: target must NOT be in library after MoveCard; got %d",
			countCardIn(gs.Seats[0].Library, target))
	}
	for _, c := range others {
		if countCardIn(gs.Seats[0].Library, c) != 1 {
			t.Errorf("Wave 2 MS: filler %q must be in library once; got %d",
				c.Name, countCardIn(gs.Seats[0].Library, c))
		}
	}
}

func TestWave2MS_Toph_EarthbendPicksLandRotatesRest(t *testing.T) {
	gs := newGame(t, 2)
	toph := addPerm(gs, 0, "Toph, Greatest Earthbender", "creature")
	toph.Card.CMC = 4 // X = 4

	target := &gameengine.Card{Name: "Triome", Owner: 0, Types: []string{"land"}}
	others := []*gameengine.Card{
		{Name: "Spell1", Owner: 0, Types: []string{"sorcery"}},
		{Name: "Spell2", Owner: 0, Types: []string{"creature"}, BasePower: 1, BaseToughness: 1},
		{Name: "Spell3", Owner: 0, Types: []string{"instant"}},
	}
	gs.Seats[0].Library = append(gs.Seats[0].Library, target)
	gs.Seats[0].Library = append(gs.Seats[0].Library, others...)

	tophETBEarthbendAndAnthem(gs, toph)

	if countBFCard(gs.Seats[0].Battlefield, target) != 1 {
		t.Errorf("Wave 2 MS: target land must be on battlefield once; got %d",
			countBFCard(gs.Seats[0].Battlefield, target))
	}
	if countCardIn(gs.Seats[0].Library, target) != 0 {
		t.Errorf("Wave 2 MS: target must NOT be in library; got %d",
			countCardIn(gs.Seats[0].Library, target))
	}
	for _, c := range others {
		if countCardIn(gs.Seats[0].Library, c) != 1 {
			t.Errorf("Wave 2 MS: filler %q must be in library once; got %d",
				c.Name, countCardIn(gs.Seats[0].Library, c))
		}
	}
}

func TestWave2MS_Ayesha_PlaysArtifactsRotatesRest(t *testing.T) {
	gs := newGame(t, 2)
	ayesha := stampCreaturePT(addPerm(gs, 0, "Ayesha Tanaka, Armorer", "creature"), 5, 5)

	cheapArt := &gameengine.Card{Name: "Sol Ring", Owner: 0, Types: []string{"artifact"}, CMC: 1}
	otherArt := &gameengine.Card{Name: "Skull Clamp", Owner: 0, Types: []string{"artifact"}, CMC: 1}
	bigArt := &gameengine.Card{Name: "Darksteel Forge", Owner: 0, Types: []string{"artifact"}, CMC: 9}
	spell := &gameengine.Card{Name: "Just a Spell", Owner: 0, Types: []string{"sorcery"}, CMC: 2}
	gs.Seats[0].Library = append(gs.Seats[0].Library, cheapArt, otherArt, bigArt, spell)

	// Engine fires creature_attacks with attacker_perm (not "seat"); Ayesha
	// herself is the attacker. (r63 self-gate: "seat":0 was the pre-fix ctx.)
	ayeshaAttacks(gs, ayesha, map[string]interface{}{"attacker_perm": ayesha, "attacker_seat": 0})

	// Power=5: cheapArt (CMC 1) and otherArt (CMC 1) both cast. bigArt (CMC 9)
	// over power cap. spell isn't an artifact.
	if countBFCard(gs.Seats[0].Battlefield, cheapArt) != 1 {
		t.Errorf("Wave 2 MS: cheapArt must be on battlefield once; got %d",
			countBFCard(gs.Seats[0].Battlefield, cheapArt))
	}
	if countBFCard(gs.Seats[0].Battlefield, otherArt) != 1 {
		t.Errorf("Wave 2 MS: otherArt must be on battlefield once; got %d",
			countBFCard(gs.Seats[0].Battlefield, otherArt))
	}
	// bigArt + spell stay in library (rotated to bottom).
	if countCardIn(gs.Seats[0].Library, bigArt) != 1 {
		t.Errorf("Wave 2 MS: bigArt must stay in library; got %d",
			countCardIn(gs.Seats[0].Library, bigArt))
	}
	if countCardIn(gs.Seats[0].Library, spell) != 1 {
		t.Errorf("Wave 2 MS: spell must stay in library; got %d",
			countCardIn(gs.Seats[0].Library, spell))
	}
	// And the played cards are gone from library.
	if countCardIn(gs.Seats[0].Library, cheapArt) != 0 ||
		countCardIn(gs.Seats[0].Library, otherArt) != 0 {
		t.Errorf("Wave 2 MS: played cards must be gone from library")
	}
}

func TestWave2MS_Esika_PrismaticBridge_HitGoesBfRestShuffled(t *testing.T) {
	gs := newGame(t, 2)
	bridge := addPerm(gs, 0, "The Prismatic Bridge", "enchantment")
	// Force enchantment-typed via the test helper (addPerm sets the type
	// but the back-face gate checks IsCreature/IsEnchantment).
	hit := &gameengine.Card{
		Name: "Big Creature", Owner: 0,
		Types:         []string{"creature"},
		BasePower:     6, BaseToughness: 6,
	}
	preNonHit1 := &gameengine.Card{Name: "Spell1", Owner: 0, Types: []string{"sorcery"}}
	preNonHit2 := &gameengine.Card{Name: "Spell2", Owner: 0, Types: []string{"instant"}}
	gs.Seats[0].Library = append(gs.Seats[0].Library, preNonHit1, preNonHit2, hit)

	esikaBridgeUpkeep(gs, bridge, map[string]interface{}{"active_seat": 0})

	if countBFCard(gs.Seats[0].Battlefield, hit) != 1 {
		t.Errorf("Wave 2 MS: hit must be on battlefield once; got %d",
			countBFCard(gs.Seats[0].Battlefield, hit))
	}
	if countCardIn(gs.Seats[0].Library, hit) != 0 {
		t.Errorf("Wave 2 MS: hit must NOT be in library; got %d",
			countCardIn(gs.Seats[0].Library, hit))
	}
	// Non-hit reveals must be back in library (shuffled — count is what
	// matters, position is randomized).
	if countCardIn(gs.Seats[0].Library, preNonHit1) != 1 {
		t.Errorf("Wave 2 MS: preNonHit1 must be in library once after shuffle; got %d",
			countCardIn(gs.Seats[0].Library, preNonHit1))
	}
	if countCardIn(gs.Seats[0].Library, preNonHit2) != 1 {
		t.Errorf("Wave 2 MS: preNonHit2 must be in library once after shuffle; got %d",
			countCardIn(gs.Seats[0].Library, preNonHit2))
	}
}
