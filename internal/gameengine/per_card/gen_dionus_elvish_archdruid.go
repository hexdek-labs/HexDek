package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerDionusElvishArchdruid wires Dionus, Elvish Archdruid.
//
// Oracle text (Scryfall, verified):
//
//	Elves you control have "Whenever this creature becomes tapped
//	during your turn, untap it and put a +1/+1 counter on it. This
//	ability triggers only once each turn."
//
// Implementation (R46 stub port):
//   - OnTrigger("tap_event"): fires from combat.go and activation.go
//     when a permanent transitions to tapped. Gate to:
//       (a) tapped perm controlled by Dionus's controller
//       (b) tapped perm is a creature with "elf" subtype
//       (c) it's Dionus's controller's turn (gs.Active matches)
//       (d) per-perm per-turn flag "dionus_used_t<turn>" not set
//     Then untap the perm, add a +1/+1 counter, and stamp the
//     once-per-turn flag.
//   - Per-turn flag uses gs.Turn+1 as the value to avoid the
//     zero-turn-vs-unset ambiguity (matches the Ashling /
//     Cecily / Zaffai conventions).
func registerDionusElvishArchdruid(r *Registry) {
	r.OnETB("Dionus, Elvish Archdruid", dionusElvishArchdruidETB)
	r.OnTrigger("Dionus, Elvish Archdruid", "tap_event", dionusElfTapped)
}

func dionusElvishArchdruidETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "dionus_elvish_archdruid_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}

func dionusElfTapped(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "dionus_elf_tap_untap_counter"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	// Only during your turn.
	if gs.Active != perm.Controller {
		return
	}
	tapped, _ := ctx["perm"].(*gameengine.Permanent)
	if tapped == nil || tapped.Card == nil {
		return
	}
	if tapped.Controller != perm.Controller {
		return
	}
	if !tapped.IsCreature() {
		return
	}
	if !cardHasSubtype(tapped.Card, "elf") {
		return
	}
	if tapped.Flags == nil {
		tapped.Flags = map[string]int{}
	}
	key := dionusTurnKey(gs.Turn)
	if tapped.Flags[key] != 0 {
		// Already triggered this turn — once-per-turn rider holds.
		return
	}
	tapped.Flags[key] = gs.Turn + 1
	tapped.Tapped = false
	tapped.AddCounter("+1/+1", 1)
	gs.InvalidateCharacteristicsCache()

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
		"elf":  tapped.Card.DisplayName(),
	})
}

func dionusTurnKey(turn int) string {
	return "dionus_used_t" + itoa(turn)
}
