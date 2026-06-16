package gameengine

// draw_n.go — DrawN, the single exported "draw N cards for a seat"
// primitive.
//
// WHY THIS EXISTS
//
// The canonical draw path is `gs.drawOne` (unexported) wrapped in the
// §614.11 replacement loop: NarsetBlocksDraw → FireDrawEvent (draw
// doublers / Lab-Maniac-style alt-win cancels) → drawOne →
// IncrementDrawCount → FireDrawTriggerObservers. Because `drawOne` is
// unexported, the per_card subpackage had no way to draw cards
// correctly — a per_card handler that wanted to draw could only call
// MoveCard("library"→"hand"), which silently SKIPS draw doublers,
// `card_drawn` triggers, the draw counter, and the observer fan-out
// (Smothering Tithe, Orcish Bowmasters, Consecrated Sphinx, …).
//
// As a result a whole class of "draw N" payoffs whose abilities parse
// to inert raw-text scaffold (e.g. Glimpse the Unthinkable's mirror,
// "draw two cards then proliferate", wheel variants) were left
// u/under-implementable from per_card. DrawN closes that gap: it is the
// exact per-seat loop `resolveDraw` runs, exported so any per_card
// handler (across every shard) can draw cards with full rules fidelity.
//
// CONTRACT
//
//	DrawN(gs, seat, n, src) -> number of cards actually drawn
//
//   - seat: the player drawing (NOT necessarily src.Controller — pass
//     the drawing seat explicitly so "target player draws" works).
//   - n: requested card count; n <= 0 is treated as a no-op (returns 0).
//   - src: the effect source for event attribution / replacement
//     context; may be nil (a synthetic spell source) — handlers pass the
//     resolving permanent or the transient spell-source Permanent.
//
// Mirrors the per-seat body of resolveDraw (resolve.go). Kept as a
// distinct exported function rather than refactoring resolveDraw to call
// it, so adopting DrawN carries zero risk of regressing the AST draw
// path — the two share the same primitives (FireDrawEvent / drawOne /
// IncrementDrawCount / FireDrawTriggerObservers) and therefore the same
// semantics.
func DrawN(gs *GameState, seat, n int, src *Permanent) int {
	if gs == nil || n <= 0 {
		return 0
	}
	if seat < 0 || seat >= len(gs.Seats) || gs.Seats[seat] == nil {
		return 0
	}

	drawn := 0
	for i := 0; i < n; i++ {
		// CR §702.52 — dredge replaces this would-draw if the player chooses.
		// A replaced draw is NOT a draw: skip drawOne + IncrementDrawCount.
		if MaybeDredgeReplaceDraw(gs, seat) {
			continue
		}
		// §614.11: replacement effects fire PER draw-card event.
		if NarsetBlocksDraw(gs, seat) {
			break
		}
		modifiedCount, cancelled := FireDrawEvent(gs, seat, src)
		if cancelled {
			continue
		}
		for k := 0; k < modifiedCount; k++ {
			if _, ok := gs.drawOne(seat); ok {
				drawn++
				IncrementDrawCount(gs, seat)
			}
		}
	}

	gs.LogEvent(Event{
		Kind:   "draw",
		Seat:   controllerSeat(src),
		Target: seat,
		Source: sourceName(src),
		Amount: drawn,
	})
	// Fire draw-trigger observers (Smothering Tithe, Orcish Bowmasters)
	// for the batch — mirrors resolveDraw / Python
	// _fire_draw_trigger_observers.
	if drawn > 0 {
		FireDrawTriggerObservers(gs, seat, drawn, false)
	}
	return drawn
}
