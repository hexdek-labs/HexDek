package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTheMasterMultiplied wires The Master, Multiplied.
//
// Oracle text (Scryfall, verified):
//
//	Myriad
//	The "legend rule" doesn't apply to creature tokens you control.
//	Triggered abilities you control can't cause you to sacrifice or
//	exile creature tokens you control.
//
// Implementation (R52 batch K port — extends R45 stub):
//   - Myriad: AST keyword pipeline.
//   - Legend-rule exception for creature tokens: engine-side via the
//     "The Master, Multiplied" presence check in sba.go sba704_5j.
//   - Trigger-driven sac/exile suppression: ETB stamps the canonical
//     "master_multiplied_trigger_protected" flag on every creature
//     token the controller currently controls, and a permanent_etb
//     refresh handler re-stamps the flag on every newly-entering
//     creature token under the controller. permanent_ltb clears the
//     stamp when Master leaves so the protection doesn't outlive him.
//     The engine's triggered-ability sac/exile dispatch (when wired)
//     reads this flag and skips matching targets per CR §603.6.
func registerTheMasterMultiplied(r *Registry) {
	r.OnETB("The Master, Multiplied", theMasterMultipliedETB)
	r.OnTrigger("The Master, Multiplied", "permanent_etb", theMasterMultipliedRefresh)
	r.OnTrigger("The Master, Multiplied", "permanent_ltb", theMasterMultipliedLTBCleanup)
}

func theMasterMultipliedETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "the_master_multiplied_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	stamped := theMasterMultipliedStampOwnTokens(gs, perm)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":              perm.Controller,
		"legend_rule_skip":  true,
		"sba_change_active": true,
		"tokens_stamped":    stamped,
	})
}

func theMasterMultipliedRefresh(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	entering, _ := ctx["perm"].(*gameengine.Permanent)
	if entering == nil || entering.Card == nil {
		return
	}
	if entering.Controller != perm.Controller {
		return
	}
	if !entering.IsCreature() || !cardHasType(entering.Card, "token") {
		return
	}
	if entering.Flags == nil {
		entering.Flags = map[string]int{}
	}
	entering.Flags["master_multiplied_trigger_protected"] = 1
}

func theMasterMultipliedLTBCleanup(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	leaving, _ := ctx["perm"].(*gameengine.Permanent)
	if leaving != perm {
		return
	}
	// If another Master is still in play, leave the stamps alone.
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p == perm || p.Card == nil {
				continue
			}
			if normalizeName(p.Card.DisplayName()) == normalizeName("The Master, Multiplied") {
				return
			}
		}
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	for _, p := range seat.Battlefield {
		if p == nil || p.Flags == nil {
			continue
		}
		delete(p.Flags, "master_multiplied_trigger_protected")
	}
}

func theMasterMultipliedStampOwnTokens(gs *gameengine.GameState, perm *gameengine.Permanent) int {
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return 0
	}
	n := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if !p.IsCreature() || !cardHasType(p.Card, "token") {
			continue
		}
		if p.Flags == nil {
			p.Flags = map[string]int{}
		}
		p.Flags["master_multiplied_trigger_protected"] = 1
		n++
	}
	return n
}
