package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// -----------------------------------------------------------------------------
// Batch I (R60) — tests for 5 new handlers
// -----------------------------------------------------------------------------

// -----------------------------------------------------------------------------
// Esper Sentinel
// -----------------------------------------------------------------------------

func TestEsperSentinel_FirstNoncreatureSpellOpponentDraws(t *testing.T) {
	gs := newGame(t, 2)
	sentinel := addPerm(gs, 0, "Esper Sentinel", "creature", "artifact")
	sentinel.Card.BasePower = 1
	sentinel.Card.BaseToughness = 1
	gs.Seats[1].ManaPool = 0
	addLibrary(gs, 0, "A", "B", "C")

	// Record a noncreature cast for seat 1 so seat.Turn.Casts has it.
	gs.Seats[1].Turn.Casts = append(gs.Seats[1].Turn.Casts, gameengine.CastRecord{
		CardName: "Ponder",
		Types:    []string{"sorcery"},
	})

	gameengine.FireCardTrigger(gs, "noncreature_spell_cast", map[string]interface{}{
		"caster_seat": 1,
		"spell_name":  "Ponder",
		"is_creature": false,
	})

	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected Sentinel controller to draw when opponent can't pay {1}, hand=%d",
			len(gs.Seats[0].Hand))
	}
}

func TestEsperSentinel_OpponentPaysTax(t *testing.T) {
	gs := newGame(t, 2)
	sentinel := addPerm(gs, 0, "Esper Sentinel", "creature", "artifact")
	sentinel.Card.BasePower = 1
	sentinel.Card.BaseToughness = 1
	gs.Seats[1].ManaPool = 3 // can afford
	addLibrary(gs, 0, "A", "B", "C")

	gs.Seats[1].Turn.Casts = append(gs.Seats[1].Turn.Casts, gameengine.CastRecord{
		CardName: "Ponder",
		Types:    []string{"sorcery"},
	})

	gameengine.FireCardTrigger(gs, "noncreature_spell_cast", map[string]interface{}{
		"caster_seat": 1,
		"spell_name":  "Ponder",
		"is_creature": false,
	})

	if gs.Seats[1].ManaPool != 2 {
		t.Errorf("expected opponent to pay {1}, mana now %d", gs.Seats[1].ManaPool)
	}
	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("Sentinel should NOT draw when opponent paid, hand=%d", len(gs.Seats[0].Hand))
	}
}

func TestEsperSentinel_OnlyFirstNoncreatureFires(t *testing.T) {
	gs := newGame(t, 2)
	sentinel := addPerm(gs, 0, "Esper Sentinel", "creature", "artifact")
	sentinel.Card.BasePower = 1
	sentinel.Card.BaseToughness = 1
	gs.Seats[1].ManaPool = 0
	addLibrary(gs, 0, "A", "B", "C")

	// Record TWO prior noncreature casts already this turn — second cast
	// should not fire Sentinel.
	gs.Seats[1].Turn.Casts = []gameengine.CastRecord{
		{CardName: "Ponder", Types: []string{"sorcery"}},
		{CardName: "Brainstorm", Types: []string{"instant"}},
	}

	gameengine.FireCardTrigger(gs, "noncreature_spell_cast", map[string]interface{}{
		"caster_seat": 1,
		"spell_name":  "Brainstorm",
		"is_creature": false,
	})

	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("Sentinel should not fire on second noncreature cast, hand=%d",
			len(gs.Seats[0].Hand))
	}
}

func TestEsperSentinel_OwnCastsIgnored(t *testing.T) {
	gs := newGame(t, 2)
	sentinel := addPerm(gs, 0, "Esper Sentinel", "creature", "artifact")
	sentinel.Card.BasePower = 1
	sentinel.Card.BaseToughness = 1
	addLibrary(gs, 0, "A", "B", "C")

	gs.Seats[0].Turn.Casts = append(gs.Seats[0].Turn.Casts, gameengine.CastRecord{
		CardName: "Ponder", Types: []string{"sorcery"},
	})

	gameengine.FireCardTrigger(gs, "noncreature_spell_cast", map[string]interface{}{
		"caster_seat": 0,
		"spell_name":  "Ponder",
		"is_creature": false,
	})

	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("Sentinel should never fire on own casts, hand=%d", len(gs.Seats[0].Hand))
	}
}

// -----------------------------------------------------------------------------
// Up the Beanstalk
// -----------------------------------------------------------------------------

func TestUpTheBeanstalk_ETBDrawsCard(t *testing.T) {
	gs := newGame(t, 2)
	addLibrary(gs, 0, "A", "B")
	bs := addPerm(gs, 0, "Up the Beanstalk", "enchantment")

	gameengine.InvokeETBHook(gs, bs)

	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected ETB to draw 1, hand=%d", len(gs.Seats[0].Hand))
	}
}

func TestUpTheBeanstalk_FiresOnCMC5Cast(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Up the Beanstalk", "enchantment")
	addLibrary(gs, 0, "A", "B", "C")

	bigSpell := &gameengine.Card{
		Name:  "Cyclonic Rift",
		Owner: 0,
		Types: []string{"instant"},
		// EffectiveCMC reads BaseCost; cheat by setting it directly via Types
		// the engine's EffectiveCMC consults card.BaseCost first.
		CMC: 6,
	}

	gameengine.FireCardTrigger(gs, "spell_cast", map[string]interface{}{
		"caster_seat": 0,
		"spell_name":  bigSpell.DisplayName(),
		"card":        bigSpell,
		"is_creature": false,
	})

	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected Beanstalk to draw on CMC 6 cast, hand=%d", len(gs.Seats[0].Hand))
	}
}

func TestUpTheBeanstalk_IgnoresCMC4Cast(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Up the Beanstalk", "enchantment")
	addLibrary(gs, 0, "A", "B", "C")

	smallSpell := &gameengine.Card{
		Name:     "Wrath of God",
		Owner:    0,
		Types:    []string{"sorcery"},
		CMC: 4,
	}

	gameengine.FireCardTrigger(gs, "spell_cast", map[string]interface{}{
		"caster_seat": 0,
		"card":        smallSpell,
		"is_creature": false,
	})

	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("Beanstalk should NOT fire on CMC < 5, hand=%d", len(gs.Seats[0].Hand))
	}
}

func TestUpTheBeanstalk_IgnoresOpponentCast(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Up the Beanstalk", "enchantment")
	addLibrary(gs, 0, "A", "B", "C")

	bigSpell := &gameengine.Card{
		Name:     "Cyclonic Rift",
		Owner:    1,
		Types:    []string{"instant"},
		CMC: 6,
	}

	gameengine.FireCardTrigger(gs, "spell_cast", map[string]interface{}{
		"caster_seat": 1,
		"card":        bigSpell,
	})

	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("Beanstalk should NOT fire on opponent's cast, hand=%d", len(gs.Seats[0].Hand))
	}
}

// -----------------------------------------------------------------------------
// Grim Tutor
// -----------------------------------------------------------------------------

func TestGrimTutor_FindsCardAndCosts3Life(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Life = 40
	addLibrary(gs, 0, "A", "B", "C", "D", "E")

	card := addCard(gs, 0, "Grim Tutor", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected 1 card tutored to hand, got %d", len(gs.Seats[0].Hand))
	}
	if gs.Seats[0].Life != 37 {
		t.Errorf("expected life 37 after paying 3, got %d", gs.Seats[0].Life)
	}
}

func TestGrimTutor_EmptyLibrarySafe(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Life = 40

	card := addCard(gs, 0, "Grim Tutor", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if gs.Seats[0].Life != 37 {
		t.Errorf("expected 3 life paid even on empty library, got %d", gs.Seats[0].Life)
	}
	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("empty library → no tutor, hand=%d", len(gs.Seats[0].Hand))
	}
}

// -----------------------------------------------------------------------------
// Scroll Rack
// -----------------------------------------------------------------------------

func TestScrollRack_SwapsLandsForFreshCards(t *testing.T) {
	gs := newGame(t, 2)
	rack := addPerm(gs, 0, "Scroll Rack", "artifact")
	gs.Seats[0].ManaPool = 1

	// Hand: 3 lands (low keep-score) + 1 spell (high).
	land1 := &gameengine.Card{Name: "Forest", Owner: 0, Types: []string{"land"}}
	land2 := &gameengine.Card{Name: "Mountain", Owner: 0, Types: []string{"land"}}
	land3 := &gameengine.Card{Name: "Plains", Owner: 0, Types: []string{"land"}}
	spell := &gameengine.Card{Name: "Bolt", Owner: 0, Types: []string{"instant", "cmc:4"}, CMC: 4}
	gs.Seats[0].Hand = []*gameengine.Card{land1, land2, land3, spell}

	// Library: 5 fresh cards.
	for _, n := range []string{"fresh1", "fresh2", "fresh3", "fresh4", "fresh5"} {
		c := &gameengine.Card{Name: n, Owner: 0, Types: []string{"sorcery"}, CMC: 4}
		gs.Seats[0].Library = append(gs.Seats[0].Library, c)
	}

	gameengine.InvokeActivatedHook(gs, rack, 0, nil)

	if !rack.Tapped {
		t.Errorf("expected Scroll Rack tapped after activation")
	}
	if gs.Seats[0].ManaPool != 0 {
		t.Errorf("expected mana spent on {1}, pool=%d", gs.Seats[0].ManaPool)
	}
	// Lands should not be in hand anymore (they were exiled then top-libraried).
	for _, c := range gs.Seats[0].Hand {
		if c == land1 || c == land2 || c == land3 {
			t.Errorf("land %s should not be in hand post-Scroll-Rack", c.Name)
		}
	}
	// Spell stays in hand.
	foundSpell := false
	for _, c := range gs.Seats[0].Hand {
		if c == spell {
			foundSpell = true
			break
		}
	}
	if !foundSpell {
		t.Errorf("high-quality spell should remain in hand")
	}
	// At least one fresh card came into hand.
	freshInHand := 0
	for _, c := range gs.Seats[0].Hand {
		for _, n := range []string{"fresh1", "fresh2", "fresh3", "fresh4", "fresh5"} {
			if c.Name == n {
				freshInHand++
			}
		}
	}
	if freshInHand == 0 {
		t.Errorf("expected at least one fresh card from library top in hand")
	}
}

func TestScrollRack_AlreadyTappedFails(t *testing.T) {
	gs := newGame(t, 2)
	rack := addPerm(gs, 0, "Scroll Rack", "artifact")
	rack.Tapped = true
	gs.Seats[0].ManaPool = 1
	gs.Seats[0].Hand = []*gameengine.Card{
		{Name: "Forest", Owner: 0, Types: []string{"land"}},
	}
	addLibrary(gs, 0, "a")

	gameengine.InvokeActivatedHook(gs, rack, 0, nil)

	if hasEvent(gs, "per_card_failed") < 1 {
		t.Errorf("expected per_card_failed when already tapped")
	}
	if gs.Seats[0].ManaPool != 1 {
		t.Errorf("mana should not be spent on failed activation, pool=%d", gs.Seats[0].ManaPool)
	}
}

func TestScrollRack_NoManaFails(t *testing.T) {
	gs := newGame(t, 2)
	rack := addPerm(gs, 0, "Scroll Rack", "artifact")
	gs.Seats[0].ManaPool = 0
	gs.Seats[0].Hand = []*gameengine.Card{
		{Name: "Forest", Owner: 0, Types: []string{"land"}},
	}
	addLibrary(gs, 0, "a")

	gameengine.InvokeActivatedHook(gs, rack, 0, nil)

	if rack.Tapped {
		t.Errorf("Rack should not tap when {1} cannot be paid")
	}
	if hasEvent(gs, "per_card_failed") < 1 {
		t.Errorf("expected per_card_failed on insufficient mana")
	}
}

// -----------------------------------------------------------------------------
// Trouble in Pairs
// -----------------------------------------------------------------------------

func TestTroubleInPairs_FiresOnSecondOpponentDraw(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Trouble in Pairs", "enchantment")
	addLibrary(gs, 0, "A", "B", "C")

	gameengine.FireCardTrigger(gs, "card_drawn", map[string]interface{}{
		"seat":          1,
		"drawer_seat":   1,
		"nth_this_turn": 2,
		"source":        "draw",
	})

	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected Trouble to draw on 2nd opponent draw, hand=%d", len(gs.Seats[0].Hand))
	}
}

func TestTroubleInPairs_IgnoresFirstDraw(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Trouble in Pairs", "enchantment")
	addLibrary(gs, 0, "A", "B")

	gameengine.FireCardTrigger(gs, "card_drawn", map[string]interface{}{
		"drawer_seat":   1,
		"nth_this_turn": 1,
	})

	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("Trouble should not fire on first draw, hand=%d", len(gs.Seats[0].Hand))
	}
}

func TestTroubleInPairs_FiresOnSecondOpponentCast(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Trouble in Pairs", "enchantment")
	addLibrary(gs, 0, "A", "B", "C")
	gs.Seats[1].Turn.SpellsCast = 2

	gameengine.FireCardTrigger(gs, "spell_cast", map[string]interface{}{
		"caster_seat": 1,
		"spell_name":  "X",
	})

	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected Trouble to draw on opponent's 2nd cast, hand=%d", len(gs.Seats[0].Hand))
	}
}

func TestTroubleInPairs_OwnDrawIgnored(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Trouble in Pairs", "enchantment")
	addLibrary(gs, 0, "A", "B")

	gameengine.FireCardTrigger(gs, "card_drawn", map[string]interface{}{
		"drawer_seat":   0,
		"nth_this_turn": 2,
	})

	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("Trouble must only fire on opponent draws, hand=%d", len(gs.Seats[0].Hand))
	}
}

func TestTroubleInPairs_FiresOnTwoAttackers(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Trouble in Pairs", "enchantment")
	addLibrary(gs, 0, "A", "B", "C")

	// Seat 1 declares 2 attackers, both targeting seat 0.
	atk1 := addPerm(gs, 1, "Grizzly Bears", "creature")
	atk2 := addPerm(gs, 1, "Llanowar Elves", "creature")
	atk1.Flags["attacking"] = 1
	atk2.Flags["attacking"] = 1
	gameengine.SetAttackerDefender(atk1, 0)
	gameengine.SetAttackerDefender(atk2, 0)

	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": atk2, // 2nd attacker fires the trigger
		"attacker_seat": 1,
		"attacker_card": atk2.Card,
	})

	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected Trouble to draw on 2+ attackers, hand=%d", len(gs.Seats[0].Hand))
	}
}

func TestTroubleInPairs_OneAttackerDoesNotFire(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Trouble in Pairs", "enchantment")
	addLibrary(gs, 0, "A", "B")

	atk1 := addPerm(gs, 1, "Grizzly Bears", "creature")
	atk1.Flags["attacking"] = 1
	gameengine.SetAttackerDefender(atk1, 0)

	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": atk1,
		"attacker_seat": 1,
		"attacker_card": atk1.Card,
	})

	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("Trouble should not fire on single attacker, hand=%d", len(gs.Seats[0].Hand))
	}
}

func TestTroubleInPairs_AttackTriggerFiresOnce(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Trouble in Pairs", "enchantment")
	addLibrary(gs, 0, "A", "B", "C", "D")

	atk1 := addPerm(gs, 1, "Grizzly Bears", "creature")
	atk2 := addPerm(gs, 1, "Llanowar Elves", "creature")
	atk3 := addPerm(gs, 1, "Wolf Token", "creature")
	for _, a := range []*gameengine.Permanent{atk1, atk2, atk3} {
		a.Flags["attacking"] = 1
		gameengine.SetAttackerDefender(a, 0)
	}

	// Fire creature_attacks for each attacker; only the first should
	// actually draw, gated by the per-turn fire-once flag.
	for _, a := range []*gameengine.Permanent{atk1, atk2, atk3} {
		gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
			"attacker_perm": a,
			"attacker_seat": 1,
			"attacker_card": a.Card,
		})
	}

	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("Trouble attack arm should fire at most once per combat, hand=%d",
			len(gs.Seats[0].Hand))
	}
}

// -----------------------------------------------------------------------------
// Registry smoke
// -----------------------------------------------------------------------------

func TestBatchIR60_AllRegistered(t *testing.T) {
	if !HasTrigger("Esper Sentinel", "noncreature_spell_cast") {
		t.Errorf("Esper Sentinel trigger not registered")
	}
	if !HasETB("Up the Beanstalk") {
		t.Errorf("Up the Beanstalk ETB not registered")
	}
	if !HasTrigger("Up the Beanstalk", "spell_cast") {
		t.Errorf("Up the Beanstalk cast trigger not registered")
	}
	if !HasResolve("Grim Tutor") {
		t.Errorf("Grim Tutor Resolve not registered")
	}
	if !HasActivated("Scroll Rack") {
		t.Errorf("Scroll Rack Activated not registered")
	}
	if !HasTrigger("Trouble in Pairs", "card_drawn") {
		t.Errorf("Trouble in Pairs draw trigger not registered")
	}
	if !HasTrigger("Trouble in Pairs", "spell_cast") {
		t.Errorf("Trouble in Pairs cast trigger not registered")
	}
	if !HasTrigger("Trouble in Pairs", "creature_attacks") {
		t.Errorf("Trouble in Pairs attack trigger not registered")
	}
}
