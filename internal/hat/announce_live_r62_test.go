package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// announce_live_r62_test.go — r62 keystone integration regression.
//
// The r61 review (reports 04 + 05) found that PR-7/PR-8's hat targeting was
// INERT live: the engine picked targets lazily at resolution via PickTarget
// and never called hat.ChooseTarget, so the beneficial-target classifier was
// dead code and every pump spell buffed the strongest ENEMY creature. These
// tests drive a REAL cast — gameengine.CastSpell with nil targets, exactly
// what the tournament turn runner does — with a production YggdrasilHat on
// the seat, and assert the spell lands where the hat aims it. This is the
// engine+hat seam test report 04 said would have caught the gap.

// castFromHand wires a spell card (Activated-with-empty-cost spell body)
// into seatIdx's hand and casts it through the live engine path.
func castFromHand(t *testing.T, gs *gameengine.GameState, seatIdx int, name string, cost int, eff gameast.Effect) {
	t.Helper()
	card := newTestCardMinimal(name, []string{"instant"}, cost, &gameast.CardAST{
		Name:      name,
		Abilities: []gameast.Ability{&gameast.Activated{Effect: eff}},
	})
	card.Owner = seatIdx
	gs.Seats[seatIdx].Hand = append(gs.Seats[seatIdx].Hand, card)
	gs.Seats[seatIdx].ManaPool = cost
	if err := gameengine.CastSpell(gs, seatIdx, card, nil); err != nil {
		t.Fatalf("CastSpell(%s) failed: %v", name, err)
	}
}

// TestLive_BeneficialPumpTargetsOwnCreature: the headline regression. A
// "+3/+3 to target creature" cast through the live path (nil targets,
// Yggdrasil hat) must land on the AI's OWN creature, not the strongest
// enemy — proving announcement-time hat consultation AND resolution-time
// honoring of the announced target.
func TestLive_BeneficialPumpTargetsOwnCreature(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Active = 0
	gs.Seats[0].Life = 40
	gs.Seats[1].Life = 40

	own := mkTargetCreature(gs, 0, "My Bear", 2, 2, nil, "")
	enemy := mkTargetCreature(gs, 1, "Their Dragon", 6, 6, nil, "")

	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 0)
	h.Noise = 0
	gs.Seats[0].Hat = h

	castFromHand(t, gs, 0, "Giant Growth", 1, &gameast.Buff{
		Power: 3, Toughness: 3,
		Target: gameast.Filter{Base: "creature", Targeted: true},
	})

	if got := gs.PowerOf(own); got != 5 {
		t.Fatalf("live-path pump missed the AI's own creature: My Bear power=%d want 5", got)
	}
	if got := gs.PowerOf(enemy); got != 6 {
		t.Fatalf("live-path pump buffed the ENEMY creature: Their Dragon power=%d want 6 (the pre-r62 inert-hat bug)", got)
	}
}

// TestLive_RemovalTargetsEnemyCreature: the harmful polarity through the
// same live path — removal must still kill the enemy threat and never the
// AI's own creature (guards the PR-7 reactive-removal reliance on
// best-enemy targeting).
func TestLive_RemovalTargetsEnemyCreature(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Active = 0
	gs.Seats[0].Life = 40
	gs.Seats[1].Life = 40

	own := mkTargetCreature(gs, 0, "My Bear", 2, 2, nil, "")
	enemy := mkTargetCreature(gs, 1, "Their Dragon", 6, 6, nil, "")

	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 0)
	h.Noise = 0
	gs.Seats[0].Hat = h

	castFromHand(t, gs, 0, "Doom Blade", 2, &gameast.Destroy{
		Target: gameast.Filter{Base: "creature", Targeted: true},
	})

	if onBattlefield(gs, enemy) {
		t.Fatalf("live-path removal failed to destroy the enemy threat (Their Dragon)")
	}
	if !onBattlefield(gs, own) {
		t.Fatalf("live-path removal destroyed the AI's OWN creature (My Bear)")
	}
}

func onBattlefield(gs *gameengine.GameState, p *gameengine.Permanent) bool {
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, q := range s.Battlefield {
			if q == p {
				return true
			}
		}
	}
	return false
}
