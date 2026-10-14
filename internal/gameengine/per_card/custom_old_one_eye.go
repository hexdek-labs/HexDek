package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerOldOneEyeCustom wires Old One Eye's "Other creatures you
// control have trample" anthem. The auto-generated handler in
// gen_old_one_eye.go covers ETB token creation; the partial it emits
// is the trample-anthem gap.
//
// The AST keyword pipeline doesn't pick up Old One Eye's anthem because
// the printed text encodes it as a static ability that needs to react
// to permanents entering/leaving. We approximate by stamping
// kw:trample on every other own creature on ETB and refreshing on
// permanent_etb (new creatures arrive). Old One Eye's own departure is
// handled by oldOneEyeOwnLTB, which strips the stamps it owns — until
// r65 the refresh ran on that event instead and RE-STAMPED, so the
// anthem outlived Old One Eye for the rest of the game.
//
// "Fast Healing" first-main-phase recursion is a delayed trigger out of
// scope here — the gen_*.go partial still documents that gap.
// oldOneEyeTrampleMarker is this anthem's source identity in the r87
// refcount (keyword_flag_grants_r87.go), replacing the old
// Flags["kw:trample_from_old_one_eye"] provenance tag, which the layer
// fold took verbatim into chars.Keywords as a keyword of that name.
const oldOneEyeTrampleMarker = "old_one_eye_trample"

func registerOldOneEyeCustom(r *Registry) {
	r.OnETB("Old One Eye", oldOneEyeApplyAnthemOnETB)
	r.OwnsETBTrigger("Old One Eye")
	r.OnTrigger("Old One Eye", "permanent_etb", oldOneEyeApplyAnthemOnEvent)
	r.OnTrigger("Old One Eye", "permanent_ltb", oldOneEyeOwnLTB)
}

func oldOneEyeApplyAnthemOnETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	oldOneEyeRefreshAnthem(gs, perm)
}

func oldOneEyeApplyAnthemOnEvent(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	oldOneEyeRefreshAnthem(gs, perm)
}

func oldOneEyeRefreshAnthem(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "old_one_eye_trample_anthem"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	stamped := 0
	for _, p := range seat.Battlefield {
		if p == nil || p == perm || !p.IsCreature() {
			continue
		}
		// r91: the guard here used to be `if p.Flags["kw:trample"] != 1`
		// — skip anything that already shows trample. That is the exact
		// defect r87's doc names: it made Old One Eye decline to RECORD
		// its own grant whenever another source had got there first, so
		// when that other source left it deleted the shared flag and
		// took trample away with Old One Eye still on the battlefield.
		// The refcount is what makes the guard unnecessary: granting
		// twice under the same marker is idempotent, and the shared flag
		// survives until the last holder revokes.
		if !gameengine.HasKeywordFlagGrant(p, "trample", oldOneEyeTrampleMarker) {
			stamped++
		}
		gameengine.GrantKeywordFlag(p, "trample", oldOneEyeTrampleMarker)
	}
	if stamped > 0 {
		gs.InvalidateCharacteristicsCache()
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":    perm.Controller,
			"stamped": stamped,
			"keyword": "trample",
		})
	}
}

// ---------------------------------------------------------------------------
// r65 — static anthem stamps outlived their own source.
//
// The anthem is applied by stamping Flags["kw:<name>"] onto each
// matching creature, with a provenance marker recording who stamped it.
// The refresh function was registered for permanent_etb AND
// permanent_ltb, so when the ANTHEM SOURCE ITSELF left the battlefield
// the handler re-ran and RE-STAMPED instead of stripping: the buff
// survived its source for the rest of the game.
//
// The provenance marker existed precisely to make the strip possible;
// nothing ever performed it. Fixed by following the in-repo canonical
// shape (brudiclad_telchor_engineer.go): a dedicated own-LTB handler
// that compares ctx["perm"] against the source and strips the grants it
// owns, falling through to the ordinary recompute when some OTHER
// permanent is the one leaving.
// ---------------------------------------------------------------------------

func oldOneEyeOwnLTB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	if leaving, _ := ctx["perm"].(*gameengine.Permanent); leaving == perm {
		oldOneEyeStripAnthem(gs, perm)
		return
	}
	oldOneEyeRefreshAnthem(gs, perm)
}

// oldOneEyeStripAnthem drops only the trample grants this Old One Eye
// stamped; printed trample and other sources' grants are untouched.
func oldOneEyeStripAnthem(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "old_one_eye_trample_anthem_strip"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	stripped := 0
	for _, p := range seat.Battlefield {
		if p == nil || p == perm || p.Flags == nil {
			continue
		}
		if !gameengine.HasKeywordFlagGrant(p, "trample", oldOneEyeTrampleMarker) {
			continue
		}
		gameengine.RevokeKeywordFlag(p, "trample", oldOneEyeTrampleMarker)
		stripped++
	}
	if stripped > 0 {
		gs.InvalidateCharacteristicsCache()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller, "stripped": stripped,
	})
}
