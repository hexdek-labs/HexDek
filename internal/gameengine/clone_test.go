package gameengine

import (
	"math/rand"
	"testing"
)

func TestCloneForRollout_NilSafe(t *testing.T) {
	var gs *GameState
	if gs.CloneForRollout(nil) != nil {
		t.Fatal("nil GameState should clone to nil")
	}
}

func TestCloneForRollout_BasicIdentity(t *testing.T) {
	gs := newFixtureGame(t)
	addLibrary(gs, 0, "CardA", "CardB")
	addHand(gs, 1, "CardC")
	addBattlefield(gs, 0, "Bear", 2, 2, "creature")
	gs.Turn = 5
	gs.Phase = "main"
	gs.Step = "precombat_main"
	gs.Active = 0

	rng := rand.New(rand.NewSource(99))
	clone := gs.CloneForRollout(rng)

	if clone == gs {
		t.Fatal("clone must be a different pointer")
	}
	if clone.Turn != 5 || clone.Phase != "main" || clone.Step != "precombat_main" || clone.Active != 0 {
		t.Errorf("turn metadata not copied: turn=%d phase=%s step=%s active=%d",
			clone.Turn, clone.Phase, clone.Step, clone.Active)
	}
	if clone.Rng != rng {
		t.Error("clone should use the provided RNG")
	}
}

func TestCloneForRollout_MutationIsolation_Library(t *testing.T) {
	gs := newFixtureGame(t)
	addLibrary(gs, 0, "A", "B", "C")

	clone := gs.CloneForRollout(rand.New(rand.NewSource(1)))

	if len(clone.Seats[0].Library) != 3 {
		t.Fatalf("expected 3 library cards, got %d", len(clone.Seats[0].Library))
	}

	clone.Seats[0].Library = clone.Seats[0].Library[:1]
	if len(gs.Seats[0].Library) != 3 {
		t.Error("mutating clone library affected original")
	}
}

func TestCloneForRollout_MutationIsolation_Hand(t *testing.T) {
	gs := newFixtureGame(t)
	addHand(gs, 1, "X", "Y")

	clone := gs.CloneForRollout(rand.New(rand.NewSource(1)))
	clone.Seats[1].Hand[0].Name = "MUTATED"

	if gs.Seats[1].Hand[0].Name == "MUTATED" {
		t.Error("mutating clone hand card affected original")
	}
}

func TestCloneForRollout_MutationIsolation_Battlefield(t *testing.T) {
	gs := newFixtureGame(t)
	p := addBattlefield(gs, 0, "Sol Ring", 0, 0, "artifact")
	p.Counters["charge"] = 3

	clone := gs.CloneForRollout(rand.New(rand.NewSource(1)))
	cp := clone.Seats[0].Battlefield[0]

	if cp == p {
		t.Fatal("cloned permanent must be different pointer")
	}
	if cp.Counters["charge"] != 3 {
		t.Errorf("counter not cloned: got %d", cp.Counters["charge"])
	}

	cp.Counters["charge"] = 99
	if p.Counters["charge"] != 3 {
		t.Error("mutating cloned permanent counters affected original")
	}
}

func TestCloneForRollout_AttachedToRemap(t *testing.T) {
	gs := newFixtureGame(t)
	creature := addBattlefield(gs, 0, "Bear", 2, 2, "creature")
	equip := addBattlefield(gs, 0, "Sword", 0, 0, "artifact", "equipment")
	equip.AttachedTo = creature

	clone := gs.CloneForRollout(rand.New(rand.NewSource(1)))
	bf := clone.Seats[0].Battlefield

	var clonedEquip, clonedCreature *Permanent
	for _, cp := range bf {
		if cp.Card.Name == "Sword" {
			clonedEquip = cp
		}
		if cp.Card.Name == "Bear" {
			clonedCreature = cp
		}
	}

	if clonedEquip == nil || clonedCreature == nil {
		t.Fatal("expected both permanents in clone")
	}
	if clonedEquip.AttachedTo != clonedCreature {
		t.Error("AttachedTo not remapped to cloned creature")
	}
	if clonedEquip.AttachedTo == creature {
		t.Error("AttachedTo still points at original creature")
	}
}

func TestCloneForRollout_CommanderDamageDeepCopy(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Seats[0].CommanderDamage = map[int]map[string]int{
		1: {"Krenko": 15},
	}

	clone := gs.CloneForRollout(rand.New(rand.NewSource(1)))
	clone.Seats[0].CommanderDamage[1]["Krenko"] = 21

	if gs.Seats[0].CommanderDamage[1]["Krenko"] != 15 {
		t.Error("mutating clone commander damage affected original")
	}
}

func TestCloneForRollout_CommanderTaxAlias(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Seats[0].CommanderCastCounts["Krenko"] = 2
	gs.Seats[0].CommanderTax["Krenko"] = 2

	clone := gs.CloneForRollout(rand.New(rand.NewSource(1)))

	clone.Seats[0].CommanderCastCounts["Krenko"] = 5
	if clone.Seats[0].CommanderTax["Krenko"] != 5 {
		t.Error("CommanderTax should alias CommanderCastCounts in clone")
	}

	if gs.Seats[0].CommanderCastCounts["Krenko"] != 2 {
		t.Error("clone CommanderCastCounts mutation leaked to original")
	}
}

func TestCloneForRollout_ManaPoolDeepCopy(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Seats[0].ManaPool = 5
	gs.Seats[0].Mana = &ColoredManaPool{
		W: 1, U: 2,
		Restricted: []RestrictedMana{{Color: "R", Amount: 3}},
	}

	clone := gs.CloneForRollout(rand.New(rand.NewSource(1)))
	clone.Seats[0].Mana.W = 99
	clone.Seats[0].Mana.Restricted[0].Amount = 99

	if gs.Seats[0].Mana.W != 1 {
		t.Error("mutating clone mana W affected original")
	}
	if gs.Seats[0].Mana.Restricted[0].Amount != 3 {
		t.Error("mutating clone restricted mana affected original")
	}
}

func TestCloneForRollout_StackClone(t *testing.T) {
	gs := newFixtureGame(t)
	card := &Card{Name: "Lightning Bolt", Owner: 0}
	perm := addBattlefield(gs, 0, "Goblin", 1, 1, "creature")
	gs.Stack = []*StackItem{
		{
			ID:         1,
			Controller: 0,
			Card:       card,
			Source:     perm,
			Kind:       "spell",
			Targets: []Target{
				{Kind: TargetKindPermanent, Permanent: perm},
			},
			CostMeta: map[string]interface{}{"evoked": true},
		},
	}

	clone := gs.CloneForRollout(rand.New(rand.NewSource(1)))

	if len(clone.Stack) != 1 {
		t.Fatalf("expected 1 stack item, got %d", len(clone.Stack))
	}
	csi := clone.Stack[0]
	if csi == gs.Stack[0] {
		t.Error("stack item should be a different pointer")
	}
	if csi.Card == card {
		t.Error("stack card should be cloned")
	}
	if csi.Card.Name != "Lightning Bolt" {
		t.Error("stack card name not preserved")
	}
	if csi.Source == perm {
		t.Error("stack source should be remapped")
	}
	if csi.Targets[0].Permanent == perm {
		t.Error("target permanent should be remapped")
	}

	csi.CostMeta["evoked"] = false
	if gs.Stack[0].CostMeta["evoked"] != true {
		t.Error("mutating clone stack CostMeta affected original")
	}
}

func TestCloneForRollout_ReplacementEffectRemap(t *testing.T) {
	gs := newFixtureGame(t)
	p := addBattlefield(gs, 0, "Doubling Season", 0, 0, "enchantment")
	gs.Replacements = []*ReplacementEffect{
		{
			EventType:      "counter_add",
			HandlerID:      "doubling_season_1",
			SourcePerm:     p,
			ControllerSeat: 0,
		},
	}

	clone := gs.CloneForRollout(rand.New(rand.NewSource(1)))

	if len(clone.Replacements) != 1 {
		t.Fatal("expected 1 replacement effect")
	}
	cre := clone.Replacements[0]
	if cre.SourcePerm == p {
		t.Error("replacement SourcePerm should be remapped")
	}
	if cre.SourcePerm.Card.Name != "Doubling Season" {
		t.Error("remapped replacement SourcePerm should reference cloned permanent")
	}
}

func TestCloneForRollout_ContinuousEffectRemap(t *testing.T) {
	gs := newFixtureGame(t)
	p := addBattlefield(gs, 0, "Elesh Norn", 4, 7, "creature")
	gs.ContinuousEffects = []*ContinuousEffect{
		{
			Layer:      7,
			SourcePerm: p,
		},
	}

	clone := gs.CloneForRollout(rand.New(rand.NewSource(1)))

	if len(clone.ContinuousEffects) != 1 {
		t.Fatal("expected 1 continuous effect")
	}
	cce := clone.ContinuousEffects[0]
	if cce.SourcePerm == p {
		t.Error("continuous effect SourcePerm should be remapped")
	}
}

func TestCloneForRollout_FlagsDeepCopy(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Flags = map[string]int{"extra_turn": 1}

	clone := gs.CloneForRollout(rand.New(rand.NewSource(1)))
	clone.Flags["extra_turn"] = 99

	if gs.Flags["extra_turn"] != 1 {
		t.Error("mutating clone game flags affected original")
	}
}

func TestCloneForRollout_SeatFlags(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Seats[0].Flags = map[string]int{"protection": 1}

	clone := gs.CloneForRollout(rand.New(rand.NewSource(1)))
	clone.Seats[0].Flags["protection"] = 0

	if gs.Seats[0].Flags["protection"] != 1 {
		t.Error("mutating clone seat flags affected original")
	}
}

func TestCloneForRollout_EventLogCopied(t *testing.T) {
	gs := newFixtureGame(t)
	gs.EventLog = []Event{
		{Kind: "draw", Seat: 0, Amount: 1},
		{Kind: "damage", Seat: 1, Amount: 3},
	}

	clone := gs.CloneForRollout(rand.New(rand.NewSource(1)))

	if len(clone.EventLog) != 2 {
		t.Fatalf("expected 2 events, got %d", len(clone.EventLog))
	}

	clone.EventLog = append(clone.EventLog, Event{Kind: "test"})
	if len(gs.EventLog) != 2 {
		t.Error("appending to clone EventLog affected original")
	}
}

func TestCloneForRollout_SharedCorpus(t *testing.T) {
	gs := newFixtureGame(t)
	clone := gs.CloneForRollout(rand.New(rand.NewSource(1)))
	if clone.Cards != gs.Cards {
		t.Error("corpus (Cards) should be shared, not cloned")
	}
}

func TestCloneForRollout_ZoneCastGrantsRemap(t *testing.T) {
	gs := newFixtureGame(t)
	card := &Card{Name: "Release to the Wind", Owner: 0}
	gs.Seats[0].Exile = append(gs.Seats[0].Exile, card)
	gs.ZoneCastGrants = map[*Card]*ZoneCastPermission{
		card: {Zone: "exile", Keyword: "static_exile_cast"},
	}

	clone := gs.CloneForRollout(rand.New(rand.NewSource(1)))

	if len(clone.ZoneCastGrants) != 1 {
		t.Fatal("expected 1 zone cast grant")
	}

	for cc, perm := range clone.ZoneCastGrants {
		if cc == card {
			t.Error("ZoneCastGrants card key should be remapped to cloned card")
		}
		if perm.Zone != "exile" {
			t.Error("ZoneCastGrants permission not preserved")
		}
	}
}

func TestCloneForRollout_GraveyardAndExile(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Seats[0].Graveyard = []*Card{{Name: "Dark Ritual", Owner: 0}}
	gs.Seats[1].Exile = []*Card{{Name: "Path Target", Owner: 1}}

	clone := gs.CloneForRollout(rand.New(rand.NewSource(1)))

	if len(clone.Seats[0].Graveyard) != 1 || clone.Seats[0].Graveyard[0].Name != "Dark Ritual" {
		t.Error("graveyard not cloned")
	}
	if clone.Seats[0].Graveyard[0] == gs.Seats[0].Graveyard[0] {
		t.Error("graveyard card should be a different pointer")
	}

	if len(clone.Seats[1].Exile) != 1 || clone.Seats[1].Exile[0].Name != "Path Target" {
		t.Error("exile not cloned")
	}
}

func TestCloneForRollout_PermanentFlags(t *testing.T) {
	gs := newFixtureGame(t)
	p := addBattlefield(gs, 0, "Hexproof Bear", 2, 2, "creature")
	p.Flags["hexproof"] = 1

	clone := gs.CloneForRollout(rand.New(rand.NewSource(1)))
	cp := clone.Seats[0].Battlefield[0]

	if cp.Flags["hexproof"] != 1 {
		t.Error("permanent flags not cloned")
	}
	cp.Flags["hexproof"] = 0
	if p.Flags["hexproof"] != 1 {
		t.Error("mutating clone permanent flags affected original")
	}
}

func TestCloneForRollout_LifeAndLossState(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Seats[0].Life = 15
	gs.Seats[0].PoisonCounters = 3
	gs.Seats[1].Lost = true
	gs.Seats[1].LossReason = "commander_damage"

	clone := gs.CloneForRollout(rand.New(rand.NewSource(1)))

	if clone.Seats[0].Life != 15 || clone.Seats[0].PoisonCounters != 3 {
		t.Error("life/poison not copied")
	}
	if !clone.Seats[1].Lost || clone.Seats[1].LossReason != "commander_damage" {
		t.Error("loss state not copied")
	}

	clone.Seats[0].Life = 0
	if gs.Seats[0].Life != 15 {
		t.Error("mutating clone life affected original")
	}
}

func TestCloneForRollout_PreventionShields(t *testing.T) {
	gs := newFixtureGame(t)
	gs.PreventionShields = []PreventionShield{
		{SourceCard: "Fog", Amount: -1, TargetSeat: 0},
	}

	clone := gs.CloneForRollout(rand.New(rand.NewSource(1)))

	if len(clone.PreventionShields) != 1 {
		t.Fatal("expected 1 prevention shield")
	}
	if clone.PreventionShields[0].SourceCard != "Fog" {
		t.Error("prevention shield not copied")
	}

	clone.PreventionShields = append(clone.PreventionShields, PreventionShield{SourceCard: "Holy Day"})
	if len(gs.PreventionShields) != 1 {
		t.Error("appending to clone prevention shields affected original")
	}
}

func TestCloneForRollout_DelayedTriggers(t *testing.T) {
	gs := newFixtureGame(t)
	gs.DelayedTriggers = []*DelayedTrigger{
		{
			TriggerAt:      "end_of_turn",
			ControllerSeat: 0,
			SourceCardName: "Sneak Attack",
			CreatedTurn:    3,
		},
	}

	clone := gs.CloneForRollout(rand.New(rand.NewSource(1)))

	if len(clone.DelayedTriggers) != 1 {
		t.Fatal("expected 1 delayed trigger")
	}
	if clone.DelayedTriggers[0] == gs.DelayedTriggers[0] {
		t.Error("delayed trigger should be shallow-cloned (different pointer)")
	}
	if clone.DelayedTriggers[0].SourceCardName != "Sneak Attack" {
		t.Error("delayed trigger content not preserved")
	}
}
