package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// chain_attack_signals_r60_test.go — pins the R60 round 12+ chain-attack
// awareness signals. Two helpers:
//   chainAttackPendingBonus — reads gs.PendingExtraCombats FIFO depth
//   chainAttackAnticipationBonus — scans legal pool for "additional
//     combat" trigger sources about to swing

// -----------------------------------------------------------------------
// chainAttackPendingBonus
// -----------------------------------------------------------------------

func TestChainAttackPendingBonus_NilStateIsZero(t *testing.T) {
	if got := chainAttackPendingBonus(nil); got != 0 {
		t.Errorf("nil GameState should be 0; got %.2f", got)
	}
}

func TestChainAttackPendingBonus_EmptyQueueIsZero(t *testing.T) {
	gs := newTestGame(t, 2)
	if got := chainAttackPendingBonus(gs); got != 0 {
		t.Errorf("empty queue should be 0; got %.2f", got)
	}
}

func TestChainAttackPendingBonus_OneQueuedIsFivePercent(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.PendingExtraCombats = []gameengine.PendingExtraCombat{{SourceCard: "Aurelia"}}
	if got := chainAttackPendingBonus(gs); got != 0.05 {
		t.Errorf("1 queued extra combat should be +0.05; got %.2f", got)
	}
}

func TestChainAttackPendingBonus_TwoQueuedIsTenPercent(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.PendingExtraCombats = []gameengine.PendingExtraCombat{
		{SourceCard: "Aurelia"},
		{SourceCard: "Aggravated Assault"},
	}
	if got := chainAttackPendingBonus(gs); got != 0.10 {
		t.Errorf("2 queued extra combats should be +0.10; got %.2f", got)
	}
}

func TestChainAttackPendingBonus_CapsAtFifteenPercent(t *testing.T) {
	gs := newTestGame(t, 2)
	// 5 queued — bonus would be 0.25 raw but caps at 0.15.
	gs.PendingExtraCombats = []gameengine.PendingExtraCombat{
		{}, {}, {}, {}, {},
	}
	if got := chainAttackPendingBonus(gs); got != 0.15 {
		t.Errorf("5 queued should cap at +0.15; got %.2f", got)
	}
}

// -----------------------------------------------------------------------
// chainAttackAnticipationBonus
// -----------------------------------------------------------------------

func TestChainAttackAnticipationBonus_EmptyLegalIsZero(t *testing.T) {
	if got := chainAttackAnticipationBonus(nil); got != 0 {
		t.Errorf("nil legal slice should be 0; got %.2f", got)
	}
	if got := chainAttackAnticipationBonus([]*gameengine.Permanent{}); got != 0 {
		t.Errorf("empty legal slice should be 0; got %.2f", got)
	}
}

func TestChainAttackAnticipationBonus_VanillaLegalsIsZero(t *testing.T) {
	mk := func(name string) *gameengine.Permanent {
		c := &gameengine.Card{
			Name:  name,
			AST:   &gameast.CardAST{Name: name},
			Types: []string{"creature"},
		}
		return &gameengine.Permanent{Card: c, Flags: map[string]int{}}
	}
	legal := []*gameengine.Permanent{mk("Bear"), mk("Goblin"), mk("Soldier")}
	if got := chainAttackAnticipationBonus(legal); got != 0 {
		t.Errorf("vanilla legal pool should be 0; got %.2f", got)
	}
}

func TestChainAttackAnticipationBonus_AureliaInLegalsFires(t *testing.T) {
	bear := &gameengine.Permanent{
		Card: &gameengine.Card{
			Name:  "Bear",
			AST:   &gameast.CardAST{Name: "Bear"},
			Types: []string{"creature"},
		},
		Flags: map[string]int{},
	}
	aurelia := &gameengine.Permanent{
		Card: &gameengine.Card{
			Name:  "Aurelia",
			Types: []string{"creature"},
			AST: &gameast.CardAST{
				Name: "Aurelia",
				Abilities: []gameast.Ability{
					&gameast.Triggered{Raw: "whenever aurelia attacks for the first time each turn, after this phase there is an additional combat phase"},
				},
			},
		},
		Flags: map[string]int{},
	}
	if got := chainAttackAnticipationBonus([]*gameengine.Permanent{bear, aurelia}); got != 0.05 {
		t.Errorf("Aurelia in legal pool should fire +0.05 anticipation; got %.2f", got)
	}
}

func TestChainAttackAnticipationBonus_NajeelaTriggersAlso(t *testing.T) {
	najeela := &gameengine.Permanent{
		Card: &gameengine.Card{
			Name:  "Najeela",
			Types: []string{"creature"},
			AST: &gameast.CardAST{
				Name: "Najeela",
				Abilities: []gameast.Ability{
					&gameast.Activated{Raw: "{w}{u}{b}{r}{g}: untap all creatures, they gain haste and trample, after this phase there is an additional combat phase"},
				},
			},
		},
		Flags: map[string]int{},
	}
	if got := chainAttackAnticipationBonus([]*gameengine.Permanent{najeela}); got != 0.05 {
		t.Errorf("Najeela in legal pool (additional combat in activated ability) should fire +0.05; got %.2f", got)
	}
}

func TestChainAttackAnticipationBonus_NilPermanentSkipped(t *testing.T) {
	// nil entries shouldn't crash; should be skipped.
	if got := chainAttackAnticipationBonus([]*gameengine.Permanent{nil, nil}); got != 0 {
		t.Errorf("nil permanents should be skipped without panic; got %.2f", got)
	}
}

// -----------------------------------------------------------------------
// Integration: ChooseAttackers folds in the chain bonus per attacker
// -----------------------------------------------------------------------

// TestChooseAttackers_PendingExtraCombatLiftsMarginalAttacker pins the
// end-to-end effect. A marginal 1/1 attacker that would normally HOLD
// at midrange threshold gets lifted by chainBonus when extra combats
// are queued — chip damage compounds across the chain.
func TestChooseAttackers_PendingExtraCombatLiftsMarginalAttacker(t *testing.T) {
	mkGS := func(queueDepth int) (*gameengine.GameState, *gameengine.Permanent) {
		gs := newTestGame(t, 2)
		gs.Seats[0].Life = 40
		gs.Seats[1].Life = 40
		gs.Turn = 5
		for i := 0; i < queueDepth; i++ {
			gs.PendingExtraCombats = append(gs.PendingExtraCombats,
				gameengine.PendingExtraCombat{SourceCard: "test"})
		}
		// 1/1 vanilla — val base = 0.1, marginal.
		atk := &gameengine.Permanent{
			Card: &gameengine.Card{
				Name:          "Marginal 1/1",
				Types:         []string{"creature"},
				AST:           &gameast.CardAST{Name: "Marginal 1/1"},
				BasePower:     1,
				BaseToughness: 1,
			},
			Controller: 0, Owner: 0, Flags: map[string]int{},
		}
		gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, atk)
		// Fat blocker on opp so open-lane bonus doesn't fire and
		// confound the chain-bonus measurement.
		newTestPermanent(gs.Seats[1], newTestCardMinimal("Wall", []string{"creature"}, 3, nil), 0, 5)
		return gs, atk
	}

	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 0)

	// Baseline: no queued combats. Marginal attacker may HOLD.
	gsBase, atkBase := mkGS(0)
	baseAttacks := len(h.ChooseAttackers(gsBase, 0, []*gameengine.Permanent{atkBase})) == 1

	// Pumped: 3 queued combats → +0.15 chain bonus.
	gsPumped, atkPumped := mkGS(3)
	pumpedAttacks := len(h.ChooseAttackers(gsPumped, 0, []*gameengine.Permanent{atkPumped})) == 1

	// Monotonicity: with 3 queued extras, the marginal attacker should
	// attack in any scenario where it would have at zero queue depth,
	// AND should still attack (or at least not strictly worse) under
	// the chain pressure.
	if baseAttacks && !pumpedAttacks {
		t.Error("chain bonus should be monotonic: 3 queued extras MUST attack any scenario the baseline attacks")
	}
	if !baseAttacks && !pumpedAttacks {
		t.Error("3 queued combats should lift the marginal 1/1 over threshold; got HOLD in both scenarios")
	}
}

// TestChooseAttackers_AureliaInLegalsLiftsOtherAttacker pins the
// anticipation bonus end-to-end: a marginal vanilla attacker scored
// alongside an Aurelia legal should benefit from the +0.05
// anticipation bonus (Aurelia about to trigger → extra combat coming
// → commit chip damage now).
func TestChooseAttackers_AureliaInLegalsLiftsOtherAttacker(t *testing.T) {
	mkGS := func(withAurelia bool) (*gameengine.GameState, *gameengine.Permanent) {
		gs := newTestGame(t, 2)
		gs.Seats[0].Life = 40
		gs.Seats[1].Life = 40
		gs.Turn = 5

		bear := &gameengine.Permanent{
			Card: &gameengine.Card{
				Name:          "Bear",
				Types:         []string{"creature"},
				AST:           &gameast.CardAST{Name: "Bear"},
				BasePower:     2,
				BaseToughness: 2,
			},
			Controller: 0, Owner: 0, Flags: map[string]int{},
		}
		gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, bear)
		var legal []*gameengine.Permanent
		legal = append(legal, bear)
		if withAurelia {
			aurelia := &gameengine.Permanent{
				Card: &gameengine.Card{
					Name:          "Aurelia",
					Types:         []string{"creature"},
					BasePower:     3,
					BaseToughness: 4,
					AST: &gameast.CardAST{
						Name: "Aurelia",
						Abilities: []gameast.Ability{
							&gameast.Triggered{Raw: "whenever aurelia attacks for the first time, additional combat phase"},
						},
					},
				},
				Controller: 0, Owner: 0, Flags: map[string]int{},
			}
			gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, aurelia)
			legal = append(legal, aurelia)
		}
		// Wall on opp to block open-lane bonus.
		newTestPermanent(gs.Seats[1], newTestCardMinimal("Wall", []string{"creature"}, 3, nil), 0, 5)
		_ = legal
		return gs, bear
	}

	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 0)

	gsBase, bearBase := mkGS(false)
	gsAur, bearAur := mkGS(true)

	// We mirror real legal-pool construction by passing the same slice.
	baseLegal := []*gameengine.Permanent{bearBase}
	aurLegal := []*gameengine.Permanent{bearAur, gsAur.Seats[0].Battlefield[1]}

	baseAttacks := false
	for _, p := range h.ChooseAttackers(gsBase, 0, baseLegal) {
		if p == bearBase {
			baseAttacks = true
		}
	}
	aurAttacks := false
	for _, p := range h.ChooseAttackers(gsAur, 0, aurLegal) {
		if p == bearAur {
			aurAttacks = true
		}
	}

	// Monotonicity: bear attacks at least as often with Aurelia in the
	// legal pool as without.
	if baseAttacks && !aurAttacks {
		t.Error("bear should attack with Aurelia in legals (anticipation bonus is monotonic) if it attacks without")
	}
}
