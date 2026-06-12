package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// -----------------------------------------------------------------------------
// r62 — legality validator PHASE 3: mana-ability discipline (CR 605.1a /
// 605.3a) and replacement-application sanity (CR 614.1a / 614.1c).
// -----------------------------------------------------------------------------

func phase3Game(t *testing.T) (*GameState, *LegalityValidator) {
	t.Helper()
	gs := newFixtureGame(t)
	v := NewLegalityValidator(62)
	gs.Legality = v
	return gs, v
}

func phase3ManaPerm(seat int, effect gameast.Effect) *Permanent {
	return &Permanent{
		Card: &Card{
			Name:  "Test Rock",
			Owner: seat,
			Types: []string{"artifact"},
			AST: &gameast.CardAST{
				Name: "Test Rock",
				Abilities: []gameast.Ability{
					&gameast.Activated{
						Cost:   gameast.Cost{Tap: true},
						Effect: effect,
					},
				},
			},
		},
		Controller: seat,
		Owner:      seat,
	}
}

// Real-path clean pins: a genuine mana ability resolves inline with no
// violations; a genuine non-mana ability goes to the stack with none.
func TestLegalityPhase3_ManaAbility_RealPathsClean(t *testing.T) {
	gs, v := phase3Game(t)
	gs.Active = 0
	gs.Phase = "main"

	rock := phase3ManaPerm(0, &gameast.AddMana{Pool: []gameast.ManaSymbol{{Raw: "{c}", Color: []string{"C"}}}})
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, rock)
	if err := ActivateAbility(gs, 0, rock, 0, nil); err != nil {
		t.Fatalf("mana ability activation failed: %v", err)
	}

	drawer := phase3ManaPerm(0, &gameast.Draw{Count: gameast.NumberOrRef{IsInt: true, Int: 1}})
	drawer.Card.Name = "Test Drawer"
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, drawer)
	if err := ActivateAbility(gs, 0, drawer, 0, nil); err != nil {
		t.Fatalf("non-mana ability activation failed: %v", err)
	}

	for _, viol := range v.Violations {
		t.Errorf("legal activation flagged: %s", viol.String())
	}
}

// A non-mana ability completed inline while CLAIMING to be a mana
// ability must flag 605.1a (the engine drifting on IsManaAbility is the
// guarded failure mode — opponents get no response window).
func TestLegalityPhase3_ManaAbility_InlineNonManaFlagged(t *testing.T) {
	gs, v := phase3Game(t)
	drawer := phase3ManaPerm(1, &gameast.Draw{Count: gameast.NumberOrRef{IsInt: true, Int: 1}})
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, drawer)

	obs := v.BeginActivation(gs, 1, drawer, 0, drawer.Card.AST.Abilities[0].(*gameast.Activated))
	obs.SetNoStackReason("mana_ability") // the engine's (wrong) claim
	v.FinishActivation(gs, obs, nil)

	found := false
	for _, viol := range v.Violations {
		if viol.Rule == "605.1a" {
			found = true
		}
	}
	if !found {
		t.Fatalf("inline non-mana ability not flagged: %v", v.Violations)
	}
}

// A true mana ability pushed onto the stack must flag 605.3a.
func TestLegalityPhase3_ManaAbility_StackedManaFlagged(t *testing.T) {
	gs, v := phase3Game(t)
	rock := phase3ManaPerm(0, &gameast.AddMana{Pool: []gameast.ManaSymbol{{Raw: "{c}", Color: []string{"C"}}}})
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, rock)

	obs := v.BeginActivation(gs, 0, rock, 0, rock.Card.AST.Abilities[0].(*gameast.Activated))
	v.FinishActivation(gs, obs, &StackItem{Kind: "activated", Controller: 0, Source: rock})

	found := false
	for _, viol := range v.Violations {
		if viol.Rule == "605.3a" {
			found = true
		}
	}
	if !found {
		t.Fatalf("stacked mana ability not flagged: %v", v.Violations)
	}
}

// Replacement-ETB: a permanent whose AST says "enters with two +1/+1
// counters" arriving WITHOUT them flags 614.1c; the real cascade
// (which applies the counters) stays clean.
func TestLegalityPhase3_ReplacementETB(t *testing.T) {
	gs, v := phase3Game(t)
	mk := func(name string) *Permanent {
		return &Permanent{
			Card: &Card{
				Name:  name,
				Owner: 0,
				Types: []string{"creature"},
				AST: &gameast.CardAST{
					Name: name,
					Abilities: []gameast.Ability{
						&gameast.Static{
							Modification: &gameast.Modification{
								ModKind: "etb_with_counters",
								Args:    []interface{}{float64(2), "+1/+1"},
							},
						},
					},
				},
			},
			Controller: 0,
			Owner:      0,
			Counters:   map[string]int{},
		}
	}

	// Real cascade path: counters applied, clean.
	honest := mk("Honest Hydra")
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, honest)
	FirePermanentETBTriggers(gs, honest)
	if len(v.Violations) != 0 {
		t.Fatalf("honest ETB flagged: %v", v.Violations)
	}
	if honest.Counters["+1/+1"] != 2 {
		t.Fatalf("fixture sanity: cascade should apply 2 counters, got %d", honest.Counters["+1/+1"])
	}

	// Bypass path: entered with no counters — the replacement was skipped.
	skipped := mk("Skipped Hydra")
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, skipped)
	v.ObserveETB(gs, skipped)
	found := false
	for _, viol := range v.Violations {
		if viol.Rule == "614.1c" {
			found = true
		}
	}
	if !found {
		t.Fatalf("skipped enters-with-counters not flagged: %v", v.Violations)
	}
}

// Replacement-graveyard: a battlefield->graveyard arrival while an
// applicable opponent-sourced redirect replacement (RIP-class) is
// registered flags 614.1a; with no applicable replacement (or only a
// self-sourced one) the same arrival is clean.
func TestLegalityPhase3_ReplacementGraveyard(t *testing.T) {
	gs, v := phase3Game(t)

	dead := &Permanent{
		Card:       &Card{Name: "Doomed Bear", Owner: 0, Types: []string{"creature"}},
		Controller: 0,
		Owner:      0,
	}

	// Clean arm: no replacements registered.
	FireZoneChangeTriggers(gs, dead, dead.Card, "battlefield", "graveyard")
	if len(v.Violations) != 0 {
		t.Fatalf("graveyard arrival with no replacements flagged: %v", v.Violations)
	}

	// RIP-class redirect registered by an opponent's permanent: arrival
	// in the graveyard means the §614 chain was bypassed.
	rip := &Permanent{
		Card:       &Card{Name: "Rest in Peace", Owner: 1, Types: []string{"enchantment"}},
		Controller: 1,
		Owner:      1,
	}
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, rip)
	gs.RegisterReplacement(&ReplacementEffect{
		EventType:      "would_be_put_into_graveyard",
		HandlerID:      "Rest in Peace:exile_instead:test",
		SourcePerm:     rip,
		ControllerSeat: 1,
		Category:       CategoryOther,
		Applies:        func(*GameState, *ReplEvent) bool { return true },
		ApplyFn:        func(_ *GameState, ev *ReplEvent) { ev.Payload["to_zone"] = "exile" },
	})

	FireZoneChangeTriggers(gs, dead, dead.Card, "battlefield", "graveyard")
	found := false
	for _, viol := range v.Violations {
		if viol.Rule == "614.1a" {
			found = true
		}
	}
	if !found {
		t.Fatalf("graveyard arrival past an applicable RIP-class replacement not flagged: %v", v.Violations)
	}

	// Self-sourced replacements are exempt.
	v.Violations = nil
	gs.Replacements = nil
	gs.RegisterReplacement(&ReplacementEffect{
		EventType:  "would_die",
		HandlerID:  "Doomed Bear:self:test",
		SourcePerm: dead,
		Applies:    func(*GameState, *ReplEvent) bool { return true },
		ApplyFn:    func(*GameState, *ReplEvent) {},
	})
	FireZoneChangeTriggers(gs, dead, dead.Card, "battlefield", "graveyard")
	if len(v.Violations) != 0 {
		t.Fatalf("self-sourced replacement should be exempt: %v", v.Violations)
	}
}

// Default-off pin: phase-3 hooks are nil-receiver no-ops.
func TestLegalityPhase3_DefaultOff_NoOp(t *testing.T) {
	gs := newFixtureGame(t)
	dead := &Permanent{
		Card:       &Card{Name: "Bear", Owner: 0, Types: []string{"creature"}},
		Controller: 0,
	}
	FireZoneChangeTriggers(gs, dead, dead.Card, "battlefield", "graveyard")
	FirePermanentETBTriggers(gs, dead)
	// Reaching here without a panic is the pin (gs.Legality is nil).
}
