package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerEchocastingSymposium wires up Echocasting Symposium.
//
// Oracle text:
//
//	Target player creates a token that's a copy of target creature
//	you control. Paradigm
//
// {3}{U} Sorcery — Secrets of Strixhaven.
//
// Simplified: pick the best creature on controller's battlefield
// (highest P+T), DeepCopy it as a token, and enter the battlefield
// under the target player's control. MVP: target player = controller.
func registerEchocastingSymposium(r *Registry) {
	r.OnResolve("Echocasting Symposium", echocastingSymposiumResolve)
}

func echocastingSymposiumResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "echocasting_symposium"
	const cardName = "Echocasting Symposium"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil {
		return
	}

	// --- Effect: find the best creature on controller's battlefield. ---
	var bestPerm *gameengine.Permanent
	bestScore := -1
	for _, p := range s.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if !cardHasType(p.Card, "creature") {
			continue
		}
		score := p.Card.BasePower + p.Card.BaseToughness
		if score > bestScore {
			bestScore = score
			bestPerm = p
		}
	}
	if bestPerm == nil {
		emitFail(gs, slug, cardName, "no_creature_on_battlefield", nil)
		// Still do paradigm exile.
		paradigmExileItem(gs, item, seat, slug, cardName)
		return
	}

	// Create a token copy via DeepCopy.
	tokenCard := bestPerm.Card.DeepCopy()
	tokenCard.Name = bestPerm.Card.DisplayName() + " Token"
	tokenCard.Owner = seat
	// Ensure token type is present.
	hasToken := false
	for _, t := range tokenCard.Types {
		if t == "token" {
			hasToken = true
			break
		}
	}
	if !hasToken {
		tokenCard.Types = append(tokenCard.Types, "token")
	}
	tokenCard.IsCopy = true

	perm := enterBattlefieldWithETB(gs, seat, tokenCard, false)
	gs.LogEvent(gameengine.Event{
		Kind: "create_token",
		Seat: seat,
		Source: cardName,
		Details: map[string]interface{}{
			"token":  tokenCard.Name,
			"copied": bestPerm.Card.DisplayName(),
			"reason": slug,
		},
	})
	_ = perm // suppress unused

	// --- Paradigm: exile instead of graveyard, register for auto-copy. ---
	paradigmExileItem(gs, item, seat, slug, cardName)
}

// paradigmExileItem is a shared helper for paradigm spell post-resolution:
// set exile-on-resolve flag and register for paradigm auto-copy.
//
// Skips both for paradigm-cast copies. CR §707.10 — a copy of a non-
// permanent spell ceases to exist on resolution; it never lands in exile
// to be paradigm-cast again. The original (placed in ParadigmExile by the
// FIRST resolution) is what funds future copies. r41 game 181 / Decorum
// Dissertation tripped the ZoneConservation invariant because every
// paradigm tick re-registered the (copy's) Card into ParadigmExile —
// accumulating 11 dangling refs by turn 47 even though the actual exile
// zone only held the single original.
func paradigmExileItem(gs *gameengine.GameState, item *gameengine.StackItem, seat int, slug, cardName string) {
	if item != nil && item.IsCopy {
		emit(gs, slug, cardName, map[string]interface{}{
			"seat":          seat,
			"paradigm_copy": true,
		})
		return
	}
	if item.CostMeta == nil {
		item.CostMeta = map[string]interface{}{}
	}
	item.CostMeta["exile_on_resolve"] = true
	gameengine.RegisterParadigmExile(gs, seat, item.Card)

	emit(gs, slug, cardName, map[string]interface{}{
		"seat":     seat,
		"paradigm": true,
	})
}
