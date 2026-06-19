package gameengine

// keywords_manifest.go — Manifest (CR §701.34, Fate Reforged 2015) as a
// real top-of-library face-down ETB helper with a flip-face-up entry
// point that restores the underlying card's identity.
//
// CR §701.34a: To manifest a card, the player turns it face down (it
//                becomes a 2/2 face-down creature with no abilities,
//                name, types, mana cost, or color), then puts it onto
//                the battlefield under their control.
// CR §701.34b: If a manifested card is a creature card, the player
//                may turn it face up at any time by paying its mana
//                cost. Turning it face up reveals its full
//                characteristics.
// CR §701.34c: "Manifest the top N cards of your library" is the
//                bulk variant — each top card is manifested in order.
//
// Engine surface (canonical):
//
//   - HasManifest(card) bool
//       Oracle-text detector for "manifest the top" preambles.
//
//   - ApplyManifestTop(gs, seatIdx, n) int
//       Manifests the top N cards of `seatIdx`'s library. Returns the
//       actual number manifested (may be less than n if the library
//       runs dry — CR §701.34c is silent about insufficient cards,
//       but the engine treats it as a "manifest as many as possible"
//       no-op for the rest).
//
//   - ManifestedFaceUp(gs, perm, cost) error
//       Flips a manifested permanent face up. CR §701.34b — pays the
//       given mana cost and clears the face-down overlay on the real card
//       (perm.Card in the overlay model) so the layer system reads its
//       printed characteristics. Rejects non-manifested perms.
//
//   - IsManifested(perm) bool
//       Reads perm.Flags["manifested"]. Backs "while manifested" gates.

import "strings"

// ---------------------------------------------------------------------------
// HasManifest
// ---------------------------------------------------------------------------

// HasManifest scans the card's oracle text for a "manifest the top"
// preamble (the canonical FRF / NEO printing of the Manifest keyword
// action). Used by tooling and tests to identify manifest cards
// without rerunning the AST parser.
func HasManifest(card *Card) bool {
	if card == nil {
		return false
	}
	text := strings.ToLower(OracleTextLower(card))
	if text == "" {
		return false
	}
	patterns := []string{
		"manifest the top",
		"manifest dread",         // manifest_dread variant
		"manifest a creature",    // some rare phrasings target a card
	}
	for _, p := range patterns {
		if strings.Contains(text, p) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// IsManifested
// ---------------------------------------------------------------------------

// IsManifested reports whether the permanent was placed on the
// battlefield via a manifest action — perm.Flags["manifested"] > 0.
// Distinguishes a Manifest face-down from a Morph face-down (CR
// §702.36); the two share the face-down state but trigger different
// "while face-down" abilities.
func IsManifested(perm *Permanent) bool {
	if perm == nil || perm.Flags == nil {
		return false
	}
	return perm.Flags["manifested"] > 0
}

// ---------------------------------------------------------------------------
// ApplyManifestTop
// ---------------------------------------------------------------------------

// ApplyManifestTop manifests the top `n` cards of `seatIdx`'s library
// per CR §701.34a/c, in the real-card overlay model: each manifested
// card itself becomes the face-down permanent (the permanent of record),
// so it can move zones, be owned, and fire dies-triggers correctly. Its
// §707.2 2/2-vanilla shape comes from the "manifest" FaceDownTemplate via
// the layers system — there is no synthetic wrapper and no OriginalCard
// orphan.
//
// Returns the actual number manifested. Less than `n` when the
// library runs dry; 0 when the library is empty up front, `n` is
// non-positive, or arguments are invalid.
//
// On success per card:
//   - The top library card is removed from the library.
//   - It becomes a face-down permanent: perm.Card = the real card with
//     Card.FaceDown=true, FaceDownTemplate="manifest", Flags
//     {"manifested":1,"face_down":1}, SummoningSick=true.
//   - ETB triggers fire (the §708.4 face-down gate suppresses the hidden
//     card's own self-triggers/statics); replacements skip while hidden.
//   - A "manifest" event is emitted (one per card, plus a final
//     summary event with Amount=count for tooling that batches).
func ApplyManifestTop(gs *GameState, seatIdx, n int) int {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return 0
	}
	if n <= 0 {
		return 0
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return 0
	}
	manifested := 0
	for i := 0; i < n && len(seat.Library) > 0; i++ {
		underlying := seat.Library[0]
		seat.Library = seat.Library[1:]
		if underlying == nil {
			continue
		}
		// CR §701.34a — the card itself becomes face down (real-card
		// overlay model): the REAL card stays the permanent of record, so
		// it can move zones, be owned, and fire dies-triggers correctly.
		// The §707.2 2/2-vanilla characteristics come from the "manifest"
		// FaceDownTemplate via the layers system — perm.Card's printed P/T
		// is ignored while face down. No synthetic wrapper, no OriginalCard
		// orphan (the bug the wrapper model caused: on leave the throwaway
		// wrapper went to the graveyard while the real card was lost).
		perm := &Permanent{
			Card:          underlying,
			Controller:    seatIdx,
			Owner:         underlying.Owner,
			Timestamp:     gs.NextTimestamp(),
			Counters:      map[string]int{},
			SummoningSick: true,
		}
		// Stamp the manifest overlay: Card.FaceDown, FaceDownTemplate=
		// "manifest", the "manifested" marker, and "face_down" so the ETB /
		// replacement face-down gates treat the real card's abilities as
		// absent (§708.4).
		makeFaceDown(gs, perm, "manifest", faceDownOpts{Markers: []string{"manifested", "face_down"}})

		seat.Battlefield = append(seat.Battlefield, perm)
		RegisterReplacementsForPermanent(gs, perm)
		FirePermanentETBTriggers(gs, perm)
		manifested++

		gs.LogEvent(Event{
			Kind:   "manifest",
			Seat:   seatIdx,
			Source: underlying.DisplayName(),
			Details: map[string]interface{}{
				"underlying_name": underlying.DisplayName(),
				"rule":            "701.34a",
			},
		})
	}

	// Summary event when callers asked for >1 — lets observers batch
	// without scanning the per-card events.
	if n > 1 {
		gs.LogEvent(Event{
			Kind:   "manifest_batch",
			Seat:   seatIdx,
			Amount: manifested,
			Details: map[string]interface{}{
				"requested": n,
				"completed": manifested,
				"rule":      "701.34c",
			},
		})
	}
	return manifested
}

// ---------------------------------------------------------------------------
// ManifestedFaceUp
// ---------------------------------------------------------------------------

// ManifestedFaceUp flips a manifested permanent face up. CR §701.34b
// — the controller may turn a manifested creature face up at any time
// by paying its mana cost (a special action that doesn't use the stack).
//
// Validation (atomic — no mutation on failure):
//   - perm must be non-nil and IsManifested
//   - the real card (perm.Card, in the overlay model) must be a creature
//     card (§701.34b — only creature cards can be turned face up via the
//     manifest right)
//   - perm.Controller's ManaPool must cover `cost`
//
// On success:
//   - Mana paid (logged as pay_mana with reason="manifest_flip_cost")
//   - the face-down overlay is cleared (Card.FaceDown=false,
//     FaceDownTemplate="", manifest markers removed) so the real card
//     reveals its printed characteristics
//   - the real card's replacement effects are re-registered (§708.4)
//   - the §613 cache is invalidated
//   - A "manifest_flip" event is emitted with the revealed card name
//
// Returns nil on success, *CastError on failure.
func ManifestedFaceUp(gs *GameState, perm *Permanent, cost int) error {
	if gs == nil {
		return &CastError{Reason: "nil_game"}
	}
	if perm == nil {
		return &CastError{Reason: "nil_perm"}
	}
	if !IsManifested(perm) {
		return &CastError{Reason: "not_manifested"}
	}
	// Real-card overlay model: the real card is already perm.Card (face
	// down). No OriginalCard swap is needed — turning face up is just
	// clearing the overlay.
	underlying := perm.Card
	if underlying == nil {
		return &CastError{Reason: "no_underlying_card"}
	}
	if !cardHasType(underlying, "creature") {
		// CR §701.34b — only creature cards can be turned face up via
		// the manifest right (non-creature manifests stay face-down
		// until cleared by other effects).
		return &CastError{Reason: "underlying_not_creature"}
	}
	if cost < 0 {
		return &CastError{Reason: "invalid_cost"}
	}
	seatIdx := perm.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return &CastError{Reason: "invalid_controller"}
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return &CastError{Reason: "nil_seat"}
	}
	if seat.ManaPool < cost {
		return &CastError{Reason: "insufficient_mana_for_flip"}
	}

	// Pay the flip cost.
	seat.ManaPool -= cost
	SyncManaAfterSpend(seat)
	if cost > 0 {
		gs.LogEvent(Event{
			Kind:   "pay_mana",
			Seat:   seatIdx,
			Amount: cost,
			Source: underlying.DisplayName(),
			Details: map[string]interface{}{
				"reason": "manifest_flip_cost",
				"rule":   "701.34b",
			},
		})
	}

	// Clear the face-down overlay — the real card (already perm.Card)
	// reveals its printed name, types, P/T, AST, and mana cost. Drop the
	// FaceDownTemplate and the manifest markers so the layers system stops
	// applying the §707.2 override and the face-down ETB/replacement gates
	// release.
	underlying.FaceDown = false
	perm.FaceDownTemplate = ""
	delete(perm.Flags, "manifested")
	delete(perm.Flags, "manifest_dread")
	delete(perm.Flags, "manifest_real_card_exists")
	delete(perm.Flags, "manifest_is_creature")
	delete(perm.Flags, "face_down")

	// The real card's static abilities (replacement effects) become active
	// now that it's face up — they were skipped at ETB by the §708.4
	// face-down guard. Turning face up is not an ETB, so we re-register
	// replacements without re-firing ETB triggers.
	RegisterReplacementsForPermanent(gs, perm)

	// §613 cache invalidation + new timestamp on the face change.
	perm.Timestamp = gs.NextTimestamp()
	gs.InvalidateCharacteristicsCache()

	gs.LogEvent(Event{
		Kind:   "manifest_flip",
		Seat:   seatIdx,
		Source: underlying.DisplayName(),
		Amount: cost,
		Details: map[string]interface{}{
			"revealed": underlying.DisplayName(),
			"rule":     "701.34b",
		},
	})
	FireCardTrigger(gs, "manifest_flipped", map[string]interface{}{
		"perm":            perm,
		"card":            underlying,
		"card_name":       underlying.DisplayName(),
		"controller_seat": seatIdx,
	})

	return nil
}
