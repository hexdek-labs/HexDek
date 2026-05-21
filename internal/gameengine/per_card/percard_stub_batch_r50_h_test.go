package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// R50 stub-batch H ports — 10 tribal-lord / creature-type-buff
// handlers. Avoids A/B/C/D/E/F/G picks per the campaign ledger.
//
// Picks:
//   1. Mabel, Heir to Cragflame    Other Mice +1/+1
//   2. Commander Mustard           Other Soldiers vigilance/trample/haste
//   3. Rienne, Angel of Rebirth    Other multicolored +1/+0
//   4. Kolodin, Triumph Caster     Mounts/Vehicles haste anthem
//   5. Jodah, the Unifier          Legendary creatures +X/+X (X = legendary count)
//   6. Gornog, the Red Reaper      Coward-tag attack + warrior anthem
//   7. Gisa, the Hellraiser        Skeletons/Zombies +1/+1 + menace
//   8. Aphelia, Viper Whisperer    Gorgons/Snakes tribal-halving activated
//   9. Tovolar, Dire Overlord      Human-Werewolf night-flip transform
//  10. Samut, the Driving Force    Noncreature {X}-discount in cost_modifiers

// ---------------------------------------------------------------------------
// 1. Mabel — Other Mice +1/+1
// ---------------------------------------------------------------------------

func TestMabel_StampsAnthemOnOwnMice(t *testing.T) {
	gs := newGame(t, 2)
	mabel := addPerm(gs, 0, "Mabel, Heir to Cragflame", "creature", "legendary", "mouse")
	mouse := addPerm(gs, 0, "Cuddly Mouse", "creature", "mouse")
	mouse.Card.BasePower = 1
	mouse.Card.BaseToughness = 1
	bear := addPerm(gs, 0, "Grizzly Bears", "creature", "bear")
	bear.Card.BasePower = 2
	bear.Card.BaseToughness = 2

	mabelRefreshAnthemOnETB(gs, mabel)

	if mouse.Power() != 2 || mouse.Toughness() != 2 {
		t.Errorf("Mouse should be 2/2 (1/1 +1/+1 from Mabel); got %d/%d", mouse.Power(), mouse.Toughness())
	}
	if bear.Power() != 2 {
		t.Errorf("Non-Mouse should be unaffected; got %d", bear.Power())
	}
}

func TestMabel_DoesNotBuffSelf(t *testing.T) {
	gs := newGame(t, 2)
	mabel := addPerm(gs, 0, "Mabel, Heir to Cragflame", "creature", "legendary", "mouse")
	mabel.Card.BasePower = 3
	mabel.Card.BaseToughness = 3
	mabelRefreshAnthemOnETB(gs, mabel)

	if mabel.Power() != 3 {
		t.Errorf("Mabel should NOT buff herself ('Other Mice'); got %d", mabel.Power())
	}
}

// ---------------------------------------------------------------------------
// 2. Commander Mustard — Soldier anthem
// ---------------------------------------------------------------------------

func TestCommanderMustard_StampsKeywordsOnSoldiers(t *testing.T) {
	gs := newGame(t, 2)
	mustard := addPerm(gs, 0, "Commander Mustard", "creature", "legendary", "soldier")
	sol := addPerm(gs, 0, "Skyclave Squad", "creature", "soldier")
	rogue := addPerm(gs, 0, "Crooked Pal", "creature", "rogue")

	mustardRefreshAnthemOnETB(gs, mustard)

	if sol.Flags["kw:vigilance"] != 1 || sol.Flags["kw:trample"] != 1 || sol.Flags["kw:haste"] != 1 {
		t.Errorf("Soldier should gain vigilance/trample/haste; flags=%v", sol.Flags)
	}
	if rogue.Flags["kw:vigilance"] == 1 || rogue.Flags["kw:haste"] == 1 {
		t.Errorf("Non-Soldier should NOT be stamped; flags=%v", rogue.Flags)
	}
}

// ---------------------------------------------------------------------------
// 3. Rienne — Multicolored anthem
// ---------------------------------------------------------------------------

func TestRienne_BuffsMulticoloredCreatures(t *testing.T) {
	gs := newGame(t, 2)
	rienne := addPerm(gs, 0, "Rienne, Angel of Rebirth", "creature", "legendary", "angel")
	multi := addPerm(gs, 0, "Shio", "creature")
	multi.Card.Colors = []string{"W", "U"}
	multi.Card.BasePower = 2
	multi.Card.BaseToughness = 2
	mono := addPerm(gs, 0, "Llanowar Elves", "creature")
	mono.Card.Colors = []string{"G"}
	mono.Card.BasePower = 1
	mono.Card.BaseToughness = 1

	rienneRefreshAnthemOnETB(gs, rienne)

	if multi.Power() != 3 || multi.Toughness() != 2 {
		t.Errorf("Multicolored should be 3/2 (+1/+0); got %d/%d", multi.Power(), multi.Toughness())
	}
	if mono.Power() != 1 {
		t.Errorf("Monocolor should not be buffed; got %d", mono.Power())
	}
}

// ---------------------------------------------------------------------------
// 4. Kolodin — Mount/Vehicle haste anthem
// ---------------------------------------------------------------------------

func TestKolodin_StampsHasteOnMountsAndVehicles(t *testing.T) {
	gs := newGame(t, 2)
	kol := addPerm(gs, 0, "Kolodin, Triumph Caster", "creature", "legendary")
	mount := addPerm(gs, 0, "Mountainback Lion", "creature", "mount")
	vehicle := addPerm(gs, 0, "Smuggler's Copter", "artifact", "vehicle")
	bear := addPerm(gs, 0, "Grizzly Bears", "creature", "bear")

	kolodinRefreshAnthemOnETB(gs, kol)

	if mount.Flags["kw:haste"] != 1 {
		t.Errorf("Mount should be stamped with kw:haste")
	}
	if vehicle.Flags["kw:haste"] != 1 {
		t.Errorf("Vehicle should be stamped with kw:haste")
	}
	if bear.Flags["kw:haste"] == 1 {
		t.Errorf("Bear should NOT be stamped")
	}
}

// ---------------------------------------------------------------------------
// 5. Jodah, the Unifier — Legendary +X/+X anthem
// ---------------------------------------------------------------------------

func TestJodah_LegendaryAnthemBuffsByCount(t *testing.T) {
	gs := newGame(t, 2)
	jodah := addPerm(gs, 0, "Jodah, the Unifier", "creature", "legendary")
	jodah.Card.BasePower = 4
	jodah.Card.BaseToughness = 4

	leg1 := addPerm(gs, 0, "Atraxa", "creature", "legendary", "phyrexian")
	leg1.Card.BasePower = 4
	leg1.Card.BaseToughness = 4

	leg2 := addPerm(gs, 0, "Avacyn", "creature", "legendary", "angel")
	leg2.Card.BasePower = 8
	leg2.Card.BaseToughness = 8

	bear := addPerm(gs, 0, "Grizzly Bears", "creature")
	bear.Card.BasePower = 2
	bear.Card.BaseToughness = 2

	jodahRefreshAnthemOnETB(gs, jodah)

	// X = 3 (Jodah + Atraxa + Avacyn). Each legendary +3/+3.
	if jodah.Power() != 7 || jodah.Toughness() != 7 {
		t.Errorf("Jodah should be 7/7 (4/4 + 3/3); got %d/%d", jodah.Power(), jodah.Toughness())
	}
	if leg1.Power() != 7 {
		t.Errorf("Atraxa should be 7/7; got %d", leg1.Power())
	}
	if bear.Power() != 2 {
		t.Errorf("Non-legendary bear should NOT be buffed; got %d", bear.Power())
	}
}

func TestJodah_RefreshOnPermanentETB(t *testing.T) {
	gs := newGame(t, 2)
	jodah := addPerm(gs, 0, "Jodah, the Unifier", "creature", "legendary")
	jodah.Card.BasePower = 4
	jodah.Card.BaseToughness = 4

	jodahRefreshAnthemOnETB(gs, jodah)
	if jodah.Power() != 5 {
		t.Fatalf("Baseline: with only Jodah X=1, Jodah should be 5/5; got %d", jodah.Power())
	}

	// New legendary creature enters.
	leg := addPerm(gs, 0, "Sliver Legion", "creature", "legendary", "sliver")
	leg.Card.BasePower = 7
	leg.Card.BaseToughness = 7
	jodahRefreshAnthemOnEvent(gs, jodah, nil)

	// X = 2; Jodah goes to 4+2=6/6; leg goes to 7+2=9/9.
	if jodah.Power() != 6 {
		t.Errorf("After 2nd legendary, Jodah should be 6/6; got %d", jodah.Power())
	}
	if leg.Power() != 9 {
		t.Errorf("Sliver Legion should be 9/9; got %d", leg.Power())
	}
}

// ---------------------------------------------------------------------------
// 6. Gornog — Coward-tag + warrior anthem
// ---------------------------------------------------------------------------

func TestGornog_TagsOpponentCreatureOnWarriorAttack(t *testing.T) {
	gs := newGame(t, 2)
	gornog := addPerm(gs, 0, "Gornog, the Red Reaper", "creature", "legendary", "warrior")
	warrior := addPerm(gs, 0, "Adamant Will Warrior", "creature", "warrior")
	warrior.Flags["attacking"] = 1
	opp := addPerm(gs, 1, "Goblin", "creature", "goblin")
	opp.Card.BasePower = 1
	opp.Card.BaseToughness = 1

	gornogOnAttack(gs, gornog, map[string]interface{}{
		"attacker_perm":  warrior,
		"defending_seat": 1,
	})

	hasCoward := false
	for _, t := range opp.Card.Types {
		if t == "coward" {
			hasCoward = true
		}
	}
	if !hasCoward {
		t.Errorf("Opp creature should be tagged coward; types=%v", opp.Card.Types)
	}
}

func TestGornog_BlockRestrictionFlagSet(t *testing.T) {
	gs := newGame(t, 2)
	gornog := addPerm(gs, 0, "Gornog, the Red Reaper", "creature", "legendary")
	gornogETBSetBlockRestriction(gs, gornog)
	if gs.Seats[0].Flags["gornog_cowards_cant_block_warriors"] != 1 {
		t.Errorf("Block restriction flag should be set")
	}
}

// ---------------------------------------------------------------------------
// 7. Gisa — Skeleton/Zombie anthem
// ---------------------------------------------------------------------------

func TestGisa_BuffsSkeletonsAndZombies(t *testing.T) {
	gs := newGame(t, 2)
	gisa := addPerm(gs, 0, "Gisa, the Hellraiser", "creature", "legendary")
	zomb := addPerm(gs, 0, "Festering Zombie", "creature", "zombie")
	zomb.Card.BasePower = 1
	zomb.Card.BaseToughness = 1
	skel := addPerm(gs, 0, "Bone Knight", "creature", "skeleton")
	skel.Card.BasePower = 2
	skel.Card.BaseToughness = 2
	bear := addPerm(gs, 0, "Grizzly Bears", "creature", "bear")
	bear.Card.BasePower = 2
	bear.Card.BaseToughness = 2

	gisaRefreshAnthemOnETB(gs, gisa)

	if zomb.Power() != 2 || zomb.Toughness() != 2 {
		t.Errorf("Zombie should be 2/2 (+1/+1); got %d/%d", zomb.Power(), zomb.Toughness())
	}
	if zomb.Flags["kw:menace"] != 1 {
		t.Errorf("Zombie should have kw:menace; flags=%v", zomb.Flags)
	}
	if skel.Flags["kw:menace"] != 1 {
		t.Errorf("Skeleton should have kw:menace")
	}
	if bear.Power() != 2 {
		t.Errorf("Bear should NOT be buffed")
	}
	if bear.Flags["kw:menace"] == 1 {
		t.Errorf("Bear should NOT have menace")
	}
}

// ---------------------------------------------------------------------------
// 8. Aphelia — Gorgon/Snake tribal halving
// ---------------------------------------------------------------------------

func TestAphelia_TribalHalvingActivation(t *testing.T) {
	gs := newGame(t, 2)
	aph := addPerm(gs, 0, "Aphelia, Viper Whisperer", "creature", "legendary", "snake")
	gs.Seats[0].ManaPool = 5

	apheliaActivateTribalHalving(gs, aph, 0, nil)

	if gs.Seats[0].Flags["aphelia_tribal_damage_eot_turn"] != gs.Turn+1 {
		t.Errorf("Marker should be set to turn+1 after activation; flags=%v", gs.Seats[0].Flags)
	}
}

func TestAphelia_SnakeCombatHalvesDefender(t *testing.T) {
	gs := newGame(t, 2)
	aph := addPerm(gs, 0, "Aphelia, Viper Whisperer", "creature", "legendary", "snake")
	gs.Seats[0].ManaPool = 5
	apheliaActivateTribalHalving(gs, aph, 0, nil)

	snake := addPerm(gs, 0, "Coiling Snake", "creature", "snake")
	gs.Seats[1].Life = 21

	apheliaTribalHalvingDispatch(gs, aph, map[string]interface{}{
		"attacker_perm": snake,
		"target_seat":   1,
	})

	// life=21 → loses ceil(21/2)=11 → 10
	if gs.Seats[1].Life != 10 {
		t.Errorf("Snake-damage halving should leave defender at 10 life (21 - ceil(21/2)=11); got %d", gs.Seats[1].Life)
	}
}

func TestAphelia_NonTribalCombatNoHalving(t *testing.T) {
	gs := newGame(t, 2)
	aph := addPerm(gs, 0, "Aphelia, Viper Whisperer", "creature", "legendary", "snake")
	gs.Seats[0].ManaPool = 5
	apheliaActivateTribalHalving(gs, aph, 0, nil)

	bear := addPerm(gs, 0, "Grizzly Bears", "creature", "bear")
	startLife := gs.Seats[1].Life

	apheliaTribalHalvingDispatch(gs, aph, map[string]interface{}{
		"attacker_perm": bear,
		"target_seat":   1,
	})

	if gs.Seats[1].Life != startLife {
		t.Errorf("Bear combat should NOT trigger halving; life %d → %d", startLife, gs.Seats[1].Life)
	}
}

// ---------------------------------------------------------------------------
// 9. Tovolar — Human-Werewolf transform on night flip
// ---------------------------------------------------------------------------

func TestTovolar_TransformsHumanWerewolvesOnNightFlip(t *testing.T) {
	gs := newGame(t, 2)
	tov := addPerm(gs, 0, "Tovolar, Dire Overlord", "creature", "legendary", "human", "werewolf")
	hw := addPerm(gs, 0, "Reckless Stormseeker", "creature", "human", "werewolf")
	// Mark hw as DFC so TransformPermanent doesn't no-op.
	hw.FrontFaceAST = &gameast.CardAST{}
	hw.BackFaceAST = &gameast.CardAST{}
	hw.FrontFaceName = "Reckless Stormseeker"
	hw.BackFaceName = "Stormrider Spirit"
	// Pre-flip: set is_night to simulate the hand-written upkeep handler.
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["is_night"] = 1

	tovolarTransformHumanWerewolves(gs, tov, map[string]interface{}{
		"active_seat": 0,
	})

	if !hw.Transformed {
		t.Errorf("Human Werewolf should be transformed after night flip")
	}
}

func TestTovolar_DoesNotTransformWhenDay(t *testing.T) {
	gs := newGame(t, 2)
	tov := addPerm(gs, 0, "Tovolar, Dire Overlord", "creature", "legendary")
	hw := addPerm(gs, 0, "Reckless Stormseeker", "creature", "human", "werewolf")
	// is_night not set.

	tovolarTransformHumanWerewolves(gs, tov, map[string]interface{}{
		"active_seat": 0,
	})

	if hw.Transformed {
		t.Errorf("Human Werewolf should not transform when not night")
	}
}

// ---------------------------------------------------------------------------
// 10. Samut — noncreature {X}-discount in cost_modifiers
// ---------------------------------------------------------------------------

func TestSamut_NoncreatureDiscountBySpeed(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Samut, the Driving Force", "creature", "legendary")
	if gs.Seats[0].Flags == nil {
		gs.Seats[0].Flags = map[string]int{}
	}
	gs.Seats[0].Flags["speed"] = 3

	instant := &gameengine.Card{Name: "Lightning Bolt", Owner: 0, Types: []string{"instant"}}
	mods := gameengine.ScanCostModifiers(gs, instant, 0)
	total := 0
	for _, m := range mods {
		if m.Source == "Samut, the Driving Force" {
			total += m.Amount
		}
	}
	if total != 3 {
		t.Errorf("expected Samut to discount noncreature by 3 (speed=3), got %d (mods=%+v)", total, mods)
	}
}

func TestSamut_CreatureSpellsNotDiscounted(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Samut, the Driving Force", "creature", "legendary")
	if gs.Seats[0].Flags == nil {
		gs.Seats[0].Flags = map[string]int{}
	}
	gs.Seats[0].Flags["speed"] = 4

	creature := &gameengine.Card{Name: "Bear", Owner: 0, Types: []string{"creature"}}
	mods := gameengine.ScanCostModifiers(gs, creature, 0)
	for _, m := range mods {
		if m.Source == "Samut, the Driving Force" {
			t.Errorf("Samut should not discount creature spells: %+v", m)
		}
	}
}

// ---------------------------------------------------------------------------
// Registration smoke
// ---------------------------------------------------------------------------

func TestBatchHR50_AllHandlersRegistered(t *testing.T) {
	customs := []struct {
		card  string
		event string // "etb" or "trigger:<event>"
	}{
		{"Mabel, Heir to Cragflame", "etb"},
		{"Commander Mustard", "etb"},
		{"Rienne, Angel of Rebirth", "etb"},
		{"Kolodin, Triumph Caster", "etb"},
		{"Gornog, the Red Reaper", "etb"},
		{"Jodah, the Unifier", "etb"},
		{"Gisa, the Hellraiser", "etb"},
		{"Aphelia, Viper Whisperer", "trigger:creature_combat_damage_to_player"},
		{"Tovolar, Dire Overlord", "trigger:upkeep_controller"},
	}
	for _, c := range customs {
		var ok bool
		if c.event == "etb" {
			ok = HasETB(c.card)
		} else {
			ok = HasTrigger(c.card, c.event[len("trigger:"):])
		}
		if !ok {
			t.Errorf("batchH card %q: missing registration for %q", c.card, c.event)
		}
	}
}
