package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTeysaKarlov wires Teysa Karlov.
//
// Oracle text:
//
//	If a creature dying causes a triggered ability of a permanent you
//	control to trigger, that ability triggers an additional time.
//	Creature tokens you control have vigilance and lifelink.
//
// Implementation:
//
//   - Death trigger doubling: OnETB sets seat-level flag
//     "teysa_death_double" so the engine's death-trigger dispatch can
//     check for it and fire each matching trigger an extra time.
//     The actual doubling requires the engine's fireTrigger /
//     FireZoneChangeTriggers path to read the flag and re-fire;
//     emitPartial flags the coverage gap until that hook is wired.
//
//   - Token vigilance + lifelink: OnETB sets seat-level flags
//     "teysa_token_vigilance" and "teysa_token_lifelink". The engine's
//     token creation and combat systems should grant vigilance and
//     lifelink to creature tokens when these flags are present.
//     emitPartial flags the gap until the layers/combat pipeline reads
//     these flags.
//
//   - OnTrigger("permanent_ltb"): when Teysa herself leaves the
//     battlefield all three flags are removed from her controller's seat.
func registerTeysaKarlov(r *Registry) {
	r.OnETB("Teysa Karlov", teysaKarlovETB)
	r.OnTrigger("Teysa Karlov", "permanent_ltb", teysaKarlovLTB)
}

func teysaKarlovETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "teysa_karlov_etb"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil {
		return
	}
	if s.Flags == nil {
		s.Flags = map[string]int{}
	}

	// Death trigger doubling flag.
	s.Flags["teysa_death_double"] = 1

	// Token keyword grants.
	s.Flags["teysa_token_vigilance"] = 1
	s.Flags["teysa_token_lifelink"] = 1

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   seat,
		"effect": "death_trigger_double_and_token_keywords",
	})

	// The engine does not yet have a "would_fire_death_trigger"
	// replacement event (unlike "would_fire_etb_trigger" used by Yarok).
	// The flag is set so future engine work can read it, but until the
	// death-trigger dispatch path checks teysa_death_double, the
	// doubling is incomplete.
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"death trigger doubling: engine fireTrigger path does not yet check seat.Flags[\"teysa_death_double\"] to re-fire creature-dies triggers")

	// Token vigilance + lifelink requires the layers/combat pipeline to
	// read these seat flags and grant keywords to creature tokens.
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"token vigilance+lifelink: engine layers/combat do not yet read seat.Flags[\"teysa_token_vigilance\"/\"teysa_token_lifelink\"] for creature token keyword grants")
}

// teysaKarlovLTB fires on "permanent_ltb" for every permanent that leaves
// the battlefield. We gate on the leaving permanent being Teysa herself
// (name match) to clean up the seat flags.
func teysaKarlovLTB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "teysa_karlov_ltb"
	if gs == nil || perm == nil || ctx == nil {
		return
	}

	// The trigger fires for EVERY permanent that leaves. We only care
	// when the permanent that left IS Teysa Karlov — gate on the dying
	// permanent's name from ctx["perm"].
	leavingPerm, _ := ctx["perm"].(*gameengine.Permanent)
	if leavingPerm == nil || leavingPerm.Card == nil {
		return
	}
	if normalizeName(leavingPerm.Card.DisplayName()) != normalizeName("Teysa Karlov") {
		return
	}
	// Only clean up flags for OUR seat (the Teysa that left).
	if leavingPerm.Controller != perm.Controller {
		return
	}

	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil || s.Flags == nil {
		return
	}

	delete(s.Flags, "teysa_death_double")
	delete(s.Flags, "teysa_token_vigilance")
	delete(s.Flags, "teysa_token_lifelink")

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   seat,
		"effect": "flags_removed",
	})
}
