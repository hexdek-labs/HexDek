package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// r63 protection probe — CR §702.16 DEBT consistency across the damage,
// blocking, and targeting axes. Before this fix the engine used TWO
// incompatible flag vocabularies for protection-from-color: combat read
// prot:<LETTER> (Akroma, Animar, Progenitus) + the AST "protection from
// <color>" raw text, while targeting read protection_from_<colorname>
// (Mother of Runes, Akroma's Will). A card that set only one was protected
// on only some axes. And non-combat damage honored none.

func pu_game() *GameState { return NewGameState(2, rand.New(rand.NewSource(9)), nil) }

func pu_creature(gs *GameState, seat int, name string, flags map[string]int, colors ...string) *Permanent {
	p := &Permanent{
		Card: &Card{Name: name, Owner: seat, Types: []string{"creature"},
			BasePower: 2, BaseToughness: 2, Colors: append([]string(nil), colors...)},
		Controller: seat, Owner: seat, Counters: map[string]int{},
		Flags: map[string]int{},
	}
	for k, v := range flags {
		p.Flags[k] = v
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

// (a) Non-combat damage from a source of the quality is prevented.
func TestProtection_NonCombatDamagePrevented(t *testing.T) {
	gs := pu_game()
	// prot:R representation (combat-vocab) — must now also stop non-combat damage.
	target := pu_creature(gs, 0, "Silver Knight", map[string]int{"prot:R": 1})
	redSrc := pu_creature(gs, 1, "Red Pinger", map[string]int{}, "R")

	if got := PreventDamageToPermanent(gs, target, 3, redSrc); got != 0 {
		t.Errorf("non-combat red damage to a protection-from-red creature must be prevented, got %d", got)
	}
	// A non-red source still deals full damage.
	blueSrc := pu_creature(gs, 1, "Blue Pinger", map[string]int{}, "U")
	if got := PreventDamageToPermanent(gs, target, 3, blueSrc); got != 3 {
		t.Errorf("non-matching colored source should deal full damage, got %d", got)
	}
}

// (a)/(e) protection_from_<colorname> vocabulary (Mother of Runes / Akroma's
// Will) must ALSO stop combat-class damage — previously combat ignored it.
func TestProtection_GrantVocabStopsDamage(t *testing.T) {
	gs := pu_game()
	target := pu_creature(gs, 0, "Bears", map[string]int{"protection_from_red": 1})
	redSrc := pu_creature(gs, 1, "Goblin", map[string]int{}, "R")
	if got := PreventDamageToPermanent(gs, target, 4, redSrc); got != 0 {
		t.Errorf("Mother-of-Runes (protection_from_red) must prevent red damage in combat-class path, got %d", got)
	}
}

// (d) Targeting: a creature with prot:R (Akroma vocabulary) can't be
// targeted by a red source — previously targeting read only
// protection_from_red and wrongly allowed it.
func TestProtection_ProtRVocabBlocksTargeting(t *testing.T) {
	gs := pu_game()
	akroma := pu_creature(gs, 1, "Akroma, Angel of Wrath",
		map[string]int{"prot:R": 1, "prot:B": 1})
	redBolt := &Card{Name: "Lightning Bolt", Types: []string{"instant"}, Colors: []string{"R"}}
	blackKill := &Card{Name: "Doom Blade", Types: []string{"instant"}, Colors: []string{"B"}}
	blueDraw := &Card{Name: "Brainstorm", Types: []string{"instant"}, Colors: []string{"U"}}

	if CanBeTargetedByCombat(akroma, 0, redBolt) {
		t.Error("Akroma (prot:R) must not be targetable by a red spell")
	}
	if CanBeTargetedByCombat(akroma, 0, blackKill) {
		t.Error("Akroma (prot:B) must not be targetable by a black spell")
	}
	if !CanBeTargetedByCombat(akroma, 0, blueDraw) {
		t.Error("Akroma must remain targetable by a blue spell")
	}
}

// (d)/(g) AST "protection from everything" / from-color raw text blocks
// targeting too (combat protectionColors source of truth).
func TestProtection_ASTRawBlocksTargeting(t *testing.T) {
	gs := pu_game()
	p := pu_creature(gs, 1, "Progenitus", map[string]int{})
	p.Card.AST = &gameast.CardAST{Name: "Progenitus",
		Abilities: []gameast.Ability{&gameast.Keyword{Name: "protection", Raw: "Protection from everything"}}}
	anySrc := &Card{Name: "Swords to Plowshares", Types: []string{"instant"}, Colors: []string{"W"}}
	if CanBeTargetedByCombat(p, 0, anySrc) {
		t.Error("protection from everything must block all targeting")
	}
	colorless := &Card{Name: "Spatial Contortion", Types: []string{"instant"}}
	if CanBeTargetedByCombat(p, 0, colorless) {
		t.Error("protection from everything must block even a colorless source")
	}
}

// (e) Layered color: a creature turned red by a layer-5 effect is matched
// against protection-from-red for non-combat damage (attackerHasProtectionFromGS
// reads ColorsOf, not printed Card.Colors).
func TestProtection_LayeredSourceColorDamage(t *testing.T) {
	gs := pu_game()
	target := pu_creature(gs, 0, "White Knight", map[string]int{"prot:R": 1})
	// Source printed blue, but a layer-5 effect makes it RED.
	src := pu_creature(gs, 1, "Painted Drake", map[string]int{}, "U")
	gs.RegisterContinuousEffect(&ContinuousEffect{
		Layer: LayerColor, Timestamp: gs.NextTimestamp(),
		SourcePerm: src, HandlerID: "pu_make_red",
		Predicate: func(_ *GameState, t *Permanent) bool { return t == src },
		ApplyFn:   func(_ *GameState, _ *Permanent, chars *Characteristics) { chars.Colors = []string{"R"} },
	})
	if cols := gs.ColorsOf(src); len(cols) != 1 || cols[0] != "R" {
		t.Fatalf("fixture: source should be red, got %v", cols)
	}
	if got := PreventDamageToPermanent(gs, target, 2, src); got != 0 {
		t.Errorf("a source turned red (layer 5) must be stopped by protection-from-red, got %d", got)
	}
}
