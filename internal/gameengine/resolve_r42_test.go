package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// TestResolveSacrifice_SelfBase verifies that a Sacrifice effect whose
// Query.Base is a self-reference ("self"/"it"/"this") sacrifices the
// source permanent directly instead of iterating the battlefield with
// matchesPermanent (which has no "self" type and would silently no-op).
//
// Real cards: Pestilence ("sacrifice this enchantment"), Pyrohemia,
// Withering Wisps, Task Mage Assembly. R41 goldilocks measured these
// four as "dead-effect" failures; the engine never moved the source
// out of the battlefield.
func TestResolveSacrifice_SelfBase(t *testing.T) {
	for _, base := range []string{"self", "it", "this", "that_creature"} {
		t.Run(base, func(t *testing.T) {
			gs := newFixtureGame(t)
			src := addBattlefield(gs, 0, "Pestilence Stand-In", 0, 0, "enchantment")
			// A second permanent that should NOT be touched by the effect.
			bystander := addBattlefield(gs, 0, "Bystander Creature", 2, 2, "creature")

			eff := &gameast.Sacrifice{
				Query: gameast.Filter{Base: base},
				Actor: "controller",
			}
			ResolveEffect(gs, src, eff)

			// Source should be off the battlefield, bystander should remain.
			for _, p := range gs.Seats[0].Battlefield {
				if p == src {
					t.Fatalf("base=%q: source still on battlefield after sacrifice", base)
				}
			}
			found := false
			for _, p := range gs.Seats[0].Battlefield {
				if p == bystander {
					found = true
				}
			}
			if !found {
				t.Fatalf("base=%q: bystander incorrectly removed by self-sacrifice", base)
			}
		})
	}
}

// TestResolveSacrifice_ActorSelf verifies that Actor="self" is aliased
// to the controller of the source — the parser shape used by Planar
// Engineering ("sacrifice two lands") and other static "typed_spell_effect"
// clauses whose subject is implicitly the controller. Without this alias
// the actor-seats slice stays empty and the effect no-ops.
func TestResolveSacrifice_ActorSelf(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Planar Engineering Stand-In", 0, 0, "sorcery")
	land := addBattlefield(gs, 0, "Forest", 0, 0, "land", "basic")

	eff := &gameast.Sacrifice{
		Query: gameast.Filter{Base: "land", YouControl: true, Targeted: true},
		Actor: "self",
	}
	ResolveEffect(gs, src, eff)

	for _, p := range gs.Seats[0].Battlefield {
		if p == land {
			t.Fatalf("Actor=self did not sacrifice the land — actor alias broken")
		}
	}
}

// TestParseLifeChange verifies the regex helper used by life_effect's
// raw-text fallback. Covers Reaver Drone / Scourge of Numai / Fathom
// Fleet Boarder / Lord of Tresserhorn shapes.
func TestParseLifeChange(t *testing.T) {
	cases := []struct {
		raw      string
		expected int
		ok       bool
	}{
		{"you lose 1 life unless you control another colorless creature", -1, true},
		{"you lose 2 life if you don't control an Ogre", -2, true},
		{"you lose 2 life unless you control another Pirate", -2, true},
		{"you lose 2 life, you sacrifice two creatures, and target opponent draws two cards", -2, true},
		{"target opponent gains 3 life", 3, true},
		{"you gain 5 life", 5, true},
		{"each player loses half their life", 0, false}, // half — no integer
		{"draw a card", 0, false},
		{"", 0, false},
	}
	for _, tc := range cases {
		got, ok := parseLifeChange(tc.raw)
		if ok != tc.ok || got != tc.expected {
			t.Errorf("parseLifeChange(%q) = (%d, %v), want (%d, %v)",
				tc.raw, got, ok, tc.expected, tc.ok)
		}
	}
}

// TestResolveModificationEffect_LifeEffectTextFallback verifies that a
// Modification(kind="life_effect", args=[<raw text>]) parses the
// magnitude+direction out of the raw text and applies the life change.
// Without this, Reaver Drone et al. emit zero events.
func TestResolveModificationEffect_LifeEffectTextFallback(t *testing.T) {
	cases := []struct {
		name     string
		raw      string
		preLife  int
		postLife int
	}{
		{"reaver_drone", "you lose 1 life unless you control another colorless creature", 20, 19},
		{"scourge_numai", "you lose 2 life if you don't control an Ogre", 20, 18},
		{"fathom_fleet", "you lose 2 life unless you control another Pirate", 20, 18},
		{"tresserhorn", "you lose 2 life, you sacrifice two creatures, and target opponent draws two cards", 20, 18},
		{"gain_life", "you gain 3 life", 20, 23},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gs := newFixtureGame(t)
			src := addBattlefield(gs, 0, "Stand-In", 0, 0, "creature")
			gs.Seats[0].Life = tc.preLife

			eff := &gameast.ModificationEffect{
				ModKind: "life_effect",
				Args:    []interface{}{tc.raw},
			}
			ResolveEffect(gs, src, eff)

			if gs.Seats[0].Life != tc.postLife {
				t.Fatalf("life=%d, want %d (raw=%q)", gs.Seats[0].Life, tc.postLife, tc.raw)
			}
		})
	}
}

// TestResolveExile_CardBaseGraveyardFallback verifies that
// Exile{Filter{Base:"card"}} falls back to a graveyard search when no
// matching permanent is on the battlefield — covers Soul-Guide Lantern's
// "exile target card from a graveyard" ETB once the parser strips the
// graveyard hint.
func TestResolveExile_CardBaseGraveyardFallback(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Soul-Guide Lantern Stand-In", 0, 0, "artifact")

	// Seed seat 1's graveyard with a card.
	target := &Card{Name: "Doomed Necromancer", Owner: 1, Types: []string{"creature"}}
	gs.Seats[1].Graveyard = append(gs.Seats[1].Graveyard, target)

	eff := &gameast.Exile{
		Target: gameast.Filter{Base: "card", Targeted: true},
	}
	ResolveEffect(gs, src, eff)

	// Card should have moved from seat 1's graveyard to exile.
	for _, c := range gs.Seats[1].Graveyard {
		if c == target {
			t.Fatalf("card was not exiled from seat 1's graveyard")
		}
	}
	found := false
	for _, c := range gs.Seats[1].Exile {
		if c == target {
			found = true
			break
		}
	}
	if !found {
		// Some engines route exiled cards to a shared exile zone; tolerate
		// that as long as it's no longer in the graveyard. The key
		// regression check is that resolveExile did NOT silently no-op.
		// MoveCard always logs a zone_change event we can use as a fallback.
		if countEvents(gs, "zone_change") == 0 {
			t.Fatalf("expected zone_change event from graveyard exile, got none")
		}
	}
}
