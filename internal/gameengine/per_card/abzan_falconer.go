package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAbzanFalconer wires Abzan Falconer (Muninn parser-gap rank ~134,
// counters-matter +1/+1 anthem-grant).
//
// Oracle text (Scryfall, verified 2026-05-17 via hexdek.dev oracle):
//
//	{2}{W}
//	Creature — Human Soldier
//	Outlast {W} ({W}, {T}: Put a +1/+1 counter on this creature. Outlast
//	only as a sorcery.)
//	Each creature you control with a +1/+1 counter on it has flying.
//
// Implementation:
//   - Static "creatures with +1/+1 counters have flying" is a CR §613
//     continuous effect — the engine doesn't yet evaluate per-card
//     static grants from arbitrary battlefield permanents. Closest
//     existing pattern is Modular's keyword grant (post-death) which
//     stamps Flags["kw:flying"]. We mirror that by sweeping on every
//     counter_placed event and stamping kw:flying on each creature the
//     controller owns that currently has a +1/+1 counter. We also sweep
//     on permanent_etb so freshly-entered creatures pick up the grant.
//   - Outlast activation (cost {W}, tap, sorcery-speed) is the same
//     activated-ability cost-gating gap noted across the outlast family
//     — emitPartial.
//   - Stripping the grant when Abzan Falconer leaves the battlefield
//     uses permanent_ltb on Abzan itself: clear kw:flying flags we set.
//     We can't reliably distinguish flags we set from a native AST
//     flying; safe approach is to leave already-flying creatures alone
//     (skip the grant when they already have flying via AST/HasKeyword).
func registerAbzanFalconer(r *Registry) {
	r.OnETB("Abzan Falconer", abzanFalconerETB)
	r.OnTrigger("Abzan Falconer", "counter_placed", abzanFalconerCounterPlaced)
	r.OnTrigger("Abzan Falconer", "permanent_etb", abzanFalconerPermETB)
	r.OnTrigger("Abzan Falconer", "permanent_ltb", abzanFalconerOwnLTB)
}

func abzanFalconerETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	abzanFalconerGrantSweep(gs, perm)
}

func abzanFalconerCounterPlaced(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	abzanFalconerGrantSweep(gs, perm)
}

func abzanFalconerPermETB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	abzanFalconerGrantSweep(gs, perm)
}

// abzanFalconerFlyingMarker is this anthem's source identity in the r87
// refcount (keyword_flag_grants_r87.go). The private
// "abzan_falconer_grant" boolean stays as the per-permanent bookkeeping
// the LTB sweep iterates on; the refcount is what makes the shared
// Flags["kw:flying"] safe to share with another granting source.
const abzanFalconerFlyingMarker = "abzan_falconer_flying"

func abzanFalconerOwnLTB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	leaving, _ := ctx["perm"].(*gameengine.Permanent)
	if leaving != perm {
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
		// r93: the private "abzan_falconer_grant" boolean was correct
		// provenance but the delete beside it was not — it removed the
		// SHARED Flags["kw:flying"], taking away any other source's
		// flying grant too. RevokeKeywordFlag drops only this source's
		// marker and deletes the shared flag only when it was the last.
		if gameengine.RevokeKeywordFlag(p, "flying", abzanFalconerFlyingMarker) {
			stripped++
		}
		delete(p.Flags, "abzan_falconer_grant")
	}
	gs.InvalidateCharacteristicsCache()
	emit(gs, "abzan_falconer_grant_strip", perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"stripped": stripped,
	})
}

func abzanFalconerGrantSweep(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "abzan_falconer_flying_grant"
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	granted := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || !p.IsCreature() {
			continue
		}
		if p.Counters == nil || p.Counters["+1/+1"] <= 0 {
			continue
		}
		// r93: two guards used to sit here — `p.HasKeyword("flying")`
		// and `p.Flags["kw:flying"] == 1` — both of which made Abzan
		// decline to RECORD its own grant whenever anything else had
		// got to the shared flag first. That other source then strips
		// and the creature loses flying with Abzan still on the
		// battlefield and the +1/+1 counter still on the creature. The
		// refcount makes both guards unnecessary: granting twice under
		// one marker is idempotent, and a printed AST keyword is not in
		// Flags for the revoke to reach.
		if p.Flags == nil {
			p.Flags = map[string]int{}
		}
		if gameengine.GrantKeywordFlag(p, "flying", abzanFalconerFlyingMarker) {
			granted++
		}
		p.Flags["abzan_falconer_grant"] = 1
	}
	if granted > 0 {
		gs.InvalidateCharacteristicsCache()
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":    perm.Controller,
			"granted": granted,
		})
	}
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"outlast_activation_cost_w_tap_sorcery_speed_unwired_pending_activated_cost_gate")
}
