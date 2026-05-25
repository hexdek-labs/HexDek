package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// R41 Targeting Protection Sweep
//
// Covers 20 sample cards across the four targeting-protection mechanics:
//   - Hexproof  (CR §702.11)
//   - Shroud    (CR §702.18)
//   - Protection from [color]  (CR §702.16)
//   - Ward      (CR §702.21)
//
// PickTarget rejection is checked via pickPermanentTarget for hexproof /
// shroud / protection — those mechanics make a targeted filter skip the
// candidate (targets.go:268, CanBeTargetedByCombat in keywords_combat.go:1764).
//
// Ward is enforced at target-selection time by CheckWardOnTargeting
// (stack.go:1738): the spell is countered if the controller can't or won't
// pay the ward cost.
// ---------------------------------------------------------------------------

// r41AddCreature builds a battlefield creature with optional AST keywords and
// runtime flags. Used by the sweep below.
func r41AddCreature(gs *GameState, seat int, name string, kws []string, flags map[string]int, colors ...string) *Permanent {
	card := &Card{
		Name:          name,
		Owner:         seat,
		BasePower:     2,
		BaseToughness: 2,
		Types:         []string{"creature"},
		Colors:        append([]string{}, colors...),
	}
	if len(kws) > 0 {
		abilities := make([]gameast.Ability, 0, len(kws))
		for _, k := range kws {
			abilities = append(abilities, &gameast.Keyword{Name: k})
		}
		card.AST = &gameast.CardAST{Name: name, Abilities: abilities}
	}
	p := &Permanent{
		Card:       card,
		Controller: seat,
		Owner:      seat,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}
	for k, v := range flags {
		p.Flags[k] = v
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

// targetsContain reports whether the candidate list includes perm.
func targetsContain(list []Target, perm *Permanent) bool {
	for _, t := range list {
		if t.Permanent == perm {
			return true
		}
	}
	return false
}

// TestTargetingProtectionSweepR41_Hexproof — 5 sample cards. Opponent cannot
// target; controller can. Hexproof from [color] is exercised in the
// protection-from-color section below to keep the categories clean.
func TestTargetingProtectionSweepR41_Hexproof(t *testing.T) {
	cards := []string{
		"Troll Ascetic",
		"Geist of Saint Traft",
		"Slippery Bogle",
		"Witchstalker",
		"Sylvan Caryatid",
	}
	for _, name := range cards {
		t.Run(name, func(t *testing.T) {
			gs := NewGameState(2, nil, nil)
			perm := r41AddCreature(gs, 1, name, []string{"hexproof"}, nil)

			f := gameast.Filter{Base: "creature", Targeted: true}
			// Opponent (seat 0) cannot target.
			if got := pickPermanentTarget(gs, f, 0, nil); targetsContain(got, perm) {
				t.Fatalf("%s: hexproof should reject opponent targeting", name)
			}
			// Controller (seat 1) can target.
			if got := pickPermanentTarget(gs, f, 1, nil); !targetsContain(got, perm) {
				t.Fatalf("%s: hexproof should still allow controller to target", name)
			}
		})
	}
}

// TestTargetingProtectionSweepR41_Shroud — 4 sample cards. No one can target,
// not even the controller (CR §702.18).
func TestTargetingProtectionSweepR41_Shroud(t *testing.T) {
	cards := []string{
		"Argothian Enchantress",
		"Autochthon Wurm",
		"Spike Weaver",
		"Spectral Lynx",
	}
	for _, name := range cards {
		t.Run(name, func(t *testing.T) {
			gs := NewGameState(2, nil, nil)
			perm := r41AddCreature(gs, 1, name, []string{"shroud"}, nil)

			f := gameast.Filter{Base: "creature", Targeted: true}
			if got := pickPermanentTarget(gs, f, 0, nil); targetsContain(got, perm) {
				t.Fatalf("%s: shroud should reject opponent targeting", name)
			}
			if got := pickPermanentTarget(gs, f, 1, nil); targetsContain(got, perm) {
				t.Fatalf("%s: shroud should reject controller targeting too", name)
			}
			// Untargeted ("each creature") fan-out ignores shroud.
			fEach := gameast.Filter{Base: "creature", Quantifier: "each", Targeted: false}
			if got := allPermanentTargets(gs, fEach, 0); !targetsContain(got, perm) {
				t.Fatalf("%s: untargeted 'each creature' should still include shrouded creature", name)
			}
		})
	}
}

// TestTargetingProtectionSweepR41_ProtectionFromColor — 5 sample cards. The
// permanent cannot be targeted by spells of the protected color. A
// non-matching color source can still target it.
func TestTargetingProtectionSweepR41_ProtectionFromColor(t *testing.T) {
	cases := []struct {
		name          string
		protections   []string // color names: "black", "white", "red", "green", "blue"
		blockedSource *Card    // a colored source that should be rejected
		allowedSource *Card    // a differently-colored source that should pass
	}{
		{
			name:          "White Knight",
			protections:   []string{"black"},
			blockedSource: &Card{Name: "Murder", Types: []string{"instant"}, Colors: []string{"B"}},
			allowedSource: &Card{Name: "Lightning Bolt", Types: []string{"instant"}, Colors: []string{"R"}},
		},
		{
			name:          "Black Knight",
			protections:   []string{"white"},
			blockedSource: &Card{Name: "Swords to Plowshares", Types: []string{"instant"}, Colors: []string{"W"}},
			allowedSource: &Card{Name: "Doom Blade", Types: []string{"instant"}, Colors: []string{"B"}},
		},
		{
			name:          "Paladin en-Vec",
			protections:   []string{"black", "red"},
			blockedSource: &Card{Name: "Lightning Bolt", Types: []string{"instant"}, Colors: []string{"R"}},
			allowedSource: &Card{Name: "Mana Drain", Types: []string{"instant"}, Colors: []string{"U"}},
		},
		{
			name:          "Silver Knight",
			protections:   []string{"red"},
			blockedSource: &Card{Name: "Shock", Types: []string{"instant"}, Colors: []string{"R"}},
			allowedSource: &Card{Name: "Negate", Types: []string{"instant"}, Colors: []string{"U"}},
		},
		{
			name:          "Mirran Crusader",
			protections:   []string{"black", "green"},
			blockedSource: &Card{Name: "Plummet", Types: []string{"instant"}, Colors: []string{"G"}},
			allowedSource: &Card{Name: "Counterspell", Types: []string{"instant"}, Colors: []string{"U"}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gs := NewGameState(2, nil, nil)
			flags := map[string]int{"kw:protection": 1}
			for _, color := range tc.protections {
				flags["protection_from_"+color] = 1
			}
			perm := r41AddCreature(gs, 1, tc.name, []string{"protection"}, flags)

			// Direct CanBeTargetedByCombat assertions — this is what
			// pickPermanentTarget gates on at targets.go:268.
			if CanBeTargetedByCombat(perm, 0, tc.blockedSource) {
				t.Fatalf("%s: protection from %v should reject source %s (%v)",
					tc.name, tc.protections, tc.blockedSource.Name, tc.blockedSource.Colors)
			}
			if !CanBeTargetedByCombat(perm, 0, tc.allowedSource) {
				t.Fatalf("%s: non-matching colored source %s (%v) should still be legal",
					tc.name, tc.allowedSource.Name, tc.allowedSource.Colors)
			}
		})
	}
}

// TestTargetingProtectionSweepR41_Ward — 6 sample cards. CheckWardOnTargeting
// either consumes the ward cost from the controller's mana pool or counters
// the spell when the controller can't afford it.
func TestTargetingProtectionSweepR41_Ward(t *testing.T) {
	cases := []struct {
		name     string
		wardCost int
	}{
		{"Sheoldred, the Apocalypse", 2},
		{"Adeline, Resplendent Cathar", 1},
		{"Tenacious Underdog", 1},
		{"Faerie Mastermind", 1},
		{"Junji, the Midnight Sky", 2},
		{"Atraxa, Grand Unifier", 4},
	}

	for _, tc := range cases {
		t.Run(tc.name+"/paid", func(t *testing.T) {
			gs := NewGameState(2, nil, nil)
			perm := r41AddCreature(gs, 1, tc.name, []string{"ward"},
				map[string]int{"kw:ward": 1, "ward_cost": tc.wardCost})

			gs.Seats[0].ManaPool = tc.wardCost + 3
			item := &StackItem{
				Controller: 0,
				Card:       &Card{Name: "Removal Spell", Types: []string{"instant"}},
				Targets:    []Target{{Kind: TargetKindPermanent, Permanent: perm}},
			}
			CheckWardOnTargeting(gs, item)
			if item.Countered {
				t.Fatalf("%s: ward %d should be payable with %d mana available",
					tc.name, tc.wardCost, tc.wardCost+3)
			}
			if got, want := gs.Seats[0].ManaPool, 3; got != want {
				t.Fatalf("%s: expected %d mana remaining after paying ward %d, got %d",
					tc.name, want, tc.wardCost, got)
			}
			if !r41HasEvent(gs, "ward_paid") {
				t.Fatalf("%s: expected 'ward_paid' event in log", tc.name)
			}
		})

		t.Run(tc.name+"/countered", func(t *testing.T) {
			gs := NewGameState(2, nil, nil)
			perm := r41AddCreature(gs, 1, tc.name, []string{"ward"},
				map[string]int{"kw:ward": 1, "ward_cost": tc.wardCost})

			gs.Seats[0].ManaPool = tc.wardCost - 1 // insufficient
			item := &StackItem{
				Controller: 0,
				Card:       &Card{Name: "Removal Spell", Types: []string{"instant"}},
				Targets:    []Target{{Kind: TargetKindPermanent, Permanent: perm}},
			}
			CheckWardOnTargeting(gs, item)
			if !item.Countered {
				t.Fatalf("%s: ward %d should counter spell when caster has only %d mana",
					tc.name, tc.wardCost, tc.wardCost-1)
			}
			if !r41HasEvent(gs, "ward_counter") {
				t.Fatalf("%s: expected 'ward_counter' event in log", tc.name)
			}
		})

		t.Run(tc.name+"/owner-bypasses-ward", func(t *testing.T) {
			gs := NewGameState(2, nil, nil)
			perm := r41AddCreature(gs, 0, tc.name, []string{"ward"},
				map[string]int{"kw:ward": 1, "ward_cost": tc.wardCost})

			gs.Seats[0].ManaPool = 0
			item := &StackItem{
				Controller: 0,
				Card:       &Card{Name: "Self-Buff", Types: []string{"instant"}},
				Targets:    []Target{{Kind: TargetKindPermanent, Permanent: perm}},
			}
			CheckWardOnTargeting(gs, item)
			if item.Countered {
				t.Fatalf("%s: ward must not trigger when controller targets own permanent", tc.name)
			}
		})
	}
}

func r41HasEvent(gs *GameState, kind string) bool {
	for _, ev := range gs.EventLog {
		if ev.Kind == kind {
			return true
		}
	}
	return false
}
