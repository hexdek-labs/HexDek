package gameengine

import "testing"

// layer5_color_r63_test.go — CR §613 layer-5 color-change audit.

// (b) protection from a color must read the SOURCE's LAYERED color (§613.1e),
// not its printed color. Pre-fix attackerHasProtectionFrom read
// cardColors(source.Card) (printed), so a creature recolored by a layer-5
// effect slipped past "protection from <that color>".
func TestLayer5_ProtectionFromColor_ReadsLayeredColor(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Repainted Knight", 2, 2, "creature")
	src.Card.Colors = []string{"W"} // printed white
	// Layer-5 "becomes red" (set, overriding printed).
	gs.RegisterContinuousEffect(&ContinuousEffect{
		Layer: LayerColor, Timestamp: gs.NextTimestamp(),
		SourcePerm: src, HandlerID: "l5_make_red",
		Predicate: func(_ *GameState, t *Permanent) bool { return t == src },
		ApplyFn:   func(_ *GameState, _ *Permanent, chars *Characteristics) { chars.Colors = []string{"R"} },
	})
	if cols := gs.ColorsOf(src); len(cols) != 1 || cols[0] != "R" {
		t.Fatalf("(a) layer-5 set should make src red (override printed white), got %v", cols)
	}

	prot := addBattlefield(gs, 1, "Guard", 2, 2, "creature")
	prot.Flags["prot:R"] = 1
	if !attackerHasProtectionFromGS(gs, prot, src) {
		t.Error("(b) protection from red must apply to a creature turned red by a layer-5 effect")
	}
	// The printed-color (gs-less) path still reads white — documents the gap the
	// layer-aware path closes.
	if attackerHasProtectionFrom(prot, src) {
		t.Error("printed-color path should read white (sanity check on the fixture)")
	}
}

// (b) end-to-end: Painter's Servant names red, a printed-white attacker becomes
// white+red, and a protection-from-red blocker takes no combat damage.
func TestLayer5_PainterColor_ProtectionPreventsCombatDamage(t *testing.T) {
	gs := newCombatGame(t)
	painter := addBattlefield(gs, 0, "Painter's Servant", 1, 3, "artifact", "creature")
	if painter.Flags == nil {
		painter.Flags = map[string]int{}
	}
	painter.Flags["painter_color_R"] = 1
	RegisterPaintersServant(gs, painter)

	atk := addCreature(gs, 0, "White Knight", 4, 4)
	atk.Card.Colors = []string{"W"} // printed white; Painter adds red
	if !containsColor(gs.ColorsOf(atk), "R") {
		t.Fatalf("fixture: Painter should make the attacker red, colors=%v", gs.ColorsOf(atk))
	}

	blk := addCreature(gs, 1, "Mother of Runes", 2, 2)
	blk.Flags["prot:R"] = 1

	CombatPhase(gs)

	if blk.MarkedDamage != 0 {
		t.Errorf("(b) prot-from-red blocker must take 0 from a Painter-reddened white attacker, got %d",
			blk.MarkedDamage)
	}
}

// (b) devotion DISTINCTION: devotion reads mana-cost pips, NOT current color.
// A white creature recolored black by a layer-5 effect must NOT add to black
// devotion (and still counts toward white devotion via its {W} pip).
func TestLayer5_Devotion_ReadsManaPipsNotLayeredColor(t *testing.T) {
	gs := newFixtureGame(t)
	c := addBattlefield(gs, 0, "Whitey", 2, 2, "creature")
	c.Card.Colors = []string{"W"}
	c.Card.ManaCostString = "{W}"
	// Layer-5: becomes black.
	gs.RegisterContinuousEffect(&ContinuousEffect{
		Layer: LayerColor, Timestamp: gs.NextTimestamp(),
		SourcePerm: c, HandlerID: "l5_make_black",
		Predicate: func(_ *GameState, t *Permanent) bool { return t == c },
		ApplyFn:   func(_ *GameState, _ *Permanent, chars *Characteristics) { chars.Colors = []string{"B"} },
	})
	if cols := gs.ColorsOf(c); len(cols) != 1 || cols[0] != "B" {
		t.Fatalf("fixture: creature should be black via layer 5, got %v", cols)
	}
	if got := DevotionToBlack(gs, 0); got != 0 {
		t.Errorf("devotion must read mana pips, not layered color — black devotion should be 0, got %d", got)
	}
	if got := DevotionToWhite(gs, 0); got != 1 {
		t.Errorf("white devotion should be 1 from the {W} pip, got %d", got)
	}
}

func containsColor(cols []string, want string) bool {
	for _, c := range cols {
		if c == want {
			return true
		}
	}
	return false
}
