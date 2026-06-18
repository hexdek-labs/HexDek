package gameengine

// r63 mechanic-probe (PLOT, CR §702.172). CastPlot hand-rolled the stack push
// and skipped every cast-side side effect the canonical CastFromZone pipeline
// performs: it fired no "whenever you cast a spell" triggers and the plot cast
// counted toward nothing (magecraft / prowess / storm / second-spell payoffs).
// The fix routes the cast-side bookkeeping + trigger dispatch through the same
// helpers CastFromZone uses, while keeping CastPlot's historical contract of
// leaving resolution to the caller (the spell sits on the stack).
//
// This regression proves the free cast now (a) fires cast-trigger observers
// (prowess buffs a Monastery Swiftspear), and (b) is counted by the cast-count
// bookkeeping (SpellsCast / SpellsCastThisTurn / CastFromExile).

import "testing"

func TestCastPlot_FiresCastTriggersAndCounts(t *testing.T) {
	gs := newPlotGame(t)
	gs.Turn = 3
	gs.Seats[0].ManaPool = 5

	// A prowess creature is the observable proof that "whenever you cast a
	// spell" abilities see the plot cast.
	prowess := addCreature(gs, 0, "Monastery Swiftspear", 1, 2, "prowess")

	// Plot a noncreature (sorcery) spell on turn 3.
	card := newPlotCard("Slickshot Show-Off", 0, 3, "{2}")
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)
	if _, err := ActivatePlot(gs, 0, card, 2); err != nil {
		t.Fatalf("ActivatePlot failed: %v", err)
	}

	// Cast it for free on a later turn.
	gs.Turn = 4
	if _, err := CastPlot(gs, 0, card); err != nil {
		t.Fatalf("CastPlot failed: %v", err)
	}

	// (a) Cast-trigger observers fired — prowess gave +1/+1.
	if got := powerBuff(prowess); got != 1 {
		t.Fatalf("prowess should fire on the plot cast (+1/+1); got power buff %d "+
			"(cast-trigger dispatch missing from CastPlot)", got)
	}
	if prowessEventCount(gs) != 1 {
		t.Fatalf("expected exactly 1 prowess event from the plot cast, got %d", prowessEventCount(gs))
	}

	// (b) Cast-count bookkeeping ran — the plot cast IS a cast (CR §601.2i).
	if gs.Seats[0].Turn.SpellsCast != 1 {
		t.Fatalf("plot cast should increment seat Turn.SpellsCast, got %d", gs.Seats[0].Turn.SpellsCast)
	}
	if gs.SpellsCastThisTurn != 1 {
		t.Fatalf("plot cast should increment gs.SpellsCastThisTurn, got %d", gs.SpellsCastThisTurn)
	}
	if gs.Seats[0].Turn.CastFromExile != 1 {
		t.Fatalf("plot cast should increment Turn.CastFromExile, got %d", gs.Seats[0].Turn.CastFromExile)
	}

	// The historical contract still holds: the spell sits on the stack
	// (CastPlot does not resolve inline) and no mana was spent on the cast.
	if len(gs.Stack) != 1 {
		t.Fatalf("expected the plot spell to remain on the stack (1 item), got %d", len(gs.Stack))
	}
	if !IsPlotCast(gs.Stack[0]) {
		t.Fatal("the stack item should still be flagged as a plot cast")
	}
}
