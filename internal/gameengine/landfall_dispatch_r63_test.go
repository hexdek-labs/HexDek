package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// landfall_dispatch_r63_test.go — mechanic-probe (LANDFALL, CR §603.6 land
// ETB). Generic-AST landfall payoffs (Lotus Cobra, Scute Swarm, Tireless
// Provisioner — every landfall card WITHOUT a per_card handler) were INERT:
// Thor parses "Landfall — Whenever a land you control enters, …" as a
// Static{ability_word → Triggered} wrapper, which the observer-ETB scan
// (type-asserting top-level *Triggered) never reached. Fixed by unwrapping
// the landfall ability word in observerTriggeredFromAbility + a land-scoped
// match. These pin each property.

func lfGame(t *testing.T) *GameState {
	t.Helper()
	return NewGameState(2, rand.New(rand.NewSource(99)), nil)
}

// addLandfallPayoff puts an observer on `seat` whose AST carries the
// landfall ability-word wrapper (gain 1 life to its controller on each land
// you control entering) — the exact shape Thor emits for Lotus Cobra et al.
func addLandfallPayoff(gs *GameState, seat int, name string) *Permanent {
	inner := &gameast.Triggered{
		Trigger: gameast.Trigger{Event: "tribe_you_control_etb"},
		Effect: &gameast.GainLife{
			Amount: gameast.NumberOrRef{IsInt: true, Int: 1},
			Target: gameast.Filter{Base: "controller"},
		},
		Raw: "landfall — whenever a land you control enters, you gain 1 life",
	}
	card := &Card{Name: name, Owner: seat, Types: []string{"enchantment"}}
	card.AST = &gameast.CardAST{Name: name, Abilities: []gameast.Ability{
		&gameast.Static{
			Modification: &gameast.Modification{
				ModKind: "ability_word",
				Args:    []interface{}{"landfall", "triggered", inner},
			},
			Raw: "landfall — whenever a land you control enters, you gain 1 life",
		},
	}}
	p := &Permanent{Card: card, Controller: seat, Owner: seat,
		Timestamp: gs.NextTimestamp(), Counters: map[string]int{}, Flags: map[string]int{}}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

// enterLand puts a land onto `seat`'s battlefield and fires the full ETB
// cascade — the SAME path tryPlayLand and every effect-placed land use.
func enterLand(gs *GameState, seat int, name string, token bool) *Permanent {
	types := []string{"land"}
	if token {
		types = append(types, "token")
	}
	c := &Card{Name: name, Owner: seat, Types: types}
	p := &Permanent{Card: c, Controller: seat, Owner: seat,
		Timestamp: gs.NextTimestamp(), Counters: map[string]int{}, Flags: map[string]int{}}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	FirePermanentETBTriggers(gs, p)
	return p
}

// (e) generic-AST landfall payoff fires; (a) once per land.
func TestLandfall_FiresOncePerLand(t *testing.T) {
	gs := lfGame(t)
	addLandfallPayoff(gs, 0, "Lotus Cobra")
	life := gs.Seats[0].Life

	enterLand(gs, 0, "Forest", false)
	if got := gs.Seats[0].Life - life; got != 1 {
		t.Fatalf("landfall after 1 land fired %d times, want 1 (was INERT pre-fix)", got)
	}
	enterLand(gs, 0, "Island", false)
	if got := gs.Seats[0].Life - life; got != 2 {
		t.Fatalf("landfall after 2 lands fired %d times total, want 2 (one per land)", got)
	}
}

// (b) fires for ANY land entering, incl. a token land placed by an effect.
func TestLandfall_FiresForTokenAndEffectPlacedLand(t *testing.T) {
	gs := lfGame(t)
	addLandfallPayoff(gs, 0, "Tireless Provisioner")
	life := gs.Seats[0].Life
	enterLand(gs, 0, "Treasure-into-land", true) // token land via an effect
	if got := gs.Seats[0].Life - life; got != 1 {
		t.Fatalf("landfall did not fire for a token/effect-placed land: got %d, want 1", got)
	}
}

// (c) does NOT fire for an opponent's land (landfall is by-definition
// "you control").
func TestLandfall_DoesNotFireForOpponentLand(t *testing.T) {
	gs := lfGame(t)
	addLandfallPayoff(gs, 0, "Scute Swarm")
	life := gs.Seats[0].Life
	enterLand(gs, 1, "Opp Forest", false) // opponent's land
	if gs.Seats[0].Life != life {
		t.Fatalf("landfall fired for an OPPONENT's land (got +%d); 'you control' must gate it", gs.Seats[0].Life-life)
	}
}

// (c') does NOT fire on a NONLAND entering under your control.
func TestLandfall_DoesNotFireForNonland(t *testing.T) {
	gs := lfGame(t)
	addLandfallPayoff(gs, 0, "Lotus Cobra")
	life := gs.Seats[0].Life
	// a creature enters under seat 0
	cr := &Permanent{Card: &Card{Name: "Bear", Owner: 0, Types: []string{"creature"}, BasePower: 2, BaseToughness: 2},
		Controller: 0, Owner: 0, Timestamp: gs.NextTimestamp(), Counters: map[string]int{}, Flags: map[string]int{}}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, cr)
	FirePermanentETBTriggers(gs, cr)
	if gs.Seats[0].Life != life {
		t.Fatalf("landfall fired on a nonland ETB (got +%d)", gs.Seats[0].Life-life)
	}
}

// (f) a land that enters and immediately leaves still triggered the ETB
// landfall (the trigger resolves at ETB time; later removal doesn't undo it).
func TestLandfall_FiresEvenIfLandLeavesAfter(t *testing.T) {
	gs := lfGame(t)
	addLandfallPayoff(gs, 0, "Lotus Cobra")
	life := gs.Seats[0].Life
	land := enterLand(gs, 0, "Fetched Forest", false)
	if got := gs.Seats[0].Life - life; got != 1 {
		t.Fatalf("landfall did not fire at ETB: got %d", got)
	}
	// land leaves (e.g. fetchland sacrifices, or a flicker) — life is unchanged.
	bf := gs.Seats[0].Battlefield[:0]
	for _, p := range gs.Seats[0].Battlefield {
		if p != land {
			bf = append(bf, p)
		}
	}
	gs.Seats[0].Battlefield = bf
	if got := gs.Seats[0].Life - life; got != 1 {
		t.Fatalf("life changed after the land left; landfall must already have resolved: got %d", got)
	}
}

// double-fire guard: when a per_card handler owns this card's land ETB, the
// generic landfall path must NOT also fire (mirrors the nil-actor guard).
func TestLandfall_PerCardGuardNoDoubleFire(t *testing.T) {
	gs := lfGame(t)
	addLandfallPayoff(gs, 0, "Tireless Tracker")

	old := HasTriggerHook
	HasTriggerHook = func(cardName, event string) bool {
		return cardName == "Tireless Tracker" && event == "permanent_etb"
	}
	defer func() { HasTriggerHook = old }()

	life := gs.Seats[0].Life
	enterLand(gs, 0, "Forest", false)
	if gs.Seats[0].Life != life {
		t.Fatalf("generic landfall fired despite a per_card permanent_etb handler (double-fire): +%d", gs.Seats[0].Life-life)
	}
}

// scope guard: the unwrap is landfall-ONLY — other ability words
// (constellation, magecraft, …) own dedicated dispatchers and must not be
// unwrapped here (else they double-fire).
func TestObserverTriggeredFromAbility_LandfallOnly(t *testing.T) {
	mkWord := func(word string) gameast.Ability {
		return &gameast.Static{Modification: &gameast.Modification{
			ModKind: "ability_word",
			Args:    []interface{}{word, "triggered", &gameast.Triggered{Trigger: gameast.Trigger{Event: "tribe_you_control_etb"}}},
		}}
	}
	if trig, lf := observerTriggeredFromAbility(mkWord("landfall")); trig == nil || !lf {
		t.Fatalf("landfall ability word must unwrap (trig=%v landfall=%v)", trig != nil, lf)
	}
	if trig, lf := observerTriggeredFromAbility(mkWord("constellation")); trig != nil || lf {
		t.Fatalf("non-landfall ability word must NOT unwrap here (trig=%v landfall=%v)", trig != nil, lf)
	}
	// top-level Triggered passes through unchanged, not flagged landfall.
	top := &gameast.Triggered{Trigger: gameast.Trigger{Event: "etb"}}
	if trig, lf := observerTriggeredFromAbility(top); trig != top || lf {
		t.Fatalf("top-level Triggered must pass through (trig==top=%v landfall=%v)", trig == top, lf)
	}
}
