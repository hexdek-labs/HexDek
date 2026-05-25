package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// second_main_phase_r60_test.go — R60 Second Main Phase audit.
//
// Audit: ChooseCastFromHand never read gs.Phase, so it treated
// precombat_main and postcombat_main identically. Two missed
// signals:
//
//   Signal A (main2 deploy push): the archetype-based passBoost
//     represents "save mana for instants / hold the counterspell." In
//     postcombat_main the only window left for that mana is our own
//     end step + opp upkeep — most of the boost's intent (preserve
//     combat-window flexibility) is dead. Trim it so we actually
//     deploy threats before EOT instead of passing mana into the
//     void. Counter-hold passBoost stays (still useful for opp end-
//     step plays); only the archetype-stance component relaxes.
//
//   Signal B (main1 haste / attack-trigger boost): cards that
//     lose THIS-TURN value if they sit through combat — haste
//     creatures, attack-trigger commanders (Etali, Zur, Narset),
//     anthems, equipment — get a +0.15 heuristic in precombat_main
//     so the cast-vs-pass UCB tilts toward casting them BEFORE
//     combat. In postcombat_main the boost does nothing (already
//     too late for this turn's combat).

// hatForCastTest builds a YggdrasilHat wired with zero noise and a
// midrange strategy so the per-archetype passBoost lookup is stable.
func hatForCastTest(t *testing.T) *YggdrasilHat {
	t.Helper()
	return NewYggdrasilHatWithNoise(&StrategyProfile{Archetype: ArchetypeMidrange}, 0, 0)
}

// -----------------------------------------------------------------------------
// cardPrefersMain1 — unit
// -----------------------------------------------------------------------------

func TestCardPrefersMain1_HasteCreatureMatches(t *testing.T) {
	c := newTestCardMinimal("Goblin Guide", []string{"creature"}, 1,
		&gameast.CardAST{
			Name:      "Goblin Guide",
			Abilities: []gameast.Ability{&gameast.Static{Raw: "Haste"}},
		})
	if !cardPrefersMain1(c) {
		t.Errorf("haste creature should prefer main1")
	}
}

func TestCardPrefersMain1_AttackTriggerCommanderMatches(t *testing.T) {
	c := newTestCardMinimal("Etali, Primal Storm", []string{"creature", "legendary"}, 6,
		&gameast.CardAST{
			Name:      "Etali, Primal Storm",
			Abilities: []gameast.Ability{&gameast.Static{Raw: "Whenever Etali attacks, exile the top card of each player's library."}},
		})
	if !cardPrefersMain1(c) {
		t.Errorf("attack-trigger card should prefer main1 (Etali pattern)")
	}
}

func TestCardPrefersMain1_AnthemMatches(t *testing.T) {
	c := newTestCardMinimal("Glorious Anthem", []string{"enchantment"}, 3,
		&gameast.CardAST{
			Name:      "Glorious Anthem",
			Abilities: []gameast.Ability{&gameast.Static{Raw: "Creatures you control get +1/+1."}},
		})
	if !cardPrefersMain1(c) {
		t.Errorf("anthem (creatures you control get +X/+X) should prefer main1")
	}
}

func TestCardPrefersMain1_EquipmentMatches(t *testing.T) {
	c := newTestCardMinimal("Embercleave", []string{"artifact", "equipment"}, 6,
		&gameast.CardAST{
			Name:      "Embercleave",
			Abilities: []gameast.Ability{&gameast.Static{Raw: "Equipped creature gets +1/+1 and has double strike and trample."}},
		})
	if !cardPrefersMain1(c) {
		t.Errorf("equipment should prefer main1 so it can equip pre-combat")
	}
}

func TestCardPrefersMain1_VanillaCreatureDoesNotMatch(t *testing.T) {
	c := newTestCardMinimal("Grizzly Bears", []string{"creature"}, 2,
		&gameast.CardAST{Name: "Grizzly Bears"})
	if cardPrefersMain1(c) {
		t.Errorf("vanilla creature (no haste / no attack trigger / no anthem) should NOT prefer main1")
	}
}

func TestCardPrefersMain1_DrawSorceryDoesNotMatch(t *testing.T) {
	c := newTestCardMinimal("Divination", []string{"sorcery"}, 3,
		&gameast.CardAST{
			Name:      "Divination",
			Abilities: []gameast.Ability{&gameast.Static{Raw: "Draw two cards."}},
		})
	if cardPrefersMain1(c) {
		t.Errorf("draw sorcery has no combat-relevance; should NOT prefer main1")
	}
}

func TestCardPrefersMain1_NilSafe(t *testing.T) {
	if cardPrefersMain1(nil) {
		t.Errorf("nil card must return false")
	}
}

// -----------------------------------------------------------------------------
// Integration — main1 vs main2 dispatch shifts cast preference
// -----------------------------------------------------------------------------

// TestCardHeuristic_Main1BoostFiresForHaste pins the cardHeuristic
// adjustment directly: in precombat_main, a haste creature scores
// +0.30 above its postcombat_main baseline. Tests at the helper
// level so the assertion isn't sensitive to budget-mode dispatch
// (budget=0 routes through castHeuristic which picks by category
// order, not cardHeuristic score).
func TestCardHeuristic_Main1BoostFiresForHaste(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Turn = 5
	gs.Seats[0].ManaPool = 10

	haste := newTestCardMinimal("Goblin Guide", []string{"creature"}, 1,
		&gameast.CardAST{
			Name:      "Goblin Guide",
			Abilities: []gameast.Ability{&gameast.Static{Raw: "Haste."}},
		})

	h := hatForCastTest(t)

	gs.Phase = "postcombat_main"
	gs.Step = "postcombat_main"
	post := h.cardHeuristic(gs, 0, haste)

	gs.Phase = "precombat_main"
	gs.Step = "precombat_main"
	pre := h.cardHeuristic(gs, 0, haste)

	if pre <= post {
		t.Errorf("haste creature must score higher in precombat_main than postcombat_main; pre=%.3f post=%.3f",
			pre, post)
	}
	// Boost is documented at +0.30; allow a small float wobble.
	if delta := pre - post; delta < 0.25 || delta > 0.35 {
		t.Errorf("main1 boost should be ~0.30; got delta=%.3f (pre=%.3f, post=%.3f)",
			delta, pre, post)
	}
}

// TestCardHeuristic_Main1BoostDoesNotFireForVanilla pins the negative
// side: a vanilla creature with no main1-relevant signal gets the
// same heuristic regardless of phase.
func TestCardHeuristic_Main1BoostDoesNotFireForVanilla(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Turn = 5
	gs.Seats[0].ManaPool = 10

	vanilla := newTestCardMinimal("Grizzly Bears", []string{"creature"}, 2,
		&gameast.CardAST{Name: "Grizzly Bears"})

	h := hatForCastTest(t)

	gs.Phase = "postcombat_main"
	gs.Step = "postcombat_main"
	post := h.cardHeuristic(gs, 0, vanilla)

	gs.Phase = "precombat_main"
	gs.Step = "precombat_main"
	pre := h.cardHeuristic(gs, 0, vanilla)

	if pre != post {
		t.Errorf("vanilla creature should not receive a main1 boost; pre=%.3f post=%.3f",
			pre, post)
	}
}

// TestChooseCastFromHand_Main2DeployPushReducesPass pins Signal A:
// in postcombat_main, with the same hand (only a non-combat-relevant
// draw sorcery available), the main2 deploy push trims the
// archetype pass boost enough that the hat casts the spell rather
// than passing mana into the void.
func TestChooseCastFromHand_Main2DeployPushReducesPass(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Phase = "postcombat_main"
	gs.Step = "postcombat_main"
	gs.Seats[0].ManaPool = 10

	draw := newTestCardMinimal("Divination", []string{"sorcery"}, 3,
		&gameast.CardAST{
			Name:      "Divination",
			Abilities: []gameast.Ability{&gameast.Static{Raw: "Draw two cards."}},
		})
	gs.Seats[0].Hand = []*gameengine.Card{draw}
	gs.Seats[0].Hat = hatForCastTest(t)

	pick := gs.Seats[0].Hat.ChooseCastFromHand(gs, 0, gs.Seats[0].Hand)
	if pick == nil {
		t.Errorf("main2 deploy push should encourage casting an affordable spell rather than passing mana to EOT")
	}
}

// TestChooseCastFromHand_HasteBoostDoesNotFireInMain2 pins the
// negative side of Signal B: the +0.15 haste/attack-trigger boost
// only applies in precombat_main. In postcombat_main the boost is
// gated off (already too late for this turn's combat), so the
// non-haste path is the same.
func TestChooseCastFromHand_HasteBoostScopedToMain1(t *testing.T) {
	// Set up an isolated game in postcombat_main with two creatures
	// in hand: haste vs vanilla. The boost should NOT fire, so the
	// pick reflects baseline heuristics (which may prefer either —
	// the assertion is on the BOOST being absent, not on a specific
	// pick). We verify this by checking the cardPrefersMain1 path
	// produces the same heuristic regardless of phase via direct
	// helper assertion above; the integration smoke test below just
	// verifies the dispatch doesn't error in main2.
	gs := newTestGame(t, 2)
	gs.Phase = "postcombat_main"
	gs.Step = "postcombat_main"
	gs.Seats[0].ManaPool = 10

	haste := newTestCardMinimal("Goblin Guide", []string{"creature"}, 1,
		&gameast.CardAST{
			Name:      "Goblin Guide",
			Abilities: []gameast.Ability{&gameast.Static{Raw: "Haste."}},
		})
	gs.Seats[0].Hand = []*gameengine.Card{haste}
	gs.Seats[0].Hat = hatForCastTest(t)

	// Just verifies no panic and a sane pick. The detailed boost
	// gating is asserted at the cardPrefersMain1 helper level above.
	pick := gs.Seats[0].Hat.ChooseCastFromHand(gs, 0, gs.Seats[0].Hand)
	if pick != nil && pick.DisplayName() != "Goblin Guide" {
		t.Errorf("unexpected pick in main2 single-card test: %s", pick.DisplayName())
	}
}
