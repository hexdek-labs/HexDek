package gameengine

// Scry and Surveil — CR §701.18 / §701.46 with real library reorder.
//
// Comp-rules citations:
//
//   §701.18a — "To scry N means to look at the top N cards of your
//              library, then put any number of them on the bottom of
//              your library in any order and the rest on top of your
//              library in any order."
//   §701.18b — "If a player is instructed to scry 0, no scry event
//              occurs."
//   §701.46a — "To surveil N means to look at the top N cards of your
//              library, then put any number of them into your graveyard
//              and the rest on top of your library in any order."
//
// These functions mutate the library in place, using the seat's Hat to
// make the top/bottom/graveyard decision.

// Scry implements CR §701.18 — look at top N, Hat decides which go on
// top (in order) and which go on bottom (in order).
func Scry(gs *GameState, seatIdx int, count int) {
	if gs == nil || count <= 0 || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil || len(seat.Library) == 0 {
		return
	}

	// Look at top N.
	n := count
	if n > len(seat.Library) {
		n = len(seat.Library)
	}
	looked := make([]*Card, n)
	copy(looked, seat.Library[:n])

	// Hat decides top vs bottom.
	var top, bottom []*Card
	if seat.Hat != nil {
		top, bottom = seat.Hat.ChooseScry(gs, seatIdx, looked)
	}

	// Validate: if hat didn't return valid splits, default to all on top.
	if len(top)+len(bottom) != n {
		top = looked
		bottom = nil
	}

	// Rebuild the library: top cards first, then remaining library, then bottom.
	remaining := make([]*Card, len(seat.Library)-n)
	copy(remaining, seat.Library[n:])

	newLib := make([]*Card, 0, len(seat.Library))
	newLib = append(newLib, top...)
	newLib = append(newLib, remaining...)
	newLib = append(newLib, bottom...)
	seat.Library = newLib

	gs.LogEvent(Event{
		Kind:   "scry",
		Seat:   seatIdx,
		Amount: count,
		Details: map[string]interface{}{
			"looked":      n,
			"kept_on_top": len(top),
			"to_bottom":   len(bottom),
			"rule":        "701.18",
		},
	})
}

// Surveil implements CR §701.46 — look at top N, Hat decides which go
// to graveyard and which stay on top (in order).
func Surveil(gs *GameState, seatIdx int, count int) {
	if gs == nil || count <= 0 || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil || len(seat.Library) == 0 {
		return
	}

	// Look at top N.
	n := count
	if n > len(seat.Library) {
		n = len(seat.Library)
	}
	looked := make([]*Card, n)
	copy(looked, seat.Library[:n])

	// Hat decides graveyard vs top.
	var graveyard, top []*Card
	if seat.Hat != nil {
		graveyard, top = seat.Hat.ChooseSurveil(gs, seatIdx, looked)
	}

	// Validate: if hat didn't return valid splits, default to all on top.
	if len(graveyard)+len(top) != n {
		top = looked
		graveyard = nil
	}

	// Rebuild the library first (top cards + remaining), THEN route
	// graveyard-bound cards through MoveCard so §614/§903.9b replacements
	// and surveil triggers fire. Cards have already been pulled from the
	// library at this point, so MoveCard's library-source removal is a
	// no-op — the append-to-graveyard is the work that matters here.
	remaining := make([]*Card, len(seat.Library)-n)
	copy(remaining, seat.Library[n:])

	newLib := make([]*Card, 0, len(seat.Library)-len(graveyard))
	newLib = append(newLib, top...)
	newLib = append(newLib, remaining...)
	seat.Library = newLib

	for _, c := range graveyard {
		MoveCard(gs, c, seatIdx, "library", "graveyard", "surveil")
	}

	gs.LogEvent(Event{
		Kind:   "surveil",
		Seat:   seatIdx,
		Amount: count,
		Details: map[string]interface{}{
			"looked":      n,
			"to_graveyard": len(graveyard),
			"kept_on_top":  len(top),
			"rule":         "701.46",
		},
	})
}
