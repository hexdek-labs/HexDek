package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSkullclamp wires Skullclamp.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Skullclamp):
//
//	Equipped creature gets +1/-1.
//	Whenever equipped creature dies, draw two cards.
//	Equip {1}
//
// {1} Artifact Equipment. The "+1/-1" continuous effect rides through
// the AST keyword pipeline; the equip cost activation is handled by
// the engine's standard equipment surface. This handler implements
// the "equipped creature dies → draw 2" trigger.
//
// Implementation note: zone_change.go calls detachAll(gs, perm) BEFORE
// FireZoneChangeTriggers, so by the time a creature_dies trigger fires
// the dying perm is no longer connected to its attachments (Skullclamp's
// AttachedTo is already nil and the dying perm has no upward link to
// Skullclamp either). To preserve the equipped-at-death fact across
// the detach, this handler hooks the `would_die` REPLACEMENT — fired
// inside FireDieEvent BEFORE detach — and stamps the dying perm's
// Flags with "skullclamp_draw_for_seat_<seat>_p1" (controller + 1, so
// zero distinguishes "not equipped" from "seat 0"). The creature_dies
// trigger then reads that flag and draws 2 cards.
//
// The replacement does NOT cancel or mutate the event; it's purely
// observational. UnregisterReplacementsForPermanent automatically
// drops the replacement on Skullclamp LTB.
func registerSkullclamp(r *Registry) {
	r.OnETB("Skullclamp", skullclampETB)
	r.OnTrigger("Skullclamp", "creature_dies", skullclampOnCreatureDies)
}

func skullclampETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	registerSkullclampStampReplacement(gs, perm)
	emit(gs, "skullclamp_etb", perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"timestamp": perm.Timestamp,
	})
}

// registerSkullclampStampReplacement registers a would_die replacement
// that stamps the dying perm's Flags when it was attached to this
// Skullclamp. The stamp survives detachAll because it lives on the
// dying perm, not on Skullclamp. The replacement itself does not
// alter the event.
func registerSkullclampStampReplacement(gs *gameengine.GameState, sc *gameengine.Permanent) {
	if gs == nil || sc == nil {
		return
	}
	flagKey := skullclampStampFlagKey(sc)
	gs.RegisterReplacement(&gameengine.ReplacementEffect{
		EventType:      "would_die",
		HandlerID:      "skullclamp:stamp:" + sc.Card.DisplayName(),
		SourcePerm:     sc,
		ControllerSeat: sc.Controller,
		Timestamp:      sc.Timestamp,
		Category:       gameengine.CategoryOther,
		Applies: func(gs *gameengine.GameState, ev *gameengine.ReplEvent) bool {
			if ev == nil || ev.TargetPerm == nil {
				return false
			}
			return ev.TargetPerm == sc.AttachedTo
		},
		ApplyFn: func(gs *gameengine.GameState, ev *gameengine.ReplEvent) {
			if ev == nil || ev.TargetPerm == nil {
				return
			}
			if ev.TargetPerm.Flags == nil {
				ev.TargetPerm.Flags = map[string]int{}
			}
			ev.TargetPerm.Flags[flagKey] = sc.Controller + 1
		},
	})
}

func skullclampStampFlagKey(sc *gameengine.Permanent) string {
	// One key per Skullclamp permanent (multiple Skullclamps can co-equip
	// in theory across creatures; tag by timestamp to keep distinct).
	return "skullclamp_die_stamp_ts:" + itoaSC(sc.Timestamp)
}

func itoaSC(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	buf := []byte{}
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	if neg {
		buf = append([]byte{'-'}, buf...)
	}
	return string(buf)
}

func skullclampOnCreatureDies(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "skullclamp_draw"
	if gs == nil || perm == nil {
		return
	}
	dying, _ := ctx["perm"].(*gameengine.Permanent)
	if dying == nil || dying.Flags == nil {
		return
	}
	flagKey := skullclampStampFlagKey(perm)
	stamp := dying.Flags[flagKey]
	if stamp == 0 {
		return // dying creature was not attached to this Skullclamp at death
	}
	seat := stamp - 1
	if seat != perm.Controller {
		return // stale stamp from a renamed Skullclamp; ignore
	}
	// Clear the stamp so repeated creature_dies fan-outs don't double-draw.
	delete(dying.Flags, flagKey)

	drawOne(gs, seat, "Skullclamp")
	drawOne(gs, seat, "Skullclamp")
	emit(gs, slug, "Skullclamp", map[string]interface{}{
		"seat":  seat,
		"dying": dying.Card.DisplayName(),
		"drew":  2,
	})
}
