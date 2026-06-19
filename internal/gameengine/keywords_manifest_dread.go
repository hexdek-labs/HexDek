package gameengine

// keywords_manifest_dread.go — Manifest Dread (CR §701.62, Duskmourn:
// House of Horror 2024) with explicit caller-controlled choice.
//
// CR §701.62a: "To manifest dread a card, look at the top two cards of
//               your library. Manifest one of them, then put the other
//               into your graveyard."
// CR §701.40b: "To manifest a card, put it onto the battlefield face
//               down. It's a 2/2 creature card with no text, no name,
//               no subtypes, and no mana cost."
//
// The package already exposes a no-argument ManifestDread in
// keywords_batch5.go that auto-picks "prefer the first creature" — fine
// for fixture / smoke tests but it doesn't model the actual choice the
// printed rules give the player. ApplyManifestDread is the canonical
// per_card / hat-driven entry point: the caller passes a callback
// inspecting the two peeked cards and returning the index (0 or 1) of
// the card to manifest.
//
// Boundary cases:
//
//   - Empty library  → no-op, returns nil.
//   - Library of one → manifest that one (no graveyard side effect),
//                      callback is NOT invoked (no choice to make).
//   - Callback returns out-of-range index → defaults to 0 (first card)
//                                            and logs a warning event.
//   - Nil callback   → defaults to 0.
//
// All paths log structured manifest_dread events so analytics / hat
// observability can reconstruct the decision chain.

// ApplyManifestDread implements the §701.62 manifest-dread action.
// Returns the created face-down Permanent, or nil when the library is
// empty (or when manifest construction otherwise fails, e.g. invalid
// seat). The chosen card is moved from library to battlefield face
// down as a 2/2; the other card (if any) goes to the graveyard.
//
// choiceCallback signature: receives the top two peeked cards in
// library order ([0]=top, [1]=second-from-top) and returns 0 or 1 to
// indicate which to manifest. Treated as defaulting to 0 when nil or
// when returning an out-of-range value. Cards that are nil in the
// callback slot are still passed through (the library content is the
// source of truth) but the caller should be defensive.
func ApplyManifestDread(gs *GameState, seatIdx int, choiceCallback func(top2 [2]*Card) int) *Permanent {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil
	}
	seat := gs.Seats[seatIdx]
	if seat == nil || len(seat.Library) == 0 {
		return nil
	}

	// Library of one — manifest that card, no choice, no mill.
	if len(seat.Library) == 1 {
		chosen := seat.Library[0]
		seat.Library = seat.Library[1:]
		gs.LogEvent(Event{
			Kind:   "manifest_dread",
			Seat:   seatIdx,
			Source: "Manifested Creature",
			Details: map[string]interface{}{
				"looked":    1,
				"rule":      "701.62",
				"choice":    0,
				"no_mill":   true,
				"face_down": true,
			},
		})
		return makeManifestDreadPermanent(gs, seatIdx, chosen)
	}

	// Peek top two. We don't snip them off the library yet — the
	// callback only LOOKS, and the unchosen card will be moved through
	// MoveCard (which expects the card to still be in the source zone).
	var top2 [2]*Card
	top2[0] = seat.Library[0]
	top2[1] = seat.Library[1]

	gs.LogEvent(Event{
		Kind:   "manifest_dread_peek",
		Seat:   seatIdx,
		Source: "Manifested Creature",
		Details: map[string]interface{}{
			"top0": cardDisplayName(top2[0]),
			"top1": cardDisplayName(top2[1]),
			"rule": "701.62",
		},
	})

	choice := 0
	if choiceCallback != nil {
		picked := choiceCallback(top2)
		if picked < 0 || picked > 1 {
			gs.LogEvent(Event{
				Kind:   "manifest_dread_invalid_choice",
				Seat:   seatIdx,
				Amount: picked,
				Details: map[string]interface{}{
					"rule":      "701.62",
					"defaulted": 0,
				},
			})
		} else {
			choice = picked
		}
	}

	chosen := top2[choice]
	otherIdx := 1 - choice
	other := top2[otherIdx]

	// Mill the unchosen card FIRST (it's still in the library at
	// position otherIdx). MoveCard handles library→graveyard with the
	// right zone-change triggers + invariant accounting.
	if other != nil {
		MoveCard(gs, other, seatIdx, ZoneLibrary, ZoneGraveyard, "manifest_dread_mill")
	} else {
		// Defensive: nil slot — just remove it by index. Should not
		// happen in practice but guards against malformed libraries.
		seat.Library = append(seat.Library[:otherIdx], seat.Library[otherIdx+1:]...)
	}

	// At this point the chosen card has shifted to the top of the
	// library (or stays at top if it was already at index 0).
	// Re-find it by identity and snip.
	chosenIdx := -1
	for i, c := range seat.Library {
		if c == chosen {
			chosenIdx = i
			break
		}
	}
	if chosenIdx >= 0 {
		seat.Library = append(seat.Library[:chosenIdx], seat.Library[chosenIdx+1:]...)
	}

	gs.LogEvent(Event{
		Kind:   "manifest_dread",
		Seat:   seatIdx,
		Source: "Manifested Creature",
		Details: map[string]interface{}{
			"looked":      2,
			"rule":        "701.62",
			"choice":      choice,
			"chosen_card": cardDisplayName(chosen),
			"milled_card": cardDisplayName(other),
			"face_down":   true,
		},
	})

	return makeManifestDreadPermanent(gs, seatIdx, chosen)
}

// makeManifestDreadPermanent puts the chosen card onto the battlefield
// face down per §701.40b/§701.62, in the real-card overlay model: the
// chosen REAL card is the permanent of record (so it can move zones, be
// owned, and fire dies-triggers) and its §707.2 2/2-vanilla shape comes
// from the "manifest" FaceDownTemplate. This replaces the old synthetic
// "Manifested Creature" wrapper, which left the chosen real card orphaned
// entirely (it was pulled from the library but only its AST was kept on
// BackFaceAST — a zone-conservation leak on leave). Sets flags so per_card
// / SBA code can distinguish manifest-dread origin from plain manifest.
func makeManifestDreadPermanent(gs *GameState, seatIdx int, chosen *Card) *Permanent {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) || chosen == nil {
		return nil
	}
	perm := &Permanent{
		Card:          chosen,
		Controller:    seatIdx,
		Owner:         chosen.Owner,
		SummoningSick: true,
		Timestamp:     gs.NextTimestamp(),
		Counters:      map[string]int{},
	}
	// Stamp the manifest overlay (FaceDownTemplate="manifest") via the
	// unified primitive, preserving the manifest-dread markers + "face_down"
	// so the ETB / replacement face-down gates treat the real card's
	// abilities as absent (§708.4).
	makeFaceDown(gs, perm, "manifest", faceDownOpts{
		Markers: []string{"manifested", "manifest_dread", "face_down", "manifest_real_card_exists"},
	})
	if cardHasType(chosen, "creature") {
		perm.Flags["manifest_is_creature"] = 1
	}

	seat := gs.Seats[seatIdx]
	seat.Battlefield = append(seat.Battlefield, perm)
	RegisterReplacementsForPermanent(gs, perm)
	FirePermanentETBTriggers(gs, perm)
	return perm
}

// cardDisplayName is a nil-safe DisplayName for log details where the
// card pointer might legitimately be nil (e.g. peek slots in a tiny
// library).
func cardDisplayName(c *Card) string {
	if c == nil {
		return "<nil>"
	}
	return c.DisplayName()
}
