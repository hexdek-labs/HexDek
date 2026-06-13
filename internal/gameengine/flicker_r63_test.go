package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// r63 FLICKER / BLINK mechanic audit regressions.

func bfCount(gs *GameState, seat int, name string) int {
	n := 0
	for _, p := range gs.Seats[seat].Battlefield {
		if p != nil && p.Card != nil && p.Card.Name == name {
			n++
		}
	}
	return n
}

// New-object identity (CR §400.7): the returned permanent is a NEW object —
// new (higher) timestamp, summoning sick, counters/damage cleared, auras
// dropped, and exactly one copy on the battlefield.
func TestFlicker_NewObjectIdentity(t *testing.T) {
	gs := newFixtureGame(t)
	bear := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")
	bear.SummoningSick = false
	bear.Counters["+1/+1"] = 3
	bear.MarkedDamage = 1
	oldTS := bear.Timestamp
	// An aura attached to the bear.
	aura := addBattlefield(gs, 0, "Pacifism", 0, 0, "enchantment", "aura")
	aura.AttachedTo = bear

	np := FlickerPermanent(gs, bear, 0)
	if np == nil {
		t.Fatal("flicker should return a new permanent for a nontoken creature")
	}
	if np == bear {
		t.Fatal("returned permanent must be a NEW object, not the same pointer")
	}
	if np.Timestamp <= oldTS {
		t.Fatalf("new object should have a fresh (higher) timestamp; old=%d new=%d", oldTS, np.Timestamp)
	}
	if !np.SummoningSick {
		t.Error("returned creature should be summoning sick")
	}
	if np.Counters["+1/+1"] != 0 {
		t.Errorf("counters should be cleared, got +1/+1=%d", np.Counters["+1/+1"])
	}
	if np.MarkedDamage != 0 {
		t.Errorf("marked damage should be cleared, got %d", np.MarkedDamage)
	}
	if aura.AttachedTo != nil {
		t.Error("aura should have been detached (its target ceased to exist as the old object)")
	}
	if c := bfCount(gs, 0, "Grizzly Bears"); c != 1 {
		t.Fatalf("exactly one Grizzly Bears should be on the battlefield, got %d", c)
	}
}

// Haste creature returns NOT summoning sick.
func TestFlicker_HasteNotSick(t *testing.T) {
	gs := newFixtureGame(t)
	hasty := &Card{Name: "Hasty Bear", Owner: 0, Types: []string{"creature"},
		AST: &gameast.CardAST{Abilities: []gameast.Ability{&gameast.Keyword{Name: "haste"}}}}
	p := &Permanent{Card: hasty, Controller: 0, Owner: 0, Timestamp: gs.NextTimestamp(),
		Counters: map[string]int{}, Flags: map[string]int{}}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, p)
	np := FlickerPermanent(gs, p, 0)
	if np == nil {
		t.Fatal("expected return")
	}
	if np.SummoningSick {
		t.Error("haste creature should not be summoning sick after flicker")
	}
}

// ETB triggers fire on the return.
func TestFlicker_ETBFiresOnReturn(t *testing.T) {
	gs := newFixtureGame(t)
	card := &Card{
		Name:  "ETB Bear",
		Owner: 0,
		Types: []string{"creature"},
		AST: &gameast.CardAST{Abilities: []gameast.Ability{
			&gameast.Triggered{
				Trigger: gameast.Trigger{Event: "etb"},
				Effect:  &gameast.GainLife{Amount: gameast.NumberOrRef{IsInt: true, Int: 2}},
			},
		}},
	}
	p := &Permanent{Card: card, Controller: 0, Owner: 0, Timestamp: gs.NextTimestamp(),
		Counters: map[string]int{}, Flags: map[string]int{}}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, p)

	before := len(gs.EventLog)
	np := FlickerPermanent(gs, p, 0)
	if np == nil {
		t.Fatal("expected return")
	}
	fired := false
	for _, ev := range gs.EventLog[before:] {
		if ev.Kind == "triggered_ability" && ev.Source == "ETB Bear" {
			fired = true
		}
	}
	if !fired {
		t.Error("ETB trigger should fire on the returned (new) object")
	}
}

// A token ceases and does NOT return (CR §111.7 / §704.5d).
func TestFlicker_TokenCeasesDoesNotReturn(t *testing.T) {
	gs := newFixtureGame(t)
	tok := &Card{Name: "Servo", Owner: 0, Types: []string{"artifact", "creature", "token"}}
	MintTokenInstanceID(gs, tok, "", "")
	id := tok.InstanceID
	p := &Permanent{Card: tok, Controller: 0, Owner: 0, Timestamp: gs.NextTimestamp(),
		Counters: map[string]int{}, Flags: map[string]int{}}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, p)

	np := FlickerPermanent(gs, p, 0)
	if np != nil {
		t.Fatal("a flickered token must NOT return")
	}
	if bfCount(gs, 0, "Servo") != 0 {
		t.Fatal("token should be gone from the battlefield")
	}
	if id != "" {
		if _, ceased := gs.CeasedInstanceIDs[id]; !ceased {
			t.Error("token InstanceID should be ceased")
		}
	}
}

// Control: "under your control" keeps control with the spell's controller;
// default returns under the owner's control.
func TestFlicker_ControlSeat(t *testing.T) {
	gs := newFixtureGame(t)
	// Creature owned by seat 1 but controlled by seat 0 (stolen).
	card := &Card{Name: "Borrowed Beast", Owner: 1, Types: []string{"creature"}}
	p := &Permanent{Card: card, Controller: 0, Owner: 1, Timestamp: gs.NextTimestamp(),
		Counters: map[string]int{}, Flags: map[string]int{}}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, p)

	// "under your control" → returns under seat 0 (the flickerer).
	np := FlickerPermanent(gs, p, 0)
	if np == nil || np.Controller != 0 {
		t.Fatalf("under-your-control flicker should return under seat 0, got %v", np)
	}

	// Now default (owner's control): flicker again under owner (seat 1).
	np2 := FlickerPermanent(gs, np, 1)
	if np2 == nil || np2.Controller != 1 {
		t.Fatalf("owner-control flicker should return under seat 1 (owner), got %v", np2)
	}
}

// Integration through resolveExile: a Cloudshift-shaped spell (raw carries
// the immediate-flicker clause) returns the creature instead of exiling it.
func TestFlicker_Integration_CloudshiftReturns(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Cloudshift", 0, 0, "instant")
	src.Card.AST = &gameast.CardAST{Abilities: []gameast.Ability{
		&gameast.Static{Raw: "exile target creature you control, then return that card to the battlefield under your control"},
	}}
	addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")

	ResolveEffect(gs, src, &gameast.Exile{Target: gameast.Filter{Base: "creature", YouControl: true, Targeted: true}})

	if bfCount(gs, 0, "Grizzly Bears") != 1 {
		t.Fatalf("Cloudshift should return the creature to the battlefield, bf=%d exile=%d",
			bfCount(gs, 0, "Grizzly Bears"), len(gs.Seats[0].Exile))
	}
	if len(gs.Seats[0].Exile) != 0 {
		t.Errorf("creature should not remain in exile, exile=%d", len(gs.Seats[0].Exile))
	}
}

// Regression guard: a plain exile spell (no return clause) STILL exiles
// permanently — the flicker detection must not hijack normal exiles.
func TestFlicker_NonFlickerExileStillExiles(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Swords to Plowshares", 0, 0, "instant")
	src.Card.AST = &gameast.CardAST{Abilities: []gameast.Ability{
		&gameast.Static{Raw: "exile target creature"},
	}}
	addBattlefield(gs, 1, "Goblin", 1, 1, "creature")

	ResolveEffect(gs, src, &gameast.Exile{Target: gameast.Filter{Base: "creature", Targeted: true}})

	if bfCount(gs, 1, "Goblin") != 0 {
		t.Fatal("plain exile must remove the creature from the battlefield")
	}
	if len(gs.Seats[1].Exile) != 1 {
		t.Fatalf("plain exile must place the card in exile, got exile=%d", len(gs.Seats[1].Exile))
	}
}

// exileFlickerSpec gating: end-step ("delayed") returns are NOT treated as
// immediate flicker.
func TestFlicker_DelayedReturnNotImmediate(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Otherworldly Journey", 0, 0, "instant")
	src.Card.AST = &gameast.CardAST{Abilities: []gameast.Ability{
		&gameast.Static{Raw: "exile target creature"},
		&gameast.Triggered{Raw: "at the beginning of the next end step, return that card to the battlefield under its owner's control"},
	}}
	isFlicker, _ := exileFlickerSpec(src)
	if isFlicker {
		t.Error("end-step delayed return must NOT be detected as immediate flicker")
	}
}
