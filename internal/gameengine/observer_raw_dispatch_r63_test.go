package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// observer_raw_dispatch_r63_test.go — the PROGRESSION widening's engine
// findings:
//
//  1. Observer ETB/cast dispatch was DEAD for nil-Actor triggers (the
//     parser drops actor phrases; isSelfTrigger treated nil-actor as
//     self and observerETBMatches required a non-nil Actor) — Ajani's
//     Welcome never gained life, Archmage of Runes never drew.
//  2. Begin-combat triggers parse to Phase "combat_start_yours"/"_each",
//     which the old isCombatBeginTrigger never matched — all 228 were
//     silent; "_each" must also fire on opponents' combats.
//  3. With the AST observer-cast dispatch live, the hardcoded
//     Young Pyromancer / Third Path Iconoclast / Monastery Mentor
//     inline arms double-fired — deleted (Wave-1b fold), pinned to
//     exactly one token.

func obsGame(t *testing.T) *GameState {
	t.Helper()
	gs := NewGameState(2, rand.New(rand.NewSource(63)), nil)
	return gs
}

func obsPerm(gs *GameState, seat int, name string, types []string, trig *gameast.Triggered) *Permanent {
	card := &Card{Name: name, Owner: seat, Types: types, BasePower: 2, BaseToughness: 2}
	if trig != nil {
		card.AST = &gameast.CardAST{Name: name, Abilities: []gameast.Ability{trig}}
	}
	p := &Permanent{Card: card, Controller: seat, Owner: seat,
		Timestamp: gs.NextTimestamp(), Counters: map[string]int{}, Flags: map[string]int{}}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

func gainOneAST(raw string, event string) *gameast.Triggered {
	return &gameast.Triggered{
		Trigger: gameast.Trigger{Event: event},
		Effect: &gameast.GainLife{
			Amount: gameast.NumberOrRef{IsInt: true, Int: 1},
			Target: gameast.Filter{Base: "controller"},
		},
		Raw: raw,
	}
}

func TestObserverETB_AllyTriggerFires(t *testing.T) {
	gs := obsGame(t)
	obsPerm(gs, 0, "Test Welcome", []string{"enchantment"},
		gainOneAST("whenever a creature you control enters, you gain 1 life", "tribe_you_control_etb"))

	// FIRE: own creature enters.
	ally := obsPerm(gs, 0, "Ally Bear", []string{"creature"}, nil)
	life0 := gs.Seats[0].Life
	FirePermanentETBTriggers(gs, ally)
	if got := gs.Seats[0].Life - life0; got != 1 {
		t.Fatalf("ally ETB fired %d times, want 1 (was: NEVER fired pre-fix)", got)
	}

	// CONTROLLER GATE: opponent's creature enters — silent.
	opp := obsPerm(gs, 1, "Opp Bear", []string{"creature"}, nil)
	life0 = gs.Seats[0].Life
	FirePermanentETBTriggers(gs, opp)
	if gs.Seats[0].Life != life0 {
		t.Fatalf("ally-ETB trigger fired for an opponent's creature")
	}
}

func TestObserverETB_AnotherExcludesSelfAndTypeGates(t *testing.T) {
	gs := obsGame(t)
	bearer := obsPerm(gs, 0, "Test Lord", []string{"creature"},
		gainOneAST("whenever another zombie creature you control enters, you gain 1 life", "another_typed_enters"))

	// Self entering must not fire ("another").
	life0 := gs.Seats[0].Life
	FirePermanentETBTriggers(gs, bearer)
	if gs.Seats[0].Life != life0 {
		t.Fatalf("'another' trigger fired on the bearer's own ETB")
	}

	// Non-zombie ally: silent (type gate).
	bear := obsPerm(gs, 0, "Plain Bear", []string{"creature"}, nil)
	FirePermanentETBTriggers(gs, bear)
	if gs.Seats[0].Life != life0 {
		t.Fatalf("typed trigger fired for a non-matching creature")
	}

	// Zombie ally: fires.
	zombie := obsPerm(gs, 0, "Shambler", []string{"creature", "zombie"}, nil)
	FirePermanentETBTriggers(gs, zombie)
	if got := gs.Seats[0].Life - life0; got != 1 {
		t.Fatalf("zombie-typed ally ETB fired %d times, want 1", got)
	}
}

func castInstantFrom(gs *GameState, seat int) {
	c := &Card{Name: "Test Trick", Owner: seat, Types: []string{"instant"}}
	gs.Seats[seat].Hand = append(gs.Seats[seat].Hand, c)
	_ = CastSpell(gs, seat, c, nil)
}

func TestObserverCast_FilterAndCasterScopes(t *testing.T) {
	gs := obsGame(t)
	obsPerm(gs, 0, "Test Archmage", []string{"creature"},
		gainOneAST("whenever you cast an instant or sorcery spell, you gain 1 life", "cast_filtered"))

	// FIRE: own instant.
	life0 := gs.Seats[0].Life
	castInstantFrom(gs, 0)
	if got := gs.Seats[0].Life - life0; got != 1 {
		t.Fatalf("iss cast trigger fired %d times, want 1 (was: NEVER fired pre-fix)", got)
	}

	// CASTER GATE: opponent's instant — silent.
	life0 = gs.Seats[0].Life
	castInstantFrom(gs, 1)
	if gs.Seats[0].Life != life0 {
		t.Fatalf("'you cast' trigger fired for an opponent's cast")
	}

	// FILTER GATE: own creature spell — silent.
	life0 = gs.Seats[0].Life
	cc := &Card{Name: "Test Critter", Owner: 0, Types: []string{"creature"}, BasePower: 1, BaseToughness: 1}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, cc)
	_ = CastSpell(gs, 0, cc, nil)
	if gs.Seats[0].Life != life0 {
		t.Fatalf("iss-filtered trigger fired for a creature spell")
	}
}

func TestObserverCast_OpponentScopeFires(t *testing.T) {
	gs := obsGame(t)
	obsPerm(gs, 0, "Test Eternal", []string{"creature"},
		gainOneAST("whenever an opponent casts a spell, you gain 1 life", "opp_cast"))

	life0 := gs.Seats[0].Life
	castInstantFrom(gs, 1)
	if got := gs.Seats[0].Life - life0; got != 1 {
		t.Fatalf("opp-cast trigger fired %d times for opponent's cast, want 1", got)
	}
	life0 = gs.Seats[0].Life
	castInstantFrom(gs, 0)
	if gs.Seats[0].Life != life0 {
		t.Fatalf("opp-cast trigger fired for the controller's own cast")
	}
}

func TestObserverCast_PerCardGuardNoDoubleFire(t *testing.T) {
	gs := obsGame(t)
	obsPerm(gs, 0, "Guarded Caster", []string{"creature"},
		gainOneAST("whenever you cast an instant or sorcery spell, you gain 1 life", "cast_filtered"))

	oldHook := HasTriggerHook
	HasTriggerHook = func(cardName, event string) bool {
		return cardName == "Guarded Caster" && event == "instant_or_sorcery_cast"
	}
	defer func() { HasTriggerHook = oldHook }()

	life0 := gs.Seats[0].Life
	castInstantFrom(gs, 0)
	if gs.Seats[0].Life != life0 {
		t.Fatalf("AST observer-cast dispatch fired despite a registered per_card handler (double-fire)")
	}
}

func TestCombatBegin_YoursAndEachScopes(t *testing.T) {
	mk := func(phase string) (*GameState, int) {
		gs := obsGame(t)
		obsPerm(gs, 0, "Test Herald", []string{"enchantment"},
			&gameast.Triggered{
				Trigger: gameast.Trigger{Event: "phase", Phase: phase},
				Effect: &gameast.GainLife{
					Amount: gameast.NumberOrRef{IsInt: true, Int: 1},
					Target: gameast.Filter{Base: "controller"},
				},
				Raw: "at the beginning of combat, you gain 1 life",
			})
		return gs, gs.Seats[0].Life
	}

	// "_yours": fires on own combat only (was: NEVER fired pre-fix).
	gs, life0 := mk("combat_start_yours")
	FireBeginCombatTriggersForTest(gs, 0)
	if got := gs.Seats[0].Life - life0; got != 1 {
		t.Fatalf("combat_start_yours fired %d times on own combat, want 1", got)
	}
	gs2, life2 := mk("combat_start_yours")
	FireBeginCombatTriggersForTest(gs2, 1)
	if gs2.Seats[0].Life != life2 {
		t.Fatalf("combat_start_yours fired on the opponent's combat")
	}

	// "_each": fires on both.
	gs3, life3 := mk("combat_start_each")
	FireBeginCombatTriggersForTest(gs3, 1)
	if got := gs3.Seats[0].Life - life3; got != 1 {
		t.Fatalf("combat_start_each fired %d times on opponent's combat, want 1", got)
	}
}

// TestYoungPyromancerSingleToken pins the Wave-1b inline-arm deletion:
// the AST observer dispatch is now the ONE token source per cast.
func TestYoungPyromancerSingleToken(t *testing.T) {
	gs := obsGame(t)
	obsPerm(gs, 0, "Young Pyromancer", []string{"creature"},
		&gameast.Triggered{
			Trigger: gameast.Trigger{Event: "cast_filtered"},
			Effect: &gameast.CreateToken{
				Count: gameast.NumberOrRef{IsInt: true, Int: 1},
				PT:    &[2]int{1, 1},
				Types: []string{"elemental"},
			},
			Raw: "whenever you cast an instant or sorcery spell, create a 1/1 red elemental creature token",
		})

	before := len(gs.Seats[0].Battlefield)
	castInstantFrom(gs, 0)
	tokens := len(gs.Seats[0].Battlefield) - before
	if tokens != 1 {
		t.Fatalf("Young Pyromancer cast made %d tokens, want exactly 1 (inline arm + AST dispatch double-fired pre-fix)", tokens)
	}
}
