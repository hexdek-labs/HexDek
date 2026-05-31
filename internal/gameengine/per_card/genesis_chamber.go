package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerGenesisChamber wires Genesis Chamber (Muninn parser-gap #41, 22,662 hits).
//
// Oracle text (Scryfall, verified 2026-05-16 via hexdek.dev oracle):
//
//	{2}
//	Artifact
//	Whenever a nontoken creature enters, if this artifact is untapped,
//	that creature's controller creates a 1/1 colorless Myr artifact
//	creature token.
//
// Note: this is the symmetric form. The trigger fires for EVERY player's
// nontoken creature ETB, and the resulting token is owned by the
// entering creature's controller — not by the Genesis Chamber controller.
// We deliberately do NOT gate on perm.Controller for that reason.
func registerGenesisChamber(r *Registry) {
	r.OnTrigger("Genesis Chamber", "permanent_etb", genesisChamberPermETB)
}

func genesisChamberPermETB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "genesis_chamber_myr_token"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	if perm.Tapped {
		return
	}
	entering, _ := ctx["perm"].(*gameengine.Permanent)
	if entering == nil {
		entering, _ = ctx["permanent"].(*gameengine.Permanent)
	}
	if entering == nil || entering == perm || entering.Card == nil {
		return
	}
	if !entering.IsCreature() {
		return
	}
	if entering.IsToken() {
		return
	}
	controller := entering.Controller
	if controller < 0 || controller >= len(gs.Seats) {
		return
	}
	token := &gameengine.Card{
		Name:          "Myr Token",
		Owner:         controller,
		BasePower:     1,
		BaseToughness: 1,
		Types:         []string{"token", "artifact", "creature", "myr"},
		TypeLine:      "Token Artifact Creature — Myr",
	}
	enterBattlefieldWithETB(gs, controller, token, false)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"trigger":  controller,
		"entered":  entering.Card.DisplayName(),
	})
}
