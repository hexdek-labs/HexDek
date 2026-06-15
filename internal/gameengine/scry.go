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

	// Validate: top+bottom must be a proper PARTITION of `looked` — every
	// looked card appears EXACTLY ONCE across top+bottom, with no extras and
	// no duplicates. A count-only check (len(top)+len(bottom)==n) is NOT
	// sufficient: a Hat that returns the same card in BOTH top and bottom (or
	// drops one card and duplicates another) passes the count yet makes the
	// rebuild below append that card into the library TWICE — the r63
	// within-zone CardIdentity dup (a Hat top-empty fallback put cards[0] on
	// top but removed a DIFFERENT card from bottom, leaving cards[0] in both).
	// Fall back to "all on top" on any malformed split so Scry can never
	// corrupt the library regardless of Hat behavior.
	if !scryValidPartition(looked, top, bottom) {
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

	// Dispatch per-card trigger so cards like Elrond, Master of Healing
	// can react to scry events.
	FireCardTrigger(gs, "scry", map[string]interface{}{
		"seat":   seatIdx,
		"amount": count,
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

	// Validate: graveyard+top must be a proper PARTITION of `looked` (same
	// guard as Scry — a count-only check lets a Hat overlap/duplicate a card
	// across both halves, which would leave it in the library via `top` AND
	// route it to the graveyard via MoveCard below, a cross-zone CardIdentity
	// dup). Fall back to "all on top" on any malformed split.
	if !scryValidPartition(looked, graveyard, top) {
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

// scryValidPartition reports whether top+bottom is a proper partition of
// looked: each *Card in looked appears exactly once across top+bottom, and
// neither top nor bottom contains a card absent from looked. Pointer-identity
// based (the scry split moves the same *Card pointers). Guards Scry against a
// Hat returning a duplicated / overlapping split that would double-insert a
// card into the library (CR §701.18 — scry reorders, never duplicates).
func scryValidPartition(looked, top, bottom []*Card) bool {
	if len(top)+len(bottom) != len(looked) {
		return false
	}
	want := make(map[*Card]int, len(looked))
	for _, c := range looked {
		want[c]++
	}
	for _, c := range top {
		want[c]--
	}
	for _, c := range bottom {
		want[c]--
	}
	for _, n := range want {
		if n != 0 {
			return false
		}
	}
	return true
}
