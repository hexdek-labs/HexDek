package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// attack_trigger_dispatch_r63_test.go — the PROGRESSION dimension's
// deferred dispatch-consistency audit: the parser drops attack-trigger
// actor phrases, and the old two-substring ally whitelist left whole
// classes silent (attached "equipped/enchanted creature attacks",
// any-controller "a creature attacks", filtered/subtype ally attacks).
// Pins the classifier + the additive dispatch.

func gainOneTrigger(raw string) *gameast.Triggered {
	return &gameast.Triggered{
		Trigger: gameast.Trigger{Event: "attack"},
		Effect: &gameast.GainLife{
			Amount: gameast.NumberOrRef{IsInt: true, Int: 1},
			Target: gameast.Filter{Base: "controller"},
		},
		Raw:     raw,
	}
}

// addBearer places a non-creature bearer (enchantment/equipment) with
// the given attack trigger on seat's battlefield.
func addBearer(gs *GameState, seat int, name string, types []string, trig *gameast.Triggered) *Permanent {
	card := &Card{
		AST:   &gameast.CardAST{Name: name, Abilities: []gameast.Ability{trig}},
		Name:  name,
		Owner: seat,
		Types: types,
	}
	p := &Permanent{
		Card:       card,
		Controller: seat,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

func addVanillaAttacker(gs *GameState, seat int, name string, types ...string) *Permanent {
	card := &Card{
		Name: name, Owner: seat,
		BasePower: 2, BaseToughness: 2,
		Types: append([]string{"creature"}, types...),
	}
	p := &Permanent{
		Card:       card,
		Controller: seat,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

func TestAttackTrigger_EquippedCreatureFires(t *testing.T) {
	gs := newCombatGame(t)
	atk := addVanillaAttacker(gs, 0, "Bearer Bear")
	eq := addBearer(gs, 0, "Test Blade", []string{"artifact", "equipment"},
		gainOneTrigger("Whenever equipped creature attacks, you gain 1 life."))
	eq.AttachedTo = atk

	before := gs.Seats[0].Life
	fireAttackTriggers(gs, 0, []*Permanent{atk})
	if got := gs.Seats[0].Life - before; got != 1 {
		t.Fatalf("equipped-creature trigger fired %d times, want 1 (was: NEVER fired pre-fix)", got)
	}

	// Unattached equipment must not fire on someone else's attack.
	eq.AttachedTo = nil
	other := addVanillaAttacker(gs, 0, "Other Bear")
	before = gs.Seats[0].Life
	fireAttackTriggers(gs, 0, []*Permanent{other})
	if gs.Seats[0].Life != before {
		t.Fatalf("detached equipment fired on an unrelated attacker")
	}
}

func TestAttackTrigger_EnchantedCreatureOnOpponentsAttacker(t *testing.T) {
	gs := newCombatGame(t)
	// Seat 1 controls the aura, enchanting SEAT 0's creature — the
	// aura's controller gets the trigger when that creature attacks.
	atk := addVanillaAttacker(gs, 0, "Enchanted Bear")
	aura := addBearer(gs, 1, "Test Arms", []string{"enchantment", "aura"},
		gainOneTrigger("Whenever enchanted creature attacks, you gain 1 life."))
	aura.AttachedTo = atk

	before := gs.Seats[1].Life
	fireAttackTriggers(gs, 0, []*Permanent{atk})
	if got := gs.Seats[1].Life - before; got != 1 {
		t.Fatalf("cross-seat enchanted-creature trigger fired %d times, want 1", got)
	}
}

func TestAttackTrigger_AnyCreatureFiresForDefendersBearer(t *testing.T) {
	gs := newCombatGame(t)
	// Ondu Rising shape on the DEFENDING seat: "Whenever a creature
	// attacks," fires for any controller's attack.
	addBearer(gs, 1, "Test Rising", []string{"enchantment"},
		gainOneTrigger("Whenever a creature attacks, you gain 1 life."))
	atk := addVanillaAttacker(gs, 0, "Opposing Bear")

	before := gs.Seats[1].Life
	fireAttackTriggers(gs, 0, []*Permanent{atk})
	if got := gs.Seats[1].Life - before; got != 1 {
		t.Fatalf("any-creature trigger fired %d times for opponent's attack, want 1", got)
	}
}

func TestAttackTrigger_SubtypeAllyFilter(t *testing.T) {
	gs := newCombatGame(t)
	addBearer(gs, 0, "Test Hellkite Caller", []string{"enchantment"},
		gainOneTrigger("Whenever a Dragon you control attacks, you gain 1 life."))
	dragon := addVanillaAttacker(gs, 0, "Wee Dragon", "dragon")
	bear := addVanillaAttacker(gs, 0, "Plain Bear")

	before := gs.Seats[0].Life
	fireAttackTriggers(gs, 0, []*Permanent{dragon, bear})
	if got := gs.Seats[0].Life - before; got != 1 {
		t.Fatalf("dragon-filter trigger fired %d times for [dragon, bear], want exactly 1", got)
	}
}

func TestAttackTrigger_FilterSuffixes(t *testing.T) {
	gs := newCombatGame(t)
	addBearer(gs, 0, "Test Bombardment", []string{"enchantment"},
		gainOneTrigger("Whenever a creature you control with power 2 or less attacks, you gain 1 life."))
	small := addVanillaAttacker(gs, 0, "Small Bear") // power 2
	big := addVanillaAttacker(gs, 0, "Big Bear")
	big.Card.BasePower = 5

	before := gs.Seats[0].Life
	fireAttackTriggers(gs, 0, []*Permanent{small, big})
	if got := gs.Seats[0].Life - before; got != 1 {
		t.Fatalf("power<=2 trigger fired %d times for [2-power, 5-power], want 1", got)
	}

	gs2 := newCombatGame(t)
	addBearer(gs2, 0, "Test Forger", []string{"enchantment"},
		gainOneTrigger("Whenever a creature you control with a +1/+1 counter on it attacks, you gain 1 life."))
	counted := addVanillaAttacker(gs2, 0, "Counted Bear")
	counted.Counters["+1/+1"] = 1
	plain := addVanillaAttacker(gs2, 0, "Plain Bear")

	before = gs2.Seats[0].Life
	fireAttackTriggers(gs2, 0, []*Permanent{counted, plain})
	if got := gs2.Seats[0].Life - before; got != 1 {
		t.Fatalf("counter-filter trigger fired %d times, want 1", got)
	}
}

func TestAttackTrigger_PerCardGuardSkipsNewClasses(t *testing.T) {
	gs := newCombatGame(t)
	addBearer(gs, 0, "Guarded Caller", []string{"enchantment"},
		gainOneTrigger("Whenever a Dragon you control attacks, you gain 1 life."))
	dragon := addVanillaAttacker(gs, 0, "Wee Dragon", "dragon")

	oldHook := HasTriggerHook
	HasTriggerHook = func(cardName, event string) bool {
		return cardName == "Guarded Caller" && event == "creature_attacks"
	}
	defer func() { HasTriggerHook = oldHook }()

	before := gs.Seats[0].Life
	fireAttackTriggers(gs, 0, []*Permanent{dragon})
	if gs.Seats[0].Life != before {
		t.Fatalf("new-class AST dispatch fired despite a registered per_card handler (double-fire)")
	}
}

func TestAttackTrigger_LegacyAllyWordingUnchanged(t *testing.T) {
	gs := newCombatGame(t)
	// The historical pool-2 wording must keep firing — including for
	// bearers with per_card handlers (legacy bypasses the guard).
	addBearer(gs, 0, "Test Ascension", []string{"enchantment"},
		gainOneTrigger("Whenever a creature you control attacks, you gain 1 life."))
	atk := addVanillaAttacker(gs, 0, "Ally Bear")

	oldHook := HasTriggerHook
	HasTriggerHook = func(cardName, event string) bool { return true }
	defer func() { HasTriggerHook = oldHook }()

	before := gs.Seats[0].Life
	fireAttackTriggers(gs, 0, []*Permanent{atk})
	if got := gs.Seats[0].Life - before; got != 1 {
		t.Fatalf("legacy ally wording fired %d times, want 1 (behavior must be unchanged)", got)
	}
}

func TestClassifyAttackTrigger_FailClosed(t *testing.T) {
	cases := []struct {
		raw  string
		want attackTriggerClass
	}{
		{"Whenever a creature attacks you, destroy it.", atkUnknown},
		{"Whenever a creature attacks alone, it gets +2/+2.", atkUnknown}, // no comma directly after "attacks"
		{"Whenever a creature attacks one of your opponents, draw a card.", atkUnknown},
		{"Whenever a zombie token you control with power 6 or greater attacks, draw.", atkUnknown},
		{"Whenever a modified creature you control attacks, draw a card.", atkUnknown},
		{"Whenever a creature attacks, put a +1/+1 counter on it.", atkAnyCreature},
		{"Whenever equipped creature attacks, it gets +1/+0.", atkAttached},
		{"Whenever a Wolf you control attacks, you gain 1 life.", atkAllyYouControl},
		{"Whenever another creature you control attacks, it gains trample.", atkAllyYouControl},
		{"Whenever a creature you control with power 4 or greater attacks, draw a card.", atkAllyYouControl},
	}
	for _, tc := range cases {
		got := classifyAttackTrigger(tc.raw)
		if got.class != tc.want {
			t.Errorf("classify(%q) = %v, want %v", tc.raw, got.class, tc.want)
		}
	}
	// "attacks alone" with classifiable prefix must NOT classify.
	if c := classifyAttackTrigger("Whenever a Dragon you control attacks alone, it gets +2/+2."); c.class != atkUnknown {
		t.Errorf("attacks-alone rider classified as %v, want unknown", c.class)
	}
}
