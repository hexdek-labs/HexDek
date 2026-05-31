package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAdrixAndNevTwincasters wires Adrix and Nev, Twincasters.
//
// Oracle text (Scryfall, verified 2026-05-04):
//
//	{2}{G}{U}
//	Legendary Creature — Merfolk Wizard
//	Ward {2}
//	If one or more tokens would be created under your control, twice
//	that many of those tokens are created instead.
//
// Implementation:
//   - permanent_etb gated on the entering permanent being a token
//     controlled by Adrix's controller: mint one additional copy. This is
//     a replacement effect modeled as a fire-after, which gets the count
//     right (1 token original + 1 doubled copy = 2) but loses the "would
//     be created" timing — emitPartial flags this.
//   - Self-mints from the doubled copy must be filtered to avoid an
//     infinite loop. We mark the doubled token with Flags["adrix_copy"]=1
//     and skip those on re-trigger.
//   - Ward is handled by the AST keyword pipeline.
func registerAdrixAndNevTwincasters(r *Registry) {
	r.OnTrigger("Adrix and Nev, Twincasters", "permanent_etb", adrixAndNevDouble)
}

func adrixAndNevDouble(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "adrix_and_nev_token_doubler"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	enteringPerm, _ := ctx["perm"].(*gameengine.Permanent)
	if enteringPerm == nil {
		enteringPerm, _ = ctx["permanent"].(*gameengine.Permanent)
	}
	if enteringPerm == nil || enteringPerm.Card == nil {
		return
	}
	if enteringPerm.Controller != perm.Controller {
		return
	}
	if !enteringPerm.IsToken() {
		return
	}
	if enteringPerm.Flags != nil && enteringPerm.Flags["adrix_copy"] == 1 {
		return
	}
	// Recursion guard: the doubled copy's Card.Name carries a " (Adrix copy)"
	// suffix. The Flags["adrix_copy"]=1 stamp lands AFTER enterBattlefieldWithETB
	// returns, but the inner ETB cascade re-fires permanent_etb for the copy
	// BEFORE that stamp lands — so the Flags-only gate above doesn't break the
	// loop on its own. Name-suffix check catches the copy mid-cascade.
	if strings.HasSuffix(enteringPerm.Card.Name, " (Adrix copy)") {
		return
	}
	src := enteringPerm.Card
	copy := &gameengine.Card{
		Name:          src.DisplayName() + " (Adrix copy)",
		Owner:         perm.Controller,
		BasePower:     src.BasePower,
		BaseToughness: src.BaseToughness,
		Types:         append([]string{}, src.Types...),
		Colors:        append([]string{}, src.Colors...),
		TypeLine:      src.TypeLine,
	}
	tokenPerm := enterBattlefieldWithETB(gs, perm.Controller, copy, enteringPerm.Tapped)
	if tokenPerm != nil {
		if tokenPerm.Flags == nil {
			tokenPerm.Flags = map[string]int{}
		}
		tokenPerm.Flags["adrix_copy"] = 1
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"original": src.DisplayName(),
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"replacement_effect_modeled_as_post_etb_fire_timing_drift_acceptable")
}
