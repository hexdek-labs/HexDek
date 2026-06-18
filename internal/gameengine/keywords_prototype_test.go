package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// keywords_prototype_test.go — CR §702.160 / §718 Prototype.
//
// Pins: the prototype alternative cost in the cast path (reduced cost + P/T +
// color + MV), the all-zones copiable-value model (devotion sees the reduced
// color, a copy copies the prototype stats), and the unchanged normal-cast
// (full printed version) path.

// protoCreatureCard builds a Skitterbeam-style prototype artifact creature:
// printed {9} colorless 7/5, Prototype {3}{R}{R} — 2/2.
func protoCreatureCard(name string) *Card {
	return &Card{
		Name:           name,
		Owner:          0,
		BasePower:      7,
		BaseToughness:  5,
		CMC:            9,
		ManaCostString: "{9}",
		Types:          []string{"artifact", "creature", "cost:9"},
		AST: &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "prototype", Args: []interface{}{"{3}{R}{R}", float64(2), float64(2)}},
			},
		},
	}
}

func findProtoPerm(gs *GameState, seat int, name string) *Permanent {
	for _, p := range gs.Seats[seat].Battlefield {
		if p != nil && p.Card != nil && p.Card.Name == name {
			return p
		}
	}
	return nil
}

// PrototypeProfile derives MV + colors from the prototype mana-cost string.
func TestPrototypeProfile_ParsesCostPTColors(t *testing.T) {
	card := protoCreatureCard("Skitterbeam Battalion")
	profile, ok := PrototypeProfile(card)
	if !ok {
		t.Fatal("PrototypeProfile should parse a card with the prototype keyword + args")
	}
	if profile.CMC != 5 {
		t.Errorf("prototype MV of {3}{R}{R} should be 5, got %d", profile.CMC)
	}
	if profile.Power != 2 || profile.Toughness != 2 {
		t.Errorf("prototype P/T should be 2/2, got %d/%d", profile.Power, profile.Toughness)
	}
	if len(profile.Colors) != 1 || profile.Colors[0] != "R" {
		t.Errorf("prototype color should be [R], got %v", profile.Colors)
	}
}

// (1)/(2) Cast for prototype → reduced cost paid, permanent has prototype
// P/T + color + MV, and devotion sees the reduced colored pips (all zones).
func TestPrototype_CastForPrototype_ReducedCostPTColorMV(t *testing.T) {
	gs := newKWCombatGame(t)
	gs.Seats[0].Hat = &payHat{} // chooses every optional cost → prototype
	gs.Seats[0].ManaPool = 5    // exactly the prototype cost ({3}{R}{R} = 5)
	card := protoCreatureCard("Skitterbeam Battalion")
	gs.Seats[0].Hand = []*Card{card}

	if err := CastSpell(gs, 0, card, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}
	if gs.Seats[0].ManaPool != 0 {
		t.Errorf("prototype cost is 5; expected pool 0, got %d", gs.Seats[0].ManaPool)
	}
	perm := findProtoPerm(gs, 0, "Skitterbeam Battalion")
	if perm == nil {
		t.Fatal("prototyped creature should resolve onto the battlefield")
	}
	if !perm.Card.IsPrototyped() {
		t.Fatal("resolved permanent's card should carry the prototype override")
	}
	if perm.Power() != 2 || perm.Toughness() != 2 {
		t.Errorf("prototype P/T should be 2/2, got %d/%d", perm.Power(), perm.Toughness())
	}
	if colors := gs.ColorsOf(perm); len(colors) != 1 || colors[0] != "R" {
		t.Errorf("prototyped permanent should be red, got %v", colors)
	}
	if mv := perm.Card.EffectiveCMC(); mv != 5 {
		t.Errorf("prototyped permanent MV should be 5, got %d", mv)
	}
	if dev := CountDevotion(gs, 0, "R"); dev != 2 {
		t.Errorf("devotion to red should see the 2 prototype red pips, got %d", dev)
	}
}

// (4) Cast normally → full printed version: full cost, printed P/T, printed
// MV, no prototype color, Proto cleared.
func TestPrototype_CastNormally_FullPrintedVersion(t *testing.T) {
	gs := newKWCombatGame(t)
	gs.Seats[0].Hat = &declineHat{} // declines the prototype cost
	gs.Seats[0].ManaPool = 9        // the full printed cost
	card := protoCreatureCard("Skitterbeam Battalion")
	gs.Seats[0].Hand = []*Card{card}

	if err := CastSpell(gs, 0, card, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}
	if gs.Seats[0].ManaPool != 0 {
		t.Errorf("full cost is 9; expected pool 0, got %d", gs.Seats[0].ManaPool)
	}
	perm := findProtoPerm(gs, 0, "Skitterbeam Battalion")
	if perm == nil {
		t.Fatal("normally-cast creature should resolve onto the battlefield")
	}
	if perm.Card.IsPrototyped() {
		t.Fatal("normal cast must NOT carry a prototype override")
	}
	if perm.Power() != 7 || perm.Toughness() != 5 {
		t.Errorf("printed P/T should be 7/5, got %d/%d", perm.Power(), perm.Toughness())
	}
	if mv := perm.Card.EffectiveCMC(); mv != 9 {
		t.Errorf("printed MV should be 9, got %d", mv)
	}
	if dev := CountDevotion(gs, 0, "R"); dev != 0 {
		t.Errorf("printed {9} contributes 0 red devotion, got %d", dev)
	}
}

// (3) A normal cast clears a stale prototype override from a prior prototyped
// life — pins that the override is per-cast, not sticky once chosen.
func TestPrototype_NormalCastClearsStaleOverride(t *testing.T) {
	gs := newKWCombatGame(t)
	gs.Seats[0].Hat = &declineHat{}
	gs.Seats[0].ManaPool = 9
	card := protoCreatureCard("Skitterbeam Battalion")
	// Simulate leftover prototype state from a previous prototyped cast.
	if !ApplyPrototype(card) {
		t.Fatal("ApplyPrototype should set the override")
	}
	gs.Seats[0].Hand = []*Card{card}

	if err := CastSpell(gs, 0, card, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}
	if card.IsPrototyped() {
		t.Fatal("a normal cast must clear any stale prototype override")
	}
}

// (5) Prototype values are copiable (CR §718.3c/d): DeepCopy and
// MintTokenAsCopyOf both carry the prototype stats; the copy's Colors slice is
// independent of the source.
func TestPrototype_CopyCopiesPrototypeStats(t *testing.T) {
	card := protoCreatureCard("Skitterbeam Battalion")
	if !ApplyPrototype(card) {
		t.Fatal("ApplyPrototype should set the override")
	}

	dc := card.DeepCopy()
	if !dc.IsPrototyped() {
		t.Fatal("DeepCopy of a prototyped card must copy the prototype override")
	}
	if dc.EffectiveBasePower() != 2 || dc.EffectiveBaseToughness() != 2 || dc.EffectiveCMC() != 5 {
		t.Errorf("copy should be 2/2 MV-5, got %d/%d MV %d",
			dc.EffectiveBasePower(), dc.EffectiveBaseToughness(), dc.EffectiveCMC())
	}
	if len(dc.EffectiveColors()) != 1 || dc.EffectiveColors()[0] != "R" {
		t.Errorf("copy should be red, got %v", dc.EffectiveColors())
	}
	// Slice independence: mutating the copy's proto colors must not touch source.
	dc.Proto.Colors[0] = "G"
	if card.Proto.Colors[0] != "R" {
		t.Error("copy's prototype Colors slice must be independent of the source")
	}

	gs := newKWCombatGame(t)
	tok := MintTokenAsCopyOf(gs, card, 0, "")
	if tok == nil || !tok.IsPrototyped() || tok.EffectiveBasePower() != 2 {
		t.Fatalf("MintTokenAsCopyOf must copy the prototype stats (CR §718.3d); got %+v", tok)
	}
}

// A card cast normally is unaffected by the prototype keyword detector when the
// args are absent — PrototypeProfile declines rather than fabricating values.
func TestPrototypeProfile_DeclinesWhenArgsMissing(t *testing.T) {
	card := &Card{
		Name:  "Malformed",
		Types: []string{"creature"},
		AST: &gameast.CardAST{
			Name:      "Malformed",
			Abilities: []gameast.Ability{&gameast.Keyword{Name: "prototype"}}, // no args
		},
	}
	if _, ok := PrototypeProfile(card); ok {
		t.Fatal("PrototypeProfile must decline when the prototype args are missing")
	}
}
