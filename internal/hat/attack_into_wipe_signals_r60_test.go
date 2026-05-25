package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// attack_into_wipe_signals_r60_test.go — pins the R60 round 11+
// counter-balance to the existing wrath hold-back at yggdrasil.go:5191.
// Two helpers (wipeBaitExpendableBonus, useItOrLoseItTriggerBonus) tilt
// expendable / triggered attackers FORWARD under wrath pressure,
// complementing the existing -0.15 hold-back on value engines.

// mkWipeTestCreature builds a creature permanent on seat. Optional
// oracle text on the AST so postcombat/attack-trigger detection works.
func mkWipeTestCreature(seat *gameengine.Seat, name string, power, toughness int, oracle string) *gameengine.Permanent {
	ast := &gameast.CardAST{Name: name}
	if oracle != "" {
		ast.Abilities = append(ast.Abilities, &gameast.Triggered{Raw: oracle})
	}
	card := &gameengine.Card{
		Name:          name,
		AST:           ast,
		Types:         []string{"creature"},
		BasePower:     power,
		BaseToughness: toughness,
	}
	p := &gameengine.Permanent{
		Card:       card,
		Controller: seat.Idx,
		Owner:      seat.Idx,
		Flags:      map[string]int{},
	}
	seat.Battlefield = append(seat.Battlefield, p)
	return p
}

// -----------------------------------------------------------------------
// wipeBaitExpendableBonus
// -----------------------------------------------------------------------

func TestWipeBaitExpendableBonus_NoWrathReturnsZero(t *testing.T) {
	gs := newTestGame(t, 2)
	p := mkWipeTestCreature(gs.Seats[0], "Bear", 2, 2, "")
	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 0)
	if got := h.wipeBaitExpendableBonus(gs, 0, p, false, 0.0); got != 0 {
		t.Errorf("no wrath should return 0; got %.2f", got)
	}
}

func TestWipeBaitExpendableBonus_DesperateReturnsZero(t *testing.T) {
	gs := newTestGame(t, 2)
	p := mkWipeTestCreature(gs.Seats[0], "Bear", 2, 2, "")
	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 0)
	// relPos = -0.3 → desperate; the existing all-in logic already swings
	// everything so this signal stays out of the way.
	if got := h.wipeBaitExpendableBonus(gs, 0, p, true, -0.3); got != 0 {
		t.Errorf("desperate relPos (<= -0.2) should return 0; got %.2f", got)
	}
}

func TestWipeBaitExpendableBonus_ExpendableFiresUnderWrath(t *testing.T) {
	gs := newTestGame(t, 2)
	p := mkWipeTestCreature(gs.Seats[0], "Bear", 2, 2, "")
	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 0)
	if got := h.wipeBaitExpendableBonus(gs, 0, p, true, 0.0); got != 0.10 {
		t.Errorf("expendable creature under wrath should be +0.10; got %.2f", got)
	}
}

func TestWipeBaitExpendableBonus_CommanderExcluded(t *testing.T) {
	gs := newTestGame(t, 2)
	commanderCard := &gameengine.Card{
		Name:  "Commander",
		Types: []string{"creature", "legendary"},
		AST:   &gameast.CardAST{Name: "Commander"},
	}
	gs.Seats[0].CommanderNames = []string{"Commander"}
	p := &gameengine.Permanent{Card: commanderCard, Controller: 0, Owner: 0, Flags: map[string]int{}}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, p)
	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 0)
	if got := h.wipeBaitExpendableBonus(gs, 0, p, true, 0.0); got != 0 {
		t.Errorf("commander should be excluded from wipe-bait bonus (tax on recast); got %.2f", got)
	}
}

func TestWipeBaitExpendableBonus_ValueEngineExcluded(t *testing.T) {
	gs := newTestGame(t, 2)
	p := mkWipeTestCreature(gs.Seats[0], "Bonecrusher", 2, 2, "")
	// Tag as a value-engine key — mirrors what Freya would emit.
	h := NewYggdrasilHat(&StrategyProfile{
		Archetype: ArchetypeMidrange,
	}, 0)
	h.valueEngineSet = map[string]bool{"Bonecrusher": true}
	if got := h.wipeBaitExpendableBonus(gs, 0, p, true, 0.0); got != 0 {
		t.Errorf("value-engine creature should be excluded — strategic asset; got %.2f", got)
	}
}

// -----------------------------------------------------------------------
// useItOrLoseItTriggerBonus
// -----------------------------------------------------------------------

func TestUseItOrLoseItTriggerBonus_NoWrathReturnsZero(t *testing.T) {
	gs := newTestGame(t, 2)
	p := mkWipeTestCreature(gs.Seats[0], "Hellrider",
		2, 2, "whenever hellrider attacks, hellrider deals 1 damage to defending player for each attacking creature")
	if got := useItOrLoseItTriggerBonus(p, false, 0.0); got != 0 {
		t.Errorf("no wrath should return 0; got %.2f", got)
	}
	_ = gs
}

func TestUseItOrLoseItTriggerBonus_DesperateReturnsZero(t *testing.T) {
	gs := newTestGame(t, 2)
	p := mkWipeTestCreature(gs.Seats[0], "Hellrider",
		2, 2, "whenever hellrider attacks, hellrider deals 1 damage to defending player for each attacking creature")
	if got := useItOrLoseItTriggerBonus(p, true, -0.3); got != 0 {
		t.Errorf("desperate position should bail (all-in arm covers it); got %.2f", got)
	}
	_ = gs
}

func TestUseItOrLoseItTriggerBonus_TriggerBearerFires(t *testing.T) {
	gs := newTestGame(t, 2)
	// Hellrider — attack-declaration damage trigger (attackTriggerBonus=+0.20).
	p := mkWipeTestCreature(gs.Seats[0], "Hellrider",
		2, 2, "whenever hellrider attacks, hellrider deals 1 damage to defending player for each attacking creature")
	if got := useItOrLoseItTriggerBonus(p, true, 0.0); got != 0.10 {
		t.Errorf("attack-trigger bearer under wrath should be +0.10; got %.2f", got)
	}
	// Toski — postcombat draw trigger (postcombatTriggerBonus=+0.20).
	p2 := mkWipeTestCreature(gs.Seats[0], "Toski",
		1, 1, "whenever toski deals combat damage to a player, draw a card")
	if got := useItOrLoseItTriggerBonus(p2, true, 0.0); got != 0.10 {
		t.Errorf("postcombat-trigger bearer under wrath should be +0.10; got %.2f", got)
	}
}

func TestUseItOrLoseItTriggerBonus_VanillaReturnsZero(t *testing.T) {
	gs := newTestGame(t, 2)
	p := mkWipeTestCreature(gs.Seats[0], "Grizzly Bears", 2, 2, "")
	if got := useItOrLoseItTriggerBonus(p, true, 0.0); got != 0 {
		t.Errorf("vanilla without any trigger should return 0 — nothing to use-or-lose; got %.2f", got)
	}
	_ = gs
}

// -----------------------------------------------------------------------
// Composition: both signals stack on an expendable triggered creature
// -----------------------------------------------------------------------

// TestWipeSignals_StackOnExpendableTriggered pins the design intent:
// a non-strategic creature with a trigger gets BOTH bonuses under
// wrath = +0.20 total. That's the strongest swing-into-wipe push,
// matching the Magic intuition "expendable Toski-shape attacker
// under wrath threat should ALWAYS swing — both because the body
// dies anyway AND because the trigger captures permanent value".
func TestWipeSignals_StackOnExpendableTriggered(t *testing.T) {
	gs := newTestGame(t, 2)
	// Toski-shape: cheap, triggered, NOT in the value-engine set.
	p := mkWipeTestCreature(gs.Seats[0], "Random Trigger Creature",
		2, 2, "whenever this creature deals combat damage to a player, draw a card")
	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 0)
	a := h.wipeBaitExpendableBonus(gs, 0, p, true, 0.0)
	b := useItOrLoseItTriggerBonus(p, true, 0.0)
	total := a + b
	if total != 0.20 {
		t.Errorf("expendable triggered creature under wrath should stack to +0.20 (0.10 + 0.10); got bait=%.2f trigger=%.2f total=%.2f",
			a, b, total)
	}
}

// TestWipeSignals_ValueEngineTriggeredNetsPositive — symmetric pin to
// the design comment in useItOrLoseItTriggerBonus: an Edric-shape
// creature (value engine + draw trigger) under wrath gets:
//   - existing -0.15 from line 5191 (value-engine penalty)
//   - postcombatTriggerBonus +0.20 (draw on combat damage)
//   - useItOrLoseItTriggerBonus +0.10 (use-it-or-lose-it)
//   - wipeBaitExpendableBonus +0.00 (VE excluded)
//
// Net (above the base creature val) = +0.15 — STILL positive. The
// Edric attacks into the wipe to get one last draw before dying.
func TestWipeSignals_ValueEngineTriggeredNetsPositive(t *testing.T) {
	gs := newTestGame(t, 2)
	p := mkWipeTestCreature(gs.Seats[0], "Edric",
		2, 3, "whenever a creature you control deals combat damage to a player, you may draw a card")
	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 0)
	h.valueEngineSet = map[string]bool{"Edric": true}

	existingPenalty := 0.0
	if h.isValueEngineKey(p.Card) {
		existingPenalty = -0.15 // mirrors yggdrasil.go:5191
	}
	trigger := postcombatTriggerBonus(p.Card)
	bait := h.wipeBaitExpendableBonus(gs, 0, p, true, 0.0)
	use := useItOrLoseItTriggerBonus(p, true, 0.0)

	net := existingPenalty + trigger + bait + use
	if net <= 0 {
		t.Errorf("value-engine triggered creature under wrath should net POSITIVE swing bias (use-it-or-lose-it dominates); got %+.2f penalty + %+.2f trigger + %+.2f bait + %+.2f use = %+.2f",
			existingPenalty, trigger, bait, use, net)
	}
	if bait != 0 {
		t.Errorf("VE creature should NOT receive bait bonus (strategic exclusion); got %.2f", bait)
	}
	if use != 0.10 {
		t.Errorf("VE creature with trigger should still receive use-it-or-lose-it (intentional — value engines with triggers want to swing); got %.2f",
			use)
	}
}
