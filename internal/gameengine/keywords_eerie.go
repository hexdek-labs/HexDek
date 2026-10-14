package gameengine

// keywords_eerie.go — CR §702.182 Eerie (Duskmourn: House of Horror, 2024).
//
// Eerie is a triggered ability gated on either of two events:
//
//   - An enchantment the controller controls enters the battlefield.
//   - The controller fully unlocks a Room (both halves unlocked).
//
// Reminder text (§702.182a):
//
//   "Eerie — When an enchantment you control enters or you fully
//    unlock a Room, [effect]."
//
// Modeling:
//
//   - HasEerie(card)                    keyword detection.
//   - OnEnchantmentETB(gs, perm)        scan the controller's
//                                       battlefield for eerie permanents
//                                       and fire one "eerie" trigger per
//                                       carrier. Called from
//                                       FirePermanentETBTriggers when the
//                                       entering permanent is an
//                                       enchantment (or via a per-card
//                                       hook for handlers that need an
//                                       earlier entry point).
//   - OnRoomFullyUnlocked(gs, perm)     mirror of OnEnchantmentETB for
//                                       the Room-unlock branch. Callers
//                                       (the future Room infrastructure)
//                                       invoke this when a Room transi-
//                                       tions to fully unlocked.
//   - IsRoomFullyUnlocked(perm)         minimal Room-state predicate:
//                                       true when Flags["room_unlocked_a"]
//                                       AND Flags["room_unlocked_b"] are
//                                       both set. A standalone helper so
//                                       the eerie call site doesn't have
//                                       to bake Room conventions into its
//                                       own logic, and so tests can mock
//                                       the unlock state with a single
//                                       flag-set call.
//
// The "eerie" trigger event is dispatched via FireCardTrigger with ctx:
//
//   "trigger_perm":    *Permanent  — the enchantment that ETB'd, or the
//                                    Room that was fully unlocked.
//   "eerie_source":    *Permanent  — the permanent carrying the eerie
//                                    keyword whose ability is firing.
//   "controller_seat": int         — the eerie source's controller (the
//                                    same as trigger_perm's controller;
//                                    eerie only fires for the carrier's
//                                    OWN controller's events).
//   "cause":           string      — "enchantment_etb" or "room_unlocked"

// HasEerie reports whether the card carries the eerie keyword.
func HasEerie(card *Card) bool {
	return cardHasKeywordByName(card, "eerie")
}

// IsRoomFullyUnlocked reports whether `perm` is a Room with both halves
// unlocked. The minimal Room state model used here:
//
//   - Flags["room_unlocked_a"] = 1 — first half unlocked
//   - Flags["room_unlocked_b"] = 1 — second half unlocked
//
// Both must be set. A non-Room permanent (one whose Card.Types doesn't
// include "room" — Rooms are an enchantment subtype) returns false even
// when both flags happen to be set, since "fully unlock a Room" per
// §702.182a is specifically about Rooms.
//
// Returns false for nil perm.
func IsRoomFullyUnlocked(perm *Permanent) bool {
	if perm == nil || perm.Card == nil {
		return false
	}
	if !cardHasSubtype(perm.Card, "room") {
		return false
	}
	if perm.Flags == nil {
		return false
	}
	return perm.Flags["room_unlocked_a"] == 1 && perm.Flags["room_unlocked_b"] == 1
}

// OnEnchantmentETB fires one "eerie" trigger per eerie-carrying
// permanent on `enteredPerm`'s controller's battlefield. CR §702.182a,
// first branch.
//
// Gating rules:
//   - `enteredPerm` must be an enchantment. Non-enchantment ETBs are
//     no-ops (defensive — call sites should already filter, but the
//     wider FirePermanentETBTriggers fan-out makes a guard worth it).
//   - Only eerie carriers controlled by the SAME seat as `enteredPerm`
//     fire. Per the printed text "an enchantment YOU CONTROL enters,"
//     opponents' enchantments don't trigger your eerie.
//   - The entering permanent itself can trigger its own eerie if it has
//     the keyword (a card with "Eerie — ..." that's also an enchantment
//     reads its own ETB as the triggering event).
//   - Each eerie source fires exactly once per call, even if multiple
//     enchantments ETB in the same SBA pass (the caller fires this hook
//     per ETB, so the per-call semantics match the per-event semantics).
func OnEnchantmentETB(gs *GameState, enteredPerm *Permanent) {
	if gs == nil || enteredPerm == nil || enteredPerm.Card == nil {
		return
	}
	if !enteredPerm.IsEnchantment() {
		return
	}
	seatIdx := enteredPerm.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return
	}

	enteredName := enteredPerm.Card.DisplayName()

	for _, src := range seat.Battlefield {
		if src == nil || src.Card == nil {
			continue
		}
		if !HasEerie(src.Card) {
			continue
		}
		// Don't fire for face-down permanents — CR §708.2 strips their
		// abilities, so an eerie keyword on the printed AST is silent
		// while face-down.
		if src.Flags != nil && src.Flags["face_down"] == 1 {
			continue
		}
		FireCardTrigger(gs, "eerie", map[string]interface{}{
			"trigger_perm":    enteredPerm,
			"eerie_source":    src,
			"controller_seat": seatIdx,
			"cause":           "enchantment_etb",
		})
		gs.LogEvent(Event{
			Kind:   "eerie_trigger",
			Seat:   seatIdx,
			Source: src.Card.DisplayName(),
			Details: map[string]interface{}{
				"cause":         "enchantment_etb",
				"trigger_perm":  enteredName,
				"rule":          "702.182a",
			},
		})
	}
}

// OnRoomFullyUnlocked fires one "eerie" trigger per eerie-carrying
// permanent on the Room's controller's battlefield. CR §702.182a,
// second branch.
//
// The caller is responsible for actually transitioning the Room's
// unlock state and for invoking this exactly once per "fully unlock"
// event (CR §726.x Room rules — when a player activates the second
// door's mana cost, the Room becomes fully unlocked). We do verify
// `room` IS a fully-unlocked Room via IsRoomFullyUnlocked, so a
// premature call (only one half unlocked) is a no-op.
//
// Gating rules mirror OnEnchantmentETB: only eerie carriers controlled
// by the Room's controller fire (opponents' unlocks don't trigger
// your eerie).
func OnRoomFullyUnlocked(gs *GameState, room *Permanent) {
	if gs == nil || room == nil || room.Card == nil {
		return
	}
	if !IsRoomFullyUnlocked(room) {
		return
	}
	seatIdx := room.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return
	}

	roomName := room.Card.DisplayName()

	for _, src := range seat.Battlefield {
		if src == nil || src.Card == nil {
			continue
		}
		if !HasEerie(src.Card) {
			continue
		}
		if src.Flags != nil && src.Flags["face_down"] == 1 {
			continue
		}
		FireCardTrigger(gs, "eerie", map[string]interface{}{
			"trigger_perm":    room,
			"eerie_source":    src,
			"controller_seat": seatIdx,
			"cause":           "room_unlocked",
		})
		gs.LogEvent(Event{
			Kind:   "eerie_trigger",
			Seat:   seatIdx,
			Source: src.Card.DisplayName(),
			Details: map[string]interface{}{
				"cause":        "room_unlocked",
				"trigger_perm": roomName,
				"rule":         "702.182a",
			},
		})
	}
}
