package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// mana_tracking_r60_test.go — wiring regressions for the R60
// AvailableColoredManaEstimate + CanPayColoredCost pair. Pre-R60 the
// ChooseResponse fast-path used generic AvailableManaEstimate, so a
// Counterspell {U}{U} would false-positive against a colorless pool.

// newLand attaches a land to the seat's battlefield. typeLine + oracle
// drive the color-mask helper (e.g. "Basic Land — Island" or "Land —
// Island Mountain" for a Volcanic Island).
func newLand(seat *gameengine.Seat, name, typeLine, oracle string) *gameengine.Permanent {
	c := &gameengine.Card{
		Name:     name,
		Types:    []string{"land"},
		TypeLine: typeLine,
	}
	if oracle != "" {
		c.OracleTextCache = oracle
	}
	p := &gameengine.Permanent{
		Card:       c,
		Controller: seat.Idx,
		Owner:      seat.Idx,
		Flags:      map[string]int{},
	}
	seat.Battlefield = append(seat.Battlefield, p)
	return p
}

// counterspellWithCost builds a Counterspell-shaped card carrying both
// the AST shape CardHasCounterSpell looks for AND a printed
// ManaCostString for the color-aware affordability check.
func counterspellWithCost(cost string) *gameengine.Card {
	c := newTestCardMinimal("Counterspell", []string{"instant"}, 2,
		&gameast.CardAST{
			Name: "Counterspell",
			Abilities: []gameast.Ability{
				&gameast.Activated{
					Effect: &gameast.CounterSpell{
						Target: gameast.Filter{Base: "spell", Targeted: true},
					},
				},
			},
		})
	c.ManaCostString = cost
	return c
}

func TestChooseResponse_R60_RejectsCounterWhenColorMissing(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Hand = []*gameengine.Card{counterspellWithCost("{U}{U}")}
	// Two Mountains — total 2 mana, ALL red. Old gate would accept
	// (avail=2 >= cmc=2); new gate rejects ({U}{U} can't be paid).
	newLand(gs.Seats[0], "Mountain", "Basic Land — Mountain", "")
	newLand(gs.Seats[0], "Mountain #2", "Basic Land — Mountain", "")

	h := NewYggdrasilHatWithNoise(&StrategyProfile{Archetype: ArchetypeControl}, 0, 0)
	top := &gameengine.StackItem{
		Controller: 1,
		Card: newTestCardMinimal("Wrath of God", []string{"sorcery"}, 4,
			&gameast.CardAST{
				Name:      "Wrath of God",
				Abilities: []gameast.Ability{&gameast.Static{Raw: "Destroy all creatures."}},
			}),
		Effect:     &gameast.Damage{},
	}
	if got := h.ChooseResponse(gs, 0, top); got != nil {
		t.Fatalf("hat must not cast {U}{U} counter from red-only mana base; got %v",
			got.Card.DisplayName())
	}
}

func TestChooseResponse_R60_AcceptsCounterWhenColorAvailable(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Hand = []*gameengine.Card{counterspellWithCost("{U}{U}")}
	newLand(gs.Seats[0], "Island", "Basic Land — Island", "")
	newLand(gs.Seats[0], "Island #2", "Basic Land — Island", "")

	h := NewYggdrasilHatWithNoise(&StrategyProfile{Archetype: ArchetypeControl}, 0, 0)
	top := &gameengine.StackItem{
		Controller: 1,
		Card: newTestCardMinimal("Wrath of God", []string{"sorcery"}, 4,
			&gameast.CardAST{
				Name:      "Wrath of God",
				Abilities: []gameast.Ability{&gameast.Static{Raw: "Destroy all creatures."}},
			}),
		Effect:     &gameast.Damage{},
	}
	got := h.ChooseResponse(gs, 0, top)
	if got == nil {
		t.Fatal("hat should accept Counterspell when 2 Islands are available")
	}
	if got.Card.DisplayName() != "Counterspell" {
		t.Fatalf("expected Counterspell, got %v", got.Card.DisplayName())
	}
}

func TestChooseResponse_R60_DualLandSatisfiesCounterColor(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Hand = []*gameengine.Card{counterspellWithCost("{U}{U}")}
	// Two Volcanic Islands — each can produce U or R. Should satisfy {U}{U}.
	newLand(gs.Seats[0], "Volcanic Island", "Land — Island Mountain", "")
	newLand(gs.Seats[0], "Volcanic Island #2", "Land — Island Mountain", "")

	h := NewYggdrasilHatWithNoise(&StrategyProfile{Archetype: ArchetypeControl}, 0, 0)
	top := &gameengine.StackItem{
		Controller: 1,
		Card: newTestCardMinimal("Wrath of God", []string{"sorcery"}, 4,
			&gameast.CardAST{
				Name:      "Wrath of God",
				Abilities: []gameast.Ability{&gameast.Static{Raw: "Destroy all creatures."}},
			}),
		Effect:     &gameast.Damage{},
	}
	if got := h.ChooseResponse(gs, 0, top); got == nil {
		t.Fatal("two Volcanic Islands should satisfy {U}{U} via flex")
	}
}

func TestChooseResponse_R60_PoolColorContributes(t *testing.T) {
	// Empty battlefield, but pool already has U U from a previous tap.
	gs := newTestGame(t, 2)
	gs.Seats[0].Hand = []*gameengine.Card{counterspellWithCost("{U}{U}")}
	gs.Seats[0].Mana = &gameengine.ColoredManaPool{U: 2}

	h := NewYggdrasilHatWithNoise(&StrategyProfile{Archetype: ArchetypeControl}, 0, 0)
	top := &gameengine.StackItem{
		Controller: 1,
		Card: newTestCardMinimal("Wrath of God", []string{"sorcery"}, 4,
			&gameast.CardAST{
				Name:      "Wrath of God",
				Abilities: []gameast.Ability{&gameast.Static{Raw: "Destroy all creatures."}},
			}),
		Effect:     &gameast.Damage{},
	}
	if got := h.ChooseResponse(gs, 0, top); got == nil {
		t.Fatal("pool {U}{U} should satisfy Counterspell cost")
	}
}

func TestChooseResponse_R60_EmptyManaCostStringAcceptsOnTotal(t *testing.T) {
	// Counterspell with no printed ManaCostString — fallback to CMC
	// gate. With 2 Mountains (total=2, cmc=2), affordability passes.
	gs := newTestGame(t, 2)
	gs.Seats[0].Hand = []*gameengine.Card{counterspellWithCost("")}
	newLand(gs.Seats[0], "Mountain", "Basic Land — Mountain", "")
	newLand(gs.Seats[0], "Mountain #2", "Basic Land — Mountain", "")

	h := NewYggdrasilHatWithNoise(&StrategyProfile{Archetype: ArchetypeControl}, 0, 0)
	top := &gameengine.StackItem{
		Controller: 1,
		Card: newTestCardMinimal("Wrath of God", []string{"sorcery"}, 4,
			&gameast.CardAST{
				Name:      "Wrath of God",
				Abilities: []gameast.Ability{&gameast.Static{Raw: "Destroy all creatures."}},
			}),
		Effect:     &gameast.Damage{},
	}
	if got := h.ChooseResponse(gs, 0, top); got == nil {
		t.Fatal("empty ManaCostString should fall back to CMC gate (passable here)")
	}
}
