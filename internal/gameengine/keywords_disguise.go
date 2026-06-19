package gameengine

// keywords_disguise.go — CR §702.166 Disguise (Murders at Karlov Manor, 2024).
//
// Disguise is the modern (post-2024) Morph variant. A card with disguise
// has two new abilities, per §702.166a:
//
//   1. "You may cast this card face down for {3} as a 2/2 creature with
//      ward {2}."                                                — §702.166a
//   2. "You may turn this permanent face up any time for its disguise
//      cost."                                                    — §702.166b
//
// Differences from Morph (§702.36 / §702.37):
//
//   - The face-down creature has ward {2} (Morph face-downs have no
//     abilities at all).
//   - The face-up activation can happen at any time, NOT just at sorcery
//     speed (Morph's turn-face-up is also any time, but disguise inherits
//     the same flexibility and is explicitly "any time").
//
// Architecture:
//
//   - We model the cast face-down as a CR §601.2f alternative cost. The
//     card's FaceDown flag is stamped before any cost is paid, the {3} is
//     paid, and a StackItem with CostMeta["disguise_face_down"]=true is
//     constructed (for the alt-cost trail / cast analytics) before the
//     spell resolves into a face-down 2/2 creature on the battlefield.
//
//   - The face-down permanent carries the runtime flags HasKeyword reads:
//     Flags["kw:ward"]=1 + Flags["ward_cost"]=2, so CheckWardOnTargeting
//     in stack.go sees the ward without needing AST surgery (which would
//     be wrong because §613.2b strips the AST keywords from the face-down
//     characteristics in the layers system). The ward keyword on a
//     face-down disguise creature is granted by §702.166a, not by the
//     card's own AST, so the runtime-flag path is the correct seat for it.
//
//   - TurnFaceUpDisguise pays the disguise cost (extracted from the
//     card's AST keyword args via keywordArgCost), clears the
//     ward-from-disguise flags, and delegates to TurnFaceUp in dfc.go for
//     the actual flip — that helper handles Card.FaceDown clearing,
//     timestamp bump, and the layers cache invalidation. The full
//     printed characteristics (name, P/T, abilities) come back via the
//     existing layers system, which simply stops applying the §707.2
//     face-down override once Card.FaceDown is false.
//
// Reused plumbing: PayGenericCost, EnsureTypedPool, PushStackItem, the
// FaceDown override in layers.go BaseCharacteristics, CheckWardOnTargeting
// in stack.go, TurnFaceUp in dfc.go, IsFaceDown in keywords_batch.go,
// MoveCard's existing FaceDown reset on zone change in phases.go.

// DisguiseFaceDownCost is the alternative cast cost for a disguise spell
// cast face down — {3}, per §702.166a.
const DisguiseFaceDownCost = 3

// DisguiseFaceDownWardCost is the ward cost granted to a face-down disguise
// creature — ward {2}, per §702.166a.
const DisguiseFaceDownWardCost = 2

// HasDisguise reports whether the card carries the disguise keyword.
func HasDisguise(card *Card) bool {
	return cardHasKeywordByName(card, "disguise")
}

// DisguiseCost returns the printed disguise cost (the cost to turn the
// permanent face up). 0 if the card has no disguise keyword args.
//
// We fall back to keywordArgCost which itself falls back to the card's CMC
// when no numeric arg is parsed. Disguise costs are printed (and parsed)
// the same way as Morph costs, so the existing helper covers both.
func DisguiseCost(card *Card) int {
	return keywordArgCost(card, "disguise")
}

// CastDisguiseFaceDown casts `card` from `seatIdx`'s hand face-down for
// the disguise alt cost of {3}. CR §702.166a + §601.2f.
//
// On success:
//   - Pays {3} from `seatIdx`'s mana pool.
//   - Removes `card` from hand.
//   - Sets `card.FaceDown = true` (so the layers system applies the
//     §707.2 / §613.2b face-down characteristic override).
//   - Pushes a StackItem flagged with CostMeta["disguise_face_down"]=true
//     and CostMeta["alt_cost"]="disguise" — the alt-cost trail per
//     §601.2f. The item is then resolved immediately into a face-down 2/2
//     creature with ward {2} on `seatIdx`'s battlefield.
//
// Returns the newly created face-down permanent on success, nil + a
// CastError on failure (no keyword, not in hand, can't pay).
//
// The cast does NOT enforce sorcery speed: face-down disguise spells are
// cast as the card's normal type (so an instant printed with disguise
// could be cast face-down at instant speed); §702.166a inherits the
// timing of the card's true type. Callers that need sorcery-speed gating
// (the most common case for creature-printed disguise) must enforce it
// before calling.
func CastDisguiseFaceDown(gs *GameState, seatIdx int, card *Card) (*Permanent, error) {
	if gs == nil {
		return nil, &CastError{Reason: "nil game"}
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil, &CastError{Reason: "invalid seat"}
	}
	if card == nil {
		return nil, &CastError{Reason: "nil card"}
	}
	if !HasDisguise(card) {
		return nil, &CastError{Reason: "no_disguise_keyword"}
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return nil, &CastError{Reason: "nil seat"}
	}

	// Mana check + payment.
	pool := EnsureTypedPool(seat)
	if pool.Total() < DisguiseFaceDownCost {
		return nil, &CastError{Reason: "insufficient_mana"}
	}
	if !PayGenericCost(gs, seat, DisguiseFaceDownCost, "disguise",
		"cast_face_down", card.DisplayName()) {
		seat.ManaPool -= DisguiseFaceDownCost
		SyncManaAfterSpend(seat)
	}

	// Remove from hand.
	handIdx := -1
	for i, c := range seat.Hand {
		if c == card {
			handIdx = i
			break
		}
	}
	if handIdx < 0 {
		// Refund — caller violated the contract by passing a card not in hand.
		seat.ManaPool += DisguiseFaceDownCost
		SyncManaAfterSpend(seat)
		return nil, &CastError{Reason: "not_in_hand"}
	}
	seat.Hand = append(seat.Hand[:handIdx], seat.Hand[handIdx+1:]...)

	// Stamp the alt-cost trail per §601.2f. The StackItem is constructed
	// for analytics / hat introspection (and for future resolve-time hooks
	// like Kadena's cost reduction). We push it onto the stack and then
	// immediately materialize the permanent — face-down spells resolve
	// without a meaningful on-stack response window distinct from the
	// face-down-creature ETB, and the existing Morph path (CastFaceDown
	// in keywords_batch.go) also short-circuits the stack.
	card.FaceDown = true
	item := &StackItem{
		Card:       card,
		Controller: seatIdx,
		CastZone:   ZoneHand,
		CostMeta: map[string]interface{}{
			"disguise_face_down": true,
			"alt_cost":           "disguise",
			"alt_cost_paid":      DisguiseFaceDownCost,
		},
	}
	PushStackItem(gs, item)

	perm := materializeDisguiseFaceDown(gs, seatIdx, card)

	// Pop the item we just pushed — we resolved it in-place. We don't
	// route through ResolveStackTop because the resolution path for a
	// face-down creature spell is just "put it onto the battlefield as a
	// 2/2", which we've already done. Leaving the item on the stack would
	// mislead any priority-pass logic.
	if len(gs.Stack) > 0 && gs.Stack[len(gs.Stack)-1] == item {
		gs.Stack = gs.Stack[:len(gs.Stack)-1]
	}

	gs.LogEvent(Event{
		Kind:   "cast_face_down",
		Seat:   seatIdx,
		Source: "face-down disguise creature",
		Amount: DisguiseFaceDownCost,
		Details: map[string]interface{}{
			"alt_cost": "disguise",
			"rule":     "702.166a",
		},
	})

	return perm, nil
}

// materializeDisguiseFaceDown drops the face-down 2/2 ward {2} creature
// onto seatIdx's battlefield. Split out from CastDisguiseFaceDown so the
// resolve path can be invoked from per-card handlers (e.g. effects that
// "put a disguise card face down onto the battlefield" without going
// through cast).
func materializeDisguiseFaceDown(gs *GameState, seatIdx int, card *Card) *Permanent {
	perm := &Permanent{
		Card:          card,
		Controller:    seatIdx,
		Owner:         card.Owner,
		SummoningSick: true,
		Timestamp:     gs.NextTimestamp(),
		Counters:      map[string]int{},
	}
	// Stamp the disguise face-down overlay through the unified primitive:
	// Card.FaceDown, FaceDownTemplate="disguise", ward {2} (from the
	// template), plus the family markers HasKeyword / PayMorphCost / Kadena
	// / TurnFaceUpDisguise read.
	makeFaceDown(gs, perm, "disguise", faceDownOpts{
		Markers: []string{"face_down", "morph_creature", "disguise_face_down"},
	})
	gs.Seats[seatIdx].Battlefield = append(gs.Seats[seatIdx].Battlefield, perm)
	RegisterReplacementsForPermanent(gs, perm)
	FirePermanentETBTriggers(gs, perm)
	return perm
}

// TurnFaceUpDisguise pays the disguise cost and turns `perm` face-up.
// CR §702.166b: "You may turn this permanent face up any time for its
// disguise cost."
//
// Validates that:
//   - `perm` is currently a face-down disguise creature (Flags
//     ["disguise_face_down"]=1 AND IsFaceDown). This is a stricter check
//     than the generic morph turn-face-up — Morph and Disguise can both
//     live on the same battlefield and we don't want to accept a
//     morph-cost payment for a disguise permanent (or vice versa).
//   - The controller can pay `cost` from their mana pool. The caller is
//     responsible for matching `cost` to the printed disguise cost; we
//     don't re-parse the AST here because at face-down time the AST
//     keyword Args may have already been frozen into the permanent at
//     ETB by the layers system, and TurnFaceUpDisguise wants a clean
//     "pay this number" contract.
//
// On success:
//   - Pays `cost`.
//   - Clears the disguise-specific runtime flags: "kw:ward",
//     "ward_cost", "disguise_face_down", "face_down", "morph_creature".
//     The ward {2} granted by §702.166a only applies while face-down,
//     so it must come off the moment the permanent flips face-up; the
//     printed ward (if any) on the AST will continue to be detected by
//     HasKeyword via the standard AST path.
//   - Delegates to TurnFaceUp(gs, perm, "disguise") for the actual flip
//     (Card.FaceDown clearing, timestamp bump, characteristics cache
//     invalidation, "turn_face_up" event).
//   - Logs a "disguise_turn_face_up" event with the rule citation and
//     cost.
//
// Returns true on a successful flip, false if validation or payment
// failed.
func TurnFaceUpDisguise(gs *GameState, perm *Permanent, cost int) bool {
	if gs == nil || perm == nil || perm.Card == nil {
		return false
	}
	if !IsFaceDown(perm) {
		return false
	}
	if perm.Flags == nil || perm.Flags["disguise_face_down"] != 1 {
		return false
	}
	seatIdx := perm.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return false
	}
	pool := EnsureTypedPool(seat)
	if pool.Total() < cost {
		return false
	}

	// Pay the disguise cost.
	if !PayGenericCost(gs, seat, cost, "disguise",
		"disguise_turn_face_up", perm.Card.DisplayName()) {
		seat.ManaPool -= cost
		SyncManaAfterSpend(seat)
	}

	// Strip the face-down + ward {2} runtime grants. The printed
	// abilities on the AST come back into play automatically once
	// Card.FaceDown is false — the layers system stops applying the
	// §707.2 override and BaseCharacteristics walks the AST as normal.
	delete(perm.Flags, "kw:ward")
	delete(perm.Flags, "ward_cost")
	delete(perm.Flags, "disguise_face_down")
	delete(perm.Flags, "face_down")
	delete(perm.Flags, "morph_creature")

	// Flip via the canonical TurnFaceUp helper (dfc.go). It clears
	// Card.FaceDown, bumps the timestamp, invalidates the cache, and
	// emits the "turn_face_up" event with rule 702.36e. We then log a
	// disguise-specific event so analytics / hat can distinguish a
	// disguise flip from a morph flip.
	if !TurnFaceUp(gs, perm, "disguise") {
		return false
	}
	perm.SummoningSick = false

	gs.LogEvent(Event{
		Kind:   "disguise_turn_face_up",
		Seat:   seatIdx,
		Source: perm.Card.DisplayName(),
		Amount: cost,
		Details: map[string]interface{}{
			"rule": "702.166b",
		},
	})
	return true
}
