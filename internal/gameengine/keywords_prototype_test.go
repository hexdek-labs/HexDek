package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// keywords_prototype_test.go — CR §702.160 / §718 Prototype.
//
// Pins the OVERLAY/MASK model (CR §718.4): the prototype P/T+color+MV apply
// ONLY while the object is a permanent on the battlefield (or a spell on the
// stack for casting). In every other zone the card reverts to its full printed
// characteristics BY CONSTRUCTION (the base is never written). Prototype values
// are copiable on the battlefield (CR §718.3c/d). Normal cast = full printed.

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

// castPrototyped casts the card prototyped (payHat chooses the cost) and returns
// the resolved battlefield permanent.
func castPrototyped(t *testing.T, gs *GameState, card *Card) *Permanent {
	t.Helper()
	gs.Seats[0].Hat = &payHat{}
	gs.Seats[0].ManaPool = 5 // prototype cost ({3}{R}{R} = 5)
	gs.Seats[0].Hand = []*Card{card}
	if err := CastSpell(gs, 0, card, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}
	perm := findProtoPerm(gs, 0, card.Name)
	if perm == nil {
		t.Fatal("prototyped creature should resolve onto the battlefield")
	}
	return perm
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

// (1)/(2) Cast for prototype → reduced cost paid; ON THE BATTLEFIELD the
// permanent wears the prototype P/T + color + MV, and devotion sees the reduced
// colored pips.
func TestPrototype_CastForPrototype_OnBattlefieldShowsPrototype(t *testing.T) {
	gs := newKWCombatGame(t)
	card := protoCreatureCard("Skitterbeam Battalion")
	perm := castPrototyped(t, gs, card)

	if gs.Seats[0].ManaPool != 0 {
		t.Errorf("prototype cost is 5; expected pool 0, got %d", gs.Seats[0].ManaPool)
	}
	if perm.Power() != 2 || perm.Toughness() != 2 {
		t.Errorf("prototype P/T should be 2/2, got %d/%d", perm.Power(), perm.Toughness())
	}
	if colors := gs.ColorsOf(perm); len(colors) != 1 || colors[0] != "R" {
		t.Errorf("prototyped permanent should be red, got %v", colors)
	}
	if mv := perm.ManaValue(); mv != 5 {
		t.Errorf("prototyped permanent MV should be 5, got %d", mv)
	}
	if dev := CountDevotion(gs, 0, "R"); dev != 2 {
		t.Errorf("devotion to red should see the 2 prototype red pips, got %d", dev)
	}
}

// Core of 7174n1c's fix: a prototyped permanent that LEAVES the battlefield
// (bounced to hand / dies to graveyard / exiled) shows its FULL PRINTED
// P/T + color + MV in that zone — the overlay applies only on the battlefield
// (CR §718.4). Verified for each destination zone.
func TestPrototype_RevertsToPrintedOffBattlefield(t *testing.T) {
	for _, dest := range []string{"hand", "graveyard", "exile"} {
		t.Run(dest, func(t *testing.T) {
			gs := newKWCombatGame(t)
			card := protoCreatureCard("Skitterbeam Battalion")
			_ = castPrototyped(t, gs, card)

			// Move the card off the battlefield (no more *Permanent).
			gs.Seats[0].Battlefield = nil
			switch dest {
			case "hand":
				gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)
			case "graveyard":
				gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, card)
			case "exile":
				gs.Seats[0].Exile = append(gs.Seats[0].Exile, card)
			}

			// Off the battlefield, bare-card reads return the printed base.
			if card.BasePower != 7 || card.BaseToughness != 5 {
				t.Errorf("%s: printed P/T should be 7/5, got %d/%d", dest, card.BasePower, card.BaseToughness)
			}
			if mv := card.EffectiveCMC(); mv != 9 {
				t.Errorf("%s: printed MV should be 9, got %d", dest, mv)
			}
			if cols := cardColors(card); len(cols) != 0 {
				t.Errorf("%s: printed Skitterbeam is colorless, got %v", dest, cols)
			}
			// No battlefield permanent → contributes 0 devotion.
			if dev := CountDevotion(gs, 0, "R"); dev != 0 {
				t.Errorf("%s: off-battlefield card contributes 0 red devotion, got %d", dest, dev)
			}
		})
	}
}

// (4) Cast normally → full printed version on the battlefield: full cost,
// printed P/T, printed MV, no prototype color, no override.
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
	if mv := perm.ManaValue(); mv != 9 {
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
	if !ApplyPrototype(card) { // leftover prototype state from a previous cast
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

// (5) Prototype values are copiable on the battlefield (CR §718.3c/d):
// DeepCopy / MintTokenAsCopyOf carry the prototype values, and a copy placed on
// the battlefield wears the prototype overlay. The copy's Colors slice is
// independent of the source.
func TestPrototype_CopyOnBattlefieldCopiesPrototype(t *testing.T) {
	card := protoCreatureCard("Skitterbeam Battalion")
	if !ApplyPrototype(card) {
		t.Fatal("ApplyPrototype should set the override")
	}

	// The copiable VALUES travel with the card object.
	dc := card.DeepCopy()
	if dc.Proto == nil || dc.Proto.Power != 2 || dc.Proto.Toughness != 2 || dc.Proto.CMC != 5 {
		t.Fatalf("DeepCopy must copy the prototype values, got %+v", dc.Proto)
	}
	if len(dc.Proto.Colors) != 1 || dc.Proto.Colors[0] != "R" {
		t.Errorf("copy prototype color should be [R], got %v", dc.Proto.Colors)
	}
	// Slice independence: mutating the copy's proto colors must not touch source.
	dc.Proto.Colors[0] = "G"
	if card.Proto.Colors[0] != "R" {
		t.Error("copy's prototype Colors slice must be independent of the source")
	}

	// A token copy placed on the battlefield wears the prototype overlay.
	gs := newKWCombatGame(t)
	tok := MintTokenAsCopyOf(gs, card, 0, "")
	if tok == nil || tok.Proto == nil {
		t.Fatal("MintTokenAsCopyOf must copy the prototype values (CR §718.3d)")
	}
	perm := &Permanent{Card: tok, Controller: 0, Owner: 0, Counters: map[string]int{}, Flags: map[string]int{}, Timestamp: gs.NextTimestamp()}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)
	if perm.Power() != 2 || perm.Toughness() != 2 {
		t.Errorf("copy on battlefield should be 2/2, got %d/%d", perm.Power(), perm.Toughness())
	}
	if cols := gs.ColorsOf(perm); len(cols) != 1 || cols[0] != "R" {
		t.Errorf("copy on battlefield should be red, got %v", cols)
	}
	if mv := perm.ManaValue(); mv != 5 {
		t.Errorf("copy on battlefield MV should be 5, got %d", mv)
	}
}

// PrototypeProfile declines rather than fabricating values when args are absent.
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
